package tailscale

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"NanoKVM-Server/utils"

	log "github.com/sirupsen/logrus"
)

const (
	OriginalURL = "https://pkgs.tailscale.com/stable/tailscale_latest_riscv64.tgz"
	Workspace   = "/root/.tailscale"

	maxTailscaleDownloadBytes = 128 << 20
	tailscaleHTTPTimeout      = 30 * time.Second
)

func isInstalled() bool {
	_, err1 := os.Stat(TailscalePath)
	_, err2 := os.Stat(TailscaledPath)

	return err1 == nil && err2 == nil
}

func install() error {
	_ = os.MkdirAll(Workspace, 0o700)
	defer func() {
		_ = os.RemoveAll(Workspace)
	}()

	tarFile, err := workspacePath("tailscale_riscv64.tgz")
	if err != nil {
		return err
	}

	// download
	if err := download(tarFile); err != nil {
		log.Errorf("failed to download tailscale: %s", err)
		return err
	}

	// decompress
	dir, err := utils.UnTarGz(tarFile, Workspace)
	if err != nil {
		log.Errorf("failed to decompress tailscale: %s", err)
		return err
	}

	// move
	tailscalePath := fmt.Sprintf("%s/tailscale", dir)
	err = utils.MoveFile(tailscalePath, TailscalePath)
	if err != nil {
		log.Errorf("failed to move tailscale: %s", err)
		return err
	}

	tailscaledPath := fmt.Sprintf("%s/tailscaled", dir)
	err = utils.MoveFile(tailscaledPath, TailscaledPath)
	if err != nil {
		log.Errorf("failed to move tailscaled: %s", err)
		return err
	}

	log.Debugf("install tailscale successfully")
	return nil
}

func download(target string) error {
	rawURL, err := getDownloadURL()
	if err != nil {
		log.Errorf("failed to get Tailscale download url: %s", err)
		return err
	}
	if err := validateTailscaleDownloadURL(rawURL); err != nil {
		return err
	}

	if err := downloadToFile(rawURL, target, maxTailscaleDownloadBytes); err != nil {
		return err
	}

	log.Debugf("download Tailscale successfully")
	return nil
}

func downloadToFile(rawURL string, target string, maxBytes int64) error {
	client := &http.Client{Timeout: tailscaleHTTPTimeout}
	resp, err := client.Get(rawURL)
	if err != nil {
		log.Errorf("failed to download Tailscale: %s", err)
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	out, err := os.OpenFile(target, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		log.Errorf("failed to create file: %s", err)
		return err
	}
	defer func() {
		_ = out.Close()
	}()

	limited := &io.LimitedReader{R: resp.Body, N: maxBytes + 1}
	_, err = io.Copy(out, limited)
	if err != nil {
		log.Errorf("failed to copy response body to file: %s", err)
		return err
	}
	if limited.N == 0 {
		return errors.New("tailscale download exceeds maximum size")
	}

	return nil
}

func getDownloadURL() (string, error) {
	client := &http.Client{Timeout: tailscaleHTTPTimeout}
	resp, err := client.Get(OriginalURL)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return resp.Request.URL.String(), nil
}

func validateTailscaleDownloadURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("tailscale download URL must use https")
	}
	host := parsed.Hostname()
	if host != "pkgs.tailscale.com" && !strings.HasSuffix(host, ".tailscale.com") {
		return fmt.Errorf("unexpected tailscale download host")
	}
	return nil
}

func workspacePath(name string) (string, error) {
	if name == "" || filepath.Base(name) != name || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid workspace filename")
	}
	workspace := filepath.Clean(Workspace)
	target := filepath.Join(workspace, name)
	rel, err := filepath.Rel(workspace, target)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("workspace path escapes directory")
	}
	return target, nil
}
