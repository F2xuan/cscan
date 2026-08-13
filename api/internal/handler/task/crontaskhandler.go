package task

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cscan/api/internal/svc"
	"cscan/internal/model"
	"cscan/pkg/response"
	"cscan/internal/scheduler"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
	"go.mongodb.org/mongo-driver/bson"
)

// cronParser 包级别Cron解析器（秒级精度），避免重复创建
var cronParser = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// CronTaskListReq 定时任务列表请求
type CronTaskListReq struct {
	Page     int    `json:"page,optional"`
	PageSize int    `json:"pageSize,optional"`
	Keyword  string `json:"keyword,optional"`
	TaskType string `json:"taskType,optional"` // 按任务类型过滤：scan / space_engine
}

// CronTaskListResp 定时任务列表响应
type CronTaskListResp struct {
	Code int                   `json:"code"`
	Msg  string                `json:"msg"`
	Data *CronTaskListRespData `json:"data"`
}

type CronTaskListRespData struct {
	List  []*CronTaskItem `json:"list"`
	Total int             `json:"total"`
}

type CronTaskItem struct {
	Id           string `json:"id"`
	Name         string `json:"name"`
	TaskType     string `json:"taskType"` // scan / space_engine
	ScheduleType string `json:"scheduleType"`
	CronSpec     string `json:"cronSpec"`
	ScheduleTime string `json:"scheduleTime"`
	Status       string `json:"status"`
	LastRunTime  string `json:"lastRunTime"`
	NextRunTime  string `json:"nextRunTime"`
	RunCount     int64  `json:"runCount"`

	// ===== scan 类型字段 =====
	TargetMode          string   `json:"targetMode,omitempty"`
	Target              string   `json:"target,omitempty"`
	TargetShort         string   `json:"targetShort,omitempty"` // 截断后的目标（用于列表显示）
	AssetIds            []string `json:"assetIds,omitempty"`
	OrgId               string   `json:"orgId,omitempty"`
	EnableSubdomainPull bool     `json:"enableSubdomainPull,omitempty"`
	ConfigSource        string   `json:"configSource,omitempty"`
	TemplateId          string   `json:"templateId,omitempty"`
	Config              string   `json:"config,omitempty"`

	// ===== space_engine 类型字段 =====
	Platform   string `json:"platform,omitempty"`
	Query      string `json:"query,omitempty"`
	MaxResults int    `json:"maxResults,omitempty"`
}

// CronTaskSaveReq 保存定时任务请求
type CronTaskSaveReq struct {
	Id           string `json:"id,optional"`
	Name         string `json:"name"`
	TaskType     string `json:"taskType"` // scan / space_engine
	ScheduleType string `json:"scheduleType"`
	CronSpec     string `json:"cronSpec,optional"`
	ScheduleTime string `json:"scheduleTime,optional"`

	// ===== scan 类型字段 =====
	TargetMode          string   `json:"targetMode,optional"`
	Target              string   `json:"target,optional"`
	AssetIds            []string `json:"assetIds,optional"`
	OrgId               string   `json:"orgId,optional"`
	EnableSubdomainPull bool     `json:"enableSubdomainPull,optional"`
	ConfigSource        string   `json:"configSource,optional"`
	TemplateId          string   `json:"templateId,optional"`
	Config              string   `json:"config,optional"`

	// ===== space_engine 类型字段 =====
	Platform   string `json:"platform,optional"`
	Query      string `json:"query,optional"`
	MaxResults int    `json:"maxResults,optional"`
}

// CronTaskToggleReq 开关定时任务请求
type CronTaskToggleReq struct {
	Id     string `json:"id"`
	Status string `json:"status"` // enable/disable
}

// CronTaskDeleteReq 删除定时任务请求
type CronTaskDeleteReq struct {
	Id string `json:"id"`
}

// CronTaskRunNowReq 立即执行定时任务请求
type CronTaskRunNowReq struct {
	Id string `json:"id"`
}

