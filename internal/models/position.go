package models

import (
	"time"

	"gorm.io/gorm"
)

// Position 持仓信息
type Position struct {
	ID               string         `gorm:"primaryKey;type:varchar(26)" json:"id"`
	Symbol           string         `gorm:"type:varchar(20);not null;index" json:"symbol"`        // 交易对，如 BTCUSDT
	Side             string         `gorm:"type:varchar(10);not null" json:"side"`                // long/short
	Quantity         float64        `gorm:"type:decimal(20,8);not null" json:"quantity"`          // 持仓数量
	EntryPrice       float64        `gorm:"type:decimal(20,8);not null" json:"entry_price"`       // 开仓价格
	CurrentPrice     float64        `gorm:"type:decimal(20,8)" json:"current_price"`              // 当前价格
	LiquidationPrice float64        `gorm:"type:decimal(20,8)" json:"liquidation_price"`          // 强平价格
	UnrealizedPnl    float64        `gorm:"type:decimal(20,8)" json:"unrealized_pnl"`             // 未实现盈亏（USDT）
	Leverage         int            `gorm:"type:int;not null" json:"leverage"`                    // 杠杆倍数
	Margin           float64        `gorm:"type:decimal(20,8)" json:"margin"`                     // 保证金（USDT）
	OrderID          string         `gorm:"type:varchar(50)" json:"order_id"`                     // 开仓订单ID
	EntryReason      string         `gorm:"type:text" json:"entry_reason"`                        // 开仓理由
	ExitPlan         string         `gorm:"type:text" json:"exit_plan"`                           // 退出条件/计划
	StopLoss         float64        `gorm:"type:decimal(20,8)" json:"stop_loss"`                  // 止损价格
	TakeProfit       float64        `gorm:"type:decimal(20,8)" json:"take_profit"`                // 止盈价格
	PeakPnlPercent   float64        `gorm:"type:decimal(10,4);default:0" json:"peak_pnl_percent"` // 历史最高盈亏百分比
	OpenedAt         time.Time      `gorm:"not null" json:"opened_at"`                            // 开仓时间
	CreatedAt        time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName 指定表名
func (Position) TableName() string {
	return "positions"
}

// CalculatePnlPercent 计算盈亏百分比（考虑杠杆）
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

// CalculateHoldingHours 计算持仓小时数
func (p *Position) CalculateHoldingHours() float64 {
	return time.Since(p.OpenedAt).Hours()
}

// CalculateHoldingCycles 计算持仓周期数（假设每10分钟一个周期）
func (p *Position) CalculateHoldingCycles() int {
	minutes := time.Since(p.OpenedAt).Minutes()
	return int(minutes / 10)
}

// RemainingHours 计算距离36小时的剩余时间
func (p *Position) RemainingHours() float64 {
	remaining := 36 - p.CalculateHoldingHours()
	if remaining < 0 {
		return 0
	}
	return remaining
}
