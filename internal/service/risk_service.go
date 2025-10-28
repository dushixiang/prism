package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/dushixiang/prism/internal/models"
	"github.com/dushixiang/prism/pkg/exchange"
	"go.uber.org/zap"
)

// RiskService 风控系统服务
type RiskService struct {
	binanceClient   *exchange.BinanceClient
	positionService *PositionService
	logger          *zap.Logger
}

// NewRiskService 创建风控服务
func NewRiskService(binanceClient *exchange.BinanceClient, positionService *PositionService, logger *zap.Logger) *RiskService {
	return &RiskService{
		binanceClient:   binanceClient,
		positionService: positionService,
		logger:          logger,
	}
}

// RiskCheckResult 风控检查结果
type RiskCheckResult struct {
	ShouldClose bool   `json:"should_close"` // 是否应该平仓
	Reason      string `json:"reason"`       // 平仓原因
}

// CheckPositionRisk 检查单个持仓的风险（36小时、动态止损、移动止盈、峰值回撤）
func (s *RiskService) CheckPositionRisk(ctx context.Context, position *models.Position) (*RiskCheckResult, error) {
	// 计算盈亏百分比
	pnlPercent := position.CalculatePnlPercent()

	// 更新峰值盈亏
	if pnlPercent > position.PeakPnlPercent {
		if err := s.positionService.UpdatePeakPnl(ctx, position.ID, pnlPercent); err != nil {
			s.logger.Warn("failed to update peak pnl", zap.Error(err))
		}
		position.PeakPnlPercent = pnlPercent
	}

	// 1. 检查36小时强制平仓
	holdingHours := position.CalculateHoldingHours()
	if holdingHours >= 36 {
		return &RiskCheckResult{
			ShouldClose: true,
			Reason:      fmt.Sprintf("持仓时间超过36小时限制 (%.1f小时)", holdingHours),
		}, nil
	}

	// 2. 检查动态止损（根据杠杆）
	stopLossPercent := s.getStopLossPercent(position.Leverage)
	if pnlPercent <= stopLossPercent {
		return &RiskCheckResult{
			ShouldClose: true,
			Reason:      fmt.Sprintf("触发动态止损 (盈亏: %.2f%%, 止损线: %.2f%%)", pnlPercent, stopLossPercent),
		}, nil
	}

	// 3. 检查移动止盈
	trailingStopPercent := s.getTrailingStopPercent(pnlPercent, stopLossPercent)
	if pnlPercent < trailingStopPercent && trailingStopPercent > stopLossPercent {
		return &RiskCheckResult{
			ShouldClose: true,
			Reason:      fmt.Sprintf("触发移动止盈 (盈亏: %.2f%%, 移动止损线: %.2f%%)", pnlPercent, trailingStopPercent),
		}, nil
	}

	// 4. 检查峰值回撤保护
	if position.PeakPnlPercent > 5 {
		drawdownFromPeak := (position.PeakPnlPercent - pnlPercent) / position.PeakPnlPercent * 100
		if drawdownFromPeak >= 30 {
			return &RiskCheckResult{
				ShouldClose: true,
				Reason: fmt.Sprintf("触发峰值回撤保护 (峰值: %.2f%%, 当前: %.2f%%, 回撤: %.2f%%)",
					position.PeakPnlPercent, pnlPercent, drawdownFromPeak),
			}, nil
		}
	}

	return &RiskCheckResult{
		ShouldClose: false,
	}, nil
}

// getStopLossPercent 根据杠杆获取止损百分比
func (s *RiskService) getStopLossPercent(leverage int) float64 {
	if leverage >= 12 {
		return -3.0
	} else if leverage >= 8 {
		return -4.0
	}
	return -5.0
}

// getTrailingStopPercent 获取移动止损百分比
func (s *RiskService) getTrailingStopPercent(pnlPercent float64, initialStopLoss float64) float64 {
	if pnlPercent >= 25 {
		return 15.0
	} else if pnlPercent >= 15 {
		return 8.0
	} else if pnlPercent >= 8 {
		return 3.0
	}
	return initialStopLoss
}

// ForceClosePosition 强制平仓
func (s *RiskService) ForceClosePosition(ctx context.Context, position *models.Position, reason string) error {
	s.logger.Info("force closing position",
		zap.String("position_id", position.ID),
		zap.String("symbol", position.Symbol),
		zap.String("side", position.Side),
		zap.String("reason", reason))

	var err error

	// 根据持仓方向平仓
	if position.Side == "long" {
		_, err = s.binanceClient.CloseLongPosition(ctx, position.Symbol, position.Quantity)
	} else {
		_, err = s.binanceClient.CloseShortPosition(ctx, position.Symbol, position.Quantity)
	}

	if err != nil {
		return fmt.Errorf("failed to close position: %w", err)
	}

	// 删除持仓记录
	if err := s.positionService.DeletePosition(ctx, position.ID); err != nil {
		s.logger.Warn("failed to delete position from db", zap.Error(err))
	}

	s.logger.Info("position closed successfully",
		zap.String("position_id", position.ID),
		zap.String("symbol", position.Symbol))

	return nil
}

