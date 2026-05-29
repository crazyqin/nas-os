// Package storagesetupwizard 提供存储设置向导功能
// 引导用户完成存储池、RAID、卷的初始设置
// 对标群晖存储管理器向导和TrueNAS存储设置向导
package storagesetupwizard

import (
	"fmt"
	"time"
)

// Step 向导步骤
type Step string

const (
	StepDiskSelection  Step = "disk_selection"  // 磁盘选择
	StepRAIDConfig     Step = "raid_config"     // RAID配置
	StepPoolCreation   Step = "pool_creation"   // 存储池创建
	StepVolumeSetup    Step = "volume_setup"    // 卷设置
	StepShareConfig    Step = "share_config"    // 共享配置
	StepReview         Step = "review"          // 确认
	StepComplete       Step = "complete"        // 完成
)

// RAIDType RAID类型
type RAIDType string

const (
	RAIDBasic  RAIDType = "basic"  // 基础（单盘）
	RAID0      RAIDType = "raid0"  // 条带化
	RAID1      RAIDType = "raid1"  // 镜像
	RAID5      RAIDType = "raid5"  // RAID5
	RAID6      RAIDType = "raid6"  // RAID6
	RAID10     RAIDType = "raid10" // RAID10
	RAIDZ1     RAIDType = "raidz1" // ZFS RAIDZ1
	RAIDZ2     RAIDType = "raidz2" // ZFS RAIDZ2
	RAIDZ3     RAIDType = "raidz3" // ZFS RAIDZ3
)

// DiskInfo 磁盘信息
type DiskInfo struct {
	ID         string  `json:"id"`
	Device     string  `json:"device"`
	Model      string  `json:"model"`
	Serial     string  `json:"serial"`
	Size       int64   `json:"size"`       // 字节
	RPM        int     `json:"rpm"`        // 转速（SSD为0）
	Type       string  `json:"type"`       // hdd/ssd/nvme
	Health     string  `json:"health"`     // healthy/degraded/faulted
	Temperature int    `json:"temperature"` // 温度
	Interface  string  `json:"interface"`  // sata/sas/nvme/usb
}

// RAIDConfig RAID配置
type RAIDConfig struct {
	Type        RAIDType `json:"type"`
	Disks       []string `json:"disks"`       // 磁盘ID列表
	StripeSize  int      `json:"stripe_size"` // 条带大小（KB）
	Spares      int      `json:"spares"`      // 热备盘数量
}

// PoolConfig 存储池配置
type PoolConfig struct {
	Name        string     `json:"name"`
	RAID        RAIDConfig `json:"raid"`
	Compression string     `json:"compression"` // lz4/zstd/off
	Dedup       bool       `json:"dedup"`
	Encryption  bool       `json:"encryption"`
	Quota       int64      `json:"quota"` // 配额（字节，0为无限制）
}

// VolumeConfig 卷配置
type VolumeConfig struct {
	Name       string `json:"name"`
	PoolName   string `json:"pool_name"`
	Size       int64  `json:"size"`        // 字节
	FileSystem string `json:"filesystem"`  // ext4/btrfs/zfs/xfs
	MountPoint string `json:"mount_point"`
	Quota      int64  `json:"quota"`
}

// ShareConfig 共享配置
type ShareConfig struct {
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	Protocol   []string `json:"protocol"` // smb/nfs/ftp/webdav
	ReadOnly   bool     `json:"read_only"`
	GuestAccess bool    `json:"guest_access"`
	Users      []string `json:"users"`
	Groups     []string `json:"groups"`
}

// SetupSession 设置会话
type SetupSession struct {
	ID          string       `json:"id"`
	CurrentStep Step         `json:"current_step"`
	Disks       []DiskInfo   `json:"disks"`
	Pool        PoolConfig   `json:"pool"`
	Volume      VolumeConfig `json:"volume"`
	Share       ShareConfig  `json:"share"`
	Status      string       `json:"status"` // in_progress/completed/failed
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
	Error       string       `json:"error,omitempty"`
}

// RAIDRecommendation RAID推荐
type RAIDRecommendation struct {
	Type        RAIDType `json:"type"`
	Description string   `json:"description"`
	MinDisks    int      `json:"min_disks"`
	MaxDisks    int      `json:"max_disks"`
	FaultTolerance int   `json:"fault_tolerance"` // 容错盘数
	UsableRatio float64  `json:"usable_ratio"`    // 可用空间比例
	Performance string   `json:"performance"`     // 读写性能评级
	Recommended bool     `json:"recommended"`     // 是否推荐
	Reason      string   `json:"reason"`          // 推荐原因
}

