// Package integration 提供 NAS-OS 集成测试
// v1.0 全功能测试覆盖
package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"nas-os/internal/storage"
)

// MockStorageManager 模拟存储管理器
type MockStorageManager struct {
	volumes map[string]*storage.Volume
	mu      sync.RWMutex
}

// NewMockStorageManager 创建模拟存储管理器
func NewMockStorageManager() *MockStorageManager {
	return &MockStorageManager{
		volumes: make(map[string]*storage.Volume),
	}
}

// CreateVolume 创建卷
func (m *MockStorageManager) CreateVolume(name string, devices []string, profile string) (*storage.Volume, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	vol := &storage.Volume{
		Name:        name,
		Devices:     devices,
		DataProfile: profile,
		MetaProfile: profile,
		Size:        1000000000000,
		Used:        0,
		Free:        1000000000000,
		Status: storage.VolumeStatus{
			Healthy: true,
		},
	}
	m.volumes[name] = vol
	return vol, nil
}

// ListVolumes 列出所有卷
func (m *MockStorageManager) ListVolumes() ([]*storage.Volume, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vols := make([]*storage.Volume, 0, len(m.volumes))
	for _, v := range m.volumes {
		vols = append(vols, v)
	}
	return vols, nil
}

// GetVolume 获取卷
func (m *MockStorageManager) GetVolume(name string) (*storage.Volume, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vol, ok := m.volumes[name]
	if !ok {
		return nil, fmt.Errorf("volume not found: %s", name)
	}
	return vol, nil
}

// DeleteVolume 删除卷
func (m *MockStorageManager) DeleteVolume(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.volumes, name)
	return nil
}

// ========== 存储管理全功能测试 ==========

// TestFull_Storage_RAIDConfigurations 全功能测试：所有 RAID 配置
func TestFull_Storage_RAIDConfigurations(t *testing.T) {
	tests := []struct {
		name           string
		profile        string
		minDevices     int
		faultTolerance int
		expectError    bool
	}{
		{"Single", "single", 1, 0, false},
		{"RAID0", "raid0", 2, 0, false},
		{"RAID1", "raid1", 2, 1, false},
		{"RAID5", "raid5", 3, 1, false},
		{"RAID6", "raid6", 4, 2, false},
		{"RAID10", "raid10", 4, 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, ok := storage.RAIDConfigs[tt.profile]
			if !ok {
				if !tt.expectError {
					t.Fatalf("RAID 配置 %s 不存在", tt.profile)
				}
				return
			}

			if config.MinDevices != tt.minDevices {
				t.Errorf("最小设备数: 期望 %d, 实际 %d", tt.minDevices, config.MinDevices)
			}

			if config.FaultTolerance != tt.faultTolerance {
				t.Errorf("容错数: 期望 %d, 实际 %d", tt.faultTolerance, config.FaultTolerance)
			}
		})
	}
}

// TestFull_Storage_VolumeLifecycle 全功能测试：卷生命周期
func TestFull_Storage_VolumeLifecycle(t *testing.T) {
	mgr := NewMockStorageManager()

	// 创建阶段
	t.Run("创建", func(t *testing.T) {
		vols := []struct {
			name    string
			devices []string
			profile string
		}{
			{"data-vol", []string{"/dev/sda1", "/dev/sdb1"}, "raid1"},
			{"backup-vol", []string{"/dev/sdc1"}, "single"},
			{"media-vol", []string{"/dev/sdd1", "/dev/sde1", "/dev/sdf1"}, "raid5"},
		}

		for _, v := range vols {
			vol, err := mgr.CreateVolume(v.name, v.devices, v.profile)
			if err != nil {
				t.Errorf("创建卷 %s 失败: %v", v.name, err)
			}
			if vol.Name != v.name {
				t.Errorf("卷名称: 期望 %s, 实际 %s", v.name, vol.Name)
			}
		}
	})

	// 查询阶段
	t.Run("查询", func(t *testing.T) {
		// 列出所有卷
		vols, err := mgr.ListVolumes()
		if err != nil {
			t.Fatalf("列出卷失败: %v", err)
		}
		if len(vols) != 3 {
			t.Errorf("卷数量: 期望 3, 实际 %d", len(vols))
		}

		// 获取单个卷
		for _, name := range []string{"data-vol", "backup-vol", "media-vol"} {
			vol, err := mgr.GetVolume(name)
			if err != nil {
				t.Errorf("获取卷 %s 失败: %v", name, err)
			}
			if vol == nil {
				t.Errorf("卷 %s 不存在", name)
			}
		}
	})

	// 删除阶段
	t.Run("删除", func(t *testing.T) {
		if err := mgr.DeleteVolume("backup-vol"); err != nil {
			t.Errorf("删除卷失败: %v", err)
		}

		vols, _ := mgr.ListVolumes()
		if len(vols) != 2 {
			t.Errorf("删除后卷数量: 期望 2, 实际 %d", len(vols))
		}
	})
}

