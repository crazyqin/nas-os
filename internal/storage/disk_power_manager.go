// Package storage 提供磁盘智能休眠管理功能
// 参考飞牛fnOS的硬盘休眠唤醒机制实现
package storage

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ============== 休眠级别定义 ==============

// PowerState 磁盘电源状态
type PowerState string

const (
	// PowerStateActive 活跃状态 - 磁盘完全运行
	PowerStateActive PowerState = "active"
	// PowerStateLightSleep 轻度休眠 - 5分钟无访问，快速唤醒
	PowerStateLightSleep PowerState = "light_sleep"
	// PowerStateDeepSleep 深度休眠 - 30分钟无访问，中等唤醒时间
	PowerStateDeepSleep PowerState = "deep_sleep"
	// PowerStateHibernate 休眠 - 2小时无访问，完全停止
	PowerStateHibernate PowerState = "hibernate"
)

// WakePriority 唤醒优先级
type WakePriority int

const (
	// PriorityLow 低优先级 - 普通读取操作
	PriorityLow WakePriority = iota
	// PriorityNormal 普通优先级 - 用户交互操作
	PriorityNormal
	// PriorityHigh 高优先级 - 备份/同步任务
	PriorityHigh
	// PriorityCritical 紧急优先级 - 系统关键操作
	PriorityCritical
)

// PowerConfig 电源管理配置
type PowerConfig struct {
	// LightSleepThreshold 轻度休眠阈值（无访问时间）
	LightSleepThreshold time.Duration `json:"lightSleepThreshold"`
	// DeepSleepThreshold 深度休眠阈值
	DeepSleepThreshold time.Duration `json:"deepSleepThreshold"`
	// HibernateThreshold 完全休眠阈值
	HibernateThreshold time.Duration `json:"hibernateThreshold"`
	// WakeDelay 唤醒延迟，避免瞬间多次唤醒
	WakeDelay time.Duration `json:"wakeDelay"`
	// CheckInterval 状态检查间隔
	CheckInterval time.Duration `json:"checkInterval"`
	// EnableSmartSleep 启用智能休眠（根据使用模式调整）
	EnableSmartSleep bool `json:"enableSmartSleep"`
}

// DefaultPowerConfig 默认电源管理配置
var DefaultPowerConfig = &PowerConfig{
	LightSleepThreshold: 5 * time.Minute,
	DeepSleepThreshold:  30 * time.Minute,
	HibernateThreshold:  2 * time.Hour,
	WakeDelay:           5 * time.Second,
	CheckInterval:       30 * time.Second,
	EnableSmartSleep:    true,
}

// ============== 磁盘电源状态 ==============

// DiskPowerState 磁盘电源状态信息
type DiskPowerState struct {
	// Device 设备路径（如 /dev/sda）
	Device string `json:"device"`
	// CurrentState 当前电源状态
	CurrentState PowerState `json:"currentState"`
	// LastAccess 最后访问时间
	LastAccess time.Time `json:"lastAccess"`
	// LastStateChange 最后状态变更时间
	LastStateChange time.Time `json:"lastStateChange"`
	// WakeCount 唤醒次数统计
	WakeCount int64 `json:"wakeCount"`
	// SleepCount 休眠次数统计
	SleepCount int64 `json:"sleepCount"`
	// TotalSleepTime 总休眠时间
	TotalSleepTime time.Duration `json:"totalSleepTime"`
	// PendingWake 是否有待处理唤醒
	PendingWake bool `json:"pendingWake"`
	// WakeRequestedAt 唤醒请求时间
	WakeRequestedAt *time.Time `json:"wakeRequestedAt,omitempty"`
	// CurrentWakePriority 当前唤醒优先级
	CurrentWakePriority WakePriority `json:"currentWakePriority"`
}

// ============== 唤醒请求 ==============

// WakeRequest 唤醒请求
type WakeRequest struct {
	// Device 设备路径
	Device string `json:"device"`
	// Priority 优先级
	Priority WakePriority `json:"priority"`
	// Reason 唤醒原因
	Reason string `json:"reason"`
	// RequestedAt 请求时间
	RequestedAt time.Time `json:"requestedAt"`
	// TaskID 关联任务ID（用于追踪）
	TaskID string `json:"taskId,omitempty"`
	// ResponseChan 响应通道（唤醒完成后通知）
	ResponseChan chan error `json:"-"`
}

