package alertguided

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestManager() *Manager {
	return NewManager(zap.NewNop())
}

func setupTestHandlers(m *Manager) *Handlers {
	return NewHandlers(zap.NewNop(), m)
}

func setupTestRouter(m *Manager) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")
	h := setupTestHandlers(m)
	h.RegisterRoutes(v1)
	return r
}

// ========== Manager Tests ==========

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
		t.Errorf("expected severity WARNING, got %s", alert.Severity)
	}
	if alert.Category != CategoryHardware {
		t.Errorf("expected category hardware, got %s", alert.Category)
	}
	if alert.Status != StatusOpen {
		t.Errorf("expected status OPEN, got %s", alert.Status)
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
	if len(alert.AutoFixActions) == 0 {
		t.Error("expected auto fix actions")
	}
	if alert.Count != 1 {
		t.Errorf("expected count 1, got %d", alert.Count)
	}
}

func TestManager_FireUnknownRule(t *testing.T) {
	m := setupTestManager()

	alert := m.Fire("unknown_rule", "test message")
	if alert == nil {
		t.Fatal("Fire returned nil")
	}
	if alert.Severity != SeverityWarning {
		t.Errorf("expected default severity WARNING, got %s", alert.Severity)
	}
	if alert.Status != StatusOpen {
		t.Errorf("expected status OPEN, got %s", alert.Status)
	}
}

func TestManager_Aggregation(t *testing.T) {
	m := setupTestManager()

	alert1 := m.Fire("disk_space_low", "sda space 91%")
	alert2 := m.Fire("disk_space_low", "sdb space 92%")
	alert3 := m.Fire("disk_space_low", "sdc space 93%")

	// 同一个聚合规则应该返回同一个告警对象
	if alert1.ID != alert2.ID {
		t.Error("expected aggregated alerts to have same ID")
	}
	if alert1.ID != alert3.ID {
		t.Error("expected aggregated alerts to have same ID")
	}
	if alert1.Count != 3 {
		t.Errorf("expected count 3, got %d", alert1.Count)
	}
	// 消息应该是最新的
	if alert1.Message != "sdc space 93%" {
		t.Errorf("expected latest message, got %s", alert1.Message)
	}
}

func TestManager_AggregationResolved(t *testing.T) {
	m := setupTestManager()

	alert1 := m.Fire("disk_space_low", "sda space 91%")
	m.UpdateStatus(alert1.ID, StatusResolved, "space freed", "admin")

	alert2 := m.Fire("disk_space_low", "sdb space 92%")

	// 解决后应该创建新告警
	if alert1.ID == alert2.ID {
		t.Error("expected new alert after resolved")
	}
	if alert2.Count != 1 {
		t.Errorf("expected count 1, got %d", alert2.Count)
	}
}

func TestManager_CorrelateAlerts(t *testing.T) {
	m := setupTestManager()

	alert1 := m.Fire("smart_warning", "disk sda issue") // hardware
	alert2 := m.Fire("pool_degraded", "pool degraded")  // storage
	alert3 := m.Fire("disk_space_low", "space low")     // storage (aggregated, so first creates new)

	// alert2 和 alert3 应该关联（都是 storage 类别，且 alert3 会创建新告警）
	// 但 alert3 有 aggregation key，所以可能聚合，这里验证关联逻辑
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

	// alert1 是 hardware，不应该和 storage 关联
	for _, id := range alert1.RelatedAlertIDs {
		if id == alert2.ID || id == alert3.ID {
			t.Error("hardware alert should not be related to storage alerts")
		}
	}
}

