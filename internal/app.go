package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/dushixiang/prism/frontend"
	"github.com/dushixiang/prism/internal/config"
	"github.com/dushixiang/prism/internal/ioc"
	"github.com/dushixiang/prism/internal/models"
	"github.com/dushixiang/prism/internal/telegram"
	"github.com/dushixiang/prism/pkg/nostd"
	"github.com/go-orz/orz"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
)

func Run(configPath string) error {
	app := NewPrismApp()

	framework, err := orz.NewFramework(
		orz.WithConfig(configPath),
		orz.WithLoggerFromConfig(),
		orz.WithDatabase(),
		orz.WithHTTP(),
		orz.WithApplication(app),
	)
	if err != nil {
		return err
	}

	return framework.Run()
}

func NewPrismApp() orz.Application {
	return &PrismApp{}
}

var _ orz.Application = (*PrismApp)(nil)

type PrismApp struct {
	container *ioc.Container
	conf      *config.Config
	tg        *telegram.Telegram
}

func (r *PrismApp) Configure(app *orz.App) error {
	logger := app.Logger()

	e, err := app.GetEcho()
	if err != nil {
		return err
	}

	db, err := app.GetDatabase()
	if err != nil {
		return err
	}

	var conf config.Config
	err = app.GetConfig().App.Unmarshal(&conf)
	if err != nil {
		return fmt.Errorf("failed to unmarshal config: %v", err)
	}

	var httpClient = &http.Client{
		Timeout: time.Second * 10,
	}
	if runtime.GOOS == "darwin" {
		u, _ := url.Parse("http://127.0.0.1:7890")
		httpClient.Transport = &http.Transport{
			Proxy: http.ProxyURL(u),
		}
	}
	// 初始化telegram
	if conf.Telegram.Enabled {
		tg, err := telegram.NewTelegram(logger,
			telegram.Settings{
				Token:  conf.Telegram.Token,
				Client: httpClient,
			})
		if err != nil {
			return fmt.Errorf("failed to init telegram: %v", err)
		}
		r.tg = tg
		r.tg.Start()
	}

	r.container = ProviderContainer(logger, db, &conf)

	if err := db.AutoMigrate(
		models.User{}, models.Session{}, models.LoginLog{}, models.Property{},
		models.News{},
	); err != nil {
		logger.Fatal("database auto migrate failed", zap.Error(err))
	}

	if err := r.Init(); err != nil {
		logger.Fatal("app init failed", zap.Error(err))
	}

	e.HidePort = true
	e.HideBanner = true

	e.Use(middleware.Gzip())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		Skipper:      middleware.DefaultSkipper,
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodPost, http.MethodDelete},
	}))
	e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
			sugar := logger.Sugar()
			sugar.Error(fmt.Sprintf("[PANIC RECOVER] %v %s\n", err, stack))
			return err
		},
	}))
	e.Use(WithErrorHandler(logger))
	customValidator := nostd.CustomValidator{Validator: validator.New()}
	if err := customValidator.TransInit(); err != nil {
		logger.Sugar().Fatal("failed to init custom validator", zap.Error(err))
	}
	e.Validator = &customValidator

	e.Use(middleware.StaticWithConfig(middleware.StaticConfig{
		Skipper: func(c echo.Context) bool {
			path := c.Request().RequestURI
			if strings.HasPrefix(path, "/api") {
				return true
			}
			return false
		},
		Root:       "",
		Index:      "index.html",
		HTML5:      true,
		Browse:     false,
		IgnoreBase: false,
		Filesystem: http.FS(frontend.Assets()),
	}))

	e.POST("/api/login", r.container.AccountHandler.Login)

	api := e.Group("/api", r.Auth)
	{
		account := api.Group("/account")
		{
			accountHandler := r.container.AccountHandler
			account.GET("/info", accountHandler.Info)
			account.POST("/logout", accountHandler.Logout)
			account.POST("/change-password", accountHandler.ChangePassword)
			account.POST("/change-profile", accountHandler.ChangeProfile)
		}

		admin := api.Group("/admin", r.Admin)
		{
			user := admin.Group("/user")
			{
				userHandler := r.container.UserHandler
				user.GET("/paging", userHandler.Paging)
				user.POST("", userHandler.Create)
				user.PUT("/:id", userHandler.Update)
				user.GET("/:id", userHandler.Get)
				user.DELETE("/:id", userHandler.Delete)
				user.POST("/:id/change-password", userHandler.ChangePassword)
				user.POST("/enabled", userHandler.Enabled)
				user.POST("/disabled", userHandler.Disabled)
			}

			property := admin.Group("/property")
			{
				h := r.container.PropertyHandler
				property.PUT("", h.Set)
				property.GET("", h.Get)
			}
		}

		// 市场分析API
		market := api.Group("/market")
		{
			marketHandler := r.container.MarketHandler
			market.POST("/analyze/symbol", marketHandler.AnalyzeSymbol)
			market.GET("/kline", marketHandler.GetKlineData)
			market.GET("/symbols", marketHandler.GetSymbols)
			market.GET("/overview", marketHandler.GetMarketOverview)
			market.GET("/trending", marketHandler.GetTrendingSymbols)

			market.POST("/llm/prompt", marketHandler.BuildLLMPrompt)
			market.POST("/llm/stream", marketHandler.LLMAnalyzeStream)
		}

		// 新闻资讯API
		news := api.Group("/news")
		{
			newsHandler := r.container.NewsHandler
			news.GET("/latest", newsHandler.GetLatestNews)
			news.GET("/statistics", newsHandler.GetNewsStatistics)
		}
	}

	r.container.NewsService.StartSubscriber()

	return nil
}

func (r *PrismApp) Init() error {
	ctx := context.Background()
	if err := r.container.AccountService.Init(ctx); err != nil {
		return fmt.Errorf("init account err: %w", err)
	}
	if err := r.container.PropertyService.Init(ctx); err != nil {
		return fmt.Errorf("init property err: %w", err)
	}
	return nil
}
