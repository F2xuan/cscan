package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// Worker 管理 分组：Worker 进程注册、心跳、配置拉取、任务结果回传、控制台文件 / 终端、审计日志、安装命令。
//
// 说明：Worker 相关 handler 多为 handler-local inline JSON 解码（不在 types 包内声明请求结构体），
// 因此本文件中 ReqType 留空，spec.go 会在 POST 端点注入开放 JSON object body 以便 Swagger UI Try it out 编辑。
func init() {
	tag := "Worker 管理"
	tagDesc := "分布式 Worker 进程注册、心跳、配置拉取、任务结果回传与控制台"

	// ===== 公开（Install 前准备） =====
	register(http.MethodGet, "/api/v1/worker/download", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "下载 Worker 安装包",
		Description: "公开接口，返回 Worker 二进制压缩包。",
		Security:    TierPublic,
	})
	register(http.MethodPost, "/api/v1/worker/validate", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "校验 Install Key",
		Description: "校验 Install Key 有效性。",
		ReqType:     "",
		Security:    TierPublic,
		Errors:      []int{400, 500},
	})
	register(http.MethodGet, "/api/v1/worker/ws", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "Worker WebSocket 长连接",
		Description: "Worker 通过 WebSocket 与 API 保持长连接。鉴权在握手后进行。",
		Security:    TierPublic,
	})

	// ===== Worker Key 认证（Worker 进程内调用） =====
 register(http.MethodPost, "/api/v1/worker/task/check", Meta{Tag: tag, TagDesc: tagDesc, Summary: "拉取任务", Description: "Worker 长轮询拉取待执行任务。Body：workerId / concurrency 等。", Security: TierWorker, Errors: []int{400, 500}})
 register(http.MethodPost, "/api/v1/worker/task/update", Meta{Tag: tag, TagDesc: tagDesc, Summary: "更新任务状态", Description: "Worker 上报任务状态变化。", Security: TierWorker, Errors: []int{400, 500}})
 register(http.MethodPost, "/api/v1/worker/task/result", Meta{Tag: tag, TagDesc: tagDesc, Summary: "上报任务结果", Description: "Worker 上报扫描结果（资产 / 端口）。", Security: TierWorker, Errors: []int{400, 500}})
 register(http.MethodPost, "/api/v1/worker/task/vul", Meta{Tag: tag, TagDesc: tagDesc, Summary: "上报漏洞结果", Description: "Worker 上报漏洞扫描结果。", Security: TierWorker, Errors: []int{400, 500}})
 register(http.MethodPost, "/api/v1/worker/task/dirscan", Meta{Tag: tag, TagDesc: tagDesc, Summary: "上报目录扫描结果", Description: "Worker 上报目录扫描结果。", Security: TierWorker, Errors: []int{400, 500}})
 register(http.MethodPost, "/api/v1/worker/task/subtask/done", Meta{Tag: tag, TagDesc: tagDesc, Summary: "子任务完成", Description: "Worker 上报单个子任务完成状态。", Security: TierWorker, Errors: []int{400, 500}})
 register(http.MethodPost, "/api/v1/worker/task/control", Meta{Tag: tag, TagDesc: tagDesc, Summary: "任务控制回执", Description: "Worker 应答任务控制指令（暂停 / 恢复 / 停止 / 重试）。", Security: TierWorker, Errors: []int{400, 500}})
 register(http.MethodPost, "/api/v1/worker/task/recovery", Meta{Tag: tag, TagDesc: tagDesc, Summary: "任务恢复回执", Description: "Worker 应答孤儿任务恢复指令。", Security: TierWorker, Errors: []int{400, 500}})
 register(http.MethodPost, "/api/v1/worker/heartbeat", Meta{Tag: tag, TagDesc: tagDesc, Summary: "Worker 心跳", Description: "Worker 定期上报心跳，携带当前并发任务数。", Security: TierWorker, Errors: []int{400, 500}})
 register(http.MethodPost, "/api/v1/worker/offline", Meta{Tag: tag, TagDesc: tagDesc, Summary: "Worker 离线通知", Description: "Worker 主动下线通知，触发处理中任务的重排队。", Security: TierWorker, Errors: []int{400, 500}})

	// ===== Worker 配置拉取 =====
 register(http.MethodPost, "/api/v1/worker/config/templates", Meta{Tag: tag, TagDesc: tagDesc, Summary: "拉取扫描模板", Description: "Worker 拉取扫描配置模板。", Security: TierWorker})
 register(http.MethodPost, "/api/v1/worker/config/fingerprints", Meta{Tag: tag, TagDesc: tagDesc, Summary: "拉取指纹库", Description: "Worker 拉取启用的指纹规则。", Security: TierWorker})
 register(http.MethodPost, "/api/v1/worker/config/subfinder", Meta{Tag: tag, TagDesc: tagDesc, Summary: "拉取 Subfinder 配置", Description: "Worker 拉取 Subfinder 提供者配置。", Security: TierWorker})
 register(http.MethodPost, "/api/v1/worker/config/httpservice", Meta{Tag: tag, TagDesc: tagDesc, Summary: "拉取 HTTP 服务映射", Description: "Worker 拉取 HTTP 服务名映射。", Security: TierWorker})
 register(http.MethodPost, "/api/v1/worker/config/httpservice/settings", Meta{Tag: tag, TagDesc: tagDesc, Summary: "拉取 HTTP 服务设置", Description: "Worker 拉取 HTTP 服务全局设置。", Security: TierWorker})
 register(http.MethodPost, "/api/v1/worker/config/activefingerprints", Meta{Tag: tag, TagDesc: tagDesc, Summary: "拉取主动指纹", Description: "Worker 拉取主动指纹规则。", Security: TierWorker})
 register(http.MethodPost, "/api/v1/worker/config/poc", Meta{Tag: tag, TagDesc: tagDesc, Summary: "拉取 POC 库", Description: "Worker 拉取启用的 POC 模板。", Security: TierWorker})
 register(http.MethodPost, "/api/v1/worker/config/dirscandict", Meta{Tag: tag, TagDesc: tagDesc, Summary: "拉取目录扫描字典", Description: "Worker 拉取启用的目录扫描字典。", Security: TierWorker})
 register(http.MethodPost, "/api/v1/worker/config/subdomaindict", Meta{Tag: tag, TagDesc: tagDesc, Summary: "拉取子域名字典", Description: "Worker 拉取启用的子域名字典。", Security: TierWorker})
 register(http.MethodPost, "/api/v1/worker/config/weakpassdict", Meta{Tag: tag, TagDesc: tagDesc, Summary: "拉取弱口令字典", Description: "Worker 拉取启用的弱口令字典。", Security: TierWorker})
 register(http.MethodPost, "/api/v1/worker/config/blacklist", Meta{Tag: tag, TagDesc: tagDesc, Summary: "拉取黑名单", Description: "Worker 拉取黑名单规则。", Security: TierWorker})
 register(http.MethodPost, "/api/v1/worker/config/jsfinder", Meta{Tag: tag, TagDesc: tagDesc, Summary: "拉取 JSFinder 配置", Description: "Worker 拉取 JSFinder 配置。", Security: TierWorker})
 register(http.MethodPost, "/api/v1/worker/jsfinder/save", Meta{Tag: tag, TagDesc: tagDesc, Summary: "上报 JSFinder 结果", Description: "Worker 上报 JSFinder 提取结果。", Security: TierWorker, Errors: []int{400, 500}})

	// ===== JWT 认证（管理 Worker 资源 + 日志） =====
 register(http.MethodPost, "/api/v1/worker/list", Meta{Tag: tag, TagDesc: tagDesc, Summary: "Worker 列表", Description: "返回当前工作空间下的 Worker 列表。", RespType: "WorkerListResp", Security: TierAuth})
 register(http.MethodPost, "/api/v1/worker/delete", Meta{Tag: tag, TagDesc: tagDesc, Summary: "删除 Worker", Description: "按 id 删除一个 Worker。", ReqType: "WorkerDeleteReq", RespType: "WorkerDeleteResp", Security: TierAuth, Errors: []int{400, 500}})
 register(http.MethodPost, "/api/v1/worker/rename", Meta{Tag: tag, TagDesc: tagDesc, Summary: "重命名 Worker", Description: "重命名 Worker。", ReqType: "WorkerRenameReq", RespType: "WorkerRenameResp", Security: TierAuth, Errors: []int{400, 500}})
 register(http.MethodPost, "/api/v1/worker/restart", Meta{Tag: tag, TagDesc: tagDesc, Summary: "重启 Worker", Description: "触发 Worker 远程重启。", ReqType: "WorkerRestartReq", RespType: "WorkerRestartResp", Security: TierAuth, Errors: []int{400, 500}})
 register(http.MethodPost, "/api/v1/worker/concurrency", Meta{Tag: tag, TagDesc: tagDesc, Summary: "更新并发数", Description: "更新 Worker 最大并发任务数。", ReqType: "WorkerSetConcurrencyReq", RespType: "WorkerSetConcurrencyResp", Security: TierAuth, Errors: []int{400, 500}})
