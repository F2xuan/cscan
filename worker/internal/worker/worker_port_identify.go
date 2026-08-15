package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cscan/internal/scanner"
	"cscan/internal/scheduler"
)

func (w *Worker) executePortIdentify(ctx context.Context, task *scheduler.TaskInfo, assets []*scanner.Asset, config *scheduler.PortIdentifyConfig, orgId string) []*scanner.Asset {
	// 添加 panic 恢复机制
	defer func() {
		if r := recover(); r != nil {
			w.taskLog(task.TaskId, LevelError, "Port identify panic recovered: %v, stack: %s", r, string(getStackTrace()))
			// panic 时返回原始资产，确保任务能继续执行
			for _, asset := range assets {
				asset.IsHTTP = scanner.IsHTTPService(asset.Service, asset.Port)
			}
		}
	}()

	// 确定使用的工具
	tool := config.Tool
	if tool == "" {
		w.taskLog(task.TaskId, LevelInfo, "Port identify: tool not specified, using default 'nmap'")
		tool = "nmap" // 默认使用 nmap
	}

	w.taskLog(task.TaskId, LevelInfo, "Port identify: using tool '%s' (%d assets)", tool, len(assets))

	// 根据工具选择不同的执行逻辑
	if tool == "fingerprintx" {
		w.taskLog(task.TaskId, LevelInfo, "Port identify: executing with Fingerprintx")
		return w.executePortIdentifyWithFingerprintx(ctx, task, assets, config, orgId)
	} else {
		w.taskLog(task.TaskId, LevelInfo, "Port identify: executing with Nmap")
		return w.executePortIdentifyWithNmap(ctx, task, assets, config, orgId)
	}
}

// executePortIdentifyWithNmap 使用 Nmap 执行端口识别
func (w *Worker) executePortIdentifyWithNmap(ctx context.Context, task *scheduler.TaskInfo, assets []*scanner.Asset, config *scheduler.PortIdentifyConfig, orgId string) []*scanner.Asset {
	// 获取超时配置
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 30 // 默认30秒/主机
	}

	// 按主机分组（去重，防止同一端口被 Nmap 重复扫描）
	hostPorts := make(map[string][]int)
	hostAssets := make(map[string][]*scanner.Asset)
	seenPortPerHost := make(map[string]map[int]bool)
	for _, asset := range assets {
		if seenPortPerHost[asset.Host] == nil {
			seenPortPerHost[asset.Host] = make(map[int]bool)
		}
		if !seenPortPerHost[asset.Host][asset.Port] {
			seenPortPerHost[asset.Host][asset.Port] = true
			hostPorts[asset.Host] = append(hostPorts[asset.Host], asset.Port)
		}
		hostAssets[asset.Host] = append(hostAssets[asset.Host], asset)
	}

	// 计算总超时时间：单主机超时 × 主机数 / 并发数
	concurrency := config.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	totalTimeout := timeout * len(hostPorts) / concurrency
	if totalTimeout < 60 {
		totalTimeout = 60
	}
	w.taskLog(task.TaskId, LevelInfo, "Port identify(nmap): timeout=%ds (single=%ds, hosts=%d, concurrency=%d)",
		totalTimeout, timeout, len(hostPorts), concurrency)

	// 使用分离的 Context 以避免前面模块的超时拖累此独立模块
	// 同时使用一个 goroutine 定期检查整个任务有没有被用户下发全局 STOP 命令
	identifyCtx, identifyCancel := context.WithTimeout(context.Background(), time.Duration(totalTimeout)*time.Second)
	defer identifyCancel()

	// 监听父上下文取消或主动停止信号
	go func() {
		select {
		case <-identifyCtx.Done():
		case <-ctx.Done(): // 接收任务全局退出（异常停滞等）
			identifyCancel()
		}
	}()

	var identifiedAssets []*scanner.Asset
	nmapScanner := w.scanners["nmap"]

	// Nmap 内部并发数（nmap 本身已有并发控制，这里限制同时扫描的端口数）
	nmapConcurrency := concurrency
	if nmapConcurrency > 5 {
		nmapConcurrency = 5
	}

	for host, ports := range hostPorts {
		// 检查是否被停止或超时
		if identifyCtx.Err() == context.DeadlineExceeded {
			w.taskLog(task.TaskId, LevelWarn, "Port identify timeout, using partial results")
			// 超时时使用原始资产
			for _, asset := range hostAssets[host] {
				asset.IsHTTP = scanner.IsHTTPService(asset.Service, asset.Port)
				identifiedAssets = append(identifiedAssets, asset)
			}
			continue
		}
		if ctx.Err() != nil || w.checkTaskControl(ctx, task.TaskId) == "STOP" {
			w.taskLog(task.TaskId, LevelInfo, "Task stopped")
			return identifiedAssets
		}

		// naabu/masscan 未识别到端口时跳过 nmap，避免产生大量 "Nmap: no ports to scan" 噪音日志
		if len(ports) == 0 {
			w.taskLog(task.TaskId, LevelDebug, "Port identify(nmap): skipping %s, no ports to scan", host)
			for _, asset := range hostAssets[host] {
				asset.IsHTTP = scanner.IsHTTPService(asset.Service, asset.Port)
				identifiedAssets = append(identifiedAssets, asset)
			}
			continue
		}

		// 构建端口字符串
		portStrs := make([]string, len(ports))
		for i, p := range ports {
			portStrs[i] = fmt.Sprintf("%d", p)
		}
		portsStr := strings.Join(portStrs, ",")

		// 构建 Nmap 选项
		nmapOpts := &scanner.NmapOptions{
			Ports:      portsStr,
			Timeout:    timeout,
			Concurrent: nmapConcurrency,
		}
		if config.Args != "" {
			nmapOpts.Args = config.Args
		}

		nmapResult, err := nmapScanner.Scan(identifyCtx, &scanner.ScanConfig{
			Target:  host,
			Options: nmapOpts,
			TaskLogger: func(level, format string, args ...interface{}) {
				w.taskLog(task.TaskId, level, format, args...)
			},
		})

		// 检查是否被停止
		if ctx.Err() != nil || w.checkTaskControl(ctx, task.TaskId) == "STOP" {
			w.taskLog(task.TaskId, LevelInfo, "Task stopped")
			return identifiedAssets
		}

		if err != nil {
			w.taskLog(task.TaskId, LevelError, "Nmap error %s: %v", host, err)
			// Nmap失败时，使用原始资产
			for _, asset := range hostAssets[host] {
				asset.IsHTTP = scanner.IsHTTPService(asset.Service, asset.Port)
				identifiedAssets = append(identifiedAssets, asset)
			}
			continue
		}

		if nmapResult != nil && len(nmapResult.Assets) > 0 {
			// 设置 IsHTTP 字段
			for _, asset := range nmapResult.Assets {
				asset.IsHTTP = scanner.IsHTTPService(asset.Service, asset.Port)
			}
			identifiedAssets = append(identifiedAssets, nmapResult.Assets...)
			// 流式入库：单主机端口识别完成立即保存
			w.saveAssetResultWithFallback(ctx, task.MainTaskId, orgId, nmapResult.Assets)
		} else {
			// Nmap没有结果时，使用原始资产
			for _, asset := range hostAssets[host] {
				asset.IsHTTP = scanner.IsHTTPService(asset.Service, asset.Port)
				identifiedAssets = append(identifiedAssets, asset)
			}
		}
	}

	w.taskLog(task.TaskId, LevelInfo, "Port identify completed: %d assets", len(identifiedAssets))
	return identifiedAssets
}

