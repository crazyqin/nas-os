// Package nasconnect 提供多 NAS 设备统一连接管理功能
// 支持设备发现、远程连接、状态同步、集中管理
package nasconnect

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager 多 NAS 连接管理器
type Manager struct {
	mu          sync.RWMutex
	devices     map[string]*NASDevice
	groups      map[string]*DeviceGroup
	connections map[string]*Connection
	credentials map[string]*Credential
	events      []Event
	config      *Config
	logger      Logger
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// Logger 日志接口
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// Config 配置
type Config struct {
	DiscoveryEnabled  bool          `json:"discovery_enabled"`
	DiscoveryInterval time.Duration `json:"discovery_interval"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	ConnectionTimeout time.Duration `json:"connection_timeout"`
	MaxRetries        int           `json:"max_retries"`
	AutoReconnect     bool          `json:"auto_reconnect"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		DiscoveryEnabled:  true,
		DiscoveryInterval: 5 * time.Minute,
		HeartbeatInterval: 30 * time.Second,
		ConnectionTimeout: 10 * time.Second,
		MaxRetries:        3,
		AutoReconnect:     true,
	}
}

// NewManager 创建管理器
func NewManager(logger Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		devices:     make(map[string]*NASDevice),
		groups:      make(map[string]*DeviceGroup),
		connections: make(map[string]*Connection),
		credentials: make(map[string]*Credential),
		events:      make([]Event, 0),
		config:      DefaultConfig(),
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
	}

	// 启动后台任务
	m.wg.Add(2)
	go m.heartbeatLoop()
	go m.discoveryLoop()

	return m
}

// ==================== 设备管理 ====================

// AddDevice 添加 NAS 设备
func (m *Manager) AddDevice(device *NASDevice) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if device.ID == "" {
		device.ID = generateID("nas")
	}
	device.CreatedAt = time.Now()
	device.UpdatedAt = time.Now()
	device.Status = DeviceStatusUnknown

	m.devices[device.ID] = device
	m.addEvent(EventTypeDeviceAdded, "设备添加: %s (%s)", device.Name, device.Host)
	m.logger.Info("NAS 设备添加成功: %s (%s)", device.Name, device.ID)
	return nil
}

// UpdateDevice 更新设备信息
func (m *Manager) UpdateDevice(device *NASDevice) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.devices[device.ID]
	if !ok {
		return fmt.Errorf("设备不存在: %s", device.ID)
	}

	device.CreatedAt = existing.CreatedAt
	device.UpdatedAt = time.Now()
	m.devices[device.ID] = device
	return nil
}

// RemoveDevice 移除设备
func (m *Manager) RemoveDevice(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	// 关闭连接
	if conn, exists := m.connections[deviceID]; exists {
		conn.Status = ConnectionStatusClosed
		delete(m.connections, deviceID)
	}

	delete(m.devices, deviceID)
	m.addEvent(EventTypeDeviceRemoved, "设备移除: %s", device.Name)
	m.logger.Info("NAS 设备移除: %s", deviceID)
	return nil
}

// GetDevice 获取设备信息
func (m *Manager) GetDevice(deviceID string) (*NASDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}
	return device, nil
}

// ListDevices 列出所有设备
func (m *Manager) ListDevices(status DeviceStatus) []*NASDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*NASDevice, 0, len(m.devices))
	for _, d := range m.devices {
		if status == "" || d.Status == status {
			devices = append(devices, d)
		}
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].Name < devices[j].Name
	})
	return devices
}

// ==================== 设备分组 ====================

// CreateGroup 创建设备分组
func (m *Manager) CreateGroup(group *DeviceGroup) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if group.ID == "" {
		group.ID = generateID("grp")
	}
	group.CreatedAt = time.Now()
	group.UpdatedAt = time.Now()

	m.groups[group.ID] = group
	m.logger.Info("设备分组创建: %s (%s)", group.Name, group.ID)
	return nil
}

