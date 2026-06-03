// Package bluetoothprovision 实现蓝牙配网功能
// 参考飞牛fnOS的蓝牙配网功能，支持BLE设备扫描发现、WiFi凭证安全传输、
// 配网状态实时推送、多网络管理、配网历史记录
package bluetoothprovision

import (
	"sync"
	"time"
)

// BLEDevice 表示BLE设备信息
type BLEDevice struct {
	ID          string            `json:"id"`           // 设备唯一标识
	Name        string            `json:"name"`         // 设备名称
	Address     string            `json:"address"`      // MAC地址
	RSSI        int               `json:"rssi"`         // 信号强度
	Connected   bool              `json:"connected"`    // 是否已连接
	Services    []string          `json:"services"`     // 支持的服务UUID
	Manufacturer string           `json:"manufacturer"` // 制造商
	Model       string            `json:"model"`        // 型号
	FirmwareVer string            `json:"firmware_ver"` // 固件版本
	Metadata    map[string]string `json:"metadata"`     // 扩展元数据
	Discovered  time.Time         `json:"discovered"`   // 发现时间
	LastSeen    time.Time         `json:"last_seen"`    // 最后可见时间
}

// WiFiConfig 表示WiFi配置信息
type WiFiConfig struct {
	SSID     string `json:"ssid" validate:"required"`           // WiFi名称
	Password string `json:"password" validate:"required"`       // WiFi密码
	Security string `json:"security"`                           // 加密方式: WPA2, WPA3, WEP, OPEN
	Band     string `json:"band"`                               // 频段: 2.4GHz, 5GHz, auto
	Hidden   bool   `json:"hidden"`                             // 是否隐藏网络
	Priority int    `json:"priority"`                           // 优先级 (1-10)
	StaticIP *IPConfig `json:"static_ip,omitempty"`             // 静态IP配置
}

// IPConfig 表示静态IP配置
type IPConfig struct {
	IP      string `json:"ip" validate:"required,ip"`        // IP地址
	Netmask string `json:"netmask" validate:"required"`      // 子网掩码
	Gateway string `json:"gateway" validate:"required,ip"`   // 网关
	DNS     []string `json:"dns"`                             // DNS服务器
}

// ProvisionSession 表示配网会话
type ProvisionSession struct {
	ID          string            `json:"id"`           // 会话ID
	DeviceID    string            `json:"device_id"`    // 目标设备ID
	DeviceName  string            `json:"device_name"`  // 目标设备名称
	WiFiConfig  WiFiConfig        `json:"wifi_config"`  // WiFi配置
	Status      ProvisionStatus   `json:"status"`       // 配网状态
	Progress    int               `json:"progress"`     // 进度百分比 (0-100)
	Error       string            `json:"error"`        // 错误信息
	StartTime   time.Time         `json:"start_time"`   // 开始时间
	EndTime     *time.Time        `json:"end_time"`     // 结束时间
	Steps       []ProvisionStep   `json:"steps"`        // 配网步骤
	NetworkInfo *NetworkInfo      `json:"network_info"` // 配网成功后的网络信息
}

// ProvisionStatus 表示配网状态
type ProvisionStatus string

const (
	StatusPending    ProvisionStatus = "pending"    // 等待中
	StatusConnecting ProvisionStatus = "connecting" // 连接设备中
	StatusAuth       ProvisionStatus = "auth"       // 认证中
	StatusSending    ProvisionStatus = "sending"    // 发送配置中
	StatusVerifying  ProvisionStatus = "verifying"  // 验证中
	StatusSuccess    ProvisionStatus = "success"    // 成功
	StatusFailed     ProvisionStatus = "failed"     // 失败
	StatusTimeout    ProvisionStatus = "timeout"    // 超时
	StatusCancelled  ProvisionStatus = "cancelled"  // 已取消
)

// ProvisionStep 表示配网步骤
type ProvisionStep struct {
	Name      string    `json:"name"`       // 步骤名称
	Status    string    `json:"status"`     // 步骤状态: pending, running, success, failed
	StartTime time.Time `json:"start_time"` // 开始时间
	EndTime   time.Time `json:"end_time"`   // 结束时间
	Error     string    `json:"error"`      // 错误信息
}

// NetworkInfo 表示网络连接信息
type NetworkInfo struct {
	SSID       string `json:"ssid"`        // WiFi名称
	IP         string `json:"ip"`          // 获取的IP地址
	MAC        string `json:"mac"`         // 设备MAC地址
	Gateway    string `json:"gateway"`     // 网关
	DNS        string `json:"dns"`         // DNS
	Signal     int    `json:"signal"`      // 信号强度
	Speed      int    `json:"speed"`       // 连接速度 Mbps
	Connected  bool   `json:"connected"`   // 是否已连接
	ConnectedAt time.Time `json:"connected_at"` // 连接时间
}

