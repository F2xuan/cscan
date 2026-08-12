package worker

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
)

func (w *Worker) keepAliveLoop() {
	// 启动时立即发送一次心跳
	w.sendHeartbeat()

	const (
		normalInterval   = 30 * time.Second // 正常心跳间隔
		circuitInterval  = 60 * time.Second // 熔断期间心跳间隔
		circuitThreshold = 5                // 连续失败多少次进入熔断
	)

	ticker := time.NewTicker(normalInterval)
	defer ticker.Stop()

	consecutiveFailures := 0
	inCircuit := false

	for {
		select {
		case <-w.stopChan:
			return
		case <-ticker.C:
			if err := w.sendHeartbeatWithRetry(); err != nil {
				consecutiveFailures++

				// 进入熔断状态
				if !inCircuit && consecutiveFailures >= circuitThreshold {
					inCircuit = true
					ticker.Reset(circuitInterval)
					w.logger.Warn("Heartbeat circuit breaker OPEN after %d failures, interval increased to %v", consecutiveFailures, circuitInterval)
				}
			} else {
				// 退出熔断状态
				if inCircuit {
					inCircuit = false
					ticker.Reset(normalInterval)
					w.logger.Info("Heartbeat circuit breaker CLOSED, recovered after %d failures", consecutiveFailures)
				}
				consecutiveFailures = 0
			}
		}
	}
}

// sendHeartbeatWithRetry 带重试的心跳发送
func (w *Worker) sendHeartbeatWithRetry() error {
	var lastErr error
	for i := 0; i < 2; i++ { // 最多重试 1 次
		if i > 0 {
			time.Sleep(2 * time.Second)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := w.doSendHeartbeat(ctx)
		cancel()

		if err == nil {
			return nil
		}
		lastErr = err
	}
	return lastErr
}

// doSendHeartbeat 执行心跳发送
func (w *Worker) doSendHeartbeat(ctx context.Context) error {
	cpuLoad := GetCPULoad()
	memUsed := GetMemoryUsage()

	if cpuLoad < 0 || cpuLoad > 100 {
		cpuLoad = 0.0
	}
	if memUsed < 0 || memUsed > 100 {
		memUsed = 0.0
	}

	// 优先使用 Redis 直连
	if w.schedClient != nil {
		resp, err := w.schedClient.KeepAliveWithResponse(ctx, cpuLoad, memUsed,
			w.taskStarted, w.taskExecuted, w.config.Concurrency)
		if err != nil {
			return err
		}

		if resp.ManualStopFlag {
			w.logger.Info("received stop signal, stopping worker...")
			go func() {
				w.Stop()
				os.Exit(0)
			}()
		} else if resp.ManualReloadFlag {
			w.logger.Info("received reload/restart signal, restarting worker...")
			go func() {
				w.Stop()
				os.Exit(0)
			}()
		}

		if resp.DesiredConcurrency > 0 {
			w.applyConcurrency(resp.DesiredConcurrency)
		}
		return nil
	}

	// 回退到 HTTP
	resp, err := w.httpClient.Heartbeat(ctx, &HeartbeatReq{
		WorkerName:         w.config.Name,
		IP:                 w.config.IP,
		CpuLoad:            cpuLoad,
		MemUsed:            memUsed,
		TaskStartedNumber:  int32(w.taskStarted),
		TaskExecutedNumber: int32(w.taskExecuted),
		IsDaemon:           false,
		Concurrency:        w.config.Concurrency,
	})
	if err != nil {
		return err
	}

	if resp.ManualStopFlag {
		w.logger.Info("received stop signal, stopping worker...")
		go func() {
			w.Stop()
			os.Exit(0)
		}()
	} else if resp.ManualReloadFlag {
		w.logger.Info("received reload/restart signal, restarting worker...")
		go func() {
			w.Stop()
			os.Exit(0)
		}()
	}

	if resp.DesiredConcurrency > 0 {
		w.applyConcurrency(resp.DesiredConcurrency)
	}

	return nil
}

// sendHeartbeat 发送心跳（简单包装，用于外部调用）
func (w *Worker) sendHeartbeat() {
	_ = w.sendHeartbeatWithRetry()
}

// controlPollingLoop 控制信号循环（内部方法，作为WebSocket的备份方案）
// 使用 Redis Pub/Sub 订阅 cscan:task:ctrl:* 频道，实时接收控制信号
func (w *Worker) controlPollingLoop() {
	// 优先使用 Redis Pub/Sub 实时订阅
	if w.schedClient != nil {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// 监听 stopChan 以取消订阅
		go func() {
			select {
			case <-w.stopChan:
				cancel()
			case <-ctx.Done():
			}
		}()

		signalCh := w.schedClient.SubscribeCancel(ctx)
		for signal := range signalCh {
			if signal != nil {
				w.handleControlSignal(signal.TaskId, signal.Action)
			}
		}
		return
	}

	// 回退到 HTTP 轮询
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopChan:
			return
		case <-ticker.C:
			taskIds := w.getRunningTaskIds()
			if len(taskIds) == 0 {
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			resp, err := w.httpClient.GetTaskControlSignals(ctx, taskIds)
			cancel()

			if err != nil {
				continue
			}

			for _, signal := range resp.Signals {
				w.handleControlSignal(signal.TaskId, signal.Action)
			}
		}
	}
}

