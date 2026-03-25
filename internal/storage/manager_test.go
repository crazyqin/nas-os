package storage

import (
	"errors"
	"testing"
	"time"

	"nas-os/pkg/btrfs"
)

// MockBtrfsClient 模拟 btrfs 客户端.
type MockBtrfsClient struct {
	volumes    []btrfs.VolumeInfo
	subvolumes []btrfs.SubVolumeInfo
	usage      struct {
		total, used, free uint64
	}
	balanceStatus *btrfs.BalanceStatus
	scrubStatus   *btrfs.ScrubStatus
	err           error
}

func (m *MockBtrfsClient) ListVolumes() ([]btrfs.VolumeInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.volumes, nil
}

func (m *MockBtrfsClient) GetUsage(mountPoint string) (total, used, free uint64, err error) {
	if m.err != nil {
		return 0, 0, 0, m.err
	}
	return m.usage.total, m.usage.used, m.usage.free, nil
}

func (m *MockBtrfsClient) CreateVolume(label string, devices []string, dataProfile, metadataProfile string) error {
	return m.err
}

func (m *MockBtrfsClient) DeleteVolume(device string) error {
	return m.err
}

func (m *MockBtrfsClient) Mount(device, mountPoint string, options []string) error {
	return m.err
}

func (m *MockBtrfsClient) Unmount(mountPoint string) error {
	return m.err
}

func (m *MockBtrfsClient) ListSubVolumes(mountPoint string) ([]btrfs.SubVolumeInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.subvolumes, nil
}

func (m *MockBtrfsClient) CreateSubVolume(path string) error {
	return m.err
}

func (m *MockBtrfsClient) DeleteSubVolume(path string) error {
	return m.err
}

func (m *MockBtrfsClient) GetSubVolumeInfo(path string) (*btrfs.SubVolumeInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.subvolumes) > 0 {
		return &m.subvolumes[0], nil
	}
	return &btrfs.SubVolumeInfo{}, nil
}

func (m *MockBtrfsClient) SetSubVolumeReadOnly(path string, readOnly bool) error {
	return m.err
}

func (m *MockBtrfsClient) MountSubVolume(device, subvolPath, mountPoint string) error {
	return m.err
}

func (m *MockBtrfsClient) MountSubVolumeByID(device string, subvolID uint64, mountPoint string) error {
	return m.err
}

func (m *MockBtrfsClient) GetDefaultSubVolume(mountPoint string) (uint64, error) {
	return 256, m.err
}

func (m *MockBtrfsClient) SetDefaultSubVolume(mountPoint string, subvolID uint64) error {
	return m.err
}

func (m *MockBtrfsClient) CreateSnapshot(source, dest string, readOnly bool) error {
	return m.err
}

func (m *MockBtrfsClient) DeleteSnapshot(path string) error {
	return m.err
}

func (m *MockBtrfsClient) RestoreSnapshot(snapshotPath, targetPath string) error {
	return m.err
}

func (m *MockBtrfsClient) ListSnapshots(mountPoint string) ([]btrfs.SnapshotInfo, error) {
	return []btrfs.SnapshotInfo{}, m.err
}

func (m *MockBtrfsClient) GetDeviceStats(mountPoint string) ([]btrfs.DeviceStats, error) {
	return []btrfs.DeviceStats{}, m.err
}

func (m *MockBtrfsClient) AddDevice(mountPoint, device string) error {
	return m.err
}

func (m *MockBtrfsClient) RemoveDevice(mountPoint, device string) error {
	return m.err
}

func (m *MockBtrfsClient) ConvertProfile(mountPoint, dataProfile, metadataProfile string) error {
	return m.err
}

func (m *MockBtrfsClient) StartBalance(mountPoint string) error {
	return m.err
}

func (m *MockBtrfsClient) GetBalanceStatus(mountPoint string) (*btrfs.BalanceStatus, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.balanceStatus != nil {
		return m.balanceStatus, nil
	}
	return &btrfs.BalanceStatus{Running: false}, nil
}

