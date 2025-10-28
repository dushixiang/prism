package repo

import (
	"context"

	"github.com/dushixiang/prism/internal/models"
	"github.com/go-orz/orz"
	"gorm.io/gorm"
)

func NewTradeRepo(db *gorm.DB) *TradeRepo {
	return &TradeRepo{
		Repository: orz.NewRepository[models.Trade, string](db),
	}
}

type TradeRepo struct {
	orz.Repository[models.Trade, string]
}

// FindRecentTrades 获取最近的交易记录
func (r TradeRepo) FindRecentTrades(ctx context.Context, limit int) ([]models.Trade, error) {
	var trades []models.Trade
	db := r.GetDB(ctx)
	err := db.Table(r.GetTableName()).
		Order("executed_at DESC").
		Limit(limit).
		Find(&trades).Error
	return trades, err
}
