// Package smarthub provides smart home hub functionality for NAS-OS.
package smarthub

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager manages smart home devices, scenes, and automation.
type Manager struct {
	logger *zap.Logger

	// 设备管理
	devices    map[string]*Device
	devicesMu  sync.RWMutex

	// 协议网关
	gateways    map[string]*ProtocolGateway
	gatewaysMu  sync.RWMutex

	// 设备分组
	groups    map[string]*DeviceGroup
	groupsMu  sync.RWMutex

	// 房间
	rooms    map[string]*Room
	roomsMu  sync.RWMutex

	// 场景自动化
	scenes    map[string]*Scene
	scenesMu  sync.RWMutex

	// 能耗数据
	energyStats    map[string]*EnergyStats
	energyStatsMu  sync.RWMutex

	// 语音命令历史
	voiceHistory    []*VoiceCommand
	voiceHistoryMu  sync.RWMutex

	// 控制通道
	discoverChan chan *DeviceDiscoveryResult
	stopChan     chan struct{}
}

// NewManager creates a new smart home manager.
func NewManager(logger *zap.Logger) (*Manager, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	m := &Manager{
		logger:       logger,
		devices:      make(map[string]*Device),
		gateways:     make(map[string]*ProtocolGateway),
		groups:       make(map[string]*DeviceGroup),
		rooms:        make(map[string]*Room),
		scenes:       make(map[string]*Scene),
		energyStats:  make(map[string]*EnergyStats),
		voiceHistory: make([]*VoiceCommand, 0),
		discoverChan: make(chan *DeviceDiscoveryResult, 100),
		stopChan:     make(chan struct{}),
	}

	// 初始化默认房间
	m.initDefaultRooms()

	// 初始化默认网关
	m.initDefaultGateways()

	return m, nil
}

// initDefaultRooms initializes default rooms.
func (m *Manager) initDefaultRooms() {
	defaultRooms := []struct{ id, name string }{
		{"living_room", "客厅"},
		{"bedroom", "卧室"},
		{"kitchen", "厨房"},
		{"bathroom", "卫生间"},
		{"study", "书房"},
	}
	for _, r := range defaultRooms {
		m.rooms[r.id] = &Room{
			ID:   r.id,
			Name: r.name,
		}
	}
}

// initDefaultGateways initializes default protocol gateways.
func (m *Manager) initDefaultGateways() {
	defaultGateways := []struct {
		id       string
		protocol Protocol
		name     string
		port     int
	}{
		{"zigbee-gw", ProtocolZigbee, "Zigbee Gateway", 8080},
		{"zwave-gw", ProtocolZWave, "Z-Wave Gateway", 8081},
		{"matter-gw", ProtocolMatter, "Matter Gateway", 8082},
		{"ble-gw", ProtocolBLE, "BLE Gateway", 8083},
	}
	for _, gw := range defaultGateways {
		m.gateways[gw.id] = &ProtocolGateway{
			ID:       gw.id,
			Protocol: gw.protocol,
			Name:     gw.name,
			Port:     gw.port,
			Status:   GatewayStopped,
		}
	}
}

// Close stops the manager.
func (m *Manager) Close() error {
	close(m.stopChan)
	return nil
}

// ============================================================
// 设备管理
// ============================================================

// ListDevices lists all devices.
func (m *Manager) ListDevices() []*Device {
	m.devicesMu.RLock()
	defer m.devicesMu.RUnlock()

	devices := make([]*Device, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, d)
	}
	return devices
}

// GetDevice gets a device by ID.
func (m *Manager) GetDevice(id string) (*Device, error) {
	m.devicesMu.RLock()
	defer m.devicesMu.RUnlock()

	device, ok := m.devices[id]
	if !ok {
		return nil, fmt.Errorf("device not found: %s", id)
	}
	return device, nil
}

