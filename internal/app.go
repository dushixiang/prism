package internal

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dushixiang/prism/internal/config"
	"github.com/dushixiang/prism/internal/handler"
	authmw "github.com/dushixiang/prism/internal/middleware"
	"github.com/dushixiang/prism/internal/models"
	"github.com/dushixiang/prism/internal/service"
	"github.com/dushixiang/prism/internal/telegram"
	"github.com/dushixiang/prism/pkg/nostd"
	"github.com/dushixiang/prism/web"
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

type AppComponents struct {
	TradingHandler *handler.TradingHandler
	AdminHandler   *handler.AdminHandler
	AuthHandler    *handler.AuthHandler
	SetupHandler   *handler.SetupHandler

	// Trading system services
	TradingLoop           *service.TradingLoop
	MarketService         *service.MarketService
	TradingAccountService *service.TradingAccountService
	PositionService       *service.PositionService
	AgentService          *service.AgentService
	AuthService           *service.AuthService
	AdminConfigService    *service.AdminConfigService

	tg *telegram.Telegram
}

type PrismApp struct {
	components *AppComponents
	conf       *config.Config
}

// GetComponents 获取应用组件
func (r *PrismApp) GetComponents() *AppComponents {
	return r.components
}

func (r *PrismApp) Configure(app *orz.App) error {
	logger := app.Logger()
	e := app.GetEcho()
	db := app.GetDatabase()

	var conf config.Config
	err := app.GetConfig().App.Unmarshal(&conf)
	if err != nil {
		return fmt.Errorf("failed to unmarshal config: %v", err)
	}

	components, err := InitializeApp(logger, db, &conf)
	if err != nil {
		return fmt.Errorf("failed to initialize app: %v", err)
	}
	r.components = components
	r.conf = &conf

	if err := db.AutoMigrate(
		// Trading system models
		models.AccountHistory{}, models.Position{}, models.Trade{}, models.Decision{}, models.LLMLog{}, models.Order{},
		// Admin models
		models.TradingConfig{}, models.AdminUser{}, models.SystemPrompt{},
	); err != nil {
		logger.Fatal("database auto migrate failed", zap.Error(err))
	}

	if err := r.Init(logger); err != nil {
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
		Filesystem: http.FS(web.Assets()),
	}))

	api := e.Group("/api")
	{
		// Setup API routes (首次设置，无需认证)
		if r.components.SetupHandler != nil {
			r.components.SetupHandler.RegisterRoutes(api)
		}

		// Trading API routes (无需认证)
		if r.components.TradingHandler != nil {
			r.components.TradingHandler.RegisterRoutes(api)
		}

		// Auth API routes (登录等公开接口)
		if r.components.AuthHandler != nil {
			r.components.AuthHandler.RegisterRoutes(api)

			// 需要JWT认证的auth接口
			if r.components.AuthService != nil {
				jwtMiddleware := authmw.JWTAuth(authmw.JWTAuthConfig{
					AuthService: r.components.AuthService,
					Logger:      logger,
				})
				authProtected := api.Group("/auth", jwtMiddleware)
				r.components.AuthHandler.RegisterProtectedRoutes(authProtected)
			}
		}

		// Admin API routes (支持两种认证方式)
		if r.components.AdminHandler != nil {
			// JWT认证（用于前端页面）
			var jwtMiddleware echo.MiddlewareFunc
			if r.components.AuthService != nil {
				jwtMiddleware = authmw.JWTAuth(authmw.JWTAuthConfig{
					AuthService: r.components.AuthService,
					Logger:      logger,
				})
			}

			// 创建支持两种认证方式的中间件
			dualAuthMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error {
					// 先尝试JWT认证
					authHeader := c.Request().Header.Get("Authorization")
					if authHeader != "" && jwtMiddleware != nil {
						return jwtMiddleware(next)(c)
					}

					// 都没有，返回401
					return c.JSON(http.StatusUnauthorized, map[string]interface{}{
						"error": "需要认证：请提供JWT Token或API Key",
					})
				}
			}

			// 为admin路由组应用双重认证中间件
			adminGroup := api.Group("/admin", dualAuthMiddleware)

			// 注册admin路由到带认证的组
			r.components.AdminHandler.RegisterRoutesWithGroup(adminGroup)
		}
	}

	return nil
}

func (r *PrismApp) Init(logger *zap.Logger) error {
	logger.Info("=================================================")
	logger.Info("Prism Trading System Starting...")
	logger.Info("=================================================")

	components := r.GetComponents()
	if components == nil {
		return fmt.Errorf("components not initialized")
	}

	if components.TradingLoop == nil {
		return fmt.Errorf("trading loop not available, please check Binance and LLM API configuration")
	}

	if components.AdminConfigService != nil {
		components.AdminConfigService.Initialize(context.Background())
		// 设置 TradingLoop 引用，用于配置更新后的自动重启
		if components.TradingLoop != nil {
			components.AdminConfigService.SetTradingLoop(components.TradingLoop)
			logger.Info("TradingLoop reference set to AdminConfigService")
		}
	}

	// 启动持仓同步worker（每3秒同步一次，保证数据实时性）
	if components.PositionService != nil {
		logger.Info("Starting position sync worker...")
		components.PositionService.StartSyncWorker(context.Background(), 3*time.Second)
	}

	logger.Info("Trading loop initialized, starting...")

	go func() {
		if err := components.TradingLoop.Start(context.Background()); err != nil {
			logger.Error("trading loop error", zap.Error(err))
		}
	}()
	return nil
}
