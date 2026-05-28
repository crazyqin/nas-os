package wanrepl

import (
	"testing"
	"time"
)

// ============================================================
// 测试配置
// ============================================================

func TestDefaultReplConfig(t *testing.T) {
	cfg := DefaultReplConfig()

	if cfg.DataDir != "/var/lib/nas-os/wanrepl" {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, "/var/lib/nas-os/wanrepl")
	}
	if cfg.MaxConcurrent != 4 {
		t.Errorf("MaxConcurrent = %d, want 4", cfg.MaxConcurrent)
	}
	if cfg.DefaultCompress != "zstd" {
		t.Errorf("DefaultCompress = %q, want %q", cfg.DefaultCompress, "zstd")
	}
	if cfg.TransferBufSize != 4*1024*1024 {
		t.Errorf("TransferBufSize = %d, want %d", cfg.TransferBufSize, 4*1024*1024)
	}
	if cfg.RetryAttempts != 3 {
		t.Errorf("RetryAttempts = %d, want 3", cfg.RetryAttempts)
	}
	if cfg.RetryDelay != 5*time.Second {
		t.Errorf("RetryDelay = %v, want 5s", cfg.RetryDelay)
	}
	if cfg.HealthCheckSec != 30 {
		t.Errorf("HealthCheckSec = %d, want 30", cfg.HealthCheckSec)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
}

// ============================================================
// 测试引擎初始化
// ============================================================

func TestNewReplicationEngine_NilConfig(t *testing.T) {
	engine := NewReplicationEngine(nil)
	if engine == nil {
		t.Fatal("NewReplicationEngine(nil) returned nil")
	}
	if engine.running {
		t.Error("engine should not be running initially")
	}
	if len(engine.sites) != 0 {
		t.Errorf("sites = %d, want 0", len(engine.sites))
	}
	if len(engine.jobs) != 0 {
		t.Errorf("jobs = %d, want 0", len(engine.jobs))
	}
}

func TestNewReplicationEngine_CustomConfig(t *testing.T) {
	cfg := &ReplConfig{
		DataDir:        "/custom/path",
		MaxConcurrent:  8,
		DefaultCompress: "lz4",
	}
	engine := NewReplicationEngine(cfg)
	if engine == nil {
		t.Fatal("NewReplicationEngine returned nil")
	}
	if engine.config.DataDir != "/custom/path" {
		t.Errorf("DataDir = %q, want %q", engine.config.DataDir, "/custom/path")
	}
	if engine.config.MaxConcurrent != 8 {
		t.Errorf("MaxConcurrent = %d, want 8", engine.config.MaxConcurrent)
	}
	if engine.config.DefaultCompress != "lz4" {
		t.Errorf("DefaultCompress = %q, want %q", engine.config.DefaultCompress, "lz4")
	}
}

// ============================================================
// 测试站点管理
// ============================================================