// CreateDevice creates a new device.
func (m *Manager) CreateDevice(req CreateDeviceRequest) (*Device, error) {
	m.devicesMu.Lock()
	defer m.devicesMu.Unlock()

	// Check duplicate MAC if provided
	if req.MACAddress != "" {
		for _, d := range m.devices {
			if d.MACAddress == req.MACAddress {
				return nil, fmt.Errorf("device with MAC %s already exists: %s", req.MACAddress, d.ID)
			}
		}
	}

	now := time.Now()
	device := &Device{
		ID:           uuid.New().String(),
		Name:         req.Name,
		Type:         req.Type,
		Protocol:     req.Protocol,
		Manufacturer: req.Manufacturer,
		Model:        req.Model,
		MACAddress:   req.MACAddress,
		IPAddress:    req.IPAddress,
		RoomID:       req.RoomID,
		Status:       StatusOnline,
		Properties:   make(map[string]interface{}),
		LastSeen:     now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	m.devices[device.ID] = device
	m.logger.Info("device created", zap.String("id", device.ID), zap.String("name", device.Name))

	// Update room device list
	if req.RoomID != "" {
		m.addDeviceToRoom(device.ID, req.RoomID)
	}

	return device, nil
}

// UpdateDevice updates device info.
func (m *Manager) UpdateDevice(id string, req UpdateDeviceRequest) (*Device, error) {
	m.devicesMu.Lock()
	defer m.devicesMu.Unlock()

	device, ok := m.devices[id]
	if !ok {
		return nil, fmt.Errorf("device not found: %s", id)
	}

	if req.Name != "" {
		device.Name = req.Name
	}
	if req.RoomID != "" {
		// Remove from old room
		if device.RoomID != "" {
			m.removeDeviceFromRoom(device.ID, device.RoomID)
		}
		device.RoomID = req.RoomID
		m.addDeviceToRoom(device.ID, req.RoomID)
	}
	if req.GroupIDs != nil {
		device.GroupIDs = req.GroupIDs
	}
	device.UpdatedAt = time.Now()

	return device, nil
}

// DeleteDevice deletes a device.
func (m *Manager) DeleteDevice(id string) error {
	m.devicesMu.Lock()
	defer m.devicesMu.Unlock()

	device, ok := m.devices[id]
	if !ok {
		return fmt.Errorf("device not found: %s", id)
	}

	// Remove from room
	if device.RoomID != "" {
		m.removeDeviceFromRoom(device.ID, device.RoomID)
	}

	// Remove from groups
	m.groupsMu.Lock()
	for _, group := range m.groups {
		newIDs := make([]string, 0, len(group.DeviceIDs))
		for _, did := range group.DeviceIDs {
			if did != id {
				newIDs = append(newIDs, did)
			}
		}
		group.DeviceIDs = newIDs
	}
	m.groupsMu.Unlock()

	delete(m.devices, id)
	m.logger.Info("device deleted", zap.String("id", id))
	return nil
}

// ControlDevice controls a device property.
func (m *Manager) ControlDevice(id string, req ControlDeviceRequest) (*Device, error) {
	m.devicesMu.Lock()
	defer m.devicesMu.Unlock()

	device, ok := m.devices[id]
	if !ok {
		return nil, fmt.Errorf("device not found: %s", id)
	}

	if device.Status != StatusOnline {
		return nil, fmt.Errorf("device is offline: %s", id)
	}

	if device.Properties == nil {
		device.Properties = make(map[string]interface{})
	}
	device.Properties[req.Property] = req.Value
	device.UpdatedAt = time.Now()

	m.logger.Info("device controlled",
		zap.String("device_id", id),
		zap.String("property", req.Property),
		zap.Any("value", req.Value),
	)

	return device, nil
}

// addDeviceToRoom adds a device to a room.
func (m *Manager) addDeviceToRoom(deviceID, roomID string) {
	m.roomsMu.Lock()
	defer m.roomsMu.Unlock()

	room, ok := m.rooms[roomID]
	if !ok {
		return
	}

	for _, id := range room.DeviceIDs {
		if id == deviceID {
			return
		}
	}
	room.DeviceIDs = append(room.DeviceIDs, deviceID)
}

// removeDeviceFromRoom removes a device from a room.
func (m *Manager) removeDeviceFromRoom(deviceID, roomID string) {
	m.roomsMu.Lock()
	defer m.roomsMu.Unlock()

	room, ok := m.rooms[roomID]
	if !ok {
		return
	}

	newIDs := make([]string, 0, len(room.DeviceIDs))
	for _, id := range room.DeviceIDs {
		if id != deviceID {
			newIDs = append(newIDs, id)
		}
	}
	room.DeviceIDs = newIDs
}

// ============================================================
// 设备发现
// ============================================================

// DiscoverDevices starts device discovery.
func (m *Manager) DiscoverDevices(ctx context.Context, req DiscoverDevicesRequest) (*DeviceDiscoveryResult, error) {
	if req.TimeoutSec <= 0 {
		req.TimeoutSec = 30
	}

	result := &DeviceDiscoveryResult{
		Method:    req.Methods[0],
		Devices:   make([]*Device, 0),
		ScannedAt: time.Now(),
	}

	m.logger.Info("device discovery started",
		zap.Any("methods", req.Methods),
		zap.Int("timeout_sec", req.TimeoutSec),
	)

	// Simulate discovery - in real implementation, this would call mDNS/SSDP/BLE
	// For now, return existing devices that match the requested protocols
	m.devicesMu.RLock()
	for _, device := range m.devices {
		for _, method := range req.Methods {
			if (method == DiscoveryMDNS && device.Protocol == ProtocolWiFi) ||
				(method == DiscoveryBLE && device.Protocol == ProtocolBLE) ||
				method == DiscoverySSDP {
				result.Devices = append(result.Devices, device)
			}
		}
	}
	m.devicesMu.RUnlock()

	m.logger.Info("device discovery completed", zap.Int("found", len(result.Devices)))
	return result, nil
}

// ============================================================
// 协议网关管理
// ============================================================

// ListGateways lists all protocol gateways.
func (m *Manager) ListGateways() []*ProtocolGateway {
	m.gatewaysMu.RLock()
	defer m.gatewaysMu.RUnlock()

	gateways := make([]*ProtocolGateway, 0, len(m.gateways))
	for _, gw := range m.gateways {
		// Update device count
		gw.DeviceCount = m.countDevicesByProtocol(gw.Protocol)
		gateways = append(gateways, gw)
	}
	return gateways
}

// GetGateway gets a gateway by ID.
func (m *Manager) GetGateway(id string) (*ProtocolGateway, error) {
	m.gatewaysMu.RLock()
	defer m.gatewaysMu.RUnlock()

	gw, ok := m.gateways[id]
	if !ok {
		return nil, fmt.Errorf("gateway not found: %s", id)
	}
	gw.DeviceCount = m.countDevicesByProtocol(gw.Protocol)
	return gw, nil
}

// StartGateway starts a protocol gateway.
func (m *Manager) StartGateway(ctx context.Context, id string) error {
	m.gatewaysMu.Lock()
	defer m.gatewaysMu.Unlock()

	gw, ok := m.gateways[id]
	if !ok {
		return fmt.Errorf("gateway not found: %s", id)
	}

	if gw.Status == GatewayRunning {
		return fmt.Errorf("gateway already running: %s", id)
	}

	now := time.Now()
	gw.Status = GatewayRunning
	gw.StartedAt = &now
	gw.ErrorMsg = ""

	m.logger.Info("gateway started", zap.String("id", id), zap.String("protocol", string(gw.Protocol)))
	return nil
}

// StopGateway stops a protocol gateway.
func (m *Manager) StopGateway(ctx context.Context, id string) error {
	m.gatewaysMu.Lock()
	defer m.gatewaysMu.Unlock()

	gw, ok := m.gateways[id]
	if !ok {
		return fmt.Errorf("gateway not found: %s", id)
	}

	if gw.Status == GatewayStopped {
		return fmt.Errorf("gateway already stopped: %s", id)
	}

	gw.Status = GatewayStopped
	gw.StartedAt = nil
	gw.ErrorMsg = ""

	m.logger.Info("gateway stopped", zap.String("id", id))
	return nil
}

// countDevicesByProtocol counts devices by protocol.
func (m *Manager) countDevicesByProtocol(protocol Protocol) int {
	m.devicesMu.RLock()
	defer m.devicesMu.RUnlock()

	count := 0
	for _, d := range m.devices {
		if d.Protocol == protocol {
			count++
		}
	}
	return count
}

// ============================================================
// 设备分组管理
// ============================================================

// ListGroups lists all device groups.
func (m *Manager) ListGroups() []*DeviceGroup {
	m.groupsMu.RLock()
	defer m.groupsMu.RUnlock()

	groups := make([]*DeviceGroup, 0, len(m.groups))
	for _, g := range m.groups {
		groups = append(groups, g)
	}
	return groups
}

// GetGroup gets a group by ID.
func (m *Manager) GetGroup(id string) (*DeviceGroup, error) {
	m.groupsMu.RLock()
	defer m.groupsMu.RUnlock()

	group, ok := m.groups[id]
	if !ok {
		return nil, fmt.Errorf("group not found: %s", id)
	}
	return group, nil
}

// CreateGroup creates a device group.
func (m *Manager) CreateGroup(req CreateGroupRequest) (*DeviceGroup, error) {
	m.groupsMu.Lock()
	defer m.groupsMu.Unlock()

	now := time.Now()
	group := &DeviceGroup{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		DeviceIDs:   req.DeviceIDs,
		RoomID:      req.RoomID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	m.groups[group.ID] = group
	m.logger.Info("group created", zap.String("id", group.ID), zap.String("name", group.Name))
	return group, nil
}

// UpdateGroup updates a device group.
func (m *Manager) UpdateGroup(id string, req CreateGroupRequest) (*DeviceGroup, error) {
	m.groupsMu.Lock()
	defer m.groupsMu.Unlock()

	group, ok := m.groups[id]
	if !ok {
		return nil, fmt.Errorf("group not found: %s", id)
	}

	if req.Name != "" {
		group.Name = req.Name
	}
	if req.Description != "" {
		group.Description = req.Description
	}
	if req.DeviceIDs != nil {
		group.DeviceIDs = req.DeviceIDs
	}
	if req.RoomID != "" {
		group.RoomID = req.RoomID
	}
	group.UpdatedAt = time.Now()

	return group, nil
}

// DeleteGroup deletes a device group.
func (m *Manager) DeleteGroup(id string) error {
	m.groupsMu.Lock()
	defer m.groupsMu.Unlock()

	if _, ok := m.groups[id]; !ok {
		return fmt.Errorf("group not found: %s", id)
	}

	delete(m.groups, id)
	m.logger.Info("group deleted", zap.String("id", id))
	return nil
}

// ============================================================
// 房间管理
// ============================================================

// ListRooms lists all rooms.
func (m *Manager) ListRooms() []*Room {
	m.roomsMu.RLock()
	defer m.roomsMu.RUnlock()

	rooms := make([]*Room, 0, len(m.rooms))
	for _, r := range m.rooms {
		rooms = append(rooms, r)
	}
	return rooms
}

// CreateRoom creates a new room.
func (m *Manager) CreateRoom(name string) (*Room, error) {
	m.roomsMu.Lock()
	defer m.roomsMu.Unlock()

	room := &Room{
		ID:   uuid.New().String(),
		Name: name,
	}
	m.rooms[room.ID] = room
	return room, nil
}

// ============================================================
// 场景自动化
// ============================================================

// ListScenes lists all scenes.
func (m *Manager) ListScenes() []*Scene {
	m.scenesMu.RLock()
	defer m.scenesMu.RUnlock()

	scenes := make([]*Scene, 0, len(m.scenes))
	for _, s := range m.scenes {
		scenes = append(scenes, s)
	}
	return scenes
}

// GetScene gets a scene by ID.
func (m *Manager) GetScene(id string) (*Scene, error) {
	m.scenesMu.RLock()
	defer m.scenesMu.RUnlock()

	scene, ok := m.scenes[id]
	if !ok {
		return nil, fmt.Errorf("scene not found: %s", id)
	}
	return scene, nil
}

// CreateScene creates a new scene.
func (m *Manager) CreateScene(req CreateSceneRequest) (*Scene, error) {
	m.scenesMu.Lock()
	defer m.scenesMu.Unlock()

	now := time.Now()
	scene := &Scene{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Enabled:     true,
		Triggers:    req.Triggers,
		Actions:     req.Actions,
		RunCount:    0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	m.scenes[scene.ID] = scene
	m.logger.Info("scene created", zap.String("id", scene.ID), zap.String("name", scene.Name))
	return scene, nil
}

// UpdateScene updates a scene.
func (m *Manager) UpdateScene(id string, req CreateSceneRequest) (*Scene, error) {
	m.scenesMu.Lock()
	defer m.scenesMu.Unlock()

	scene, ok := m.scenes[id]
	if !ok {
		return nil, fmt.Errorf("scene not found: %s", id)
	}

	scene.Name = req.Name
	scene.Description = req.Description
	scene.Triggers = req.Triggers
	scene.Actions = req.Actions
	scene.UpdatedAt = time.Now()

	return scene, nil
}

// DeleteScene deletes a scene.
func (m *Manager) DeleteScene(id string) error {
	m.scenesMu.Lock()
	defer m.scenesMu.Unlock()

	if _, ok := m.scenes[id]; !ok {
		return fmt.Errorf("scene not found: %s", id)
	}

	delete(m.scenes, id)
	m.logger.Info("scene deleted", zap.String("id", id))
	return nil
}

// RunScene manually runs a scene.
func (m *Manager) RunScene(ctx context.Context, id string) (*SceneExecution, error) {
	m.scenesMu.RLock()
	scene, ok := m.scenes[id]
	m.scenesMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("scene not found: %s", id)
	}

	if !scene.Enabled {
		return nil, fmt.Errorf("scene is disabled: %s", id)
	}

	execution := &SceneExecution{
		ID:        uuid.New().String(),
		SceneID:   id,
		Trigger:   "manual",
		Status:    "success",
		StartedAt: time.Now(),
	}

	// Execute actions
	for _, action := range scene.Actions {
		if err := m.executeAction(ctx, action); err != nil {
			execution.Status = "failed"
			execution.Error = err.Error()
			break
		}
	}

	execution.EndedAt = time.Now()

	// Update scene stats
	m.scenesMu.Lock()
	scene.RunCount++
	now := time.Now()
	scene.LastRun = &now
	m.scenesMu.Unlock()

	m.logger.Info("scene executed",
		zap.String("scene_id", id),
		zap.String("status", execution.Status),
	)

	return execution, nil
}

// executeAction executes a single action.
func (m *Manager) executeAction(ctx context.Context, action Action) error {
	switch action.Type {
	case ActionSetProperty:
		if action.DeviceID == "" {
			return fmt.Errorf("device_id required for set_property")
		}
		_, err := m.ControlDevice(action.DeviceID, ControlDeviceRequest{
			Property: action.Property,
			Value:    action.Value,
		})
		return err

	case ActionRunScene:
		if action.SceneID == "" {
			return fmt.Errorf("scene_id required for run_scene")
		}
		_, err := m.RunScene(ctx, action.SceneID)
		return err

	case ActionSendNotification:
		m.logger.Info("notification sent",
			zap.String("message", fmt.Sprintf("%v", action.Value)),
		)
		return nil

	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
}

// ============================================================
// 能耗监控
// ============================================================

// GetEnergyStats gets energy stats for a device.
func (m *Manager) GetEnergyStats(deviceID string) (*EnergyStats, error) {
	m.energyStatsMu.RLock()
	defer m.energyStatsMu.RUnlock()

	stats, ok := m.energyStats[deviceID]
	if !ok {
		return nil, fmt.Errorf("no energy stats for device: %s", deviceID)
	}
	return stats, nil
}

// GetAllEnergyStats gets all energy stats.
func (m *Manager) GetAllEnergyStats() []*EnergyStats {
	m.energyStatsMu.RLock()
	defer m.energyStatsMu.RUnlock()

	stats := make([]*EnergyStats, 0, len(m.energyStats))
	for _, s := range m.energyStats {
		stats = append(stats, s)
	}
	return stats
}

// RecordEnergyReading records an energy reading.
func (m *Manager) RecordEnergyReading(reading EnergyReading) error {
	m.energyStatsMu.Lock()
	defer m.energyStatsMu.Unlock()

	stats, ok := m.energyStats[reading.DeviceID]
	if !ok {
		stats = &EnergyStats{
			DeviceID:   reading.DeviceID,
			MinPower:   reading.Power,
			MaxPower:   reading.Power,
		}
		m.energyStats[reading.DeviceID] = stats
	}

	// Update stats
	stats.CurrentPower = reading.Power
	stats.TotalEnergy = reading.Energy
	stats.LastReading = reading.Timestamp
	stats.ReadingCount++

	// Update min/max
	if reading.Power > stats.MaxPower {
		stats.MaxPower = reading.Power
	}
	if reading.Power < stats.MinPower {
		stats.MinPower = reading.Power
	}

	// Calculate average
	if stats.ReadingCount > 0 {
		stats.AvgPower = (stats.AvgPower*float64(stats.ReadingCount-1) + reading.Power) / float64(stats.ReadingCount)
	}

	stats.UpdatedAt = time.Now()

	// Check for high power alert
	if reading.Power > 1000 { // Threshold: 1000W
		m.logger.Warn("high power usage detected",
			zap.String("device_id", reading.DeviceID),
			zap.Float64("power", reading.Power),
		)
	}

	return nil
}

// ============================================================
// 语音控制
// ============================================================

// ProcessVoiceCommand processes a voice command.
func (m *Manager) ProcessVoiceCommand(ctx context.Context, req VoiceCommandRequest) (*VoiceResponse, error) {
	if req.Language == "" {
		req.Language = "zh-CN"
	}

	cmd := &VoiceCommand{
		ID:        uuid.New().String(),
		Text:      req.Text,
		Language:  req.Language,
		Timestamp: time.Now(),
	}

	// Parse intent from text
	intent, deviceID, action, value := m.parseVoiceIntent(req.Text, req.Language)
	cmd.Intent = intent
	cmd.DeviceID = deviceID
	cmd.Action = action
	cmd.Value = value
	cmd.Confidence = 0.85 // Simulated confidence

	// Execute command
	var err error
	var message string

	switch intent {
	case "control_device":
		if deviceID != "" && action != "" {
			_, err = m.ControlDevice(deviceID, ControlDeviceRequest{
				Property: action,
				Value:    value,
			})
			if err != nil {
				message = fmt.Sprintf("操作失败: %v", err)
			} else {
				message = "操作成功"
				cmd.Processed = true
			}
		} else {
			message = "未识别到有效的设备操作"
		}

	case "run_scene":
		if deviceID != "" {
			_, err = m.RunScene(ctx, deviceID)
			if err != nil {
				message = fmt.Sprintf("场景执行失败: %v", err)
			} else {
				message = "场景已执行"
				cmd.Processed = true
			}
		} else {
			message = "未找到匹配的场景"
		}

	case "query_status":
		message = "设备状态查询功能开发中"
		cmd.Processed = true

	default:
		message = "抱歉，我无法理解这个指令"
		cmd.Confidence = 0.3
	}

	if err != nil {
		cmd.Error = err.Error()
	}

	// Save to history
	m.voiceHistoryMu.Lock()
	m.voiceHistory = append(m.voiceHistory, cmd)
	// Keep only last 100 commands
	if len(m.voiceHistory) > 100 {
		m.voiceHistory = m.voiceHistory[len(m.voiceHistory)-100:]
	}
	m.voiceHistoryMu.Unlock()

	return &VoiceResponse{
		Success: cmd.Processed,
		Message: message,
		Data:    cmd,
	}, nil
}

// parseVoiceIntent parses voice command intent.
func (m *Manager) parseVoiceIntent(text, language string) (intent, deviceID, action string, value interface{}) {
	// Simple keyword-based parsing for demo
	// Real implementation would use NLP/LLM

	// Control commands
	controlKeywords := map[string]struct {
		action string
		value  interface{}
	}{
		"开灯":   {"on", true},
		"关灯":   {"on", false},
		"打开灯":  {"on", true},
		"关闭灯":  {"on", false},
		"调亮":   {"brightness", 100},
		"调暗":   {"brightness", 30},
		"开空调":  {"on", true},
		"关空调":  {"on", false},
		"温度调高": {"temperature", 26},
		"温度调低": {"temperature", 20},
	}

	for keyword, cmd := range controlKeywords {
		if contains(text, keyword) {
			return "control_device", "", cmd.action, cmd.value
		}
	}

	// Scene commands
	sceneKeywords := []string{"执行场景", "运行场景", "回家模式", "离家模式", "睡眠模式"}
	for _, kw := range sceneKeywords {
		if contains(text, kw) {
			return "run_scene", "", "", nil
		}
	}

	// Query commands
	queryKeywords := []string{"查询", "状态", "温度", "湿度"}
	for _, kw := range queryKeywords {
		if contains(text, kw) {
			return "query_status", "", "", nil
		}
	}

	return "unknown", "", "", nil
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetVoiceHistory gets voice command history.
func (m *Manager) GetVoiceHistory() []*VoiceCommand {
	m.voiceHistoryMu.RLock()
	defer m.voiceHistoryMu.RUnlock()

	history := make([]*VoiceCommand, len(m.voiceHistory))
	copy(history, m.voiceHistory)
	return history
}

// ============================================================
// 统计信息
// ============================================================

// GetStats gets overall statistics.
func (m *Manager) GetStats() map[string]interface{} {
	m.devicesMu.RLock()
	deviceCount := len(m.devices)
	onlineCount := 0
	for _, d := range m.devices {
		if d.Status == StatusOnline {
			onlineCount++
		}
	}
	m.devicesMu.RUnlock()

	m.gatewaysMu.RLock()
	gatewayCount := len(m.gateways)
	runningGateways := 0
	for _, gw := range m.gateways {
		if gw.Status == GatewayRunning {
			runningGateways++
		}
	}
	m.gatewaysMu.RUnlock()

	m.groupsMu.RLock()
	groupCount := len(m.groups)
	m.groupsMu.RUnlock()

	m.scenesMu.RLock()
	sceneCount := len(m.scenes)
	enabledScenes := 0
	for _, s := range m.scenes {
		if s.Enabled {
			enabledScenes++
		}
	}
	m.scenesMu.RUnlock()

	m.voiceHistoryMu.RLock()
	voiceCommandCount := len(m.voiceHistory)
	m.voiceHistoryMu.RUnlock()

	return map[string]interface{}{
		"devices":           deviceCount,
		"online_devices":    onlineCount,
		"gateways":          gatewayCount,
		"running_gateways":  runningGateways,
		"groups":            groupCount,
		"scenes":            sceneCount,
		"enabled_scenes":    enabledScenes,
		"voice_commands":    voiceCommandCount,
	}
}
