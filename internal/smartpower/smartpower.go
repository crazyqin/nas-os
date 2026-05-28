package smartpower

import (
	"fmt"
	"sync"
	"time"
)

// ========== 构造函数 ==========

// NewPowerManager 创建电源管理器.
func NewPowerManager(config *PowerConfig) *PowerManager {
	if config == nil {
		config = DefaultPowerConfig()
	}

	return &PowerManager{
		config:       config,
		disks:        make(map[string]*DiskState),
		schedules:    make(map[string]*WakeSchedule),
		thermalZones: make(map[string]*ThermalZone),
		stats: &PowerStats{
			CurrentWatts: 0,
			DailyKWh:     0,
			MonthlyKWh:   0,
			CostEstimate: 0,
		},
	}
}

// ========== 磁盘管理方法 ==========

// GetDiskStates 获取所有磁盘状态.
func (p *PowerManager) GetDiskStates() []*DiskState {
	p.mu.RLock()
	defer p.mu.RUnlock()

	states := make([]*DiskState, 0, len(p.disks))
	for _, state := range p.disks {
		states = append(states, state)
	}
	return states
}

// SpindownDisk 让指定磁盘休眠.
func (p *PowerManager) SpindownDisk(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	disk, exists := p.disks[name]
	if !exists {
		return fmt.Errorf("%w: %s", ErrDiskNotFound, name)
	}

	// 设置为非旋转状态
	disk.IsSpinning = false
	disk.SpindownTimer = 0
	disk.LastAccess = time.Now()

	return nil
}

// WakeupDisk 唤醒指定磁盘.
func (p *PowerManager) WakeupDisk(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	disk, exists := p.disks[name]
	if !exists {
		return fmt.Errorf("%w: %s", ErrDiskNotFound, name)
	}

	// 设置为旋转状态
	disk.IsSpinning = true
	disk.SpindownTimer = p.config.DiskSpindownSec
	disk.LastAccess = time.Now()

	return nil
}

// ========== 电源方案方法 ==========

// ApplyProfile 应用电源方案.
func (p *PowerManager) ApplyProfile(profile *PowerProfile) error {
	if profile == nil {
		return ErrProfileNotFound
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// 保存当前方案
	p.profile = profile

	// 更新 CPU 调频策略
	if profile.CPUGovernor != "" {
		p.config.CPUGovernor = profile.CPUGovernor
	}

	// 更新风扇转速
	if profile.FanSpeed >= 0 && profile.FanSpeed <= 100 {
		p.config.FanSpeed = profile.FanSpeed
	}

	// 更新磁盘休眠配置
	if profile.DiskSpindownSec > 0 {
		p.config.DiskSpindownSec = profile.DiskSpindownSec
		// 更新所有磁盘的休眠定时器
		for _, disk := range p.disks {
			if disk.IsSpinning {
				disk.SpindownTimer = profile.DiskSpindownSec
			}
		}
	}

	// 合并唤醒调度
	if len(profile.WakeSchedule) > 0 {
		for _, schedule := range profile.WakeSchedule {
			if schedule != nil && schedule.ID != "" {
				p.schedules[schedule.ID] = schedule
			}
		}
	}

	return nil
}

// ========== 温度监控方法 ==========

// GetThermalStatus 获取所有温度区域状态.
func (p *PowerManager) GetThermalStatus() []*ThermalZone {
	p.mu.RLock()
	defer p.mu.RUnlock()

	zones := make([]*ThermalZone, 0, len(p.thermalZones))
	for _, zone := range p.thermalZones {
		zones = append(zones, zone)
	}
	return zones
}

// ========== 功耗统计方法 ==========

// GetPowerStats 获取功耗统计.
func (p *PowerManager) GetPowerStats() *PowerStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 返回统计副本
	return &PowerStats{
		CurrentWatts: p.stats.CurrentWatts,
		DailyKWh:     p.stats.DailyKWh,
		MonthlyKWh:   p.stats.MonthlyKWh,
		CostEstimate: p.stats.CostEstimate,
	}
}

// ========== 调度管理方法 ==========

