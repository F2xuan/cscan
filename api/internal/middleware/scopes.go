package middleware

import (
	"strings"
)

// APIScope 标识一个 API 分组的访问权限，格式为 "<group>:<action>"
// 或 "*"（放行所有）。action ∈ {read, create, update, delete}。
type APIScope string

const (
	ScopeAll         APIScope = "*"
	ActionRead       string   = "read"
	ActionCreate     string   = "create"
	ActionUpdate     string   = "update"
	ActionDelete     string   = "delete"
)

const (
	ScopeUser         APIScope = "user"
	ScopeAsset        APIScope = "asset"
	ScopeTask         APIScope = "task"
	ScopeVul          APIScope = "vul"
	ScopeWorker       APIScope = "worker"
	ScopeFingerprint  APIScope = "fingerprint"
	ScopePoc          APIScope = "poc"
	ScopeOnlineAPI    APIScope = "onlineapi"
	ScopeOrganization APIScope = "organization"
	ScopeWorkspace    APIScope = "workspace"
	ScopeDirscan      APIScope = "dirscan"
	ScopeSubdomain    APIScope = "subdomain"
	ScopeSubfinder    APIScope = "subfinder"
	ScopeNotify       APIScope = "notify"
	ScopeReport       APIScope = "report"
	ScopeAI           APIScope = "ai"
	ScopeContainer    APIScope = "container"
	ScopeJSFinder     APIScope = "jsfinder"
	ScopeBlacklist    APIScope = "blacklist"
	ScopeWeakpass     APIScope = "weakpass"
)

// ScopeGroupMeta 描述一个分组的可读信息
type ScopeGroupMeta struct {
	Value       APIScope
	Label       string
	Description string
}

// ScopeGroups 返回所有分组（不含 "*"）的元信息
func ScopeGroups() []ScopeGroupMeta {
	return []ScopeGroupMeta{
		{ScopeUser, "用户管理", "用户账号、个人资料、API Token"},
		{ScopeAsset, "资产管理", "端口、站点、域名、IP、截图、历史"},
		{ScopeTask, "任务管理", "任务创建、暂停、恢复、停止、定时任务"},
		{ScopeVul, "漏洞管理", "漏洞列表、详情、统计"},
		{ScopeWorker, "Worker 管理", "Worker 注册、心跳、配置"},
		{ScopeFingerprint, "指纹管理", "指纹 CRUD、分类、同步"},
		{ScopePoc, "POC 管理", "自定义 POC、Nuclei 模板、AI 生成"},
		{ScopeOnlineAPI, "在线搜索", "FOFA/Hunter/Quake API 聚合"},
		{ScopeOrganization, "组织管理", "组织 CRUD、状态切换"},
		{ScopeWorkspace, "工作空间", "工作空间 CRUD"},
		{ScopeDirscan, "目录扫描", "字典管理、扫描结果"},
		{ScopeSubdomain, "子域名字典", "子域名字典管理"},
		{ScopeSubfinder, "Subfinder 配置", "Subfinder 子域名发现配置"},
		{ScopeNotify, "通知配置", "通知配置、主题、高危过滤器"},
		{ScopeReport, "报告", "报告详情、导出"},
		{ScopeAI, "AI", "AI POC 生成"},
		{ScopeContainer, "容器", "容器管理、日志、终端"},
		{ScopeJSFinder, "JSFinder", "JavaScript 外链发现"},
		{ScopeBlacklist, "黑名单", "黑名单规则配置"},
		{ScopeWeakpass, "弱口令字典", "弱口令字典管理"},
	}
}

// ScopeActions 返回所有 CRUD 动作标识
func ScopeActions() []string {
	return []string{ActionRead, ActionCreate, ActionUpdate, ActionDelete}
}

