package container

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/auth"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/config"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/health"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/logger"
	"go.uber.org/zap"
)

type Container struct {
	Config        config.Config
	Logger        *zap.Logger
	Pool          *pgxpool.Pool
	HealthService *health.Service
	AuthService   *auth.Service
	TokenManager  *auth.TokenManager
}

func New() (*Container, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	log, err := logger.New(cfg.App.LogLevel, cfg.App.Env == "development")
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}
	pool, err := pgxpool.New(context.Background(), cfg.Database.DSN())
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	tokens, err := auth.NewTokenManager(cfg.Auth.JWTSecret, cfg.Auth.JWTIssuer, cfg.Auth.AccessTokenTTL)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create token manager: %w", err)
	}
	users := auth.NewPostgresRepository(pool)
	return &Container{Config: cfg, Logger: log, Pool: pool, HealthService: health.NewService(), TokenManager: tokens, AuthService: auth.NewService(users, tokens)}, nil
}

func (c *Container) Close() { c.Pool.Close() }
