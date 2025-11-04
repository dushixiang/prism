package models

import (
	"time"

	"gorm.io/gorm"
)

// Trade 交易记录
type Trade struct {
	ID         string         `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Symbol     string         `gorm:"not null;index" json:"symbol"`      // 交易对
	Type       string         `gorm:"not null" json:"type"`              // open/close
	Side       string         `gorm:"not null" json:"side"`              // long/short
	Price      float64        `gorm:"not null" json:"price"`             // 成交价格
	Quantity   float64        `gorm:"not null" json:"quantity"`          // 成交数量
	Leverage   int            `json:"leverage"`                          // 杠杆倍数
	Fee        float64        `json:"fee"`                               // 手续费
	Pnl        float64        `json:"pnl"`                               // 平仓盈亏(仅平仓时有值)
	Reason     string         `json:"reason"`                            // 开仓/平仓原因
	OrderID    string         `gorm:"index" json:"order_id"`             // 订单ID
	PositionID string         `gorm:"index" json:"position_id"`          // 关联的持仓ID
	ExecutedAt time.Time      `gorm:"not null;index" json:"executed_at"` // 执行时间
	CreatedAt  time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName 指定表名
func (Trade) TableName() string {
	return "trades"
}
