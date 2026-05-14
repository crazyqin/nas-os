package resmon

import (
	"sync"
	"time"
)

// SystemMetrics 系统指标快照.
type SystemMetrics struct {
	Timestamp    time.Time     `json:"timestamp"`
	CPU          CPUUsage      `json:"cpu"`
	Memory       MemoryUsage   `json:"memory"`
	Disks        []DiskUsage   `json:"disks"`
	Network      []NetUsage    `json:"network"`
	LoadAvg      [3]float64    `json:"load_avg"`
	Uptime       time.Duration `json:"uptime"`
	ProcessCount int           `json:"process_count"`
	Temperature  float64       `json:"temperature_c"`
}

// CPUUsage CPU 使用率.
type CPUUsage struct {
	TotalPercent float64   `json:"total_percent"`
	PerCore      []float64 `json:"per_core"`
	CoreCount    int       `json:"core_count"`
	UserPercent  float64   `json:"user_percent"`
	SystemPercent float64  `json:"system_percent"`
	IdlePercent  float64   `json:"idle_percent"`
}

// MemoryUsage 内存使用.
type MemoryUsage struct {
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	FreeBytes      uint64  `json:"free_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	CachedBytes    uint64  `json:"cached_bytes"`
	SwapTotalBytes uint64  `json:"swap_total_bytes"`
	SwapUsedBytes  uint64  `json:"swap_used_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

// DiskUsage 磁盘使用.
type DiskUsage struct {
	MountPoint  string  `json:"mount_point"`
	Device      string  `json:"device"`
	FSType      string  `json:"fs_type"`
	TotalBytes  uint64  `json:"total_bytes"`
	UsedBytes   uint64  `json:"used_bytes"`
	FreeBytes   uint64  `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
	InodesTotal uint64  `json:"inodes_total"`
	InodesUsed  uint64  `json:"inodes_used"`
}

// NetUsage 网络使用.
type NetUsage struct {
	Interface  string `json:"interface"`
	RxBytes    uint64 `json:"rx_bytes"`
	TxBytes    uint64 `json:"tx_bytes"`
	RxPackets  uint64 `json:"rx_packets"`
	TxPackets  uint64 `json:"tx_packets"`
	RxRateBps  uint64 `json:"rx_rate_bps"`
	TxRateBps  uint64 `json:"tx_rate_bps"`
}

// AlertLevel 告警级别.
type AlertLevel string

const (
	AlertInfo     AlertLevel = "info"
	AlertWarning  AlertLevel = "warning"
	AlertCritical AlertLevel = "critical"
)

// Alert 资源告警.
type Alert struct {
	ID        string     `json:"id"`
	Level     AlertLevel `json:"level"`
	Resource  string     `json:"resource"`
	Message   string     `json:"message"`
	Value     float64    `json:"value"`
	Threshold float64    `json:"threshold"`
	Timestamp time.Time  `json:"timestamp"`
	Acked     bool       `json:"acked"`
}

// AlertRule 告警规则.
type AlertRule struct {
	ID        string     `json:"id"`
	Resource  string     `json:"resource"`
	Threshold float64    `json:"threshold"`
	Level     AlertLevel `json:"level"`
	Enabled   bool       `json:"enabled"`
	Message   string     `json:"message"`
}

// MonitorConfig 监控配置.
type MonitorConfig struct {
	CollectIntervalS int    `json:"collect_interval_s"`
	HistoryRetention int    `json:"history_retention_hours"`
	MaxHistoryPoints int    `json:"max_history_points"`
	TempThresholdC   float64 `json:"temp_threshold_c"`
	CPUThresholdPct  float64 `json:"cpu_threshold_pct"`
	MemThresholdPct  float64 `json:"mem_threshold_pct"`
	DiskThresholdPct float64 `json:"disk_threshold_pct"`
}

// Manager 资源监控管理器.
type Manager struct {
	mu        sync.RWMutex
	config    *MonitorConfig
	latest    *SystemMetrics
	history   []SystemMetrics
	alerts    []Alert
	alertRules []AlertRule
	maxHistory int
	maxAlerts  int
}

// NewManager 创建资源监控管理器.
func NewManager() *Manager {
	return &Manager{
		config: &MonitorConfig{
			CollectIntervalS: 30,
			HistoryRetention: 24,
			MaxHistoryPoints: 2880,
			TempThresholdC:   80.0,
			CPUThresholdPct:  90.0,
			MemThresholdPct:  85.0,
			DiskThresholdPct: 90.0,
		},
		history:    make([]SystemMetrics, 0, 2880),
		alerts:     make([]Alert, 0, 1000),
		alertRules: make([]AlertRule, 0),
		maxHistory: 2880,
		maxAlerts:  10000,
	}
}

// RecordMetrics 记录指标快照.
func (m *Manager) RecordMetrics(metrics *SystemMetrics) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.latest = metrics
	m.history = append(m.history, *metrics)
	if len(m.history) > m.maxHistory {
		m.history = m.history[1:]
	}

	m.checkAlertsLocked(metrics)
}

// GetLatest 获取最新指标.
func (m *Manager) GetLatest() *SystemMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.latest == nil {
		return nil
	}
	// 返回副本
	latest := *m.latest
	return &latest
}

// GetHistory 获取历史指标.
func (m *Manager) GetHistory(hours int) []SystemMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if hours <= 0 {
		hours = 1
	}
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
	result := make([]SystemMetrics, 0)
	for _, h := range m.history {
		if h.Timestamp.After(cutoff) {
			result = append(result, h)
		}
	}
	return result
}

// GetAlerts 获取告警列表.
func (m *Manager) GetAlerts(unackedOnly bool) []Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Alert, 0)
	for _, a := range m.alerts {
		if unackedOnly && a.Acked {
			continue
		}
		result = append(result, a)
	}
	return result
}

// AckAlert 确认告警.
func (m *Manager) AckAlert(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.alerts {
		if m.alerts[i].ID == id {
			m.alerts[i].Acked = true
			return true
		}
	}
	return false
}

// AddAlertRule 添加告警规则.
func (m *Manager) AddAlertRule(rule AlertRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertRules = append(m.alertRules, rule)
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() *MonitorConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(cfg *MonitorConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
}

func (m *Manager) checkAlertsLocked(metrics *SystemMetrics) {
	cfg := m.config
	now := time.Now()

	if metrics.CPU.TotalPercent > cfg.CPUThresholdPct {
		m.appendAlert(Alert{
			ID:        time.Now().Format("20060102150405.000"),
			Level:     AlertWarning,
			Resource:  "cpu",
			Message:   "CPU 使用率超过阈值",
			Value:     metrics.CPU.TotalPercent,
			Threshold: cfg.CPUThresholdPct,
			Timestamp: now,
		})
	}
	if metrics.Memory.UsedPercent > cfg.MemThresholdPct {
		m.appendAlert(Alert{
			ID:        time.Now().Format("20060102150405.001"),
			Level:     AlertWarning,
			Resource:  "memory",
			Message:   "内存使用率超过阈值",
			Value:     metrics.Memory.UsedPercent,
			Threshold: cfg.MemThresholdPct,
			Timestamp: now,
		})
	}
	for _, d := range metrics.Disks {
		if d.UsedPercent > cfg.DiskThresholdPct {
			m.appendAlert(Alert{
				ID:        time.Now().Format("20060102150405.002"),
				Level:     AlertCritical,
				Resource:  "disk:" + d.MountPoint,
				Message:   "磁盘使用率超过阈值: " + d.MountPoint,
				Value:     d.UsedPercent,
				Threshold: cfg.DiskThresholdPct,
				Timestamp: now,
			})
		}
	}
	if metrics.Temperature > cfg.TempThresholdC {
		m.appendAlert(Alert{
			ID:        time.Now().Format("20060102150405.003"),
			Level:     AlertCritical,
			Resource:  "temperature",
			Message:   "CPU 温度过高",
			Value:     metrics.Temperature,
			Threshold: cfg.TempThresholdC,
			Timestamp: now,
		})
	}
}

func (m *Manager) appendAlert(alert Alert) {
	m.alerts = append(m.alerts, alert)
	if len(m.alerts) > m.maxAlerts {
		m.alerts = m.alerts[1:]
	}
}
