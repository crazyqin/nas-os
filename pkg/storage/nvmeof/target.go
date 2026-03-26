// Package nvmeof - Target 端管理
// NVMe-oF Target 暴露本地 NVMe 设备或文件作为网络块设备
package nvmeof

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ========== Target 端核心结构 ==========

// Subsystem NVMe-oF 子系统
type Subsystem struct {
	mu sync.RWMutex

	// 基本信息
	NQN         string         `json:"nqn"`         // NVMe Qualified Name
	Name        string         `json:"name"`        // 简短名称
	Description string         `json:"description"` // 描述
	State       SubsystemState `json:"state"`       // 状态

	// 配置
	AllowAnyHost bool `json:"allowAnyHost"` // 允许任何主机
	MaxNamespaces int `json:"maxNamespaces"` // 最大命名空间数

	// 命名空间
	Namespaces map[string]*Namespace `json:"namespaces"`

	// 监听器
	Listeners map[string]*Listener `json:"listeners"`

	// 允许的主机
	AllowedHosts map[string]*Host `json:"allowedHosts"`

	// 统计
	Stats SubsystemStats `json:"stats"`

	// 创建时间
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Namespace NVMe 命名空间
type Namespace struct {
	mu sync.RWMutex

	// 基本信息
	NSID    uint32 `json:"nsid"`    // Namespace ID (1-based)
	Name    string `json:"name"`    // 名称
	Enabled bool   `json:"enabled"` // 是否启用

	// 后端存储
	DevicePath string `json:"devicePath"` // 后端设备路径 (如 /dev/nvme0n1)
	BlockSize  uint32 `json:"blockSize"`  // 块大小
	Size       uint64 `json:"size"`        // 大小（字节）

	// 配置
	ReadOnly      bool   `json:"readOnly"`      // 只读
	PiEnable      bool   `json:"piEnable"`      // 保护信息
	NGUID         string `json:"nguid"`         // Namespace GUID
	UUID          string `json:"uuid"`          // Namespace UUID
	EUI64         string `json:"eui64"`         // IEEE Extended Unique Identifier
	Anagrpid      uint32 `json:"anagrpid"`      // ANA Group ID
	TransferSize  uint32 `json:"transferSize"`  // 传输大小

	// 统计
	Stats NamespaceStats `json:"stats"`

	// 所属子系统
	SubsystemNQN string `json:"subsystemNqn"`
}

// Listener NVMe-oF 监听器
type Listener struct {
	mu sync.RWMutex

	// 基本信息
	ID   string `json:"id"`
	Name string `json:"name"`

	// 网络配置
	Transport   TransportType `json:"transport"`   // 传输类型
	TrAddress   string        `json:"trAddress"`   // 传输地址 (IP)
	TrSVCID     string        `json:"trSvcid"`     // 传输服务 ID (端口)
	TrType      string        `json:"trType"`      // 传输类型字符串

	// RDMA 特定配置
	TrDHCHAPCtrlKey string `json:"trDhchapCtrlKey,omitempty"` // DHCHAP 控制器密钥

	// 状态
	State     ConnectionState `json:"state"`
	Enabled   bool            `json:"enabled"`

	// 所属子系统
	SubsystemNQN string `json:"subsystemNqn"`

	// 统计
	Stats ListenerStats `json:"stats"`
}

// Host NVMe-oF 主机
type Host struct {
	mu sync.RWMutex

	// 基本信息
	NQN  string `json:"nqn"`  // Host NQN
	Name string `json:"name"` // 简短名称

	// 配置
	DHCHAPKey string `json:"dhchapKey,omitempty"` // DHCHAP 密钥

	// 允许访问的子系统
	AllowedSubsystems map[string]bool `json:"allowedSubsystems"`

	// 连接状态
	Connected bool `json:"connected"`

	// 所属子系统
	SubsystemNQN string `json:"subsystemNqn"`

	// 统计
	Stats HostStats `json:"stats"`
}

// ========== 统计结构 ==========

// SubsystemStats 子系统统计
type SubsystemStats struct {
	TotalReadBytes    uint64 `json:"totalReadBytes"`
	TotalWriteBytes   uint64 `json:"totalWriteBytes"`
	TotalReadOps      uint64 `json:"totalReadOps"`
	TotalWriteOps     uint64 `json:"totalWriteOps"`
	ActiveConnections int    `json:"activeConnections"`
	TotalConnections  uint64 `json:"totalConnections"`
	Errors            uint64 `json:"errors"`
}

// NamespaceStats 命名空间统计
type NamespaceStats struct {
	ReadBytes  uint64 `json:"readBytes"`
	WriteBytes uint64 `json:"writeBytes"`
	ReadOps    uint64 `json:"readOps"`
	WriteOps   uint64 `json:"writeOps"`
	Latency    uint64 `json:"latency"` // 平均延迟（微秒）
}

// ListenerStats 监听器统计
type ListenerStats struct {
	Connections    uint64 `json:"connections"`
	ActiveSessions int    `json:"activeSessions"`
	BytesReceived  uint64 `json:"bytesReceived"`
	BytesSent      uint64 `json:"bytesSent"`
}

// HostStats 主机统计
type HostStats struct {
	BytesRead    uint64 `json:"bytesRead"`
	BytesWritten uint64 `json:"bytesWritten"`
	ReadOps      uint64 `json:"readOps"`
	WriteOps     uint64 `json:"writeOps"`
	Connections  uint64 `json:"connections"`
}

// TargetStats Target 端统计
type TargetStats struct {
	Subsystems      int `json:"subsystems"`
	Namespaces      int `json:"namespaces"`
	Listeners       int `json:"listeners"`
	Hosts           int `json:"hosts"`
	ActiveConnections int `json:"activeConnections"`
}

// ========== Target Manager ==========

// TargetManager Target 端管理器
type TargetManager struct {
	mu sync.RWMutex

	config *NVMeOFConfig

	// 子系统
	subsystems map[string]*Subsystem

	// 运行状态
	running atomic.Bool

	// 事件通道
	eventCh chan<- NVMeOFEvent
}

// NewTargetManager 创建 Target 管理器
func NewTargetManager(config *NVMeOFConfig) (*TargetManager, error) {
	return &TargetManager{
		config:     config,
		subsystems: make(map[string]*Subsystem),
	}, nil
}

// Start 启动 Target 管理器
func (m *TargetManager) Start(ctx context.Context) error {
	if !m.running.CompareAndSwap(false, true) {
		return nil
	}

	// 加载已存在的子系统配置
	// 实际实现需要读取 /sys/kernel/config/nvmet/subsystems/

	return nil
}

// Stop 停止 Target 管理器
func (m *TargetManager) Stop() error {
	m.running.Store(false)
	return nil
}

// SetEventChannel 设置事件通道
func (m *TargetManager) SetEventChannel(ch chan<- NVMeOFEvent) {
	m.eventCh = ch
}

// ========== 子系统管理 ==========

// CreateSubsystem 创建子系统
func (m *TargetManager) CreateSubsystem(ctx context.Context, req *CreateSubsystemRequest) (*Subsystem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已存在
	if _, exists := m.subsystems[req.NQN]; exists {
		return nil, ErrSubsystemExists
	}

	// 检查数量限制
	if len(m.subsystems) >= m.config.Target.MaxSubsystems {
		return nil, fmt.Errorf("maximum subsystems reached: %d", m.config.Target.MaxSubsystems)
	}

	// 创建子系统
	subsystem := &Subsystem{
		NQN:           req.NQN,
		Name:          req.Name,
		Description:   req.Description,
		State:         SubsystemStateInactive,
		AllowAnyHost:  req.AllowAnyHost,
		MaxNamespaces: m.config.Target.MaxNamespaces,
		Namespaces:    make(map[string]*Namespace),
		Listeners:     make(map[string]*Listener),
		AllowedHosts:  make(map[string]*Host),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// 实际实现需要:
	// - 创建 /sys/kernel/config/nvmet/subsystems/<nqn>
	// - 设置 attr_allow_any_host

	m.subsystems[req.NQN] = subsystem

	// 发送事件
	if m.eventCh != nil {
		m.eventCh <- NVMeOFEvent{
			Type:      EventSubsystemCreated,
			Message:   fmt.Sprintf("Subsystem %s created", req.NQN),
			Subsystem: req.NQN,
			Time:      time.Now(),
		}
	}

	return subsystem, nil
}

// DeleteSubsystem 删除子系统
func (m *TargetManager) DeleteSubsystem(ctx context.Context, nqn string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	subsystem, exists := m.subsystems[nqn]
	if !exists {
		return ErrSubsystemNotFound
	}

	// 检查是否有连接
	if subsystem.Stats.ActiveConnections > 0 && !force {
		return fmt.Errorf("subsystem has active connections, use force to delete")
	}

	// 删除所有监听器
	for _, listener := range subsystem.Listeners {
		_ = m.deleteListenerInternal(subsystem, listener.ID)
	}

	// 删除所有命名空间
	for _, ns := range subsystem.Namespaces {
		_ = m.deleteNamespaceInternal(subsystem, ns.NSID)
	}

	// 实际实现需要:
	// - 删除 /sys/kernel/config/nvmet/subsystems/<nqn>

	delete(m.subsystems, nqn)

	// 发送事件
	if m.eventCh != nil {
		m.eventCh <- NVMeOFEvent{
			Type:      EventSubsystemDeleted,
			Message:   fmt.Sprintf("Subsystem %s deleted", nqn),
			Subsystem: nqn,
			Time:      time.Now(),
		}
	}

	return nil
}

// GetSubsystem 获取子系统
func (m *TargetManager) GetSubsystem(nqn string) (*Subsystem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subsystem, exists := m.subsystems[nqn]
	if !exists {
		return nil, ErrSubsystemNotFound
	}

	return subsystem, nil
}

// ListSubsystems 列出所有子系统
func (m *TargetManager) ListSubsystems() []*Subsystem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Subsystem, 0, len(m.subsystems))
	for _, s := range m.subsystems {
		result = append(result, s)
	}
	return result
}

