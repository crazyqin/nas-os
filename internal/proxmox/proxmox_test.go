package proxmox

import (
	"context"
	"testing"
)

func TestRegisterCluster(t *testing.T) {
	pm := NewProxmoxManager()
	ctx := context.Background()

	cluster := &ProxmoxVE{
		ID:       "cluster-1",
		Name:     "Test Cluster",
		Endpoint: "https://pve1.local:8006",
	}

	err := pm.RegisterCluster(ctx, cluster)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cluster.Status != "online" {
		t.Errorf("expected status online, got %s", cluster.Status)
	}

	// Register same cluster should update
	cluster.Name = "Updated Cluster"
	err = pm.RegisterCluster(ctx, cluster)
	if err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}
}

func TestRegisterClusterNoID(t *testing.T) {
	pm := NewProxmoxManager()
	ctx := context.Background()

	cluster := &ProxmoxVE{
		Name: "No ID Cluster",
	}

	err := pm.RegisterCluster(ctx, cluster)
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestCreateBackup(t *testing.T) {
	pm := NewProxmoxManager()
	ctx := context.Background()

	// Register cluster first
	pm.RegisterCluster(ctx, &ProxmoxVE{
		ID:       "cluster-1",
		Name:     "Test Cluster",
		Endpoint: "https://pve1.local:8006",
	})

	// Create backup
	backup, err := pm.CreateBackup(ctx, "cluster-1", 100, "full")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if backup.VMID != 100 {
		t.Errorf("expected VMID 100, got %d", backup.VMID)
	}
	if backup.Type != "full" {
		t.Errorf("expected type full, got %s", backup.Type)
	}
	if backup.Status != "pending" {
		t.Errorf("expected status pending, got %s", backup.Status)
	}
}

func TestCreateBackupClusterNotFound(t *testing.T) {
	pm := NewProxmoxManager()
	ctx := context.Background()

	_, err := pm.CreateBackup(ctx, "nonexistent", 100, "full")
	if err == nil {
		t.Fatal("expected error for nonexistent cluster")
	}
}

func TestCreateBackupClusterOffline(t *testing.T) {
	pm := NewProxmoxManager()
	ctx := context.Background()

	pm.RegisterCluster(ctx, &ProxmoxVE{
		ID:     "cluster-1",
		Status: "offline",
	})

	// Manually set offline
	pm.mu.Lock()
	pm.clusters["cluster-1"].Status = "offline"
	pm.mu.Unlock()

	_, err := pm.CreateBackup(ctx, "cluster-1", 100, "full")
	if err == nil {
		t.Fatal("expected error for offline cluster")
	}
}

func TestRestoreBackup(t *testing.T) {
	pm := NewProxmoxManager()
	ctx := context.Background()

	// Setup
	pm.RegisterCluster(ctx, &ProxmoxVE{
		ID:       "cluster-1",
		Name:     "Test Cluster",
		Endpoint: "https://pve1.local:8006",
	})

	backup, _ := pm.CreateBackup(ctx, "cluster-1", 100, "full")

	// Manually set backup as completed
	pm.mu.Lock()
	backup.Status = "completed"
	pm.mu.Unlock()

	// Restore
	restore, err := pm.RestoreBackup(ctx, backup.ID, "node-2", 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if restore.TargetVMID != 200 {
		t.Errorf("expected target VMID 200, got %d", restore.TargetVMID)
	}
	if restore.Status != "pending" {
		t.Errorf("expected status pending, got %s", restore.Status)
	}
}

func TestRestoreBackupNotCompleted(t *testing.T) {
	pm := NewProxmoxManager()
	ctx := context.Background()

	pm.RegisterCluster(ctx, &ProxmoxVE{
		ID:       "cluster-1",
		Name:     "Test Cluster",
		Endpoint: "https://pve1.local:8006",
	})

	backup, _ := pm.CreateBackup(ctx, "cluster-1", 100, "full")
	// Backup is still pending

	_, err := pm.RestoreBackup(ctx, backup.ID, "node-2", 200)
	if err == nil {
		t.Fatal("expected error for non-completed backup")
	}
}

func TestCreateSchedule(t *testing.T) {
	pm := NewProxmoxManager()
	ctx := context.Background()

	schedule := &BackupSchedule{
		ID:         "sched-1",
		ClusterID:  "cluster-1",
		Name:       "Daily Backup",
		VMFilter:   "*",
		Schedule:   "0 2 * * *",
		Enabled:    true,
		BackupType: "incremental",
		Retention: RetentionPolicy{
			KeepDaily:   7,
			KeepWeekly:  4,
			KeepMonthly: 6,
		},
	}

	err := pm.CreateSchedule(ctx, schedule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pm.schedules["sched-1"] == nil {
		t.Fatal("schedule not stored")
	}
}

func TestListBackups(t *testing.T) {
	pm := NewProxmoxManager()
	ctx := context.Background()

	pm.RegisterCluster(ctx, &ProxmoxVE{
		ID:       "cluster-1",
		Name:     "Test Cluster",
		Endpoint: "https://pve1.local:8006",
	})

	// Create multiple backups
	pm.CreateBackup(ctx, "cluster-1", 100, "full")
	pm.CreateBackup(ctx, "cluster-1", 101, "incremental")
	pm.CreateBackup(ctx, "cluster-1", 102, "snapshot")

	backups := pm.ListBackups(ctx, "cluster-1")
	if len(backups) != 3 {
		t.Errorf("expected 3 backups, got %d", len(backups))
	}
}

func TestGetClusterStatus(t *testing.T) {
	pm := NewProxmoxManager()
	ctx := context.Background()

	pm.RegisterCluster(ctx, &ProxmoxVE{
		ID:       "cluster-1",
		Name:     "Test Cluster",
		Endpoint: "https://pve1.local:8006",
	})

	cluster, err := pm.GetClusterStatus(ctx, "cluster-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cluster.Name != "Test Cluster" {
		t.Errorf("expected name Test Cluster, got %s", cluster.Name)
	}
}

func TestGetClusterStatusNotFound(t *testing.T) {
	pm := NewProxmoxManager()
	ctx := context.Background()

	_, err := pm.GetClusterStatus(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent cluster")
	}
}
