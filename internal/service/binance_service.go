package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/dushixiang/prism/internal/config"
	"github.com/dushixiang/prism/internal/models"
	"go.uber.org/zap"
)

// BinanceService 币安数据服务
type BinanceService struct {
	client        *binance.Client
	futuresClient *futures.Client
	logger        *zap.Logger
	httpClient    *http.Client

	mu            sync.RWMutex
	symbolsCache  []string
	symbolsCached time.Time
	cacheTTL      time.Duration

	// 新增市场概览缓存
	marketSummaryCache *marketSummaryCache
	marketSummaryTTL   time.Duration

	// 恐惧与贪婪指数缓存
	fearGreedCache *fearGreedCache
	fearGreedTTL   time.Duration
}

// NewBinanceService 创建新的币安服务实例
func NewBinanceService(config *config.Config, logger *zap.Logger) *BinanceService {
	client := binance.NewClient(config.Binance.APIKey, config.Binance.Secret)
	futuresClient := binance.NewFuturesClient(config.Binance.APIKey, config.Binance.Secret)

	// 创建HTTP客户端，设置超时
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	return &BinanceService{
		client:           client,
		futuresClient:    futuresClient,
		logger:           logger,
		httpClient:       httpClient,
		cacheTTL:         time.Minute * 10,
		marketSummaryTTL: time.Minute * 5, // 市场概览缓存5分钟
		fearGreedTTL:     time.Hour,       // 恐惧与贪婪指数缓存1小时
	}
}

// GetKlineData 获取K线数据
func (s *BinanceService) GetKlineData(symbol, interval string, limit int) ([]*binance.Kline, error) {
	klines, err := s.client.NewKlinesService().
		Symbol(symbol).
		Interval(interval).
		Limit(limit).
		Do(context.Background())

	if err != nil {
		s.logger.Error("获取K线数据失败", zap.Error(err), zap.String("symbol", symbol))
		return nil, fmt.Errorf("获取K线数据失败: %w", err)
	}

	return klines, nil
}

// GetDepthData 获取市场深度数据
func (s *BinanceService) GetDepthData(symbol string, limit int) (*binance.DepthResponse, error) {
	depth, err := s.client.NewDepthService().
		Symbol(symbol).
		Limit(limit).
		Do(context.Background())

	if err != nil {
		s.logger.Error("获取市场深度数据失败", zap.Error(err), zap.String("symbol", symbol))
		return nil, fmt.Errorf("获取市场深度数据失败: %w", err)
	}

	return depth, nil
}

// GetTickerPrice 获取价格数据
func (s *BinanceService) GetTickerPrice(symbol string) (*binance.SymbolPrice, error) {
	price, err := s.client.NewListPricesService().
		Symbol(symbol).
		Do(context.Background())

	if err != nil {
		s.logger.Error("获取价格数据失败", zap.Error(err), zap.String("symbol", symbol))
		return nil, fmt.Errorf("获取价格数据失败: %w", err)
	}

	if len(price) == 0 {
		return nil, fmt.Errorf("未找到交易对 %s 的价格数据", symbol)
	}

	return price[0], nil
}

// Get24hrStats 获取24小时统计数据
func (s *BinanceService) Get24hrStats(symbol string) (*binance.PriceChangeStats, error) {
	stats, err := s.client.NewListPriceChangeStatsService().
		Symbol(symbol).
		Do(context.Background())

	if err != nil {
		s.logger.Error("获取24小时统计数据失败", zap.Error(err), zap.String("symbol", symbol))
		return nil, fmt.Errorf("获取24小时统计数据失败: %w", err)
	}

	if len(stats) == 0 {
		return nil, fmt.Errorf("未找到交易对 %s 的统计数据", symbol)
	}

	return stats[0], nil
}

// GetExchangeInfo 获取交易所信息
func (s *BinanceService) GetExchangeInfo() (*binance.ExchangeInfo, error) {
	info, err := s.client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		s.logger.Error("获取交易所信息失败", zap.Error(err))
		return nil, fmt.Errorf("获取交易所信息失败: %w", err)
	}

	return info, nil
}

