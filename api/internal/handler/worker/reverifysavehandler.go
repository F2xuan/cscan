package worker

import (
	"encoding/json"
	"net/http"
	"time"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// WorkerVulReverifyHandler Worker 复测完成后回传单条漏洞复验结果
// POST /api/v1/worker/task/vul/reverify  （worker 专用，需 Install Key 认证）
func WorkerVulReverifyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.WorkerVulReverifyReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.OkJson(w, &types.WorkerVulReverifyResp{Code: 400, Msg: "参数解析失败"})
			return
		}
		if req.WorkspaceId == "" || req.VulnId == "" || req.Conclusion == "" {
			httpx.OkJson(w, &types.WorkerVulReverifyResp{Code: 400, Msg: "workspaceId/vulnId/conclusion 不能为空"})
			return
		}

		// 校验结论取值
		switch req.Conclusion {
		case model.ReverifyConclusionFixed,
			model.ReverifyConclusionStillVuln,
			model.ReverifyConclusionUnreachable,
			model.ReverifyConclusionReachableUntested:
		default:
			httpx.OkJson(w, &types.WorkerVulReverifyResp{Code: 400, Msg: "未知复验结论: " + req.Conclusion})
			return
		}

		reverifyAt := time.Now()
		if req.ReverifyAt != "" {
			if t, perr := time.Parse(time.RFC3339, req.ReverifyAt); perr == nil {
				reverifyAt = t
			}
		}

		result := &model.VulReverifyResult{
			Conclusion: req.Conclusion,
			Reviewer:   req.Reviewer,
			Message:    req.Message,
			ReverifyAt: reverifyAt,
		}

		vulModel := svcCtx.GetVulModel(req.WorkspaceId)
		if err := vulModel.ApplyReverifyResult(r.Context(), req.VulnId, result); err != nil {
			logx.Errorf("[WorkerVulReverify] ApplyReverifyResult failed for vuln %s: %v", req.VulnId, err)
			httpx.OkJson(w, &types.WorkerVulReverifyResp{Code: 500, Msg: err.Error()})
			return
		}

		httpx.OkJson(w, &types.WorkerVulReverifyResp{Code: 0, Msg: "success", Success: true})
	}
}
