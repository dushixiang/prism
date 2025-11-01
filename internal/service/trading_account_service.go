package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/dushixiang/prism/internal/models"
	"github.com/dushixiang/prism/internal/repo"
	"github.com/dushixiang/prism/pkg/exchange"
	"github.com/go-orz/orz"
	"github.com/oklog/ulid/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// TradingAccountService 交易账户管理服务
type TradingAccountService struct {
	logger *zap.Logger

	*orz.Service
	*repo.AccountHistoryRepo

	exchange exchange.Exchange
}

// NewTradingAccountService 创建交易账户服务
func NewTradingAccountService(db *gorm.DB, exchange exchange.Exchange, logger *zap.Logger) *TradingAccountService {
	return &TradingAccountService{
		logger:             logger,
		Service:            orz.NewService(db),
		AccountHistoryRepo: repo.NewAccountHistoryRepo(db),
		exchange:           exchange,
	}
}

// AccountMetrics 账户指标
type AccountMetrics struct {
	TotalBalance        float64 `json:"total_balance"`         // 账户总净值（包含未实现盈亏）
	Available           float64 `json:"available"`             // 可用余额
	UnrealisedPnl       float64 `json:"unrealised_pnl"`        // 未实现盈亏
	InitialBalance      float64 `json:"initial_balance"`       // 初始资金
	PeakBalance         float64 `json:"peak_balance"`          // 峰值资金
	ReturnPercent       float64 `json:"return_percent"`        // 收益率
	DrawdownFromPeak    float64 `json:"drawdown_from_peak"`    // 从峰值的回撤
	DrawdownFromInitial float64 `json:"drawdown_from_initial"` // 从初始的回撤
	SharpeRatio         float64 `json:"sharpe_ratio"`          // 夏普比率
}

// GetAccountMetrics 获取账户指标
func (s *TradingAccountService) GetAccountMetrics(ctx context.Context) (*AccountMetrics, error) {
	// 从交易所获取账户数据
	accountInfo, err := s.exchange.GetAccountInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}

	// 账户总净值（包含未实现盈亏）
	// 注意：accountInfo.TotalBalance 从交易所返回时已经包含了未实现盈亏
	totalBalance := accountInfo.TotalBalance

	// 从数据库获取初始资金和峰值资金
	// 获取初始资金（第一条记录）
	firstHistory, err := s.AccountHistoryRepo.FindInitialBalance(ctx)

	initialBalance := totalBalance
	if err == nil {
		initialBalance = firstHistory.TotalBalance
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		s.logger.Warn("failed to get initial balance", zap.Error(err))
	}

	// 获取峰值资金
	peakHistory, err := s.AccountHistoryRepo.FindPeakBalance(ctx)

	peakBalance := totalBalance
	if err == nil && peakHistory.TotalBalance > totalBalance {
		peakBalance = peakHistory.TotalBalance
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		s.logger.Warn("failed to get peak balance", zap.Error(err))
	}

	// 更新峰值（如果当前余额更高）
	if totalBalance > peakBalance {
		peakBalance = totalBalance
	}

	// 计算收益率
	returnPercent := 0.0
	if initialBalance > 0 {
		returnPercent = (totalBalance - initialBalance) / initialBalance * 100
	}

	// 计算回撤（返回负数表示下跌）
	drawdownFromPeak := 0.0
	if peakBalance > 0 {
		drawdownFromPeak = (totalBalance - peakBalance) / peakBalance * 100
	}

	drawdownFromInitial := 0.0
	if initialBalance > 0 && totalBalance < initialBalance {
		drawdownFromInitial = (totalBalance - initialBalance) / initialBalance * 100
	}

	// 计算Sharpe Ratio
	sharpeRatio := s.calculateSharpeRatio(ctx)

	metrics := &AccountMetrics{
		TotalBalance:        totalBalance,
		Available:           accountInfo.AvailableBalance,
		UnrealisedPnl:       accountInfo.UnrealizedPnl,
		InitialBalance:      initialBalance,
		PeakBalance:         peakBalance,
		ReturnPercent:       returnPercent,
		DrawdownFromPeak:    drawdownFromPeak,
		DrawdownFromInitial: drawdownFromInitial,
		SharpeRatio:         sharpeRatio,
	}

	return metrics, nil
}

// calculateSharpeRatio 计算夏普比率
func (s *TradingAccountService) calculateSharpeRatio(ctx context.Context) float64 {
	histories, err := s.AccountHistoryRepo.FindAllOrderByRecordedAt(ctx)

	if err != nil || len(histories) < 2 {
		return 0.0
	}

	// 计算每次的收益率
	returns := make([]float64, 0, len(histories)-1)
	for i := 1; i < len(histories); i++ {
		if histories[i-1].TotalBalance > 0 {
			ret := (histories[i].TotalBalance - histories[i-1].TotalBalance) / histories[i-1].TotalBalance
			returns = append(returns, ret)
		}
	}

	if len(returns) == 0 {
		return 0.0
	}

	// 计算平均收益率
	sum := 0.0
	for _, r := range returns {
		sum += r
	}
	avgReturn := sum / float64(len(returns))

	// 计算标准差
	variance := 0.0
	for _, r := range returns {
		variance += math.Pow(r-avgReturn, 2)
	}
	variance /= float64(len(returns))
	stdDev := math.Sqrt(variance)

	// 计算Sharpe Ratio（假设无风险利率为0）
	if stdDev == 0 {
		return 0.0
	}

	return avgReturn / stdDev
}

// SaveAccountHistory 保存账户历史记录
func (s *TradingAccountService) SaveAccountHistory(ctx context.Context, metrics *AccountMetrics, iteration int) error {
	history := &models.AccountHistory{
		ID:                  ulid.Make().String(),
		TotalBalance:        metrics.TotalBalance,
		Available:           metrics.Available,
		UnrealisedPnl:       metrics.UnrealisedPnl,
		InitialBalance:      metrics.InitialBalance,
		PeakBalance:         metrics.PeakBalance,
		ReturnPercent:       metrics.ReturnPercent,
		DrawdownFromPeak:    metrics.DrawdownFromPeak,
		DrawdownFromInitial: metrics.DrawdownFromInitial,
		SharpeRatio:         metrics.SharpeRatio,
		Iteration:           iteration,
		RecordedAt:          time.Now(),
	}

	return s.AccountHistoryRepo.Create(ctx, history)
}

// GetAccountHistories 获取所有账户历史记录
func (s *TradingAccountService) GetAccountHistories(ctx context.Context) ([]models.AccountHistory, error) {
	return s.AccountHistoryRepo.FindAllOrderByRecordedAt(ctx)
}

// CheckStopLoss 检查账户止损线
func (s *TradingAccountService) CheckStopLoss(metrics *AccountMetrics, stopLossUSDT float64) bool {
	return metrics.TotalBalance <= stopLossUSDT
}

// CheckTakeProfit 检查账户止盈线
func (s *TradingAccountService) CheckTakeProfit(metrics *AccountMetrics, takeProfitUSDT float64) bool {
	return metrics.TotalBalance >= takeProfitUSDT
}
