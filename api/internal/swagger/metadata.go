package swagger

import "strings"

// AuthTier 描述路由的鉴权层级，用于决定 OpenAPI 的 security 字段与 401/403 响应。
type AuthTier string

const (
	TierPublic    AuthTier = "public"    // 无认证：/health、登录、Worker 安装相关
	TierWorker    AuthTier = "worker"   // Worker Install Key 认证
	TierAuth      AuthTier = "auth"     // JWT 认证（普通用户即可）
	TierAdmin     AuthTier = "admin"    // JWT + 管理员权限
	TierConsole   AuthTier = "console"  // JWT + 管理员权限 + 控制台权限
	TierTerminal  AuthTier = "terminal" // 终端 WebSocket，URL 参数 token
	TierContainer AuthTier = "container" // 容器日志 SSE，URL 参数 token
)

// Meta 描述单个端点的人工元数据。
type Meta struct {
	Tag         string // 中文分组名（如 "用户管理"）
	TagDesc     string // 分组的中文说明（仅在每个分组第一次出现时使用）
	Summary     string // 一句话中文标题
	Description string // 多行中文说明，可含字段约束、业务规则
	ReqType     string // types 包内请求结构体名；空表示无 body
	RespType    string // types 包内响应结构体名；空表示无具名响应
	Security    AuthTier
	Errors      []int // xerr 错误码列表
	Deprecated  bool
}

// metadata 端点元数据表，key = "METHOD:/api/v1/xxx"。
var metadata = map[string]Meta{}

// register 由各 metadata_xxx.go 的 init() 调用，把端点元数据登记到表中。
func register(method, path string, m Meta) {
	key := methodKey(method, path)
	metadata[key] = m
}

// LookupMeta 按 method+path 查询端点元数据，第二个返回值表示是否命中。
func LookupMeta(method, path string) (Meta, bool) {
	m, ok := metadata[methodKey(method, path)]
	return m, ok
}

// AllMeta 返回元数据表的快照，仅供测试使用。
func AllMeta() map[string]Meta {
	out := make(map[string]Meta, len(metadata))
	for k, v := range metadata {
		out[k] = v
	}
	return out
}

func methodKey(method, path string) string {
	return strings.ToUpper(method) + ":" + path
}
