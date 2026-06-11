// Package hybridpoolmgr 混合存储池类型测试
package hybridpoolmgr

import (
	"testing"
	"time"
)

// TestDeviceTier 测试设备层级常量.
func TestDeviceTier(t *testing.T) {
	if TierNVMe != "nvme" {
		t.Errorf("期望 TierNVMe 为 'nvme'，实际为 '%s'", TierNVMe)
	}
	if TierSSD != "ssd" {
		t.Errorf("期望 TierSSD 为 'ssd'，实际为 '%s'", TierSSD)
	}
	if TierHDD != "hdd" {
		t.Errorf("期望 TierHDD 为 'hdd'，实际为 '%s'", TierHDD)
	}
}

// TestPoolStatus 测试池状态常量.
func TestPoolStatus(t *testing.T) {
	statuses := []struct {
		status PoolStatus
		expect string
	}{
		{PoolStatusOnline, "online"},
		{PoolStatusDegraded, "degraded"},
		{PoolStatusFaulted, "faulted"},
		{PoolStatusOffline, "offline"},
	}
	for _, s := range statuses {
		if string(s.status) != s.expect {
			t.Errorf("期望 %s 为 '%s'，实际为 '%s'", s.expect, s.expect, s.status)
		}
	}
}

// TestAlertLevel 测试告警级别常量.
func TestAlertLevel(t *testing.T) {
	levels := []struct {
		level  AlertLevel
		expect string
	}{
		{AlertLevelInfo, "info"},
		{AlertLevelWarning, "warning"},
		{AlertLevelCritical, "critical"},
	}
	for _, l := range levels {
		if string(l.level) != l.expect {
			t.Errorf("期望 AlertLevel 为 '%s'，实际为 '%s'", l.expect, l.level)
		}
	}
}

// TestStorageDevice 测试存储设备结构体.
func TestStorageDevice(t *testing.T) {
	dev := &StorageDevice{
		Path:        "/dev/nvme0n1",
		Name:        "nvme0n1",
		Tier:        TierNVMe,
		Model:       "Samsung 990 Pro",
		Serial:      "S12345",
		TotalBytes:  1024 * 1024 * 1024 * 1024,
		UsedBytes:   512 * 1024 * 1024 * 1024,
		FreeBytes:   512 * 1024 * 1024 * 1024,
		Healthy:     true,
		Temperature: 42,
		WearLevel:   5,
		AddedAt:     time.Now(),
	}

	if dev.Path != "/dev/nvme0n1" {
		t.Errorf("期望 Path 为 '/dev/nvme0n1'，实际为 '%s'", dev.Path)
	}
	if dev.Tier != TierNVMe {
		t.Errorf("期望 Tier 为 '%s'，实际为 '%s'", TierNVMe, dev.Tier)
	}
	if dev.TotalBytes != 1024*1024*1024*1024 {
		t.Errorf("期望 TotalBytes 为 1TB，实际为 %d", dev.TotalBytes)
	}
	if !dev.Healthy {
		t.Error("期望 Healthy 为 true")
	}
}

