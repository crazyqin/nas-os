// Package disk provides disk power management functionality.
// Implements intelligent disk sleep/wake patterns inspired by飞牛fnOS.
// Version: v2.387.0 - 按需唤醒 + 智能调度 + 能耗统计API增强
package disk

import (
	"context"
	"sync"
	"time"
)

// PowerState represents disk power state.
type PowerState string

const (
	PowerStateActive  PowerState = "active"  // 磁盘活跃
	PowerStateIdle    PowerState = "idle"    // 磁盘空闲
	PowerStateStandby PowerState = "standby" // 磁盘待机
	PowerStateSleep   PowerState = "sleep"   // 磁盘休眠
)

// BusinessPeriod 业务时段定义 - 用于智能调度避开业务高峰
type BusinessPeriod struct {
	StartHour int `json:"start_hour"` // 开始时间(小时, 0-23)
	EndHour   int `json:"end_hour"`   // 结束时间(小时, 0-23)
	Priority  int `json:"priority"`   // 优先级 (1-10, 高优先级时段不执行节能操作)
}

// SleepPolicy defines disk sleep policy.
type SleepPolicy struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	IdleThreshold    time.Duration `json:"idle_threshold"`    // 空闲阈值(秒)
	StandbyThreshold time.Duration `json:"standby_threshold"` // 待机阈值(秒)
	SleepThreshold   time.Duration `json:"sleep_threshold"`   // 休眠阈值(秒)
	Enabled          bool          `json:"enabled"`
	ExcludedDisks    []string      `json:"excluded_disks"` // 排除的磁盘

	// v2.387.0: 智能调度配置
	BusinessPeriods  []BusinessPeriod `json:"business_periods,omitempty"` // 业务高峰时段
	AllowSleepInPeak bool             `json:"allow_sleep_in_peak"`        // 是否允许在高峰时段休眠
	MaxWakePerHour   int              `json:"max_wake_per_hour"`         // 每小时最大唤醒次数限制
}

// WakeRequest 按需唤醒请求
type WakeRequest struct {
	DiskID     string    `json:"disk_id"`
	Reason     string    `json:"reason"`     // 唤醒原因
	Priority   int       `json:"priority"`   // 请求优先级 (1-10)
	Timestamp  time.Time `json:"timestamp"`
	RequestedBy string   `json:"requested_by"` // 请求来源 (api/user/system)
}

// DiskPowerStatus represents disk power status.
type DiskPowerStatus struct {
	DiskID       string        `json:"disk_id"`
	State        PowerState    `json:"state"`
	LastActivity time.Time     `json:"last_activity"`
	IdleDuration time.Duration `json:"idle_duration"`
	Policy       *SleepPolicy  `json:"policy"`
	WakeCount    int           `json:"wake_count"`    // 唤醒次数
	SleepCount   int           `json:"sleep_count"`   // 休眠次数
	EnergySaved  float64       `json:"energy_saved"`  // kWh节省

	// v2.387.0: 智能电源管理增强
	WakeRequests     []WakeRequest   `json:"wake_requests,omitempty"` // 待处理的唤醒请求
	LastWakeReason   string          `json:"last_wake_reason"`        // 最近唤醒原因
	WakeCountHour    int             `json:"wake_count_hour"`         // 当前小时唤醒次数
	LastWakeHour     int             `json:"last_wake_hour"`          // 最近唤醒的小时
	WakeCostSaved    float64         `json:"wake_cost_saved"`         // 避免唤醒节省的能耗(kWh)
	PredictedNextWake time.Time      `json:"predicted_next_wake,omitempty"` // 预测下次唤醒时间
}

