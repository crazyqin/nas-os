// Package api provides enhanced WebSocket with heartbeat and reconnection support
// Version: 2.35.0 - WebSocket rooms, optimized broadcast, and message persistence
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
	// ErrConnectionClosed indicates the connection has been closed
	ErrConnectionClosed = errors.New("connection closed")
	// ErrReconnectFailed indicates reconnection has failed
	ErrReconnectFailed    = errors.New("reconnection failed")
	ErrMaxReconnect       = errors.New("max reconnection attempts reached")
	ErrHeartbeatTimeout   = errors.New("heartbeat timeout")
	ErrConnectionNotFound = errors.New("connection not found")
	ErrRoomNotFound       = errors.New("room not found")
	ErrNotInRoom          = errors.New("client not in room")
	ErrQueueFull          = errors.New("message queue full")
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
	PingInterval:          30 * time.Second,
	PongTimeout:           10 * time.Second,
	MaxMissedPongs:        3,
	EnableServerHeartbeat: true,
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
	// StateDisconnected represents disconnected state
	StateDisconnected ConnectionState = iota
	// StateConnecting represents connecting state
	StateConnecting
	// StateConnected represents connected state
	StateConnected
	// StateReconnecting represents reconnecting state
	StateReconnecting
	// StateClosing represents closing state
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
	ID               string    `json:"id"`
	UserID           string    `json:"userId"`
	State            string    `json:"state"`
	ConnectedAt      time.Time `json:"connectedAt"`
	LastActivity     time.Time `json:"lastActivity"`
	ReconnectCount   int       `json:"reconnectCount"`
	MessagesSent     int64     `json:"messagesSent"`
	MessagesReceived int64     `json:"messagesReceived"`
	BytesSent        int64     `json:"bytesSent"`
	BytesReceived    int64     `json:"bytesReceived"`
	MissedHeartbeats int       `json:"missedHeartbeats"`
}

// Room represents a chat room/channel
type Room struct {
	ID          string
	Name        string
	Description string
	Clients     map[string]*EnhancedClient
	CreatedAt   time.Time
	MaxClients  int
	mu          sync.RWMutex
}

// NewRoom creates a new room
func NewRoom(id, name, description string, maxClients int) *Room {
	if maxClients <= 0 {
		maxClients = 100 // default max clients
	}
	return &Room{
		ID:          id,
		Name:        name,
		Description: description,
		Clients:     make(map[string]*EnhancedClient),
		CreatedAt:   time.Now(),
		MaxClients:  maxClients,
	}
}

// AddClient adds a client to the room
func (r *Room) AddClient(client *EnhancedClient) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.Clients) >= r.MaxClients {
		return errors.New("room is full")
	}

	r.Clients[client.ID] = client
	return nil
}

// RemoveClient removes a client from the room
func (r *Room) RemoveClient(clientID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.Clients, clientID)
}

// GetClientCount returns the number of clients in the room
func (r *Room) GetClientCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.Clients)
}

// RoomStats represents room statistics
type RoomStats struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ClientCount int       `json:"clientCount"`
	MaxClients  int       `json:"maxClients"`
	CreatedAt   time.Time `json:"createdAt"`
}

