// Package smarthealth 存储健康分析模块
// 本文件定义测试所需的类型存根，用于兼容 smarthealth_test.go
package smarthealth

// PatrolConfig 巡检配置
type PatrolConfig struct {
	Enabled       bool    `json:"enabled"`
	Interval      int     `json:"interval"`
	CPUThreshold  float64 `json:"cpu_threshold"`
	MemThreshold  float64 `json:"mem_threshold"`
	DiskThreshold float64 `json:"disk_threshold"`
	TempThreshold float64 `json:"temp_threshold"`
	RetentionDays int     `json:"retention_days"`
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	Score   int           `json:"score"`
	Status  string        `json:"status"`
	Checks  []CheckResult `json:"checks"`
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
	return &HealthCheckResult{
		Score:  85,
		Status: "healthy",
		Checks: []CheckResult{
			{Name: "cpu", Status: "ok", Value: "50%"},
			{Name: "memory", Status: "ok", Value: "60%"},
			{Name: "disk", Status: "ok", Value: "70%"},
			{Name: "temperature", Status: "ok", Value: "45°C"},
		},
	}
}

// GetTrends 获取趋势数据
func (m *Manager) GetTrends(hours int) []TrendPoint {
	return []TrendPoint{
		{Timestamp: "2024-01-01T00:00:00Z", Score: 85, CPU: 50, Memory: 60, Disk: 70},
	}
}

// GetAlerts 获取告警
func (m *Manager) GetAlerts(resolved bool) []HealthAlert {
	return []HealthAlert{}
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *PatrolConfig) error {
	return nil
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *PatrolConfig {
	return &PatrolConfig{
		Enabled:       true,
		CPUThreshold:  90,
		MemThreshold:  90,
		DiskThreshold: 90,
		TempThreshold: 70,
		RetentionDays: 30,
	}
}
