package scheduler

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"cscan/model"
	"cscan/pkg/notify"
	"cscan/scanner/brute"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/mongo"
)

// 弱口令复验专用超时（T3.3）；cron / maxTargets / concurrency 与 T3.4 共用，见 reverify_common.go
const (
	defaultReverifyTimeout      = 10 // 单次凭据验证超时（秒）
	defaultReverifyProbeTimeout = 3  // 可达性探测超时（秒）
)

// WeakPassReverifier 弱口令持续复验器（T3.3）
//
// 设计要点：
//   - 在 master 侧执行，不下发 SubTask，不占用 Worker 扫描队列（R8）。
//   - 仅验证已记录的凭据组合（"确认是否已修复"），不做字典爆破（禁止事项）。
//   - 严格区分"目标不可达（连不上）"与"凭据无效"：不可达仅标记待确认，不误判为已修复。
//   - 复验结果经 T1.3 状态机流转（MarkReverified / MarkFixed(auto:rescan) / MarkVerifyUnreachable）。
type WeakPassReverifier struct {
	db     *mongo.Database
	sender ReverifySender
}

// NewWeakPassReverifier 创建弱口令复验器
func NewWeakPassReverifier(db *mongo.Database, sender ReverifySender) *WeakPassReverifier {
	return &WeakPassReverifier{db: db, sender: sender}
}

// Run 执行一次全量复验（可被手动触发或 runNow 端点调用）——忽略 NextRunTime，立即执行所有启用的 workspace
func (r *WeakPassReverifier) Run(ctx context.Context) error {
	configModel := model.NewReverifyConfigModel(r.db)
	configs, err := configModel.FindEnabledWeakPass(ctx)
	if err != nil {
		logx.Errorf("[WeakPassReverifier] load enabled configs failed: %v", err)
		return err
	}
	if len(configs) == 0 {
		logx.Infof("[WeakPassReverifier] no enabled workspace config, skip")
		return nil
	}

	for _, cfg := range configs {
		if cfg.WorkspaceId == "" {
			continue
		}
		r.reverifyWorkspace(ctx, cfg)
	}
	return nil
}

// RunDue sweep 模式：仅执行 NextRunTime <= now 且启用的 workspace（修复 H-8，尊重用户配置的 CronSpec）。
// 原子性：先查询到期配置，执行后按各自 CronSpec 计算下次运行时间并回写 NextRunTime。
func (r *WeakPassReverifier) RunDue(ctx context.Context) {
	configModel := model.NewReverifyConfigModel(r.db)
	configs, err := configModel.FindEnabledWeakPass(ctx)
	if err != nil {
		logx.Errorf("[WeakPassReverifier] load enabled configs failed: %v", err)
		return
	}
	now := time.Now()
	for _, cfg := range configs {
		if cfg.WorkspaceId == "" {
			continue
		}
		// NextRunTime 为零值（首次）或已到期时执行
		if !cfg.NextRunTime.IsZero() && cfg.NextRunTime.After(now) {
			continue
		}
		logx.Infof("[WeakPassReverifier] workspace=%s due (next_run=%v), running", cfg.WorkspaceId, cfg.NextRunTime)
		r.reverifyWorkspace(ctx, cfg)
	}
}

// RunWorkspace 对单个工作空间立即执行复验（供 runNow 端点调用）
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
	r.reverifyWorkspace(ctx, *cfg)
	return nil
}

