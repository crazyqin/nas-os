// Package disk provides intelligent disk sleep enhancement.
// 智能休眠策略引擎 - 基于访问模式学习、温度联动、服务感知的高级休眠管理。
package disk

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ==================== 时间槽与调度 ====================

// TimeSlot 时间槽 - 用于描述周期性的时间窗口
type TimeSlot struct {
	DayOfWeek time.Weekday `json:"day_of_week"` // 星期几 (0=周日)
	StartHour int         `json:"start_hour"`   // 开始小时 (0-23)
	EndHour   int         `json:"end_hour"`     // 结束小时 (0-23)
	Label     string      `json:"label"`        // 标签（如"工作日白天"）
}

// SleepSchedule 休眠调度表 - 工作日/周末差异化策略
type SleepSchedule struct {
	WorkdayPolicyID  string `json:"workday_policy_id"`  // 工作日使用的策略ID
	WeekendPolicyID  string `json:"weekend_policy_id"`  // 周末使用的策略ID
	HolidayPolicyID  string `json:"holiday_policy_id"`  // 节假日使用的策略ID
	Enabled          bool   `json:"enabled"`
}

// ==================== 访问模式学习 ====================

// AccessRecord 单次访问记录
type AccessRecord struct {
	Timestamp time.Time `json:"timestamp"` // 访问时间
	Duration  time.Duration `json:"duration"` // 访问持续时长
	Type      AccessType    `json:"type"`     // 访问类型
}

// AccessType 访问类型
type AccessType string

const (
	AccessRead    AccessType = "read"    // 读取
	AccessWrite   AccessType = "write"   // 写入
	AccessService AccessType = "service" // 服务访问
)

// AccessPattern 访问模式 - 学习磁盘的使用规律
type AccessPattern struct {
	DiskID           string           `json:"disk_id"`
	HourlyFrequency  [24]int          `json:"hourly_frequency"`  // 24小时访问频率分布
	DailyFrequency   [7]int           `json:"daily_frequency"`   // 每周7天访问频率
	AvgAccessInterval time.Duration   `json:"avg_access_interval"` // 平均访问间隔
	LastAccess       time.Time        `json:"last_access"`       // 最近访问时间
	TotalAccessCount int64            `json:"total_access_count"` // 总访问次数
	PeakHours        []int            `json:"peak_hours"`        // 高峰时段
	QuietHours       []int            `json:"quiet_hours"`       // 低谷时段
	Records          []AccessRecord   `json:"records"`           // 近期访问记录
	MaxRecords       int              `json:"max_records"`       // 最大记录数
}

// ServiceDependency 服务依赖
type ServiceDependency struct {
	Name         string    `json:"name"`          // 服务名（smb/nfs/ftp等）
	Active       bool      `json:"active"`        // 是否活跃
	LastCheck    time.Time `json:"last_check"`    // 最近检查时间
	ActiveConns  int       `json:"active_conns"`  // 活跃连接数
}

// ==================== 温度监控 ====================

// TemperatureThreshold 温度阈值配置
type TemperatureThreshold struct {
	WarningTemp  float64 `json:"warning_temp"`  // 警告温度(℃)
	ThrottleTemp float64 `json:"throttle_temp"` // 降频温度(℃)，提前休眠散热
	CriticalTemp float64 `json:"critical_temp"` // 临界温度(℃)，强制休眠
}

// DiskTemperatureInfo 磁盘温度信息
type DiskTemperatureInfo struct {
	DiskID        string    `json:"disk_id"`
	CurrentTemp   float64   `json:"current_temp"`   // 当前温度(℃)
	AverageTemp   float64   `json:"average_temp"`   // 平均温度(℃)
	MaxTemp       float64   `json:"max_temp"`       // 最高温度(℃)
	LastUpdated   time.Time `json:"last_updated"`   // 最近更新时间
	TempThreshold TemperatureThreshold `json:"temp_threshold"` // 温度阈值
}

// ==================== 智能休眠策略 ====================

// SmartSleepPolicy 智能休眠策略（扩展自基础 SleepPolicy）
type SmartSleepPolicy struct {
	*SleepPolicy                          // 嵌入基础策略
	TemperatureThreshold TemperatureThreshold `json:"temperature_threshold"` // 温度阈值
	PredictiveWake       bool                `json:"predictive_wake"`       // 是否启用预测性唤醒
	PredictiveWakeWindow time.Duration       `json:"predictive_wake_window"` // 预测唤醒提前量
	ServiceCheckEnabled  bool                `json:"service_check_enabled"` // 是否检查服务依赖
	MinIdleBeforeSleep   time.Duration       `json:"min_idle_before_sleep"` // 最小空闲时长才允许休眠
}

