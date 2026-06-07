package pxe

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager manages PXE boot services (TFTP + HTTP boot)
type Manager struct {
	mu           sync.RWMutex
	server       PXEServer
	config       PXEConfig
	clients      map[string]PXEClient // key: MAC address
	images       map[string]PXEImage  // key: image ID
	running      bool
	totalBoots   int
	successBoots int
}

// NewPXEManager creates a new PXE manager with default mock data
func NewPXEManager() *Manager {
	m := &Manager{
		clients: make(map[string]PXEClient),
		images:  make(map[string]PXEImage),
		server: PXEServer{
			IP:         "192.168.1.1",
			SubnetMask: "255.255.255.0",
			Gateway:    "192.168.1.1",
			DNS:        "8.8.8.8",
			BootFile:   "ipxe.efi",
			Interface:  "eth0",
			Enabled:    true,
			Status:     "stopped",
		},
		config: PXEConfig{
			TFTPPath:  "/var/lib/pxe/tftpboot",
			HTTPPath:  "/var/lib/pxe/httpboot",
			DHCPRange: "192.168.1.100-192.168.1.200",
			LogLevel:  "info",
			BootMenu: []BootMenuItem{
				{
					ID:      uuid.New().String(),
					Label:   "Ubuntu 24.04 LTS",
					ImageID: "img-ubuntu-2404",
					Kernel:  "/images/ubuntu-2404/vmlinuz",
					Initrd:  "/images/ubuntu-2404/initrd",
					Default: true,
				},
				{
					ID:      uuid.New().String(),
					Label:   "Rescue Shell",
					ImageID: "img-rescue",
					Kernel:  "/images/rescue/vmlinuz",
					Initrd:  "/images/rescue/initrd",
					Default: false,
				},
			},
		},
		running:      false,
		totalBoots:   42,
		successBoots: 39,
	}

	m.addMockClients()
	m.addMockImages()

	return m
}

func (m *Manager) addMockClients() {
	mockClients := []struct {
		mac      string
		ip       string
		hostname string
		image    string
	}{
		{"aa:bb:cc:dd:ee:01", "192.168.1.101", "node-01", "img-ubuntu-2404"},
		{"aa:bb:cc:dd:ee:02", "192.168.1.102", "node-02", "img-ubuntu-2404"},
		{"aa:bb:cc:dd:ee:03", "192.168.1.103", "node-03", "img-rescue"},
		{"aa:bb:cc:dd:ee:04", "192.168.1.104", "node-04", ""},
	}
	for _, mc := range mockClients {
		c := PXEClient{
			MACAddress: mc.mac,
			IP:         mc.ip,
			Hostname:   mc.hostname,
			LastBoot:   time.Now().Add(-time.Duration(time.Now().Unix()%120) * time.Minute),
			Status:     "offline",
			OSImage:    mc.image,
		}
		if mc.image != "" {
			c.Status = "online"
		}
		m.clients[mc.mac] = c
	}
}

func (m *Manager) addMockImages() {
	mockImages := []PXEImage{
		{ID: "img-ubuntu-2404", Name: "Ubuntu 24.04 LTS", Path: "/var/lib/pxe/images/ubuntu-2404", Size: 1073741824, Type: "linux", CreatedAt: time.Now().Add(-30 * 24 * time.Hour)},
		{ID: "img-windows-11", Name: "Windows 11 Pro", Path: "/var/lib/pxe/images/windows-11", Size: 5368709120, Type: "windows", CreatedAt: time.Now().Add(-15 * 24 * time.Hour)},
		{ID: "img-rescue", Name: "SystemRescue", Path: "/var/lib/pxe/images/rescue", Size: 536870912, Type: "rescue", CreatedAt: time.Now().Add(-7 * 24 * time.Hour)},
	}
	for _, img := range mockImages {
		m.images[img.ID] = img
	}
}

// Start starts the PXE services (TFTP + HTTP boot)
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("PXE server already running")
	}

	m.running = true
	m.server.Status = "running"
	return nil
}

// Stop stops the PXE services
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return fmt.Errorf("PXE server not running")
	}

	m.running = false
	m.server.Status = "stopped"
	return nil
}

