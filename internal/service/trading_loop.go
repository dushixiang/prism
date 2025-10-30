package service

import (
	"context"
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
	promptService *PromptService,
	agentService *AgentService,
	logger *zap.Logger,
) *TradingLoop {
	return &TradingLoop{
		config:          config.Trading,
		marketService:   marketService,
		accountService:  accountService,
		positionService: positionService,
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
		}
	})

	if err != nil {
		t.isRunning = false
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	// 启动 cron 调度器
	t.cron.Start()

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
	t.logger.Info("[STEP 1/6] Collecting market data...")
	marketData, err := t.marketService.CollectAllSymbols(ctx, t.config.Symbols)
	if err != nil {
		return fmt.Errorf("step 1 failed - collect market data: %w", err)
	}
	t.logger.Info("[STEP 1/6] Market data collected",
		zap.Int("symbols_count", len(marketData)))

	// ========== Step 2: 获取账户信息 ==========
	t.logger.Info("[STEP 2/6] Getting account metrics...")
	accountMetrics, err := t.accountService.GetAccountMetrics(ctx)
	if err != nil {
		return fmt.Errorf("step 2 failed - get account metrics: %w", err)
	}
	t.logger.Info("[STEP 2/6] Account metrics retrieved",
		zap.Float64("total_balance", accountMetrics.TotalBalance),
		zap.Float64("return_percent", accountMetrics.ReturnPercent),
		zap.Float64("drawdown_from_peak", accountMetrics.DrawdownFromPeak),
		zap.Float64("sharpe_ratio", accountMetrics.SharpeRatio))

	// ========== Step 3: 同步持仓数据 ==========
	t.logger.Info("[STEP 3/6] Syncing positions...")
	if err := t.positionService.SyncPositions(ctx); err != nil {
		return fmt.Errorf("step 3 failed - sync positions: %w", err)
	}
	positions, _ := t.positionService.GetAllPositions(ctx)
	t.logger.Info("[STEP 3/6] Positions synced",
		zap.Int("position_count", len(positions)))

	// ========== Step 4: 生成AI提示词 ==========
	t.logger.Info("[STEP 4/6] Generating LLM prompt...")

	// 获取历史交易
	recentTrades, _ := t.agentService.GetRecentTrades(ctx, 10)

	// 准备夏普比率（如果有效则传入）
	var sharpeRatio *float64
	if accountMetrics.SharpeRatio != 0.0 {
		sharpeRatio = &accountMetrics.SharpeRatio
	}

	promptData := &PromptData{
		StartTime:      t.startTime,
		Iteration:      t.iteration,
		AccountMetrics: accountMetrics,
		MarketDataMap:  marketData,
		Positions:      positions,
		RecentTrades:   recentTrades,
		SharpeRatio:    sharpeRatio,
	}

	prompt := t.promptService.GeneratePrompt(ctx, promptData)
	systemInstructions := t.promptService.GetSystemInstructions()

	t.logger.Info("[STEP 4/6] LLM prompt generated",
		zap.Int("prompt_length", len(prompt)))

	// ========== Step 5: LLM Agent决策 ==========
	t.logger.Info("[STEP 5/6] Executing LLM decision...")

	// 先创建决策记录以获取决策ID（先保存一个占位记录）
	decisionID, err := t.agentService.SaveDecision(ctx, t.iteration, accountMetrics.TotalBalance,
		len(positions), "执行中...", 0, 0)
	if err != nil {
		t.logger.Error("[STEP 5/6] Failed to create decision record", zap.Error(err))
		return fmt.Errorf("step 5 failed - create decision: %w", err)
	}

	// 执行LLM决策
	decision, err := t.agentService.ExecuteDecision(ctx, decisionID, systemInstructions, prompt, accountMetrics)
	if err != nil {
		t.logger.Error("[STEP 5/6] LLM decision failed", zap.Error(err))
		return fmt.Errorf("step 5 failed - LLM decision: %w", err)
	}

	t.logger.Info("[STEP 5/6] LLM decision executed",
		zap.Int("tools_called", decision.ToolsCalled),
		zap.Int("prompt_tokens", decision.PromptTokens),
		zap.Int("completion_tokens", decision.CompletionTokens),
		zap.String("decision_preview", truncateString(decision.DecisionText, 200)))

	// 更新决策记录为完整内容
	if err := t.agentService.UpdateDecision(ctx, decisionID, decision.DecisionText, decision.PromptTokens, decision.CompletionTokens); err != nil {
		t.logger.Error("failed to update decision", zap.Error(err))
	}

	// ========== Step 6: 执行后处理 ==========
	t.logger.Info("[STEP 6/6] Performing post-processing...")

	// 6a. 重新同步持仓
	if err := t.positionService.SyncPositions(ctx); err != nil {
		t.logger.Error("failed to re-sync positions", zap.Error(err))
	}

	// 6b. 重新获取账户信息
	finalAccountMetrics, err := t.accountService.GetAccountMetrics(ctx)
	if err != nil {
		t.logger.Error("failed to get final account metrics", zap.Error(err))
		finalAccountMetrics = accountMetrics
	} else {
		// 6c. 保存账户历史
		if err := t.accountService.SaveAccountHistory(ctx, finalAccountMetrics, t.iteration); err != nil {
			t.logger.Error("failed to save account history", zap.Error(err))
		}
	}

	// 6d. 获取最终持仓
	finalPositions, _ := t.positionService.GetAllPositions(ctx)

	t.logger.Info("[STEP 6/6] Post-processing completed")

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
			)
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
