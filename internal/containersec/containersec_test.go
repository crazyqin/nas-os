package containersec

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// MockVulnDatabase implements VulnDatabase for testing
type MockVulnDatabase struct {
	vulns map[string][]CVE
}

func NewMockVulnDatabase() *MockVulnDatabase {
	return &MockVulnDatabase{
		vulns: make(map[string][]CVE),
	}
}

func (m *MockVulnDatabase) Lookup(ctx context.Context, pkg, version, distro string) ([]CVE, error) {
	key := pkg + ":" + version
	if vulns, ok := m.vulns[key]; ok {
		return vulns, nil
	}
	return nil, nil
}

func (m *MockVulnDatabase) Update(ctx context.Context) error {
	return nil
}

func (m *MockVulnDatabase) LastUpdate() time.Time {
	return time.Now()
}

func (m *MockVulnDatabase) AddVulnerability(pkg, version string, cve CVE) {
	key := pkg + ":" + version
	m.vulns[key] = append(m.vulns[key], cve)
}

func TestScanner_CacheTTL(t *testing.T) {
	logger := zap.NewNop()
	mockDB := NewMockVulnDatabase()
	scanner := NewScanner(logger, mockDB, 100*time.Millisecond)

	// Store in cache
	result := &ScanResult{
		Image:    "test:latest",
		ScanTime: time.Now(),
	}
	scanner.setCache("test:latest", result)

	// Should be in cache
	cached := scanner.getFromCache("test:latest")
	assert.NotNil(t, cached)

	// Wait for TTL
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	cached = scanner.getFromCache("test:latest")
	assert.Nil(t, cached)
}

func TestScanner_ClearCache(t *testing.T) {
	logger := zap.NewNop()
	mockDB := NewMockVulnDatabase()
	scanner := NewScanner(logger, mockDB, time.Hour)

	// Store in cache
	result := &ScanResult{
		Image:    "test:latest",
		ScanTime: time.Now(),
	}
	scanner.setCache("test:latest", result)

	// Clear cache
	scanner.ClearCache()

	// Should be gone
	cached := scanner.getFromCache("test:latest")
	assert.Nil(t, cached)
}

func TestPolicyEngine_AddRemovePolicy(t *testing.T) {
	logger := zap.NewNop()
	engine := NewPolicyEngine(logger)

	// Default policy should exist
	policies := engine.ListPolicies()
	assert.Len(t, policies, 1)

	// Add custom policy
	custom := SecurityPolicy{
		ID:   "custom",
		Name: "Custom Policy",
	}
	engine.AddPolicy(custom)

	policies = engine.ListPolicies()
	assert.Len(t, policies, 2)

	// Get policy
	got := engine.GetPolicy("custom")
	require.NotNil(t, got)
	assert.Equal(t, "Custom Policy", got.Name)

	// Remove policy
	assert.True(t, engine.RemovePolicy("custom"))
	assert.False(t, engine.RemovePolicy("nonexistent"))

	policies = engine.ListPolicies()
	assert.Len(t, policies, 1)
}

func TestPolicyEngine_EvaluateCriticalVulns(t *testing.T) {
	logger := zap.NewNop()
	engine := NewPolicyEngine(logger)

	result := &ScanResult{
		Image: "test:latest",
		Summary: VulnSummary{
			Total:    5,
			Critical: 2,
			High:     1,
		},
	}

	violations := engine.EvaluateImage(result)
	assert.Len(t, violations, 1) // Only critical violation
	assert.Equal(t, "max_critical", violations[0].Rule)
	assert.Equal(t, "blocking", violations[0].Severity)
}

func TestPolicyEngine_EvaluateHighVulns(t *testing.T) {
	logger := zap.NewNop()
	engine := NewPolicyEngine(logger)

	result := &ScanResult{
		Image: "test:latest",
		Summary: VulnSummary{
			Total: 10,
			High:  6,
		},
	}

	violations := engine.EvaluateImage(result)
	assert.Len(t, violations, 1)
	assert.Equal(t, "max_high", violations[0].Rule)
}

func TestPolicyEngine_EvaluateBlockedPackages(t *testing.T) {
	logger := zap.NewNop()
	engine := NewPolicyEngine(logger)

	// Update default policy to block a package
	policy := engine.GetPolicy("default")
	require.NotNil(t, policy)
	policy.BlockedPackages = []string{"malware-pkg"}
	engine.AddPolicy(*policy)

	result := &ScanResult{
		Image: "test:latest",
		Vulns: []Vulnerability{
			{
				CVE:          CVE{ID: "CVE-2024-0001"},
				InstalledPkg: "malware-pkg",
			},
		},
	}

	violations := engine.EvaluateImage(result)
	assert.Len(t, violations, 1)
	assert.Equal(t, "blocked_package", violations[0].Rule)
}

