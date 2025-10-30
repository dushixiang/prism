package repo

import (
	"context"

	"github.com/dushixiang/prism/internal/models"
	"github.com/go-orz/orz"
	"gorm.io/gorm"
)

func NewLLMLogRepo(db *gorm.DB) *LLMLogRepo {
	return &LLMLogRepo{
		Repository: orz.NewRepository[models.LLMLog, string](db),
	}
}

type LLMLogRepo struct {
	orz.Repository[models.LLMLog, string]
}

// FindByDecisionID 根据决策ID查询所有日志
func (r LLMLogRepo) FindByDecisionID(ctx context.Context, decisionID string) ([]models.LLMLog, error) {
	var logs []models.LLMLog
	db := r.GetDB(ctx)
	err := db.Table(r.GetTableName()).
		Where("decision_id = ?", decisionID).
		Order("round_number ASC").
		Find(&logs).Error
	return logs, err
}

// FindByIteration 根据迭代次数查询所有日志
func (r LLMLogRepo) FindByIteration(ctx context.Context, iteration int) ([]models.LLMLog, error) {
	var logs []models.LLMLog
	db := r.GetDB(ctx)
	err := db.Table(r.GetTableName()).
		Where("iteration = ?", iteration).
		Order("round_number ASC").
		Find(&logs).Error
	return logs, err
}

// FindRecentLogs 获取最近的日志记录
func (r LLMLogRepo) FindRecentLogs(ctx context.Context, limit int) ([]models.LLMLog, error) {
	var logs []models.LLMLog
	db := r.GetDB(ctx)
	err := db.Table(r.GetTableName()).
		Order("executed_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// CountByDecisionID 统计某个决策的日志数量
func (r LLMLogRepo) CountByDecisionID(ctx context.Context, decisionID string) (int64, error) {
	var count int64
	db := r.GetDB(ctx)
	err := db.Table(r.GetTableName()).
		Where("decision_id = ?", decisionID).
		Count(&count).Error
	return count, err
}
