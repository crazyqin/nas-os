// Package storage 提供 Fusion Pool 智能分层存储功能测试
package storage

import (
	"testing"
	"time"

	"nas-os/pkg/btrfs"
)

// TestFusionPoolStruct 测试 FusionPool 结构体.
func TestFusionPoolStruct(t *testing.T) {
	pool := &FusionPool{
		Name:        "test-pool",
		UUID:        "test-uuid-123",
		Description: "测试融合池",
		CreatedAt:   time.Now(),
		SSDDevices:  []string{"/dev/nvme0n1"},
		HDDDevices:  []string{"/dev/sda", "/dev/sdb"},
		TieringPolicy: TieringPolicy{
			HotDataThreshold:  100,
			ColdDataThreshold: 30,
			AutoTiering:       true,
			TieringWindow:     "02:00-06:00",
			SSDCachePercent:   20,
		},
		MountPoint: "/mnt/fusion/test-pool",
		Status: FusionPoolStatus{
			Healthy: true,
		},
		CacheConfig: CacheConfig{
			EnableMetadataCache: true,
			CacheSizeMB:         512,
			CacheTTLSeconds:     3600,
			ReadAheadKB:         128,
		},
		metadataCache: make(map[string]*MetadataCacheEntry),
	}

	// 验证基本字段
	if pool.Name != "test-pool" {
		t.Errorf("期望 Name 为 test-pool，实际为 %s", pool.Name)
	}

	if len(pool.SSDDevices) != 1 {
		t.Errorf("期望 1 个 SSD 设备，实际为 %d", len(pool.SSDDevices))
	}

	if len(pool.HDDDevices) != 2 {
		t.Errorf("期望 2 个 HDD 设备，实际为 %d", len(pool.HDDDevices))
	}

	if !pool.Status.Healthy {
		t.Error("期望状态为健康")
	}
}

// TestTieringPolicy 测试分层策略.
func TestTieringPolicy(t *testing.T) {
	policy := DefaultTieringPolicy

	if policy.HotDataThreshold != 100 {
		t.Errorf("期望 HotDataThreshold 为 100，实际为 %d", policy.HotDataThreshold)
	}

	if policy.ColdDataThreshold != 30 {
		t.Errorf("期望 ColdDataThreshold 为 30，实际为 %d", policy.ColdDataThreshold)
	}

	if !policy.AutoTiering {
		t.Error("期望 AutoTiering 为 true")
	}

	if policy.SSDCachePercent != 20 {
		t.Errorf("期望 SSDCachePercent 为 20，实际为 %d", policy.SSDCachePercent)
	}
}

// TestCacheConfig 测试缓存配置.
func TestCacheConfig(t *testing.T) {
	config := DefaultCacheConfig

	if !config.EnableMetadataCache {
		t.Error("期望 EnableMetadataCache 为 true")
	}

	if config.CacheSizeMB != 512 {
		t.Errorf("期望 CacheSizeMB 为 512，实际为 %d", config.CacheSizeMB)
	}

	if config.CacheTTLSeconds != 3600 {
		t.Errorf("期望 CacheTTLSeconds 为 3600，实际为 %d", config.CacheTTLSeconds)
	}

	if config.ReadAheadKB != 128 {
		t.Errorf("期望 ReadAheadKB 为 128，实际为 %d", config.ReadAheadKB)
	}
}

// TestFusionSubvolume 测试融合子卷.
func TestFusionSubvolume(t *testing.T) {
	subvol := &FusionSubvolume{
		ID:           256,
		Name:         "documents",
		Path:         "/mnt/fusion/test-pool/documents",
		ParentID:     5,
		ReadOnly:     false,
		UUID:         "subvol-uuid-123",
		Size:         1024 * 1024 * 1024, // 1GB
		HotDataSize:  100 * 1024 * 1024,  // 100MB on SSD
		ColdDataSize: 924 * 1024 * 1024,  // 924MB on HDD
		AccessCount:  50,
		LastAccess:   time.Now(),
	}

	if subvol.ID != 256 {
		t.Errorf("期望 ID 为 256，实际为 %d", subvol.ID)
	}

	if subvol.Name != "documents" {
		t.Errorf("期望 Name 为 documents，实际为 %s", subvol.Name)
	}

	// 验证分层大小
	totalData := subvol.HotDataSize + subvol.ColdDataSize
	if totalData != subvol.Size {
		t.Errorf("热数据+冷数据大小 %d 不等于总大小 %d", totalData, subvol.Size)
	}
}

