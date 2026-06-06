package usb

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type massStorageOwner int

const (
	massStorageOwnerNone massStorageOwner = iota
	massStorageOwnerMedia
	massStorageOwnerDataDisk
)

type massStorageLUNProfile struct {
	backing        string
	readOnly       bool
	cdrom          bool
	inquiryProduct string
}

type massStorageBootState struct {
	mediaFlagExists bool
	mediaImage      string
	mediaReadOnly   bool
	mediaCDROM      bool
	dataDiskFlag    bool
}

func SetVirtualMediaEnabled(h HIDController, enabled bool) error {
	return WithDetachedUDC(h, func() error {
		if enabled {
			return switchMassStorageOwner(
				mediaLUNProfile("", false),
				true,
				func() error {
					return writeMassStorageBootState(mediaBootState("", false))
				},
			)
		}

		if currentMassStorageOwner() == massStorageOwnerDataDisk {
			return nil
		}
		return disableMassStorageOwner(massStorageOwnerMedia)
	})
}

func SetDataDiskEnabled(h HIDController, enabled bool) error {
	return WithDetachedUDC(h, func() error {
		if enabled {
			return switchMassStorageOwner(
				dataDiskLUNProfile(),
				false,
				func() error {
					return writeMassStorageBootState(dataDiskBootState())
				},
			)
		}

		if !Exists(DataDiskFlag) {
			return nil
		}
		if currentMassStorageOwner() == massStorageOwnerMedia {
			return RemoveIfExists(DataDiskFlag)
		}
		return disableMassStorageOwner(massStorageOwnerDataDisk)
	})
}

func VirtualMediaEnabled() bool {
	return currentMassStorageOwner() == massStorageOwnerMedia
}

func DataDiskEnabled() bool {
	return currentMassStorageOwner() == massStorageOwnerDataDisk
}

func currentMassStorageOwner() massStorageOwner {
	if !Exists(MassStorageLink) {
		return massStorageOwnerNone
	}
	return readMassStorageBootState().owner()
}

func disableMassStorageOwner(owner massStorageOwner) error {
	current := currentMassStorageOwner()
	if current != owner {
		if owner == massStorageOwnerDataDisk && current == massStorageOwnerMedia {
			return RemoveIfExists(DataDiskFlag)
		}
		return nil
	}

	switch owner {
	case massStorageOwnerMedia:
		if !Exists(MassStorageFlag) {
			return nil
		}
		if Exists(LUNFile) {
			if err := DetachLUN(); err != nil {
				return err
			}
		}
		return errors.Join(
			RemoveIfExists(MassStorageLink),
			writeMassStorageBootState(noMassStorageBootState()),
		)
	case massStorageOwnerDataDisk:
		if Exists(LUNFile) {
			if err := ClearString(LUNFile); err != nil {
				return err
			}
		}
		state := readMassStorageBootState()
		errs := []error{
			RemoveIfExists(MassStorageLink),
			RemoveIfExists(DataDiskFlag),
		}
		if state.hasLegacyDataDiskMediaState() {
			errs = append(errs, removeMassStorageState())
		}
		return errors.Join(errs...)
	default:
		return nil
	}
}

func readMassStorageBootState() massStorageBootState {
	image, mediaFlagExists := massStorageFlagImage()
	return massStorageBootState{
		mediaFlagExists: mediaFlagExists,
		mediaImage:      image,
		mediaReadOnly:   Exists(MassStorageROFlag),
		mediaCDROM:      Exists(MassStorageCDROMFlag),
		dataDiskFlag:    Exists(DataDiskFlag),
	}
}

func (state massStorageBootState) owner() massStorageOwner {
	if state.mediaFlagExists && state.mediaImage != LegacyNoMediaImage {
		return massStorageOwnerMedia
	}
	if state.dataDiskFlag && (!state.mediaFlagExists || state.mediaImage == LegacyNoMediaImage) {
		return massStorageOwnerDataDisk
	}
	return massStorageOwnerNone
}

func (state massStorageBootState) mountedMediaImage() string {
	if state.owner() != massStorageOwnerMedia || state.mediaImage == "" {
		return ""
	}
	return state.mediaImage
}

func (state massStorageBootState) hasLegacyDataDiskMediaState() bool {
	return state.mediaFlagExists && state.mediaImage == LegacyNoMediaImage
}

func mediaBootState(image string, cdrom bool) massStorageBootState {
	cdrom = image != "" && cdrom
	return massStorageBootState{
		mediaFlagExists: true,
		mediaImage:      image,
		mediaReadOnly:   cdrom,
		mediaCDROM:      cdrom,
	}
}

func dataDiskBootState() massStorageBootState {
	return massStorageBootState{dataDiskFlag: true}
}

func noMassStorageBootState() massStorageBootState {
	return massStorageBootState{}
}

func massStorageFlagImage() (string, bool) {
	image, err := ReadTrimmed(MassStorageFlag)
	return image, err == nil
}

func activeMediaBootState() (massStorageBootState, bool) {
	state := readMassStorageBootState()
	return state, Exists(MassStorageLink) && state.mountedMediaImage() != ""
}

func SetLUNImage(h HIDController, image string, cdrom bool) error {
	image = NormalizeImage(image)
	return WithDetachedUDC(h, func() error {
		if image == "" && currentMassStorageOwner() == massStorageOwnerDataDisk {
			return nil
		}
		return switchMassStorageOwner(
			mediaLUNProfile(image, cdrom),
			true,
			func() error {
				return writeMassStorageBootState(mediaBootState(image, cdrom))
			},
		)
	})
}