// GetConfig returns the current PXE configuration
func (m *Manager) GetConfig() PXEConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig updates PXE configuration fields
func (m *Manager) UpdateConfig(req CreatePXEConfigRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.TFTPPath != nil {
		m.config.TFTPPath = *req.TFTPPath
	}
	if req.HTTPPath != nil {
		m.config.HTTPPath = *req.HTTPPath
	}
	if req.DHCPRange != nil {
		m.config.DHCPRange = *req.DHCPRange
	}
	if req.LogLevel != nil {
		m.config.LogLevel = *req.LogLevel
	}
	return nil
}

// ConfigureTFTP sets the TFTP root path
func (m *Manager) ConfigureTFTP(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if path == "" {
		return fmt.Errorf("TFTP path cannot be empty")
	}
	m.config.TFTPPath = path
	return nil
}

// ConfigureDHCP sets the DHCP address range
func (m *Manager) ConfigureDHCP(startIP, endIP string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if startIP == "" || endIP == "" {
		return fmt.Errorf("DHCP range requires both start and end IP")
	}
	m.config.DHCPRange = startIP + "-" + endIP
	return nil
}

// AddBootImage registers a new boot image
func (m *Manager) AddBootImage(image PXEImage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if image.ID == "" {
		image.ID = uuid.New().String()
	}
	if image.Name == "" {
		return fmt.Errorf("image name is required")
	}
	if image.Type == "" {
		return fmt.Errorf("image type is required")
	}

	// Check duplicate ID
	if _, exists := m.images[image.ID]; exists {
		return fmt.Errorf("image with ID %s already exists", image.ID)
	}

	if image.CreatedAt.IsZero() {
		image.CreatedAt = time.Now()
	}

	m.images[image.ID] = image
	return nil
}

// RemoveBootImage removes a boot image by ID
func (m *Manager) RemoveBootImage(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.images[id]; !ok {
		return fmt.Errorf("image not found: %s", id)
	}
	delete(m.images, id)
	return nil
}

// ListImages returns all registered boot images
func (m *Manager) ListImages() []PXEImage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	images := make([]PXEImage, 0, len(m.images))
	for _, img := range m.images {
		images = append(images, img)
	}
	return images
}

// GetImage returns a boot image by ID
func (m *Manager) GetImage(id string) (*PXEImage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	img, ok := m.images[id]
	if !ok {
		return nil, fmt.Errorf("image not found: %s", id)
	}
	return &img, nil
}

// ListClients returns all known PXE clients
func (m *Manager) ListClients() []PXEClient {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clients := make([]PXEClient, 0, len(m.clients))
	for _, c := range m.clients {
		clients = append(clients, c)
	}
	return clients
}

// GetClientByMAC returns a client by MAC address
func (m *Manager) GetClientByMAC(mac string) (*PXEClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, ok := m.clients[mac]
	if !ok {
		return nil, fmt.Errorf("client not found: %s", mac)
	}
	return &c, nil
}

// UpdateClient updates an existing PXE client
func (m *Manager) UpdateClient(mac string, req UpdateClientRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.clients[mac]
	if !ok {
		return fmt.Errorf("client not found: %s", mac)
	}

	if req.Hostname != nil {
		c.Hostname = *req.Hostname
	}
	if req.OSImage != nil {
		c.OSImage = *req.OSImage
	}
	if req.Status != nil {
		c.Status = *req.Status
	}

	m.clients[mac] = c
	return nil
}

// SetBootMenu replaces the entire boot menu
func (m *Manager) SetBootMenu(menu []BootMenuItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.BootMenu = menu
	return nil
}

// GetStats returns aggregated PXE statistics
func (m *Manager) GetStats() PXEStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := PXEStats{
		TotalClients: len(m.clients),
		TotalImages:  len(m.images),
	}

	for _, c := range m.clients {
		if c.Status == "online" || c.Status == "booting" {
			stats.ActiveClients++
		}
	}

	if m.totalBoots > 0 {
		stats.BootSuccessRate = float64(m.successBoots) / float64(m.totalBoots) * 100
	}

	return stats
}
