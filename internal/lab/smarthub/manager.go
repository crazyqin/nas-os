package smarthub

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager 智能家居中枢管理器.
type Manager struct {
	mu          sync.RWMutex
	config      HubConfig
	devices     map[string]*Device
	scenes      map[string]*Scene
	automations map[string]*Automation
	energyLog   []EnergyRecord
	running     bool
	stopCh      chan struct{}
}

// NewManager 创建管理器.
func NewManager(cfg HubConfig) *Manager {
	if cfg.DeviceTimeout == 0 {
		cfg.DeviceTimeout = 5 * time.Minute
	}
	if cfg.DiscoveryInterval == 0 {
		cfg.DiscoveryInterval = 30 * time.Second
	}
	if cfg.TariffPerKWh == 0 {
		cfg.TariffPerKWh = 0.55
	}
	return &Manager{
		config:      cfg,
		devices:     make(map[string]*Device),
		scenes:      make(map[string]*Scene),
		automations: make(map[string]*Automation),
		stopCh:      make(chan struct{}),
	}
}

// Start 启动中枢.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return nil
	}
	m.running = true
	m.stopCh = make(chan struct{})
	go m.discoveryLoop()
	go m.automationLoop()
	go m.energyLoop()
	log.Println("[SmartHub] 智能家居中枢已启动")
	return nil
}

// Stop 停止中枢.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopCh)
	log.Println("[SmartHub] 智能家居中枢已停止")
}

// IsRunning 运行状态.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// ========== 设备管理 ==========

// AddDevice 添加设备.
func (m *Manager) AddDevice(dev *Device) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.devices[dev.ID]; exists {
		return ErrDuplicateDevice
	}
	dev.State = StateOnline
	dev.LastSeen = time.Now()
	dev.CreatedAt = time.Now()
	dev.UpdatedAt = time.Now()
	if dev.Properties == nil {
		dev.Properties = make(map[string]string)
	}
	m.devices[dev.ID] = dev
	return nil
}

// RemoveDevice 移除设备.
func (m *Manager) RemoveDevice(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.devices[id]; !ok {
		return ErrDeviceNotFound
	}
	delete(m.devices, id)
	return nil
}

// GetDevice 获取设备.
func (m *Manager) GetDevice(id string) (*Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	dev, ok := m.devices[id]
	if !ok {
		return nil, ErrDeviceNotFound
	}
	return dev, nil
}

// ListDevices 列出设备.
func (m *Manager) ListDevices(room string, devType DeviceType) []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Device
	for _, d := range m.devices {
		if room != "" && d.Room != room {
			continue
		}
		if devType != "" && d.Type != devType {
			continue
		}
		result = append(result, d)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// SendCommand 发送设备命令.
func (m *Manager) SendCommand(deviceID, command string, params map[string]string) error {
	m.mu.RLock()
	dev, ok := m.devices[deviceID]
	m.mu.RUnlock()
	if !ok {
		return ErrDeviceNotFound
	}
	if dev.State != StateOnline {
		return ErrDeviceOffline
	}
	m.mu.Lock()
	dev.UpdatedAt = time.Now()
	for k, v := range params {
		dev.Properties[k] = v
	}
	m.mu.Unlock()
	return nil
}

// ========== 场景管理 ==========

// CreateScene 创建场景.
func (m *Manager) CreateScene(scene *Scene) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	scene.CreatedAt = time.Now()
	scene.Enabled = true
	m.scenes[scene.ID] = scene
	return nil
}

// GetScene 获取场景.
func (m *Manager) GetScene(id string) (*Scene, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.scenes[id]
	if !ok {
		return nil, ErrSceneNotFound
	}
	return s, nil
}

// DeleteScene 删除场景.
func (m *Manager) DeleteScene(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.scenes[id]; !ok {
		return ErrSceneNotFound
	}
	delete(m.scenes, id)
	return nil
}

// ActivateScene 激活场景.
func (m *Manager) ActivateScene(id string) error {
	m.mu.Lock()
	scene, ok := m.scenes[id]
	if !ok {
		m.mu.Unlock()
		return ErrSceneNotFound
	}
	scene.LastRun = time.Now()
	scene.RunCount++
	m.mu.Unlock()

	for _, action := range scene.Actions {
		if action.Delay > 0 {
			time.Sleep(action.Delay)
		}
		_ = m.SendCommand(action.DeviceID, action.Command, action.Parameters)
	}
	return nil
}

// ListScenes 列出场景.
func (m *Manager) ListScenes() []*Scene {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Scene
	for _, s := range m.scenes {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// ========== 自动化规则 ==========

// CreateAutomation 创建自动化.
func (m *Manager) CreateAutomation(auto *Automation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	auto.CreatedAt = time.Now()
	auto.Enabled = true
	m.automations[auto.ID] = auto
	return nil
}

// DeleteAutomation 删除自动化.
func (m *Manager) DeleteAutomation(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.automations[id]; !ok {
		return ErrDeviceNotFound
	}
	delete(m.automations, id)
	return nil
}

// ListAutomations 列出自动化.
func (m *Manager) ListAutomations() []*Automation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Automation
	for _, a := range m.automations {
		result = append(result, a)
	}
	return result
}

// ========== 能耗统计 ==========

// GetEnergyStats 获取设备能耗统计.
func (m *Manager) GetEnergyStats(deviceID string, since time.Time) EnergyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var totalEnergy, totalPower, peakPower float64
	var count int
	for _, r := range m.energyLog {
		if r.DeviceID != deviceID || r.Timestamp.Before(since) {
			continue
		}
		totalEnergy += r.Energy
		totalPower += r.Power
		if r.Power > peakPower {
			peakPower = r.Power
		}
		count++
	}
	avgPower := 0.0
	if count > 0 {
		avgPower = totalPower / float64(count)
	}
	dailyCost := totalEnergy * m.config.TariffPerKWh
	return EnergyStats{
		DeviceID:        deviceID,
		TotalEnergy:     totalEnergy,
		AvgPower:        avgPower,
		PeakPower:       peakPower,
		DailyCost:       dailyCost,
		MonthlyCost:     dailyCost * 30,
		CarbonFootprint: totalEnergy * 0.5703, // 中国电网碳排放因子
	}
}

