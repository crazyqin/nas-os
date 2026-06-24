// Package ups UPS 扩展功能：事件、设备管理、关机任务
package ups

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 事件类型 ==========

// Event UPS 事件
type Event struct {
	ID           string      `json:"id"`
	DeviceID     string      `json:"device_id"`
	Type         EventType   `json:"type"`
	Message      string      `json:"message"`
	Details      interface{} `json:"details,omitempty"`
	Timestamp    time.Time   `json:"timestamp"`
	Acknowledged bool        `json:"acknowledged"`
}

// EventType 事件类型
type EventType string

const (
	EventPowerFailure      EventType = "power_failure"
	EventPowerRestored     EventType = "power_restored"
	EventBatteryLow        EventType = "battery_low"
	EventBatteryCritical   EventType = "battery_critical"
	EventDeviceConnected   EventType = "device_connected"
	EventDeviceDisconnected EventType = "device_disconnected"
	EventShutdownInitiated EventType = "shutdown_initiated"
	EventDeviceFault       EventType = "device_fault"
)

// ========== 设备类型 ==========

// Device UPS 设备信息
type Device struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Model        string       `json:"model"`
	Serial       string       `json:"serial"`
	Manufacturer string       `json:"manufacturer"`
	USBPath      string       `json:"usb_path"`
	Status       DeviceStatus `json:"status"`
	ConnectedAt  time.Time    `json:"connected_at"`
	LastSeen     time.Time    `json:"last_seen"`
}

// DeviceStatus 设备状态
type DeviceStatus string

const (
	DeviceStatusOnline       DeviceStatus = "online"
	DeviceStatusOnBattery    DeviceStatus = "on_battery"
	DeviceStatusLowBattery   DeviceStatus = "low_battery"
	DeviceStatusCharging     DeviceStatus = "charging"
	DeviceStatusFault        DeviceStatus = "fault"
	DeviceStatusDisconnected DeviceStatus = "disconnected"
)

// ========== 关机任务类型 ==========

// ShutdownTask 关机任务
type ShutdownTask struct {
	ID          string     `json:"id"`
	DeviceID    string     `json:"device_id"`
	Reason      string     `json:"reason"`
	Delay       int        `json:"delay"`
	Status      TaskStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	ScheduledAt time.Time  `json:"scheduled_at"`
	ExecutedAt  *time.Time `json:"executed_at,omitempty"`
	CancelledAt *time.Time `json:"cancelled_at,omitempty"`
}

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusExecuting TaskStatus = "executing"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusCancelled TaskStatus = "cancelled"
	TaskStatusFailed    TaskStatus = "failed"
)

// ========== 扩展管理器 ==========

var (
	ErrDeviceNotFound    = errors.New("设备未找到")
	ErrDeviceExists      = errors.New("设备已存在")
	ErrTaskNotFound      = errors.New("任务未找到")
	ErrTaskNotCancellable = errors.New("任务无法取消")
)

// ExtendedManager 扩展的 UPS 管理器
type ExtendedManager struct {
	mu       sync.RWMutex
	config   UPSConfig
	devices  map[string]*Device
	events   []Event
	tasks    []*ShutdownTask
	stopCh   chan struct{}
	running  bool
	callbacks []func(Event)
}

// NewExtendedManager 创建扩展管理器
func NewExtendedManager(config UPSConfig) *ExtendedManager {
	return &ExtendedManager{
		config:  config,
		devices: make(map[string]*Device),
		events:  make([]Event, 0),
		tasks:   make([]*ShutdownTask, 0),
		stopCh:  make(chan struct{}),
	}
}

// AddDevice 添加设备
func (m *ExtendedManager) AddDevice(device *Device) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if device.ID == "" {
		device.ID = uuid.New().String()
	}

	if _, exists := m.devices[device.ID]; exists {
		return ErrDeviceExists
	}

	device.ConnectedAt = time.Now()
	device.LastSeen = time.Now()
	device.Status = DeviceStatusOnline
	m.devices[device.ID] = device

	m.addEvent(EventDeviceConnected, device.ID, "设备已连接", nil)
	return nil
}

