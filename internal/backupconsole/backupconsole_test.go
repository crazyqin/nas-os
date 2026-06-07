package backupconsole

import (
	"testing"
)

func TestBackupConsole_RegisterSource(t *testing.T) {
	bc := NewBackupConsole()

	src := &BackupSource{
		ID:       "src-001",
		Name:     "WebServer-01",
		Platform: PlatformLinux,
		IP:       "192.168.1.10",
		Hostname: "web01.local",
	}
	bc.RegisterSource(src)

	got, ok := bc.GetSource("src-001")
	if !ok {
		t.Fatal("expected source to exist")
	}
	if got.Name != "WebServer-01" {
		t.Errorf("expected WebServer-01, got %q", got.Name)
	}
}

func TestBackupConsole_CreateJob(t *testing.T) {
	bc := NewBackupConsole()

	bc.RegisterSource(&BackupSource{ID: "src-001", Name: "Server", Platform: PlatformWindows})

	job := &BackupJob{
		ID:         "job-001",
		Name:       "Daily Full Backup",
		SourceID:   "src-001",
		SourceType: PlatformWindows,
		TargetPool: "pool-backup",
		BackupType: BackupTypeFull,
		Schedule:   "0 2 * * *",
		Retention:  30,
		Enabled:    true,
	}

	if err := bc.CreateJob(job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := bc.GetJob("job-001")
	if !ok {
		t.Fatal("expected job to exist")
	}
	if got.Name != "Daily Full Backup" {
		t.Errorf("expected Daily Full Backup, got %q", got.Name)
	}
}

func TestBackupConsole_CreateJobNoSource(t *testing.T) {
	bc := NewBackupConsole()

	job := &BackupJob{ID: "job-001", SourceID: "nonexistent"}
	err := bc.CreateJob(job)
	if err == nil {
		t.Error("expected error for nonexistent source")
	}
}

func TestBackupConsole_RunAndCompleteBackup(t *testing.T) {
	bc := NewBackupConsole()

	bc.RegisterSource(&BackupSource{ID: "src-001", Name: "Server", Platform: PlatformLinux})
	bc.CreateJob(&BackupJob{
		ID:         "job-001",
		Name:       "Test Backup",
		SourceID:   "src-001",
		BackupType: BackupTypeFull,
		Retention:  7,
		Enabled:    true,
	})

	record, err := bc.RunBackup("job-001")
	if err != nil {
		t.Fatalf("run backup error: %v", err)
	}
	if record.Status != BackupStatusRunning {
		t.Errorf("expected running, got %q", record.Status)
	}

	bc.CompleteBackup(record.ID, 10*1024*1024*1024, 6*1024*1024*1024, 1.5, 1.3)

	records := bc.GetRecords("job-001", 0)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Status != BackupStatusDone {
		t.Errorf("expected done, got %q", records[0].Status)
	}
	if records[0].DedupRatio != 1.5 {
		t.Errorf("expected dedup 1.5, got %f", records[0].DedupRatio)
	}
}

func TestBackupConsole_FailBackup(t *testing.T) {
	bc := NewBackupConsole()

	bc.RegisterSource(&BackupSource{ID: "src-001", Name: "Server", Platform: PlatformLinux})
	bc.CreateJob(&BackupJob{ID: "job-001", SourceID: "src-001", Enabled: true})

	record, _ := bc.RunBackup("job-001")
	bc.FailBackup(record.ID, "connection timeout")

	records := bc.GetRecords("job-001", 0)
	if records[0].Status != BackupStatusFailed {
		t.Errorf("expected failed, got %q", records[0].Status)
	}
	if records[0].ErrorMessage != "connection timeout" {
		t.Errorf("expected error message, got %q", records[0].ErrorMessage)
	}
}

func TestBackupConsole_RestorePoints(t *testing.T) {
	bc := NewBackupConsole()

	bc.RegisterSource(&BackupSource{ID: "src-001", Name: "Server", Platform: PlatformLinux})
	bc.CreateJob(&BackupJob{ID: "job-001", SourceID: "src-001", BackupType: BackupTypeFull, Retention: 30, Enabled: true})

	bc.RunBackup("job-001")
	bc.RunBackup("job-001")

	points := bc.GetRestorePoints("job-001")
	if len(points) != 2 {
		t.Errorf("expected 2 restore points, got %d", len(points))
	}

	// 验证过期时间
	if points[0].RetentionExpire == nil {
		t.Error("expected retention expiry to be set")
	}
}

func TestBackupConsole_Dashboard(t *testing.T) {
	bc := NewBackupConsole()

	bc.RegisterSource(&BackupSource{ID: "s1", Platform: PlatformLinux, Protected: true})
	bc.RegisterSource(&BackupSource{ID: "s2", Platform: PlatformWindows, Protected: false})

	bc.CreateJob(&BackupJob{ID: "j1", SourceID: "s1", Enabled: true})
	bc.CreateJob(&BackupJob{ID: "j2", SourceID: "s2", Enabled: false})

	dash := bc.GetDashboard()
	if dash.TotalSources != 2 {
		t.Errorf("expected 2 sources, got %d", dash.TotalSources)
	}
	if dash.ProtectedHosts != 1 {
		t.Errorf("expected 1 protected, got %d", dash.ProtectedHosts)
	}
	if dash.ActiveJobs != 1 {
		t.Errorf("expected 1 active job, got %d", dash.ActiveJobs)
	}
}

func TestBackupConsole_ListSources(t *testing.T) {
	bc := NewBackupConsole()

	bc.RegisterSource(&BackupSource{ID: "s1", Platform: PlatformLinux})
	bc.RegisterSource(&BackupSource{ID: "s2", Platform: PlatformWindows})
	bc.RegisterSource(&BackupSource{ID: "s3", Platform: PlatformLinux})

	linux := bc.ListSources(PlatformLinux)
	if len(linux) != 2 {
		t.Errorf("expected 2 linux sources, got %d", len(linux))
	}

	all := bc.ListSources("")
	if len(all) != 3 {
		t.Errorf("expected 3 total sources, got %d", len(all))
	}
}

func TestBackupConsole_JobList(t *testing.T) {
	bc := NewBackupConsole()

	bc.RegisterSource(&BackupSource{ID: "s1", Platform: PlatformLinux})
	bc.CreateJob(&BackupJob{ID: "j1", SourceID: "s1", Enabled: true})
	bc.CreateJob(&BackupJob{ID: "j2", SourceID: "s1", Enabled: false})

	all := bc.ListJobs(false)
	if len(all) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(all))
	}

	enabled := bc.ListJobs(true)
	if len(enabled) != 1 {
		t.Errorf("expected 1 enabled, got %d", len(enabled))
	}
}
