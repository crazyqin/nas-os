// Package replication 复制模块性能基准测试
package replication

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// ========== 复制管理器基准测试 ==========

// BenchmarkReplicationManager_CreateTask 创建任务性能测试.
func BenchmarkReplicationManager_CreateTask(b *testing.B) {
	mgr := setupTestReplicationManager(b)
	defer mgr.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task := &Task{
			Name:       fmt.Sprintf("task-%d", i),
			SourcePath: "/tmp/source",
			TargetPath: "/tmp/target",
			Type:       TypeScheduled,
			Schedule:   "hourly",
			Enabled:    true,
			Compress:   true,
		}
		_ = mgr.CreateTask(task)
	}
}

// BenchmarkReplicationManager_ListTasks 列出任务性能测试.
func BenchmarkReplicationManager_ListTasks(b *testing.B) {
	mgr := setupTestReplicationManager(b)
	defer mgr.Stop()

	// 预创建 50 个任务
	for i := 0; i < 50; i++ {
		task := &Task{
			Name:       fmt.Sprintf("task-%d", i),
			SourcePath: "/tmp/source",
			TargetPath: "/tmp/target",
			Type:       TypeScheduled,
			Enabled:    true,
		}
		_ = mgr.CreateTask(task)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.ListTasks()
	}
}

// BenchmarkReplicationManager_GetTask 获取任务性能测试.
func BenchmarkReplicationManager_GetTask(b *testing.B) {
	mgr := setupTestReplicationManager(b)
	defer mgr.Stop()

	task := &Task{
		Name:       "test-task",
		SourcePath: "/tmp/source",
		TargetPath: "/tmp/target",
		Type:       TypeScheduled,
		Enabled:    true,
	}
	_ = mgr.CreateTask(task)

	tasks := mgr.ListTasks()
	if len(tasks) == 0 {
		b.Skip("No tasks created")
	}
	taskID := tasks[0].ID

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mgr.GetTask(taskID)
	}
}

// BenchmarkReplicationManager_ConcurrentAccess 并发访问测试.
func BenchmarkReplicationManager_ConcurrentAccess(b *testing.B) {
	mgr := setupTestReplicationManager(b)
	defer mgr.Stop()

	// 预创建任务
	for i := 0; i < 20; i++ {
		task := &Task{
			Name:       fmt.Sprintf("task-%d", i),
			SourcePath: "/tmp/source",
			TargetPath: "/tmp/target",
			Type:       TypeScheduled,
			Enabled:    true,
		}
		_ = mgr.CreateTask(task)
	}

	tasks := mgr.ListTasks()
	taskIDs := make([]string, len(tasks))
	for i, t := range tasks {
		taskIDs[i] = t.ID
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if len(taskIDs) > 0 {
				_, _ = mgr.GetTask(taskIDs[i%len(taskIDs)])
			}
			i++
		}
	})
}

// ========== 冲突检测基准测试 ==========

// BenchmarkConflictDetector_Detect 检测冲突性能测试.
func BenchmarkConflictDetector_Detect(b *testing.B) {
	detector := NewConflictDetector(ConflictNewerWins)

	// 创建测试冲突
	conflicts := make([]*ConflictInfo, 100)
	for i := 0; i < 100; i++ {
		conflicts[i] = &ConflictInfo{
			ID:            fmt.Sprintf("conflict-%d", i),
			TaskID:        "test-task",
			SourcePath:    fmt.Sprintf("/src/file-%d.txt", i),
			TargetPath:    fmt.Sprintf("/dst/file-%d.txt", i),
			SourceModTime: time.Now().Add(-time.Hour),
			TargetModTime: time.Now(),
			Strategy:      ConflictNewerWins,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, c := range conflicts {
			_ = detector.ResolveConflict(c)
		}
	}
}

// BenchmarkConflictDetector_Resolve 解决冲突性能测试.
func BenchmarkConflictDetector_Resolve(b *testing.B) {
	detector := NewConflictDetector(ConflictNewerWins)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conflict := &ConflictInfo{
			ID:            fmt.Sprintf("conflict-%d", i),
			TaskID:        "test-task",
			SourcePath:    "/src/file.txt",
			TargetPath:    "/dst/file.txt",
			SourceModTime: time.Now().Add(-time.Hour),
			TargetModTime: time.Now(),
			Strategy:      ConflictNewerWins,
		}
		_ = detector.ResolveConflict(conflict)
	}
}

// ========== rsync 输出解析基准测试 ==========

// BenchmarkRsyncOutput_Parse 解析 rsync 输出性能测试.
func BenchmarkRsyncOutput_Parse(b *testing.B) {
	output := `sending incremental file list
file1.txt
file2.txt
dir1/
dir1/file3.txt

sent 1,234,567 bytes  received 1,234 bytes  823,179.33 bytes/sec
total size is 1,234,567,890  speedup is 1,000.00
Number of regular files transferred: 3`

	mgr := setupTestReplicationManager(b)
	defer mgr.Stop()

	task := &Task{
		Name:       "parse-test",
		SourcePath: "/tmp/source",
		TargetPath: "/tmp/target",
		Type:       TypeScheduled,
		Enabled:    true,
	}
	_ = mgr.CreateTask(task)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.parseRsyncOutput(task, output)
	}
}

// ========== 调度计算基准测试 ==========

