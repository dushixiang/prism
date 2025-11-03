package service

import (
	"context"
	_ "embed"
	"sort"
	"time"

	"github.com/dushixiang/prism/internal/models"
	"github.com/dushixiang/prism/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var DefaultTradingConfig = models.TradingConfig{
	ID:                 "00000000-0000-0000-0000-000000000000",
	Symbols:            []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"},
	IntervalMinutes:    15,
	MaxDrawdownPercent: 10,
	MaxPositions:       3,
	MaxLeverage:        10,
	MinLeverage:        3,
	CreatedAt:          time.Now(),
	UpdatedAt:          time.Now(),
}

//go:embed templates/system_instructions.txt
var defaultSystemInstructionsTemplate string
var defaultSystemPrompt = models.SystemPrompt{
	ID:       "00000000-0000-0000-0000-000000000000",
	Version:  1,
	Content:  defaultSystemInstructionsTemplate,
	IsActive: true,
	Remark:   "系统默认初始化",
}

type AdminConfigService struct {
	logger            *zap.Logger
	tradingConfigRepo *repo.TradingConfigRepo
	systemPromptRepo  *repo.SystemPromptRepo
	tradingLoop       *TradingLoop
}

func NewAdminConfigService(logger *zap.Logger, db *gorm.DB) *AdminConfigService {
	return &AdminConfigService{
		logger:            logger,
		tradingConfigRepo: repo.NewTradingConfigRepo(db),
		systemPromptRepo:  repo.NewSystemPromptRepo(db),
	}
}

// SetTradingLoop 设置交易循环引用（用于配置更新后重启）
func (s *AdminConfigService) SetTradingLoop(tradingLoop *TradingLoop) {
	s.tradingLoop = tradingLoop
}

func (s *AdminConfigService) Initialize(ctx context.Context) {
	// 初始化默认交易配置
	s.initializeTradingConfig(ctx)

	// 初始化默认系统提示词
	s.initializeSystemPrompt(ctx)
}

// initializeTradingConfig 初始化默认交易配置
func (s *AdminConfigService) initializeTradingConfig(ctx context.Context) {
	count, err := s.tradingConfigRepo.Count(ctx)
	if err != nil {
		s.logger.Error("获取交易配置失败", zap.Error(err))
		return
	}

	// 如果数据库中没有配置,创建默认配置
	if count == 0 {
		tradingConfig := DefaultTradingConfig
		if err := s.tradingConfigRepo.Create(ctx, &tradingConfig); err != nil {
			s.logger.Error("创建默认交易配置失败", zap.Error(err))
			return
		}
		s.logger.Info("默认交易配置初始化成功")
	}
}

// initializeSystemPrompt 初始化默认系统提示词
func (s *AdminConfigService) initializeSystemPrompt(ctx context.Context) {
	count, err := s.systemPromptRepo.Count(ctx)
	if err != nil {
		s.logger.Error("获取系统提示词失败", zap.Error(err))
		return
	}

	// 如果数据库中没有提示词,创建默认提示词
	if count == 0 {
		if err := s.systemPromptRepo.Create(ctx, &defaultSystemPrompt); err != nil {
			s.logger.Error("创建默认系统提示词失败", zap.Error(err))
			return
		}
		s.logger.Info("默认系统提示词初始化成功")
	}
}

func (s *AdminConfigService) GetTradingConfig(ctx context.Context) (*models.TradingConfig, error) {
	configs, err := s.tradingConfigRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	if len(configs) == 0 {
		tradingConfig := DefaultTradingConfig
		if err := s.tradingConfigRepo.Create(ctx, &tradingConfig); err != nil {
			return nil, err
		}
		return &tradingConfig, nil
	}
	return &configs[0], nil
}

