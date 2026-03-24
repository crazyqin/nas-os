// Package tunnel 提供内网穿透服务 - 隧道服务
package tunnel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// TunnelService 隧道服务
type TunnelService struct {
	config      Config
	logger      *zap.Logger
	p2pConfig   P2PConfig
	signal      *SignalClient
	manager     *Manager
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	running     atomic.Bool
	activeConns map[string]*TunnelConnection
}

// TunnelConnection 隧道连接
type TunnelConnection struct {
	ID         string
	LocalAddr  net.Addr
	RemoteAddr net.Addr
	P2PConn    *P2PConn
	StartTime  time.Time
	BytesSent  int64
	BytesRecv  int64
	closed     atomic.Bool
}

// NewTunnelService 创建隧道服务
func NewTunnelService(config Config, logger *zap.Logger) (*TunnelService, error) {
	// 创建管理器
	manager, err := NewManager(config, logger)
	if err != nil {
		return nil, err
	}

	// 创建 P2P 配置
	p2pConfig := P2PConfig{
		STUNServers:      config.STUNServers,
		TURNServers:      config.TURNServers,
		TURNUser:         config.TURNUser,
		TURNPass:         config.TURNPass,
		ConnectTimeout:   time.Duration(config.Timeout) * time.Second,
		ICEGatherTimeout: 10 * time.Second,
		HolePunchRetries: 5,
	}

	// 创建信令客户端配置
	signalConfig := SignalConfig{
		ServerURL:      fmt.Sprintf("%s:%d", config.ServerAddr, config.ServerPort),
		AuthToken:      config.AuthToken,
		DeviceID:       config.DeviceID,
		DeviceName:     config.DeviceName,
		ReconnectDelay: time.Duration(config.ReconnectInt) * time.Second,
		MaxReconnect:   config.MaxReconnect,
		PingInterval:   30 * time.Second,
		PongWait:       60 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &TunnelService{
		config:      config,
		logger:      logger,
		p2pConfig:   p2pConfig,
		manager:     manager,
		signal:      NewSignalClient(signalConfig, logger),
		ctx:         ctx,
		cancel:      cancel,
		activeConns: make(map[string]*TunnelConnection),
	}, nil
}

// Start 启动隧道服务
func (s *TunnelService) Start(ctx context.Context) error {
	if s.running.Swap(true) {
		return errors.New("service already running")
	}

	s.logger.Info("starting tunnel service",
		zap.String("device_id", s.config.DeviceID),
		zap.String("server", s.config.ServerAddr),
	)

	// 启动管理器
	if err := s.manager.Start(ctx); err != nil {
		s.running.Store(false)
		return err
	}

	// 连接信令服务器
	signalCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := s.signal.Connect(signalCtx); err != nil {
		s.logger.Warn("failed to connect signal server, will retry", zap.Error(err))
		go s.reconnectSignal()
	}

	// 设置信令消息处理
	s.signal.OnMessage(s.handleSignalMessage)
	s.signal.OnDisconnect(func() {
		go s.reconnectSignal()
	})

	s.logger.Info("tunnel service started")
	return nil
}

// Stop 停止隧道服务
func (s *TunnelService) Stop() error {
	if !s.running.Swap(false) {
		return nil
	}

	s.logger.Info("stopping tunnel service")

	s.cancel()

	// 关闭所有连接
	s.mu.Lock()
	for _, conn := range s.activeConns {
		_ = conn.Close()
	}
	s.activeConns = make(map[string]*TunnelConnection)
	s.mu.Unlock()

	// 断开信令连接
	_ = s.signal.Disconnect()

	// 停止管理器
	_ = s.manager.Stop()

	s.logger.Info("tunnel service stopped")
	return nil
}

// reconnectSignal 重连信令服务器
func (s *TunnelService) reconnectSignal() {
	retries := 0
	for {
		if !s.running.Load() {
			return
		}

		retries++
		if retries > s.config.MaxReconnect {
			s.logger.Error("max reconnect attempts reached")
			return
		}

		time.Sleep(time.Duration(s.config.ReconnectInt) * time.Second)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := s.signal.Connect(ctx)
		cancel()

		if err != nil {
			s.logger.Debug("signal reconnect failed", zap.Int("attempt", retries), zap.Error(err))
			continue
		}

		s.logger.Info("signal server reconnected")
		return
	}
}

// handleSignalMessage 处理信令消息
func (s *TunnelService) handleSignalMessage(msg SignalMessage) {
	s.logger.Debug("received signal message",
		zap.String("type", string(msg.Type)),
		zap.String("from", msg.FromID),
	)

	switch msg.Type {
	case SignalMessageTypeOffer:
		go s.handleOffer(msg)
	case SignalMessageTypeAnswer:
		go s.handleAnswer(msg)
	case SignalMessageTypeCandidate:
		go s.handleCandidate(msg)
	case SignalMessageTypeConnected:
		s.logger.Info("peer connected", zap.String("peer_id", msg.FromID))
	}
}

// handleOffer 处理 Offer 消息
func (s *TunnelService) handleOffer(msg SignalMessage) {
	var payload SessionPayload
	if err := jsonUnmarshal(msg.Payload, &payload); err != nil {
		s.logger.Debug("failed to parse offer", zap.Error(err))
		return
	}

	// 创建 P2P 连接
	conn := NewP2PConn(s.p2pConfig, s.logger)
	conn.session.LocalID = s.config.DeviceID
	conn.session.RemoteID = msg.FromID

	// 收集本地候选
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	localCandidates, err := conn.GatherCandidates(ctx)
	cancel()
	if err != nil {
		s.logger.Debug("failed to gather candidates", zap.Error(err))
		return
	}

	// 设置远程候选
	conn.SetRemoteCandidates(payload.Session.LocalCandidates)

	// 尝试连接
	connectCtx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	err = conn.Connect(connectCtx)
	cancel()

	if err != nil {
		s.logger.Debug("failed to establish P2P connection", zap.Error(err))
		return
	}

	// 发送 Answer
	session := conn.GetSession()
	session.LocalID = s.config.DeviceID
	session.RemoteID = msg.FromID
	session.LocalCandidates = localCandidates

	if err := s.signal.SendAnswer(msg.FromID, session); err != nil {
		s.logger.Debug("failed to send answer", zap.Error(err))
		return
	}

	// 发送已连接通知
	_ = s.signal.SendConnected(msg.FromID)

	s.logger.Info("P2P connection established (incoming)",
		zap.String("peer_id", msg.FromID),
	)
}

// handleAnswer 处理 Answer 消息
func (s *TunnelService) handleAnswer(msg SignalMessage) {
	s.mu.RLock()
	conn, exists := s.activeConns[msg.SessionID]
	s.mu.RUnlock()

	if !exists {
		s.logger.Debug("no pending connection for answer", zap.String("session_id", msg.SessionID))
		return
	}

	var payload SessionPayload
	if err := jsonUnmarshal(msg.Payload, &payload); err != nil {
		s.logger.Debug("failed to parse answer", zap.Error(err))
		return
	}

	// 设置远程候选并连接
	if conn.P2PConn != nil {
		conn.P2PConn.SetRemoteCandidates(payload.Session.LocalCandidates)

		ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
		err := conn.P2PConn.Connect(ctx)
		cancel()

		if err != nil {
			s.logger.Debug("failed to connect after answer", zap.Error(err))
			return
		}

		// 发送已连接通知
		_ = s.signal.SendConnected(msg.FromID)

		s.logger.Info("P2P connection established (outgoing)",
			zap.String("peer_id", msg.FromID),
		)
	}
}

// handleCandidate 处理 ICE 候选消息
func (s *TunnelService) handleCandidate(msg SignalMessage) {
	var payload CandidatePayload
	if err := jsonUnmarshal(msg.Payload, &payload); err != nil {
		s.logger.Debug("failed to parse candidate", zap.Error(err))
		return
	}

	s.mu.RLock()
	conn, exists := s.activeConns[msg.SessionID]
	s.mu.RUnlock()

	if !exists {
		return
	}

	if conn.P2PConn != nil {
		conn.P2PConn.SetRemoteCandidates([]ICECandidate{payload.Candidate})
	}
}

// ConnectToPeer 连接到对端
func (s *TunnelService) ConnectToPeer(ctx context.Context, peerID string) (*TunnelConnection, error) {
	// 创建 P2P 连接
	p2pConn := NewP2PConn(s.p2pConfig, s.logger)
	p2pConn.session.LocalID = s.config.DeviceID
	p2pConn.session.RemoteID = peerID

	// 收集本地候选
	candidates, err := p2pConn.GatherCandidates(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to gather candidates: %w", err)
	}

	// 生成会话 ID
	sessionID := generateID()
	s.signal.SetSessionID(sessionID)

	// 发送 Offer
	session := p2pConn.GetSession()
	session.LocalCandidates = candidates

	if err := s.signal.SendOffer(peerID, session); err != nil {
		return nil, fmt.Errorf("failed to send offer: %w", err)
	}

	// 创建连接对象
	conn := &TunnelConnection{
		ID:        sessionID,
		P2PConn:   p2pConn,
		StartTime: time.Now(),
	}

	s.mu.Lock()
	s.activeConns[sessionID] = conn
	s.mu.Unlock()

	// 等待连接建立（通过 Answer 消息）
	select {
	case <-ctx.Done():
		s.mu.Lock()
		delete(s.activeConns, sessionID)
		s.mu.Unlock()
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		s.mu.Lock()
		delete(s.activeConns, sessionID)
		s.mu.Unlock()
		return nil, errors.New("connection timeout")
	}
}

// Dial 创建隧道连接到指定地址
func (s *TunnelService) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	// 解析目标地址（格式：deviceID:port）
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	// 通过 P2P 连接到对端设备
	conn, err := s.ConnectToPeer(ctx, host)
	if err != nil {
		return nil, err
	}

	// 包装为 net.Conn
	return &TunnelNetConn{
		TunnelConnection: conn,
		remotePort:       port,
	}, nil
}