// DiskRecommendation 磁盘推荐
type DiskRecommendation struct {
	DiskID      string `json:"disk_id"`
	Role        string `json:"role"` // data/spare/cache/log
	Reason      string `json:"reason"`
}

// CapacityEstimation 容量估算
type CapacityEstimation struct {
	TotalRaw     int64   `json:"total_raw"`     // 原始总容量
	Usable       int64   `json:"usable"`         // 可用容量
	Overhead     int64   `json:"overhead"`       // 开销
	OverheadPct  float64 `json:"overhead_pct"`   // 开销比例
	Recommended  bool    `json:"recommended"`    // 是否推荐此配置
}

// DefaultRAIDDefaults 返回默认RAID配置
func DefaultRAIDDefaults() RAIDConfig {
	return RAIDConfig{
		Type:       RAID1,
		StripeSize: 64,
		Spares:     0,
	}
}

// DefaultPoolDefaults 返回默认存储池配置
func DefaultPoolDefaults() PoolConfig {
	return PoolConfig{
		Compression: "lz4",
		Dedup:       false,
		Encryption:  false,
		Quota:       0,
	}
}

// DefaultVolumeDefaults 返回默认卷配置
func DefaultVolumeDefaults() VolumeConfig {
	return VolumeConfig{
		FileSystem: "btrfs",
		Quota:      0,
	}
}

// ValidateRAIDConfig 验证RAID配置
func ValidateRAIDConfig(config RAIDConfig, disks []DiskInfo) error {
	if len(config.Disks) == 0 {
		return fmt.Errorf("至少需要选择一个磁盘")
	}

	// 检查磁盘是否存在
	diskMap := make(map[string]bool)
	for _, d := range disks {
		diskMap[d.ID] = true
	}
	for _, id := range config.Disks {
		if !diskMap[id] {
			return fmt.Errorf("磁盘 %s 不存在", id)
		}
	}

	// RAID类型验证
	switch config.Type {
	case RAID0:
		if len(config.Disks) < 2 {
			return fmt.Errorf("RAID0至少需要2个磁盘")
		}
	case RAID1:
		if len(config.Disks) < 2 {
			return fmt.Errorf("RAID1至少需要2个磁盘")
		}
	case RAID5:
		if len(config.Disks) < 3 {
			return fmt.Errorf("RAID5至少需要3个磁盘")
		}
	case RAID6:
		if len(config.Disks) < 4 {
			return fmt.Errorf("RAID6至少需要4个磁盘")
		}
	case RAID10:
		if len(config.Disks) < 4 || len(config.Disks)%2 != 0 {
			return fmt.Errorf("RAID10需要偶数个磁盘，至少4个")
		}
	case RAIDZ1:
		if len(config.Disks) < 3 {
			return fmt.Errorf("RAIDZ1至少需要3个磁盘")
		}
	case RAIDZ2:
		if len(config.Disks) < 4 {
			return fmt.Errorf("RAIDZ2至少需要4个磁盘")
		}
	case RAIDZ3:
		if len(config.Disks) < 5 {
			return fmt.Errorf("RAIDZ3至少需要5个磁盘")
		}
	}

	return nil
}