// reverifyWorkspace 复验单个工作空间的弱口令漏洞
func (r *WeakPassReverifier) reverifyWorkspace(ctx context.Context, cfg model.ReverifyConfig) {
	wsId := cfg.WorkspaceId
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
		_ = model.NewReverifyConfigModel(r.db).UpdateRunState(ctx, wsId, now, "success", 0, "", nextReverifyRunTime(cfg.CronSpec, now))
		return
	}

	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = defaultReverifyConcurrency
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var fixedIDs, verifiedIDs, pendingIDs []string

	for _, vul := range vuls {
		sem <- struct{}{}
		wg.Add(1)
		go func(vul model.Vul) {
			defer wg.Done()
			defer func() { <-sem }()

			service, host, port, username, password, ok := parseReverifyTarget(vul)
			if !ok {
				logx.Infof("[WeakPassReverifier] workspace=%s skip vul %s: 无法解析目标(authority=%q url=%q)",
					wsId, vul.Id.Hex(), vul.Authority, vul.Url)
				return
			}
			plugin := brute.GetPlugin(service)
			if plugin == nil {
				logx.Infof("[WeakPassReverifier] workspace=%s 未知服务 %q，跳过", wsId, service)
				return
			}
			if port == 0 {
				port = brute.ServicePortMap[service]
			}

			// 1) 可达性探测：不可达 → 标记待确认，不误判为已修复（验收标准 5）
			probeCtx, probeCancel := context.WithTimeout(ctx, time.Duration(defaultReverifyProbeTimeout)*time.Second)
			reachable := plugin.Probe(probeCtx, host, port)
			probeCancel()
			if !reachable {
				mu.Lock()
				pendingIDs = append(pendingIDs, vul.Id.Hex())
				mu.Unlock()
				logx.Infof("[WeakPassReverifier] workspace=%s %s:%d 不可达，标记待确认（不误判修复）", wsId, host, port)
				return
			}

			// 2) 仅验证已知凭据（不爆破）：凭据仍有效 → 仍弱；凭据无效 → 已修复
			// 修复 H-9：依据 ErrorType 严格区分"明确认证拒绝"与"网络/协议错误"，
			// 后者无法判定凭据是否仍有效，必须标记为 pending（待确认），不得误判为已修复。
			verifyCtx, verifyCancel := context.WithTimeout(ctx, time.Duration(defaultReverifyTimeout)*time.Second)
			defer verifyCancel()
			result := plugin.Brute(verifyCtx, host, port, []string{username}, []string{password}, defaultReverifyTimeout)

			mu.Lock()
			switch {
			case result.Success:
				// 凭据仍有效，漏洞仍存在
				verifiedIDs = append(verifiedIDs, vul.Id.Hex())
			case result.ErrorType == "auth_reject":
				// 明确认证拒绝 → 凭据已失效 → 已修复
				fixedIDs = append(fixedIDs, vul.Id.Hex())
			default:
				// 网络错误/协议错误/超时等 → 无法判定 → 待确认（不误判修复）
				pendingIDs = append(pendingIDs, vul.Id.Hex())
				logx.Infof("[WeakPassReverifier] workspace=%s %s:%d verify inconclusive (errorType=%q msg=%q), marking pending",
					wsId, host, port, result.ErrorType, result.Message)
			}
			mu.Unlock()
		}(vul)
	}
	wg.Wait()

	// 状态流转
	if len(verifiedIDs) > 0 {
		if _, e := vulModel.MarkReverified(ctx, verifiedIDs); e != nil {
			logx.Errorf("[WeakPassReverifier] workspace=%s MarkReverified failed: %v", wsId, e)
		}
	}
	if len(pendingIDs) > 0 {
		if _, e := vulModel.MarkVerifyUnreachable(ctx, pendingIDs); e != nil {
			logx.Errorf("[WeakPassReverifier] workspace=%s MarkVerifyUnreachable failed: %v", wsId, e)
		}
	}
	if len(fixedIDs) > 0 {
		if _, e := vulModel.MarkFixed(ctx, fixedIDs, model.VulFixSourceRescan); e != nil {
			logx.Errorf("[WeakPassReverifier] workspace=%s MarkFixed failed: %v", wsId, e)
		}
	}

	fixedCount := len(fixedIDs)
	verifiedCount := len(verifiedIDs)
	pendingCount := len(pendingIDs)

	// 回写运行状态（不触碰配置字段）
	runStatus := "success"
	runErr := ""
	if fixedCount == 0 && verifiedCount == 0 && pendingCount == 0 {
		runStatus = "success"
	}
	_ = model.NewReverifyConfigModel(r.db).UpdateRunState(ctx, wsId, now, runStatus, len(vuls), runErr, nextReverifyRunTime(cfg.CronSpec, now))

	// 通知（修复确认汇总，走 T1.4 的 FixedVulCount 进通知）
	if fixedCount > 0 && r.sender != nil {
		result := &notify.NotifyResult{
			TaskId:      "weakpass-reverify",
			TaskName:    "弱口令持续复验",
			Status:      "SUCCESS",
			WorkspaceId: wsId,
			HighRiskInfo: &notify.HighRiskInfo{
				FixedVulCount: fixedCount,
			},
		}
		if sErr := r.sender(ctx, result); sErr != nil {
			logx.Errorf("[WeakPassReverifier] workspace=%s 发送修复通知失败: %v", wsId, sErr)
		}
	}

	logx.Infof("[WeakPassReverifier] workspace=%s 复验完成: 共%d 已修复%d 仍有效%d 不可达%d",
		wsId, len(vuls), fixedCount, verifiedCount, pendingCount)
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