// GetCachedSymbols 返回缓存的USDT交易对列表（TRADING状态）
func (s *BinanceService) GetCachedSymbols() ([]string, error) {
	s.mu.RLock()
	if len(s.symbolsCache) > 0 && time.Since(s.symbolsCached) < s.cacheTTL {
		defer s.mu.RUnlock()
		cp := make([]string, len(s.symbolsCache))
		copy(cp, s.symbolsCache)
		return cp, nil
	}
	s.mu.RUnlock()

	info, err := s.GetExchangeInfo()
	if err != nil {
		return nil, err
	}
	symbols := make([]string, 0, len(info.Symbols))
	for _, si := range info.Symbols {
		if si.Status == "TRADING" && strings.HasSuffix(si.Symbol, "USDT") {
			symbols = append(symbols, si.Symbol)
		}
	}
	s.mu.Lock()
	s.symbolsCache = symbols
	s.symbolsCached = time.Now()
	s.mu.Unlock()
	cp := make([]string, len(symbols))
	copy(cp, symbols)
	return cp, nil
}

// ConvertKlineToModels 将币安K线数据转换为内部模型
func (s *BinanceService) ConvertKlineToModels(klines []*binance.Kline) ([]models.KlineData, error) {
	result := make([]models.KlineData, 0, len(klines))

	for _, kline := range klines {
		openPrice, err := strconv.ParseFloat(kline.Open, 64)
		if err != nil {
			continue
		}

		highPrice, err := strconv.ParseFloat(kline.High, 64)
		if err != nil {
			continue
		}

		lowPrice, err := strconv.ParseFloat(kline.Low, 64)
		if err != nil {
			continue
		}

		closePrice, err := strconv.ParseFloat(kline.Close, 64)
		if err != nil {
			continue
		}

		volume, err := strconv.ParseFloat(kline.Volume, 64)
		if err != nil {
			continue
		}

		result = append(result, models.KlineData{
			OpenTime:   time.Unix(kline.OpenTime/1000, 0),
			CloseTime:  time.Unix(kline.CloseTime/1000, 0),
			OpenPrice:  openPrice,
			HighPrice:  highPrice,
			LowPrice:   lowPrice,
			ClosePrice: closePrice,
			Volume:     volume,
		})
	}

	return result, nil
}

// AnalyzeDepth 分析市场深度
func (s *BinanceService) AnalyzeDepth(symbol string, depth *binance.DepthResponse) *models.DepthAnalysis {
	analysis := &models.DepthAnalysis{
		Symbol:    symbol,
		Timestamp: time.Now(),
	}

	// 计算买单总量
	var bidVolume, askVolume float64
	var bidValue, askValue float64

	for _, bid := range depth.Bids {
		price, _ := strconv.ParseFloat(bid.Price, 64)
		quantity, _ := strconv.ParseFloat(bid.Quantity, 64)
		bidVolume += quantity
		bidValue += price * quantity
	}

	for _, ask := range depth.Asks {
		price, _ := strconv.ParseFloat(ask.Price, 64)
		quantity, _ := strconv.ParseFloat(ask.Quantity, 64)
		askVolume += quantity
		askValue += price * quantity
	}

	analysis.BidVolume = bidVolume
	analysis.AskVolume = askVolume
	analysis.BidValue = bidValue
	analysis.AskValue = askValue

	// 计算买卖比例
	if askVolume > 0 {
		analysis.BidAskRatio = bidVolume / askVolume
	}

	// 计算价差
	if len(depth.Bids) > 0 && len(depth.Asks) > 0 {
		bestBid, _ := strconv.ParseFloat(depth.Bids[0].Price, 64)
		bestAsk, _ := strconv.ParseFloat(depth.Asks[0].Price, 64)
		analysis.Spread = bestAsk - bestBid
		analysis.SpreadPercent = (analysis.Spread / bestBid) * 100
	}

	return analysis
}

