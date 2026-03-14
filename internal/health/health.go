// Package health 提供系统健康检查功能
package health

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// HealthStatus 健康状态
type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusUnhealthy HealthStatus = "unhealthy"
	StatusDegraded  HealthStatus = "degraded"
)

// CheckType 检查类型
type CheckType string

const (
	CheckTypeDatabase CheckType = "database"
	CheckTypeStorage  CheckType = "storage"
	CheckTypeMemory   CheckType = "memory"
	CheckTypeNetwork  CheckType = "network"
	CheckTypeService  CheckType = "service"
	CheckTypeCustom   CheckType = "custom"
)

// HealthChecker 健康检查器接口
type HealthChecker interface {
	Name() string
	Type() CheckType
	Check(ctx context.Context) *CheckResult
}

// CheckResult 检查结果
type CheckResult struct {
	Name      string                 `json:"name"`
	Type      CheckType              `json:"type"`
	Status    HealthStatus           `json:"status"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Duration  time.Duration          `json:"duration"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// HealthManager 健康管理器
type HealthManager struct {
	checkers map[string]HealthChecker
	results  map[string]*CheckResult
	mu       sync.RWMutex
	timeout  time.Duration
}

// HealthReport 健康报告
type HealthReport struct {
	Status    HealthStatus            `json:"status"`
	Timestamp time.Time               `json:"timestamp"`
	Checks    map[string]*CheckResult `json:"checks"`
	Summary   *HealthSummary          `json:"summary"`
	Uptime    time.Duration           `json:"uptime"`
	Version   string                  `json:"version,omitempty"`
}

// HealthSummary 健康摘要
type HealthSummary struct {
	Total     int `json:"total"`
	Healthy   int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
	Degraded  int `json:"degraded"`
}

// HealthConfig 健康检查配置
type HealthConfig struct {
	Timeout         time.Duration
	CheckInterval   time.Duration
	EnableScheduler bool
	Version         string
}

// NewHealthManager 创建健康管理器
func NewHealthManager(timeout time.Duration) *HealthManager {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &HealthManager{
		checkers: make(map[string]HealthChecker),
		results:  make(map[string]*CheckResult),
		timeout:  timeout,
	}
}

// NewHealthManagerWithConfig 使用配置创建健康管理器
func NewHealthManagerWithConfig(config *HealthConfig) *HealthManager {
	if config == nil {
		config = &HealthConfig{}
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	return &HealthManager{
		checkers: make(map[string]HealthChecker),
		results:  make(map[string]*CheckResult),
		timeout:  config.Timeout,
	}
}

// RegisterChecker 注册检查器
func (m *HealthManager) RegisterChecker(checker HealthChecker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checkers[checker.Name()] = checker
}

// RemoveChecker 移除检查器
func (m *HealthManager) RemoveChecker(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.checkers, name)
}

// GetChecker 获取检查器
func (m *HealthManager) GetChecker(name string) (HealthChecker, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	checker, exists := m.checkers[name]
	return checker, exists
}

// ListCheckers 列出所有检查器
func (m *HealthManager) ListCheckers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.checkers))
	for name := range m.checkers {
		names = append(names, name)
	}
	return names
}

// RunCheck 执行单个检查
func (m *HealthManager) RunCheck(ctx context.Context, name string) (*CheckResult, error) {
	m.mu.RLock()
	checker, exists := m.checkers[name]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("checker %s not found", name)
	}

	checkCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	start := time.Now()
	result := checker.Check(checkCtx)
	result.Duration = time.Since(start)
	result.Timestamp = time.Now()

	m.mu.Lock()
	m.results[name] = result
	m.mu.Unlock()

	return result, nil
}

