package config

import "testing"

func TestRequestPayloadSecurityContracts(t *testing.T) {
	cases := []sourceSecurityContract{
		{
			name:       "SetGpioTypeRequiresExplicitJSONBinding",
			path:       "../proto/vm.go",
			vulnerable: []string{`Type     string ` + "`validate:\"required\"`"},
			message:    "GPIO state-changing requests should pin JSON field names instead of accepting binder drift",
		},
		{
			name:       "SetGpioDurationRequiresExplicitJSONBinding",
			path:       "../proto/vm.go",
			vulnerable: []string{`Duration uint   ` + "`validate:\"omitempty\"`"},
			message:    "GPIO duration should use explicit JSON binding and server-side bounds",
		},
		{
			name:       "SetScreenTypeRequiresExplicitJSONBinding",
			path:       "../proto/vm.go",
			vulnerable: []string{`Type  string ` + "`validate:\"required\"`"},
			message:    "screen settings should pin JSON field names instead of accepting binder drift",
		},
		{
			name:       "SetScreenValueRequiresExplicitJSONBinding",
			path:       "../proto/vm.go",
			vulnerable: []string{`Value int    ` + "`validate:\"number\"`"},
			message:    "screen setting values should be explicitly JSON-bound and bounded",
		},
		{
			name:       "RunScriptNameRequiresExplicitJSONBinding",
			path:       "../proto/vm.go",
			vulnerable: []string{`Name string ` + "`validate:\"required\"`"},
			message:    "script names should be explicitly JSON-bound before reaching privileged execution paths",
		},
		{
			name:       "RunScriptTypeRequiresExplicitJSONBindingAndOneOf",
			path:       "../proto/vm.go",
			vulnerable: []string{`Type string ` + "`validate:\"required\"`"},
			fixedBy:    []string{"oneof=foreground background"},
			message:    "script run mode should be JSON-bound and constrained to known values",
		},
		{
			name:       "DeleteScriptNameRequiresExplicitJSONBinding",
			path:       "../proto/vm.go",
			vulnerable: []string{`Name string ` + "`validate:\"required\"`"},
			message:    "script deletion should not rely on implicit binder field names",
		},
		{
			name:       "VirtualDeviceRequestRequiresExplicitJSONBinding",
			path:       "../proto/vm.go",
			vulnerable: []string{`Device string ` + "`validate:\"required\"`"},
			message:    "virtual-device toggles should pin JSON field names",
		},
		{
			name:       "MemoryLimitEnabledRequiresExplicitJSONBinding",
			path:       "../proto/vm.go",
			vulnerable: []string{`Enabled bool  ` + "`validate:\"omitempty\"`"},
			message:    "memory-limit toggles should use explicit JSON binding",
		},
		{
			name:       "MemoryLimitValueRequiresServerBounds",
			path:       "../proto/vm.go",
			vulnerable: []string{`Limit   int64 ` + "`validate:\"omitempty\"`"},
			fixedBy:    []string{"min=", "max="},
			message:    "memory-limit values should have server-side min/max bounds",
		},
		{
			name:       "OledSleepRequiresServerBounds",
			path:       "../proto/vm.go",
			vulnerable: []string{`Sleep int ` + "`validate:\"omitempty\"`"},
			fixedBy:    []string{"min=", "max="},
			message:    "OLED sleep values should have server-side min/max bounds",
		},
		{
			name:       "SwapSizeRequiresServerBounds",
			path:       "../proto/vm.go",
			vulnerable: []string{`Size int64 ` + "`validate:\"omitempty\"`"},
			fixedBy:    []string{"min=", "max="},
			message:    "swap size changes should have server-side min/max bounds",
		},
		{
			name:       "MouseJigglerEnabledRequiresExplicitJSONBinding",
			path:       "../proto/vm.go",
			vulnerable: []string{`Enabled bool   ` + "`validate:\"omitempty\"`"},
			message:    "mouse-jiggler toggles should pin JSON field names",
		},
		{
			name:       "MouseJigglerModeRequiresOneOf",
			path:       "../proto/vm.go",
			vulnerable: []string{`Mode    string ` + "`validate:\"omitempty\"`"},
			fixedBy:    []string{"oneof="},
			message:    "mouse-jiggler mode should be constrained to known values",
		},
		{
			name:       "HostnameRequiresServerValidation",
			path:       "../proto/vm.go",
			vulnerable: []string{`Hostname string ` + "`validate:\"required\"`"},
			message:    "hostname changes should be constrained before writing /etc/hosts and /etc/hostname",
		},
		{
			name:       "WebTitleRequiresLengthValidation",
			path:       "../proto/vm.go",
			vulnerable: []string{`Title string ` + "`validate:\"omitempty\"`"},
			fixedBy:    []string{"max="},
			message:    "web-title updates should have a server-side length bound",
		},
		{
			name:       "TLSRequestRequiresExplicitJSONBinding",
			path:       "../proto/vm.go",
			vulnerable: []string{`Enabled bool ` + "`validate:\"omitempty\"`"},
			message:    "TLS state changes should pin JSON field names",
		},
		{
			name:       "HIDModeRequiresExplicitJSONBinding",
			path:       "../proto/hid.go",
			vulnerable: []string{`Mode string ` + "`validate:\"required\"`"},
			message:    "HID mode changes should pin JSON field names",
		},
		{
			name:       "HIDModeRequiresOneOf",
			path:       "../proto/hid.go",
			vulnerable: []string{`Mode string ` + "`validate:\"required\"`"},
			fixedBy:    []string{"oneof=normal hid-only"},
			message:    "HID mode should be constrained to supported values",
		},
		{
			name:       "ShortcutKeysRequireExplicitJSONBinding",
			path:       "../proto/hid.go",
			vulnerable: []string{`Keys []ShortcutKey ` + "`validate:\"required\"`"},
			message:    "shortcut key arrays should pin JSON field names and validation",
		},
		{
			name:       "DeleteShortcutIDRequiresExplicitJSONBinding",
			path:       "../proto/hid.go",
			vulnerable: []string{`ID string ` + "`validate:\"required\"`"},
			message:    "shortcut deletion should not rely on implicit binder field names",
		},
		{
			name:       "LeaderKeyRequiresExplicitJSONBinding",
			path:       "../proto/hid.go",
			vulnerable: []string{`Key string ` + "`validate:\"omitempty\"`"},
			message:    "leader-key updates should pin JSON field names",
		},
		{
			name:       "WakeOnLanDoesNotUseFormOnlyBinding",
			path:       "../proto/network.go",
			vulnerable: []string{`Mac string ` + "`form:\"mac\" validate:\"required\"`"},
			message:    "wake-on-LAN should not rely on form-only credential-like request binding",
		},
		{
			name:       "DeleteWolMacDoesNotUseFormOnlyBinding",
			path:       "../proto/network.go",
			vulnerable: []string{`Mac string ` + "`form:\"mac\" validate:\"required\"`"},
			message:    "WOL MAC deletion should pin JSON field names rather than form-only binding",
		},
		{
			name:       "SetWolMacNameDoesNotUseFormOnlyMacBinding",
			path:       "../proto/network.go",
			vulnerable: []string{`Mac  string ` + "`form:\"mac\" validate:\"required\"`"},
			message:    "WOL MAC naming should not use form-only MAC binding",
		},
		{
			name:       "SetWolMacNameDoesNotUseFormOnlyNameBinding",
			path:       "../proto/network.go",
			vulnerable: []string{`Name string ` + "`form:\"name\" validate:\"required\"`"},
			message:    "WOL labels should not use form-only binding on authenticated state changes",
		},
		{
			name:       "PreviewUpdateToggleRequiresExplicitJSONBinding",
			path:       "../proto/application.go",
			vulnerable: []string{`Enable bool ` + "`validate:\"omitempty\"`"},
			message:    "preview update toggles should pin JSON field names",
		},
		{
			name:       "DNSManualServersRequireValidation",
			path:       "../proto/network.go",
			vulnerable: []string{`Servers []string ` + "`json:\"servers\"`"},
			fixedBy:    []string{"dive", "ip"},
			message:    "manual DNS server lists should be validated as IP addresses before persistence",
		},
		{
			name:       "MountImagePathRequiresValidationTag",
			path:       "../proto/storage.go",
			vulnerable: []string{`File  string ` + "`json:\"file\" validate:\"omitempty\"`"},
			fixedBy:    []string{"startswith=/data", "filepath"},
			message:    "image mount paths should be constrained to the image directory before sysfs writes",
		},
		{
			name:       "DeleteImagePathRequiresValidationTag",
			path:       "../proto/storage.go",
			vulnerable: []string{`File string ` + "`json:\"file\" validate:\"required\"`"},
			fixedBy:    []string{"startswith=/data", "filepath"},
			message:    "image deletion paths should be constrained before file removal",
		},
	}

	runSourceSecurityContracts(t, cases, 30, "request-payload")
}
