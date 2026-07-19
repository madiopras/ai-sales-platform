package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/response"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) Live(c *gin.Context)     { response.OK(c, h.service.Live()) }
func (h *Handler) Ready(c *gin.Context) {
	status := h.service.Ready(c.Request.Context())
	if status.Status == "unavailable" {
		response.Fail(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "service unavailable")
		return
	}
	response.OK(c, status)
}
