package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cscan/internal/scanner/brute"
	"cscan/internal/scheduler"
	"cscan/pkg/httpclient"
)

// 复验超时（对齐原 scheduler 侧取值）
const (
	reverifyWeakpassProbeTimeout = 3 * time.Second  // 可达性探测超时
	reverifyWeakpassVerifyTtl    = 10 * time.Second // 单次凭据验证超时
	reverifyExposureProbeTimeout = 5 * time.Second  // 单 URL 探测超时
)

// ReverifyBatchItem 单条复验结论
type ReverifyBatchItem struct {
	Id      string `json:"id"`      // vulnId（weakpass）或 jsfinder/dirscan 记录 ID（exposure）
	Kind    string `json:"kind"`    // exposure 回写集合归属: jsfinder / dirscan（weakpass 忽略）
	Outcome string `json:"outcome"` // weakpass: fixed/still_vuln/unreachable; exposure: resolved/verified/pending
}

// ReverifyBatchReq 批量复验结果上报请求
type ReverifyBatchReq struct {
	WorkspaceId string              `json:"workspaceId"`
	Kind        string              `json:"kind"` // weakpass / exposure
	Results     []ReverifyBatchItem `json:"results"`
}

// ReverifyBatchResp 批量复验结果上报响应
type ReverifyBatchResp struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Success bool   `json:"success"`
}

// executeReverifyWeakpassTask 执行弱口令持续复验任务（T3.3）
// 仅验证已记录的凭据组合（"确认是否已修复"），不做字典爆破。
// 判定规则：不可达 → unreachable（待确认，不误判修复）；凭据仍有效 → still_vuln；
// 明确认证拒绝（auth_reject）→ fixed；网络/协议错误 → unreachable（无法判定，不误判修复）。
func (w *Worker) executeReverifyWeakpassTask(ctx context.Context, task *scheduler.TaskInfo, taskConfig map[string]interface{}, startTime time.Time) {
	// panic 恢复：兜底上报，避免复验周期卡住
	defer func() {
		if r := recover(); r != nil {
			w.taskLog(task.TaskId, LevelError, "Weakpass reverify task panic recovered: %v, stack: %s", r, string(getStackTrace()))
			_ = w.reportReverifyBatch(ctx, task, taskConfig, "weakpass", nil)
		}
	}()

	wsId, _ := taskConfig["workspaceId"].(string)
	if wsId == "" {
		wsId = task.WorkspaceId
	}
	if wsId == "" {
		wsId = "default"
	}

	var targets []scheduler.ReverifyWeakpassTarget
	if err := decodeTargets(taskConfig["targets"], &targets); err != nil || len(targets) == 0 {
		w.taskLog(task.TaskId, LevelError, "[%s] 弱口令复验失败: 目标列表为空或解析失败: %v", task.TaskId, err)
		w.updateTaskStatus(ctx, task.TaskId, scheduler.TaskStatusFailure, "复验目标列表为空或解析失败")
		return
	}
	w.taskLog(task.TaskId, LevelInfo, "[%s] 收到弱口令复验任务, 目标数: %d", task.TaskId, len(targets))

	results := make([]ReverifyBatchItem, 0, len(targets))
	for _, t := range targets {
		plugin := brute.GetPlugin(t.Service)
		if plugin == nil {
			w.taskLog(task.TaskId, LevelInfo, "[%s] 未知服务 %q，跳过 %s", task.TaskId, t.Service, t.VulnId)
			continue
		}
		port := t.Port
		if port == 0 {
			port = brute.ServicePortMap[t.Service]
		}

		// 1) 可达性探测：不可达 → 待确认，不误判为已修复
		probeCtx, probeCancel := context.WithTimeout(ctx, reverifyWeakpassProbeTimeout)
		reachable := plugin.Probe(probeCtx, t.Host, port)
		probeCancel()
		if !reachable {
			results = append(results, ReverifyBatchItem{Id: t.VulnId, Outcome: "unreachable"})
			w.taskLog(task.TaskId, LevelInfo, "[%s] %s:%d 不可达，标记待确认", task.TaskId, t.Host, port)
			continue
		}

		// 2) 仅验证已知凭据（不爆破）
		verifyCtx, verifyCancel := context.WithTimeout(ctx, reverifyWeakpassVerifyTtl)
		result := plugin.Brute(verifyCtx, t.Host, port, []string{t.Username}, []string{t.Password}, int(reverifyWeakpassVerifyTtl.Seconds()))
		verifyCancel()

		switch {
		case result.Success:
			results = append(results, ReverifyBatchItem{Id: t.VulnId, Outcome: "still_vuln"})
		case result.ErrorType == "auth_reject":
			results = append(results, ReverifyBatchItem{Id: t.VulnId, Outcome: "fixed"})
		default:
			results = append(results, ReverifyBatchItem{Id: t.VulnId, Outcome: "unreachable"})
			w.taskLog(task.TaskId, LevelInfo, "[%s] %s:%d verify inconclusive (errorType=%q msg=%q), marking unreachable",
				task.TaskId, t.Host, port, result.ErrorType, result.Message)
		}
	}

	duration := time.Since(startTime).Seconds()
	w.taskLog(task.TaskId, LevelInfo, "[%s] 弱口令复验完成, 共%d个结论, 耗时%.1fs", task.TaskId, len(results), duration)
	if err := w.reportReverifyBatch(ctx, task, taskConfig, "weakpass", results); err != nil {
		w.taskLog(task.TaskId, LevelError, "[%s] 弱口令复验结果上报失败: %v", task.TaskId, err)
		w.updateTaskStatus(ctx, task.TaskId, scheduler.TaskStatusFailure, "复验结果上报失败: "+err.Error())
		return
	}
	// 必须置终态：否则任务滞留 cscan:task:processing，被孤儿恢复重排队重复执行
	w.updateTaskStatus(ctx, task.TaskId, scheduler.TaskStatusSuccess,
		fmt.Sprintf("弱口令复验完成，共 %d 个结论", len(results)))
}