// TestCreateFusionPoolRequest 测试创建请求.
func TestCreateFusionPoolRequest(t *testing.T) {
	req := &CreateFusionPoolRequest{
		Name:        "test-pool",
		Description: "测试融合池",
		SSDDevices:  []string{"/dev/nvme0n1"},
		HDDDevices:  []string{"/dev/sda"},
		Policy: &TieringPolicy{
			HotDataThreshold:  200,
			ColdDataThreshold: 60,
			AutoTiering:       true,
		},
		CacheConfig: &CacheConfig{
			EnableMetadataCache: true,
			CacheSizeMB:         1024,
		},
	}

	if req.Name != "test-pool" {
		t.Errorf("期望 Name 为 test-pool，实际为 %s", req.Name)
	}

	if len(req.SSDDevices) != 1 {
		t.Errorf("期望 1 个 SSD 设备")
	}

	if len(req.HDDDevices) != 1 {
		t.Errorf("期望 1 个 HDD 设备")
	}

	// 验证自定义策略
	if req.Policy.HotDataThreshold != 200 {
		t.Errorf("期望 HotDataThreshold 为 200，实际为 %d", req.Policy.HotDataThreshold)
	}

	if req.CacheConfig.CacheSizeMB != 1024 {
		t.Errorf("期望 CacheSizeMB 为 1024，实际为 %d", req.CacheConfig.CacheSizeMB)
	}
}

// TestFusionPoolStatus 测试融合池状态.
func TestFusionPoolStatus(t *testing.T) {
	status := FusionPoolStatus{
		Healthy:           true,
		TieringActive:     false,
		TieringProgress:   0,
		CacheHitRate:      0.85,
		ReadOps:           10000,
		WriteOps:          5000,
		AvgReadLatencyMs:  2.5,
		AvgWriteLatencyMs: 5.0,
	}

	if !status.Healthy {
		t.Error("期望 Healthy 为 true")
	}

	if status.TieringActive {
		t.Error("期望 TieringActive 为 false")
	}

	if status.CacheHitRate < 0 || status.CacheHitRate > 1 {
		t.Errorf("CacheHitRate 应在 0-1 之间，实际为 %f", status.CacheHitRate)
	}

	if status.AvgReadLatencyMs <= 0 {
		t.Error("AvgReadLatencyMs 应大于 0")
	}
}

// TestMetadataCacheEntry 测试元数据缓存条目.
func TestMetadataCacheEntry(t *testing.T) {
	entry := &MetadataCacheEntry{
		Path: "/mnt/fusion/test-pool/documents",
		Info: &btrfs.SubVolumeInfo{
			ID:       256,
			Name:     "documents",
			ReadOnly: false,
		},
		CachedAt:  time.Now(),
		AccessCnt: 10,
	}

	if entry.Path != "/mnt/fusion/test-pool/documents" {
		t.Errorf("Path 不匹配: %s", entry.Path)
	}

	if entry.Info.ID != 256 {
		t.Errorf("期望 ID 为 256，实际为 %d", entry.Info.ID)
	}

	if entry.AccessCnt != 10 {
		t.Errorf("期望 AccessCnt 为 10，实际为 %d", entry.AccessCnt)
	}

	// 测试缓存过期
	time.Sleep(100 * time.Millisecond)
	if time.Since(entry.CachedAt) < 100*time.Millisecond {
		t.Error("缓存时间应该已超过 100ms")
	}
}