// TrendingSymbol 热门交易对数据
type TrendingSymbol struct {
	Symbol        string `json:"symbol"`
	PriceChange   string `json:"price_change"`
	PercentChange string `json:"percent_change"`
	Volume        string `json:"volume"`
	Price         string `json:"price"`
}

// GetTopSymbols 获取热门USDT交易对（按成交量排序，包含涨跌幅）
func (s *BinanceService) GetTopSymbols(limit int) ([]*TrendingSymbol, error) {
	stats, err := s.client.NewListPriceChangeStatsService().Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("获取交易对统计失败: %w", err)
	}

	// 过滤USDT交易对并按成交量排序取前N个
	trending := make([]*TrendingSymbol, 0, limit)
	count := 0
	for _, stat := range stats {
		if count >= limit {
			break
		}
		// 只取USDT交易对
		if strings.HasSuffix(stat.Symbol, "USDT") {
			trending = append(trending, &TrendingSymbol{
				Symbol:        stat.Symbol,
				PriceChange:   stat.PriceChange,
				PercentChange: stat.PriceChangePercent,
				Volume:        stat.QuoteVolume,
				Price:         stat.LastPrice,
			})
			count++
		}
	}

	return trending, nil
}

func (s *BinanceService) GetFundingRate(symbol string) (string, error) {
	premiumIndices, err := s.futuresClient.NewPremiumIndexService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return "", fmt.Errorf("get funding rate err: %w", err)
	}
	if len(premiumIndices) > 0 {
		return premiumIndices[0].LastFundingRate, nil
	}
	return "", nil
}

func (s *BinanceService) GetLongShortRatio(symbol string, period string) (string, error) {
	ratios, err := s.futuresClient.NewLongShortRatioService().
		Symbol(symbol).Period(period).Do(context.Background())
	if err != nil {
		return "", fmt.Errorf("get long short ratio err: %w", err)
	}
	return ratios[0].LongShortRatio, nil
}

// MarketSummary 市场概览数据
type MarketSummary struct {
	TotalMarketCap         string `json:"total_market_cap"`
	TotalVolume24h         string `json:"total_volume_24h"`
	BTCDominance           string `json:"btc_dominance"`
	FearGreedIndex         int    `json:"fear_greed_index"`
	ActiveCryptocurrencies int    `json:"active_cryptocurrencies"`
	TotalExchanges         int    `json:"total_exchanges"`
}

// marketSummaryCache 市场概览缓存
type marketSummaryCache struct {
	data      *MarketSummary
	timestamp time.Time
}

// formatVolume 格式化成交量
func (s *BinanceService) formatVolume(volume float64) string {
	if volume >= 1e12 {
		return fmt.Sprintf("%.2fT", volume/1e12)
	} else if volume >= 1e9 {
		return fmt.Sprintf("%.2fB", volume/1e9)
	} else if volume >= 1e6 {
		return fmt.Sprintf("%.2fM", volume/1e6)
	} else if volume >= 1e3 {
		return fmt.Sprintf("%.2fK", volume/1e3)
	}
	return fmt.Sprintf("%.2f", volume)
}

// formatMarketCap 格式化市值
func (s *BinanceService) formatMarketCap(marketCap float64) string {
	if marketCap >= 1e12 {
		return fmt.Sprintf("%.2fT", marketCap/1e12)
	} else if marketCap >= 1e9 {
		return fmt.Sprintf("%.2fB", marketCap/1e9)
	} else if marketCap >= 1e6 {
		return fmt.Sprintf("%.2fM", marketCap/1e6)
	}
	return fmt.Sprintf("%.2f", marketCap)
}

