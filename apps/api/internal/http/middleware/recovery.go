package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/response"
	"go.uber.org/zap"
)

func Recovery(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error("panic recovered", zap.Any("error", recovered), zap.String("request_id", c.GetString("request_id")))
				response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
			}
		}()
		c.Next()
	}
}
