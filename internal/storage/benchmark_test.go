// Package storage_test 存储模块性能基准测试
package storage_test

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"nas-os/internal/storage"
)

// ========== 数据结构基准测试 ==========

// BenchmarkVolume_Creation 创建 Volume 对象性能测试
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

// BenchmarkSubVolume_Creation 创建 SubVolume 对象性能测试
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

// BenchmarkSnapshot_Creation 创建 Snapshot 对象性能测试
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

// BenchmarkRAIDConfig_Lookup RAID 配置查找性能测试
func BenchmarkRAIDConfig_Lookup(b *testing.B) {
	profiles := []string{"single", "raid0", "raid1", "raid5", "raid6", "raid10"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		profile := profiles[i%len(profiles)]
		_ = storage.RAIDConfigs[profile]
	}
}

// ========== 内存分配基准测试 ==========

// BenchmarkMemory_VolumeSlice Volume 切片内存分配
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

// BenchmarkMemory_VolumeMap Volume Map 内存分配
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

// BenchmarkMemory_LargeSlice 大切片内存分配
func BenchmarkMemory_LargeSlice(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		items := make([]*storage.Volume, 0, 10000)
		for j := 0; j < 10000; j++ {
			items = append(items, &storage.Volume{
				Name: fmt.Sprintf("vol-%d", j),
			})
		}
		_ = items
	}
}

// ========== 并发基准测试 ==========

// BenchmarkConcurrency_Mutex Mutex 并发性能
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

// BenchmarkConcurrency_RWMutex_Read RWMutex 读并发性能
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

// BenchmarkConcurrency_RWMutex_Write RWMutex 写并发性能
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

// BenchmarkConcurrency_RWMutex_Mixed RWMutex 混合读写性能
func BenchmarkConcurrency_RWMutex_Mixed(b *testing.B) {
	var mu sync.RWMutex
	data := make(map[string]int)
	for i := 0; i < 100; i++ {
		data[fmt.Sprintf("key-%d", i)] = i
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%10 == 0 {
				mu.Lock()
				data[fmt.Sprintf("key-%d", i%100)] = i
				mu.Unlock()
			} else {
				mu.RLock()
				_ = data[fmt.Sprintf("key-%d", i%100)]
				mu.RUnlock()
			}
			i++
		}
	})
}

// ========== GC 压力测试 ==========

// BenchmarkGC_Pressure GC 压力测试
func BenchmarkGC_Pressure(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			_ = &storage.Volume{
				Name:    fmt.Sprintf("gc-test-%d-%d", i, j),
				Devices: make([]string, 10),
			}
		}
	}
}

// BenchmarkGC_WithManualTrigger 手动触发 GC 测试
func BenchmarkGC_WithManualTrigger(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			_ = &storage.Volume{
				Name:    fmt.Sprintf("gc-test-%d-%d", i, j),
				Devices: make([]string, 10),
			}
		}
		if i%100 == 0 {
			runtime.GC()
		}
	}
}

// ========== 分布式存储基准测试 ==========

// BenchmarkStorageNode_Creation 创建存储节点性能测试
func BenchmarkStorageNode_Creation(b *testing.B) {
	now := time.Now()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &storage.Node{
			ID:          fmt.Sprintf("node-%d", i),
			Name:        fmt.Sprintf("node-%d", i),
			Address:     "192.168.1.1:8080",
			Status:      storage.NodeStatusOnline,
			Capacity:    10000000000000,
			Used:        5000000000000,
			Available:   5000000000000,
			Zone:        "zone-1",
			Region:      "region-1",
			Labels:      map[string]string{"type": "ssd"},
			LastCheck:   now,
			LastOnline:  now,
			HealthScore: 100,
			Version:     "1.0.0",
			CreatedAt:   now,
		}
	}
}

// BenchmarkShard_Creation 创建分片性能测试
func BenchmarkShard_Creation(b *testing.B) {
	now := time.Now()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &storage.Shard{
			ID:           fmt.Sprintf("shard-%d", i),
			PoolID:       "pool-1",
			ShardIndex:   i,
			PrimaryNode:  "node-1",
			ReplicaNodes: []string{"node-2", "node-3"},
			Size:         1000000000,
			ObjectCount:  10000,
			Status:       "active",
			CreatedAt:    now,
		}
	}
}

// BenchmarkReplicaPolicy_Creation 创建副本策略性能测试
func BenchmarkReplicaPolicy_Creation(b *testing.B) {
	now := time.Now()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &storage.ReplicaPolicy{
			ID:               fmt.Sprintf("policy-%d", i),
			Name:             fmt.Sprintf("policy-%d", i),
			ReplicaCount:     3,
			Strategy:         storage.ReplicaSync,
			ConsistencyLevel: "quorum",
			WriteQuorum:      2,
			ReadQuorum:       2,
			RepairInterval:   time.Hour,
			MaxLatency:       100 * time.Millisecond,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
	}
}