// ============== 电源管理器 ==============

// DiskPowerManager 磁盘电源管理器
type DiskPowerManager struct {
	config    *PowerConfig
	disks     map[string]*DiskPowerState
	wakeQueue *WakePriorityQueue
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	logger    *zap.Logger

	// 回调函数
	onStateChange  func(device string, oldState, newState PowerState)
	onWakeRequest  func(req *WakeRequest)
	onDiskSleep    func(device string, state PowerState)
	onDiskWake     func(device string, priority WakePriority)

	// 智能休眠学习数据
	accessPatterns map[string]*AccessPattern
}

// AccessPattern 访问模式学习数据
type AccessPattern struct {
	Device         string        `json:"device"`
	HourlyAccess   [24]int       `json:"hourlyAccess"`   // 每小时访问次数
	DailyAccess    [7]int        `json:"dailyAccess"`   // 每周每天访问次数
	LastUpdate     time.Time     `json:"lastUpdate"`
	AvgIdleTime    time.Duration `json:"avgIdleTime"`    // 平均空闲时间
	SleepTolerance time.Duration `json:"sleepTolerance"` // 可容忍休眠时间
}

// NewDiskPowerManager 创建磁盘电源管理器
func NewDiskPowerManager(config *PowerConfig, logger *zap.Logger) *DiskPowerManager {
	if config == nil {
		config = DefaultPowerConfig
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &DiskPowerManager{
		config:        config,
		disks:         make(map[string]*DiskPowerState),
		wakeQueue:     NewWakePriorityQueue(),
		ctx:           ctx,
		cancel:        cancel,
		logger:        logger,
		accessPatterns: make(map[string]*AccessPattern),
	}
}

// Start 启动电源管理器
func (m *DiskPowerManager) Start() {
	m.wg.Add(1)
	go m.stateMonitor()
	m.wg.Add(1)
	go m.wakeProcessor()

	if m.logger != nil {
		m.logger.Info("磁盘电源管理器已启动",
			zap.Duration("轻度休眠阈值", m.config.LightSleepThreshold),
			zap.Duration("深度休眠阈值", m.config.DeepSleepThreshold),
			zap.Duration("完全休眠阈值", m.config.HibernateThreshold),
			zap.Duration("唤醒延迟", m.config.WakeDelay),
		)
	}
}

// Stop 停止电源管理器
func (m *DiskPowerManager) Stop() {
	m.cancel()
	m.wg.Wait()

	if m.logger != nil {
		m.logger.Info("磁盘电源管理器已停止")
	}
}

// ============== 磁盘注册与状态管理 ==============

// RegisterDisk 注册磁盘到电源管理
func (m *DiskPowerManager) RegisterDisk(device string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.disks[device]; !exists {
		now := time.Now()
		m.disks[device] = &DiskPowerState{
			Device:          device,
			CurrentState:    PowerStateActive,
			LastAccess:      now,
			LastStateChange: now,
		}
		m.accessPatterns[device] = &AccessPattern{
			Device:     device,
			LastUpdate: now,
		}

		if m.logger != nil {
			m.logger.Info("磁盘已注册到电源管理", zap.String("device", device))
		}
	}
}

// UnregisterDisk 从电源管理移除磁盘
func (m *DiskPowerManager) UnregisterDisk(device string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.disks, device)
	delete(m.accessPatterns, device)

	if m.logger != nil {
		m.logger.Info("磁盘已从电源管理移除", zap.String("device", device))
	}
}

// GetDiskState 获取磁盘电源状态
func (m *DiskPowerManager) GetDiskState(device string) (*DiskPowerState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.disks[device]
	if !exists {
		return nil, ErrDiskNotRegistered
	}

	// 返回副本，避免外部修改
	copy := *state
	return &copy, nil
}

// GetAllDiskStates 获取所有磁盘电源状态
func (m *DiskPowerManager) GetAllDiskStates() map[string]*DiskPowerState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*DiskPowerState, len(m.disks))
	for device, state := range m.disks {
		copy := *state
		result[device] = &copy
	}
	return result
}

// ============== 访问记录 ==============