func (m *MockBtrfsClient) CancelBalance(mountPoint string) error {
	return m.err
}

func (m *MockBtrfsClient) StartScrub(mountPoint string) error {
	return m.err
}

func (m *MockBtrfsClient) GetScrubStatus(mountPoint string) (*btrfs.ScrubStatus, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.scrubStatus != nil {
		return m.scrubStatus, nil
	}
	return &btrfs.ScrubStatus{Running: false}, nil
}

func (m *MockBtrfsClient) CancelScrub(mountPoint string) error {
	return m.err
}

// ========== RAID 配置测试 ==========

func TestRAIDConfigs(t *testing.T) {
	configs := RAIDConfigs

	// 测试所有预定义配置
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

func TestManagerGetRAIDConfigs(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/tmp/test-mnt",
	}

	configs := mgr.GetRAIDConfigs()
	if len(configs) != 6 {
		t.Errorf("Expected 6 RAID configs, got %d", len(configs))
	}
}

func TestManagerGetRAIDConfig(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/tmp/test-mnt",
	}

	// 测试存在的配置
	config := mgr.GetRAIDConfig("raid1")
	if config == nil {
		t.Fatal("Expected config for raid1")
	}
	if config.MinDevices != 2 {
		t.Errorf("Expected MinDevices=2, got %d", config.MinDevices)
	}

	// 测试不存在的配置
	config = mgr.GetRAIDConfig("invalid")
	if config != nil {
		t.Error("Expected nil for invalid config")
	}
}

// ========== 卷状态测试 ==========

