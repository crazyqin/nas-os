package matterhub

import (
	"errors"
	"fmt"
	"math/rand"
	"time"
)

// ============================================================
// 错误定义
// ============================================================

var (
	ErrDeviceNotFound       = errors.New("device not found")
	ErrDeviceExists         = errors.New("device already exists")
	ErrBRNotFound           = errors.New("border router not found")
	ErrBRExists             = errors.New("border router already exists")
	ErrSceneNotFound        = errors.New("scene not found")
	ErrSceneExists          = errors.New("scene already exists")
	ErrSceneDisabled        = errors.New("scene is disabled")
	ErrGroupNotFound        = errors.New("group not found")
	ErrGroupExists          = errors.New("group already exists")
	ErrInvalidDeviceType    = errors.New("invalid device type")
	ErrInvalidTrigger       = errors.New("invalid trigger")
	ErrInvalidAction        = errors.New("invalid action")
	ErrCommissionFailed     = errors.New("commission failed")
	ErrCommissionTimeout    = errors.New("commission timeout")
	ErrUnsupportedOperation = errors.New("unsupported operation")
	ErrInvalidAttribute     = errors.New("invalid attribute value")
)

// ============================================================
// 中枢生命周期
// ============================================================

// Start 启动中枢
func (h *Hub) Start() error {
	if !h.config.Enabled {
		return nil
	}
	if h.config.DiscoveryEnabled {
		go h.runDiscovery()
	}
	go h.runAutomationEngine()
	return nil
}

// Stop 停止中枢
func (h *Hub) Stop() {
	h.cancel()
}

func (h *Hub) runDiscovery() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			// placeholder for mDNS/Thread discovery
		}
	}
}

func (h *Hub) runAutomationEngine() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.evaluateAutomations()
		}
	}
}

// ============================================================
// Matter 设备配对
// ============================================================

// CommissionDevice 配对 Matter 设备
func (h *Hub) CommissionDevice(req CommissionRequest) (*CommissionResult, error) {
	if req.SetupCode == "" && req.SetupPIN == 0 {
		return nil, fmt.Errorf("setup_code or setup_pin is required")
	}

	timeout := req.TimeoutSec
	if timeout <= 0 {
		timeout = h.config.CommissionTimeoutSec
	}
	if timeout <= 0 {
		timeout = 120
	}

	taskID := fmt.Sprintf("commission_%d_%d", req.Discriminator, time.Now().UnixNano())
	result := &CommissionResult{
		Status:    CommissionStatusInProgress,
		StartedAt: time.Now(),
	}

	h.mu.Lock()
	h.commissionTasks[taskID] = result
	h.mu.Unlock()

	// 模拟配对流程
	nodeID := rand.Uint64()%10000 + 1000
	deviceID := fmt.Sprintf("matter_%04x_%04x_%d", 0, 0, nodeID)
	now := time.Now()
	result.Status = CommissionStatusSuccess
	result.DeviceID = deviceID
	result.NodeID = nodeID
	result.EndedAt = &now

	// 自动创建设备记录
	device := &MatterDevice{
		ID:           deviceID,
		Name:         fmt.Sprintf("Matter Device %d", nodeID),
		Type:         DeviceTypeOther,
		State:        DeviceStateOnline,
		NodeID:       nodeID,
		Attributes:   make(map[string]any),
		CommissionedAt: &now,
	}
	h.mu.Lock()
	h.devices[deviceID] = device
	h.addEventLocked(DeviceEvent{
		DeviceID:  deviceID,
		Type:      "device_commissioned",
		Timestamp: now,
	})
	h.mu.Unlock()

	return result, nil
}

// GetCommissionStatus 获取配对状态
func (h *Hub) GetCommissionStatus(taskID string) (*CommissionResult, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result, ok := h.commissionTasks[taskID]
	if !ok {
		return nil, fmt.Errorf("commission task not found: %s", taskID)
	}
	return result, nil
}

