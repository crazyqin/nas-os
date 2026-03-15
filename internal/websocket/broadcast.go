// Package websocket 提供消息广播功能
// Version: v2.56.0 - 多房间广播、主题订阅
package websocket

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Broadcaster 消息广播器
type Broadcaster struct {
	mu sync.RWMutex

	// 房间管理
	rooms      map[string]*Room        // roomID -> Room
	clientRoom map[string]map[string]bool // clientID -> roomID set

	// 主题订阅
	topics       map[string]*Topic        // topic -> Topic
	clientTopics map[string]map[string]bool // clientID -> topic set

	// 房间到主题的映射
	roomTopics map[string]map[string]bool // roomID -> topic set

	// 消息队列
	messageQueue chan *BroadcastMessage

	// 配置
	config BroadcasterConfig

	// 统计
	sentCount     int64
	droppedCount  int64
	roomCount     int32
	topicCount    int32
	clientCount   int32

	// 控制
	stopChan chan struct{}
	running  bool
}

// BroadcasterConfig 广播器配置
type BroadcasterConfig struct {
	QueueSize        int           `json:"queueSize"`        // 消息队列大小
	MaxRooms        int           `json:"maxRooms"`         // 最大房间数
	MaxTopics       int           `json:"maxTopics"`        // 最大主题数
	MaxClientsPerRoom int         `json:"maxClientsPerRoom"` // 每个房间最大客户端数
	BroadcastTimeout time.Duration `json:"broadcastTimeout"` // 广播超时
	EnableHistory    bool         `json:"enableHistory"`    // 启用历史记录
	HistorySize      int          `json:"historySize"`      // 历史记录大小
}

// DefaultBroadcasterConfig 默认广播器配置
var DefaultBroadcasterConfig = BroadcasterConfig{
	QueueSize:         10000,
	MaxRooms:          1000,
	MaxTopics:         1000,
	MaxClientsPerRoom: 1000,
	BroadcastTimeout:  5 * time.Second,
	EnableHistory:     true,
	HistorySize:       100,
}

// Room 房间
type Room struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Clients     map[string]*Client `json:"-"` // clientID -> Client
	CreatedAt   time.Time         `json:"createdAt"`
	LastActive  time.Time         `json:"lastActive"`
	MessageCount int64            `json:"messageCount"`
	history     []*BroadcastMessage `json:"-"`
}

// Topic 主题
type Topic struct {
	Name        string    `json:"name"`
	Subscribers map[string]bool `json:"-"` // clientID -> bool
	CreatedAt   time.Time `json:"createdAt"`
	LastActive  time.Time `json:"lastActive"`
	MessageCount int64    `json:"messageCount"`
}

// Client 客户端
type Client struct {
	ID         string    `json:"id"`
	UserID     string    `json:"userId,omitempty"`
	Name       string    `json:"name,omitempty"`
	Connected  bool      `json:"connected"`
	JoinedAt   time.Time `json:"joinedAt"`
	LastActive time.Time `json:"lastActive"`
	sendChan   chan *BroadcastMessage
}

