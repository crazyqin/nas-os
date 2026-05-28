package trafficclassifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(zap.NewNop(), nil)
}

func setupTestRouter(t *testing.T, m *Manager) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	h := NewHandlers(m)
	h.RegisterRoutes(rg)
	return r
}

func makeTestFlow(id string, dstPort int, protocol string) TrafficFlow {
	now := time.Now()
	return TrafficFlow{
		ID: id, SrcIP: "192.168.1.100", DstIP: "10.0.0.1",
		SrcPort: 54321, DstPort: dstPort, Protocol: protocol,
		BytesIn: 1024000, BytesOut: 512000, PacketsIn: 1000, PacketsOut: 500,
		StartTime: now.Add(-10 * time.Second), EndTime: now,
	}
}

func TestNewManager(t *testing.T) {
	m := setupTestManager(t)
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if m.IsRunning() {
		t.Error("expected manager not running initially")
	}
}

func TestStartStop(t *testing.T) {
	m := setupTestManager(t)
	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !m.IsRunning() {
		t.Error("expected running after Start")
	}
	if err := m.Start(); err == nil {
		t.Error("expected error on double start")
	}
	m.Stop()
	if m.IsRunning() {
		t.Error("expected not running after Stop")
	}
}

func TestAnalyzeFlows(t *testing.T) {
	m := setupTestManager(t)
	_ = m.Start()
	defer m.Stop()

	flows := []TrafficFlow{
		makeTestFlow("flow-1", 443, "tcp"),
		makeTestFlow("flow-2", 80, "tcp"),
		makeTestFlow("flow-3", 5060, "udp"),
	}
	resp, err := m.AnalyzeFlows(&AnalyzeRequest{Flows: flows})
	if err != nil {
		t.Fatalf("AnalyzeFlows failed: %v", err)
	}
	if resp.Status != AnalysisStatusCompleted {
		t.Errorf("expected status completed, got %v", resp.Status)
	}
	if len(resp.Results) != 3 {
		t.Errorf("expected 3 results, got %d", len(resp.Results))
	}
	if resp.Stats == nil {
		t.Error("expected non-nil stats")
	}
}

func TestAnalyzeFlowsDisabled(t *testing.T) {
	cfg := DefaultClassifierConfig()
	cfg.Enabled = false
	m := NewManager(zap.NewNop(), cfg)
	_, err := m.AnalyzeFlows(&AnalyzeRequest{Flows: []TrafficFlow{makeTestFlow("f1", 80, "tcp")}})
	if err == nil {
		t.Error("expected error when disabled")
	}
}

func TestClassifyByPort(t *testing.T) {
	m := setupTestManager(t)
	tests := []struct {
		port     int
		expected TrafficType
	}{
		{443, TrafficTypeOffice}, {80, TrafficTypeOffice}, {554, TrafficTypeVideo},
		{5060, TrafficTypeAudio}, {27015, TrafficTypeGame}, {6881, TrafficTypeDownload},
		{1883, TrafficTypeIoT}, {99999, TrafficTypeUnknown},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("port-%d", tt.port), func(t *testing.T) {
			if result := m.classifyByPort(tt.port); result != tt.expected {
				t.Errorf("classifyByPort(%d) = %v, want %v", tt.port, result, tt.expected)
			}
		})
	}
}

func TestCustomRules(t *testing.T) {
	m := setupTestManager(t)
	rule := &ClassificationRule{
		Name: "test-rule", TrafficType: TrafficTypeVideo, DstIPPattern: "10.0.0.0/8",
		Ports: []int{8080}, Priority: 10, Enabled: true,
	}
	m.AddRule(rule)
	if rule.ID == "" {
		t.Error("expected non-empty rule ID after add")
	}
	if len(m.ListRules()) != 1 {
		t.Errorf("expected 1 rule, got %d", len(m.ListRules()))
	}
	got, err := m.GetRule(rule.ID)
	if err != nil {
		t.Fatalf("GetRule failed: %v", err)
	}
	if got.Name != "test-rule" {
		t.Errorf("expected name 'test-rule', got %s", got.Name)
	}
	rule.Name = "updated-rule"
	if err := m.UpdateRule(rule); err != nil {
		t.Fatalf("UpdateRule failed: %v", err)
	}
	if err := m.DeleteRule(rule.ID); err != nil {
		t.Fatalf("DeleteRule failed: %v", err)
	}
	if len(m.ListRules()) != 0 {
		t.Error("expected 0 rules after delete")
	}
	if err := m.DeleteRule("nonexistent"); err == nil {
		t.Error("expected error for nonexistent rule")
	}
}

