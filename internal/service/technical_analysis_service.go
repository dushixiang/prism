package service

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/dushixiang/prism/internal/models"
	"github.com/dushixiang/prism/pkg/ta"
	"go.uber.org/zap"
)

// TechnicalAnalysisService 技术分析服务
type TechnicalAnalysisService struct {
	logger *zap.Logger
}

// NewTechnicalAnalysisService 创建技术分析服务实例
func NewTechnicalAnalysisService(logger *zap.Logger) *TechnicalAnalysisService {
	return &TechnicalAnalysisService{
		logger: logger,
	}
}

// CalculateIndicators 计算技术指标
func (s *TechnicalAnalysisService) CalculateIndicators(klineData []models.KlineData) (*models.TechnicalIndicators, error) {
	if len(klineData) < 2 {
		return nil, fmt.Errorf("数据点不足，至少需要2个数据点")
	}

	// 提取价格数据
	closePrices := make([]float64, len(klineData))
	highPrices := make([]float64, len(klineData))
	lowPrices := make([]float64, len(klineData))
	volumes := make([]float64, len(klineData))

	for i, kline := range klineData {
		closePrices[i] = kline.ClosePrice
		highPrices[i] = kline.HighPrice
		lowPrices[i] = kline.LowPrice
		volumes[i] = kline.Volume
	}

	indicators := &models.TechnicalIndicators{}

	// 计算移动平均线 - 只有数据足够时才计算
	if len(closePrices) >= 5 {
		ma5 := ta.SMA(closePrices, 5)
		if len(ma5) > 0 {
			indicators.MA5 = ma5[len(ma5)-1]
		}
	}

	if len(closePrices) >= 10 {
		ma10 := ta.SMA(closePrices, 10)
		if len(ma10) > 0 {
			indicators.MA10 = ma10[len(ma10)-1]
		}
	}

	if len(closePrices) >= 20 {
		ma20 := ta.SMA(closePrices, 20)
		if len(ma20) > 0 {
			indicators.MA20 = ma20[len(ma20)-1]
		}
	}

	if len(closePrices) >= 50 {
		ma50 := ta.SMA(closePrices, 50)
		if len(ma50) > 0 {
			indicators.MA50 = ma50[len(ma50)-1]
		}
	}

	if len(closePrices) >= 200 {
		ma200 := ta.SMA(closePrices, 200)
		if len(ma200) > 0 {
			indicators.MA200 = ma200[len(ma200)-1]
		}
	}

	// 计算EMA - 确保有足够数据
	if len(closePrices) >= 12 {
		ema12 := ta.EMA(closePrices, 12)
		if len(ema12) > 0 {
			indicators.EMA12 = ema12[len(ema12)-1]
		}
	}

	if len(closePrices) >= 26 {
		ema26 := ta.EMA(closePrices, 26)
		if len(ema26) > 0 {
			indicators.EMA26 = ema26[len(ema26)-1]
		}
	}

	// 计算MACD - 需要至少35个数据点 (26+9)
	if len(closePrices) >= 35 {
		macd, signal, hist := ta.MACD(closePrices, 12, 26, 9)
		if len(macd) > 0 {
			indicators.MACD = macd[len(macd)-1]
			indicators.MACDSignal = signal[len(signal)-1]
			indicators.MACDHist = hist[len(hist)-1]
		}
	}

	// 计算RSI - 需要至少15个数据点
	if len(closePrices) >= 15 {
		rsi := ta.RSI(closePrices, 14)
		if len(rsi) > 0 {
			v := rsi[len(rsi)-1]
			if v < 0 {
				v = 0
			} else if v > 100 {
				v = 100
			}
			indicators.RSI = v
		}
	}

	// 计算布林带 - 需要至少20个数据点
	if len(closePrices) >= 20 {
		upper, middle, lower := ta.BB(closePrices, 20, 2.0, ta.TypeSMA)
		if len(upper) > 0 {
			indicators.BBUpper = upper[len(upper)-1]
			indicators.BBMiddle = middle[len(middle)-1]
			indicators.BBLower = lower[len(lower)-1]
		}
	}

	// 计算KDJ (使用Stoch) - 需要至少15个数据点
	if len(closePrices) >= 15 {
		stochK, stochD := ta.Stoch(highPrices, lowPrices, closePrices, 9, 3, ta.TypeSMA, 3, ta.TypeSMA)
		if len(stochK) > 0 && len(stochD) > 0 {
			indicators.StochK = stochK[len(stochK)-1]
			indicators.StochD = stochD[len(stochD)-1]
			// J = 3*K - 2*D
			indicators.StochJ = 3*indicators.StochK - 2*indicators.StochD
		}
	}

	// 计算CCI - 需要至少20个数据点
	if len(closePrices) >= 20 {
		cci := ta.CCI(highPrices, lowPrices, closePrices, 20)
		if len(cci) > 0 {
			indicators.CCI = cci[len(cci)-1]
		}
	}

	// 计算SAR - 需要至少2个数据点
	if len(closePrices) >= 2 {
		sar := ta.SAR(highPrices, lowPrices, 0.02, 0.2)
		if len(sar) > 0 {
			indicators.SAR = sar[len(sar)-1]
		}
	}

	// 计算ATR - 需要至少15个数据点
	if len(closePrices) >= 15 {
		atr := ta.ATR(highPrices, lowPrices, closePrices, 14)
		if len(atr) > 0 {
			indicators.ATR = atr[len(atr)-1]
		}
	}

	// 计算ADX - 需要至少15个数据点
	if len(closePrices) >= 15 {
		adx := ta.ADX(highPrices, lowPrices, closePrices, 14)
		if len(adx) > 0 {
			indicators.ADX = adx[len(adx)-1]
		}
	}

	// 计算OBV - 需要至少2个数据点
	if len(closePrices) >= 2 {
		obv := ta.OBV(closePrices, volumes)
		if len(obv) > 0 {
			indicators.OBV = obv[len(obv)-1]
		}
	}

	// 计算MFI - 需要至少15个数据点
	if len(closePrices) >= 15 {
		mfi := ta.MFI(highPrices, lowPrices, closePrices, volumes, 14)
		if len(mfi) > 0 {
			indicators.MFI = mfi[len(mfi)-1]
		}
	}

	return indicators, nil
}

