package config

type Config struct {
	Telegram TelegramConf `json:"telegram"`
	Binance  BinanceConf  `json:"binance"`
	Trading  TradingConf  `json:"trading"`
	LLM      LlmConf      `json:"llm"`
}

type TelegramConf struct {
	Enabled bool   `json:"enabled"`
	Token   string `json:"token"`
	ChatID  string `json:"chat_id"`
}

type BinanceConf struct {
	APIKey   string `json:"api_key"`
	Secret   string `json:"secret"`
	ProxyURL string `json:"proxy_url"` // 代理地址，例如: http://127.0.0.1:7890
	Testnet  bool   `json:"testnet"`   // 是否使用测试网
}

type TradingConf struct {
	Symbols             []string `json:"symbols"`                // 交易币种，如 ["BTCUSDT", "ETHUSDT"]
	IntervalMinutes     int      `json:"interval_minutes"`       // 交易周期（分钟），默认10
	StopLossUSDT        float64  `json:"stop_loss_usdt"`         // 账户止损线（USDT）
	TakeProfitUSDT      float64  `json:"take_profit_usdt"`       // 账户止盈线（USDT）
	MaxDrawdownPercent  float64  `json:"max_drawdown_percent"`   // 最大回撤百分比，默认15
	MaxHoldingHours     int      `json:"max_holding_hours"`      // 最大持仓时间（小时），默认36
	MaxPositions        int      `json:"max_positions"`          // 最大持仓数，默认5
	MaxLeverage         int      `json:"max_leverage"`           // 最大杠杆，默认15
	MinLeverage         int      `json:"min_leverage"`           // 最小杠杆，默认5
	RiskPercentPerTrade float64  `json:"risk_percent_per_trade"` // 单笔交易风险百分比，默认2-3%
}

type LlmConf struct {
	BaseURL  string `json:"base_url"`  // LLM API基础URL
	APIKey   string `json:"api_key"`   // LLM API密钥
	Model    string `json:"model"`     // 模型名称
	ProxyURL string `json:"proxy_url"` // 代理地址，例如: http://127.0.0.1:7890
}
