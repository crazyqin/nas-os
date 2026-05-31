package rdmastorageaccel

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// 配置文件路径
	configFilePath = "/etc/nas-os/rdmastorageaccel.json"
	// 默认设备发现命令
	ibvDevInfoCmd = "ibv_devinfo"
	// 默认性能监控间隔
	monitorInterval = 10 * time.Second
	// 最大历史记录数
	maxHistorySize = 1000
)

// Manager RDMA 存储加速管理器
type Manager struct {
	mu              sync.RWMutex
	config          *RDMAConfig
	devices         map[string]*RDMADevice
	targets         map[string]*StorageTarget
	connections     map[string]*ConnectionInfo
	metrics         map[string]*PerfMetrics
	profiles        []TuningProfile
	benchmarks      map[string]*BenchmarkResult
	history         []PerfMetrics
	healthChecks    map[string]*HealthCheckResult
	monitorTicker   *time.Ticker
	stopMonitor     chan struct{}
	configPath      string
}

// NewManager 创建新的管理器实例
func NewManager() *Manager {
	m := &Manager{
		devices:      make(map[string]*RDMADevice),
		targets:      make(map[string]*StorageTarget),
		connections:  make(map[string]*ConnectionInfo),
		metrics:      make(map[string]*PerfMetrics),
		profiles:     DefaultTuningProfiles(),
		benchmarks:   make(map[string]*BenchmarkResult),
		healthChecks: make(map[string]*HealthCheckResult),
		stopMonitor:  make(chan struct{}),
		configPath:   configFilePath,
	}

	// 加载持久化配置
	if err := m.loadConfig(); err != nil {
		log.Printf("加载配置失败，使用默认配置: %v", err)
	}
	// 确保配置不为空
	if m.config == nil {
		m.config = DefaultRDMAConfig()
	}

	// 同步发现设备
	m.discoverDevices()

	// 启动性能监控
	go m.startMonitoring()

	return m
}

// loadConfig 从文件加载配置
func (m *Manager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config RDMAConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	m.config = &config
	return nil
}

// saveConfig 保存配置到文件
func (m *Manager) saveConfig() error {
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// discoverDevices 发现 RDMA 设备
func (m *Manager) discoverDevices() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 实际实现中会调用 ibv_devinfo 或读取 /sys/class/infiniband
	// 这里创建模拟设备
	mockDevices := []RDMADevice{
		{
			ID:          uuid.New().String(),
			Name:        "mlx5_0",
			PCIAddress:  "0000:03:00.0",
			PortCount:   2,
			Speed:       "100Gb/s",
			Status:      DeviceStatusActive,
			Driver:      "mlx5_core",
			FirmwareVer: "16.35.2000",
			NodeGUID:    "0x0002c90300a1b2c3",
			Ports: []RDMAPort{
				{
					PortNum:   1,
					LID:       1,
					GID:       "fe80:0000:0000:0000:0002:c903:00a1:b2c3",
					State:     "Active",
					PhysState: "LinkUp",
					Speed:     "100Gb/s",
					Width:     4,
				},
				{
					PortNum:   2,
					LID:       2,
					GID:       "fe80:0000:0000:0000:0002:c903:00a1:b2c4",
					State:     "Active",
					PhysState: "LinkUp",
					Speed:     "100Gb/s",
					Width:     4,
				},
			},
			Capabilities: []string{"RDMA_WRITE", "RDMA_READ", "ATOMIC_CMP_AND_SWAP", "ATOMIC_FETCH_AND_ADD"},
			UpdatedAt:    time.Now(),
		},
		{
			ID:          uuid.New().String(),
			Name:        "mlx5_1",
			PCIAddress:  "0000:04:00.0",
			PortCount:   1,
			Speed:       "200Gb/s",
			Status:      DeviceStatusActive,
			Driver:      "mlx5_core",
			FirmwareVer: "16.35.2000",
			NodeGUID:    "0x0002c90300a1b2c4",
			Ports: []RDMAPort{
				{
					PortNum:   1,
					LID:       3,
					GID:       "fe80:0000:0000:0000:0002:c903:00a1:b2c5",
					State:     "Active",
					PhysState: "LinkUp",
					Speed:     "200Gb/s",
					Width:     4,
				},
			},
			Capabilities: []string{"RDMA_WRITE", "RDMA_READ", "ATOMIC_CMP_AND_SWAP"},
			UpdatedAt:    time.Now(),
		},
	}

	// 清空旧设备
	m.devices = make(map[string]*RDMADevice)
	for _, device := range mockDevices {
		d := device
		m.devices[d.ID] = &d
	}

	log.Printf("发现 %d 个 RDMA 设备", len(m.devices))
}

