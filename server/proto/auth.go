package proto

type LoginReq struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type LoginRsp struct {
	Token string `json:"token"`
}

type GetAccountRsp struct {
	Username string `json:"username"`
}

type ChangePasswordReq struct {
	Username    string `json:"username" validate:"required"`
	OldPassword string `json:"oldPassword" validate:"required"`
	Password    string `json:"password" validate:"required"`
}

type IsPasswordUpdatedRsp struct {
	IsUpdated bool `json:"isUpdated"`
}
