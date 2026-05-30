package smarthomehub

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Manager 智能家居中枢管理器
type Manager struct {
	mu          sync.RWMutex
	config      *HubConfig
	devices     map[string]*Device
	scenes      map[string]*Scene
	automations map[string]*Automation
	rooms       map[string]*Room
}

// NewManager 创建新的管理器
func NewManager(config *HubConfig) *Manager {
	if config == nil {
		config = &HubConfig{
			MatterEnabled:    true,
			ZigbeePort:       "/dev/ttyUSB0",
			ZWavePort:        "/dev/ttyACM0",
			DiscoveryTimeout: 30,
			MaxDevices:       100,
		}
	}
	return &Manager{
		config:      config,
		devices:     make(map[string]*Device),
		scenes:      make(map[string]*Scene),
		automations: make(map[string]*Automation),
		rooms:       make(map[string]*Room),
	}
}

// DiscoverDevices 多协议设备发现
func (m *Manager) DiscoverDevices(ctx context.Context, timeout int) ([]Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if timeout <= 0 {
		timeout = m.config.DiscoveryTimeout
	}

	// 模拟设备发现过程
	discovered := []Device{
		{
			ID:              fmt.Sprintf("dev-%d", time.Now().UnixNano()),
			Name:            "Living Room Light",
			Type:            DeviceTypeLight,
			Protocol:        ProtocolMatter,
			Room:            "living-room",
			Status:          StatusOnline,
			Capabilities:    []string{"on", "brightness", "color"},
			FirmwareVersion: "1.0.0",
			BatteryLevel:    0,
			LastSeen:        time.Now(),
			Online:          true,
		},
		{
			ID:              fmt.Sprintf("dev-%d", time.Now().UnixNano()+1),
			Name:            "Bedroom Thermostat",
			Type:            DeviceTypeThermostat,
			Protocol:        ProtocolZigbee,
			Room:            "bedroom",
			Status:          StatusOnline,
			Capabilities:    []string{"temperature", "mode"},
			FirmwareVersion: "2.1.0",
			BatteryLevel:    85,
			LastSeen:        time.Now(),
			Online:          true,
		},
	}

	return discovered, nil
}

// AddDevice 添加设备
func (m *Manager) AddDevice(device Device) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.devices) >= m.config.MaxDevices {
		return fmt.Errorf("已达到最大设备数量限制: %d", m.config.MaxDevices)
	}

	if _, exists := m.devices[device.ID]; exists {
		return fmt.Errorf("设备已存在: %s", device.ID)
	}

	device.LastSeen = time.Now()
	if device.Status == "" {
		device.Status = StatusOnline
	}
	m.devices[device.ID] = &device
	return nil
}

// GetDevice 获取设备
func (m *Manager) GetDevice(deviceID string) (*Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, exists := m.devices[deviceID]
	if !exists {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}
	return device, nil
}

// ControlDevice 控制设备
func (m *Manager) ControlDevice(deviceID string, command string, params map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[deviceID]
	if !exists {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	if !device.Online {
		return fmt.Errorf("设备离线: %s", deviceID)
	}

	// 模拟执行命令
	device.LastSeen = time.Now()
	return nil
}

// ListDevices 列出设备
func (m *Manager) ListDevices(roomID string) ([]Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var devices []Device
	for _, device := range m.devices {
		if roomID == "" || device.Room == roomID {
			devices = append(devices, *device)
		}
	}
	return devices, nil
}

// CreateScene 创建场景
func (m *Manager) CreateScene(scene Scene) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.scenes[scene.ID]; exists {
		return fmt.Errorf("场景已存在: %s", scene.ID)
	}

	if !scene.Enabled {
		scene.Enabled = true
	}
	m.scenes[scene.ID] = &scene
	return nil
}

// ActivateScene 激活场景
func (m *Manager) ActivateScene(sceneID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	scene, exists := m.scenes[sceneID]
	if !exists {
		return fmt.Errorf("场景不存在: %s", sceneID)
	}

	if !scene.Enabled {
		return fmt.Errorf("场景已禁用: %s", sceneID)
	}

	now := time.Now()
	scene.LastTriggered = &now

	// 模拟执行场景动作
	for _, action := range scene.Actions {
		device, exists := m.devices[action.DeviceID]
		if exists && device.Online {
			device.LastSeen = time.Now()
		}
	}

	return nil
}

// CreateAutomation 创建自动化规则
func (m *Manager) CreateAutomation(automation Automation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.automations[automation.ID]; exists {
		return fmt.Errorf("自动化规则已存在: %s", automation.ID)
	}

	if !automation.Enabled {
		automation.Enabled = true
	}
	m.automations[automation.ID] = &automation
	return nil
}

// EvaluateAutomations 评估所有自动化规则
func (m *Manager) EvaluateAutomations(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, automation := range m.automations {
		if !automation.Enabled {
			continue
		}

		// 检查冷却时间
		if automation.LastFired != nil {
			cooldown := time.Duration(automation.Cooldown) * time.Second
			if time.Since(*automation.LastFired) < cooldown {
				continue
			}
		}

		// 评估条件
		allConditionsMet := true
		for _, condition := range automation.Conditions {
			device, exists := m.devices[condition.DeviceID]
			if !exists {
				allConditionsMet = false
				break
			}

			if !device.Online {
				allConditionsMet = false
				break
			}

			// 模拟条件评估
			if condition.Duration > 0 {
				// 检查设备状态是否持续满足条件
				if time.Since(device.LastSeen) > time.Duration(condition.Duration)*time.Second {
					allConditionsMet = false
					break
				}
			}
		}

		if allConditionsMet {
			// 执行动作
			now := time.Now()
			automation.LastFired = &now

			for _, action := range automation.Actions {
				device, exists := m.devices[action.DeviceID]
				if exists && device.Online {
					device.LastSeen = time.Now()
				}
			}
		}
	}

	return nil
}

// AddRoom 添加房间
func (m *Manager) AddRoom(room Room) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rooms[room.ID]; exists {
		return fmt.Errorf("房间已存在: %s", room.ID)
	}

	m.rooms[room.ID] = &room
	return nil
}

// GetRooms 获取所有房间
func (m *Manager) GetRooms() ([]Room, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rooms := make([]Room, 0, len(m.rooms))
	for _, room := range m.rooms {
		rooms = append(rooms, *room)
	}
	return rooms, nil
}

// GetHubStatus 获取中枢状态
func (m *Manager) GetHubStatus() (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	onlineDevices := 0
	for _, device := range m.devices {
		if device.Online {
			onlineDevices++
		}
	}

	status := map[string]interface{}{
		"total_devices":   len(m.devices),
		"online_devices":  onlineDevices,
		"total_scenes":    len(m.scenes),
		"total_automations": len(m.automations),
		"total_rooms":     len(m.rooms),
		"config": map[string]interface{}{
			"matter_enabled":    m.config.MatterEnabled,
			"zigbee_port":       m.config.ZigbeePort,
			"zwave_port":        m.config.ZWavePort,
			"discovery_timeout": m.config.DiscoveryTimeout,
			"max_devices":       m.config.MaxDevices,
		},
		"uptime": time.Now().Format(time.RFC3339),
	}

	return status, nil
}
