package webrtc

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type recordingTransport struct {
	t *testing.T
}

func (rt recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if got := req.URL.Scheme; got != "http" {
		rt.t.Fatalf("unexpected scheme %q", got)
	}
	if got := req.URL.Host; got != "127.0.0.1:6040" {
		rt.t.Fatalf("unexpected host %q", got)
	}
	if got := req.URL.Path; got != "/api/stream/h264" {
		rt.t.Fatalf("unexpected path %q", got)
	}
	if got := req.Host; got != "127.0.0.1:6040" {
		rt.t.Fatalf("unexpected host header %q", got)
	}

	return &http.Response{
		StatusCode: http.StatusNoContent,
		Status:     "204 No Content",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("")),
		Request:    req,
	}, nil
}

func TestConnectProxiesToFixedRustSidecarTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalTransport := http.DefaultTransport
	http.DefaultTransport = recordingTransport{t: t}
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	engine := gin.New()
	engine.GET("/api/stream/h264", Connect)

	req := httptest.NewRequest(http.MethodGet, "/api/stream/h264?foo=bar", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected proxied response, got %d", rec.Code)
	}
}