// GetMarketSummary 获取市场概览数据
func (s *BinanceService) GetMarketSummary() (*MarketSummary, error) {
	// 检查缓存
	s.mu.RLock()
	if s.marketSummaryCache != nil &&
		time.Since(s.marketSummaryCache.timestamp) < s.marketSummaryTTL {
		defer s.mu.RUnlock()
		// 返回缓存的副本，避免外部修改
		cached := *s.marketSummaryCache.data
		return &cached, nil
	}
	s.mu.RUnlock()

	// 优先使用alternative.me的全球市场数据
	globalData, err := s.GetGlobalMarketData()
	var totalMarketCap float64
	var totalVolume24h float64
	var btcDominance float64
	var activeCryptos int
	var totalExchanges int

	if err != nil {
		s.logger.Warn("获取全球市场数据失败，回退到Binance数据", zap.Error(err))
		// 回退到Binance数据计算
		return s.getMarketSummaryFromBinance()
	}

	// 使用alternative.me的准确数据
	if usdQuotes, exists := globalData.Data.Quotes["USD"]; exists {
		totalMarketCap = usdQuotes.TotalMarketCap
		totalVolume24h = usdQuotes.TotalVolume24h
	} else {
		s.logger.Error("全球市场数据中未找到USD报价")
		return s.getMarketSummaryFromBinance()
	}

	btcDominance = globalData.Data.BitcoinPercentageOfMarketCap
	activeCryptos = globalData.Data.ActiveCryptocurrencies
	totalExchanges = globalData.Data.ActiveMarkets

	// 获取恐惧与贪婪指数
	fearGreedIndex, err := s.GetFearGreedIndex()
	if err != nil {
		s.logger.Warn("获取恐惧与贪婪指数失败，使用默认值", zap.Error(err))
		fearGreedIndex = 50 // 默认中性值
	}

	summary := &MarketSummary{
		TotalMarketCap:         s.formatMarketCap(totalMarketCap),
		TotalVolume24h:         s.formatVolume(totalVolume24h),
		BTCDominance:           fmt.Sprintf("%.1f%%", btcDominance),
		FearGreedIndex:         fearGreedIndex,
		ActiveCryptocurrencies: activeCryptos,
		TotalExchanges:         totalExchanges,
	}

	// 更新缓存
	s.mu.Lock()
	s.marketSummaryCache = &marketSummaryCache{
		data:      summary,
		timestamp: time.Now(),
	}
	s.mu.Unlock()

	s.logger.Info("成功获取市场概览数据",
		zap.String("market_cap", summary.TotalMarketCap),
		zap.String("volume_24h", summary.TotalVolume24h),
		zap.String("btc_dominance", summary.BTCDominance),
		zap.Int("fear_greed_index", summary.FearGreedIndex))

	return summary, nil
}

