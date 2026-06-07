package guidedalert

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestManager() *Manager {
	return NewManager(slog.Default())
}

func setupTestRouter(m *Manager) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")
	h := NewHandler(m)
	h.RegisterRoutes(v1)
	return r
}

// Manager Tests

func TestManager_Fire(t *testing.T) {
	m := setupTestManager()

	alert := m.Fire("smart_warning", "磁盘sda检测到坏扇区")
	if alert == nil {
		t.Fatal("Fire returned nil")
	}
	if alert.Title != "smart_warning" {
		t.Errorf("expected title smart_warning, got %s", alert.Title)
	}
	if alert.Severity != SeverityWarning {
		t.Errorf("expected severity warning, got %s", alert.Severity)
	}
	if alert.Category != CategoryHardware {
		t.Errorf("expected category hardware, got %s", alert.Category)
	}
	if alert.Acknowledged {
		t.Error("expected not acknowledged")
	}
	if alert.Silenced {
		t.Error("expected not silenced")
	}
	if alert.TroubleshootingGuide == nil {
		t.Error("expected troubleshooting guide")
	}
	if len(alert.TroubleshootingGuide.Steps) == 0 {
		t.Error("expected troubleshooting steps")
	}
}

func TestManager_FireUnknownRule(t *testing.T) {
	m := setupTestManager()

	alert := m.Fire("unknown_rule", "test message")
	if alert == nil {
		t.Fatal("Fire returned nil")
	}
	if alert.Severity != SeverityWarning {
		t.Errorf("expected default severity warning, got %s", alert.Severity)
	}
}

func TestManager_Acknowledge(t *testing.T) {
	m := setupTestManager()
	alert := m.Fire("smart_warning", "test")

	err := m.Acknowledge(alert.ID)
	if err != nil {
		t.Fatalf("Acknowledge failed: %v", err)
	}

	updated, ok := m.Get(alert.ID)
	if !ok {
		t.Fatal("alert not found")
	}
	if !updated.Acknowledged {
		t.Error("expected acknowledged")
	}
}

func TestManager_AcknowledgeNotFound(t *testing.T) {
	m := setupTestManager()
	err := m.Acknowledge("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent alert")
	}
}

func TestManager_Silence(t *testing.T) {
	m := setupTestManager()
	alert := m.Fire("smart_warning", "test")

	err := m.Silence(alert.ID)
	if err != nil {
		t.Fatalf("Silence failed: %v", err)
	}

	updated, ok := m.Get(alert.ID)
	if !ok {
		t.Fatal("alert not found")
	}
	if !updated.Silenced {
		t.Error("expected silenced")
	}
}

func TestManager_SilenceNotFound(t *testing.T) {
	m := setupTestManager()
	err := m.Silence("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent alert")
	}
}

func TestManager_CorrelateAlerts(t *testing.T) {
	m := setupTestManager()

	alert1 := m.Fire("smart_warning", "disk sda issue") // hardware
	alert2 := m.Fire("pool_degraded", "pool degraded")  // storage
	alert3 := m.Fire("disk_space_low", "space low")     // storage

	// alert2 and alert3 are both storage category, should be correlated
	found2to3 := false
	for _, id := range alert2.RelatedAlertIDs {
		if id == alert3.ID {
			found2to3 = true
			break
		}
	}
	found3to2 := false
	for _, id := range alert3.RelatedAlertIDs {
		if id == alert2.ID {
			found3to2 = true
			break
		}
	}
	if !found2to3 || !found3to2 {
		t.Error("expected storage alerts to be correlated")
	}

	// alert1 is hardware, should not be related to storage alerts
	for _, id := range alert1.RelatedAlertIDs {
		if id == alert2.ID || id == alert3.ID {
			t.Error("hardware alert should not be related to storage alerts")
		}
	}
}

func TestManager_Summary(t *testing.T) {
	m := setupTestManager()

	m.Fire("smart_warning", "test1")
	m.Fire("pool_degraded", "test2")
	m.Fire("disk_space_low", "test3")
	m.Fire("network_down", "test4")
	m.Fire("high_cpu", "test5")

	summary := m.Summary()
	if summary.Total != 5 {
		t.Errorf("expected total 5, got %d", summary.Total)
	}
	if summary.ByCategory[CategoryStorage] != 2 {
		t.Errorf("expected 2 storage alerts, got %d", summary.ByCategory[CategoryStorage])
	}
	if summary.ByCategory[CategoryNetwork] != 1 {
		t.Errorf("expected 1 network alert, got %d", summary.ByCategory[CategoryNetwork])
	}
	if summary.ByCategory[CategoryPerformance] != 1 {
		t.Errorf("expected 1 performance alert, got %d", summary.ByCategory[CategoryPerformance])
	}
	if summary.BySeverity[SeverityWarning] != 3 {
		t.Errorf("expected 3 warning alerts, got %d", summary.BySeverity[SeverityWarning])
	}
	if summary.BySeverity[SeverityCritical] != 2 {
		t.Errorf("expected 2 critical alerts, got %d", summary.BySeverity[SeverityCritical])
	}
}

func TestManager_List(t *testing.T) {
	m := setupTestManager()

	m.Fire("smart_warning", "test1")
	m.Fire("pool_degraded", "test2")

	alerts := m.List()
	if len(alerts) != 2 {
		t.Errorf("expected 2 alerts, got %d", len(alerts))
	}
}