// AnalyzeMarket 市场分析
func (s *TechnicalAnalysisService) AnalyzeMarket(symbol string, klineData []models.KlineData, indicators *models.TechnicalIndicators) (*models.MarketAnalysis, error) {
	if len(klineData) == 0 {
		return nil, fmt.Errorf("k线数据为空")
	}

	latestKline := klineData[len(klineData)-1]

	analysis := &models.MarketAnalysis{
		Symbol:       symbol,
		Timestamp:    latestKline.OpenTime.UnixMilli(),
		CurrentPrice: latestKline.ClosePrice,
	}

	// 以时间为准计算过去24小时的价格变化与成交量（自适应任意interval）
	priceChange24h, priceChangePct, volume24h := s.calculate24hMetrics(klineData)
	analysis.PriceChange24h = priceChange24h
	analysis.PriceChangePercent = priceChangePct
	analysis.Volume24h = volume24h

	// 结合多信号评估趋势与强度
	trend, strength := s.evaluateTrendAndStrength(klineData, indicators)
	analysis.Trend = trend
	analysis.Strength = strength

	// 支撑阻力位
	support, resistance := s.calculateSupportResistance(klineData)
	analysis.SupportLevel, analysis.ResistanceLevel = support, resistance

	// 市场状态（Regime）
	analysis.MarketRegime = s.determineMarketRegime(indicators, klineData)

	// 风险评估
	analysis.RiskLevel = s.assessRisk(indicators, analysis.Strength, analysis.CurrentPrice, support, resistance)

	return analysis, nil
}

