// Package api provides enhanced WebSocket with heartbeat and reconnection support
// Version: 2.33.0 - WebSocket heartbeat and reconnection mechanism
package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var (
	ErrConnectionClosed   = errors.New("connection closed")
	ErrReconnectFailed    = errors.New("reconnection failed")
	ErrMaxReconnect       = errors.New("max reconnection attempts reached")
	ErrHeartbeatTimeout   = errors.New("heartbeat timeout")
	ErrConnectionNotFound = errors.New("connection not found")
)

// HeartbeatConfig 心跳配置
type HeartbeatConfig struct {
	// PingInterval 发送 ping 的间隔
	PingInterval time.Duration
	// PongTimeout 等待 pong 响应的超时时间
	PongTimeout time.Duration
	// MaxMissedPongs 最大未响应 pong 次数
	MaxMissedPongs int
	// EnableServerHeartbeat 是否启用服务端主动心跳
	EnableServerHeartbeat bool
}

// DefaultHeartbeatConfig 默认心跳配置
var DefaultHeartbeatConfig = HeartbeatConfig{
	PingInterval:           30 * time.Second,
	PongTimeout:            10 * time.Second,
	MaxMissedPongs:         3,
	EnableServerHeartbeat:  true,
}

// ReconnectConfig 重连配置
type ReconnectConfig struct {
	// Enable 是否启用自动重连
	Enable bool
	// MaxAttempts 最大重连次数（0 表示无限）
	MaxAttempts int
	// InitialDelay 初始重连延迟
	InitialDelay time.Duration
	// MaxDelay 最大重连延迟
	MaxDelay time.Duration
	// BackoffFactor 退避因子
	BackoffFactor float64
	// ResetAfter 连接稳定后重置重连计数的时间
	ResetAfter time.Duration
}

// DefaultReconnectConfig 默认重连配置
var DefaultReconnectConfig = ReconnectConfig{
	Enable:        true,
	MaxAttempts:   5,
	InitialDelay:  1 * time.Second,
	MaxDelay:      30 * time.Second,
	BackoffFactor: 2.0,
	ResetAfter:    60 * time.Second,
}

// ConnectionState 连接状态
type ConnectionState int32

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateReconnecting
	StateClosing
)

func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateReconnecting:
		return "reconnecting"
	case StateClosing:
		return "closing"
	default:
		return "unknown"
	}
}

// ConnectionStats 连接统计
type ConnectionStats struct {
	ID               string          `json:"id"`
	UserID           string          `json:"userId"`
	State            string          `json:"state"`
	ConnectedAt      time.Time       `json:"connectedAt"`
	LastActivity     time.Time       `json:"lastActivity"`
	ReconnectCount   int             `json:"reconnectCount"`
	MessagesSent     int64           `json:"messagesSent"`
	MessagesReceived int64           `json:"messagesReceived"`
	BytesSent        int64           `json:"bytesSent"`
	BytesReceived    int64           `json:"bytesReceived"`
	MissedHeartbeats int             `json:"missedHeartbeats"`
}

// EnhancedClient 增强版 WebSocket 客户端
type EnhancedClient struct {
	ID             string
	UserID         string
	Connection     *websocket.Conn
	Send           chan []byte
	Subscriptions  map[MessageType]bool
	ConnectedAt    time.Time
	LastActivity   time.Time

	// 状态管理
	state            int32 // atomic: ConnectionState
	reconnectCount   int32
	messagesSent     int64
	messagesReceived int64
	bytesSent        int64
	bytesReceived    int64
	missedHeartbeats int32

	// 心跳
	heartbeatConfig HeartbeatConfig
	pongChan        chan struct{}
	missedPongs     int

	// 重连
	reconnectConfig ReconnectConfig
	reconnectTimer  *time.Timer
	stableTimer     *time.Timer

	// 控制
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.RWMutex
	wg        sync.WaitGroup
	closeOnce sync.Once // 确保 Close 只执行一次
	closed    bool      // 标记是否已关闭
}

