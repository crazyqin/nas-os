// Package appleshare 提供 Apple 生态核心管理逻辑
package appleshare

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager Apple 生态管理器
type Manager struct {
	mu                sync.RWMutex
	logger            *zap.Logger
	config            *AppleShareConfig
	devices           map[string]*AirPlayDevice
	timeMachineShares map[string]*TimeMachineShare
	spotlightIndexes  map[string]*SpotlightIndex
	smbConfig         *SMBConfig
	stopChan          chan struct{}
	running           bool
}

// NewManager 创建 Apple 生态管理器
func NewManager(config *AppleShareConfig) *Manager {
	if config == nil {
		config = DefaultAppleShareConfig()
	}

	m := &Manager{
		logger:            zap.NewNop(),
		config:            config,
		devices:           make(map[string]*AirPlayDevice),
		timeMachineShares: make(map[string]*TimeMachineShare),
		spotlightIndexes:  make(map[string]*SpotlightIndex),
		smbConfig:         &SMBConfig{},
		stopChan:          make(chan struct{}),
	}

	return m
}

// NewManagerWithLogger 创建带 logger 的 Apple 生态管理器
func NewManagerWithLogger(logger *zap.Logger, config *AppleShareConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	m := NewManager(config)
	m.logger = logger
	return m
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// DiscoverAirPlayDevices 发现 AirPlay 设备 (mDNS/Bonjour)
func (m *Manager) DiscoverAirPlayDevices(ctx context.Context) ([]AirPlayDevice, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("apple share module is disabled")
	}

	m.logger.Info("starting AirPlay device discovery")

	// 使用 mDNS 发现设备
	discoveredDevices := make([]AirPlayDevice, 0)

	// 模拟 mDNS 查询 - 实际实现应使用 mdns 库
	// 这里使用简化的网络扫描作为演示
	timeout := time.Duration(m.config.DiscoveryTimeout) * time.Second
	devices, err := m.scanForAirPlayDevices(ctx, timeout)
	if err != nil {
		m.logger.Warn("device discovery failed", zap.Error(err))
		// 返回已缓存的设备
		return m.getCachedDevices(), nil
	}

	// 更新设备缓存
	m.mu.Lock()
	for _, device := range devices {
		m.devices[device.ID] = &device
		discoveredDevices = append(discoveredDevices, device)
	}
	m.mu.Unlock()

	m.logger.Info("AirPlay device discovery completed",
		zap.Int("devices_found", len(discoveredDevices)))

	return discoveredDevices, nil
}

// scanForAirPlayDevices 扫描 AirPlay 设备
func (m *Manager) scanForAirPlayDevices(ctx context.Context, timeout time.Duration) ([]AirPlayDevice, error) {
	devices := make([]AirPlayDevice, 0)

	// AirPlay 使用端口 7000 和 7100
	airplayPorts := []int{7000, 7100}

	// 获取本地子网
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get network interfaces: %w", err)
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.To4() == nil {
				continue
			}

			// 扫描子网中的 AirPlay 设备
			baseIP := ipNet.IP.Mask(ipNet.Mask)
			for i := 1; i < 255; i++ {
				select {
				case <-ctx.Done():
					return devices, ctx.Err()
				default:
				}

				ip := net.IPv4(baseIP[0], baseIP[1], baseIP[2], byte(i))
				for _, port := range airplayPorts {
					addr := net.JoinHostPort(ip.String(), fmt.Sprintf("%d", port))
					conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
					if err != nil {
						continue
					}
					conn.Close()

					// 发现 AirPlay 设备
					device := AirPlayDevice{
						ID:            generateID(),
						Name:          fmt.Sprintf("AirPlay Device %s", ip.String()),
						IP:            ip.String(),
						Port:          port,
						Type:          AirPlayTypeReceiver,
						Status:        AirPlayStatusOnline,
						SupportsVideo: true,
						SupportsAudio: true,
						LastSeen:      time.Now(),
					}
					devices = append(devices, device)
				}
			}
		}
	}

	return devices, nil
}

// getCachedDevices 获取缓存的设备列表
func (m *Manager) getCachedDevices() []AirPlayDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]AirPlayDevice, 0, len(m.devices))
	for _, d := range m.devices {
		// 标记超过 5 分钟未见的设备为离线
		if time.Since(d.LastSeen) > 5*time.Minute {
			d.Status = AirPlayStatusOffline
		}
		devices = append(devices, *d)
	}
	return devices
}

