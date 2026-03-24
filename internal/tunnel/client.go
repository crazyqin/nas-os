// Package tunnel 提供内网穿透客户端实现
package tunnel

import (
	"context"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

// BaseClient 基础客户端
type BaseClient struct {
	config    TunnelConfig
	mgrConfig Config
	logger    *zap.Logger
	conn      net.Conn
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	status    TunnelStatus
	connected bool
}

// NewP2PClient 创建 P2P 客户端
func NewP2PClient(config TunnelConfig, mgrConfig Config, logger *zap.Logger) TunnelClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &P2PClient{
		BaseClient: BaseClient{
			config:    config,
			mgrConfig: mgrConfig,
			logger:    logger,
			ctx:       ctx,
			cancel:    cancel,
			status: TunnelStatus{
				ID:    config.ID,
				Name:  config.Name,
				Mode:  ModeP2P,
				State: StateDisconnected,
			},
		},
	}
}

// NewRelayClient 创建中继客户端
func NewRelayClient(config TunnelConfig, mgrConfig Config, logger *zap.Logger) TunnelClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &RelayClient{
		BaseClient: BaseClient{
			config:    config,
			mgrConfig: mgrConfig,
			logger:    logger,
			ctx:       ctx,
			cancel:    cancel,
			status: TunnelStatus{
				ID:    config.ID,
				Name:  config.Name,
				Mode:  ModeRelay,
				State: StateDisconnected,
			},
		},
	}
}

// NewReverseClient 创建反向代理客户端
func NewReverseClient(config TunnelConfig, mgrConfig Config, logger *zap.Logger) TunnelClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &ReverseClient{
		BaseClient: BaseClient{
			config:    config,
			mgrConfig: mgrConfig,
			logger:    logger,
			ctx:       ctx,
			cancel:    cancel,
			status: TunnelStatus{
				ID:    config.Name,
				Name:  config.Name,
				Mode:  ModeReverse,
				State: StateDisconnected,
			},
		},
	}
}

// NewAutoClient 创建自动模式客户端
func NewAutoClient(config TunnelConfig, mgrConfig Config, logger *zap.Logger) TunnelClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &AutoClient{
		BaseClient: BaseClient{
			config:    config,
			mgrConfig: mgrConfig,
			logger:    logger,
			ctx:       ctx,
			cancel:    cancel,
			status: TunnelStatus{
				ID:    config.ID,
				Name:  config.Name,
				Mode:  ModeAuto,
				State: StateDisconnected,
			},
		},
	}
}

// P2PClient P2P 直连客户端
type P2PClient struct {
	BaseClient
	stunAddr string
}

// Connect 实现 P2P 连接
func (c *P2PClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Info("connecting via P2P mode", zap.String("id", c.config.ID))

	// TODO: 实现 STUN 打洞逻辑
	// 1. 通过 STUN 服务器获取公网地址
	// 2. 与对端交换地址信息
	// 3. 尝试直连

	c.status.State = StateConnected
	c.status.LastConnected = time.Now()
	c.connected = true
	return nil
}

// Disconnect 断开连接
func (c *P2PClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.connected = false
		return c.conn.Close()
	}
	c.status.State = StateDisconnected
	c.connected = false
	return nil
}

// GetStatus 获取状态
func (c *P2PClient) GetStatus() TunnelStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// Send 发送数据
func (c *P2PClient) Send(data []byte) (int, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return 0, ErrNotConnected
	}
	n, err := c.conn.Write(data)
	if err == nil {
		c.status.BytesSent += int64(n)
	}
	return n, err
}

// Receive 接收数据
func (c *P2PClient) Receive() ([]byte, error) {
	buf := make([]byte, 4096)
	n, err := c.conn.Read(buf)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.status.BytesReceived += int64(n)
	c.mu.Unlock()
	return buf[:n], nil
}

// IsConnected 检查是否已连接
func (c *P2PClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// RelayClient 中继客户端
type RelayClient struct {
	BaseClient
	relayAddr string
}

// Connect 实现中继连接
func (c *RelayClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Info("connecting via relay mode", zap.String("id", c.config.ID))

	// TODO: 实现 TURN 中继连接
	// 1. 连接到 TURN 服务器
	// 2. 申请中继资源
	// 3. 通过中继转发数据

	c.status.State = StateConnected
	c.status.LastConnected = time.Now()
	c.connected = true
	return nil
}

// Disconnect 断开连接
func (c *RelayClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.connected = false
		return c.conn.Close()
	}
	c.status.State = StateDisconnected
	c.connected = false
	return nil
}

