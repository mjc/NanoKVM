package application

import "os"

const (
	StableURL  = "https://cdn.sipeed.com/nanokvm"
	PreviewURL = "https://cdn.sipeed.com/nanokvm/preview"

	AppDir    = "/kvmapp"
	BackupDir = "/root/old"
	CacheDir  = "/root/.kvmcache"
)

var (
	stableURL  = StableURL
	previewURL = PreviewURL

	appDir    = AppDir
	backupDir = BackupDir
	cacheDir  = CacheDir

	createFile = os.Create
	mkdirAll   = os.MkdirAll
	readFile   = os.ReadFile
	removeAll  = os.RemoveAll
	removeFile = os.Remove
	statFile   = os.Stat
	writeFile  = os.WriteFile
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}
