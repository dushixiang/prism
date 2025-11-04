package models

import (
	"time"

	"gorm.io/gorm"
)

// LLMLog LLM通信日志记录
type LLMLog struct {
	ID               string         `gorm:"primaryKey;type:varchar(36)" json:"id"`
	DecisionID       string         `gorm:"index" json:"decision_id"`          // 关联的决策ID
	Iteration        int            `gorm:"not null;index" json:"iteration"`   // 决策迭代次数
	RoundNumber      int            `gorm:"not null" json:"round_number"`      // 本次决策的轮次(工具调用轮次)
	Model            string         `json:"model"`                             // 使用的AI模型
	SystemPrompt     string         `json:"-"`                                 // 系统提示词(前端隐藏)
	UserPrompt       string         `json:"user_prompt"`                       // 用户提示词
	Messages         string         `json:"messages"`                          // 完整的消息历史(JSON格式)
	AssistantContent string         `json:"assistant_content"`                 // AI返回的内容
	ToolCalls        string         `json:"tool_calls"`                        // AI调用的工具(JSON格式)
	ToolResponses    string         `json:"tool_responses"`                    // 工具执行的响应(JSON格式)
	PromptTokens     int            `json:"prompt_tokens"`                     // 提示词token数
	CompletionTokens int            `json:"completion_tokens"`                 // 完成token数
	TotalTokens      int            `json:"total_tokens"`                      // 总token数
	FinishReason     string         `json:"finish_reason"`                     // 结束原因
	Duration         int64          `json:"duration"`                          // 请求耗时(毫秒)
	Error            string         `json:"error"`                             // 错误信息(如果有)
	ExecutedAt       time.Time      `gorm:"not null;index" json:"executed_at"` // 执行时间
	CreatedAt        time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName 指定表名
func (LLMLog) TableName() string {
	return "llm_logs"
}
