package securityhardening

import (
	"testing"
	"time"
)

func TestNewSecurityHardener(t *testing.T) {
	config := HardenerConfig{
		Level:           SecurityStandard,
		AutoFix:         true,
		ScanInterval:    time.Hour,
		NotifyOnFailure: true,
	}

	hardener := NewSecurityHardener(config)
	if hardener == nil {
		t.Fatal("NewSecurityHardener returned nil")
	}
}

func TestSecurityHardener_RunAllChecks(t *testing.T) {
	config := HardenerConfig{
		Level:   SecurityStandard,
		AutoFix: false,
	}

	hardener := NewSecurityHardener(config)

	checks, err := hardener.RunAllChecks()
	if err != nil {
		t.Fatalf("RunAllChecks failed: %v", err)
	}

	if len(checks) == 0 {
		t.Error("Expected at least one check result")
	}

	// 统计检查结果
	passCount := 0
	failCount := 0
	warningCount := 0

	for _, check := range checks {
		switch check.Status {
		case "pass":
			passCount++
		case "fail":
			failCount++
		case "warning":
			warningCount++
		}
	}

	t.Logf("Checks: %d pass, %d fail, %d warning", passCount, failCount, warningCount)
}

func TestSecurityHardener_GetSecurityScore(t *testing.T) {
	config := HardenerConfig{
		Level: SecurityStandard,
	}

	hardener := NewSecurityHardener(config)

	// 运行检查以更新状态
	hardener.RunAllChecks()

	score := hardener.GetSecurityScore()
	t.Logf("Security score: %d", score)

	if score < 0 || score > 100 {
		t.Errorf("Invalid security score: %d", score)
	}
}

func TestSecurityHardener_GetCVEDefinitions(t *testing.T) {
	config := HardenerConfig{
		Level: SecurityStandard,
	}

	hardener := NewSecurityHardener(config)

	cves := hardener.GetCVEDefinitions()
	if len(cves) == 0 {
		t.Error("Expected at least one CVE definition")
	}

	// 检查CVE-2026-24061是否存在
	found := false
	for _, cve := range cves {
		if cve.ID == "CVE-2026-24061" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected CVE-2026-24061 to be in the database")
	}
}

func TestSecurityHardener_CheckCVE(t *testing.T) {
	config := HardenerConfig{
		Level: SecurityStandard,
	}

	hardener := NewSecurityHardener(config)

	affected, err := hardener.CheckCVE("CVE-2026-24061")
	if err != nil {
		t.Fatalf("CheckCVE failed: %v", err)
	}

	t.Logf("System affected by CVE-2026-24061: %v", affected)
}

func TestSecurityHardener_GenerateReport(t *testing.T) {
	config := HardenerConfig{
		Level: SecurityHigh,
	}

	hardener := NewSecurityHardener(config)

	// 运行检查
	hardener.RunAllChecks()

	report := hardener.GenerateReport()

	if report.Score < 0 || report.Score > 100 {
		t.Errorf("Invalid report score: %d", report.Score)
	}

	if len(report.Checks) == 0 {
		t.Error("Expected report to contain checks")
	}

	t.Logf("Security report: Level=%d, Score=%d, Checks=%d",
		report.Level, report.Score, len(report.Checks))
}

func TestSecurityHardener_FixIssue(t *testing.T) {
	config := HardenerConfig{
		Level:   SecurityStandard,
		AutoFix: true,
	}

	hardener := NewSecurityHardener(config)

	// 尝试修复telnet问题
	err := hardener.FixIssue("telnet_disabled")
	if err != nil {
		t.Fatalf("FixIssue failed: %v", err)
	}
}
