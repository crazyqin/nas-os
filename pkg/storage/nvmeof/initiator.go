// Package nvmeof - Initiator 端管理
// NVMe-oF Initiator 连接远程 NVMe-oF Target 并呈现为本地块设备
package nvmeof

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ========== Initiator 端核心结构 ==========

// Controller NVMe 控制器
type Controller struct {
	// 基本信息
	Name      string          `json:"name"`
	Transport TransportType   `json:"transport"`
	State     ControllerState `json:"state"`

	// 连接信息
	TrAddress string `json:"trAddress"` // 目标地址
	TrSVCID   string `json:"trSvcid"`   // 目标端口
	SubsysNQN string `json:"subsysNqn"` // 目标子系统 NQN
	HostNQN   string `json:"hostNqn"`   // 本地 Host NQN
	HostID    string `json:"hostId"`    // Host UUID

	// RDMA 特定配置
	TrSVCIDType string `json:"trSvcidType,omitempty"` // port, udp, tcp
	TrDHCHAPKey string `json:"trDhchapKey,omitempty"` // DHCHAP 密钥

	// 性能配置
	IOQueues     int  `json:"ioQueues"`     // IO 队列数
	IOQueueDepth int  `json:"ioQueueDepth"` // IO 队列深度
	KeepAliveTmo int  `json:"keepAliveTmo"` // Keep-alive 超时（秒）
	ReconnectTmo int  `json:"reconnectTmo"` // 重连超时（秒）
	PollMode     bool `json:"pollMode"`     // 轮询模式

	// 命名空间
	Namespaces map[string]*DiscoveredNamespace `json:"namespaces"`

	// 统计
	Stats ControllerStats `json:"stats"`

	// 连接时间
	ConnectedAt time.Time `json:"connectedAt,omitempty"`
}

// DiscoveredNamespace 发现的命名空间
type DiscoveredNamespace struct {
	// 基本信息
	NSID       uint32 `json:"nsid"`
	Name       string `json:"name"`
	DevicePath string `json:"devicePath"` // 本地设备路径 (如 /dev/nvme0n1)

	// 容量信息
	BlockSize uint32 `json:"blockSize"`
	Size      uint64 `json:"size"`

	// 状态
	Online   bool `json:"online"`
	ReadOnly bool `json:"readOnly"`

	// 挂载点
	MountPoint string `json:"mountPoint,omitempty"`
	FileSystem string `json:"fileSystem,omitempty"`

	// 所属控制器
	ControllerName string `json:"controllerName"`
}

// DiscoveryLogEntry 发现日志条目
type DiscoveryLogEntry struct {
	// 目标信息
	TrAddress string        `json:"trAddress"`
	TrSVCID   string        `json:"trSvcid"`
	Transport TransportType `json:"transport"`
	SubsysNQN string        `json:"subsysNqn"`

	// 目标类型
	Subtype string `json:"subtype"` // nvme (子系统), discovery (发现服务)

	// 安全信息
	SecureChannel bool `json:"secureChannel"`
}

// ControllerStats 控制器统计
type ControllerStats struct {
	ReadBytes    uint64 `json:"readBytes"`
	WriteBytes   uint64 `json:"writeBytes"`
	ReadOps      uint64 `json:"readOps"`
	WriteOps     uint64 `json:"writeOps"`
	AvgLatency   uint64 `json:"avgLatency"` // 微秒
	MaxLatency   uint64 `json:"maxLatency"` // 微秒
	Reconnects   uint64 `json:"reconnects"`
	Errors       uint64 `json:"errors"`
	ControllerID uint16 `json:"controllerId"`
}

// InitiatorStats Initiator 统计
type InitiatorStats struct {
	Controllers       int `json:"controllers"`
	Namespaces        int `json:"namespaces"`
	ActiveConnections int `json:"activeConnections"`
	DiscoverySessions int `json:"discoverySessions"`
}

// ========== Initiator Manager ==========

// InitiatorManager Initiator 端管理器
type InitiatorManager struct {
	mu sync.RWMutex

	config *NVMeOFConfig

	// 控制器
	controllers map[string]*Controller

	// 运行状态
	running atomic.Bool

	// 事件通道
	eventCh chan<- NVMeOFEvent
}

// NewInitiatorManager 创建 Initiator 管理器
func NewInitiatorManager(config *NVMeOFConfig) (*InitiatorManager, error) {
	return &InitiatorManager{
		config:      config,
		controllers: make(map[string]*Controller),
	}, nil
}

// Start 启动 Initiator 管理器
func (m *InitiatorManager) Start(ctx context.Context) error {
	if !m.running.CompareAndSwap(false, true) {
		return nil
	}

	// 初始化本地 Host NQN
	// 实际实现需要:
	// - 读取或生成 /etc/nvme/hostnqn
	// - 读取或生成 /etc/nvme/hostid

	return nil
}