// PowerManager manages disk power states.
type PowerManager struct {
	mu          sync.RWMutex
	statuses    map[string]*DiskPowerStatus
	policies    map[string]*SleepPolicy
	activityMon *ActivityMonitor
	config      *PowerConfig

	// v2.387.0: 按需唤醒 + 能耗统计
	wakeQueue      map[string][]WakeRequest // 按需唤醒请求队列
	energyStats    *EnergyStatistics         // 能耗统计
	wakeQueueMu    sync.Mutex
	pendingWakes   map[string]context.CancelFunc // 待取消的唤醒任务
	businessHours  []BusinessPeriod             // 业务高峰时段配置
}

// PowerConfig holds power management configuration.
type PowerConfig struct {
	CheckInterval    time.Duration `json:"check_interval"`
	DefaultPolicy    string        `json:"default_policy"`
	EnableMonitoring bool          `json:"enable_monitoring"`

	// v2.387.0: 新增配置
	EnableWakeOnDemand      bool          `json:"enable_wake_on_demand"`       // 启用按需唤醒
	EnableSmartScheduling   bool          `json:"enable_smart_scheduling"`     // 启用智能调度
	EnergyTrackingInterval  time.Duration `json:"energy_tracking_interval"`    // 能耗统计间隔
	DefaultDiskPowerWatts   float64       `json:"default_disk_power_watts"`    // 默认磁盘功耗(W)
	WakePowerSpikeWatts     float64       `json:"wake_power_spike_watts"`      // 唤醒瞬时功耗(W)
	WakeDurationSeconds     float64       `json:"wake_duration_seconds"`       // 唤醒持续时间(s)
}

// EnergyStatistics 能耗统计数据
type EnergyStatistics struct {
	mu             sync.RWMutex
	StartTime      time.Time         `json:"start_time"`
	TotalEnergyUsed    float64       `json:"total_energy_used"`     // kWh 总能耗
	TotalEnergySaved   float64       `json:"total_energy_saved"`    // kWh 总节省
	WakeEnergyCost     float64       `json:"wake_energy_cost"`      // kWh 唤醒能耗开销
	SavedWakeCount     int           `json:"saved_wake_count"`      // 节省的唤醒次数
	DiskStats          map[string]*DiskEnergyStat `json:"disk_stats"`
	HourlyStats        []HourlyEnergyStat         `json:"hourly_stats"`
}

// DiskEnergyStat 单磁盘能耗统计
type DiskEnergyStat struct {
	DiskID          string    `json:"disk_id"`
	ActiveHours     float64   `json:"active_hours"`     // 活跃时长(小时)
	SleepHours      float64   `json:"sleep_hours"`      // 休眠时长(小时)
	WakeCount       int       `json:"wake_count"`       // 唤醒次数
	EnergyConsumed  float64   `json:"energy_consumed"`  // kWh 已消耗
	EnergySaved     float64   `json:"energy_saved"`     // kWh 已节省
	LastStateChange time.Time `json:"last_state_change"`
}

// HourlyEnergyStat 小时级能耗统计
type HourlyEnergyStat struct {
	Hour            int       `json:"hour"`             // 0-23
	Date            time.Time `json:"date"`             // 日期
	ActiveDisks     int       `json:"active_disks"`     // 活跃磁盘数
	SleepingDisks   int       `json:"sleeping_disks"`   // 休眠磁盘数
	EnergyUsed      float64   `json:"energy_used"`      // kWh
	EnergySaved     float64   `json:"energy_saved"`     // kWh
	WakeEvents      int       `json:"wake_events"`      // 唤醒事件数
	SleepEvents     int       `json:"sleep_events"`     // 休眠事件数
}

// NewPowerManager creates a new power manager.
func NewPowerManager(cfg *PowerConfig) *PowerManager {
	if cfg == nil {
		cfg = &PowerConfig{
			CheckInterval:          30 * time.Second,
			DefaultPolicy:          "default",
			EnableMonitoring:       true,
			EnableWakeOnDemand:     true,
			EnableSmartScheduling:  true,
			EnergyTrackingInterval: 1 * time.Hour,
			DefaultDiskPowerWatts:  10.0,
			WakePowerSpikeWatts:    15.0,
			WakeDurationSeconds:    10.0,
		}
	}
	return &PowerManager{
		statuses:       make(map[string]*DiskPowerStatus),
		policies:       make(map[string]*SleepPolicy),
		activityMon:    NewActivityMonitor(),
		config:         cfg,
		wakeQueue:      make(map[string][]WakeRequest),
		pendingWakes:   make(map[string]context.CancelFunc),
		businessHours:  DefaultBusinessPeriods(),
		energyStats:    NewEnergyStatistics(),
	}
}