// ==================== SmartSleepManager ====================

// SmartSleepManager 智能休眠策略引擎
type SmartSleepManager struct {
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	running     bool

	// 访问模式
	patterns    map[string]*AccessPattern   // diskID -> 访问模式

	// 休眠策略与调度
	policies    map[string]*SmartSleepPolicy // policyID -> 智能策略
	schedules   map[string]*SleepSchedule    // diskID -> 调度表
	defaultPolicy *SmartSleepPolicy

	// 温度监控
	temperatures map[string]*DiskTemperatureInfo // diskID -> 温度信息

	// 服务依赖
	dependencies map[string][]ServiceDependency // diskID -> 服务依赖列表

	// 配置
	config SmartSleepConfig

	// 学习控制
	learnTicker  *time.Ticker
	patternMu    sync.RWMutex
}

// SmartSleepConfig 智能休眠配置
type SmartSleepConfig struct {
	// 访问模式学习
	LearnInterval     time.Duration `json:"learn_interval"`      // 学习周期
	PatternWindowSize int           `json:"pattern_window_size"` // 模式窗口大小（记录数）
	QuietHourThreshold int          `json:"quiet_hour_threshold"` // 低谷判定阈值（每小时访问次数）

	// 温度联动
	TempCheckInterval time.Duration `json:"temp_check_interval"` // 温度检查间隔
	DefaultTempThreshold TemperatureThreshold `json:"default_temp_threshold"` // 默认温度阈值

	// 预测唤醒
	PredictionLookahead time.Duration `json:"prediction_lookahead"` // 预测提前量

	// 服务依赖
	ServiceCheckInterval time.Duration `json:"service_check_interval"` // 服务检查间隔
}

// DefaultSmartSleepConfig 默认智能休眠配置
func DefaultSmartSleepConfig() SmartSleepConfig {
	return SmartSleepConfig{
		LearnInterval:      10 * time.Minute,
		PatternWindowSize:  1000,
		QuietHourThreshold: 5,
		TempCheckInterval:  30 * time.Second,
		DefaultTempThreshold: TemperatureThreshold{
			WarningTemp:  45.0,
			ThrottleTemp: 50.0,
			CriticalTemp: 55.0,
		},
		PredictionLookahead:  15 * time.Minute,
		ServiceCheckInterval: 1 * time.Minute,
	}
}

// ==================== 构造与生命周期 ====================

// NewSmartSleepManager 创建智能休眠管理器
func NewSmartSleepManager(cfg *SmartSleepConfig) *SmartSleepManager {
	if cfg == nil {
		defaultCfg := DefaultSmartSleepConfig()
		cfg = &defaultCfg
	}

	return &SmartSleepManager{
		patterns:     make(map[string]*AccessPattern),
		policies:     make(map[string]*SmartSleepPolicy),
		schedules:    make(map[string]*SleepSchedule),
		temperatures: make(map[string]*DiskTemperatureInfo),
		dependencies: make(map[string][]ServiceDependency),
		config:       *cfg,
	}
}

// Start 启动智能休眠管理器
func (m *SmartSleepManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("SmartSleepManager已在运行")
	}

	m.ctx, m.cancel = context.WithCancel(ctx)
	m.running = true
	m.learnTicker = time.NewTicker(m.config.LearnInterval)

	// 启动后台学习循环
	go m.learningLoop()

	return nil
}

// Stop 停止智能休眠管理器
func (m *SmartSleepManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false
	if m.cancel != nil {
		m.cancel()
	}
	if m.learnTicker != nil {
		m.learnTicker.Stop()
	}
}

