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

// FindFirstTrade 获取第一笔交易记录
func (r TradeRepo) FindFirstTrade(ctx context.Context) (*models.Trade, error) {
	var trade models.Trade
	db := r.GetDB(ctx)
	err := db.Table(r.GetTableName()).
		Order("executed_at ASC").
		First(&trade).Error
	if err != nil {
		return nil, err
	}
	return &trade, nil
}

// TradeStats 交易统计数据
type TradeStats struct {
	TotalTrades   int     `json:"total_trades,omitempty"`   // 总交易数
	CloseTrades   int     `json:"close_trades,omitempty"`   // 平仓交易数
	WinningTrades int     `json:"winning_trades,omitempty"` // 盈利交易数
	LosingTrades  int     `json:"losing_trades,omitempty"`  // 亏损交易数
	WinRate       float64 `json:"win_rate,omitempty"`       // 胜率(%)
	TotalPnl      float64 `json:"total_pnl,omitempty"`      // 总盈亏
	TotalFee      float64 `json:"total_fee,omitempty"`      // 总手续费
	AvgWin        float64 `json:"avg_win,omitempty"`        // 平均盈利
	AvgLoss       float64 `json:"avg_loss,omitempty"`       // 平均亏损
	LargestWin    float64 `json:"largest_win,omitempty"`    // 最大盈利
	LargestLoss   float64 `json:"largest_loss,omitempty"`   // 最大亏损
	ProfitFactor  float64 `json:"profit_factor,omitempty"`  // 盈亏比(总盈利/总亏损)
}

// GetTradeStats 获取交易统计数据
func (r TradeRepo) GetTradeStats(ctx context.Context) (*TradeStats, error) {
	db := r.GetDB(ctx)

	stats := &TradeStats{}

	// 获取总交易数
	var totalCount int64
	if err := db.Table(r.GetTableName()).Count(&totalCount).Error; err != nil {
		return nil, err
	}
	stats.TotalTrades = int(totalCount)

	// 获取所有平仓交易
	var closeTrades []models.Trade
	if err := db.Table(r.GetTableName()).
		Where("type = ?", "close").
		Find(&closeTrades).Error; err != nil {
		return nil, err
	}

	stats.CloseTrades = len(closeTrades)

	// 如果没有平仓交易,直接返回
	if stats.CloseTrades == 0 {
		return stats, nil
	}

	// 计算各项统计数据
	var totalWin, totalLoss float64
	stats.LargestWin = 0
	stats.LargestLoss = 0

	for _, trade := range closeTrades {
		stats.TotalPnl += trade.Pnl
		stats.TotalFee += trade.Fee

		if trade.Pnl > 0 {
			stats.WinningTrades++
			totalWin += trade.Pnl
			if trade.Pnl > stats.LargestWin {
				stats.LargestWin = trade.Pnl
			}
		} else if trade.Pnl < 0 {
			stats.LosingTrades++
			totalLoss += trade.Pnl
			if trade.Pnl < stats.LargestLoss {
				stats.LargestLoss = trade.Pnl
			}
		}
	}

	// 计算胜率
	if stats.CloseTrades > 0 {
		stats.WinRate = float64(stats.WinningTrades) / float64(stats.CloseTrades) * 100
	}

	// 计算平均盈利和亏损
	if stats.WinningTrades > 0 {
		stats.AvgWin = totalWin / float64(stats.WinningTrades)
	}
	if stats.LosingTrades > 0 {
		stats.AvgLoss = totalLoss / float64(stats.LosingTrades)
	}

	// 计算盈亏比
	if totalLoss < 0 {
		stats.ProfitFactor = totalWin / (-totalLoss)
	}

	return stats, nil
}
