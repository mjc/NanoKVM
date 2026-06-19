package common

import "testing"

func TestGetKvmVisionStubDoesNotTouchHardware(t *testing.T) {
	vision := GetKvmVision()
	if vision == nil {
		t.Fatal("expected stub kvm vision instance")
	}

	if data, result := vision.ReadMjpeg(1920, 1080, 3000); data != nil || result != -1 {
		t.Fatalf("unexpected mjpeg stub result: data=%v result=%d", data, result)
	}

	if data, result := vision.ReadH264(1920, 1080, 3000); data != nil || result != -1 {
		t.Fatalf("unexpected h264 stub result: data=%v result=%d", data, result)
	}

	if got := vision.SetHDMI(true); got != 0 {
		t.Fatalf("unexpected hdmi stub result: %d", got)
	}
}
