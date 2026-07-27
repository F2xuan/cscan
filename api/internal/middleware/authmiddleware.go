package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cscan/model"

	"github.com/golang-jwt/jwt/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ContextKey string

const (
	UserIdKey      ContextKey = "userId"
	UsernameKey    ContextKey = "username"
	RoleKey        ContextKey = "role"
	WorkspaceIdKey ContextKey = "workspaceId"
)

// PATLookup 由调用方注入的 PAT 认证回调。
// 入参为 token 明文，返回 (userId, role, status, tokenId, scopes, error)；
// token 不存在/失效时返回 (primitive.NilObjectID, "", "", primitive.NilObjectID, nil, nil)。
type PATLookup func(ctx context.Context, token string) (userId primitive.ObjectID, role, status string, tokenId primitive.ObjectID, scopes []string, err error)

// PATUsageRecorder 异步记录 PAT 使用信息的回调（可不提供）
type PATUsageRecorder func(ctx context.Context, tokenId primitive.ObjectID, ip string)

type AuthMiddleware struct {
	AccessSecret string
	UserModel    *model.UserModel
	PATLookup    PATLookup
	PATRecorder  PATUsageRecorder
}

func NewAuthMiddleware(accessSecret string) *AuthMiddleware {
	return &AuthMiddleware{
		AccessSecret: accessSecret,
	}
}

// WithPAT 注入 PAT 查询与审计回调，启用 PAT 认证路径
func (m *AuthMiddleware) WithPAT(lookup PATLookup, recorder PATUsageRecorder, userModel *model.UserModel) *AuthMiddleware {
	m.PATLookup = lookup
	m.PATRecorder = recorder
	m.UserModel = userModel
	return m
}

func (m *AuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var tokenStr string

		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenStr = parts[1]
			}
		}
		if tokenStr == "" {
			tokenStr = r.URL.Query().Get("token")
		}

		if tokenStr == "" {
			unauthorized(w, "未提供认证信息")
			return
		}

		// PAT 路径：以 cscan_pat_ 前缀开头，优先走 PAT 认证
		if strings.HasPrefix(tokenStr, model.PATPrefix) {
			if m.PATLookup == nil {
				unauthorized(w, "Token无效或已过期")
				return
			}
			m.handlePAT(w, r, tokenStr, next)
			return
		}

		// JWT 路径
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(m.AccessSecret), nil
		})
		if err != nil || !token.Valid {
			unauthorized(w, "Token无效或已过期")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			unauthorized(w, "Token解析失败")
			return
		}

		ctx := r.Context()
		if userId, ok := claims["userId"].(string); ok {
			ctx = context.WithValue(ctx, UserIdKey, userId)
		}
		if username, ok := claims["username"].(string); ok {
			ctx = context.WithValue(ctx, UsernameKey, username)
		}
		if role, ok := claims["role"].(string); ok {
			ctx = context.WithValue(ctx, RoleKey, role)
		}
		ctx = context.WithValue(ctx, WorkspaceIdKey, r.Header.Get("X-Workspace-Id"))

		next(w, r.WithContext(ctx))
	}
}

func (m *AuthMiddleware) handlePAT(w http.ResponseWriter, r *http.Request, tokenStr string, next http.HandlerFunc) {
	ctx := r.Context()
	uid, role, status, tokenId, scopes, err := m.PATLookup(ctx, tokenStr)
	if err != nil {
		unauthorized(w, "Token无效或已过期")
		return
	}
	if uid.IsZero() {
		unauthorized(w, "Token无效或已过期")
		return
	}
	if status != model.StatusEnable {
		unauthorized(w, "Token已失效")
		return
	}
	if role == "" {
		role = "user"
	}

	if !ScopeAllowed(scopes, r.URL.Path) {
		forbidden(w, "Token不允许调用此API分组")
		return
	}

	newCtx := context.WithValue(ctx, UserIdKey, uid.Hex())
	newCtx = context.WithValue(newCtx, UsernameKey, "")
	newCtx = context.WithValue(newCtx, RoleKey, role)
	newCtx = context.WithValue(newCtx, WorkspaceIdKey, r.Header.Get("X-Workspace-Id"))

	// 异步记录使用信息，避免阻塞请求
	if m.PATRecorder != nil {
		ip := clientIP(r)
		go func(id primitive.ObjectID, ipStr string) {
			recCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			m.PATRecorder(recCtx, id, ipStr)
		}(tokenId, ip)
	}

	next(w, r.WithContext(newCtx))
}

func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if idx := strings.Index(v, ","); idx > 0 {
			return strings.TrimSpace(v[:idx])
		}
		return strings.TrimSpace(v)
	}
	if v := r.Header.Get("X-Real-IP"); v != "" {
		return strings.TrimSpace(v)
	}
	// 截掉端口
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx > 0 {
		host = host[:idx]
	}
	return strings.TrimSpace(host)
}

func unauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 401,
		"msg":  msg,
	})
}

func forbidden(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 403,
		"msg":  msg,
	})
}

// GetUserId 从Context获取用户ID
func GetUserId(ctx context.Context) string {
	if v := ctx.Value(UserIdKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetUsername 从Context获取用户名
func GetUsername(ctx context.Context) string {
	if v := ctx.Value(UsernameKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetRole 从Context获取角色
func GetRole(ctx context.Context) string {
	if v := ctx.Value(RoleKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetWorkspaceId 从Context获取工作空间ID
func GetWorkspaceId(ctx context.Context) string {
	if v := ctx.Value(WorkspaceIdKey); v != nil {
		return v.(string)
	}
	return ""
}

// RequireAdmin 管理员权限中间件，需要先经过认证中间件
func RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		role := GetRole(r.Context())
		if role != "admin" && role != "superadmin" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 403,
				"msg":  "需要管理员权限",
			})
			return
		}
		next(w, r)
	}
}
