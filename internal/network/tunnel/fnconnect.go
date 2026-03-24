// Package tunnel 提供内网穿透服务
// 参考 飞牛fnOS FN Connect 的免费内网穿透实现
package tunnel

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// FNConnect 飞牛Connect免费内网穿透客户端
// 提供类似 frp 的功能，但使用免费公共服务
type FNConnect struct {
	config     *FNConnectConfig
	logger     *zap.Logger
	httpClient *http.Client

	// 连接状态
	state       FNConnectState
	peerID      string
	publicURL   string
	accessToken string

	// 隧道管理
	tunnels    map[string]*FNCTunnel
	activeConn map[string]net.Conn

	// 控制通道
	controlConn net.Conn
	msgChan     chan *FNCMessage

	// 事件处理
	eventHandlers []FNConnectEventHandler
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// FNConnectConfig 配置
type FNConnectConfig struct {
	// 服务器配置
	ServerURL    string `json:"server_url"`    // 穿透服务器地址
	Region       string `json:"region"`        // 区域: cn, us, eu
	DeviceID     string `json:"device_id"`     // 设备ID
	DeviceName   string `json:"device_name"`   // 设备名称
	AuthToken    string `json:"auth_token"`    // 认证令牌（可选）

	// 连接配置
	ReconnectInterval time.Duration `json:"reconnect_interval"` // 重连间隔
	HeartbeatInterval time.Duration `json:"heartbeat_interval"` // 心跳间隔
	Timeout           time.Duration `json:"timeout"`            // 超时时间
	MaxRetries        int           `json:"max_retries"`        // 最大重试次数

	// 安全配置
	EnableTLS    bool   `json:"enable_tls"`    // 启用TLS
	TokenFile    string `json:"token_file"`    // 令牌存储文件
	AllowInsecure bool  `json:"allow_insecure"` // 允许不安全连接

	// 带宽配置
	MaxBandwidth int64 `json:"max_bandwidth"` // 最大带宽 (bytes/s)
	QoSLevel     int   `json:"qos_level"`     // QoS级别 1-5
}

// FNConnectState 连接状态
type FNConnectState string

const (
	FNCStateDisconnected FNConnectState = "disconnected"
	FNCStateConnecting   FNConnectState = "connecting"
	FNCStateConnected    FNConnectState = "connected"
	FNCStateReconnecting FNConnectState = "reconnecting"
	FNCStateError        FNConnectState = "error"
)

// FNCTunnel 隧道配置
type FNCTunnel struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Protocol    string    `json:"protocol"`     // tcp, udp, http, https
	LocalPort   int       `json:"local_port"`
	RemotePort  int       `json:"remote_port"`  // 0表示自动分配
	Subdomain   string    `json:"subdomain"`    // HTTP隧道子域名
	CustomDomain string   `json:"custom_domain"` // 自定义域名
	State       FNConnectState `json:"state"`
	PublicURL   string    `json:"public_url"`   // 公网访问地址
	CreatedAt   time.Time `json:"created_at"`
	BytesTx     int64     `json:"bytes_tx"`
	BytesRx     int64     `json:"bytes_rx"`
}