// startMonitoring 启动性能监控
func (m *Manager) startMonitoring() {
	m.monitorTicker = time.NewTicker(monitorInterval)
	defer m.monitorTicker.Stop()

	for {
		select {
		case <-m.monitorTicker.C:
			m.collectMetrics()
		case <-m.stopMonitor:
			return
		}
	}
}

// collectMetrics 收集性能指标
func (m *Manager) collectMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for deviceID := range m.devices {
		// 实际实现中会读取 /sys/class/infiniband/<device>/ports/<port>/counters/
		metric := &PerfMetrics{
			ID:               uuid.New().String(),
			DeviceID:         deviceID,
			BandwidthMBs:     12500.0 + float64(time.Now().UnixNano()%1000),
			ReadBandwidthMBs: 7500.0 + float64(time.Now().UnixNano()%500),
			WriteBandwidthMBs: 5000.0 + float64(time.Now().UnixNano()%500),
			LatencyUs:        2.5 + float64(time.Now().UnixNano()%100)/100.0,
			ReadLatencyUs:    2.0 + float64(time.Now().UnixNano()%80)/100.0,
			WriteLatencyUs:   3.0 + float64(time.Now().UnixNano()%120)/100.0,
			IOPS:             500000 + time.Now().UnixNano()%100000,
			ReadIOPS:         300000 + time.Now().UnixNano()%60000,
			WriteIOPS:        200000 + time.Now().UnixNano()%40000,
			QueueDepth:       32 + int(time.Now().UnixNano()%32),
			MaxQueueDepth:    256,
			CPUUsage:         15.0 + float64(time.Now().UnixNano()%200)/10.0,
			MemoryUsageMB:    256 + time.Now().UnixNano()%128,
			CongestionEvents: time.Now().UnixNano() % 10,
			Retransmissions:  time.Now().UnixNano() % 5,
			Timestamp:        time.Now(),
		}

		m.metrics[deviceID] = metric

		// 保存历史记录
		m.history = append(m.history, *metric)
		if len(m.history) > maxHistorySize {
			m.history = m.history[len(m.history)-maxHistorySize:]
		}
	}
}

// GetDevices 获取所有 RDMA 设备
func (m *Manager) GetDevices() []*RDMADevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*RDMADevice, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, d)
	}
	return devices
}

// GetDevice 获取指定设备
func (m *Manager) GetDevice(id string) (*RDMADevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[id]
	if !ok {
		return nil, fmt.Errorf("设备未找到: %s", id)
	}
	return device, nil
}

// GetConfig 获取 RDMA 配置
func (m *Manager) GetConfig() *RDMAConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新 RDMA 配置
func (m *Manager) UpdateConfig(config *RDMAConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证配置
	if !IsValidProtocol(config.Protocol) {
		return fmt.Errorf("无效的协议类型: %s", config.Protocol)
	}

	if config.MTU < 1500 || config.MTU > 9000 {
		return fmt.Errorf("MTU 必须在 1500-9000 之间")
	}

	if !IsValidCongestionAlgorithm(config.CongestionControl) {
		return fmt.Errorf("无效的拥塞控制算法: %s", config.CongestionControl)
	}

	config.UpdatedAt = time.Now()
	m.config = config

	return m.saveConfig()
}

