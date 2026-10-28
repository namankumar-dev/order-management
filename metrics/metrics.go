package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"method", "path", "status"})

	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	HTTPErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_errors_total",
		Help: "Total number of HTTP responses with status 5xx.",
	}, []string{"method", "path", "status"})

	DBErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "db_errors_total",
		Help: "Total number of database errors.",
	})

	ProductCalls = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "product_service_calls_total",
		Help: "Calls to the product service by outcome.",
	}, []string{"outcome"})
)
