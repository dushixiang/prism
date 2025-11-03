package middleware

import (
	"net/http"
	"strings"

	"github.com/dushixiang/prism/internal/service"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// JWTAuthConfig JWT认证配置
type JWTAuthConfig struct {
	AuthService *service.AuthService
	Logger      *zap.Logger
}

// JWTAuth JWT认证中间件
func JWTAuth(config JWTAuthConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 从Header中获取Token
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				config.Logger.Warn("JWT token missing",
					zap.String("path", c.Request().URL.Path),
					zap.String("remote_ip", c.RealIP()))

				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": "未授权：缺少token",
				})
			}

			// 解析Bearer Token
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": "未授权：token格式错误",
				})
			}

			tokenString := parts[1]

			// 验证Token
			claims, err := config.AuthService.ValidateToken(tokenString)
			if err != nil {
				config.Logger.Warn("invalid JWT token",
					zap.String("path", c.Request().URL.Path),
					zap.String("remote_ip", c.RealIP()),
					zap.Error(err))

				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": "未授权：token无效或已过期",
				})
			}

			// 将用户信息存入Context
			c.Set("user_id", claims.UserID)
			c.Set("username", claims.Username)
			c.Set("role", claims.Role)

			config.Logger.Debug("JWT authenticated",
				zap.String("user_id", claims.UserID),
				zap.String("username", claims.Username),
				zap.String("path", c.Request().URL.Path))

			return next(c)
		}
	}
}
