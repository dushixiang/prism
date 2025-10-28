package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dushixiang/prism/internal/config"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// TradingLoop 交易循环调度器
type TradingLoop struct {
	config          config.TradingConf
	marketService   *MarketService
	accountService  *TradingAccountService
	positionService *PositionService
	riskService     *RiskService
	promptService   *PromptService
	agentService    *AgentService
	logger          *zap.Logger

	startTime time.Time
	iteration int
	isRunning bool
	stopChan  chan struct{}
	cron      *cron.Cron
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewTradingLoop 创建交易循环
func NewTradingLoop(
	config *config.Config,
	marketService *MarketService,
	accountService *TradingAccountService,
	positionService *PositionService,
	riskService *RiskService,
	promptService *PromptService,
	agentService *AgentService,
	logger *zap.Logger,
) *TradingLoop {
	return &TradingLoop{
		config:          config.Trading,
		marketService:   marketService,
		accountService:  accountService,
		positionService: positionService,
		riskService:     riskService,
		promptService:   promptService,
		agentService:    agentService,
		logger:          logger,
		startTime:       time.Now(),
		iteration:       0,
		isRunning:       false,
		stopChan:        make(chan struct{}),
	}
}

// Start 启动交易循环
func (t *TradingLoop) Start(ctx context.Context) error {
	if t.isRunning {
		return fmt.Errorf("trading loop is already running")
	}

	t.isRunning = true
	t.startTime = time.Now()
	t.ctx, t.cancel = context.WithCancel(ctx)

	// 加载最近一次执行的迭代编号，避免重启后从 0 开始
	if lastIteration, err := t.agentService.GetLatestIteration(ctx); err != nil {
		t.logger.Warn("failed to load latest iteration, fallback to 0", zap.Error(err))
	} else {
		t.iteration = lastIteration
		t.logger.Info("resume iteration counter from history", zap.Int("iteration", t.iteration))
	}

	// 生成 cron 表达式：每 N 分钟的整点执行
	// 例如 interval=10: "*/10 * * * *" 表示每小时的 0, 10, 20, 30, 40, 50 分执行
	cronExpr := fmt.Sprintf("*/%d * * * *", t.config.IntervalMinutes)

	t.logger.Info("trading loop started",
		zap.Strings("symbols", t.config.Symbols),
		zap.Int("interval_minutes", t.config.IntervalMinutes),
		zap.String("cron_expression", cronExpr))

	// 创建 cron 调度器（使用秒级精度）
	t.cron = cron.New()

	// 添加定时任务
	_, err := t.cron.AddFunc(cronExpr, func() {
		if err := t.ExecuteCycle(context.Background()); err != nil {
			t.logger.Error("cycle execution failed", zap.Error(err))
			// 检查是否是致命错误
			if errors.Is(err, ErrStopLossTriggered) || errors.Is(err, ErrMaxDrawdownTriggered) {
				t.logger.Error("fatal error, stopping trading loop", zap.Error(err))
				t.Stop()
			}
		}
	})

	if err != nil {
		t.isRunning = false
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	// 启动 cron 调度器
	t.cron.Start()

	// 立即执行第一次
	go func() {
		if err := t.ExecuteCycle(context.Background()); err != nil {
			t.logger.Error("first cycle execution failed", zap.Error(err))
		}
	}()

	// 等待停止信号
	select {
	case <-t.stopChan:
		t.logger.Info("trading loop stopped by user")
		return nil
	case <-ctx.Done():
		t.logger.Info("trading loop stopped by context")
		return ctx.Err()
	}
}

// Stop 停止交易循环
func (t *TradingLoop) Stop() {
	if !t.isRunning {
		return
	}

	t.logger.Info("stopping trading loop...")

	// 停止 cron 调度器
	if t.cron != nil {
		ctx := t.cron.Stop()
		<-ctx.Done() // 等待所有任务完成
		t.logger.Info("cron scheduler stopped")
	}

	// 取消 context
	if t.cancel != nil {
		t.cancel()
	}

	t.isRunning = false
	close(t.stopChan)
	t.logger.Info("trading loop stopped")
}

// ExecuteCycle 执行一个完整的交易周期（7步流程）
func (t *TradingLoop) ExecuteCycle(ctx context.Context) error {
	t.iteration++
	cycleStart := time.Now()

	t.logger.Info("========== TRADING CYCLE START ==========",
		zap.Int("iteration", t.iteration),
		zap.Time("start_time", cycleStart))

	// ========== Step 1: 收集市场数据 ==========
	t.logger.Info("[STEP 1/7] Collecting market data...")
	marketData, err := t.marketService.CollectAllSymbols(ctx, t.config.Symbols)
	if err != nil {
		return fmt.Errorf("step 1 failed - collect market data: %w", err)
	}
	t.logger.Info("[STEP 1/7] Market data collected",
		zap.Int("symbols_count", len(marketData)))

	// ========== Step 2: 获取账户信息 ==========
	t.logger.Info("[STEP 2/7] Getting account metrics...")
	accountMetrics, err := t.accountService.GetAccountMetrics(ctx)
	if err != nil {
		return fmt.Errorf("step 2 failed - get account metrics: %w", err)
	}
	t.logger.Info("[STEP 2/7] Account metrics retrieved",
		zap.Float64("total_balance", accountMetrics.TotalBalance),
		zap.Float64("return_percent", accountMetrics.ReturnPercent),
		zap.Float64("drawdown_from_peak", accountMetrics.DrawdownFromPeak))

	// ========== Step 3: 同步持仓数据 ==========
	t.logger.Info("[STEP 3/7] Syncing positions...")
	if err := t.positionService.SyncPositions(ctx); err != nil {
		return fmt.Errorf("step 3 failed - sync positions: %w", err)
	}
	positions, _ := t.positionService.GetAllPositions(ctx)
	t.logger.Info("[STEP 3/7] Positions synced",
		zap.Int("position_count", len(positions)))

	// ========== Step 4: 强制风控检查（最高优先级） ==========
	t.logger.Info("[STEP 4/7] Performing risk control checks...")

	// 4a. 检查账户止损线
	if t.accountService.CheckStopLoss(accountMetrics, t.config.StopLossUSDT) {
		t.logger.Error("[RISK] Account stop loss triggered",
			zap.Float64("balance", accountMetrics.TotalBalance),
			zap.Float64("stop_loss", t.config.StopLossUSDT))

		if err := t.riskService.CloseAllPositions(ctx, "触发账户止损线"); err != nil {
			t.logger.Error("failed to close all positions", zap.Error(err))
		}
		return ErrStopLossTriggered
	}

	// 4b. 检查账户止盈线
	if t.accountService.CheckTakeProfit(accountMetrics, t.config.TakeProfitUSDT) {
		t.logger.Info("[RISK] Account take profit triggered",
			zap.Float64("balance", accountMetrics.TotalBalance),
			zap.Float64("take_profit", t.config.TakeProfitUSDT))

		if err := t.riskService.CloseAllPositions(ctx, "触发账户止盈线"); err != nil {
			t.logger.Error("failed to close all positions", zap.Error(err))
		}
		return ErrTakeProfitTriggered
	}

	// 4c. 检查账户回撤保护
	if accountMetrics.DrawdownFromPeak >= 20 {
		t.logger.Error("[RISK] Maximum drawdown triggered (>=20%)",
			zap.Float64("drawdown", accountMetrics.DrawdownFromPeak))

		if err := t.riskService.CloseAllPositions(ctx, "回撤≥20%，触发强制平仓"); err != nil {
			t.logger.Error("failed to close all positions", zap.Error(err))
		}
		return ErrMaxDrawdownTriggered
	}

	// 4d. 检查所有持仓的风控（36小时、动态止损、移动止盈、峰值回撤）
	closedCount, err := t.riskService.CheckAllPositions(ctx)
	if err != nil {
		t.logger.Error("failed to check positions risk", zap.Error(err))
	} else if closedCount > 0 {
		t.logger.Info("[RISK] Force closed positions",
			zap.Int("closed_count", closedCount))
	}

	t.logger.Info("[STEP 4/7] Risk control checks completed",
		zap.Int("positions_closed", closedCount))

	// 重新获取持仓（可能已被风控平仓）
	positions, _ = t.positionService.GetAllPositions(ctx)

	// 风控操作可能影响账户资金，刷新账户指标以避免后续使用过期数据
	if refreshedMetrics, refreshErr := t.accountService.GetAccountMetrics(ctx); refreshErr != nil {
		t.logger.Warn("failed to refresh account metrics after risk checks", zap.Error(refreshErr))
	} else {
		accountMetrics = refreshedMetrics
		t.logger.Info("[STEP 4/7] Account metrics refreshed after risk controls",
			zap.Float64("total_balance", accountMetrics.TotalBalance),
			zap.Float64("drawdown_from_peak", accountMetrics.DrawdownFromPeak))
	}

	// ========== Step 5: 生成AI提示词 ==========
	t.logger.Info("[STEP 5/7] Generating LLM prompt...")

	// 获取历史交易和决策
	recentTrades, _ := t.agentService.GetRecentTrades(ctx, 10)
	recentDecisions, _ := t.agentService.GetRecentDecisions(ctx, 3)

	promptData := &PromptData{
		StartTime:       t.startTime,
		Iteration:       t.iteration,
		AccountMetrics:  accountMetrics,
		MarketDataMap:   marketData,
		Positions:       positions,
		RecentTrades:    recentTrades,
		RecentDecisions: recentDecisions,
	}

	prompt := t.promptService.GeneratePrompt(ctx, promptData)
	systemInstructions := t.promptService.GetSystemInstructions()

	t.logger.Info("[STEP 5/7] LLM prompt generated",
		zap.Int("prompt_length", len(prompt)))

	// ========== Step 6: LLM Agent决策 ==========
	t.logger.Info("[STEP 6/7] Executing LLM decision...")

	decision, err := t.agentService.ExecuteDecision(ctx, systemInstructions, prompt, accountMetrics)
	if err != nil {
		t.logger.Error("[STEP 6/7] LLM decision failed", zap.Error(err))
		return fmt.Errorf("step 6 failed - LLM decision: %w", err)
	}

	t.logger.Info("[STEP 6/7] LLM decision executed",
		zap.Int("tools_called", decision.ToolsCalled),
		zap.Int("prompt_tokens", decision.PromptTokens),
		zap.Int("completion_tokens", decision.CompletionTokens),
		zap.String("decision_preview", truncateString(decision.DecisionText, 200)))

	// 保存决策记录
	if err := t.agentService.SaveDecision(ctx, t.iteration, accountMetrics.TotalBalance,
		len(positions), decision.DecisionText, decision.PromptTokens, decision.CompletionTokens); err != nil {
		t.logger.Error("failed to save decision", zap.Error(err))
	}

	// ========== Step 7: 执行后处理 ==========
	t.logger.Info("[STEP 7/7] Performing post-processing...")

	// 7a. 重新同步持仓
	if err := t.positionService.SyncPositions(ctx); err != nil {
		t.logger.Error("failed to re-sync positions", zap.Error(err))
	}

	// 7b. 重新获取账户信息
	finalAccountMetrics, err := t.accountService.GetAccountMetrics(ctx)
	if err != nil {
		t.logger.Error("failed to get final account metrics", zap.Error(err))
		finalAccountMetrics = accountMetrics
	} else {
		// 7c. 保存账户历史
		if err := t.accountService.SaveAccountHistory(ctx, finalAccountMetrics, t.iteration); err != nil {
			t.logger.Error("failed to save account history", zap.Error(err))
		}
	}

	// 7d. 获取最终持仓
	finalPositions, _ := t.positionService.GetAllPositions(ctx)

	t.logger.Info("[STEP 7/7] Post-processing completed")

	// ========== 周期总结 ==========
	cycleDuration := time.Since(cycleStart)
	t.logger.Info("========== TRADING CYCLE END ==========",
		zap.Int("iteration", t.iteration),
		zap.Duration("duration", cycleDuration),
		zap.Float64("balance", finalAccountMetrics.TotalBalance),
		zap.Float64("return_percent", finalAccountMetrics.ReturnPercent),
		zap.Int("positions", len(finalPositions)),
		zap.Float64("unrealized_pnl", finalAccountMetrics.UnrealisedPnl))

	// 输出持仓详情
	if len(finalPositions) > 0 {
		t.logger.Info("Current positions:")
		for i, pos := range finalPositions {
			t.logger.Info(fmt.Sprintf("  Position #%d", i+1),
				zap.String("symbol", pos.Symbol),
				zap.String("side", pos.Side),
				zap.Int("leverage", pos.Leverage),
				zap.Float64("pnl_percent", pos.CalculatePnlPercent()),
				zap.Float64("pnl_usdt", pos.UnrealizedPnl),
				zap.Float64("holding_hours", pos.CalculateHoldingHours()))
		}
	}

	return nil
}

// IsRunning 检查是否正在运行
func (t *TradingLoop) IsRunning() bool {
	return t.isRunning
}

// GetStatus 获取状态信息
func (t *TradingLoop) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"is_running":       t.isRunning,
		"iteration":        t.iteration,
		"start_time":       t.startTime,
		"elapsed_hours":    time.Since(t.startTime).Hours(),
		"symbols":          t.config.Symbols,
		"interval_minutes": t.config.IntervalMinutes,
	}
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// 错误定义
var (
	ErrStopLossTriggered    = fmt.Errorf("account stop loss triggered")
	ErrTakeProfitTriggered  = fmt.Errorf("account take profit triggered")
	ErrMaxDrawdownTriggered = fmt.Errorf("maximum drawdown triggered (>=20%%)")
)
