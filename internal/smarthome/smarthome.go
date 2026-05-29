// Package smarthome 智能家居集成模块
// 对标飞牛fnOS智能家居，支持多协议设备发现与管理、场景自动化、能耗统计
package smarthome

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// 能耗记录
// ============================================================

// RecordEnergyReading 记录能耗读数
func (m *Manager) RecordEnergyReading(reading EnergyReading) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证设备存在
	if _, ok := m.devices[reading.DeviceID]; !ok {
		return ErrDeviceNotFound
	}

	if reading.Timestamp.IsZero() {
		reading.Timestamp = time.Now()
	}

	m.energyData[reading.DeviceID] = append(m.energyData[reading.DeviceID], reading)

	// 限制每个设备最多保留 10000 条记录
	const maxReadings = 10000
	if len(m.energyData[reading.DeviceID]) > maxReadings {
		m.energyData[reading.DeviceID] = m.energyData[reading.DeviceID][len(m.energyData[reading.DeviceID])-maxReadings:]
	}

	return nil
}

// GetDeviceReadings 获取设备能耗原始读数
func (m *Manager) GetDeviceReadings(deviceID string, since time.Time) ([]EnergyReading, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.devices[deviceID]; !ok {
		return nil, ErrDeviceNotFound
	}

	readings, ok := m.energyData[deviceID]
	if !ok {
		return nil, nil
	}

	result := make([]EnergyReading, 0)
	for _, r := range readings {
		if r.Timestamp.After(since) || r.Timestamp.Equal(since) {
			result = append(result, r)
		}
	}
	return result, nil
}

// ClearDeviceReadings 清除设备能耗数据
func (m *Manager) ClearDeviceReadings(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.devices[deviceID]; !ok {
		return ErrDeviceNotFound
	}

	delete(m.energyData, deviceID)
	return nil
}

// ============================================================
// 配置导出/导入
// ============================================================

// ExportConfig 导出智能家居完整配置
func (m *Manager) ExportConfig() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config := struct {
		Devices     map[string]*Device         `json:"devices"`
		Rooms       map[string]*Room           `json:"rooms"`
		Groups      map[string]*Group          `json:"groups"`
		Scenes      map[string]*Scene          `json:"scenes"`
		Tasks       map[string]*ScheduledTask  `json:"tasks"`
		ExportedAt  time.Time                  `json:"exported_at"`
	}{
		Devices:    m.devices,
		Rooms:      m.rooms,
		Groups:     m.groups,
		Scenes:     m.scenes,
		Tasks:      m.tasks,
		ExportedAt: time.Now(),
	}

	return json.MarshalIndent(config, "", "  ")
}

