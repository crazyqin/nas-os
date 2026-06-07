package smartalerttriage

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestManager() *Manager {
	return NewManager(zap.NewNop())
}

func newTestHandler() (*Handler, *Manager) {
	manager := newTestManager()
	handler := NewHandler(manager, zap.NewNop())
	return handler, manager
}

func TestNewManager(t *testing.T) {
	manager := newTestManager()
	if manager == nil {
		t.Fatal("NewManager() returned nil")
	}
	if manager.alerts == nil {
		t.Error("alerts map not initialized")
	}
	if manager.groups == nil {
		t.Error("groups map not initialized")
	}
	if manager.rootCauses == nil {
		t.Error("rootCauses map not initialized")
	}
	if manager.suppressions == nil {
		t.Error("suppressions map not initialized")
	}
	if len(manager.policies) == 0 {
		t.Error("default policies not loaded")
	}
}

func TestIngestAlert(t *testing.T) {
	manager := newTestManager()

	alert := manager.Ingest(
		"磁盘SMART检测异常",
		"磁盘 /dev/sda SMART指标异常",
		"disk-monitor",
		"/dev/sda",
		map[string]string{"pool": "tank"},
	)

	if alert == nil {
		t.Fatal("Ingest() returned nil")
	}
	if alert.ID == "" {
		t.Error("alert ID is empty")
	}
	if alert.Title != "磁盘SMART检测异常" {
		t.Errorf("Title = %s, expected 磁盘SMART检测异常", alert.Title)
	}
	if alert.Category != CategoryStorage {
		t.Errorf("Category = %s, expected storage", alert.Category)
	}
	if alert.Priority != PriorityCritical {
		t.Errorf("Priority = %s, expected critical", alert.Priority)
	}
	if alert.Count != 1 {
		t.Errorf("Count = %d, expected 1", alert.Count)
	}
}

func TestIngestAlertDeduplication(t *testing.T) {
	manager := newTestManager()

	// 第一次接收
	alert1 := manager.Ingest("磁盘故障", "SMART异常", "disk-monitor", "/dev/sda", nil)
	if alert1.Count != 1 {
		t.Errorf("First ingest count = %d, expected 1", alert1.Count)
	}

	// 第二次接收相同告警（应去重）
	alert2 := manager.Ingest("磁盘故障", "SMART异常更新", "disk-monitor", "/dev/sda", nil)
	if alert2.ID != alert1.ID {
		t.Error("Deduplication failed: different IDs for same fingerprint")
	}
	if alert2.Count != 2 {
		t.Errorf("Second ingest count = %d, expected 2", alert2.Count)
	}
	if alert2.Description != "SMART异常更新" {
		t.Error("Description not updated on dedup")
	}
}

func TestClassifyAlert(t *testing.T) {
	manager := newTestManager()

	tests := []struct {
		name        string
		title       string
		desc        string
		expectedCat Category
		expectedPri Priority
	}{
		{
			name:        "存储告警",
			title:       "磁盘SMART检测异常",
			desc:        "磁盘 /dev/sda SMART指标异常",
			expectedCat: CategoryStorage,
			expectedPri: PriorityCritical,
		},
		{
			name:        "网络告警",
			title:       "网络连接失败",
			desc:        "eth0 网卡 timeout",
			expectedCat: CategoryNetwork,
			expectedPri: PriorityHigh,
		},
		{
			name:        "系统告警",
			title:       "CPU负载过高",
			desc:        "系统负载超过阈值",
			expectedCat: CategorySystem,
			expectedPri: PriorityHigh,
		},
		{
			name:        "安全告警",
			title:       "暴力破解检测",
			desc:        "检测到异常登录失败",
			expectedCat: CategorySecurity,
			expectedPri: PriorityCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alert := manager.Ingest(tt.title, tt.desc, "test", "test-resource", nil)
			if alert.Category != tt.expectedCat {
				t.Errorf("Category = %s, expected %s", alert.Category, tt.expectedCat)
			}
			if alert.Priority != tt.expectedPri {
				t.Errorf("Priority = %s, expected %s", alert.Priority, tt.expectedPri)
			}
		})
	}
}

