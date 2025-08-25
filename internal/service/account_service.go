package service

import (
	"context"
	"time"

	"github.com/dushixiang/prism/internal/models"
	"github.com/dushixiang/prism/internal/repo"
	"github.com/dushixiang/prism/internal/views"
	"github.com/dushixiang/prism/internal/xe"
	"github.com/dushixiang/prism/pkg/nostd"
	"github.com/go-orz/cache"
	"github.com/go-orz/orz"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func NewAccountService(logger *zap.Logger, db *gorm.DB) *AccountService {
	service := AccountService{
		logger:       logger,
		Service:      orz.NewService(db),
		UserRepo:     repo.NewUserRepo(db),
		SessionRepo:  repo.NewSessionRepo(db),
		LoginLogRepo: repo.NewLoginLogRepo(db),
		otp:          cache.New[string, string](time.Minute),
		sessions:     nil,
	}
	service.sessions = cache.New[string, models.Session](time.Minute, cache.Option[string, models.Session]{
		OnEvicted: service.onEvicted,
	})
	return &service
}

type AccountService struct {
	logger *zap.Logger
	*orz.Service
	UserRepo     *repo.UserRepo
	SessionRepo  *repo.SessionRepo
	LoginLogRepo *repo.LoginLogRepo

	otp      cache.Cache[string, string]
	sessions cache.Cache[string, models.Session]
}

func (s *AccountService) Login(ctx context.Context, account views.LoginAccount) (v *views.LoginResult, err error) {
	defer func() {
		// 存储登录日志
		loginLog := models.LoginLog{
			ID:           uuid.NewString(),
			Account:      account.Account,
			IP:           account.IP,
			UserAgentRaw: account.UserAgent,
			LoginAt:      time.Now().UnixMilli(),
			Success:      true,
			Reason:       "",
			Region:       "",
			UserAgent:    nil,
		}
		if err != nil {
			loginLog.Success = false
			loginLog.Reason = err.Error()
		}
		_ = s.LoginLogRepo.Create(ctx, &loginLog)
	}()

	user, err := s.UserRepo.FindByAccount(ctx, account.Account)
	if err != nil {
		return nil, xe.ErrIncorrectPassword
	}
	if nostd.BcryptMatch([]byte(user.Password), []byte(account.Password)) != nil {
		return nil, orz.NewError(400, "account or password incorrect")
	}

	if !user.IsAdmin() {
		return nil, orz.NewError(400, "you are not admin")
	}

	if !user.Enabled {
		return nil, orz.NewError(400, "account is disabled")
	}

	token, err := s.loginSuccess(ctx, user, account.Remember)
	if err != nil {
		return nil, err
	}

	return &views.LoginResult{
		Token: token,
		OTP:   false,
	}, nil
}

func (s *AccountService) loginSuccess(ctx context.Context, user models.User, remember bool) (string, error) {
	var d = time.Hour
	if remember {
		d = time.Hour * 24
	}
	token := uuid.NewString()
	session := models.Session{
		ID:        token,
		UserId:    user.ID,
		UserType:  user.Type,
		Remember:  remember,
		CreatedAt: time.Now().UnixMilli(),
	}
	s.sessions.Set(token, session, d)
	if err := s.SessionRepo.Create(ctx, &session); err != nil {
		return "", err
	}
	return token, nil
}

func (s *AccountService) Logout(ctx context.Context, token string) error {
	s.sessions.Delete(token)
	return nil
}

func (s *AccountService) AccountId(token string) (string, bool) {
	session, ok := s.sessions.Get(token)
	if ok {
		var d = time.Hour
		if session.Remember {
			d = time.Hour * 24
		}
		s.sessions.Set(token, session, d)
	}
	return session.UserId, ok
}

func (s *AccountService) IsAdmin(token string) bool {
	session, ok := s.sessions.Get(token)
	if ok {
		return session.IsAdmin()
	}
	return false
}

func (s *AccountService) DeleteByUserId(ctx context.Context, userId string) error {
	return s.Transaction(ctx, func(ctx context.Context) error {
		items, err := s.SessionRepo.FindByUserId(ctx, userId)
		if err != nil {
			return err
		}
		var sessionIds = make([]string, 0, len(items))
		for _, item := range items {
			s.sessions.Delete(item.ID)
			sessionIds = append(sessionIds, item.ID)
		}

		return s.SessionRepo.DeleteByIdIn(ctx, sessionIds)
	})
}

func (s *AccountService) onEvicted(token string, session models.Session) {
	ctx := context.Background()
	err := s.SessionRepo.DeleteById(ctx, token)
	if err != nil {
		s.logger.Sugar().Errorf(`delete session err: %v`, err)
	}
}

func (s *AccountService) Init(ctx context.Context) error {
	items, err := s.SessionRepo.FindAll(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		var d = time.Hour
		if item.Remember {
			d = time.Hour * 24
		}
		s.sessions.Set(item.ID, item, d)
	}
	s.logger.Sugar().Infof("reload session count: %v", len(items))
	return nil
}
