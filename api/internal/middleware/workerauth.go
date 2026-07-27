package middleware

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

// WorkerNameKey Context key for worker name
const WorkerNameKey ContextKey = "workerName"

// WorkerAuthMiddleware Worker认证中间件
type WorkerAuthMiddleware struct {
	RedisClient *redis.Client
}

// NewWorkerAuthMiddleware 创建Worker认证中间件
func NewWorkerAuthMiddleware(redisClient *redis.Client) *WorkerAuthMiddleware {
	return &WorkerAuthMiddleware{
		RedisClient: redisClient,
	}
}

// Handle Worker认证处理
// 修复 C-19：原实现将 Redis 故障（网络错误/超时）与"密钥未配置"混为一谈，
// 一律返回 401，导致 Worker 误认为自己的密钥无效而反复重试。
// 现区分 redis.Nil（未配置→401）与其他 Redis 错误（基础设施故障→503），
// 并设置 3s 超时避免 Redis 故障时请求挂起。
func (m *WorkerAuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 从请求头获取Install Key
		installKey := r.Header.Get("X-Worker-Key")
		if installKey == "" {
			workerUnauthorized(w, "未提供Worker认证密钥")
			logx.Errorf("[WorkerAuth] Missing X-Worker-Key header from %s", r.RemoteAddr)
			return
		}

		// 从Redis获取存储的Install Key（带超时，避免 Redis 故障时挂起）
		installKeyKey := "cscan:worker:install_key"
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		storedKey, err := m.RedisClient.Get(ctx, installKeyKey).Result()
		cancel()

		if err != nil {
			if errors.Is(err, redis.Nil) {
				// 密钥未配置：业务问题，返回 401
				workerUnauthorized(w, "服务端未配置安装密钥")
				logx.Errorf("[WorkerAuth] Install key not configured in Redis")
				return
			}
			// Redis 基础设施故障：返回 503，避免 Worker 误认为密钥无效
			logx.Errorf("[WorkerAuth] Redis unavailable during worker auth: %v", err)
			workerServiceUnavailable(w, "认证服务暂时不可用")
			return
		}
		if storedKey == "" {
			workerUnauthorized(w, "服务端未配置安装密钥")
			logx.Errorf("[WorkerAuth] Install key not configured in Redis")
			return
		}

		// 验证Install Key
		if subtle.ConstantTimeCompare([]byte(installKey), []byte(storedKey)) != 1 {
			workerUnauthorized(w, "Worker认证密钥无效")
			logx.Errorf("[WorkerAuth] Invalid install key attempt from %s", r.RemoteAddr)
			return
		}

		// 可选：从请求头获取Worker名称并存入Context
		workerName := r.Header.Get("X-Worker-Name")
		if workerName != "" {
			ctx := context.WithValue(r.Context(), WorkerNameKey, workerName)
			r = r.WithContext(ctx)
		}

		next(w, r)
	}
}

// workerServiceUnavailable 返回503服务不可用响应（基础设施故障）
func workerServiceUnavailable(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 503,
		"msg":  msg,
	})
}

// workerUnauthorized 返回401未授权响应
func workerUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 401,
		"msg":  msg,
	})
}

// GetWorkerName 从Context获取Worker名称
func GetWorkerName(ctx context.Context) string {
	if v := ctx.Value(WorkerNameKey); v != nil {
		return v.(string)
	}
	return ""
}
