package apilevelmeter

import (
	"testing"
	"time"
)

func TestKeyManagement(t *testing.T) {
	m := NewManager()
	key := &APIKey{
		ID:    "key-001",
		Label: "Test Key",
		UserID: "user-001",
		KeyPrefix: "sk-test12",
		Scopes: []string{"read", "write"},
	}
	if err := m.RegisterKey(key); err != nil {
		t.Fatalf("RegisterKey failed: %v", err)
	}
	keys := m.ListKeys()
	if len(keys) != 1 {
		t.Errorf("expected 1 key, got %d", len(keys))
	}
	if err := m.DisableKey("key-001"); err != nil {
		t.Fatalf("DisableKey failed: %v", err)
	}
	if !key.Disabled {
		t.Error("expected key to be disabled")
	}
}

func TestUsageRecording(t *testing.T) {
	m := NewManager()
	m.RegisterKey(&APIKey{ID: "key-001", Label: "Test", UserID: "u1", KeyPrefix: "sk-test"})
	for i := 0; i < 100; i++ {
		rec := UsageRecord{
			KeyID:      "key-001",
			Endpoint:   "/api/v1/data",
			Method:     "GET",
			StatusCode: 200,
			ResponseTimeMs: int64(10 + i),
			BytesIn:    100,
			BytesOut:   1024,
			Timestamp:  time.Now(),
		}
		if err := m.RecordUsage(rec); err != nil {
			t.Fatalf("RecordUsage failed: %v", err)
		}
	}
	summary, err := m.GetSummary("key-001", 60)
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}
	if summary.TotalRequests != 100 {
		t.Errorf("expected 100 requests, got %d", summary.TotalRequests)
	}
	if summary.AvgResponseMs <= 0 {
		t.Error("expected positive avg response time")
	}
}

func TestQuotaCheck(t *testing.T) {
	m := NewManager()
	m.RegisterKey(&APIKey{ID: "key-001", Label: "Test", UserID: "u1", KeyPrefix: "sk-test"})
	quota := &Quota{
		KeyID:             "key-001",
		MaxRequestsPerMin: 10,
		CurrentReqPerMin:  15,
	}
	m.SetQuota(quota)
	result, err := m.CheckQuota("key-001")
	if err != nil {
		t.Fatalf("CheckQuota failed: %v", err)
	}
	if !result.Throttled {
		t.Error("expected throttled")
	}
	alerts := m.ListAlerts()
	if len(alerts) == 0 {
		t.Error("expected alerts")
	}
}

func TestKeyNotFound(t *testing.T) {
	m := NewManager()
	if err := m.DisableKey("nonexistent"); err == nil {
		t.Error("expected error for nonexistent key")
	}
	// GetSummary returns empty summary for unknown key (not an error)
	s, err := m.GetSummary("nonexistent", 60)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.TotalRequests != 0 {
		t.Error("expected 0 requests for unknown key")
	}
	// RecordUsage should fail for unknown key
	if err := m.RecordUsage(UsageRecord{KeyID: "nonexistent", Timestamp: time.Now()}); err == nil {
		t.Error("expected error recording usage for nonexistent key")
	}
}