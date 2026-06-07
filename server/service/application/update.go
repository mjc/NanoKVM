package application

import (
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"NanoKVM-Server/proto"
	"NanoKVM-Server/utils"
)

const (
	maxTries              = 3
	maxUpdatePackageBytes = 512 << 20
	updateHTTPTimeout     = 30 * time.Second
)

func (s *Service) Update(c *gin.Context) {
	var rsp proto.Response

	if !acquireUpdateLock() {
		rsp.ErrRsp(c, -1, "update already in progress")
		return
	}
	defer releaseUpdateLock()

	if err := update(); err != nil {
		rsp.ErrRsp(c, -1, fmt.Sprintf("update failed: %s", err))
		return
	}

	rsp.OkRsp(c)
	log.Debugf("update application success")

	// Sleep for a second before restarting the device
	time.Sleep(1 * time.Second)

	_ = restartNanoKVM()
}

func update() error {
	_ = os.RemoveAll(CacheDir)
	_ = os.MkdirAll(CacheDir, 0o700)
	defer func() {
		_ = os.RemoveAll(CacheDir)
	}()

	// get latest information
	latest, err := getLatest()
	if err != nil {
		return err
	}

	// download
	target, err := safeCachePath(latest.Name)
	if err != nil {
		return err
	}
	if err := download(latest.Url, target); err != nil {
		log.Errorf("download app failed: %s", err)
		return err
	}

	// check sha512
	if err := checksum(target, latest.Sha512); err != nil {
		log.Errorf("check sha512 failed: %s", err)
		return err
	}

	// install
	if err := installPackage(target); err != nil {
		log.Errorf("failed to install package: %v", err)
		return err
	}

	return nil
}

func download(url string, target string) (err error) {
	if err := validateUpdateURL(url); err != nil {
		return err
	}

	for i := range maxTries {
		log.Debugf("attempt #%d/%d", i+1, maxTries)
		if i > 0 {
			time.Sleep(time.Second * 3)
		}

		var req *http.Request
		req, err = http.NewRequest("GET", url, nil)
		if err != nil {
			log.Errorf("new request err: %s", err)
			continue
		}

		err = utils.Download(req, target)
		if err != nil {
			log.Errorf("downloading latest application failed, try again...")
			continue
		}
		return nil
	}
	return err
}

func checksum(filePath string, expectedHash string) error {
	file, err := os.Open(filePath)
	if err != nil {
		log.Errorf("failed to open file %s: %v", filePath, err)
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	hasher := sha512.New()

	_, err = io.Copy(hasher, file)
	if err != nil {
		log.Errorf("failed to copy file contents to hasher: %v", err)
		return err
	}

	hash := base64.StdEncoding.EncodeToString(hasher.Sum(nil))

	if hash != expectedHash {
		log.Error("invalid sha512")
		return fmt.Errorf("invalid sha512")
	}

	return nil
}

func safeCachePath(filename string) (string, error) {
	if err := validateFilename(filename); err != nil {
		return "", err
	}

	cacheDir := filepath.Clean(CacheDir)
	target := filepath.Join(cacheDir, filename)
	rel, err := filepath.Rel(cacheDir, target)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("invalid cache path")
	}
	return target, nil
}

func validateUpdateURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("update URL must use https")
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("update URL missing host")
	}
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return fmt.Errorf("update URL host is not public")
	}
	if ip, err := netip.ParseAddr(host); err == nil && !isPublicIP(ip) {
		return fmt.Errorf("update URL host is not public")
	}
	if ip := net.ParseIP(host); ip != nil {
		addr, ok := netip.AddrFromSlice(ip)
		if ok && !isPublicIP(addr) {
			return fmt.Errorf("update URL host is not public")
		}
	}
	return nil
}

func isPublicIP(ip netip.Addr) bool {
	return ip.IsValid() &&
		!ip.IsLoopback() &&
		!ip.IsPrivate() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsMulticast() &&
		!ip.IsUnspecified()
}
