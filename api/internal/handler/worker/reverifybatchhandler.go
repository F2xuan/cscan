package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"
	"cscan/internal/scheduler"
	"cscan/pkg/notify"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// WorkerReverifyBatchHandler Worker 持续复验完成后回传批量结果（T3.3 弱口令 / T3.4 敏感信息）
// POST /api/v1/worker/reverify/result  （worker 专用，需 Install Key 认证）
//
// 状态流转（与原 Scheduler 侧逻辑一致）：
//   - weakpass: fixed → MarkFixed；still_vuln → MarkReverified；unreachable → MarkVerifyUnreachable
//   - exposure: resolved → MarkReverify(resolved)；verified → MarkReverify(verified)；pending → MarkReverify(pending)
//     处理完成后回写 NextRunTime 并按修复数量发送通知。
func WorkerReverifyBatchHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.WorkerReverifyBatchReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.OkJson(w, &types.WorkerReverifyBatchResp{Code: 400, Msg: "参数解析失败"})
			return
		}
		wsId := "default"
		if req.Kind == "" || len(req.Results) == 0 {
			httpx.OkJson(w, &types.WorkerReverifyBatchResp{Code: 400, Msg: "kind/results 不能为空"})
			return
		}

		switch req.Kind {
		case "weakpass":
			applyWeakpassReverify(r.Context(), svcCtx, wsId, req.Results)
		case "exposure":
			applyExposureReverify(r.Context(), svcCtx, wsId, req.Results)
		default:
			httpx.OkJson(w, &types.WorkerReverifyBatchResp{Code: 400, Msg: "未知复验类型: " + req.Kind})
			return
		}

		httpx.OkJson(w, &types.WorkerReverifyBatchResp{Code: 0, Msg: "success", Success: true})
	}
}

// applyWeakpassReverify 弱口令复验状态流转 + 运行状态回写 + 修复通知
func applyWeakpassReverify(ctx context.Context, svcCtx *svc.ServiceContext, wsId string, items []types.WorkerReverifyBatchItem) {
	var fixedIDs, verifiedIDs, pendingIDs []string
	for _, it := range items {
		switch it.Outcome {
		case "fixed":
			fixedIDs = append(fixedIDs, it.Id)
		case "still_vuln":
			verifiedIDs = append(verifiedIDs, it.Id)
		case "unreachable":
			pendingIDs = append(pendingIDs, it.Id)
		default:
			logx.Errorf("[WorkerReverifyBatch] workspace=%s 未知 weakpass 结论 %q, 忽略", wsId, it.Outcome)
		}
	}

	vulModel := svcCtx.GetVulModel()
	if len(verifiedIDs) > 0 {
		if _, e := vulModel.MarkReverified(ctx, verifiedIDs); e != nil {
			logx.Errorf("[WorkerReverifyBatch] workspace=%s MarkReverified failed: %v", wsId, e)
		}
	}
	if len(pendingIDs) > 0 {
		if _, e := vulModel.MarkVerifyUnreachable(ctx, pendingIDs); e != nil {
			logx.Errorf("[WorkerReverifyBatch] workspace=%s MarkVerifyUnreachable failed: %v", wsId, e)
		}
	}
	if len(fixedIDs) > 0 {
		if _, e := vulModel.MarkFixed(ctx, fixedIDs, model.VulFixSourceRescan); e != nil {
			logx.Errorf("[WorkerReverifyBatch] workspace=%s MarkFixed failed: %v", wsId, e)
		}
		// 失效漏洞统计缓存，使工作台安全评分即时反映复验修复
		svcCtx.QueryCache.Delete("vul_stat")
	}

	finishReverifyRun(ctx, svcCtx, wsId, "weakpass-reverify", "弱口令持续复验", len(items), len(fixedIDs))
}

// applyExposureReverify 敏感信息复验状态回写 + 运行状态回写 + 修复通知
func applyExposureReverify(ctx context.Context, svcCtx *svc.ServiceContext, wsId string, items []types.WorkerReverifyBatchItem) {
	var resolved, verified, pending []types.WorkerReverifyBatchItem
	for _, it := range items {
		switch it.Outcome {
		case "resolved":
			resolved = append(resolved, it)
		case "verified":
			verified = append(verified, it)
		case "pending":
			pending = append(pending, it)
		default:
			logx.Errorf("[WorkerReverifyBatch] workspace=%s 未知 exposure 结论 %q, 忽略", wsId, it.Outcome)
		}
	}

	applyExposureOutcome(ctx, svcCtx, wsId, resolved, "resolved")
	applyExposureOutcome(ctx, svcCtx, wsId, verified, "verified")
	applyExposureOutcome(ctx, svcCtx, wsId, pending, "pending")

	finishReverifyRun(ctx, svcCtx, wsId, "exposure-reverify", "敏感信息持续复验", len(items), len(resolved))
}

// applyExposureOutcome 将分类结果回写到对应集合的记录（按上报 Kind 区分 jsfinder / dirscan）
func applyExposureOutcome(ctx context.Context, svcCtx *svc.ServiceContext, wsId string, items []types.WorkerReverifyBatchItem, status string) {
	if len(items) == 0 {
		return
	}
	var jsIds, dirIds []string
	for _, it := range items {
		if it.Kind == "jsfinder" {
			jsIds = append(jsIds, it.Id)
		} else {
			dirIds = append(dirIds, it.Id)
		}
	}
	now := time.Now()
	pending := status == "pending"

	if len(jsIds) > 0 {
		jsModel := model.NewJSFinderResultModel(svcCtx.MongoDB)
		if err := jsModel.MarkReverify(ctx, jsIds, status, now, pending); err != nil {
			logx.Errorf("[WorkerReverifyBatch] workspace=%s mark jsfinder reverify failed: %v", wsId, err)
		}
	}
	if len(dirIds) > 0 {
		dirModel := model.NewDirScanResultModel(svcCtx.MongoDB)
		if err := dirModel.MarkReverify(ctx, dirIds, status, now, pending); err != nil {
			logx.Errorf("[WorkerReverifyBatch] workspace=%s mark dirscan reverify failed: %v", wsId, err)
		}
	}
}

// finishReverifyRun 回写复验运行状态（NextRunTime 按配置 CronSpec 推进）并发送修复通知
func finishReverifyRun(ctx context.Context, svcCtx *svc.ServiceContext, wsId, taskId, taskName string, total, fixedCount int) {
	now := time.Now()
	configModel := model.NewReverifyConfigModel(svcCtx.MongoDB)
	if cfg, err := configModel.GetByWorkspace(ctx, wsId); err == nil && cfg != nil {
		_ = configModel.UpdateRunState(ctx, wsId, now, "success", total, "", scheduler.NextReverifyRunTime(cfg.CronSpec, now))
	} else if err != nil {
		logx.Errorf("[WorkerReverifyBatch] workspace=%s 读取复验配置失败: %v", wsId, err)
	}

	if fixedCount > 0 {
		if err := svcCtx.SendReverifyNotify(ctx, &notify.NotifyResult{
			TaskId:      taskId,
			TaskName:    taskName,
			Status:      "SUCCESS",
			HighRiskInfo: &notify.HighRiskInfo{
				FixedVulCount: fixedCount,
			},
		}); err != nil {
			logx.Errorf("[WorkerReverifyBatch] workspace=%s 发送修复通知失败: %v", wsId, err)
		}
	}

	logx.Infof("[WorkerReverifyBatch] workspace=%s %s 复验完成: 共%d 已修复%d", wsId, taskName, total, fixedCount)
}
