package repo

import (
	"context"
	"time"

	"github.com/dushixiang/prism/internal/models"
	"gorm.io/gorm"
)

// AdminUserRepo 管理员用户仓储
type AdminUserRepo struct {
	db *gorm.DB
}

// NewAdminUserRepo 创建管理员用户仓储
func NewAdminUserRepo(db *gorm.DB) *AdminUserRepo {
	return &AdminUserRepo{db: db}
}

// Create 创建用户
func (r *AdminUserRepo) Create(ctx context.Context, user *models.AdminUser) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// FindByUsername 根据用户名查找用户
func (r *AdminUserRepo) FindByUsername(ctx context.Context, username string) (*models.AdminUser, error) {
	var user models.AdminUser
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByID 根据ID查找用户
func (r *AdminUserRepo) FindByID(ctx context.Context, id string) (*models.AdminUser, error) {
	var user models.AdminUser
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateLastLogin 更新最后登录信息
func (r *AdminUserRepo) UpdateLastLogin(ctx context.Context, id string, ip string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&models.AdminUser{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_login_at": now,
			"last_login_ip": ip,
		}).Error
}

// UpdatePassword 更新密码
func (r *AdminUserRepo) UpdatePassword(ctx context.Context, id string, passwordHash string) error {
	return r.db.WithContext(ctx).Model(&models.AdminUser{}).
		Where("id = ?", id).
		Update("password_hash", passwordHash).Error
}

// FindAll 查找所有用户
func (r *AdminUserRepo) FindAll(ctx context.Context) ([]models.AdminUser, error) {
	var users []models.AdminUser
	err := r.db.WithContext(ctx).Order("created_at DESC").Find(&users).Error
	return users, err
}

// Delete 删除用户
func (r *AdminUserRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.AdminUser{}, "id = ?", id).Error
}

// CountUsers 统计用户数量
func (r *AdminUserRepo) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.AdminUser{}).Count(&count).Error
	return count, err
}
