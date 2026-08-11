package worker

import (
	"context"
	crypto_rand "crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ResultQueue 本地结果队列
// API 不可用时将任务结果持久化到本地文件，API 恢复后自动重放
type ResultQueue struct {
	mu           sync.Mutex
	dir          string                                                      // 队列文件目录
	maxSize      int                                                         // 最大文件数量
	replayFn     func(ctx context.Context, req *TaskResultReq) error         // 重放函数
	replayVulFn  func(ctx context.Context, req *VulResultReq) error          // 漏洞重放函数
	replayJSFn   func(ctx context.Context, req *SaveJSFinderResultReq) error // JS结果重放函数
	replayCertFn func(ctx context.Context, req *SaveCertResultReq) error     // 证书结果重放函数
	stopChan     chan struct{}
	stopOnce     sync.Once
	logger       func(level, format string, args ...interface{})

	// perQueueSeq 在同一进程内单调递增,消除同毫秒内多次入队导致的文件名碰撞
	// 修复 H1:原文件名 {millisecond}_{mainTaskId[:8]}_vul_{rand4}.json 在同毫秒+同任务+crypto_rand 失败返回 0 时会覆盖,静默丢漏洞结果
	seqCounter uint64
}

// queuedResult 队列中的结果条目
type queuedResult struct {
	EnqueueTime time.Time      `json:"enqueueTime"`
	Request     *TaskResultReq `json:"request"`
}

// queuedVulResult 漏洞队列中的结果条目
// 修复 C-02：漏洞结果本地持久化，避免 API 不可用时丢失
type queuedVulResult struct {
	EnqueueTime time.Time     `json:"enqueueTime"`
	Request     *VulResultReq `json:"request"`
}

// queuedJSResult JS 扫描结果队列条目
// 修复 P0：JS 结果与资产/漏洞对称处理，API 不可用时本地持久化，恢复后重放
type queuedJSResult struct {
	EnqueueTime time.Time              `json:"enqueueTime"`
	Request     *SaveJSFinderResultReq `json:"request"`
}

// queuedCertResult 证书采集结果队列条目
// 修复 P0：证书结果与资产/漏洞对称处理，API 不可用时本地持久化，恢复后重放
type queuedCertResult struct {
	EnqueueTime time.Time          `json:"enqueueTime"`
	Request     *SaveCertResultReq `json:"request"`
}

// NewResultQueue 创建结果队列
func NewResultQueue(dir string, maxSize int, replayFn func(ctx context.Context, req *TaskResultReq) error) *ResultQueue {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &ResultQueue{
		dir:      dir,
		maxSize:  maxSize,
		replayFn: replayFn,
		stopChan: make(chan struct{}),
	}
}

// SetVulReplayFn 设置漏洞结果的重放函数
func (q *ResultQueue) SetVulReplayFn(fn func(ctx context.Context, req *VulResultReq) error) {
	q.replayVulFn = fn
}

// SetJSReplayFn 设置 JS 结果的重放函数
func (q *ResultQueue) SetJSReplayFn(fn func(ctx context.Context, req *SaveJSFinderResultReq) error) {
	q.replayJSFn = fn
}

// SetCertReplayFn 设置证书结果的重放函数
func (q *ResultQueue) SetCertReplayFn(fn func(ctx context.Context, req *SaveCertResultReq) error) {
	q.replayCertFn = fn
}

// SetLogger 设置日志回调
func (q *ResultQueue) SetLogger(logger func(level, format string, args ...interface{})) {
	q.logger = logger
}

func (q *ResultQueue) log(level, format string, args ...interface{}) {
	if q.logger != nil {
		q.logger(level, format, args...)
	}
}

// Start 启动队列，创建目录并启动重放协程
func (q *ResultQueue) Start(ctx context.Context) error {
	if err := os.MkdirAll(q.dir, 0755); err != nil {
		return fmt.Errorf("create result queue dir: %w", err)
	}
	go q.replayLoop(ctx)
	return nil
}

// Stop 停止队列
func (q *ResultQueue) Stop() {
	q.stopOnce.Do(func() {
		close(q.stopChan)
	})
}

// 缺陷 2 防御：入队落盘前对超大字段做截断，避免队列文件（含完整 httpBody / screenshot /
// iconData base64）撑爆磁盘。截断仅在落盘副本上进行，不修改调用方的原始 req。
const maxQueuedFieldSize = 4 * 1024

func truncateStr(s string) string {
	if len(s) > maxQueuedFieldSize {
		return s[:maxQueuedFieldSize] + fmt.Sprintf("...[truncated %d bytes]", len(s)-maxQueuedFieldSize)
	}
	return s
}

func truncateBytes(b []byte) []byte {
	if len(b) > maxQueuedFieldSize {
		return b[:maxQueuedFieldSize]
	}
	return b
}

// truncateTaskReqForQueue 返回 req 的浅克隆，截断资产中的超大字段；不修改原始 req。
// 注意：Assets 是值切片（[]AssetDocument），这里新建切片并逐元素复制，避免改动调用方的底层数组。
func truncateTaskReqForQueue(req *TaskResultReq) *TaskResultReq {
	if req == nil {
		return nil
	}
	clone := *req
	if len(req.Assets) > 0 {
		clone.Assets = make([]AssetDocument, len(req.Assets))
		for i := range req.Assets {
			c := req.Assets[i]
			c.HttpBody = truncateStr(c.HttpBody)
			c.Screenshot = truncateStr(c.Screenshot)
			c.IconData = truncateBytes(c.IconData)
			clone.Assets[i] = c
		}
	}
	return &clone
}

// truncateVulSlice 返回 vuls 的浅克隆，截断漏洞结果中的大字段（Result/Extra/Request/Response）。
// Vuls 是值切片（[]VulDocument），同样新建切片避免改动调用方数据。
func truncateVulSlice(vuls []VulDocument) []VulDocument {
	if len(vuls) == 0 {
		return vuls
	}
	out := make([]VulDocument, len(vuls))
	for i := range vuls {
		c := vuls[i]
		c.Result = truncateStr(c.Result)
		c.Extra = truncateStr(c.Extra)
		if c.Request != nil {
			t := truncateStr(*c.Request)
			c.Request = &t
		}
		if c.Response != nil {
			t := truncateStr(*c.Response)
			c.Response = &t
		}
		out[i] = c
	}
	return out
}

// Enqueue 将失败的结果入队到本地文件
func (q *ResultQueue) Enqueue(req *TaskResultReq) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 检查队列大小，超出时丢弃最旧的
	files := q.listFilesLocked()
	if len(files) >= q.maxSize {
		if len(files) > 0 {
			os.Remove(filepath.Join(q.dir, files[0]))
			q.log("WARN", "Result queue full, dropped oldest entry: %s", files[0])
		}
	}

	entry := queuedResult{
		EnqueueTime: time.Now(),
		Request:     truncateTaskReqForQueue(req),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal queued result: %w", err)
	}

	suffix := req.MainTaskId
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	// 文件名:`{millisecond}_{seq}_{suffix}.json`
	// seq 来自进程内单调计数器,确保同毫秒内多次入队不冲突(修复 H1)
	filename := fmt.Sprintf("%d_%d_%s.json", time.Now().UnixMilli(), q.nextSeq(), suffix)
	path := filepath.Join(q.dir, filename)

	// 原子写入：先写临时文件再重命名
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write queued result: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename queued result: %w", err)
	}

	q.log("INFO", "Queued result for task %s (%d assets) to %s", req.MainTaskId, len(req.Assets), filename)
	return nil
}

