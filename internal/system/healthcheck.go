package system

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthStatus 健康状态类型.
type HealthStatus string

const (
	// StatusHealthy 系统健康，所有组件正常.
	StatusHealthy HealthStatus = "healthy"
	// StatusDegraded 系统降级，部分组件异常但系统仍可用.
	StatusDegraded HealthStatus = "degraded"
	// StatusUnhealthy 系统不健康，多个组件异常.
	StatusUnhealthy HealthStatus = "unhealthy"
	// StatusCritical 系统严重异常，核心组件不可用.
	StatusCritical HealthStatus = "critical"
)

// statusOrder 状态严重程度排序，值越大越严重.
var statusOrder = map[HealthStatus]int{
	StatusHealthy:   0,
	StatusDegraded:  1,
	StatusUnhealthy: 2,
	StatusCritical:  3,
}

// ComponentHealth 单个组件的健康信息.
type ComponentHealth struct {
	Name      string                 `json:"name"`           // 组件名称
	Status    HealthStatus           `json:"status"`         // 健康状态
	Message   string                 `json:"message"`        // 状态描述
	CheckedAt time.Time              `json:"checkedAt"`      // 检查时间
	Duration  string                 `json:"duration"`       // 检查耗时
	Details   map[string]interface{} `json:"details,omitempty"` // 详细数据
}

// SystemHealth 系统整体健康报告.
type SystemHealth struct {
	Status     HealthStatus      `json:"status"`      // 整体健康状态
	Message    string            `json:"message"`     // 状态描述
	Hostname   string            `json:"hostname"`    // 主机名
	CheckedAt  time.Time         `json:"checkedAt"`   // 检查时间
	Duration   string            `json:"duration"`    // 总检查耗时
	Components []*ComponentHealth `json:"components"`  // 各组件健康信息
	Summary    map[string]int    `json:"summary"`     // 各状态数量统计
}

// HealthCheckConfig 健康检查配置.
type HealthCheckConfig struct {
	// CPU 阈值
	CPUWarningThreshold  float64 `json:"cpuWarningThreshold"`  // CPU 使用率警告阈值（默认 70）
	CPUCriticalThreshold float64 `json:"cpuCriticalThreshold"` // CPU 使用率严重阈值（默认 90）

	// 内存阈值
	MemWarningThreshold  float64 `json:"memWarningThreshold"`  // 内存使用率警告阈值（默认 80）
	MemCriticalThreshold float64 `json:"memCriticalThreshold"` // 内存使用率严重阈值（默认 95）

	// 磁盘阈值
	DiskWarningThreshold  float64 `json:"diskWarningThreshold"`  // 磁盘使用率警告阈值（默认 80）
	DiskCriticalThreshold float64 `json:"diskCriticalThreshold"` // 磁盘使用率严重阈值（默认 95）

	// 温度阈值（摄氏度）
	TempWarningThreshold  int `json:"tempWarningThreshold"`  // 温度警告阈值（默认 70）
	TempCriticalThreshold int `json:"tempCriticalThreshold"` // 温度严重阈值（默认 85）

	// 负载阈值（相对于 CPU 核心数的倍数）
	LoadWarningMultiplier  float64 `json:"loadWarningMultiplier"`  // 负载警告倍数（默认 1.0）
	LoadCriticalMultiplier float64 `json:"loadCriticalMultiplier"` // 负载严重倍数（默认 2.0）

	// 网络连通性
	PingHosts   []string `json:"pingHosts"`   // Ping 检测目标
	PingTimeout int      `json:"pingTimeout"` // Ping 超时（秒，默认 5）

	// 服务列表
	CoreServices []string `json:"coreServices"` // 核心服务列表（如 smb, nfs, docker）

	// 并发控制
	ParallelCheck bool `json:"parallelCheck"` // 是否并行检查组件
}

// DefaultHealthCheckConfig 返回默认健康检查配置.
func DefaultHealthCheckConfig() *HealthCheckConfig {
	return &HealthCheckConfig{
		CPUWarningThreshold:    70,
		CPUCriticalThreshold:   90,
		MemWarningThreshold:    80,
		MemCriticalThreshold:   95,
		DiskWarningThreshold:   80,
		DiskCriticalThreshold:  95,
		TempWarningThreshold:   70,
		TempCriticalThreshold:  85,
		LoadWarningMultiplier:  1.0,
		LoadCriticalMultiplier: 2.0,
		PingHosts: []string{
			"8.8.8.8",
			"114.114.114.114",
		},
		PingTimeout:    5,
		CoreServices:   []string{"smbd", "nfs-kernel-server", "docker"},
		ParallelCheck:  true,
	}
}

