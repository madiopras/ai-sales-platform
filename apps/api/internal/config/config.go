package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

type Config struct {
	App      AppConfig
	HTTP     HTTPConfig
	Database DatabaseConfig
	Redis    RedisConfig
	RabbitMQ RabbitMQConfig
	MinIO    MinIOConfig
	Auth     AuthConfig
}

type AppConfig struct {
	Name     string
	Env      string
	LogLevel string
}

type HTTPConfig struct {
	Host         string
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

func (h HTTPConfig) Address() string { return h.Host + ":" + h.Port }

type DatabaseConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
	SSLMode  string
}

type RedisConfig struct{ Address string }
type RabbitMQConfig struct{ URL string }
type MinIOConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

type AuthConfig struct {
	JWTSecret      string
	JWTIssuer      string
	AccessTokenTTL time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		App: AppConfig{Name: env("APP_NAME", "ai-sales-api"), Env: env("APP_ENV", "development"), LogLevel: env("LOG_LEVEL", "debug")},
		HTTP: HTTPConfig{
			Host: env("HTTP_HOST", "0.0.0.0"), Port: env("HTTP_PORT", "8081"),
			ReadTimeout: duration("HTTP_READ_TIMEOUT", 10*time.Second), WriteTimeout: duration("HTTP_WRITE_TIMEOUT", 15*time.Second), IdleTimeout: duration("HTTP_IDLE_TIMEOUT", 60*time.Second),
		},
		Database: DatabaseConfig{Host: env("POSTGRES_HOST", "localhost"), Port: env("POSTGRES_PORT", "5432"), Name: env("POSTGRES_DB", "ai_sales"), User: env("POSTGRES_USER", "postgres"), Password: os.Getenv("POSTGRES_PASSWORD"), SSLMode: env("POSTGRES_SSLMODE", "disable")},
		Redis:    RedisConfig{Address: env("REDIS_ADDR", "localhost:6379")},
		RabbitMQ: RabbitMQConfig{URL: env("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")},
		MinIO:    MinIOConfig{Endpoint: env("MINIO_ENDPOINT", "localhost:9000"), AccessKey: os.Getenv("MINIO_ROOT_USER"), SecretKey: os.Getenv("MINIO_ROOT_PASSWORD"), UseSSL: env("MINIO_USE_SSL", "false") == "true"},
		Auth:     AuthConfig{JWTSecret: os.Getenv("AUTH_JWT_SECRET"), JWTIssuer: env("AUTH_JWT_ISSUER", "ai-sales-platform"), AccessTokenTTL: duration("AUTH_ACCESS_TOKEN_TTL", 15*time.Minute)},
	}

	if !oneOf(cfg.App.Env, "development", "test", "staging", "production") {
		return Config{}, fmt.Errorf("invalid APP_ENV: %q", cfg.App.Env)
	}
	if len(cfg.Auth.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("AUTH_JWT_SECRET must contain at least 32 characters")
	}
	return cfg, nil
}

func (d DatabaseConfig) DSN() string {
	return (&url.URL{Scheme: "postgres", User: url.UserPassword(d.User, d.Password), Host: d.Host + ":" + d.Port, Path: d.Name, RawQuery: "sslmode=" + d.SSLMode}).String()
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func duration(key string, fallback time.Duration) time.Duration {
	value := env(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}