// Listen 在指定端口监听隧道连接
func (s *TunnelService) Listen(network, addr string) (net.Listener, error) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	return &TunnelListener{
		service:  s,
		port:     port,
		acceptCh: make(chan *TunnelConnection, 10),
	}, nil
}

// GetStatus 获取服务状态
func (s *TunnelService) GetStatus() *TunnelServiceStatus {
	managerStatus := s.manager.GetStatus()

	conns := make([]*ConnectionStatus, 0)
	s.mu.RLock()
	for _, conn := range s.activeConns {
		sent, recv := conn.P2PConn.GetStats()
		conns = append(conns, &ConnectionStatus{
			ID:        conn.ID,
			PeerID:    conn.P2PConn.session.RemoteID,
			State:     conn.P2PConn.session.State,
			BytesSent: sent,
			BytesRecv: recv,
			Duration:  time.Since(conn.StartTime).Seconds(),
		})
	}
	s.mu.RUnlock()

	return &TunnelServiceStatus{
		Running:         s.running.Load(),
		SignalConnected: s.signal.IsConnected(),
		NATType:         managerStatus.NATType,
		PublicIP:        managerStatus.PublicIP,
		PublicPort:      managerStatus.PublicPort,
		ActiveConns:     len(conns),
		Connections:     conns,
		TotalBytesSent:  managerStatus.TotalBytesTx,
		TotalBytesRecv:  managerStatus.TotalBytesRx,
		Uptime:          time.Since(managerStatus.StartTime).Seconds(),
	}
}

