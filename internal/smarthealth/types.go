// Package smarthealth 智能健康巡检系统
// 对标群晖Resource Monitor + TrueNAS Reporting
// 自动巡检CPU/内存/磁盘/网络/服务状态，计算健康评分，历史趋势
package smarthealth

import (
	"fmt"
	"sync"
	"time"
)

// CheckLevel 检查级别
type CheckLevel string

const (
	LevelInfo     CheckLevel = "info"
	LevelWarning  CheckLevel = "warning"
	LevelCritical CheckLevel = "critical"
)

// CheckCategory 检查类别
type CheckCategory string

const (
	CatCPU     CheckCategory = "cpu"
	CatMemory  CheckCategory = "memory"
	CatDisk    CheckCategory = "disk"
	CatNetwork CheckCategory = "network"
	CatService CheckCategory = "service"
	CatRAID    CheckCategory = "raid"
	CatTemp    CheckCategory = "temperature"
)

// HealthStatus 健康状态
type HealthStatus string

const (
	StatusHealthy  HealthStatus = "healthy"
	StatusDegraded HealthStatus = "degraded"
	StatusWarning  HealthStatus = "warning"
	StatusCritical HealthStatus = "critical"
)

// SystemHealth 系统健康状态
type SystemHealth struct {
	Score     int              `json:"score"`     // 0-100
	Status    HealthStatus     `json:"status"`
	Checks    []CheckResult    `json:"checks"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// CheckResult 单项检查结果
type CheckResult struct {
	ID        string       `json:"id"`
	Category  CheckCategory `json:"category"`
	Name      string       `json:"name"`
	Level     CheckLevel   `json:"level"`
	Value     float64      `json:"value"`
	Threshold float64      `json:"threshold"`
	Message   string       `json:"message"`
	Timestamp time.Time    `json:"timestamp"`
}

// HealthTrend 健康趋势
type HealthTrend struct {
	Timestamp time.Time `json:"timestamp"`
	Score     int       `json:"score"`
	Status    HealthStatus `json:"status"`
}

// PatrolConfig 巡检配置
type PatrolConfig struct {
	Enabled         bool          `json:"enabled"`
	Interval        time.Duration `json:"interval"`
	CPUThreshold    float64       `json:"cpu_threshold"`     // %
	MemThreshold    float64       `json:"mem_threshold"`     // %
	DiskThreshold   float64       `json:"disk_threshold"`    // %
	TempThreshold   float64       `json:"temp_threshold"`    // ℃
	AlertWebhook    string        `json:"alert_webhook"`
	RetentionDays   int           `json:"retention_days"`
}

// DefaultPatrolConfig 默认巡检配置
func DefaultPatrolConfig() *PatrolConfig {
	return &PatrolConfig{
		Enabled:       true,
		Interval:      5 * time.Minute,
		CPUThreshold:  90,
		MemThreshold:  85,
		DiskThreshold: 90,
		TempThreshold: 75,
		RetentionDays: 30,
	}
}

// Manager 智能健康管理器
type Manager struct {
	mu         sync.RWMutex
	config     *PatrolConfig
	current    *SystemHealth
	trends     []HealthTrend
	checks     []CheckResult
	alerts     []Alert
	stopCh     chan struct{}
}

// Alert 告警记录
type Alert struct {
	ID        string       `json:"id"`
	Category  CheckCategory `json:"category"`
	Level     CheckLevel   `json:"level"`
	Message   string       `json:"message"`
	Value     float64      `json:"value"`
	Threshold float64      `json:"threshold"`
	CreatedAt time.Time    `json:"created_at"`
	Resolved  bool         `json:"resolved"`
}

// NewManager 创建健康管理器
func NewManager() *Manager {
	return &Manager{
		config: DefaultPatrolConfig(),
		trends: make([]HealthTrend, 0),
		checks: make([]CheckResult, 0),
		alerts: make([]Alert, 0),
		stopCh: make(chan struct{}),
	}
}

// Start 启动巡检
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return fmt.Errorf("健康巡检已禁用")
	}

	go m.patrolLoop()
	return nil
}

// Stop 停止巡检
func (m *Manager) Stop() {
	close(m.stopCh)
}

func (m *Manager) patrolLoop() {
	ticker := time.NewTicker(m.config.Interval)
	defer ticker.Stop()

	// 立即执行一次
	m.runPatrol()

	for {
		select {
		case <-ticker.C:
			m.runPatrol()
		case <-m.stopCh:
			return
		}
	}
}

func (m *Manager) runPatrol() {
	m.mu.Lock()
	defer m.mu.Unlock()

	checks := make([]CheckResult, 0)

	// CPU 检查
	cpuUsage := m.getCPUUsage()
	checks = append(checks, CheckResult{
		ID:        fmt.Sprintf("cpu_%d", time.Now().Unix()),
		Category:  CatCPU,
		Name:      "CPU 使用率",
		Level:     m.getLevel(cpuUsage, m.config.CPUThreshold),
		Value:     cpuUsage,
		Threshold: m.config.CPUThreshold,
		Message:   fmt.Sprintf("CPU 使用率 %.1f%%", cpuUsage),
		Timestamp: time.Now(),
	})

	// 内存检查
	memUsage := m.getMemoryUsage()
	checks = append(checks, CheckResult{
		ID:        fmt.Sprintf("mem_%d", time.Now().Unix()),
		Category:  CatMemory,
		Name:      "内存使用率",
		Level:     m.getLevel(memUsage, m.config.MemThreshold),
		Value:     memUsage,
		Threshold: m.config.MemThreshold,
		Message:   fmt.Sprintf("内存使用率 %.1f%%", memUsage),
		Timestamp: time.Now(),
	})

	// 磁盘检查
	diskUsage := m.getDiskUsage()
	checks = append(checks, CheckResult{
		ID:        fmt.Sprintf("disk_%d", time.Now().Unix()),
		Category:  CatDisk,
		Name:      "磁盘使用率",
		Level:     m.getLevel(diskUsage, m.config.DiskThreshold),
		Value:     diskUsage,
		Threshold: m.config.DiskThreshold,
		Message:   fmt.Sprintf("磁盘使用率 %.1f%%", diskUsage),
		Timestamp: time.Now(),
	})

	// 温度检查
	temp := m.getTemperature()
	checks = append(checks, CheckResult{
		ID:        fmt.Sprintf("temp_%d", time.Now().Unix()),
		Category:  CatTemp,
		Name:      "CPU 温度",
		Level:     m.getLevel(temp, m.config.TempThreshold),
		Value:     temp,
		Threshold: m.config.TempThreshold,
		Message:   fmt.Sprintf("CPU 温度 %.1f°C", temp),
		Timestamp: time.Now(),
	})

	m.checks = checks
	score := m.calculateScore(checks)
	status := m.scoreToStatus(score)

	health := &SystemHealth{
		Score:     score,
		Status:    status,
		Checks:    checks,
		UpdatedAt: time.Now(),
	}
	m.current = health

	// 记录趋势
	m.trends = append(m.trends, HealthTrend{
		Timestamp: time.Now(),
		Score:     score,
		Status:    status,
	})

	// 保留最近N天数据
	m.trimTrends()

	// 检查告警
	for _, check := range checks {
		if check.Level == LevelWarning || check.Level == LevelCritical {
			m.alerts = append(m.alerts, Alert{
				ID:        fmt.Sprintf("alert_%d", time.Now().UnixNano()),
				Category:  check.Category,
				Level:     check.Level,
				Message:   check.Message,
				Value:     check.Value,
				Threshold: check.Threshold,
				CreatedAt: time.Now(),
			})
		}
	}
}

func (m *Manager) getLevel(value, threshold float64) CheckLevel {
	if value >= threshold {
		return LevelCritical
	}
	if value >= threshold*0.8 {
		return LevelWarning
	}
	return LevelInfo
}

func (m *Manager) calculateScore(checks []CheckResult) int {
	if len(checks) == 0 {
		return 100
	}
	total := 0.0
	weight := 0.0
	for _, c := range checks {
		w := 1.0
		switch c.Category {
		case CatCPU:
			w = 2.0
		case CatDisk:
			w = 2.0
		case CatTemp:
			w = 1.5
		}
		ratio := c.Value / c.Threshold
		if ratio > 1 {
			ratio = 1
		}
		total += ratio * w * 100
		weight += w
	}
	if weight == 0 {
		return 100
	}
	avg := total / weight
	return int(100 - avg)
}

func (m *Manager) scoreToStatus(score int) HealthStatus {
	switch {
	case score >= 80:
		return StatusHealthy
	case score >= 60:
		return StatusDegraded
	case score >= 40:
		return StatusWarning
	default:
		return StatusCritical
	}
}

func (m *Manager) trimTrends() {
	cutoff := time.Now().AddDate(0, 0, -m.config.RetentionDays)
	trimmed := make([]HealthTrend, 0)
	for _, t := range m.trends {
		if t.Timestamp.After(cutoff) {
			trimmed = append(trimmed, t)
		}
	}
	m.trends = trimmed
}

// 模拟系统指标采集（实际实现读取/proc等）
func (m *Manager) getCPUUsage() float64    { return 35.5 }
func (m *Manager) getMemoryUsage() float64  { return 62.3 }
func (m *Manager) getDiskUsage() float64    { return 74.0 }
func (m *Manager) getTemperature() float64  { return 48.0 }

// GetCurrentHealth 获取当前健康状态
func (m *Manager) GetCurrentHealth() *SystemHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// GetTrends 获取健康趋势
func (m *Manager) GetTrends(hours int) []HealthTrend {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
	result := make([]HealthTrend, 0)
	for _, t := range m.trends {
		if t.Timestamp.After(cutoff) {
			result = append(result, t)
		}
	}
	return result
}

// GetAlerts 获取告警列表
func (m *Manager) GetAlerts(resolved bool) []Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Alert, 0)
	for _, a := range m.alerts {
		if a.Resolved == resolved {
			result = append(result, a)
		}
	}
	return result
}

// ResolveAlert 解决告警
func (m *Manager) ResolveAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.alerts {
		if m.alerts[i].ID == alertID {
			m.alerts[i].Resolved = true
			return nil
		}
	}
	return fmt.Errorf("告警不存在: %s", alertID)
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *PatrolConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
	return nil
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *PatrolConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// RunManualCheck 手动触发巡检
func (m *Manager) RunManualCheck() *SystemHealth {
	m.runPatrol()
	return m.GetCurrentHealth()
}
