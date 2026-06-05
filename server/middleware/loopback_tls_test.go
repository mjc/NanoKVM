package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"NanoKVM-Server/config"
	"github.com/gin-gonic/gin"
)

func withInternalToken(t *testing.T, token string, err error) {
	t.Helper()

	old := getPicoclawInternalToken
	getPicoclawInternalToken = func() (string, error) {
		return token, err
	}
	t.Cleanup(func() {
		getPicoclawInternalToken = old
	})
}

func TestRedirectHost(t *testing.T) {
	for _, tc := range []struct {
		host      string
		httpsPort string
		want      string
	}{
		{host: "nanokvm.local:80", httpsPort: "443", want: "nanokvm.local"},
		{host: "nanokvm.local:80", httpsPort: "8443", want: "nanokvm.local:8443"},
		{host: "[::1]:80", httpsPort: "443", want: "[::1]"},
		{host: "[::1]", httpsPort: "8443", want: "[::1]:8443"},
		{host: "::1", httpsPort: "443", want: "[::1]"},
	} {
		t.Run(tc.host+"-"+tc.httpsPort, func(t *testing.T) {
			if got := redirectHost(tc.host, tc.httpsPort); got != tc.want {
				t.Fatalf("redirectHost = %q", got)
			}
		})
	}
}

func TestLoopbackTokenHelpers(t *testing.T) {
	withInternalToken(t, "secret", nil)

	req := httptest.NewRequest(http.MethodGet, "/allowed", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set(config.PicoclawInternalTokenHeader, "secret")

	if !allowByLoopbackInternalToken(req) {
		t.Fatal("loopback token was not allowed")
	}
	if !isLoopbackAllowedPath(req, map[string]struct{}{"/allowed": {}}) {
		t.Fatal("allowed path was rejected")
	}
	if isLoopbackAllowedPath(req, map[string]struct{}{"/other": {}}) {
		t.Fatal("unlisted path was allowed")
	}
	if isLoopbackAllowedPath(nil, map[string]struct{}{"/allowed": {}}) {
		t.Fatal("nil request path was allowed")
	}

	req.RemoteAddr = "192.0.2.1:1234"
	if allowByLoopbackInternalToken(req) {
		t.Fatal("non-loopback remote was allowed")
	}

	if allowByLoopbackInternalToken(nil) {
		t.Fatal("nil request was allowed")
	}

	withInternalToken(t, "", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	if hasValidLoopbackHTTPToken(req) {
		t.Fatal("empty configured token was allowed")
	}

	withInternalToken(t, "", errors.New("token failed"))
	if hasValidLoopbackHTTPToken(req) {
		t.Fatal("token lookup failure was allowed")
	}
	if hasValidLoopbackHTTPToken(nil) {
		t.Fatal("nil request token was allowed")
	}
}

func TestIsLoopbackRemote(t *testing.T) {
	for _, remote := range []string{"127.0.0.1:1234", "[::1]:1234", "::1", "localhost"} {
		got := isLoopbackRemote(remote)
		want := remote != "localhost"
		if got != want {
			t.Fatalf("%q loopback = %t", remote, got)
		}
	}
}

func TestCheckTokenOrLoopbackInternalToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withTestConfig(t, testAuthConfig())
	withInternalToken(t, "internal", nil)

	r := gin.New()
	r.GET("/protected", CheckTokenOrLoopbackInternalToken(), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set(config.PicoclawInternalTokenHeader, "internal")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Body.String() != "ok" {
		t.Fatalf("loopback token response = %d %q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", w.Code)
	}
}

func TestCheckLoopbackInternalToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withInternalToken(t, "internal", nil)

	r := gin.New()
	r.GET("/internal", CheckLoopbackInternalToken(), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/internal", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set(config.PicoclawInternalTokenHeader, "internal")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/internal", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", w.Code)
	}
}

func TestTlsMiddlewareRedirectsHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/", Tls(), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://nanokvm.local/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d", w.Code)
	}
	if got := w.Header().Get("Location"); got != "https://nanokvm.local/" {
		t.Fatalf("location = %q", got)
	}
}

func TestTlsMiddlewareAllowsHTTPS(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/", Tls(), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://nanokvm.local/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Fatalf("body = %q", w.Body.String())
	}
}
