// Package powerevent 电源事件管理模块
// 负责定时开关机、UPS事件处理、电源故障恢复、WOL唤醒等功能
package powerevent

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// PowerEventType 电源事件类型
type PowerEventType string

const (
	PowerEventPowerOn     PowerEventType = "power_on"
	PowerEventPowerOff    PowerEventType = "power_off"
	PowerEventRestart     PowerEventType = "restart"
	PowerEventUPSOnBattery PowerEventType = "ups_on_battery"
	PowerEventUPSOnLine   PowerEventType = "ups_on_line"
	PowerEventUPSLowBattery PowerEventType = "ups_low_battery"
	PowerEventUPSShutdown  PowerEventType = "ups_shutdown"
	PowerEventWOL         PowerEventType = "wol"
	PowerEventScheduled    PowerEventType = "scheduled"
)

// PowerEventState 电源事件状态
type PowerEventState string

const (
	StatePending   PowerEventState = "pending"
	StateRunning   PowerEventState = "running"
	StateCompleted PowerEventState = "completed"
	StateFailed    PowerEventState = "failed"
	StateCancelled PowerEventState = "cancelled"
)

// ShutdownPolicy 关机策略类型
type ShutdownPolicy string

const (
	ShutdownPolicyGraceful  ShutdownPolicy = "graceful"  // 优雅关机
	ShutdownPolicyImmediate ShutdownPolicy = "immediate" // 立即关机
	ShutdownPolicyDelayed   ShutdownPolicy = "delayed"   // 延迟关机
)

// UPSStatus UPS状态
type UPSStatus struct {
	Online        bool      `json:"online"`
	BatteryLevel  int       `json:"battery_level"`  // 0-100
	BatteryHealth string    `json:"battery_health"` // good, replace, unknown
	InputVoltage  float64   `json:"input_voltage"`
	OutputVoltage float64   `json:"output_voltage"`
	LoadPercent   int       `json:"load_percent"`
	Temperature   float64   `json:"temperature"`
	EstimatedMin  int       `json:"estimated_minutes"` // 预计剩余分钟数
	LastUpdated   time.Time `json:"last_updated"`
}