// PersistedMessage represents a message stored in the queue
type PersistedMessage struct {
	ID        string          `json:"id"`
	Type      MessageType     `json:"type"`
	RoomID    string          `json:"roomId,omitempty"`
	UserID    string          `json:"userId,omitempty"`
	Timestamp int64           `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
	Delivered bool            `json:"delivered"`
}

// MessageQueueConfig configures the message queue
type MessageQueueConfig struct {
	// MaxSize maximum number of messages to persist
	MaxSize int
	// PersistToDisk whether to persist messages to disk
	PersistToDisk bool
	// StoragePath path for disk persistence
	StoragePath string
	// TTL message time-to-live in seconds
	TTL time.Duration
}

// DefaultMessageQueueConfig default queue configuration
var DefaultMessageQueueConfig = MessageQueueConfig{
	MaxSize:       1000,
	PersistToDisk: false,
	TTL:           24 * time.Hour,
}

// MessageQueue manages message persistence
type MessageQueue struct {
	messages []*PersistedMessage
	config   MessageQueueConfig
	mu       sync.RWMutex
}

// NewMessageQueue creates a new message queue
func NewMessageQueue(config MessageQueueConfig) *MessageQueue {
	if config.MaxSize <= 0 {
		config.MaxSize = DefaultMessageQueueConfig.MaxSize
	}
	if config.TTL <= 0 {
		config.TTL = DefaultMessageQueueConfig.TTL
	}
	return &MessageQueue{
		messages: make([]*PersistedMessage, 0),
		config:   config,
	}
}

// Enqueue adds a message to the queue
func (q *MessageQueue) Enqueue(msg *PersistedMessage) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Remove oldest if at capacity
	if len(q.messages) >= q.config.MaxSize {
		q.messages = q.messages[1:]
	}

	q.messages = append(q.messages, msg)
	return nil
}

// Dequeue removes and returns the oldest message
func (q *MessageQueue) Dequeue() (*PersistedMessage, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.messages) == 0 {
		return nil, errors.New("queue is empty")
	}

	msg := q.messages[0]
	q.messages = q.messages[1:]
	return msg, nil
}

// Peek returns the oldest message without removing it
func (q *MessageQueue) Peek() (*PersistedMessage, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if len(q.messages) == 0 {
		return nil, errors.New("queue is empty")
	}

	return q.messages[0], nil
}

// GetMessages returns all messages for a room
func (q *MessageQueue) GetMessages(roomID string) []*PersistedMessage {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]*PersistedMessage, 0)
	for _, msg := range q.messages {
		if msg.RoomID == roomID || roomID == "" {
			result = append(result, msg)
		}
	}
	return result
}

// GetMessagesForUser returns all undelivered messages for a user
func (q *MessageQueue) GetMessagesForUser(userID string) []*PersistedMessage {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]*PersistedMessage, 0)
	for _, msg := range q.messages {
		if msg.UserID == userID && !msg.Delivered {
			result = append(result, msg)
		}
	}
	return result
}

// MarkDelivered marks a message as delivered
func (q *MessageQueue) MarkDelivered(messageID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, msg := range q.messages {
		if msg.ID == messageID {
			msg.Delivered = true
			break
		}
	}
}

// Cleanup removes expired messages
func (q *MessageQueue) Cleanup() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now().Unix()
	active := make([]*PersistedMessage, 0)
	removed := 0

	for _, msg := range q.messages {
		if now-msg.Timestamp < int64(q.config.TTL.Seconds()) {
			active = append(active, msg)
		} else {
			removed++
		}
	}

	q.messages = active
	return removed
}

// Size returns the current queue size
func (q *MessageQueue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.messages)
}

// BroadcastConfig configures broadcast optimization
type BroadcastConfig struct {
	// BatchSize number of messages to batch
	BatchSize int
	// BatchTimeout max time to wait for batch
	BatchTimeout time.Duration
	// WorkerCount number of broadcast workers
	WorkerCount int
}

// DefaultBroadcastConfig default broadcast configuration
var DefaultBroadcastConfig = BroadcastConfig{
	BatchSize:    100,
	BatchTimeout: 10 * time.Millisecond,
	WorkerCount:  4,
}

// EnhancedClient 增强版 WebSocket 客户端
type EnhancedClient struct {
	ID            string
	UserID        string
	Connection    *websocket.Conn
	Send          chan []byte
	Subscriptions map[MessageType]bool
	ConnectedAt   time.Time
	LastActivity  time.Time

	// 房间支持
	Rooms map[string]bool

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

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
	wg     sync.WaitGroup
}

// EnhancedWebSocketHub 增强版 WebSocket Hub
type EnhancedWebSocketHub struct {
	clients      map[string]*EnhancedClient
	broadcast    chan *WebSocketMessage
	register     chan *EnhancedClient
	unregister   chan *EnhancedClient
	statusChange chan *ClientStatusChange
	mu           sync.RWMutex

	// 配置
	heartbeatConfig HeartbeatConfig
	reconnectConfig ReconnectConfig
	broadcastConfig BroadcastConfig

	// 房间管理
	rooms  map[string]*Room
	roomMu sync.RWMutex

	// 消息队列
	messageQueue *MessageQueue

	// 统计
	totalConnections int64
	totalMessages    int64
	totalBroadcasts  int64

	// 广播优化
	broadcastWorkers sync.WaitGroup
	broadcastQueue   chan *broadcastTask

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
}

// broadcastTask represents a broadcast task
type broadcastTask struct {
	message *WebSocketMessage
	targets []*EnhancedClient
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
	hub := &EnhancedWebSocketHub{
		clients:         make(map[string]*EnhancedClient),
		broadcast:       make(chan *WebSocketMessage, 512),
		register:        make(chan *EnhancedClient, 64),
		unregister:      make(chan *EnhancedClient, 64),
		statusChange:    make(chan *ClientStatusChange, 128),
		heartbeatConfig: heartbeat,
		reconnectConfig: reconnect,
		broadcastConfig: DefaultBroadcastConfig,
		rooms:           make(map[string]*Room),
		messageQueue:    NewMessageQueue(DefaultMessageQueueConfig),
		broadcastQueue:  make(chan *broadcastTask, 256),
		ctx:             ctx,
		cancel:          cancel,
	}
	return hub
}

// NewEnhancedWebSocketHubWithConfig creates hub with full configuration
func NewEnhancedWebSocketHubWithConfig(heartbeat HeartbeatConfig, reconnect ReconnectConfig, broadcast BroadcastConfig, queueConfig MessageQueueConfig) *EnhancedWebSocketHub {
	ctx, cancel := context.WithCancel(context.Background())
	return &EnhancedWebSocketHub{
		clients:         make(map[string]*EnhancedClient),
		broadcast:       make(chan *WebSocketMessage, 512),
		register:        make(chan *EnhancedClient, 64),
		unregister:      make(chan *EnhancedClient, 64),
		statusChange:    make(chan *ClientStatusChange, 128),
		heartbeatConfig: heartbeat,
		reconnectConfig: reconnect,
		broadcastConfig: broadcast,
		rooms:           make(map[string]*Room),
		messageQueue:    NewMessageQueue(queueConfig),
		broadcastQueue:  make(chan *broadcastTask, 256),
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Run 启动 Hub
func (h *EnhancedWebSocketHub) Run() {
	// 启动广播工作器
	for i := 0; i < h.broadcastConfig.WorkerCount; i++ {
		h.broadcastWorkers.Add(1)
		go h.broadcastWorker(i)
	}

	// 启动消息队列清理
	go h.messageQueueCleaner()

	// 启动状态监控
	go h.monitorStatus()

	for {
		select {
		case <-h.ctx.Done():
			h.closeAllClients()
			h.broadcastWorkers.Wait()
			return

		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case status := <-h.statusChange:
			h.handleStatusChange(status)

		case message := <-h.broadcast:
			h.broadcastMessage(message)

		case task := <-h.broadcastQueue:
			h.processBroadcastTask(task)
		}
	}
}

// broadcastWorker processes broadcast tasks
func (h *EnhancedWebSocketHub) broadcastWorker(id int) {
	defer h.broadcastWorkers.Done()

	for {
		select {
		case <-h.ctx.Done():
			return
		case task := <-h.broadcastQueue:
			h.processBroadcastTask(task)
		}
	}
}

// processBroadcastTask processes a single broadcast task
func (h *EnhancedWebSocketHub) processBroadcastTask(task *broadcastTask) {
	data, err := json.Marshal(task.message)
	if err != nil {
		return
	}

	for _, client := range task.targets {
		select {
		case client.Send <- data:
			atomic.AddInt64(&client.messagesSent, 1)
			atomic.AddInt64(&client.bytesSent, int64(len(data)))
		default:
			// Buffer full, skip
		}
	}
}

// messageQueueCleaner periodically cleans up expired messages
func (h *EnhancedWebSocketHub) messageQueueCleaner() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			removed := h.messageQueue.Cleanup()
			if removed > 0 {
				log.Printf("[WebSocket] Cleaned up %d expired messages from queue", removed)
			}
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
	delete(h.clients, client.ID)
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
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

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

// === Room Management ===

// CreateRoom creates a new room
func (h *EnhancedWebSocketHub) CreateRoom(id, name, description string, maxClients int) *Room {
	h.roomMu.Lock()
	defer h.roomMu.Unlock()

	room := NewRoom(id, name, description, maxClients)
	h.rooms[id] = room
	log.Printf("[WebSocket] Room created: %s (%s)", id, name)
	return room
}

// GetRoom gets a room by ID
func (h *EnhancedWebSocketHub) GetRoom(roomID string) (*Room, error) {
	h.roomMu.RLock()
	defer h.roomMu.RUnlock()

	room, exists := h.rooms[roomID]
	if !exists {
		return nil, ErrRoomNotFound
	}
	return room, nil
}

// DeleteRoom deletes a room
func (h *EnhancedWebSocketHub) DeleteRoom(roomID string) error {
	h.roomMu.Lock()
	defer h.roomMu.Unlock()

	room, exists := h.rooms[roomID]
	if !exists {
		return ErrRoomNotFound
	}

	// Remove all clients from the room
	room.mu.RLock()
	for _, client := range room.Clients {
		client.mu.Lock()
		delete(client.Rooms, roomID)
		client.mu.Unlock()
	}
	room.mu.RUnlock()

	delete(h.rooms, roomID)
	log.Printf("[WebSocket] Room deleted: %s", roomID)
	return nil
}

// JoinRoom adds a client to a room
func (h *EnhancedWebSocketHub) JoinRoom(roomID, clientID string) error {
	h.roomMu.RLock()
	room, exists := h.rooms[roomID]
	h.roomMu.RUnlock()

	if !exists {
		return ErrRoomNotFound
	}

	h.mu.RLock()
	client, exists := h.clients[clientID]
	h.mu.RUnlock()

	if !exists {
		return ErrConnectionNotFound
	}

	if err := room.AddClient(client); err != nil {
		return err
	}

	client.mu.Lock()
	if client.Rooms == nil {
		client.Rooms = make(map[string]bool)
	}
	client.Rooms[roomID] = true
	client.mu.Unlock()

	log.Printf("[WebSocket] Client %s joined room %s", clientID, roomID)
	return nil
}

// LeaveRoom removes a client from a room
func (h *EnhancedWebSocketHub) LeaveRoom(roomID, clientID string) error {
	h.roomMu.RLock()
	room, exists := h.rooms[roomID]
	h.roomMu.RUnlock()

	if !exists {
		return ErrRoomNotFound
	}

	h.mu.RLock()
	client, exists := h.clients[clientID]
	h.mu.RUnlock()

	if !exists {
		return ErrConnectionNotFound
	}

	room.RemoveClient(clientID)

	client.mu.Lock()
	delete(client.Rooms, roomID)
	client.mu.Unlock()

	log.Printf("[WebSocket] Client %s left room %s", clientID, roomID)
	return nil
}

// BroadcastToRoom broadcasts a message to all clients in a room
func (h *EnhancedWebSocketHub) BroadcastToRoom(roomID string, msgType MessageType, data interface{}, excludeClientID string) error {
	h.roomMu.RLock()
	room, exists := h.rooms[roomID]
	h.roomMu.RUnlock()

	if !exists {
		return ErrRoomNotFound
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	msg := &WebSocketMessage{
		Type:      msgType,
		Timestamp: time.Now().Unix(),
		Data:      dataBytes,
	}

	// Persist message
	persistedMsg := &PersistedMessage{
		ID:        generateSecureID(),
		Type:      msgType,
		RoomID:    roomID,
		Timestamp: msg.Timestamp,
		Data:      dataBytes,
	}
	if err := h.messageQueue.Enqueue(persistedMsg); err != nil {
		log.Printf("[WebSocket] Failed to enqueue message: %v", err)
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	room.mu.RLock()
	for _, client := range room.Clients {
		if client.ID == excludeClientID {
			continue
		}
		select {
		case client.Send <- msgBytes:
			atomic.AddInt64(&client.messagesSent, 1)
		default:
		}
	}
	room.mu.RUnlock()

	return nil
}

// GetRoomStats returns statistics for all rooms
func (h *EnhancedWebSocketHub) GetRoomStats() []RoomStats {
	h.roomMu.RLock()
	defer h.roomMu.RUnlock()

	stats := make([]RoomStats, 0, len(h.rooms))
	for _, room := range h.rooms {
		stats = append(stats, RoomStats{
			ID:          room.ID,
			Name:        room.Name,
			Description: room.Description,
			ClientCount: room.GetClientCount(),
			MaxClients:  room.MaxClients,
			CreatedAt:   room.CreatedAt,
		})
	}
	return stats
}

// GetQueueStats returns message queue statistics
func (h *EnhancedWebSocketHub) GetQueueStats() map[string]interface{} {
	return map[string]interface{}{
		"size":    h.messageQueue.Size(),
		"maxSize": h.messageQueue.config.MaxSize,
		"ttl":     h.messageQueue.config.TTL.String(),
	}
}

// GetPendingMessages returns pending messages for a user
func (h *EnhancedWebSocketHub) GetPendingMessages(userID string) []*PersistedMessage {
	return h.messageQueue.GetMessagesForUser(userID)
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
		Rooms:           make(map[string]bool),
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
	if err := c.Connection.SetReadDeadline(time.Now().Add(c.heartbeatConfig.PongTimeout + c.heartbeatConfig.PingInterval)); err != nil {
		log.Printf("[WebSocket] Failed to set read deadline: %v", err)
	}

	// Pong 处理器
	c.Connection.SetPongHandler(func(string) error {
		if err := c.Connection.SetReadDeadline(time.Now().Add(c.heartbeatConfig.PongTimeout + c.heartbeatConfig.PingInterval)); err != nil {
			log.Printf("[WebSocket] Failed to reset read deadline in pong handler: %v", err)
		}
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

			if err := c.Connection.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				log.Printf("[WebSocket] Failed to set write deadline: %v", err)
				return
			}
			if err := c.Connection.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

			// 批量发送
			for i := 0; i < len(c.Send); i++ {
				msg := <-c.Send
				if err := c.Connection.WriteMessage(websocket.TextMessage, msg); err != nil {
					log.Printf("[WebSocket] Failed to write batch message: %v", err)
				}
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

	if err := c.Connection.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		log.Printf("[WebSocket] Failed to set write deadline for ping: %v", err)
	}
	return c.Connection.WriteMessage(websocket.PingMessage, nil)
}

// sendCloseMessage 发送关闭消息
func (c *EnhancedClient) sendCloseMessage() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Connection != nil {
		if err := c.Connection.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
			log.Printf("[WebSocket] Failed to set write deadline for close message: %v", err)
		}
		if err := c.Connection.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
			log.Printf("[WebSocket] Failed to send close message: %v", err)
		}
	}
}

// handleMessage 处理消息
func (c *EnhancedClient) handleMessage(messageType int, message []byte, hub *EnhancedWebSocketHub) {
	var msg struct {
		Action        string          `json:"action"`
		Subscriptions []MessageType   `json:"subscriptions,omitempty"`
		RoomID        string          `json:"roomId,omitempty"`
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

	case "joinRoom":
		if msg.RoomID != "" {
			if err := hub.JoinRoom(msg.RoomID, c.ID); err != nil {
				// Send error response
				errResp, mErr := json.Marshal(&WebSocketMessage{
					Type:      MessageTypeSystem,
					Timestamp: time.Now().Unix(),
					Data:      json.RawMessage(`{"error":"` + err.Error() + `"}`),
				})
				if mErr == nil {
					select {
					case c.Send <- errResp:
					default:
					}
				}
			} else {
				// Send success response
				resp, mErr := json.Marshal(&WebSocketMessage{
					Type:      MessageTypeSystem,
					Timestamp: time.Now().Unix(),
					Data:      json.RawMessage(`{"joinedRoom":"` + msg.RoomID + `"}`),
				})
				if mErr == nil {
					select {
					case c.Send <- resp:
					default:
					}
				}
			}
		}

	case "leaveRoom":
		if msg.RoomID != "" {
			if err := hub.LeaveRoom(msg.RoomID, c.ID); err != nil {
				log.Printf("[WebSocket] Failed to leave room %s: %v", msg.RoomID, err)
			}
			resp, mErr := json.Marshal(&WebSocketMessage{
				Type:      MessageTypeSystem,
				Timestamp: time.Now().Unix(),
				Data:      json.RawMessage(`{"leftRoom":"` + msg.RoomID + `"}`),
			})
			if mErr == nil {
				select {
				case c.Send <- resp:
				default:
				}
			}
		}

	case "roomMessage":
		if msg.RoomID != "" && len(msg.Data) > 0 {
			if err := hub.BroadcastToRoom(msg.RoomID, MessageTypeEvent, msg.Data, c.ID); err != nil {
				log.Printf("[WebSocket] Failed to broadcast to room %s: %v", msg.RoomID, err)
			}
		}

	case "ping":
		// 响应 pong
		response, mErr := json.Marshal(&WebSocketMessage{
			Type:      MessageTypeSystem,
			Timestamp: time.Now().Unix(),
			Data:      json.RawMessage(`{"pong":true,"time":"` + time.Now().Format(time.RFC3339) + `"}`),
		})
		if mErr == nil {
			select {
			case c.Send <- response:
			default:
			}
		}

	case "heartbeat":
		// 客户端发起的心跳
		response, mErr := json.Marshal(&WebSocketMessage{
			Type:      MessageTypeSystem,
			Timestamp: time.Now().Unix(),
			Data:      json.RawMessage(`{"heartbeat":"ack"}`),
		})
		if mErr == nil {
			select {
			case c.Send <- response:
			default:
			}
		}

	case "getPendingMessages":
		// 获取待发送消息
		pending := hub.GetPendingMessages(c.UserID)
		data, dErr := json.Marshal(map[string]interface{}{
			"pendingMessages": pending,
		})
		if dErr == nil {
			response, mErr := json.Marshal(&WebSocketMessage{
				Type:      MessageTypeSystem,
				Timestamp: time.Now().Unix(),
				Data:      data,
			})
			if mErr == nil {
				select {
				case c.Send <- response:
				default:
				}
			}
		}
	}
}

// Close 关闭客户端
func (c *EnhancedClient) Close() {
	c.setState(StateClosing)
	c.cancel()

	c.mu.Lock()
	if c.Connection != nil {
		if err := c.Connection.Close(); err != nil {
			log.Printf("[WebSocket] Error closing connection: %v", err)
		}
		c.Connection = nil
	}
	// Only close channel once
	select {
	case <-c.Send:
		// Already closed
	default:
		close(c.Send)
	}
	c.mu.Unlock()

	c.wg.Wait()
	c.setState(StateDisconnected)
}

// generateSecureID 生成安全的随机 ID
func generateSecureID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand 失败是致命错误
	}
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

// === Room API Handlers ===

// CreateRoom creates a new room
func (h *EnhancedWebSocketHandler) CreateRoom(c *gin.Context) {
	var req struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		MaxClients  int    `json:"maxClients"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request body",
		})
		return
	}

	if req.ID == "" {
		req.ID = generateSecureID()
	}
	if req.Name == "" {
		req.Name = "Room " + req.ID[:8]
	}

	room := h.hub.CreateRoom(req.ID, req.Name, req.Description, req.MaxClients)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "room created",
		"data": RoomStats{
			ID:          room.ID,
			Name:        room.Name,
			Description: room.Description,
			ClientCount: 0,
			MaxClients:  room.MaxClients,
			CreatedAt:   room.CreatedAt,
		},
	})
}

