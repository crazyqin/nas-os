package cloudbackup

import (
	"testing"
	"time"
)

func newTestManager() *Manager {
	return NewManager(BackupConfig{
		MaxConcurrent: 2,
		RetryCount:    2,
		RetryDelay:    time.Second,
	})
}

func TestAddAccount(t *testing.T) {
	m := newTestManager()
	acc := &CloudAccount{
		ID:       "acc-1",
		Name:     "公司M365",
		Provider: ProviderM365,
		TenantID: "tenant-123",
	}
	if err := m.AddAccount(acc); err != nil {
		t.Fatalf("AddAccount failed: %v", err)
	}
	got, err := m.GetAccount("acc-1")
	if err != nil {
		t.Fatalf("GetAccount failed: %v", err)
	}
	if got.Name != "公司M365" {
		t.Errorf("name = %q, want %q", got.Name, "公司M365")
	}
}

func TestListAccounts(t *testing.T) {
	m := newTestManager()
	_ = m.AddAccount(&CloudAccount{ID: "a1", Name: "M365", Provider: ProviderM365})
	_ = m.AddAccount(&CloudAccount{ID: "a2", Name: "GWS", Provider: ProviderGWS})
	_ = m.AddAccount(&CloudAccount{ID: "a3", Name: "M365-2", Provider: ProviderM365})

	m365 := m.ListAccounts(ProviderM365)
	if len(m365) != 2 {
		t.Errorf("M365 accounts = %d, want 2", len(m365))
	}
	all := m.ListAccounts("")
	if len(all) != 3 {
		t.Errorf("all accounts = %d, want 3", len(all))
	}
}

func TestRemoveAccount(t *testing.T) {
	m := newTestManager()
	_ = m.AddAccount(&CloudAccount{ID: "a1", Name: "test", Provider: ProviderM365})
	if err := m.RemoveAccount("a1"); err != nil {
		t.Fatalf("RemoveAccount failed: %v", err)
	}
	if _, err := m.GetAccount("a1"); err != ErrProviderNotFound {
		t.Errorf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestBackupJob(t *testing.T) {
	m := newTestManager()
	_ = m.AddAccount(&CloudAccount{ID: "acc-1", Name: "test", Provider: ProviderM365})
	job := &BackupJob{
		ID:         "job-1",
		AccountID:  "acc-1",
		Provider:   ProviderM365,
		Services:   []ServiceType{ServiceOneDrive, ServiceExchange},
		TotalItems: 100,
		TotalBytes: 1024 * 1024,
	}
	if err := m.CreateJob(job); err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}
	if err := m.RunJob("job-1"); err != nil {
		t.Fatalf("RunJob failed: %v", err)
	}
	// 等待完成
	time.Sleep(2 * time.Second)
	got, _ := m.GetJob("job-1")
	if got.Status != JobCompleted {
		t.Errorf("status = %q, want %q", got.Status, JobCompleted)
	}
}

func TestCancelJob(t *testing.T) {
	m := newTestManager()
	_ = m.AddAccount(&CloudAccount{ID: "acc-1", Name: "test", Provider: ProviderGWS})
	_ = m.CreateJob(&BackupJob{ID: "j1", AccountID: "acc-1", Provider: ProviderGWS, TotalItems: 100000, TotalBytes: 1 << 30})
	_ = m.RunJob("j1")
	time.Sleep(200 * time.Millisecond)
	if err := m.CancelJob("j1"); err != nil {
		t.Fatalf("CancelJob failed: %v", err)
	}
	got, _ := m.GetJob("j1")
	if got.Status != JobCancelled {
		t.Errorf("status = %q, want %q", got.Status, JobCancelled)
	}
}

func TestSchedule(t *testing.T) {
	m := newTestManager()
	sched := &BackupSchedule{
		ID:        "sched-1",
		AccountID: "acc-1",
		Services:  []ServiceType{ServiceOneDrive},
		CronExpr:  "0 2 * * *",
		Retention: 30,
	}
	if err := m.CreateSchedule(sched); err != nil {
		t.Fatalf("CreateSchedule failed: %v", err)
	}
	list := m.ListSchedules()
	if len(list) != 1 {
		t.Errorf("schedules = %d, want 1", len(list))
	}
}

func TestRestore(t *testing.T) {
	m := newTestManager()
	req := &RestoreRequest{
		ID:      "r1",
		JobID:   "job-1",
		Service: ServiceOneDrive,
		ItemID:  "file-123",
		Target:  "/restore/path",
	}
	if err := m.CreateRestore(req); err != nil {
		t.Fatalf("CreateRestore failed: %v", err)
	}
	got, err := m.GetRestore("r1")
	if err != nil {
		t.Fatalf("GetRestore failed: %v", err)
	}
	if got.Target != "/restore/path" {
		t.Errorf("target = %q, want %q", got.Target, "/restore/path")
	}
}

func TestStats(t *testing.T) {
	m := newTestManager()
	_ = m.AddAccount(&CloudAccount{ID: "a1", Name: "M365", Provider: ProviderM365})
	_ = m.CreateJob(&BackupJob{ID: "j1", AccountID: "a1", Provider: ProviderM365, Status: JobCompleted, BackedItems: 100})
	_ = m.CreateJob(&BackupJob{ID: "j2", AccountID: "a1", Provider: ProviderM365, Status: JobFailed})
	stats := m.GetStats()
	if stats.TotalAccounts != 1 {
		t.Errorf("TotalAccounts = %d, want 1", stats.TotalAccounts)
	}
	if stats.SuccessJobs != 1 {
		t.Errorf("SuccessJobs = %d, want 1", stats.SuccessJobs)
	}
	if stats.FailedJobs != 1 {
		t.Errorf("FailedJobs = %d, want 1", stats.FailedJobs)
	}
}

func TestStartStop(t *testing.T) {
	m := newTestManager()
	_ = m.Start()
	m.Stop()
}
