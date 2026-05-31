// Package diskhibernate 磁盘智能休眠引擎
// 基于访问模式学习，智能控制磁盘休眠策略
package diskhibernate

import (
	"fmt"
	"sync"
	"time"
)

// DiskState 磁盘状态
type DiskState string

const (
	StateActive   DiskState = "active"
	StateIdle     DiskState = "idle"
	StateStandby  DiskState = "standby"
	StateSleep    DiskState = "sleep"
	StateSpindown DiskState = "spindown"
)

// Disk 磁盘信息
type Disk struct {
	ID           string    `json:"id"`
	Device       string    `json:"device"`
	Model        string    `json:"model"`
	Serial       string    `json:"serial"`
	Size         int64     `json:"size"`
	State        DiskState `json:"state"`
	Temperature  int       `json:"temperature"`
	PowerOnHours int64     `json:"power_on_hours"`
	LastAccess   time.Time `json:"last_access"`
	AccessCount  int64     `json:"access_count"`
	SpinUpCount  int64     `json:"spin_up_count"`
	TotalIO      int64     `json:"total_io"`
}

// AccessPattern 访问模式
type AccessPattern struct {
	HourlyAccess [24]int     `json:"hourly_access"` // 每小时访问次数
	DailyAccess  [7]int      `json:"daily_access"`  // 每天访问次数
	PeakHours    []int       `json:"peak_hours"`    // 高峰时段
	QuietHours   []int       `json:"quiet_hours"`   // 安静时段
	LastUpdated  time.Time   `json:"last_updated"`
	TotalRecords int         `json:"total_records"`
	AvgInterval  time.Duration `json:"avg_interval"` // 平均访问间隔
}

// HibernatePolicy 休眠策略
type HibernatePolicy struct {
	Enabled          bool          `json:"enabled"`
	IdleTimeout      time.Duration `json:"idle_timeout"`       // 空闲超时
	StandbyTimeout   time.Duration `json:"standby_timeout"`    // 待机超时
	SleepTimeout     time.Duration `json:"sleep_timeout"`      // 休眠超时
	TemperatureThreshold int      `json:"temperature_threshold"` // 温度阈值
	SpinDownLimit    int           `json:"spin_down_limit"`    // 每日休眠次数限制
	SmartEnabled     bool          `json:"smart_enabled"`      // 启用智能学习
	ForceHibernate   bool          `json:"force_hibernate"`    // 强制休眠模式
}

// AccessRecord 访问记录
type AccessRecord struct {
	DiskID    string    `json:"disk_id"`
	Timestamp time.Time `json:"timestamp"`
	IOSize    int64     `json:"io_size"`
	OpType    string    `json:"op_type"` // read/write
}

// Manager 磁盘休眠管理器
type Manager struct {
	mu           sync.RWMutex
	disks        map[string]*Disk
	patterns     map[string]*AccessPattern
	policies     map[string]*HibernatePolicy
	records      []AccessRecord
	maxRecords   int
	learningWindow time.Duration
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		disks:          make(map[string]*Disk),
		patterns:       make(map[string]*AccessPattern),
		policies:       make(map[string]*HibernatePolicy),
		maxRecords:     10000,
		learningWindow: 7 * 24 * time.Hour, // 7天学习窗口
	}
}

// RegisterDisk 注册磁盘
func (m *Manager) RegisterDisk(device, model, serial string, size int64) *Disk {
	m.mu.Lock()
	defer m.mu.Unlock()

	disk := &Disk{
		ID:         fmt.Sprintf("%s-%s", device, serial),
		Device:     device,
		Model:      model,
		Serial:     serial,
		Size:       size,
		State:      StateActive,
		LastAccess: time.Now(),
	}

	m.disks[disk.ID] = disk

	// 初始化访问模式
	m.patterns[disk.ID] = &AccessPattern{
		LastUpdated: time.Now(),
	}

	// 初始化默认策略
	m.policies[disk.ID] = &HibernatePolicy{
		Enabled:              true,
		IdleTimeout:          30 * time.Minute,
		StandbyTimeout:       1 * time.Hour,
		SleepTimeout:         4 * time.Hour,
		TemperatureThreshold: 55,
		SpinDownLimit:        10,
		SmartEnabled:         true,
	}

	return disk
}

// RecordAccess 记录访问
func (m *Manager) RecordAccess(diskID string, ioSize int64, opType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	disk, ok := m.disks[diskID]
	if !ok {
		return
	}

	now := time.Now()
	disk.LastAccess = now
	disk.AccessCount++
	disk.TotalIO += ioSize
	disk.State = StateActive

	// 记录访问
	record := AccessRecord{
		DiskID:    diskID,
		Timestamp: now,
		IOSize:    ioSize,
		OpType:    opType,
	}
	m.records = append(m.records, record)

	// 限制记录数量
	if len(m.records) > m.maxRecords {
		m.records = m.records[len(m.records)-m.maxRecords:]
	}

	// 更新访问模式
	m.updatePattern(diskID, now)
}

// updatePattern 更新访问模式
func (m *Manager) updatePattern(diskID string, t time.Time) {
	pattern, ok := m.patterns[diskID]
	if !ok {
		return
	}

	pattern.HourlyAccess[t.Hour()]++
	pattern.DailyAccess[int(t.Weekday())]++
	pattern.TotalRecords++
	pattern.LastUpdated = t

	// 计算高峰和安静时段
	m.calculatePeakQuietHours(pattern)

	// 计算平均访问间隔
	if len(m.records) > 1 {
		var totalGap time.Duration
		count := 0
		for i := len(m.records) - 1; i > 0 && i > len(m.records)-100; i-- {
			if m.records[i].DiskID == diskID && m.records[i-1].DiskID == diskID {
				totalGap += m.records[i].Timestamp.Sub(m.records[i-1].Timestamp)
				count++
			}
		}
		if count > 0 {
			pattern.AvgInterval = totalGap / time.Duration(count)
		}
	}
}

