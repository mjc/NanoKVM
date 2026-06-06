package vm

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"NanoKVM-Server/proto"
	"NanoKVM-Server/service/auth"
	"NanoKVM-Server/utils"
)

const (
	ScriptDirectory            = "/etc/kvm/scripts"
	MaxScriptFileSize          = 1 * 1024 * 1024
	MaxScriptOutputSize        = 1 * 1024 * 1024
	maxScriptUploadRequestSize = MaxScriptFileSize + 64*1024
)

var (
	scriptNamePattern                = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	errScriptOutputTooLarge          = errors.New("script output too large")
	isPasswordUpdatedForScriptRunner = auth.IsDefaultPasswordChanged
)

func (s *Service) GetScripts(c *gin.Context) {
	var rsp proto.Response

	if rejectDefaultPasswordScriptAccess(c, &rsp) {
		return
	}

	files, err := getScriptFiles(ScriptDirectory)
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

	if rejectDefaultPasswordScriptAccess(c, &rsp) {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxScriptUploadRequestSize)

	header, err := c.FormFile("file")
	if err != nil {
		rsp.ErrRsp(c, -1, "bad request")
		return
	}

	if err := validateUploadedScriptSize(header.Size); err != nil {
		rsp.ErrRsp(c, -2, "invalid arguments")
		return
	}

	filename, target, err := resolveScriptPath(header.Filename)
	if err != nil {
		rsp.ErrRsp(c, -2, "invalid arguments")
		return
	}

	if err := os.MkdirAll(ScriptDirectory, 0o755); err != nil {
		rsp.ErrRsp(c, -2, "save failed")
		return
	}

	if err := validateScriptTargetForWrite(target); err != nil {
		rsp.ErrRsp(c, -2, "save failed")
		return
	}

	if err := c.SaveUploadedFile(header, target); err != nil {
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

	if rejectDefaultPasswordScriptAccess(c, &rsp) {
		return
	}

	if err := proto.ParseFormRequest(c, &req); err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	filename, target, err := resolveScriptPath(req.Name)
	if err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	var output []byte
	switch req.Type {
	case "foreground":
		output, err = runScriptForeground(filename, target)
	case "background":
		err = runScriptBackground(filename, target)
	default:
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	if err != nil {
		log.Errorf("run script %s failed: %s", filename, err)
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

	if rejectDefaultPasswordScriptAccess(c, &rsp) {
		return
	}

	if err := proto.ParseFormRequest(c, &req); err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	filename, target, err := resolveScriptDeletePath(req.Name)
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
	switch strings.ToLower(filepath.Ext(name)) {
	case ".sh", ".py":
		return true
	default:
		return false
	}
}

func resolveScriptPath(name string) (string, string, error) {
	filename := strings.TrimSpace(name)
	if filename != name || !validScriptName(filename) {
		return "", "", fmt.Errorf("invalid script name: %s", name)
	}

	return scriptTarget(filename)
}

func resolveScriptDeletePath(name string) (string, string, error) {
	if !validScriptBasename(name) {
		return "", "", fmt.Errorf("invalid script name: %s", name)
	}

	return scriptTarget(name)
}

func validScriptName(name string) bool {
	return validScriptBasename(name) && scriptNamePattern.MatchString(name)
}

func validScriptBasename(name string) bool {
	return name != "" &&
		!strings.ContainsRune(name, '\x00') &&
		!filepath.IsAbs(name) &&
		filepath.Base(name) == name &&
		isScript(name)
}

func scriptTarget(filename string) (string, string, error) {
	target := filepath.Join(ScriptDirectory, filename)
	if err := ensurePathInside(ScriptDirectory, target); err != nil {
		return "", "", err
	}

	return filename, target, nil
}

func getScriptFiles(root string) ([]string, error) {
	var files []string

	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return files, nil
	}
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.Mode().IsRegular() && isScript(info.Name()) {
			files = append(files, info.Name())
		}
	}

	return files, nil
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

func runScriptForeground(filename string, target string) ([]byte, error) {
	if err := validateScriptTargetForRun(target); err != nil {
		return nil, err
	}

	output, err := runCommandCombinedOutput(scriptCommand(filename, target))
	if shouldRetryShellScriptWithSh(filename, err) {
		return runCommandCombinedOutput(exec.Command("sh", target))
	}

	return output, err
}

func runScriptBackground(filename string, target string) error {
	if err := validateScriptTargetForRun(target); err != nil {
		return err
	}

	cmd := scriptCommand(filename, target)
	err := cmd.Start()
	if shouldRetryShellScriptWithSh(filename, err) {
		cmd = exec.Command("sh", target)
		err = cmd.Start()
	}

	if err != nil {
		return err
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			log.Errorf("run script %s in background failed: %s", filename, err)
		}
	}()

	return nil
}

func shouldRetryShellScriptWithSh(filename string, err error) bool {
	return err != nil &&
		strings.HasSuffix(strings.ToLower(filename), ".sh") &&
		errors.Is(err, syscall.ENOEXEC)
}

func rejectDefaultPasswordScriptAccess(c *gin.Context, rsp *proto.Response) bool {
	allowed, err := isPasswordUpdatedForScriptRunner()
	if err != nil {
		log.Errorf("check script runner password state failed: %s", err)
		rsp.ErrRsp(c, -3, "check password failed")
		return true
	}
	if !allowed {
		rsp.ErrRsp(c, -3, "change default password first")
		return true
	}

	return false
}

func validateUploadedScriptSize(size int64) error {
	if size < 0 || size > MaxScriptFileSize {
		return fmt.Errorf("script upload too large")
	}

	return nil
}

func validateScriptTargetForWrite(target string) error {
	info, err := os.Lstat(target)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	return validateRegularScriptTarget(info)
}

func validateScriptTargetForRun(target string) error {
	info, err := os.Lstat(target)
	if err != nil {
		return err
	}

	return validateRegularScriptTarget(info)
}

func validateRegularScriptTarget(info os.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("invalid script target")
	}

	return nil
}

func runCommandCombinedOutput(cmd *exec.Cmd) ([]byte, error) {
	var output limitedScriptOutput
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()
	if output.truncated || errors.Is(err, errScriptOutputTooLarge) {
		return output.Bytes(), errScriptOutputTooLarge
	}

	return output.Bytes(), err
}

type limitedScriptOutput struct {
	bytes.Buffer
	truncated bool
}

func (b *limitedScriptOutput) Write(p []byte) (int, error) {
	remaining := MaxScriptOutputSize - b.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}

	if len(p) > remaining {
		_, _ = b.Buffer.Write(p[:remaining])
		b.truncated = true
		return len(p), nil
	}

	return b.Buffer.Write(p)
}
