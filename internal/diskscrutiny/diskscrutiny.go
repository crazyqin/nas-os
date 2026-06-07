// Package diskscrutiny 磁盘健康监控仪表盘
// 灵感来源: TrueNAS Scrutiny 集成 + 群晖 HDD/SSD 健康监控
package diskscrutiny

import (
	"fmt"
	"sync"
	"time"
)

// DiskStatus 磁盘状态
type DiskStatus string

const (
	DiskStatusHealthy  DiskStatus = "healthy"
	DiskStatusWarning  DiskStatus = "warning"
	DiskStatusCritical DiskStatus = "critical"
	DiskStatusUnknown  DiskStatus = "unknown"
)

// SMARTAttribute SMART 属性
type SMARTAttribute struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Value     int    `json:"value"`
	Worst     int    `json:"worst"`
	Threshold int    `json:"threshold"`
	RawValue  int64  `json:"raw_value"`
	Status    string `json:"status"` // ok, warning, critical
}

// DiskInfo 磁盘信息
type DiskInfo struct {
	Device       string           `json:"device"`
	Model        string           `json:"model"`
	Serial       string           `json:"serial"`
	Interface    string           `json:"interface"` // SATA, SAS, NVMe, USB
	Capacity     int64            `json:"capacity_bytes"`
	Temperature  int              `json:"temperature_celsius"`
	PowerOnHours int64            `json:"power_on_hours"`
	Status       DiskStatus       `json:"status"`
	SMART        []SMARTAttribute `json:"smart_attributes"`
	LastChecked  time.Time        `json:"last_checked"`
	HealthScore  float64          `json:"health_score"` // 0-100
}

// AlertRule 告警规则
type AlertRule struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Attribute string  `json:"attribute"` // temperature, reallocated_sectors, etc.
	Operator  string  `json:"operator"`  // gt, lt, eq, gte
	Threshold float64 `json:"threshold"`
	Severity  string  `json:"severity"` // info, warning, critical
	Enabled   bool    `json:"enabled"`
	Guidance  string  `json:"guidance"` // 引导式解决建议
}

// DiskMonitor 磁盘健康监控器
type DiskMonitor struct {
	mu         sync.RWMutex
	disks      map[string]*DiskInfo
	alerts     []AlertRule
	history    map[string][]HealthSnapshot
	maxHistory int
}

// HealthSnapshot 健康快照
type HealthSnapshot struct {
	Timestamp   time.Time  `json:"timestamp"`
	Temperature int        `json:"temperature"`
	HealthScore float64    `json:"health_score"`
	Status      DiskStatus `json:"status"`
}

// NewDiskMonitor 创建磁盘监控器
func NewDiskMonitor() *DiskMonitor {
	return &DiskMonitor{
		disks:      make(map[string]*DiskInfo),
		alerts:     defaultAlertRules(),
		history:    make(map[string][]HealthSnapshot),
		maxHistory: 1440, // 每分钟一次，保留24小时
	}
}

// RegisterDisk 注册磁盘
func (dm *DiskMonitor) RegisterDisk(info *DiskInfo) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	info.LastChecked = time.Now()
	if info.HealthScore == 0 {
		info.HealthScore = 100.0
	}
	if info.Status == "" {
		info.Status = evaluateStatus(info.HealthScore, info.Temperature)
	}
	dm.disks[info.Device] = info
	dm.recordSnapshot(info)
}

// UpdateSMART 更新 SMART 数据
func (dm *DiskMonitor) UpdateSMART(device string, attrs []SMARTAttribute) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	disk, exists := dm.disks[device]
	if !exists {
		return fmt.Errorf("disk %s not registered", device)
	}

	disk.SMART = attrs
	disk.LastChecked = time.Now()
	disk.HealthScore = calculateHealthScore(attrs)
	disk.Status = evaluateStatus(disk.HealthScore, disk.Temperature)
	dm.recordSnapshot(disk)

	return dm.checkAlerts(disk)
}

// GetDisk 获取磁盘信息
func (dm *DiskMonitor) GetDisk(device string) (*DiskInfo, bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	disk, ok := dm.disks[device]
	return disk, ok
}

// GetAllDisks 获取所有磁盘
func (dm *DiskMonitor) GetAllDisks() []*DiskInfo {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	disks := make([]*DiskInfo, 0, len(dm.disks))
	for _, d := range dm.disks {
		disks = append(disks, d)
	}
	return disks
}