func DetachLUN() error {
	if Exists(LUNForcedEject) {
		return errors.Join(
			WriteString(LUNForcedEject, "1"),
			ClearString(LUNFile),
		)
	}
	return ClearString(LUNFile)
}

func MountedImage() (string, error) {
	if _, ok := activeMediaBootState(); !ok {
		return "", nil
	}
	image, err := ReadTrimmed(LUNFile)
	if err != nil {
		return "", err
	}
	return NormalizeImage(image), nil
}

func CDROMFlag() (int64, error) {
	if _, ok := activeMediaBootState(); !ok {
		return 0, nil
	}
	flag, err := ReadTrimmed(LUNCDROM)
	if err != nil {
		return 0, err
	}
	if flag == "1" {
		return 1, nil
	}
	return 0, nil
}

func NormalizeImage(image string) string {
	image = strings.TrimSpace(image)
	if image == LegacyNoMediaImage {
		return ""
	}
	return image
}

func lunInquiry(product string) string {
	return fmt.Sprintf("%-8s%-16s%04x", "NanoKVM", product, 0x0520)
}

func prepareMassStorageLUN(image string, cdrom bool) error {
	return prepareSharedMassStorageLUN(mediaLUNProfile(image, cdrom), true)
}

func prepareDataDiskLUN() error {
	return prepareSharedMassStorageLUN(dataDiskLUNProfile(), false)
}

func switchMassStorageOwner(profile massStorageLUNProfile, detach bool, persist func() error) error {
	if err := prepareSharedMassStorageLUN(profile, detach); err != nil {
		return err
	}
	return persist()
}

func prepareSharedMassStorageLUN(profile massStorageLUNProfile, detach bool) error {
	if err := ensureSharedMassStorageFunction(); err != nil {
		return err
	}
	if err := writeMassStorageLUNMode(profile); err != nil {
		return err
	}
	if detach {
		if err := DetachLUN(); err != nil {
			return err
		}
	}
	if err := attachMassStorageBacking(profile); err != nil {
		return err
	}
	return linkSharedMassStorageFunction()
}

func mediaLUNProfile(image string, cdrom bool) massStorageLUNProfile {
	profile := massStorageLUNProfile{
		backing:        image,
		inquiryProduct: massStorageInquiry,
	}
	if image != "" && cdrom {
		profile.readOnly = true
		profile.cdrom = true
		profile.inquiryProduct = cdromInquiry
	}
	return profile
}

func dataDiskLUNProfile() massStorageLUNProfile {
	return massStorageLUNProfile{
		backing:        LegacyNoMediaImage,
		inquiryProduct: dataDiskInquiry,
	}
}

func writeMassStorageLUNMode(profile massStorageLUNProfile) error {
	if err := WriteString(LUNRO, boolFlag(profile.readOnly)); err != nil {
		return fmt.Errorf("set read-only flag: %w", err)
	}
	if err := WriteString(LUNCDROM, boolFlag(profile.cdrom)); err != nil {
		return fmt.Errorf("set cdrom flag: %w", err)
	}
	if err := WriteString(LUNInquiryString, lunInquiry(profile.inquiryProduct)); err != nil {
		return fmt.Errorf("set inquiry string: %w", err)
	}
	return nil
}

func attachMassStorageBacking(profile massStorageLUNProfile) error {
	if profile.backing == "" {
		return nil
	}
	return WriteString(LUNFile, profile.backing)
}

func boolFlag(enabled bool) string {
	if enabled {
		return "1"
	}
	return "0"
}

func removeMassStorageState() error {
	return writeMassStorageBootState(noMassStorageBootState())
}

func writeMassStorageBootState(state massStorageBootState) error {
	if state.dataDiskFlag {
		if err := EnsureFile(DataDiskFlag); err != nil {
			return err
		}
		return errors.Join(
			RemoveIfExists(MassStorageFlag),
			RemoveIfExists(MassStorageROFlag),
			RemoveIfExists(MassStorageCDROMFlag),
		)
	}

	if state.mediaFlagExists {
		if err := WriteString(MassStorageFlag, state.mediaImage); err != nil {
			return err
		}
		errs := []error{RemoveIfExists(DataDiskFlag)}
		if state.mediaReadOnly {
			errs = append(errs, EnsureFile(MassStorageROFlag))
		} else {
			errs = append(errs, RemoveIfExists(MassStorageROFlag))
		}
		if state.mediaCDROM {
			errs = append(errs, EnsureFile(MassStorageCDROMFlag))
		} else {
			errs = append(errs, RemoveIfExists(MassStorageCDROMFlag))
		}
		return errors.Join(errs...)
	}

	errs := []error{RemoveIfExists(DataDiskFlag)}
	return errors.Join(
		append(errs,
			RemoveIfExists(MassStorageFlag),
			RemoveIfExists(MassStorageROFlag),
			RemoveIfExists(MassStorageCDROMFlag),
		)...,
	)
}

func ensureSharedMassStorageFunction() error {
	if !Exists(MassStorageFunction) {
		if err := os.Mkdir(MassStorageFunction, 0o777); err != nil {
			return fmt.Errorf("create mass storage function: %w", err)
		}
	}
	if err := WriteString(filepath.Join(LUNPath, "removable"), "1"); err != nil {
		return fmt.Errorf("set removable flag: %w", err)
	}
	return nil
}

func linkSharedMassStorageFunction() error {
	if !Exists(MassStorageLink) {
		return os.Symlink(MassStorageFunction, MassStorageLink)
	}
	return nil
}
