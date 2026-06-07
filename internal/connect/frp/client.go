// Package frp provides FRP client implementation
// FRP客户端核心连接逻辑
package frp

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
)

// 错误定义
var (
	ErrNotConnected      = errors.New("frp client not connected")
	ErrAlreadyConnected  = errors.New("frp client already connected")
	ErrServerUnreachable = errors.New("frp server unreachable")
	ErrAuthFailed        = errors.New("frp authentication failed")
	ErrTunnelNotFound    = errors.New("tunnel not found")
	ErrInvalidConfig     = errors.New("invalid frp configuration")
)

// Client FRP客户端
type Client struct {
	config  *ClientConfig
	conn    net.Conn
	status  string
	tunnels map[string]*TunnelSession
	stats   *ClientStats
	logger  *zap.Logger

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 回调
	onConnect      func()
	onDisconnect   func(error)
	onTunnelChange func(string, string)
}

// TunnelSession 隧道会话
type TunnelSession struct {
	config     TunnelConfig
	status     string
	conn       net.Conn
	listener   net.Listener
	bytesSent  uint64
	bytesRecv  uint64
	lastActive time.Time
	mu         sync.Mutex
}

// ClientStats 客户端统计
type ClientStats struct {
	BytesSent     uint64    `json:"bytes_sent"`
	BytesReceived uint64    `json:"bytes_received"`
	Connections   int       `json:"connections"`
	LastActivity  time.Time `json:"last_activity"`
	Latency       int       `json:"latency_ms"`
	Uptime        string    `json:"uptime"`
	ConnectedAt   time.Time `json:"connected_at"`
}

// NewClient 创建FRP客户端
func NewClient(config *ClientConfig, logger *zap.Logger) (*Client, error) {
	if config.Common.ServerAddr == "" {
		return nil, ErrInvalidConfig
	}

	if config.Common.ServerPort <= 0 {
		config.Common.ServerPort = 7000
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		config:  config,
		status:  "disconnected",
		tunnels: make(map[string]*TunnelSession),
		stats:   &ClientStats{},
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

// Start 启动客户端
func (c *Client) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status == "connected" {
		return ErrAlreadyConnected
	}

	c.status = "connecting"
	c.logger.Info("Starting FRP client",
		zap.String("server", c.config.Common.ServerAddr),
		zap.Int("port", c.config.Common.ServerPort))

	// 连接服务器
	if err := c.connectServer(); err != nil {
		c.status = "error"
		return err
	}

	// 启动隧道
	for _, tunnel := range c.config.Tunnels {
		if tunnel.Enabled {
			if err := c.startTunnel(tunnel); err != nil {
				c.logger.Error("Failed to start tunnel",
					zap.String("id", tunnel.ID),
					zap.Error(err))
				continue
			}
		}
	}

	c.status = "connected"
	c.stats.ConnectedAt = time.Now()

	// 启动心跳
	c.wg.Add(1)
	go c.heartbeatLoop()

	// 启动连接监听
	c.wg.Add(1)
	go c.connectionLoop()

	if c.onConnect != nil {
		c.onConnect()
	}

	c.logger.Info("FRP client started successfully")
	return nil
}

// connectServer 连接到FRP服务器
func (c *Client) connectServer() error {
	addr := fmt.Sprintf("%s:%d", c.config.Common.ServerAddr, c.config.Common.ServerPort)

	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}

	var conn net.Conn
	var err error

	if c.config.Common.TLSEnable {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
		}

		if c.config.Common.TLSCertFile != "" && c.config.Common.TLSKeyFile != "" {
			cert, err := tls.LoadX509KeyPair(c.config.Common.TLSCertFile, c.config.Common.TLSKeyFile)
			if err == nil {
				tlsConfig.Certificates = []tls.Certificate{cert}
			}
		}

		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	} else {
		conn, err = dialer.DialContext(c.ctx, "tcp", addr)
	}

	if err != nil {
		return fmt.Errorf("%w: %v", ErrServerUnreachable, err)
	}

	c.conn = conn

	// 发送认证
	if err := c.authenticate(); err != nil {
		_ = conn.Close()
		return err
	}

	return nil
}

