package smarthdd

import (
	"fmt"
	"sync"
	"time"
)

// SmartHDDManager 磁盘健康管理器.
type SmartHDDManager struct {
	mu     sync.RWMutex
	disks  map[string]*DiskInfo
	alerts []*HealthAlert
	config *SmartConfig
}

// SmartConfig 配置.
type SmartConfig struct {
	TempThreshold    int  `json:"temp_threshold_celsius"`
	ReallocThreshold int  `json:"realloc_threshold"`
	PendingThreshold int  `json:"pending_threshold"`
	ScanInterval     int  `json:"scan_interval_hours"`
	AlertEnabled     bool `json:"alert_enabled"`
}

// DiskInfo 磁盘信息.
type DiskInfo struct {
	ID             string            `json:"id"`
	Device         string            `json:"device"`
	Model          string            `json:"model"`
	Serial         string            `json:"serial"`
	Size           int64             `json:"size_bytes"`
	Health         HealthStatus      `json:"health"`
	Temperature    int               `json:"temperature_celsius"`
	PowerOnHours   int64             `json:"power_on_hours"`
	PowerCycles    int64             `json:"power_cycles"`
	ReallocSectors int64             `json:"realloc_sectors"`
	PendingSectors int64             `json:"pending_sectors"`
	UNCorrectable  int64             `json:"uncorrectable_errors"`
	SeekErrors     int64             `json:"seek_errors"`
	ReadErrors     int64             `json:"read_errors"`
	WriteErrors    int64             `json:"write_errors"`
	SMARTPassed    bool              `json:"smart_passed"`
	LastScan       time.Time         `json:"last_scan"`
	LastSMART      time.Time         `json:"last_smart_check"`
	Attributes     []*SMARTAttribute `json:"attributes"`
}

// SMARTAttribute SMART属性.
type SMARTAttribute struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Value     int    `json:"value"`
	Worst     int    `json:"worst"`
	Threshold int    `json:"threshold"`
	RawValue  int64  `json:"raw_value"`
	Status    string `json:"status"`
}

// HealthStatus 健康状态.
type HealthStatus string

const (
	HealthGood     HealthStatus = "good"
	HealthWarning  HealthStatus = "warning"
	HealthCritical HealthStatus = "critical"
	HealthUnknown  HealthStatus = "unknown"
)

// HealthAlert 健康告警.
type HealthAlert struct {
	ID         string      `json:"id"`
	DiskID     string      `json:"disk_id"`
	Device     string      `json:"device"`
	Level      AlertLevel  `json:"level"`
	Message    string      `json:"message"`
	Value      interface{} `json:"value"`
	Threshold  interface{} `json:"threshold"`
	CreatedAt  time.Time   `json:"created_at"`
	Resolved   bool        `json:"resolved"`
	ResolvedAt *time.Time  `json:"resolved_at,omitempty"`
}

// AlertLevel 告警级别.
type AlertLevel string

const (
	LevelInfo     AlertLevel = "info"
	LevelWarning  AlertLevel = "warning"
	LevelCritical AlertLevel = "critical"
)

// DiskStats 磁盘统计.
type DiskStats struct {
	TotalDisks    int            `json:"total_disks"`
	HealthyDisks  int            `json:"healthy_disks"`
	WarningDisks  int            `json:"warning_disks"`
	CriticalDisks int            `json:"critical_disks"`
	TotalCapacity int64          `json:"total_capacity_bytes"`
	UsedCapacity  int64          `json:"used_capacity_bytes"`
	AvgTemp       float64        `json:"avg_temperature"`
	MaxTemp       int            `json:"max_temperature"`
	Alerts        int            `json:"active_alerts"`
	ByHealth      map[string]int `json:"by_health"`
}

// NewSmartHDDManager 创建管理器.
func NewSmartHDDManager(config *SmartConfig) *SmartHDDManager {
	if config == nil {
		config = &SmartConfig{
			TempThreshold:    55,
			ReallocThreshold: 100,
			PendingThreshold: 50,
			ScanInterval:     24,
			AlertEnabled:     true,
		}
	}
	return &SmartHDDManager{
		disks:  make(map[string]*DiskInfo),
		alerts: make([]*HealthAlert, 0),
		config: config,
	}
}

// RegisterDisk 注册磁盘.
func (m *SmartHDDManager) RegisterDisk(disk *DiskInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if disk.Device == "" {
		return fmt.Errorf("设备路径不能为空")
	}

	if disk.ID == "" {
		disk.ID = fmt.Sprintf("disk_%d", time.Now().UnixNano())
	}

	disk.LastScan = time.Now()
	m.disks[disk.ID] = disk

	// 检查健康状态
	m.checkDiskHealth(disk)

	return nil
}

// UnregisterDisk 注销磁盘.
func (m *SmartHDDManager) UnregisterDisk(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.disks[id]; !exists {
		return fmt.Errorf("磁盘不存在: %s", id)
	}

	delete(m.disks, id)
	return nil
}

// GetDisk 获取磁盘信息.
func (m *SmartHDDManager) GetDisk(id string) (*DiskInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, exists := m.disks[id]
	if !exists {
		return nil, fmt.Errorf("磁盘不存在: %s", id)
	}

	return disk, nil
}

// ListDisks 列出所有磁盘.
func (m *SmartHDDManager) ListDisks() []*DiskInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disks := make([]*DiskInfo, 0, len(m.disks))
	for _, disk := range m.disks {
		disks = append(disks, disk)
	}
	return disks
}

