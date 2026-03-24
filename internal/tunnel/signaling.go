// Package tunnel 提供内网穿透服务 - 信令服务
package tunnel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// SignalMessageType 信令消息类型
type SignalMessageType string

// 信令消息类型常量
const (
	// SignalMessageTypeOffer Offer消息
	SignalMessageTypeOffer SignalMessageType = "offer"
	// SignalMessageTypeAnswer Answer消息
	SignalMessageTypeAnswer SignalMessageType = "answer"
	// SignalMessageTypeCandidate ICE候选消息
	SignalMessageTypeCandidate SignalMessageType = "candidate"
	// SignalMessageTypeConnected 已连接消息
	SignalMessageTypeConnected SignalMessageType = "connected"
	// SignalMessageTypeDisconnect 断开连接消息
	SignalMessageTypeDisconnect SignalMessageType = "disconnect"
	// SignalMessageTypeError 错误消息
	SignalMessageTypeError SignalMessageType = "error"
)

// SignalMessage 信令消息
type SignalMessage struct {
	Type      SignalMessageType `json:"type"`
	FromID    string            `json:"from_id"`
	ToID      string            `json:"to_id"`
	SessionID string            `json:"session_id,omitempty"`
	Payload   json.RawMessage   `json:"payload,omitempty"`
	Timestamp int64             `json:"timestamp"`
}

// CandidatePayload ICE 候选负载
type CandidatePayload struct {
	Candidate ICECandidate `json:"candidate"`
}

// SessionPayload 会话负载
type SessionPayload struct {
	Session P2PSession `json:"session"`
}

// SignalConfig 信令服务配置
type SignalConfig struct {
	ServerURL      string        `json:"server_url"`
	AuthToken      string        `json:"auth_token"`
	DeviceID       string        `json:"device_id"`
	DeviceName     string        `json:"device_name"`
	ReconnectDelay time.Duration `json:"reconnect_delay"`
	MaxReconnect   int           `json:"max_reconnect"`
	PingInterval   time.Duration `json:"ping_interval"`
	PongWait       time.Duration `json:"pong_wait"`
}

// DefaultSignalConfig 默认信令配置
func DefaultSignalConfig() SignalConfig {
	return SignalConfig{
		ReconnectDelay: 5 * time.Second,
		MaxReconnect:   10,
		PingInterval:   30 * time.Second,
		PongWait:       60 * time.Second,
	}
}

// SignalClient 信令客户端
type SignalClient struct {
	config       SignalConfig
	logger       *zap.Logger
	conn         *websocket.Conn
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	connected    bool
	onMessage    func(msg SignalMessage)
	onConnect    func()
	onDisconnect func()
	sessionID    string
}

// NewSignalClient 创建信令客户端
func NewSignalClient(config SignalConfig, logger *zap.Logger) *SignalClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &SignalClient{
		config: config,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Connect 连接到信令服务器
func (s *SignalClient) Connect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.connected {
		return errors.New("already connected")
	}

	// 构建 WebSocket URL
	url := s.config.ServerURL
	if len(url) > 0 && url[0] != 'w' {
		url = "wss://" + url
	}

	// 创建连接头
	headers := make(map[string][]string)
	if s.config.AuthToken != "" {
		headers["Authorization"] = []string{"Bearer " + s.config.AuthToken}
	}
	if s.config.DeviceID != "" {
		headers["X-Device-ID"] = []string{s.config.DeviceID}
	}

	// 建立连接
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	conn, resp, err := dialer.DialContext(ctx, url, headers)
	if err != nil {
		return fmt.Errorf("failed to connect to signal server: %w", err)
	}
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	s.conn = conn
	s.connected = true

	// 发送注册消息
	if err := s.sendRegister(); err != nil {
		_ = conn.Close()
		s.connected = false
		return err
	}

	// 启动消息处理
	go s.readLoop()
	go s.pingLoop()

	if s.onConnect != nil {
		go s.onConnect()
	}

	return nil
}

