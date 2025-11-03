package handler

import (
	"net/http"

	"github.com/dushixiang/prism/internal/service"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// SetupHandler 首次设置处理器
type SetupHandler struct {
	logger      *zap.Logger
	authService *service.AuthService
}

// NewSetupHandler 创建设置处理器
func NewSetupHandler(logger *zap.Logger, authService *service.AuthService) *SetupHandler {
	return &SetupHandler{
		logger:      logger,
		authService: authService,
	}
}

// CheckSetupStatus 检查是否需要初始化设置
// GET /api/setup/status
func (h *SetupHandler) CheckSetupStatus(c echo.Context) error {
	ctx := c.Request().Context()

	needsSetup, err := h.authService.NeedsSetup(ctx)
	if err != nil {
		h.logger.Error("failed to check setup status", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "检查设置状态失败",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"needs_setup": needsSetup,
	})
}

// InitialSetup 初始化设置（创建第一个管理员）
// POST /api/setup/init
func (h *SetupHandler) InitialSetup(c echo.Context) error {
	ctx := c.Request().Context()

	// 先检查是否已经初始化
	needsSetup, err := h.authService.NeedsSetup(ctx)
	if err != nil {
		h.logger.Error("failed to check setup status", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "检查设置状态失败",
		})
	}

	if !needsSetup {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "系统已经初始化，无法重复设置",
		})
	}

	var req struct {
		Username string `json:"username" validate:"required"`
		Password string `json:"password" validate:"required,min=5"`
		Nickname string `json:"nickname"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "请求参数错误",
		})
	}

	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "用户名和密码不能为空，密码长度至少5位",
		})
	}

	// 如果没有提供昵称，使用用户名
	nickname := req.Nickname
	if nickname == "" {
		nickname = req.Username
	}

	// 创建管理员用户
	if err := h.authService.CreateUser(ctx, req.Username, req.Password, nickname, "admin"); err != nil {
		h.logger.Error("failed to create admin user", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "创建管理员用户失败: " + err.Error(),
		})
	}

	h.logger.Info("initial admin user created",
		zap.String("username", req.Username),
		zap.String("ip", c.RealIP()))

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "初始化设置成功",
		"user": map[string]interface{}{
			"username": req.Username,
			"nickname": nickname,
			"role":     "admin",
		},
	})
}

// RegisterRoutes 注册路由
func (h *SetupHandler) RegisterRoutes(g *echo.Group) {
	setup := g.Group("/setup")

	// 公开接口（无需认证）
	setup.GET("/status", h.CheckSetupStatus)
	setup.POST("/init", h.InitialSetup)
}
