package download

import (
	"NanoKVM-Server/proto"
	"NanoKVM-Server/utils"
	"encoding/json"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Service struct{}

const maxImageBytes = 8 << 30

var sentinelPath = "/etc/kvm/download_state.json"

type downloadState struct {
	Status     string `json:"status"`
	Label      string `json:"label"`
	Percentage string `json:"percentage"`
}

func writeDownloadState(state downloadState) {
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	_ = utils.WritePrivateFile(sentinelPath, data)
}

func readDownloadState() (downloadState, error) {
	data, err := utils.ReadPrivateFile(sentinelPath)
	if err != nil {
		return downloadState{}, err
	}
	var state downloadState
	if err := json.Unmarshal(data, &state); err != nil {
		return downloadState{}, err
	}
	return state, nil
}

func NewService() *Service {
	// Clear sentinel
	// If we are starting from scratch, we need to remove the sentinel file as any downloads at this point are done or broken
	_ = os.Remove(sentinelPath)
	return &Service{}
}

func (s *Service) ImageEnabled(c *gin.Context) {
	var rsp proto.Response

	// Check if /data mount is RO/RW
	testFile := "/data/.testfile"
	file, err := os.OpenFile(testFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsPermission(err) {
			rsp.OkRspWithData(c, &proto.ImageEnabledRsp{
				Enabled: false,
			})
			return
		}
		rsp.OkRspWithData(c, &proto.ImageEnabledRsp{
			Enabled: false,
		})
		return
	}
	defer file.Close()
	defer os.Remove(testFile)

	rsp.OkRspWithData(c, &proto.ImageEnabledRsp{
		Enabled: true,
	})
}

func isISO9660(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// ISO-9660 Magic "CD001" bei Offset 32769
	_, err = f.Seek(0x8001, io.SeekStart)
	if err != nil {
		return false, err
	}

	buf := make([]byte, 5)
	_, err = io.ReadFull(f, buf)
	if err != nil {
		return false, err
	}

	return string(buf) == "CD001", nil
}

func (s *Service) StatusImage(c *gin.Context) {
	var rsp proto.Response

	// Check if the sentinel file exists
	log.Debug("StatusImage")
	if _, err := os.Stat(sentinelPath); err == nil {
		state, err := readDownloadState()
		if err != nil {
			log.Error("Failed to read sentinel file")
			rsp.OkRspWithData(c, &proto.StatusImageRsp{
				Status:     "in_progress",
				File:       "",
				Percentage: "",
			})
			return
		}
		rsp.OkRspWithData(c, &proto.StatusImageRsp{
			Status:     state.Status,
			File:       state.Label,
			Percentage: state.Percentage,
		})

		return
	}
	rsp.OkRspWithData(c, &proto.StatusImageRsp{
		Status:     "idle",
		File:       "",
		Percentage: "",
	})
}

func (s *Service) DownloadImageFile(c *gin.Context) {
	var rsp proto.Response

	log.Debug("DownloadImage")

	// Set a sentinel file to mark that there is a download in progress
	// This is to prevent multiple downloads at the same time
	if _, err := os.Stat(sentinelPath); err == nil {
		log.Debug("Download in progress")
		rsp.ErrRsp(c, -1, "download in progress")
		return
	}

	// Create the sentinel file
	writeDownloadState(downloadState{Status: "start"})

	// Multipart Reader direkt nutzen (keine FormFile!)
	reader, err := c.Request.MultipartReader()
	if err != nil {
		log.Error("invalid multipart data")
		rsp.ErrRsp(c, -1, "invalid multipart data")
		defer os.Remove(sentinelPath)
		return
	}

	lw := newLoggingWriter(nil, 0)

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Error("failed to read part")
			rsp.ErrRsp(c, -1, "failed to read part")
			stopLoggingWriter(lw)
			defer os.Remove(sentinelPath)
			return
		}

		if part.FormName() != "file" {
			continue
		}

		filename := part.FileName()
		if filename == "" {
			log.Error("no filename")
			rsp.ErrRsp(c, -1, "no filename")
			stopLoggingWriter(lw)
			defer os.Remove(sentinelPath)
			return
		}

		filename = filepath.Base(filename)

		if filename != part.FileName() {
			log.Warn("path detected in filename")
			rsp.ErrRsp(c, -1, "invalid filename")
			defer os.Remove(sentinelPath)
			return
		}

		if strings.Contains(filename, "..") {
			log.Warn("path traversal attempt")
			rsp.ErrRsp(c, -1, "invalid filename")
			defer os.Remove(sentinelPath)
			return
		}

		if !strings.HasSuffix(strings.ToLower(filename), ".iso") {
			rsp.ErrRsp(c, -1, "only .iso files allowed")
			defer os.Remove(sentinelPath)
			return
		}

		valid := regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
		if !valid.MatchString(filename) {
			rsp.ErrRsp(c, -1, "invalid filename")
			defer os.Remove(sentinelPath)
			return
		}

		state, err := readDownloadState()
		if err != nil {
			log.Error("Read failed")
			rsp.ErrRsp(c, -1, "Read failed")
			stopLoggingWriter(lw)
			defer os.Remove(sentinelPath)
			return
		}

		outPath := filepath.Join("/data", filename)
		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			log.Error("cannot create file")
			rsp.ErrRsp(c, -1, "cannot create file")
			stopLoggingWriter(lw)
			defer os.Remove(sentinelPath)
			return
		}
		defer out.Close()

		if state.Status == "start" {
			writeDownloadState(downloadState{Status: "in_progress", Label: filename})
			lw = &loggingWriter{writer: out, totalSize: maxImageBytes}
			lw.startTicker()
		} else {
			if state.Label != filename {
				log.Error("failed")
				rsp.ErrRsp(c, -1, "failed")
				stopLoggingWriter(lw)
				defer os.Remove(outPath)
				defer os.Remove(sentinelPath)
				return
			}
		}

		// Direkt streamen → kein RAM-Bedarf außer kleinem Buffer
		_, err = io.Copy(lw, io.LimitReader(part, maxImageBytes))
		if err != nil {
			log.Error("write failed")
			rsp.ErrRsp(c, -1, "write failed")
			stopLoggingWriter(lw)
			defer os.Remove(outPath)
			defer os.Remove(sentinelPath)
			return
		}

		isoOK, err := isISO9660(outPath)
		if err != nil || !isoOK {
			rsp.ErrRsp(c, -1, "file is not a valid ISO image")
			stopLoggingWriter(lw)
			defer os.Remove(outPath)
			defer os.Remove(sentinelPath)
			return
		}
	}
	stopLoggingWriter(lw)

	rsp.OkRspWithData(c, &proto.StatusImageRsp{
		Status:     "idle",
		File:       "",
		Percentage: "",
	})

	defer os.Remove(sentinelPath)
	return
}

