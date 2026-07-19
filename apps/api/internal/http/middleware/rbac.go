package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/auth"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/response"
)

func Authorize(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get("auth_claims")
		if !exists {
			response.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication is required")
			c.Abort()
			return
		}

		userClaims, ok := claims.(auth.Claims)
		if !ok {
			response.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid claims")
			c.Abort()
			return
		}

		// Check if user role is in allowed roles
		roleAllowed := false
		for _, role := range allowedRoles {
			if strings.EqualFold(userClaims.Role, role) {
				roleAllowed = true
				break
			}
		}

		if !roleAllowed {
			response.Fail(c, http.StatusForbidden, "FORBIDDEN", "insufficient permissions")
			c.Abort()
			return
		}

		c.Next()
	}
}