// executePortIdentifyWithFingerprintx 使用 Fingerprintx 执行端口识别
func (w *Worker) executePortIdentifyWithFingerprintx(ctx context.Context, task *scheduler.TaskInfo, assets []*scanner.Asset, config *scheduler.PortIdentifyConfig, orgId string) []*scanner.Asset {
	// 获取配置
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 10 // fingerprintx 默认10秒/目标
	}

	concurrency := config.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}

	// 按单目标超时计算总超时：单目标超时 × 目标数 / 并发数
	totalTimeout := timeout * len(assets) / concurrency
	if totalTimeout < 60 {
		totalTimeout = 60
	}
	w.taskLog(task.TaskId, LevelInfo, "Port identify(fingerprintx): timeout=%ds (single=%ds, assets=%d, concurrency=%d)",
		totalTimeout, timeout, len(assets), concurrency)

	// 同样做防超时继承处理，保证独立阶段时间充足
	fingerCtx, fingerCancel := context.WithTimeout(context.Background(), time.Duration(totalTimeout)*time.Second)
	defer fingerCancel()

	go func() {
		select {
		case <-fingerCtx.Done():
		case <-ctx.Done():
			fingerCancel()
		}
	}()

	// 构建 fingerprintx 选项
	fpxOpts := &scanner.FingerprintxOptions{
		Timeout:     timeout,
		Concurrency: concurrency,
		UDP:         config.UDP,
		FastMode:    config.FastMode,
	}

	// 创建扫描配置
	scanConfig := &scanner.ScanConfig{
		Assets:     assets,
		Options:    fpxOpts,
		MainTaskId: task.TaskId,
		TaskLogger: func(level, format string, args ...interface{}) {
			w.taskLog(task.TaskId, level, format, args...)
		},
		OnProgress: func(progress int, message string) {
			// 可以在这里更新任务进度
			w.taskLog(task.TaskId, LevelDebug, "Progress: %d%% - %s", progress, message)
		},
	}

	// 执行扫描
	fpxScanner := w.scanners["fingerprintx"]
	result, err := fpxScanner.Scan(ctx, scanConfig)

	if err != nil {
		w.taskLog(task.TaskId, LevelError, "Fingerprintx error: %v", err)
		// 失败时返回原始资产
		for _, asset := range assets {
			asset.IsHTTP = scanner.IsHTTPService(asset.Service, asset.Port)
		}
		return assets
	}

	w.taskLog(task.TaskId, LevelInfo, "Port identify completed: %d assets", len(result.Assets))
	return result.Assets
}

// executeBruteScan 执行弱口令扫描阶段