// authenticate 认证
func (c *Client) authenticate() error {
	// 发送认证请求
	authReq := AuthRequest{
		Version:   "0.52.0",
		Token:     c.config.Common.Token,
		Timestamp: time.Now().Unix(),
		RunID:     generateRunID(),
	}

	data, err := EncodeMessage(MsgTypeAuth, authReq)
	if err != nil {
		return err
	}

	if _, err := c.conn.Write(data); err != nil {
		return fmt.Errorf("%w: failed to send auth", ErrAuthFailed)
	}

	// 等待响应
	resp, err := c.readResponse()
	if err != nil {
		return err
	}

	if resp.Type == MsgTypeAuthResp {
		authResp, err := DecodeMessage(resp)
		if err != nil {
			return err
		}

		authRespData, ok := authResp.(AuthResponse)
		if !ok {
			return ErrAuthFailed
		}

		if authRespData.Error != "" {
			return fmt.Errorf("%w: %s", ErrAuthFailed, authRespData.Error)
		}

		c.logger.Debug("FRP authentication successful",
			zap.String("run_id", authRespData.RunID))
	}

	return nil
}

// readResponse 读取服务器响应
func (c *Client) readResponse() (*Message, error) {
	header := make([]byte, 8)
	if _, err := io.ReadFull(c.conn, header); err != nil {
		return nil, err
	}

	msgType := binary.BigEndian.Uint16(header[0:2])
	msgLen := binary.BigEndian.Uint64(header[2:8])

	if msgLen > 1024*1024 {
		return nil, errors.New("message too large")
	}

	body := make([]byte, msgLen)
	if _, err := io.ReadFull(c.conn, body); err != nil {
		return nil, err
	}

	return &Message{
		Type: MessageType(msgType),
		Len:  msgLen,
		Data: body,
	}, nil
}

// startTunnel 启动隧道
func (c *Client) startTunnel(cfg TunnelConfig) error {
	session := &TunnelSession{
		config:     cfg,
		status:     "starting",
		lastActive: time.Now(),
	}

	c.tunnels[cfg.ID] = session

	// 发送隧道请求
	tunnelReq := TunnelRequest{
		Name:          cfg.Name,
		Type:          string(cfg.Type),
		LocalIP:       cfg.LocalIP,
		LocalPort:     cfg.LocalPort,
		RemotePort:    cfg.RemotePort,
		SubDomain:     cfg.SubDomain,
		CustomDomains: cfg.CustomDomains,
		Sk:            cfg.Sk,
	}

	data, err := EncodeMessage(MsgTypeNewProxy, tunnelReq)
	if err != nil {
		session.status = "error"
		return err
	}

	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return ErrNotConnected
	}

	if _, err := conn.Write(data); err != nil {
		session.status = "error"
		return err
	}

	session.status = "running"
	session.lastActive = time.Now()

	c.logger.Info("Tunnel started",
		zap.String("id", cfg.ID),
		zap.String("type", string(cfg.Type)),
		zap.String("name", cfg.Name))

	return nil
}

// Stop 停止客户端
func (c *Client) Stop() error {
	c.cancel()
	c.wg.Wait()

	c.mu.Lock()
	defer c.mu.Unlock()

	// 关闭所有隧道
	for id, session := range c.tunnels {
		session.mu.Lock()
		if session.conn != nil {
			_ = session.conn.Close()
		}
		if session.listener != nil {
			_ = session.listener.Close()
		}
		session.status = "stopped"
		session.mu.Unlock()
		delete(c.tunnels, id)
	}

	// 关闭服务器连接
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}

	c.status = "stopped"

	c.logger.Info("FRP client stopped")
	return nil
}

// heartbeatLoop 心跳循环
func (c *Client) heartbeatLoop() {
	defer c.wg.Done()

	interval := time.Duration(c.config.Common.HeartbeatInterval) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if err := c.sendHeartbeat(); err != nil {
				c.logger.Debug("Heartbeat failed", zap.Error(err))
				c.mu.Lock()
				if c.status == "connected" {
					c.status = "reconnecting"
					c.mu.Unlock()

					// 尝试重连
					if err := c.reconnect(); err != nil {
						c.logger.Error("Reconnect failed", zap.Error(err))
						c.mu.Lock()
						c.status = "error"
						c.mu.Unlock()

						if c.onDisconnect != nil {
							c.onDisconnect(err)
						}
					}
				} else {
					c.mu.Unlock()
				}
			}
		}
	}
}

