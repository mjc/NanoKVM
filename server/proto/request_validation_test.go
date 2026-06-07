package proto

import "testing"

func expectValidationError(t *testing.T, name string, req interface{}) {
	t.Helper()
	if err := ValidateRequest(req); err == nil {
		t.Fatalf("%s: expected validation error", name)
	}
}

func expectValidationOK(t *testing.T, name string, req interface{}) {
	t.Helper()
	if err := ValidateRequest(req); err != nil {
		t.Fatalf("%s: unexpected validation error: %v", name, err)
	}
}

func TestPrivilegedRequestValidationRejectsUnknownModesAndOutOfRangeValues(t *testing.T) {
	expectValidationError(t, "gpio type", &SetGpioReq{Type: "halt"})
	expectValidationError(t, "gpio duration", &SetGpioReq{Type: "reset", Duration: 30001})
	expectValidationError(t, "screen type", &SetScreenReq{Type: "brightness", Value: 1})
	expectValidationError(t, "screen value", &SetScreenReq{Type: "fps", Value: 10001})
	expectValidationError(t, "script type", &RunScriptReq{Name: "job.sh", Type: "daemon"})
	expectValidationError(t, "virtual device", &UpdateVirtualDeviceReq{Device: "keyboard"})
	expectValidationError(t, "memory limit", &SetMemoryLimitReq{Enabled: true, Limit: 1073741825})
	expectValidationError(t, "oled sleep", &SetOledReq{Sleep: 86401})
	expectValidationError(t, "swap size", &SetSwapReq{Size: 65537})
	expectValidationError(t, "mouse jiggler mode", &SetMouseJigglerReq{Enabled: true, Mode: "teleport"})
	expectValidationError(t, "hostname", &SetHostnameReq{Hostname: "bad host"})
	expectValidationError(t, "title length", &SetWebTitleReq{Title: string(make([]byte, 65))})
}

func TestNetworkAndStorageRequestsValidateIdentifiers(t *testing.T) {
	expectValidationError(t, "wol mac", &WakeOnLANReq{Mac: "not-a-mac"})
	expectValidationError(t, "dns server", &SetDNSReq{Mode: "manual", Servers: []string{"not-an-ip"}})
	expectValidationError(t, "mount image outside data", &MountImageReq{File: "/tmp/evil.iso"})
	expectValidationError(t, "delete image outside data", &DeleteImageReq{File: "/tmp/evil.iso"})

	expectValidationOK(t, "wol mac", &WakeOnLANReq{Mac: "00:11:22:33:44:55"})
	expectValidationOK(t, "dns server", &SetDNSReq{Mode: "manual", Servers: []string{"1.1.1.1"}})
	expectValidationOK(t, "mount image", &MountImageReq{File: "/data/install.iso"})
	expectValidationOK(t, "delete image", &DeleteImageReq{File: "/data/install.iso"})
}

func TestHIDRequestValidationRejectsUnsupportedModes(t *testing.T) {
	expectValidationError(t, "hid mode", &SetHidModeReq{Mode: "both"})
	expectValidationError(t, "empty shortcut", &AddShortcutReq{})

	expectValidationOK(t, "hid-only", &SetHidModeReq{Mode: "hid-only"})
	expectValidationOK(t, "shortcut", &AddShortcutReq{Keys: []ShortcutKey{{Code: "KeyA", Label: "A"}}})
}
