package service

import (
	"context"
	"encoding/json"
	"errors"
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

// DecisionRound 决策轮次记录
type DecisionRound struct {
	Reasoning string   // AI的思考过程
	ToolCalls []string // 本轮调用的工具
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
	var finalText string
	var rounds []DecisionRound
	totalPromptTokens := 0
	totalCompletionTokens := 0

	maxIterations := 10 // 防止无限循环
	for iteration := 0; iteration < maxIterations; iteration++ {
		// 调用 OpenAI API
		resp, err := s.openAIClient.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:    s.model,
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
				finalText = strings.TrimSpace(message.Content)
			}
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
		}

		// 保存本轮记录
		rounds = append(rounds, currentRound)

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
	for i, round := range rounds {
		var roundContent strings.Builder

		// 先写轮次标题
		roundContent.WriteString(fmt.Sprintf("【第%d轮】\n", i+1))

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
	} else if len(rounds) > 0 {
		// 如果没有最终总结但有操作，生成简要说明
		sections = append(sections, "【决策总结】\n已完成上述操作")
	}

	if len(sections) == 0 {
		return "无操作"
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
						"exit_plan": map[string]interface{}{
							"type":        "string",
							"description": "详细的退出计划，必须明确包含以下至少一种条件：1)止损条件（价格/百分比/指标）；2)止盈条件（目标价/阻力位）；3)结构破坏条件；4)时间条件。平仓时的 reason 必须明确对应这些条件之一。",
						},
					},
					"required": []string{"symbol", "side", "leverage", "quantity", "reason", "exit_plan"},
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
	}
}

