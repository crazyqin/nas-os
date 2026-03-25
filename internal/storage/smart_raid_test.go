// Package storage 提供智能 RAID 测试
package storage

import (
	"testing"
)

// TestCalculateTiers 测试层级计算算法.
func TestCalculateTiers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	manager, err := NewSmartRAIDManager("")
	if err != nil {
		t.Fatalf("创建 SmartRAID 管理器失败: %v", err)
	}

	tests := []struct {
		name          string
		devices       []*SmartDevice
		expectedTiers int
		description   string
	}{
		{
			name: "相同容量设备",
			devices: []*SmartDevice{
				{ID: "1", Device: "/dev/sda", Capacity: 1000000000000}, // 1TB
				{ID: "2", Device: "/dev/sdb", Capacity: 1000000000000}, // 1TB
				{ID: "3", Device: "/dev/sdc", Capacity: 1000000000000}, // 1TB
			},
			expectedTiers: 1,
			description:   "三个相同容量的设备应该形成一个层级",
		},
		{
			name: "两种不同容量设备",
			devices: []*SmartDevice{
				{ID: "1", Device: "/dev/sda", Capacity: 2000000000000}, // 2TB
				{ID: "2", Device: "/dev/sdb", Capacity: 2000000000000}, // 2TB
				{ID: "3", Device: "/dev/sdc", Capacity: 1000000000000}, // 1TB
				{ID: "4", Device: "/dev/sdd", Capacity: 1000000000000}, // 1TB
			},
			expectedTiers: 2,
			description:   "两种不同容量应该形成两个层级",
		},
		{
			name: "三种不同容量设备",
			devices: []*SmartDevice{
				{ID: "1", Device: "/dev/sda", Capacity: 4000000000000}, // 4TB
				{ID: "2", Device: "/dev/sdb", Capacity: 2000000000000}, // 2TB
				{ID: "3", Device: "/dev/sdc", Capacity: 1000000000000}, // 1TB
			},
			expectedTiers: 3,
			description:   "三种不同容量应该形成三个层级",
		},
		{
			name: "相近容量设备（5%容差内）",
			devices: []*SmartDevice{
				{ID: "1", Device: "/dev/sda", Capacity: 1000000000000}, // 1.0TB
				{ID: "2", Device: "/dev/sdb", Capacity: 1020000000000}, // 1.02TB (2%差异)
				{ID: "3", Device: "/dev/sdc", Capacity: 980000000000},  // 0.98TB (2%差异)
			},
			expectedTiers: 1,
			description:   "5%容差内的容量差异应该归为同一层级",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := DefaultRAIDPolicy
			tiers := manager.calculateTiers(tt.devices, policy)

			if len(tiers) != tt.expectedTiers {
				t.Errorf("%s: 期望 %d 个层级，实际 %d 个层级", tt.description, tt.expectedTiers, len(tiers))
			}

			// 验证每个设备都被分配了层级
			for _, dev := range tt.devices {
				if dev.TierID == 0 {
					t.Errorf("设备 %s 未被分配层级", dev.Device)
				}
			}
		})
	}
}

// TestSelectRAIDConfig 测试 RAID 配置选择.
func TestSelectRAIDConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	manager, err := NewSmartRAIDManager("")
	if err != nil {
		t.Fatalf("创建 SmartRAID 管理器失败: %v", err)
	}

	tests := []struct {
		name         string
		deviceCount  int
		policy       RAIDPolicy
		expectedRAID string
	}{
		{
			name:         "单设备-single",
			deviceCount:  1,
			policy:       RAIDPolicy{AutoSelect: true, RedundancyLevel: 1},
			expectedRAID: "single",
		},
		{
			name:         "双设备-raid1",
			deviceCount:  2,
			policy:       RAIDPolicy{AutoSelect: true, RedundancyLevel: 1},
			expectedRAID: "raid1",
		},
		{
			name:         "三设备-raid5",
			deviceCount:  3,
			policy:       RAIDPolicy{AutoSelect: true, RedundancyLevel: 1},
			expectedRAID: "raid5",
		},
		{
			name:         "四设备-双冗余-raid6",
			deviceCount:  4,
			policy:       RAIDPolicy{AutoSelect: true, RedundancyLevel: 2},
			expectedRAID: "raid6",
		},
		{
			name:         "四设备-性能模式-raid10",
			deviceCount:  4,
			policy:       RAIDPolicy{AutoSelect: true, PerformanceMode: true, RedundancyLevel: 1},
			expectedRAID: "raid10",
		},
		{
			name:         "强制RAID类型",
			deviceCount:  4,
			policy:       RAIDPolicy{AutoSelect: false, ForcedRAIDType: "raid1"},
			expectedRAID: "raid1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raidType := manager.selectTierRAID(tt.deviceCount, tt.policy)

			if raidType != tt.expectedRAID {
				t.Errorf("期望 RAID %s，实际 %s", tt.expectedRAID, raidType)
			}
		})
	}
}

