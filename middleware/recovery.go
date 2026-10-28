package middleware

import (
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"order-management/apperror"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic recovered",
					"request_id", GetRequestID(c),
					"panic", fmt.Sprint(r),
					"stack", string(debug.Stack()),
				)
				writeError(c, apperror.Internal())
			}
		}()
		c.Next()
	}
}