// CheckerFunc 自定义组件检查函数.
type CheckerFunc func(ctx context.Context) *ComponentHealth

// SystemHealthChecker 系统健康检查器.
type SystemHealthChecker struct {
	monitor  *Monitor
	config   *HealthCheckConfig
	checkers map[string]CheckerFunc
	mu       sync.RWMutex
}

// NewSystemHealthChecker 创建系统健康检查器.
func NewSystemHealthChecker(monitor *Monitor, config *HealthCheckConfig) *SystemHealthChecker {
	if config == nil {
		config = DefaultHealthCheckConfig()
	}

	hc := &SystemHealthChecker{
		monitor:  monitor,
		config:   config,
		checkers: make(map[string]CheckerFunc),
	}

	// 注册内置检查器
	hc.RegisterChecker("cpu", hc.checkCPU)
	hc.RegisterChecker("memory", hc.checkMemory)
	hc.RegisterChecker("disk", hc.checkDisk)
	hc.RegisterChecker("network", hc.checkNetwork)
	hc.RegisterChecker("services", hc.checkServices)
	hc.RegisterChecker("temperature", hc.checkTemperature)
	hc.RegisterChecker("load", hc.checkLoad)

	return hc
}

// RegisterChecker 注册自定义组件检查器.
func (hc *SystemHealthChecker) RegisterChecker(name string, checker CheckerFunc) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checkers[name] = checker
}

// CheckAll 执行所有组件的健康检查，返回系统整体健康报告.
func (hc *SystemHealthChecker) CheckAll(ctx context.Context) *SystemHealth {
	start := time.Now()

	hc.mu.RLock()
	checkers := make(map[string]CheckerFunc, len(hc.checkers))
	for k, v := range hc.checkers {
		checkers[k] = v
	}
	hc.mu.RUnlock()

	var components []*ComponentHealth

	if hc.config.ParallelCheck {
		components = hc.checkParallel(ctx, checkers)
	} else {
		components = hc.checkSequential(ctx, checkers)
	}

	// 计算整体状态
	overallStatus := hc.calculateOverallStatus(components)
	summary := hc.buildSummary(components)

	return &SystemHealth{
		Status:     overallStatus,
		Message:    statusMessage(overallStatus),
		Hostname:   hc.monitor.GetHostname(),
		CheckedAt:  start,
		Duration:   time.Since(start).String(),
		Components: components,
		Summary:    summary,
	}
}

// CheckComponent 执行指定组件的健康检查.
func (hc *SystemHealthChecker) CheckComponent(ctx context.Context, name string) (*ComponentHealth, error) {
	hc.mu.RLock()
	checker, ok := hc.checkers[name]
	hc.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("未知的组件：%s", name)
	}

	return checker(ctx), nil
}

// GetOverallStatus 获取系统整体健康状态（快速检查，不含详细报告）.
func (hc *SystemHealthChecker) GetOverallStatus(ctx context.Context) HealthStatus {
	report := hc.checkParallel(ctx, map[string]CheckerFunc{
		"cpu":  hc.checkCPU,
		"load": hc.checkLoad,
	})
	return hc.calculateOverallStatus(report)
}

// HealthCheckHandler HTTP 处理器，返回 JSON 格式健康报告.
func (hc *SystemHealthChecker) HealthCheckHandler(c *gin.Context) {
	report := hc.CheckAll(c.Request.Context())

	// 根据健康状态设置 HTTP 状态码
	var httpStatus int
	switch report.Status {
	case StatusHealthy:
		httpStatus = http.StatusOK
	case StatusDegraded:
		httpStatus = http.StatusOK // 降级但可用，仍返回 200
	case StatusUnhealthy:
		httpStatus = http.StatusServiceUnavailable
	case StatusCritical:
		httpStatus = http.StatusServiceUnavailable
	default:
		httpStatus = http.StatusOK
	}

	c.JSON(httpStatus, report)
}

