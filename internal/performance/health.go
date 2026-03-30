package performance

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// HealthStatus 健康状态.
type HealthStatus string

// 健康状态常量，表示系统或组件的运行状态。
const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// HealthCheckResult 健康检查结果.
type HealthCheckResult struct {
	Name      string       `json:"name"`
	Status    HealthStatus `json:"status"`
	Message   string       `json:"message,omitempty"`
	Details   interface{}  `json:"details,omitempty"`
	Duration  int64        `json:"duration_ms"`
	Timestamp time.Time    `json:"timestamp"`
}

// SystemHealth 系统健康状态.
type SystemHealth struct {
	Status      HealthStatus        `json:"status"`
	Score       int                 `json:"score"` // 0-100
	Checks      []HealthCheckResult `json:"checks"`
	Issues      []string            `json:"issues,omitempty"`
	Uptime      uint64              `json:"uptime_seconds"`
	LastChecked time.Time           `json:"last_checked"`
	Version     string              `json:"version"`
}

// HealthChecker 健康检查器.
type HealthChecker struct {
	logger    *zap.Logger
	collector *SystemCollector
	storage   *StorageCollector
	mu        sync.RWMutex

	// 检查项
	checks     []HealthCheck
	interval   time.Duration
	lastResult *SystemHealth

	// 阈值配置
	thresholds HealthThresholds
}

// HealthCheck 健康检查函数.
type HealthCheck struct {
	Name  string
	Check func() HealthCheckResult
}

// HealthThresholds 健康检查阈值.
type HealthThresholds struct {
	CPUWarningPercent     float64 `json:"cpu_warning_percent"`
	CPUCriticalPercent    float64 `json:"cpu_critical_percent"`
	MemoryWarningPercent  float64 `json:"memory_warning_percent"`
	MemoryCriticalPercent float64 `json:"memory_critical_percent"`
	DiskWarningPercent    float64 `json:"disk_warning_percent"`
	DiskCriticalPercent   float64 `json:"disk_critical_percent"`
	DiskLatencyMs         float64 `json:"disk_latency_ms"`
}

// DefaultHealthThresholds 默认阈值.
func DefaultHealthThresholds() HealthThresholds {
	return HealthThresholds{
		CPUWarningPercent:     80,
		CPUCriticalPercent:    95,
		MemoryWarningPercent:  85,
		MemoryCriticalPercent: 95,
		DiskWarningPercent:    85,
		DiskCriticalPercent:   95,
		DiskLatencyMs:         50,
	}
}

// NewHealthChecker 创建健康检查器.
func NewHealthChecker(
	logger *zap.Logger,
	collector *SystemCollector,
	storage *StorageCollector,
) *HealthChecker {
	hc := &HealthChecker{
		logger:     logger,
		collector:  collector,
		storage:    storage,
		thresholds: DefaultHealthThresholds(),
		interval:   30 * time.Second,
	}

	// 注册默认检查项
	hc.registerDefaultChecks()

	return hc
}

// registerDefaultChecks 注册默认健康检查项.
func (hc *HealthChecker) registerDefaultChecks() {
	hc.checks = []HealthCheck{
		{Name: "cpu", Check: hc.checkCPU},
		{Name: "memory", Check: hc.checkMemory},
		{Name: "disk", Check: hc.checkDisk},
		{Name: "disk_io", Check: hc.checkDiskIO},
		{Name: "network", Check: hc.checkNetwork},
		{Name: "services", Check: hc.checkServices},
		{Name: "uptime", Check: hc.checkUptime},
		{Name: "btrfs", Check: hc.checkBtrfs},
		{Name: "shares", Check: hc.checkShares},
	}
}

// SetThresholds 设置阈值.
func (hc *HealthChecker) SetThresholds(thresholds HealthThresholds) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.thresholds = thresholds
}

