// Package api provides tests for enhanced WebSocket room functionality
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

// TestRoom tests room creation and basic operations
func TestRoom(t *testing.T) {
	room := NewRoom("test-room", "Test Room", "A test room", 10)

	if room.ID != "test-room" {
		t.Errorf("Expected ID 'test-room', got %s", room.ID)
	}

	if room.Name != "Test Room" {
		t.Errorf("Expected name 'Test Room', got %s", room.Name)
	}

	if room.MaxClients != 10 {
		t.Errorf("Expected MaxClients 10, got %d", room.MaxClients)
	}

	if room.GetClientCount() != 0 {
		t.Errorf("Expected 0 clients, got %d", room.GetClientCount())
	}
}

// TestRoomDefaultMaxClients tests default max clients
func TestRoomDefaultMaxClients(t *testing.T) {
	room := NewRoom("test", "Test", "", 0)

	if room.MaxClients != 100 {
		t.Errorf("Expected default MaxClients 100, got %d", room.MaxClients)
	}
}

// TestMessageQueue tests message queue operations
func TestMessageQueue(t *testing.T) {
	queue := NewMessageQueue(MessageQueueConfig{
		MaxSize: 10,
	})

	// Test enqueue
	msg := &PersistedMessage{
		ID:        "msg-1",
		Type:      MessageTypeSystem,
		Timestamp: time.Now().Unix(),
		Data:      json.RawMessage(`{"test":"value"}`),
	}

	err := queue.Enqueue(msg)
	if err != nil {
		t.Errorf("Failed to enqueue: %v", err)
	}

	if queue.Size() != 1 {
		t.Errorf("Expected size 1, got %d", queue.Size())
	}

	// Test dequeue
	dequeued, err := queue.Dequeue()
	if err != nil {
		t.Errorf("Failed to dequeue: %v", err)
	}

	if dequeued.ID != "msg-1" {
		t.Errorf("Expected ID 'msg-1', got %s", dequeued.ID)
	}

	if queue.Size() != 0 {
		t.Errorf("Expected size 0, got %d", queue.Size())
	}
}

// TestMessageQueueCapacity tests queue capacity
func TestMessageQueueCapacity(t *testing.T) {
	queue := NewMessageQueue(MessageQueueConfig{
		MaxSize: 5,
	})

	// Add more than capacity
	for i := 0; i < 10; i++ {
		msg := &PersistedMessage{
			ID:        string(rune('a' + i)),
			Timestamp: time.Now().Unix(),
		}
		queue.Enqueue(msg)
	}

	// Should only keep last 5
	if queue.Size() != 5 {
		t.Errorf("Expected size 5, got %d", queue.Size())
	}
}

// TestMessageQueueGetMessages tests getting messages
func TestMessageQueueGetMessages(t *testing.T) {
	queue := NewMessageQueue(MessageQueueConfig{MaxSize: 10})

	// Add messages for different rooms
	queue.Enqueue(&PersistedMessage{ID: "1", RoomID: "room-a"})
	queue.Enqueue(&PersistedMessage{ID: "2", RoomID: "room-b"})
	queue.Enqueue(&PersistedMessage{ID: "3", RoomID: "room-a"})

	// Get messages for room-a
	messages := queue.GetMessages("room-a")
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages for room-a, got %d", len(messages))
	}

	// Get all messages
	allMessages := queue.GetMessages("")
	if len(allMessages) != 3 {
		t.Errorf("Expected 3 total messages, got %d", len(allMessages))
	}
}

// TestMessageQueueGetMessagesForUser tests getting user messages
func TestMessageQueueGetMessagesForUser(t *testing.T) {
	queue := NewMessageQueue(MessageQueueConfig{MaxSize: 10})

	// Add messages for different users
	queue.Enqueue(&PersistedMessage{ID: "1", UserID: "user-a", Delivered: false})
	queue.Enqueue(&PersistedMessage{ID: "2", UserID: "user-b", Delivered: false})
	queue.Enqueue(&PersistedMessage{ID: "3", UserID: "user-a", Delivered: true})

	// Get undelivered messages for user-a
	messages := queue.GetMessagesForUser("user-a")
	if len(messages) != 1 {
		t.Errorf("Expected 1 undelivered message for user-a, got %d", len(messages))
	}
}

// TestMessageQueueMarkDelivered tests marking delivered
func TestMessageQueueMarkDelivered(t *testing.T) {
	queue := NewMessageQueue(MessageQueueConfig{MaxSize: 10})

	queue.Enqueue(&PersistedMessage{ID: "msg-1", Delivered: false})

	queue.MarkDelivered("msg-1")

	messages := queue.GetMessagesForUser("")
	for _, m := range messages {
		if m.ID == "msg-1" && !m.Delivered {
			t.Error("Expected message to be marked as delivered")
		}
	}
}

