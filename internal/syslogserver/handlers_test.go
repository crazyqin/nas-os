// Package syslogserver 测试
package syslogserver

import (
	"encoding/json"
	"testing"
	"time"
)

// TestNewManager 测试创建管理器.
func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected manager to be created")
	}
	if m.entries == nil {
		t.Fatal("expected entries map to be initialized")
	}
	if m.forwardTargets == nil {
		t.Fatal("expected forwardTargets map to be initialized")
	}
	if m.alertRules == nil {
		t.Fatal("expected alertRules map to be initialized")
	}
}

// TestCreateForwardTarget 测试创建转发目标.
func TestCreateForwardTarget(t *testing.T) {
	m := NewManager()

	req := CreateForwardTargetRequest{
		Name:     "test-target",
		Host:     "192.168.1.100",
		Port:     514,
		Protocol: "udp",
		Enabled:  true,
		Filter:   "daemon:warning",
	}

	target := m.CreateForwardTarget(req)
	if target == nil {
		t.Fatal("expected target to be created")
	}
	if target.Name != "test-target" {
		t.Errorf("expected name 'test-target', got %q", target.Name)
	}
	if target.Host != "192.168.1.100" {
		t.Errorf("expected host '192.168.1.100', got %q", target.Host)
	}
	if target.Port != 514 {
		t.Errorf("expected port 514, got %d", target.Port)
	}
	if target.Protocol != "udp" {
		t.Errorf("expected protocol 'udp', got %q", target.Protocol)
	}
	if !target.Enabled {
		t.Error("expected target to be enabled")
	}
}

// TestGetForwardTarget 测试获取转发目标.
func TestGetForwardTarget(t *testing.T) {
	m := NewManager()

	// 创建目标
	req := CreateForwardTargetRequest{
		Name:     "test-target",
		Host:     "192.168.1.100",
		Port:     514,
		Protocol: "tcp",
		Enabled:  true,
	}
	target := m.CreateForwardTarget(req)

	// 获取目标
	got, err := m.GetForwardTarget(target.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != target.ID {
		t.Errorf("expected ID %q, got %q", target.ID, got.ID)
	}
	if got.Name != "test-target" {
		t.Errorf("expected name 'test-target', got %q", got.Name)
	}
}

// TestGetForwardTargetNotFound 测试获取不存在的转发目标.
func TestGetForwardTargetNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetForwardTarget("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent target")
	}
}

// TestListForwardTargets 测试列出转发目标.
func TestListForwardTargets(t *testing.T) {
	m := NewManager()

	// 初始应该为空
	targets := m.ListForwardTargets()
	if len(targets) != 0 {
		t.Errorf("expected 0 targets, got %d", len(targets))
	}

	// 创建两个目标
	m.CreateForwardTarget(CreateForwardTargetRequest{
		Name: "target-1", Host: "10.0.0.1", Port: 514, Protocol: "udp", Enabled: true,
	})
	m.CreateForwardTarget(CreateForwardTargetRequest{
		Name: "target-2", Host: "10.0.0.2", Port: 514, Protocol: "tcp", Enabled: false,
	})

	targets = m.ListForwardTargets()
	if len(targets) != 2 {
		t.Errorf("expected 2 targets, got %d", len(targets))
	}
}

// TestUpdateForwardTarget 测试更新转发目标.
func TestUpdateForwardTarget(t *testing.T) {
	m := NewManager()

	// 创建目标
	target := m.CreateForwardTarget(CreateForwardTargetRequest{
		Name: "original", Host: "10.0.0.1", Port: 514, Protocol: "udp", Enabled: true,
	})

	// 更新目标
	newName := "updated"
	newPort := 1514
	updated, err := m.UpdateForwardTarget(target.ID, UpdateForwardTargetRequest{
		Name: &newName,
		Port: &newPort,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "updated" {
		t.Errorf("expected name 'updated', got %q", updated.Name)
	}
	if updated.Port != 1514 {
		t.Errorf("expected port 1514, got %d", updated.Port)
	}
}

// TestDeleteForwardTarget 测试删除转发目标.
func TestDeleteForwardTarget(t *testing.T) {
	m := NewManager()

	// 创建目标
	target := m.CreateForwardTarget(CreateForwardTargetRequest{
		Name: "to-delete", Host: "10.0.0.1", Port: 514, Protocol: "udp", Enabled: true,
	})

	// 删除目标
	err := m.DeleteForwardTarget(target.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证已删除
	_, err = m.GetForwardTarget(target.ID)
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

// TestDeleteForwardTargetNotFound 测试删除不存在的目标.
func TestDeleteForwardTargetNotFound(t *testing.T) {
	m := NewManager()

	err := m.DeleteForwardTarget("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent target")
	}
}

// TestCreateAlertRule 测试创建告警规则.
func TestCreateAlertRule(t *testing.T) {
	m := NewManager()

	req := CreateAlertRuleRequest{
		Name:       "error-alert",
		Enabled:    true,
		Type:       "keyword",
		Keyword:    "error",
		NotifyType: "log",
	}

	rule := m.CreateAlertRule(req)
	if rule == nil {
		t.Fatal("expected rule to be created")
	}
	if rule.Name != "error-alert" {
		t.Errorf("expected name 'error-alert', got %q", rule.Name)
	}
	if rule.Type != "keyword" {
		t.Errorf("expected type 'keyword', got %q", rule.Type)
	}
	if rule.Keyword != "error" {
		t.Errorf("expected keyword 'error', got %q", rule.Keyword)
	}
}

// TestGetAlertRule 测试获取告警规则.
func TestGetAlertRule(t *testing.T) {
	m := NewManager()

	rule := m.CreateAlertRule(CreateAlertRuleRequest{
		Name: "test-rule", Enabled: true, Type: "keyword", Keyword: "test", NotifyType: "log",
	})

	got, err := m.GetAlertRule(rule.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != rule.ID {
		t.Errorf("expected ID %q, got %q", rule.ID, got.ID)
	}
}

// TestListAlertRules 测试列出告警规则.
func TestListAlertRules(t *testing.T) {
	m := NewManager()

	// 初始应该为空
	rules := m.ListAlertRules()
	if len(rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(rules))
	}

	// 创建规则
	m.CreateAlertRule(CreateAlertRuleRequest{
		Name: "rule-1", Enabled: true, Type: "keyword", Keyword: "error", NotifyType: "log",
	})
	m.CreateAlertRule(CreateAlertRuleRequest{
		Name: "rule-2", Enabled: false, Type: "frequency", Frequency: 100, NotifyType: "webhook",
	})

	rules = m.ListAlertRules()
	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rules))
	}
}

