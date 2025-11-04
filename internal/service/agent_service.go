package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dushixiang/prism/internal/config"
	"github.com/dushixiang/prism/internal/models"
	"github.com/dushixiang/prism/internal/repo"
	"github.com/dushixiang/prism/pkg/exchange"
	"github.com/go-orz/orz"
	"github.com/oklog/ulid/v2"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"
	"github.com/openai/openai-go/shared/constant"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AgentService LLM Agent决策执行服务
type AgentService struct {
	logger *zap.Logger

	*orz.Service
	*repo.TradeRepo
	*repo.DecisionRepo
	*repo.LLMLogRepo
	*repo.OrderRepo

	openAIClient       *openai.Client
	exchange           exchange.Exchange
	positionService    *PositionService
	adminConfigService *AdminConfigService
	model              string
}

// NewAgentService 创建AI Agent服务
func NewAgentService(
	logger *zap.Logger,
	db *gorm.DB,
	openAIClient *openai.Client,
	exchange exchange.Exchange,
	positionService *PositionService,
	adminConfigService *AdminConfigService,
	config *config.Config,
) *AgentService {
	return &AgentService{
		logger:             logger,
		Service:            orz.NewService(db),
		TradeRepo:          repo.NewTradeRepo(db),
		DecisionRepo:       repo.NewDecisionRepo(db),
		LLMLogRepo:         repo.NewLLMLogRepo(db),
		OrderRepo:          repo.NewOrderRepo(db),
		openAIClient:       openAIClient,
		exchange:           exchange,
		positionService:    positionService,
		adminConfigService: adminConfigService,
		model:              config.LLM.Model,
	}
}

