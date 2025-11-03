package models

import "time"

// AdminUser 管理员用户表
type AdminUser struct {
	ID           string     `gorm:"primaryKey;size:26" json:"id"`
	Username     string     `gorm:"uniqueIndex;size:50;not null" json:"username"`
	PasswordHash string     `gorm:"size:255;not null" json:"-"`                   // 密码哈希，不返回给前端
	Nickname     string     `gorm:"size:100" json:"nickname"`                     // 昵称
	Role         string     `gorm:"size:20;not null;default:'admin'" json:"role"` // 角色：admin, viewer
	IsActive     bool       `gorm:"not null;default:true" json:"is_active"`       // 是否激活
	LastLoginAt  *time.Time `gorm:"" json:"last_login_at"`                        // 最后登录时间
	LastLoginIP  string     `gorm:"size:45" json:"last_login_ip"`                 // 最后登录IP
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (AdminUser) TableName() string {
	return "admin_user"
}
