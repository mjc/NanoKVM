package main

import "testing"

func TestShouldInitializeVideoHardwareIsFalseForRustBackend(t *testing.T) {
	if shouldInitializeVideoHardware() {
		t.Fatal("rust backend should not initialize video hardware in default build")
	}
}
