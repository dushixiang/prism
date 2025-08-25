package models

type News struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
	Link       string  `json:"link"`
	Source     string  `json:"source"`    // 新闻源标识
	Sentiment  string  `json:"sentiment"` // 情绪分析结果：positive/negative/neutral
	Score      float64 `json:"score"`     // 情绪评分 0-1
	Summary    string  `json:"summary"`   // 简要总结，100字以内
	CreatedAt  int64   `json:"createdAt"`
	OriginalID string  `json:"originalId"` // 原始新闻ID
}

func (r News) TableName() string {
	return "news"
}
