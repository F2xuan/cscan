package worker

import (
	"github.com/zeromicro/go-zero/core/logx"
)

// globalCursorManager 全局游标管理器，由 Worker 初始化时设置
var globalCursorManager *CursorManager

// InitLogSync 初始化日志同步组件
// 在 Worker 启动时调用，创建文件日志器、游标管理器，并注册到 WSClient
func InitLogSync(logDir, workerName string, wsClient *WorkerWSClient) {
	// 初始化全局文件日志器
	InitGlobalFileLogger(logDir, workerName)

	// 初始化全局游标管理器
	globalCursorManager = NewCursorManager(logDir)

	// 创建日志同步读取器
	syncReader := NewLogSyncReader(logDir)

	// 注册日志同步请求处理函数
	// 修复 H-3：游标真源在 Worker 本地（持久化到磁盘），API 传空/零值游标时
	// Worker 使用本地持久化游标作为起点，避免重连后从头回灌全部历史日志。
	wsClient.SetLogSyncHandler(func(req WSLogSyncReqPayload) WSLogSyncRespPayload {
		cursor := SyncCursor{
			Filename: req.Filename,
			Offset:   req.Offset,
		}
		// 如果 API 端游标为空（新连接或游标丢失），回退到 Worker 本地持久化游标
		if cursor.Filename == "" && globalCursorManager != nil {
			local := globalCursorManager.Get()
			if local.Filename != "" {
				cursor = local
				logx.Infof("[LogSync] API cursor empty, using local persisted cursor: file=%s offset=%d",
					local.Filename, local.Offset)
			}
		}
		result, err := syncReader.ReadFrom(cursor, 500)
		if err != nil {
			logx.Infof("[LogSync] Failed to read logs from cursor: %v", err)
			return WSLogSyncRespPayload{
				Filename: cursor.Filename,
				Logs:     []FileLogEntry{},
			}
		}

		return WSLogSyncRespPayload{
			Filename:  result.Filename,
			Logs:      result.Logs,
			NewOffset: result.NewOffset,
			HasMore:   result.HasMore,
			NextFile:  result.NextFile,
		}
	})

	logx.Infof("[LogSync] Initialized: logDir=%s, worker=%s", logDir, workerName)
}
