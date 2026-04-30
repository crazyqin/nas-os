package hybridpool

import (
	"testing"
	"time"
)

func TestNewHybridPool(t *testing.T) {
	config := HybridPoolConfig{
		Name: "test-pool",
		Tiers: []PoolTier{
			{Level: TierHot, DevicePaths: []string{"/dev/nvme0n1"}, Capacity: 500 * 1024 * 1024 * 1024, Role: "data"},
			{Level: TierCold, DevicePaths: []string{"/dev/sda"}, Capacity: 4 * 1024 * 1024 * 1024 * 1024, Role: "data"},
		},
		PromoteThreshold:  70,
		DemoteThreshold:   30,
		TieringInterval:   5 * time.Minute,
		MinFileSize:       1024 * 1024, // 1MB
		EnableAutoTiering: true,
	}
	pool := NewHybridPool(config)
	if pool == nil {
		t.Fatal("NewHybridPool returned nil")
	}
	if pool.config.Name != "test-pool" {
		t.Errorf("expected name 'test-pool', got %s", pool.config.Name)
	}
	if len(pool.tiers) != 2 {
		t.Errorf("expected 2 tiers, got %d", len(pool.tiers))
	}
}

func TestRecordAccess(t *testing.T) {
	config := HybridPoolConfig{
		Name:              "test-pool",
		PromoteThreshold:  70,
		DemoteThreshold:   30,
		EnableAutoTiering: false,
	}
	pool := NewHybridPool(config)

	// Record multiple accesses to increase heat
	for i := 0; i < 20; i++ {
		pool.RecordAccess("/data/hot-file.dat", 1024*1024, 0)
	}

	heatmap := pool.GetHeatMap()
	heat, exists := heatmap["/data/hot-file.dat"]
	if !exists {
		t.Fatal("expected heat entry for hot-file.dat")
	}
	if heat.AccessCount != 20 {
		t.Errorf("expected 20 accesses, got %d", heat.AccessCount)
	}
	if heat.HeatScore <= 0 {
		t.Errorf("expected positive heat score, got %f", heat.HeatScore)
	}
}

func TestDetermineTier(t *testing.T) {
	config := HybridPoolConfig{
		PromoteThreshold: 70,
		DemoteThreshold:  30,
	}
	pool := NewHybridPool(config)

	tests := []struct {
		score    float64
		expected TierLevel
	}{
		{90, TierHot},
		{70, TierHot},
		{50, TierWarm},
		{30, TierWarm},
		{10, TierCold},
	}

	for _, tt := range tests {
		heat := &FileHeatMap{HeatScore: tt.score}
		result := pool.determineTier(heat)
		if result != tt.expected {
			t.Errorf("score %f: expected %s, got %s", tt.score, tt.expected, result)
		}
	}
}

func TestGetStats(t *testing.T) {
	config := HybridPoolConfig{
		EnableAutoTiering: false,
	}
	pool := NewHybridPool(config)

	// Add some files to different tiers
	pool.mu.Lock()
	pool.heatMap["/a"] = &FileHeatMap{CurrentTier: TierHot, HeatScore: 90}
	pool.heatMap["/b"] = &FileHeatMap{CurrentTier: TierWarm, HeatScore: 50}
	pool.heatMap["/c"] = &FileHeatMap{CurrentTier: TierCold, HeatScore: 10}
	pool.mu.Unlock()

	stats := pool.GetStats()
	if stats.TotalFiles != 3 {
		t.Errorf("expected 3 total files, got %d", stats.TotalFiles)
	}
	if stats.HotFiles != 1 {
		t.Errorf("expected 1 hot file, got %d", stats.HotFiles)
	}
}