// ActivateSubsystem 激活子系统
func (m *TargetManager) ActivateSubsystem(ctx context.Context, nqn string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	subsystem, exists := m.subsystems[nqn]
	if !exists {
		return ErrSubsystemNotFound
	}

	// 检查是否有命名空间
	if len(subsystem.Namespaces) == 0 {
		return fmt.Errorf("subsystem has no namespaces")
	}

	// 检查是否有监听器
	if len(subsystem.Listeners) == 0 {
		return fmt.Errorf("subsystem has no listeners")
	}

	subsystem.State = SubsystemStateActive
	subsystem.UpdatedAt = time.Now()

	return nil
}

// DeactivateSubsystem 停用子系统
func (m *TargetManager) DeactivateSubsystem(ctx context.Context, nqn string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	subsystem, exists := m.subsystems[nqn]
	if !exists {
		return ErrSubsystemNotFound
	}

	subsystem.State = SubsystemStateInactive
	subsystem.UpdatedAt = time.Now()

	return nil
}

// ========== 命名空间管理 ==========

// CreateNamespace 创建命名空间
func (m *TargetManager) CreateNamespace(ctx context.Context, subsystemNQN string, req *CreateNamespaceRequest) (*Namespace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	subsystem, exists := m.subsystems[subsystemNQN]
	if !exists {
		return nil, ErrSubsystemNotFound
	}

	// 检查数量限制
	if len(subsystem.Namespaces) >= subsystem.MaxNamespaces {
		return nil, fmt.Errorf("maximum namespaces reached for subsystem: %d", subsystem.MaxNamespaces)
	}

	// 分配 NSID
	nsid := req.NSID
	if nsid == 0 {
		nsid = m.allocateNSID(subsystem)
	}

	nsKey := fmt.Sprintf("%d", nsid)
	if _, exists := subsystem.Namespaces[nsKey]; exists {
		return nil, ErrNamespaceExists
	}

	// 创建命名空间
	namespace := &Namespace{
		NSID:         nsid,
		Name:         req.Name,
		Enabled:      true,
		DevicePath:   req.DevicePath,
		BlockSize:    req.BlockSize,
		Size:         req.Size,
		ReadOnly:     req.ReadOnly,
		PiEnable:     req.PiEnable,
		NGUID:        req.NGUID,
		UUID:         req.UUID,
		EUI64:        req.EUI64,
		SubsystemNQN: subsystemNQN,
	}

	// 实际实现需要:
	// - 创建 /sys/kernel/config/nvmet/subsystems/<nqn>/namespaces/<nsid>
	// - 设置 device_path, enable 等

	subsystem.Namespaces[nsKey] = namespace
	subsystem.UpdatedAt = time.Now()

	// 发送事件
	if m.eventCh != nil {
		m.eventCh <- NVMeOFEvent{
			Type:      EventNamespaceCreated,
			Message:   fmt.Sprintf("Namespace %d created in subsystem %s", nsid, subsystemNQN),
			Subsystem: subsystemNQN,
			Namespace: nsKey,
			Time:      time.Now(),
		}
	}

	return namespace, nil
}

