package service

import (
	"context"
	_ "embed"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/dushixiang/prism/internal/config"
	"github.com/dushixiang/prism/internal/models"
	"github.com/valyala/fasttemplate"
	"gorm.io/gorm"
)

// PromptService AI提示词生成服务
type PromptService struct {
	db     *gorm.DB
	config *config.Config
}

//go:embed templates/system_instructions.txt
var systemInstructionsTemplate string

// NewPromptService 创建提示词服务
func NewPromptService(db *gorm.DB, conf *config.Config) *PromptService {
	return &PromptService{
		db:     db,
		config: conf,
	}
}

// PromptData 提示词数据
type PromptData struct {
	StartTime       time.Time
	Iteration       int
	AccountMetrics  *AccountMetrics
	MarketDataMap   map[string]*MarketData
	Positions       []*models.Position
	RecentTrades    []*models.Trade
	RecentDecisions []*models.Decision
}

// GeneratePrompt 生成完整的AI提示词
func (s *PromptService) GeneratePrompt(ctx context.Context, data *PromptData) string {
	if data == nil {
		return ""
	}

	var sb strings.Builder

	s.writeConversationContext(&sb, data)

	s.writeMarketOverview(&sb, data.MarketDataMap)

	s.writeOpportunityRadar(&sb, data.MarketDataMap)

	s.writeAccountInfo(&sb, data.AccountMetrics)

	s.writePositionInfo(&sb, data.Positions)

	s.writeTradeHistory(&sb, data.RecentTrades)

	s.writeDecisionHistory(&sb, data.RecentDecisions)

	return sb.String()
}

// writeConversationContext 写入通话背景
func (s *PromptService) writeConversationContext(sb *strings.Builder, data *PromptData) {
	sb.WriteString("## 通话背景\n\n")

	now := time.Now().In(time.FixedZone("CST", 8*3600))
	currentTime := now.Format("2006-01-02 15:04:05")

	var minutesElapsed float64
	if !data.StartTime.IsZero() {
		minutesElapsed = time.Since(data.StartTime).Minutes()
		if minutesElapsed < 0 {
			minutesElapsed = 0
		}
	}

	sb.WriteString(fmt.Sprintf("- 开始交易以来已过去 %.0f 分钟\n", minutesElapsed))
	sb.WriteString(fmt.Sprintf("- 当前时间：%s（中国时区）\n", currentTime))
	sb.WriteString(fmt.Sprintf("- 本次模型调用序号：%d\n\n", data.Iteration))
}

// writeMarketOverview 写入市场数据
func (s *PromptService) writeMarketOverview(sb *strings.Builder, marketDataMap map[string]*MarketData) {
	sb.WriteString("## 市场全景\n\n")

	if len(marketDataMap) == 0 {
		sb.WriteString("暂无可用的市场数据。\n\n")
		return
	}

	symbols := make([]string, 0, len(marketDataMap))
	for symbol := range marketDataMap {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)

	for _, symbol := range symbols {
		data := marketDataMap[symbol]
		if data == nil {
			continue
		}

		sb.WriteString(fmt.Sprintf("### %s\n\n", symbol))

		sb.WriteString(fmt.Sprintf("- 最新价格：$%.2f\n", data.CurrentPrice))
		sb.WriteString(fmt.Sprintf("- 资金费率：%.6f%%\n\n", data.FundingRate*100))

		// 多时间框架指标
		sb.WriteString("### 多时间框架指标\n")
		timeframes := []string{"5m", "15m", "30m", "1h", "4h"}
		for _, tf := range timeframes {
			if ind, ok := data.Timeframes[tf]; ok {
				sb.WriteString(fmt.Sprintf("**%s**: 价格=$%.2f, EMA20=$%.2f, EMA50=$%.2f, MACD=%.2f, RSI7=%.1f, RSI14=%.1f, 成交量=%.0f\n",
					tf, ind.Price, ind.EMA20, ind.EMA50, ind.MACD, ind.RSI7, ind.RSI14, ind.Volume))
			}
		}
		sb.WriteString("\n")

		// 日内序列
		if data.IntradaySeries != nil {
			sb.WriteString("### 日内序列（5分钟级别，最近10个数据点）\n")
			sb.WriteString(fmt.Sprintf("中间价: %v\n", formatFloatArray(data.IntradaySeries.MidPrices)))
			sb.WriteString(fmt.Sprintf("EMA20: %v\n", formatFloatArray(data.IntradaySeries.EMA20Series)))
			sb.WriteString(fmt.Sprintf("MACD: %v\n", formatFloatArray(data.IntradaySeries.MACDSeries)))
			sb.WriteString(fmt.Sprintf("RSI7: %v\n", formatFloatArray(data.IntradaySeries.RSI7Series)))
			sb.WriteString(fmt.Sprintf("RSI14: %v\n", formatFloatArray(data.IntradaySeries.RSI14Series)))
			sb.WriteString("\n")
		}

		// 更长期上下文
		if data.LongerTermData != nil {
			sb.WriteString("### 更长期上下文（示例：1小时或4小时时间框架）\n")
			sb.WriteString(fmt.Sprintf("EMA20 vs EMA50: %s\n", data.LongerTermData.EMA20vsEMA50))
			sb.WriteString(fmt.Sprintf("ATR3 vs ATR14: %s\n", data.LongerTermData.ATR3vsATR14))
			sb.WriteString(fmt.Sprintf("成交量 vs 平均: %s\n", data.LongerTermData.VolumeVsAvg))
			sb.WriteString("\n")
		}
	}
}

