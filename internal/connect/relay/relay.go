// Package relay provides relay mode connection for NAT traversal
// WebSocket relay implementation
package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// 错误定义
var (
	ErrRelayNotConnected = errors.New("relay not connected")
	ErrRelayAuthFailed   = errors.New("relay authentication failed")
	ErrRelayTimeout      = errors.New("relay connection timeout")
)

// RelayConfig 中继配置
type RelayConfig struct {
	ServerURL    string        `json:"server_url"`
	Token        string        `json:"token"`
	DeviceID     string        `json:"device_id"`
	DeviceName   string        `json:"device_name"`
	TLSEnabled   bool          `json:"tls_enabled"`
	Timeout      time.Duration `json:"timeout"`
	PingInterval time.Duration `json:"ping_interval"`
}

// RelayClient 中继客户端
type RelayClient struct {
	config     *RelayConfig
	conn       *websocket.Conn
	status     string
	deviceID   string
	publicURL  string
	stats      *RelayStats
	logger     *zap.Logger
	
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	
	// 数据通道
	tunnelData chan TunnelData
	
	// 回调
	onConnect    func()
	onDisconnect func(error)
	onData       func(TunnelData)
}

// RelayStats 中继统计
type RelayStats struct {
	BytesSent     uint64    `json:"bytes_sent"`
	BytesReceived uint64    `json:"bytes_received"`
	Connections   int       `json:"connections"`
	LastActivity  time.Time `json:"last_activity"`
	Latency       int       `json:"latency_ms"`
	ConnectedAt   time.Time `json:"connected_at"`
}

// TunnelData 隧道数据
type TunnelData struct {
	TunnelID string `json:"tunnel_id"`
	Data     []byte `json:"data"`
}

// RelayMessage 中继消息
type RelayMessage struct {
	Type    string          `json:"type"` // auth, ping, pong, data, tunnel
	Payload json.RawMessage `json:"payload"`
}

// AuthPayload 认证载荷
type AuthPayload struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Token      string `json:"token"`
	Version    string `json:"version"`
}