// EnqueueVul 将失败的漏洞结果入队到本地文件
// 修复 C-02：漏洞结果与资产结果对称处理，避免 API 不可用时永久丢失
func (q *ResultQueue) EnqueueVul(req *TaskResultReq, vuls []VulDocument) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 检查队列大小，超出时丢弃最旧的（同时考虑资产和漏洞文件）
	files := q.listFilesLocked()
	if len(files) >= q.maxSize {
		if len(files) > 0 {
			os.Remove(filepath.Join(q.dir, files[0]))
			q.log("WARN", "Result queue full, dropped oldest entry: %s", files[0])
		}
	}

	vulReq := &VulResultReq{
		WorkspaceId: req.WorkspaceId,
		MainTaskId:  req.MainTaskId,
		Vuls:        vuls,
	}
	vulReq.Vuls = truncateVulSlice(vulReq.Vuls)
	entry := queuedVulResult{
		EnqueueTime: time.Now(),
		Request:     vulReq,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal queued vul result: %w", err)
	}

	// 文件名:`{millisecond}_{seq}_{suffix}_vul.json`
	// seq 来自进程内单调计数器,消除同毫秒内多次入队冲突(修复 H1)
	// 原方案混用 secureRandInt,crypto_rand 失败返回 0 时同毫秒+同任务会覆盖漏洞文件,静默丢数据
	suffix := req.MainTaskId
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	filename := fmt.Sprintf("%d_%d_%s_vul.json", time.Now().UnixMilli(), q.nextSeq(), suffix)
	path := filepath.Join(q.dir, filename)

	// 原子写入：先写临时文件再重命名
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write queued vul result: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename queued vul result: %w", err)
	}

	q.log("INFO", "Queued vul result for task %s (%d vuls) to %s", req.MainTaskId, len(vuls), filename)
	return nil
}