// writeOpportunityRadar 写入机会雷达
func (s *PromptService) writeOpportunityRadar(sb *strings.Builder, marketDataMap map[string]*MarketData) {
	sb.WriteString("## 机会雷达\n\n")

	if len(marketDataMap) == 0 {
		sb.WriteString("缺少市场数据，暂无法识别高把握机会。\n\n")
		return
	}

	type opportunity struct {
		symbol  string
		score   int
		reasons []string
	}

	longOpps := make([]opportunity, 0)
	shortOpps := make([]opportunity, 0)

	for symbol, data := range marketDataMap {
		if data == nil || len(data.Timeframes) == 0 {
			continue
		}

		var longScore int
		var longReasons []string
		if tf1h, ok := data.Timeframes["1h"]; ok && tf1h != nil {
			if tf1h.RSI14 > 0 && tf1h.RSI14 <= 35 {
				longScore += 2
				longReasons = append(longReasons, fmt.Sprintf("1h RSI14=%.1f 显著超卖", tf1h.RSI14))
			}
			if tf1h.MACD > 0 && tf1h.EMA20 > tf1h.EMA50 {
				longScore++
				longReasons = append(longReasons, "1h EMA20>EMA50 且 MACD>0，多头动量延续")
			}
		}
		if tf5m, ok := data.Timeframes["5m"]; ok && tf5m != nil {
			if tf5m.RSI14 > 0 && tf5m.RSI14 <= 30 {
				longScore++
				longReasons = append(longReasons, fmt.Sprintf("5m RSI14=%.1f 极度超卖", tf5m.RSI14))
			}
			if tf5m.MACD > 0 && tf5m.EMA20 > tf5m.EMA50 {
				longScore++
				longReasons = append(longReasons, "5m 动量转多")
			}
		}
		if tf15m, ok := data.Timeframes["15m"]; ok && tf15m != nil {
			if tf15m.RSI14 > 0 && tf15m.RSI14 <= 30 {
				longScore++
				longReasons = append(longReasons, fmt.Sprintf("15m RSI14=%.1f 极度超卖", tf15m.RSI14))
			}
			if tf15m.MACD > 0 && tf15m.EMA20 > tf15m.EMA50 {
				longScore++
				longReasons = append(longReasons, "15m 动量由空转多")
			}
		}
		if data.FundingRate < -0.0001 {
			longScore++
			longReasons = append(longReasons, fmt.Sprintf("资金费率 %.4f%% 偏空，潜在逼空", data.FundingRate*100))
		}
		if longScore > 0 {
			longOpps = append(longOpps, opportunity{
				symbol:  symbol,
				score:   longScore,
				reasons: longReasons,
			})
		}

		var shortScore int
		var shortReasons []string
		if tf1h, ok := data.Timeframes["1h"]; ok && tf1h != nil {
			if tf1h.RSI14 >= 65 {
				shortScore += 2
				shortReasons = append(shortReasons, fmt.Sprintf("1h RSI14=%.1f 进入过热区", tf1h.RSI14))
			}
			if tf1h.MACD < 0 && tf1h.EMA20 < tf1h.EMA50 {
				shortScore++
				shortReasons = append(shortReasons, "1h EMA20<EMA50 且 MACD<0，空头力量加强")
			}
		}
		if tf5m, ok := data.Timeframes["5m"]; ok && tf5m != nil {
			if tf5m.RSI14 >= 70 {
				shortScore++
				shortReasons = append(shortReasons, fmt.Sprintf("5m RSI14=%.1f 极度超买", tf5m.RSI14))
			}
			if tf5m.MACD < 0 && tf5m.EMA20 < tf5m.EMA50 {
				shortScore++
				shortReasons = append(shortReasons, "5m 动量转空")
			}
		}
		if tf15m, ok := data.Timeframes["15m"]; ok && tf15m != nil {
			if tf15m.RSI14 >= 70 {
				shortScore++
				shortReasons = append(shortReasons, fmt.Sprintf("15m RSI14=%.1f 极度超买", tf15m.RSI14))
			}
			if tf15m.MACD < 0 && tf15m.EMA20 < tf15m.EMA50 {
				shortScore++
				shortReasons = append(shortReasons, "15m 动量由多转空")
			}
		}
		if data.FundingRate > 0.0001 {
			shortScore++
			shortReasons = append(shortReasons, fmt.Sprintf("资金费率 %.4f%% 偏多，回落压力大", data.FundingRate*100))
		}
		if shortScore > 0 {
			shortOpps = append(shortOpps, opportunity{
				symbol:  symbol,
				score:   shortScore,
				reasons: shortReasons,
			})
		}
	}

	sort.Slice(longOpps, func(i, j int) bool {
		if longOpps[i].score == longOpps[j].score {
			return longOpps[i].symbol < longOpps[j].symbol
		}
		return longOpps[i].score > longOpps[j].score
	})

	sort.Slice(shortOpps, func(i, j int) bool {
		if shortOpps[i].score == shortOpps[j].score {
			return shortOpps[i].symbol < shortOpps[j].symbol
		}
		return shortOpps[i].score > shortOpps[j].score
	})

	maxItems := func(listLen int) int {
		if listLen > 3 {
			return 3
		}
		return listLen
	}

	if len(longOpps) == 0 {
		sb.WriteString("- 当前未识别到高质量的多头候选，耐心等待更明确的共振信号。\n")
	} else {
		sb.WriteString("**多头候选（排序按共振强度）**\n")
		for _, opp := range longOpps[:maxItems(len(longOpps))] {
			sb.WriteString(fmt.Sprintf("- %s（评分 %d）：%s\n", opp.symbol, opp.score, strings.Join(opp.reasons, "；")))
		}
	}

	sb.WriteString("\n")

	if len(shortOpps) == 0 {
		sb.WriteString("- 当前未识别到高质量的空头候选，可等待价格反弹或结构破坏。\n\n")
	} else {
		sb.WriteString("**空头候选（排序按共振强度）**\n")
		for _, opp := range shortOpps[:maxItems(len(shortOpps))] {
			sb.WriteString(fmt.Sprintf("- %s（评分 %d）：%s\n", opp.symbol, opp.score, strings.Join(opp.reasons, "；")))
		}
		sb.WriteString("\n")
	}
}

