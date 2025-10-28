package service

import (
	"github.com/dushixiang/prism/pkg/exchange"
	"github.com/dushixiang/prism/pkg/ta"
)

// IndicatorService 技术指标计算服务
type IndicatorService struct{}

// NewIndicatorService 创建技术指标服务
func NewIndicatorService() *IndicatorService {
	return &IndicatorService{}
}

// TimeframeIndicators 单个时间框架的指标
type TimeframeIndicators struct {
	Timeframe  string  `json:"timeframe"` // 1m/3m/5m/15m/30m/1h/4h
	Price      float64 `json:"price"`
	EMA20      float64 `json:"ema20"`
	EMA50      float64 `json:"ema50"`
	MACD       float64 `json:"macd"`
	MACDSignal float64 `json:"macd_signal"`
	MACDHist   float64 `json:"macd_hist"`
	RSI7       float64 `json:"rsi7"`
	RSI14      float64 `json:"rsi14"`
	ATR3       float64 `json:"atr3"`
	ATR14      float64 `json:"atr14"`
	Volume     float64 `json:"volume"`
	AvgVolume  float64 `json:"avg_volume"`
}

// TimeSeriesData 时序数据（最近10个数据点）
type TimeSeriesData struct {
	MidPrices   []float64 `json:"mid_prices"`
	EMA20Series []float64 `json:"ema20_series"`
	MACDSeries  []float64 `json:"macd_series"`
	RSI7Series  []float64 `json:"rsi7_series"`
	RSI14Series []float64 `json:"rsi14_series"`
}

// CalculateIndicators 计算所有技术指标
func (s *IndicatorService) CalculateIndicators(klines []*exchange.Kline) *TimeframeIndicators {
	if len(klines) < 50 {
		return nil
	}

	// 提取价格数据
	closes := make([]float64, len(klines))
	highs := make([]float64, len(klines))
	lows := make([]float64, len(klines))
	volumes := make([]float64, len(klines))

	for i, k := range klines {
		closes[i] = k.Close
		highs[i] = k.High
		lows[i] = k.Low
		volumes[i] = k.Volume
	}

	// 计算EMA
	ema20 := ta.EMA(closes, 20)
	ema50 := ta.EMA(closes, 50)

	// 计算MACD
	macd, signal, hist := ta.MACD(closes, 12, 26, 9)

	// 计算RSI
	rsi7 := ta.RSI(closes, 7)
	rsi14 := ta.RSI(closes, 14)

	// 计算ATR
	atr3 := ta.ATR(highs, lows, closes, 3)
	atr14 := ta.ATR(highs, lows, closes, 14)

	// 计算平均成交量
	avgVolume := 0.0
	for _, v := range volumes {
		avgVolume += v
	}
	avgVolume /= float64(len(volumes))

	// 获取最新值
	lastIdx := len(closes) - 1

	return &TimeframeIndicators{
		Price:      closes[lastIdx],
		EMA20:      ta.Last(ema20, 0),
		EMA50:      ta.Last(ema50, 0),
		MACD:       ta.Last(macd, 0),
		MACDSignal: ta.Last(signal, 0),
		MACDHist:   ta.Last(hist, 0),
		RSI7:       ta.Last(rsi7, 0),
		RSI14:      ta.Last(rsi14, 0),
		ATR3:       ta.Last(atr3, 0),
		ATR14:      ta.Last(atr14, 0),
		Volume:     volumes[lastIdx],
		AvgVolume:  avgVolume,
	}
}

// CalculateTimeSeries 计算时序数据（日内3分钟级别，最近10个数据点）
func (s *IndicatorService) CalculateTimeSeries(klines []*exchange.Kline) *TimeSeriesData {
	if len(klines) < 50 {
		return nil
	}

	// 提取收盘价
	closes := make([]float64, len(klines))
	highs := make([]float64, len(klines))
	lows := make([]float64, len(klines))

	for i, k := range klines {
		closes[i] = k.Close
		highs[i] = k.High
		lows[i] = k.Low
	}

	// 计算指标序列（使用全部数据）
	ema20Series := ta.EMA(closes, 20)
	macdSeries, _, _ := ta.MACD(closes, 12, 26, 9)
	rsi7Series := ta.RSI(closes, 7)
	rsi14Series := ta.RSI(closes, 14)

	// 计算中间价
	midPrices := make([]float64, len(klines))
	for i := range klines {
		midPrices[i] = (highs[i] + lows[i]) / 2
	}

	// 只返回最近10个数据点
	size := 10
	if len(closes) < size {
		size = len(closes)
	}

	return &TimeSeriesData{
		MidPrices:   ta.LastValues(midPrices, size),
		EMA20Series: ta.LastValues(ema20Series, size),
		MACDSeries:  ta.LastValues(macdSeries, size),
		RSI7Series:  ta.LastValues(rsi7Series, size),
		RSI14Series: ta.LastValues(rsi14Series, size),
	}
}

// ValidateIndicators 验证指标数据质量
func (s *IndicatorService) ValidateIndicators(indicators *TimeframeIndicators) []string {
	issues := make([]string, 0)

	// 验证价格
	if indicators.Price <= 0 {
		issues = append(issues, "invalid price")
	}

	// 验证EMA
	if indicators.EMA20 <= 0 {
		issues = append(issues, "invalid EMA20")
	}
	if indicators.EMA50 <= 0 {
		issues = append(issues, "invalid EMA50")
	}

	// 验证RSI
	if indicators.RSI14 < 0 || indicators.RSI14 > 100 {
		issues = append(issues, "RSI14 out of range")
	}

	// 验证成交量
	if indicators.Volume < 0 {
		issues = append(issues, "negative volume")
	}

	return issues
}

// DetectMultiTimeframeConfluence 检测多时间框架共振
func (s *IndicatorService) DetectMultiTimeframeConfluence(indicators map[string]*TimeframeIndicators) (string, int) {
	// 检查各时间框架的趋势方向
	bullishCount := 0
	bearishCount := 0

	for _, ind := range indicators {
		isBullish := false
		isBearish := false

		// EMA趋势
		if ind.EMA20 > ind.EMA50 {
			isBullish = true
		} else {
			isBearish = true
		}

		// MACD确认
		if ind.MACD > 0 {
			if isBullish {
				bullishCount++
			}
		} else {
			if isBearish {
				bearishCount++
			}
		}
	}

	if bullishCount >= 3 {
		return "bullish", bullishCount
	} else if bearishCount >= 3 {
		return "bearish", bearishCount
	}

	return "neutral", 0
}
