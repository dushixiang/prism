package service

import (
	"context"
	_ "embed"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dushixiang/prism/internal/models"
	"github.com/dushixiang/prism/internal/repo"
	"github.com/valyala/fasttemplate"
)

// PromptService AI提示词生成服务
type PromptService struct {
	tradeRepo          *repo.TradeRepo
	orderRepo          *repo.OrderRepo
	adminConfigService *AdminConfigService
}

// NewPromptService 创建提示词服务
func NewPromptService(tradeRepo *repo.TradeRepo, orderRepo *repo.OrderRepo, adminConfigService *AdminConfigService) *PromptService {
	return &PromptService{
		tradeRepo:          tradeRepo,
		orderRepo:          orderRepo,
		adminConfigService: adminConfigService,
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

	// 第一步: 获取交易配置
	tradingConfig, err := s.adminConfigService.GetTradingConfig(ctx)
	if err != nil {
		// 如果获取失败，使用默认值
		tradingConfig = &DefaultTradingConfig
	}

	var sb strings.Builder

	s.writeConversationContext(&sb, data)

	s.writeMarketOverview(&sb, data.MarketDataMap)

	s.writeAccountInfo(&sb, data.AccountMetrics, tradingConfig)

	s.writePositionInfo(&sb, data.Positions, data.AccountMetrics, tradingConfig)

	s.writeActiveOrders(&sb, data.ActiveOrders, data.Positions, data.MarketDataMap)

	s.writeTradeHistory(&sb, data.RecentTrades)

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

		// 根据价格确定精度
		pricePrecision := getPricePrecision(data.CurrentPrice)
		priceFormat := fmt.Sprintf("%%.%df", pricePrecision)

		sb.WriteString(fmt.Sprintf("### %s\n", symbol))

		sb.WriteString(fmt.Sprintf("💰 $"+priceFormat+" | 📊 资金费率 %.4f%%\n",
			data.CurrentPrice, data.FundingRate*100))
		if data.RecentHigh > 0 && data.RecentLow > 0 {
			sb.WriteString(fmt.Sprintf("**24h高低点**: $"+priceFormat+" / $"+priceFormat+"\n", data.RecentHigh, data.RecentLow))
		}
		sb.WriteString("\n")

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

				// ⭐ 计算价格与 EMA20 的偏离度（建议1）
				var emaDeviation float64
				var emaDeviationStr string
				if ind.EMA20 > 0 {
					emaDeviation = (ind.Price - ind.EMA20) / ind.EMA20 * 100
					emaDeviationStr = fmt.Sprintf(" 偏离EMA20 %+.2f%%", emaDeviation)
				}

				// 计算成交量比率（客观数据）
				var volumeRatioStr string
				if ind.AvgVolume > 0 {
					volumeRatio := ind.Volume / ind.AvgVolume
					volumeRatioStr = fmt.Sprintf(" (%.2fx均值)", volumeRatio)
				}

				// 使用新的多行格式
				sb.WriteString(fmt.Sprintf("- %s:\n", tf))
				sb.WriteString(fmt.Sprintf("  - 价格: $"+priceFormat+"%s\n", ind.Price, emaDeviationStr))
				sb.WriteString(fmt.Sprintf("  - 均线: EMA20=$"+priceFormat+" / EMA50=$"+priceFormat+"\n", ind.EMA20, ind.EMA50))
				sb.WriteString(fmt.Sprintf("  - 布林带: U=$"+priceFormat+" M=$"+priceFormat+" L=$"+priceFormat+"\n", ind.BBandsUpper, ind.BBandsMiddle, ind.BBandsLower))

				formatStr := fmt.Sprintf("  - 指标: MACD=%%.%df | RSI14=%%.1f | ATR14=%%.%df\n", macdPrecision, atrPrecision)
				sb.WriteString(fmt.Sprintf(formatStr, ind.MACD, ind.RSI14, ind.ATR14))

				sb.WriteString(fmt.Sprintf("  - 成交量: %s (均值: %s)%s\n",
					formatVolume(ind.Volume), formatVolume(ind.AvgVolume), volumeRatioStr))
			}
		}
		sb.WriteString("\n")

		// 价格走势概览 - 只显示收盘价趋势
		// 注意：IntradaySeries 使用15分钟K线（在 market_service.go 中定义）
		if data.IntradaySeries != nil && len(data.IntradaySeries.ClosePrices) > 0 {
			closes := data.IntradaySeries.ClosePrices
			count := len(closes)
			const intradayIntervalMinutes = 15.0 // 15分钟K线
			hours := float64(count) * intradayIntervalMinutes / 60.0

			// 计算最近6小时的价格变化
			if count > 0 {
				startPrice := closes[0]
				endPrice := closes[count-1]
				priceChange := (endPrice - startPrice) / startPrice * 100

				// 找出最高和最低价
				highPrice := closes[0]
				lowPrice := closes[0]
				for _, price := range closes {
					if price > highPrice {
						highPrice = price
					}
					if price < lowPrice {
						lowPrice = price
					}
				}
				volatility := (highPrice - lowPrice) / lowPrice * 100

				sb.WriteString(fmt.Sprintf("**价格走势 (15m周期, %.1f小时)**: ", hours))
				sb.WriteString(fmt.Sprintf("起 "+priceFormat+" → 终 "+priceFormat+" (%+.2f%%) | 区间 ["+priceFormat+"-"+priceFormat+"] 波幅%.2f%%\n",
					startPrice, endPrice, priceChange, lowPrice, highPrice, volatility))

				// 只显示最近16根K线的收盘价（约4小时），用于观察短期趋势
				recentCount := 16
				if count < recentCount {
					recentCount = count
				}
				recentCloses := closes[count-recentCount:]
				sb.WriteString(fmt.Sprintf("- 近期收盘价(最近%d根): %s\n",
					recentCount, formatPriceArray(recentCloses)))
			}
			sb.WriteString("\n")
		}

		// 1小时趋势
		if data.LongerTermData != nil {
			sb.WriteString("**1小时趋势**\n")

			// 1小时均线结构（客观描述）
			var trendDesc string
			if ind1h, ok := data.Timeframes["1h"]; ok && ind1h.Price > 0 {
				strength := (ind1h.EMA20 - ind1h.EMA50) / ind1h.Price * 100
				adx := ind1h.ADX14

				// 均线位置关系（客观描述）
				var emaRelation string
				if strength > 0.05 { // 增加一个小的阈值避免过于频繁的波动
					emaRelation = "EMA20 在 EMA50 上方"
				} else if strength < -0.05 {
					emaRelation = "EMA20 在 EMA50 下方"
				} else {
					emaRelation = "EMA20 与 EMA50 接近"
				}

				trendDesc = fmt.Sprintf("- **1h 均线关系**: %s | **ADX14**: %.1f", emaRelation, adx)
				sb.WriteString(trendDesc + "\n")
				sb.WriteString(fmt.Sprintf("- **均线偏离度**: %.2f%% (EMA20 vs EMA50)\n", strength))
			}

			// 波动率和成交量状态（客观描述）
			atrStatus := translateStatus(data.LongerTermData.ATR3vsATR14, "ATR3", "ATR14", "高于", "低于", "等于")
			volStatus := translateStatus(data.LongerTermData.VolumeVsAvg, "当前成交量", "均值", "高于", "低于", "等于")

			sb.WriteString(fmt.Sprintf("- 波动与成交量: %s | %s\n", atrStatus, volStatus))

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
func (s *PromptService) writeAccountInfo(sb *strings.Builder, metrics *AccountMetrics, tradingConfig *models.TradingConfig) {
	sb.WriteString("## 账户状态\n\n")

	if metrics == nil {
		sb.WriteString("暂无账户数据。\n\n")
		return
	}

	availablePercent := 0.0
	if metrics.TotalBalance > 0 {
		availablePercent = (metrics.Available / metrics.TotalBalance) * 100
	}

	formatPercent := func(val float64) string {
		str := fmt.Sprintf("%.2f", val)
		str = strings.TrimRight(str, "0")
		str = strings.TrimRight(str, ".")
		if str == "" {
			return "0"
		}
		return str
	}

	drawdownWarn := tradingConfig.MaxDrawdownPercent
	forcedFlat := tradingConfig.MaxDrawdownPercent + 5

	// 资金情况
	sb.WriteString(fmt.Sprintf("**资金**: 净值 $%.2f (初始$%.2f, 峰值$%.2f) | 可用 $%.2f (%.1f%%)\n",
		metrics.TotalBalance,
		metrics.InitialBalance,
		metrics.PeakBalance,
		metrics.Available,
		availablePercent))

	// 收益与风险
	returnEmoji := "📈"
	if metrics.ReturnPercent < 0 {
		returnEmoji = "📉"
	}
	sb.WriteString(fmt.Sprintf("**收益**: %s %+.2f%% | 未实现盈亏 $%+.2f\n",
		returnEmoji,
		metrics.ReturnPercent,
		metrics.UnrealisedPnl))

	// 回撤与夏普比率
	drawdownEmoji := "✅"
	riskNote := ""
	if forcedFlat > 0 && metrics.DrawdownFromPeak >= forcedFlat {
		drawdownEmoji = "🔴"
		riskNote = fmt.Sprintf(" | 已达到强制清仓阈值%s%%（系统规则）", formatPercent(forcedFlat))
	} else if drawdownWarn > 0 && metrics.DrawdownFromPeak >= drawdownWarn {
		drawdownEmoji = "⚠️"
		riskNote = fmt.Sprintf(" | 已达到警戒线%s%%（系统规则）", formatPercent(drawdownWarn))
	}

	sharpeEmoji := "📊"
	sharpeText := "N/A"
	if metrics.SharpeRatio != 0 {
		sharpeText = fmt.Sprintf("%.2f", metrics.SharpeRatio)
		if metrics.SharpeRatio > 1.0 {
			sharpeEmoji = "🌟"
		} else if metrics.SharpeRatio < 0 {
			sharpeEmoji = "⚠️"
		}
	}

	sb.WriteString(fmt.Sprintf("**风险**: %s 回撤 %.2f%%(峰值) / %.2f%%(初始) | %s 夏普比率 %s%s\n\n",
		drawdownEmoji,
		metrics.DrawdownFromPeak,
		metrics.DrawdownFromInitial,
		sharpeEmoji,
		sharpeText,
		riskNote))
}

// writePositionInfo 写入持仓信息
func (s *PromptService) writePositionInfo(sb *strings.Builder, positions []models.Position, metrics *AccountMetrics, tradingConfig *models.TradingConfig) {
	maxPositions := tradingConfig.MaxPositions
	currentCount := len(positions)

	sb.WriteString("## 当前持仓\n\n")

	if currentCount > 0 {
		sb.WriteString(fmt.Sprintf("**持仓: %d/%d**\n\n", currentCount, maxPositions))
	}

	if len(positions) == 0 {
		sb.WriteString(fmt.Sprintf("当前无持仓，最多可开 %d 个仓位\n\n", maxPositions))
	} else {
		for i := range positions {
			pos := &positions[i] // 取地址以便调用方法
			pnlPercent := pos.CalculatePnlPercent()
			holding := pos.CalculateHoldingStr()

			pricePrecision := getPricePrecision(pos.CurrentPrice)
			priceFormat := fmt.Sprintf("%%.%df", pricePrecision)

			sb.WriteString(fmt.Sprintf("### %d. %s %s\n", i+1, pos.Symbol, strings.ToUpper(pos.Side)))

			// 基本信息
			sb.WriteString(fmt.Sprintf("- 价格: 入场$"+priceFormat+" → 当前$"+priceFormat+"\n",
				pos.EntryPrice, pos.CurrentPrice))
			sb.WriteString(fmt.Sprintf("- 盈亏: $%+.2f (%+.2f%%)", pos.UnrealizedPnl, pnlPercent))

			// 显示历史峰值盈亏（如果有）
			if pos.PeakPnlPercent != 0 {
				sb.WriteString(fmt.Sprintf(" | 峰值盈亏 %+.2f%%", pos.PeakPnlPercent))
			}
			sb.WriteString("\n")

			// 杠杆和保证金
			sb.WriteString(fmt.Sprintf("- 杠杆: %dx | 保证金: $%.2f | 数量: %.4f\n",
				pos.Leverage, pos.Margin, pos.Quantity))

			// 强平价格和风险度
			if pos.LiquidationPrice > 0 {
				liquidationDistance := 0.0
				if pos.CurrentPrice > 0 {
					if pos.Side == "long" {
						liquidationDistance = (pos.LiquidationPrice - pos.CurrentPrice) / pos.CurrentPrice * 100
					} else {
						liquidationDistance = (pos.CurrentPrice - pos.LiquidationPrice) / pos.CurrentPrice * 100
					}
				}
				sb.WriteString(fmt.Sprintf("- 强平价格: $"+priceFormat+" (距当前价格 %+.2f%%)\n",
					pos.LiquidationPrice, liquidationDistance))
			}

			// 持仓时间
			sb.WriteString(fmt.Sprintf("- 持仓时间: %s\n\n", holding))

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

	// 仓位容量信息
	remainingSlots := maxPositions - currentCount
	if remainingSlots > 0 && metrics != nil && metrics.Available > 0 {
		sb.WriteString("## 仓位容量\n\n")

		sb.WriteString(fmt.Sprintf("**剩余可开仓位**: %d个（最大%d个）\n", remainingSlots, maxPositions))
		sb.WriteString(fmt.Sprintf("**当前可用余额**: $%.2f\n", metrics.Available))
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
	sb.WriteString("## 历史交易记录（最近20笔）\n\n")

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
		pricePrecision := getPricePrecision(trade.Price)
		priceFormat := fmt.Sprintf("%%.%df", pricePrecision)
		sb.WriteString(fmt.Sprintf("%d. [%s] %s %s, 价格=$"+priceFormat+", 数量=%.4f, 杠杆=%dx, 手续费=$%.2f",
			i+1, trade.ExecutedAt.Format("01-02 15:04"), trade.Type, trade.Symbol,
			trade.Price, trade.Quantity, trade.Leverage, trade.Fee))

		if trade.Type == "close" && trade.Pnl != 0 {
			pnlSign := ""
			if trade.Pnl > 0 {
				pnlSign = "+"
			}
			sb.WriteString(fmt.Sprintf(", 盈亏=%s$%.2f", pnlSign, trade.Pnl))
		}

		// 添加交易原因,帮助AI了解盈亏原因
		if strings.TrimSpace(trade.Reason) != "" {
			sb.WriteString(fmt.Sprintf(", 原因: %s", trade.Reason))
		}

		sb.WriteString("\n")
	}
	sb.WriteString("\n")
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

// formatVolume 格式化成交量，使其更具可读性
func formatVolume(vol float64) string {
	switch {
	case vol >= 1_000_000:
		return fmt.Sprintf("%.1fM", vol/1_000_000)
	case vol >= 1_000:
		return fmt.Sprintf("%.1fK", vol/1_000)
	default:
		return fmt.Sprintf("%.0f", vol)
	}
}

// translateStatus 将英文状态关键字翻译为更具可读性的中文
func translateStatus(status, v1, v2, above, below, equal string) string {
	switch status {
	case "above", "higher":
		return fmt.Sprintf("%s %s %s", v1, above, v2)
	case "below", "lower":
		return fmt.Sprintf("%s %s %s", v1, below, v2)
	case "equal":
		return fmt.Sprintf("%s %s %s", v1, equal, v2)
	default:
		return ""
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

// GetSystemInstructions 获取系统指令
func (s *PromptService) GetSystemInstructions(ctx context.Context) (string, error) {
	// 获取系统提示词
	prompt, err := s.adminConfigService.GetSystemPrompt(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get system prompt: %w", err)
	}

	// 获取交易配置
	tradingConfig, err := s.adminConfigService.GetTradingConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get trading config: %w", err)
	}

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
		"max_drawdown_percent": formatFloat(tradingConfig.MaxDrawdownPercent),
		"forced_flat_percent":  formatFloat(tradingConfig.MaxDrawdownPercent + 5),
		"max_positions":        fmt.Sprintf("%d", tradingConfig.MaxPositions),
		"min_leverage":         fmt.Sprintf("%d", tradingConfig.MinLeverage),
		"max_leverage":         fmt.Sprintf("%d", tradingConfig.MaxLeverage),
		"interval_minutes":     fmt.Sprintf("%d", tradingConfig.IntervalMinutes),
	}

	tmpl := fasttemplate.New(prompt.Content, "{{", "}}")
	return tmpl.ExecuteString(replacements), nil
}