// Disconnect 断开连接
func (s *SignalClient) Disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.connected {
		return nil
	}

	// 发送断开消息
	_ = s.send(SignalMessage{
		Type: SignalMessageTypeDisconnect,
	})

	if s.conn != nil {
		_ = s.conn.Close()
	}

	s.connected = false

	if s.onDisconnect != nil {
		go s.onDisconnect()
	}

	return nil
}

// sendRegister 发送注册消息
func (s *SignalClient) sendRegister() error {
	payload, _ := json.Marshal(map[string]string{
		"device_id":   s.config.DeviceID,
		"device_name": s.config.DeviceName,
	})

	return s.send(SignalMessage{
		Type:    "register",
		FromID:  s.config.DeviceID,
		Payload: payload,
	})
}

// send 发送消息
func (s *SignalClient) send(msg SignalMessage) error {
	if s.conn == nil {
		return errors.New("not connected")
	}

	msg.Timestamp = time.Now().Unix()
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return s.conn.WriteMessage(websocket.TextMessage, data)
}

// readLoop 读取循环
func (s *SignalClient) readLoop() {
	defer func() {
		s.mu.Lock()
		s.connected = false
		if s.conn != nil {
			_ = s.conn.Close()
		}
		s.mu.Unlock()

		if s.onDisconnect != nil {
			go s.onDisconnect()
		}
	}()

	s.conn.SetPongHandler(func(string) error {
		_ = s.conn.SetReadDeadline(time.Now().Add(s.config.PongWait))
		return nil
	})

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		_ = s.conn.SetReadDeadline(time.Now().Add(s.config.PongWait))

		_, data, err := s.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				s.logger.Debug("connection closed", zap.Error(err))
			}
			return
		}

		var msg SignalMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			s.logger.Debug("failed to parse message", zap.Error(err))
			continue
		}

		if s.onMessage != nil {
			go s.onMessage(msg)
		}
	}
}

// pingLoop 心跳循环
func (s *SignalClient) pingLoop() {
	ticker := time.NewTicker(s.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.mu.RLock()
			conn := s.conn
			s.mu.RUnlock()

			if conn != nil {
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					s.logger.Debug("ping failed", zap.Error(err))
					return
				}
			}
		}
	}
}

// SendOffer 发送 Offer
func (s *SignalClient) SendOffer(toID string, session *P2PSession) error {
	payload, err := json.Marshal(SessionPayload{Session: *session})
	if err != nil {
		return err
	}

	return s.send(SignalMessage{
		Type:      SignalMessageTypeOffer,
		FromID:    s.config.DeviceID,
		ToID:      toID,
		SessionID: s.sessionID,
		Payload:   payload,
	})
}

// SendAnswer 发送 Answer
func (s *SignalClient) SendAnswer(toID string, session *P2PSession) error {
	payload, err := json.Marshal(SessionPayload{Session: *session})
	if err != nil {
		return err
	}

	return s.send(SignalMessage{
		Type:      SignalMessageTypeAnswer,
		FromID:    s.config.DeviceID,
		ToID:      toID,
		SessionID: s.sessionID,
		Payload:   payload,
	})
}

// SendCandidate 发送 ICE 候选
func (s *SignalClient) SendCandidate(toID string, candidate ICECandidate) error {
	payload, err := json.Marshal(CandidatePayload{Candidate: candidate})
	if err != nil {
		return err
	}

	return s.send(SignalMessage{
		Type:      SignalMessageTypeCandidate,
		FromID:    s.config.DeviceID,
		ToID:      toID,
		SessionID: s.sessionID,
		Payload:   payload,
	})
}

// SendConnected 发送已连接通知
func (s *SignalClient) SendConnected(toID string) error {
	return s.send(SignalMessage{
		Type:      SignalMessageTypeConnected,
		FromID:    s.config.DeviceID,
		ToID:      toID,
		SessionID: s.sessionID,
	})
}

// OnMessage 设置消息回调
func (s *SignalClient) OnMessage(callback func(msg SignalMessage)) {
	s.onMessage = callback
}

