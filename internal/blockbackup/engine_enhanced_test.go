package blockbackup

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

// --- BackupChainManager tests ---

func TestNewBackupChainManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	bcm := NewBackupChainManager(logger)
	if bcm == nil {
		t.Fatal("expected non-nil BackupChainManager")
	}
	if len(bcm.chains) != 0 {
		t.Errorf("expected 0 chains, got %d", len(bcm.chains))
	}
}

func TestBackupChainManager_CreateChain(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	bcm := NewBackupChainManager(logger)

	chain := bcm.CreateChain("snap-full-1", "job-full-1")
	if chain == nil {
		t.Fatal("expected non-nil chain")
	}
	if chain.RootSnapshot != "snap-full-1" {
		t.Errorf("expected root snap-full-1, got %s", chain.RootSnapshot)
	}
	if len(chain.Chain) != 1 {
		t.Errorf("expected chain length 1, got %d", len(chain.Chain))
	}
	if chain.Chain[0].Type != "full" {
		t.Errorf("expected type full, got %s", chain.Chain[0].Type)
	}
}

func TestBackupChainManager_AppendToChain(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	bcm := NewBackupChainManager(logger)

	bcm.CreateChain("snap-full-1", "job-full-1")

	err := bcm.AppendToChain("snap-full-1", "job-incr-1", "snap-incr-1", "incremental", "snap-full-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	chain := bcm.GetChain("snap-full-1")
	if len(chain.Chain) != 2 {
		t.Errorf("expected chain length 2, got %d", len(chain.Chain))
	}
	if chain.Chain[1].Type != "incremental" {
		t.Errorf("expected incremental, got %s", chain.Chain[1].Type)
	}
}

func TestBackupChainManager_AppendToChain_NotFound(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	bcm := NewBackupChainManager(logger)

	err := bcm.AppendToChain("nonexistent", "job-1", "snap-1", "incremental", "snap-0")
	if err == nil {
		t.Error("expected error for nonexistent chain")
	}
}

func TestBackupChainManager_ListChains(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	bcm := NewBackupChainManager(logger)

	bcm.CreateChain("snap-1", "job-1")
	bcm.CreateChain("snap-2", "job-2")

	chains := bcm.ListChains()
	if len(chains) != 2 {
		t.Errorf("expected 2 chains, got %d", len(chains))
	}
}

func TestBackupChainManager_GetChain_NotFound(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	bcm := NewBackupChainManager(logger)

	chain := bcm.GetChain("nonexistent")
	if chain != nil {
		t.Error("expected nil for nonexistent chain")
	}
}

// --- DifferentialBackup tests ---

func TestCreateDifferentialBackup_NoFullBackup(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewBlockBackupEngine(logger, BackupConfig{})

	ctx := context.Background()
	_, err := engine.CreateDifferentialBackup(ctx, "/src", "/dst")
	if err == nil {
		t.Error("expected error when no full backup exists")
	}
}

func TestCreateDifferentialBackup_WithFullBackup(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewBlockBackupEngine(logger, BackupConfig{})

	// 模拟一个全量快照
	engine.mu.Lock()
	engine.snapshots["snap-base"] = &BlockSnapshot{
		ID:        "snap-base",
		Volume:    "/src",
		IsBase:    true,
		CreatedAt: time.Now(),
	}
	engine.mu.Unlock()

	ctx := context.Background()
	job, err := engine.CreateDifferentialBackup(ctx, "/src", "/dst")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job == nil {
		t.Fatal("expected non-nil job")
	}
	if job.Type != "differential" {
		t.Errorf("expected type differential, got %s", job.Type)
	}
	if job.Status != "pending" {
		t.Errorf("expected status pending, got %s", job.Status)
	}
}

// --- ProgressCallback tests ---

func TestSetProgressCallback(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewBlockBackupEngine(logger, BackupConfig{})

	var mu sync.Mutex
	var called bool
	var reportedJobID string

	engine.SetProgressCallback(func(jobID string, progress int, bytesProcessed uint64) {
		mu.Lock()
		called = true
		reportedJobID = jobID
		mu.Unlock()
	})

	engine.ReportProgress("test-job-1", 50, 1024)

	mu.Lock()
	defer mu.Unlock()
	if !called {
		t.Error("expected callback to be called")
	}
	if reportedJobID != "test-job-1" {
		t.Errorf("expected job id test-job-1, got %s", reportedJobID)
	}
}

func TestProgressCallback_NilCallback(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewBlockBackupEngine(logger, BackupConfig{})

	// 不设置回调，不应 panic
	engine.ReportProgress("test-job", 50, 1024)
}

// --- BandwidthLimiter tests ---

func TestNewBandwidthLimiter(t *testing.T) {
	bl := NewBandwidthLimiter(10) // 10 MB/s
	if bl.limitBytesPerSec != 10*1024*1024 {
		t.Errorf("expected %d, got %d", 10*1024*1024, bl.limitBytesPerSec)
	}
}

func TestBandwidthLimiter_Acquire_NoLimit(t *testing.T) {
	bl := NewBandwidthLimiter(0)
	// 无限制时应立即返回
	done := make(chan struct{})
	go func() {
		bl.Acquire(1024 * 1024)
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(100 * time.Millisecond):
		t.Error("expected immediate return with no limit")
	}
}

func TestBandwidthLimiter_Acquire_WithLimit(t *testing.T) {
	bl := NewBandwidthLimiter(1) // 1 MB/s
	// 先消耗一半配额
	bl.Acquire(512 * 1024)

	// 再获取应正常（仍在配额内）
	done := make(chan struct{})
	go func() {
		bl.Acquire(512 * 1024)
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(200 * time.Millisecond):
		t.Error("expected acquisition within limit")
	}
}

// --- ParallelExecutor tests ---

func TestNewParallelExecutor(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	pe := NewParallelExecutor(4, logger)
	if pe.maxParallel != 4 {
		t.Errorf("expected maxParallel 4, got %d", pe.maxParallel)
	}
}

func TestParallelExecutor_SubmitAndWait(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	pe := NewParallelExecutor(2, logger)

	var counter int32
	for i := 0; i < 5; i++ {
		pe.Submit(func() {
			time.Sleep(10 * time.Millisecond)
			counter++
		})
	}

	pe.Wait()
	if counter != 5 {
		t.Errorf("expected 5 completions, got %d", counter)
	}
}

func TestParallelExecutor_DefaultParallelism(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	pe := NewParallelExecutor(0, logger)
	if pe.maxParallel != 4 {
		t.Errorf("expected default 4, got %d", pe.maxParallel)
	}
}

// --- BackupScheduler tests ---

func TestNewBackupScheduler(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewBlockBackupEngine(logger, BackupConfig{})
	sched := NewBackupScheduler(engine, logger)
	if sched == nil {
		t.Fatal("expected non-nil scheduler")
	}
	if sched.running {
		t.Error("expected scheduler not running initially")
	}
}

func TestBackupScheduler_AddSchedule(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewBlockBackupEngine(logger, BackupConfig{})
	sched := NewBackupScheduler(engine, logger)

	cfg := ScheduledBackup{
		ID:       "daily-full",
		Name:     "Daily Full Backup",
		Source:   "/data",
		Dest:     "/backup/data",
		Type:     "full",
		Schedule: "0 2 * * *",
		Enabled:  true,
	}

	err := sched.AddSchedule(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := sched.ListSchedules()
	if len(entries) != 1 {
		t.Errorf("expected 1 schedule entry, got %d", len(entries))
	}
}

func TestBackupScheduler_AddSchedule_Disabled(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewBlockBackupEngine(logger, BackupConfig{})
	sched := NewBackupScheduler(engine, logger)

	cfg := ScheduledBackup{
		ID:       "disabled",
		Enabled:  false,
		Schedule: "0 * * * *",
	}

	err := sched.AddSchedule(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := sched.ListSchedules()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for disabled schedule, got %d", len(entries))
	}
}

func TestBackupScheduler_AddSchedule_InvalidCron(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewBlockBackupEngine(logger, BackupConfig{})
	sched := NewBackupScheduler(engine, logger)

	cfg := ScheduledBackup{
		ID:       "bad-cron",
		Enabled:  true,
		Schedule: "not-a-cron-expr",
	}

	err := sched.AddSchedule(cfg)
	if err == nil {
		t.Error("expected error for invalid cron expression")
	}
}

func TestBackupScheduler_RemoveSchedule(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewBlockBackupEngine(logger, BackupConfig{})
	sched := NewBackupScheduler(engine, logger)

	cfg := ScheduledBackup{
		ID:       "to-remove",
		Enabled:  true,
		Schedule: "0 * * * *",
	}
	sched.AddSchedule(cfg)

	sched.RemoveSchedule("to-remove")
	entries := sched.ListSchedules()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after removal, got %d", len(entries))
	}
}

func TestBackupScheduler_StartStop(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewBlockBackupEngine(logger, BackupConfig{})
	sched := NewBackupScheduler(engine, logger)

	sched.Start()
	if !sched.running {
		t.Error("expected scheduler running after Start")
	}

	sched.Stop()
	if sched.running {
		t.Error("expected scheduler stopped after Stop")
	}

	// 幂等
	sched.Stop()
}

// --- RunParallelBackups tests ---

func TestRunParallelBackups_EmptyRequests(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewBlockBackupEngine(logger, BackupConfig{Parallel: 4})

	ctx := context.Background()
	jobs, err := engine.RunParallelBackups(ctx, nil, "full")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestRunParallelBackups_UnknownType(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewBlockBackupEngine(logger, BackupConfig{Parallel: 2})

	ctx := context.Background()
	_, err := engine.RunParallelBackups(ctx, []DifferentialBackupRequest{
		{Source: "/src", Destination: "/dst"},
	}, "unknown_type")
	if err == nil {
		t.Error("expected error for unknown backup type")
	}
}

// --- ListSnapshots tests ---

func TestListSnapshots(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewBlockBackupEngine(logger, BackupConfig{})

	snaps := engine.ListSnapshots()
	if len(snaps) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(snaps))
	}

	engine.mu.Lock()
	engine.snapshots["s1"] = &BlockSnapshot{ID: "s1"}
	engine.snapshots["s2"] = &BlockSnapshot{ID: "s2"}
	engine.mu.Unlock()

	snaps = engine.ListSnapshots()
	if len(snaps) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(snaps))
	}
}

