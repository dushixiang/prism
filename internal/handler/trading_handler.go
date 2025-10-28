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
	riskService     *service.RiskService
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
	riskService *service.RiskService,
	logger *zap.Logger,
) *TradingHandler {
	return &TradingHandler{
		tradingLoop:     tradingLoop,
		accountService:  accountService,
		positionService: positionService,
		agentService:    agentService,
		riskService:     riskService,
		logger:          logger,
	}
}

// GetStatus 获取交易状态
// GET /api/trading/status
func (h *TradingHandler) GetStatus(c echo.Context) error {
	ctx := c.Request().Context()

	// 获取交易循环状态
	loopStatus := h.tradingLoop.GetStatus()

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
			"id":              pos.ID,
			"symbol":          pos.Symbol,
			"side":            pos.Side,
			"quantity":        pos.Quantity,
			"entry_price":     pos.EntryPrice,
			"current_price":   pos.CurrentPrice,
			"unrealized_pnl":  pos.UnrealizedPnl,
			"pnl_percent":     pos.CalculatePnlPercent(),
			"leverage":        pos.Leverage,
			"holding_hours":   pos.CalculateHoldingHours(),
			"remaining_hours": pos.RemainingHours(),
			"opened_at":       pos.OpenedAt,
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

	// 获取账户警告
	warnings := h.accountService.GetAccountWarnings(accountMetrics)

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
		"warnings":              warnings,
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
		warnings := h.positionService.GetPositionWarnings(pos)

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
			"holding_hours":     pos.CalculateHoldingHours(),
			"holding_cycles":    pos.CalculateHoldingCycles(),
			"remaining_hours":   pos.RemainingHours(),
			"opened_at":         pos.OpenedAt,
			"warnings":          warnings,
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
			"timestamp":            h.RecordedAt.Unix() * 1000, // 转换为毫秒时间戳
			"time":                 h.RecordedAt,
			"total_balance":        h.TotalBalance,
			"available":            h.Available,
			"unrealised_pnl":       h.UnrealisedPnl,
			"return_percent":       h.ReturnPercent,
			"drawdown_from_peak":   h.DrawdownFromPeak,
			"drawdown_from_initial": h.DrawdownFromInitial,
			"iteration":            h.Iteration,
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
		"status":  h.tradingLoop.GetStatus(),
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

// ForceCloseAll 强制平仓所有持仓
// POST /api/trading/force-close
func (h *TradingHandler) ForceCloseAll(c echo.Context) error {
	ctx := c.Request().Context()

	var req struct {
		Reason string `json:"reason"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "invalid request",
		})
	}

	if req.Reason == "" {
		req.Reason = "手动强制平仓"
	}

	if err := h.riskService.CloseAllPositions(ctx, req.Reason); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	h.logger.Info("force closed all positions via API", zap.String("reason", req.Reason))

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "all positions closed",
	})
}

// RegisterRoutes 注册路由
func (h *TradingHandler) RegisterRoutes(g *echo.Group) {
	trading := g.Group("/trading")

	trading.GET("/status", h.GetStatus)
	trading.GET("/account", h.GetAccount)
	trading.GET("/positions", h.GetPositions)
	trading.GET("/decisions", h.GetDecisions)
	trading.GET("/trades", h.GetTrades)
	trading.GET("/equity-curve", h.GetEquityCurve)
}