// BroadcastMessage 广播消息
type BroadcastMessage struct {
	ID        string          `json:"id"`
	RoomID    string          `json:"roomId,omitempty"`
	Topic     string          `json:"topic,omitempty"`
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data"`
	From      string          `json:"from,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	Priority  MessagePriority `json:"priority"`
	Exclude   []string        `json:"exclude,omitempty"` // 排除的客户端
}

// NewBroadcaster 创建广播器
func NewBroadcaster(config BroadcasterConfig) *Broadcaster {
	if config.QueueSize <= 0 {
		config.QueueSize = DefaultBroadcasterConfig.QueueSize
	}
	if config.MaxRooms <= 0 {
		config.MaxRooms = DefaultBroadcasterConfig.MaxRooms
	}
	if config.MaxTopics <= 0 {
		config.MaxTopics = DefaultBroadcasterConfig.MaxTopics
	}

	return &Broadcaster{
		rooms:        make(map[string]*Room),
		clientRoom:   make(map[string]map[string]bool),
		topics:       make(map[string]*Topic),
		clientTopics: make(map[string]map[string]bool),
		roomTopics:   make(map[string]map[string]bool),
		messageQueue: make(chan *BroadcastMessage, config.QueueSize),
		config:       config,
		stopChan:     make(chan struct{}),
	}
}

// Start 启动广播器
func (b *Broadcaster) Start() {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return
	}
	b.running = true
	b.mu.Unlock()

	go b.processMessages()
}

// Stop 停止广播器
func (b *Broadcaster) Stop() {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return
	}
	b.running = false
	b.mu.Unlock()

	close(b.stopChan)
}

// ========== 房间管理 ==========

// CreateRoom 创建房间
func (b *Broadcaster) CreateRoom(roomID, name string) (*Room, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.rooms) >= b.config.MaxRooms {
		return nil, fmt.Errorf("达到最大房间数限制")
	}

	if _, exists := b.rooms[roomID]; exists {
		return nil, fmt.Errorf("房间已存在: %s", roomID)
	}

	room := &Room{
		ID:         roomID,
		Name:       name,
		Clients:    make(map[string]*Client),
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
	}

	if b.config.EnableHistory {
		room.history = make([]*BroadcastMessage, 0, b.config.HistorySize)
	}

	b.rooms[roomID] = room
	b.roomTopics[roomID] = make(map[string]bool)
	atomic.AddInt32(&b.roomCount, 1)

	return room, nil
}

// GetRoom 获取房间
func (b *Broadcaster) GetRoom(roomID string) (*Room, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	room, exists := b.rooms[roomID]
	if !exists {
		return nil, fmt.Errorf("房间不存在: %s", roomID)
	}
	return room, nil
}

// DeleteRoom 删除房间
func (b *Broadcaster) DeleteRoom(roomID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	room, exists := b.rooms[roomID]
	if !exists {
		return fmt.Errorf("房间不存在: %s", roomID)
	}

	// 移除所有客户端
	for clientID := range room.Clients {
		if rooms, ok := b.clientRoom[clientID]; ok {
			delete(rooms, roomID)
		}
	}

	// 清理主题映射
	delete(b.roomTopics, roomID)

	// 删除房间
	delete(b.rooms, roomID)
	atomic.AddInt32(&b.roomCount, -1)

	return nil
}

// ListRooms 列出所有房间
func (b *Broadcaster) ListRooms() []*Room {
	b.mu.RLock()
	defer b.mu.RUnlock()

	rooms := make([]*Room, 0, len(b.rooms))
	for _, room := range b.rooms {
		rooms = append(rooms, room)
	}
	return rooms
}

// ========== 客户端管理 ==========

// JoinRoom 加入房间
func (b *Broadcaster) JoinRoom(roomID, clientID string, client *Client) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	room, exists := b.rooms[roomID]
	if !exists {
		return fmt.Errorf("房间不存在: %s", roomID)
	}

	if len(room.Clients) >= b.config.MaxClientsPerRoom {
		return fmt.Errorf("房间已满")
	}

	// 添加客户端到房间
	room.Clients[clientID] = client
	room.LastActive = time.Now()

	// 更新客户端-房间映射
	if b.clientRoom[clientID] == nil {
		b.clientRoom[clientID] = make(map[string]bool)
	}
	b.clientRoom[clientID][roomID] = true

	atomic.AddInt32(&b.clientCount, 1)

	return nil
}

// LeaveRoom 离开房间
func (b *Broadcaster) LeaveRoom(roomID, clientID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	room, exists := b.rooms[roomID]
	if !exists {
		return fmt.Errorf("房间不存在: %s", roomID)
	}

	delete(room.Clients, clientID)

	// 更新客户端-房间映射
	if rooms, ok := b.clientRoom[clientID]; ok {
		delete(rooms, roomID)
		if len(rooms) == 0 {
			delete(b.clientRoom, clientID)
		}
	}

	atomic.AddInt32(&b.clientCount, -1)

	return nil
}

// LeaveAllRooms 离开所有房间
func (b *Broadcaster) LeaveAllRooms(clientID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	rooms, exists := b.clientRoom[clientID]
	if !exists {
		return
	}

	for roomID := range rooms {
		if room, ok := b.rooms[roomID]; ok {
			delete(room.Clients, clientID)
		}
	}

	delete(b.clientRoom, clientID)

	// 清理主题订阅
	delete(b.clientTopics, clientID)
}

// GetRoomClients 获取房间客户端
func (b *Broadcaster) GetRoomClients(roomID string) ([]*Client, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	room, exists := b.rooms[roomID]
	if !exists {
		return nil, fmt.Errorf("房间不存在: %s", roomID)
	}

	clients := make([]*Client, 0, len(room.Clients))
	for _, client := range room.Clients {
		clients = append(clients, client)
	}
	return clients, nil
}

// ========== 主题订阅 ==========

// SubscribeTopic 订阅主题
func (b *Broadcaster) SubscribeTopic(clientID, topicName string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// 创建主题（如果不存在）
	if len(b.topics) >= b.config.MaxTopics {
		// 检查是否已存在
		if _, exists := b.topics[topicName]; !exists {
			return fmt.Errorf("达到最大主题数限制")
		}
	}

	topic, exists := b.topics[topicName]
	if !exists {
		topic = &Topic{
			Name:        topicName,
			Subscribers: make(map[string]bool),
			CreatedAt:   time.Now(),
			LastActive:  time.Now(),
		}
		b.topics[topicName] = topic
		atomic.AddInt32(&b.topicCount, 1)
	}

	topic.Subscribers[clientID] = true
	topic.LastActive = time.Now()

	// 更新客户端-主题映射
	if b.clientTopics[clientID] == nil {
		b.clientTopics[clientID] = make(map[string]bool)
	}
	b.clientTopics[clientID][topicName] = true

	return nil
}

// UnsubscribeTopic 取消订阅主题
func (b *Broadcaster) UnsubscribeTopic(clientID, topicName string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	topic, exists := b.topics[topicName]
	if !exists {
		return fmt.Errorf("主题不存在: %s", topicName)
	}

	delete(topic.Subscribers, clientID)

	// 更新客户端-主题映射
	if topics, ok := b.clientTopics[clientID]; ok {
		delete(topics, topicName)
		if len(topics) == 0 {
			delete(b.clientTopics, clientID)
		}
	}

	return nil
}

// UnsubscribeAll 取消所有订阅
func (b *Broadcaster) UnsubscribeAll(clientID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	topics, exists := b.clientTopics[clientID]
	if !exists {
		return
	}

	for topicName := range topics {
		if topic, ok := b.topics[topicName]; ok {
			delete(topic.Subscribers, clientID)
		}
	}

	delete(b.clientTopics, clientID)
}

// GetTopicSubscribers 获取主题订阅者
func (b *Broadcaster) GetTopicSubscribers(topicName string) ([]string, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	topic, exists := b.topics[topicName]
	if !exists {
		return nil, fmt.Errorf("主题不存在: %s", topicName)
	}

	subscribers := make([]string, 0, len(topic.Subscribers))
	for clientID := range topic.Subscribers {
		subscribers = append(subscribers, clientID)
	}
	return subscribers, nil
}

// ListTopics 列出所有主题
func (b *Broadcaster) ListTopics() []*Topic {
	b.mu.RLock()
	defer b.mu.RUnlock()

	topics := make([]*Topic, 0, len(b.topics))
	for _, topic := range b.topics {
		topics = append(topics, topic)
	}
	return topics
}

// GetClientTopics 获取客户端订阅的主题
func (b *Broadcaster) GetClientTopics(clientID string) []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	topics, exists := b.clientTopics[clientID]
	if !exists {
		return nil
	}

	result := make([]string, 0, len(topics))
	for topic := range topics {
		result = append(result, topic)
	}
	return result
}

// ========== 房间-主题映射 ==========

// BindRoomTopic 绑定房间到主题
func (b *Broadcaster) BindRoomTopic(roomID, topicName string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.rooms[roomID]; !exists {
		return fmt.Errorf("房间不存在: %s", roomID)
	}

	if b.roomTopics[roomID] == nil {
		b.roomTopics[roomID] = make(map[string]bool)
	}
	b.roomTopics[roomID][topicName] = true

	return nil
}

// UnbindRoomTopic 解绑房间与主题
func (b *Broadcaster) UnbindRoomTopic(roomID, topicName string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if topics, ok := b.roomTopics[roomID]; ok {
		delete(topics, topicName)
	}

	return nil
}

// GetRoomTopics 获取房间绑定的主题
func (b *Broadcaster) GetRoomTopics(roomID string) []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	topics, exists := b.roomTopics[roomID]
	if !exists {
		return nil
	}

	result := make([]string, 0, len(topics))
	for topic := range topics {
		result = append(result, topic)
	}
	return result
}

// ========== 消息广播 ==========

// BroadcastToRoom 广播到房间
func (b *Broadcaster) BroadcastToRoom(roomID string, msg *BroadcastMessage) error {
	if msg.ID == "" {
		msg.ID = generateMessageID()
	}
	msg.RoomID = roomID
	msg.Timestamp = time.Now()

	select {
	case b.messageQueue <- msg:
		return nil
	default:
		atomic.AddInt64(&b.droppedCount, 1)
		return fmt.Errorf("消息队列已满")
	}
}

// BroadcastToTopic 广播到主题
func (b *Broadcaster) BroadcastToTopic(topicName string, msg *BroadcastMessage) error {
	if msg.ID == "" {
		msg.ID = generateMessageID()
	}
	msg.Topic = topicName
	msg.Timestamp = time.Now()

	select {
	case b.messageQueue <- msg:
		return nil
	default:
		atomic.AddInt64(&b.droppedCount, 1)
		return fmt.Errorf("消息队列已满")
	}
}

// BroadcastToAll 广播到所有客户端
func (b *Broadcaster) BroadcastToAll(msg *BroadcastMessage) error {
	if msg.ID == "" {
		msg.ID = generateMessageID()
	}
	msg.Timestamp = time.Now()

	select {
	case b.messageQueue <- msg:
		return nil
	default:
		atomic.AddInt64(&b.droppedCount, 1)
		return fmt.Errorf("消息队列已满")
	}
}

// BroadcastToClient 发送消息到指定客户端
func (b *Broadcaster) BroadcastToClient(clientID string, msg *BroadcastMessage) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// 查找客户端所在的所有房间
	rooms, exists := b.clientRoom[clientID]
	if !exists {
		return fmt.Errorf("客户端未加入任何房间")
	}

	msg.ID = generateMessageID()
	msg.Timestamp = time.Now()
	msg.Exclude = nil

	// 发送到客户端所在的所有房间
	for roomID := range rooms {
		if room, ok := b.rooms[roomID]; ok {
			if client, ok := room.Clients[clientID]; ok {
				select {
				case client.sendChan <- msg:
					atomic.AddInt64(&b.sentCount, 1)
				default:
					atomic.AddInt64(&b.droppedCount, 1)
				}
			}
		}
	}

	return nil
}

// processMessages 处理消息队列
func (b *Broadcaster) processMessages() {
	for {
		select {
		case <-b.stopChan:
			return
		case msg := <-b.messageQueue:
			b.deliverMessage(msg)
		}
	}
}

// deliverMessage 投递消息
func (b *Broadcaster) deliverMessage(msg *BroadcastMessage) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	excludeSet := make(map[string]bool)
	for _, clientID := range msg.Exclude {
		excludeSet[clientID] = true
	}

	// 广播到房间
	if msg.RoomID != "" {
		room, exists := b.rooms[msg.RoomID]
		if !exists {
			return
		}

		room.MessageCount++
		room.LastActive = time.Now()

		// 保存历史
		if b.config.EnableHistory && room.history != nil {
			room.history = append(room.history, msg)
			if len(room.history) > b.config.HistorySize {
				room.history = room.history[1:]
			}
		}

		// 发送到房间内的所有客户端
		for clientID, client := range room.Clients {
			if excludeSet[clientID] {
				continue
			}

			select {
			case client.sendChan <- msg:
				atomic.AddInt64(&b.sentCount, 1)
			default:
				atomic.AddInt64(&b.droppedCount, 1)
			}
		}
		return
	}

	// 广播到主题
	if msg.Topic != "" {
		topic, exists := b.topics[msg.Topic]
		if !exists {
			return
		}

		topic.MessageCount++
		topic.LastActive = time.Now()

		// 发送给订阅该主题的所有客户端
		for clientID := range topic.Subscribers {
			if excludeSet[clientID] {
				continue
			}

			// 找到客户端并发送
			if rooms, ok := b.clientRoom[clientID]; ok {
				for roomID := range rooms {
					if room, ok := b.rooms[roomID]; ok {
						if client, ok := room.Clients[clientID]; ok {
							select {
							case client.sendChan <- msg:
								atomic.AddInt64(&b.sentCount, 1)
							default:
								atomic.AddInt64(&b.droppedCount, 1)
							}
							break // 只发送一次
						}
					}
				}
			}
		}
		return
	}

	// 广播到所有客户端
	for _, room := range b.rooms {
		for clientID, client := range room.Clients {
			if excludeSet[clientID] {
				continue
			}

			select {
			case client.sendChan <- msg:
				atomic.AddInt64(&b.sentCount, 1)
			default:
				atomic.AddInt64(&b.droppedCount, 1)
			}
		}
	}
}

// GetRoomHistory 获取房间历史消息
func (b *Broadcaster) GetRoomHistory(roomID string, limit int) ([]*BroadcastMessage, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	room, exists := b.rooms[roomID]
	if !exists {
		return nil, fmt.Errorf("房间不存在: %s", roomID)
	}

	if !b.config.EnableHistory || room.history == nil {
		return nil, nil
	}

	if limit <= 0 || limit > len(room.history) {
		limit = len(room.history)
	}

	// 返回最近的消息
	start := len(room.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*BroadcastMessage, limit)
	copy(result, room.history[start:])

	return result, nil
}

// ========== 统计信息 ==========

// BroadcasterStats 广播器统计
type BroadcasterStats struct {
	RoomCount     int32 `json:"roomCount"`
	TopicCount    int32 `json:"topicCount"`
	ClientCount   int32 `json:"clientCount"`
	SentCount     int64 `json:"sentCount"`
	DroppedCount  int64 `json:"droppedCount"`
	QueueSize     int   `json:"queueSize"`
	QueueUsed     int   `json:"queueUsed"`
}

// Stats 获取统计信息
func (b *Broadcaster) Stats() BroadcasterStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return BroadcasterStats{
		RoomCount:    atomic.LoadInt32(&b.roomCount),
		TopicCount:   atomic.LoadInt32(&b.topicCount),
		ClientCount:  atomic.LoadInt32(&b.clientCount),
		SentCount:    atomic.LoadInt64(&b.sentCount),
		DroppedCount: atomic.LoadInt64(&b.droppedCount),
		QueueSize:    cap(b.messageQueue),
		QueueUsed:    len(b.messageQueue),
	}
}

// RoomStats 房间统计
type RoomStats struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	ClientCount  int       `json:"clientCount"`
	MessageCount int64     `json:"messageCount"`
	CreatedAt    time.Time `json:"createdAt"`
	LastActive   time.Time `json:"lastActive"`
}

// GetRoomStats 获取房间统计
func (b *Broadcaster) GetRoomStats(roomID string) (*RoomStats, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	room, exists := b.rooms[roomID]
	if !exists {
		return nil, fmt.Errorf("房间不存在: %s", roomID)
	}

	return &RoomStats{
		ID:           room.ID,
		Name:         room.Name,
		ClientCount:  len(room.Clients),
		MessageCount: room.MessageCount,
		CreatedAt:    room.CreatedAt,
		LastActive:   room.LastActive,
	}, nil
}

// TopicStats 主题统计
type TopicStats struct {
	Name         string    `json:"name"`
	SubscriberCount int    `json:"subscriberCount"`
	MessageCount int64     `json:"messageCount"`
	CreatedAt    time.Time `json:"createdAt"`
	LastActive   time.Time `json:"lastActive"`
}

// GetTopicStats 获取主题统计
func (b *Broadcaster) GetTopicStats(topicName string) (*TopicStats, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	topic, exists := b.topics[topicName]
	if !exists {
		return nil, fmt.Errorf("主题不存在: %s", topicName)
	}

	return &TopicStats{
		Name:            topic.Name,
		SubscriberCount: len(topic.Subscribers),
		MessageCount:    topic.MessageCount,
		CreatedAt:       topic.CreatedAt,
		LastActive:      topic.LastActive,
	}, nil
}