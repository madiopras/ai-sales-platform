package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/audit"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/auth"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/catalog"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/customer"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/health"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/http/middleware"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/order"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/payment"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/response"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/promo"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/shipping"
	"go.uber.org/zap"
)

// Deps bundles everything the router needs. Grouping them in a struct keeps the
// constructor signature stable as new handlers/middleware are added.
type Deps struct {
	Logger           *zap.Logger
	Tokens           *auth.TokenManager
	AuditService     *audit.Service
	ServiceToken     string
	WebhookRateLimit *middleware.RateLimit
	AllowedOrigins   []string

	Health   *health.Handler
	Auth     *auth.Handler
	Catalog  *catalog.Handler
	Customer *customer.Handler
	Order    *order.Handler
	Payment  *payment.Handler
	Shipping *shipping.Handler
	Promo    *promo.Handler
	Audit    *audit.Handler
}

func New(d Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(middleware.RequestID(), middleware.Recovery(d.Logger), middleware.Logging(d.Logger), middleware.ErrorHandler(d.Logger), middleware.CORS(d.AllowedOrigins))

	// Health endpoints (unchanged)
	router.GET("/health/live", d.Health.Live)
	router.GET("/health/ready", d.Health.Ready)

	// Auth endpoints (unchanged)
	router.POST("/auth/login", d.Auth.Login)
	protected := router.Group("/auth")
	protected.Use(middleware.Authenticate(d.Tokens))
	protected.GET("/me", d.Auth.Me)

	// API v1 group (admin endpoints with RBAC). The audit middleware records every
	// successful mutating request in this group (BR-049).
	apiV1 := router.Group("/api/v1")
	apiV1.Use(middleware.Authenticate(d.Tokens))
	apiV1.Use(middleware.Authorize("admin", "manager"))
	if d.AuditService != nil {
		apiV1.Use(middleware.Audit(d.AuditService, d.Logger))
	}

	// Catalog admin routes
	apiV1.POST("/categories", d.Catalog.CreateCategory)
	apiV1.GET("/categories", d.Catalog.ListCategories)
	apiV1.GET("/categories/:id", d.Catalog.GetCategory)
	apiV1.PUT("/categories/:id", d.Catalog.UpdateCategory)
	apiV1.DELETE("/categories/:id", d.Catalog.DeleteCategory)

	apiV1.POST("/products", d.Catalog.CreateProduct)
	apiV1.GET("/products", d.Catalog.ListProducts)
	apiV1.GET("/products/:id", d.Catalog.GetProduct)
	apiV1.PUT("/products/:id", d.Catalog.UpdateProduct)
	apiV1.DELETE("/products/:id", d.Catalog.DeleteProduct)

	apiV1.POST("/products/:id/variants", d.Catalog.CreateVariant)
	apiV1.GET("/products/:id/variants", d.Catalog.ListVariantsByProduct)
	apiV1.GET("/products/:id/variants/:variant_id", d.Catalog.GetVariant)
	apiV1.PUT("/products/:id/variants/:variant_id", d.Catalog.UpdateVariant)
	apiV1.DELETE("/products/:id/variants/:variant_id", d.Catalog.DeleteVariant)

	apiV1.PUT("/variants/:id/stock", d.Catalog.UpdateStock)

	// Customer admin routes (read-only)
	apiV1.GET("/customers", d.Customer.ListCustomers)
	apiV1.GET("/customers/:id", d.Customer.GetCustomer)
	apiV1.GET("/customers/:id/addresses", d.Customer.ListAddresses)

	// Order admin routes (list/get + fulfillment status transitions BR-026)
	apiV1.GET("/orders", d.Order.ListOrders)
	apiV1.GET("/orders/:id", d.Order.GetOrder)
	apiV1.PUT("/orders/:id/status", d.Order.UpdateStatus)

	// Payment admin routes (read invoices)
	apiV1.GET("/invoices/:id", d.Payment.GetInvoice)
	apiV1.GET("/orders/:id/invoice", d.Payment.GetInvoiceByOrder)

	// Shipping admin routes (book shipment BR-028, read tracking)
	apiV1.POST("/shipments", d.Shipping.CreateShipment)
	apiV1.GET("/shipments/:id", d.Shipping.GetShipment)
	apiV1.GET("/orders/:id/shipment", d.Shipping.GetShipmentByOrder)

	// Promo/voucher admin routes (BR-048)
	apiV1.POST("/vouchers", d.Promo.CreateVoucher)
	apiV1.GET("/vouchers", d.Promo.ListVouchers)
	apiV1.GET("/vouchers/:id", d.Promo.GetVoucher)
	apiV1.PUT("/vouchers/:id", d.Promo.UpdateVoucher)
	apiV1.DELETE("/vouchers/:id", d.Promo.DeleteVoucher)

	// Audit log review (BR-049) — read-only.
	apiV1.GET("/audit-logs", d.Audit.ListLogs)
	apiV1.GET("/audit-logs/:id", d.Audit.GetLog)

	// Internal AI endpoints group, guarded by a shared service token. apps/ai is a
	// trusted backend, not an end user, so it authenticates with a static token
	// rather than a per-user JWT.
	internalV1 := router.Group("/internal/v1")
	internalV1.Use(middleware.ServiceToken(d.ServiceToken))

	// Internal search & read API
	internalV1.GET("/products/search", d.Catalog.Search)
	internalV1.GET("/products/:id", d.Catalog.GetByID)
	internalV1.GET("/products/slug/:slug", d.Catalog.GetBySlug)

	// Internal customer & cart API for apps/ai
	internalV1.POST("/customers", d.Customer.UpsertCustomer)
	internalV1.GET("/customers/:id", d.Customer.GetCustomer)
	internalV1.POST("/customers/:id/addresses", d.Customer.CreateAddress)
	internalV1.GET("/customers/:id/addresses/default", d.Customer.GetDefaultAddress)
	internalV1.GET("/customers/:id/cart", d.Customer.GetOpenCart)
	internalV1.POST("/customers/:id/cart/items", d.Customer.AddCartItem)
	internalV1.PUT("/customers/:id/cart/items/:variantId", d.Customer.UpdateCartItem)
	internalV1.DELETE("/customers/:id/cart/items/:variantId", d.Customer.RemoveCartItem)

	// Internal checkout API for apps/ai (preview summary + confirm order)
	internalV1.POST("/checkout/preview", d.Order.PreviewCheckout)
	internalV1.POST("/checkout", d.Order.Checkout)
	internalV1.GET("/orders/:id", d.Order.GetOrder)

	// Internal payment API for apps/ai (create invoice + read status BR-016..BR-021)
	internalV1.POST("/payments/invoices", d.Payment.CreateInvoice)
	internalV1.GET("/payments/invoices/:id", d.Payment.GetInvoice)
	internalV1.GET("/orders/:id/invoice", d.Payment.GetInvoiceByOrder)

	// Internal shipping API for apps/ai (rate quote BR-010..BR-011, tracking BR-032)
	internalV1.POST("/shipping/rates", d.Shipping.GetRates)
	internalV1.GET("/orders/:id/shipment", d.Shipping.GetShipmentByOrder)

	// Internal voucher validation for apps/ai (BR-037..BR-038)
	internalV1.POST("/vouchers/validate", d.Promo.ValidateVoucher)

	// Webhooks — token verified inside the handler/service (BR-042/BR-043). Kept
	// outside the JWT-protected groups since the vendors authenticate via their own
	// header, not a bearer token. A per-IP rate limiter guards these
	// unauthenticated, internet-facing endpoints from abuse.
	webhooks := router.Group("/webhooks")
	if d.WebhookRateLimit != nil {
		webhooks.Use(d.WebhookRateLimit.Handler())
	}
	webhooks.POST("/xendit", d.Payment.XenditWebhook)
	webhooks.POST("/biteship", d.Shipping.BiteshipWebhook)

	router.NoRoute(func(c *gin.Context) { response.Fail(c, http.StatusNotFound, "NOT_FOUND", "route not found") })
	return router
}