func TestCustomRuleMatch(t *testing.T) {
	m := setupTestManager(t)
	_ = m.Start()
	defer m.Stop()
	m.AddRule(&ClassificationRule{
		Name: "my-video", TrafficType: TrafficTypeVideo, Ports: []int{9090}, Priority: 100, Enabled: true,
	})
	flow := makeTestFlow("flow-custom", 9090, "tcp")
	resp, err := m.AnalyzeFlows(&AnalyzeRequest{Flows: []TrafficFlow{flow}})
	if err != nil {
		t.Fatalf("AnalyzeFlows failed: %v", err)
	}
	if resp.Results[0].TrafficType != TrafficTypeVideo {
		t.Errorf("expected video, got %v", resp.Results[0].TrafficType)
	}
	if resp.Results[0].RuleName != "my-video" {
		t.Errorf("expected rule name 'my-video', got %s", resp.Results[0].RuleName)
	}
}

func TestBandwidthPolicies(t *testing.T) {
	m := setupTestManager(t)
	policies := m.ListBandwidthPolicies()
	if len(policies) < 7 {
		t.Errorf("expected at least 7 default policies, got %d", len(policies))
	}
	p := &BandwidthPolicy{
		Name: "custom", TrafficType: TrafficTypeVideo, MinMbps: 20, MaxMbps: 200, Priority: 2, Enabled: true,
	}
	m.AddBandwidthPolicy(p)
	if p.ID == "" {
		t.Error("expected non-empty ID")
	}
	got, err := m.GetBandwidthPolicy(p.ID)
	if err != nil {
		t.Fatalf("GetBandwidthPolicy failed: %v", err)
	}
	if got.Name != "custom" {
		t.Errorf("expected name 'custom', got %s", got.Name)
	}
	if err := m.DeleteBandwidthPolicy(p.ID); err != nil {
		t.Fatalf("DeleteBandwidthPolicy failed: %v", err)
	}
	if err := m.DeleteBandwidthPolicy("nonexistent"); err == nil {
		t.Error("expected error for nonexistent policy")
	}
}

func TestMirrorConfigs(t *testing.T) {
	m := setupTestManager(t)
	cfg := &MirrorConfig{Name: "mirror-1", SourceIface: "eth0", TargetIface: "eth1", Enabled: true}
	m.AddMirrorConfig(cfg)
	if cfg.ID == "" {
		t.Error("expected non-empty ID")
	}
	if len(m.ListMirrorConfigs()) != 1 {
		t.Errorf("expected 1 mirror config, got %d", len(m.ListMirrorConfigs()))
	}
	if err := m.DeleteMirrorConfig(cfg.ID); err != nil {
		t.Fatalf("DeleteMirrorConfig failed: %v", err)
	}
	if err := m.DeleteMirrorConfig("nonexistent"); err == nil {
		t.Error("expected error for nonexistent config")
	}
}

func TestQoSRules(t *testing.T) {
	m := setupTestManager(t)
	rule := &QoSRule{
		Name: "qos-1", TrafficType: TrafficTypeGame, DSCP: 46, QueueID: 1,
		RateMbps: 10, CeilMbps: 50, Priority: 1, Enabled: true,
	}
	m.AddQoSRule(rule)
	if rule.ID == "" {
		t.Error("expected non-empty ID")
	}
	if len(m.ListQoSRules()) != 1 {
		t.Errorf("expected 1 qos rule, got %d", len(m.ListQoSRules()))
	}
	if err := m.DeleteQoSRule(rule.ID); err != nil {
		t.Fatalf("DeleteQoSRule failed: %v", err)
	}
	if err := m.DeleteQoSRule("nonexistent"); err == nil {
		t.Error("expected error for nonexistent rule")
	}
}