// RunAllChecks 执行所有检查
func (m *HealthManager) RunAllChecks(ctx context.Context) *HealthReport {
	report := &HealthReport{
		Timestamp: time.Now(),
		Checks:    make(map[string]*CheckResult),
		Summary:   &HealthSummary{},
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	m.mu.RLock()
	checkers := make([]HealthChecker, 0, len(m.checkers))
	for _, c := range m.checkers {
		checkers = append(checkers, c)
	}
	m.mu.RUnlock()

	for _, checker := range checkers {
		wg.Add(1)
		go func(c HealthChecker) {
			defer wg.Done()

			result, _ := m.RunCheck(ctx, c.Name())

			mu.Lock()
			report.Checks[c.Name()] = result
			report.Summary.Total++
			switch result.Status {
			case StatusHealthy:
				report.Summary.Healthy++
			case StatusUnhealthy:
				report.Summary.Unhealthy++
			case StatusDegraded:
				report.Summary.Degraded++
			}
			mu.Unlock()
		}(checker)
	}

	wg.Wait()

	// 确定总体状态
	if report.Summary.Unhealthy > 0 {
		report.Status = StatusUnhealthy
	} else if report.Summary.Degraded > 0 {
		report.Status = StatusDegraded
	} else {
		report.Status = StatusHealthy
	}

	return report
}

// GetLastResult 获取最近检查结果
func (m *HealthManager) GetLastResult(name string) (*CheckResult, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result, exists := m.results[name]
	return result, exists
}

// GetAllLastResults 获取所有最近检查结果
func (m *HealthManager) GetAllLastResults() map[string]*CheckResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	results := make(map[string]*CheckResult, len(m.results))
	for k, v := range m.results {
		results[k] = v
	}
	return results
}

// IsHealthy 检查是否健康
func (m *HealthManager) IsHealthy() bool {
	report := m.RunAllChecks(context.Background())
	return report.Status == StatusHealthy
}

// GenerateReport 生成健康报告
func (m *HealthManager) GenerateReport(ctx context.Context, version string) *HealthReport {
	report := m.RunAllChecks(ctx)
	report.Version = version
	report.Uptime = getUptime()
	return report
}

// getUptime 获取系统运行时间
func getUptime() time.Duration {
	// 简化实现，实际应从系统启动时间计算
	return time.Since(time.Now().Add(-time.Hour))
}

// ========== 内置检查器 ==========

// MemoryChecker 内存检查器
type MemoryChecker struct {
	name      string
	threshold float64 // 内存使用阈值百分比
}

// NewMemoryChecker 创建内存检查器
func NewMemoryChecker(threshold float64) *MemoryChecker {
	return &MemoryChecker{
		name:      "memory",
		threshold: threshold,
	}
}

// NewMemoryCheckerWithName 创建带名称的内存检查器
func NewMemoryCheckerWithName(name string, threshold float64) *MemoryChecker {
	return &MemoryChecker{
		name:      name,
		threshold: threshold,
	}
}

func (c *MemoryChecker) Name() string    { return c.name }
func (c *MemoryChecker) Type() CheckType { return CheckTypeMemory }

func (c *MemoryChecker) Check(ctx context.Context) *CheckResult {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	usedPercent := float64(m.Alloc) / float64(m.Sys) * 100

	result := &CheckResult{
		Name:    c.Name(),
		Type:    c.Type(),
		Details: make(map[string]interface{}),
	}

	result.Details["alloc_mb"] = m.Alloc / 1024 / 1024
	result.Details["total_alloc_mb"] = m.TotalAlloc / 1024 / 1024
	result.Details["sys_mb"] = m.Sys / 1024 / 1024
	result.Details["heap_alloc_mb"] = m.HeapAlloc / 1024 / 1024
	result.Details["heap_sys_mb"] = m.HeapSys / 1024 / 1024
	result.Details["used_percent"] = usedPercent
	result.Details["threshold_percent"] = c.threshold
	result.Details["num_gc"] = m.NumGC

	if usedPercent > c.threshold {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Memory usage %.2f%% exceeds threshold %.2f%%", usedPercent, c.threshold)
	} else if usedPercent > c.threshold*0.8 {
		result.Status = StatusDegraded
		result.Message = fmt.Sprintf("Memory usage %.2f%% is approaching threshold", usedPercent)
	} else {
		result.Status = StatusHealthy
		result.Message = "Memory usage is normal"
	}

	return result
}

// HTTPChecker HTTP 服务检查器
type HTTPChecker struct {
	name     string
	url      string
	timeout  time.Duration
	expected int
	headers  map[string]string
}

// NewHTTPChecker 创建 HTTP 检查器
func NewHTTPChecker(name, url string, timeout time.Duration, expectedStatus int) *HTTPChecker {
	return &HTTPChecker{
		name:     name,
		url:      url,
		timeout:  timeout,
		expected: expectedStatus,
		headers:  make(map[string]string),
	}
}

// SetHeaders 设置请求头
func (c *HTTPChecker) SetHeaders(headers map[string]string) {
	c.headers = headers
}

func (c *HTTPChecker) Name() string    { return c.name }
func (c *HTTPChecker) Type() CheckType { return CheckTypeService }