// DecommissionDevice 移除已配对设备
func (h *Hub) DecommissionDevice(deviceID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	device, ok := h.devices[deviceID]
	if !ok {
		return ErrDeviceNotFound
	}

	// 从分组移除
	for _, gid := range device.GroupIDs {
		if group, ok := h.groups[gid]; ok {
			for i, did := range group.DeviceIDs {
				if did == deviceID {
					group.DeviceIDs = append(group.DeviceIDs[:i], group.DeviceIDs[i+1:]...)
					break
				}
			}
		}
	}

	delete(h.devices, deviceID)
	h.addEventLocked(DeviceEvent{
		DeviceID:  deviceID,
		Type:      "device_decommissioned",
		Timestamp: time.Now(),
	})
	return nil
}

// ============================================================
// Thread 边界路由器管理
// ============================================================

// AddBorderRouter 添加 Thread 边界路由器
func (h *Hub) AddBorderRouter(br *ThreadBorderRouter) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if br.ID == "" {
		br.ID = fmt.Sprintf("tbr_%d", time.Now().UnixNano())
	}
	if _, exists := h.borderRouters[br.ID]; exists {
		return ErrBRExists
	}

	now := time.Now()
	br.CreatedAt = now
	br.UpdatedAt = now
	if br.LastSeen.IsZero() {
		br.LastSeen = now
	}

	h.borderRouters[br.ID] = br
	h.addEventLocked(DeviceEvent{
		DeviceID:  br.ID,
		Type:      "border_router_added",
		Timestamp: now,
	})
	return nil
}

// GetBorderRouter 获取边界路由器
func (h *Hub) GetBorderRouter(id string) (*ThreadBorderRouter, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	br, ok := h.borderRouters[id]
	if !ok {
		return nil, ErrBRNotFound
	}
	return br, nil
}

// ListBorderRouters 列出所有边界路由器
func (h *Hub) ListBorderRouters() []*ThreadBorderRouter {
	h.mu.RLock()
	defer h.mu.RUnlock()
	brs := make([]*ThreadBorderRouter, 0, len(h.borderRouters))
	for _, br := range h.borderRouters {
		brs = append(brs, br)
	}
	return brs
}

// UpdateBorderRouter 更新边界路由器
func (h *Hub) UpdateBorderRouter(id string, update *ThreadBorderRouter) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	br, ok := h.borderRouters[id]
	if !ok {
		return ErrBRNotFound
	}

	if update.Name != "" {
		br.Name = update.Name
	}
	if update.State != "" {
		br.State = update.State
	}
	if update.IPAddress != "" {
		br.IPAddress = update.IPAddress
	}
	if update.FirmwareVersion != "" {
		br.FirmwareVersion = update.FirmwareVersion
	}
	br.IsActive = update.IsActive
	br.ChildCount = update.ChildCount
	br.LastSeen = time.Now()
	br.UpdatedAt = time.Now()

	return nil
}

// DeleteBorderRouter 删除边界路由器
func (h *Hub) DeleteBorderRouter(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.borderRouters[id]; !ok {
		return ErrBRNotFound
	}
	delete(h.borderRouters, id)

	h.addEventLocked(DeviceEvent{
		DeviceID:  id,
		Type:      "border_router_removed",
		Timestamp: time.Now(),
	})
	return nil
}

// ============================================================
// 设备管理
// ============================================================

// AddDevice 添加设备
func (h *Hub) AddDevice(device *MatterDevice) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if device.ID == "" {
		device.ID = fmt.Sprintf("matter_%d", time.Now().UnixNano())
	}
	if _, exists := h.devices[device.ID]; exists {
		return ErrDeviceExists
	}

	now := time.Now()
	device.CreatedAt = now
	device.UpdatedAt = now
	device.LastSeen = now
	if device.State == "" {
		device.State = DeviceStateUnknown
	}
	if device.Attributes == nil {
		device.Attributes = make(map[string]any)
	}

	h.devices[device.ID] = device
	h.addEventLocked(DeviceEvent{
		DeviceID:  device.ID,
		Type:      "device_added",
		Timestamp: now,
	})
	return nil
}