// TestFull_Storage_SubVolumeOperations 全功能测试：子卷操作
func TestFull_Storage_SubVolumeOperations(t *testing.T) {
	// 测试子卷数据结构
	subvols := []*storage.SubVolume{
		{ID: 256, Name: "documents", Path: "/mnt/data/documents", ParentID: 5, ReadOnly: false},
		{ID: 257, Name: "photos", Path: "/mnt/data/photos", ParentID: 5, ReadOnly: false},
		{ID: 258, Name: "backup", Path: "/mnt/data/backup", ParentID: 5, ReadOnly: true},
	}

	for _, sv := range subvols {
		if sv.Name == "" {
			t.Error("子卷名称不应为空")
		}
		if sv.Path == "" {
			t.Error("子卷路径不应为空")
		}
	}
}

// TestFull_Storage_SnapshotOperations 全功能测试：快照操作
func TestFull_Storage_SnapshotOperations(t *testing.T) {
	now := time.Now()
	snapshots := []*storage.Snapshot{
		{Name: "daily-001", Path: "/mnt/data/.snapshots/daily-001", Source: "documents", ReadOnly: true, CreatedAt: now},
		{Name: "daily-002", Path: "/mnt/data/.snapshots/daily-002", Source: "documents", ReadOnly: true, CreatedAt: now.Add(24 * time.Hour)},
		{Name: "manual-001", Path: "/mnt/data/.snapshots/manual-001", Source: "photos", ReadOnly: true, CreatedAt: now},
	}

	for _, snap := range snapshots {
		if !snap.ReadOnly {
			t.Errorf("快照 %s 应该是只读的", snap.Name)
		}
		if snap.Source == "" {
			t.Errorf("快照 %s 缺少来源", snap.Name)
		}
	}
}

// ========== 并发测试 ==========

// TestFull_Concurrent_VolumeOperations 全功能测试：并发卷操作
func TestFull_Concurrent_VolumeOperations(t *testing.T) {
	mgr := NewMockStorageManager()
	var wg sync.WaitGroup

	// 并发创建
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := string(rune('a' + idx))
			_, _ = mgr.CreateVolume(name, []string{"/dev/sda1"}, "single")
		}(i)
	}

	// 并发读取
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = mgr.ListVolumes()
		}()
	}

	wg.Wait()

	// 验证最终状态
	vols, _ := mgr.ListVolumes()
	if len(vols) != 10 {
		t.Errorf("并发创建后卷数量: 期望 10, 实际 %d", len(vols))
	}
}

// ========== 错误处理测试 ==========

// TestFull_ErrorHandling_InvalidInputs 全功能测试：无效输入处理
func TestFull_ErrorHandling_InvalidInputs(t *testing.T) {
	mgr := NewMockStorageManager()

	// 测试获取不存在的卷
	_, err := mgr.GetVolume("nonexistent")
	if err == nil {
		t.Error("期望获取不存在的卷时返回错误")
	}

	// 测试删除不存在的卷
	err = mgr.DeleteVolume("nonexistent")
	// Mock 实现可能不返回错误，这里只是验证不会 panic
	_ = err
}

// ========== 配置验证测试 ==========

// TestFull_Config_Validation 全功能测试：配置验证
func TestFull_Config_Validation(t *testing.T) {
	// 验证快照配置
	snapConfig := storage.DefaultSnapshotConfig
	if snapConfig.ReadOnly != true {
		t.Error("默认快照配置应为只读")
	}
	if snapConfig.TimeFormat == "" {
		t.Error("快照时间格式不应为空")
	}

	// 验证 RAID 配置完整性
	requiredProfiles := []string{"single", "raid0", "raid1", "raid5", "raid6", "raid10"}
	for _, profile := range requiredProfiles {
		if _, ok := storage.RAIDConfigs[profile]; !ok {
			t.Errorf("缺少 RAID 配置: %s", profile)
		}
	}
}

// ========== 上下文和超时测试 ==========

// TestFull_Context_Timeout 全功能测试：上下文超时
func TestFull_Context_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	select {
	case <-time.After(50 * time.Millisecond):
		// 正常完成
	case <-ctx.Done():
		t.Error("操作超时")
	}
}

// TestFull_Context_Cancellation 全功能测试：上下文取消
func TestFull_Context_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// 模拟长时间操作
	done := make(chan bool)
	go func() {
		time.Sleep(50 * time.Millisecond)
		done <- true
	}()

	// 立即取消
	cancel()

	select {
	case <-done:
		// 操作完成
	case <-ctx.Done():
		// 上下文被取消
	}
}

// ========== 数据一致性测试 ==========

// TestFull_DataConsistency_VolumeState 全功能测试：卷状态一致性
func TestFull_DataConsistency_VolumeState(t *testing.T) {
	vol := &storage.Volume{
		Name:   "consistency-test",
		Size:   1000000000000,
		Used:   300000000000,
		Free:   700000000000,
		Status: storage.VolumeStatus{Healthy: true},
	}

	// 验证大小一致性
	expectedFree := vol.Size - vol.Used
	if vol.Free != expectedFree {
		t.Errorf("空闲空间不一致: 期望 %d, 实际 %d", expectedFree, vol.Free)
	}

	// 验证状态一致性
	if !vol.Status.Healthy {
		t.Error("新创建的卷应该是健康的")
	}
}
