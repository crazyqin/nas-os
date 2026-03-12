// Package fixtures 提供 NAS-OS 测试固件
// 包含测试数据和模拟数据
package fixtures

import (
	"time"

	"nas-os/internal/storage"
)

// ========== 存储测试固件 ==========

// VolumeFixtures 卷测试固件
var VolumeFixtures = struct {
	// ValidVolumes 有效的卷数据
	ValidVolumes []*storage.Volume
	// InvalidVolumes 无效的卷数据
	InvalidVolumes []*storage.Volume
	// TestDevices 测试设备列表
	TestDevices map[string][]string
}{
	ValidVolumes: []*storage.Volume{
		{
			Name:        "data",
			UUID:        "volume-uuid-data",
			Devices:     []string{"/dev/sda1", "/dev/sdb1"},
			Size:        2000000000000, // 2TB
			Used:        500000000000,  // 500GB
			Free:        1500000000000, // 1.5TB
			DataProfile: "raid1",
			MetaProfile: "raid1",
			MountPoint:  "/mnt/data",
			Status: storage.VolumeStatus{
				Healthy:        true,
				BalanceRunning: false,
				ScrubRunning:   false,
			},
		},
		{
			Name:        "backup",
			UUID:        "volume-uuid-backup",
			Devices:     []string{"/dev/sdc1"},
			Size:        4000000000000, // 4TB
			Used:        1000000000000, // 1TB
			Free:        3000000000000, // 3TB
			DataProfile: "single",
			MetaProfile: "single",
			MountPoint:  "/mnt/backup",
			Status: storage.VolumeStatus{
				Healthy:        true,
				BalanceRunning: false,
				ScrubRunning:   false,
			},
		},
		{
			Name:        "media",
			UUID:        "volume-uuid-media",
			Devices:     []string{"/dev/sdd1", "/dev/sde1", "/dev/sdf1"},
			Size:        6000000000000, // 6TB
			Used:        2000000000000, // 2TB
			Free:        4000000000000, // 4TB
			DataProfile: "raid5",
			MetaProfile: "raid1",
			MountPoint:  "/mnt/media",
			Status: storage.VolumeStatus{
				Healthy:        true,
				BalanceRunning: false,
				ScrubRunning:   false,
			},
		},
	},
	InvalidVolumes: []*storage.Volume{
		{
			Name:    "", // 空名称
			Devices: []string{"/dev/sda1"},
		},
		{
			Name:    "no-devices",
			Devices: []string{}, // 无设备
		},
	},
	TestDevices: map[string][]string{
		"single": {"/dev/sda1"},
		"raid0":  {"/dev/sda1", "/dev/sdb1"},
		"raid1":  {"/dev/sda1", "/dev/sdb1"},
		"raid5":  {"/dev/sda1", "/dev/sdb1", "/dev/sdc1"},
		"raid6":  {"/dev/sda1", "/dev/sdb1", "/dev/sdc1", "/dev/sdd1"},
		"raid10": {"/dev/sda1", "/dev/sdb1", "/dev/sdc1", "/dev/sdd1"},
	},
}

// SubVolumeFixtures 子卷测试固件
var SubVolumeFixtures = struct {
	ValidSubVolumes []*storage.SubVolume
}{
	ValidSubVolumes: []*storage.SubVolume{
		{
			ID:       256,
			Name:     "documents",
			Path:     "/mnt/data/documents",
			ParentID: 5,
			ReadOnly: false,
			UUID:     "subvol-uuid-documents",
		},
		{
			ID:       257,
			Name:     "photos",
			Path:     "/mnt/data/photos",
			ParentID: 5,
			ReadOnly: false,
			UUID:     "subvol-uuid-photos",
		},
		{
			ID:       258,
			Name:     "archive",
			Path:     "/mnt/data/archive",
			ParentID: 5,
			ReadOnly: true,
			UUID:     "subvol-uuid-archive",
		},
	},
}

// SnapshotFixtures 快照测试固件
var SnapshotFixtures = struct {
	ValidSnapshots []*storage.Snapshot
}{
	ValidSnapshots: []*storage.Snapshot{
		{
			Name:      "daily-20260310",
			Path:      "/mnt/data/.snapshots/daily-20260310",
			Source:    "documents",
			ReadOnly:  true,
			CreatedAt: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC),
		},
		{
			Name:      "weekly-20260310",
			Path:      "/mnt/data/.snapshots/weekly-20260310",
			Source:    "documents",
			ReadOnly:  true,
			CreatedAt: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC),
		},
		{
			Name:      "manual-pre-update",
			Path:      "/mnt/data/.snapshots/manual-pre-update",
			Source:    "root",
			ReadOnly:  true,
			CreatedAt: time.Date(2026, 3, 10, 10, 30, 0, 0, time.UTC),
		},
	},
}

// RAIDConfigFixtures RAID 配置测试固件
var RAIDConfigFixtures = struct {
	ValidConfigs []struct {
		Profile        string
		MinDevices     int
		FaultTolerance int
		Description    string
	}
}{
	ValidConfigs: []struct {
		Profile        string
		MinDevices     int
		FaultTolerance int
		Description    string
	}{
		{"single", 1, 0, "单盘模式，无冗余"},
		{"raid0", 2, 0, "条带模式，性能优先，无冗余"},
		{"raid1", 2, 1, "镜像模式，1盘容错"},
		{"raid5", 3, 1, "分布式奇偶校验，1盘容错"},
		{"raid6", 4, 2, "双重奇偶校验，2盘容错"},
		{"raid10", 4, 1, "条带+镜像，1盘容错"},
	},
}

// ========== 用户测试固件 ==========

// UserFixtures 用户测试固件
var UserFixtures = struct {
	// ValidUsers 有效用户
	ValidUsers []struct {
		Username string
		Password string
		Role     string
	}
}{
	ValidUsers: []struct {
		Username string
		Password string
		Role     string
	}{
		{"admin", "admin123", "admin"},
		{"user1", "password1", "user"},
		{"guest", "guest123", "guest"},
	},
}

// ========== API 测试固件 ==========

// APIFixtures API 测试固件
var APIFixtures = struct {
	// VolumeEndpoints 卷 API 端点
	VolumeEndpoints map[string]string
	// AuthEndpoints 认证 API 端点
	AuthEndpoints map[string]string
}{
	VolumeEndpoints: map[string]string{
		"list":   "/api/v1/volumes",
		"create": "/api/v1/volumes",
		"get":    "/api/v1/volumes/:name",
		"delete": "/api/v1/volumes/:name",
	},
	AuthEndpoints: map[string]string{
		"login":  "/api/v1/auth/login",
		"logout": "/api/v1/auth/logout",
	},
}

// ========== 辅助函数 ==========

// CreateTestVolume 创建测试卷
func CreateTestVolume(name string) *storage.Volume {
	return &storage.Volume{
		Name:        name,
		UUID:        "test-uuid-" + name,
		Devices:     []string{"/dev/sda1"},
		Size:        1000000000000,
		Used:        0,
		Free:        1000000000000,
		DataProfile: "single",
		MetaProfile: "single",
		Status: storage.VolumeStatus{
			Healthy: true,
		},
	}
}

// CreateTestSnapshot 创建测试快照
func CreateTestSnapshot(name, source string) *storage.Snapshot {
	return &storage.Snapshot{
		Name:      name,
		Path:      "/mnt/data/.snapshots/" + name,
		Source:    source,
		ReadOnly:  true,
		CreatedAt: time.Now(),
	}
}
