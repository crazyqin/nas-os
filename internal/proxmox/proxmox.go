package proxmox

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ProxmoxVE represents a Proxmox VE cluster connection.
type ProxmoxVE struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Endpoint  string    `json:"endpoint"`
	Token     string    `json:"token,omitempty"`
	Status    string    `json:"status"` // online, offline, degraded
	NodeCount int       `json:"node_count"`
	VMCount   int       `json:"vm_count"`
	LastSeen  time.Time `json:"last_seen"`
}

// VMBackup represents a VM backup job.
type VMBackup struct {
	ID          string            `json:"id"`
	ClusterID   string            `json:"cluster_id"`
	VMID        int               `json:"vm_id"`
	VMName      string            `json:"vm_name"`
	Node        string            `json:"node"`
	Type        string            `json:"type"`   // full, incremental, snapshot
	Status      string            `json:"status"` // pending, running, completed, failed
	Size        int64             `json:"size"`
	StartTime   time.Time         `json:"start_time"`
	EndTime     *time.Time        `json:"end_time,omitempty"`
	Duration    time.Duration     `json:"duration"`
	Storage     string            `json:"storage"`
	Compression string            `json:"compression"` // none, gzip, lzo, zstd
	Encrypted   bool              `json:"encrypted"`
	Retention   RetentionPolicy   `json:"retention"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// RetentionPolicy defines backup retention rules.
type RetentionPolicy struct {
	KeepDaily   int `json:"keep_daily"`
	KeepWeekly  int `json:"keep_weekly"`
	KeepMonthly int `json:"keep_monthly"`
	KeepYearly  int `json:"keep_yearly"`
	MaxAgeDays  int `json:"max_age_days"`
}

// RestoreJob represents a VM restore operation.
type RestoreJob struct {
	ID            string     `json:"id"`
	BackupID      string     `json:"backup_id"`
	TargetNode    string     `json:"target_node"`
	TargetVMID    int        `json:"target_vm_id"`
	Status        string     `json:"status"` // pending, running, completed, failed
	Progress      float64    `json:"progress"`
	StartTime     time.Time  `json:"start_time"`
	EndTime       *time.Time `json:"end_time,omitempty"`
	CrossPlatform bool       `json:"cross_platform"` // restore to different hypervisor
}

// BackupSchedule defines automated backup scheduling.
type BackupSchedule struct {
	ID         string          `json:"id"`
	ClusterID  string          `json:"cluster_id"`
	Name       string          `json:"name"`
	VMFilter   string          `json:"vm_filter"` // glob pattern or tag
	Schedule   string          `json:"schedule"`  // cron expression
	Enabled    bool            `json:"enabled"`
	BackupType string          `json:"backup_type"` // full, incremental
	Retention  RetentionPolicy `json:"retention"`
	LastRun    *time.Time      `json:"last_run,omitempty"`
	NextRun    *time.Time      `json:"next_run,omitempty"`
}

// ProxmoxManager manages Proxmox VE backups.
type ProxmoxManager struct {
	mu        sync.RWMutex
	clusters  map[string]*ProxmoxVE
	backups   map[string]*VMBackup
	restores  map[string]*RestoreJob
	schedules map[string]*BackupSchedule
}

// NewProxmoxManager creates a new Proxmox manager.
func NewProxmoxManager() *ProxmoxManager {
	return &ProxmoxManager{
		clusters:  make(map[string]*ProxmoxVE),
		backups:   make(map[string]*VMBackup),
		restores:  make(map[string]*RestoreJob),
		schedules: make(map[string]*BackupSchedule),
	}
}

// RegisterCluster registers a Proxmox VE cluster.
func (pm *ProxmoxManager) RegisterCluster(ctx context.Context, cluster *ProxmoxVE) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if cluster.ID == "" {
		return fmt.Errorf("cluster ID is required")
	}

	cluster.Status = "online"
	cluster.LastSeen = time.Now()
	pm.clusters[cluster.ID] = cluster
	return nil
}

// CreateBackup creates a new VM backup.
func (pm *ProxmoxManager) CreateBackup(ctx context.Context, clusterID string, vmID int, backupType string) (*VMBackup, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	cluster, ok := pm.clusters[clusterID]
	if !ok {
		return nil, fmt.Errorf("cluster %s not found", clusterID)
	}

	if cluster.Status != "online" {
		return nil, fmt.Errorf("cluster %s is not online", clusterID)
	}

	backup := &VMBackup{
		ID:          fmt.Sprintf("bkp-%s-%d-%d", clusterID, vmID, time.Now().Unix()),
		ClusterID:   clusterID,
		VMID:        vmID,
		Type:        backupType,
		Status:      "pending",
		StartTime:   time.Now(),
		Storage:     "local",
		Compression: "zstd",
		Retention: RetentionPolicy{
			KeepDaily:   7,
			KeepWeekly:  4,
			KeepMonthly: 6,
			KeepYearly:  2,
			MaxAgeDays:  365,
		},
	}

	pm.backups[backup.ID] = backup
	return backup, nil
}

// RestoreBackup restores a VM from backup.
func (pm *ProxmoxManager) RestoreBackup(ctx context.Context, backupID string, targetNode string, targetVMID int) (*RestoreJob, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	backup, ok := pm.backups[backupID]
	if !ok {
		return nil, fmt.Errorf("backup %s not found", backupID)
	}

	if backup.Status != "completed" {
		return nil, fmt.Errorf("backup %s is not completed", backupID)
	}

	restore := &RestoreJob{
		ID:         fmt.Sprintf("rst-%s-%d", backupID, time.Now().Unix()),
		BackupID:   backupID,
		TargetNode: targetNode,
		TargetVMID: targetVMID,
		Status:     "pending",
		Progress:   0,
		StartTime:  time.Now(),
	}

	pm.restores[restore.ID] = restore
	return restore, nil
}

// CreateSchedule creates an automated backup schedule.
func (pm *ProxmoxManager) CreateSchedule(ctx context.Context, schedule *BackupSchedule) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if schedule.ID == "" {
		return fmt.Errorf("schedule ID is required")
	}

	pm.schedules[schedule.ID] = schedule
	return nil
}

// ListBackups lists all backups for a cluster.
func (pm *ProxmoxManager) ListBackups(ctx context.Context, clusterID string) []*VMBackup {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var result []*VMBackup
	for _, backup := range pm.backups {
		if backup.ClusterID == clusterID {
			result = append(result, backup)
		}
	}
	return result
}

// GetClusterStatus returns cluster health status.
func (pm *ProxmoxManager) GetClusterStatus(ctx context.Context, clusterID string) (*ProxmoxVE, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	cluster, ok := pm.clusters[clusterID]
	if !ok {
		return nil, fmt.Errorf("cluster %s not found", clusterID)
	}

	return cluster, nil
}
