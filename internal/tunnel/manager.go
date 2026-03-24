// Package tunnel 提供内网穿透服务管理器
package tunnel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

var (
	// ErrTunnelNotFound 隧道不存在
	ErrTunnelNotFound = errors.New("tunnel not found")
	// ErrTunnelAlreadyExists 隧道已存在
	ErrTunnelAlreadyExists = errors.New("tunnel already exists")
	// ErrInvalidConfig 无效配置
	ErrInvalidConfig = errors.New("invalid configuration")
	// ErrConnectionFailed 连接失败
	ErrConnectionFailed = errors.New("connection failed")
	// ErrNotConnected 未连接
	ErrNotConnected = errors.New("not connected")
	// ErrTimeout 超时
	ErrTimeout = errors.New("operation timeout")
)

// Manager 隧道管理器
type Manager struct {
	config Config
	logger *zap.Logger
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 隧道管理
	tunnels    map[string]*Tunnel
	tunnelByID map[string]*Tunnel

	// 状态信息
	state      TunnelState
	natType    NATType
	publicIP   string
	publicPort int
	startTime  time.Time

	// NAT 检测
	detector NATDetector

	// 事件回调
	eventCallbacks []EventCallback

	// 统计信息
	totalBytesTx int64
	totalBytesRx int64
}

// Tunnel 单个隧道实例
type Tunnel struct {
	config      TunnelConfig
	status      TunnelStatus
	client      TunnelClient
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.RWMutex
	connections map[string]*Connection
}

// NewManager 创建隧道管理器
func NewManager(config Config, logger *zap.Logger) (*Manager, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	// 验证配置
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	// 设置默认值
	setDefaults(&config)

	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		config:     config,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
		tunnels:    make(map[string]*Tunnel),
		tunnelByID: make(map[string]*Tunnel),
		state:      StateDisconnected,
		natType:    NATTypeUnknown,
		startTime:  time.Now(),
	}

	// 初始化 NAT 检测器
	m.detector = NewSTUNDetector(config.STUNServers, logger)

	return m, nil
}

// validateConfig 验证配置
func validateConfig(config *Config) error {
	if config.ServerAddr == "" {
		return errors.New("server address is required")
	}
	if config.ServerPort <= 0 || config.ServerPort > 65535 {
		return errors.New("invalid server port")
	}
	if config.HeartbeatInt <= 0 {
		config.HeartbeatInt = 30
	}
	if config.ReconnectInt <= 0 {
		config.ReconnectInt = 5
	}
	if config.MaxReconnect <= 0 {
		config.MaxReconnect = 10
	}
	if config.Timeout <= 0 {
		config.Timeout = 30
	}
	return nil
}

// setDefaults 设置默认值
func setDefaults(config *Config) {
	if config.Mode == "" {
		config.Mode = ModeAuto
	}
	if len(config.STUNServers) == 0 {
		config.STUNServers = []string{
			"stun:stun.l.google.com:19302",
			"stun:stun1.l.google.com:19302",
		}
	}
}

// Start 启动隧道管理器
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("starting tunnel manager",
		zap.String("server", m.config.ServerAddr),
		zap.String("mode", string(m.config.Mode)),
	)

	// 检测 NAT 类型
	go m.detectNAT(ctx)

	// 启动心跳
	m.wg.Add(1)
	go m.heartbeatLoop()

	m.state = StateConnecting

	m.logger.Info("tunnel manager started")
	return nil
}

// Stop 停止隧道管理器
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("stopping tunnel manager")

	// 关闭所有隧道
	for _, tunnel := range m.tunnels {
		if err := m.disconnectTunnel(tunnel); err != nil {
			m.logger.Error("failed to disconnect tunnel",
				zap.String("id", tunnel.config.ID),
				zap.Error(err),
			)
		}
	}

	m.cancel()
	m.wg.Wait()

	m.state = StateDisconnected
	m.logger.Info("tunnel manager stopped")
	return nil
}