// FNCMessage 消息结构
type FNCMessage struct {
	Type      string          `json:"type"`
	TunnelID  string          `json:"tunnel_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// FNConnectEventHandler 事件处理器
type FNConnectEventHandler func(event *FNConnectEvent)

// FNConnectEvent 事件
type FNConnectEvent struct {
	Type      string          `json:"type"`
	TunnelID  string          `json:"tunnel_id,omitempty"`
	State     FNConnectState  `json:"state,omitempty"`
	Error     error           `json:"error,omitempty"`
	Data      interface{}     `json:"data,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// FNConnectStats 统计信息
type FNConnectStats struct {
	State          FNConnectState `json:"state"`
	PublicURL      string         `json:"public_url"`
	ActiveTunnels  int            `json:"active_tunnels"`
	TotalBytesTx   int64          `json:"total_bytes_tx"`
	TotalBytesRx   int64          `json:"total_bytes_rx"`
	Uptime         time.Duration  `json:"uptime"`
	LastConnected  time.Time      `json:"last_connected"`
	ReconnectCount int            `json:"reconnect_count"`
}

// 默认公共穿透服务器列表
var defaultFNCServers = map[string][]string{
	"cn": {
		"connect.fnos.cn:7000",
		"tunnel.fnos.cn:7000",
	},
	"us": {
		"connect.fnos.us:7000",
	},
	"eu": {
		"connect.fnos.eu:7000",
	},
}

// NewFNConnect 创建FN Connect客户端
func NewFNConnect(config *FNConnectConfig, logger *zap.Logger) (*FNConnect, error) {
	if config == nil {
		config = DefaultFNConnectConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	// 创建HTTP客户端
	httpClient := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: config.AllowInsecure,
			},
		},
	}

	// 生成设备ID
	if config.DeviceID == "" {
		config.DeviceID = generateDeviceID()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &FNConnect{
		config:     config,
		logger:     logger,
		httpClient: httpClient,
		state:      FNCStateDisconnected,
		tunnels:    make(map[string]*FNCTunnel),
		activeConn: make(map[string]net.Conn),
		msgChan:    make(chan *FNCMessage, 100),
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

// DefaultFNConnectConfig 默认配置
func DefaultFNConnectConfig() *FNConnectConfig {
	return &FNConnectConfig{
		ServerURL:        "connect.fnos.cn:7000",
		Region:           "cn",
		ReconnectInterval: 5 * time.Second,
		HeartbeatInterval: 30 * time.Second,
		Timeout:          30 * time.Second,
		MaxRetries:       5,
		EnableTLS:        true,
		MaxBandwidth:     10 * 1024 * 1024, // 10MB/s
		QoSLevel:         3,
	}
}

// Connect 连接到穿透服务器
func (f *FNConnect) Connect(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.state == FNCStateConnected {
		return errors.New("already connected")
	}

	f.state = FNCStateConnecting
	f.emitEvent(&FNConnectEvent{
		Type: "state_change",
		State: FNCStateConnecting,
	})

	// 选择服务器
	serverAddr := f.selectServer()
	f.logger.Info("连接穿透服务器", zap.String("server", serverAddr))

	// 建立连接
	var conn net.Conn
	var err error

	for retry := 0; retry < f.config.MaxRetries; retry++ {
		conn, err = net.DialTimeout("tcp", serverAddr, f.config.Timeout)
		if err == nil {
			break
		}
		f.logger.Warn("连接失败，重试中",
			zap.Error(err),
			zap.Int("retry", retry+1))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(f.config.ReconnectInterval):
		}
	}

	if err != nil {
		f.state = FNCStateError
		f.emitEvent(&FNConnectEvent{
			Type: "error",
			Error: err,
		})
		return fmt.Errorf("连接服务器失败: %w", err)
	}

	f.controlConn = conn

	// 发送认证请求
	if err := f.authenticate(); err != nil {
		conn.Close()
		f.state = FNCStateError
		return fmt.Errorf("认证失败: %w", err)
	}

	f.state = FNCStateConnected
	f.emitEvent(&FNConnectEvent{
		Type: "connected",
	})

	// 启动消息处理
	f.wg.Add(1)
	go f.messageLoop()

	// 启动心跳
	f.wg.Add(1)
	go f.heartbeatLoop()

	// 恢复隧道
	go f.restoreTunnels()

	return nil
}

// selectServer 选择服务器
func (f *FNConnect) selectServer() string {
	if f.config.ServerURL != "" {
		return f.config.ServerURL
	}

	servers, ok := defaultFNCServers[f.config.Region]
	if !ok || len(servers) == 0 {
		servers = defaultFNCServers["cn"]
	}

	// 简单轮询选择
	return servers[0]
}

// authenticate 认证
func (f *FNConnect) authenticate() error {
	authReq := map[string]interface{}{
		"type":       "auth",
		"device_id":  f.config.DeviceID,
		"device_name": f.config.DeviceName,
		"token":      f.config.AuthToken,
		"timestamp":   time.Now().Unix(),
	}

	if err := f.sendMessage(authReq); err != nil {
		return err
	}

	// 等待认证响应
	resp, err := f.readMessage()
	if err != nil {
		return err
	}

	if resp.Type == "auth_resp" {
		var authResp struct {
			Success     bool   `json:"success"`
			PeerID      string `json:"peer_id"`
			PublicURL   string `json:"public_url"`
			AccessToken string `json:"access_token"`
			Message     string `json:"message"`
		}
		if err := json.Unmarshal(resp.Data, &authResp); err != nil {
			return err
		}

		if !authResp.Success {
			return errors.New(authResp.Message)
		}

		f.peerID = authResp.PeerID
		f.publicURL = authResp.PublicURL
		f.accessToken = authResp.AccessToken

		f.logger.Info("认证成功",
			zap.String("peer_id", f.peerID),
			zap.String("public_url", f.publicURL))

		return nil
	}

	return errors.New("invalid auth response")
}

// CreateTunnel 创建隧道
func (f *FNConnect) CreateTunnel(config *FNCTunnel) (*FNCTunnel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.state != FNCStateConnected {
		return nil, errors.New("not connected")
	}

	// 生成隧道ID
	if config.ID == "" {
		config.ID = generateTunnelID()
	}
	config.CreatedAt = time.Now()
	config.State = FNCStateConnecting

	// 发送创建隧道请求
	req := map[string]interface{}{
		"type":        "create_tunnel",
		"tunnel_id":   config.ID,
		"name":        config.Name,
		"protocol":    config.Protocol,
		"local_port":  config.LocalPort,
		"remote_port": config.RemotePort,
		"subdomain":   config.Subdomain,
	}

	if err := f.sendMessage(req); err != nil {
		return nil, err
	}

	// 等待响应
	select {
	case msg := <-f.msgChan:
		if msg.Type == "tunnel_created" {
			var resp struct {
				Success    bool   `json:"success"`
				PublicURL  string `json:"public_url"`
				RemotePort int    `json:"remote_port"`
				Message    string `json:"message"`
			}
			if err := json.Unmarshal(msg.Data, &resp); err != nil {
				return nil, err
			}

			if !resp.Success {
				return nil, errors.New(resp.Message)
			}

			config.PublicURL = resp.PublicURL
			config.RemotePort = resp.RemotePort
			config.State = FNCStateConnected

			f.tunnels[config.ID] = config

			// 启动数据转发
			f.wg.Add(1)
			go f.forwardData(config)

			f.emitEvent(&FNConnectEvent{
				Type:     "tunnel_created",
				TunnelID: config.ID,
				Data:     config,
			})

			return config, nil
		}
	case <-time.After(f.config.Timeout):
		return nil, errors.New("timeout waiting for tunnel creation")
	}

	return nil, errors.New("failed to create tunnel")
}

// DeleteTunnel 删除隧道
func (f *FNConnect) DeleteTunnel(tunnelID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	tunnel, ok := f.tunnels[tunnelID]
	if !ok {
		return errors.New("tunnel not found")
	}

	// 发送删除请求
	req := map[string]interface{}{
		"type":      "delete_tunnel",
		"tunnel_id": tunnelID,
	}

	if err := f.sendMessage(req); err != nil {
		return err
	}

	tunnel.State = FNCStateDisconnected
	delete(f.tunnels, tunnelID)

	f.emitEvent(&FNConnectEvent{
		Type:     "tunnel_deleted",
		TunnelID: tunnelID,
	})

	return nil
}

// forwardData 转发数据
func (f *FNConnect) forwardData(tunnel *FNCTunnel) {
	defer f.wg.Done()

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", tunnel.LocalPort))
	if err != nil {
		f.logger.Error("监听本地端口失败",
			zap.Error(err),
			zap.Int("port", tunnel.LocalPort))
		return
	}
	defer listener.Close()

	for {
		select {
		case <-f.ctx.Done():
			return
		default:
			conn, err := listener.Accept()
			if err != nil {
				continue
			}

			f.wg.Add(1)
			go f.handleConnection(tunnel, conn)
		}
	}
}

// handleConnection 处理连接
func (f *FNConnect) handleConnection(tunnel *FNCTunnel, localConn net.Conn) {
	defer f.wg.Done()
	defer localConn.Close()

	// 通过控制通道请求建立数据通道
	req := map[string]interface{}{
		"type":      "data_channel",
		"tunnel_id": tunnel.ID,
	}

	if err := f.sendMessage(req); err != nil {
		return
	}

	// 等待数据通道建立
	select {
	case msg := <-f.msgChan:
		if msg.Type == "data_channel_ready" {
			// 开始数据转发
			f.proxyData(tunnel, localConn)
		}
	case <-time.After(f.config.Timeout):
		return
	}
}

// proxyData 代理数据
func (f *FNConnect) proxyData(tunnel *FNCTunnel, conn net.Conn) {
	// 实现数据转发逻辑
	buf := make([]byte, 32*1024)
	for {
		select {
		case <-f.ctx.Done():
			return
		default:
			n, err := conn.Read(buf)
			if err != nil {
				return
			}

			f.mu.Lock()
			tunnel.BytesTx += int64(n)
			f.mu.Unlock()
		}
	}
}

// messageLoop 消息循环
func (f *FNConnect) messageLoop() {
	defer f.wg.Done()

	decoder := json.NewDecoder(f.controlConn)
	for {
		select {
		case <-f.ctx.Done():
			return
		default:
			var msg FNCMessage
			if err := decoder.Decode(&msg); err != nil {
				if err == io.EOF {
					f.handleDisconnect()
					return
				}
				continue
			}
			f.msgChan <- &msg
		}
	}
}

// heartbeatLoop 心跳循环
func (f *FNConnect) heartbeatLoop() {
	defer f.wg.Done()

	ticker := time.NewTicker(f.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-f.ctx.Done():
			return
		case <-ticker.C:
			if err := f.sendHeartbeat(); err != nil {
				f.logger.Warn("心跳失败", zap.Error(err))
				f.handleDisconnect()
				return
			}
		}
	}
}