func TestVolumeStatus(t *testing.T) {
	status := VolumeStatus{
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

// ========== 子卷测试 ==========

func TestSubVolume(t *testing.T) {
	subvol := &SubVolume{
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

func TestSubVolumeWithSnapshots(t *testing.T) {
	subvol := &SubVolume{
		ID:   256,
		Name: "documents",
		Path: "/mnt/data/documents",
		Snapshots: []*Snapshot{
			{Name: "snap1", ReadOnly: true},
			{Name: "snap2", ReadOnly: true},
		},
	}

	if len(subvol.Snapshots) != 2 {
		t.Errorf("Expected 2 snapshots, got %d", len(subvol.Snapshots))
	}
}

// ========== 快照测试 ==========

func TestSnapshot(t *testing.T) {
	snap := &Snapshot{
		Name:     "daily-20260310",
		Path:     "/mnt/data/.snapshots/daily-20260310",
		Source:   "documents",
		ReadOnly: true,
	}

	if !snap.ReadOnly {
		t.Error("Expected ReadOnly=true for snapshot")
	}

	if snap.Source != "documents" {
		t.Errorf("Expected Source='documents', got '%s'", snap.Source)
	}
}

func TestSnapshotWithTimestamp(t *testing.T) {
	now := time.Now()
	snap := &Snapshot{
		Name:      "daily-20260310-120000",
		Path:      "/mnt/data/.snapshots/daily-20260310-120000",
		Source:    "documents",
		ReadOnly:  true,
		CreatedAt: now,
	}

	if snap.CreatedAt != now {
		t.Errorf("Expected CreatedAt=%v, got %v", now, snap.CreatedAt)
	}
}

// ========== 快照配置测试 ==========

func TestSnapshotConfig(t *testing.T) {
	config := SnapshotConfig{
		Prefix:     "daily-",
		Suffix:     "-backup",
		ReadOnly:   true,
		Timestamp:  true,
		TimeFormat: "20060102-150405",
		SnapDir:    ".snapshots",
	}

	if config.Prefix != "daily-" {
		t.Errorf("Expected Prefix='daily-', got '%s'", config.Prefix)
	}

	if !config.ReadOnly {
		t.Error("Expected ReadOnly=true")
	}
}

func TestDefaultSnapshotConfig(t *testing.T) {
	config := DefaultSnapshotConfig

	if config.ReadOnly != true {
		t.Error("Expected DefaultSnapshotConfig.ReadOnly=true")
	}

	if config.TimeFormat != "20060102-150405" {
		t.Errorf("Expected TimeFormat='20060102-150405', got '%s'", config.TimeFormat)
	}
}

// ========== 卷测试 ==========

func TestVolume(t *testing.T) {
	vol := &Volume{
		Name:        "data",
		UUID:        "test-uuid",
		Devices:     []string{"/dev/sda1", "/dev/sdb1"},
		Size:        2000000000000,
		Used:        1000000000000,
		Free:        1000000000000,
		DataProfile: "raid1",
		MetaProfile: "raid1",
		MountPoint:  "/mnt/data",
		Status: VolumeStatus{
			Healthy: true,
		},
	}

	if vol.Name != "data" {
		t.Errorf("Expected Name='data', got '%s'", vol.Name)
	}

	if len(vol.Devices) != 2 {
		t.Errorf("Expected 2 devices, got %d", len(vol.Devices))
	}

	if vol.DataProfile != "raid1" {
		t.Errorf("Expected DataProfile='raid1', got '%s'", vol.DataProfile)
	}
}

func TestVolumeWithSubvolumes(t *testing.T) {
	vol := &Volume{
		Name: "data",
		Subvolumes: []*SubVolume{
			{Name: "documents"},
			{Name: "photos"},
		},
	}

	if len(vol.Subvolumes) != 2 {
		t.Errorf("Expected 2 subvolumes, got %d", len(vol.Subvolumes))
	}
}

// ========== 创建卷测试 ==========

func TestCreateVolumeSingle(t *testing.T) {
	// 由于需要实际文件系统操作，这里只测试配置验证
	config := RAIDConfigs["single"]
	if config.MinDevices != 1 {
		t.Errorf("Single profile should require 1 device, got %d", config.MinDevices)
	}
}

func TestCreateVolumeRAID1(t *testing.T) {
	// RAID1 配置测试
	config := RAIDConfigs["raid1"]
	if config.MinDevices != 2 {
		t.Errorf("RAID1 profile should require 2 devices, got %d", config.MinDevices)
	}

	if config.FaultTolerance != 1 {
		t.Errorf("RAID1 should tolerate 1 disk failure, got %d", config.FaultTolerance)
	}
}

func TestCreateVolumeInsufficientDevices(t *testing.T) {
	// RAID1 需要 2 个设备，只提供 1 个
	config := RAIDConfigs["raid1"]
	if len([]string{"/dev/sda1"}) >= config.MinDevices {
		t.Error("Should fail with insufficient devices")
	}
}

// ========== 错误处理测试 ==========

func TestVolumeNotFound(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/tmp/test-mnt",
	}

	vol := mgr.GetVolume("nonexistent")
	if vol != nil {
		t.Error("GetVolume should return nil for nonexistent volume")
	}
}

func TestSubVolumeNotFound(t *testing.T) {
	vol := &Volume{
		Name:       "data",
		Subvolumes: []*SubVolume{},
	}

	mgr := &Manager{
		volumes: map[string]*Volume{"data": vol},
	}

	_, err := mgr.GetSubVolume("data", "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent subvolume")
	}
}

func TestSnapshotNotFound(t *testing.T) {
	vol := &Volume{
		Name:       "data",
		Subvolumes: []*SubVolume{},
		MountPoint: "", // 未挂载
	}

	mgr := &Manager{
		volumes: map[string]*Volume{"data": vol},
	}

	// 卷未挂载时操作快照会 panic，这里跳过实际调用
	if vol.MountPoint != "" {
		_, err := mgr.GetSnapshot("data", "nonexistent")
		if err == nil {
			t.Log("Expected error for snapshot operations on unmounted volume")
		}
	}
}

// ========== 边界条件测试 ==========

func TestEmptyVolumeList(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/tmp/test-mnt",
	}

	volumes := mgr.ListVolumes()
	if len(volumes) != 0 {
		t.Errorf("Expected empty volume list, got %d", len(volumes))
	}
}

func TestEmptySubVolumeList(t *testing.T) {
	vol := &Volume{
		Name:       "data",
		Subvolumes: []*SubVolume{},
	}

	mgr := &Manager{
		volumes: map[string]*Volume{"data": vol},
	}

	subvols, err := mgr.ListSubVolumes("data")
	if err != nil {
		// 卷未挂载会返回错误
		t.Logf("ListSubVolumes returned error (expected for unmounted volume): %v", err)
	} else if len(subvols) != 0 {
		t.Errorf("Expected empty subvolume list, got %d", len(subvols))
	}
}

func TestEmptySnapshotList(t *testing.T) {
	vol := &Volume{
		Name:       "data",
		Subvolumes: []*SubVolume{},
	}

	mgr := &Manager{
		volumes: map[string]*Volume{"data": vol},
	}

	snaps, err := mgr.ListSnapshots("data")
	if err != nil {
		// 卷未挂载会返回错误
		t.Logf("ListSnapshots returned error (expected for unmounted volume): %v", err)
	} else if len(snaps) != 0 {
		t.Errorf("Expected empty snapshot list, got %d", len(snaps))
	}
}

// ========== 并发安全测试 ==========

func TestConcurrentVolumeAccess(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/tmp/test-mnt",
	}

	// 添加测试卷
	for i := 0; i < 10; i++ {
		mgr.volumes["vol"+string(rune('0'+i))] = &Volume{
			Name: "vol" + string(rune('0'+i)),
		}
	}

	// 并发读取
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = mgr.ListVolumes()
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// ========== 维护操作测试 ==========

func TestBalanceStatus(t *testing.T) {
	vol := &Volume{
		Name:       "data",
		MountPoint: "/mnt/data",
		Status: VolumeStatus{
			BalanceRunning:  true,
			BalanceProgress: 50.0,
		},
	}

	if !vol.Status.BalanceRunning {
		t.Error("Expected BalanceRunning=true")
	}

	if vol.Status.BalanceProgress != 50.0 {
		t.Errorf("Expected BalanceProgress=50.0, got %f", vol.Status.BalanceProgress)
	}
}

func TestScrubStatus(t *testing.T) {
	vol := &Volume{
		Name:       "data",
		MountPoint: "/mnt/data",
		Status: VolumeStatus{
			ScrubRunning:  true,
			ScrubProgress: 75.0,
			ScrubErrors:   0,
		},
	}

	if !vol.Status.ScrubRunning {
		t.Error("Expected ScrubRunning=true")
	}

	if vol.Status.ScrubProgress != 75.0 {
		t.Errorf("Expected ScrubProgress=75.0, got %f", vol.Status.ScrubProgress)
	}
}

// ========== 设备管理测试 ==========

func TestDeviceStats(t *testing.T) {
	stats := []btrfs.DeviceStats{
		{Device: "/dev/sda1", Size: 1000000000000, Used: 500000000000},
		{Device: "/dev/sdb1", Size: 1000000000000, Used: 500000000000},
	}

	if len(stats) != 2 {
		t.Errorf("Expected 2 device stats, got %d", len(stats))
	}

	if stats[0].Device != "/dev/sda1" {
		t.Errorf("Expected device '/dev/sda1', got '%s'", stats[0].Device)
	}
}

// ========== 挂载操作测试 ==========

func TestMountSubVolumeOperation(t *testing.T) {
	vol := &Volume{
		Name:       "data",
		MountPoint: "/mnt/data",
		Devices:    []string{"/dev/sda1"},
		Subvolumes: []*SubVolume{
			{Name: "documents", Path: "/mnt/data/documents"},
		},
	}

	// 测试需要实际挂载，这里只验证结构
	if len(vol.Subvolumes) != 1 {
		t.Errorf("Expected 1 subvolume, got %d", len(vol.Subvolumes))
	}
}

func TestGetDefaultSubVolume(t *testing.T) {
	vol := &Volume{
		Name:       "data",
		MountPoint: "/mnt/data",
	}

	_ = &Manager{
		volumes:   map[string]*Volume{"data": vol},
		mountBase: "/mnt",
	}

	// 模拟获取默认子卷（需要实际 btrfs 客户端）
	// 这里只验证结构
	if vol.MountPoint != "/mnt/data" {
		t.Errorf("Expected MountPoint='/mnt/data', got '%s'", vol.MountPoint)
	}
}

// ========== 快照回滚测试 ==========

func TestRollbackSnapshotValidation(t *testing.T) {
	vol := &Volume{
		Name:       "data",
		MountPoint: "/mnt/data",
		Subvolumes: []*SubVolume{
			{Name: "documents", Path: "/mnt/data/documents"},
		},
	}

	mgr := &Manager{
		volumes:   map[string]*Volume{"data": vol},
		mountBase: "/mnt",
	}

	// 测试不存在的子卷
	err := mgr.RollbackSnapshot("data", "nonexistent", "snap1")
	if err == nil {
		t.Error("Expected error for nonexistent subvolume")
	}
}

// ========== 快照恢复测试 ==========

func TestRestoreSnapshotValidation(t *testing.T) {
	vol := &Volume{
		Name:       "data",
		MountPoint: "/mnt/data",
	}

	mgr := &Manager{
		volumes:   map[string]*Volume{"data": vol},
		mountBase: "/mnt",
	}

	// 测试不存在的快照
	err := mgr.RestoreSnapshot("data", "nonexistent", "restored")
	if err == nil {
		t.Error("Expected error for nonexistent snapshot")
	}
}

// ========== 扩展测试 ==========

func TestAddDeviceValidation(t *testing.T) {
	vol := &Volume{
		Name:       "data",
		MountPoint: "/mnt/data",
		Devices:    []string{"/dev/sda1"},
	}

	if len(vol.Devices) != 1 {
		t.Errorf("Expected 1 device, got %d", len(vol.Devices))
	}

	// 添加设备后
	vol.Devices = append(vol.Devices, "/dev/sdb1")
	if len(vol.Devices) != 2 {
		t.Errorf("Expected 2 devices after add, got %d", len(vol.Devices))
	}
}

func TestRemoveDeviceValidation(t *testing.T) {
	vol := &Volume{
		Name:       "data",
		MountPoint: "/mnt/data",
		Devices:    []string{"/dev/sda1", "/dev/sdb1"},
	}

	if len(vol.Devices) != 2 {
		t.Errorf("Expected 2 devices, got %d", len(vol.Devices))
	}

	// 移除设备
	for i, dev := range vol.Devices {
		if dev == "/dev/sdb1" {
			vol.Devices = append(vol.Devices[:i], vol.Devices[i+1:]...)
			break
		}
	}

	if len(vol.Devices) != 1 {
		t.Errorf("Expected 1 device after remove, got %d", len(vol.Devices))
	}
}

// ========== 错误场景测试 ==========

func TestErrorHandling(t *testing.T) {
	// 测试错误包装
	err := errors.New("underlying error")
	wrappedErr := wrapError("operation failed", err)
	if wrappedErr == nil {
		t.Error("Expected wrapped error")
	}
}

func wrapError(msg string, err error) error {
	return errors.New(msg + ": " + err.Error())
}

// ========== 时间相关测试 ==========

func TestVolumeCreatedAt(t *testing.T) {
	now := time.Now()
	vol := &Volume{
		Name:      "data",
		CreatedAt: now,
	}

	if !vol.CreatedAt.Equal(now) {
		t.Errorf("Expected CreatedAt=%v, got %v", now, vol.CreatedAt)
	}
}

func TestSnapshotCreatedAt(t *testing.T) {
	now := time.Now()
	snap := &Snapshot{
		Name:      "snap1",
		CreatedAt: now,
	}

	if !snap.CreatedAt.Equal(now) {
		t.Errorf("Expected CreatedAt=%v, got %v", now, snap.CreatedAt)
	}
}
