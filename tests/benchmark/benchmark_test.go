// Package benchmark 提供 NAS-OS 性能基准测试
// 测试各模块的性能指标
package benchmark

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"nas-os/internal/storage"
)

// MockStorageManager 模拟存储管理器.
type MockStorageManager struct {
	volumes map[string]*storage.Volume
	mu      sync.RWMutex
}

// NewMockStorageManager 创建模拟存储管理器.
func NewMockStorageManager() *MockStorageManager {
	return &MockStorageManager{
		volumes: make(map[string]*storage.Volume),
	}
}

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

func (m *MockStorageManager) ListVolumes() ([]*storage.Volume, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vols := make([]*storage.Volume, 0, len(m.volumes))
	for _, v := range m.volumes {
		vols = append(vols, v)
	}
	return vols, nil
}

func (m *MockStorageManager) GetVolume(name string) (*storage.Volume, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vol, ok := m.volumes[name]
	if !ok {
		return nil, fmt.Errorf("volume not found: %s", name)
	}
	return vol, nil
}

// ========== 存储模块基准测试 ==========

// BenchmarkStorage_CreateVolume 基准测试：创建卷.
func BenchmarkStorage_CreateVolume(b *testing.B) {
	mgr := NewMockStorageManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		name := fmt.Sprintf("bench-vol-%d", i)
		_, _ = mgr.CreateVolume(name, []string{"/dev/sda1"}, "single")
	}
}

// BenchmarkStorage_ListVolumes 基准测试：列出卷.
func BenchmarkStorage_ListVolumes(b *testing.B) {
	mgr := NewMockStorageManager()

	// 预创建一些卷
	for i := 0; i < 100; i++ {
		name := fmt.Sprintf("vol-%d", i)
		_, _ = mgr.CreateVolume(name, []string{"/dev/sda1"}, "single")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mgr.ListVolumes()
	}
}

// BenchmarkStorage_GetVolume 基准测试：获取卷.
func BenchmarkStorage_GetVolume(b *testing.B) {
	mgr := NewMockStorageManager()
	_, _ = mgr.CreateVolume("test-vol", []string{"/dev/sda1"}, "single")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mgr.GetVolume("test-vol")
	}
}

// BenchmarkStorage_ConcurrentAccess 基准测试：并发访问.
func BenchmarkStorage_ConcurrentAccess(b *testing.B) {
	mgr := NewMockStorageManager()

	// 预创建卷
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("concurrent-vol-%d", i)
		_, _ = mgr.CreateVolume(name, []string{"/dev/sda1"}, "single")
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			name := fmt.Sprintf("concurrent-vol-%d", i%10)
			_, _ = mgr.GetVolume(name)
			i++
		}
	})
}

// ========== RAID 配置基准测试 ==========

// BenchmarkRAIDConfig_Lookup 基准测试：RAID 配置查找.
func BenchmarkRAIDConfig_Lookup(b *testing.B) {
	profiles := []string{"single", "raid0", "raid1", "raid5", "raid6", "raid10"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		profile := profiles[i%len(profiles)]
		_ = storage.RAIDConfigs[profile]
	}
}

// ========== 数据结构基准测试 ==========

// BenchmarkVolume_Creation 基准测试：创建 Volume 对象.
func BenchmarkVolume_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &storage.Volume{
			Name:        "benchmark-vol",
			UUID:        "test-uuid",
			Devices:     []string{"/dev/sda1", "/dev/sdb1"},
			Size:        1000000000000,
			Used:        500000000000,
			Free:        500000000000,
			DataProfile: "raid1",
			MetaProfile: "raid1",
			MountPoint:  "/mnt/data",
			Status: storage.VolumeStatus{
				Healthy:        true,
				BalanceRunning: false,
				ScrubRunning:   false,
			},
		}
	}
}

// BenchmarkSubVolume_Creation 基准测试：创建 SubVolume 对象.
func BenchmarkSubVolume_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &storage.SubVolume{
			ID:       256,
			Name:     "benchmark-subvol",
			Path:     "/mnt/data/benchmark",
			ParentID: 5,
			ReadOnly: false,
			UUID:     "test-uuid",
		}
	}
}

// BenchmarkSnapshot_Creation 基准测试：创建 Snapshot 对象.
func BenchmarkSnapshot_Creation(b *testing.B) {
	now := time.Now()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &storage.Snapshot{
			Name:      "benchmark-snap",
			Path:      "/mnt/data/.snapshots/benchmark",
			Source:    "benchmark-subvol",
			ReadOnly:  true,
			CreatedAt: now,
		}
	}
}

// ========== 内存分配基准测试 ==========

// BenchmarkMemory_VolumeSlice 基准测试：Volume 切片内存分配.
func BenchmarkMemory_VolumeSlice(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		volumes := make([]*storage.Volume, 0, 100)
		for j := 0; j < 100; j++ {
			_ = append(volumes, &storage.Volume{
				Name: fmt.Sprintf("vol-%d", j),
			})
		}
	}
}

// BenchmarkMemory_VolumeMap 基准测试：Volume Map 内存分配.
func BenchmarkMemory_VolumeMap(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		volumes := make(map[string]*storage.Volume, 100)
		for j := 0; j < 100; j++ {
			name := fmt.Sprintf("vol-%d", j)
			volumes[name] = &storage.Volume{Name: name}
		}
	}
}

// ========== 并发基准测试 ==========

// BenchmarkConcurrency_Mutex 基准测试：Mutex 并发.
func BenchmarkConcurrency_Mutex(b *testing.B) {
	var mu sync.Mutex
	data := make(map[string]int)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			mu.Lock()
			data[fmt.Sprintf("key-%d", i%100)] = i
			mu.Unlock()
			i++
		}
	})
}

// BenchmarkConcurrency_RWMutex_Read 基准测试：RWMutex 读并发.
func BenchmarkConcurrency_RWMutex_Read(b *testing.B) {
	var mu sync.RWMutex
	data := make(map[string]int)
	for i := 0; i < 100; i++ {
		data[fmt.Sprintf("key-%d", i)] = i
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			mu.RLock()
			_ = data[fmt.Sprintf("key-%d", i%100)]
			mu.RUnlock()
			i++
		}
	})
}

// BenchmarkConcurrency_RWMutex_Write 基准测试：RWMutex 写并发.
func BenchmarkConcurrency_RWMutex_Write(b *testing.B) {
	var mu sync.RWMutex
	data := make(map[string]int)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			mu.Lock()
			data[fmt.Sprintf("key-%d", i%100)] = i
			mu.Unlock()
			i++
		}
	})
}

// ========== 性能测试入口 ==========

// TestMain 性能测试入口.
func TestMain(m *testing.M) {
	fmt.Println("⚡ NAS-OS 性能基准测试 v1.0")
	fmt.Println("=====================================")
	fmt.Printf("CPU: %d 核\n", runtime.NumCPU())
	fmt.Printf("GOMAXPROCS: %d\n", runtime.GOMAXPROCS(0))
	fmt.Println("=====================================")

	code := m.Run()

	fmt.Println("=====================================")
	fmt.Println("✅ 性能基准测试完成")

	os.Exit(code)
}