// CronTaskBatchDeleteReq 批量删除定时任务请求
type CronTaskBatchDeleteReq struct {
	Ids []string `json:"ids"`
}

// syncCronTaskToRedis 将MongoDB中的定时任务同步到Redis（供调度器读取）
func syncCronTaskToRedis(ctx context.Context, svcCtx *svc.ServiceContext, cronTask *model.CronTask) {
	cronKey := "cscan:cron:tasks"
	schedTask := scheduler.CronTask{
		Id:                  cronTask.CronTaskId,
		Name:                cronTask.Name,
		TaskType:            cronTask.TaskType,
		ScheduleType:        cronTask.ScheduleType,
		CronSpec:            cronTask.CronSpec,
		ScheduleTime:        cronTask.ScheduleTime,
		Status:              cronTask.Status,
		LastRunTime:         cronTask.LastRunTime,
		NextRunTime:         cronTask.NextRunTime,
		TargetMode:          cronTask.TargetMode,
		Target:              cronTask.Target,
		AssetIds:            cronTask.AssetIds,
		OrgId:               cronTask.OrgId,
		EnableSubdomainPull: cronTask.EnableSubdomainPull,
		ConfigSource:        cronTask.ConfigSource,
		TemplateId:          cronTask.TemplateId,
		Config:              cronTask.Config,
		Platform:            cronTask.Platform,
		Query:               cronTask.Query,
		MaxResults:          cronTask.MaxResults,
	}
	data, err := json.Marshal(schedTask)
	if err != nil {
		logx.Errorf("[CronTask] failed to marshal cron task for redis sync: cronTaskId=%s, err=%v", cronTask.CronTaskId, err)
		return
	}
	if err := svcCtx.RedisClient.HSet(ctx, cronKey, cronTask.CronTaskId, data).Err(); err != nil {
		logx.Errorf("[CronTask] sync to redis failed: cronTaskId=%s, err=%v", cronTask.CronTaskId, err)
	}
}

// removeCronTaskFromRedis 从Redis中删除定时任务缓存
func removeCronTaskFromRedis(ctx context.Context, svcCtx *svc.ServiceContext, cronTaskId string) {
	cronKey := "cscan:cron:tasks"
	svcCtx.RedisClient.HDel(ctx, cronKey, cronTaskId)
	// 删除运行次数记录
	runCountKey := fmt.Sprintf("cscan:cron:runcount:%s", cronTaskId)
	svcCtx.RedisClient.Del(ctx, runCountKey)
}

// expandTemplateConfig 当 configSource=template 时从 ScanTemplateModel 获取模板配置展开为 config JSON
func expandTemplateConfig(ctx context.Context, svcCtx *svc.ServiceContext, templateId string) (string, error) {
	if templateId == "" {
		return "", fmt.Errorf("模板ID不能为空")
	}
	tmpl, err := svcCtx.ScanTemplateModel.FindById(ctx, templateId)
	if err != nil {
		return "", fmt.Errorf("查询扫描模板失败: %v", err)
	}
	if tmpl == nil {
		return "", fmt.Errorf("扫描模板不存在: %s", templateId)
	}
	if tmpl.Config == "" {
		return "", fmt.Errorf("扫描模板配置为空: %s", templateId)
	}
	return tmpl.Config, nil
}

// resolveAssetTargets 当 targetMode=asset 时从 AssetTargetMetaModel 获取资产列表拼接到 target
func resolveAssetTargets(ctx context.Context, svcCtx *svc.ServiceContext, assetIds []string) (string, error) {
	if len(assetIds) == 0 {
		return "", nil
	}
	metaModel := svcCtx.GetAssetTargetMetaModel()
	metas, err := metaModel.FindByIDs(ctx, assetIds)
	if err != nil {
		return "", fmt.Errorf("查询资产失败: %v", err)
	}
	var values []string
	for _, m := range metas {
		if m.TargetValue != "" {
			values = append(values, m.TargetValue)
		}
	}
	return strings.Join(values, "\n"), nil
}

