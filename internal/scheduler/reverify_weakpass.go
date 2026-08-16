package scheduler

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"cscan/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/mongo"
)

// 探测超时已随实现迁移到 Worker 侧（worker_reverify.go）；
// cron / maxTargets 与 T3.4 共用，见 reverify_common.go

// ReverifyWeakpassTarget 单条弱口令复验目标（随任务下发 Worker 执行）
type ReverifyWeakpassTarget struct {
	VulnId   string `json:"vulnId"`
	Service  string `json:"service"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// WeakPassReverifier 弱口令持续复验调度器（T3.3）
type WeakPassReverifier struct {
	db    *mongo.Database
	sched *Scheduler
}

// NewWeakPassReverifier 创建弱口令复验调度器
func NewWeakPassReverifier(db *mongo.Database, sched *Scheduler) *WeakPassReverifier {
	return &WeakPassReverifier{db: db, sched: sched}
}

// Run 执行一次全量复验（可被手动触发或 runNow 端点调用）——忽略 NextRunTime，立即下发所有启用的配置
func (r *WeakPassReverifier) Run(ctx context.Context) error {
	configModel := model.NewReverifyConfigModel(r.db)
	configs, err := configModel.FindEnabledWeakPass(ctx)
	if err != nil {
		logx.Errorf("[WeakPassReverifier] load enabled configs failed: %v", err)
		return err
	}
	if len(configs) == 0 {
		logx.Infof("[WeakPassReverifier] no enabled reverify config, skip")
		return nil
	}

	for _, cfg := range configs {
		r.dispatchReverify(ctx, cfg)
	}
	return nil
}

// RunDue sweep 模式：仅下发 NextRunTime <= now 且启用的配置（修复 H-8，尊重用户配置的 CronSpec）。
// 执行后的 NextRunTime 由 Worker 结果回传时按各自 CronSpec 计算回写。
func (r *WeakPassReverifier) RunDue(ctx context.Context) {
	configModel := model.NewReverifyConfigModel(r.db)
	configs, err := configModel.FindEnabledWeakPass(ctx)
	if err != nil {
		logx.Errorf("[WeakPassReverifier] load enabled configs failed: %v", err)
		return
	}
	now := time.Now()
	for _, cfg := range configs {
		if !cfg.NextRunTime.IsZero() && cfg.NextRunTime.After(now) {
			continue
		}
		logx.Infof("[WeakPassReverifier] reverify config due (next_run=%v), dispatching", cfg.NextRunTime)
		r.dispatchReverify(ctx, cfg)
	}
}

// RunNow 立即下发复验（供 runNow 端点调用），忽略 NextRunTime
func (r *WeakPassReverifier) RunNow(ctx context.Context) error {
	return r.Run(ctx)
}

// dispatchReverify 查询待复验弱口令并构造复验任务入队（探测由 Worker 执行）
func (r *WeakPassReverifier) dispatchReverify(ctx context.Context, cfg model.ReverifyConfig) {
	now := time.Now()
	vulModel := model.NewVulModel(r.db)

	maxTargets := cfg.MaxTargetsPerRun
	if maxTargets <= 0 {
		maxTargets = defaultReverifyMaxTargets
	}

	vuls, err := vulModel.FindOpenByRiskSourceOrdered(ctx, model.VulRiskSourceWeakPass, maxTargets)
	if err != nil {
		logx.Errorf("[WeakPassReverifier] load weakpass vuls failed: %v", err)
		return
	}
	if len(vuls) == 0 {
		logx.Infof("[WeakPassReverifier] 无待复验弱口令，skip")
		_ = model.NewReverifyConfigModel(r.db).UpdateRunState(ctx, "default", now, "success", 0, "", NextReverifyRunTime(cfg.CronSpec, now))
		return
	}

	targets := make([]ReverifyWeakpassTarget, 0, len(vuls))
	for _, vul := range vuls {
		service, host, port, username, password, ok := parseReverifyTarget(vul)
		if !ok {
			logx.Infof("[WeakPassReverifier] skip vul %s: 无法解析目标(authority=%q url=%q)",
				vul.Id.Hex(), vul.Authority, vul.Url)
			continue
		}
		targets = append(targets, ReverifyWeakpassTarget{
			VulnId:   vul.Id.Hex(),
			Service:  service,
			Host:     host,
			Port:     port,
			Username: username,
			Password: password,
		})
	}
	if len(targets) == 0 {
		logx.Infof("[WeakPassReverifier] 无可解析的复验目标，skip")
		_ = model.NewReverifyConfigModel(r.db).UpdateRunState(ctx, "default", now, "success", len(vuls), "", NextReverifyRunTime(cfg.CronSpec, now))
		return
	}

	cfgBytes, mErr := json.Marshal(map[string]interface{}{
		"taskType": "reverify_weakpass",
		"targets":  targets,
	})
	if mErr != nil {
		logx.Errorf("[WeakPassReverifier] marshal reverify task failed: %v", mErr)
		return
	}
	task := &TaskInfo{
		TaskName: "reverify_weakpass",
		Config:   string(cfgBytes),
		Priority: PriorityNormal,
	}
	if pErr := r.sched.PushTask(ctx, task); pErr != nil {
		logx.Errorf("[WeakPassReverifier] PushTask failed: %v", pErr)
		return
	}
	if uErr := model.NewReverifyConfigModel(r.db).MarkDispatched(ctx, now, len(targets), NextReverifyRunTime(cfg.CronSpec, now)); uErr != nil {
		logx.Errorf("[WeakPassReverifier] 推进复验调度状态失败: %v", uErr)
	}
	logx.Infof("[WeakPassReverifier] 已下发弱口令复验任务: %d 个目标（worker 执行，结果回传后流转状态）",
		len(targets))
}

// parseReverifyTarget 从漏洞的 Authority / Url 解析复验目标。
func parseReverifyTarget(vul model.Vul) (service, host string, port int, username, password string, ok bool) {
	host, portStr, hasPort := strings.Cut(vul.Authority, ":")
	if host == "" {
		return
	}
	if hasPort {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	if i := strings.Index(vul.Url, "://"); i >= 0 {
		service = vul.Url[:i]
		rest := vul.Url[i+3:]
		if at := strings.Index(rest, "@"); at >= 0 {
			userinfo := rest[:at]
			if c := strings.Index(userinfo, ":"); c >= 0 {
				username = userinfo[:c]
				password = userinfo[c+1:]
			} else {
				username = userinfo
			}
		}
	}

	if service == "" || host == "" {
		return "", "", 0, "", "", false
	}
	return service, host, port, username, password, true
}
