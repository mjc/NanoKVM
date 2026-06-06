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
		DataDiskFunction, DataDiskLink, DataDiskFlag, DataDiskLUNPath,
		DataDiskLUNFile, RNDISFunction, RNDISLink, RNDISFlag,
	}
	t.Cleanup(func() {
		GadgetPath, ConfigPath, ModeFlag, UDCPath, UDCClass, OTGRole = old[0], old[1], old[2], old[3], old[4], old[5]
		MassStorageFunction, MassStorageLink, MassStorageFlag, MassStorageROFlag = old[6], old[7], old[8], old[9]
		MassStorageCDROMFlag, LUNPath, LUNFile, LUNCDROM, LUNInquiryString, LUNRO, LUNForcedEject = old[10], old[11], old[12], old[13], old[14], old[15], old[16]
		DataDiskFunction, DataDiskLink, DataDiskFlag, DataDiskLUNPath = old[17], old[18], old[19], old[20]
		DataDiskLUNFile, RNDISFunction, RNDISLink, RNDISFlag = old[21], old[22], old[23], old[24]
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

	DataDiskFunction = MassStorageFunction
	DataDiskLink = MassStorageLink
	DataDiskFlag = filepath.Join(root, "boot", "usb.disk0")
	DataDiskLUNPath = LUNPath
	DataDiskLUNFile = LUNFile

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
	assertFile(t, MassStorageFlag, "/data/installer.iso")
	assertFile(t, MassStorageROFlag, "")
	assertFile(t, MassStorageCDROMFlag, "")
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
	assertFile(t, MassStorageFlag, "")
	if Exists(MassStorageROFlag) || Exists(MassStorageCDROMFlag) {
		t.Fatal("empty virtual media left stale read-only or cdrom boot flags")
	}
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

func TestSetVirtualMediaEnabledDisablesPersistedCDROMState(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}

	if err := SetLUNImage(hid, "/data/installer.iso", true); err != nil {
		t.Fatal(err)
	}
	assertFile(t, MassStorageFlag, "/data/installer.iso")
	assertFile(t, MassStorageROFlag, "")
	assertFile(t, MassStorageCDROMFlag, "")

	if err := SetVirtualMediaEnabled(hid, false); err != nil {
		t.Fatal(err)
	}
	if Exists(MassStorageFlag) {
		t.Fatal("media flag still exists after disabling virtual media")
	}
	if Exists(MassStorageROFlag) || Exists(MassStorageCDROMFlag) {
		t.Fatal("read-only or cdrom boot flag still exists after disabling virtual media")
	}
	if Exists(MassStorageLink) {
		t.Fatal("media link still exists after disabling virtual media")
	}
	assertFile(t, LUNFile, "\n")
}

func TestVirtualMediaAndDataDiskAreMutuallyExclusive(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}

	if err := SetLUNImage(hid, "/data/installer.iso", true); err != nil {
		t.Fatal(err)
	}
	assertFile(t, MassStorageFlag, "/data/installer.iso")
	assertFile(t, MassStorageCDROMFlag, "")
	if DataDiskEnabled() {
		t.Fatal("data disk reported enabled while media image is mounted")
	}

	if err := SetDataDiskEnabled(hid, true); err != nil {
		t.Fatal(err)
	}
	if Exists(MassStorageFlag) || Exists(MassStorageROFlag) || Exists(MassStorageCDROMFlag) {
		t.Fatal("data disk enable left virtual media boot state behind")
	}
	assertFile(t, DataDiskFlag, "")
	assertSymlink(t, MassStorageLink, MassStorageFunction)
	assertFile(t, LUNFile, LegacyNoMediaImage)
	assertFile(t, LUNRO, "0")
	assertFile(t, LUNCDROM, "0")
	assertContains(t, LUNInquiryString, dataDiskInquiry)
	if VirtualMediaEnabled() {
		t.Fatal("virtual media reported enabled while data disk is active")
	}

	if err := SetLUNImage(hid, "/data/rescue.iso", true); err != nil {
		t.Fatal(err)
	}
	if Exists(DataDiskFlag) {
		t.Fatal("media image mount left data disk boot flag behind")
	}
	assertFile(t, MassStorageFlag, "/data/rescue.iso")
	assertFile(t, MassStorageCDROMFlag, "")
	assertFile(t, LUNFile, "/data/rescue.iso")
	assertContains(t, LUNInquiryString, cdromInquiry)
	if DataDiskEnabled() {
		t.Fatal("data disk reported enabled after media image mount")
	}
}

