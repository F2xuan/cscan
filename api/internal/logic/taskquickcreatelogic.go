package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/api/internal/svc/sync"
	"cscan/api/internal/types"
	"cscan/internal/model"
	"cscan/internal/scanner"
	"cscan/internal/scheduler"

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

	// 直接对标系统内置扫描模板：quick → 内置「快速扫描」(category=quick)，
	// full → 内置「标准扫描」(category=standard)。以内置模板配置为基准，
	// 在其之上叠加按目标类型的智能模块启停（recommendConfig），保证与模板参数同源，
	// 避免此前在 quickCreate 中维护的重复副本与模板漂移。
	baseCategory := "quick"
	if mode == "full" {
		baseCategory = "standard"
	}
	baseCfg := l.loadBuiltinTemplateConfig(baseCategory)

	parser := scanner.NewTargetParser()
	parsed := parser.ParseMultiple(req.Targets)
	if len(parsed) == 0 {
		return &types.TaskQuickCreateResp{Code: 400, Msg: "未能解析出有效目标"}, nil
	}

	taskConfig, recType := recommendConfig(parsed, mode, baseCfg)
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
		CreatedBy:   middleware.GetUserId(l.ctx),
	}
	taskModel := l.svcCtx.GetMainTaskModel()
	if err := taskModel.Insert(l.ctx, task); err != nil {
		l.Logger.Errorf("TaskQuickCreate: insert failed, taskId=%s, error=%v", taskId, err)
		return &types.TaskQuickCreateResp{Code: 500, Msg: "创建任务失败: " + err.Error()}, nil
	}

	// 复用统一任务启动逻辑
	// 修复 M-16：启动失败时必须明确返回错误状态及任务 ID，避免前端误判任务已成功启动
	builder := common.NewTaskBuilder(l.ctx, l.svcCtx)
	if _, err := builder.BuildAndPushSubTasks(task, taskConfig); err != nil {
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

// loadBuiltinTemplateConfig 按分类加载内置扫描模板的配置，作为一键扫描的基准。
// 与系统内置模板（快速扫描 / 标准扫描）直接对标，复用模板参数（端口范围、速率、超时、字典等），
// 避免此前在 quickCreate 中维护的重复副本与模板漂移。DB 无对应模板或解析失败时回退到内置常量。
func (l *TaskQuickCreateLogic) loadBuiltinTemplateConfig(category string) map[string]interface{} {
	if l.svcCtx.ScanTemplateModel != nil {
		if builtins, err := l.svcCtx.ScanTemplateModel.FindBuiltinTemplates(l.ctx); err == nil {
			for _, tpl := range builtins {
				if tpl.Category == category {
					var cfg map[string]interface{}
					if err := json.Unmarshal([]byte(tpl.Config), &cfg); err == nil {
						l.Logger.Infof("TaskQuickCreate: loaded builtin template config, category=%s name=%s", category, tpl.Name)
						return cfg
					}
				}
			}
		}
	}
	// 回退：从 rules/scan-template 文件直接读取（单一真相源，与 InitBuiltinTemplates 加载的是同一组文件）
	if cfg := sync.LoadBuiltinTemplateConfig(category); cfg != nil {
		return cfg
	}
	return nil
}

// recommendConfig 根据解析后的目标类型分布与模式，在 base（内置模板配置）之上
// 智能启用/停用对应扫描阶段，返回推荐的扫描配置与推荐类型。
// 推荐类型用于前端展示：port（端口扫描）/ domain（全面扫描）/ web（Web 扫描）。
// base 已由 loadBuiltinTemplateConfig 按分类从内置模板加载，此处不覆盖模板参数，仅调整 enable 开关。
func recommendConfig(parsed []*scanner.Target, mode string, base map[string]interface{}) (map[string]interface{}, string) {
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

	// base 已是按 mode 从内置模板(quick/standard)加载的配置，模板即“轻重”唯一真相源。
	// 各扫描阶段在 worker 内自行判断输入是否适用（如子域名扫描仅对纯一级域名生效，
	// IP/CIDR/子域名直接跳过；端口识别/指纹等阶段无资产时自动 skipped）。
	// 编排层不再按目标类型或 mode 强制启停模块，避免与模板定义漂移。
	cfg := base

	// 推荐类型（仅前端展示用）
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
		return "标准"
	}
	return "快速"
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
