package service

import (
	"context"
	_ "embed"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dushixiang/prism/internal/config"
	"github.com/dushixiang/prism/internal/models"
	"github.com/dushixiang/prism/internal/repo"
	"github.com/valyala/fasttemplate"
)

// PromptService AI提示词生成服务
type PromptService struct {
	config    *config.Config
	tradeRepo *repo.TradeRepo
}

//go:embed templates/system_instructions.txt
var systemInstructionsTemplate string

// NewPromptService 创建提示词服务
func NewPromptService(conf *config.Config, tradeRepo *repo.TradeRepo) *PromptService {
	return &PromptService{
		config:    conf,
		tradeRepo: tradeRepo,
	}
}

// PromptData 提示词数据
type PromptData struct {
	StartTime      time.Time
	Iteration      int
	AccountMetrics *AccountMetrics
	MarketDataMap  map[string]*MarketData
	Positions      []models.Position // 持仓列表（值切片）
	RecentTrades   []models.Trade    // 最近交易（值切片）
	ActiveOrders   []models.Order    // 活跃的限价订单（值切片）
}

// GeneratePrompt 生成完整的AI提示词
func (s *PromptService) GeneratePrompt(ctx context.Context, data *PromptData) string {
	if data == nil {
		return ""
	}

	var sb strings.Builder

	s.writeConversationContext(&sb, data)

	s.writeMarketOverview(&sb, data.MarketDataMap)

	s.writeAccountInfo(&sb, data.AccountMetrics)

	s.writePositionInfo(&sb, data.Positions)

	s.writeActiveOrders(&sb, data.ActiveOrders, data.Positions, data.MarketDataMap)

	s.writeTradeHistory(&sb, data.RecentTrades)

	s.writePerformanceMetrics(&sb, data.AccountMetrics)

	return sb.String()
}