// NewEnergyStatistics 创建能耗统计实例.
func NewEnergyStatistics() *EnergyStatistics {
	return &EnergyStatistics{
		StartTime:   time.Now(),
		DiskStats:   make(map[string]*DiskEnergyStat),
		HourlyStats: []HourlyEnergyStat{},
	}
}

// DefaultBusinessPeriods 默认业务高峰时段配置.
func DefaultBusinessPeriods() []BusinessPeriod {
	return []BusinessPeriod{
		{StartHour: 9, EndHour: 12, Priority: 8},  // 上午工作时段
		{StartHour: 14, EndHour: 18, Priority: 9}, // 下午工作时段
		{StartHour: 20, EndHour: 22, Priority: 5}, // 晚间使用时段
	}
}

// Start starts the power manager monitoring loop.
func (pm *PowerManager) Start(ctx context.Context) error {
	if !pm.config.EnableMonitoring {
		return nil
	}

	go pm.monitorLoop(ctx)
	return nil
}

// monitorLoop monitors disk activity and applies sleep policies.
func (pm *PowerManager) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(pm.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pm.checkDiskStates()
		}
	}
}

// checkDiskStates checks all disk states and applies policies.
func (pm *PowerManager) checkDiskStates() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()
	currentHour := now.Hour()
	isPeakHour := pm.isBusinessPeakHour(currentHour)

	for diskID, status := range pm.statuses {
		if status.Policy == nil || !status.Policy.Enabled {
			continue
		}

		// Check if disk is excluded
		for _, excluded := range status.Policy.ExcludedDisks {
			if excluded == diskID {
				continue
			}
		}

		// v2.387.0: 检查是否有待处理的唤醒请求
		pm.wakeQueueMu.Lock()
		if requests, exists := pm.wakeQueue[diskID]; exists && len(requests) > 0 {
			// 有唤醒请求，立即唤醒
			pm.transitionDisk(diskID, PowerStateActive)
			status.LastWakeReason = requests[0].Reason
			pm.wakeQueue[diskID] = requests[1:] // 移除已处理的请求
		}
		pm.wakeQueueMu.Unlock()

		idleDuration := now.Sub(status.LastActivity)

		// v2.387.0: 智能调度 - 高峰时段延长休眠阈值或禁止休眠
		sleepThreshold := status.Policy.SleepThreshold
		standbyThreshold := status.Policy.StandbyThreshold

		if isPeakHour && !status.Policy.AllowSleepInPeak {
			// 高峰时段不执行节能操作
			status.IdleDuration = idleDuration
			continue
		}

		// 高峰时段延长阈值
		if isPeakHour {
			sleepThreshold *= 2
			standbyThreshold *= 2
		}

		// v2.387.0: 检查唤醒频率限制
		if pm.config.EnableSmartScheduling {
			// 如果当前小时内唤醒次数已达到限制，延迟休眠
			if status.LastWakeHour == currentHour && status.WakeCountHour >= status.Policy.MaxWakePerHour {
				status.IdleDuration = idleDuration
				continue
			}
		}

		// Apply sleep thresholds
		if idleDuration >= sleepThreshold && status.State != PowerStateSleep {
			pm.transitionDisk(diskID, PowerStateSleep)
		} else if idleDuration >= standbyThreshold && status.State != PowerStateStandby {
			pm.transitionDisk(diskID, PowerStateStandby)
		} else if idleDuration >= status.Policy.IdleThreshold && status.State != PowerStateIdle {
			pm.transitionDisk(diskID, PowerStateIdle)
		}

		status.IdleDuration = idleDuration
	}

	// v2.387.0: 更新能耗统计
	pm.updateEnergyStatistics()
}

