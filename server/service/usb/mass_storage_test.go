package usb

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

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

func TestSetLUNImageFailurePreservesDataDisk(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}
	if err := SetDataDiskEnabled(hid, true); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(LUNRO); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(LUNRO, 0o777); err != nil {
		t.Fatal(err)
	}

	if err := SetLUNImage(hid, "/data/installer.iso", true); err == nil {
		t.Fatal("SetLUNImage succeeded despite unwritable ro flag")
	}

	if !DataDiskEnabled() {
		t.Fatal("failed image mount disabled active data disk")
	}
	if Exists(MassStorageFlag) {
		t.Fatal("failed image mount left media flag behind")
	}
}

func TestSetLUNImageFailurePreservesMountedMedia(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}
	if err := SetLUNImage(hid, "/data/old.iso", false); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(LUNRO); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(LUNRO, 0o777); err != nil {
		t.Fatal(err)
	}

	if err := SetLUNImage(hid, "/data/new.iso", true); err == nil {
		t.Fatal("SetLUNImage succeeded despite unwritable ro flag")
	}

	assertFile(t, MassStorageFlag, "/data/old.iso")
	assertFile(t, LUNFile, "/data/old.iso")
}

func TestSetLUNImageFailureDoesNotExposeUnpersistedMedia(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}
	if err := os.Remove(LUNRO); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(LUNRO, 0o777); err != nil {
		t.Fatal(err)
	}

	if err := SetLUNImage(hid, "/data/installer.iso", true); err == nil {
		t.Fatal("SetLUNImage succeeded despite unwritable ro flag")
	}

	if Exists(MassStorageLink) {
		t.Fatal("failed image mount left unpersisted media link")
	}
	if Exists(MassStorageFlag) {
		t.Fatal("failed image mount left media flag behind")
	}
}

func TestSetLUNImageDetachFailureDoesNotCreateMediaState(t *testing.T) {
	withFakeGadget(t)
	if err := os.Remove(UDCPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(UDCPath, 0o777); err != nil {
		t.Fatal(err)
	}

	if err := SetLUNImage(&fakeHID{}, "/data/installer.iso", true); err == nil {
		t.Fatal("SetLUNImage succeeded despite detach failure")
	}

	if Exists(MassStorageFlag) {
		t.Fatal("detach-failed image mount created media flag")
	}
	if Exists(MassStorageLink) {
		t.Fatal("detach-failed image mount created media link")
	}
}

func TestSetLUNImageDetachFailurePreservesMountedMedia(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}
	if err := SetLUNImage(hid, "/data/old.iso", false); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(UDCPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(UDCPath, 0o777); err != nil {
		t.Fatal(err)
	}

	if err := SetLUNImage(hid, "/data/new.iso", true); err == nil {
		t.Fatal("SetLUNImage succeeded despite detach failure")
	}

	assertFile(t, MassStorageFlag, "/data/old.iso")
	assertFile(t, LUNFile, "/data/old.iso")
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

func TestNormalizeImage(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  string
	}{
		{name: "trims whitespace", image: " /data/installer.iso \n", want: "/data/installer.iso"},
		{name: "hides legacy data disk backing", image: LegacyNoMediaImage, want: ""},
		{name: "trims then hides legacy backing", image: "  " + LegacyNoMediaImage + "  ", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeImage(tt.image); got != tt.want {
				t.Fatalf("NormalizeImage(%q) = %q, want %q", tt.image, got, tt.want)
			}
		})
	}
}

func TestLUNInquiryFormatting(t *testing.T) {
	got := lunInquiry(cdromInquiry)
	want := "NanoKVM USB CD/DVD-ROM  0520"
	if got != want {
		t.Fatalf("lunInquiry() = %q, want %q", got, want)
	}
}

func TestMountedImageIgnoresStaleDataDiskImageState(t *testing.T) {
	withFakeGadget(t)
	if err := os.Symlink(MassStorageFunction, MassStorageLink); err != nil {
		t.Fatal(err)
	}
	writeFile(t, DataDiskFlag, "")
	writeFile(t, LUNFile, "/data/installer.iso")

	image, err := MountedImage()
	if err != nil {
		t.Fatal(err)
	}
	if image != "" {
		t.Fatalf("mounted image for data disk = %q, want empty", image)
	}
}