func TestDataDiskUsesSharedMassStorageSlot(t *testing.T) {
	withFakeGadget(t)

	if err := SetDataDiskEnabled(&fakeHID{}, true); err != nil {
		t.Fatal(err)
	}

	assertSymlink(t, MassStorageLink, MassStorageFunction)
	assertFile(t, DataDiskFlag, "")
	assertFile(t, LUNFile, LegacyNoMediaImage)
	assertFile(t, LUNRO, "0")
	assertFile(t, LUNCDROM, "0")
	assertContains(t, LUNInquiryString, dataDiskInquiry)
	if VirtualMediaEnabled() {
		t.Fatal("virtual media reported enabled for data disk")
	}
	if !DataDiskEnabled() {
		t.Fatal("data disk did not report enabled from shared mass-storage slot")
	}
}

func TestSetVirtualMediaEnabledEvictsDataDisk(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}

	if err := SetDataDiskEnabled(hid, true); err != nil {
		t.Fatal(err)
	}
	if !DataDiskEnabled() {
		t.Fatal("data disk did not report enabled before media toggle")
	}

	if err := SetVirtualMediaEnabled(hid, true); err != nil {
		t.Fatal(err)
	}

	if Exists(DataDiskFlag) {
		t.Fatal("virtual media enable left data disk boot flag behind")
	}
	if DataDiskEnabled() {
		t.Fatal("data disk reported enabled after virtual media enable")
	}
	if !VirtualMediaEnabled() {
		t.Fatal("virtual media did not report enabled after evicting data disk")
	}
	assertFile(t, MassStorageFlag, "")
	assertFile(t, LUNFile, "\n")
	assertFile(t, LUNRO, "0")
	assertFile(t, LUNCDROM, "0")
	assertContains(t, LUNInquiryString, massStorageInquiry)
}

func TestSetDataDiskEnabledEvictsEmptyVirtualMedia(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}

	if err := SetVirtualMediaEnabled(hid, true); err != nil {
		t.Fatal(err)
	}
	if !VirtualMediaEnabled() {
		t.Fatal("virtual media did not report enabled before data disk toggle")
	}

	if err := SetDataDiskEnabled(hid, true); err != nil {
		t.Fatal(err)
	}

	if Exists(MassStorageFlag) || Exists(MassStorageROFlag) || Exists(MassStorageCDROMFlag) {
		t.Fatal("data disk enable left empty virtual media boot state behind")
	}
	if VirtualMediaEnabled() {
		t.Fatal("virtual media reported enabled after data disk enable")
	}
	if !DataDiskEnabled() {
		t.Fatal("data disk did not report enabled after evicting virtual media")
	}
	assertFile(t, DataDiskFlag, "")
	assertFile(t, LUNFile, LegacyNoMediaImage)
	assertFile(t, LUNRO, "0")
	assertFile(t, LUNCDROM, "0")
	assertContains(t, LUNInquiryString, dataDiskInquiry)
}