// GetTotalEnergyStats 全屋能耗统计.
func (m *Manager) GetTotalEnergyStats(since time.Time) map[string]EnergyStats {
	m.mu.RLock()
	deviceIDs := make([]string, 0, len(m.devices))
	for id := range m.devices {
		deviceIDs = append(deviceIDs, id)
	}
	m.mu.RUnlock()

	result := make(map[string]EnergyStats)
	for _, id := range deviceIDs {
		stats := m.GetEnergyStats(id, since)
		if stats.TotalEnergy > 0 {
			result[id] = stats
		}
	}
	return result
}

// ========== 房间管理 ==========

// ListRooms 列出所有房间.
func (m *Manager) ListRooms() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	roomSet := make(map[string]bool)
	for _, d := range m.devices {
		if d.Room != "" {
			roomSet[d.Room] = true
		}
	}
	var rooms []string
	for r := range roomSet {
		rooms = append(rooms, r)
	}
	sort.Strings(rooms)
	return rooms
}

// ========== 统计 ==========

// HubStats 中枢统计.
type HubStats struct {
	TotalDevices     int            `json:"total_devices"`
	OnlineDevices    int            `json:"online_devices"`
	OfflineDevices   int            `json:"offline_devices"`
	TotalScenes      int            `json:"total_scenes"`
	TotalAutomations int            `json:"total_automations"`
	ProtocolDist     map[string]int `json:"protocol_dist"`
	RoomDist         map[string]int `json:"room_dist"`
}

// GetStats 获取统计信息.
func (m *Manager) GetStats() HubStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats := HubStats{
		ProtocolDist:     make(map[string]int),
		RoomDist:         make(map[string]int),
		TotalScenes:      len(m.scenes),
		TotalAutomations: len(m.automations),
	}
	for _, d := range m.devices {
		stats.TotalDevices++
		if d.State == StateOnline {
			stats.OnlineDevices++
		} else {
			stats.OfflineDevices++
		}
		stats.ProtocolDist[string(d.Protocol)]++
		if d.Room != "" {
			stats.RoomDist[d.Room]++
		}
	}
	return stats
}

// ========== 内部循环 ==========

func (m *Manager) discoveryLoop() {
	ticker := time.NewTicker(m.config.DiscoveryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkDeviceHealth()
		}
	}
}

func (m *Manager) checkDeviceHealth() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.devices {
		if time.Since(d.LastSeen) > m.config.DeviceTimeout && d.State == StateOnline {
			d.State = StateOffline
			d.UpdatedAt = time.Now()
		}
	}
}

func (m *Manager) automationLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.evaluateAutomations()
		}
	}
}

func (m *Manager) evaluateAutomations() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, auto := range m.automations {
		if !auto.Enabled {
			continue
		}
		if m.evaluateConditions(auto.Conditions, auto.LogicOp) {
			auto.LastTrigger = time.Now()
			auto.TriggerCount++
			for _, action := range auto.Actions {
				_ = m.SendCommand(action.DeviceID, action.Command, action.Parameters)
			}
		}
	}
}

func (m *Manager) evaluateConditions(conditions []Condition, logicOp string) bool {
	if len(conditions) == 0 {
		return false
	}
	for _, c := range conditions {
		dev, ok := m.devices[c.DeviceID]
		if !ok {
			if logicOp == "and" {
				return false
			}
			continue
		}
		val := dev.Properties[c.Property]
		matched := compareValues(val, c.Operator, c.Value)
		if logicOp == "or" && matched {
			return true
		}
		if logicOp == "and" && !matched {
			return false
		}
	}
	return logicOp == "and"
}

func compareValues(actual, op, expected string) bool {
	switch op {
	case "eq":
		return actual == expected
	case "neq":
		return actual != expected
	case "contains":
		return strings.Contains(actual, expected)
	default:
		return false
	}
}

func (m *Manager) energyLoop() {
	if !m.config.EnableEnergy {
		return
	}
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.collectEnergyData()
		}
	}
}

func (m *Manager) collectEnergyData() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, d := range m.devices {
		if d.State != StateOnline {
			continue
		}
		if powerStr, ok := d.Properties["power"]; ok {
			var power float64
			fmt.Sscanf(powerStr, "%f", &power)
			if power > 0 {
				m.energyLog = append(m.energyLog, EnergyRecord{
					DeviceID:  d.ID,
					Timestamp: time.Now(),
					Power:     power,
					Energy:    power * 5.0 / 60.0 / 1000.0, // 5分钟间隔换算
				})
			}
		}
	}
	// 保留最近30天
	cutoff := time.Now().AddDate(0, 0, -30)
	start := 0
	for start < len(m.energyLog) && m.energyLog[start].Timestamp.Before(cutoff) {
		start++
	}
	if start > 0 {
		m.energyLog = m.energyLog[start:]
	}
}
