//go:build !legacy_webrtc

package main

func shouldInitializeVideoHardware() bool {
	return false
}
