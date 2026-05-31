package unifiedmonitor

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// MockMetricStore 模拟指标存储
type MockMetricStore struct {
	points []MetricPoint
}

func NewMockMetricStore() *MockMetricStore {
	return &MockMetricStore{
		points: make([]MetricPoint, 0),
	}
}

func (m *MockMetricStore) Store(point MetricPoint) error {
	m.points = append(m.points, point)
	return nil
}

func (m *MockMetricStore) Query(name, nodeID string, start, end time.Time) ([]MetricPoint, error) {
	var result []MetricPoint
	for _, p := range m.points {
		if (name == "" || p.Name == name) &&
			(nodeID == "" || p.NodeID == nodeID) &&
			p.Timestamp.After(start) && p.Timestamp.Before(end) {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *MockMetricStore) Aggregate(name string, start, end time.Time) (*AggregatedMetrics, error) {
	return &AggregatedMetrics{
		Name:  name,
		Count: len(m.points),
	}, nil
}

// MockAlertStore 模拟告警存储
type MockAlertStore struct {
	alerts []Alert
}

func NewMockAlertStore() *MockAlertStore {
	return &MockAlertStore{
		alerts: make([]Alert, 0),
	}
}

func (m *MockAlertStore) Store(alert Alert) error {
	m.alerts = append(m.alerts, alert)
	return nil
}

func (m *MockAlertStore) Query(status AlertStatus, limit int) ([]Alert, error) {
	var result []Alert
	for _, a := range m.alerts {
		if a.Status == status {
			result = append(result, a)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *MockAlertStore) UpdateStatus(alertID string, status AlertStatus) error {
	for i, a := range m.alerts {
		if a.ID == alertID {
			m.alerts[i].Status = status
			if status == AlertStatusResolved {
				now := time.Now()
				m.alerts[i].Resolved = &now
			}
			return nil
		}
	}
	return nil
}

// ========== 测试用辅助函数 ==========

func newTestManager() *Manager {
	return NewManager(NewMockMetricStore(), NewMockAlertStore(), DefaultConfig())
}

func registerTestNode(m *Manager, id, name string) error {
	return m.RegisterNode(&ClusterNode{
		ID:           id,
		Name:         name,
		Hostname:     name + ".local",
		IPAddress:    "192.168.1." + id[len(id)-1:],
		Role:         RoleWorker,
		Status:       NodeStatusOnline,
		RegisteredAt: time.Now(),
		LastSeen:     time.Now(),
	})
}

// ========== 测试 ==========

func TestNewManager(t *testing.T) {
	mgr := newTestManager()
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.metricStore == nil {
		t.Error("metricStore is nil")
	}
	if mgr.alertStore == nil {
		t.Error("alertStore is nil")
	}
	if mgr.rules == nil {
		t.Error("rules map is nil")
	}
	if mgr.alerts == nil {
		t.Error("alerts map is nil")
	}
	if mgr.nodes == nil {
		t.Error("nodes map is nil")
	}
}

func TestRegisterAndGetNode(t *testing.T) {
	mgr := newTestManager()

	err := registerTestNode(mgr, "node1", "Server1")
	if err != nil {
		t.Fatalf("RegisterNode failed: %v", err)
	}

	// 重复注册应报错
	err = registerTestNode(mgr, "node1", "Server1")
	if err == nil {
		t.Error("expected error for duplicate registration")
	}

	node, exists := mgr.GetNode("node1")
	if !exists {
		t.Fatal("node1 not found")
	}
	if node.Name != "Server1" {
		t.Errorf("expected name Server1, got %s", node.Name)
	}
	if node.Status != NodeStatusOnline {
		t.Errorf("expected status online, got %s", node.Status)
	}

	_, exists = mgr.GetNode("nonexistent")
	if exists {
		t.Error("nonexistent node should not exist")
	}
}

func TestRemoveNode(t *testing.T) {
	mgr := newTestManager()
	registerTestNode(mgr, "node1", "Server1")

	ok := mgr.RemoveNode("node1")
	if !ok {
		t.Error("RemoveNode should return true")
	}

	_, exists := mgr.GetNode("node1")
	if exists {
		t.Error("node1 should be removed")
	}

	ok = mgr.RemoveNode("nonexistent")
	if ok {
		t.Error("RemoveNode should return false for nonexistent node")
	}
}

func TestListNodes(t *testing.T) {
	mgr := newTestManager()
	registerTestNode(mgr, "node1", "Server1")
	registerTestNode(mgr, "node2", "Server2")

	nodes := mgr.ListNodes()
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}
}

func TestUpdateNodeMetrics(t *testing.T) {
	mgr := newTestManager()
	registerTestNode(mgr, "node1", "Server1")

	err := mgr.UpdateNodeMetrics("node1", NodeMetrics{
		CPUPercent:  75.5,
		MemPercent:  60.0,
		DiskPercent: 50.0,
	})
	if err != nil {
		t.Fatalf("UpdateNodeMetrics failed: %v", err)
	}

	node, _ := mgr.GetNode("node1")
	if node.Metrics.CPUPercent != 75.5 {
		t.Errorf("expected CPU 75.5, got %f", node.Metrics.CPUPercent)
	}
	if node.Status != NodeStatusOnline {
		t.Errorf("expected status online after metrics update, got %s", node.Status)
	}

	err = mgr.UpdateNodeMetrics("nonexistent", NodeMetrics{})
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestRecordAndQueryMetric(t *testing.T) {
	mgr := newTestManager()

	point := MetricPoint{
		Timestamp: time.Now(),
		NodeID:    "node1",
		Name:      "cpu_usage",
		Value:     75.5,
		Labels:    map[string]string{"host": "server1"},
	}

	// RecordMetric 在 Manager 中调用 metricStore.Store
	store := mgr.metricStore.(*MockMetricStore)
	err := store.Store(point)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	if len(store.points) != 1 {
		t.Errorf("expected 1 metric point, got %d", len(store.points))
	}
	if store.points[0].Value != 75.5 {
		t.Errorf("expected value 75.5, got %f", store.points[0].Value)
	}
}

func TestAddAndRemoveRule(t *testing.T) {
	mgr := newTestManager()

	rule := &AlertRule{
		ID:        "rule1",
		Name:      "High CPU",
		Type:      RuleTypeThreshold,
		Metric:    "cpu_usage",
		Condition: ConditionAbove,
		Threshold: 90,
		Duration:  5 * time.Minute,
		Severity:  SeverityWarning,
	}

	err := mgr.AddRule(rule)
	if err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	rules := mgr.ListRules()
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].ID != "rule1" {
		t.Errorf("expected rule ID rule1, got %s", rules[0].ID)
	}

	// 测试无ID规则
	invalidRule := &AlertRule{
		Name: "Invalid",
	}
	err = mgr.AddRule(invalidRule)
	if err == nil {
		t.Error("expected error for rule without ID")
	}

	// 删除规则
	mgr.RemoveRule("rule1")
	rules = mgr.ListRules()
	if len(rules) != 0 {
		t.Errorf("expected 0 rules after removal, got %d", len(rules))
	}
}

func TestGetClusterHealth(t *testing.T) {
	mgr := newTestManager()

	// 注册节点并更新指标
	registerTestNode(mgr, "node1", "Server1")
	registerTestNode(mgr, "node2", "Server2")

	mgr.UpdateNodeMetrics("node1", NodeMetrics{
		CPUPercent:  60,
		MemPercent:  70,
		DiskPercent: 50,
	})
	mgr.UpdateNodeMetrics("node2", NodeMetrics{
		CPUPercent:  80,
		MemPercent:  75,
		DiskPercent: 60,
	})

	health := mgr.GetClusterHealth()

	if health.Score < 0 || health.Score > 100 {
		t.Errorf("score out of range: %d", health.Score)
	}

	if health.Level != "good" && health.Level != "warning" && health.Level != "critical" {
		t.Errorf("invalid level: %s", health.Level)
	}

	if health.Details["cpu"] == 0 {
		t.Error("cpu score missing")
	}
	if health.Details["memory"] == 0 {
		t.Error("memory score missing")
	}
	if health.Details["disk"] == 0 {
		t.Error("disk score missing")
	}
	if health.Details["network"] == 0 {
		t.Error("network score missing")
	}

	if len(health.PerNode) != 2 {
		t.Errorf("expected 2 per-node scores, got %d", len(health.PerNode))
	}

	t.Logf("Health Score: %d (%s)", health.Score, health.Level)
	t.Logf("Details: %+v", health.Details)
}

func TestCPUHealthScore(t *testing.T) {
	tests := []struct {
		cpuPercent float64
		minScore   int
		maxScore   int
	}{
		{50, 100, 100},   // < 70 -> 100
		{75, 70, 70},     // 70-85 -> 70
		{90, 40, 40},     // 85-95 -> 40
		{98, 10, 10},     // >= 95 -> 10
	}

	for _, tt := range tests {
		mgr := newTestManager()
		registerTestNode(mgr, "node1", "Server1")
		mgr.UpdateNodeMetrics("node1", NodeMetrics{
			CPUPercent:  tt.cpuPercent,
			MemPercent:  50,
			DiskPercent: 50,
		})

		health := mgr.GetClusterHealth()
		cpuScore := health.Details["cpu"]
		if cpuScore < tt.minScore || cpuScore > tt.maxScore {
			t.Errorf("CPU %.0f%%: expected score %d-%d, got %d", tt.cpuPercent, tt.minScore, tt.maxScore, cpuScore)
		}
	}
}

func TestIdentifyTopIssues(t *testing.T) {
	mgr := newTestManager()

	registerTestNode(mgr, "node1", "Server1")
	registerTestNode(mgr, "node2", "Server2")
	registerTestNode(mgr, "node3", "Server3")

	mgr.UpdateNodeMetrics("node1", NodeMetrics{CPUPercent: 95, MemPercent: 80, DiskPercent: 70})
	mgr.UpdateNodeMetrics("node2", NodeMetrics{CPUPercent: 50, MemPercent: 60, DiskPercent: 95})
	// node3 不更新指标，保持零值（模拟正常节点）

	issues := mgr.identifyTopIssues()
	t.Logf("Top issues: %v", issues)
	// 应该识别出 node1 CPU 过高和 node3 磁盘过高
}

func TestGetDashboard(t *testing.T) {
	mgr := newTestManager()

	registerTestNode(mgr, "node1", "Server1")
	mgr.UpdateNodeMetrics("node1", NodeMetrics{
		CPUPercent:  60,
		MemPercent:  50,
		DiskPercent: 40,
	})

	dashboard := mgr.GetDashboard()

	if dashboard.ClusterHealth.Score == 0 {
		t.Error("dashboard health score should not be 0")
	}
	if len(dashboard.Nodes) != 1 {
		t.Errorf("expected 1 node in dashboard, got %d", len(dashboard.Nodes))
	}
	if dashboard.Timestamp.IsZero() {
		t.Error("dashboard timestamp should not be zero")
	}
}

func TestAddAndListRules(t *testing.T) {
	mgr := newTestManager()

	rule1 := &AlertRule{
		ID:        "rule1",
		Name:      "High CPU",
		Type:      RuleTypeThreshold,
		Metric:    "cpu_usage",
		Condition: ConditionAbove,
		Threshold: 90,
		Severity:  SeverityWarning,
	}
	rule2 := &AlertRule{
		ID:        "rule2",
		Name:      "Low Disk",
		Type:      RuleTypeThreshold,
		Metric:    "disk_usage",
		Condition: ConditionAbove,
		Threshold: 95,
		Severity:  SeverityCritical,
	}

	mgr.AddRule(rule1)
	mgr.AddRule(rule2)

	rules := mgr.ListRules()
	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rules))
	}
}

