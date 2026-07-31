package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/model"
	"cscan/scanner"
	"cscan/scheduler"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

// T4.1 一键扫描 + 智能模板推荐
//
// 设计要点：
//   - 目标类型识别**复用** scanner.NewTargetParser().ParseMultiple（严禁重写，见 CLAUDE.md 规则）。
//   - 扫描阶段选择基于内置 quick / standard 模板的模块段（参数与既有模板完全一致），
//     按目标主导类型 + 模式智能启用对应阶段，等价于"智能模板推荐"。
//   - 任务落地复用 common.TaskBuilder.BuildAndPushSubTasks，不另写队列逻辑。
//   - 预估耗时复用 scheduler.NewTaskSplitter(...).GetSplitPreview 的静态估值（秒）。

// quickTemplateConfigJSON 快速扫描基线（仅模块段，参数取自内置 quick-scan.json）
const quickTemplateConfigJSON = `{
  "domainscan": {"enable": false, "subfinder": true, "timeout": 300, "maxEnumerationTime": 10, "threads": 10, "rateLimit": 0, "removeWildcard": true, "resolveDNS": true, "concurrent": 50, "subdomainDictIds": [], "bruteforceTimeout": 30, "recursiveBrute": false, "recursiveDictIds": [], "wildcardDetect": false},
  "portscan": {"enable": true, "tool": "naabu", "rate": 3000, "ports": "top100", "portThreshold": 100, "scanType": "s", "timeout": 60, "skipHostDiscovery": false, "excludeCDN": false, "excludeHosts": "", "workers": 50, "retries": 2, "warmUpTime": 1, "verify": false},
  "portidentify": {"enable": false, "tool": "nmap", "timeout": 60, "concurrency": 10, "args": "-sV -version-intensity 5", "udp": false, "fastMode": false, "forceScan": false},
  "fingerprint": {"enable": true, "tool": "httpx", "iconHash": true, "customEngine": true, "screenshot": false, "activeScan": false, "activeTimeout": 10, "targetTimeout": 30, "filterMode": "http_mapping", "forceScan": false},
  "brutescan": {"enable": false, "services": [], "threads": 20, "timeout": 5, "delayMs": 100, "stopOnFirst": true, "forceScan": false},
  "pocscan": {"enable": false, "mode": "auto", "useNuclei": true, "forceScan": false, "autoScan": true, "automaticScan": true, "customOnly": false, "severity": "critical,high,medium,low,info,unknown", "targetTimeout": 600, "nucleiTemplateIds": [], "customPocIds": [], "customHeaders": [], "customPocOnly": false},
  "dirscan": {"enable": false, "dictIds": [], "threads": 50, "timeout": 10, "followRedirect": true, "forceScan": false, "autoCalibration": true, "filterSize": "", "filterWords": "", "filterLines": "", "filterRegex": "", "matcherMode": "or", "filterMode": "or", "rate": 0, "recursion": false, "recursionDepth": 2},
  "jsfinder": {"enable": false, "threads": 10, "timeout": 10, "forceScan": false}
}`

