package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/dushixiang/prism/internal/config"
	"github.com/dushixiang/prism/internal/models"
	"github.com/dushixiang/prism/internal/repo"
	"github.com/dushixiang/prism/pkg/exchange"
	"github.com/go-orz/orz"
	"github.com/oklog/ulid/v2"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"
	"github.com/openai/openai-go/shared/constant"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AgentService LLM Agent决策执行服务
type AgentService struct {
	logger *zap.Logger

	*orz.Service
	*repo.TradeRepo
	*repo.DecisionRepo

	openAIClient    *openai.Client
	binanceClient   *exchange.BinanceClient
	positionService *PositionService
	tradingConf     config.TradingConf
	model           string
}

// NewAgentService 创建AI Agent服务
func NewAgentService(
	db *gorm.DB,
	openAIClient *openai.Client,
	binanceClient *exchange.BinanceClient,
	positionService *PositionService,
	logger *zap.Logger,
	conf *config.Config,
) *AgentService {
	return &AgentService{
		logger:          logger,
		Service:         orz.NewService(db),
		TradeRepo:       repo.NewTradeRepo(db),
		DecisionRepo:    repo.NewDecisionRepo(db),
		openAIClient:    openAIClient,
		binanceClient:   binanceClient,
		positionService: positionService,
		tradingConf:     conf.Trading,
		model:           conf.LLM.Model,
	}
}