// GetStatus 获取状态
func (c *RelayClient) GetStatus() TunnelStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// Send 发送数据
func (c *RelayClient) Send(data []byte) (int, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return 0, ErrNotConnected
	}
	n, err := c.conn.Write(data)
	if err == nil {
		c.status.BytesSent += int64(n)
	}
	return n, err
}

// Receive 接收数据
func (c *RelayClient) Receive() ([]byte, error) {
	buf := make([]byte, 4096)
	n, err := c.conn.Read(buf)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.status.BytesReceived += int64(n)
	c.mu.Unlock()
	return buf[:n], nil
}

// IsConnected 检查是否已连接
func (c *RelayClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// ReverseClient 反向代理客户端
type ReverseClient struct {
	BaseClient
	listener net.Listener
}

// Connect 实现反向代理连接
func (c *ReverseClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Info("connecting via reverse mode", zap.String("id", c.config.ID))

	// TODO: 实现反向代理
	// 1. 连接到隧道服务器
	// 2. 注册服务
	// 3. 等待远程连接

	c.status.State = StateConnected
	c.status.LastConnected = time.Now()
	c.connected = true
	return nil
}

// Disconnect 断开连接
func (c *ReverseClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.listener != nil {
		c.listener.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
	c.status.State = StateDisconnected
	c.connected = false
	return nil
}

// GetStatus 获取状态
func (c *ReverseClient) GetStatus() TunnelStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// Send 发送数据
func (c *ReverseClient) Send(data []byte) (int, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return 0, ErrNotConnected
	}
	n, err := c.conn.Write(data)
	if err == nil {
		c.status.BytesSent += int64(n)
	}
	return n, err
}

// Receive 接收数据
func (c *ReverseClient) Receive() ([]byte, error) {
	buf := make([]byte, 4096)
	n, err := c.conn.Read(buf)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.status.BytesReceived += int64(n)
	c.mu.Unlock()
	return buf[:n], nil
}

// IsConnected 检查是否已连接
func (c *ReverseClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// AutoClient 自动模式客户端
type AutoClient struct {
	BaseClient
	currentClient TunnelClient
}

// Connect 实现自动连接
func (c *AutoClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Info("connecting via auto mode", zap.String("id", c.config.ID))

	// 先尝试 P2P
	p2pClient := NewP2PClient(c.config, c.mgrConfig, c.logger)
	if err := p2pClient.Connect(ctx); err == nil {
		c.currentClient = p2pClient
		c.status.State = StateConnected
		c.status.LastConnected = time.Now()
		c.connected = true
		return nil
	}

	// P2P 失败，切换到中继
	c.logger.Info("P2P failed, falling back to relay mode")
	relayClient := NewRelayClient(c.config, c.mgrConfig, c.logger)
	if err := relayClient.Connect(ctx); err != nil {
		return err
	}

	c.currentClient = relayClient
	c.status.State = StateConnected
	c.status.LastConnected = time.Now()
	c.connected = true
	return nil
}

// Disconnect 断开连接
func (c *AutoClient) Disconnect() error {
	if c.currentClient != nil {
		c.connected = false
		return c.currentClient.Disconnect()
	}
	return nil
}

// GetStatus 获取状态
func (c *AutoClient) GetStatus() TunnelStatus {
	if c.currentClient != nil {
		return c.currentClient.GetStatus()
	}
	return c.status
}

// Send 发送数据
func (c *AutoClient) Send(data []byte) (int, error) {
	if c.currentClient != nil {
		return c.currentClient.Send(data)
	}
	return 0, ErrNotConnected
}

// Receive 接收数据
func (c *AutoClient) Receive() ([]byte, error) {
	if c.currentClient != nil {
		return c.currentClient.Receive()
	}
	return nil, ErrNotConnected
}

// IsConnected 检查是否已连接
func (c *AutoClient) IsConnected() bool {
	if c.currentClient != nil {
		return c.currentClient.IsConnected()
	}
	return c.connected
}