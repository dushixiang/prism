package repo

import (
	"github.com/dushixiang/prism/internal/models"
	"github.com/go-orz/orz"
	"gorm.io/gorm"
)

func NewLoginLogRepo(db *gorm.DB) *LoginLogRepo {
	return &LoginLogRepo{
		Repository: orz.NewRepository[models.LoginLog, string](db),
	}
}

type LoginLogRepo struct {
	orz.Repository[models.LoginLog, string]
}