// TestUpdateAlertRule 测试更新告警规则.
func TestUpdateAlertRule(t *testing.T) {
	m := NewManager()

	rule := m.CreateAlertRule(CreateAlertRuleRequest{
		Name: "original", Enabled: true, Type: "keyword", Keyword: "error", NotifyType: "log",
	})

	newName := "updated"
	newKeyword := "critical"
	updated, err := m.UpdateAlertRule(rule.ID, UpdateAlertRuleRequest{
		Name:    &newName,
		Keyword: &newKeyword,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "updated" {
		t.Errorf("expected name 'updated', got %q", updated.Name)
	}
	if updated.Keyword != "critical" {
		t.Errorf("expected keyword 'critical', got %q", updated.Keyword)
	}
}

// TestDeleteAlertRule 测试删除告警规则.
func TestDeleteAlertRule(t *testing.T) {
	m := NewManager()

	rule := m.CreateAlertRule(CreateAlertRuleRequest{
		Name: "to-delete", Enabled: true, Type: "keyword", Keyword: "error", NotifyType: "log",
	})

	err := m.DeleteAlertRule(rule.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = m.GetAlertRule(rule.ID)
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

// TestSearchLogs 测试日志搜索.
func TestSearchLogs(t *testing.T) {
	m := NewManager()

	// 添加测试日志
	m.entries = append(m.entries, &SyslogEntry{
		ID:        "1",
		Hostname:  "server1",
		AppName:   "nginx",
		Message:   "GET /api/test 200",
		Timestamp: time.Now(),
	})
	m.entries = append(m.entries, &SyslogEntry{
		ID:        "2",
		Hostname:  "server2",
		AppName:   "mysql",
		Message:   "Connection timeout",
		Timestamp: time.Now(),
	})

	// 搜索所有
	req := SearchRequest{Page: 1, PageSize: 10}
	result := m.SearchLogs(req)
	if result.Total != 2 {
		t.Errorf("expected 2 results, got %d", result.Total)
	}

	// 按主机名搜索
	req = SearchRequest{Hostname: "server1", Page: 1, PageSize: 10}
	result = m.SearchLogs(req)
	if result.Total != 1 {
		t.Errorf("expected 1 result, got %d", result.Total)
	}
	if result.Entries[0].Hostname != "server1" {
		t.Errorf("expected hostname 'server1', got %q", result.Entries[0].Hostname)
	}
}

// TestGetDashboardStats 测试仪表板统计.
func TestGetDashboardStats(t *testing.T) {
	m := NewManager()

	// 添加测试数据
	m.entries = append(m.entries, &SyslogEntry{
		ID:        "1",
		Hostname:  "server1",
		AppName:   "nginx",
		Message:   "test",
		Timestamp: time.Now(),
	})

	stats := m.GetDashboardStats()
	if stats.TotalEntries != 1 {
		t.Errorf("expected total_entries 1, got %d", stats.TotalEntries)
	}
}

// TestExportLogsCSV 测试 CSV 导出.
func TestExportLogsCSV(t *testing.T) {
	m := NewManager()

	m.entries = append(m.entries, &SyslogEntry{
		ID:        "1",
		Hostname:  "server1",
		AppName:   "nginx",
		Message:   "test message",
		Timestamp: time.Now(),
	})

	req := ExportRequest{Format: "csv", Limit: 10}
	data, err := m.ExportLogs(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected non-empty CSV data")
	}

	// 验证包含表头
	csvStr := string(data)
	if !contains(csvStr, "id") || !contains(csvStr, "hostname") {
		t.Error("expected CSV to contain headers")
	}
}

// TestExportLogsJSON 测试 JSON 导出.
func TestExportLogsJSON(t *testing.T) {
	m := NewManager()

	m.entries = append(m.entries, &SyslogEntry{
		ID:        "1",
		Hostname:  "server1",
		AppName:   "nginx",
		Message:   "test message",
		Timestamp: time.Now(),
	})

	req := ExportRequest{Format: "json", Limit: 10}
	data, err := m.ExportLogs(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected non-empty JSON data")
	}

	// 验证是有效的 JSON
	var entries []SyslogEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("expected valid JSON, got error: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

// TestRegisterUnregisterWSClient 测试 WebSocket 客户端管理.
func TestRegisterUnregisterWSClient(t *testing.T) {
	m := NewManager()

	client := &WSClient{
		ID:   "test-client",
		Send: make(chan []byte, 10),
	}

	// 注册
	m.RegisterWSClient(client)
	if m.GetWSClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", m.GetWSClientCount())
	}

	// 注销
	m.UnregisterWSClient("test-client")
	if m.GetWSClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", m.GetWSClientCount())
	}
}

// TestParseSyslogMessageRFC3164 测试 RFC 3164 解析.
func TestParseSyslogMessageRFC3164(t *testing.T) {
	m := NewManager()

	// RFC 3164 格式
	raw := `<134>May 28 12:34:56 myhost sshd[12345]: Accepted publickey for user`
	entry := m.parseSyslogMessage(raw, "192.168.1.1", "tcp")

	if entry.Priority != 134 {
		t.Errorf("expected priority 134, got %d", entry.Priority)
	}
	if entry.Facility != FacilityLocal0 {
		t.Errorf("expected facility local0, got %d", entry.Facility)
	}
	if entry.Severity != SeverityInformational {
		t.Errorf("expected severity informational, got %d", entry.Severity)
	}
	if entry.Hostname != "myhost" {
		t.Errorf("expected hostname 'myhost', got %q", entry.Hostname)
	}
	if entry.AppName != "sshd" {
		t.Errorf("expected app_name 'sshd', got %q", entry.AppName)
	}
	if entry.SourceIP != "192.168.1.1" {
		t.Errorf("expected source_ip '192.168.1.1', got %q", entry.SourceIP)
	}
	if entry.Protocol != "tcp" {
		t.Errorf("expected protocol 'tcp', got %q", entry.Protocol)
	}
}

// TestParseSyslogMessageSimple 测试简单格式解析.
func TestParseSyslogMessageSimple(t *testing.T) {
	m := NewManager()

	raw := `<13>this is a simple message`
	entry := m.parseSyslogMessage(raw, "10.0.0.1", "udp")

	if entry.Priority != 13 {
		t.Errorf("expected priority 13, got %d", entry.Priority)
	}
	if entry.Facility != FacilityUser {
		t.Errorf("expected facility user, got %d", entry.Facility)
	}
	// priority=13 => facility=1 (user), severity=5 (notice)
	if entry.Severity != SeverityNotice {
		t.Errorf("expected severity notice, got %d", entry.Severity)
	}
	if entry.Message != "this is a simple message" {
		t.Errorf("expected message 'this is a simple message', got %q", entry.Message)
	}
}

// TestParseSyslogMessageUnstructured 测试无法解析的消息.
func TestParseSyslogMessageUnstructured(t *testing.T) {
	m := NewManager()

	raw := `this is an unstructured message`
	entry := m.parseSyslogMessage(raw, "172.16.0.1", "udp")

	if entry.Priority != 134 {
		t.Errorf("expected priority 134, got %d", entry.Priority)
	}
	if entry.Message != "this is an unstructured message" {
		t.Errorf("expected message 'this is an unstructured message', got %q", entry.Message)
	}
	if entry.Hostname != "172.16.0.1" {
		t.Errorf("expected hostname '172.16.0.1', got %q", entry.Hostname)
	}
}

// TestListAlertEvents 测试列出告警事件.
func TestListAlertEvents(t *testing.T) {
	m := NewManager()

	// 添加事件
	m.alertEvents = append(m.alertEvents, &AlertEvent{
		ID:          "1",
		RuleID:      "rule-1",
		RuleName:    "test-rule",
		Message:     "test alert",
		TriggeredAt: time.Now(),
	})
	m.alertEvents = append(m.alertEvents, &AlertEvent{
		ID:          "2",
		RuleID:      "rule-1",
		RuleName:    "test-rule",
		Message:     "test alert 2",
		TriggeredAt: time.Now(),
	})

	// 默认限制
	events := m.ListAlertEvents(0)
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}

	// 指定限制
	events = m.ListAlertEvents(1)
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}
}

// contains 检查字符串是否包含子串.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

// searchString 在字符串中搜索子串.
func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
