// Package api provides tests for enhanced WebSocket functionality
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestNewEnhancedWebSocketHub tests hub creation
func TestNewEnhancedWebSocketHub(t *testing.T) {
	heartbeat := DefaultHeartbeatConfig
	reconnect := DefaultReconnectConfig

	hub := NewEnhancedWebSocketHub(heartbeat, reconnect)
	if hub == nil {
		t.Fatal("Expected non-nil hub")
	}

	if hub.GetClientCount() != 0 {
		t.Errorf("Expected 0 clients, got %d", hub.GetClientCount())
	}

	// Test defaults
	if heartbeat.PingInterval != 30*time.Second {
		t.Errorf("Expected PingInterval 30s, got %v", heartbeat.PingInterval)
	}

	if reconnect.MaxAttempts != 5 {
		t.Errorf("Expected MaxAttempts 5, got %d", reconnect.MaxAttempts)
	}
}

// TestEnhancedWebSocketHubBroadcast tests message broadcasting
func TestEnhancedWebSocketHubBroadcast(t *testing.T) {
	hub := NewEnhancedWebSocketHub(DefaultHeartbeatConfig, DefaultReconnectConfig)
	go hub.Run()
	defer hub.Stop()

	// Give hub time to start
	time.Sleep(10 * time.Millisecond)

	// Test broadcast with no clients
	err := hub.Broadcast(MessageTypeSystem, map[string]string{"test": "value"})
	if err != nil {
		t.Errorf("Expected no error for broadcast with no clients, got %v", err)
	}
}

