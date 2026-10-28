package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/sony/gobreaker"

	"order-management/metrics"
	"order-management/trace"
)

var (
	ErrProductNotFound           = errors.New("product not found")
	ErrProductServiceUnavailable = errors.New("product service unavailable")
)

type Product struct {
	ID    uint    `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Stock int     `json:"stock"`
}

type ProductClient interface {
	GetProduct(ctx context.Context, id uint) (*Product, error)
}

type Options struct {
	BaseURL          string
	Token            string
	Timeout          time.Duration
	MaxRetries       int
	BaseBackoff      time.Duration
	FailureThreshold uint32
	OpenTimeout      time.Duration
}

type HTTPProductClient struct {
	opts    Options
	http    *http.Client
	breaker *gobreaker.CircuitBreaker
}

type permanentError struct {
	err error
}

func (p *permanentError) Error() string { return p.err.Error() }
func (p *permanentError) Unwrap() error { return p.err }

type outcome struct {
	product  *Product
	notFound bool
}

func NewHTTPProductClient(opts Options) *HTTPProductClient {
	breaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "product-service",
		MaxRequests: 1,
		Interval:    30 * time.Second,
		Timeout:     opts.OpenTimeout,
		ReadyToTrip: func(c gobreaker.Counts) bool {
			return c.ConsecutiveFailures >= opts.FailureThreshold
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			slog.Warn("circuit breaker state change", "breaker", name, "from", from.String(), "to", to.String())
		},
	})
	return &HTTPProductClient{
		opts: opts,
		http: &http.Client{Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		}},
		breaker: breaker,
	}
}

func (c *HTTPProductClient) GetProduct(ctx context.Context, id uint) (*Product, error) {
	return trace.Run(ctx, "client.product_service.get_product", func(ctx context.Context) (*Product, error) {
		return c.getWithRetry(ctx, id)
	})
}

func (c *HTTPProductClient) getWithRetry(ctx context.Context, id uint) (*Product, error) {
	var lastErr error
	for attempt := 0; attempt <= c.opts.MaxRetries; attempt++ {
		res, err := c.breaker.Execute(func() (interface{}, error) {
			return c.fetch(ctx, id)
		})
		if err == nil {
			metrics.ProductCalls.WithLabelValues("success").Inc()
			result := res.(*outcome)
			if result.notFound {
				return nil, ErrProductNotFound
			}
			return result.product, nil
		}

		lastErr = err
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			metrics.ProductCalls.WithLabelValues("circuit_open").Inc()
			break
		}
		metrics.ProductCalls.WithLabelValues("failure").Inc()

		var permanent *permanentError
		if errors.As(err, &permanent) || ctx.Err() != nil || attempt == c.opts.MaxRetries {
			break
		}
		if err := sleep(ctx, backoff(c.opts.BaseBackoff, attempt)); err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("%w: %v", ErrProductServiceUnavailable, lastErr)
}

func (c *HTTPProductClient) fetch(ctx context.Context, id uint) (interface{}, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()

	url := fmt.Sprintf("%s/products/%d", c.opts.BaseURL, id)
	req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, &permanentError{err: err}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Internal-Token", c.opts.Token)
	req.Header.Set("X-Request-ID", trace.RequestID(ctx))

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return &outcome{notFound: true}, nil
	case resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("product service returned status %d", resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return nil, &permanentError{err: fmt.Errorf("product service returned status %d", resp.StatusCode)}
	}

	var product Product
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&product); err != nil {
		return nil, &permanentError{err: fmt.Errorf("decode product: %w", err)}
	}
	return &outcome{product: &product}, nil
}

func backoff(base time.Duration, attempt int) time.Duration {
	d := base << attempt
	if d <= 0 {
		return base
	}
	jitter := time.Duration(time.Now().UnixNano() % int64(d/2+1))
	return d/2 + jitter
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
