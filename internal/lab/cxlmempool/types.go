// Package cxlmempool 实现 CXL (Compute Express Link) 内存池化管理
// 支持 CXL 1.1/2.0/3.0 内存设备发现、NUMA 感知分配、内存分层、热迁移和性能监控
package cxlmempool

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrDeviceNotFound      = errors.New("CXL device not found")
	ErrDeviceAlreadyExists = errors.New("CXL device already exists")
	ErrInsufficientMemory  = errors.New("insufficient CXL memory")
	ErrInvalidConfig       = errors.New("invalid configuration")
	ErrManagerClosed       = errors.New("manager closed")
	ErrUnsupportedVersion  = errors.New("unsupported CXL version")
	ErrNUMANodeNotFound    = errors.New("NUMA node not found")
	ErrMigrationFailed     = errors.New("memory migration failed")
)

// CXLVersion CXL 协议版本.
type CXLVersion string

const (
	CXL11 CXLVersion = "1.1"
	CXL20 CXLVersion = "2.0"
	CXL30 CXLVersion = "3.0"
	CXL31 CXLVersion = "3.1"
)

// DeviceType CXL 设备类型.
type DeviceType string

const (
	DeviceTypeMemory DeviceType = "memory" // CXL Memory Device
	DeviceTypeSwitch DeviceType = "switch" // CXL Switch
	DeviceTypeBridge DeviceType = "bridge" // CXL Bridge
	DeviceTypeType1  DeviceType = "type1"  // Type 1 Accelerator
	DeviceTypeType2  DeviceType = "type2"  // Type 2 Accelerator
	DeviceTypeType3  DeviceType = "type3"  // Type 3 Memory Device
)

// MemoryTier 内存层级.
type MemoryTier string

const (
	TierHot     MemoryTier = "hot"     // 热数据层（本地DRAM）
	TierWarm    MemoryTier = "warm"    // 温数据层（CXL内存）
	TierCold    MemoryTier = "cold"    // 冷数据层（CXL扩展内存）
	TierArchive MemoryTier = "archive" // 归档层
)

// DeviceState 设备状态.
type DeviceState string

const (
	StateOnline      DeviceState = "online"
	StateOffline     DeviceState = "offline"
	StateDegraded    DeviceState = "degraded"
	StateMaintenance DeviceState = "maintenance"
	StateError       DeviceState = "error"
)

// CXLDevice CXL 设备信息.
type CXLDevice struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Version      CXLVersion  `json:"version"`
	Type         DeviceType  `json:"type"`
	State        DeviceState `json:"state"`
	PCIeAddr     string      `json:"pcie_addr"`
	NUMANode     int         `json:"numa_node"`
	TotalMemory  uint64      `json:"total_memory"` // bytes
	UsedMemory   uint64      `json:"used_memory"`
	AvailableMem uint64      `json:"available_mem"`
	Bandwidth    float64     `json:"bandwidth"`   // GB/s
	Latency      float64     `json:"latency"`     // ns
	Temperature  float64     `json:"temperature"` // Celsius
	PowerUsage   float64     `json:"power_usage"` // Watts
	VendorID     string      `json:"vendor_id"`
	DeviceID     string      `json:"device_id"`
	SerialNum    string      `json:"serial_num"`
	FirmwareVer  string      `json:"firmware_ver"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// MemoryPool 内存池.
type MemoryPool struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Tier        MemoryTier       `json:"tier"`
	Devices     []string         `json:"devices"` // device IDs
	TotalMemory uint64           `json:"total_memory"`
	UsedMemory  uint64           `json:"used_memory"`
	Allocations []Allocation     `json:"allocations"`
	Policy      AllocationPolicy `json:"policy"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// Allocation 内存分配记录.
