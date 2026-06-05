package usb

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	GadgetPath = "/sys/kernel/config/usb_gadget/g0"
	ConfigPath = GadgetPath + "/configs/c.1"
	ModeFlag   = GadgetPath + "/bcdDevice"
	UDCPath    = GadgetPath + "/UDC"
	UDCClass   = "/sys/class/udc"
	OTGRole    = "/proc/cviusb/otg_role"

	MassStorageFunction = GadgetPath + "/functions/mass_storage.disk0"
	MassStorageLink     = ConfigPath + "/mass_storage.disk0"
	MassStorageFlag     = "/boot/usb.disk0"
	LUNPath             = MassStorageFunction + "/lun.0"
	LUNFile             = LUNPath + "/file"
	LUNCDROM            = LUNPath + "/cdrom"
	LUNInquiryString    = LUNPath + "/inquiry_string"
	LUNRO               = LUNPath + "/ro"
	LUNForcedEject      = LUNPath + "/forced_eject"

	DataDiskFunction = GadgetPath + "/functions/mass_storage.disk1"
	DataDiskLink     = ConfigPath + "/mass_storage.disk1"
	DataDiskFlag     = "/boot/usb.disk1"
	DataDiskLUNPath  = DataDiskFunction + "/lun.0"
	DataDiskLUNFile  = DataDiskLUNPath + "/file"

	RNDISFunction = GadgetPath + "/functions/rndis.usb0"
	RNDISLink     = ConfigPath + "/rndis.usb0"
	RNDISFlag     = "/boot/usb.rndis0"

	NormalInitScript  = "/kvmapp/system/init.d/S03usbdev"
	HIDOnlyInitScript = "/kvmapp/system/init.d/S03usbhid"
	ActiveInitScript  = "/etc/init.d/S03usbdev"

	LegacyNoMediaImage = "/dev/mmcblk0p3"
)

const (
	hidReopenTimeout = 2 * time.Second
	hidReopenDelay   = 100 * time.Millisecond
)

type HIDController interface {
	Lock()
	Unlock()
	CloseNoLock()
	OpenNoLockWithRetry(time.Duration, time.Duration) error
}

func WithDetachedUDC(h HIDController, mutate func() error) error {
	h.Lock()
	h.CloseNoLock()
	defer h.Unlock()

	detachErr := DetachUDC()
	mutateErr := mutate()
	attachErr := AttachUDC()
	openErr := h.OpenNoLockWithRetry(hidReopenTimeout, hidReopenDelay)

	return errors.Join(detachErr, mutateErr, attachErr, openErr)
}

func Rebind(h HIDController) error {
	return WithDetachedUDC(h, func() error { return nil })
}

func RestartPHY(h HIDController) error {
	h.Lock()
	h.CloseNoLock()
	defer h.Unlock()

	if err := exec.Command(ActiveInitScript, "restart_phy").Run(); err != nil {
		return fmt.Errorf("restart usb phy: %w", err)
	}

	if err := h.OpenNoLockWithRetry(hidReopenTimeout, hidReopenDelay); err != nil {
		return fmt.Errorf("reopen HID devices after usb phy reset: %w", err)
	}

	return nil
}

func DetachUDC() error {
	return WriteString(UDCPath, "")
}

func AttachUDC() error {
	udc, err := FirstUDC()
	if err != nil {
		return err
	}

	if err := WriteString(UDCPath, udc); err != nil {
		return err
	}

	if Exists(OTGRole) {
		return WriteString(OTGRole, "device")
	}
	return nil
}

func FirstUDC() (string, error) {
	entries, err := os.ReadDir(UDCClass)
	if err != nil {
		return "", fmt.Errorf("read UDC list: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name != "" {
			return name, nil
		}
	}
	return "", fmt.Errorf("no UDC found in %s", UDCClass)
}

func SetVirtualMediaEnabled(h HIDController, enabled bool) error {
	return WithDetachedUDC(h, func() error {
		if enabled {
			if err := EnsureFile(MassStorageFlag); err != nil {
				return err
			}
			return ensureMassStorageLink()
		}

		errs := []error{
			RemoveIfExists(MassStorageLink),
			RemoveIfExists(MassStorageFlag),
		}
		if Exists(LUNFile) {
			errs = append([]error{DetachLUN()}, errs...)
		}
		return errors.Join(errs...)
	})
}

func SetDataDiskEnabled(h HIDController, enabled bool) error {
	return WithDetachedUDC(h, func() error {
		if enabled {
			if err := EnsureFile(DataDiskFlag); err != nil {
				return err
			}
			if err := ensureDataDiskLink(); err != nil {
				return err
			}
			return WriteString(DataDiskLUNFile, LegacyNoMediaImage)
		}

		errs := []error{
			RemoveIfExists(DataDiskLink),
			RemoveIfExists(DataDiskFlag),
		}
		if Exists(DataDiskLUNFile) {
			errs = append([]error{WriteString(DataDiskLUNFile, "")}, errs...)
		}
		return errors.Join(errs...)
	})
}

