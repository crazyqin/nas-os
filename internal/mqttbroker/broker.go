// Package mqttbroker 提供轻量级 MQTT 消息代理
// 用于 NAS 智能家居/IoT 设备集成
// 支持 MQTT 3.1.1 / QoS 0-2 / 持久会话 / 遗嘱消息 / 访问控制
package mqttbroker

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// QoS 服务质量等级.
type QoS byte

const (
	QoS0 QoS = 0 // 最多一次
	QoS1 QoS = 1 // 至少一次
	QoS2 QoS = 2 // 恰好一次
)

// MessageType 消息类型.
type MessageType byte

const (
	MsgConnect     MessageType = 1
	MsgConnAck     MessageType = 2
	MsgPublish     MessageType = 3
	MsgPubAck      MessageType = 4
	MsgSubscribe   MessageType = 8
	MsgSubAck      MessageType = 9
	MsgUnsubscribe MessageType = 10
	MsgUnsubAck    MessageType = 11
	MsgPingReq     MessageType = 12
	MsgPingResp    MessageType = 13
	MsgDisconnect  MessageType = 14
)

// ClientState 客户端状态.
type ClientState string

const (
	ClientConnected    ClientState = "connected"
	ClientDisconnected ClientState = "disconnected"
)

// Client MQTT 客户端.
type Client struct {
	mu          sync.RWMutex
	ClientID    string      `json:"clientId"`
	Username    string      `json:"username"`
	State       ClientState `json:"state"`
	CleanSession bool       `json:"cleanSession"`
	KeepAlive   int         `json:"keepAlive"` // 秒
	WillTopic   string      `json:"willTopic"`
	WillMessage []byte      `json:"willMessage"`
	WillQoS     QoS         `json:"willQoS"`
	WillRetain  bool        `json:"willRetain"`
	Subscriptions []string  `json:"subscriptions"`
	ConnectedAt time.Time   `json:"connectedAt"`
	LastSeen    time.Time   `json:"lastSeen"`
	RemoteAddr  string      `json:"remoteAddr"`
}

// Message MQTT 消息.
type Message struct {
	Topic     string    `json:"topic"`
	Payload   []byte    `json:"payload"`
	QoS       QoS       `json:"qoS"`
	Retain    bool      `json:"retain"`
	MessageID uint16    `json:"messageId"`
	Timestamp time.Time `json:"timestamp"`
}

// Broker MQTT 代理.
type Broker struct {
	mu          sync.RWMutex
	config      *BrokerConfig
	clients     map[string]*Client
	subscriptions map[string][]*Client // topic -> clients
	retained    map[string]*Message   // topic -> message
	messages    []*Message
	stats       *BrokerStats
}

// BrokerConfig 代理配置.
type BrokerConfig struct {
	ListenAddr     string        `json:"listenAddr"`     // 监听地址
	Port           int           `json:"port"`           // 监听端口
	MaxClients     int           `json:"maxClients"`     // 最大客户端数
	MaxMessageSize int           `json:"maxMessageSize"` // 最大消息大小
	AllowAnonymous bool          `json:"allowAnonymous"` // 允许匿名连接
	RetainEnabled  bool          `json:"retainEnabled"`  // 启用保留消息
	SessionExpiry  time.Duration `json:"sessionExpiry"`  // 会话过期时间
}

// BrokerStats 代理统计.
type BrokerStats struct {
	mu              sync.RWMutex
	TotalClients    int       `json:"totalClients"`
	ConnectedClients int      `json:"connectedClients"`
	TotalMessages   int64     `json:"totalMessages"`
	TotalSubscriptions int    `json:"totalSubscriptions"`
	TotalRetained   int       `json:"totalRetained"`
	StartedAt       time.Time `json:"startedAt"`
	LastMessageAt   time.Time `json:"lastMessageAt"`
}

// NewBroker 创建 MQTT 代理.
func NewBroker(config *BrokerConfig) *Broker {
	if config == nil {
		config = &BrokerConfig{
			ListenAddr:     "0.0.0.0",
			Port:           1883,
			MaxClients:     1000,
			MaxMessageSize: 1 << 20, // 1MB
			AllowAnonymous: true,
			RetainEnabled:  true,
			SessionExpiry:  2 * time.Hour,
		}
	}
	return &Broker{
		config:        config,
		clients:       make(map[string]*Client),
		subscriptions: make(map[string][]*Client),
		retained:      make(map[string]*Message),
		stats:         &BrokerStats{StartedAt: time.Now()},
	}
}

// ConnectClient 客户端连接.
func (b *Broker) ConnectClient(clientID, username string, cleanSession bool, keepAlive int) (*Client, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.clients) >= b.config.MaxClients {
		return nil, fmt.Errorf("max clients reached: %d", b.config.MaxClients)
	}

	// 检查重复连接
	if existing, ok := b.clients[clientID]; ok {
		if existing.State == ClientConnected {
			// 断开旧连接
			existing.State = ClientDisconnected
		}
	}

	client := &Client{
		ClientID:     clientID,
		Username:     username,
		State:        ClientConnected,
		CleanSession: cleanSession,
		KeepAlive:    keepAlive,
		ConnectedAt:  time.Now(),
		LastSeen:     time.Now(),
	}

	b.clients[clientID] = client

	b.stats.mu.Lock()
	b.stats.TotalClients = len(b.clients)
	b.stats.ConnectedClients++
	b.stats.mu.Unlock()

	return client, nil
}

