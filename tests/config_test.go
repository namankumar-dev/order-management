package tests

import (
	"context"
	"strings"
	"testing"

	"order-management/config"
)

func setBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("SECRETS_NAME", "")
	t.Setenv("APP_ENV", "development")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_USER", "orderuser")
	t.Setenv("DB_PASSWORD", "p@ss word")
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
}

func TestConfigLoadDefaults(t *testing.T) {
	setBaseEnv(t)
	cfg, err := config.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppPort != "8080" || cfg.DBName != "orderdb" || cfg.DBPort != "5432" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if !strings.Contains(cfg.DSN(), "p%40ss%20word") {
		t.Fatalf("password must be escaped in dsn: %s", cfg.DSN())
	}
}

func TestConfigRejectsInvalidValues(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("JWT_SECRET", "short")
	if _, err := config.Load(context.Background()); err == nil {
		t.Fatal("expected error for short JWT secret")
	}

	setBaseEnv(t)
	t.Setenv("DB_PASSWORD", "")
	if _, err := config.Load(context.Background()); err == nil {
		t.Fatal("expected error for missing DB password")
	}

	setBaseEnv(t)
	t.Setenv("APP_ENV", "production")
	if _, err := config.Load(context.Background()); err == nil {
		t.Fatal("expected error for insecure production settings")
	}
}