// IsRunning 是否在运行
func (m *SmartSleepManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// ==================== 访问记录与模式学习 ====================

// RecordAccess 记录磁盘访问事件
func (m *SmartSleepManager) RecordAccess(diskID string, accessType AccessType, duration time.Duration) {
	m.patternMu.Lock()
	defer m.patternMu.Unlock()

	now := time.Now()

	pattern := m.patterns[diskID]
	if pattern == nil {
		pattern = &AccessPattern{
			DiskID:     diskID,
			MaxRecords: m.config.PatternWindowSize,
		}
		m.patterns[diskID] = pattern
	}

	// 添加访问记录
	record := AccessRecord{
		Timestamp: now,
		Duration:  duration,
		Type:      accessType,
	}
	pattern.Records = append(pattern.Records, record)

	// 超过最大记录数则裁剪
	if len(pattern.Records) > pattern.MaxRecords {
		pattern.Records = pattern.Records[len(pattern.Records)-pattern.MaxRecords:]
	}

	// 更新频率统计
	hour := now.Hour()
	weekday := int(now.Weekday())
	pattern.HourlyFrequency[hour]++
	pattern.DailyFrequency[weekday]++
	pattern.TotalAccessCount++
	pattern.LastAccess = now

	// 更新平均访问间隔
	if pattern.TotalAccessCount > 1 {
		totalDuration := now.Sub(pattern.Records[0].Timestamp)
		pattern.AvgAccessInterval = totalDuration / time.Duration(pattern.TotalAccessCount)
	}

	// 更新高峰/低谷时段
	m.updatePeakQuietHours(pattern)
}

// UpdateTemperature 更新磁盘温度
func (m *SmartSleepManager) UpdateTemperature(diskID string, temp float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	info := m.temperatures[diskID]
	if info == nil {
		info = &DiskTemperatureInfo{
			DiskID:        diskID,
			TempThreshold: m.config.DefaultTempThreshold,
		}
		m.temperatures[diskID] = info
	}

	info.CurrentTemp = temp
	info.LastUpdated = time.Now()
	if temp > info.MaxTemp {
		info.MaxTemp = temp
	}

	// 滚动平均温度
	if info.AverageTemp == 0 {
		info.AverageTemp = temp
	} else {
		info.AverageTemp = (info.AverageTemp*0.9 + temp*0.1)
	}
}

// RegisterServiceDependency 注册服务依赖
func (m *SmartSleepManager) RegisterServiceDependency(diskID string, dep ServiceDependency) {
	m.mu.Lock()
	defer m.mu.Unlock()

	deps := m.dependencies[diskID]
	// 去重：按名称更新
	for i, d := range deps {
		if d.Name == dep.Name {
			deps[i] = dep
			m.dependencies[diskID] = deps
			return
		}
	}
	m.dependencies[diskID] = append(deps, dep)
}

// UpdateServiceStatus 更新服务活跃状态
func (m *SmartSleepManager) UpdateServiceStatus(diskID, serviceName string, active bool, conns int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	deps := m.dependencies[diskID]
	for i, d := range deps {
		if d.Name == serviceName {
			deps[i].Active = active
			deps[i].ActiveConns = conns
			deps[i].LastCheck = time.Now()
			return
		}
	}
}

// ==================== 休眠决策 ====================

// ShouldSleep 判断磁盘是否应该休眠
// 综合考虑：访问模式、温度、服务依赖、调度策略
func (m *SmartSleepManager) ShouldSleep(diskID string) (bool, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()

	// 1. 检查服务依赖
	if m.hasActiveService(diskID) {
		return false, "存在活跃的服务依赖（SMB/NFS等）"
	}

	// 2. 检查温度 - 高温时提前休眠散热
	if m.isOverTempThreshold(diskID) {
		return true, "磁盘温度过高，提前休眠散热"
	}

	// 3. 获取适用策略
	policy := m.getEffectivePolicy(diskID, now)
	if policy == nil {
		return false, "未配置休眠策略"
	}

	// 4. 检查访问模式 - 是否处于低谷时段
	if !m.isQuietPeriod(diskID, now) {
		return false, "当前时段访问频繁"
	}

	// 5. 基于历史模式判断
	if m.hasRecentAccess(diskID, policy.MinIdleBeforeSleep) {
		return false, "距离最近访问时间不足"
	}

	return true, "满足智能休眠条件"
}

// GetNextWakeTime 获取预测的下次唤醒时间
// 基于历史访问模式预测下一个活跃时段
func (m *SmartSleepManager) GetNextWakeTime(diskID string) time.Time {
	m.patternMu.RLock()
	defer m.patternMu.RUnlock()

	pattern := m.patterns[diskID]
	if pattern == nil || pattern.TotalAccessCount == 0 {
		// 无历史数据，默认15分钟后唤醒
		return time.Now().Add(15 * time.Minute)
	}

	now := time.Now()

	// 查找下一个高峰时段
	nextWake := m.findNextPeakHour(pattern, now)
	if nextWake != nil {
		// 在预测的高峰时段前唤醒，留出预热时间
		wakeTime := nextWake.Add(-m.config.PredictionLookahead)
		if wakeTime.After(now) {
			return wakeTime
		}
	}

	// 回退到平均访问间隔
	if pattern.AvgAccessInterval > 0 {
		return now.Add(pattern.AvgAccessInterval)
	}

	return now.Add(15 * time.Minute)
}

// ==================== 模式学习 ====================

// LearnPattern 执行一次模式学习，分析历史访问数据
func (m *SmartSleepManager) LearnPattern(diskID string) *AccessPattern {
	m.patternMu.Lock()
	defer m.patternMu.Unlock()

	pattern := m.patterns[diskID]
	if pattern == nil || len(pattern.Records) == 0 {
		return nil
	}

	// 重新计算访问频率
	var hourly [24]int
	var daily [7]int
	var totalInterval time.Duration
	var intervalCount int

	for i, rec := range pattern.Records {
		hourly[rec.Timestamp.Hour()]++
		daily[rec.Timestamp.Weekday()]++
		if i > 0 {
			totalInterval += rec.Timestamp.Sub(pattern.Records[i-1].Timestamp)
			intervalCount++
		}
	}

	pattern.HourlyFrequency = hourly
	pattern.DailyFrequency = daily

	if intervalCount > 0 {
		pattern.AvgAccessInterval = totalInterval / time.Duration(intervalCount)
	}

	// 更新高峰/低谷时段
	m.updatePeakQuietHours(pattern)

	return pattern
}

// GetAccessPattern 获取访问模式（只读）
func (m *SmartSleepManager) GetAccessPattern(diskID string) *AccessPattern {
	m.patternMu.RLock()
	defer m.patternMu.RUnlock()
	p := m.patterns[diskID]
	if p == nil {
		return nil
	}
	// 返回副本
	cp := *p
	cp.Records = make([]AccessRecord, len(p.Records))
	copy(cp.Records, p.Records)
	return &cp
}

// ==================== 策略管理 ====================

// UpdatePolicy 更新智能休眠策略
func (m *SmartSleepManager) UpdatePolicy(policyID string, policy *SmartSleepPolicy) error {
	if policy == nil {
		return fmt.Errorf("策略不能为空")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.policies[policyID] = policy
	return nil
}

// AddPolicy 添加智能休眠策略
func (m *SmartSleepManager) AddPolicy(policy *SmartSleepPolicy) error {
	return m.UpdatePolicy(policy.ID, policy)
}

// GetPolicy 获取智能休眠策略
func (m *SmartSleepManager) GetPolicy(policyID string) *SmartSleepPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.policies[policyID]
}

// SetDefaultPolicy 设置默认策略
func (m *SmartSleepManager) SetDefaultPolicy(policy *SmartSleepPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultPolicy = policy
}

// SetSchedule 设置磁盘休眠调度表（工作日/周末差异化）
func (m *SmartSleepManager) SetSchedule(diskID string, schedule *SleepSchedule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.schedules[diskID] = schedule
}

// ==================== 内部方法 ====================

// learningLoop 后台学习循环
func (m *SmartSleepManager) learningLoop() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.learnTicker.C:
			m.runLearningCycle()
		}
	}
}

