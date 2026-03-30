// Package team 实时通知与WebSocket功能
package team

import (
	"encoding/json"
	"sync"
	"time"
)

// Notifier 通知管理器
type Notifier struct {
	mu            sync.RWMutex
	notifications map[string][]*Notification               // userID -> notifications
	subscribers   map[string]map[string]chan *Notification // userID -> clientID -> channel
	resourceRooms map[string]map[string]bool               // resourceID -> clientID set
}

// NewNotifier 创建通知管理器
func NewNotifier() *Notifier {
	return &Notifier{
		notifications: make(map[string][]*Notification),
		subscribers:   make(map[string]map[string]chan *Notification),
		resourceRooms: make(map[string]map[string]bool),
	}
}

// Notify 发送通知
func (n *Notifier) Notify(notification *Notification) {
	if notification == nil || notification.UserID == "" {
		return
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	// 设置ID和时间
	if notification.ID == "" {
		notification.ID = generateID()
	}
	if notification.CreatedAt.IsZero() {
		notification.CreatedAt = time.Now()
	}

	// 存储通知
	n.notifications[notification.UserID] = append(n.notifications[notification.UserID], notification)

	// 发送给订阅者
	if subs, ok := n.subscribers[notification.UserID]; ok {
		for _, ch := range subs {
			select {
			case ch <- notification:
			default:
				// 通道满，跳过
			}
		}
	}
}

// Subscribe 订阅通知
func (n *Notifier) Subscribe(userID, clientID string) chan *Notification {
	n.mu.Lock()
	defer n.mu.Unlock()

	ch := make(chan *Notification, 100)

	if n.subscribers[userID] == nil {
		n.subscribers[userID] = make(map[string]chan *Notification)
	}
	n.subscribers[userID][clientID] = ch

	return ch
}

// Unsubscribe 取消订阅
func (n *Notifier) Unsubscribe(userID, clientID string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if subs, ok := n.subscribers[userID]; ok {
		if ch, ok := subs[clientID]; ok {
			close(ch)
			delete(subs, clientID)
		}
	}
}

// JoinResource 加入资源房间
func (n *Notifier) JoinResource(resourceID, clientID string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.resourceRooms[resourceID] == nil {
		n.resourceRooms[resourceID] = make(map[string]bool)
	}
	n.resourceRooms[resourceID][clientID] = true
}

// LeaveResource 离开资源房间
func (n *Notifier) LeaveResource(resourceID, clientID string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if room, ok := n.resourceRooms[resourceID]; ok {
		delete(room, clientID)
	}
}

// BroadcastToResource 广播消息到资源房间
func (n *Notifier) BroadcastToResource(resourceID string, message *WSMessage) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	data, err := json.Marshal(message)
	if err != nil {
		return
	}

	// 实际的WebSocket广播需要外部实现
	// 这里只记录需要广播的消息
	_ = data
}

// GetNotifications 获取用户通知
func (n *Notifier) GetNotifications(userID string, unreadOnly bool, limit int) []*Notification {
	n.mu.RLock()
	defer n.mu.RUnlock()

	notifications := n.notifications[userID]
	if notifications == nil {
		return []*Notification{}
	}

	result := make([]*Notification, 0)
	for i := len(notifications) - 1; i >= 0 && (limit == 0 || len(result) < limit); i-- {
		if !unreadOnly || !notifications[i].Read {
			result = append(result, notifications[i])
		}
	}

	return result
}

// MarkAsRead 标记通知为已读
func (n *Notifier) MarkAsRead(userID, notificationID string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	notifications := n.notifications[userID]
	if notifications == nil {
		return nil
	}

	for _, n := range notifications {
		if n.ID == notificationID {
			n.Read = true
			return nil
		}
	}

	return nil
}

// MarkAllAsRead 标记所有通知为已读
func (n *Notifier) MarkAllAsRead(userID string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	notifications := n.notifications[userID]
	for _, n := range notifications {
		n.Read = true
	}
}

// GetUnreadCount 获取未读通知数
func (n *Notifier) GetUnreadCount(userID string) int {
	n.mu.RLock()
	defer n.mu.RUnlock()

	count := 0
	notifications := n.notifications[userID]
	for _, n := range notifications {
		if !n.Read {
			count++
		}
	}

	return count
}

// ClearNotifications 清除用户通知
func (n *Notifier) ClearNotifications(userID string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	delete(n.notifications, userID)
}

