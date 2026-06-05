package containermigrator

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.config == nil {
		t.Fatal("expected default config")
	}
	if m.config.MaxDowntimeMs != 5000 {
		t.Fatalf("expected 5000ms max downtime, got %d", m.config.MaxDowntimeMs)
	}
}

func TestRegisterHost(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	host := &Host{
		Name:    "node-1",
		Address: "192.168.1.10",
		OS:      "linux",
	}
	if err := m.RegisterHost(host); err != nil {
		t.Fatalf("RegisterHost failed: %v", err)
	}
	if host.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
	if host.Status != "online" {
		t.Fatalf("expected online, got %s", host.Status)
	}
}

func TestRegisterHostEmptyAddress(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	host := &Host{Name: "node-1", Address: ""}
	if err := m.RegisterHost(host); err == nil {
		t.Fatal("expected error for empty address")
	}
}

func TestListHosts(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	m.RegisterHost(&Host{Name: "node-1", Address: "192.168.1.10"})
	m.RegisterHost(&Host{Name: "node-2", Address: "192.168.1.11"})
	hosts := m.ListHosts()
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}
}

func TestGetHost(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	host := &Host{Name: "node-1", Address: "192.168.1.10"}
	m.RegisterHost(host)
	got, err := m.GetHost(host.ID)
	if err != nil {
		t.Fatalf("GetHost failed: %v", err)
	}
	if got.Name != "node-1" {
		t.Fatal("name mismatch")
	}
}

func TestGetHostNotFound(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	_, err := m.GetHost("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent host")
	}
}

func TestUnregisterHost(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	host := &Host{Name: "node-1", Address: "192.168.1.10"}
	m.RegisterHost(host)
	if err := m.UnregisterHost(host.ID); err != nil {
		t.Fatalf("UnregisterHost failed: %v", err)
	}
	if len(m.ListHosts()) != 0 {
		t.Fatal("expected 0 hosts")
	}
}

func TestUnregisterHostNotFound(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	if err := m.UnregisterHost("nonexistent"); err == nil {
		t.Fatal("expected error")
	}
}

func TestRegisterContainer(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	host := &Host{Name: "node-1", Address: "192.168.1.10"}
	m.RegisterHost(host)

	c := &Container{
		ID:     "ctr-1",
		Name:   "nginx",
		Image:  "nginx:latest",
		HostID: host.ID,
		State:  "running",
	}
	if err := m.RegisterContainer(c); err != nil {
		t.Fatalf("RegisterContainer failed: %v", err)
	}
	if len(m.ListContainers()) != 1 {
		t.Fatal("expected 1 container")
	}
}