// Stop 停止 Initiator 管理器
func (m *InitiatorManager) Stop() error {
	m.running.Store(false)

	// 断开所有控制器
	for _, ctrl := range m.controllers {
		m.disconnectControllerInternal(ctrl)
	}

	return nil
}

// SetEventChannel 设置事件通道
func (m *InitiatorManager) SetEventChannel(ch chan<- NVMeOFEvent) {
	m.eventCh = ch
}

// ========== 控制器管理 ==========

// ConnectController 连接到远程控制器
func (m *InitiatorManager) ConnectController(ctx context.Context, req *ConnectControllerRequest) (*Controller, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已连接
	if ctrl, exists := m.controllers[req.Name]; exists {
		if ctrl.State == ControllerStateLive {
			return nil, ErrControllerConnected
		}
	}

	// 验证传输类型
	if !ValidTransports[req.Transport] {
		return nil, ErrInvalidTransport
	}

	// 设置默认值
	ioQueues := req.IOQueues
	if ioQueues <= 0 {
		ioQueues = m.config.Initiator.IOQueues
	}

	ioQueueDepth := req.IOQueueDepth
	if ioQueueDepth <= 0 {
		ioQueueDepth = m.config.Initiator.IOQueueDepth
	}

	keepAlive := req.KeepAliveTmo
	if keepAlive <= 0 {
		keepAlive = m.config.Initiator.KeepAliveTimeout
	}

	// 创建控制器
	ctrl := &Controller{
		Name:         req.Name,
		Transport:    req.Transport,
		State:        ControllerStateConnecting,
		TrAddress:    req.TrAddress,
		TrSVCID:      req.TrSVCID,
		SubsysNQN:    req.SubsysNQN,
		HostNQN:      req.HostNQN,
		HostID:       req.HostID,
		IOQueues:     ioQueues,
		IOQueueDepth: ioQueueDepth,
		KeepAliveTmo: keepAlive,
		ReconnectTmo: m.config.Initiator.ReconnectDelay,
		PollMode:     m.config.Performance.PollMode,
		Namespaces:   make(map[string]*DiscoveredNamespace),
	}

	// 实际实现需要:
	// - 执行 nvme connect 命令
	// - 或写入 /sys/class/nvme/nvmeX/subsystemnqn 等

	m.connectControllerInternal(ctrl)

	m.controllers[req.Name] = ctrl

	// 发送事件
	if m.eventCh != nil {
		m.eventCh <- NVMeOFEvent{
			Type:       EventControllerConnected,
			Message:    fmt.Sprintf("Controller %s connected to %s", req.Name, req.SubsysNQN),
			Controller: req.Name,
			Time:       time.Now(),
		}
	}

	return ctrl, nil
}

func (m *InitiatorManager) connectControllerInternal(ctrl *Controller) {
	// 模拟连接过程
	// 实际实现需要:
	// - 执行: nvme connect -t <transport> -a <addr> -s <port> -n <subsysnqn>
	// - 或使用 netlink/libnvme

	time.Sleep(100 * time.Millisecond) // 模拟连接延迟

	ctrl.State = ControllerStateLive
	ctrl.ConnectedAt = time.Now()

	// 模拟发现命名空间
	ctrl.Namespaces["1"] = &DiscoveredNamespace{
		NSID:           1,
		Name:           fmt.Sprintf("%sns1", ctrl.Name),
		DevicePath:     fmt.Sprintf("/dev/%sn1", ctrl.Name),
		BlockSize:      512,
		Size:           1024 * 1024 * 1024 * 1024, // 1TB
		Online:         true,
		ReadOnly:       false,
		ControllerName: ctrl.Name,
	}
}

// DisconnectController 断开控制器连接
func (m *InitiatorManager) DisconnectController(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctrl, exists := m.controllers[name]
	if !exists {
		return ErrControllerDisconnected
	}

	m.disconnectControllerInternal(ctrl)

	delete(m.controllers, name)

	// 发送事件
	if m.eventCh != nil {
		m.eventCh <- NVMeOFEvent{
			Type:       EventControllerDisconnected,
			Message:    fmt.Sprintf("Controller %s disconnected", name),
			Controller: name,
			Time:       time.Now(),
		}
	}

	return nil
}

func (m *InitiatorManager) disconnectControllerInternal(ctrl *Controller) {
	// 实际实现需要:
	// - 执行: nvme disconnect -n <subsysnqn>
	// - 或写入 /sys/class/nvme/nvmeX/delete_controller

	ctrl.State = ControllerStateDead
}

// GetController 获取控制器
func (m *InitiatorManager) GetController(name string) (*Controller, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ctrl, exists := m.controllers[name]
	if !exists {
		return nil, ErrControllerDisconnected
	}

	return ctrl, nil
}

