package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/health"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/http/middleware"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/response"
	"go.uber.org/zap"
)

func New(log *zap.Logger, healthHandler *health.Handler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(middleware.RequestID(), middleware.Recovery(log), middleware.Logging(log), middleware.ErrorHandler(log))

	router.GET("/health/live", healthHandler.Live)
	router.GET("/health/ready", healthHandler.Ready)
	router.NoRoute(func(c *gin.Context) { response.Fail(c, http.StatusNotFound, "NOT_FOUND", "route not found") })
	return router
}
