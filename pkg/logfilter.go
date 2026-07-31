package pkg

import (
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
)

// filteredLogWriter 包装 logx.Writer，过滤掉指定的高频轮询 access log
// 用于抑制 Worker 轮询接口（task/check、heartbeat、control）在空闲时产生的大量日志脏数据
type filteredLogWriter struct {
	delegate     logx.Writer
	filterSubstr []string // 日志消息包含任一子串则被过滤（仅过滤 Info/Stat 级别）
}

// NewFilteredLogWriter 创建一个过滤日志写入器
// delegate: 底层实际写入的 logx.Writer
// filterSubstr: 需要过滤的日志消息子串列表
func NewFilteredLogWriter(delegate logx.Writer, filterSubstr []string) logx.Writer {
	return &filteredLogWriter{
		delegate:     delegate,
		filterSubstr: filterSubstr,
	}
}

func (f *filteredLogWriter) shouldFilter(v any) bool {
	msg, ok := v.(string)
	if !ok {
		return false
	}
	for _, substr := range f.filterSubstr {
		if strings.Contains(msg, substr) {
			return true
		}
	}
	return false
}

func (f *filteredLogWriter) Alert(v any) {
	f.delegate.Alert(v)
}

func (f *filteredLogWriter) Close() error {
	return f.delegate.Close()
}

func (f *filteredLogWriter) Debug(v any, fields ...logx.LogField) {
	f.delegate.Debug(v, fields...)
}

func (f *filteredLogWriter) Error(v any, fields ...logx.LogField) {
	f.delegate.Error(v, fields...)
}

func (f *filteredLogWriter) Info(v any, fields ...logx.LogField) {
	if f.shouldFilter(v) {
		return
	}
	f.delegate.Info(v, fields...)
}

func (f *filteredLogWriter) Severe(v any) {
	f.delegate.Severe(v)
}

func (f *filteredLogWriter) Slow(v any, fields ...logx.LogField) {
	f.delegate.Slow(v, fields...)
}

func (f *filteredLogWriter) Stack(v any) {
	f.delegate.Stack(v)
}

func (f *filteredLogWriter) Stat(v any, fields ...logx.LogField) {
	if f.shouldFilter(v) {
		return
	}
	f.delegate.Stat(v, fields...)
}