func (c *HTTPChecker) Check(ctx context.Context) *CheckResult {
	result := &CheckResult{
		Name:    c.Name(),
		Type:    c.Type(),
		Details: make(map[string]interface{}),
	}

	client := &http.Client{Timeout: c.timeout}
	req, err := http.NewRequestWithContext(ctx, "GET", c.url, nil)
	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Failed to create request: %v", err)
		return result
	}

	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("HTTP request failed: %v", err)
		result.Details["error"] = err.Error()
		return result
	}
	defer func() { _ = resp.Body.Close() }()

	result.Details["status_code"] = resp.StatusCode
	result.Details["response_time_ms"] = duration.Milliseconds()
	result.Details["url"] = c.url

	if resp.StatusCode == c.expected {
		result.Status = StatusHealthy
		result.Message = fmt.Sprintf("HTTP status %d as expected", resp.StatusCode)
	} else if resp.StatusCode >= 500 {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Server error: status %d", resp.StatusCode)
	} else if resp.StatusCode >= 400 {
		result.Status = StatusDegraded
		result.Message = fmt.Sprintf("Client error: status %d", resp.StatusCode)
	} else {
		result.Status = StatusHealthy
		result.Message = fmt.Sprintf("HTTP status %d", resp.StatusCode)
	}

	return result
}

// DiskSpaceChecker 磁盘空间检查器
type DiskSpaceChecker struct {
	name      string
	path      string
	threshold float64
}

// NewDiskSpaceChecker 创建磁盘空间检查器
func NewDiskSpaceChecker(name string, threshold float64) *DiskSpaceChecker {
	return &DiskSpaceChecker{
		name:      name,
		path:      "/",
		threshold: threshold,
	}
}

// NewDiskSpaceCheckerWithPath 创建带路径的磁盘空间检查器
func NewDiskSpaceCheckerWithPath(name, path string, threshold float64) *DiskSpaceChecker {
	return &DiskSpaceChecker{
		name:      name,
		path:      path,
		threshold: threshold,
	}
}

func (c *DiskSpaceChecker) Name() string    { return c.name }
func (c *DiskSpaceChecker) Type() CheckType { return CheckTypeStorage }

func (c *DiskSpaceChecker) Check(ctx context.Context) *CheckResult {
	result := &CheckResult{
		Name:    c.Name(),
		Type:    c.Type(),
		Details: make(map[string]interface{}),
	}

	var stat syscall.Statfs_t
	err := syscall.Statfs(c.path, &stat)
	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Failed to get disk stats: %v", err)
		return result
	}

	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bfree * uint64(stat.Bsize)
	usedBytes := totalBytes - freeBytes
	usedPercent := float64(usedBytes) / float64(totalBytes) * 100

	result.Details["path"] = c.path
	result.Details["total_gb"] = totalBytes / 1024 / 1024 / 1024
	result.Details["used_gb"] = usedBytes / 1024 / 1024 / 1024
	result.Details["free_gb"] = freeBytes / 1024 / 1024 / 1024
	result.Details["used_percent"] = usedPercent
	result.Details["threshold_percent"] = c.threshold

	if usedPercent > c.threshold {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Disk usage %.2f%% exceeds threshold %.2f%%", usedPercent, c.threshold)
	} else if usedPercent > c.threshold*0.8 {
		result.Status = StatusDegraded
		result.Message = fmt.Sprintf("Disk usage %.2f%% is approaching threshold", usedPercent)
	} else {
		result.Status = StatusHealthy
		result.Message = "Disk space is sufficient"
	}

	return result
}

// DatabaseChecker 数据库检查器
type DatabaseChecker struct {
	name    string
	db      *sql.DB
	timeout time.Duration
	query   string
}

// NewDatabaseChecker 创建数据库检查器
func NewDatabaseChecker(name string, db *sql.DB, timeout time.Duration) *DatabaseChecker {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &DatabaseChecker{
		name:    name,
		db:      db,
		timeout: timeout,
		query:   "SELECT 1",
	}
}

// SetQuery 设置检查查询
func (c *DatabaseChecker) SetQuery(query string) {
	c.query = query
}

func (c *DatabaseChecker) Name() string    { return c.name }
func (c *DatabaseChecker) Type() CheckType { return CheckTypeDatabase }