// DecisionResult AI决策结果
type DecisionResult struct {
	DecisionText     string `json:"decision_text"`
	ToolsCalled      int    `json:"tools_called"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
}

// DecisionRound 决策轮次记录
type DecisionRound struct {
	Reasoning string   // AI的思考过程
	ToolCalls []string // 本轮调用的工具
}

// ExecuteDecision 执行AI决策
func (s *AgentService) ExecuteDecision(ctx context.Context, decisionID string, systemInstructions string, prompt string, accountMetrics *AccountMetrics) (*DecisionResult, error) {
	s.logger.Info("executing LLM decision", zap.String("decision_id", decisionID))

	// 构建工具函数定义
	tools := s.buildOpenAITools(accountMetrics)

	// 构建消息
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemInstructions),
		openai.UserMessage(prompt),
	}

	// 处理响应和工具调用
	toolsCalled := 0
	var finalText string
	var rounds []DecisionRound
	totalPromptTokens := 0
	totalCompletionTokens := 0

	maxIterations := 10 // 防止无限循环
	for iteration := 0; iteration < maxIterations; iteration++ {
		// 记录请求开始时间
		startTime := time.Now()

		// 调用 OpenAI API
		resp, err := s.openAIClient.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:    s.model,
			Messages: messages,
			Tools:    tools,
		})

		// 计算请求耗时
		duration := time.Since(startTime).Milliseconds()

		if err != nil {
			// 记录失败的LLM调用
			s.saveLLMLog(ctx, decisionID, iteration+1, iteration+1, systemInstructions, prompt, messages, "", nil, nil, 0, 0, "", duration, err.Error())
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

		// 准备记录工具调用和响应
		var toolCallsForLog []map[string]interface{}
		var toolResponsesForLog []map[string]interface{}

		// 检查是否有工具调用
		if len(message.ToolCalls) == 0 {
			// 没有工具调用，获取最终文本并结束
			if message.Content != "" {
				finalText = strings.TrimSpace(message.Content)
			}
			// 记录最终的LLM调用（无工具调用）
			finishReason := ""
			if choice.FinishReason != "" {
				finishReason = choice.FinishReason
			}
			s.saveLLMLog(ctx, decisionID, iteration+1, iteration+1, systemInstructions, prompt, messages,
				message.Content, nil, nil,
				int(resp.Usage.PromptTokens), int(resp.Usage.CompletionTokens),
				finishReason, duration, "")
			break
		}

		// 创建本轮记录
		currentRound := DecisionRound{
			Reasoning: strings.TrimSpace(message.Content),
			ToolCalls: make([]string, 0),
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

				// 记录错误的工具调用
				toolSummary := s.formatToolCall(toolCall.Function.Name, args)
				currentRound.ToolCalls = append(currentRound.ToolCalls,
					fmt.Sprintf("✗ %s - 错误: 参数解析失败", toolSummary))

				// 记录到日志
				toolCallsForLog = append(toolCallsForLog, map[string]interface{}{
					"id":        toolCall.ID,
					"function":  toolCall.Function.Name,
					"arguments": toolCall.Function.Arguments,
					"error":     "参数解析失败",
				})
				toolResponsesForLog = append(toolResponsesForLog, map[string]interface{}{
					"tool_call_id": toolCall.ID,
					"error":        "参数解析失败",
				})
				continue
			}

			s.logger.Info("LLM called tool",
				zap.String("function", toolCall.Function.Name),
				zap.Any("args", args))

			// 构建更清晰的工具调用记录
			toolSummary := s.formatToolCall(toolCall.Function.Name, args)

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

			// 记录工具调用和响应到当前轮次
			currentRound.ToolCalls = append(currentRound.ToolCalls, s.formatToolCallWithResult(toolSummary, result))

			// 记录到日志
			toolCallsForLog = append(toolCallsForLog, map[string]interface{}{
				"id":        toolCall.ID,
				"function":  toolCall.Function.Name,
				"arguments": args,
			})
			toolResponsesForLog = append(toolResponsesForLog, map[string]interface{}{
				"tool_call_id": toolCall.ID,
				"result":       result,
			})
		}

		// 保存本轮记录
		rounds = append(rounds, currentRound)

		// 记录本轮的LLM调用（包含工具调用）
		finishReason := ""
		if choice.FinishReason != "" {
			finishReason = choice.FinishReason
		}
		s.saveLLMLog(ctx, decisionID, iteration+1, iteration+1, systemInstructions, prompt, messages,
			message.Content, toolCallsForLog, toolResponsesForLog,
			int(resp.Usage.PromptTokens), int(resp.Usage.CompletionTokens),
			finishReason, duration, "")

		// 将工具响应添加到对话历史
		messages = append(messages, toolMessages...)

		// 如果是最后一次迭代，记录警告
		if iteration == maxIterations-1 {
			s.logger.Warn("reached max iterations for tool calls")
		}
	}

	// 组装最终决策文本
	decisionText := s.buildDecisionText(rounds, finalText)

	return &DecisionResult{
		DecisionText:     decisionText,
		ToolsCalled:      toolsCalled,
		PromptTokens:     totalPromptTokens,
		CompletionTokens: totalCompletionTokens,
	}, nil
}

// formatToolCall 格式化工具调用为易读的文本
func (s *AgentService) formatToolCall(functionName string, args map[string]interface{}) string {
	switch functionName {
	case "openPosition":
		symbol, _ := args["symbol"].(string)
		side, _ := args["side"].(string)
		return fmt.Sprintf("开仓 %s %s", strings.ToUpper(side), symbol)

	case "closePosition":
		symbol, _ := args["symbol"].(string)
		return fmt.Sprintf("平仓 %s", symbol)

	default:
		return fmt.Sprintf("调用工具 %s", functionName)
	}
}

// formatToolCallWithResult 格式化工具调用结果
func (s *AgentService) formatToolCallWithResult(toolSummary string, result map[string]interface{}) string {
	// 提取关键信息
	if msg, ok := result["message"].(string); ok && msg != "" {
		return fmt.Sprintf("✓ %s - %s", toolSummary, msg)
	}

	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return fmt.Sprintf("✗ %s - 错误: %s", toolSummary, errMsg)
	}

	return fmt.Sprintf("✓ %s", toolSummary)
}

// buildDecisionText 构建最终决策文本（按时间顺序）
func (s *AgentService) buildDecisionText(rounds []DecisionRound, finalText string) string {
	var sections []string

	// 按顺序输出每一轮的思考和操作
	for _, round := range rounds {
		var roundContent strings.Builder

		// 本轮的思考
		if round.Reasoning != "" {
			roundContent.WriteString("**思考**\n")
			roundContent.WriteString(round.Reasoning)
			roundContent.WriteString("\n\n")
		}

		// 本轮的操作
		if len(round.ToolCalls) > 0 {
			roundContent.WriteString("**操作**\n")
			for _, toolCall := range round.ToolCalls {
				roundContent.WriteString(toolCall)
				roundContent.WriteString("\n")
			}
		}

		sections = append(sections, roundContent.String())
	}

	// 最终总结
	if finalText != "" {
		sections = append(sections, "【决策总结】\n"+finalText)
	}

	if len(sections) == 0 {
		return "-"
	}

	return strings.Join(sections, "\n\n")
}

// buildOpenAITools 构建 OpenAI 工具函数定义
func (s *AgentService) buildOpenAITools(accountMetrics *AccountMetrics) []openai.ChatCompletionToolParam {
	functionType := constant.Function("").Default()

	return []openai.ChatCompletionToolParam{
		{
			Type: functionType,
			Function: shared.FunctionDefinitionParam{
				Name:        "openPosition",
				Description: openai.String("开仓交易（做多或做空）。开仓后将自动在交易所创建止损单作为最后防线。"),
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
							"description": "杠杆倍数（3-15），必须根据信号强度选择",
						},
						"quantity": map[string]interface{}{
							"type":        "number",
							"description": "保证金金额（USDT）。注意：实际开仓的名义价值 = 保证金 × 杠杆。例如用100 USDT保证金，10倍杠杆，实际开仓价值1000 USDT",
						},
						"stop_loss_price": map[string]interface{}{
							"type":        "number",
							"description": "【必填】止损价格。开仓后会立即在交易所创建止损单。做多时必须低于当前价，做空时必须高于当前价。建议：根据ATR、关键支撑阻力位或风险承受度设置，通常为入场价的3-5%（考虑杠杆后的账户风险）。",
						},
						"take_profit_price": map[string]interface{}{
							"type":        "number",
							"description": "【可选】止盈价格。如果设置，开仓后会在交易所创建止盈单。做多时必须高于当前价，做空时必须低于当前价。建议：基于关键阻力位或风险回报比设置（如2:1或3:1）。不设置则由AI动态管理。",
						},
						"reason": map[string]interface{}{
							"type":        "string",
							"description": "开仓理由，说明信号来源和时间框架共振情况",
						},
						"exit_plan": map[string]interface{}{
							"type":        "string",
							"description": "详细的退出计划，必须明确包含以下至少一种条件：1)止损条件（价格/百分比/指标）；2)止盈条件（目标价/阻力位）；3)结构破坏条件；4)时间条件。平仓时的 reason 必须明确对应这些条件之一。",
						},
					},
					"required": []string{"symbol", "side", "leverage", "quantity", "stop_loss_price", "reason", "exit_plan"},
				},
			},
		},
		{
			Type: functionType,
			Function: shared.FunctionDefinitionParam{
				Name:        "closePosition",
				Description: openai.String("平仓指定持仓。重要：平仓理由必须严格对应该仓位开仓时设置的退出计划(exit_plan)中的具体条件，不能随意平仓。"),
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"symbol": map[string]interface{}{
							"type":        "string",
							"description": "交易对",
						},
						"reason": map[string]interface{}{
							"type":        "string",
							"description": "平仓理由。必须明确说明触发了该仓位退出计划中的哪个具体条件（如止损、止盈、结构破坏等）。理由必须包含退出计划中的关键要素（价格、指标、条件等）。示例：\"触发止损，价格跌破 $95,000\" 或 \"达到目标价 $105,000，突破阻力位\" 或 \"市场结构破坏，跌破上升趋势线\"。不能使用模糊或无关的理由。",
						},
					},
					"required": []string{"symbol", "reason"},
				},
			},
		},
		{
			Type: functionType,
			Function: shared.FunctionDefinitionParam{
				Name:        "updateStopOrders",
				Description: openai.String("更新持仓的止损止盈单。用于移动止损保护利润、调整止盈目标等。会取消旧的止损止盈单并创建新的。"),
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"symbol": map[string]interface{}{
							"type":        "string",
							"description": "交易对",
						},
						"new_stop_loss_price": map[string]interface{}{
							"type":        "number",
							"description": "【可选】新的止损价格。如果不提供则保持原止损不变。常见场景：持仓盈利后移动止损到盈亏平衡点或更高位置保护利润。",
						},
						"new_take_profit_price": map[string]interface{}{
							"type":        "number",
							"description": "【可选】新的止盈价格。如果不提供则保持原止盈不变（或取消止盈单让AI灵活管理）。设为0表示取消止盈单。",
						},
						"reason": map[string]interface{}{
							"type":        "string",
							"description": "更新理由。说明为什么要调整止损止盈（如：持仓盈利5%，移动止损至盈亏平衡点；市场环境变化，调高止盈目标等）。",
						},
					},
					"required": []string{"symbol", "reason"},
				},
			},
		},
	}
}

// executeToolFunction 执行工具函数
func (s *AgentService) executeToolFunction(ctx context.Context, functionName string, args map[string]interface{}) (map[string]interface{}, error) {
	switch functionName {
	case "openPosition":
		return s.toolOpenPosition(ctx, args)
	case "closePosition":
		return s.toolClosePosition(ctx, args)
	case "updateStopOrders":
		return s.toolUpdateStopOrders(ctx, args)
	default:
		return nil, fmt.Errorf("unknown function: %s", functionName)
	}
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
	exitPlanRaw, _ := args["exit_plan"].(string)
	exitPlan := strings.TrimSpace(exitPlanRaw)

	// 新增：止损止盈价格
	stopLossPrice, _ := args["stop_loss_price"].(float64)
	takeProfitPrice, _ := args["take_profit_price"].(float64)

	s.logger.Info("opening position",
		zap.String("symbol", symbol),
		zap.String("side", side),
		zap.Int("leverage", leverage),
		zap.Float64("margin_usdt", quantity),
		zap.Float64("stop_loss_price", stopLossPrice),
		zap.Float64("take_profit_price", takeProfitPrice),
		zap.String("reason", reason),
		zap.String("exit_plan", exitPlan))

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
	if strings.TrimSpace(reason) == "" {
		return nil, fmt.Errorf("开仓理由 reason 不能为空")
	}
	if exitPlan == "" {
		return nil, fmt.Errorf("退出计划 exit_plan 不能为空，请明确止损与退出逻辑")
	}

	// 验证止损价格（必填）
	if stopLossPrice <= 0 {
		return nil, fmt.Errorf("止损价格 stop_loss_price 必须设置且大于0")
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
	price, err := s.exchange.GetCurrentPrice(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get current price: %w", err)
	}

	// 验证止损止盈价格的合理性
	if err := s.validateStopPrices(price, side, stopLossPrice, takeProfitPrice); err != nil {
		return nil, err
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
		order, err = s.exchange.OpenLongPosition(ctx, symbol, actualQuantity)
	} else {
		order, err = s.exchange.OpenShortPosition(ctx, symbol, actualQuantity)
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

	// 计算手续费 (0.10% = 0.001)
	feeRate := 0.001
	notionalTraded := avgPrice * executedQty
	fee := notionalTraded * feeRate

	// 记录交易
	trade := &models.Trade{
		ID:         ulid.Make().String(),
		Symbol:     symbol,
		Type:       "open",
		Side:       side,
		Price:      avgPrice,
		Quantity:   executedQty,
		Leverage:   leverage,
		Fee:        fee,
		Reason:     reason,
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

	if err := s.positionService.UpdatePositionPlan(ctx, symbol, side, reason, exitPlan); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Warn("unable to record position plan, position not found after sync",
				zap.String("symbol", symbol),
				zap.String("side", side))
		} else {
			s.logger.Error("failed to update position plan",
				zap.String("symbol", symbol),
				zap.String("side", side),
				zap.Error(err))
		}
	}

	// ⭐ 创建止损单（硬止损）
	stopLossOrderID := int64(0)
	if err := s.createStopLossOrder(ctx, symbol, side, executedQty, stopLossPrice); err != nil {
		s.logger.Error("failed to create stop loss order",
			zap.String("symbol", symbol),
			zap.Float64("stop_loss_price", stopLossPrice),
			zap.Error(err))
		// 不阻止开仓，但记录警告
	} else {
		s.logger.Info("stop loss order created",
			zap.String("symbol", symbol),
			zap.Float64("stop_loss_price", stopLossPrice))
	}

	// ⭐ 创建止盈单（可选）
	takeProfitOrderID := int64(0)
	if takeProfitPrice > 0 {
		if err := s.createTakeProfitOrder(ctx, symbol, side, executedQty, takeProfitPrice); err != nil {
			s.logger.Error("failed to create take profit order",
				zap.String("symbol", symbol),
				zap.Float64("take_profit_price", takeProfitPrice),
				zap.Error(err))
		} else {
			s.logger.Info("take profit order created",
				zap.String("symbol", symbol),
				zap.Float64("take_profit_price", takeProfitPrice))
		}
	}

	// 保存止损止盈到持仓记录
	if err := s.positionService.UpdateStopPrices(ctx, symbol, side, stopLossPrice, takeProfitPrice); err != nil {
		s.logger.Error("failed to update stop prices in position",
			zap.String("symbol", symbol),
			zap.Error(err))
	}

	message := fmt.Sprintf("成功开仓 %s %s，杠杆 %dx，保证金 %.2fU，价格 %.2f，止损 %.2f",
		side, symbol, leverage, quantity, avgPrice, stopLossPrice)
	if takeProfitPrice > 0 {
		message += fmt.Sprintf("，止盈 %.2f", takeProfitPrice)
	}

	return map[string]interface{}{
		"success":              true,
		"order_id":             order.OrderID,
		"symbol":               symbol,
		"side":                 side,
		"price":                avgPrice,
		"quantity":             executedQty,
		"leverage":             leverage,
		"stop_loss_price":      stopLossPrice,
		"take_profit_price":    takeProfitPrice,
		"stop_loss_order_id":   stopLossOrderID,
		"take_profit_order_id": takeProfitOrderID,
		"message":              message,
	}, nil
}

// toolClosePosition 平仓
func (s *AgentService) toolClosePosition(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	symbol, _ := args["symbol"].(string)
	symbol = strings.TrimSpace(symbol)
	reason, _ := args["reason"].(string)
	reason = strings.TrimSpace(reason)

	s.logger.Info("attempting to close position",
		zap.String("symbol", symbol),
		zap.String("reason", reason))

	if symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}

	// 基础理由验证
	if err := s.validateCloseReason(reason); err != nil {
		return nil, err
	}

	positions, err := s.positionService.GetAllPositions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var targetPosition *models.Position
	for i := range positions {
		if positions[i].Symbol == symbol {
			targetPosition = &positions[i]
			break
		}
	}

	if targetPosition == nil {
		return nil, fmt.Errorf("no position found for symbol %s", symbol)
	}

	// 验证平仓理由是否符合退出计划
	if err := s.validateExitPlanCompliance(targetPosition, reason); err != nil {
		// 记录警告但不阻止平仓（软约束）
		s.logger.Warn("exit plan compliance check failed",
			zap.String("symbol", symbol),
			zap.String("exit_plan", targetPosition.ExitPlan),
			zap.String("reason", reason),
			zap.Error(err))
	}

	currentPrice, err := s.exchange.GetCurrentPrice(ctx, symbol)
	if err != nil {
		s.logger.Warn("failed to get current price for close position", zap.Error(err))
		currentPrice = targetPosition.CurrentPrice
	}

	s.logger.Info("executing close position",
		zap.String("symbol", symbol),
		zap.String("side", targetPosition.Side),
		zap.Float64("quantity", targetPosition.Quantity))

	var order *exchange.OrderResult
	if targetPosition.Side == "long" {
		order, err = s.exchange.CloseLongPosition(ctx, symbol, targetPosition.Quantity)
	} else {
		order, err = s.exchange.CloseShortPosition(ctx, symbol, targetPosition.Quantity)
	}

	if err != nil {
		s.logger.Error("failed to execute close position",
			zap.String("symbol", symbol),
			zap.Error(err))
		return nil, fmt.Errorf("failed to close position: %w", err)
	}

	avgPrice := order.AvgPrice
	if avgPrice == 0 {
		avgPrice = currentPrice
	}

	executedQty := order.ExecutedQty
	if executedQty == 0 {
		executedQty = targetPosition.Quantity
	}

	pnl := targetPosition.UnrealizedPnl

	// 计算手续费 (0.10% = 0.001)
	feeRate := 0.001
	notionalTraded := avgPrice * executedQty
	fee := notionalTraded * feeRate

	trade := &models.Trade{
		ID:         ulid.Make().String(),
		Symbol:     symbol,
		Type:       "close",
		Side:       targetPosition.Side,
		Price:      avgPrice,
		Quantity:   executedQty,
		Leverage:   targetPosition.Leverage,
		Fee:        fee,
		Pnl:        pnl,
		Reason:     reason,
		OrderID:    fmt.Sprintf("%d", order.OrderID),
		PositionID: targetPosition.ID,
		ExecutedAt: time.Now(),
	}

	if err := s.TradeRepo.Create(ctx, trade); err != nil {
		s.logger.Error("failed to save trade", zap.Error(err))
	}

	// 取消该持仓的所有止损止盈订单
	if err := s.cancelPositionStopOrders(ctx, targetPosition.ID, symbol); err != nil {
		s.logger.Error("failed to cancel position stop orders",
			zap.String("position_id", targetPosition.ID),
			zap.Error(err))
		// 不阻止继续执行
	}

	if err := s.positionService.DeletePosition(ctx, targetPosition.ID); err != nil {
		s.logger.Error("failed to delete position", zap.Error(err))
	}

	if err := s.positionService.SyncPositions(ctx); err != nil {
		s.logger.Warn("failed to sync positions after closing position", zap.Error(err))
	}

	message := fmt.Sprintf("成功平仓 %s，盈亏 $%.2f", symbol, pnl)
	if reason != "" {
		message += fmt.Sprintf("（理由：%s）", reason)
	}

	s.logger.Info("close position successful",
		zap.String("symbol", symbol),
		zap.Float64("pnl", pnl),
		zap.String("reason", reason))

	return map[string]interface{}{
		"success":  true,
		"order_id": order.OrderID,
		"symbol":   symbol,
		"pnl":      pnl,
		"reason":   reason,
		"message":  message,
	}, nil
}

func (s *AgentService) leverageBounds() (int, int) {
	tradingConfig, err := s.adminConfigService.GetTradingConfig(context.Background())
	if err != nil {
		s.logger.Error("failed to get trading config", zap.Error(err))
	}
	minLeverage := tradingConfig.MinLeverage
	maxLeverage := tradingConfig.MaxLeverage

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

	if err := s.exchange.SetMarginType(ctx, symbol, exchange.MarginTypeCrossed); err != nil {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "code=-4046") && !strings.Contains(errMsg, "No need to change margin type") {
			return fmt.Errorf("failed to set margin type: %w", err)
		}
	}

	if err := s.exchange.SetLeverage(ctx, symbol, leverage); err != nil {
		return fmt.Errorf("failed to set leverage: %w", err)
	}

	return nil
}

// validateStopPrices 验证止损止盈价格的合理性
func (s *AgentService) validateStopPrices(currentPrice float64, side string, stopLossPrice, takeProfitPrice float64) error {
	if side == "long" {
		// 做多：止损必须低于当前价，止盈必须高于当前价
		if stopLossPrice >= currentPrice {
			return fmt.Errorf("做多时止损价%.2f必须低于当前价%.2f", stopLossPrice, currentPrice)
		}
		if takeProfitPrice > 0 && takeProfitPrice <= currentPrice {
			return fmt.Errorf("做多时止盈价%.2f必须高于当前价%.2f", takeProfitPrice, currentPrice)
		}
	} else {
		// 做空：止损必须高于当前价，止盈必须低于当前价
		if stopLossPrice <= currentPrice {
			return fmt.Errorf("做空时止损价%.2f必须高于当前价%.2f", stopLossPrice, currentPrice)
		}
		if takeProfitPrice > 0 && takeProfitPrice >= currentPrice {
			return fmt.Errorf("做空时止盈价%.2f必须低于当前价%.2f", takeProfitPrice, currentPrice)
		}
	}
	return nil
}

// createStopLossOrder 创建止损单
func (s *AgentService) createStopLossOrder(ctx context.Context, symbol, side string, quantity, stopPrice float64) error {
	return s.createStopLossOrderWithReason(ctx, symbol, side, quantity, stopPrice, "开仓时设置止损")
}

// createStopLossOrderWithReason 创建止损单（带原因说明）
func (s *AgentService) createStopLossOrderWithReason(ctx context.Context, symbol, side string, quantity, stopPrice float64, reason string) error {
	// 做多止损 = 卖出；做空止损 = 买入
	stopSide := exchange.OrderSideSell
	if side == "short" {
		stopSide = exchange.OrderSideBuy
	}

	// 在交易所创建订单
	orderResult, err := s.exchange.CreateStopLossOrder(ctx, symbol, stopSide, quantity, stopPrice)
	if err != nil {
		return err
	}

	// 获取持仓ID
	position, err := s.positionService.PositionRepo.FindActiveBySymbolAndSide(ctx, symbol, side)
	if err != nil {
		s.logger.Warn("failed to get position for order recording",
			zap.String("symbol", symbol),
			zap.String("side", side),
			zap.Error(err))
		// 不阻止订单创建，只是无法记录到数据库
		return nil
	}

	// 记录到数据库
	order := &models.Order{
		ID:           ulid.Make().String(),
		Symbol:       symbol,
		PositionID:   position.ID,
		PositionSide: side,
		OrderType:    models.OrderTypeStopLoss,
		TriggerPrice: stopPrice,
		Quantity:     quantity,
		ExchangeID:   fmt.Sprintf("%d", orderResult.OrderID),
		Status:       models.OrderStatusActive,
		Reason:       reason,
	}

	if err := s.OrderRepo.Create(ctx, order); err != nil {
		s.logger.Error("failed to save stop loss order to database",
			zap.String("symbol", symbol),
			zap.Error(err))
		// 不阻止订单创建
	}

	return nil
}

// createTakeProfitOrder 创建止盈单
func (s *AgentService) createTakeProfitOrder(ctx context.Context, symbol, side string, quantity, takeProfitPrice float64) error {
	return s.createTakeProfitOrderWithReason(ctx, symbol, side, quantity, takeProfitPrice, "开仓时设置止盈")
}

// createTakeProfitOrderWithReason 创建止盈单（带原因说明）
func (s *AgentService) createTakeProfitOrderWithReason(ctx context.Context, symbol, side string, quantity, takeProfitPrice float64, reason string) error {
	// 做多止盈 = 卖出；做空止盈 = 买入
	takeProfitSide := exchange.OrderSideSell
	if side == "short" {
		takeProfitSide = exchange.OrderSideBuy
	}

	// 在交易所创建订单
	orderResult, err := s.exchange.CreateTakeProfitOrder(ctx, symbol, takeProfitSide, quantity, takeProfitPrice)
	if err != nil {
		return err
	}

	// 获取持仓ID
	position, err := s.positionService.PositionRepo.FindActiveBySymbolAndSide(ctx, symbol, side)
	if err != nil {
		s.logger.Warn("failed to get position for order recording",
			zap.String("symbol", symbol),
			zap.String("side", side),
			zap.Error(err))
		// 不阻止订单创建，只是无法记录到数据库
		return nil
	}

	// 记录到数据库
	order := &models.Order{
		ID:           ulid.Make().String(),
		Symbol:       symbol,
		PositionID:   position.ID,
		PositionSide: side,
		OrderType:    models.OrderTypeTakeProfit,
		TriggerPrice: takeProfitPrice,
		Quantity:     quantity,
		ExchangeID:   fmt.Sprintf("%d", orderResult.OrderID),
		Status:       models.OrderStatusActive,
		Reason:       reason,
	}

	if err := s.OrderRepo.Create(ctx, order); err != nil {
		s.logger.Error("failed to save take profit order to database",
			zap.String("symbol", symbol),
			zap.Error(err))
		// 不阻止订单创建
	}

	return nil
}

// toolUpdateStopOrders 更新止损止盈单
func (s *AgentService) toolUpdateStopOrders(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	symbol, _ := args["symbol"].(string)
	symbol = strings.TrimSpace(symbol)
	reason, _ := args["reason"].(string)
	reason = strings.TrimSpace(reason)
	newStopLossPrice, hasStopLoss := args["new_stop_loss_price"].(float64)
	newTakeProfitPrice, hasTakeProfit := args["new_take_profit_price"].(float64)

	s.logger.Info("attempting to update stop orders",
		zap.String("symbol", symbol),
		zap.Float64("new_stop_loss", newStopLossPrice),
		zap.Float64("new_take_profit", newTakeProfitPrice),
		zap.String("reason", reason))

	if symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	if reason == "" {
		return nil, fmt.Errorf("reason is required")
	}
	if !hasStopLoss && !hasTakeProfit {
		return nil, fmt.Errorf("must provide at least one of new_stop_loss_price or new_take_profit_price")
	}

	// 获取当前持仓
	positions, err := s.positionService.GetAllPositions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var targetPosition *models.Position
	for i := range positions {
		if positions[i].Symbol == symbol {
			targetPosition = &positions[i]
			break
		}
	}

	if targetPosition == nil {
		return nil, fmt.Errorf("no position found for symbol %s", symbol)
	}

	// 获取当前价格
	currentPrice, err := s.exchange.GetCurrentPrice(ctx, symbol)
	if err != nil {
		s.logger.Warn("failed to get current price", zap.Error(err))
		currentPrice = targetPosition.CurrentPrice
	}

	// 如果提供了新止损价格，验证合理性
	if hasStopLoss && newStopLossPrice > 0 {
		if err := s.validateStopPrices(currentPrice, targetPosition.Side, newStopLossPrice, 0); err != nil {
			return nil, fmt.Errorf("invalid new stop loss price: %w", err)
		}
	}

	// 如果提供了新止盈价格且不为0，验证合理性
	if hasTakeProfit && newTakeProfitPrice > 0 {
		if err := s.validateStopPrices(currentPrice, targetPosition.Side, 0, newTakeProfitPrice); err != nil {
			return nil, fmt.Errorf("invalid new take profit price: %w", err)
		}
	}

	// 获取该持仓的所有活跃订单
	activeOrders, err := s.OrderRepo.FindByPositionID(ctx, targetPosition.ID)
	if err != nil {
		s.logger.Warn("failed to get active orders for position",
			zap.String("position_id", targetPosition.ID),
			zap.Error(err))
		activeOrders = []models.Order{}
	}

	// 精准取消：只取消需要更新的订单类型
	for i := range activeOrders {
		order := &activeOrders[i]
		if !order.IsActive() {
			continue
		}

		shouldCancel := false
		if hasStopLoss && order.IsStopLoss() {
			shouldCancel = true
		}
		if hasTakeProfit && order.IsTakeProfit() {
			shouldCancel = true
		}

		if shouldCancel && order.ExchangeID != "" {
			// 从交易所取消订单
			var exchangeOrderID = cast.ToInt64(order.ExchangeID)
			if exchangeOrderID > 0 {
				if err := s.exchange.CancelOrder(ctx, symbol, exchangeOrderID); err != nil {
					s.logger.Warn("failed to cancel order on exchange",
						zap.String("symbol", symbol),
						zap.String("order_id", order.ExchangeID),
						zap.String("order_type", string(order.OrderType)),
						zap.Error(err))
				} else {
					s.logger.Info("cancelled order on exchange",
						zap.String("symbol", symbol),
						zap.String("order_id", order.ExchangeID),
						zap.String("order_type", string(order.OrderType)))
				}
			}

			// 更新数据库订单状态为已取消
			if err := s.OrderRepo.UpdateStatus(ctx, order.ID, models.OrderStatusCanceled); err != nil {
				s.logger.Error("failed to update order status in database",
					zap.String("order_id", order.ID),
					zap.Error(err))
			}
		}
	}

	// 创建新的止损单
	if hasStopLoss && newStopLossPrice > 0 {
		if err := s.createStopLossOrderWithReason(ctx, symbol, targetPosition.Side, targetPosition.Quantity, newStopLossPrice, reason); err != nil {
			s.logger.Error("failed to create new stop loss order",
				zap.String("symbol", symbol),
				zap.Float64("new_stop_loss_price", newStopLossPrice),
				zap.Error(err))
			// 不阻止继续执行，记录错误
		} else {
			s.logger.Info("new stop loss order created",
				zap.String("symbol", symbol),
				zap.Float64("old_stop_loss", targetPosition.StopLoss),
				zap.Float64("new_stop_loss", newStopLossPrice))
		}
	} else if hasStopLoss {
		// 如果显式传入0，表示不更新止损
		newStopLossPrice = targetPosition.StopLoss
	} else {
		// 如果没有传入，保持原止损
		newStopLossPrice = targetPosition.StopLoss
	}

	// 创建新的止盈单（0表示取消）
	if hasTakeProfit && newTakeProfitPrice > 0 {
		if err := s.createTakeProfitOrderWithReason(ctx, symbol, targetPosition.Side, targetPosition.Quantity, newTakeProfitPrice, reason); err != nil {
			s.logger.Error("failed to create new take profit order",
				zap.String("symbol", symbol),
				zap.Float64("new_take_profit_price", newTakeProfitPrice),
				zap.Error(err))
		} else {
			s.logger.Info("new take profit order created",
				zap.String("symbol", symbol),
				zap.Float64("old_take_profit", targetPosition.TakeProfit),
				zap.Float64("new_take_profit", newTakeProfitPrice))
		}
	} else if hasTakeProfit && newTakeProfitPrice == 0 {
		// 设为0表示取消止盈单（已在CancelAllOrders中取消）
		s.logger.Info("take profit order cancelled",
			zap.String("symbol", symbol),
			zap.Float64("old_take_profit", targetPosition.TakeProfit))
		newTakeProfitPrice = 0
	} else {
		// 如果没有传入，保持原止盈
		newTakeProfitPrice = targetPosition.TakeProfit
	}

	// 更新数据库中的止损止盈价格
	if err := s.positionService.UpdateStopPrices(ctx, symbol, targetPosition.Side, newStopLossPrice, newTakeProfitPrice); err != nil {
		s.logger.Error("failed to update stop prices in database",
			zap.String("symbol", symbol),
			zap.Error(err))
	}

	message := fmt.Sprintf("成功更新 %s 的止损止盈单", symbol)
	if hasStopLoss && newStopLossPrice > 0 {
		message += fmt.Sprintf("，止损: %.2f → %.2f", targetPosition.StopLoss, newStopLossPrice)
	}
	if hasTakeProfit {
		if newTakeProfitPrice > 0 {
			message += fmt.Sprintf("，止盈: %.2f → %.2f", targetPosition.TakeProfit, newTakeProfitPrice)
		} else {
			message += "，已取消止盈单"
		}
	}
	message += fmt.Sprintf("（理由：%s）", reason)

	return map[string]interface{}{
		"success":         true,
		"symbol":          symbol,
		"old_stop_loss":   targetPosition.StopLoss,
		"new_stop_loss":   newStopLossPrice,
		"old_take_profit": targetPosition.TakeProfit,
		"new_take_profit": newTakeProfitPrice,
		"reason":          reason,
		"message":         message,
	}, nil
}

// SaveDecision 保存AI决策记录，返回决策ID
func (s *AgentService) SaveDecision(ctx context.Context, iteration int, accountValue float64, positionCount int,
	decisionContent string, promptTokens int, completionTokens int) (string, error) {

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

	if err := s.DecisionRepo.Create(ctx, decision); err != nil {
		return "", err
	}

	return decision.ID, nil
}

// UpdateDecision 更新决策记录
func (s *AgentService) UpdateDecision(ctx context.Context, decisionID string, decisionContent string, promptTokens int, completionTokens int) error {
	decision, err := s.DecisionRepo.FindById(ctx, decisionID)
	if err != nil {
		return err
	}

	decision.DecisionContent = decisionContent
	decision.PromptTokens = promptTokens
	decision.CompletionTokens = completionTokens

	return s.DecisionRepo.Save(ctx, &decision)
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
func (s *AgentService) GetRecentTrades(ctx context.Context, limit int) ([]models.Trade, error) {
	trades, err := s.TradeRepo.FindRecentTrades(ctx, limit)
	if err != nil {
		return nil, err
	}

	return trades, nil
}

// GetTradeStats 获取交易统计数据
func (s *AgentService) GetTradeStats(ctx context.Context) (*repo.TradeStats, error) {
	return s.TradeRepo.GetTradeStats(ctx)
}

// GetLLMLogsByDecisionID 根据决策ID获取LLM日志
func (s *AgentService) GetLLMLogsByDecisionID(ctx context.Context, decisionID string) ([]models.LLMLog, error) {
	logs, err := s.LLMLogRepo.FindByDecisionID(ctx, decisionID)
	if err != nil {
		return nil, err
	}
	return logs, nil
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

// validateCloseReason 验证平仓理由的基础格式
func (s *AgentService) validateCloseReason(reason string) error {
	if reason == "" {
		return fmt.Errorf("平仓理由不能为空")
	}

	if len(reason) < 20 {
		return fmt.Errorf("平仓理由过于简单（当前 %d 字符），请详细说明触发的退出条件（至少 20 字符）", len(reason))
	}

	return nil
}

// validateExitPlanCompliance 验证平仓理由是否符合退出计划
func (s *AgentService) validateExitPlanCompliance(position *models.Position, reason string) error {
	// 如果没有退出计划，不进行验证
	if position.ExitPlan == "" {
		s.logger.Debug("position has no exit plan, skipping compliance check",
			zap.String("symbol", position.Symbol))
		return nil
	}

	// 定义退出计划相关的关键词
	exitKeywords := []string{
		// 中文关键词
		"止损", "止盈", "目标价", "支撑", "阻力", "破位", "突破",
		"趋势", "结构", "均线", "指标", "信号", "回调", "反弹",
		"压力", "跌破", "涨破", "触发", "达到", "未达到",
		// 英文关键词（支持混合使用）
		"stop", "loss", "profit", "target", "support", "resistance",
		"break", "trend", "structure", "signal",
	}

	// 检查理由是否包含至少一个关键词
	if !containsAnyKeyword(reason, exitKeywords) {
		return fmt.Errorf("平仓理由未体现退出计划的关键条件（如止损、止盈、支撑/阻力、结构破坏等）")
	}

	// 记录成功验证
	s.logger.Info("exit plan compliance check passed",
		zap.String("symbol", position.Symbol),
		zap.String("exit_plan", position.ExitPlan),
		zap.String("reason", reason))

	return nil
}

// containsAnyKeyword 检查文本是否包含任意一个关键词（不区分大小写）
func containsAnyKeyword(text string, keywords []string) bool {
	lowerText := strings.ToLower(text)
	for _, keyword := range keywords {
		if strings.Contains(lowerText, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// cancelPositionStopOrders 取消指定持仓的所有止损止盈订单
func (s *AgentService) cancelPositionStopOrders(ctx context.Context, positionID, symbol string) error {
	// 获取该持仓的所有活跃订单
	activeOrders, err := s.OrderRepo.FindByPositionID(ctx, positionID)
	if err != nil {
		return fmt.Errorf("failed to get orders for position: %w", err)
	}

	if len(activeOrders) == 0 {
		return nil
	}

	s.logger.Info("cancelling stop orders for closed position",
		zap.String("position_id", positionID),
		zap.String("symbol", symbol),
		zap.Int("order_count", len(activeOrders)))

	// 逐个取消订单
	for i := range activeOrders {
		order := &activeOrders[i]

		// 只处理活跃订单
		if !order.IsActive() {
			continue
		}

		// 解析交易所订单ID
		var exchangeOrderID = cast.ToInt64(order.ExchangeID)
		if exchangeOrderID > 0 {
			// 从交易所取消订单
			if err := s.exchange.CancelOrder(ctx, symbol, exchangeOrderID); err != nil {
				s.logger.Warn("failed to cancel order on exchange",
					zap.String("symbol", symbol),
					zap.String("order_id", order.ExchangeID),
					zap.String("order_type", string(order.OrderType)),
					zap.Error(err))
			} else {
				s.logger.Info("cancelled order on exchange",
					zap.String("symbol", symbol),
					zap.String("order_type", string(order.OrderType)),
					zap.String("order_id", order.ExchangeID))
			}
		}

		// 更新数据库订单状态为已取消
		if err := s.OrderRepo.UpdateStatus(ctx, order.ID, models.OrderStatusCanceled); err != nil {
			s.logger.Error("failed to update order status to canceled",
				zap.String("order_id", order.ID),
				zap.Error(err))
		}
	}

	return nil
}

// saveLLMLog 保存LLM通信日志
func (s *AgentService) saveLLMLog(
	ctx context.Context,
	decisionID string,
	iteration int,
	roundNumber int,
	systemPrompt string,
	userPrompt string,
	messages []openai.ChatCompletionMessageParamUnion,
	assistantContent string,
	toolCalls []map[string]interface{},
	toolResponses []map[string]interface{},
	promptTokens int,
	completionTokens int,
	finishReason string,
	duration int64,
	errorMsg string,
) {
	// 将消息历史序列化为JSON
	messagesJSON, err := json.Marshal(messages)
	if err != nil {
		s.logger.Error("failed to marshal messages for LLM log", zap.Error(err))
		messagesJSON = []byte("[]")
	}

	// 将工具调用序列化为JSON
	toolCallsJSON := "[]"
	if len(toolCalls) > 0 {
		if data, err := json.Marshal(toolCalls); err == nil {
			toolCallsJSON = string(data)
		}
	}

	// 将工具响应序列化为JSON
	toolResponsesJSON := "[]"
	if len(toolResponses) > 0 {
		if data, err := json.Marshal(toolResponses); err == nil {
			toolResponsesJSON = string(data)
		}
	}

	// 创建日志记录
	llmLog := &models.LLMLog{
		ID:               ulid.Make().String(),
		DecisionID:       decisionID,
		Iteration:        iteration,
		RoundNumber:      roundNumber,
		Model:            s.model,
		SystemPrompt:     systemPrompt,
		UserPrompt:       userPrompt,
		Messages:         string(messagesJSON),
		AssistantContent: assistantContent,
		ToolCalls:        toolCallsJSON,
		ToolResponses:    toolResponsesJSON,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
		FinishReason:     finishReason,
		Duration:         duration,
		Error:            errorMsg,
		ExecutedAt:       time.Now(),
	}

	// 保存到数据库
	if err := s.LLMLogRepo.Create(ctx, llmLog); err != nil {
		s.logger.Error("failed to save LLM log", zap.Error(err))
	} else {
		s.logger.Debug("LLM log saved",
			zap.String("log_id", llmLog.ID),
			zap.Int("iteration", iteration),
			zap.Int("round", roundNumber),
			zap.Int("prompt_tokens", promptTokens),
			zap.Int("completion_tokens", completionTokens))
	}
}
