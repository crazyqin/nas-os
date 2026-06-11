package compliance

import (
	"os"
	"testing"
	"time"
)

func TestNewAuditLogger(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	logger, err := NewAuditLogger(tmpDir, 1000, 30)
	if err != nil {
		t.Fatalf("NewAuditLogger failed: %v", err)
	}
	defer logger.Close()

	if logger == nil {
		t.Fatal("logger should not be nil")
	}

	if logger.logDir != tmpDir {
		t.Errorf("expected log dir %s, got %s", tmpDir, logger.logDir)
	}

	if logger.maxEvents != 1000 {
		t.Errorf("expected maxEvents 1000, got %d", logger.maxEvents)
	}
}

func TestAuditLogEvent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	logger, err := NewAuditLogger(tmpDir, 1000, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	event := &AuditEvent{
		EventType: EventLogin,
		Severity:  SeverityInfo,
		Actor: AuditActor{
			ID:   "user-1",
			Name: "admin",
			Type: "user",
		},
		Resource: AuditResource{
			ID:   "system",
			Type: "system",
		},
		Action: "login",
		Result: AuditResult{
			Status:  "success",
			Message: "登录成功",
		},
		IPAddress: "192.168.1.100",
	}

	err = logger.LogEvent(event)
	if err != nil {
		t.Fatalf("LogEvent failed: %v", err)
	}

	// 验证事件已记录
	if len(logger.events) != 1 {
		t.Errorf("expected 1 event, got %d", len(logger.events))
	}

	if logger.events[0].ID == "" {
		t.Error("event ID should be generated")
	}

	if logger.events[0].Actor.ID != "user-1" {
		t.Errorf("expected actor ID 'user-1', got '%s'", logger.events[0].Actor.ID)
	}
}

func TestAuditQueryEvents(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	logger, err := NewAuditLogger(tmpDir, 1000, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	// 记录多个事件
	events := []*AuditEvent{
		{
			EventType: EventLogin,
			Severity:  SeverityInfo,
			Actor:     AuditActor{ID: "user-1", Name: "admin"},
			Result:    AuditResult{Status: "success"},
			IPAddress: "192.168.1.1",
		},
		{
			EventType: EventAccess,
			Severity:  SeverityMedium,
			Actor:     AuditActor{ID: "user-2", Name: "user"},
			Result:    AuditResult{Status: "success"},
			IPAddress: "192.168.1.2",
		},
		{
			EventType: EventLogin,
			Severity:  SeverityHigh,
			Actor:     AuditActor{ID: "user-3", Name: "unknown"},
			Result:    AuditResult{Status: "failure"},
			IPAddress: "10.0.0.1",
		},
	}

	for _, event := range events {
		if err := logger.LogEvent(event); err != nil {
			t.Fatalf("LogEvent failed: %v", err)
		}
	}

	// 查询所有事件
	result, err := logger.QueryEvents(&AuditQuery{})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}

	if result.Total != 3 {
		t.Errorf("expected 3 events, got %d", result.Total)
	}

	// 按事件类型查询
	result, err = logger.QueryEvents(&AuditQuery{
		EventTypes: []AuditEventType{EventLogin},
	})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("expected 2 login events, got %d", result.Total)
	}

	// 按执行者查询
	result, err = logger.QueryEvents(&AuditQuery{
		ActorID: "user-1",
	})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("expected 1 event for user-1, got %d", result.Total)
	}

	// 按状态查询
	result, err = logger.QueryEvents(&AuditQuery{
		Status: "failure",
	})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("expected 1 failure event, got %d", result.Total)
	}
}

