// Package nvmetempool 提供 NVMe-oF 存储池管理功能
// NVMe over Fabrics 目标端管理、远程 NVMe 设备发现、存储池创建和管理、
// 性能监控（IOPS/延迟/带宽）、故障切换和冗余
package nvmetempool

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ========== 核心类型 ==========

// TransportType 传输类型
type TransportType string

const (
	TransportRDMA  TransportType = "rdma"  // RDMA
	TransportTCP   TransportType = "tcp"   // TCP
	TransportFC    TransportType = "fc"    // Fibre Channel
)

// DeviceStatus 设备状态
type DeviceStatus string

const (
	DeviceStatusOnline  DeviceStatus = "online"  // 在线
	DeviceStatusOffline DeviceStatus = "offline" // 离线
	DeviceStatusDegraded DeviceStatus = "degraded" // 降级
	DeviceStatusFault   DeviceStatus = "fault"   // 故障
)

// PoolStatus 存储池状态
type PoolStatus string

const (
	PoolStatusActive   PoolStatus = "active"   // 活跃
	PoolStatusDegraded PoolStatus = "degraded" // 降级
	PoolStatusFault    PoolStatus = "fault"    // 故障
	PoolStatusOffline  PoolStatus = "offline"  // 离线
)

// NvmeTarget NVMe-oF 目标端
type NvmeTarget struct {
	ID          string        `json:"id"`          // 目标ID
	Name        string        `json:"name"`        // 目标名称
	Address     string        `json:"address"`      // 地址
	Port        int           `json:"port"`        // 端口
	Transport   TransportType `json:"transport"`   // 传输类型
	Subsystem   string        `json:"subsystem"`   // 子系统NQN
	Status      DeviceStatus  `json:"status"`      // 状态
	ConnectedAt time.Time     `json:"connectedAt"` // 连接时间
	UpdatedAt   time.Time     `json:"updatedAt"`
}

// NvmeDevice NVMe 设备信息
type NvmeDevice struct {
	ID         string       `json:"id"`         // 设备ID
	Model      string       `json:"model"`      // 型号
	Serial     string       `json:"serial"`     // 序列号
	Namespace  string       `json:"namespace"`  // 命名空间
	Capacity   uint64       `json:"capacity"`   // 容量 (字节)
	UsedSpace  uint64       `json:"usedSpace"`  // 已用空间
	TargetID   string       `json:"targetId"`   // 所属目标ID
	Status     DeviceStatus `json:"status"`     // 状态
	UpdatedAt  time.Time    `json:"updatedAt"`
}

// NvmePool NVMe-oF 存储池
type NvmePool struct {
	ID          string       `json:"id"`          // 存储池ID
	Name        string       `json:"name"`        // 存储池名称
	Devices     []string     `json:"devices"`     // 设备ID列表
	TotalSpace  uint64       `json:"totalSpace"`  // 总空间
	UsedSpace   uint64       `json:"usedSpace"`   // 已用空间
	FreeSpace   uint64       `json:"freeSpace"`   // 可用空间
	Status      PoolStatus   `json:"status"`      // 状态
	Redundancy  string       `json:"redundancy"`  // 冗余策略
	CreatedAt   time.Time    `json:"createdAt"`   // 创建时间
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	IOPS       float64   `json:"iops"`       // IOPS
	ReadIOPS   float64   `json:"readIops"`   // 读IOPS
	WriteIOPS  float64   `json:"writeIops"`  // 写IOPS
	Latency    float64   `json:"latency"`    // 平均延迟 (μs)
	ReadLat    float64   `json:"readLat"`    // 读延迟
	WriteLat   float64   `json:"writeLat"`   // 写延迟
	Bandwidth  float64   `json:"bandwidth"`  // 带宽 (MB/s)
	ReadBW     float64   `json:"readBw"`     // 读带宽
	WriteBW    float64   `json:"writeBw"`    // 写带宽
	Timestamp  time.Time `json:"timestamp"`
}

// PoolPerformance 存储池性能
type PoolPerformance struct {
	PoolID    string             `json:"poolId"`
	PoolName  string             `json:"poolName"`
	Metrics   *PerformanceMetrics `json:"metrics"`
	Timestamp time.Time          `json:"timestamp"`
}

// FailoverEvent 故障切换事件
type FailoverEvent struct {
	ID          string    `json:"id"`          // 事件ID
	SourceID    string    `json:"sourceId"`    // 源设备ID
	TargetID    string    `json:"targetId"`    // 目标设备ID
	Reason      string    `json:"reason"`      // 切换原因
	Timestamp   time.Time `json:"timestamp"`   // 发生时间
	Recovered   bool      `json:"recovered"`   // 是否已恢复
}

// ========== Manager ==========

