package vm

import (
	"errors"
	"os"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"NanoKVM-Server/proto"
	"NanoKVM-Server/service/hid"
	"NanoKVM-Server/service/usb"
)

const (
	virtualNetwork = usb.RNDISFlag
	virtualMedia   = usb.MassStorageFlag
	virtualDisk    = usb.DataDiskFlag
)

func (s *Service) GetVirtualDevice(c *gin.Context) {
	var rsp proto.Response

	network, _ := isDeviceExist(virtualNetwork)
	media, _ := isDeviceExist(virtualMedia)
	disk, _ := isDeviceExist(virtualDisk)

	rsp.OkRspWithData(c, &proto.GetVirtualDeviceRsp{
		Network: network,
		Media:   media,
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
	var update func(bool) error

	switch req.Device {
	case "network":
		device = virtualNetwork
		update = func(on bool) error {
			return usb.SetRNDISEnabled(hid.GetHid(), on)
		}

	case "media":
		device = virtualMedia
		update = func(on bool) error {
			return usb.SetVirtualMediaEnabled(hid.GetHid(), on)
		}

	case "disk":
		device = virtualDisk
		update = func(on bool) error {
			return usb.SetDataDiskEnabled(hid.GetHid(), on)
		}
	default:
		rsp.ErrRsp(c, -2, "invalid arguments")
		return
	}

	exist, _ := isDeviceExist(device)
	if err := update(!exist); err != nil {
		log.Errorf("update virtual device %s failed: %s", req.Device, err)
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