// isBusinessPeakHour 检查是否为业务高峰时段.
func (pm *PowerManager) isBusinessPeakHour(hour int) bool {
	for _, period := range pm.businessHours {
		if hour >= period.StartHour && hour < period.EndHour && period.Priority >= 7 {
			return true
		}
	}
	return false
}

// updateEnergyStatistics 更新能耗统计.
func (pm *PowerManager) updateEnergyStatistics() {
	now := time.Now()
	currentHour := now.Hour()

	pm.energyStats.mu.Lock()
	defer pm.energyStats.mu.Unlock()

	var activeCount, sleepingCount int
	var hourEnergyUsed, hourEnergySaved float64
	var wakeEvents, sleepEvents int

	for diskID, status := range pm.statuses {
		stat := pm.energyStats.DiskStats[diskID]
		if stat == nil {
			stat = &DiskEnergyStat{
				DiskID:          diskID,
				LastStateChange: now,
			}
			pm.energyStats.DiskStats[diskID] = stat
		}

		// 计算能耗
		if status.State == PowerStateActive || status.State == PowerStateIdle {
			stat.ActiveHours += pm.config.CheckInterval.Hours()
			stat.EnergyConsumed += pm.config.DefaultDiskPowerWatts * pm.config.CheckInterval.Hours() / 1000.0
			hourEnergyUsed += pm.config.DefaultDiskPowerWatts * pm.config.CheckInterval.Hours() / 1000.0
			activeCount++
		} else {
			stat.SleepHours += pm.config.CheckInterval.Hours()
			// 休眠时功耗约2W
			stat.EnergyConsumed += 2.0 * pm.config.CheckInterval.Hours() / 1000.0
			stat.EnergySaved += (pm.config.DefaultDiskPowerWatts - 2.0) * pm.config.CheckInterval.Hours() / 1000.0
			hourEnergySaved += (pm.config.DefaultDiskPowerWatts - 2.0) * pm.config.CheckInterval.Hours() / 1000.0
			sleepingCount++
		}

		stat.WakeCount = status.WakeCount
		stat.EnergySaved = status.EnergySaved
	}

	// 更新小时统计
	if len(pm.energyStats.HourlyStats) == 0 ||
		pm.energyStats.HourlyStats[len(pm.energyStats.HourlyStats)-1].Hour != currentHour {
		// 新的小时，添加统计记录
		hourStat := HourlyEnergyStat{
			Hour:          currentHour,
			Date:          now,
			ActiveDisks:   activeCount,
			SleepingDisks: sleepingCount,
			EnergyUsed:    hourEnergyUsed,
			EnergySaved:   hourEnergySaved,
			WakeEvents:    wakeEvents,
			SleepEvents:   sleepEvents,
		}
		pm.energyStats.HourlyStats = append(pm.energyStats.HourlyStats, hourStat)

		// 保留最近7天的统计（168小时）
		if len(pm.energyStats.HourlyStats) > 168 {
			pm.energyStats.HourlyStats = pm.energyStats.HourlyStats[len(pm.energyStats.HourlyStats)-168:]
		}
	}

	pm.energyStats.TotalEnergyUsed += hourEnergyUsed
	pm.energyStats.TotalEnergySaved += hourEnergySaved
}

// transitionDisk transitions disk to new power state.
func (pm *PowerManager) transitionDisk(diskID string, newState PowerState) {
	status, ok := pm.statuses[diskID]
	if !ok {
		return
	}

	oldState := status.State
	status.State = newState

	// Update counters
	if newState == PowerStateSleep && oldState != PowerStateSleep {
		status.SleepCount++
		// Calculate energy saved (rough estimate: 10W per sleeping disk)
		status.EnergySaved += float64(pm.config.CheckInterval.Hours()) * 10.0 / 1000.0
	}
	if newState == PowerStateActive && oldState == PowerStateSleep {
		status.WakeCount++
	}
}