// sendHeartbeat 发送心跳
func (c *Client) sendHeartbeat() error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return ErrNotConnected
	}

	data, err := EncodeMessage(MsgTypePing, nil)
	if err != nil {
		return err
	}

	start := time.Now()
	if _, err := conn.Write(data); err != nil {
		return err
	}

	// 更新延迟统计
	c.mu.Lock()
	c.stats.Latency = int(time.Since(start).Milliseconds())
	c.stats.LastActivity = time.Now()
	c.mu.Unlock()

	return nil
}

// connectionLoop 连接监听循环
func (c *Client) connectionLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()

			if conn == nil {
				time.Sleep(1 * time.Second)
				continue
			}

			// 处理数据流
			msg, err := c.readResponse()
			if err != nil {
				if c.ctx.Err() != nil {
					return
				}
				c.logger.Debug("Read error", zap.Error(err))
				continue
			}

			c.handleMessage(msg)
		}
	}
}

// handleMessage 处理服务器消息
func (c *Client) handleMessage(msg *Message) {
	switch msg.Type {
	case MsgTypePong:
		// 心跳响应
		c.mu.Lock()
		c.stats.LastActivity = time.Now()
		c.mu.Unlock()

	case MsgTypeNewProxyResp:
		// 隧道响应
		resp, err := DecodeMessage(msg)
		if err != nil {
			c.logger.Debug("Failed to decode tunnel response", zap.Error(err))
			return
		}

		tunnelResp, ok := resp.(TunnelResponse)
		if !ok {
			return
		}

		c.mu.Lock()
		if session, exists := c.tunnels[tunnelResp.ProxyName]; exists {
			session.mu.Lock()
			if tunnelResp.Error != "" {
				session.status = "error"
			} else {
				session.status = "running"
			}
			session.mu.Unlock()
		}
		c.mu.Unlock()

	case MsgTypeData:
		// 数据传输
		c.handleData(msg)

	case MsgTypeCloseProxy:
		// 关闭隧道通知
		c.mu.Lock()
		for id, session := range c.tunnels {
			session.mu.Lock()
			session.status = "stopped"
			session.mu.Unlock()
			delete(c.tunnels, id)
		}
		c.mu.Unlock()
	}
}

// handleData 处理数据传输
func (c *Client) handleData(msg *Message) {
	data, err := DecodeMessage(msg)
	if err != nil {
		return
	}

	dataMsg, ok := data.(DataMessage)
	if !ok {
		return
	}

	// 找到对应隧道
	c.mu.RLock()
	session, exists := c.tunnels[dataMsg.ProxyName]
	c.mu.RUnlock()

	if !exists {
		return
	}

	// 转发数据到本地服务
	go c.forwardData(session, dataMsg)
}

// forwardData 转发数据
func (c *Client) forwardData(session *TunnelSession, dataMsg DataMessage) {
	cfg := session.config
	localAddr := net.JoinHostPort(cfg.LocalIP, strconv.Itoa(cfg.LocalPort))

	conn, err := net.DialTimeout("tcp", localAddr, 10*time.Second)
	if err != nil {
		c.logger.Debug("Failed to connect local service",
			zap.String("addr", localAddr),
			zap.Error(err))
		return
	}
	defer func() { _ = conn.Close() }()

	// 发送数据到本地
	if _, err := conn.Write(dataMsg.Data); err != nil {
		return
	}

	// 更新统计
	session.mu.Lock()
	session.bytesSent += uint64(len(dataMsg.Data))
	session.lastActive = time.Now()
	session.mu.Unlock()

	// 读取响应
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		return
	}

	if n > 0 {
		// 发送响应回服务器
		respData := DataMessage{
			ProxyName: dataMsg.ProxyName,
			Data:      buf[:n],
		}

		c.mu.RLock()
		serverConn := c.conn
		c.mu.RUnlock()

		if serverConn != nil {
			data, _ := EncodeMessage(MsgTypeData, respData)
			_, _ = serverConn.Write(data)

			session.mu.Lock()
			session.bytesRecv += uint64(n)
			session.mu.Unlock()
		}
	}
}

