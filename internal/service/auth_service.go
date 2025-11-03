package service

import (
	"context"
	"errors"
	"time"

	"github.com/dushixiang/prism/internal/models"
	"github.com/dushixiang/prism/internal/repo"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrUserNotActive      = errors.New("用户已被禁用")
	ErrUserExists         = errors.New("用户名已存在")
)

// AuthService 认证服务
type AuthService struct {
	logger        *zap.Logger
	userRepo      *repo.AdminUserRepo
	jwtSecret     string
	jwtExpiration time.Duration
}

// NewAuthService 创建认证服务
func NewAuthService(logger *zap.Logger, db *gorm.DB, jwtSecret string) *AuthService {
	if jwtSecret == "" {
		jwtSecret = uuid.NewString()
	}
	return &AuthService{
		logger:        logger,
		userRepo:      repo.NewAdminUserRepo(db),
		jwtSecret:     jwtSecret,
		jwtExpiration: 24 * time.Hour, // JWT有效期24小时
	}
}

// JWTClaims JWT载荷
type JWTClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      UserInfo  `json:"user"`
}

// UserInfo 用户信息
type UserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Role     string `json:"role"`
}

// Login 用户登录
func (s *AuthService) Login(ctx context.Context, req LoginRequest, ip string) (*LoginResponse, error) {
	// 查找用户
	user, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			s.logger.Warn("login failed: user not found",
				zap.String("username", req.Username),
				zap.String("ip", ip))
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// 检查用户是否激活
	if !user.IsActive {
		s.logger.Warn("login failed: user not active",
			zap.String("username", req.Username),
			zap.String("ip", ip))
		return nil, ErrUserNotActive
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		s.logger.Warn("login failed: invalid password",
			zap.String("username", req.Username),
			zap.String("ip", ip))
		return nil, ErrInvalidCredentials
	}

	// 更新最后登录信息
	if err := s.userRepo.UpdateLastLogin(ctx, user.ID, ip); err != nil {
		s.logger.Error("failed to update last login", zap.Error(err))
	}

	// 生成JWT Token
	expiresAt := time.Now().Add(s.jwtExpiration)
	claims := JWTClaims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "prism",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, err
	}

	s.logger.Info("user logged in",
		zap.String("username", user.Username),
		zap.String("ip", ip))

	return &LoginResponse{
		Token:     tokenString,
		ExpiresAt: expiresAt,
		User: UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Nickname: user.Nickname,
			Role:     user.Role,
		},
	}, nil
}

// ValidateToken 验证JWT Token
func (s *AuthService) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// CreateUser 创建用户（初始化时使用）
func (s *AuthService) CreateUser(ctx context.Context, username, password, nickname, role string) error {
	// 检查用户是否已存在
	_, err := s.userRepo.FindByUsername(ctx, username)
	if err == nil {
		return ErrUserExists
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	// 加密密码
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 创建用户
	user := &models.AdminUser{
		ID:           ulid.Make().String(),
		Username:     username,
		PasswordHash: string(passwordHash),
		Nickname:     nickname,
		Role:         role,
		IsActive:     true,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return err
	}

	s.logger.Info("user created", zap.String("username", username))
	return nil
}

// ChangePassword 修改密码
func (s *AuthService) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return errors.New("旧密码错误")
	}

	// 加密新密码
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 更新密码
	if err := s.userRepo.UpdatePassword(ctx, userID, string(passwordHash)); err != nil {
		return err
	}

	s.logger.Info("password changed", zap.String("user_id", userID))
	return nil
}

// GetCurrentUser 获取当前用户信息
func (s *AuthService) GetCurrentUser(ctx context.Context, userID string) (*UserInfo, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &UserInfo{
		ID:       user.ID,
		Username: user.Username,
		Nickname: user.Nickname,
		Role:     user.Role,
	}, nil
}

// NeedsSetup 检查系统是否需要初始化设置（是否存在管理员用户）
func (s *AuthService) NeedsSetup(ctx context.Context) (bool, error) {
	count, err := s.userRepo.CountUsers(ctx)
	if err != nil {
		return false, err
	}

	// 如果没有任何用户，需要初始化设置
	return count == 0, nil
}
