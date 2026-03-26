// Package nvmeof 实现 NVMe over Fabrics (NVMe-oF) 高性能存储网络
// 支持 NVMe over TCP 和 NVMe over RDMA (RoCEv2/iWARP/InfiniBand)
package nvmeof

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ========== 核心错误定义 ==========

var (
	// ErrSubsystemNotFound 子系统未找到
	ErrSubsystemNotFound = errors.New("nvme-of subsystem not found")
	// ErrNamespaceNotFound 命名空间未找到
	ErrNamespaceNotFound = errors.New("namespace not found")
	// ErrListenerNotFound 监听器未找到
	ErrListenerNotFound = errors.New("listener not found")
	// ErrHostNotFound 主机未找到
	ErrHostNotFound = errors.New("host not found")
	// ErrSubsystemExists 子系统已存在
	ErrSubsystemExists = errors.New("subsystem already exists")
	// ErrNamespaceExists 命名空间已存在
	ErrNamespaceExists = errors.New("namespace already exists")
	// ErrInvalidTransport 无效传输类型
	ErrInvalidTransport = errors.New("invalid transport type")
	// ErrControllerConnected 控制器已连接
	ErrControllerConnected = errors.New("controller already connected")
	// ErrControllerDisconnected 控制器未连接
	ErrControllerDisconnected = errors.New("controller not connected")
	// ErrNVMENotAvailable NVMe-oF 不可用
	ErrNVMENotAvailable = errors.New("nvme-of not available on this system")
	// ErrOperationTimeout 操作超时
	ErrOperationTimeout = errors.New("nvme-of operation timeout")
	// ErrInvalidConfig 无效配置
	ErrInvalidConfig = errors.New("invalid nvme-of configuration")
	// ErrPermissionDenied 权限不足
	ErrPermissionDenied = errors.New("permission denied for nvme-of operation")
)

// ========== 传输类型 ==========

// TransportType NVMe-oF 传输类型
type TransportType string

const (
	// TransportTCP NVMe over TCP
	TransportTCP TransportType = "tcp"
	// TransportRDMA NVMe over RDMA (RoCEv2/iWARP/IB)
	TransportRDMA TransportType = "rdma"
	// TransportFC NVMe over Fibre Channel
	TransportFC TransportType = "fc"
	// TransportLoop NVMe over Loop (本地)
	TransportLoop TransportType = "loop"
)

// ValidTransports 有效传输类型
var ValidTransports = map[TransportType]bool{
	TransportTCP:  true,
	TransportRDMA: true,
	TransportFC:   true,
	TransportLoop: true,
}

// ========== 子系统状态 ==========

// SubsystemState 子系统状态
type SubsystemState string

const (
	// SubsystemStateInactive 未激活
	SubsystemStateInactive SubsystemState = "inactive"
	// SubsystemStateActive 激活中
	SubsystemStateActive SubsystemState = "active"
	// SubsystemStateDegraded 降级
	SubsystemStateDegraded SubsystemState = "degraded"
	// SubsystemStateError 错误
	SubsystemStateError SubsystemState = "error"
)

// ========== 控制器状态 ==========

// ControllerState 控制器状态
type ControllerState string

const (
	// ControllerStateConnecting 连接中
	ControllerStateConnecting ControllerState = "connecting"
	// ControllerStateLive 已连接
	ControllerStateLive ControllerState = "live"
	// ControllerStateDisconnecting 断开中
	ControllerStateDisconnecting ControllerState = "disconnecting"
	// ControllerStateDead 已断开
	ControllerStateDead ControllerState = "dead"
	// ControllerStateResetting 重置中
	ControllerStateResetting ControllerState = "resetting"
)

// ========== 连接状态 ==========

// ConnectionState 连接状态
type ConnectionState string

const (
	// ConnectionStateUp 连接正常
	ConnectionStateUp ConnectionState = "up"
	// ConnectionStateDown 连接断开
	ConnectionStateDown ConnectionState = "down"
	// ConnectionStateConnecting 连接中
	ConnectionStateConnecting ConnectionState = "connecting"
	// ConnectionStateReconnecting 重连中
	ConnectionStateReconnecting ConnectionState = "reconnecting"
)

// ========== NVMe-oF 核心配置 ==========

// NVMeOFConfig NVMe-oF 全局配置
type NVMeOFConfig struct {
	// 是否启用 NVMe-oF
	Enabled bool `json:"enabled"`

	// 默认传输类型
	DefaultTransport TransportType `json:"defaultTransport"`

	// Target 配置
	Target TargetConfig `json:"target"`

	// Initiator 配置
	Initiator InitiatorConfig `json:"initiator"`

	// 性能调优
	Performance PerformanceConfig `json:"performance"`

	// 安全配置
	Security SecurityConfig `json:"security"`

	// 监控配置
	Monitoring MonitoringConfig `json:"monitoring"`
}