// ImportConfig 导入智能家居配置
func (m *Manager) ImportConfig(data []byte, merge bool) error {
	var config struct {
		Devices map[string]*Device        `json:"devices"`
		Rooms   map[string]*Room          `json:"rooms"`
		Groups  map[string]*Group         `json:"groups"`
		Scenes  map[string]*Scene         `json:"scenes"`
		Tasks   map[string]*ScheduledTask `json:"tasks"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("invalid config format: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if !merge {
		// 全量覆盖
		m.devices = make(map[string]*Device)
		m.rooms = make(map[string]*Room)
		m.groups = make(map[string]*Group)
		m.scenes = make(map[string]*Scene)
		m.tasks = make(map[string]*ScheduledTask)
	}

	if config.Devices != nil {
		for id, d := range config.Devices {
			m.devices[id] = d
		}
	}
	if config.Rooms != nil {
		for id, r := range config.Rooms {
			m.rooms[id] = r
		}
	}
	if config.Groups != nil {
		for id, g := range config.Groups {
			m.groups[id] = g
		}
	}
	if config.Scenes != nil {
		for id, s := range config.Scenes {
			m.scenes[id] = s
		}
	}
	if config.Tasks != nil {
		for id, t := range config.Tasks {
			m.tasks[id] = t
		}
	}

	return nil
}

// ============================================================
// 批量操作
// ============================================================

// BatchControlDevice 批量控制设备（同类型操作）
func (m *Manager) BatchControlDevice(deviceIDs []string, state map[string]any) []error {
	m.mu.Lock()
	defer m.mu.Unlock()

	errs := make([]error, 0)
	now := time.Now()

	for _, id := range deviceIDs {
		device, ok := m.devices[id]
		if !ok {
			errs = append(errs, fmt.Errorf("%s: %w", id, ErrDeviceNotFound))
			continue
		}

		for k, v := range state {
			device.State[k] = v
		}
		device.UpdatedAt = now
		device.LastSeen = now

		m.addEvent(DeviceEvent{
			DeviceID:   device.ID,
			DeviceName: device.Name,
			Type:       "batch_control",
			State:      state,
			Timestamp:  now,
		})
	}

	return errs
}

// BatchMoveToRoom 批量移动设备到房间
func (m *Manager) BatchMoveToRoom(deviceIDs []string, roomID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[roomID]
	if !ok {
		return ErrRoomNotFound
	}

	now := time.Now()
	for _, id := range deviceIDs {
		device, ok := m.devices[id]
		if !ok {
			continue
		}

		// 从旧房间移除
		if device.RoomID != "" {
			if oldRoom, ok := m.rooms[device.RoomID]; ok {
				for i, did := range oldRoom.DeviceIDs {
					if did == id {
						oldRoom.DeviceIDs = append(oldRoom.DeviceIDs[:i], oldRoom.DeviceIDs[i+1:]...)
						break
					}
				}
			}
		}

		device.RoomID = roomID
		device.UpdatedAt = now
		room.DeviceIDs = append(room.DeviceIDs, id)
	}
	room.UpdatedAt = now

	return nil
}

// ============================================================
// 协议适配器接口
// ============================================================

// DeviceAdapter 设备协议适配器接口
// 各协议（Zigbee/Z-Wave/WiFi/BLE）实现此接口即可接入智能家居系统
type DeviceAdapter interface {
	// Protocol 返回协议类型
	Protocol() Protocol
	// Discover 发现设备
	Discover() ([]*Device, error)
	// Connect 连接设备
	Connect(deviceID string) error
	// Disconnect 断开设备
	Disconnect(deviceID string) error
	// SendCommand 发送控制命令
	SendCommand(deviceID string, command map[string]any) error
	// ReadState 读取设备状态
	ReadState(deviceID string) (map[string]any, error)
}

// adapterEntry 适配器注册项
type adapterEntry struct {
	adapter   DeviceAdapter
	enabled   bool
}

// ============================================================
// Manager 扩展：适配器注册
// ============================================================

// 全局适配器注册表（进程内单例）
var (
	adapterRegistry = make(map[Protocol]*adapterEntry)
	adapterMu       sync.RWMutex
)

// RegisterAdapter 注册设备协议适配器
func RegisterAdapter(adapter DeviceAdapter) {
	adapterMu.Lock()
	defer adapterMu.Unlock()
	adapterRegistry[adapter.Protocol()] = &adapterEntry{
		adapter: adapter,
		enabled: true,
	}
}

// GetAdapter 获取指定协议的适配器
func GetAdapter(protocol Protocol) (DeviceAdapter, error) {
	adapterMu.RLock()
	defer adapterMu.RUnlock()

	entry, ok := adapterRegistry[protocol]
	if !ok {
		return nil, fmt.Errorf("no adapter registered for protocol %q", protocol)
	}
	if !entry.enabled {
		return nil, fmt.Errorf("adapter for protocol %q is disabled", protocol)
	}
	return entry.adapter, nil
}

// ListAdapters 列出已注册的适配器
func ListAdapters() []Protocol {
	adapterMu.RLock()
	defer adapterMu.RUnlock()

	protocols := make([]Protocol, 0, len(adapterRegistry))
	for p, entry := range adapterRegistry {
		if entry.enabled {
			protocols = append(protocols, p)
		}
	}
	return protocols
}

// ============================================================
// Manager 扩展：按协议发现设备
// ============================================================

// DiscoverByProtocol 使用指定协议适配器发现设备
func (m *Manager) DiscoverByProtocol(protocol Protocol) ([]*Device, error) {
	adapter, err := GetAdapter(protocol)
	if err != nil {
		return nil, err
	}

	discovered, err := adapter.Discover()
	if err != nil {
		return nil, fmt.Errorf("discovery failed for %q: %w", protocol, err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	added := make([]*Device, 0)

	for _, d := range discovered {
		// 跳过已存在的设备
		if _, exists := m.devices[d.ID]; exists {
			continue
		}

		if d.ID == "" {
			d.ID = uuid.New().String()
		}
		d.CreatedAt = now
		d.UpdatedAt = now
		d.LastSeen = now
		if d.Status == "" {
			d.Status = DeviceStatusOnline
		}
		if d.State == nil {
			d.State = make(map[string]any)
		}
		if d.Metadata == nil {
			d.Metadata = make(map[string]string)
		}

		m.devices[d.ID] = d
		added = append(added, d)

		m.addEvent(DeviceEvent{
			DeviceID:   d.ID,
			DeviceName: d.Name,
			Type:       "device_discovered",
			Timestamp:  now,
		})
	}

	return added, nil
}

// ============================================================
// Manager 扩展：设备控制
// ============================================================

// ControlDevice 向设备发送控制命令
func (m *Manager) ControlDevice(deviceID string, command map[string]any) error {
	m.mu.RLock()
	device, ok := m.devices[deviceID]
	if !ok {
		m.mu.RUnlock()
		return ErrDeviceNotFound
	}
	protocol := device.Protocol
	m.mu.RUnlock()

	adapter, err := GetAdapter(protocol)
	if err != nil {
		return err
	}

	if err := adapter.SendCommand(deviceID, command); err != nil {
		return fmt.Errorf("send command failed: %w", err)
	}

	// 更新设备状态
	m.mu.Lock()
	if d, ok := m.devices[deviceID]; ok {
		for k, v := range command {
			d.State[k] = v
		}
		now := time.Now()
		d.UpdatedAt = now
		d.LastSeen = now
		d.Status = DeviceStatusOnline

		m.addEvent(DeviceEvent{
			DeviceID:   d.ID,
			DeviceName: d.Name,
			Type:       "command_sent",
			State:      command,
			Timestamp:  now,
		})
	}
	m.mu.Unlock()

	return nil
}

// SyncDeviceState 从设备同步最新状态
func (m *Manager) SyncDeviceState(deviceID string) error {
	m.mu.RLock()
	device, ok := m.devices[deviceID]
	if !ok {
		m.mu.RUnlock()
		return ErrDeviceNotFound
	}
	protocol := device.Protocol
	m.mu.RUnlock()

	adapter, err := GetAdapter(protocol)
	if err != nil {
		return err
	}

	state, err := adapter.ReadState(deviceID)
	if err != nil {
		return fmt.Errorf("read state failed: %w", err)
	}

	return m.UpdateDeviceState(deviceID, state)
}

// ============================================================
// Manager 扩展：搜索与过滤
// ============================================================

// DeviceFilter 设备过滤条件
type DeviceFilter struct {
	Name     string       `json:"name,omitempty"`      // 模糊匹配
	Type     DeviceType   `json:"type,omitempty"`
	Protocol Protocol     `json:"protocol,omitempty"`
	RoomID   string       `json:"room_id,omitempty"`
	Status   DeviceStatus `json:"status,omitempty"`
}

// SearchDevices 按条件搜索设备
func (m *Manager) SearchDevices(filter DeviceFilter) []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Device, 0)
	for _, d := range m.devices {
		if filter.Name != "" && !containsIgnoreCase(d.Name, filter.Name) {
			continue
		}
		if filter.Type != "" && d.Type != filter.Type {
			continue
		}
		if filter.Protocol != "" && d.Protocol != filter.Protocol {
			continue
		}
		if filter.RoomID != "" && d.RoomID != filter.RoomID {
			continue
		}
		if filter.Status != "" && d.Status != filter.Status {
			continue
		}
		result = append(result, d)
	}
	return result
}

// containsIgnoreCase 不区分大小写检查子串
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(toLower(s), toLower(substr))
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// ============================================================
// Manager 扩展：场景历史与统计
// ============================================================

// SceneExecution 场景执行记录
type SceneExecution struct {
	SceneID   string    `json:"scene_id"`
	SceneName string    `json:"scene_name"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
	Duration  int64     `json:"duration_ms"`
	Timestamp time.Time `json:"timestamp"`
}

// GetSceneHistory 获取场景执行历史（从事件中提取）
func (m *Manager) GetSceneHistory(limit int) []SceneExecution {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := make([]SceneExecution, 0)
	for i := len(m.events) - 1; i >= 0 && len(history) < limit; i-- {
		ev := m.events[i]
		if ev.Type == "scene_action" || ev.Type == "scene_executed" {
			history = append(history, SceneExecution{
				SceneID:   ev.DeviceID,
				SceneName: ev.DeviceName,
				Success:   true,
				Timestamp: ev.Timestamp,
			})
		}
	}
	return history
}

// ============================================================
// Manager 扩展：统计概览
// ============================================================

// ProtocolStats 协议统计
type ProtocolStats struct {
	Protocol Protocol `json:"protocol"`
	Count    int      `json:"count"`
	Online   int      `json:"online"`
}

// GetProtocolStats 获取各协议设备统计
func (m *Manager) GetProtocolStats() []ProtocolStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statsMap := make(map[Protocol]*ProtocolStats)
	for _, d := range m.devices {
		s, ok := statsMap[d.Protocol]
		if !ok {
			s = &ProtocolStats{Protocol: d.Protocol}
			statsMap[d.Protocol] = s
		}
		s.Count++
		if d.Status == DeviceStatusOnline {
			s.Online++
		}
	}

	result := make([]ProtocolStats, 0, len(statsMap))
	for _, s := range statsMap {
		result = append(result, *s)
	}
	return result
}

