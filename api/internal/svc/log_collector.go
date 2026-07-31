package svc

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"cscan/api/internal/config"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/zeromicro/go-zero/core/logx"
)

// LogCollector 后台持续采集 cscan 容器日志,写入本地文件(按日轮转)
// 模仿 flocks 日志架构: YYYY-MM-DD/{container}.log,自动清理过期文件
type LogCollector struct {
	cli         *client.Client
	logDir      string
	prefix      string
	registry    string
	extraNames  map[string]struct{}
	retention   int // 保留天数
	maxFileSize int64

	mu      sync.RWMutex
	writers map[string]*dayWriter // container -> current writer
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// dayWriter 单个容器当天的日志文件写入器
type dayWriter struct {
	file   *os.File
	date   string
	size   int64
	seq    int    // 当日分片序号（修复 M-12：达上限后切分新文件，而非重开同文件）
	name   string // 容器名
}

// LogFileInfo 日志文件元信息
type LogFileInfo struct {
	Name    string `json:"name"`    // 容器名
	Date    string `json:"date"`    // YYYY-MM-DD
	Size    int64  `json:"size"`    // 字节
	ModTime string `json:"modTime"` // 修改时间
}

// NewLogCollector 创建日志采集器;若 Docker 不可达返回 error
func NewLogCollector(cfg config.DockerConfig, logDir string, retentionDays int) (*LogCollector, error) {
	opts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}
	if cfg.Host != "" {
		opts = []client.Opt{client.WithHost(cfg.Host), client.WithAPIVersionNegotiation()}
	}
	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := cli.Ping(ctx); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("ping docker daemon: %w", err)
	}

	if logDir == "" {
		logDir = "log/containers"
	}
	if retentionDays <= 0 {
		retentionDays = 7
	}

	prefix := cfg.ContainerPrefix
	if prefix == "" {
		prefix = "cscan"
	}
	extras := make(map[string]struct{}, len(cfg.ExtraNames))
	for _, n := range cfg.ExtraNames {
		if n != "" {
			extras[n] = struct{}{}
		}
	}

	lc := &LogCollector{
		cli:         cli,
		logDir:      logDir,
		prefix:      prefix,
		registry:    cfg.ImageRegistry,
		extraNames:  extras,
		retention:   retentionDays,
		maxFileSize: 200 * 1024 * 1024, // 200MB 单文件上限
		writers:     make(map[string]*dayWriter),
		stopCh:      make(chan struct{}),
	}
	return lc, nil
}

// Start 启动后台采集循环
func (lc *LogCollector) Start() {
	// 确保根目录存在
	_ = os.MkdirAll(lc.logDir, 0o755)

	// 启动时清理过期日志
	lc.cleanup()

	// 主循环: 发现容器并为每个容器启动 tail goroutine
	lc.wg.Add(1)
	go lc.discoverLoop()

	// 每日清理 + 轮转检查
	lc.wg.Add(1)
	go lc.maintenanceLoop()

	logx.Infof("[LogCollector] started, dir=%s retention=%dd", lc.logDir, lc.retention)
}

// Stop 优雅停止
func (lc *LogCollector) Stop() {
	close(lc.stopCh)
	lc.wg.Wait()
	lc.mu.Lock()
	for name, w := range lc.writers {
		_ = w.file.Close()
		delete(lc.writers, name)
	}
	lc.mu.Unlock()
	if lc.cli != nil {
		_ = lc.cli.Close()
	}
	logx.Infof("[LogCollector] stopped")
}

// discoverLoop 定期发现 cscan 容器,为新容器启动 tail
func (lc *LogCollector) discoverLoop() {
	defer lc.wg.Done()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// 立即执行一次
	lc.discoverAndTail()

	for {
		select {
		case <-lc.stopCh:
			return
		case <-ticker.C:
			lc.discoverAndTail()
		}
	}
}

func (lc *LogCollector) discoverAndTail() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	list, err := lc.cli.ContainerList(ctx, container.ListOptions{All: false})
	if err != nil {
		logx.Errorf("[LogCollector] list containers: %v", err)
		return
	}

	for _, c := range list {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		if name == "" || !lc.isCscanContainer(name, c.Image) {
			continue
		}
		// Worker 容器也纳入采集：Worker 业务日志走游标同步写文件，
		// Worker 框架日志（go-zero/panic/runtime）通过 LogCollector tail stdout 捕获
		lc.ensureTailing(name)
	}
}

// ensureTailing 确保某容器有活跃的 tail goroutine
func (lc *LogCollector) ensureTailing(name string) {
	lc.mu.RLock()
	_, active := lc.writers[name]
	lc.mu.RUnlock()
	if active {
		return
	}

	lc.wg.Add(1)
	go lc.tailContainer(name)
}