// Connect 建立新隧道
func (m *Manager) Connect(ctx context.Context, req ConnectRequest) (*ConnectResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已存在同名隧道
	if _, exists := m.tunnels[req.Name]; exists {
		return nil, ErrTunnelAlreadyExists
	}

	// 生成隧道 ID
	tunnelID := generateID()

	// 确定连接模式
	mode := req.Mode
	if mode == "" {
		mode = m.config.Mode
	}

	// 创建隧道配置
	config := TunnelConfig{
		ID:          tunnelID,
		Name:        req.Name,
		Mode:        mode,
		LocalAddr:   fmt.Sprintf("127.0.0.1:%d", req.LocalPort),
		RemotePort:  req.RemotePort,
		Protocol:    req.Protocol,
		Description: req.Description,
		Enabled:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if config.Protocol == "" {
		config.Protocol = "tcp"
	}

	// 创建隧道实例
	tunnelCtx, tunnelCancel := context.WithCancel(m.ctx)
	tunnel := &Tunnel{
		config:      config,
		connections: make(map[string]*Connection),
		ctx:         tunnelCtx,
		cancel:      tunnelCancel,
		status: TunnelStatus{
			ID:        tunnelID,
			Name:      req.Name,
			Mode:      mode,
			State:     StateConnecting,
			LocalAddr: config.LocalAddr,
			NATType:   m.natType,
		},
	}

	// 根据模式创建客户端
	switch mode {
	case ModeP2P:
		tunnel.client = NewP2PClient(config, m.config, m.logger)
	case ModeRelay:
		tunnel.client = NewRelayClient(config, m.config, m.logger)
	case ModeReverse:
		tunnel.client = NewReverseClient(config, m.config, m.logger)
	case ModeAuto:
		// 自动模式：先尝试 P2P，失败则切换到 Relay
		tunnel.client = NewAutoClient(config, m.config, m.logger)
	}

	// 启动隧道连接
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		if err := tunnel.client.Connect(ctx); err != nil {
			m.logger.Error("tunnel connection failed",
				zap.String("id", tunnelID),
				zap.Error(err),
			)
			tunnel.mu.Lock()
			tunnel.status.State = StateError
			tunnel.status.LastError = err.Error()
			tunnel.mu.Unlock()

			m.emitEvent(Event{
				Type:     EventTunnelError,
				TunnelID: tunnelID,
				Error:    err.Error(),
			})
			return
		}

		tunnel.mu.Lock()
		tunnel.status.State = StateConnected
		tunnel.status.LastConnected = time.Now()
		tunnel.mu.Unlock()

		m.emitEvent(Event{
			Type:     EventTunnelConnected,
			TunnelID: tunnelID,
		})
	}()

	// 保存隧道
	m.tunnels[req.Name] = tunnel
	m.tunnelByID[tunnelID] = tunnel

	// 发送事件
	m.emitEvent(Event{
		Type:      EventTunnelCreated,
		TunnelID:  tunnelID,
		Timestamp: time.Now(),
		Data:      config,
	})

	return &ConnectResponse{
		TunnelID:   tunnelID,
		Name:       req.Name,
		Mode:       mode,
		State:      StateConnecting,
		LocalAddr:  config.LocalAddr,
		PublicAddr: m.getPublicAddr(),
		Message:    "Tunnel is connecting",
	}, nil
}

// Disconnect 断开隧道
func (m *Manager) Disconnect(tunnelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnel, exists := m.tunnelByID[tunnelID]
	if !exists {
		return ErrTunnelNotFound
	}

	return m.disconnectTunnel(tunnel)
}

// disconnectTunnel 内部断开隧道方法（需要已持有锁）
func (m *Manager) disconnectTunnel(tunnel *Tunnel) error {
	if tunnel.client != nil {
		if err := tunnel.client.Disconnect(); err != nil {
			m.logger.Error("failed to disconnect tunnel client",
				zap.String("id", tunnel.config.ID),
				zap.Error(err),
			)
		}
	}

	tunnel.cancel()
	tunnel.mu.Lock()
	tunnel.status.State = StateDisconnected
	tunnel.mu.Unlock()

	delete(m.tunnels, tunnel.config.Name)
	delete(m.tunnelByID, tunnel.config.ID)

	m.emitEvent(Event{
		Type:      EventTunnelDisconnected,
		TunnelID:  tunnel.config.ID,
		Timestamp: time.Now(),
	})

	return nil
}

// GetStatus 获取管理器状态
func (m *Manager) GetStatus() ManagerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tunnels := make([]TunnelStatus, 0, len(m.tunnels))
	activeTunnels := 0
	for _, t := range m.tunnels {
		t.mu.RLock()
		status := t.status
		t.mu.RUnlock()
		tunnels = append(tunnels, status)
		if status.State == StateConnected {
			activeTunnels++
		}
	}

	return ManagerStatus{
		State:         m.state,
		NATType:       m.natType,
		PublicIP:      m.publicIP,
		PublicPort:    m.publicPort,
		Tunnels:       tunnels,
		ActiveTunnels: activeTunnels,
		TotalBytesTx:  m.totalBytesTx,
		TotalBytesRx:  m.totalBytesRx,
		StartTime:     m.startTime,
	}
}

// GetTunnelStatus 获取单个隧道状态
func (m *Manager) GetTunnelStatus(tunnelID string) (TunnelStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tunnel, exists := m.tunnelByID[tunnelID]
	if !exists {
		return TunnelStatus{}, ErrTunnelNotFound
	}

	tunnel.mu.RLock()
	defer tunnel.mu.RUnlock()
	return tunnel.status, nil
}

// ListTunnels 列出所有隧道
func (m *Manager) ListTunnels() []TunnelStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tunnels := make([]TunnelStatus, 0, len(m.tunnels))
	for _, t := range m.tunnels {
		t.mu.RLock()
		status := t.status
		t.mu.RUnlock()
		tunnels = append(tunnels, status)
	}
	return tunnels
}

