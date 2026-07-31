package worker

import (
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// 日志级别常量
const (
	LevelDebug = "DEBUG"
	LevelInfo  = "INFO"
	LevelWarn  = "WARN"
	LevelError = "ERROR"
)

// LogEntry 日志条目（统一结构）
type LogEntry struct {
	Timestamp  string `json:"timestamp"`
	Level      string `json:"level"`
	WorkerName string `json:"workerName"`
	TaskId     string `json:"taskId,omitempty"`
	Message    string `json:"message"`
}

// Logger 统一日志接口
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}

// ==================== Local Logger (No Redis) ====================

// WorkerLogger Worker 日志记录器（本地输出）
type WorkerLogger struct {
	workerName string
}

// NewWorkerLoggerLocal 创建本地日志记录器
func NewWorkerLoggerLocal(workerName string) *WorkerLogger {
	return &WorkerLogger{
		workerName: workerName,
	}
}

// log 内部日志方法，输出到控制台
func (l *WorkerLogger) log(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Local().Format("2006-01-02 15:04:05")

	// 输出到控制台
	logx.Infof("%s [%s] [%s] %s", timestamp, level, l.workerName, msg)
}

func (l *WorkerLogger) Debug(format string, args ...interface{}) {
	l.log(LevelDebug, format, args...)
}

func (l *WorkerLogger) Info(format string, args ...interface{}) {
	l.log(LevelInfo, format, args...)
}

func (l *WorkerLogger) Warn(format string, args ...interface{}) {
	l.log(LevelWarn, format, args...)
}

func (l *WorkerLogger) Error(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
}

// TaskLogger 任务日志记录器（本地输出）
type TaskLogger struct {
	workerName string
	taskId     string
}

// NewTaskLoggerLocal 创建本地任务日志记录器
func NewTaskLoggerLocal(workerName, taskId string) *TaskLogger {
	return &TaskLogger{
		workerName: workerName,
		taskId:     taskId,
	}
}

// log 内部日志方法，输出到控制台
func (l *TaskLogger) log(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Local().Format("2006-01-02 15:04:05")

	// 输出到控制台
	logx.Infof("%s [%s] [%s] [Task:%s] %s", timestamp, level, l.workerName, l.taskId, msg)
}

func (l *TaskLogger) Debug(format string, args ...interface{}) {
	l.log(LevelDebug, format, args...)
}

func (l *TaskLogger) Info(format string, args ...interface{}) {
	l.log(LevelInfo, format, args...)
}

func (l *TaskLogger) Warn(format string, args ...interface{}) {
	l.log(LevelWarn, format, args...)
}

func (l *TaskLogger) Error(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
}

// ==================== File-based Logger (本地文件 + 游标同步) ====================

// WorkerLoggerWS Worker 日志记录器
// 日志先写入本地文件（事实源），通过游标同步机制传输到 API 端
// 不再使用内存缓冲区，断连时日志持续写入文件，重连后从游标续传
type WorkerLoggerWS struct {
	workerName string
	fileLogger *FileLogger
}

// NewWorkerLoggerWS 创建日志记录器
// wsClient 参数保留兼容性，但日志不再通过 wsClient.SendLogImmediate 发送
// 实际同步由 LogSyncManager 通过游标协议完成
func NewWorkerLoggerWS(workerName string, wsClient *WorkerWSClient) *WorkerLoggerWS {
	return &WorkerLoggerWS{
		workerName: workerName,
		fileLogger: globalFileLogger,
	}
}

// log 内部日志方法
func (l *WorkerLoggerWS) log(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Local().Format("2006-01-02 15:04:05")

	// 输出到控制台
	logx.Infof("%s [%s] [%s] %s", timestamp, level, l.workerName, msg)

	// DEBUG 级别日志仅本地输出，不写入文件（避免指纹探测等大量 DEBUG 日志）
	if level == LevelDebug {
		return
	}

	// 写入本地文件（事实源），游标同步机制会自动将其传输到 API
	if l.fileLogger != nil {
		l.fileLogger.Write(level, "", msg)
	}
}

func (l *WorkerLoggerWS) Debug(format string, args ...interface{}) {
	l.log(LevelDebug, format, args...)
}

func (l *WorkerLoggerWS) Info(format string, args ...interface{}) {
	l.log(LevelInfo, format, args...)
}

func (l *WorkerLoggerWS) Warn(format string, args ...interface{}) {
	l.log(LevelWarn, format, args...)
}

func (l *WorkerLoggerWS) Error(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
}

// TaskLoggerWS 任务日志记录器
// 日志先写入本地文件（事实源），通过游标同步机制传输到 API 端
// 不再使用内存缓冲区，断连时日志持续写入文件，重连后从游标续传
type TaskLoggerWS struct {
	workerName string
	taskId     string
	fileLogger *FileLogger
}

// NewTaskLoggerWS 创建任务日志记录器
func NewTaskLoggerWS(workerName, taskId string, wsClient *WorkerWSClient) *TaskLoggerWS {
	return &TaskLoggerWS{
		workerName: workerName,
		taskId:     taskId,
		fileLogger: globalFileLogger,
	}
}

// log 内部日志方法
func (l *TaskLoggerWS) log(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Local().Format("2006-01-02 15:04:05")

	// 输出到控制台
	logx.Infof("%s [%s] [%s] [Task:%s] %s", timestamp, level, l.workerName, l.taskId, msg)

	// DEBUG 级别日志仅本地输出，不写入文件（避免指纹探测等大量 DEBUG 日志）
	if level == LevelDebug {
		return
	}

	// 写入本地文件（事实源），游标同步机制会自动将其传输到 API
	if l.fileLogger != nil {
		l.fileLogger.Write(level, l.taskId, msg)
	}
}

func (l *TaskLoggerWS) Debug(format string, args ...interface{}) {
	l.log(LevelDebug, format, args...)
}

func (l *TaskLoggerWS) Info(format string, args ...interface{}) {
	l.log(LevelInfo, format, args...)
}

func (l *TaskLoggerWS) Warn(format string, args ...interface{}) {
	l.log(LevelWarn, format, args...)
}

func (l *TaskLoggerWS) Error(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
}

// ==================== 全局 FileLogger 实例 ====================

// globalFileLogger 全局文件日志器，由 Worker 初始化时设置
// 所有 Logger 实例共享同一个 FileLogger，日志统一写入同一文件
var globalFileLogger *FileLogger

// InitGlobalFileLogger 初始化全局文件日志器
func InitGlobalFileLogger(logDir, workerName string) {
	globalFileLogger = NewFileLogger(logDir, workerName)
}

// GetGlobalFileLogger 获取全局文件日志器
func GetGlobalFileLogger() *FileLogger {
	return globalFileLogger
}

// UpdateGlobalFileLoggerWorkerName 更新全局文件日志器的 worker 名称（rename 后调用）
func UpdateGlobalFileLoggerWorkerName(name string) {
	if globalFileLogger != nil {
		globalFileLogger.SetWorkerName(name)
	}
}