// getRunningTaskIds 获取当前正在执行的任务ID列表
func (w *Worker) getRunningTaskIds() []string {
	var taskIds []string
	w.runningTasks.Range(func(key, value interface{}) bool {
		if taskId, ok := key.(string); ok {
			taskIds = append(taskIds, taskId)
		}
		return true
	})
	return taskIds
}

// CPU负载阈值常量
const (
	CPULoadThreshold     = 80.0 // CPU负载阈值，超过此值暂停任务拉取
	CPULoadRecovery      = 60.0 // CPU负载恢复阈值，低于此值恢复任务拉取
	CPUCheckInterval     = 5    // CPU检查间隔(秒)
	CPUOverloadThreshold = 3    // 连续过载次数阈值，超过则进入限流
	ThrottleDuration     = 30   // 限流持续时间(秒)
)

// isCPUOverloaded 检查CPU是否过载
// 当CPU负载超过80%时返回true，暂停任务下发以防止扫描引擎崩溃
// 实现智能限流：连续多次过载后进入限流状态，等待一段时间后自动恢复
func (w *Worker) isCPUOverloaded() bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 检查是否处于限流状态
	if w.isThrottled {
		if time.Now().Before(w.throttleUntil) {
			return true // 仍在限流期间
		}
		// 限流期结束，重置状态
		w.isThrottled = false
		w.cpuOverloadCount = 0
		w.logger.Info("CPU throttle period ended, resuming task fetch")
	}

	// 避免频繁检查CPU
	if time.Since(w.lastCPUCheck) < time.Duration(CPUCheckInterval)*time.Second {
		return false
	}
	w.lastCPUCheck = time.Now()

	// 快速获取CPU使用率（1秒采样）
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil || len(cpuPercent) == 0 {
		return false // 获取失败时不阻止任务
	}

	cpuLoad := cpuPercent[0]

	if cpuLoad >= CPULoadThreshold {
		w.cpuOverloadCount++
		w.logger.Warn("CPU load %.1f%% exceeds threshold %.1f%% (count: %d/%d)",
			cpuLoad, CPULoadThreshold, w.cpuOverloadCount, CPUOverloadThreshold)

		// 连续多次过载，进入限流状态
		if w.cpuOverloadCount >= CPUOverloadThreshold {
			w.isThrottled = true
			w.throttleUntil = time.Now().Add(time.Duration(ThrottleDuration) * time.Second)
			w.logger.Warn("Entering throttle mode for %d seconds to prevent engine crash", ThrottleDuration)
		}
		return true
	} else if cpuLoad < CPULoadRecovery {
		// CPU负载恢复正常，重置计数
		if w.cpuOverloadCount > 0 {
			w.cpuOverloadCount = 0
			w.logger.Info("CPU load %.1f%% recovered below %.1f%%, resetting overload count",
				cpuLoad, CPULoadRecovery)
		}
	}

	return false
}

// GetWorkerName 获取Worker名称
func GetWorkerName() string {
	hostname, _ := os.Hostname()
	// 使用 hostname + pid + 随机后缀，确保唯一性
	return fmt.Sprintf("%s-%d-%s", hostname, os.Getpid(), randomSuffix(4))
}

// randomSuffix 生成随机后缀
func randomSuffix(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}

// GetLocalIP 获取本机IP地址
func GetLocalIP() string {
	// 1. 优先使用环境变量 WORKER_IP（适用于 Docker 等容器环境）
	if ip := os.Getenv("WORKER_IP"); ip != "" {
		return ip
	}

	// 2. 尝试通过 UDP 连接获取出口 IP（更可靠的方式）
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		if localAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			return localAddr.IP.String()
		}
	}

	// 3. 回退到遍历网络接口
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

// GetSystemInfo 获取系统信息
func GetSystemInfo() map[string]interface{} {
	return map[string]interface{}{
		"os":       runtime.GOOS,
		"arch":     runtime.GOARCH,
		"cpus":     runtime.NumCPU(),
		"hostname": func() string { h, _ := os.Hostname(); return h }(),
		"ip":       GetLocalIP(),
	}
}

// TagMatchInfo 标签匹配信息