// DeleteNamespace 删除命名空间
func (m *TargetManager) DeleteNamespace(ctx context.Context, subsystemNQN string, nsid uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	subsystem, exists := m.subsystems[subsystemNQN]
	if !exists {
		return ErrSubsystemNotFound
	}

	return m.deleteNamespaceInternal(subsystem, nsid)
}

func (m *TargetManager) deleteNamespaceInternal(subsystem *Subsystem, nsid uint32) error {
	nsKey := fmt.Sprintf("%d", nsid)
	if _, exists := subsystem.Namespaces[nsKey]; !exists {
		return ErrNamespaceNotFound
	}

	// 实际实现需要:
	// - 禁用: /sys/kernel/config/nvmet/subsystems/<nqn>/namespaces/<nsid>/enable = 0
	// - 删除目录

	delete(subsystem.Namespaces, nsKey)
	subsystem.UpdatedAt = time.Now()

	// 发送事件
	if m.eventCh != nil {
		m.eventCh <- NVMeOFEvent{
			Type:      EventNamespaceDeleted,
			Message:   fmt.Sprintf("Namespace %d deleted from subsystem %s", nsid, subsystem.NQN),
			Subsystem: subsystem.NQN,
			Namespace: nsKey,
			Time:      time.Now(),
		}
	}

	return nil
}