func TestAnomalyDetectionDDoS(t *testing.T) {
	m := setupTestManager(t)
	_ = m.Start()
	defer m.Stop()
	now := time.Now()
	flow := TrafficFlow{
		ID: "ddos-flow", SrcIP: "1.2.3.4", DstIP: "10.0.0.1",
		SrcPort: 12345, DstPort: 80, Protocol: "tcp",
		BytesIn: 100000, BytesOut: 100, PacketsIn: 50000, PacketsOut: 10,
		StartTime: now, EndTime: now.Add(1 * time.Second),
	}
	resp, err := m.AnalyzeFlows(&AnalyzeRequest{Flows: []TrafficFlow{flow}})
	if err != nil {
		t.Fatalf("AnalyzeFlows failed: %v", err)
	}
	if len(resp.Alerts) == 0 {
		t.Error("expected DDoS anomaly alert")
	}
}

func TestAnomalyDetectionDataLeak(t *testing.T) {
	m := setupTestManager(t)
	_ = m.Start()
	defer m.Stop()
	now := time.Now()
	flow := TrafficFlow{
		ID: "leak-flow", SrcIP: "10.0.0.1", DstIP: "1.2.3.4",
		SrcPort: 12345, DstPort: 443, Protocol: "tcp",
		BytesIn: 10000000, BytesOut: 2000000000, PacketsIn: 10000, PacketsOut: 500000,
		StartTime: now.Add(-3600 * time.Second), EndTime: now,
	}
	resp, err := m.AnalyzeFlows(&AnalyzeRequest{Flows: []TrafficFlow{flow}})
	if err != nil {
		t.Fatalf("AnalyzeFlows failed: %v", err)
	}
	if len(resp.Alerts) == 0 {
		t.Error("expected data leak anomaly alert")
	}
}

func TestAlertManagement(t *testing.T) {
	m := setupTestManager(t)
	alert := &AnomalyAlert{
		ID: "alert-1", AnomalyType: AnomalyTypeDDoS, Severity: AlertSeverityCritical,
		SourceIP: "1.2.3.4", Description: "test alert", FirstSeen: time.Now(), LastSeen: time.Now(),
	}
	m.mu.Lock()
	m.anomalies[alert.ID] = alert
	m.mu.Unlock()
	alerts := m.ListAlerts(false)
	if len(alerts) != 1 {
		t.Errorf("expected 1 unresolved alert, got %d", len(alerts))
	}
	if err := m.ResolveAlert("alert-1"); err != nil {
		t.Fatalf("ResolveAlert failed: %v", err)
	}
	if len(m.ListAlerts(false)) != 0 {
		t.Error("expected 0 unresolved alerts")
	}
	if len(m.ListAlerts(true)) != 1 {
		t.Error("expected 1 resolved alert")
	}
	if err := m.ResolveAlert("nonexistent"); err == nil {
		t.Error("expected error for nonexistent alert")
	}
}

func TestReportGeneration(t *testing.T) {
	m := setupTestManager(t)
	_ = m.Start()
	defer m.Stop()
	m.AnalyzeFlows(&AnalyzeRequest{Flows: []TrafficFlow{makeTestFlow("r-flow", 443, "tcp")}})
	now := time.Now()
	report := m.GenerateReport(&ReportRequest{StartTime: now.Add(-1 * time.Hour), EndTime: now, Title: "测试报告"})
	if report.ID == "" {
		t.Error("expected non-empty report ID")
	}
	if report.Title != "测试报告" {
		t.Errorf("expected title '测试报告', got %s", report.Title)
	}
	if report.Summary == "" {
		t.Error("expected non-empty summary")
	}
	got, err := m.GetReport(report.ID)
	if err != nil {
		t.Fatalf("GetReport failed: %v", err)
	}
	if got.ID != report.ID {
		t.Errorf("expected ID %s, got %s", report.ID, got.ID)
	}
	if len(m.ListReports()) != 1 {
		t.Errorf("expected 1 report, got %d", len(m.ListReports()))
	}
	if _, err := m.GetReport("nonexistent"); err == nil {
		t.Error("expected error for nonexistent report")
	}
}