func TestListAlerts(t *testing.T) {
	manager := newTestManager()

	// 创建多个告警
	manager.Ingest("磁盘故障", "SMART异常", "disk-monitor", "/dev/sda", nil)
	manager.Ingest("网络超时", "连接失败", "net-monitor", "eth0", nil)
	manager.Ingest("CPU负载高", "负载超过80%", "sys-monitor", "host", nil)

	// 列出所有
	all := manager.List(ListQuery{})
	if len(all) != 3 {
		t.Errorf("List() returned %d alerts, expected 3", len(all))
	}

	// 按分类筛选
	storage := manager.List(ListQuery{Category: CategoryStorage})
	if len(storage) != 1 {
		t.Errorf("List(storage) returned %d alerts, expected 1", len(storage))
	}

	// 按优先级筛选
	critical := manager.List(ListQuery{Priority: PriorityCritical})
	if len(critical) != 1 {
		t.Errorf("List(critical) returned %d alerts, expected 1", len(critical))
	}
}

func TestAcknowledgeAlert(t *testing.T) {
	manager := newTestManager()

	alert := manager.Ingest("测试告警", "描述", "test", "resource", nil)

	err := manager.Acknowledge(alert.ID, "admin")
	if err != nil {
		t.Fatalf("Acknowledge() error: %v", err)
	}

	updated, _ := manager.Get(alert.ID)
	if updated.State != StateAcknowledged {
		t.Errorf("State = %s, expected acknowledged", updated.State)
	}
	if updated.AcknowledgedBy != "admin" {
		t.Errorf("AcknowledgedBy = %s, expected admin", updated.AcknowledgedBy)
	}
	if updated.AcknowledgedAt == nil {
		t.Error("AcknowledgedAt is nil")
	}
}

func TestAcknowledgeAlertNotFound(t *testing.T) {
	manager := newTestManager()

	err := manager.Acknowledge("nonexistent-id", "admin")
	if err == nil {
		t.Error("Expected error for nonexistent alert")
	}
}

func TestResolveAlert(t *testing.T) {
	manager := newTestManager()

	alert := manager.Ingest("测试告警", "描述", "test", "resource", nil)

	err := manager.Resolve(alert.ID)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}

	updated, _ := manager.Get(alert.ID)
	if updated.State != StateResolved {
		t.Errorf("State = %s, expected resolved", updated.State)
	}
	if updated.Priority != PriorityInfo {
		t.Errorf("Priority = %s, expected info for resolved alert", updated.Priority)
	}
	if updated.ResolvedAt == nil {
		t.Error("ResolvedAt is nil")
	}
}

func TestSuppressionRules(t *testing.T) {
	manager := newTestManager()

	// 创建抑制规则
	rule := manager.AddSuppression(SuppressRequest{
		Name:        "维护窗口",
		Description: "系统维护期间抑制所有存储告警",
		Category:    CategoryStorage,
		DurationMin: 60,
		Reason:      "计划维护",
		CreatedBy:   "admin",
	})

	if rule == nil {
		t.Fatal("AddSuppression() returned nil")
	}
	if rule.Name != "维护窗口" {
		t.Errorf("Name = %s, expected 维护窗口", rule.Name)
	}
	if !rule.Enabled {
		t.Error("Rule should be enabled")
	}

	// 列出抑制规则
	rules := manager.ListSuppressions()
	if len(rules) != 1 {
		t.Errorf("ListSuppressions() returned %d rules, expected 1", len(rules))
	}

	// 验证抑制效果
	alert := manager.Ingest("磁盘空间不足", "存储空间使用率过高", "disk-monitor", "/dev/sda", nil)
	if alert.State != StateSuppressed {
		t.Errorf("Alert should be suppressed, got state: %s", alert.State)
	}

	// 删除抑制规则
	err := manager.RemoveSuppression(rule.ID)
	if err != nil {
		t.Fatalf("RemoveSuppression() error: %v", err)
	}

	rules = manager.ListSuppressions()
	if len(rules) != 0 {
		t.Errorf("ListSuppressions() after delete returned %d rules, expected 0", len(rules))
	}
}

