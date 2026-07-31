package container

import (
	"net/http"
	"strconv"

	"cscan/api/internal/svc"
	"cscan/pkg/response"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// ContainerLogDatesHandler 返回有日志的日期列表(降序)
// GET /api/v1/container/logs/dates
func ContainerLogDatesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svcCtx.LogCollector == nil {
			response.ErrorWithCode(w, 503, "log collector unavailable")
			return
		}
		dates := svcCtx.LogCollector.ListDates()
		httpx.OkJson(w, map[string]interface{}{
			"code":  0,
			"msg":   "success",
			"dates": dates,
		})
	}
}

// ContainerLogFilesHandler 返回某天有日志的容器文件列表
// GET /api/v1/container/logs/files?date=2026-07-28
func ContainerLogFilesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svcCtx.LogCollector == nil {
			response.ErrorWithCode(w, 503, "log collector unavailable")
			return
		}
		date := r.URL.Query().Get("date")
		if len(date) != 10 {
			response.ParamError(w, "date param required (YYYY-MM-DD)")
			return
		}
		files := svcCtx.LogCollector.ListContainersForDate(date)
		httpx.OkJson(w, map[string]interface{}{
			"code":  0,
			"msg":   "success",
			"files": files,
		})
	}
}

// ContainerLogHistoryHandler 读取指定日期+容器的历史日志
// GET /api/v1/container/logs/history?date=2026-07-28&name=cscan_api&tail=500
func ContainerLogHistoryHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svcCtx.LogCollector == nil {
			response.ErrorWithCode(w, 503, "log collector unavailable")
			return
		}
		q := r.URL.Query()
		date := q.Get("date")
		name := q.Get("name")
		if len(date) != 10 || name == "" {
			response.ParamError(w, "date and name params required")
			return
		}
		tail := 500
		if ts := q.Get("tail"); ts != "" {
			if v, err := strconv.Atoi(ts); err == nil && v > 0 {
				tail = v
			}
		}

		lines, total, err := svcCtx.LogCollector.ReadLog(date, name, tail)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, map[string]interface{}{
			"code":       0,
			"msg":        "success",
			"lines":      lines,
			"total":      total,
			"returned":   len(lines),
			"truncated":  total > len(lines),
		})
	}
}
