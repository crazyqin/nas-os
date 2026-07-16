// Package smartnicoffload 实现 SmartNIC/DPU 网络卸载管理
// 支持 SmartNIC 设备发现、网络功能卸载（OVS/IPsec/压缩）、流量编程和性能监控
package smartnicoffload

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrDeviceNotFound      = errors.New("SmartNIC device not found")
	ErrDeviceExists        = errors.New("SmartNIC device already exists")
	ErrDeviceNotReady      = errors.New("SmartNIC device not ready")
	ErrOffloadNotFound     = errors.New("offload function not found")
	ErrOffloadExists       = errors.New("offload function already exists")
	ErrOffloadNotSupported = errors.New("offload not supported by device")
	ErrManagerClosed       = errors.New("manager closed")
	ErrInvalidConfig       = errors.New("invalid configuration")
)

// DeviceType 设备类型.
type DeviceType string

const (
	DeviceTypeSmartNIC DeviceType = "smartnic" // 智能网卡
	DeviceTypeDPU      DeviceType = "dpu"      // 数据处理单元
	DeviceTypeIPU      DeviceType = "ipu"      // 基础设施处理单元
)

// DeviceState 设备状态.
type DeviceState string

const (
	StateInit     DeviceState = "init"
	StateReady    DeviceState = "ready"
	StateRunning  DeviceState = "running"
	StateError    DeviceState = "error"
	StateDisabled DeviceState = "disabled"
)

// OffloadType 卸载功能类型.
type OffloadType string

const (
	OffloadOVS         OffloadType = "ovs"          // Open vSwitch
	OffloadIPsec       OffloadType = "ipsec"        // IPsec 加解密
	OffloadTLS         OffloadType = "tls"          // TLS 终结
	OffloadCompress    OffloadType = "compress"     // 数据压缩
	OffloadDedup       OffloadType = "dedup"        // 数据去重
	OffloadFirewall    OffloadType = "firewall"     // 防火墙
	OffloadNAT         OffloadType = "nat"          // NAT
	OffloadVxLAN       OffloadType = "vxlan"        // VXLAN 封装
	OffloadGRE         OffloadType = "gre"          // GRE 封装
	OffloadRDMA        OffloadType = "rdma"         // RDMA
	OffloadRegEx       OffloadType = "regex"        // 正则匹配
	OffloadMLInference OffloadType = "ml_inference" // ML推理
)

// OffloadState 卸载状态.
type OffloadState string

const (
	OffloadStateDisabled OffloadState = "disabled"
	OffloadStateEnabled  OffloadState = "enabled"
	OffloadStateError    OffloadState = "error"
)

// SmartNICDevice SmartNIC 设备.
type SmartNICDevice struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Type        DeviceType    `json:"type"`
	State       DeviceState   `json:"state"`
	PCIeAddr    string        `json:"pcie_addr"`
	VendorID    string        `json:"vendor_id"`
	DeviceID    string        `json:"device_id"`
	SerialNum   string        `json:"serial_num"`
	FirmwareVer string        `json:"firmware_ver"`
	NumCores    int           `json:"num_cores"`
	Memory      uint64        `json:"memory"` // bytes
	NumPorts    int           `json:"num_ports"`
	MaxOffloads int           `json:"max_offloads"`
	Offloads    []OffloadType `json:"supported_offloads"`
	Speed       uint64        `json:"speed"` // Gbps
	Temperature float64       `json:"temperature"`
	PowerUsage  float64       `json:"power_usage"`
	Stats       DeviceStats   `json:"stats"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// DeviceStats 设备统计.
type DeviceStats struct {
	RXPackets      uint64  `json:"rx_packets"`
	TXPackets      uint64  `json:"tx_packets"`
	RXBytes        uint64  `json:"rx_bytes"`
	TXBytes        uint64  `json:"tx_bytes"`
	OffloadedPkts  uint64  `json:"offloaded_packets"`
	OffloadedBytes uint64  `json:"offloaded_bytes"`
	CPUUsage       float64 `json:"cpu_usage"`    // %
	MemoryUsage    float64 `json:"memory_usage"` // %
}

// OffloadFunction 卸载功能实例.
type OffloadFunction struct {
	ID        string            `json:"id"`
	DeviceID  string            `json:"device_id"`
	Type      OffloadType       `json:"type"`
	State     OffloadState      `json:"state"`
	Config    map[string]string `json:"config,omitempty"`
	Stats     OffloadStats      `json:"stats"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// OffloadStats 卸载统计.
