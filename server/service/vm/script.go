package vm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"NanoKVM-Server/proto"
	"NanoKVM-Server/utils"
)

const ScriptDirectory = "/etc/kvm/scripts"

var scriptNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func (s *Service) GetScripts(c *gin.Context) {
	var rsp proto.Response

	var files []string
	err := filepath.Walk(ScriptDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && isScript(info.Name()) {
			files = append(files, info.Name())
		}

		return nil
	})
	if err != nil {
		rsp.ErrRsp(c, -1, "get scripts failed")
		return
	}

	rsp.OkRspWithData(c, &proto.GetScriptsRsp{
		Files: files,
	})

	log.Debugf("get scripts total %d", len(files))
}

func (s *Service) UploadScript(c *gin.Context) {
	var rsp proto.Response

	header, err := c.FormFile("file")
	if err != nil {
		rsp.ErrRsp(c, -1, "bad request")
		return
	}

	filename, target, err := resolveScriptPath(header.Filename)
	if err != nil {
		rsp.ErrRsp(c, -2, "invalid arguments")
		return
	}

	if err = os.MkdirAll(ScriptDirectory, 0o755); err != nil {
		rsp.ErrRsp(c, -2, "save failed")
		return
	}

	err = c.SaveUploadedFile(header, target)
	if err != nil {
		rsp.ErrRsp(c, -2, "save failed")
		return
	}

	_ = utils.EnsurePermission(target, 0o100)

	data := &proto.UploadScriptRsp{
		File: filename,
	}
	rsp.OkRspWithData(c, data)

	log.Debugf("upload script %s success", filename)
}

func (s *Service) RunScript(c *gin.Context) {
	var req proto.RunScriptReq
	var rsp proto.Response

	if err := proto.ParseFormRequest(c, &req); err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	filename, target, err := resolveScriptPath(req.Name)
	if err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	if req.Type != "foreground" && req.Type != "background" {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	var output []byte
	cmd := scriptCommand(filename, target)

	if req.Type == "foreground" {
		output, err = cmd.CombinedOutput()
	} else {
		cmd.Stdout = nil
		cmd.Stderr = nil
		go func() {
			err := cmd.Run()
			if err != nil {
				log.Errorf("run script %s in background failed: %s", filename, err)
			}
		}()
	}

	if err != nil {
		log.Errorf("run script %s failed: %s", filename, err.Error())
		rsp.ErrRsp(c, -2, "run script failed")
		return
	}

	rsp.OkRspWithData(c, &proto.RunScriptRsp{
		Log: string(output),
	})

	log.Debugf("run script %s success", filename)
}

func (s *Service) DeleteScript(c *gin.Context) {
	var req proto.DeleteScriptReq
	var rsp proto.Response

	if err := proto.ParseFormRequest(c, &req); err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	filename, target, err := resolveScriptPath(req.Name)
	if err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	if err := os.Remove(target); err != nil {
		log.Errorf("delete script %s failed: %s", filename, err)
		rsp.ErrRsp(c, -3, "delete failed")
		return
	}

	rsp.OkRsp(c)
	log.Debugf("delete script %s success", filename)
}

func isScript(name string) bool {
	nameLower := strings.ToLower(name)
	if strings.HasSuffix(nameLower, ".sh") || strings.HasSuffix(nameLower, ".py") {
		return true
	}

	return false
}

func resolveScriptPath(name string) (string, string, error) {
	filename := strings.TrimSpace(name)
	if filename == "" || filename != name {
		return "", "", fmt.Errorf("empty script name")
	}

	if strings.ContainsRune(filename, '\x00') ||
		filepath.IsAbs(filename) ||
		filepath.Base(filename) != filename ||
		!scriptNamePattern.MatchString(filename) ||
		!isScript(filename) {
		return "", "", fmt.Errorf("invalid script name: %s", name)
	}

	target := filepath.Join(ScriptDirectory, filename)
	if err := ensurePathInside(ScriptDirectory, target); err != nil {
		return "", "", err
	}

	return filename, target, nil
}

func ensurePathInside(root string, target string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	absTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}

	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return err
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("script path escapes script directory")
	}

	return nil
}

func scriptCommand(filename string, target string) *exec.Cmd {
	if strings.HasSuffix(strings.ToLower(filename), ".py") {
		return exec.Command("python", target)
	}

	return exec.Command(target)
}
