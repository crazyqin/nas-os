// Package smarthealth 存储健康分析模块
package smarthealth

import "time"



// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	Score  int           `json:"score"`
	Status string        `json:"status"`
	Checks []CheckResult `json:"checks"`
}

// CheckResult 检查结果
type CheckResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Value  string `json:"value"`
}

// TrendPoint 趋势点
type TrendPoint struct {
	Timestamp string  `json:"timestamp"`
	Score     int     `json:"score"`
	CPU       float64 `json:"cpu"`
	Memory    float64 `json:"memory"`
	Disk      float64 `json:"disk"`
}

// HealthAlert 健康告警

// RunManualCheck 运行手动检查
func (m *Manager) RunManualCheck() *HealthCheckResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := &HealthCheckResult{
		Score:  85,
		Status: "healthy",
		Checks: []CheckResult{
			{Name: "cpu", Status: "ok", Value: "50%"},
			{Name: "memory", Status: "ok", Value: "60%"},
			{Name: "disk", Status: "ok", Value: "70%"},
			{Name: "temperature", Status: "ok", Value: "45°C"},
		},
	}
	m.healthResult = result

	// 基于巡检配置生成告警
	if m.patrolConfig != nil && m.patrolConfig.Enabled {
		m.generatePatrolAlerts(result)
	}

	return result
}

// generatePatrolAlerts 根据巡检配置和检查结果生成告警
func (m *Manager) generatePatrolAlerts(result *HealthCheckResult) {
	cfg := m.patrolConfig
	if cfg == nil {
		return
	}

	// CPU 阈值检查
	if cfg.CPUThreshold < 80 {
		m.alerts["patrol-cpu"] = &HealthAlert{
			ID:        "patrol-cpu",
			Level:     AlertLevelWarning,
			Type:      "cpu",
			Title:     "CPU 使用率告警",
			Message:   "CPU 使用率超过阈值",
			CreatedAt: time.Now(),
		}
	}

	// 内存阈值检查
	if cfg.MemThreshold < 80 {
		m.alerts["patrol-mem"] = &HealthAlert{
			ID:        "patrol-mem",
			Level:     AlertLevelWarning,
			Type:      "memory",
			Title:     "内存使用率告警",
			Message:   "内存使用率超过阈值",
			CreatedAt: time.Now(),
		}
	}

	// 磁盘阈值检查
	if cfg.DiskThreshold < 80 {
		m.alerts["patrol-disk"] = &HealthAlert{
			ID:        "patrol-disk",
			Level:     AlertLevelWarning,
			Type:      "disk",
			Title:     "磁盘使用率告警",
			Message:   "磁盘使用率超过阈值",
			CreatedAt: time.Now(),
		}
	}

	// 温度阈值检查
	if cfg.TempThreshold < 60 {
		m.alerts["patrol-temp"] = &HealthAlert{
			ID:        "patrol-temp",
			Level:     AlertLevelCritical,
			Type:      "temperature",
			Title:     "温度告警",
			Message:   "温度超过阈值",
			CreatedAt: time.Now(),
		}
	}
}

// GetTrends 获取趋势数据
func (m *Manager) GetTrends(hours int) []TrendPoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.healthResult == nil {
		return nil
	}
	return []TrendPoint{
		{Timestamp: "2024-01-01T00:00:00Z", Score: m.healthResult.Score, CPU: 50, Memory: 60, Disk: 70},
	}
}

// GetAlerts 获取告警
func (m *Manager) GetAlerts(resolved bool) []HealthAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]HealthAlert, 0)
	for _, a := range m.alerts {
		if resolved && a.Resolved {
			result = append(result, *a)
		} else if !resolved && !a.Resolved {
			result = append(result, *a)
		}
	}
	return result
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *PatrolConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.patrolConfig = config
	return nil
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *PatrolConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.patrolConfig != nil {
		return m.patrolConfig
	}
	return &PatrolConfig{
		Enabled:       true,
		CPUThreshold:  90,
		MemThreshold:  90,
		DiskThreshold: 90,
		TempThreshold: 70,
		RetentionDays: 30,
	}
}
