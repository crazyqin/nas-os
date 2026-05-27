// Package assetmgr 提供IT资产管理功能
package assetmgr

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// Manager 资产管理器.
type Manager struct {
	mu        sync.RWMutex
	assets    map[string]*Asset
	groups    map[string]*AssetGroup
	schedules map[string]*MaintenanceSchedule
	scanner   *Scanner
}

// NewManager 创建资产管理器.
func NewManager() *Manager {
	return &Manager{
		assets:    make(map[string]*Asset),
		groups:    make(map[string]*AssetGroup),
		schedules: make(map[string]*MaintenanceSchedule),
		scanner:   NewScanner(),
	}
}

// ========== 资产管理 ==========

// AddAsset 添加资产.
func (m *Manager) AddAsset(asset *Asset) error {
	if asset.ID == "" {
		return ErrInvalidInput
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	asset.CreatedAt = time.Now()
	asset.UpdatedAt = time.Now()
	if asset.Status == "" {
		asset.Status = StatusOnline
	}
	m.assets[asset.ID] = asset
	return nil
}

// GetAsset 获取资产.
func (m *Manager) GetAsset(id string) (*Asset, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.assets[id]
	if !ok {
		return nil, ErrAssetNotFound
	}
	return a, nil
}

// UpdateAsset 更新资产.
func (m *Manager) UpdateAsset(asset *Asset) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.assets[asset.ID]; !ok {
		return ErrAssetNotFound
	}
	asset.UpdatedAt = time.Now()
	m.assets[asset.ID] = asset
	return nil
}

// DeleteAsset 删除资产.
func (m *Manager) DeleteAsset(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.assets[id]; !ok {
		return ErrAssetNotFound
	}
	delete(m.assets, id)
	return nil
}

// ListAssets 列出资产.
func (m *Manager) ListAssets(assetType AssetType, status AssetStatus) []*Asset {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Asset, 0)
	for _, a := range m.assets {
		if assetType != "" && a.Type != assetType {
			continue
		}
		if status != "" && a.Status != status {
			continue
		}
		result = append(result, a)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// SearchAssets 搜索资产（按名称、IP、序列号）.
func (m *Manager) SearchAssets(query string) []*Asset {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Asset, 0)
	for _, a := range m.assets {
		if containsIgnoreCase(a.Name, query) ||
			containsIgnoreCase(a.IPAddress, query) ||
			containsIgnoreCase(a.SerialNumber, query) ||
			containsIgnoreCase(a.MACAddress, query) {
			result = append(result, a)
		}
	}
	return result
}

func containsIgnoreCase(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	return len(s) >= len(sub) && containsFold(s, sub)
}