// Manager NVMe-oF 存储池管理器
type Manager struct {
	mu          sync.RWMutex
	targets     map[string]*NvmeTarget
	devices     map[string]*NvmeDevice
	pools       map[string]*NvmePool
	metrics     map[string]*PerformanceMetrics
	failovers   []FailoverEvent
	maxFailovers int
	stopCh      chan struct{}
	running     bool
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		targets:      make(map[string]*NvmeTarget),
		devices:      make(map[string]*NvmeDevice),
		pools:        make(map[string]*NvmePool),
		metrics:      make(map[string]*PerformanceMetrics),
		maxFailovers: 100,
		stopCh:       make(chan struct{}),
	}
}

// AddTarget 添加 NVMe-oF 目标端
func (m *Manager) AddTarget(target *NvmeTarget) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if target.ID == "" {
		return fmt.Errorf("target ID is required")
	}

	if _, exists := m.targets[target.ID]; exists {
		return fmt.Errorf("target already exists: %s", target.ID)
	}

	target.ConnectedAt = time.Now()
	target.UpdatedAt = time.Now()
	target.Status = DeviceStatusOnline

	m.targets[target.ID] = target
	log.Printf("[NVMe-oF] 添加目标端: %s (%s:%d)", target.Name, target.Address, target.Port)
	return nil
}

// RemoveTarget 移除目标端
func (m *Manager) RemoveTarget(targetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.targets[targetID]; !exists {
		return fmt.Errorf("target not found: %s", targetID)
	}

	// 检查是否有设备关联
	for _, device := range m.devices {
		if device.TargetID == targetID {
			return fmt.Errorf("cannot remove target with associated devices")
		}
	}

	delete(m.targets, targetID)
	log.Printf("[NVMe-oF] 移除目标端: %s", targetID)
	return nil
}

// GetTarget 获取目标端
func (m *Manager) GetTarget(targetID string) (*NvmeTarget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	target, ok := m.targets[targetID]
	if !ok {
		return nil, fmt.Errorf("target not found: %s", targetID)
	}
	return target, nil
}

// ListTargets 列出所有目标端
func (m *Manager) ListTargets() []*NvmeTarget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var targets []*NvmeTarget
	for _, t := range m.targets {
		targets = append(targets, t)
	}
	return targets
}

// AddDevice 添加设备
func (m *Manager) AddDevice(device *NvmeDevice) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if device.ID == "" {
		return fmt.Errorf("device ID is required")
	}

	if _, exists := m.devices[device.ID]; exists {
		return fmt.Errorf("device already exists: %s", device.ID)
	}

	// 验证目标端存在
	if _, exists := m.targets[device.TargetID]; !exists {
		return fmt.Errorf("target not found: %s", device.TargetID)
	}

	device.Status = DeviceStatusOnline
	device.UpdatedAt = time.Now()

	m.devices[device.ID] = device
	log.Printf("[NVMe-oF] 添加设备: %s (%s)", device.Model, device.ID)
	return nil
}

// RemoveDevice 移除设备
func (m *Manager) RemoveDevice(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.devices[deviceID]; !exists {
		return fmt.Errorf("device not found: %s", deviceID)
	}

	// 检查是否属于某个存储池
	for _, pool := range m.pools {
		for _, id := range pool.Devices {
			if id == deviceID {
				return fmt.Errorf("cannot remove device that belongs to pool: %s", pool.ID)
			}
		}
	}

	delete(m.devices, deviceID)
	log.Printf("[NVMe-oF] 移除设备: %s", deviceID)
	return nil
}

// GetDevice 获取设备
func (m *Manager) GetDevice(deviceID string) (*NvmeDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return nil, fmt.Errorf("device not found: %s", deviceID)
	}
	return device, nil
}

// ListDevices 列出所有设备
func (m *Manager) ListDevices() []*NvmeDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var devices []*NvmeDevice
	for _, d := range m.devices {
		devices = append(devices, d)
	}
	return devices
}

// CreatePool 创建存储池
func (m *Manager) CreatePool(pool *NvmePool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pool.ID == "" {
		return fmt.Errorf("pool ID is required")
	}

	if _, exists := m.pools[pool.ID]; exists {
		return fmt.Errorf("pool already exists: %s", pool.ID)
	}

	// 验证设备存在
	var totalSpace uint64
	for _, deviceID := range pool.Devices {
		device, exists := m.devices[deviceID]
		if !exists {
			return fmt.Errorf("device not found: %s", deviceID)
		}
		totalSpace += device.Capacity
	}

	pool.TotalSpace = totalSpace
	pool.FreeSpace = totalSpace
	pool.Status = PoolStatusActive
	pool.CreatedAt = time.Now()
	pool.UpdatedAt = time.Now()

	m.pools[pool.ID] = pool
	log.Printf("[NVMe-oF] 创建存储池: %s (容量: %d bytes)", pool.Name, totalSpace)
	return nil
}

