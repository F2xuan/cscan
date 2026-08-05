package worker

import (
	"context"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"cscan/internal/scanner"
)

// ScheduleMode 调度模式
type ScheduleMode int

const (
	ModeAggressive   ScheduleMode = iota // 激进模式：最大化吞吐量
	ModeNormal                           // 正常模式：平衡性能和资源
	ModeConservative                     // 保守模式：优先保护系统稳定
	ModeCritical                         // 危急模式：最小化资源使用
)

func (m ScheduleMode) String() string {
	switch m {
	case ModeAggressive:
		return "aggressive"
	case ModeNormal:
		return "normal"
	case ModeConservative:
		return "conservative"
	case ModeCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// ResourceMetrics 资源指标
type ResourceMetrics struct {
	CPUPercent        float64   // CPU使用率
	MemPercent        float64   // 系统内存使用率
	ProcessMemMB      uint64    // Worker 进程内存(MB)
	ProcessMemPercent float32   // Worker 进程内存占系统总内存百分比
	CPUCores          int       // CPU核心数
	TotalMemoryMB     uint64    // 总内存(MB)
	AvailMemoryMB     uint64    // 可用内存(MB)
	LoadAvg1          float64   // 1分钟负载
	LoadAvg5          float64   // 5分钟负载
	LoadAvg15         float64   // 15分钟负载
	Timestamp         time.Time // 采集时间
}

// AdaptiveSchedulerConfig 自适应调度器配置
type AdaptiveSchedulerConfig struct {
	// 基础配置
	BaseConcurrency int // 基础并发数（用户配置的值）
	MinConcurrency  int // 最小并发数
	MaxConcurrency  int // 最大并发数（不超过基础并发数）

	// 资源阈值配置
	CPULowThreshold               float64 // CPU低负载阈值（可以增加并发）
	CPUHighThreshold              float64 // CPU高负载阈值（需要减少并发）
	CPUCriticalThreshold          float64 // CPU危急阈值（立即限流）
	MemLowThreshold               float64 // 系统内存低负载阈值
	MemHighThreshold              float64 // 系统内存高负载阈值
	MemCriticalThreshold          float64 // 系统内存危急阈值
	ProcessMemLowThresholdMB      uint64  // Worker进程内存低负载阈值(MB)
	ProcessMemHighThresholdMB     uint64  // Worker进程内存高负载阈值(MB)
	ProcessMemCriticalThresholdMB uint64  // Worker进程内存危急阈值(MB)

	// 调度参数
	SampleInterval    time.Duration // 资源采样间隔
	AdjustInterval    time.Duration // 并发调整间隔
	HistorySize       int           // 历史数据保留数量
	SmoothingFactor   float64       // 平滑因子（0-1，越大越平滑）
	ScaleUpCooldown   time.Duration // 扩容冷却时间
	ScaleDownCooldown time.Duration // 缩容冷却时间

	// 任务拉取配置
	MinPullInterval time.Duration // 最小拉取间隔
	MaxPullInterval time.Duration // 最大拉取间隔
	IdleMultiplier  float64       // 空闲时拉取间隔倍数
}

// DefaultAdaptiveSchedulerConfig 默认配置
// 根据系统硬件自适应调整阈值，低配机器使用更宽松的阈值
// 复用 scanner.DetectSystemProfile() 统一硬件档位判定
func DefaultAdaptiveSchedulerConfig(baseConcurrency int) *AdaptiveSchedulerConfig {
	if baseConcurrency <= 0 {
		baseConcurrency = runtime.NumCPU()
	}

	// 复用 scanner 包的硬件档位判定，确保与扫描器参数一致
	profile := scanner.DetectSystemProfile()

	var cpuLow, cpuHigh, cpuCritical float64
	var memLow, memHigh, memCritical float64
	var processMemLowMB, processMemHighMB, processMemCriticalMB uint64

	switch profile {
	case scanner.ProfileLow:
		// 低配 (<=4核 或 <=4GB): 适当放宽阈值避免频繁限流
		// 注意: Go 程序 1.5GB RSS 是正常的，阈值必须高于此值
		// 6/29 OOM 事故修复：阈值下调，提前介入避免内存突破临界点
		cpuLow = 50.0
		cpuHigh = 85.0
		cpuCritical = 95.0
		memLow = 65.0
		memHigh = 80.0     // 从 85 下调到 80
		memCritical = 88.0 // 从 95 下调到 88，避免接近 OOM 才触发
		processMemLowMB = 1024
		processMemHighMB = 1280     // 从 1536 下调到 1280
		processMemCriticalMB = 1536 // 从 2048 下调到 1536，对 7551MB 总内存的 worker 更合理
	case scanner.ProfileMedium:
		// 中配 (<=8核 且 <=16GB): 适度放宽
		cpuLow = 45.0
		cpuHigh = 75.0
		cpuCritical = 88.0
		memLow = 55.0
		memHigh = 78.0
		memCritical = 92.0
		processMemLowMB = 1024
		processMemHighMB = 2048
		processMemCriticalMB = 3072
	default:
		// 高配 (>8核 或 >16GB): 使用较严格阈值
		cpuLow = 40.0
		cpuHigh = 70.0
		cpuCritical = 85.0
		memLow = 50.0
		memHigh = 75.0
		memCritical = 90.0
		processMemLowMB = 1536
		processMemHighMB = 3072
		processMemCriticalMB = 4096
	}

	return &AdaptiveSchedulerConfig{
		BaseConcurrency:               baseConcurrency,
		MinConcurrency:                1,
		MaxConcurrency:                baseConcurrency,
		CPULowThreshold:               cpuLow,
		CPUHighThreshold:              cpuHigh,
		CPUCriticalThreshold:          cpuCritical,
		MemLowThreshold:               memLow,
		MemHighThreshold:              memHigh,
		MemCriticalThreshold:          memCritical,
		ProcessMemLowThresholdMB:      processMemLowMB,
		ProcessMemHighThresholdMB:     processMemHighMB,
		ProcessMemCriticalThresholdMB: processMemCriticalMB,
		SampleInterval:                time.Second,
		AdjustInterval:                15 * time.Second, // 从 5s 增加到 15s，减少振荡频率
		HistorySize:                   60,
		SmoothingFactor:               0.3,
		ScaleUpCooldown:               30 * time.Second,
		ScaleDownCooldown:             15 * time.Second, // 从 10s 增加到 15s，与扩容冷却更平衡
		MinPullInterval:               3 * time.Second,
		MaxPullInterval:               10 * time.Second,
		IdleMultiplier:                2.0,
	}
}

// AdaptiveScheduler 自适应调度器
type AdaptiveScheduler struct {
	config *AdaptiveSchedulerConfig

	mu sync.RWMutex

	// 当前状态
	currentMode        ScheduleMode
	currentConcurrency int
	currentTasks       int32 // 使用atomic操作

	// 资源历史（ring buffer）
	metricsHistory []ResourceMetrics
	metricsHead    int // ring buffer 写入位置
	metricsCount   int // ring buffer 当前元素数量
	smoothedCPU    float64
	smoothedMem    float64

	// 冷却时间
	lastScaleUp   time.Time
	lastScaleDown time.Time

	// 日志抑制：连续相同模式时降频输出
	consecutiveModeCount int       // 当前模式连续出现次数
	lastModeLogTime      time.Time // 上次输出模式变更日志的时间

	// GC 触发标志：防止在 critical 模式下每个采样周期都触发 STW GC，导致自反馈恶化
	// 仅在首次跨越 80% 阈值时触发一次，退出 critical 后重置
	gcTriggered bool

	// 统计信息
	stats SchedulerStats

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 日志回调
	logger func(level, format string, args ...interface{})
}

// SchedulerStats 调度器统计
type SchedulerStats struct {
	TotalTasksAccepted   int64     // 接受的任务总数
	TotalTasksRejected   int64     // 拒绝的任务总数
	TotalScaleUps        int64     // 扩容次数
	TotalScaleDowns      int64     // 缩容次数
	TotalThrottles       int64     // 限流次数
	CurrentMode          string    // 当前模式
	CurrentConcurrency   int       // 当前并发数
	CurrentTasks         int       // 当前运行任务数
	AvailableSlots       int       // 当前剩余槽位数
	AvgCPU               float64   // 平均CPU
	AvgMem               float64   // 平均系统内存
	ProcessMemMB         uint64    // Worker进程内存(MB)
	ProcessMemHighMB     uint64    // Worker进程高阈值(MB)
	ProcessMemCriticalMB uint64    // Worker进程危急阈值(MB)
	LastAdjustTime       time.Time // 上次调整时间
	LastThrottleTime     time.Time // 上次限流时间
	ThrottledUntil       time.Time // 限流结束时间
	PullInterval         int64     // 当前拉取间隔(ms)
}

// NewAdaptiveScheduler 创建自适应调度器
func NewAdaptiveScheduler(config *AdaptiveSchedulerConfig) *AdaptiveScheduler {
	if config == nil {
		config = DefaultAdaptiveSchedulerConfig(runtime.NumCPU())
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &AdaptiveScheduler{
		config:             config,
		currentMode:        ModeNormal,
		currentConcurrency: config.BaseConcurrency,
		metricsHistory:     make([]ResourceMetrics, config.HistorySize),
		ctx:                ctx,
		cancel:             cancel,
	}

	return s
}

// SetLogger 设置日志回调
func (s *AdaptiveScheduler) SetLogger(logger func(level, format string, args ...interface{})) {
	s.logger = logger
}

func (s *AdaptiveScheduler) log(level, format string, args ...interface{}) {
	if s.logger != nil {
		s.logger(level, format, args...)
	}
}

// Start 启动调度器
func (s *AdaptiveScheduler) Start() {
	s.wg.Add(1)
	go s.monitorLoop()
	s.log("INFO", "Adaptive scheduler started, base concurrency: %d", s.config.BaseConcurrency)
}

// Stop 停止调度器
func (s *AdaptiveScheduler) Stop() {
	s.cancel()
	s.wg.Wait()
	s.log("INFO", "Adaptive scheduler stopped")
}

// monitorLoop 监控循环
func (s *AdaptiveScheduler) monitorLoop() {
	defer s.wg.Done()

	sampleTicker := time.NewTicker(s.config.SampleInterval)
	adjustTicker := time.NewTicker(s.config.AdjustInterval)
	defer sampleTicker.Stop()
	defer adjustTicker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-sampleTicker.C:
			s.sampleMetrics()
		case <-adjustTicker.C:
			s.adjustConcurrency()
		}
	}
}

// sampleMetrics 采样资源指标
func (s *AdaptiveScheduler) sampleMetrics() {
	metrics := s.collectMetrics()

	s.mu.Lock()
	defer s.mu.Unlock()

	// 添加到历史（ring buffer，O(1) 写入）
	s.metricsHistory[s.metricsHead] = metrics
	s.metricsHead = (s.metricsHead + 1) % s.config.HistorySize
	if s.metricsCount < s.config.HistorySize {
		s.metricsCount++
	}

	// 指数移动平均平滑
	alpha := s.config.SmoothingFactor
	s.smoothedCPU = alpha*metrics.CPUPercent + (1-alpha)*s.smoothedCPU
	s.smoothedMem = alpha*metrics.MemPercent + (1-alpha)*s.smoothedMem

	// 更新统计
	s.stats.AvgCPU = s.smoothedCPU
	s.stats.AvgMem = s.smoothedMem
}

// collectMetrics 收集资源指标（复用 sysinfo.go 的统一采集函数）
func (s *AdaptiveScheduler) collectMetrics() ResourceMetrics {
	metrics := ResourceMetrics{
		Timestamp: time.Now(),
		CPUCores:  runtime.NumCPU(),
	}

	// CPU使用率（复用 GetCPULoad：使用0避免阻塞，返回自上次调用以来的平均值）
	metrics.CPUPercent = GetCPULoad()

	// 内存使用率（复用 GetMemoryInfo：含进程内存与总/可用内存快照）
	memInfo := GetMemoryInfo()
	metrics.MemPercent = memInfo.UsedPercent
	metrics.TotalMemoryMB = memInfo.TotalMB
	metrics.AvailMemoryMB = memInfo.AvailMB
	metrics.ProcessMemMB = memInfo.ProcessMemMB
	metrics.ProcessMemPercent = memInfo.ProcessMemPct

	// 负载（仅Linux/Unix，Windows上会返回错误）
	if loadAvg, err := getLoadAvg(); err == nil {
		metrics.LoadAvg1 = loadAvg.Load1
		metrics.LoadAvg5 = loadAvg.Load5
		metrics.LoadAvg15 = loadAvg.Load15
	}

	return metrics
}

// LoadAvgInfo 负载信息
type LoadAvgInfo struct {
	Load1  float64
	Load5  float64
	Load15 float64
}

// adjustConcurrency 调整并发数
func (s *AdaptiveScheduler) adjustConcurrency() {
	s.mu.Lock()
	defer s.mu.Unlock()

	oldMode := s.currentMode
	oldConcurrency := s.currentConcurrency

	// 确定新模式
	newMode := s.determineMode()
	s.currentMode = newMode

	// 根据模式调整并发
	targetConcurrency := s.calculateTargetConcurrency(newMode)

	// 应用冷却时间
	now := time.Now()
	if targetConcurrency > s.currentConcurrency {
		if now.Sub(s.lastScaleUp) < s.config.ScaleUpCooldown {
			return // 扩容冷却中
		}
		s.lastScaleUp = now
		atomic.AddInt64(&s.stats.TotalScaleUps, 1)
	} else if targetConcurrency < s.currentConcurrency {
		if now.Sub(s.lastScaleDown) < s.config.ScaleDownCooldown {
			return // 缩容冷却中
		}
		s.lastScaleDown = now
		atomic.AddInt64(&s.stats.TotalScaleDowns, 1)
	}

	// 渐进式调整（每次最多调整25%）
	maxChange := int(math.Ceil(float64(s.currentConcurrency) * 0.25))
	if maxChange < 1 {
		maxChange = 1
	}

	diff := targetConcurrency - s.currentConcurrency
	if diff > maxChange {
		diff = maxChange
	} else if diff < -maxChange {
		diff = -maxChange
	}

	s.currentConcurrency += diff

	// 边界检查
	if s.currentConcurrency < s.config.MinConcurrency {
		s.currentConcurrency = s.config.MinConcurrency
	}
	if s.currentConcurrency > s.config.MaxConcurrency {
		s.currentConcurrency = s.config.MaxConcurrency
	}

	// 更新统计
	s.stats.CurrentMode = newMode.String()
	s.stats.CurrentConcurrency = s.currentConcurrency
	s.stats.LastAdjustTime = now

	// 记录变化（连续相同模式时降频输出，避免日志泛滥）
	if oldMode != newMode || oldConcurrency != s.currentConcurrency {
		if newMode == s.currentMode && newMode == ModeCritical {
			// 连续 critical 模式，每 5 次或每 2 分钟才输出一次日志
			s.consecutiveModeCount++
			if s.consecutiveModeCount%5 == 0 || now.Sub(s.lastModeLogTime) >= 2*time.Minute {
				s.log("INFO", "Scheduler adjusted: mode %s->%s, concurrency %d->%d (CPU:%.1f%%, SysMem:%.1f%%, ProcMem:%dMB) [%d consecutive critical adjustments]",
					oldMode, newMode, oldConcurrency, s.currentConcurrency, s.smoothedCPU, s.smoothedMem, s.getLatestProcessMemMB(), s.consecutiveModeCount)
				s.lastModeLogTime = now
			}
		} else {
			// 模式变更或非 critical 模式，正常输出
			s.consecutiveModeCount = 1
			s.lastModeLogTime = now
			s.log("INFO", "Scheduler adjusted: mode %s->%s, concurrency %d->%d (CPU:%.1f%%, SysMem:%.1f%%, ProcMem:%dMB)",
				oldMode, newMode, oldConcurrency, s.currentConcurrency, s.smoothedCPU, s.smoothedMem, s.getLatestProcessMemMB())
		}
	}

	// 主动 GC：当进程内存首次跨越 critical 阈值的 80% 时，触发一次 GC 释放内存
	// 修复 C3：原逻辑每个采样周期都触发 runtime.GC()，在 critical 模式下造成 STW 自反馈恶化
	// （GC 拖慢任务执行→任务积压→内存不释放→继续 critical→继续 GC）
	// 改为：仅首次跨越阈值时触发一次，退出 critical 后重置标志
	latestProcMem := s.getLatestProcessMemMB()
	gcThreshold := uint64(float64(s.config.ProcessMemCriticalThresholdMB) * 0.8)
	if latestProcMem > 0 && latestProcMem >= gcThreshold && !s.gcTriggered {
		runtime.GC()
		s.gcTriggered = true
		s.log("WARN", "Triggered GC at %dMB (threshold %dMB), will not trigger again until exiting critical mode",
			latestProcMem, gcThreshold)
	}

	// 退出 critical 模式时重置 GC 触发标志，允许下次再次触发
	if oldMode == ModeCritical && newMode != ModeCritical {
		s.gcTriggered = false
	}
}

// determineMode 确定调度模式
// 使用加权评分代替 OR 逻辑，避免单指标异常导致过度降级
// 增加滞回区间（Hysteresis）：已在 critical 模式时，需要资源降到更低阈值才退出
func (s *AdaptiveScheduler) determineMode() ScheduleMode {
	cpu := s.smoothedCPU
	mem := s.smoothedMem
	processMemMB := s.getLatestProcessMemMB()

	// 滞回区间：退出 critical 的阈值比进入时低 10%，避免在边界反复切换
	exitCriticalCPU := s.config.CPUCriticalThreshold - 10.0
	exitCriticalMem := s.config.MemCriticalThreshold - 8.0

	// 1. 已在 critical 模式时，使用更低的退出阈值（滞回）
	if s.currentMode == ModeCritical {
		// 仍然超过进入阈值，保持 critical
		if cpu >= s.config.CPUCriticalThreshold || mem >= s.config.MemCriticalThreshold || processMemMB >= s.config.ProcessMemCriticalThresholdMB {
			return ModeCritical
		}
		// 在退出阈值以上，保持 critical（滞回区间）
		if cpu >= exitCriticalCPU || mem >= exitCriticalMem {
			return ModeCritical
		}
		// 低于退出阈值，可以降级
	}

	// 2. 快速路径：任一指标超过危急阈值立即进入 critical（安全兜底）
	if cpu >= s.config.CPUCriticalThreshold || mem >= s.config.MemCriticalThreshold || processMemMB >= s.config.ProcessMemCriticalThresholdMB {
		return ModeCritical
	}

	// 3. 加权评分：综合考量 CPU、系统内存、进程内存
	// 每个指标 0-100 分，越高表示压力越大
	cpuScore := s.computePressureScore(cpu, s.config.CPULowThreshold, s.config.CPUHighThreshold)
	memScore := s.computePressureScore(mem, s.config.MemLowThreshold, s.config.MemHighThreshold)
	processMemScore := s.computeProcessMemPressureScore(processMemMB)

	// 加权平均：CPU 权重 0.4，系统内存 0.3，进程内存 0.3
	weightedScore := cpuScore*0.4 + memScore*0.3 + processMemScore*0.3

	// 4. 根据综合评分确定模式
	if weightedScore >= 70 {
		return ModeConservative
	}

	// 激进模式：资源整体较低时才允许
	if cpu < s.config.CPULowThreshold && mem < s.config.MemLowThreshold && processMemMB < s.config.ProcessMemLowThresholdMB {
		return ModeAggressive
	}

	return ModeNormal
}

// computePressureScore 计算资源压力评分 (0-100)
// low 以下为 0，high 以上为 100，中间线性插值
func (s *AdaptiveScheduler) computePressureScore(current, lowThreshold, highThreshold float64) float64 {
	if current <= lowThreshold {
		return 0
	}
	if current >= highThreshold {
		return 100
	}
	return (current - lowThreshold) / (highThreshold - lowThreshold) * 100
}

// computeProcessMemPressureScore 计算进程内存压力评分 (0-100)
func (s *AdaptiveScheduler) computeProcessMemPressureScore(processMemMB uint64) float64 {
	low := float64(s.config.ProcessMemLowThresholdMB)
	high := float64(s.config.ProcessMemHighThresholdMB)
	current := float64(processMemMB)

	if current <= low {
		return 0
	}
	if current >= high {
		return 100
	}
	return (current - low) / (high - low) * 100
}

func (s *AdaptiveScheduler) getLatestProcessMemMB() uint64 {
	if s.metricsCount == 0 {
		return 0
	}
	// ring buffer: 最新元素在 (head-1+size) % size
	idx := (s.metricsHead - 1 + s.config.HistorySize) % s.config.HistorySize
	return s.metricsHistory[idx].ProcessMemMB
}

func (s *AdaptiveScheduler) getLatestAvailMemoryMB() uint64 {
	if s.metricsCount == 0 {
		return 0
	}
	idx := (s.metricsHead - 1 + s.config.HistorySize) % s.config.HistorySize
	return s.metricsHistory[idx].AvailMemoryMB
}

// calculateTargetConcurrency 计算目标并发数
// 根据模式使用更细粒度的梯度，避免粗暴折扣导致并发骤降
func (s *AdaptiveScheduler) calculateTargetConcurrency(mode ScheduleMode) int {
	base := s.config.BaseConcurrency

	switch mode {
	case ModeCritical:
		// 危急模式：至少保留 1，最多 50%
		target := int(float64(base) * 0.5)
		if target < 1 {
			target = 1
		}
		return target
	case ModeConservative:
		// 保守模式：保留 75%，不再一刀切到 50%
		return int(float64(base) * 0.75)
	case ModeNormal:
		return base
	case ModeAggressive:
		return base
	default:
		return base
	}
}

// CanAcceptTask 检查是否可以接受新任务
// 注：critical 模式下直接拒收新任务，任务仍保留在服务端队列中，
// 等 worker 恢复到 normal/high 模式后下次轮询会继续拉取执行，不会丢失
func (s *AdaptiveScheduler) CanAcceptTask() bool {
	s.mu.RLock()
	mode := s.currentMode
	maxConcurrency := s.currentConcurrency
	s.mu.RUnlock()

	currentTasks := int(atomic.LoadInt32(&s.currentTasks))

	// critical 模式下直接拒收所有新任务，避免内存继续增长
	// 任务在 API 端仍处于 pending 状态，等内存恢复后继续执行
	if mode == ModeCritical {
		atomic.AddInt64(&s.stats.TotalTasksRejected, 1)
		return false
	}

	// 检查并发数
	if currentTasks >= maxConcurrency {
		atomic.AddInt64(&s.stats.TotalTasksRejected, 1)
		return false
	}

	// 实时检查资源（快速路径）
	if s.isResourceCritical() {
		atomic.AddInt64(&s.stats.TotalTasksRejected, 1)
		atomic.AddInt64(&s.stats.TotalThrottles, 1)
		s.mu.Lock()
		s.stats.LastThrottleTime = time.Now()
		s.mu.Unlock()
		return false
	}

	atomic.AddInt64(&s.stats.TotalTasksAccepted, 1)
	return true
}

// isResourceCritical 快速检查资源是否处于危急状态
func (s *AdaptiveScheduler) isResourceCritical() bool {
	// 使用缓存的平滑值进行快速判断
	s.mu.RLock()
	cpu := s.smoothedCPU
	mem := s.smoothedMem
	processMemMB := s.getLatestProcessMemMB()
	s.mu.RUnlock()

	return cpu >= s.config.CPUCriticalThreshold || mem >= s.config.MemCriticalThreshold || processMemMB >= s.config.ProcessMemCriticalThresholdMB
}

// AcquireSlot 获取任务槽位
// critical 模式下直接拒绝（任务保留在服务端队列，等恢复后继续）
func (s *AdaptiveScheduler) AcquireSlot() bool {
	s.mu.RLock()
	mode := s.currentMode
	maxConcurrency := s.currentConcurrency
	s.mu.RUnlock()

	if mode == ModeCritical {
		atomic.AddInt64(&s.stats.TotalTasksRejected, 1)
		return false
	}

	if s.isResourceCritical() {
		atomic.AddInt64(&s.stats.TotalTasksRejected, 1)
		atomic.AddInt64(&s.stats.TotalThrottles, 1)
		s.mu.Lock()
		s.stats.LastThrottleTime = time.Now()
		s.mu.Unlock()
		return false
	}

	for {
		current := atomic.LoadInt32(&s.currentTasks)
		if int(current) >= maxConcurrency {
			atomic.AddInt64(&s.stats.TotalTasksRejected, 1)
			return false
		}
		if atomic.CompareAndSwapInt32(&s.currentTasks, current, current+1) {
			atomic.AddInt64(&s.stats.TotalTasksAccepted, 1)
			return true
		}
	}
}

// ReleaseSlot 释放任务槽位
func (s *AdaptiveScheduler) ReleaseSlot() {
	if atomic.LoadInt32(&s.currentTasks) > 0 {
		atomic.AddInt32(&s.currentTasks, -1)
	}
}

// GetPullInterval 获取当前建议的任务拉取间隔
func (s *AdaptiveScheduler) GetPullInterval() time.Duration {
	s.mu.RLock()
	mode := s.currentMode
	currentTasks := atomic.LoadInt32(&s.currentTasks)
	maxConcurrency := s.currentConcurrency
	s.mu.RUnlock()

	// 基础间隔
	baseInterval := s.config.MinPullInterval

	// 根据模式调整
	switch mode {
	case ModeAggressive:
		baseInterval = s.config.MinPullInterval
	case ModeNormal:
		baseInterval = s.config.MinPullInterval * 2
	case ModeConservative:
		baseInterval = s.config.MinPullInterval * 4
	case ModeCritical:
		baseInterval = s.config.MaxPullInterval
	}

	// 根据负载调整
	loadRatio := float64(currentTasks) / float64(maxConcurrency)
	if loadRatio > 0.8 {
		// 高负载时增加间隔
		baseInterval = time.Duration(float64(baseInterval) * (1 + loadRatio))
	} else if loadRatio < 0.2 && int(currentTasks) < maxConcurrency {
		// 低负载且有空闲槽位时减少间隔
		baseInterval = time.Duration(float64(baseInterval) * 0.5)
	}

	// 边界检查
	if baseInterval < s.config.MinPullInterval {
		baseInterval = s.config.MinPullInterval
	}
	if baseInterval > s.config.MaxPullInterval {
		baseInterval = s.config.MaxPullInterval
	}

	// 更新统计
	s.mu.Lock()
	s.stats.PullInterval = int64(baseInterval / time.Millisecond)
	s.mu.Unlock()

	return baseInterval
}

// GetScannerConfig 获取扫描器配置建议
func (s *AdaptiveScheduler) GetScannerConfig() ScannerConfigRecommendation {
	s.mu.RLock()
	mode := s.currentMode
	cpu := s.smoothedCPU
	mem := s.smoothedMem
	s.mu.RUnlock()

	config := ScannerConfigRecommendation{}

	// 根据模式和资源状况计算推荐配置
	switch mode {
	case ModeAggressive:
		config.NaabuRate = 3000
		config.NaabuWorkers = 50
		config.NucleiConcurrency = 25
		config.NucleiRateLimit = 150
		config.FingerprintConcurrency = 20
	case ModeNormal:
		config.NaabuRate = 2000
		config.NaabuWorkers = 30
		config.NucleiConcurrency = 15
		config.NucleiRateLimit = 100
		config.FingerprintConcurrency = 10
	case ModeConservative:
		config.NaabuRate = 1000
		config.NaabuWorkers = 20
		config.NucleiConcurrency = 10
		config.NucleiRateLimit = 50
		config.FingerprintConcurrency = 5
	case ModeCritical:
		config.NaabuRate = 500
		config.NaabuWorkers = 10
		config.NucleiConcurrency = 5
		config.NucleiRateLimit = 20
		config.FingerprintConcurrency = 3
	}

	// 根据内存进一步调整
	if mem > 80 {
		config.NaabuWorkers = int(float64(config.NaabuWorkers) * 0.5)
		config.NucleiConcurrency = int(float64(config.NucleiConcurrency) * 0.5)
	}

	// 根据CPU进一步调整
	if cpu > 80 {
		config.NaabuRate = int(float64(config.NaabuRate) * 0.5)
		config.NucleiRateLimit = int(float64(config.NucleiRateLimit) * 0.5)
	}

	return config
}

// ScannerConfigRecommendation 扫描器配置建议
type ScannerConfigRecommendation struct {
	NaabuRate              int // Naabu 每秒发包数
	NaabuWorkers           int // Naabu 工作线程数
	NucleiConcurrency      int // Nuclei 并发数
	NucleiRateLimit        int // Nuclei 速率限制
	FingerprintConcurrency int // 指纹识别并发数
}

// GetCurrentMode 获取当前调度模式
func (s *AdaptiveScheduler) GetCurrentMode() ScheduleMode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentMode
}