func (s *AdminConfigService) SetTradingConfig(ctx context.Context, newTradingConfig models.TradingConfig) error {
	config, err := s.GetTradingConfig(ctx)
	if err != nil {
		return err
	}

	// 检查交易周期是否发生变化
	oldInterval := config.IntervalMinutes
	intervalChanged := oldInterval != newTradingConfig.IntervalMinutes

	config.Symbols = newTradingConfig.Symbols
	config.IntervalMinutes = newTradingConfig.IntervalMinutes
	config.MaxDrawdownPercent = newTradingConfig.MaxDrawdownPercent
	config.MaxPositions = newTradingConfig.MaxPositions
	config.MaxLeverage = newTradingConfig.MaxLeverage
	config.MinLeverage = newTradingConfig.MinLeverage
	config.UpdatedAt = time.Now()

	err = s.tradingConfigRepo.UpdateById(ctx, config)
	if err != nil {
		return err
	}

	// 如果交易周期发生变化且交易循环正在运行，则重启交易循环
	if intervalChanged && s.tradingLoop != nil && s.tradingLoop.IsRunning() {
		s.logger.Info("交易周期配置已更新，正在重启交易循环...",
			zap.Int("old_interval", oldInterval),
			zap.Int("new_interval", newTradingConfig.IntervalMinutes))

		// 停止当前的交易循环
		s.tradingLoop.Stop()

		// 启动新的交易循环（使用 goroutine 避免阻塞）
		go func() {
			if err := s.tradingLoop.Start(context.Background()); err != nil {
				s.logger.Error("重启交易循环失败", zap.Error(err))
			}
		}()

		s.logger.Info("交易循环已重启")
	}

	return nil
}

// GetSystemPrompt 获取当前激活的系统提示词
func (s *AdminConfigService) GetSystemPrompt(ctx context.Context) (*models.SystemPrompt, error) {
	prompt, err := s.systemPromptRepo.GetActiveSystemPrompt(ctx)
	if err != nil {
		return nil, err
	}
	return prompt, nil
}

// SetSystemPrompt 设置新的系统提示词(创建新版本并激活)
func (s *AdminConfigService) SetSystemPrompt(ctx context.Context, content, remark string) (*models.SystemPrompt, error) {
	// 获取当前最大版本号
	maxVersion, err := s.systemPromptRepo.GetMaxVersion(ctx)
	if err != nil {
		return nil, err
	}

	// 创建新版本
	newPrompt := models.SystemPrompt{
		ID:       uuid.NewString(),
		Version:  maxVersion + 1,
		Content:  content,
		IsActive: true,
		Remark:   remark,
	}

	// 开启事务:先将所有提示词设为非激活,再创建新版本
	err = s.systemPromptRepo.GetDB(ctx).Transaction(func(tx *gorm.DB) error {
		// 将所有提示词设为非激活
		if err := s.systemPromptRepo.DeactivateAll(ctx); err != nil {
			return err
		}

		// 创建新版本
		if err := s.systemPromptRepo.Create(ctx, &newPrompt); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &newPrompt, nil
}

// GetSystemPromptHistory 获取系统提示词历史记录
func (s *AdminConfigService) GetSystemPromptHistory(ctx context.Context) ([]models.SystemPrompt, error) {
	prompts, err := s.systemPromptRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	// 按版本号降序排序
	sort.Slice(prompts, func(i, j int) bool {
		return prompts[i].Version > prompts[j].Version
	})

	return prompts, nil
}

// RollbackSystemPrompt 回滚到指定版本的系统提示词
func (s *AdminConfigService) RollbackSystemPrompt(ctx context.Context, id string) error {
	// 查找指定的提示词(验证是否存在)
	_, err := s.systemPromptRepo.FindById(ctx, id)
	if err != nil {
		return err
	}

	// 开启事务:先将所有提示词设为非激活,再激活指定版本
	return s.systemPromptRepo.GetDB(ctx).Transaction(func(tx *gorm.DB) error {
		// 将所有提示词设为非激活
		if err := s.systemPromptRepo.DeactivateAll(ctx); err != nil {
			return err
		}

		// 激活指定版本
		if err := s.systemPromptRepo.ActivateById(ctx, id); err != nil {
			return err
		}

		return nil
	})
}

// DeleteSystemPrompt 删除指定的系统提示词历史记录
func (s *AdminConfigService) DeleteSystemPrompt(ctx context.Context, id string) error {
	// 查找指定的提示词(验证是否存在)
	prompt, err := s.systemPromptRepo.FindById(ctx, id)
	if err != nil {
		return err
	}

	// 不允许删除当前激活的提示词
	if prompt.IsActive {
		return gorm.ErrInvalidData
	}

	// 删除提示词
	return s.systemPromptRepo.DeleteById(ctx, id)
}
