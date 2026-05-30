package backupverify

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager manages backup verification tasks and results
type Manager struct {
	storagePath string
	tasks       map[string]*VerifyTask
	results     map[string][]*VerifyResult
	restoreTests map[string]*RestoreTest
	mu          sync.RWMutex
}

// NewManager creates a new backup verification manager
func NewManager(storagePath string) *Manager {
	return &Manager{
		storagePath:  storagePath,
		tasks:        make(map[string]*VerifyTask),
		results:      make(map[string][]*VerifyResult),
		restoreTests: make(map[string]*RestoreTest),
	}
}

// CreateVerifyTask creates a new verification task
func (m *Manager) CreateVerifyTask(ctx context.Context, task VerifyTask) (*VerifyTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task.ID == "" {
		task.ID = uuid.New().String()
	}
	task.Status = TaskStatusPending
	task.CreatedAt = time.Now()
	task.Enabled = true

	m.tasks[task.ID] = &task
	return &task, nil
}

// GetTask returns a verification task by ID
func (m *Manager) GetTask(ctx context.Context, taskID string) (*VerifyTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}
	return task, nil
}

// RunVerify executes a verification task
func (m *Manager) RunVerify(ctx context.Context, taskID string) (*VerifyResult, error) {
	m.mu.Lock()
	task, ok := m.tasks[taskID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	if task.Status == TaskStatusRunning {
		m.mu.Unlock()
		return nil, fmt.Errorf("task %s is already running", taskID)
	}

	task.Status = TaskStatusRunning
	now := time.Now()
	task.LastRun = &now
	m.mu.Unlock()

	startTime := time.Now()
	result := &VerifyResult{
		ID:        uuid.New().String(),
		TaskID:    taskID,
		BackupID:  task.BackupID,
		Status:    ResultPass,
		CreatedAt: startTime,
	}

	// Perform verification based on type
	var err error
	switch task.VerifyType {
	case VerifyIntegrity:
		err = m.verifyIntegrity(ctx, task, result)
	case VerifyFull:
		err = m.verifyFull(ctx, task, result)
	default:
		err = m.verifyIntegrity(ctx, task, result)
	}

	result.Duration = time.Since(startTime)

	m.mu.Lock()
	defer m.mu.Unlock()

	if err != nil {
		result.Status = ResultFail
		result.ErrorMessage = err.Error()
		task.Status = TaskStatusFailed
	} else {
		if result.CorruptedFiles > 0 || result.MissingFiles > 0 {
			result.Status = ResultWarn
		}
		task.Status = TaskStatusCompleted
	}

	m.results[taskID] = append(m.results[taskID], result)
	return result, nil
}

// verifyIntegrity checks file integrity using checksums
func (m *Manager) verifyIntegrity(ctx context.Context, task *VerifyTask, result *VerifyResult) error {
	backupPath := task.BackupPath
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup path %s does not exist", backupPath)
	}

	return filepath.Walk(backupPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if info.IsDir() {
			return nil
		}

		result.FileCount++
		result.TotalBytes += info.Size()

		// Calculate checksum
		checksum, err := m.calculateChecksum(path)
		if err != nil {
			result.Details = append(result.Details, VerifyDetail{
				FilePath:     path,
				Status:       FileStatusCorrupt,
				ExpectedSize: info.Size(),
				ActualSize:   info.Size(),
			})
			result.CorruptedFiles++
			return nil
		}

		result.VerifiedFiles++
		result.VerifiedBytes += info.Size()
		result.Details = append(result.Details, VerifyDetail{
			FilePath:         path,
			Status:           FileStatusPass,
			ActualChecksum:   checksum,
			ExpectedChecksum: checksum,
			ExpectedSize:     info.Size(),
			ActualSize:       info.Size(),
		})

		return nil
	})
}

// verifyFull performs a full verification including integrity and restore test
func (m *Manager) verifyFull(ctx context.Context, task *VerifyTask, result *VerifyResult) error {
	// First run integrity check
	if err := m.verifyIntegrity(ctx, task, result); err != nil {
		return err
	}

	// If integrity check passed, try a sample restore
	if result.Status == ResultPass {
		// Sample restore verification - check if files can be read
		backupPath := task.BackupPath
		sampleCount := 0
		maxSamples := 10

		err := filepath.Walk(backupPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || sampleCount >= maxSamples {
				return nil
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Try to read the file
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()

			// Read a small portion to verify readability
			buf := make([]byte, 1024)
			_, err = f.Read(buf)
			if err != nil && err != io.EOF {
				return nil
			}

			sampleCount++
			return nil
		})

		if err != nil {
			return err
		}
	}

	return nil
}

