package repo

import (
	"context"

	"github.com/dushixiang/prism/internal/models"
	"github.com/go-orz/orz"
	"gorm.io/gorm"
)

func NewSessionRepo(db *gorm.DB) *SessionRepo {
	return &SessionRepo{
		Repository: orz.NewRepository[models.Session, string](db),
	}
}

type SessionRepo struct {
	orz.Repository[models.Session, string]
}

func (r SessionRepo) FindByUserId(ctx context.Context, userId string) (items []models.Session, err error) {
	err = r.GetDB(ctx).Where("user_id = ?", userId).Find(&items).Error
	return
}