// EnhancedWebSocketHub 增强版 WebSocket Hub
type EnhancedWebSocketHub struct {
	clients         map[string]*EnhancedClient
	broadcast       chan *WebSocketMessage
	register        chan *EnhancedClient
	unregister      chan *EnhancedClient
	statusChange    chan *ClientStatusChange
	mu              sync.RWMutex

	// 配置
	heartbeatConfig  HeartbeatConfig
	reconnectConfig  ReconnectConfig

	// 统计
	totalConnections  int64
	totalMessages     int64
	totalBroadcasts   int64

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
}

// ClientStatusChange 客户端状态变化
type ClientStatusChange struct {
	ClientID string
	OldState ConnectionState
	NewState ConnectionState
	Error    error
}

// NewEnhancedWebSocketHub 创建增强版 WebSocket Hub
func NewEnhancedWebSocketHub(heartbeat HeartbeatConfig, reconnect ReconnectConfig) *EnhancedWebSocketHub {
	ctx, cancel := context.WithCancel(context.Background())
	return &EnhancedWebSocketHub{
		clients:         make(map[string]*EnhancedClient),
		broadcast:       make(chan *WebSocketMessage, 512),
		register:        make(chan *EnhancedClient, 64),
		unregister:      make(chan *EnhancedClient, 64),
		statusChange:    make(chan *ClientStatusChange, 128),
		heartbeatConfig: heartbeat,
		reconnectConfig: reconnect,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Run 启动 Hub
func (h *EnhancedWebSocketHub) Run() {
	// 启动状态监控
	go h.monitorStatus()

	for {
		select {
		case <-h.ctx.Done():
			h.closeAllClients()
			return

		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case status := <-h.statusChange:
			h.handleStatusChange(status)

		case message := <-h.broadcast:
			h.broadcastMessage(message)
		}
	}
}

// registerClient 注册客户端
func (h *EnhancedWebSocketHub) registerClient(client *EnhancedClient) {
	h.mu.Lock()
	h.clients[client.ID] = client
	h.totalConnections++
	h.mu.Unlock()

	client.setState(StateConnected)
	log.Printf("[WebSocket] Client connected: %s (total: %d)", client.ID, len(h.clients))
}

// unregisterClient 注销客户端
func (h *EnhancedWebSocketHub) unregisterClient(client *EnhancedClient) {
	h.mu.Lock()
	if _, exists := h.clients[client.ID]; exists {
		delete(h.clients, client.ID)
	}
	h.mu.Unlock()

	client.setState(StateDisconnected)
	log.Printf("[WebSocket] Client disconnected: %s (total: %d)", client.ID, len(h.clients))
}

// handleStatusChange 处理状态变化
func (h *EnhancedWebSocketHub) handleStatusChange(status *ClientStatusChange) {
	log.Printf("[WebSocket] Client %s state: %s -> %s", 
		status.ClientID, status.OldState, status.NewState)
	
	if status.Error != nil {
		log.Printf("[WebSocket] Client %s error: %v", status.ClientID, status.Error)
	}
}

// broadcastMessage 广播消息
func (h *EnhancedWebSocketHub) broadcastMessage(message *WebSocketMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	data, err := json.Marshal(message)
	if err != nil {
		return
	}

	h.totalBroadcasts++
	for _, client := range h.clients {
		if len(client.Subscriptions) == 0 || client.Subscriptions[message.Type] {
			select {
			case client.Send <- data:
				atomic.AddInt64(&client.messagesSent, 1)
				atomic.AddInt64(&client.bytesSent, int64(len(data)))
			default:
				// Buffer full, skip
			}
		}
	}
}

// closeAllClients 关闭所有客户端
func (h *EnhancedWebSocketHub) closeAllClients() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, client := range h.clients {
		client.Close()
	}
	h.clients = make(map[string]*EnhancedClient)
}

// monitorStatus 监控状态
func (h *EnhancedWebSocketHub) monitorStatus() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.logStats()
		}
	}
}