func containsFold(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			sc, tc := s[i+j], sub[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if tc >= 'A' && tc <= 'Z' {
				tc += 32
			}
			if sc != tc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// ========== 网络发现 ==========

// ScanNetwork 触发网络扫描.
func (m *Manager) ScanNetwork(scanRange string) (*ScanResult, error) {
	return m.scanner.Scan(scanRange)
}

// ========== 资产分组 ==========

// CreateGroup 创建资产分组.
func (m *Manager) CreateGroup(group *AssetGroup) error {
	if group.ID == "" {
		return ErrInvalidInput
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	group.CreatedAt = time.Now()
	m.groups[group.ID] = group
	return nil
}

// GetGroup 获取资产分组.
func (m *Manager) GetGroup(id string) (*AssetGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	g, ok := m.groups[id]
	if !ok {
		return nil, ErrGroupNotFound
	}
	return g, nil
}

// ListGroups 列出所有分组.
func (m *Manager) ListGroups() []*AssetGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()
	groups := make([]*AssetGroup, 0, len(m.groups))
	for _, g := range m.groups {
		groups = append(groups, g)
	}
	return groups
}

// AddAssetToGroup 将资产添加到分组.
func (m *Manager) AddAssetToGroup(groupID, assetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.groups[groupID]
	if !ok {
		return ErrGroupNotFound
	}
	if _, ok := m.assets[assetID]; !ok {
		return ErrAssetNotFound
	}
	// 检查是否已存在
	for _, id := range g.AssetIDs {
		if id == assetID {
			return nil
		}
	}
	g.AssetIDs = append(g.AssetIDs, assetID)
	return nil
}

// RemoveAssetFromGroup 从分组移除资产.
func (m *Manager) RemoveAssetFromGroup(groupID, assetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.groups[groupID]
	if !ok {
		return ErrGroupNotFound
	}
	for i, id := range g.AssetIDs {
		if id == assetID {
			g.AssetIDs = append(g.AssetIDs[:i], g.AssetIDs[i+1:]...)
			return nil
		}
	}
	return nil
}

// DeleteGroup 删除分组.
func (m *Manager) DeleteGroup(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.groups[id]; !ok {
		return ErrGroupNotFound
	}
	delete(m.groups, id)
	return nil
}

// ========== 维护计划 ==========

// CreateMaintenanceSchedule 创建维护计划.
func (m *Manager) CreateMaintenanceSchedule(schedule *MaintenanceSchedule) error {
	if schedule.ID == "" {
		return ErrInvalidInput
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	schedule.CreatedAt = time.Now()
	if !schedule.LastMaintenance.IsZero() && schedule.IntervalDays > 0 {
		schedule.NextMaintenance = schedule.LastMaintenance.AddDate(0, 0, schedule.IntervalDays)
	}
	m.schedules[schedule.ID] = schedule
	return nil
}

// GetMaintenanceSchedule 获取维护计划.
func (m *Manager) GetMaintenanceSchedule(id string) (*MaintenanceSchedule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.schedules[id]
	if !ok {
		return nil, ErrScheduleNotFound
	}
	return s, nil
}

// ListMaintenanceSchedules 列出维护计划.
func (m *Manager) ListMaintenanceSchedules() []*MaintenanceSchedule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	schedules := make([]*MaintenanceSchedule, 0, len(m.schedules))
	for _, s := range m.schedules {
		schedules = append(schedules, s)
	}
	return schedules
}

// RecordMaintenance 记录维护完成.
func (m *Manager) RecordMaintenance(scheduleID string, maintenanceTime time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.schedules[scheduleID]
	if !ok {
		return ErrScheduleNotFound
	}
	s.LastMaintenance = maintenanceTime
	if s.IntervalDays > 0 {
		s.NextMaintenance = maintenanceTime.AddDate(0, 0, s.IntervalDays)
	}
	return nil
}

// GetUpcomingMaintenance 获取即将到期的维护计划.
func (m *Manager) GetUpcomingMaintenance(withinDays int) []*MaintenanceSchedule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	deadline := time.Now().AddDate(0, 0, withinDays)
	result := make([]*MaintenanceSchedule, 0)
	for _, s := range m.schedules {
		if !s.NextMaintenance.IsZero() && s.NextMaintenance.Before(deadline) {
			result = append(result, s)
		}
	}
	return result
}

// ========== 生命周期管理 ==========

// UpdateLifecycleStage 更新资产生命周期阶段.
func (m *Manager) UpdateLifecycleStage(assetID string, stage LifecycleStage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.assets[assetID]
	if !ok {
		return ErrAssetNotFound
	}
	// 根据生命周期阶段更新资产状态
	switch stage {
	case StageDecommissioned:
		a.Status = StatusDecommissioned
	case StageMaintenance:
		a.Status = StatusMaintenance
	}
	a.UpdatedAt = time.Now()
	return nil
}

// GetAgingAssets 获取老化资产（超过指定年限）.
func (m *Manager) GetAgingAssets(years int) []*Asset {
	m.mu.RLock()
	defer m.mu.RUnlock()
	deadline := time.Now().AddDate(-years, 0, 0)
	result := make([]*Asset, 0)
	for _, a := range m.assets {
		if a.Status == StatusDecommissioned {
			continue
		}
		if !a.PurchaseDate.IsZero() && a.PurchaseDate.Before(deadline) {
			result = append(result, a)
		}
	}
	return result
}

// GetExpiredWarranty 获取保修到期资产.
func (m *Manager) GetExpiredWarranty() []*Asset {
	m.mu.RLock()
	defer m.mu.RUnlock()
	now := time.Now()
	result := make([]*Asset, 0)
	for _, a := range m.assets {
		if a.Status == StatusDecommissioned {
			continue
		}
		if !a.WarrantyEnd.IsZero() && a.WarrantyEnd.Before(now) {
			result = append(result, a)
		}
	}
	return result
}

// ========== 统计 ==========

// GetAssetSummary 资产统计摘要.
func (m *Manager) GetAssetSummary() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	byType := make(map[AssetType]int)
	byStatus := make(map[AssetStatus]int)
	for _, a := range m.assets {
		byType[a.Type]++
		byStatus[a.Status]++
	}

	// 内联计算 aging 和 warranty_expired，避免递归 RLock
	now := time.Now()
	agingDeadline := now.AddDate(-5, 0, 0)
	agingCount := 0
	warrantyExpiredCount := 0
	for _, a := range m.assets {
		if a.Status == StatusDecommissioned {
			continue
		}
		if !a.PurchaseDate.IsZero() && a.PurchaseDate.Before(agingDeadline) {
			agingCount++
		}
		if !a.WarrantyEnd.IsZero() && a.WarrantyEnd.Before(now) {
			warrantyExpiredCount++
		}
	}

	return map[string]interface{}{
		"total":             len(m.assets),
		"by_type":           byType,
		"by_status":         byStatus,
		"groups":            len(m.groups),
		"schedules":         len(m.schedules),
		"aging":             agingCount,
		"warranty_expired":  warrantyExpiredCount,
	}
}

// ListHardwareInventory 列出硬件清单.
func (m *Manager) ListHardwareInventory() []*Asset {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Asset, 0)
	for _, a := range m.assets {
		if a.Hardware != nil {
			result = append(result, a)
		}
	}
	return result
}

// ListSoftwareInventory 列出软件清单.
func (m *Manager) ListSoftwareInventory() []*Asset {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Asset, 0)
	for _, a := range m.assets {
		if a.Software != nil {
			result = append(result, a)
		}
	}
	return result
}

// GetAssetsByLocation 按位置获取资产.
func (m *Manager) GetAssetsByLocation(location string) []*Asset {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Asset, 0)
	for _, a := range m.assets {
		if a.Location == location {
			result = append(result, a)
		}
	}
	return result
}

// ExportInventory 导出资产清单摘要.
func (m *Manager) ExportInventory() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	summary := m.GetAssetSummary()
	return fmt.Sprintf("资产总数: %v | 在线: %v | 离线: %v | 维护中: %v | 已退役: %v",
		summary["total"],
		summary["by_status"].(map[AssetStatus]int)[StatusOnline],
		summary["by_status"].(map[AssetStatus]int)[StatusOffline],
		summary["by_status"].(map[AssetStatus]int)[StatusMaintenance],
		summary["by_status"].(map[AssetStatus]int)[StatusDecommissioned],
	)
}
