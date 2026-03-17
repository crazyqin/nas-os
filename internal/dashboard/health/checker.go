// Package health 提供系统健康检查功能
// v2.52.0 - 系统健康检查器
package health

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

// Status 健康状态
type Status string

const (
	StatusHealthy  Status = "healthy"
	StatusWarning  Status = "warning"
	StatusCritical Status = "critical"
	StatusUnknown  Status = "unknown"
)

// CheckType 检查类型
type CheckType string

const (
	CheckTypeCPU     CheckType = "cpu"
	CheckTypeMemory  CheckType = "memory"
	CheckTypeDisk    CheckType = "disk"
	CheckTypeLoad    CheckType = "load"
	CheckTypeProcess CheckType = "process"
	CheckTypeService CheckType = "service"
	CheckTypeNetwork CheckType = "network"
)

// CheckResult 单项检查结果
type CheckResult struct {
	Type      CheckType     `json:"type"`
	Name      string        `json:"name"`
	Status    Status        `json:"status"`
	Message   string        `json:"message"`
	Value     interface{}   `json:"value,omitempty"`
	Threshold Threshold     `json:"threshold,omitempty"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
}

// Threshold 阈值配置
type Threshold struct {
	Warning  float64 `json:"warning"`
	Critical float64 `json:"critical"`
}

// HealthReport 健康检查报告
type HealthReport struct {
	OverallStatus Status        `json:"overallStatus"`
	Hostname      string        `json:"hostname"`
	Timestamp     time.Time     `json:"timestamp"`
	Duration      time.Duration `json:"duration"`
	Checks        []CheckResult `json:"checks"`
	Summary       Summary       `json:"summary"`
	Version       string        `json:"version"`
}

// Summary 检查摘要
type Summary struct {
	Total    int `json:"total"`
	Healthy  int `json:"healthy"`
	Warning  int `json:"warning"`
	Critical int `json:"critical"`
	Unknown  int `json:"unknown"`
}

// CheckerConfig 检查器配置
type CheckerConfig struct {
	// 磁盘阈值
	DiskWarningThreshold  float64
	DiskCriticalThreshold float64

	// 内存阈值
	MemoryWarningThreshold  float64
	MemoryCriticalThreshold float64

	// CPU 阈值
	CPUWarningThreshold  float64
	CPUCriticalThreshold float64

	// 负载阈值（相对于核心数）
	LoadWarningThreshold  float64
	LoadCriticalThreshold float64

	// 检查路径
	DiskPaths []string

	// 服务检查
	Services []ServiceCheck

	// 超时
	Timeout time.Duration
}

// ServiceCheck 服务检查配置
type ServiceCheck struct {
	Name    string
	Port    int
	Process string
	URL     string
}

// DefaultConfig 返回默认配置
func DefaultConfig() *CheckerConfig {
	return &CheckerConfig{
		DiskWarningThreshold:    80,
		DiskCriticalThreshold:   90,
		MemoryWarningThreshold:  80,
		MemoryCriticalThreshold: 90,
		CPUWarningThreshold:     80,
		CPUCriticalThreshold:    95,
		LoadWarningThreshold:    1.0, // 核心数 * 1.0
		LoadCriticalThreshold:   2.0, // 核心数 * 2.0
		DiskPaths:               []string{"/", "/var/lib/nas-os"},
		Services:                []ServiceCheck{},
		Timeout:                 30 * time.Second,
	}
}

// Checker 健康检查器
type Checker struct {
	config   *CheckerConfig
	hostname string
	version  string
	mu       sync.RWMutex
}

// NewChecker 创建健康检查器
func NewChecker(config *CheckerConfig) *Checker {
	if config == nil {
		config = DefaultConfig()
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	return &Checker{
		config:   config,
		hostname: hostname,
		version:  "v2.52.0",
	}
}

// SetVersion 设置版本
func (c *Checker) SetVersion(version string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.version = version
}

// Check 执行完整健康检查
func (c *Checker) Check(ctx context.Context) *HealthReport {
	start := time.Now()

	// 设置超时上下文
	if c.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.Timeout)
		defer cancel()
	}

	var checks []CheckResult

	// 并行执行检查
	var wg sync.WaitGroup
	var mu sync.Mutex

	checkFuncs := []func(context.Context) CheckResult{
		c.checkCPU,
		c.checkMemory,
		c.checkDisk,
		c.checkLoad,
		c.checkProcess,
	}

	// 添加服务检查
	for _, svc := range c.config.Services {
		svc := svc // 捕获变量
		checkFuncs = append(checkFuncs, func(ctx context.Context) CheckResult {
			return c.checkService(ctx, svc)
		})
	}

	for _, fn := range checkFuncs {
		wg.Add(1)
		go func(f func(context.Context) CheckResult) {
			defer wg.Done()
			result := f(ctx)
			mu.Lock()
			checks = append(checks, result)
			mu.Unlock()
		}(fn)
	}

	wg.Wait()

	// 计算总体状态和摘要
	summary := Summary{Total: len(checks)}
	overallStatus := StatusHealthy

	for _, check := range checks {
		switch check.Status {
		case StatusHealthy:
			summary.Healthy++
		case StatusWarning:
			summary.Warning++
			if overallStatus == StatusHealthy {
				overallStatus = StatusWarning
			}
		case StatusCritical:
			summary.Critical++
			overallStatus = StatusCritical
		default:
			summary.Unknown++
		}
	}

	c.mu.RLock()
	version := c.version
	c.mu.RUnlock()

	return &HealthReport{
		OverallStatus: overallStatus,
		Hostname:      c.hostname,
		Timestamp:     time.Now(),
		Duration:      time.Since(start),
		Checks:        checks,
		Summary:       summary,
		Version:       version,
	}
}

// QuickCheck 快速健康检查
func (c *Checker) QuickCheck(ctx context.Context) *HealthReport {
	start := time.Now()

	var checks []CheckResult

	// 只检查关键项
	checks = append(checks, c.checkProcess(ctx))
	checks = append(checks, c.checkMemory(ctx))
	checks = append(checks, c.checkDisk(ctx))

	summary := Summary{Total: len(checks)}
	var overallStatus Status = StatusHealthy

	for _, check := range checks {
		switch check.Status {
		case StatusHealthy:
			summary.Healthy++
		case StatusWarning:
			summary.Warning++
			if overallStatus == StatusHealthy {
				overallStatus = StatusWarning
			}
		case StatusCritical:
			summary.Critical++
			overallStatus = StatusCritical
		default:
			summary.Unknown++
		}
	}

	c.mu.RLock()
	version := c.version
	c.mu.RUnlock()

	return &HealthReport{
		OverallStatus: overallStatus,
		Hostname:      c.hostname,
		Timestamp:     time.Now(),
		Duration:      time.Since(start),
		Checks:        checks,
		Summary:       summary,
		Version:       version,
	}
}

// checkCPU 检查 CPU 状态
func (c *Checker) checkCPU(ctx context.Context) CheckResult {
	start := time.Now()
	result := CheckResult{
		Type:      CheckTypeCPU,
		Name:      "CPU 使用率",
		Timestamp: time.Now(),
	}

	percentages, err := cpu.PercentWithContext(ctx, 1*time.Second, false)
	if err != nil {
		result.Status = StatusUnknown
		result.Message = fmt.Sprintf("无法获取 CPU 使用率: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	if len(percentages) == 0 {
		result.Status = StatusUnknown
		result.Message = "无法获取 CPU 使用率: 无数据"
		result.Duration = time.Since(start)
		return result
	}

	usage := percentages[0]
	result.Value = map[string]interface{}{
		"usage":     usage,
		"cores":     runtime.NumCPU(),
		"threshold": c.config.CPUWarningThreshold,
	}
	result.Threshold = Threshold{
		Warning:  c.config.CPUWarningThreshold,
		Critical: c.config.CPUCriticalThreshold,
	}

	if usage >= c.config.CPUCriticalThreshold {
		result.Status = StatusCritical
		result.Message = fmt.Sprintf("CPU 使用率过高: %.1f%%", usage)
	} else if usage >= c.config.CPUWarningThreshold {
		result.Status = StatusWarning
		result.Message = fmt.Sprintf("CPU 使用率较高: %.1f%%", usage)
	} else {
		result.Status = StatusHealthy
		result.Message = fmt.Sprintf("CPU 使用率正常: %.1f%%", usage)
	}

	result.Duration = time.Since(start)
	return result
}

// checkMemory 检查内存状态
func (c *Checker) checkMemory(ctx context.Context) CheckResult {
	start := time.Now()
	result := CheckResult{
		Type:      CheckTypeMemory,
		Name:      "内存使用率",
		Timestamp: time.Now(),
	}

	vmStat, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		result.Status = StatusUnknown
		result.Message = fmt.Sprintf("无法获取内存信息: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	usage := vmStat.UsedPercent
	result.Value = map[string]interface{}{
		"total":     vmStat.Total,
		"used":      vmStat.Used,
		"free":      vmStat.Free,
		"available": vmStat.Available,
		"usage":     usage,
		"swapTotal": vmStat.SwapTotal,
		"swapUsed":  vmStat.SwapTotal - vmStat.SwapFree,
	}
	result.Threshold = Threshold{
		Warning:  c.config.MemoryWarningThreshold,
		Critical: c.config.MemoryCriticalThreshold,
	}

	if usage >= c.config.MemoryCriticalThreshold {
		result.Status = StatusCritical
		result.Message = fmt.Sprintf("内存使用率过高: %.1f%% (%.1f GB / %.1f GB)",
			usage, float64(vmStat.Used)/1024/1024/1024, float64(vmStat.Total)/1024/1024/1024)
	} else if usage >= c.config.MemoryWarningThreshold {
		result.Status = StatusWarning
		result.Message = fmt.Sprintf("内存使用率较高: %.1f%% (%.1f GB / %.1f GB)",
			usage, float64(vmStat.Used)/1024/1024/1024, float64(vmStat.Total)/1024/1024/1024)
	} else {
		result.Status = StatusHealthy
		result.Message = fmt.Sprintf("内存使用率正常: %.1f%% (%.1f GB / %.1f GB)",
			usage, float64(vmStat.Used)/1024/1024/1024, float64(vmStat.Total)/1024/1024/1024)
	}

	result.Duration = time.Since(start)
	return result
}

// checkDisk 检查磁盘状态
func (c *Checker) checkDisk(ctx context.Context) CheckResult {
	start := time.Now()
	result := CheckResult{
		Type:      CheckTypeDisk,
		Name:      "磁盘使用率",
		Timestamp: time.Now(),
	}

	// 检查所有配置的路径
	var deviceResults []map[string]interface{}
	var worstStatus Status = StatusHealthy
	var worstMsg string

	for _, path := range c.config.DiskPaths {
		usage, err := disk.UsageWithContext(ctx, path)
		if err != nil {
			// 路径不存在，跳过
			continue
		}

		deviceResults = append(deviceResults, map[string]interface{}{
			"path":   path,
			"total":  usage.Total,
			"used":   usage.Used,
			"free":   usage.Free,
			"usage":  usage.UsedPercent,
			"fstype": usage.Fstype,
		})

		// 更新最差状态
		if usage.UsedPercent >= c.config.DiskCriticalThreshold && worstStatus != StatusCritical {
			worstStatus = StatusCritical
			worstMsg = fmt.Sprintf("磁盘 %s 使用率过高: %.1f%%", path, usage.UsedPercent)
		} else if usage.UsedPercent >= c.config.DiskWarningThreshold && worstStatus == StatusHealthy {
			worstStatus = StatusWarning
			worstMsg = fmt.Sprintf("磁盘 %s 使用率较高: %.1f%%", path, usage.UsedPercent)
		}
	}

	if len(deviceResults) == 0 {
		result.Status = StatusUnknown
		result.Message = "无法获取磁盘信息"
		result.Duration = time.Since(start)
		return result
	}

	result.Value = map[string]interface{}{
		"devices": deviceResults,
	}
	result.Threshold = Threshold{
		Warning:  c.config.DiskWarningThreshold,
		Critical: c.config.DiskCriticalThreshold,
	}

	if worstStatus == StatusCritical {
		result.Status = StatusCritical
		result.Message = worstMsg
	} else if worstStatus == StatusWarning {
		result.Status = StatusWarning
		result.Message = worstMsg
	} else {
		result.Status = StatusHealthy
		result.Message = fmt.Sprintf("磁盘使用率正常 (检查了 %d 个挂载点)", len(deviceResults))
	}

	result.Duration = time.Since(start)
	return result
}

// checkLoad 检查系统负载
func (c *Checker) checkLoad(ctx context.Context) CheckResult {
	start := time.Now()
	result := CheckResult{
		Type:      CheckTypeLoad,
		Name:      "系统负载",
		Timestamp: time.Now(),
	}

	loadAvg, err := load.AvgWithContext(ctx)
	if err != nil {
		result.Status = StatusUnknown
		result.Message = fmt.Sprintf("无法获取系统负载: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	cores := float64(runtime.NumCPU())
	result.Value = map[string]interface{}{
		"load1":  loadAvg.Load1,
		"load5":  loadAvg.Load5,
		"load15": loadAvg.Load15,
		"cores":  cores,
	}
	result.Threshold = Threshold{
		Warning:  cores * c.config.LoadWarningThreshold,
		Critical: cores * c.config.LoadCriticalThreshold,
	}

	// 根据 1 分钟负载判断
	if loadAvg.Load1 >= cores*c.config.LoadCriticalThreshold {
		result.Status = StatusCritical
		result.Message = fmt.Sprintf("系统负载过高: %.2f (核心数: %.0f)", loadAvg.Load1, cores)
	} else if loadAvg.Load1 >= cores*c.config.LoadWarningThreshold {
		result.Status = StatusWarning
		result.Message = fmt.Sprintf("系统负载较高: %.2f (核心数: %.0f)", loadAvg.Load1, cores)
	} else {
		result.Status = StatusHealthy
		result.Message = fmt.Sprintf("系统负载正常: %.2f (核心数: %.0f)", loadAvg.Load1, cores)
	}

	result.Duration = time.Since(start)
	return result
}

// checkProcess 检查进程状态
func (c *Checker) checkProcess(ctx context.Context) CheckResult {
	start := time.Now()
	result := CheckResult{
		Type:      CheckTypeProcess,
		Name:      "进程状态",
		Timestamp: time.Now(),
	}

	// 检查 nasd 进程
	cmd := exec.CommandContext(ctx, "pgrep", "-x", "nasd")
	output, err := cmd.Output()
	if err != nil {
		result.Status = StatusCritical
		result.Message = "nasd 进程未运行"
		result.Duration = time.Since(start)
		return result
	}

	pid := strings.TrimSpace(string(output))
	if pid == "" {
		result.Status = StatusCritical
		result.Message = "nasd 进程未运行"
		result.Duration = time.Since(start)
		return result
	}

	// 获取进程信息
	pidInt, _ := strconv.Atoi(pid) // 忽略错误，pid 来自 pgrep 输出，格式可信

	// 获取内存使用
	var memRSS uint64
	var cpuPercent float64

	if pidInt > 0 {
		// 读取 /proc/<pid>/status 获取内存
		if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pidInt)); err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "VmRSS:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						if kb, parseErr := strconv.ParseUint(fields[1], 10, 64); parseErr == nil {
							memRSS = kb * 1024 // 转为字节
						}
					}
					break
				}
			}
		}

		// 读取 /proc/<pid>/stat 获取 CPU
		if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pidInt)); err == nil {
			fields := strings.Fields(string(data))
			if len(fields) >= 17 {
				utime, _ := strconv.ParseFloat(fields[13], 64) // 忽略错误，使用默认值 0
				stime, _ := strconv.ParseFloat(fields[14], 64) // 忽略错误，使用默认值 0
				cpuPercent = (utime + stime) / 100.0           // 转为百分比
			}
		}
	}

	result.Status = StatusHealthy
	result.Message = fmt.Sprintf("nasd 进程运行中 (PID: %s)", pid)
	result.Value = map[string]interface{}{
		"pid":        pidInt,
		"memRSS":     memRSS,
		"cpuPercent": cpuPercent,
	}
	result.Duration = time.Since(start)
	return result
}

// checkService 检查服务状态
func (c *Checker) checkService(ctx context.Context, svc ServiceCheck) CheckResult {
	start := time.Now()
	result := CheckResult{
		Type:      CheckTypeService,
		Name:      fmt.Sprintf("服务 %s", svc.Name),
		Timestamp: time.Now(),
	}

	// 检查进程
	if svc.Process != "" {
		cmd := exec.CommandContext(ctx, "pgrep", "-x", svc.Process)
		if err := cmd.Run(); err != nil {
			result.Status = StatusCritical
			result.Message = fmt.Sprintf("服务 %s 进程未运行", svc.Name)
			result.Duration = time.Since(start)
			return result
		}
	}

	// 检查端口
	if svc.Port > 0 {
		cmd := exec.CommandContext(ctx, "ss", "-tuln")
		output, err := cmd.Output()
		if err == nil {
			if !strings.Contains(string(output), fmt.Sprintf(":%d", svc.Port)) {
				result.Status = StatusWarning
				result.Message = fmt.Sprintf("服务 %s 端口 %d 未监听", svc.Name, svc.Port)
				result.Duration = time.Since(start)
				return result
			}
		}
	}

	// 检查 URL
	if svc.URL != "" {
		cmd := exec.CommandContext(ctx, "curl", "-sf", "--max-time", "5", svc.URL)
		if err := cmd.Run(); err != nil {
			result.Status = StatusWarning
			result.Message = fmt.Sprintf("服务 %s URL %s 不可达", svc.Name, svc.URL)
			result.Duration = time.Since(start)
			return result
		}
	}

	result.Status = StatusHealthy
	result.Message = fmt.Sprintf("服务 %s 正常", svc.Name)
	result.Value = map[string]interface{}{
		"name":    svc.Name,
		"port":    svc.Port,
		"process": svc.Process,
		"url":     svc.URL,
	}
	result.Duration = time.Since(start)
	return result
}

// GetSystemInfo 获取系统信息
func (c *Checker) GetSystemInfo(ctx context.Context) (map[string]interface{}, error) {
	info := make(map[string]interface{})

	// 主机信息
	hostInfo, err := host.InfoWithContext(ctx)
	if err == nil {
		info["hostname"] = hostInfo.Hostname
		info["os"] = hostInfo.OS
		info["platform"] = hostInfo.Platform
		info["platformVersion"] = hostInfo.PlatformVersion
		info["kernelVersion"] = hostInfo.KernelVersion
		info["uptime"] = hostInfo.Uptime
	}

	// CPU 信息
	cpuInfo, err := cpu.InfoWithContext(ctx)
	if err == nil && len(cpuInfo) > 0 {
		info["cpuModel"] = cpuInfo[0].ModelName
		info["cpuCores"] = cpuInfo[0].Cores
	}
	info["numCPU"] = runtime.NumCPU()

	// 内存信息
	vmStat, err := mem.VirtualMemoryWithContext(ctx)
	if err == nil {
		info["memTotal"] = vmStat.Total
		info["memAvailable"] = vmStat.Available
	}

	return info, nil
}