type OffloadStats struct {
	ProcessedPackets uint64  `json:"processed_packets"`
	ProcessedBytes   uint64  `json:"processed_bytes"`
	ErrorCount       uint64  `json:"error_count"`
	AvgLatencyUs     float64 `json:"avg_latency_us"`
	ThroughputGbps   float64 `json:"throughput_gbps"`
}

// Manager SmartNIC 卸载管理器.
type Manager struct {
	mu       sync.RWMutex
	devices  map[string]*SmartNICDevice
	offloads map[string]*OffloadFunction
	closed   bool
	stopCh   chan struct{}
}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		devices:  make(map[string]*SmartNICDevice),
		offloads: make(map[string]*OffloadFunction),
		stopCh:   make(chan struct{}),
	}
}

// RegisterDevice 注册设备.
func (m *Manager) RegisterDevice(dev *SmartNICDevice) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrManagerClosed
	}
	if _, exists := m.devices[dev.ID]; exists {
		return ErrDeviceExists
	}
	dev.State = StateReady
	dev.CreatedAt = time.Now()
	dev.UpdatedAt = time.Now()
	m.devices[dev.ID] = dev
	return nil
}

// UnregisterDevice 注销设备.
func (m *Manager) UnregisterDevice(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.devices[id]; !exists {
		return ErrDeviceNotFound
	}
	delete(m.devices, id)
	return nil
}

// GetDevice 获取设备.
func (m *Manager) GetDevice(id string) (*SmartNICDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	dev, exists := m.devices[id]
	if !exists {
		return nil, ErrDeviceNotFound
	}
	return dev, nil
}

// ListDevices 列出所有设备.
func (m *Manager) ListDevices() []*SmartNICDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()
	devices := make([]*SmartNICDevice, 0, len(m.devices))
	for _, dev := range m.devices {
		devices = append(devices, dev)
	}
	return devices
}

// EnableOffload 启用卸载功能.
func (m *Manager) EnableOffload(deviceID string, offloadType OffloadType, config map[string]string) (*OffloadFunction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	dev, exists := m.devices[deviceID]
	if !exists {
		return nil, ErrDeviceNotFound
	}
	if dev.State != StateReady && dev.State != StateRunning {
		return nil, ErrDeviceNotReady
	}
	// Check if device supports this offload
	supported := false
	for _, o := range dev.Offloads {
		if o == offloadType {
			supported = true
			break
		}
	}
	if !supported {
		return nil, ErrOffloadNotSupported
	}

	offload := &OffloadFunction{
		ID:        "offload-" + deviceID + "-" + string(offloadType),
		DeviceID:  deviceID,
		Type:      offloadType,
		State:     OffloadStateEnabled,
		Config:    config,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.offloads[offload.ID] = offload
	dev.State = StateRunning
	dev.UpdatedAt = time.Now()
	return offload, nil
}

// DisableOffload 禁用卸载功能.
func (m *Manager) DisableOffload(offloadID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	offload, exists := m.offloads[offloadID]
	if !exists {
		return ErrOffloadNotFound
	}
	offload.State = OffloadStateDisabled
	offload.UpdatedAt = time.Now()
	return nil
}

// ListOffloads 列出设备的卸载功能.
func (m *Manager) ListOffloads(deviceID string) []*OffloadFunction {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*OffloadFunction
	for _, o := range m.offloads {
		if o.DeviceID == deviceID {
			result = append(result, o)
		}
	}
	return result
}

// GetDeviceStats 获取设备统计.
func (m *Manager) GetDeviceStats(deviceID string) (*DeviceStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	dev, exists := m.devices[deviceID]
	if !exists {
		return nil, ErrDeviceNotFound
	}
	return &dev.Stats, nil
}

// Close 关闭管理器.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	close(m.stopCh)
	return nil
}
