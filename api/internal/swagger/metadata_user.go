package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// 用户管理分组：包含用户 CRUD、个人中心、个人 API Token、头像上传、扫描配置。
// 涵盖三类鉴权：
//   - TierAuth：普通登录用户（个人中心、Token 创建/列表/吊销、扫描配置、头像）
//   - TierAdmin：用户 create/update/delete（敏感写操作）
func init() {
	tag := "用户管理"
	tagDesc := "用户账号、个人中心、个人 API Token 与扫描配置"

	// ===== 普通登录用户（TierAuth） =====
	register(http.MethodPost, "/api/v1/user/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "用户列表",
		Description: "分页返回所有用户基本信息（不含密码）。\n\n**字段说明**\n\n- `page` / `pageSize`：分页参数，默认 1 / 20。\n- 响应 `list` 中每项含 `id / username / role / status / avatar`。\n\n**典型错误码**\n\n- 401 未授权\n- 500 服务器错误",
		ReqType:     "PageReq",
		RespType:    "UserListResp",
		Security:    TierAuth,
		Errors:      []int{500},
	})

	register(http.MethodPost, "/api/v1/user/firstLoginResetPassword", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "首次登录重置密码",
		Description: "首次登录或被管理员重置密码后强制修改密码，不需要验证旧密码。\n\n**业务规则**\n\n- 仅在用户 `needChangePwd=true` 时可调用。\n- 成功后强制下次登录使用新密码。",
		ReqType:     "UserFirstLoginResetPasswordReq",
		Security:    TierAuth,
		Errors:      []int{10001, 500},
	})

	register(http.MethodPost, "/api/v1/user/resetPassword", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "用户修改密码",
		Description: "已登录用户验证旧密码后修改为新密码。\n\n**业务规则**\n\n- 需要旧密码正确，否则返回 10002。\n- 成功后当前 Token 不立即失效（仍可继续完成本次会话）。",
		ReqType:     "UserResetPasswordReq",
		Security:    TierAuth,
		Errors:      []int{10001, 10002, 500},
	})

	register(http.MethodPost, "/api/v1/user/scanConfig/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存个人扫描配置",
		Description: "保存当前登录用户的扫描配置（自由 JSON 字符串，前端约定结构）。\n\n- `config` 为字符串化的 JSON，长度上限以 `MaxBytes` 为准。",
		ReqType:     "SaveScanConfigReq",
		Security:    TierAuth,
		Errors:      []int{500},
	})

	register(http.MethodPost, "/api/v1/user/scanConfig/get", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "获取个人扫描配置",
		Description: "返回当前登录用户的扫描配置 JSON，若未保存过则返回空字符串。",
		RespType:    "GetScanConfigResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/user/avatar/upload", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "上传头像",
		Description: "上传头像图片文件，服务端落盘到 `/static/avatars/` 并返回可访问 URL。\n\n- 仅接受常见图片格式（PNG / JPEG / SVG）。\n- 单文件大小受 `MaxBytes` 限制。",
		RespType:    "AvatarUploadResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/user/avatar/update", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "更新头像",
		Description: "把之前上传的头像 URL 绑定到当前登录用户。",
		ReqType:     "UserUpdateAvatarReq",
		Security:    TierAuth,
	})

	// ===== 个人中心 =====
	register(http.MethodPost, "/api/v1/user/profile/get", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "获取个人信息",
		Description: "返回当前登录用户的个人信息：邮箱、手机、头像、角色、状态、最后登录时间与创建时间。",
		RespType:    "UserProfileGetResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/user/profile/update", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "更新个人信息",
		Description: "更新当前登录用户的个人信息：用户名、邮箱、手机、头像。所有字段均可选，留空表示不修改。",
		ReqType:     "UserProfileUpdateReq",
		Security:    TierAuth,
		Errors:      []int{500},
	})

	register(http.MethodPost, "/api/v1/user/password/change", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "修改密码",
		Description: "验证旧密码后设为新密码。响应 `mustReLogin` 为 true 时，前端必须强制重新登录并可吊销所有 PAT。",
		ReqType:     "UserPasswordChangeReq",
		RespType:    "UserPasswordChangeResp",
		Security:    TierAuth,
		Errors:      []int{10001, 10002, 500},
	})

	// ===== 个人 API Token =====
	register(http.MethodPost, "/api/v1/user/token/create", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "创建个人 API Token",
		Description: "创建一个个人访问令牌（PAT），用于替代 JWT 调用 `/api/v1/*` 业务接口。\n\n- `name`：可读名称，便于审计。\n- `expiresAt`：unix 秒，0 表示永久。\n- `scopes`：可选授权作用域，空或 `\"*\"` 表示全量；否则按 `<group>:<action>` 粒度授权。\n\n**业务规则**\n\n- 创建成功后 `token` 返回明文，可在个人中心随时查看。\n- 单用户 Token 数量有上限，超过返回 10007。",
		ReqType:     "UserTokenCreateReq",
		RespType:    "UserTokenCreateResp",
		Security:    TierAuth,
		Errors:      []int{10004, 10007, 10009, 500},
	})

	register(http.MethodPost, "/api/v1/user/token/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "列出个人 API Token",
		Description: "返回当前登录用户已创建的全部 PAT（含明文 token、`prefix`、`scopes`、最近使用时间/IP、状态等）。",
		RespType:    "UserTokenListResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/user/token/scopes", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "列出可用 Token 作用域",
		Description: "返回创建 Token 时可选择的全部作用域分组（20 组 × 4 动作），供前端按分组水平展示复选框矩阵。",
		RespType:    "UserTokenScopeListResp",
		Security:    TierAuth,
	})

	// ===== 管理员（TierAdmin） =====
	register(http.MethodPost, "/api/v1/user/create", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "创建用户",
		Description: "管理员创建新用户。\n\n**字段约束**\n\n- `username`：唯一登录名。\n- `password`：初始密码（必填）。\n- `role`：可选，缺省为 `user`。\n- `status`：`enable` / `disable`。\n- `avatar`：可选，相对 URL。\n\n**典型错误码**\n\n- 400 参数校验失败\n- 401 / 403 非管理员\n- 500 服务器错误",
		ReqType:     "UserCreateReq",
		Security:    TierAdmin,
		Errors:      []int{400, 500},
	})

	register(http.MethodPost, "/api/v1/user/update", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "更新用户",
		Description: "管理员更新用户基本信息（用户名、角色、状态、头像）。`id` 必填。",
		ReqType:     "UserUpdateReq",
		Security:    TierAdmin,
		Errors:      []int{10001, 400, 500},
	})

	register(http.MethodPost, "/api/v1/user/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除用户",
		Description: "按 `id` 删除用户。删除后该用户所有 PAT 同时失效。",
		ReqType:     "UserDeleteReq",
		Security:    TierAdmin,
		Errors:      []int{10001, 500},
	})

	// 反射器入口：把本分组用到的所有 types 结构体登记一次
	RegisterTypes(
		types.PageReq{},
		types.UserListResp{},
		types.UserInfo{},
		types.UserFirstLoginResetPasswordReq{},
		types.UserResetPasswordReq{},
		types.SaveScanConfigReq{},
		types.GetScanConfigResp{},
		types.AvatarUploadResp{},
		types.UserUpdateAvatarReq{},
		types.UserProfileGetResp{},
		types.UserProfileUpdateReq{},
		types.UserPasswordChangeReq{},
		types.UserPasswordChangeResp{},
		types.UserTokenCreateReq{},
		types.UserTokenCreateResp{},
		types.UserTokenListResp{},
		types.UserTokenListItem{},
		types.UserTokenScopeListResp{},
		types.UserTokenScopeItem{},
		types.UserTokenScopeGroup{},
		types.UserCreateReq{},
		types.UserUpdateReq{},
		types.UserDeleteReq{},
	)
}
