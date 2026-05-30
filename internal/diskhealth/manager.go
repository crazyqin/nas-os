// Package diskhealth 磁盘健康监控模块
// 学习 TrueNAS / 群晖的存储监控功能
package diskhealth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// Disk 磁盘信息
type Disk struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Model        string    `json:"model"`
	Serial       string    `json:"serial"`
	Size         int64     `json:"size"`
	SectorSize   int       `json:"sector_size"`
	Interface    string    `json:"interface"` // SATA, NVMe, USB, etc.
	RPM          int       `json:"rpm,omitempty"` // HDD 转速
	IsSSD        bool      `json:"is_ssd"`
	Temperature  int       `json:"temperature"` // 摄氏度
	PowerOnHours int64     `json:"power_on_hours"`
	PowerCycles  int64     `json:"power_cycles"`
	Status       string    `json:"status"` // healthy, warning, critical, failed
	Health       float64   `json:"health"` // 0-100 健康度
	Performance  float64   `json:"performance"` // 0-100 性能分
	TBW          int64     `json:"tbw,omitempty"` // Total Bytes Written (SSD)
	TBWLimit     int64     `json:"tbw_limit,omitempty"` // TBW 限制
	LastCheck    time.Time `json:"last_check"`
	Errors       []DiskError `json:"errors,omitempty"`
	Partitions   []Partition `json:"partitions"`
}

// Partition 分区
type Partition struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	Used       int64  `json:"used"`
	Available  int64  `json:"available"`
	MountPoint string `json:"mount_point"`
	FileSystem string `json:"filesystem"`
	Label      string `json:"label"`
	UUID       string `json:"uuid"`
}

// DiskError 磁盘错误
type DiskError struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"` // read, write, seek, timeout, etc.
	Sector      int64     `json:"sector,omitempty"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"` // info, warning, critical
}

// SMARTInfo SMART 信息
type SMARTInfo struct {
	DiskID       string       `json:"disk_id"`
	Status       string       `json:"status"` // passed, failed, unknown
	Temperature  int          `json:"temperature"`
	PowerOnHours int64        `json:"power_on_hours"`
	PowerCycles  int64        `json:"power_cycles"`
	Attributes   []SMARTAttr  `json:"attributes"`
	LastUpdate   time.Time    `json:"last_update"`
}

// SMARTAttr SMART 属性
type SMARTAttr struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Value      int    `json:"value"`
	Worst      int    `json:"worst"`
	Threshold  int    `json:"threshold"`
	RawValue   int64  `json:"raw_value"`
	Status     string `json:"status"` // ok, warning, critical
}

// Pool 存储池
type Pool struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"` // online, degraded, faulted, offline
	Size        int64     `json:"size"`
	Used        int64     `json:"used"`
	Available   int64     `json:"available"`
	Health      string    `json:"health"` // healthy, warning, critical
	RAIDLevel   string    `json:"raid_level"`
	Disks       []string  `json:"disk_ids"`
	ScrubStatus string    `json:"scrub_status"`
	LastScrub   time.Time `json:"last_scrub"`
	CreatedAt   time.Time `json:"created_at"`
}

