package models

import (
	"time"

	"gorm.io/datatypes"
)

// SystemPrompt 系统提示词版本记录
type SystemPrompt struct {
	ID        string    `json:"id"`
	Version   int       `json:"version"`
	Content   string    `json:"content"`
	IsActive  bool      `json:"is_active"`
	Remark    string    `json:"remark"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (SystemPrompt) TableName() string {
	return "system_prompt"
}

// TradingConfig 交易参数配置
type TradingConfig struct {
	ID                 string                      `json:"id"`
	Symbols            datatypes.JSONSlice[string] `json:"symbols"`
	IntervalMinutes    int                         `json:"interval_minutes"`
	MaxDrawdownPercent float64                     `json:"max_drawdown_percent"`
	MaxPositions       int                         `json:"max_positions"`
	MaxLeverage        int                         `json:"max_leverage"`
	MinLeverage        int                         `json:"min_leverage"`
	CreatedAt          time.Time                   `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt          time.Time                   `gorm:"autoUpdateTime" json:"updated_at"`
}

func (TradingConfig) TableName() string {
	return "trading_config"
}
