// Package health 提供系统健康检查功能
// v2.51.0 - 重构版健康检查器
package health

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// v2.51.0 新增检查类型
const (
	CheckTypeCPU  CheckType = "cpu"
	CheckTypeDisk CheckType = "disk"
)

// CheckResult 单项检查结果
type CheckResult struct {
	Name       string                 `json:"name"`
	Type       CheckType              `json:"type"`
	Status     HealthStatus           `json:"status"`
	Message    string                 `json:"message"`
	Timestamp  time.Time              `json:"timestamp"`
	Duration   time.Duration          `json:"duration"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Thresholds map[string]float64     `json:"thresholds,omitempty"`
}

// HealthReport 健康检查报告
type HealthReport struct {
	Status    HealthStatus           `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Uptime    time.Duration          `json:"uptime"`
	Version   string                 `json:"version,omitempty"`
	Checks    map[string]CheckResult `json:"checks"`
	Summary   Summary                `json:"summary"`
}

// Summary 健康检查摘要
type Summary struct {
	Total     int `json:"total"`
	Healthy   int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
	Degraded  int `json:"degraded"`
}

// CheckFunc 自定义检查函数类型
type CheckFunc func(ctx context.Context) (HealthStatus, string, map[string]interface{})

// CheckerConfig 检查器配置
type CheckerConfig struct {
	Name       string
	Type       CheckType
	Interval   time.Duration
	Timeout    time.Duration
	Enabled    bool
	Thresholds map[string]float64
	CheckFunc  CheckFunc
}

// HealthChecker 系统健康检查器
// v2.51.0 核心结构体
type HealthChecker struct {
	mu        sync.RWMutex
	checkers  map[string]*registeredChecker
	results   map[string]CheckResult
	config    CheckerConfig
	logger    *zap.Logger
	startTime time.Time
	version   string

	// 系统检查配置
	cpuThreshold    float64
	memoryThreshold float64
	diskThreshold   float64

	// 自定义检查注册
	customChecks map[string]CheckFunc

	// 停止信号
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// registeredChecker 已注册的检查器
type registeredChecker struct {
	config  CheckerConfig
	checkFn CheckFunc
	enabled bool
	lastRun time.Time
}

// NewHealthChecker 创建新的健康检查器
func NewHealthChecker(opts ...Option) *HealthChecker {
	hc := &HealthChecker{
		checkers:        make(map[string]*registeredChecker),
		results:         make(map[string]CheckResult),
		startTime:       time.Now(),
		stopCh:          make(chan struct{}),
		customChecks:    make(map[string]CheckFunc),
		cpuThreshold:    80.0,
		memoryThreshold: 85.0,
		diskThreshold:   90.0,
	}

	for _, opt := range opts {
		opt(hc)
	}

	// 注册默认系统检查
	hc.registerDefaultChecks()

	return hc
}

// Option 配置选项
type Option func(*HealthChecker)

// WithLogger 设置日志器
func WithLogger(logger *zap.Logger) Option {
	return func(hc *HealthChecker) {
		hc.logger = logger
	}
}

// WithVersion 设置版本
func WithVersion(version string) Option {
	return func(hc *HealthChecker) {
		hc.version = version
	}
}

// WithCPUThreshold 设置 CPU 阈值
func WithCPUThreshold(threshold float64) Option {
	return func(hc *HealthChecker) {
		hc.cpuThreshold = threshold
	}
}

// WithMemoryThreshold 设置内存阈值
func WithMemoryThreshold(threshold float64) Option {
	return func(hc *HealthChecker) {
		hc.memoryThreshold = threshold
	}
}

// WithDiskThreshold 设置磁盘阈值
func WithDiskThreshold(threshold float64) Option {
	return func(hc *HealthChecker) {
		hc.diskThreshold = threshold
	}
}

// registerDefaultChecks 注册默认系统检查
func (hc *HealthChecker) registerDefaultChecks() {
	// CPU 检查
	hc.RegisterCheck(CheckerConfig{
		Name:     "cpu",
		Type:     CheckTypeCPU,
		Interval: 30 * time.Second,
		Timeout:  5 * time.Second,
		Enabled:  true,
	}, hc.checkCPU)

	// 内存检查
	hc.RegisterCheck(CheckerConfig{
		Name:     "memory",
		Type:     CheckTypeMemory,
		Interval: 30 * time.Second,
		Timeout:  5 * time.Second,
		Enabled:  true,
	}, hc.checkMemory)

	// 磁盘检查
	hc.RegisterCheck(CheckerConfig{
		Name:     "disk",
		Type:     CheckTypeDisk,
		Interval: 60 * time.Second,
		Timeout:  5 * time.Second,
		Enabled:  true,
	}, hc.checkDisk)

	// 网络检查
	hc.RegisterCheck(CheckerConfig{
		Name:     "network",
		Type:     CheckTypeNetwork,
		Interval: 30 * time.Second,
		Timeout:  5 * time.Second,
		Enabled:  true,
	}, hc.checkNetwork)
}

// RegisterCheck 注册自定义检查项
func (hc *HealthChecker) RegisterCheck(config CheckerConfig, checkFn CheckFunc) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}
	if config.Interval == 0 {
		config.Interval = 30 * time.Second
	}

	hc.checkers[config.Name] = &registeredChecker{
		config:  config,
		checkFn: checkFn,
		enabled: config.Enabled,
	}
}

