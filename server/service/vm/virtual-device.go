package vm

import (
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"NanoKVM-Server/proto"
	"NanoKVM-Server/service/hid"
	"NanoKVM-Server/service/usb"
)

func (s *Service) GetVirtualDevice(c *gin.Context) {
	var rsp proto.Response

	rsp.OkRspWithData(c, &proto.GetVirtualDeviceRsp{
		Network: usb.RNDISEnabled(),
		Media:   usb.VirtualMediaEnabled(),
		Disk:    usb.DataDiskEnabled(),
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

	var update func(bool) error
	var enabled func() bool

	switch req.Device {
	case "network":
		enabled = usb.RNDISEnabled
		update = func(on bool) error {
			return usb.SetRNDISEnabled(hid.GetHid(), on)
		}

	case "media":
		enabled = usb.VirtualMediaEnabled
		update = func(on bool) error {
			return usb.SetVirtualMediaEnabled(hid.GetHid(), on)
		}

	case "disk":
		enabled = usb.DataDiskEnabled
		update = func(on bool) error {
			return usb.SetDataDiskEnabled(hid.GetHid(), on)
		}
	default:
		rsp.ErrRsp(c, -2, "invalid arguments")
		return
	}

	if err := update(!enabled()); err != nil {
		log.Errorf("update virtual device %s failed: %s", req.Device, err)
		rsp.ErrRsp(c, -3, "operation failed")
		return
	}

	rsp.OkRspWithData(c, &proto.UpdateVirtualDeviceRsp{
		On: enabled(),
	})

	log.Debugf("update virtual device %s success", req.Device)
}
