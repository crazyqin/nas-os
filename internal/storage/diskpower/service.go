// Package diskpower 磁盘电源管理
// 实现按需唤醒硬盘，对标飞牛fnOS省电功能
package diskpower

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ========== 电源状态定义 ==========

// PowerState 磁盘电源状态.
type PowerState uint8

const (
	// PowerActive 活动状态（完全供电）.
	PowerActive PowerState = 0x00
	// PowerIdle 待机状态（部分供电）.
	PowerIdle PowerState = 0x01
	// PowerStandby 待机状态（低功耗）.
	PowerStandby PowerState = 0x02
	// PowerSleep 睡眠状态（极低功耗）.
	PowerSleep PowerState = 0x03
	// PowerSpindown 停转状态（电机停止）.
	PowerSpindown PowerState = 0x04
	// PowerUnknown 未知状态.
	PowerUnknown PowerState = 0xFF
)

// String 返回电源状态字符串.
func (s PowerState) String() string {
	switch s {
	case PowerActive:
		return "active"
	case PowerIdle:
		return "idle"
	case PowerStandby:
		return "standby"
	case PowerSleep:
		return "sleep"
	case PowerSpindown:
		return "spindown"
	default:
		return "unknown"
	}
}

// ========== 磁盘配置 ==========

// DiskConfig 磁盘电源配置.
type DiskConfig struct {
	DevicePath       string        `json:"device_path"`        // 设备路径 /dev/sdX
	SerialNumber     string        `json:"serial_number"`      // 序列号
	Model            string        `json:"model"`              // 型号
	IsSSD            bool          `json:"is_ssd"`             // 是否SSD
	SupportsAPM      bool          `json:"supports_apm"`       // 是否支持APM
	SupportsStandby  bool          `json:"supports_standby"`   // 是否支持待机
	IdleTimeout      time.Duration `json:"idle_timeout"`       // 空闲超时
	StandbyTimeout   time.Duration `json:"standby_timeout"`    // 待机超时
	SpindownTimeout  time.Duration `json:"spindown_timeout"`   // 停转超时
	WakeupLatency    time.Duration `json:"wakeup_latency"`     // 唤醒延迟预估
	MaxWakeupLatency time.Duration `json:"max_wakeup_latency"` // 最大允许唤醒延迟
	PowerSaveEnabled bool          `json:"power_save_enabled"` // 启用节能
	SmartMonitored   bool          `json:"smart_monitored"`    // SMART监控
	LastActivity     time.Time     `json:"last_activity"`      // 最后活动时间
}

// ========== 电源管理策略 ==========

// PowerPolicy 电源管理策略.
type PowerPolicy string

const (
	// PolicyAlwaysOn 常开模式（不节能）.
	PolicyAlwaysOn PowerPolicy = "always_on"
	// PolicyModerate 中等节能（适度停转）.
	PolicyModerate PowerPolicy = "moderate"
	// PolicyAggressive 激进节能（快速停转）.
	PolicyAggressive PowerPolicy = "aggressive"
	// PolicySmart 智能节能（学习用户行为）.
	PolicySmart PowerPolicy = "smart"
	// PolicyCustom 自定义策略.
	PolicyCustom PowerPolicy = "custom"
)

// PolicyConfig 策略配置.
type PolicyConfig struct {
	Policy            PowerPolicy   `json:"policy"`
	IdleThreshold     time.Duration `json:"idle_threshold"`     // 空闲阈值
	StandbyThreshold  time.Duration `json:"standby_threshold"`  // 待机阈值
	SpindownThreshold time.Duration `json:"spindown_threshold"` // 停转阈值
	WakeupCost        float64       `json:"wakeup_cost"`        // 唤醒成本评分
	PowerCost         float64       `json:"power_cost"`         // 功耗成本评分
	ActivityPattern   []timeRange   `json:"activity_pattern"`   // 活动模式
	ExcludeDisks      []string      `json:"exclude_disks"`      // 排除磁盘
}