// allocateNSID 分配 NSID
func (m *TargetManager) allocateNSID(subsystem *Subsystem) uint32 {
	for i := uint32(1); i <= uint32(subsystem.MaxNamespaces); i++ {
		key := fmt.Sprintf("%d", i)
		if _, exists := subsystem.Namespaces[key]; !exists {
			return i
		}
	}
	return 1
}

// ListNamespaces 列出子系统的命名空间
func (m *TargetManager) ListNamespaces(subsystemNQN string) ([]*Namespace, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subsystem, exists := m.subsystems[subsystemNQN]
	if !exists {
		return nil, ErrSubsystemNotFound
	}

	result := make([]*Namespace, 0, len(subsystem.Namespaces))
	for _, ns := range subsystem.Namespaces {
		result = append(result, ns)
	}
	return result, nil
}

// ========== 监听器管理 ==========

// CreateListener 创建监听器
func (m *TargetManager) CreateListener(ctx context.Context, subsystemNQN string, req *CreateListenerRequest) (*Listener, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	subsystem, exists := m.subsystems[subsystemNQN]
	if !exists {
		return nil, ErrSubsystemNotFound
	}

	// 验证传输类型
	if !ValidTransports[req.Transport] {
		return nil, ErrInvalidTransport
	}

	// 生成监听器 ID
	listenerID := fmt.Sprintf("%s-%s-%s", req.Transport, req.TrAddress, req.TrSVCID)
	if _, exists := subsystem.Listeners[listenerID]; exists {
		return nil, fmt.Errorf("listener already exists: %s", listenerID)
	}

	// 默认端口
	port := req.TrSVCID
	if port == "" {
		port = fmt.Sprintf("%d", m.config.Target.DefaultPort)
	}

	// 创建监听器
	listener := &Listener{
		ID:           listenerID,
		Name:         req.Name,
		Transport:    req.Transport,
		TrAddress:    req.TrAddress,
		TrSVCID:      port,
		TrType:       string(req.Transport),
		State:        ConnectionStateUp,
		Enabled:      true,
		SubsystemNQN: subsystemNQN,
	}

	// 实际实现需要:
	// - 创建 /sys/kernel/config/nvmet/ports/<port>/
	// - 设置 addr_trtype, addr_traddr, addr_trsvcid
	// - 链接到子系统: /sys/kernel/config/nvmet/ports/<port>/subsystems/<nqn>

	subsystem.Listeners[listenerID] = listener
	subsystem.UpdatedAt = time.Now()

	return listener, nil
}