func TestActiveMediaBootStateRequiresLinkedNonEmptyMedia(t *testing.T) {
	tests := []struct {
		name      string
		link      bool
		mediaFlag *string
		want      bool
	}{
		{name: "unlinked media", mediaFlag: stringPtr("/data/installer.iso")},
		{name: "linked empty media", link: true, mediaFlag: stringPtr("")},
		{name: "linked media image", link: true, mediaFlag: stringPtr("/data/installer.iso"), want: true},
		{name: "linked without media flag", link: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withFakeGadget(t)
			if tt.link {
				if err := os.Symlink(MassStorageFunction, MassStorageLink); err != nil {
					t.Fatal(err)
				}
			}
			if tt.mediaFlag != nil {
				writeFile(t, MassStorageFlag, *tt.mediaFlag)
			}

			_, got := activeMediaBootState()
			if got != tt.want {
				t.Fatalf("activeMediaBootState() active = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMountedImageIgnoresStaleUnlinkedMediaImage(t *testing.T) {
	withFakeGadget(t)
	writeFile(t, MassStorageFlag, "/data/installer.iso")
	writeFile(t, LUNFile, "/data/installer.iso")

	image, err := MountedImage()
	if err != nil {
		t.Fatal(err)
	}
	if image != "" {
		t.Fatalf("mounted image with unlinked media = %q, want empty", image)
	}
}

func TestMountedImageIgnoresStaleUnflaggedMediaImage(t *testing.T) {
	withFakeGadget(t)
	if err := os.Symlink(MassStorageFunction, MassStorageLink); err != nil {
		t.Fatal(err)
	}
	writeFile(t, LUNFile, "/data/installer.iso")

	image, err := MountedImage()
	if err != nil {
		t.Fatal(err)
	}
	if image != "" {
		t.Fatalf("mounted image without media flag = %q, want empty", image)
	}
}

func TestCDROMFlagIgnoresStaleUnlinkedMediaFlag(t *testing.T) {
	withFakeGadget(t)
	writeFile(t, MassStorageFlag, "/data/installer.iso")
	writeFile(t, LUNCDROM, "1")

	flag, err := CDROMFlag()
	if err != nil {
		t.Fatal(err)
	}
	if flag != 0 {
		t.Fatalf("CDROMFlag with unlinked media = %d, want 0", flag)
	}
}

func TestCDROMFlagIgnoresStaleUnflaggedMediaFlag(t *testing.T) {
	withFakeGadget(t)
	if err := os.Symlink(MassStorageFunction, MassStorageLink); err != nil {
		t.Fatal(err)
	}
	writeFile(t, LUNCDROM, "1")

	flag, err := CDROMFlag()
	if err != nil {
		t.Fatal(err)
	}
	if flag != 0 {
		t.Fatalf("CDROMFlag without media flag = %d, want 0", flag)
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

func TestPrepareMassStorageLUNLinksAfterModeAndBacking(t *testing.T) {
	withFakeGadget(t)

	if err := prepareMassStorageLUN("/data/installer.iso", true); err != nil {
		t.Fatal(err)
	}

	assertSymlink(t, MassStorageLink, MassStorageFunction)
	assertFile(t, LUNFile, "/data/installer.iso")
	assertFile(t, LUNRO, "1")
	assertFile(t, LUNCDROM, "1")
	assertContains(t, LUNInquiryString, cdromInquiry)
}

func TestLinkSharedMassStorageFunctionDoesNotResetPreparedLUN(t *testing.T) {
	withFakeGadget(t)

	if err := ensureSharedMassStorageFunction(); err != nil {
		t.Fatal(err)
	}
	profile := mediaLUNProfile("/data/installer.iso", true)
	if err := writeMassStorageLUNMode(profile); err != nil {
		t.Fatal(err)
	}
	if err := attachMassStorageBacking(profile); err != nil {
		t.Fatal(err)
	}
	if err := linkSharedMassStorageFunction(); err != nil {
		t.Fatal(err)
	}

	assertFile(t, LUNFile, "/data/installer.iso")
	assertFile(t, LUNRO, "1")
	assertFile(t, LUNCDROM, "1")
	assertContains(t, LUNInquiryString, cdromInquiry)
}

func TestPrepareDataDiskLUNLinksAfterModeAndBacking(t *testing.T) {
	withFakeGadget(t)
	writeFile(t, LUNRO, "1")
	writeFile(t, LUNCDROM, "1")
	writeFile(t, LUNInquiryString, lunInquiry(cdromInquiry))

	if err := prepareDataDiskLUN(); err != nil {
		t.Fatal(err)
	}

	assertSymlink(t, MassStorageLink, MassStorageFunction)
	assertFile(t, LUNFile, LegacyNoMediaImage)
	assertFile(t, LUNRO, "0")
	assertFile(t, LUNCDROM, "0")
	assertContains(t, LUNInquiryString, dataDiskInquiry)
}

func TestMassStorageLUNProfiles(t *testing.T) {
	tests := []struct {
		name      string
		profile   massStorageLUNProfile
		wantFile  string
		wantRO    string
		wantCDROM string
		wantQuery string
	}{
		{
			name:      "empty media",
			profile:   mediaLUNProfile("", false),
			wantRO:    "0",
			wantCDROM: "0",
			wantQuery: massStorageInquiry,
		},
		{
			name:      "writable media image",
			profile:   mediaLUNProfile("/data/disk.img", false),
			wantFile:  "/data/disk.img",
			wantRO:    "0",
			wantCDROM: "0",
			wantQuery: massStorageInquiry,
		},
		{
			name:      "cdrom media image",
			profile:   mediaLUNProfile("/data/installer.iso", true),
			wantFile:  "/data/installer.iso",
			wantRO:    "1",
			wantCDROM: "1",
			wantQuery: cdromInquiry,
		},
		{
			name:      "data disk",
			profile:   dataDiskLUNProfile(),
			wantFile:  LegacyNoMediaImage,
			wantRO:    "0",
			wantCDROM: "0",
			wantQuery: dataDiskInquiry,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withFakeGadget(t)
			writeFile(t, LUNFile, "stale")

			if err := writeMassStorageLUNMode(tt.profile); err != nil {
				t.Fatal(err)
			}
			if err := attachMassStorageBacking(tt.profile); err != nil {
				t.Fatal(err)
			}

			wantFile := tt.wantFile
			if wantFile == "" {
				wantFile = "stale"
			}
			assertFile(t, LUNFile, wantFile)
			assertFile(t, LUNRO, tt.wantRO)
			assertFile(t, LUNCDROM, tt.wantCDROM)
			assertContains(t, LUNInquiryString, tt.wantQuery)
		})
	}
}

func TestSwitchMassStorageOwnerPersistsOnlyAfterPrepare(t *testing.T) {
	withFakeGadget(t)
	if err := os.Remove(LUNRO); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(LUNRO, 0o777); err != nil {
		t.Fatal(err)
	}
	persisted := false

	err := switchMassStorageOwner(mediaLUNProfile("/data/installer.iso", true), true, func() error {
		persisted = true
		return nil
	})

	if err == nil {
		t.Fatal("switchMassStorageOwner succeeded despite prepare failure")
	}
	if persisted {
		t.Fatal("switchMassStorageOwner persisted state after prepare failure")
	}
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

func TestSetVirtualMediaEnabledDisableFailurePreservesMediaState(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}
	if err := SetLUNImage(hid, "/data/installer.iso", true); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(LUNFile); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(LUNFile, 0o777); err != nil {
		t.Fatal(err)
	}

	if err := SetVirtualMediaEnabled(hid, false); err == nil {
		t.Fatal("SetVirtualMediaEnabled disable succeeded despite unwritable LUN file")
	}

	if !Exists(MassStorageFlag) {
		t.Fatal("failed media disable removed media flag")
	}
	if !Exists(MassStorageLink) {
		t.Fatal("failed media disable removed media link")
	}
}

func TestSetVirtualMediaEnabledDisableDetachFailurePreservesMediaState(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}
	if err := SetLUNImage(hid, "/data/installer.iso", true); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(UDCPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(UDCPath, 0o777); err != nil {
		t.Fatal(err)
	}

	if err := SetVirtualMediaEnabled(hid, false); err == nil {
		t.Fatal("SetVirtualMediaEnabled disable succeeded despite detach failure")
	}

	if !Exists(MassStorageFlag) {
		t.Fatal("detach-failed media disable removed media flag")
	}
	if !Exists(MassStorageLink) {
		t.Fatal("detach-failed media disable removed media link")
	}
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

func TestSetVirtualMediaEnabledFailurePreservesDataDisk(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}
	if err := SetDataDiskEnabled(hid, true); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(LUNRO); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(LUNRO, 0o777); err != nil {
		t.Fatal(err)
	}

	if err := SetVirtualMediaEnabled(hid, true); err == nil {
		t.Fatal("SetVirtualMediaEnabled succeeded despite unwritable ro flag")
	}

	if !Exists(DataDiskFlag) {
		t.Fatal("failed media enable removed data disk flag")
	}
	if Exists(MassStorageFlag) {
		t.Fatal("failed media enable left media flag behind")
	}
}

func TestSetVirtualMediaEnabledFailureDoesNotExposeUnpersistedMedia(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}
	if err := os.Remove(LUNRO); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(LUNRO, 0o777); err != nil {
		t.Fatal(err)
	}

	if err := SetVirtualMediaEnabled(hid, true); err == nil {
		t.Fatal("SetVirtualMediaEnabled succeeded despite unwritable ro flag")
	}

	if Exists(MassStorageLink) {
		t.Fatal("failed media enable left unpersisted media link")
	}
	if Exists(MassStorageFlag) {
		t.Fatal("failed media enable left media flag behind")
	}
}

func TestSetVirtualMediaEnabledDetachFailureDoesNotCreateMediaState(t *testing.T) {
	withFakeGadget(t)
	if err := os.Remove(UDCPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(UDCPath, 0o777); err != nil {
		t.Fatal(err)
	}

	if err := SetVirtualMediaEnabled(&fakeHID{}, true); err == nil {
		t.Fatal("SetVirtualMediaEnabled succeeded despite detach failure")
	}

	if Exists(MassStorageFlag) {
		t.Fatal("detach-failed media enable created media flag")
	}
	if Exists(MassStorageLink) {
		t.Fatal("detach-failed media enable created media link")
	}
}

func TestSetVirtualMediaEnabledDetachFailurePreservesDataDisk(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}
	if err := SetDataDiskEnabled(hid, true); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(UDCPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(UDCPath, 0o777); err != nil {
		t.Fatal(err)
	}

	if err := SetVirtualMediaEnabled(hid, true); err == nil {
		t.Fatal("SetVirtualMediaEnabled succeeded despite detach failure")
	}

	if !DataDiskEnabled() {
		t.Fatal("detach-failed media enable disabled active data disk")
	}
	if Exists(MassStorageFlag) {
		t.Fatal("detach-failed media enable created media flag")
	}
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

func TestSetDataDiskEnabledFailurePreservesMedia(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}
	if err := SetLUNImage(hid, "/data/installer.iso", true); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(LUNRO); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(LUNRO, 0o777); err != nil {
		t.Fatal(err)
	}

	if err := SetDataDiskEnabled(hid, true); err == nil {
		t.Fatal("SetDataDiskEnabled succeeded despite unwritable ro flag")
	}

	if !Exists(MassStorageFlag) {
		t.Fatal("failed data disk enable removed media flag")
	}
	if Exists(DataDiskFlag) {
		t.Fatal("failed data disk enable left data disk flag behind")
	}
}

func TestSetDataDiskEnabledFailureDoesNotExposeUnpersistedDataDisk(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}
	if err := os.Remove(LUNFile); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(LUNFile, 0o777); err != nil {
		t.Fatal(err)
	}

	if err := SetDataDiskEnabled(hid, true); err == nil {
		t.Fatal("SetDataDiskEnabled succeeded despite unwritable LUN file")
	}

	if Exists(DataDiskFlag) {
		t.Fatal("failed data disk enable left data disk flag behind")
	}
	if Exists(MassStorageLink) {
		t.Fatal("failed data disk enable left unpersisted data disk link")
	}
}

func TestSetDataDiskEnabledDetachFailureDoesNotCreateDataDiskState(t *testing.T) {
	withFakeGadget(t)
	if err := os.Remove(UDCPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(UDCPath, 0o777); err != nil {
		t.Fatal(err)
	}

	if err := SetDataDiskEnabled(&fakeHID{}, true); err == nil {
		t.Fatal("SetDataDiskEnabled succeeded despite detach failure")
	}

	if Exists(DataDiskFlag) {
		t.Fatal("detach-failed data disk enable created data disk flag")
	}
	if Exists(MassStorageLink) {
		t.Fatal("detach-failed data disk enable created data disk link")
	}
}

func TestSetDataDiskEnabledDetachFailurePreservesMedia(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}
	if err := SetLUNImage(hid, "/data/installer.iso", true); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(UDCPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(UDCPath, 0o777); err != nil {
		t.Fatal(err)
	}

	if err := SetDataDiskEnabled(hid, true); err == nil {
		t.Fatal("SetDataDiskEnabled succeeded despite detach failure")
	}

	if !VirtualMediaEnabled() {
		t.Fatal("detach-failed data disk enable disabled active media")
	}
	if Exists(DataDiskFlag) {
		t.Fatal("detach-failed data disk enable created data disk flag")
	}
	assertFile(t, MassStorageFlag, "/data/installer.iso")
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

func TestSetVirtualMediaEnabledDisablesInactiveLegacyMediaPreservesDataDisk(t *testing.T) {
	withFakeGadget(t)
	if err := os.Symlink(MassStorageFunction, MassStorageLink); err != nil {
		t.Fatal(err)
	}
	writeFile(t, MassStorageFlag, LegacyNoMediaImage)
	writeFile(t, DataDiskFlag, "")
	writeFile(t, LUNFile, LegacyNoMediaImage)

	if err := SetVirtualMediaEnabled(&fakeHID{}, false); err != nil {
		t.Fatal(err)
	}

	if !DataDiskEnabled() {
		t.Fatal("disabling inactive legacy media disabled active data disk")
	}
	assertFile(t, MassStorageFlag, LegacyNoMediaImage)
	assertFile(t, DataDiskFlag, "")
	assertSymlink(t, MassStorageLink, MassStorageFunction)
	assertFile(t, LUNFile, LegacyNoMediaImage)
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

func TestUnmountInactiveLegacyMediaPreservesActiveDataDisk(t *testing.T) {
	withFakeGadget(t)
	if err := os.Symlink(MassStorageFunction, MassStorageLink); err != nil {
		t.Fatal(err)
	}
	writeFile(t, MassStorageFlag, LegacyNoMediaImage)
	writeFile(t, DataDiskFlag, "")
	writeFile(t, LUNFile, LegacyNoMediaImage)
	writeFile(t, LUNInquiryString, lunInquiry(dataDiskInquiry))

	if err := SetLUNImage(&fakeHID{}, "", false); err != nil {
		t.Fatal(err)
	}

	if !DataDiskEnabled() {
		t.Fatal("unmounting inactive legacy media disabled active data disk")
	}
	if VirtualMediaEnabled() {
		t.Fatal("unmounting inactive legacy media enabled virtual media")
	}
	assertFile(t, MassStorageFlag, LegacyNoMediaImage)
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

	assertFile(t, LUNFile, "\n")
	if Exists(DataDiskFlag) {
		t.Fatal("data disk flag still exists after disabling data disk")
	}
	if Exists(MassStorageLink) {
		t.Fatal("data disk link still exists after disabling data disk")
	}
}

func TestSetDataDiskEnabledDisableFailurePreservesDataDiskState(t *testing.T) {
	withFakeGadget(t)
	hid := &fakeHID{}
	if err := SetDataDiskEnabled(hid, true); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(LUNFile); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(LUNFile, 0o777); err != nil {
		t.Fatal(err)
	}

	if err := SetDataDiskEnabled(hid, false); err == nil {
		t.Fatal("SetDataDiskEnabled disable succeeded despite unwritable LUN file")
	}

	if !Exists(DataDiskFlag) {
		t.Fatal("failed data disk disable removed data disk flag")
	}
	if !Exists(MassStorageLink) {
		t.Fatal("failed data disk disable removed data disk link")
	}
}

func TestSetDataDiskEnabledDisablesLegacyMediaBackedDataDisk(t *testing.T) {
	withFakeGadget(t)
	if err := os.Symlink(MassStorageFunction, MassStorageLink); err != nil {
		t.Fatal(err)
	}
	writeFile(t, MassStorageFlag, LegacyNoMediaImage)
	writeFile(t, MassStorageROFlag, "")
	writeFile(t, MassStorageCDROMFlag, "")
	writeFile(t, DataDiskFlag, "")
	writeFile(t, LUNFile, LegacyNoMediaImage)

	if err := SetDataDiskEnabled(&fakeHID{}, false); err != nil {
		t.Fatal(err)
	}

	if Exists(DataDiskFlag) {
		t.Fatal("data disk flag still exists after disabling legacy-backed data disk")
	}
	if Exists(MassStorageLink) {
		t.Fatal("shared mass-storage link still exists after disabling legacy-backed data disk")
	}
	if Exists(MassStorageFlag) || Exists(MassStorageROFlag) || Exists(MassStorageCDROMFlag) {
		t.Fatal("legacy media boot state still exists after disabling legacy-backed data disk")
	}
	assertFile(t, LUNFile, "\n")
}

func TestDisableMassStorageOwner(t *testing.T) {
	tests := []struct {
		name           string
		owner          massStorageOwner
		setup          func(t *testing.T)
		wantLink       bool
		wantMediaFlag  *string
		wantDataDisk   bool
		wantLUN        string
		wantForcedEmit bool
	}{
		{
			name:  "media",
			owner: massStorageOwnerMedia,
			setup: func(t *testing.T) {
				if err := SetLUNImage(&fakeHID{}, "/data/installer.iso", true); err != nil {
					t.Fatal(err)
				}
				writeFile(t, LUNForcedEject, "")
			},
			wantLUN:        "\n",
			wantForcedEmit: true,
		},
		{
			name:  "data disk",
			owner: massStorageOwnerDataDisk,
			setup: func(t *testing.T) {
				if err := SetDataDiskEnabled(&fakeHID{}, true); err != nil {
					t.Fatal(err)
				}
			},
			wantLUN: "\n",
		},
		{
			name:  "legacy-backed data disk",
			owner: massStorageOwnerDataDisk,
			setup: func(t *testing.T) {
				if err := os.Symlink(MassStorageFunction, MassStorageLink); err != nil {
					t.Fatal(err)
				}
				writeFile(t, MassStorageFlag, LegacyNoMediaImage)
				writeFile(t, DataDiskFlag, "")
				writeFile(t, LUNFile, LegacyNoMediaImage)
			},
			wantLUN: "\n",
		},
		{
			name:  "inactive data disk leaves media",
			owner: massStorageOwnerDataDisk,
			setup: func(t *testing.T) {
				if err := SetLUNImage(&fakeHID{}, "/data/installer.iso", true); err != nil {
					t.Fatal(err)
				}
				writeFile(t, DataDiskFlag, "")
			},
			wantLink:      true,
			wantMediaFlag: stringPtr("/data/installer.iso"),
			wantLUN:       "/data/installer.iso",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withFakeGadget(t)
			tt.setup(t)

			if err := disableMassStorageOwner(tt.owner); err != nil {
				t.Fatal(err)
			}

			if got := Exists(MassStorageLink); got != tt.wantLink {
				t.Fatalf("mass storage link exists = %v, want %v", got, tt.wantLink)
			}
			if tt.wantMediaFlag == nil {
				if Exists(MassStorageFlag) {
					t.Fatal("media flag exists")
				}
			} else {
				assertFile(t, MassStorageFlag, *tt.wantMediaFlag)
			}
			if got := Exists(DataDiskFlag); got != tt.wantDataDisk {
				t.Fatalf("data disk flag exists = %v, want %v", got, tt.wantDataDisk)
			}
			if tt.wantLUN != "" {
				assertFile(t, LUNFile, tt.wantLUN)
			}
			if tt.wantForcedEmit {
				assertFile(t, LUNForcedEject, "1")
			}
		})
	}
}

func TestCDROMFlagTreatsOnlyOneAsEnabled(t *testing.T) {
	withFakeGadget(t)
	if err := os.Symlink(MassStorageFunction, MassStorageLink); err != nil {
		t.Fatal(err)
	}
	writeFile(t, MassStorageFlag, "/data/installer.iso")

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

func TestCDROMFlagIgnoresStaleDataDiskCDROMState(t *testing.T) {
	withFakeGadget(t)
	if err := os.Symlink(MassStorageFunction, MassStorageLink); err != nil {
		t.Fatal(err)
	}
	writeFile(t, MassStorageFlag, LegacyNoMediaImage)
	writeFile(t, DataDiskFlag, "")
	writeFile(t, LUNCDROM, "1\n")

	flag, err := CDROMFlag()
	if err != nil {
		t.Fatal(err)
	}
	if flag != 0 {
		t.Fatalf("CDROMFlag = %d for data disk, want 0", flag)
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

func TestEnabledStateTreatsLegacyMediaBackingAsDataDisk(t *testing.T) {
	withFakeGadget(t)
	if err := os.Symlink(MassStorageFunction, MassStorageLink); err != nil {
		t.Fatal(err)
	}
	writeFile(t, MassStorageFlag, LegacyNoMediaImage)
	writeFile(t, DataDiskFlag, "")

	if VirtualMediaEnabled() {
		t.Fatal("legacy media backing reported as virtual media")
	}
	if !DataDiskEnabled() {
		t.Fatal("data disk did not report enabled with legacy media backing")
	}
}

func TestMassStorageOwnerClassifiesSharedSlot(t *testing.T) {
	tests := []struct {
		name          string
		link          bool
		mediaFlag     *string
		dataDiskFlag  bool
		wantOwner     massStorageOwner
		wantMedia     bool
		wantDataDisk  bool
		wantMounted   string
		wantCDROMFlag int64
	}{
		{
			name:         "unlinked boot flags are stale",
			mediaFlag:    stringPtr("/data/installer.iso"),
			dataDiskFlag: true,
			wantOwner:    massStorageOwnerNone,
		},
		{
			name:          "empty media owns linked slot",
			link:          true,
			mediaFlag:     stringPtr(""),
			wantOwner:     massStorageOwnerMedia,
			wantMedia:     true,
			wantMounted:   "",
			wantCDROMFlag: 0,
		},
		{
			name:          "mounted image owns linked slot as media",
			link:          true,
			mediaFlag:     stringPtr("/data/installer.iso"),
			dataDiskFlag:  true,
			wantOwner:     massStorageOwnerMedia,
			wantMedia:     true,
			wantMounted:   "/data/installer.iso",
			wantCDROMFlag: 1,
		},
		{
			name:         "legacy media backing belongs to data disk",
			link:         true,
			mediaFlag:    stringPtr(LegacyNoMediaImage),
			dataDiskFlag: true,
			wantOwner:    massStorageOwnerDataDisk,
			wantDataDisk: true,
		},
		{
			name:         "data flag without media flag owns linked slot",
			link:         true,
			dataDiskFlag: true,
			wantOwner:    massStorageOwnerDataDisk,
			wantDataDisk: true,
		},
		{
			name:      "legacy media without data flag is ownerless",
			link:      true,
			mediaFlag: stringPtr(LegacyNoMediaImage),
			wantOwner: massStorageOwnerNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withFakeGadget(t)
			writeFile(t, LUNFile, "/data/installer.iso")
			writeFile(t, LUNCDROM, "1\n")
			if tt.link {
				if err := os.Symlink(MassStorageFunction, MassStorageLink); err != nil {
					t.Fatal(err)
				}
			}
			if tt.mediaFlag != nil {
				writeFile(t, MassStorageFlag, *tt.mediaFlag)
			}
			if tt.dataDiskFlag {
				writeFile(t, DataDiskFlag, "")
			}

			if got := currentMassStorageOwner(); got != tt.wantOwner {
				t.Fatalf("currentMassStorageOwner() = %v, want %v", got, tt.wantOwner)
			}
			if got := VirtualMediaEnabled(); got != tt.wantMedia {
				t.Fatalf("VirtualMediaEnabled() = %v, want %v", got, tt.wantMedia)
			}
			if got := DataDiskEnabled(); got != tt.wantDataDisk {
				t.Fatalf("DataDiskEnabled() = %v, want %v", got, tt.wantDataDisk)
			}
			mounted, err := MountedImage()
			if err != nil {
				t.Fatal(err)
			}
			if mounted != tt.wantMounted {
				t.Fatalf("MountedImage() = %q, want %q", mounted, tt.wantMounted)
			}
			cdrom, err := CDROMFlag()
			if err != nil {
				t.Fatal(err)
			}
			if cdrom != tt.wantCDROMFlag {
				t.Fatalf("CDROMFlag() = %d, want %d", cdrom, tt.wantCDROMFlag)
			}
		})
	}
}

func TestMassStorageBootStateClassifiesLegacyFlags(t *testing.T) {
	tests := []struct {
		name             string
		mediaFlag        *string
		dataDiskFlag     bool
		wantOwner        massStorageOwner
		wantMountedImage string
		wantLegacyState  bool
	}{
		{
			name:      "no flags",
			wantOwner: massStorageOwnerNone,
		},
		{
			name:             "media image",
			mediaFlag:        stringPtr("/data/installer.iso"),
			wantOwner:        massStorageOwnerMedia,
			wantMountedImage: "/data/installer.iso",
		},
		{
			name:      "empty media",
			mediaFlag: stringPtr(""),
			wantOwner: massStorageOwnerMedia,
		},
		{
			name:            "legacy media only is ownerless",
			mediaFlag:       stringPtr(LegacyNoMediaImage),
			wantOwner:       massStorageOwnerNone,
			wantLegacyState: true,
		},
		{
			name:            "legacy media with data disk flag is data disk",
			mediaFlag:       stringPtr(LegacyNoMediaImage),
			dataDiskFlag:    true,
			wantOwner:       massStorageOwnerDataDisk,
			wantLegacyState: true,
		},
		{
			name:         "data disk flag without media",
			dataDiskFlag: true,
			wantOwner:    massStorageOwnerDataDisk,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withFakeGadget(t)
			if tt.mediaFlag != nil {
				writeFile(t, MassStorageFlag, *tt.mediaFlag)
			}
			if tt.dataDiskFlag {
				writeFile(t, DataDiskFlag, "")
			}

			state := readMassStorageBootState()
			if got := state.owner(); got != tt.wantOwner {
				t.Fatalf("owner() = %v, want %v", got, tt.wantOwner)
			}
			if got := state.mountedMediaImage(); got != tt.wantMountedImage {
				t.Fatalf("mountedMediaImage() = %q, want %q", got, tt.wantMountedImage)
			}
			if got := state.hasLegacyDataDiskMediaState(); got != tt.wantLegacyState {
				t.Fatalf("hasLegacyDataDiskMediaState() = %v, want %v", got, tt.wantLegacyState)
			}
		})
	}
}

func TestWriteMassStorageBootState(t *testing.T) {
	tests := []struct {
		name          string
		state         massStorageBootState
		wantMedia     *string
		wantDataDisk  bool
		wantReadOnly  bool
		wantCDROM     bool
		staleReadOnly bool
		staleCDROM    bool
		staleDataDisk bool
	}{
		{
			name:      "empty media",
			state:     mediaBootState("", false),
			wantMedia: stringPtr(""),
		},
		{
			name:          "cdrom media",
			state:         mediaBootState("/data/installer.iso", true),
			wantMedia:     stringPtr("/data/installer.iso"),
			wantReadOnly:  true,
			wantCDROM:     true,
			staleDataDisk: true,
		},
		{
			name:          "data disk",
			state:         dataDiskBootState(),
			wantDataDisk:  true,
			staleReadOnly: true,
			staleCDROM:    true,
		},
		{
			name:          "none",
			state:         noMassStorageBootState(),
			staleReadOnly: true,
			staleCDROM:    true,
			staleDataDisk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withFakeGadget(t)
			writeFile(t, MassStorageFlag, "/data/stale.iso")
			if tt.staleReadOnly {
				writeFile(t, MassStorageROFlag, "")
			}
			if tt.staleCDROM {
				writeFile(t, MassStorageCDROMFlag, "")
			}
			if tt.staleDataDisk {
				writeFile(t, DataDiskFlag, "")
			}

			if err := writeMassStorageBootState(tt.state); err != nil {
				t.Fatal(err)
			}

			if tt.wantMedia == nil {
				if Exists(MassStorageFlag) {
					t.Fatal("media flag exists")
				}
			} else {
				assertFile(t, MassStorageFlag, *tt.wantMedia)
			}
			if got := Exists(DataDiskFlag); got != tt.wantDataDisk {
				t.Fatalf("data disk flag exists = %v, want %v", got, tt.wantDataDisk)
			}
			if got := Exists(MassStorageROFlag); got != tt.wantReadOnly {
				t.Fatalf("read-only flag exists = %v, want %v", got, tt.wantReadOnly)
			}
			if got := Exists(MassStorageCDROMFlag); got != tt.wantCDROM {
				t.Fatalf("cdrom flag exists = %v, want %v", got, tt.wantCDROM)
			}
		})
	}
}

func TestWriteMassStorageBootStateFailurePreservesOldOwner(t *testing.T) {
	t.Run("media write failure preserves data disk", func(t *testing.T) {
		withFakeGadget(t)
		writeFile(t, DataDiskFlag, "")
		if err := os.Mkdir(MassStorageFlag, 0o777); err != nil {
			t.Fatal(err)
		}

		err := writeMassStorageBootState(mediaBootState("/data/installer.iso", true))
		if err == nil {
			t.Fatal("writeMassStorageBootState succeeded despite unwritable media flag")
		}
		if !Exists(DataDiskFlag) {
			t.Fatal("failed media state write removed existing data disk flag")
		}
	})

	t.Run("data disk write failure preserves media", func(t *testing.T) {
		withFakeGadget(t)
		writeFile(t, MassStorageFlag, "/data/installer.iso")
		if err := os.Remove(DataDiskFlag); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if err := os.Mkdir(DataDiskFlag, 0o777); err != nil {
			t.Fatal(err)
		}

		err := writeMassStorageBootState(dataDiskBootState())
		if err == nil {
			t.Fatal("writeMassStorageBootState succeeded despite unwritable data disk flag")
		}
		if !Exists(MassStorageFlag) {
			t.Fatal("failed data disk state write removed existing media flag")
		}
	})
}
