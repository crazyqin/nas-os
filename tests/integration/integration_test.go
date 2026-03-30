// Package integration 提供 NAS-OS 集成测试
// 测试各模块之间的交互和端到端功能
package integration

import (
	"fmt"
	"os"
	"testing"
	"time"

	"nas-os/internal/storage"
)

// TestMain 测试入口，管理测试生命周期.
func TestMain(m *testing.M) {
	// 设置测试环境
	setup()

	// 运行测试
	code := m.Run()

	// 清理测试环境
	teardown()

	os.Exit(code)
}

// setup 初始化测试环境.
func setup() {
	fmt.Println("🔧 初始化集成测试环境...")
}

// teardown 清理测试环境.
func teardown() {
	fmt.Println("🧹 清理集成测试环境...")
}

// ========== 配置测试 ==========

// TestRAIDConfigurations 测试 RAID 配置.
func TestRAIDConfigurations(t *testing.T) {
	configs := storage.RAIDConfigs

	tests := []struct {
		profile        string
		minDevices     int
		faultTolerance int
	}{
		{"single", 1, 0},
		{"raid0", 2, 0},
		{"raid1", 2, 1},
		{"raid10", 4, 1},
		{"raid5", 3, 1},
		{"raid6", 4, 2},
	}

	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			config, ok := configs[tt.profile]
			if !ok {
				t.Fatalf("RAID config %s not found", tt.profile)
			}

			if config.MinDevices != tt.minDevices {
				t.Errorf("Expected MinDevices=%d, got %d", tt.minDevices, config.MinDevices)
			}

			if config.FaultTolerance != tt.faultTolerance {
				t.Errorf("Expected FaultTolerance=%d, got %d", tt.faultTolerance, config.FaultTolerance)
			}
		})
	}
}

// TestSnapshotConfig 测试快照配置.
func TestSnapshotConfig(t *testing.T) {
	config := storage.DefaultSnapshotConfig

	if config.ReadOnly != true {
		t.Error("Expected DefaultSnapshotConfig.ReadOnly=true")
	}

	if config.TimeFormat != "20060102-150405" {
		t.Errorf("Expected TimeFormat='20060102-150405', got '%s'", config.TimeFormat)
	}
}

// ========== 数据结构测试 ==========

// TestVolumeStruct 测试 Volume 数据结构.
func TestVolumeStruct(t *testing.T) {
	vol := &storage.Volume{
		Name:        "data",
		UUID:        "test-uuid",
		Devices:     []string{"/dev/sda1", "/dev/sdb1"},
		Size:        2000000000000,
		Used:        1000000000000,
		Free:        1000000000000,
		DataProfile: "raid1",
		MetaProfile: "raid1",
		MountPoint:  "/mnt/data",
		Status: storage.VolumeStatus{
			Healthy: true,
		},
	}

	if vol.Name != "data" {
		t.Errorf("Expected Name='data', got '%s'", vol.Name)
	}

	if len(vol.Devices) != 2 {
		t.Errorf("Expected 2 devices, got %d", len(vol.Devices))
	}

	if !vol.Status.Healthy {
		t.Error("Expected Healthy=true")
	}
}

// TestSubVolumeStruct 测试 SubVolume 数据结构.
func TestSubVolumeStruct(t *testing.T) {
	subvol := &storage.SubVolume{
		ID:       256,
		Name:     "documents",
		Path:     "/mnt/data/documents",
		ParentID: 5,
		ReadOnly: false,
		UUID:     "test-uuid",
	}

	if subvol.Name != "documents" {
		t.Errorf("Expected Name='documents', got '%s'", subvol.Name)
	}

	if subvol.ReadOnly {
		t.Error("Expected ReadOnly=false")
	}
}

// TestSnapshotStruct 测试 Snapshot 数据结构.
func TestSnapshotStruct(t *testing.T) {
	now := time.Now()
	snap := &storage.Snapshot{
		Name:      "daily-20260310",
		Path:      "/mnt/data/.snapshots/daily-20260310",
		Source:    "documents",
		ReadOnly:  true,
		CreatedAt: now,
	}

	if !snap.ReadOnly {
		t.Error("Expected ReadOnly=true for snapshot")
	}

	if snap.Source != "documents" {
		t.Errorf("Expected Source='documents', got '%s'", snap.Source)
	}

	if !snap.CreatedAt.Equal(now) {
		t.Error("Expected CreatedAt to match")
	}
}

// TestVolumeStatus 测试 VolumeStatus.
func TestVolumeStatus(t *testing.T) {
	status := storage.VolumeStatus{
		BalanceRunning: false,
		ScrubRunning:   false,
		Healthy:        true,
	}

	if !status.Healthy {
		t.Error("Expected Healthy=true")
	}

	if status.BalanceRunning {
		t.Error("Expected BalanceRunning=false")
	}
}

// ========== 并发安全测试 ==========

// TestConcurrentRAIDConfigAccess 测试并发访问 RAID 配置.
func TestConcurrentRAIDConfigAccess(t *testing.T) {
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			configs := storage.RAIDConfigs
			_ = configs["raid1"]
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// ========== 性能测试 ==========

// BenchmarkRAIDConfig 性能测试：RAID 配置查询.
func BenchmarkRAIDConfig(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = storage.RAIDConfigs["raid1"]
	}
}

// BenchmarkVolumeCreation 性能测试：创建 Volume 对象.
func BenchmarkVolumeCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &storage.Volume{
			Name:        "test",
			Devices:     []string{"/dev/sda1"},
			DataProfile: "single",
			MetaProfile: "single",
			Status:      storage.VolumeStatus{Healthy: true},
		}
	}
}
