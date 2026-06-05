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
		LUNPath, LUNFile, LUNCDROM, LUNInquiryString, LUNRO, LUNForcedEject,
		DataDiskFunction, DataDiskLink, DataDiskFlag, DataDiskLUNPath,
		DataDiskLUNFile, RNDISFunction, RNDISLink, RNDISFlag,
	}
	t.Cleanup(func() {
		GadgetPath, ConfigPath, ModeFlag, UDCPath, UDCClass, OTGRole = old[0], old[1], old[2], old[3], old[4], old[5]
		MassStorageFunction, MassStorageLink, MassStorageFlag, MassStorageROFlag = old[6], old[7], old[8], old[9]
		LUNPath, LUNFile, LUNCDROM, LUNInquiryString, LUNRO, LUNForcedEject = old[10], old[11], old[12], old[13], old[14], old[15]
		DataDiskFunction, DataDiskLink, DataDiskFlag, DataDiskLUNPath = old[16], old[17], old[18], old[19]
		DataDiskLUNFile, RNDISFunction, RNDISLink, RNDISFlag = old[20], old[21], old[22], old[23]
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
	LUNPath = filepath.Join(MassStorageFunction, "lun.0")
	LUNFile = filepath.Join(LUNPath, "file")
	LUNCDROM = filepath.Join(LUNPath, "cdrom")
	LUNInquiryString = filepath.Join(LUNPath, "inquiry_string")
	LUNRO = filepath.Join(LUNPath, "ro")
	LUNForcedEject = filepath.Join(LUNPath, "forced_eject")

	DataDiskFunction = filepath.Join(GadgetPath, "functions", "mass_storage.disk1")
	DataDiskLink = filepath.Join(ConfigPath, "mass_storage.disk1")
	DataDiskFlag = filepath.Join(root, "boot", "usb.disk0")
	DataDiskLUNPath = filepath.Join(DataDiskFunction, "lun.0")
	DataDiskLUNFile = filepath.Join(DataDiskLUNPath, "file")

	RNDISFunction = filepath.Join(GadgetPath, "functions", "rndis.usb0")
	RNDISLink = filepath.Join(ConfigPath, "rndis.usb0")
	RNDISFlag = filepath.Join(root, "boot", "usb.rndis0")

	for _, dir := range []string{
		ConfigPath,
		UDCClass,
		filepath.Dir(MassStorageFlag),
		LUNPath,
		DataDiskLUNPath,
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
		DataDiskLUNFile,
		filepath.Join(DataDiskLUNPath, "removable"),
		filepath.Join(DataDiskLUNPath, "inquiry_string"),
	} {
		writeFile(t, path, "")
	}
}

func TestSetLUNImageMountsExplicitCDROM(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}

	if err := SetLUNImage(hid, " /data/installer.iso ", true); err != nil {
		t.Fatal(err)
	}

	assertFile(t, UDCPath, "4340000.usb")
	assertFile(t, OTGRole, "device")
	assertFile(t, MassStorageFlag, "")
	assertSymlink(t, MassStorageLink, MassStorageFunction)
	assertFile(t, LUNFile, "/data/installer.iso")
	assertFile(t, LUNRO, "1")
	assertFile(t, LUNCDROM, "1")
	assertContains(t, LUNInquiryString, "USB CD/DVD-ROM")
	assertHIDCycle(t, hid)
}

func TestSetLUNImageUnmountLeavesNoMedia(t *testing.T) {
	withFakeGadget(t)
	writeFile(t, LUNFile, "/data/installer.iso")

	if err := SetLUNImage(&fakeHID{}, "", false); err != nil {
		t.Fatal(err)
	}

	image, err := MountedImage()
	if err != nil {
		t.Fatal(err)
	}
	if image != "" {
		t.Fatalf("mounted image = %q, want empty", image)
	}
	assertFile(t, LUNFile, "\n")
	assertFile(t, LUNRO, "0")
	assertFile(t, LUNCDROM, "0")
	assertContains(t, LUNInquiryString, "USB Mass Storage")
}

func TestSetLUNImageUsesForcedEjectWhenAvailable(t *testing.T) {
	withFakeGadget(t)
	writeFile(t, LUNFile, "/data/installer.iso")
	writeFile(t, LUNForcedEject, "")

	if err := SetLUNImage(&fakeHID{}, "", false); err != nil {
		t.Fatal(err)
	}

	assertFile(t, LUNForcedEject, "1")
	assertFile(t, LUNFile, "\n")
}

func TestLegacyNoMediaImageIsHiddenFromMountedImage(t *testing.T) {
	withFakeGadget(t)
	writeFile(t, LUNFile, LegacyNoMediaImage)

	image, err := MountedImage()
	if err != nil {
		t.Fatal(err)
	}
	if image != "" {
		t.Fatalf("mounted image = %q, want empty", image)
	}
}

func TestSetVirtualMediaEnabledSetsFunctionDefaults(t *testing.T) {
	withFakeGadget(t)

	if err := SetVirtualMediaEnabled(&fakeHID{}, true); err != nil {
		t.Fatal(err)
	}

	assertFile(t, filepath.Join(LUNPath, "removable"), "1")
	assertContains(t, LUNInquiryString, "USB Mass Storage")
	assertSymlink(t, MassStorageLink, MassStorageFunction)
}