// CheckAllPositions 检查所有持仓的风险并执行强制平仓
func (s *RiskService) CheckAllPositions(ctx context.Context) (int, error) {
	positions, err := s.positionService.GetAllPositions(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get positions: %w", err)
	}

	closedCount := 0

	for _, position := range positions {
		result, err := s.CheckPositionRisk(ctx, position)
		if err != nil {
			s.logger.Error("failed to check position risk",
				zap.String("position_id", position.ID),
				zap.Error(err))
			continue
		}

		if result.ShouldClose {
			if err := s.ForceClosePosition(ctx, position, result.Reason); err != nil {
				s.logger.Error("failed to force close position",
					zap.String("position_id", position.ID),
					zap.Error(err))
				continue
			}
			closedCount++
		}
	}

	return closedCount, nil
}

// CloseAllPositions 平掉所有持仓
func (s *RiskService) CloseAllPositions(ctx context.Context, reason string) error {
	positions, err := s.positionService.GetAllPositions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	s.logger.Info("closing all positions", zap.String("reason", reason), zap.Int("count", len(positions)))

	for _, position := range positions {
		if err := s.ForceClosePosition(ctx, position, reason); err != nil {
			s.logger.Error("failed to close position",
				zap.String("position_id", position.ID),
				zap.Error(err))
			// 继续平其他仓位
			continue
		}
	}

	return nil
}

// CanOpenNewPosition 检查是否允许开新仓
func (s *RiskService) CanOpenNewPosition(ctx context.Context, accountMetrics *AccountMetrics) (bool, string) {
	// 检查回撤是否超过15%
	if accountMetrics.DrawdownFromPeak >= 15 {
		return false, "账户回撤≥15%，禁止新开仓"
	}

	// 检查当前持仓数量
	positions, err := s.positionService.GetAllPositions(ctx)
	if err != nil {
		s.logger.Error("failed to get positions", zap.Error(err))
		return false, "无法获取持仓信息"
	}

	if len(positions) >= 5 {
		return false, "持仓数量已达上限（5个）"
	}

	return true, ""
}

// CalculatePositionSize 计算仓位大小
func (s *RiskService) CalculatePositionSize(accountValue float64, riskPercent float64, leverage int, stopLossPercent float64) float64 {
	// 单笔交易风险金额
	riskAmount := accountValue * riskPercent / 100

	// 仓位大小 = 风险金额 / (止损百分比 × 杠杆)
	if stopLossPercent == 0 {
		stopLossPercent = float64(s.getStopLossPercent(leverage))
		if stopLossPercent < 0 {
			stopLossPercent = -stopLossPercent
		}
	}

	positionSize := riskAmount / (stopLossPercent / 100 * float64(leverage))

	return positionSize
}

// ValidateLeverage 验证杠杆是否在允许范围内
func (s *RiskService) ValidateLeverage(leverage int) bool {
	return leverage >= 5 && leverage <= 15
}

// GetRecommendedLeverage 根据信号强度推荐杠杆
func (s *RiskService) GetRecommendedLeverage(confluenceCount int) int {
	if confluenceCount >= 4 {
		return 12 // 4个或更多时间框架一致：12-15倍
	} else if confluenceCount >= 3 {
		return 10 // 3个时间框架一致：8-12倍
	} else if confluenceCount >= 2 {
		return 7 // 2个时间框架一致：5-8倍
	}
	return 5 // 默认最低杠杆
}

// SetupPositionLeverage 设置持仓杠杆
func (s *RiskService) SetupPositionLeverage(ctx context.Context, symbol string, leverage int) error {
	// 验证杠杆
	if !s.ValidateLeverage(leverage) {
		return fmt.Errorf("invalid leverage: %d (must be between 5 and 15)", leverage)
	}

	// 设置逐仓模式
	if err := s.binanceClient.SetMarginType(ctx, symbol, futures.MarginTypeIsolated); err != nil {
		// 如果保证金类型已经是逐仓模式,忽略错误(-4046: No need to change margin type)
		errMsg := err.Error()
		if !strings.Contains(errMsg, "code=-4046") && !strings.Contains(errMsg, "No need to change margin type") {
			s.logger.Warn("failed to set margin type", zap.Error(err))
		}
	}

	// 设置杠杆
	if err := s.binanceClient.SetLeverage(ctx, symbol, leverage); err != nil {
		return fmt.Errorf("failed to set leverage: %w", err)
	}

	return nil
}
