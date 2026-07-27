package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// 系统与公共端点：健康检查、登录、全局主题配置读取等无需认证的入口。
func init() {
	register(http.MethodGet, "/health", Meta{
		Tag:         "系统",
		TagDesc:     "系统健康检查与基础信息",
		Summary:     "健康检查",
		Description: "检查后端 API、MongoDB、Redis 三个依赖的连通性。\n\n- 三者全部就绪：返回 HTTP 200 与 \"OK\"。\n- 任一不可用：返回 HTTP 503 与对应错误信息。\n\n该端点不经过任何认证中间件，供负载均衡 / 监控系统直接探测。",
		RespType:    "",
		Security:    TierPublic,
	})

	// 登录端点：写入 LoginReq/LoginResp 以便在 UI 上展示请求体与响应结构
	register(http.MethodPost, "/api/v1/login", Meta{
		Tag:         "用户认证",
		TagDesc:     "用户登录与公共配置读取",
		Summary:     "用户登录",
		Description: "用户使用用户名 + 密码登录，成功后返回 JWT Token、用户 ID、角色与当前默认工作空间 ID。\n\n**业务规则**\n\n- 用户必须存在且状态为启用，才能签发 Token。\n- 首次登录或被管理员重置密码后，`needChangePwd=true`，前端必须强制走 `/api/v1/user/firstLoginResetPassword`。\n- 若认证服务（如 MongoDB）暂时不可用，返回真实 HTTP 503 与 `{code:503,msg:\"认证服务暂时不可用\"}`，与登录失败（HTTP 200 + 业务错误码）区分。\n\n**典型错误码**\n\n- 10001 用户不存在\n- 10002 用户名或密码错误\n- 10003 用户已禁用\n- 500 服务器错误",
		ReqType:     "LoginReq",
		RespType:    "LoginResp",
		Security:    TierPublic,
		Errors:      []int{10001, 10002, 10003, 500},
	})

	register(http.MethodPost, "/api/v1/theme/config/get", Meta{
		Tag:         "主题配置",
		TagDesc:     "全局主题（明暗模式 / 主色）配置读取，公开访问",
		Summary:     "读取全局主题配置",
		Description: "返回平台主题模式（light / dark / system）与主色配置。\n\n该端点对所有人开放（无需 Token），用于登录页与未登录访问者展示统一视觉。",
		RespType:    "",
		Security:    TierPublic,
	})

	// 把 LoginReq/RegisterBySample 注册到反射器入口
	RegisterTypes(
		types.LoginReq{},
		types.LoginResp{},
	)
}