// sendHeartbeat 发送心跳
func (f *FNConnect) sendHeartbeat() error {
	req := map[string]interface{}{
		"type": "heartbeat",
		"timestamp": time.Now().Unix(),
	}
	return f.sendMessage(req)
}

// sendMessage 发送消息
func (f *FNConnect) sendMessage(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = f.controlConn.Write(data)
	return err
}

// readMessage 读取消息
func (f *FNConnect) readMessage() (*FNCMessage, error) {
	decoder := json.NewDecoder(f.controlConn)
	var msg FNCMessage
	if err := decoder.Decode(&msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// handleDisconnect 处理断开
func (f *FNConnect) handleDisconnect() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.state = FNCStateReconnecting
	f.emitEvent(&FNConnectEvent{
		Type: "disconnected",
	})

	// 自动重连
	go func() {
		for retry := 0; retry < f.config.MaxRetries; retry++ {
			select {
			case <-f.ctx.Done():
				return
			case <-time.After(f.config.ReconnectInterval):
				if err := f.Connect(f.ctx); err == nil {
					return
				}
			}
		}
		f.state = FNCStateError
	}()
}

// restoreTunnels 恢复隧道
func (f *FNConnect) restoreTunnels() {
	f.mu.RLock()
	tunnels := make([]*FNCTunnel, 0, len(f.tunnels))
	for _, t := range f.tunnels {
		tunnels = append(tunnels, t)
	}
	f.mu.RUnlock()

	for _, t := range tunnels {
		t.State = FNCStateReconnecting
		_, err := f.CreateTunnel(t)
		if err != nil {
			f.logger.Warn("恢复隧道失败",
				zap.String("tunnel_id", t.ID),
				zap.Error(err))
		}
	}
}

// emitEvent 发送事件
func (f *FNConnect) emitEvent(event *FNConnectEvent) {
	event.Timestamp = time.Now()
	for _, handler := range f.eventHandlers {
		go handler(event)
	}
}

// OnEvent 注册事件处理器
func (f *FNConnect) OnEvent(handler FNConnectEventHandler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.eventHandlers = append(f.eventHandlers, handler)
}

// Disconnect 断开连接
func (f *FNConnect) Disconnect() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.cancel()
	f.wg.Wait()

	if f.controlConn != nil {
		f.controlConn.Close()
	}

	f.state = FNCStateDisconnected
	f.emitEvent(&FNConnectEvent{
		Type: "disconnected",
	})

	return nil
}

