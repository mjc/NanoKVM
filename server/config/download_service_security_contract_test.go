package config

import "testing"

func TestDownloadServiceSecurityContracts(t *testing.T) {
	cases := []sourceSecurityContract{
		{
			name:       "DownloadImageEnabledDefersCloseBeforeNilCheck",
			path:       "../service/download/service.go",
			vulnerable: []string{`file, err := os.Create(testFile)`, `defer file.Close()`},
			message:    "image-enabled probe should not defer Close before checking whether os.Create returned a file",
		},
		{
			name:       "DownloadImageEnabledCreatesProbeWithDefaultPermissions",
			path:       "../service/download/service.go",
			vulnerable: []string{`file, err := os.Create(testFile)`},
			message:    "image-enabled probe should use explicit restrictive permissions and symlink-safe creation",
		},
		{
			name:       "DownloadStatusReadsPredictableSentinelWholeFile",
			path:       "../service/download/service.go",
			vulnerable: []string{`content, err := os.ReadFile(sentinelPath)`},
			message:    "download status should not read unbounded content from a predictable /tmp sentinel",
		},
		{
			name:       "DownloadStatusSplitsUntrustedSentinel",
			path:       "../service/download/service.go",
			vulnerable: []string{`splitted := strings.Split(string(content), ";")`},
			message:    "download status should parse sentinel state through a structured private file",
		},
		{
			name:       "DownloadImageFileStartSentinelWorldReadable",
			path:       "../service/download/service.go",
			vulnerable: []string{`os.WriteFile(sentinelPath, []byte("start"), 0644)`},
			message:    "multipart image upload sentinel should not be world-readable",
		},
		{
			name:       "DownloadImageFileUsesRequestContentLengthForProgress",
			path:       "../service/download/service.go",
			vulnerable: []string{`lw = &loggingWriter{writer: out, totalSize: c.Request.ContentLength}`},
			message:    "multipart image upload should not trust request Content-Length for progress or limits",
		},
		{
			name:       "DownloadImageFileReadsSentinelBeforeUpload",
			path:       "../service/download/service.go",
			vulnerable: []string{`data, err := os.ReadFile(sentinelPath)`},
			message:    "multipart image upload should not coordinate state through a shared readable sentinel",
		},
		{
			name:       "DownloadImageFileSentinelUsesContainsStart",
			path:       "../service/download/service.go",
			vulnerable: []string{`if strings.Contains(string(data), "start") {`},
			message:    "multipart image upload sentinel state should use exact structured states instead of substring checks",
		},
		{
			name:       "DownloadImageFileSentinelUsesContainsFilename",
			path:       "../service/download/service.go",
			vulnerable: []string{`if !strings.Contains(string(data), filename) {`},
			message:    "multipart image upload sentinel state should not authorize progress by substring filename matching",
		},
		{
			name:       "DownloadImageFileWritesFilenameSentinelWorldReadable",
			path:       "../service/download/service.go",
			vulnerable: []string{`os.WriteFile(sentinelPath, []byte(filename), 0644)`},
			message:    "multipart image upload should not write uploaded filenames into a world-readable /tmp sentinel",
		},
		{
			name:       "DownloadImageFileValidatesISOAfterWritingFullFile",
			path:       "../service/download/service.go",
			vulnerable: []string{`ok, err := isISO9660(outPath)`},
			message:    "multipart image upload should validate type during streaming and enforce size before writing a full file",
		},
		{
			name:       "DownloadImageRemoteHeadNoTimeout",
			path:       "../service/download/service.go",
			vulnerable: []string{`resp, err := http.Head(req.File)`},
			message:    "remote image HEAD checks should use an HTTP client with explicit timeouts",
		},
		{
			name:       "DownloadImageRemoteGetCreatesDefaultPermissionFile",
			path:       "../service/download/service.go",
			vulnerable: []string{`out, err := os.Create(destPath)`},
			message:    "remote image downloads should create destination files with explicit permissions",
		},
		{
			name:       "DownloadProgressWriterCanDeadlockOnStop",
			path:       "../service/download/service.go",
			vulnerable: []string{`lw.done <- true`},
			message:    "download progress stop should close a done channel or use context cancellation instead of blocking sends",
		},
		{
			name:       "DownloadProgressWritesSentinelWorldReadable",
			path:       "../service/download/service.go",
			vulnerable: []string{`os.WriteFile(sentinelPath, []byte(fmt.Sprintf("%s;%.2f%%", splitted[0], percentage)), 0644)`},
			message:    "download progress should not write progress into a world-readable /tmp sentinel",
		},
		{
			name:       "DownloadStatusExposesRequestedURL",
			path:       "../service/download/service.go",
			vulnerable: []string{"File:       req.File"},
			message:    "download status should not expose arbitrary requested URLs back to all authenticated clients",
		},
		{
			name:       "DownloadSentinelStoresRequestedURL",
			path:       "../service/download/service.go",
			vulnerable: []string{"os.WriteFile(sentinelPath, []byte(req.File), 0644)"},
			message:    "download sentinel should not store arbitrary URLs in a world-readable temp file",
		},
		{
			name:       "DownloadSentinelUsesPredictableTmpPath",
			path:       "../service/download/service.go",
			vulnerable: []string{`sentinelPath = "/tmp/.download_in_progress"`},
			message:    "download coordination should not use a predictable /tmp path",
		},
		{
			name:       "DownloadURLAllowsNonHTTPS",
			path:       "../service/download/service.go",
			vulnerable: []string{`u.Scheme == "" || u.Host == ""`},
			fixedBy:    []string{`u.Scheme != "https"`},
			message:    "remote image download should require HTTPS URLs",
		},
		{
			name:       "DownloadURLDoesNotBlockPrivateNetworks",
			path:       "../service/download/service.go",
			vulnerable: []string{"http.Head(req.File)"},
			fixedBy:    []string{"IsPrivate", "IsLoopback", "netip"},
			message:    "remote image download should block SSRF to loopback/private networks",
		},
		{
			name:       "DownloadHeadChecksResponseBeforeError",
			path:       "../service/download/service.go",
			vulnerable: []string{"if resp.StatusCode != http.StatusOK || err != nil"},
			message:    "download URL validation should handle nil responses before dereferencing status",
		},
		{
			name:       "DownloadRunsHTTPGetInBackgroundWithGinResponse",
			path:       "../service/download/service.go",
			vulnerable: []string{"go func() {", "rsp.ErrRsp(c, -1"},
			message:    "background downloads should not write Gin responses after the handler returns",
		},
		{
			name:       "DownloadImageUsesHTTPGetWithoutClientTimeout",
			path:       "../service/download/service.go",
			vulnerable: []string{"http.Get(req.File)"},
			fixedBy:    []string{"Timeout:"},
			message:    "remote image downloads should use an HTTP client with explicit timeouts",
		},
		{
			name:       "DownloadImageHasNoSizeCap",
			path:       "../service/download/service.go",
			vulnerable: []string{"io.Copy(lw, resp.Body)"},
			fixedBy:    []string{"LimitReader", "ContentLength >"},
			message:    "remote image downloads should enforce a maximum size",
		},
		{
			name:       "DownloadImageFileStopTickerCanPanicBeforeInit",
			path:       "../service/download/service.go",
			vulnerable: []string{"var lw *loggingWriter", "lw.stopTicker()"},
			message:    "multipart image upload should not call stopTicker on an uninitialized logging writer",
		},
		{
			name:       "DownloadImageFileCreatesPredictableDataPath",
			path:       "../service/download/service.go",
			vulnerable: []string{`outPath := "/data/" + filename`},
			message:    "multipart image upload should safe-join target paths",
		},
		{
			name:       "DownloadImageFileHasNoStreamingSizeLimit",
			path:       "../service/download/service.go",
			vulnerable: []string{"io.Copy(lw, part)"},
			fixedBy:    []string{"LimitReader", "MaxBytesReader"},
			message:    "multipart image upload should enforce a maximum size while streaming",
		},
		{
			name:       "DownloadProgressPercentageCanDivideByZero",
			path:       "../service/download/service.go",
			vulnerable: []string{"percentage := float64(lw.total) / float64(lw.totalSize) * 100"},
			message:    "download progress should handle unknown or zero content length safely",
		},
	}

	runSourceSecurityContracts(t, cases, 28, "download-service")
}
