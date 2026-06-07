package proto

type GetHidModeRsp struct {
	Mode string `json:"mode"` // normal or hid-only
}

type SetHidModeReq struct {
	Mode string `json:"mode" validate:"required,oneof=normal hid-only"` // normal or hid-only
}

type ShortcutKey struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

type Shortcut struct {
	ID   string        `json:"id"`
	Keys []ShortcutKey `json:"keys"`
}

type GetShortcutsRsp struct {
	Shortcuts []Shortcut `json:"shortcuts"`
}

type AddShortcutReq struct {
	Keys []ShortcutKey `json:"keys" validate:"required,dive"`
}

type DeleteShortcutReq struct {
	ID string `json:"id" validate:"required"`
}

type SetLeaderKeyReq struct {
	Key string `json:"key" validate:"omitempty"`
}

type GetLeaderKeyRsp struct {
	Key string `json:"key"`
}