// executeReverifyExposureTask 执行敏感信息持续复验任务（T3.4）
// 严格区分"目标不可达（连不上）"与"已不可访问（404/410）"：不可达仅标记 pending，不误判为已修复。
// 内容特征兜底：URL 仍返回 200（软 404）时，若原 ExtractedResults 敏感内容已不在响应体中 → resolved（内容消失）。
func (w *Worker) executeReverifyExposureTask(ctx context.Context, task *scheduler.TaskInfo, taskConfig map[string]interface{}, startTime time.Time) {
	defer func() {
		if r := recover(); r != nil {
			w.taskLog(task.TaskId, LevelError, "Exposure reverify task panic recovered: %v, stack: %s", r, string(getStackTrace()))
			_ = w.reportReverifyBatch(ctx, task, taskConfig, "exposure", nil)
		}
	}()

	wsId, _ := taskConfig["workspaceId"].(string)
	if wsId == "" {
		wsId = task.WorkspaceId
	}
	if wsId == "" {
		wsId = "default"
	}

	var targets []scheduler.ReverifyExposureTarget
	if err := decodeTargets(taskConfig["targets"], &targets); err != nil || len(targets) == 0 {
		w.taskLog(task.TaskId, LevelError, "[%s] 敏感信息复验失败: 目标列表为空或解析失败: %v", task.TaskId, err)
		w.updateTaskStatus(ctx, task.TaskId, scheduler.TaskStatusFailure, "复验目标列表为空或解析失败")
		return
	}
	w.taskLog(task.TaskId, LevelInfo, "[%s] 收到敏感信息复验任务, 目标数: %d", task.TaskId, len(targets))

	results := make([]ReverifyBatchItem, 0, len(targets))
	for _, t := range targets {
		results = append(results, ReverifyBatchItem{
			Id:      t.Id,
			Kind:    t.Kind,
			Outcome: probeExposure(ctx, t, reverifyExposureProbeTimeout),
		})
	}

	duration := time.Since(startTime).Seconds()
	w.taskLog(task.TaskId, LevelInfo, "[%s] 敏感信息复验完成, 共%d个结论, 耗时%.1fs", task.TaskId, len(results), duration)
	if err := w.reportReverifyBatch(ctx, task, taskConfig, "exposure", results); err != nil {
		w.taskLog(task.TaskId, LevelError, "[%s] 敏感信息复验结果上报失败: %v", task.TaskId, err)
		w.updateTaskStatus(ctx, task.TaskId, scheduler.TaskStatusFailure, "复验结果上报失败: "+err.Error())
		return
	}
	// 必须置终态：否则任务滞留 cscan:task:processing，被孤儿恢复重排队重复执行
	w.updateTaskStatus(ctx, task.TaskId, scheduler.TaskStatusSuccess,
		fmt.Sprintf("敏感信息复验完成，共 %d 个结论", len(results)))
}

// reportReverifyBatch 将复验结论批量上报给 API 服务
func (w *Worker) reportReverifyBatch(ctx context.Context, task *scheduler.TaskInfo, taskConfig map[string]interface{}, kind string, results []ReverifyBatchItem) error {
	wsId, _ := taskConfig["workspaceId"].(string)
	if wsId == "" {
		wsId = task.WorkspaceId
	}
	if wsId == "" {
		wsId = "default"
	}
	resp, err := w.httpClient.SaveReverifyBatch(ctx, &ReverifyBatchReq{
		WorkspaceId: wsId,
		Kind:        kind,
		Results:     results,
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Msg)
	}
	return nil
}

// decodeTargets 将任务配置中的 targets 数组解码为强类型列表
func decodeTargets(v interface{}, out interface{}) error {
	if v == nil {
		return fmt.Errorf("targets missing")
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

// probeExposure 探测敏感 URL 的可访问性并分类（从 scheduler 迁移，纯函数）。
// 返回: "resolved"（已修复/不可访问）/ "verified"（仍暴露）/ "pending"（不可达，待确认）
func probeExposure(ctx context.Context, t scheduler.ReverifyExposureTarget, timeout time.Duration) string {
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, t.Url, nil)
	if err != nil {
		// URL 非法 → 视为不可达，待确认
		return "pending"
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; CScan-Reverify/1.0)")

	resp, err := httpclient.Do(req)
	if err != nil {
		// 连接拒绝 / 超时 / DNS 失败 / TLS 错误：目标不可达，仅标记待确认，不误判为已修复
		return "pending"
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotFound, http.StatusGone: // 404 / 410：资源已移除 → 已修复
		return "resolved"
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout: // 5xx：暂态错误，不判定为已修复
		return "pending"
	case http.StatusForbidden, http.StatusUnauthorized, http.StatusMethodNotAllowed: // 403/401/405：仍存在但访问受限 → 仍暴露
		return "verified"
	}

	// 2xx / 3xx（或其它 4xx）：可达。内容特征兜底：
	// 若原泄露内容（ExtractedResults）非空且当前响应体中已不再出现 → 视为已修复（软 404 / 内容消失）。
	if len(t.Extracted) > 0 {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
		if readErr == nil {
			bodyLower := strings.ToLower(string(body))
			stillLeaking := false
			for _, ex := range t.Extracted {
				if ex == "" {
					continue
				}
				if strings.Contains(bodyLower, strings.ToLower(ex)) {
					stillLeaking = true
					break
				}
			}
			if !stillLeaking {
				return "resolved"
			}
		}
	}
	return "verified"
}