// CronTaskListHandler 定时任务列表（从MongoDB读取）
func CronTaskListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CronTaskListReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		ctx := r.Context()

		// 从MongoDB读取定时任务（关键字和任务类型过滤在MongoDB层完成）
		tasks, total, err := svcCtx.CronTaskModel.FindTasks(ctx, req.Keyword, req.TaskType, req.Page, req.PageSize)
		if err != nil {
			response.Error(w, fmt.Errorf("获取定时任务失败: %v", err))
			return
		}

		var list []*CronTaskItem
		for _, task := range tasks {
			// 从Redis获取运行次数（仅运行次数保留在Redis，属于临时计数器）
			runCountKey := fmt.Sprintf("cscan:cron:runcount:%s", task.CronTaskId)
			runCount, _ := svcCtx.RedisClient.Get(ctx, runCountKey).Int64()

			item := &CronTaskItem{
				Id:                  task.CronTaskId,
				Name:                task.Name,
				TaskType:            task.TaskType,
				ScheduleType:        task.ScheduleType,
				CronSpec:            task.CronSpec,
				ScheduleTime:        task.ScheduleTime,
				Status:              task.Status,
				LastRunTime:         task.LastRunTime,
				NextRunTime:         task.NextRunTime,
				RunCount:            runCount,
				TargetMode:          task.TargetMode,
				Target:              task.Target,
				AssetIds:            task.AssetIds,
				OrgId:               task.OrgId,
				EnableSubdomainPull: task.EnableSubdomainPull,
				ConfigSource:        task.ConfigSource,
				TemplateId:          task.TemplateId,
				Config:              task.Config,
				Platform:            task.Platform,
				Query:               task.Query,
				MaxResults:          task.MaxResults,
			}

			// 截取目标显示（用于列表，仅 scan 类型）
			if task.TaskType == string(model.CronTaskTypeScan) {
				targetShort := task.Target
				if len(targetShort) > 100 {
					targetShort = targetShort[:100] + "..."
				}
				item.TargetShort = targetShort
			}

			list = append(list, item)
		}

		if list == nil {
			list = []*CronTaskItem{}
		}

		httpx.OkJson(w, &CronTaskListResp{
			Code: 0,
			Msg:  "success",
			Data: &CronTaskListRespData{
				List:  list,
				Total: int(total),
			},
		})
	}
}

// CronTaskDetailReq 获取定时任务详情请求
type CronTaskDetailReq struct {
	Id string `json:"id"`
}

// CronTaskDetailHandler 获取定时任务详情
func CronTaskDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CronTaskDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		if req.Id == "" {
			response.ParamError(w, "任务ID不能为空")
			return
		}

		ctx := r.Context()
		cronTask, err := svcCtx.CronTaskModel.FindByCronTaskId(ctx, req.Id)
		if err != nil {
			response.Error(w, fmt.Errorf("查询定时任务失败: %v", err))
			return
		}
		if cronTask == nil {
			response.ParamError(w, "定时任务不存在")
			return
		}

		// 从Redis获取运行次数
		runCountKey := fmt.Sprintf("cscan:cron:runcount:%s", cronTask.CronTaskId)
		runCount, _ := svcCtx.RedisClient.Get(ctx, runCountKey).Int64()

		item := &CronTaskItem{
			Id:                  cronTask.CronTaskId,
			Name:                cronTask.Name,
			TaskType:            cronTask.TaskType,
			ScheduleType:        cronTask.ScheduleType,
			CronSpec:            cronTask.CronSpec,
			ScheduleTime:        cronTask.ScheduleTime,
			Status:              cronTask.Status,
			LastRunTime:         cronTask.LastRunTime,
			NextRunTime:         cronTask.NextRunTime,
			RunCount:            runCount,
			TargetMode:          cronTask.TargetMode,
			Target:              cronTask.Target,
			AssetIds:            cronTask.AssetIds,
			OrgId:               cronTask.OrgId,
			EnableSubdomainPull: cronTask.EnableSubdomainPull,
			ConfigSource:        cronTask.ConfigSource,
			TemplateId:          cronTask.TemplateId,
			Config:              cronTask.Config,
			Platform:            cronTask.Platform,
			Query:               cronTask.Query,
			MaxResults:          cronTask.MaxResults,
		}

		httpx.OkJson(w, map[string]any{
			"code": 0,
			"msg":  "success",
			"data": item,
		})
	}
}

