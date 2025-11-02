package exchange

import "context"

// Exchange 交易所接口，定义所有交易所需要实现的方法
// 使用通用类型，便于支持多个交易所（币安、OKX、Bybit等）
type Exchange interface {
	// 市场数据
	GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]*Kline, error)
	GetCurrentPrice(ctx context.Context, symbol string) (float64, error)
	GetFundingRate(ctx context.Context, symbol string) (float64, error)

	// 账户信息
	GetAccountInfo(ctx context.Context) (*AccountInfo, error)
	GetPositions(ctx context.Context) ([]*Position, error)

	// 交易参数设置
	SetLeverage(ctx context.Context, symbol string, leverage int) error
	SetMarginType(ctx context.Context, symbol string, marginType MarginType) error

	// 订单操作
	CreateMarketOrder(ctx context.Context, symbol string, side OrderSide, quantity float64, reduceOnly bool) (*OrderResult, error)
	OpenLongPosition(ctx context.Context, symbol string, quantity float64) (*OrderResult, error)
	OpenShortPosition(ctx context.Context, symbol string, quantity float64) (*OrderResult, error)
	CloseLongPosition(ctx context.Context, symbol string, quantity float64) (*OrderResult, error)
	CloseShortPosition(ctx context.Context, symbol string, quantity float64) (*OrderResult, error)
	CancelOrder(ctx context.Context, symbol string, orderID int64) error
	GetOrderStatus(ctx context.Context, symbol string, orderID int64) (*OrderResult, error)

	// 止损止盈订单（限价单/止损单）
	CreateStopLossOrder(ctx context.Context, symbol string, side OrderSide, quantity float64, stopPrice float64) (*OrderResult, error)
	CreateTakeProfitOrder(ctx context.Context, symbol string, side OrderSide, quantity float64, takeProfitPrice float64) (*OrderResult, error)
	CancelAllOrders(ctx context.Context, symbol string) error

	// 交易历史
	GetTradeHistory(ctx context.Context, symbol string, orderId int64, limit int) ([]*TradeHistory, error)

	// 交易对信息
	GetSymbolInfo(ctx context.Context, symbol string) (*SymbolInfo, error)
	FormatQuantity(ctx context.Context, symbol string, quantity float64) (float64, error)
}
