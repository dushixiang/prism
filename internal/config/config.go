package config

type Config struct {
	Telegram TelegramConf `json:"telegram"`
	Binance  BinanceConf  `json:"binance"`
	Proxy    ProxyConf    `json:"proxy"`
	LLM      LLMConf      `json:"llm"`
	Signals  SignalsConf  `json:"signals"`
}

type TelegramConf struct {
	Enabled bool   `json:"enabled"`
	Token   string `json:"token"`
	ChatID  string `json:"chat_id"`
}

type BinanceConf struct {
	APIKey string `json:"api_key"`
	Secret string `json:"secret"`
}

type ProxyConf struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`
}

type LLMConf struct {
	APIKey  string    `json:"api_key"`
	BaseURL string    `json:"base_url"`
	Model   LLMModels `json:"model"`
}

type LLMModels struct {
	Default           string `json:"default"`            // 默认模型
	SentimentAnalysis string `json:"sentiment_analysis"` // 情感分析
}

type SignalsConf struct {
	Enabled             bool     `json:"enabled"`
	Symbols             []string `json:"symbols"`
	Intervals           []string `json:"intervals"`
	ScanInterval        int      `json:"scan_interval"`
	StrengthThreshold   float64  `json:"strength_threshold"`
	ConfidenceThreshold float64  `json:"confidence_threshold"`
	MaxSignalsPerDay    int      `json:"max_signals_per_day"`
	CleanupDays         int      `json:"cleanup_days"`
}