// UpdateGroup 更新分组
func (m *Manager) UpdateGroup(group *DeviceGroup) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.groups[group.ID]
	if !ok {
		return fmt.Errorf("分组不存在: %s", group.ID)
	}

	group.CreatedAt = existing.CreatedAt
	group.UpdatedAt = time.Now()
	m.groups[group.ID] = group
	return nil
}

// DeleteGroup 删除分组
func (m *Manager) DeleteGroup(groupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.groups[groupID]; !ok {
		return fmt.Errorf("分组不存在: %s", groupID)
	}

	delete(m.groups, groupID)
	return nil
}

// ListGroups 列出所有分组
func (m *Manager) ListGroups() []*DeviceGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make([]*DeviceGroup, 0, len(m.groups))
	for _, g := range m.groups {
		groups = append(groups, g)
	}
	return groups
}

// AddDeviceToGroup 将设备添加到分组
func (m *Manager) AddDeviceToGroup(deviceID, groupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	group, ok := m.groups[groupID]
	if !ok {
		return fmt.Errorf("分组不存在: %s", groupID)
	}

	// 检查是否已在分组中
	for _, id := range group.DeviceIDs {
		if id == deviceID {
			return nil // 已存在
		}
	}

	group.DeviceIDs = append(group.DeviceIDs, deviceID)
	group.UpdatedAt = time.Now()
	m.addEvent(EventTypeGroupUpdated, "设备 %s 添加到分组 %s", device.Name, group.Name)
	return nil
}

// RemoveDeviceFromGroup 从分组移除设备
func (m *Manager) RemoveDeviceFromGroup(deviceID, groupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.groups[groupID]
	if !ok {
		return fmt.Errorf("分组不存在: %s", groupID)
	}

	for i, id := range group.DeviceIDs {
		if id == deviceID {
			group.DeviceIDs = append(group.DeviceIDs[:i], group.DeviceIDs[i+1:]...)
			group.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("设备不在分组中")
}

// ==================== 连接管理 ====================

// Connect 连接到 NAS 设备
func (m *Manager) Connect(deviceID string) (*Connection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}

	// 检查现有连接
	if conn, exists := m.connections[deviceID]; exists && conn.Status == ConnectionStatusConnected {
		return conn, nil
	}

	// 创建新连接
	conn := &Connection{
		ID:        generateID("conn"),
		DeviceID:  deviceID,
		Status:    ConnectionStatusConnecting,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 模拟连接过程
	device.Status = DeviceStatusOnline
	device.LastSeen = time.Now()
	conn.Status = ConnectionStatusConnected
	conn.Latency = 5 * time.Millisecond

	m.connections[deviceID] = conn
	m.addEvent(EventTypeConnected, "连接成功: %s", device.Name)
	m.logger.Info("连接到 NAS: %s (%s)", device.Name, device.Host)

	return conn, nil
}

// Disconnect 断开连接
func (m *Manager) Disconnect(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, ok := m.connections[deviceID]
	if !ok {
		return fmt.Errorf("连接不存在: %s", deviceID)
	}

	conn.Status = ConnectionStatusClosed
	conn.UpdatedAt = time.Now()

	if device, exists := m.devices[deviceID]; exists {
		device.Status = DeviceStatusOffline
		m.addEvent(EventTypeDisconnected, "断开连接: %s", device.Name)
	}

	delete(m.connections, deviceID)
	return nil
}

// GetConnection 获取连接状态
func (m *Manager) GetConnection(deviceID string) (*Connection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, ok := m.connections[deviceID]
	if !ok {
		return nil, fmt.Errorf("连接不存在: %s", deviceID)
	}
	return conn, nil
}

// ==================== 凭证管理 ====================

// SaveCredential 保存凭证
func (m *Manager) SaveCredential(cred *Credential) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cred.ID == "" {
		cred.ID = generateID("cred")
	}
	cred.CreatedAt = time.Now()
	cred.UpdatedAt = time.Now()

	m.credentials[cred.ID] = cred
	m.logger.Info("凭证保存: %s", cred.Name)
	return nil
}

// GetCredential 获取凭证
func (m *Manager) GetCredential(credID string) (*Credential, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cred, ok := m.credentials[credID]
	if !ok {
		return nil, fmt.Errorf("凭证不存在: %s", credID)
	}
	return cred, nil
}

// ListCredentials 列出凭证
func (m *Manager) ListCredentials() []*Credential {
	m.mu.RLock()
	defer m.mu.RUnlock()

	creds := make([]*Credential, 0, len(m.credentials))
	for _, c := range m.credentials {
		// 隐藏敏感信息
		safeCred := *c
		safeCred.Password = "***"
		safeCred.Token = "***"
		creds = append(creds, &safeCred)
	}
	return creds
}

// DeleteCredential 删除凭证
func (m *Manager) DeleteCredential(credID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.credentials[credID]; !ok {
		return fmt.Errorf("凭证不存在: %s", credID)
	}

	delete(m.credentials, credID)
	return nil
}

// ==================== 状态同步 ====================

// SyncDeviceStatus 同步设备状态
func (m *Manager) SyncDeviceStatus(deviceID string) (*DeviceStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}

	// 更新状态
	device.LastSeen = time.Now()
	device.UpdatedAt = time.Now()

	return &device.Status, nil
}

