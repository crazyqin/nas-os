package vmrestore

import (
	"testing"
	"time"
)

func TestSnapshotCreate(t *testing.T) {
	m := NewManager()
	s := &VMSnapshot{
		VMID:   "vm-001",
		VMName: "test-vm",
		Name:   "pre-update",
		Type:   RestoreTypeFull,
		SizeBytes: 1024 * 1024 * 1024,
		Checksum: "sha256:abc",
	}
	if err := m.CreateSnapshot(s); err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	if s.ID == "" {
		t.Error("expected snapshot ID")
	}
	snaps := m.ListSnapshots("vm-001")
	if len(snaps) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(snaps))
	}
}

func TestSnapshotDelete(t *testing.T) {
	m := NewManager()
	s := &VMSnapshot{VMID: "vm-001", VMName: "vm", Name: "snap1", Type: RestoreTypeFull, SizeBytes: 100}
	m.CreateSnapshot(s)
	if err := m.DeleteSnapshot(s.ID); err != nil {
		t.Fatalf("DeleteSnapshot failed: %v", err)
	}
	snaps := m.ListSnapshots("vm-001")
	if len(snaps) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(snaps))
	}
}

func TestRestoreFull(t *testing.T) {
	m := NewManager()
	s := &VMSnapshot{
		VMID: "vm-001", VMName: "test-vm", Name: "snap-full",
		Type: RestoreTypeFull, SizeBytes: 1024 * 1024 * 500,
	}
	m.CreateSnapshot(s)
	job := &RestoreJob{
		VMID:       "vm-001",
		SnapshotID: s.ID,
		TargetPath: "/mnt/pool1/vms/test-vm",
	}
	if err := m.StartRestore(job); err != nil {
		t.Fatalf("StartRestore failed: %v", err)
	}
	if job.Status != RestoreStatusRunning {
		t.Errorf("expected running, got %s", job.Status)
	}
	if job.TotalBytes != s.SizeBytes {
		t.Errorf("expected %d, got %d", s.SizeBytes, job.TotalBytes)
	}
	m.UpdateProgress(job.ID, 50.0, 1024*1024*250)
	if job.Progress != 50.0 {
		t.Errorf("expected 50%%, got %f", job.Progress)
	}
	m.UpdateProgress(job.ID, 100.0, s.SizeBytes)
	if job.Status != RestoreStatusCompleted {
		t.Errorf("expected completed, got %s", job.Status)
	}
}

func TestRestoreNotFound(t *testing.T) {
	m := NewManager()
	job := &RestoreJob{VMID: "vm-001", SnapshotID: "nonexistent"}
	if err := m.StartRestore(job); err == nil {
		t.Error("expected error for nonexistent snapshot")
	}
}

func TestRestoreCancel(t *testing.T) {
	m := NewManager()
	s := &VMSnapshot{VMID: "vm-001", VMName: "vm", Name: "snap", Type: RestoreTypeFull, SizeBytes: 100}
	m.CreateSnapshot(s)
	job := &RestoreJob{VMID: "vm-001", SnapshotID: s.ID}
	m.StartRestore(job)
	if err := m.CancelRestore(job.ID); err != nil {
		t.Fatalf("CancelRestore failed: %v", err)
	}
	if job.Status != RestoreStatusCancelled {
		t.Errorf("expected cancelled, got %s", job.Status)
	}
}

func TestRestoreFail(t *testing.T) {
	m := NewManager()
	s := &VMSnapshot{VMID: "vm-001", VMName: "vm", Name: "snap", Type: RestoreTypeFull, SizeBytes: 100}
	m.CreateSnapshot(s)
	job := &RestoreJob{VMID: "vm-001", SnapshotID: s.ID}
	m.StartRestore(job)
	m.FailRestore(job.ID, "disk full")
	if job.Status != RestoreStatusFailed {
		t.Errorf("expected failed, got %s", job.Status)
	}
	if job.ErrorMsg != "disk full" {
		t.Errorf("expected error msg, got %s", job.ErrorMsg)
	}
}

func TestVerifySnapshot(t *testing.T) {
	m := NewManager()
	s := &VMSnapshot{VMID: "vm-001", VMName: "vm", Name: "snap", Type: RestoreTypeFull, SizeBytes: 1024}
	m.CreateSnapshot(s)
	result, err := m.VerifySnapshot(s.ID)
	if err != nil {
		t.Fatalf("VerifySnapshot failed: %v", err)
	}
	if !result.Valid {
		t.Error("expected valid snapshot")
	}
	if !s.Verified {
		t.Error("expected snapshot to be marked verified")
	}
}

func TestVerifyZeroSize(t *testing.T) {
	m := NewManager()
	s := &VMSnapshot{VMID: "vm-001", VMName: "vm", Name: "empty", Type: RestoreTypeFull, SizeBytes: 0}
	m.CreateSnapshot(s)
	result, _ := m.VerifySnapshot(s.ID)
	if result.Valid {
		t.Error("expected invalid for zero size")
	}
}

func TestListRestorePoints(t *testing.T) {
	m := NewManager()
	s1 := &VMSnapshot{VMID: "vm-001", VMName: "vm", Name: "snap1", Type: RestoreTypeFull, SizeBytes: 100}
	s2 := &VMSnapshot{VMID: "vm-001", VMName: "vm", Name: "snap2", Type: RestoreTypeDelta, SizeBytes: 50}
	m.CreateSnapshot(s1)
	m.CreateSnapshot(s2)
	m.VerifySnapshot(s1.ID)
	m.VerifySnapshot(s2.ID)
	points := m.ListRestorePoints("vm-001")
	if len(points) != 2 {
		t.Errorf("expected 2 restore points, got %d", len(points))
	}
	// unverified should not show
	s3 := &VMSnapshot{VMID: "vm-001", VMName: "vm", Name: "unverified", Type: RestoreTypeFull, SizeBytes: 100}
	m.CreateSnapshot(s3)
	points = m.ListRestorePoints("vm-001")
	if len(points) != 2 {
		t.Errorf("expected 2 verified points, got %d", len(points))
	}
}

func TestRestoreEstimate(t *testing.T) {
	m := NewManager()
	s := &VMSnapshot{VMID: "vm-001", VMName: "vm", Name: "snap", Type: RestoreTypeFull, SizeBytes: 100 * 1024 * 1024}
	m.CreateSnapshot(s)
	dur, err := m.GetRestoreEstimate(s.ID, 50.0) // 50 MB/s
	if err != nil {
		t.Fatalf("GetRestoreEstimate failed: %v", err)
	}
	if dur <= 0 {
		t.Error("expected positive duration")
	}
	expected := time.Duration(float64(100*1024*1024)/(50.0*1024*1024)) * time.Second
	if dur != expected {
		t.Errorf("expected %v, got %v", expected, dur)
	}
}

func TestListJobs(t *testing.T) {
	m := NewManager()
	s := &VMSnapshot{VMID: "vm-001", VMName: "vm", Name: "snap", Type: RestoreTypeFull, SizeBytes: 100}
	m.CreateSnapshot(s)
	m.StartRestore(&RestoreJob{VMID: "vm-001", SnapshotID: s.ID})
	m.StartRestore(&RestoreJob{VMID: "vm-001", SnapshotID: s.ID})
	jobs := m.ListJobs()
	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(jobs))
	}
}