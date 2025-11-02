package exchange

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// PaperWallet 纸钱包（模拟交易）
type PaperWallet struct {
	binanceClient *BinanceClient // 用于获取真实市场数据
	logger        *zap.Logger

	// 模拟账户数据
	balance          float64              // 账户余额
	initialBalance   float64              // 初始余额
	positions        map[string]*Position // symbol -> position
	orderID          int64                // 订单ID计数器
	symbolLeverages  map[string]int       // 每个交易对的杠杆设置
	symbolMarginType map[string]MarginType
	mu               sync.RWMutex
}

// NewPaperWallet 创建纸钱包
func NewPaperWallet(binanceClient *BinanceClient, initialBalance float64, logger *zap.Logger) *PaperWallet {
	return &PaperWallet{
		binanceClient:    binanceClient,
		logger:           logger,
		balance:          initialBalance,
		initialBalance:   initialBalance,
		positions:        make(map[string]*Position),
		orderID:          1000000, // 从1000000开始的模拟订单ID
		symbolLeverages:  make(map[string]int),
		symbolMarginType: make(map[string]MarginType),
	}
}

// GetKlines 获取K线数据（使用真实数据）
func (p *PaperWallet) GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]*Kline, error) {
	return p.binanceClient.GetKlines(ctx, symbol, interval, limit)
}

// GetCurrentPrice 获取当前价格（使用真实数据）
func (p *PaperWallet) GetCurrentPrice(ctx context.Context, symbol string) (float64, error) {
	return p.binanceClient.GetCurrentPrice(ctx, symbol)
}

// GetFundingRate 获取资金费率（使用真实数据）
func (p *PaperWallet) GetFundingRate(ctx context.Context, symbol string) (float64, error) {
	return p.binanceClient.GetFundingRate(ctx, symbol)
}

// GetAccountInfo 获取模拟账户信息
func (p *PaperWallet) GetAccountInfo(ctx context.Context) (*AccountInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 计算未实现盈亏
	unrealizedPnl := 0.0
	for _, pos := range p.positions {
		unrealizedPnl += pos.UnrealizedProfit
	}

	// 计算可用余额（余额 + 未实现盈亏）
	totalBalance := p.balance + unrealizedPnl

	// 计算已用保证金
	usedMargin := 0.0
	for _, pos := range p.positions {
		// 保证金 = 持仓价值 / 杠杆
		positionValue := pos.PositionAmount * pos.EntryPrice
		usedMargin += positionValue / float64(pos.Leverage)
	}

	availableBalance := totalBalance - usedMargin

	p.logger.Debug("paper wallet account info",
		zap.Float64("balance", p.balance),
		zap.Float64("unrealized_pnl", unrealizedPnl),
		zap.Float64("total_balance", totalBalance),
		zap.Float64("available_balance", availableBalance))

	return &AccountInfo{
		TotalBalance:     totalBalance,
		AvailableBalance: availableBalance,
		UnrealizedPnl:    unrealizedPnl,
	}, nil
}

// GetPositions 获取模拟持仓
func (p *PaperWallet) GetPositions(ctx context.Context) ([]*Position, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 更新所有持仓的未实现盈亏
	result := make([]*Position, 0, len(p.positions))
	for _, pos := range p.positions {
		// 获取当前价格
		currentPrice, err := p.GetCurrentPrice(ctx, pos.Symbol)
		if err != nil {
			p.logger.Warn("failed to get current price for position",
				zap.String("symbol", pos.Symbol),
				zap.Error(err))
			currentPrice = pos.MarkPrice // 使用上次的标记价格
		}

		// 更新标记价格
		updatedPos := *pos
		updatedPos.MarkPrice = currentPrice

		// 计算未实现盈亏
		pnl := 0.0
		if pos.Side == "long" {
			pnl = (currentPrice - pos.EntryPrice) * pos.PositionAmount
		} else {
			pnl = (pos.EntryPrice - currentPrice) * pos.PositionAmount
		}
		updatedPos.UnrealizedProfit = pnl

		result = append(result, &updatedPos)
	}

	return result, nil
}

// SetLeverage 设置杠杆（仅记录）
func (p *PaperWallet) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.symbolLeverages[symbol] = leverage
	p.logger.Info("paper wallet: set leverage",
		zap.String("symbol", symbol),
		zap.Int("leverage", leverage))
	return nil
}

// SetMarginType 设置保证金模式（仅记录）
func (p *PaperWallet) SetMarginType(ctx context.Context, symbol string, marginType MarginType) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.symbolMarginType[symbol] = marginType
	p.logger.Info("paper wallet: set margin type",
		zap.String("symbol", symbol),
		zap.String("margin_type", marginType.String()))
	return nil
}