// GetDevice 获取设备
func (h *Hub) GetDevice(id string) (*MatterDevice, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	device, ok := h.devices[id]
	if !ok {
		return nil, ErrDeviceNotFound
	}
	return device, nil
}

// ListDevices 列出所有设备
func (h *Hub) ListDevices() []*MatterDevice {
	h.mu.RLock()
	defer h.mu.RUnlock()
	devices := make([]*MatterDevice, 0, len(h.devices))
	for _, d := range h.devices {
		devices = append(devices, d)
	}
	return devices
}

// ListDevicesByType 按类型列出设备
func (h *Hub) ListDevicesByType(deviceType DeviceType) []*MatterDevice {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var devices []*MatterDevice
	for _, d := range h.devices {
		if d.Type == deviceType {
			devices = append(devices, d)
		}
	}
	return devices
}

// ListDevicesByRoom 按房间列出设备
func (h *Hub) ListDevicesByRoom(roomID string) []*MatterDevice {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var devices []*MatterDevice
	for _, d := range h.devices {
		if d.RoomID == roomID {
			devices = append(devices, d)
		}
	}
	return devices
}

// UpdateDevice 更新设备信息
func (h *Hub) UpdateDevice(id string, update *MatterDevice) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	device, ok := h.devices[id]
	if !ok {
		return ErrDeviceNotFound
	}

	if update.Name != "" {
		device.Name = update.Name
	}
	if update.RoomID != "" {
		device.RoomID = update.RoomID
	}
	if update.IPAddress != "" {
		device.IPAddress = update.IPAddress
	}
	if update.Firmware != "" {
		device.Firmware = update.Firmware
	}
	if update.Manufacturer != "" {
		device.Manufacturer = update.Manufacturer
	}
	if update.Model != "" {
		device.Model = update.Model
	}
	if update.Attributes != nil {
		for k, v := range update.Attributes {
			device.Attributes[k] = v
		}
	}

	device.UpdatedAt = time.Now()
	return nil
}

// DeleteDevice 删除设备
func (h *Hub) DeleteDevice(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	device, ok := h.devices[id]
	if !ok {
		return ErrDeviceNotFound
	}

	// 从分组移除
	for _, gid := range device.GroupIDs {
		if group, ok := h.groups[gid]; ok {
			for i, did := range group.DeviceIDs {
				if did == id {
					group.DeviceIDs = append(group.DeviceIDs[:i], group.DeviceIDs[i+1:]...)
					break
				}
			}
		}
	}

	delete(h.devices, id)
	h.addEventLocked(DeviceEvent{
		DeviceID:  id,
		Type:      "device_removed",
		Timestamp: time.Now(),
	})
	return nil
}

// SetDeviceOnline 设置设备在线状态
func (h *Hub) SetDeviceOnline(id string, online bool) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	device, ok := h.devices[id]
	if !ok {
		return ErrDeviceNotFound
	}

	now := time.Now()
	if online {
		device.State = DeviceStateOnline
		device.LastSeen = now
	} else {
		device.State = DeviceStateOffline
	}
	device.UpdatedAt = now
	return nil
}

// ============================================================
// 设备控制
// ============================================================

// TurnOn 打开设备
func (h *Hub) TurnOn(deviceID string) error {
	return h.setAttribute(deviceID, "on_off", true)
}

// TurnOff 关闭设备
func (h *Hub) TurnOff(deviceID string) error {
	return h.setAttribute(deviceID, "on_off", false)
}

// SetBrightness 设置亮度 (0-254)
func (h *Hub) SetBrightness(deviceID string, level uint8) error {
	if level > 254 {
		return ErrInvalidAttribute
	}
	return h.setAttribute(deviceID, "brightness", level)
}