// TunnelServiceStatus 隧道服务状态
type TunnelServiceStatus struct {
	Running         bool                `json:"running"`
	SignalConnected bool                `json:"signal_connected"`
	NATType         NATType             `json:"nat_type"`
	PublicIP        string              `json:"public_ip"`
	PublicPort      int                 `json:"public_port"`
	ActiveConns     int                 `json:"active_conns"`
	Connections     []*ConnectionStatus `json:"connections"`
	TotalBytesSent  int64               `json:"total_bytes_sent"`
	TotalBytesRecv  int64               `json:"total_bytes_recv"`
	Uptime          float64             `json:"uptime"`
}

// ConnectionStatus 连接状态
type ConnectionStatus struct {
	ID        string  `json:"id"`
	PeerID    string  `json:"peer_id"`
	State     string  `json:"state"`
	BytesSent int64   `json:"bytes_sent"`
	BytesRecv int64   `json:"bytes_recv"`
	Duration  float64 `json:"duration"`
}

// Close 关闭连接
func (c *TunnelConnection) Close() error {
	if c.closed.Swap(true) {
		return nil
	}
	if c.P2PConn != nil {
		return c.P2PConn.Close()
	}
	return nil
}

// TunnelNetConn 包装隧道连接为 net.Conn
type TunnelNetConn struct {
	*TunnelConnection
	remotePort string
}

func (c *TunnelNetConn) Read(b []byte) (n int, err error) {
	if c.P2PConn == nil {
		return 0, io.EOF
	}
	data, err := c.P2PConn.Receive()
	if err != nil {
		return 0, err
	}
	n = copy(b, data)
	atomic.AddInt64(&c.BytesRecv, int64(n))
	return n, nil
}

func (c *TunnelNetConn) Write(b []byte) (n int, err error) {
	if c.P2PConn == nil {
		return 0, io.ErrClosedPipe
	}
	n, err = c.P2PConn.Send(b)
	atomic.AddInt64(&c.BytesSent, int64(n))
	return n, err
}

// LocalAddr 返回本地地址
func (c *TunnelNetConn) LocalAddr() net.Addr {
	if c.TunnelConnection.LocalAddr != nil {
		return c.TunnelConnection.LocalAddr
	}
	return &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 0}
}

// RemoteAddr 返回远程地址
func (c *TunnelNetConn) RemoteAddr() net.Addr {
	if c.TunnelConnection.RemoteAddr != nil {
		return c.TunnelConnection.RemoteAddr
	}
	return &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 0}
}

// SetDeadline 设置读写截止时间
func (c *TunnelNetConn) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline 设置读取截止时间
func (c *TunnelNetConn) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline 设置写入截止时间
func (c *TunnelNetConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// TunnelListener 隧道监听器
type TunnelListener struct {
	service  *TunnelService
	port     string
	acceptCh chan *TunnelConnection
}

// Accept 接受新连接
func (l *TunnelListener) Accept() (net.Conn, error) {
	conn, ok := <-l.acceptCh
	if !ok {
		return nil, io.EOF
	}
	return &TunnelNetConn{TunnelConnection: conn}, nil
}

// Close 关闭监听器
func (l *TunnelListener) Close() error {
	close(l.acceptCh)
	return nil
}

// Addr 返回监听地址
func (l *TunnelListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 0}
}

// jsonUnmarshal 辅助函数
func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
