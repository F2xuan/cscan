package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"cscan/model"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// buildMW 构造一个 AuthMiddleware，注入测试用 PAT 查询表
func buildMW(lookup map[string]lookupResult) *AuthMiddleware {
	mw := NewAuthMiddleware("test-secret")
	mw.UserModel = nil
	mw.PATLookup = func(ctx context.Context, token string) (primitive.ObjectID, string, string, primitive.ObjectID, []string, error) {
		if r, ok := lookup[token]; ok {
			return r.userId, r.role, r.status, r.tokenId, r.scopes, nil
		}
		return primitive.NilObjectID, "", "", primitive.NilObjectID, nil, nil
	}
	mw.PATRecorder = nil
	return mw
}

type lookupResult struct {
	userId  primitive.ObjectID
	role    string
	status  string
	tokenId primitive.ObjectID
	scopes  []string
}

func TestPATRoute_Success(t *testing.T) {
	uid := primitive.NewObjectID()
	mw := buildMW(map[string]lookupResult{
		"cscan_pat_valid_token": {userId: uid, role: "user", status: model.StatusEnable, tokenId: primitive.NewObjectID()},
	})

	var capturedUserId string
	handler := mw.Handle(func(w http.ResponseWriter, r *http.Request) {
		capturedUserId = GetUserId(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/list", nil)
	req.Header.Set("Authorization", "Bearer cscan_pat_valid_token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, uid.Hex(), capturedUserId)
}

func TestPATRoute_UnknownToken_Unauthorized(t *testing.T) {
	mw := buildMW(map[string]lookupResult{})

	handler := mw.Handle(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("下游不应被调用")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/list", nil)
	req.Header.Set("Authorization", "Bearer cscan_pat_unknown")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestPATRoute_DisabledToken_Unauthorized(t *testing.T) {
	uid := primitive.NewObjectID()
	mw := buildMW(map[string]lookupResult{
		"cscan_pat_revoked": {userId: uid, role: "user", status: model.StatusDisable, tokenId: primitive.NewObjectID()},
	})

	handler := mw.Handle(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("已吊销 Token 不应通过")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/list", nil)
	req.Header.Set("Authorization", "Bearer cscan_pat_revoked")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestPATRoute_DisabledUser_Unauthorized(t *testing.T) {
	uid := primitive.NewObjectID()
	mw := buildMW(map[string]lookupResult{
		"cscan_pat_user_disabled": {userId: uid, role: "user", status: model.StatusDisable, tokenId: primitive.NewObjectID()},
	})

	handler := mw.Handle(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("已禁用用户的 Token 不应通过")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/list", nil)
	req.Header.Set("Authorization", "Bearer cscan_pat_user_disabled")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestPATRoute_NoAuth_Unauthorized(t *testing.T) {
	mw := buildMW(map[string]lookupResult{})
	handler := mw.Handle(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("下游不应被调用")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/list", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// 验证 PAT 路径不带异步 recorder 也能正常工作（避免 nil panic）
func TestPATRoute_NilRecorder_NoPanic(t *testing.T) {
	uid := primitive.NewObjectID()
	mw := NewAuthMiddleware("test-secret")
	mw.PATLookup = func(ctx context.Context, token string) (primitive.ObjectID, string, string, primitive.ObjectID, []string, error) {
		return uid, "user", model.StatusEnable, primitive.NewObjectID(), nil, nil
	}
	mw.PATRecorder = nil

	called := false
	handler := mw.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/x", nil)
	req.Header.Set("Authorization", "Bearer cscan_pat_x")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPATRoute_ScopeAllowed_PassThrough(t *testing.T) {
	uid := primitive.NewObjectID()
	mw := buildMW(map[string]lookupResult{
		"cscan_pat_scope_asset": {userId: uid, role: "user", status: model.StatusEnable, tokenId: primitive.NewObjectID(), scopes: []string{"asset"}},
	})

	called := false
	handler := mw.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/asset/list", nil)
	req.Header.Set("Authorization", "Bearer cscan_pat_scope_asset")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPATRoute_ScopeDenied_Forbidden(t *testing.T) {
	uid := primitive.NewObjectID()
	mw := buildMW(map[string]lookupResult{
		"cscan_pat_scope_asset": {userId: uid, role: "user", status: model.StatusEnable, tokenId: primitive.NewObjectID(), scopes: []string{"asset"}},
	})

	handler := mw.Handle(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("scope 外路由不应被调用")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/list", nil)
	req.Header.Set("Authorization", "Bearer cscan_pat_scope_asset")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestPATRoute_ScopeAll_PassThrough(t *testing.T) {
	uid := primitive.NewObjectID()
	mw := buildMW(map[string]lookupResult{
		"cscan_pat_scope_all": {userId: uid, role: "user", status: model.StatusEnable, tokenId: primitive.NewObjectID(), scopes: []string{"*"}},
	})

	called := false
	handler := mw.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/asset/list", nil)
	req.Header.Set("Authorization", "Bearer cscan_pat_scope_all")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPATRoute_NilScopes_TreatedAsAll(t *testing.T) {
	uid := primitive.NewObjectID()
	mw := buildMW(map[string]lookupResult{
		"cscan_pat_nil_scope": {userId: uid, role: "user", status: model.StatusEnable, tokenId: primitive.NewObjectID(), scopes: nil},
	})

	called := false
	handler := mw.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/asset/list", nil)
	req.Header.Set("Authorization", "Bearer cscan_pat_nil_scope")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPATRoute_ScopeCRUD_GranularPassThrough(t *testing.T) {
	uid := primitive.NewObjectID()
	mw := buildMW(map[string]lookupResult{
		"cscan_pat_asset_read": {userId: uid, role: "user", status: model.StatusEnable, tokenId: primitive.NewObjectID(), scopes: []string{"asset:read"}},
	})

	called := false
	handler := mw.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/asset/list", nil)
	req.Header.Set("Authorization", "Bearer cscan_pat_asset_read")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPATRoute_ScopeCRUD_GranularForbidden(t *testing.T) {
	uid := primitive.NewObjectID()
	mw := buildMW(map[string]lookupResult{
		"cscan_pat_asset_read": {userId: uid, role: "user", status: model.StatusEnable, tokenId: primitive.NewObjectID(), scopes: []string{"asset:read"}},
	})

	handler := mw.Handle(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("asset:read 不应放行 asset:delete 路由")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/asset/delete", nil)
	req.Header.Set("Authorization", "Bearer cscan_pat_asset_read")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}
