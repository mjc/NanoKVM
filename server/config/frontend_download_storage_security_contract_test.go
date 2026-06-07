package config

import "testing"

func TestFrontendDownloadStorageSecurityContracts(t *testing.T) {
	cases := []sourceSecurityContract{
		{
			name:       "StorageAPIMountImageSendsRawPath",
			path:       "../../web/src/api/storage.ts",
			vulnerable: []string{`return http.post('/api/storage/image/mount', data);`},
			message:    "storage API should send opaque image ids instead of raw filesystem paths",
		},
		{
			name:       "StorageAPIDeleteImageSendsRawPath",
			path:       "../../web/src/api/storage.ts",
			vulnerable: []string{`return http.post('/api/storage/image/delete', data);`},
			message:    "storage API should delete images by server-issued ids instead of raw filesystem paths",
		},
		{
			name:       "DownloadAPISendsRemoteURLAsFileField",
			path:       "../../web/src/api/download.ts",
			vulnerable: []string{`file: file ? file : ''`},
			message:    "download API should distinguish remote URLs from local storage paths in typed payloads",
		},
		{
			name:       "DownloadFrontendDisplaysRemoteURLInLog",
			path:       "../../web/src/pages/desktop/menu/download.tsx",
			vulnerable: []string{`setLog('Downloading: ' + url)`},
			message:    "download UI should avoid reflecting arbitrary remote URLs into shared status text",
		},
		{
			name:       "DownloadFrontendCopiesStatusFileIntoInput",
			path:       "../../web/src/pages/desktop/menu/download.tsx",
			vulnerable: []string{`setInput(rsp.data.file);`},
			message:    "download UI should not copy server status file fields back into a user-editable URL input",
		},
		{
			name:       "DownloadFrontendPollsSharedStatus",
			path:       "../../web/src/pages/desktop/menu/download.tsx",
			vulnerable: []string{`intervalId.current = setInterval(getDownloadStatus, 2500);`},
			message:    "download UI should poll a session-bound job id instead of a shared global status endpoint",
		},
		{
			name:       "DownloadFrontendOnlyChecksISOExtension",
			path:       "../../web/src/pages/desktop/menu/download.tsx",
			vulnerable: []string{`!file.name.toLowerCase().endsWith(".iso")`},
			message:    "download UI extension checks should not be the only client-side signal for image uploads",
		},
	}

	runSourceSecurityContracts(t, cases, 7, "frontend-download-storage")
}
