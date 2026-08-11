package common

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cscan/api/internal/svc"
	"cscan/internal/model"
	"cscan/internal/scheduler"
	"cscan/pkg/utils"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

// TaskBuilder handles common task creation logic
type TaskBuilder struct {
	ctx      context.Context
	svcCtx   *svc.ServiceContext
	log      logx.Logger
	Priority int // 入队优先级（默认 PriorityLow=1）；自动触发任务注入 Background=0 以降低优先级
}

func NewTaskBuilder(ctx context.Context, svcCtx *svc.ServiceContext) *TaskBuilder {
	return &TaskBuilder{
		ctx:      ctx,
		svcCtx:   svcCtx,
		log:      logx.WithContext(ctx),
		Priority: scheduler.PriorityLow,
	}
}

// BuildAndPushSubTasks splits targets and pushes sub-tasks to Redis queue
func (b *TaskBuilder) BuildAndPushSubTasks(workspaceId string, task *model.MainTask, taskConfig map[string]interface{}) (int, error) {
	// 1. Determine Batch Size — 自动计算最佳值
	batchSize := b.CalculateOptimalBatchSize(task.Target, taskConfig)
	b.log.Infof("TaskBuilder: auto-calculated batchSize=%d for task %s", batchSize, task.TaskId)

	// 2. Split Targets
	splitter := scheduler.NewTargetSplitter(batchSize)
	batches := splitter.SplitTargets(task.Target)

	// 注意：此处原先会异步 prewrite 初始资产——用 GenerateAssetsFromTargetsWithoutDNS 把用户输入的
	// 文本目标直接 upsert 进资产表（Source="user_input", IsNewAsset=true）。这在 worker 尚未进行任何
	// 存活/端口/指纹识别前就把目标录入暴露面，属逻辑错误，已移除。
	// 资产统一由 worker 扫描完成后通过 SaveTaskResult RPC 回写（rpc/task/internal/logic/savetaskresultlogic.go），
	// prewriteInitialAssets 等函数保留但不再调用，待后续清理。

	// 3. Calculate SubTask Count
	// subTaskCount = 批次数 × (启用模块数 + 1)
	// 与 worker 端单任务应发增量口径一致：worker 每完成一个扫描模块递增 1 次，
	// 加上最终"完成"阶段递增 1 次（见 worker.go expectedTaskIncr = CountEnabledModules + 1）。
	// 进度 = done / subTaskCount × 100。两侧口径必须保持一致，否则会出现 done > count 倒挂。
	// 无任何模块启用时，subTaskCount = 批次数（仅"完成"阶段递增）。
	enabledModules := utils.CountEnabledModules(taskConfig)
	subTaskCount := len(batches) * (enabledModules + 1)
	if enabledModules == 0 {
		subTaskCount = len(batches)
	}

	// 4. Update Main Task Status
	now := time.Now()
	b.svcCtx.GetMainTaskModel(workspaceId).Update(b.ctx, task.Id.Hex(), bson.M{
		"status":         model.TaskStatusPending,
		"sub_task_count": subTaskCount,
		"sub_task_done":  0,
		"batch_count":    len(batches),
		"start_time":     now,
	})

	// 5. Cache Info to Redis
	b.cacheTaskInfo(workspaceId, task, subTaskCount, len(batches), enabledModules)

	// 6. Push Sub-Tasks
	workers := b.extractWorkers(taskConfig)

	b.log.Infof("TaskBuilder: pushing %d batches for task %s", len(batches), task.TaskId)

	for i, batch := range batches {
		if err := b.pushSingleBatch(workspaceId, task, taskConfig, batch, i, len(batches), workers); err != nil {
			b.log.Errorf("Failed to push batch %d: %v", i, err)
			// Continue pushing other batches
		}
	}

	return len(batches), nil
}

