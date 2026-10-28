package main

import (
	"crypto/subtle"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

type Product struct {
	ID    uint    `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Stock int     `json:"stock"`
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	token := os.Getenv("INTERNAL_TOKEN")
	if token == "" {
		log.Fatal("INTERNAL_TOKEN is required")
	}

	products := map[uint]Product{
		501: {ID: 501, Name: "Mechanical Keyboard", Price: 79.99, Stock: 500},
		502: {ID: 502, Name: "USB-C Hub", Price: 34.50, Stock: 250},
		503: {ID: 503, Name: "27-inch Monitor", Price: 219.00, Stock: 40},
	}

	var failing atomic.Bool

	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())

	r.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	})

	internal := r.Group("/", func(c *gin.Context) {
		provided := []byte(c.GetHeader("X-Internal-Token"))
		if subtle.ConstantTimeCompare(provided, []byte(token)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	})

	internal.GET("/products/:id", func(c *gin.Context) {
		if failing.Load() {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "simulated outage"})
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		p, ok := products[uint(id)]
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		c.JSON(http.StatusOK, p)
	})

	internal.POST("/chaos/:mode", func(c *gin.Context) {
		switch c.Param("mode") {
		case "on":
			failing.Store(true)
		case "off":
			failing.Store(false)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "mode must be on or off"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"failing": failing.Load()})
	})

	srv := &http.Server{
		Addr:              ":" + getenv("PORT", "8081"),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}
