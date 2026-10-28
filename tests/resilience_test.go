package tests

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"order-management/client"
)

const productJSON = `{"id":501,"name":"Mechanical Keyboard","price":79.99,"stock":10}`

func newTestClient(url string) *client.HTTPProductClient {
	return client.NewHTTPProductClient(client.Options{
		BaseURL:          url,
		Token:            "token",
		Timeout:          200 * time.Millisecond,
		MaxRetries:       2,
		BaseBackoff:      time.Millisecond,
		FailureThreshold: 3,
		OpenTimeout:      time.Minute,
	})
}

func TestRetrySucceedsAfterTransientFailures(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(productJSON))
	}))
	defer srv.Close()

	p, err := newTestClient(srv.URL).GetProduct(context.Background(), 501)
	if err != nil {
		t.Fatal(err)
	}
	if p.Price != 79.99 || calls.Load() != 3 {
		t.Fatalf("price=%v calls=%d", p.Price, calls.Load())
	}
}

func TestRetriesAreBounded(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := client.NewHTTPProductClient(client.Options{
		BaseURL: srv.URL, Token: "t", Timeout: time.Second, MaxRetries: 2,
		BaseBackoff: time.Millisecond, FailureThreshold: 100, OpenTimeout: time.Minute,
	})
	_, err := c.GetProduct(context.Background(), 501)
	if !errors.Is(err, client.ErrProductServiceUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if calls.Load() != 3 {
		t.Fatalf("calls = %d, want 3", calls.Load())
	}
}

func TestCircuitBreakerOpens(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if _, err := c.GetProduct(context.Background(), 501); !errors.Is(err, client.ErrProductServiceUnavailable) {
		t.Fatalf("err = %v", err)
	}
	before := calls.Load()
	if before != 3 {
		t.Fatalf("calls before open = %d", before)
	}

	_, err := c.GetProduct(context.Background(), 501)
	if !errors.Is(err, client.ErrProductServiceUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if calls.Load() != before {
		t.Fatalf("circuit open must not call upstream, calls = %d", calls.Load())
	}
}

func TestNotFoundIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).GetProduct(context.Background(), 1)
	if !errors.Is(err, client.ErrProductNotFound) || calls.Load() != 1 {
		t.Fatalf("err=%v calls=%d", err, calls.Load())
	}
}

func TestPerAttemptTimeout(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		select {
		case <-time.After(time.Second):
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()

	c := client.NewHTTPProductClient(client.Options{
		BaseURL: srv.URL, Token: "t", Timeout: 30 * time.Millisecond, MaxRetries: 1,
		BaseBackoff: time.Millisecond, FailureThreshold: 100, OpenTimeout: time.Minute,
	})
	_, err := c.GetProduct(context.Background(), 501)
	if !errors.Is(err, client.ErrProductServiceUnavailable) || calls.Load() != 2 {
		t.Fatalf("err=%v calls=%d", err, calls.Load())
	}
}

func TestContextCancellationStopsRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(5 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()

	c := client.NewHTTPProductClient(client.Options{
		BaseURL: srv.URL, Token: "t", Timeout: 3 * time.Second, MaxRetries: 3,
		BaseBackoff: time.Millisecond, FailureThreshold: 100, OpenTimeout: time.Minute,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	started := time.Now()
	_, err := c.GetProduct(ctx, 501)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatalf("cancellation took %s", time.Since(started))
	}
}
