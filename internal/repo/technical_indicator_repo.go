package repo

import (
	"context"

	"github.com/dushixiang/prism/internal/models"
	"github.com/go-orz/orz"
	"gorm.io/gorm"
)

func NewTechnicalIndicatorRepo(db *gorm.DB) *TechnicalIndicatorRepo {
	return &TechnicalIndicatorRepo{
		Repository: orz.NewRepository[models.TechnicalIndicator, string](db),
	}
}

type TechnicalIndicatorRepo struct {
	orz.Repository[models.TechnicalIndicator, string]
}

// FindLatestBySymbolAndTimeframe 获取最新的技术指标
func (r TechnicalIndicatorRepo) FindLatestBySymbolAndTimeframe(ctx context.Context, symbol, timeframe string) (m models.TechnicalIndicator, err error) {
	db := r.GetDB(ctx)
	err = db.Table(r.GetTableName()).
		Where("symbol = ? AND timeframe = ?", symbol, timeframe).
		Order("calculated_at DESC").
		First(&m).Error
	return m, err
}