// TestMessageQueueCleanup tests cleanup
func TestMessageQueueCleanup(t *testing.T) {
	queue := NewMessageQueue(MessageQueueConfig{
		MaxSize: 10,
		TTL:     1 * time.Second,
	})

	// Add old message
	oldMsg := &PersistedMessage{
		ID:        "old",
		Timestamp: time.Now().Add(-10 * time.Second).Unix(),
	}
	queue.Enqueue(oldMsg)

	// Add new message
	newMsg := &PersistedMessage{
		ID:        "new",
		Timestamp: time.Now().Unix(),
	}
	queue.Enqueue(newMsg)

	// Cleanup
	removed := queue.Cleanup()
	if removed != 1 {
		t.Errorf("Expected 1 removed, got %d", removed)
	}

	if queue.Size() != 1 {
		t.Errorf("Expected size 1, got %d", queue.Size())
	}
}

// TestEnhancedWebSocketHubRoom tests hub room operations
func TestEnhancedWebSocketHubRoom(t *testing.T) {
	hub := NewEnhancedWebSocketHub(DefaultHeartbeatConfig, DefaultReconnectConfig)
	go hub.Run()
	defer hub.Stop()

	time.Sleep(10 * time.Millisecond)

	// Create room
	room := hub.CreateRoom("test-room", "Test", "Test room", 10)
	if room == nil {
		t.Fatal("Failed to create room")
	}

	// Get room
	gotRoom, err := hub.GetRoom("test-room")
	if err != nil {
		t.Errorf("Failed to get room: %v", err)
	}

	if gotRoom.ID != "test-room" {
		t.Errorf("Expected room ID 'test-room', got %s", gotRoom.ID)
	}

	// Get non-existent room
	_, err = hub.GetRoom("non-existent")
	if err != ErrRoomNotFound {
		t.Errorf("Expected ErrRoomNotFound, got %v", err)
	}
}

// TestEnhancedWebSocketHubDeleteRoom tests room deletion
func TestEnhancedWebSocketHubDeleteRoom(t *testing.T) {
	hub := NewEnhancedWebSocketHub(DefaultHeartbeatConfig, DefaultReconnectConfig)
	go hub.Run()
	defer hub.Stop()

	time.Sleep(10 * time.Millisecond)

	// Create and delete room
	hub.CreateRoom("test-room", "Test", "", 10)
	err := hub.DeleteRoom("test-room")
	if err != nil {
		t.Errorf("Failed to delete room: %v", err)
	}

	// Try to get deleted room
	_, err = hub.GetRoom("test-room")
	if err != ErrRoomNotFound {
		t.Errorf("Expected ErrRoomNotFound, got %v", err)
	}

	// Delete non-existent room
	err = hub.DeleteRoom("non-existent")
	if err != ErrRoomNotFound {
		t.Errorf("Expected ErrRoomNotFound, got %v", err)
	}
}

// TestEnhancedWebSocketHubRoomStats tests room stats
func TestEnhancedWebSocketHubRoomStats(t *testing.T) {
	hub := NewEnhancedWebSocketHub(DefaultHeartbeatConfig, DefaultReconnectConfig)
	go hub.Run()
	defer hub.Stop()

	time.Sleep(10 * time.Millisecond)

	// Create rooms
	hub.CreateRoom("room-1", "Room 1", "", 10)
	hub.CreateRoom("room-2", "Room 2", "", 20)

	stats := hub.GetRoomStats()
	if len(stats) != 2 {
		t.Errorf("Expected 2 rooms, got %d", len(stats))
	}
}

// TestEnhancedWebSocketHubQueueStats tests queue stats
func TestEnhancedWebSocketHubQueueStats(t *testing.T) {
	hub := NewEnhancedWebSocketHub(DefaultHeartbeatConfig, DefaultReconnectConfig)

	stats := hub.GetQueueStats()
	if stats == nil {
		t.Fatal("Expected non-nil stats")
	}

	if _, ok := stats["size"]; !ok {
		t.Error("Expected 'size' in stats")
	}
}

// TestBroadcastConfig tests broadcast configuration
func TestBroadcastConfig(t *testing.T) {
	config := BroadcastConfig{
		BatchSize:     50,
		BatchTimeout:  5 * time.Millisecond,
		WorkerCount:   2,
	}

	if config.BatchSize != 50 {
		t.Errorf("Expected BatchSize 50, got %d", config.BatchSize)
	}

	// Test default
	defaultConfig := DefaultBroadcastConfig
	if defaultConfig.BatchSize != 100 {
		t.Errorf("Expected default BatchSize 100, got %d", defaultConfig.BatchSize)
	}
}

// TestNewEnhancedWebSocketHubWithConfig tests hub creation with config
func TestNewEnhancedWebSocketHubWithConfig(t *testing.T) {
	heartbeat := HeartbeatConfig{PingInterval: 10 * time.Second}
	reconnect := ReconnectConfig{MaxAttempts: 3}
	broadcast := BroadcastConfig{BatchSize: 50}
	queueConfig := MessageQueueConfig{MaxSize: 500}

	hub := NewEnhancedWebSocketHubWithConfig(heartbeat, reconnect, broadcast, queueConfig)

	if hub == nil {
		t.Fatal("Expected non-nil hub")
	}

	if hub.messageQueue.config.MaxSize != 500 {
		t.Errorf("Expected queue max size 500, got %d", hub.messageQueue.config.MaxSize)
	}
}

