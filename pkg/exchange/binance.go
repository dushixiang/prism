package exchange

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

// BinanceClient Binance期货API客户端
type BinanceClient struct {
	client         *futures.Client
	symbolInfoMap  map[string]*SymbolInfo
	symbolInfoLock sync.RWMutex
}

// SymbolInfo 交易对信息
type SymbolInfo struct {
	Symbol            string
	QuantityPrecision int
	PricePrecision    int
	MinQuantity       float64
	MaxQuantity       float64
	StepSize          float64
	MinNotional       float64
	lastUpdated       time.Time
}

// NewBinanceClient 创建Binance客户端
func NewBinanceClient(apiKey, secretKey, proxyURL string, testnet bool) *BinanceClient {
	var client *futures.Client
	if proxyURL != "" {
		client = futures.NewProxiedClient(apiKey, secretKey, proxyURL)
	} else {
		client = futures.NewClient(apiKey, secretKey)
	}

	if testnet {
		// 测试网URL
		futures.UseTestnet = true
	}

	return &BinanceClient{
		client:        client,
		symbolInfoMap: make(map[string]*SymbolInfo),
	}
}

// Kline K线数据
type Kline struct {
	OpenTime  time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	CloseTime time.Time
}

// GetKlines 获取K线数据
func (b *BinanceClient) GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]*Kline, error) {
	klines, err := b.client.NewKlinesService().
		Symbol(symbol).
		Interval(interval).
		Limit(limit).
		Do(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get klines: %w", err)
	}

	result := make([]*Kline, 0, len(klines))
	for _, k := range klines {
		open, _ := strconv.ParseFloat(k.Open, 64)
		high, _ := strconv.ParseFloat(k.High, 64)
		low, _ := strconv.ParseFloat(k.Low, 64)
		close, _ := strconv.ParseFloat(k.Close, 64)
		volume, _ := strconv.ParseFloat(k.Volume, 64)

		result = append(result, &Kline{
			OpenTime:  time.Unix(k.OpenTime/1000, 0),
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: time.Unix(k.CloseTime/1000, 0),
		})
	}

	return result, nil
}

// AccountInfo 账户信息
type AccountInfo struct {
	TotalBalance     float64 // 总余额
	AvailableBalance float64 // 可用余额
	UnrealizedPnl    float64 // 未实现盈亏
}

// GetAccountInfo 获取账户信息
func (b *BinanceClient) GetAccountInfo(ctx context.Context) (*AccountInfo, error) {
	account, err := b.client.NewGetAccountService().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}

	totalBalance, _ := strconv.ParseFloat(account.TotalWalletBalance, 64)
	availableBalance, _ := strconv.ParseFloat(account.AvailableBalance, 64)
	unrealizedPnl, _ := strconv.ParseFloat(account.TotalUnrealizedProfit, 64)

	return &AccountInfo{
		TotalBalance:     totalBalance,
		AvailableBalance: availableBalance,
		UnrealizedPnl:    unrealizedPnl,
	}, nil
}

// Position 持仓信息
type Position struct {
	Symbol           string
	Side             string  // LONG/SHORT
	PositionAmount   float64 // 持仓数量
	EntryPrice       float64 // 开仓均价
	MarkPrice        float64 // 标记价格
	UnrealizedProfit float64 // 未实现盈亏
	Leverage         int     // 杠杆倍数
	LiquidationPrice float64 // 强平价格
}

// GetPositions 获取当前持仓
func (b *BinanceClient) GetPositions(ctx context.Context) ([]*Position, error) {
	positions, err := b.client.NewGetPositionRiskService().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	result := make([]*Position, 0)
	for _, p := range positions {
		positionAmt, _ := strconv.ParseFloat(p.PositionAmt, 64)

		// 过滤掉空仓位
		if positionAmt == 0 {
			continue
		}

		entryPrice, _ := strconv.ParseFloat(p.EntryPrice, 64)
		markPrice, _ := strconv.ParseFloat(p.MarkPrice, 64)
		unrealizedProfit, _ := strconv.ParseFloat(p.UnRealizedProfit, 64)
		leverage, _ := strconv.Atoi(p.Leverage)
		liquidationPrice, _ := strconv.ParseFloat(p.LiquidationPrice, 64)

		side := "long"
		if positionAmt < 0 {
			side = "short"
			positionAmt = -positionAmt
		}

		result = append(result, &Position{
			Symbol:           p.Symbol,
			Side:             side,
			PositionAmount:   positionAmt,
			EntryPrice:       entryPrice,
			MarkPrice:        markPrice,
			UnrealizedProfit: unrealizedProfit,
			Leverage:         leverage,
			LiquidationPrice: liquidationPrice,
		})
	}

	return result, nil
}