// logStats 记录统计信息
func (h *EnhancedWebSocketHub) logStats() {
	h.mu.RLock()
	count := len(h.clients)
	h.mu.RUnlock()

	if count > 0 {
		log.Printf("[WebSocket] Stats - Clients: %d, Total Connections: %d, Messages: %d, Broadcasts: %d",
			count, 
			atomic.LoadInt64(&h.totalConnections),
			atomic.LoadInt64(&h.totalMessages),
			atomic.LoadInt64(&h.totalBroadcasts))
	}
}

// Broadcast 广播消息
func (h *EnhancedWebSocketHub) Broadcast(msgType MessageType, data interface{}) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	msg := &WebSocketMessage{
		Type:      msgType,
		Timestamp: time.Now().Unix(),
		Data:      dataBytes,
	}

	select {
	case h.broadcast <- msg:
		return nil
	default:
		return errors.New("broadcast channel full")
	}
}

// BroadcastToUser 向特定用户广播
func (h *EnhancedWebSocketHub) BroadcastToUser(userID string, msgType MessageType, data interface{}) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	msg := &WebSocketMessage{
		Type:      msgType,
		Timestamp: time.Now().Unix(),
		Data:      dataBytes,
	}
	msgBytes, _ := json.Marshal(msg)

	for _, client := range h.clients {
		if client.UserID == userID {
			select {
			case client.Send <- msgBytes:
				atomic.AddInt64(&client.messagesSent, 1)
			default:
			}
		}
	}
	return nil
}

// GetClientCount 获取客户端数量
func (h *EnhancedWebSocketHub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetClientStats 获取客户端统计
func (h *EnhancedWebSocketHub) GetClientStats(clientID string) (*ConnectionStats, error) {
	h.mu.RLock()
	client, exists := h.clients[clientID]
	h.mu.RUnlock()

	if !exists {
		return nil, ErrConnectionNotFound
	}

	return client.GetStats(), nil
}

// GetAllClientStats 获取所有客户端统计
func (h *EnhancedWebSocketHub) GetAllClientStats() []*ConnectionStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stats := make([]*ConnectionStats, 0, len(h.clients))
	for _, client := range h.clients {
		stats = append(stats, client.GetStats())
	}
	return stats
}

// Stop 停止 Hub
func (h *EnhancedWebSocketHub) Stop() {
	h.cancel()
}

