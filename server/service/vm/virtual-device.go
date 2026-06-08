package vm

import (
	"errors"
	"os"

	"NanoKVM-Server/proto"
	"NanoKVM-Server/service/hid"
	"NanoKVM-Server/utils"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

const (
	virtualNetwork        = "/boot/usb.rndis0"
	virtualDisk           = "/boot/usb.disk0"
	usbDevScript          = "/etc/init.d/S03usbdev"
	networkConfigPath     = "/sys/kernel/config/usb_gadget/g0/configs/c.1/rndis.usb0"
	massStorageConfigPath = "/sys/kernel/config/usb_gadget/g0/configs/c.1/mass_storage.disk0"
)

func (s *Service) GetVirtualDevice(c *gin.Context) {
	var rsp proto.Response

	network, _ := isDeviceExist(virtualNetwork)
	disk, _ := isDeviceExist(virtualDisk)

	rsp.OkRspWithData(c, &proto.GetVirtualDeviceRsp{
		Network: network,
		Disk:    disk,
	})
	log.Debugf("get virtual device success")
}

func (s *Service) UpdateVirtualDevice(c *gin.Context) {
	var req proto.UpdateVirtualDeviceReq
	var rsp proto.Response

	if err := proto.ParseFormRequest(c, &req); err != nil {
		rsp.ErrRsp(c, -1, "invalid argument")
		return
	}

	var device string
	var action func() error

	switch req.Device {
	case "network":
		device = virtualNetwork

		exist, _ := isDeviceExist(device)
		if !exist {
			action = mountVirtualDeviceNetwork
		} else {
			action = unmountVirtualDeviceNetwork
		}
	case "disk":
		device = virtualDisk

		exist, _ := isDeviceExist(device)
		if !exist {
			action = mountVirtualDeviceDisk
		} else {
			action = unmountVirtualDeviceDisk
		}
	default:
		rsp.ErrRsp(c, -2, "invalid arguments")
		return
	}

	h := hid.GetHid()
	h.Lock()
	h.CloseNoLock()
	defer func() {
		h.OpenNoLock()
		h.Unlock()
	}()

	if err := action(); err != nil {
		rsp.ErrRsp(c, -3, "operation failed")
		return
	}

	on, _ := isDeviceExist(device)
	rsp.OkRspWithData(c, &proto.UpdateVirtualDeviceRsp{
		On: on,
	})

	log.Debugf("update virtual device %s success", req.Device)
}

func isDeviceExist(device string) (bool, error) {
	_, err := os.Stat(device)

	if err == nil {
		return true, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	log.Errorf("check file %s err: %s", device, err)
	return false, err
}

func mountVirtualDeviceNetwork() error {
	if err := ensureVirtualDeviceFile(virtualNetwork); err != nil {
		return err
	}
	return restartUSBDeviceScript()
}

func unmountVirtualDeviceNetwork() error {
	if err := utils.Run(usbDevScript, "stop"); err != nil {
		return err
	}
	if err := os.RemoveAll(networkConfigPath); err != nil {
		return err
	}
	if err := os.Remove(virtualNetwork); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return utils.Run(usbDevScript, "start")
}

func mountVirtualDeviceDisk() error {
	if err := ensureVirtualDeviceFile(virtualDisk); err != nil {
		return err
	}
	return restartUSBDeviceScript()
}

func unmountVirtualDeviceDisk() error {
	if err := utils.Run(usbDevScript, "stop"); err != nil {
		return err
	}
	if err := os.RemoveAll(massStorageConfigPath); err != nil {
		return err
	}
	if err := os.Remove(virtualDisk); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return utils.Run(usbDevScript, "start")
}

func ensureVirtualDeviceFile(path string) error {
	return os.WriteFile(path, nil, 0o644)
}

func restartUSBDeviceScript() error {
	if err := utils.Run(usbDevScript, "stop"); err != nil {
		return err
	}
	return utils.Run(usbDevScript, "start")
}
