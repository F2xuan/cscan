package logic

import (
	"context"
	"encoding/json"
	"time"

	"cscan/rpc/task/internal/svc"
	"cscan/rpc/task/pb"
	"cscan/scheduler"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ValidateFingerprintLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewValidateFingerprintLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateFingerprintLogic {
	return &ValidateFingerprintLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ValidateFingerprint 指纹验证 - 创建验证任务并推送到队列，由Worker执行
// 与POC验证不同：指纹验证是快速操作（单次HTTP请求+规则匹配），采用同步等待模式
func (l *ValidateFingerprintLogic) ValidateFingerprint(in *pb.ValidateFingerprintReq) (*pb.ValidateFingerprintResp, error) {
	ctx, cancel := context.WithTimeout(l.ctx, 5*time.Second)
	defer cancel()

	// 1. 检查是否有在线的Worker
	workers, err := l.svcCtx.RedisClient.SMembers(ctx, "cscan:workers").Result()
	if err != nil {
		l.Logger.Errorf("ValidateFingerprint: failed to get workers, error=%v", err)
		return &pb.ValidateFingerprintResp{
			Success: false,
			Message: "获取Worker列表失败: " + err.Error(),
		}, nil
	}

	hasActiveWorker := false
	for _, worker := range workers {
		exists, _ := l.svcCtx.RedisClient.Exists(ctx, "cscan:worker:"+worker).Result()
		if exists > 0 {
			hasActiveWorker = true
			break
		}
	}

	if !hasActiveWorker {
		return &pb.ValidateFingerprintResp{
			Success: false,
			Message: "当前没有在线的扫描节点(Worker)，无法执行指纹验证。请检查Worker服务状态。",
		}, nil
	}

	// 生成任务ID
	taskId := uuid.New().String()

	// 确定任务类型
	// - activeFpId 不为空 -> 单个主动指纹验证
	// - fingerprintId 不为空 -> 单个被动指纹验证
	// - fingerprintId 为空且 scope=active -> 批量主动指纹验证
	// - fingerprintId 为空且 scope!=active -> 批量被动指纹验证
	taskType := "fingerprint_validate"
	taskName := "指纹验证"
	timeout := 30
	if in.ActiveFpId != "" {
		taskType = "active_fingerprint_validate"
		taskName = "主动指纹验证"
		timeout = 60
	} else if in.FingerprintId == "" {
		// fingerprintId为空，批量验证
		if in.Scope == "active" {
			taskType = "active_fingerprint_batch_validate"
			taskName = "批量主动指纹验证"
		} else {
			taskType = "fingerprint_batch_validate"
			taskName = "批量指纹验证"
		}
		timeout = 120 // 批量验证可能需要更长时间（主动指纹要发HTTP请求）
	}

	// 构建任务配置
	taskConfig := map[string]interface{}{
		"taskType":      taskType,
		"url":           in.Url,
		"fingerprintId": in.FingerprintId,
		"activeFpId":    in.ActiveFpId,
		"scope":         in.Scope,
		"timeout":       timeout,
	}
	configBytes, _ := json.Marshal(taskConfig)

	// 创建任务信息（高优先级）
	task := &scheduler.TaskInfo{
		TaskId:      taskId,
		MainTaskId:  taskId,
		WorkspaceId: "default",
		TaskName:    taskName,
		Config:      string(configBytes),
		Priority:    2,
	}

	// 推送到调度器队列
	if l.svcCtx.Scheduler == nil {
		l.Logger.Errorf("ValidateFingerprint: scheduler is nil, cannot push task")
		return nil, status.Errorf(codes.Internal, "scheduler not initialized")
	}
	if err := l.svcCtx.Scheduler.PushTask(ctx, task); err != nil {
		l.Logger.Errorf("ValidateFingerprint: failed to push task, taskId=%s, error=%v", taskId, err)
		return nil, status.Errorf(codes.Internal, "push task failed: %v", err)
	}

	// 持久化任务信息（24h TTL，与 CheckTask 弹出时保持一致，包含 workspaceId/mainTaskId 供 UpdateTask 更新DB）
	taskInfoKey := "cscan:task:info:" + taskId
	taskInfoData, _ := json.Marshal(map[string]interface{}{
		"taskId":        taskId,
		"mainTaskId":    taskId,
		"workspaceId":   "default",
		"taskType":      taskType,
		"taskName":      taskName,
		"url":           in.Url,
		"fingerprintId": in.FingerprintId,
		"activeFpId":    in.ActiveFpId,
		"scope":         in.Scope,
		"createTime":    time.Now().Local().Format("2006-01-02 15:04:05"),
	})
	if err := l.svcCtx.RedisClient.Set(ctx, taskInfoKey, taskInfoData, 24*time.Hour).Err(); err != nil {
		l.Logger.Errorf("ValidateFingerprint: failed to persist taskInfo, taskId=%s, error=%v", taskId, err)
	}

	l.Logger.Infof("ValidateFingerprint: task created, taskId=%s, taskType=%s, url=%s, fpId=%s, activeFpId=%s, scope=%s",
		taskId, taskType, in.Url, in.FingerprintId, in.ActiveFpId, in.Scope)

	return &pb.ValidateFingerprintResp{
		Success: true,
		Message: taskName + "任务已下发",
		TaskId:  taskId,
	}, nil
}
