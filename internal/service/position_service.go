package service

import (
	"context"
	"fmt"
	"strings"
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
