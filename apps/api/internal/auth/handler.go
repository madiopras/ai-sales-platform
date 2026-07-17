package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/response"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type userResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

func (h *Handler) Login(c *gin.Context) {
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "email and password are required")
		return
	}
	result, err := h.service.Login(c.Request.Context(), request.Email, request.Password)
	if errors.Is(err, ErrInvalidCredentials) {
		response.Fail(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email or password")
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, gin.H{"access_token": result.AccessToken, "token_type": "Bearer", "expires_at": result.ExpiresAt.UTC().Format(time.RFC3339), "user": presentUser(result.User)})
}

func (h *Handler) Me(c *gin.Context) {
	claims, ok := ClaimsFromContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication is required")
		return
	}
	user, err := h.service.CurrentUser(c.Request.Context(), claims.Subject)
	if errors.Is(err, ErrUserNotFound) || !user.IsActive {
		response.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication is required")
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, presentUser(user))
}

func presentUser(user User) userResponse {
	return userResponse{ID: user.ID, Email: user.Email, Name: user.Name, Role: user.Role}
}

const claimsContextKey = "auth_claims"

func ClaimsFromContext(c *gin.Context) (Claims, bool) {
	value, ok := c.Get(claimsContextKey)
	claims, valid := value.(Claims)
	return claims, ok && valid
}
