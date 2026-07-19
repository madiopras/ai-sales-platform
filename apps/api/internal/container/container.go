package container

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/audit"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/auth"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/catalog"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/config"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/customer"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/events"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/health"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/order"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/payment"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/payment/xendit"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/logger"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/promo"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/shipping"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/shipping/biteship"

	"github.com/redis/go-redis/v9"

	"go.uber.org/zap"
)

type Container struct {
	Config          config.Config
	Logger          *zap.Logger
	Pool            *pgxpool.Pool
	Redis           *redis.Client
	HealthService   *health.Service
	AuthService     *auth.Service
	TokenManager    *auth.TokenManager
	CatalogService  *catalog.Service
	CustomerService *customer.Service
	OrderService    *order.Service
	PaymentService  *payment.Service
	ShippingService *shipping.Service
	PromoService    *promo.Service
	AuditService    *audit.Service
}

// voucherAdapter bridges *promo.Service to the order package's VoucherValidator
// interface so the order package doesn't import promo directly. It maps
// promo.ApplyResult onto order.VoucherResult.
type voucherAdapter struct{ promo *promo.Service }

func (a voucherAdapter) Validate(ctx context.Context, code string, subtotal float64) (order.VoucherResult, error) {
	result, err := a.promo.Validate(ctx, code, subtotal)
	if err != nil {
		return order.VoucherResult{}, err
	}
	return order.VoucherResult{Code: result.Code, Discount: result.Discount}, nil
}

func (a voucherAdapter) Redeem(ctx context.Context, code string, subtotal float64) (order.VoucherResult, error) {
	result, err := a.promo.Redeem(ctx, code, subtotal)
	if err != nil {
		return order.VoucherResult{}, err
	}
	return order.VoucherResult{Code: result.Code, Discount: result.Discount}, nil
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

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.Redis.Address,
	})
	// Verify Redis connection
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Warn("Redis connection failed, continuing without Redis", zap.Error(err))
	}

	tokens, err := auth.NewTokenManager(cfg.Auth.JWTSecret, cfg.Auth.JWTIssuer, cfg.Auth.AccessTokenTTL)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create token manager: %w", err)
	}
	users := auth.NewPostgresRepository(pool)

	// Initialize catalog service
	catalogRepo := catalog.NewPostgresRepository(pool)
	catalogService := catalog.NewService(catalogRepo)

	// Initialize customer service (depends on catalog repo for stock validation)
	customerRepo := customer.NewPostgresRepository(pool)
	customerService := customer.NewService(customerRepo, catalogRepo)

	// Initialize promo service (voucher CRUD + checkout validation/redemption).
	promoService := promo.NewService(promo.NewPostgresRepository(pool))

	// Initialize audit service (records admin mutations, BR-049).
	auditService := audit.NewService(audit.NewPostgresRepository(pool))

	// Domain-event publisher for apps/ai notifications (BR-041). Log-backed by
	// default; swap for a RabbitMQ publisher in production.
	eventPublisher := events.NewLogPublisher(log)

	// Initialize order service (checkout reads cart + catalog, reserves stock)
	orderRepo := order.NewPostgresRepository(pool)
	orderService := order.NewService(orderRepo, customerService, catalogRepo)
	orderService.SetVoucherValidator(voucherAdapter{promo: promoService})

	// Initialize payment service (creates Xendit invoices, handles webhooks,
	// commits/releases stock on paid/expired).
	paymentRepo := payment.NewPostgresRepository(pool)
	xenditClient := xendit.NewClient(xendit.Config{
		BaseURL:   cfg.Xendit.BaseURL,
		SecretKey: cfg.Xendit.SecretKey,
		Timeout:   cfg.Xendit.HTTPTimeout,
	})
	paymentService := payment.NewService(paymentRepo, xenditClient, payment.Config{
		CallbackToken:      cfg.Xendit.CallbackToken,
		InvoiceDuration:    cfg.Xendit.InvoiceDuration,
		SuccessRedirectURL: cfg.Xendit.SuccessRedirectURL,
		FailureRedirectURL: cfg.Xendit.FailureRedirectURL,
	})
	paymentService.SetEventPublisher(eventPublisher)

	// Initialize shipping service (Biteship rates + shipments, handles tracking
	// webhooks that advance the order to shipped/delivered).
	shippingRepo := shipping.NewPostgresRepository(pool)
	biteshipClient := biteship.NewClient(biteship.Config{
		BaseURL: cfg.Biteship.BaseURL,
		APIKey:  cfg.Biteship.APIKey,
		Timeout: cfg.Biteship.HTTPTimeout,
	})
	shippingService := shipping.NewService(shippingRepo, biteshipClient, shipping.Config{
		WebhookToken:       cfg.Biteship.WebhookToken,
		DefaultCouriers:    cfg.Biteship.DefaultCouriers,
		DefaultWeight:      cfg.Biteship.DefaultWeight,
		OriginPostalCode:   cfg.Biteship.OriginPostalCode,
		OriginContactName:  cfg.Biteship.OriginContactName,
		OriginContactPhone: cfg.Biteship.OriginContactPhone,
		OriginAddress:      cfg.Biteship.OriginAddress,
		OriginNote:         cfg.Biteship.OriginNote,
	})
	shippingService.SetEventPublisher(eventPublisher)

	return &Container{
		Config:          cfg,
		Logger:          log,
		Pool:            pool,
		Redis:           redisClient,
		HealthService:   health.NewService(pool),
		TokenManager:    tokens,
		AuthService:     auth.NewService(users, tokens),
		CatalogService:  catalogService,
		CustomerService: customerService,
		OrderService:    orderService,
		PaymentService:  paymentService,
		ShippingService: shippingService,
		PromoService:    promoService,
		AuditService:    auditService,
	}, nil

}

func (c *Container) Close() {
	c.Pool.Close()
	if c.Redis != nil {
		c.Redis.Close()
	}
}
