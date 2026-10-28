package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"order-management/auth"
	"order-management/client"
	"order-management/config"
	"order-management/database"
	"order-management/graph"
	"order-management/repository"
	"order-management/router"
	"order-management/service"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func setupLogger(cfg *config.Config) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler = slog.NewJSONHandler(os.Stdout, opts)
	if cfg.LogFormat == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(ctx)
	if err != nil {
		return err
	}
	setupLogger(cfg)
	slog.Info("starting order-management", "version", version, "env", cfg.AppEnv)

	db, err := database.Connect(cfg)
	if err != nil {
		return err
	}
	if sqlDB, err := db.DB(); err == nil {
		defer func() { _ = sqlDB.Close() }()
	}
	if cfg.AutoMigrate {
		if err := database.Migrate(db); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}

	orderRepo := repository.NewOrderRepository(db)
	userRepo := repository.NewUserRepository(db)
	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTTTL)
	products := client.NewHTTPProductClient(client.Options{
		BaseURL:          cfg.ProductServiceURL,
		Token:            cfg.ProductServiceToken,
		Timeout:          cfg.ProductTimeout,
		MaxRetries:       cfg.ProductMaxRetries,
		BaseBackoff:      cfg.ProductBackoff,
		FailureThreshold: cfg.ProductBreakerThreshold,
		OpenTimeout:      cfg.ProductBreakerOpen,
	})

	orderSvc := service.NewOrderService(orderRepo, products)
	authSvc := service.NewAuthService(userRepo, tokens)
	if err := authSvc.SeedAdmin(ctx, cfg.AdminEmail, cfg.AdminPassword); err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}

	schema, err := graph.NewSchema(orderSvc)
	if err != nil {
		return fmt.Errorf("graphql schema: %w", err)
	}

	engine, err := router.New(router.Deps{
		Config: cfg,
		Orders: orderSvc,
		Auth:   authSvc,
		Tokens: tokens,
		Ready:  orderRepo.Ping,
		Schema: schema,
	})
	if err != nil {
		return err
	}

	api := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	metricsServer := &http.Server{
		Addr:              ":" + cfg.MetricsPort,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 2)
	go func() {
		var err error
		if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
			err = api.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			err = api.ListenAndServe()
		}
		if !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	go func() {
		if err := metricsServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	slog.Info("listening", "api", api.Addr, "metrics", metricsServer.Addr)

	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := api.Shutdown(shutdownCtx); err != nil {
		slog.Error("api shutdown", "error", err)
	}
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("metrics shutdown", "error", err)
	}
	return nil
}