// CreateMarketOrder 创建模拟市价单
func (p *PaperWallet) CreateMarketOrder(ctx context.Context, symbol string, side OrderSide,
	quantity float64, reduceOnly bool) (*OrderResult, error) {

	p.mu.Lock()
	defer p.mu.Unlock()

	// 获取当前价格（作为成交价）
	price, err := p.GetCurrentPrice(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get current price: %w", err)
	}

	// 生成订单ID
	p.orderID++
	orderID := p.orderID

	// 获取杠杆（默认为1）
	leverage := 1
	if lev, exists := p.symbolLeverages[symbol]; exists {
		leverage = lev
	}

	p.logger.Info("paper wallet: creating market order",
		zap.String("symbol", symbol),
		zap.String("side", side.String()),
		zap.Float64("quantity", quantity),
		zap.Float64("price", price),
		zap.Bool("reduce_only", reduceOnly),
		zap.Int64("order_id", orderID))

	// 处理开仓和平仓
	if reduceOnly {
		// 平仓操作
		pos, exists := p.positions[symbol]
		if !exists {
			return nil, fmt.Errorf("no position to close for %s", symbol)
		}

		// 计算盈亏
		pnl := 0.0
		if pos.Side == "long" {
			pnl = (price - pos.EntryPrice) * quantity
		} else {
			pnl = (pos.EntryPrice - price) * quantity
		}

		// 更新余额
		p.balance += pnl

		// 减少持仓
		if quantity >= pos.PositionAmount {
			// 完全平仓
			delete(p.positions, symbol)
			p.logger.Info("paper wallet: position fully closed",
				zap.String("symbol", symbol),
				zap.Float64("pnl", pnl))
		} else {
			// 部分平仓
			pos.PositionAmount -= quantity
			p.logger.Info("paper wallet: position partially closed",
				zap.String("symbol", symbol),
				zap.Float64("remaining", pos.PositionAmount),
				zap.Float64("pnl", pnl))
		}
	} else {
		// 开仓操作
		positionValue := price * quantity
		requiredMargin := positionValue / float64(leverage)

		// 检查余额是否足够
		if requiredMargin > p.balance {
			return nil, fmt.Errorf("insufficient balance: required %.2f, available %.2f", requiredMargin, p.balance)
		}

		// 扣除保证金（暂不扣除，因为保证金计算在GetAccountInfo中）
		// p.balance -= requiredMargin

		positionSide := "long"
		if side == OrderSideSell {
			positionSide = "short"
		}

		// 创建或更新持仓
		if existingPos, exists := p.positions[symbol]; exists {
			// 如果已有持仓，检查方向
			if existingPos.Side != positionSide {
				return nil, fmt.Errorf("cannot open %s position while holding %s position for %s",
					positionSide, existingPos.Side, symbol)
			}

			// 增加持仓（加权平均成本）
			totalCost := existingPos.EntryPrice*existingPos.PositionAmount + price*quantity
			totalAmount := existingPos.PositionAmount + quantity
			existingPos.EntryPrice = totalCost / totalAmount
			existingPos.PositionAmount = totalAmount
			existingPos.MarkPrice = price

			p.logger.Info("paper wallet: position increased",
				zap.String("symbol", symbol),
				zap.Float64("entry_price", existingPos.EntryPrice),
				zap.Float64("amount", existingPos.PositionAmount))
		} else {
			// 创建新持仓
			p.positions[symbol] = &Position{
				Symbol:           symbol,
				Side:             positionSide,
				PositionAmount:   quantity,
				EntryPrice:       price,
				MarkPrice:        price,
				UnrealizedProfit: 0,
				Leverage:         leverage,
				LiquidationPrice: 0, // 简化处理，不计算强平价
			}

			p.logger.Info("paper wallet: new position opened",
				zap.String("symbol", symbol),
				zap.String("side", positionSide),
				zap.Float64("entry_price", price),
				zap.Float64("amount", quantity),
				zap.Int("leverage", leverage))
		}
	}

	return &OrderResult{
		OrderID:     orderID,
		Symbol:      symbol,
		Side:        side.String(),
		Type:        OrderTypeMarket.String(),
		Quantity:    quantity,
		Price:       price,
		AvgPrice:    price,
		Status:      OrderStatusFilled.String(),
		ExecutedQty: quantity,
	}, nil
}

// OpenLongPosition 开多仓
func (p *PaperWallet) OpenLongPosition(ctx context.Context, symbol string, quantity float64) (*OrderResult, error) {
	return p.CreateMarketOrder(ctx, symbol, OrderSideBuy, quantity, false)
}

// OpenShortPosition 开空仓
func (p *PaperWallet) OpenShortPosition(ctx context.Context, symbol string, quantity float64) (*OrderResult, error) {
	return p.CreateMarketOrder(ctx, symbol, OrderSideSell, quantity, false)
}

// CloseLongPosition 平多仓
func (p *PaperWallet) CloseLongPosition(ctx context.Context, symbol string, quantity float64) (*OrderResult, error) {
	return p.CreateMarketOrder(ctx, symbol, OrderSideSell, quantity, true)
}

