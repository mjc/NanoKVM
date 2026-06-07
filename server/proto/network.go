package proto

type WakeOnLANReq struct {
	Mac string `json:"mac" validate:"required,mac"`
}

type GetMacRsp struct {
	Macs []string `json:"macs"`
}

type DeleteMacReq struct {
	Mac string `json:"mac" validate:"required,mac"`
}

type SetMacNameReq struct {
	Mac  string `json:"mac" validate:"required,mac"`
	Name string `json:"name" validate:"required,max=64"`
}

type GetWifiRsp struct {
	Supported bool   `json:"supported"`
	ApMode    bool   `json:"apMode"`
	Connected bool   `json:"connected"`
	Ssid      string `json:"ssid"`
}

type ConnectWifiReq struct {
	Ssid     string `json:"ssid" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type GetDNSRsp struct {
	Mode      string   `json:"mode"`
	Servers   []string `json:"servers"`
	Effective []string `json:"effective"`
	DHCP      []string `json:"dhcp"`
	Info      DNSInfo  `json:"info"`
}

type SetDNSReq struct {
	Mode    string   `json:"mode" validate:"required,oneof=manual dhcp"`
	Servers []string `json:"servers" validate:"omitempty,dive,ip"`
}

type DNSInfo struct {
	Interface     string   `json:"interface"`
	Type          string   `json:"type"`
	Address       string   `json:"address"`
	SubnetMask    string   `json:"subnetMask"`
	Gateway       string   `json:"gateway"`
	SearchDomains []string `json:"searchDomains"`
}