// tailContainer 持续跟踪单个容器日志,断线自动重连
func (lc *LogCollector) tailContainer(name string) {
	defer lc.wg.Done()
	backoff := time.Second

	for {
		select {
		case <-lc.stopCh:
			return
		default:
		}

		err := lc.streamToFile(name)
		if err != nil {
			select {
			case <-lc.stopCh:
				return
			default:
			}
			logx.Errorf("[LogCollector] tail %s error: %v, retry in %v", name, err, backoff)
			time.Sleep(backoff)
			backoff = min(backoff*2, 30*time.Second)
			continue
		}
		// streamToFile 正常返回(容器停止),等待后重试
		select {
		case <-lc.stopCh:
			return
		case <-time.After(5 * time.Second):
		}
		backoff = time.Second
	}
}

// streamToFile 从 Docker API 读取日志流并写入文件
// 修复 M-13：goroutine 同时监听 stopCh 和 ctx.Done()，ctx 取消（断流/重连）时 goroutine 立即退出，
// 原实现仅监听全局 stopCh，正常断流后 ctx 已 cancel 但 goroutine 仍阻塞在 <-lc.stopCh，造成泄漏。
func (lc *LogCollector) streamToFile(name string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听 stop 信号或 ctx 取消
	go func() {
		select {
		case <-lc.stopCh:
			cancel()
		case <-ctx.Done():
			// 流结束/重连时退出，不泄漏 goroutine
		}
	}()

	reader, err := lc.cli.ContainerLogs(ctx, name, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "10000", // 重连时取最近10000行，减小日志丢失窗口
		Timestamps: true,
	})
	if err != nil {
		return fmt.Errorf("container logs: %w", err)
	}
	defer reader.Close()

	// 标记为活跃
	lc.mu.Lock()
	lc.writers[name] = &dayWriter{date: ""} // placeholder, getWriter 会初始化
	lc.mu.Unlock()

	defer func() {
		lc.mu.Lock()
		delete(lc.writers, name)
		lc.mu.Unlock()
	}()

	// 用 stdcopy 解复用 stdout/stderr
	out := &fileLineWriter{lc: lc, name: name, stream: "stdout"}
	errW := &fileLineWriter{lc: lc, name: name, stream: "stderr"}

	_, err = stdcopy.StdCopy(out, errW, reader)
	if ctx.Err() != nil {
		return nil // 正常停止
	}
	return err
}

// fileLineWriter 将 stdcopy 输出按行写入日志文件
type fileLineWriter struct {
	lc     *LogCollector
	name   string
	stream string
	carry  []byte
}

func (w *fileLineWriter) Write(p []byte) (int, error) {
	w.carry = append(w.carry, p...)
	for {
		idx := strings.IndexByte(string(w.carry), '\n')
		if idx < 0 {
			// 防止单行过大
			if len(w.carry) > 1<<20 {
				w.flushLine()
			}
			break
		}
		line := string(w.carry[:idx])
		w.carry = w.carry[idx+1:]
		w.writeLine(line)
	}
	return len(p), nil
}

func (w *fileLineWriter) flushLine() {
	if len(w.carry) > 0 {
		w.writeLine(string(w.carry))
		w.carry = w.carry[:0]
	}
}

// writeLine 写入一行: {docker_ts}\t{stream}\t{body}
func (w *fileLineWriter) writeLine(line string) {
	if line == "" {
		return
	}
	f, err := w.lc.getWriter(w.name)
	if err != nil {
		return
	}
	// 格式: 原始 Docker 时间戳行,前缀 stream 标记
	// "2026-07-28T08:37:34.123Z [stdout] actual log content"
	entry := fmt.Sprintf("%s [%s] %s\n", extractTs(line), w.stream, stripTs(line))
	n, _ := f.WriteString(entry)

	w.lc.mu.Lock()
	if dw, ok := w.lc.writers[w.name]; ok {
		dw.size += int64(n)
	}
	w.lc.mu.Unlock()
}

// extractTs 从 Docker 日志行提取时间戳部分
func extractTs(line string) string {
	// Docker Timestamps:true 格式: "2026-07-28T08:37:34.123456789Z message"
	idx := strings.IndexByte(line, ' ')
	if idx > 0 && idx < 40 {
		return line[:idx]
	}
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// stripTs 去掉 Docker 时间戳前缀
func stripTs(line string) string {
	idx := strings.IndexByte(line, ' ')
	if idx > 0 && idx < 40 {
		return line[idx+1:]
	}
	return line
}

// getWriter 获取容器当天的日志文件(自动轮转)
// 修复 M-12：达到 maxFileSize 后按序号切分新文件（name.log → name.1.log → name.2.log），
// 原实现关闭后以 O_APPEND 重开同名文件，导致文件无限增长，上限形同虚设。
func (lc *LogCollector) getWriter(name string) (*os.File, error) {
	today := time.Now().Format("2006-01-02")

	lc.mu.Lock()
	defer lc.mu.Unlock()

	dw, ok := lc.writers[name]
	if ok && dw.date == today && dw.file != nil {
		if dw.size < lc.maxFileSize {
			return dw.file, nil
		}
		// 达到上限：关闭当前分片，序号递增
		_ = dw.file.Close()
		dw.file = nil
		dw.seq++
	}

	// 关闭旧文件（日期切换场景）
	if ok && dw.file != nil {
		_ = dw.file.Close()
	}

	if !ok {
		dw = &dayWriter{name: name}
	}
	dw.date = today
	// 日期切换时重置序号
	if dw.date != today {
		dw.seq = 0
	}

	dir := filepath.Join(lc.logDir, today)
	_ = os.MkdirAll(dir, 0o755)
	safeName := sanitizeFilename(name)
	// 分片文件名：seq=0 → name.log；seq>0 → name.{seq}.log
	var fname string
	if dw.seq == 0 {
		fname = safeName + ".log"
	} else {
		fname = fmt.Sprintf("%s.%d.log", safeName, dw.seq)
	}
	fpath := filepath.Join(dir, fname)

	f, err := os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}

	info, _ := f.Stat()
	var size int64
	if info != nil {
		size = info.Size()
	}

	dw.file = f
	dw.size = size
	lc.writers[name] = dw
	return f, nil
}