// ========== 内置检查器 ==========

// checkCPU 检查 CPU 使用率.
func (hc *SystemHealthChecker) checkCPU(ctx context.Context) *ComponentHealth {
	start := time.Now()
	ch := &ComponentHealth{
		Name:      "cpu",
		CheckedAt: start,
		Details:   make(map[string]interface{}),
	}

	stats, err := hc.monitor.GetSystemStats()
	if err != nil {
		ch.Status = StatusCritical
		ch.Message = fmt.Sprintf("无法获取 CPU 信息：%v", err)
		ch.Duration = time.Since(start).String()
		return ch
	}

	ch.Details["usage"] = stats.CPUUsage
	ch.Details["cores"] = stats.CPUCores

	switch {
	case stats.CPUUsage >= hc.config.CPUCriticalThreshold:
		ch.Status = StatusCritical
		ch.Message = fmt.Sprintf("CPU 使用率严重过高：%.1f%%（阈值 %.1f%%）", stats.CPUUsage, hc.config.CPUCriticalThreshold)
	case stats.CPUUsage >= hc.config.CPUWarningThreshold:
		ch.Status = StatusDegraded
		ch.Message = fmt.Sprintf("CPU 使用率偏高：%.1f%%（阈值 %.1f%%）", stats.CPUUsage, hc.config.CPUWarningThreshold)
	default:
		ch.Status = StatusHealthy
		ch.Message = fmt.Sprintf("CPU 使用率正常：%.1f%%", stats.CPUUsage)
	}

	ch.Duration = time.Since(start).String()
	return ch
}

// checkMemory 检查内存使用率.
func (hc *SystemHealthChecker) checkMemory(ctx context.Context) *ComponentHealth {
	start := time.Now()
	ch := &ComponentHealth{
		Name:      "memory",
		CheckedAt: start,
		Details:   make(map[string]interface{}),
	}

	stats, err := hc.monitor.GetSystemStats()
	if err != nil {
		ch.Status = StatusCritical
		ch.Message = fmt.Sprintf("无法获取内存信息：%v", err)
		ch.Duration = time.Since(start).String()
		return ch
	}

	ch.Details["usage"] = stats.MemoryUsage
	ch.Details["total"] = stats.MemoryTotal
	ch.Details["used"] = stats.MemoryUsed
	ch.Details["free"] = stats.MemoryFree
	ch.Details["swapUsage"] = stats.SwapUsage

	switch {
	case stats.MemoryUsage >= hc.config.MemCriticalThreshold:
		ch.Status = StatusCritical
		ch.Message = fmt.Sprintf("内存使用率严重过高：%.1f%%（阈值 %.1f%%）", stats.MemoryUsage, hc.config.MemCriticalThreshold)
	case stats.MemoryUsage >= hc.config.MemWarningThreshold:
		ch.Status = StatusDegraded
		ch.Message = fmt.Sprintf("内存使用率偏高：%.1f%%（阈值 %.1f%%）", stats.MemoryUsage, hc.config.MemWarningThreshold)
	default:
		ch.Status = StatusHealthy
		ch.Message = fmt.Sprintf("内存使用率正常：%.1f%%", stats.MemoryUsage)
	}

	// Swap 使用过高也标记为降级
	if ch.Status == StatusHealthy && stats.SwapUsage > 50 {
		ch.Status = StatusDegraded
		ch.Message = fmt.Sprintf("内存正常但 Swap 使用率偏高：%.1f%%", stats.SwapUsage)
	}

	ch.Duration = time.Since(start).String()
	return ch
}