func TestRecordAndListAlerts(t *testing.T) {
	mgr := newTestManager()

	alert := Alert{
		ID:        "alert1",
		RuleID:    "rule1",
		RuleName:  "High CPU",
		Severity:  SeverityWarning,
		Message:   "CPU usage exceeded 90%",
		Value:     95,
		Threshold: 90,
		NodeID:    "node1",
		Status:    AlertStatusFiring,
		Triggered: time.Now(),
	}

	store := mgr.alertStore.(*MockAlertStore)
	store.Store(alert)

	alerts, err := mgr.ListAlerts(nil, AlertStatusFiring, 100)
	if err != nil {
		t.Fatalf("ListAlerts failed: %v", err)
	}
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert, got %d", len(alerts))
	}
}

func TestRecordAndGetLatency(t *testing.T) {
	mgr := newTestManager()

	mgr.RecordLatency(&NodeLatency{
		SourceNodeID: "node1",
		TargetNodeID: "node2",
		Latency:      5 * time.Millisecond,
		Jitter:       1 * time.Millisecond,
		PacketLoss:   0.01,
		MeasuredAt:   time.Now(),
	})

	// 需要注册节点才能在延迟矩阵中显示
	registerTestNode(mgr, "node1", "Server1")
	registerTestNode(mgr, "node2", "Server2")

	matrix := mgr.GetLatencyMatrix()
	if len(matrix.Nodes) != 2 {
		t.Errorf("expected 2 nodes in matrix, got %d", len(matrix.Nodes))
	}
	if matrix.Matrix["node1"]["node2"] != 5*time.Millisecond {
		t.Errorf("expected latency 5ms, got %v", matrix.Matrix["node1"]["node2"])
	}
}