// CloseShortPosition 平空仓
func (p *PaperWallet) CloseShortPosition(ctx context.Context, symbol string, quantity float64) (*OrderResult, error) {
	return p.CreateMarketOrder(ctx, symbol, OrderSideBuy, quantity, true)
}

// CancelOrder 取消订单（纸钱包不支持待成交订单）
func (p *PaperWallet) CancelOrder(ctx context.Context, symbol string, orderID int64) error {
	p.logger.Warn("paper wallet: cancel order not supported",
		zap.String("symbol", symbol),
		zap.Int64("order_id", orderID))
	return fmt.Errorf("paper wallet does not support pending orders")
}

// GetOrderStatus 获取订单状态（纸钱包所有订单立即成交）
func (p *PaperWallet) GetOrderStatus(ctx context.Context, symbol string, orderID int64) (*OrderResult, error) {
	p.logger.Warn("paper wallet: get order status not fully supported",
		zap.String("symbol", symbol),
		zap.Int64("order_id", orderID))
	return &OrderResult{
		OrderID: orderID,
		Symbol:  symbol,
		Status:  "FILLED",
	}, nil
}

// GetSymbolInfo 获取交易对信息（使用真实数据）
func (p *PaperWallet) GetSymbolInfo(ctx context.Context, symbol string) (*SymbolInfo, error) {
	return p.binanceClient.GetSymbolInfo(ctx, symbol)
}

// FormatQuantity 格式化数量（使用真实规则）
func (p *PaperWallet) FormatQuantity(ctx context.Context, symbol string, quantity float64) (float64, error) {
	return p.binanceClient.FormatQuantity(ctx, symbol, quantity)
}

// GetBalance 获取当前余额（用于测试和调试）
func (p *PaperWallet) GetBalance() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.balance
}

// GetInitialBalance 获取初始余额
func (p *PaperWallet) GetInitialBalance() float64 {
	return p.initialBalance
}

// Reset 重置纸钱包到初始状态
func (p *PaperWallet) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.balance = p.initialBalance
	p.positions = make(map[string]*Position)
	p.symbolLeverages = make(map[string]int)
	p.symbolMarginType = make(map[string]MarginType)

	p.logger.Info("paper wallet reset to initial state",
		zap.Float64("initial_balance", p.initialBalance))
}

// CreateStopLossOrder 创建止损单（模拟）
func (p *PaperWallet) CreateStopLossOrder(ctx context.Context, symbol string, side OrderSide, quantity float64, stopPrice float64) (*OrderResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.orderID++
	orderID := p.orderID

	p.logger.Info("paper wallet: stop loss order created (simulated)",
		zap.String("symbol", symbol),
		zap.String("side", string(side)),
		zap.Float64("quantity", quantity),
		zap.Float64("stop_price", stopPrice),
		zap.Int64("order_id", orderID))

	// 纸钱包模式：止损单不会真实执行
	// 实际的止损逻辑由AI在每15分钟的循环中检查并手动平仓
	// 这里只是记录止损单已创建

	return &OrderResult{
		OrderID:     orderID,
		Symbol:      symbol,
		Side:        string(side),
		Type:        "STOP_MARKET",
		Quantity:    quantity,
		Price:       stopPrice,
		AvgPrice:    0,
		Status:      "NEW",
		ExecutedQty: 0,
	}, nil
}

// CreateTakeProfitOrder 创建止盈单（模拟）
func (p *PaperWallet) CreateTakeProfitOrder(ctx context.Context, symbol string, side OrderSide, quantity float64, takeProfitPrice float64) (*OrderResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.orderID++
	orderID := p.orderID

	p.logger.Info("paper wallet: take profit order created (simulated)",
		zap.String("symbol", symbol),
		zap.String("side", string(side)),
		zap.Float64("quantity", quantity),
		zap.Float64("take_profit_price", takeProfitPrice),
		zap.Int64("order_id", orderID))

	// 纸钱包模式：止盈单不会真实执行
	// 实际的止盈逻辑由AI在每15分钟的循环中检查并手动平仓

	return &OrderResult{
		OrderID:     orderID,
		Symbol:      symbol,
		Side:        string(side),
		Type:        "TAKE_PROFIT_MARKET",
		Quantity:    quantity,
		Price:       takeProfitPrice,
		AvgPrice:    0,
		Status:      "NEW",
		ExecutedQty: 0,
	}, nil
}

// CancelAllOrders 取消所有挂单（模拟）
func (p *PaperWallet) CancelAllOrders(ctx context.Context, symbol string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.logger.Info("paper wallet: all orders cancelled (simulated)",
		zap.String("symbol", symbol))

	// 纸钱包模式：没有实际的挂单需要取消
	return nil
}

// GetTradeHistory 获取交易历史（纸钱包模式返回空）
func (p *PaperWallet) GetTradeHistory(ctx context.Context, symbol string, orderId int64, limit int) ([]*TradeHistory, error) {
	// 纸钱包模式不记录详细的交易历史，返回空数组
	// 交易记录在应用层通过 Trade 表管理
	return []*TradeHistory{}, nil
}