func TestRemoveSuppressionNotFound(t *testing.T) {
	manager := newTestManager()

	err := manager.RemoveSuppression("nonexistent-id")
	if err == nil {
		t.Error("Expected error for nonexistent suppression rule")
	}
}

func TestGetStats(t *testing.T) {
	manager := newTestManager()

	// 创建多个告警
	manager.Ingest("告警1", "描述1", "source1", "resource1", nil)
	manager.Ingest("告警2", "描述2", "source1", "resource2", nil)
	manager.Ingest("告警3", "描述3", "source2", "resource1", nil)

	stats := manager.GetStats(24)
	if stats == nil {
		t.Fatal("GetStats() returned nil")
	}
	if stats.TotalAlerts != 3 {
		t.Errorf("TotalAlerts = %d, expected 3", stats.TotalAlerts)
	}
	if stats.ActiveAlerts != 3 {
		t.Errorf("ActiveAlerts = %d, expected 3", stats.ActiveAlerts)
	}
}

func TestGetTrend(t *testing.T) {
	manager := newTestManager()

	manager.Ingest("告警1", "描述1", "source1", "resource1", nil)

	trend := manager.GetTrend(24, 24)
	if trend == nil {
		t.Fatal("GetTrend() returned nil")
	}
	if len(trend) != 24 {
		t.Errorf("Trend points = %d, expected 24", len(trend))
	}
}