// BenchmarkSchedule_CalculateNext 计算下次同步时间性能测试.
func BenchmarkSchedule_CalculateNext(b *testing.B) {
	mgr := setupTestReplicationManager(b)
	defer mgr.Stop()

	schedules := []string{"hourly", "daily", "weekly", "0 0 * * *", "*/5 * * * *"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task := &Task{
			Schedule: schedules[i%len(schedules)],
		}
		_ = mgr.calculateNextSync(task)
	}
}

// ========== 统计信息基准测试 ==========

// BenchmarkStats_GetStats 获取统计信息性能测试.
func BenchmarkStats_GetStats(b *testing.B) {
	mgr := setupTestReplicationManager(b)
	defer mgr.Stop()

	// 预创建任务
	for i := 0; i < 10; i++ {
		task := &Task{
			Name:       fmt.Sprintf("stats-task-%d", i),
			SourcePath: "/tmp/source",
			TargetPath: "/tmp/target",
			Type:       TypeScheduled,
			Enabled:    true,
		}
		_ = mgr.CreateTask(task)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.GetStats()
	}
}

// ========== 内存分配基准测试 ==========

// BenchmarkMemory_Task 任务结构体内存分配.
func BenchmarkMemory_Task(b *testing.B) {
	now := time.Now()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &Task{
			ID:               fmt.Sprintf("task-%d", i),
			Name:             fmt.Sprintf("task-name-%d", i),
			SourcePath:       "/tmp/source",
			TargetPath:       "/tmp/target",
			TargetHost:       "remote.example.com",
			Type:             TypeScheduled,
			Status:           StatusIdle,
			Schedule:         "hourly",
			LastSyncAt:       now,
			NextSyncAt:       now.Add(time.Hour),
			BytesTransferred: 1000000000,
			TotalBytes:       2000000000,
			FilesCount:       1000,
			CreatedAt:        now,
			UpdatedAt:        now,
			Enabled:          true,
			Compress:         true,
			DeleteExtraneous: true,
		}
	}
}

// BenchmarkMemory_ConflictInfo 冲突信息内存分配.
func BenchmarkMemory_ConflictInfo(b *testing.B) {
	now := time.Now()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &ConflictInfo{
			ID:            fmt.Sprintf("conflict-%d", i),
			TaskID:        fmt.Sprintf("task-%d", i),
			SourcePath:    "/src/file.txt",
			TargetPath:    "/dst/file.txt",
			SourceModTime: now,
			TargetModTime: now,
			Strategy:      ConflictNewerWins,
		}
	}
}

// BenchmarkMemory_Config 配置结构体内存分配.
func BenchmarkMemory_Config(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &Config{
			MaxConcurrentTasks: 2,
			BandwidthLimit:     1000,
			SSHKeyPath:         "~/.ssh/id_rsa",
			Retries:            3,
			Timeout:            3600,
		}
	}
}

// BenchmarkMemory_TaskSlice 任务切片内存分配.
func BenchmarkMemory_TaskSlice(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tasks := make([]*Task, 0, 1000)
		for j := 0; j < 1000; j++ {
			tasks = append(tasks, &Task{
				ID:   fmt.Sprintf("task-%d", j),
				Name: fmt.Sprintf("task-%d", j),
			})
		}
		_ = tasks
	}
}

// BenchmarkMemory_TaskMap 任务 Map 内存分配.
func BenchmarkMemory_TaskMap(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tasks := make(map[string]*Task, 1000)
		for j := 0; j < 1000; j++ {
			id := fmt.Sprintf("task-%d", j)
			tasks[id] = &Task{ID: id}
		}
		_ = tasks
	}
}

// ========== 并发锁基准测试 ==========

// BenchmarkReplicationMutex_Read 读锁性能.
func BenchmarkReplicationMutex_Read(b *testing.B) {
	var mu sync.RWMutex
	tasks := make(map[string]*Task)
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("task-%d", i)
		tasks[id] = &Task{ID: id}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mu.RLock()
		_ = tasks[fmt.Sprintf("task-%d", i%100)]
		mu.RUnlock()
	}
}

// BenchmarkReplicationMutex_Write 写锁性能.
func BenchmarkReplicationMutex_Write(b *testing.B) {
	var mu sync.RWMutex
	tasks := make(map[string]*Task)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mu.Lock()
		tasks[fmt.Sprintf("task-%d", i%100)] = &Task{ID: fmt.Sprintf("task-%d", i)}
		mu.Unlock()
	}
}

// BenchmarkReplicationMutex_Parallel 并行读写测试.
func BenchmarkReplicationMutex_Parallel(b *testing.B) {
	var mu sync.RWMutex
	tasks := make(map[string]*Task)
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("task-%d", i)
		tasks[id] = &Task{ID: id}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%10 == 0 {
				mu.Lock()
				tasks[fmt.Sprintf("task-%d", i%100)] = &Task{ID: fmt.Sprintf("task-%d", i)}
				mu.Unlock()
			} else {
				mu.RLock()
				_ = tasks[fmt.Sprintf("task-%d", i%100)]
				mu.RUnlock()
			}
			i++
		}
	})
}

// setupTestReplicationManager 设置测试复制管理器.
func setupTestReplicationManager(b *testing.B) *Manager {
	configPath := filepath.Join(b.TempDir(), "replication-config.json")

	mgr, err := NewManager(configPath, DefaultConfig())
	if err != nil {
		b.Skipf("无法创建复制管理器: %v", err)
	}
	return mgr
}
