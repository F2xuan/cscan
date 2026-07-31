package svc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// WorkerLogEntry Worker 日志条目（与 Worker 端 FileLogEntry 结构一致）
type WorkerLogEntry struct {
	Ts     string `json:"ts"`
	Level  string `json:"level"`
	Worker string `json:"worker"`
	TaskId string `json:"taskId,omitempty"`
	Msg    string `json:"msg"`
}

// WorkerLogWriter 将 Worker 同步来的日志写入文件
// 使用有界 channel + 单 goroutine flush，channel 满时阻塞（反压），不丢日志
// 修复 H-2：提供同步写入接口 SyncWriteBatch，写盘并 Flush 后返回，供 ACK 前等待使用
type WorkerLogWriter struct {
	logDir    string // log/worker
	logCh     chan WorkerLogEntry
	wg        sync.WaitGroup
	closeOnce sync.Once
	closeChan chan struct{}

	// 同步写入请求通道（handler 发来 → flushLoop 写盘后回复）
	syncReqCh chan *syncWriteReq
}

// syncWriteReq 同步写入请求
type syncWriteReq struct {
	entries []WorkerLogEntry
	respCh  chan error
}

const (
	workerLogChannelSize = 10000 // channel 上限，约 2MB 内存
)

// NewWorkerLogWriter 创建 Worker 日志写入器
func NewWorkerLogWriter(baseLogDir string) *WorkerLogWriter {
	logDir := filepath.Join(baseLogDir, "worker")
	_ = os.MkdirAll(logDir, 0o755)

	w := &WorkerLogWriter{
		logDir:     logDir,
		logCh:      make(chan WorkerLogEntry, workerLogChannelSize),
		closeChan:  make(chan struct{}),
		syncReqCh:  make(chan *syncWriteReq, 64),
	}

	w.wg.Add(1)
	go w.flushLoop()

	return w
}

// SyncWriteBatch 同步写入日志并 Flush，写盘成功后返回 nil。
// 修复 H-2：用于日志同步场景，API 必须等待日志真正落盘后才向 Worker 发送 ACK，
// 否则 API 崩溃时已 ACK 但未写盘的日志会永久丢失。
func (w *WorkerLogWriter) SyncWriteBatch(entries []WorkerLogEntry) error {
	req := &syncWriteReq{
		entries: entries,
		respCh:  make(chan error, 1),
	}
	select {
	case w.syncReqCh <- req:
	case <-w.closeChan:
		return fmt.Errorf("worker log writer closed")
	}
	select {
	case err := <-req.respCh:
		return err
	case <-w.closeChan:
		return fmt.Errorf("worker log writer closed")
	}
}

// Write 将日志条目写入 channel（阻塞式，channel 满时反压调用方）
func (w *WorkerLogWriter) Write(entry WorkerLogEntry) {
	select {
	case w.logCh <- entry:
	case <-w.closeChan:
	}
}

// WriteBatch 批量写入日志条目
func (w *WorkerLogWriter) WriteBatch(entries []WorkerLogEntry) {
	for _, e := range entries {
		select {
		case w.logCh <- e:
		case <-w.closeChan:
			return
		}
	}
}