// DeleteListener 删除监听器
func (m *TargetManager) DeleteListener(ctx context.Context, subsystemNQN string, listenerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	subsystem, exists := m.subsystems[subsystemNQN]
	if !exists {
		return ErrSubsystemNotFound
	}

	return m.deleteListenerInternal(subsystem, listenerID)
}

func (m *TargetManager) deleteListenerInternal(subsystem *Subsystem, listenerID string) error {
	if _, exists := subsystem.Listeners[listenerID]; !exists {
		return ErrListenerNotFound
	}

	// 实际实现需要:
	// - 取消链接: /sys/kernel/config/nvmet/ports/<port>/subsystems/<nqn>
	// - 删除端口配置

	delete(subsystem.Listeners, listenerID)
	subsystem.UpdatedAt = time.Now()

	return nil
}

// ListListeners 列出监听器
func (m *TargetManager) ListListeners(subsystemNQN string) ([]*Listener, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subsystem, exists := m.subsystems[subsystemNQN]
	if !exists {
		return nil, ErrSubsystemNotFound
	}

	result := make([]*Listener, 0, len(subsystem.Listeners))
	for _, l := range subsystem.Listeners {
		result = append(result, l)
	}
	return result, nil
}

// ========== 主机管理 ==========

// AddHost 添加允许的主机
func (m *TargetManager) AddHost(ctx context.Context, subsystemNQN string, req *AddHostRequest) (*Host, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	subsystem, exists := m.subsystems[subsystemNQN]
	if !exists {
		return nil, ErrSubsystemNotFound
	}

	if _, exists := subsystem.AllowedHosts[req.NQN]; exists {
		return nil, fmt.Errorf("host already exists: %s", req.NQN)
	}

	host := &Host{
		NQN:               req.NQN,
		Name:              req.Name,
		DHCHAPKey:         req.DHCHAPKey,
		AllowedSubsystems: make(map[string]bool),
		Connected:         false,
		SubsystemNQN:      subsystemNQN,
	}

	// 实际实现需要:
	// - 创建 /sys/kernel/config/nvmet/subsystems/<nqn>/allowed_hosts/<hostnqn>

	subsystem.AllowedHosts[req.NQN] = host
	subsystem.UpdatedAt = time.Now()

	// 发送事件
	if m.eventCh != nil {
		m.eventCh <- NVMeOFEvent{
			Type:      EventHostConnected,
			Message:   fmt.Sprintf("Host %s added to subsystem %s", req.NQN, subsystemNQN),
			Subsystem: subsystemNQN,
			Host:      req.NQN,
			Time:      time.Now(),
		}
	}

	return host, nil
}

