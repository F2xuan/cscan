package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"cscan/api/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// WorkerNameKey Context key for worker name
const WorkerNameKey ContextKey = "workerName"

// WorkerAuthMiddleware Worker认证中间件
type WorkerAuthMiddleware struct {
	svcCtx *svc.ServiceContext
}

// NewWorkerAuthMiddleware 创建Worker认证中间件
func NewWorkerAuthMiddleware(svcCtx *svc.ServiceContext) *WorkerAuthMiddleware {
	return &WorkerAuthMiddleware{
		svcCtx: svcCtx,
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

		// 统一双密钥校验（环境变量 CSCAN_WORKER_KEY 或 Redis install_key）
		valid, infraError := m.svcCtx.ValidateWorkerKey(r.Context(), installKey)
		if infraError {
			// Redis 基础设施故障：返回 503，避免 Worker 误认为密钥无效
			logx.Errorf("[WorkerAuth] Auth service unavailable from %s", r.RemoteAddr)
			workerServiceUnavailable(w, "认证服务暂时不可用")
			return
		}
		if !valid {
			workerUnauthorized(w, "Worker认证密钥无效")
			logx.Errorf("[WorkerAuth] Invalid worker key attempt from %s", r.RemoteAddr)
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