// executeToolFunction 执行工具函数
func (s *AgentService) executeToolFunction(ctx context.Context, functionName string, args map[string]interface{}) (map[string]interface{}, error) {
	switch functionName {
	case "openPosition":
		return s.toolOpenPosition(ctx, args)
	case "closePosition":
		return s.toolClosePosition(ctx, args)
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

	s.logger.Info("opening position",
		zap.String("symbol", symbol),
		zap.String("side", side),
		zap.Int("leverage", leverage),
		zap.Float64("margin_usdt", quantity),
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

	return map[string]interface{}{
		"success":  true,
		"order_id": order.OrderID,
		"symbol":   symbol,
		"side":     side,
		"price":    avgPrice,
		"quantity": executedQty,
		"leverage": leverage,
		"message":  fmt.Sprintf("成功开仓 %s %s，杠杆 %dx，保证金 %.2fU 价值 %.2f", side, symbol, leverage, quantity, avgPrice),
	}, nil
}

// ClosePositionValidation 平仓验证结果
type ClosePositionValidation struct {
	Position       *models.Position
	ExitPlan       string
	ExitConditions []ExitCondition
	TriggeredType  ExitConditionType
	HoldingHours   float64
	Reason         string
}

// validateClosePosition 统一的平仓验证入口
func (s *AgentService) validateClosePosition(ctx context.Context, symbol, reason string) (*ClosePositionValidation, error) {
	// 1. 基础参数验证
	if symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	if reason == "" {
		return nil, fmt.Errorf("平仓理由 reason 不能为空，请说明触发的具体条件")
	}

	// 2. 查找持仓
	positions, err := s.positionService.GetAllPositions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
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

	// 3. 构建验证上下文
	validation := &ClosePositionValidation{
		Position:      targetPosition,
		ExitPlan:      strings.TrimSpace(targetPosition.ExitPlan),
		HoldingHours:  targetPosition.CalculateHoldingHours(),
		Reason:        reason,
		TriggeredType: s.identifyTriggeredConditionType(reason),
	}

	// 4. 提取退出计划中的条件
	if validation.ExitPlan != "" {
		validation.ExitConditions = s.extractExitConditions(validation.ExitPlan)
	}

	// 5. 验证平仓理由与退出计划的匹配性
	if err := s.validateExitPlanMatch(validation); err != nil {
		return nil, err
	}

	// 6. 验证持仓时间限制
	if err := s.validateHoldingTime(validation); err != nil {
		return nil, err
	}

	return validation, nil
}

// validateExitPlanMatch 验证平仓理由是否匹配退出计划
func (s *AgentService) validateExitPlanMatch(v *ClosePositionValidation) error {
	if v.ExitPlan == "" {
		// 没有退出计划，跳过此检查
		return nil
	}

	if s.reasonMatchesExitPlan(v.Reason, v.ExitPlan) {
		// 匹配成功
		return nil
	}

	// 匹配失败，构建详细错误信息
	var conditionTypes []string
	for _, cond := range v.ExitConditions {
		conditionTypes = append(conditionTypes, string(cond.Type))
	}

	errorMsg := fmt.Sprintf("【平仓被拒绝】平仓理由与退出计划不匹配\n\n")
	errorMsg += fmt.Sprintf("退出计划：\n%s\n\n", v.ExitPlan)

	if len(v.ExitConditions) > 0 {
		errorMsg += fmt.Sprintf("计划中的退出条件：%s\n", strings.Join(conditionTypes, "、"))
		errorMsg += fmt.Sprintf("您的平仓理由：%s\n", v.Reason)
		if v.TriggeredType != "" {
			errorMsg += fmt.Sprintf("识别的条件类型：%s\n\n", v.TriggeredType)
		} else {
			errorMsg += "识别的条件类型：未识别\n\n"
		}
		errorMsg += "请在 reason 中明确说明触发了计划中的哪个具体条件。\n"
		errorMsg += "示例：\"触发止损，价格跌破 $95,000\" 或 \"达到目标价 $105,000\""
	} else {
		errorMsg += "请确保 reason 与退出计划的描述一致"
	}

	return fmt.Errorf(errorMsg)
}

// validateHoldingTime 验证持仓时间限制
func (s *AgentService) validateHoldingTime(v *ClosePositionValidation) error {
	if v.HoldingHours >= 1.0 {
		// 持仓时间足够，无需检查
		return nil
	}

	// 检查是否为紧急条件
	if s.isUrgentExitCondition(v.TriggeredType) {
		// 紧急条件，允许平仓
		return nil
	}

	// 不满足条件，构建错误信息
	allowedTypes := []string{string(ExitTypeStopLoss), string(ExitTypeTrailing), string(ExitTypeStructure)}

	errorMsg := fmt.Sprintf("【平仓被拒绝】持仓时间不足\n\n")
	errorMsg += fmt.Sprintf("当前持仓时长：%.1f 小时\n", v.HoldingHours)
	errorMsg += "最小持仓要求：1.0 小时\n\n"
	errorMsg += fmt.Sprintf("允许提前平仓的紧急条件：%s\n", strings.Join(allowedTypes, "、"))

	if v.TriggeredType != "" {
		errorMsg += fmt.Sprintf("您触发的条件类型：%s（非紧急条件）\n\n", v.TriggeredType)
	} else {
		errorMsg += "您触发的条件类型：未识别\n\n"
	}

	errorMsg += "请在 reason 中明确说明触发了紧急条件，或等待持仓满1小时后再平仓。\n"
	errorMsg += "示例：\"触发止损\" 或 \"市场结构破坏，跌破支撑位\""

	return fmt.Errorf(errorMsg)
}

// toolClosePosition 平仓
func (s *AgentService) toolClosePosition(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	// 1. 提取参数
	symbol, _ := args["symbol"].(string)
	symbol = strings.TrimSpace(symbol)
	reason, _ := args["reason"].(string)
	reason = strings.TrimSpace(reason)

	s.logger.Info("attempting to close position",
		zap.String("symbol", symbol),
		zap.String("reason", reason))

	// 2. 统一验证（包括所有业务规则检查）
	validation, err := s.validateClosePosition(ctx, symbol, reason)
	if err != nil {
		s.logger.Warn("close position validation failed",
			zap.String("symbol", symbol),
			zap.String("reason", reason),
			zap.Error(err))
		return nil, err
	}

	targetPosition := validation.Position

	// 3. 获取当前价格用于记录
	currentPrice, err := s.binanceClient.GetCurrentPrice(ctx, symbol)
	if err != nil {
		s.logger.Warn("failed to get current price for close position", zap.Error(err))
		currentPrice = targetPosition.CurrentPrice
	}

	// 4. 执行平仓操作
	s.logger.Info("executing close position",
		zap.String("symbol", symbol),
		zap.String("side", targetPosition.Side),
		zap.Float64("quantity", targetPosition.Quantity),
		zap.String("triggered_type", string(validation.TriggeredType)),
		zap.Float64("holding_hours", validation.HoldingHours))

	var order *exchange.OrderResult
	if targetPosition.Side == "long" {
		order, err = s.binanceClient.CloseLongPosition(ctx, symbol, targetPosition.Quantity)
	} else {
		order, err = s.binanceClient.CloseShortPosition(ctx, symbol, targetPosition.Quantity)
	}

	if err != nil {
		s.logger.Error("failed to execute close position",
			zap.String("symbol", symbol),
			zap.Error(err))
		return nil, fmt.Errorf("failed to close position: %w", err)
	}

	// 5. 处理订单结果
	avgPrice := order.AvgPrice
	if avgPrice == 0 {
		avgPrice = currentPrice
	}

	executedQty := order.ExecutedQty
	if executedQty == 0 {
		executedQty = targetPosition.Quantity
	}

	pnl := targetPosition.UnrealizedPnl

	// 6. 记录交易到数据库
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

	// 7. 删除持仓记录
	if err := s.positionService.DeletePosition(ctx, targetPosition.ID); err != nil {
		s.logger.Error("failed to delete position", zap.Error(err))
	}

	// 8. 同步持仓状态
	if err := s.positionService.SyncPositions(ctx); err != nil {
		s.logger.Warn("failed to sync positions after closing position", zap.Error(err))
	}

	// 9. 记录成功日志
	s.logger.Info("close position successful",
		zap.String("symbol", symbol),
		zap.Float64("pnl", pnl),
		zap.String("triggered_type", string(validation.TriggeredType)),
		zap.String("reason", reason))

	// 10. 构建返回消息
	successMessage := fmt.Sprintf("成功平仓 %s，盈亏 $%.2f", symbol, pnl)
	if validation.TriggeredType != "" {
		successMessage += fmt.Sprintf("（触发条件：%s）", validation.TriggeredType)
	}

	return map[string]interface{}{
		"success":        true,
		"order_id":       order.OrderID,
		"symbol":         symbol,
		"pnl":            pnl,
		"reason":         reason,
		"triggered_type": string(validation.TriggeredType),
		"holding_hours":  validation.HoldingHours,
		"message":        successMessage,
	}, nil
}

// identifyTriggeredConditionType 识别平仓理由触发的条件类型
func (s *AgentService) identifyTriggeredConditionType(reason string) ExitConditionType {
	if reason == "" {
		return ""
	}

	reasonLower := strings.ToLower(reason)

	// 按优先级检查各类条件（越紧急的越优先）
	conditionPatterns := []struct {
		condType ExitConditionType
		keywords []string
	}{
		{ExitTypeStopLoss, []string{"止损", "停损", "stop loss", "sl", "止损价", "止损位"}},
		{ExitTypeTrailing, []string{"追踪止损", "移动止损", "trailing stop", "trailing"}},
		{ExitTypeStructure, []string{"结构", "破坏", "失效", "突破", "跌破", "守不住", "站不稳", "break", "breakdown", "支撑", "失守"}},
		{ExitTypeTakeProfit, []string{"止盈", "获利", "目标", "盈利", "take profit", "tp", "目标价", "阻力", "压力位"}},
		{ExitTypeReversal, []string{"反转", "转向", "reversal", "反向", "转势", "趋势改变"}},
		{ExitTypeIndicator, []string{"指标", "背离", "divergence", "rsi", "macd", "ema", "sma", "均线", "死叉", "金叉"}},
		{ExitTypeTimeout, []string{"超时", "时间", "持有时间", "timeout"}},
	}

	for _, pattern := range conditionPatterns {
		for _, keyword := range pattern.keywords {
			if strings.Contains(reasonLower, strings.ToLower(keyword)) {
				return pattern.condType
			}
		}
	}

	// 未识别出具体类型
	return ""
}

// isUrgentExitCondition 判断条件类型是否属于紧急条件（允许1小时内平仓）
func (s *AgentService) isUrgentExitCondition(condType ExitConditionType) bool {
	switch condType {
	case ExitTypeStopLoss, ExitTypeTrailing, ExitTypeStructure:
		// 紧急条件：止损、追踪止损、结构破坏
		return true
	case ExitTypeTakeProfit, ExitTypeIndicator, ExitTypeReversal, ExitTypeTimeout:
		// 非紧急条件：止盈、指标、反转、超时
		return false
	default:
		// 未识别的类型，不允许
		return false
	}
}

// isCriticalExitReason 已废弃，保留用于兼容性
// 请使用 identifyTriggeredConditionType 和 isUrgentExitCondition 替代
func (s *AgentService) isCriticalExitReason(reasonLower string) bool {
	condType := s.identifyTriggeredConditionType(reasonLower)
	return s.isUrgentExitCondition(condType)
}

// ExitConditionType 退出条件类型
type ExitConditionType string

const (
	ExitTypeStopLoss   ExitConditionType = "止损"   // 止损条件
	ExitTypeTakeProfit ExitConditionType = "止盈"   // 止盈条件
	ExitTypeStructure  ExitConditionType = "结构"   // 市场结构破坏
	ExitTypeTimeout    ExitConditionType = "超时"   // 持仓超时
	ExitTypeIndicator  ExitConditionType = "指标"   // 技术指标信号
	ExitTypeTrailing   ExitConditionType = "追踪止损" // 追踪止损
	ExitTypeReversal   ExitConditionType = "反转"   // 趋势反转
)

// ExitCondition 退出条件
type ExitCondition struct {
	Type        ExitConditionType
	Keywords    []string // 关键词列表
	Description string   // 条件描述
}

// extractExitConditions 从退出计划中提取所有退出条件
func (s *AgentService) extractExitConditions(exitPlan string) []ExitCondition {
	if exitPlan == "" {
		return nil
	}

	exitPlanLower := strings.ToLower(exitPlan)
	var conditions []ExitCondition

	// 定义各类退出条件的关键词模式
	conditionPatterns := []struct {
		condType ExitConditionType
		keywords []string
	}{
		{
			ExitTypeStopLoss,
			[]string{"止损", "停损", "亏损", "损失", "stop loss", "sl", "止损价", "止损位"},
		},
		{
			ExitTypeTakeProfit,
			[]string{"止盈", "获利", "目标", "盈利", "take profit", "tp", "目标价", "阻力", "压力位"},
		},
		{
			ExitTypeTrailing,
			[]string{"追踪止损", "移动止损", "trailing stop", "trailing", "追踪"},
		},
		{
			ExitTypeStructure,
			[]string{"结构", "破坏", "失效", "突破", "跌破", "守不住", "站不稳", "break", "breakdown", "支撑", "失守"},
		},
		{
			ExitTypeReversal,
			[]string{"反转", "转向", "reversal", "反向", "转势", "趋势改变"},
		},
		{
			ExitTypeIndicator,
			[]string{"指标", "背离", "divergence", "rsi", "macd", "ema", "sma", "均线", "死叉", "金叉", "指标信号"},
		},
		{
			ExitTypeTimeout,
			[]string{"超时", "时间", "持有时间", "timeout", "time", "小时", "天", "周期"},
		},
	}

	// 提取每种类型的条件
	for _, pattern := range conditionPatterns {
		var matchedKeywords []string
		var segments []string

		for _, keyword := range pattern.keywords {
			if strings.Contains(exitPlanLower, strings.ToLower(keyword)) {
				matchedKeywords = append(matchedKeywords, keyword)
				// 提取包含该关键词的句子片段
				segments = append(segments, s.extractSentenceContaining(exitPlan, keyword)...)
			}
		}

		if len(matchedKeywords) > 0 {
			description := strings.Join(removeDuplicates(segments), "; ")
			if description == "" {
				description = string(pattern.condType) + "条件"
			}
			conditions = append(conditions, ExitCondition{
				Type:        pattern.condType,
				Keywords:    matchedKeywords,
				Description: description,
			})
		}
	}

	return conditions
}

// extractSentenceContaining 提取包含关键词的句子片段
func (s *AgentService) extractSentenceContaining(text string, keyword string) []string {
	lowerText := strings.ToLower(text)
	lowerKeyword := strings.ToLower(keyword)

	if !strings.Contains(lowerText, lowerKeyword) {
		return nil
	}

	// 按常见分隔符分割
	separators := []string{"\n", "。", "；", ";", "!", "！"}
	sentences := []string{text}

	for _, sep := range separators {
		var newSentences []string
		for _, sent := range sentences {
			parts := strings.Split(sent, sep)
			for _, part := range parts {
				trimmed := strings.TrimSpace(part)
				if trimmed != "" {
					newSentences = append(newSentences, trimmed)
				}
			}
		}
		sentences = newSentences
	}

	// 找出包含关键词的句子
	var result []string
	for _, sent := range sentences {
		if strings.Contains(strings.ToLower(sent), lowerKeyword) {
			// 限制长度，避免太长
			if len(sent) > 100 {
				sent = sent[:100] + "..."
			}
			result = append(result, sent)
		}
	}

	return result
}

// removeDuplicates 移除重复字符串
func removeDuplicates(items []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

// reasonMatchesExitPlan 检查平仓理由是否匹配退出计划
func (s *AgentService) reasonMatchesExitPlan(reason string, exitPlan string) bool {
	if exitPlan == "" {
		return true
	}

	// 提取退出计划中的所有条件
	conditions := s.extractExitConditions(exitPlan)
	if len(conditions) == 0 {
		// 如果无法识别任何条件，使用宽松匹配
		return true
	}

	reasonLower := strings.ToLower(reason)

	// 检查理由是否匹配任何一个退出条件
	for _, condition := range conditions {
		for _, keyword := range condition.Keywords {
			if strings.Contains(reasonLower, strings.ToLower(keyword)) {
				return true
			}
		}
	}

	return false
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

	if err := s.binanceClient.SetMarginType(ctx, symbol, futures.MarginTypeCrossed); err != nil {
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
