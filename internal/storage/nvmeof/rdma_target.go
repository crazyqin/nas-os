// Package nvmeof - NVMe-oF RDMA Target 系统实现
// 封装 Linux 内核 nvmet-rdma 模块配置
// 通过 configfs 配置 NVMe/RDMA Target

package nvmeof

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	pkgnvmeof "nas-os/pkg/storage/nvmeof"
)

// ========== RDMA 常量定义 ==========

const (
	// RDMAConfigPath RDMA configfs 路径
	RDMAConfigPath = "/sys/kernel/config/nvmet/ports"

	// RDMADefaultPort RDMA 默认服务端口
	RDMADefaultPort = 4420

	// RDMADefaultMTU RDMA 默认 MTU
	RDMADefaultMTU = 9000

	// RDMADefaultQueueDepth RDMA 默认队列深度
	RDMADefaultQueueDepth = 128
)

// ========== RDMA Target 系统管理器 ==========

// RDMATargetSysManager RDMA Target 系统管理器
type RDMATargetSysManager struct {
	mu sync.RWMutex

	// pkg 层 RDMA 管理器
	pkgRdmaManager *pkgnvmeof.RDMAManager

	// pkg 层 Target 管理器
	pkgTargetManager *pkgnvmeof.TargetManager

	// RDMA 配置
	rdmaConfig *pkgnvmeof.RDMAConfig

	// 端口 ID 分配
	portIDs    map[int]bool
	nextPortID int

	// 运行状态
	running bool
}

// NewRDMATargetSysManager 创建 RDMA Target 系统管理器
func NewRDMATargetSysManager(rdmaConfig *pkgnvmeof.RDMAConfig, targetManager *pkgnvmeof.TargetManager) (*RDMATargetSysManager, error) {
	if rdmaConfig == nil {
		rdmaConfig = pkgnvmeof.DefaultRDMAConfig()
	}

	pkgRdmaManager, err := pkgnvmeof.NewRDMAManager(rdmaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create pkg rdma manager: %w", err)
	}

	m := &RDMATargetSysManager{
		pkgRdmaManager:   pkgRdmaManager,
		pkgTargetManager: targetManager,
		rdmaConfig:       rdmaConfig,
		portIDs:          make(map[int]bool),
		nextPortID:       1,
	}

	// 检查 RDMA 模块是否可用
	if err := m.checkRDMAAvailable(); err != nil {
		return nil, fmt.Errorf("rdma not available: %w", err)
	}

	// 加载现有配置
	m.loadExistingConfig()

	return m, nil
}

// checkRDMAAvailable 检查 RDMA 模块是否可用
func (m *RDMATargetSysManager) checkRDMAAvailable() error {
	// 检查 configfs 是否挂载
	if _, err := os.Stat("/sys/kernel/config"); err != nil {
		return fmt.Errorf("configfs not mounted: %w", err)
	}

	// 检查 nvmet 目录是否存在
	if _, err := os.Stat(NVMetConfigPath); err != nil {
		// 尝试加载 nvmet 模块
		if err := m.loadModule("nvmet"); err != nil {
			return fmt.Errorf("nvmet module not loaded: %w", err)
		}
	}

	// 检查 RDMA 设备
	if !m.pkgRdmaManager.IsAvailable() {
		return fmt.Errorf("rdma devices not available")
	}

	// 检查是否有可用的 RDMA 设备
	devices := m.pkgRdmaManager.GetDevices()
	if len(devices) == 0 {
		return fmt.Errorf("no rdma devices found")
	}

	return nil
}

// loadModule 加载内核模块
func (m *RDMATargetSysManager) loadModule(module string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "modprobe", module)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to load module %s: %w, output: %s", module, err, output)
	}
	return nil
}

// loadExistingConfig 加载现有配置
func (m *RDMATargetSysManager) loadExistingConfig() {
	// 读取已存在的端口
	portsDir, err := os.ReadDir(RDMAConfigPath)
	if err == nil {
		for _, entry := range portsDir {
			if entry.IsDir() {
				portID, _ := strconv.Atoi(entry.Name())
				m.portIDs[portID] = true
				if portID >= m.nextPortID {
					m.nextPortID = portID + 1
				}

				// 检查端口传输类型
				trTypePath := filepath.Join(RDMAConfigPath, entry.Name(), "addr_trtype")
				if data, err := os.ReadFile(trTypePath); err == nil {
					trType := strings.TrimSpace(string(data))
					if trType == "rdma" {
						// 这是 RDMA 端口，读取详细信息
						m.loadRDMAPortConfig(portID)
					}
				}
			}
		}
	}
}