// TestHybridPool 测试混合池结构体.
func TestHybridPool(t *testing.T) {
	pool := &HybridPool{
		Name:        "test-pool",
		UUID:        "test-uuid",
		Description: "测试池",
		CreatedAt:   time.Now(),
		NVMEDevices: []*StorageDevice{{Path: "/dev/nvme0n1", Tier: TierNVMe, TotalBytes: 1024 * 1024 * 1024 * 1024}},
		SSDDevices:  []*StorageDevice{{Path: "/dev/sda", Tier: TierSSD, TotalBytes: 2 * 1024 * 1024 * 1024 * 1024}},
		HDDDevices:  []*StorageDevice{{Path: "/dev/sdb", Tier: TierHDD, TotalBytes: 8 * 1024 * 1024 * 1024 * 1024}},
		TotalBytes:  11 * 1024 * 1024 * 1024 * 1024,
		Status:      PoolStatusOnline,
		Healthy:     true,
	}

	if pool.Name != "test-pool" {
		t.Errorf("期望 Name 为 'test-pool'，实际为 '%s'", pool.Name)
	}
	if len(pool.NVMEDevices) != 1 {
		t.Errorf("期望 1 个 NVMe 设备，实际为 %d", len(pool.NVMEDevices))
	}
	if len(pool.SSDDevices) != 1 {
		t.Errorf("期望 1 个 SSD 设备，实际为 %d", len(pool.SSDDevices))
	}
	if len(pool.HDDDevices) != 1 {
		t.Errorf("期望 1 个 HDD 设备，实际为 %d", len(pool.HDDDevices))
	}
	if pool.Status != PoolStatusOnline {
		t.Errorf("期望 Status 为 '%s'，实际为 '%s'", PoolStatusOnline, pool.Status)
	}
}

// TestDefaultTieringConfig 测试默认分层配置.
func TestDefaultTieringConfig(t *testing.T) {
	cfg := DefaultTieringConfig

	if !cfg.Enabled {
		t.Error("期望 Enabled 为 true")
	}
	if cfg.HotThreshold != 1000 {
		t.Errorf("期望 HotThreshold 为 1000，实际为 %f", cfg.HotThreshold)
	}
	if cfg.WarmThreshold != 100 {
		t.Errorf("期望 WarmThreshold 为 100，实际为 %f", cfg.WarmThreshold)
	}
	if cfg.ColdAgeDays != 30 {
		t.Errorf("期望 ColdAgeDays 为 30，实际为 %d", cfg.ColdAgeDays)
	}
	if cfg.PromotePolicy != "moderate" {
		t.Errorf("期望 PromotePolicy 为 'moderate'，实际为 '%s'", cfg.PromotePolicy)
	}
	if cfg.DemotePolicy != "conservative" {
		t.Errorf("期望 DemotePolicy 为 'conservative'，实际为 '%s'", cfg.DemotePolicy)
	}
	if cfg.MaxPromoteMBps != 500 {
		t.Errorf("期望 MaxPromoteMBps 为 500，实际为 %d", cfg.MaxPromoteMBps)
	}
	if cfg.TieringWindow != "02:00-06:00" {
		t.Errorf("期望 TieringWindow 为 '02:00-06:00'，实际为 '%s'", cfg.TieringWindow)
	}
}

// TestDefaultRebalancePolicy 测试默认重平衡策略.
func TestDefaultRebalancePolicy(t *testing.T) {
	p := DefaultRebalancePolicy

	if !p.Enabled {
		t.Error("期望 Enabled 为 true")
	}
	if p.ThresholdPercent != 15.0 {
		t.Errorf("期望 ThresholdPercent 为 15.0，实际为 %f", p.ThresholdPercent)
	}
	if p.MaxMigrateMBps != 300 {
		t.Errorf("期望 MaxMigrateMBps 为 300，实际为 %d", p.MaxMigrateMBps)
	}
	if p.MinFreePercent != 10.0 {
		t.Errorf("期望 MinFreePercent 为 10.0，实际为 %f", p.MinFreePercent)
	}
	if p.ScheduleCron != "0 3 * * 0" {
		t.Errorf("期望 ScheduleCron 为 '0 3 * * 0'，实际为 '%s'", p.ScheduleCron)
	}
}

// TestBlockHeat 测试块热度结构体.
func TestBlockHeat(t *testing.T) {
	heat := &BlockHeat{
		BlockID:    "block-001",
		Path:       "/data/hot/file.dat",
		Tier:       TierSSD,
		Size:       4096,
		ReadCount:  100,
		WriteCount: 50,
		LastAccess: time.Now(),
		HeatScore:  75.5,
	}

	if heat.BlockID != "block-001" {
		t.Errorf("期望 BlockID 为 'block-001'，实际为 '%s'", heat.BlockID)
	}
	if heat.Tier != TierSSD {
		t.Errorf("期望 Tier 为 '%s'，实际为 '%s'", TierSSD, heat.Tier)
	}
	if heat.ReadCount != 100 {
		t.Errorf("期望 ReadCount 为 100，实际为 %d", heat.ReadCount)
	}
	if heat.HeatScore != 75.5 {
		t.Errorf("期望 HeatScore 为 75.5，实际为 %f", heat.HeatScore)
	}
}

