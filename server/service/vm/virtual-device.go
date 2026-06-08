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
	var configPath string
	var mount bool

	switch req.Device {
	case "network":
		device = virtualNetwork
		configPath = networkConfigPath

		exist, _ := isDeviceExist(device)
		mount = !exist
	case "disk":
		device = virtualDisk
		configPath = massStorageConfigPath

		exist, _ := isDeviceExist(device)
		mount = !exist
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

	if err := setVirtualDevice(device, configPath, mount); err != nil {
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

func setVirtualDevice(device string, configPath string, mount bool) error {
	if mount {
		if err := os.WriteFile(device, nil, 0o644); err != nil {
			return err
		}
		return restartUSBDeviceScript()
	}

	if err := utils.Run(usbDevScript, "stop"); err != nil {
		return err
	}
	if err := os.RemoveAll(configPath); err != nil {
		return err
	}
	if err := os.Remove(device); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return utils.Run(usbDevScript, "start")
}

func restartUSBDeviceScript() error {
	if err := utils.Run(usbDevScript, "stop"); err != nil {
		return err
	}
	return utils.Run(usbDevScript, "start")
}