// CronTaskSaveHandler 保存定时任务（MongoDB主存储 + Redis调度缓存同步）
func CronTaskSaveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CronTaskSaveReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		if req.Name == "" {
			response.ParamError(w, "任务名称不能为空")
			return
		}
		if req.TaskType == "" {
			req.TaskType = string(model.CronTaskTypeScan)
		}
		if req.TaskType != string(model.CronTaskTypeScan) && req.TaskType != string(model.CronTaskTypeSpaceEngine) {
			response.ParamError(w, "无效的任务类型")
			return
		}
		if req.ScheduleType == "" {
			req.ScheduleType = "cron"
		}

		var nextRunTime string

		// 验证调度配置
		if req.ScheduleType == "cron" {
			if req.CronSpec == "" {
				response.ParamError(w, "Cron表达式不能为空")
				return
			}
			schedule, err := cronParser.Parse(req.CronSpec)
			if err != nil {
				response.ParamError(w, fmt.Sprintf("无效的Cron表达式: %v", err))
				return
			}
			nextRunTime = schedule.Next(time.Now()).Local().Format("2006-01-02 15:04:05")
		} else if req.ScheduleType == "once" {
			if req.ScheduleTime == "" {
				response.ParamError(w, "请选择执行时间")
				return
			}
			t, err := time.ParseInLocation("2006-01-02 15:04:05", req.ScheduleTime, time.Local)
			if err != nil {
				response.ParamError(w, "时间格式无效，请使用 YYYY-MM-DD HH:mm:ss 格式")
				return
			}
			if t.Before(time.Now()) {
				response.ParamError(w, "执行时间不能早于当前时间")
				return
			}
			nextRunTime = req.ScheduleTime
		} else {
			response.ParamError(w, "无效的调度类型")
			return
		}

		ctx := r.Context()

		// 构造待持久化的 CronTask 文档（先填充通用字段，类型字段在 switch 内填充）
		cronTaskId := req.Id
		isNew := cronTaskId == ""
		if isNew {
			cronTaskId = uuid.New().String()
		}

		cronTask := &model.CronTask{
			CronTaskId:   cronTaskId,
			Name:         req.Name,
			TaskType:     req.TaskType,
			ScheduleType: req.ScheduleType,
			CronSpec:     req.CronSpec,
			ScheduleTime: req.ScheduleTime,
			Status:       "enable",
			NextRunTime:  nextRunTime,
		}

		// 根据 taskType 做字段校验与补全
		switch model.CronTaskType(req.TaskType) {
		case model.CronTaskTypeScan:
			// 目标模式校验
			targetMode := req.TargetMode
			if targetMode == "" {
				targetMode = string(model.CronTargetModeManual)
			}
			if targetMode != string(model.CronTargetModeManual) && targetMode != string(model.CronTargetModeAsset) {
				response.ParamError(w, "无效的目标选择模式")
				return
			}
			cronTask.TargetMode = targetMode

			var resolvedTarget string
			switch model.CronTaskTargetMode(targetMode) {
			case model.CronTargetModeManual:
				if strings.TrimSpace(req.Target) == "" {
					response.ParamError(w, "扫描目标不能为空")
					return
				}
				resolvedTarget = req.Target
			case model.CronTargetModeAsset:
				if len(req.AssetIds) == 0 {
					response.ParamError(w, "请选择资产")
					return
				}
				assetTarget, err := resolveAssetTargets(ctx, svcCtx, req.AssetIds)
				if err != nil {
					response.Error(w, err)
					return
				}
				if strings.TrimSpace(assetTarget) == "" {
					response.ParamError(w, "所选资产未解析到有效目标")
					return
				}
				resolvedTarget = assetTarget
				// 手动输入的目标作为追加（可选）
				if strings.TrimSpace(req.Target) != "" {
					resolvedTarget = strings.TrimSpace(resolvedTarget) + "\n" + strings.TrimSpace(req.Target)
				}
				cronTask.AssetIds = req.AssetIds
			}
			cronTask.Target = resolvedTarget
			cronTask.OrgId = req.OrgId
			cronTask.EnableSubdomainPull = req.EnableSubdomainPull

			// 配置来源校验
			configSource := req.ConfigSource
			if configSource == "" {
				configSource = string(model.CronConfigSourceCustom)
			}
			if configSource != string(model.CronConfigSourceTemplate) && configSource != string(model.CronConfigSourceCustom) {
				response.ParamError(w, "无效的配置来源")
				return
			}
			cronTask.ConfigSource = configSource

			var resolvedConfig string
			switch model.CronTaskConfigSource(configSource) {
			case model.CronConfigSourceTemplate:
				if req.TemplateId == "" {
					response.ParamError(w, "请选择扫描模板")
					return
				}
				tmplConfig, err := expandTemplateConfig(ctx, svcCtx, req.TemplateId)
				if err != nil {
					response.Error(w, err)
					return
				}
				resolvedConfig = tmplConfig
				cronTask.TemplateId = req.TemplateId
			case model.CronConfigSourceCustom:
				if strings.TrimSpace(req.Config) == "" {
					response.ParamError(w, "扫描配置不能为空")
					return
				}
				resolvedConfig = req.Config
			}
			cronTask.Config = resolvedConfig

		case model.CronTaskTypeSpaceEngine:
			if req.Platform == "" {
				response.ParamError(w, "请选择空间引擎平台")
				return
			}
			if strings.TrimSpace(req.Query) == "" {
				response.ParamError(w, "查询语句不能为空")
				return
			}
			if req.MaxResults <= 0 {
				req.MaxResults = 100
			}
			cronTask.Platform = req.Platform
			cronTask.Query = req.Query
			cronTask.MaxResults = req.MaxResults
		}

		if isNew {
			// 新建 - 写入MongoDB
			if err := svcCtx.CronTaskModel.Insert(ctx, cronTask); err != nil {
				response.Error(w, fmt.Errorf("保存定时任务失败: %v", err))
				return
			}
			// 同步到Redis调度缓存
			syncCronTaskToRedis(ctx, svcCtx, cronTask)
			// 通知调度器重新加载
			svcCtx.RedisClient.Publish(ctx, "cscan:cron:reload", cronTaskId)
		} else {
			// 更新 - 从MongoDB获取并更新
			existingTask, err := svcCtx.CronTaskModel.FindByCronTaskId(ctx, req.Id)
			if err != nil || existingTask == nil {
				response.ParamError(w, "定时任务不存在")
				return
			}

			update := bson.M{
				"name":           req.Name,
				"task_type":      req.TaskType,
				"schedule_type":  req.ScheduleType,
				"cron_spec":      req.CronSpec,
				"schedule_time":  req.ScheduleTime,
				"next_run_time":  nextRunTime,
				"status":         "enable",
				// scan 字段
				"target_mode":            cronTask.TargetMode,
				"target":                 cronTask.Target,
				"asset_ids":              cronTask.AssetIds,
				"org_id":                 cronTask.OrgId,
				"enable_subdomain_pull":  cronTask.EnableSubdomainPull,
				"config_source":          cronTask.ConfigSource,
				"template_id":            cronTask.TemplateId,
				"config":                 cronTask.Config,
				// space_engine 字段
				"platform":    cronTask.Platform,
				"query":       cronTask.Query,
				"max_results": cronTask.MaxResults,
			}
			// 清空无关字段（避免类型切换后脏数据残留）
			if model.CronTaskType(req.TaskType) == model.CronTaskTypeScan {
				update["platform"] = ""
				update["query"] = ""
				update["max_results"] = 0
			} else {
				update["target_mode"] = ""
				update["target"] = ""
				update["asset_ids"] = nil
				update["org_id"] = ""
				update["enable_subdomain_pull"] = false
				update["config_source"] = ""
				update["template_id"] = ""
				update["config"] = ""
			}
			if err := svcCtx.CronTaskModel.UpdateByCronTaskId(ctx, req.Id, update); err != nil {
				response.Error(w, fmt.Errorf("更新定时任务失败: %v", err))
				return
			}

			// 读取更新后的完整数据同步到Redis
			updatedTask, _ := svcCtx.CronTaskModel.FindByCronTaskId(ctx, req.Id)
			if updatedTask != nil {
				syncCronTaskToRedis(ctx, svcCtx, updatedTask)
			}

			// 通知调度器重新加载（无论启用/禁用状态，确保内存与MongoDB一致）
			svcCtx.RedisClient.Publish(ctx, "cscan:cron:reload", req.Id)
		}

		// 模板使用次数 +1（异步无阻塞）
		if cronTask.ConfigSource == string(model.CronConfigSourceTemplate) && cronTask.TemplateId != "" {
			go func(tid string) {
				if err := svcCtx.ScanTemplateModel.IncrUseCount(context.Background(), tid); err != nil {
					logx.Errorf("[CronTaskSave] failed to incr template use count: templateId=%s, err=%v", tid, err)
				}
			}(cronTask.TemplateId)
		}

		response.SuccessWithMsg(w, "保存成功")
	}
}