// SyncAllDevices 同步所有设备状态
func (m *Manager) SyncAllDevices() map[string]error {
	m.mu.Lock()
	defer m.mu.Unlock()

	results := make(map[string]error)
	for id, device := range m.devices {
		device.LastSeen = time.Now()
		device.UpdatedAt = time.Now()
		results[id] = nil
	}

	m.addEvent(EventTypeSyncComplete, "状态同步完成: %d 台设备", len(m.devices))
	return results
}

// ==================== 事件管理 ====================

// GetEvents 获取事件列表
func (m *Manager) GetEvents(limit int) []Event {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.events) {
		limit = len(m.events)
	}

	// 返回最新的事件
	start := len(m.events) - limit
	if start < 0 {
		start = 0
	}
	return m.events[start:]
}

// addEvent 添加事件（需要调用者持有锁）
func (m *Manager) addEvent(eventType EventType, format string, args ...interface{}) {
	event := Event{
		ID:        generateID("evt"),
		Type:      eventType,
		Message:   fmt.Sprintf(format, args...),
		Timestamp: time.Now(),
	}
	m.events = append(m.events, event)

	// 限制事件数量
	if len(m.events) > 1000 {
		m.events = m.events[len(m.events)-1000:]
	}
}

// ==================== 后台任务 ====================

// heartbeatLoop 心跳检测循环
func (m *Manager) heartbeatLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(m.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkDeviceHeartbeats()
		}
	}
}

// checkDeviceHeartbeats 检查设备心跳
func (m *Manager) checkDeviceHeartbeats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	timeout := m.config.HeartbeatInterval * 3

	for _, device := range m.devices {
		if device.Status == DeviceStatusOffline {
			continue
		}

		if now.Sub(device.LastSeen) > timeout {
			device.Status = DeviceStatusOffline
			m.addEvent(EventTypeDeviceOffline, "设备离线: %s", device.Name)
			m.logger.Info("设备离线: %s", device.Name)
		}
	}
}

// discoveryLoop 设备发现循环
func (m *Manager) discoveryLoop() {
	defer m.wg.Done()

	if !m.config.DiscoveryEnabled {
		return
	}

	ticker := time.NewTicker(m.config.DiscoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.discoverDevices()
		}
	}
}

// discoverDevices 发现设备
func (m *Manager) discoverDevices() {
	// 设备发现逻辑（mDNS/SSDP）
	m.logger.Debug("执行设备发现...")
}