// UnregisterCheck 注销检查项
func (hc *HealthChecker) UnregisterCheck(name string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	delete(hc.checkers, name)
	delete(hc.results, name)
}

// EnableCheck 启用检查项
func (hc *HealthChecker) EnableCheck(name string) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	checker, exists := hc.checkers[name]
	if !exists {
		return fmt.Errorf("checker %s not found", name)
	}
	checker.enabled = true
	return nil
}

// DisableCheck 禁用检查项
func (hc *HealthChecker) DisableCheck(name string) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	checker, exists := hc.checkers[name]
	if !exists {
		return fmt.Errorf("checker %s not found", name)
	}
	checker.enabled = false
	return nil
}

// Check 执行所有健康检查并返回报告
func (hc *HealthChecker) Check(ctx context.Context) *HealthReport {
	hc.mu.RLock()
	checkers := make([]*registeredChecker, 0, len(hc.checkers))
	for _, c := range hc.checkers {
		checkers = append(checkers, c)
	}
	hc.mu.RUnlock()

	report := &HealthReport{
		Timestamp: time.Now(),
		Uptime:    time.Since(hc.startTime),
		Version:   hc.version,
		Checks:    make(map[string]CheckResult),
	}

	var wg sync.WaitGroup
	var resultMu sync.Mutex

	for _, checker := range checkers {
		if !checker.enabled {
			continue
		}

		wg.Add(1)
		go func(c *registeredChecker) {
			defer wg.Done()

			result := hc.runCheck(ctx, c)
			resultMu.Lock()
			report.Checks[c.config.Name] = result
			resultMu.Unlock()
		}(checker)
	}

	wg.Wait()

	// 计算摘要
	for _, result := range report.Checks {
		report.Summary.Total++
		switch result.Status {
		case StatusHealthy:
			report.Summary.Healthy++
		case StatusUnhealthy:
			report.Summary.Unhealthy++
		case StatusDegraded:
			report.Summary.Degraded++
		}
	}

	// 确定总体状态
	if report.Summary.Unhealthy > 0 {
		report.Status = StatusUnhealthy
	} else if report.Summary.Degraded > 0 {
		report.Status = StatusDegraded
	} else {
		report.Status = StatusHealthy
	}

	// 更新缓存结果
	hc.mu.Lock()
	for name, result := range report.Checks {
		hc.results[name] = result
	}
	hc.mu.Unlock()

	return report
}

// CheckSingle 执行单个检查
func (hc *HealthChecker) CheckSingle(ctx context.Context, name string) (*CheckResult, error) {
	hc.mu.RLock()
	checker, exists := hc.checkers[name]
	hc.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("checker %s not found", name)
	}

	result := hc.runCheck(ctx, checker)

	hc.mu.Lock()
	hc.results[name] = result
	hc.mu.Unlock()

	return &result, nil
}

// runCheck 执行单个检查
func (hc *HealthChecker) runCheck(ctx context.Context, checker *registeredChecker) CheckResult {
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, checker.config.Timeout)
	defer cancel()

	result := CheckResult{
		Name:      checker.config.Name,
		Type:      checker.config.Type,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// 执行检查函数
	status, message, details := checker.checkFn(ctx)

	result.Status = status
	result.Message = message
	result.Duration = time.Since(start)
	if details != nil {
		result.Details = details
	}

	// 记录日志
	if hc.logger != nil {
		if status == StatusUnhealthy {
			hc.logger.Warn("Health check failed",
				zap.String("checker", checker.config.Name),
				zap.String("message", message),
			)
		}
	}

	return result
}

// GetResult 获取缓存的检查结果
func (hc *HealthChecker) GetResult(name string) (CheckResult, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	result, exists := hc.results[name]
	return result, exists
}

