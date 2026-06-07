package config

import "testing"

func TestFrontendLocalStorageSecurityContracts(t *testing.T) {
	cases := []sourceSecurityContract{
		{
			name:       "LocalStorageExpiryJSONParseCanCrashAuthBanner",
			path:       "../../web/src/lib/localstorage.ts",
			vulnerable: []string{`const item: ItemWithExpiry = JSON.parse(itemStr);`},
			message:    "auth and security nags backed by localStorage should tolerate malformed JSON",
		},
		{
			name:       "LocalStorageResolutionAtobParseCanCrashApp",
			path:       "../../web/src/lib/localstorage.ts",
			vulnerable: []string{`const obj = JSON.parse(window.atob(resolution));`},
			message:    "resolution preferences should be parsed defensively so localStorage tampering cannot break login flow",
		},
		{
			name:       "LocalStorageResolutionUsesObfuscationNotValidation",
			path:       "../../web/src/lib/localstorage.ts",
			vulnerable: []string{`localStorage.setItem(WEB_RESOLUTION_KEY, window.btoa(JSON.stringify(resolution)));`},
			message:    "client preferences should validate schema instead of relying on base64 encoding",
		},
		{
			name:       "LocalStorageVideoModeControlsViewerState",
			path:       "../../web/src/lib/localstorage.ts",
			vulnerable: []string{`localStorage.setItem(VIDEO_MODE_KEY, mode);`},
			message:    "viewer mode read from localStorage should be allowlisted before influencing privileged UI behavior",
		},
		{
			name:       "LocalStorageVideoScaleAcceptsUnboundedNumber",
			path:       "../../web/src/lib/localstorage.ts",
			vulnerable: []string{`localStorage.setItem(VIDEO_SCALE_KEY, String(scale));`},
			message:    "video scale preferences should be clamped before storing or applying them",
		},
		{
			name:       "LocalStorageFPSAcceptsUnboundedNumber",
			path:       "../../web/src/lib/localstorage.ts",
			vulnerable: []string{`localStorage.setItem(FPS_KEY, String(fps));`},
			message:    "FPS preferences should be bounded before client code applies them to streaming controls",
		},
		{
			name:       "LocalStorageQualityAcceptsUnboundedNumber",
			path:       "../../web/src/lib/localstorage.ts",
			vulnerable: []string{`localStorage.setItem(QUALITY_KEY, String(quality));`},
			message:    "quality preferences should be bounded before reaching stream-control requests",
		},
		{
			name:       "LocalStorageGOPAcceptsUnboundedNumber",
			path:       "../../web/src/lib/localstorage.ts",
			vulnerable: []string{`localStorage.setItem(GOP_KEY, String(gop));`},
			message:    "GOP preferences should be bounded before reaching stream-control requests",
		},
		{
			name:       "LocalStorageFrameDetectControlsDetectionMode",
			path:       "../../web/src/lib/localstorage.ts",
			vulnerable: []string{`localStorage.setItem(FRAME_DETECT_KEY, String(enabled));`},
			message:    "frame detection toggles stored in localStorage should be reconciled with server-side authorization",
		},
		{
			name:       "LocalStorageMouseStyleControlsInputMode",
			path:       "../../web/src/lib/localstorage.ts",
			vulnerable: []string{`localStorage.setItem(MOUSE_STYLE_KEY, mouse);`},
			message:    "mouse style preferences should be allowlisted before affecting remote-control input behavior",
		},
		{
			name:       "LocalStorageMouseModeControlsInputMode",
			path:       "../../web/src/lib/localstorage.ts",
			vulnerable: []string{`localStorage.setItem(MOUSE_MODE_KEY, mouse);`},
			message:    "mouse mode preferences should be allowlisted before affecting remote-control input behavior",
		},
		{
			name:       "LocalStorageMouseScrollDirectionAcceptsUnboundedNumber",
			path:       "../../web/src/lib/localstorage.ts",
			vulnerable: []string{`localStorage.setItem(MOUSE_SCROLL_DIRECTION_KEY, String(direction));`},
			message:    "mouse scroll direction should be constrained to known values",
		},
		{
			name:       "LocalStorageMouseScrollIntervalAcceptsUnboundedNumber",
			path:       "../../web/src/lib/localstorage.ts",
			vulnerable: []string{`localStorage.setItem(MOUSE_SCROLL_INTERVAL_KEY, String(interval));`},
			message:    "mouse scroll interval should be bounded before affecting remote-control input timing",
		},
		{
			name:       "LocalStorageKeyboardSystemControlsInputMapping",
			path:       "../../web/src/lib/localstorage.ts",
			vulnerable: []string{`localStorage.setItem(KEYBOARD_SYSTEM_KEY, system);`},
			message:    "keyboard system preferences should be allowlisted before affecting HID input mapping",
		},
		{
			name:       "LocalStorageKeyboardLanguageControlsInputMapping",
			path:       "../../web/src/lib/localstorage.ts",
			vulnerable: []string{`localStorage.setItem(KEYBOARD_LANGUAGE_KEY, language);`},
			message:    "keyboard language preferences should be allowlisted before affecting HID input mapping",
		},
		{
			name:       "LocalStorageMenuDisabledItemsCanHideSecurityUI",
			path:       "../../web/src/lib/localstorage.ts",
			vulnerable: []string{`localStorage.setItem(MENU_DISABLED_ITEMS_KEY, value);`},
			message:    "localStorage should not be able to hide security-sensitive menu items without server policy",
		},
		{
			name:       "LocalStorageMenuDisabledItemsJSONParseCanCrash",
			path:       "../../web/src/lib/localstorage.ts",
			vulnerable: []string{`return value ? JSON.parse(value) : [];`},
			message:    "menu preferences should tolerate malformed localStorage JSON",
		},
		{
			name:       "LocalStorageMenuDisplayModeCanHideControls",
			path:       "../../web/src/lib/localstorage.ts",
			vulnerable: []string{`localStorage.setItem(MENU_AUTO_HIDE_KEY, mode);`},
			message:    "menu display mode should be allowlisted and should not hide security controls by localStorage alone",
		},
		{
			name:       "LocalStoragePowerConfirmCanDisableSafetyPrompt",
			path:       "../../web/src/lib/localstorage.ts",
			vulnerable: []string{`localStorage.setItem(POWER_CONFIRM_KEY, String(enabled));`},
			message:    "localStorage should not be the only authority for disabling destructive power confirmations",
		},
	}

	runSourceSecurityContracts(t, cases, 19, "frontend-local-storage")
}