func TestListGroups(t *testing.T) {
	manager := newTestManager()

	// 创建同源告警（会聚合）
	manager.Ingest("告警1", "描述1", "source1", "resource1", nil)
	manager.Ingest("告警2", "描述2", "source1", "resource2", nil)

	groups := manager.ListGroups()
	if len(groups) == 0 {
		t.Error("Expected at least 1 group")
	}

	// 验证聚合组
	found := false
	for _, g := range groups {
		if g.Source == "source1" && g.Count >= 2 {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected a group with count >= 2 for source1")
	}
}

func TestRootCauseCorrelation(t *testing.T) {
	manager := newTestManager()

	// 创建相同资源的告警（会触发关联）
	manager.Ingest("磁盘SMART异常", "SMART检测到问题", "disk-monitor", "/dev/sda", nil)
	manager.Ingest("磁盘IO异常", "IO延迟过高", "io-monitor", "/dev/sda", nil)

	// 检查是否有根因
	rootCauses := manager.rootCauses
	if len(rootCauses) == 0 {
		t.Error("Expected at least 1 root cause for correlated alerts")
	}

	// 获取第二个告警，检查关联
	alerts := manager.List(ListQuery{})
	var correlatedAlert *Alert
	for _, a := range alerts {
		if a.Title == "磁盘IO异常" {
			correlatedAlert = a
			break
		}
	}

	if correlatedAlert == nil {
		t.Fatal("Could not find correlated alert")
	}
	if correlatedAlert.RootCauseID == "" {
		t.Error("Expected RootCauseID to be set for correlated alert")
	}
	if len(correlatedAlert.RelatedIDs) == 0 {
		t.Error("Expected RelatedIDs to be non-empty")
	}
}

func TestGetRootCause(t *testing.T) {
	manager := newTestManager()

	// 创建关联告警
	manager.Ingest("告警1", "描述1", "source1", "resource1", nil)
	manager.Ingest("告警2", "描述2", "source1", "resource1", nil)

	rootCauses := manager.rootCauses
	if len(rootCauses) == 0 {
		t.Skip("No root causes created")
	}

	// 获取第一个根因
	for id := range rootCauses {
		rc, err := manager.GetRootCause(id)
		if err != nil {
			t.Fatalf("GetRootCause() error: %v", err)
		}
		if rc.ID != id {
			t.Errorf("RootCause ID = %s, expected %s", rc.ID, id)
		}
		break
	}
}

func TestGetRootCauseNotFound(t *testing.T) {
	manager := newTestManager()

	_, err := manager.GetRootCause("nonexistent-id")
	if err == nil {
		t.Error("Expected error for nonexistent root cause")
	}
}

// ========== Handler 测试 ==========

func TestHandlerIngestAlert(t *testing.T) {
	handler, _ := newTestHandler()

	body := ClassifyRequest{
		Title:       "磁盘SMART异常",
		Description: "磁盘 /dev/sda SMART检测到问题",
		Source:      "disk-monitor",
		Resource:    "/dev/sda",
		Labels:      map[string]string{"pool": "tank"},
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/ingest", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ingestAlert(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["code"].(float64) != 0 {
		t.Errorf("Response code = %v, expected 0", response["code"])
	}
}

func TestHandlerIngestAlertInvalidJSON(t *testing.T) {
	handler, _ := newTestHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/ingest", bytes.NewReader([]byte("invalid")))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ingestAlert(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandlerListAlerts(t *testing.T) {
	handler, manager := newTestHandler()

	// 先创建一些告警
	manager.Ingest("告警1", "描述1", "source1", "resource1", nil)
	manager.Ingest("告警2", "描述2", "source2", "resource2", nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/list", nil)

	handler.listAlerts(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data := response["data"].(map[string]interface{})
	total := data["total"].(float64)
	if total != 2 {
		t.Errorf("Total = %v, expected 2", total)
	}
}

func TestHandlerGetAlert(t *testing.T) {
	handler, manager := newTestHandler()

	alert := manager.Ingest("测试告警", "描述", "source", "resource", nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/"+alert.ID, nil)
	c.Params = gin.Params{{Key: "id", Value: alert.ID}}

	handler.getAlert(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data := response["data"].(map[string]interface{})
	if data["title"] != "测试告警" {
		t.Errorf("Title = %v, expected 测试告警", data["title"])
	}
}

func TestHandlerGetAlertNotFound(t *testing.T) {
	handler, _ := newTestHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/nonexistent", nil)
	c.Params = gin.Params{{Key: "id", Value: "nonexistent"}}

	handler.getAlert(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandlerAcknowledge(t *testing.T) {
	handler, manager := newTestHandler()

	alert := manager.Ingest("测试告警", "描述", "source", "resource", nil)

	body := AcknowledgeRequest{Operator: "admin"}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/"+alert.ID+"/acknowledge", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: alert.ID}}

	handler.acknowledge(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlerAcknowledgeNotFound(t *testing.T) {
	handler, _ := newTestHandler()

	body := AcknowledgeRequest{Operator: "admin"}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/nonexistent/acknowledge", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "nonexistent"}}

	handler.acknowledge(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandlerResolve(t *testing.T) {
	handler, manager := newTestHandler()

	alert := manager.Ingest("测试告警", "描述", "source", "resource", nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/"+alert.ID+"/resolve", nil)
	c.Params = gin.Params{{Key: "id", Value: alert.ID}}

	handler.resolveAlert(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlerGetStats(t *testing.T) {
	handler, manager := newTestHandler()

	manager.Ingest("告警1", "描述1", "source1", "resource1", nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/stats?hours=24", nil)

	handler.getStats(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data := response["data"].(map[string]interface{})
	total := data["total_alerts"].(float64)
	if total != 1 {
		t.Errorf("TotalAlerts = %v, expected 1", total)
	}
}

func TestHandlerGetTrend(t *testing.T) {
	handler, manager := newTestHandler()

	manager.Ingest("告警1", "描述1", "source1", "resource1", nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/trend?hours=24&points=12", nil)

	handler.getTrend(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data := response["data"].(map[string]interface{})
	points := data["points"].([]interface{})
	if len(points) != 12 {
		t.Errorf("Trend points = %d, expected 12", len(points))
	}
}

func TestHandlerListGroups(t *testing.T) {
	handler, manager := newTestHandler()

	manager.Ingest("告警1", "描述1", "source1", "resource1", nil)
	manager.Ingest("告警2", "描述2", "source1", "resource2", nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/groups", nil)

	handler.listGroups(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlerCreateSuppression(t *testing.T) {
	handler, _ := newTestHandler()

	body := SuppressRequest{
		Name:        "维护窗口",
		Description: "系统维护",
		Category:    CategoryStorage,
		DurationMin: 60,
		Reason:      "计划维护",
		CreatedBy:   "admin",
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/suppression", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.createSuppression(c)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}
}

func TestHandlerCreateSuppressionInvalidJSON(t *testing.T) {
	handler, _ := newTestHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/suppression", bytes.NewReader([]byte("invalid")))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.createSuppression(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandlerListSuppressions(t *testing.T) {
	handler, manager := newTestHandler()

	manager.AddSuppression(SuppressRequest{
		Name:        "维护窗口",
		DurationMin: 60,
		Reason:      "计划维护",
		CreatedBy:   "admin",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/suppression", nil)

	handler.listSuppressions(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data := response["data"].(map[string]interface{})
	total := data["total"].(float64)
	if total != 1 {
		t.Errorf("Total = %v, expected 1", total)
	}
}

func TestHandlerRemoveSuppression(t *testing.T) {
	handler, manager := newTestHandler()

	rule := manager.AddSuppression(SuppressRequest{
		Name:        "维护窗口",
		DurationMin: 60,
		Reason:      "计划维护",
		CreatedBy:   "admin",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/suppression/"+rule.ID, nil)
	c.Params = gin.Params{{Key: "id", Value: rule.ID}}

	handler.removeSuppression(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlerRemoveSuppressionNotFound(t *testing.T) {
	handler, _ := newTestHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/suppression/nonexistent", nil)
	c.Params = gin.Params{{Key: "id", Value: "nonexistent"}}

	handler.removeSuppression(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestRegisterRoutes(t *testing.T) {
	handler, _ := newTestHandler()

	router := gin.New()
	group := router.Group("/api/v1")
	handler.RegisterRoutes(group)

	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, route := range routes {
		routeMap[route.Path] = true
	}

	expectedRoutes := []string{
		"/api/v1/smartalerttriage/ingest",
		"/api/v1/smartalerttriage/list",
		"/api/v1/smartalerttriage/stats",
		"/api/v1/smartalerttriage/trend",
		"/api/v1/smartalerttriage/groups",
		"/api/v1/smartalerttriage/suppression",
	}

	for _, expected := range expectedRoutes {
		if !routeMap[expected] {
			t.Errorf("Expected route %s not found", expected)
		}
	}
}

func TestNotificationRegistration(t *testing.T) {
	manager := newTestManager()

	// 创建一个简单的测试通知器
	notifier := &testNotifier{channel: ChannelEmail}
	manager.RegisterNotifier(notifier)

	if _, ok := manager.notifiers[ChannelEmail]; !ok {
		t.Error("Email notifier not registered")
	}
}

// testNotifier 测试用通知器.
type testNotifier struct {
	channel NotificationChannel
	sent    bool
}

func (n *testNotifier) Send(alert *Alert, message string) error {
	n.sent = true
	return nil
}

func (n *testNotifier) Channel() NotificationChannel {
	return n.channel
}

func TestEscalation(t *testing.T) {
	manager := newTestManager()

	// 创建告警
	alert := manager.Ingest("测试告警", "描述", "source", "resource", nil)

	// 模拟时间流逝（手动修改LastSeen）
	manager.mu.Lock()
	alert.LastSeen = alert.LastSeen.Add(-35 * time.Minute) // 超过30分钟阈值（medium策略）
	// 设置为活跃状态，以便升级检查
	alert.State = StateClassified
	manager.mu.Unlock()

	// 运行升级检查
	escalated := manager.RunEscalation()

	if escalated != 1 {
		t.Errorf("Escalated = %d, expected 1", escalated)
	}

	// 验证优先级已升级
	updated, _ := manager.Get(alert.ID)
	if updated.State != StateEscalated {
		t.Errorf("State = %s, expected escalated", updated.State)
	}
}

func TestGetAlert(t *testing.T) {
	manager := newTestManager()

	alert := manager.Ingest("测试告警", "描述", "source", "resource", nil)

	got, err := manager.Get(alert.ID)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.ID != alert.ID {
		t.Errorf("Get() ID = %s, expected %s", got.ID, alert.ID)
	}
}

func TestGetAlertNotFound(t *testing.T) {
	manager := newTestManager()

	_, err := manager.Get("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent alert")
	}
}
