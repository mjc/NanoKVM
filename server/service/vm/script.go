package vm

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"NanoKVM-Server/proto"
	"NanoKVM-Server/utils"
)

const ScriptDirectory = "/etc/kvm/scripts"

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

	_, header, err := c.Request.FormFile("file")
	if err != nil {
		rsp.ErrRsp(c, -1, "bad request")
		return
	}

	scriptPath, cleanName, err := safeScriptPath(header.Filename)
	if err != nil {
		rsp.ErrRsp(c, -2, "invalid arguments")
		return
	}

	if _, err = os.Stat(ScriptDirectory); err != nil {
		_ = os.MkdirAll(ScriptDirectory, 0o700)
	}

	err = c.SaveUploadedFile(header, scriptPath)
	if err != nil {
		rsp.ErrRsp(c, -2, "save failed")
		return
	}

	_ = utils.EnsurePermission(scriptPath, 0o500)

	data := &proto.UploadScriptRsp{
		File: cleanName,
	}
	rsp.OkRspWithData(c, data)

	log.Debugf("upload script success")
}

func (s *Service) RunScript(c *gin.Context) {
	var req proto.RunScriptReq
	var rsp proto.Response

	if err := proto.ParseFormRequest(c, &req); err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	scriptPath, _, err := safeScriptPath(req.Name)
	if err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	var output []byte
	var cmd *exec.Cmd
	if strings.HasSuffix(strings.ToLower(scriptPath), ".py") {
		cmd = exec.Command("python", scriptPath)
	} else {
		cmd = exec.Command(scriptPath)
	}

	if req.Type == "foreground" {
		output, err = cmd.CombinedOutput()
	} else if req.Type == "background" {
		cmd.Stdout = nil
		cmd.Stderr = nil
		go func() {
			err := cmd.Run()
			if err != nil {
				log.Errorf("run script in background failed: %s", err)
			}
		}()
	} else {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	if err != nil {
		log.Errorf("run script failed: %s", err.Error())
		rsp.ErrRsp(c, -2, "run script failed")
		return
	}

	rsp.OkRspWithData(c, &proto.RunScriptRsp{
		Log: boundedScriptOutput(output),
	})

	log.Debugf("run script success")
}

func (s *Service) DeleteScript(c *gin.Context) {
	var req proto.DeleteScriptReq
	var rsp proto.Response

	if err := proto.ParseFormRequest(c, &req); err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	file, _, err := safeScriptPath(req.Name)
	if err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	if err := os.Remove(file); err != nil {
		log.Errorf("delete script failed: %s", err)
		rsp.ErrRsp(c, -3, "delete failed")
		return
	}

	rsp.OkRsp(c)
	log.Debugf("delete script success")
}

func safeScriptPath(name string) (string, string, error) {
	cleanName := filepath.Base(name)
	if cleanName != name || !isScript(cleanName) {
		return "", "", os.ErrPermission
	}
	target := filepath.Join(ScriptDirectory, cleanName)
	rel, err := filepath.Rel(ScriptDirectory, target)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", "", os.ErrPermission
	}
	return target, cleanName, nil
}

func boundedScriptOutput(output []byte) string {
	const maxScriptOutput = 4096
	if len(output) > maxScriptOutput {
		output = output[:maxScriptOutput]
	}
	return string(output)
}

func isScript(name string) bool {
	nameLower := strings.ToLower(name)
	if strings.HasSuffix(nameLower, ".sh") || strings.HasSuffix(nameLower, ".py") {
		return true
	}

	return false
}