// SetColor 设置颜色 (Hue 0-360, Saturation 0-254)
func (h *Hub) SetColor(deviceID string, hue uint16, saturation uint8) error {
	if hue > 360 {
		return ErrInvalidAttribute
	}
	if saturation > 254 {
		return ErrInvalidAttribute
	}
	return h.setAttributes(deviceID, map[string]any{
		"color_hue":        hue,
		"color_saturation": saturation,
	})
}

// SetColorTemperature 设置色温 (Kelvin: 1000-10000)
func (h *Hub) SetColorTemperature(deviceID string, kelvin uint16) error {
	if kelvin < 1000 || kelvin > 10000 {
		return ErrInvalidAttribute
	}
	return h.setAttribute(deviceID, "color_temperature", kelvin)
}

// SetTargetTemperature 设置目标温度 (摄氏度 * 100)
func (h *Hub) SetTargetTemperature(deviceID string, temp int16) error {
	return h.setAttribute(deviceID, "target_temperature", temp)
}

// LockDoor 锁门
func (h *Hub) LockDoor(deviceID string) error {
	return h.setAttribute(deviceID, "lock_state", "locked")
}

// UnlockDoor 开门
func (h *Hub) UnlockDoor(deviceID string) error {
	return h.setAttribute(deviceID, "lock_state", "unlocked")
}

// SetWindowPosition 设置窗帘位置 (0-100)
func (h *Hub) SetWindowPosition(deviceID string, position uint8) error {
	if position > 100 {
		return ErrInvalidAttribute
	}
	return h.setAttribute(deviceID, "current_position", position)
}

// SetFanSpeed 设置风扇速度 (0-100)
func (h *Hub) SetFanSpeed(deviceID string, speed uint8) error {
	if speed > 100 {
		return ErrInvalidAttribute
	}
	return h.setAttribute(deviceID, "fan_speed", speed)
}

// GetAttribute 获取设备属性
func (h *Hub) GetAttribute(deviceID, attribute string) (any, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	device, ok := h.devices[deviceID]
	if !ok {
		return nil, ErrDeviceNotFound
	}

	val, ok := device.Attributes[attribute]
	if !ok {
		return nil, fmt.Errorf("attribute %s not found on device %s", attribute, deviceID)
	}
	return val, nil
}

// setAttribute 设置单个属性（需在锁外调用）
func (h *Hub) setAttribute(deviceID, key string, value any) error {
	return h.setAttributes(deviceID, map[string]any{key: value})
}

// setAttributes 批量设置属性
func (h *Hub) setAttributes(deviceID string, attrs map[string]any) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	device, ok := h.devices[deviceID]
	if !ok {
		return ErrDeviceNotFound
	}

	for k, v := range attrs {
		device.Attributes[k] = v
	}
	now := time.Now()
	device.LastSeen = now
	device.UpdatedAt = now
	if device.State == DeviceStateOffline || device.State == DeviceStateUnknown {
		device.State = DeviceStateOnline
	}

	h.addEventLocked(DeviceEvent{
		DeviceID:   deviceID,
		DeviceName: device.Name,
		Type:       "attribute_changed",
		State:      attrs,
		Timestamp:  now,
	})
	return nil
}

// ============================================================
// 场景管理
// ============================================================

// AddScene 添加场景
func (h *Hub) AddScene(scene *Scene) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if scene.ID == "" {
		scene.ID = fmt.Sprintf("scene_%d", time.Now().UnixNano())
	}
	if _, exists := h.scenes[scene.ID]; exists {
		return ErrSceneExists
	}

	if err := validateTrigger(&scene.Trigger); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidTrigger, err)
	}
	for i, a := range scene.Actions {
		if err := validateAction(&a); err != nil {
			return fmt.Errorf("action[%d]: %w: %v", i, ErrInvalidAction, err)
		}
	}

	now := time.Now()
	scene.CreatedAt = now
	scene.UpdatedAt = now
	if !scene.Enabled {
		scene.Enabled = true
	}

	h.scenes[scene.ID] = scene
	return nil
}

// GetScene 获取场景
func (h *Hub) GetScene(id string) (*Scene, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	scene, ok := h.scenes[id]
	if !ok {
		return nil, ErrSceneNotFound
	}
	return scene, nil
}

