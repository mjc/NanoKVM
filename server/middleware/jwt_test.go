package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"NanoKVM-Server/config"
	"github.com/gin-gonic/gin"
)

func withTestConfig(t *testing.T, conf *config.Config) {
	t.Helper()

	old := getConfig
	getConfig = func() *config.Config {
		return conf
	}
	t.Cleanup(func() {
		getConfig = old
	})
}

func testAuthConfig() *config.Config {
	return &config.Config{
		Authentication: "enable",
		JWT: config.JWT{
			SecretKey:            "test-secret",
			RefreshTokenDuration: 3600,
		},
	}
}

func TestGenerateAndParseJWT(t *testing.T) {
	withTestConfig(t, testAuthConfig())

	token, err := GenerateJWT("alice")
	if err != nil {
		t.Fatal(err)
	}

	claims, err := ParseJWT(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Username != "alice" {
		t.Fatalf("username = %q", claims.Username)
	}

	if _, err := ParseJWT(token + "broken"); err == nil {
		t.Fatal("expected invalid token error")
	}
}

func TestCheckTokenMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withTestConfig(t, testAuthConfig())

	token, err := GenerateJWT("alice")
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name       string
		cookie     string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "valid",
			cookie:     token,
			wantStatus: http.StatusOK,
			wantBody:   "ok",
		},
		{
			name:       "missing",
			wantStatus: http.StatusUnauthorized,
			wantBody:   `"unauthorized"`,
		},
		{
			name:       "invalid",
			cookie:     "not-a-token",
			wantStatus: http.StatusUnauthorized,
			wantBody:   `"unauthorized"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/protected", CheckToken(), func(c *gin.Context) {
				c.String(http.StatusOK, "ok")
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "nano-kvm-token", Value: tc.cookie})
			}

			r.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d", w.Code)
			}
			if w.Body.String() != tc.wantBody {
				t.Fatalf("body = %q", w.Body.String())
			}
		})
	}
}

func TestCheckTokenAllowsDisabledAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	conf := testAuthConfig()
	conf.Authentication = "disable"
	withTestConfig(t, conf)

	r := gin.New()
	r.GET("/protected", CheckToken(), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Fatalf("body = %q", w.Body.String())
	}
}
