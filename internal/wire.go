//go:build wireinject
// +build wireinject

package internal

import (
	"github.com/google/wire"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/dushixiang/prism/internal/config"
	"github.com/dushixiang/prism/internal/handler"
	"github.com/dushixiang/prism/internal/ioc"
	"github.com/dushixiang/prism/internal/service"
)

func ProviderContainer(logger *zap.Logger, db *gorm.DB, conf *config.Config) *ioc.Container {
	panic(wire.Build(appSet))
}

var appSet = wire.NewSet(
	serviceSet,
	apiSet,
	wire.Struct(new(ioc.Container), "*"),
)

var apiSet = wire.NewSet(
	handler.NewUserHandler,
	handler.NewAccountHandler,
	handler.NewPropertyHandler,
	handler.NewMarketHandler,
	handler.NewNewsHandler,
)

var serviceSet = wire.NewSet(
	service.NewUserService,
	service.NewAccountService,
	service.NewPropertyService,
	service.NewNewsService,
	service.NewBinanceService,
	service.NewLLMAnalysisService,
	service.NewTechnicalAnalysisService,
)
