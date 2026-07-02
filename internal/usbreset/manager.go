// Package usbreset 提供 USB 设备管理功能
// 设备枚举、端口控制、策略管理、热插拔事件
package usbreset

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ========== 核心类型 ==========

// DeviceType USB 设备类型.
type DeviceType string

const (
	DeviceTypeStorage DeviceType = "storage" // 存储设备
	DeviceTypeAudio   DeviceType = "audio"   // 音频设备
	DeviceTypeNetwork DeviceType = "network" // 网络设备
	DeviceTypeHID     DeviceType = "hid"     // 人机接口
	DeviceTypeHub     DeviceType = "hub"     // USB Hub
	DeviceTypeUnknown DeviceType = "unknown" // 未知设备
)

// PolicyAction 策略动作.
type PolicyAction string

const (
	PolicyAllow    PolicyAction = "allow"    // 允许
	PolicyDeny     PolicyAction = "deny"     // 拒绝
	PolicyReadOnly PolicyAction = "readonly" // 只读
)

// EventType 事件类型.
type EventType string

const (
	EventInsert EventType = "insert" // 插入
	EventRemove EventType = "remove" // 拔出
	EventReset  EventType = "reset"  // 重置
	EventError  EventType = "error"  // 错误
)

// USBDevice USB 设备信息.
type USBDevice struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	VendorID    string     `json:"vendorId"`
	ProductID   string     `json:"productId"`
	Type        DeviceType `json:"type"`
	Speed       string     `json:"speed"` // low/full/high/super/super_plus
	Port        string     `json:"port"`
	MountPoint  string     `json:"mountPoint,omitempty"`
	Connected   bool       `json:"connected"`
	ConnectedAt time.Time  `json:"connectedAt"`
}

// USBPort USB 端口.
type USBPort struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	DeviceID   string `json:"deviceId,omitempty"`
	Powered    bool   `json:"powered"`
	Speed      string `json:"speed"`
	BusNumber  int    `json:"busNumber"`
	PortNumber int    `json:"portNumber"`
}

// USBPolicy USB 策略.
type USBPolicy struct {
	ID         string       `json:"id"`
	DeviceType DeviceType   `json:"deviceType"`
	Action     PolicyAction `json:"action"`
	Priority   int          `json:"priority"`            // 优先级，数值越大越优先
	VendorID   string       `json:"vendorId,omitempty"`  // 特定厂商
	ProductID  string       `json:"productId,omitempty"` // 特定产品
	Name       string       `json:"name"`
	Enabled    bool         `json:"enabled"`
	CreatedAt  time.Time    `json:"createdAt"`
}

