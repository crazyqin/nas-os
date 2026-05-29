// Package securityadvisor provides security advisory functionality tests.
package securityadvisor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultScanConfig(t *testing.T) {
	config := DefaultScanConfig()

	assert.True(t, config.Enabled)
	assert.True(t, config.WeakPasswords)
	assert.True(t, config.OpenPorts)
	assert.True(t, config.FilePermissions)
	assert.True(t, config.SSLCertificates)
	assert.True(t, config.SystemUpdates)
	assert.True(t, config.MalwareScan)
	assert.True(t, config.FirewallCheck)
}

func TestDefaultPasswordPolicy(t *testing.T) {
	policy := DefaultPasswordPolicy()

	assert.Equal(t, 12, policy.MinLength)
	assert.True(t, policy.RequireUppercase)
	assert.True(t, policy.RequireLowercase)
	assert.True(t, policy.RequireNumbers)
	assert.True(t, policy.RequireSpecial)
	assert.Equal(t, 90, policy.MaxAge)
}

func TestDefaultPortRiskConfig(t *testing.T) {
	config := DefaultPortRiskConfig()

	assert.Contains(t, config.HighRiskPorts, 21)
	assert.Contains(t, config.HighRiskPorts, 23)
	assert.Contains(t, config.HighRiskPorts, 3389)
	assert.Contains(t, config.MediumRiskPorts, 80)
	assert.Contains(t, config.MediumRiskPorts, 443)
}

func TestDefaultCriticalFileConfig(t *testing.T) {
	config := DefaultCriticalFileConfig()

	assert.Contains(t, config.Paths, "/etc/passwd")
	assert.Contains(t, config.Paths, "/etc/shadow")
	assert.Contains(t, config.Paths, "/etc/ssh/sshd_config")
	assert.Equal(t, "0644", config.MaxPermission)
	assert.Equal(t, "root", config.RequiredOwner)
}

func TestDefaultSSLCheckConfig(t *testing.T) {
	config := DefaultSSLCheckConfig()

	assert.Equal(t, 30, config.WarningDays)
	assert.Equal(t, 7, config.CriticalDays)
	assert.Empty(t, config.Domains)
}

func TestDefaultUpdateCheckConfig(t *testing.T) {
	config := DefaultUpdateCheckConfig()

	assert.True(t, config.Enabled)
	assert.True(t, config.CheckSecurity)
	assert.True(t, config.CheckBugfix)
	assert.False(t, config.AutoUpdate)
}

func TestDefaultMalwareScanConfig(t *testing.T) {
	config := DefaultMalwareScanConfig()

	assert.True(t, config.Enabled)
	assert.Contains(t, config.ScanPaths, "/home")
	assert.Contains(t, config.ScanPaths, "/tmp")
	assert.Contains(t, config.ExcludePaths, "/proc")
	assert.Equal(t, int64(100*1024*1024), config.MaxFileSize)
	assert.True(t, config.QuickScan)
}

func TestDefaultScoreWeight(t *testing.T) {
	weight := DefaultScoreWeight()

	assert.InDelta(t, 0.15, weight.Password, 0.001)
	assert.InDelta(t, 0.10, weight.Port, 0.001)
	assert.InDelta(t, 0.05, weight.Permission, 0.001)
	assert.InDelta(t, 0.05, weight.SSL, 0.001)
	assert.InDelta(t, 0.05, weight.Update, 0.001)
	assert.InDelta(t, 0.10, weight.Malware, 0.001)
	assert.InDelta(t, 0.05, weight.Firewall, 0.001)
}

func TestCalculateOverallScore(t *testing.T) {
	tests := []struct {
		name     string
		checks   []SecurityCheck
		expected int
	}{
		{
			name:     "empty checks",
			checks:   []SecurityCheck{},
			expected: 0,
		},
		{
			name: "all pass",
			checks: []SecurityCheck{
				{Category: "password", Score: 100},
				{Category: "port", Score: 100},
			},
			expected: 100,
		},
		{
			name: "mixed scores",
			checks: []SecurityCheck{
				{Category: "password", Score: 80},
				{Category: "port", Score: 60},
			},
			expected: 72, // 加权平均
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := CalculateOverallScore(tt.checks)
			assert.Equal(t, tt.expected, score)
		})
	}
}