// loadRDMAPortConfig 加载 RDMA 端口配置
func (m *RDMATargetSysManager) loadRDMAPortConfig(portID int) {
	portPath := filepath.Join(RDMAConfigPath, strconv.Itoa(portID))

	// 读取地址
	addrPath := filepath.Join(portPath, "addr_traddr")
	if data, err := os.ReadFile(addrPath); err == nil {
		// 存储地址信息
		_ = strings.TrimSpace(string(data))
	}

	// 读取端口
	svcidPath := filepath.Join(portPath, "addr_trsvcid")
	if data, err := os.ReadFile(svcidPath); err == nil {
		_ = strings.TrimSpace(string(data))
	}
}

// ========== RDMA Target 端口管理 ==========

// CreateRDMAPort 创建 RDMA Target 端口
func (m *RDMATargetSysManager) CreateRDMAPort(ctx context.Context, req *CreateRDMAPortRequest) (*RDMAPort, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证请求
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// 分配端口 ID
	portID := m.allocatePortID()

	// 创建端口目录
	portPath := filepath.Join(RDMAConfigPath, strconv.Itoa(portID))
	if err := os.MkdirAll(portPath, 0o755); err != nil {
		m.releasePortID(portID)
		return nil, fmt.Errorf("failed to create port directory: %w", err)
	}

	// 设置传输类型为 RDMA
	trTypePath := filepath.Join(portPath, "addr_trtype")
	if err := os.WriteFile(trTypePath, []byte("rdma"), 0o644); err != nil {
		_ = os.RemoveAll(portPath)
		m.releasePortID(portID)
		return nil, fmt.Errorf("failed to set addr_trtype: %w", err)
	}

	// 设置地址
	addrPath := filepath.Join(portPath, "addr_traddr")
	if err := os.WriteFile(addrPath, []byte(req.Address), 0o644); err != nil {
		_ = os.RemoveAll(portPath)
		m.releasePortID(portID)
		return nil, fmt.Errorf("failed to set addr_traddr: %w", err)
	}

	// 设置服务端口
	svcidPath := filepath.Join(portPath, "addr_trsvcid")
	svcid := strconv.Itoa(req.ServicePort)
	if err := os.WriteFile(svcidPath, []byte(svcid), 0o644); err != nil {
		_ = os.RemoveAll(portPath)
		m.releasePortID(portID)
		return nil, fmt.Errorf("failed to set addr_trsvcid: %w", err)
	}

	// 创建端口对象
	port := &RDMAPort{
		ID:            portID,
		Address:       req.Address,
		ServicePort:   req.ServicePort,
		Device:        req.Device,
		GIDIndex:      req.GIDIndex,
		MTU:           req.MTU,
		TransportType: pkgnvmeof.RDMATransportRoCEv2,
		State:         RDMAPortStateUp,
		CreatedAt:     time.Now(),
	}

	return port, nil
}

// DeleteRDMAPort 删除 RDMA Target 端口
func (m *RDMATargetSysManager) DeleteRDMAPort(ctx context.Context, portID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	portPath := filepath.Join(RDMAConfigPath, strconv.Itoa(portID))

	// 检查端口是否存在
	if _, err := os.Stat(portPath); os.IsNotExist(err) {
		return nil
	}

	// 先删除所有子系统链接
	subsysDir := filepath.Join(portPath, "subsystems")
	if entries, err := os.ReadDir(subsysDir); err == nil {
		for _, entry := range entries {
			linkPath := filepath.Join(subsysDir, entry.Name())
			_ = os.Remove(linkPath)
		}
	}

	// 删除端口目录
	if err := os.RemoveAll(portPath); err != nil {
		return fmt.Errorf("failed to remove port directory: %w", err)
	}

	// 释放端口 ID
	m.releasePortID(portID)

	return nil
}

// LinkSubsystemToRDMAPort 将子系统链接到 RDMA 端口
func (m *RDMATargetSysManager) LinkSubsystemToRDMAPort(ctx context.Context, portID int, subsysNQN string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	portPath := filepath.Join(RDMAConfigPath, strconv.Itoa(portID))
	linkPath := filepath.Join(portPath, "subsystems", subsysNQN)

	// 检查端口是否存在
	if _, err := os.Stat(portPath); os.IsNotExist(err) {
		return fmt.Errorf("port %d does not exist", portID)
	}

	// 创建链接
	if err := os.MkdirAll(linkPath, 0o755); err != nil {
		return fmt.Errorf("failed to create subsystem link: %w", err)
	}

	return nil
}

