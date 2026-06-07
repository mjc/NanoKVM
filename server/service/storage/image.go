package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"NanoKVM-Server/proto"
	"NanoKVM-Server/service/hid"
)

const (
	imageDirectory = "/data"
	imageNone      = ""
	cdromFlag      = "/sys/kernel/config/usb_gadget/g0/functions/mass_storage.disk0/lun.0/cdrom"
	mountDevice    = "/sys/kernel/config/usb_gadget/g0/functions/mass_storage.disk0/lun.0/file"
	inquiryString  = "/sys/kernel/config/usb_gadget/g0/functions/mass_storage.disk0/lun.0/inquiry_string"
	roFlag         = "/sys/kernel/config/usb_gadget/g0/functions/mass_storage.disk0/lun.0/ro"
)

func (s *Service) GetImages(c *gin.Context) {
	var rsp proto.Response
	var images []string

	err := filepath.Walk(imageDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			name := strings.ToLower(info.Name())
			if strings.HasSuffix(name, ".iso") || strings.HasSuffix(name, ".img") {
				rel, relErr := filepath.Rel(imageDirectory, path)
				if relErr == nil && rel != "." && !strings.HasPrefix(rel, "..") {
					images = append(images, rel)
				}
			}
		}

		return nil
	})
	if err != nil {
		rsp.ErrRsp(c, -2, "get images failed")
		return
	}

	rsp.OkRspWithData(c, &proto.GetImagesRsp{
		Files: images,
	})
	log.Debugf("get images success, total %d", len(images))
}

func (s *Service) MountImage(c *gin.Context) {
	var req proto.MountImageReq
	var rsp proto.Response

	if err := proto.ParseFormRequest(c, &req); err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	// cdrom and ro flag
	// set to 0 when unmount image
	// set to 1 when mount image and the CD-ROM is enabled
	if req.File == "" || req.Cdrom {
		flag := "0"
		if req.File != "" && req.Cdrom {
			flag = "1"
		}

		// unmount
		if err := os.WriteFile(mountDevice, []byte("\n"), 0o644); err != nil {
			log.Errorf("unmount file failed: %s", err)
			rsp.ErrRsp(c, -2, "unmount image failed")
			return
		}

		// ro flag
		if err := os.WriteFile(roFlag, []byte(flag), 0o644); err != nil {
			log.Errorf("set ro flag failed: %s", err)
			rsp.ErrRsp(c, -2, "set ro flag failed")
			return
		}

		// cdrom flag
		if err := os.WriteFile(cdromFlag, []byte(flag), 0o644); err != nil {
			log.Errorf("set cdrom flag failed: %s", err)
			rsp.ErrRsp(c, -2, "set cdrom flag failed")
			return
		}
	}

	inquiryVen := "NanoKVM"
	inquiryPrd := "USB Mass Storage"
	inquiryVer := 0x0520
	if req.Cdrom {
		inquiryPrd = "USB CD/DVD-ROM"
	}
	inquiryData := fmt.Sprintf("%-8s%-16s%04x", inquiryVen, inquiryPrd, inquiryVer)

	if err := os.WriteFile(inquiryString, []byte(inquiryData), 0o644); err != nil {
		log.Errorf("set inquiry %s failed: %s", inquiryData, err)
		rsp.ErrRsp(c, -2, "set inquiry failed")
		return
	}

	// mount
	image, err := safeImagePath(req.File)
	if err != nil {
		rsp.ErrRsp(c, -2, "invalid arguments")
		return
	}
	if image == "" {
		image = imageNone
	}

	if err := os.WriteFile(mountDevice, []byte(image), 0o644); err != nil {
		log.Errorf("mount file %s failed: %s", image, err)
		rsp.ErrRsp(c, -2, "mount image failed")
		return
	}

	h := hid.GetHid()
	h.Lock()
	h.CloseNoLock()
	defer func() {
		h.OpenNoLock()
		h.Unlock()
	}()

	// reset usb
	if err := os.WriteFile("/sys/kernel/config/usb_gadget/g0/UDC", []byte(""), 0o644); err != nil {
		rsp.ErrRsp(c, -2, "execute command failed")
		return
	}
	udcs, err := os.ReadDir("/sys/class/udc")
	if err != nil || len(udcs) == 0 {
		rsp.ErrRsp(c, -2, "execute command failed")
		return
	}
	if err := os.WriteFile("/sys/kernel/config/usb_gadget/g0/UDC", []byte(udcs[0].Name()), 0o644); err != nil {
		rsp.ErrRsp(c, -2, "execute command failed")
		return
	}

	rsp.OkRsp(c)
	log.Debugf("mount image success")
}

func (s *Service) GetMountedImage(c *gin.Context) {
	var rsp proto.Response

	content, err := os.ReadFile(mountDevice)
	if err != nil {
		rsp.ErrRsp(c, -2, "read failed")
		return
	}

	image := strings.ReplaceAll(string(content), "\n", "")
	if image == imageNone {
		image = ""
	}

	data := &proto.GetMountedImageRsp{
		File: image,
	}

	rsp.OkRspWithData(c, data)
}

func (s *Service) GetCdRom(c *gin.Context) {
	var rsp proto.Response

	content, err := os.ReadFile(cdromFlag)
	if err != nil {
		rsp.ErrRsp(c, -1, "read failed")
		return
	}

	flag := strings.ReplaceAll(string(content), "\n", "")
	flatInt, err := strconv.ParseInt(flag, 10, 64)
	if err != nil {
		rsp.ErrRsp(c, -2, "parse failed")
		return
	}

	data := &proto.GetCdRomRsp{
		Cdrom: flatInt,
	}

	rsp.OkRspWithData(c, data)
}

func (s *Service) DeleteImage(c *gin.Context) {
	var req proto.DeleteImageReq
	var rsp proto.Response

	if err := proto.ParseFormRequest(c, &req); err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	filename, pathErr := safeImagePath(req.File)
	if pathErr != nil {
		rsp.ErrRsp(c, -2, "invalid arguments")
		return
	}
	validSuffix := strings.HasSuffix(filename, ".iso") || strings.HasSuffix(filename, ".img")

	if !validSuffix {
		rsp.ErrRsp(c, -2, "invalid arguments")
		return
	}

	if err := os.Remove(filename); err != nil {
		rsp.ErrRsp(c, -3, "remove file failed")
		log.Errorf("failed to remove image: %s", err)
		return
	}

	rsp.OkRsp(c)
	log.Debugf("delete image success")
}

func safeImagePath(name string) (string, error) {
	if name == "" {
		return "", nil
	}
	clean := filepath.Clean(name)
	if filepath.IsAbs(clean) {
		rel, err := filepath.Rel(imageDirectory, clean)
		if err != nil {
			return "", err
		}
		clean = rel
	}
	target := filepath.Join(imageDirectory, clean)
	rel, err := filepath.Rel(imageDirectory, target)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", os.ErrPermission
	}
	return target, nil
}