// ListControllers 列出所有控制器
func (m *InitiatorManager) ListControllers() []*Controller {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Controller, 0, len(m.controllers))
	for _, c := range m.controllers {
		result = append(result, c)
	}
	return result
}

// ReconnectController 重新连接控制器
func (m *InitiatorManager) ReconnectController(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctrl, exists := m.controllers[name]
	if !exists {
		return ErrControllerDisconnected
	}

	if ctrl.State == ControllerStateLive {
		return nil
	}

	ctrl.State = ControllerStateResetting

	m.connectControllerInternal(ctrl)

	ctrl.Stats.Reconnects++

	return nil
}

// ========== 发现服务 ==========

// DiscoveryController 发现控制器
type DiscoveryController struct {
	Transport TransportType `json:"transport"`
	TrAddress string        `json:"trAddress"`
	TrSVCID   string        `json:"trSvcid"`
	HostNQN   string        `json:"hostNqn"`
	HostID    string        `json:"hostId"`
	Connected bool          `json:"connected"`

	// 缓存的发现日志
	Entries []DiscoveryLogEntry `json:"entries"`
}

// Discover 发现远程子系统
func (m *InitiatorManager) Discover(ctx context.Context, req *DiscoverRequest) (*DiscoveryResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 实际实现需要:
	// - 执行: nvme discover -t <transport> -a <addr> -s <port>

	result := &DiscoveryResult{
		Transport: req.Transport,
		Address:   req.TrAddress,
		Port:      req.TrSVCID,
		Entries: []DiscoveryLogEntry{
			{
				TrAddress: req.TrAddress,
				TrSVCID:   "4420",
				Transport: req.Transport,
				SubsysNQN: "nqn.2024-01.example:nvme:subsystem1",
				Subtype:   "nvme",
			},
		},
		DiscoveredAt: time.Now(),
	}

	return result, nil
}

// DiscoveryResult 发现结果
type DiscoveryResult struct {
	Transport    TransportType       `json:"transport"`
	Address      string              `json:"address"`
	Port         string              `json:"port"`
	Entries      []DiscoveryLogEntry `json:"entries"`
	DiscoveredAt time.Time           `json:"discoveredAt"`
}

// GetDiscoveredNamespaces 获取发现的命名空间
func (m *InitiatorManager) GetDiscoveredNamespaces(controllerName string) ([]*DiscoveredNamespace, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ctrl, exists := m.controllers[controllerName]
	if !exists {
		return nil, ErrControllerDisconnected
	}

	result := make([]*DiscoveredNamespace, 0, len(ctrl.Namespaces))
	for _, ns := range ctrl.Namespaces {
		result = append(result, ns)
	}
	return result, nil
}

// GetStats 获取统计信息
func (m *InitiatorManager) GetStats() *InitiatorStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &InitiatorStats{
		Controllers: len(m.controllers),
	}

	for _, ctrl := range m.controllers {
		stats.Namespaces += len(ctrl.Namespaces)
		if ctrl.State == ControllerStateLive {
			stats.ActiveConnections++
		}
	}

	return stats
}

// ========== 请求结构 ==========

// ConnectControllerRequest 连接控制器请求
type ConnectControllerRequest struct {
	Name         string        `json:"name" validate:"required"`      // 控制器名称
	Transport    TransportType `json:"transport" validate:"required"` // 传输类型
	TrAddress    string        `json:"trAddress" validate:"required"` // 目标地址
	TrSVCID      string        `json:"trSvcid"`                       // 目标端口 (默认 4420)
	SubsysNQN    string        `json:"subsysNqn" validate:"required"` // 目标子系统 NQN
	HostNQN      string        `json:"hostNqn,omitempty"`             // Host NQN (可选)
	HostID       string        `json:"hostId,omitempty"`              // Host UUID (可选)
	IOQueues     int           `json:"ioQueues,omitempty"`            // IO 队列数
	IOQueueDepth int           `json:"ioQueueDepth,omitempty"`        // IO 队列深度
	KeepAliveTmo int           `json:"keepAliveTmo,omitempty"`        // Keep-alive 超时
	DHCHAPKey    string        `json:"dhchapKey,omitempty"`           // DHCHAP 密钥
}

// DiscoverRequest 发现请求
type DiscoverRequest struct {
	Transport TransportType `json:"transport" validate:"required"` // 传输类型
	TrAddress string        `json:"trAddress" validate:"required"` // 发现服务地址
	TrSVCID   string        `json:"trSvcid"`                       // 端口 (默认 8009)
	HostNQN   string        `json:"hostNqn,omitempty"`             // Host NQN
	HostID    string        `json:"hostId,omitempty"`              // Host UUID
}