func TestGetConfig(t *testing.T) {
	m := setupTestManager(t)
	cfg := m.GetConfig()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if !cfg.Enabled {
		t.Error("expected enabled by default")
	}
	if cfg.MaxFlows != 100000 {
		t.Errorf("expected MaxFlows 100000, got %d", cfg.MaxFlows)
	}
}

func TestUpdateConfig(t *testing.T) {
	m := setupTestManager(t)
	cfg := m.GetConfig()
	cfg.MaxFlows = 50000
	m.UpdateConfig(cfg)
	if got := m.GetConfig(); got.MaxFlows != 50000 {
		t.Errorf("expected MaxFlows 50000, got %d", got.MaxFlows)
	}
}

func TestDefaultClassifierConfig(t *testing.T) {
	cfg := DefaultClassifierConfig()
	if !cfg.Enabled {
		t.Error("expected enabled")
	}
	if cfg.MaxFlows != 100000 {
		t.Errorf("expected 100000, got %d", cfg.MaxFlows)
	}
	if cfg.AnomalyThreshold != 0.8 {
		t.Errorf("expected 0.8, got %f", cfg.AnomalyThreshold)
	}
}

func TestDPISignaturesInitialized(t *testing.T) {
	m := setupTestManager(t)
	if len(m.dpiSignatures) == 0 {
		t.Error("expected DPI signatures to be initialized")
	}
}

func TestExtractFeatures(t *testing.T) {
	m := setupTestManager(t)
	flow := makeTestFlow("feat-flow", 443, "tcp")
	features := m.extractFeatures(&flow)
	if features.Protocol != "tcp" {
		t.Errorf("expected protocol tcp, got %s", features.Protocol)
	}
	if features.DstPort != 443 {
		t.Errorf("expected dst port 443, got %d", features.DstPort)
	}
	if features.PacketRate <= 0 {
		t.Error("expected positive packet rate")
	}
	if features.ByteRate <= 0 {
		t.Error("expected positive byte rate")
	}
}

