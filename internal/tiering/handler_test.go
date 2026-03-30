package tiering

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestHandler() (*Handler, *gin.Engine) {
	config := DefaultPolicyEngineConfig()
	// Use a temp file for config to avoid file errors
	manager := NewManager("/tmp/tiering-test-config.json", config)
	// Don't call Initialize() to avoid starting goroutines
	manager.mu.Lock()
	manager.initDefaultTiers()
	manager.mu.Unlock()

	handler := NewHandler(manager)
	router := gin.New()
	api := router.Group("/api")
	handler.RegisterRoutes(api)

	return handler, router
}

func TestNewHandler(t *testing.T) {
	config := DefaultPolicyEngineConfig()
	manager := NewManager("", config)

	handler := NewHandler(manager)
	if handler == nil {
		t.Fatal("NewHandler returned nil")
	}
	if handler.manager == nil {
		t.Error("Handler.manager is nil")
	}
}

func TestHandler_RegisterRoutes(t *testing.T) {
	_, router := setupTestHandler()

	routes := router.Routes()
	if len(routes) == 0 {
		t.Error("No routes registered")
	}

	// Check specific routes exist
	expectedRoutes := []string{
		"/api/tiering/tiers",
		"/api/tiering/policies",
		"/api/tiering/tasks",
		"/api/tiering/status",
		"/api/tiering/stats",
	}

	for _, route := range expectedRoutes {
		found := false
		for _, r := range routes {
			if r.Path == route {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Route %s not found", route)
		}
	}
}

func TestHandler_ListTiers(t *testing.T) {
	_, router := setupTestHandler()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/tiering/tiers", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}

	if response["code"].(float64) != 0 {
		t.Errorf("Expected code 0, got %v", response["code"])
	}
}

func TestHandler_CreateTier_Success(t *testing.T) {
	_, router := setupTestHandler()

	config := TierConfig{
		Type:      TierTypeMemory,
		Name:      "Memory Cache",
		Path:      "/mnt/memory",
		Priority:  200,
		Enabled:   true,
		Threshold: 70,
	}

	body, _ := json.Marshal(config)
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/tiering/tiers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_CreateTier_InvalidJSON(t *testing.T) {
	_, router := setupTestHandler()

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/tiering/tiers", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandler_CreateTier_EmptyType(t *testing.T) {
	_, router := setupTestHandler()

	config := TierConfig{
		Name:      "Test Tier",
		Path:      "/mnt/test",
		Priority:  100,
		Enabled:   true,
		Threshold: 80,
	}

	body, _ := json.Marshal(config)
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/tiering/tiers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for empty type, got %d", w.Code)
	}
}

func TestHandler_GetTier_Success(t *testing.T) {
	_, router := setupTestHandler()

	// SSD tier should exist by default
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/tiering/tiers/ssd", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Error("Response data is not a map")
		return
	}

	if data["type"].(string) != "ssd" {
		t.Errorf("Expected tier type 'ssd', got %v", data["type"])
	}
}