func TestAuditQueryPagination(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	logger, err := NewAuditLogger(tmpDir, 1000, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	// 记录 10 个事件
	for i := 0; i < 10; i++ {
		event := &AuditEvent{
			EventType: EventAccess,
			Severity:  SeverityInfo,
			Actor:     AuditActor{ID: "user-1"},
			Result:    AuditResult{Status: "success"},
		}
		if err := logger.LogEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	// 测试分页
	result, err := logger.QueryEvents(&AuditQuery{
		Limit:  3,
		Offset: 0,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Events) != 3 {
		t.Errorf("expected 3 events, got %d", len(result.Events))
	}

	if !result.HasMore {
		t.Error("should have more results")
	}

	// 第二页
	result, err = logger.QueryEvents(&AuditQuery{
		Limit:  3,
		Offset: 3,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Events) != 3 {
		t.Errorf("expected 3 events, got %d", len(result.Events))
	}
}

func TestAuditUserProfiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	logger, err := NewAuditLogger(tmpDir, 1000, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	// 记录用户事件
	events := []*AuditEvent{
		{
			EventType: EventLogin,
			Actor:     AuditActor{ID: "user-1", Name: "admin"},
			IPAddress: "192.168.1.1",
			Result:    AuditResult{Status: "success"},
		},
		{
			EventType: EventAccess,
			Actor:     AuditActor{ID: "user-1", Name: "admin"},
			IPAddress: "192.168.1.1",
			Result:    AuditResult{Status: "success"},
		},
		{
			EventType: EventModify,
			Actor:     AuditActor{ID: "user-1", Name: "admin"},
			IPAddress: "192.168.1.2",
			Result:    AuditResult{Status: "success"},
		},
	}

	for _, event := range events {
		if err := logger.LogEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	// 获取用户画像
	profile, err := logger.GetUserProfile("user-1")
	if err != nil {
		t.Fatalf("GetUserProfile failed: %v", err)
	}

	if profile.UserID != "user-1" {
		t.Errorf("expected user ID 'user-1', got '%s'", profile.UserID)
	}

	if profile.TotalEvents != 3 {
		t.Errorf("expected 3 events, got %d", profile.TotalEvents)
	}

	if profile.EventCounts[string(EventLogin)] != 1 {
		t.Errorf("expected 1 login event, got %d", profile.EventCounts[string(EventLogin)])
	}

	if len(profile.IPAddressHistory) != 2 {
		t.Errorf("expected 2 IP addresses, got %d", len(profile.IPAddressHistory))
	}
}

func TestAuditListUserProfiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	logger, err := NewAuditLogger(tmpDir, 1000, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	// 记录不同用户的事件
	for i := 0; i < 5; i++ {
		event := &AuditEvent{
			EventType: EventLogin,
			Actor:     AuditActor{ID: "user-" + string(rune('A'+i))},
			Result:    AuditResult{Status: "success"},
		}
		logger.LogEvent(event)
	}

	profiles := logger.ListUserProfiles()
	if len(profiles) != 5 {
		t.Errorf("expected 5 profiles, got %d", len(profiles))
	}
}

func TestAuditDetectBruteForce(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	logger, err := NewAuditLogger(tmpDir, 1000, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	// 模拟暴力破解
	for i := 0; i < 6; i++ {
		event := &AuditEvent{
			EventType: EventLogin,
			Severity:  SeverityHigh,
			Actor:     AuditActor{ID: "unknown", Name: "attacker"},
			Result:    AuditResult{Status: "failure"},
			IPAddress: "10.0.0.1",
		}
		logger.LogEvent(event)
	}

	anomalies := logger.DetectAnomalies()

	if len(anomalies) == 0 {
		t.Error("should detect brute force anomaly")
	}

	found := false
	for _, anomaly := range anomalies {
		if anomaly.AnomalyType == "brute_force" {
			found = true
			if anomaly.Severity != SeverityHigh {
				t.Errorf("expected severity 'high', got '%s'", anomaly.Severity)
			}
		}
	}

	if !found {
		t.Error("should detect brute_force anomaly")
	}
}

func TestAuditDetectPrivilegeEscalation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	logger, err := NewAuditLogger(tmpDir, 1000, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	// 模拟权限提升
	event := &AuditEvent{
		EventType: EventPermission,
		Severity:  SeverityCritical,
		Actor:     AuditActor{ID: "user-1", Name: "user"},
		Action:    "grant",
		Result:    AuditResult{Status: "success"},
		Details: map[string]interface{}{
			"new_role": "admin",
		},
	}
	logger.LogEvent(event)

	anomalies := logger.DetectAnomalies()

	found := false
	for _, anomaly := range anomalies {
		if anomaly.AnomalyType == "privilege_escalation" {
			found = true
			if anomaly.Severity != SeverityCritical {
				t.Errorf("expected severity 'critical', got '%s'", anomaly.Severity)
			}
		}
	}

	if !found {
		t.Error("should detect privilege_escalation anomaly")
	}
}

func TestAuditGenerateReport(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	logger, err := NewAuditLogger(tmpDir, 1000, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	// 记录一些事件
	for i := 0; i < 10; i++ {
		event := &AuditEvent{
			EventType: EventLogin,
			Severity:  SeverityInfo,
			Actor:     AuditActor{ID: "user-1"},
			Result:    AuditResult{Status: "success"},
		}
		logger.LogEvent(event)
	}

	for i := 0; i < 3; i++ {
		event := &AuditEvent{
			EventType: EventSecurity,
			Severity:  SeverityHigh,
			Actor:     AuditActor{ID: "system"},
			Result:    AuditResult{Status: "failure"},
		}
		logger.LogEvent(event)
	}

	// 生成报告
	period := ReportPeriod{
		Start: time.Now().Add(-1 * time.Hour),
		End:   time.Now().Add(1 * time.Hour),
	}

	report := logger.GenerateAuditReport(period)

	if report == nil {
		t.Fatal("report should not be nil")
	}

	if report.ID == "" {
		t.Error("report ID should not be empty")
	}

	if report.Summary.TotalEvents != 13 {
		t.Errorf("expected 13 events, got %d", report.Summary.TotalEvents)
	}

	if report.Summary.EventsByType[string(EventLogin)] != 10 {
		t.Errorf("expected 10 login events, got %d", report.Summary.EventsByType[string(EventLogin)])
	}
}

func TestAuditExportJSONL(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	logger, err := NewAuditLogger(tmpDir, 1000, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	// 记录事件
	event := &AuditEvent{
		EventType: EventLogin,
		Actor:     AuditActor{ID: "user-1"},
		Result:    AuditResult{Status: "success"},
	}
	logger.LogEvent(event)

	// 导出 JSONL
	data, err := logger.ExportAuditLogs(&AuditQuery{}, "jsonl")
	if err != nil {
		t.Fatalf("ExportAuditLogs failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("exported data should not be empty")
	}

	// 验证是有效的 JSONL
	lines := splitLines(string(data))
	if len(lines) != 1 {
		t.Errorf("expected 1 line, got %d", len(lines))
	}
}

func TestAuditSeverityConstants(t *testing.T) {
	tests := []struct {
		severity AuditSeverity
		expected string
	}{
		{SeverityCritical, "critical"},
		{SeverityHigh, "high"},
		{SeverityMedium, "medium"},
		{SeverityLow, "low"},
		{SeverityInfo, "info"},
	}

	for _, tt := range tests {
		if string(tt.severity) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.severity))
		}
	}
}

func TestAuditEventTypeConstants(t *testing.T) {
	tests := []struct {
		eventType AuditEventType
		expected  string
	}{
		{EventLogin, "login"},
		{EventLogout, "logout"},
		{EventAccess, "access"},
		{EventModify, "modify"},
		{EventDelete, "delete"},
		{EventSecurity, "security"},
		{EventConfig, "config"},
	}

	for _, tt := range tests {
		if string(tt.eventType) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.eventType))
		}
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
