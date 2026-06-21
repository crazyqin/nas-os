// Package cloudportal 实现云端管理门户。
// 支持多设备统一管理、远程访问、状态监控、配置同步，
// 对标 TrueNAS Connect 云端管理平台。
package cloudportal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"
)

// PortalManager 云端门户管理器
type PortalManager struct {
	mu        sync.RWMutex
	config    *PortalConfig
	devices   map[string]*Device
	sessions  map[string]*Session
	ctx       context.Context
	cancel    context.CancelFunc
	running   bool
}

// PortalConfig 门户配置
type PortalConfig struct {
	APIEndpoint   string        `json:"api_endpoint"`
	DeviceID      string        `json:"device_id"`
	DeviceName    string        `json:"device_name"`
	EnableRemote  bool          `json:"enable_remote"`
	EnableSync    bool          `json:"enable_sync"`
	SyncInterval  time.Duration `json:"sync_interval"`
	MaxDevices    int           `json:"max_devices"`
	EnableMFA     bool          `json:"enable_mfa"`
}

// DefaultPortalConfig 默认配置
func DefaultPortalConfig() *PortalConfig {
	return &PortalConfig{
		APIEndpoint:  "https://portal.nas-os.com",
		EnableRemote: true,
		EnableSync:   true,
		SyncInterval: 5 * time.Minute,
		MaxDevices:   10,
		EnableMFA:    false,
	}
}

// Device 管理设备
type Device struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"` // nas, server, workstation
	IP          string    `json:"ip"`
	Status      string    `json:"status"` // online, offline, maintenance
	LastSeen    time.Time `json:"last_seen"`
	Version     string    `json:"version"`
	CPU         float64   `json:"cpu_usage"`
	Memory      float64   `json:"memory_usage"`
	Disk        float64   `json:"disk_usage"`
	Uptime      int64     `json:"uptime_seconds"`
	Tags        []string  `json:"tags,omitempty"`
}