// ListScenes 列出所有场景
func (h *Hub) ListScenes() []*Scene {
	h.mu.RLock()
	defer h.mu.RUnlock()
	scenes := make([]*Scene, 0, len(h.scenes))
	for _, s := range h.scenes {
		scenes = append(scenes, s)
	}
	return scenes
}

// UpdateScene 更新场景
func (h *Hub) UpdateScene(id string, update *Scene) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	scene, ok := h.scenes[id]
	if !ok {
		return ErrSceneNotFound
	}

	if update.Name != "" {
		scene.Name = update.Name
	}
	if update.Description != "" {
		scene.Description = update.Description
	}
	if update.Trigger.Type != "" {
		if err := validateTrigger(&update.Trigger); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidTrigger, err)
		}
		scene.Trigger = update.Trigger
	}
	if update.Conditions != nil {
		scene.Conditions = update.Conditions
	}
	if update.Actions != nil {
		for i, a := range update.Actions {
			if err := validateAction(&a); err != nil {
				return fmt.Errorf("action[%d]: %w: %v", i, ErrInvalidAction, err)
			}
		}
		scene.Actions = update.Actions
	}
	scene.Enabled = update.Enabled
	scene.UpdatedAt = time.Now()
	return nil
}

// DeleteScene 删除场景
func (h *Hub) DeleteScene(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.scenes[id]; !ok {
		return ErrSceneNotFound
	}
	// 清理关联的自动化
	for _, a := range h.automations {
		for _, act := range a.Actions {
			if act.Type == ActionTypeScene && act.SceneID == id {
				a.Enabled = false
				break
			}
		}
	}
	delete(h.scenes, id)
	return nil
}

// ActivateScene 激活/执行场景
func (h *Hub) ActivateScene(id string) error {
	h.mu.Lock()
	scene, ok := h.scenes[id]
	if !ok {
		h.mu.Unlock()
		return ErrSceneNotFound
	}
	if !scene.Enabled {
		h.mu.Unlock()
		return ErrSceneDisabled
	}

	// 评估条件
	if !h.evaluateConditionsLocked(scene.Conditions) {
		h.mu.Unlock()
		return nil
	}

	now := time.Now()
	scene.LastRun = &now
	scene.RunCount++
	actions := scene.Actions
	h.mu.Unlock()

	return h.executeActions(actions)
}

// EnableScene 启用场景
func (h *Hub) EnableScene(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	scene, ok := h.scenes[id]
	if !ok {
		return ErrSceneNotFound
	}
	scene.Enabled = true
	scene.UpdatedAt = time.Now()
	return nil
}

// DisableScene 禁用场景
func (h *Hub) DisableScene(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	scene, ok := h.scenes[id]
	if !ok {
		return ErrSceneNotFound
	}
	scene.Enabled = false
	scene.UpdatedAt = time.Now()
	return nil
}

// ============================================================
// 自动化规则引擎
// ============================================================

// AddAutomation 添加自动化规则
func (h *Hub) AddAutomation(auto *Automation) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if auto.ID == "" {
		auto.ID = fmt.Sprintf("auto_%d", time.Now().UnixNano())
	}
	if _, exists := h.automations[auto.ID]; exists {
		return ErrSceneExists // 复用场景错误
	}

	if err := validateTrigger(&auto.Trigger); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidTrigger, err)
	}
	for i, a := range auto.Actions {
		if err := validateAction(&a); err != nil {
			return fmt.Errorf("action[%d]: %w: %v", i, ErrInvalidAction, err)
		}
	}

	now := time.Now()
	auto.CreatedAt = now
	auto.UpdatedAt = now
	if !auto.Enabled {
		auto.Enabled = true
	}

	h.automations[auto.ID] = auto
	return nil
}

// GetAutomation 获取自动化规则
func (h *Hub) GetAutomation(id string) (*Automation, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	auto, ok := h.automations[id]
	if !ok {
		return nil, ErrSceneNotFound
	}
	return auto, nil
}

