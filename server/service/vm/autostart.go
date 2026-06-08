package vm

import (
	"NanoKVM-Server/proto"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

const autostartDirectory = "/etc/kvm/autostart"
const maxAutostartContent = 64 * 1024

func (s *Service) GetAutostart(c *gin.Context) {
	var rsp proto.Response

	var files []string
	err := filepath.Walk(autostartDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			files = append(files, info.Name())
		}

		return nil
	})
	if err != nil {
		rsp.ErrRsp(c, -1, "get autostart directory fail")
		return
	}
	rsp.OkRspWithData(c, &proto.GetAutostartRsp{
		Files: files,
	})

	log.Debugf("get autostart total %d", len(files))
}

func (s *Service) UploadAutostart(c *gin.Context) {
	var req proto.UploadAutostartReq
	var rsp proto.Response

	fileName := c.Param("name")
	target, cleanName, err := safeAutostartPath(fileName)
	if err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	if err := proto.ParseFormRequest(c, &req); err != nil {
		rsp.ErrRsp(c, -1, "parse form request fail")
		return
	}
	if len(req.Content) > maxAutostartContent {
		rsp.ErrRsp(c, -1, "content too large")
		return
	}

	if _, err := os.Stat(autostartDirectory); err != nil {
		_ = os.MkdirAll(autostartDirectory, 0o700)
	}

	f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o500)
	if err != nil {
		rsp.ErrRsp(c, -1, "create file fail")
		return
	}

	defer f.Close()
	_, err = f.WriteString(req.Content)
	if err != nil {
		rsp.ErrRsp(c, -1, "write content fail")
		return
	}

	rsp.OkRspWithData(c, cleanName)

	log.Debugf("upload autostart success")
}

func (s *Service) DeleteAutostart(c *gin.Context) {
	var rsp proto.Response

	fileName := c.Param("name")

	file, _, err := safeAutostartPath(fileName)
	if err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}
	if err := os.Remove(file); err != nil {
		log.Errorf("delete autostart file fail")
		rsp.ErrRsp(c, -3, "remove file fail")
		return
	}

	rsp.OkRsp(c)
	log.Debugf("delete autostart success")
}

func (s *Service) GetAutostartContent(c *gin.Context) {
	var rsp proto.Response
	fileName := c.Param("name")
	file, _, err := safeAutostartPath(fileName)
	if err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}
	content, err := os.ReadFile(file)
	if err != nil {
		rsp.ErrRsp(c, -1, "read file fail")
		return
	}

	rsp.OkRspWithData(c, string(content))
	log.Debugf("get autostart content success")
}

func safeAutostartPath(name string) (string, string, error) {
	cleanName := filepath.Base(name)
	if cleanName != name || strings.TrimSpace(cleanName) == "" {
		return "", "", os.ErrPermission
	}
	target := filepath.Join(autostartDirectory, cleanName)
	rel, err := filepath.Rel(autostartDirectory, target)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", "", os.ErrPermission
	}
	return target, cleanName, nil
}