// checkDisk 检查磁盘空间.
func (hc *SystemHealthChecker) checkDisk(ctx context.Context) *ComponentHealth {
	start := time.Now()
	ch := &ComponentHealth{
		Name:      "disk",
		CheckedAt: start,
		Details:   make(map[string]interface{}),
	}

	disks, err := hc.monitor.GetDiskStats(ctx)
	if err != nil {
		ch.Status = StatusCritical
		ch.Message = fmt.Sprintf("无法获取磁盘信息：%v", err)
		ch.Duration = time.Since(start).String()
		return ch
	}

	if len(disks) == 0 {
		ch.Status = StatusDegraded
		ch.Message = "未检测到磁盘"
		ch.Duration = time.Since(start).String()
		return ch
	}

	worstStatus := StatusHealthy
	var diskDetails []map[string]interface{}
	var problematicDisks []string

	for _, disk := range disks {
		diskInfo := map[string]interface{}{
			"device":       disk.Device,
			"mountPoint":   disk.MountPoint,
			"usagePercent": disk.UsagePercent,
			"total":        disk.Total,
			"free":         disk.Free,
		}
		diskDetails = append(diskDetails, diskInfo)

		var diskStatus HealthStatus
		switch {
		case disk.UsagePercent >= hc.config.DiskCriticalThreshold:
			diskStatus = StatusCritical
			problematicDisks = append(problematicDisks, fmt.Sprintf("%s(%.1f%%)", disk.MountPoint, disk.UsagePercent))
		case disk.UsagePercent >= hc.config.DiskWarningThreshold:
			diskStatus = StatusDegraded
			problematicDisks = append(problematicDisks, fmt.Sprintf("%s(%.1f%%)", disk.MountPoint, disk.UsagePercent))
		default:
			diskStatus = StatusHealthy
		}

		if statusOrder[diskStatus] > statusOrder[worstStatus] {
			worstStatus = diskStatus
		}
	}

	ch.Details["disks"] = diskDetails
	ch.Status = worstStatus

	switch worstStatus {
	case StatusCritical:
		ch.Message = fmt.Sprintf("磁盘空间严重不足：%s", strings.Join(problematicDisks, ", "))
	case StatusDegraded:
		ch.Message = fmt.Sprintf("磁盘空间偏高：%s", strings.Join(problematicDisks, ", "))
	default:
		ch.Message = fmt.Sprintf("磁盘状态正常（%d 个分区）", len(disks))
	}

	ch.Duration = time.Since(start).String()
	return ch
}

// checkNetwork 检查网络连通性.
func (hc *SystemHealthChecker) checkNetwork(ctx context.Context) *ComponentHealth {
	start := time.Now()
	ch := &ComponentHealth{
		Name:      "network",
		CheckedAt: start,
		Details:   make(map[string]interface{}),
	}

	hosts := hc.config.PingHosts
	if len(hosts) == 0 {
		hosts = []string{"8.8.8.8", "114.114.114.114"}
	}

	timeout := hc.config.PingTimeout
	if timeout <= 0 {
		timeout = 5
	}

	reachable := 0
	var unreachableHosts []string
	var hostResults []map[string]interface{}

	for _, host := range hosts {
		hostStart := time.Now()
		reachable_ := pingHost(host, timeout)
		elapsed := time.Since(hostStart)

		result := map[string]interface{}{
			"host":      host,
			"reachable": reachable_,
			"latency":   elapsed.String(),
		}
		hostResults = append(hostResults, result)

		if reachable_ {
			reachable++
		} else {
			unreachableHosts = append(unreachableHosts, host)
		}
	}

	ch.Details["hosts"] = hostResults
	ch.Details["reachable"] = reachable
	ch.Details["total"] = len(hosts)

	total := len(hosts)
	switch {
	case reachable == 0:
		ch.Status = StatusCritical
		ch.Message = "网络完全不可达"
	case reachable < total:
		ch.Status = StatusDegraded
		ch.Message = fmt.Sprintf("部分网络不可达：%s", strings.Join(unreachableHosts, ", "))
	default:
		ch.Status = StatusHealthy
		ch.Message = fmt.Sprintf("网络连通正常（%d/%d 可达）", reachable, total)
	}

	ch.Duration = time.Since(start).String()
	return ch
}

