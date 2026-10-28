package middleware

import (
	"github.com/gin-gonic/gin"

	"order-management/apperror"
)

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Details   map[string]string `json:"details,omitempty"`
	RequestID string            `json:"requestId"`
}

func writeError(c *gin.Context, ae *apperror.Error) {
	c.AbortWithStatusJSON(ae.Status, errorBody{Error: errorPayload{
		Code:      ae.Code,
		Message:   ae.Message,
		Details:   ae.Details,
		RequestID: GetRequestID(c),
	}})
}

func fail(c *gin.Context, err error) {
	_ = c.Error(err)
	c.Abort()
}

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}
		err := c.Errors.Last().Err
		ae, ok := apperror.As(err)
		if !ok {
			ae = apperror.Internal().WithCause(err)
		}
		writeError(c, ae)
	}
}