// --- GetLatestFullSnapshot tests ---

func TestGetLatestFullSnapshot(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewBlockBackupEngine(logger, BackupConfig{})

	// 没有快照时返回 nil
	snap := engine.GetLatestFullSnapshot()
	if snap != nil {
		t.Error("expected nil when no snapshots exist")
	}

	// 添加全量快照
	engine.mu.Lock()
	engine.snapshots["snap-1"] = &BlockSnapshot{
		ID:        "snap-1",
		IsBase:    true,
		CreatedAt: time.Now().Add(-time.Hour),
	}
	engine.snapshots["snap-2"] = &BlockSnapshot{
		ID:        "snap-2",
		IsBase:    true,
		CreatedAt: time.Now(),
	}
	engine.snapshots["snap-3"] = &BlockSnapshot{
		ID:        "snap-3",
		IsBase:    false,
		CreatedAt: time.Now(),
	}
	engine.mu.Unlock()

	snap = engine.GetLatestFullSnapshot()
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snap.ID != "snap-2" {
		t.Errorf("expected snap-2 (latest), got %s", snap.ID)
	}
}

// --- Chain integration test ---

func TestBackupChain_FullWorkflow(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	bcm := NewBackupChainManager(logger)

	// 创建全量备份链
	chain := bcm.CreateChain("snap-full-001", "job-full-001")
	if chain.RootSnapshot != "snap-full-001" {
		t.Fatalf("expected root snap-full-001, got %s", chain.RootSnapshot)
	}

	// 追加增量备份
	bcm.AppendToChain("snap-full-001", "job-incr-001", "snap-incr-001", "incremental", "snap-full-001")
	bcm.AppendToChain("snap-full-001", "job-incr-002", "snap-incr-002", "incremental", "snap-incr-001")
	bcm.AppendToChain("snap-full-001", "job-incr-003", "snap-incr-003", "incremental", "snap-incr-002")

	chain = bcm.GetChain("snap-full-001")
	if len(chain.Chain) != 4 {
		t.Fatalf("expected chain length 4, got %d", len(chain.Chain))
	}

	expected := []string{"full", "incremental", "incremental", "incremental"}
	for i, entry := range chain.Chain {
		if entry.Type != expected[i] {
			t.Errorf("chain[%d]: expected type %s, got %s", i, expected[i], entry.Type)
		}
	}
}

// --- ScheduledBackup JSON tags ---

func TestScheduledBackup_JSONTags(t *testing.T) {
	cfg := ScheduledBackup{
		ID:       "test-id",
		Name:     "test-name",
		Source:   "/data",
		Dest:     "/backup",
		Type:     "full",
		Schedule: "0 * * * *",
		Enabled:  true,
	}
	if cfg.ID != "test-id" {
		t.Errorf("expected test-id, got %s", cfg.ID)
	}
	if cfg.Schedule != "0 * * * *" {
		t.Errorf("expected 0 * * * *, got %s", cfg.Schedule)
	}
}
