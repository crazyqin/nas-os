// Package tunnel 提供安全隧道服务
// 支持 TLS 加密、双向认证、访问控制
package tunnel

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// 隧道状态.
const (
	TunnelStateInitializing = "initializing"
	TunnelStateActive       = "active"
	TunnelStateDegraded     = "degraded"
	TunnelStateClosed       = "closed"
)

// 错误定义.
var (
	ErrTunnelClosed      = errors.New("tunnel is closed")
	ErrTunnelBusy        = errors.New("tunnel is busy")
	ErrInvalidCertificate = errors.New("invalid certificate")
	ErrAccessDenied      = errors.New("access denied")
	ErrConnectionRefused = errors.New("connection refused")
)

// TunnelConfig 隧道配置.
type TunnelConfig struct {
	// 隧道ID
	ID string `json:"id" yaml:"id"`

	// 名称
	Name string `json:"name" yaml:"name"`

	// 本地监听地址
	LocalAddr string `json:"local_addr" yaml:"local_addr"`

	// 远程目标地址
	RemoteAddr string `json:"remote_addr" yaml:"remote_addr"`

	// TLS 配置
	TLSEnabled   bool   `json:"tls_enabled" yaml:"tls_enabled"`
	CertFile     string `json:"cert_file" yaml:"cert_file"`
	KeyFile      string `json:"key_file" yaml:"key_file"`
	CAFile       string `json:"ca_file" yaml:"ca_file"`
	ServerName   string `json:"server_name" yaml:"server_name"`

	// 双向认证
	MutualTLS bool `json:"mutual_tls" yaml:"mutual_tls"`

	// 访问控制
	AllowedIPs    []string `json:"allowed_ips" yaml:"allowed_ips"`
	AllowedUsers  []string `json:"allowed_users" yaml:"allowed_users"`

	// 连接限制
	MaxConnections int `json:"max_connections" yaml:"max_connections"`
	ConnectTimeout time.Duration `json:"connect_timeout" yaml:"connect_timeout"`
	ReadTimeout    time.Duration `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout   time.Duration `json:"write_timeout" yaml:"write_timeout"`

	// 心跳配置
	HeartbeatInterval time.Duration `json:"heartbeat_interval" yaml:"heartbeat_interval"`
	HeartbeatTimeout  time.Duration `json:"heartbeat_timeout" yaml:"heartbeat_timeout"`

	// 带宽限制 (bytes/s, 0 = 不限制)
	MaxBandwidth int64 `json:"max_bandwidth" yaml:"max_bandwidth"`
}

// TunnelStats 隧道统计.
type TunnelStats struct {
	Connections     uint64 `json:"connections"`
	ActiveConns     uint32 `json:"active_conns"`
	BytesSent       uint64 `json:"bytes_sent"`
	BytesReceived   uint64 `json:"bytes_received"`
	Errors          uint64 `json:"errors"`
	LastActivity    int64  `json:"last_activity"` // Unix timestamp
	Uptime          int64  `json:"uptime"` // seconds
}

// Tunnel 隧道实例.
type Tunnel struct {
	config     *TunnelConfig
	state      string
	listener   net.Listener
	stats      TunnelStats
	startTime  time.Time

	tlsConfig  *tls.Config
	ipFilter   *IPFilter

	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	logger     *zap.Logger
}

// NewTunnel 创建隧道.
func NewTunnel(config *TunnelConfig, logger *zap.Logger) (*Tunnel, error) {
	if config.ID == "" {
		config.ID = fmt.Sprintf("tunnel-%d", time.Now().UnixNano())
	}
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = 30 * time.Second
	}
	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 30 * time.Second
	}
	if config.HeartbeatTimeout == 0 {
		config.HeartbeatTimeout = 60 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	t := &Tunnel{
		config:    config,
		state:     TunnelStateInitializing,
		ctx:       ctx,
		cancel:    cancel,
		logger:    logger,
		startTime: time.Now(),
	}

	// 初始化 TLS
	if config.TLSEnabled {
		if err := t.initTLS(); err != nil {
			cancel()
			return nil, fmt.Errorf("init TLS failed: %w", err)
		}
	}

	// 初始化 IP 过滤器
	t.ipFilter = NewIPFilter(config.AllowedIPs)

	return t, nil
}

// initTLS 初始化 TLS 配置.
func (t *Tunnel) initTLS() error {
	cfg := &tls.Config{
		ServerName: t.config.ServerName,
		MinVersion: tls.VersionTLS12,
	}

	// 加载证书
	if t.config.CertFile != "" && t.config.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(t.config.CertFile, t.config.KeyFile)
		if err != nil {
			return fmt.Errorf("load certificate: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}

	// 加载 CA
	if t.config.CAFile != "" {
		caData, err := os.ReadFile(t.config.CAFile)
		if err != nil {
			return fmt.Errorf("read CA file: %w", err)
		}

		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caData) {
			return errors.New("failed to parse CA certificate")
		}

		cfg.RootCAs = pool
		if t.config.MutualTLS {
			cfg.ClientCAs = pool
			cfg.ClientAuth = tls.RequireAndVerifyClientCert
		}
	}

	t.tlsConfig = cfg
	return nil
}

// Start 启动隧道.
func (t *Tunnel) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state == TunnelStateActive {
		return errors.New("tunnel already active")
	}

	// 创建监听器
	var listener net.Listener
	var err error

	if t.config.TLSEnabled && t.tlsConfig != nil {
		listener, err = tls.Listen("tcp", t.config.LocalAddr, t.tlsConfig)
	} else {
		listener, err = net.Listen("tcp", t.config.LocalAddr)
	}

	if err != nil {
		return fmt.Errorf("listen failed: %w", err)
	}

	t.listener = listener
	t.state = TunnelStateActive

	// 启动接受连接协程
	t.wg.Add(1)
	go t.acceptLoop()

	t.logger.Info("Tunnel started",
		zap.String("id", t.config.ID),
		zap.String("local", t.config.LocalAddr),
		zap.String("remote", t.config.RemoteAddr),
		zap.Bool("tls", t.config.TLSEnabled),
	)

	return nil
}

// acceptLoop 接受连接循环.
func (t *Tunnel) acceptLoop() {
	defer t.wg.Done()

	for {
		select {
		case <-t.ctx.Done():
			return
		default:
		}

		conn, err := t.listener.Accept()
		if err != nil {
			select {
			case <-t.ctx.Done():
				return
			default:
				t.logger.Debug("Accept error", zap.Error(err))
				continue
			}
		}

		// 检查连接数限制
		if t.config.MaxConnections > 0 && atomic.LoadUint32(&t.stats.ActiveConns) >= uint32(t.config.MaxConnections) {
			t.logger.Warn("Connection limit reached, rejecting", zap.String("remote", conn.RemoteAddr().String()))
			_ = conn.Close()
			atomic.AddUint64(&t.stats.Errors, 1)
			continue
		}

		// 检查 IP 过滤
		remoteAddr := conn.RemoteAddr().String()
		if !t.ipFilter.Allow(remoteAddr) {
			t.logger.Debug("IP not allowed", zap.String("ip", remoteAddr))
			_ = conn.Close()
			continue
		}

		// 处理连接
		t.wg.Add(1)
		go t.handleConnection(conn)
	}
}

// handleConnection 处理单个连接.
func (t *Tunnel) handleConnection(conn net.Conn) {
	defer t.wg.Done()
	defer func() {
		_ = conn.Close()
		atomic.AddUint32(&t.stats.ActiveConns, ^uint32(0)) // -1
	}()

	atomic.AddUint64(&t.stats.Connections, 1)
	atomic.AddUint32(&t.stats.ActiveConns, 1)

	// TLS 握手超时
	if t.config.TLSEnabled {
		if tlsConn, ok := conn.(*tls.Conn); ok {
			if err := tlsConn.SetDeadline(time.Now().Add(t.config.ConnectTimeout)); err != nil {
				t.logger.Debug("Failed to set deadline", zap.Error(err))
			}
			if err := tlsConn.Handshake(); err != nil {
				t.logger.Debug("TLS handshake failed", zap.Error(err))
				atomic.AddUint64(&t.stats.Errors, 1)
				return
			}
			if err := tlsConn.SetDeadline(time.Time{}); err != nil {
				t.logger.Debug("Failed to reset deadline", zap.Error(err))
			}
		}
	}

	// 连接远程目标
	remoteConn, err := net.DialTimeout("tcp", t.config.RemoteAddr, t.config.ConnectTimeout)
	if err != nil {
		t.logger.Debug("Failed to connect remote", zap.Error(err), zap.String("remote", t.config.RemoteAddr))
		atomic.AddUint64(&t.stats.Errors, 1)
		return
	}
	defer func() { _ = remoteConn.Close() }()

	t.logger.Debug("Tunnel connection established",
		zap.String("client", conn.RemoteAddr().String()),
		zap.String("remote", t.config.RemoteAddr),
	)

	// 双向数据转发
	done := make(chan struct{}, 2)

	// 本地 -> 远程
	go func() {
		defer func() { done <- struct{}{} }()
		n, _ := io.Copy(remoteConn, conn)
		atomic.AddUint64(&t.stats.BytesSent, uint64(n))
	}()

	// 远程 -> 本地
	go func() {
		defer func() { done <- struct{}{} }()
		n, _ := io.Copy(conn, remoteConn)
		atomic.AddUint64(&t.stats.BytesReceived, uint64(n))
	}()

	// 等待任一方向完成
	<-done
	atomic.StoreInt64(&t.stats.LastActivity, time.Now().Unix())
}

// Stop 停止隧道.
func (t *Tunnel) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state == TunnelStateClosed {
		return nil
	}

	t.cancel()

	if t.listener != nil {
		_ = t.listener.Close()
	}

	t.wg.Wait()
	t.state = TunnelStateClosed

	t.logger.Info("Tunnel stopped", zap.String("id", t.config.ID))
	return nil
}

// GetState 获取状态.
func (t *Tunnel) GetState() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

// GetStats 获取统计.
func (t *Tunnel) GetStats() TunnelStats {
	stats := TunnelStats{
		Connections:   atomic.LoadUint64(&t.stats.Connections),
		ActiveConns:   atomic.LoadUint32(&t.stats.ActiveConns),
		BytesSent:     atomic.LoadUint64(&t.stats.BytesSent),
		BytesReceived: atomic.LoadUint64(&t.stats.BytesReceived),
		Errors:        atomic.LoadUint64(&t.stats.Errors),
		LastActivity:  atomic.LoadInt64(&t.stats.LastActivity),
	}

	if !t.startTime.IsZero() {
		stats.Uptime = int64(time.Since(t.startTime).Seconds())
	}

	return stats
}

// GetConfig 获取配置.
func (t *Tunnel) GetConfig() TunnelConfig {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return *t.config
}

// ID 获取隧道ID.
func (t *Tunnel) ID() string {
	return t.config.ID
}

// Name 获取隧道名称.
func (t *Tunnel) Name() string {
	return t.config.Name
}

// IPFilter IP 过滤器.
type IPFilter struct {
	allowed []*net.IPNet
}

// NewIPFilter 创建 IP 过滤器.
func NewIPFilter(allowedIPs []string) *IPFilter {
	f := &IPFilter{}
	for _, ip := range allowedIPs {
		_, network, err := net.ParseCIDR(ip)
		if err != nil {
			// 单个 IP
			_, network, _ = net.ParseCIDR(ip + "/32")
		}
		if network != nil {
			f.allowed = append(f.allowed, network)
		}
	}
	return f
}

// Allow 检查是否允许.
func (f *IPFilter) Allow(addr string) bool {
	if len(f.allowed) == 0 {
		return true
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	for _, network := range f.allowed {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// Manager 隧道管理器.
type Manager struct {
	tunnels map[string]*Tunnel
	mu      sync.RWMutex
	logger  *zap.Logger
}

// NewManager 创建管理器.
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		tunnels: make(map[string]*Tunnel),
		logger:  logger,
	}
}

// CreateTunnel 创建隧道.
func (m *Manager) CreateTunnel(config *TunnelConfig) (*Tunnel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tunnels[config.ID]; exists {
		return nil, errors.New("tunnel already exists")
	}

	tunnel, err := NewTunnel(config, m.logger)
	if err != nil {
		return nil, err
	}

	m.tunnels[config.ID] = tunnel
	return tunnel, nil
}

// StartTunnel 启动隧道.
func (m *Manager) StartTunnel(id string) error {
	m.mu.RLock()
	tunnel, exists := m.tunnels[id]
	m.mu.RUnlock()

	if !exists {
		return errors.New("tunnel not found")
	}

	return tunnel.Start()
}

// StopTunnel 停止隧道.
func (m *Manager) StopTunnel(id string) error {
	m.mu.RLock()
	tunnel, exists := m.tunnels[id]
	m.mu.RUnlock()

	if !exists {
		return errors.New("tunnel not found")
	}

	return tunnel.Stop()
}

// RemoveTunnel 移除隧道.
func (m *Manager) RemoveTunnel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnel, exists := m.tunnels[id]
	if !exists {
		return errors.New("tunnel not found")
	}

	_ = tunnel.Stop()
	delete(m.tunnels, id)

	return nil
}

// GetTunnel 获取隧道.
func (m *Manager) GetTunnel(id string) (*Tunnel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tunnel, exists := m.tunnels[id]
	return tunnel, exists
}

// ListTunnels 列出所有隧道.
func (m *Manager) ListTunnels() []*Tunnel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tunnels := make([]*Tunnel, 0, len(m.tunnels))
	for _, t := range m.tunnels {
		tunnels = append(tunnels, t)
	}
	return tunnels
}

// StopAll 停止所有隧道.
func (m *Manager) StopAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, t := range m.tunnels {
		_ = t.Stop()
	}
}

// TunnelInfo 隧道信息.
type TunnelInfo struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	LocalAddr   string      `json:"local_addr"`
	RemoteAddr  string      `json:"remote_addr"`
	TLSEnabled  bool        `json:"tls_enabled"`
	MutualTLS   bool        `json:"mutual_tls"`
	State       string      `json:"state"`
	Stats       TunnelStats `json:"stats"`
}

// GetTunnelInfo 获取隧道信息.
func (t *Tunnel) GetTunnelInfo() TunnelInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return TunnelInfo{
		ID:         t.config.ID,
		Name:       t.config.Name,
		LocalAddr:  t.config.LocalAddr,
		RemoteAddr: t.config.RemoteAddr,
		TLSEnabled: t.config.TLSEnabled,
		MutualTLS:  t.config.MutualTLS,
		State:      t.state,
		Stats:      t.GetStats(),
	}
}

// 隧道协议常量.
const (
	ProtocolVersion = 1
	MsgTypeData     = 0x01
	MsgTypeHeartbeat = 0x02
	MsgTypeClose    = 0x03
	MsgTypeAuth     = 0x04
)

// TunnelMessage 隧道消息.
type TunnelMessage struct {
	Type    byte
	Length  uint32
	Payload []byte
}

// Encode 编码消息.
func (m *TunnelMessage) Encode() []byte {
	buf := make([]byte, 5+len(m.Payload))
	buf[0] = m.Type
	binary.BigEndian.PutUint32(buf[1:5], m.Length)
	copy(buf[5:], m.Payload)
	return buf
}

// DecodeMessage 解码消息.
func DecodeMessage(r io.Reader) (*TunnelMessage, error) {
	header := make([]byte, 5)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}

	msg := &TunnelMessage{
		Type:   header[0],
		Length: binary.BigEndian.Uint32(header[1:5]),
	}

	if msg.Length > 0 {
		msg.Payload = make([]byte, msg.Length)
		if _, err := io.ReadFull(r, msg.Payload); err != nil {
			return nil, err
		}
	}

	return msg, nil
}