package models

import (
	"time"

	"gorm.io/gorm"
)

// Decision AI决策记录
type Decision struct {
	ID               string         `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Iteration        int            `gorm:"not null;index" json:"iteration"`   // 调用次数
	AccountValue     float64        `json:"account_value"`                     // 决策时账户价值
	PositionCount    int            `json:"position_count"`                    // 持仓数量
	DecisionContent  string         `json:"decision_content"`                  // AI决策内容
	PromptTokens     int            `json:"prompt_tokens"`                     // 提示词token数
	CompletionTokens int            `json:"completion_tokens"`                 // 完成token数
	Model            string         `json:"model"`                             // 使用的AI模型
	ExecutedAt       time.Time      `gorm:"not null;index" json:"executed_at"` // 执行时间
	CreatedAt        time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName 指定表名
func (Decision) TableName() string {
	return "decisions"
}