register(http.MethodPost, "/api/v1/worker/logs/history", Meta{Tag: tag, TagDesc: tagDesc, Summary: "Worker 历史日志", Description: "按 workerId / since / until / level 过滤历史日志。", Security: TierAuth, Errors: []int{400, 500}})
 register(http.MethodPost, "/api/v1/worker/logs/export", Meta{Tag: tag, TagDesc: tagDesc, Summary: "导出 Worker 日志", Description: "导出 Worker 历史日志为文本。", Security: TierAuth, Errors: []int{400, 500}})
 register(http.MethodPost, "/api/v1/worker/logs/clear", Meta{Tag: tag, TagDesc: tagDesc, Summary: "清空 Worker 日志", Description: "按 workerId 清空历史日志。", Security: TierAuth, Errors: []int{400, 500}})

	// ===== 管理员（Install Key 管理） =====
 register(http.MethodPost, "/api/v1/worker/install/command", Meta{Tag: tag, TagDesc: tagDesc, Summary: "生成 Worker 安装命令", Description: "生成包含 Install Key 的 Worker 安装命令。", ReqType: "WorkerInstallCommandReq", RespType: "WorkerInstallCommandResp", Security: TierAdmin, Errors: []int{400, 500}})
 register(http.MethodPost, "/api/v1/worker/install/refresh", Meta{Tag: tag, TagDesc: tagDesc, Summary: "刷新 Install Key", Description: "刷新当前 Install Key，老 Key 立即失效。", RespType: "WorkerRefreshKeyResp", Security: TierAdmin})

	// ===== Console / 文件 / 终端 / 审计 =====
 register(http.MethodGet, "/api/v1/worker/console/info", Meta{Tag: tag, TagDesc: tagDesc, Summary: "Worker 控制台信息", Description: "返回 Worker 概览信息（OS、架构、并发数等）。Query：workerId。", Security: TierConsole})
 register(http.MethodGet, "/api/v1/worker/console/files", Meta{Tag: tag, TagDesc: tagDesc, Summary: "Worker 文件列表", Description: "返回 Worker 文件浏览列表。Query：workerId / path。", Security: TierConsole})
 register(http.MethodPost, "/api/v1/worker/console/files/upload", Meta{Tag: tag, TagDesc: tagDesc, Summary: "上传文件到 Worker", Description: "multipart/form-data 上传文件到 Worker 指定路径。", Security: TierConsole, Errors: []int{400, 500}})
 register(http.MethodGet, "/api/v1/worker/console/files/download", Meta{Tag: tag, TagDesc: tagDesc, Summary: "下载 Worker 文件", Description: "下载 Worker 上的指定文件。Query：workerId / path。", Security: TierConsole})
 register(http.MethodDelete, "/api/v1/worker/console/files", Meta{Tag: tag, TagDesc: tagDesc, Summary: "删除 Worker 文件", Description: "按 workerId / path 删除 Worker 上的文件或目录。", Security: TierConsole, Errors: []int{400, 500}})
 register(http.MethodPost, "/api/v1/worker/console/files/mkdir", Meta{Tag: tag, TagDesc: tagDesc, Summary: "在 Worker 上创建目录", Description: "在 Worker 指定路径下创建目录。", Security: TierConsole, Errors: []int{400, 500}})
 register(http.MethodPost, "/api/v1/worker/console/terminal/open", Meta{Tag: tag, TagDesc: tagDesc, Summary: "打开终端会话", Description: "在 Worker 上打开一个交互式终端会话。", Security: TierTerminal, Errors: []int{400, 500}})
 register(http.MethodPost, "/api/v1/worker/console/terminal/close", Meta{Tag: tag, TagDesc: tagDesc, Summary: "关闭终端会话", Description: "关闭指定 terminalId 的终端会话。", Security: TierTerminal, Errors: []int{400, 500}})
 register(http.MethodPost, "/api/v1/worker/console/terminal/exec", Meta{Tag: tag, TagDesc: tagDesc, Summary: "执行终端命令", Description: "在已有终端会话中执行命令。", Security: TierTerminal, Errors: []int{400, 500}})
 register(http.MethodGet, "/api/v1/worker/console/terminal/history", Meta{Tag: tag, TagDesc: tagDesc, Summary: "终端命令历史", Description: "Query：workerId / sessionId。", Security: TierTerminal})
 register(http.MethodGet, "/api/v1/worker/console/audit", Meta{Tag: tag, TagDesc: tagDesc, Summary: "审计日志列表", Description: "Query：workerId / since / until / action / user。", Security: TierConsole})
 register(http.MethodDelete, "/api/v1/worker/console/audit", Meta{Tag: tag, TagDesc: tagDesc, Summary: "清空审计日志", Description: "Query：workerId / before。", Security: TierConsole, Errors: []int{400, 500}})
 register(http.MethodGet, "/api/v1/worker/console/terminal", Meta{Tag: tag, TagDesc: tagDesc, Summary: "Worker 终端 WebSocket", Description: "升级为 WebSocket，转发终端 I/O。鉴权在握手完成后进行。", Security: TierTerminal})

	RegisterTypes(
		types.WorkerListResp{},
		types.WorkerDeleteReq{},
		types.WorkerDeleteResp{},
		types.WorkerRenameReq{},
		types.WorkerRenameResp{},
		types.WorkerRestartReq{},
		types.WorkerRestartResp{},
		types.WorkerSetConcurrencyReq{},
		types.WorkerSetConcurrencyResp{},
		types.WorkerInstallCommandReq{},
		types.WorkerInstallCommandResp{},
		types.WorkerRefreshKeyResp{},
	)
}
