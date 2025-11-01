package repo

import (
	"context"

	"github.com/dushixiang/prism/internal/models"
	"github.com/go-orz/orz"
	"gorm.io/gorm"
)

func NewOrderRepo(db *gorm.DB) *OrderRepo {
	return &OrderRepo{
		Repository: orz.NewRepository[models.Order, string](db),
	}
}

type OrderRepo struct {
	orz.Repository[models.Order, string]
}

// FindActiveByPositionID 查找指定持仓的所有活跃订单
func (r OrderRepo) FindActiveByPositionID(ctx context.Context, positionID string) ([]models.Order, error) {
	db := r.GetDB(ctx)
	var orders []models.Order
	err := db.Table(r.GetTableName()).
		Where("position_id = ? AND status = ?", positionID, models.OrderStatusActive).
		Order("created_at DESC").
		Find(&orders).Error
	return orders, err
}

// FindActiveBySymbol 查找指定交易对的所有活跃订单
func (r OrderRepo) FindActiveBySymbol(ctx context.Context, symbol string) ([]models.Order, error) {
	db := r.GetDB(ctx)
	var orders []models.Order
	err := db.Table(r.GetTableName()).
		Where("symbol = ? AND status = ?", symbol, models.OrderStatusActive).
		Order("created_at DESC").
		Find(&orders).Error
	return orders, err
}

// FindAllActive 查找所有活跃订单
func (r OrderRepo) FindAllActive(ctx context.Context) ([]models.Order, error) {
	db := r.GetDB(ctx)
	var orders []models.Order
	err := db.Table(r.GetTableName()).
		Where("status = ?", models.OrderStatusActive).
		Order("created_at DESC").
		Find(&orders).Error
	return orders, err
}

// CancelByPositionID 取消指定持仓的所有活跃订单
func (r OrderRepo) CancelByPositionID(ctx context.Context, positionID string) error {
	db := r.GetDB(ctx)
	return db.Table(r.GetTableName()).
		Where("position_id = ? AND status = ?", positionID, models.OrderStatusActive).
		Updates(map[string]interface{}{
			"status":      models.OrderStatusCanceled,
			"canceled_at": gorm.Expr("NOW()"),
		}).Error
}

// CancelBySymbol 取消指定交易对的所有活跃订单
func (r OrderRepo) CancelBySymbol(ctx context.Context, symbol string) error {
	db := r.GetDB(ctx)
	return db.Table(r.GetTableName()).
		Where("symbol = ? AND status = ?", symbol, models.OrderStatusActive).
		Updates(map[string]interface{}{
			"status":      models.OrderStatusCanceled,
			"canceled_at": gorm.Expr("NOW()"),
		}).Error
}

// UpdateStatus 更新订单状态
func (r OrderRepo) UpdateStatus(ctx context.Context, id string, status models.OrderStatus) error {
	db := r.GetDB(ctx)
	updates := map[string]interface{}{
		"status": status,
	}

	// 根据状态添加时间戳
	if status == models.OrderStatusTriggered {
		updates["triggered_at"] = gorm.Expr("NOW()")
	} else if status == models.OrderStatusCanceled {
		updates["canceled_at"] = gorm.Expr("NOW()")
	}

	return db.Table(r.GetTableName()).
		Where("id = ?", id).
		Updates(updates).Error
}

// DeleteAll 删除所有订单记录
func (r OrderRepo) DeleteAll(ctx context.Context) error {
	db := r.GetDB(ctx)
	return db.Where("1 = 1").Delete(&models.Order{}).Error
}
