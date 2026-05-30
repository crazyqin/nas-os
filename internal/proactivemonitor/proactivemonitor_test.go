package proactivemonitor

import (
	"testing"
	"time"
)

func TestCreateRule(t *testing.T) {
	m := NewManager("/tmp/test_monitor.json")
	m.Initialize()

	req := CreateRuleRequest{
		Name:     "测试规则",
		Category: "disk",
		Metric:   "disk_usage",
		Condition: Condition{Operator: "gte", Value: 80},
		Severity: 7,
		Duration: 300,
	}

	rule, err := m.CreateRule(req)
	if err != nil {
		t.Fatalf("CreateRule failed: %v", err)
	}
	if rule.Name != "测试规则" {
		t.Errorf("Expected name '测试规则', got '%s'", rule.Name)
	}
}

func TestReportMetric(t *testing.T) {
	m := NewManager("/tmp/test_monitor.json")
	m.Initialize()

	// 创建规则
	m.CreateRule(CreateRuleRequest{
		Name:     "CPU警告",
		Category: "cpu",
		Metric:   "cpu_usage",
		Condition: Condition{Operator: "gte", Value: 80},
		Severity: 7,
	})

	// 上报指标触发告警
	m.ReportMetric(Metric{
		Name:      "cpu_usage",
		Value:     90,
		Unit:      "%",
		Timestamp: time.Now(),
	})

	alerts := m.ListAlerts("active", "")
	if len(alerts) == 0 {
		t.Error("Expected at least one active alert")
	}
}

func TestAcknowledgeAlert(t *testing.T) {
	m := NewManager("/tmp/test_monitor.json")
	m.Initialize()

	m.CreateRule(CreateRuleRequest{
		Name:     "测试",
		Category: "cpu",
		Metric:   "cpu_usage",
		Condition: Condition{Operator: "gte", Value: 50},
		Severity: 5,
	})

	m.ReportMetric(Metric{Name: "cpu_usage", Value: 60, Timestamp: time.Now()})

	alerts := m.ListAlerts("active", "")
	if len(alerts) == 0 {
		t.Fatal("No alerts found")
	}

	err := m.AcknowledgeAlert(AcknowledgeAlertRequest{
		AlertID: alerts[0].ID,
		User:    "admin",
	})
	if err != nil {
		t.Fatalf("AcknowledgeAlert failed: %v", err)
	}

	ackAlerts := m.ListAlerts("acknowledged", "")
	if len(ackAlerts) == 0 {
		t.Error("Expected acknowledged alert")
	}
}

func TestResolveAlert(t *testing.T) {
	m := NewManager("/tmp/test_monitor.json")
	m.Initialize()

	m.CreateRule(CreateRuleRequest{
		Name:     "测试",
		Category: "cpu",
		Metric:   "cpu_usage",
		Condition: Condition{Operator: "gte", Value: 50},
		Severity: 5,
	})

	m.ReportMetric(Metric{Name: "cpu_usage", Value: 60, Timestamp: time.Now()})

	alerts := m.ListAlerts("active", "")
	if len(alerts) == 0 {
		t.Fatal("No alerts found")
	}

	err := m.ResolveAlert(alerts[0].ID, "admin")
	if err != nil {
		t.Fatalf("ResolveAlert failed: %v", err)
	}
}

func TestHealthCheck(t *testing.T) {
	m := NewManager("/tmp/test_monitor.json")
	m.Initialize()

	check := HealthCheck{
		ID:       "web",
		Name:     "Web服务",
		Type:     "http",
		Target:   "http://localhost:8080",
		Interval: 60,
	}

	err := m.AddHealthCheck(check)
	if err != nil {
		t.Fatalf("AddHealthCheck failed: %v", err)
	}

	m.UpdateHealthCheckStatus("web", "healthy", 50, "OK")

	checks := m.GetHealthChecks()
	if len(checks) != 1 {
		t.Errorf("Expected 1 health check, got %d", len(checks))
	}
	if checks[0].Status != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", checks[0].Status)
	}
}

func TestGetAlertStats(t *testing.T) {
	m := NewManager("/tmp/test_monitor.json")
	m.Initialize()

	m.CreateRule(CreateRuleRequest{
		Name:     "测试",
		Category: "cpu",
		Metric:   "cpu_usage",
		Condition: Condition{Operator: "gte", Value: 50},
		Severity: 5,
	})

	m.ReportMetric(Metric{Name: "cpu_usage", Value: 60, Timestamp: time.Now()})

	stats := m.GetAlertStats()
	if stats["active"] == 0 {
		t.Error("Expected active alerts in stats")
	}
}

func TestListRules(t *testing.T) {
	m := NewManager("/tmp/test_monitor.json")
	m.Initialize()

	rules := m.ListRules("")
	if len(rules) < 7 {
		t.Errorf("Expected at least 7 default rules, got %d", len(rules))
	}
}