// checkServices 检查核心服务状态.
func (hc *SystemHealthChecker) checkServices(ctx context.Context) *ComponentHealth {
	start := time.Now()
	ch := &ComponentHealth{
		Name:      "services",
		CheckedAt: start,
		Details:   make(map[string]interface{}),
	}

	services := hc.config.CoreServices
	if len(services) == 0 {
		ch.Status = StatusHealthy
		ch.Message = "未配置核心服务检查"
		ch.Duration = time.Since(start).String()
		return ch
	}

	var serviceResults []map[string]interface{}
	var failedServices []string
	var stoppedServices []string

	for _, svc := range services {
		svcStatus := checkServiceStatus(ctx, svc)
		serviceResults = append(serviceResults, map[string]interface{}{
			"name":   svc,
			"status": svcStatus,
		})

		switch svcStatus {
		case "active", "running":
			// 正常
		case "inactive", "stopped", "dead":
			stoppedServices = append(stoppedServices, svc)
		default:
			failedServices = append(failedServices, svc)
		}
	}

	ch.Details["services"] = serviceResults

	switch {
	case len(failedServices) > 0:
		ch.Status = StatusCritical
		ch.Message = fmt.Sprintf("服务异常：%s", strings.Join(failedServices, ", "))
	case len(stoppedServices) > 0:
		ch.Status = StatusDegraded
		ch.Message = fmt.Sprintf("服务已停止：%s", strings.Join(stoppedServices, ", "))
	default:
		ch.Status = StatusHealthy
		ch.Message = fmt.Sprintf("所有服务运行正常（%d 个）", len(services))
	}

	ch.Duration = time.Since(start).String()
	return ch
}

// checkTemperature 检查系统温度.
func (hc *SystemHealthChecker) checkTemperature(ctx context.Context) *ComponentHealth {
	start := time.Now()
	ch := &ComponentHealth{
		Name:      "temperature",
		CheckedAt: start,
		Details:   make(map[string]interface{}),
	}

	stats, err := hc.monitor.GetSystemStats()
	if err != nil || stats.CPUTemp == 0 {
		ch.Status = StatusHealthy
		ch.Message = "温度传感器不可用或未检测到"
		ch.Duration = time.Since(start).String()
		return ch
	}

	temp := stats.CPUTemp
	ch.Details["cpuTemp"] = temp

	switch {
	case temp >= hc.config.TempCriticalThreshold:
		ch.Status = StatusCritical
		ch.Message = fmt.Sprintf("CPU 温度过高：%d°C（阈值 %d°C）", temp, hc.config.TempCriticalThreshold)
	case temp >= hc.config.TempWarningThreshold:
		ch.Status = StatusDegraded
		ch.Message = fmt.Sprintf("CPU 温度偏高：%d°C（阈值 %d°C）", temp, hc.config.TempWarningThreshold)
	default:
		ch.Status = StatusHealthy
		ch.Message = fmt.Sprintf("CPU 温度正常：%d°C", temp)
	}

	ch.Duration = time.Since(start).String()
	return ch
}

// checkLoad 检查系统负载.
func (hc *SystemHealthChecker) checkLoad(ctx context.Context) *ComponentHealth {
	start := time.Now()
	ch := &ComponentHealth{
		Name:      "load",
		CheckedAt: start,
		Details:   make(map[string]interface{}),
	}

	stats, err := hc.monitor.GetSystemStats()
	if err != nil {
		ch.Status = StatusCritical
		ch.Message = fmt.Sprintf("无法获取负载信息：%v", err)
		ch.Duration = time.Since(start).String()
		return ch
	}

	if len(stats.LoadAvg) < 3 {
		ch.Status = StatusHealthy
		ch.Message = "负载数据不可用"
		ch.Duration = time.Since(start).String()
		return ch
	}

	load1 := stats.LoadAvg[0]
	load5 := stats.LoadAvg[1]
	load15 := stats.LoadAvg[2]
	cores := float64(stats.CPUCores)

	ch.Details["load1"] = load1
	ch.Details["load5"] = load5
	ch.Details["load15"] = load15
	ch.Details["cores"] = stats.CPUCores

	warnThreshold := cores * hc.config.LoadWarningMultiplier
	critThreshold := cores * hc.config.LoadCriticalMultiplier

	switch {
	case load1 >= critThreshold:
		ch.Status = StatusCritical
		ch.Message = fmt.Sprintf("系统负载严重过高：%.2f（阈值 %.2f，%d 核）", load1, critThreshold, stats.CPUCores)
	case load1 >= warnThreshold:
		ch.Status = StatusDegraded
		ch.Message = fmt.Sprintf("系统负载偏高：%.2f（阈值 %.2f，%d 核）", load1, warnThreshold, stats.CPUCores)
	default:
		ch.Status = StatusHealthy
		ch.Message = fmt.Sprintf("系统负载正常：%.2f（%d 核）", load1, stats.CPUCores)
	}

	ch.Duration = time.Since(start).String()
	return ch
}

