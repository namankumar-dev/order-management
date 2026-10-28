package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"order-management/metrics"
)

func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		code := c.Writer.Status()
		status := strconv.Itoa(code)

		metrics.HTTPRequests.WithLabelValues(c.Request.Method, path, status).Inc()
		metrics.HTTPDuration.WithLabelValues(c.Request.Method, path).Observe(time.Since(start).Seconds())
		if code >= 500 {
			metrics.HTTPErrors.WithLabelValues(c.Request.Method, path, status).Inc()
		}
	}
}
