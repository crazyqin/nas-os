package smbaudit

import (
	"testing"
	"time"
)

func TestNewSMBAuditLogger(t *testing.T) {
	logger := NewSMBAuditLogger(nil)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}

	config := logger.GetConfig()
	if !config.Enabled {
		t.Error("expected logger to be enabled by default")
	}
}

func TestLogEntry(t *testing.T) {
	logger := NewSMBAuditLogger(nil)

	entry := &AuditEntry{
		ID:        "entry1",
		UserID:    "user1",
		Username:  "testuser",
		ClientIP:  "192.168.1.100",
		ShareName: "public",
		FilePath:  "/data/public/file.txt",
		Action:    ActionWrite,
		Result:    ResultSuccess,
	}

	logger.Log(entry)

	// 查询
	filter := &AuditFilter{Username: "testuser"}
	entries := logger.Query(filter, 10, 0)
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

func TestLogDeleteAction(t *testing.T) {
	logger := NewSMBAuditLogger(nil)

	entry := &AuditEntry{
		ID:        "entry1",
		UserID:    "user1",
		Username:  "testuser",
		ShareName: "public",
		FilePath:  "/data/public/file.txt",
		Action:    ActionDelete,
		Result:    ResultSuccess,
	}

	logger.Log(entry)

	// 检查告警
	alerts := logger.GetAlerts(10)
	if len(alerts) == 0 {
		t.Error("expected alert for delete action")
	}
}

func TestQueryWithFilter(t *testing.T) {
	logger := NewSMBAuditLogger(nil)

	// 记录多个条目
	logger.Log(&AuditEntry{
		ID:        "entry1",
		UserID:    "user1",
		Username:  "user1",
		ShareName: "public",
		FilePath:  "/file1.txt",
		Action:    ActionWrite,
		Result:    ResultSuccess,
	})

	logger.Log(&AuditEntry{
		ID:        "entry2",
		UserID:    "user2",
		Username:  "user2",
		ShareName: "private",
		FilePath:  "/file2.txt",
		Action:    ActionRead,
		Result:    ResultSuccess,
	})

	// 按用户过滤
	filter := &AuditFilter{Username: "user1"}
	entries := logger.Query(filter, 10, 0)
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Username != "user1" {
		t.Errorf("expected user1, got %q", entries[0].Username)
	}
}

func TestGetStats(t *testing.T) {
	logger := NewSMBAuditLogger(nil)

	logger.Log(&AuditEntry{
		ID:        "entry1",
		UserID:    "user1",
		Username:  "user1",
		ShareName: "public",
		Action:    ActionWrite,
		Result:    ResultSuccess,
	})

	logger.Log(&AuditEntry{
		ID:        "entry2",
		UserID:    "user2",
		Username:  "user2",
		ShareName: "public",
		Action:    ActionDelete,
		Result:    ResultSuccess,
	})

	stats := logger.GetStats()
	totalEntries := stats["total_entries"].(int)
	if totalEntries != 2 {
		t.Errorf("expected 2 entries, got %d", totalEntries)
	}
}

func TestClear(t *testing.T) {
	logger := NewSMBAuditLogger(nil)

	logger.Log(&AuditEntry{
		ID:        "entry1",
		UserID:    "user1",
		Username:  "user1",
		ShareName: "public",
		Action:    ActionWrite,
		Result:    ResultSuccess,
	})

	logger.Clear()

	stats := logger.GetStats()
	totalEntries := stats["total_entries"].(int)
	if totalEntries != 0 {
		t.Errorf("expected 0 entries after clear, got %d", totalEntries)
	}
}

func TestAuditFilterMatch(t *testing.T) {
	entry := &AuditEntry{
		UserID:    "user1",
		Username:  "testuser",
		ClientIP:  "192.168.1.100",
		ShareName: "public",
		Action:    ActionWrite,
		Result:    ResultSuccess,
		Severity:  SeverityInfo,
		Timestamp: time.Now(),
	}

	// 测试空过滤器
	filter := &AuditFilter{}
	if !filter.Match(entry) {
		t.Error("expected empty filter to match")
	}

	// 测试用户名过滤
	filter = &AuditFilter{Username: "testuser"}
	if !filter.Match(entry) {
		t.Error("expected username filter to match")
	}

	filter = &AuditFilter{Username: "otheruser"}
	if filter.Match(entry) {
		t.Error("expected username filter not to match")
	}

	// 测试动作过滤
	action := ActionWrite
	filter = &AuditFilter{Action: &action}
	if !filter.Match(entry) {
		t.Error("expected action filter to match")
	}

	action = ActionDelete
	filter = &AuditFilter{Action: &action}
	if filter.Match(entry) {
		t.Error("expected action filter not to match")
	}
}
