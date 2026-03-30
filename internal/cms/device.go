// Package cms 提供集中管理系统功能，类似群晖 CMS
package cms

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// 错误定义
var (
	ErrDeviceNotFound      = errors.New("device not found")
	ErrDeviceAlreadyExists = errors.New("device already exists")
	ErrDeviceOffline       = errors.New("device is offline")
	ErrRegistrationFailed  = errors.New("device registration failed")
	ErrInvalidDeviceType   = errors.New("invalid device type")
)

// DeviceType 设备类型
type DeviceType string

const (
	DeviceTypeNAS     DeviceType = "nas"     // NAS 设备
	DeviceTypeStorage DeviceType = "storage" // 存储服务器
	DeviceTypeEdge    DeviceType = "edge"    // 边缘设备
	DeviceTypeGateway DeviceType = "gateway" // 网关设备
	DeviceTypeVM      DeviceType = "vm"      // 虚拟机
)

// DeviceStatus 设备状态
type DeviceStatus string

const (
	DeviceStatusOnline      DeviceStatus = "online"      // 在线
	DeviceStatusOffline     DeviceStatus = "offline"     // 离线
	DeviceStatusWarning     DeviceStatus = "warning"     // 警告状态
	DeviceStatusError       DeviceStatus = "error"       // 错误状态
	DeviceStatusMaintenance DeviceStatus = "maintenance" // 维护中
	DeviceStatusRegistering DeviceStatus = "registering" // 注册中
)

// DeviceGroup 设备分组
type DeviceGroup struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	ParentID    string    `json:"parent_id,omitempty"` // 支持嵌套分组
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Device 设备信息
type Device struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Type            DeviceType        `json:"type"`
	Status          DeviceStatus      `json:"status"`
	IPAddress       string            `json:"ip_address"`
	Port            int               `json:"port"`
	MACAddress      string            `json:"mac_address,omitempty"`
	SerialNumber    string            `json:"serial_number,omitempty"`
	Model           string            `json:"model,omitempty"`
	FirmwareVersion string            `json:"firmware_version,omitempty"`
	GroupID         string            `json:"group_id,omitempty"`
	Location        string            `json:"location,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`

	// 系统信息
	CPUUsage    float64 `json:"cpu_usage,omitempty"`
	MemoryUsage float64 `json:"memory_usage,omitempty"`
	DiskUsage   float64 `json:"disk_usage,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	Uptime      int64   `json:"uptime,omitempty"` // 秒

	// 网络信息
	NetworkRxRate float64 `json:"network_rx_rate,omitempty"` // KB/s
	NetworkTxRate float64 `json:"network_tx_rate,omitempty"` // KB/s

	// 连接状态
	LastSeen      time.Time `json:"last_seen"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	ConnectedAt   time.Time `json:"connected_at"`

	// 注册信息
	RegisterToken string    `json:"register_token,omitempty"`
	RegisteredAt  time.Time `json:"registered_at"`
	RegisteredBy  string    `json:"registered_by"`

	// 健康检查
	HealthScore int `json:"health_score"` // 0-100
	AlertCount  int `json:"alert_count"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DeviceAlert 设备告警
type DeviceAlert struct {
	ID         string     `json:"id"`
	DeviceID   string     `json:"device_id"`
	Type       string     `json:"type"`     // cpu_high, disk_full, offline, etc.
	Severity   string     `json:"severity"` // low, medium, high, critical
	Message    string     `json:"message"`
	Value      float64    `json:"value,omitempty"`
	Threshold  float64    `json:"threshold,omitempty"`
	Timestamp  time.Time  `json:"timestamp"`
	Resolved   bool       `json:"resolved"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// DeviceRegistry 设备注册器
type DeviceRegistry struct {
	mu      sync.RWMutex
	devices map[string]*Device
	groups  map[string]*DeviceGroup
	alerts  map[string][]*DeviceAlert

	ctx    context.Context
	cancel context.CancelFunc
	logger *zap.Logger

	// 心跳超时配置
	heartbeatTimeout    time.Duration
	healthCheckInterval time.Duration
}

