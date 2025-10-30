package exchange

// 通用交易类型定义，独立于任何特定交易所
// 这样可以方便地支持多个交易所（币安、OKX、Bybit等）

// OrderSide 订单方向
type OrderSide string

const (
	OrderSideBuy  OrderSide = "BUY"
	OrderSideSell OrderSide = "SELL"
)

// PositionSide 持仓方向
type PositionSide string

const (
	PositionSideLong  PositionSide = "long"
	PositionSideShort PositionSide = "short"
)

// MarginType 保证金类型
type MarginType string

const (
	MarginTypeCrossed  MarginType = "CROSSED"  // 全仓
	MarginTypeIsolated MarginType = "ISOLATED" // 逐仓
)

// OrderType 订单类型
type OrderType string

const (
	OrderTypeLimit  OrderType = "LIMIT"  // 限价单
	OrderTypeMarket OrderType = "MARKET" // 市价单
)

// OrderStatus 订单状态
type OrderStatus string

const (
	OrderStatusNew             OrderStatus = "NEW"
	OrderStatusPartiallyFilled OrderStatus = "PARTIALLY_FILLED"
	OrderStatusFilled          OrderStatus = "FILLED"
	OrderStatusCanceled        OrderStatus = "CANCELED"
	OrderStatusRejected        OrderStatus = "REJECTED"
	OrderStatusExpired         OrderStatus = "EXPIRED"
)

// String 方法用于日志输出
func (s OrderSide) String() string {
	return string(s)
}

func (s PositionSide) String() string {
	return string(s)
}

func (m MarginType) String() string {
	return string(m)
}

func (o OrderType) String() string {
	return string(o)
}

func (o OrderStatus) String() string {
	return string(o)
}