// TestEnhancedWebSocketHubBroadcastToUser tests user-specific broadcasting
func TestEnhancedWebSocketHubBroadcastToUser(t *testing.T) {
	hub := NewEnhancedWebSocketHub(DefaultHeartbeatConfig, DefaultReconnectConfig)
	go hub.Run()
	defer hub.Stop()

	time.Sleep(10 * time.Millisecond)

	err := hub.BroadcastToUser("test-user", MessageTypeSystem, map[string]string{"test": "value"})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

// TestEnhancedClientState tests client state management
func TestEnhancedClientState(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Keep connection open briefly
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	// Connect as client
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := NewEnhancedClient(conn, "test-user", DefaultHeartbeatConfig, DefaultReconnectConfig)

	// Test initial state
	if client.GetState() != StateConnecting {
		t.Errorf("Expected Connecting state, got %v", client.GetState())
	}

	// Test state change
	client.setState(StateConnected)
	if client.GetState() != StateConnected {
		t.Errorf("Expected Connected state, got %v", client.GetState())
	}

	// Test stats
	stats := client.GetStats()
	if stats.ID == "" {
		t.Error("Expected non-empty client ID")
	}
	if stats.UserID != "test-user" {
		t.Errorf("Expected UserID test-user, got %s", stats.UserID)
	}
}

// TestConnectionStateString tests state string conversion
func TestConnectionStateString(t *testing.T) {
	tests := []struct {
		state    ConnectionState
		expected string
	}{
		{StateDisconnected, "disconnected"},
		{StateConnecting, "connecting"},
		{StateConnected, "connected"},
		{StateReconnecting, "reconnecting"},
		{StateClosing, "closing"},
		{ConnectionState(99), "unknown"},
	}

	for _, tt := range tests {
		result := tt.state.String()
		if result != tt.expected {
			t.Errorf("State %d: expected %s, got %s", tt.state, tt.expected, result)
		}
	}
}

// TestHeartbeatConfig tests heartbeat configuration
func TestHeartbeatConfig(t *testing.T) {
	config := HeartbeatConfig{
		PingInterval:          15 * time.Second,
		PongTimeout:           5 * time.Second,
		MaxMissedPongs:        5,
		EnableServerHeartbeat: true,
	}

	if config.PingInterval != 15*time.Second {
		t.Errorf("Expected PingInterval 15s, got %v", config.PingInterval)
	}

	if config.MaxMissedPongs != 5 {
		t.Errorf("Expected MaxMissedPongs 5, got %d", config.MaxMissedPongs)
	}

	// Test defaults
	defaultConfig := DefaultHeartbeatConfig
	if defaultConfig.PingInterval != 30*time.Second {
		t.Errorf("Expected default PingInterval 30s, got %v", defaultConfig.PingInterval)
	}
}

// TestReconnectConfig tests reconnect configuration
func TestReconnectConfig(t *testing.T) {
	config := ReconnectConfig{
		Enable:        true,
		MaxAttempts:   10,
		InitialDelay:  2 * time.Second,
		MaxDelay:      60 * time.Second,
		BackoffFactor: 1.5,
		ResetAfter:    120 * time.Second,
	}

	if config.MaxAttempts != 10 {
		t.Errorf("Expected MaxAttempts 10, got %d", config.MaxAttempts)
	}

	if config.BackoffFactor != 1.5 {
		t.Errorf("Expected BackoffFactor 1.5, got %f", config.BackoffFactor)
	}
}

// TestWebSocketMessage tests message types
func TestWebSocketMessage(t *testing.T) {
	msg := WebSocketMessage{
		Type:      MessageTypeSystem,
		Timestamp: time.Now().Unix(),
		Data:      json.RawMessage(`{"test":"value"}`),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	var unmarshaled WebSocketMessage
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if unmarshaled.Type != MessageTypeSystem {
		t.Errorf("Expected type %s, got %s", MessageTypeSystem, unmarshaled.Type)
	}
}

// TestConnectionStats tests connection statistics
func TestConnectionStats(t *testing.T) {
	stats := ConnectionStats{
		ID:               "test-id",
		State:            "connected",
		ConnectedAt:      time.Now(),
		LastActivity:     time.Now(),
		ReconnectCount:   2,
		MessagesSent:     100,
		MessagesReceived: 200,
		BytesSent:        1024,
		BytesReceived:    2048,
		MissedHeartbeats: 0,
	}

	if stats.ID != "test-id" {
		t.Errorf("Expected ID test-id, got %s", stats.ID)
	}

	if stats.MessagesSent != 100 {
		t.Errorf("Expected MessagesSent 100, got %d", stats.MessagesSent)
	}
}

// TestGetAllClientStats tests getting all client stats
func TestGetAllClientStats(t *testing.T) {
	hub := NewEnhancedWebSocketHub(DefaultHeartbeatConfig, DefaultReconnectConfig)

	stats := hub.GetAllClientStats()
	if stats == nil {
		t.Error("Expected non-nil stats slice")
	}

	if len(stats) != 0 {
		t.Errorf("Expected 0 clients, got %d", len(stats))
	}
}

// TestGetClientStatsNotFound tests getting non-existent client stats
func TestGetClientStatsNotFound(t *testing.T) {
	hub := NewEnhancedWebSocketHub(DefaultHeartbeatConfig, DefaultReconnectConfig)

	_, err := hub.GetClientStats("non-existent-id")
	if err != ErrConnectionNotFound {
		t.Errorf("Expected ErrConnectionNotFound, got %v", err)
	}
}

// TestGenerateSecureID tests ID generation
func TestGenerateSecureID(t *testing.T) {
	id1 := generateSecureID()
	id2 := generateSecureID()

	if id1 == "" {
		t.Error("Expected non-empty ID")
	}

	if id1 == id2 {
		t.Error("Expected different IDs")
	}

	if len(id1) != 32 {
		t.Errorf("Expected ID length 32, got %d", len(id1))
	}
}

// TestEnhancedWebSocketHandler tests the handler
func TestEnhancedWebSocketHandler(t *testing.T) {
	hub := NewEnhancedWebSocketHub(DefaultHeartbeatConfig, DefaultReconnectConfig)
	go hub.Run()
	defer hub.Stop()

	handler := NewEnhancedWebSocketHandler(hub)
	if handler == nil {
		t.Fatal("Expected non-nil handler")
	}

	if handler.hub != hub {
		t.Error("Expected hub to be set")
	}
}

// TestEnhancedWebSocketHandlerStatus tests status endpoint
func TestEnhancedWebSocketHandlerStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hub := NewEnhancedWebSocketHub(DefaultHeartbeatConfig, DefaultReconnectConfig)
	go hub.Run()
	defer hub.Stop()

	handler := NewEnhancedWebSocketHandler(hub)

	router := gin.New()
	router.GET("/ws/status", handler.GetStatus)

	req := httptest.NewRequest("GET", "/ws/status", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["code"].(float64) != 0 {
		t.Errorf("Expected code 0, got %v", response["code"])
	}
}

// TestEnhancedWebSocketHandlerClientStatus tests client status endpoint
func TestEnhancedWebSocketHandlerClientStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hub := NewEnhancedWebSocketHub(DefaultHeartbeatConfig, DefaultReconnectConfig)
	go hub.Run()
	defer hub.Stop()

	handler := NewEnhancedWebSocketHandler(hub)

	router := gin.New()
	router.GET("/ws/client/:id", handler.GetClientStatus)

	req := httptest.NewRequest("GET", "/ws/client/non-existent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// TestConcurrentBroadcast tests concurrent broadcasting
func TestConcurrentBroadcast(t *testing.T) {
	hub := NewEnhancedWebSocketHub(DefaultHeartbeatConfig, DefaultReconnectConfig)
	go hub.Run()
	defer hub.Stop()

	time.Sleep(10 * time.Millisecond)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hub.Broadcast(MessageTypeMetric, map[string]int{"value": 1})
		}()
	}

	wg.Wait()
}

// TestMessageTypeString tests message type constants
func TestMessageTypeString(t *testing.T) {
	types := []MessageType{
		MessageTypeSystem,
		MessageTypeNotification,
		MessageTypeMetric,
		MessageTypeAlert,
		MessageTypeEvent,
		MessageTypeContainer,
		MessageTypeStorage,
		MessageTypeBackup,
		MessageTypeSync,
	}

	for _, mt := range types {
		if string(mt) == "" {
			t.Errorf("Message type %v should not be empty", mt)
		}
	}
}

// TestNotificationTypes tests notification structures
func TestNotificationTypes(t *testing.T) {
	// SystemNotification
	sysNotif := SystemNotification{
		Title:   "Test",
		Message: "Test message",
		Level:   "info",
	}
	_, err := json.Marshal(sysNotif)
	if err != nil {
		t.Errorf("Failed to marshal SystemNotification: %v", err)
	}

	// MetricUpdate
	metric := MetricUpdate{
		CPU:    50.5,
		Memory: 75.0,
		Disk:   60.0,
	}
	metric.Network.Rx = 1024
	metric.Network.Tx = 2048
	data, err := json.Marshal(metric)
	if err != nil {
		t.Errorf("Failed to marshal MetricUpdate: %v", err)
	}
	t.Logf("MetricUpdate JSON: %s", string(data))

	// AlertNotification
	alert := AlertNotification{
		ID:           "alert-1",
		Type:         "disk",
		Severity:     "warning",
		Title:        "Disk Space Low",
		Message:      "Disk usage above 90%",
		Source:       "monitor",
		Timestamp:    time.Now().Unix(),
		Acknowledged: false,
	}
	_, err = json.Marshal(alert)
	if err != nil {
		t.Errorf("Failed to marshal AlertNotification: %v", err)
	}

	// ContainerEvent
	container := ContainerEvent{
		ContainerID: "container-1",
		Name:        "nginx",
		Action:      "start",
		Status:      "running",
		Timestamp:   time.Now().Unix(),
	}
	_, err = json.Marshal(container)
	if err != nil {
		t.Errorf("Failed to marshal ContainerEvent: %v", err)
	}

	// StorageEvent
	storage := StorageEvent{
		VolumeName: "volume-1",
		EventType:  "mount",
		Message:    "Volume mounted successfully",
		Timestamp:  time.Now().Unix(),
	}
	_, err = json.Marshal(storage)
	if err != nil {
		t.Errorf("Failed to marshal StorageEvent: %v", err)
	}

	// BackupEvent
	backup := BackupEvent{
		JobID:     "job-1",
		Status:    "completed",
		Progress:  100,
		Message:   "Backup completed successfully",
		Timestamp: time.Now().Unix(),
	}
	_, err = json.Marshal(backup)
	if err != nil {
		t.Errorf("Failed to marshal BackupEvent: %v", err)
	}
}

// TestHubStop tests graceful hub shutdown
func TestHubStop(t *testing.T) {
	hub := NewEnhancedWebSocketHub(DefaultHeartbeatConfig, DefaultReconnectConfig)
	go hub.Run()

	time.Sleep(20 * time.Millisecond)

	// Stop should not panic
	hub.Stop()

	// Multiple stops should be safe
	hub.Stop()
}

// TestClientClose tests client close
func TestClientClose(t *testing.T) {
	// Create a minimal test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		time.Sleep(50 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	client := NewEnhancedClient(conn, "test", DefaultHeartbeatConfig, DefaultReconnectConfig)

	// Close should not panic
	client.Close()

	// Multiple closes should be safe
	client.Close()
}

// BenchmarkBroadcast benchmarks broadcast performance
func BenchmarkBroadcast(b *testing.B) {
	hub := NewEnhancedWebSocketHub(DefaultHeartbeatConfig, DefaultReconnectConfig)
	go hub.Run()
	defer hub.Stop()

	time.Sleep(10 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.Broadcast(MessageTypeMetric, map[string]int{"value": i})
	}
}

// BenchmarkGenerateSecureID benchmarks ID generation
func BenchmarkGenerateSecureID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateSecureID()
	}
}
