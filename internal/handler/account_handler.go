package handler

import (
	"github.com/dushixiang/prism/internal/service"
	"github.com/dushixiang/prism/internal/views"
	"github.com/dushixiang/prism/pkg/nostd"
	"github.com/go-orz/orz"
	"github.com/labstack/echo/v4"
)

func NewAccountHandler(accountService *service.AccountService, userService *service.UserService) *AccountHandler {
	return &AccountHandler{
		accountService: accountService,
		userService:    userService,
	}
}

type AccountHandler struct {
	accountService *service.AccountService
	userService    *service.UserService
}

func (r AccountHandler) Login(c echo.Context) error {
	var account views.LoginAccount
	if err := c.Bind(&account); err != nil {
		return err
	}
	if err := c.Validate(&account); err != nil {
		return err
	}
	ctx := c.Request().Context()
	account.IP = c.RealIP()
	account.UserAgent = c.Request().UserAgent()

	result, err := r.accountService.Login(ctx, account)
	if err != nil {
		return err
	}
	return orz.Ok(c, result)
}

func (r AccountHandler) Logout(c echo.Context) error {
	ctx := c.Request().Context()
	token := nostd.GetToken(c)
	return r.accountService.Logout(ctx, token)
}

func (r AccountHandler) AccountId(c echo.Context) string {
	token := nostd.GetToken(c)
	accountId, ok := r.accountService.AccountId(token)
	if !ok {
		return ""
	}
	return accountId
}

func (r AccountHandler) Info(c echo.Context) error {
	token := nostd.GetToken(c)
	ctx := c.Request().Context()

	userId, _ := r.accountService.AccountId(token)
	item, err := r.userService.FindById(ctx, userId)
	if err != nil {
		return err
	}
	return orz.Ok(c, item)
}

func (r AccountHandler) ChangeProfile(c echo.Context) error {
	var item views.ChangeProfile
	if err := c.Bind(&item); err != nil {
		return err
	}
	token := nostd.GetToken(c)
	userId, _ := r.accountService.AccountId(token)
	ctx := c.Request().Context()
	return r.userService.ChangeProfile(ctx, userId, item)
}

func (r AccountHandler) ChangePassword(c echo.Context) error {
	var cp views.ChangePassword
	if err := c.Bind(&cp); err != nil {
		return err
	}
	token := nostd.GetToken(c)
	userId, _ := r.accountService.AccountId(token)
	ctx := c.Request().Context()
	err := r.userService.ChangePasswordBySelf(ctx, userId, cp)
	if err != nil {
		return err
	}
	return r.Logout(c)
}