// TestRoomJoinLeave tests client join/leave room
func TestRoomJoinLeave(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	// Connect client
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Setup hub
	hub := NewEnhancedWebSocketHub(DefaultHeartbeatConfig, DefaultReconnectConfig)
	go hub.Run()
	defer hub.Stop()

	time.Sleep(10 * time.Millisecond)

	// Create client and room
	client := NewEnhancedClient(conn, "test-user", DefaultHeartbeatConfig, DefaultReconnectConfig)
	hub.register <- client
	time.Sleep(20 * time.Millisecond)

	hub.CreateRoom("test-room", "Test", "", 10)

	// Join room
	err = hub.JoinRoom("test-room", client.ID)
	if err != nil {
		t.Errorf("Failed to join room: %v", err)
	}

	// Check client is in room
	client.mu.RLock()
	inRoom := client.Rooms["test-room"]
	client.mu.RUnlock()

	if !inRoom {
		t.Error("Expected client to be in room")
	}

	// Leave room
	err = hub.LeaveRoom("test-room", client.ID)
	if err != nil {
		t.Errorf("Failed to leave room: %v", err)
	}

	// Check client is not in room
	client.mu.RLock()
	inRoom = client.Rooms["test-room"]
	client.mu.RUnlock()

	if inRoom {
		t.Error("Expected client to not be in room")
	}
}

// TestRoomBroadcast tests broadcasting to room
func TestRoomBroadcast(t *testing.T) {
	hub := NewEnhancedWebSocketHub(DefaultHeartbeatConfig, DefaultReconnectConfig)
	go hub.Run()
	defer hub.Stop()

	time.Sleep(10 * time.Millisecond)

	// Create room
	hub.CreateRoom("test-room", "Test", "", 10)

	// Broadcast to empty room should work
	err := hub.BroadcastToRoom("test-room", MessageTypeSystem, map[string]string{"test": "value"}, "")
	if err != nil {
		t.Errorf("Failed to broadcast to room: %v", err)
	}

	// Broadcast to non-existent room should fail
	err = hub.BroadcastToRoom("non-existent", MessageTypeSystem, nil, "")
	if err != ErrRoomNotFound {
		t.Errorf("Expected ErrRoomNotFound, got %v", err)
	}
}

// TestEnhancedWebSocketHandlerRoomAPI tests room API endpoints
func TestEnhancedWebSocketHandlerRoomAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hub := NewEnhancedWebSocketHub(DefaultHeartbeatConfig, DefaultReconnectConfig)
	go hub.Run()
	defer hub.Stop()

	time.Sleep(10 * time.Millisecond)

	router := gin.New()
	RegisterEnhancedWebSocketRoutes(router.Group("/api"), hub)

	// Test create room
	req := httptest.NewRequest("POST", "/api/ws/rooms", strings.NewReader(`{"id":"test-room","name":"Test Room"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test get rooms
	req = httptest.NewRequest("GET", "/api/ws/rooms", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test get specific room
	req = httptest.NewRequest("GET", "/api/ws/rooms/test-room", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test delete room
	req = httptest.NewRequest("DELETE", "/api/ws/rooms/test-room", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test get queue stats
	req = httptest.NewRequest("GET", "/api/ws/queue", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestConcurrentRoomOperations tests concurrent room access
func TestConcurrentRoomOperations(t *testing.T) {
	hub := NewEnhancedWebSocketHub(DefaultHeartbeatConfig, DefaultReconnectConfig)
	go hub.Run()
	defer hub.Stop()

	time.Sleep(10 * time.Millisecond)

	// Create rooms concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			hub.CreateRoom(string(rune('a'+n)), "Room", "", 10)
		}(i)
	}
	wg.Wait()

	stats := hub.GetRoomStats()
	if len(stats) != 10 {
		t.Errorf("Expected 10 rooms, got %d", len(stats))
	}
}

// BenchmarkRoomOperations benchmarks room operations
func BenchmarkRoomOperations(b *testing.B) {
	hub := NewEnhancedWebSocketHub(DefaultHeartbeatConfig, DefaultReconnectConfig)
	go hub.Run()
	defer hub.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.CreateRoom(string(rune(i%26+'a')), "Room", "", 10)
	}
}

// BenchmarkMessageQueue benchmarks message queue
func BenchmarkMessageQueue(b *testing.B) {
	queue := NewMessageQueue(MessageQueueConfig{MaxSize: 10000})

	msg := &PersistedMessage{
		ID:        "test",
		Timestamp: time.Now().Unix(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queue.Enqueue(msg)
	}
}

// BenchmarkBroadcastToRoom benchmarks room broadcast
func BenchmarkBroadcastToRoom(b *testing.B) {
	hub := NewEnhancedWebSocketHub(DefaultHeartbeatConfig, DefaultReconnectConfig)
	go hub.Run()
	defer hub.Stop()

	hub.CreateRoom("test-room", "Test", "", 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.BroadcastToRoom("test-room", MessageTypeSystem, map[string]int{"value": i}, "")
	}
}