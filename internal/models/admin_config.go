package models

import (
	"time"

	"gorm.io/datatypes"
)

// SystemPrompt 系统提示词版本记录
type SystemPrompt struct {
	ID        string    `gorm:"primaryKey;size:26" json:"id"`
	Version   int       `gorm:"uniqueIndex;not null" json:"version"`
	Content   string    `gorm:"type:longtext;not null" json:"content"`
	IsActive  bool      `gorm:"index;not null;default:false" json:"is_active"`
	Remark    string    `gorm:"size:500" json:"remark"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (SystemPrompt) TableName() string {
	return "system_prompt"
}

// TradingConfig 交易参数配置
type TradingConfig struct {
	ID                 string                      `gorm:"primaryKey;size:26" json:"id"`
	Symbols            datatypes.JSONSlice[string] `gorm:"type:json" json:"symbols"`
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
