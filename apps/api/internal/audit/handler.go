package audit

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/response"
)

// Handler exposes read access to audit logs for the admin review UI (BR-049).
type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

// ListLogs returns audit logs with optional resource_type/resource_id/actor filters.
func (h *Handler) ListLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	logs, err := h.service.List(c.Request.Context(), ListFilter{
		ResourceType: c.Query("resource_type"),
		ResourceID:   c.Query("resource_id"),
		ActorUserID:  c.Query("actor_user_id"),
		Limit:        limit,
		Offset:       offset,
	})
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, gin.H{"logs": logs})
}

// GetLog returns a single audit log by id.
func (h *Handler) GetLog(c *gin.Context) {
	log, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		response.Fail(c, http.StatusNotFound, "AUDIT_LOG_NOT_FOUND", "audit log not found")
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, log)
}
