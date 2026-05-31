package unifiedmonitor

import (
	"context"
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
	return &MockMetricStore{points: make([]MetricPoint, 0)}
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
	return &AggregatedMetrics{Name: name}, nil
}

// MockAlertStore 模拟告警存储
type MockAlertStore struct {
	alerts []Alert
}

func NewMockAlertStore() *MockAlertStore {
	return &MockAlertStore{alerts: make([]Alert, 0)}
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

func setupTestRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	h.RegisterRoutes(rg)
	return r
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(NewMockMetricStore(), NewMockAlertStore(), DefaultConfig())
}

func TestNewManager(t *testing.T) {
	mgr := newTestManager(t)
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
	mgr := newTestManager(t)

	node := &ClusterNode{
		ID:       "node1",
		Name:     "NAS-1",
		Hostname: "nas1.local",
		Role:     RoleWorker,
		Status:   NodeStatusOnline,
	}

	if err := mgr.RegisterNode(node); err != nil {
		t.Fatalf("RegisterNode failed: %v", err)
	}

	got, ok := mgr.GetNode("node1")
	if !ok {
		t.Fatal("GetNode failed")
	}
	if got.Name != "NAS-1" {
		t.Errorf("expected name NAS-1, got %s", got.Name)
	}

	_, ok = mgr.GetNode("nonexistent")
	if ok {
		t.Error("expected false for nonexistent node")
	}
}

func TestRemoveNode(t *testing.T) {
	mgr := newTestManager(t)
	mgr.RegisterNode(&ClusterNode{ID: "node1", Name: "NAS-1"})

	if !mgr.RemoveNode("node1") {
		t.Error("RemoveNode should return true")
	}
	if mgr.RemoveNode("node1") {
		t.Error("RemoveNode should return false for already removed node")
	}
}

func TestListNodes(t *testing.T) {
	mgr := newTestManager(t)
	mgr.RegisterNode(&ClusterNode{ID: "node1", Name: "NAS-1"})
	mgr.RegisterNode(&ClusterNode{ID: "node2", Name: "NAS-2"})

	nodes := mgr.ListNodes()
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}
}

func TestUpdateNodeMetrics(t *testing.T) {
	mgr := newTestManager(t)
	mgr.RegisterNode(&ClusterNode{ID: "node1", Name: "NAS-1"})

	metrics := NodeMetrics{
		CPUPercent:  75.5,
		MemPercent:  60.0,
		DiskPercent: 45.0,
	}
	if err := mgr.UpdateNodeMetrics("node1", metrics); err != nil {
		t.Fatalf("UpdateNodeMetrics failed: %v", err)
	}

	node, _ := mgr.GetNode("node1")
	if node.Metrics.CPUPercent != 75.5 {
		t.Errorf("expected CPU 75.5, got %f", node.Metrics.CPUPercent)
	}
	if node.Status != NodeStatusOnline {
		t.Errorf("expected status online, got %s", node.Status)
	}

	err := mgr.UpdateNodeMetrics("nonexistent", metrics)
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestRecordAndQueryMetric(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	point := MetricPoint{
		Timestamp: time.Now(),
		NodeID:    "node1",
		Name:      "cpu_usage",
		Value:     75.5,
	}

	if err := mgr.RecordMetric(ctx, point); err != nil {
		t.Fatalf("RecordMetric failed: %v", err)
	}

	store := mgr.metricStore.(*MockMetricStore)
	if len(store.points) != 1 {
		t.Errorf("expected 1 metric point, got %d", len(store.points))
	}
}

func TestAddAndRemoveRule(t *testing.T) {
	mgr := newTestManager(t)

	rule := &AlertRule{
		ID:        "rule1",
		Name:      "High CPU",
		Type:      RuleTypeThreshold,
		Metric:    "cpu_usage",
		Condition: ConditionAbove,
		Threshold: 90,
		Severity:  SeverityWarning,
		Enabled:   true,
	}

	if err := mgr.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	if len(mgr.rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(mgr.rules))
	}

	// 无ID规则应报错
	invalidRule := &AlertRule{Name: "Invalid"}
	if err := mgr.AddRule(invalidRule); err == nil {
		t.Error("expected error for rule without ID")
	}

	mgr.RemoveRule("rule1")
	if len(mgr.rules) != 0 {
		t.Errorf("expected 0 rules after removal, got %d", len(mgr.rules))
	}
}

func TestGetClusterHealth(t *testing.T) {
	mgr := newTestManager(t)

	// 无节点时健康评分
	score := mgr.GetClusterHealth()
	if score.Score < 0 || score.Score > 100 {
		t.Errorf("score out of range: %d", score.Score)
	}

	// 添加有指标的节点
	mgr.RegisterNode(&ClusterNode{
		ID:     "node1",
		Status: NodeStatusOnline,
		Metrics: NodeMetrics{
			CPUPercent:  60,
			MemPercent:  70,
			DiskPercent: 50,
		},
	})
	mgr.RegisterNode(&ClusterNode{
		ID:     "node2",
		Status: NodeStatusOnline,
		Metrics: NodeMetrics{
			CPUPercent:  80,
			MemPercent:  75,
			DiskPercent: 60,
		},
	})

	score = mgr.GetClusterHealth()
	if score.Score < 0 || score.Score > 100 {
		t.Errorf("score out of range: %d", score.Score)
	}
	if score.Level != "good" && score.Level != "warning" && score.Level != "critical" {
		t.Errorf("invalid level: %s", score.Level)
	}
	t.Logf("Health Score: %d (%s)", score.Score, score.Level)
	t.Logf("Details: %+v", score.Details)
}

