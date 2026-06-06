package usb

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeHID struct {
	locks  int
	closes int
	opens  int
}

func (h *fakeHID) Lock() {
	h.locks++
}

func (h *fakeHID) Unlock() {}

func (h *fakeHID) CloseNoLock() {
	h.closes++
}

func (h *fakeHID) OpenNoLockWithRetry(time.Duration, time.Duration) error {
	h.opens++
	return nil
}

func withFakeGadget(t *testing.T) {
	t.Helper()

	old := []string{
		GadgetPath, ConfigPath, ModeFlag, UDCPath, UDCClass, OTGRole,
		MassStorageFunction, MassStorageLink, MassStorageFlag, MassStorageROFlag,
		MassStorageCDROMFlag, LUNPath, LUNFile, LUNCDROM, LUNInquiryString, LUNRO, LUNForcedEject,
		DataDiskFlag, RNDISFunction, RNDISLink, RNDISFlag,
	}
	t.Cleanup(func() {
		GadgetPath, ConfigPath, ModeFlag, UDCPath, UDCClass, OTGRole = old[0], old[1], old[2], old[3], old[4], old[5]
		MassStorageFunction, MassStorageLink, MassStorageFlag, MassStorageROFlag = old[6], old[7], old[8], old[9]
		MassStorageCDROMFlag, LUNPath, LUNFile, LUNCDROM, LUNInquiryString, LUNRO, LUNForcedEject = old[10], old[11], old[12], old[13], old[14], old[15], old[16]
		DataDiskFlag, RNDISFunction, RNDISLink, RNDISFlag = old[17], old[18], old[19], old[20]
	})

	root := t.TempDir()
	GadgetPath = filepath.Join(root, "g0")
	ConfigPath = filepath.Join(GadgetPath, "configs", "c.1")
	ModeFlag = filepath.Join(GadgetPath, "bcdDevice")
	UDCPath = filepath.Join(GadgetPath, "UDC")
	UDCClass = filepath.Join(root, "udc")
	OTGRole = filepath.Join(root, "otg_role")

	MassStorageFunction = filepath.Join(GadgetPath, "functions", "mass_storage.disk0")
	MassStorageLink = filepath.Join(ConfigPath, "mass_storage.disk0")
	MassStorageFlag = filepath.Join(root, "boot", "usb.media0")
	MassStorageROFlag = filepath.Join(root, "boot", "usb.media0.ro")
	MassStorageCDROMFlag = filepath.Join(root, "boot", "usb.media0.cdrom")
	LUNPath = filepath.Join(MassStorageFunction, "lun.0")
	LUNFile = filepath.Join(LUNPath, "file")
	LUNCDROM = filepath.Join(LUNPath, "cdrom")
	LUNInquiryString = filepath.Join(LUNPath, "inquiry_string")
	LUNRO = filepath.Join(LUNPath, "ro")
	LUNForcedEject = filepath.Join(LUNPath, "forced_eject")

	DataDiskFlag = filepath.Join(root, "boot", "usb.disk0")

	RNDISFunction = filepath.Join(GadgetPath, "functions", "rndis.usb0")
	RNDISLink = filepath.Join(ConfigPath, "rndis.usb0")
	RNDISFlag = filepath.Join(root, "boot", "usb.rndis0")

	for _, dir := range []string{
		ConfigPath,
		UDCClass,
		filepath.Dir(MassStorageFlag),
		LUNPath,
		filepath.Dir(RNDISFunction),
	} {
		if err := os.MkdirAll(dir, 0o777); err != nil {
			t.Fatal(err)
		}
	}

	writeFile(t, filepath.Join(UDCClass, "4340000.usb"), "")
	writeFile(t, UDCPath, "4340000.usb")
	writeFile(t, OTGRole, "device")
	for _, path := range []string{
		LUNFile, LUNCDROM, LUNInquiryString, LUNRO,
		filepath.Join(LUNPath, "removable"),
	} {
		writeFile(t, path, "")
	}
}

type failingHID struct{}

func (h *failingHID) Lock() {}

func (h *failingHID) Unlock() {}

func (h *failingHID) CloseNoLock() {}

func (h *failingHID) OpenNoLockWithRetry(time.Duration, time.Duration) error {
	return errors.New("open failed")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o666); err != nil {
		t.Fatal(err)
	}
}

func stringPtr(value string) *string {
	return &value
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, string(got), want)
	}
}

func assertContains(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), want) {
		t.Fatalf("%s = %q, want substring %q", path, string(got), want)
	}
}

func assertSymlink(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.Readlink(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("symlink %s -> %q, want %q", path, got, want)
	}
}

func assertHIDCycle(t *testing.T, hid *fakeHID) {
	t.Helper()
	if hid.locks != 1 || hid.closes != 1 || hid.opens != 1 {
		t.Fatalf("hid cycle locks=%d closes=%d opens=%d, want 1/1/1", hid.locks, hid.closes, hid.opens)
	}
}
