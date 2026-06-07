package networkaudit

import (
	"testing"
	"time"
)

func TestAuditLog(t *testing.T) {
	tmpDir := t.TempDir()
	logger := NewAuditLogger(tmpDir, 1000)

	logger.Log(&AuditEntry{
		Protocol: ProtocolSMB,
		Action:   ActionFileRead,
		User:     "admin",
		SourceIP: "192.168.1.100",
		Target:   "/share/docs",
		Resource: "test.pdf",
		Severity: SeverityLow,
		Status:   "SUCCESS",
	})

	logger.Log(&AuditEntry{
		Protocol: ProtocolNFS,
		Action:   ActionFileWrite,
		User:     "user1",
		SourceIP: "192.168.1.101",
		Target:   "/share/data",
		Resource: "data.csv",
		Severity: SeverityMedium,
		Status:   "SUCCESS",
	})

	entries, total := logger.Query(AuditFilter{PageSize: 10})
	if total != 2 {
		t.Fatalf("expected 2 entries, got %d", total)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries in page, got %d", len(entries))
	}
}

func TestAuditStats(t *testing.T) {
	tmpDir := t.TempDir()
	logger := NewAuditLogger(tmpDir, 1000)

	for i := 0; i < 5; i++ {
		logger.Log(&AuditEntry{
			Protocol: ProtocolSMB,
			Action:   ActionFileRead,
			User:     "admin",
			SourceIP: "192.168.1.100",
			Severity: SeverityLow,
		})
	}

	stats := logger.GetStats(1)
	if stats.TotalEntries != 5 {
		t.Fatalf("expected 5 entries, got %d", stats.TotalEntries)
	}
	if stats.ByProtocol[ProtocolSMB] != 5 {
		t.Fatal("protocol count mismatch")
	}
}

func TestAuditAlert(t *testing.T) {
	tmpDir := t.TempDir()
	logger := NewAuditLogger(tmpDir, 1000)

	alertTriggered := false
	logger.OnAlert(func(entry *AuditEntry) {
		alertTriggered = true
	})

	logger.AddAlertRule(&AlertRule{
		ID:       "test-rule",
		Name:     "Test Rule",
		Action:   ActionLogin,
		Severity: SeverityHigh,
		Enabled:  true,
	})

	logger.Log(&AuditEntry{
		Action:   ActionLogin,
		User:     "attacker",
		SourceIP: "10.0.0.1",
		Severity: SeverityHigh,
		Status:   "FAILED",
	})

	time.Sleep(100 * time.Millisecond)
	if !alertTriggered {
		t.Fatal("alert should have been triggered")
	}
}