// GetStats 获取通知统计
func (n *Notifier) GetStats() map[string]interface{} {
	n.mu.RLock()
	defer n.mu.RUnlock()

	totalNotifications := 0
	unreadCount := 0
	userCount := len(n.notifications)

	for _, notifications := range n.notifications {
		totalNotifications += len(notifications)
		for _, n := range notifications {
			if !n.Read {
				unreadCount++
			}
		}
	}

	return map[string]interface{}{
		"total_notifications": totalNotifications,
		"unread_count":        unreadCount,
		"user_count":          userCount,
		"active_rooms":        len(n.resourceRooms),
	}
}

// WebSocketHub WebSocket连接管理中心
type WebSocketHub struct {
	mu          sync.RWMutex
	connections map[string]*WSConnection   // clientID -> connection
	userClients map[string]map[string]bool // userID -> clientID set
	broadcast   chan *WSBroadcast
	register    chan *WSConnection
	unregister  chan *WSConnection
	stopChan    chan struct{}
	running     bool
}

// WSConnection WebSocket连接
type WSConnection struct {
	ClientID   string
	UserID     string
	Username   string
	TeamID     string
	ResourceID string
	send       chan []byte
}

// WSBroadcast 广播消息
type WSBroadcast struct {
	TeamID     string
	ResourceID string
	Message    []byte
	Exclude    []string
}

// NewWebSocketHub 创建WebSocket Hub
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		connections: make(map[string]*WSConnection),
		userClients: make(map[string]map[string]bool),
		broadcast:   make(chan *WSBroadcast, 1000),
		register:    make(chan *WSConnection),
		unregister:  make(chan *WSConnection),
		stopChan:    make(chan struct{}),
	}
}

// Run 运行Hub
func (h *WebSocketHub) Run() {
	h.mu.Lock()
	h.running = true
	h.mu.Unlock()

	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			h.connections[conn.ClientID] = conn
			if h.userClients[conn.UserID] == nil {
				h.userClients[conn.UserID] = make(map[string]bool)
			}
			h.userClients[conn.UserID][conn.ClientID] = true
			h.mu.Unlock()

		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.connections[conn.ClientID]; ok {
				delete(h.connections, conn.ClientID)
				if clients, ok := h.userClients[conn.UserID]; ok {
					delete(clients, conn.ClientID)
				}
				close(conn.send)
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			for _, conn := range h.connections {
				// 检查是否需要排除
				excluded := false
				for _, excludeID := range msg.Exclude {
					if conn.ClientID == excludeID {
						excluded = true
						break
					}
				}

				if excluded {
					continue
				}

				// 检查是否在目标房间
				if msg.TeamID != "" && conn.TeamID != msg.TeamID {
					continue
				}
				if msg.ResourceID != "" && conn.ResourceID != msg.ResourceID {
					continue
				}

				select {
				case conn.send <- msg.Message:
				default:
					// 发送失败，连接可能已断开
				}
			}
			h.mu.RUnlock()

		case <-h.stopChan:
			return
		}
	}
}

// Stop 停止Hub
func (h *WebSocketHub) Stop() {
	h.mu.Lock()
	h.running = false
	h.mu.Unlock()

	close(h.stopChan)
}

// Register 注册连接
func (h *WebSocketHub) Register(conn *WSConnection) {
	h.register <- conn
}

// Unregister 注销连接
func (h *WebSocketHub) Unregister(conn *WSConnection) {
	h.unregister <- conn
}

// Broadcast 广播消息
func (h *WebSocketHub) Broadcast(teamID, resourceID string, message []byte, exclude []string) {
	h.broadcast <- &WSBroadcast{
		TeamID:     teamID,
		ResourceID: resourceID,
		Message:    message,
		Exclude:    exclude,
	}
}

// SendToUser 发送消息给指定用户
func (h *WebSocketHub) SendToUser(userID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := h.userClients[userID]
	for clientID := range clients {
		if conn, ok := h.connections[clientID]; ok {
			select {
			case conn.send <- message:
			default:
			}
		}
	}
}

// GetOnlineUsers 获取在线用户
func (h *WebSocketHub) GetOnlineUsers(teamID string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	users := make(map[string]bool)
	for _, conn := range h.connections {
		if teamID == "" || conn.TeamID == teamID {
			users[conn.UserID] = true
		}
	}

	result := make([]string, 0, len(users))
	for userID := range users {
		result = append(result, userID)
	}
	return result
}

// GetConnectionCount 获取连接数
func (h *WebSocketHub) GetConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

// IsOnline 检查用户是否在线
func (h *WebSocketHub) IsOnline(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := h.userClients[userID]
	return len(clients) > 0
}

// NotifyTeamMembers 通知团队成员
func (n *Notifier) NotifyTeamMembers(teamID string, notification *Notification, excludeUserID string) {
	// 实际实现需要结合团队管理器获取成员列表
	// 这里只提供接口
}
