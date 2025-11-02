package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dushixiang/prism/internal/models"
	"github.com/dushixiang/prism/internal/repo"
	"github.com/dushixiang/prism/pkg/exchange"
	"github.com/go-orz/orz"
	"github.com/oklog/ulid/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// PositionService 持仓管理服务
type PositionService struct {
	logger *zap.Logger

	*orz.Service
	*repo.PositionRepo

	exchange  exchange.Exchange
	orderRepo *repo.OrderRepo
	tradeRepo *repo.TradeRepo

	// 后台同步相关
	syncMutex sync.Mutex // 防止并发同步
	stopChan  chan struct{}
	stopped   bool
}

// NewPositionService 创建持仓服务
func NewPositionService(db *gorm.DB, exchange exchange.Exchange, orderRepo *repo.OrderRepo, tradeRepo *repo.TradeRepo, logger *zap.Logger) *PositionService {
	return &PositionService{
		logger:       logger,
		Service:      orz.NewService(db),
		PositionRepo: repo.NewPositionRepo(db),
		exchange:     exchange,
		orderRepo:    orderRepo,
		tradeRepo:    tradeRepo,
	}
}

// SyncPositions 同步持仓数据
func (s *PositionService) SyncPositions(ctx context.Context) error {
	// 使用互斥锁避免并发同步
	s.syncMutex.Lock()
	defer s.syncMutex.Unlock()

	// 从交易所获取实时持仓
	positions, err := s.exchange.GetPositions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get positions from binance: %w", err)
	}

	// 提前加载本地持仓，便于保留开仓时间、止盈止损等信息
	existingPositions, err := s.PositionRepo.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to load existing positions: %w", err)
	}

	existingMap := make(map[string]*models.Position, len(existingPositions))
	for i := range existingPositions {
		pos := &existingPositions[i]
		key := fmt.Sprintf("%s|%s", pos.Symbol, pos.Side)
		existingMap[key] = pos
	}

	err = s.Transaction(ctx, func(ctx context.Context) error {
		seen := make(map[string]struct{}, len(positions))

		// 更新或新增持仓
		for _, p := range positions {
			// 计算保证金
			margin := 0.0
			if p.Leverage != 0 {
				margin = p.EntryPrice * p.PositionAmount / float64(p.Leverage)
			}
			if margin < 0 {
				margin = -margin
			}

			key := fmt.Sprintf("%s|%s", p.Symbol, p.Side)
			if existingPos, ok := existingMap[key]; ok {
				existingPos.Quantity = p.PositionAmount
				existingPos.EntryPrice = p.EntryPrice
				existingPos.CurrentPrice = p.MarkPrice
				existingPos.LiquidationPrice = p.LiquidationPrice
				existingPos.UnrealizedPnl = p.UnrealizedProfit
				existingPos.Leverage = p.Leverage
				existingPos.Margin = margin

				// 更新峰值收益
				if pnlPercent := existingPos.CalculatePnlPercent(); pnlPercent > existingPos.PeakPnlPercent {
					existingPos.PeakPnlPercent = pnlPercent
				}

				if err := s.PositionRepo.Save(ctx, existingPos); err != nil {
					return fmt.Errorf("failed to update position %s %s: %w", p.Symbol, p.Side, err)
				}
			} else {
				position := &models.Position{
					ID:               ulid.Make().String(),
					Symbol:           p.Symbol,
					Side:             p.Side,
					Quantity:         p.PositionAmount,
					EntryPrice:       p.EntryPrice,
					CurrentPrice:     p.MarkPrice,
					LiquidationPrice: p.LiquidationPrice,
					UnrealizedPnl:    p.UnrealizedProfit,
					Leverage:         p.Leverage,
					Margin:           margin,
					OpenedAt:         time.Now(),
				}

				// 新建持仓的初始峰值以当前值为基准
				if pnlPercent := position.CalculatePnlPercent(); pnlPercent > 0 {
					position.PeakPnlPercent = pnlPercent
				}

				if err := s.PositionRepo.Create(ctx, position); err != nil {
					return fmt.Errorf("failed to create position: %w", err)
				}

				existingMap[key] = position
			}

			seen[key] = struct{}{}
		}

		// 删除已经不存在的持仓
		for key, pos := range existingMap {
			if _, ok := seen[key]; !ok {
				if err := s.PositionRepo.DeleteById(ctx, pos.ID); err != nil {
					return fmt.Errorf("failed to delete stale position %s %s: %w", pos.Symbol, pos.Side, err)
				}
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	// 同步订单状态（检测止损止盈单是否被触发）
	if err := s.syncOrderStatus(ctx); err != nil {
		s.logger.Warn("failed to sync order status", zap.Error(err))
		// 不返回错误，继续执行
	}

	return nil
}

// parseExchangeOrderID 解析交易所订单ID字符串为int64
func (s *PositionService) parseExchangeOrderID(exchangeID string) (int64, error) {
	if exchangeID == "" {
		return 0, fmt.Errorf("empty exchange order id")
	}

	orderID, err := strconv.ParseInt(exchangeID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid exchange order id: %w", err)
	}

	return orderID, nil
}

// cancelOrderOnExchange 在交易所取消单个订单
func (s *PositionService) cancelOrderOnExchange(ctx context.Context, order *models.Order, reason string) error {
	if order.ExchangeID == "" {
		return nil
	}

	exchangeOrderID, err := s.parseExchangeOrderID(order.ExchangeID)
	if err != nil {
		s.logger.Warn("skip cancelling order with invalid exchange id",
			zap.String("order_id", order.ID),
			zap.String("exchange_id", order.ExchangeID),
			zap.Error(err))
		return nil
	}

	if err := s.exchange.CancelOrder(ctx, order.Symbol, exchangeOrderID); err != nil {
		s.logger.Warn("failed to cancel order on exchange",
			zap.String("symbol", order.Symbol),
			zap.String("order_id", order.ExchangeID),
			zap.String("order_type", string(order.OrderType)),
			zap.String("reason", reason),
			zap.Error(err))
		return err
	}

	s.logger.Info("cancelled order on exchange",
		zap.String("symbol", order.Symbol),
		zap.String("order_type", string(order.OrderType)),
		zap.String("order_id", order.ExchangeID),
		zap.String("reason", reason))

	return nil
}

// updateOrderStatusToCanceled 更新订单状态为已取消
func (s *PositionService) updateOrderStatusToCanceled(ctx context.Context, orderID string) {
	if err := s.orderRepo.UpdateStatus(ctx, orderID, models.OrderStatusCanceled); err != nil {
		s.logger.Error("failed to update order status to canceled",
			zap.String("order_id", orderID),
			zap.Error(err))
	}
}

// cancelPositionOrders 取消指定持仓的所有活跃订单
func (s *PositionService) cancelPositionOrders(ctx context.Context, positionID string) error {
	// 获取该持仓的所有活跃订单
	activeOrders, err := s.orderRepo.FindByPositionID(ctx, positionID)
	if err != nil {
		return fmt.Errorf("failed to get orders for position: %w", err)
	}

	if len(activeOrders) == 0 {
		return nil
	}

	symbol := activeOrders[0].Symbol
	s.logger.Info("cancelling orders for deleted position",
		zap.String("position_id", positionID),
		zap.String("symbol", symbol),
		zap.Int("order_count", len(activeOrders)))

	// 逐个取消订单
	for i := range activeOrders {
		order := &activeOrders[i]

		// 只处理活跃订单
		if !order.IsActive() {
			continue
		}

		// 从交易所取消订单
		_ = s.cancelOrderOnExchange(ctx, order, "position deleted")

		// 更新数据库订单状态
		s.updateOrderStatusToCanceled(ctx, order.ID)
	}

	return nil
}

// syncOrderStatus 同步订单状态（通过交易所 API 查询订单真实状态）
func (s *PositionService) syncOrderStatus(ctx context.Context) error {
	if s.orderRepo == nil {
		return nil // orderRepo 未注入，跳过
	}

	// 获取所有活跃订单
	activeOrders, err := s.orderRepo.FindAllActive(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active orders: %w", err)
	}

	if len(activeOrders) == 0 {
		return nil
	}

	// 按持仓ID分组订单（一个持仓可能有多个订单）
	ordersByPosition := make(map[string][]models.Order)
	for i := range activeOrders {
		posID := activeOrders[i].PositionID
		ordersByPosition[posID] = append(ordersByPosition[posID], activeOrders[i])
	}

	// 检查每个持仓的订单状态
	for posID, orders := range ordersByPosition {
		s.checkPositionOrders(ctx, posID, orders)
	}

	return nil
}

// queryExchangeOrderStatus 查询单个订单在交易所的状态
func (s *PositionService) queryExchangeOrderStatus(ctx context.Context, order *models.Order) (string, error) {
	if order.ExchangeID == "" {
		return "", fmt.Errorf("empty exchange order id")
	}

	exchangeOrderID, err := s.parseExchangeOrderID(order.ExchangeID)
	if err != nil {
		return "", err
	}

	orderResult, err := s.exchange.GetOrderStatus(ctx, order.Symbol, exchangeOrderID)
	if err != nil {
		return "", fmt.Errorf("failed to get order status: %w", err)
	}

	if orderResult == nil {
		return "", fmt.Errorf("empty order result")
	}

	return orderResult.Status, nil
}

// handleTriggeredOrder 处理被触发的订单（记录交易并取消其他订单）
func (s *PositionService) handleTriggeredOrder(ctx context.Context, triggeredOrder *models.Order, allOrders []models.Order) {
	s.logger.Info("detected triggered order via exchange API",
		zap.String("order_id", triggeredOrder.ID),
		zap.String("symbol", triggeredOrder.Symbol),
		zap.String("order_type", string(triggeredOrder.OrderType)),
		zap.Float64("trigger_price", triggeredOrder.TriggerPrice))

	// 记录平仓交易
	if s.tradeRepo != nil {
		s.recordTriggeredOrderTrade(ctx, triggeredOrder)
	}

	// 取消该持仓的其他活跃订单（例如：止损触发了，需要取消止盈单）
	s.cancelOtherOrders(ctx, triggeredOrder.ID, allOrders)
}

// cancelOtherOrders 取消除指定订单外的其他订单
func (s *PositionService) cancelOtherOrders(ctx context.Context, excludeOrderID string, orders []models.Order) {
	for i := range orders {
		order := &orders[i]
		if order.ID == excludeOrderID {
			continue // 跳过被排除的订单
		}

		// 从交易所取消订单
		s.cancelOrderOnExchange(ctx, order, "other order triggered")

		// 更新数据库订单状态为已取消
		s.updateOrderStatusToCanceled(ctx, order.ID)
	}
}

// checkPositionOrders 检查一个持仓的所有订单状态
func (s *PositionService) checkPositionOrders(ctx context.Context, positionID string, orders []models.Order) {
	if len(orders) == 0 {
		return
	}

	type orderStatusUpdate struct {
		order          *models.Order
		exchangeStatus string
	}

	var triggeredOrder *models.Order
	var updates []orderStatusUpdate

	// 逐个查询交易所订单状态
	for i := range orders {
		order := &orders[i]
		exchangeStatus, err := s.queryExchangeOrderStatus(ctx, order)
		if err != nil {
			s.logger.Warn("failed to query order status from exchange",
				zap.String("symbol", order.Symbol),
				zap.String("order_id", order.ExchangeID),
				zap.Error(err))
			continue
		}

		// 记录需要更新的订单
		updates = append(updates, orderStatusUpdate{
			order:          order,
			exchangeStatus: exchangeStatus,
		})

		// 检查是否有订单被触发
		if exchangeStatus == "FILLED" {
			triggeredOrder = order
		}
	}

	// 同步所有订单状态
	for _, update := range updates {
		s.syncSingleOrderStatus(ctx, update.order, update.exchangeStatus)
	}

	// 如果有订单被触发，需要额外处理
	if triggeredOrder != nil {
		s.handleTriggeredOrder(ctx, triggeredOrder, orders)
	}
}

// mapExchangeStatusToLocal 映射交易所订单状态到本地状态
func (s *PositionService) mapExchangeStatusToLocal(exchangeStatus string) (models.OrderStatus, bool) {
	switch exchangeStatus {
	case "NEW", "PARTIALLY_FILLED":
		return models.OrderStatusActive, false // 仍然活跃，无需更新

	case "FILLED":
		return models.OrderStatusTriggered, true

	case "CANCELED":
		return models.OrderStatusCanceled, true

	case "REJECTED", "EXPIRED":
		return models.OrderStatusFailed, true

	default:
		return "", false // 未知状态
	}
}

// syncSingleOrderStatus 同步单个订单的状态
func (s *PositionService) syncSingleOrderStatus(ctx context.Context, order *models.Order, exchangeStatus string) {
	// 映射交易所状态到本地状态
	localStatus, shouldUpdate := s.mapExchangeStatusToLocal(exchangeStatus)

	if !shouldUpdate {
		if localStatus == "" {
			s.logger.Warn("unknown exchange order status",
				zap.String("order_id", order.ID),
				zap.String("exchange_status", exchangeStatus))
		}
		return // 状态未变化或未知状态，无需更新
	}

	// 检查状态是否变化
	if order.Status == localStatus {
		return // 状态未变化，无需更新
	}

	// 更新数据库订单状态
	if err := s.orderRepo.UpdateStatus(ctx, order.ID, localStatus); err != nil {
		s.logger.Error("failed to sync order status",
			zap.String("order_id", order.ID),
			zap.String("old_status", string(order.Status)),
			zap.String("new_status", string(localStatus)),
			zap.Error(err))
	} else {
		s.logger.Info("synced order status from exchange",
			zap.String("order_id", order.ID),
			zap.String("symbol", order.Symbol),
			zap.String("order_type", string(order.OrderType)),
			zap.String("old_status", string(order.Status)),
			zap.String("new_status", string(localStatus)),
			zap.String("exchange_status", exchangeStatus))
	}
}

// recordTriggeredOrderTrade 记录由订单触发的平仓交易
// 使用交易所的交易历史获取准确的成交价格、数量和手续费
// 将多笔成交合并为一条记录,使用最后一笔成交的时间
func (s *PositionService) recordTriggeredOrderTrade(ctx context.Context, order *models.Order) {
	// 解析交易所订单ID
	exchangeOrderID, err := s.parseExchangeOrderID(order.ExchangeID)
	if err != nil {
		s.logger.Error("failed to parse exchange order id for trade recording",
			zap.String("order_id", order.ID),
			zap.String("exchange_id", order.ExchangeID),
			zap.Error(err))
		return
	}

	// 从交易所获取该订单的真实成交记录
	tradeHistory, err := s.exchange.GetTradeHistory(ctx, order.Symbol, exchangeOrderID, 10)
	if err != nil {
		s.logger.Error("failed to get trade history from exchange",
			zap.String("symbol", order.Symbol),
			zap.Int64("order_id", exchangeOrderID),
			zap.Error(err))
		return
	}

	if len(tradeHistory) == 0 {
		s.logger.Error("no trade history found for triggered order",
			zap.String("symbol", order.Symbol),
			zap.Int64("order_id", exchangeOrderID),
			zap.String("order_type", string(order.OrderType)))
		return
	}

	// 汇总所有成交记录（订单可能分多笔成交）
	var totalQuantity, totalCommission, totalRealizedPnl float64
	var weightedPriceSum float64
	var lastTradeTime int64

	for _, t := range tradeHistory {
		totalQuantity += t.Quantity
		totalCommission += t.Commission
		totalRealizedPnl += t.RealizedPnl
		weightedPriceSum += t.Price * t.Quantity
		// 记录最后一笔成交时间
		if t.Time > lastTradeTime {
			lastTradeTime = t.Time
		}
	}

	// 计算加权平均成交价
	avgPrice := order.TriggerPrice
	if totalQuantity > 0 {
		avgPrice = weightedPriceSum / totalQuantity
	}

	trade := &models.Trade{
		ID:         ulid.Make().String(),
		Symbol:     order.Symbol,
		Type:       "close",
		Side:       order.PositionSide,
		Price:      avgPrice,
		Quantity:   totalQuantity,
		Fee:        totalCommission,
		Pnl:        totalRealizedPnl,
		Reason:     fmt.Sprintf("订单触发: %s @ $%.2f", order.OrderType, avgPrice),
		OrderID:    order.ExchangeID,
		PositionID: order.PositionID,
		ExecutedAt: time.UnixMilli(lastTradeTime),
	}

	if err := s.tradeRepo.Create(ctx, trade); err != nil {
		s.logger.Error("failed to record triggered order trade",
			zap.String("order_id", order.ID),
			zap.Error(err))
	} else {
		s.logger.Info("recorded triggered order trade from exchange history",
			zap.String("trade_id", trade.ID),
			zap.String("symbol", trade.Symbol),
			zap.String("order_type", string(order.OrderType)),
			zap.Float64("avg_price", avgPrice),
			zap.Float64("quantity", totalQuantity),
			zap.Float64("fee", totalCommission),
			zap.Float64("pnl", totalRealizedPnl),
			zap.Int("exchange_trades", len(tradeHistory)))
	}
}

// GetAllPositions 获取所有持仓
func (s *PositionService) GetAllPositions(ctx context.Context) ([]models.Position, error) {
	positions, err := s.PositionRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	return positions, nil
}

// GetPosition 获取单个持仓
func (s *PositionService) GetPosition(ctx context.Context, id string) (*models.Position, error) {
	position, err := s.PositionRepo.FindById(ctx, id)
	if err != nil {
		return nil, err
	}

	return &position, nil
}

// UpdatePeakPnl 更新峰值盈亏
func (s *PositionService) UpdatePeakPnl(ctx context.Context, positionID string, pnlPercent float64) error {
	position, err := s.PositionRepo.FindById(ctx, positionID)
	if err != nil {
		return err
	}

	if pnlPercent > position.PeakPnlPercent {
		return s.PositionRepo.UpdatePeakPnlPercent(ctx, positionID, pnlPercent)
	}

	return nil
}

// DeletePosition 删除持仓记录
func (s *PositionService) DeletePosition(ctx context.Context, id string) error {
	return s.PositionRepo.DeleteById(ctx, id)
}

// UpdatePositionPlan 更新持仓的开仓理由与退出计划
func (s *PositionService) UpdatePositionPlan(ctx context.Context, symbol, side, entryReason, exitPlan string) error {
	entryReason = strings.TrimSpace(entryReason)
	exitPlan = strings.TrimSpace(exitPlan)

	if entryReason == "" && exitPlan == "" {
		return nil
	}

	position, err := s.PositionRepo.FindActiveBySymbolAndSide(ctx, symbol, side)
	if err != nil {
		return err
	}

	updated := false
	if entryReason != "" && position.EntryReason != entryReason {
		position.EntryReason = entryReason
		updated = true
	}
	if exitPlan != "" && position.ExitPlan != exitPlan {
		position.ExitPlan = exitPlan
		updated = true
	}

	if !updated {
		return nil
	}

	return s.PositionRepo.Save(ctx, &position)
}

// StartSyncWorker 启动后台持仓同步worker
func (s *PositionService) StartSyncWorker(ctx context.Context, interval time.Duration) {
	s.stopChan = make(chan struct{})
	s.stopped = false

	s.logger.Info("starting position sync worker", zap.Duration("interval", interval))

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// 立即执行一次同步
		if err := s.SyncPositions(ctx); err != nil {
			s.logger.Error("failed to sync positions on startup", zap.Error(err))
		}

		for {
			select {
			case <-ticker.C:
				if err := s.SyncPositions(ctx); err != nil {
					s.logger.Error("failed to sync positions", zap.Error(err))
				}
			case <-s.stopChan:
				s.logger.Info("position sync worker stopped")
				return
			case <-ctx.Done():
				s.logger.Info("position sync worker stopped by context")
				return
			}
		}
	}()
}

// StopSyncWorker 停止后台持仓同步worker
func (s *PositionService) StopSyncWorker() {
	if !s.stopped && s.stopChan != nil {
		close(s.stopChan)
		s.stopped = true
		s.logger.Info("position sync worker stop signal sent")
	}
}

// UpdateStopPrices 更新持仓的止损止盈价格
func (s *PositionService) UpdateStopPrices(ctx context.Context, symbol, side string, stopLoss, takeProfit float64) error {
	pos, err := s.PositionRepo.FindBySymbolAndSide(ctx, symbol, side)
	if err != nil {
		return err
	}

	pos.StopLoss = stopLoss
	pos.TakeProfit = takeProfit

	return s.PositionRepo.Save(ctx, &pos)
}
