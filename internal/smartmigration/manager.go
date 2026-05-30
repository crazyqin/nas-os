package smartmigration

import (
	"fmt"
	"sync"
	"time"
)

// MigrationType represents the type of migration
type MigrationType string

const (
	MigrationTypeDisk     MigrationType = "disk"
	MigrationTypeVolume   MigrationType = "volume"
	MigrationTypeShare    MigrationType = "share"
	MigrationTypeFull     MigrationType = "full_system"
	MigrationTypeRemote   MigrationType = "remote"
	MigrationTypeCloud    MigrationType = "cloud"
)

// MigrationStatus represents the status of a migration
type MigrationStatus string

const (
	StatusPending    MigrationStatus = "pending"
	StatusPlanning   MigrationStatus = "planning"
	StatusRunning    MigrationStatus = "running"
	StatusPaused     MigrationStatus = "paused"
	StatusCompleted  MigrationStatus = "completed"
	StatusFailed     MigrationStatus = "failed"
	StatusCancelled  MigrationStatus = "cancelled"
	StatusVerifying  MigrationStatus = "verifying"
)

// Migration represents a data migration task
type Migration struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Type          MigrationType   `json:"type"`
	Status        MigrationStatus `json:"status"`
	Source        MigrationEndpoint `json:"source"`
	Destination   MigrationEndpoint `json:"destination"`
	Options       MigrationOptions  `json:"options"`
	Progress      MigrationProgress `json:"progress"`
	StartTime     time.Time       `json:"start_time"`
	EndTime       *time.Time      `json:"end_time,omitempty"`
	Duration      time.Duration   `json:"duration"`
	EstimatedTime time.Duration   `json:"estimated_time"`
	ErrorMsg      string          `json:"error_msg,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// MigrationEndpoint represents a migration endpoint
type MigrationEndpoint struct {
	Type     string `json:"type"` // local, remote, cloud
	Host     string `json:"host,omitempty"`
	Path     string `json:"path"`
	Protocol string `json:"protocol,omitempty"`
	Port     int    `json:"port,omitempty"`
	Username string `json:"username,omitempty"`
	Token    string `json:"token,omitempty"`
}

// MigrationOptions represents migration options
type MigrationOptions struct {
	BandwidthLimit    int  `json:"bandwidth_limit_mbps"`
	Compression       bool `json:"compression"`
	Deduplication     bool `json:"deduplication"`
	Encryption        bool `json:"encryption"`
	VerifyAfterCopy   bool `json:"verify_after_copy"`
	DeleteSource      bool `json:"delete_source_after"`
	SyncMode          string `json:"sync_mode"` // full, incremental, differential
	RetryCount        int  `json:"retry_count"`
	ChunkSize         int  `json:"chunk_size_mb"`
	ParallelTransfers int  `json:"parallel_transfers"`
	PreservePerms     bool `json:"preserve_permissions"`
	PreserveTimestamps bool `json:"preserve_timestamps"`
}

// MigrationProgress represents migration progress
type MigrationProgress struct {
	TotalBytes      int64   `json:"total_bytes"`
	TransferredBytes int64  `json:"transferred_bytes"`
	TotalFiles      int     `json:"total_files"`
	TransferredFiles int   `json:"transferred_files"`
	FailedFiles     int     `json:"failed_files"`
	SkippedFiles    int     `json:"skipped_files"`
	PercentComplete float64 `json:"percent_complete"`
	CurrentSpeed    float64 `json:"current_speed_mbps"`
	AverageSpeed    float64 `json:"average_speed_mbps"`
	ETA             string  `json:"eta"`
	CurrentFile     string  `json:"current_file"`
}

// MigrationPlan represents a migration plan
type MigrationPlan struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Type            MigrationType   `json:"type"`
	Source          MigrationEndpoint `json:"source"`
	Destination     MigrationEndpoint `json:"destination"`
	EstimatedSize   int64           `json:"estimated_size_bytes"`
	EstimatedTime   time.Duration   `json:"estimated_time"`
	Steps           []PlanStep      `json:"steps"`
	Recommendations []string        `json:"recommendations"`
	Risks           []string        `json:"risks"`
	CreatedAt       time.Time       `json:"created_at"`
}

// PlanStep represents a step in the migration plan
type PlanStep struct {
	Order       int    `json:"order"`
	Name        string `json:"name"`
	Description string `json:"description"`
	EstimatedDuration time.Duration `json:"estimated_duration"`
	IsRequired  bool   `json:"is_required"`
}

// Manager manages data migrations
type Manager struct {
	mu         sync.RWMutex
	migrations map[string]*Migration
	plans      map[string]*MigrationPlan
}

// NewManager creates a new migration manager
func NewManager() *Manager {
	return &Manager{
		migrations: make(map[string]*Migration),
		plans:      make(map[string]*MigrationPlan),
	}
}

// CreateMigration creates a new migration task
func (m *Manager) CreateMigration(name string, mtype MigrationType, source, dest MigrationEndpoint, opts MigrationOptions) (*Migration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	migration := &Migration{
		ID:          fmt.Sprintf("mig-%d", time.Now().UnixNano()),
		Name:        name,
		Type:        mtype,
		Status:      StatusPending,
		Source:      source,
		Destination: dest,
		Options:     opts,
		Progress: MigrationProgress{
			PercentComplete: 0,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.migrations[migration.ID] = migration
	return migration, nil
}

// StartMigration starts a migration
func (m *Manager) StartMigration(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	migration, ok := m.migrations[id]
	if !ok {
		return fmt.Errorf("migration not found: %s", id)
	}

	if migration.Status != StatusPending && migration.Status != StatusPaused {
		return fmt.Errorf("migration cannot be started from status: %s", migration.Status)
	}

	migration.Status = StatusRunning
	migration.StartTime = time.Now()
	migration.UpdatedAt = time.Now()

	return nil
}

// PauseMigration pauses a running migration
func (m *Manager) PauseMigration(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	migration, ok := m.migrations[id]
	if !ok {
		return fmt.Errorf("migration not found: %s", id)
	}

	if migration.Status != StatusRunning {
		return fmt.Errorf("migration is not running")
	}

	migration.Status = StatusPaused
	migration.UpdatedAt = time.Now()

	return nil
}

// ResumeMigration resumes a paused migration
func (m *Manager) ResumeMigration(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	migration, ok := m.migrations[id]
	if !ok {
		return fmt.Errorf("migration not found: %s", id)
	}

	if migration.Status != StatusPaused {
		return fmt.Errorf("migration is not paused")
	}

	migration.Status = StatusRunning
	migration.UpdatedAt = time.Now()

	return nil
}

// CancelMigration cancels a migration
func (m *Manager) CancelMigration(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	migration, ok := m.migrations[id]
	if !ok {
		return fmt.Errorf("migration not found: %s", id)
	}

	if migration.Status == StatusCompleted || migration.Status == StatusCancelled {
		return fmt.Errorf("migration already finished")
	}

	migration.Status = StatusCancelled
	now := time.Now()
	migration.EndTime = &now
	migration.Duration = now.Sub(migration.StartTime)
	migration.UpdatedAt = now

	return nil
}

// UpdateProgress updates migration progress
func (m *Manager) UpdateProgress(id string, progress MigrationProgress) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	migration, ok := m.migrations[id]
	if !ok {
		return fmt.Errorf("migration not found: %s", id)
	}

	migration.Progress = progress
	migration.UpdatedAt = time.Now()

	if progress.PercentComplete >= 100 {
		migration.Status = StatusCompleted
		now := time.Now()
		migration.EndTime = &now
		migration.Duration = now.Sub(migration.StartTime)
	}

	return nil
}

// GetMigration returns a migration by ID
func (m *Manager) GetMigration(id string) (*Migration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	migration, ok := m.migrations[id]
	if !ok {
		return nil, fmt.Errorf("migration not found: %s", id)
	}

	return migration, nil
}

// ListMigrations lists all migrations
func (m *Manager) ListMigrations(status MigrationStatus) []*Migration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Migration, 0)
	for _, migration := range m.migrations {
		if status == "" || migration.Status == status {
			result = append(result, migration)
		}
	}

	return result
}

// CreatePlan creates a migration plan
func (m *Manager) CreatePlan(name string, mtype MigrationType, source, dest MigrationEndpoint) (*MigrationPlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan := &MigrationPlan{
		ID:   fmt.Sprintf("plan-%d", time.Now().UnixNano()),
		Name: name,
		Type: mtype,
		Source: source,
		Destination: dest,
		Steps: []PlanStep{
			{Order: 1, Name: "pre_check", Description: "Pre-migration checks", EstimatedDuration: 5 * time.Minute, IsRequired: true},
			{Order: 2, Name: "prepare", Description: "Prepare destination", EstimatedDuration: 10 * time.Minute, IsRequired: true},
			{Order: 3, Name: "copy_data", Description: "Copy data", EstimatedDuration: 60 * time.Minute, IsRequired: true},
			{Order: 4, Name: "verify", Description: "Verify data integrity", EstimatedDuration: 15 * time.Minute, IsRequired: true},
			{Order: 5, Name: "cleanup", Description: "Cleanup and finalize", EstimatedDuration: 5 * time.Minute, IsRequired: false},
		},
		Recommendations: []string{
			"Schedule migration during low-usage hours",
			"Ensure sufficient disk space on destination",
			"Create a backup before starting",
		},
		Risks: []string{
			"Network interruption may require restart",
			"Large files may take longer than estimated",
		},
		CreatedAt: time.Now(),
	}

	m.plans[plan.ID] = plan
	return plan, nil
}

// GetPlan returns a plan by ID
func (m *Manager) GetPlan(id string) (*MigrationPlan, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[id]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", id)
	}

	return plan, nil
}

// EstimateMigration estimates migration time and size
func (m *Manager) EstimateMigration(source MigrationEndpoint) (int64, time.Duration, error) {
	// Simulate estimation
	totalBytes := int64(1024 * 1024 * 1024 * 100) // 100GB
	estimatedTime := 2 * time.Hour

	return totalBytes, estimatedTime, nil
}