// TargetConfig Target 端配置
type TargetConfig struct {
	// 是否启用 Target 模式
	Enabled bool `json:"enabled"`

	// 默认端口
	DefaultPort int `json:"defaultPort"`

	// 最大子系统数
	MaxSubsystems int `json:"maxSubsystems"`

	// 最大命名空间数
	MaxNamespaces int `json:"maxNamespaces"`

	// 最大连接数
	MaxConnections int `json:"maxConnections"`

	// IO队列深度
	IOQueueDepth int `json:"ioQueueDepth"`

	// 管理队列深度
	AdminQueueDepth int `json:"adminQueueDepth"`

	// 是否允许任何主机连接
	AllowAnyHost bool `json:"allowAnyHost"`
}

// InitiatorConfig Initiator 端配置
type InitiatorConfig struct {
	// 是否启用 Initiator 模式
	Enabled bool `json:"enabled"`

	// 重连策略
	ReconnectDelay int `json:"reconnectDelay"` // 秒
	MaxReconnect   int `json:"maxReconnect"`

	// IO队列数
	IOQueues int `json:"ioQueues"`

	// IO队列深度
	IOQueueDepth int `json:"ioQueueDepth"`

	// 控制器超时
	ControllerTimeout int `json:"controllerTimeout"` // 秒

	// Keep-alive 超时
	KeepAliveTimeout int `json:"keepAliveTimeout"` // 秒
}

