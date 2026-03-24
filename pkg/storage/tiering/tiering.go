// Package tiering provides hot/cold data tiering for NAS storage optimization.
// Inspired by Synology DSM 7.3 Tiering feature.
package tiering

import (
	"context"
	"sync"
	"time"
)

// Tier represents a storage tier level
type Tier int

const (
	// TierHot is for frequently accessed data (SSD/NVMe)
	TierHot Tier = iota
	// TierWarm is for moderately accessed data (HDD)
	TierWarm
	// TierCold is for rarely accessed data (Archive/Cloud)
	TierCold
)

// String returns the tier name
func (t Tier) String() string {
	switch t {
	case TierHot:
		return "hot"
	case TierWarm:
		return "warm"
	case TierCold:
		return "cold"
	default:
		return "unknown"
	}
}

// Policy defines tiering policy rules
type Policy struct {
	// Name is the policy identifier
	Name string `json:"name"`
	// Enabled indicates if policy is active
	Enabled bool `json:"enabled"`
	// HotThreshold is the access count threshold for hot tier
	HotThreshold int `json:"hot_threshold"`
	// ColdThreshold is the days since last access for cold tier
	ColdThreshold int `json:"cold_threshold"`
	// MinFileSize is minimum file size in bytes for tiering
	MinFileSize int64 `json:"min_file_size"`
	// MaxFileSize is maximum file size in bytes for tiering
	MaxFileSize int64 `json:"max_file_size"`
	// Schedule is the tiering schedule (cron format)
	Schedule string `json:"schedule"`
	// ExcludePatterns are patterns to exclude from tiering
	ExcludePatterns []string `json:"exclude_patterns"`
	// IncludePaths are paths to include for tiering
	IncludePaths []string `json:"include_paths"`
}

// FileInfo represents file metadata for tiering decisions
type FileInfo struct {
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	CurrentTier  Tier      `json:"current_tier"`
	AccessCount  int       `json:"access_count"`
	LastAccess   time.Time `json:"last_access"`
	LastModified time.Time `json:"last_modified"`
	CreatedAt    time.Time `json:"created_at"`
}

// MigrationTask represents a tiering migration task
type MigrationTask struct {
	ID          string    `json:"id"`
	SourcePath  string    `json:"source_path"`
	SourceTier  Tier      `json:"source_tier"`
	TargetTier  Tier      `json:"target_tier"`
	Status      string    `json:"status"`
	Progress    float64   `json:"progress"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	Error       string    `json:"error,omitempty"`
}

// Stats holds tiering statistics
type Stats struct {
	TotalFiles      int64            `json:"total_files"`
	TotalSize       int64            `json:"total_size"`
	FilesByTier     map[Tier]int64   `json:"files_by_tier"`
	SizeByTier      map[Tier]int64   `json:"size_by_tier"`
	MigrationsToday int              `json:"migrations_today"`
	LastRun         time.Time        `json:"last_run"`
}

// Manager manages storage tiering operations
type Manager struct {
	mu       sync.RWMutex
	policies map[string]*Policy
	tasks    map[string]*MigrationTask
	stats    Stats
	running  bool
	cancel   context.CancelFunc
}

// NewManager creates a new tiering manager
func NewManager() *Manager {
	return &Manager{
		policies: make(map[string]*Policy),
		tasks:    make(map[string]*MigrationTask),
		stats: Stats{
			FilesByTier: make(map[Tier]int64),
			SizeByTier:  make(map[Tier]int64),
		},
	}
}

// DefaultPolicy returns the default tiering policy
func DefaultPolicy() *Policy {
	return &Policy{
		Name:           "default",
		Enabled:        true,
		HotThreshold:   100,   // 100+ accesses = hot
		ColdThreshold:  30,    // 30+ days = cold
		MinFileSize:    1024,  // 1KB minimum
		MaxFileSize:    0,     // no max limit
		Schedule:       "0 2 * * *", // 2 AM daily
		ExcludePatterns: []string{"*.tmp", "*.log", "*.cache"},
	}
}

// AddPolicy adds a tiering policy
func (m *Manager) AddPolicy(policy *Policy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.Name == "" {
		return ErrInvalidPolicy
	}

	m.policies[policy.Name] = policy
	return nil
}

// GetPolicy retrieves a policy by name
func (m *Manager) GetPolicy(name string) (*Policy, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.policies[name]
	return policy, ok
}

// ListPolicies returns all policies
func (m *Manager) ListPolicies() []*Policy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*Policy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// RemovePolicy removes a policy
func (m *Manager) RemovePolicy(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[name]; !ok {
		return ErrPolicyNotFound
	}

	delete(m.policies, name)
	return nil
}

// AnalyzePath analyzes a path and returns tiering recommendations
func (m *Manager) AnalyzePath(ctx context.Context, path string) ([]FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// TODO: Implement actual file system scanning
	return nil, nil
}

// DetermineTier determines the appropriate tier for a file
func (m *Manager) DetermineTier(info FileInfo, policy *Policy) Tier {
	// Check access frequency for hot tier
	if info.AccessCount >= policy.HotThreshold {
		return TierHot
	}

	// Check last access time for cold tier
	daysSinceAccess := int(time.Since(info.LastAccess).Hours() / 24)
	if daysSinceAccess >= policy.ColdThreshold {
		return TierCold
	}

	// Default to warm tier
	return TierWarm
}

// MigrateFile migrates a file to a different tier
func (m *Manager) MigrateFile(ctx context.Context, info FileInfo, targetTier Tier) (*MigrationTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task := &MigrationTask{
		ID:         generateTaskID(),
		SourcePath: info.Path,
		SourceTier: info.CurrentTier,
		TargetTier: targetTier,
		Status:     "pending",
		StartedAt:  time.Now(),
	}

	m.tasks[task.ID] = task

	// TODO: Implement actual file migration
	return task, nil
}

// GetTask retrieves a migration task by ID
func (m *Manager) GetTask(id string) (*MigrationTask, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[id]
	return task, ok
}

// ListTasks returns all migration tasks
func (m *Manager) ListTasks() []*MigrationTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*MigrationTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// GetStats returns tiering statistics
func (m *Manager) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.stats
}

// Start starts the tiering manager
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return ErrAlreadyRunning
	}

	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.running = true

	go m.runScheduler(ctx)

	return nil
}

// Stop stops the tiering manager
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return ErrNotRunning
	}

	m.cancel()
	m.running = false

	return nil
}

func (m *Manager) runScheduler(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.runTieringJob(ctx)
		}
	}
}

func (m *Manager) runTieringJob(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.LastRun = time.Now()
	// TODO: Implement actual tiering job
}

func generateTaskID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().Nanosecond()%len(letters)]
	}
	return string(b)
}