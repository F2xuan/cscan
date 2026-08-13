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
	Id      string `json:"id"`
	Kind    string `json:"kind"`
	Outcome string `json:"outcome"`
}

// ReverifyBatchReq 批量复验结果上报请求
type ReverifyBatchReq struct {
	Kind    string              `json:"kind"`
	Results []ReverifyBatchItem `json:"results"`
}

// ReverifyBatchResp 批量复验结果上报响应
type ReverifyBatchResp struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Success bool   `json:"success"`
}

// executeReverifyWeakpassTask 执行弱口令持续复验任务（T3.3）
func (w *Worker) executeReverifyWeakpassTask(ctx context.Context, task *scheduler.TaskInfo, taskConfig map[string]interface{}, startTime time.Time) {
	defer func() {
		if r := recover(); r != nil {
			w.taskLog(task.TaskId, LevelError, "Weakpass reverify task panic recovered: %v, stack: %s", r, string(getStackTrace()))
			_ = w.reportReverifyBatch(ctx, task, taskConfig, "weakpass", nil)
		}
	}()

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

		probeCtx, probeCancel := context.WithTimeout(ctx, reverifyWeakpassProbeTimeout)
		reachable := plugin.Probe(probeCtx, t.Host, port)
		probeCancel()
		if !reachable {
			results = append(results, ReverifyBatchItem{Id: t.VulnId, Outcome: "unreachable"})
			w.taskLog(task.TaskId, LevelInfo, "[%s] %s:%d 不可达，标记待确认", task.TaskId, t.Host, port)
			continue
		}

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
	w.updateTaskStatus(ctx, task.TaskId, scheduler.TaskStatusSuccess,
		fmt.Sprintf("弱口令复验完成，共 %d 个结论", len(results)))
}

// executeReverifyExposureTask 执行敏感信息持续复验任务（T3.4）
func (w *Worker) executeReverifyExposureTask(ctx context.Context, task *scheduler.TaskInfo, taskConfig map[string]interface{}, startTime time.Time) {
	defer func() {
		if r := recover(); r != nil {
			w.taskLog(task.TaskId, LevelError, "Exposure reverify task panic recovered: %v, stack: %s", r, string(getStackTrace()))
			_ = w.reportReverifyBatch(ctx, task, taskConfig, "exposure", nil)
		}
	}()

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
	w.updateTaskStatus(ctx, task.TaskId, scheduler.TaskStatusSuccess,
		fmt.Sprintf("敏感信息复验完成，共 %d 个结论", len(results)))
}

// reportReverifyBatch 将复验结论批量上报给 API 服务
func (w *Worker) reportReverifyBatch(ctx context.Context, task *scheduler.TaskInfo, taskConfig map[string]interface{}, kind string, results []ReverifyBatchItem) error {
	resp, err := w.httpClient.SaveReverifyBatch(ctx, &ReverifyBatchReq{
		Kind:    kind,
		Results: results,
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

// probeExposure 探测敏感 URL 的可访问性并分类
func probeExposure(ctx context.Context, t scheduler.ReverifyExposureTarget, timeout time.Duration) string {
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, t.Url, nil)
	if err != nil {
		return "pending"
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; CScan-Reverify/1.0)")

	resp, err := httpclient.Do(req)
	if err != nil {
		return "pending"
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotFound, http.StatusGone:
		return "resolved"
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return "pending"
	case http.StatusForbidden, http.StatusUnauthorized, http.StatusMethodNotAllowed:
		return "verified"
	}

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
