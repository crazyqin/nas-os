// Package wanrepl 测试
package wanrepl

import (
	"testing"
)

func newTestEngine() *ReplicationEngine {
	engine := NewReplicationEngine(DefaultReplConfig())
	engine.Start()
	return engine
}

func TestNewReplicationEngine(t *testing.T) {
	e := NewReplicationEngine(DefaultReplConfig())
	if e == nil {
		t.Fatal("NewReplicationEngine returned nil")
	}
	if e.IsRunning() {
		t.Fatal("engine should not be running initially")
	}
}

func TestStartStop(t *testing.T) {
	e := NewReplicationEngine(DefaultReplConfig())
	e.Start()
	if !e.IsRunning() {
		t.Fatal("expected running")
	}
	e.Stop()
	if e.IsRunning() {
		t.Fatal("expected stopped")
	}
}

func TestAddSite(t *testing.T) {
	e := newTestEngine()
	site := &RemoteSite{
		ID:       "site1",
		Name:     "Backup Site",
		Endpoint: "10.0.0.2:22",
	}
	if err := e.AddSite(site); err != nil {
		t.Fatalf("AddSite failed: %v", err)
	}
	if site.Status != SiteStatusUnknown {
		t.Fatalf("expected unknown status, got %s", site.Status)
	}
}

func TestAddSiteDuplicate(t *testing.T) {
	e := newTestEngine()
	e.AddSite(&RemoteSite{ID: "s1", Name: "test", Endpoint: "10.0.0.1:22"})
	if err := e.AddSite(&RemoteSite{ID: "s1", Name: "dup", Endpoint: "10.0.0.2:22"}); err != ErrSiteExists {
		t.Fatalf("expected ErrSiteExists, got %v", err)
	}
}

func TestRemoveSite(t *testing.T) {
	e := newTestEngine()
	e.AddSite(&RemoteSite{ID: "s1", Name: "test", Endpoint: "10.0.0.1:22"})
	if err := e.RemoveSite("s1"); err != nil {
		t.Fatalf("RemoveSite failed: %v", err)
	}
	if len(e.ListSites()) != 0 {
		t.Fatal("site not removed")
	}
}

func TestRemoveSiteNotFound(t *testing.T) {
	e := newTestEngine()
	if err := e.RemoveSite("nonexistent"); err != ErrSiteNotFound {
		t.Fatalf("expected ErrSiteNotFound, got %v", err)
	}
}

func TestUpdateSiteStatus(t *testing.T) {
	e := newTestEngine()
	e.AddSite(&RemoteSite{ID: "s1", Name: "test", Endpoint: "10.0.0.1:22"})
	if err := e.UpdateSiteStatus("s1", SiteStatusOnline, 15); err != nil {
		t.Fatalf("UpdateSiteStatus failed: %v", err)
	}
	site, _ := e.GetSite("s1")
	if site.Status != SiteStatusOnline {
		t.Fatalf("expected online, got %s", site.Status)
	}
	if site.Latency != 15 {
		t.Fatalf("expected 15ms latency, got %d", site.Latency)
	}
}

func TestCreateJob(t *testing.T) {
	e := newTestEngine()
	e.AddSite(&RemoteSite{ID: "s1", Name: "remote", Endpoint: "10.0.0.1:22"})

	job := &ReplicationJob{
		Source:       "/data/photos",
		Destination:  "/backup/photos",
		TargetSiteID: "s1",
	}
	if err := e.CreateJob(job); err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}
	if job.ID == "" {
		t.Fatal("job ID not set")
	}
	if job.Status != JobStatusPending {
		t.Fatalf("expected pending, got %s", job.Status)
	}
	if job.Compression != CompressionZstd {
		t.Fatalf("expected zstd, got %s", job.Compression)
	}
	if job.Encryption != EncryptionTLS {
		t.Fatalf("expected tls, got %s", job.Encryption)
	}
}

func TestCreateJobSiteNotFound(t *testing.T) {
	e := newTestEngine()
	job := &ReplicationJob{TargetSiteID: "nonexistent"}
	if err := e.CreateJob(job); err != ErrSiteNotFound {
		t.Fatalf("expected ErrSiteNotFound, got %v", err)
	}
}

func TestStartJob(t *testing.T) {
	e := newTestEngine()
	e.AddSite(&RemoteSite{ID: "s1", Name: "remote", Endpoint: "10.0.0.1:22"})
	job := &ReplicationJob{Source: "/data", Destination: "/backup", TargetSiteID: "s1"}
	e.CreateJob(job)

	if err := e.StartJob(job.ID); err != nil {
		t.Fatalf("StartJob failed: %v", err)
	}

	updated, _ := e.GetJob(job.ID)
	if updated.Status != JobStatusRunning {
		t.Fatalf("expected running, got %s", updated.Status)
	}
	if updated.StartedAt == nil {
		t.Fatal("StartedAt not set")
	}
}