// PerformanceConfig 性能配置
type PerformanceConfig struct {
	// 是否启用轮询模式
	PollMode bool `json:"pollMode"`

	// 批量 IO 大小
	MaxIOSize int `json:"maxIoSize"` // KB

	// 内联数据大小
	InlineDataSize int `json:"inlineDataSize"`

	// 内存页大小
	PageSize int `json:"pageSize"` // KB

	// CPU 亲和性
	CPUAffinity []int `json:"cpuAffinity"`

	// 是否启用零拷贝
	ZeroCopy bool `json:"zeroCopy"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	// 是否启用认证
	EnableAuth bool `json:"enableAuth"`

	// DHCHAP 配置
	DHCHAPConfig *DHCHAPConfig `json:"dhchapConfig,omitempty"`

	// TLS 配置
	TLSConfig *TLSConfig `json:"tlsConfig,omitempty"`

	// 允许的 IP 范围
	AllowedIPs []string `json:"allowedIps"`
}

// DHCHAPConfig DHCHAP 认证配置
type DHCHAPConfig struct {
	Enabled    bool   `json:"enabled"`
	HashType   string `json:"hashType"`   // sha256, sha384, sha512
	DHGroup    string `json:"dhGroup"`    // null, 2048, 3072, 4096, 6144, 8192
	KeyFile    string `json:"keyFile"`    // DHCHAP key 文件路径
	CtrlKey    string `json:"ctrlKey"`    // 控制器密钥
	HostKey    string `json:"hostKey"`    // 主机密钥
	DigestSync bool   `json:"digestSync"` // 同步摘要
}

// TLSConfig TLS 配置
type TLSConfig struct {
	Enabled      bool     `json:"enabled"`
	CertFile     string   `json:"certFile"`
	KeyFile      string   `json:"keyFile"`
	CAFile       string   `json:"caFile"`
	CipherSuites []string `json:"cipherSuites"`
	MinVersion   string   `json:"minVersion"` // TLS 1.2, TLS 1.3
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	// 是否启用监控
	Enabled bool `json:"enabled"`

	// 指标采集间隔
	MetricsInterval int `json:"metricsInterval"` // 秒

	// 是否启用事件通知
	EnableEvents bool `json:"enableEvents"`

	// 是否记录 IO 统计
	IOStats bool `json:"ioStats"`

	// 告警阈值
	AlertThresholds AlertThresholds `json:"alertThresholds"`
}

// AlertThresholds 告警阈值
type AlertThresholds struct {
	// 连接延迟阈值（毫秒）
	LatencyHigh int `json:"latencyHigh"`
	// 错误率阈值（百分比）
	ErrorRateHigh int `json:"errorRateHigh"`
	// 队列深度阈值（百分比）
	QueueDepthHigh int `json:"queueDepthHigh"`
}

// DefaultNVMeOFConfig 返回默认配置
func DefaultNVMeOFConfig() *NVMeOFConfig {
	return &NVMeOFConfig{
		Enabled:          true,
		DefaultTransport: TransportTCP,
		Target: TargetConfig{
			Enabled:         true,
			DefaultPort:     4420,
			MaxSubsystems:   1024,
			MaxNamespaces:   1024,
			MaxConnections:  4096,
			IOQueueDepth:    128,
			AdminQueueDepth: 32,
			AllowAnyHost:    false,
		},
		Initiator: InitiatorConfig{
			Enabled:           true,
			ReconnectDelay:    10,
			MaxReconnect:      30,
			IOQueues:          8,
			IOQueueDepth:      128,
			ControllerTimeout: 60,
			KeepAliveTimeout:  30,
		},
		Performance: PerformanceConfig{
			PollMode:       true,
			MaxIOSize:      128,
			InlineDataSize: 4096,
			PageSize:       4,
			ZeroCopy:       true,
		},
		Security: SecurityConfig{
			EnableAuth:  false,
			AllowedIPs:  []string{},
		},
		Monitoring: MonitoringConfig{
			Enabled:         true,
			MetricsInterval: 10,
			EnableEvents:    true,
			IOStats:         true,
			AlertThresholds: AlertThresholds{
				LatencyHigh:    100,
				ErrorRateHigh:  5,
				QueueDepthHigh: 80,
			},
		},
	}
}

// Validate 验证配置
func (c *NVMeOFConfig) Validate() error {
	if !ValidTransports[c.DefaultTransport] {
		c.DefaultTransport = TransportTCP
	}

	if c.Target.DefaultPort <= 0 || c.Target.DefaultPort > 65535 {
		c.Target.DefaultPort = 4420
	}

	if c.Target.MaxSubsystems <= 0 {
		c.Target.MaxSubsystems = 1024
	}

	if c.Target.MaxNamespaces <= 0 {
		c.Target.MaxNamespaces = 1024
	}

	if c.Initiator.IOQueues <= 0 {
		c.Initiator.IOQueues = 8
	}

	if c.Initiator.IOQueueDepth <= 0 {
		c.Initiator.IOQueueDepth = 128
	}

	if c.Initiator.ReconnectDelay <= 0 {
		c.Initiator.ReconnectDelay = 10
	}

	return nil
}

// ========== NVMe-oF Manager ==========

// NVMeOFManager NVMe-oF 管理器
type NVMeOFManager struct {
	mu sync.RWMutex

	// 配置
	config *NVMeOFConfig

	// Target 端
	targetManager *TargetManager

	// Initiator 端
	initiatorManager *InitiatorManager

	// 连接监控
	connectionMonitor *ConnectionMonitor

	// 事件通道
	eventCh chan NVMeOFEvent

	// 运行状态
	running atomic.Bool

	// 可用性
	available bool
}

// NewNVMeOFManager 创建 NVMe-oF 管理器
func NewNVMeOFManager(config *NVMeOFConfig) (*NVMeOFManager, error) {
	if config == nil {
		config = DefaultNVMeOFConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	m := &NVMeOFManager{
		config:  config,
		eventCh: make(chan NVMeOFEvent, 256),
	}

	// 检测 NVMe-oF 可用性
	m.checkAvailability()

	// 初始化 Target 管理器
	if config.Target.Enabled {
		tm, err := NewTargetManager(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create target manager: %w", err)
		}
		m.targetManager = tm
	}

	// 初始化 Initiator 管理器
	if config.Initiator.Enabled {
		im, err := NewInitiatorManager(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create initiator manager: %w", err)
		}
		m.initiatorManager = im
	}

	// 初始化连接监控
	if config.Monitoring.Enabled {
		m.connectionMonitor = NewConnectionMonitor(config)
	}

	return m, nil
}

// checkAvailability 检测 NVMe-oF 可用性
func (m *NVMeOFManager) checkAvailability() {
	// 检查内核模块
	// 实际实现需要检查 /sys/module/nvme_core 和 /sys/module/nvme_fabrics
	// 以及 /sys/module/nvmet (target) 和 /sys/module/nvme_tcp/nvme_rdma
	m.available = true
}

// IsAvailable 检查 NVMe-oF 是否可用
func (m *NVMeOFManager) IsAvailable() bool {
	return m.available
}

// Start 启动管理器
func (m *NVMeOFManager) Start(ctx context.Context) error {
	if !m.running.CompareAndSwap(false, true) {
		return nil
	}

	// 启动 Target 管理器
	if m.targetManager != nil {
		if err := m.targetManager.Start(ctx); err != nil {
			m.running.Store(false)
			return fmt.Errorf("failed to start target manager: %w", err)
		}
	}

	// 启动 Initiator 管理器
	if m.initiatorManager != nil {
		if err := m.initiatorManager.Start(ctx); err != nil {
			if m.targetManager != nil {
				_ = m.targetManager.Stop()
			}
			m.running.Store(false)
			return fmt.Errorf("failed to start initiator manager: %w", err)
		}
	}

	// 启动连接监控
	if m.connectionMonitor != nil {
		m.connectionMonitor.Start(ctx)
	}

	m.eventCh <- NVMeOFEvent{
		Type:    EventManagerStarted,
		Message: "NVMe-oF manager started",
		Time:    time.Now(),
	}

	return nil
}

// Stop 停止管理器
func (m *NVMeOFManager) Stop() error {
	if !m.running.CompareAndSwap(true, false) {
		return nil
	}

	// 停止连接监控
	if m.connectionMonitor != nil {
		m.connectionMonitor.Stop()
	}

	// 停止 Initiator 管理器
	if m.initiatorManager != nil {
		_ = m.initiatorManager.Stop()
	}

	// 停止 Target 管理器
	if m.targetManager != nil {
		_ = m.targetManager.Stop()
	}

	m.eventCh <- NVMeOFEvent{
		Type:    EventManagerStopped,
		Message: "NVMe-oF manager stopped",
		Time:    time.Now(),
	}

	return nil
}

// GetTargetManager 获取 Target 管理器
func (m *NVMeOFManager) GetTargetManager() *TargetManager {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.targetManager
}

// GetInitiatorManager 获取 Initiator 管理器
func (m *NVMeOFManager) GetInitiatorManager() *InitiatorManager {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.initiatorManager
}

// GetConfig 获取配置
func (m *NVMeOFManager) GetConfig() *NVMeOFConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// Events 返回事件通道
func (m *NVMeOFManager) Events() <-chan NVMeOFEvent {
	return m.eventCh
}

// GetStats 获取统计信息
func (m *NVMeOFManager) GetStats() *NVMeOFStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &NVMeOFStats{
		Available: m.available,
		Running:   m.running.Load(),
	}

	if m.targetManager != nil {
		stats.Target = m.targetManager.GetStats()
	}

	if m.initiatorManager != nil {
		stats.Initiator = m.initiatorManager.GetStats()
	}

	return stats
}

// NVMeOFStats NVMe-oF 统计信息
type NVMeOFStats struct {
	Available bool              `json:"available"`
	Running   bool              `json:"running"`
	Target    *TargetStats      `json:"target,omitempty"`
	Initiator *InitiatorStats   `json:"initiator,omitempty"`
}

// ========== 事件定义 ==========

// NVMeOFEventType 事件类型
type NVMeOFEventType int

const (
	// EventManagerStarted 管理器启动
	EventManagerStarted NVMeOFEventType = iota
	// EventManagerStopped 管理器停止
	EventManagerStopped
	// EventSubsystemCreated 子系统创建
	EventSubsystemCreated
	// EventSubsystemDeleted 子系统删除
	EventSubsystemDeleted
	// EventNamespaceCreated 命名空间创建
	EventNamespaceCreated
	// EventNamespaceDeleted 命名空间删除
	EventNamespaceDeleted
	// EventHostConnected 主机连接
	EventHostConnected
	// EventHostDisconnected 主机断开
	EventHostDisconnected
	// EventControllerConnected 控制器连接
	EventControllerConnected
	// EventControllerDisconnected 控制器断开
	EventControllerDisconnected
	// EventConnectionStateChanged 连接状态变化
	EventConnectionStateChanged
	// EventError 错误事件
	EventError
	// EventAlert 告警事件
	EventAlert
)

// NVMeOFEvent NVMe-oF 事件
type NVMeOFEvent struct {
	Type      NVMeOFEventType `json:"type"`
	Message   string          `json:"message"`
	Subsystem string          `json:"subsystem,omitempty"`
	Namespace string          `json:"namespace,omitempty"`
	Host      string          `json:"host,omitempty"`
	Controller string         `json:"controller,omitempty"`
	Error     error           `json:"error,omitempty"`
	Time      time.Time       `json:"time"`
}

// ========== 工具函数 ==========

// CheckNVMeOFAvailable 检查 NVMe-oF 是否可用
func CheckNVMeOFAvailable() (bool, error) {
	// 检查内核模块
	// 实际实现需要检查:
	// - /sys/module/nvme_core
	// - /sys/module/nvme_fabrics
	// - /sys/module/nvme_tcp 或 /sys/module/nvme_rdma
	return true, nil
}

// GetKernelModules 获取已加载的 NVMe 内核模块
func GetKernelModules() ([]string, error) {
	// 实际实现需要读取 /proc/modules 或 /sys/module
	return []string{"nvme_core", "nvme_fabrics", "nvme_tcp"}, nil
}

// LoadKernelModule 加载内核模块
func LoadKernelModule(module string) error {
	// 实际实现需要调用 modprobe
	return nil
}