// Package networkmap provides network topology visualization for NAS-OS
// Features: Auto-discovery, topology map, bandwidth monitoring, device management
// Competitor benchmark: 对标群晖网络工具, 超越TrueNAS网络管理
package networkmap

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// DeviceType represents network device type.
type DeviceType string

const (
	DeviceNAS     DeviceType = "nas"
	DeviceRouter  DeviceType = "router"
	DeviceSwitch  DeviceType = "switch"
	DeviceAP      DeviceType = "access_point"
	DeviceServer  DeviceType = "server"
	DevicePC      DeviceType = "pc"
	DevicePhone   DeviceType = "phone"
	DeviceTablet  DeviceType = "tablet"
	DeviceTV      DeviceType = "tv"
	DevicePrinter DeviceType = "printer"
	DeviceCamera  DeviceType = "camera"
	DeviceIoT     DeviceType = "iot"
	DeviceUnknown DeviceType = "unknown"
)

// ConnectionType represents connection type.
type ConnectionType string

const (
	ConnEthernet  ConnectionType = "ethernet"
	ConnWiFi      ConnectionType = "wifi"
	ConnUSB       ConnectionType = "usb"
	ConnBluetooth ConnectionType = "bluetooth"
	ConnVPN       ConnectionType = "vpn"
)

// DeviceStatus represents device status.
type DeviceStatus string

const (
	StatusOnline  DeviceStatus = "online"
	StatusOffline DeviceStatus = "offline"
	StatusWarning DeviceStatus = "warning"
)

// NetworkDevice represents a discovered network device.
type NetworkDevice struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Hostname     string            `json:"hostname"`
	IPAddress    string            `json:"ip_address"`
	MACAddress   string            `json:"mac_address"`
	Type         DeviceType        `json:"type"`
	Manufacturer string            `json:"manufacturer"`
	Model        string            `json:"model"`
	OS           string            `json:"os"`
	Status       DeviceStatus      `json:"status"`
	Connection   ConnectionType    `json:"connection"`
	Ports        []PortInfo        `json:"ports"`
	Bandwidth    *BandwidthInfo    `json:"bandwidth"`
	FirstSeen    time.Time         `json:"first_seen"`
	LastSeen     time.Time         `json:"last_seen"`
	Tags         []string          `json:"tags"`
	Metadata     map[string]string `json:"metadata"`
}

// PortInfo represents a network port.
type PortInfo struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Service  string `json:"service"`
	State    string `json:"state"`
}

// BandwidthInfo represents bandwidth usage.
type BandwidthInfo struct {
	InboundBPS  int64     `json:"inbound_bps"`
	OutboundBPS int64     `json:"outbound_bps"`
	TotalBytes  int64     `json:"total_bytes"`
	Timestamp   time.Time `json:"timestamp"`
}

// NetworkLink represents a connection between devices.
type NetworkLink struct {
	ID        string         `json:"id"`
	SourceID  string         `json:"source_id"`
	TargetID  string         `json:"target_id"`
	Type      ConnectionType `json:"type"`
	Speed     string         `json:"speed"` // 1Gbps, 100Mbps, etc.
	Latency   int64          `json:"latency_ms"`
	Bandwidth *BandwidthInfo `json:"bandwidth"`
	IsActive  bool           `json:"is_active"`
}

