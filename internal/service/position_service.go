package service

import (
	"context"
	"fmt"
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

	binanceClient *exchange.BinanceClient
}

// NewPositionService 创建持仓服务
func NewPositionService(db *gorm.DB, binanceClient *exchange.BinanceClient, logger *zap.Logger) *PositionService {
	return &PositionService{
		logger:        logger,
		Service:       orz.NewService(db),
		PositionRepo:  repo.NewPositionRepo(db),
		binanceClient: binanceClient,
	}
}

// SyncPositions 同步持仓数据
func (s *PositionService) SyncPositions(ctx context.Context) error {
	// 从Binance获取实时持仓
	positions, err := s.binanceClient.GetPositions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get positions from binance: %w", err)
	}

	s.logger.Info("syncing positions", zap.Int("count", len(positions)))

	return s.Transaction(ctx, func(ctx context.Context) error {
		// 清除所有现有持仓
		if err := s.PositionRepo.DeleteAll(ctx); err != nil {
			return fmt.Errorf("failed to delete existing positions: %w", err)
		}

		// 插入新的持仓
		for _, p := range positions {
			// 检查是否已存在该持仓的记录（用于保留开仓时间等元数据）
			existingPos, err := s.PositionRepo.FindBySymbolAndSide(ctx, p.Symbol, p.Side)

			// 计算保证金
			margin := p.EntryPrice * p.PositionAmount / float64(p.Leverage)

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

			// 如果找到旧记录，保留某些字段
			if err == nil {
				position.OpenedAt = existingPos.OpenedAt
				position.OrderID = existingPos.OrderID
				position.StopLoss = existingPos.StopLoss
				position.TakeProfit = existingPos.TakeProfit
				position.PeakPnlPercent = existingPos.PeakPnlPercent
			}

			if err := s.PositionRepo.Create(ctx, position); err != nil {
				return fmt.Errorf("failed to create position: %w", err)
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

// GetPositionWarnings 获取持仓警告信息
func (s *PositionService) GetPositionWarnings(position *models.Position) []string {
	warnings := make([]string, 0)

	remainingHours := position.RemainingHours()

	if remainingHours <= 0 {
		warnings = append(warnings, "🚨 持仓时间已超过36小时限制")
	} else if remainingHours < 2 {
		warnings = append(warnings, "⚠️ 警告：即将达到36小时限制，必须立即平仓")
	} else if remainingHours < 4 {
		warnings = append(warnings, "⚠️ 提醒：距离36小时限制不足4小时，请准备平仓")
	}

	return warnings
}
