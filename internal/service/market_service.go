package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dushixiang/prism/internal/models"
	"github.com/dushixiang/prism/internal/repo"
	"github.com/dushixiang/prism/pkg/exchange"
	"github.com/go-orz/orz"
	"github.com/oklog/ulid/v2"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// MarketService 市场数据收集服务
type MarketService struct {
	logger *zap.Logger

	*orz.Service
	*repo.TechnicalIndicatorRepo

	binanceClient    *exchange.BinanceClient
	indicatorService *IndicatorService
}

// NewMarketService 创建市场数据服务
func NewMarketService(db *gorm.DB, binanceClient *exchange.BinanceClient,
	indicatorService *IndicatorService, logger *zap.Logger) *MarketService {
	return &MarketService{
		logger:                 logger,
		Service:                orz.NewService(db),
		TechnicalIndicatorRepo: repo.NewTechnicalIndicatorRepo(db),
		binanceClient:          binanceClient,
		indicatorService:       indicatorService,
	}
}

// MarketData 市场数据
type MarketData struct {
	Symbol         string                          `json:"symbol"`
	CurrentPrice   float64                         `json:"current_price"`
	FundingRate    float64                         `json:"funding_rate"`
	Timeframes     map[string]*TimeframeIndicators `json:"timeframes"`
	IntradaySeries *TimeSeriesData                 `json:"intraday_series"`  // 日内3分钟序列
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
		{"1m", "1m", 60},
		{"3m", "3m", 60},
		{"5m", "5m", 100},
		{"15m", "15m", 96},
		{"30m", "30m", 90},
		{"1h", "1h", 120},
	}

	marketData := &MarketData{
		Symbol:     symbol,
		Timeframes: make(map[string]*TimeframeIndicators),
	}

	// 获取各时间框架的K线数据并计算指标
	var klines1h []*exchange.Kline
	var klines3m []*exchange.Kline

	for _, tf := range timeframes {
		klines, err := s.binanceClient.GetKlines(ctx, symbol, tf.interval, tf.limit)
		if err != nil {
			s.logger.Error("failed to get klines",
				zap.String("symbol", symbol),
				zap.String("timeframe", tf.name),
				zap.Error(err))
			continue
		}

		// 保存特定时间框架的数据用于后续处理
		if tf.name == "1h" {
			klines1h = klines
		} else if tf.name == "3m" {
			klines3m = klines
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

			// 保存到数据库
			if err := s.saveTechnicalIndicator(ctx, symbol, tf.name, indicators, nil); err != nil {
				s.logger.Error("failed to save technical indicator",
					zap.String("symbol", symbol),
					zap.String("timeframe", tf.name),
					zap.Error(err))
			}
		}
	}

	// 获取当前价格
	if len(marketData.Timeframes) > 0 {
		// 从最短时间框架获取最新价格
		if ind, ok := marketData.Timeframes["1m"]; ok {
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

	// 计算日内时序数据（使用3分钟K线）
	if len(klines3m) > 0 {
		marketData.IntradaySeries = s.indicatorService.CalculateTimeSeries(klines3m)
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

	// TODO: 计算MACD和RSI14序列（需要实现完整的序列计算）
	// 这里暂时留空，因为需要对整个K线序列逐步计算
	// 而不是只获取最后一个值

	context := &LongerTermContext{
		MACDSeries:  []float64{}, // TODO: 填充实际数据
		RSI14Series: []float64{}, // TODO: 填充实际数据
	}

	// EMA20 vs EMA50
	if indicators.EMA20 > indicators.EMA50 {
		context.EMA20vsEMA50 = "above"
	} else {
		context.EMA20vsEMA50 = "below"
	}

	// ATR3 vs ATR14
	if indicators.ATR3 > indicators.ATR14 {
		context.ATR3vsATR14 = "higher"
	} else {
		context.ATR3vsATR14 = "lower"
	}

	// Volume vs AvgVolume
	if indicators.Volume > indicators.AvgVolume {
		context.VolumeVsAvg = "above"
	} else {
		context.VolumeVsAvg = "below"
	}

	return context
}

// saveTechnicalIndicator 保存技术指标到数据库
func (s *MarketService) saveTechnicalIndicator(ctx context.Context, symbol string, timeframe string,
	indicators *TimeframeIndicators, timeSeries *TimeSeriesData) error {

	var priceSeriesJSON, ema20SeriesJSON, macdSeriesJSON, rsi7SeriesJSON, rsi14SeriesJSON datatypes.JSON

	if timeSeries != nil {
		priceSeriesJSON, _ = json.Marshal(timeSeries.MidPrices)
		ema20SeriesJSON, _ = json.Marshal(timeSeries.EMA20Series)
		macdSeriesJSON, _ = json.Marshal(timeSeries.MACDSeries)
		rsi7SeriesJSON, _ = json.Marshal(timeSeries.RSI7Series)
		rsi14SeriesJSON, _ = json.Marshal(timeSeries.RSI14Series)
	}

	indicator := &models.TechnicalIndicator{
		ID:           ulid.Make().String(),
		Symbol:       symbol,
		Timeframe:    timeframe,
		Price:        indicators.Price,
		EMA20:        indicators.EMA20,
		EMA50:        indicators.EMA50,
		MACD:         indicators.MACD,
		MACDSignal:   indicators.MACDSignal,
		MACDHist:     indicators.MACDHist,
		RSI7:         indicators.RSI7,
		RSI14:        indicators.RSI14,
		ATR3:         indicators.ATR3,
		ATR14:        indicators.ATR14,
		Volume:       indicators.Volume,
		AvgVolume:    indicators.AvgVolume,
		PriceSeries:  priceSeriesJSON,
		EMA20Series:  ema20SeriesJSON,
		MACDSeries:   macdSeriesJSON,
		RSI7Series:   rsi7SeriesJSON,
		RSI14Series:  rsi14SeriesJSON,
		CalculatedAt: time.Now(),
	}

	return s.TechnicalIndicatorRepo.Create(ctx, indicator)
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

// GetLatestIndicators 获取最新的技术指标
func (s *MarketService) GetLatestIndicators(ctx context.Context, symbol string, timeframe string) (*models.TechnicalIndicator, error) {
	indicator, err := s.TechnicalIndicatorRepo.FindLatestBySymbolAndTimeframe(ctx, symbol, timeframe)
	if err != nil {
		return nil, err
	}

	return &indicator, nil
}