// Run 执行所有健康检查.
func (hc *HealthChecker) Run() *SystemHealth {
	start := time.Now()

	health := &SystemHealth{
		Status:      HealthStatusHealthy,
		Score:       100,
		LastChecked: start,
	}

	// 执行所有检查
	for _, check := range hc.checks {
		result := check.Check()
		health.Checks = append(health.Checks, result)

		// 更新整体状态
		switch result.Status {
		case HealthStatusUnhealthy:
			if health.Status != HealthStatusUnhealthy {
				health.Status = HealthStatusUnhealthy
			}
		case HealthStatusDegraded:
			if health.Status == HealthStatusHealthy {
				health.Status = HealthStatusDegraded
			}
		}

		// 扣减分数
		switch result.Status {
		case HealthStatusDegraded:
			health.Score -= 10
		case HealthStatusUnhealthy:
			health.Score -= 25
		}

		if result.Message != "" {
			health.Issues = append(health.Issues, result.Message)
		}
	}

	// 确保分数在合理范围
	if health.Score < 0 {
		health.Score = 0
	}

	health.Uptime = hc.collector.getUptime()

	hc.mu.Lock()
	hc.lastResult = health
	hc.mu.Unlock()

	return health
}

// Start 启动定期健康检查.
func (hc *HealthChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(hc.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				hc.Run()
			}
		}
	}()
}

// GetHealth 获取最新的健康状态.
func (hc *HealthChecker) GetHealth() *SystemHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	if hc.lastResult == nil {
		return hc.Run()
	}
	return hc.lastResult
}

// 单项检查函数

func (hc *HealthChecker) checkCPU() HealthCheckResult {
	start := time.Now()
	metric := hc.collector.collectCPU()

	result := HealthCheckResult{
		Name:      "cpu",
		Timestamp: start,
		Details:   metric,
	}

	hc.mu.RLock()
	thresholds := hc.thresholds
	hc.mu.RUnlock()

	if metric.UsagePercent >= thresholds.CPUCriticalPercent {
		result.Status = HealthStatusUnhealthy
		result.Message = fmt.Sprintf("CPU 使用率过高: %.1f%% (阈值: %.1f%%)", metric.UsagePercent, thresholds.CPUCriticalPercent)
	} else if metric.UsagePercent >= thresholds.CPUWarningPercent {
		result.Status = HealthStatusDegraded
		result.Message = fmt.Sprintf("CPU 使用率偏高: %.1f%% (警告阈值: %.1f%%)", metric.UsagePercent, thresholds.CPUWarningPercent)
	} else {
		result.Status = HealthStatusHealthy
	}

	result.Duration = time.Since(start).Milliseconds()
	return result
}

func (hc *HealthChecker) checkMemory() HealthCheckResult {
	start := time.Now()
	metric := hc.collector.collectMemory()

	result := HealthCheckResult{
		Name:      "memory",
		Timestamp: start,
		Details:   metric,
	}

	hc.mu.RLock()
	thresholds := hc.thresholds
	hc.mu.RUnlock()

	if metric.UsagePercent >= thresholds.MemoryCriticalPercent {
		result.Status = HealthStatusUnhealthy
		result.Message = fmt.Sprintf("内存使用率过高: %.1f%% (阈值: %.1f%%)", metric.UsagePercent, thresholds.MemoryCriticalPercent)
	} else if metric.UsagePercent >= thresholds.MemoryWarningPercent {
		result.Status = HealthStatusDegraded
		result.Message = fmt.Sprintf("内存使用率偏高: %.1f%% (警告阈值: %.1f%%)", metric.UsagePercent, thresholds.MemoryWarningPercent)
	} else {
		result.Status = HealthStatusHealthy
	}

	result.Duration = time.Since(start).Milliseconds()
	return result
}

func (hc *HealthChecker) checkDisk() HealthCheckResult {
	start := time.Now()
	metrics := hc.collector.collectDisks()

	result := HealthCheckResult{
		Name:      "disk",
		Timestamp: start,
		Status:    HealthStatusHealthy,
		Details:   metrics,
	}

	hc.mu.RLock()
	thresholds := hc.thresholds
	hc.mu.RUnlock()

	var issues []string
	for _, m := range metrics {
		if m.UsagePercent >= thresholds.DiskCriticalPercent {
			result.Status = HealthStatusUnhealthy
			issues = append(issues, fmt.Sprintf("磁盘 %s 使用率过高: %.1f%%", m.Device, m.UsagePercent))
		} else if m.UsagePercent >= thresholds.DiskWarningPercent {
			if result.Status == HealthStatusHealthy {
				result.Status = HealthStatusDegraded
			}
			issues = append(issues, fmt.Sprintf("磁盘 %s 使用率偏高: %.1f%%", m.Device, m.UsagePercent))
		}
	}

	if len(issues) > 0 {
		result.Message = issues[0]
		if len(issues) > 1 {
			result.Details = issues
		}
	}

	result.Duration = time.Since(start).Milliseconds()
	return result
}