// ScopeMeta 兼容旧引用：以单条 scope 描述形式返回所有 <group>:<action> 组合
// （前端切换为新的分组矩阵结构后此用法已废弃，但保留以避免破坏外部调用）
type ScopeMeta struct {
	Value       APIScope
	Label       string
	Description string
}

// AllScopes 返回所有 <group>:<action> 组合的元信息（不含 "*"）。
// 兼容旧前端 UI 的扁平列表结构；新前端使用 ScopeGroups + ScopeActions。
func AllScopes() []ScopeMeta {
	groups := ScopeGroups()
	actions := ScopeActions()
	out := make([]ScopeMeta, 0, len(groups)*len(actions))
	for _, g := range groups {
		for _, a := range actions {
			label := g.Label + " · " + actionLabel(a)
			out = append(out, ScopeMeta{
				Value:       APIScope(string(g.Value) + ":" + a),
				Label:       label,
				Description: g.Description,
			})
		}
	}
	return out
}

func actionLabel(a string) string {
	switch a {
	case ActionRead:
		return "读"
	case ActionCreate:
		return "增"
	case ActionUpdate:
		return "改"
	case ActionDelete:
		return "删"
	}
	return a
}

// ValidScope 判断字符串是否为合法的 scope 标识：
//   - "*" 全量放行
//   - "<group>"         匹配整个分组（视为该分组全部动作放行；向后兼容）
//   - "<group>:<action>" 匹配分组下的具体动作
func ValidScope(s string) bool {
	if s == string(ScopeAll) {
		return true
	}
	group, action := splitScope(s)
	if action == "" {
		// 纯分组：校验是否为已知分组
		return knownGroup(group)
	}
	return knownGroup(group) && knownAction(action)
}

func knownGroup(g string) bool {
	for _, m := range ScopeGroups() {
		if string(m.Value) == g {
			return true
		}
	}
	return false
}

func knownAction(a string) bool {
	for _, x := range ScopeActions() {
		if x == a {
			return true
		}
	}
	return false
}

// splitScope 拆分 "<group>:<action>"，无 ":" 时返回 (group, "")
func splitScope(s string) (group, action string) {
	if idx := strings.Index(s, ":"); idx >= 0 {
		return s[:idx], s[idx+1:]
	}
	return s, ""
}

// RouteToScope 将 /api/v1/<group>/... 路径映射到 "<group>:<action>"
// 路径不含 /api/v1 前缀或第三段为空时返回 "*"；
// 末段动词未能识别为 CRUD 时默认归为 read。
func RouteToScope(path string) APIScope {
	const prefix = "/api/v1/"
	if !strings.HasPrefix(path, prefix) {
		return ScopeAll
	}
	rest := strings.TrimPrefix(path, prefix)
	if rest == "" {
		return ScopeAll
	}
	// 取第一段作 group
	idx := strings.Index(rest, "/")
	group := rest
	tail := ""
	if idx >= 0 {
		group = rest[:idx]
		tail = rest[idx+1:]
	}
	if group == "" {
		return ScopeAll
	}
	return APIScope(group + ":" + actionOfPath(tail))
}

// actionOfPath 由末尾若干段推断 CRUD 动作：
//   - create/save/import/upload/add/enable/sync/run/use/validate/start/recovery/generate/runNow → create
//   - update/edit/set/rename/move/reset/disable/refresh/restart/control/pause/resume/clear     → update
//   - delete/remove/revoke/stop                                                                  → delete
//   - 其它（list/get/stat/detail/info/find/export/batch*/sync 等）                              → read
//
// 当末段本身不是动词（如 "labels"）时，回退扫描整段路径中的动词前缀，
// 让 "updateLabels"、"updateEnabled"、"updateStatus"、"generatePoc" 也能被识别。
func actionOfPath(tail string) string {
	if tail == "" {
		return ActionRead
	}
	segs := strings.Split(tail, "/")
	last := strings.ToLower(segs[len(segs)-1])
	if last == "" {
		return ActionRead
	}
	if a, ok := verbOf(last); ok {
		return a
	}
	if strings.HasPrefix(last, "batch") {
		if strings.Contains(last, "delete") || strings.Contains(last, "remove") {
			return ActionDelete
		}
		if strings.Contains(last, "update") || strings.Contains(last, "edit") || strings.Contains(last, "set") {
			return ActionUpdate
		}
		return ActionRead
	}
	// 末段不是动词：回退扫描每段是否以动词开头（updateLabels → update）
	for i := len(segs) - 1; i >= 0; i-- {
		seg := strings.ToLower(segs[i])
		if a, ok := verbStartWith(seg); ok {
			return a
		}
	}
	return ActionRead
}

