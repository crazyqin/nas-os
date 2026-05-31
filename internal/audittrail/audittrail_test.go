package audittrail

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestManager() *Manager {
	return NewManager()
}

func setupTestRouter(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api")
	handler.RegisterRoutes(group)
	return router
}

func TestNewManager(t *testing.T) {
	m := setupTestManager()
	assert.NotNil(t, m)
	assert.NotNil(t, m.events)
	assert.NotNil(t, m.reports)
	assert.NotNil(t, m.alerts)
	assert.Equal(t, 100000, m.maxEvents)
}

func TestLogEvent(t *testing.T) {
	m := setupTestManager()

	event := AuditEvent{
		UserID:    "user1",
		UserName:  "测试用户",
		Action:    "login",
		Resource:  "system",
		IP:        "192.168.1.1",
		RiskLevel: RiskLow,
	}

	m.LogEvent(event)

	events := m.QueryEvents(EventFilter{})
	assert.Len(t, events, 1)
	assert.Equal(t, "user1", events[0].UserID)
	assert.Equal(t, "login", events[0].Action)
	assert.NotEmpty(t, events[0].ID)
	assert.False(t, events[0].Timestamp.IsZero())
}

func TestGetEvent(t *testing.T) {
	m := setupTestManager()

	event := AuditEvent{
		ID:        "test_event_1",
		UserID:    "user1",
		Action:    "login",
		RiskLevel: RiskLow,
	}
	m.LogEvent(event)

	found, err := m.GetEvent("test_event_1")
	require.NoError(t, err)
	assert.Equal(t, "user1", found.UserID)

	_, err = m.GetEvent("nonexistent")
	assert.Error(t, err)
}

func TestQueryEvents(t *testing.T) {
	m := setupTestManager()

	m.LogEvent(AuditEvent{UserID: "user1", Action: "login", RiskLevel: RiskLow})
	m.LogEvent(AuditEvent{UserID: "user2", Action: "logout", RiskLevel: RiskMedium})
	m.LogEvent(AuditEvent{UserID: "user1", Action: "delete", RiskLevel: RiskHigh})
	m.LogEvent(AuditEvent{UserID: "user3", Action: "login", RiskLevel: RiskLow})

	tests := []struct {
		name   string
		filter EventFilter
		count  int
	}{
		{"all", EventFilter{}, 4},
		{"by_user", EventFilter{UserID: "user1"}, 2},
		{"by_action", EventFilter{Action: "login"}, 2},
		{"by_risk", EventFilter{RiskLevel: RiskHigh}, 1},
		{"by_limit", EventFilter{Limit: 2}, 2},
		{"combined", EventFilter{UserID: "user1", RiskLevel: RiskLow}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := m.QueryEvents(tt.filter)
			assert.Len(t, events, tt.count)
		})
	}
}

func TestQueryEventsByTime(t *testing.T) {
	m := setupTestManager()

	now := time.Now()
	m.LogEvent(AuditEvent{ID: "evt1", Timestamp: now.Add(-2 * time.Hour)})
	m.LogEvent(AuditEvent{ID: "evt2", Timestamp: now.Add(-1 * time.Hour)})
	m.LogEvent(AuditEvent{ID: "evt3", Timestamp: now})

	events := m.QueryEvents(EventFilter{
		StartTime: now.Add(-90 * time.Minute),
	})
	assert.Len(t, events, 2)
}

func TestGenerateReport(t *testing.T) {
	m := setupTestManager()

	m.LogEvent(AuditEvent{UserID: "user1", Action: "login", RiskLevel: RiskLow})
	m.LogEvent(AuditEvent{UserID: "user1", Action: "delete", RiskLevel: RiskHigh})
	m.LogEvent(AuditEvent{UserID: "user2", Action: "login", RiskLevel: RiskMedium})
	m.LogEvent(AuditEvent{UserID: "user2", Action: "logout", RiskLevel: RiskLow})

	report := m.GenerateReport("last_7_days")

	assert.Equal(t, "last_7_days", report.Period)
	assert.Equal(t, 4, report.TotalEvents)
	assert.Len(t, report.TopUsers, 2)
	assert.Len(t, report.TopActions, 3)
	assert.Equal(t, 1, report.RiskSummary[RiskHigh])
	assert.Equal(t, 2, report.RiskSummary[RiskLow])

	reports := m.GetReports()
	assert.Len(t, reports, 1)
}

