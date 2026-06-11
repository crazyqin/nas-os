package hybridflash

import (
	"testing"
)

func TestCreatePoolHybrid(t *testing.T) {
	mgr := NewHybridFlashPoolManager()

	pool := FlashPool{
		ID:   "pool1",
		Name: "Main Pool",
		FlashVdevs: []VDev{
			{ID: "vdev1", Type: TierNVMe, SizeGB: 500, RAIDType: "mirror"},
		},
		HDDVdevs: []VDev{
			{ID: "vdev2", Type: TierHDD, SizeGB: 4000, RAIDType: "raidz1"},
		},
	}

	err := mgr.CreatePool(pool)
	if err != nil {
		t.Fatalf("CreatePool failed: %v", err)
	}

	got, err := mgr.GetPool("pool1")
	if err != nil {
		t.Fatalf("GetPool failed: %v", err)
	}
	if got.TotalFlashGB != 500 {
		t.Errorf("expected 500GB flash, got %d", got.TotalFlashGB)
	}
	if got.TotalHDDGB != 4000 {
		t.Errorf("expected 4000GB HDD, got %d", got.TotalHDDGB)
	}
	if got.Status != "online" {
		t.Errorf("expected status 'online', got %s", got.Status)
	}
}

func TestCreateDuplicatePoolHybrid(t *testing.T) {
	mgr := NewHybridFlashPoolManager()
	pool := FlashPool{ID: "pool1", Name: "Pool"}
	mgr.CreatePool(pool)

	err := mgr.CreatePool(pool)
	if err == nil {
		t.Error("should fail on duplicate pool")
	}
}

func TestDeletePoolHybrid(t *testing.T) {
	mgr := NewHybridFlashPoolManager()
	mgr.CreatePool(FlashPool{ID: "pool1", Name: "Pool"})

	err := mgr.DeletePool("pool1")
	if err != nil {
		t.Fatalf("DeletePool failed: %v", err)
	}

	_, err = mgr.GetPool("pool1")
	if err == nil {
		t.Error("pool should not exist after deletion")
	}
}

func TestBindDatasetHybrid(t *testing.T) {
	mgr := NewHybridFlashPoolManager()

	err := mgr.BindDataset(DatasetTierBinding{
		DatasetPath: "/data/hot",
		Tier:        TierNVMe,
		Priority:    PriorityHot,
	})
	if err != nil {
		t.Fatalf("BindDataset failed: %v", err)
	}

	tier, err := mgr.GetDatasetTier("/data/hot")
	if err != nil {
		t.Fatalf("GetDatasetTier failed: %v", err)
	}
	if tier != TierNVMe {
		t.Errorf("expected NVMe tier, got %s", tier)
	}
}

func TestDefaultTierHybrid(t *testing.T) {
	mgr := NewHybridFlashPoolManager()

	tier, err := mgr.GetDatasetTier("/unknown")
	if err != nil {
		t.Fatalf("GetDatasetTier failed: %v", err)
	}
	if tier != TierHDD {
		t.Errorf("expected default HDD tier, got %s", tier)
	}
}

func TestRecommendTierHybrid(t *testing.T) {
	mgr := NewHybridFlashPoolManager()

	tests := []struct {
		name      string
		sizeKB    int64
		freq      float64
		metadata  bool
		expected  TierType
	}{
		{"metadata", 100, 0, true, TierNVMe},
		{"small file", 100, 0.5, false, TierSSD},
		{"hot data", 1000, 20, false, TierNVMe},
		{"warm data", 1000, 2, false, TierSSD},
		{"cold data", 10000, 0.1, false, TierHDD},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mgr.RecommendTier(tt.sizeKB, tt.freq, tt.metadata)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestUpdateTieringPolicyHybrid(t *testing.T) {
	mgr := NewHybridFlashPoolManager()
	mgr.CreatePool(FlashPool{ID: "pool1", Name: "Pool"})

	policy := TieringPolicy{
		Enabled:         true,
		HotThreshold:    10.0,
		ColdThreshold:   1.0,
		MetadataOnFlash: true,
		SmallFileOnFlash: true,
		SmallFileMaxKB:  256,
	}

	err := mgr.UpdateTieringPolicy("pool1", policy)
	if err != nil {
		t.Fatalf("UpdateTieringPolicy failed: %v", err)
	}

	pool, _ := mgr.GetPool("pool1")
	if !pool.TieringPolicy.Enabled {
		t.Error("tiering policy should be enabled")
	}
}

func TestListPoolsHybrid(t *testing.T) {
	mgr := NewHybridFlashPoolManager()
	mgr.CreatePool(FlashPool{ID: "pool1", Name: "Pool 1"})
	mgr.CreatePool(FlashPool{ID: "pool2", Name: "Pool 2"})

	pools := mgr.ListPools()
	if len(pools) != 2 {
		t.Errorf("expected 2 pools, got %d", len(pools))
	}
}
