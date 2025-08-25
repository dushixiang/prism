package models

type Session struct {
	ID        string   `json:"id"`
	UserId    string   `json:"userId"`
	UserType  UserType `json:"userType"`
	Remember  bool     `json:"remember"`
	CreatedAt int64    `json:"createdAt"` // 创建时间
}

func (r Session) TableName() string {
	return "session"
}

func (r Session) IsAdmin() bool {
	return r.UserType == AdminUser
}
