package cron

import (
	"testing"
	"time"
)

func TestAddAndGetJob(t *testing.T) {
	mgr := NewManager()
	job := &CronJob{
		ID:       "j1",
		Name:     "backup",
		Command:  "/usr/bin/backup.sh",
		Schedule: Schedule{Type: ScheduleDaily, Time: "02:00"},
	}
	mgr.AddJob(job)
	got, ok := mgr.GetJob("j1")
	if !ok {
		t.Fatal("expected job to exist")
	}
	if got.Name != "backup" {
		t.Errorf("expected backup, got %s", got.Name)
	}
	if got.Status != StatusEnabled {
		t.Errorf("expected enabled, got %s", got.Status)
	}
}

func TestListJobs(t *testing.T) {
	mgr := NewManager()
	mgr.AddJob(&CronJob{ID: "j1", Name: "a", Command: "echo a", Status: StatusEnabled})
	mgr.AddJob(&CronJob{ID: "j2", Name: "b", Command: "echo b", Status: StatusDisabled})
	all := mgr.ListJobs(false)
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}
	enabled := mgr.ListJobs(true)
	if len(enabled) != 1 {
		t.Errorf("expected 1 enabled, got %d", len(enabled))
	}
}

func TestUpdateJob(t *testing.T) {
	mgr := NewManager()
	mgr.AddJob(&CronJob{ID: "j1", Name: "old", Command: "echo old"})
	updated := &CronJob{Name: "new", Command: "echo new", Schedule: Schedule{Type: ScheduleWeekly, DayOfWeek: 1, Time: "09:00"}}
	if !mgr.UpdateJob("j1", updated) {
		t.Error("expected update to succeed")
	}
	got, _ := mgr.GetJob("j1")
	if got.Name != "new" {
		t.Errorf("expected new, got %s", got.Name)
	}
}

func TestDeleteJob(t *testing.T) {
	mgr := NewManager()
	mgr.AddJob(&CronJob{ID: "j1", Name: "a", Command: "echo a"})
	if !mgr.DeleteJob("j1") {
		t.Error("expected delete to succeed")
	}
	if _, ok := mgr.GetJob("j1"); ok {
		t.Error("expected job to be deleted")
	}
}

func TestDeleteJobNotFound(t *testing.T) {
	mgr := NewManager()
	if mgr.DeleteJob("nonexistent") {
		t.Error("expected false")
	}
}

func TestEnableDisableJob(t *testing.T) {
	mgr := NewManager()
	mgr.AddJob(&CronJob{ID: "j1", Name: "a", Command: "echo a", Status: StatusEnabled})
	mgr.DisableJob("j1")
	got, _ := mgr.GetJob("j1")
	if got.Status != StatusDisabled {
		t.Errorf("expected disabled, got %s", got.Status)
	}
	mgr.EnableJob("j1")
	got, _ = mgr.GetJob("j1")
	if got.Status != StatusEnabled {
		t.Errorf("expected enabled, got %s", got.Status)
	}
}

func TestRunNow(t *testing.T) {
	mgr := NewManager()
	mgr.AddJob(&CronJob{ID: "j1", Name: "test", Command: "echo hello"})
	run, err := mgr.RunNow("j1")
	if err != nil {
		t.Fatalf("RunNow: %v", err)
	}
	if !run.Success {
		t.Error("expected success")
	}
	if run.JobID != "j1" {
		t.Errorf("expected j1, got %s", run.JobID)
	}
	job, _ := mgr.GetJob("j1")
	if job.RunCount != 1 {
		t.Errorf("expected run_count=1, got %d", job.RunCount)
	}
}

func TestRunNowMaxConcurrent(t *testing.T) {
	mgr := NewManager()
	mgr.config.MaxConcurrent = 0
	mgr.AddJob(&CronJob{ID: "j1", Name: "a", Command: "echo a"})
	_, err := mgr.RunNow("j1")
	if err != ErrMaxConcurrent {
		t.Errorf("expected ErrMaxConcurrent, got %v", err)
	}
}

func TestRunNowJobNotFound(t *testing.T) {
	mgr := NewManager()
	_, err := mgr.RunNow("nonexistent")
	if err != ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestHistory(t *testing.T) {
	mgr := NewManager()
	mgr.AddJob(&CronJob{ID: "j1", Name: "a", Command: "echo a"})
	mgr.RunNow("j1")
	mgr.RunNow("j1")
	history := mgr.GetHistory("j1", 10)
	if len(history) != 2 {
		t.Errorf("expected 2 runs, got %d", len(history))
	}
}

func TestHistoryAll(t *testing.T) {
	mgr := NewManager()
	mgr.AddJob(&CronJob{ID: "j1", Name: "a", Command: "echo a"})
	mgr.AddJob(&CronJob{ID: "j2", Name: "b", Command: "echo b"})
	mgr.RunNow("j1")
	mgr.RunNow("j2")
	history := mgr.GetHistory("", 10)
	if len(history) != 2 {
		t.Errorf("expected 2 runs, got %d", len(history))
	}
}

func TestConfig(t *testing.T) {
	mgr := NewManager()
	cfg := mgr.GetConfig()
	if !cfg.Enabled {
		t.Error("expected enabled")
	}
	cfg.MaxConcurrent = 10
	mgr.UpdateConfig(cfg)
	if mgr.GetConfig().MaxConcurrent != 10 {
		t.Errorf("expected 10, got %d", mgr.GetConfig().MaxConcurrent)
	}
}

func TestStats(t *testing.T) {
	mgr := NewManager()
	mgr.AddJob(&CronJob{ID: "j1", Name: "a", Command: "echo a", Status: StatusEnabled})
	mgr.AddJob(&CronJob{ID: "j2", Name: "b", Command: "echo b", Status: StatusDisabled})
	stats := mgr.GetStats()
	if stats["total_jobs"] != 2 {
		t.Errorf("expected 2, got %v", stats["total_jobs"])
	}
	if stats["enabled_jobs"] != 1 {
		t.Errorf("expected 1, got %v", stats["enabled_jobs"])
	}
}

func TestJobTimestamps(t *testing.T) {
	mgr := NewManager()
	before := time.Now()
	mgr.AddJob(&CronJob{ID: "j1", Name: "a", Command: "echo a"})
	job, _ := mgr.GetJob("j1")
	if job.CreatedAt.Before(before) {
		t.Error("CreatedAt should be set")
	}
}
