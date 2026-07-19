package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/audit"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/auth"
	"go.uber.org/zap"
)

// AuditRecorder is the subset of audit.Service the middleware needs. Kept as an
// interface so the middleware can be tested without a database.
type AuditRecorder interface {
	Record(ctx context.Context, entry audit.Entry) (audit.Log, error)
}

// Audit records every successful mutating admin request (POST/PUT/PATCH/DELETE)
// in the audit log (BR-049). It runs after the handler so it can read the final
// status code and only logs 2xx responses — failed/rejected requests didn't
// change state and would just be noise.
//
// The actor is taken from the JWT claims set by Authenticate; the resource type
// and id are derived from the request path so no per-handler wiring is needed.
func Audit(recorder AuditRecorder, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if !isMutating(c.Request.Method) {
			return
		}
		status := c.Writer.Status()
		if status < 200 || status >= 300 {
			return
		}

		claims, actorID, actorEmail := actorFromContext(c)
		_ = claims
		resourceType, resourceID := resourceFromPath(c.Request.URL.Path)

		entry := audit.Entry{
			ActorUserID:  actorID,
			ActorEmail:   actorEmail,
			Action:       strings.ToLower(c.Request.Method) + "." + resourceType,
			ResourceType: resourceType,
			ResourceID:   resourceID,
			Metadata: map[string]any{
				"method":     c.Request.Method,
				"path":       c.Request.URL.Path,
				"status":     status,
				"request_id": c.GetString("request_id"),
			},
		}
		// Best-effort: an audit write failure must never affect the response the
		// user already received. Log it and move on.
		if _, err := recorder.Record(c.Request.Context(), entry); err != nil && log != nil {
			log.Warn("failed to record audit log",
				zap.String("path", c.Request.URL.Path),
				zap.Error(err))
		}
	}
}

func isMutating(method string) bool {
	switch method {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}

// actorFromContext pulls the authenticated admin's id + email from the JWT
// claims. Returns empty strings when the request is unauthenticated.
func actorFromContext(c *gin.Context) (auth.Claims, string, string) {
	value, exists := c.Get("auth_claims")
	if !exists {
		return auth.Claims{}, "", ""
	}
	claims, ok := value.(auth.Claims)
	if !ok {
		return auth.Claims{}, "", ""
	}
	return claims, claims.Subject, claims.Email
}

// resourceFromPath derives a resource type + id from a REST-style path. It strips
// the /api/v1 prefix and takes the first segment as the type and the following
// segment as the id when present. Examples:
//
//	/api/v1/products            -> ("products", "")
//	/api/v1/products/abc        -> ("products", "abc")
//	/api/v1/orders/abc/status   -> ("orders", "abc")
func resourceFromPath(path string) (string, string) {
	trimmed := strings.TrimPrefix(path, "/api/v1/")
	trimmed = strings.TrimPrefix(trimmed, "/")
	segments := strings.Split(trimmed, "/")
	if len(segments) == 0 || segments[0] == "" {
		return "unknown", ""
	}
	resourceType := segments[0]
	resourceID := ""
	if len(segments) >= 2 {
		resourceID = segments[1]
	}
	return resourceType, resourceID
}
