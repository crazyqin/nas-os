// Package backup_test 备份模块性能基准测试
package backup_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"nas-os/internal/backup"
)

// ========== 数据结构基准测试 ==========

// BenchmarkJobConfig_Creation 创建配置性能测试.
func BenchmarkJobConfig_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &backup.JobConfig{
			ID:           fmt.Sprintf("cfg-%d", i),
			Name:         fmt.Sprintf("backup-%d", i),
			Source:       "/tmp/source",
			Destination:  "/tmp/dest",
			Type:         backup.BackupTypeLocal,
			Schedule:     "daily",
			Retention:    7,
			Enabled:      true,
			Exclude:      []string{"*.log", "*.tmp"},
			RsyncOptions: []string{"-av", "--progress"},
		}
	}
}

// BenchmarkBackupTask_Creation 创建任务性能测试.
func BenchmarkBackupTask_Creation(b *testing.B) {
	now := time.Now()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &backup.BackupTask{
			ID:         fmt.Sprintf("task-%d", i),
			ConfigID:   fmt.Sprintf("cfg-%d", i),
			Status:     backup.TaskStatusRunning,
			StartTime:  now,
			Progress:   50,
			TotalSize:  1000000000,
			TotalFiles: 10000,
			Speed:      50000000,
		}
	}
}

// BenchmarkBackupHistory_Creation 历史记录创建性能测试.
func BenchmarkBackupHistory_Creation(b *testing.B) {
	now := time.Now()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &backup.BackupHistory{
			ID:        fmt.Sprintf("history-%d", i),
			ConfigID:  fmt.Sprintf("cfg-%d", i),
			Name:      fmt.Sprintf("backup-%d", i),
			Type:      backup.BackupTypeLocal,
			Size:      1000000000,
			FileCount: 10000,
			Duration:  3600,
			CreatedAt: now,
			Path:      "/backup/path",
			Verified:  true,
		}
	}
}

// BenchmarkBackupStats_Creation 统计信息创建性能测试.
func BenchmarkBackupStats_Creation(b *testing.B) {
	now := time.Now()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &backup.BackupStats{
			TotalBackups:    100,
			TotalSize:       100000000000,
			TotalSizeHuman:  "100 GB",
			AvgDuration:     time.Hour,
			AverageDuration: time.Hour,
			SuccessCount:    95,
			FailedCount:     5,
			SuccessRate:     0.95,
			LastBackupTime:  now,
			NextBackupTime:  now.Add(24 * time.Hour),
		}
	}
}

// ========== 内存分配基准测试 ==========

// BenchmarkMemory_ConfigSlice 配置切片内存分配.
func BenchmarkMemory_ConfigSlice(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		configs := make([]*backup.JobConfig, 0, 100)
		for j := 0; j < 100; j++ {
			configs = append(configs, &backup.JobConfig{
				ID:   fmt.Sprintf("cfg-%d", j),
				Name: fmt.Sprintf("backup-%d", j),
			})
		}
		_ = configs
	}
}

// BenchmarkMemory_TaskMap 任务 Map 内存分配.
func BenchmarkMemory_TaskMap(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tasks := make(map[string]*backup.BackupTask, 100)
		for j := 0; j < 100; j++ {
			id := fmt.Sprintf("task-%d", j)
			tasks[id] = &backup.BackupTask{ID: id}
		}
		_ = tasks
	}
}

// ========== 并发基准测试 ==========

// BenchmarkMutex_Read 读锁性能.
func BenchmarkMutex_Read(b *testing.B) {
	var mu sync.RWMutex
	configs := make(map[string]*backup.JobConfig)
	for i := 0; i < 100; i++ {
		configs[fmt.Sprintf("cfg-%d", i)] = &backup.JobConfig{ID: fmt.Sprintf("cfg-%d", i)}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mu.RLock()
		_ = configs[fmt.Sprintf("cfg-%d", i%100)]
		mu.RUnlock()
	}
}

// BenchmarkMutex_Write 写锁性能.
func BenchmarkMutex_Write(b *testing.B) {
	var mu sync.RWMutex
	configs := make(map[string]*backup.JobConfig)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mu.Lock()
		configs[fmt.Sprintf("cfg-%d", i%100)] = &backup.JobConfig{ID: fmt.Sprintf("cfg-%d", i)}
		mu.Unlock()
	}
}

// BenchmarkMutex_Parallel 并行读写测试.
func BenchmarkMutex_Parallel(b *testing.B) {
	var mu sync.RWMutex
	configs := make(map[string]*backup.JobConfig)
	for i := 0; i < 100; i++ {
		configs[fmt.Sprintf("cfg-%d", i)] = &backup.JobConfig{ID: fmt.Sprintf("cfg-%d", i)}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%10 == 0 {
				mu.Lock()
				configs[fmt.Sprintf("cfg-%d", i%100)] = &backup.JobConfig{ID: fmt.Sprintf("cfg-%d", i)}
				mu.Unlock()
			} else {
				mu.RLock()
				_ = configs[fmt.Sprintf("cfg-%d", i%100)]
				mu.RUnlock()
			}
			i++
		}
	})
}

// ========== Context 基准测试 ==========

// BenchmarkContext_Timeout Context 超时性能.
func BenchmarkContext_Timeout(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_ = ctx
		cancel()
	}
}

// BenchmarkContext_Cancel Context 取消性能.
func BenchmarkContext_Cancel(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		_ = ctx
		cancel()
	}
}

// ========== 类型常量基准测试 ==========

// BenchmarkBackupType_Compare 备份类型比较性能.
func BenchmarkBackupType_Compare(b *testing.B) {
	types := []backup.BackupType{
		backup.BackupTypeLocal,
		backup.BackupTypeRemote,
		backup.BackupTypeRsync,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t := types[i%3]
		_ = t == backup.BackupTypeLocal
		_ = t == backup.BackupTypeRemote
		_ = t == backup.BackupTypeRsync
	}
}

// BenchmarkTaskStatus_Compare 任务状态比较性能.
func BenchmarkTaskStatus_Compare(b *testing.B) {
	statuses := []backup.TaskStatus{
		backup.TaskStatusPending,
		backup.TaskStatusRunning,
		backup.TaskStatusCompleted,
		backup.TaskStatusFailed,
		backup.TaskStatusCancelled,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := statuses[i%5]
		_ = s == backup.TaskStatusRunning
		_ = s == backup.TaskStatusCompleted
		_ = s == backup.TaskStatusFailed
	}
}