// calculatePeakQuietHours 计算高峰和安静时段
func (m *Manager) calculatePeakQuietHours(pattern *AccessPattern) {
	if pattern.TotalRecords < 100 {
		return
	}

	// 计算平均访问量
	var total int
	for _, count := range pattern.HourlyAccess {
		total += count
	}
	avg := total / 24

	pattern.PeakHours = nil
	pattern.QuietHours = nil

	for hour, count := range pattern.HourlyAccess {
		if count > avg*2 {
			pattern.PeakHours = append(pattern.PeakHours, hour)
		} else if count < avg/2 {
			pattern.QuietHours = append(pattern.QuietHours, hour)
		}
	}
}

// CheckHibernate 检查是否应该休眠
func (m *Manager) CheckHibernate(diskID string) (bool, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, ok := m.disks[diskID]
	if !ok {
		return false, "磁盘不存在"
	}

	policy, ok := m.policies[diskID]
	if !ok || !policy.Enabled {
		return false, "策略未启用"
	}

	// 检查温度
	if disk.Temperature > policy.TemperatureThreshold {
		return true, fmt.Sprintf("温度过高: %d°C > %d°C", disk.Temperature, policy.TemperatureThreshold)
	}

	// 检查空闲时间
	idleTime := time.Since(disk.LastAccess)

	// 智能模式：考虑访问模式
	if policy.SmartEnabled {
		pattern := m.patterns[diskID]
		if pattern != nil && pattern.TotalRecords > 100 {
			now := time.Now()
			hour := now.Hour()

			// 在安静时段，更积极地休眠
			for _, quietHour := range pattern.QuietHours {
				if hour == quietHour {
					if idleTime > policy.IdleTimeout/2 {
						return true, "智能模式：当前为安静时段"
					}
				}
			}

			// 在高峰时段，延迟休眠
			for _, peakHour := range pattern.PeakHours {
				if hour == peakHour {
					return false, "智能模式：当前为高峰时段"
				}
			}
		}
	}

	// 标准检查
	if idleTime > policy.SleepTimeout {
		return true, fmt.Sprintf("空闲时间超过休眠阈值: %v > %v", idleTime.Round(time.Minute), policy.SleepTimeout)
	}

	if idleTime > policy.StandbyTimeout {
		return true, fmt.Sprintf("空闲时间超过待机阈值: %v > %v", idleTime.Round(time.Minute), policy.StandbyTimeout)
	}

	return false, ""
}

// HibernateDisk 休眠磁盘
func (m *Manager) HibernateDisk(diskID string, state DiskState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	disk, ok := m.disks[diskID]
	if !ok {
		return fmt.Errorf("磁盘 %s 不存在", diskID)
	}

	// 检查每日休眠限制
	policy := m.policies[diskID]
	if policy != nil && disk.SpinUpCount > int64(policy.SpinDownLimit) {
		return fmt.Errorf("磁盘 %s 今日休眠次数已达上限 %d", diskID, policy.SpinDownLimit)
	}

	disk.State = state
	disk.SpinUpCount++

	return nil
}

// WakeDisk 唤醒磁盘
func (m *Manager) WakeDisk(diskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	disk, ok := m.disks[diskID]
	if !ok {
		return fmt.Errorf("磁盘 %s 不存在", diskID)
	}

	disk.State = StateActive
	disk.LastAccess = time.Now()

	return nil
}

// GetDisk 获取磁盘信息
func (m *Manager) GetDisk(diskID string) (*Disk, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, ok := m.disks[diskID]
	if !ok {
		return nil, fmt.Errorf("磁盘 %s 不存在", diskID)
	}

	return disk, nil
}

// GetPattern 获取访问模式
func (m *Manager) GetPattern(diskID string) (*AccessPattern, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pattern, ok := m.patterns[diskID]
	if !ok {
		return nil, fmt.Errorf("磁盘 %s 的访问模式不存在", diskID)
	}

	return pattern, nil
}

// UpdatePolicy 更新策略
func (m *Manager) UpdatePolicy(diskID string, policy *HibernatePolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.disks[diskID]; !ok {
		return fmt.Errorf("磁盘 %s 不存在", diskID)
	}

	m.policies[diskID] = policy
	return nil
}

// ListDisks 列出所有磁盘
func (m *Manager) ListDisks() []*Disk {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Disk
	for _, disk := range m.disks {
		result = append(result, disk)
	}

	return result
}

// GetHibernateReport 获取休眠报告
func (m *Manager) GetHibernateReport() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := make(map[string]interface{})
	var totalSaved int64
	var hibernatedCount int

	for id, disk := range m.disks {
		if disk.State == StateSleep || disk.State == StateStandby || disk.State == StateSpindown {
			hibernatedCount++
			// 估算节省的电量（假设每块硬盘约8W）
			totalSaved += 8
		}

		report[id] = map[string]interface{}{
			"state":        disk.State,
			"temperature":  disk.Temperature,
			"access_count": disk.AccessCount,
			"spin_up_count": disk.SpinUpCount,
		}
	}

	report["total_disks"] = len(m.disks)
	report["hibernated_count"] = hibernatedCount
	report["estimated_watts_saved"] = totalSaved

	return report
}
