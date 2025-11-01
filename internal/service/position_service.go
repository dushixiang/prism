package service

import (
	"context"
	"fmt"
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

	exchange exchange.Exchange

	// 后台同步相关
	syncMutex sync.Mutex // 防止并发同步
	stopChan  chan struct{}
	stopped   bool
}

// NewPositionService 创建持仓服务
func NewPositionService(db *gorm.DB, exchange exchange.Exchange, logger *zap.Logger) *PositionService {
	return &PositionService{
		logger:       logger,
		Service:      orz.NewService(db),
		PositionRepo: repo.NewPositionRepo(db),
		exchange:     exchange,
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

	return s.Transaction(ctx, func(ctx context.Context) error {
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
}

// GetAllPositions 获取所有持仓
func (s *PositionService) GetAllPositions(ctx context.Context) ([]*models.Position, error) {
	positions, err := s.PositionRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*models.Position, len(positions))
	for i := range positions {
		result[i] = &positions[i]
	}

	return result, nil
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