func TestPolicyEngine_ApproveImage(t *testing.T) {
	logger := zap.NewNop()
	engine := NewPolicyEngine(logger)

	// Clean image - should be approved
	cleanResult := &ScanResult{
		Image: "clean:latest",
		Summary: VulnSummary{
			Total:    2,
			Critical: 0,
			High:     0,
		},
	}

	approved, suggestions := engine.ApproveImage(cleanResult)
	assert.True(t, approved)
	assert.Empty(t, suggestions)

	// Dirty image - should not be approved
	dirtyResult := &ScanResult{
		Image: "dirty:latest",
		Summary: VulnSummary{
			Total:    10,
			Critical: 3,
			High:     5,
		},
		Vulns: []Vulnerability{
			{
				CVE:          CVE{ID: "CVE-2024-0001", Severity: SeverityCritical, FixedIn: "1.2.3"},
				InstalledPkg: "vulnerable-pkg",
			},
		},
	}

	approved, suggestions = engine.ApproveImage(dirtyResult)
	assert.False(t, approved)
	assert.NotEmpty(t, suggestions)
}

func TestPolicyEngine_GenerateRemediation(t *testing.T) {
	logger := zap.NewNop()
	engine := NewPolicyEngine(logger)

	violations := []PolicyViolation{
		{Rule: "max_critical"},
		{Rule: "blocked_package"},
		{Rule: "allowed_registries"},
	}

	result := &ScanResult{
		Vulns: []Vulnerability{
			{
				CVE:          CVE{ID: "CVE-2024-0001"},
				InstalledPkg: "pkg1",
				Version:      "1.0.0",
			},
		},
	}
	result.Vulns[0].FixedIn = "1.1.0"

	suggestions := engine.GenerateRemediation(violations, result)
	assert.NotEmpty(t, suggestions)
	assert.Contains(t, suggestions[0], "1.1.0") // Should mention fix version
}

func TestPolicyEngine_BenchmarkScore(t *testing.T) {
	logger := zap.NewNop()
	engine := NewPolicyEngine(logger)

	// Update policy to require benchmark score
	policy := engine.GetPolicy("default")
	require.NotNil(t, policy)
	policy.BenchmarkMinScore = 70.0
	engine.AddPolicy(*policy)

	result := &ScanResult{
		Image: "test:latest",
		BenchmarkScore: &BenchmarkScore{
			TotalScore: 50.0,
		},
	}

	violations := engine.EvaluateImage(result)
	assert.Len(t, violations, 1)
	assert.Equal(t, "benchmark_min_score", violations[0].Rule)
}

func TestScanner_ScanImage(t *testing.T) {
	logger := zap.NewNop()
	mockDB := NewMockVulnDatabase()

	// Add some test vulnerabilities
	mockDB.AddVulnerability("curl", "7.81.0-1ubuntu1.16", CVE{
		ID:          "CVE-2024-1234",
		Severity:    SeverityHigh,
		Title:       "Test vulnerability",
		Description: "A test vulnerability",
		Package:     "curl",
		Version:     "7.81.0-1ubuntu1.16",
		FixedIn:     "7.81.0-1ubuntu1.17",
	})

	scanner := NewScanner(logger, mockDB, time.Hour)

	ctx := context.Background()
	result, err := scanner.ScanImage(ctx, "test:latest", "docker.io", false)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "docker.io/test:latest", result.Image)
	assert.NotNil(t, result.Summary)
	assert.NotNil(t, result.BenchmarkScore)
}

func TestVulnSummary_Counts(t *testing.T) {
	vulns := []Vulnerability{
		{CVE: CVE{Severity: SeverityCritical}},
		{CVE: CVE{Severity: SeverityCritical}},
		{CVE: CVE{Severity: SeverityHigh}},
		{CVE: CVE{Severity: SeverityMedium}},
		{CVE: CVE{Severity: SeverityLow}},
		{CVE: CVE{Severity: SeverityInfo}},
	}

	scanner := &Scanner{}
	summary := scanner.buildSummary(vulns)

	assert.Equal(t, 6, summary.Total)
	assert.Equal(t, 2, summary.Critical)
	assert.Equal(t, 1, summary.High)
	assert.Equal(t, 1, summary.Medium)
	assert.Equal(t, 1, summary.Low)
	assert.Equal(t, 1, summary.Info)
}

func TestDefaultSecurityPolicy(t *testing.T) {
	policy := DefaultSecurityPolicy()

	assert.Equal(t, "default", policy.ID)
	assert.True(t, policy.Enabled)
	assert.Equal(t, 0, policy.MaxCritical)
	assert.Equal(t, 5, policy.MaxHigh)
	assert.Equal(t, 50, policy.MaxTotal)
	assert.True(t, policy.RequireNonRoot)
	assert.Equal(t, 60.0, policy.BenchmarkMinScore)
}

func TestPolicyEngine_DisabledPolicy(t *testing.T) {
	logger := zap.NewNop()
	engine := NewPolicyEngine(logger)

	// Disable default policy
	policy := engine.GetPolicy("default")
	require.NotNil(t, policy)
	policy.Enabled = false
	engine.AddPolicy(*policy)

	// Result with critical vulns - should pass because policy is disabled
	result := &ScanResult{
		Summary: VulnSummary{
			Critical: 100,
		},
	}

	violations := engine.EvaluateImage(result)
	assert.Empty(t, violations)
}