func TestAddSite_Success(t *testing.T) {
	engine := NewReplicationEngine(nil)
	site := &RemoteSite{
		ID:       "site-1",
		Name:     "Site 1",
		Endpoint: "192.168.1.100:22",
	}

	err := engine.AddSite(site)
	if err != nil {
		t.Fatalf("AddSite() error = %v", err)
	}
	if site.Status != SiteStatusUnknown {
		t.Errorf("Status = %q, want %q", site.Status, SiteStatusUnknown)
	}
	if site.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if site.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestAddSite_NilSite(t *testing.T) {
	engine := NewReplicationEngine(nil)
	err := engine.AddSite(nil)
	if err == nil {
		t.Error("AddSite(nil) should return error")
	}
}

func TestAddSite_EmptyID(t *testing.T) {
	engine := NewReplicationEngine(nil)
	site := &RemoteSite{Endpoint: "192.168.1.100:22"}
	err := engine.AddSite(site)
	if err == nil {
		t.Error("AddSite() with empty ID should return error")
	}
}

func TestAddSite_EmptyEndpoint(t *testing.T) {
	engine := NewReplicationEngine(nil)
	site := &RemoteSite{ID: "site-1"}
	err := engine.AddSite(site)
	if err == nil {
		t.Error("AddSite() with empty endpoint should return error")
	}
}

func TestAddSite_Duplicate(t *testing.T) {
	engine := NewReplicationEngine(nil)
	site := &RemoteSite{ID: "site-1", Endpoint: "192.168.1.100:22"}
	if err := engine.AddSite(site); err != nil {
		t.Fatalf("AddSite() error = %v", err)
	}

	site2 := &RemoteSite{ID: "site-1", Endpoint: "192.168.1.101:22"}
	err := engine.AddSite(site2)
	if err == nil {
		t.Error("AddSite() duplicate should return error")
	}
}

func TestRemoveSite_Success(t *testing.T) {
	engine := NewReplicationEngine(nil)
	site := &RemoteSite{ID: "site-1", Endpoint: "192.168.1.100:22"}
	engine.AddSite(site)

	err := engine.RemoveSite("site-1")
	if err != nil {
		t.Fatalf("RemoveSite() error = %v", err)
	}

	sites := engine.ListSites()
	if len(sites) != 0 {
		t.Errorf("sites count = %d, want 0", len(sites))
	}
}

func TestRemoveSite_NotFound(t *testing.T) {
	engine := NewReplicationEngine(nil)
	err := engine.RemoveSite("nonexistent")
	if err == nil {
		t.Error("RemoveSite() non-existing should return error")
	}
}

func TestListSites_Empty(t *testing.T) {
	engine := NewReplicationEngine(nil)
	sites := engine.ListSites()
	if sites == nil {
		t.Fatal("ListSites() returned nil")
	}
	if len(sites) != 0 {
		t.Errorf("sites count = %d, want 0", len(sites))
	}
}

func TestListSites_Multiple(t *testing.T) {
	engine := NewReplicationEngine(nil)
	engine.AddSite(&RemoteSite{ID: "s1", Endpoint: "host1:22"})
	engine.AddSite(&RemoteSite{ID: "s2", Endpoint: "host2:22"})

	sites := engine.ListSites()
	if len(sites) != 2 {
		t.Errorf("sites count = %d, want 2", len(sites))
	}
}

func TestGetSite_Success(t *testing.T) {
	engine := NewReplicationEngine(nil)
	engine.AddSite(&RemoteSite{ID: "site-1", Name: "Site 1", Endpoint: "host1:22"})

	site, err := engine.GetSite("site-1")
	if err != nil {
		t.Fatalf("GetSite() error = %v", err)
	}
	if site.ID != "site-1" {
		t.Errorf("site.ID = %q, want %q", site.ID, "site-1")
	}
	if site.Name != "Site 1" {
		t.Errorf("site.Name = %q, want %q", site.Name, "Site 1")
	}
}

func TestGetSite_NotFound(t *testing.T) {
	engine := NewReplicationEngine(nil)
	_, err := engine.GetSite("nonexistent")
	if err == nil {
		t.Error("GetSite() non-existing should return error")
	}
}

// ============================================================
// 测试任务管理
// ============================================================

func TestCreateJob_Success(t *testing.T) {
	engine := NewReplicationEngine(nil)
	engine.AddSite(&RemoteSite{ID: "site-1", Endpoint: "host1:22"})

	job := &ReplicationJob{
		ID:           "job-1",
		Source:       "/data/source",
		Destination:  "/data/dest",
		TargetSiteID: "site-1",
	}

	err := engine.CreateJob(job)
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	if job.Status != JobStatusPending {
		t.Errorf("Status = %q, want %q", job.Status, JobStatusPending)
	}
	if job.Strategy != StrategyIncremental {
		t.Errorf("Strategy = %q, want %q", job.Strategy, StrategyIncremental)
	}
	if job.Compression != CompressionZstd {
		t.Errorf("Compression = %q, want %q", job.Compression, CompressionZstd)
	}
	if job.Encryption != EncryptionTLS {
		t.Errorf("Encryption = %q, want %q", job.Encryption, EncryptionTLS)
	}
	if job.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestCreateJob_NilJob(t *testing.T) {
	engine := NewReplicationEngine(nil)
	err := engine.CreateJob(nil)
	if err == nil {
		t.Error("CreateJob(nil) should return error")
	}
}

func TestCreateJob_EmptyID(t *testing.T) {
	engine := NewReplicationEngine(nil)
	job := &ReplicationJob{Source: "/data", Destination: "/dest"}
	err := engine.CreateJob(job)
	if err == nil {
		t.Error("CreateJob() with empty ID should return error")
	}
}

func TestCreateJob_EmptySource(t *testing.T) {
	engine := NewReplicationEngine(nil)
	job := &ReplicationJob{ID: "job-1", Destination: "/dest"}
	err := engine.CreateJob(job)
	if err == nil {
		t.Error("CreateJob() with empty source should return error")
	}
}

func TestCreateJob_EmptyDestination(t *testing.T) {
	engine := NewReplicationEngine(nil)
	job := &ReplicationJob{ID: "job-1", Source: "/data"}
	err := engine.CreateJob(job)
	if err == nil {
		t.Error("CreateJob() with empty destination should return error")
	}
}

func TestCreateJob_Duplicate(t *testing.T) {
	engine := NewReplicationEngine(nil)
	job1 := &ReplicationJob{ID: "job-1", Source: "/data", Destination: "/dest"}
	engine.CreateJob(job1)

	job2 := &ReplicationJob{ID: "job-1", Source: "/data2", Destination: "/dest2"}
	err := engine.CreateJob(job2)
	if err == nil {
		t.Error("CreateJob() duplicate should return error")
	}
}

func TestCreateJob_NonExistingSite(t *testing.T) {
	engine := NewReplicationEngine(nil)
	job := &ReplicationJob{
		ID:           "job-1",
		Source:       "/data",
		Destination:  "/dest",
		TargetSiteID: "nonexistent",
	}
	err := engine.CreateJob(job)
	if err == nil {
		t.Error("CreateJob() with non-existing target site should return error")
	}
}

func TestCreateJob_NoTargetSite(t *testing.T) {
	engine := NewReplicationEngine(nil)
	job := &ReplicationJob{
		ID:          "job-1",
		Source:      "/data",
		Destination: "/dest",
	}
	err := engine.CreateJob(job)
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
}

func TestDeleteJob_Success(t *testing.T) {
	engine := NewReplicationEngine(nil)
	job := &ReplicationJob{ID: "job-1", Source: "/data", Destination: "/dest"}
	engine.CreateJob(job)

	err := engine.DeleteJob("job-1")
	if err != nil {
		t.Fatalf("DeleteJob() error = %v", err)
	}
}

func TestDeleteJob_NotFound(t *testing.T) {
	engine := NewReplicationEngine(nil)
	err := engine.DeleteJob("nonexistent")
	if err == nil {
		t.Error("DeleteJob() non-existing should return error")
	}
}

func TestDeleteJob_Running(t *testing.T) {
	engine := NewReplicationEngine(nil)
	job := &ReplicationJob{ID: "job-1", Source: "/data", Destination: "/dest"}
	engine.CreateJob(job)
	job.Status = JobStatusRunning

	err := engine.DeleteJob("job-1")
	if err == nil {
		t.Error("DeleteJob() running job should return error")
	}
}

func TestListJobs_Empty(t *testing.T) {
	engine := NewReplicationEngine(nil)
	jobs := engine.ListJobs()
	if jobs == nil {
		t.Fatal("ListJobs() returned nil")
	}
	if len(jobs) != 0 {
		t.Errorf("jobs count = %d, want 0", len(jobs))
	}
}

func TestListJobs_Multiple(t *testing.T) {
	engine := NewReplicationEngine(nil)
	engine.CreateJob(&ReplicationJob{ID: "j1", Source: "/a", Destination: "/b"})
	engine.CreateJob(&ReplicationJob{ID: "j2", Source: "/c", Destination: "/d"})

	jobs := engine.ListJobs()
	if len(jobs) != 2 {
		t.Errorf("jobs count = %d, want 2", len(jobs))
	}
}

func TestGetJob_Success(t *testing.T) {
	engine := NewReplicationEngine(nil)
	engine.CreateJob(&ReplicationJob{ID: "job-1", Source: "/data", Destination: "/dest"})

	job, err := engine.GetJob("job-1")
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if job.ID != "job-1" {
		t.Errorf("job.ID = %q, want %q", job.ID, "job-1")
	}
}

func TestGetJob_NotFound(t *testing.T) {
	engine := NewReplicationEngine(nil)
	_, err := engine.GetJob("nonexistent")
	if err == nil {
		t.Error("GetJob() non-existing should return error")
	}
}

func TestIsRunning_Default(t *testing.T) {
	engine := NewReplicationEngine(nil)
	if engine.IsRunning() {
		t.Error("IsRunning() should be false initially")
	}
}

func TestSetBandwidthLimit_Success(t *testing.T) {
	engine := NewReplicationEngine(nil)
	engine.CreateJob(&ReplicationJob{ID: "job-1", Source: "/data", Destination: "/dest"})

	err := engine.SetBandwidthLimit("job-1", 10*1024*1024)
	if err != nil {
		t.Fatalf("SetBandwidthLimit() error = %v", err)
	}

	job, _ := engine.GetJob("job-1")
	if job.BandwidthLimit != 10*1024*1024 {
		t.Errorf("BandwidthLimit = %d, want %d", job.BandwidthLimit, 10*1024*1024)
	}
}

func TestSetBandwidthLimit_NotFound(t *testing.T) {
	engine := NewReplicationEngine(nil)
	err := engine.SetBandwidthLimit("nonexistent", 1000)
	if err == nil {
		t.Error("SetBandwidthLimit() non-existing should return error")
	}
}

// ============================================================
// 测试同步状态管理
// ============================================================

func TestGetSyncState_Success(t *testing.T) {
	engine := NewReplicationEngine(nil)
	engine.CreateJob(&ReplicationJob{ID: "job-1", Source: "/data", Destination: "/dest"})

	state, err := engine.GetSyncState("job-1")
	if err != nil {
		t.Fatalf("GetSyncState() error = %v", err)
	}
	if state.JobID != "job-1" {
		t.Errorf("state.JobID = %q, want %q", state.JobID, "job-1")
	}
	if state.Progress != 0 {
		t.Errorf("Progress = %f, want 0", state.Progress)
	}
}

func TestGetSyncState_NotFound(t *testing.T) {
	engine := NewReplicationEngine(nil)
	_, err := engine.GetSyncState("nonexistent")
	if err == nil {
		t.Error("GetSyncState() non-existing should return error")
	}
}

func TestGetTransferStats_Success(t *testing.T) {
	engine := NewReplicationEngine(nil)
	engine.CreateJob(&ReplicationJob{ID: "job-1", Source: "/data", Destination: "/dest"})

	stats := engine.GetTransferStats("job-1")
	if stats == nil {
		t.Fatal("GetTransferStats() returned nil")
	}
	if stats.JobID != "job-1" {
		t.Errorf("stats.JobID = %q, want %q", stats.JobID, "job-1")
	}
}

func TestGetTransferStats_NotFound(t *testing.T) {
	engine := NewReplicationEngine(nil)
	stats := engine.GetTransferStats("nonexistent")
	if stats != nil {
		t.Error("GetTransferStats() non-existing should return nil")
	}
}

func TestGetConflicts_Empty(t *testing.T) {
	engine := NewReplicationEngine(nil)
	engine.CreateJob(&ReplicationJob{ID: "job-1", Source: "/data", Destination: "/dest"})

	conflicts := engine.GetConflicts("job-1")
	if conflicts == nil {
		t.Fatal("GetConflicts() returned nil")
	}
	if len(conflicts) != 0 {
		t.Errorf("conflicts count = %d, want 0", len(conflicts))
	}
}

func TestGetConflicts_NotFound(t *testing.T) {
	engine := NewReplicationEngine(nil)
	conflicts := engine.GetConflicts("nonexistent")
	if conflicts != nil {
		t.Error("GetConflicts() non-existing should return nil")
	}
}

// ============================================================
// 测试同步生命周期
// ============================================================

func TestStartSync_Success(t *testing.T) {
	engine := NewReplicationEngine(nil)
	engine.CreateJob(&ReplicationJob{ID: "job-1", Source: "/data", Destination: "/dest"})

	err := engine.StartSync("job-1")
	if err != nil {
		t.Fatalf("StartSync() error = %v", err)
	}

	job, _ := engine.GetJob("job-1")
	if job.Status != JobStatusRunning {
		t.Errorf("Status = %q, want %q", job.Status, JobStatusRunning)
	}
	if job.StartedAt == nil {
		t.Error("StartedAt should be set")
	}

	// 清理：停止任务
	engine.StopSync("job-1")
}

func TestStartSync_NotFound(t *testing.T) {
	engine := NewReplicationEngine(nil)
	err := engine.StartSync("nonexistent")
	if err == nil {
		t.Error("StartSync() non-existing should return error")
	}
}

func TestStartSync_AlreadyRunning(t *testing.T) {
	engine := NewReplicationEngine(nil)
	engine.CreateJob(&ReplicationJob{ID: "job-1", Source: "/data", Destination: "/dest"})
	engine.StartSync("job-1")

	err := engine.StartSync("job-1")
	if err == nil {
		t.Error("StartSync() already running should return error")
	}

	engine.StopSync("job-1")
}

func TestStopSync_Success(t *testing.T) {
	engine := NewReplicationEngine(nil)
	engine.CreateJob(&ReplicationJob{ID: "job-1", Source: "/data", Destination: "/dest"})
	engine.StartSync("job-1")

	// 等待一下让同步开始
	time.Sleep(100 * time.Millisecond)

	err := engine.StopSync("job-1")
	if err != nil {
		t.Fatalf("StopSync() error = %v", err)
	}

	job, _ := engine.GetJob("job-1")
	if job.Status != JobStatusCancelled {
		t.Errorf("Status = %q, want %q", job.Status, JobStatusCancelled)
	}
}

func TestStopSync_NotFound(t *testing.T) {
	engine := NewReplicationEngine(nil)
	err := engine.StopSync("nonexistent")
	if err == nil {
		t.Error("StopSync() non-existing should return error")
	}
}

func TestStopSync_NotRunning(t *testing.T) {
	engine := NewReplicationEngine(nil)
	engine.CreateJob(&ReplicationJob{ID: "job-1", Source: "/data", Destination: "/dest"})

	err := engine.StopSync("job-1")
	if err == nil {
		t.Error("StopSync() not running should return error")
	}
}

func TestPauseSync_Success(t *testing.T) {
	engine := NewReplicationEngine(nil)
	engine.CreateJob(&ReplicationJob{ID: "job-1", Source: "/data", Destination: "/dest"})
	engine.StartSync("job-1")

	time.Sleep(100 * time.Millisecond)

	err := engine.PauseSync("job-1")
	if err != nil {
		t.Fatalf("PauseSync() error = %v", err)
	}

	job, _ := engine.GetJob("job-1")
	if job.Status != JobStatusPaused {
		t.Errorf("Status = %q, want %q", job.Status, JobStatusPaused)
	}

	engine.StopSync("job-1")
}

func TestPauseSync_NotFound(t *testing.T) {
	engine := NewReplicationEngine(nil)
	err := engine.PauseSync("nonexistent")
	if err == nil {
		t.Error("PauseSync() non-existing should return error")
	}
}

func TestPauseSync_NotRunning(t *testing.T) {
	engine := NewReplicationEngine(nil)
	engine.CreateJob(&ReplicationJob{ID: "job-1", Source: "/data", Destination: "/dest"})

	err := engine.PauseSync("job-1")
	if err == nil {
		t.Error("PauseSync() not running should return error")
	}
}

func TestResumeSync_Success(t *testing.T) {
	engine := NewReplicationEngine(nil)
	engine.CreateJob(&ReplicationJob{ID: "job-1", Source: "/data", Destination: "/dest"})
	engine.StartSync("job-1")
	time.Sleep(100 * time.Millisecond)
	engine.PauseSync("job-1")

	err := engine.ResumeSync("job-1")
	if err != nil {
		t.Fatalf("ResumeSync() error = %v", err)
	}

	job, _ := engine.GetJob("job-1")
	if job.Status != JobStatusRunning {
		t.Errorf("Status = %q, want %q", job.Status, JobStatusRunning)
	}

	engine.StopSync("job-1")
}

func TestResumeSync_NotFound(t *testing.T) {
	engine := NewReplicationEngine(nil)
	err := engine.ResumeSync("nonexistent")
	if err == nil {
		t.Error("ResumeSync() non-existing should return error")
	}
}

func TestResumeSync_NotPaused(t *testing.T) {
	engine := NewReplicationEngine(nil)
	engine.CreateJob(&ReplicationJob{ID: "job-1", Source: "/data", Destination: "/dest"})

	err := engine.ResumeSync("job-1")
	if err == nil {
		t.Error("ResumeSync() not paused should return error")
	}
}

// ============================================================
// 测试冲突解决
// ============================================================

func TestResolveConflict_NilConflict(t *testing.T) {
	engine := NewReplicationEngine(nil)
	err := engine.ResolveConflict(nil, "local")
	if err == nil {
		t.Error("ResolveConflict(nil) should return error")
	}
}

func TestResolveConflict_InvalidResolution(t *testing.T) {
	engine := NewReplicationEngine(nil)
	engine.CreateJob(&ReplicationJob{ID: "job-1", Source: "/data", Destination: "/dest"})

	conflict := &ConflictRecord{
		ID:    "conflict-1",
		JobID: "job-1",
	}
	err := engine.ResolveConflict(conflict, "invalid_strategy")
	if err == nil {
		t.Error("ResolveConflict() invalid resolution should return error")
	}
}

func TestResolveConflict_ConflictNotFound(t *testing.T) {
	engine := NewReplicationEngine(nil)
	engine.CreateJob(&ReplicationJob{ID: "job-1", Source: "/data", Destination: "/dest"})

	conflict := &ConflictRecord{
		ID:    "nonexistent",
		JobID: "job-1",
	}
	err := engine.ResolveConflict(conflict, "local")
	if err == nil {
		t.Error("ResolveConflict() non-existing conflict should return error")
	}
}

func TestResolveConflict_JobNotFound(t *testing.T) {
	engine := NewReplicationEngine(nil)
	conflict := &ConflictRecord{
		ID:    "conflict-1",
		JobID: "nonexistent",
	}
	err := engine.ResolveConflict(conflict, "local")
	if err == nil {
		t.Error("ResolveConflict() non-existing job should return error")
	}
}

// ============================================================
// 测试站点状态更新
// ============================================================

func TestUpdateSiteStatus(t *testing.T) {
	engine := NewReplicationEngine(nil)
	engine.AddSite(&RemoteSite{ID: "site-1", Endpoint: "host1:22"})

	engine.updateSiteStatus("site-1", SiteStatusOnline)

	site, _ := engine.GetSite("site-1")
	if site.Status != SiteStatusOnline {
		t.Errorf("Status = %q, want %q", site.Status, SiteStatusOnline)
	}
}

func TestUpdateSiteStatus_NotFound(t *testing.T) {
	engine := NewReplicationEngine(nil)
	// 不应 panic
	engine.updateSiteStatus("nonexistent", SiteStatusOnline)
}