// Alert 告警
type Alert struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // disk, pool, temperature, smart, etc.
	Severity  string    `json:"severity"` // info, warning, critical
	DiskID    string    `json:"disk_id,omitempty"`
	PoolID    string    `json:"pool_id,omitempty"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Acked     bool      `json:"acked"`
	AckedAt   time.Time `json:"acked_at,omitempty"`
}

// AlertRule 告警规则
type AlertRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Condition   string `json:"condition"`
	Threshold   float64 `json:"threshold"`
	Severity    string `json:"severity"`
	Enabled     bool   `json:"enabled"`
	NotifyEmail bool   `json:"notify_email"`
	NotifyWebhook bool `json:"notify_webhook"`
}

// Manager 磁盘健康管理器
type Manager struct {
	mu       sync.RWMutex
	disks    map[string]*Disk
	pools    map[string]*Pool
	alerts   map[string]*Alert
	rules    map[string]*AlertRule
	smart    map[string]*SMARTInfo
	stopChan chan struct{}
}

// NewManager 创建磁盘健康管理器
func NewManager() *Manager {
	m := &Manager{
		disks:    make(map[string]*Disk),
		pools:    make(map[string]*Pool),
		alerts:   make(map[string]*Alert),
		rules:    make(map[string]*AlertRule),
		smart:    make(map[string]*SMARTInfo),
		stopChan: make(chan struct{}),
	}
	m.loadDefaultRules()
	go m.startMonitoring()
	return m
}

// ScanDisks 扫描磁盘
func (m *Manager) ScanDisks(ctx context.Context) ([]Disk, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// TODO: 实际扫描磁盘
	// 返回已知磁盘
	disks := make([]Disk, 0, len(m.disks))
	for _, d := range m.disks {
		disks = append(disks, *d)
	}

	return disks, nil
}

// GetDisk 获取磁盘详情
func (m *Manager) GetDisk(ctx context.Context, id string) (*Disk, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, exists := m.disks[id]
	if !exists {
		return nil, fmt.Errorf("disk not found: %s", id)
	}

	return disk, nil
}

// GetSMART 获取 SMART 信息
func (m *Manager) GetSMART(ctx context.Context, diskID string) (*SMARTInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	smart, exists := m.smart[diskID]
	if !exists {
		return nil, fmt.Errorf("SMART info not found for disk: %s", diskID)
	}

	return smart, nil
}

// RunSMARTTest 运行 SMART 测试
func (m *Manager) RunSMARTTest(ctx context.Context, diskID, testType string) error {
	m.mu.RLock()
	_, exists := m.disks[diskID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("disk not found: %s", diskID)
	}

	validTests := map[string]bool{
		"short":  true,
		"long":   true,
		"conveyance": true,
	}

	if !validTests[testType] {
		return fmt.Errorf("invalid test type: %s", testType)
	}

	log.Printf("Starting SMART %s test for disk: %s", testType, diskID)
	// TODO: 实际运行测试

	return nil
}

// CreatePool 创建存储池
func (m *Manager) CreatePool(ctx context.Context, name, raidLevel string, diskIDs []string) (*Pool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证磁盘
	for _, id := range diskIDs {
		if _, exists := m.disks[id]; !exists {
			return nil, fmt.Errorf("disk not found: %s", id)
		}
	}

	// 计算容量
	totalSize := int64(0)
	for _, id := range diskIDs {
		totalSize += m.disks[id].Size
	}

	pool := &Pool{
		ID:        generateID(),
		Name:      name,
		Status:    "online",
		Size:      totalSize,
		Health:    "healthy",
		RAIDLevel: raidLevel,
		Disks:     diskIDs,
		CreatedAt: time.Now(),
	}

	m.pools[pool.ID] = pool
	log.Printf("Storage pool created: %s (%s)", name, pool.ID)

	return pool, nil
}

// GetPool 获取存储池详情
func (m *Manager) GetPool(ctx context.Context, id string) (*Pool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, exists := m.pools[id]
	if !exists {
		return nil, fmt.Errorf("pool not found: %s", id)
	}

	return pool, nil
}

// ListPools 列出存储池
func (m *Manager) ListPools(ctx context.Context) ([]Pool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pools := make([]Pool, 0, len(m.pools))
	for _, p := range m.pools {
		pools = append(pools, *p)
	}

	return pools, nil
}

// StartScrub 开始清洗
func (m *Manager) StartScrub(ctx context.Context, poolID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return fmt.Errorf("pool not found: %s", poolID)
	}

	pool.ScrubStatus = "running"
	pool.LastScrub = time.Now()
	log.Printf("Scrub started for pool: %s", pool.Name)

	return nil
}

// ListAlerts 列出告警
func (m *Manager) ListAlerts(ctx context.Context, severity string, unackedOnly bool) ([]Alert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]Alert, 0)
	for _, a := range m.alerts {
		if severity != "" && a.Severity != severity {
			continue
		}
		if unackedOnly && a.Acked {
			continue
		}
		alerts = append(alerts, *a)
	}

	return alerts, nil
}

// AckAlert 确认告警
func (m *Manager) AckAlert(ctx context.Context, alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, exists := m.alerts[alertID]
	if !exists {
		return fmt.Errorf("alert not found: %s", alertID)
	}

	alert.Acked = true
	alert.AckedAt = time.Now()

	return nil
}

// AddAlertRule 添加告警规则
func (m *Manager) AddAlertRule(ctx context.Context, rule *AlertRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = generateID()
	}

	m.rules[rule.ID] = rule
	log.Printf("Alert rule added: %s", rule.Name)

	return nil
}

// GetStats 获取统计信息
func (m *Manager) GetStats(ctx context.Context) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalDiskSize := int64(0)
	healthyCount := 0
	warningCount := 0
	criticalCount := 0

	for _, d := range m.disks {
		totalDiskSize += d.Size
		switch d.Status {
		case "healthy":
			healthyCount++
		case "warning":
			warningCount++
		case "critical", "failed":
			criticalCount++
		}
	}

	totalPoolSize := int64(0)
	poolHealthy := 0
	poolDegraded := 0

	for _, p := range m.pools {
		totalPoolSize += p.Size
		if p.Health == "healthy" {
			poolHealthy++
		} else {
			poolDegraded++
		}
	}

	unackedAlerts := 0
	for _, a := range m.alerts {
		if !a.Acked {
			unackedAlerts++
		}
	}

	return map[string]interface{}{
		"total_disks":       len(m.disks),
		"disks_healthy":     healthyCount,
		"disks_warning":     warningCount,
		"disks_critical":    criticalCount,
		"total_disk_size":   totalDiskSize,
		"total_pools":       len(m.pools),
		"pools_healthy":     poolHealthy,
		"pools_degraded":    poolDegraded,
		"total_pool_size":   totalPoolSize,
		"active_alerts":     unackedAlerts,
		"alert_rules":       len(m.rules),
	}, nil
}

// 内部方法

func (m *Manager) loadDefaultRules() {
	defaultRules := []*AlertRule{
		{
			ID:        "temp_high",
			Name:      "High Temperature",
			Type:      "temperature",
			Condition: "greater_than",
			Threshold: 60,
			Severity:  "warning",
			Enabled:   true,
		},
		{
			ID:        "temp_critical",
			Name:      "Critical Temperature",
			Type:      "temperature",
			Condition: "greater_than",
			Threshold: 70,
			Severity:  "critical",
			Enabled:   true,
		},
		{
			ID:        "smart_warning",
			Name:      "SMART Warning",
			Type:      "smart",
			Condition: "equals",
			Threshold: 0,
			Severity:  "warning",
			Enabled:   true,
		},
		{
			ID:        "disk_space_low",
			Name:      "Low Disk Space",
			Type:      "disk_space",
			Condition: "less_than",
			Threshold: 10, // 10%
			Severity:  "warning",
			Enabled:   true,
		},
	}

	for _, r := range defaultRules {
		m.rules[r.ID] = r
	}
}

func (m *Manager) startMonitoring() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkHealth()
		case <-m.stopChan:
			return
		}
	}
}

func (m *Manager) checkHealth() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查温度
	for _, disk := range m.disks {
		if disk.Temperature > 70 {
			m.createAlert("temperature", "critical", disk.ID, "", 
				fmt.Sprintf("Disk %s temperature critical: %d°C", disk.Name, disk.Temperature))
		} else if disk.Temperature > 60 {
			m.createAlert("temperature", "warning", disk.ID, "",
				fmt.Sprintf("Disk %s temperature high: %d°C", disk.Name, disk.Temperature))
		}
	}

	// 检查 SMART 状态
	for _, smart := range m.smart {
		if smart.Status == "failed" {
			m.createAlert("smart", "critical", smart.DiskID, "",
				fmt.Sprintf("Disk %s SMART test failed", smart.DiskID))
		}
	}
}

func (m *Manager) createAlert(alertType, severity, diskID, poolID, message string) {
	alert := &Alert{
		ID:        generateID(),
		Type:      alertType,
		Severity:  severity,
		DiskID:    diskID,
		PoolID:    poolID,
		Message:   message,
		Timestamp: time.Now(),
	}

	m.alerts[alert.ID] = alert
	log.Printf("Alert created: %s - %s", severity, message)
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Export 导出磁盘信息
func (m *Manager) Export(ctx context.Context) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := map[string]interface{}{
		"disks":  m.disks,
		"pools":  m.pools,
		"smart":  m.smart,
		"alerts": m.alerts,
	}

	return json.MarshalIndent(data, "", "  ")
}
