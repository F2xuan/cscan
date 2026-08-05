package scheduler

import (
	"context"
	"encoding/json"
	"time"

	"cscan/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/mongo"
)

// 探测超时已随实现迁移到 Worker 侧（worker_reverify.go）

// ReverifyExposureTarget 单条敏感信息复验目标（随任务下发 Worker 执行）
type ReverifyExposureTarget struct {
	Kind      string   `json:"kind"` // "jsfinder" / "dirscan"
	Id        string   `json:"id"`
	Url       string   `json:"url"`
	Extracted []string `json:"extracted,omitempty"` // 原泄露内容特征（软 404 内容兜底）
}

// ExposureReverifier 敏感信息持续复验调度器（T3.4）
//
// 设计要点（对齐 T3.3 弱口令复验，且匹配实际数据存储架构）：
//   - 敏感信息发现产物存于独立集合 jsfinder（URL + ExtractedResults）与全局 dirscan_result（URL + StatusCode），
//     并不写入 vul（deriveRiskSource 不产出 auto:info-leak，且 jsfinder/dirscan 不走 gRPC SaveVulResult）。
//     故复验直接探测这些发现集合中的 URL，而非 vul 的 risk_source=auto:info-leak（该路径当前无数据）。
//   - 探测动作全部由 Worker 执行（R8）：本器只负责到期判定、目标查询与任务入队；
//     Worker 复验完成后经 /api/v1/worker/reverify/result 回传，由 API 侧完成状态回写、通知与 NextRunTime 回写。
type ExposureReverifier struct {
	db    *mongo.Database
	sched *Scheduler
}

// NewExposureReverifier 创建敏感信息复验调度器
func NewExposureReverifier(db *mongo.Database, sched *Scheduler) *ExposureReverifier {
	return &ExposureReverifier{db: db, sched: sched}
}

// Run 执行一次全量复验（手动触发或 runNow 端点调用）——忽略 NextRunTime，立即下发所有启用的 workspace
func (r *ExposureReverifier) Run(ctx context.Context) error {
	configModel := model.NewReverifyConfigModel(r.db)
	configs, err := configModel.FindEnabledExposure(ctx)
	if err != nil {
		logx.Errorf("[ExposureReverifier] load enabled configs failed: %v", err)
		return err
	}
	if len(configs) == 0 {
		logx.Infof("[ExposureReverifier] no enabled reverify config, skip")
		return nil
	}
	for _, cfg := range configs {
		r.dispatchWorkspace(ctx, cfg)
	}
	return nil
}

// RunDue sweep 模式：仅下发 NextRunTime <= now 且启用的 workspace（修复 H-8，尊重用户配置的 CronSpec）
// 执行后的 NextRunTime 由 Worker 结果回传时按各自 CronSpec 计算回写。
func (r *ExposureReverifier) RunDue(ctx context.Context) {
	configModel := model.NewReverifyConfigModel(r.db)
	configs, err := configModel.FindEnabledExposure(ctx)
	if err != nil {
		logx.Errorf("[ExposureReverifier] load enabled configs failed: %v", err)
		return
	}
	now := time.Now()
	for _, cfg := range configs {
		if !cfg.NextRunTime.IsZero() && cfg.NextRunTime.After(now) {
			continue
		}
		logx.Infof("[ExposureReverifier] reverify config due (next_run=%v), dispatching", cfg.NextRunTime)
		r.dispatchWorkspace(ctx, cfg)
	}
}

// RunWorkspace 对单个工作空间立即下发复验（供 runNow 端点调用）
func (r *ExposureReverifier) RunWorkspace(ctx context.Context, workspaceId string) error {
	configModel := model.NewReverifyConfigModel(r.db)
	cfg, err := configModel.GetByWorkspace(ctx, workspaceId)
	if err != nil {
		return err
	}
	if cfg == nil || !cfg.ExposureEnabled {
		logx.Infof("[ExposureReverifier] workspace=%s 敏感信息复验未启用，skip", workspaceId)
		return nil
	}
	r.dispatchWorkspace(ctx, *cfg)
	return nil
}