func TestGetSecurityLevel(t *testing.T) {
	tests := []struct {
		score    int
		expected string
	}{
		{100, "good"},
		{80, "good"},
		{79, "warning"},
		{60, "warning"},
		{59, "critical"},
		{0, "critical"},
	}

	for _, tt := range tests {
		level := GetSecurityLevel(tt.score)
		assert.Equal(t, tt.expected, level)
	}
}

func TestFormatSecurityLevel(t *testing.T) {
	tests := []struct {
		level    string
		expected string
	}{
		{"good", "Good"},
		{"warning", "Warning"},
		{"critical", "Critical"},
		{"unknown", "Unknown"},
	}

	for _, tt := range tests {
		result := FormatSecurityLevel(tt.level)
		assert.Equal(t, tt.expected, result)
	}
}

func TestGetScoreColor(t *testing.T) {
	tests := []struct {
		score    int
		expected string
	}{
		{100, "green"},
		{80, "green"},
		{79, "yellow"},
		{60, "yellow"},
		{59, "red"},
		{0, "red"},
	}

	for _, tt := range tests {
		color := GetScoreColor(tt.score)
		assert.Equal(t, tt.expected, color)
	}
}

func TestGetPriority(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"critical", "high"},
		{"warning", "medium"},
		{"pass", "low"},
		{"info", "low"},
	}

	for _, tt := range tests {
		priority := getPriority(tt.status)
		assert.Equal(t, tt.expected, priority)
	}
}

func TestCalculateCategoryScore(t *testing.T) {
	checks := []SecurityCheck{
		{Category: "password", Score: 80},
		{Category: "password", Score: 100},
		{Category: "port", Score: 60},
	}

	tests := []struct {
		category string
		expected int
	}{
		{"password", 90},
		{"port", 60},
		{"ssl", 100}, // 默认满分
	}

	for _, tt := range tests {
		score := CalculateCategoryScore(checks, tt.category)
		assert.Equal(t, tt.expected, score)
	}
}

func TestGetCategoryStatus(t *testing.T) {
	checks := []SecurityCheck{
		{Category: "password", Score: 90},
		{Category: "port", Score: 50},
	}

	tests := []struct {
		category string
		expected string
	}{
		{"password", "good"},
		{"port", "critical"},
	}

	for _, tt := range tests {
		status := GetCategoryStatus(checks, tt.category)
		assert.Equal(t, tt.expected, status)
	}
}

func TestGenerateRecommendations(t *testing.T) {
	checks := []SecurityCheck{
		{
			ID:       "test-1",
			Category: "password",
			Status:   "warning",
			Message:  "Weak password detected",
		},
		{
			ID:       "test-2",
			Category: "port",
			Status:   "critical",
			Message:  "High risk port open",
		},
		{
			ID:       "test-3",
			Category: "ssl",
			Status:   "pass",
			Message:  "SSL valid",
		},
	}

	recommendations := GenerateRecommendations(checks)
	assert.Len(t, recommendations, 2) // 只有 warning 和 critical

	// 验证建议内容
	found := false
	for _, rec := range recommendations {
		if rec.Category == "password" {
			found = true
			assert.Equal(t, "high", rec.Priority)
		}
	}
	assert.True(t, found)
}

func TestNewScanner(t *testing.T) {
	config := DefaultScanConfig()
	logger := zap.NewNop()

	scanner := NewScanner(config, logger)
	assert.NotNil(t, scanner)
	assert.Equal(t, config, scanner.config)
}

func TestNewScannerNilLogger(t *testing.T) {
	config := DefaultScanConfig()

	scanner := NewScanner(config, nil)
	assert.NotNil(t, scanner)
	assert.NotNil(t, scanner.logger)
}

func TestParsePasswdFile(t *testing.T) {
	content := `root:x:0:0:root:/root:/bin/bash
daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
testuser:x:1000:1000:Test User:/home/testuser:/bin/bash
anotheruser:x:1001:1001:Another User:/home/anotheruser:/bin/bash`

	users := parsePasswdFile(content)
	assert.Len(t, users, 2)
	assert.Contains(t, users, "testuser")
	assert.Contains(t, users, "anotheruser")
}