// NewEnhancedClient 创建增强版客户端
func NewEnhancedClient(conn *websocket.Conn, userID string, heartbeat HeartbeatConfig, reconnect ReconnectConfig) *EnhancedClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &EnhancedClient{
		ID:              generateSecureID(),
		UserID:          userID,
		Connection:      conn,
		Send:            make(chan []byte, 512),
		Subscriptions:   make(map[MessageType]bool),
		ConnectedAt:     time.Now(),
		LastActivity:    time.Now(),
		state:           int32(StateConnecting),
		heartbeatConfig: heartbeat,
		pongChan:        make(chan struct{}, 1),
		reconnectConfig: reconnect,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// setState 设置状态
func (c *EnhancedClient) setState(state ConnectionState) {
	atomic.StoreInt32(&c.state, int32(state))
}

// GetState 获取状态
func (c *EnhancedClient) GetState() ConnectionState {
	return ConnectionState(atomic.LoadInt32(&c.state))
}

// GetStats 获取统计信息
func (c *EnhancedClient) GetStats() *ConnectionStats {
	return &ConnectionStats{
		ID:               c.ID,
		UserID:           c.UserID,
		State:            c.GetState().String(),
		ConnectedAt:      c.ConnectedAt,
		LastActivity:     c.LastActivity,
		ReconnectCount:   int(atomic.LoadInt32(&c.reconnectCount)),
		MessagesSent:     atomic.LoadInt64(&c.messagesSent),
		MessagesReceived: atomic.LoadInt64(&c.messagesReceived),
		BytesSent:        atomic.LoadInt64(&c.bytesSent),
		BytesReceived:    atomic.LoadInt64(&c.bytesReceived),
		MissedHeartbeats: int(atomic.LoadInt32(&c.missedHeartbeats)),
	}
}

// StartPumps 启动读写泵
func (c *EnhancedClient) StartPumps(hub *EnhancedWebSocketHub) {
	c.wg.Add(3)
	go c.writePump()
	go c.readPump(hub)
	go c.heartbeatPump(hub)
}

// readPump 读泵
func (c *EnhancedClient) readPump(hub *EnhancedWebSocketHub) {
	defer func() {
		c.wg.Done()
		hub.unregister <- c
		c.Close()
	}()

	c.Connection.SetReadLimit(65536)
	c.Connection.SetReadDeadline(time.Now().Add(c.heartbeatConfig.PongTimeout + c.heartbeatConfig.PingInterval))
	
	// Pong 处理器
	c.Connection.SetPongHandler(func(string) error {
		c.Connection.SetReadDeadline(time.Now().Add(c.heartbeatConfig.PongTimeout + c.heartbeatConfig.PingInterval))
		c.missedPongs = 0
		atomic.StoreInt32(&c.missedHeartbeats, 0)
		c.LastActivity = time.Now()
		
		select {
		case c.pongChan <- struct{}{}:
		default:
		}
		return nil
	})

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			messageType, message, err := c.Connection.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("[WebSocket] Read error for %s: %v", c.ID, err)
				}
				return
			}

			atomic.AddInt64(&c.messagesReceived, 1)
			atomic.AddInt64(&c.bytesReceived, int64(len(message)))
			c.LastActivity = time.Now()

			// 处理消息
			c.handleMessage(messageType, message, hub)
		}
	}
}

// writePump 写泵
func (c *EnhancedClient) writePump() {
	defer func() {
		c.wg.Done()
		c.Close()
	}()

	for {
		select {
		case <-c.ctx.Done():
			c.sendCloseMessage()
			return

		case message, ok := <-c.Send:
			if !ok {
				c.sendCloseMessage()
				return
			}

			c.Connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Connection.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

			// 批量发送
			for i := 0; i < len(c.Send); i++ {
				msg := <-c.Send
				c.Connection.WriteMessage(websocket.TextMessage, msg)
			}
		}
	}
}

// heartbeatPump 心跳泵
func (c *EnhancedClient) heartbeatPump(hub *EnhancedWebSocketHub) {
	defer func() {
		c.wg.Done()
	}()

	if !c.heartbeatConfig.EnableServerHeartbeat {
		return
	}

	ticker := time.NewTicker(c.heartbeatConfig.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return

		case <-ticker.C:
			if err := c.sendPing(); err != nil {
				c.missedPongs++
				atomic.AddInt32(&c.missedHeartbeats, 1)

				if c.missedPongs >= c.heartbeatConfig.MaxMissedPongs {
					log.Printf("[WebSocket] Client %s heartbeat timeout after %d missed pongs", 
						c.ID, c.missedPongs)
					hub.statusChange <- &ClientStatusChange{
						ClientID: c.ID,
						OldState: StateConnected,
						NewState: StateDisconnected,
						Error:    ErrHeartbeatTimeout,
					}
					c.Close()
					return
				}
			} else {
				c.missedPongs = 0
			}

		case <-c.pongChan:
			// Pong received, reset missed count
			c.missedPongs = 0
		}
	}
}

// sendPing 发送 ping
func (c *EnhancedClient) sendPing() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Connection == nil {
		return ErrConnectionClosed
	}

	c.Connection.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return c.Connection.WriteMessage(websocket.PingMessage, nil)
}

// sendCloseMessage 发送关闭消息
func (c *EnhancedClient) sendCloseMessage() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Connection != nil {
		c.Connection.SetWriteDeadline(time.Now().Add(5 * time.Second))
		c.Connection.WriteMessage(websocket.CloseMessage, 
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}
}