// ListAutomations 列出所有自动化规则
func (h *Hub) ListAutomations() []*Automation {
	h.mu.RLock()
	defer h.mu.RUnlock()
	autos := make([]*Automation, 0, len(h.automations))
	for _, a := range h.automations {
		autos = append(autos, a)
	}
	return autos
}

// UpdateAutomation 更新自动化规则
func (h *Hub) UpdateAutomation(id string, update *Automation) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	auto, ok := h.automations[id]
	if !ok {
		return ErrSceneNotFound
	}

	if update.Name != "" {
		auto.Name = update.Name
	}
	if update.Description != "" {
		auto.Description = update.Description
	}
	if update.Trigger.Type != "" {
		if err := validateTrigger(&update.Trigger); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidTrigger, err)
		}
		auto.Trigger = update.Trigger
	}
	if update.Conditions != nil {
		auto.Conditions = update.Conditions
	}
	if update.Actions != nil {
		for i, a := range update.Actions {
			if err := validateAction(&a); err != nil {
				return fmt.Errorf("action[%d]: %w: %v", i, ErrInvalidAction, err)
			}
		}
		auto.Actions = update.Actions
	}
	auto.Enabled = update.Enabled
	auto.UpdatedAt = time.Now()
	return nil
}

// DeleteAutomation 删除自动化规则
func (h *Hub) DeleteAutomation(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.automations[id]; !ok {
		return ErrSceneNotFound
	}
	delete(h.automations, id)
	return nil
}

// evaluateAutomations 评估所有自动化规则
func (h *Hub) evaluateAutomations() {
	h.mu.RLock()
	var toExecute []Action
	for _, auto := range h.automations {
		if !auto.Enabled {
			continue
		}
		if !h.evaluateConditionsLocked(auto.Conditions) {
			continue
		}
		toExecute = append(toExecute, auto.Actions...)
		// 更新统计
		now := time.Now()
		auto.LastRun = &now
		auto.RunCount++
	}
	h.mu.RUnlock()

	if len(toExecute) > 0 {
		h.executeActions(toExecute)
	}
}

// evaluateConditionsLocked 评估条件列表（需持有锁）
func (h *Hub) evaluateConditionsLocked(conditions []Condition) bool {
	for _, cond := range conditions {
		device, ok := h.devices[cond.DeviceID]
		if !ok {
			return false
		}
		current, ok := device.Attributes[cond.Field]
		if !ok {
			return false
		}
		if !compareValues(current, cond.Value, cond.Operator) {
			return false
		}
	}
	return true
}

// ============================================================
// 设备分组管理
// ============================================================

// AddGroup 添加设备分组
func (h *Hub) AddGroup(group *DeviceGroup) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if group.ID == "" {
		group.ID = fmt.Sprintf("group_%d", time.Now().UnixNano())
	}
	if _, exists := h.groups[group.ID]; exists {
		return ErrGroupExists
	}

	now := time.Now()
	group.CreatedAt = now
	group.UpdatedAt = now
	if group.DeviceIDs == nil {
		group.DeviceIDs = make([]string, 0)
	}
	h.groups[group.ID] = group
	return nil
}

// GetGroup 获取分组
func (h *Hub) GetGroup(id string) (*DeviceGroup, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	group, ok := h.groups[id]
	if !ok {
		return nil, ErrGroupNotFound
	}
	return group, nil
}

// ListGroups 列出所有分组
func (h *Hub) ListGroups() []*DeviceGroup {
	h.mu.RLock()
	defer h.mu.RUnlock()
	groups := make([]*DeviceGroup, 0, len(h.groups))
	for _, g := range h.groups {
		groups = append(groups, g)
	}
	return groups
}

