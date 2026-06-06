package usb

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWithDetachedUDCReattachesAfterMutationError(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}
	mutateErr := errors.New("boom")

	err := WithDetachedUDC(hid, func() error {
		assertFile(t, UDCPath, "\n")
		return mutateErr
	})

	if !errors.Is(err, mutateErr) {
		t.Fatalf("error = %v, want joined mutation error", err)
	}
	assertFile(t, UDCPath, "4340000.usb")
	assertFile(t, OTGRole, "device")
	assertHIDCycle(t, hid)
}

func TestWithDetachedUDCReportsDetachAttachAndOpenErrors(t *testing.T) {
	withFakeGadget(t)
	if err := os.Remove(UDCPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(UDCPath, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(UDCClass, "4340000.usb")); err != nil {
		t.Fatal(err)
	}
	hid := &failingHID{}
	mutated := false

	err := WithDetachedUDC(hid, func() error {
		mutated = true
		return errors.New("mutation failed")
	})
	if err == nil {
		t.Fatal("WithDetachedUDC succeeded with detach, attach, mutation, and open failures")
	}
	for _, want := range []string{"clear", "open failed"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err, want)
		}
	}
	if mutated {
		t.Fatal("WithDetachedUDC ran mutation after detach failure")
	}
	if strings.Contains(err.Error(), "mutation failed") || strings.Contains(err.Error(), "no UDC found") {
		t.Fatalf("error %q includes mutation or attach failure after detach failure", err)
	}
}

func TestFirstUDCRequiresADevice(t *testing.T) {
	withFakeGadget(t)
	if err := os.Remove(filepath.Join(UDCClass, "4340000.usb")); err != nil {
		t.Fatal(err)
	}

	if _, err := FirstUDC(); err == nil {
		t.Fatal("FirstUDC succeeded with no UDC entries")
	}
}

func TestFirstUDCSelectsSortedFirstDevice(t *testing.T) {
	withFakeGadget(t)
	writeFile(t, filepath.Join(UDCClass, "4330000.usb"), "")
	writeFile(t, filepath.Join(UDCClass, "4350000.usb"), "")

	udc, err := FirstUDC()
	if err != nil {
		t.Fatal(err)
	}
	if udc != "4330000.usb" {
		t.Fatalf("FirstUDC() = %q, want sorted first device", udc)
	}
}
