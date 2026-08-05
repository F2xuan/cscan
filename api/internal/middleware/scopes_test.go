package middleware

import (
	"testing"
)

func TestRouteToScope(t *testing.T) {
	cases := []struct {
		name string
		path string
		want APIScope
	}{
		{"user list → read", "/api/v1/user/list", "user:read"},
		{"user profile get → read", "/api/v1/user/profile/get", "user:read"},
		{"user create → create", "/api/v1/user/create", "user:create"},
		{"user update → update", "/api/v1/user/update", "user:update"},
		{"user delete → delete", "/api/v1/user/delete", "user:delete"},
		{"user avatar upload → create", "/api/v1/user/avatar/upload", "user:create"},
		{"asset list → read", "/api/v1/asset/list", "asset:read"},
		{"asset stat → read", "/api/v1/asset/stat", "asset:read"},
		{"asset save → create", "/api/v1/asset/save", "asset:create"},
		{"asset updateLabels → update", "/api/v1/asset/updateLabels", "asset:update"},
		{"asset batchDelete → delete", "/api/v1/asset/batchDelete", "asset:delete"},
		{"asset delete → delete", "/api/v1/asset/delete", "asset:delete"},
		{"asset import → create", "/api/v1/asset/import", "asset:create"},
		{"asset clear → update", "/api/v1/asset/clear", "asset:update"},
		{"asset site list → read", "/api/v1/asset/site/list", "asset:read"},
		{"asset icon batchDelete → delete", "/api/v1/asset/icon/batchDelete", "asset:delete"},
		{"task create → create", "/api/v1/task/create", "task:create"},
		{"task start → create", "/api/v1/task/start", "task:create"},
		{"task pause → update", "/api/v1/task/pause", "task:update"},
		{"task resume → update", "/api/v1/task/resume", "task:update"},
		{"task stop → delete", "/api/v1/task/stop", "task:delete"},
		{"task retry → create", "/api/v1/task/retry", "task:create"},
		{"task cron list → read", "/api/v1/task/cron/list", "task:read"},
		{"task cron save → create", "/api/v1/task/cron/save", "task:create"},
		{"task cron runNow → create", "/api/v1/task/cron/runNow", "task:create"},
		{"task cron batchDelete → delete", "/api/v1/task/cron/batchDelete", "task:delete"},
		{"task logs → read", "/api/v1/task/logs", "task:read"},
		{"vul list → read", "/api/v1/vul/list", "vul:read"},
		{"vul detail → read", "/api/v1/vul/detail", "vul:read"},
		{"vul clear → update", "/api/v1/vul/clear", "vul:update"},
		{"vul batchDelete → delete", "/api/v1/vul/batchDelete", "vul:delete"},
		{"worker ws → read", "/api/v1/worker/ws", "worker:read"},
		{"fingerprint list → read", "/api/v1/fingerprint/list", "fingerprint:read"},
		{"fingerprint sync → create", "/api/v1/fingerprint/sync", "fingerprint:create"},
		{"fingerprint delete → delete", "/api/v1/fingerprint/delete", "fingerprint:delete"},
		{"fingerprint updateEnabled → update", "/api/v1/fingerprint/updateEnabled", "fingerprint:update"},
		{"poc custom save → create", "/api/v1/poc/custom/save", "poc:create"},
		{"poc custom delete → delete", "/api/v1/poc/custom/delete", "poc:delete"},
		{"poc nuclei templates → read", "/api/v1/poc/nuclei/templates", "poc:read"},
		{"onlineapi search → read", "/api/v1/onlineapi/search", "onlineapi:read"},
		{"onlineapi import → create", "/api/v1/onlineapi/import", "onlineapi:create"},
		{"organization list → read", "/api/v1/organization/list", "organization:read"},
		{"organization save → create", "/api/v1/organization/save", "organization:create"},
		{"organization updateStatus → update", "/api/v1/organization/updateStatus", "organization:update"},
		{"workspace list → read", "/api/v1/workspace/list", "workspace:read"},
		{"workspace save → create", "/api/v1/workspace/save", "workspace:create"},
		{"workspace delete → delete", "/api/v1/workspace/delete", "workspace:delete"},
		{"dirscan dict list → read", "/api/v1/dirscan/dict/list", "dirscan:read"},
		{"dirscan dict save → create", "/api/v1/dirscan/dict/save", "dirscan:create"},
		{"dirscan dict clear → update", "/api/v1/dirscan/dict/clear", "dirscan:update"},
		{"subdomain dict list → read", "/api/v1/subdomain/dict/list", "subdomain:read"},
		{"subfinder provider save → create", "/api/v1/subfinder/provider/save", "subfinder:create"},
		{"notify config list → read", "/api/v1/notify/config/list", "notify:read"},
		{"notify config test → create", "/api/v1/notify/config/test", "notify:create"},
		{"report detail → read", "/api/v1/report/detail", "report:read"},
		{"report export → read", "/api/v1/report/export", "report:read"},
		{"ai generatePoc → create", "/api/v1/ai/generatePoc", "ai:create"},
		{"ai config get → read", "/api/v1/ai/config/get", "ai:read"},
		{"container list → read", "/api/v1/container/list", "container:read"},
		{"jsfinder config get → read", "/api/v1/jsfinder/config/get", "jsfinder:read"},
		{"jsfinder clear → update", "/api/v1/jsfinder/clear", "jsfinder:update"},
		{"blacklist config get → read", "/api/v1/blacklist/config/get", "blacklist:read"},
		{"blacklist config save → create", "/api/v1/blacklist/config/save", "blacklist:create"},
		{"weakpass dict list → read", "/api/v1/weakpass/dict/list", "weakpass:read"},
		{"weakpass dict import → create", "/api/v1/weakpass/dict/import", "weakpass:create"},
		{"non /api/v1 prefix → *", "/health", "*"},
		{"empty group → *", "/api/v1/", "*"},
		{"prefix only → *", "/api/v1", "*"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RouteToScope(tc.path)
			if got != tc.want {
				t.Errorf("RouteToScope(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestValidScope(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"*", true},
		{"user", true},      // 纯分组（兼容）
		{"user:read", true}, // 分组 + 动作
		{"asset:create", true},
		{"task:delete", true},
		{"unknown:read", false}, // 分组不存在
		{"user:foo", false},      // 动作不存在
		{"foo", false},          // 分组不存在
		{"", false},              // 空串
	}
	for _, tc := range cases {
		t.Run(tc.s, func(t *testing.T) {
			if got := ValidScope(tc.s); got != tc.want {
				t.Errorf("ValidScope(%q) = %v, want %v", tc.s, got, tc.want)
			}
		})
	}
}

func TestScopeAllowed(t *testing.T) {
	cases := []struct {
		name   string
		scopes []string
		path   string
		want   bool
	}{
		{"all scope passes any", []string{"*"}, "/api/v1/asset/list", true},
		{"all scope passes user create", []string{"*"}, "/api/v1/user/create", true},
		{"nil scopes treated as all", nil, "/api/v1/asset/list", true},
		{"empty scopes treated as all", []string{}, "/api/v1/asset/list", true},

		// 精确 <group>:<action> 匹配
		{"read matches list", []string{"asset:read"}, "/api/v1/asset/list", true},
		{"read matches stat", []string{"asset:read"}, "/api/v1/asset/stat", true},
		{"read does not match save", []string{"asset:read"}, "/api/v1/asset/save", false},
		{"create matches save", []string{"asset:create"}, "/api/v1/asset/save", true},
		{"delete matches batchDelete", []string{"asset:delete"}, "/api/v1/asset/batchDelete", true},
		{"update matches updateLabels", []string{"asset:update"}, "/api/v1/asset/updateLabels", true},

		// 多 scope 组合
		{"multiple actions matched", []string{"task:read", "task:create"}, "/api/v1/task/create", true},
		{"multiple groups mismatched", []string{"asset:read"}, "/api/v1/task/list", false},
		{"scope set with mismatched action", []string{"asset:read"}, "/api/v1/asset/delete", false},

		// 兼容：纯分组 "<group>" 视为该分组全部动作
		{"pure group passes all actions", []string{"asset"}, "/api/v1/asset/delete", true},
		{"pure group passes any action", []string{"asset"}, "/api/v1/asset/list", true},
		{"pure group does not affect other groups", []string{"asset"}, "/api/v1/user/list", false},

		// 非 /api/v1 路径
		{"non /api/v1 path with single scope", []string{"asset:read"}, "/health", false},
		{"non /api/v1 path with all scope", []string{"*"}, "/health", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ScopeAllowed(tc.scopes, tc.path)
			if got != tc.want {
				t.Errorf("ScopeAllowed(%v, %q) = %v, want %v", tc.scopes, tc.path, got, tc.want)
			}
		})
	}
}

// TestScopeMatrix_AllCombinationsValid 每个分组衍生的 <group>:<action> 组合都必须是合法 scope，
// 保证前端按 ScopeGroups × ScopeActions 渲染出的矩阵不会产生服务端拒绝的 scope。
func TestScopeMatrix_AllCombinationsValid(t *testing.T) {
	groups := ScopeGroups()
	actions := ScopeActions()
	if len(groups) == 0 || len(actions) == 0 {
		t.Fatalf("ScopeGroups=%d ScopeActions=%d，两者均不应为空", len(groups), len(actions))
	}

	for _, g := range groups {
		for _, a := range actions {
			scope := string(g.Value) + ":" + a
			if !ValidScope(scope) {
				t.Errorf("ValidScope(%q) = false，分组矩阵组合必须合法", scope)
			}
		}
	}
}