// GetStats 获取统计信息
func (f *FNConnect) GetStats() *FNConnectStats {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var totalTx, totalRx int64
	for _, t := range f.tunnels {
		totalTx += t.BytesTx
		totalRx += t.BytesRx
	}

	return &FNConnectStats{
		State:         f.state,
		PublicURL:     f.publicURL,
		ActiveTunnels: len(f.tunnels),
		TotalBytesTx:  totalTx,
		TotalBytesRx:  totalRx,
	}
}

// GetTunnels 获取所有隧道
func (f *FNConnect) GetTunnels() []*FNCTunnel {
	f.mu.RLock()
	defer f.mu.RUnlock()

	tunnels := make([]*FNCTunnel, 0, len(f.tunnels))
	for _, t := range f.tunnels {
		tunnels = append(tunnels, t)
	}
	return tunnels
}

// GetPublicURL 获取公网地址
func (f *FNConnect) GetPublicURL() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.publicURL
}

// IsConnected 检查连接状态
func (f *FNConnect) IsConnected() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.state == FNCStateConnected
}

// 辅助函数
func generateDeviceID() string {
	return fmt.Sprintf("device-%d", time.Now().UnixNano())
}

func generateTunnelID() string {
	return fmt.Sprintf("tunnel-%d", time.Now().UnixNano())
}