// RecommendRAID 根据磁盘数量和用途推荐RAID类型
func RecommendRAID(diskCount int, priority string) []RAIDRecommendation {
	var recommendations []RAIDRecommendation

	switch {
	case diskCount == 1:
		recommendations = append(recommendations, RAIDRecommendation{
			Type:        RAIDBasic,
			Description: "单盘基础存储",
			MinDisks:    1,
			MaxDisks:    1,
			FaultTolerance: 0,
			UsableRatio: 1.0,
			Performance: "单盘性能",
			Recommended: true,
			Reason:      "单盘配置，无冗余",
		})
	case diskCount == 2:
		if priority == "performance" {
			recommendations = append(recommendations, RAIDRecommendation{
				Type:        RAID0,
				Description: "条带化，高性能",
				MinDisks:    2,
				MaxDisks:    2,
				FaultTolerance: 0,
				UsableRatio: 1.0,
				Performance: "高",
				Recommended: true,
				Reason:      "追求性能，无冗余",
			})
		}
		recommendations = append(recommendations, RAIDRecommendation{
			Type:        RAID1,
			Description: "镜像，数据安全",
			MinDisks:    2,
			MaxDisks:    2,
			FaultTolerance: 1,
			UsableRatio: 0.5,
			Performance: "中",
			Recommended: priority != "performance",
			Reason:      "数据安全优先",
		})
	case diskCount >= 3 && diskCount <= 4:
		recommendations = append(recommendations, RAIDRecommendation{
			Type:        RAID5,
			Description: "RAID5，平衡性能与安全",
			MinDisks:    3,
			MaxDisks:    16,
			FaultTolerance: 1,
			UsableRatio: float64(diskCount-1) / float64(diskCount),
			Performance: "中高",
			Recommended: true,
			Reason:      "性能与安全的平衡",
		})
		if diskCount >= 4 {
			recommendations = append(recommendations, RAIDRecommendation{
				Type:        RAID6,
				Description: "RAID6，双重冗余",
				MinDisks:    4,
				MaxDisks:    16,
				FaultTolerance: 2,
				UsableRatio: float64(diskCount-2) / float64(diskCount),
				Performance: "中",
				Recommended: false,
				Reason:      "更高安全性",
			})
			recommendations = append(recommendations, RAIDRecommendation{
				Type:        RAID10,
				Description: "RAID10，高性能镜像",
				MinDisks:    4,
				MaxDisks:    16,
				FaultTolerance: 1,
				UsableRatio: 0.5,
				Performance: "高",
				Recommended: priority == "performance",
				Reason:      "高性能需求",
			})
		}
	case diskCount >= 5:
		recommendations = append(recommendations, RAIDRecommendation{
			Type:        RAID5,
			Description: "RAID5，平衡性能与安全",
			MinDisks:    3,
			MaxDisks:    16,
			FaultTolerance: 1,
			UsableRatio: float64(diskCount-1) / float64(diskCount),
			Performance: "中高",
			Recommended: priority == "balanced",
			Reason:      "性能与安全的平衡",
		})
		recommendations = append(recommendations, RAIDRecommendation{
			Type:        RAID6,
			Description: "RAID6，双重冗余",
			MinDisks:    4,
			MaxDisks:    16,
			FaultTolerance: 2,
			UsableRatio: float64(diskCount-2) / float64(diskCount),
			Performance: "中",
			Recommended: priority == "safety",
			Reason:      "更高安全性",
		})
		recommendations = append(recommendations, RAIDRecommendation{
			Type:        RAID10,
			Description: "RAID10，高性能镜像",
			MinDisks:    4,
			MaxDisks:    16,
			FaultTolerance: 1,
			UsableRatio: 0.5,
			Performance: "高",
			Recommended: priority == "performance",
			Reason:      "高性能需求",
		})
		recommendations = append(recommendations, RAIDRecommendation{
			Type:        RAIDZ2,
			Description: "ZFS RAIDZ2，企业级冗余",
			MinDisks:    4,
			MaxDisks:    16,
			FaultTolerance: 2,
			UsableRatio: float64(diskCount-2) / float64(diskCount),
			Performance: "中",
			Recommended: false,
			Reason:      "ZFS高级功能",
		})
	}

	return recommendations
}

// EstimateCapacity 估算可用容量
func EstimateCapacity(diskCount int, minDiskSize int64, raidType RAIDType) CapacityEstimation {
	totalRaw := minDiskSize * int64(diskCount)

	var usable int64
	var overheadPct float64

	switch raidType {
	case RAIDBasic:
		usable = totalRaw
	case RAID0:
		usable = totalRaw
	case RAID1:
		usable = totalRaw / 2
		overheadPct = 0.5
	case RAID5:
		usable = minDiskSize * int64(diskCount-1)
		overheadPct = 1.0 / float64(diskCount)
	case RAID6:
		usable = minDiskSize * int64(diskCount-2)
		overheadPct = 2.0 / float64(diskCount)
	case RAID10:
		usable = totalRaw / 2
		overheadPct = 0.5
	case RAIDZ1:
		usable = minDiskSize * int64(diskCount-1)
		overheadPct = 1.0 / float64(diskCount)
	case RAIDZ2:
		usable = minDiskSize * int64(diskCount-2)
		overheadPct = 2.0 / float64(diskCount)
	case RAIDZ3:
		usable = minDiskSize * int64(diskCount-3)
		overheadPct = 3.0 / float64(diskCount)
	}

	return CapacityEstimation{
		TotalRaw:    totalRaw,
		Usable:      usable,
		Overhead:    totalRaw - usable,
		OverheadPct: overheadPct,
		Recommended: overheadPct <= 0.5,
	}
}