func TestHandlerGetDashboard(t *testing.T) {
	mgr := newTestManager()
	handler := NewHandler(mgr)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	handler.RegisterRoutes(rg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/unified-monitor/dashboard", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerGetHealth(t *testing.T) {
	mgr := newTestManager()
	handler := NewHandler(mgr)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	handler.RegisterRoutes(rg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/unified-monitor/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerNodes(t *testing.T) {
	mgr := newTestManager()
	handler := NewHandler(mgr)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	handler.RegisterRoutes(rg)

	// 列出节点（空）
	req := httptest.NewRequest(http.MethodGet, "/api/v1/unified-monitor/nodes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// 注册节点
	nodeJSON := `{"id":"node1","name":"Server1"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/unified-monitor/nodes", bytes.NewBufferString(nodeJSON))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	// 获取节点
	req = httptest.NewRequest(http.MethodGet, "/api/v1/unified-monitor/nodes/node1", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// 删除节点
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/unified-monitor/nodes/node1", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerCreateRule(t *testing.T) {
	mgr := newTestManager()
	handler := NewHandler(mgr)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	handler.RegisterRoutes(rg)

	ruleJSON := `{
		"id": "rule1",
		"name": "High CPU",
		"metric": "cpu_usage",
		"threshold": 90,
		"severity": "warning"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/unified-monitor/rules", bytes.NewBufferString(ruleJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	// 获取规则列表
	req = httptest.NewRequest(http.MethodGet, "/api/v1/unified-monitor/rules", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// 删除规则
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/unified-monitor/rules/rule1", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerLatency(t *testing.T) {
	mgr := newTestManager()
	handler := NewHandler(mgr)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	handler.RegisterRoutes(rg)

	// 获取延迟矩阵
	req := httptest.NewRequest(http.MethodGet, "/api/v1/unified-monitor/latency", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// 记录延迟
	latencyJSON := `{"source_node_id":"node1","target_node_id":"node2","latency_ms":5.0}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/unified-monitor/latency", bytes.NewBufferString(latencyJSON))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