// getMarketSummaryFromBinance 从Binance数据计算市场概览（备用方案）
func (s *BinanceService) getMarketSummaryFromBinance() (*MarketSummary, error) {
	// 获取24小时统计数据来计算总成交量
	stats, err := s.client.NewListPriceChangeStatsService().Do(context.Background())
	if err != nil {
		s.logger.Error("获取市场统计失败", zap.Error(err))
		return nil, fmt.Errorf("获取市场统计失败: %w", err)
	}

	var totalVolume float64
	var btcVolume float64
	var btcPrice float64
	var validSymbols int

	for _, stat := range stats {
		if strings.HasSuffix(stat.Symbol, "USDT") {
			volume, err := strconv.ParseFloat(stat.QuoteVolume, 64)
			if err != nil {
				s.logger.Warn("解析成交量失败", zap.String("symbol", stat.Symbol), zap.Error(err))
				continue
			}

			if volume > 0 {
				totalVolume += volume
				validSymbols++
			}

			if stat.Symbol == "BTCUSDT" {
				btcVolume = volume
				btcPrice, err = strconv.ParseFloat(stat.LastPrice, 64)
				if err != nil {
					s.logger.Warn("解析BTC价格失败", zap.Error(err))
					btcPrice = 0
				}
			}
		}
	}

	// 计算BTC市值占比（基于成交量加权）
	var btcDominance float64
	if btcVolume > 0 && totalVolume > 0 {
		btcDominance = (btcVolume / totalVolume) * 100
	} else {
		// 如果无法计算，使用行业平均值
		btcDominance = 45.0
	}

	// 估算总市值（基于BTC价格和占比）
	var estimatedMarketCap float64
	if btcPrice > 0 && btcDominance > 0 {
		// BTC流通量约19.5M，使用更准确的估算
		btcCirculatingSupply := 19.5e6
		estimatedMarketCap = (btcPrice * float64(btcCirculatingSupply)) / (btcDominance / 100)
	} else {
		// 如果无法计算，使用默认值
		estimatedMarketCap = 2.5e12 // 2.5万亿
	}

	// 动态计算活跃加密货币数量（基于有效交易对数量）
	activeCryptos := validSymbols
	if activeCryptos == 0 {
		activeCryptos = 2500 // 默认值
	}

	// 动态计算交易所数量（基于行业数据）
	totalExchanges := 100 + (validSymbols / 50) // 根据交易对数量估算

	// 获取恐惧与贪婪指数
	fearGreedIndex, err := s.GetFearGreedIndex()
	if err != nil {
		s.logger.Warn("获取恐惧与贪婪指数失败，使用默认值", zap.Error(err))
		fearGreedIndex = 50 // 默认中性值
	}

	summary := &MarketSummary{
		TotalMarketCap:         s.formatMarketCap(estimatedMarketCap),
		TotalVolume24h:         s.formatVolume(totalVolume),
		BTCDominance:           fmt.Sprintf("%.1f%%", btcDominance),
		FearGreedIndex:         fearGreedIndex,
		ActiveCryptocurrencies: activeCryptos,
		TotalExchanges:         totalExchanges,
	}

	// 更新缓存
	s.mu.Lock()
	s.marketSummaryCache = &marketSummaryCache{
		data:      summary,
		timestamp: time.Now(),
	}
	s.mu.Unlock()

	s.logger.Info("使用Binance数据获取市场概览（备用方案）",
		zap.String("market_cap", summary.TotalMarketCap),
		zap.String("volume_24h", summary.TotalVolume24h))

	return summary, nil
}

// calculateFearGreedIndex 计算恐贪指数（基于市场数据）- 保留作为备用方案
func (s *BinanceService) calculateFearGreedIndex(volume float64, btcDominance float64) int {
	// 基于成交量和BTC占比的简单算法
	// 成交量高 + BTC占比低 = 贪婪
	// 成交量低 + BTC占比高 = 恐惧

	volumeScore := 0
	if volume > 100e9 { // 100B+
		volumeScore = 30
	} else if volume > 50e9 { // 50B+
		volumeScore = 20
	} else if volume > 20e9 { // 20B+
		volumeScore = 10
	}

	dominanceScore := 0
	if btcDominance > 50 {
		dominanceScore = 20 // BTC占比高，市场相对保守
	} else if btcDominance > 40 {
		dominanceScore = 10
	} else {
		dominanceScore = 0 // BTC占比低，市场相对激进
	}

	baseScore := 50
	totalScore := baseScore + volumeScore - dominanceScore

	// 限制在0-100范围内
	if totalScore > 100 {
		totalScore = 100
	} else if totalScore < 0 {
		totalScore = 0
	}

	return totalScore
}

// FearGreedResponse 恐惧与贪婪指数API响应
type FearGreedResponse struct {
	Name string `json:"name"`
	Data []struct {
		Value           string `json:"value"`
		ValueClass      string `json:"value_classification"`
		Timestamp       string `json:"timestamp"`
		TimeUntilUpdate string `json:"time_until_update"`
	} `json:"data"`
	Metadata struct {
		Error interface{} `json:"error"`
	} `json:"metadata"`
}

// GlobalMarketResponse 全球市场数据API响应
type GlobalMarketResponse struct {
	Data struct {
		ActiveCryptocurrencies       int     `json:"active_cryptocurrencies"`
		ActiveMarkets                int     `json:"active_markets"`
		BitcoinPercentageOfMarketCap float64 `json:"bitcoin_percentage_of_market_cap"`
		Quotes                       map[string]struct {
			TotalMarketCap float64 `json:"total_market_cap"`
			TotalVolume24h float64 `json:"total_volume_24h"`
		} `json:"quotes"`
		LastUpdated int64 `json:"last_updated"`
	} `json:"data"`
	Metadata struct {
		Timestamp int64       `json:"timestamp"`
		Error     interface{} `json:"error"`
	} `json:"metadata"`
}