// DisconnectClient 客户端断开.
func (b *Broker) DisconnectClient(clientID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	client, ok := b.clients[clientID]
	if !ok {
		return fmt.Errorf("client %s not found", clientID)
	}

	// 发送遗嘱消息
	if client.WillTopic != "" {
		willMsg := &Message{
			Topic:     client.WillTopic,
			Payload:   client.WillMessage,
			QoS:       client.WillQoS,
			Retain:    client.WillRetain,
			Timestamp: time.Now(),
		}
		b.publishToSubscribers(willMsg)
	}

	client.State = ClientDisconnected

	// 清理会话
	if client.CleanSession {
		b.removeClientSubscriptions(clientID)
		delete(b.clients, clientID)
	}

	b.stats.mu.Lock()
	b.stats.ConnectedClients--
	if client.CleanSession {
		b.stats.TotalClients = len(b.clients)
	}
	b.stats.mu.Unlock()

	return nil
}

// Subscribe 订阅主题.
func (b *Broker) Subscribe(clientID, topic string, qos QoS) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	client, ok := b.clients[clientID]
	if !ok {
		return fmt.Errorf("client %s not found", clientID)
	}

	// 添加订阅
	b.subscriptions[topic] = append(b.subscriptions[topic], client)
	client.Subscriptions = append(client.Subscriptions, topic)

	b.stats.mu.Lock()
	b.stats.TotalSubscriptions++
	b.stats.mu.Unlock()

	// 发送保留消息
	if b.config.RetainEnabled {
		if msg, ok := b.retained[topic]; ok {
			b.deliverToClient(client, msg)
		}
	}

	return nil
}

// Unsubscribe 取消订阅.
func (b *Broker) Unsubscribe(clientID, topic string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	client, ok := b.clients[clientID]
	if !ok {
		return fmt.Errorf("client %s not found", clientID)
	}

	// 从订阅列表移除
	subs := b.subscriptions[topic]
	for i, c := range subs {
		if c.ClientID == clientID {
			b.subscriptions[topic] = append(subs[:i], subs[i+1:]...)
			break
		}
	}

	// 从客户端订阅列表移除
	for i, t := range client.Subscriptions {
		if t == topic {
			client.Subscriptions = append(client.Subscriptions[:i], client.Subscriptions[i+1:]...)
			break
		}
	}

	return nil
}

// Publish 发布消息.
func (b *Broker) Publish(topic string, payload []byte, qos QoS, retain bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	msg := &Message{
		Topic:     topic,
		Payload:   payload,
		QoS:       qos,
		Retain:    retain,
		Timestamp: time.Now(),
	}

	// 保留消息
	if retain && b.config.RetainEnabled {
		b.retained[topic] = msg
	}

	// 发送给订阅者
	b.publishToSubscribers(msg)

	b.stats.mu.Lock()
	b.stats.TotalMessages++
	b.stats.LastMessageAt = time.Now()
	b.stats.TotalRetained = len(b.retained)
	b.stats.mu.Unlock()
}

// publishToSubscribers 发送给匹配的订阅者.
func (b *Broker) publishToSubscribers(msg *Message) {
	for _, client := range b.subscriptions[msg.Topic] {
		if client.State == ClientConnected {
			b.deliverToClient(client, msg)
		}
	}

	// 通配符匹配
	for pattern, clients := range b.subscriptions {
		if matchTopic(pattern, msg.Topic) {
			for _, client := range clients {
				if client.State == ClientConnected {
					b.deliverToClient(client, msg)
				}
			}
		}
	}
}

// deliverToClient 投递消息给客户端.
func (b *Broker) deliverToClient(client *Client, msg *Message) {
	client.mu.Lock()
	client.LastSeen = time.Now()
	client.mu.Unlock()
	// 实际投递需要网络层实现
}

// matchTopic 主题匹配（支持 + 和 # 通配符）.
func matchTopic(pattern, topic string) bool {
	if pattern == topic {
		return true
	}

	patternParts := strings.Split(pattern, "/")
	topicParts := strings.Split(topic, "/")

	for i, part := range patternParts {
		if part == "#" {
			return true
		}
		if i >= len(topicParts) {
			return false
		}
		if part != "+" && part != topicParts[i] {
			return false
		}
	}

	return len(patternParts) == len(topicParts)
}

// GetStats 获取代理统计.
func (b *Broker) GetStats() *BrokerStats {
	b.stats.mu.RLock()
	defer b.stats.mu.RUnlock()
	return &BrokerStats{
		TotalClients:       b.stats.TotalClients,
		ConnectedClients:   b.stats.ConnectedClients,
		TotalMessages:      b.stats.TotalMessages,
		TotalSubscriptions: b.stats.TotalSubscriptions,
		TotalRetained:      b.stats.TotalRetained,
		StartedAt:          b.stats.StartedAt,
		LastMessageAt:      b.stats.LastMessageAt,
	}
}

// ListClients 列出所有客户端.
func (b *Broker) ListClients() []*Client {
	b.mu.RLock()
	defer b.mu.RUnlock()

	clients := make([]*Client, 0, len(b.clients))
	for _, c := range b.clients {
		clients = append(clients, c)
	}
	return clients
}

// GetRetainedMessages 获取所有保留消息.
func (b *Broker) GetRetainedMessages() []*Message {
	b.mu.RLock()
	defer b.mu.RUnlock()

	msgs := make([]*Message, 0, len(b.retained))
	for _, m := range b.retained {
		msgs = append(msgs, m)
	}
	return msgs
}

// removeClientSubscriptions 移除客户端所有订阅.
func (b *Broker) removeClientSubscriptions(clientID string) {
	for topic, clients := range b.subscriptions {
		for i, c := range clients {
			if c.ClientID == clientID {
				b.subscriptions[topic] = append(clients[:i], clients[i+1:]...)
				break
			}
		}
	}
}
