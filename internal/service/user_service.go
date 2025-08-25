package service

import (
	"context"
	"strings"
	"time"

	"github.com/dushixiang/prism/internal/models"
	"github.com/dushixiang/prism/internal/repo"
	"github.com/dushixiang/prism/internal/tools"
	"github.com/dushixiang/prism/internal/views"
	"github.com/dushixiang/prism/internal/xe"
	"github.com/dushixiang/prism/pkg/nostd"
	"github.com/go-orz/orz"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func NewUserService(logger *zap.Logger, db *gorm.DB, accountService *AccountService) *UserService {
	return &UserService{
		logger:         logger,
		Service:        orz.NewService(db),
		UserRepo:       repo.NewUserRepo(db),
		accountService: accountService,
	}
}

type UserService struct {
	logger *zap.Logger

	*orz.Service
	*repo.UserRepo

	accountService *AccountService
}

func (r UserService) Create(ctx context.Context, req views.UserCreateRequest) error {
	return r.Transaction(ctx, func(ctx context.Context) error {
		exist, err := r.UserRepo.ExistByAccount(ctx, req.Account)
		if err != nil {
			return err
		}
		if exist {
			return xe.ErrAccountAlreadyUsed
		}
		password := req.Password

		var pass []byte
		if pass, err = nostd.BcryptEncode([]byte(password)); err != nil {
			return err
		}

		if req.ID == "" {
			req.ID = uuid.New().String()
		}
		var user = models.User{
			ID:        req.ID,
			Name:      req.Name,
			Account:   req.Account,
			Password:  string(pass),
			Avatar:    req.Avatar,
			CreatedAt: time.Now().UnixMilli(),
			Enabled:   true,
			Type:      req.Type,
		}
		if err := r.UserRepo.Create(ctx, &user); err != nil {
			return err
		}
		return nil
	})
}

func (r UserService) UpdateById(ctx context.Context, user models.User) error {
	return r.Transaction(ctx, func(ctx context.Context) error {
		dbUser, err := r.UserRepo.FindById(ctx, user.ID)
		if err != nil {
			return err
		}

		if dbUser.Account != user.Account {
			// 修改了登录账号
			exist, err := r.UserRepo.ExistByAccount(ctx, user.Account)
			if err != nil {
				return err
			}
			if exist {
				return xe.ErrAccountAlreadyUsed
			}
		}
		return r.UserRepo.UpdateById(ctx, &user)
	})
}

func (r UserService) DeleteById(ctx context.Context, id string) error {
	return r.Transaction(ctx, func(ctx context.Context) error {
		if err := r.accountService.DeleteByUserId(ctx, id); err != nil {
			return err
		}
		return r.UserRepo.DeleteById(ctx, id)
	})
}

func (r UserService) ChangePassword(ctx context.Context, id string, password string) (models.User, error) {
	item, err := r.UserRepo.FindById(ctx, id)
	if err != nil {
		return models.User{}, err
	}
	return item, r.Transaction(ctx, func(ctx context.Context) error {
		var pass []byte
		if pass, err = nostd.BcryptEncode([]byte(password)); err != nil {
			return err
		}
		item.Password = string(pass)
		return r.UserRepo.UpdateById(ctx, &item)
	})
}

func (r UserService) ChangePasswordBySelf(ctx context.Context, id string, cp views.ChangePassword) error {
	item, err := r.UserRepo.FindById(ctx, id)
	if err != nil {
		return err
	}
	err = nostd.BcryptMatch([]byte(item.Password), []byte(cp.OldPassword))
	if err != nil {
		return xe.ErrIncorrectOldPassword
	}
	_, err = r.ChangePassword(ctx, id, cp.NewPassword)
	return err
}

func (r UserService) ChangeProfile(ctx context.Context, id string, item views.ChangeProfile) error {
	if item.Avatar != "" {
		var ok = false
		parts := strings.Split(item.Avatar, ",")
		if len(parts) == 2 {
			ok = tools.IsBase64ImageAndSizeLessThan(parts[1], 1024*256)
		}

		if !ok {
			return xe.ErrInvalidAvatar
		}
	}

	user := models.User{
		ID:     id,
		Name:   item.Name,
		Avatar: item.Avatar,
	}
	return r.UserRepo.UpdateById(ctx, &user)
}

func (r UserService) UpdateEnabledByIdIn(ctx context.Context, enabled bool, ids []string) error {
	return r.Transaction(ctx, func(ctx context.Context) error {
		// 如果是禁用，要下线用户
		if !enabled {
			for _, id := range ids {
				if err := r.accountService.DeleteByUserId(ctx, id); err != nil {
					return err
				}
			}
		}
		return r.UserRepo.UpdateEnabledByIdIn(ctx, enabled, ids)
	})
}
