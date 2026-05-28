// Package diskwearlevel - 磁盘磨损均衡管理器
package diskwearlevel

import (
	"fmt"
	"sync"
	"time"
)

// Manager 磁盘磨损均衡管理器
type Manager struct {
	mu       sync.RWMutex
	disks    map[string]*DiskInfo
	policies map[string]*WearPolicy
	plans    []*RebalancePlan
	smart    map[string]*SMARTData
	stats    *WearStats
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		disks:    make(map[string]*DiskInfo),
		policies: make(map[string]*WearPolicy),
		plans:    make([]*RebalancePlan, 0),
		smart:    make(map[string]*SMARTData),
		stats:    &WearStats{HealthBreakdown: make(map[string]int)},
	}
}

// RegisterDisk 注册磁盘
func (m *Manager) RegisterDisk(disk *DiskInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if disk.ID == "" {
		disk.ID = fmt.Sprintf("disk-%s", disk.Serial)
	}
	disk.CreatedAt = time.Now()
	m.disks[disk.ID] = disk
	m.updateStatsLocked()
	return nil
}

// GetDisk 获取磁盘信息
func (m *Manager) GetDisk(id string) (*DiskInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, ok := m.disks[id]
	if !ok {
		return nil, fmt.Errorf("disk %s not found", id)
	}
	return disk, nil
}

// ListDisks 列出所有磁盘
func (m *Manager) ListDisks() []*DiskInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disks := make([]*DiskInfo, 0, len(m.disks))
	for _, d := range m.disks {
		disks = append(disks, d)
	}
	return disks
}

// UpdateSMARTData 更新 SMART 数据
func (m *Manager) UpdateSMARTData(data *SMARTData) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data.CheckedAt = time.Now()
	m.smart[data.DiskID] = data

	// 更新磁盘健康状态
	if disk, ok := m.disks[data.DiskID]; ok {
		disk.Temperature = data.Temperature
		disk.LastSMART = data.CheckedAt
		disk.WearPercent = float64(data.PercentageUsed)
		disk.WearLevel = m.calcWearLevel(disk.WearPercent)
		disk.Health = m.calcHealth(disk, data)
	}
	return nil
}

// CreatePolicy 创建策略
func (m *Manager) CreatePolicy(policy *WearPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		policy.ID = fmt.Sprintf("wpolicy-%d", time.Now().UnixNano())
	}
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	m.policies[policy.ID] = policy
	return nil
}

// GenerateRebalancePlan 生成均衡计划
func (m *Manager) GenerateRebalancePlan() *RebalancePlan {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 找到磨损最严重的磁盘
	var maxWear *DiskInfo
	var minWear *DiskInfo
	for _, d := range m.disks {
		if d.Type == DiskTypeSSD || d.Type == DiskTypeNVMe {
			if maxWear == nil || d.WearPercent > maxWear.WearPercent {
				maxWear = d
			}
			if minWear == nil || d.WearPercent < minWear.WearPercent {
				minWear = d
			}
		}
	}

	if maxWear == nil || minWear == nil || maxWear.ID == minWear.ID {
		return nil
	}

	plan := &RebalancePlan{
		ID:         fmt.Sprintf("rebalance-%d", time.Now().UnixNano()),
		CreatedAt:  time.Now(),
		SourceDisk: maxWear.Device,
		TargetDisk: minWear.Device,
		EstBytes:   100 * 1024 * 1024 * 1024, // 100GB
		Reason:     fmt.Sprintf("磨损差异: %s(%.1f%%) → %s(%.1f%%)", maxWear.Device, maxWear.WearPercent, minWear.Device, minWear.WearPercent),
		Actions: []*RebalanceAction{
			{Type: "move_data", SourcePath: "/data/hot", TargetPath: "/data/cold", Priority: 1},
			{Type: "swap_role", Priority: 2},
		},
		Status: "pending",
	}

	m.plans = append(m.plans, plan)
	return plan
}

// GetStats 获取统计
func (m *Manager) GetStats() *WearStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

func (m *Manager) calcWearLevel(wear float64) WearLevel {
	switch {
	case wear < 30:
		return WearLevelLow
	case wear < 60:
		return WearLevelMedium
	case wear < 80:
		return WearLevelHigh
	default:
		return WearLevelSevere
	}
}

func (m *Manager) calcHealth(disk *DiskInfo, smart *SMARTData) DiskHealth {
	switch {
	case disk.WearPercent >= 90 || smart.CriticalWarning > 0:
		return HealthCritical
	case disk.WearPercent >= 70:
		return HealthPoor
	case disk.WearPercent >= 50:
		return HealthFair
	case disk.WearPercent >= 20:
		return HealthGood
	default:
		return HealthExcellent
	}
}

func (m *Manager) updateStatsLocked() {
	m.stats.TotalDisks = len(m.disks)
	m.stats.HealthBreakdown = make(map[string]int)
	totalWear := 0.0
	maxWear := 0.0
	minWear := 100.0

	for _, d := range m.disks {
		m.stats.HealthBreakdown[string(d.Health)]++
		totalWear += d.WearPercent
		if d.WearPercent > maxWear {
			maxWear = d.WearPercent
		}
		if d.WearPercent < minWear {
			minWear = d.WearPercent
		}
		switch d.Health {
		case HealthExcellent, HealthGood:
			m.stats.HealthyDisks++
		case HealthFair:
			m.stats.WarningDisks++
		case HealthPoor, HealthCritical, HealthFailed:
			m.stats.CriticalDisks++
		}
	}

	if m.stats.TotalDisks > 0 {
		m.stats.AvgWearPercent = totalWear / float64(m.stats.TotalDisks)
		m.stats.MaxWearPercent = maxWear
		m.stats.MinWearPercent = minWear
	}
}
