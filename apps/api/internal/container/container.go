package container

import (
	"fmt"

	"github.com/prasdios/ai-sales-platform/apps/api/internal/config"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/health"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/logger"
	"go.uber.org/zap"
)

type Container struct {
	Config        config.Config
	Logger        *zap.Logger
	HealthService *health.Service
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
	return &Container{Config: cfg, Logger: log, HealthService: health.NewService()}, nil
}
