// Package api provides WebSocket real-time communication for NAS-OS
// Version: 2.0 - WebSocket support for real-time notifications
package api

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WebSocket upgrader configuration
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// In production, implement proper origin checking
		return true
	},
}

// MessageType defines WebSocket message types
type MessageType string

const (
	// MessageTypeSystem represents a system message type
	MessageTypeSystem       MessageType = "system"
	// MessageTypeNotification represents a notification message type
	MessageTypeNotification MessageType = "notification"
	MessageTypeMetric       MessageType = "metric"
	MessageTypeAlert        MessageType = "alert"
	MessageTypeEvent        MessageType = "event"
	MessageTypeContainer    MessageType = "container"
	MessageTypeStorage      MessageType = "storage"
	MessageTypeBackup       MessageType = "backup"
	MessageTypeSync         MessageType = "sync"
)

// WebSocketMessage represents a WebSocket message
type WebSocketMessage struct {
	Type      MessageType     `json:"type"`
	Timestamp int64           `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// Client represents a connected WebSocket client
type Client struct {
	ID            string
	Connection    *websocket.Conn
	Send          chan []byte
	Subscriptions map[MessageType]bool
	UserID        string
	ConnectedAt   time.Time
}

// WebSocketHub manages WebSocket connections and broadcasts
type WebSocketHub struct {
	clients    map[*Client]bool
	broadcast  chan *WebSocketMessage
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// NewWebSocketHub creates a new WebSocket hub
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan *WebSocketMessage, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the WebSocket hub
func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("WebSocket client connected: %s (total: %d)", client.ID, len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
			h.mu.Unlock()
			log.Printf("WebSocket client disconnected: %s (total: %d)", client.ID, len(h.clients))

		case message := <-h.broadcast:
			h.mu.RLock()
			data, err := json.Marshal(message)
			if err != nil {
				h.mu.RUnlock()
				continue
			}

			for client := range h.clients {
				// Check if client is subscribed to this message type
				if len(client.Subscriptions) == 0 || client.Subscriptions[message.Type] {
					select {
					case client.Send <- data:
					default:
						// Client buffer full, skip
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a message to all connected clients
func (h *WebSocketHub) Broadcast(msgType MessageType, data interface{}) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	msg := &WebSocketMessage{
		Type:      msgType,
		Timestamp: time.Now().Unix(),
		Data:      dataBytes,
	}

	h.broadcast <- msg
	return nil
}

// BroadcastToUser sends a message to a specific user's clients
func (h *WebSocketHub) BroadcastToUser(userID string, msgType MessageType, data interface{}) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	msg := &WebSocketMessage{
		Type:      msgType,
		Timestamp: time.Now().Unix(),
		Data:      dataBytes,
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	for client := range h.clients {
		if client.UserID == userID {
			select {
			case client.Send <- msgBytes:
			default:
				// Buffer full
			}
		}
	}
	return nil
}

// GetClientCount returns the number of connected clients
func (h *WebSocketHub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ReadPump handles incoming messages from a client
func (c *Client) ReadPump(h *WebSocketHub) {
	defer func() {
		h.unregister <- c
		_ = c.Connection.Close()
	}()

	c.Connection.SetReadLimit(512)
	_ = c.Connection.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Connection.SetPongHandler(func(string) error {
		_ = c.Connection.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Connection.ReadMessage()
		if err != nil {
			break
		}

		// Handle client messages (subscriptions, etc.)
		var msg struct {
			Action        string        `json:"action"`
			Subscriptions []MessageType `json:"subscriptions,omitempty"`
		}
		if err := json.Unmarshal(message, &msg); err == nil {
			switch msg.Action {
			case "subscribe":
				for _, t := range msg.Subscriptions {
					c.Subscriptions[t] = true
				}
			case "unsubscribe":
				for _, t := range msg.Subscriptions {
					delete(c.Subscriptions, t)
				}
			case "ping":
				// Respond with pong
				_ = c.Connection.WriteJSON(&WebSocketMessage{
					Type:      "system",
					Timestamp: time.Now().Unix(),
					Data:      json.RawMessage(`{"pong":true}`),
				})
			}
		}
	}
}

// WritePump handles outgoing messages to a client
func (c *Client) WritePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		_ = c.Connection.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			_ = c.Connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.Connection.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Connection.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := w.Write(message); err != nil {
				return
			}

			// Batch messages
			n := len(c.Send)
			for i := 0; i < n; i++ {
				if _, err := w.Write([]byte{'\n'}); err != nil {
					return
				}
				if _, err := w.Write(<-c.Send); err != nil {
					return
				}
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.Connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Connection.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	hub *WebSocketHub
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(hub *WebSocketHub) *WebSocketHandler {
	return &WebSocketHandler{hub: hub}
}

// HandleWebSocket handles WebSocket upgrade and connection
func (h *WebSocketHandler) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		ID:            generateClientID(),
		Connection:    conn,
		Send:          make(chan []byte, 256),
		Subscriptions: make(map[MessageType]bool),
		UserID:        c.GetString("userID"),
		ConnectedAt:   time.Now(),
	}

	h.hub.register <- client

	// Start read/write pumps
	go client.WritePump()
	go client.ReadPump(h.hub)
}

// GetStatus returns WebSocket server status
func (h *WebSocketHandler) GetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"connectedClients": h.hub.GetClientCount(),
			"status":           "running",
		},
	})
}

// generateClientID generates a unique client ID
func generateClientID() string {
	return fmt.Sprintf("ws-%d-%s", time.Now().UnixNano(), randomString(6))
}

// randomString generates a random string using crypto/rand
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	// 使用 crypto/rand 生成安全的随机数
	_, err := rand.Read(b)
	if err != nil {
		// 回退到时间戳（仅在极端情况下）
		for i := range b {
			b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		}
		return string(b)
	}
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}

// Notification types for WebSocket

// SystemNotification represents a system notification
type SystemNotification struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	Level   string `json:"level"` // info, warning, error, success
}

// MetricUpdate represents a metric update
type MetricUpdate struct {
	CPU     float64 `json:"cpu"`
	Memory  float64 `json:"memory"`
	Disk    float64 `json:"disk"`
	Network struct {
		Rx int64 `json:"rx"`
		Tx int64 `json:"tx"`
	} `json:"network"`
}

// AlertNotification represents an alert
type AlertNotification struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Severity     string `json:"severity"`
	Title        string `json:"title"`
	Message      string `json:"message"`
	Source       string `json:"source"`
	Timestamp    int64  `json:"timestamp"`
	Acknowledged bool   `json:"acknowledged"`
}

// ContainerEvent represents a container event
type ContainerEvent struct {
	ContainerID string `json:"containerId"`
	Name        string `json:"name"`
	Action      string `json:"action"` // start, stop, restart, create, destroy
	Status      string `json:"status"`
	Timestamp   int64  `json:"timestamp"`
}

// StorageEvent represents a storage event
type StorageEvent struct {
	VolumeName string `json:"volumeName"`
	EventType  string `json:"eventType"` // mount, unmount, error, warning
	Message    string `json:"message"`
	Timestamp  int64  `json:"timestamp"`
}

// BackupEvent represents a backup event
type BackupEvent struct {
	JobID     string `json:"jobId"`
	Status    string `json:"status"` // started, completed, failed
	Progress  int    `json:"progress"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// RegisterWebSocketRoutes registers WebSocket routes
func RegisterWebSocketRoutes(r *gin.RouterGroup, hub *WebSocketHub) {
	handler := NewWebSocketHandler(hub)

	// WebSocket endpoint
	r.GET("/ws", handler.HandleWebSocket)

	// WebSocket status endpoint
	r.GET("/ws/status", handler.GetStatus)
}