// SetLeverage 设置杠杆倍数
func (b *BinanceClient) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	_, err := b.client.NewChangeLeverageService().
		Symbol(symbol).
		Leverage(leverage).
		Do(ctx)

	if err != nil {
		return fmt.Errorf("failed to set leverage: %w", err)
	}

	return nil
}

// SetMarginType 设置保证金模式
func (b *BinanceClient) SetMarginType(ctx context.Context, symbol string, marginType futures.MarginType) error {
	err := b.client.NewChangeMarginTypeService().
		Symbol(symbol).
		MarginType(marginType).
		Do(ctx)

	if err != nil {
		return fmt.Errorf("failed to set margin type: %w", err)
	}

	return nil
}

// OrderResult 订单结果
type OrderResult struct {
	OrderID     int64
	Symbol      string
	Side        string
	Type        string
	Quantity    float64
	Price       float64
	AvgPrice    float64
	Status      string
	ExecutedQty float64
}

// CreateMarketOrder 创建市价单
func (b *BinanceClient) CreateMarketOrder(ctx context.Context, symbol string, side futures.SideType,
	quantity float64, reduceOnly bool) (*OrderResult, error) {

	// 格式化数量以符合交易对精度要求
	formattedQty, err := b.FormatQuantity(ctx, symbol, quantity)
	if err != nil {
		return nil, fmt.Errorf("failed to format quantity: %w", err)
	}

	// 获取精度信息用于格式化字符串
	info, err := b.GetSymbolInfo(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get symbol info: %w", err)
	}

	// 使用正确的精度格式化数量字符串
	quantityStr := strconv.FormatFloat(formattedQty, 'f', info.QuantityPrecision, 64)

	service := b.client.NewCreateOrderService().
		Symbol(symbol).
		Side(side).
		Type(futures.OrderTypeMarket).
		Quantity(quantityStr)

	if reduceOnly {
		service.ReduceOnly(true)
	}

	order, err := service.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create market order: %w", err)
	}

	avgPrice, _ := strconv.ParseFloat(order.AvgPrice, 64)
	executedQty, _ := strconv.ParseFloat(order.ExecutedQuantity, 64)
	origQty, _ := strconv.ParseFloat(order.OrigQuantity, 64)

	return &OrderResult{
		OrderID:     order.OrderID,
		Symbol:      order.Symbol,
		Side:        string(order.Side),
		Type:        string(order.Type),
		Quantity:    origQty,
		AvgPrice:    avgPrice,
		Status:      string(order.Status),
		ExecutedQty: executedQty,
	}, nil
}

// OpenLongPosition 开多仓
func (b *BinanceClient) OpenLongPosition(ctx context.Context, symbol string, quantity float64) (*OrderResult, error) {
	return b.CreateMarketOrder(ctx, symbol, futures.SideTypeBuy, quantity, false)
}

// OpenShortPosition 开空仓
func (b *BinanceClient) OpenShortPosition(ctx context.Context, symbol string, quantity float64) (*OrderResult, error) {
	return b.CreateMarketOrder(ctx, symbol, futures.SideTypeSell, quantity, false)
}

// CloseLongPosition 平多仓
func (b *BinanceClient) CloseLongPosition(ctx context.Context, symbol string, quantity float64) (*OrderResult, error) {
	return b.CreateMarketOrder(ctx, symbol, futures.SideTypeSell, quantity, true)
}

// CloseShortPosition 平空仓
func (b *BinanceClient) CloseShortPosition(ctx context.Context, symbol string, quantity float64) (*OrderResult, error) {
	return b.CreateMarketOrder(ctx, symbol, futures.SideTypeBuy, quantity, true)
}

// GetCurrentPrice 获取当前价格
func (b *BinanceClient) GetCurrentPrice(ctx context.Context, symbol string) (float64, error) {
	prices, err := b.client.NewListPricesService().Symbol(symbol).Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get current price: %w", err)
	}

	if len(prices) == 0 {
		return 0, fmt.Errorf("no price data for symbol %s", symbol)
	}

	price, _ := strconv.ParseFloat(prices[0].Price, 64)
	return price, nil
}

// GetFundingRate 获取资金费率
func (b *BinanceClient) GetFundingRate(ctx context.Context, symbol string) (float64, error) {
	rates, err := b.client.NewFundingRateService().
		Symbol(symbol).
		Limit(1).
		Do(ctx)

	if err != nil {
		return 0, fmt.Errorf("failed to get funding rate: %w", err)
	}

	if len(rates) == 0 {
		return 0, fmt.Errorf("no funding rate data for symbol %s", symbol)
	}

	rate, _ := strconv.ParseFloat(rates[0].FundingRate, 64)
	return rate, nil
}