func (s *Service) DownloadImage(c *gin.Context) {
	var req proto.MountImageReq
	var rsp proto.Response

	log.Debug("DownloadImage")

	if err := proto.ParseFormRequest(c, &req); err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	if req.File == "" {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}
	// Parse the URI to see if its valid http/s
	u, err := url.Parse(req.File)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		rsp.ErrRsp(c, -1, "invalid url")
		return
	}
	if blocksPrivateAddress(u.Hostname()) {
		rsp.ErrRsp(c, -1, "invalid url")
		return
	}

	// Set a sentinel file to mark that there is a download in progress
	// This is to prevent multiple downloads at the same time
	if _, err := os.Stat(sentinelPath); err == nil {
		log.Debug("Download in progress")
		rsp.ErrRsp(c, -1, "download in progress")
		return
	}
	// Create the sentinel file
	writeDownloadState(downloadState{Status: "in_progress", Label: "remote"})

	// Check if it actually exists and fail if it doesn't
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Head(req.File)
	if err != nil || resp == nil || resp.StatusCode != http.StatusOK {
		rsp.ErrRsp(c, -1, "failed when checking the url")
		log.Error("Failed to check the URL")
		defer os.Remove(sentinelPath)
		return
	}
	defer resp.Body.Close()
	if resp.ContentLength > maxImageBytes {
		rsp.ErrRsp(c, -1, "file too large")
		defer os.Remove(sentinelPath)
		return
	}

	// Download the image asynchronously to avoid blocking the request.
	go downloadRemoteImage(client, req.File, filepath.Base(u.Path))
	rsp.OkRspWithData(c, &proto.StatusImageRsp{
		Status:     "in_progress",
		File:       "remote",
		Percentage: "",
	})
}

func blocksPrivateAddress(host string) bool {
	ips, err := net.LookupIP(host)
	if err != nil {
		return true
	}
	for _, ip := range ips {
		addr, ok := netip.AddrFromSlice(ip)
		if !ok || addr.IsLoopback() || addr.IsPrivate() {
			return true
		}
	}
	return false
}

type loggingWriter struct {
	writer    io.Writer
	total     int64
	totalSize int64
	ticker    *time.Ticker
	done      chan struct{}
}

func newLoggingWriter(writer io.Writer, totalSize int64) *loggingWriter {
	return &loggingWriter{writer: writer, totalSize: totalSize}
}

func stopLoggingWriter(lw *loggingWriter) {
	if lw != nil {
		lw.stopTicker()
	}
}

func (lw *loggingWriter) startTicker() {
	lw.ticker = time.NewTicker(2500 * time.Millisecond)
	lw.done = make(chan struct{})
	go runLoggingWriter(lw)
}

func runLoggingWriter(lw *loggingWriter) {
	for {
		select {
		case <-lw.done:
			return
		case <-lw.ticker.C:
			lw.updateSentinel()
		}
	}
}

func (lw *loggingWriter) stopTicker() {
	if lw == nil || lw.ticker == nil {
		return
	}
	lw.ticker.Stop()
	close(lw.done)
}

func (lw *loggingWriter) updateSentinel() {
	if lw.totalSize <= 0 {
		return
	}
	ratio := float64(lw.total) / float64(lw.totalSize)
	percentage := ratio * 100
	writeDownloadState(downloadState{Status: "in_progress", Percentage: strconv.FormatFloat(percentage, 'f', 2, 64) + "%"})
}

func (lw *loggingWriter) Write(p []byte) (int, error) {
	n, err := lw.writer.Write(p)
	lw.total += int64(n)
	return n, err
}

func downloadRemoteImage(client *http.Client, sourceURL string, destName string) {
	defer os.Remove(sentinelPath)
	resp, err := client.Get(sourceURL)
	if err != nil {
		log.Error("Failed to download the file")
		return
	}
	defer resp.Body.Close()

	destPath := filepath.Join("/data", destName)
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		log.Error("Failed to create destination file")
		return
	}
	defer out.Close()

	lw := newLoggingWriter(out, resp.ContentLength)
	lw.startTicker()
	if _, err = io.Copy(lw, io.LimitReader(resp.Body, maxImageBytes)); err != nil {
		log.Error("Failed to save the file")
		stopLoggingWriter(lw)
		return
	}
	stopLoggingWriter(lw)
}