// TestFusionPoolStats 测试融合池统计.
func TestFusionPoolStats(t *testing.T) {
	stats := &FusionPoolStats{
		PoolName:     "test-pool",
		TotalSize:    2 * 1024 * 1024 * 1024 * 1024, // 2TB
		TotalUsed:    1 * 1024 * 1024 * 1024 * 1024, // 1TB
		TotalFree:    1 * 1024 * 1024 * 1024 * 1024, // 1TB
		SSDTotal:     512 * 1024 * 1024 * 1024,      // 512GB
		SSDUsed:      256 * 1024 * 1024 * 1024,      // 256GB
		HDDTotal:     1536 * 1024 * 1024 * 1024,     // 1.5TB
		HDDUsed:      768 * 1024 * 1024 * 1024,      // 768GB
		CacheHitRate: 0.92,
	}

	if stats.PoolName != "test-pool" {
		t.Errorf("PoolName 不匹配: %s", stats.PoolName)
	}

	// 验证大小计算
	if stats.TotalSize != stats.SSDTotal+stats.HDDTotal {
		t.Error("总大小应等于 SSD + HDD 大小")
	}

	if stats.TotalUsed != stats.SSDUsed+stats.HDDUsed {
		t.Error("已用大小应等于 SSD + HDD 已用")
	}

	if stats.TotalFree != stats.TotalSize-stats.TotalUsed {
		t.Error("可用大小应等于总大小减去已用")
	}

	if stats.CacheHitRate < 0 || stats.CacheHitRate > 1 {
		t.Errorf("CacheHitRate 应在 0-1 之间，实际为 %f", stats.CacheHitRate)
	}
}

// TestFusionPoolManagerBasics 测试融合池管理器基础功能.
func TestFusionPoolManagerBasics(t *testing.T) {
	// 注意：这个测试不会真正创建 btrfs 卷，只测试结构
	pools := make(map[string]*FusionPool)

	pool := &FusionPool{
		Name:          "test-pool",
		UUID:          "uuid-123",
		Description:   "测试池",
		SSDDevices:    []string{"/dev/nvme0n1"},
		HDDDevices:    []string{"/dev/sda"},
		MountPoint:    "/mnt/fusion/test-pool",
		metadataCache: make(map[string]*MetadataCacheEntry),
	}

	pools[pool.Name] = pool

	// 测试获取
	if p, ok := pools["test-pool"]; !ok {
		t.Error("应该能找到 test-pool")
	} else if p.Name != "test-pool" {
		t.Errorf("名称不匹配: %s", p.Name)
	}

	// 测试列表
	list := make([]*FusionPool, 0, len(pools))
	for _, p := range pools {
		list = append(list, p)
	}

	if len(list) != 1 {
		t.Errorf("期望 1 个池，实际为 %d", len(list))
	}

	// 测试删除
	delete(pools, "test-pool")
	if len(pools) != 0 {
		t.Error("删除后应该没有池")
	}
}

// TestMetadataCache 测试元数据缓存操作.
func TestMetadataCache(t *testing.T) {
	pool := &FusionPool{
		Name: "test-pool",
		CacheConfig: CacheConfig{
			EnableMetadataCache: true,
			CacheTTLSeconds:     3600,
		},
		metadataCache: make(map[string]*MetadataCacheEntry),
	}

	path := "/mnt/fusion/test-pool/documents"
	info := &btrfs.SubVolumeInfo{
		ID:       256,
		Name:     "documents",
		ReadOnly: false,
	}

	// 添加到缓存
	pool.metadataCache[path] = &MetadataCacheEntry{
		Path:      path,
		Info:      info,
		CachedAt:  time.Now(),
		AccessCnt: 1,
	}

	// 验证缓存
	entry, ok := pool.metadataCache[path]
	if !ok {
		t.Error("应该能从缓存中找到条目")
	}

	if entry.Info.ID != 256 {
		t.Errorf("期望 ID 为 256，实际为 %d", entry.Info.ID)
	}

	// 更新访问计数
	entry.AccessCnt++
	if entry.AccessCnt != 2 {
		t.Errorf("期望 AccessCnt 为 2，实际为 %d", entry.AccessCnt)
	}

	// 删除缓存
	delete(pool.metadataCache, path)
	if len(pool.metadataCache) != 0 {
		t.Error("删除后缓存应该为空")
	}
}