// writeAccountInfo 写入账户信息
func (s *PromptService) writeAccountInfo(sb *strings.Builder, metrics *AccountMetrics) {
	sb.WriteString("## 账户与风险状态\n\n")

	if metrics == nil {
		sb.WriteString("暂无账户数据。\n\n")
		return
	}

	sb.WriteString(fmt.Sprintf("- 初始账户净值: $%.2f\n", metrics.InitialBalance))
	sb.WriteString(fmt.Sprintf("- 峰值账户净值: $%.2f\n", metrics.PeakBalance))
	sb.WriteString(fmt.Sprintf("- 当前账户价值: $%.2f\n", metrics.TotalBalance))
	sb.WriteString(fmt.Sprintf("- 账户回撤（从峰值）: %.2f%%\n", metrics.DrawdownFromPeak))
	sb.WriteString(fmt.Sprintf("- 账户回撤（从初始）: %.2f%%\n", metrics.DrawdownFromInitial))
	sb.WriteString(fmt.Sprintf("- 当前总收益率: %.2f%%\n", metrics.ReturnPercent))
	sb.WriteString(fmt.Sprintf("- 可用资金: $%.2f\n", metrics.Available))
	sb.WriteString(fmt.Sprintf("- 未实现盈亏: $%.2f\n\n", metrics.UnrealisedPnl))

	sb.WriteString("### 自主管理提醒\n")
	sb.WriteString("- 后端不会自动触发止损、止盈或强制平仓，请根据纪律自行调用工具执行。\n")

	//tc := s.config.Trading
	//maxDrawdown := tc.MaxDrawdownPercent
	//if maxDrawdown > 0 {
	//	forcedFlat := maxDrawdown + 5
	//	switch {
	//	case metrics.DrawdownFromPeak >= forcedFlat:
	//		sb.WriteString(fmt.Sprintf("- 回撤已达 %.2f%%（高于参考强平线 %.2f%%），必须制定并执行全仓退出计划。\n", metrics.DrawdownFromPeak, forcedFlat))
	//	case metrics.DrawdownFromPeak >= maxDrawdown:
	//		sb.WriteString(fmt.Sprintf("- 回撤 %.2f%% ≥ 参考阈值 %.2f%%，暂停新开仓，先处理存量风险。\n", metrics.DrawdownFromPeak, maxDrawdown))
	//	default:
	//		sb.WriteString(fmt.Sprintf("- 回撤 %.2f%% 低于参考阈值 %.2f%%，可继续谨慎评估机会。\n", metrics.DrawdownFromPeak, maxDrawdown))
	//	}
	//} else {
	//	sb.WriteString("- 配置未提供回撤阈值，请自行定义并严格执行风控纪律。\n")
	//}
	sb.WriteString("\n")
}

