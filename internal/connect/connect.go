// Package connect 提供 NAS Connect 远程访问服务
// 类似 TrueNAS Connect，支持安全远程访问、内网穿透、WebRTC 直连
package connect

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// 连接状态.
const (
	StatusDisconnected = "disconnected"
	StatusConnecting   = "connecting"
	StatusConnected    = "connected"
	StatusReconnecting = "reconnecting"
	StatusError        = "error"
)

// 连接模式.
const (
	ModeDirect = "direct" // P2P 直连
	ModeRelay  = "relay"  // 中继模式
	ModeAuto   = "auto"   // 自动选择
)

// Errors 错误定义.
var (
	ErrNotConnected      = errors.New("not connected to NAS Connect service")
	ErrAlreadyConnected  = errors.New("already connected")
	ErrInvalidConfig     = errors.New("invalid configuration")
	ErrServiceUnreachable = errors.New("NAS Connect service unreachable")
	ErrAuthFailed        = errors.New("authentication failed")
)

// Config NAS Connect 配置.
type Config struct {
	// 是否启用
	Enabled bool `json:"enabled" yaml:"enabled"`

	// 服务端点
	ServerURL string `json:"server_url" yaml:"server_url"`

	// 设备ID（自动生成或手动指定）
	DeviceID string `json:"device_id" yaml:"device_id"`

	// 设备名称
	DeviceName string `json:"device_name" yaml:"device_name"`

	// 认证令牌
	Token string `json:"token" yaml:"token"`

	// 连接模式
	Mode string `json:"mode" yaml:"mode"`

	// TLS 配置
	TLSEnabled  bool   `json:"tls_enabled" yaml:"tls_enabled"`
	TLSCertFile string `json:"tls_cert_file" yaml:"tls_cert_file"`
	TLSKeyFile  string `json:"tls_key_file" yaml:"tls_key_file"`

	// 本地服务端口
	LocalPort int `json:"local_port" yaml:"local_port"`

	// 心跳间隔
	HeartbeatInterval time.Duration `json:"heartbeat_interval" yaml:"heartbeat_interval"`

	// 重连配置
	ReconnectEnabled bool          `json:"reconnect_enabled" yaml:"reconnect_enabled"`
	ReconnectDelay   time.Duration `json:"reconnect_delay" yaml:"reconnect_delay"`
	MaxReconnectTries int          `json:"max_reconnect_tries" yaml:"max_reconnect_tries"`

	// STUN/TURN 服务器
	STUNServers []string `json:"stun_servers" yaml:"stun_servers"`
	TURNServers []string `json:"turn_servers" yaml:"turn_servers"`

	// 带宽限制 (KB/s, 0 = 不限制)
	MaxBandwidth int `json:"max_bandwidth" yaml:"max_bandwidth"`

	// 访问控制
	AllowedNetworks []string `json:"allowed_networks" yaml:"allowed_networks"`
}

// ServiceInfo 服务信息.
type ServiceInfo struct {
	DeviceID    string    `json:"device_id"`
	DeviceName  string    `json:"device_name"`
	PublicURL   string    `json:"public_url"`
	LocalURL    string    `json:"local_url"`
	Status      string    `json:"status"`
	Mode        string    `json:"mode"`
	ConnectedAt time.Time `json:"connected_at"`
	Uptime      string    `json:"uptime"`
}

// ConnectionStats 连接统计.
type ConnectionStats struct {
	BytesSent     uint64    `json:"bytes_sent"`
	BytesReceived uint64    `json:"bytes_received"`
	Connections   int       `json:"connections"`
	LastActivity  time.Time `json:"last_activity"`
	Latency       int       `json:"latency_ms"`
}

// ConnectService NAS Connect 服务.
type ConnectService struct {
	config     *Config
	status     string
	deviceID   string
	publicURL  string
	localURL   string
	conn       net.Conn
	httpClient *http.Client
	stats      *ConnectionStats
	connectedAt time.Time

	// 回调
	onConnect    func()
	onDisconnect func(error)
	onStatusChange func(string)

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	logger *zap.Logger
}

// NewConnectService 创建 NAS Connect 服务.
func NewConnectService(config *Config, logger *zap.Logger) (*ConnectService, error) {
	if config.ServerURL == "" {
		return nil, ErrInvalidConfig
	}

	if config.DeviceID == "" {
		// 生成设备ID
		hostname, _ := os.Hostname()
		config.DeviceID = fmt.Sprintf("nas-%s-%d", hostname, time.Now().Unix())
	}

	if config.Mode == "" {
		config.Mode = ModeAuto
	}

	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 30 * time.Second
	}

	if config.ReconnectDelay == 0 {
		config.ReconnectDelay = 5 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &ConnectService{
		config: config,
		status: StatusDisconnected,
		stats:  &ConnectionStats{},
		ctx:    ctx,
		cancel: cancel,
		logger: logger,
	}

	// 创建 HTTP 客户端
	s.httpClient = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
	}

	return s, nil
}

