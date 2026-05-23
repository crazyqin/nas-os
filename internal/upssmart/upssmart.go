// Package upssmart 提供 UPS 智能管理功能
// 与 internal/ups 包互补，专注于多 UPS 支持、智能监控和电源事件管理
package upssmart

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// UPSProtocol 定义 UPS 通信协议类型
type UPSProtocol string

const (
	ProtocolUSBHID UPSProtocol = "usbhid" // USB HID 协议
	ProtocolSNMP   UPSProtocol = "snmp"   // SNMP 协议
	ProtocolNUT    UPSProtocol = "nut"    // NUT (Network UPS Tools) 协议
)

// UPSRole 定义 UPS 角色
type UPSRole string

const (
	RolePrimary   UPSRole = "primary"   // 主 UPS
	RoleSecondary UPSRole = "secondary" // 从 UPS
)

// PowerEvent 电源事件类型
type PowerEvent string

const (
	EventPowerOut      PowerEvent = "power_out"      // 停电
	EventPowerRestore  PowerEvent = "power_restore"  // 来电
	EventVoltageSag    PowerEvent = "voltage_sag"     // 电压过低
	EventVoltageSwell  PowerEvent = "voltage_swell"   // 电压过高
	EventFreqDeviation PowerEvent = "freq_deviation"  // 频率异常
	EventBatteryLow    PowerEvent = "battery_low"     // 电池电量低
	EventOverload      PowerEvent = "overload"        // 过载
	EventFault         PowerEvent = "fault"           // 故障
)

// UPSDevice 表示单个 UPS 设备
type UPSDevice struct {
	ID              string        `json:"id"`               // 设备唯一标识
	Name            string        `json:"name"`             // 设备名称
	Model           string        `json:"model"`            // 设备型号
	SerialNumber    string        `json:"serial_number"`    // 序列号
	Protocol        UPSProtocol   `json:"protocol"`         // 通信协议
	Address         string        `json:"address"`          // 设备地址（端口或IP）
	Role            UPSRole       `json:"role"`             // 角色（主/从）
	Status          UPSStatus     `json:"status"`           // 当前状态
	HealthScore     int           `json:"health_score"`     // 健康评分 0-100
	LastTestTime    time.Time     `json:"last_test_time"`   // 上次测试时间
	LastTestResult  string        `json:"last_test_result"` // 上次测试结果
	InstalledDate   time.Time     `json:"installed_date"`   // 安装日期
	BatteryAge      int           `json:"battery_age"`      // 电池年龄（月）
}

// UPSStatus UPS 实时状态
type UPSStatus struct {
	BatteryLevel    int           `json:"battery_level"`    // 电池电量 0-100%
	BatteryVoltage  float64       `json:"battery_voltage"`  // 电池电压 V
	LoadPercent     int           `json:"load_percent"`     // 负载百分比 0-100%
	InputVoltage    float64       `json:"input_voltage"`    // 输入电压 V
	OutputVoltage   float64       `json:"output_voltage"`   // 输出电压 V
	InputFrequency  float64       `json:"input_frequency"`  // 输入频率 Hz
	Temperature     float64       `json:"temperature"`      // 温度 ℃
	RuntimeLeft     time.Duration `json:"runtime_left"`     // 剩余运行时间
	OnBattery       bool          `json:"on_battery"`       // 是否使用电池
	Charging        bool          `json:"charging"`         // 是否充电中
	BatteryHealthy  bool          `json:"battery_healthy"`  // 电池健康状态
	Overloaded      bool          `json:"overloaded"`       // 是否过载
	LastUpdated     time.Time     `json:"last_updated"`     // 最后更新时间
}

// PowerEventRecord 电源事件记录
type PowerEventRecord struct {
	ID        string      `json:"id"`
	UPSID     string      `json:"ups_id"`
	Event     PowerEvent  `json:"event"`
	Timestamp time.Time   `json:"timestamp"`
	Details   string      `json:"details"`
	Resolved  bool        `json:"resolved"`
	ResovledAt time.Time  `json:"resolved_at,omitempty"`
}

// UPSManagerConfig UPS 管理器配置
type UPSManagerConfig struct {
	PollInterval     time.Duration `json:"poll_interval"`      // 轮询间隔
	EventRetention   time.Duration `json:"event_retention"`    // 事件保留时长
	EnableAutoTest   bool          `json:"enable_auto_test"`   // 启用自动测试
	AutoTestInterval time.Duration `json:"auto_test_interval"` // 自动测试间隔
}

// DefaultUPSManagerConfig 返回默认配置
func DefaultUPSManagerConfig() UPSManagerConfig {
	return UPSManagerConfig{
		PollInterval:     10 * time.Second,
		EventRetention:   30 * 24 * time.Hour, // 30天
		EnableAutoTest:   true,
		AutoTestInterval: 7 * 24 * time.Hour, // 每周
	}
}

