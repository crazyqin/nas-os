// Package smartcachetier 提供多级缓存智能管理功能
package smartcachetier

import (
	"path/filepath"
	"testing"
)

// ========== Manager 基础测试 ==========

func TestNewManager(t *testing.T) {
	mgr := NewManager(nil, "")
	if mgr == nil {
		t.Fatal("Manager should not be nil")
	}
}

func TestNewManager_WithConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cache-config.json")

	mgr := NewManager(nil, configPath)
	if mgr == nil {
		t.Fatal("Manager should not be nil")
	}
}

// ========== 层级管理测试 ==========

func TestCreateTier(t *testing.T) {
	mgr := NewManager(nil, "")

	req := TierCreateRequest{
		Level:      TierHDD,
		Name:       "HDD缓存",
		DevicePath: "/dev/sda1",
		TotalBytes: 1 << 40, // 1TB
		MaxEntries: 50000,
		Policy:     PolicyLRU,
	}

	tier, err := mgr.CreateTier(req)
	if err != nil {
		t.Fatalf("CreateTier failed: %v", err)
	}
	if tier.Level != TierHDD {
		t.Errorf("expected level %d, got %d", TierHDD, tier.Level)
	}
	if tier.Name != "HDD缓存" {
		t.Errorf("expected name HDD缓存, got %s", tier.Name)
	}
	if tier.Policy != PolicyLRU {
		t.Errorf("expected policy LRU, got %s", tier.Policy)
	}
}

func TestCreateTier_Duplicate(t *testing.T) {
	mgr := NewManager(nil, "")

	req := TierCreateRequest{
		Level:      TierSSD,
		Name:       "SSD缓存",
		DevicePath: "/dev/nvme0n1",
		TotalBytes: 500 << 30, // 500GB
	}

	_, err := mgr.CreateTier(req)
	if err != nil {
		t.Fatalf("first CreateTier failed: %v", err)
	}

	_, err = mgr.CreateTier(req)
	if err != ErrDuplicateTier {
		t.Errorf("expected ErrDuplicateTier, got: %v", err)
	}
}

func TestCreateTier_InvalidPolicy(t *testing.T) {
	mgr := NewManager(nil, "")

	req := TierCreateRequest{
		Level:      TierNVMe,
		Name:       "NVMe缓存",
		DevicePath: "/dev/nvme1n1",
		TotalBytes: 100 << 30,
		Policy:     CachePolicy("INVALID"),
	}

	_, err := mgr.CreateTier(req)
	if err != ErrInvalidPolicy {
		t.Errorf("expected ErrInvalidPolicy, got: %v", err)
	}
}

func TestGetTier(t *testing.T) {
	mgr := NewManager(nil, "")

	req := TierCreateRequest{
		Level:      TierSSD,
		Name:       "SSD缓存",
		DevicePath: "/dev/sdb1",
		TotalBytes: 200 << 30,
	}
	mgr.CreateTier(req)

	tier, err := mgr.GetTier(TierSSD)
	if err != nil {
		t.Fatalf("GetTier failed: %v", err)
	}
	if tier.Name != "SSD缓存" {
		t.Errorf("expected name SSD缓存, got %s", tier.Name)
	}
}

func TestGetTier_NotFound(t *testing.T) {
	mgr := NewManager(nil, "")

	_, err := mgr.GetTier(TierNVMe)
	if err != ErrCacheTierNotFound {
		t.Errorf("expected ErrCacheTierNotFound, got: %v", err)
	}
}

func TestListTiers(t *testing.T) {
	mgr := NewManager(nil, "")

	mgr.CreateTier(TierCreateRequest{Level: TierHDD, Name: "HDD", DevicePath: "/dev/sda", TotalBytes: 1 << 40})
	mgr.CreateTier(TierCreateRequest{Level: TierSSD, Name: "SSD", DevicePath: "/dev/sdb", TotalBytes: 500 << 30})
	mgr.CreateTier(TierCreateRequest{Level: TierNVMe, Name: "NVMe", DevicePath: "/dev/nvme0", TotalBytes: 100 << 30})

	tiers := mgr.ListTiers()
	if len(tiers) != 3 {
		t.Errorf("expected 3 tiers, got %d", len(tiers))
	}
}

func TestDeleteTier(t *testing.T) {
	mgr := NewManager(nil, "")

	mgr.CreateTier(TierCreateRequest{Level: TierHDD, Name: "HDD", DevicePath: "/dev/sda", TotalBytes: 1 << 40})

	err := mgr.DeleteTier(TierHDD)
	if err != nil {
		t.Fatalf("DeleteTier failed: %v", err)
	}

	_, err = mgr.GetTier(TierHDD)
	if err != ErrCacheTierNotFound {
		t.Errorf("expected ErrCacheTierNotFound after delete, got: %v", err)
	}
}