// analyzeTrend 分析趋势
func (s *TechnicalAnalysisService) analyzeTrend(indicators *models.TechnicalIndicators) string {
	bullishSignals := 0
	bearishSignals := 0

	// MA趋势 - 检查指标是否存在且有效
	if indicators.MA5 > 0 && indicators.MA20 > 0 && indicators.MA50 > 0 {
		if indicators.MA5 > indicators.MA20 && indicators.MA20 > indicators.MA50 {
			bullishSignals++
		} else if indicators.MA5 < indicators.MA20 && indicators.MA20 < indicators.MA50 {
			bearishSignals++
		}
	}

	// MACD趋势 - 检查MACD指标是否计算
	if indicators.MACD != 0 || indicators.MACDSignal != 0 {
		if indicators.MACD > indicators.MACDSignal && indicators.MACDHist > 0 {
			bullishSignals++
		} else if indicators.MACD < indicators.MACDSignal && indicators.MACDHist < 0 {
			bearishSignals++
		}
	}

	// RSI趋势 - 检查RSI是否有效
	if indicators.RSI > 0 && indicators.RSI <= 100 {
		if indicators.RSI > 50 && indicators.RSI < 70 {
			bullishSignals++
		} else if indicators.RSI < 50 && indicators.RSI > 30 {
			bearishSignals++
		}
	}

	if bullishSignals > bearishSignals {
		return "up"
	} else if bearishSignals > bullishSignals {
		return "down"
	}
	return "sideways"
}

// calculateTrendStrength 计算趋势强度
func (s *TechnicalAnalysisService) calculateTrendStrength(indicators *models.TechnicalIndicators) float64 {
	strength := 0.0

	// RSI强度 - 检查RSI是否有效
	if indicators.RSI > 0 && indicators.RSI <= 100 {
		if indicators.RSI > 70 {
			strength += 3.0
		} else if indicators.RSI > 60 {
			strength += 2.0
		} else if indicators.RSI > 50 {
			strength += 1.0
		} else if indicators.RSI < 30 {
			strength += 3.0
		} else if indicators.RSI < 40 {
			strength += 2.0
		} else if indicators.RSI < 50 {
			strength += 1.0
		}
	}

	// MACD强度 - 检查MACD是否计算
	if indicators.MACDHist != 0 {
		if math.Abs(indicators.MACDHist) > 0.1 {
			strength += 2.0
		} else if math.Abs(indicators.MACDHist) > 0.05 {
			strength += 1.0
		}
	}

	// ADX强度 - 检查ADX是否有效
	if indicators.ADX > 0 {
		if indicators.ADX > 50 {
			strength += 3.0
		} else if indicators.ADX > 25 {
			strength += 2.0
		} else if indicators.ADX > 20 {
			strength += 1.0
		}
	}

	// 标准化到0-10
	return math.Min(strength, 10.0)
}

// calculate24hMetrics 基于时间窗口计算过去24小时的价格变化与成交量
func (s *TechnicalAnalysisService) calculate24hMetrics(klineData []models.KlineData) (float64, float64, float64) {
	if len(klineData) == 0 {
		return 0, 0, 0
	}

	latest := klineData[len(klineData)-1]
	cutoff := latest.CloseTime.Add(-24 * time.Hour)

	// 计算基准价格（尽量取<= cutoff的最后一根K线）
	baselineIdx := -1
	for i := 0; i < len(klineData); i++ {
		if !klineData[i].CloseTime.After(cutoff) {
			baselineIdx = i
		} else {
			break
		}
	}
	var basePrice float64
	if baselineIdx >= 0 {
		basePrice = klineData[baselineIdx].ClosePrice
	} else {
		basePrice = klineData[0].ClosePrice
	}

	// 累计24小时成交量（CloseTime >= cutoff）
	var volume24h float64
	for i := 0; i < len(klineData); i++ {
		if !klineData[i].CloseTime.Before(cutoff) {
			volume24h += klineData[i].Volume
		}
	}

	priceChange := latest.ClosePrice - basePrice
	var pct float64
	if basePrice != 0 {
		pct = (priceChange / basePrice) * 100
	}
	return priceChange, pct, volume24h
}