// UpdateDiskStats 更新磁盘状态.
func (m *SmartHDDManager) UpdateDiskStats(id string, stats *DiskInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	disk, exists := m.disks[id]
	if !exists {
		return fmt.Errorf("磁盘不存在: %s", id)
	}

	if stats.Temperature > 0 {
		disk.Temperature = stats.Temperature
	}
	if stats.ReallocSectors >= 0 {
		disk.ReallocSectors = stats.ReallocSectors
	}
	if stats.PendingSectors >= 0 {
		disk.PendingSectors = stats.PendingSectors
	}
	if stats.SMARTPassed {
		disk.SMARTPassed = stats.SMARTPassed
	}

	disk.LastSMART = time.Now()

	// 重新检查健康状态
	m.checkDiskHealth(disk)

	return nil
}

// ScanDisk 扫描磁盘.
func (m *SmartHDDManager) ScanDisk(id string) (*DiskInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	disk, exists := m.disks[id]
	if !exists {
		return nil, fmt.Errorf("磁盘不存在: %s", id)
	}

	disk.LastScan = time.Now()
	m.checkDiskHealth(disk)

	return disk, nil
}

// ScanAll 扫描所有磁盘.
func (m *SmartHDDManager) ScanAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, disk := range m.disks {
		disk.LastScan = time.Now()
		m.checkDiskHealth(disk)
	}
}

// GetStats 获取统计信息.
func (m *SmartHDDManager) GetStats() *DiskStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &DiskStats{
		TotalDisks: len(m.disks),
		ByHealth:   make(map[string]int),
	}

	totalTemp := 0
	for _, disk := range m.disks {
		stats.TotalCapacity += disk.Size
		totalTemp += disk.Temperature

		if disk.Temperature > stats.MaxTemp {
			stats.MaxTemp = disk.Temperature
		}

		switch disk.Health {
		case HealthGood:
			stats.HealthyDisks++
		case HealthWarning:
			stats.WarningDisks++
		case HealthCritical:
			stats.CriticalDisks++
		}

		stats.ByHealth[string(disk.Health)]++
	}

	if stats.TotalDisks > 0 {
		stats.AvgTemp = float64(totalTemp) / float64(stats.TotalDisks)
	}

	// 统计活跃告警
	for _, alert := range m.alerts {
		if !alert.Resolved {
			stats.Alerts++
		}
	}

	return stats
}

// GetAlerts 获取告警列表.
func (m *SmartHDDManager) GetAlerts(resolved bool) []*HealthAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]*HealthAlert, 0)
	for _, alert := range m.alerts {
		if alert.Resolved == resolved {
			alerts = append(alerts, alert)
		}
	}
	return alerts
}

// ResolveAlert 解决告警.
func (m *SmartHDDManager) ResolveAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, alert := range m.alerts {
		if alert.ID == id && !alert.Resolved {
			alert.Resolved = true
			now := time.Now()
			alert.ResolvedAt = &now
			return nil
		}
	}

	return fmt.Errorf("告警不存在或已解决: %s", id)
}

// checkDiskHealth 检查磁盘健康.
func (m *SmartHDDManager) checkDiskHealth(disk *DiskInfo) {
	// 检查温度
	if disk.Temperature >= m.config.TempThreshold {
		disk.Health = HealthCritical
		m.addAlert(disk, LevelCritical, fmt.Sprintf("温度过高: %d°C (阈值: %d°C)", disk.Temperature, m.config.TempThreshold), disk.Temperature, m.config.TempThreshold)
	} else if disk.Temperature >= m.config.TempThreshold-5 {
		if disk.Health != HealthCritical {
			disk.Health = HealthWarning
		}
	}

	// 检查重新分配扇区
	if disk.ReallocSectors > int64(m.config.ReallocThreshold) {
		disk.Health = HealthCritical
		m.addAlert(disk, LevelCritical, fmt.Sprintf("重新分配扇区过多: %d (阈值: %d)", disk.ReallocSectors, m.config.ReallocThreshold), disk.ReallocSectors, m.config.ReallocThreshold)
	} else if disk.ReallocSectors > int64(m.config.ReallocThreshold/2) {
		if disk.Health != HealthCritical {
			disk.Health = HealthWarning
		}
	}

	// 检查待处理扇区
	if disk.PendingSectors > int64(m.config.PendingThreshold) {
		disk.Health = HealthCritical
		m.addAlert(disk, LevelCritical, fmt.Sprintf("待处理扇区过多: %d (阈值: %d)", disk.PendingSectors, m.config.PendingThreshold), disk.PendingSectors, m.config.PendingThreshold)
	}

	// 检查SMART状态
	if !disk.SMARTPassed {
		disk.Health = HealthCritical
		m.addAlert(disk, LevelCritical, "SMART自检失败", nil, nil)
	}

	// 如果没有问题，标记为健康
	if disk.Health == HealthUnknown || disk.Health == "" {
		disk.Health = HealthGood
	}
}

// addAlert 添加告警.
func (m *SmartHDDManager) addAlert(disk *DiskInfo, level AlertLevel, message string, value, threshold interface{}) {
	if !m.config.AlertEnabled {
		return
	}

	// 检查是否已有相同告警
	for _, alert := range m.alerts {
		if alert.DiskID == disk.ID && alert.Message == message && !alert.Resolved {
			return
		}
	}

	alert := &HealthAlert{
		ID:        fmt.Sprintf("alert_%d", time.Now().UnixNano()),
		DiskID:    disk.ID,
		Device:    disk.Device,
		Level:     level,
		Message:   message,
		Value:     value,
		Threshold: threshold,
		CreatedAt: time.Now(),
	}

	m.alerts = append(m.alerts, alert)
}