type timeRange struct {
	StartHour int `json:"start_hour"`
	EndHour   int `json:"end_hour"`
}

// ========== 电源管理器 ==========

// PowerManager 磁盘电源管理器.
type PowerManager struct {
	disks    map[string]*DiskInfo        // 磁盘信息
	policy   *PolicyConfig               // 当前策略
	activity map[string]*ActivityTracker // 活动追踪器
	config   *PowerConfig                // 全局配置
	running  atomic.Bool                 // 运行状态
	ctx      context.Context             // 上下文
	cancel   context.CancelFunc          // 取消函数
	mu       sync.RWMutex                // 保护状态
	logger   Logger                      // 日志接口
	eventBus EventBus                    // 事件总线
}

// DiskInfo 磁盘运行信息.
type DiskInfo struct {
	Config      DiskConfig   `json:"config"`
	State       PowerState   `json:"state"`
	PrevState   PowerState   `json:"prev_state"`
	LastWakeUp  time.Time    `json:"last_wakeup"`
	WakeupCount uint64       `json:"wakeup_count"`
	PowerHours  float64      `json:"power_hours"` // 累计供电小时
	SaveHours   float64      `json:"save_hours"`  // 累计节能小时
	IOCount     uint64       `json:"io_count"`    // I/O计数
	mu          sync.RWMutex `json:"-"`
}

// ActivityTracker 活动追踪器.
type ActivityTracker struct {
	RecentIO     []time.Time         `json:"recent_io"`     // 最近I/O时间
	HourlyStats  map[int]*HourlyStat `json:"hourly_stats"`  // 每小时统计
	DailyStats   map[int]*DailyStat  `json:"daily_stats"`   // 每日统计
	PredictModel *PredictionModel    `json:"predict_model"` // 预测模型
	mu           sync.RWMutex        `json:"-"`
}

// HourlyStat 每小时统计.
type HourlyStat struct {
	Hour       int     `json:"hour"`
	IOCount    uint64  `json:"io_count"`
	ActiveTime float64 `json:"active_time"`
}

// DailyStat 每日统计.
type DailyStat struct {
	Day        int     `json:"day"`
	IOCount    uint64  `json:"io_count"`
	ActiveTime float64 `json:"active_time"`
}

// PredictionModel 预测模型.
type PredictionModel struct {
	PredictedActiveHours []int     `json:"predicted_active_hours"`
	Confidence           float64   `json:"confidence"`
	LastUpdated          time.Time `json:"last_updated"`
}

// PowerConfig 全局电源配置.
type PowerConfig struct {
	CheckInterval     time.Duration `json:"check_interval"`      // 检查间隔
	WakeupTimeout     time.Duration `json:"wakeup_timeout"`      // 唤醒超时
	MaxConcurrentWake int           `json:"max_concurrent_wake"` // 最大并发唤醒
	EnablePredictive  bool          `json:"enable_predictive"`   // 启用预测
	EnableSmart       bool          `json:"enable_smart"`        // 启用智能模式
	WakeupRetryCount  int           `json:"wakeup_retry_count"`  // 唤醒重试次数
	WakeupRetryDelay  time.Duration `json:"wakeup_retry_delay"`  // 唤醒重试延迟
}

// DefaultPowerConfig 默认电源配置.
func DefaultPowerConfig() *PowerConfig {
	return &PowerConfig{
		CheckInterval:     30 * time.Second,
		WakeupTimeout:     10 * time.Second,
		MaxConcurrentWake: 4,
		EnablePredictive:  true,
		EnableSmart:       true,
		WakeupRetryCount:  3,
		WakeupRetryDelay:  500 * time.Millisecond,
	}
}

// ========== 错误定义 ==========