// RegistryConfig 注册器配置
type RegistryConfig struct {
	HeartbeatTimeout    time.Duration `json:"heartbeat_timeout"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	MaxDevices          int           `json:"max_devices"`
}

// NewDeviceRegistry 创建设备注册器
func NewDeviceRegistry(config *RegistryConfig, logger *zap.Logger) *DeviceRegistry {
	ctx, cancel := context.WithCancel(context.Background())

	if config == nil {
		config = &RegistryConfig{
			HeartbeatTimeout:    30 * time.Second,
			HealthCheckInterval: 60 * time.Second,
			MaxDevices:          1000,
		}
	}

	return &DeviceRegistry{
		devices:             make(map[string]*Device),
		groups:              make(map[string]*DeviceGroup),
		alerts:              make(map[string][]*DeviceAlert),
		ctx:                 ctx,
		cancel:              cancel,
		logger:              logger,
		heartbeatTimeout:    config.HeartbeatTimeout,
		healthCheckInterval: config.HealthCheckInterval,
	}
}

// Start 启动注册器
func (r *DeviceRegistry) Start() {
	go r.healthCheckLoop()
	r.logger.Info("Device registry started")
}

// Stop 停止注册器
func (r *DeviceRegistry) Stop() {
	r.cancel()
	r.logger.Info("Device registry stopped")
}

// RegisterDevice 注册设备
func (r *DeviceRegistry) RegisterDevice(deviceType DeviceType, name, ip string, port int, registeredBy string) (*Device, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查是否已存在
	for _, d := range r.devices {
		if d.IPAddress == ip && d.Port == port {
			return nil, ErrDeviceAlreadyExists
		}
	}

	now := time.Now()
	device := &Device{
		ID:            uuid.New().String(),
		Name:          name,
		Type:          deviceType,
		Status:        DeviceStatusRegistering,
		IPAddress:     ip,
		Port:          port,
		RegisterToken: uuid.New().String(),
		RegisteredAt:  now,
		RegisteredBy:  registeredBy,
		ConnectedAt:   now,
		LastHeartbeat: now,
		LastSeen:      now,
		HealthScore:   100,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	r.devices[device.ID] = device
	r.logger.Info("Device registered",
		zap.String("device_id", device.ID),
		zap.String("name", name),
		zap.String("ip", ip),
	)

	return device, nil
}

// ConfirmRegistration 确认设备注册
func (r *DeviceRegistry) ConfirmRegistration(deviceID, token string) (*Device, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	device, exists := r.devices[deviceID]
	if !exists {
		return nil, ErrDeviceNotFound
	}

	if device.RegisterToken != token {
		return nil, ErrRegistrationFailed
	}

	device.Status = DeviceStatusOnline
	device.RegisterToken = ""
	device.UpdatedAt = time.Now()

	r.logger.Info("Device registration confirmed", zap.String("device_id", deviceID))
	return device, nil
}

// UnregisterDevice 注销设备
func (r *DeviceRegistry) UnregisterDevice(deviceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.devices[deviceID]; !exists {
		return ErrDeviceNotFound
	}

	delete(r.devices, deviceID)
	delete(r.alerts, deviceID)

	r.logger.Info("Device unregistered", zap.String("device_id", deviceID))
	return nil
}

// GetDevice 获取设备
func (r *DeviceRegistry) GetDevice(deviceID string) (*Device, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	device, exists := r.devices[deviceID]
	if !exists {
		return nil, ErrDeviceNotFound
	}
	return device, nil
}

// GetDeviceByIP 通过 IP 获取设备
func (r *DeviceRegistry) GetDeviceByIP(ip string, port int) (*Device, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, device := range r.devices {
		if device.IPAddress == ip && device.Port == port {
			return device, nil
		}
	}
	return nil, ErrDeviceNotFound
}

// UpdateDevice 更新设备状态
func (r *DeviceRegistry) UpdateDevice(deviceID string, updates map[string]interface{}) (*Device, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	device, exists := r.devices[deviceID]
	if !exists {
		return nil, ErrDeviceNotFound
	}

	now := time.Now()
	device.UpdatedAt = now
	device.LastSeen = now

	// 更新字段
	if status, ok := updates["status"].(DeviceStatus); ok {
		device.Status = status
	}
	if cpuUsage, ok := updates["cpu_usage"].(float64); ok {
		device.CPUUsage = cpuUsage
	}
	if memoryUsage, ok := updates["memory_usage"].(float64); ok {
		device.MemoryUsage = memoryUsage
	}
	if diskUsage, ok := updates["disk_usage"].(float64); ok {
		device.DiskUsage = diskUsage
	}
	if temperature, ok := updates["temperature"].(float64); ok {
		device.Temperature = temperature
	}
	if uptime, ok := updates["uptime"].(int64); ok {
		device.Uptime = uptime
	}
	if networkRx, ok := updates["network_rx_rate"].(float64); ok {
		device.NetworkRxRate = networkRx
	}
	if networkTx, ok := updates["network_tx_rate"].(float64); ok {
		device.NetworkTxRate = networkTx
	}
	if firmware, ok := updates["firmware_version"].(string); ok {
		device.FirmwareVersion = firmware
	}
	if groupID, ok := updates["group_id"].(string); ok {
		device.GroupID = groupID
	}
	if location, ok := updates["location"].(string); ok {
		device.Location = location
	}

	// 更新健康评分
	r.updateHealthScore(device)

	return device, nil
}

// UpdateHeartbeat 更新心跳
func (r *DeviceRegistry) UpdateHeartbeat(deviceID string, metrics map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	device, exists := r.devices[deviceID]
	if !exists {
		return ErrDeviceNotFound
	}

	now := time.Now()
	device.LastHeartbeat = now
	device.LastSeen = now
	device.Status = DeviceStatusOnline
	device.UpdatedAt = now

	// 更新指标
	for k, v := range metrics {
		switch k {
		case "cpu_usage":
			if val, ok := v.(float64); ok {
				device.CPUUsage = val
			}
		case "memory_usage":
			if val, ok := v.(float64); ok {
				device.MemoryUsage = val
			}
		case "disk_usage":
			if val, ok := v.(float64); ok {
				device.DiskUsage = val
			}
		case "temperature":
			if val, ok := v.(float64); ok {
				device.Temperature = val
			}
		case "uptime":
			if val, ok := v.(int64); ok {
				device.Uptime = val
			}
		case "network_rx_rate":
			if val, ok := v.(float64); ok {
				device.NetworkRxRate = val
			}
		case "network_tx_rate":
			if val, ok := v.(float64); ok {
				device.NetworkTxRate = val
			}
		}
	}

	r.updateHealthScore(device)

	// 检查告警
	r.checkAlerts(device)

	return nil
}

// ListDevices 列出设备
func (r *DeviceRegistry) ListDevices(filter DeviceFilter) []*Device {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Device, 0)
	for _, device := range r.devices {
		if r.matchesFilter(device, filter) {
			result = append(result, device)
		}
	}

	return result
}

// DeviceFilter 设备筛选条件
type DeviceFilter struct {
	Type     []DeviceType   `json:"type,omitempty"`
	Status   []DeviceStatus `json:"status,omitempty"`
	GroupID  string         `json:"group_id,omitempty"`
	Location string         `json:"location,omitempty"`
	Search   string         `json:"search,omitempty"`
}

// matchesFilter 检查设备是否匹配筛选条件
func (r *DeviceRegistry) matchesFilter(device *Device, filter DeviceFilter) bool {
	if len(filter.Type) > 0 {
		matched := false
		for _, t := range filter.Type {
			if device.Type == t {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if len(filter.Status) > 0 {
		matched := false
		for _, s := range filter.Status {
			if device.Status == s {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if filter.GroupID != "" && device.GroupID != filter.GroupID {
		return false
	}

	if filter.Location != "" && device.Location != filter.Location {
		return false
	}

	return true
}

// CreateGroup 创建设备分组
func (r *DeviceRegistry) CreateGroup(name, description, parentID string) (*DeviceGroup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	group := &DeviceGroup{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		ParentID:    parentID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	r.groups[group.ID] = group
	return group, nil
}

// GetGroup 获取分组
func (r *DeviceRegistry) GetGroup(groupID string) (*DeviceGroup, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	group, exists := r.groups[groupID]
	if !exists {
		return nil, errors.New("group not found")
	}
	return group, nil
}

// ListGroups 列出分组
func (r *DeviceRegistry) ListGroups() []*DeviceGroup {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*DeviceGroup, 0, len(r.groups))
	for _, group := range r.groups {
		result = append(result, group)
	}
	return result
}

// AssignDeviceToGroup 将设备分配到分组
func (r *DeviceRegistry) AssignDeviceToGroup(deviceID, groupID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	device, exists := r.devices[deviceID]
	if !exists {
		return ErrDeviceNotFound
	}

	if groupID != "" {
		if _, exists := r.groups[groupID]; !exists {
			return errors.New("group not found")
		}
	}

	device.GroupID = groupID
	device.UpdatedAt = time.Now()
	return nil
}

// healthCheckLoop 健康检查循环
func (r *DeviceRegistry) healthCheckLoop() {
	ticker := time.NewTicker(r.healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.checkAllDevices()
		}
	}
}

// checkAllDevices 检查所有设备状态
func (r *DeviceRegistry) checkAllDevices() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for _, device := range r.devices {
		if device.Status == DeviceStatusMaintenance {
			continue
		}

		// 心跳超时检测
		if now.Sub(device.LastHeartbeat) > r.heartbeatTimeout {
			if device.Status != DeviceStatusOffline {
				device.Status = DeviceStatusOffline
				r.logger.Warn("Device offline detected",
					zap.String("device_id", device.ID),
					zap.String("name", device.Name),
				)
				r.addAlert(device.ID, "offline", "high", "设备离线", 0, 0)
			}
		}

		// 更新健康评分
		r.updateHealthScore(device)
	}
}

// updateHealthScore 更新健康评分
func (r *DeviceRegistry) updateHealthScore(device *Device) {
	score := 100

	// CPU 使用率影响
	if device.CPUUsage > 90 {
		score -= 20
	} else if device.CPUUsage > 80 {
		score -= 10
	}

	// 内存使用率影响
	if device.MemoryUsage > 90 {
		score -= 20
	} else if device.MemoryUsage > 80 {
		score -= 10
	}

	// 磁盘使用率影响
	if device.DiskUsage > 95 {
		score -= 25
	} else if device.DiskUsage > 85 {
		score -= 15
	}

	// 温度影响
	if device.Temperature > 80 {
		score -= 15
	} else if device.Temperature > 70 {
		score -= 5
	}

	// 状态影响
	if device.Status == DeviceStatusOffline {
		score = 0
	} else if device.Status == DeviceStatusWarning {
		score -= 20
	} else if device.Status == DeviceStatusError {
		score -= 40
	}

	if score < 0 {
		score = 0
	}

	device.HealthScore = score
}

// checkAlerts 检查告警条件
func (r *DeviceRegistry) checkAlerts(device *Device) {
	// CPU 高告警
	if device.CPUUsage > 85 {
		r.addAlert(device.ID, "cpu_high", "medium", "CPU 使用率过高", device.CPUUsage, 85)
	}

	// 内存高告警
	if device.MemoryUsage > 85 {
		r.addAlert(device.ID, "memory_high", "medium", "内存使用率过高", device.MemoryUsage, 85)
	}

	// 磁盘高告警
	if device.DiskUsage > 90 {
		r.addAlert(device.ID, "disk_high", "high", "磁盘使用率过高", device.DiskUsage, 90)
	}

	// 温度高告警
	if device.Temperature > 75 {
		r.addAlert(device.ID, "temperature_high", "medium", "设备温度过高", device.Temperature, 75)
	}
}

// addAlert 添加告警
func (r *DeviceRegistry) addAlert(deviceID, alertType, severity, message string, value, threshold float64) {
	alert := &DeviceAlert{
		ID:        uuid.New().String(),
		DeviceID:  deviceID,
		Type:      alertType,
		Severity:  severity,
		Message:   message,
		Value:     value,
		Threshold: threshold,
		Timestamp: time.Now(),
		Resolved:  false,
	}

	r.alerts[deviceID] = append(r.alerts[deviceID], alert)

	// 更新设备告警计数
	if device, exists := r.devices[deviceID]; exists {
		device.AlertCount = len(r.alerts[deviceID])
	}
}

// GetDeviceAlerts 获取设备告警
func (r *DeviceRegistry) GetDeviceAlerts(deviceID string, resolved bool) []*DeviceAlert {
	r.mu.RLock()
	defer r.mu.RUnlock()

	alerts := r.alerts[deviceID]
	result := make([]*DeviceAlert, 0)

	for _, alert := range alerts {
		if resolved == alert.Resolved {
			result = append(result, alert)
		}
	}

	return result
}

// ResolveAlert 解决告警
func (r *DeviceRegistry) ResolveAlert(alertID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for deviceID, alerts := range r.alerts {
		for _, alert := range alerts {
			if alert.ID == alertID {
				now := time.Now()
				alert.Resolved = true
				alert.ResolvedAt = &now

				// 更新设备告警计数
				if device, exists := r.devices[deviceID]; exists {
					unresolvedCount := 0
					for _, a := range r.alerts[deviceID] {
						if !a.Resolved {
							unresolvedCount++
						}
					}
					device.AlertCount = unresolvedCount
				}

				return nil
			}
		}
	}

	return errors.New("alert not found")
}

// RegistryStats 注册器统计
type RegistryStats struct {
	TotalDevices     int            `json:"total_devices"`
	OnlineDevices    int            `json:"online_devices"`
	OfflineDevices   int            `json:"offline_devices"`
	ByType           map[string]int `json:"by_type"`
	ByGroup          map[string]int `json:"by_group"`
	TotalAlerts      int            `json:"total_alerts"`
	UnresolvedAlerts int            `json:"unresolved_alerts"`
}

// GetStats 获取统计
func (r *DeviceRegistry) GetStats() RegistryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := RegistryStats{
		ByType:  make(map[string]int),
		ByGroup: make(map[string]int),
	}

	for _, device := range r.devices {
		stats.TotalDevices++
		stats.ByType[string(device.Type)]++

		if device.GroupID != "" {
			stats.ByGroup[device.GroupID]++
		}

		switch device.Status {
		case DeviceStatusOnline:
			stats.OnlineDevices++
		case DeviceStatusOffline:
			stats.OfflineDevices++
		}
	}

	for _, alerts := range r.alerts {
		stats.TotalAlerts += len(alerts)
		for _, alert := range alerts {
			if !alert.Resolved {
				stats.UnresolvedAlerts++
			}
		}
	}

	return stats
}
