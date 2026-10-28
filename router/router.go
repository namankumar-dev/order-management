package router

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/graphql-go/graphql"

	"order-management/apperror"
	"order-management/auth"
	"order-management/config"
	"order-management/controller"
	"order-management/graph"
	"order-management/middleware"
	"order-management/model"
	"order-management/service"
	"order-management/validation"
)

type Deps struct {
	Config *config.Config
	Orders service.OrderService
	Auth   service.AuthService
	Tokens *auth.TokenManager
	Ready  func(ctx context.Context) error
	Schema graphql.Schema
}

func New(d Deps) (*gin.Engine, error) {
	production := d.Config.IsProduction()
	if production {
		gin.SetMode(gin.ReleaseMode)
	}
	validation.Register()

	r := gin.New()
	r.HandleMethodNotAllowed = true
	if err := r.SetTrustedProxies(d.Config.TrustedProxies); err != nil {
		return nil, err
	}

	r.Use(
		middleware.RequestID(),
		middleware.Logging(),
		middleware.Metrics(),
		middleware.Recovery(),
		middleware.SecurityHeaders(production),
		middleware.RequireHTTPS(production),
		middleware.BodyLimit(1<<20),
		middleware.ErrorHandler(),
	)

	r.NoRoute(func(c *gin.Context) {
		_ = c.Error(apperror.NotFound("route not found"))
	})
	r.NoMethod(func(c *gin.Context) {
		_ = c.Error(apperror.New(405, "METHOD_NOT_ALLOWED", "method not allowed"))
	})

	health := controller.NewHealthController(d.Ready)
	r.GET("/health/live", health.Live)
	r.GET("/health/ready", health.Ready)

	authController := controller.NewAuthController(d.Auth)
	orderController := controller.NewOrderController(d.Orders)

	v1 := r.Group("/api/v1")
	v1.POST("/auth/register", authController.Register)
	v1.POST("/auth/login", authController.Login)

	orders := v1.Group("/orders",
		middleware.Authenticate(d.Tokens),
		middleware.RequireRoles(model.RoleAdmin, model.RoleCustomer),
	)
	orders.POST("", orderController.Create)
	orders.GET("", orderController.List)
	orders.GET("/:id", orderController.Get)
	orders.PUT("/:id/status", orderController.UpdateStatus)

	r.POST("/graphql", middleware.Authenticate(d.Tokens), graph.Handler(d.Schema))

	return r, nil
}
