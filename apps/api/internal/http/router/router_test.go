package router_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prasdios/ai-sales-platform/apps/api/internal/auth"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/health"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/http/router"
	"go.uber.org/zap"
)

func TestLiveHealthCheck(t *testing.T) {
	tokens, err := auth.NewTokenManager("a-secure-development-secret-with-32-chars", "test", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	engine := router.New(zap.NewNop(), health.NewHandler(health.NewService()), auth.NewHandler(auth.NewService(fakeRepository{}, tokens)), tokens)
	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if recorder.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected request ID header")
	}
}

type fakeRepository struct{}

func (fakeRepository) FindByEmail(context.Context, string) (auth.User, error) {
	return auth.User{}, auth.ErrUserNotFound
}
func (fakeRepository) FindByID(context.Context, string) (auth.User, error) {
	return auth.User{}, auth.ErrUserNotFound
}
func (fakeRepository) Create(context.Context, auth.User) (auth.User, error) { return auth.User{}, nil }