// maintenanceLoop 每小时执行清理
func (lc *LogCollector) maintenanceLoop() {
	defer lc.wg.Done()
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-lc.stopCh:
			return
		case <-ticker.C:
			lc.cleanup()
		}
	}
}

// cleanup 删除超过保留天数的日志目录
func (lc *LogCollector) cleanup() {
	entries, err := os.ReadDir(lc.logDir)
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -lc.retention).Format("2006-01-02")
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// 目录名格式: YYYY-MM-DD
		if len(e.Name()) == 10 && e.Name() < cutoff {
			dirPath := filepath.Join(lc.logDir, e.Name())
			_ = os.RemoveAll(dirPath)
			logx.Infof("[LogCollector] cleaned old log dir: %s", e.Name())
		}
	}
}

// ==================== 读取接口 ====================

// ListDates 返回有日志的日期列表(降序)
func (lc *LogCollector) ListDates() []string {
	entries, err := os.ReadDir(lc.logDir)
	if err != nil {
		return nil
	}
	var dates []string
	for _, e := range entries {
		if e.IsDir() && len(e.Name()) == 10 {
			dates = append(dates, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	return dates
}

// ListContainersForDate 返回某天有日志的容器列表
func (lc *LogCollector) ListContainersForDate(date string) []LogFileInfo {
	dir := filepath.Join(lc.logDir, date)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []LogFileInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".log")
		files = append(files, LogFileInfo{
			Name:    name,
			Date:    date,
			Size:    info.Size(),
			ModTime: info.ModTime().Format(time.RFC3339),
		})
	}
	return files
}

// ReadLog 读取指定日期+容器的日志(tail 行)
// 修复 H-4：严格解析日期格式（YYYY-MM-DD）、清洗容器名，并校验最终绝对路径位于日志根目录内，防止路径穿越
func (lc *LogCollector) ReadLog(date, name string, tail int) ([]string, int, error) {
	// 路径安全检查：容器名
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return nil, 0, fmt.Errorf("invalid container name")
	}
	safeName := sanitizeFilename(name)
	if safeName == "" || safeName != name {
		return nil, 0, fmt.Errorf("invalid container name")
	}

	// 严格校验日期格式 YYYY-MM-DD（原仅检查长度 == 10，攻击者可传 "..\\..\\etc" 等 10 字节字符串穿越）
	if len(date) != 10 {
		return nil, 0, fmt.Errorf("invalid date format")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return nil, 0, fmt.Errorf("invalid date format: %w", err)
	}
	// 日期格式固定为 10 位 ASCII，不含路径分隔符，二次防御
	if strings.ContainsAny(date, `/\`) {
		return nil, 0, fmt.Errorf("invalid date format")
	}

	fpath := filepath.Join(lc.logDir, date, safeName+".log")
	// 校验最终绝对路径位于日志根目录内（防符号链接/拼接穿越）
	absLogDir, err1 := filepath.Abs(lc.logDir)
	absPath, err2 := filepath.Abs(fpath)
	if err1 == nil && err2 == nil {
		if !strings.HasPrefix(absPath, absLogDir+string(os.PathSeparator)) && absPath != absLogDir {
			return nil, 0, fmt.Errorf("path traversal detected")
		}
	}

	f, err := os.Open(fpath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	defer f.Close()

	if tail <= 0 {
		tail = 500
	}
	if tail > 10000 {
		tail = 10000
	}

	// 逐行扫描,保留最后 tail 行
	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	total := 0
	for scanner.Scan() {
		total++
		lines = append(lines, scanner.Text())
		if len(lines) > tail {
			lines = lines[1:]
		}
	}
	return lines, total, scanner.Err()
}

// ==================== 工具函数 ====================

func (lc *LogCollector) isCscanContainer(name string, image string) bool {
	if _, ok := lc.extraNames[name]; ok {
		return true
	}
	if strings.HasPrefix(name, lc.prefix+"-") || strings.HasPrefix(name, lc.prefix+"_") {
		return true
	}
	if lc.registry != "" && strings.HasPrefix(image, lc.registry) {
		return true
	}
	return false
}

func sanitizeFilename(name string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return r.Replace(name)
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