func TestRegisterContainerEmptyID(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	c := &Container{ID: "", HostID: "h1"}
	if err := m.RegisterContainer(c); err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestRegisterContainerInvalidHost(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	c := &Container{ID: "ctr-1", HostID: "nonexistent"}
	if err := m.RegisterContainer(c); err == nil {
		t.Fatal("expected error for nonexistent host")
	}
}

func TestGetContainer(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	host := &Host{Name: "node-1", Address: "192.168.1.10"}
	m.RegisterHost(host)
	c := &Container{ID: "ctr-1", Name: "nginx", HostID: host.ID}
	m.RegisterContainer(c)

	got, err := m.GetContainer("ctr-1")
	if err != nil {
		t.Fatalf("GetContainer failed: %v", err)
	}
	if got.Name != "nginx" {
		t.Fatal("name mismatch")
	}
}

func TestGetContainerNotFound(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	_, err := m.GetContainer("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUnregisterContainer(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	host := &Host{Name: "node-1", Address: "192.168.1.10"}
	m.RegisterHost(host)
	m.RegisterContainer(&Container{ID: "ctr-1", HostID: host.ID})

	if err := m.UnregisterContainer("ctr-1"); err != nil {
		t.Fatalf("UnregisterContainer failed: %v", err)
	}
	if len(m.ListContainers()) != 0 {
		t.Fatal("expected 0 containers")
	}
}

func TestUnregisterContainerNotFound(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	if err := m.UnregisterContainer("nonexistent"); err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateSnapshot(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	host := &Host{Name: "node-1", Address: "192.168.1.10"}
	m.RegisterHost(host)
	m.RegisterContainer(&Container{ID: "ctr-1", HostID: host.ID})

	snap, err := m.CreateSnapshot("ctr-1")
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	if snap.ID == "" {
		t.Fatal("expected snapshot ID")
	}
	if snap.ContainerID != "ctr-1" {
		t.Fatal("container ID mismatch")
	}
	if snap.ExpiresAt == nil {
		t.Fatal("expected expiration time")
	}
}

func TestCreateSnapshotNotFound(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	_, err := m.CreateSnapshot("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListSnapshots(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	host := &Host{Name: "node-1", Address: "192.168.1.10"}
	m.RegisterHost(host)
	m.RegisterContainer(&Container{ID: "ctr-1", HostID: host.ID})
	m.RegisterContainer(&Container{ID: "ctr-2", HostID: host.ID})

	m.CreateSnapshot("ctr-1")
	m.CreateSnapshot("ctr-2")
	m.CreateSnapshot("ctr-1")

	all := m.ListSnapshots("")
	if len(all) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(all))
	}
	filtered := m.ListSnapshots("ctr-1")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 snapshots for ctr-1, got %d", len(filtered))
	}
}

func TestDeleteSnapshot(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	host := &Host{Name: "node-1", Address: "192.168.1.10"}
	m.RegisterHost(host)
	m.RegisterContainer(&Container{ID: "ctr-1", HostID: host.ID})
	snap, _ := m.CreateSnapshot("ctr-1")

	if err := m.DeleteSnapshot(snap.ID); err != nil {
		t.Fatalf("DeleteSnapshot failed: %v", err)
	}
	if len(m.ListSnapshots("")) != 0 {
		t.Fatal("expected 0 snapshots")
	}
}

func TestDeleteSnapshotNotFound(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	if err := m.DeleteSnapshot("nonexistent"); err == nil {
		t.Fatal("expected error")
	}
}

func TestStartMigration(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	src := &Host{Name: "node-1", Address: "192.168.1.10"}
	dst := &Host{Name: "node-2", Address: "192.168.1.11"}
	m.RegisterHost(src)
	m.RegisterHost(dst)
	m.RegisterContainer(&Container{ID: "ctr-1", Name: "nginx", HostID: src.ID, State: "running"})

	task, err := m.StartMigration("ctr-1", dst.ID)
	if err != nil {
		t.Fatalf("StartMigration failed: %v", err)
	}
	if task.Status != "pending" {
		t.Fatalf("expected pending, got %s", task.Status)
	}
	if task.SourceHostID != src.ID {
		t.Fatal("source host mismatch")
	}
	if task.TargetHostID != dst.ID {
		t.Fatal("target host mismatch")
	}
	if task.SnapshotID == "" {
		t.Fatal("expected snapshot ID")
	}
	if task.RollbackPoint == "" {
		t.Fatal("expected rollback point")
	}
}

func TestStartMigrationContainerNotFound(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	host := &Host{Name: "node-1", Address: "192.168.1.10"}
	m.RegisterHost(host)
	_, err := m.StartMigration("nonexistent", host.ID)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestStartMigrationTargetNotFound(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	host := &Host{Name: "node-1", Address: "192.168.1.10"}
	m.RegisterHost(host)
	m.RegisterContainer(&Container{ID: "ctr-1", HostID: host.ID})
	_, err := m.StartMigration("ctr-1", "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestStartMigrationSameHost(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	host := &Host{Name: "node-1", Address: "192.168.1.10"}
	m.RegisterHost(host)
	m.RegisterContainer(&Container{ID: "ctr-1", HostID: host.ID})
	_, err := m.StartMigration("ctr-1", host.ID)
	if err == nil {
		t.Fatal("expected error for same host migration")
	}
}

func TestStartMigrationDuplicate(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	src := &Host{Name: "node-1", Address: "192.168.1.10"}
	dst := &Host{Name: "node-2", Address: "192.168.1.11"}
	m.RegisterHost(src)
	m.RegisterHost(dst)
	m.RegisterContainer(&Container{ID: "ctr-1", HostID: src.ID})

	m.StartMigration("ctr-1", dst.ID)
	_, err := m.StartMigration("ctr-1", dst.ID)
	if err == nil {
		t.Fatal("expected error for duplicate migration")
	}
}

func TestUpdateMigrationProgress(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	src := &Host{Name: "node-1", Address: "192.168.1.10"}
	dst := &Host{Name: "node-2", Address: "192.168.1.11"}
	m.RegisterHost(src)
	m.RegisterHost(dst)
	m.RegisterContainer(&Container{ID: "ctr-1", Name: "nginx", HostID: src.ID, State: "running"})

	task, _ := m.StartMigration("ctr-1", dst.ID)

	// Update to 50%
	if err := m.UpdateMigrationProgress(task.ID, 50, "sync", 1024*1024*50); err != nil {
		t.Fatalf("UpdateMigrationProgress failed: %v", err)
	}
	got, _ := m.GetMigration(task.ID)
	if got.Progress != 50 {
		t.Fatalf("expected 50%%, got %f", got.Progress)
	}
	if got.Phase != "sync" {
		t.Fatalf("expected sync phase, got %s", got.Phase)
	}

	// Update to 100%
	if err := m.UpdateMigrationProgress(task.ID, 100, "done", 1024*1024*100); err != nil {
		t.Fatalf("UpdateMigrationProgress failed: %v", err)
	}
	got, _ = m.GetMigration(task.ID)
	if got.Status != "completed" {
		t.Fatalf("expected completed, got %s", got.Status)
	}

	// Container should be on target host
	c, _ := m.GetContainer("ctr-1")
	if c.HostID != dst.ID {
		t.Fatalf("expected container on target host, got %s", c.HostID)
	}
}

func TestUpdateMigrationProgressNotFound(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	if err := m.UpdateMigrationProgress("nonexistent", 50, "sync", 0); err == nil {
		t.Fatal("expected error")
	}
}

func TestRollbackMigration(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	src := &Host{Name: "node-1", Address: "192.168.1.10"}
	dst := &Host{Name: "node-2", Address: "192.168.1.11"}
	m.RegisterHost(src)
	m.RegisterHost(dst)
	m.RegisterContainer(&Container{ID: "ctr-1", Name: "nginx", HostID: src.ID, State: "running"})

	task, _ := m.StartMigration("ctr-1", dst.ID)
	if err := m.RollbackMigration(task.ID); err != nil {
		t.Fatalf("RollbackMigration failed: %v", err)
	}

	got, _ := m.GetMigration(task.ID)
	if got.Status != "rolled_back" {
		t.Fatalf("expected rolled_back, got %s", got.Status)
	}

	c, _ := m.GetContainer("ctr-1")
	if c.HostID != src.ID {
		t.Fatalf("expected container back on source host, got %s", c.HostID)
	}
}

func TestRollbackCompletedMigration(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	src := &Host{Name: "node-1", Address: "192.168.1.10"}
	dst := &Host{Name: "node-2", Address: "192.168.1.11"}
	m.RegisterHost(src)
	m.RegisterHost(dst)
	m.RegisterContainer(&Container{ID: "ctr-1", HostID: src.ID})

	task, _ := m.StartMigration("ctr-1", dst.ID)
	m.UpdateMigrationProgress(task.ID, 100, "done", 0)

	if err := m.RollbackMigration(task.ID); err == nil {
		t.Fatal("expected error for completed migration rollback")
	}
}

func TestRollbackMigrationNotFound(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	if err := m.RollbackMigration("nonexistent"); err == nil {
		t.Fatal("expected error")
	}
}

func TestGetMigrationNotFound(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	_, err := m.GetMigration("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	src := &Host{Name: "node-1", Address: "192.168.1.10"}
	dst := &Host{Name: "node-2", Address: "192.168.1.11"}
	m.RegisterHost(src)
	m.RegisterHost(dst)
	m.RegisterContainer(&Container{ID: "ctr-1", HostID: src.ID})

	task, _ := m.StartMigration("ctr-1", dst.ID)
	m.UpdateMigrationProgress(task.ID, 100, "done", 1024*1024*100)

	stats := m.GetStats()
	if stats.TotalMigrations != 1 {
		t.Fatalf("expected 1 migration, got %d", stats.TotalMigrations)
	}
	if stats.Successful != 1 {
		t.Fatalf("expected 1 successful, got %d", stats.Successful)
	}
	if stats.TotalContainers != 1 {
		t.Fatalf("expected 1 container, got %d", stats.TotalContainers)
	}
	if stats.TotalHosts != 2 {
		t.Fatalf("expected 2 hosts, got %d", stats.TotalHosts)
	}
}

func TestConfig(t *testing.T) {
	m := NewManager("/tmp/test-migrator")
	cfg := m.GetConfig()
	if !cfg.AutoRollback {
		t.Fatal("expected auto rollback enabled")
	}

	cfg.MaxDowntimeMs = 3000
	m.UpdateConfig(cfg)
	got := m.GetConfig()
	if got.MaxDowntimeMs != 3000 {
		t.Fatalf("expected 3000, got %d", got.MaxDowntimeMs)
	}
}