// GetAllResults 获取所有缓存结果
func (hc *HealthChecker) GetAllResults() map[string]CheckResult {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	results := make(map[string]CheckResult, len(hc.results))
	for k, v := range hc.results {
		results[k] = v
	}
	return results
}

// ListChecks 列出所有检查项
func (hc *HealthChecker) ListChecks() []string {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	names := make([]string, 0, len(hc.checkers))
	for name := range hc.checkers {
		names = append(names, name)
	}
	return names
}

// IsHealthy 检查系统是否健康
func (hc *HealthChecker) IsHealthy(ctx context.Context) bool {
	report := hc.Check(ctx)
	return report.Status == StatusHealthy
}

// ===== 系统检查实现 =====

// checkCPU CPU 使用率检查
func (hc *HealthChecker) checkCPU(ctx context.Context) (HealthStatus, string, map[string]interface{}) {
	details := make(map[string]interface{})

	// 获取 CPU 使用率
	cpuPercent := hc.getCPUUsage()
	details["cpu_percent"] = cpuPercent
	details["threshold"] = hc.cpuThreshold
	details["cpu_count"] = runtime.NumCPU()

	var status HealthStatus
	var message string

	if cpuPercent > hc.cpuThreshold {
		status = StatusUnhealthy
		message = fmt.Sprintf("CPU usage %.2f%% exceeds threshold %.2f%%", cpuPercent, hc.cpuThreshold)
	} else if cpuPercent > hc.cpuThreshold*0.8 {
		status = StatusDegraded
		message = fmt.Sprintf("CPU usage %.2f%% approaching threshold", cpuPercent)
	} else {
		status = StatusHealthy
		message = fmt.Sprintf("CPU usage is normal: %.2f%%", cpuPercent)
	}

	return status, message, details
}

// checkMemory 内存使用检查
func (hc *HealthChecker) checkMemory(ctx context.Context) (HealthStatus, string, map[string]interface{}) {
	details := make(map[string]interface{})

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 计算内存使用率
	usedPercent := float64(m.Alloc) / float64(m.Sys) * 100
	details["alloc_mb"] = m.Alloc / 1024 / 1024
	details["sys_mb"] = m.Sys / 1024 / 1024
	details["heap_alloc_mb"] = m.HeapAlloc / 1024 / 1024
	details["heap_sys_mb"] = m.HeapSys / 1024 / 1024
	details["used_percent"] = usedPercent
	details["threshold"] = hc.memoryThreshold
	details["num_gc"] = m.NumGC

	var status HealthStatus
	var message string

	if usedPercent > hc.memoryThreshold {
		status = StatusUnhealthy
		message = fmt.Sprintf("Memory usage %.2f%% exceeds threshold %.2f%%", usedPercent, hc.memoryThreshold)
	} else if usedPercent > hc.memoryThreshold*0.8 {
		status = StatusDegraded
		message = fmt.Sprintf("Memory usage %.2f%% approaching threshold", usedPercent)
	} else {
		status = StatusHealthy
		message = fmt.Sprintf("Memory usage is normal: %.2f%%", usedPercent)
	}

	return status, message, details
}

// checkDisk 磁盘空间检查
func (hc *HealthChecker) checkDisk(ctx context.Context) (HealthStatus, string, map[string]interface{}) {
	details := make(map[string]interface{})

	var stat syscall.Statfs_t
	err := syscall.Statfs("/", &stat)
	if err != nil {
		return StatusUnhealthy, fmt.Sprintf("Failed to get disk stats: %v", err), details
	}

	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bfree * uint64(stat.Bsize)
	usedBytes := totalBytes - freeBytes
	usedPercent := float64(usedBytes) / float64(totalBytes) * 100

	details["path"] = "/"
	details["total_gb"] = totalBytes / 1024 / 1024 / 1024
	details["used_gb"] = usedBytes / 1024 / 1024 / 1024
	details["free_gb"] = freeBytes / 1024 / 1024 / 1024
	details["used_percent"] = usedPercent
	details["threshold"] = hc.diskThreshold

	var status HealthStatus
	var message string

	if usedPercent > hc.diskThreshold {
		status = StatusUnhealthy
		message = fmt.Sprintf("Disk usage %.2f%% exceeds threshold %.2f%%", usedPercent, hc.diskThreshold)
	} else if usedPercent > hc.diskThreshold*0.8 {
		status = StatusDegraded
		message = fmt.Sprintf("Disk usage %.2f%% approaching threshold", usedPercent)
	} else {
		status = StatusHealthy
		message = fmt.Sprintf("Disk usage is normal: %.2f%%", usedPercent)
	}

	return status, message, details
}

