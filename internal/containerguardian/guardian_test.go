package containerguardian

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestNewGuardian(t *testing.T) {
	g := NewGuardian(nil)
	if g == nil {
		t.Fatal("NewGuardian returned nil")
	}
	if g.results == nil {
		t.Error("results map not initialized")
	}
	if g.vulnDB == nil {
		t.Error("vulnDB map not initialized")
	}
}

func TestScanImage_Nginx(t *testing.T) {
	g := NewGuardian(zap.NewNop())
	ctx := context.Background()

	result, err := g.ScanImage(ctx, ScanRequest{Image: "nginx:1.20"})
	if err != nil {
		t.Fatalf("ScanImage failed: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Image != "nginx:1.20" {
		t.Errorf("Image = %q, want nginx:1.20", result.Image)
	}

	if result.Status != ScanStatusCompleted {
		t.Errorf("Status = %q, want %q", result.Status, ScanStatusCompleted)
	}

	if result.Score <= 0 {
		t.Errorf("Score = %f, want > 0", result.Score)
	}

	if result.Grade == "" {
		t.Error("Grade should not be empty")
	}
}

func TestScanImage_Clean(t *testing.T) {
	g := NewGuardian(zap.NewNop())
	ctx := context.Background()

	result, err := g.ScanImage(ctx, ScanRequest{Image: "alpine:3.18"})
	if err != nil {
		t.Fatalf("ScanImage failed: %v", err)
	}

	if len(result.Vulnerabilities) != 0 {
		t.Errorf("Expected 0 vulns for clean image, got %d", len(result.Vulnerabilities))
	}

	if result.Score < 90 {
		t.Errorf("Score = %f, want >= 90 for clean image", result.Score)
	}
}

func TestScanImage_WithCompliance(t *testing.T) {
	g := NewGuardian(zap.NewNop())
	ctx := context.Background()

	result, err := g.ScanImage(ctx, ScanRequest{
		Image:          "nginx:1.20",
		SkipCompliance: false,
		SkipSignature:  true,
		SkipSensitive:  true,
	})
	if err != nil {
		t.Fatalf("ScanImage failed: %v", err)
	}

	if len(result.Compliance) == 0 {
		t.Error("Expected compliance rules to be checked")
	}
}

func TestScanImage_WithSensitiveDetection(t *testing.T) {
	g := NewGuardian(zap.NewNop())
	ctx := context.Background()

	result, err := g.ScanImage(ctx, ScanRequest{
		Image:         "app:latest",
		SkipSensitive: false,
	})
	if err != nil {
		t.Fatalf("ScanImage failed: %v", err)
	}

	// Sensitive findings may or may not exist for this test image
	if result.Sensitive == nil {
		t.Error("Sensitive findings should be initialized (not nil)")
	}
}

func TestGetScanResult(t *testing.T) {
	g := NewGuardian(zap.NewNop())
	ctx := context.Background()

	result, _ := g.ScanImage(ctx, ScanRequest{Image: "nginx:1.20"})

	got := g.GetScanResult(result.ID)
	if got == nil {
		t.Fatal("GetScanResult returned nil")
	}

	if got.ID != result.ID {
		t.Errorf("ID = %q, want %q", got.ID, result.ID)
	}
}

func TestGetScanResult_NotFound(t *testing.T) {
	g := NewGuardian(zap.NewNop())

	got := g.GetScanResult("nonexistent")
	if got != nil {
		t.Error("Expected nil for nonexistent scan")
	}
}

func TestListScanResults(t *testing.T) {
	g := NewGuardian(zap.NewNop())
	ctx := context.Background()

	g.ScanImage(ctx, ScanRequest{Image: "nginx:1.20"})
	g.ScanImage(ctx, ScanRequest{Image: "redis:6.0"})

	results := g.ListScanResults()
	if len(results) != 2 {
		t.Errorf("ListScanResults returned %d results, want 2", len(results))
	}
}

func TestDeleteScanResult(t *testing.T) {
	g := NewGuardian(zap.NewNop())
	ctx := context.Background()

	result, _ := g.ScanImage(ctx, ScanRequest{Image: "nginx:1.20"})

	if !g.DeleteScanResult(result.ID) {
		t.Error("DeleteScanResult should return true")
	}

	if g.GetScanResult(result.ID) != nil {
		t.Error("Deleted result should not be found")
	}

	if g.DeleteScanResult("nonexistent") {
		t.Error("DeleteScanResult should return false for nonexistent")
	}
}

func TestGetSecurityScore(t *testing.T) {
	g := NewGuardian(zap.NewNop())
	ctx := context.Background()

	result, _ := g.ScanImage(ctx, ScanRequest{Image: "nginx:1.20"})

	score, err := g.GetSecurityScore(result.ID)
	if err != nil {
		t.Fatalf("GetSecurityScore failed: %v", err)
	}

	if score.Overall <= 0 {
		t.Errorf("Overall score = %f, want > 0", score.Overall)
	}

	if score.Grade == "" {
		t.Error("Grade should not be empty")
	}
}

func TestGetAuditLog(t *testing.T) {
	g := NewGuardian(zap.NewNop())
	ctx := context.Background()

	g.ScanImage(ctx, ScanRequest{Image: "nginx:1.20"})

	logs := g.GetAuditLog("")
	if len(logs) == 0 {
		t.Error("Expected audit log entries")
	}

	if logs[0].Action != "ScanImage" {
		t.Errorf("Action = %q, want ScanImage", logs[0].Action)
	}
}

func TestAddVulnerability(t *testing.T) {
	g := NewGuardian(zap.NewNop())

	vuln := Vulnerability{
		ID:       "CUSTOM-001",
		CVE:      "CVE-2024-0001",
		Severity: SeverityHigh,
		Package:  "custom-pkg",
		Version:  "1.0",
		FixedIn:  "1.1",
	}

	g.AddVulnerability("custom:1.0", vuln)

	ctx := context.Background()
	result, _ := g.ScanImage(ctx, ScanRequest{Image: "custom:1.0"})

	found := false
	for _, v := range result.Vulnerabilities {
		if v.ID == "CUSTOM-001" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Custom vulnerability not found in scan results")
	}
}

func TestScanImage_EmptyImage(t *testing.T) {
	g := NewGuardian(zap.NewNop())
	ctx := context.Background()

	_, err := g.ScanImage(ctx, ScanRequest{Image: ""})
	if err == nil {
		t.Error("Expected error for empty image")
	}
}
