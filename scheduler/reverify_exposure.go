package scheduler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"cscan/model"
	"cscan/pkg/httpclient"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/mongo"
)

// 敏感信息复验默认参数（T3.4）
const (
	defaultExposureProbeTimeout = 5 * time.Second // 单 URL 探测超时
)

// ExposureReverifier 敏感信息持续复验器（T3.4）
//
// 设计要点（对齐 T3.3 弱口令复验，且匹配实际数据存储架构）：
//   - 敏感信息发现产物存于独立集合 {wsId}_jsfinder（URL + ExtractedResults）与全局 dirscan_result（URL + StatusCode），
//     并不写入 {wsId}_vul（deriveRiskSource 不产出 auto:info-leak，且 jsfinder/dirscan 不走 gRPC SaveVulResult）。
//     故复验直接探测这些发现集合中的 URL，而非 {wsId}_vul 的 risk_source=auto:info-leak（该路径当前无数据）。
//   - 严格区分"目标不可达（连不上）"与"已不可访问（404/410）"：不可达仅标记待确认，不误判为已修复（验收标准 3）。
//   - 内容特征兜底（验收标准 4）：URL 仍返回 200（软 404）时，若原 ExtractedResults 敏感内容已不在响应体中，
//     判定为已修复（内容消失）；若仍包含敏感内容，判定为仍泄露。
type ExposureReverifier struct {
	db     *mongo.Database
	sender ReverifySender
}

// NewExposureReverifier 创建敏感信息复验器
func NewExposureReverifier(db *mongo.Database, sender ReverifySender) *ExposureReverifier {
	return &ExposureReverifier{db: db, sender: sender}
}

// Run 执行一次全量复验（手动触发或 runNow 端点调用）——忽略 NextRunTime，立即执行所有启用的 workspace
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
		r.reverifyWorkspace(ctx, cfg)
	}
	return nil
}

// RunDue sweep 模式：仅执行 NextRunTime <= now 且启用的 workspace（修复 H-8，尊重用户配置的 CronSpec）
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
		logx.Infof("[ExposureReverifier] reverify config due (next_run=%v), running", cfg.NextRunTime)
		r.reverifyWorkspace(ctx, cfg)
	}
}

// RunWorkspace 对单个工作空间立即执行复验（供 runNow 端点调用）
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
	r.reverifyWorkspace(ctx, *cfg)
	return nil
}

// exposureOutcome 探测分类结果
type exposureOutcome int

const (
	exposurePending  exposureOutcome = iota // 目标不可达，待确认（不误判为已修复）
	exposureResolved                        // 已确认不可访问 / 内容已消失 → 视为修复
	exposureVerified                        // 仍存在 / 仍暴露
)

// exposureTarget 单个复验目标
type exposureTarget struct {
	kind      string // "jsfinder" / "dirscan"
	id        string
	url       string
	extracted []string // 原泄露内容特征（用于软 404 内容兜底）
}

