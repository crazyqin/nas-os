package smarthome

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// 错误定义
// ============================================================

var (
	ErrDeviceNotFound   = errors.New("device not found")
	ErrRoomNotFound     = errors.New("room not found")
	ErrGroupNotFound    = errors.New("group not found")
	ErrDeviceExists     = errors.New("device already exists")
	ErrRoomExists       = errors.New("room already exists")
	ErrGroupExists      = errors.New("group already exists")
	ErrInvalidProtocol  = errors.New("invalid protocol")
	ErrInvalidDeviceType = errors.New("invalid device type")
)

// ============================================================
// 设备管理
// ============================================================

// AddDevice 添加设备
func (m *Manager) AddDevice(device *Device) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if device.ID == "" {
		device.ID = uuid.New().String()
	}

	if _, exists := m.devices[device.ID]; exists {
		return ErrDeviceExists
	}

	// 验证协议
	switch device.Protocol {
	case ProtocolMQTT, ProtocolHTTP, ProtocolZigbee:
		// valid
	default:
		return ErrInvalidProtocol
	}

	// 验证设备类型
	switch device.Type {
	case DeviceTypeLight, DeviceTypeSwitch, DeviceTypeSensor, DeviceTypeThermostat,
		DeviceTypeCamera, DeviceTypeLock, DeviceTypePlug, DeviceTypeFan,
		DeviceTypeCurtain, DeviceTypeSpeaker, DeviceTypeOther:
		// valid
	default:
		return ErrInvalidDeviceType
	}

	now := time.Now()
	device.CreatedAt = now
	device.UpdatedAt = now
	device.LastSeen = now
	if device.Status == "" {
		device.Status = DeviceStatusUnknown
	}
	if device.State == nil {
		device.State = make(map[string]any)
	}
	if device.Metadata == nil {
		device.Metadata = make(map[string]string)
	}

	m.devices[device.ID] = device

	// 添加到房间
	if device.RoomID != "" {
		if room, ok := m.rooms[device.RoomID]; ok {
			room.DeviceIDs = append(room.DeviceIDs, device.ID)
			room.UpdatedAt = now
		}
	}

	// 添加到分组
	for _, gid := range device.GroupIDs {
		if group, ok := m.groups[gid]; ok {
			group.DeviceIDs = append(group.DeviceIDs, device.ID)
			group.UpdatedAt = now
		}
	}

	// 记录事件
	m.addEvent(DeviceEvent{
		DeviceID:   device.ID,
		DeviceName: device.Name,
		Type:       "device_added",
		Timestamp:  now,
	})

	return nil
}

// GetDevice 获取设备
func (m *Manager) GetDevice(id string) (*Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[id]
	if !ok {
		return nil, ErrDeviceNotFound
	}
	return device, nil
}

// ListDevices 列出所有设备
func (m *Manager) ListDevices() []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*Device, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, d)
	}
	return devices
}

// ListDevicesByRoom 按房间列出设备
func (m *Manager) ListDevicesByRoom(roomID string) []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	room, ok := m.rooms[roomID]
	if !ok {
		return nil
	}

	devices := make([]*Device, 0, len(room.DeviceIDs))
	for _, id := range room.DeviceIDs {
		if d, ok := m.devices[id]; ok {
			devices = append(devices, d)
		}
	}
	return devices
}

// ListDevicesByType 按类型列出设备
func (m *Manager) ListDevicesByType(deviceType DeviceType) []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*Device, 0)
	for _, d := range m.devices {
		if d.Type == deviceType {
			devices = append(devices, d)
		}
	}
	return devices
}

// UpdateDevice 更新设备信息
func (m *Manager) UpdateDevice(id string, update *Device) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[id]
	if !ok {
		return ErrDeviceNotFound
	}

	if update.Name != "" {
		device.Name = update.Name
	}
	if update.RoomID != "" {
		// 从旧房间移除
		if device.RoomID != "" && device.RoomID != update.RoomID {
			if oldRoom, ok := m.rooms[device.RoomID]; ok {
				for i, did := range oldRoom.DeviceIDs {
					if did == id {
						oldRoom.DeviceIDs = append(oldRoom.DeviceIDs[:i], oldRoom.DeviceIDs[i+1:]...)
						break
					}
				}
			}
		}
		device.RoomID = update.RoomID
		// 添加到新房间
		if room, ok := m.rooms[update.RoomID]; ok {
			room.DeviceIDs = append(room.DeviceIDs, id)
			room.UpdatedAt = time.Now()
		}
	}
	if update.IPAddress != "" {
		device.IPAddress = update.IPAddress
	}
	if update.Metadata != nil {
		for k, v := range update.Metadata {
			device.Metadata[k] = v
		}
	}

	device.UpdatedAt = time.Now()
	return nil
}