// CancelOrder 取消订单
func (b *BinanceClient) CancelOrder(ctx context.Context, symbol string, orderID int64) error {
	_, err := b.client.NewCancelOrderService().
		Symbol(symbol).
		OrderID(orderID).
		Do(ctx)

	if err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	return nil
}

// GetOrderStatus 获取订单状态
func (b *BinanceClient) GetOrderStatus(ctx context.Context, symbol string, orderID int64) (*OrderResult, error) {
	order, err := b.client.NewGetOrderService().
		Symbol(symbol).
		OrderID(orderID).
		Do(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get order status: %w", err)
	}

	avgPrice, _ := strconv.ParseFloat(order.AvgPrice, 64)
	executedQty, _ := strconv.ParseFloat(order.ExecutedQuantity, 64)
	origQty, _ := strconv.ParseFloat(order.OrigQuantity, 64)
	price, _ := strconv.ParseFloat(order.Price, 64)

	return &OrderResult{
		OrderID:     order.OrderID,
		Symbol:      order.Symbol,
		Side:        string(order.Side),
		Type:        string(order.Type),
		Quantity:    origQty,
		Price:       price,
		AvgPrice:    avgPrice,
		Status:      string(order.Status),
		ExecutedQty: executedQty,
	}, nil
}

// GetSymbolInfo 获取交易对信息
func (b *BinanceClient) GetSymbolInfo(ctx context.Context, symbol string) (*SymbolInfo, error) {
	// 检查缓存（5分钟有效期）
	b.symbolInfoLock.RLock()
	if info, exists := b.symbolInfoMap[symbol]; exists {
		if time.Since(info.lastUpdated) < 5*time.Minute {
			b.symbolInfoLock.RUnlock()
			return info, nil
		}
	}
	b.symbolInfoLock.RUnlock()

	// 获取交易对信息
	exchangeInfo, err := b.client.NewExchangeInfoService().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange info: %w", err)
	}

	// 查找指定交易对
	for _, s := range exchangeInfo.Symbols {
		if s.Symbol == symbol {
			info := &SymbolInfo{
				Symbol:            s.Symbol,
				QuantityPrecision: s.QuantityPrecision,
				PricePrecision:    s.PricePrecision,
				lastUpdated:       time.Now(),
			}

			// 解析过滤器
			for _, filter := range s.Filters {
				switch filter["filterType"] {
				case "LOT_SIZE":
					if minQty, ok := filter["minQty"].(string); ok {
						info.MinQuantity, _ = strconv.ParseFloat(minQty, 64)
					}
					if maxQty, ok := filter["maxQty"].(string); ok {
						info.MaxQuantity, _ = strconv.ParseFloat(maxQty, 64)
					}
					if stepSize, ok := filter["stepSize"].(string); ok {
						info.StepSize, _ = strconv.ParseFloat(stepSize, 64)
					}
				case "MIN_NOTIONAL":
					if notional, ok := filter["notional"].(string); ok {
						info.MinNotional, _ = strconv.ParseFloat(notional, 64)
					}
				}
			}

			// 缓存信息
			b.symbolInfoLock.Lock()
			b.symbolInfoMap[symbol] = info
			b.symbolInfoLock.Unlock()

			return info, nil
		}
	}

	return nil, fmt.Errorf("symbol %s not found", symbol)
}

// FormatQuantity 根据交易对精度格式化数量
func (b *BinanceClient) FormatQuantity(ctx context.Context, symbol string, quantity float64) (float64, error) {
	info, err := b.GetSymbolInfo(ctx, symbol)
	if err != nil {
		return 0, err
	}

	// 根据 stepSize 调整数量
	if info.StepSize > 0 {
		quantity = math.Floor(quantity/info.StepSize) * info.StepSize
	}

	// 根据精度截断
	precision := math.Pow10(info.QuantityPrecision)
	quantity = math.Floor(quantity*precision) / precision

	// 验证范围
	if quantity < info.MinQuantity {
		return 0, fmt.Errorf("quantity %.8f is below minimum %.8f for %s", quantity, info.MinQuantity, symbol)
	}
	if info.MaxQuantity > 0 && quantity > info.MaxQuantity {
		return 0, fmt.Errorf("quantity %.8f exceeds maximum %.8f for %s", quantity, info.MaxQuantity, symbol)
	}

	return quantity, nil
}