// standardTemplateConfigJSON 深度扫描基线（仅模块段，参数取自内置 standard-scan.json）
const standardTemplateConfigJSON = `{
  "domainscan": {"enable": false, "subfinder": true, "timeout": 300, "maxEnumerationTime": 10, "threads": 10, "rateLimit": 0, "removeWildcard": true, "resolveDNS": true, "concurrent": 50, "subdomainDictIds": [], "bruteforceTimeout": 30, "recursiveBrute": false, "recursiveDictIds": [], "wildcardDetect": false},
  "portscan": {"enable": true, "tool": "naabu", "rate": 3000, "ports": "top100", "portThreshold": 100, "scanType": "s", "timeout": 60, "skipHostDiscovery": false, "excludeCDN": false, "excludeHosts": "", "workers": 50, "retries": 2, "warmUpTime": 1, "verify": false},
  "portidentify": {"enable": true, "tool": "nmap", "timeout": 60, "concurrency": 10, "args": "-sV -version-intensity 5", "udp": false, "fastMode": false, "forceScan": false},
  "fingerprint": {"enable": true, "tool": "httpx", "iconHash": true, "customEngine": true, "screenshot": true, "activeScan": true, "activeTimeout": 10, "targetTimeout": 90, "filterMode": "http_mapping", "forceScan": false},
  "brutescan": {"enable": true, "services": [], "threads": 20, "timeout": 5, "delayMs": 100, "stopOnFirst": true, "forceScan": false},
  "pocscan": {"enable": true, "mode": "auto", "useNuclei": true, "forceScan": false, "autoScan": true, "automaticScan": true, "customOnly": false, "severity": "critical,high,medium,low,info,unknown", "targetTimeout": 600, "nucleiTemplateIds": [], "customPocIds": [], "customHeaders": [], "customPocOnly": false},
  "dirscan": {"enable": true, "dictIds": ["69fd35207eaed2f49d40abec"], "threads": 50, "timeout": 10, "followRedirect": true, "forceScan": false, "autoCalibration": true, "filterSize": "", "filterWords": "", "filterLines": "", "filterRegex": "", "matcherMode": "or", "filterMode": "or", "rate": 0, "recursion": false, "recursionDepth": 2},
  "jsfinder": {"enable": true, "threads": 10, "timeout": 10, "forceScan": false}
}`