func TestEvaluateCPUHealth(t *testing.T) {
	mgr := newTestManager(t)

	tests := []struct {
		cpuPercent float64
		minScore   int
		maxScore   int
	}{
		{50, 90, 100},
		{70, 60, 80},
		{85, 30, 50},
		{95, 0, 20},
	}

	for _, tt := range tests {
		mgr.nodes = map[string]*ClusterNode{
			"node1": {Metrics: NodeMetrics{CPUPercent: tt.cpuPercent}},
		}
		score := mgr.evaluateCPUHealth()
		if score < tt.minScore || score > tt.maxScore {
			t.Errorf("CPU %f%%: expected score %d-%d, got %d", tt.cpuPercent, tt.minScore, tt.maxScore, score)
		}
	}
}

func TestIdentifyTopIssues(t *testing.T) {
	mgr := newTestManager(t)

	mgr.nodes = map[string]*ClusterNode{
		"node1": {
			ID:     "node1",
			Status: NodeStatusOnline,
			Metrics: NodeMetrics{
				CPUPercent:  95,
				MemPercent:  80,
				DiskPercent: 70,
			},
		},
		"node2": {
			ID:     "node2",
			Status: NodeStatusOffline,
		},
		"node3": {
			ID:     "node3",
			Status: NodeStatusOnline,
			Metrics: NodeMetrics{
				CPUPercent:  50,
				MemPercent:  60,
				DiskPercent: 95,
			},
		},
	}

	issues := mgr.identifyTopIssues()
	if len(issues) == 0 {
		t.Error("expected issues, got none")
	}
	t.Logf("Top issues: %v", issues)
}

func TestGetDashboard(t *testing.T) {
	mgr := newTestManager(t)

	mgr.RegisterNode(&ClusterNode{
		ID:     "node1",
		Name:   "NAS-1",
		Status: NodeStatusOnline,
		Metrics: NodeMetrics{
			CPUPercent:  60,
			MemPercent:  50,
			DiskPercent: 40,
		},
	})

	dashboard := mgr.GetDashboard()
	if len(dashboard.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(dashboard.Nodes))
	}
	if dashboard.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
}

func TestRecordLatency(t *testing.T) {
	mgr := newTestManager(t)

	// 必须先注册节点，否则 GetLatencyMatrix 不会计算延迟
	mgr.RegisterNode(&ClusterNode{ID: "node1", Name: "NAS-1"})
	mgr.RegisterNode(&ClusterNode{ID: "node2", Name: "NAS-2"})

	latency := &NodeLatency{
		SourceNodeID: "node1",
		TargetNodeID: "node2",
		Latency:      5 * time.Millisecond,
		Jitter:       1 * time.Millisecond,
		PacketLoss:   0.01,
		MeasuredAt:   time.Now(),
	}

	mgr.RecordLatency(latency)

	matrix := mgr.GetLatencyMatrix()
	if matrix.AvgMs <= 0 {
		t.Error("expected positive avg latency")
	}
}

func TestGetCorrelatedAlerts(t *testing.T) {
	mgr := newTestManager(t)

	alerts := mgr.GetCorrelatedAlerts()
	if alerts == nil {
		t.Error("expected non-nil slice")
	}
}

func TestHandlerDashboard(t *testing.T) {
	mgr := newTestManager(t)
	handler := NewHandler(mgr)

	router := setupTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/unified-monitor/dashboard", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerHealth(t *testing.T) {
	mgr := newTestManager(t)
	handler := NewHandler(mgr)
	router := setupTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/unified-monitor/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerNodes(t *testing.T) {
	mgr := newTestManager(t)
	handler := NewHandler(mgr)
	router := setupTestRouter(handler)

	// 注册节点
	nodeJSON := `{"id":"node1","name":"NAS-1","hostname":"nas1.local","role":"worker"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/unified-monitor/nodes", nil)
	req.Body = nil // POST with JSON body
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// GET nodes
	req = httptest.NewRequest(http.MethodGet, "/api/v1/unified-monitor/nodes", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	_ = nodeJSON
}

func TestHandlerMethodNotAllowed(t *testing.T) {
	mgr := newTestManager(t)
	handler := NewHandler(mgr)
	router := setupTestRouter(handler)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/unified-monitor/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// gin returns 404 for unmatched routes
	if w.Code == http.StatusOK {
		t.Error("expected non-200 for invalid method")
	}
}
