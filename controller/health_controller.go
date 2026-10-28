package controller

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type HealthController struct {
	ready func(ctx context.Context) error
}

func NewHealthController(ready func(ctx context.Context) error) *HealthController {
	return &HealthController{ready: ready}
}

func (h *HealthController) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}

func (h *HealthController) Ready(c *gin.Context) {
	if h.ready != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := h.ready(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
