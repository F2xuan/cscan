package worker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// FileLogEntry 日志条目，写入本地 JSONL 文件
type FileLogEntry struct {
	Ts     string `json:"ts"`
	Level  string `json:"level"`
	Worker string `json:"worker"`
	TaskId string `json:"taskId,omitempty"`
	Msg    string `json:"msg"`
}

// FileLogger 将日志写入本地 JSONL 文件（按日轮转），作为日志事实源
// 游标同步机制依赖此文件：WebSocket 断连时日志持续写入文件，重连后从游标续传
// 修复 M-14：新增保留期清理，根据 ACK 游标删除已同步的旧日志文件，并按天数+总容量双上限清理
type FileLogger struct {
	mu         sync.Mutex
	logDir     string // 日志目录 (如 "log")
	workerName string
	file       *os.File
	curDate    string // 当前文件对应的日期 YYYY-MM-DD
	curOffset  int64  // 当前文件已写入的字节数
	retention  int    // 日志保留天数
	maxBytes   int64  // 日志目录总容量上限（字节）
}

// NewFileLogger 创建本地文件日志器
func NewFileLogger(logDir, workerName string) *FileLogger {
	if logDir == "" {
		logDir = "log"
	}
	fl := &FileLogger{
		logDir:     logDir,
		workerName: workerName,
		retention:  7,                 // 默认保留 7 天
		maxBytes:   500 * 1024 * 1024, // 默认总容量上限 500MB
	}
	_ = os.MkdirAll(logDir, 0o755)
	return fl
}

// Cleanup 根据游标和保留策略清理已同步的旧日志文件
// 修复 M-14：每次日志同步 ACK 后可调用，删除游标之前的旧文件并按保留期/容量上限清理
func (fl *FileLogger) Cleanup(cursor SyncCursor) {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	// 1) 删除早于游标日期的已同步文件（整文件已确认同步）
	if cursor.Filename != "" {
		entries, err := os.ReadDir(fl.logDir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(name, ".jsonl") || len(name) != 16 {
				continue
			}
			// 早于游标文件日期的文件可安全删除
			if name < cursor.Filename {
				os.Remove(filepath.Join(fl.logDir, name))
			}
		}
	}

	// 2) 按保留天数删除过期文件
	cutoff := time.Now().AddDate(0, 0, -fl.retention).Format("2006-01-02") + ".jsonl"
	entries, _ := os.ReadDir(fl.logDir)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") || len(e.Name()) != 16 {
			continue
		}
		if e.Name() < cutoff {
			os.Remove(filepath.Join(fl.logDir, e.Name()))
		}
	}

	// 3) 总容量上限：按日期从旧到新删除直到低于上限
	if fl.maxBytes > 0 {
		fl.enforceSizeLimitLocked(entries)
	}
}

// enforceSizeLimitLocked 在持锁状态下按日期从旧到新删除文件直到总大小低于 maxBytes
func (fl *FileLogger) enforceSizeLimitLocked(entries []os.DirEntry) {
	// 收集所有 jsonl 文件并按名称升序（日期升序）
	type fileInfo struct {
		name string
		size int64
	}
	var files []fileInfo
	var total int64
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") || len(e.Name()) != 16 {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fileInfo{e.Name(), info.Size()})
		total += info.Size()
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })

	// 从最旧的文件开始删除直到低于上限
	for i := 0; i < len(files) && total > fl.maxBytes; i++ {
		// 不要删除当前正在写入的文件
		if files[i].name == fl.curDate+".jsonl" {
			continue
		}
		os.Remove(filepath.Join(fl.logDir, files[i].name))
		total -= files[i].size
	}
}

