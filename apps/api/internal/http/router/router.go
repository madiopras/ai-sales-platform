package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/auth"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/health"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/http/middleware"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/response"
	"go.uber.org/zap"
)

func New(log *zap.Logger, healthHandler *health.Handler, authHandler *auth.Handler, tokens *auth.TokenManager) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(middleware.RequestID(), middleware.Recovery(log), middleware.Logging(log), middleware.ErrorHandler(log))

	router.GET("/health/live", healthHandler.Live)
	router.GET("/health/ready", healthHandler.Ready)
	router.POST("/auth/login", authHandler.Login)
	protected := router.Group("/auth")
	protected.Use(middleware.Authenticate(tokens))
	protected.GET("/me", authHandler.Me)
	router.NoRoute(func(c *gin.Context) { response.Fail(c, http.StatusNotFound, "NOT_FOUND", "route not found") })
	return router
}