func TestAlertRules(t *testing.T) {
	m := setupTestManager()

	rule := m.AddAlertRule(AlertRule{
		Name:      "高风险告警",
		Condition: "high_risk_count > 10",
		Action:    "notify",
		Enabled:   true,
	})

	assert.NotEmpty(t, rule.ID)
	assert.Equal(t, "高风险告警", rule.Name)

	rules := m.GetAlertRules()
	assert.Len(t, rules, 1)

	err := m.UpdateAlertRule(rule.ID, AlertRule{
		Name:    "更新后的规则",
		Enabled: false,
	})
	assert.NoError(t, err)

	rules = m.GetAlertRules()
	assert.Equal(t, "更新后的规则", rules[0].Name)
	assert.False(t, rules[0].Enabled)

	err = m.DeleteAlertRule(rule.ID)
	assert.NoError(t, err)

	rules = m.GetAlertRules()
	assert.Len(t, rules, 0)
}

func TestAlertRulesErrors(t *testing.T) {
	m := setupTestManager()

	err := m.UpdateAlertRule("nonexistent", AlertRule{})
	assert.Error(t, err)

	err = m.DeleteAlertRule("nonexistent")
	assert.Error(t, err)
}

func TestCheckAlerts(t *testing.T) {
	m := setupTestManager()

	m.AddAlertRule(AlertRule{
		Name:      "高风险告警",
		Condition: "high_risk_count > 10",
		Action:    "notify",
		Enabled:   true,
	})

	// 不触发告警
	triggered := m.CheckAlerts()
	assert.Len(t, triggered, 0)

	// 添加超过10个高风险事件
	for i := 0; i < 11; i++ {
		m.LogEvent(AuditEvent{RiskLevel: RiskHigh})
	}

	triggered = m.CheckAlerts()
	assert.Len(t, triggered, 1)
}

func TestExportEvents(t *testing.T) {
	m := setupTestManager()

	m.LogEvent(AuditEvent{
		ID:        "evt1",
		UserID:    "user1",
		UserName:  "测试",
		Action:    "login",
		Resource:  "system",
		RiskLevel: RiskLow,
	})

	tests := []struct {
		name      string
		format    ExportFormat
		expectErr bool
	}{
		{"json", FormatJSON, false},
		{"csv", FormatCSV, false},
		{"invalid", "xml", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := m.ExportEvents(tt.format)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, data)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, data)
			}
		})
	}
}

// HTTP Handler Tests

func TestHTTPLogEvent(t *testing.T) {
	m := setupTestManager()
	handler := NewHandler(m)
	router := setupTestRouter(handler)

	event := AuditEvent{
		UserID:    "user1",
		Action:    "login",
		RiskLevel: RiskLow,
	}
	body, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/api/audit-trail/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestHTTPGetEvent(t *testing.T) {
	m := setupTestManager()
	m.LogEvent(AuditEvent{ID: "test123", UserID: "user1", Action: "login"})

	handler := NewHandler(m)
	router := setupTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/audit-trail/events/test123", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTPQueryEvents(t *testing.T) {
	m := setupTestManager()
	m.LogEvent(AuditEvent{UserID: "user1", Action: "login", RiskLevel: RiskLow})
	m.LogEvent(AuditEvent{UserID: "user2", Action: "logout", RiskLevel: RiskHigh})

	handler := NewHandler(m)
	router := setupTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/audit-trail/events?user_id=user1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTPGetReports(t *testing.T) {
	m := setupTestManager()
	m.GenerateReport("test")

	handler := NewHandler(m)
	router := setupTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/audit-trail/reports", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTPGenerateReport(t *testing.T) {
	m := setupTestManager()
	handler := NewHandler(m)
	router := setupTestRouter(handler)

	body := bytes.NewBufferString(`{"period":"last_7_days"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/audit-trail/reports/generate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestHTTPAlertRules(t *testing.T) {
	m := setupTestManager()
	handler := NewHandler(m)
	router := setupTestRouter(handler)

	// 创建规则
	body := bytes.NewBufferString(`{"name":"test","condition":"test","action":"notify","enabled":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/audit-trail/alert-rules", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// 获取规则列表
	req = httptest.NewRequest(http.MethodGet, "/api/audit-trail/alert-rules", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTPExportEvents(t *testing.T) {
	m := setupTestManager()
	m.LogEvent(AuditEvent{ID: "evt1", UserID: "user1", Action: "test"})

	handler := NewHandler(m)
	router := setupTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/audit-trail/export?format=json", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