func TestHandler_Analyze(t *testing.T) {
	m := setupTestManager(t)
	m.Start()
	defer m.Stop()
	r := setupTestRouter(t, m)
	flow := makeTestFlow("h-flow", 443, "tcp")
	body, _ := json.Marshal(AnalyzeRequest{Flows: []TrafficFlow{flow}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/traffic-classifier/analyze", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_AnalyzeInvalidBody(t *testing.T) {
	m := setupTestManager(t)
	m.Start()
	defer m.Stop()
	r := setupTestRouter(t, m)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/traffic-classifier/analyze", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_Stats(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/traffic-classifier/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_RulesCRUD(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)
	body := `{"name":"test","traffic_type":"video","priority":10,"enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/traffic-classifier/rules", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/traffic-classifier/rules", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp tcResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := json.Marshal(resp.Data)
	var rules []ClassificationRule
	json.Unmarshal(data, &rules)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	ruleID := rules[0].ID
	req = httptest.NewRequest(http.MethodGet, "/api/v1/traffic-classifier/rules/"+ruleID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	updateBody := `{"name":"updated","traffic_type":"audio","priority":5,"enabled":true}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/traffic-classifier/rules/"+ruleID, bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/traffic-classifier/rules/"+ruleID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_BandwidthPolicies(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/traffic-classifier/bandwidth-policies", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := `{"name":"custom","traffic_type":"video","min_mbps":10,"max_mbps":100,"priority":1,"enabled":true}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/traffic-classifier/bandwidth-policies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_QoSRules(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)
	body := `{"name":"qos-test","traffic_type":"game","dscp":46,"queue_id":1,"rate_mbps":10,"ceil_mbps":50,"priority":1,"enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/traffic-classifier/qos-rules", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/traffic-classifier/qos-rules", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_Alerts(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/traffic-classifier/alerts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_Reports(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)
	now := time.Now()
	body := fmt.Sprintf(`{"start_time":"%s","end_time":"%s","title":"test"}`,
		now.Add(-1*time.Hour).Format(time.RFC3339), now.Format(time.RFC3339))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/traffic-classifier/reports", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/traffic-classifier/reports", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_Config(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/traffic-classifier/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := `{"enabled":true,"max_flows":50000}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/traffic-classifier/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_StartStop(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/traffic-classifier/start", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/traffic-classifier/start", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/traffic-classifier/stop", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_Status(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/traffic-classifier/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_Mirrors(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)
	body := `{"name":"mirror-test","source_iface":"eth0","target_iface":"eth1","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/traffic-classifier/mirrors", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/traffic-classifier/mirrors", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_GetNonexistentRule(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/traffic-classifier/rules/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_GetNonexistentBandwidthPolicy(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/traffic-classifier/bandwidth-policies/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestClassifyByFeatures(t *testing.T) {
	m := setupTestManager(t)
	tests := []struct {
		name     string
		feature  *TrafficFeature
		expected TrafficType
	}{
		{"high-bw-long-dur", &TrafficFeature{ByteRate: 10000000, FlowDuration: 120, PacketRate: 100, AvgPacketSize: 1200}, TrafficTypeVideo},
		{"high-bw-short-dur", &TrafficFeature{ByteRate: 10000000, FlowDuration: 10, PacketRate: 100, AvgPacketSize: 1200}, TrafficTypeDownload},
		{"low-bw-low-rate", &TrafficFeature{ByteRate: 5000, PacketRate: 5, FlowDuration: 60, AvgPacketSize: 100}, TrafficTypeIoT},
		{"high-pkt-small", &TrafficFeature{ByteRate: 50000, PacketRate: 50, FlowDuration: 30, AvgPacketSize: 100}, TrafficTypeGame},
		{"low-bw-med-rate", &TrafficFeature{ByteRate: 100000, PacketRate: 50, FlowDuration: 30, AvgPacketSize: 500}, TrafficTypeAudio},
		{"unknown", &TrafficFeature{ByteRate: 5000, PacketRate: 20, FlowDuration: 5, AvgPacketSize: 300}, TrafficTypeUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := m.classifyByFeatures(tt.feature); result != tt.expected {
				t.Errorf("classifyByFeatures = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		ip      string
		want    bool
	}{
		{"192.168.1.0/24", "192.168.1.100", true},
		{"192.168.1.0/24", "10.0.0.1", false},
		{"10.0.0.1", "10.0.0.1", true},
		{"10.0.0.1", "10.0.0.2", false},
	}
	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.ip, func(t *testing.T) {
			got, err := matchPattern(tt.pattern, tt.ip)
			if err != nil {
				t.Fatalf("matchPattern error: %v", err)
			}
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.ip, got, tt.want)
			}
		})
	}
}

func TestMatchRule(t *testing.T) {
	m := setupTestManager(t)
	rule := &ClassificationRule{
		TrafficType: TrafficTypeVideo, DstIPPattern: "10.0.0.0/8",
		Ports: []int{8080, 9090}, Protocol: "tcp", Priority: 10, Enabled: true,
	}
	flowMatch := &TrafficFlow{SrcIP: "192.168.1.1", DstIP: "10.0.0.5", SrcPort: 12345, DstPort: 8080, Protocol: "tcp"}
	flowNoMatch := &TrafficFlow{SrcIP: "192.168.1.1", DstIP: "172.16.0.1", SrcPort: 12345, DstPort: 8080, Protocol: "tcp"}
	if !m.matchRule(rule, flowMatch) {
		t.Error("expected match for flowMatch")
	}
	if m.matchRule(rule, flowNoMatch) {
		t.Error("expected no match for flowNoMatch")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()
	if id1 == "" || id2 == "" {
		t.Error("expected non-empty IDs")
	}
	if id1 == id2 {
		t.Error("expected unique IDs")
	}
}

func TestUpdateStats(t *testing.T) {
	m := setupTestManager(t)
	flows := []TrafficFlow{makeTestFlow("s1", 443, "tcp"), makeTestFlow("s2", 80, "tcp")}
	m.updateStats(flows)
	stats := m.GetStats()
	if stats.TotalBytes == 0 {
		t.Error("expected non-zero total bytes")
	}
	if stats.TotalPackets == 0 {
		t.Error("expected non-zero total packets")
	}
	if stats.ProtocolBreakdown["tcp"] != 2 {
		t.Errorf("expected 2 tcp flows, got %d", stats.ProtocolBreakdown["tcp"])
	}
}

func TestNilLoggerDefaultsToNop(t *testing.T) {
	m := NewManager(nil, nil)
	if m.logger == nil {
		t.Error("expected nop logger when nil passed")
	}
}

func TestNilConfigDefaults(t *testing.T) {
	m := NewManager(zap.NewNop(), nil)
	if !m.GetConfig().Enabled {
		t.Error("expected enabled by default")
	}
}

func TestEstimateBursts(t *testing.T) {
	tests := []struct {
		packets     int64
		duration    float64
		minExpected int
	}{
		{100000, 1, 100},
		{100, 10, 1},
		{0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d-%.1f", tt.packets, tt.duration), func(t *testing.T) {
			if result := estimateBursts(tt.packets, tt.duration); result < tt.minExpected {
				t.Errorf("estimateBursts(%d, %.1f) = %d, want >= %d", tt.packets, tt.duration, result, tt.minExpected)
			}
		})
	}
}

func TestMax64(t *testing.T) {
	if max64(1, 2) != 2 {
		t.Error("expected 2")
	}
	if max64(5, 3) != 5 {
		t.Error("expected 5")
	}
	if max64(7, 7) != 7 {
		t.Error("expected 7")
	}
}

func TestAnalyzeWithDPI(t *testing.T) {
	m := setupTestManager(t)
	m.Start()
	defer m.Stop()
	flow := makeTestFlow("dpi-flow", 554, "tcp")
	resp, err := m.AnalyzeFlows(&AnalyzeRequest{Flows: []TrafficFlow{flow}, WithDPI: true})
	if err != nil {
		t.Fatalf("AnalyzeFlows with DPI failed: %v", err)
	}
	if resp.Results[0].TrafficType != TrafficTypeVideo {
		t.Errorf("expected video with DPI, got %v", resp.Results[0].TrafficType)
	}
	if resp.Results[0].ModelName == "" {
		t.Error("expected model name for DPI result")
	}
}

func TestUpdateRuleNonexistent(t *testing.T) {
	m := setupTestManager(t)
	if err := m.UpdateRule(&ClassificationRule{ID: "nonexistent", Name: "test", TrafficType: TrafficTypeVideo}); err == nil {
		t.Error("expected error for nonexistent rule")
	}
}

func TestGetRuleNonexistent(t *testing.T) {
	m := setupTestManager(t)
	if _, err := m.GetRule("nonexistent"); err == nil {
		t.Error("expected error for nonexistent rule")
	}
}

func TestGetBandwidthPolicyNonexistent(t *testing.T) {
	m := setupTestManager(t)
	if _, err := m.GetBandwidthPolicy("nonexistent"); err == nil {
		t.Error("expected error for nonexistent policy")
	}
}

func TestReportDefaultTitle(t *testing.T) {
	m := setupTestManager(t)
	now := time.Now()
	report := m.GenerateReport(&ReportRequest{StartTime: now.Add(-1 * time.Hour), EndTime: now})
	if report.Title == "" {
		t.Error("expected auto-generated title when empty")
	}
}

func TestGetReportNonexistent(t *testing.T) {
	m := setupTestManager(t)
	if _, err := m.GetReport("nonexistent"); err == nil {
		t.Error("expected error for nonexistent report")
	}
}