// Write 写入一条日志到本地文件，返回写入的文件名和偏移量信息
func (fl *FileLogger) Write(level, taskId, msg string) {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	now := time.Now().Local()
	dateStr := now.Format("2006-01-02")

	// 日期切换时轮转文件
	if fl.curDate != dateStr || fl.file == nil {
		if fl.file != nil {
			fl.file.Close()
		}
		fl.curDate = dateStr
		fl.curOffset = 0

		fpath := filepath.Join(fl.logDir, dateStr+".jsonl")
		f, err := os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			logx.Errorf("[FileLogger] Failed to open log file %s: %v", fpath, err)
			return
		}
		fl.file = f

		// 记录当前文件大小作为起始偏移
		if info, err := f.Stat(); err == nil {
			fl.curOffset = info.Size()
		}
	}

	entry := FileLogEntry{
		Ts:     now.Format("2006-01-02T15:04:05.000-07:00"),
		Level:  level,
		Worker: fl.workerName,
		TaskId: taskId,
		Msg:    msg,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	data = append(data, '\n')

	n, err := fl.file.Write(data)
	if err != nil {
		logx.Errorf("[FileLogger] Failed to write log: %v", err)
		return
	}
	fl.curOffset += int64(n)
}

// Close 关闭文件
func (fl *FileLogger) Close() {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	if fl.file != nil {
		fl.file.Close()
		fl.file = nil
	}
}

// SetWorkerName 更新 worker 名称（rename 后调用）
func (fl *FileLogger) SetWorkerName(name string) {
	fl.mu.Lock()
	fl.workerName = name
	fl.mu.Unlock()
}

// GetCurrentDate 返回当前日志文件的日期
func (fl *FileLogger) GetCurrentDate() string {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	return fl.curDate
}

// GetCurrentOffset 返回当前日志文件的偏移量
func (fl *FileLogger) GetCurrentOffset() int64 {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	return fl.curOffset
}

// ==================== 游标管理 ====================

// SyncCursor 同步游标，记录已成功同步到 API 的位置
type SyncCursor struct {
	Filename string `json:"filename"` // YYYY-MM-DD.jsonl
	Offset   int64  `json:"offset"`   // 已同步的字节偏移
}

// CursorManager 游标管理器，持久化到磁盘
type CursorManager struct {
	mu       sync.Mutex
	filePath string // 游标文件路径 (如 "logs/.sync_offset")
	cursor   SyncCursor
}

// NewCursorManager 创建游标管理器
func NewCursorManager(logDir string) *CursorManager {
	if logDir == "" {
		logDir = "log"
	}
	cm := &CursorManager{
		filePath: filepath.Join(logDir, ".sync_offset"),
	}
	cm.load()
	return cm
}

// load 从磁盘加载游标
func (cm *CursorManager) load() {
	data, err := os.ReadFile(cm.filePath)
	if err != nil {
		return
	}
	json.Unmarshal(data, &cm.cursor)
}

// save 持久化游标到磁盘
func (cm *CursorManager) save() error {
	data, err := json.Marshal(cm.cursor)
	if err != nil {
		return err
	}
	return os.WriteFile(cm.filePath, data, 0o644)
}

// Get 获取当前游标
func (cm *CursorManager) Get() SyncCursor {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.cursor
}

// Update 更新游标并持久化
func (cm *CursorManager) Update(filename string, offset int64) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.cursor = SyncCursor{Filename: filename, Offset: offset}
	return cm.save()
}

// ==================== 文件续读器 ====================

// LogSyncReader 从指定游标位置读取日志文件内容，支持跨日续读
type LogSyncReader struct {
	logDir string
}

// NewLogSyncReader 创建日志同步读取器
func NewLogSyncReader(logDir string) *LogSyncReader {
	if logDir == "" {
		logDir = "log"
	}
	return &LogSyncReader{logDir: logDir}
}

// SyncReadResult 同步读取结果
type SyncReadResult struct {
	Filename  string         `json:"filename"`  // 当前读取的文件名
	Logs      []FileLogEntry `json:"logs"`      // 读取到的日志
	NewOffset int64          `json:"newOffset"` // 读取后的新偏移
	HasMore   bool           `json:"hasMore"`   // 是否还有更多数据
	NextFile  string         `json:"nextFile"`  // 下一个待读文件（跨日续读）
}