func TestDisablingInactiveSelectorPreservesActiveMassStorage(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}

	if err := SetDataDiskEnabled(hid, true); err != nil {
		t.Fatal(err)
	}
	if err := SetVirtualMediaEnabled(hid, false); err != nil {
		t.Fatal(err)
	}
	if !DataDiskEnabled() {
		t.Fatal("disabling inactive virtual media disabled active data disk")
	}
	assertFile(t, DataDiskFlag, "")
	assertSymlink(t, MassStorageLink, MassStorageFunction)
	assertFile(t, LUNFile, LegacyNoMediaImage)

	if err := SetLUNImage(hid, "/data/installer.iso", true); err != nil {
		t.Fatal(err)
	}
	if err := SetDataDiskEnabled(hid, false); err != nil {
		t.Fatal(err)
	}
	if !VirtualMediaEnabled() {
		t.Fatal("disabling inactive data disk disabled active virtual media")
	}
	assertFile(t, MassStorageFlag, "/data/installer.iso")
	assertSymlink(t, MassStorageLink, MassStorageFunction)
	assertFile(t, LUNFile, "/data/installer.iso")
}

func TestUnmountInactiveMediaPreservesActiveDataDisk(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}

	if err := SetDataDiskEnabled(hid, true); err != nil {
		t.Fatal(err)
	}
	if err := SetLUNImage(hid, "", false); err != nil {
		t.Fatal(err)
	}

	if !DataDiskEnabled() {
		t.Fatal("unmounting inactive media disabled active data disk")
	}
	if VirtualMediaEnabled() {
		t.Fatal("unmounting inactive media enabled virtual media")
	}
	assertFile(t, DataDiskFlag, "")
	assertSymlink(t, MassStorageLink, MassStorageFunction)
	assertFile(t, LUNFile, LegacyNoMediaImage)
	assertContains(t, LUNInquiryString, dataDiskInquiry)
}

func TestSetLUNImagePersistsBootMediaState(t *testing.T) {
	withFakeGadget(t)

	if err := SetLUNImage(&fakeHID{}, "/data/disk.img", false); err != nil {
		t.Fatal(err)
	}
	assertFile(t, MassStorageFlag, "/data/disk.img")
	if Exists(MassStorageROFlag) || Exists(MassStorageCDROMFlag) {
		t.Fatal("disk image mount left read-only or cdrom boot flags")
	}

	if err := SetLUNImage(&fakeHID{}, "/data/installer.iso", true); err != nil {
		t.Fatal(err)
	}
	assertFile(t, MassStorageFlag, "/data/installer.iso")
	assertFile(t, MassStorageROFlag, "")
	assertFile(t, MassStorageCDROMFlag, "")

	if err := SetLUNImage(&fakeHID{}, "", false); err != nil {
		t.Fatal(err)
	}
	assertFile(t, MassStorageFlag, "")
	if Exists(MassStorageROFlag) || Exists(MassStorageCDROMFlag) {
		t.Fatal("unmount left read-only or cdrom boot flags")
	}
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

func TestEnabledStateComesFromConfigLinks(t *testing.T) {
	withFakeGadget(t)
	writeFile(t, MassStorageFlag, "")
	writeFile(t, DataDiskFlag, "")
	writeFile(t, RNDISFlag, "")

	if VirtualMediaEnabled() || DataDiskEnabled() || RNDISEnabled() {
		t.Fatal("enabled state used boot flags instead of configfs links")
	}

	if err := os.Symlink(MassStorageFunction, MassStorageLink); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(RNDISFunction, RNDISLink); err != nil {
		t.Fatal(err)
	}

	if !VirtualMediaEnabled() || DataDiskEnabled() || !RNDISEnabled() {
		t.Fatal("enabled state did not require configfs links")
	}

	if err := os.Remove(MassStorageFlag); err != nil {
		t.Fatal(err)
	}
	if VirtualMediaEnabled() {
		t.Fatal("virtual media stayed enabled without media flag")
	}
	if !DataDiskEnabled() {
		t.Fatal("data disk did not stay enabled with data disk flag and shared link")
	}

	if err := os.Remove(DataDiskFlag); err != nil {
		t.Fatal(err)
	}
	writeFile(t, MassStorageFlag, "")
	if !VirtualMediaEnabled() {
		t.Fatal("virtual media did not report enabled with media flag and shared link")
	}
	if DataDiskEnabled() {
		t.Fatal("data disk stayed enabled without data disk flag")
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