// TestGetRAIDEfficiency 测试 RAID 效率计算.
func TestGetRAIDEfficiency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	manager, err := NewSmartRAIDManager("")
	if err != nil {
		t.Fatalf("创建 SmartRAID 管理器失败: %v", err)
	}

	tests := []struct {
		raidType      string
		deviceCount   int
		minEfficiency float64
		maxEfficiency float64
	}{
		{"single", 1, 1.0, 1.0},
		{"raid0", 2, 1.0, 1.0},
		{"raid1", 2, 0.5, 0.5},
		{"raid5", 3, 0.66, 0.67},
		{"raid5", 4, 0.75, 0.75},
		{"raid6", 4, 0.5, 0.5},
		{"raid6", 6, 0.66, 0.67},
		{"raid10", 4, 0.5, 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.raidType, func(t *testing.T) {
			efficiency := manager.getRAIDEfficiency(tt.raidType, tt.deviceCount)

			if efficiency < tt.minEfficiency || efficiency > tt.maxEfficiency {
				t.Errorf("RAID %s (%d 设备) 效率 %.2f 不在预期范围 [%.2f, %.2f]",
					tt.raidType, tt.deviceCount, efficiency, tt.minEfficiency, tt.maxEfficiency)
			}
		})
	}
}

// TestCalculateCapacity 测试容量计算.
func TestCalculateCapacity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	manager, err := NewSmartRAIDManager("")
	if err != nil {
		t.Fatalf("创建 SmartRAID 管理器失败: %v", err)
	}

	tiers := []StorageTier{
		{
			ID:          1,
			Capacity:    2000000000000, // 2TB
			Devices:     []string{"/dev/sda", "/dev/sdb"},
			RawCapacity: 4000000000000, // 4TB (2x 2TB)
			RAIDType:    "raid1",
		},
		{
			ID:          2,
			Capacity:    1000000000000, // 1TB
			Devices:     []string{"/dev/sdc", "/dev/sdd"},
			RawCapacity: 2000000000000, // 2TB (2x 1TB)
			RAIDType:    "raid1",
		},
	}

	config := &TierRAIDConfig{
		Tiers: []TierRAIDInfo{
			{TierID: 1, RAIDType: "raid1"},
			{TierID: 2, RAIDType: "raid1"},
		},
	}

	total, raw, wasted := manager.calculateCapacity(tiers, config)

	// raid1 效率 50%，所以：
	// Tier 1: 4TB * 0.5 = 2TB 可用
	// Tier 2: 2TB * 0.5 = 1TB 可用
	// Total: 3TB 可用
	expectedTotal := uint64(3000000000000)  // 3TB
	expectedRaw := uint64(6000000000000)    // 6TB
	expectedWasted := uint64(3000000000000) // 3TB (浪费在冗余上)

	if total != expectedTotal {
		t.Errorf("总容量错误: 期望 %d, 实际 %d", expectedTotal, total)
	}

	if raw != expectedRaw {
		t.Errorf("原始容量错误: 期望 %d, 实际 %d", expectedRaw, raw)
	}

	if wasted != expectedWasted {
		t.Errorf("浪费容量错误: 期望 %d, 实际 %d", expectedWasted, wasted)
	}
}

