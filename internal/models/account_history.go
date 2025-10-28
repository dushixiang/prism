package models

import (
	"time"

	"gorm.io/gorm"
)

// AccountHistory 账户历史记录
type AccountHistory struct {
	ID                  string         `gorm:"primaryKey;type:varchar(26)" json:"id"`
	TotalBalance        float64        `gorm:"type:decimal(20,8);not null" json:"total_balance"` // 总资产（不含未实现盈亏）
	Available           float64        `gorm:"type:decimal(20,8)" json:"available"`              // 可用余额
	UnrealisedPnl       float64        `gorm:"type:decimal(20,8)" json:"unrealised_pnl"`         // 未实现盈亏
	InitialBalance      float64        `gorm:"type:decimal(20,8)" json:"initial_balance"`        // 初始资金
	PeakBalance         float64        `gorm:"type:decimal(20,8)" json:"peak_balance"`           // 峰值资金
	ReturnPercent       float64        `gorm:"type:decimal(10,4)" json:"return_percent"`         // 收益率
	DrawdownFromPeak    float64        `gorm:"type:decimal(10,4)" json:"drawdown_from_peak"`     // 从峰值的回撤
	DrawdownFromInitial float64        `gorm:"type:decimal(10,4)" json:"drawdown_from_initial"`  // 从初始的回撤
	SharpeRatio         float64        `gorm:"type:decimal(10,4)" json:"sharpe_ratio"`           // 夏普比率
	Iteration           int            `gorm:"type:int;index" json:"iteration"`                  // 交易周期数
	RecordedAt          time.Time      `gorm:"not null;index" json:"recorded_at"`                // 记录时间
	CreatedAt           time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt           time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName 指定表名
func (AccountHistory) TableName() string {
	return "account_histories"
}