// UPSManager UPS 智能管理器
type UPSManager struct {
	mu          sync.RWMutex
	config      UPSManagerConfig
	devices     map[string]*UPSDevice  // 设备列表，key 为设备 ID
	events      []PowerEventRecord     // 电源事件记录
	eventCh     chan PowerEventRecord   // 事件通知通道
	stopCh      chan struct{}
	running     bool
	onEvent     func(PowerEventRecord) // 事件回调
}

// NewUPSManager 创建新的 UPS 管理器
func NewUPSManager(config UPSManagerConfig) *UPSManager {
	return &UPSManager{
		config:  config,
		devices: make(map[string]*UPSDevice),
		events:  make([]PowerEventRecord, 0),
		eventCh: make(chan PowerEventRecord, 100),
		stopCh:  make(chan struct{}),
	}
}

// RegisterEventCallback 注册电源事件回调
func (m *UPSManager) RegisterEventCallback(fn func(PowerEventRecord)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onEvent = fn
}

// AddDevice 添加 UPS 设备
func (m *UPSManager) AddDevice(device *UPSDevice) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if device.ID == "" {
		return fmt.Errorf("设备 ID 不能为空")
	}

	if _, exists := m.devices[device.ID]; exists {
		return fmt.Errorf("设备 %s 已存在", device.ID)
	}

	// 如果是第一个设备，设为主 UPS
	if len(m.devices) == 0 {
		device.Role = RolePrimary
	}

	m.devices[device.ID] = device
	log.Printf("✅ 添加 UPS 设备: %s (%s)", device.Name, device.ID)
	return nil
}

// RemoveDevice 移除 UPS 设备
func (m *UPSManager) RemoveDevice(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.devices[deviceID]; !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}

	delete(m.devices, deviceID)
	log.Printf("✅ 移除 UPS 设备: %s", deviceID)
	return nil
}

// GetDevice 获取指定设备
func (m *UPSManager) GetDevice(deviceID string) (*UPSDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, exists := m.devices[deviceID]
	if !exists {
		return nil, fmt.Errorf("设备 %s 不存在", deviceID)
	}

	return device, nil
}

// GetAllDevices 获取所有设备
func (m *UPSManager) GetAllDevices() []*UPSDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*UPSDevice, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, d)
	}
	return devices
}

// GetPrimaryUPS 获取主 UPS
func (m *UPSManager) GetPrimaryUPS() (*UPSDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, d := range m.devices {
		if d.Role == RolePrimary {
			return d, nil
		}
	}

	return nil, fmt.Errorf("未找到主 UPS 设备")
}

// GetEvents 获取电源事件历史
func (m *UPSManager) GetEvents(limit int, eventFilter *PowerEvent) []PowerEventRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := make([]PowerEventRecord, 0)

	// 从最新的开始
	for i := len(m.events) - 1; i >= 0; i-- {
		if eventFilter != nil && m.events[i].Event != *eventFilter {
			continue
		}
		events = append(events, m.events[i])
		if limit > 0 && len(events) >= limit {
			break
		}
	}

	return events
}

// Start 启动 UPS 管理器
func (m *UPSManager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	// 启动轮询协程
	go m.pollLoop()

	// 启动事件处理协程
	go m.eventLoop()

	// 启动自动测试协程
	if m.config.EnableAutoTest {
		go m.autoTestLoop()
	}

	// 启动事件清理协程
	go m.cleanupLoop()

	log.Println("✅ UPS 智能管理器已启动")
}

// Stop 停止 UPS 管理器
func (m *UPSManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	close(m.stopCh)
	m.running = false
	log.Println("UPS 智能管理器已停止")
}

// pollLoop 轮询所有 UPS 设备状态
func (m *UPSManager) pollLoop() {
	ticker := time.NewTicker(m.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.pollAllDevices()
		}
	}
}

// eventLoop 处理电源事件
func (m *UPSManager) eventLoop() {
	for {
		select {
		case <-m.stopCh:
			return
		case event := <-m.eventCh:
			m.handleEvent(event)
		}
	}
}

// autoTestLoop 自动电池测试
func (m *UPSManager) autoTestLoop() {
	ticker := time.NewTicker(m.config.AutoTestInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.runAutoTest()
		}
	}
}

// cleanupLoop 清理过期事件
func (m *UPSManager) cleanupLoop() {
	ticker := time.NewTicker(24 * time.Hour) // 每天清理一次
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.cleanupEvents()
		}
	}
}

// pollAllDevices 轮询所有设备状态
func (m *UPSManager) pollAllDevices() {
	m.mu.RLock()
	deviceIDs := make([]string, 0, len(m.devices))
	for id := range m.devices {
		deviceIDs = append(deviceIDs, id)
	}
	m.mu.RUnlock()

	for _, id := range deviceIDs {
		m.pollDevice(id)
	}
}

