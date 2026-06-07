package backupencrypt

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"
)

// Manager manages the backup encryption system
type Manager struct {
	mu          sync.RWMutex
	config      *BackupEncryptConfig
	backups     map[string]*EncryptedBackup
	keys        map[string]*BackupKey
	schedules   map[string]*BackupSchedule
	restores    map[string]*RestoreJob
	integrities map[string]*IntegrityCheck
}

// NewManager creates a new backup encryption manager
func NewManager(config *BackupEncryptConfig) *Manager {
	if config == nil {
		config = &BackupEncryptConfig{
			DefaultAlgo:       AES256GCM,
			ChunkSize:         1024 * 1024, // 1MB
			MaxParallel:       4,
			VerifyAfterBackup: true,
			AutoKeyRotation:   false,
		}
	}

	return &Manager{
		config:      config,
		backups:     make(map[string]*EncryptedBackup),
		keys:        make(map[string]*BackupKey),
		schedules:   make(map[string]*BackupSchedule),
		restores:    make(map[string]*RestoreJob),
		integrities: make(map[string]*IntegrityCheck),
	}
}

// CreateBackup creates a new encrypted backup
func (m *Manager) CreateBackup(name, src, dst, keyID string) (*EncryptedBackup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate key exists
	key, ok := m.keys[keyID]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", keyID)
	}

	if key.RevokedAt != nil {
		return nil, fmt.Errorf("key is revoked: %s", keyID)
	}

	now := time.Now()
	backup := &EncryptedBackup{
		ID:             fmt.Sprintf("backup-%d", now.UnixNano()),
		Name:           name,
		SourcePath:     src,
		DestPath:       dst,
		Status:         StatusPending,
		EncryptionAlgo: key.Algorithm,
		KeyID:          keyID,
		Progress:       0,
		CreatedAt:      now,
	}

	m.backups[backup.ID] = backup

	// Simulate async backup process
	go m.processBackup(backup)

	return backup, nil
}

func (m *Manager) processBackup(backup *EncryptedBackup) {
	m.mu.Lock()
	backup.Status = StatusEncrypting
	backup.Progress = 0.1
	m.mu.Unlock()

	time.Sleep(100 * time.Millisecond)

	m.mu.Lock()
	backup.Status = StatusUploading
	backup.Progress = 0.5
	m.mu.Unlock()

	time.Sleep(100 * time.Millisecond)

	m.mu.Lock()
	backup.Status = StatusCompleted
	backup.Progress = 1.0
	now := time.Now()
	backup.CompletedAt = &now
	backup.Size = 1024 * 1024 // Simulated size
	backup.EncryptedSize = int64(float64(backup.Size) * 1.05)
	backup.CompressionRatio = 0.95
	backup.Checksum = m.generateChecksum(backup.ID)
	m.mu.Unlock()

	log.Printf("Backup completed: %s", backup.ID)
}

// GetBackup returns a backup by ID
func (m *Manager) GetBackup(backupID string) (*EncryptedBackup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	backup, ok := m.backups[backupID]
	if !ok {
		return nil, fmt.Errorf("backup not found: %s", backupID)
	}

	return backup, nil
}

// ListBackups returns all backups
func (m *Manager) ListBackups() ([]EncryptedBackup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	backups := make([]EncryptedBackup, 0, len(m.backups))
	for _, backup := range m.backups {
		backups = append(backups, *backup)
	}

	return backups, nil
}

// RestoreBackup starts a restore job
func (m *Manager) RestoreBackup(backupID, destPath, keyID string) (*RestoreJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	backup, ok := m.backups[backupID]
	if !ok {
		return nil, fmt.Errorf("backup not found: %s", backupID)
	}

	if backup.Status != StatusCompleted {
		return nil, fmt.Errorf("backup is not completed: %s", backupID)
	}

	key, ok := m.keys[keyID]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", keyID)
	}

	if key.RevokedAt != nil {
		return nil, fmt.Errorf("key is revoked: %s", keyID)
	}

	now := time.Now()
	job := &RestoreJob{
		ID:         fmt.Sprintf("restore-%d", now.UnixNano()),
		BackupID:   backupID,
		DestPath:   destPath,
		Status:     RestorePending,
		Progress:   0,
		KeyID:      keyID,
		VerifyOnly: false,
		CreatedAt:  now,
	}

	m.restores[job.ID] = job

	// Simulate async restore
	go m.processRestore(job)

	return job, nil
}

func (m *Manager) processRestore(job *RestoreJob) {
	m.mu.Lock()
	job.Status = RestoreRestoring
	job.Progress = 0.5
	m.mu.Unlock()

	time.Sleep(200 * time.Millisecond)

	m.mu.Lock()
	job.Status = RestoreCompleted
	job.Progress = 1.0
	now := time.Now()
	job.CompletedAt = &now
	m.mu.Unlock()

	log.Printf("Restore completed: %s", job.ID)
}