// UpdateGroup 更新分组
func (h *Hub) UpdateGroup(id string, update *DeviceGroup) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	group, ok := h.groups[id]
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
func (h *Hub) DeleteGroup(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	group, ok := h.groups[id]
	if !ok {
		return ErrGroupNotFound
	}

	// 从设备中移除分组引用
	for _, did := range group.DeviceIDs {
		if device, ok := h.devices[did]; ok {
			for i, gid := range device.GroupIDs {
				if gid == id {
					device.GroupIDs = append(device.GroupIDs[:i], device.GroupIDs[i+1:]...)
					break
				}
			}
		}
	}

	delete(h.groups, id)
	return nil
}

// AddDeviceToGroup 将设备添加到分组
func (h *Hub) AddDeviceToGroup(deviceID, groupID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	device, ok := h.devices[deviceID]
	if !ok {
		return ErrDeviceNotFound
	}
	group, ok := h.groups[groupID]
	if !ok {
		return ErrGroupNotFound
	}

	for _, gid := range device.GroupIDs {
		if gid == groupID {
			return nil
		}
	}

	device.GroupIDs = append(device.GroupIDs, groupID)
	device.UpdatedAt = time.Now()
	group.DeviceIDs = append(group.DeviceIDs, deviceID)
	group.UpdatedAt = time.Now()
	return nil
}

// RemoveDeviceFromGroup 从分组移除设备
func (h *Hub) RemoveDeviceFromGroup(deviceID, groupID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	device, ok := h.devices[deviceID]
	if !ok {
		return ErrDeviceNotFound
	}
	group, ok := h.groups[groupID]
	if !ok {
		return ErrGroupNotFound
	}

	for i, gid := range device.GroupIDs {
		if gid == groupID {
			device.GroupIDs = append(device.GroupIDs[:i], device.GroupIDs[i+1:]...)
			break
		}
	}
	device.UpdatedAt = time.Now()

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
// 事件与统计
// ============================================================

// addEventLocked 记录事件（需持有写锁）
func (h *Hub) addEventLocked(event DeviceEvent) {
	h.events = append(h.events, event)
	if len(h.events) > h.config.MaxEvents {
		h.events = h.events[len(h.events)-h.config.MaxEvents:]
	}
}

// GetEvents 获取最近事件
func (h *Hub) GetEvents(limit int) []DeviceEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if limit <= 0 || limit > len(h.events) {
		limit = len(h.events)
	}
	start := len(h.events) - limit
	events := make([]DeviceEvent, limit)
	copy(events, h.events[start:])
	return events
}

// GetDeviceCount 获取设备统计
func (h *Hub) GetDeviceCount() (total, online, offline int) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, d := range h.devices {
		total++
		switch d.State {
		case DeviceStateOnline:
			online++
		case DeviceStateOffline:
			offline++
		}
	}
	return
}

// GetDashboardSummary 获取仪表盘摘要
func (h *Hub) GetDashboardSummary() DashboardSummary {
	h.mu.RLock()
	defer h.mu.RUnlock()

	summary := DashboardSummary{
		DevicesByType: make(map[string]int),
		DevicesByRoom: make(map[string]int),
		UpdatedAt:     time.Now(),
	}

	for _, d := range h.devices {
		summary.TotalDevices++
		switch d.State {
		case DeviceStateOnline:
			summary.OnlineDevices++
		case DeviceStateOffline:
			summary.OfflineDevices++
		}
		summary.DevicesByType[string(d.Type)]++
		if d.RoomID != "" {
			summary.DevicesByRoom[d.RoomID]++
		}
	}

	summary.TotalBRs = len(h.borderRouters)
	for _, br := range h.borderRouters {
		if br.IsActive {
			summary.ActiveBRs++
		}
	}

	summary.TotalScenes = len(h.scenes)
	for _, s := range h.scenes {
		if s.Enabled {
			summary.ActiveScenes++
		}
	}

	summary.TotalGroups = len(h.groups)

	// 最近 10 条事件
	limit := 10
	if limit > len(h.events) {
		limit = len(h.events)
	}
	start := len(h.events) - limit
	summary.RecentEvents = make([]DeviceEvent, limit)
	copy(summary.RecentEvents, h.events[start:])

	return summary
}