// USBEvent USB 事件.
type USBEvent struct {
	ID        string    `json:"id"`
	DeviceID  string    `json:"deviceId"`
	EventType EventType `json:"eventType"`
	Port      string    `json:"port,omitempty"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// USBBandwidth USB 带宽信息.
type USBBandwidth struct {
	PortID    string   `json:"portId"`
	UsedMbps  int      `json:"usedMbps"`
	MaxMbps   int      `json:"maxMbps"`
	DeviceIDs []string `json:"deviceIds,omitempty"`
}

// AutoMountPolicy 自动挂载策略.
type AutoMountPolicy struct {
	Enabled    bool   `json:"enabled"`
	Policy     string `json:"policy"` // readonly/readwrite/deny
	MountPoint string `json:"mountPoint,omitempty"`
}

// ========== Manager ==========

// Manager USB 设备管理器.
type Manager struct {
	mu           sync.RWMutex
	devices      map[string]*USBDevice
	ports        map[string]*USBPort
	policies     map[string]*USBPolicy
	events       []USBEvent
	bandwidth    map[string]*USBBandwidth
	autoMount    AutoMountPolicy
	nextPolicyID int
	nextEventID  int
}

// NewManager 创建管理器.
func NewManager() *Manager {
	m := &Manager{
		devices:      make(map[string]*USBDevice),
		ports:        make(map[string]*USBPort),
		policies:     make(map[string]*USBPolicy),
		bandwidth:    make(map[string]*USBBandwidth),
		autoMount:    AutoMountPolicy{Enabled: true, Policy: "readonly"},
		nextPolicyID: 1,
		nextEventID:  1,
	}
	m.initDefaults()
	return m
}

// initDefaults 初始化默认配置.
func (m *Manager) initDefaults() {
	// 默认 USB 端口
	m.ports["usb-1-1"] = &USBPort{
		ID: "usb-1-1", Path: "/sys/bus/usb/devices/1-1", Powered: true, Speed: "super", BusNumber: 1, PortNumber: 1,
	}
	m.ports["usb-1-2"] = &USBPort{
		ID: "usb-1-2", Path: "/sys/bus/usb/devices/1-2", Powered: true, Speed: "super", BusNumber: 1, PortNumber: 2,
	}
	m.ports["usb-2-1"] = &USBPort{
		ID: "usb-2-1", Path: "/sys/bus/usb/devices/2-1", Powered: true, Speed: "high", BusNumber: 2, PortNumber: 1,
	}
	m.ports["usb-2-2"] = &USBPort{
		ID: "usb-2-2", Path: "/sys/bus/usb/devices/2-2", Powered: true, Speed: "high", BusNumber: 2, PortNumber: 2,
	}

	// 默认带宽
	m.bandwidth["usb-1-1"] = &USBBandwidth{PortID: "usb-1-1", UsedMbps: 0, MaxMbps: 5000}
	m.bandwidth["usb-1-2"] = &USBBandwidth{PortID: "usb-1-2", UsedMbps: 0, MaxMbps: 5000}
	m.bandwidth["usb-2-1"] = &USBBandwidth{PortID: "usb-2-1", UsedMbps: 0, MaxMbps: 480}
	m.bandwidth["usb-2-2"] = &USBBandwidth{PortID: "usb-2-2", UsedMbps: 0, MaxMbps: 480}

	// 默认策略：允许存储设备，拒绝 HID
	m.policies["pol-1"] = &USBPolicy{
		ID: "pol-1", DeviceType: DeviceTypeStorage, Action: PolicyAllow, Priority: 10,
		Name: "允许存储设备", Enabled: true, CreatedAt: time.Now(),
	}
	m.policies["pol-2"] = &USBPolicy{
		ID: "pol-2", DeviceType: DeviceTypeHID, Action: PolicyAllow, Priority: 5,
		Name: "允许 HID 设备", Enabled: true, CreatedAt: time.Now(),
	}
	m.nextPolicyID = 3

	// 模拟设备
	m.devices["dev-1"] = &USBDevice{
		ID: "dev-1", Name: "USB 硬盘", VendorID: "0x0781", ProductID: "0x5567",
		Type: DeviceTypeStorage, Speed: "super", Port: "usb-1-1",
		MountPoint: "/mnt/usb1", Connected: true, ConnectedAt: time.Now().Add(-2 * time.Hour),
	}
	m.devices["dev-2"] = &USBDevice{
		ID: "dev-2", Name: "USB Hub", VendorID: "0x05e3", ProductID: "0x0610",
		Type: DeviceTypeHub, Speed: "high", Port: "usb-2-1",
		Connected: true, ConnectedAt: time.Now().Add(-1 * time.Hour),
	}
	m.devices["dev-3"] = &USBDevice{
		ID: "dev-3", Name: "无线鼠标", VendorID: "0x046d", ProductID: "0xc534",
		Type: DeviceTypeHID, Speed: "full", Port: "usb-2-2",
		Connected: true, ConnectedAt: time.Now().Add(-30 * time.Minute),
	}

	// 更新端口关联
	m.ports["usb-1-1"].DeviceID = "dev-1"
	m.ports["usb-2-1"].DeviceID = "dev-2"
	m.ports["usb-2-2"].DeviceID = "dev-3"

	// 更新带宽
	m.bandwidth["usb-1-1"].UsedMbps = 400
	m.bandwidth["usb-1-1"].DeviceIDs = []string{"dev-1"}
	m.bandwidth["usb-2-2"].UsedMbps = 1
	m.bandwidth["usb-2-2"].DeviceIDs = []string{"dev-3"}

	// 初始事件
	m.events = []USBEvent{
		{ID: "evt-1", DeviceID: "dev-1", EventType: EventInsert, Port: "usb-1-1", Message: "USB 硬盘已插入", Timestamp: time.Now().Add(-2 * time.Hour)},
		{ID: "evt-2", DeviceID: "dev-2", EventType: EventInsert, Port: "usb-2-1", Message: "USB Hub 已插入", Timestamp: time.Now().Add(-1 * time.Hour)},
		{ID: "evt-3", DeviceID: "dev-3", EventType: EventInsert, Port: "usb-2-2", Message: "无线鼠标已插入", Timestamp: time.Now().Add(-30 * time.Minute)},
	}
	m.nextEventID = 4
}

// ========== 设备管理 ==========

// ListDevices 列出所有 USB 设备.
func (m *Manager) ListDevices() []USBDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]USBDevice, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, *d)
	}
	return devices
}

// GetDevice 获取 USB 设备.
func (m *Manager) GetDevice(id string) *USBDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dev, ok := m.devices[id]
	if !ok {
		return nil
	}
	return dev
}

// ========== 重置操作 ==========

// ResetDevice 重置 USB 设备.
func (m *Manager) ResetDevice(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dev, ok := m.devices[id]
	if !ok {
		return fmt.Errorf("device %s not found", id)
	}

	if !dev.Connected {
		return fmt.Errorf("device %s not connected", id)
	}

	m.addEvent(id, EventReset, dev.Port, fmt.Sprintf("设备 %s 已重置", dev.Name))
	log.Printf("[USB] 重置设备: %s (%s)", dev.Name, id)
	return nil
}

// ResetPort 重置 USB 端口.
func (m *Manager) ResetPort(portID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	port, ok := m.ports[portID]
	if !ok {
		return fmt.Errorf("port %s not found", portID)
	}

	// 重置端口上所有设备
	for _, dev := range m.devices {
		if dev.Port == portID && dev.Connected {
			m.addEvent(dev.ID, EventReset, portID, fmt.Sprintf("端口 %s 重置，设备 %s", portID, dev.Name))
		}
	}

	log.Printf("[USB] 重置端口: %s (路径: %s)", portID, port.Path)
	return nil
}

// SetPortPower 设置端口电源.
func (m *Manager) SetPortPower(portID string, on bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	port, ok := m.ports[portID]
	if !ok {
		return fmt.Errorf("port %s not found", portID)
	}

	port.Powered = on

	state := "开启"
	if !on {
		state = "关闭"
	}
	log.Printf("[USB] 端口 %s 电源已%s", portID, state)
	return nil
}

// ListPorts 列出所有端口.
func (m *Manager) ListPorts() []USBPort {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ports := make([]USBPort, 0, len(m.ports))
	for _, p := range m.ports {
		ports = append(ports, *p)
	}
	return ports
}

// ========== 策略管理 ==========

// AddPolicy 添加策略.
func (m *Manager) AddPolicy(policy *USBPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		policy.ID = fmt.Sprintf("pol-%d", m.nextPolicyID)
		m.nextPolicyID++
	}
	policy.CreatedAt = time.Now()

	m.policies[policy.ID] = policy
	log.Printf("[USB] 添加策略: %s (%s)", policy.Name, policy.ID)
	return nil
}

// RemovePolicy 删除策略.
func (m *Manager) RemovePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[id]; !ok {
		return fmt.Errorf("policy %s not found", id)
	}

	delete(m.policies, id)
	log.Printf("[USB] 删除策略: %s", id)
	return nil
}

// ListPolicies 列出所有策略.
func (m *Manager) ListPolicies() []USBPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]USBPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, *p)
	}
	return policies
}

// CheckPolicy 检查设备策略.
func (m *Manager) CheckPolicy(deviceID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dev, ok := m.devices[deviceID]
	if !ok {
		return "", fmt.Errorf("device %s not found", deviceID)
	}

	// 按优先级排序查找匹配策略
	var bestPolicy *USBPolicy
	for _, pol := range m.policies {
		if !pol.Enabled {
			continue
		}

		// 精确匹配（厂商+产品）
		if pol.VendorID != "" && pol.ProductID != "" {
			if pol.VendorID == dev.VendorID && pol.ProductID == dev.ProductID {
				if bestPolicy == nil || pol.Priority > bestPolicy.Priority {
					bestPolicy = pol
				}
				continue
			}
		}

		// 类型匹配
		if pol.DeviceType == dev.Type {
			if bestPolicy == nil || pol.Priority > bestPolicy.Priority {
				bestPolicy = pol
			}
		}
	}

	if bestPolicy == nil {
		return string(PolicyAllow), nil // 默认允许
	}

	return string(bestPolicy.Action), nil
}

// ========== 事件管理 ==========

// addEvent 添加事件.
func (m *Manager) addEvent(deviceID string, eventType EventType, port, message string) {
	event := USBEvent{
		ID:        fmt.Sprintf("evt-%d", m.nextEventID),
		DeviceID:  deviceID,
		EventType: eventType,
		Port:      port,
		Message:   message,
		Timestamp: time.Now(),
	}
	m.events = append(m.events, event)
	m.nextEventID++
}

// GetEvents 获取事件列表.
func (m *Manager) GetEvents(since time.Time, deviceID string) []USBEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var events []USBEvent
	for _, e := range m.events {
		if e.Timestamp.Before(since) {
			continue
		}
		if deviceID != "" && e.DeviceID != deviceID {
			continue
		}
		events = append(events, e)
	}
	return events
}

// ========== 带宽监控 ==========

// GetBandwidth 获取端口带宽.
func (m *Manager) GetBandwidth(portID string) (*USBBandwidth, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bw, ok := m.bandwidth[portID]
	if !ok {
		return nil, fmt.Errorf("bandwidth info for port %s not found", portID)
	}
	return bw, nil
}

// ========== 自动挂载 ==========

// SetAutoMount 设置自动挂载策略.
func (m *Manager) SetAutoMount(enable bool, policy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy != "readonly" && policy != "readwrite" && policy != "deny" {
		return fmt.Errorf("invalid auto mount policy: %s", policy)
	}

	m.autoMount.Enabled = enable
	m.autoMount.Policy = policy
	log.Printf("[USB] 自动挂载: enabled=%v, policy=%s", enable, policy)
	return nil
}
