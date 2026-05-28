package unifiedmonitor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

func (m *MockMetricStore) Store(ctx context.Context, point MetricPoint) error {
	m.points = append(m.points, point)
	return nil
}

func (m *MockMetricStore) Query(ctx context.Context, name, nodeID string, start, end time.Time) ([]MetricPoint, error) {
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

func (m *MockMetricStore) Aggregate(ctx context.Context, name, nodeID string, start, end time.Time, interval time.Duration) ([]MetricPoint, error) {
	return m.Query(ctx, name, nodeID, start, end)
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

func (m *MockAlertStore) Store(ctx context.Context, alert Alert) error {
	m.alerts = append(m.alerts, alert)
	return nil
}

func (m *MockAlertStore) Query(ctx context.Context, status AlertStatus, limit int) ([]Alert, error) {
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

func (m *MockAlertStore) UpdateStatus(ctx context.Context, alertID string, status AlertStatus) error {
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

func TestNewMonitor(t *testing.T) {
	metricStore := NewMockMetricStore()
	alertStore := NewMockAlertStore()
	config := DefaultConfig()
	
	monitor := NewMonitor(metricStore, alertStore, config)
	
	if monitor == nil {
		t.Fatal("NewMonitor returned nil")
	}
	if monitor.metricStore == nil {
		t.Error("metricStore is nil")
	}
	if monitor.alertStore == nil {
		t.Error("alertStore is nil")
	}
	if monitor.rules == nil {
		t.Error("rules map is nil")
	}
	if monitor.alerts == nil {
		t.Error("alerts map is nil")
	}
	if monitor.nodeStatus == nil {
		t.Error("nodeStatus map is nil")
	}
}

func TestRecordMetric(t *testing.T) {
	metricStore := NewMockMetricStore()
	alertStore := NewMockAlertStore()
	config := DefaultConfig()
	
	monitor := NewMonitor(metricStore, alertStore, config)
	ctx := context.Background()
	
	point := MetricPoint{
		Timestamp: time.Now(),
		NodeID:    "node1",
		Name:      "cpu_usage",
		Value:     75.5,
		Labels:    map[string]string{"host": "server1"},
	}
	
	err := monitor.RecordMetric(ctx, point)
	if err != nil {
		t.Fatalf("RecordMetric failed: %v", err)
	}
	
	if len(metricStore.points) != 1 {
		t.Errorf("expected 1 metric point, got %d", len(metricStore.points))
	}
	
	if metricStore.points[0].Value != 75.5 {
		t.Errorf("expected value 75.5, got %f", metricStore.points[0].Value)
	}
	
	// 验证节点状态更新
	monitor.mu.RLock()
	node, exists := monitor.nodeStatus["node1"]
	monitor.mu.RUnlock()
	
	if !exists {
		t.Error("node1 not found in nodeStatus")
	} else {
		if node.CPUPercent != 75.5 {
			t.Errorf("expected CPU 75.5%%, got %f%%", node.CPUPercent)
		}
		if !node.Online {
			t.Error("node1 should be online")
		}
	}
}

func TestGetHealthScore(t *testing.T) {
	metricStore := NewMockMetricStore()
	alertStore := NewMockAlertStore()
	config := DefaultConfig()
	
	monitor := NewMonitor(metricStore, alertStore, config)
	ctx := context.Background()
	
	// 添加节点数据
	monitor.mu.Lock()
	monitor.nodeStatus["node1"] = &NodeStatus{
		NodeID:      "node1",
		Online:      true,
		CPUPercent:  60,
		MemPercent:  70,
		DiskPercent: 50,
	}
	monitor.nodeStatus["node2"] = &NodeStatus{
		NodeID:      "node2",
		Online:      true,
		CPUPercent:  80,
		MemPercent:  75,
		DiskPercent: 60,
	}
	monitor.mu.Unlock()
	
	score, err := monitor.GetHealthScore(ctx)
	if err != nil {
		t.Fatalf("GetHealthScore failed: %v", err)
	}
	
	if score == nil {
		t.Fatal("GetHealthScore returned nil")
	}
	
	if score.Score < 0 || score.Score > 100 {
		t.Errorf("score out of range: %d", score.Score)
	}
	
	if score.Level != "good" && score.Level != "warning" && score.Level != "critical" {
		t.Errorf("invalid level: %s", score.Level)
	}
	
	if score.Details["cpu"] == 0 {
		t.Error("cpu score missing")
	}
	if score.Details["memory"] == 0 {
		t.Error("memory score missing")
	}
	if score.Details["disk"] == 0 {
		t.Error("disk score missing")
	}
	if score.Details["network"] == 0 {
		t.Error("network score missing")
	}
	
	t.Logf("Health Score: %d (%s)", score.Score, score.Level)
	t.Logf("Details: %+v", score.Details)
}

func TestAddAndRemoveRule(t *testing.T) {
	metricStore := NewMockMetricStore()
	alertStore := NewMockAlertStore()
	config := DefaultConfig()
	
	monitor := NewMonitor(metricStore, alertStore, config)
	
	rule := &AlertRule{
		ID:       "rule1",
		Name:     "High CPU",
		Type:     RuleTypeThreshold,
		Metric:   "cpu_usage",
		Condition: ConditionAbove,
		Threshold: 90,
		Duration: 5 * time.Minute,
		Severity: SeverityWarning,
		Enabled:  true,
	}
	
	// 添加规则
	err := monitor.AddRule(rule)
	if err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}
	
	monitor.mu.RLock()
	if len(monitor.rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(monitor.rules))
	}
	if _, exists := monitor.rules["rule1"]; !exists {
		t.Error("rule1 not found")
	}
	monitor.mu.RUnlock()
	
	// 测试无ID规则
	invalidRule := &AlertRule{
		Name: "Invalid",
	}
	err = monitor.AddRule(invalidRule)
	if err == nil {
		t.Error("expected error for rule without ID")
	}
	
	// 删除规则
	monitor.RemoveRule("rule1")
	monitor.mu.RLock()
	if len(monitor.rules) != 0 {
		t.Errorf("expected 0 rules after removal, got %d", len(monitor.rules))
	}
	monitor.mu.RUnlock()
}

func TestEvaluateThreshold(t *testing.T) {
	metricStore := NewMockMetricStore()
	alertStore := NewMockAlertStore()
	config := DefaultConfig()
	
	monitor := NewMonitor(metricStore, alertStore, config)
	
	tests := []struct {
		condition AlertCondition
		value     float64
		threshold float64
		expected  bool
	}{
		{ConditionAbove, 95, 90, true},
		{ConditionAbove, 85, 90, false},
		{ConditionBelow, 10, 20, true},
		{ConditionBelow, 30, 20, false},
		{ConditionEqual, 50, 50, true},
		{ConditionEqual, 50.02, 50, false},
	}
	
	for _, tt := range tests {
		result := monitor.evaluateThreshold(tt.condition, tt.value, tt.threshold)
		if result != tt.expected {
			t.Errorf("evaluateThreshold(%s, %f, %f) = %v, want %v",
				tt.condition, tt.value, tt.threshold, result, tt.expected)
		}
	}
}

func TestCalculateTrend(t *testing.T) {
	metricStore := NewMockMetricStore()
	alertStore := NewMockAlertStore()
	config := DefaultConfig()
	
	monitor := NewMonitor(metricStore, alertStore, config)
	
	// 上升趋势
	points := []MetricPoint{
		{Value: 10},
		{Value: 20},
		{Value: 30},
		{Value: 40},
		{Value: 50},
	}
	
	trend := monitor.calculateTrend(points)
	if trend <= 0 {
		t.Errorf("expected positive trend, got %f", trend)
	}
	
	// 下降趋势
	points = []MetricPoint{
		{Value: 50},
		{Value: 40},
		{Value: 30},
		{Value: 20},
		{Value: 10},
	}
	
	trend = monitor.calculateTrend(points)
	if trend >= 0 {
		t.Errorf("expected negative trend, got %f", trend)
	}
	
	// 空数据
	points = []MetricPoint{}
	trend = monitor.calculateTrend(points)
	if trend != 0 {
		t.Errorf("expected 0 trend for empty points, got %f", trend)
	}
}

func TestCalculateStats(t *testing.T) {
	metricStore := NewMockMetricStore()
	alertStore := NewMockAlertStore()
	config := DefaultConfig()
	
	monitor := NewMonitor(metricStore, alertStore, config)
	
	points := []MetricPoint{
		{Value: 10},
		{Value: 20},
		{Value: 30},
		{Value: 40},
		{Value: 50},
	}
	
	mean, stddev := monitor.calculateStats(points)
	
	if mean != 30 {
		t.Errorf("expected mean 30, got %f", mean)
	}
	
	if stddev < 14 || stddev > 15 {
		t.Errorf("expected stddev around 14.14, got %f", stddev)
	}
}

func TestCreateAlert(t *testing.T) {
	metricStore := NewMockMetricStore()
	alertStore := NewMockAlertStore()
	config := DefaultConfig()
	
	monitor := NewMonitor(metricStore, alertStore, config)
	
	rule := &AlertRule{
		ID:       "rule1",
		Name:     "High CPU",
		Type:     RuleTypeThreshold,
		Metric:   "cpu_usage",
		Condition: ConditionAbove,
		Threshold: 90,
		Severity: SeverityWarning,
	}
	
	// 创建告警
	monitor.createAlert(rule, 95)
	
	monitor.mu.RLock()
	if len(monitor.alerts) != 1 {
		t.Errorf("expected 1 alert, got %d", len(monitor.alerts))
	}
	monitor.mu.RUnlock()
	
	// 等待异步存储
	time.Sleep(100 * time.Millisecond)
	
	if len(alertStore.alerts) != 1 {
		t.Errorf("expected 1 alert in store, got %d", len(alertStore.alerts))
	}
	
	// 测试去重
	monitor.createAlert(rule, 96)
	time.Sleep(100 * time.Millisecond)
	
	monitor.mu.RLock()
	if len(monitor.alerts) != 1 {
		t.Errorf("expected 1 alert after dedup, got %d", len(monitor.alerts))
	}
	monitor.mu.RUnlock()
}

func TestHealthEvaluation(t *testing.T) {
	metricStore := NewMockMetricStore()
	alertStore := NewMockAlertStore()
	config := DefaultConfig()
	
	monitor := NewMonitor(metricStore, alertStore, config)
	
	// 测试CPU评分
	tests := []struct {
		cpuPercent float64
		expected   int
	}{
		{50, 100},
		{60, 100},
		{69, 100},
		{70, 70},
		{75, 70},
		{84, 70},
		{85, 40},
		{90, 40},
		{94, 40},
		{95, 10},
		{98, 10},
	}
	
	for _, tt := range tests {
		monitor.mu.Lock()
		monitor.nodeStatus = map[string]*NodeStatus{
			"node1": {CPUPercent: tt.cpuPercent},
		}
		monitor.mu.Unlock()
		
		score := monitor.evaluateCPUHealth()
		if score != tt.expected {
			t.Errorf("CPU %f%%: expected score %d, got %d", tt.cpuPercent, tt.expected, score)
		}
	}
}

func TestIdentifyTopIssues(t *testing.T) {
	metricStore := NewMockMetricStore()
	alertStore := NewMockAlertStore()
	config := DefaultConfig()
	
	monitor := NewMonitor(metricStore, alertStore, config)
	
	// 添加有问题的节点
	monitor.mu.Lock()
	monitor.nodeStatus = map[string]*NodeStatus{
		"node1": {
			NodeID:      "node1",
			Online:      true,
			CPUPercent:  95,
			MemPercent:  80,
			DiskPercent: 70,
		},
		"node2": {
			NodeID:      "node2",
			Online:      false,
		},
		"node3": {
			NodeID:      "node3",
			Online:      true,
			CPUPercent:  50,
			MemPercent:  60,
			DiskPercent: 95,
		},
	}
	monitor.mu.Unlock()
	
	issues := monitor.identifyTopIssues()
	
	if len(issues) == 0 {
		t.Error("expected issues, got none")
	}
	
	t.Logf("Top issues: %v", issues)
}

func TestHandlerHealth(t *testing.T) {
	metricStore := NewMockMetricStore()
	alertStore := NewMockAlertStore()
	config := DefaultConfig()
	
	monitor := NewMonitor(metricStore, alertStore, config)
	handler := NewHandler(monitor)
	
	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitor/health", nil)
	w := httptest.NewRecorder()
	
	handler.handleHealth(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	
	var resp APIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	
	if !resp.Success {
		t.Errorf("expected success=true, got false: %s", resp.Error)
	}
}

func TestHandlerMetrics(t *testing.T) {
	metricStore := NewMockMetricStore()
	alertStore := NewMockAlertStore()
	config := DefaultConfig()
	
	monitor := NewMonitor(metricStore, alertStore, config)
	handler := NewHandler(monitor)
	
	// 测试缺少参数
	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitor/metrics", nil)
	w := httptest.NewRecorder()
	
	handler.handleMetrics(w, req)
	
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
	
	// 测试正常查询
	req = httptest.NewRequest(http.MethodGet, "/api/v1/monitor/metrics?name=cpu_usage", nil)
	w = httptest.NewRecorder()
	
	handler.handleMetrics(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerRules(t *testing.T) {
	metricStore := NewMockMetricStore()
	alertStore := NewMockAlertStore()
	config := DefaultConfig()
	
	monitor := NewMonitor(metricStore, alertStore, config)
	handler := NewHandler(monitor)
	
	// 创建规则
	ruleJSON := `{
		"id": "rule1",
		"name": "High CPU",
		"metric": "cpu_usage",
		"threshold": 90,
		"severity": "warning"
	}`
	
	req := httptest.NewRequest(http.MethodPost, "/api/v1/monitor/rules", strings.NewReader(ruleJSON))
	w := httptest.NewRecorder()
	
	handler.handleRules(w, req)
	
	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
	
	var resp APIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	
	if !resp.Success {
		t.Errorf("expected success=true, got false: %s", resp.Error)
	}
	
	// 获取规则列表
	req = httptest.NewRequest(http.MethodGet, "/api/v1/monitor/rules", nil)
	w = httptest.NewRecorder()
	
	handler.handleRules(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	
	// 删除规则
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/monitor/rules?id=rule1", nil)
	w = httptest.NewRecorder()
	
	handler.handleRules(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerDashboard(t *testing.T) {
	metricStore := NewMockMetricStore()
	alertStore := NewMockAlertStore()
	config := DefaultConfig()
	
	monitor := NewMonitor(metricStore, alertStore, config)
	handler := NewHandler(monitor)
	
	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitor/dashboard", nil)
	w := httptest.NewRecorder()
	
	handler.handleDashboard(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	
	var resp APIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	
	if !resp.Success {
		t.Errorf("expected success=true, got false: %s", resp.Error)
	}
}

func TestHandlerMethodNotAllowed(t *testing.T) {
	metricStore := NewMockMetricStore()
	alertStore := NewMockAlertStore()
	config := DefaultConfig()
	
	monitor := NewMonitor(metricStore, alertStore, config)
	handler := NewHandler(monitor)
	
	// 测试POST方法到health端点
	req := httptest.NewRequest(http.MethodPost, "/api/v1/monitor/health", nil)
	w := httptest.NewRecorder()
	
	handler.handleHealth(w, req)
	
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestNodeStatus(t *testing.T) {
	metricStore := NewMockMetricStore()
	alertStore := NewMockAlertStore()
	config := DefaultConfig()
	
	monitor := NewMonitor(metricStore, alertStore, config)
	
	// 添加多个指标更新节点状态
	points := []MetricPoint{
		{NodeID: "node1", Name: "cpu_usage", Value: 75},
		{NodeID: "node1", Name: "memory_usage", Value: 60},
		{NodeID: "node1", Name: "disk_usage", Value: 50},
		{NodeID: "node2", Name: "cpu_usage", Value: 85},
	}
	
	for _, p := range points {
		monitor.updateNodeStatus(p)
	}
	
	monitor.mu.RLock()
	defer monitor.mu.RUnlock()
	
	node1, exists := monitor.nodeStatus["node1"]
	if !exists {
		t.Fatal("node1 not found")
	}
	
	if node1.CPUPercent != 75 {
		t.Errorf("expected CPU 75, got %f", node1.CPUPercent)
	}
	if node1.MemPercent != 60 {
		t.Errorf("expected memory 60, got %f", node1.MemPercent)
	}
	if node1.DiskPercent != 50 {
		t.Errorf("expected disk 50, got %f", node1.DiskPercent)
	}
	
	node2, exists := monitor.nodeStatus["node2"]
	if !exists {
		t.Fatal("node2 not found")
	}
	
	if node2.CPUPercent != 85 {
		t.Errorf("expected CPU 85, got %f", node2.CPUPercent)
	}
}

func TestGetNodeStatusMap(t *testing.T) {
	metricStore := NewMockMetricStore()
	alertStore := NewMockAlertStore()
	config := DefaultConfig()
	
	monitor := NewMonitor(metricStore, alertStore, config)
	
	monitor.mu.Lock()
	monitor.nodeStatus = map[string]*NodeStatus{
		"node1": {NodeID: "node1", Online: true},
		"node2": {NodeID: "node2", Online: false},
	}
	monitor.mu.Unlock()
	
	statusMap := monitor.getNodeStatusMap()
	
	if len(statusMap) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(statusMap))
	}
	
	if _, exists := statusMap["node1"]; !exists {
		t.Error("node1 not found in status map")
	}
	if _, exists := statusMap["node2"]; !exists {
		t.Error("node2 not found in status map")
	}
}
