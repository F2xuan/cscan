package logic

import (
	"context"
	"encoding/json"
	"time"

	"cscan/api/internal/svc"
	"cscan/internal/model"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

// RecoveredTaskInfo 恢复的任务信息
type RecoveredTaskInfo struct {
	TaskId      string `json:"taskId"`
	MainTaskId  string `json:"mainTaskId"`
	WorkspaceId string `json:"workspaceId"`
	Status      string `json:"status"`
	StartTime   string `json:"startTime"`
}

// requeueTask 将任务重新入队到公共队列
// 1. 更新 MongoDB 状态为 PENDING
// 2. 注入 resumeState（如有 TaskState）
// 3. 推入 Redis 公共队列
func requeueTask(ctx context.Context, svcCtx *svc.ServiceContext, task *model.MainTask, workspaceId string) (*RecoveredTaskInfo, error) {
	taskModel := svcCtx.GetMainTaskModel(workspaceId)
	if err := taskModel.UpdateByTaskId(ctx, task.TaskId, bson.M{
		"status":      "PENDING",
		"update_time": time.Now(),
	}); err != nil {
		return nil, err
	}

	// 注入 resumeState 以保留已完成阶段的进度
	configStr := task.Config
	if task.TaskState != "" {
		var configMap map[string]interface{}
		if err := json.Unmarshal([]byte(task.Config), &configMap); err == nil {
			configMap["resumeState"] = task.TaskState
			if newConfig, err := json.Marshal(configMap); err == nil {
				configStr = string(newConfig)
			}
		}
	}

	taskInfo := map[string]interface{}{
		"taskId":      task.TaskId,
		"mainTaskId":  task.TaskId,
		"workspaceId": workspaceId,
		"taskName":    task.Name,
		"config":      configStr,
		"priority":    5,
		"createTime":  time.Now().Format("2006-01-02 15:04:05"),
	}

	taskData, _ := json.Marshal(taskInfo)
	score := float64(time.Now().Unix()) - 5000
	publicQueueKey := "cscan:task:queue"
	if err := svcCtx.RedisClient.ZAdd(ctx, publicQueueKey, redis.Z{
		Score:  score,
		Member: taskData,
	}).Err(); err != nil {
		return nil, err
	}

	startTimeStr := ""
	if task.StartTime != nil {
		startTimeStr = task.StartTime.Format("2006-01-02 15:04:05")
	}

	return &RecoveredTaskInfo{
		TaskId:      task.TaskId,
		MainTaskId:  task.TaskId,
		WorkspaceId: workspaceId,
		Status:      task.Status,
		StartTime:   startTimeStr,
	}, nil
}

// findStartedTask 查找指定 taskId 且状态为 STARTED 的任务（单租户：default）
func findStartedTask(ctx context.Context, svcCtx *svc.ServiceContext, taskId string) (*model.MainTask, string) {
	taskModel := svcCtx.GetMainTaskModel("default")
	task, err := taskModel.FindByTaskId(ctx, taskId)
	if err == nil && task != nil && task.Status == "STARTED" {
		return task, "default"
	}
	return nil, ""
}

// RecoverOrphanedByHeartbeat 通过 Redis 心跳快速检测离线 Worker 的任务并恢复
// 遍历 cscan:task:processing 集合，检查每个任务对应 Worker 的心跳 key 是否存在
// 如果心跳 key 不存在（Worker 已离线），立即恢复该任务
func RecoverOrphanedByHeartbeat(ctx context.Context, svcCtx *svc.ServiceContext) ([]RecoveredTaskInfo, error) {
	processingKey := "cscan:task:processing"
	taskIds, err := svcCtx.RedisClient.SMembers(ctx, processingKey).Result()
	if err != nil {
		logx.Errorf("[OrphanedTaskRecovery] Failed to get processing tasks: %v", err)
		return nil, err
	}

	if len(taskIds) == 0 {
		return nil, nil
	}

	var recoveredTasks []RecoveredTaskInfo

	for _, taskId := range taskIds {
		execKey := "cscan:task:execution:" + taskId
		execData, err := svcCtx.RedisClient.Get(ctx, execKey).Result()
		if err != nil {
			continue
		}

		var execInfo struct {
			WorkerName string `json:"workerName"`
		}
		if err := json.Unmarshal([]byte(execData), &execInfo); err != nil || execInfo.WorkerName == "" {
			continue
		}

		workerKey := "cscan:worker:" + execInfo.WorkerName
		exists, err := svcCtx.RedisClient.Exists(ctx, workerKey).Result()
		if err != nil || exists > 0 {
			continue
		}

		svcCtx.RedisClient.SRem(ctx, processingKey, taskId)

		foundTask, workspaceId := findStartedTask(ctx, svcCtx, taskId)
		if foundTask == nil {
			continue
		}

		info, err := requeueTask(ctx, svcCtx, foundTask, workspaceId)
		if err != nil {
			logx.Errorf("[OrphanedTaskRecovery] Failed to requeue task %s: %v", taskId, err)
			continue
		}

		recoveredTasks = append(recoveredTasks, *info)
		logx.Infof("[OrphanedTaskRecovery] Heartbeat check: recovered task %s (worker %s offline)", taskId, execInfo.WorkerName)
	}

	return recoveredTasks, nil
}

// RecoverOrphanedTasks 查找并恢复卡住的任务
func RecoverOrphanedTasks(ctx context.Context, svcCtx *svc.ServiceContext, timeout time.Duration) ([]RecoveredTaskInfo, error) {
	cutoffTime := time.Now().Add(-timeout)
	var recoveredTasks []RecoveredTaskInfo

	taskModel := svcCtx.GetMainTaskModel("default")

	filter := bson.M{
		"status": "STARTED",
		"update_time": bson.M{
			"$lt": cutoffTime,
		},
	}

	tasks, err := taskModel.Find(ctx, filter, 0, 50)
	if err != nil {
		logx.Errorf("[OrphanedTaskRecovery] Failed to find tasks: %v", err)
		return nil, err
	}

	for i := range tasks {
		task := tasks[i]
		info, err := requeueTask(ctx, svcCtx, &task, "default")
		if err != nil {
			logx.Errorf("[OrphanedTaskRecovery] Failed to requeue task %s: %v", task.TaskId, err)
			continue
		}

		recoveredTasks = append(recoveredTasks, *info)
		logx.Infof("[OrphanedTaskRecovery] Recovered task %s", task.TaskId)
	}

	return recoveredTasks, nil
}

// RecoverWorkerTasks Worker 离线时立即恢复其处理中的任务
// 在 Worker 调用 /api/v1/worker/offline 时触发，立即扫描该 Worker 的任务并重新入队
func RecoverWorkerTasks(ctx context.Context, svcCtx *svc.ServiceContext, workerName string) ([]RecoveredTaskInfo, error) {
	processingKey := "cscan:task:processing"
	taskIds, err := svcCtx.RedisClient.SMembers(ctx, processingKey).Result()
	if err != nil {
		logx.Errorf("[WorkerOffline] Failed to get processing tasks: %v", err)
		return nil, err
	}

	if len(taskIds) == 0 {
		return nil, nil
	}

	var recoveredTasks []RecoveredTaskInfo

	for _, taskId := range taskIds {
		// context 超时或取消时，已恢复的任务保留，不再继续
		if ctx.Err() != nil {
			logx.Infof("[WorkerOffline] Recovery cancelled for %s (ctx: %v), returning %d recovered tasks",
				workerName, ctx.Err(), len(recoveredTasks))
			return recoveredTasks, nil
		}

		execKey := "cscan:task:execution:" + taskId
		execData, err := svcCtx.RedisClient.Get(ctx, execKey).Result()
		if err != nil {
			continue
		}

		var execInfo struct {
			WorkerName string `json:"workerName"`
		}
		if err := json.Unmarshal([]byte(execData), &execInfo); err != nil || execInfo.WorkerName != workerName {
			continue
		}

		svcCtx.RedisClient.SRem(ctx, processingKey, taskId)
		svcCtx.RedisClient.Del(ctx, execKey)
		svcCtx.RedisClient.Del(ctx, "cscan:task:status:"+taskId)

		foundTask, workspaceId := findStartedTask(ctx, svcCtx, taskId)
		if foundTask == nil {
			continue
		}

		info, err := requeueTask(ctx, svcCtx, foundTask, workspaceId)
		if err != nil {
			logx.Errorf("[WorkerOffline] Failed to requeue task %s: %v", taskId, err)
			continue
		}

		recoveredTasks = append(recoveredTasks, *info)
		logx.Infof("[WorkerOffline] Recovered task %s from offline worker %s", taskId, workerName)
	}

	return recoveredTasks, nil
}

// CleanupStaleProcessingTasks 清理过期的处理中任务记录
func CleanupStaleProcessingTasks(ctx context.Context, svcCtx *svc.ServiceContext, workerName string) {
	processingKey := "cscan:task:processing"
	taskIds, err := svcCtx.RedisClient.SMembers(ctx, processingKey).Result()
	if err != nil {
		return
	}

	cleaned := 0
	for _, taskId := range taskIds {
		statusKey := "cscan:task:status:" + taskId
		statusData, err := svcCtx.RedisClient.Get(ctx, statusKey).Result()
		if err != nil {
			svcCtx.RedisClient.SRem(ctx, processingKey, taskId)
			cleaned++
			continue
		}

		if workerName != "" {
			var status map[string]interface{}
			if err := json.Unmarshal([]byte(statusData), &status); err == nil {
				if worker, ok := status["worker"].(string); ok && worker == workerName {
					svcCtx.RedisClient.SRem(ctx, processingKey, taskId)
					svcCtx.RedisClient.Del(ctx, statusKey)
					cleaned++
				}
			}
		}
	}

	if cleaned > 0 {
		logx.Infof("[OrphanedTaskRecovery] Cleaned up %d stale processing records", cleaned)
	}
}

// RecoverStaleMongoTasks 从 MongoDB 直接查找卡住的 STARTED 任务并恢复
// 作为 Redis 检测的兜底机制，处理 Worker 异常退出（OOM/SIGKILL）导致 Redis 状态不一致的情况
func RecoverStaleMongoTasks(ctx context.Context, svcCtx *svc.ServiceContext, timeout time.Duration) ([]RecoveredTaskInfo, error) {
	cutoffTime := time.Now().Add(-timeout)
	var recoveredTasks []RecoveredTaskInfo

	taskModel := svcCtx.GetMainTaskModel("default")

	// 查找 STARTED 状态且 update_time 超时的任务
	filter := bson.M{
		"status": "STARTED",
		"update_time": bson.M{
			"$lt": cutoffTime,
		},
	}

	tasks, err := taskModel.Find(ctx, filter, 0, 50)
	if err != nil {
		logx.Errorf("[StaleTaskRecovery] Failed to find tasks: %v", err)
		return nil, err
	}

	for i := range tasks {
		task := tasks[i]
		// 检查是否已在 Redis 处理集合中（避免重复恢复）
		processingKey := "cscan:task:processing"
		isMember, _ := svcCtx.RedisClient.SIsMember(ctx, processingKey, task.TaskId).Result()
		if isMember {
			// 在处理集合中，检查 Worker 是否在线
			execKey := "cscan:task:execution:" + task.TaskId
			execData, err := svcCtx.RedisClient.Get(ctx, execKey).Result()
			if err == nil {
				var execInfo struct {
					WorkerName string `json:"workerName"`
				}
				if json.Unmarshal([]byte(execData), &execInfo) == nil && execInfo.WorkerName != "" {
					workerKey := "cscan:worker:" + execInfo.WorkerName
					exists, _ := svcCtx.RedisClient.Exists(ctx, workerKey).Result()
					if exists > 0 {
						// Worker 在线，跳过
						continue
					}
				}
			}
			// Worker 离线，清理 Redis 状态
			svcCtx.RedisClient.SRem(ctx, processingKey, task.TaskId)
			svcCtx.RedisClient.Del(ctx, execKey)
		}

		info, err := requeueTask(ctx, svcCtx, &task, "default")
		if err != nil {
			logx.Errorf("[StaleTaskRecovery] Failed to requeue task %s: %v", task.TaskId, err)
			continue
		}

		recoveredTasks = append(recoveredTasks, *info)
		logx.Infof("[StaleTaskRecovery] Recovered stale task %s (last update: %v)",
			task.TaskId, task.UpdateTime)
	}

	return recoveredTasks, nil
}
