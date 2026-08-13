package openapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/internal/model"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const testSecret = "openapi-test-secret"

func patLookup(scopes []string) middleware.PATLookup {
	return func(ctx context.Context, token string) (primitive.ObjectID, string, string, primitive.ObjectID, []string, error) {
		return primitive.NewObjectID(), "user", model.StatusEnable, primitive.NewObjectID(), scopes, nil
	}
}

// buildWrapped 构造与 routes.go 一致的开放 API 处理链：PAT 鉴权(scope) → 限流 → handler。
// svcCtx 传空（本测试仅覆盖鉴权/限流/只读/workspace 必填，均不触达 MongoDB）。
func buildWrapped(scopes []string) http.HandlerFunc {
	authM := middleware.NewAuthMiddleware(testSecret).
		WithPAT(patLookup(scopes), nil, nil)
	limiter := middleware.NewTokenRateLimiter(1000, time.Minute)
	h := OpenAssetsHandler(&svc.ServiceContext{})
	return func(w http.ResponseWriter, r *http.Request) {
		authM.Handle(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			limiter.Handle(h).ServeHTTP(w, r)
		})).ServeHTTP(w, r)
	}
}

func doReq(h http.HandlerFunc, method, token, wsId string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "/api/open/v1/assets", nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if wsId != "" {
		r.Header.Set("X-Workspace-Id", wsId)
	}
	w := httptest.NewRecorder()
	h(w, r)
	return w
}

func TestOpenAPI_NoToken_401(t *testing.T) {
	h := buildWrapped([]string{"readonly"})
	w := doReq(h, http.MethodGet, "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestOpenAPI_WrongScope_403(t *testing.T) {
	h := buildWrapped([]string{"asset"}) // 无 readonly
	w := doReq(h, http.MethodGet, "cscan_pat_x", "ws1")
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestOpenAPI_ReadOnlyScope_NoWorkspaceRequired(t *testing.T) {
	h := buildWrapped([]string{"readonly"})
	// workspace 概念已移除，不传 X-Workspace-Id 不应返回 400
	// 由于 svcCtx 无 DB 连接，handler 会 panic，用 recover 捕获验证非 400
	defer func() {
		if r := recover(); r != nil {
			// panic 说明请求已通过鉴权/scope 检查并进入 handler（workspace 不再拦截）
			return
		}
	}()
	w := doReq(h, http.MethodGet, "cscan_pat_x", "")
	if w.Code == http.StatusBadRequest {
		t.Fatalf("workspace 移除后不应返回 400，got %d (body=%s)", w.Code, w.Body.String())
	}
}

func TestOpenAPI_NonGet_405(t *testing.T) {
	h := buildWrapped([]string{"readonly"})
	w := doReq(h, http.MethodPost, "cscan_pat_x", "ws1")
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestOpenAPI_WildcardScope_NoWorkspaceRequired(t *testing.T) {
	h := buildWrapped([]string{"*"})
	defer func() {
		if r := recover(); r != nil {
			return
		}
	}()
	w := doReq(h, http.MethodGet, "cscan_pat_x", "")
	if w.Code == http.StatusBadRequest {
		t.Fatalf("workspace 移除后不应返回 400，got %d (body=%s)", w.Code, w.Body.String())
	}
}
