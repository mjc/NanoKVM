package config

import (
	"strings"
	"testing"
)

func TestAuthDisabledLoginDoesNotReturnMagicToken(t *testing.T) {
	content := readSource(t, "../service/auth/login.go")
	if strings.Contains(content, `Token: "disabled"`) {
		t.Fatal("authentication:disable login should not mint a magic token value consumed like a real session")
	}
}
func TestAuthenticationDisableDoesNotEnablePermissiveCORS(t *testing.T) {
	content := readSource(t, "../main.go")
	if strings.Contains(content, `conf.Authentication == "disable"`) && strings.Contains(content, "cors.AllowAll()") {
		t.Fatal("authentication:disable should not automatically expose the API through permissive CORS")
	}
}
func TestCookieAuthenticatedStateChangesHaveCSRFProtection(t *testing.T) {
	middlewareSource := readSource(t, "../middleware/jwt.go")
	httpSource := readSource(t, "../../web/src/lib/http.ts")

	if strings.Contains(middlewareSource, `c.Cookie("nano-kvm-token")`) &&
		strings.Contains(httpSource, "withCredentials") &&
		!strings.Contains(middlewareSource, "CSRF") &&
		!strings.Contains(middlewareSource, "Origin") {
		t.Fatal("cookie-authenticated state-changing API calls should have CSRF/Origin protection")
	}
}
func TestAuthenticationDisableDoesNotBypassJWTMiddleware(t *testing.T) {
	content := readSource(t, "../middleware/jwt.go")
	if strings.Contains(content, `conf.Authentication == "disable"`) && strings.Contains(content, "return true") {
		t.Fatal("authentication:disable should not make JWT middleware allow every protected route")
	}
}
func TestUnauthenticatedAPWifiRoutesUseExplicitAuthMiddleware(t *testing.T) {
	content := readSource(t, "../router/network.go")
	if strings.Contains(content, `r.POST("/api/network/wifi", service.ConnectWifiNoAuth)`) ||
		strings.Contains(content, `r.POST("/api/network/wifi/verify", service.VerifyApLogin)`) {
		t.Fatal("AP Wi-Fi setup routes should be grouped behind explicit AP-auth/rate-limit middleware instead of raw unauthenticated registrations")
	}
}
func TestLoopbackInternalTokenComparisonAvoidsLengthOracle(t *testing.T) {
	content := readSource(t, "../middleware/loopback_http.go")
	if strings.Contains(content, "subtle.ConstantTimeCompare([]byte(provided), []byte(token))") &&
		!strings.Contains(content, "sha256") {
		t.Fatal("internal token comparison should avoid ConstantTimeCompare's length-dependent early return")
	}
}
func assertWebSocketPathDoesNotAllowAllOrigins(t *testing.T, path string) {
	t.Helper()

	source := readSource(t, path)
	if strings.Contains(source, "CheckOrigin: func") && strings.Contains(source, "return true") {
		t.Fatalf("%s websocket upgrader allows all origins", path)
	}
}
func TestWebSocketUpgradersDoNotAllowAllOrigins(t *testing.T) {
	paths := []string{
		"../service/ws/service.go",
		"../service/vm/terminal.go",
		"../service/stream/webrtc/h264.go",
		"../service/stream/direct/h264.go",
		"../service/picoclaw/gateway_proxy.go",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			assertWebSocketPathDoesNotAllowAllOrigins(t, path)
		})
	}
}
func TestTLSCannotBeDisabledThroughAuthenticatedWebUI(t *testing.T) {
	content := readSource(t, "../service/vm/tls.go")
	if strings.Contains(content, "disableTls()") || strings.Contains(content, `conf.Proto = "http"`) {
		t.Fatal("authenticated web UI should not provide a route that downgrades the control plane from HTTPS to HTTP")
	}
}
func TestAuthDisabledLoginStillParsesAndAuditsAttempt(t *testing.T) {
	content := readSource(t, "../service/auth/login.go")
	disabled := strings.Index(content, `conf.Authentication == "disable"`)
	parse := strings.Index(content, "proto.ParseFormRequest")
	clientIP := strings.Index(content, "GetClientIP")
	if disabled >= 0 && (parse < 0 || disabled < parse) {
		t.Fatal("authentication:disable login should still parse the request before returning a development-only response")
	}
	if disabled >= 0 && (clientIP < 0 || disabled < clientIP) {
		t.Fatal("authentication:disable login should still pass through the same audit/rate-limit accounting path")
	}
}
func TestLogoutAlwaysInvalidatesPresentedSession(t *testing.T) {
	content := readSource(t, "../service/auth/login.go")
	if strings.Contains(content, "if conf.JWT.RevokeTokensOnLogout") {
		t.Fatal("logout should invalidate the presented session unconditionally instead of being disabled by configuration")
	}
}
func TestLoginFailureMessagesDoNotDifferentiateMalformedAndBadCredentials(t *testing.T) {
	content := readSource(t, "../service/auth/login.go")
	if strings.Contains(content, `"invalid parameters"`) && strings.Contains(content, `"invalid username or password"`) {
		t.Fatal("login failures should use a uniform external error message for malformed and bad-credential attempts")
	}
}
func TestLoginFailuresUseUniformDelay(t *testing.T) {
	content := readSource(t, "../service/auth/login.go")
	if strings.Contains(content, "3 * time.Second") &&
		(strings.Contains(content, "2 * time.Second") || strings.Contains(content, "1 * time.Second")) {
		t.Fatal("login failure paths should use a uniform delay to avoid timing differences between malformed, bad-credential, and token-generation failures")
	}
}