func TestHandler_GetTier_NotFound(t *testing.T) {
	_, router := setupTestHandler()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/tiering/tiers/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandler_UpdateTier_Success(t *testing.T) {
	_, router := setupTestHandler()

	config := TierConfig{
		Name:      "Updated SSD",
		Path:      "/mnt/ssd-updated",
		Priority:  150,
		Enabled:   true,
		Threshold: 85,
	}

	body, _ := json.Marshal(config)
	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/api/tiering/tiers/ssd", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UpdateTier_NotFound(t *testing.T) {
	_, router := setupTestHandler()

	config := TierConfig{
		Name:      "Test",
		Path:      "/mnt/test",
		Priority:  100,
		Enabled:   true,
		Threshold: 80,
	}

	body, _ := json.Marshal(config)
	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/api/tiering/tiers/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestHandler_DeleteTier_Success(t *testing.T) {
	_, router := setupTestHandler()

	// First create a tier to delete
	config := TierConfig{
		Type:      TierTypeMemory,
		Name:      "Memory",
		Path:      "/mnt/memory",
		Priority:  200,
		Enabled:   true,
		Threshold: 70,
	}
	body, _ := json.Marshal(config)
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/tiering/tiers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Now delete it
	req, _ = http.NewRequestWithContext(context.Background(), "DELETE", "/api/tiering/tiers/memory", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandler_DeleteTier_NotFound(t *testing.T) {
	_, router := setupTestHandler()

	req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/tiering/tiers/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestHandler_ListPolicies(t *testing.T) {
	_, router := setupTestHandler()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/tiering/policies", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}
}

func TestHandler_CreatePolicy_Success(t *testing.T) {
	_, router := setupTestHandler()

	policy := Policy{
		Name:        "Test Policy",
		Description: "Test policy description",
		Enabled:     true,
		SourceTier:  TierTypeSSD,
		TargetTier:  TierTypeHDD,
		Action:      PolicyActionMove,
	}

	body, _ := json.Marshal(policy)
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/tiering/policies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_CreatePolicy_InvalidJSON(t *testing.T) {
	_, router := setupTestHandler()

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/tiering/policies", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandler_GetPolicy_NotFound(t *testing.T) {
	_, router := setupTestHandler()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/tiering/policies/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandler_UpdatePolicy_NotFound(t *testing.T) {
	_, router := setupTestHandler()

	policy := Policy{
		Name:       "Updated Policy",
		Enabled:    true,
		SourceTier: TierTypeSSD,
		TargetTier: TierTypeHDD,
		Action:     PolicyActionMove,
	}

	body, _ := json.Marshal(policy)
	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/api/tiering/policies/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestHandler_DeletePolicy_NotFound(t *testing.T) {
	_, router := setupTestHandler()

	req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/tiering/policies/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestHandler_ExecutePolicy_NotFound(t *testing.T) {
	_, router := setupTestHandler()

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/tiering/policies/nonexistent/execute", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestHandler_Migrate_InvalidJSON(t *testing.T) {
	_, router := setupTestHandler()

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/tiering/migrate", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandler_ListTasks(t *testing.T) {
	_, router := setupTestHandler()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/tiering/tasks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}
}

func TestHandler_GetTask_NotFound(t *testing.T) {
	_, router := setupTestHandler()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/tiering/tasks/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandler_CancelTask_NotFound(t *testing.T) {
	_, router := setupTestHandler()

	req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/tiering/tasks/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestHandler_GetStatus(t *testing.T) {
	_, router := setupTestHandler()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/tiering/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Error("Response data is not a map")
		return
	}

	// Check status fields
	if _, ok := data["enabled"]; !ok {
		t.Error("Status should have 'enabled' field")
	}
	if _, ok := data["tiers"]; !ok {
		t.Error("Status should have 'tiers' field")
	}
}

func TestHandler_GetStats(t *testing.T) {
	_, router := setupTestHandler()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/tiering/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}
}

func TestHandler_GetTierStats_Success(t *testing.T) {
	_, router := setupTestHandler()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/tiering/stats/ssd", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandler_GetTierStats_NotFound(t *testing.T) {
	_, router := setupTestHandler()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/tiering/stats/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// Test types.
func TestTierType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		tierType TierType
		expected string
	}{
		{"SSD", TierTypeSSD, "ssd"},
		{"HDD", TierTypeHDD, "hdd"},
		{"Cloud", TierTypeCloud, "cloud"},
		{"Memory", TierTypeMemory, "memory"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.tierType) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.tierType)
			}
		})
	}
}

func TestPolicyAction_Constants(t *testing.T) {
	tests := []struct {
		name     string
		action   PolicyAction
		expected string
	}{
		{"Move", PolicyActionMove, "move"},
		{"Copy", PolicyActionCopy, "copy"},
		{"Archive", PolicyActionArchive, "archive"},
		{"Delete", PolicyActionDelete, "delete"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.action) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.action)
			}
		})
	}
}

func TestMigrateStatus_Constants(t *testing.T) {
	tests := []struct {
		name     string
		status   MigrateStatus
		expected string
	}{
		{"Pending", MigrateStatusPending, "pending"},
		{"Running", MigrateStatusRunning, "running"},
		{"Completed", MigrateStatusCompleted, "completed"},
		{"Failed", MigrateStatusFailed, "failed"},
		{"Cancelled", MigrateStatusCancelled, "cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.status)
			}
		})
	}
}

func TestAccessFrequency_Constants(t *testing.T) {
	tests := []struct {
		name      string
		frequency AccessFrequency
		expected  string
	}{
		{"Hot", AccessFrequencyHot, "hot"},
		{"Warm", AccessFrequencyWarm, "warm"},
		{"Cold", AccessFrequencyCold, "cold"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.frequency) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.frequency)
			}
		})
	}
}

func TestPolicyStatus_Constants(t *testing.T) {
	tests := []struct {
		name     string
		status   PolicyStatus
		expected string
	}{
		{"Enabled", PolicyStatusEnabled, "enabled"},
		{"Disabled", PolicyStatusDisabled, "disabled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.status)
			}
		})
	}
}

