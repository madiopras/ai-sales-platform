package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/response"
)

// ServiceToken guards the internal (apps/ai) API surface. apps/ai is a trusted
// backend service, not an end user, so it authenticates with a shared static
// token rather than a per-user JWT. The token is sent in the `X-Service-Token`
// header (or `Authorization: Bearer <token>`).
//
// Fail-closed: when no token is configured the middleware rejects every request
// so a misconfigured deployment can't accidentally expose the internal API
// unauthenticated.
func ServiceToken(expected string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.TrimSpace(expected) == "" {
			response.Fail(c, http.StatusServiceUnavailable, "SERVICE_TOKEN_NOT_CONFIGURED",
				"internal API is not configured")
			return
		}
		received := c.GetHeader("X-Service-Token")
		if received == "" {
			// Also accept a bearer token so callers can reuse HTTP auth plumbing.
			if parts := strings.Fields(c.GetHeader("Authorization")); len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				received = parts[1]
			}
		}
		if subtle.ConstantTimeCompare([]byte(received), []byte(expected)) != 1 {
			response.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid service token")
			return
		}
		c.Next()
	}
}