func (b *TaskBuilder) pushSingleBatch(workspaceId string, task *model.MainTask, baseConfig map[string]interface{}, batchTarget string, index, total int, workers []string) error {
	// Deep copy config
	subConfig := make(map[string]interface{})
	for k, v := range baseConfig {
		subConfig[k] = v
	}
	subConfig["target"] = batchTarget
	subConfig["subTaskIndex"] = index
	subConfig["subTaskTotal"] = total

	configBytes, err := json.Marshal(subConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal sub-task config: %w", err)
	}
	subTaskId := task.TaskId
	if total > 1 {
		subTaskId = fmt.Sprintf("%s-%d", task.TaskId, index)
	}

	schedTask := &scheduler.TaskInfo{
		TaskId:      subTaskId,
		MainTaskId:  task.Id.Hex(),
		WorkspaceId: workspaceId,
		TaskName:    task.Name,
		Config:      string(configBytes),
		Priority:    b.Priority,
		Workers:     workers,
	}

	return b.svcCtx.Scheduler.PushTask(b.ctx, schedTask)
}

func (b *TaskBuilder) cacheTaskInfo(workspaceId string, task *model.MainTask, subTaskCount, batchCount, modules int) {
	key := fmt.Sprintf("cscan:task:info:%s", task.TaskId)
	data := map[string]interface{}{
		"workspaceId":    workspaceId,
		"mainTaskId":     task.Id.Hex(),
		"subTaskCount":   subTaskCount,
		"batchCount":     batchCount,
		"enabledModules": modules,
	}
	bytes, _ := json.Marshal(data)
	b.svcCtx.RedisClient.Set(b.ctx, key, bytes, 24*time.Hour)
}

func (b *TaskBuilder) extractWorkers(config map[string]interface{}) []string {
	var workers []string
	if w, ok := config["workers"].([]interface{}); ok {
		for _, v := range w {
			if s, ok := v.(string); ok {
				workers = append(workers, s)
			}
		}
	}
	return workers
}

// CalculateOptimalBatchSize 根据目标数量和启用的模块自动计算最佳批次大小
// 核心原则：控制子任务总数（batches × modules）在合理范围内（10~30），
// 避免碎片化（子任务过多导致调度开销大）和过度聚合（单批次过大导致超时）
func (b *TaskBuilder) CalculateOptimalBatchSize(target string, taskConfig map[string]interface{}) int {
	// 如果用户显式设置了 batchSize > 0，优先使用（向后兼容）
	if bs, ok := taskConfig["batchSize"].(float64); ok && bs > 0 {
		return int(bs)
	}

	// 计算目标总数
	splitter := scheduler.NewTargetSplitter(1000000) // 用大值获取总目标数
	targetCount := splitter.GetTargetCount(target)

	// 获取启用的模块数（用于 batch 分配；为避免除零，最小取 1）
	enabledModules := utils.CountEnabledModules(taskConfig)
	if enabledModules == 0 {
		enabledModules = 1
	}

	// 最佳子任务总数范围：10~30
	// 太少 → 单批次过大 → POC扫描超时
	// 太多 → 调度开销大、进度卡顿感明显
	const (
		minSubTasks = 10
		maxSubTasks = 30
	)

	// 反推最佳批次数 = 子任务总数 / 模块数
	optimalBatches := (minSubTasks + maxSubTasks) / 2 / enabledModules
	if optimalBatches < 1 {
		optimalBatches = 1
	}

	// 反推最佳 batchSize = 目标总数 / 批次数
	batchSize := targetCount / optimalBatches
	if batchSize < 1 {
		batchSize = 1
	}

	// 限制 batchSize 在合理范围内
	// 最小 20：避免过度碎片化（如 batchSize=5 导致子任务爆炸）
	// 最大 200：避免单批次过大导致 POC 超时
	const (
		minBatchSize = 20
		maxBatchSize = 200
	)
	if batchSize < minBatchSize {
		batchSize = minBatchSize
	}
	if batchSize > maxBatchSize {
		batchSize = maxBatchSize
	}

	// 如果目标数量小于 minBatchSize，则不拆分
	if targetCount <= minBatchSize {
		return targetCount
	}

	return batchSize
}