// OnEvent 注册事件回调
func (m *Manager) OnEvent(callback EventCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventCallbacks = append(m.eventCallbacks, callback)
}

// emitEvent 发送事件
func (m *Manager) emitEvent(event Event) {
	event.Timestamp = time.Now()

	m.mu.RLock()
	callbacks := make([]EventCallback, len(m.eventCallbacks))
	copy(callbacks, m.eventCallbacks)
	m.mu.RUnlock()

	for _, cb := range callbacks {
		go cb(event)
	}
}

// detectNAT 检测 NAT 类型
func (m *Manager) detectNAT(ctx context.Context) {
	natType, publicIP, publicPort, err := m.detector.Detect(ctx)
	if err != nil {
		m.logger.Warn("NAT detection failed", zap.Error(err))
		return
	}

	m.mu.Lock()
	m.natType = natType
	m.publicIP = publicIP
	m.publicPort = publicPort
	m.mu.Unlock()

	m.logger.Info("NAT detected",
		zap.String("type", string(natType)),
		zap.String("public_ip", publicIP),
		zap.Int("public_port", publicPort),
	)

	m.emitEvent(Event{
		Type: EventNATDetected,
		Data: map[string]interface{}{
			"nat_type":    natType,
			"public_ip":   publicIP,
			"public_port": publicPort,
		},
	})
}

// heartbeatLoop 心跳循环
func (m *Manager) heartbeatLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(time.Duration(m.config.HeartbeatInt) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.sendHeartbeat()
		}
	}
}

// sendHeartbeat 发送心跳
func (m *Manager) sendHeartbeat() {
	m.mu.RLock()
	tunnels := make([]*Tunnel, 0, len(m.tunnels))
	for _, t := range m.tunnels {
		tunnels = append(tunnels, t)
	}
	m.mu.RUnlock()

	for _, tunnel := range tunnels {
		if tunnel.client != nil && tunnel.client.IsConnected() {
			// 发送心跳包
			_, err := tunnel.client.Send([]byte("PING"))
			if err != nil {
				m.logger.Debug("heartbeat failed",
					zap.String("tunnel_id", tunnel.config.ID),
					zap.Error(err),
				)
			}
		}
	}
}

// getPublicAddr 获取公网地址
func (m *Manager) getPublicAddr() string {
	if m.publicIP != "" && m.publicPort > 0 {
		return fmt.Sprintf("%s:%d", m.publicIP, m.publicPort)
	}
	return ""
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// STUNDetector STUN NAT 检测器实现
type STUNDetector struct {
	servers []string
	logger  *zap.Logger
}

// NewSTUNDetector 创建 STUN 检测器
func NewSTUNDetector(servers []string, logger *zap.Logger) *STUNDetector {
	return &STUNDetector{
		servers: servers,
		logger:  logger,
	}
}

// Detect 检测 NAT 类型
func (d *STUNDetector) Detect(ctx context.Context) (NATType, string, int, error) {
	// 简化实现：尝试连接 STUN 服务器获取公网地址
	// 实际实现需要完整的 STUN 协议支持
	for _, server := range d.servers {
		addr, err := d.discoverPublicAddr(ctx, server)
		if err != nil {
			d.logger.Debug("STUN discovery failed",
				zap.String("server", server),
				zap.Error(err),
			)
			continue
		}

		host, port, err := net.SplitHostPort(addr.String())
		if err != nil {
			continue
		}

		var portInt int
		_, _ = fmt.Sscanf(port, "%d", &portInt)

		// 简化：假设为端口受限锥形 NAT
		// 实际实现需要完整的 RFC 3489 测试流程
		return NATTypePortRestrictedCone, host, portInt, nil
	}

	return NATTypeUnknown, "", 0, errors.New("all STUN servers failed")
}

// discoverPublicAddr 发现公网地址
func (d *STUNDetector) discoverPublicAddr(ctx context.Context, server string) (net.Addr, error) {
	// 解析 STUN 服务器地址
	stunAddr, err := net.ResolveUDPAddr("udp", server)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve STUN server: %w", err)
	}

	// 创建 UDP 连接
	conn, err := net.DialUDP("udp", nil, stunAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial STUN server: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// 设置超时
	deadline, ok := ctx.Deadline()
	if ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	}

	// 发送 STUN Binding Request
	// 简化实现：实际的 STUN 协议需要构造正确的消息格式
	_, err = conn.Write([]byte("STUN_REQUEST"))
	if err != nil {
		return nil, fmt.Errorf("failed to send STUN request: %w", err)
	}

	// 获取本地地址
	localAddr := conn.LocalAddr()

	// 注意：这是简化实现，实际需要解析 STUN 响应获取映射地址
	// 这里返回本地地址作为示例
	return localAddr, nil
}

// GetPublicAddr 获取公网地址
func (d *STUNDetector) GetPublicAddr() (string, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, ip, port, err := d.Detect(ctx)
	return ip, port, err
}