// handleMessage 处理消息
func (c *EnhancedClient) handleMessage(messageType int, message []byte, hub *EnhancedWebSocketHub) {
	var msg struct {
		Action        string        `json:"action"`
		Subscriptions []MessageType `json:"subscriptions,omitempty"`
		Data          json.RawMessage `json:"data,omitempty"`
	}

	if err := json.Unmarshal(message, &msg); err != nil {
		return
	}

	switch msg.Action {
	case "subscribe":
		c.mu.Lock()
		for _, t := range msg.Subscriptions {
			c.Subscriptions[t] = true
		}
		c.mu.Unlock()

	case "unsubscribe":
		c.mu.Lock()
		for _, t := range msg.Subscriptions {
			delete(c.Subscriptions, t)
		}
		c.mu.Unlock()

	case "ping":
		// 响应 pong
		response, _ := json.Marshal(&WebSocketMessage{
			Type:      MessageTypeSystem,
			Timestamp: time.Now().Unix(),
			Data:      json.RawMessage(`{"pong":true,"time":"` + time.Now().Format(time.RFC3339) + `"}`),
		})
		select {
		case c.Send <- response:
		default:
		}

	case "heartbeat":
		// 客户端发起的心跳
		response, _ := json.Marshal(&WebSocketMessage{
			Type:      MessageTypeSystem,
			Timestamp: time.Now().Unix(),
			Data:      json.RawMessage(`{"heartbeat":"ack"}`),
		})
		select {
		case c.Send <- response:
		default:
		}
	}
}

// Close 关闭客户端
func (c *EnhancedClient) Close() {
	c.closeOnce.Do(func() {
		c.setState(StateClosing)
		c.cancel()

		c.mu.Lock()
		c.closed = true
		if c.Connection != nil {
			c.Connection.Close()
			c.Connection = nil
		}
		close(c.Send)
		c.mu.Unlock()

		c.wg.Wait()
		c.setState(StateDisconnected)
	})
}

// generateSecureID 生成安全的随机 ID
func generateSecureID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// EnhancedWebSocketHandler 增强版 WebSocket 处理器
type EnhancedWebSocketHandler struct {
	hub             *EnhancedWebSocketHub
	heartbeatConfig HeartbeatConfig
	reconnectConfig ReconnectConfig
}

// NewEnhancedWebSocketHandler 创建增强版处理器
func NewEnhancedWebSocketHandler(hub *EnhancedWebSocketHub) *EnhancedWebSocketHandler {
	return &EnhancedWebSocketHandler{
		hub:             hub,
		heartbeatConfig: DefaultHeartbeatConfig,
		reconnectConfig: DefaultReconnectConfig,
	}
}

// HandleWebSocket 处理 WebSocket 连接
func (h *EnhancedWebSocketHandler) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WebSocket] Upgrade error: %v", err)
		return
	}

	userID, _ := c.Get("userID")
	userIDStr, _ := userID.(string)

	client := NewEnhancedClient(conn, userIDStr, h.heartbeatConfig, h.reconnectConfig)
	h.hub.register <- client
	client.StartPumps(h.hub)
}

// GetStatus 获取状态
func (h *EnhancedWebSocketHandler) GetStatus(c *gin.Context) {
	stats := h.hub.GetAllClientStats()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"connectedClients": h.hub.GetClientCount(),
			"status":           "running",
			"clients":          stats,
		},
	})
}

// GetClientStatus 获取特定客户端状态
func (h *EnhancedWebSocketHandler) GetClientStatus(c *gin.Context) {
	clientID := c.Param("id")
	stats, err := h.hub.GetClientStats(clientID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// RegisterEnhancedWebSocketRoutes 注册增强版 WebSocket 路由
func RegisterEnhancedWebSocketRoutes(r *gin.RouterGroup, hub *EnhancedWebSocketHub) {
	handler := NewEnhancedWebSocketHandler(hub)

	r.GET("/ws", handler.HandleWebSocket)
	r.GET("/ws/status", handler.GetStatus)
	r.GET("/ws/client/:id", handler.GetClientStatus)
}