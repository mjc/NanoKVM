package config

import "testing"

func TestFrontendTransportSecurityContracts(t *testing.T) {
	cases := []sourceSecurityContract{
		{
			name:       "WebSocketClientDoesNotAuthenticateHandshakeExplicitly",
			path:       "../../web/src/lib/websocket.ts",
			vulnerable: []string{"this.instance = new W3cWebSocket(this.options.url)"},
			fixedBy:    []string{"Sec-WebSocket-Protocol", "token"},
			message:    "browser WebSocket clients should have an explicit auth handshake rather than relying on ambient cookies",
		},
		{
			name:       "WebSocketClientLogsErrors",
			path:       "../../web/src/lib/websocket.ts",
			vulnerable: []string{"console.error('[WebSocket] Error:', error)"},
			message:    "WebSocket errors should not be logged with potentially sensitive connection details",
		},
		{
			name:       "WebSocketClientLogsParseFailures",
			path:       "../../web/src/lib/websocket.ts",
			vulnerable: []string{"console.log(err)"},
			message:    "WebSocket message parse failures should not log raw errors from untrusted server data",
		},
		{
			name:       "BaseURLAllowsPlainHTTP",
			path:       "../../web/src/lib/service.ts",
			vulnerable: []string{"return isDefaultPort ? baseUrl : `${baseUrl}:${port}`"},
			message:    "frontend service URLs should fail closed instead of accepting plain HTTP for authenticated APIs",
		},
		{
			name:       "MSWMocksRealisticAuthToken",
			path:       "../../web/src/mocks/browser.ts",
			vulnerable: []string{"token: 'mocked_token'"},
			message:    "auth mocks should not normalize non-JWT magic token shapes",
		},
		{
			name:       "FrontendHTTPClientUsesOneMinuteTimeoutForPrivilegedActions",
			path:       "../../web/src/lib/http.ts",
			vulnerable: []string{`timeout: 60 * 1000`},
			message:    "Frontend privileged state-changing requests should use shorter, endpoint-specific timeouts",
		},
	}

	runSourceSecurityContracts(t, cases, 6, "frontend-transport")
}
