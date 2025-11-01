package models

import (
	"time"

	"gorm.io/gorm"
)

// OrderType 订单类型
type OrderType string

const (
	OrderTypeStopLoss   OrderType = "stop_loss"   // 止损单
	OrderTypeTakeProfit OrderType = "take_profit" // 止盈单
)

// OrderStatus 订单状态
type OrderStatus string

const (
	OrderStatusActive    OrderStatus = "active"    // 活跃中
	OrderStatusTriggered OrderStatus = "triggered" // 已触发
	OrderStatusCanceled  OrderStatus = "canceled"  // 已取消
	OrderStatusFailed    OrderStatus = "failed"    // 失败
)

// Order 限价订单（止损/止盈）
type Order struct {
	ID           string         `gorm:"primaryKey;type:varchar(26)" json:"id"`
	Symbol       string         `gorm:"type:varchar(20);not null;index" json:"symbol"`            // 交易对
	PositionID   string         `gorm:"type:varchar(26);not null;index" json:"position_id"`       // 关联的持仓ID
	PositionSide string         `gorm:"type:varchar(10);not null" json:"position_side"`           // 持仓方向 (long/short)
	OrderType    OrderType      `gorm:"type:varchar(20);not null" json:"order_type"`              // 订单类型 (stop_loss/take_profit)
	TriggerPrice float64        `gorm:"type:decimal(20,8);not null" json:"trigger_price"`         // 触发价格
	Quantity     float64        `gorm:"type:decimal(20,8);not null" json:"quantity"`              // 订单数量
	ExchangeID   string         `gorm:"type:varchar(50)" json:"exchange_id"`                      // 交易所订单ID
	Status       OrderStatus    `gorm:"type:varchar(20);not null;default:'active'" json:"status"` // 订单状态
	Reason       string         `gorm:"type:text" json:"reason"`                                  // 创建/更新原因
	CreatedAt    time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	TriggeredAt  *time.Time     `json:"triggered_at,omitempty"` // 触发时间
	CanceledAt   *time.Time     `json:"canceled_at,omitempty"`  // 取消时间
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName 指定表名
func (*Order) TableName() string {
	return "orders"
}

// IsStopLoss 是否是止损单
func (o *Order) IsStopLoss() bool {
	return o.OrderType == OrderTypeStopLoss
}

// IsTakeProfit 是否是止盈单
func (o *Order) IsTakeProfit() bool {
	return o.OrderType == OrderTypeTakeProfit
}

// IsActive 是否活跃
func (o *Order) IsActive() bool {
	return o.Status == OrderStatusActive
}

// CalculateDistancePercent 计算触发价格与当前价格的距离百分比
func (o *Order) CalculateDistancePercent(currentPrice float64) float64 {
	if currentPrice == 0 {
		return 0
	}
	return (o.TriggerPrice - currentPrice) / currentPrice * 100
}