// writeConversationContext 写入通话背景
func (s *PromptService) writeConversationContext(sb *strings.Builder, data *PromptData) {
	now := time.Now().In(time.FixedZone("CST", 8*3600))
	currentTime := now.Format("2006-01-02 15:04:05")

	var minutesElapsed float64
	// 使用第一笔交易时间作为起始时间，如果没有交易则使用启动时间
	ctx := context.Background()
	firstTrade, err := s.tradeRepo.FindFirstTrade(ctx)
	if err == nil && firstTrade != nil {
		minutesElapsed = time.Since(firstTrade.ExecutedAt).Minutes()
		if minutesElapsed < 0 {
			minutesElapsed = 0
		}
	} else if !data.StartTime.IsZero() {
		// 如果没有交易记录，仍然显示启动后的时间
		minutesElapsed = time.Since(data.StartTime).Minutes()
		if minutesElapsed < 0 {
			minutesElapsed = 0
		}
	}

	sb.WriteString(fmt.Sprintf("**时间**: %s | **周期**: #%d | **运行**: %.0f分钟\n\n",
		currentTime, data.Iteration, minutesElapsed))
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

		sb.WriteString(fmt.Sprintf("### %s\n", symbol))

		// 根据价格确定精度
		pricePrecision := getPricePrecision(data.CurrentPrice)
		priceFormat := fmt.Sprintf("%%.%df", pricePrecision)

		sb.WriteString(fmt.Sprintf("价格$"+priceFormat+" | 资金费率%.4f%%\n\n", data.CurrentPrice, data.FundingRate*100))

		// 多时间框架指标（紧凑格式）
		sb.WriteString("**多周期指标**\n")
		timeframes := []string{"15m", "30m", "1h"}
		for _, tf := range timeframes {
			if ind, ok := data.Timeframes[tf]; ok {
				// ATR精度：使用价格精度+2（因为ATR通常比价格小1-2个数量级）
				atrPrecision := pricePrecision + 2
				if atrPrecision > 8 {
					atrPrecision = 8
				}

				// MACD精度：对于低价币使用更高精度
				macdPrecision := 4
				if data.CurrentPrice < 1.0 {
					macdPrecision = 6
				}

				// 动态构建格式字符串
				formatStr := fmt.Sprintf("- %%s: 价格$%s | EMA20/50: $%s/$%s | MACD=%%.%df(信号%%.%df,柱%%.%df) | RSI7/14=%%.1f/%%.1f | ATR3/14=%%.%df/%%.%df | 成交量=%%.0f(均%%.0f)\n",
					priceFormat, priceFormat, priceFormat, macdPrecision, macdPrecision, macdPrecision, atrPrecision, atrPrecision)

				sb.WriteString(fmt.Sprintf(formatStr,
					tf, ind.Price, ind.EMA20, ind.EMA50,
					ind.MACD, ind.MACDSignal, ind.MACDHist,
					ind.RSI7, ind.RSI14,
					ind.ATR3, ind.ATR14,
					ind.Volume, ind.AvgVolume))
			}
		}
		sb.WriteString("\n")

		// 日内序列（15分钟K线）- 紧凑格式
		if data.IntradaySeries != nil && len(data.IntradaySeries.ClosePrices) > 0 {
			count := len(data.IntradaySeries.OpenPrices)
			hours := float64(count) * 15.0 / 60.0
			sb.WriteString(fmt.Sprintf("**15分钟K线序列**（最近%d根，约%.1f小时）\n", count, hours))

			// OHLC合并为紧凑格式: [O|H|L|C]
			sb.WriteString(fmt.Sprintf("- K线[O|H|L|C]: %s\n",
				formatOHLCArray(data.IntradaySeries.OpenPrices,
					data.IntradaySeries.HighPrices,
					data.IntradaySeries.LowPrices,
					data.IntradaySeries.ClosePrices)))
			sb.WriteString("\n")
		}

		// 1小时趋势
		if data.LongerTermData != nil {
			sb.WriteString("**1小时趋势**\n")
			sb.WriteString(fmt.Sprintf("- EMA20 vs EMA50: %s | ATR3 vs ATR14: %s | 成交量 vs 均值: %s\n",
				data.LongerTermData.EMA20vsEMA50,
				data.LongerTermData.ATR3vsATR14,
				data.LongerTermData.VolumeVsAvg))

			// 1小时序列数据（最近10点）
			if len(data.LongerTermData.MACDSeries) > 0 || len(data.LongerTermData.RSI14Series) > 0 {
				sb.WriteString("- MACD序列: ")
				sb.WriteString(formatFloatArray(data.LongerTermData.MACDSeries))
				sb.WriteString("\n")
				sb.WriteString("- RSI14序列: ")
				sb.WriteString(formatFloatArray(data.LongerTermData.RSI14Series))
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
	}
}

// writeAccountInfo 写入账户信息
func (s *PromptService) writeAccountInfo(sb *strings.Builder, metrics *AccountMetrics) {
	sb.WriteString("## 账户状态\n\n")

	if metrics == nil {
		sb.WriteString("暂无账户数据。\n\n")
		return
	}

	availablePercent := 0.0
	if metrics.TotalBalance > 0 {
		availablePercent = (metrics.Available / metrics.TotalBalance) * 100
	}

	sb.WriteString(fmt.Sprintf("净值$%.2f(初始$%.2f,峰值$%.2f) | 可用$%.2f(%.1f%%) | 收益%+.2f%% | 回撤%.2f%%(峰值)/%.2f%%(初始) | 未实现盈亏$%+.2f\n\n",
		metrics.TotalBalance,
		metrics.InitialBalance,
		metrics.PeakBalance,
		metrics.Available,
		availablePercent,
		metrics.ReturnPercent,
		metrics.DrawdownFromPeak,
		metrics.DrawdownFromInitial,
		metrics.UnrealisedPnl))
}

// writePositionInfo 写入持仓信息
func (s *PromptService) writePositionInfo(sb *strings.Builder, positions []models.Position) {
	maxPositions := s.config.Trading.MaxPositions
	currentCount := len(positions)

	sb.WriteString("## 当前持仓\n\n")

	if currentCount > 0 {
		sb.WriteString(fmt.Sprintf("**持仓: %d/%d**\n\n", currentCount, maxPositions))
	}

	if len(positions) == 0 {
		sb.WriteString(fmt.Sprintf("当前无持仓，最多可开 %d 个仓位\n\n", maxPositions))
		return
	}

	for i := range positions {
		pos := &positions[i] // 取地址以便调用方法
		pnlPercent := pos.CalculatePnlPercent()
		holding := pos.CalculateHoldingStr()

		sb.WriteString(fmt.Sprintf("### %d. %s %s\n", i+1, pos.Symbol, strings.ToUpper(pos.Side)))
		sb.WriteString(fmt.Sprintf("入场$%.2f → 当前$%.2f | 盈亏$%+.2f (%+.2f%%) | %dx杠杆 | 持仓时间 %s\n\n",
			pos.EntryPrice, pos.CurrentPrice, pos.UnrealizedPnl, pnlPercent, pos.Leverage, holding))

		// 开仓理由和退出计划
		if strings.TrimSpace(pos.EntryReason) != "" {
			sb.WriteString(fmt.Sprintf("**开仓理由**: %s\n\n", pos.EntryReason))
		}
		if strings.TrimSpace(pos.ExitPlan) != "" {
			sb.WriteString(fmt.Sprintf("**退出计划**: %s\n\n", pos.ExitPlan))
		}

		sb.WriteString("\n")
	}
}

// writeActiveOrders 写入活跃的限价订单信息
func (s *PromptService) writeActiveOrders(sb *strings.Builder, orders []models.Order, positions []models.Position, marketDataMap map[string]*MarketData) {
	sb.WriteString("## 活跃限价单\n\n")

	if len(orders) == 0 {
		sb.WriteString("当前无活跃限价单\n\n")
		return
	}

	// 按持仓分组订单
	ordersByPosition := make(map[string][]models.Order)
	for i := range orders {
		if orders[i].IsActive() {
			ordersByPosition[orders[i].PositionID] = append(ordersByPosition[orders[i].PositionID], orders[i])
		}
	}

	if len(ordersByPosition) == 0 {
		sb.WriteString("当前无活跃限价单\n\n")
		return
	}

	// 创建持仓ID到持仓的映射
	positionMap := make(map[string]*models.Position)
	for i := range positions {
		positionMap[positions[i].ID] = &positions[i]
	}

	// 按持仓展示订单
	posIdx := 1
	for posID, posOrders := range ordersByPosition {
		pos := positionMap[posID]
		if pos == nil {
			continue
		}

		// 获取当前价格
		currentPrice := pos.CurrentPrice
		if marketData, ok := marketDataMap[pos.Symbol]; ok && marketData != nil {
			currentPrice = marketData.CurrentPrice
		}

		sb.WriteString(fmt.Sprintf("### 持仓#%d %s %s\n", posIdx, pos.Symbol, strings.ToUpper(pos.Side)))

		// 分类订单
		var stopLossOrders []models.Order
		var takeProfitOrders []models.Order
		for i := range posOrders {
			if posOrders[i].IsStopLoss() {
				stopLossOrders = append(stopLossOrders, posOrders[i])
			} else if posOrders[i].IsTakeProfit() {
				takeProfitOrders = append(takeProfitOrders, posOrders[i])
			}
		}

		// 显示止损单
		if len(stopLossOrders) > 0 {
			for i := range stopLossOrders {
				order := &stopLossOrders[i]
				distance := order.CalculateDistancePercent(currentPrice)
				createdTime := order.CreatedAt.Format("01-02 15:04")
				pricePrecision := getPricePrecision(order.TriggerPrice)
				priceFormat := fmt.Sprintf("%%.%df", pricePrecision)

				sb.WriteString(fmt.Sprintf("- **止损**: $"+priceFormat+" (距当前价格 %+.2f%%) | 创建于 %s",
					order.TriggerPrice, distance, createdTime))

				if order.Reason != "" {
					sb.WriteString(fmt.Sprintf(" | 原因: %s", order.Reason))
				}
				sb.WriteString("\n")
			}
		}

		// 显示止盈单
		if len(takeProfitOrders) > 0 {
			for i := range takeProfitOrders {
				order := &takeProfitOrders[i]
				distance := order.CalculateDistancePercent(currentPrice)
				createdTime := order.CreatedAt.Format("01-02 15:04")
				pricePrecision := getPricePrecision(order.TriggerPrice)
				priceFormat := fmt.Sprintf("%%.%df", pricePrecision)

				sb.WriteString(fmt.Sprintf("- **止盈**: $"+priceFormat+" (距当前价格 %+.2f%%) | 创建于 %s",
					order.TriggerPrice, distance, createdTime))

				if order.Reason != "" {
					sb.WriteString(fmt.Sprintf(" | 原因: %s", order.Reason))
				}
				sb.WriteString("\n")
			}
		}

		sb.WriteString("\n")
		posIdx++
	}
}

// writeTradeHistory 写入交易历史
func (s *PromptService) writeTradeHistory(sb *strings.Builder, trades []models.Trade) {
	sb.WriteString("## 历史交易记录（最近10笔）\n\n")

	if len(trades) == 0 {
		sb.WriteString("暂无交易记录\n\n")
		return
	}

	// 统计信息
	var totalPnl, totalFees float64
	var wins, losses int
	for i := range trades {
		if trades[i].Type == "close" {
			totalPnl += trades[i].Pnl
			if trades[i].Pnl > 0 {
				wins++
			} else if trades[i].Pnl < 0 {
				losses++
			}
		}
		totalFees += trades[i].Fee
	}

	closedTrades := wins + losses
	if closedTrades > 0 {
		winRate := float64(wins) / float64(closedTrades) * 100
		sb.WriteString(fmt.Sprintf("**统计**: 胜率 %.0f%% (%d胜/%d负) | 净盈亏 $%.2f | 累计手续费 $%.2f\n\n",
			winRate, wins, losses, totalPnl, totalFees))
	}

	// 交易列表
	for i := range trades {
		trade := &trades[i]
		sb.WriteString(fmt.Sprintf("%d. [%s] %s %s, 价格=$%.2f, 数量=%.4f, 杠杆=%dx, 手续费=$%.2f",
			i+1, trade.ExecutedAt.Format("01-02 15:04"), trade.Type, trade.Symbol,
			trade.Price, trade.Quantity, trade.Leverage, trade.Fee))

		if trade.Type == "close" && trade.Pnl != 0 {
			pnlSign := ""
			if trade.Pnl > 0 {
				pnlSign = "+"
			}
			sb.WriteString(fmt.Sprintf(", 盈亏=%s$%.2f", pnlSign, trade.Pnl))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
}

// writePerformanceMetrics 写入性能指标
func (s *PromptService) writePerformanceMetrics(sb *strings.Builder, metrics *AccountMetrics) {
	if metrics == nil {
		return
	}

	sb.WriteString("## 性能指标\n\n")
	sb.WriteString(fmt.Sprintf("- 夏普比率: %.2f", metrics.SharpeRatio))
}

// getPricePrecision 根据价格范围获取合适的小数精度
func getPricePrecision(avgPrice float64) int {
	switch {
	case avgPrice >= 100:
		return 1 // 大于100: 保留1位 (如 BTC: 50000.1)
	case avgPrice >= 1:
		return 2 // 1-100: 保留2位 (如 ETH: 2500.12)
	case avgPrice >= 0.01:
		return 4 // 0.01-1: 保留4位 (如某些山寨币: 0.1234)
	default:
		return 6 // 小于0.01: 保留6位 (如 SHIB: 0.000012)
	}
}

// formatFloatArray 格式化浮点数组（固定2位小数，用于RSI/MACD等指标）
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

// formatPriceArray 格式化价格数组（自适应精度）
func formatPriceArray(arr []float64) string {
	if len(arr) == 0 {
		return "[]"
	}

	// 计算平均值以确定精度
	avgPrice := 0.0
	for _, v := range arr {
		avgPrice += v
	}
	avgPrice /= float64(len(arr))

	precision := getPricePrecision(avgPrice)
	formatStr := fmt.Sprintf("%%.%df", precision)
	strs := make([]string, len(arr))
	for i, v := range arr {
		strs[i] = fmt.Sprintf(formatStr, v)
	}
	return "[" + strings.Join(strs, ", ") + "]"
}

// formatOHLCArray 格式化OHLC数组为紧凑格式
func formatOHLCArray(opens, highs, lows, closes []float64) string {
	if len(opens) == 0 || len(opens) != len(highs) || len(opens) != len(lows) || len(opens) != len(closes) {
		return "[]"
	}

	// 计算平均价格以确定精度
	avgPrice := 0.0
	for i := range closes {
		avgPrice += closes[i]
	}
	avgPrice /= float64(len(closes))

	precision := getPricePrecision(avgPrice)
	formatStr := fmt.Sprintf("[%%.%df|%%.%df|%%.%df|%%.%df]", precision, precision, precision, precision)

	var sb strings.Builder
	sb.WriteString("[")
	for i := range opens {
		if i > 0 {
			sb.WriteString(", ")
		}
		// 格式: [O|H|L|C] 例如 [100.5|101.2|99.8|100.9]
		sb.WriteString(fmt.Sprintf(formatStr, opens[i], highs[i], lows[i], closes[i]))
	}
	sb.WriteString("]")
	return sb.String()
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

	replacements := map[string]interface{}{
		"minutes_elapsed":        "{{minutes_elapsed}}",
		"current_time":           "{{current_time}}",
		"iteration_count":        "{{iteration_count}}",
		"max_drawdown_percent":   formatFloat(tc.MaxDrawdownPercent),
		"forced_flat_percent":    formatFloat(tc.MaxDrawdownPercent + 5),
		"max_positions":          fmt.Sprintf("%d", tc.MaxPositions),
		"risk_percent_per_trade": formatFloat(tc.RiskPercentPerTrade),
		"min_leverage":           fmt.Sprintf("%d", tc.MinLeverage),
		"max_leverage":           fmt.Sprintf("%d", tc.MaxLeverage),
	}

	tmpl := fasttemplate.New(systemInstructionsTemplate, "{{", "}}")
	return tmpl.ExecuteString(replacements)
}
