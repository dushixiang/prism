package models

import (
	"time"

	"gorm.io/gorm"
)

// Trade 交易记录
type Trade struct {
	ID         string         `gorm:"primaryKey;type:varchar(26)" json:"id"`
	Symbol     string         `gorm:"type:varchar(20);not null;index" json:"symbol"` // 交易对
	Type       string         `gorm:"type:varchar(10);not null" json:"type"`         // open/close
	Side       string         `gorm:"type:varchar(10);not null" json:"side"`         // long/short
	Price      float64        `gorm:"type:decimal(20,8);not null" json:"price"`      // 成交价格
	Quantity   float64        `gorm:"type:decimal(20,8);not null" json:"quantity"`   // 成交数量
	Leverage   int            `gorm:"type:int" json:"leverage"`                      // 杠杆倍数
	Fee        float64        `gorm:"type:decimal(20,8)" json:"fee"`                 // 手续费
	Pnl        float64        `gorm:"type:decimal(20,8)" json:"pnl"`                 // 平仓盈亏（仅平仓时有值）
	OrderID    string         `gorm:"type:varchar(50);index" json:"order_id"`        // 订单ID
	PositionID string         `gorm:"type:varchar(26);index" json:"position_id"`     // 关联的持仓ID
	ExecutedAt time.Time      `gorm:"not null;index" json:"executed_at"`             // 执行时间
	CreatedAt  time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName 指定表名
func (Trade) TableName() string {
	return "trades"
}