var (
	ErrDiskNotFound       = errors.New("disk not found")
	ErrDiskAlreadyActive  = errors.New("disk already active")
	ErrDiskAlreadyStopped = errors.New("disk already stopped")
	ErrWakeupFailed       = errors.New("disk wakeup failed")
	ErrSpindownFailed     = errors.New("disk spindown failed")
	ErrPolicyNotSupported = errors.New("power policy not supported")
	ErrSSDCannotSpindown  = errors.New("ssd cannot spindown")
	ErrWakeInProgress     = errors.New("wake operation in progress")
)

// ========== 管理器方法 ==========

// NewPowerManager 创建电源管理器.
func NewPowerManager(config *PowerConfig, policy *PolicyConfig, logger Logger, eventBus EventBus) *PowerManager {
	if config == nil {
		config = DefaultPowerConfig()
	}
	if policy == nil {
		policy = &PolicyConfig{
			Policy:            PolicyModerate,
			IdleThreshold:     5 * time.Minute,
			StandbyThreshold:  15 * time.Minute,
			SpindownThreshold: 30 * time.Minute,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &PowerManager{
		disks:    make(map[string]*DiskInfo),
		policy:   policy,
		activity: make(map[string]*ActivityTracker),
		config:   config,
		ctx:      ctx,
		cancel:   cancel,
		logger:   logger,
		eventBus: eventBus,
	}
}

// Start 启动电源管理器.
func (m *PowerManager) Start() error {
	m.running.Store(true)

	// 启动状态检查循环
	go m.stateCheckLoop()

	// 启动活动追踪循环
	go m.activityTrackLoop()

	// 启动预测学习（如果启用）
	if m.config.EnablePredictive {
		go m.predictionLoop()
	}

	m.logger.Info("Disk power manager started")
	return nil
}

// Stop 停止电源管理器.
func (m *PowerManager) Stop() {
	m.running.Store(false)
	m.cancel()

	// 唤醒所有磁盘到活动状态（安全退出）
	m.wakeupAll()

	m.logger.Info("Disk power manager stopped")
}

// RegisterDisk 注册磁盘.
func (m *PowerManager) RegisterDisk(config DiskConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.disks[config.DevicePath]; exists {
		return errors.New("disk already registered")
	}

	disk := &DiskInfo{
		Config:     config,
		State:      PowerActive,
		PrevState:  PowerActive,
		LastWakeUp: time.Now(),
	}

	m.disks[config.DevicePath] = disk
	m.activity[config.DevicePath] = NewActivityTracker()

	m.logger.Infof("Disk registered: %s (%s)", config.DevicePath, config.Model)
	return nil
}

// UnregisterDisk 注销磁盘.
func (m *PowerManager) UnregisterDisk(devicePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.disks[devicePath]; !exists {
		return ErrDiskNotFound
	}

	delete(m.disks, devicePath)
	delete(m.activity, devicePath)

	m.logger.Infof("Disk unregistered: %s", devicePath)
	return nil
}

// WakeDisk 唤醒磁盘.
func (m *PowerManager) WakeDisk(devicePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	disk, exists := m.disks[devicePath]
	if !exists {
		return ErrDiskNotFound
	}

	disk.mu.Lock()
	defer disk.mu.Unlock()

	if disk.State == PowerActive {
		return ErrDiskAlreadyActive
	}

	// SSD不支持停转，但可能处于低功耗状态
	if disk.Config.IsSSD {
		disk.State = PowerActive
		disk.LastWakeUp = time.Now()
		disk.WakeupCount++
		m.logger.Infof("SSD wakeup: %s", devicePath)
		return nil
	}

	// HDD唤醒逻辑（实际需要调用hdparm或内核接口）
	// 这里模拟唤醒过程
	prevState := disk.State
	disk.PrevState = prevState
	disk.State = PowerActive
	disk.LastWakeUp = time.Now()
	disk.WakeupCount++

	// 发布唤醒事件
	if m.eventBus != nil {
		m.eventBus.Publish("disk.wakeup", map[string]interface{}{
			"device":     devicePath,
			"prev_state": prevState.String(),
			"wakeup_at":  disk.LastWakeUp,
		})
	}

	m.logger.Infof("Disk wakeup: %s (%s -> active)", devicePath, prevState.String())
	return nil
}

// SpindownDisk 停转磁盘.
func (m *PowerManager) SpindownDisk(devicePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	disk, exists := m.disks[devicePath]
	if !exists {
		return ErrDiskNotFound
	}

	disk.mu.Lock()
	defer disk.mu.Unlock()

	if disk.Config.IsSSD {
		return ErrSSDCannotSpindown
	}

	if disk.State == PowerSpindown {
		return ErrDiskAlreadyStopped
	}

	// 停转逻辑（实际需要调用hdparm -Y或内核接口）
	prevState := disk.State
	disk.PrevState = prevState
	disk.State = PowerSpindown

	// 发布停转事件
	if m.eventBus != nil {
		m.eventBus.Publish("disk.spindown", map[string]interface{}{
			"device":      devicePath,
			"prev_state":  prevState.String(),
			"spindown_at": time.Now(),
		})
	}

	m.logger.Infof("Disk spindown: %s (%s -> spindown)", devicePath, prevState.String())
	return nil
}

// GetDiskState 获取磁盘状态.
func (m *PowerManager) GetDiskState(devicePath string) (PowerState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, exists := m.disks[devicePath]
	if !exists {
		return PowerUnknown, ErrDiskNotFound
	}

	disk.mu.RLock()
	defer disk.mu.RUnlock()
	return disk.State, nil
}

// SetPolicy 设置电源策略.
func (m *PowerManager) SetPolicy(policy *PolicyConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.policy = policy
	m.logger.Infof("Power policy changed: %s", policy.Policy)
	return nil
}

// ========== 内部循环 ==========

func (m *PowerManager) stateCheckLoop() {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkDiskStates()
		}
	}
}

