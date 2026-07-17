package bootstrap

import (
	"context"
	"errors"
	"net/http"

	"github.com/prasdios/ai-sales-platform/apps/api/internal/auth"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/container"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/health"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/http/router"
	"go.uber.org/zap"
)

var ErrServerClosed = http.ErrServerClosed

type App struct {
	Logger *zap.Logger
	server *http.Server
	close  func()
}

func New() (*App, error) {
	container, err := container.New()
	if err != nil {
		return nil, err
	}

	handler := health.NewHandler(container.HealthService)
	authHandler := auth.NewHandler(container.AuthService)
	server := &http.Server{
		Addr: container.Config.HTTP.Address(), Handler: router.New(container.Logger, handler, authHandler, container.TokenManager),
		ReadTimeout: container.Config.HTTP.ReadTimeout, WriteTimeout: container.Config.HTTP.WriteTimeout, IdleTimeout: container.Config.HTTP.IdleTimeout,
	}
	return &App{Logger: container.Logger, server: server, close: container.Close}, nil
}

func (a *App) Run() error                         { return a.server.ListenAndServe() }
func (a *App) Shutdown(ctx context.Context) error { return a.server.Shutdown(ctx) }
func (a *App) Close() error {
	a.close()
	return errors.Join(a.Logger.Sync())
}