// RegisterDisk registers a disk for power management.
func (pm *PowerManager) RegisterDisk(diskID string, policyID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	policy := pm.policies[policyID]
	if policy == nil && policyID != "" {
		policy = pm.policies[pm.config.DefaultPolicy]
	}

	pm.statuses[diskID] = &DiskPowerStatus{
		DiskID:       diskID,
		State:        PowerStateActive,
		LastActivity: time.Now(),
		Policy:       policy,
	}

	return nil
}

// RecordActivity records disk activity (wakes disk if sleeping).
func (pm *PowerManager) RecordActivity(diskID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	status, ok := pm.statuses[diskID]
	if !ok {
		return nil
	}

	// Wake disk if sleeping
	if status.State == PowerStateSleep || status.State == PowerStateStandby {
		pm.transitionDisk(diskID, PowerStateActive)
	}

	status.LastActivity = time.Now()
	status.IdleDuration = 0

	return nil
}

// GetDiskStatus returns disk power status.
func (pm *PowerManager) GetDiskStatus(diskID string) (*DiskPowerStatus, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	status, ok := pm.statuses[diskID]
	if !ok {
		return nil, nil
	}
	return status, nil
}

// GetAllStatuses returns all disk power statuses.
func (pm *PowerManager) GetAllStatuses() map[string]*DiskPowerStatus {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make(map[string]*DiskPowerStatus)
	for k, v := range pm.statuses {
		result[k] = v
	}
	return result
}

// AddPolicy adds a sleep policy.
func (pm *PowerManager) AddPolicy(policy *SleepPolicy) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.policies[policy.ID] = policy
	return nil
}

// GetPolicy returns a sleep policy.
func (pm *PowerManager) GetPolicy(policyID string) (*SleepPolicy, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	policy, ok := pm.policies[policyID]
	if !ok {
		return nil, nil
	}
	return policy, nil
}

// GetEnergyReport returns energy saving report.
func (pm *PowerManager) GetEnergyReport() *EnergyReport {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	report := &EnergyReport{
		GeneratedAt: time.Now(),
		Disks:       make([]DiskEnergyInfo, 0),
	}

	totalSaved := 0.0
	for diskID, status := range pm.statuses {
		info := DiskEnergyInfo{
			DiskID:      diskID,
			State:       status.State,
			SleepCount:  status.SleepCount,
			WakeCount:   status.WakeCount,
			EnergySaved: status.EnergySaved,
		}
		report.Disks = append(report.Disks, info)
		totalSaved += status.EnergySaved
	}

	report.TotalEnergySaved = totalSaved
	return report
}

// EnergyReport represents energy saving report.
type EnergyReport struct {
	GeneratedAt      time.Time        `json:"generated_at"`
	Disks            []DiskEnergyInfo `json:"disks"`
	TotalEnergySaved float64          `json:"total_energy_saved"` // kWh
}

// DiskEnergyInfo represents disk energy information.
type DiskEnergyInfo struct {
	DiskID      string     `json:"disk_id"`
	State       PowerState `json:"state"`
	SleepCount  int        `json:"sleep_count"`
	WakeCount   int        `json:"wake_count"`
	EnergySaved float64    `json:"energy_saved"` // kWh
}

// DefaultSleepPolicy returns the default sleep policy.
func DefaultSleepPolicy() *SleepPolicy {
	return &SleepPolicy{
		ID:               "default",
		Name:             "默认节能策略",
		IdleThreshold:    5 * time.Minute,
		StandbyThreshold: 15 * time.Minute,
		SleepThreshold:   30 * time.Minute,
		Enabled:          true,
		ExcludedDisks:    []string{},
	}
}