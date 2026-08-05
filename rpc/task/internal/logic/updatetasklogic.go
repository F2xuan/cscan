package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"cscan/internal/model"
	"cscan/pkg/notify"
	"cscan/rpc/task/internal/svc"
	"cscan/rpc/task/pb"
	"cscan/internal/scheduler"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdateTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateTaskLogic {
	return &UpdateTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 更新任务状态
func (l *UpdateTaskLogic) UpdateTask(in *pb.UpdateTaskReq) (*pb.UpdateTaskResp, error) {
	// C-4 修复：使用局部 ctx，不回写 l.ctx，避免 defer cancel 后逃逸使用拿到已取消的 ctx。
	ctx, cancel := context.WithTimeout(l.ctx, 5*time.Second)
	defer cancel()

	taskId := in.TaskId
	state := in.State

	l.Logger.Infof("UpdateTask: taskId=%s, state=%s, phase=%s", taskId, state, in.Phase)

	// 更新任务进度（用于恢复机制）
	if in.Phase != "" {
		progress := 0 // 可以从 result 中解析进度
		if err := l.svcCtx.TaskRecoveryManager.UpdateTaskProgress(taskId, in.Phase, progress); err != nil {
			l.Logger.Errorf("UpdateTask: failed to update task progress: %v", err)
		}
	}

	// 从处理中集合移除（检查 Redis 错误，避免静默吞掉）
	processingKey := "cscan:task:processing"
	if err := l.svcCtx.RedisClient.SRem(ctx, processingKey, taskId).Err(); err != nil {
		l.Logger.Errorf("UpdateTask: failed to SRem processing set, taskId=%s, error=%v", taskId, err)
		return nil, status.Errorf(codes.Unavailable, "redis update failed: %v", err)
	}

	// 先读取 taskInfo（workspaceId/mainTaskId 等），再在终态时删除
	// 修复：原代码先 Del taskInfoKey 再在 updateTaskInDBWithPhase 中 Get，导致 redis: nil
	var taskInfoData string
	if state == "SUCCESS" || state == "FAILURE" || state == "COMPLETED" {
		taskInfoKey := "cscan:task:info:" + taskId
		if data, err := l.svcCtx.RedisClient.Get(ctx, taskInfoKey).Result(); err == nil {
			taskInfoData = data
		} else if err != redis.Nil {
			l.Logger.Errorf("UpdateTask: failed to Get taskInfo before delete, taskId=%s, error=%v", taskId, err)
		}
	}

	// 更新任务状态到Redis（包含当前阶段）
	// 修复 M-31：原 TTL=0（永不过期），任务结束后 statusKey 残留导致 Redis 内存持续增长。
	// 现使用 24h TTL，与 taskInfo 保持一致；任务运行中每次 UpdateTask 会刷新 TTL。
	statusKey := "cscan:task:status:" + taskId
	statusData := map[string]interface{}{
		"taskId": taskId,
		"state":  state,
		"worker": in.Worker,
		"result": in.Result,
		"phase":  in.Phase,
	}
	statusJson, _ := json.Marshal(statusData)
	if err := l.svcCtx.RedisClient.Set(ctx, statusKey, statusJson, 24*time.Hour).Err(); err != nil {
		l.Logger.Errorf("UpdateTask: failed to Set status, taskId=%s, error=%v", taskId, err)
		return nil, status.Errorf(codes.Unavailable, "redis update failed: %v", err)
	}

	// 更新进度信息到Redis（用于前端实时获取当前阶段）
	if in.Phase != "" {
		progressKey := "cscan:task:progress:" + taskId
		progressData := map[string]interface{}{
			"currentPhase": in.Phase,
		}
		progressJson, _ := json.Marshal(progressData)
		if err := l.svcCtx.RedisClient.Set(ctx, progressKey, progressJson, 24*time.Hour).Err(); err != nil {
			l.Logger.Errorf("UpdateTask: failed to Set progress, taskId=%s, error=%v", taskId, err)
			return nil, status.Errorf(codes.Unavailable, "redis update failed: %v", err)
		}
	}

	// 如果任务完成或失败，清理执行记录和任务信息
	if state == "SUCCESS" || state == "FAILURE" || state == "COMPLETED" {
		if err := l.svcCtx.TaskRecoveryManager.RemoveTaskExecution(taskId); err != nil {
			l.Logger.Errorf("UpdateTask: failed to remove task execution: %v", err)
		}
		// 清理任务信息（检查 Redis 错误）
		taskInfoKey := "cscan:task:info:" + taskId
		if err := l.svcCtx.RedisClient.Del(ctx, taskInfoKey).Err(); err != nil {
			l.Logger.Errorf("UpdateTask: failed to Del taskInfo, taskId=%s, error=%v", taskId, err)
			return nil, status.Errorf(codes.Unavailable, "redis update failed: %v", err)
		}

		// 添加到完成集合
		completedKey := "cscan:task:completed"
		taskInfo := scheduler.TaskInfo{
			TaskId: taskId,
		}
		taskJson, _ := json.Marshal(taskInfo)
		if err := l.svcCtx.RedisClient.SAdd(ctx, completedKey, string(taskJson)).Err(); err != nil {
			l.Logger.Errorf("UpdateTask: failed to SAdd completed set, taskId=%s, error=%v", taskId, err)
			return nil, status.Errorf(codes.Unavailable, "redis update failed: %v", err)
		}
	}

	// 更新数据库中的任务状态（包括开始时间、结束时间、进度、当前阶段）
	// 传入已读取的 taskInfoData，避免再次读取已删除的 key
	l.updateTaskInDBWithPhase(taskId, state, in.Result, in.Phase, taskInfoData)

	return &pb.UpdateTaskResp{
		Success: true,
		Message: "Task status updated",
	}, nil
}

// updateTaskInDB 更新数据库中的任务状态
func (l *UpdateTaskLogic) updateTaskInDB(taskId, state, result string) {
	l.updateTaskInDBWithPhase(taskId, state, result, "", "")
}

// updateTaskInDBWithPhase 更新数据库中的任务状态（包含阶段）
// taskInfoData: 预读取的 taskInfo JSON（终态时由调用方提前读取，避免 Del 后再 Get 导致 redis:nil）
func (l *UpdateTaskLogic) updateTaskInDBWithPhase(taskId, state, result, phase, taskInfoData string) {
	// 如果状态为空且阶段为空，只是进度更新，不更新数据库状态
	if state == "" && phase == "" {
		l.Logger.Infof("UpdateTask: state and phase are empty for taskId=%s, skipping DB update (progress only)", taskId)
		return
	}

	// 从Redis获取任务信息（workspaceId）
	// 如果调用方已预读取（终态时 taskInfo 已被 Del），直接使用
	var taskInfo map[string]interface{}
	if taskInfoData != "" {
		if err := json.Unmarshal([]byte(taskInfoData), &taskInfo); err != nil {
			l.Logger.Errorf("UpdateTask: failed to parse pre-read task info, taskId=%s, error=%v", taskId, err)
			return
		}
	} else {
		taskInfoKey := "cscan:task:info:" + taskId
		data, err := l.svcCtx.RedisClient.Get(l.ctx, taskInfoKey).Result()
		if err != nil {
			l.Logger.Errorf("UpdateTask: failed to get task info from Redis, taskId=%s, error=%v", taskId, err)
			return
		}
		if err := json.Unmarshal([]byte(data), &taskInfo); err != nil {
			l.Logger.Errorf("UpdateTask: failed to parse task info, taskId=%s, error=%v", taskId, err)
			return
		}
	}

	workspaceId, _ := taskInfo["workspaceId"].(string)
	mainTaskId, _ := taskInfo["mainTaskId"].(string) // MongoDB ObjectID (Hex) 或 UUID（快速验证类任务）
	subTaskCount := 1
	if count, ok := taskInfo["subTaskCount"].(float64); ok {
		subTaskCount = int(count)
	}
	if workspaceId == "" {
		l.Logger.Errorf("UpdateTask: workspaceId is empty, taskId=%s", taskId)
		return
	}

	// 快速验证类任务（指纹验证、POC验证）的 taskId/mainTaskId 是 UUID 格式，
	// 不是有效的 MongoDB ObjectID，且这类任务不在 MongoDB 中创建 MainTask 记录，
	// 因此跳过数据库更新（结果已通过 Redis statusKey 返回给 API）。
	if mainTaskId == "" || !isValidObjectID(mainTaskId) {
		l.Logger.Debugf("UpdateTask: taskId=%s has non-ObjectID mainTaskId=%s (quick validation task), skipping DB update", taskId, mainTaskId)
		return
	}

	// 获取任务模型
	taskModel := l.svcCtx.GetMainTaskModel(workspaceId)
	now := time.Now()

	// 构建更新字段
	update := bson.M{}

	// 如果有状态，更新状态
	if state != "" {
		update["status"] = state
	}

	// 如果有阶段，更新当前阶段
	if phase != "" {
		update["current_phase"] = phase
	}

	l.Logger.Infof("UpdateTask: taskId=%s, mainTaskId=%s, subTaskCount=%d, state=%s, phase=%s", taskId, mainTaskId, subTaskCount, state, phase)

	// 根据状态设置不同字段
	switch state {
	case "STARTED":
		// 任务开始时设置开始时间和状态
		// 检查主任务当前状态，如果已经是STARTED则不重复设置
		task, err := taskModel.FindById(l.ctx, mainTaskId)
		if err != nil {
			l.Logger.Errorf("UpdateTask: failed to find task, mainTaskId=%s, error=%v", mainTaskId, err)
			// 查询失败时仍然尝试更新状态和开始时间
			update["start_time"] = now
		} else if task == nil {
			// 主任务不存在，仅记录日志，不更新状态
			l.Logger.Errorf("UpdateTask: main task not found, mainTaskId=%s", mainTaskId)
			update["start_time"] = now
		} else if task.Status == "STARTED" {
			// 主任务已经是STARTED状态，只更新阶段（如果有）
			if phase != "" {
				l.Logger.Infof("UpdateTask: main task %s already STARTED, updating phase only", mainTaskId)
				update = bson.M{"current_phase": phase}
			} else {
				l.Logger.Infof("UpdateTask: main task %s already STARTED, skipping update", mainTaskId)
				return
			}
		} else {
			// 主任务不是STARTED状态（如PENDING/CREATED），更新状态和开始时间
			l.Logger.Infof("UpdateTask: updating main task %s from %s to STARTED", mainTaskId, task.Status)
			update["start_time"] = now
		}
	case "SUCCESS", "COMPLETED":
		// 如果有多个子任务（subTaskCount > 1），不在这里更新主任务状态
		// 主任务的完成状态由 IncrSubTaskDone 在所有子任务完成后设置
		if subTaskCount > 1 {
			l.Logger.Infof("UpdateTask: task %s has %d sub-tasks, skipping status update (managed by IncrSubTaskDone)", taskId, subTaskCount)
			return
		}
		// 单任务（subTaskCount <= 1）完成时设置结束时间
		update["end_time"] = now
		update["result"] = result
		// 触发任务完成通知
		l.sendTaskNotification(workspaceId, mainTaskId, state)
	case "FAILURE":
		// 如果有多个子任务（subTaskCount > 1），不在这里更新主任务状态
		// 主任务的失败状态由 IncrSubTaskDone 在所有子任务完成后设置
		if subTaskCount > 1 {
			l.Logger.Infof("UpdateTask: task %s has %d sub-tasks, skipping FAILURE status update (managed by IncrSubTaskDone)", taskId, subTaskCount)
			return
		}
		// 单任务（subTaskCount <= 1）失败时设置结束时间
		update["end_time"] = now
		update["result"] = result
		// 触发任务失败通知
		l.sendTaskNotification(workspaceId, mainTaskId, state)
	case "STOPPED":
		// 任务停止时设置结束时间
		update["end_time"] = now
		update["result"] = "任务已停止"
	case "":
		// 只更新阶段，不更新状态
		if phase == "" {
			return
		}
		// phase 已经在上面设置了，直接更新
	}

	// 更新数据库，mainTaskId 是 MongoDB ObjectID
	if mainTaskId != "" {
		if err := taskModel.Update(l.ctx, mainTaskId, update); err != nil {
			l.Logger.Errorf("UpdateTask: failed to update task in DB, mainTaskId=%s, error=%v", mainTaskId, err)
		} else {
			l.Logger.Infof("UpdateTask: task updated in DB, mainTaskId=%s, state=%s", mainTaskId, state)
		}
	}
}

// sendTaskNotification 发送任务完成通知
func (l *UpdateTaskLogic) sendTaskNotification(workspaceId, mainTaskId, status string) {
	// 获取任务详情
	taskModel := l.svcCtx.GetMainTaskModel(workspaceId)
	task, err := taskModel.FindById(l.ctx, mainTaskId)
	if err != nil {
		l.Logger.Errorf("sendTaskNotification: failed to get task, mainTaskId=%s, error=%v", mainTaskId, err)
		return
	}
	if task == nil {
		l.Logger.Errorf("sendTaskNotification: task not found, mainTaskId=%s", mainTaskId)
		return
	}

	// T1.2: baseline 抑制。首次扫描完成时建立基线；首次扫描产生的新增只入 scan_diff，
	// 不进通知（避免 G2 通知轰炸）。基线仅建立一次，后续扫描不再抑制。
	baselineModel := model.NewWorkspaceBaselineModel(l.svcCtx.MongoDB)
	baselineJustEstablished := false
	if existing, gerr := baselineModel.Get(l.ctx, workspaceId); gerr == nil && existing == nil {
		if _, eerr := baselineModel.Establish(l.ctx, workspaceId, mainTaskId); eerr != nil {
			l.Logger.Errorf("[Baseline] establish failed: %v", eerr)
		} else {
			baselineJustEstablished = true
			l.Logger.Infof("[Baseline] established for workspace=%s task=%s", workspaceId, mainTaskId)
		}
	}

	// 获取资产和漏洞统计
	assetModel := l.svcCtx.GetAssetModel(workspaceId)
	vulModel := l.svcCtx.GetVulModel(workspaceId)

	assetCount, _ := assetModel.CountByTaskId(l.ctx, mainTaskId)
	vulCount, _ := vulModel.CountByTaskId(l.ctx, mainTaskId)

	// 获取启用的通知配置
	configs, err := l.svcCtx.NotifyConfigModel.FindEnabled(l.ctx)
	if err != nil {
		l.Logger.Errorf("sendTaskNotification: failed to get notify configs, error=%v", err)
		return
	}

	if len(configs) == 0 {
		l.Logger.Infof("sendTaskNotification: no enabled notify configs")
		return
	}

	// 构建通知配置列表
	var configItems []notify.ConfigItem
	var webURL string // 用于生成报告URL
	for _, c := range configs {
		item := notify.ConfigItem{
			Provider:        c.Provider,
			Config:          c.Config,
			Status:          c.Status,
			MessageTemplate: c.MessageTemplate,
			WebURL:          c.WebURL,
		}
		// 转换高危过滤配置
		if c.HighRiskFilter != nil {
			item.HighRiskFilter = &notify.HighRiskFilter{
				Enabled:               c.HighRiskFilter.Enabled,
				HighRiskFingerprints:  c.HighRiskFilter.HighRiskFingerprints,
				HighRiskPorts:         c.HighRiskFilter.HighRiskPorts,
				HighRiskPocSeverities: c.HighRiskFilter.HighRiskPocSeverities,
				NewAssetNotify:        c.HighRiskFilter.NewAssetNotify,
				NewRiskNotify:         c.HighRiskFilter.NewRiskNotify,
				FixedNotify:           c.HighRiskFilter.FixedNotify,
			}
		}
		configItems = append(configItems, item)
		// 获取第一个配置的WebURL作为报告URL的基础
		if webURL == "" && c.WebURL != "" {
			webURL = c.WebURL
		}
	}

	// 加载全局高危过滤配置并合并到没有自带有效 HighRiskFilter 的配置项
	// 如果通知配置启用了高危过滤但没有设置任何条件，也要使用全局配置
	globalHighRiskFilter := l.loadGlobalHighRiskFilter()
	if globalHighRiskFilter != nil && globalHighRiskFilter.Enabled {
		l.Logger.Infof("sendTaskNotification: global filter is enabled, fingerprints=%v, ports=%v, severities=%v",
			globalHighRiskFilter.HighRiskFingerprints, globalHighRiskFilter.HighRiskPorts, globalHighRiskFilter.HighRiskPocSeverities)

		for i := range configItems {
			if configItems[i].HighRiskFilter == nil {
				// 没有 HighRiskFilter，直接使用全局配置
				configItems[i].HighRiskFilter = globalHighRiskFilter
				l.Logger.Infof("sendTaskNotification: using global filter for provider %s (no local filter)", configItems[i].Provider)
			} else if configItems[i].HighRiskFilter.Enabled {
				// 通知配置启用了高危过滤，检查是否有设置有效条件
				hasValidFilter := len(configItems[i].HighRiskFilter.HighRiskFingerprints) > 0 ||
					len(configItems[i].HighRiskFilter.HighRiskPorts) > 0 ||
					len(configItems[i].HighRiskFilter.HighRiskPocSeverities) > 0 ||
					configItems[i].HighRiskFilter.NewAssetNotify

				if !hasValidFilter {
					// 启用了过滤但没有设置条件，使用全局配置
					configItems[i].HighRiskFilter = globalHighRiskFilter
					l.Logger.Infof("sendTaskNotification: using global high-risk filter for provider %s (no valid local filter)", configItems[i].Provider)
				} else {
					l.Logger.Infof("sendTaskNotification: provider %s has valid local filter, not using global", configItems[i].Provider)
				}
			}
		}
	} else {
		l.Logger.Infof("sendTaskNotification: global filter is nil or disabled, globalHighRiskFilter=%v", globalHighRiskFilter)
	}

	// 构建报告URL
	reportURL := ""
	if webURL != "" {
		// 去除末尾的斜杠
		webURL = strings.TrimSuffix(webURL, "/")
		reportURL = fmt.Sprintf("%s/report?taskId=%s", webURL, mainTaskId)
	}

	// 构建通知结果
	result := &notify.NotifyResult{
		TaskId:      mainTaskId,
		TaskName:    task.Name,
		Status:      status,
		AssetCount:  int(assetCount),
		VulCount:    int(vulCount),
		WorkspaceId: workspaceId,
		ReportURL:   reportURL,
	}

	// 设置时间（处理指针类型）
	if task.StartTime != nil {
		result.StartTime = *task.StartTime
	}
	if task.EndTime != nil {
		result.EndTime = *task.EndTime
	}

	// 计算耗时
	if task.StartTime != nil && task.EndTime != nil {
		d := task.EndTime.Sub(*task.StartTime)
		if d.Hours() >= 1 {
			result.Duration = d.Round(time.Minute).String()
		} else if d.Minutes() >= 1 {
			result.Duration = d.Round(time.Second).String()
		} else {
			result.Duration = d.Round(time.Millisecond).String()
		}
	}

	// 收集高危信息（用于高危过滤判断）
	result.HighRiskInfo = l.collectHighRiskInfo(workspaceId, mainTaskId, configItems)

	// T1.2: 首次扫描（刚建立基线）不产生新增资产通知，仅发常规完成通知
	if baselineJustEstablished && result.HighRiskInfo != nil {
		result.HighRiskInfo.NewAssetCount = 0
		result.HighRiskInfo.NewAssetList = nil
	}

	// 异步发送通知
	notify.SendNotificationAsync(l.ctx, configItems, result)
	l.Logger.Infof("sendTaskNotification: notification queued for task %s, status=%s", mainTaskId, status)
}

// collectHighRiskInfo 收集任务的高危信息（委托给公共实现，消除重复 B5）
func (l *UpdateTaskLogic) collectHighRiskInfo(workspaceId, mainTaskId string, configs []notify.ConfigItem) *notify.HighRiskInfo {
	return collectHighRiskInfoShared(l.ctx, l.svcCtx, workspaceId, mainTaskId, configs)
}

// loadGlobalHighRiskFilter 从 system_config 集合加载全局高危过滤配置
func (l *UpdateTaskLogic) loadGlobalHighRiskFilter() *notify.HighRiskFilter {
	collection := l.svcCtx.MongoDB.Collection("system_config")

	var result struct {
		Key    string   `bson:"key"`
		Config bson.Raw `bson:"config"`
	}

	err := collection.FindOne(l.ctx, bson.M{"key": "high_risk_filter_config"}).Decode(&result)
	if err != nil {
		l.Logger.Infof("loadGlobalHighRiskFilter: not found in DB, error=%v", err)
		return nil
	}

	var config struct {
		Enabled               bool   `bson:"enabled" json:"enabled"`
		HighRiskFingerprints  []string `bson:"high_risk_fingerprints" json:"highRiskFingerprints"`
		HighRiskPorts         []int  `bson:"high_risk_ports" json:"highRiskPorts"`
		HighRiskPocSeverities []string `bson:"high_risk_poc_severities" json:"highRiskPocSeverities"`
		NewAssetNotify        bool   `bson:"new_asset_notify" json:"newAssetNotify"`
		NewRiskNotify         *bool  `bson:"new_risk_notify,omitempty" json:"newRiskNotify,omitempty"`
		FixedNotify           *bool  `bson:"fixed_notify,omitempty" json:"fixedNotify,omitempty"`
	}

	if err := bson.Unmarshal(result.Config, &config); err != nil {
		l.Logger.Errorf("loadGlobalHighRiskFilter: failed to unmarshal config: %v", err)
		return nil
	}

	l.Logger.Infof("loadGlobalHighRiskFilter: enabled=%v, fingerprints=%v, ports=%v, severities=%v, newAsset=%v, newRisk=%v, fixed=%v",
		config.Enabled, config.HighRiskFingerprints, config.HighRiskPorts, config.HighRiskPocSeverities, config.NewAssetNotify, config.NewRiskNotify, config.FixedNotify)

	return &notify.HighRiskFilter{
		Enabled:               config.Enabled,
		HighRiskFingerprints:  config.HighRiskFingerprints,
		HighRiskPorts:         config.HighRiskPorts,
		HighRiskPocSeverities: config.HighRiskPocSeverities,
		NewAssetNotify:        config.NewAssetNotify,
		NewRiskNotify:         config.NewRiskNotify,
		FixedNotify:           config.FixedNotify,
	}
}

// isValidObjectID 判断字符串是否为有效的 MongoDB ObjectID（24位十六进制）
func isValidObjectID(s string) bool {
	if len(s) != 24 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
