package worker

import (
	"context"
	"net/http"
	"strings"
	"time"

	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
)

// AuditService 审计服务
type AuditService struct {
	svcCtx *svc.ServiceContext
}

// NewAuditService 创建审计服务
func NewAuditService(svcCtx *svc.ServiceContext) *AuditService {
	return &AuditService{
		svcCtx: svcCtx,
	}
}

// RecordFileOperation 记录文件操作
func (s *AuditService) RecordFileOperation(ctx context.Context, r *http.Request, logType model.AuditLogType, workerName, path string, success bool, errMsg string, duration time.Duration) {
	// 在 goroutine 之前提取用户信息，避免 context 失效
	userId := middleware.GetUserId(ctx)
	username := middleware.GetUsername(ctx)
	clientIP := getClientIPFromReq(r)

	go func() {
		auditCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		log := &model.AuditLog{
			Type:       logType,
			WorkerName: workerName,
			UserId:     userId,
			Username:   username,
			ClientIP:   clientIP,
			Path:       path,
			Success:    success,
			Error:      errMsg,
			Duration:   duration.Milliseconds(),
			CreateTime: time.Now(),
		}

		if err := s.svcCtx.AuditLogModel.RecordAudit(auditCtx, log); err != nil {
			logx.Errorf("[AuditService] Failed to record file operation: %v", err)
		}
	}()
}

// RecordTerminalOperation 记录终端操作
func (s *AuditService) RecordTerminalOperation(ctx context.Context, r *http.Request, logType model.AuditLogType, workerName, sessionId, command string, success bool, errMsg string, duration time.Duration) {
	// 在 goroutine 之前提取用户信息，避免 context 失效
	userId := middleware.GetUserId(ctx)
	username := middleware.GetUsername(ctx)
	clientIP := getClientIPFromReq(r)

	go func() {
		auditCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		log := &model.AuditLog{
			Type:       logType,
			WorkerName: workerName,
			UserId:     userId,
			Username:   username,
			ClientIP:   clientIP,
			SessionId:  sessionId,
			Command:    command,
			Success:    success,
			Error:      errMsg,
			Duration:   duration.Milliseconds(),
			CreateTime: time.Now(),
		}

		if err := s.svcCtx.AuditLogModel.RecordAudit(auditCtx, log); err != nil {
			logx.Errorf("[AuditService] Failed to record terminal operation: %v", err)
		}
	}()
}

// RecordConsoleAccess 记录控制台访问
func (s *AuditService) RecordConsoleAccess(ctx context.Context, r *http.Request, workerName string, success bool, errMsg string) {
	// 在 goroutine 之前提取用户信息，避免 context 失效
	userId := middleware.GetUserId(ctx)
	username := middleware.GetUsername(ctx)
	clientIP := getClientIPFromReq(r)

	go func() {
		auditCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		log := &model.AuditLog{
			Type:       model.AuditLogTypeConsoleInfo,
			WorkerName: workerName,
			UserId:     userId,
			Username:   username,
			ClientIP:   clientIP,
			Success:    success,
			Error:      errMsg,
			CreateTime: time.Now(),
		}

		if err := s.svcCtx.AuditLogModel.RecordAudit(auditCtx, log); err != nil {
			logx.Errorf("[AuditService] Failed to record console access: %v", err)
		}
	}()
}

// getClientIPFromReq 从请求获取客户端IP
func getClientIPFromReq(r *http.Request) string {
	if r == nil {
		return ""
	}

	// 尝试从X-Forwarded-For获取
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// 取第一个IP
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// 尝试从X-Real-IP获取
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// 从RemoteAddr获取
	ip := r.RemoteAddr
	// 移除端口号
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// GlobalAuditService 全局审计服务实例
var GlobalAuditService *AuditService

// InitAuditService 初始化审计服务
func InitAuditService(svcCtx *svc.ServiceContext) {
	GlobalAuditService = NewAuditService(svcCtx)
}

// GetAuditService 获取审计服务
func GetAuditService() *AuditService {
	return GlobalAuditService
}
