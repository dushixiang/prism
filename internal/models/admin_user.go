package models

import "time"

// AdminUser 管理员用户表
type AdminUser struct {
	ID           string     `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Username     string     `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash string     `gorm:"not null" json:"-"`                      // 密码哈希,不返回给前端
	Nickname     string     `json:"nickname"`                               // 昵称
	Role         string     `gorm:"not null;default:'admin'" json:"role"`   // 角色:admin, viewer
	IsActive     bool       `gorm:"not null;default:true" json:"is_active"` // 是否激活
	LastLoginAt  *time.Time `json:"last_login_at"`                          // 最后登录时间
	LastLoginIP  string     `json:"last_login_ip"`                          // 最后登录IP
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (AdminUser) TableName() string {
	return "admin_user"
}