// evaluateTrendAndStrength 多信号加权评估趋势方向与强度
func (s *TechnicalAnalysisService) evaluateTrendAndStrength(klineData []models.KlineData, indicators *models.TechnicalIndicators) (string, float64) {
	if indicators == nil {
		return "sideways", 0
	}

	// 准备价格序列
	closePrices := make([]float64, len(klineData))
	highPrices := make([]float64, len(klineData))
	lowPrices := make([]float64, len(klineData))
	for i, k := range klineData {
		closePrices[i] = k.ClosePrice
		highPrices[i] = k.HighPrice
		lowPrices[i] = k.LowPrice
	}

	bull := 0.0
	bear := 0.0

	// 1) 均线多头/空头排列
	if indicators.MA5 > 0 && indicators.MA20 > 0 && indicators.MA50 > 0 {
		if indicators.MA5 > indicators.MA20 && indicators.MA20 > indicators.MA50 {
			bull += 2
		} else if indicators.MA5 < indicators.MA20 && indicators.MA20 < indicators.MA50 {
			bear += 2
		}
	}

	// 2) MA20斜率
	if len(closePrices) >= 23 { // 20期均线，至少再留3期用于斜率
		ma20Series := ta.SMA(closePrices, 20)
		if len(ma20Series) >= 4 {
			delta := ma20Series[len(ma20Series)-1] - ma20Series[len(ma20Series)-4]
			if delta > 0 {
				bull += 1
			} else if delta < 0 {
				bear += 1
			}
		}
	}

	// 3) MACD
	if indicators.MACD != 0 || indicators.MACDSignal != 0 || indicators.MACDHist != 0 {
		if indicators.MACD > indicators.MACDSignal && indicators.MACDHist > 0 {
			bull += 1
		} else if indicators.MACD < indicators.MACDSignal && indicators.MACDHist < 0 {
			bear += 1
		}
	}

	// 4) RSI
	if indicators.RSI > 0 && indicators.RSI <= 100 {
		if indicators.RSI >= 60 {
			bull += 0.7
		} else if indicators.RSI <= 40 {
			bear += 0.7
		}
	}

	// 5) 布林带位置
	if indicators.BBMiddle > 0 {
		lastPrice := closePrices[len(closePrices)-1]
		if lastPrice > indicators.BBMiddle {
			bull += 0.5
		} else if lastPrice < indicators.BBMiddle {
			bear += 0.5
		}
	}

	// 6) SuperTrend
	trendLine := ta.SuperTrend(highPrices, lowPrices, closePrices, 10, 3.0)
	if len(trendLine) > 0 {
		lastST := trendLine[len(trendLine)-1]
		lastPrice := closePrices[len(closePrices)-1]
		if lastPrice > lastST {
			bull += 1
		} else if lastPrice < lastST {
			bear += 1
		}
	}

	// 趋势方向
	diff := bull - bear
	trend := "sideways"
	if diff > 1.0 {
		trend = "up"
	} else if diff < -1.0 {
		trend = "down"
	}

	// 强度评分（0-10）
	strength := 0.0
	absDiff := math.Abs(diff)
	if absDiff > 0 {
		if absDiff > 3 {
			strength += 3
		} else if absDiff > 2 {
			strength += 2
		} else if absDiff > 1 {
			strength += 1
		}
	}

	// ADX增强
	if indicators.ADX > 0 {
		if indicators.ADX > 50 {
			strength += 3
		} else if indicators.ADX > 25 {
			strength += 2
		} else if indicators.ADX > 20 {
			strength += 1
		}
	}

	// MACD动量增强
	absHist := math.Abs(indicators.MACDHist)
	if absHist > 0.2 {
		strength += 2
	} else if absHist > 0.1 {
		strength += 1
	}

	// MA20斜率增强（按价格比例）
	if len(closePrices) >= 23 {
		ma20Series := ta.SMA(closePrices, 20)
		if len(ma20Series) >= 4 {
			delta := ma20Series[len(ma20Series)-1] - ma20Series[len(ma20Series)-4]
			price := closePrices[len(closePrices)-1]
			if price > 0 {
				pct := math.Abs(delta / price)
				if pct > 0.01 {
					strength += 2
				} else if pct > 0.005 {
					strength += 1
				}
			}
		}
	}

	if strength > 10 {
		strength = 10
	}
	return trend, strength
}