// CreateTimeMachineShare 创建 Time Machine 共享
func (m *Manager) CreateTimeMachineShare(name, path string, quota int64) (*TimeMachineShare, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("apple share module is disabled")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查名称是否已存在
	for _, share := range m.timeMachineShares {
		if share.Name == name {
			return nil, fmt.Errorf("share with name %s already exists", name)
		}
	}

	share := &TimeMachineShare{
		ID:         generateID(),
		Name:       name,
		Path:       path,
		Quota:      quota,
		UsedSpace:  0,
		SMBEnabled: true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	m.timeMachineShares[share.ID] = share

	m.logger.Info("Time Machine share created",
		zap.String("id", share.ID),
		zap.String("name", share.Name),
		zap.String("path", share.Path))

	return share, nil
}

// GetTimeMachineStatus 获取 Time Machine 共享状态
func (m *Manager) GetTimeMachineStatus(shareID string) (*TimeMachineShare, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	share, ok := m.timeMachineShares[shareID]
	if !ok {
		return nil, fmt.Errorf("time machine share not found: %s", shareID)
	}

	// 模拟更新使用空间（实际应查询文件系统）
	shareCopy := *share
	return &shareCopy, nil
}

// ListTimeMachineShares 列出所有 Time Machine 共享
func (m *Manager) ListTimeMachineShares() []*TimeMachineShare {
	m.mu.RLock()
	defer m.mu.RUnlock()

	shares := make([]*TimeMachineShare, 0, len(m.timeMachineShares))
	for _, s := range m.timeMachineShares {
		shares = append(shares, s)
	}
	return shares
}

// UpdateSMBConfig 更新 Apple 优化的 SMB 配置
func (m *Manager) UpdateSMBConfig(config SMBConfig) error {
	if !m.config.Enabled {
		return fmt.Errorf("apple share module is disabled")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.smbConfig = &config

	m.logger.Info("SMB config updated",
		zap.Bool("signing", config.Signing),
		zap.Bool("aapl_extensions", config.AAPLExtensions),
		zap.Bool("streams", config.Streams),
		zap.Bool("vfs_fruit", config.VFSFruitEnabled),
		zap.Bool("spotlight", config.SpotlightEnabled))

	return nil
}

// GetSMBConfig 获取当前 SMB 配置
func (m *Manager) GetSMBConfig() *SMBConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config := *m.smbConfig
	return &config
}

// RebuildSpotlightIndex 重建 Spotlight 索引
func (m *Manager) RebuildSpotlightIndex(volumeID string) error {
	if !m.config.Enabled {
		return fmt.Errorf("apple share module is disabled")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已有索引
	index, exists := m.spotlightIndexes[volumeID]
	if exists && index.Status == SpotlightStatusIndexing {
		return fmt.Errorf("spotlight index for volume %s is already being rebuilt", volumeID)
	}

	// 创建或更新索引
	if !exists {
		index = &SpotlightIndex{
			ID:        generateID(),
			VolumeID:  volumeID,
			IndexPath: fmt.Sprintf("%s/%s", m.config.SpotlightIndexPath, volumeID),
			Status:    SpotlightStatusIndexing,
			UpdatedAt: time.Now(),
		}
		m.spotlightIndexes[volumeID] = index
	} else {
		index.Status = SpotlightStatusIndexing
		index.Progress = 0
		index.UpdatedAt = time.Now()
	}

	m.logger.Info("Spotlight index rebuild started",
		zap.String("volume_id", volumeID),
		zap.String("index_path", index.IndexPath))

	// 模拟索引重建过程（实际应异步执行）
	go m.simulateIndexRebuild(volumeID)

	return nil
}

// simulateIndexRebuild 模拟索引重建过程
func (m *Manager) simulateIndexRebuild(volumeID string) {
	totalSteps := 10
	for i := 0; i <= totalSteps; i++ {
		time.Sleep(100 * time.Millisecond)

		m.mu.Lock()
		index, exists := m.spotlightIndexes[volumeID]
		if !exists {
			m.mu.Unlock()
			return
		}

		index.Progress = float64(i) / float64(totalSteps) * 100
		index.TotalFiles = int64(i * 1000)
		index.IndexSize = int64(i * 1024 * 1024) // 每步 1MB
		index.UpdatedAt = time.Now()

		if i == totalSteps {
			index.Status = SpotlightStatusIdle
			index.LastIndexed = time.Now()
			m.logger.Info("Spotlight index rebuild completed",
				zap.String("volume_id", volumeID))
		}
		m.mu.Unlock()
	}
}

// GetSpotlightStatus 获取 Spotlight 索引状态
func (m *Manager) GetSpotlightStatus(volumeID string) (*SpotlightIndex, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	index, ok := m.spotlightIndexes[volumeID]
	if !ok {
		return nil, fmt.Errorf("spotlight index not found for volume: %s", volumeID)
	}

	indexCopy := *index
	return &indexCopy, nil
}

// ListSpotlightIndexes 列出所有 Spotlight 索引
func (m *Manager) ListSpotlightIndexes() []*SpotlightIndex {
	m.mu.RLock()
	defer m.mu.RUnlock()

	indexes := make([]*SpotlightIndex, 0, len(m.spotlightIndexes))
	for _, idx := range m.spotlightIndexes {
		indexes = append(indexes, idx)
	}
	return indexes
}

// GetConnectedClients 获取已连接的客户端
func (m *Manager) GetConnectedClients() ([]AirPlayDevice, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("apple share module is disabled")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	clients := make([]AirPlayDevice, 0)
	for _, device := range m.devices {
		if device.Status == AirPlayStatusOnline || device.Status == AirPlayStatusStreaming {
			clients = append(clients, *device)
		}
	}

	return clients, nil
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *AppleShareConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *AppleShareConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}

// GetDevice 获取指定设备
func (m *Manager) GetDevice(deviceID string) (*AirPlayDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return nil, fmt.Errorf("device not found: %s", deviceID)
	}

	deviceCopy := *device
	return &deviceCopy, nil
}

// ListDevices 列出所有设备
func (m *Manager) ListDevices() []AirPlayDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]AirPlayDevice, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, *d)
	}
	return devices
}

// RemoveDevice 移除设备
func (m *Manager) RemoveDevice(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.devices[deviceID]; !ok {
		return fmt.Errorf("device not found: %s", deviceID)
	}

	delete(m.devices, deviceID)
	m.logger.Info("device removed", zap.String("device_id", deviceID))
	return nil
}
