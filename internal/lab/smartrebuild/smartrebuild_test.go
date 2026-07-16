// Package smartrebuild 智能RAID重建引擎 - 单元测试
package smartrebuild

import (
	"context"
	"testing"
)

func TestNewManager(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.jobs == nil {
		t.Error("jobs map is nil")
	}
	if mgr.schedules == nil {
		t.Error("schedules map is nil")
	}
	if mgr.hotDataMap == nil {
		t.Error("hotDataMap is nil")
	}

	// 检查默认值
	if mgr.config.MaxParallelJobs != 2 {
		t.Errorf("expected MaxParallelJobs=2, got %d", mgr.config.MaxParallelJobs)
	}
	if mgr.config.BusinessIOWeight != 0.7 {
		t.Errorf("expected BusinessIOWeight=0.7, got %f", mgr.config.BusinessIOWeight)
	}
}

func TestCreateAndListJobs(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)
	ctx := context.Background()

	// 创建任务
	targetDisk := DiskInfo{
		ID:        "disk-1",
		Path:      "/dev/sda",
		SizeBytes: 1024 * 1024 * 1024 * 100, // 100GB
		Status:    DiskStatusFaulted,
	}

	sourceDisks := []DiskInfo{
		{ID: "disk-2", Path: "/dev/sdb", SizeBytes: 1024 * 1024 * 1024 * 100},
		{ID: "disk-3", Path: "/dev/sdc", SizeBytes: 1024 * 1024 * 1024 * 100},
	}

	job, err := mgr.CreateJob(ctx, "tank", targetDisk, sourceDisks)
	if err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	if job.ID == "" {
		t.Error("job ID is empty")
	}
	if job.State != StatePending {
		t.Errorf("expected state PENDING, got %s", job.State)
	}
	if job.PoolName != "tank" {
		t.Errorf("expected pool tank, got %s", job.PoolName)
	}

	// 列出任务
	jobs := mgr.ListJobs(ctx)
	if len(jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(jobs))
	}

	// 无源盘创建失败
	_, err = mgr.CreateJob(ctx, "tank", targetDisk, nil)
	if err == nil {
		t.Error("expected error for empty source disks")
	}
}

func TestJobLifecycle(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)
	ctx := context.Background()

	targetDisk := DiskInfo{
		ID:        "disk-1",
		Path:      "/dev/sda",
		SizeBytes: 1024 * 1024 * 1024, // 1GB
		Status:    DiskStatusFaulted,
	}
	sourceDisks := []DiskInfo{
		{ID: "disk-2", Path: "/dev/sdb", SizeBytes: 1024 * 1024 * 1024},
	}

	job, _ := mgr.CreateJob(ctx, "tank", targetDisk, sourceDisks)

	// 启动
	if err := mgr.StartJob(ctx, job.ID); err != nil {
		t.Fatalf("StartJob failed: %v", err)
	}

	// 获取任务检查状态
	runningJob, _ := mgr.GetJob(ctx, job.ID)
	if runningJob.State != StateRunning {
		t.Errorf("expected state RUNNING, got %s", runningJob.State)
	}

	// 暂停
	if err := mgr.PauseJob(ctx, job.ID); err != nil {
		t.Fatalf("PauseJob failed: %v", err)
	}

	pausedJob, _ := mgr.GetJob(ctx, job.ID)
	if pausedJob.State != StatePaused {
		t.Errorf("expected state PAUSED, got %s", pausedJob.State)
	}

	// 恢复
	if err := mgr.StartJob(ctx, job.ID); err != nil {
		t.Fatalf("StartJob from paused failed: %v", err)
	}

	// 取消
	if err := mgr.CancelJob(ctx, job.ID); err != nil {
		t.Fatalf("CancelJob failed: %v", err)
	}

	cancelledJob, _ := mgr.GetJob(ctx, job.ID)
	if cancelledJob.State != StateCancelled {
		t.Errorf("expected state CANCELLED, got %s", cancelledJob.State)
	}
}

func TestParallelJobLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxParallelJobs = 2
	mgr := NewManager(cfg)
	ctx := context.Background()

	targetDisk := DiskInfo{
		ID:        "disk-1",
		Path:      "/dev/sda",
		SizeBytes: 1024 * 1024 * 1024,
		Status:    DiskStatusFaulted,
	}
	sourceDisks := []DiskInfo{
		{ID: "disk-2", Path: "/dev/sdb", SizeBytes: 1024 * 1024 * 1024},
	}

	// 创建并启动2个任务
	job1, err1 := mgr.CreateJob(ctx, "tank", targetDisk, sourceDisks)
	if err1 != nil {
		t.Fatalf("CreateJob 1 failed: %v", err1)
	}
	job2, err2 := mgr.CreateJob(ctx, "tank", targetDisk, sourceDisks)
	if err2 != nil {
		t.Fatalf("CreateJob 2 failed: %v", err2)
	}
	mgr.StartJob(ctx, job1.ID)
	mgr.StartJob(ctx, job2.ID)

	// 检查活跃任务数
	if count := mgr.GetActiveJobCount(); count != 2 {
		t.Errorf("expected 2 active jobs, got %d", count)
	}

	// 第3个任务应该失败（并行限制）
	err := mgr.StartJob(ctx, job1.ID) // 尝试重复启动应该失败
	if err == nil {
		t.Error("expected error for starting already running job")
	}
}

func TestPrioritizeSegments(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)

	segments := []DataSegment{
		{ID: "seg-1", HotScore: 0.2, Importance: 0.3},
		{ID: "seg-2", HotScore: 0.9, Importance: 0.8},
		{ID: "seg-3", HotScore: 0.5, Importance: 0.5},
		{ID: "seg-4", HotScore: 0.7, Importance: 0.6},
	}

	prioritized := mgr.PrioritizeSegments(segments)

	// 检查排序
	if len(prioritized) != 4 {
		t.Fatalf("expected 4 segments, got %d", len(prioritized))
	}

	// 第一个应该是最高优先级 (seg-2: 0.9*0.6 + 0.8*0.4 = 0.86)
	if prioritized[0].ID != "seg-2" {
		t.Errorf("expected seg-2 first, got %s", prioritized[0].ID)
	}
	if prioritized[0].Priority != PriorityCritical {
		t.Errorf("expected PriorityCritical, got %d", prioritized[0].Priority)
	}

	// 最后应该是最低优先级 (seg-1: 0.2*0.6 + 0.3*0.4 = 0.24)
	if prioritized[3].ID != "seg-1" {
		t.Errorf("expected seg-1 last, got %s", prioritized[3].ID)
	}
	if prioritized[3].Priority != PriorityLow {
		t.Errorf("expected PriorityLow, got %d", prioritized[3].Priority)
	}
}

func TestProgressUpdateAndETA(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)
	ctx := context.Background()

	targetDisk := DiskInfo{
		ID:        "disk-1",
		Path:      "/dev/sda",
		SizeBytes: 1024 * 1024 * 1024, // 1GB
		Status:    DiskStatusFaulted,
	}
	sourceDisks := []DiskInfo{
		{ID: "disk-2", Path: "/dev/sdb", SizeBytes: 1024 * 1024 * 1024},
	}

	job, _ := mgr.CreateJob(ctx, "tank", targetDisk, sourceDisks)
	mgr.StartJob(ctx, job.ID)

	// 更新进度到50%
	rebuiltBytes := int64(512 * 1024 * 1024) // 512MB
	currentSpeed := int64(100 * 1024 * 1024) // 100MB/s
	if err := mgr.UpdateProgress(job.ID, rebuiltBytes, currentSpeed); err != nil {
		t.Fatalf("UpdateProgress failed: %v", err)
	}

	// 检查进度
	updatedJob, _ := mgr.GetJob(ctx, job.ID)
	if updatedJob.Progress < 49.9 || updatedJob.Progress > 50.1 {
		t.Errorf("expected progress ~50%%, got %f", updatedJob.Progress)
	}

	// 检查ETA
	if updatedJob.ETA <= 0 {
		t.Error("expected positive ETA")
	}

	// 更新到100%应该自动完成
	if err := mgr.UpdateProgress(job.ID, 1024*1024*1024, currentSpeed); err != nil {
		t.Fatalf("UpdateProgress to 100%% failed: %v", err)
	}

	completedJob, _ := mgr.GetJob(ctx, job.ID)
	if completedJob.State != StateCompleted {
		t.Errorf("expected state COMPLETED, got %s", completedJob.State)
	}
	if completedJob.EndTime == nil {
		t.Error("EndTime should be set for completed job")
	}
}

