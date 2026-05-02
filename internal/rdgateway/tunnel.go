// Package rdgateway 提供 WebSocket 隧道功能
package rdgateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// WSTunnel WebSocket 隧道，用于浏览器到远程主机的数据传输.
type WSTunnel struct {
	mu       sync.RWMutex
	tunnels  map[string]*Tunnel // sessionID -> Tunnel
	upgrader WebSocketUpgrader
}

// WebSocketUpgrader 抽象的 WebSocket 升级接口.
type WebSocketUpgrader interface {
	Upgrade(w http.ResponseWriter, r *http.Request) (WebSocketConn, error)
}

// WebSocketConn 抽象的 WebSocket 连接.
type WebSocketConn interface {
	ReadMessage() (int, []byte, error)
	WriteMessage(int, []byte) error
	Close() error
}

// Tunnel 单个会话的隧道.
type Tunnel struct {
	SessionID string
	Conn      WebSocketConn
	CreatedAt time.Time
	mu        sync.Mutex
}

// TunnelMessage 隧道消息.
type TunnelMessage struct {
	Type    string          `json:"type"` // input, clipboard, file, resize, disconnect
	Payload json.RawMessage `json:"payload"`
}

// NewWSTunnel 创建 WebSocket 隧道.
func NewWSTunnel(upgrader WebSocketUpgrader) *WSTunnel {
	return &WSTunnel{
		tunnels:  make(map[string]*Tunnel),
		upgrader: upgrader,
	}
}

// HandleTunnel 处理 WebSocket 隧道连接.
func (t *WSTunnel) HandleTunnel(w http.ResponseWriter, r *http.Request, sessionID string) error {
	if t.upgrader == nil {
		return http.ErrNotSupported
	}

	conn, err := t.upgrader.Upgrade(w, r)
	if err != nil {
		return err
	}

	tunnel := &Tunnel{
		SessionID: sessionID,
		Conn:      conn,
		CreatedAt: time.Now(),
	}

	t.mu.Lock()
	t.tunnels[sessionID] = tunnel
	t.mu.Unlock()

	return nil
}

// SendToTunnel 向隧道发送消息.
func (t *WSTunnel) SendToTunnel(sessionID string, msg *TunnelMessage) error {
	t.mu.RLock()
	tunnel, ok := t.tunnels[sessionID]
	t.mu.RUnlock()

	if !ok {
		return ErrTunnelNotFound
	}

	tunnel.mu.Lock()
	defer tunnel.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return tunnel.Conn.WriteMessage(1, data) // 1 = TextMessage
}

// CloseTunnel 关闭隧道.
func (t *WSTunnel) CloseTunnel(sessionID string) error {
	t.mu.Lock()
	tunnel, ok := t.tunnels[sessionID]
	if ok {
		delete(t.tunnels, sessionID)
	}
	t.mu.Unlock()

	if !ok {
		return ErrTunnelNotFound
	}

	return tunnel.Conn.Close()
}

// TunnelCount 返回活跃隧道数.
func (t *WSTunnel) TunnelCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.tunnels)
}

// ErrTunnelNotFound 隧道不存在.
var ErrTunnelNotFound = fmt.Errorf("tunnel not found")