// DeletePool 删除存储池
func (m *Manager) DeletePool(poolID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.pools[poolID]; !exists {
		return fmt.Errorf("pool not found: %s", poolID)
	}

	delete(m.pools, poolID)
	log.Printf("[NVMe-oF] 删除存储池: %s", poolID)
	return nil
}

// GetPool 获取存储池
func (m *Manager) GetPool(poolID string) (*NvmePool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, ok := m.pools[poolID]
	if !ok {
		return nil, fmt.Errorf("pool not found: %s", poolID)
	}
	return pool, nil
}

// ListPools 列出所有存储池
func (m *Manager) ListPools() []*NvmePool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var pools []*NvmePool
	for _, p := range m.pools {
		pools = append(pools, p)
	}
	return pools
}

// GetPoolPerformance 获取存储池性能
func (m *Manager) GetPoolPerformance(poolID string) (*PoolPerformance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, ok := m.pools[poolID]
	if !ok {
		return nil, fmt.Errorf("pool not found: %s", poolID)
	}

	metrics, ok := m.metrics[poolID]
	if !ok {
		// 返回空指标
		metrics = &PerformanceMetrics{
			Timestamp: time.Now(),
		}
	}

	return &PoolPerformance{
		PoolID:    poolID,
		PoolName:  pool.Name,
		Metrics:   metrics,
		Timestamp: time.Now(),
	}, nil
}

// GetFailoverEvents 获取故障切换事件
func (m *Manager) GetFailoverEvents() []FailoverEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := make([]FailoverEvent, len(m.failovers))
	copy(events, m.failovers)
	return events
}

// collect 采集一次性能数据
func (m *Manager) collect() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// 更新每个存储池的性能指标
	for poolID := range m.pools {
		if _, exists := m.metrics[poolID]; !exists {
			m.metrics[poolID] = &PerformanceMetrics{}
		}

		metrics := m.metrics[poolID]
		// 模拟性能数据波动
		baseIOPS := 10000.0
		metrics.IOPS = baseIOPS + float64(now.Second()%100)*100
		metrics.ReadIOPS = metrics.IOPS * 0.6
		metrics.WriteIOPS = metrics.IOPS * 0.4
		metrics.Latency = 50.0 + float64(now.Second()%20)
		metrics.ReadLat = metrics.Latency * 0.8
		metrics.WriteLat = metrics.Latency * 1.2
		metrics.Bandwidth = 500.0 + float64(now.Second()%50)
		metrics.ReadBW = metrics.Bandwidth * 0.6
		metrics.WriteBW = metrics.Bandwidth * 0.4
		metrics.Timestamp = now
	}

	// 检查设备故障
	m.checkDeviceHealth()
}

// checkDeviceHealth 检查设备健康状态
func (m *Manager) checkDeviceHealth() {
	for _, device := range m.devices {
		// 模拟故障检测
		if device.Status == DeviceStatusFault {
			// 触发故障切换
			m.triggerFailover(device.ID, "设备故障")
		}
	}
}

// triggerFailover 触发故障切换
func (m *Manager) triggerFailover(sourceID, reason string) {
	event := FailoverEvent{
		ID:        fmt.Sprintf("failover-%d", time.Now().UnixNano()),
		SourceID:  sourceID,
		Reason:    reason,
		Timestamp: time.Now(),
	}

	m.failovers = append(m.failovers, event)
	if len(m.failovers) > m.maxFailovers {
		m.failovers = m.failovers[len(m.failovers)-m.maxFailovers:]
	}

	log.Printf("[NVMe-oF] 故障切换事件: %s -> %s", sourceID, reason)
}

// Start 启动定时采集
func (m *Manager) Start(interval time.Duration) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopCh = make(chan struct{})
	m.mu.Unlock()

	go func() {
		// 立即采集一次
		m.collect()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.collect()
			case <-m.stopCh:
				return
			}
		}
	}()

	log.Printf("[NVMe-oF] 启动定时采集，间隔 %v", interval)
}

// Stop 停止定时采集
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false
	close(m.stopCh)
	log.Println("[NVMe-oF] 停止定时采集")
}

// DiscoverTargets 发现远程 NVMe 设备
func (m *Manager) DiscoverTargets(address string, transport TransportType) ([]*NvmeTarget, error) {
	// 模拟发现过程
	log.Printf("[NVMe-oF] 发现目标: %s (传输: %s)", address, transport)

	// 这里实际实现会调用 nvme discover 命令
	// 返回模拟数据
	targets := []*NvmeTarget{
		{
			ID:        fmt.Sprintf("target-%d", time.Now().UnixNano()),
			Name:      "Discovered Target",
			Address:   address,
			Port:      4420,
			Transport: transport,
			Subsystem: "nqn.2024-01.com.example:storage",
			Status:    DeviceStatusOnline,
		},
	}

	return targets, nil
}