// CreateTarget 创建存储目标
func (m *Manager) CreateTarget(req *StorageTarget) (*StorageTarget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证目标类型
	if !IsValidTargetType(req.Type) {
		return nil, fmt.Errorf("无效的目标类型: %s", req.Type)
	}

	// 验证设备存在
	if _, ok := m.devices[req.DeviceID]; !ok {
		return nil, fmt.Errorf("设备未找到: %s", req.DeviceID)
	}

	// 创建目标
	target := &StorageTarget{
		ID:           uuid.New().String(),
		Name:         req.Name,
		Type:         req.Type,
		Status:       TargetStatusActive,
		DeviceID:     req.DeviceID,
		TargetAddr:   req.TargetAddr,
		Port:         req.Port,
		LUNMappings:  req.LUNMappings,
		NFSSettings:  req.NFSSettings,
		ISCSISettings: req.ISCSISettings,
		Tags:         req.Tags,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// 设置默认 RDMA 配置
	if req.RDMAConfig != nil {
		target.RDMAConfig = req.RDMAConfig
	} else {
		target.RDMAConfig = m.config
	}

	m.targets[target.ID] = target

	// 创建连接信息
	conn := &ConnectionInfo{
		ID:             uuid.New().String(),
		SourceDeviceID: req.DeviceID,
		SourceDevice:   m.devices[req.DeviceID].Name,
		TargetDeviceID: target.ID,
		TargetDevice:   target.Name,
		Status:         ConnectionStatusConnected,
		Protocol:       m.config.Protocol,
		LocalAddr:      "0.0.0.0",
		RemoteAddr:     target.TargetAddr,
		LocalPort:      0,
		RemotePort:     target.Port,
		QueuePairNum:   1,
		EstablishedAt:  time.Now(),
		LastActivity:   time.Now(),
	}
	m.connections[conn.ID] = conn

	log.Printf("创建存储目标: %s (类型: %s)", target.Name, target.Type)
	return target, nil
}

// GetTargets 获取所有存储目标
func (m *Manager) GetTargets() []*StorageTarget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	targets := make([]*StorageTarget, 0, len(m.targets))
	for _, t := range m.targets {
		targets = append(targets, t)
	}
	return targets
}

// DeleteTarget 删除存储目标
func (m *Manager) DeleteTarget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	target, ok := m.targets[id]
	if !ok {
		return fmt.Errorf("目标未找到: %s", id)
	}

	// 删除关联的连接
	for connID, conn := range m.connections {
		if conn.TargetDeviceID == id {
			delete(m.connections, connID)
		}
	}

	delete(m.targets, id)
	log.Printf("删除存储目标: %s", target.Name)
	return nil
}

// GetMetrics 获取性能指标
func (m *Manager) GetMetrics(deviceID string) *PerfMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if deviceID != "" {
		if metric, ok := m.metrics[deviceID]; ok {
			return metric
		}
		return nil
	}

	// 返回第一个设备的指标
	for _, metric := range m.metrics {
		return metric
	}
	return nil
}

// GetMetricsHistory 获取性能指标历史
func (m *Manager) GetMetricsHistory(limit int) []PerfMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	start := len(m.history) - limit
	if start < 0 {
		start = 0
	}

	return m.history[start:]
}