// GetDashboard 获取仪表盘数据
func (dm *DiskMonitor) GetDashboard() *Dashboard {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	dash := &Dashboard{
		GeneratedAt: time.Now(),
		Disks:       make([]*DiskSummary, 0),
	}

	for device, disk := range dm.disks {
		summary := &DiskSummary{
			Device:       device,
			Model:        disk.Model,
			Status:       disk.Status,
			Temperature:  disk.Temperature,
			HealthScore:  disk.HealthScore,
			PowerOnHours: disk.PowerOnHours,
		}
		dash.TotalDisks++
		switch disk.Status {
		case DiskStatusHealthy:
			dash.HealthyCount++
		case DiskStatusWarning:
			dash.WarningCount++
		case DiskStatusCritical:
			dash.CriticalCount++
		}
		dash.Disks = append(dash.Disks, summary)
	}

	return dash
}

// GetHistory 获取磁盘历史
func (dm *DiskMonitor) GetHistory(device string, limit int) []HealthSnapshot {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	snaps := dm.history[device]
	if limit > 0 && limit < len(snaps) {
		return snaps[len(snaps)-limit:]
	}
	return snaps
}

// AddAlertRule 添加告警规则
func (dm *DiskMonitor) AddAlertRule(rule AlertRule) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.alerts = append(dm.alerts, rule)
}

// recordSnapshot 记录快照
func (dm *DiskMonitor) recordSnapshot(disk *DiskInfo) {
	snap := HealthSnapshot{
		Timestamp:   time.Now(),
		Temperature: disk.Temperature,
		HealthScore: disk.HealthScore,
		Status:      disk.Status,
	}
	dm.history[disk.Device] = append(dm.history[disk.Device], snap)
	if len(dm.history[disk.Device]) > dm.maxHistory {
		dm.history[disk.Device] = dm.history[disk.Device][1:]
	}
}

// checkAlerts 检查告警
func (dm *DiskMonitor) checkAlerts(disk *DiskInfo) error {
	for _, rule := range dm.alerts {
		if !rule.Enabled {
			continue
		}
		if evaluateRule(disk, rule) {
			// 告警触发，记录日志（实际应推送通知）
			fmt.Printf("[DISK ALERT] %s: %s - %s\n", disk.Device, rule.Name, rule.Guidance)
		}
	}
	return nil
}

// Dashboard 仪表盘
type Dashboard struct {
	GeneratedAt   time.Time      `json:"generated_at"`
	TotalDisks    int            `json:"total_disks"`
	HealthyCount  int            `json:"healthy_count"`
	WarningCount  int            `json:"warning_count"`
	CriticalCount int            `json:"critical_count"`
	Disks         []*DiskSummary `json:"disks"`
}

// DiskSummary 磁盘摘要
type DiskSummary struct {
	Device       string     `json:"device"`
	Model        string     `json:"model"`
	Status       DiskStatus `json:"status"`
	Temperature  int        `json:"temperature_celsius"`
	HealthScore  float64    `json:"health_score"`
	PowerOnHours int64      `json:"power_on_hours"`
}

func calculateHealthScore(attrs []SMARTAttribute) float64 {
	if len(attrs) == 0 {
		return 50.0
	}
	total := 0.0
	count := 0
	for _, attr := range attrs {
		if attr.Threshold > 0 {
			ratio := float64(attr.Value) / float64(attr.Threshold)
			if ratio > 1.0 {
				ratio = 1.0
			}
			total += ratio * 100
			count++
		}
	}
	if count == 0 {
		return 100.0
	}
	return total / float64(count)
}

func evaluateStatus(score float64, temp int) DiskStatus {
	if temp > 60 {
		return DiskStatusCritical
	}
	if score < 50 {
		return DiskStatusCritical
	}
	if score < 75 || temp > 50 {
		return DiskStatusWarning
	}
	return DiskStatusHealthy
}

func evaluateRule(disk *DiskInfo, rule AlertRule) bool {
	var value float64
	switch rule.Attribute {
	case "temperature":
		value = float64(disk.Temperature)
	case "health_score":
		value = disk.HealthScore
	default:
		return false
	}
	switch rule.Operator {
	case "gt":
		return value > rule.Threshold
	case "lt":
		return value < rule.Threshold
	case "gte":
		return value >= rule.Threshold
	case "eq":
		return value == rule.Threshold
	}
	return false
}

func defaultAlertRules() []AlertRule {
	return []AlertRule{
		{
			ID: "temp-high", Name: "温度过高", Attribute: "temperature",
			Operator: "gt", Threshold: 55, Severity: "warning", Enabled: true,
			Guidance: "检查散热系统，清理风扇灰尘，确保通风良好",
		},
		{
			ID: "temp-critical", Name: "温度危险", Attribute: "temperature",
			Operator: "gt", Threshold: 65, Severity: "critical", Enabled: true,
			Guidance: "立即关机检查！硬盘可能损坏，备份数据后更换硬盘",
		},
		{
			ID: "health-low", Name: "健康度低", Attribute: "health_score",
			Operator: "lt", Threshold: 50, Severity: "critical", Enabled: true,
			Guidance: "硬盘健康度严重下降，建议立即备份数据并更换硬盘",
		},
	}
}