// ============================================================
// 辅助函数
// ============================================================

// compareValues 比较值
func compareValues(current, expected any, op ComparisonOperator) bool {
	switch op {
	case OpEqual:
		return fmt.Sprintf("%v", current) == fmt.Sprintf("%v", expected)
	case OpNotEqual:
		return fmt.Sprintf("%v", current) != fmt.Sprintf("%v", expected)
	case OpGreaterThan:
		return toFloat64(current) > toFloat64(expected)
	case OpLessThan:
		return toFloat64(current) < toFloat64(expected)
	case OpGreaterEqual:
		return toFloat64(current) >= toFloat64(expected)
	case OpLessEqual:
		return toFloat64(current) <= toFloat64(expected)
	default:
		return false
	}
}

func toFloat64(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case uint:
		return float64(val)
	case uint8:
		return float64(val)
	case uint16:
		return float64(val)
	case uint64:
		return float64(val)
	default:
		return 0
	}
}

// executeActions 执行动作列表
func (h *Hub) executeActions(actions []Action) error {
	for _, a := range actions {
		if err := h.executeAction(a); err != nil {
			return err
		}
	}
	return nil
}

// executeAction 执行单个动作
func (h *Hub) executeAction(action Action) error {
	switch action.Type {
	case ActionTypeDeviceControl:
		h.mu.Lock()
		device, ok := h.devices[action.DeviceID]
		if !ok {
			h.mu.Unlock()
			return ErrDeviceNotFound
		}
		for k, v := range action.Properties {
			device.Attributes[k] = v
		}
		device.UpdatedAt = time.Now()
		device.LastSeen = time.Now()
		h.addEventLocked(DeviceEvent{
			DeviceID:   device.ID,
			DeviceName: device.Name,
			Type:       "automation_action",
			State:      action.Properties,
			Timestamp:  time.Now(),
		})
		h.mu.Unlock()
		return nil

	case ActionTypeScene:
		return h.ActivateScene(action.SceneID)

	case ActionTypeNotification:
		h.mu.Lock()
		h.addEventLocked(DeviceEvent{
			DeviceID:  "system",
			Type:      "notification",
			State:     map[string]any{"message": action.Message},
			Timestamp: time.Now(),
		})
		h.mu.Unlock()
		return nil

	case ActionTypeDelay:
		if action.DelayMs > 0 {
			time.Sleep(time.Duration(action.DelayMs) * time.Millisecond)
		}
		return nil

	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
}

// ============================================================
// 验证函数
// ============================================================

func validateTrigger(trigger *Trigger) error {
	switch trigger.Type {
	case TriggerTypeDevice, TriggerTypeTemperature:
		if trigger.DeviceID == "" {
			return fmt.Errorf("device_id required for %s trigger", trigger.Type)
		}
		if trigger.Field == "" {
			return fmt.Errorf("field required for %s trigger", trigger.Type)
		}
	case TriggerTypeTime:
		if trigger.CronExpr == "" && trigger.TimeStr == "" {
			return fmt.Errorf("cron_expr or time_str required for time trigger")
		}
	case TriggerTypeSunrise, TriggerTypeSunset, TriggerTypeManual:
		// no extra params needed
	default:
		return fmt.Errorf("unknown trigger type: %s", trigger.Type)
	}
	return nil
}

func validateAction(action *Action) error {
	switch action.Type {
	case ActionTypeDeviceControl:
		if action.DeviceID == "" {
			return fmt.Errorf("device_id required for device_control")
		}
		if len(action.Properties) == 0 {
			return fmt.Errorf("properties required for device_control")
		}
	case ActionTypeScene:
		if action.SceneID == "" {
			return fmt.Errorf("scene_id required for scene action")
		}
	case ActionTypeNotification:
		if action.Message == "" {
			return fmt.Errorf("message required for notification")
		}
	case ActionTypeDelay:
		if action.DelayMs <= 0 {
			return fmt.Errorf("delay_ms must be positive")
		}
	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
	return nil
}
