package ioc

import (
	"github.com/dushixiang/prism/internal/handler"
	"github.com/dushixiang/prism/internal/service"
)

type Container struct {
	UserHandler     *handler.UserHandler
	AccountHandler  *handler.AccountHandler
	PropertyHandler *handler.PropertyHandler
	MarketHandler   *handler.MarketHandler
	NewsHandler     *handler.NewsHandler

	UserService        *service.UserService
	AccountService     *service.AccountService
	PropertyService    *service.PropertyService
	NewsService        *service.NewsService
	BinanceService     *service.BinanceService
	TechnicalService   *service.TechnicalAnalysisService
	LLMAnalysisService *service.LLMAnalysisService
}