// EnqueueJS 将失败的 JS 扫描结果入队到本地文件
// 修复 P0：与资产/漏洞对称，API 不可用时本地持久化，恢复后由 replayLoop 重放
func (q *ResultQueue) EnqueueJS(req *SaveJSFinderResultReq) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	files := q.listFilesLocked()
	if len(files) >= q.maxSize {
		if len(files) > 0 {
			os.Remove(filepath.Join(q.dir, files[0]))
			q.log("WARN", "Result queue full, dropped oldest entry: %s", files[0])
		}
	}

	entry := queuedJSResult{
		EnqueueTime: time.Now(),
		Request:     req,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal queued js result: %w", err)
	}

	// 文件名:`{millisecond}_{seq}_js.json`，后缀 _js.json 供 replayOne 识别
	suffix := req.MainTaskId
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	filename := fmt.Sprintf("%d_%d_%s_js.json", time.Now().UnixMilli(), q.nextSeq(), suffix)
	path := filepath.Join(q.dir, filename)

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write queued js result: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename queued js result: %w", err)
	}

	q.log("INFO", "Queued JS result for task %s (%d findings) to %s", req.MainTaskId, len(req.Results), filename)
	return nil
}

// EnqueueCert 将失败的证书采集结果入队到本地文件
// 修复 P0：与资产/漏洞对称，API 不可用时本地持久化，恢复后由 replayLoop 重放
func (q *ResultQueue) EnqueueCert(req *SaveCertResultReq) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	files := q.listFilesLocked()
	if len(files) >= q.maxSize {
		if len(files) > 0 {
			os.Remove(filepath.Join(q.dir, files[0]))
			q.log("WARN", "Result queue full, dropped oldest entry: %s", files[0])
		}
	}

	entry := queuedCertResult{
		EnqueueTime: time.Now(),
		Request:     req,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal queued cert result: %w", err)
	}

	// 文件名:`{millisecond}_{seq}_cert.json`，后缀 _cert.json 供 replayOne 识别
	suffix := req.MainTaskId
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	filename := fmt.Sprintf("%d_%d_%s_cert.json", time.Now().UnixMilli(), q.nextSeq(), suffix)
	path := filepath.Join(q.dir, filename)

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write queued cert result: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename queued cert result: %w", err)
	}

	q.log("INFO", "Queued cert result for task %s (%d certs) to %s", req.MainTaskId, len(req.Results), filename)
	return nil
}

// nextSeq 返回进程内单调递增序号,已持锁调用
func (q *ResultQueue) nextSeq() uint64 {
	q.seqCounter++
	return q.seqCounter
}

// maxQueueAge 队列项最大存活时间。超过该时间的条目视为已无重试价值（如系统性故障
// 导致长期无法写入），重放时直接丢弃并打印 WARN，避免队列无限堆积、挤掉新结果。
const maxQueueAge = 24 * time.Hour

// isExpired 判断入队时间是否已过期
func (q *ResultQueue) isExpired(t time.Time) bool {
	return !t.IsZero() && time.Since(t) > maxQueueAge
}

// secureRandInt 生成 [0,n) 随机数，n<=0 时返回 0
// H-10 修复：原实现 int(b[0])<<8 | int(b[1])%n 因运算符优先级导致分布偏差且可能超过 n。
// 改用 crypto/rand.Int 确保均匀分布。
// 保留以供未来场景使用,当前文件名生成已切到 nextSeq 以彻底消除碰撞。
func secureRandInt(n int) int {
	if n <= 0 {
		return 0
	}
	result, err := crypto_rand.Int(crypto_rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		// 失败时用 UnixNano 作为后备熵,避免返回 0 造成同毫秒+同任务文件名覆盖
		return int(time.Now().UnixNano() % int64(n))
	}
	return int(result.Int64())
}

// replayLoop 定期检查并重放队列中的结果
func (q *ResultQueue) replayLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-q.stopChan:
			return
		case <-ticker.C:
			q.replayAll(ctx)
		}
	}
}

// replayAll 重放队列中的所有结果
func (q *ResultQueue) replayAll(ctx context.Context) {
	q.mu.Lock()
	files := q.listFilesLocked()
	q.mu.Unlock()

	if len(files) == 0 {
		return
	}

	q.log("INFO", "Replaying %d queued results...", len(files))

	for _, filename := range files {
		select {
		case <-ctx.Done():
			return
		case <-q.stopChan:
			return
		default:
		}

		path := filepath.Join(q.dir, filename)
		if err := q.replayOne(ctx, path); err != nil {
			q.log("WARN", "Failed to replay %s: %v, will retry later", filename, err)
			// 不阻塞其余文件：单条永久失败不应卡住整个队列，继续重放后续条目
			continue
		}
		// 成功后删除文件（过期丢弃分支已在 replayOne 内 os.Remove）
		os.Remove(path)
		q.log("INFO", "Replayed and removed %s", filename)
	}
}