func TestSetVirtualMediaEnabledClearsStaleMediaState(t *testing.T) {
	withFakeGadget(t)
	writeFile(t, LUNFile, LegacyNoMediaImage)
	writeFile(t, LUNRO, "1")
	writeFile(t, LUNCDROM, "1")
	writeFile(t, LUNInquiryString, lunInquiry(cdromInquiry))

	if err := SetVirtualMediaEnabled(&fakeHID{}, true); err != nil {
		t.Fatal(err)
	}

	assertFile(t, LUNFile, "\n")
	assertFile(t, LUNRO, "0")
	assertFile(t, LUNCDROM, "0")
	assertContains(t, LUNInquiryString, massStorageInquiry)
}

func TestSetVirtualMediaEnabledIsIdempotent(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}

	if err := SetVirtualMediaEnabled(hid, true); err != nil {
		t.Fatal(err)
	}
	if err := SetVirtualMediaEnabled(hid, true); err != nil {
		t.Fatal(err)
	}
	if err := SetVirtualMediaEnabled(hid, false); err != nil {
		t.Fatal(err)
	}
	if err := SetVirtualMediaEnabled(hid, false); err != nil {
		t.Fatal(err)
	}

	if Exists(MassStorageFlag) {
		t.Fatal("media flag still exists after repeated disable")
	}
	if Exists(MassStorageLink) {
		t.Fatal("media link still exists after repeated disable")
	}
}

func TestVirtualMediaAndDataDiskAreIndependent(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}

	if err := SetVirtualMediaEnabled(hid, true); err != nil {
		t.Fatal(err)
	}
	if err := SetDataDiskEnabled(hid, true); err != nil {
		t.Fatal(err)
	}

	assertFile(t, MassStorageFlag, "")
	assertSymlink(t, MassStorageLink, MassStorageFunction)
	assertFile(t, DataDiskFlag, "")
	assertSymlink(t, DataDiskLink, DataDiskFunction)
	assertFile(t, DataDiskLUNFile, LegacyNoMediaImage)

	if err := SetVirtualMediaEnabled(hid, false); err != nil {
		t.Fatal(err)
	}
	if Exists(MassStorageFlag) {
		t.Fatal("media flag still exists after disabling virtual media")
	}
	if Exists(MassStorageLink) {
		t.Fatal("media link still exists after disabling virtual media")
	}
	assertFile(t, DataDiskFlag, "")
	assertSymlink(t, DataDiskLink, DataDiskFunction)
	assertFile(t, DataDiskLUNFile, LegacyNoMediaImage)
}

func TestSetDataDiskEnabledDisablesCleanly(t *testing.T) {
	withFakeGadget(t)

	if err := SetDataDiskEnabled(&fakeHID{}, true); err != nil {
		t.Fatal(err)
	}
	if err := SetDataDiskEnabled(&fakeHID{}, false); err != nil {
		t.Fatal(err)
	}

	assertFile(t, DataDiskLUNFile, "\n")
	if Exists(DataDiskFlag) {
		t.Fatal("data disk flag still exists after disabling data disk")
	}
	if Exists(DataDiskLink) {
		t.Fatal("data disk link still exists after disabling data disk")
	}
}

func TestCDROMFlagTreatsOnlyOneAsEnabled(t *testing.T) {
	withFakeGadget(t)

	writeFile(t, LUNCDROM, "1\n")
	flag, err := CDROMFlag()
	if err != nil {
		t.Fatal(err)
	}
	if flag != 1 {
		t.Fatalf("CDROMFlag = %d, want 1", flag)
	}

	writeFile(t, LUNCDROM, "0\n")
	flag, err = CDROMFlag()
	if err != nil {
		t.Fatal(err)
	}
	if flag != 0 {
		t.Fatalf("CDROMFlag = %d, want 0", flag)
	}
}

func TestSetRNDISEnabledTogglesFlagAndLink(t *testing.T) {
	withFakeGadget(t)

	if err := SetRNDISEnabled(&fakeHID{}, true); err != nil {
		t.Fatal(err)
	}
	assertFile(t, RNDISFlag, "")
	if !Exists(RNDISFunction) {
		t.Fatal("RNDIS function was not created")
	}
	assertSymlink(t, RNDISLink, RNDISFunction)

	if err := SetRNDISEnabled(&fakeHID{}, false); err != nil {
		t.Fatal(err)
	}
	if Exists(RNDISFlag) {
		t.Fatal("RNDIS flag still exists after disabling RNDIS")
	}
	if Exists(RNDISLink) {
		t.Fatal("RNDIS link still exists after disabling RNDIS")
	}
}

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
	if err := os.Remove(filepath.Join(UDCClass, "4340000.usb")); err != nil {
		t.Fatal(err)
	}
	hid := &failingHID{}

	err := WithDetachedUDC(hid, func() error { return errors.New("mutation failed") })
	if err == nil {
		t.Fatal("WithDetachedUDC succeeded with detach, attach, mutation, and open failures")
	}
	for _, want := range []string{"mutation failed", "no UDC found", "open failed"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err, want)
		}
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
