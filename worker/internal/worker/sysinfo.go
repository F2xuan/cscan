package worker

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

// WorkerInfoPayload Worker详细信息
type WorkerInfoPayload struct {
	Name         string          `json:"name"`
	IP           string          `json:"ip"`
	OS           string          `json:"os"`
	Arch         string          `json:"arch"`
	Version      string          `json:"version"`
	Hostname     string          `json:"hostname"`
	Uptime       int64           `json:"uptime"`       // Worker运行时长（秒）
	SystemUptime int64           `json:"systemUptime"` // 系统运行时长（秒）
	CpuCores     int             `json:"cpuCores"`
	CpuLoad      float64         `json:"cpuLoad"`
	MemTotal     uint64          `json:"memTotal"`
	MemUsed      uint64          `json:"memUsed"`
	MemPercent   float64         `json:"memPercent"`
	DiskTotal    uint64          `json:"diskTotal"`
	DiskUsed     uint64          `json:"diskUsed"`
	DiskPercent  float64         `json:"diskPercent"`
	Concurrency  int             `json:"concurrency"` // 并发数
	TaskStarted  int             `json:"taskStarted"`
	TaskRunning  int             `json:"taskRunning"`
	Tools        map[string]bool `json:"tools"`
}

// SysInfoCollector 系统信息收集器
type SysInfoCollector struct {
	workerName string
	workerIP   string
	version    string
	startTime  time.Time
}

// NewSysInfoCollector 创建系统信息收集器
func NewSysInfoCollector(workerName, workerIP, version string) *SysInfoCollector {
	return &SysInfoCollector{
		workerName: workerName,
		workerIP:   workerIP,
		version:    version,
		startTime:  time.Now(),
	}
}

// Collect 收集系统信息
func (c *SysInfoCollector) Collect(taskStarted, taskRunning, concurrency int) *WorkerInfoPayload {
	info := &WorkerInfoPayload{
		Name:        c.workerName,
		IP:          c.workerIP,
		Version:     c.version,
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		Uptime:      int64(time.Since(c.startTime).Seconds()),
		Concurrency: concurrency,
		TaskStarted: taskStarted,
		TaskRunning: taskRunning,
		Tools:       make(map[string]bool),
	}

	// 获取主机名
	if hostname, err := os.Hostname(); err == nil {
		info.Hostname = hostname
	}

	// 获取系统运行时长
	if hostInfo, err := host.Info(); err == nil {
		info.SystemUptime = int64(hostInfo.Uptime)
	}

	// 获取CPU信息
	info.CpuCores = runtime.NumCPU()
	if cpuPercent, err := cpu.Percent(0, false); err == nil && len(cpuPercent) > 0 {
		info.CpuLoad = cpuPercent[0]
	}

	// 获取内存信息
	if memInfo, err := mem.VirtualMemory(); err == nil {
		info.MemTotal = memInfo.Total
		info.MemUsed = memInfo.Used
		info.MemPercent = memInfo.UsedPercent
	}

	// 获取磁盘信息（根目录）
	diskPath := "/"
	if runtime.GOOS == "windows" {
		diskPath = "C:\\"
	}
	if diskInfo, err := disk.Usage(diskPath); err == nil {
		info.DiskTotal = diskInfo.Total
		info.DiskUsed = diskInfo.Used
		info.DiskPercent = diskInfo.UsedPercent
	}

	// 检测已安装的扫描工具
	info.Tools = c.detectTools()

	return info
}

// detectTools 检测已安装的扫描工具
func (c *SysInfoCollector) detectTools() map[string]bool {
	tools := map[string]bool{
		"nmap":      false,
		"masscan":   false,
		"nuclei":    false,
		"naabu":     false,
		"subfinder": false,
		"httpx":     false,
		"ffuf":      false,
		"dirsearch": false,
	}

	for tool := range tools {
		tools[tool] = c.isToolInstalled(tool)
	}

	return tools
}

// isToolInstalled 检查工具是否已安装
func (c *SysInfoCollector) isToolInstalled(toolName string) bool {
	// 尝试使用 which/where 命令查找
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("where", toolName)
	} else {
		cmd = exec.Command("which", toolName)
	}

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// 检查输出是否包含有效路径
	path := strings.TrimSpace(string(output))
	return path != "" && !strings.Contains(strings.ToLower(path), "not found")
}

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

// GetDiskUsage 获取磁盘使用信息
func GetDiskUsage() (total, used uint64, percent float64) {
	diskPath := "/"
	if runtime.GOOS == "windows" {
		diskPath = "C:\\"
	}
	if diskInfo, err := disk.Usage(diskPath); err == nil {
		return diskInfo.Total, diskInfo.Used, diskInfo.UsedPercent
	}
	return 0, 0, 0
}