// RecordAccess 记录磁盘访问（更新最后访问时间）
func (m *DiskPowerManager) RecordAccess(device string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.disks[device]
	if !exists {
		return ErrDiskNotRegistered
	}

	now := time.Now()
	oldState := state.CurrentState

	// 更新访问时间
	state.LastAccess = now

	// 如果磁盘处于休眠状态，触发唤醒
	if state.CurrentState != PowerStateActive {
		state.PendingWake = true
		state.WakeRequestedAt = &now
		state.CurrentWakePriority = PriorityNormal

		// 添加到唤醒队列
		m.wakeQueue.Push(&WakeRequest{
			Device:      device,
			Priority:    PriorityNormal,
			Reason:      "访问请求",
			RequestedAt: now,
		})
	}

	// 更新访问模式学习数据
	m.updateAccessPatternLocked(device, now)

	// 如果从休眠变为活跃，记录唤醒
	if oldState != PowerStateActive && state.CurrentState == PowerStateActive {
		state.WakeCount++
		if m.onDiskWake != nil {
			m.onDiskWake(device, PriorityNormal)
		}
	}

	return nil
}

// RecordAccessWithPriority 记录高优先级访问（备份/同步任务）
func (m *DiskPowerManager) RecordAccessWithPriority(device string, priority WakePriority, taskID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.disks[device]
	if !exists {
		return ErrDiskNotRegistered
	}

	now := time.Now()
	oldState := state.CurrentState

	// 更新访问时间
	state.LastAccess = now

	// 如果磁盘处于休眠状态，添加高优先级唤醒请求
	if state.CurrentState != PowerStateActive {
		state.PendingWake = true
		state.WakeRequestedAt = &now
		state.CurrentWakePriority = priority

		m.wakeQueue.Push(&WakeRequest{
			Device:      device,
			Priority:    priority,
			Reason:      reason,
			RequestedAt: now,
			TaskID:      taskID,
		})
	}

	// 更新访问模式
	m.updateAccessPatternLocked(device, now)

	// 状态变化回调
	if oldState != PowerStateActive && state.CurrentState == PowerStateActive {
		state.WakeCount++
		if m.onDiskWake != nil {
			m.onDiskWake(device, priority)
		}
	}

	return nil
}

// updateAccessPatternLocked 更新访问模式（需要持有锁）
func (m *DiskPowerManager) updateAccessPatternLocked(device string, accessTime time.Time) {
	pattern, exists := m.accessPatterns[device]
	if !exists {
		pattern = &AccessPattern{
			Device:     device,
			LastUpdate: accessTime,
		}
		m.accessPatterns[device] = pattern
	}

	// 更新小时访问计数
	hour := accessTime.Hour()
	pattern.HourlyAccess[hour]++

	// 更新星期访问计数
	weekday := int(accessTime.Weekday())
	pattern.DailyAccess[weekday]++

	pattern.LastUpdate = accessTime
}

// ============== 休眠控制 ==============

// ForceSleep 强制磁盘进入指定休眠状态
func (m *DiskPowerManager) ForceSleep(device string, state PowerState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	diskState, exists := m.disks[device]
	if !exists {
		return ErrDiskNotRegistered
	}

	return m.transitionToStateLocked(diskState, state)
}

// ForceWake 强制唤醒磁盘
func (m *DiskPowerManager) ForceWake(device string, priority WakePriority) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.disks[device]
	if !exists {
		return ErrDiskNotRegistered
	}

	if state.CurrentState == PowerStateActive {
		// 已经是活跃状态，更新访问时间
		state.LastAccess = time.Now()
		return nil
	}

	// 添加高优先级唤醒请求
	now := time.Now()
	state.PendingWake = true
	state.WakeRequestedAt = &now
	state.CurrentWakePriority = priority

	m.wakeQueue.Push(&WakeRequest{
		Device:      device,
		Priority:    priority,
		Reason:      "强制唤醒",
		RequestedAt: now,
	})

	return nil
}