func (c *DatabaseChecker) Check(ctx context.Context) *CheckResult {
	result := &CheckResult{
		Name:    c.Name(),
		Type:    c.Type(),
		Details: make(map[string]interface{}),
	}

	if c.db == nil {
		result.Status = StatusUnhealthy
		result.Message = "Database connection is nil"
		return result
	}

	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	start := time.Now()
	var dummy int
	err := c.db.QueryRowContext(checkCtx, c.query).Scan(&dummy)
	duration := time.Since(start)

	result.Details["response_time_ms"] = duration.Milliseconds()
	result.Details["query"] = c.query

	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Database query failed: %v", err)
		result.Details["error"] = err.Error()
		return result
	}

	// 检查连接池状态
	stats := c.db.Stats()
	result.Details["open_connections"] = stats.OpenConnections
	result.Details["in_use"] = stats.InUse
	result.Details["idle"] = stats.Idle
	result.Details["wait_count"] = stats.WaitCount
	result.Details["wait_duration_ms"] = stats.WaitDuration.Milliseconds()

	// 如果等待时间过长，标记为降级
	if stats.WaitDuration > time.Second {
		result.Status = StatusDegraded
		result.Message = "Database is responsive but connection pool is under pressure"
	} else {
		result.Status = StatusHealthy
		result.Message = "Database connection is healthy"
	}

	return result
}

// NetworkChecker 网络连接检查器
type NetworkChecker struct {
	name    string
	target  string
	timeout time.Duration
}

// NewNetworkChecker 创建网络检查器
func NewNetworkChecker(name, target string, timeout time.Duration) *NetworkChecker {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &NetworkChecker{
		name:    name,
		target:  target,
		timeout: timeout,
	}
}

func (c *NetworkChecker) Name() string    { return c.name }
func (c *NetworkChecker) Type() CheckType { return CheckTypeNetwork }

func (c *NetworkChecker) Check(ctx context.Context) *CheckResult {
	result := &CheckResult{
		Name:    c.Name(),
		Type:    c.Type(),
		Details: make(map[string]interface{}),
	}

	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	start := time.Now()
	var d net.Dialer
	conn, err := d.DialContext(checkCtx, "tcp", c.target)
	duration := time.Since(start)

	result.Details["target"] = c.target
	result.Details["response_time_ms"] = duration.Milliseconds()

	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Network connection failed: %v", err)
		result.Details["error"] = err.Error()
		return result
	}
	conn.Close()

	// 如果响应时间超过阈值的一半，标记为降级
	if duration > c.timeout/2 {
		result.Status = StatusDegraded
		result.Message = fmt.Sprintf("Network connection is slow: %v", duration)
	} else {
		result.Status = StatusHealthy
		result.Message = "Network connection is healthy"
	}

	return result
}

// DNSChecker DNS 解析检查器
type DNSChecker struct {
	name    string
	host    string
	timeout time.Duration
}

// NewDNSChecker 创建 DNS 检查器
func NewDNSChecker(name, host string, timeout time.Duration) *DNSChecker {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &DNSChecker{
		name:    name,
		host:    host,
		timeout: timeout,
	}
}

func (c *DNSChecker) Name() string    { return c.name }
func (c *DNSChecker) Type() CheckType { return CheckTypeNetwork }

func (c *DNSChecker) Check(ctx context.Context) *CheckResult {
	result := &CheckResult{
		Name:    c.Name(),
		Type:    c.Type(),
		Details: make(map[string]interface{}),
	}

	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	start := time.Now()
	addrs, err := net.DefaultResolver.LookupHost(checkCtx, c.host)
	duration := time.Since(start)

	result.Details["host"] = c.host
	result.Details["response_time_ms"] = duration.Milliseconds()

	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("DNS resolution failed: %v", err)
		result.Details["error"] = err.Error()
		return result
	}

	if len(addrs) == 0 {
		result.Status = StatusUnhealthy
		result.Message = "DNS resolution returned no addresses"
		return result
	}

	result.Details["resolved_addresses"] = addrs
	result.Status = StatusHealthy
	result.Message = fmt.Sprintf("DNS resolved to %d addresses", len(addrs))

	return result
}

// CustomChecker 自定义检查器
type CustomChecker struct {
	name    string
	checkFn func(ctx context.Context) (HealthStatus, string, map[string]interface{})
}

// NewCustomChecker 创建自定义检查器
func NewCustomChecker(name string, checkFn func(ctx context.Context) (HealthStatus, string, map[string]interface{})) *CustomChecker {
	return &CustomChecker{
		name:    name,
		checkFn: checkFn,
	}
}