// pollDevice 轮询单个设备状态（模拟实现）
func (m *UPSManager) pollDevice(deviceID string) {
	m.mu.Lock()
	device, exists := m.devices[deviceID]
	if !exists {
		m.mu.Unlock()
		return
	}

	// 模拟状态更新（实际应通过协议获取）
	device.Status = UPSStatus{
		BatteryLevel:   95,
		BatteryVoltage: 27.5,
		LoadPercent:    35,
		InputVoltage:   220.5,
		OutputVoltage:  220.1,
		InputFrequency: 50.0,
		Temperature:    28.5,
		RuntimeLeft:    45 * time.Minute,
		OnBattery:      false,
		Charging:       false,
		BatteryHealthy: true,
		Overloaded:     false,
		LastUpdated:    time.Now(),
	}
	m.mu.Unlock()

	// 检测电源事件
	m.detectEvents(deviceID)
}

// detectEvents 检测电源事件
func (m *UPSManager) detectEvents(deviceID string) {
	m.mu.RLock()
	device := m.devices[deviceID]
	m.mu.RUnlock()

	if device == nil {
		return
	}

	status := device.Status

	// 检测停电
	if status.OnBattery {
		m.emitEvent(deviceID, EventPowerOut, "检测到停电，切换到电池供电")
	}

	// 检测电压异常
	if status.InputVoltage < 200 {
		m.emitEvent(deviceID, EventVoltageSag, fmt.Sprintf("输入电压过低: %.1fV", status.InputVoltage))
	} else if status.InputVoltage > 240 {
		m.emitEvent(deviceID, EventVoltageSwell, fmt.Sprintf("输入电压过高: %.1fV", status.InputVoltage))
	}

	// 检测频率异常
	if status.InputFrequency < 49 || status.InputFrequency > 51 {
		m.emitEvent(deviceID, EventFreqDeviation, fmt.Sprintf("输入频率异常: %.1fHz", status.InputFrequency))
	}

	// 检测电量低
	if status.BatteryLevel < 20 {
		m.emitEvent(deviceID, EventBatteryLow, fmt.Sprintf("电池电量低: %d%%", status.BatteryLevel))
	}

	// 检测过载
	if status.LoadPercent > 90 {
		m.emitEvent(deviceID, EventOverload, fmt.Sprintf("负载过高: %d%%", status.LoadPercent))
	}
}

// emitEvent 发送电源事件
func (m *UPSManager) emitEvent(upsID string, event PowerEvent, details string) {
	record := PowerEventRecord{
		ID:        fmt.Sprintf("%s-%s-%d", upsID, event, time.Now().UnixNano()),
		UPSID:     upsID,
		Event:     event,
		Timestamp: time.Now(),
		Details:   details,
		Resolved:  false,
	}

	select {
	case m.eventCh <- record:
	default:
		log.Printf("⚠️ 事件队列已满，丢弃事件: %s", event)
	}
}

// handleEvent 处理电源事件
func (m *UPSManager) handleEvent(record PowerEventRecord) {
	m.mu.Lock()
	m.events = append(m.events, record)
	onEvent := m.onEvent
	m.mu.Unlock()

	log.Printf("⚡ 电源事件: [%s] %s - %s", record.UPSID, record.Event, record.Details)

	// 调用回调
	if onEvent != nil {
		go onEvent(record)
	}
}

// runAutoTest 运行自动电池测试
func (m *UPSManager) runAutoTest() {
	m.mu.RLock()
	deviceIDs := make([]string, 0, len(m.devices))
	for id := range m.devices {
		deviceIDs = append(deviceIDs, id)
	}
	m.mu.RUnlock()

	for _, id := range deviceIDs {
		m.testBattery(id)
	}
}

// testBattery 测试电池
func (m *UPSManager) testBattery(deviceID string) {
	m.mu.Lock()
	device, exists := m.devices[deviceID]
	if !exists {
		m.mu.Unlock()
		return
	}

	// 模拟测试（实际应发送测试命令）
	log.Printf("🔋 执行电池测试: %s", device.Name)

	device.LastTestTime = time.Now()
	device.LastTestResult = "passed"
	device.HealthScore = 85 // 模拟健康评分
	m.mu.Unlock()
}

// cleanupEvents 清理过期事件
func (m *UPSManager) cleanupEvents() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-m.config.EventRetention)
	filtered := make([]PowerEventRecord, 0)

	for _, event := range m.events {
		if event.Timestamp.After(cutoff) {
			filtered = append(filtered, event)
		}
	}

	removed := len(m.events) - len(filtered)
	m.events = filtered

	if removed > 0 {
		log.Printf("✅ 清理 %d 条过期电源事件", removed)
	}
}

// TriggerBatteryTest 手动触发电池测试
func (m *UPSManager) TriggerBatteryTest(deviceID string) error {
	m.mu.RLock()
	_, exists := m.devices[deviceID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}

	go m.testBattery(deviceID)
	return nil
}

// String 返回管理器摘要
func (m *UPSManager) String() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return fmt.Sprintf("UPSManager[devices=%d, events=%d, running=%v]",
		len(m.devices), len(m.events), m.running)
}
