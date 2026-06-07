package config

import "testing"

func TestDownloadStorageRouteSecurityContracts(t *testing.T) {
	cases := []sourceSecurityContract{
		{
			name:       "DownloadRouterStatusEndpointUsesSameAmbientJWTOnly",
			path:       "../router/download.go",
			vulnerable: []string{`api.GET("/download/image/status", service.StatusImage)`},
			message:    "download status should bind to the initiating session instead of any valid JWT",
		},
		{
			name:       "DownloadRouterFileUploadNeedsStepUp",
			path:       "../router/download.go",
			vulnerable: []string{`api.POST("/download/file", service.DownloadImageFile)`},
			message:    "ISO upload should require step-up authorization before writing to /data",
		},
		{
			name:       "StorageMountRouteNoStepUp",
			path:       "../router/storage.go",
			vulnerable: []string{`api.POST("/storage/image/mount", service.MountImage)`},
			message:    "USB mass-storage mount should require step-up authorization",
		},
		{
			name:       "StorageDeleteRouteNoStepUp",
			path:       "../router/storage.go",
			vulnerable: []string{`api.POST("/storage/image/delete", service.DeleteImage)`},
			message:    "image deletion should require step-up authorization",
		},
	}

	runSourceSecurityContracts(t, cases, 4, "download-storage-route")
}
