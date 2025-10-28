package repo

import (
	"context"
	"errors"

	"github.com/dushixiang/prism/internal/models"
	"github.com/go-orz/orz"
	"gorm.io/gorm"
)

func NewDecisionRepo(db *gorm.DB) *DecisionRepo {
	return &DecisionRepo{
		Repository: orz.NewRepository[models.Decision, string](db),
	}
}

type DecisionRepo struct {
	orz.Repository[models.Decision, string]
}

// FindRecentDecisions 获取最近的决策记录
func (r DecisionRepo) FindRecentDecisions(ctx context.Context, limit int) ([]models.Decision, error) {
	var decisions []models.Decision
	db := r.GetDB(ctx)
	err := db.Table(r.GetTableName()).
		Order("executed_at DESC").
		Limit(limit).
		Find(&decisions).Error
	return decisions, err
}

// FindLatestIteration 获取最新的迭代编号
func (r DecisionRepo) FindLatestIteration(ctx context.Context) (int, error) {
	var decision models.Decision
	db := r.GetDB(ctx)
	err := db.Table(r.GetTableName()).
		Order("iteration DESC").
		First(&decision).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return decision.Iteration, nil
}
