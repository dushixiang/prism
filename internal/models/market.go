package models

import "time"

// KlineData K线数据
type KlineData struct {
	OpenTime   time.Time `json:"open_time"`
	CloseTime  time.Time `json:"close_time"`
	OpenPrice  float64   `json:"open_price"`
	HighPrice  float64   `json:"high_price"`
	LowPrice   float64   `json:"low_price"`
	ClosePrice float64   `json:"close_price"`
	Volume     float64   `json:"volume"`
}

// TechnicalIndicators 技术指标
type TechnicalIndicators struct {
	// 移动平均线
	MA5   float64 `json:"ma5"`
	MA10  float64 `json:"ma10"`
	MA20  float64 `json:"ma20"`
	MA50  float64 `json:"ma50"`
	MA200 float64 `json:"ma200"`

	// 指数移动平均线
	EMA12 float64 `json:"ema12"`
	EMA26 float64 `json:"ema26"`

	// MACD指标
	MACD       float64 `json:"macd"`
	MACDSignal float64 `json:"macd_signal"`
	MACDHist   float64 `json:"macd_hist"`

	// RSI指标
	RSI float64 `json:"rsi"`

	// 布林带
	BBUpper  float64 `json:"bb_upper"`
	BBMiddle float64 `json:"bb_middle"`
	BBLower  float64 `json:"bb_lower"`

	// KDJ指标
	StochK float64 `json:"stoch_k"`
	StochD float64 `json:"stoch_d"`
	StochJ float64 `json:"stoch_j"`

	// 其他指标
	CCI float64 `json:"cci"`
	SAR float64 `json:"sar"`
	ATR float64 `json:"atr"`
	ADX float64 `json:"adx"`

	// 成交量指标
	OBV float64 `json:"obv"`
	MFI float64 `json:"mfi"`
}

type Timelines struct {
	Symbol string    `json:"symbol"`
	Hourly *Timeline `json:"hourly"`
	Daily  *Timeline `json:"daily"`
	News   []News    `json:"news"`

	FundingRate    string `json:"funding_rate"`     // 最新的永续合约资金费率
	LongShortRatio string `json:"long_short_ratio"` // 最新的永续合约多空比
}

type Timeline struct {
	Interval   string               `json:"interval"`
	Data       []KlineData          `json:"data"`
	Indicators *TechnicalIndicators `json:"indicators"`
}

// DepthAnalysis 市场深度分析
type DepthAnalysis struct {
	Symbol        string    `json:"symbol"`
	Timestamp     time.Time `json:"timestamp"`
	BidVolume     float64   `json:"bid_volume"`     // 买单总量
	AskVolume     float64   `json:"ask_volume"`     // 卖单总量
	BidValue      float64   `json:"bid_value"`      // 买单总价值
	AskValue      float64   `json:"ask_value"`      // 卖单总价值
	BidAskRatio   float64   `json:"bid_ask_ratio"`  // 买卖比例
	Spread        float64   `json:"spread"`         // 价差
	SpreadPercent float64   `json:"spread_percent"` // 价差百分比
}

// MarketAnalysis 市场分析结果
type MarketAnalysis struct {
	Symbol             string         `json:"symbol"`
	Timestamp          int64          `json:"timestamp"`
	CurrentPrice       float64        `json:"current_price"`
	PriceChange24h     float64        `json:"price_change_24h"`
	PriceChangePercent float64        `json:"price_change_percent"`
	Volume24h          float64        `json:"volume_24h"`
	DepthAnalysis      *DepthAnalysis `json:"depth_analysis"`
	Trend              string         `json:"trend"`            // 趋势方向：up/down/sideways
	Strength           float64        `json:"strength"`         // 趋势强度 0-10
	SupportLevel       float64        `json:"support_level"`    // 支撑位
	ResistanceLevel    float64        `json:"resistance_level"` // 阻力位
	RiskLevel          string         `json:"risk_level"`       // 风险等级：low/medium/high
	MarketRegime       string         `json:"market_regime"`    // 市场状态: Trending, Ranging, Uncertain
}

// TradingSignal 交易信号
type TradingSignal struct {
	Symbol     string  `json:"symbol"`
	Interval   string  `json:"interval"` // 时间间隔 (1m, 5m, 15m, 1h, 4h, 1d等)
	Timestamp  int64   `json:"timestamp"`
	SignalType string  `json:"signal_type"` // buy/sell/hold
	Strength   float64 `json:"strength"`    // 信号强度 0-10
	Price      float64 `json:"price"`       // 触发价格
	StopLoss   float64 `json:"stop_loss"`   // 止损价
	TakeProfit float64 `json:"take_profit"` // 止盈价
	Reasoning  string  `json:"reasoning"`   // 信号理由
	Confidence float64 `json:"confidence"`  // 置信度
	RiskLevel  string  `json:"risk_level"`  // 风险等级
	IsSent     bool    `json:"is_sent"`     // 是否已推送
}
