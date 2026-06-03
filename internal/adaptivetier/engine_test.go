package adaptivetier

import (
	"testing"
	"time"
)

func TestTierEngine(t *testing.T) {
	tmpDir := t.TempDir()
	engine := NewAdaptiveTierEngine(tmpDir)

	engine.RegisterFile("/data/hot-file.txt", TierHot, 1024*1024)
	engine.RegisterFile("/data/warm-file.txt", TierWarm, 2*1024*1024)

	files := engine.GetFiles(nil)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}

func TestAccessPattern(t *testing.T) {
	tmpDir := t.TempDir()
	engine := NewAdaptiveTierEngine(tmpDir)

	engine.RegisterFile("/data/test.txt", TierHot, 1024)

	for i := 0; i < 10; i++ {
		engine.RecordAccess("/data/test.txt")
		time.Sleep(10 * time.Millisecond)
	}

	files := engine.GetFiles(nil)
	if len(files) == 0 {
		t.Fatal("expected files")
	}
	if files[0].Pattern.AccessCount != 10 {
		t.Fatalf("expected 10 accesses, got %d", files[0].Pattern.AccessCount)
	}
}

func TestTierStats(t *testing.T) {
	tmpDir := t.TempDir()
	engine := NewAdaptiveTierEngine(tmpDir)

	engine.RegisterFile("/hot", TierHot, 1000)
	engine.RegisterFile("/warm", TierWarm, 2000)
	engine.RegisterFile("/cold", TierCold, 3000)

	stats := engine.GetStats()
	if stats.TotalFiles != 3 {
		t.Fatalf("expected 3 files, got %d", stats.TotalFiles)
	}
	if stats.HotSize != 1000 {
		t.Fatal("hot size mismatch")
	}
}

func TestCustomRule(t *testing.T) {
	tmpDir := t.TempDir()
	engine := NewAdaptiveTierEngine(tmpDir)

	engine.AddRule(&TierRule{
		ID:       "custom",
		Name:     "Custom Rule",
		FromTier: TierHot,
		ToTier:   TierCold,
		IdleDays: 7,
		Enabled:  true,
	})

	rules := engine.GetRules()
	found := false
	for _, r := range rules {
		if r.ID == "custom" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("custom rule not found")
	}
}
