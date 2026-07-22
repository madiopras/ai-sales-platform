package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
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
	Xendit   XenditConfig
	Biteship BiteshipConfig
	Internal InternalConfig
	CORS     CORSConfig
}

// CORSConfig holds allowed origins for CORS middleware
type CORSConfig struct {
	AllowedOrigins []string
}

// InternalConfig holds settings for the internal (apps/ai) API surface and the
// webhook rate limiter (Phase 10). ServiceToken authenticates apps/ai; the
// webhook limiter caps unauthenticated callback traffic per client IP.
type InternalConfig struct {
	ServiceToken      string
	WebhookRatePerSec float64
	WebhookRateBurst  int
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

// XenditConfig holds credentials + defaults for the Xendit Invoice API integration.
// SecretKey is used for HTTP Basic auth (SECRET_KEY:) and CallbackToken is compared
// against the `x-callback-token` header on incoming webhooks (BR-042).
type XenditConfig struct {
	SecretKey          string
	PublicKey          string
	CallbackToken      string
	BaseURL            string
	InvoiceDuration    time.Duration
	SuccessRedirectURL string
	FailureRedirectURL string
	HTTPTimeout        time.Duration
}

// BiteshipConfig holds credentials + defaults for the Biteship shipping API
// integration. APIKey authenticates outbound calls (raw Authorization header)
// and WebhookToken is compared against the incoming webhook auth header (BR-043).
type BiteshipConfig struct {
	APIKey       string
	WebhookToken string
	BaseURL      string
	HTTPTimeout  time.Duration

	// Rate/booking defaults.
	DefaultCouriers  string
	DefaultWeight    int
	OriginPostalCode int

	// Origin (warehouse) shipper details used when booking a shipment.
	OriginContactName  string
	OriginContactPhone string
	OriginAddress      string
	OriginNote         string

	// AutoCompleteAfter: how long an order stays `delivered` before the worker
	// auto-completes it (BR-034).
	AutoCompleteAfter time.Duration
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
		Xendit: XenditConfig{
			SecretKey:     os.Getenv("XENDIT_SECRET_KEY"),
			PublicKey:     os.Getenv("XENDIT_PUBLIC_KEY"),
			CallbackToken: os.Getenv("XENDIT_CALLBACK_TOKEN"),

			BaseURL:            env("XENDIT_BASE_URL", "https://api.xendit.co"),
			InvoiceDuration:    duration("XENDIT_INVOICE_DURATION", 24*time.Hour),
			SuccessRedirectURL: os.Getenv("XENDIT_SUCCESS_REDIRECT_URL"),
			FailureRedirectURL: os.Getenv("XENDIT_FAILURE_REDIRECT_URL"),
			HTTPTimeout:        duration("XENDIT_HTTP_TIMEOUT", 15*time.Second),
		},
		Biteship: BiteshipConfig{
			APIKey:       os.Getenv("BITESHIP_API_KEY"),
			WebhookToken: os.Getenv("BITESHIP_WEBHOOK_TOKEN"),
			BaseURL:      env("BITESHIP_BASE_URL", "https://api.biteship.com"),
			HTTPTimeout:  duration("BITESHIP_HTTP_TIMEOUT", 15*time.Second),

			DefaultCouriers:  env("BITESHIP_DEFAULT_COURIERS", "jne,jnt,sicepat,anteraja"),
			DefaultWeight:    intEnv("BITESHIP_DEFAULT_WEIGHT", 1000),
			OriginPostalCode: intEnv("BITESHIP_ORIGIN_POSTAL_CODE", 0),

			OriginContactName:  os.Getenv("BITESHIP_ORIGIN_CONTACT_NAME"),
			OriginContactPhone: os.Getenv("BITESHIP_ORIGIN_CONTACT_PHONE"),
			OriginAddress:      os.Getenv("BITESHIP_ORIGIN_ADDRESS"),
			OriginNote:         os.Getenv("BITESHIP_ORIGIN_NOTE"),

			AutoCompleteAfter: duration("BITESHIP_AUTOCOMPLETE_AFTER", 72*time.Hour),
		},
		Internal: InternalConfig{
			ServiceToken:      os.Getenv("INTERNAL_SERVICE_TOKEN"),
			WebhookRatePerSec: floatEnv("WEBHOOK_RATE_PER_SEC", 5),
			WebhookRateBurst:  intEnv("WEBHOOK_RATE_BURST", 10),
		},
		CORS: CORSConfig{
			AllowedOrigins: parseAllowedOrigins(env("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:3001")),
		},
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

func intEnv(key string, fallback int) int {
	value := env(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func floatEnv(key string, fallback float64) float64 {
	value := env(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
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

func parseAllowedOrigins(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	var origins []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}