func (hc *HealthChecker) checkDiskIO() HealthCheckResult {
	start := time.Now()
	metrics := hc.collector.collectDiskIO()

	result := HealthCheckResult{
		Name:      "disk_io",
		Timestamp: start,
		Status:    HealthStatusHealthy,
		Details:   metrics,
	}

	hc.mu.RLock()
	thresholds := hc.thresholds
	hc.mu.RUnlock()

	var issues []string
	for _, m := range metrics {
		avgLatency := (m.ReadLatency + m.WriteLatency) / 2
		if avgLatency > thresholds.DiskLatencyMs {
			if result.Status == HealthStatusHealthy {
				result.Status = HealthStatusDegraded
			}
			issues = append(issues, fmt.Sprintf("磁盘 %s 延迟过高: %.1fms", m.Device, avgLatency))
		}
	}

	if len(issues) > 0 {
		result.Message = issues[0]
	}

	result.Duration = time.Since(start).Milliseconds()
	return result
}

func (hc *HealthChecker) checkNetwork() HealthCheckResult {
	start := time.Now()
	metrics := hc.collector.collectNetwork()

	result := HealthCheckResult{
		Name:      "network",
		Timestamp: start,
		Status:    HealthStatusHealthy,
		Details:   metrics,
	}

	for _, m := range metrics {
		// 检查网络错误
		if m.RXErrors > 0 || m.TXErrors > 0 {
			result.Status = HealthStatusDegraded
			result.Message = fmt.Sprintf("网络接口 %s 存在错误", m.Interface)
			break
		}
		if m.RXDropped > 100 || m.TXDropped > 100 {
			result.Status = HealthStatusDegraded
			result.Message = fmt.Sprintf("网络接口 %s 存在丢包", m.Interface)
			break
		}
	}

	result.Duration = time.Since(start).Milliseconds()
	return result
}

func (hc *HealthChecker) checkServices() HealthCheckResult {
	start := time.Now()

	result := HealthCheckResult{
		Name:      "services",
		Timestamp: start,
		Status:    HealthStatusHealthy,
		Details:   make(map[string]string),
	}

	services := []string{"nasd", "docker"}
	serviceStatus := make(map[string]string)

	for _, svc := range services {
		status := hc.checkServiceStatus(svc)
		serviceStatus[svc] = status
		if status != "running" && status != "active" {
			result.Status = HealthStatusDegraded
			result.Message = fmt.Sprintf("服务 %s 状态异常: %s", svc, status)
		}
	}

	result.Details = serviceStatus
	result.Duration = time.Since(start).Milliseconds()
	return result
}