// NetworkTopology represents the full network topology.
type NetworkTopology struct {
	Devices   []*NetworkDevice `json:"devices"`
	Links     []*NetworkLink   `json:"links"`
	Subnets   []*SubnetInfo    `json:"subnets"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// SubnetInfo represents network subnet information.
type SubnetInfo struct {
	CIDR        string `json:"cidr"`
	Gateway     string `json:"gateway"`
	DHCPRange   string `json:"dhcp_range"`
	VLAN        int    `json:"vlan"`
	DeviceCount int    `json:"device_count"`
}

// ScanResult represents a network scan result.
type ScanResult struct {
	ScanID       string           `json:"scan_id"`
	StartTime    time.Time        `json:"start_time"`
	EndTime      time.Time        `json:"end_time"`
	Subnet       string           `json:"subnet"`
	DevicesFound int              `json:"devices_found"`
	NewDevices   int              `json:"new_devices"`
	Devices      []*NetworkDevice `json:"devices"`
}

// NetworkStats represents network statistics.
type NetworkStats struct {
	TotalDevices   int              `json:"total_devices"`
	OnlineDevices  int              `json:"online_devices"`
	OfflineDevices int              `json:"offline_devices"`
	TotalBandwidth int64            `json:"total_bandwidth_bps"`
	ByType         map[string]int   `json:"by_type"`
	ByConnection   map[string]int   `json:"by_connection"`
	TopTalkers     []*NetworkDevice `json:"top_talkers"`
	LastScan       time.Time        `json:"last_scan"`
}

// AlertType represents network alert types.
type AlertType string

const (
	AlertNewDevice      AlertType = "new_device"
	AlertDeviceLeft     AlertType = "device_left"
	AlertHighBandwidth  AlertType = "high_bandwidth"
	AlertPortScan       AlertType = "port_scan"
	AlertUnusualTraffic AlertType = "unusual_traffic"
)

// NetworkAlert represents a network alert.
type NetworkAlert struct {
	ID        string    `json:"id"`
	Type      AlertType `json:"type"`
	DeviceID  string    `json:"device_id"`
	Message   string    `json:"message"`
	Severity  string    `json:"severity"` // info, warning, critical
	Timestamp time.Time `json:"timestamp"`
	Acked     bool      `json:"acked"`
}

// Config holds network map configuration.
type Config struct {
	Enabled          bool     `json:"enabled"`
	ScanInterval     int      `json:"scan_interval_minutes"`
	Subnets          []string `json:"subnets"`
	AutoDiscover     bool     `json:"auto_discover"`
	BandwidthMonitor bool     `json:"bandwidth_monitor"`
	AlertNewDevices  bool     `json:"alert_new_devices"`
	MaxDevices       int      `json:"max_devices"`
	ScanPorts        bool     `json:"scan_ports"`
	CommonPorts      []int    `json:"common_ports"`
}

// Manager manages network topology.
type Manager struct {
	config   *Config
	devices  map[string]*NetworkDevice
	links    map[string]*NetworkLink
	topology *NetworkTopology
	stats    *NetworkStats
	alerts   []*NetworkAlert
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewManager creates a new network map manager.
func NewManager(config *Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		config:  config,
		devices: make(map[string]*NetworkDevice),
		links:   make(map[string]*NetworkLink),
		topology: &NetworkTopology{
			Devices:   make([]*NetworkDevice, 0),
			Links:     make([]*NetworkLink, 0),
			Subnets:   make([]*SubnetInfo, 0),
			UpdatedAt: time.Now(),
		},
		stats: &NetworkStats{
			ByType:       make(map[string]int),
			ByConnection: make(map[string]int),
		},
		alerts: make([]*NetworkAlert, 0),
		ctx:    ctx,
		cancel: cancel,
	}
}

// AddDevice adds or updates a network device.
func (m *Manager) AddDevice(device *NetworkDevice) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, exists := m.devices[device.ID]; exists {
		existing.LastSeen = time.Now()
		existing.Status = StatusOnline
		existing.IPAddress = device.IPAddress
		if device.Name != "" {
			existing.Name = device.Name
		}
		return nil
	}

	device.FirstSeen = time.Now()
	device.LastSeen = time.Now()
	device.Status = StatusOnline
	m.devices[device.ID] = device

	m.stats.TotalDevices = len(m.devices)
	m.stats.ByType[string(device.Type)]++
	m.stats.ByConnection[string(device.Connection)]++

	if m.config.AlertNewDevices {
		m.addAlert(AlertNewDevice, device.ID, fmt.Sprintf("New device: %s (%s)", device.Name, device.IPAddress), "info")
	}

	return nil
}

// RemoveDevice removes a device.
func (m *Manager) RemoveDevice(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[id]
	if !exists {
		return fmt.Errorf("device not found: %s", id)
	}

	device.Status = StatusOffline
	m.stats.TotalDevices = len(m.devices) - 1
	m.stats.OnlineDevices--

	return nil
}

// AddLink adds a network link.
func (m *Manager) AddLink(link *NetworkLink) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	link.IsActive = true
	m.links[link.ID] = link
	return nil
}

// ScanSubnet scans a subnet for devices.
func (m *Manager) ScanSubnet(cidr string) (*ScanResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR: %v", err)
	}

	result := &ScanResult{
		ScanID:    fmt.Sprintf("scan-%d", time.Now().Unix()),
		StartTime: time.Now(),
		Subnet:    cidr,
		Devices:   make([]*NetworkDevice, 0),
	}

	// Simulate device discovery
	for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); inc(ip) {
		// Check if device exists
		for _, device := range m.devices {
			if device.IPAddress == ip.String() {
				device.LastSeen = time.Now()
				device.Status = StatusOnline
				result.Devices = append(result.Devices, device)
			}
		}
	}

	result.EndTime = time.Now()
	result.DevicesFound = len(result.Devices)
	return result, nil
}

// GetTopology returns the full network topology.
func (m *Manager) GetTopology() *NetworkTopology {
	m.mu.RLock()
	defer m.mu.RUnlock()

	topo := &NetworkTopology{
		Devices:   make([]*NetworkDevice, 0, len(m.devices)),
		Links:     make([]*NetworkLink, 0, len(m.links)),
		Subnets:   m.topology.Subnets,
		UpdatedAt: time.Now(),
	}

	for _, d := range m.devices {
		topo.Devices = append(topo.Devices, d)
	}
	for _, l := range m.links {
		topo.Links = append(topo.Links, l)
	}

	return topo
}

// GetDevice returns a device by ID.
func (m *Manager) GetDevice(id string) (*NetworkDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	device, exists := m.devices[id]
	if !exists {
		return nil, fmt.Errorf("device not found: %s", id)
	}
	return device, nil
}

// ListDevices returns all devices.
func (m *Manager) ListDevices() []*NetworkDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()
	devices := make([]*NetworkDevice, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, d)
	}
	return devices
}

// GetStats returns network statistics.
func (m *Manager) GetStats() *NetworkStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.stats.OnlineDevices = 0
	m.stats.OfflineDevices = 0
	for _, d := range m.devices {
		if d.Status == StatusOnline {
			m.stats.OnlineDevices++
		} else {
			m.stats.OfflineDevices++
		}
	}
	return m.stats
}

// GetAlerts returns network alerts.
func (m *Manager) GetAlerts(unackedOnly bool) []*NetworkAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !unackedOnly {
		return m.alerts
	}
	var alerts []*NetworkAlert
	for _, a := range m.alerts {
		if !a.Acked {
			alerts = append(alerts, a)
		}
	}
	return alerts
}

func (m *Manager) addAlert(alertType AlertType, deviceID, message, severity string) {
	m.alerts = append(m.alerts, &NetworkAlert{
		ID:        fmt.Sprintf("alert-%d", len(m.alerts)+1),
		Type:      alertType,
		DeviceID:  deviceID,
		Message:   message,
		Severity:  severity,
		Timestamp: time.Now(),
	})
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// Stop stops the network map manager.
func (m *Manager) Stop() {
	m.cancel()
}
