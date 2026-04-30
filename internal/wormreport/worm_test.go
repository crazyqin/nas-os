package wormreport

import (
	"testing"
	"time"
)

func TestLockAndVerify(t *testing.T) {
	m := NewWORMManager()
	file, err := m.Lock("/data/contract.pdf", 1024*1024, RetentionCompliance, "admin", nil)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}
	if file.Status != WORMProtected {
		t.Errorf("expected status protected, got %s", file.Status)
	}
	if file.FileHash == "" {
		t.Error("expected file hash")
	}

	// Verify
	ok, err := m.Verify(file.ID)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if !ok {
		t.Error("expected integrity ok")
	}

	// Cannot lock same file twice
	_, err = m.Lock("/data/contract.pdf", 1024, RetentionCompliance, "admin", nil)
	if err == nil {
		t.Error("expected error for duplicate lock")
	}
}

func TestExpiry(t *testing.T) {
	m := NewWORMManager()
	d := 1 * time.Millisecond
	file, _ := m.Lock("/data/temp.dat", 100, RetentionGovernance, "admin", &d)
	time.Sleep(5 * time.Millisecond)
	count := m.ExpireExpired()
	if count != 1 {
		t.Errorf("expected 1 expired, got %d", count)
	}
	updated, _ := m.Get(file.ID)
	if updated.Status != WORMExpired {
		t.Errorf("expected expired, got %s", updated.Status)
	}
}

func TestReport(t *testing.T) {
	m := NewWORMManager()
	m.Lock("/a.pdf", 100, RetentionCompliance, "u1", nil)
	m.Lock("/b.pdf", 200, RetentionLegal, "u1", nil)
	report := m.GenerateReport("daily")
	if report.TotalFiles != 2 {
		t.Errorf("expected 2 files, got %d", report.TotalFiles)
	}
	if report.IntegrityScore != 100 {
		t.Errorf("expected 100 score, got %f", report.IntegrityScore)
	}
	if report.Summary == "" {
		t.Error("expected summary")
	}
}

func TestListFilter(t *testing.T) {
	m := NewWORMManager()
	m.Lock("/a.pdf", 100, RetentionCompliance, "u1", nil)
	m.Lock("/b.pdf", 200, RetentionLegal, "u1", nil)
	all := m.List("", "")
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}
	compliance := m.List("", RetentionCompliance)
	if len(compliance) != 1 {
		t.Errorf("expected 1 compliance, got %d", len(compliance))
	}
}
