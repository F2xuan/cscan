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
//
// 设计要点：
//   - 探测动作全部由 Worker 执行（R8）：本器只负责到期判定、目标查询与任务入队；
//     Worker 复验完成后经 /api/v1/worker/reverify/result 回传，由 API 侧完成状态流转、通知与 NextRunTime 回写。
//   - 仅验证已记录的凭据组合（"确认是否已修复"），不做字典爆破（禁止事项）。
//   - 严格区分"目标不可达（连不上）"与"凭据无效"：不可达仅标记待确认，不误判为已修复。
type WeakPassReverifier struct {
	db    *mongo.Database
	sched *Scheduler
}

// NewWeakPassReverifier 创建弱口令复验调度器
func NewWeakPassReverifier(db *mongo.Database, sched *Scheduler) *WeakPassReverifier {
	return &WeakPassReverifier{db: db, sched: sched}
}

// Run 执行一次全量复验（可被手动触发或 runNow 端点调用）——忽略 NextRunTime，立即下发所有启用的 workspace
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
		r.dispatchWorkspace(ctx, cfg)
	}
	return nil
}

// RunDue sweep 模式：仅下发 NextRunTime <= now 且启用的 workspace（修复 H-8，尊重用户配置的 CronSpec）。
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
		// NextRunTime 为零值（首次）或已到期时执行
		if !cfg.NextRunTime.IsZero() && cfg.NextRunTime.After(now) {
			continue
		}
		logx.Infof("[WeakPassReverifier] reverify config due (next_run=%v), dispatching", cfg.NextRunTime)
		r.dispatchWorkspace(ctx, cfg)
	}
}

// RunWorkspace 对单个工作空间立即下发复验（供 runNow 端点调用）
func (r *WeakPassReverifier) RunWorkspace(ctx context.Context, workspaceId string) error {
	configModel := model.NewReverifyConfigModel(r.db)
	cfg, err := configModel.GetByWorkspace(ctx, workspaceId)
	if err != nil {
		return err
	}
	if cfg == nil || !cfg.WeakPassEnabled {
		logx.Infof("[WeakPassReverifier] workspace=%s 弱口令复验未启用，skip", workspaceId)
		return nil
	}
	r.dispatchWorkspace(ctx, *cfg)
	return nil
}

// dispatchWorkspace 查询待复验弱口令并构造复验任务入队（探测由 Worker 执行）
func (r *WeakPassReverifier) dispatchWorkspace(ctx context.Context, cfg model.ReverifyConfig) {
	wsId := "default"
	now := time.Now()
	vulModel := model.NewVulModel(r.db, wsId)

	maxTargets := cfg.MaxTargetsPerRun
	if maxTargets <= 0 {
		maxTargets = defaultReverifyMaxTargets
	}

	// 修复 M-11：按 last_reverify_time 升序 + limit 直接查询，保证轮转，避免目标永久饥饿。
	// 原实现按 create_time 固定排序，先创建的 open 漏洞反复被选中，后续漏洞永远得不到复验。
	vuls, err := vulModel.FindOpenByRiskSourceOrdered(ctx, model.VulRiskSourceWeakPass, maxTargets)
	if err != nil {
		logx.Errorf("[WeakPassReverifier] workspace=%s load weakpass vuls failed: %v", wsId, err)
		return
	}
	if len(vuls) == 0 {
		logx.Infof("[WeakPassReverifier] workspace=%s 无待复验弱口令，skip", wsId)
		_ = model.NewReverifyConfigModel(r.db).UpdateRunState(ctx, wsId, now, "success", 0, "", NextReverifyRunTime(cfg.CronSpec, now))
		return
	}

	// 构造复验目标列表（仅提取已知凭据，不做字典爆破）
	targets := make([]ReverifyWeakpassTarget, 0, len(vuls))
	for _, vul := range vuls {
		service, host, port, username, password, ok := parseReverifyTarget(vul)
		if !ok {
			logx.Infof("[WeakPassReverifier] workspace=%s skip vul %s: 无法解析目标(authority=%q url=%q)",
				wsId, vul.Id.Hex(), vul.Authority, vul.Url)
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
		logx.Infof("[WeakPassReverifier] workspace=%s 无可解析的复验目标，skip", wsId)
		_ = model.NewReverifyConfigModel(r.db).UpdateRunState(ctx, wsId, now, "success", len(vuls), "", NextReverifyRunTime(cfg.CronSpec, now))
		return
	}

	cfgBytes, mErr := json.Marshal(map[string]interface{}{
		"taskType":    "reverify_weakpass",
		"workspaceId": wsId,
		"targets":     targets,
	})
	if mErr != nil {
		logx.Errorf("[WeakPassReverifier] workspace=%s marshal reverify task failed: %v", wsId, mErr)
		return
	}
	task := &TaskInfo{
		WorkspaceId: wsId,
		TaskName:    "reverify_weakpass",
		Config:      string(cfgBytes),
		Priority:    PriorityNormal,
	}
	if pErr := r.sched.PushTask(ctx, task); pErr != nil {
		logx.Errorf("[WeakPassReverifier] workspace=%s PushTask failed: %v", wsId, pErr)
		return
	}
	// 入队即推进 NextRunTime：否则每分钟 sweep 会持续命中同一到期配置，
	// 在 Worker 回传结果前重复下发任务。终态状态由结果回传时回写。
	if uErr := model.NewReverifyConfigModel(r.db).MarkDispatched(ctx, now, len(targets), NextReverifyRunTime(cfg.CronSpec, now)); uErr != nil {
		logx.Errorf("[WeakPassReverifier] workspace=%s 推进复验调度状态失败: %v", wsId, uErr)
	}
	logx.Infof("[WeakPassReverifier] workspace=%s 已下发弱口令复验任务: %d 个目标（worker 执行，结果回传后流转状态）",
		wsId, len(targets))
}

// parseReverifyTarget 从漏洞的 Authority / Url 解析复验目标。
// 优先用 Authority(host:port) 取地址，Url 的 scheme 取服务名、userinfo 取已知凭据。
// 该解析不依赖 net/url（避免 oracle 等 URL 中 "(xe)" 后缀导致解析失败）。
func parseReverifyTarget(vul model.Vul) (service, host string, port int, username, password string, ok bool) {
	// host:port
	host, portStr, hasPort := strings.Cut(vul.Authority, ":")
	if host == "" {
		return
	}
	if hasPort {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	// service（scheme）与 user:pass（@ 前 userinfo）
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
