package repo

import (
	"context"

	"github.com/dushixiang/prism/internal/models"
	"github.com/go-orz/orz"
	"gorm.io/gorm"
)

func NewPositionRepo(db *gorm.DB) *PositionRepo {
	return &PositionRepo{
		Repository: orz.NewRepository[models.Position, string](db),
	}
}

type PositionRepo struct {
	orz.Repository[models.Position, string]
}

// FindBySymbolAndSide 根据交易对和方向查找最近的持仓记录（包括已删除）
func (r PositionRepo) FindBySymbolAndSide(ctx context.Context, symbol, side string) (m models.Position, err error) {
	db := r.GetDB(ctx)
	err = db.Unscoped().
		Table(r.GetTableName()).
		Where("symbol = ? AND side = ?", symbol, side).
		Order("created_at DESC").
		First(&m).Error
	return m, err
}

// DeleteAll 删除所有持仓记录
func (r PositionRepo) DeleteAll(ctx context.Context) error {
	db := r.GetDB(ctx)
	return db.Where("1 = 1").Delete(&models.Position{}).Error
}

// UpdatePeakPnlPercent 更新峰值盈亏百分比
func (r PositionRepo) UpdatePeakPnlPercent(ctx context.Context, id string, pnlPercent float64) error {
	db := r.GetDB(ctx)
	return db.Table(r.GetTableName()).
		Where("id = ?", id).
		Update("peak_pnl_percent", pnlPercent).Error
}