func (c *CustomChecker) Name() string    { return c.name }
func (c *CustomChecker) Type() CheckType { return CheckTypeCustom }

func (c *CustomChecker) Check(ctx context.Context) *CheckResult {
	result := &CheckResult{
		Name:      c.Name(),
		Type:      c.Type(),
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	if c.checkFn == nil {
		result.Status = StatusUnhealthy
		result.Message = "Check function is nil"
		return result
	}

	status, message, details := c.checkFn(ctx)
	result.Status = status
	result.Message = message
	if details != nil {
		result.Details = details
	}

	return result
}

// TCPChecker TCP 端口检查器
type TCPChecker struct {
	name    string
	address string
	timeout time.Duration
}

// NewTCPChecker 创建 TCP 检查器
func NewTCPChecker(name, address string, timeout time.Duration) *TCPChecker {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &TCPChecker{
		name:    name,
		address: address,
		timeout: timeout,
	}
}

func (c *TCPChecker) Name() string    { return c.name }
func (c *TCPChecker) Type() CheckType { return CheckTypeService }

func (c *TCPChecker) Check(ctx context.Context) *CheckResult {
	result := &CheckResult{
		Name:    c.Name(),
		Type:    c.Type(),
		Details: make(map[string]interface{}),
	}

	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	start := time.Now()
	var d net.Dialer
	conn, err := d.DialContext(checkCtx, "tcp", c.address)
	duration := time.Since(start)

	result.Details["address"] = c.address
	result.Details["response_time_ms"] = duration.Milliseconds()

	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("TCP connection failed: %v", err)
		result.Details["error"] = err.Error()
		return result
	}
	conn.Close()

	result.Status = StatusHealthy
	result.Message = "TCP port is open and accepting connections"
	return result
}

// FileChecker 文件存在检查器
type FileChecker struct {
	name     string
	path     string
	checkDir bool
}

// NewFileChecker 创建文件检查器
func NewFileChecker(name, path string) *FileChecker {
	return &FileChecker{
		name:     name,
		path:     path,
		checkDir: false,
	}
}

// NewDirChecker 创建目录检查器
func NewDirChecker(name, path string) *FileChecker {
	return &FileChecker{
		name:     name,
		path:     path,
		checkDir: true,
	}
}

func (c *FileChecker) Name() string    { return c.name }
func (c *FileChecker) Type() CheckType { return CheckTypeStorage }

func (c *FileChecker) Check(ctx context.Context) *CheckResult {
	result := &CheckResult{
		Name:    c.Name(),
		Type:    c.Type(),
		Details: make(map[string]interface{}),
	}

	result.Details["path"] = c.path

	info, err := os.Stat(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			result.Status = StatusUnhealthy
			result.Message = fmt.Sprintf("Path does not exist: %s", c.path)
		} else {
			result.Status = StatusUnhealthy
			result.Message = fmt.Sprintf("Failed to stat path: %v", err)
		}
		return result
	}

	if c.checkDir && !info.IsDir() {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Path is not a directory: %s", c.path)
		return result
	}

	if !c.checkDir && info.IsDir() {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Path is a directory, expected file: %s", c.path)
		return result
	}

	result.Status = StatusHealthy
	result.Message = fmt.Sprintf("Path is accessible: %s", c.path)
	result.Details["size"] = info.Size()
	result.Details["mode"] = info.Mode().String()
	result.Details["modified"] = info.ModTime().Format(time.RFC3339)

	return result
}

// ProcessChecker 进程检查器
type ProcessChecker struct {
	name       string
	pid        int
	checkAlive bool
}

// NewProcessChecker 创建进程检查器
func NewProcessChecker(name string, pid int) *ProcessChecker {
	return &ProcessChecker{
		name:       name,
		pid:        pid,
		checkAlive: true,
	}
}

func (c *ProcessChecker) Name() string    { return c.name }
func (c *ProcessChecker) Type() CheckType { return CheckTypeService }

func (c *ProcessChecker) Check(ctx context.Context) *CheckResult {
	result := &CheckResult{
		Name:    c.Name(),
		Type:    c.Type(),
		Details: make(map[string]interface{}),
	}

	result.Details["pid"] = c.pid

	process, err := os.FindProcess(c.pid)
	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Failed to find process: %v", err)
		return result
	}

	// 发送信号 0 检查进程是否存在
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Process is not running: %v", err)
		return result
	}

	result.Status = StatusHealthy
	result.Message = fmt.Sprintf("Process %d is running", c.pid)
	return result
}