func TestScheduleType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		schedule ScheduleType
		expected string
	}{
		{"Manual", ScheduleTypeManual, "manual"},
		{"Interval", ScheduleTypeInterval, "interval"},
		{"Cron", ScheduleTypeCron, "cron"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.schedule) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.schedule)
			}
		})
	}
}

func TestTierConfig_JSON(t *testing.T) {
	config := TierConfig{
		Type:       TierTypeSSD,
		Name:       "SSD Cache",
		Path:       "/mnt/ssd",
		Capacity:   1000000000,
		Used:       500000000,
		Threshold:  80,
		Priority:   100,
		Enabled:    true,
		ProviderID: "provider-123",
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal TierConfig: %v", err)
	}

	var unmarshaled TierConfig
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal TierConfig: %v", err)
	}

	if unmarshaled.Type != config.Type {
		t.Errorf("Type mismatch: expected %s, got %s", config.Type, unmarshaled.Type)
	}
	if unmarshaled.Name != config.Name {
		t.Errorf("Name mismatch: expected %s, got %s", config.Name, unmarshaled.Name)
	}
	if unmarshaled.Capacity != config.Capacity {
		t.Errorf("Capacity mismatch: expected %d, got %d", config.Capacity, unmarshaled.Capacity)
	}
}

func TestPolicy_JSON(t *testing.T) {
	policy := Policy{
		ID:          "policy-1",
		Name:        "Test Policy",
		Description: "Test description",
		Enabled:     true,
		Status:      PolicyStatusEnabled,
		SourceTier:  TierTypeSSD,
		TargetTier:  TierTypeHDD,
		Action:      PolicyActionMove,
	}

	data, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("Failed to marshal Policy: %v", err)
	}

	var unmarshaled Policy
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal Policy: %v", err)
	}

	if unmarshaled.ID != policy.ID {
		t.Errorf("ID mismatch: expected %s, got %s", policy.ID, unmarshaled.ID)
	}
	if unmarshaled.Name != policy.Name {
		t.Errorf("Name mismatch: expected %s, got %s", policy.Name, unmarshaled.Name)
	}
}

func TestMigrateTask_JSON(t *testing.T) {
	task := MigrateTask{
		ID:         "task-1",
		PolicyID:   "policy-1",
		Status:     MigrateStatusRunning,
		SourceTier: TierTypeSSD,
		TargetTier: TierTypeHDD,
		Action:     PolicyActionMove,
		TotalFiles: 100,
		TotalBytes: 1000000000,
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("Failed to marshal MigrateTask: %v", err)
	}

	var unmarshaled MigrateTask
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal MigrateTask: %v", err)
	}

	if unmarshaled.ID != task.ID {
		t.Errorf("ID mismatch: expected %s, got %s", task.ID, unmarshaled.ID)
	}
	if unmarshaled.TotalFiles != task.TotalFiles {
		t.Errorf("TotalFiles mismatch: expected %d, got %d", task.TotalFiles, unmarshaled.TotalFiles)
	}
}

func TestMigrateRequest_JSON(t *testing.T) {
	req := MigrateRequest{
		Paths:      []string{"/path/1", "/path/2"},
		SourceTier: TierTypeSSD,
		TargetTier: TierTypeHDD,
		Action:     PolicyActionMove,
		DryRun:     true,
		Preserve:   false,
		Pattern:    "*.txt",
		MinSize:    1024,
		MaxSize:    10240,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal MigrateRequest: %v", err)
	}

	var unmarshaled MigrateRequest
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal MigrateRequest: %v", err)
	}

	if len(unmarshaled.Paths) != len(req.Paths) {
		t.Errorf("Paths length mismatch: expected %d, got %d", len(req.Paths), len(unmarshaled.Paths))
	}
	if unmarshaled.SourceTier != req.SourceTier {
		t.Errorf("SourceTier mismatch: expected %s, got %s", req.SourceTier, unmarshaled.SourceTier)
	}
}

func TestStatus_JSON(t *testing.T) {
	status := Status{
		Enabled:      true,
		RunningTasks: 2,
		PendingTasks: 5,
		Policies:     10,
		ActivePolicy: 3,
		Tiers:        make(map[TierType]*TierConfig),
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("Failed to marshal Status: %v", err)
	}

	var unmarshaled Status
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal Status: %v", err)
	}

	if unmarshaled.Enabled != status.Enabled {
		t.Errorf("Enabled mismatch: expected %v, got %v", status.Enabled, unmarshaled.Enabled)
	}
	if unmarshaled.RunningTasks != status.RunningTasks {
		t.Errorf("RunningTasks mismatch: expected %d, got %d", status.RunningTasks, unmarshaled.RunningTasks)
	}
}