// reconnect 重连服务器
func (c *Client) reconnect() error {
	c.logger.Info("Attempting to reconnect...")

	// 关闭旧连接
	c.mu.Lock()
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	c.status = "reconnecting"
	c.mu.Unlock()

	// 重新连接
	if err := c.connectServer(); err != nil {
		return err
	}

	// 重启隧道
	c.mu.RLock()
	tunnelConfigs := make([]TunnelConfig, 0)
	for _, session := range c.tunnels {
		tunnelConfigs = append(tunnelConfigs, session.config)
	}
	c.mu.RUnlock()

	for _, cfg := range tunnelConfigs {
		if err := c.startTunnel(cfg); err != nil {
			c.logger.Debug("Failed to restart tunnel",
				zap.String("id", cfg.ID),
				zap.Error(err))
		}
	}

	c.mu.Lock()
	c.status = "connected"
	c.mu.Unlock()

	c.logger.Info("Reconnected successfully")
	return nil
}

// GetStatus 获取状态
func (c *Client) GetStatus() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// GetStats 获取统计
func (c *Client) GetStats() ClientStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.stats.ConnectedAt.IsZero() {
		c.stats.Uptime = time.Since(c.stats.ConnectedAt).Round(time.Second).String()
	}

	return *c.stats
}

// GetTunnelStatus 获取隧道状态
func (c *Client) GetTunnelStatus(id string) *TunnelStatus {
	c.mu.RLock()
	session, exists := c.tunnels[id]
	c.mu.RUnlock()

	if !exists {
		return nil
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	cfg := session.config
	status := &TunnelStatus{
		ID:         cfg.ID,
		Name:       cfg.Name,
		Type:       cfg.Type,
		Status:     session.status,
		LocalAddr:  fmt.Sprintf("%s:%d", cfg.LocalIP, cfg.LocalPort),
		BytesSent:  session.bytesSent,
		BytesRecv:  session.bytesRecv,
		LastActive: session.lastActive,
	}

	if cfg.Type == TunnelTypeHTTP || cfg.Type == TunnelTypeHTTPS {
		if cfg.SubDomain != "" {
			status.PublicURL = fmt.Sprintf("https://%s.%s", cfg.SubDomain, c.config.Common.ServerAddr)
		}
	} else if cfg.RemotePort > 0 {
		status.RemoteAddr = fmt.Sprintf("%s:%d", c.config.Common.ServerAddr, cfg.RemotePort)
		status.PublicURL = fmt.Sprintf("tcp://%s:%d", c.config.Common.ServerAddr, cfg.RemotePort)
	}

	return status
}

// ListTunnelStatus 列出所有隧道状态
func (c *Client) ListTunnelStatus() []TunnelStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	statuses := make([]TunnelStatus, 0, len(c.tunnels))
	for id := range c.tunnels {
		if status := c.GetTunnelStatus(id); status != nil {
			statuses = append(statuses, *status)
		}
	}
	return statuses
}

// AddTunnel 添加并启动隧道
func (c *Client) AddTunnel(cfg TunnelConfig) error {
	cfg.ID = generateTunnelID()
	cfg.CreatedAt = time.Now()
	cfg.UpdatedAt = time.Now()
	cfg.Enabled = true

	c.config.AddTunnel(cfg)

	if c.status == "connected" {
		return c.startTunnel(cfg)
	}
	return nil
}

// RemoveTunnel 移除隧道
func (c *Client) RemoveTunnel(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	session, exists := c.tunnels[id]
	if !exists {
		return ErrTunnelNotFound
	}

	session.mu.Lock()
	if session.conn != nil {
		_ = session.conn.Close()
	}
	if session.listener != nil {
		_ = session.listener.Close()
	}
	session.mu.Unlock()

	delete(c.tunnels, id)
	c.config.RemoveTunnel(id)

	return nil
}

// SetOnConnect 设置连接回调
func (c *Client) SetOnConnect(fn func()) {
	c.mu.Lock()
	c.onConnect = fn
	c.mu.Unlock()
}

// SetOnDisconnect 设置断开回调
func (c *Client) SetOnDisconnect(fn func(error)) {
	c.mu.Lock()
	c.onDisconnect = fn
	c.mu.Unlock()
}

// generateRunID 生成运行ID
func generateRunID() string {
	return fmt.Sprintf("run_%d", time.Now().UnixNano())
}
