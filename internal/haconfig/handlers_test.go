package haconfig

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func setupTestHandler(t *testing.T) *Handler {
	t.Helper()

	logger, _ := zap.NewDevelopment()
	tmpDir := t.TempDir()
	config := &HAConfig{
		ClusterName:       "test-cluster",
		NodeID:            "node-1",
		NodeRole:          RolePrimary,
		Priority:          100,
		HeartbeatEnabled:  false,
		HealthCheckEnabled: false,
		SyncEnabled:       false,
		FailoverEnabled:   true,
		FailbackEnabled:   true,
		PeerNodes:         []string{"node-2", "node-3"},
		BindAddress:       "192.168.1.100",
		DataDir:           tmpDir,
	}

	manager, err := NewHAConfigManager(config, logger)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	return NewHandler(manager)
}

func TestHandleStatus(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ha/status", nil)
	w := httptest.NewRecorder()

	handler.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response HAStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.State != HAStateActive {
		t.Errorf("expected state 'active', got '%s'", response.State)
	}
}

func TestHandleStatusMethodNotAllowed(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ha/status", nil)
	w := httptest.NewRecorder()

	handler.handleStatus(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandleGetConfig(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ha/config", nil)
	w := httptest.NewRecorder()

	handler.handleConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response HAConfigResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.VirtualIP != "192.168.1.100" {
		t.Errorf("expected virtual IP '192.168.1.100', got '%s'", response.VirtualIP)
	}
}

func TestHandleUpdateConfig(t *testing.T) {
	handler := setupTestHandler(t)

	newIP := "192.168.1.200"
	reqBody := HAConfigUpdateRequest{
		VirtualIP: &newIP,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/ha/config", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.handleConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify config was updated
	config := handler.manager.GetConfig()
	if config.BindAddress != "192.168.1.200" {
		t.Errorf("expected bind address '192.168.1.200', got '%s'", config.BindAddress)
	}
}

func TestHandleUpdateConfigInvalidBody(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/ha/config", bytes.NewReader([]byte("invalid")))
	w := httptest.NewRecorder()

	handler.handleConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleFailover(t *testing.T) {
	handler := setupTestHandler(t)

	reqBody := FailoverRequest{
		TargetNode: "node-2",
		Reason:     "test",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ha/failover", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.handleFailover(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response FailoverResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Errorf("expected success, got false")
	}
}

func TestHandleFailoverMethodNotAllowed(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ha/failover", nil)
	w := httptest.NewRecorder()

	handler.handleFailover(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandleFailback(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ha/failback", nil)
	w := httptest.NewRecorder()

	handler.handleFailback(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response FailbackResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Errorf("expected success, got false")
	}
}

func TestHandleFailbackMethodNotAllowed(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ha/failback", nil)
	w := httptest.NewRecorder()

	handler.handleFailback(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandleNodes(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ha/nodes", nil)
	w := httptest.NewRecorder()

	handler.handleNodes(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response NodesResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Total < 1 {
		t.Errorf("expected at least 1 node, got %d", response.Total)
	}
}

func TestHandleNodesMethodNotAllowed(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ha/nodes", nil)
	w := httptest.NewRecorder()

	handler.handleNodes(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandleEvents(t *testing.T) {
	handler := setupTestHandler(t)

	// Add an event first
	handler.addEvent(EventTypeHeartbeat, "test event", "node-1")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ha/events", nil)
	w := httptest.NewRecorder()

	handler.handleEvents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response EventsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Total != 1 {
		t.Errorf("expected 1 event, got %d", response.Total)
	}
}

func TestHandleEventsMethodNotAllowed(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ha/events", nil)
	w := httptest.NewRecorder()

	handler.handleEvents(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandleTest(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ha/test", nil)
	w := httptest.NewRecorder()

	handler.handleTest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response TestResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

func TestHandleTestMethodNotAllowed(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ha/test", nil)
	w := httptest.NewRecorder()

	handler.handleTest(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestAddEvent(t *testing.T) {
	handler := setupTestHandler(t)

	handler.addEvent(EventTypeFailover, "test failover", "node-1")
	handler.addEvent(EventTypeHeartbeat, "test heartbeat", "node-2")

	if len(handler.events) != 2 {
		t.Errorf("expected 2 events, got %d", len(handler.events))
	}

	if handler.events[0].Type != EventTypeFailover {
		t.Errorf("expected event type '%s', got '%s'", EventTypeFailover, handler.events[0].Type)
	}
}

func TestDetermineHAState(t *testing.T) {
	handler := setupTestHandler(t)

	tests := []struct {
		name     string
		status   HAStatus
		expected HAState
	}{
		{
			name: "active when primary",
			status: HAStatus{
				CurrentRole: RolePrimary,
			},
			expected: HAStateActive,
		},
		{
			name: "standby when secondary",
			status: HAStatus{
				CurrentRole: RoleSecondary,
			},
			expected: HAStateStandby,
		},
		{
			name: "failover when active",
			status: HAStatus{
				IsFailoverActive: true,
			},
			expected: HAStateFailover,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.determineHAState(tt.status)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestFindSecondaryNode(t *testing.T) {
	handler := setupTestHandler(t)

	nodes := map[string]NodeState{
		"node-1": {NodeID: "node-1", Role: RolePrimary},
		"node-2": {NodeID: "node-2", Role: RoleSecondary},
	}

	result := handler.findSecondaryNode(nodes)
	if result != "node-2" {
		t.Errorf("expected 'node-2', got '%s'", result)
	}
}

func TestFindSecondaryNodeNone(t *testing.T) {
	handler := setupTestHandler(t)

	nodes := map[string]NodeState{
		"node-1": {NodeID: "node-1", Role: RolePrimary},
	}

	result := handler.findSecondaryNode(nodes)
	if result != "" {
		t.Errorf("expected empty string, got '%s'", result)
	}
}

func TestWriteJSON(t *testing.T) {
	handler := setupTestHandler(t)

	w := httptest.NewRecorder()
	handler.writeJSON(w, http.StatusOK, SuccessResponse{
		Success: true,
		Message: "test",
	})

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected content type 'application/json', got '%s'", w.Header().Get("Content-Type"))
	}
}

func TestWriteError(t *testing.T) {
	handler := setupTestHandler(t)

	w := httptest.NewRecorder()
	handler.writeError(w, http.StatusBadRequest, "test error")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var response ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Message != "test error" {
		t.Errorf("expected message 'test error', got '%s'", response.Message)
	}
}
