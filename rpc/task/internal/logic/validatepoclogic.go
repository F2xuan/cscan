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

type ValidatePocLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewValidatePocLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidatePocLogic {
	return &ValidatePocLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// POC验证 - 创建验证任务并推送到队列，由Worker执行
func (l *ValidatePocLogic) ValidatePoc(in *pb.ValidatePocReq) (*pb.ValidatePocResp, error) {
	// C-4 修复：使用局部 ctx，不回写 l.ctx，避免 defer cancel 后逃逸使用拿到已取消的 ctx。
	ctx, cancel := context.WithTimeout(l.ctx, 5*time.Second)
	defer cancel()

	// 1. 检查是否有在线的Worker
	workers, err := l.svcCtx.RedisClient.SMembers(ctx, "cscan:workers").Result()
	if err != nil {
		l.Logger.Errorf("ValidatePoc: failed to get workers, error=%v", err)
		return &pb.ValidatePocResp{
			Success: false,
			Message: "获取Worker列表失败: " + err.Error(),
		}, nil
	}

	hasActiveWorker := false
	for _, worker := range workers {
		// 检查Worker心跳key是否存在
		exists, _ := l.svcCtx.RedisClient.Exists(ctx, "cscan:worker:"+worker).Result()
		if exists > 0 {
			hasActiveWorker = true
			break
		}
	}

	if !hasActiveWorker {
		return &pb.ValidatePocResp{
			Success: false,
			Message: "当前没有在线的扫描节点(Worker)，无法执行任务。请检查Worker服务状态。",
		}, nil
	}

	// 生成任务ID
	taskId := uuid.New().String()

	// 获取workspaceId，如果未指定则使用default
	workspaceId := in.WorkspaceId
	if workspaceId == "" {
		workspaceId = "default"
	}

	// 判断是批量模式还是单目标模式
	taskType := "poc_validate"
	var targetUrls []string
	if in.BatchMode && len(in.Urls) > 0 {
		taskType = "poc_batch_validate"
		targetUrls = in.Urls
	} else if in.Url != "" {
		targetUrls = []string{in.Url}
	}

	// 构建任务配置
	taskConfig := map[string]interface{}{
		"taskType":    taskType,
		"urls":        targetUrls,
		"pocId":       in.PocId,
		"pocType":     in.PocType,
		"timeout":     in.Timeout,
		"workspaceId": workspaceId,
		"batchMode":   in.BatchMode,
	}
	// 兼容单目标模式
	if len(targetUrls) == 1 {
		taskConfig["url"] = targetUrls[0]
	}
	configBytes, _ := json.Marshal(taskConfig)

	// 创建任务信息
	taskName := "POC验证"
	if in.BatchMode {
		taskName = "POC批量扫描"
	}
	task := &scheduler.TaskInfo{
		TaskId:      taskId,
		MainTaskId:  taskId,
		WorkspaceId: workspaceId,
		TaskName:    taskName,
		Config:      string(configBytes),
		Priority:    2, // 高优先级
	}

	// 统一通过 Scheduler.PushTask 入队：
	//   1. 由 calculatePriorityScore 计算优先级分数（避免优先级失效）
	//   2. 自动感知 enablePriorityBucket 灰度开关（分桶/单 ZSet）
	//   3. 失败时返回 gRPC Internal 让调用方感知
	if l.svcCtx.Scheduler == nil {
		l.Logger.Errorf("ValidatePoc: scheduler is nil, cannot push task")
		return nil, status.Errorf(codes.Internal, "scheduler not initialized")
	}
	if err := l.svcCtx.Scheduler.PushTask(ctx, task); err != nil {
		l.Logger.Errorf("ValidatePoc: failed to push task via scheduler, taskId=%s, error=%v", taskId, err)
		return nil, status.Errorf(codes.Internal, "push task failed: %v", err)
	}

	// 持久化 taskInfo（24h TTL），供结果查询与 TaskRecoveryManager 恢复使用
	taskInfoKey := "cscan:task:info:" + taskId
	taskInfoData, _ := json.Marshal(map[string]interface{}{
		"workspaceId": workspaceId,
		"mainTaskId":  taskId,
		"taskType":    taskType,
		"urls":        targetUrls,
		"pocId":       in.PocId,
		"pocType":     in.PocType,
		"batchMode":   in.BatchMode,
		"createTime":  time.Now().Local().Format("2006-01-02 15:04:05"),
	})
	if err := l.svcCtx.RedisClient.Set(ctx, taskInfoKey, taskInfoData, 24*time.Hour).Err(); err != nil {
		// taskInfo 持久化失败不影响入队，但记录错误（恢复时无法读取 taskInfo）
		l.Logger.Errorf("ValidatePoc: failed to persist taskInfo, taskId=%s, error=%v", taskId, err)
	}

	l.Logger.Infof("ValidatePoc: task created, taskId=%s, targets=%d, pocId=%s, workspaceId=%s, batchMode=%v",
		taskId, len(targetUrls), in.PocId, workspaceId, in.BatchMode)

	return &pb.ValidatePocResp{
		Success: true,
		Message: "POC验证任务已下发",
		Matched: false,
		TaskId:  taskId,
	}, nil
}
