package vm

import (
	"fmt"
	"os"
	"strings"
	"time"

	"NanoKVM-Server/proto"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

const (
	SwapFile    = "/swapfile"
	InittabPath = "/etc/inittab"
	TempInittab = "/etc/.inittab.tmp"
)

func (s *Service) GetSwap(c *gin.Context) {
	var rsp proto.Response

	rsp.OkRspWithData(c, &proto.GetSwapRsp{
		Size: getSwapSize(),
	})
}

func (s *Service) SetSwap(c *gin.Context) {
	var rsp proto.Response
	var req proto.SetSwapReq

	if err := proto.ParseFormRequest(c, &req); err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	size := getSwapSize()
	if req.Size == size {
		rsp.OkRsp(c)
		return
	}

	if req.Size == 0 {
		if err := disableSwap(); err != nil {
			rsp.ErrRsp(c, -2, "disable swap failed")
			return
		}
		if err := disableInittab(); err != nil {
			rsp.ErrRsp(c, -3, "disable inittab failed")
			return
		}
	} else {
		if err := enableSwap(req.Size); err != nil {
			rsp.ErrRsp(c, -4, "enable swap failed")
			return
		}
		if err := enableInittab(); err != nil {
			rsp.ErrRsp(c, -5, "enable inittab failed")
			return
		}
	}

	rsp.OkRsp(c)
}

func getSwapSize() int64 {
	fileInfo, err := os.Stat(SwapFile)
	if err != nil {
		return 0
	}

	return fileInfo.Size() / 1024 / 1024
}

func enableSwap(size int64) error {
	if getSwapSize() > 0 {
		if err := disableSwap(); err != nil {
			return err
		}
	}

	commands := buildSwapCommands(size)

	if err := runCommandSpecs(commands, 300*time.Millisecond); err != nil {
		log.Errorf("failed to configure swap commands: %s", err)
		return err
	}

	log.Debugf("set swap file size: %d", size)
	return nil
}

func buildSwapCommands(size int64) []commandSpec {
	return []commandSpec{
		{name: "fallocate", args: []string{"-l", fmt.Sprintf("%dM", size), SwapFile}},
		{name: "chmod", args: []string{"600", SwapFile}},
		{name: "mkswap", args: []string{SwapFile}},
		{name: "swapon", args: []string{SwapFile}},
	}
}

func disableSwap() error {
	if err := runCommandSpecs([]commandSpec{{name: "swapoff", args: []string{"-a"}}}, 0); err != nil {
		log.Errorf("failed to execute swapoff: %s", err)
		return err
	}

	if err := os.Remove(SwapFile); err != nil {
		log.Errorf("failed to delete %s: %s", SwapFile, err)
		return err
	}

	return nil
}

func enableInittab() error {
	f, err := os.OpenFile(InittabPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Errorf("read inittab failed: %s", err)
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	content := fmt.Sprintf("\nsi11::sysinit:/sbin/swapon %s", SwapFile)
	_, err = f.WriteString(content)
	if err != nil {
		log.Errorf("write inittab failed: %s", err)
		return err
	}

	log.Debugf("write to %s: %s", InittabPath, content)
	return nil
}

func disableInittab() error {
	defer func() {
		_ = os.Remove(TempInittab)
	}()

	input, err := os.ReadFile(InittabPath)
	if err != nil {
		log.Errorf("read fstab failed: %s", err)
		return err
	}

	lines := strings.Split(string(input), "\n")
	output := make([]string, 0)

	for _, line := range lines {
		if strings.HasSuffix(line, SwapFile) {
			log.Debugf("%s delete line: %s", InittabPath, line)
		} else {
			output = append(output, line)
		}
	}

	content := strings.Join(output, "\n")
	content = strings.TrimSuffix(content, "\n")
	if err := os.WriteFile(TempInittab, []byte(content), 0644); err != nil {
		log.Errorf("write temp fstab failed: %s", err)
		return err
	}

	if err := os.Rename(TempInittab, InittabPath); err != nil {
		log.Errorf("replace fstab failed: %s", err)
		return err
	}

	return nil
}
