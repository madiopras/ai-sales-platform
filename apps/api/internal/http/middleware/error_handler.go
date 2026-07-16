package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/apperror"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/response"
	"go.uber.org/zap"
)

func ErrorHandler(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 || c.IsAborted() {
			return
		}

		last := c.Errors.Last().Err
		var appErr *apperror.Error
		if errors.As(last, &appErr) {
			response.Fail(c, appErr.Status, appErr.Code, appErr.Message)
			return
		}

		log.Error("unhandled request error", zap.Error(last), zap.String("request_id", c.GetString("request_id")))
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}