func TestManager_Acknowledge(t *testing.T) {
	m := setupTestManager()
	alert := m.Fire("smart_warning", "test")

	err := m.Acknowledge(alert.ID, "test reason")
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
	err := m.Acknowledge("nonexistent", "")
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

func TestManager_UpdateStatus(t *testing.T) {
	m := setupTestManager()
	alert := m.Fire("smart_warning", "test")

	// 更新为 IN_PROGRESS
	err := m.UpdateStatus(alert.ID, StatusInProgress, "investigating", "admin")
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	updated, ok := m.Get(alert.ID)
	if !ok {
		t.Fatal("alert not found")
	}
	if updated.Status != StatusInProgress {
		t.Errorf("expected status IN_PROGRESS, got %s", updated.Status)
	}
	if len(updated.StatusHistory) != 2 {
		t.Errorf("expected 2 status history entries, got %d", len(updated.StatusHistory))
	}

	// 更新为 RESOLVED
	err = m.UpdateStatus(alert.ID, StatusResolved, "fixed", "admin")
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	updated, _ = m.Get(alert.ID)
	if updated.Status != StatusResolved {
		t.Errorf("expected status RESOLVED, got %s", updated.Status)
	}
}

func TestManager_UpdateStatusNotFound(t *testing.T) {
	m := setupTestManager()
	err := m.UpdateStatus("nonexistent", StatusResolved, "", "")
	if err == nil {
		t.Error("expected error for nonexistent alert")
	}
}

func TestManager_ListBySeverity(t *testing.T) {
	m := setupTestManager()

	m.Fire("smart_warning", "test1")   // WARNING
	m.Fire("pool_degraded", "test2")   // CRITICAL
	m.Fire("security_breach", "test3") // EMERGENCY
	m.Fire("high_cpu", "test4")        // WARNING

	warnings := m.ListBySeverity(SeverityWarning)
	if len(warnings) != 2 {
		t.Errorf("expected 2 warnings, got %d", len(warnings))
	}

	criticals := m.ListBySeverity(SeverityCritical)
	if len(criticals) != 1 {
		t.Errorf("expected 1 critical, got %d", len(criticals))
	}

	emergencies := m.ListBySeverity(SeverityEmergency)
	if len(emergencies) != 1 {
		t.Errorf("expected 1 emergency, got %d", len(emergencies))
	}
}

func TestManager_ListByStatus(t *testing.T) {
	m := setupTestManager()

	alert1 := m.Fire("smart_warning", "test1")
	alert2 := m.Fire("pool_degraded", "test2")
	m.Fire("high_cpu", "test3")

	m.UpdateStatus(alert1.ID, StatusInProgress, "", "")
	m.UpdateStatus(alert2.ID, StatusResolved, "", "")

	openAlerts := m.ListByStatus(StatusOpen)
	if len(openAlerts) != 1 {
		t.Errorf("expected 1 open, got %d", len(openAlerts))
	}

	inProgress := m.ListByStatus(StatusInProgress)
	if len(inProgress) != 1 {
		t.Errorf("expected 1 in_progress, got %d", len(inProgress))
	}

	resolved := m.ListByStatus(StatusResolved)
	if len(resolved) != 1 {
		t.Errorf("expected 1 resolved, got %d", len(resolved))
	}
}

func TestManager_Summary(t *testing.T) {
	m := setupTestManager()

	m.Fire("smart_warning", "test1")
	m.Fire("pool_degraded", "test2")
	m.Fire("disk_space_low", "test3") // aggregated
	m.Fire("network_down", "test4")
	m.Fire("high_cpu", "test5")
	m.Fire("security_breach", "test6")

	summary := m.Summary()
	if summary.Total != 6 {
		t.Errorf("expected total 6, got %d", summary.Total)
	}
	if summary.ByCategory[CategoryHardware] != 1 {
		t.Errorf("expected 1 hardware alert, got %d", summary.ByCategory[CategoryHardware])
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
	if summary.BySeverity[SeverityEmergency] != 1 {
		t.Errorf("expected 1 emergency alert, got %d", summary.BySeverity[SeverityEmergency])
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
		Tags:      []string{"custom"},
	}
	m.RegisterRule(rule)

	alert := m.Fire("custom_rule", "test")
	if alert.Severity != SeverityInfo {
		t.Errorf("expected severity INFO, got %s", alert.Severity)
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

func TestManager_ContextInfo(t *testing.T) {
	m := setupTestManager()

	m.Fire("smart_warning", "test1")
	alert := m.Fire("pool_degraded", "test2")

	if alert.Context == nil {
		t.Fatal("expected context info")
	}
	if alert.Context.ActiveAlerts != 2 {
		t.Errorf("expected 2 active alerts, got %d", alert.Context.ActiveAlerts)
	}
}

func TestManager_StatusHistory(t *testing.T) {
	m := setupTestManager()
	alert := m.Fire("smart_warning", "test")

	// 初始状态应该有一条记录
	if len(alert.StatusHistory) != 1 {
		t.Errorf("expected 1 status history entry, got %d", len(alert.StatusHistory))
	}

	m.UpdateStatus(alert.ID, StatusInProgress, "investigating", "admin")
	m.UpdateStatus(alert.ID, StatusResolved, "fixed", "admin")

	updated, _ := m.Get(alert.ID)
	if len(updated.StatusHistory) != 3 {
		t.Errorf("expected 3 status history entries, got %d", len(updated.StatusHistory))
	}

	// 验证最后一条记录
	last := updated.StatusHistory[2]
	if last.From != StatusInProgress {
		t.Errorf("expected from IN_PROGRESS, got %s", last.From)
	}
	if last.To != StatusResolved {
		t.Errorf("expected to RESOLVED, got %s", last.To)
	}
	if last.ChangedBy != "admin" {
		t.Errorf("expected changed by admin, got %s", last.ChangedBy)
	}
}

// ========== Handler Tests ==========

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

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if int(resp["total"].(float64)) != 2 {
		t.Errorf("expected total 2, got %v", resp["total"])
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

func TestHandler_ListBySeverity(t *testing.T) {
	m := setupTestManager()
	m.Fire("smart_warning", "test1")
	m.Fire("pool_degraded", "test2")
	m.Fire("high_cpu", "test3")

	r := setupTestRouter(m)
	req := httptest.NewRequest("GET", "/api/v1/alerts/guided/severity/WARNING", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if int(resp["total"].(float64)) != 2 {
		t.Errorf("expected 2 warnings, got %v", resp["total"])
	}
}

func TestHandler_ListByStatus(t *testing.T) {
	m := setupTestManager()
	alert := m.Fire("smart_warning", "test1")
	m.Fire("pool_degraded", "test2")
	m.UpdateStatus(alert.ID, StatusResolved, "", "")

	r := setupTestRouter(m)
	req := httptest.NewRequest("GET", "/api/v1/alerts/guided/status/RESOLVED", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if int(resp["total"].(float64)) != 1 {
		t.Errorf("expected 1 resolved, got %v", resp["total"])
	}
}

func TestHandler_Acknowledge(t *testing.T) {
	m := setupTestManager()
	alert := m.Fire("smart_warning", "test")

	r := setupTestRouter(m)

	body := AcknowledgeRequest{Reason: "test reason"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/alerts/guided/"+alert.ID+"/acknowledge", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
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

func TestHandler_UpdateStatus(t *testing.T) {
	m := setupTestManager()
	alert := m.Fire("smart_warning", "test")

	r := setupTestRouter(m)

	body := UpdateStatusRequest{Status: StatusInProgress, Reason: "investigating"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/alerts/guided/"+alert.ID+"/status", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	updated, _ := m.Get(alert.ID)
	if updated.Status != StatusInProgress {
		t.Errorf("expected status IN_PROGRESS, got %s", updated.Status)
	}
}

func TestHandler_UpdateStatusInvalid(t *testing.T) {
	m := setupTestManager()
	alert := m.Fire("smart_warning", "test")

	r := setupTestRouter(m)

	body := map[string]string{"invalid": "data"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/alerts/guided/"+alert.ID+"/status", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
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

func TestHandler_ListRules(t *testing.T) {
	m := setupTestManager()
	r := setupTestRouter(m)

	req := httptest.NewRequest("GET", "/api/v1/alerts/guided/rules", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if int(resp["total"].(float64)) < 6 {
		t.Errorf("expected at least 6 rules, got %v", resp["total"])
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
		Tags:      []string{"test"},
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

// ========== Integration Tests ==========

func TestIntegration_FullWorkflow(t *testing.T) {
	m := setupTestManager()
	r := setupTestRouter(m)

	// 1. 触发告警
	alert := m.Fire("smart_warning", "disk sda detected bad sectors")

	// 2. 获取告警
	req := httptest.NewRequest("GET", "/api/v1/alerts/guided/"+alert.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET failed: %d", w.Code)
	}

	// 3. 更新状态为处理中
	body := UpdateStatusRequest{Status: StatusInProgress, Reason: "investigating"}
	jsonBody, _ := json.Marshal(body)
	req = httptest.NewRequest("POST", "/api/v1/alerts/guided/"+alert.ID+"/status", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("update status failed: %d", w.Code)
	}

	// 4. 确认告警
	req = httptest.NewRequest("POST", "/api/v1/alerts/guided/"+alert.ID+"/acknowledge", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("acknowledge failed: %d", w.Code)
	}

	// 5. 解决告警
	body = UpdateStatusRequest{Status: StatusResolved, Reason: "disk replaced"}
	jsonBody, _ = json.Marshal(body)
	req = httptest.NewRequest("POST", "/api/v1/alerts/guided/"+alert.ID+"/status", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("resolve failed: %d", w.Code)
	}

	// 6. 验证最终状态
	updated, _ := m.Get(alert.ID)
	if !updated.Acknowledged {
		t.Error("expected acknowledged")
	}
	if updated.Status != StatusResolved {
		t.Errorf("expected status RESOLVED, got %s", updated.Status)
	}
	if len(updated.StatusHistory) != 3 {
		t.Errorf("expected 3 status history entries, got %d", len(updated.StatusHistory))
	}

	// 7. 获取汇总
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
	if summary.Acknowledged != 1 {
		t.Errorf("expected 1 acknowledged, got %d", summary.Acknowledged)
	}
}

func TestIntegration_AggregationWorkflow(t *testing.T) {
	m := setupTestManager()

	// 触发多次相同聚合规则的告警
	m.Fire("disk_space_low", "sda space 91%")
	m.Fire("disk_space_low", "sdb space 92%")
	m.Fire("disk_space_low", "sdc space 93%")

	// 应该只有一个告警，计数为3
	alerts := m.List()
	if len(alerts) != 1 {
		t.Errorf("expected 1 aggregated alert, got %d", len(alerts))
	}
	if alerts[0].Count != 3 {
		t.Errorf("expected count 3, got %d", alerts[0].Count)
	}

	// 汇总应该显示聚合数
	summary := m.Summary()
	if summary.Aggregated != 1 {
		t.Errorf("expected 1 aggregated, got %d", summary.Aggregated)
	}
}