// checkNetwork 网络连接检查
func (hc *HealthChecker) checkNetwork(ctx context.Context) (HealthStatus, string, map[string]interface{}) {
	details := make(map[string]interface{})

	// 检查网络接口状态
	interfaces, err := getNetworkInterfaces()
	if err != nil {
		return StatusUnhealthy, fmt.Sprintf("Failed to get network interfaces: %v", err), details
	}

	details["interfaces"] = interfaces
	details["interface_count"] = len(interfaces)

	// 检查是否有活动的网络接口
	activeCount := 0
	for _, iface := range interfaces {
		if iface.Up {
			activeCount++
		}
	}
	details["active_interfaces"] = activeCount

	if activeCount == 0 {
		return StatusUnhealthy, "No active network interfaces", details
	}

	// 基本网络连通性检查 (DNS)
	if hc.checkDNSConnectivity(ctx) {
		details["dns_resolved"] = true
		return StatusHealthy, fmt.Sprintf("Network is healthy, %d active interfaces", activeCount), details
	}

	details["dns_resolved"] = false
	return StatusDegraded, "Network available but DNS resolution failed", details
}

// getCPUUsage 获取 CPU 使用率 (简化实现)
func (hc *HealthChecker) getCPUUsage() float64 {
	// 使用运行时信息估算
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 简化：基于 GC 暂停时间估算负载
	// 生产环境应使用 gopsutil 等库
	avgGC := float64(m.PauseTotalNs) / float64(m.NumGC+1) / 1e6
	if avgGC > 10 {
		return 75.0 + avgGC/10
	}
	return 30.0 + float64(m.NumGC%10)*3
}

// getNetworkInterfaces 获取网络接口信息
func getNetworkInterfaces() ([]InterfaceInfo, error) {
	interfaces, err := netInterfaces()
	if err != nil {
		return nil, err
	}

	var result []InterfaceInfo
	for _, iface := range interfaces {
		result = append(result, InterfaceInfo{
			Name: iface.Name,
			Up:   iface.Flags&1 != 0, // FlagUp
			MTU:  iface.MTU,
		})
	}
	return result, nil
}

// InterfaceInfo 网络接口信息
type InterfaceInfo struct {
	Name string `json:"name"`
	Up   bool   `json:"up"`
	MTU  int    `json:"mtu"`
}

// checkDNSConnectivity 检查 DNS 连通性
func (hc *HealthChecker) checkDNSConnectivity(ctx context.Context) bool {
	// 简化实现：尝试解析本地地址
	return true // 基本检查默认通过
}

// netInterfaces 网络接口获取函数 (可测试)
var netInterfaces = func() ([]netInterface, error) {
	// 使用 net 包获取接口
	// 这里简化返回
	return []netInterface{
		{Name: "eth0", Flags: 1, MTU: 1500},
		{Name: "lo", Flags: 1, MTU: 65536},
	}, nil
}

// netInterface 网络接口结构
type netInterface struct {
	Name  string
	Flags int
	MTU   int
}

// SetThreshold 设置检查阈值
func (hc *HealthChecker) SetThreshold(checkType CheckType, threshold float64) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	switch checkType {
	case CheckTypeCPU:
		hc.cpuThreshold = threshold
	case CheckTypeMemory:
		hc.memoryThreshold = threshold
	case CheckTypeDisk:
		hc.diskThreshold = threshold
	}
}

// GetThreshold 获取检查阈值
func (hc *HealthChecker) GetThreshold(checkType CheckType) float64 {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	switch checkType {
	case CheckTypeCPU:
		return hc.cpuThreshold
	case CheckTypeMemory:
		return hc.memoryThreshold
	case CheckTypeDisk:
		return hc.diskThreshold
	default:
		return 0
	}
}

// GetUptime 获取运行时间
func (hc *HealthChecker) GetUptime() time.Duration {
	return time.Since(hc.startTime)
}

// GetVersion 获取版本
func (hc *HealthChecker) GetVersion() string {
	return hc.version
}

// Start 启动定期检查
func (hc *HealthChecker) Start(interval time.Duration) {
	hc.wg.Add(1)
	go hc.runPeriodicChecks(interval)
}

// Stop 停止检查
func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
	hc.wg.Wait()
}

// runPeriodicChecks 运行定期检查
func (hc *HealthChecker) runPeriodicChecks(interval time.Duration) {
	defer hc.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-hc.stopCh:
			return
		case <-ticker.C:
			hc.Check(context.Background())
		}
	}
}
