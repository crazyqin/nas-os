// Package diskhealth 硬盘健康监控
// S.M.A.R.T. 数据采集、健康评分、预警
package diskhealth

import (
	"sync"
	"time"
)

// DiskInfo 磁盘信息
type DiskInfo struct {
	Device       string    `json:"device"`
	Model        string    `json:"model"`
	Serial       string    `json:"serial"`
	Size         int64     `json:"size"`
	Temperature  int       `json:"temperature"`
	PowerOnHours int64     `json:"power_on_hours"`
	HealthScore  float64   `json:"health_score"` // 0-100
	SmartStatus  string    `json:"smart_status"` // passed, failed, unknown
	LastCheck    time.Time `json:"last_check"`
}

// SmartAttribute S.M.A.R.T. 属性
type SmartAttribute struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Value     int    `json:"value"`
	Worst     int    `json:"worst"`
	Threshold int    `json:"threshold"`
	RawValue  int64  `json:"raw_value"`
	Status    string `json:"status"` // ok, warning, critical
}

// HealthReport 健康报告
type HealthReport struct {
	Device     string           `json:"device"`
	Score      float64          `json:"score"`
	Status     string           `json:"status"` // healthy, warning, critical
	Attributes []SmartAttribute `json:"attributes"`
	Warnings   []string         `json:"warnings"`
	Timestamp  time.Time        `json:"timestamp"`
}

// AlertConfig 告警配置
type AlertConfig struct {
	TemperatureThreshold        int     `json:"temperature_threshold"`
	HealthScoreThreshold        float64 `json:"health_score_threshold"`
	ReallocatedSectorsThreshold int64   `json:"reallocated_sectors_threshold"`
}

// HistoryRecord 历史记录
type HistoryRecord struct {
	Timestamp   time.Time `json:"timestamp"`
	HealthScore float64   `json:"health_score"`
	Temperature int       `json:"temperature"`
	Status      string    `json:"status"`
}

// Manager 硬盘健康管理器
type Manager struct {
	mu      sync.RWMutex
	disks   map[string]*DiskInfo
	history map[string][]HistoryRecord
	config  AlertConfig
}

// NewManager 创建健康管理器
func NewManager() *Manager {
	return &Manager{
		disks:   make(map[string]*DiskInfo),
		history: make(map[string][]HistoryRecord),
		config: AlertConfig{
			TemperatureThreshold:        60,
			HealthScoreThreshold:        70,
			ReallocatedSectorsThreshold: 100,
		},
	}
}

// ScanDisks 扫描磁盘
func (m *Manager) ScanDisks() []DiskInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disks := make([]DiskInfo, 0, len(m.disks))
	for _, d := range m.disks {
		disks = append(disks, *d)
	}
	return disks
}

// GetDiskInfo 获取磁盘信息
func (m *Manager) GetDiskInfo(device string) (*DiskInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, exists := m.disks[device]
	if !exists {
		return nil, false
	}
	return disk, true
}

// GetHealthReport 获取健康报告
func (m *Manager) GetHealthReport(device string) (*HealthReport, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, exists := m.disks[device]
	if !exists {
		return nil, false
	}

	report := &HealthReport{
		Device:    device,
		Score:     disk.HealthScore,
		Timestamp: time.Now(),
	}

	// 确定状态
	if disk.HealthScore >= 80 {
		report.Status = "healthy"
	} else if disk.HealthScore >= 60 {
		report.Status = "warning"
	} else {
		report.Status = "critical"
	}

	// 检查警告
	report.Warnings = m.checkWarnings(disk)

	return report, true
}

// CheckAlerts 检查告警
func (m *Manager) CheckAlerts() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var alerts []string
	for _, disk := range m.disks {
		if disk.Temperature > m.config.TemperatureThreshold {
			alerts = append(alerts, "磁盘 "+disk.Device+" 温度过高: "+string(rune(disk.Temperature))+"°C")
		}
		if disk.HealthScore < m.config.HealthScoreThreshold {
			alerts = append(alerts, "磁盘 "+disk.Device+" 健康评分过低")
		}
	}
	return alerts
}

// GetHistory 获取历史记录
func (m *Manager) GetHistory(device string) []HistoryRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.history[device]
}

// GetConfig 获取告警配置
func (m *Manager) GetConfig() AlertConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.config
}

// UpdateConfig 更新告警配置
func (m *Manager) UpdateConfig(config AlertConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
}

// AddDisk 添加磁盘
func (m *Manager) AddDisk(info DiskInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()

	info.LastCheck = time.Now()
	m.disks[info.Device] = &info
}

// RemoveDisk 移除磁盘
func (m *Manager) RemoveDisk(device string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.disks, device)
	delete(m.history, device)
}

// checkWarnings 检查警告
func (m *Manager) checkWarnings(disk *DiskInfo) []string {
	var warnings []string

	if disk.Temperature > m.config.TemperatureThreshold {
		warnings = append(warnings, "温度过高")
	}
	if disk.HealthScore < m.config.HealthScoreThreshold {
		warnings = append(warnings, "健康评分过低")
	}
	if disk.SmartStatus == "failed" {
		warnings = append(warnings, "S.M.A.R.T. 检测失败")
	}
	return warnings
}