// GetRoomStats 获取房间设备统计
func (m *Manager) GetRoomStats() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]int)
	for _, d := range m.devices {
		if d.RoomID != "" {
			stats[d.RoomID]++
		}
	}
	return stats
}

// ============================================================
// 设备分组批量操作
// ============================================================

// ExecuteGroup 执行分组内所有设备的控制命令
func (m *Manager) ExecuteGroup(groupID string, state map[string]any) error {
	m.mu.RLock()
	group, ok := m.groups[groupID]
	if !ok {
		m.mu.RUnlock()
		return ErrGroupNotFound
	}
	deviceIDs := make([]string, len(group.DeviceIDs))
	copy(deviceIDs, group.DeviceIDs)
	m.mu.RUnlock()

	errs := m.BatchControlDevice(deviceIDs, state)
	if len(errs) > 0 {
		return fmt.Errorf("batch control failed for %d devices: %v", len(errs), errs[0])
	}
	return nil
}

// ============================================================
// Manager 扩展：设备能力查询
// ============================================================

// GetDeviceCapabilities 获取设备支持的能力列表
func (m *Manager) GetDeviceCapabilities(deviceID string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return nil, ErrDeviceNotFound
	}
	return device.Capabilities, nil
}

// SetDeviceCapabilities 设置设备能力
func (m *Manager) SetDeviceCapabilities(deviceID string, capabilities []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return ErrDeviceNotFound
	}

	device.Capabilities = capabilities
	device.UpdatedAt = time.Now()
	return nil
}

// ============================================================
// 错误补充
// ============================================================

var (
	ErrAdapterNotFound = errors.New("adapter not found")
)
