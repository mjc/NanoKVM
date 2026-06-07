package config

import "testing"

func TestFrontendAuthPagesSecurityContracts(t *testing.T) {
	cases := []sourceSecurityContract{
		{
			name:       "LoginTrustsAmbientTokenPresenceForRedirect",
			path:       "../../web/src/pages/auth/login/index.tsx",
			vulnerable: []string{`if (existToken()) {`, `navigate('/', { replace: true });`},
			message:    "login redirect should validate token freshness with the server instead of trusting cookie presence",
		},
		{
			name:       "LoginClearsErrorsAfterFixedDelay",
			path:       "../../web/src/pages/auth/login/index.tsx",
			vulnerable: []string{`setTimeout(() => setMsg(''), 3000);`},
			message:    "login lockout and auth errors should not disappear on a fixed client-only timer",
		},
		{
			name:       "LoginDistinguishesInvalidUserResponse",
			path:       "../../web/src/pages/auth/login/index.tsx",
			vulnerable: []string{`if (rsp.code === -2) errorMsg = t('auth.invalidUser');`},
			message:    "login UI should avoid account-state or credential oracles in distinct public errors",
		},
		{
			name:       "LoginDistinguishesLockedResponse",
			path:       "../../web/src/pages/auth/login/index.tsx",
			vulnerable: []string{`else if (rsp.code === -5) errorMsg = t('auth.locked');`},
			message:    "login UI should avoid distinguishing per-account lockout state from generic failure",
		},
		{
			name:       "LoginDistinguishesGlobalLockoutResponse",
			path:       "../../web/src/pages/auth/login/index.tsx",
			vulnerable: []string{`else if (rsp.code === -4) errorMsg = t('auth.globalLocked');`},
			message:    "login UI should avoid exposing global lockout state as a probing oracle",
		},
		{
			name:       "LoginReloadsWholeAppAfterSettingToken",
			path:       "../../web/src/pages/auth/login/index.tsx",
			vulnerable: []string{`setToken(rsp.data.token);`, `window.location.reload();`},
			message:    "login should transition auth state without a full reload that can mask cookie write failures",
		},
		{
			name:       "LoginTipsLinkNoNoopener",
			path:       "../../web/src/pages/auth/login/tips.tsx",
			vulnerable: []string{`href="https://wiki.sipeed.com/hardware/en/kvm/NanoKVM/reset.html"`, `target="_blank"`},
			message:    "password-reset help links opened from the auth page should include rel noopener noreferrer",
		},
		{
			name:       "LoginTipsDisplayDefaultAdminCredentials",
			path:       "../../web/src/pages/auth/login/tips.tsx",
			vulnerable: []string{`<Text code={true}>admin/admin</Text>`},
			message:    "auth UI should not continue advertising default web credentials after first-run setup is available",
		},
		{
			name:       "LoginTipsDisplayDefaultRootCredentials",
			path:       "../../web/src/pages/auth/login/tips.tsx",
			vulnerable: []string{`<Text code={true}>root/root</Text>`},
			message:    "auth UI should not advertise default SSH credentials as a normal recovery path",
		},
		{
			name:       "PasswordClearsErrorsAfterFixedDelay",
			path:       "../../web/src/pages/auth/password/index.tsx",
			vulnerable: []string{`setTimeout(() => setMsg(''), 3000);`},
			message:    "password-change errors should remain visible until the user corrects them",
		},
		{
			name:       "PasswordValidationRejectsOnlyQuoteAndSlash",
			path:       "../../web/src/pages/auth/password/index.tsx",
			vulnerable: []string{`const regex = /['"\\/]/;`},
			message:    "password validation should align with server-side length, control-character, and bcrypt-limit rules",
		},
		{
			name:       "PasswordInvalidPasswordMessageSkipsTranslation",
			path:       "../../web/src/pages/auth/password/index.tsx",
			vulnerable: []string{`setMsg('auth.illegalPassword');`},
			message:    "password validation errors should use the same translated message contract as other auth failures",
		},
		{
			name:       "PasswordCancelUsesHardLocationReplace",
			path:       "../../web/src/pages/auth/password/index.tsx",
			vulnerable: []string{`window.location.replace('/');`},
			message:    "password-change cancellation should use router navigation and preserve auth state handling",
		},
	}

	runSourceSecurityContracts(t, cases, 13, "frontend-auth-pages")
}