// OnConnect 设置连接回调
func (s *SignalClient) OnConnect(callback func()) {
	s.onConnect = callback
}

// OnDisconnect 设置断开回调
func (s *SignalClient) OnDisconnect(callback func()) {
	s.onDisconnect = callback
}

// IsConnected 检查是否已连接
func (s *SignalClient) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connected
}

// SetSessionID 设置会话 ID
func (s *SignalClient) SetSessionID(id string) {
	s.sessionID = id
}

// SignalServer 信令服务器
type SignalServer struct {
	addr     string
	logger   *zap.Logger
	server   net.Listener
	upgrader websocket.Upgrader
	clients  map[string]*SignalClientConn
	rooms    map[string]map[string]bool // session -> device IDs
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
}

// SignalClientConn 信令服务器客户端连接
type SignalClientConn struct {
	conn       *websocket.Conn
	deviceID   string
	deviceName string
	sessionID  string
	sendCh     chan SignalMessage
}

// NewSignalServer 创建信令服务器
func NewSignalServer(addr string, logger *zap.Logger) *SignalServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &SignalServer{
		addr:     addr,
		logger:   logger,
		upgrader: websocket.Upgrader{},
		clients:  make(map[string]*SignalClientConn),
		rooms:    make(map[string]map[string]bool),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start 启动信令服务器
func (s *SignalServer) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.server = listener
	s.logger.Info("signal server started", zap.String("addr", s.addr))

	go s.acceptLoop()

	return nil
}

// Stop 停止信令服务器
func (s *SignalServer) Stop() error {
	s.cancel()
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// acceptLoop 接受连接循环
func (s *SignalServer) acceptLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		conn, err := s.server.Accept()
		if err != nil {
			if s.ctx.Err() == nil {
				s.logger.Debug("accept error", zap.Error(err))
			}
			continue
		}

		go s.handleConnection(conn)
	}
}

// handleConnection 处理 WebSocket 连接
func (s *SignalServer) handleConnection(conn net.Conn) {
	// 简化实现：直接升级为 WebSocket
	// 实际实现需要 HTTP 升级
	wsConn := &SignalClientConn{
		conn:   &websocket.Conn{},
		sendCh: make(chan SignalMessage, 10),
	}

	defer func() {
		if wsConn.deviceID != "" {
			s.mu.Lock()
			delete(s.clients, wsConn.deviceID)
			if wsConn.sessionID != "" {
				if room, ok := s.rooms[wsConn.sessionID]; ok {
					delete(room, wsConn.deviceID)
				}
			}
			s.mu.Unlock()
		}
	}()

	// 处理消息
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		var msg SignalMessage
		if err := wsConn.conn.ReadJSON(&msg); err != nil {
			return
		}

		s.handleMessage(wsConn, msg)
	}
}

// handleMessage 处理信令消息
func (s *SignalServer) handleMessage(client *SignalClientConn, msg SignalMessage) {
	switch msg.Type {
	case "register":
		// 注册设备
		var payload struct {
			DeviceID   string `json:"device_id"`
			DeviceName string `json:"device_name"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}

		client.deviceID = payload.DeviceID
		client.deviceName = payload.DeviceName

		s.mu.Lock()
		s.clients[client.deviceID] = client
		s.mu.Unlock()

		s.logger.Debug("device registered",
			zap.String("device_id", client.deviceID),
			zap.String("device_name", client.deviceName),
		)

	case SignalMessageTypeOffer, SignalMessageTypeAnswer, SignalMessageTypeCandidate, SignalMessageTypeConnected:
		// 转发消息到目标设备
		s.mu.RLock()
		target, ok := s.clients[msg.ToID]
		s.mu.RUnlock()

		if ok {
			msg.FromID = client.deviceID
			select {
			case target.sendCh <- msg:
			default:
				s.logger.Debug("target client buffer full", zap.String("target", msg.ToID))
			}
		}

	case SignalMessageTypeDisconnect:
		// 处理断开
		s.mu.Lock()
		delete(s.clients, client.deviceID)
		s.mu.Unlock()
	}
}