// writePositionInfo 写入持仓信息
func (s *PromptService) writePositionInfo(sb *strings.Builder, positions []*models.Position) {
	sb.WriteString("## 当前持仓\n\n")

	if len(positions) == 0 {
		sb.WriteString("当前无持仓。\n\n")
		return
	}

	maxHoldingHours := s.config.Trading.MaxHoldingHours

	for i, pos := range positions {
		pnlPercent := pos.CalculatePnlPercent()
		holdingHours := pos.CalculateHoldingHours()
		holdingCycles := pos.CalculateHoldingCycles()
		remainingHours := pos.RemainingHours()

		sb.WriteString(fmt.Sprintf("### 持仓 #%d\n", i+1))
		sb.WriteString(fmt.Sprintf("- 币种: %s\n", pos.Symbol))
		sb.WriteString(fmt.Sprintf("- 方向: %s\n", pos.Side))
		sb.WriteString(fmt.Sprintf("- 杠杆: %dx\n", pos.Leverage))
		sb.WriteString(fmt.Sprintf("- 盈亏百分比: %.2f%% （已考虑杠杆）\n", pnlPercent))
		sb.WriteString(fmt.Sprintf("- 盈亏金额: $%.2f\n", pos.UnrealizedPnl))
		sb.WriteString(fmt.Sprintf("- 开仓价: $%.2f\n", pos.EntryPrice))
		sb.WriteString(fmt.Sprintf("- 当前价: $%.2f\n", pos.CurrentPrice))
		sb.WriteString(fmt.Sprintf("- 开仓时间: %s\n", pos.OpenedAt.Format("2006-01-02 15:04:05")))
		sb.WriteString(fmt.Sprintf("- 已持仓: %.1f 小时 / %d 个周期\n", holdingHours, holdingCycles))
		if maxHoldingHours > 0 {
			sb.WriteString(fmt.Sprintf("- 距离参考持仓上限（%d 小时）剩余: %.1f 小时\n", maxHoldingHours, remainingHours))
		}

		if maxHoldingHours > 0 {
			if remainingHours <= 0 {
				sb.WriteString("- 时间提示：已超过参考持仓上限，必须制定并执行退出方案。\n")
			} else if remainingHours < 2 {
				sb.WriteString("- 时间提示：不足2小时到达参考持仓上限，请优先评估退出方案。\n")
			} else if remainingHours < 4 {
				sb.WriteString("- 时间提示：距离参考持仓上限不足4小时，请开始规划退出。\n")
			}
		}
		sb.WriteString("- 执行提示：若计划平仓，请在行动方案中明确调用 `closePosition` 并说明触发条件。\n")
		if strings.TrimSpace(pos.EntryReason) != "" {
			sb.WriteString(fmt.Sprintf("- 开仓理由：%s\n", pos.EntryReason))
		} else {
			sb.WriteString("- 开仓理由：未记录，请补充入场逻辑以便后续复盘。\n")
		}
		if strings.TrimSpace(pos.ExitPlan) != "" {
			sb.WriteString(fmt.Sprintf("- 退出计划：%s\n", pos.ExitPlan))
		} else {
			sb.WriteString("- 退出计划：缺失，请补充明确的止损/止盈或退出条件。\n")
		}
		sb.WriteString("\n")
	}
}

