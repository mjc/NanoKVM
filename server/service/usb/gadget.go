package usb

var (
	GadgetPath = "/sys/kernel/config/usb_gadget/g0"
	ConfigPath = GadgetPath + "/configs/c.1"
	ModeFlag   = GadgetPath + "/bcdDevice"
	UDCPath    = GadgetPath + "/UDC"
	UDCClass   = "/sys/class/udc"
	OTGRole    = "/proc/cviusb/otg_role"

	MassStorageFunction  = GadgetPath + "/functions/mass_storage.disk0"
	MassStorageLink      = ConfigPath + "/mass_storage.disk0"
	MassStorageFlag      = "/boot/usb.media0"
	MassStorageROFlag    = "/boot/usb.media0.ro"
	MassStorageCDROMFlag = "/boot/usb.media0.cdrom"
	LUNPath              = MassStorageFunction + "/lun.0"
	LUNFile              = LUNPath + "/file"
	LUNCDROM             = LUNPath + "/cdrom"
	LUNInquiryString     = LUNPath + "/inquiry_string"
	LUNRO                = LUNPath + "/ro"
	LUNForcedEject       = LUNPath + "/forced_eject"

	DataDiskFlag = "/boot/usb.disk0"

	RNDISFunction = GadgetPath + "/functions/rndis.usb0"
	RNDISLink     = ConfigPath + "/rndis.usb0"
	RNDISFlag     = "/boot/usb.rndis0"

	NormalInitScript  = "/kvmapp/system/init.d/S03usbdev"
	HIDOnlyInitScript = "/kvmapp/system/init.d/S03usbhid"
	ActiveInitScript  = "/etc/init.d/S03usbdev"

	LegacyNoMediaImage = "/dev/mmcblk0p3"

	massStorageInquiry = "USB Mass Storage"
	cdromInquiry       = "USB CD/DVD-ROM"
	dataDiskInquiry    = "USB Data Disk"
)
