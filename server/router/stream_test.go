package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestStreamRouterRequiresAuthForH264(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	streamRouter(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/stream/h264", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized response, got %d", rec.Code)
	}
}

func TestStreamRouterRequiresAuthForDirectH264(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	streamRouter(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/stream/h264/direct", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized response, got %d", rec.Code)
	}
}