// RemoveHost 移除主机
func (m *TargetManager) RemoveHost(ctx context.Context, subsystemNQN string, hostNQN string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	subsystem, exists := m.subsystems[subsystemNQN]
	if !exists {
		return ErrSubsystemNotFound
	}

	if _, exists := subsystem.AllowedHosts[hostNQN]; !exists {
		return ErrHostNotFound
	}

	// 实际实现需要:
	// - 删除 /sys/kernel/config/nvmet/subsystems/<nqn>/allowed_hosts/<hostnqn>

	delete(subsystem.AllowedHosts, hostNQN)
	subsystem.UpdatedAt = time.Now()

	// 发送事件
	if m.eventCh != nil {
		m.eventCh <- NVMeOFEvent{
			Type:      EventHostDisconnected,
			Message:   fmt.Sprintf("Host %s removed from subsystem %s", hostNQN, subsystemNQN),
			Subsystem: subsystemNQN,
			Host:      hostNQN,
			Time:      time.Now(),
		}
	}

	return nil
}

// ListHosts 列出允许的主机
func (m *TargetManager) ListHosts(subsystemNQN string) ([]*Host, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subsystem, exists := m.subsystems[subsystemNQN]
	if !exists {
		return nil, ErrSubsystemNotFound
	}

	result := make([]*Host, 0, len(subsystem.AllowedHosts))
	for _, h := range subsystem.AllowedHosts {
		result = append(result, h)
	}
	return result, nil
}

// GetStats 获取统计信息
func (m *TargetManager) GetStats() *TargetStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &TargetStats{
		Subsystems: len(m.subsystems),
	}

	for _, s := range m.subsystems {
		stats.Namespaces += len(s.Namespaces)
		stats.Listeners += len(s.Listeners)
		stats.Hosts += len(s.AllowedHosts)
		stats.ActiveConnections += s.Stats.ActiveConnections
	}

	return stats
}

// ========== 请求结构 ==========

// CreateSubsystemRequest 创建子系统请求
type CreateSubsystemRequest struct {
	NQN          string `json:"nqn" validate:"required"`          // NVMe Qualified Name
	Name         string `json:"name"`                             // 简短名称
	Description  string `json:"description"`                      // 描述
	AllowAnyHost bool   `json:"allowAnyHost"`                     // 允许任何主机
	MaxNamespaces int   `json:"maxNamespaces,omitempty"`          // 最大命名空间数
}

// CreateNamespaceRequest 创建命名空间请求
type CreateNamespaceRequest struct {
	NSID       uint32 `json:"nsid,omitempty"`       // Namespace ID (可选，自动分配)
	Name       string `json:"name"`                 // 名称
	DevicePath string `json:"devicePath" validate:"required"` // 后端设备路径
	BlockSize  uint32 `json:"blockSize,omitempty"`  // 块大小 (默认 512)
	Size       uint64 `json:"size,omitempty"`       // 大小 (可选，从设备获取)
	ReadOnly   bool   `json:"readOnly"`             // 只读
	PiEnable   bool   `json:"piEnable"`             // 保护信息
	NGUID      string `json:"nguid,omitempty"`      // Namespace GUID
	UUID       string `json:"uuid,omitempty"`       // Namespace UUID
	EUI64      string `json:"eui64,omitempty"`      // EUI-64
}

// CreateListenerRequest 创建监听器请求
type CreateListenerRequest struct {
	Name      string        `json:"name"`                // 名称
	Transport TransportType `json:"transport" validate:"required"` // 传输类型
	TrAddress string        `json:"trAddress" validate:"required"` // IP 地址
	TrSVCID   string        `json:"trSvcid"`             // 端口 (可选)
}

// AddHostRequest 添加主机请求
type AddHostRequest struct {
	NQN       string `json:"nqn" validate:"required"` // Host NQN
	Name      string `json:"name"`                    // 简短名称
	DHCHAPKey string `json:"dhchapKey,omitempty"`     // DHCHAP 密钥
}