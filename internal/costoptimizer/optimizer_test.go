package costoptimizer

import (
	"testing"
)

func TestGenerateReport(t *testing.T) {
	optimizer := NewCostOptimizer()
	allocs := []StorageAllocation{
		{
			Path:        "/data/hot",
			Tier:        TierNVMe,
			SizeBytes:   1024 * 1024 * 1024 * 1024, // 1TB
			UsedBytes:   500 * 1024 * 1024 * 1024,   // 500GB
			AccessCount: 10000,
		},
		{
			Path:        "/data/cold",
			Tier:        TierNVMe,
			SizeBytes:   2 * 1024 * 1024 * 1024 * 1024, // 2TB
			UsedBytes:   1 * 1024 * 1024 * 1024 * 1024,  // 1TB
			AccessCount: 2, // 冷数据在NVMe上
		},
	}
	optimizer.SetAllocations(allocs)
	report := optimizer.GenerateReport()
	if report == nil {
		t.Fatal("GenerateReport returned nil")
	}
	if report.TotalMonthlyCost <= 0 {
		t.Error("expected positive monthly cost")
	}
	if len(report.Suggestions) == 0 {
		t.Error("expected optimization suggestions for cold data on NVMe")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}
	for _, tt := range tests {
		result := FormatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("FormatBytes(%d) = %s, expected %s", tt.bytes, result, tt.expected)
		}
	}
}