// CronTaskToggleHandler 开关定时任务
func CronTaskToggleHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CronTaskToggleReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		if req.Id == "" {
			response.ParamError(w, "任务ID不能为空")
			return
		}
		if req.Status != "enable" && req.Status != "disable" {
			response.ParamError(w, "状态值无效")
			return
		}

		ctx := r.Context()

		// 从MongoDB获取现有任务
		task, err := svcCtx.CronTaskModel.FindByCronTaskId(ctx, req.Id)
		if err != nil || task == nil {
			response.ParamError(w, "任务不存在")
			return
		}

		update := bson.M{"status": req.Status}

		// 如果启用，更新下次运行时间
		if req.Status == "enable" {
			if task.ScheduleType == "cron" {
				if task.CronSpec == "" {
					response.ParamError(w, "Cron表达式为空，请先编辑任务设置Cron表达式")
					return
				}
				schedule, parseErr := cronParser.Parse(task.CronSpec)
				if parseErr != nil {
					response.ParamError(w, fmt.Sprintf("Cron表达式无效，请先编辑任务修正: %v", parseErr))
					return
				}
				nextRun := schedule.Next(time.Now()).Local().Format("2006-01-02 15:04:05")
				update["next_run_time"] = nextRun
			} else if task.ScheduleType == "once" {
				if task.ScheduleTime == "" {
					response.ParamError(w, "执行时间未设置，请先编辑任务设置执行时间")
					return
				}
				t, parseErr := time.ParseInLocation("2006-01-02 15:04:05", task.ScheduleTime, time.Local)
				if parseErr != nil {
					response.ParamError(w, "执行时间格式无效，请先编辑任务重新设置")
					return
				}
				if t.Before(time.Now()) {
					response.ParamError(w, "指定的执行时间已过，请先编辑任务修改执行时间")
					return
				}
				update["next_run_time"] = task.ScheduleTime
			}
		}

		// 更新MongoDB
		if err := svcCtx.CronTaskModel.UpdateByCronTaskId(ctx, req.Id, update); err != nil {
			response.Error(w, fmt.Errorf("更新定时任务失败: %v", err))
			return
		}

		// 读取更新后的完整数据同步到Redis
		updatedTask, _ := svcCtx.CronTaskModel.FindByCronTaskId(ctx, req.Id)
		if updatedTask != nil {
			syncCronTaskToRedis(ctx, svcCtx, updatedTask)
		}

		// 通知调度器
		svcCtx.RedisClient.Publish(ctx, "cscan:cron:reload", req.Id)

		msg := "已启用"
		if req.Status == "disable" {
			msg = "已禁用"
		}
		response.SuccessWithMsg(w, msg)
	}
}