// flushLoop 单 goroutine 从 channel 读取日志并写入文件
func (w *WorkerLogWriter) flushLoop() {
	defer w.wg.Done()

	type fileHandle struct {
		file   *os.File
		date   string
		worker string
	}
	var fh fileHandle

	closeFile := func() {
		if fh.file != nil {
			// 关闭前 Sync 确保数据落盘
			_ = fh.file.Sync()
			fh.file.Close()
			fh.file = nil
		}
	}
	defer closeFile()

	// writeEntry 写入单条日志到当前 fh（调用方已持有 fh 有效）
	writeEntry := func(entry WorkerLogEntry) error {
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		data = append(data, '\n')
		_, err = fh.file.Write(data)
		return err
	}

	// ensureFile 确保 fh 指向 entry 对应的正确文件
	ensureFile := func(entry WorkerLogEntry) error {
		dateStr := time.Now().Format("2006-01-02")
		if fh.date == dateStr && fh.file != nil && fh.worker == entry.Worker {
			return nil
		}
		closeFile()
		fh.date = dateStr
		fh.worker = entry.Worker
		dateDir := filepath.Join(w.logDir, dateStr)
		_ = os.MkdirAll(dateDir, 0o755)
		fpath := filepath.Join(dateDir, sanitizeWorkerName(entry.Worker)+".log")
		f, err := os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}
		fh.file = f
		return nil
	}

	for {
		select {
		case req := <-w.syncReqCh:
			// 同步写入：所有条目写盘 + Sync 后返回结果
			var firstErr error
			for _, entry := range req.entries {
				if err := ensureFile(entry); err != nil {
					if firstErr == nil {
						firstErr = err
					}
					continue
				}
				if err := writeEntry(entry); err != nil {
					if firstErr == nil {
						firstErr = err
					}
				}
			}
			// 强制 Sync 确保落盘
			if fh.file != nil {
				if err := fh.file.Sync(); err != nil && firstErr == nil {
					firstErr = err
				}
			}
			req.respCh <- firstErr

		case entry := <-w.logCh:
			if err := ensureFile(entry); err != nil {
				logx.Errorf("[WorkerLogWriter] Failed to open file for worker=%s: %v", entry.Worker, err)
				continue
			}
			if err := writeEntry(entry); err != nil {
				logx.Errorf("[WorkerLogWriter] Failed to write log: %v", err)
			}

		case <-w.closeChan:
			// 排空同步请求
			for {
				select {
				case req := <-w.syncReqCh:
					req.respCh <- fmt.Errorf("writer closed")
				default:
					goto drainDone
				}
			}
		drainDone:
			// 退出前 flush 剩余异步日志
			for {
				select {
				case entry := <-w.logCh:
					_ = ensureFile(entry)
					_ = writeEntry(entry)
				default:
					return
				}
			}
		}
	}
}

// sanitizeWorkerName 清理 worker 名中的路径分隔符，防止路径穿越
func sanitizeWorkerName(name string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", "..", "_", ":", "_")
	return replacer.Replace(name)
}

// dateRegexp 严格匹配 YYYY-MM-DD 格式
var dateRegexp = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// validateDate 校验日期格式，防止路径穿越
func validateDate(date string) bool {
	return dateRegexp.MatchString(date)
}

// writeToFile 直接写入文件（仅 flushLoop 退出时使用）
func (w *WorkerLogWriter) writeToFile(entry WorkerLogEntry) {
	dateStr := time.Now().Format("2006-01-02")
	dateDir := filepath.Join(w.logDir, dateStr)
	_ = os.MkdirAll(dateDir, 0o755)

	fpath := filepath.Join(dateDir, sanitizeWorkerName(entry.Worker)+".log")
	f, err := os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	data, _ := json.Marshal(entry)
	data = append(data, '\n')
	f.Write(data)
}

// Close 关闭写入器，flush 剩余日志
func (w *WorkerLogWriter) Close() {
	w.closeOnce.Do(func() {
		close(w.closeChan)
	})
	w.wg.Wait()
}

// GetLogDir 获取日志目录
func (w *WorkerLogWriter) GetLogDir() string {
	return w.logDir
}

// ==================== Worker 日志文件读取器 ====================

// WorkerLogReader 读取 Worker 日志文件
type WorkerLogReader struct {
	logDir string
}

// NewWorkerLogReader 创建 Worker 日志读取器
func NewWorkerLogReader(baseLogDir string) *WorkerLogReader {
	return &WorkerLogReader{
		logDir: filepath.Join(baseLogDir, "worker"),
	}
}

// ReadTail 读取指定 worker 在指定日期的最后 N 条日志
// date 为空时取最新日期，lines 为 0 时默认 500
func (r *WorkerLogReader) ReadTail(workerName, date string, lines int) ([]WorkerLogEntry, error) {
	if lines <= 0 {
		lines = 500
	}
	if lines > 10000 {
		lines = 10000
	}

	if date == "" {
		date = r.findLatestDate()
		if date == "" {
			return []WorkerLogEntry{}, nil
		}
	}

	// 安全校验：防止路径穿越
	if !validateDate(date) {
		return nil, fmt.Errorf("invalid date format: %s", date)
	}
	workerName = sanitizeWorkerName(workerName)

	fpath := filepath.Join(r.logDir, date, workerName+".log")
	return readTailEntries(fpath, lines)
}