// PowerEvent 电源事件
type PowerEvent struct {
	ID          string         `json:"id"`
	Type        PowerEventType `json:"type"`
	State       PowerEventState `json:"state"`
	ScheduledAt *time.Time     `json:"scheduled_at,omitempty"`
	ExecutedAt  *time.Time     `json:"executed_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	TargetMAC   string         `json:"target_mac,omitempty"`   // WOL目标MAC
	TargetIP    string         `json:"target_ip,omitempty"`    // WOL目标IP
	Message     string         `json:"message"`
	Error       string         `json:"error,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// PowerSchedule 电源调度
type PowerSchedule struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Enabled     bool           `json:"enabled"`
	EventType   PowerEventType `json:"event_type"`
	CronExpr    string         `json:"cron_expr"`    // cron表达式
	TargetMAC   string         `json:"target_mac,omitempty"`
	TargetIP    string         `json:"target_ip,omitempty"`
	ShutdownPolicy ShutdownPolicy `json:"shutdown_policy,omitempty"`
	DelaySeconds int           `json:"delay_seconds,omitempty"` // 延迟关机秒数
	LastRun     *time.Time     `json:"last_run,omitempty"`
	NextRun     *time.Time     `json:"next_run,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// Config 电源事件管理器配置
type Config struct {
	LowBatteryThreshold   int           `json:"low_battery_threshold"`   // 低电量阈值(默认20%)
	CriticalBatteryThreshold int        `json:"critical_battery_threshold"` // 临界电量阈值(默认10%)
	UPSCheckInterval      time.Duration `json:"ups_check_interval"`      // UPS检查间隔
	ShutdownDelay         time.Duration `json:"shutdown_delay"`          // 关机延迟
	WOLRetryCount         int           `json:"wol_retry_count"`         // WOL重试次数
	WOLRetryInterval      time.Duration `json:"wol_retry_interval"`      // WOL重试间隔
	MaxHistorySize        int           `json:"max_history_size"`        // 最大历史记录数
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		LowBatteryThreshold:    20,
		CriticalBatteryThreshold: 10,
		UPSCheckInterval:      30 * time.Second,
		ShutdownDelay:         5 * time.Minute,
		WOLRetryCount:         3,
		WOLRetryInterval:      5 * time.Second,
		MaxHistorySize:        1000,
	}
}

// Manager 电源事件管理器
type Manager struct {
	config      Config
	logger      *zap.Logger
	mu          sync.RWMutex
	events      []PowerEvent
	schedules   map[string]*PowerSchedule
	upsStatus   UPSStatus
	policy      ShutdownPolicy
	cancelFunc  context.CancelFunc
	eventChan   chan PowerEvent
}

// NewManager 创建电源事件管理器
func NewManager(config Config, logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		config:    config,
		logger:    logger,
		schedules: make(map[string]*PowerSchedule),
		policy:    ShutdownPolicyGraceful,
		eventChan: make(chan PowerEvent, 100),
	}
}

// Start 启动电源事件管理器
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	m.cancelFunc = cancel

	m.logger.Info("电源事件管理器启动")

	// 启动UPS监控
	go m.monitorUPS(ctx)

	// 启动事件处理器
	go m.processEvents(ctx)

	return nil
}

// Stop 停止电源事件管理器
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancelFunc != nil {
		m.cancelFunc()
		m.logger.Info("电源事件管理器停止")
	}
}

// SchedulePowerOn 定时开机
func (m *Manager) SchedulePowerOn(ctx context.Context, scheduledAt time.Time, targetMAC, targetIP string) (*PowerEvent, error) {
	if targetMAC == "" {
		return nil, fmt.Errorf("目标MAC地址不能为空")
	}

	event := &PowerEvent{
		ID:          uuid.New().String(),
		Type:        PowerEventPowerOn,
		State:       StatePending,
		ScheduledAt: &scheduledAt,
		TargetMAC:   targetMAC,
		TargetIP:    targetIP,
		Message:     fmt.Sprintf("定时开机: %s -> %s", targetMAC, targetIP),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.addEvent(*event)

	// 设置定时器
	go func() {
		timer := time.NewTimer(time.Until(scheduledAt))
		defer timer.Stop()

		select {
		case <-timer.C:
			if err := m.TriggerWakeOnLan(ctx, targetMAC, targetIP); err != nil {
				m.updateEventState(event.ID, StateFailed, err.Error())
				m.logger.Error("定时开机失败", zap.Error(err))
			} else {
				m.updateEventState(event.ID, StateCompleted, "")
				m.logger.Info("定时开机成功", zap.String("mac", targetMAC))
			}
		case <-ctx.Done():
			m.updateEventState(event.ID, StateCancelled, "上下文取消")
		}
	}()

	return event, nil
}

// SchedulePowerOff 定时关机
func (m *Manager) SchedulePowerOff(ctx context.Context, scheduledAt time.Time, policy ShutdownPolicy, delaySeconds int) (*PowerEvent, error) {
	if policy == "" {
		policy = m.policy
	}

	event := &PowerEvent{
		ID:          uuid.New().String(),
		Type:        PowerEventPowerOff,
		State:       StatePending,
		ScheduledAt: &scheduledAt,
		Message:     fmt.Sprintf("定时关机: 策略=%s, 延迟=%ds", policy, delaySeconds),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.addEvent(*event)

	go func() {
		timer := time.NewTimer(time.Until(scheduledAt))
		defer timer.Stop()

		select {
		case <-timer.C:
			if err := m.executePowerOff(ctx, policy, delaySeconds); err != nil {
				m.updateEventState(event.ID, StateFailed, err.Error())
				m.logger.Error("定时关机失败", zap.Error(err))
			} else {
				m.updateEventState(event.ID, StateCompleted, "")
				m.logger.Info("定时关机完成")
			}
		case <-ctx.Done():
			m.updateEventState(event.ID, StateCancelled, "上下文取消")
		}
	}()

	return event, nil
}

// ScheduleRestart 定时重启
func (m *Manager) ScheduleRestart(ctx context.Context, scheduledAt time.Time, policy ShutdownPolicy, delaySeconds int) (*PowerEvent, error) {
	if policy == "" {
		policy = m.policy
	}

	event := &PowerEvent{
		ID:          uuid.New().String(),
		Type:        PowerEventRestart,
		State:       StatePending,
		ScheduledAt: &scheduledAt,
		Message:     fmt.Sprintf("定时重启: 策略=%s, 延迟=%ds", policy, delaySeconds),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.addEvent(*event)

	go func() {
		timer := time.NewTimer(time.Until(scheduledAt))
		defer timer.Stop()

		select {
		case <-timer.C:
			if err := m.executeRestart(ctx, policy, delaySeconds); err != nil {
				m.updateEventState(event.ID, StateFailed, err.Error())
				m.logger.Error("定时重启失败", zap.Error(err))
			} else {
				m.updateEventState(event.ID, StateCompleted, "")
				m.logger.Info("定时重启完成")
			}
		case <-ctx.Done():
			m.updateEventState(event.ID, StateCancelled, "上下文取消")
		}
	}()

	return event, nil
}

// HandleUPSEvent 处理UPS事件
func (m *Manager) HandleUPSEvent(ctx context.Context, eventType PowerEventType, upsStatus UPSStatus) error {
	m.mu.Lock()
	m.upsStatus = upsStatus
	m.mu.Unlock()

	event := &PowerEvent{
		ID:        uuid.New().String(),
		Type:      eventType,
		State:     StateRunning,
		Message:   fmt.Sprintf("UPS事件: %s, 电量=%d%%", eventType, upsStatus.BatteryLevel),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.addEvent(*event)

	switch eventType {
	case PowerEventUPSOnBattery:
		m.logger.Warn("UPS切换到电池供电", zap.Int("battery_level", upsStatus.BatteryLevel))
		// 检查是否需要低电量保护
		if upsStatus.BatteryLevel <= m.config.CriticalBatteryThreshold {
			return m.handleCriticalBattery(ctx, event, upsStatus)
		}
		if upsStatus.BatteryLevel <= m.config.LowBatteryThreshold {
			return m.handleLowBattery(ctx, event, upsStatus)
		}

	case PowerEventUPSOnLine:
		m.logger.Info("UPS恢复市电供电")
		m.updateEventState(event.ID, StateCompleted, "市电恢复")

	case PowerEventUPSLowBattery:
		return m.handleCriticalBattery(ctx, event, upsStatus)

	case PowerEventUPSShutdown:
		m.logger.Warn("UPS请求关机")
		if err := m.executePowerOff(ctx, ShutdownPolicyGraceful, 0); err != nil {
			m.updateEventState(event.ID, StateFailed, err.Error())
			return err
		}
		m.updateEventState(event.ID, StateCompleted, "UPS关机完成")
	}

	return nil
}

// SetShutdownPolicy 设置关机策略
func (m *Manager) SetShutdownPolicy(policy ShutdownPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policy = policy
	m.logger.Info("设置关机策略", zap.String("policy", string(policy)))
}

// GetShutdownPolicy 获取当前关机策略
func (m *Manager) GetShutdownPolicy() ShutdownPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.policy
}

// GetPowerHistory 获取电源事件历史
func (m *Manager) GetPowerHistory(limit int) []PowerEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.events) {
		limit = len(m.events)
	}

	// 返回最新的事件（倒序）
	result := make([]PowerEvent, limit)
	for i := 0; i < limit; i++ {
		result[i] = m.events[len(m.events)-1-i]
	}
	return result
}

// TriggerWakeOnLan 触发WOL唤醒
func (m *Manager) TriggerWakeOnLan(ctx context.Context, targetMAC, targetIP string) error {
	mac, err := net.ParseMAC(targetMAC)
	if err != nil {
		return fmt.Errorf("无效的MAC地址: %w", err)
	}

	// 构建魔术包
	magicPacket := buildMagicPacket(mac)

	// 发送WOL包
	addr := &net.UDPAddr{
		IP:   net.ParseIP(targetIP),
		Port: 9,
	}

	if addr.IP == nil {
		// 默认广播
		addr.IP = net.IPv4bcast
	}

	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return fmt.Errorf("创建UDP连接失败: %w", err)
	}
	defer conn.Close()

	// 重试发送
	for i := 0; i < m.config.WOLRetryCount; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if _, err := conn.Write(magicPacket); err != nil {
			m.logger.Warn("WOL发送失败", zap.Int("attempt", i+1), zap.Error(err))
			if i < m.config.WOLRetryCount-1 {
				time.Sleep(m.config.WOLRetryInterval)
				continue
			}
			return fmt.Errorf("WOL发送失败: %w", err)
		}

		m.logger.Info("WOL魔术包已发送",
			zap.String("mac", targetMAC),
			zap.String("ip", targetIP),
			zap.Int("attempt", i+1),
		)

		if i < m.config.WOLRetryCount-1 {
			time.Sleep(m.config.WOLRetryInterval)
		}
	}

	// 记录事件
	event := PowerEvent{
		ID:        uuid.New().String(),
		Type:      PowerEventWOL,
		State:     StateCompleted,
		TargetMAC: targetMAC,
		TargetIP:  targetIP,
		Message:   fmt.Sprintf("WOL唤醒: %s -> %s", targetMAC, targetIP),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.addEvent(event)

	return nil
}

// CheckBatteryStatus 检查电池状态
func (m *Manager) CheckBatteryStatus() UPSStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.upsStatus
}

// UpdateUPSStatus 更新UPS状态
func (m *Manager) UpdateUPSStatus(status UPSStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	status.LastUpdated = time.Now()
	m.upsStatus = status
}

// AddSchedule 添加电源调度
func (m *Manager) AddSchedule(schedule *PowerSchedule) error {
	if schedule.ID == "" {
		schedule.ID = uuid.New().String()
	}
	schedule.CreatedAt = time.Now()
	schedule.UpdatedAt = time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()
	m.schedules[schedule.ID] = schedule
	m.logger.Info("添加电源调度", zap.String("id", schedule.ID), zap.String("name", schedule.Name))
	return nil
}

// RemoveSchedule 移除电源调度
func (m *Manager) RemoveSchedule(scheduleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.schedules[scheduleID]; !ok {
		return fmt.Errorf("调度不存在: %s", scheduleID)
	}

	delete(m.schedules, scheduleID)
	m.logger.Info("移除电源调度", zap.String("id", scheduleID))
	return nil
}

// GetSchedules 获取所有电源调度
func (m *Manager) GetSchedules() []*PowerSchedule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	schedules := make([]*PowerSchedule, 0, len(m.schedules))
	for _, s := range m.schedules {
		schedules = append(schedules, s)
	}
	return schedules
}

// UpdateSchedule 更新电源调度
func (m *Manager) UpdateSchedule(schedule *PowerSchedule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.schedules[schedule.ID]; !ok {
		return fmt.Errorf("调度不存在: %s", schedule.ID)
	}

	schedule.UpdatedAt = time.Now()
	m.schedules[schedule.ID] = schedule
	return nil
}

// 内部方法

func (m *Manager) addEvent(event PowerEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 限制历史记录大小
	if len(m.events) >= m.config.MaxHistorySize {
		m.events = m.events[1:]
	}
	m.events = append(m.events, event)

	// 发送到事件通道
	select {
	case m.eventChan <- event:
	default:
		m.logger.Warn("事件通道已满，丢弃事件")
	}
}

func (m *Manager) updateEventState(eventID string, state PowerEventState, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.events {
		if m.events[i].ID == eventID {
			m.events[i].State = state
			m.events[i].Error = errMsg
			now := time.Now()
			m.events[i].UpdatedAt = now
			if state == StateCompleted || state == StateFailed {
				m.events[i].CompletedAt = &now
			}
			return
		}
	}
}

func (m *Manager) handleLowBattery(ctx context.Context, event *PowerEvent, upsStatus UPSStatus) error {
	m.logger.Warn("UPS低电量保护",
		zap.Int("battery_level", upsStatus.BatteryLevel),
		zap.Int("threshold", m.config.LowBatteryThreshold),
	)

	// 发送警告，但不立即关机
	m.updateEventState(event.ID, StateCompleted, "低电量警告已发送")
	return nil
}

func (m *Manager) handleCriticalBattery(ctx context.Context, event *PowerEvent, upsStatus UPSStatus) error {
	m.logger.Error("UPS临界电量，准备关机",
		zap.Int("battery_level", upsStatus.BatteryLevel),
		zap.Int("threshold", m.config.CriticalBatteryThreshold),
		zap.Int("estimated_minutes", upsStatus.EstimatedMin),
	)

	// 执行优雅关机
	if err := m.executePowerOff(ctx, ShutdownPolicyGraceful, 30); err != nil {
		m.updateEventState(event.ID, StateFailed, err.Error())
		return err
	}

	m.updateEventState(event.ID, StateCompleted, "临界电量保护关机完成")
	return nil
}

func (m *Manager) executePowerOff(ctx context.Context, policy ShutdownPolicy, delaySeconds int) error {
	m.logger.Info("执行关机",
		zap.String("policy", string(policy)),
		zap.Int("delay_seconds", delaySeconds),
	)

	if delaySeconds > 0 {
		timer := time.NewTimer(time.Duration(delaySeconds) * time.Second)
		defer timer.Stop()

		select {
		case <-timer.C:
			// 继续执行关机
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// 实际关机操作（在真实系统中调用shutdown命令）
	// 这里只是模拟
	m.logger.Info("系统关机执行完成")
	return nil
}

func (m *Manager) executeRestart(ctx context.Context, policy ShutdownPolicy, delaySeconds int) error {
	m.logger.Info("执行重启",
		zap.String("policy", string(policy)),
		zap.Int("delay_seconds", delaySeconds),
	)

	if delaySeconds > 0 {
		timer := time.NewTimer(time.Duration(delaySeconds) * time.Second)
		defer timer.Stop()

		select {
		case <-timer.C:
			// 继续执行重启
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// 实际重启操作
	m.logger.Info("系统重启执行完成")
	return nil
}

func (m *Manager) monitorUPS(ctx context.Context) {
	ticker := time.NewTicker(m.config.UPSCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkUPSBattery(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (m *Manager) checkUPSBattery(ctx context.Context) {
	status := m.CheckBatteryStatus()

	// 检查电量级别
	if status.BatteryLevel <= m.config.CriticalBatteryThreshold && status.Online {
		m.HandleUPSEvent(ctx, PowerEventUPSLowBattery, status)
	} else if status.BatteryLevel <= m.config.LowBatteryThreshold && status.Online {
		// 低电量警告（但不触发关机）
		m.logger.Warn("UPS电量低",
			zap.Int("battery_level", status.BatteryLevel),
		)
	}
}

func (m *Manager) processEvents(ctx context.Context) {
	for {
		select {
		case event := <-m.eventChan:
			m.logger.Debug("处理电源事件",
				zap.String("id", event.ID),
				zap.String("type", string(event.Type)),
				zap.String("state", string(event.State)),
			)
		case <-ctx.Done():
			return
		}
	}
}

// buildMagicPacket 构建WOL魔术包
// 格式: 6字节0xFF + 16次重复的MAC地址
func buildMagicPacket(mac net.HardwareAddr) []byte {
	packet := make([]byte, 6+16*6)

	// 填充6字节的0xFF
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}

	// 重复MAC地址16次
	for i := 0; i < 16; i++ {
		copy(packet[6+i*6:], mac)
	}

	return packet
}