func SetRNDISEnabled(h HIDController, enabled bool) error {
	return WithDetachedUDC(h, func() error {
		if enabled {
			if err := EnsureFile(RNDISFlag); err != nil {
				return err
			}
			if err := ensureRNDISFunction(); err != nil {
				return err
			}
			if !Exists(RNDISLink) {
				return os.Symlink(RNDISFunction, RNDISLink)
			}
			return nil
		}

		return errors.Join(
			RemoveIfExists(RNDISLink),
			RemoveIfExists(RNDISFlag),
		)
	})
}

func SetLUNImage(h HIDController, image string, cdrom bool) error {
	image = NormalizeImage(image)
	return WithDetachedUDC(h, func() error {
		if err := EnsureFile(MassStorageFlag); err != nil {
			return err
		}
		if err := ensureMassStorageLink(); err != nil {
			return err
		}

		if err := DetachLUN(); err != nil {
			return err
		}

		flag := "0"
		if image != "" && cdrom {
			flag = "1"
		}

		if err := WriteString(LUNRO, flag); err != nil {
			return fmt.Errorf("set read-only flag: %w", err)
		}
		if err := WriteString(LUNCDROM, flag); err != nil {
			return fmt.Errorf("set cdrom flag: %w", err)
		}

		inquiryProduct := "USB Mass Storage"
		if flag == "1" {
			inquiryProduct = "USB CD/DVD-ROM"
		}
		inquiry := fmt.Sprintf("%-8s%-16s%04x", "NanoKVM", inquiryProduct, 0x0520)
		if err := WriteString(LUNInquiryString, inquiry); err != nil {
			return fmt.Errorf("set inquiry string: %w", err)
		}

		if image == "" {
			return nil
		}
		return WriteString(LUNFile, image)
	})
}

func DetachLUN() error {
	if Exists(LUNForcedEject) {
		if err := WriteString(LUNForcedEject, "1"); err == nil {
			return nil
		}
	}
	return WriteString(LUNFile, "")
}

func MountedImage() (string, error) {
	image, err := ReadTrimmed(LUNFile)
	if err != nil {
		return "", err
	}
	return NormalizeImage(image), nil
}

func CDROMFlag() (int64, error) {
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

func Exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func EnsureFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	return file.Close()
}

func RemoveIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

func ReadTrimmed(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func WriteString(path, value string) error {
	if err := os.WriteFile(filepath.Clean(path), []byte(value), 0o666); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func ensureMassStorageFunction() error {
	if !Exists(MassStorageFunction) {
		if err := os.Mkdir(MassStorageFunction, 0o777); err != nil {
			return fmt.Errorf("create mass storage function: %w", err)
		}
	}
	if err := WriteString(filepath.Join(LUNPath, "removable"), "1"); err != nil {
		return fmt.Errorf("set removable flag: %w", err)
	}
	if err := WriteString(LUNInquiryString, fmt.Sprintf("%-8s%-16s%04x", "NanoKVM", "USB Mass Storage", 0x0520)); err != nil {
		return fmt.Errorf("set inquiry string: %w", err)
	}
	return nil
}

func ensureMassStorageLink() error {
	if err := ensureMassStorageFunction(); err != nil {
		return err
	}
	if !Exists(MassStorageLink) {
		return os.Symlink(MassStorageFunction, MassStorageLink)
	}
	return nil
}

func ensureRNDISFunction() error {
	if Exists(RNDISFunction) {
		return nil
	}
	if err := os.Mkdir(RNDISFunction, 0o777); err != nil {
		return fmt.Errorf("create RNDIS function: %w", err)
	}
	return nil
}

func ensureDataDiskFunction() error {
	if !Exists(DataDiskFunction) {
		if err := os.Mkdir(DataDiskFunction, 0o777); err != nil {
			return fmt.Errorf("create data disk function: %w", err)
		}
	}
	if err := WriteString(filepath.Join(DataDiskLUNPath, "removable"), "1"); err != nil {
		return fmt.Errorf("set data disk removable flag: %w", err)
	}
	if err := WriteString(filepath.Join(DataDiskLUNPath, "inquiry_string"), fmt.Sprintf("%-8s%-16s%04x", "NanoKVM", "USB Data Disk", 0x0520)); err != nil {
		return fmt.Errorf("set data disk inquiry string: %w", err)
	}
	return nil
}

func ensureDataDiskLink() error {
	if err := ensureDataDiskFunction(); err != nil {
		return err
	}
	if !Exists(DataDiskLink) {
		return os.Symlink(DataDiskFunction, DataDiskLink)
	}
	return nil
}