// ReadByTaskId 按 taskId 过滤日志
func (r *WorkerLogReader) ReadByTaskId(workerName, taskId, date string, lines int) ([]WorkerLogEntry, error) {
	if lines <= 0 {
		lines = 500
	}
	if lines > 10000 {
		lines = 10000
	}

	if date == "" {
		date = r.findLatestDate()
		if date == "" {
			return []WorkerLogEntry{}, nil
		}
	}

	// 安全校验：防止路径穿越
	if !validateDate(date) {
		return nil, fmt.Errorf("invalid date format: %s", date)
	}
	workerName = sanitizeWorkerName(workerName)

	fpath := filepath.Join(r.logDir, date, workerName+".log")
	entries, err := readTailEntries(fpath, lines*5) // 多读一些用于过滤
	if err != nil {
		return nil, err
	}

	// 按 taskId 过滤
	result := make([]WorkerLogEntry, 0, lines)
	for i := len(entries) - 1; i >= 0 && len(result) < lines; i-- {
		if entries[i].TaskId == taskId {
			result = append([]WorkerLogEntry{entries[i]}, result...)
		}
	}
	return result, nil
}

// ListDates 返回有日志的日期列表
func (r *WorkerLogReader) ListDates() ([]string, error) {
	entries, err := os.ReadDir(r.logDir)
	if err != nil {
		return []string{}, nil
	}
	var dates []string
	for _, e := range entries {
		if e.IsDir() && len(e.Name()) == 10 {
			dates = append(dates, e.Name())
		}
	}
	return dates, nil
}

// ListWorkers 返回指定日期下的 worker 列表
func (r *WorkerLogReader) ListWorkers(date string) ([]string, error) {
	if date == "" {
		date = r.findLatestDate()
	}
	if date == "" {
		return []string{}, nil
	}

	// 安全校验：防止路径穿越
	if !validateDate(date) {
		return nil, fmt.Errorf("invalid date format: %s", date)
	}

	dateDir := filepath.Join(r.logDir, date)
	entries, err := os.ReadDir(dateDir)
	if err != nil {
		return []string{}, nil
	}
	var workers []string
	for _, e := range entries {
		if !e.IsDir() {
			name := e.Name()
			if len(name) > 4 && name[len(name)-4:] == ".log" {
				workers = append(workers, name[:len(name)-4])
			}
		}
	}
	return workers, nil
}

// findLatestDate 找最新的日志日期
func (r *WorkerLogReader) findLatestDate() string {
	date, _ := r.FindLatestDate()
	return date
}

// FindLatestDate 找最新的日志日期（导出方法）
func (r *WorkerLogReader) FindLatestDate() (string, error) {
	dates, err := r.ListDates()
	if err != nil || len(dates) == 0 {
		return "", err
	}
	latest := dates[0]
	for _, d := range dates[1:] {
		if d > latest {
			latest = d
		}
	}
	return latest, nil
}

// GetLogDir 获取日志目录路径
func (r *WorkerLogReader) GetLogDir() string {
	return r.logDir
}

// readTailEntries 从文件末尾读取最后 N 条 JSONL 日志
func readTailEntries(fpath string, lines int) ([]WorkerLogEntry, error) {
	f, err := os.Open(fpath)
	if err != nil {
		return []WorkerLogEntry{}, nil
	}
	defer f.Close()

	// 读取整个文件并按行切分
	// 对于 50MB 文件，全扫描约 0.5~1 秒，可接受
	var allEntries []WorkerLogEntry
	buf := make([]byte, 0, 64*1024)
	tmp := make([]byte, 32*1024)

	for {
		n, err := f.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			// 按行切分
			for {
				idx := -1
				for i, b := range buf {
					if b == '\n' {
						idx = i
						break
					}
				}
				if idx < 0 {
					break
				}
				line := buf[:idx]
				buf = buf[idx+1:]

				var entry WorkerLogEntry
				if json.Unmarshal(line, &entry) == nil {
					allEntries = append(allEntries, entry)
				}
			}
		}
		if err != nil {
			break
		}
	}

	// 处理最后不满一行
	if len(buf) > 0 {
		var entry WorkerLogEntry
		if json.Unmarshal(buf, &entry) == nil {
			allEntries = append(allEntries, entry)
		}
	}

	// 返回最后 N 条
	if len(allEntries) > lines {
		allEntries = allEntries[len(allEntries)-lines:]
	}

	return allEntries, nil
}
