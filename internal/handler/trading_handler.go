package handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/dushixiang/prism/internal/service"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// TradingHandler 交易系统HTTP处理器
type TradingHandler struct {
	tradingLoop     *service.TradingLoop
	accountService  *service.TradingAccountService
	positionService *service.PositionService
	agentService    *service.AgentService
	logger          *zap.Logger
	loopCtx         context.Context
	loopCancel      context.CancelFunc
}

// NewTradingHandler 创建交易处理器
func NewTradingHandler(
	tradingLoop *service.TradingLoop,
	accountService *service.TradingAccountService,
	positionService *service.PositionService,
	agentService *service.AgentService,
	logger *zap.Logger,
) *TradingHandler {
	return &TradingHandler{
		tradingLoop:     tradingLoop,
		accountService:  accountService,
		positionService: positionService,
		agentService:    agentService,
		logger:          logger,
	}
}

// GetStatus 获取交易状态
// GET /api/trading/status
func (h *TradingHandler) GetStatus(c echo.Context) error {
	ctx := c.Request().Context()

	// 获取交易循环状态
	loopStatus, err := h.tradingLoop.GetStatus(ctx)
	if err != nil {
		return err
	}

	// 获取账户信息
	accountMetrics, err := h.accountService.GetAccountMetrics(ctx)
	if err != nil {
		h.logger.Error("failed to get account metrics", zap.Error(err))
		return c.JSON(http.StatusOK, map[string]interface{}{
			"loop": loopStatus,
		})
	}

	// 获取持仓
	positions, err := h.positionService.GetAllPositions(ctx)
	if err != nil {
		h.logger.Error("failed to get positions", zap.Error(err))
	}

	positionsData := make([]map[string]interface{}, 0, len(positions))
	for _, pos := range positions {
		positionsData = append(positionsData, map[string]interface{}{
			"id":             pos.ID,
			"symbol":         pos.Symbol,
			"side":           pos.Side,
			"quantity":       pos.Quantity,
			"entry_price":    pos.EntryPrice,
			"current_price":  pos.CurrentPrice,
			"unrealized_pnl": pos.UnrealizedPnl,
			"pnl_percent":    pos.CalculatePnlPercent(),
			"leverage":       pos.Leverage,
			"holding":        pos.CalculateHoldingStr(),
			"opened_at":      pos.OpenedAt,
			"entry_reason":   pos.EntryReason,
			"exit_plan":      pos.ExitPlan,
			"stop_loss":      pos.StopLoss,
			"take_profit":    pos.TakeProfit,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"loop": loopStatus,
		"account": map[string]interface{}{
			"total_balance":         accountMetrics.TotalBalance,
			"available":             accountMetrics.Available,
			"unrealised_pnl":        accountMetrics.UnrealisedPnl,
			"return_percent":        accountMetrics.ReturnPercent,
			"drawdown_from_peak":    accountMetrics.DrawdownFromPeak,
			"drawdown_from_initial": accountMetrics.DrawdownFromInitial,
			"sharpe_ratio":          accountMetrics.SharpeRatio,
		},
		"positions": positionsData,
	})
}

// GetAccount 获取账户信息
// GET /api/trading/account
func (h *TradingHandler) GetAccount(c echo.Context) error {
	ctx := c.Request().Context()

	accountMetrics, err := h.accountService.GetAccountMetrics(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"total_balance":         accountMetrics.TotalBalance,
		"available":             accountMetrics.Available,
		"unrealised_pnl":        accountMetrics.UnrealisedPnl,
		"initial_balance":       accountMetrics.InitialBalance,
		"peak_balance":          accountMetrics.PeakBalance,
		"return_percent":        accountMetrics.ReturnPercent,
		"drawdown_from_peak":    accountMetrics.DrawdownFromPeak,
		"drawdown_from_initial": accountMetrics.DrawdownFromInitial,
		"sharpe_ratio":          accountMetrics.SharpeRatio,
	})
}

// GetPositions 获取持仓列表
// GET /api/trading/positions
func (h *TradingHandler) GetPositions(c echo.Context) error {
	ctx := c.Request().Context()

	positions, err := h.positionService.GetAllPositions(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	positionsData := make([]map[string]interface{}, 0, len(positions))
	for _, pos := range positions {
		positionsData = append(positionsData, map[string]interface{}{
			"id":                pos.ID,
			"symbol":            pos.Symbol,
			"side":              pos.Side,
			"quantity":          pos.Quantity,
			"entry_price":       pos.EntryPrice,
			"current_price":     pos.CurrentPrice,
			"liquidation_price": pos.LiquidationPrice,
			"unrealized_pnl":    pos.UnrealizedPnl,
			"pnl_percent":       pos.CalculatePnlPercent(),
			"leverage":          pos.Leverage,
			"margin":            pos.Margin,
			"peak_pnl_percent":  pos.PeakPnlPercent,
			"holding":           pos.CalculateHoldingStr(),
			"opened_at":         pos.OpenedAt,
			"entry_reason":      pos.EntryReason,
			"exit_plan":         pos.ExitPlan,
			"stop_loss":         pos.StopLoss,
			"take_profit":       pos.TakeProfit,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"count":     len(positions),
		"positions": positionsData,
	})
}

// GetDecisions 获取决策历史
// GET /api/trading/decisions?limit=10
func (h *TradingHandler) GetDecisions(c echo.Context) error {
	ctx := c.Request().Context()

	limit := 10
	if l := c.QueryParam("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	decisions, err := h.agentService.GetRecentDecisions(ctx, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"count":     len(decisions),
		"decisions": decisions,
	})
}

// GetTrades 获取交易历史
// GET /api/trading/trades?limit=20
func (h *TradingHandler) GetTrades(c echo.Context) error {
	ctx := c.Request().Context()

	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	trades, err := h.agentService.GetRecentTrades(ctx, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"count":  len(trades),
		"trades": trades,
	})
}

