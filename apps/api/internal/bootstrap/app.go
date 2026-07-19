package bootstrap

import (
	"context"
	"errors"
	"net/http"

	"github.com/prasdios/ai-sales-platform/apps/api/internal/audit"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/auth"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/catalog"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/container"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/customer"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/health"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/http/middleware"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/http/router"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/order"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/payment"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/promo"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/shipping"
	"go.uber.org/zap"
)

var ErrServerClosed = http.ErrServerClosed

type App struct {
	Logger *zap.Logger
	server *http.Server
	close  func()
}

func New() (*App, error) {
	ctr, err := container.New()
	if err != nil {
		return nil, err
	}

	deps := router.Deps{
		Logger:           ctr.Logger,
		Tokens:           ctr.TokenManager,
		AuditService:     ctr.AuditService,
		ServiceToken:     ctr.Config.Internal.ServiceToken,
		WebhookRateLimit: middleware.NewRateLimit(ctr.Config.Internal.WebhookRatePerSec, ctr.Config.Internal.WebhookRateBurst),

		Health:   health.NewHandler(ctr.HealthService),
		Auth:     auth.NewHandler(ctr.AuthService),
		Catalog:  catalog.NewHandler(ctr.CatalogService),
		Customer: customer.NewHandler(ctr.CustomerService),
		Order:    order.NewHandler(ctr.OrderService),
		Payment:  payment.NewHandler(ctr.PaymentService),
		Shipping: shipping.NewHandler(ctr.ShippingService),
		Promo:    promo.NewHandler(ctr.PromoService),
		Audit:    audit.NewHandler(ctr.AuditService),
	}

	server := &http.Server{
		Addr:    ctr.Config.HTTP.Address(),
		Handler: router.New(deps),

		ReadTimeout:  ctr.Config.HTTP.ReadTimeout,
		WriteTimeout: ctr.Config.HTTP.WriteTimeout,
		IdleTimeout:  ctr.Config.HTTP.IdleTimeout,
	}
	return &App{Logger: ctr.Logger, server: server, close: ctr.Close}, nil
}

func (a *App) Run() error                         { return a.server.ListenAndServe() }
func (a *App) Shutdown(ctx context.Context) error { return a.server.Shutdown(ctx) }
func (a *App) Close() error {
	a.close()
	return errors.Join(a.Logger.Sync())
}