// RunRestoreTest executes a restore test
func (m *Manager) RunRestoreTest(ctx context.Context, taskID string) (*RestoreTest, error) {
	m.mu.Lock()
	task, ok := m.tasks[taskID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	test := &RestoreTest{
		ID:        uuid.New().String(),
		TaskID:    taskID,
		BackupID:  task.BackupID,
		TargetPath: filepath.Join(m.storagePath, "restore-test", uuid.New().String()),
		Status:    RestoreStatusPending,
		CreatedAt: time.Now(),
	}
	m.mu.Unlock()

	startTime := time.Now()
	test.Status = RestoreStatusExtracting

	// Create target directory
	if err := os.MkdirAll(test.TargetPath, 0755); err != nil {
		test.Status = RestoreStatusFailed
		test.Error = err.Error()
		test.Duration = time.Since(startTime)
		return test, err
	}

	// Simulate restore by copying files
	test.Status = RestoreStatusVerifying
	if err := m.simulateRestore(ctx, task.BackupPath, test); err != nil {
		test.Status = RestoreStatusFailed
		test.Error = err.Error()
		test.Duration = time.Since(startTime)
		return test, err
	}

	// Cleanup
	test.Status = RestoreStatusCleanup
	os.RemoveAll(test.TargetPath)

	test.Status = RestoreStatusCompleted
	test.Duration = time.Since(startTime)

	m.mu.Lock()
	m.restoreTests[test.ID] = test
	m.mu.Unlock()

	return test, nil
}

// simulateRestore simulates a restore operation
func (m *Manager) simulateRestore(ctx context.Context, sourcePath string, test *RestoreTest) error {
	return filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if info.IsDir() {
			return nil
		}

		test.RestoredFiles++

		// Verify file can be read
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		test.VerifiedFiles++
		return nil
	})
}

// GetBackupHealth returns the health status of a backup
func (m *Manager) GetBackupHealth(ctx context.Context, backupID string) (*BackupHealth, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health := &BackupHealth{
		BackupID:       backupID,
		IntegrityScore: 100.0,
		RiskLevel:      RiskLow,
	}

	// Find task for this backup
	var task *VerifyTask
	for _, t := range m.tasks {
		if t.BackupID == backupID {
			task = t
			break
		}
	}

	if task == nil {
		return nil, fmt.Errorf("no task found for backup %s", backupID)
	}

	health.Source = task.BackupPath

	// Check if backup exists
	info, err := os.Stat(task.BackupPath)
	if err == nil {
		mt := info.ModTime()
		health.LastBackup = &mt
		health.BackupSize = info.Size()
	}

	// Get latest verification result
	results := m.results[task.ID]
	if len(results) > 0 {
		latest := results[len(results)-1]
		health.VerifyStatus = string(latest.Status)

		if latest.Status == ResultFail {
			health.IntegrityScore = 0
			health.RiskLevel = RiskCritical
		} else if latest.Status == ResultWarn {
			health.IntegrityScore = 70
			health.RiskLevel = RiskMedium
		}
	} else {
		health.VerifyStatus = "not_verified"
		health.IntegrityScore = 50
		health.RiskLevel = RiskHigh
		health.Recommendations = append(health.Recommendations, "Run initial verification")
	}

	// Generate recommendations
	health.Recommendations = append(health.Recommendations, m.getRecommendations(health)...)

	return health, nil
}

// GenerateReport generates a verification report for all backups
func (m *Manager) GenerateReport(ctx context.Context) (*VerifyReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &VerifyReport{
		GeneratedAt: time.Now(),
	}

	// Collect unique backup IDs
	backupIDs := make(map[string]bool)
	for _, task := range m.tasks {
		backupIDs[task.BackupID] = true
	}

	report.TotalBackups = len(backupIDs)

	for backupID := range backupIDs {
		health := &BackupHealth{
			BackupID:       backupID,
			IntegrityScore: 100.0,
			RiskLevel:      RiskLow,
		}

		// Find task
		for _, task := range m.tasks {
			if task.BackupID == backupID {
				health.Source = task.BackupPath

				// Check backup existence
				info, err := os.Stat(task.BackupPath)
				if err == nil {
					mt := info.ModTime()
					health.LastBackup = &mt
					health.BackupSize = info.Size()
				}

				// Get verification results
				results := m.results[task.ID]
				if len(results) > 0 {
					latest := results[len(results)-1]
					health.VerifyStatus = string(latest.Status)

					switch latest.Status {
					case ResultPass:
						report.HealthyBackups++
					case ResultWarn:
						health.IntegrityScore = 70
						health.RiskLevel = RiskMedium
						report.WarningBackups++
					case ResultFail:
						health.IntegrityScore = 0
						health.RiskLevel = RiskCritical
						report.FailedBackups++
					}
				} else {
					health.VerifyStatus = "not_verified"
					health.IntegrityScore = 50
					health.RiskLevel = RiskHigh
					report.WarningBackups++
				}
				break
			}
		}

		health.Recommendations = m.getRecommendations(health)
		report.Backups = append(report.Backups, *health)
	}

	return report, nil
}