// TunnelPayload 隧道载荷
type TunnelPayload struct {
	TunnelID string `json:"tunnel_id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	LocalIP  string `json:"local_ip"`
	LocalPort int   `json:"local_port"`
}

// NewRelayClient 创建中继客户端
func NewRelayClient(config *RelayConfig, logger *zap.Logger) (*RelayClient, error) {
	if config.ServerURL == "" {
		return nil, errors.New("relay server URL required")
	}
	
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	
	if config.PingInterval == 0 {
		config.PingInterval = 30 * time.Second
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &RelayClient{
		config:     config,
		status:     "disconnected",
		stats:      &RelayStats{},
		tunnelData: make(chan TunnelData, 100),
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

// Connect 连接到中继服务器
func (r *RelayClient) Connect() error {
	r.mu.Lock()
	if r.status == "connected" {
		r.mu.Unlock()
		return nil
	}
	r.status = "connecting"
	r.mu.Unlock()
	
	// WebSocket 连接
	dialer := websocket.Dialer{
		HandshakeTimeout: r.config.Timeout,
	}
	
	wsURL := r.config.ServerURL
	if !r.config.TLSEnabled {
		// 替换 ws/wss
		if len(wsURL) > 6 && wsURL[:6] == "wss://" {
			wsURL = "ws://" + wsURL[6:]
		}
	}
	
	conn, _, err := dialer.Dial(wsURL+"/relay", nil)
	if err != nil {
		r.mu.Lock()
		r.status = "error"
		r.mu.Unlock()
		return fmt.Errorf("%w: %v", ErrRelayNotConnected, err)
	}
	
	r.mu.Lock()
	r.conn = conn
	r.mu.Unlock()
	
	// 发送认证
	if err := r.authenticate(); err != nil {
		_ = conn.Close()
		r.mu.Lock()
		r.status = "error"
		r.mu.Unlock()
		return err
	}
	
	r.mu.Lock()
	r.status = "connected"
	r.stats.ConnectedAt = time.Now()
	r.mu.Unlock()
	
	// 启动监听循环
	r.wg.Add(1)
	go r.readLoop()
	
	// 启动心跳
	r.wg.Add(1)
	go r.pingLoop()
	
	if r.onConnect != nil {
		r.onConnect()
	}
	
	r.logger.Info("Relay connected",
		zap.String("device_id", r.deviceID),
		zap.String("server", r.config.ServerURL))
	
	return nil
}

// authenticate 认证
func (r *RelayClient) authenticate() error {
	auth := RelayMessage{
		Type: "auth",
		Payload: mustMarshal(AuthPayload{
			DeviceID:   r.config.DeviceID,
			DeviceName: r.config.DeviceName,
			Token:      r.config.Token,
			Version:    "1.0",
		}),
	}
	
	if err := r.conn.WriteJSON(auth); err != nil {
		return fmt.Errorf("%w: failed to send auth", ErrRelayAuthFailed)
	}
	
	// 读取响应
	var resp RelayMessage
	if err := r.conn.ReadJSON(&resp); err != nil {
		return fmt.Errorf("%w: failed to read auth response", ErrRelayAuthFailed)
	}
	
	if resp.Type != "auth_resp" {
		return ErrRelayAuthFailed
	}
	
	var authResp struct {
		Success   bool   `json:"success"`
		DeviceID  string `json:"device_id"`
		PublicURL string `json:"public_url"`
		Error     string `json:"error,omitempty"`
	}
	
	if err := json.Unmarshal(resp.Payload, &authResp); err != nil {
		return ErrRelayAuthFailed
	}
	
	if !authResp.Success {
		return fmt.Errorf("%w: %s", ErrRelayAuthFailed, authResp.Error)
	}
	
	r.deviceID = authResp.DeviceID
	r.publicURL = authResp.PublicURL
	
	return nil
}

// readLoop 读取循环
func (r *RelayClient) readLoop() {
	defer r.wg.Done()
	defer r.conn.Close()
	
	for {
		select {
		case <-r.ctx.Done():
			return
		default:
			var msg RelayMessage
			if err := r.conn.ReadJSON(&msg); err != nil {
				if r.ctx.Err() != nil {
					return
				}
				r.logger.Debug("Relay read error", zap.Error(err))
				r.handleDisconnect(err)
				return
			}
			
			r.handleMessage(msg)
		}
	}
}

// handleMessage 处理消息
func (r *RelayClient) handleMessage(msg RelayMessage) {
	r.mu.Lock()
	r.stats.LastActivity = time.Now()
	r.mu.Unlock()
	
	switch msg.Type {
	case "pong":
		// 心跳响应
		r.mu.Lock()
		r.stats.Latency = 0 // 已更新
		r.mu.Unlock()
		
	case "data":
		// 数据消息
		var data TunnelData
		if err := json.Unmarshal(msg.Payload, &data); err == nil {
			r.mu.Lock()
			r.stats.BytesReceived += uint64(len(data.Data))
			r.mu.Unlock()
			
			if r.onData != nil {
				r.onData(data)
			}
			
			// 发送到数据通道
			select {
			case r.tunnelData <- data:
			default:
				r.logger.Debug("Tunnel data channel full")
			}
		}
		
	case "tunnel_resp":
		// 隧道响应
		var resp struct {
			TunnelID string `json:"tunnel_id"`
			Success  bool   `json:"success"`
			Error    string `json:"error,omitempty"`
		}
		if err := json.Unmarshal(msg.Payload, &resp); err == nil {
			r.logger.Debug("Tunnel response",
				zap.String("tunnel_id", resp.TunnelID),
				zap.Bool("success", resp.Success))
		}
		
	case "close":
		// 服务器关闭通知
		r.handleDisconnect(errors.New("server closed connection"))
	}
}

// pingLoop 心跳循环
func (r *RelayClient) pingLoop() {
	defer r.wg.Done()
	
	ticker := time.NewTicker(r.config.PingInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.mu.RLock()
			conn := r.conn
			r.mu.RUnlock()
			
			if conn == nil {
				continue
			}
			
			ping := RelayMessage{
				Type:    "ping",
				Payload: []byte{},
			}
			
			start := time.Now()
			if err := conn.WriteJSON(ping); err != nil {
				r.logger.Debug("Relay ping failed", zap.Error(err))
				r.handleDisconnect(err)
				return
			}
			
			r.mu.Lock()
			r.stats.Latency = int(time.Since(start).Milliseconds())
			r.mu.Unlock()
		}
	}
}

// handleDisconnect 处理断开
func (r *RelayClient) handleDisconnect(err error) {
	r.mu.Lock()
	if r.status == "disconnected" {
		r.mu.Unlock()
		return
	}
	r.status = "disconnected"
	if r.conn != nil {
		_ = r.conn.Close()
		r.conn = nil
	}
	r.mu.Unlock()
	
	if r.onDisconnect != nil {
		r.onDisconnect(err)
	}
	
	r.logger.Info("Relay disconnected", zap.Error(err))
}

// SendData 发送数据
func (r *RelayClient) SendData(data TunnelData) error {
	r.mu.RLock()
	conn := r.conn
	r.mu.RUnlock()
	
	if conn == nil {
		return ErrRelayNotConnected
	}
	
	msg := RelayMessage{
		Type:    "data",
		Payload: mustMarshal(data),
	}
	
	if err := conn.WriteJSON(msg); err != nil {
		return err
	}
	
	r.mu.Lock()
	r.stats.BytesSent += uint64(len(data.Data))
	r.stats.LastActivity = time.Now()
	r.mu.Unlock()
	
	return nil
}

// CreateTunnel 创建隧道
func (r *RelayClient) CreateTunnel(tunnel TunnelPayload) error {
	r.mu.RLock()
	conn := r.conn
	r.mu.RUnlock()
	
	if conn == nil {
		return ErrRelayNotConnected
	}
	
	msg := RelayMessage{
		Type:    "tunnel",
		Payload: mustMarshal(tunnel),
	}
	
	return conn.WriteJSON(msg)
}

// Disconnect 断开连接
func (r *RelayClient) Disconnect() error {
	r.cancel()
	r.wg.Wait()
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.conn != nil {
		_ = r.conn.Close()
		r.conn = nil
	}
	
	r.status = "disconnected"
	
	return nil
}

// GetStatus 获取状态
func (r *RelayClient) GetStatus() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}

// GetStats 获取统计
func (r *RelayClient) GetStats() RelayStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return *r.stats
}

// GetPublicURL 获取公网URL
func (r *RelayClient) GetPublicURL() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.publicURL
}

// GetDeviceID 获取设备ID
func (r *RelayClient) GetDeviceID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.deviceID
}

// SetOnConnect 设置连接回调
func (r *RelayClient) SetOnConnect(fn func()) {
	r.mu.Lock()
	r.onConnect = fn
	r.mu.Unlock()
}

// SetOnDisconnect 设置断开回调
func (r *RelayClient) SetOnDisconnect(fn func(error)) {
	r.mu.Lock()
	r.onDisconnect = fn
	r.mu.Unlock()
}

// SetOnData 设置数据回调
func (r *RelayClient) SetOnData(fn func(TunnelData)) {
	r.mu.Lock()
	r.onData = fn
	r.mu.Unlock()
}

// mustMarshal 必须成功的序列化
func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

// DialLocalService 连接本地服务
func (r *RelayClient) DialLocalService(localIP string, localPort int) (net.Conn, error) {
	addr := net.JoinHostPort(localIP, strconv.Itoa(localPort))
	return net.DialTimeout("tcp", addr, 10*time.Second)
}

// ForwardLocalData 转发数据到本地
func (r *RelayClient) ForwardLocalData(tunnelID, localIP string, localPort int, data []byte) error {
	conn, err := r.DialLocalService(localIP, localPort)
	if err != nil {
		return err
	}
	defer conn.Close()
	
	// 发送数据
	if _, err := conn.Write(data); err != nil {
		return err
	}
	
	// 读取响应
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		return err
	}
	
	if n > 0 {
		// 发送响应回中继
		return r.SendData(TunnelData{
			TunnelID: tunnelID,
			Data:     buf[:n],
		})
	}
	
	return nil
}