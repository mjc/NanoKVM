package config

import (
	"strings"
	"testing"
)

func TestPasswordPayloadEncryptionDoesNotUseHardcodedSharedKey(t *testing.T) {
	serverContent := readSource(t, "../utils/encrypt.go")
	clientContent := readSource(t, "../../web/src/lib/encrypt.ts")

	if strings.Contains(serverContent, `const SecretKey = "nanokvm-sipeed-2024"`) ||
		strings.Contains(clientContent, `const SECRET_KEY = 'nanokvm-sipeed-2024'`) {
		t.Fatal("password payload encryption should not depend on a hard-coded shared key shipped in both client and server")
	}
}
func TestAuthAPIWrapperEncryptsLoginPasswordAtBoundary(t *testing.T) {
	content := readSource(t, "../../web/src/api/auth.ts")
	if containsAll(content, "password", "http.post('/api/auth/login', data)") &&
		!strings.Contains(content, "encrypt(") {
		t.Fatal("auth API wrapper should encrypt login passwords at the network boundary instead of relying on every caller to pre-encrypt")
	}
}
func TestAuthAPIWrapperEncryptsPasswordChangeAtBoundary(t *testing.T) {
	content := readSource(t, "../../web/src/api/auth.ts")
	if containsAll(content, "changePassword(username: string, password: string)", "http.post('/api/auth/password', data)") &&
		!strings.Contains(content, "encrypt(") {
		t.Fatal("auth API wrapper should encrypt password-change payloads at the network boundary instead of relying on every caller to pre-encrypt")
	}
}
func TestFrontendTokenCookieUsesBrowserSecurityAttributes(t *testing.T) {
	content := readSource(t, "../../web/src/lib/cookie.ts")
	if !strings.Contains(content, "sameSite") {
		t.Fatal("token cookie should set a SameSite policy")
	}
	if !strings.Contains(content, "secure") {
		t.Fatal("token cookie should set the Secure attribute when available")
	}
}
func TestFrontendTokenCookieExpiryFollowsServerJWTDuration(t *testing.T) {
	content := readSource(t, "../../web/src/lib/cookie.ts")
	if strings.Contains(content, "expires: 30") {
		t.Fatal("token cookie expiry should follow server JWT duration instead of a hard-coded 30 days")
	}
}
func TestTokenCookieRemovalUsesSameSecurityAttributes(t *testing.T) {
	content := readSource(t, "../../web/src/lib/cookie.ts")
	if strings.Contains(content, "Cookies.remove(COOKIE_TOKEN_KEY)") &&
		!strings.Contains(content, "Cookies.remove(COOKIE_TOKEN_KEY,") {
		t.Fatal("token cookie removal should specify the same path/domain/security attributes used when setting it")
	}
}
func TestFrontendPasswordValidationMatchesBackendBoundaries(t *testing.T) {
	content := readSource(t, "../../web/src/pages/auth/password/index.tsx")
	if !strings.Contains(content, "72") {
		t.Fatal("frontend password validation should enforce bcrypt's 72-byte backend limit")
	}
	if !strings.Contains(content, "trim") {
		t.Fatal("frontend username validation should reject leading/trailing whitespace like the backend")
	}
	if !strings.Contains(content, "control") && !strings.Contains(content, "\\x00") {
		t.Fatal("frontend validation should reject control characters like the backend")
	}
}
func TestAPWifiPageDoesNotAcceptPasswordFromURLQuery(t *testing.T) {
	content := readSource(t, "../../web/src/pages/wifi/index.tsx")
	if strings.Contains(content, `searchParams.get('p')`) || strings.Contains(content, `searchParams.get('P')`) {
		t.Fatal("AP Wi-Fi page should not read the AP password from URL query parameters")
	}
}
func TestAPWifiPageDoesNotKeepAPPasswordInReactState(t *testing.T) {
	content := readSource(t, "../../web/src/pages/wifi/index.tsx")
	if strings.Contains(content, "useState<string>('')") && strings.Contains(content, "setApPassword(password)") {
		t.Fatal("AP Wi-Fi password should not be retained in long-lived React component state after verification")
	}
}
func TestAPWifiPageDoesNotLogConnectionErrorsWithSecrets(t *testing.T) {
	content := readSource(t, "../../web/src/pages/wifi/index.tsx")
	if strings.Contains(content, "console.error(err)") || strings.Contains(content, "console.log(err)") {
		t.Fatal("AP Wi-Fi flow should not log caught errors because request objects may include AP or station passwords")
	}
}
func TestLoginPageDoesNotExposeLockoutOracleToAttackers(t *testing.T) {
	content := readSource(t, "../../web/src/pages/auth/login/index.tsx")
	if strings.Contains(content, "rsp.code === -5") || strings.Contains(content, "rsp.code === -4") {
		t.Fatal("login UI should not expose distinct lockout/protection states to unauthenticated callers")
	}
}
func TestLoginResponseDoesNotExposeBearerTokenToJavaScript(t *testing.T) {
	loginSource := readSource(t, "../service/auth/login.go")
	cookieSource := readSource(t, "../../web/src/lib/cookie.ts")

	if strings.Contains(loginSource, "Token: token") && strings.Contains(cookieSource, "Cookies.set") {
		t.Fatal("login returns the bearer token to JavaScript and the frontend stores it in a JS-readable cookie")
	}
}
func TestLogoutClearsAuthCookieServerSide(t *testing.T) {
	loginSource := readSource(t, "../service/auth/login.go")
	apiSource := readSource(t, "../../web/src/api/auth.ts")
	logoutSource := readSource(t, "../../web/src/pages/desktop/menu/settings/account/logout.tsx")

	serverClearsCookie := strings.Contains(loginSource, "SetCookie") || strings.Contains(loginSource, "MaxAge")
	clientOnlyClearsCookie := strings.Contains(apiSource, "http.post('/api/auth/logout')") && strings.Contains(logoutSource, "removeToken()")
	if clientOnlyClearsCookie && !serverClearsCookie {
		t.Fatal("logout should expire the auth cookie server-side instead of relying only on JavaScript removal")
	}
}
func TestProtectedRouteDoesNotTrustCookiePresenceOnly(t *testing.T) {
	content := readSource(t, "../../web/src/components/auth.tsx")
	if strings.Contains(content, "existToken()") && !strings.Contains(content, "isPasswordUpdated") && !strings.Contains(content, "getAccount") {
		t.Fatal("frontend protected route should validate the session with the server instead of trusting cookie presence")
	}
}
func TestDefaultPasswordWarningCannotBeLocallySkipped(t *testing.T) {
	notificationSource := readSource(t, "../../web/src/pages/desktop/notification.tsx")
	localStorageSource := readSource(t, "../../web/src/lib/localstorage.ts")

	if strings.Contains(notificationSource, "setSkipModifyPassword(true)") &&
		strings.Contains(localStorageSource, "SKIP_MODIFY_PASSWORD_KEY") {
		t.Fatal("default-password warning should not be suppressible only through client-side localStorage")
	}
}
func TestFrontendHTTPClientDoesNotLogAuthErrorsWithPayloads(t *testing.T) {
	content := readSource(t, "../../web/src/lib/http.ts")
	if strings.Contains(content, "console.log(error)") {
		t.Fatal("frontend HTTP interceptor should not log full auth errors because axios errors may include request payloads and headers")
	}
}
