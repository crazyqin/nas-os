// Package tiering provides hot/cold data tiering for NAS storage optimization.
// This file implements automatic data migration between tiers.
package tiering

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// Migrator handles file migration between tiers
type Migrator struct {
	mu       sync.RWMutex
	config   MigratorConfig
	queues   map[Tier][]MigrationTask
	running  bool
	cancel   context.CancelFunc
	stats    MigrationStats
	progress map[string]MigrationProgress
}

// MigratorConfig configures the migrator
type MigratorConfig struct {
	// HotPath is the path for hot tier storage (SSD/NVMe)
	HotPath string `json:"hot_path"`
	// WarmPath is the path for warm tier storage (HDD)
	WarmPath string `json:"warm_path"`
	// ColdPath is the path for cold tier storage (Archive/Cloud)
	ColdPath string `json:"cold_path"`
	// MaxConcurrency is the maximum number of concurrent migrations
	MaxConcurrency int `json:"max_concurrency"`
	// BatchSize is the number of files to process in each batch
	BatchSize int `json:"batch_size"`
	// PreservePermissions preserves file permissions during migration
	PreservePermissions bool `json:"preserve_permissions"`
	// VerifyAfterMigration verifies file integrity after migration
	VerifyAfterMigration bool `json:"verify_after_migration"`
	// DeleteSource deletes source file after successful migration
	DeleteSource bool `json:"delete_source"`
	// RetryAttempts is the number of retry attempts for failed migrations
	RetryAttempts int `json:"retry_attempts"`
	// RetryDelay is the delay between retry attempts
	RetryDelay time.Duration `json:"retry_delay"`
}

// MigrationStats holds migration statistics
type MigrationStats struct {
	TotalMigrations  int64         `json:"total_migrations"`
	Successful       int64         `json:"successful"`
	Failed           int64         `json:"failed"`
	TotalBytesMoved  int64         `json:"total_bytes_moved"`
	TotalTime        time.Duration `json:"total_time"`
	LastMigration    time.Time     `json:"last_migration"`
	ActiveMigrations int           `json:"active_migrations"`
}

// MigrationProgress tracks individual migration progress
type MigrationProgress struct {
	TaskID       string    `json:"task_id"`
	Path         string    `json:"path"`
	SourceTier   Tier      `json:"source_tier"`
	TargetTier   Tier      `json:"target_tier"`
	BytesTotal   int64     `json:"bytes_total"`
	BytesMoved   int64     `json:"bytes_moved"`
	Percent      float64   `json:"percent"`
	Speed        float64   `json:"speed"` // MB/s
	StartedAt    time.Time `json:"started_at"`
	EstimatedEnd time.Time `json:"estimated_end,omitempty"`
	Status       string    `json:"status"`
}

// TierLocation maps tiers to storage locations
type TierLocation struct {
	Tier     Tier
	Path     string
	Capacity int64
	Used     int64
}

// NewMigrator creates a new migrator
func NewMigrator(config MigratorConfig) *Migrator {
	if config.MaxConcurrency <= 0 {
		config.MaxConcurrency = 4
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 100
	}
	if config.RetryAttempts <= 0 {
		config.RetryAttempts = 3
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = 5 * time.Second
	}

	return &Migrator{
		config:   config,
		queues:   make(map[Tier][]MigrationTask),
		progress: make(map[string]MigrationProgress),
	}
}