// transitionToStateLocked 状态转换（需要持有锁）
func (m *DiskPowerManager) transitionToStateLocked(state *DiskPowerState, newState PowerState) error {
	oldState := state.CurrentState
	if oldState == newState {
		return nil
	}

	now := time.Now()

	// 记录休眠时间
	if oldState == PowerStateActive && newState != PowerStateActive {
		state.SleepCount++
	}

	// 执行状态转换
	state.CurrentState = newState
	state.LastStateChange = now

	// 回调通知
	if m.onStateChange != nil {
		m.onStateChange(state.Device, oldState, newState)
	}

	if newState != PowerStateActive && m.onDiskSleep != nil {
		m.onDiskSleep(state.Device, newState)
	}

	if m.logger != nil {
		m.logger.Info("磁盘状态转换",
			zap.String("device", state.Device),
			zap.String("oldState", string(oldState)),
			zap.String("newState", string(newState)),
		)
	}

	return nil
}

// ============== 状态监控协程 ==============

// stateMonitor 状态监控主循环
func (m *DiskPowerManager) stateMonitor() {
	defer m.wg.Done()

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

// checkDiskStates 检查所有磁盘状态
func (m *DiskPowerManager) checkDiskStates() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	for device, state := range m.disks {
		// 跳过有待处理唤醒的磁盘
		if state.PendingWake {
			continue
		}

		// 计算空闲时间
		idleTime := now.Sub(state.LastAccess)

		// 根据配置的阈值决定休眠状态
		var newState PowerState
		switch {
		case idleTime >= m.config.HibernateThreshold:
			newState = PowerStateHibernate
		case idleTime >= m.config.DeepSleepThreshold:
			newState = PowerStateDeepSleep
		case idleTime >= m.config.LightSleepThreshold:
			newState = PowerStateLightSleep
		default:
			newState = PowerStateActive
		}

		// 智能休眠调整
		if m.config.EnableSmartSleep && newState != PowerStateActive {
			if adjustedState := m.adjustSleepByPattern(device, idleTime); adjustedState != newState {
				newState = adjustedState
			}
		}

		// 执行状态转换
		if state.CurrentState != newState {
			m.transitionToStateLocked(state, newState)
		}
	}
}

// adjustSleepByPattern 根据访问模式调整休眠状态
func (m *DiskPowerManager) adjustSleepByPattern(device string, idleTime time.Duration) PowerState {
	pattern, exists := m.accessPatterns[device]
	if !exists {
		return PowerStateActive
	}

	// 检查当前时段是否通常有高访问量
	now := time.Now()
	hour := now.Hour()

	// 如果当前小时通常有高访问量，延迟进入更深的休眠
	if pattern.HourlyAccess[hour] > 5 { // 阈值：5次访问
		// 用户通常在这个时段活跃，保持较浅的休眠
		if idleTime < m.config.DeepSleepThreshold*2 {
			return PowerStateLightSleep
		}
	}

	return PowerStateActive // 保持当前状态
}

// ============== 唤醒处理协程 ==============

// wakeProcessor 唤醒处理主循环
func (m *DiskPowerManager) wakeProcessor() {
	defer m.wg.Done()

	// 唤醒处理定时器
	wakeCheckInterval := time.NewTicker(100 * time.Millisecond)
	defer wakeCheckInterval.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-wakeCheckInterval.C:
			m.processWakeQueue()
		}
	}
}

// processWakeQueue 处理唤醒队列
func (m *DiskPowerManager) processWakeQueue() {
	// 从优先级队列获取最高优先级请求
	req := m.wakeQueue.Pop()
	if req == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.disks[req.Device]
	if !exists {
		if req.ResponseChan != nil {
			req.ResponseChan <- ErrDiskNotRegistered
		}
		return
	}

	// 已经是活跃状态
	if state.CurrentState == PowerStateActive {
		state.LastAccess = time.Now()
		state.PendingWake = false
		if req.ResponseChan != nil {
			req.ResponseChan <- nil
		}
		return
	}

	// 延迟唤醒机制：检查是否需要等待
	if state.WakeRequestedAt != nil {
		elapsed := time.Since(*state.WakeRequestedAt)
		if elapsed < m.config.WakeDelay {
			// 重新放回队列，等待延迟时间
			m.wakeQueue.Push(req)
			return
		}
	}

	// 执行唤醒
	m.performWakeLocked(state, req)
}

