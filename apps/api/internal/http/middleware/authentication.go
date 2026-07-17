package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/auth"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/response"
)

func Authenticate(tokens *auth.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		parts := strings.Fields(header)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication is required")
			return
		}
		claims, err := tokens.Parse(parts[1])
		if err != nil {
			response.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired access token")
			return
		}
		c.Set("auth_claims", claims)
		c.Next()
	}
}