// Start 启动服务.
func (s *ConnectService) Start() error {
	if !s.config.Enabled {
		s.logger.Info("NAS Connect is disabled")
		return nil
	}

	s.logger.Info("Starting NAS Connect service",
		zap.String("device_id", s.config.DeviceID),
		zap.String("mode", s.config.Mode),
	)

	// 连接服务
	if err := s.connect(); err != nil {
		if s.config.ReconnectEnabled {
			s.logger.Warn("Initial connection failed, will retry", zap.Error(err))
			s.wg.Add(1)
			go s.reconnectLoop()
		} else {
			return err
		}
	}

	// 启动心跳
	s.wg.Add(1)
	go s.heartbeatLoop()

	return nil
}

// connect 连接到服务.
func (s *ConnectService) connect() error {
	s.mu.Lock()
	s.status = StatusConnecting
	s.mu.Unlock()

	if s.onStatusChange != nil {
		s.onStatusChange(StatusConnecting)
	}

	// 1. 认证
	authResp, err := s.authenticate()
	if err != nil {
		s.mu.Lock()
		s.status = StatusError
		s.mu.Unlock()
		return fmt.Errorf("authentication failed: %w", err)
	}

	s.deviceID = authResp.DeviceID
	s.publicURL = authResp.PublicURL

	// 2. 建立隧道连接
	switch s.config.Mode {
	case ModeDirect:
		if err := s.establishDirectConnection(); err != nil {
			s.logger.Warn("Direct connection failed, falling back to relay", zap.Error(err))
			if err := s.establishRelayConnection(); err != nil {
				return err
			}
		}
	case ModeRelay:
		if err := s.establishRelayConnection(); err != nil {
			return err
		}
	case ModeAuto:
		// 优先尝试直连
		if err := s.establishDirectConnection(); err != nil {
			s.logger.Info("Direct connection unavailable, using relay", zap.Error(err))
			if err := s.establishRelayConnection(); err != nil {
				return err
			}
		}
	}

	s.mu.Lock()
	s.status = StatusConnected
	s.connectedAt = time.Now()
	s.localURL = fmt.Sprintf("http://127.0.0.1:%d", s.config.LocalPort)
	s.mu.Unlock()

	if s.onStatusChange != nil {
		s.onStatusChange(StatusConnected)
	}
	if s.onConnect != nil {
		s.onConnect()
	}

	s.logger.Info("NAS Connect established",
		zap.String("device_id", s.deviceID),
		zap.String("public_url", s.publicURL),
		zap.String("mode", s.config.Mode),
	)

	return nil
}

// AuthResponse 认证响应.
type AuthResponse struct {
	DeviceID  string `json:"device_id"`
	PublicURL string `json:"public_url"`
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

// authenticate 认证.
func (s *ConnectService) authenticate() (*AuthResponse, error) {
	req := map[string]string{
		"device_id":   s.config.DeviceID,
		"device_name": s.config.DeviceName,
		"token":       s.config.Token,
		"version":     "2.0",
	}

	reqBody, _ := json.Marshal(req)
	// TODO: 发送认证请求到服务器
	_ = reqBody

	// 模拟响应
	return &AuthResponse{
		DeviceID:  s.config.DeviceID,
		PublicURL: fmt.Sprintf("https://%s.connect.nas.local", s.config.DeviceID),
		Token:     s.config.Token,
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	}, nil
}

// establishDirectConnection 建立直连.
func (s *ConnectService) establishDirectConnection() error {
	// 使用 STUN 进行 NAT 穿透
	publicAddr, err := s.discoverPublicAddress()
	if err != nil {
		return fmt.Errorf("NAT discovery failed: %w", err)
	}

	s.logger.Debug("Public address discovered",
		zap.String("ip", publicAddr.IP),
		zap.Int("port", publicAddr.Port),
	)

	// TODO: 实现 WebRTC 或类似 P2P 连接

	return nil
}

// PublicAddr 公网地址.
type PublicAddr struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

// discoverPublicAddress 发现公网地址.
func (s *ConnectService) discoverPublicAddress() (*PublicAddr, error) {
	// 使用 STUN 服务器获取公网地址
	for _, stunServer := range s.config.STUNServers {
		addr, err := s.querySTUN(stunServer)
		if err == nil {
			return addr, nil
		}
		s.logger.Debug("STUN query failed", zap.String("server", stunServer), zap.Error(err))
	}

	// 回退到 HTTP 方式
	return s.discoverPublicAddressHTTP()
}

// querySTUN 查询 STUN 服务器.
func (s *ConnectService) querySTUN(server string) (*PublicAddr, error) {
	// TODO: 实现完整的 STUN 协议
	return nil, errors.New("STUN not implemented")
}

// discoverPublicAddressHTTP 通过 HTTP 获取公网地址.
func (s *ConnectService) discoverPublicAddressHTTP() (*PublicAddr, error) {
	resp, err := s.httpClient.Get("https://api.ipify.org?format=json")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &PublicAddr{IP: result.IP, Port: 0}, nil
}

// establishRelayConnection 建立中继连接.
func (s *ConnectService) establishRelayConnection() error {
	// 连接到中继服务器
	relayURL := fmt.Sprintf("%s/relay", s.config.ServerURL)

	// TODO: 实现 WebSocket 或长连接到中继服务器
	_ = relayURL

	s.mu.Lock()
	s.config.Mode = ModeRelay
	s.mu.Unlock()

	return nil
}

// heartbeatLoop 心跳循环.
func (s *ConnectService) heartbeatLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.sendHeartbeat(); err != nil {
				s.logger.Debug("Heartbeat failed", zap.Error(err))
				if s.config.ReconnectEnabled {
					s.reconnect()
				}
			}
		}
	}
}