// ProvisionHistory 表示配网历史记录
type ProvisionHistory struct {
	ID         string          `json:"id"`          // 记录ID
	DeviceName string          `json:"device_name"` // 设备名称
	DeviceMAC  string          `json:"device_mac"`  // 设备MAC
	SSID       string          `json:"ssid"`        // WiFi名称
	Status     ProvisionStatus `json:"status"`      // 配网结果
	Error      string          `json:"error"`       // 错误信息
	StartTime  time.Time       `json:"start_time"`  // 开始时间
	Duration   time.Duration   `json:"duration"`    // 耗时
}

// ScanRequest 表示扫描请求
type ScanRequest struct {
	Duration   int      `json:"duration"`    // 扫描时长(秒), 默认10秒
	Filter     []string `json:"filter"`      // 服务UUID过滤
	MinRSSI    int      `json:"min_rssi"`    // 最小信号强度
	MaxDevices int      `json:"max_devices"` // 最大设备数
}

// ProvisionRequest 表示配网请求
type ProvisionRequest struct {
	DeviceID   string     `json:"device_id" validate:"required"` // 目标设备ID
	WiFiConfig WiFiConfig `json:"wifi_config" validate:"required"` // WiFi配置
	Timeout    int        `json:"timeout"`                       // 超时时间(秒), 默认60秒
	RetryCount int        `json:"retry_count"`                   // 重试次数, 默认3次
}

// Scanner 定义BLE扫描器接口
type Scanner interface {
	Scan(req ScanRequest) ([]BLEDevice, error)
	StopScan() error
	Connect(deviceID string) error
	Disconnect(deviceID string) error
	IsScanning() bool
}

// Provisioner 定义配网引擎接口
type Provisioner interface {
	StartProvision(req ProvisionRequest) (*ProvisionSession, error)
	CancelProvision(sessionID string) error
	GetSession(sessionID string) (*ProvisionSession, error)
	GetHistory(limit int) ([]ProvisionHistory, error)
	ClearHistory() error
}

// Manager 管理蓝牙配网模块的核心状态
type Manager struct {
	mu            sync.RWMutex
	scanner       Scanner
	provisioner   Provisioner
	devices       map[string]*BLEDevice      // 已发现设备
	sessions      map[string]*ProvisionSession // 活跃会话
	history       []ProvisionHistory          // 历史记录
	networks      []WiFiConfig               // 已保存的网络配置
	subscribers   map[string]chan ProvisionEvent // 事件订阅者
	maxHistory    int                         // 最大历史记录数
	scanTimeout   time.Duration               // 扫描超时
	provisionTimeout time.Duration            // 配网超时
}

// ProvisionEvent 表示配网事件
type ProvisionEvent struct {
	Type      string          `json:"type"`       // 事件类型: device_found, status_change, progress, error, complete
	SessionID string          `json:"session_id"` // 会话ID
	DeviceID  string          `json:"device_id"`  // 设备ID
	Status    ProvisionStatus `json:"status"`     // 配网状态
	Progress  int             `json:"progress"`   // 进度
	Message   string          `json:"message"`    // 消息内容
	Timestamp time.Time       `json:"timestamp"`  // 时间戳
}

// NewManager 创建新的蓝牙配网管理器
func NewManager(opts ...Option) *Manager {
	m := &Manager{
		devices:          make(map[string]*BLEDevice),
		sessions:         make(map[string]*ProvisionSession),
		history:          make([]ProvisionHistory, 0),
		networks:         make([]WiFiConfig, 0),
		subscribers:      make(map[string]chan ProvisionEvent),
		maxHistory:       100,
		scanTimeout:      30 * time.Second,
		provisionTimeout: 60 * time.Second,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Option 定义管理器配置选项
type Option func(*Manager)

// WithMaxHistory 设置最大历史记录数
func WithMaxHistory(n int) Option {
	return func(m *Manager) {
		m.maxHistory = n
	}
}

// WithScanTimeout 设置扫描超时时间
func WithScanTimeout(d time.Duration) Option {
	return func(m *Manager) {
		m.scanTimeout = d
	}
}

// WithProvisionTimeout 设置配网超时时间
func WithProvisionTimeout(d time.Duration) Option {
	return func(m *Manager) {
		m.provisionTimeout = d
	}
}

// WithScanner 设置自定义扫描器
func WithScanner(s Scanner) Option {
	return func(m *Manager) {
		m.scanner = s
	}
}

// WithProvisioner 设置自定义配网器
func WithProvisioner(p Provisioner) Option {
	return func(m *Manager) {
		m.provisioner = p
	}
}

// Subscribe 订阅配网事件
func (m *Manager) Subscribe(id string) <-chan ProvisionEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan ProvisionEvent, 100)
	m.subscribers[id] = ch
	return ch
}

// Unsubscribe 取消订阅
func (m *Manager) Unsubscribe(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ch, ok := m.subscribers[id]; ok {
		close(ch)
		delete(m.subscribers, id)
	}
}

// publish 发布事件给所有订阅者
func (m *Manager) publish(event ProvisionEvent) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, ch := range m.subscribers {
		select {
		case ch <- event:
		default:
			// 队列满则丢弃，避免阻塞
		}
	}
}
