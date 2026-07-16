package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prasdios/ai-sales-platform/apps/api/internal/health"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/http/router"
	"go.uber.org/zap"
)

func TestLiveHealthCheck(t *testing.T) {
	engine := router.New(zap.NewNop(), health.NewHandler(health.NewService()))
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
