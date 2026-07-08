package sysbulletin

import (
	"testing"
)

func TestGenerate_SMARTWarning(t *testing.T) {
	s := Signal{SMARTWarnings: 2}
	bulletins, recs := Generate(s)
	if len(bulletins) == 0 {
		t.Fatal("expected bulletins for SMART warnings")
	}
	if len(recs) == 0 {
		t.Fatal("expected recommendations for SMART warnings")
	}
	found := false
	for _, b := range bulletins {
		if b.Severity == SeverityCritical {
			found = true
		}
	}
	if !found {
		t.Fatal("expected critical severity bulletin")
	}
}

func TestGenerate_SecurityAlerts(t *testing.T) {
	s := Signal{SecurityAlerts: 3}
	bulletins, _ := Generate(s)
	found := false
	for _, b := range bulletins {
		if b.Severity == SeveritySecurity {
			found = true
		}
	}
	if !found {
		t.Fatal("expected security severity bulletin")
	}
}

func TestGenerate_PendingUpdates(t *testing.T) {
	s := Signal{PendingUpdates: 5}
	bulletins, _ := Generate(s)
	found := false
	for _, b := range bulletins {
		if b.Category == CategoryUpdate {
			found = true
		}
	}
	if !found {
		t.Fatal("expected update category bulletin")
	}
}

func TestGenerate_FailedBackups(t *testing.T) {
	s := Signal{FailedBackups: 2}
	bulletins, recs := Generate(s)
	foundB := false
	foundR := false
	for _, b := range bulletins {
		if b.Category == CategoryBackup {
			foundB = true
		}
	}
	for _, r := range recs {
		if r.ID == "bulletin-backup-action" {
			foundR = true
		}
	}
	if !foundB {
		t.Fatal("expected backup bulletin")
	}
	if !foundR {
		t.Fatal("expected backup action recommendation")
	}
}

func TestGenerate_DiskCritical(t *testing.T) {
	s := Signal{DiskUsagePercent: 95}
	bulletins, _ := Generate(s)
	found := false
	for _, b := range bulletins {
		if b.Severity == SeverityCritical && b.Category == CategoryStorage {
			found = true
		}
	}
	if !found {
		t.Fatal("expected critical storage bulletin")
	}
}

func TestGenerate_NetworkIssues(t *testing.T) {
	s := Signal{NetworkIssues: 1}
	bulletins, _ := Generate(s)
	found := false
	for _, b := range bulletins {
		if b.Category == CategoryNetwork {
			found = true
		}
	}
	if !found {
		t.Fatal("expected network bulletin")
	}
}

func TestGenerate_NoIssues(t *testing.T) {
	s := Signal{}
	bulletins, recs := Generate(s)
	if len(bulletins) != 0 {
		t.Fatalf("expected no bulletins, got %d", len(bulletins))
	}
	if len(recs) != 0 {
		t.Fatalf("expected no recommendations, got %d", len(recs))
	}
}