// determineMarketRegime 判定市场状态（趋势/震荡/不确定）
func (s *TechnicalAnalysisService) determineMarketRegime(indicators *models.TechnicalIndicators, klineData []models.KlineData) string {
	if indicators == nil {
		return "Uncertain"
	}
	bw := 0.0
	if indicators.BBMiddle > 0 {
		bw = (indicators.BBUpper - indicators.BBLower) / indicators.BBMiddle
	}
	if indicators.ADX >= 25 && bw > 0.04 {
		return "Trending"
	}
	if indicators.ADX < 20 && bw > 0 && bw < 0.03 {
		return "Ranging"
	}
	return "Uncertain"
}

// calculateSupportResistance 计算支撑阻力位
func (s *TechnicalAnalysisService) calculateSupportResistance(klineData []models.KlineData) (float64, float64) {
	if len(klineData) < 10 {
		return 0, 0
	}

	// 选择计算窗口
	window := 200
	if len(klineData) < window {
		window = len(klineData)
	}
	recent := klineData[len(klineData)-window:]

	highs := make([]float64, len(recent))
	lows := make([]float64, len(recent))
	closes := make([]float64, len(recent))
	for i, k := range recent {
		highs[i] = k.HighPrice
		lows[i] = k.LowPrice
		closes[i] = k.ClosePrice
	}

	currentPrice := closes[len(closes)-1]

	// 计算ATR用于聚类容差
	atrSeries := ta.ATR(highs, lows, closes, 14)
	atr := 0.0
	if len(atrSeries) > 0 {
		atr = atrSeries[len(atrSeries)-1]
	}

	// 寻找枢轴点（Pivot High/Low）
	pivotLeft, pivotRight := 2, 2
	var pivotHighs, pivotLows []float64
	for i := pivotLeft; i < len(highs)-pivotRight; i++ {
		isHigh := true
		for l := 1; l <= pivotLeft; l++ {
			if !(highs[i] > highs[i-l]) { // 严格大于左侧
				isHigh = false
				break
			}
		}
		if isHigh {
			for r := 1; r <= pivotRight; r++ {
				if !(highs[i] >= highs[i+r]) { // 大于等于右侧，避免漏掉平台
					isHigh = false
					break
				}
			}
		}
		if isHigh {
			pivotHighs = append(pivotHighs, highs[i])
		}

		isLow := true
		for l := 1; l <= pivotLeft; l++ {
			if !(lows[i] < lows[i-l]) {
				isLow = false
				break
			}
		}
		if isLow {
			for r := 1; r <= pivotRight; r++ {
				if !(lows[i] <= lows[i+r]) {
					isLow = false
					break
				}
			}
		}
		if isLow {
			pivotLows = append(pivotLows, lows[i])
		}
	}

	// 聚类：将接近的价格水平合并
	cluster := func(levels []float64, tolerance float64) []float64 {
		if len(levels) == 0 {
			return levels
		}
		sort.Float64s(levels)
		var clustered []float64
		start := levels[0]
		sum := levels[0]
		count := 1
		for i := 1; i < len(levels); i++ {
			if math.Abs(levels[i]-start) <= tolerance {
				sum += levels[i]
				count++
			} else {
				clustered = append(clustered, sum/float64(count))
				start = levels[i]
				sum = levels[i]
				count = 1
			}
		}
		clustered = append(clustered, sum/float64(count))
		return clustered
	}

	// 容差：ATR的一部分与价格比例二者取较大，保证在不同币价下表现稳定
	// 例如 0.5 * ATR 或 0.25% 价格
	priceTol := currentPrice * 0.0025
	atrTol := atr * 0.5
	tolerance := math.Max(priceTol, atrTol)
	if tolerance <= 0 {
		// 回退容差：价格的0.3%
		tolerance = currentPrice * 0.003
	}

	ch := cluster(pivotHighs, tolerance)
	cl := cluster(pivotLows, tolerance)

	// 从聚类后的水平中，选取最近的上方阻力和下方支撑
	resistance := 0.0
	support := 0.0
	minAbove := math.MaxFloat64
	maxBelow := -math.MaxFloat64

	for _, lv := range ch {
		if lv > currentPrice && lv-currentPrice < minAbove {
			minAbove = lv - currentPrice
			resistance = lv
		}
	}
	for _, lv := range cl {
		if lv < currentPrice && currentPrice-lv > maxBelow {
			maxBelow = currentPrice - lv
			support = lv
		}
	}

	// 回退：若未找到，则使用窗口内极值
	if resistance == 0 {
		mx := highs[0]
		for _, h := range highs {
			if h > mx {
				mx = h
			}
		}
		resistance = mx
	}
	if support == 0 {
		mn := lows[0]
		for _, l := range lows {
			if l < mn {
				mn = l
			}
		}
		support = mn
	}

	// 最终保证支撑<阻力
	if support > resistance {
		// 简单交换以避免异常
		support, resistance = resistance, support
	}

	return support, resistance
}