// DeleteDevice 删除设备
func (m *Manager) DeleteDevice(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[id]
	if !ok {
		return ErrDeviceNotFound
	}

	// 从房间移除
	if device.RoomID != "" {
		if room, ok := m.rooms[device.RoomID]; ok {
			for i, did := range room.DeviceIDs {
				if did == id {
					room.DeviceIDs = append(room.DeviceIDs[:i], room.DeviceIDs[i+1:]...)
					break
				}
			}
		}
	}

	// 从分组移除
	for _, gid := range device.GroupIDs {
		if group, ok := m.groups[gid]; ok {
			for i, did := range group.DeviceIDs {
				if did == id {
					group.DeviceIDs = append(group.DeviceIDs[:i], group.DeviceIDs[i+1:]...)
					break
				}
			}
		}
	}

	// 清理能耗数据
	delete(m.energyData, id)

	delete(m.devices, id)

	m.addEvent(DeviceEvent{
		DeviceID:   id,
		DeviceName: device.Name,
		Type:       "device_removed",
		Timestamp:  time.Now(),
	})

	return nil
}

// UpdateDeviceState 更新设备状态
func (m *Manager) UpdateDeviceState(id string, state map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[id]
	if !ok {
		return ErrDeviceNotFound
	}

	now := time.Now()
	device.State = state
	device.LastSeen = now
	device.UpdatedAt = now
	device.Status = DeviceStatusOnline

	m.addEvent(DeviceEvent{
		DeviceID:   id,
		DeviceName: device.Name,
		Type:       "state_changed",
		State:      state,
		Timestamp:  now,
	})

	return nil
}

// SetDeviceOnline 设置设备在线状态
func (m *Manager) SetDeviceOnline(id string, online bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[id]
	if !ok {
		return ErrDeviceNotFound
	}

	now := time.Now()
	if online {
		device.Status = DeviceStatusOnline
		device.LastSeen = now
	} else {
		device.Status = DeviceStatusOffline
	}
	device.UpdatedAt = now

	return nil
}

// ============================================================
// 房间管理
// ============================================================

// AddRoom 添加房间
func (m *Manager) AddRoom(room *Room) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if room.ID == "" {
		room.ID = uuid.New().String()
	}

	if _, exists := m.rooms[room.ID]; exists {
		return ErrRoomExists
	}

	now := time.Now()
	room.CreatedAt = now
	room.UpdatedAt = now
	if room.DeviceIDs == nil {
		room.DeviceIDs = make([]string, 0)
	}

	m.rooms[room.ID] = room
	return nil
}

// GetRoom 获取房间
func (m *Manager) GetRoom(id string) (*Room, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	room, ok := m.rooms[id]
	if !ok {
		return nil, ErrRoomNotFound
	}
	return room, nil
}

// ListRooms 列出所有房间
func (m *Manager) ListRooms() []*Room {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rooms := make([]*Room, 0, len(m.rooms))
	for _, r := range m.rooms {
		rooms = append(rooms, r)
	}
	return rooms
}

// UpdateRoom 更新房间
func (m *Manager) UpdateRoom(id string, update *Room) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[id]
	if !ok {
		return ErrRoomNotFound
	}

	if update.Name != "" {
		room.Name = update.Name
	}
	if update.Icon != "" {
		room.Icon = update.Icon
	}

	room.UpdatedAt = time.Now()
	return nil
}

// DeleteRoom 删除房间
func (m *Manager) DeleteRoom(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[id]
	if !ok {
		return ErrRoomNotFound
	}

	// 将房间内的设备移出
	for _, did := range room.DeviceIDs {
		if device, ok := m.devices[did]; ok {
			device.RoomID = ""
			device.UpdatedAt = time.Now()
		}
	}

	delete(m.rooms, id)
	return nil
}

// ============================================================
// 分组管理
// ============================================================

// AddGroup 添加设备分组
func (m *Manager) AddGroup(group *Group) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if group.ID == "" {
		group.ID = uuid.New().String()
	}

	if _, exists := m.groups[group.ID]; exists {
		return ErrGroupExists
	}

	now := time.Now()
	group.CreatedAt = now
	group.UpdatedAt = now
	if group.DeviceIDs == nil {
		group.DeviceIDs = make([]string, 0)
	}

	m.groups[group.ID] = group
	return nil
}

// GetGroup 获取分组
func (m *Manager) GetGroup(id string) (*Group, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	group, ok := m.groups[id]
	if !ok {
		return nil, ErrGroupNotFound
	}
	return group, nil
}