func (hc *HealthChecker) checkServiceStatus(service string) string {
	// 简单的进程检查
	var procPath string
	switch service {
	case "nasd":
		procPath = "/proc/self/status"
	default:
		return "unknown"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:8080/api/v1/health", nil)
	if err == nil {
		client := &http.Client{Timeout: 5 * time.Second}
		if resp, err := client.Do(req); err == nil {
			defer func() { _ = resp.Body.Close() }()
			return "running"
		}
	}

	_ = procPath // 避免未使用警告
	return "unknown"
}

func (hc *HealthChecker) checkUptime() HealthCheckResult {
	start := time.Now()
	uptime := hc.collector.getUptime()

	result := HealthCheckResult{
		Name:      "uptime",
		Timestamp: start,
		Status:    HealthStatusHealthy,
		Details: map[string]interface{}{
			"uptime_seconds": uptime,
			"uptime_human":   formatUptime(uptime),
		},
	}

	// 如果刚启动，可能需要额外检查
	if uptime < 60 {
		result.Message = "系统刚启动"
	}

	result.Duration = time.Since(start).Milliseconds()
	return result
}

// checkBtrfs 检查 Btrfs 文件系统健康状态.
func (hc *HealthChecker) checkBtrfs() HealthCheckResult {
	start := time.Now()

	result := HealthCheckResult{
		Name:      "btrfs",
		Timestamp: start,
		Status:    HealthStatusHealthy,
		Details:   make(map[string]interface{}),
	}

	details, ok := result.Details.(map[string]interface{})
	if !ok {
		details = make(map[string]interface{})
		result.Details = details
	}
	var issues []string

	// 检查 /proc/mounts 中的 btrfs 挂载
	if file, err := os.Open("/proc/mounts"); err == nil {
		defer func() { _ = file.Close() }()
		scanner := bufio.NewScanner(file)
		btrfsMounts := []string{}

		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "btrfs") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					btrfsMounts = append(btrfsMounts, fields[1])
				}
			}
		}

		details["mounts"] = btrfsMounts

		// 检查每个 btrfs 卷的状态
		for _, mount := range btrfsMounts {
			// 尝试读取 btrfs 设备状态
			deviceStatsPath := mount + "/.btrfs_device_stats"
			if _, err := os.Stat(deviceStatsPath); err == nil {
				// 如果存在设备统计文件，检查是否有错误
				if data, err := os.ReadFile(deviceStatsPath); err == nil {
					stats := string(data)
					if strings.Contains(stats, "write_io_errs") &&
						!strings.Contains(stats, "write_io_errs 0") {
						result.Status = HealthStatusDegraded
						issues = append(issues, fmt.Sprintf("Btrfs 卷 %s 存在 I/O 错误", mount))
					}
				}
			}
		}
	}

	// 检查 btrfs scrub 状态（如果存在）
	scrubPath := "/var/lib/btrfs/scrub.status"
	if _, err := os.Stat(scrubPath); err == nil {
		details["scrub_available"] = true
	}

	if len(issues) > 0 {
		result.Message = issues[0]
		details["issues"] = issues
	}

	result.Duration = time.Since(start).Milliseconds()
	return result
}

// checkShares 检查共享服务状态.
func (hc *HealthChecker) checkShares() HealthCheckResult {
	start := time.Now()

	result := HealthCheckResult{
		Name:      "shares",
		Timestamp: start,
		Status:    HealthStatusHealthy,
		Details:   make(map[string]interface{}),
	}

	details, ok := result.Details.(map[string]interface{})
	if !ok {
		details = make(map[string]interface{})
		result.Details = details
	}
	var issues []string

	// 检查 SMB 服务
	smbStatus := "unknown"
	if output, err := exec.CommandContext(context.Background(), "systemctl", "is-active", "smbd").Output(); err == nil {
		smbStatus = strings.TrimSpace(string(output))
	}
	details["smb_status"] = smbStatus

	if smbStatus != "active" && smbStatus != "running" {
		result.Status = HealthStatusDegraded
		issues = append(issues, "SMB 服务未运行")
	}

	// 检查 NFS 服务
	nfsStatus := "unknown"
	if output, err := exec.CommandContext(context.Background(), "systemctl", "is-active", "nfs-server").Output(); err == nil {
		nfsStatus = strings.TrimSpace(string(output))
	}
	details["nfs_status"] = nfsStatus

	if nfsStatus != "active" && nfsStatus != "running" {
		// NFS 未运行不一定是问题，可能未配置
		details["nfs_note"] = "NFS 服务未运行，可能未配置"
	}

	// 检查共享目录是否存在
	sharePaths := []string{"/mnt", "/srv"}
	accessibleShares := 0
	for _, path := range sharePaths {
		if _, err := os.Stat(path); err == nil {
			accessibleShares++
		}
	}
	details["accessible_mounts"] = accessibleShares

	if len(issues) > 0 {
		result.Message = issues[0]
		details["issues"] = issues
	}

	result.Duration = time.Since(start).Milliseconds()
	return result
}

// AddCheck 添加自定义检查.
func (hc *HealthChecker) AddCheck(name string, check func() HealthCheckResult) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checks = append(hc.checks, HealthCheck{Name: name, Check: check})
}

// RemoveCheck 移除检查.
func (hc *HealthChecker) RemoveCheck(name string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	for i, c := range hc.checks {
		if c.Name == name {
			hc.checks = append(hc.checks[:i], hc.checks[i+1:]...)
			break
		}
	}
}

// formatUptime 格式化运行时间.
func formatUptime(seconds uint64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	mins := (seconds % 3600) / 60

	if days > 0 {
		return fmt.Sprintf("%d天 %d小时 %d分钟", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%d小时 %d分钟", hours, mins)
	}
	return fmt.Sprintf("%d分钟", mins)
}