func TestDeleteTier_NotFound(t *testing.T) {
	mgr := NewManager(nil, "")

	err := mgr.DeleteTier(TierNVMe)
	if err != ErrCacheTierNotFound {
		t.Errorf("expected ErrCacheTierNotFound, got: %v", err)
	}
}

// ========== 缓存操作测试 ==========

func TestSet(t *testing.T) {
	mgr := NewManager(nil, "")

	mgr.CreateTier(TierCreateRequest{Level: TierHDD, Name: "HDD", DevicePath: "/dev/sda", TotalBytes: 1 << 40})

	entry, err := mgr.Set(CacheSetRequest{Key: "file-1", Size: 1024})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if entry.Key != "file-1" {
		t.Errorf("expected key file-1, got %s", entry.Key)
	}
	if entry.Tier != TierHDD {
		t.Errorf("expected tier %d, got %d", TierHDD, entry.Tier)
	}
}

func TestGet(t *testing.T) {
	mgr := NewManager(nil, "")

	mgr.CreateTier(TierCreateRequest{Level: TierHDD, Name: "HDD", DevicePath: "/dev/sda", TotalBytes: 1 << 40})
	mgr.Set(CacheSetRequest{Key: "file-1", Size: 1024})

	entry, err := mgr.Get("file-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if entry.HitCount != 1 {
		t.Errorf("expected hit count 1, got %d", entry.HitCount)
	}
}

func TestGet_NotFound(t *testing.T) {
	mgr := NewManager(nil, "")

	_, err := mgr.Get("nonexistent")
	if err != ErrCacheEntryNotFound {
		t.Errorf("expected ErrCacheEntryNotFound, got: %v", err)
	}
}