// CronTaskDeleteHandler 删除定时任务
func CronTaskDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CronTaskDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		if req.Id == "" {
			response.ParamError(w, "任务ID不能为空")
			return
		}

		ctx := r.Context()

		// 从MongoDB删除
		if err := svcCtx.CronTaskModel.DeleteByCronTaskId(ctx, req.Id); err != nil {
			response.Error(w, fmt.Errorf("删除定时任务失败: %v", err))
			return
		}

		// 从Redis删除调度缓存
		removeCronTaskFromRedis(ctx, svcCtx, req.Id)

		// 通知调度器移除任务
		svcCtx.RedisClient.Publish(ctx, "cscan:cron:remove", req.Id)

		response.SuccessWithMsg(w, "删除成功")
	}
}

// CronTaskBatchDeleteHandler 批量删除定时任务
func CronTaskBatchDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CronTaskBatchDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		if len(req.Ids) == 0 {
			response.ParamError(w, "请选择要删除的任务")
			return
		}

		ctx := r.Context()

		// 从MongoDB批量删除
		deletedCount, err := svcCtx.CronTaskModel.BatchDeleteByCronTaskIds(ctx, req.Ids)
		if err != nil {
			response.Error(w, fmt.Errorf("批量删除定时任务失败: %v", err))
			return
		}

		// 从Redis删除调度缓存并通知调度器
		for _, id := range req.Ids {
			removeCronTaskFromRedis(ctx, svcCtx, id)
			svcCtx.RedisClient.Publish(ctx, "cscan:cron:remove", id)
		}

		response.SuccessWithMsg(w, fmt.Sprintf("成功删除 %d 个定时任务", deletedCount))
	}
}