// Migrate migrates a file to a target tier
func (m *Migrator) Migrate(ctx context.Context, task *MigrationTask) error {
	m.mu.Lock()
	m.stats.ActiveMigrations++
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.stats.ActiveMigrations--
		m.mu.Unlock()
	}()

	// Get source and target paths
	sourcePath, targetPath, err := m.getTierPaths(task.SourceTier, task.TargetTier, task.SourcePath)
	if err != nil {
		return err
	}

	// Check if same tier
	if task.SourceTier == task.TargetTier {
		return ErrSameTier
	}

	// Get file info
	info, err := os.Stat(sourcePath)
	if err != nil {
		return ErrFileNotFound
	}

	// Initialize progress
	progress := MigrationProgress{
		TaskID:     task.ID,
		Path:       task.SourcePath,
		SourceTier: task.SourceTier,
		TargetTier: task.TargetTier,
		BytesTotal: info.Size(),
		StartedAt:  time.Now(),
		Status:     "in_progress",
	}

	m.mu.Lock()
	m.progress[task.ID] = progress
	m.mu.Unlock()

	// Create target directory if needed
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Perform migration with retries
	var lastErr error
	for attempt := 0; attempt < m.config.RetryAttempts; attempt++ {
		err := m.doMigration(ctx, sourcePath, targetPath, info, task.ID)
		if err == nil {
			// Success
			m.mu.Lock()
			m.stats.TotalMigrations++
			m.stats.Successful++
			m.stats.TotalBytesMoved += info.Size()
			m.stats.LastMigration = time.Now()
			progress.Status = "completed"
			progress.BytesMoved = info.Size()
			progress.Percent = 100
			m.progress[task.ID] = progress
			m.mu.Unlock()

			// Update task status
			task.Status = "completed"
			task.CompletedAt = time.Now()

			return nil
		}
		lastErr = err

		// Wait before retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(m.config.RetryDelay):
		}
	}

	// All retries failed
	m.mu.Lock()
	m.stats.TotalMigrations++
	m.stats.Failed++
	progress.Status = "failed"
	m.progress[task.ID] = progress
	m.mu.Unlock()

	task.Status = "failed"
	task.Error = lastErr.Error()

	return fmt.Errorf("migration failed after %d attempts: %w", m.config.RetryAttempts, lastErr)
}

// doMigration performs the actual file migration
func (m *Migrator) doMigration(ctx context.Context, sourcePath, targetPath string, info os.FileInfo, taskID string) error {
	// Open source file
	src, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer src.Close()

	// Create target file
	dst, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create target: %w", err)
	}
	defer dst.Close()

	// Copy file content with progress tracking
	buf := make([]byte, 32*1024) // 32KB buffer
	var copied int64

	for {
		select {
		case <-ctx.Done():
			// Clean up partial file
			os.Remove(targetPath)
			return ctx.Err()
		default:
		}

		n, err := src.Read(buf)
		if n > 0 {
			written, writeErr := dst.Write(buf[:n])
			if writeErr != nil {
				os.Remove(targetPath)
				return writeErr
			}
			copied += int64(written)

			// Update progress
			m.mu.Lock()
			if p, ok := m.progress[taskID]; ok {
				p.BytesMoved = copied
				p.Percent = float64(copied) / float64(info.Size()) * 100
				if time.Since(p.StartedAt) > 0 {
					p.Speed = float64(copied) / 1024 / 1024 / time.Since(p.StartedAt).Seconds()
				}
				m.progress[taskID] = p
			}
			m.mu.Unlock()
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			os.Remove(targetPath)
			return fmt.Errorf("read error: %w", err)
		}
	}

	// Sync to ensure data is written
	if err := dst.Sync(); err != nil {
		os.Remove(targetPath)
		return fmt.Errorf("sync error: %w", err)
	}

	// Preserve permissions if configured
	if m.config.PreservePermissions {
		if err := os.Chmod(targetPath, info.Mode()); err != nil {
			os.Remove(targetPath)
			return fmt.Errorf("failed to set permissions: %w", err)
		}

		// Try to preserve timestamps
		if err := os.Chtimes(targetPath, time.Now(), info.ModTime()); err != nil {
			// Non-fatal, just log
		}
	}

	// Verify if configured
	if m.config.VerifyAfterMigration {
		if err := m.verifyMigration(sourcePath, targetPath); err != nil {
			os.Remove(targetPath)
			return fmt.Errorf("verification failed: %w", err)
		}
	}

	// Delete source if configured
	if m.config.DeleteSource {
		if err := os.Remove(sourcePath); err != nil {
			// Non-fatal, but return warning
		}
	}

	return nil
}