// DecisionResult AI决策结果
type DecisionResult struct {
	DecisionText     string `json:"decision_text"`
	ToolsCalled      int    `json:"tools_called"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
}

// ExecuteDecision 执行AI决策
func (s *AgentService) ExecuteDecision(ctx context.Context, systemInstructions string, prompt string, accountMetrics *AccountMetrics) (*DecisionResult, error) {
	s.logger.Info("executing LLM decision")

	// 构建工具函数定义
	tools := s.buildOpenAITools(accountMetrics)

	// 构建消息
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemInstructions),
		openai.UserMessage(prompt),
	}

	// 处理响应和工具调用
	toolsCalled := 0
	var finalText strings.Builder
	totalPromptTokens := 0
	totalCompletionTokens := 0

	maxIterations := 10 // 防止无限循环
	for iteration := 0; iteration < maxIterations; iteration++ {
		// 调用 OpenAI API
		resp, err := s.openAIClient.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:    shared.ChatModel(s.model),
			Messages: messages,
			Tools:    tools,
		})

		if err != nil {
			return nil, fmt.Errorf("failed to call OpenAI API: %w", err)
		}

		// 累计 token 使用
		totalPromptTokens += int(resp.Usage.PromptTokens)
		totalCompletionTokens += int(resp.Usage.CompletionTokens)

		if len(resp.Choices) == 0 {
			break
		}

		choice := resp.Choices[0]
		message := choice.Message

		// 添加助手消息到对话历史
		messages = append(messages, message.ToParam())

		// 检查是否有工具调用
		if len(message.ToolCalls) == 0 {
			// 没有工具调用，获取最终文本并结束
			if message.Content != "" {
				finalText.WriteString(message.Content)
			}
			break
		}

		// 处理工具调用
		var toolMessages []openai.ChatCompletionMessageParamUnion
		for _, toolCall := range message.ToolCalls {
			toolsCalled++

			// 解析参数
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				s.logger.Error("failed to parse tool arguments",
					zap.String("function", toolCall.Function.Name),
					zap.Error(err))
				// 即使解析失败，也要返回错误响应，不能跳过
				result := map[string]interface{}{
					"error": fmt.Sprintf("failed to parse arguments: %v", err),
				}
				resultJSON, _ := json.Marshal(result)
				toolMessages = append(toolMessages, openai.ToolMessage(toolCall.ID, string(resultJSON)))
				continue
			}

			s.logger.Info("LLM called tool",
				zap.String("function", toolCall.Function.Name),
				zap.Any("args", args))

			// 执行工具函数
			result, err := s.executeToolFunction(ctx, toolCall.Function.Name, args)
			if err != nil {
				s.logger.Error("tool execution failed",
					zap.String("function", toolCall.Function.Name),
					zap.Error(err))
				result = map[string]interface{}{
					"error": err.Error(),
				}
			}

			// 将结果转换为 JSON
			resultJSON, _ := json.Marshal(result)

			// 添加工具响应消息
			toolMessages = append(toolMessages, openai.ToolMessage(string(resultJSON), toolCall.ID))
		}

		// 将工具响应添加到对话历史
		messages = append(messages, toolMessages...)

		// 如果是最后一次迭代，记录警告
		if iteration == maxIterations-1 {
			s.logger.Warn("reached max iterations for tool calls")
		}
	}

	return &DecisionResult{
		DecisionText:     finalText.String(),
		ToolsCalled:      toolsCalled,
		PromptTokens:     totalPromptTokens,
		CompletionTokens: totalCompletionTokens,
	}, nil
}

// buildOpenAITools 构建 OpenAI 工具函数定义
func (s *AgentService) buildOpenAITools(accountMetrics *AccountMetrics) []openai.ChatCompletionToolParam {
	functionType := constant.Function("").Default()

	return []openai.ChatCompletionToolParam{
		{
			Type: functionType,
			Function: shared.FunctionDefinitionParam{
				Name:        "getMarketPrice",
				Description: openai.String("获取指定交易对的当前市场价格"),
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"symbol": map[string]interface{}{
							"type":        "string",
							"description": "交易对符号，如 BTCUSDT",
						},
					},
					"required": []string{"symbol"},
				},
			},
		},
		{
			Type: functionType,
			Function: shared.FunctionDefinitionParam{
				Name:        "getAccountBalance",
				Description: openai.String("获取账户余额和未实现盈亏"),
				Parameters: shared.FunctionParameters{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: functionType,
			Function: shared.FunctionDefinitionParam{
				Name:        "getPositions",
				Description: openai.String("获取当前所有持仓信息"),
				Parameters: shared.FunctionParameters{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: functionType,
			Function: shared.FunctionDefinitionParam{
				Name:        "openPosition",
				Description: openai.String("开仓交易（做多或做空）。开仓前必须先设置杠杆。"),
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"symbol": map[string]interface{}{
							"type":        "string",
							"description": "交易对，如 BTCUSDT",
						},
						"side": map[string]interface{}{
							"type":        "string",
							"description": "方向：long（做多）或 short（做空）",
							"enum":        []string{"long", "short"},
						},
						"leverage": map[string]interface{}{
							"type":        "integer",
							"description": "杠杆倍数（5-15），必须根据信号强度选择",
						},
						"quantity": map[string]interface{}{
							"type":        "number",
							"description": "保证金金额（USDT）。注意：实际开仓的名义价值 = 保证金 × 杠杆。例如用100 USDT保证金，10倍杠杆，实际开仓价值1000 USDT",
						},
						"reason": map[string]interface{}{
							"type":        "string",
							"description": "开仓理由，说明信号来源和时间框架共振情况",
						},
					},
					"required": []string{"symbol", "side", "leverage", "quantity", "reason"},
				},
			},
		},
		{
			Type: functionType,
			Function: shared.FunctionDefinitionParam{
				Name:        "closePosition",
				Description: openai.String("平仓指定持仓"),
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"symbol": map[string]interface{}{
							"type":        "string",
							"description": "交易对",
						},
						"reason": map[string]interface{}{
							"type":        "string",
							"description": "平仓理由",
						},
					},
					"required": []string{"symbol", "reason"},
				},
			},
		},
		{
			Type: functionType,
			Function: shared.FunctionDefinitionParam{
				Name:        "calculateRisk",
				Description: openai.String("计算建议的保证金大小。返回的 position_size_usdt 就是应该传给 openPosition 的 quantity 参数值"),
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"leverage": map[string]interface{}{
							"type":        "integer",
							"description": "计划使用的杠杆倍数",
						},
						"riskPercent": map[string]interface{}{
							"type":        "number",
							"description": "单笔交易风险百分比（2-3%）",
						},
					},
					"required": []string{"leverage", "riskPercent"},
				},
			},
		},
	}
}

// executeToolFunction 执行工具函数
func (s *AgentService) executeToolFunction(ctx context.Context, functionName string, args map[string]interface{}) (map[string]interface{}, error) {
	switch functionName {
	case "getMarketPrice":
		return s.toolGetMarketPrice(ctx, args)
	case "getAccountBalance":
		return s.toolGetAccountBalance(ctx)
	case "getPositions":
		return s.toolGetPositions(ctx)
	case "openPosition":
		return s.toolOpenPosition(ctx, args)
	case "closePosition":
		return s.toolClosePosition(ctx, args)
	case "calculateRisk":
		return s.toolCalculateRisk(ctx, args)
	default:
		return nil, fmt.Errorf("unknown function: %s", functionName)
	}
}

// toolGetMarketPrice 获取市场价格
func (s *AgentService) toolGetMarketPrice(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	symbol, ok := args["symbol"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid symbol parameter")
	}

	price, err := s.binanceClient.GetCurrentPrice(ctx, symbol)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"symbol": symbol,
		"price":  price,
	}, nil
}

// toolGetAccountBalance 获取账户余额
func (s *AgentService) toolGetAccountBalance(ctx context.Context) (map[string]interface{}, error) {
	accountInfo, err := s.binanceClient.GetAccountInfo(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_balance":     accountInfo.TotalBalance,
		"available_balance": accountInfo.AvailableBalance,
		"unrealized_pnl":    accountInfo.UnrealizedPnl,
	}, nil
}

// toolGetPositions 获取持仓
func (s *AgentService) toolGetPositions(ctx context.Context) (map[string]interface{}, error) {
	positions, err := s.positionService.GetAllPositions(ctx)
	if err != nil {
		return nil, err
	}

	positionsData := make([]map[string]interface{}, 0, len(positions))
	for _, pos := range positions {
		positionsData = append(positionsData, map[string]interface{}{
			"symbol":         pos.Symbol,
			"side":           pos.Side,
			"quantity":       pos.Quantity,
			"entry_price":    pos.EntryPrice,
			"current_price":  pos.CurrentPrice,
			"unrealized_pnl": pos.UnrealizedPnl,
			"pnl_percent":    pos.CalculatePnlPercent(),
			"leverage":       pos.Leverage,
			"holding_hours":  pos.CalculateHoldingHours(),
		})
	}

	return map[string]interface{}{
		"count":     len(positions),
		"positions": positionsData,
	}, nil
}

// toolOpenPosition 开仓
func (s *AgentService) toolOpenPosition(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	// 解析参数
	symbol, _ := args["symbol"].(string)
	side, _ := args["side"].(string)
	leverageFloat, _ := args["leverage"].(float64)
	leverage := int(leverageFloat)
	quantity, _ := args["quantity"].(float64)
	reason, _ := args["reason"].(string)

	s.logger.Info("opening position",
		zap.String("symbol", symbol),
		zap.String("side", side),
		zap.Int("leverage", leverage),
		zap.Float64("margin_usdt", quantity),
		zap.String("reason", reason))

	// 验证参数
	if symbol == "" || side == "" {
		return nil, fmt.Errorf("symbol and side are required")
	}
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive, got %.8f USDT", quantity)
	}
	if quantity < 5 {
		return nil, fmt.Errorf("保证金太小（%.2f USDT），最少需要 5 USDT 才能满足币安最小名义价值要求", quantity)
	}

	// 验证杠杆
	if !s.validateLeverage(leverage) {
		minLeverage, maxLeverage := s.leverageBounds()
		return nil, fmt.Errorf("invalid leverage: %d (allowed range %d-%d)", leverage, minLeverage, maxLeverage)
	}

	// 设置杠杆
	if err := s.setupPositionLeverage(ctx, symbol, leverage); err != nil {
		return nil, fmt.Errorf("failed to setup leverage: %w", err)
	}

	// 获取当前价格计算数量
	price, err := s.binanceClient.GetCurrentPrice(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get current price: %w", err)
	}

	// 计算实际数量
	// quantity 是保证金（USDT），实际名义价值 = quantity × leverage
	// 币的数量 = 名义价值 / 价格
	notionalValue := quantity * float64(leverage)
	actualQuantity := notionalValue / price

	s.logger.Info("calculated order quantity",
		zap.Float64("margin_usdt", quantity),
		zap.Int("leverage", leverage),
		zap.Float64("notional_value", notionalValue),
		zap.Float64("price", price),
		zap.Float64("coin_quantity", actualQuantity))

	// 执行开仓
	var order *exchange.OrderResult
	if side == "long" {
		order, err = s.binanceClient.OpenLongPosition(ctx, symbol, actualQuantity)
	} else {
		order, err = s.binanceClient.OpenShortPosition(ctx, symbol, actualQuantity)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to open position: %w", err)
	}

	// 如果 AvgPrice 为 0,使用当前价格
	avgPrice := order.AvgPrice
	if avgPrice == 0 {
		avgPrice = price
	}

	// 如果 ExecutedQty 为 0,使用实际下单数量
	executedQty := order.ExecutedQty
	if executedQty == 0 {
		executedQty = actualQuantity
	}

	// 记录交易
	trade := &models.Trade{
		ID:         ulid.Make().String(),
		Symbol:     symbol,
		Type:       "open",
		Side:       side,
		Price:      avgPrice,
		Quantity:   executedQty,
		Leverage:   leverage,
		OrderID:    fmt.Sprintf("%d", order.OrderID),
		ExecutedAt: time.Now(),
	}

	if err := s.TradeRepo.Create(ctx, trade); err != nil {
		s.logger.Error("failed to save trade", zap.Error(err))
	}

	// 同步本地持仓，保证前端能立即看到最新仓位
	if err := s.positionService.SyncPositions(ctx); err != nil {
		s.logger.Warn("failed to sync positions after opening position", zap.Error(err))
	}

	return map[string]interface{}{
		"success":  true,
		"order_id": order.OrderID,
		"symbol":   symbol,
		"side":     side,
		"price":    avgPrice,
		"quantity": executedQty,
		"leverage": leverage,
		"message":  fmt.Sprintf("成功开仓 %s %s，杠杆 %dx，价格 $%.2f", side, symbol, leverage, avgPrice),
	}, nil
}

// toolClosePosition 平仓
func (s *AgentService) toolClosePosition(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	symbol, _ := args["symbol"].(string)
	reason, _ := args["reason"].(string)

	s.logger.Info("closing position",
		zap.String("symbol", symbol),
		zap.String("reason", reason))

	// 获取持仓
	positions, err := s.positionService.GetAllPositions(ctx)
	if err != nil {
		return nil, err
	}

	var targetPosition *models.Position
	for _, pos := range positions {
		if pos.Symbol == symbol {
			targetPosition = pos
			break
		}
	}

	if targetPosition == nil {
		return nil, fmt.Errorf("no position found for symbol %s", symbol)
	}

	// 获取当前价格用于记录
	currentPrice, err := s.binanceClient.GetCurrentPrice(ctx, symbol)
	if err != nil {
		s.logger.Warn("failed to get current price for close position", zap.Error(err))
		currentPrice = targetPosition.CurrentPrice
	}

	// 执行平仓
	var order *exchange.OrderResult
	if targetPosition.Side == "long" {
		order, err = s.binanceClient.CloseLongPosition(ctx, symbol, targetPosition.Quantity)
	} else {
		order, err = s.binanceClient.CloseShortPosition(ctx, symbol, targetPosition.Quantity)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to close position: %w", err)
	}

	// 如果 AvgPrice 为 0,使用当前价格
	avgPrice := order.AvgPrice
	if avgPrice == 0 {
		avgPrice = currentPrice
	}

	// 如果 ExecutedQty 为 0,使用持仓数量
	executedQty := order.ExecutedQty
	if executedQty == 0 {
		executedQty = targetPosition.Quantity
	}

	// 计算盈亏
	pnl := targetPosition.UnrealizedPnl

	// 记录交易
	trade := &models.Trade{
		ID:         ulid.Make().String(),
		Symbol:     symbol,
		Type:       "close",
		Side:       targetPosition.Side,
		Price:      avgPrice,
		Quantity:   executedQty,
		Leverage:   targetPosition.Leverage,
		Pnl:        pnl,
		OrderID:    fmt.Sprintf("%d", order.OrderID),
		PositionID: targetPosition.ID,
		ExecutedAt: time.Now(),
	}

	if err := s.TradeRepo.Create(ctx, trade); err != nil {
		s.logger.Error("failed to save trade", zap.Error(err))
	}

	// 删除持仓记录
	if err := s.positionService.DeletePosition(ctx, targetPosition.ID); err != nil {
		s.logger.Error("failed to delete position", zap.Error(err))
	}

	// 再次同步，确保剩余仓位状态统一
	if err := s.positionService.SyncPositions(ctx); err != nil {
		s.logger.Warn("failed to sync positions after closing position", zap.Error(err))
	}

	return map[string]interface{}{
		"success":  true,
		"order_id": order.OrderID,
		"symbol":   symbol,
		"pnl":      pnl,
		"message":  fmt.Sprintf("成功平仓 %s，盈亏 $%.2f", symbol, pnl),
	}, nil
}

// toolCalculateRisk 计算风险和仓位
func (s *AgentService) toolCalculateRisk(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	leverageFloat, _ := args["leverage"].(float64)
	leverage := int(leverageFloat)
	riskPercent, _ := args["riskPercent"].(float64)

	// 获取账户信息
	accountInfo, err := s.binanceClient.GetAccountInfo(ctx)
	if err != nil {
		return nil, err
	}

	accountValue := accountInfo.TotalBalance - accountInfo.UnrealizedPnl

	// 计算仓位大小
	positionSize := s.calculatePositionSize(accountValue, riskPercent, leverage, 0)

	// 计算止损百分比
	stopLossPercent := s.stopLossPercentForLeverage(leverage)

	// 确保仓位大小至少为 5 USDT
	if positionSize < 5 {
		positionSize = 5
	}

	notionalValue := positionSize * float64(leverage)

	return map[string]interface{}{
		"account_value":       accountValue,
		"risk_percent":        riskPercent,
		"leverage":            leverage,
		"position_size_usdt":  positionSize,
		"notional_value_usdt": notionalValue,
		"stop_loss_percent":   stopLossPercent,
		"max_loss_usdt":       accountValue * riskPercent / 100,
		"recommendation":      fmt.Sprintf("建议使用保证金 %.2f USDT，杠杆 %dx，实际开仓价值 %.2f USDT", positionSize, leverage, notionalValue),
		"usage_instruction":   fmt.Sprintf("请将 position_size_usdt (%.2f) 作为 openPosition 的 quantity 参数", positionSize),
	}, nil
}

func (s *AgentService) leverageBounds() (int, int) {
	minLeverage := s.tradingConf.MinLeverage
	maxLeverage := s.tradingConf.MaxLeverage

	if minLeverage <= 0 {
		minLeverage = 1
	}
	if maxLeverage <= 0 {
		maxLeverage = 125
	}
	if maxLeverage < minLeverage {
		maxLeverage = minLeverage
	}

	return minLeverage, maxLeverage
}

func (s *AgentService) validateLeverage(leverage int) bool {
	minLeverage, maxLeverage := s.leverageBounds()
	return leverage >= minLeverage && leverage <= maxLeverage
}

func (s *AgentService) setupPositionLeverage(ctx context.Context, symbol string, leverage int) error {
	if !s.validateLeverage(leverage) {
		minLeverage, maxLeverage := s.leverageBounds()
		return fmt.Errorf("leverage %d out of allowed range %d-%d", leverage, minLeverage, maxLeverage)
	}

	if err := s.binanceClient.SetMarginType(ctx, symbol, futures.MarginTypeIsolated); err != nil {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "code=-4046") && !strings.Contains(errMsg, "No need to change margin type") {
			return fmt.Errorf("failed to set margin type: %w", err)
		}
	}

	if err := s.binanceClient.SetLeverage(ctx, symbol, leverage); err != nil {
		return fmt.Errorf("failed to set leverage: %w", err)
	}

	return nil
}

func (s *AgentService) stopLossPercentForLeverage(leverage int) float64 {
	if leverage <= 0 {
		return -5.0
	}

	switch {
	case leverage >= 12:
		return -3.0
	case leverage >= 8:
		return -4.0
	default:
		return -5.0
	}
}

func (s *AgentService) calculatePositionSize(accountValue float64, riskPercent float64, leverage int, stopLossPercent float64) float64 {
	if leverage <= 0 {
		return 0
	}

	if stopLossPercent == 0 {
		stopLossPercent = s.stopLossPercentForLeverage(leverage)
	}

	if stopLossPercent == 0 {
		return 0
	}

	if stopLossPercent < 0 {
		stopLossPercent = -stopLossPercent
	}

	riskAmount := accountValue * riskPercent / 100
	if stopLossPercent == 0 {
		return 0
	}

	return riskAmount / (stopLossPercent / 100 * float64(leverage))
}

// SaveDecision 保存AI决策记录
func (s *AgentService) SaveDecision(ctx context.Context, iteration int, accountValue float64, positionCount int,
	decisionContent string, promptTokens int, completionTokens int) error {

	decision := &models.Decision{
		ID:               ulid.Make().String(),
		Iteration:        iteration,
		AccountValue:     accountValue,
		PositionCount:    positionCount,
		DecisionContent:  decisionContent,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		Model:            s.model,
		ExecutedAt:       time.Now(),
	}

	return s.DecisionRepo.Create(ctx, decision)
}

// GetLatestIteration 获取最近一次决策的迭代编号
func (s *AgentService) GetLatestIteration(ctx context.Context) (int, error) {
	return s.DecisionRepo.FindLatestIteration(ctx)
}

// GetRecentDecisions 获取最近的决策记录
func (s *AgentService) GetRecentDecisions(ctx context.Context, limit int) ([]*models.Decision, error) {
	decisions, err := s.DecisionRepo.FindRecentDecisions(ctx, limit)
	if err != nil {
		return nil, err
	}

	result := make([]*models.Decision, len(decisions))
	for i := range decisions {
		result[i] = &decisions[i]
	}

	return result, nil
}

// GetRecentTrades 获取最近的交易记录
func (s *AgentService) GetRecentTrades(ctx context.Context, limit int) ([]*models.Trade, error) {
	trades, err := s.TradeRepo.FindRecentTrades(ctx, limit)
	if err != nil {
		return nil, err
	}

	result := make([]*models.Trade, len(trades))
	for i := range trades {
		result[i] = &trades[i]
	}

	return result, nil
}

// MarshalJSON 用于调试
func (r *DecisionResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"decision_text":     r.DecisionText,
		"tools_called":      r.ToolsCalled,
		"prompt_tokens":     r.PromptTokens,
		"completion_tokens": r.CompletionTokens,
	})
}