// UnlinkSubsystemFromRDMAPort 将子系统从 RDMA 端口解链
func (m *RDMATargetSysManager) UnlinkSubsystemFromRDMAPort(ctx context.Context, portID int, subsysNQN string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	portPath := filepath.Join(RDMAConfigPath, strconv.Itoa(portID))
	linkPath := filepath.Join(portPath, "subsystems", subsysNQN)

	// 删除链接
	if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove subsystem link: %w", err)
	}

	return nil
}

// allocatePortID 分配端口 ID
func (m *RDMATargetSysManager) allocatePortID() int {
	for {
		if !m.portIDs[m.nextPortID] {
			m.portIDs[m.nextPortID] = true
			id := m.nextPortID
			m.nextPortID++
			return id
		}
		m.nextPortID++
	}
}

// releasePortID 释放端口 ID
func (m *RDMATargetSysManager) releasePortID(portID int) {
	delete(m.portIDs, portID)
}

// ListRDMAPorts 列出 RDMA 端口
func (m *RDMATargetSysManager) ListRDMAPorts() ([]*RDMAPort, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ports := make([]*RDMAPort, 0)

	portsDir, err := os.ReadDir(RDMAConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ports, nil
		}
		return nil, fmt.Errorf("failed to read ports directory: %w", err)
	}

	for _, entry := range portsDir {
		if !entry.IsDir() {
			continue
		}

		portID, _ := strconv.Atoi(entry.Name())
		portPath := filepath.Join(RDMAConfigPath, entry.Name())

		// 检查传输类型
		trTypePath := filepath.Join(portPath, "addr_trtype")
		if data, err := os.ReadFile(trTypePath); err == nil {
			trType := strings.TrimSpace(string(data))
			if trType != "rdma" {
				continue
			}
		} else {
			continue
		}

		// 读取地址
		address := ""
		if data, err := os.ReadFile(filepath.Join(portPath, "addr_traddr")); err == nil {
			address = strings.TrimSpace(string(data))
		}

		// 读取服务端口
		svcid := RDMADefaultPort
		if data, err := os.ReadFile(filepath.Join(portPath, "addr_trsvcid")); err == nil {
			if p, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
				svcid = p
			}
		}

		ports = append(ports, &RDMAPort{
			ID:            portID,
			Address:       address,
			ServicePort:   svcid,
			TransportType: pkgnvmeof.RDMATransportRoCEv2,
			State:         RDMAPortStateUp,
		})
	}

	return ports, nil
}

// GetRDMAPort 获取 RDMA 端口详情
func (m *RDMATargetSysManager) GetRDMAPort(portID int) (*RDMAPort, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	portPath := filepath.Join(RDMAConfigPath, strconv.Itoa(portID))

	// 检查端口是否存在
	if _, err := os.Stat(portPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("port %d does not exist", portID)
	}

	// 检查传输类型
	trTypePath := filepath.Join(portPath, "addr_trtype")
	if data, err := os.ReadFile(trTypePath); err == nil {
		trType := strings.TrimSpace(string(data))
		if trType != "rdma" {
			return nil, fmt.Errorf("port %d is not an RDMA port", portID)
		}
	}

	// 读取地址
	address := ""
	if data, err := os.ReadFile(filepath.Join(portPath, "addr_traddr")); err == nil {
		address = strings.TrimSpace(string(data))
	}

	// 读取服务端口
	svcid := RDMADefaultPort
	if data, err := os.ReadFile(filepath.Join(portPath, "addr_trsvcid")); err == nil {
		if p, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			svcid = p
		}
	}

	return &RDMAPort{
		ID:            portID,
		Address:       address,
		ServicePort:   svcid,
		TransportType: pkgnvmeof.RDMATransportRoCEv2,
		State:         RDMAPortStateUp,
	}, nil
}

// ========== RDMA 服务管理 ==========

// Start 启动 RDMA Target 服务
func (m *RDMATargetSysManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	// 加载必要内核模块
	if err := m.loadModule("nvmet"); err != nil {
		return fmt.Errorf("failed to load nvmet module: %w", err)
	}

	if err := m.loadModule("nvmet-rdma"); err != nil {
		return fmt.Errorf("failed to load nvmet-rdma module: %w", err)
	}

	// 加载 RDMA 相关模块
	rdmaModules := []string{"ib_core", "ib_uverbs", "rdma_cm"}
	for _, module := range rdmaModules {
		if err := m.loadModule(module); err != nil {
			// 部分模块可能已内置，忽略错误
			_ = err
		}
	}

	m.running = true

	return m.pkgRdmaManager.Start(ctx)
}

// Stop 停止 RDMA Target 服务
func (m *RDMATargetSysManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	m.running = false
	return m.pkgRdmaManager.Stop()
}