// ========== 内部辅助方法 ==========

// checkParallel 并行执行组件检查.
func (hc *SystemHealthChecker) checkParallel(ctx context.Context, checkers map[string]CheckerFunc) []*ComponentHealth {
	results := make([]*ComponentHealth, 0, len(checkers))
	ch := make(chan *ComponentHealth, len(checkers))

	for _, checker := range checkers {
		go func(fn CheckerFunc) {
			ch <- fn(ctx)
		}(checker)
	}

	for range checkers {
		results = append(results, <-ch)
	}

	return results
}

// checkSequential 顺序执行组件检查.
func (hc *SystemHealthChecker) checkSequential(ctx context.Context, checkers map[string]CheckerFunc) []*ComponentHealth {
	results := make([]*ComponentHealth, 0, len(checkers))
	for _, checker := range checkers {
		results = append(results, checker(ctx))
	}
	return results
}

// calculateOverallStatus 根据各组件状态计算整体状态.
func (hc *SystemHealthChecker) calculateOverallStatus(components []*ComponentHealth) HealthStatus {
	if len(components) == 0 {
		return StatusHealthy
	}

	// 存在任何 Critical 则整体 Critical
	// 存在任何 Unhealthy 则整体 Unhealthy
	// 存在任何 Degraded 则整体 Degraded
	// 全部 Healthy 则整体 Healthy
	overall := StatusHealthy
	for _, comp := range components {
		if statusOrder[comp.Status] > statusOrder[overall] {
			overall = comp.Status
		}
	}

	return overall
}

// buildSummary 构建各状态数量统计.
func (hc *SystemHealthChecker) buildSummary(components []*ComponentHealth) map[string]int {
	summary := map[string]int{
		string(StatusHealthy):   0,
		string(StatusDegraded):  0,
		string(StatusUnhealthy): 0,
		string(StatusCritical):  0,
	}
	for _, comp := range components {
		summary[string(comp.Status)]++
	}
	return summary
}

// statusMessage 返回状态描述消息.
func statusMessage(status HealthStatus) string {
	switch status {
	case StatusHealthy:
		return "系统运行正常"
	case StatusDegraded:
		return "系统部分功能降级"
	case StatusUnhealthy:
		return "系统不健康，多个组件异常"
	case StatusCritical:
		return "系统严重异常，需要立即处理"
	default:
		return "未知状态"
	}
}

// pingHost 检测网络连通性（使用 TCP 连接代替 ICMP，避免需要 root 权限）.
func pingHost(host string, timeoutSec int) bool {
	// 如果是纯 IP 地址，尝试 TCP 连接常见端口
	ports := []string{"53", "80", "443"}
	for _, port := range ports {
		addr := net.JoinHostPort(host, port)
		conn, err := net.DialTimeout("tcp", addr, time.Duration(timeoutSec)*time.Second)
		if err == nil {
			_ = conn.Close()
			return true
		}
	}

	// 尝试直接解析域名
	if _, err := net.LookupHost(host); err == nil {
		return true
	}

	return false
}

// checkServiceStatus 检查服务运行状态.
func checkServiceStatus(ctx context.Context, service string) string {
	// 尝试 systemctl（systemd 系统）
	cmd := exec.CommandContext(ctx, "systemctl", "is-active", service)
	output, err := cmd.Output()
	if err == nil {
		status := strings.TrimSpace(string(output))
		if status != "" {
			return status
		}
	}

	// 尝试 service 命令
	cmd = exec.CommandContext(ctx, "service", service, "status")
	output, err = cmd.Output()
	if err == nil {
		outputStr := string(output)
		if strings.Contains(outputStr, "running") || strings.Contains(outputStr, "is running") {
			return "running"
		}
		if strings.Contains(outputStr, "stopped") || strings.Contains(outputStr, "is not running") {
			return "stopped"
		}
	}

	// 尝试检查进程是否存在
	cmd = exec.CommandContext(ctx, "pgrep", "-x", service)
	if err := cmd.Run(); err == nil {
		return "running"
	}

	return "unknown"
}
