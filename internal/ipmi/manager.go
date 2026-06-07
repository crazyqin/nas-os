package ipmi

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// Manager IPMI 远程管理器.
type Manager struct {
	mu      sync.RWMutex
	config  IPMIConfig
	devices map[string]*IPMIDevice
	sensors map[string]*Sensor
	events  []*SystemEvent
	running bool
	stopCh  chan struct{}
}

// NewManager 创建管理器.
func NewManager(cfg IPMIConfig) *Manager {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 30 * time.Second
	}
	if cfg.EventLimit == 0 {
		cfg.EventLimit = 10000
	}
	if cfg.SensorThreshold == 0 {
		cfg.SensorThreshold = 80
	}
	return &Manager{
		config:  cfg,
		devices: make(map[string]*IPMIDevice),
		sensors: make(map[string]*Sensor),
		events:  make([]*SystemEvent, 0),
		stopCh:  make(chan struct{}),
	}
}

// Start 启动管理器.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return nil
	}
	m.running = true
	m.stopCh = make(chan struct{})
	go m.pollLoop()
	log.Println("[IPMI] IPMI 远程管理已启动")
	return nil
}

// Stop 停止.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopCh)
	log.Println("[IPMI] IPMI 远程管理已停止")
}

// ========== 设备管理 ==========

// AddDevice 添加 IPMI 设备.
func (m *Manager) AddDevice(device *IPMIDevice) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if device.ID == "" {
		return fmt.Errorf("设备 ID 不能为空")
	}
	if _, exists := m.devices[device.ID]; exists {
		return fmt.Errorf("设备 %s 已存在", device.ID)
	}
	device.Status = DeviceStatusOnline
	device.LastSeen = time.Now()
	device.CreatedAt = time.Now()
	m.devices[device.ID] = device
	log.Printf("[IPMI] 设备已添加: %s (Host: %s)", device.ID, device.Host)
	return nil
}

// RemoveDevice 移除设备.
func (m *Manager) RemoveDevice(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.devices[id]; !exists {
		return fmt.Errorf("设备 %s 不存在", id)
	}
	delete(m.devices, id)
	log.Printf("[IPMI] 设备已移除: %s", id)
	return nil
}

// GetDevice 获取设备.
func (m *Manager) GetDevice(id string) (*IPMIDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	device, exists := m.devices[id]
	if !exists {
		return nil, fmt.Errorf("设备 %s 不存在", id)
	}
	return device, nil
}

// ListDevices 列出所有设备.
func (m *Manager) ListDevices() []*IPMIDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()
	devices := make([]*IPMIDevice, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, d)
	}
	return devices
}

// ========== 电源控制 ==========

// PowerOn 开机.
func (m *Manager) PowerOn(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	device, exists := m.devices[deviceID]
	if !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}
	device.PowerState = PowerStateOn
	device.LastSeen = time.Now()
	m.addEvent(deviceID, EventTypePower, "电源开机")
	log.Printf("[IPMI] 设备 %s 已开机", deviceID)
	return nil
}

// PowerOff 关机.
func (m *Manager) PowerOff(deviceID string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	device, exists := m.devices[deviceID]
	if !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}
	if !force && device.PowerState == PowerStateOn {
		return fmt.Errorf("设备 %s 正在运行，请使用 force=true 强制关机", deviceID)
	}
	device.PowerState = PowerStateOff
	device.LastSeen = time.Now()
	m.addEvent(deviceID, EventTypePower, "电源关机")
	log.Printf("[IPMI] 设备 %s 已关机 (force=%v)", deviceID, force)
	return nil
}

// PowerCycle 重启.
func (m *Manager) PowerCycle(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	device, exists := m.devices[deviceID]
	if !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}
	device.PowerState = PowerStateOn
	device.LastSeen = time.Now()
	m.addEvent(deviceID, EventTypePower, "电源重启")
	log.Printf("[IPMI] 设备 %s 已重启", deviceID)
	return nil
}

// Reset 硬重置.
func (m *Manager) Reset(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	device, exists := m.devices[deviceID]
	if !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}
	device.PowerState = PowerStateOn
	device.LastSeen = time.Now()
	m.addEvent(deviceID, EventTypePower, "硬重置")
	log.Printf("[IPMI] 设备 %s 已硬重置", deviceID)
	return nil
}

// ========== 传感器管理 ==========

// RegisterSensor 注册传感器.
func (m *Manager) RegisterSensor(sensor *Sensor) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sensor.ID == "" {
		return fmt.Errorf("传感器 ID 不能为空")
	}
	sensor.LastReading = time.Now()
	m.sensors[sensor.ID] = sensor
	return nil
}

// GetSensor 获取传感器.
func (m *Manager) GetSensor(id string) (*Sensor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sensor, exists := m.sensors[id]
	if !exists {
		return nil, fmt.Errorf("传感器 %s 不存在", id)
	}
	return sensor, nil
}

// ListSensors 列出所有传感器.
func (m *Manager) ListSensors(deviceID string) []*Sensor {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sensors := make([]*Sensor, 0)
	for _, s := range m.sensors {
		if deviceID == "" || s.DeviceID == deviceID {
			sensors = append(sensors, s)
		}
	}
	return sensors
}

// ========== 事件管理 ==========

// GetEvents 获取系统事件.
func (m *Manager) GetEvents(deviceID string, limit int) []*SystemEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	events := make([]*SystemEvent, 0)
	for i := len(m.events) - 1; i >= 0; i-- {
		if deviceID == "" || m.events[i].DeviceID == deviceID {
			events = append(events, m.events[i])
			if limit > 0 && len(events) >= limit {
				break
			}
		}
	}
	return events
}

// ClearEvents 清除事件.
func (m *Manager) ClearEvents(deviceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if deviceID == "" {
		m.events = make([]*SystemEvent, 0)
	} else {
		filtered := make([]*SystemEvent, 0)
		for _, e := range m.events {
			if e.DeviceID != deviceID {
				filtered = append(filtered, e)
			}
		}
		m.events = filtered
	}
}

// ========== 统计 ==========

// GetStats 获取统计信息.
func (m *Manager) GetStats() *IPMIStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	online := 0
	offline := 0
	for _, d := range m.devices {
		if d.Status == DeviceStatusOnline {
			online++
		} else {
			offline++
		}
	}
	return &IPMIStats{
		TotalDevices:   len(m.devices),
		OnlineDevices:  online,
		OfflineDevices: offline,
		TotalSensors:   len(m.sensors),
		TotalEvents:    len(m.events),
	}
}

// ========== 内部方法 ==========

// pollLoop 定期轮询循环.
func (m *Manager) pollLoop() {
	ticker := time.NewTicker(m.config.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.pollDevices()
		}
	}
}

// pollDevices 轮询所有设备.
func (m *Manager) pollDevices() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, device := range m.devices {
		device.LastSeen = time.Now()
	}
}

// addEvent 添加事件.
func (m *Manager) addEvent(deviceID string, eventType EventType, message string) {
	event := &SystemEvent{
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		DeviceID:  deviceID,
		Type:      eventType,
		Message:   message,
		Timestamp: time.Now(),
	}
	m.events = append(m.events, event)
	if len(m.events) > m.config.EventLimit {
		m.events = m.events[1:]
	}
}