func TestDelete(t *testing.T) {
	mgr := NewManager(nil, "")

	mgr.CreateTier(TierCreateRequest{Level: TierHDD, Name: "HDD", DevicePath: "/dev/sda", TotalBytes: 1 << 40})
	mgr.Set(CacheSetRequest{Key: "file-1", Size: 1024})

	err := mgr.Delete("file-1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = mgr.Get("file-1")
	if err != ErrCacheEntryNotFound {
		t.Errorf("expected ErrCacheEntryNotFound after delete, got: %v", err)
	}
}

func TestSetToTier(t *testing.T) {
	mgr := NewManager(nil, "")

	mgr.CreateTier(TierCreateRequest{Level: TierHDD, Name: "HDD", DevicePath: "/dev/sda", TotalBytes: 1 << 40})
	mgr.CreateTier(TierCreateRequest{Level: TierSSD, Name: "SSD", DevicePath: "/dev/sdb", TotalBytes: 500 << 30})

	entry, err := mgr.SetToTier(CacheSetRequest{Key: "hot-file", Size: 4096}, TierSSD)
	if err != nil {
		t.Fatalf("SetToTier failed: %v", err)
	}
	if entry.Tier != TierSSD {
		t.Errorf("expected tier %d, got %d", TierSSD, entry.Tier)
	}
}

// ========== 分层操作测试 ==========

func TestPromoteEntry(t *testing.T) {
	mgr := NewManager(nil, "")

	mgr.CreateTier(TierCreateRequest{Level: TierHDD, Name: "HDD", DevicePath: "/dev/sda", TotalBytes: 1 << 40})
	mgr.CreateTier(TierCreateRequest{Level: TierSSD, Name: "SSD", DevicePath: "/dev/sdb", TotalBytes: 500 << 30})

	mgr.Set(CacheSetRequest{Key: "hot-file", Size: 1024})

	err := mgr.PromoteEntry("hot-file")
	if err != nil {
		t.Fatalf("PromoteEntry failed: %v", err)
	}

	entry, _ := mgr.Get("hot-file")
	if entry.Tier != TierSSD {
		t.Errorf("expected tier %d after promote, got %d", TierSSD, entry.Tier)
	}
}

func TestPromoteEntry_AlreadyTop(t *testing.T) {
	mgr := NewManager(nil, "")

	mgr.CreateTier(TierCreateRequest{Level: TierNVMe, Name: "NVMe", DevicePath: "/dev/nvme0", TotalBytes: 100 << 30})
	mgr.SetToTier(CacheSetRequest{Key: "file", Size: 1024}, TierNVMe)

	err := mgr.PromoteEntry("file")
	if err != nil {
		t.Fatalf("PromoteEntry on top tier should not error: %v", err)
	}
}

func TestDemoteEntry(t *testing.T) {
	mgr := NewManager(nil, "")

	mgr.CreateTier(TierCreateRequest{Level: TierHDD, Name: "HDD", DevicePath: "/dev/sda", TotalBytes: 1 << 40})
	mgr.CreateTier(TierCreateRequest{Level: TierSSD, Name: "SSD", DevicePath: "/dev/sdb", TotalBytes: 500 << 30})

	mgr.SetToTier(CacheSetRequest{Key: "cold-file", Size: 1024}, TierSSD)

	err := mgr.DemoteEntry("cold-file")
	if err != nil {
		t.Fatalf("DemoteEntry failed: %v", err)
	}

	entry, _ := mgr.Get("cold-file")
	if entry.Tier != TierHDD {
		t.Errorf("expected tier %d after demote, got %d", TierHDD, entry.Tier)
	}
}

func TestPromoteEntry_NotFound(t *testing.T) {
	mgr := NewManager(nil, "")

	err := mgr.PromoteEntry("nonexistent")
	if err != ErrCacheEntryNotFound {
		t.Errorf("expected ErrCacheEntryNotFound, got: %v", err)
	}
}

func TestDemoteEntry_NotFound(t *testing.T) {
	mgr := NewManager(nil, "")

	err := mgr.DemoteEntry("nonexistent")
	if err != ErrCacheEntryNotFound {
		t.Errorf("expected ErrCacheEntryNotFound, got: %v", err)
	}
}

func TestRunAutoTiering(t *testing.T) {
	mgr := NewManager(nil, "")

	mgr.CreateTier(TierCreateRequest{Level: TierHDD, Name: "HDD", DevicePath: "/dev/sda", TotalBytes: 1 << 40})
	mgr.CreateTier(TierCreateRequest{Level: TierSSD, Name: "SSD", DevicePath: "/dev/sdb", TotalBytes: 500 << 30})
	mgr.CreateTier(TierCreateRequest{Level: TierNVMe, Name: "NVMe", DevicePath: "/dev/nvme0", TotalBytes: 100 << 30})

	// 设置条目并增加命中次数
	mgr.Set(CacheSetRequest{Key: "hot-file", Size: 1024})
	for i := 0; i < 15; i++ {
		mgr.Get("hot-file")
	}

	promoted, demoted := mgr.RunAutoTiering()
	if promoted != 1 {
		t.Errorf("expected 1 promoted, got %d", promoted)
	}
	_ = demoted
}

// ========== 统计测试 ==========

func TestGetStats(t *testing.T) {
	mgr := NewManager(nil, "")

	mgr.CreateTier(TierCreateRequest{Level: TierHDD, Name: "HDD", DevicePath: "/dev/sda", TotalBytes: 1 << 40})
	mgr.Set(CacheSetRequest{Key: "file-1", Size: 1024})
	mgr.Set(CacheSetRequest{Key: "file-2", Size: 2048})
	mgr.Get("file-1")
	mgr.Get("file-1")

	stats := mgr.GetStats()
	if stats.TotalEntries != 2 {
		t.Errorf("expected 2 entries, got %d", stats.TotalEntries)
	}
	if stats.TotalHitCount != 2 {
		t.Errorf("expected 2 hits, got %d", stats.TotalHitCount)
	}
}

// ========== 配置测试 ==========

func TestGetConfig(t *testing.T) {
	mgr := NewManager(nil, "")

	config := mgr.GetConfig()
	if config == nil {
		t.Fatal("config should not be nil")
	}
	if config.DefaultPolicy != PolicyLRU {
		t.Errorf("expected default policy LRU, got %s", config.DefaultPolicy)
	}
}

func TestUpdateConfig(t *testing.T) {
	mgr := NewManager(nil, "")

	newConfig := &CacheConfig{
		DefaultPolicy:     PolicyLFU,
		EnableAutoTiering: false,
	}

	err := mgr.UpdateConfig(newConfig)
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	config := mgr.GetConfig()
	if config.DefaultPolicy != PolicyLFU {
		t.Errorf("expected policy LFU after update, got %s", config.DefaultPolicy)
	}
}

func TestUpdateConfig_InvalidPolicy(t *testing.T) {
	mgr := NewManager(nil, "")

	newConfig := &CacheConfig{
		DefaultPolicy: CachePolicy("INVALID"),
	}

	err := mgr.UpdateConfig(newConfig)
	if err != ErrInvalidPolicy {
		t.Errorf("expected ErrInvalidPolicy, got: %v", err)
	}
}
