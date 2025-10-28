package repo

import (
	"context"

	"github.com/dushixiang/prism/internal/models"
	"github.com/go-orz/orz"
	"gorm.io/gorm"
)

func NewAccountHistoryRepo(db *gorm.DB) *AccountHistoryRepo {
	return &AccountHistoryRepo{
		Repository: orz.NewRepository[models.AccountHistory, string](db),
	}
}

type AccountHistoryRepo struct {
	orz.Repository[models.AccountHistory, string]
}

// FindInitialBalance 获取初始余额记录
func (r AccountHistoryRepo) FindInitialBalance(ctx context.Context) (m models.AccountHistory, err error) {
	db := r.GetDB(ctx)
	err = db.Table(r.GetTableName()).
		Order("recorded_at ASC").
		First(&m).Error
	return m, err
}

// FindPeakBalance 获取峰值余额记录
func (r AccountHistoryRepo) FindPeakBalance(ctx context.Context) (m models.AccountHistory, err error) {
	db := r.GetDB(ctx)
	err = db.Table(r.GetTableName()).
		Order("peak_balance DESC").
		First(&m).Error
	return m, err
}

// FindAllOrderByRecordedAt 获取所有账户历史记录（按时间排序）
func (r AccountHistoryRepo) FindAllOrderByRecordedAt(ctx context.Context) ([]models.AccountHistory, error) {
	var histories []models.AccountHistory
	db := r.GetDB(ctx)
	err := db.Table(r.GetTableName()).
		Order("recorded_at ASC").
		Find(&histories).Error
	return histories, err
}