type Allocation struct {
	ID        string     `json:"id"`
	PoolID    string     `json:"pool_id"`
	ProcessID int        `json:"process_id"`
	Size      uint64     `json:"size"` // bytes
	Aligned   uint64     `json:"aligned"`
	Tier      MemoryTier `json:"tier"`
	Priority  int        `json:"priority"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// AllocationPolicy 分配策略.
type AllocationPolicy string

const (
	PolicyFirstFit     AllocationPolicy = "first_fit"
	PolicyBestFit      AllocationPolicy = "best_fit"
	PolicyRoundRobin   AllocationPolicy = "round_robin"
	PolicyNUMAAware    AllocationPolicy = "numa_aware"
	PolicyLatencyOpt   AllocationPolicy = "latency_optimized"
	PolicyBandwidthOpt AllocationPolicy = "bandwidth_optimized"
)

// Manager CXL 内存池管理器.
type Manager struct {
	mu          sync.RWMutex
	devices     map[string]*CXLDevice
	pools       map[string]*MemoryPool
	allocations map[string]*Allocation
	policy      AllocationPolicy
	closed      bool
	stopCh      chan struct{}
}

// NewManager 创建管理器.
func NewManager(policy AllocationPolicy) *Manager {
	return &Manager{
		devices:     make(map[string]*CXLDevice),
		pools:       make(map[string]*MemoryPool),
		allocations: make(map[string]*Allocation),
		policy:      policy,
		stopCh:      make(chan struct{}),
	}
}

// RegisterDevice 注册 CXL 设备.
func (m *Manager) RegisterDevice(dev *CXLDevice) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrManagerClosed
	}
	if _, exists := m.devices[dev.ID]; exists {
		return ErrDeviceAlreadyExists
	}
	dev.State = StateOnline
	dev.CreatedAt = time.Now()
	dev.UpdatedAt = time.Now()
	m.devices[dev.ID] = dev
	return nil
}

// RemoveDevice 移除设备.
func (m *Manager) RemoveDevice(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.devices[id]; !exists {
		return ErrDeviceNotFound
	}
	delete(m.devices, id)
	return nil
}

// GetDevice 获取设备信息.
func (m *Manager) GetDevice(id string) (*CXLDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	dev, exists := m.devices[id]
	if !exists {
		return nil, ErrDeviceNotFound
	}
	return dev, nil
}

// ListDevices 列出所有设备.
func (m *Manager) ListDevices() []*CXLDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()
	devices := make([]*CXLDevice, 0, len(m.devices))
	for _, dev := range m.devices {
		devices = append(devices, dev)
	}
	return devices
}

// CreatePool 创建内存池.
func (m *Manager) CreatePool(name string, tier MemoryTier, deviceIDs []string) (*MemoryPool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, ErrManagerClosed
	}
	var totalMem uint64
	for _, id := range deviceIDs {
		dev, exists := m.devices[id]
		if !exists {
			return nil, ErrDeviceNotFound
		}
		totalMem += dev.AvailableMem
	}
	pool := &MemoryPool{
		ID:          "pool-" + name,
		Name:        name,
		Tier:        tier,
		Devices:     deviceIDs,
		TotalMemory: totalMem,
		Policy:      m.policy,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.pools[pool.ID] = pool
	return pool, nil
}

// Allocate 分配内存.
func (m *Manager) Allocate(poolID string, size uint64) (*Allocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pool, exists := m.pools[poolID]
	if !exists {
		return nil, ErrDeviceNotFound
	}
	if pool.UsedMemory+size > pool.TotalMemory {
		return nil, ErrInsufficientMemory
	}
	alloc := &Allocation{
		ID:        "alloc-" + poolID,
		PoolID:    poolID,
		Size:      size,
		Tier:      pool.Tier,
		CreatedAt: time.Now(),
	}
	pool.UsedMemory += size
	pool.Allocations = append(pool.Allocations, *alloc)
	m.allocations[alloc.ID] = alloc
	return alloc, nil
}

// Free 释放内存.
func (m *Manager) Free(allocID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	alloc, exists := m.allocations[allocID]
	if !exists {
		return ErrDeviceNotFound
	}
	pool := m.pools[alloc.PoolID]
	pool.UsedMemory -= alloc.Size
	delete(m.allocations, allocID)
	return nil
}

// GetPool 获取内存池.
func (m *Manager) GetPool(id string) (*MemoryPool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pool, exists := m.pools[id]
	if !exists {
		return nil, ErrDeviceNotFound
	}
	return pool, nil
}

// Close 关闭管理器.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	close(m.stopCh)
	return nil
}
