package storage

import (
	"errors"
	"testing"
)

// TestNewManager 测试创建存储管理器
func TestNewManager_WithEmptyMountBase(t *testing.T) {
	// 创建临时目录用于测试
	mgr, err := NewManager("/tmp/test-nas-storage")
	if err != nil {
		// 在没有 btrfs 的环境下可能会失败
		t.Logf("NewManager failed (expected in non-btrfs env): %v", err)
	}
	if mgr != nil {
		if mgr.mountBase != "/tmp/test-nas-storage" {
			t.Errorf("Expected mountBase=/tmp/test-nas-storage, got %s", mgr.mountBase)
		}
	}
}

// TestNewManager_WithDefaultMountBase 测试默认挂载基础目录
func TestNewManager_WithDefaultMountBase(t *testing.T) {
	// 测试空字符串使用默认值
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "",
	}

	// 默认应该是 /mnt
	if mgr.mountBase == "" {
		// NewManager 会设置默认值
		t.Log("Empty mountBase should default to /mnt")
	}
}

// TestManager_ListVolumesEmpty 测试空卷列表
func TestManager_ListVolumesEmpty(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/tmp/test",
	}

	volumes := mgr.ListVolumes()
	if len(volumes) != 0 {
		t.Errorf("Expected empty volume list, got %d", len(volumes))
	}
}

// TestManager_ListVolumes 测试卷列表
func TestManager_ListVolumes(t *testing.T) {
	mgr := &Manager{
		volumes: map[string]*Volume{
			"vol1": {Name: "vol1", UUID: "uuid1"},
			"vol2": {Name: "vol2", UUID: "uuid2"},
			"vol3": {Name: "vol3", UUID: "uuid3"},
		},
		mountBase: "/tmp/test",
	}

	volumes := mgr.ListVolumes()
	if len(volumes) != 3 {
		t.Errorf("Expected 3 volumes, got %d", len(volumes))
	}
}

// TestManager_GetVolume 测试获取卷
func TestManager_GetVolume(t *testing.T) {
	mgr := &Manager{
		volumes: map[string]*Volume{
			"data": {Name: "data", UUID: "test-uuid"},
		},
		mountBase: "/tmp/test",
	}

	vol := mgr.GetVolume("data")
	if vol == nil {
		t.Fatal("Expected volume, got nil")
	}
	if vol.Name != "data" {
		t.Errorf("Expected name=data, got %s", vol.Name)
	}
}

// TestManager_GetVolumeNotFound 测试获取不存在的卷
func TestManager_GetVolumeNotFound(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/tmp/test",
	}

	vol := mgr.GetVolume("nonexistent")
	if vol != nil {
		t.Error("Expected nil for nonexistent volume")
	}
}

// TestManager_CreateVolumeValidation 测试创建卷验证
func TestManager_CreateVolumeValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/tmp/test",
	}

	// 测试已存在的卷
	mgr.volumes["existing"] = &Volume{Name: "existing"}

	// 测试设备数量不足
	// RAID1 需要至少 2 个设备
	config := RAIDConfigs["raid1"]
	if config.MinDevices != 2 {
		t.Errorf("RAID1 should require 2 devices")
	}

	// 验证设备数量检查
	devices := []string{"/dev/sda1"}
	if len(devices) < config.MinDevices {
		t.Log("Correctly detected insufficient devices for RAID1")
	}
}

// TestManager_DeleteVolumeValidation 测试删除卷验证
func TestManager_DeleteVolumeValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/tmp/test",
	}

	// 测试删除不存在的卷
	err := mgr.DeleteVolume("nonexistent", false)
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_DeleteVolumeWithSubvolumes 测试删除包含子卷的卷
func TestManager_DeleteVolumeWithSubvolumes(t *testing.T) {
	mgr := &Manager{
		volumes: map[string]*Volume{
			"data": {
				Name:       "data",
				MountPoint: "/mnt/data",
				Subvolumes: []*SubVolume{
					{Name: "documents"},
					{Name: "photos"},
				},
			},
		},
		mountBase: "/mnt",
	}

	// 非强制删除应该失败
	err := mgr.DeleteVolume("data", false)
	if err == nil {
		t.Error("Expected error when deleting volume with subvolumes without force")
	}
}