// writeTradeHistory 写入交易历史
func (s *PromptService) writeTradeHistory(sb *strings.Builder, trades []*models.Trade) {
	sb.WriteString("## 历史交易记录（最近10笔）\n\n")

	if len(trades) == 0 {
		sb.WriteString("暂无交易记录\n\n")
		return
	}

	for i, trade := range trades {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s %s, 价格=$%.2f, 数量=%.4f, 杠杆=%dx, 手续费=$%.2f",
			i+1, trade.ExecutedAt.Format("01-02 15:04"), trade.Type, trade.Symbol,
			trade.Price, trade.Quantity, trade.Leverage, trade.Fee))

		if trade.Type == "close" && trade.Pnl != 0 {
			sb.WriteString(fmt.Sprintf(", 盈亏=$%.2f", trade.Pnl))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
}

// writeDecisionHistory 写入决策历史
func (s *PromptService) writeDecisionHistory(sb *strings.Builder, decisions []*models.Decision) {
	sb.WriteString("## 历史AI决策（最近3次）\n\n")

	if len(decisions) == 0 {
		sb.WriteString("暂无历史决策。\n\n")
		return
	}

	sb.WriteString("回顾以下记录，评估哪些策略仍然有效，哪些需要调整。\n\n")

	for i, decision := range decisions {
		sb.WriteString(fmt.Sprintf("### 决策 #%d\n", i+1))
		sb.WriteString(fmt.Sprintf("- 时间: %s\n", decision.ExecutedAt.Format("2006-01-02 15:04:05")))
		sb.WriteString(fmt.Sprintf("- 调用次数: %d\n", decision.Iteration))
		sb.WriteString(fmt.Sprintf("- 账户价值: $%.2f\n", decision.AccountValue))
		sb.WriteString(fmt.Sprintf("- 持仓数量: %d\n", decision.PositionCount))
		sb.WriteString(fmt.Sprintf("- 决策内容:\n%s\n\n", decision.DecisionContent))
	}
}

// formatFloatArray 格式化浮点数组
func formatFloatArray(arr []float64) string {
	if len(arr) == 0 {
		return "[]"
	}

	strs := make([]string, len(arr))
	for i, v := range arr {
		strs[i] = fmt.Sprintf("%.2f", v)
	}
	return "[" + strings.Join(strs, ", ") + "]"
}

// GetSystemInstructions 获取系统指令
func (s *PromptService) GetSystemInstructions() string {
	tc := s.config.Trading

	formatFloat := func(val float64) string {
		str := fmt.Sprintf("%.2f", val)
		str = strings.TrimRight(str, "0")
		str = strings.TrimRight(str, ".")
		if str == "" {
			return "0"
		}
		return str
	}

	lowRange, midRange, highRange := leverageBands(tc.MinLeverage, tc.MaxLeverage)

	replacements := map[string]interface{}{
		"minutes_elapsed":        "{{minutes_elapsed}}",
		"current_time":           "{{current_time}}",
		"iteration_count":        "{{iteration_count}}",
		"max_drawdown_percent":   formatFloat(tc.MaxDrawdownPercent),
		"forced_flat_percent":    formatFloat(tc.MaxDrawdownPercent + 5),
		"max_holding_hours":      fmt.Sprintf("%d", tc.MaxHoldingHours),
		"max_positions":          fmt.Sprintf("%d", tc.MaxPositions),
		"risk_percent_per_trade": formatFloat(tc.RiskPercentPerTrade),
		"low_leverage_range":     lowRange,
		"mid_leverage_range":     midRange,
		"high_leverage_range":    highRange,
	}

	tmpl := fasttemplate.New(systemInstructionsTemplate, "{{", "}}")
	return tmpl.ExecuteString(replacements)
}

func leverageBands(minLev, maxLev int) (string, string, string) {
	if minLev >= maxLev {
		rangeStr := fmt.Sprintf("%d", minLev)
		return rangeStr, rangeStr, rangeStr
	}

	span := maxLev - minLev
	lowEnd := minLev + int(math.Ceil(float64(span)/3))
	midEnd := minLev + int(math.Ceil(float64(span)*2/3))

	if lowEnd > maxLev {
		lowEnd = maxLev
	}
	if midEnd > maxLev {
		midEnd = maxLev
	}

	lowRange := fmt.Sprintf("%d-%d", minLev, lowEnd)
	midRange := fmt.Sprintf("%d-%d", lowEnd, midEnd)
	highRange := fmt.Sprintf("%d-%d", midEnd, maxLev)

	return lowRange, midRange, highRange
}