// GetCurrentConcurrency 获取当前并发数
func (s *AdaptiveScheduler) GetCurrentConcurrency() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentConcurrency
}

// GetCurrentTasks 获取当前任务数
func (s *AdaptiveScheduler) GetCurrentTasks() int {
	return int(atomic.LoadInt32(&s.currentTasks))
}

// GetStats 获取统计信息
func (s *AdaptiveScheduler) GetStats() SchedulerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := s.stats
	stats.CurrentConcurrency = s.currentConcurrency
	stats.CurrentMode = s.currentMode.String()
	stats.CurrentTasks = int(atomic.LoadInt32(&s.currentTasks))
	stats.AvailableSlots = s.currentConcurrency - stats.CurrentTasks
	if stats.AvailableSlots < 0 {
		stats.AvailableSlots = 0
	}
	stats.ProcessMemMB = s.getLatestProcessMemMB()
	stats.ProcessMemHighMB = s.config.ProcessMemHighThresholdMB
	stats.ProcessMemCriticalMB = s.config.ProcessMemCriticalThresholdMB
	return stats
}


// SetMaxConcurrency 动态设置最大并发数
// 管理端显式设置时当前并发直接对齐新值，资源紧张时由调整循环自动回落
func (s *AdaptiveScheduler) SetMaxConcurrency(maxConcurrency int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if maxConcurrency > 0 {
		s.config.BaseConcurrency = maxConcurrency
		s.config.MaxConcurrency = maxConcurrency
		s.currentConcurrency = maxConcurrency
	}
}

// AvailableSlots 获取可用槽位数
func (s *AdaptiveScheduler) AvailableSlots() int {
	s.mu.RLock()
	maxConcurrency := s.currentConcurrency
	s.mu.RUnlock()

	currentTasks := int(atomic.LoadInt32(&s.currentTasks))
	available := maxConcurrency - currentTasks
	if available < 0 {
		return 0
	}
	return available
}

// CurrentTasks 获取当前任务数（兼容旧接口）
func (s *AdaptiveScheduler) CurrentTasks() int {
	return int(atomic.LoadInt32(&s.currentTasks))
}

// IsThrottled 检查是否处于限流状态
func (s *AdaptiveScheduler) IsThrottled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentMode == ModeCritical
}