func (m *PowerManager) checkDiskStates() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()

	for devicePath, disk := range m.disks {
		disk.mu.Lock()

		// 检查是否在排除列表
		excluded := false
		for _, ex := range m.policy.ExcludeDisks {
			if ex == devicePath {
				excluded = true
				break
			}
		}

		if excluded || !disk.Config.PowerSaveEnabled {
			disk.mu.Unlock()
			continue
		}

		// 计算空闲时间
		idleTime := now.Sub(disk.Config.LastActivity)

		// SSD低功耗处理
		if disk.Config.IsSSD {
			if idleTime > m.policy.StandbyThreshold && disk.State == PowerActive {
				disk.PrevState = disk.State
				disk.State = PowerStandby
				m.logger.Infof("SSD standby: %s", devicePath)
			}
			disk.mu.Unlock()
			continue
		}

		// HDD停转处理
		switch m.policy.Policy {
		case PolicyAlwaysOn:
			// 不节能

		case PolicyModerate:
			if idleTime > m.policy.SpindownThreshold && disk.State != PowerSpindown {
				disk.PrevState = disk.State
				disk.State = PowerSpindown
				m.logger.Infof("HDD spindown (moderate): %s, idle=%v",
					devicePath, idleTime)
			}

		case PolicyAggressive:
			if idleTime > m.policy.IdleThreshold && disk.State == PowerActive {
				disk.PrevState = disk.State
				disk.State = PowerIdle
			}
			if idleTime > m.policy.SpindownThreshold && disk.State != PowerSpindown {
				disk.PrevState = disk.State
				disk.State = PowerSpindown
				m.logger.Infof("HDD spindown (aggressive): %s, idle=%v",
					devicePath, idleTime)
			}

		case PolicySmart:
			// 使用预测模型决定
			tracker := m.activity[devicePath]
			if tracker != nil {
				if m.shouldBeActive(tracker, now) {
					// 保持活跃
					if disk.State != PowerActive {
						m.WakeDisk(devicePath)
					}
				} else if idleTime > m.policy.SpindownThreshold && disk.State != PowerSpindown {
					disk.PrevState = disk.State
					disk.State = PowerSpindown
					m.logger.Infof("HDD spindown (smart): %s", devicePath)
				}
			}
		}

		disk.mu.Unlock()
	}
}

