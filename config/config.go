package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type Config struct {
	AppEnv      string
	AppPort     string
	MetricsPort string

	DBHost            string
	DBPort            string
	DBName            string
	DBUser            string
	DBPassword        string
	DBSSLMode         string
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration
	AutoMigrate       bool

	LogLevel  string
	LogFormat string

	JWTSecret string
	JWTTTL    time.Duration

	AdminEmail    string
	AdminPassword string

	ProductServiceURL       string
	ProductServiceToken     string
	ProductTimeout          time.Duration
	ProductMaxRetries       int
	ProductBackoff          time.Duration
	ProductBreakerThreshold uint32
	ProductBreakerOpen      time.Duration

	TrustedProxies []string
	TLSCertFile    string
	TLSKeyFile     string
}

func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

func (c *Config) DSN() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.DBUser, c.DBPassword),
		Host:     net.JoinHostPort(c.DBHost, c.DBPort),
		Path:     "/" + c.DBName,
		RawQuery: "sslmode=" + url.QueryEscape(c.DBSSLMode),
	}
	return u.String()
}

type loader struct {
	secrets map[string]string
	errs    []error
}

func (l *loader) str(key, def string) string {
	if v, ok := l.secrets[key]; ok && v != "" {
		return v
	}
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func (l *loader) integer(key string, def int) int {
	raw := l.str(key, "")
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		l.errs = append(l.errs, fmt.Errorf("%s must be an integer", key))
		return def
	}
	return v
}

func (l *loader) duration(key string, def time.Duration) time.Duration {
	raw := l.str(key, "")
	if raw == "" {
		return def
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		l.errs = append(l.errs, fmt.Errorf("%s must be a duration such as 5s", key))
		return def
	}
	return v
}

func (l *loader) boolean(key string, def bool) bool {
	raw := l.str(key, "")
	if raw == "" {
		return def
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		l.errs = append(l.errs, fmt.Errorf("%s must be true or false", key))
		return def
	}
	return v
}

func (l *loader) list(key string) []string {
	raw := l.str(key, "")
	if raw == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func Load(ctx context.Context) (*Config, error) {
	l := &loader{secrets: map[string]string{}}
	if name := os.Getenv("SECRETS_NAME"); name != "" {
		s, err := loadSecrets(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("load secrets: %w", err)
		}
		l.secrets = s
	}

	c := &Config{
		AppEnv:      l.str("APP_ENV", "development"),
		AppPort:     l.str("APP_PORT", "8080"),
		MetricsPort: l.str("METRICS_PORT", "9090"),

		DBHost:            l.str("DB_HOST", ""),
		DBPort:            l.str("DB_PORT", "5432"),
		DBName:            l.str("DB_NAME", "orderdb"),
		DBUser:            l.str("DB_USER", ""),
		DBPassword:        l.str("DB_PASSWORD", ""),
		DBSSLMode:         l.str("DB_SSLMODE", "disable"),
		DBMaxOpenConns:    l.integer("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:    l.integer("DB_MAX_IDLE_CONNS", 10),
		DBConnMaxLifetime: l.duration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
		AutoMigrate:       l.boolean("AUTO_MIGRATE", false),

		LogLevel:  l.str("LOG_LEVEL", "info"),
		LogFormat: l.str("LOG_FORMAT", "json"),

		JWTSecret: l.str("JWT_SECRET", ""),
		JWTTTL:    l.duration("JWT_TTL", 15*time.Minute),

		AdminEmail:    l.str("ADMIN_EMAIL", ""),
		AdminPassword: l.str("ADMIN_PASSWORD", ""),

		ProductServiceURL:       strings.TrimRight(l.str("PRODUCT_SERVICE_URL", "http://localhost:8081"), "/"),
		ProductServiceToken:     l.str("PRODUCT_SERVICE_TOKEN", ""),
		ProductTimeout:          l.duration("PRODUCT_TIMEOUT", 2*time.Second),
		ProductMaxRetries:       l.integer("PRODUCT_MAX_RETRIES", 2),
		ProductBackoff:          l.duration("PRODUCT_BACKOFF", 100*time.Millisecond),
		ProductBreakerThreshold: uint32(l.integer("PRODUCT_BREAKER_THRESHOLD", 5)),
		ProductBreakerOpen:      l.duration("PRODUCT_BREAKER_OPEN", 30*time.Second),

		TrustedProxies: l.list("TRUSTED_PROXIES"),
		TLSCertFile:    l.str("TLS_CERT_FILE", ""),
		TLSKeyFile:     l.str("TLS_KEY_FILE", ""),
	}

	if err := errors.Join(append(l.errs, c.validate())...); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	return c, nil
}

func (c *Config) validate() error {
	var errs []error
	required := func(name, value string) {
		if value == "" {
			errs = append(errs, fmt.Errorf("%s is required", name))
		}
	}
	required("DB_HOST", c.DBHost)
	required("DB_USER", c.DBUser)
	required("DB_PASSWORD", c.DBPassword)
	required("JWT_SECRET", c.JWTSecret)
	if c.JWTSecret != "" && len(c.JWTSecret) < 32 {
		errs = append(errs, errors.New("JWT_SECRET must be at least 32 characters"))
	}
	if c.DBMaxIdleConns > c.DBMaxOpenConns {
		errs = append(errs, errors.New("DB_MAX_IDLE_CONNS must not exceed DB_MAX_OPEN_CONNS"))
	}
	if c.ProductMaxRetries < 0 || c.ProductMaxRetries > 5 {
		errs = append(errs, errors.New("PRODUCT_MAX_RETRIES must be between 0 and 5"))
	}
	if c.IsProduction() {
		required("PRODUCT_SERVICE_TOKEN", c.ProductServiceToken)
		if c.DBSSLMode == "disable" {
			errs = append(errs, errors.New("DB_SSLMODE must not be disable in production"))
		}
	}
	return errors.Join(errs...)
}

func loadSecrets(ctx context.Context, name string) (map[string]string, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	client := secretsmanager.NewFromConfig(awsCfg)
	out, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: aws.String(name)})
	if err != nil {
		return nil, err
	}
	if out.SecretString == nil {
		return nil, errors.New("secret has no string value")
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(*out.SecretString), &raw); err != nil {
		return nil, fmt.Errorf("secret must be a JSON object: %w", err)
	}
	aliases := map[string]string{
		"username": "DB_USER",
		"password": "DB_PASSWORD",
		"host":     "DB_HOST",
		"port":     "DB_PORT",
		"dbname":   "DB_NAME",
	}
	result := make(map[string]string, len(raw))
	for k, v := range raw {
		key := k
		if alias, ok := aliases[k]; ok {
			key = alias
		}
		result[key] = fmt.Sprint(v)
	}
	return result, nil
}