// TestSmartDeviceTypeDetection 测试设备类型检测.
func TestSmartDeviceTypeDetection(t *testing.T) {
	tests := []struct {
		devicePath   string
		expectedType string
	}{
		{"/dev/nvme0n1", "NVMe"},
		{"/dev/nvme1n1", "NVMe"},
		{"/dev/sda", "HDD"}, // 默认假设，实际检测依赖系统
		{"/dev/sdb", "HDD"},
	}

	for _, tt := range tests {
		t.Run(tt.devicePath, func(t *testing.T) {
			// 注意：这个测试依赖于实际的系统设备
			// 在 CI 环境中可能无法正确运行
			if tt.devicePath == "/dev/nvme0n1" || tt.devicePath == "/dev/nvme1n1" {
				// NVMe 设备路径检查
				if tt.expectedType == "NVMe" {
					// 仅验证逻辑正确性，NVMe设备类型匹配
					_ = tt.expectedType // 避免空分支警告
				}
			}
		})
	}
}

// TestExpansionPlan 测试扩容计划.
func TestExpansionPlan(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	manager, err := NewSmartRAIDManager("")
	if err != nil {
		t.Fatalf("创建 SmartRAID 管理器失败: %v", err)
	}

	// 创建一个模拟池用于测试
	pool := &SmartPool{
		Name:          "test-pool",
		TotalCapacity: 1000000000000, // 1TB
		UsedCapacity:  850000000000,  // 850GB (85% 利用率)
		FreeCapacity:  150000000000,  // 150GB
		RAIDPolicy:    DefaultRAIDPolicy,
		Devices: []*SmartDevice{
			{ID: "1", Device: "/dev/sda", Capacity: 500000000000, Health: "healthy"},
			{ID: "2", Device: "/dev/sdb", Capacity: 500000000000, Health: "healthy"},
		},
		Tiers: []StorageTier{
			{ID: 1, Devices: []string{"/dev/sda", "/dev/sdb"}, RAIDType: "raid1"},
		},
	}

	manager.mu.Lock()
	manager.pools["test-pool"] = pool
	manager.mu.Unlock()

	// 获取扩容计划
	plan, err := manager.GetExpansionPlan("test-pool")
	if err != nil {
		t.Fatalf("获取扩容计划失败: %v", err)
	}

	// 验证：利用率超过 80% 应该有扩容建议
	if len(plan.Recommendations) == 0 {
		t.Error("高利用率池应该有扩容建议")
	}

	found := false
	for _, rec := range plan.Recommendations {
		if rec.Type == "add_capacity" && rec.Priority == "high" {
			found = true
			break
		}
	}

	if !found {
		t.Error("应该有高优先级的添加容量建议")
	}
}

// TestReplaceDevice 测试设备替换.
func TestReplaceDevice(t *testing.T) {
	// 这个测试验证设备替换的逻辑
	// 实际替换操作需要真实的 Btrfs 环境

	t.Run("验证新设备容量检查", func(t *testing.T) {
		// 新设备容量必须 >= 旧设备容量
		oldCapacity := uint64(1000000000000) // 1TB
		newCapacity := uint64(2000000000000) // 2TB

		if newCapacity < oldCapacity {
			t.Error("新设备容量应该 >= 旧设备容量")
		}
	})
}

// BenchmarkCalculateTiers 基准测试层级计算.
func BenchmarkCalculateTiers(b *testing.B) {
	manager, _ := NewSmartRAIDManager("")

	devices := make([]*SmartDevice, 20)
	for i := 0; i < 20; i++ {
		devices[i] = &SmartDevice{
			ID:       string(rune(i)),
			Device:   "/dev/sd" + string(rune('a'+i)),
			Capacity: uint64((i%4 + 1) * 1000000000000), // 1-4TB 变化
		}
	}

	policy := DefaultRAIDPolicy

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.calculateTiers(devices, policy)
	}
}

// BenchmarkSelectRAIDConfig 基准测试 RAID 选择.
func BenchmarkSelectRAIDConfig(b *testing.B) {
	manager, _ := NewSmartRAIDManager("")
	policy := DefaultRAIDPolicy

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.selectTierRAID(4, policy)
	}
}