func (m *PowerManager) activityTrackLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.updateActivityStats()
		}
	}
}

func (m *PowerManager) updateActivityStats() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	hour := now.Hour()
	day := now.Day()

	for _, tracker := range m.activity {
		tracker.mu.Lock()

		// 更新每小时统计
		if tracker.HourlyStats[hour] == nil {
			tracker.HourlyStats[hour] = &HourlyStat{Hour: hour}
		}

		// 更新每日统计
		if tracker.DailyStats[day] == nil {
			tracker.DailyStats[day] = &DailyStat{Day: day}
		}

		tracker.mu.Unlock()
	}
}

func (m *PowerManager) predictionLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.updatePredictionModel()
		}
	}
}

func (m *PowerManager) updatePredictionModel() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for devicePath, tracker := range m.activity {
		tracker.mu.Lock()

		// 分析每小时活动模式
		activeHours := make([]int, 0)
		for hour, stat := range tracker.HourlyStats {
			if stat.IOCount > 100 { // 活动阈值
				activeHours = append(activeHours, hour)
			}
		}

		tracker.PredictModel = &PredictionModel{
			PredictedActiveHours: activeHours,
			Confidence:           0.8,
			LastUpdated:          time.Now(),
		}

		tracker.mu.Unlock()
		m.logger.Infof("Prediction updated for %s: active_hours=%v", devicePath, activeHours)
	}
}

func (m *PowerManager) shouldBeActive(tracker *ActivityTracker, now time.Time) bool {
	tracker.mu.RLock()
	defer tracker.mu.RUnlock()

	if tracker.PredictModel == nil {
		return false
	}

	hour := now.Hour()
	for _, h := range tracker.PredictModel.PredictedActiveHours {
		if h == hour {
			return true
		}
	}
	return false
}

func (m *PowerManager) wakeupAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for devicePath := range m.disks {
		m.WakeDisk(devicePath)
	}
}

// ========== 统计信息 ==========

// GetStats 获取电源统计.
func (m *PowerManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	diskStats := make([]map[string]interface{}, 0)
	for path, disk := range m.disks {
		disk.mu.RLock()
		diskStats = append(diskStats, map[string]interface{}{
			"device":       path,
			"model":        disk.Config.Model,
			"is_ssd":       disk.Config.IsSSD,
			"state":        disk.State.String(),
			"wakeup_count": disk.WakeupCount,
			"power_hours":  disk.PowerHours,
			"save_hours":   disk.SaveHours,
		})
		disk.mu.RUnlock()
	}

	return map[string]interface{}{
		"policy":      m.policy.Policy,
		"disks":       diskStats,
		"total_disks": len(m.disks),
		"running":     m.running.Load(),
	}
}

// ========== ActivityTracker构造 ==========

// NewActivityTracker 创建活动追踪器.
func NewActivityTracker() *ActivityTracker {
	return &ActivityTracker{
		RecentIO:    make([]time.Time, 0),
		HourlyStats: make(map[int]*HourlyStat),
		DailyStats:  make(map[int]*DailyStat),
	}
}

// ========== 接口定义 ==========

// Logger 日志接口.
type Logger interface {
	Info(msg string)
	Infof(format string, args ...interface{})
	Warn(msg string)
	Warnf(format string, args ...interface{})
	Error(msg string)
	Errorf(format string, args ...interface{})
}

// EventBus 事件总线接口.
type EventBus interface {
	Publish(topic string, data map[string]interface{})
}
