package pxeboot

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// PXEBootManager PXE启动管理器
type PXEBootManager struct {
	mu      sync.RWMutex
	clients map[string]*PXEClient
	images  map[string]*BootImage
	config  *PXEConfig
}

// PXEConfig PXE配置
type PXEConfig struct {
	Enabled    bool   `json:"enabled"`
	TFTPServer string `json:"tftp_server"`
	TFTPPort   int    `json:"tftp_port"`
	DHCPRange  string `json:"dhcp_range"`
	BootFile   string `json:"boot_file"`
	RootPath   string `json:"root_path"`
}

// PXEClient PXE客户端
type PXEClient struct {
	ID        string    `json:"id"`
	MAC       string    `json:"mac"`
	IP        string    `json:"ip"`
	Hostname  string    `json:"hostname"`
	Status    string    `json:"status"`
	BootImage string    `json:"boot_image"`
	LastBoot  time.Time `json:"last_boot"`
	CreatedAt time.Time `json:"created_at"`
}

// BootImage 启动镜像
type BootImage struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	Type        string    `json:"type"`
	Version     string    `json:"version"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

// NewPXEBootManager 创建PXE启动管理器
func NewPXEBootManager(config *PXEConfig) *PXEBootManager {
	if config == nil {
		config = &PXEConfig{
			Enabled:    true,
			TFTPServer: "0.0.0.0",
			TFTPPort:   69,
			BootFile:   "pxelinux.0",
			RootPath:   "/var/lib/tftpboot",
		}
	}
	return &PXEBootManager{
		clients: make(map[string]*PXEClient),
		images:  make(map[string]*BootImage),
		config:  config,
	}
}

// RegisterClient 注册客户端
func (m *PXEBootManager) RegisterClient(mac, hostname string) (*PXEClient, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if mac == "" {
		return nil, fmt.Errorf("MAC address is required")
	}

	// 验证MAC地址
	_, err := net.ParseMAC(mac)
	if err != nil {
		return nil, fmt.Errorf("invalid MAC address: %w", err)
	}

	// 检查是否已注册
	for _, client := range m.clients {
		if client.MAC == mac {
			return client, nil
		}
	}

	client := &PXEClient{
		ID:        fmt.Sprintf("pxe_%d", time.Now().UnixNano()),
		MAC:       mac,
		Hostname:  hostname,
		Status:    "registered",
		CreatedAt: time.Now(),
	}

	m.clients[client.ID] = client
	return client, nil
}

// UnregisterClient 注销客户端
func (m *PXEBootManager) UnregisterClient(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[id]; !exists {
		return fmt.Errorf("client not found: %s", id)
	}

	delete(m.clients, id)
	return nil
}

// GetClient 获取客户端
func (m *PXEBootManager) GetClient(id string) (*PXEClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, exists := m.clients[id]
	if !exists {
		return nil, fmt.Errorf("client not found: %s", id)
	}
	return client, nil
}

// ListClients 列出所有客户端
func (m *PXEBootManager) ListClients() []*PXEClient {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clients := make([]*PXEClient, 0, len(m.clients))
	for _, client := range m.clients {
		clients = append(clients, client)
	}
	return clients
}

// AddBootImage 添加启动镜像
func (m *PXEBootManager) AddBootImage(image *BootImage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if image.Name == "" {
		return fmt.Errorf("image name is required")
	}

	if image.Path == "" {
		return fmt.Errorf("image path is required")
	}

	if image.ID == "" {
		image.ID = fmt.Sprintf("img_%d", time.Now().UnixNano())
	}

	image.Enabled = true
	image.CreatedAt = time.Now()

	m.images[image.ID] = image
	return nil
}

// RemoveBootImage 移除启动镜像
func (m *PXEBootManager) RemoveBootImage(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.images[id]; !exists {
		return fmt.Errorf("image not found: %s", id)
	}

	delete(m.images, id)
	return nil
}

// GetBootImage 获取启动镜像
func (m *PXEBootManager) GetBootImage(id string) (*BootImage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	image, exists := m.images[id]
	if !exists {
		return nil, fmt.Errorf("image not found: %s", id)
	}
	return image, nil
}

// ListBootImages 列出所有启动镜像
func (m *PXEBootManager) ListBootImages() []*BootImage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	images := make([]*BootImage, 0, len(m.images))
	for _, image := range m.images {
		images = append(images, image)
	}
	return images
}

// SetClientBootImage 设置客户端启动镜像
func (m *PXEBootManager) SetClientBootImage(clientID, imageID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[clientID]
	if !exists {
		return fmt.Errorf("client not found: %s", clientID)
	}

	image, exists := m.images[imageID]
	if !exists {
		return fmt.Errorf("image not found: %s", imageID)
	}

	if !image.Enabled {
		return fmt.Errorf("image is disabled: %s", imageID)
	}

	client.BootImage = imageID
	return nil
}

// BootClient 启动客户端
func (m *PXEBootManager) BootClient(clientID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[clientID]
	if !exists {
		return fmt.Errorf("client not found: %s", clientID)
	}

	if client.BootImage == "" {
		return fmt.Errorf("no boot image assigned to client: %s", clientID)
	}

	client.Status = "booting"
	client.LastBoot = time.Now()

	// 模拟启动过程
	go func() {
		time.Sleep(5 * time.Second)
		m.mu.Lock()
		defer m.mu.Unlock()
		client.Status = "running"
	}()

	return nil
}

// GetClientByMAC 根据MAC地址获取客户端
func (m *PXEBootManager) GetClientByMAC(mac string) (*PXEClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, client := range m.clients {
		if client.MAC == mac {
			return client, nil
		}
	}

	return nil, fmt.Errorf("client not found with MAC: %s", mac)
}

// GetStats 获取统计信息
func (m *PXEBootManager) GetStats() *PXEStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &PXEStats{
		TotalClients: len(m.clients),
		TotalImages:  len(m.images),
	}

	for _, client := range m.clients {
		switch client.Status {
		case "running":
			stats.RunningClients++
		case "booting":
			stats.BootingClients++
		case "registered":
			stats.RegisteredClients++
		}
	}

	for _, image := range m.images {
		if image.Enabled {
			stats.EnabledImages++
		}
	}

	return stats
}

// PXEStats PXE统计
type PXEStats struct {
	TotalClients      int `json:"total_clients"`
	RunningClients    int `json:"running_clients"`
	BootingClients    int `json:"booting_clients"`
	RegisteredClients int `json:"registered_clients"`
	TotalImages       int `json:"total_images"`
	EnabledImages     int `json:"enabled_images"`
}