// GetRestoreJob returns a restore job by ID
func (m *Manager) GetRestoreJob(jobID string) (*RestoreJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.restores[jobID]
	if !ok {
		return nil, fmt.Errorf("restore job not found: %s", jobID)
	}

	return job, nil
}

// GenerateKey generates a new encryption key
func (m *Manager) GenerateKey(name, algorithm string) (*BackupKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	algo := EncryptionAlgorithm(algorithm)
	if algo != AES256GCM && algo != ChaCha20 {
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	// Generate random key data
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate key: %v", err)
	}

	keyData := hex.EncodeToString(keyBytes)
	fingerprint := fmt.Sprintf("%x", sha256.Sum256([]byte(keyData)))[:16]

	now := time.Now()
	key := &BackupKey{
		ID:          fmt.Sprintf("key-%d", now.UnixNano()),
		Name:        name,
		Algorithm:   algo,
		KeyData:     keyData,
		CreatedAt:   now,
		Fingerprint: fingerprint,
		IsPrimary:   len(m.keys) == 0, // First key is primary
	}

	m.keys[key.ID] = key

	return key, nil
}

// ListKeys returns all keys
func (m *Manager) ListKeys() ([]BackupKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]BackupKey, 0, len(m.keys))
	for _, key := range m.keys {
		keys = append(keys, *key)
	}

	return keys, nil
}

// RevokeKey revokes an encryption key
func (m *Manager) RevokeKey(keyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, ok := m.keys[keyID]
	if !ok {
		return fmt.Errorf("key not found: %s", keyID)
	}

	if key.RevokedAt != nil {
		return fmt.Errorf("key already revoked: %s", keyID)
	}

	now := time.Now()
	key.RevokedAt = &now
	key.IsPrimary = false

	return nil
}

// CreateSchedule creates a backup schedule
func (m *Manager) CreateSchedule(schedule BackupSchedule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate key exists
	if _, ok := m.keys[schedule.EncryptionKeyID]; !ok {
		return fmt.Errorf("encryption key not found: %s", schedule.EncryptionKeyID)
	}

	if schedule.ID == "" {
		schedule.ID = fmt.Sprintf("schedule-%d", time.Now().UnixNano())
	}

	m.schedules[schedule.ID] = &schedule

	return nil
}

// RunIntegrityCheck runs an integrity check on a backup
func (m *Manager) RunIntegrityCheck(backupID string) (*IntegrityCheck, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	backup, ok := m.backups[backupID]
	if !ok {
		return nil, fmt.Errorf("backup not found: %s", backupID)
	}

	now := time.Now()
	check := &IntegrityCheck{
		ID:            fmt.Sprintf("check-%d", now.UnixNano()),
		BackupID:      backupID,
		Status:        IntegrityPassing,
		LastChecked:   now,
		ChecksumMatch: true,
		FilesChecked:  100, // Simulated
		FilesFailed:   0,
	}

	// Simulate verification
	expectedChecksum := m.generateChecksum(backupID)
	if backup.Checksum != expectedChecksum {
		check.Status = IntegrityFailing
		check.ChecksumMatch = false
		check.FilesFailed = 1
	}

	m.integrities[check.ID] = check

	return check, nil
}

// VerifyBackup verifies a backup can be restored
func (m *Manager) VerifyBackup(backupID, keyID string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	backup, ok := m.backups[backupID]
	if !ok {
		return false, fmt.Errorf("backup not found: %s", backupID)
	}

	key, ok := m.keys[keyID]
	if !ok {
		return false, fmt.Errorf("key not found: %s", keyID)
	}

	if key.RevokedAt != nil {
		return false, fmt.Errorf("key is revoked: %s", keyID)
	}

	if backup.Status != StatusCompleted {
		return false, nil
	}

	// Verify encryption algorithm matches
	if backup.EncryptionAlgo != key.Algorithm {
		return false, nil
	}

	return true, nil
}

// GetBackupStats returns backup statistics
func (m *Manager) GetBackupStats() (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_backups":     len(m.backups),
		"total_keys":        len(m.keys),
		"total_schedules":   len(m.schedules),
		"total_restores":    len(m.restores),
		"total_integrities": len(m.integrities),
	}

	// Count by status
	statusCounts := make(map[string]int)
	for _, backup := range m.backups {
		statusCounts[string(backup.Status)]++
	}
	stats["backups_by_status"] = statusCounts

	// Calculate total sizes
	var totalSize, totalEncryptedSize int64
	for _, backup := range m.backups {
		totalSize += backup.Size
		totalEncryptedSize += backup.EncryptedSize
	}
	stats["total_size"] = totalSize
	stats["total_encrypted_size"] = totalEncryptedSize

	// Active keys
	activeKeys := 0
	for _, key := range m.keys {
		if key.RevokedAt == nil {
			activeKeys++
		}
	}
	stats["active_keys"] = activeKeys

	return stats, nil
}

func (m *Manager) generateChecksum(input string) string {
	hash := sha256.Sum256([]byte(input + time.Now().String()))
	return hex.EncodeToString(hash[:])
}