// TestPoolHealth 测试池健康结构体.
func TestPoolHealth(t *testing.T) {
	health := &PoolHealth{
		PoolName: "test-pool",
		Status:   PoolStatusOnline,
		Healthy:  true,
		DeviceHealth: []*DeviceHealth{
			{
				Device:  "/dev/nvme0n1",
				Tier:    TierNVMe,
				Healthy: true,
			},
		},
		Alerts:        make([]*PoolAlert, 0),
		LastCheckTime: time.Now(),
		UptimeSeconds: 3600,
	}

	if health.PoolName != "test-pool" {
		t.Errorf("期望 PoolName 为 'test-pool'，实际为 '%s'", health.PoolName)
	}
	if !health.Healthy {
		t.Error("期望 Healthy 为 true")
	}
	if len(health.DeviceHealth) != 1 {
		t.Errorf("期望 1 个设备健康记录，实际为 %d", len(health.DeviceHealth))
	}
}

// TestCreatePoolRequest 测试创建池请求.
func TestCreatePoolRequest(t *testing.T) {
	req := &CreatePoolRequest{
		Name:        "my-pool",
		Description: "我的混合池",
		NVMEDevices: []string{"/dev/nvme0n1"},
		SSDDevices:  []string{"/dev/sda"},
		HDDDevices:  []string{"/dev/sdb", "/dev/sdc"},
	}

	if req.Name != "my-pool" {
		t.Errorf("期望 Name 为 'my-pool'，实际为 '%s'", req.Name)
	}
	if len(req.NVMEDevices) != 1 {
		t.Errorf("期望 1 个 NVMe 设备，实际为 %d", len(req.NVMEDevices))
	}
	if len(req.HDDDevices) != 2 {
		t.Errorf("期望 2 个 HDD 设备，实际为 %d", len(req.HDDDevices))
	}
}

// TestTierIOStats 测试层级 IO 统计.
func TestTierIOStats(t *testing.T) {
	stats := &TierIOStats{
		Tier:       TierNVMe,
		ReadOps:    1000,
		WriteOps:   500,
		ReadBytes:  4096000,
		WriteBytes: 2048000,
		ReadIOPS:   100.5,
		WriteIOPS:  50.3,
		AvgLatency: 15.2,
		UpdatedAt:  time.Now(),
	}

	if stats.Tier != TierNVMe {
		t.Errorf("期望 Tier 为 '%s'，实际为 '%s'", TierNVMe, stats.Tier)
	}
	if stats.ReadOps != 1000 {
		t.Errorf("期望 ReadOps 为 1000，实际为 %d", stats.ReadOps)
	}
}

// TestPoolAlert 测试池告警.
func TestPoolAlert(t *testing.T) {
	alert := &PoolAlert{
		ID:        "alert-001",
		PoolName:  "test-pool",
		Level:     AlertLevelWarning,
		Device:    "/dev/sda",
		Message:   "设备温度过高",
		CreatedAt: time.Now(),
		Resolved:  false,
	}

	if alert.ID != "alert-001" {
		t.Errorf("期望 ID 为 'alert-001'，实际为 '%s'", alert.ID)
	}
	if alert.Level != AlertLevelWarning {
		t.Errorf("期望 Level 为 '%s'，实际为 '%s'", AlertLevelWarning, alert.Level)
	}
	if alert.Resolved {
		t.Error("期望 Resolved 为 false")
	}
}