func TestStartJobAlreadyRunning(t *testing.T) {
	e := newTestEngine()
	e.AddSite(&RemoteSite{ID: "s1", Name: "remote", Endpoint: "10.0.0.1:22"})
	job := &ReplicationJob{Source: "/data", Destination: "/backup", TargetSiteID: "s1"}
	e.CreateJob(job)
	e.StartJob(job.ID)

	if err := e.StartJob(job.ID); err != ErrJobRunning {
		t.Fatalf("expected ErrJobRunning, got %v", err)
	}
}

func TestPauseJob(t *testing.T) {
	e := newTestEngine()
	e.AddSite(&RemoteSite{ID: "s1", Name: "remote", Endpoint: "10.0.0.1:22"})
	job := &ReplicationJob{Source: "/data", Destination: "/backup", TargetSiteID: "s1"}
	e.CreateJob(job)
	e.StartJob(job.ID)

	if err := e.PauseJob(job.ID); err != nil {
		t.Fatalf("PauseJob failed: %v", err)
	}

	updated, _ := e.GetJob(job.ID)
	if updated.Status != JobStatusPaused {
		t.Fatalf("expected paused, got %s", updated.Status)
	}
}

func TestCompleteJob(t *testing.T) {
	e := newTestEngine()
	e.AddSite(&RemoteSite{ID: "s1", Name: "remote", Endpoint: "10.0.0.1:22"})
	job := &ReplicationJob{Source: "/data", Destination: "/backup", TargetSiteID: "s1"}
	e.CreateJob(job)
	e.StartJob(job.ID)

	if err := e.CompleteJob(job.ID); err != nil {
		t.Fatalf("CompleteJob failed: %v", err)
	}

	updated, _ := e.GetJob(job.ID)
	if updated.Status != JobStatusCompleted {
		t.Fatalf("expected completed, got %s", updated.Status)
	}
	if updated.CompletedAt == nil {
		t.Fatal("CompletedAt not set")
	}
}

func TestUpdateSyncProgress(t *testing.T) {
	e := newTestEngine()
	e.AddSite(&RemoteSite{ID: "s1", Name: "remote", Endpoint: "10.0.0.1:22"})
	job := &ReplicationJob{Source: "/data", Destination: "/backup", TargetSiteID: "s1"}
	e.CreateJob(job)

	if err := e.UpdateSyncProgress(job.ID, 0.5, 500*1024*1024, 100*1024*1024, "/data/file1.dat"); err != nil {
		t.Fatalf("UpdateSyncProgress failed: %v", err)
	}

	state, _ := e.GetSyncState(job.ID)
	if state.Progress != 0.5 {
		t.Fatalf("expected 0.5 progress, got %f", state.Progress)
	}
	if state.CurrentFile != "/data/file1.dat" {
		t.Fatalf("expected /data/file1.dat, got %s", state.CurrentFile)
	}
}

func TestReportConflict(t *testing.T) {
	e := newTestEngine()
	e.AddSite(&RemoteSite{ID: "s1", Name: "remote", Endpoint: "10.0.0.1:22"})
	job := &ReplicationJob{Source: "/data", Destination: "/backup", TargetSiteID: "s1"}
	e.CreateJob(job)

	conflict := ConflictRecord{
		Path:   "/data/doc.txt",
		LocalSize:  1024,
		RemoteSize: 2048,
	}
	if err := e.ReportConflict(job.ID, conflict); err != nil {
		t.Fatalf("ReportConflict failed: %v", err)
	}

	conflicts, _ := e.GetConflicts(job.ID, false)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].ID == "" {
		t.Fatal("conflict ID not set")
	}
}

func TestResolveConflict(t *testing.T) {
	e := newTestEngine()
	e.AddSite(&RemoteSite{ID: "s1", Name: "remote", Endpoint: "10.0.0.1:22"})
	job := &ReplicationJob{Source: "/data", Destination: "/backup", TargetSiteID: "s1"}
	e.CreateJob(job)
	e.ReportConflict(job.ID, ConflictRecord{Path: "/data/doc.txt"})

	conflicts, _ := e.GetConflicts(job.ID, false)
	if err := e.ResolveConflict(job.ID, conflicts[0].ID, ConflictNewest); err != nil {
		t.Fatalf("ResolveConflict failed: %v", err)
	}

	resolved, _ := e.GetConflicts(job.ID, true)
	if len(resolved) != 0 {
		t.Fatal("expected 0 unresolved conflicts")
	}
}

func TestListJobs(t *testing.T) {
	e := newTestEngine()
	e.AddSite(&RemoteSite{ID: "s1", Name: "remote", Endpoint: "10.0.0.1:22"})

	for i := 0; i < 3; i++ {
		e.CreateJob(&ReplicationJob{Source: "/data", Destination: "/backup", TargetSiteID: "s1"})
	}

	if len(e.ListJobs()) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(e.ListJobs()))
	}
}

func TestListSites(t *testing.T) {
	e := newTestEngine()
	for i := 0; i < 3; i++ {
		e.AddSite(&RemoteSite{ID: "s" + string(rune('1'+i)), Name: "test", Endpoint: "10.0.0.1:22"})
	}
	if len(e.ListSites()) != 3 {
		t.Fatalf("expected 3 sites, got %d", len(e.ListSites()))
	}
}
