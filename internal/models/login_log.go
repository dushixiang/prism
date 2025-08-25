package models

type LoginLog struct {
	ID           string `gorm:"primary_key,type:varchar(191)" json:"id"`
	Account      string `gorm:"type:varchar(200)" json:"account"`
	IP           string `gorm:"type:varchar(200)" json:"ip"`
	UserAgentRaw string `json:"-"`
	LoginAt      int64  `json:"loginAt"`
	Success      bool   `json:"success"`
	Reason       string `gorm:"type:varchar(500)" json:"reason"`

	Region    string `gorm:"-" json:"region"`
	UserAgent any    `gorm:"-" json:"userAgent"`
}

func (r *LoginLog) TableName() string {
	return "login_log"
}