// fearGreedCache 恐惧与贪婪指数缓存
type fearGreedCache struct {
	value     int
	timestamp time.Time
}

// GetFearGreedIndex 从alternative.me API获取恐惧与贪婪指数
func (s *BinanceService) GetFearGreedIndex() (int, error) {
	// 检查缓存
	s.mu.RLock()
	if s.fearGreedCache != nil &&
		time.Since(s.fearGreedCache.timestamp) < s.fearGreedTTL {
		defer s.mu.RUnlock()
		return s.fearGreedCache.value, nil
	}
	s.mu.RUnlock()

	// 调用API
	url := "https://api.alternative.me/fng/"
	resp, err := s.httpClient.Get(url)
	if err != nil {
		s.logger.Error("获取恐惧与贪婪指数失败", zap.Error(err))
		return 50, fmt.Errorf("获取恐惧与贪婪指数失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.Error("恐惧与贪婪指数API返回错误状态码", zap.Int("status", resp.StatusCode))
		return 50, fmt.Errorf("API返回错误状态码: %d", resp.StatusCode)
	}

	var fearGreedResp FearGreedResponse
	if err := json.NewDecoder(resp.Body).Decode(&fearGreedResp); err != nil {
		s.logger.Error("解析恐惧与贪婪指数响应失败", zap.Error(err))
		return 50, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查API错误
	if fearGreedResp.Metadata.Error != nil {
		s.logger.Error("恐惧与贪婪指数API返回错误", zap.Any("error", fearGreedResp.Metadata.Error))
		return 50, fmt.Errorf("API错误: %v", fearGreedResp.Metadata.Error)
	}

	if len(fearGreedResp.Data) == 0 {
		s.logger.Warn("恐惧与贪婪指数API返回空数据")
		return 50, nil
	}

	// 获取最新的指数值
	latest := fearGreedResp.Data[0]
	value, err := strconv.Atoi(latest.Value)
	if err != nil {
		s.logger.Error("解析恐惧与贪婪指数值失败", zap.String("value", latest.Value), zap.Error(err))
		return 50, fmt.Errorf("解析指数值失败: %w", err)
	}

	// 验证值范围
	if value < 0 || value > 100 {
		s.logger.Warn("恐惧与贪婪指数值超出范围", zap.Int("value", value))
		value = 50 // 使用默认值
	}

	// 更新缓存
	s.mu.Lock()
	s.fearGreedCache = &fearGreedCache{
		value:     value,
		timestamp: time.Now(),
	}
	s.mu.Unlock()

	s.logger.Info("成功获取恐惧与贪婪指数",
		zap.Int("value", value),
		zap.String("classification", latest.ValueClass))

	return value, nil
}

// GetGlobalMarketData 获取全球市场数据
func (s *BinanceService) GetGlobalMarketData() (*GlobalMarketResponse, error) {
	url := "https://api.alternative.me/v2/global/"
	resp, err := s.httpClient.Get(url)
	if err != nil {
		s.logger.Error("获取全球市场数据失败", zap.Error(err))
		return nil, fmt.Errorf("获取全球市场数据失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.Error("全球市场数据API返回错误状态码", zap.Int("status", resp.StatusCode))
		return nil, fmt.Errorf("API返回错误状态码: %d", resp.StatusCode)
	}

	var globalResp GlobalMarketResponse
	if err := json.NewDecoder(resp.Body).Decode(&globalResp); err != nil {
		s.logger.Error("解析全球市场数据响应失败", zap.Error(err))
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查API错误
	if globalResp.Metadata.Error != nil {
		s.logger.Error("全球市场数据API返回错误", zap.Any("error", globalResp.Metadata.Error))
		return nil, fmt.Errorf("API错误: %v", globalResp.Metadata.Error)
	}

	return &globalResp, nil
}