func TestManager_RegisterRule(t *testing.T) {
	m := setupTestManager()

	rule := &AlertRule{
		Name:      "custom_rule",
		Condition: "custom condition",
		Severity:  SeverityInfo,
		Category:  CategorySecurity,
	}
	m.RegisterRule(rule)

	alert := m.Fire("custom_rule", "test")
	if alert.Severity != SeverityInfo {
		t.Errorf("expected severity info, got %s", alert.Severity)
	}
	if alert.Category != CategorySecurity {
		t.Errorf("expected category security, got %s", alert.Category)
	}
}

func TestManager_GetRules(t *testing.T) {
	m := setupTestManager()
	rules := m.GetRules()
	if len(rules) < 6 {
		t.Errorf("expected at least 6 builtin rules, got %d", len(rules))
	}
}

// Handler Tests

func TestHandler_List(t *testing.T) {
	m := setupTestManager()
	m.Fire("smart_warning", "test1")
	m.Fire("pool_degraded", "test2")

	r := setupTestRouter(m)
	req := httptest.NewRequest("GET", "/api/v1/alerts/guided", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var alerts []*GuidedAlert
	if err := json.Unmarshal(w.Body.Bytes(), &alerts); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(alerts) != 2 {
		t.Errorf("expected 2 alerts, got %d", len(alerts))
	}
}

func TestHandler_Get(t *testing.T) {
	m := setupTestManager()
	alert := m.Fire("smart_warning", "test")

	r := setupTestRouter(m)
	req := httptest.NewRequest("GET", "/api/v1/alerts/guided/"+alert.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result GuidedAlert
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if result.ID != alert.ID {
		t.Errorf("expected id %s, got %s", alert.ID, result.ID)
	}
}

func TestHandler_GetNotFound(t *testing.T) {
	m := setupTestManager()
	r := setupTestRouter(m)

	req := httptest.NewRequest("GET", "/api/v1/alerts/guided/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandler_Acknowledge(t *testing.T) {
	m := setupTestManager()
	alert := m.Fire("smart_warning", "test")

	r := setupTestRouter(m)
	req := httptest.NewRequest("POST", "/api/v1/alerts/guided/"+alert.ID+"/acknowledge", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	updated, _ := m.Get(alert.ID)
	if !updated.Acknowledged {
		t.Error("expected alert to be acknowledged")
	}
}

func TestHandler_AcknowledgeNotFound(t *testing.T) {
	m := setupTestManager()
	r := setupTestRouter(m)

	req := httptest.NewRequest("POST", "/api/v1/alerts/guided/nonexistent/acknowledge", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandler_Silence(t *testing.T) {
	m := setupTestManager()
	alert := m.Fire("smart_warning", "test")

	r := setupTestRouter(m)
	req := httptest.NewRequest("POST", "/api/v1/alerts/guided/"+alert.ID+"/silence", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	updated, _ := m.Get(alert.ID)
	if !updated.Silenced {
		t.Error("expected alert to be silenced")
	}
}

func TestHandler_SilenceNotFound(t *testing.T) {
	m := setupTestManager()
	r := setupTestRouter(m)

	req := httptest.NewRequest("POST", "/api/v1/alerts/guided/nonexistent/silence", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandler_Summary(t *testing.T) {
	m := setupTestManager()
	m.Fire("smart_warning", "test1")
	m.Fire("pool_degraded", "test2")

	r := setupTestRouter(m)
	req := httptest.NewRequest("GET", "/api/v1/alerts/guided/summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var summary AlertSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if summary.Total != 2 {
		t.Errorf("expected total 2, got %d", summary.Total)
	}
}

func TestHandler_CreateRule(t *testing.T) {
	m := setupTestManager()
	r := setupTestRouter(m)

	body := CreateRuleRequest{
		Name:      "test_rule",
		Condition: "test condition",
		Severity:  SeverityInfo,
		Category:  CategorySecurity,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/alerts/guided/rules", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var rule AlertRule
	if err := json.Unmarshal(w.Body.Bytes(), &rule); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if rule.Name != "test_rule" {
		t.Errorf("expected name test_rule, got %s", rule.Name)
	}
}

func TestHandler_CreateRuleInvalid(t *testing.T) {
	m := setupTestManager()
	r := setupTestRouter(m)

	body := map[string]string{"name": "test"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/alerts/guided/rules", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// Integration Test

func TestIntegration_FullWorkflow(t *testing.T) {
	m := setupTestManager()
	r := setupTestRouter(m)

	// Fire an alert
	body := map[string]string{
		"message": "disk sda detected bad sectors",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/alerts/guided/fire", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// This endpoint doesn't exist in our routes, but let's test the manager directly
	alert := m.Fire("smart_warning", "disk sda detected bad sectors")

	// Get the alert
	req = httptest.NewRequest("GET", "/api/v1/alerts/guided/"+alert.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET failed: %d", w.Code)
	}

	// Acknowledge
	req = httptest.NewRequest("POST", "/api/v1/alerts/guided/"+alert.ID+"/acknowledge", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("acknowledge failed: %d", w.Code)
	}

	// Silence
	req = httptest.NewRequest("POST", "/api/v1/alerts/guided/"+alert.ID+"/silence", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("silence failed: %d", w.Code)
	}

	// Check final state
	updated, _ := m.Get(alert.ID)
	if !updated.Acknowledged {
		t.Error("expected acknowledged")
	}
	if !updated.Silenced {
		t.Error("expected silenced")
	}

	// Get summary
	req = httptest.NewRequest("GET", "/api/v1/alerts/guided/summary", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("summary failed: %d", w.Code)
	}

	var summary AlertSummary
	json.Unmarshal(w.Body.Bytes(), &summary)
	if summary.Total != 1 {
		t.Errorf("expected 1 alert, got %d", summary.Total)
	}
}