// sendHeartbeat 发送心跳.
func (s *ConnectService) sendHeartbeat() error {
	// TODO: 发送心跳到服务器
	s.stats.LastActivity = time.Now()
	return nil
}

// reconnectLoop 重连循环.
func (s *ConnectService) reconnectLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.ReconnectDelay)
	defer ticker.Stop()

	attempts := 0

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.mu.RLock()
			status := s.status
			s.mu.RUnlock()

			if status == StatusConnected {
				return
			}

			attempts++
			if s.config.MaxReconnectTries > 0 && attempts > s.config.MaxReconnectTries {
				s.logger.Error("Max reconnect attempts reached")
				return
			}

			s.mu.Lock()
			s.status = StatusReconnecting
			s.mu.Unlock()

			if s.onStatusChange != nil {
				s.onStatusChange(StatusReconnecting)
			}

			if err := s.connect(); err != nil {
				s.logger.Debug("Reconnect attempt failed", zap.Int("attempt", attempts), zap.Error(err))
			} else {
				attempts = 0
			}
		}
	}
}

// reconnect 执行重连.
func (s *ConnectService) reconnect() {
	s.mu.Lock()
	if s.status == StatusReconnecting || s.status == StatusConnecting {
		s.mu.Unlock()
		return
	}
	s.status = StatusReconnecting
	s.mu.Unlock()

	if s.onStatusChange != nil {
		s.onStatusChange(StatusReconnecting)
	}

	if err := s.connect(); err != nil {
		s.logger.Debug("Reconnect failed", zap.Error(err))
	}
}

// Stop 停止服务.
func (s *ConnectService) Stop() error {
	s.cancel()
	s.wg.Wait()

	s.mu.Lock()
	if s.conn != nil {
		_ = s.conn.Close()
	}
	s.status = StatusDisconnected
	s.mu.Unlock()

	s.logger.Info("NAS Connect service stopped")
	return nil
}

// GetStatus 获取状态.
func (s *ConnectService) GetStatus() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// GetInfo 获取服务信息.
func (s *ConnectService) GetInfo() ServiceInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	uptime := ""
	if !s.connectedAt.IsZero() {
		uptime = time.Since(s.connectedAt).Round(time.Second).String()
	}

	return ServiceInfo{
		DeviceID:    s.deviceID,
		DeviceName:  s.config.DeviceName,
		PublicURL:   s.publicURL,
		LocalURL:    s.localURL,
		Status:      s.status,
		Mode:        s.config.Mode,
		ConnectedAt: s.connectedAt,
		Uptime:      uptime,
	}
}

// GetStats 获取统计.
func (s *ConnectService) GetStats() ConnectionStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return *s.stats
}

// SetOnConnect 设置连接回调.
func (s *ConnectService) SetOnConnect(fn func()) {
	s.onConnect = fn
}

// SetOnDisconnect 设置断开回调.
func (s *ConnectService) SetOnDisconnect(fn func(error)) {
	s.onDisconnect = fn
}

// SetOnStatusChange 设置状态变化回调.
func (s *ConnectService) SetOnStatusChange(fn func(string)) {
	s.onStatusChange = fn
}

// LoadTLSCertificate 加载 TLS 证书.
func (s *ConnectService) LoadTLSCertificate() (tls.Certificate, error) {
	if s.config.TLSCertFile == "" || s.config.TLSKeyFile == "" {
		return tls.Certificate{}, errors.New("TLS certificate files not configured")
	}

	return tls.LoadX509KeyPair(s.config.TLSCertFile, s.config.TLSKeyFile)
}

// LoadCAPool 加载 CA 证书池.
func (s *ConnectService) LoadCAPool(caFile string) (*x509.CertPool, error) {
	caData, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caData) {
		return nil, errors.New("failed to parse CA certificate")
	}

	return pool, nil
}

// GenerateDeviceID 生成设备ID.
func GenerateDeviceID() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	data := fmt.Sprintf("%s-%d-%d", hostname, time.Now().UnixNano(), os.Getpid())
	return fmt.Sprintf("nas-%x", data[:16]), nil
}

// SaveConfig 保存配置.
func SaveConfig(config *Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// LoadConfig 加载配置.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}