// runLearningCycle 执行一轮学习
func (m *SmartSleepManager) runLearningCycle() {
	m.patternMu.RLock()
	diskIDs := make([]string, 0, len(m.patterns))
	for id := range m.patterns {
		diskIDs = append(diskIDs, id)
	}
	m.patternMu.RUnlock()

	for _, diskID := range diskIDs {
		m.LearnPattern(diskID)
	}
}

// updatePeakQuietHours 更新高峰和低谷时段
func (m *SmartSleepManager) updatePeakQuietHours(pattern *AccessPattern) {
	threshold := m.config.QuietHourThreshold

	var peaks, quiets []int
	for hour, freq := range pattern.HourlyFrequency {
		if freq > threshold*2 {
			peaks = append(peaks, hour)
		} else if freq < threshold {
			quiets = append(quiets, hour)
		}
	}

	pattern.PeakHours = peaks
	pattern.QuietHours = quiets
}

// hasActiveService 检查是否有活跃的服务依赖
func (m *SmartSleepManager) hasActiveService(diskID string) bool {
	deps := m.dependencies[diskID]
	for _, dep := range deps {
		if dep.Active && dep.ActiveConns > 0 {
			return true
		}
	}
	return false
}

// isOverTempThreshold 检查是否超过温度阈值
func (m *SmartSleepManager) isOverTempThreshold(diskID string) bool {
	info := m.temperatures[diskID]
	if info == nil {
		return false
	}
	// 超过降频温度时触发提前休眠
	return info.CurrentTemp >= info.TempThreshold.ThrottleTemp
}

