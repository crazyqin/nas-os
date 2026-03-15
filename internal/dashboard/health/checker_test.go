package health

import (
	"context"
	"testing"
	"time"
)

func TestStatus_Constants(t *testing.T) {
	tests := []struct {
		status   Status
		expected string
	}{
		{StatusHealthy, "healthy"},
		{StatusWarning, "warning"},
		{StatusCritical, "critical"},
		{StatusUnknown, "unknown"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.status))
		}
	}
}

func TestCheckType_Constants(t *testing.T) {
	tests := []struct {
		checkType CheckType
		expected  string
	}{
		{CheckTypeCPU, "cpu"},
		{CheckTypeMemory, "memory"},
		{CheckTypeDisk, "disk"},
		{CheckTypeLoad, "load"},
		{CheckTypeProcess, "process"},
		{CheckTypeService, "service"},
		{CheckTypeNetwork, "network"},
	}

	for _, tt := range tests {
		if string(tt.checkType) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.checkType))
		}
	}
}

func TestCheckResult_Fields(t *testing.T) {
	result := CheckResult{
		Type:      CheckTypeCPU,
		Name:      "CPU Usage",
		Status:    StatusHealthy,
		Message:   "CPU usage is normal",
		Value:     45.5,
		Duration:  100 * time.Millisecond,
		Timestamp: time.Now(),
	}

	if result.Type != CheckTypeCPU {
		t.Error("Type mismatch")
	}
	if result.Status != StatusHealthy {
		t.Error("Status mismatch")
	}
}

func TestThreshold_Fields(t *testing.T) {
	threshold := Threshold{
		Warning:  70.0,
		Critical: 90.0,
	}

	if threshold.Warning != 70.0 {
		t.Error("Warning threshold mismatch")
	}
	if threshold.Critical != 90.0 {
		t.Error("Critical threshold mismatch")
	}
}

func TestHealthReport_Fields(t *testing.T) {
	report := HealthReport{
		OverallStatus: StatusHealthy,
		Hostname:      "test-host",
		Timestamp:     time.Now(),
		Duration:      500 * time.Millisecond,
		Checks:        []CheckResult{},
		Summary: Summary{
			Total:    0,
			Healthy:  0,
			Warning:  0,
			Critical: 0,
		},
	}

	if report.OverallStatus != StatusHealthy {
		t.Error("OverallStatus mismatch")
	}
	if report.Hostname != "test-host" {
		t.Error("Hostname mismatch")
	}
}

func TestSummary_Fields(t *testing.T) {
	summary := Summary{
		Total:    5,
		Healthy:  3,
		Warning:  1,
		Critical: 1,
	}

	if summary.Total != 5 {
		t.Error("Total mismatch")
	}
	if summary.Healthy != 3 {
		t.Error("Healthy mismatch")
	}
	if summary.Warning != 1 {
		t.Error("Warning mismatch")
	}
	if summary.Critical != 1 {
		t.Error("Critical mismatch")
	}
}

func TestNewChecker(t *testing.T) {
	config := DefaultConfig()
	checker := NewChecker(config)

	if checker == nil {
		t.Fatal("NewChecker returned nil")
	}
	if checker.config.CPUWarningThreshold != config.CPUWarningThreshold {
		t.Error("Config not set properly")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.CPUWarningThreshold <= 0 {
		t.Error("CPUWarningThreshold should be positive")
	}
	if config.CPUCriticalThreshold <= config.CPUWarningThreshold {
		t.Error("CPUCriticalThreshold should be greater than CPUWarningThreshold")
	}
	if config.Timeout <= 0 {
		t.Error("Timeout should be positive")
	}
}

func TestChecker_CheckCPU(t *testing.T) {
	config := DefaultConfig()
	checker := NewChecker(config)

	result := checker.checkCPU(context.Background())
	if result.Type != CheckTypeCPU {
		t.Error("Type should be CPU")
	}
	if result.Status == "" {
		t.Error("Status should not be empty")
	}
}

func TestChecker_CheckMemory(t *testing.T) {
	config := DefaultConfig()
	checker := NewChecker(config)

	result := checker.checkMemory(context.Background())
	if result.Type != CheckTypeMemory {
		t.Error("Type should be Memory")
	}
	if result.Status == "" {
		t.Error("Status should not be empty")
	}
}

func TestChecker_CheckDisk(t *testing.T) {
	config := DefaultConfig()
	checker := NewChecker(config)

	result := checker.checkDisk(context.Background())
	if result.Type != CheckTypeDisk {
		t.Error("Type should be Disk")
	}
}

func TestChecker_CheckLoad(t *testing.T) {
	config := DefaultConfig()
	checker := NewChecker(config)

	result := checker.checkLoad(context.Background())
	if result.Type != CheckTypeLoad {
		t.Error("Type should be Load")
	}
}
