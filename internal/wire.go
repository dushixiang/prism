//go:build wireinject
// +build wireinject

package internal

import (
	"net/http"
	"net/url"
	"time"

	"github.com/dushixiang/prism/pkg/exchange"
	"github.com/google/wire"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/dushixiang/prism/internal/config"
	"github.com/dushixiang/prism/internal/handler"
	"github.com/dushixiang/prism/internal/service"
	"github.com/dushixiang/prism/internal/telegram"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

const (
	telegramHTTPTimeout     = 10 * time.Second
	openaiProviderName      = "openai"
	logFieldMissingDeps     = "missing_dependencies"
	logFieldConfiguredModel = "model"
)

var (
	handlerSet = wire.NewSet(
		handler.NewTradingHandler,
	)

	tradingSet = wire.NewSet(
		provideBinanceClient,
		provideOpenAIClient,
		service.NewIndicatorService,
		service.NewMarketService,
		service.NewTradingAccountService,
		service.NewPositionService,
		service.NewRiskService,
		service.NewPromptService,
		service.NewAgentService,
		service.NewTradingLoop,
	)
)

// InitializeApp 初始化应用
func InitializeApp(logger *zap.Logger, db *gorm.DB, conf *config.Config) (*AppComponents, error) {
	wire.Build(
		handlerSet,
		tradingSet,
		provideTelegram,
		wire.Struct(new(AppComponents), "*"),
	)
	return nil, nil
}

// provideTelegram provides telegram instance
func provideTelegram(logger *zap.Logger, conf *config.Config) *telegram.Telegram {
	if !conf.Telegram.Enabled {
		return nil
	}

	httpClient := &http.Client{Timeout: telegramHTTPTimeout}

	tg, err := telegram.NewTelegram(logger, telegram.Settings{
		Token:  conf.Telegram.Token,
		Client: httpClient,
	})
	if err != nil {
		logger.Error("failed to init telegram", zap.Error(err))
		return nil
	}

	return tg
}

// provideBinanceClient provides Binance client
func provideBinanceClient(conf *config.Config, logger *zap.Logger) *exchange.BinanceClient {
	client := exchange.NewBinanceClient(
		conf.Binance.APIKey,
		conf.Binance.Secret,
		conf.Binance.ProxyURL,
		conf.Binance.Testnet,
	)

	if conf.Binance.APIKey == "" || conf.Binance.Secret == "" {
		logger.Warn("Binance API credentials not configured; some private endpoints may fail")
	}

	logger.Info("Binance client initialized",
		zap.Bool("testnet", conf.Binance.Testnet),
		zap.Bool("has_credentials", conf.Binance.APIKey != "" && conf.Binance.Secret != ""),
	)
	return client
}

// provideOpenAIClient provides OpenAI client
func provideOpenAIClient(conf *config.Config, logger *zap.Logger) *openai.Client {
	var options = []option.RequestOption{
		option.WithBaseURL(conf.LLM.BaseURL),
		option.WithAPIKey(conf.LLM.APIKey),
	}
	if conf.LLM.ProxyURL != "" {
		u, err := url.Parse(conf.LLM.ProxyURL)
		if err != nil {
			logger.Fatal("failed to parse proxy URL", zap.Error(err))
		}
		httpClient := &http.Client{
			Timeout: time.Minute,
			Transport: &http.Transport{
				Proxy: http.ProxyURL(u),
			},
		}
		options = append(options, option.WithHTTPClient(httpClient))
	}

	client := openai.NewClient(options...)

	logger.Info("OpenAI client initialized",
		zap.String(logFieldConfiguredModel, conf.LLM.Model),
		zap.String("provider", openaiProviderName),
	)
	return &client
}
