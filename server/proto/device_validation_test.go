package proto

import (
	"strings"
	"testing"
)

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

func TestDeviceRequestValidationRejectsUnsafeHostnamesTitlesAndJigglerModes(t *testing.T) {
	expectValidationError(t, "hostname with spaces", &SetHostnameReq{Hostname: "bad host"})
	expectValidationError(t, "long web title", &SetWebTitleReq{Title: strings.Repeat("x", 65)})
	expectValidationError(t, "unknown jiggler mode", &SetMouseJigglerReq{Enabled: true, Mode: "diagonal"})

	expectValidationOK(t, "hostname", &SetHostnameReq{Hostname: "nano-kvm.local"})
	expectValidationOK(t, "title", &SetWebTitleReq{Title: "NanoKVM Lab"})
	expectValidationOK(t, "jiggler", &SetMouseJigglerReq{Enabled: true, Mode: "relative"})
}
