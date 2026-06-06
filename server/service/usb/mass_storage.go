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
	dataDiskFlag    bool
}

func SetVirtualMediaEnabled(h HIDController, enabled bool) error {
	return WithDetachedUDC(h, func() error {
		if enabled {
			if err := prepareMassStorageLUN("", false); err != nil {
				return err
			}
			return errors.Join(
				persistMassStorageState("", false),
				RemoveIfExists(DataDiskFlag),
			)
		}

		if currentMassStorageOwner() == massStorageOwnerDataDisk {
			return nil
		}
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
			RemoveIfExists(MassStorageFlag),
			RemoveIfExists(MassStorageROFlag),
			RemoveIfExists(MassStorageCDROMFlag),
		)
	})
}

func SetDataDiskEnabled(h HIDController, enabled bool) error {
	return WithDetachedUDC(h, func() error {
		if enabled {
			if err := prepareDataDiskLUN(); err != nil {
				return err
			}
			return errors.Join(
				EnsureFile(DataDiskFlag),
				removeMassStorageState(),
			)
		}

		if !Exists(DataDiskFlag) {
			return nil
		}
		if currentMassStorageOwner() == massStorageOwnerMedia {
			return RemoveIfExists(DataDiskFlag)
		}
		if Exists(LUNFile) {
			if err := ClearString(LUNFile); err != nil {
				return err
			}
		}
		errs := []error{
			RemoveIfExists(MassStorageLink),
			RemoveIfExists(DataDiskFlag),
		}
		if readMassStorageBootState().hasLegacyDataDiskMediaState() {
			errs = append(errs, removeMassStorageState())
		}
		return errors.Join(errs...)
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

func readMassStorageBootState() massStorageBootState {
	image, mediaFlagExists := massStorageFlagImage()
	return massStorageBootState{
		mediaFlagExists: mediaFlagExists,
		mediaImage:      image,
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

func massStorageFlagImage() (string, bool) {
	image, err := ReadTrimmed(MassStorageFlag)
	return image, err == nil
}

func SetLUNImage(h HIDController, image string, cdrom bool) error {
	image = NormalizeImage(image)
	return WithDetachedUDC(h, func() error {
		if image == "" && currentMassStorageOwner() == massStorageOwnerDataDisk {
			return nil
		}
		if err := prepareMassStorageLUN(image, cdrom); err != nil {
			return err
		}
		return errors.Join(
			persistMassStorageState(image, cdrom),
			RemoveIfExists(DataDiskFlag),
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
	state := readMassStorageBootState()
	if !Exists(MassStorageLink) || state.mountedMediaImage() == "" {
		return "", nil
	}
	image, err := ReadTrimmed(LUNFile)
	if err != nil {
		return "", err
	}
	return NormalizeImage(image), nil
}

func CDROMFlag() (int64, error) {
	state := readMassStorageBootState()
	if !Exists(MassStorageLink) || state.mountedMediaImage() == "" {
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

func persistMassStorageState(image string, cdrom bool) error {
	if err := WriteString(MassStorageFlag, image); err != nil {
		return err
	}

	errs := []error{}
	if image != "" && cdrom {
		errs = append(errs, EnsureFile(MassStorageROFlag), EnsureFile(MassStorageCDROMFlag))
	} else {
		errs = append(errs, RemoveIfExists(MassStorageROFlag), RemoveIfExists(MassStorageCDROMFlag))
	}
	return errors.Join(errs...)
}

func removeMassStorageState() error {
	return errors.Join(
		RemoveIfExists(MassStorageFlag),
		RemoveIfExists(MassStorageROFlag),
		RemoveIfExists(MassStorageCDROMFlag),
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