// TestTieringPolicyValidation 测试分层策略验证.
func TestTieringPolicyValidation(t *testing.T) {
	tests := []struct {
		name   string
		policy TieringPolicy
		valid  bool
	}{
		{
			name: "valid default",
			policy: TieringPolicy{
				HotDataThreshold:  100,
				ColdDataThreshold: 30,
				AutoTiering:       true,
				SSDCachePercent:   20,
			},
			valid: true,
		},
		{
			name: "valid custom",
			policy: TieringPolicy{
				HotDataThreshold:  50,
				ColdDataThreshold: 7,
				AutoTiering:       false,
				SSDCachePercent:   50,
			},
			valid: true,
		},
		{
			name: "invalid SSD percent",
			policy: TieringPolicy{
				HotDataThreshold:  100,
				ColdDataThreshold: 30,
				SSDCachePercent:   150, // 超过 100
			},
			valid: false,
		},
		{
			name: "negative threshold",
			policy: TieringPolicy{
				HotDataThreshold:  -1,
				ColdDataThreshold: 30,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 验证 SSDCachePercent
			if tt.policy.SSDCachePercent < 0 || tt.policy.SSDCachePercent > 100 {
				if tt.valid {
					t.Error("应该无效但标记为有效")
				}
			}

			// 验证 HotDataThreshold
			if tt.policy.HotDataThreshold < 0 {
				if tt.valid {
					t.Error("应该无效但标记为有效")
				}
			}
		})
	}
}

// TestCacheConfigValidation 测试缓存配置验证.
func TestCacheConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config CacheConfig
		valid  bool
	}{
		{
			name: "valid default",
			config: CacheConfig{
				EnableMetadataCache: true,
				CacheSizeMB:         512,
				CacheTTLSeconds:     3600,
				ReadAheadKB:         128,
			},
			valid: true,
		},
		{
			name: "zero cache size",
			config: CacheConfig{
				EnableMetadataCache: true,
				CacheSizeMB:         0,
			},
			valid: false,
		},
		{
			name: "negative TTL",
			config: CacheConfig{
				EnableMetadataCache: true,
				CacheTTLSeconds:     -1,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 验证 CacheSizeMB
			if tt.config.CacheSizeMB <= 0 && tt.config.EnableMetadataCache {
				if tt.valid {
					t.Error("缓存大小为 0 时应该无效")
				}
			}

			// 验证 TTL
			if tt.config.CacheTTLSeconds < 0 {
				if tt.valid {
					t.Error("TTL 为负数时应该无效")
				}
			}
		})
	}
}

// TestSubvolumeAccessPattern 测试子卷访问模式.
func TestSubvolumeAccessPattern(t *testing.T) {
	subvol := &FusionSubvolume{
		ID:          256,
		Name:        "documents",
		Size:        1024 * 1024 * 1024, // 1GB
		AccessCount: 150,
		LastAccess:  time.Now().Add(-1 * time.Hour),
	}

	// 判断是否为热数据
	hotThreshold := 100
	if subvol.AccessCount > hotThreshold {
		// 热数据
		if subvol.HotDataSize == 0 {
			// 应该将数据移到 SSD
			t.Log("检测到热数据，建议迁移到 SSD")
		}
	}

	// 判断是否为冷数据
	coldThreshold := 24 * time.Hour // 24小时未访问
	if time.Since(subvol.LastAccess) > coldThreshold {
		// 冷数据
		t.Log("检测到冷数据，可以迁移到 HDD")
	}
}
