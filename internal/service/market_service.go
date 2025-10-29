package service

import (
	"context"
	"fmt"

	"github.com/dushixiang/prism/pkg/exchange"
	"github.com/dushixiang/prism/pkg/ta"
	"github.com/go-orz/orz"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// MarketService 市场数据收集服务
type MarketService struct {
	logger *zap.Logger

	*orz.Service

	binanceClient    *exchange.BinanceClient
	indicatorService *IndicatorService
}

// NewMarketService 创建市场数据服务
func NewMarketService(db *gorm.DB, binanceClient *exchange.BinanceClient,
	indicatorService *IndicatorService, logger *zap.Logger) *MarketService {
	return &MarketService{
		logger:           logger,
		Service:          orz.NewService(db),
		binanceClient:    binanceClient,
		indicatorService: indicatorService,
	}
}

// MarketData 市场数据
type MarketData struct {
	Symbol         string                          `json:"symbol"`
	CurrentPrice   float64                         `json:"current_price"`
	FundingRate    float64                         `json:"funding_rate"`
	Timeframes     map[string]*TimeframeIndicators `json:"timeframes"`
	IntradaySeries *TimeSeriesData                 `json:"intraday_series"`  // 日内5分钟序列
	LongerTermData *LongerTermContext              `json:"longer_term_data"` // 1小时更长期上下文
}

// LongerTermContext 更长期上下文（1小时级别）
type LongerTermContext struct {
	EMA20vsEMA50 string    `json:"ema20_vs_ema50"` // "above" or "below"
	ATR3vsATR14  string    `json:"atr3_vs_atr14"`  // "higher" or "lower"
	VolumeVsAvg  string    `json:"volume_vs_avg"`  // "above" or "below"
	MACDSeries   []float64 `json:"macd_series"`    // 最近10个MACD值
	RSI14Series  []float64 `json:"rsi14_series"`   // 最近10个RSI14值
}

// CollectMarketData 收集指定币种的市场数据（所有时间框架）
func (s *MarketService) CollectMarketData(ctx context.Context, symbol string) (*MarketData, error) {
	s.logger.Info("collecting market data", zap.String("symbol", symbol))

	// 定义需要获取的时间框架
	timeframes := []struct {
		name     string
		interval string
		limit    int
	}{
		{"5m", "5m", 120},
		{"15m", "15m", 96},
		{"30m", "30m", 90},
		{"1h", "1h", 120},
		{"4h", "4h", 180},
	}

	marketData := &MarketData{
		Symbol:     symbol,
		Timeframes: make(map[string]*TimeframeIndicators),
	}

	// 获取各时间框架的K线数据并计算指标
	var shortestFrame string
	var klines1h []*exchange.Kline
	var klines5m []*exchange.Kline

	for _, tf := range timeframes {
		klines, err := s.binanceClient.GetKlines(ctx, symbol, tf.interval, tf.limit)
		if err != nil {
			s.logger.Error("failed to get klines",
				zap.String("symbol", symbol),
				zap.String("timeframe", tf.name),
				zap.Error(err))
			continue
		}

		if shortestFrame == "" {
			shortestFrame = tf.name
		}

		// 保存特定时间框架的数据用于后续处理
		if tf.name == "1h" {
			klines1h = klines
		} else if tf.name == "5m" {
			klines5m = klines
		}

		// 计算技术指标
		indicators := s.indicatorService.CalculateIndicators(klines)
		if indicators != nil {
			indicators.Timeframe = tf.name
			marketData.Timeframes[tf.name] = indicators

			// 验证数据质量
			issues := s.indicatorService.ValidateIndicators(indicators)
			if len(issues) > 0 {
				s.logger.Warn("data quality issues",
					zap.String("symbol", symbol),
					zap.String("timeframe", tf.name),
					zap.Strings("issues", issues))
			}
		}
	}

	// 获取当前价格
	if len(marketData.Timeframes) > 0 && shortestFrame != "" {
		if ind, ok := marketData.Timeframes[shortestFrame]; ok {
			marketData.CurrentPrice = ind.Price
		}
	}

	// 获取资金费率
	fundingRate, err := s.binanceClient.GetFundingRate(ctx, symbol)
	if err != nil {
		s.logger.Warn("failed to get funding rate", zap.String("symbol", symbol), zap.Error(err))
	} else {
		marketData.FundingRate = fundingRate
	}

	// 计算日内时序数据（使用5分钟K线）
	if len(klines5m) > 0 {
		marketData.IntradaySeries = s.indicatorService.CalculateTimeSeries(klines5m)
	}

	// 计算更长期上下文（使用1小时K线）
	if len(klines1h) > 0 {
		marketData.LongerTermData = s.calculateLongerTermContext(klines1h)
	}

	return marketData, nil
}

// calculateLongerTermContext 计算更长期上下文
func (s *MarketService) calculateLongerTermContext(klines []*exchange.Kline) *LongerTermContext {
	if len(klines) < 50 {
		return nil
	}

	indicators := s.indicatorService.CalculateIndicators(klines)
	if indicators == nil {
		return nil
	}

	// 提取收盘价
	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}

	// 计算MACD与RSI序列，返回最近10个数据点
	macdSeries, _, _ := ta.MACD(closes, 12, 26, 9)
	rsi14Series := ta.RSI(closes, 14)

	seriesSize := 10
	if len(macdSeries) < seriesSize {
		seriesSize = len(macdSeries)
	}
	lastMACD := ta.LastValues(macdSeries, seriesSize)
	lastRSI14 := ta.LastValues(rsi14Series, seriesSize)

	longerTermCtx := &LongerTermContext{
		MACDSeries:  lastMACD,
		RSI14Series: lastRSI14,
	}

	// EMA20 vs EMA50
	if indicators.EMA20 > indicators.EMA50 {
		longerTermCtx.EMA20vsEMA50 = "above"
	} else {
		longerTermCtx.EMA20vsEMA50 = "below"
	}

	// ATR3 vs ATR14
	if indicators.ATR3 > indicators.ATR14 {
		longerTermCtx.ATR3vsATR14 = "higher"
	} else {
		longerTermCtx.ATR3vsATR14 = "lower"
	}

	// Volume vs AvgVolume
	if indicators.Volume > indicators.AvgVolume {
		longerTermCtx.VolumeVsAvg = "above"
	} else {
		longerTermCtx.VolumeVsAvg = "below"
	}

	return longerTermCtx
}

// CollectAllSymbols 收集所有交易对的市场数据
func (s *MarketService) CollectAllSymbols(ctx context.Context, symbols []string) (map[string]*MarketData, error) {
	result := make(map[string]*MarketData)

	for _, symbol := range symbols {
		data, err := s.CollectMarketData(ctx, symbol)
		if err != nil {
			s.logger.Error("failed to collect market data",
				zap.String("symbol", symbol),
				zap.Error(err))
			continue
		}
		result[symbol] = data
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("failed to collect market data for any symbol")
	}

	return result, nil
}