// verifyMigration verifies file integrity after migration
func (m *Migrator) verifyMigration(sourcePath, targetPath string) error {
	srcInfo, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}

	dstInfo, err := os.Stat(targetPath)
	if err != nil {
		return err
	}

	// Check size matches
	if srcInfo.Size() != dstInfo.Size() {
		return fmt.Errorf("size mismatch: source=%d, target=%d", srcInfo.Size(), dstInfo.Size())
	}

	// Could add checksum verification here
	return nil
}

// getTierPaths returns source and target paths for migration
func (m *Migrator) getTierPaths(sourceTier, targetTier Tier, path string) (string, string, error) {
	sourceBase, ok := m.getTierBasePath(sourceTier)
	if !ok {
		return "", "", fmt.Errorf("unknown source tier: %s", sourceTier)
	}

	targetBase, ok := m.getTierBasePath(targetTier)
	if !ok {
		return "", "", fmt.Errorf("unknown target tier: %s", targetTier)
	}

	// Use original path if it exists, otherwise construct
	sourcePath := path
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		// Try relative to tier base
		sourcePath = filepath.Join(sourceBase, path)
	}

	// Construct target path
	relPath := path
	if filepath.IsAbs(path) {
		// Convert to relative path
		relPath = filepath.Base(path)
	}
	targetPath := filepath.Join(targetBase, relPath)

	return sourcePath, targetPath, nil
}

// getTierBasePath returns the base path for a tier
func (m *Migrator) getTierBasePath(tier Tier) (string, bool) {
	switch tier {
	case TierHot:
		return m.config.HotPath, true
	case TierWarm:
		return m.config.WarmPath, true
	case TierCold:
		return m.config.ColdPath, true
	default:
		return "", false
	}
}

// QueueMigration queues a migration task
func (m *Migrator) QueueMigration(task MigrationTask) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.queues[task.TargetTier] = append(m.queues[task.TargetTier], task)
}

// ProcessQueue processes pending migrations
func (m *Migrator) ProcessQueue(ctx context.Context, targetTier Tier) error {
	m.mu.Lock()
	tasks := m.queues[targetTier]
	m.queues[targetTier] = nil
	m.mu.Unlock()

	for _, task := range tasks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := m.Migrate(ctx, &task); err != nil {
			// Log error but continue with other tasks
			continue
		}
	}

	return nil
}

// GetProgress returns migration progress for a task
func (m *Migrator) GetProgress(taskID string) (MigrationProgress, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	progress, ok := m.progress[taskID]
	return progress, ok
}

// GetStats returns migration statistics
func (m *Migrator) GetStats() MigrationStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// Start starts the migrator background process
func (m *Migrator) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return ErrAlreadyRunning
	}

	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.running = true
	m.mu.Unlock()

	go m.runBackground(ctx)
	return nil
}

// Stop stops the migrator
func (m *Migrator) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return ErrNotRunning
	}

	m.cancel()
	m.running = false
	return nil
}

func (m *Migrator) runBackground(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Process all tier queues
			for tier := range m.queues {
				m.ProcessQueue(ctx, tier)
			}
		}
	}
}

// TierCapacity returns capacity info for a tier
func (m *Migrator) TierCapacity(tier Tier) (*TierLocation, error) {
	basePath, ok := m.getTierBasePath(tier)
	if !ok {
		return nil, fmt.Errorf("unknown tier: %s", tier)
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(basePath, &stat); err != nil {
		return nil, err
	}

	capacity := stat.Blocks * uint64(stat.Bsize)
	used := (stat.Blocks - stat.Bfree) * uint64(stat.Bsize)

	return &TierLocation{
		Tier:     tier,
		Path:     basePath,
		Capacity: int64(capacity),
		Used:     int64(used),
	}, nil
}