// RunBenchmark 运行基准测试
func (m *Manager) RunBenchmark(config *BenchmarkConfig) (*BenchmarkResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证设备
	if _, ok := m.devices[config.DeviceID]; !ok {
		return nil, fmt.Errorf("设备未找到: %s", config.DeviceID)
	}

	// 设置默认值
	if config.Duration <= 0 {
		config.Duration = 10
	}
	if config.BlockSize <= 0 {
		config.BlockSize = 4096
	}
	if config.QueueDepth <= 0 {
		config.QueueDepth = 32
	}
	if config.NumThreads <= 0 {
		config.NumThreads = 4
	}

	// 模拟基准测试执行
	// 实际实现中会调用 ib_write_bw, ib_read_bw, ib_write_lat 等工具
	time.Sleep(time.Duration(config.Duration) * time.Second / 10) // 模拟测试时间

	result := &BenchmarkResult{
		ID:     uuid.New().String(),
		Config: *config,
		BandwidthMBs:     12500.0 * float64(config.QueueDepth) / 32.0,
		ReadBandwidthMBs:  7500.0 * float64(config.QueueDepth) / 32.0,
		WriteBandwidthMBs: 5000.0 * float64(config.QueueDepth) / 32.0,
		LatencyUs:        2.5 + float64(config.BlockSize)/1024.0,
		ReadLatencyUs:    2.0 + float64(config.BlockSize)/1024.0,
		WriteLatencyUs:   3.0 + float64(config.BlockSize)/1024.0,
		IOPS:             int64(1000000.0 / (2.5 + float64(config.BlockSize)/1024.0)),
		ReadIOPS:         int64(600000.0 / (2.0 + float64(config.BlockSize)/1024.0)),
		WriteIOPS:        int64(400000.0 / (3.0 + float64(config.BlockSize)/1024.0)),
		CPUUsage:         45.0 + float64(config.NumThreads)*5.0,
		CompletedAt:      time.Now(),
		Duration:         time.Duration(config.Duration) * time.Second,
	}

	m.benchmarks[result.ID] = result
	log.Printf("基准测试完成: %s, 带宽: %.2f MB/s, 延迟: %.2f μs", result.ID, result.BandwidthMBs, result.LatencyUs)

	return result, nil
}

// GetTuningProfiles 获取调优预设
func (m *Manager) GetTuningProfiles() []TuningProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.profiles
}

// ApplyTuningProfile 应用调优预设
func (m *Manager) ApplyTuningProfile(profileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var profile *TuningProfile
	for i, p := range m.profiles {
		if p.ID == profileID {
			profile = &m.profiles[i]
			break
		}
	}

	if profile == nil {
		return fmt.Errorf("调优预设未找到: %s", profileID)
	}

	// 应用预设到配置
	m.config.MTU = profile.MTU
	m.config.CongestionControl = profile.Congestion
	m.config.QoS = profile.QoS
	m.config.Advanced.MaxSendWR = profile.MaxSendWR
	m.config.Advanced.MaxRecvWR = profile.MaxRecvWR
	m.config.UpdatedAt = time.Now()

	log.Printf("应用调优预设: %s", profile.Name)
	return m.saveConfig()
}

// HealthCheck 执行健康检查
func (m *Manager) HealthCheck(deviceID, targetID string) *HealthCheckResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := &HealthCheckResult{
		DeviceID: deviceID,
		TargetID: targetID,
		Healthy:  true,
		Status:   "healthy",
		Details:  make(map[string]string),
	}

	// 检查设备状态
	if device, ok := m.devices[deviceID]; ok {
		if device.Status != DeviceStatusActive {
			result.Healthy = false
			result.Status = "device_inactive"
			result.Details["device_status"] = string(device.Status)
		}
	} else {
		result.Healthy = false
		result.Status = "device_not_found"
	}

	// 检查目标状态
	if targetID != "" {
		if target, ok := m.targets[targetID]; ok {
			if target.Status != TargetStatusActive {
				result.Healthy = false
				result.Status = "target_inactive"
				result.Details["target_status"] = string(target.Status)
			}
		} else {
			result.Healthy = false
			result.Status = "target_not_found"
		}
	}

	// 模拟延迟和丢包检测
	result.LatencyMs = 0.5 + float64(time.Now().UnixNano()%100)/100.0
	result.PacketLoss = float64(time.Now().UnixNano()%10) / 100.0
	result.LastChecked = time.Now()

	m.healthChecks[deviceID+targetID] = result
	return result
}

// Stop 停止管理器
func (m *Manager) Stop() {
	close(m.stopMonitor)
	if m.monitorTicker != nil {
		m.monitorTicker.Stop()
	}
}