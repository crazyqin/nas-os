package nasopsintel

import (
	"context"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.events == nil {
		t.Error("events slice not initialized")
	}
	if m.incidents == nil {
		t.Error("incidents map not initialized")
	}
}

func TestIngestEvent(t *testing.T) {
	m := NewManager()
	event := OpsEvent{
		Source:      SourceStorage,
		Severity:    SeverityWarning,
		Title:       "磁盘空间不足",
		Description: "磁盘 /dev/sda 使用率超过 90%",
		Service:     "storage",
	}

	m.IngestEvent(event)

	events := m.ListEvents(10, "", "")
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Title != "磁盘空间不足" {
		t.Errorf("expected title 磁盘空间不足, got %s", events[0].Title)
	}
}

func TestIngestEventAutoID(t *testing.T) {
	m := NewManager()
	event := OpsEvent{
		Source:   SourceNetwork,
		Severity: SeverityInfo,
		Title:    "网络连接正常",
	}

	m.IngestEvent(event)

	events := m.ListEvents(10, "", "")
	if events[0].ID == "" {
		t.Error("expected auto-generated ID")
	}
}

func TestCorrelateEvents(t *testing.T) {
	m := NewManager()

	// 发送同源的警告事件
	m.IngestEvent(OpsEvent{
		Source:   SourceStorage,
		Severity: SeverityWarning,
		Title:    "磁盘空间警告",
		Service:  "storage",
	})

	m.IngestEvent(OpsEvent{
		Source:   SourceStorage,
		Severity: SeverityError,
		Title:    "磁盘空间严重不足",
		Service:  "storage",
	})

	incidents := m.ListIncidents("")
	if len(incidents) != 1 {
		t.Fatalf("expected 1 correlated incident, got %d", len(incidents))
	}

	inc := incidents[0]
	if len(inc.Events) != 2 {
		t.Errorf("expected 2 events in incident, got %d", len(inc.Events))
	}
	if inc.Severity != SeverityError {
		t.Errorf("expected severity error, got %s", inc.Severity)
	}
}

func TestResolveIncident(t *testing.T) {
	m := NewManager()

	m.IngestEvent(OpsEvent{
		Source:   SourceStorage,
		Severity: SeverityError,
		Title:    "磁盘故障",
		Service:  "storage",
	})

	incidents := m.ListIncidents("")
	if len(incidents) == 0 {
		t.Fatal("expected at least 1 incident")
	}

	inc := incidents[0]
	err := m.ResolveIncident(inc.ID, "磁盘老化", "更换磁盘")
	if err != nil {
		t.Fatalf("ResolveIncident failed: %v", err)
	}

	resolved, _ := m.GetIncident(inc.ID)
	if resolved.Status != IncidentResolved {
		t.Errorf("expected status resolved, got %s", resolved.Status)
	}
	if resolved.RootCause != "磁盘老化" {
		t.Errorf("expected root cause 磁盘老化, got %s", resolved.RootCause)
	}
}

func TestResolveIncidentNotFound(t *testing.T) {
	m := NewManager()
	err := m.ResolveIncident("nonexistent", "cause", "fix")
	if err == nil {
		t.Error("expected error for nonexistent incident")
	}
}

func TestListEventsFilter(t *testing.T) {
	m := NewManager()

	m.IngestEvent(OpsEvent{Source: SourceStorage, Severity: SeverityWarning, Title: "Storage Warning"})
	m.IngestEvent(OpsEvent{Source: SourceNetwork, Severity: SeverityInfo, Title: "Network Info"})
	m.IngestEvent(OpsEvent{Source: SourceStorage, Severity: SeverityError, Title: "Storage Error"})

	// 按来源过滤
	storageEvents := m.ListEvents(100, SourceStorage, "")
	if len(storageEvents) != 2 {
		t.Errorf("expected 2 storage events, got %d", len(storageEvents))
	}

	// 按严重程度过滤
	warningEvents := m.ListEvents(100, "", SeverityWarning)
	if len(warningEvents) != 1 {
		t.Errorf("expected 1 warning event, got %d", len(warningEvents))
	}
}

func TestAnomalyDetection(t *testing.T) {
	m := NewManager()

	// 发送大量事件触发异常检测
	for i := 0; i < 60; i++ {
		m.IngestEvent(OpsEvent{
			Source:   SourceSystem,
			Severity: SeverityInfo,
			Title:    "Test Event",
		})
	}

	// 手动触发异常检测
	m.detectAnomalies()

	anomalies := m.ListAnomalies(10)
	if len(anomalies) == 0 {
		t.Error("expected at least 1 anomaly")
	}
}

func TestHealthScore(t *testing.T) {
	m := NewManager()

	// 初始健康评分
	health := m.GetHealthScore()
	if health == nil {
		t.Fatal("GetHealthScore returned nil")
	}

	// 发送事件后健康评分应降低
	m.IngestEvent(OpsEvent{
		Source:   SourceStorage,
		Severity: SeverityCritical,
		Title:    "严重故障",
	})

	m.calculateHealth()
	health = m.GetHealthScore()
	if health.Overall >= 100 {
		t.Error("health score should decrease after critical event")
	}
}

func TestAddRule(t *testing.T) {
	m := NewManager()
	rule := &Rule{
		ID:       "rule-1",
		Name:     "磁盘空间规则",
		Source:   SourceStorage,
		Severity: SeverityWarning,
		Enabled:  true,
	}

	if err := m.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	rules := m.ListRules()
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}
}

func TestAddRuleNoID(t *testing.T) {
	m := NewManager()
	rule := &Rule{Name: "No ID"}
	if err := m.AddRule(rule); err == nil {
		t.Error("expected error for rule without ID")
	}
}

func TestGetMetrics(t *testing.T) {
	m := NewManager()
	metrics := m.GetMetrics()
	if metrics == nil {
		t.Fatal("GetMetrics returned nil")
	}
	if metrics.Uptime <= 0 {
		t.Error("uptime should be positive")
	}
}

func TestStartStop(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	m.Stop()
}

func TestGetIncidentNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.GetIncident("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent incident")
	}
}
