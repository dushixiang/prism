package xe

import "github.com/go-orz/orz"

var (
	ErrInvalidParams        = orz.NewError(10400, "参数无效")
	ErrInvalidToken         = orz.NewError(10403, "令牌无效")
	ErrPermissionDenied     = orz.NewError(10401, "您没有权限查看/修改/删除此数据")
	ErrAccountAlreadyUsed   = orz.NewError(10000, "账户已被使用")
	ErrIncorrectPassword    = orz.NewError(10001, "账户或密码错误")
	ErrInvalidAvatar        = orz.NewError(10002, "图片格式不正确或大小超过限制")
	ErrIncorrectOldPassword = orz.NewError(10003, "原密码错误")
	ErrCurrentNotAllowed    = orz.NewError(10004, "当前不允许操作")
	ErrMailProviderExpired  = orz.NewError(10005, "邮件服务商已失效")
	ErrAppNotFound          = orz.NewError(10006, "应用平台不存在")
	ErrAppDisabled          = orz.NewError(10007, "应用平台已被禁用")
	ErrOrgDisabled          = orz.NewError(10008, "您所在的组织已被禁用")

	ErrToolVersionExists = orz.NewError(10009, "该版本已存在")
	ErrNotSupport        = orz.NewError(10010, "尚未支持")
)
