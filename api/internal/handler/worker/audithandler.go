package worker

import (
	"fmt"
	"net/http"
	"time"

	"cscan/api/internal/svc"
	"cscan/internal/model"
	"cscan/pkg/response"

	"github.com/zeromicro/go-zero/core/logx"
)

// WorkerAuditLogHandler 获取审计日志
// GET /api/v1/worker/console/audit
func WorkerAuditLogHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 解析查询参数
		workerName := r.URL.Query().Get("workerName")
		logType := r.URL.Query().Get("type")
		userId := r.URL.Query().Get("userId")
		username := r.URL.Query().Get("username")
		startTimeStr := r.URL.Query().Get("startTime")
		endTimeStr := r.URL.Query().Get("endTime")

		// 解析分页参数
		page := 1
		pageSize := 20
		if p := r.URL.Query().Get("page"); p != "" {
			fmt.Sscanf(p, "%d", &page)
		}
		if ps := r.URL.Query().Get("pageSize"); ps != "" {
			fmt.Sscanf(ps, "%d", &pageSize)
		}

		// 限制pageSize
		if pageSize > 100 {
			pageSize = 100
		}
		if pageSize < 1 {
			pageSize = 20
		}
		if page < 1 {
			page = 1
		}

		// 构建过滤条件
		filter := model.AuditLogFilter{
			WorkerName: workerName,
			UserId:     userId,
			Username:   username,
		}

		if logType != "" {
			filter.Type = model.AuditLogType(logType)
		}

		// 解析时间范围
		if startTimeStr != "" {
			if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
				filter.StartTime = t
			}
		}
		if endTimeStr != "" {
			if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
				filter.EndTime = t
			}
		}

		ctx := r.Context()

		// 查询审计日志
		logs, total, err := svcCtx.AuditLogModel.Search(ctx, filter, page, pageSize)
		if err != nil {
			response.ErrorWithCode(w, http.StatusInternalServerError, "failed to get audit logs: "+err.Error())
			return
		}

		response.Success(w, map[string]interface{}{
			"list":     logs,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		})
	}
}

// WorkerAuditLogClearHandler 清空审计日志
// DELETE /api/v1/worker/console/audit
func WorkerAuditLogClearHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workerName := r.URL.Query().Get("workerName")
		ctx := r.Context()

		var deleted int64
		var err error

		if workerName != "" {
			// 清空指定 Worker 的审计日志
			deleted, err = svcCtx.AuditLogModel.ClearByWorker(ctx, workerName)
		} else {
			// 清空所有审计日志
			deleted, err = svcCtx.AuditLogModel.ClearAll(ctx)
		}

		if err != nil {
			response.ErrorWithCode(w, http.StatusInternalServerError, "failed to clear audit logs: "+err.Error())
			return
		}

		logx.Infof("Audit logs cleared: workerName=%s, deleted=%d", workerName, deleted)

		response.Success(w, map[string]interface{}{
			"deleted": deleted,
			"msg":     fmt.Sprintf("已清空 %d 条审计日志", deleted),
		})
	}
}
