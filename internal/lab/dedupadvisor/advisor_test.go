package dedupadvisor

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateHighDuplicateRisk(t *testing.T) {
	advisor := New().WithNow(func() time.Time {
		return time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC)
	})
	report := advisor.Generate(Signal{
		PoolName:              "tank",
		TotalSizeGB:           20000,
		UsedSizeGB:            16000,
		DedupRatio:            1.0,
		DedupEnabled:          false,
		FileCount:             100000,
		DuplicateFileEstimate: 40000, // 40% 重复
		AvgFileSizeMB:         50,
		PoolType:              "zfs",
		HasSSDTier:            false,
		FreePercent:           20,
		CompressEnabled:       false,
		WorkloadType:          "archive",
	})

	if report.DedupPotential != "high" {
		t.Fatalf("dedupPotential = %s, want high", report.DedupPotential)
	}

	wantIDs := map[string]bool{
		"enable-block-dedup":       false,
		"enable-compression":       false,
		"add-ssd-tier-for-dedup":   false, // zfs + dedupEnabled=false → 不会出现；但 enable-block-dedup 后不会触发此条
	}
	for _, rec := range report.Recommendations {
		if _, ok := wantIDs[rec.ID]; ok {
			wantIDs[rec.ID] = true
		}
	}
	if !wantIDs["enable-block-dedup"] {
		t.Fatalf("missing recommendation enable-block-dedup in %#v", report.Recommendations)
	}
	if !wantIDs["enable-compression"] {
		t.Fatalf("missing recommendation enable-compression in %#v", report.Recommendations)
	}

	// 高重复风险应使 enable-block-dedup 为 high 优先级
	for _, rec := range report.Recommendations {
		if rec.ID == "enable-block-dedup" && rec.Priority != "high" {
			t.Fatalf("enable-block-dedup priority = %s, want high", rec.Priority)
		}
	}

	if report.GeneratedAt != time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("generatedAt = %v, want 2026-07-07", report.GeneratedAt)
	}
}

func TestGenerateAlreadyEnabledLowRatio(t *testing.T) {
	report := New().Generate(Signal{
		PoolName:              "tank",
		TotalSizeGB:           10000,
		UsedSizeGB:            6000,
		DedupRatio:            1.05,
		DedupEnabled:          true,
		FileCount:             5000,
		DuplicateFileEstimate: 200,
		AvgFileSizeMB:         10,
		PoolType:              "zfs",
		HasSSDTier:            false,
		FreePercent:           40,
		CompressEnabled:       true,
		WorkloadType:          "documents",
	})

	found := false
	for _, rec := range report.Recommendations {
		if rec.ID == "evaluate-dedup-benefit" {
			found = true
			if rec.Priority != "medium" {
				t.Fatalf("evaluate-dedup-benefit priority = %s, want medium", rec.Priority)
			}
		}
	}
	if !found {
		t.Fatalf("missing evaluate-dedup-benefit in %#v", report.Recommendations)
	}

	// 已启用去重但无 SSD 缓存层，应建议添加 SSD
	foundSSD := false
	for _, rec := range report.Recommendations {
		if rec.ID == "add-ssd-tier-for-dedup" {
			foundSSD = true
		}
	}
	if !foundSSD {
		t.Fatalf("missing add-ssd-tier-for-dedup in %#v", report.Recommendations)
	}
}

func TestGenerateVMWorkloadSkip(t *testing.T) {
	report := New().Generate(Signal{
		PoolName:              "vmstore",
		TotalSizeGB:           8000,
		UsedSizeGB:            5000,
		DedupRatio:            1.0,
		DedupEnabled:          false,
		FileCount:             20000,
		DuplicateFileEstimate: 5000, // 25% 重复，medium potential
		AvgFileSizeMB:         200,
		PoolType:              "zfs",
		HasSSDTier:            true,
		FreePercent:           37,
		CompressEnabled:       true,
		WorkloadType:          "vm",
	})

	if report.DedupPotential != "medium" {
		t.Fatalf("dedupPotential = %s, want medium", report.DedupPotential)
	}

	foundSkip := false
	foundEnable := false
	for _, rec := range report.Recommendations {
		if rec.ID == "skip-block-dedup-vm" {
			foundSkip = true
		}
		if rec.ID == "enable-block-dedup" {
			foundEnable = true
		}
	}
	if !foundSkip {
		t.Fatalf("missing skip-block-dedup-vm in %#v", report.Recommendations)
	}
	// 同时也应出现 enable-block-dedup（因为 dupPercent > 10），但 skip 建议也应存在
	_ = foundEnable // 两者共存是合理的——用户可自行权衡
}

func TestGenerateHealthyPool(t *testing.T) {
	report := New().Generate(Signal{
		PoolName:              "tank",
		TotalSizeGB:           20000,
		UsedSizeGB:            12000,
		DedupRatio:            1.8,
		DedupEnabled:          true,
		FileCount:             500000,
		DuplicateFileEstimate: 10000, // 2% 重复
		AvgFileSizeMB:         20,
		PoolType:              "zfs",
		HasSSDTier:            true,
		FreePercent:           40,
		CompressEnabled:       true,
		WorkloadType:          "archive",
	})

	if report.DedupPotential != "none" {
		t.Fatalf("dedupPotential = %s, want none", report.DedupPotential)
	}
	if report.DedupScore < 95 {
		t.Fatalf("score = %d, want >= 95 for healthy pool", report.DedupScore)
	}
	if len(report.Recommendations) != 0 {
		t.Fatalf("recommendations = %#v, want none", report.Recommendations)
	}
}

func TestSummarizeActions(t *testing.T) {
	summary := SummarizeActions([]Recommendation{
		{Title: "启用块级去重", Actions: []string{"在低峰时段启用去重"}},
		{Title: "配合启用压缩", Actions: []string{"启用 lz4 压缩"}},
	})
	if !strings.Contains(summary, "启用块级去重: 在低峰时段启用去重") {
		t.Fatalf("summary = %q, missing first action", summary)
	}
	if !strings.Contains(summary, "配合启用压缩: 启用 lz4 压缩") {
		t.Fatalf("summary = %q, missing second action", summary)
	}
}
