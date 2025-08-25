package views

type LoginAccount struct {
	Account   string `json:"account" validate:"required"`
	Password  string `json:"password" validate:"required"`
	Remember  bool   `json:"remember"`
	IP        string `json:"-"`
	UserAgent string `json:"-"`
}

type LoginOTP struct {
	Token    string `json:"token" validate:"required"`
	OTP      string `json:"otp" validate:"required"`
	Remember bool   `json:"remember"`
}

type LoginResult struct {
	Token string `json:"token"` // token
	OTP   bool   `json:"otp"`   // 是否需要OTP验证
}

type ChangePassword struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

type ChangeProfile struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}
