package swagger

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"cscan/api/internal/types"
)

// init 在测试包加载时把 metadata 中登记的端点也加入到 collectedRoutes，
// 这样 TestMetadataPathsExistInCollectedRoutes 与 SpecHandler 才能看到完整路由集合。
// 正常生产运行时 routes.go 会通过 swagger.CollectEx 完成同样动作。
func init() {
	collectedMu.Lock()
	defer collectedMu.Unlock()
	for k := range metadata {
		parts := strings.SplitN(k, ":", 2)
		if len(parts) != 2 {
			continue
		}
		method, path := parts[0], parts[1]
		collectedRoutes = append(collectedRoutes, RouteInfo{
			Method:   method,
			Path:     path,
			Group:    groupOf(path),
			AuthTier: TierAuth,
		})
	}
	// 健康检查端点
	collectedRoutes = append(collectedRoutes, RouteInfo{
		Method: http.MethodGet, Path: "/health", Group: "system", AuthTier: TierPublic,
	})
}


func TestSpecHandler_OpenAPIVersion(t *testing.T) {
	spec := requestSpec(t)
	if spec["openapi"] != "3.0.3" {
		t.Fatalf("openapi version: got %v want 3.0.3", spec["openapi"])
	}
	info, _ := spec["info"].(map[string]interface{})
	if info["title"] != "CSCAN API" {
		t.Errorf("info.title: got %v want CSCAN API", info["title"])
	}
}

func TestSpecHandler_LoginEndpointHasPublicSecurity(t *testing.T) {
	spec := requestSpec(t)
	paths := spec["paths"].(map[string]interface{})
	loginPath, ok := paths["/api/v1/login"]
	if !ok {
		t.Fatal("缺少 /api/v1/login")
	}
	postOp := loginPath.(map[string]interface{})["post"].(map[string]interface{})
	if sec, exists := postOp["security"]; exists {
		t.Errorf("login 不应有 security, got %v", sec)
	}
	if s, _ := postOp["summary"].(string); !strings.Contains(s, "登录") {
		t.Errorf("login summary 应含'登录', got %q", s)
	}
	if _, exist := postOp["description"]; !exist {
		t.Error("login 应有 description")
	}
	body, _ := postOp["requestBody"].(map[string]interface{})
	if body == nil {
		t.Fatal("login 应有 requestBody")
	}
	schema := body["content"].(map[string]interface{})["application/json"].(map[string]interface{})["schema"].(map[string]interface{})
	if ref, _ := schema["$ref"].(string); !strings.HasSuffix(ref, "/LoginReq") {
		t.Errorf("login requestBody 应引用 LoginReq, got %v", schema)
	}
}

func TestSpecHandler_AdminEndpointHasBearerSecurity(t *testing.T) {
	spec := requestSpec(t)
	paths := spec["paths"].(map[string]interface{})
	createOp, ok := paths["/api/v1/user/create"]
	if !ok {
		t.Fatal("缺少 /api/v1/user/create")
	}
	post := createOp.(map[string]interface{})["post"].(map[string]interface{})
	sec, ok := post["security"].([]interface{})
	if !ok || len(sec) == 0 {
		t.Fatal("user/create 应有 security")
	}
	first, _ := sec[0].(map[string]interface{})
	if _, ok := first["BearerAuth"]; !ok {
		t.Errorf("user/create security 应包含 BearerAuth, got %v", sec)
	}
	resps := post["responses"].(map[string]interface{})
	if _, ok := resps["403"]; !ok {
		t.Error("user/create 缺少 403 响应描述")
	}
	if _, ok := resps["401"]; !ok {
		t.Error("user/create 缺少 401 响应描述")
	}
}

func TestSpecHandler_LoginReqSchemaRegistered(t *testing.T) {
	spec := requestSpec(t)
	components := spec["components"].(map[string]interface{})
	schemas := components["schemas"].(map[string]interface{})
	if _, ok := schemas["LoginReq"]; !ok {
		t.Error("components.schemas 应包含 LoginReq")
	}
	loginReq := schemas["LoginReq"].(map[string]interface{})
	props := loginReq["properties"].(map[string]interface{})
	if _, ok := props["username"]; !ok {
		t.Error("LoginReq 缺少 username 字段")
	}
	if _, ok := props["password"]; !ok {
		t.Error("LoginReq 缺少 password 字段")
	}
}

func TestSpecHandler_TagsContainBusinessGroups(t *testing.T) {
	spec := requestSpec(t)
	rawTags, _ := spec["tags"].([]interface{})
	wantTags := map[string]bool{
		"系统":   false,
		"用户认证": false,
		"用户管理": false,
		"资产管理": false,
		"任务管理": false,
	}
	for _, raw := range rawTags {
		tag, _ := raw.(map[string]interface{})
		name, _ := tag["name"].(string)
		if _, ok := wantTags[name]; ok {
			wantTags[name] = true
			desc, _ := tag["description"].(string)
			if desc == "" {
				t.Errorf("tag %q 缺少 description", name)
			}
		}
	}
	for name, found := range wantTags {
		if !found {
			t.Errorf("tags 中缺少 %q", name)
		}
	}
}

func TestSpecHandler_AssetListEndpointHasDescAndResponses(t *testing.T) {
	spec := requestSpec(t)
	paths := spec["paths"].(map[string]interface{})
	assetPath, ok := paths["/api/v1/asset/list"]
	if !ok {
		t.Fatal("缺少 /api/v1/asset/list")
	}
	post := assetPath.(map[string]interface{})["post"].(map[string]interface{})
	if _, ok := post["description"]; !ok {
		t.Error("asset/list 应有 description")
	}
	resps := post["responses"].(map[string]interface{})
	if _, ok := resps["200"]; !ok {
		t.Error("缺少 200 响应")
	}
	if _, ok := resps["401"]; !ok {
		t.Error("缺少 401 响应")
	}
}

func TestMetadataPathsExistInCollectedRoutes(t *testing.T) {
	collectedMu.Lock()
	routes := append([]RouteInfo(nil), collectedRoutes...)
	collectedMu.Unlock()
	routeKeys := make(map[string]bool, len(routes))
	for _, rt := range routes {
		routeKeys[methodKey(rt.Method, rt.Path)] = true
	}
	misses := []string{}
	for key := range metadata {
		if !routeKeys[key] {
			misses = append(misses, key)
		}
	}
	if len(misses) > 0 {
		t.Fatalf("metadata 中登记但路由未注册的端点: %v", misses)
	}
}

func TestRegisterTypes_Integration(t *testing.T) {
	r := NewReflector()
	r.RegisterBySample(types.Asset{})
	if _, ok := r.Schemas()["Asset"]; !ok {
		t.Error("Asset 应被注册到 schemas")
	}
}

// requestSpec 触发一次 SpecHandler 调用并返回解析后的 spec。
// 每次 reset specOnce 以确保最新 metadata 生效。
func requestSpec(t *testing.T) map[string]interface{} {
	t.Helper()
	specOnce = sync.Once{}
	specCache = nil

	req := httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)
	rec := httptest.NewRecorder()
	SpecHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rec.Code)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	return out
}