// performWakeLocked 执行唤醒（需要持有锁）
func (m *DiskPowerManager) performWakeLocked(state *DiskPowerState, req *WakeRequest) {
	now := time.Now()

	// 状态转换
	oldState := state.CurrentState
	state.CurrentState = PowerStateActive
	state.LastAccess = now
	state.LastStateChange = now
	state.PendingWake = false
	state.WakeRequestedAt = nil
	state.WakeCount++

	// 回调通知
	if m.onStateChange != nil {
		m.onStateChange(state.Device, oldState, PowerStateActive)
	}

	if m.onDiskWake != nil {
		m.onDiskWake(state.Device, req.Priority)
	}

	if m.onWakeRequest != nil {
		m.onWakeRequest(req)
	}

	if m.logger != nil {
		m.logger.Info("磁盘已唤醒",
			zap.String("device", state.Device),
			zap.String("oldState", string(oldState)),
			zap.Int("priority", int(req.Priority)),
			zap.String("reason", req.Reason),
		)
	}

	// 响应请求者
	if req.ResponseChan != nil {
		req.ResponseChan <- nil
	}
}

// ============== 回调设置 ==============

// OnStateChange 设置状态变化回调
func (m *DiskPowerManager) OnStateChange(callback func(device string, oldState, newState PowerState)) {
	m.onStateChange = callback
}

// OnWakeRequest 设置唤醒请求回调
func (m *DiskPowerManager) OnWakeRequest(callback func(req *WakeRequest)) {
	m.onWakeRequest = callback
}

// OnDiskSleep 设置磁盘休眠回调
func (m *DiskPowerManager) OnDiskSleep(callback func(device string, state PowerState)) {
	m.onDiskSleep = callback
}

// OnDiskWake 设置磁盘唤醒回调
func (m *DiskPowerManager) OnDiskWake(callback func(device string, priority WakePriority)) {
	m.onDiskWake = callback
}

// ============== 统计与配置 ==============

// PowerStats 电源管理统计
type PowerStats struct {
	TotalDisks       int           `json:"totalDisks"`
	ActiveDisks      int           `json:"activeDisks"`
	SleepingDisks    int           `json:"sleepingDisks"`
	TotalWakeCount   int64         `json:"totalWakeCount"`
	TotalSleepCount  int64         `json:"totalSleepCount"`
	TotalSleepTime   time.Duration `json:"totalSleepTime"`
	PendingWakeQueue int           `json:"pendingWakeQueue"`
}

// GetStats 获取电源管理统计
func (m *DiskPowerManager) GetStats() *PowerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &PowerStats{
		TotalDisks:       len(m.disks),
		PendingWakeQueue: m.wakeQueue.Len(),
	}

	for _, state := range m.disks {
		if state.CurrentState == PowerStateActive {
			stats.ActiveDisks++
		} else {
			stats.SleepingDisks++
		}
		stats.TotalWakeCount += state.WakeCount
		stats.TotalSleepCount += state.SleepCount
		stats.TotalSleepTime += state.TotalSleepTime
	}

	return stats
}

// UpdateConfig 更新配置
func (m *DiskPowerManager) UpdateConfig(config *PowerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.LightSleepThreshold > 0 {
		m.config.LightSleepThreshold = config.LightSleepThreshold
	}
	if config.DeepSleepThreshold > 0 {
		m.config.DeepSleepThreshold = config.DeepSleepThreshold
	}
	if config.HibernateThreshold > 0 {
		m.config.HibernateThreshold = config.HibernateThreshold
	}
	if config.WakeDelay > 0 {
		m.config.WakeDelay = config.WakeDelay
	}
	if config.CheckInterval > 0 {
		m.config.CheckInterval = config.CheckInterval
	}
	m.config.EnableSmartSleep = config.EnableSmartSleep
}

// GetConfig 获取当前配置
func (m *DiskPowerManager) GetConfig() *PowerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 返回副本
	copy := *m.config
	return &copy
}

// ============== 错误定义 ==============

var (
	// ErrDiskNotRegistered 磁盘未注册
	ErrDiskNotRegistered = errors.New("磁盘未注册到电源管理")
	// ErrInvalidPowerState 无效的电源状态
	ErrInvalidPowerState = errors.New("无效的电源状态")
	// ErrWakeTimeout 唤醒超时
	ErrWakeTimeout = errors.New("磁盘唤醒超时")
	// ErrDiskAlreadyActive 磁盘已处于活跃状态
	ErrDiskAlreadyActive = errors.New("磁盘已处于活跃状态")
)