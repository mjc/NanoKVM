package jiggler

import "testing"

func TestIsValidModeAllowlist(t *testing.T) {
	for _, mode := range []string{"relative", "absolute"} {
		if !isValidMode(mode) {
			t.Fatalf("mode %q should be valid", mode)
		}
	}
	for _, mode := range []string{"", "diagonal", "../relative"} {
		if isValidMode(mode) {
			t.Fatalf("mode %q should be rejected", mode)
		}
	}
}
