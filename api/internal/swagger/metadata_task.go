package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// 任务管理分组：主任务 CRUD、控制（开始/暂停/恢复/停止/重试）、统计、任务日志（含流式）、
// 任务配置（profile）、扫描配置模板（template）、任务分片（chunk）、定时任务（cron）。
//
// 全部接口为 JWT 鉴权（TierAuth），任务的实际执行由 Worker 拉取 Redis 队列后异步完成。
func init() {
	tag := "任务管理"
	tagDesc := "扫描任务的创建、控制、日志、配置模板、分片与定时调度"

	// ===== 主任务 CRUD =====
	register(http.MethodPost, "/api/v1/task/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "主任务列表",
		Description: "按工作空间分页返回主任务列表，支持按名称、状态、标签过滤。\n\n`workspaceId` 可在请求体中显式传递（优先级高于 `X-Workspace-Id` 请求头）。",
		ReqType:     "MainTaskListReq",
		RespType:    "MainTaskListResp",
		Security:    TierAuth,
		Errors:      []int{500},
	})

	register(http.MethodPost, "/api/v1/task/create", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "创建主任务",
		Description: "创建主任务并按目标数量自动分批为子任务（batchSize=50），推入 Redis Sorted Set 队列等待 Worker 拉取。\n\n**关键参数**\n\n- `target`：扫描目标（必填）。\n- `profileId` 或 `templateId`：二选一，引用扫描配置；二者皆空时使用 `config` 直接传 JSON。\n- `workers`：可选，指定由哪些 Worker 执行；为空则任意在线 Worker 可拉取。\n- `workspaceId`：可选，跨工作空间创建时使用。\n\n**典型错误码**\n\n- 10102 任务配置不存在\n- 500 服务器错误",
		ReqType:     "MainTaskCreateReq",
		Security:    TierAuth,
		Errors:      []int{10102, 500},
	})

	register(http.MethodPost, "/api/v1/task/update", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "更新主任务",
		Description: "更新主任务的可修改字段（名称、目标、配置 ID）。修改 `target` 不会自动重新分批，需要在合适时机调用 `retry` 或新建任务。",
		ReqType:     "MainTaskUpdateReq",
		Security:    TierAuth,
		Errors:      []int{10101, 10103, 500},
	})

	register(http.MethodPost, "/api/v1/task/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除主任务",
		Description: "按 `id` 删除主任务及其全部子任务与日志。若任务正在执行，会先尝试停止再删除。",
		ReqType:     "MainTaskDeleteReq",
		Security:    TierAuth,
		Errors:      []int{10101, 500},
	})

	register(http.MethodPost, "/api/v1/task/batchDelete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "批量删除主任务",
		Description: "按 `ids` 数组批量删除主任务。空数组返回成功不执行删除。",
		ReqType:     "MainTaskBatchDeleteReq",
		Security:    TierAuth,
		Errors:      []int{10101, 500},
	})

	register(http.MethodPost, "/api/v1/task/retry", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "重试主任务",
		Description: "按 `id` 重新入队失败或被停止的子任务。已完成子任务不会被重跑。",
		ReqType:     "MainTaskRetryReq",
		Security:    TierAuth,
		Errors:      []int{10101, 10103, 500},
	})

	// ===== 任务控制 =====
	register(http.MethodPost, "/api/v1/task/start", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "启动主任务",
		Description: "启动一个处于 PENDING 的主任务，触发子任务入队。",
		ReqType:     "MainTaskControlReq",
		Security:    TierAuth,
		Errors:      []int{10101, 10103, 500},
	})

	register(http.MethodPost, "/api/v1/task/pause", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "暂停主任务",
		Description: "暂停正在执行的主任务：未开始的子任务不再入队，已下发的子任务执行完当前步骤后退出。",
		ReqType:     "MainTaskControlReq",
		Security:    TierAuth,
		Errors:      []int{10101, 10103, 500},
	})

	register(http.MethodPost, "/api/v1/task/resume", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "恢复主任务",
		Description: "从暂停状态恢复主任务：重新激活未完成的部分。",
		ReqType:     "MainTaskControlReq",
		Security:    TierAuth,
		Errors:      []int{10101, 10103, 500},
	})

	register(http.MethodPost, "/api/v1/task/stop", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "停止主任务",
		Description: "强制停止主任务，所有未完成子任务立即置为 FAILURE，已采集结果保留。",
		ReqType:     "MainTaskControlReq",
		Security:    TierAuth,
		Errors:      []int{10101, 10103, 500},
	})

	register(http.MethodPost, "/api/v1/task/stat", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "任务统计",
		Description: "返回任务大盘：总数、完成、运行中、失败、待执行，以及近 7 天每日完成 / 失败趋势。",
		RespType:    "TaskStatResp",
		Security:    TierAuth,
	})

	// ===== 任务配置（profile） =====
	register(http.MethodPost, "/api/v1/task/profile/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "任务配置列表",
		Description: "返回当前工作空间下的任务配置（profile）列表，包含 id / name / description / config JSON。",
		RespType:    "TaskProfileListResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/task/profile/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存任务配置",
		Description: "保存任务配置（profile）。`id` 为空表示新建，非空表示更新对应 ID 的配置。\n\n`config` 为扫描配置 JSON 字符串。",
		ReqType:     "TaskProfileSaveReq",
		Security:    TierAuth,
		Errors:      []int{10102, 500},
	})

	register(http.MethodPost, "/api/v1/task/profile/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除任务配置",
		Description: "按 `id` 删除单个任务配置。",
		ReqType:     "TaskProfileDeleteReq",
		Security:    TierAuth,
		Errors:      []int{10102, 500},
	})

	// ===== 任务日志 =====
	register(http.MethodPost, "/api/v1/task/logs", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "获取任务日志",
		Description: "分页 / 关键词查询某 `taskId` 的日志，最近 `limit` 条优先返回。`search` 为模糊关键词。",
		ReqType:     "GetTaskLogsReq",
		RespType:    "GetTaskLogsResp",
		Security:    TierAuth,
		Errors:      []int{10101, 500},
	})

	register(http.MethodGet, "/api/v1/task/logs/stream", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "实时任务日志流（SSE）",
		Description: "以 Server-Sent Events 推送某 `taskId` 的实时日志。前端用 `EventSource` 连接，禁止设置 `Authorization` 头，token 通过 query 参数 `token=` 传递。",
		Security:    TierAuth,
	})

	// ===== 任务分片 =====
	register(http.MethodPost, "/api/v1/task/chunk/progress", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "查询分片进度",
		Description: "返回 `taskId` 的分片执行进度：总数、完成数、失败数、运行中数、完成率与各分片详情。",
		ReqType:     "ChunkProgressReq",
		RespType:    "ChunkProgressResp",
		Security:    TierAuth,
		Errors:      []int{10101, 500},
	})

	register(http.MethodPost, "/api/v1/task/chunk/preview", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "分片预览",
		Description: "对扫描目标预演分片策略，预估总目标数、预计分片数、平均分片大小、是否需要拆分、执行时长、推荐分片大小、内存峰值与并行能力，供前端在创建任务时确认分片方案。",
		ReqType:     "ChunkPreviewReq",
		RespType:    "ChunkPreviewResp",
		Security:    TierAuth,
	})

	// ===== 扫描配置模板 =====
	register(http.MethodPost, "/api/v1/task/template/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "模板列表",
		Description: "分页返回扫描配置模板，可按 `keyword`、`category`、`tags` 过滤。",
		ReqType:     "ScanTemplateListReq",
		RespType:    "ScanTemplateListResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/task/template/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存模板",
		Description: "保存一个扫描配置模板。`id` 为空表示新建，非空更新对应模板。\n\n内置模板（`isBuiltin=true`）不可保存修改。",
		ReqType:     "ScanTemplateSaveReq",
		Security:    TierAuth,
		Errors:      []int{500},
	})

	register(http.MethodPost, "/api/v1/task/template/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除模板",
		Description: "按 `id` 删除一个用户自定义模板。内置模板不允许删除。",
		ReqType:     "ScanTemplateDeleteReq",
		Security:    TierAuth,
		Errors:      []int{500},
	})

	register(http.MethodPost, "/api/v1/task/template/detail", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "模板详情",
		Description: "按 `id` 返回单个扫描模板的完整配置 JSON。",
		ReqType:     "ScanTemplateDetailReq",
		RespType:    "ScanTemplateDetailResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/task/template/fromTask", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "从任务另存为模板",
		Description: "把已存在任务 `taskId` 的当前配置另存为一个新模板，命名为 `name`。",
		ReqType:     "ScanTemplateFromTaskReq",
		Security:    TierAuth,
		Errors:      []int{10101, 500},
	})

	register(http.MethodPost, "/api/v1/task/template/categories", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "模板分类与标签",
		Description: "返回当前所有模板用到的分类（含内置）与用户标签集合。",
		RespType:    "ScanTemplateCategoriesResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/task/template/export", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "导出模板",
		Description: "把指定 `ids` 数组的模板导出成一个 JSON 字符串打包下载。`ids` 为空时导出全部用户模板。",
		ReqType:     "ScanTemplateExportReq",
		RespType:    "ScanTemplateExportResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/task/template/import", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "导入模板",
		Description: "导入 JSON 字符串形式的模板数据。`skipExisting=true` 时跳过已存在的同名模板。",
		ReqType:     "ScanTemplateImportReq",
		RespType:    "ScanTemplateImportResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	register(http.MethodPost, "/api/v1/task/template/use", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "应用模板",
		Description: "把指定模板 `id` 的配置应用到某个任务，需要前端在创建 / 更新任务时再走 `templateId` 参数。",
		ReqType:     "ScanTemplateUseReq",
		Security:    TierAuth,
		Errors:      []int{500},
	})

	// ===== 定时任务 =====
	// 注：Cron 任务的请求/响应类型定义在 api/internal/handler/task/crontaskhandler.go 包内，
	// 不属于 types 包，因此不在 OpenAPI components.schemas 中登记具名结构体。
	register(http.MethodPost, "/api/v1/task/cron/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "定时任务列表",
		Description: "分页返回当前工作空间下的定时任务，可按 `keyword` 模糊匹配任务名 / 目标。响应 `data.list` 含每个定时任务的下一次触发时间、最近执行时间与累计执行次数。",
		Security:    TierAuth,
		Errors:      []int{500},
	})

	register(http.MethodPost, "/api/v1/task/cron/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存定时任务",
		Description: "新建或更新定时任务。\n\n**字段约束**\n\n- `scheduleType`: `cron`（Cron 表达式）或 `once`（指定时间）。\n- `cronSpec`: 6 段秒级 cron（秒 分 时 日 月 周）。\n- `scheduleTime`: `2006-01-02 15:04:05` 字符串。\n- `mainTaskId`: 关联主任务 ID，用于读取初始配置。\n- `target` / `config`: 可选，覆盖关联任务的默认值。",
		Security:    TierAuth,
		Errors:      []int{10101, 400, 500},
	})

	register(http.MethodPost, "/api/v1/task/cron/toggle", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "启停定时任务",
		Description: "把 `id` 指定的定时任务切换为 `enable` 或 `disable`。停止后立即从 Redis 调度器移除，下次触发时不会执行。",
		Security:    TierAuth,
		Errors:      []int{500},
	})

	register(http.MethodPost, "/api/v1/task/cron/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除定时任务",
		Description: "按 `id` 删除单个定时任务，并同步从 Redis 调度器移除。",
		Security:    TierAuth,
		Errors:      []int{500},
	})

	register(http.MethodPost, "/api/v1/task/cron/batchDelete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "批量删除定时任务",
		Description: "按 `ids` 批量删除定时任务。",
		Security:    TierAuth,
		Errors:      []int{500},
	})

	register(http.MethodPost, "/api/v1/task/cron/runNow", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "立即执行定时任务",
		Description: "把 `id` 指定的定时任务立刻推入 Redis `cscan:cron:execute` 频道触发一次，不影响原定计划。",
		Security:    TierAuth,
		Errors:      []int{500},
	})

	register(http.MethodPost, "/api/v1/task/cron/validate", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "校验 Cron 表达式",
		Description: "校验用户输入的 cron 表达式是否合法（6 段秒级格式），并返回下一次触发时间。",
		Security:    TierAuth,
		Errors:      []int{400},
	})

	// 反射器入口
	RegisterTypes(
		types.MainTask{},
		types.MainTaskListReq{},
		types.MainTaskListResp{},
		types.MainTaskCreateReq{},
		types.MainTaskUpdateReq{},
		types.MainTaskDeleteReq{},
		types.MainTaskBatchDeleteReq{},
		types.MainTaskRetryReq{},
		types.MainTaskControlReq{},
		types.TaskProfile{},
		types.TaskProfileListResp{},
		types.TaskProfileSaveReq{},
		types.TaskProfileDeleteReq{},
		types.GetTaskLogsReq{},
		types.TaskLogEntry{},
		types.GetTaskLogsResp{},
		types.TaskStatResp{},
		types.ChunkProgressReq{},
		types.ChunkStatus{},
		types.ChunkProgressResp{},
		types.ChunkPreviewReq{},
		types.ChunkPreviewResp{},
		types.ScanTemplate{},
		types.ScanTemplateListReq{},
		types.ScanTemplateListResp{},
		types.ScanTemplateSaveReq{},
		types.ScanTemplateDeleteReq{},
		types.ScanTemplateDetailReq{},
		types.ScanTemplateDetailResp{},
		types.ScanTemplateFromTaskReq{},
		types.ScanTemplateCategoriesResp{},
		types.ScanTemplateExportReq{},
		types.ScanTemplateExportResp{},
		types.ScanTemplateImportReq{},
		types.ScanTemplateImportResp{},
		types.ScanTemplateUseReq{},
	)
}