// GetRooms returns all rooms
func (h *EnhancedWebSocketHandler) GetRooms(c *gin.Context) {
	stats := h.hub.GetRoomStats()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// GetRoom returns a specific room
func (h *EnhancedWebSocketHandler) GetRoom(c *gin.Context) {
	roomID := c.Param("roomId")
	room, err := h.hub.GetRoom(roomID)
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
		"data": RoomStats{
			ID:          room.ID,
			Name:        room.Name,
			Description: room.Description,
			ClientCount: room.GetClientCount(),
			MaxClients:  room.MaxClients,
			CreatedAt:   room.CreatedAt,
		},
	})
}

// DeleteRoom deletes a room
func (h *EnhancedWebSocketHandler) DeleteRoom(c *gin.Context) {
	roomID := c.Param("roomId")
	if err := h.hub.DeleteRoom(roomID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "room deleted",
	})
}

// GetQueueStats returns message queue statistics
func (h *EnhancedWebSocketHandler) GetQueueStats(c *gin.Context) {
	stats := h.hub.GetQueueStats()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// RegisterEnhancedWebSocketRoutes 注册增强版 WebSocket 路由
func RegisterEnhancedWebSocketRoutes(r *gin.RouterGroup, hub *EnhancedWebSocketHub) {
	handler := NewEnhancedWebSocketHandler(hub)

	// WebSocket connection
	r.GET("/ws", handler.HandleWebSocket)

	// WebSocket status
	r.GET("/ws/status", handler.GetStatus)
	r.GET("/ws/client/:id", handler.GetClientStatus)
	r.GET("/ws/queue", handler.GetQueueStats)

	// Room management
	r.POST("/ws/rooms", handler.CreateRoom)
	r.GET("/ws/rooms", handler.GetRooms)
	r.GET("/ws/rooms/:roomId", handler.GetRoom)
	r.DELETE("/ws/rooms/:roomId", handler.DeleteRoom)
}
