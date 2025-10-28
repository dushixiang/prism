package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// TechnicalIndicator 技术指标
type TechnicalIndicator struct {
	ID          string  `gorm:"primaryKey;type:varchar(26)" json:"id"`
	Symbol      string  `gorm:"type:varchar(20);not null;index:idx_symbol_timeframe_time" json:"symbol"`    // 交易对
	Timeframe   string  `gorm:"type:varchar(10);not null;index:idx_symbol_timeframe_time" json:"timeframe"` // 时间框架：1m/3m/5m/15m/30m/1h/4h
	Price       float64 `gorm:"type:decimal(20,8)" json:"price"`                                            // 当前价格
	EMA20       float64 `gorm:"type:decimal(20,8)" json:"ema20"`                                            // EMA20
	EMA50       float64 `gorm:"type:decimal(20,8)" json:"ema50"`                                            // EMA50
	MACD        float64 `gorm:"type:decimal(20,8)" json:"macd"`                                             // MACD
	MACDSignal  float64 `gorm:"type:decimal(20,8)" json:"macd_signal"`                                      // MACD信号线
	MACDHist    float64 `gorm:"type:decimal(20,8)" json:"macd_hist"`                                        // MACD柱状图
	RSI7        float64 `gorm:"type:decimal(10,4)" json:"rsi7"`                                             // RSI7
	RSI14       float64 `gorm:"type:decimal(10,4)" json:"rsi14"`                                            // RSI14
	ATR3        float64 `gorm:"type:decimal(20,8)" json:"atr3"`                                             // ATR3
	ATR14       float64 `gorm:"type:decimal(20,8)" json:"atr14"`                                            // ATR14
	Volume      float64 `gorm:"type:decimal(20,8)" json:"volume"`                                           // 成交量
	AvgVolume   float64 `gorm:"type:decimal(20,8)" json:"avg_volume"`                                       // 平均成交量
	FundingRate float64 `gorm:"type:decimal(10,8)" json:"funding_rate"`                                     // 资金费率

	// 时序数据（JSON格式存储最近10个数据点）
	PriceSeries datatypes.JSON `gorm:"type:json" json:"price_series"` // 价格序列
	EMA20Series datatypes.JSON `gorm:"type:json" json:"ema20_series"` // EMA20序列
	MACDSeries  datatypes.JSON `gorm:"type:json" json:"macd_series"`  // MACD序列
	RSI7Series  datatypes.JSON `gorm:"type:json" json:"rsi7_series"`  // RSI7序列
	RSI14Series datatypes.JSON `gorm:"type:json" json:"rsi14_series"` // RSI14序列

	CalculatedAt time.Time      `gorm:"not null;index:idx_symbol_timeframe_time" json:"calculated_at"` // 计算时间
	CreatedAt    time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName 指定表名
func (TechnicalIndicator) TableName() string {
	return "technical_indicators"
}
