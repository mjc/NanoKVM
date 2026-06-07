package proto

type IP struct {
	Name    string `json:"name"`
	Addr    string `json:"addr"`
	Version string `json:"version"`
	Type    string `json:"type"`
}

type GetInfoRsp struct {
	IPs         []IP   `json:"ips"`
	Mdns        string `json:"mdns"`
	Image       string `json:"image"`
	Application string `json:"application"`
	DeviceKey   string `json:"deviceKey"`
}

type GetHardwareRsp struct {
	Version string `json:"version"`
}

type SetGpioReq struct {
	Type     string `json:"type" validate:"required,oneof=reset power"`    // reset / power
	Duration uint   `json:"duration" validate:"omitempty,min=0,max=30000"` // press time (unit: milliseconds)
}

type GetGpioRsp struct {
	PWR bool `json:"pwr"` // power led
	HDD bool `json:"hdd"` // hdd led
}

type SetScreenReq struct {
	Type  string `json:"type" validate:"required,oneof=resolution fps quality"` // resolution / fps / quality
	Value int    `json:"value" validate:"number,min=0,max=10000"`               // value
}

type GetScriptsRsp struct {
	Files []string `json:"files"`
}

type UploadScriptRsp struct {
	File string `json:"file"`
}

type RunScriptReq struct {
	Name string `json:"name" validate:"required"`
	Type string `json:"type" validate:"required,oneof=foreground background"` // foreground | background
}

type RunScriptRsp struct {
	Log string `json:"log"`
}

type DeleteScriptReq struct {
	Name string `json:"name" validate:"required"`
}

// autostart
type GetAutostartRsp struct {
	Files []string `json:"files"`
}

type UploadAutostartReq struct {
	Content string `json:"content"`
}

type GetVirtualDeviceRsp struct {
	Network bool `json:"network"`
	Media   bool `json:"media"`
	Disk    bool `json:"disk"`
}

type UpdateVirtualDeviceReq struct {
	Device string `json:"device" validate:"required,oneof=network media disk"`
}

type UpdateVirtualDeviceRsp struct {
	On bool `json:"on"`
}

type SetMemoryLimitReq struct {
	Enabled bool  `json:"enabled" validate:"omitempty"`
	Limit   int64 `json:"limit" validate:"omitempty,min=0,max=1073741824"`
}

type GetMemoryLimitRsp struct {
	Enabled bool  `json:"enabled"`
	Limit   int64 `json:"limit"`
}

type SetOledReq struct {
	Sleep int `json:"sleep" validate:"omitempty,min=0,max=86400"`
}

type GetOLEDRsp struct {
	Exist bool `json:"exist"`
	Sleep int  `json:"sleep"`
}

type GetGetHdmiStateRsp struct {
	Enabled bool `json:"enabled"`
}

type GetSSHStateRsp struct {
	Enabled bool `json:"enabled"`
}

type GetSwapRsp struct {
	Size int64 `json:"size"` // unit: MB
}

type SetSwapReq struct {
	Size int64 `json:"size" validate:"omitempty,min=0,max=65536"` // unit: MB
}

type GetMouseJigglerRsp struct {
	Enabled bool   `json:"enabled"`
	Mode    string `json:"mode"`
}

type SetMouseJigglerReq struct {
	Enabled bool   `json:"enabled" validate:"omitempty"`
	Mode    string `json:"mode" validate:"omitempty,oneof=random circle"`
}

type GetMdnsStateRsp struct {
	Enabled bool `json:"enabled"`
}

type SetHostnameReq struct {
	Hostname string `json:"hostname" validate:"required,hostname,max=253"`
}

type GetHostnameRsp struct {
	Hostname string `json:"hostname"`
}

type SetWebTitleReq struct {
	Title string `json:"title" validate:"omitempty,max=64"`
}

type GetWebTitleRsp struct {
	Title string `json:"title"`
}

type SetTlsReq struct {
	Enabled bool `json:"enabled" validate:"omitempty"`
}