// TaskQuickCreateLogic 一键扫描
type TaskQuickCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTaskQuickCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TaskQuickCreateLogic {
	return &TaskQuickCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// TaskQuickCreate 智能识别目标类型并选扫描阶段创建任务
func (l *TaskQuickCreateLogic) TaskQuickCreate(req *types.TaskQuickCreateReq) (*types.TaskQuickCreateResp, error) {
	wsId := req.WorkspaceId
	if wsId == "" {
		wsId = middleware.GetWorkspaceId(l.ctx)
	}
	// 工作台默认选中"全部工作空间"时 header 为 "all"。一键扫描是写操作，必须把任务落到真实工作空间集合，
	// 否则会插入到 bogus 的 all_maintask 集合，而详情跨工作空间检索只枚举真实工作空间 → 永远查不到 → "任务不存在或已被删除"。
	wsId = common.GetDefaultWorkspaceId(l.ctx, l.svcCtx, wsId)
	if wsId == "" {
		return &types.TaskQuickCreateResp{Code: 400, Msg: "workspaceId不能为空"}, nil
	}
	if strings.TrimSpace(req.Targets) == "" {
		return &types.TaskQuickCreateResp{Code: 400, Msg: "扫描目标不能为空"}, nil
	}
	if errs := common.ValidateTargets(req.Targets); len(errs) > 0 {
		return &types.TaskQuickCreateResp{Code: 400, Msg: common.FormatValidationErrors(errs)}, nil
	}
	mode := req.Mode
	if mode != "full" {
		mode = "quick"
	}

	parser := scanner.NewTargetParser()
	parsed := parser.ParseMultiple(req.Targets)
	if len(parsed) == 0 {
		return &types.TaskQuickCreateResp{Code: 400, Msg: "未能解析出有效目标"}, nil
	}

	taskConfig, recType := recommendConfig(parsed, mode)
	taskConfig["target"] = req.Targets

	if !hasAnyScanPhaseEnabled(taskConfig) {
		return &types.TaskQuickCreateResp{Code: 400, Msg: "推荐配置未启用任何扫描阶段，请改用高级配置"}, nil
	}

	// 预估耗时（复用 scheduler 静态估值，秒→分钟，至少 1 分钟）
	estimatedMinutes := 1
	if splitter := scheduler.NewTaskSplitter(scheduler.DefaultChunkConfig()); splitter != nil {
		if preview, err := splitter.GetSplitPreview(req.Targets, taskConfig); err == nil && preview.EstimatedTime > 0 {
			if em := (preview.EstimatedTime + 59) / 60; em >= 1 {
				estimatedMinutes = em
			}
		}
	}

	profileName := fmt.Sprintf("智能推荐·%s(%s)", recommendedTypeName(recType), modeName(mode))
	taskId := uuid.New().String()
	task := &model.MainTask{
		TaskId:      taskId,
		Name:        fmt.Sprintf("一键扫描-%s", recommendedTypeName(recType)),
		Target:      req.Targets,
		ProfileName: profileName,
		Config:      mustJSON(taskConfig),
		Status:      model.TaskStatusCreated,
	}
	taskModel := l.svcCtx.GetMainTaskModel(wsId)
	if err := taskModel.Insert(l.ctx, task); err != nil {
		l.Logger.Errorf("TaskQuickCreate: insert failed, taskId=%s, error=%v", taskId, err)
		return &types.TaskQuickCreateResp{Code: 500, Msg: "创建任务失败: " + err.Error()}, nil
	}

	// 复用统一任务启动逻辑
	// 修复 M-16：启动失败时必须明确返回错误状态及任务 ID，避免前端误判任务已成功启动
	builder := common.NewTaskBuilder(l.ctx, l.svcCtx)
	if _, err := builder.BuildAndPushSubTasks(wsId, task, taskConfig); err != nil {
		l.Logger.Errorf("TaskQuickCreate: failed to start task %s: %v", taskId, err)
		// 任务已创建但未启动，返回明确的部分成功状态码供前端提示用户
		return &types.TaskQuickCreateResp{
			Code:             501,
			Msg:              fmt.Sprintf("任务已创建但启动失败，请在任务列表手动启动: %v", err),
			TaskId:           task.Id.Hex(),
			RecommendedType:  recType,
			Mode:             mode,
			EstimatedMinutes: estimatedMinutes,
		}, nil
	}

	return &types.TaskQuickCreateResp{
		Code:             0,
		Msg:              "任务创建成功",
		TaskId:           task.Id.Hex(),
		RecommendedType:  recType,
		Mode:             mode,
		EstimatedMinutes: estimatedMinutes,
	}, nil
}

// recommendConfig 根据解析后的目标类型分布与模式，返回推荐的扫描配置与推荐类型。
// 推荐类型用于前端展示：port（端口扫描）/ domain（全面扫描）/ web（Web 扫描）。
func recommendConfig(parsed []*scanner.Target, mode string) (map[string]interface{}, string) {
	hasDomain, hasURL, hasIP := false, false, false
	for _, t := range parsed {
		switch t.Type {
		case scanner.TargetTypeURL:
			hasURL = true
		case scanner.TargetTypeDomain:
			hasDomain = true
		default: // ipv4 / ipv6 / cidr / range
			hasIP = true
		}
	}

	base := quickTemplateConfigJSON
	if mode == "full" {
		base = standardTemplateConfigJSON
	}
	var cfg map[string]interface{}
	_ = json.Unmarshal([]byte(base), &cfg)

	// 模块启用策略
	enableModule(cfg, "domainscan", hasDomain)
	enableModule(cfg, "portscan", hasDomain || hasIP)
	enableModule(cfg, "portidentify", (hasDomain || hasIP) && mode == "full")
	enableModule(cfg, "fingerprint", true)
	enableModule(cfg, "dirscan", (hasURL || hasDomain) && mode == "full")
	enableModule(cfg, "pocscan", mode == "full")
	enableModule(cfg, "brutescan", mode == "full")
	enableModule(cfg, "jsfinder", hasDomain || hasURL)
	// 内置模板无 certcheck 段，保持禁用以规避未知参数风险
	enableModule(cfg, "certcheck", false)

	// 推荐类型（前端展示用）
	var recType string
	switch {
	case hasURL && !hasDomain && !hasIP:
		recType = "web"
	case hasIP && !hasDomain && !hasURL:
		recType = "port"
	default:
		recType = "domain"
	}
	return cfg, recType
}

// enableModule 设置某扫描阶段的启用标志；段不存在则创建。
func enableModule(cfg map[string]interface{}, key string, enable bool) {
	section, ok := cfg[key].(map[string]interface{})
	if !ok {
		section = map[string]interface{}{}
		cfg[key] = section
	}
	section["enable"] = enable
}

func recommendedTypeName(t string) string {
	switch t {
	case "port":
		return "端口扫描"
	case "web":
		return "Web扫描"
	default:
		return "全面扫描"
	}
}

func modeName(m string) string {
	if m == "full" {
		return "深度"
	}
	return "快速"
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