// reverifyWorkspace 复验单个工作空间的敏感信息泄露
func (r *ExposureReverifier) reverifyWorkspace(ctx context.Context, cfg model.ReverifyConfig) {
	wsId := "default"
	now := time.Now()

	targets := r.collectTargets(ctx, wsId)
	if len(targets) == 0 {
		logx.Infof("[ExposureReverifier] workspace=%s 无待复验敏感信息，skip", wsId)
		_ = model.NewReverifyConfigModel(r.db).UpdateRunState(ctx, wsId, now, "success", 0, "", nextReverifyRunTime(cfg.CronSpec, now))
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

	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = defaultReverifyConcurrency
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var resolved, verified, pending []exposureTarget

	for _, t := range targets {
		sem <- struct{}{}
		wg.Add(1)
		go func(t exposureTarget) {
			defer wg.Done()
			defer func() { <-sem }()

			outcome := probeExposure(ctx, t, defaultExposureProbeTimeout)
			mu.Lock()
			switch outcome {
			case exposureResolved:
				resolved = append(resolved, t)
			case exposureVerified:
				verified = append(verified, t)
			default:
				pending = append(pending, t)
			}
			mu.Unlock()
		}(t)
	}
	wg.Wait()

	// 回写各记录复验状态
	r.applyOutcome(ctx, wsId, resolved, exposureResolved)
	r.applyOutcome(ctx, wsId, verified, exposureVerified)
	r.applyOutcome(ctx, wsId, pending, exposurePending)

	resolvedCount := len(resolved)
	verifiedCount := len(verified)
	pendingCount := len(pending)

	// 回写运行状态（不触碰配置字段）
	_ = model.NewReverifyConfigModel(r.db).UpdateRunState(ctx, wsId, now, "success", len(targets), "", nextReverifyRunTime(cfg.CronSpec, now))

	// 通知（修复/不可访问确认汇总，走 T1.4 的 FixedVulCount 进通知）
	if resolvedCount > 0 {
		sendReverifyNotify(ctx, r.sender, buildReverifyNotify("exposure-reverify", "敏感信息持续复验", wsId, resolvedCount), wsId, "exposure-reverify")
	}

	logx.Infof("[ExposureReverifier] workspace=%s 复验完成: 共%d 已修复/不可访问%d 仍暴露%d 不可达%d",
		wsId, len(targets), resolvedCount, verifiedCount, pendingCount)
}

// collectTargets 收集待复验的敏感信息泄露目标（jsfinder 泄露发现 + dirscan 已发现路径）
func (r *ExposureReverifier) collectTargets(ctx context.Context, wsId string) []exposureTarget {
	var targets []exposureTarget

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
			targets = append(targets, exposureTarget{
				kind:      "jsfinder",
				id:        j.Id.Hex(),
				url:       j.URL,
				extracted: j.ExtractedResults,
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
			targets = append(targets, exposureTarget{
				kind: "dirscan",
				id:   d.Id.Hex(),
				url:  d.URL,
			})
		}
	}

	return targets
}

// probeExposure 探测敏感 URL 的可访问性并分类（纯函数，便于单测）。
func probeExposure(ctx context.Context, t exposureTarget, timeout time.Duration) exposureOutcome {
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, t.url, nil)
	if err != nil {
		// URL 非法 → 视为不可达，待确认
		return exposurePending
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; CScan-Reverify/1.0)")

	resp, err := httpclient.Do(req)
	if err != nil {
		// 连接拒绝 / 超时 / DNS 失败 / TLS 错误：目标不可达，仅标记待确认，不误判为已修复（验收标准 3）
		return exposurePending
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotFound, http.StatusGone: // 404 / 410：资源已移除 → 已修复
		return exposureResolved
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout: // 5xx：暂态错误，不判定为已修复
		return exposurePending
	case http.StatusForbidden, http.StatusUnauthorized, http.StatusMethodNotAllowed: // 403/401/405：仍存在但访问受限 → 仍暴露
		return exposureVerified
	}

	// 2xx / 3xx（或其它 4xx）：可达。内容特征兜底（验收标准 4）：
	// 若原泄露内容（ExtractedResults）非空且当前响应体中已不再出现 → 视为已修复（软 404 / 内容消失）。
	if len(t.extracted) > 0 {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
		if readErr == nil {
			bodyLower := strings.ToLower(string(body))
			stillLeaking := false
			for _, ex := range t.extracted {
				if ex == "" {
					continue
				}
				if strings.Contains(bodyLower, strings.ToLower(ex)) {
					stillLeaking = true
					break
				}
			}
			if !stillLeaking {
				return exposureResolved
			}
		}
	}
	return exposureVerified
}

// applyOutcome 将分类结果回写到对应集合的记录
func (r *ExposureReverifier) applyOutcome(ctx context.Context, wsId string, items []exposureTarget, outcome exposureOutcome) {
	if len(items) == 0 {
		return
	}
	var jsIds, dirIds []string
	for _, it := range items {
		switch it.kind {
		case "jsfinder":
			jsIds = append(jsIds, it.id)
		case "dirscan":
			dirIds = append(dirIds, it.id)
		}
	}
	status := reverifyStatusFromOutcome(outcome)
	now := time.Now()
	pending := outcome == exposurePending

	if len(jsIds) > 0 {
		jsModel := model.NewJSFinderResultModel(r.db, wsId)
		if err := jsModel.MarkReverify(ctx, jsIds, status, now, pending); err != nil {
			logx.Errorf("[ExposureReverifier] workspace=%s mark jsfinder reverify failed: %v", wsId, err)
		}
	}
	if len(dirIds) > 0 {
		dirModel := model.NewDirScanResultModel(r.db)
		if err := dirModel.MarkReverify(ctx, dirIds, status, now, pending); err != nil {
			logx.Errorf("[ExposureReverifier] workspace=%s mark dirscan reverify failed: %v", wsId, err)
		}
	}
}

// reverifyStatusFromOutcome 将分类结果映射为存储状态串
func reverifyStatusFromOutcome(o exposureOutcome) string {
	switch o {
	case exposureResolved:
		return "resolved"
	case exposureVerified:
		return "verified"
	default:
		return "pending"
	}
}