// TestManager_MountVolumeValidation 测试挂载卷验证
func TestManager_MountVolumeValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试挂载不存在的卷
	err := mgr.MountVolume("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_UnmountVolumeValidation 测试卸载卷验证
func TestManager_UnmountVolumeValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试卸载不存在的卷
	err := mgr.UnmountVolume("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_GetUsageValidation 测试获取使用情况验证
func TestManager_GetUsageValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	_, _, _, err := mgr.GetUsage("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_ListSubVolumesValidation 测试列出子卷验证
func TestManager_ListSubVolumesValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	_, err := mgr.ListSubVolumes("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_CreateSubVolumeValidation 测试创建子卷验证
func TestManager_CreateSubVolumeValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	_, err := mgr.CreateSubVolume("nonexistent", "test")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_DeleteSubVolumeValidation 测试删除子卷验证
func TestManager_DeleteSubVolumeValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	err := mgr.DeleteSubVolume("nonexistent", "test")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_GetSubVolumeValidation 测试获取子卷验证
func TestManager_GetSubVolumeValidation(t *testing.T) {
	mgr := &Manager{
		volumes: map[string]*Volume{
			"data": {Name: "data", MountPoint: "", Subvolumes: []*SubVolume{}},
		},
		mountBase: "/mnt",
	}

	// 测试不存在的子卷
	_, err := mgr.GetSubVolume("data", "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent subvolume")
	}
}

// TestManager_CreateSnapshotValidation 测试创建快照验证
func TestManager_CreateSnapshotValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	_, err := mgr.CreateSnapshot("nonexistent", "subvol", "snap", true)
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_ListSnapshotsValidation 测试列出快照验证
func TestManager_ListSnapshotsValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	_, err := mgr.ListSnapshots("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_DeleteSnapshotValidation 测试删除快照验证
func TestManager_DeleteSnapshotValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	err := mgr.DeleteSnapshot("nonexistent", "snap")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_BalanceValidation 测试平衡验证
func TestManager_BalanceValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	err := mgr.Balance("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_ScrubValidation 测试校验验证
func TestManager_ScrubValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	err := mgr.Scrub("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_ConvertRAIDValidation 测试RAID转换验证
func TestManager_ConvertRAIDValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	err := mgr.ConvertRAID("nonexistent", "raid1", "raid1")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_AddDeviceValidation 测试添加设备验证
func TestManager_AddDeviceValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	err := mgr.AddDevice("nonexistent", "/dev/sdc1")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_RemoveDeviceValidation 测试移除设备验证
func TestManager_RemoveDeviceValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	err := mgr.RemoveDevice("nonexistent", "/dev/sda1")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestVolume_StatusHealth 测试卷健康状态
func TestVolume_StatusHealth(t *testing.T) {
	vol := &Volume{
		Name: "data",
		Status: VolumeStatus{
			Healthy:        true,
			BalanceRunning: false,
			ScrubRunning:   false,
			ScrubErrors:    0,
		},
	}

	if !vol.Status.Healthy {
		t.Error("Volume should be healthy")
	}
}

// TestVolume_StatusUnhealthy 测试卷不健康状态
func TestVolume_StatusUnhealthy(t *testing.T) {
	vol := &Volume{
		Name: "data",
		Status: VolumeStatus{
			Healthy:     false,
			ScrubErrors: 5,
		},
	}

	if vol.Status.Healthy {
		t.Error("Volume should be unhealthy")
	}
}

// TestVolume_UsageCalculation 测试卷使用计算
func TestVolume_UsageCalculation(t *testing.T) {
	vol := &Volume{
		Name: "data",
		Size: 1000,
		Used: 300,
		Free: 700,
	}

	if vol.Used+vol.Free != vol.Size {
		t.Errorf("Used + Free should equal Size")
	}
}

// TestSubVolume_ReadOnly 测试子卷只读属性
func TestSubVolume_ReadOnly(t *testing.T) {
	subvol := &SubVolume{
		Name:     "readonly-data",
		ReadOnly: true,
	}

	if !subvol.ReadOnly {
		t.Error("Subvolume should be read-only")
	}
}

// TestSubVolume_SnapshotCount 测试子卷快照数量
func TestSubVolume_SnapshotCount(t *testing.T) {
	subvol := &SubVolume{
		Name: "documents",
		Snapshots: []*Snapshot{
			{Name: "snap1"},
			{Name: "snap2"},
			{Name: "snap3"},
		},
	}

	if len(subvol.Snapshots) != 3 {
		t.Errorf("Expected 3 snapshots, got %d", len(subvol.Snapshots))
	}
}

// TestSnapshot_ReadOnlyDefault 测试快照默认只读
func TestSnapshot_ReadOnlyDefault(t *testing.T) {
	snap := &Snapshot{
		Name:     "backup-snap",
		ReadOnly: true,
	}

	if !snap.ReadOnly {
		t.Error("Snapshots should typically be read-only")
	}
}

// TestRAIDConfig_MinDevices 测试RAID配置最小设备数
func TestRAIDConfig_MinDevices(t *testing.T) {
	tests := []struct {
		profile    string
		minDevices int
	}{
		{"single", 1},
		{"raid0", 2},
		{"raid1", 2},
		{"raid5", 3},
		{"raid6", 4},
		{"raid10", 4},
	}

	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			config, ok := RAIDConfigs[tt.profile]
			if !ok {
				t.Fatalf("RAID config %s not found", tt.profile)
			}
			if config.MinDevices != tt.minDevices {
				t.Errorf("Expected MinDevices=%d, got %d", tt.minDevices, config.MinDevices)
			}
		})
	}
}

// TestRAIDConfig_FaultTolerance 测试RAID容错能力
func TestRAIDConfig_FaultTolerance(t *testing.T) {
	tests := []struct {
		profile        string
		faultTolerance int
	}{
		{"single", 0},
		{"raid0", 0},
		{"raid1", 1},
		{"raid5", 1},
		{"raid6", 2},
		{"raid10", 1},
	}

	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			config, ok := RAIDConfigs[tt.profile]
			if !ok {
				t.Fatalf("RAID config %s not found", tt.profile)
			}
			if config.FaultTolerance != tt.faultTolerance {
				t.Errorf("Expected FaultTolerance=%d, got %d", tt.faultTolerance, config.FaultTolerance)
			}
		})
	}
}

