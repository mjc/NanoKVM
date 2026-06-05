package hid

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"NanoKVM-Server/proto"
	"NanoKVM-Server/service/usb"
)

const (
	ModeNormal  = "normal"
	ModeHidOnly = "hid-only"
)

var modeMap = map[string]string{
	"0x0510": ModeNormal,
	"0x0623": ModeHidOnly,
}

func (s *Service) GetHidMode(c *gin.Context) {
	var rsp proto.Response

	mode, err := getHidMode()
	if err != nil {
		rsp.ErrRsp(c, -1, "get HID mode failed")
		return
	}

	rsp.OkRspWithData(c, &proto.GetHidModeRsp{
		Mode: mode,
	})
	log.Debugf("get hid mode: %s", mode)
}

func (s *Service) SetHidMode(c *gin.Context) {
	var req proto.SetHidModeReq
	var rsp proto.Response

	if err := proto.ParseFormRequest(c, &req); err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}
	if req.Mode != ModeNormal && req.Mode != ModeHidOnly {
		rsp.ErrRsp(c, -2, "invalid arguments")
		return
	}

	if mode, _ := getHidMode(); req.Mode == mode {
		rsp.OkRsp(c)
		return
	}

	h := GetHid()
	h.Lock()
	h.CloseNoLock()
	defer func() {
		h.OpenNoLock()
		h.Unlock()
	}()

	srcScript := usb.NormalInitScript
	if req.Mode == ModeHidOnly {
		srcScript = usb.HIDOnlyInitScript
	}

	if err := copyModeFile(srcScript); err != nil {
		rsp.ErrRsp(c, -3, "operation failed")
		return
	}

	rsp.OkRsp(c)

	log.Println("reboot system...")
	time.Sleep(500 * time.Millisecond)
	_ = exec.Command("reboot").Run()
}

func (s *Service) ResetHid(c *gin.Context) {
	var rsp proto.Response

	if err := ResetUSBPHY(); err != nil {
		log.Errorf("failed to reset hid: %v", err)
		rsp.ErrRsp(c, -1, "failed to reset hid")
		return
	}

	rsp.OkRsp(c)
	log.Debugf("reset hid success")
}

func (s *Service) RecoverUSB(c *gin.Context) {
	var rsp proto.Response

	if err := ResetUSBPHY(); err != nil {
		log.Errorf("failed to recover usb: %v", err)
		rsp.ErrRsp(c, -1, "failed to recover usb")
		return
	}

	rsp.OkRsp(c)
	log.Debugf("recover usb success")
}

func ResetUSBPHY() error {
	return usb.RestartPHY(GetHid())
}

func copyModeFile(srcScript string) error {
	// open the source file
	srcFile, err := os.Open(srcScript)
	if err != nil {
		log.Errorf("failed to open %s: %s", srcScript, err)
		return err
	}
	defer func() {
		_ = srcFile.Close()
	}()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		log.Errorf("failed to get %s info: %s", srcScript, err)
		return err
	}

	// create and copy to temporary file
	tmpFile, err := os.CreateTemp("/etc/init.d/", ".S03usbdev-")
	if err != nil {
		log.Errorf("failed to create temp %s: %s", usb.ActiveInitScript, err)
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	log.Debugf("create temporary file: %s", tmpPath)

	if err := tmpFile.Chmod(srcInfo.Mode()); err != nil {
		_ = tmpFile.Close()
		log.Errorf("failed to set %s mode: %s", tmpPath, err)
		return err
	}

	if _, err := io.Copy(tmpFile, srcFile); err != nil {
		_ = tmpFile.Close()
		log.Errorf("failed to copy %s: %s", srcScript, err)
		return err
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		log.Errorf("failed to sync %s: %s", tmpPath, err)
		return err
	}

	if err := tmpFile.Close(); err != nil {
		log.Errorf("failed to close %s: %s", tmpPath, err)
		return err
	}

	// replace the target file with the temporary file
	if err := os.Rename(tmpPath, usb.ActiveInitScript); err != nil {
		log.Errorf("failed to rename %s: %s", tmpPath, err)
		return err
	}

	log.Debugf("copy %s to %s successful", srcScript, usb.ActiveInitScript)
	return nil
}

func getHidMode() (string, error) {
	data, err := os.ReadFile(usb.ModeFlag)
	if err != nil {
		log.Errorf("failed to read %s: %s", usb.ModeFlag, err)
		return "", err
	}

	key := strings.TrimSpace(string(data))
	mode, ok := modeMap[key]
	if !ok {
		log.Errorf("invalid mode flag: %s", key)
		return "", errors.New("invalid mode flag")
	}

	return mode, nil
}
