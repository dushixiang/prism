package handler

import (
	"errors"
	"net/http"

	"github.com/dushixiang/prism/internal/service"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	logger      *zap.Logger
	authService *service.AuthService
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(logger *zap.Logger, authService *service.AuthService) *AuthHandler {
	return &AuthHandler{
		logger:      logger,
		authService: authService,
	}
}

// Login 用户登录
// POST /api/auth/login
func (h *AuthHandler) Login(c echo.Context) error {
	ctx := c.Request().Context()

	var req service.LoginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "请求参数错误",
		})
	}

	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "用户名和密码不能为空",
		})
	}

	// 获取客户端IP
	ip := c.RealIP()

	// 执行登录
	resp, err := h.authService.Login(ctx, req, ip)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			return c.JSON(http.StatusUnauthorized, map[string]interface{}{
				"error": "用户名或密码错误",
			})
		}
		if errors.Is(err, service.ErrUserNotActive) {
			return c.JSON(http.StatusForbidden, map[string]interface{}{
				"error": "用户已被禁用",
			})
		}

		h.logger.Error("login failed", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "登录失败，请稍后重试",
		})
	}

	return c.JSON(http.StatusOK, resp)
}

// GetCurrentUser 获取当前用户信息
// GET /api/auth/me
func (h *AuthHandler) GetCurrentUser(c echo.Context) error {
	ctx := c.Request().Context()

	// 从Context中获取用户ID（由JWT中间件设置）
	userID := c.Get("user_id").(string)

	user, err := h.authService.GetCurrentUser(ctx, userID)
	if err != nil {
		h.logger.Error("failed to get current user", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "获取用户信息失败",
		})
	}

	return c.JSON(http.StatusOK, user)
}

// ChangePassword 修改密码
// POST /api/auth/change-password
func (h *AuthHandler) ChangePassword(c echo.Context) error {
	ctx := c.Request().Context()

	var req struct {
		OldPassword string `json:"old_password" validate:"required"`
		NewPassword string `json:"new_password" validate:"required,min=5"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "请求参数错误",
		})
	}

	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "密码长度至少5位",
		})
	}

	// 从Context中获取用户ID
	userID := c.Get("user_id").(string)

	if err := h.authService.ChangePassword(ctx, userID, req.OldPassword, req.NewPassword); err != nil {
		h.logger.Error("failed to change password",
			zap.String("user_id", userID),
			zap.Error(err))

		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": err.Error(),
		})
	}

	h.logger.Info("password changed successfully", zap.String("user_id", userID))

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "密码修改成功",
	})
}

// RegisterRoutes 注册路由
func (h *AuthHandler) RegisterRoutes(g *echo.Group) {
	auth := g.Group("/auth")

	// 公开接口（无需认证）
	auth.POST("/login", h.Login)

	// 需要认证的接口（由外部添加中间件）
	// auth.GET("/me", h.GetCurrentUser) - 在app.go中注册并添加中间件
	// auth.POST("/change-password", h.ChangePassword)
}

// RegisterProtectedRoutes 注册需要认证的路由
func (h *AuthHandler) RegisterProtectedRoutes(g *echo.Group) {
	g.GET("/me", h.GetCurrentUser)
	g.POST("/change-password", h.ChangePassword)
}
