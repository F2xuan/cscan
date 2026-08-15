package worker

import (
	"os"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

// GetCPULoad 获取当前CPU负载
func GetCPULoad() float64 {
	if cpuPercent, err := cpu.Percent(0, false); err == nil && len(cpuPercent) > 0 {
		return cpuPercent[0]
	}
	return 0
}

// MemoryInfo 内存信息快照
type MemoryInfo struct {
	TotalMB       uint64  // 总内存(MB)
	AvailMB       uint64  // 可用内存(MB)
	UsedPercent   float64 // 使用率(%)
	ProcessMemMB  uint64  // 当前进程内存(MB)
	ProcessMemPct float32 // 当前进程内存占系统总内存百分比
}

// GetMemoryInfo 获取内存信息快照（供心跳与调度器复用，避免重复采集代码）
func GetMemoryInfo() MemoryInfo {
	info := MemoryInfo{}
	if memInfo, err := mem.VirtualMemory(); err == nil {
		info.TotalMB = memInfo.Total / 1024 / 1024
		info.AvailMB = memInfo.Available / 1024 / 1024
		info.UsedPercent = memInfo.UsedPercent
	}
	if proc, err := process.NewProcess(int32(os.Getpid())); err == nil {
		if procMem, err := proc.MemoryInfo(); err == nil {
			info.ProcessMemMB = procMem.RSS / 1024 / 1024
		}
		if percent, err := proc.MemoryPercent(); err == nil {
			info.ProcessMemPct = percent
		}
	}
	return info
}

// GetMemoryUsage 获取当前内存使用率
func GetMemoryUsage() float64 {
	return GetMemoryInfo().UsedPercent
}
