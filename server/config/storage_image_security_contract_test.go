package config

import "testing"

func TestStorageImageSecurityContracts(t *testing.T) {
	cases := []sourceSecurityContract{
		{
			name:       "MountImageDefaultsToInternalPartition",
			path:       "../service/storage/image.go",
			vulnerable: []string{`imageNone      = "/dev/mmcblk0p3"`},
			message:    "unmount/default mass-storage behavior should not expose an internal partition path",
		},
		{
			name:       "MountImageLogsRequestPath",
			path:       "../service/storage/image.go",
			vulnerable: []string{`log.Debugf("mount image %s success", req.File)`},
			message:    "mount image should not log request-controlled filesystem paths",
		},
		{
			name:       "MountImageResetsUDCThroughShellEcho",
			path:       "../service/storage/image.go",
			vulnerable: []string{`"echo > /sys/kernel/config/usb_gadget/g0/UDC"`},
			message:    "mass-storage remount should manipulate UDC through direct file APIs",
		},
		{
			name:       "MountImageBindsFirstUDCThroughShellPipeline",
			path:       "../service/storage/image.go",
			vulnerable: []string{`"ls /sys/class/udc/ | cat > /sys/kernel/config/usb_gadget/g0/UDC"`},
			message:    "mass-storage remount should bind a validated UDC name without shell pipelines",
		},
		{
			name:       "StorageImageListReturnsAbsolutePaths",
			path:       "../service/storage/image.go",
			vulnerable: []string{"images = append(images, path)"},
			message:    "image listing should not expose absolute server filesystem paths to the browser",
		},
		{
			name:       "MountImageWritesRequestPathToMassStorage",
			path:       "../service/storage/image.go",
			vulnerable: []string{"os.WriteFile(mountDevice, []byte(image), 0o666)"},
			message:    "mount image should not write an arbitrary request path into USB mass-storage sysfs",
		},
		{
			name:       "MassStorageMountDeviceWriteUsesWorldPermissions",
			path:       "../service/storage/image.go",
			vulnerable: []string{"os.WriteFile(mountDevice, []byte(\"\\n\"), 0o666)"},
			message:    "mass-storage sysfs writes should not use world-writable permissions",
		},
		{
			name:       "MassStorageReadOnlyFlagWriteUsesWorldPermissions",
			path:       "../service/storage/image.go",
			vulnerable: []string{"os.WriteFile(roFlag, []byte(flag), 0o666)"},
			message:    "mass-storage read-only flag writes should not use world-writable permissions",
		},
		{
			name:       "MassStorageCdromFlagWriteUsesWorldPermissions",
			path:       "../service/storage/image.go",
			vulnerable: []string{"os.WriteFile(cdromFlag, []byte(flag), 0o666)"},
			message:    "mass-storage cdrom flag writes should not use world-writable permissions",
		},
		{
			name:       "MassStorageInquiryWriteUsesWorldPermissions",
			path:       "../service/storage/image.go",
			vulnerable: []string{"os.WriteFile(inquiryString, []byte(inquiryData), 0o666)"},
			message:    "mass-storage inquiry writes should not use world-writable permissions",
		},
		{
			name:       "MountImageResetsUSBThroughShell",
			path:       "../service/storage/image.go",
			vulnerable: []string{`exec.Command("sh", "-c", command).Run()`},
			message:    "USB reset commands should use fixed argv calls instead of shell execution",
		},
		{
			name:       "DeleteImageUsesStringPrefixPathCheck",
			path:       "../service/storage/image.go",
			vulnerable: []string{"strings.HasPrefix(filename, imageDirectory)"},
			message:    "image deletion should use filepath.Clean/Rel instead of string prefix checks",
		},
		{
			name:       "DeleteImageRemovesRequestPath",
			path:       "../service/storage/image.go",
			vulnerable: []string{"os.Remove(req.File)"},
			message:    "image deletion should remove a verified path, not the raw request path",
		},
		{
			name:       "DeleteImageLogsRequestPath",
			path:       "../service/storage/image.go",
			vulnerable: []string{`log.Debugf("delete image %s success", req.File)`},
			message:    "image deletion should not log request-provided filesystem paths",
		},
	}

	runSourceSecurityContracts(t, cases, 14, "storage-image")
}