// GetStats 获取交易统计数据
// GET /api/trading/stats
func (h *TradingHandler) GetStats(c echo.Context) error {
	ctx := c.Request().Context()

	stats, err := h.agentService.GetTradeStats(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, stats)
}

// GetEquityCurve 获取资金曲线数据
// GET /api/trading/equity-curve
func (h *TradingHandler) GetEquityCurve(c echo.Context) error {
	ctx := c.Request().Context()

	// 获取账户历史数据
	histories, err := h.accountService.GetAccountHistories(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	// 转换为前端需要的格式
	data := make([]map[string]interface{}, 0, len(histories))
	for _, h := range histories {
		data = append(data, map[string]interface{}{
			"timestamp":             h.RecordedAt.Unix(), // 转换为秒时间戳
			"time":                  h.RecordedAt,
			"total_balance":         h.TotalBalance,
			"available":             h.Available,
			"unrealised_pnl":        h.UnrealisedPnl,
			"return_percent":        h.ReturnPercent,
			"drawdown_from_peak":    h.DrawdownFromPeak,
			"drawdown_from_initial": h.DrawdownFromInitial,
			"iteration":             h.Iteration,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"count": len(data),
		"data":  data,
	})
}

// Start 启动交易循环
// POST /api/trading/start
func (h *TradingHandler) Start(c echo.Context) error {
	if h.tradingLoop.IsRunning() {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "trading loop is already running",
		})
	}

	// 创建新的context
	h.loopCtx, h.loopCancel = context.WithCancel(context.Background())

	// 在goroutine中启动
	go func() {
		if err := h.tradingLoop.Start(h.loopCtx); err != nil {
			h.logger.Error("trading loop error", zap.Error(err))
		}
	}()

	h.logger.Info("trading loop started via API")

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "trading loop started",
	})
}

// Stop 停止交易循环
// POST /api/trading/stop
func (h *TradingHandler) Stop(c echo.Context) error {
	if !h.tradingLoop.IsRunning() {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "trading loop is not running",
		})
	}

	h.tradingLoop.Stop()
	if h.loopCancel != nil {
		h.loopCancel()
	}

	h.logger.Info("trading loop stopped via API")

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "trading loop stopped",
	})
}

// Restart 重启交易循环
// POST /api/trading/restart
func (h *TradingHandler) Restart(c echo.Context) error {
	wasRunning := h.tradingLoop.IsRunning()

	// 如果正在运行，先停止
	if wasRunning {
		h.tradingLoop.Stop()
		if h.loopCancel != nil {
			h.loopCancel()
		}
		h.logger.Info("trading loop stopped for restart")
	}

	// 创建新的context
	h.loopCtx, h.loopCancel = context.WithCancel(context.Background())

	// 在goroutine中启动
	go func() {
		if err := h.tradingLoop.Start(h.loopCtx); err != nil {
			h.logger.Error("trading loop error on restart", zap.Error(err))
		}
	}()

	h.logger.Info("trading loop restarted via API")

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "trading loop restarted",
	})
}

// GetLLMLogs 获取LLM通信日志
// GET /api/trading/llm-logs?decision_id=xxx 或 ?limit=100&iteration=1
func (h *TradingHandler) GetLLMLogs(c echo.Context) error {
	ctx := c.Request().Context()

	// 获取查询参数
	decisionID := c.QueryParam("decision_id")
	if decisionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "decision_id is required",
		})
	}

	var logs interface{}
	var err error

	// 优先按决策ID查询
	logs, err = h.agentService.GetLLMLogsByDecisionID(ctx, decisionID)
	if err != nil {
		h.logger.Error("failed to get LLM logs", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"logs": logs,
	})
}

// RegisterRoutes 注册路由
func (h *TradingHandler) RegisterRoutes(g *echo.Group) {
	trading := g.Group("/trading")

	// 查询接口
	trading.GET("/status", h.GetStatus)
	trading.GET("/account", h.GetAccount)
	trading.GET("/positions", h.GetPositions)
	trading.GET("/decisions", h.GetDecisions)
	trading.GET("/trades", h.GetTrades)
	trading.GET("/stats", h.GetStats)
	trading.GET("/equity-curve", h.GetEquityCurve)
	trading.GET("/llm-logs", h.GetLLMLogs)

	// 控制接口
	trading.POST("/start", h.Start)
	trading.POST("/stop", h.Stop)
	trading.POST("/restart", h.Restart)
}