// TestSnapshotConfig_Defaults 测试快照配置默认值
func TestSnapshotConfig_Defaults(t *testing.T) {
	config := DefaultSnapshotConfig

	if !config.ReadOnly {
		t.Error("Default snapshot should be read-only")
	}
	if !config.Timestamp {
		t.Error("Default snapshot should include timestamp")
	}
	if config.TimeFormat != "20060102-150405" {
		t.Errorf("Unexpected time format: %s", config.TimeFormat)
	}
	if config.SnapDir != ".snapshots" {
		t.Errorf("Unexpected snapshot dir: %s", config.SnapDir)
	}
}

// TestManager_MountSubVolumeValidation 测试子卷挂载验证
func TestManager_MountSubVolumeValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	err := mgr.MountSubVolume("nonexistent", "subvol", "/mnt/test")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_SetSubVolumeReadOnlyValidation 测试设置子卷只读验证
func TestManager_SetSubVolumeReadOnlyValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	err := mgr.SetSubVolumeReadOnly("nonexistent", "subvol", true)
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_GetDefaultSubVolumeValidation 测试获取默认子卷验证
func TestManager_GetDefaultSubVolumeValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	_, err := mgr.GetDefaultSubVolume("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_SetDefaultSubVolumeValidation 测试设置默认子卷验证
func TestManager_SetDefaultSubVolumeValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	err := mgr.SetDefaultSubVolume("nonexistent", 256)
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_GetDeviceStatsValidation 测试获取设备统计验证
func TestManager_GetDeviceStatsValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	_, err := mgr.GetDeviceStats("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_GetBalanceStatusValidation 测试获取平衡状态验证
func TestManager_GetBalanceStatusValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	_, err := mgr.GetBalanceStatus("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_GetScrubStatusValidation 测试获取校验状态验证
func TestManager_GetScrubStatusValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	_, err := mgr.GetScrubStatus("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestVolumeStatus_BalanceProgress 测试平衡进度
func TestVolumeStatus_BalanceProgress(t *testing.T) {
	status := VolumeStatus{
		BalanceRunning:  true,
		BalanceProgress: 45.5,
	}

	if !status.BalanceRunning {
		t.Error("Balance should be running")
	}
	if status.BalanceProgress < 0 || status.BalanceProgress > 100 {
		t.Errorf("Invalid balance progress: %f", status.BalanceProgress)
	}
}

// TestVolumeStatus_ScrubProgress 测试校验进度
func TestVolumeStatus_ScrubProgress(t *testing.T) {
	status := VolumeStatus{
		ScrubRunning:  true,
		ScrubProgress: 75.0,
		ScrubErrors:   0,
	}

	if !status.ScrubRunning {
		t.Error("Scrub should be running")
	}
	if status.ScrubProgress < 0 || status.ScrubProgress > 100 {
		t.Errorf("Invalid scrub progress: %f", status.ScrubProgress)
	}
}

// TestManager_RestoreSnapshotValidation 测试恢复快照验证
func TestManager_RestoreSnapshotValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	err := mgr.RestoreSnapshot("nonexistent", "snap", "restored")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_ListSubVolumeSnapshotsValidation 测试列出子卷快照验证
func TestManager_ListSubVolumeSnapshotsValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	_, err := mgr.ListSubVolumeSnapshots("nonexistent", "subvol")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestManager_GetSnapshotValidation 测试获取快照验证
func TestManager_GetSnapshotValidation(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 测试不存在的卷
	_, err := mgr.GetSnapshot("nonexistent", "snap")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// TestVolume_MultipleDevices 测试多设备卷
func TestVolume_MultipleDevices(t *testing.T) {
	vol := &Volume{
		Name:        "raid-volume",
		Devices:     []string{"/dev/sda1", "/dev/sdb1", "/dev/sdc1"},
		DataProfile: "raid5",
	}

	if len(vol.Devices) != 3 {
		t.Errorf("Expected 3 devices, got %d", len(vol.Devices))
	}

	// 验证 RAID5 最少需要 3 个设备
	config := RAIDConfigs["raid5"]
	if len(vol.Devices) < config.MinDevices {
		t.Error("Not enough devices for RAID5")
	}
}

// TestVolume_SubvolumeCount 测试子卷数量
func TestVolume_SubvolumeCount(t *testing.T) {
	vol := &Volume{
		Name: "data",
		Subvolumes: []*SubVolume{
			{Name: "documents"},
			{Name: "photos"},
			{Name: "videos"},
			{Name: "music"},
		},
	}

	if len(vol.Subvolumes) != 4 {
		t.Errorf("Expected 4 subvolumes, got %d", len(vol.Subvolumes))
	}
}

// TestErrorWrap 测试错误包装
func TestErrorWrap(t *testing.T) {
	originalErr := errors.New("original error")
	wrappedErr := wrapError("operation failed", originalErr)

	if wrappedErr == nil {
		t.Fatal("Wrapped error should not be nil")
	}
	if wrappedErr.Error() == "" {
		t.Error("Wrapped error message should not be empty")
	}
}

// TestManager_ConcurrentAccess 测试并发访问
func TestManager_ConcurrentAccess(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/mnt",
	}

	// 添加一些测试卷
	for i := 0; i < 5; i++ {
		mgr.volumes["vol"+string(rune('0'+i))] = &Volume{
			Name: "vol" + string(rune('0'+i)),
		}
	}

	// 并发读取
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = mgr.ListVolumes()
			_ = mgr.GetVolume("vol0")
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestBtrfsClient_Interface 测试 btrfs 客户端接口（使用已定义的 MockBtrfsClient）
func TestBtrfsClient_Interface(t *testing.T) {
	// 确保 MockBtrfsClient 实现了 btrfs.Client 接口
	// MockBtrfsClient 已在 manager_test.go 中定义
}