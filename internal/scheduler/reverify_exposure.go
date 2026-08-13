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
	Extracted []string `json:"extracted,omitempty"`
}

// ExposureReverifier 敏感信息持续复验调度器（T3.4）
type ExposureReverifier struct {
	db    *mongo.Database
	sched *Scheduler
}

// NewExposureReverifier 创建敏感信息复验调度器
func NewExposureReverifier(db *mongo.Database, sched *Scheduler) *ExposureReverifier {
	return &ExposureReverifier{db: db, sched: sched}
}

// Run 执行一次全量复验（手动触发或 runNow 端点调用）——忽略 NextRunTime，立即下发所有启用的配置
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

// RunDue sweep 模式：仅下发 NextRunTime <= now 且启用的配置
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

// RunWorkspace 立即下发复验（供 runNow 端点调用）
func (r *ExposureReverifier) RunWorkspace(ctx context.Context) error {
	return r.Run(ctx)
}

// dispatchWorkspace 收集待复验敏感信息并构造复验任务入队（探测由 Worker 执行）
func (r *ExposureReverifier) dispatchWorkspace(ctx context.Context, cfg model.ReverifyConfig) {
	now := time.Now()

	targets := r.collectTargets(ctx)
	if len(targets) == 0 {
		logx.Infof("[ExposureReverifier] 无待复验敏感信息，skip")
		_ = model.NewReverifyConfigModel(r.db).UpdateRunState(ctx, "default", now, "success", 0, "", NextReverifyRunTime(cfg.CronSpec, now))
		return
	}

	maxTargets := cfg.MaxTargetsPerRun
	if maxTargets <= 0 {
		maxTargets = defaultReverifyMaxTargets
	}
	if len(targets) > maxTargets {
		logx.Infof("[ExposureReverifier] 本次覆盖 %d/%d 个目标，剩余 %d 个将于下个周期继续（不静默截断）",
			maxTargets, len(targets), len(targets)-maxTargets)
		targets = targets[:maxTargets]
	}

	cfgBytes, mErr := json.Marshal(map[string]interface{}{
		"taskType": "reverify_exposure",
		"targets":  targets,
	})
	if mErr != nil {
		logx.Errorf("[ExposureReverifier] marshal reverify task failed: %v", mErr)
		return
	}
	task := &TaskInfo{
		TaskName: "reverify_exposure",
		Config:   string(cfgBytes),
		Priority: PriorityNormal,
	}
	if pErr := r.sched.PushTask(ctx, task); pErr != nil {
		logx.Errorf("[ExposureReverifier] PushTask failed: %v", pErr)
		return
	}
	if uErr := model.NewReverifyConfigModel(r.db).MarkDispatched(ctx, now, len(targets), NextReverifyRunTime(cfg.CronSpec, now)); uErr != nil {
		logx.Errorf("[ExposureReverifier] 推进复验调度状态失败: %v", uErr)
	}
	logx.Infof("[ExposureReverifier] 已下发敏感信息复验任务: %d 个目标（worker 执行，结果回传后回写状态）",
		len(targets))
}

// collectTargets 收集待复验的敏感信息泄露目标（jsfinder 泄露发现 + dirscan 已发现路径）
func (r *ExposureReverifier) collectTargets(ctx context.Context) []ReverifyExposureTarget {
	var targets []ReverifyExposureTarget

	jsModel := model.NewJSFinderResultModel(r.db)
	jsResults, err := jsModel.FindSensitiveForReverify(ctx, 0)
	if err != nil {
		logx.Errorf("[ExposureReverifier] load jsfinder sensitive failed: %v", err)
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

	dirModel := model.NewDirScanResultModel(r.db)
	dirResults, err := dirModel.FindFoundForReverify(ctx, 0)
	if err != nil {
		logx.Errorf("[ExposureReverifier] load dirscan found failed: %v", err)
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
