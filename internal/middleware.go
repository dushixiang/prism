package internal

import (
	"github.com/dushixiang/prism/internal/xe"
	"github.com/dushixiang/prism/pkg/nostd"
	"github.com/labstack/echo/v4"
)

func (r *PrismApp) AccountId(c echo.Context) string {
	token := nostd.GetToken(c)
	accountId, _ := r.container.AccountService.AccountId(token)
	return accountId
}

func (r *PrismApp) IsAdmin(c echo.Context) bool {
	token := nostd.GetToken(c)
	return r.container.AccountService.IsAdmin(token)
}

func (r *PrismApp) Admin(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if r.IsAdmin(c) {
			return next(c)
		}
		return xe.ErrPermissionDenied
	}
}

func (r *PrismApp) Auth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if r.AccountId(c) != "" {
			return next(c)
		}
		return xe.ErrInvalidToken
	}
}