// verbOf 判断整段是否为一个完整动词
func verbOf(seg string) (string, bool) {
	switch seg {
	case "create", "save", "import", "upload", "add", "enable", "sync", "run", "use", "validate", "start", "recovery", "importall", "runnow", "generate", "test", "retry":
		return ActionCreate, true
	case "update", "edit", "set", "rename", "move", "clear", "reset", "disable", "refresh", "restart", "control", "pause", "resume":
		return ActionUpdate, true
	case "delete", "remove", "revoke", "stop":
		return ActionDelete, true
	}
	return "", false
}

// verbStartWith 判断段是否以动词前缀开头（updateLabels → update）
func verbStartWith(seg string) (string, bool) {
	prefixes := []struct {
		prefix string
		act    string
	}{
		{"create", ActionCreate},
		{"save", ActionCreate},
		{"import", ActionCreate},
		{"upload", ActionCreate},
		{"add", ActionCreate},
		{"enable", ActionCreate},
		{"sync", ActionCreate},
		{"run", ActionCreate},
		{"use", ActionCreate},
		{"validate", ActionCreate},
		{"start", ActionCreate},
		{"generate", ActionCreate},
		{"test", ActionCreate},
		{"update", ActionUpdate},
		{"edit", ActionUpdate},
		{"set", ActionUpdate},
		{"rename", ActionUpdate},
		{"move", ActionUpdate},
		{"reset", ActionUpdate},
		{"disable", ActionUpdate},
		{"refresh", ActionUpdate},
		{"restart", ActionUpdate},
		{"control", ActionUpdate},
		{"pause", ActionUpdate},
		{"resume", ActionUpdate},
		{"clear", ActionUpdate},
		{"delete", ActionDelete},
		{"remove", ActionDelete},
		{"revoke", ActionDelete},
		{"stop", ActionDelete},
	}
	for _, p := range prefixes {
		if strings.HasPrefix(seg, p.prefix) {
			return p.act, true
		}
	}
	return "", false
}

// ScopeAllowed 检查 PAT scopes 是否放行某路由。
// 规则：
//   - scopes 为 nil/空 → 放行（防御性：旧 Token 未配置 scope 时保持兼容）
//   - scopes 含 "*" → 放行所有
//   - 否则按路由推导 "<group>:<action>"，必须在 scopes 集合内。
//   - 若 PAT 持有的是纯分组（无 action），视为该分组下全部动作放行。
func ScopeAllowed(scopes []string, path string) bool {
	if len(scopes) == 0 {
		return true
	}
	need := string(RouteToScope(path))
	if need == string(ScopeAll) {
		// 路径不在 /api/v1/* 下，仅全量 scope 放行
	}
	scopeSet := make(map[string]struct{}, len(scopes))
	for _, s := range scopes {
		if s == string(ScopeAll) {
			return true
		}
		scopeSet[s] = struct{}{}
	}
	// 精确匹配 <group>:<action>
	if _, ok := scopeSet[need]; ok {
		return true
	}
	// 兼容：PAT 持有纯分组 "<group>"，则该分组下所有动作放行
	grp, _ := splitScope(need)
	if _, ok := scopeSet[grp]; ok {
		return true
	}
	return false
}
