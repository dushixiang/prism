package models

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// Position 持仓信息
type Position struct {
	ID               string         `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Symbol           string         `gorm:"not null;index" json:"symbol"`      // 交易对,如 BTCUSDT
	Side             string         `gorm:"not null" json:"side"`              // long/short
	Quantity         float64        `gorm:"not null" json:"quantity"`          // 持仓数量
	EntryPrice       float64        `gorm:"not null" json:"entry_price"`       // 开仓价格
	CurrentPrice     float64        `json:"current_price"`                     // 当前价格
	LiquidationPrice float64        `json:"liquidation_price"`                 // 强平价格
	UnrealizedPnl    float64        `json:"unrealized_pnl"`                    // 未实现盈亏(USDT)
	Leverage         int            `gorm:"not null" json:"leverage"`          // 杠杆倍数
	Margin           float64        `json:"margin"`                            // 保证金(USDT)
	OrderID          string         `json:"order_id"`                          // 开仓订单ID
	EntryReason      string         `json:"entry_reason"`                      // 开仓理由
	ExitPlan         string         `json:"exit_plan"`                         // 退出条件/计划
	StopLoss         float64        `json:"stop_loss"`                         // 止损价格
	TakeProfit       float64        `json:"take_profit"`                       // 止盈价格
	PeakPnlPercent   float64        `gorm:"default:0" json:"peak_pnl_percent"` // 历史最高盈亏百分比
	OpenedAt         time.Time      `gorm:"not null" json:"opened_at"`         // 开仓时间
	CreatedAt        time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName 指定表名
func (*Position) TableName() string {
	return "positions"
}

// CalculatePnlPercent 计算盈亏百分比(考虑杠杆)
func (p *Position) CalculatePnlPercent() float64 {
	if p.EntryPrice == 0 {
		return 0
	}

	priceChange := (p.CurrentPrice - p.EntryPrice) / p.EntryPrice * 100

	// 做空时价格变动反向
	if p.Side == "short" {
		priceChange = -priceChange
	}

	// 盈亏百分比 = 价格变动百分比 × 杠杆
	return priceChange * float64(p.Leverage)
}

func (p *Position) CalculateHoldingStr() string {
	holding := time.Since(p.OpenedAt)
	holdingStr, _ := strings.CutSuffix(holding.Round(time.Minute).String(), "0s")
	return holdingStr
}
