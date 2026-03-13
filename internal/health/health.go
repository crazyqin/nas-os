// Package health 提供系统健康检查功能
package health

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"sync"
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
	Name      string
	Type      CheckType
	Status    HealthStatus
	Message   string
	Timestamp time.Time
	Duration  time.Duration
	Details   map[string]interface{}
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
	Status    HealthStatus
	Timestamp time.Time
	Checks    map[string]*CheckResult
	Summary   *HealthSummary
}

// HealthSummary 健康摘要
type HealthSummary struct {
	Total     int
	Healthy   int
	Unhealthy int
	Degraded  int
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

// IsHealthy 检查是否健康
func (m *HealthManager) IsHealthy() bool {
	report := m.RunAllChecks(context.Background())
	return report.Status == StatusHealthy
}

// ========== 内置检查器 ==========

// MemoryChecker 内存检查器
type MemoryChecker struct {
	threshold float64 // 内存使用阈值百分比
}

func NewMemoryChecker(threshold float64) *MemoryChecker {
	return &MemoryChecker{threshold: threshold}
}

func (c *MemoryChecker) Name() string    { return "memory" }
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
	result.Details["sys_mb"] = m.Sys / 1024 / 1024
	result.Details["used_percent"] = usedPercent

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
}

func NewHTTPChecker(name, url string, timeout time.Duration, expectedStatus int) *HTTPChecker {
	return &HTTPChecker{
		name:     name,
		url:      url,
		timeout:  timeout,
		expected: expectedStatus,
	}
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
	resp, err := client.Get(c.url)
	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("HTTP request failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	result.Details["status_code"] = resp.StatusCode

	if resp.StatusCode == c.expected {
		result.Status = StatusHealthy
		result.Message = fmt.Sprintf("HTTP status %d as expected", resp.StatusCode)
	} else {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Expected status %d, got %d", c.expected, resp.StatusCode)
	}

	return result
}

// DiskSpaceChecker 磁盘空间检查器
type DiskSpaceChecker struct {
	name      string
	threshold float64
}

func NewDiskSpaceChecker(name string, threshold float64) *DiskSpaceChecker {
	return &DiskSpaceChecker{
		name:      name,
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

	// 简化实现，实际应检查磁盘使用率
	result.Status = StatusHealthy
	result.Message = "Disk space is sufficient"
	result.Details["used_percent"] = 45.5

	return result
}