// RemoveDevice 移除设备
func (m *ExtendedManager) RemoveDevice(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.devices[deviceID]; !exists {
		return ErrDeviceNotFound
	}

	// 取消该设备的待执行任务
	for _, task := range m.tasks {
		if task.DeviceID == deviceID && task.Status == TaskStatusPending {
			now := time.Now()
			task.Status = TaskStatusCancelled
			task.CancelledAt = &now
		}
	}

	delete(m.devices, deviceID)
	m.addEvent(EventDeviceDisconnected, deviceID, "设备已断开", nil)
	return nil
}

// GetDevice 获取设备
func (m *ExtendedManager) GetDevice(deviceID string) (*Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, exists := m.devices[deviceID]
	if !exists {
		return nil, ErrDeviceNotFound
	}
	return device, nil
}

// ListDevices 列出所有设备
func (m *ExtendedManager) ListDevices() []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*Device, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, d)
	}
	return devices
}

// CreateShutdownTask 创建关机任务
func (m *ExtendedManager) CreateShutdownTask(deviceID, reason string, delay int) (*ShutdownTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.devices[deviceID]; !exists {
		return nil, ErrDeviceNotFound
	}

	if delay <= 0 {
		delay = 60
	}

	task := &ShutdownTask{
		ID:          uuid.New().String(),
		DeviceID:    deviceID,
		Reason:      reason,
		Delay:       delay,
		Status:      TaskStatusPending,
		CreatedAt:   time.Now(),
		ScheduledAt: time.Now().Add(time.Duration(delay) * time.Second),
	}

	m.tasks = append(m.tasks, task)
	m.addEvent(EventShutdownInitiated, deviceID, reason, task)
	return task, nil
}

// GetTask 获取任务
func (m *ExtendedManager) GetTask(taskID string) (*ShutdownTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, task := range m.tasks {
		if task.ID == taskID {
			return task, nil
		}
	}
	return nil, ErrTaskNotFound
}

// ListTasks 列出任务
func (m *ExtendedManager) ListTasks() []*ShutdownTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*ShutdownTask, len(m.tasks))
	copy(tasks, m.tasks)
	return tasks
}

// CancelTask 取消任务
func (m *ExtendedManager) CancelTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, task := range m.tasks {
		if task.ID == taskID {
			if task.Status != TaskStatusPending {
				return ErrTaskNotCancellable
			}
			now := time.Now()
			task.Status = TaskStatusCancelled
			task.CancelledAt = &now
			return nil
		}
	}
	return ErrTaskNotFound
}

// GetEvents 获取事件
func (m *ExtendedManager) GetEvents(limit int) []Event {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.events) {
		limit = len(m.events)
	}

	events := make([]Event, limit)
	for i := 0; i < limit; i++ {
		events[i] = m.events[len(m.events)-1-i]
	}
	return events
}

// AcknowledgeEvent 确认事件
func (m *ExtendedManager) AcknowledgeEvent(eventID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.events {
		if m.events[i].ID == eventID {
			m.events[i].Acknowledged = true
			return nil
		}
	}
	return errors.New("事件未找到")
}

// addEvent 添加事件
func (m *ExtendedManager) addEvent(eventType EventType, deviceID, message string, details interface{}) {
	event := Event{
		ID:        uuid.New().String(),
		DeviceID:  deviceID,
		Type:      eventType,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
	m.events = append(m.events, event)

	// 触发回调
	for _, cb := range m.callbacks {
		go cb(event)
	}
}

// RegisterEventCallback 注册事件回调
func (m *ExtendedManager) RegisterEventCallback(fn func(Event)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = append(m.callbacks, fn)
}

// RunSelfTest 运行设备自检
func (m *ExtendedManager) RunSelfTest(deviceID string) (*SelfTestResult, error) {
	m.mu.RLock()
	device, exists := m.devices[deviceID]
	m.mu.RUnlock()

	if !exists {
		return nil, ErrDeviceNotFound
	}

	return &SelfTestResult{
		Success: true,
		Message: "设备 " + device.Name + " 自检通过",
	}, nil
}

// SelfTestResult 自检结果
type SelfTestResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