// ListGroups 列出所有分组
func (m *Manager) ListGroups() []*Group {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make([]*Group, 0, len(m.groups))
	for _, g := range m.groups {
		groups = append(groups, g)
	}
	return groups
}

// UpdateGroup 更新分组
func (m *Manager) UpdateGroup(id string, update *Group) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.groups[id]
	if !ok {
		return ErrGroupNotFound
	}

	if update.Name != "" {
		group.Name = update.Name
	}
	if update.RoomID != "" {
		group.RoomID = update.RoomID
	}

	group.UpdatedAt = time.Now()
	return nil
}

// DeleteGroup 删除分组
func (m *Manager) DeleteGroup(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.groups[id]
	if !ok {
		return ErrGroupNotFound
	}

	// 从设备中移除分组引用
	for _, did := range group.DeviceIDs {
		if device, ok := m.devices[did]; ok {
			for i, gid := range device.GroupIDs {
				if gid == id {
					device.GroupIDs = append(device.GroupIDs[:i], device.GroupIDs[i+1:]...)
					break
				}
			}
			device.UpdatedAt = time.Now()
		}
	}

	delete(m.groups, id)
	return nil
}

// AddDeviceToGroup 将设备添加到分组
func (m *Manager) AddDeviceToGroup(deviceID, groupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return ErrDeviceNotFound
	}

	group, ok := m.groups[groupID]
	if !ok {
		return ErrGroupNotFound
	}

	// 检查是否已在分组中
	for _, gid := range device.GroupIDs {
		if gid == groupID {
			return nil // 已存在
		}
	}

	device.GroupIDs = append(device.GroupIDs, groupID)
	device.UpdatedAt = time.Now()

	group.DeviceIDs = append(group.DeviceIDs, deviceID)
	group.UpdatedAt = time.Now()

	return nil
}

// RemoveDeviceFromGroup 从分组移除设备
func (m *Manager) RemoveDeviceFromGroup(deviceID, groupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return ErrDeviceNotFound
	}

	group, ok := m.groups[groupID]
	if !ok {
		return ErrGroupNotFound
	}

	// 从设备移除分组
	for i, gid := range device.GroupIDs {
		if gid == groupID {
			device.GroupIDs = append(device.GroupIDs[:i], device.GroupIDs[i+1:]...)
			break
		}
	}
	device.UpdatedAt = time.Now()

	// 从分组移除设备
	for i, did := range group.DeviceIDs {
		if did == deviceID {
			group.DeviceIDs = append(group.DeviceIDs[:i], group.DeviceIDs[i+1:]...)
			break
		}
	}
	group.UpdatedAt = time.Now()

	return nil
}

// ============================================================
// 辅助方法
// ============================================================

// addEvent 添加设备事件（需在锁内调用）
func (m *Manager) addEvent(event DeviceEvent) {
	m.events = append(m.events, event)
	// 超过最大事件数时裁剪
	if len(m.events) > m.config.MaxEvents {
		m.events = m.events[len(m.events)-m.config.MaxEvents:]
	}
}

// GetEvents 获取最近的设备事件
func (m *Manager) GetEvents(limit int) []DeviceEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.events) {
		limit = len(m.events)
	}

	start := len(m.events) - limit
	events := make([]DeviceEvent, limit)
	copy(events, m.events[start:])
	return events
}

// DiscoverDevices 模拟设备发现
func (m *Manager) DiscoverDevices() []*Device {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 模拟发现新设备
	discovered := make([]*Device, 0)

	// 检查离线设备
	now := time.Now()
	for _, device := range m.devices {
		if device.Status == DeviceStatusOffline {
			// 简单模拟：随机将一些离线设备标记为在线
			if now.Sub(device.LastSeen) < 24*time.Hour {
				device.Status = DeviceStatusOnline
				discovered = append(discovered, device)
			}
		}
	}

	return discovered
}

// GetDeviceCount 获取设备数量统计
func (m *Manager) GetDeviceCount() (total, online, offline int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, d := range m.devices {
		total++
		switch d.Status {
		case DeviceStatusOnline:
			online++
		case DeviceStatusOffline:
			offline++
		}
	}
	return
}

// String 返回设备类型的字符串表示
func (dt DeviceType) String() string {
	return string(dt)
}

// String 返回协议的字符串表示
func (p Protocol) String() string {
	return string(p)
}

// Validate 验证设备配置
func (d *Device) Validate() error {
	if d.Name == "" {
		return fmt.Errorf("device name is required")
	}
	if d.Protocol == "" {
		return fmt.Errorf("device protocol is required")
	}
	if d.Type == "" {
		return fmt.Errorf("device type is required")
	}
	return nil
}
