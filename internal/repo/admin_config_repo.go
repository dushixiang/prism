package repo

import (
	"context"

	"github.com/dushixiang/prism/internal/models"
	"github.com/go-orz/orz"
	"gorm.io/gorm"
)

type TradingConfigRepo struct {
	orz.Repository[models.TradingConfig, string]
}

func NewTradingConfigRepo(db *gorm.DB) *TradingConfigRepo {
	return &TradingConfigRepo{
		Repository: orz.NewRepository[models.TradingConfig, string](db),
	}
}

type SystemPromptRepo struct {
	orz.Repository[models.SystemPrompt, string]
}

func NewSystemPromptRepo(db *gorm.DB) *SystemPromptRepo {
	return &SystemPromptRepo{
		Repository: orz.NewRepository[models.SystemPrompt, string](db),
	}
}

func (r *SystemPromptRepo) GetActiveSystemPrompt(ctx context.Context) (*models.SystemPrompt, error) {
	var prompt models.SystemPrompt
	err := r.GetDB(ctx).WithContext(ctx).
		Where("is_active = ?", true).
		Order("version DESC").
		First(&prompt).Error
	if err != nil {
		return nil, err
	}
	return &prompt, nil
}

// GetMaxVersion 获取当前最大版本号
func (r *SystemPromptRepo) GetMaxVersion(ctx context.Context) (int, error) {
	var maxVersion int
	err := r.GetDB(ctx).WithContext(ctx).
		Model(&models.SystemPrompt{}).
		Select("COALESCE(MAX(version), 0)").
		Scan(&maxVersion).Error
	if err != nil {
		return 0, err
	}
	return maxVersion, nil
}

// DeactivateAll 将所有提示词设为非激活状态
func (r *SystemPromptRepo) DeactivateAll(ctx context.Context) error {
	return r.GetDB(ctx).WithContext(ctx).
		Model(&models.SystemPrompt{}).
		Where("is_active = ?", true).
		Update("is_active", false).Error
}

// ActivateById 激活指定ID的提示词
func (r *SystemPromptRepo) ActivateById(ctx context.Context, id string) error {
	return r.GetDB(ctx).WithContext(ctx).
		Model(&models.SystemPrompt{}).
		Where("id = ?", id).
		Update("is_active", true).Error
}
