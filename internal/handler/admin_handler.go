package handler

import (
	"net/http"

	"github.com/dushixiang/prism/internal/models"
	"github.com/dushixiang/prism/internal/service"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// AdminHandler 管理员处理器
type AdminHandler struct {
	logger             *zap.Logger
	adminConfigService *service.AdminConfigService
}

// NewAdminHandler 创建管理员处理器
func NewAdminHandler(
	logger *zap.Logger,
	adminConfigService *service.AdminConfigService,
) *AdminHandler {
	return &AdminHandler{
		logger:             logger,
		adminConfigService: adminConfigService,
	}
}

// GetTradingConfig 获取交易配置
// GET /api/admin/trading-config
func (h *AdminHandler) GetTradingConfig(c echo.Context) error {
	ctx := c.Request().Context()

	config, err := h.adminConfigService.GetTradingConfig(ctx)
	if err != nil {
		h.logger.Error("failed to get trading config", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, config)
}

// SetTradingConfig 更新交易配置
// PUT /api/admin/trading-config
func (h *AdminHandler) SetTradingConfig(c echo.Context) error {
	ctx := c.Request().Context()

	var tradingConfig models.TradingConfig

	if err := c.Bind(&tradingConfig); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "invalid request body",
		})
	}

	if err := h.adminConfigService.SetTradingConfig(ctx, tradingConfig); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "update success",
	})
}

// GetSystemPrompt 获取当前激活的系统提示词
// GET /api/admin/system-prompt
func (h *AdminHandler) GetSystemPrompt(c echo.Context) error {
	ctx := c.Request().Context()

	prompt, err := h.adminConfigService.GetSystemPrompt(ctx)
	if err != nil {
		h.logger.Error("failed to get system prompt", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, prompt)
}

// SetSystemPrompt 更新系统提示词(创建新版本)
// PUT /api/admin/system-prompt
func (h *AdminHandler) SetSystemPrompt(c echo.Context) error {
	ctx := c.Request().Context()

	var req struct {
		Content string `json:"content"`
		Remark  string `json:"remark"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "invalid request body",
		})
	}

	if req.Content == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "content is required",
		})
	}

	prompt, err := h.adminConfigService.SetSystemPrompt(ctx, req.Content, req.Remark)
	if err != nil {
		h.logger.Error("failed to set system prompt", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, prompt)
}

// GetSystemPromptHistory 获取系统提示词历史记录
// GET /api/admin/system-prompt/history
func (h *AdminHandler) GetSystemPromptHistory(c echo.Context) error {
	ctx := c.Request().Context()

	prompts, err := h.adminConfigService.GetSystemPromptHistory(ctx)
	if err != nil {
		h.logger.Error("failed to get system prompt history", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, prompts)
}

// RollbackSystemPrompt 回滚到指定版本的系统提示词
// GET /api/admin/system-prompt/history/:id/rollback
func (h *AdminHandler) RollbackSystemPrompt(c echo.Context) error {
	ctx := c.Request().Context()
	id := c.Param("id")

	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "id is required",
		})
	}

	err := h.adminConfigService.RollbackSystemPrompt(ctx, id)
	if err != nil {
		h.logger.Error("failed to rollback system prompt", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "rollback success",
	})
}

// RegisterRoutesWithGroup 注册路由到指定的组（支持中间件）
func (h *AdminHandler) RegisterRoutesWithGroup(admin *echo.Group) {
	// 通用配置接口
	admin.GET("/trading-config", h.GetTradingConfig)
	admin.PUT("/trading-config", h.SetTradingConfig)

	admin.GET("/system-prompt", h.GetSystemPrompt)
	admin.PUT("/system-prompt", h.SetSystemPrompt)

	admin.GET("/system-prompt/history", h.GetSystemPromptHistory)
	admin.GET("/system-prompt/history/:id/rollback", h.RollbackSystemPrompt)
	admin.DELETE("/system-prompt/history/:id", h.DeleteSystemPromptHistory)
}

// DeleteSystemPromptHistory 删除系统提示词历史记录
// DELETE /api/admin/system-prompt/history/:id
func (h *AdminHandler) DeleteSystemPromptHistory(c echo.Context) error {
	ctx := c.Request().Context()
	id := c.Param("id")

	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "id is required",
		})
	}

	err := h.adminConfigService.DeleteSystemPrompt(ctx, id)
	if err != nil {
		h.logger.Error("failed to delete system prompt", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "delete success",
	})
}