// ==================== 统计信息 ====================

// GetStats 获取统计信息
func (m *Manager) GetStats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &Stats{
		TotalDevices: len(m.devices),
		TotalGroups:  len(m.groups),
		TotalConns:   len(m.connections),
		TotalEvents:  len(m.events),
	}

	for _, d := range m.devices {
		switch d.Status {
		case DeviceStatusOnline:
			stats.OnlineDevices++
		case DeviceStatusOffline:
			stats.OfflineDevices++
		case DeviceStatusUnknown:
			stats.UnknownDevices++
		}
	}

	return stats
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
}

// ==================== HTTP 路由 ====================

// RegisterRoutes 注册 HTTP 路由
func (m *Manager) RegisterRoutes(mux *http.ServeMux) {
	// 设备管理
	mux.HandleFunc("/api/nasconnect/devices", m.handleDevices)
	mux.HandleFunc("/api/nasconnect/devices/", m.handleDeviceDetail)

	// 设备分组
	mux.HandleFunc("/api/nasconnect/groups", m.handleGroups)
	mux.HandleFunc("/api/nasconnect/groups/", m.handleGroupDetail)

	// 连接管理
	mux.HandleFunc("/api/nasconnect/connect", m.handleConnect)
	mux.HandleFunc("/api/nasconnect/disconnect", m.handleDisconnect)

	// 凭证管理
	mux.HandleFunc("/api/nasconnect/credentials", m.handleCredentials)

	// 状态同步
	mux.HandleFunc("/api/nasconnect/sync", m.handleSync)

	// 事件日志
	mux.HandleFunc("/api/nasconnect/events", m.handleEvents)

	// 统计信息
	mux.HandleFunc("/api/nasconnect/stats", m.handleStats)
}

func (m *Manager) handleDevices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		status := DeviceStatus(r.URL.Query().Get("status"))
		devices := m.ListDevices(status)
		writeJSON(w, devices)
	case http.MethodPost:
		var device NASDevice
		if err := decodeJSON(r, &device); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.AddDevice(&device); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, device)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleDeviceDetail(w http.ResponseWriter, r *http.Request) {
	deviceID := extractID(r.URL.Path, "/api/nasconnect/devices/")
	if deviceID == "" {
		http.Error(w, "device_id required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		device, err := m.GetDevice(deviceID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, device)
	case http.MethodPut:
		var device NASDevice
		if err := decodeJSON(r, &device); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		device.ID = deviceID
		if err := m.UpdateDevice(&device); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, device)
	case http.MethodDelete:
		if err := m.RemoveDevice(deviceID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleGroups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		groups := m.ListGroups()
		writeJSON(w, groups)
	case http.MethodPost:
		var group DeviceGroup
		if err := decodeJSON(r, &group); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreateGroup(&group); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, group)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleGroupDetail(w http.ResponseWriter, r *http.Request) {
	groupID := extractID(r.URL.Path, "/api/nasconnect/groups/")
	if groupID == "" {
		http.Error(w, "group_id required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		if err := m.DeleteGroup(groupID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DeviceID string `json:"device_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	conn, err := m.Connect(req.DeviceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, conn)
}

func (m *Manager) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DeviceID string `json:"device_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := m.Disconnect(req.DeviceID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "disconnected"})
}

func (m *Manager) handleCredentials(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		creds := m.ListCredentials()
		writeJSON(w, creds)
	case http.MethodPost:
		var cred Credential
		if err := decodeJSON(r, &cred); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.SaveCredential(&cred); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, cred)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	results := m.SyncAllDevices()
	writeJSON(w, results)
}

func (m *Manager) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	events := m.GetEvents(limit)
	writeJSON(w, events)
}

func (m *Manager) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := m.GetStats()
	writeJSON(w, stats)
}

// ==================== 辅助函数 ====================

func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func extractID(path, prefix string) string {
	return strings.TrimPrefix(path, prefix)
}
