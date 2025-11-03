package config

type Config struct {
	Telegram TelegramConf `json:"telegram"`
	Binance  BinanceConf  `json:"binance"`
	Trading  TradingConf  `json:"trading"`
	LLM      LlmConf      `json:"llm"`
	Admin    AdminConf    `json:"admin"`
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
	Enabled     bool            `json:"enabled"`      // 是否启用真实交易，false时使用纸钱包模式
	PaperWallet PaperWalletConf `json:"paper_wallet"` // 纸钱包配置
}

type PaperWalletConf struct {
	InitialBalance float64 `json:"initial_balance"` // 初始余额（USDT），默认1000
}

type LlmConf struct {
	BaseURL  string `json:"base_url"`  // LLM API基础URL
	APIKey   string `json:"api_key"`   // LLM API密钥
	Model    string `json:"model"`     // 模型名称
	ProxyURL string `json:"proxy_url"` // 代理地址，例如: http://127.0.0.1:7890
}

type AdminConf struct {
	JWTSecret string `json:"jwt_secret"` // JWT密钥（用于前端登录认证）
}
