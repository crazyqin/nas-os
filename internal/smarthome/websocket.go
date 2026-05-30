// Package smarthome WebSocket 实时推送支持
package smarthome

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// WebSocketMessage WebSocket 消息
type WebSocketMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
	Time    time.Time   `json:"time"`
}

// WebSocketHub WebSocket 中心
type WebSocketHub struct {
	clients    map[*WebSocketClient]bool
	broadcast  chan []byte
	register   chan *WebSocketClient
	unregister chan *WebSocketClient
	mu         sync.RWMutex
}

// WebSocketClient WebSocket 客户端
type WebSocketClient struct {
	hub  *WebSocketHub
	send chan []byte
	id   string
}

// NewWebSocketHub 创建 WebSocket 中心
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*WebSocketClient]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *WebSocketClient),
		unregister: make(chan *WebSocketClient),
	}
}

// Run 运行 WebSocket 中心
func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			
		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastDeviceState 推送设备状态
func (h *WebSocketHub) BroadcastDeviceState(device *Device) {
	msg := WebSocketMessage{
		Type:    "device_state",
		Payload: device,
		Time:    time.Now(),
	}
	
	data, _ := json.Marshal(msg)
	h.broadcast <- data
}

// BroadcastAutomationEvent 推送自动化事件
func (h *WebSocketHub) BroadcastAutomationEvent(event interface{}) {
	msg := WebSocketMessage{
		Type:    "automation_event",
		Payload: event,
		Time:    time.Now(),
	}
	
	data, _ := json.Marshal(msg)
	h.broadcast <- data
}

// BroadcastEnergyUpdate 推送能耗更新
func (h *WebSocketHub) BroadcastEnergyUpdate(reading EnergyReading) {
	msg := WebSocketMessage{
		Type:    "energy_update",
		Payload: reading,
		Time:    time.Now(),
	}
	
	data, _ := json.Marshal(msg)
	h.broadcast <- data
}

// ClientCount 返回客户端数量
func (h *WebSocketHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// HandleWebSocket 处理 WebSocket 连接
func (h *WebSocketHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 简化的 WebSocket 处理（实际实现需要使用 gorilla/websocket 等库）
	// 这里仅作为接口定义
}

// MQTTBrokerConfig MQTT Broker 配置
type MQTTBrokerConfig struct {
	Type     string `json:"type"`     // mosquitto, emqx
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	ClientID string `json:"client_id"`
}

// ZigbeeConfig Zigbee 配置
type ZigbeeConfig struct {
	Enabled  bool   `json:"enabled"`
	Adapter  string `json:"adapter"`  // zstack, deconz
	Port     string `json:"port"`     // /dev/ttyUSB0
	BaudRate int    `json:"baud_rate"`
}

// ZWaveConfig Z-Wave 配置
type ZWaveConfig struct {
	Enabled  bool   `json:"enabled"`
	Port     string `json:"port"`
	NetworkKey string `json:"network_key"`
}

// DiscoveryConfig 设备发现配置
type DiscoveryConfig struct {
	Enabled         bool     `json:"enabled"`
	Interval        int      `json:"interval"` // seconds
	Protocols       []string `json:"protocols"`
	AutoRegister    bool     `json:"auto_register"`
}

// AutomationRuleConfig 自动化规则配置
type AutomationRuleConfig struct {
	MaxRules        int  `json:"max_rules"`
	EvaluateInterval int `json:"evaluate_interval"` // seconds
	LogExecutions   bool `json:"log_executions"`
}