// dispatchWorkspace 收集待复验敏感信息并构造复验任务入队（探测由 Worker 执行）
func (r *ExposureReverifier) dispatchWorkspace(ctx context.Context, cfg model.ReverifyConfig) {
	wsId := "default"
	now := time.Now()

	targets := r.collectTargets(ctx, wsId)
	if len(targets) == 0 {
		logx.Infof("[ExposureReverifier] workspace=%s 无待复验敏感信息，skip", wsId)
		_ = model.NewReverifyConfigModel(r.db).UpdateRunState(ctx, wsId, now, "success", 0, "", NextReverifyRunTime(cfg.CronSpec, now))
		return
	}

	maxTargets := cfg.MaxTargetsPerRun
	if maxTargets <= 0 {
		maxTargets = defaultReverifyMaxTargets
	}
	// 覆盖日志：超出部分下个周期继续，禁止静默截断（验收标准对齐 T3.3）
	if len(targets) > maxTargets {
		logx.Infof("[ExposureReverifier] workspace=%s 本次覆盖 %d/%d 个目标，剩余 %d 个将于下个周期继续（不静默截断）",
			wsId, maxTargets, len(targets), len(targets)-maxTargets)
		targets = targets[:maxTargets]
	}

	cfgBytes, mErr := json.Marshal(map[string]interface{}{
		"taskType":    "reverify_exposure",
		"workspaceId": wsId,
		"targets":     targets,
	})
	if mErr != nil {
		logx.Errorf("[ExposureReverifier] workspace=%s marshal reverify task failed: %v", wsId, mErr)
		return
	}
	task := &TaskInfo{
		WorkspaceId: wsId,
		TaskName:    "reverify_exposure",
		Config:      string(cfgBytes),
		Priority:    PriorityNormal,
	}
	if pErr := r.sched.PushTask(ctx, task); pErr != nil {
		logx.Errorf("[ExposureReverifier] workspace=%s PushTask failed: %v", wsId, pErr)
		return
	}
	// 入队即推进 NextRunTime：否则每分钟 sweep 会持续命中同一到期配置，
	// 在 Worker 回传结果前重复下发任务。终态状态由结果回传时回写。
	if uErr := model.NewReverifyConfigModel(r.db).MarkDispatched(ctx, now, len(targets), NextReverifyRunTime(cfg.CronSpec, now)); uErr != nil {
		logx.Errorf("[ExposureReverifier] workspace=%s 推进复验调度状态失败: %v", wsId, uErr)
	}
	logx.Infof("[ExposureReverifier] workspace=%s 已下发敏感信息复验任务: %d 个目标（worker 执行，结果回传后回写状态）",
		wsId, len(targets))
}

// collectTargets 收集待复验的敏感信息泄露目标（jsfinder 泄露发现 + dirscan 已发现路径）
func (r *ExposureReverifier) collectTargets(ctx context.Context, wsId string) []ReverifyExposureTarget {
	var targets []ReverifyExposureTarget

	// 1) JSFinder 泄露发现（含 info-leak / sensitive / high-risk 标签，URL + 原泄露内容特征）
	jsModel := model.NewJSFinderResultModel(r.db, wsId)
	jsResults, err := jsModel.FindSensitiveForReverify(ctx, 0)
	if err != nil {
		logx.Errorf("[ExposureReverifier] workspace=%s load jsfinder sensitive failed: %v", wsId, err)
	} else {
		for _, j := range jsResults {
			if j.URL == "" {
				continue
			}
			targets = append(targets, ReverifyExposureTarget{
				Kind:      "jsfinder",
				Id:        j.Id.Hex(),
				Url:       j.URL,
				Extracted: j.ExtractedResults,
			})
		}
	}

	// 2) DirScan 已发现路径（状态码 2xx/3xx 视为暴露，URL 为探测目标）
	dirModel := model.NewDirScanResultModel(r.db)
	dirResults, err := dirModel.FindFoundForReverify(ctx, wsId, 0)
	if err != nil {
		logx.Errorf("[ExposureReverifier] workspace=%s load dirscan found failed: %v", wsId, err)
	} else {
		for _, d := range dirResults {
			if d.URL == "" {
				continue
			}
			targets = append(targets, ReverifyExposureTarget{
				Kind: "dirscan",
				Id:   d.Id.Hex(),
				Url:  d.URL,
			})
		}
	}

	return targets
}