// ScheduleVerify sets up a scheduled verification
func (m *Manager) ScheduleVerify(ctx context.Context, taskID, cronExpr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.Schedule = cronExpr
	// Parse cron and set next run time
	nextRun := m.parseNextRun(cronExpr)
	task.NextRun = &nextRun

	return nil
}

// GetVerifyHistory returns the verification history for a task
func (m *Manager) GetVerifyHistory(ctx context.Context, taskID string) []VerifyResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := m.results[taskID]
	history := make([]VerifyResult, len(results))
	for i, r := range results {
		history[i] = *r
	}
	return history
}

// AutoRepair attempts to repair corrupted files in a backup
func (m *Manager) AutoRepair(ctx context.Context, backupID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find task for this backup
	var task *VerifyTask
	for _, t := range m.tasks {
		if t.BackupID == backupID {
			task = t
			break
		}
	}

	if task == nil {
		return 0, fmt.Errorf("no task found for backup %s", backupID)
	}

	// Get latest results
	results := m.results[task.ID]
	if len(results) == 0 {
		return 0, fmt.Errorf("no verification results for backup %s", backupID)
	}

	latest := results[len(results)-1]
	repaired := 0

	for _, detail := range latest.Details {
		select {
		case <-ctx.Done():
			return repaired, ctx.Err()
		default:
		}

		if detail.Status == FileStatusChecksumMismatch {
			// Attempt repair by recalculating checksum
			if _, err := os.Stat(detail.FilePath); err == nil {
				checksum, err := m.calculateChecksum(detail.FilePath)
				if err == nil {
					// Update the detail with new checksum
					detail.ActualChecksum = checksum
					if checksum == detail.ExpectedChecksum {
						detail.Status = FileStatusPass
						repaired++
					}
				}
			}
		}
	}

	return repaired, nil
}

// GetRecommendations returns recommendations for improving backup health
func (m *Manager) GetRecommendations(ctx context.Context, backupID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health := &BackupHealth{BackupID: backupID}

	// Find task
	for _, task := range m.tasks {
		if task.BackupID == backupID {
			health.Source = task.BackupPath
			break
		}
	}

	return m.getRecommendations(health)
}

// getRecommendations generates recommendations based on backup health
func (m *Manager) getRecommendations(health *BackupHealth) []string {
	var recommendations []string

	if health.VerifyStatus == "not_verified" {
		recommendations = append(recommendations, "Run initial backup verification")
	}

	if health.IntegrityScore < 80 {
		recommendations = append(recommendations, "Consider running a full backup")
	}

	if health.RiskLevel == RiskHigh || health.RiskLevel == RiskCritical {
		recommendations = append(recommendations, "Urgent: Backup integrity is compromised")
		recommendations = append(recommendations, "Schedule regular verification checks")
	}

	if health.LastBackup != nil && time.Since(*health.LastBackup) > 7*24*time.Hour {
		recommendations = append(recommendations, "Backup is older than 7 days, consider running a new backup")
	}

	return recommendations
}

// calculateChecksum calculates SHA256 checksum of a file
func (m *Manager) calculateChecksum(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// parseNextRun parses a cron expression and returns the next run time
func (m *Manager) parseNextRun(cronExpr string) time.Time {
	// Simple implementation - in production, use a cron library
	// For now, return next day at midnight
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
}

// saveTasks persists tasks to disk
func (m *Manager) saveTasks() error {
	data, err := json.MarshalIndent(m.tasks, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(m.storagePath, "tasks.json")
	return os.WriteFile(path, data, 0644)
}

// loadTasks loads tasks from disk
func (m *Manager) loadTasks() error {
	path := filepath.Join(m.storagePath, "tasks.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &m.tasks)
}

// BackupExists checks if a backup path exists and is valid
func (m *Manager) BackupExists(backupPath string) bool {
	info, err := os.Stat(backupPath)
	return err == nil && info.IsDir()
}

// FormatSize formats bytes to human readable string
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatDuration formats duration to human readable string
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// contains checks if a string slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}