// Session 管理会话
type Session struct {
	ID        string    `json:"id"`
	DeviceID  string    `json:"device_id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	IP        string    `json:"ip"`
	IsActive  bool      `json:"is_active"`
}

// DeviceStats 设备统计
type DeviceStats struct {
	TotalDevices   int     `json:"total_devices"`
	OnlineDevices  int     `json:"online_devices"`
	OfflineDevices int     `json:"offline_devices"`
	AvgCPU         float64 `json:"avg_cpu"`
	AvgMemory      float64 `json:"avg_memory"`
	TotalStorage   int64   `json:"total_storage"`
	UsedStorage    int64   `json:"used_storage"`
}

// SyncConfig 同步配置
type SyncConfig struct {
	SyncSettings  bool `json:"sync_settings"`
	SyncUsers     bool `json:"sync_users"`
	SyncShares    bool `json:"sync_shares"`
	SyncBackups   bool `json:"sync_backups"`
	SyncApps      bool `json:"sync_apps"`
}

// NewPortalManager 创建门户管理器
func NewPortalManager(config *PortalConfig) *PortalManager {
	if config == nil {
		config = DefaultPortalConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &PortalManager{
		config:   config,
		devices:  make(map[string]*Device),
		sessions: make(map[string]*Session),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start 启动门户管理器
func (pm *PortalManager) Start() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.running {
		return fmt.Errorf("门户管理器已在运行")
	}

	pm.running = true

	// 注册本机设备
	pm.devices[pm.config.DeviceID] = &Device{
		ID:       pm.config.DeviceID,
		Name:     pm.config.DeviceName,
		Type:     "nas",
		Status:   "online",
		LastSeen: time.Now(),
	}

	// 启动同步
	if pm.config.EnableSync {
		go pm.syncLoop()
	}

	// 启动健康检查
	go pm.healthCheckLoop()

	log.Printf("[CloudPortal] 云端门户启动成功，设备: %s", pm.config.DeviceName)
	return nil
}

// Stop 停止门户管理器
func (pm *PortalManager) Stop() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.running {
		return nil
	}

	pm.cancel()
	pm.running = false

	log.Println("[CloudPortal] 云端门户已停止")
	return nil
}

// RegisterDevice 注册设备
func (pm *PortalManager) RegisterDevice(device *Device) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.running {
		return fmt.Errorf("门户管理器未运行")
	}

	if len(pm.devices) >= pm.config.MaxDevices {
		return fmt.Errorf("已达到最大设备数 (%d)", pm.config.MaxDevices)
	}

	if device.ID == "" {
		device.ID = generateDeviceID(device.Name)
	}

	device.LastSeen = time.Now()
	if device.Status == "" {
		device.Status = "online"
	}

	pm.devices[device.ID] = device

	log.Printf("[CloudPortal] 注册设备: %s (%s)", device.Name, device.ID)
	return nil
}

// RemoveDevice 移除设备
func (pm *PortalManager) RemoveDevice(deviceID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.devices[deviceID]; !exists {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	delete(pm.devices, deviceID)

	log.Printf("[CloudPortal] 移除设备: %s", deviceID)
	return nil
}

// GetDevices 获取所有设备
func (pm *PortalManager) GetDevices() []*Device {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*Device, 0, len(pm.devices))
	for _, d := range pm.devices {
		result = append(result, d)
	}
	return result
}

// GetDevice 获取指定设备
func (pm *PortalManager) GetDevice(deviceID string) (*Device, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	device, exists := pm.devices[deviceID]
	if !exists {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}

	return device, nil
}

// UpdateDeviceStatus 更新设备状态
func (pm *PortalManager) UpdateDeviceStatus(deviceID string, status *Device) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	device, exists := pm.devices[deviceID]
	if !exists {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	if status.CPU > 0 {
		device.CPU = status.CPU
	}
	if status.Memory > 0 {
		device.Memory = status.Memory
	}
	if status.Disk > 0 {
		device.Disk = status.Disk
	}
	if status.Version != "" {
		device.Version = status.Version
	}
	device.LastSeen = time.Now()
	device.Status = "online"

	return nil
}

// GetStats 获取设备统计
func (pm *PortalManager) GetStats() *DeviceStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	stats := &DeviceStats{}

	var totalCPU, totalMem float64
	var count int

	for _, d := range pm.devices {
		stats.TotalDevices++
		if d.Status == "online" {
			stats.OnlineDevices++
			totalCPU += d.CPU
			totalMem += d.Memory
			count++
		} else {
			stats.OfflineDevices++
		}
	}

	if count > 0 {
		stats.AvgCPU = totalCPU / float64(count)
		stats.AvgMemory = totalMem / float64(count)
	}

	return stats
}

// CreateSession 创建管理会话
func (pm *PortalManager) CreateSession(deviceID, userID, ip string, duration time.Duration) (*Session, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.devices[deviceID]; !exists {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}

	sessionID := generateSessionID()

	session := &Session{
		ID:        sessionID,
		DeviceID:  deviceID,
		UserID:    userID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(duration),
		IP:        ip,
		IsActive:  true,
	}

	pm.sessions[sessionID] = session

	log.Printf("[CloudPortal] 创建会话: %s (设备: %s, 用户: %s)", sessionID, deviceID, userID)
	return session, nil
}

// ValidateSession 验证会话
func (pm *PortalManager) ValidateSession(sessionID string) (*Session, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	session, exists := pm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("会话不存在")
	}

	if !session.IsActive || session.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("会话已过期")
	}

	return session, nil
}

// SyncConfig 同步配置到其他设备
func (pm *PortalManager) SyncConfig(targetDeviceID string, config *SyncConfig) error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if !pm.config.EnableSync {
		return fmt.Errorf("同步功能未启用")
	}

	device, exists := pm.devices[targetDeviceID]
	if !exists {
		return fmt.Errorf("目标设备不存在: %s", targetDeviceID)
	}

	if device.Status != "online" {
		return fmt.Errorf("目标设备离线: %s", targetDeviceID)
	}

	log.Printf("[CloudPortal] 同步配置到设备: %s", targetDeviceID)
	return nil
}

// GetRemoteAccessURL 获取远程访问URL
func (pm *PortalManager) GetRemoteAccessURL(deviceID string) (string, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if !pm.config.EnableRemote {
		return "", fmt.Errorf("远程访问未启用")
	}

	device, exists := pm.devices[deviceID]
	if !exists {
		return "", fmt.Errorf("设备不存在: %s", deviceID)
	}

	if device.Status != "online" {
		return "", fmt.Errorf("设备离线")
	}

	return fmt.Sprintf("%s/remote/%s", pm.config.APIEndpoint, deviceID), nil
}

// 内部方法

func (pm *PortalManager) syncLoop() {
	ticker := time.NewTicker(pm.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			pm.syncDeviceStatus()
		}
	}
}

func (pm *PortalManager) syncDeviceStatus() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()
	for _, device := range pm.devices {
		if device.ID == pm.config.DeviceID {
			continue // 跳过本机
		}

		// 超过5分钟未更新标记为离线
		if now.Sub(device.LastSeen) > 5*time.Minute {
			device.Status = "offline"
		}
	}
}

func (pm *PortalManager) healthCheckLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			// 更新本机状态
			pm.mu.Lock()
			if device, exists := pm.devices[pm.config.DeviceID]; exists {
				device.LastSeen = time.Now()
				device.Status = "online"
			}
			pm.mu.Unlock()
		}
	}
}

func generateDeviceID(name string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s_%d", name, time.Now().UnixNano())))
	return hex.EncodeToString(hash[:8])
}

func generateSessionID() string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("session_%d", time.Now().UnixNano())))
	return hex.EncodeToString(hash[:16])
}