// CronTaskRunNowHandler 立即执行定时任务
func CronTaskRunNowHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CronTaskRunNowReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		if req.Id == "" {
			response.ParamError(w, "任务ID不能为空")
			return
		}

		ctx := r.Context()

		// 先确保Redis缓存是最新的（从MongoDB同步）
		cronTask, err := svcCtx.CronTaskModel.FindByCronTaskId(ctx, req.Id)
		if err != nil || cronTask == nil {
			response.Error(w, fmt.Errorf("定时任务不存在"))
			return
		}
		syncCronTaskToRedis(ctx, svcCtx, cronTask)

		// 通知调度器立即执行
		svcCtx.RedisClient.Publish(ctx, "cscan:cron:runnow", req.Id)

		response.SuccessWithMsg(w, "已触发执行")
	}
}

// ValidateCronSpecHandler 验证Cron表达式
func ValidateCronSpecHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			CronSpec string `json:"cronSpec"`
		}
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		schedule, err := cronParser.Parse(req.CronSpec)
		if err != nil {
			httpx.OkJson(w, map[string]any{
				"code": 1,
				"msg":  fmt.Sprintf("无效的Cron表达式: %v", err),
				"data": nil,
			})
			return
		}

		var nextTimes []string
		t := time.Now()
		for i := 0; i < 5; i++ {
			t = schedule.Next(t)
			nextTimes = append(nextTimes, t.Local().Format("2006-01-02 15:04:05"))
		}

		httpx.OkJson(w, map[string]any{
			"code": 0,
			"msg":  "success",
			"data": map[string]any{
				"valid":     true,
				"nextTimes": nextTimes,
			},
		})
	}
}