// replayOne 重放单个结果文件
// 修复 C-02：识别漏洞文件（含 _vul_ 标记）并调用漏洞重放函数
func (q *ResultQueue) replayOne(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	filename := filepath.Base(path)
	// 漏洞文件名格式:{ts}_{seq}_{suffix}_vul.json
	// 修复 H6:原检测 strings.Contains(filename, "_vul_") 对 H1 修复后的新命名不匹配
	//   新名后缀是 "_vul.json" 而非 "_vul_",_vul_ 子串已不存在,漏洞文件会被当资产文件解析
	//   解析失败→保留文件→循环重试→占满 maxSize 后丢旧,真实结果被静默挤掉
	// 改用后缀 ".json" 前的 "_vul" 判定,覆盖 _vul.json 和历史 _vul_{rand}.json 两种命名
	if strings.HasSuffix(filename, "_vul.json") || strings.Contains(filename, "_vul_") {
		var vulEntry queuedVulResult
		if err := json.Unmarshal(data, &vulEntry); err != nil {
			// 修复 RQ1：解析失败不返回 nil（会导致文件被删除），返回错误保留文件供排查
			q.log("WARN", "Corrupted vul queue file %s, keeping for investigation", filename)
			return fmt.Errorf("unmarshal vul queue file: %w", err)
		}
		if q.isExpired(vulEntry.EnqueueTime) {
			q.log("WARN", "Dropping expired vul queue file %s (enqueued %v)", filename, vulEntry.EnqueueTime)
			os.Remove(path)
			return nil
		}
		if q.replayVulFn == nil {
			return fmt.Errorf("vul replay function not set")
		}
		replayCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()
		return q.replayVulFn(replayCtx, vulEntry.Request)
	}

	// 修复 P0：JS 结果文件后缀 _js.json
	if strings.HasSuffix(filename, "_js.json") {
		var jsEntry queuedJSResult
		if err := json.Unmarshal(data, &jsEntry); err != nil {
			q.log("WARN", "Corrupted js queue file %s, keeping for investigation", filename)
			return fmt.Errorf("unmarshal js queue file: %w", err)
		}
		if q.isExpired(jsEntry.EnqueueTime) {
			q.log("WARN", "Dropping expired js queue file %s (enqueued %v)", filename, jsEntry.EnqueueTime)
			os.Remove(path)
			return nil
		}
		if q.replayJSFn == nil {
			return fmt.Errorf("js replay function not set")
		}
		replayCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()
		return q.replayJSFn(replayCtx, jsEntry.Request)
	}

	// 修复 P0：证书结果文件后缀 _cert.json
	if strings.HasSuffix(filename, "_cert.json") {
		var certEntry queuedCertResult
		if err := json.Unmarshal(data, &certEntry); err != nil {
			q.log("WARN", "Corrupted cert queue file %s, keeping for investigation", filename)
			return fmt.Errorf("unmarshal cert queue file: %w", err)
		}
		if q.isExpired(certEntry.EnqueueTime) {
			q.log("WARN", "Dropping expired cert queue file %s (enqueued %v)", filename, certEntry.EnqueueTime)
			os.Remove(path)
			return nil
		}
		if q.replayCertFn == nil {
			return fmt.Errorf("cert replay function not set")
		}
		replayCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()
		return q.replayCertFn(replayCtx, certEntry.Request)
	}

	var entry queuedResult
	if err := json.Unmarshal(data, &entry); err != nil {
		// 修复 RQ1：解析失败不返回 nil（会导致文件被删除），返回错误保留文件供排查
		q.log("WARN", "Corrupted queue file %s, keeping for investigation", filename)
		return fmt.Errorf("unmarshal queue file: %w", err)
	}
	if q.isExpired(entry.EnqueueTime) {
		q.log("WARN", "Dropping expired queue file %s (enqueued %v)", filename, entry.EnqueueTime)
		os.Remove(path)
		return nil
	}

	replayCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	return q.replayFn(replayCtx, entry.Request)
}

// listFilesLocked 列出队列目录中的文件（需要持有锁）
func (q *ResultQueue) listFilesLocked() []string {
	entries, err := os.ReadDir(q.dir)
	if err != nil {
		return nil
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files
}

// Size 获取队列中的文件数量
func (q *ResultQueue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.listFilesLocked())
}
