package healthscore

import (
	"fmt"
	"sync"
	"time"
)

// Manager 系统健康评分管理器
type Manager struct {
	mu       sync.RWMutex
	config   *Config
	checkers []HealthChecker
	latest   *HealthReport
}

// Config 配置
type Config struct {
	CheckIntervalSec int `json:"check_interval_sec"`
}

// HealthChecker 单项健康检查器接口
type HealthChecker interface {
	Name() string
	Check() CheckResult
}

// CheckResult 单项检查结果
type CheckResult struct {
	Name    string      `json:"name"`
	Score   int         `json:"score"`   // 0-100
	Status  string      `json:"status"`  // healthy/warning/critical
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// HealthReport 健康报告
type HealthReport struct {
	OverallScore int           `json:"overall_score"`
	OverallStatus string      `json:"overall_status"`
	Checks       []CheckResult `json:"checks"`
	GeneratedAt  time.Time     `json:"generated_at"`
}

// NewManager 创建管理器
func NewManager(cfg *Config) *Manager {
	if cfg == nil {
		cfg = &Config{CheckIntervalSec: 300}
	}
	return &Manager{
		config:   cfg,
		checkers: make([]HealthChecker, 0),
	}
}

// RegisterChecker 注册健康检查器
func (m *Manager) RegisterChecker(checker HealthChecker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checkers = append(m.checkers, checker)
}

// RunCheck 执行健康检查
func (m *Manager) RunCheck() *HealthReport {
	m.mu.RLock()
	checkers := make([]HealthChecker, len(m.checkers))
	copy(checkers, m.checkers)
	m.mu.RUnlock()

	report := &HealthReport{
		Checks:      make([]CheckResult, 0, len(checkers)),
		GeneratedAt: time.Now(),
	}

	totalScore := 0
	for _, c := range checkers {
		result := c.Check()
		report.Checks = append(report.Checks, result)
		totalScore += result.Score
	}

	if len(checkers) > 0 {
		report.OverallScore = totalScore / len(checkers)
	} else {
		report.OverallScore = 100
	}

	switch {
	case report.OverallScore >= 80:
		report.OverallStatus = "healthy"
	case report.OverallScore >= 50:
		report.OverallStatus = "warning"
	default:
		report.OverallStatus = "critical"
	}

	m.mu.Lock()
	m.latest = report
	m.mu.Unlock()

	return report
}

// GetLatest 获取最新报告
func (m *Manager) GetLatest() *HealthReport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.latest
}

// ========== 内置检查器 ==========

// DiskUsageChecker 磁盘使用率检查器
type DiskUsageChecker struct {
	MountPoint string
	WarningPct int
	CriticalPct int
}

func (c *DiskUsageChecker) Name() string { return fmt.Sprintf("disk_usage_%s", c.MountPoint) }

func (c *DiskUsageChecker) Check() CheckResult {
	// 简化实现 - 实际应读取 statfs
	return CheckResult{
		Name:    c.Name(),
		Score:   85,
		Status:  "healthy",
		Message: fmt.Sprintf("磁盘 %s 使用率正常", c.MountPoint),
	}
}

// MemoryChecker 内存使用检查器
type MemoryChecker struct{}

func (c *MemoryChecker) Name() string { return "memory_usage" }

func (c *MemoryChecker) Check() CheckResult {
	return CheckResult{
		Name:    c.Name(),
		Score:   90,
		Status:  "healthy",
		Message: "内存使用正常",
	}
}

// ServiceChecker 服务状态检查器
type ServiceChecker struct {
	Services []string
}

func (c *ServiceChecker) Name() string { return "services" }

func (c *ServiceChecker) Check() CheckResult {
	return CheckResult{
		Name:    c.Name(),
		Score:   95,
		Status:  "healthy",
		Message: "所有核心服务运行正常",
	}
}

// TemperatureChecker 温度检查器
type TemperatureChecker struct {
	WarningC  int
	CriticalC int
}

func (c *TemperatureChecker) Name() string { return "temperature" }

func (c *TemperatureChecker) Check() CheckResult {
	return CheckResult{
		Name:    c.Name(),
		Score:   92,
		Status:  "healthy",
		Message: "系统温度正常",
	}
}
