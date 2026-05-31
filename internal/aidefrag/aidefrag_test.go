package aidefrag

import (
	"testing"
	"time"
)

func newTestManager() *Manager {
	return NewManager(DefragConfig{
		ScanInterval:  time.Hour,
		FragThreshold: 10.0,
		MaxConcurrent: 1,
		IOLimitMBps:   50,
	})
}

func TestRegisterDisk(t *testing.T) {
	m := newTestManager()
	disk := &DiskInfo{
		ID:         "disk-1",
		Device:     "/dev/sda1",
		MountPoint: "/data",
		FileSystem: FsBtrfs,
		TotalBytes: 1024 * 1024 * 1024 * 500,
		UsedBytes:  1024 * 1024 * 1024 * 300,
		FreeBytes:  1024 * 1024 * 1024 * 200,
		FragPercent: 15.5,
	}
	if err := m.RegisterDisk(disk); err != nil {
		t.Fatalf("RegisterDisk failed: %v", err)
	}
	got, err := m.GetDisk("disk-1")
	if err != nil {
		t.Fatalf("GetDisk failed: %v", err)
	}
	if got.Device != "/dev/sda1" {
		t.Errorf("device = %q, want %q", got.Device, "/dev/sda1")
	}
}

func TestListDisks(t *testing.T) {
	m := newTestManager()
	_ = m.RegisterDisk(&DiskInfo{ID: "d1", Device: "/dev/sda", FileSystem: FsBtrfs})
	_ = m.RegisterDisk(&DiskInfo{ID: "d2", Device: "/dev/sdb", FileSystem: FsExt4})
	list := m.ListDisks()
	if len(list) != 2 {
		t.Errorf("ListDisks() = %d, want 2", len(list))
	}
}

func TestDefrag(t *testing.T) {
	m := newTestManager()
	disk := &DiskInfo{
		ID: "d1", Device: "/dev/sda1", MountPoint: "/data",
		FileSystem: FsBtrfs, FragPercent: 25, TotalBytes: 1 << 30,
	}
	_ = m.RegisterDisk(disk)
	job, err := m.StartDefrag("d1", "/data")
	if err != nil {
		t.Fatalf("StartDefrag failed: %v", err)
	}
	// 等待完成
	time.Sleep(3 * time.Second)
	got, _ := m.GetJob(job.ID)
	if got.State != StateCompleted {
		t.Errorf("state = %q, want %q", got.State, StateCompleted)
	}
	d, _ := m.GetDisk("d1")
	if d.FragPercent >= 25 {
		t.Errorf("frag percent should decrease after defrag")
	}
}

func TestStopDefrag(t *testing.T) {
	m := newTestManager()
	_ = m.RegisterDisk(&DiskInfo{ID: "d1", Device: "/dev/sda1", FileSystem: FsExt4, TotalBytes: 1 << 30})
	_, _ = m.StartDefrag("d1", "/")
	time.Sleep(100 * time.Millisecond)
	if err := m.StopDefrag(); err != nil {
		t.Fatalf("StopDefrag failed: %v", err)
	}
}

func TestAnalyzeFragments(t *testing.T) {
	m := newTestManager()
	_ = m.RegisterDisk(&DiskInfo{ID: "d1", Device: "/dev/sda1", FileSystem: FsBtrfs})
 frags, err := m.AnalyzeFragments("d1")
	if err != nil {
		t.Fatalf("AnalyzeFragments failed: %v", err)
	}
	if len(frags) == 0 {
		t.Error("expected fragment results")
	}
}

func TestPolicy(t *testing.T) {
	m := newTestManager()
	policy := &DefragPolicy{
		ID:            "p1",
		Name:          "低碎片策略",
		Schedule:      "0 3 * * 0",
		FragThreshold: 15.0,
		MaxDuration:   2 * time.Hour,
		PrioritizeHot: true,
	}
	if err := m.CreatePolicy(policy); err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}
	list := m.ListPolicies()
	if len(list) != 1 {
		t.Errorf("policies = %d, want 1", len(list))
	}
}

func TestStats(t *testing.T) {
	m := newTestManager()
	_ = m.RegisterDisk(&DiskInfo{ID: "d1", Device: "/dev/sda", FileSystem: FsBtrfs, FragPercent: 20})
	_ = m.RegisterDisk(&DiskInfo{ID: "d2", Device: "/dev/sdb", FileSystem: FsExt4, FragPercent: 5})
	stats := m.GetStats()
	if stats.TotalDisks != 2 {
		t.Errorf("TotalDisks = %d, want 2", stats.TotalDisks)
	}
	if stats.NeedDefrag != 1 {
		t.Errorf("NeedDefrag = %d, want 1", stats.NeedDefrag)
	}
}

func TestStartStop(t *testing.T) {
	m := newTestManager()
	_ = m.Start()
	m.Stop()
}