func TestAssessPortRisk(t *testing.T) {
	config := DefaultPortRiskConfig()

	tests := []struct {
		port     int
		expected string
	}{
		{21, "high"},
		{23, "high"},
		{3389, "high"},
		{80, "medium"},
		{443, "medium"},
		{8080, "medium"},
		{22, "low"},
		{8000, "low"},
	}

	for _, tt := range tests {
		risk := assessPortRisk(tt.port, config)
		assert.Equal(t, tt.expected, risk)
	}
}

func TestCheckPasswordStrength(t *testing.T) {
	policy := DefaultPasswordPolicy()
	strength := checkPasswordStrength("testuser", policy)
	assert.Contains(t, []string{"weak", "medium", "strong"}, strength)
}

func TestRunFullScanContextCancellation(t *testing.T) {
	config := DefaultScanConfig()
	logger := zap.NewNop()
	scanner := NewScanner(config, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	report, err := scanner.RunFullScan(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, report)
}

func TestHandler(t *testing.T) {
	config := DefaultScanConfig()
	logger := zap.NewNop()
	scanner := NewScanner(config, logger)

	handler := NewHandler(scanner, logger)
	assert.NotNil(t, handler)
}

func TestHandlerNilLogger(t *testing.T) {
	config := DefaultScanConfig()
	logger := zap.NewNop()
	scanner := NewScanner(config, logger)

	handler := NewHandler(scanner, nil)
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.logger)
}

func TestRunFullScan(t *testing.T) {
	config := DefaultScanConfig()
	logger := zap.NewNop()
	scanner := NewScanner(config, logger)

	report, err := scanner.RunFullScan(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.NotEmpty(t, report.ID)
	assert.True(t, report.OverallScore >= 0 && report.OverallScore <= 100)
	assert.Contains(t, []string{"good", "warning", "critical"}, report.SecurityLevel)
}

func TestScanMalwareNoClamscan(t *testing.T) {
	config := DefaultScanConfig()
	config.MalwareScan = true
	logger := zap.NewNop()
	scanner := NewScanner(config, logger)

	checks := scanner.scanMalware(context.Background())
	assert.NotEmpty(t, checks)
	assert.Equal(t, "malware", checks[0].Category)
}

func TestScanFirewallStatus(t *testing.T) {
	config := DefaultScanConfig()
	config.FirewallCheck = true
	logger := zap.NewNop()
	scanner := NewScanner(config, logger)

	checks := scanner.scanFirewall(context.Background())
	assert.NotEmpty(t, checks)
	assert.Equal(t, "firewall", checks[0].Category)
}

func TestScanOpenPortsParsing(t *testing.T) {
	config := DefaultScanConfig()
	config.OpenPorts = true
	logger := zap.NewNop()
	scanner := NewScanner(config, logger)

	checks := scanner.scanOpenPorts(context.Background())
	// 结果取决于系统状态，但应该返回结果
	assert.NotNil(t, checks)
}

func TestScanSystemUpdatesParsing(t *testing.T) {
	config := DefaultScanConfig()
	config.SystemUpdates = true
	logger := zap.NewNop()
	scanner := NewScanner(config, logger)

	checks := scanner.scanSystemUpdates(context.Background())
	assert.NotEmpty(t, checks)
	assert.Equal(t, "update", checks[0].Category)
}

func TestScanSSLCertificatesWithDomains(t *testing.T) {
	config := DefaultScanConfig()
	config.SSLCertificates = true
	logger := zap.NewNop()
	scanner := NewScanner(config, logger)

	checks := scanner.scanSSLCertificates(context.Background())
	// 没有配置域名，应该返回空
	assert.Empty(t, checks)
}

func TestScanFilePermissionsCheck(t *testing.T) {
	config := DefaultScanConfig()
	config.FilePermissions = true
	logger := zap.NewNop()
	scanner := NewScanner(config, logger)

	checks := scanner.scanFilePermissions(context.Background())
	assert.NotEmpty(t, checks)
	assert.Equal(t, "permission", checks[0].Category)
}

func TestGenerateSummary(t *testing.T) {
	tests := []struct {
		level    string
		expected string
	}{
		{"good", "Your system security is good. No critical issues found."},
		{"warning", "Some security issues detected. Please review the recommendations."},
		{"critical", "Critical security issues found! Immediate action required."},
		{"unknown", "Security scan completed."},
	}

	for _, tt := range tests {
		report := &SecurityReport{SecurityLevel: tt.level}
		summary := generateSummary(report)
		assert.Equal(t, tt.expected, summary)
	}
}