// findLocalMaxima 寻找局部最大值
func (s *TechnicalAnalysisService) findLocalMaxima(data []float64) float64 {
	if len(data) < 3 {
		return 0
	}

	var maxima []float64
	for i := 1; i < len(data)-1; i++ {
		if data[i] > data[i-1] && data[i] > data[i+1] {
			maxima = append(maxima, data[i])
		}
	}

	if len(maxima) == 0 {
		// 如果没有局部最大值，返回整体最大值
		max := data[0]
		for _, v := range data {
			if v > max {
				max = v
			}
		}
		return max
	}

	// 返回最近的局部最大值
	return maxima[len(maxima)-1]
}

// findLocalMinima 寻找局部最小值
func (s *TechnicalAnalysisService) findLocalMinima(data []float64) float64 {
	if len(data) < 3 {
		return 0
	}

	var minima []float64
	for i := 1; i < len(data)-1; i++ {
		if data[i] < data[i-1] && data[i] < data[i+1] {
			minima = append(minima, data[i])
		}
	}

	if len(minima) == 0 {
		// 如果没有局部最小值，返回整体最小值
		min := data[0]
		for _, v := range data {
			if v < min {
				min = v
			}
		}
		return min
	}

	// 返回最近的局部最小值
	return minima[len(minima)-1]
}

// assessRisk 风险评估
func (s *TechnicalAnalysisService) assessRisk(indicators *models.TechnicalIndicators, strength float64, currentPrice, support, resistance float64) string {
	riskScore := 0.0

	// RSI风险 - 检查RSI是否有效
	if indicators.RSI > 0 && indicators.RSI <= 100 {
		if indicators.RSI > 80 || indicators.RSI < 20 {
			riskScore += 3.0
		} else if indicators.RSI > 70 || indicators.RSI < 30 {
			riskScore += 2.0
		}
	}

	// 波动性风险 (基于ATR/价格)
	if indicators.ATR > 0 && currentPrice > 0 {
		atrPct := indicators.ATR / currentPrice * 100
		if atrPct > 5 {
			riskScore += 2.0
		} else if atrPct > 2.5 {
			riskScore += 1.5
		} else if atrPct > 1.5 {
			riskScore += 1.0
		}
	}

	// 趋势强度风险
	if strength > 8.0 {
		riskScore += 2.0 // 过强趋势可能反转
	} else if strength < 2.0 {
		riskScore += 1.0 // 趋势不明确
	}

	// 价格与支撑/阻力的相对位置（越接近边界，风险越高）
	if indicators.ATR > 0 && currentPrice > 0 && (support > 0 || resistance > 0) {
		dist := math.MaxFloat64
		if support > 0 {
			dist = math.Min(dist, math.Abs(currentPrice-support))
		}
		if resistance > 0 {
			dist = math.Min(dist, math.Abs(resistance-currentPrice))
		}
		if dist < math.MaxFloat64 {
			ratio := dist / indicators.ATR
			if ratio < 0.8 {
				riskScore += 2.0
			} else if ratio < 1.5 {
				riskScore += 1.0
			}
		}
	}

	if riskScore >= 5.0 {
		return "high"
	} else if riskScore >= 3.0 {
		return "medium"
	}
	return "low"
}