func TestThrottleRebuild(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxDiskSpeedMBps = 200
	cfg.RebuildIOWeight = 0.3
	cfg.TempThreshold = 60
	mgr := NewManager(cfg)
	ctx := context.Background()

	targetDisk := DiskInfo{
		ID:        "disk-1",
		Path:      "/dev/sda",
		SizeBytes: 1024 * 1024 * 1024,
		Status:    DiskStatusFaulted,
		TempC:     50,
	}
	sourceDisks := []DiskInfo{
		{ID: "disk-2", Path: "/dev/sdb", SizeBytes: 1024 * 1024 * 1024},
	}

	job, _ := mgr.CreateJob(ctx, "tank", targetDisk, sourceDisks)
	mgr.StartJob(ctx, job.ID)

	// 测试低业务IO时的限速
	throttledSpeed, err := mgr.ThrottleRebuild(job.ID, 100)
	if err != nil {
		t.Fatalf("ThrottleRebuild failed: %v", err)
	}

	// 预期速度 = 200MB/s * 0.3 = 60MB/s = 62914560 bytes/s
	expectedSpeed := int64(200 * 1024 * 1024 * 0.3)
	if throttledSpeed != expectedSpeed {
		t.Errorf("expected throttled speed %d, got %d", expectedSpeed, throttledSpeed)
	}

	// 测试高业务IO时的限速
	highIOSpeed, _ := mgr.ThrottleRebuild(job.ID, 2000)
	// 高业务负载时，速度降低50%
	expectedHighSpeed := expectedSpeed / 2
	if highIOSpeed != expectedHighSpeed {
		t.Errorf("expected high IO speed %d, got %d", expectedHighSpeed, highIOSpeed)
	}
}

func TestScheduleCRUD(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)
	ctx := context.Background()

	// 创建调度计划
	schedule := &RebuildSchedule{
		PoolName:     "tank",
		Disks:        []string{"disk-1", "disk-2"},
		Strategy:     "priority",
		MaxParallel:  3,
		ThrottleMBps: 150,
		Enabled:      true,
	}

	created, err := mgr.CreateSchedule(ctx, schedule)
	if err != nil {
		t.Fatalf("CreateSchedule failed: %v", err)
	}
	if created.ID == "" {
		t.Error("created schedule has empty ID")
	}
	if created.MaxParallel != 3 {
		t.Errorf("expected MaxParallel=3, got %d", created.MaxParallel)
	}

	// 列出
	schedules := mgr.ListSchedules(ctx)
	if len(schedules) != 1 {
		t.Errorf("expected 1 schedule, got %d", len(schedules))
	}

	// 获取
	got, err := mgr.GetSchedule(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetSchedule failed: %v", err)
	}
	if got.PoolName != "tank" {
		t.Errorf("expected pool_name=tank, got %s", got.PoolName)
	}

	// 删除
	if err := mgr.DeleteSchedule(ctx, created.ID); err != nil {
		t.Fatalf("DeleteSchedule failed: %v", err)
	}

	// 验证已删除
	_, err = mgr.GetSchedule(ctx, created.ID)
	if err == nil {
		t.Error("expected error for deleted schedule")
	}
}

func TestProgressSnapshot(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)

	// 设置IO指标
	mgr.SetIOMetrics(IOMetrics{
		ReadIOPS:  500,
		WriteIOPS: 300,
		ReadMBps:  100,
		WriteMBps: 50,
		Ioutil:    0.45,
	})

	snapshot := mgr.GetProgressSnapshot()
	if snapshot.ActiveJobs != 0 {
		t.Errorf("expected 0 active jobs, got %d", snapshot.ActiveJobs)
	}
	if snapshot.IOUtil != 0.45 {
		t.Errorf("expected IOUtil=0.45, got %f", snapshot.IOUtil)
	}

	// 创建运行中的任务后再测试
	ctx := context.Background()
	targetDisk := DiskInfo{ID: "d1", Path: "/dev/sda", SizeBytes: 1024 * 1024 * 1024}
	sourceDisks := []DiskInfo{{ID: "d2", Path: "/dev/sdb", SizeBytes: 1024 * 1024 * 1024}}
	job, _ := mgr.CreateJob(ctx, "tank", targetDisk, sourceDisks)
	mgr.StartJob(ctx, job.ID)

	snapshot = mgr.GetProgressSnapshot()
	if snapshot.ActiveJobs != 1 {
		t.Errorf("expected 1 active job, got %d", snapshot.ActiveJobs)
	}
	if snapshot.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
}
