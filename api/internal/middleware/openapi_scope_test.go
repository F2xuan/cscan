package middleware

import "testing"

// TestRouteToScopeOpenAPI 验证开放 API 路径被映射到 readonly:read scope。
func TestRouteToScopeOpenAPI(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/api/open/v1/assets", "readonly:read"},
		{"/api/open/v1/assets/abc123", "readonly:read"},
		{"/api/open/v1/vulns", "readonly:read"},
		{"/api/open/v1/vulns/x", "readonly:read"},
		{"/api/open/v1/certs", "readonly:read"},
		{"/api/v1/asset/list", "asset:read"},
		{"/health", "*"},
	}
	for _, c := range cases {
		if got := RouteToScope(c.path); string(got) != c.want {
			t.Errorf("RouteToScope(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// TestScopeAllowedOpenAPI 验证 readonly scope 放行开放 API，其它分组 scope 不放行。
func TestScopeAllowedOpenAPI(t *testing.T) {
	openPath := "/api/open/v1/assets"

	if !ScopeAllowed([]string{"readonly"}, openPath) {
		t.Error("readonly scope should allow open API")
	}
	if !ScopeAllowed([]string{"readonly:read"}, openPath) {
		t.Error("readonly:read scope should allow open API")
	}
	if !ScopeAllowed([]string{"*"}, openPath) {
		t.Error("* scope should allow open API")
	}
	// 业务分组 scope 不得放行开放 API
	if ScopeAllowed([]string{"asset"}, openPath) {
		t.Error("asset scope must NOT allow open API")
	}
	if ScopeAllowed([]string{"asset:read"}, openPath) {
		t.Error("asset:read scope must NOT allow open API")
	}
	// 防御性：无 scope 兼容放行
	if !ScopeAllowed(nil, openPath) {
		t.Error("nil scopes should be compatibly allowed")
	}
}

// TestReadonlyScopeRegistered 验证 readonly 已注册到分组元数据。
func TestReadonlyScopeRegistered(t *testing.T) {
	found := false
	for _, g := range ScopeGroups() {
		if g.Value == ScopeReadonly {
			found = true
			if g.Label == "" {
				t.Error("readonly scope label must not be empty")
			}
		}
	}
	if !found {
		t.Error("readonly scope not registered in ScopeGroups")
	}
	if !ValidScope(string(ScopeReadonly)) {
		t.Error("readonly should be a valid scope")
	}
}