// getEffectivePolicy 获取当前生效的策略（考虑调度表）
func (m *SmartSleepManager) getEffectivePolicy(diskID string, now time.Time) *SmartSleepPolicy {
	// 检查是否有调度表
	schedule := m.schedules[diskID]
	if schedule != nil && schedule.Enabled {
		isWeekend := now.Weekday() == time.Saturday || now.Weekday() == time.Sunday

		var policyID string
		if isWeekend {
			policyID = schedule.WeekendPolicyID
		} else {
			policyID = schedule.WorkdayPolicyID
		}

		if policy, ok := m.policies[policyID]; ok {
			return policy
		}
	}

	// 回退到默认策略
	if m.defaultPolicy != nil {
		return m.defaultPolicy
	}

	// 尝试查找任何可用策略
	for _, p := range m.policies {
		return p
	}

	return nil
}

// isQuietPeriod 判断当前是否为低谷时段
func (m *SmartSleepManager) isQuietPeriod(diskID string, now time.Time) bool {
	pattern := m.patterns[diskID]
	if pattern == nil || pattern.TotalAccessCount == 0 {
		// 无历史数据，默认认为是低谷
		return true
	}

	hour := now.Hour()
	for _, qh := range pattern.QuietHours {
		if qh == hour {
			return true
		}
	}

	// 检查频率是否低于阈值
	if pattern.HourlyFrequency[hour] < m.config.QuietHourThreshold {
		return true
	}

	return false
}

// hasRecentAccess 检查是否有近期访问
func (m *SmartSleepManager) hasRecentAccess(diskID string, minIdle time.Duration) bool {
	pattern := m.patterns[diskID]
	if pattern == nil {
		return false
	}

	return time.Since(pattern.LastAccess) < minIdle
}

// findNextPeakHour 查找下一个高峰时段
func (m *SmartSleepManager) findNextPeakHour(pattern *AccessPattern, from time.Time) *time.Time {
	if len(pattern.PeakHours) == 0 {
		return nil
	}

	currentHour := from.Hour()

	// 查找今天剩余的高峰时段
	for _, peakHour := range pattern.PeakHours {
		if peakHour > currentHour {
			t := time.Date(from.Year(), from.Month(), from.Day(), peakHour, 0, 0, 0, from.Location())
			return &t
		}
	}

	// 找明天的第一个高峰时段
	if len(pattern.PeakHours) > 0 {
		// 排序查找最小的高峰小时
		minHour := 24
		for _, h := range pattern.PeakHours {
			if h < minHour {
				minHour = h
			}
		}
		t := time.Date(from.Year(), from.Month(), from.Day()+1, minHour, 0, 0, 0, from.Location())
		return &t
	}

	return nil
}

// ==================== 查询接口 ====================

// GetTemperatureInfo 获取磁盘温度信息
func (m *SmartSleepManager) GetTemperatureInfo(diskID string) *DiskTemperatureInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info := m.temperatures[diskID]
	if info == nil {
		return nil
	}
	cp := *info
	return &cp
}

// GetServiceDependencies 获取磁盘服务依赖
func (m *SmartSleepManager) GetServiceDependencies(diskID string) []ServiceDependency {
	m.mu.RLock()
	defer m.mu.RUnlock()
	deps := m.dependencies[diskID]
	result := make([]ServiceDependency, len(deps))
	copy(result, deps)
	return result
}

// GetSchedule 获取磁盘调度表
func (m *SmartSleepManager) GetSchedule(diskID string) *SleepSchedule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.schedules[diskID]
}

// GetConfig 获取配置
func (m *SmartSleepManager) GetConfig() SmartSleepConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// NewSmartSleepPolicy 创建智能休眠策略
func NewSmartSleepPolicy(id, name string, idleThreshold, standbyThreshold, sleepThreshold time.Duration) *SmartSleepPolicy {
	return &SmartSleepPolicy{
		SleepPolicy: &SleepPolicy{
			ID:               id,
			Name:             name,
			IdleThreshold:    idleThreshold,
			StandbyThreshold: standbyThreshold,
			SleepThreshold:   sleepThreshold,
			Enabled:          true,
		},
		TemperatureThreshold: TemperatureThreshold{
			WarningTemp:  45.0,
			ThrottleTemp: 50.0,
			CriticalTemp: 55.0,
		},
		PredictiveWake:       true,
		PredictiveWakeWindow: 15 * time.Minute,
		ServiceCheckEnabled:  true,
		MinIdleBeforeSleep:   5 * time.Minute,
	}
}

// DefaultSleepSchedule 默认工作日/周末调度表
func DefaultSleepSchedule(workdayPolicyID, weekendPolicyID string) *SleepSchedule {
	return &SleepSchedule{
		WorkdayPolicyID: workdayPolicyID,
		WeekendPolicyID: weekendPolicyID,
		Enabled:         true,
	}
}