// SetSchedule 设置唤醒/关机调度计划.
func (p *PowerManager) SetSchedule(schedule *WakeSchedule) error {
	if schedule == nil {
		return ErrScheduleNotFound
	}
	if schedule.ID == "" {
		return fmt.Errorf("%w: schedule ID is required", ErrInvalidConfig)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// 设置创建时间
	if schedule.CreatedAt.IsZero() {
		schedule.CreatedAt = time.Now()
	}

	p.schedules[schedule.ID] = schedule
	return nil
}

// ========== 辅助方法 ==========

// GetConfig 获取当前配置（只读副本）.
func (p *PowerManager) GetConfig() *PowerConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return &PowerConfig{
		DiskSpindownSec:      p.config.DiskSpindownSec,
		CPUGovernor:          p.config.CPUGovernor,
		TempCheckIntervalSec: p.config.TempCheckIntervalSec,
		FanSpeed:             p.config.FanSpeed,
		Enabled:              p.config.Enabled,
	}
}

// GetProfile 获取当前电源方案.
func (p *PowerManager) GetProfile() *PowerProfile {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.profile
}

// GetSchedule 获取指定调度计划.
func (p *PowerManager) GetSchedule(id string) (*WakeSchedule, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	schedule, exists := p.schedules[id]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrScheduleNotFound, id)
	}
	return schedule, nil
}

// ListSchedules 列出所有调度计划.
func (p *PowerManager) ListSchedules() []*WakeSchedule {
	p.mu.RLock()
	defer p.mu.RUnlock()

	schedules := make([]*WakeSchedule, 0, len(p.schedules))
	for _, s := range p.schedules {
		schedules = append(schedules, s)
	}
	return schedules
}

// RemoveSchedule 删除调度计划.
func (p *PowerManager) RemoveSchedule(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.schedules[id]; !exists {
		return fmt.Errorf("%w: %s", ErrScheduleNotFound, id)
	}

	delete(p.schedules, id)
	return nil
}

// AddDisk 添加磁盘到管理.
func (p *PowerManager) AddDisk(name string, temperature float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.disks[name] = &DiskState{
		Name:          name,
		IsSpinning:    true,
		LastAccess:    time.Now(),
		Temperature:   temperature,
		SpindownTimer: p.config.DiskSpindownSec,
	}
}

// RemoveDisk 移除磁盘.
func (p *PowerManager) RemoveDisk(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.disks, name)
}

// AddThermalZone 添加温度区域.
func (p *PowerManager) AddThermalZone(name string, temperature, threshold, critical float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.thermalZones[name] = &ThermalZone{
		Name:        name,
		Temperature: temperature,
		Threshold:   threshold,
		Critical:    critical,
	}
}

// UpdateThermalZone 更新温度区域温度.
func (p *PowerManager) UpdateThermalZone(name string, temperature float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	zone, exists := p.thermalZones[name]
	if !exists {
		return fmt.Errorf("%w: %s", ErrThermalZoneNotFound, name)
	}

	zone.Temperature = temperature
	return nil
}

// UpdatePowerStats 更新功耗统计.
func (p *PowerManager) UpdatePowerStats(currentWatts, dailyKWh, monthlyKWh, costEstimate float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats.CurrentWatts = currentWatts
	p.stats.DailyKWh = dailyKWh
	p.stats.MonthlyKWh = monthlyKWh
	p.stats.CostEstimate = costEstimate
}

// CheckThermalAlerts 检查温度告警，返回超阈值的区域.
func (p *PowerManager) CheckThermalAlerts() []*ThermalZone {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var alerts []*ThermalZone
	for _, zone := range p.thermalZones {
		if zone.Temperature >= zone.Threshold {
			alerts = append(alerts, zone)
		}
	}
	return alerts
}

// CheckCriticalThermal 检查临界温度，返回超临界值的区域.
func (p *PowerManager) CheckCriticalThermal() []*ThermalZone {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var critical []*ThermalZone
	for _, zone := range p.thermalZones {
		if zone.Temperature >= zone.Critical {
			critical = append(critical, zone)
		}
	}
	return critical
}

// Ensure interface compliance at compile time.
var _ = (*PowerManager)(nil)

// Suppress unused import warnings.
var _ = sync.RWMutex{}