// ReadFrom 从指定游标位置读取最多 maxLines 条日志
// 如果当前文件已读完但存在更新的日志文件，会自动切换并返回 NextFile
func (r *LogSyncReader) ReadFrom(cursor SyncCursor, maxLines int) (*SyncReadResult, error) {
	if maxLines <= 0 {
		maxLines = 500
	}

	result := &SyncReadResult{
		Logs: make([]FileLogEntry, 0, maxLines),
	}

	// 确定起始文件
	startFile := cursor.Filename
	if startFile == "" {
		// 首次同步，找最早的日志文件
		startFile = r.findEarliestFile("")
		if startFile == "" {
			return result, nil // 没有日志文件
		}
	}

	fpath := filepath.Join(r.logDir, startFile)
	info, err := os.Stat(fpath)
	if err != nil {
		// 文件可能已被清理，尝试找下一个文件
		nextFile := r.findEarliestFile(startFile)
		if nextFile == "" {
			return result, nil
		}
		startFile = nextFile
		fpath = filepath.Join(r.logDir, startFile)
		info, err = os.Stat(fpath)
		if err != nil {
			return result, nil
		}
	}

	// 打开文件从 offset 开始读取
	f, err := os.Open(fpath)
	if err != nil {
		return result, fmt.Errorf("open file %s: %w", startFile, err)
	}
	defer f.Close()

	// 定位到游标偏移
	startOffset := cursor.Offset
	if startOffset > info.Size() {
		// 文件被截断或轮转，从头开始
		startOffset = 0
	}
	if _, err := f.Seek(startOffset, 0); err != nil {
		return result, fmt.Errorf("seek file %s: %w", startFile, err)
	}

	// 逐行读取 JSONL
	// 修复 H-1：游标按实际消费的字节推进，而非按原始读取块字节数。
	// 原实现 totalRead 累加 f.Read 返回的 n，但当 maxLines 触发 break 时，
	// buf 中剩余的未消费行也被计入 totalRead，导致下次读取时跳过这些行，永久丢失日志。
	buf := make([]byte, 0, 64*1024)
	tmp := make([]byte, 32*1024)
	consumedBytes := int64(0) // 已成功解析为日志行的字节数（含换行符）

	for len(result.Logs) < maxLines {
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
				// idx+1 包含换行符本身
				lineBytes := idx + 1
				buf = buf[lineBytes:]

				var entry FileLogEntry
				if json.Unmarshal(line, &entry) == nil {
					result.Logs = append(result.Logs, entry)
					consumedBytes += int64(lineBytes)
					if len(result.Logs) >= maxLines {
						break
					}
				} else {
					// 解析失败的行也推进游标，避免无限重试损坏行
					consumedBytes += int64(lineBytes)
				}
			}
		}
		if err != nil {
			break
		}
	}

	result.Filename = startFile
	result.NewOffset = startOffset + consumedBytes

	// 检查是否还有更多数据：重新 stat 获取当前文件大小（文件可能在读取期间被追加）
	// 修复 H-1：原实现使用旧的 info.Size()（打开时的快照），可能漏判新写入的数据
	if curInfo, err := os.Stat(fpath); err == nil {
		if result.NewOffset < curInfo.Size() {
			result.HasMore = true
		}
	}
	if !result.HasMore {
		// 当前文件读完，检查是否有更新的文件
		nextFile := r.findEarliestFile(startFile)
		if nextFile != "" {
			result.HasMore = true
			result.NextFile = nextFile
		}
	}

	return result, nil
}

// findEarliestFile 找到比 afterFile 更早或最新的日志文件
// afterFile 为空时返回最早的文件；非空时返回严格大于 afterFile 的最早文件
func (r *LogSyncReader) findEarliestFile(afterFile string) string {
	entries, err := os.ReadDir(r.logDir)
	if err != nil {
		return ""
	}

	var earliest string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// 只处理 YYYY-MM-DD.jsonl 格式（长度 16: 10位日期 + 1个点 + 5位jsonl）
		if !strings.HasSuffix(name, ".jsonl") || len(name) != 16 {
			continue
		}
		if afterFile != "" && name <= afterFile {
			continue
		}
		if earliest == "" || name < earliest {
			earliest = name
		}
	}
	return earliest
}
