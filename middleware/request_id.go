package middleware

import (
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"order-management/trace"
)

const (
	HeaderRequestID = "X-Request-ID"
	requestIDKey    = "request_id"
)

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if !validRequestID.MatchString(id) {
			id = uuid.NewString()
		}
		c.Set(requestIDKey, id)
		c.Header(HeaderRequestID, id)
		c.Request = c.Request.WithContext(trace.WithRequestID(c.Request.Context(), id))
		c.Next()
	}
}

func GetRequestID(c *gin.Context) string {
	return c.GetString(requestIDKey)
}
