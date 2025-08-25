package repo

import (
	"context"

	"github.com/dushixiang/prism/internal/models"
	"github.com/go-orz/orz"
	"gorm.io/gorm"
)

func NewNewsRepo(db *gorm.DB) *NewsRepo {
	return &NewsRepo{
		Repository: orz.NewRepository[models.News, string](db),
		db:         db,
	}
}

type NewsRepo struct {
	orz.Repository[models.News, string]
	db *gorm.DB
}

// ExistsByOriginalIDAndSource 检查新闻是否已存在（基于原始ID和来源）
func (r *NewsRepo) ExistsByOriginalIDAndSource(ctx context.Context, originalID, source string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.News{}).
		Where("original_id = ? AND source = ?", originalID, source).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// FindLatest 获取最新新闻列表
func (r *NewsRepo) FindLatest(ctx context.Context, limit int) ([]models.News, error) {
	var news []models.News
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Limit(limit).
		Find(&news).Error
	return news, err
}

// FindByTimeRange 根据时间范围获取新闻
func (r *NewsRepo) FindByTimeRange(ctx context.Context, startTime, endTime int64) ([]models.News, error) {
	var news []models.News
	err := r.db.WithContext(ctx).
		Where("created_at BETWEEN ? AND ?", startTime, endTime).
		Order("created_at DESC").
		Find(&news).Error
	return news, err
}

// SearchByKeyword 搜索新闻
func (r *NewsRepo) SearchByKeyword(ctx context.Context, keyword string, limit int) ([]models.News, error) {
	var news []models.News
	err := r.db.WithContext(ctx).
		Where("title LIKE ? OR content LIKE ?", "%"+keyword+"%", "%"+keyword+"%").
		Order("created_at DESC").
		Limit(limit).
		Find(&news).Error
	return news, err
}

// FindBySource 根据新闻源获取新闻
func (r *NewsRepo) FindBySource(ctx context.Context, source string, limit int) ([]models.News, error) {
	var news []models.News
	err := r.db.WithContext(ctx).
		Where("source = ?", source).
		Order("created_at DESC").
		Limit(limit).
		Find(&news).Error
	return news, err
}

// FindBySentiment 根据情绪获取新闻
func (r *NewsRepo) FindBySentiment(ctx context.Context, sentiment string, limit int) ([]models.News, error) {
	var news []models.News
	err := r.db.WithContext(ctx).
		Where("sentiment = ?", sentiment).
		Order("created_at DESC").
		Limit(limit).
		Find(&news).Error
	return news, err
}

// CountToday 统计今日新闻数量
func (r *NewsRepo) CountToday(ctx context.Context, todayStart, todayEnd int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.News{}).
		Where("created_at BETWEEN ? AND ?", todayStart, todayEnd).
		Count(&count).Error
	return count, err
}

// CountBySource 按新闻源统计
func (r *NewsRepo) CountBySource(ctx context.Context, source string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.News{}).
		Where("source = ?", source).
		Count(&count).Error
	return count, err
}

// CountBySentiment 按情绪统计
func (r *NewsRepo) CountBySentiment(ctx context.Context, sentiment string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.News{}).
		Where("sentiment = ?", sentiment).
		Count(&count).Error
	return count, err
}