// IsRunning 检查是否运行中
func (m *RDMATargetSysManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// ========== RDMA 设备管理 ==========

// GetRDMADevices 获取 RDMA 设备列表
func (m *RDMATargetSysManager) GetRDMADevices() []*pkgnvmeof.RDMADevice {
	return m.pkgRdmaManager.GetDevices()
}

// GetRDMADevice 获取指定 RDMA 设备
func (m *RDMATargetSysManager) GetRDMADevice(name string) (*pkgnvmeof.RDMADevice, error) {
	return m.pkgRdmaManager.GetDevice(name)
}

// ========== RDMA 统计 ==========

// GetRDMAStats 获取 RDMA 统计信息
func (m *RDMATargetSysManager) GetRDMAStats() *RDMATargetStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &RDMATargetStats{
		Available: m.pkgRdmaManager.IsAvailable(),
		Running:   m.running,
		PortCount: len(m.portIDs),
	}

	// 获取设备统计
	devices := m.pkgRdmaManager.GetDevices()
	stats.DeviceCount = len(devices)

	for _, device := range devices {
		stats.TotalTxBytes += device.Stats.TxBytes
		stats.TotalRxBytes += device.Stats.RxBytes
		stats.TotalTxPackets += device.Stats.TxPackets
		stats.TotalRxPackets += device.Stats.RxPackets
		stats.TotalTxErrors += device.Stats.TxErrors
		stats.TotalRxErrors += device.Stats.RxErrors
	}

	return stats
}

// ========== RDMA 端口对象 ==========

// RDMAPort RDMA 端口
type RDMAPort struct {
	// 端口 ID
	ID int `json:"id"`

	// IP 地址
	Address string `json:"address"`

	// 服务端口
	ServicePort int `json:"servicePort"`

	// RDMA 设备名称
	Device string `json:"device"`

	// GID 索引
	GIDIndex int `json:"gidIndex"`

	// MTU
	MTU int `json:"mtu"`

	// 传输类型
	TransportType pkgnvmeof.RDMATransportType `json:"transportType"`

	// 状态
	State RDMAPortState `json:"state"`

	// 链接的子系统
	LinkedSubsystems []string `json:"linkedSubsystems"`

	// 创建时间
	CreatedAt time.Time `json:"createdAt"`
}

// RDMAPortState RDMA 端口状态
type RDMAPortState string

const (
	// RDMAPortStateUp 端口在线
	RDMAPortStateUp RDMAPortState = "up"
	// RDMAPortStateDown 端口离线
	RDMAPortStateDown RDMAPortState = "down"
	// RDMAPortStateError 端口错误
	RDMAPortStateError RDMAPortState = "error"
)

// RDMATargetStats RDMA Target 统计
type RDMATargetStats struct {
	// 可用性
	Available bool `json:"available"`

	// 运行状态
	Running bool `json:"running"`

	// 设备数量
	DeviceCount int `json:"deviceCount"`

	// 端口数量
	PortCount int `json:"portCount"`

	// 发送统计
	TotalTxBytes   uint64 `json:"totalTxBytes"`
	TotalTxPackets uint64 `json:"totalTxPackets"`
	TotalTxErrors  uint64 `json:"totalTxErrors"`

	// 接收统计
	TotalRxBytes   uint64 `json:"totalRxBytes"`
	TotalRxPackets uint64 `json:"totalRxPackets"`
	TotalRxErrors  uint64 `json:"totalRxErrors"`

	// 连接统计
	ActiveConnections int `json:"activeConnections"`
}

// ========== RDMA 端口请求 ==========

// CreateRDMAPortRequest 创建 RDMA 端口请求
type CreateRDMAPortRequest struct {
	// IP 地址
	Address string `json:"address"`

	// 服务端口
	ServicePort int `json:"servicePort"`

	// RDMA 设备名称
	Device string `json:"device"`

	// GID 索引
	GIDIndex int `json:"gidIndex"`

	// MTU
	MTU int `json:"mtu"`

	// 子系统 NQN (可选，创建后自动链接)
	SubsystemNQN string `json:"subsystemNqn,omitempty"`
}

// Validate 验证请求
func (r *CreateRDMAPortRequest) Validate() error {
	if r.Address == "" {
		return fmt.Errorf("address is required")
	}

	if r.ServicePort <= 0 || r.ServicePort > 65535 {
		r.ServicePort = RDMADefaultPort
	}

	if r.MTU <= 0 {
		r.MTU = RDMADefaultMTU
	}

	if r.GIDIndex < 0 {
		r.GIDIndex = 0
	}

	return nil
}
