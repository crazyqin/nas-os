// Package immutablebackup provides write-once backup protection.
// Once a backup is sealed, it cannot be modified or deleted until the
// retention period expires. Inspired by TrueNAS Immutable Backup and
// enterprise data protection best practices.
package immutablebackup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// BackupState represents the lifecycle of an immutable backup.
type BackupState string

const (
	StateCreating  BackupState = "creating"
	StateSealed    BackupState = "sealed"
	StateExpired   BackupState = "expired"
	StateDestroyed BackupState = "destroyed"
)

// Retention defines how long a backup remains immutable.
type Retention struct {
	Duration  time.Duration `json:"duration"`   // How long the backup is protected
	ExpiresAt time.Time     `json:"expires_at"` // Calculated expiration
	CanExtend bool          `json:"can_extend"` // Whether retention can be extended
}

// Backup represents an immutable backup snapshot.
type Backup struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	State       BackupState `json:"state"`
	SizeBytes   int64       `json:"size_bytes"`
	SourcePath  string      `json:"source_path"`
	StoragePath string      `json:"storage_path"`
	Checksum    string      `json:"checksum"` // SHA-256 of backup data
	Retention   Retention   `json:"retention"`
	CreatedAt   time.Time   `json:"created_at"`
	SealedAt    time.Time   `json:"sealed_at"`
	AccessCount int64       `json:"access_count"`
	Tags        []string    `json:"tags,omitempty"`
}

// Manager handles immutable backup lifecycle.
type Manager struct {
	mu       sync.RWMutex
	backups  map[string]*Backup
	auditLog []AuditEntry
}

// AuditEntry records immutable operations for compliance.
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	BackupID  string    `json:"backup_id"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	Success   bool      `json:"success"`
}

// NewManager creates a new immutable backup manager.
func NewManager() *Manager {
	return &Manager{
		backups:  make(map[string]*Backup),
		auditLog: make([]AuditEntry, 0),
	}
}

// Create initializes a new backup in "creating" state.
func (m *Manager) Create(name, description, sourcePath, storagePath string, retentionDuration time.Duration, canExtend bool, tags []string) (*Backup, error) {
	if name == "" {
		return nil, fmt.Errorf("backup name is required")
	}
	if retentionDuration < time.Hour {
		return nil, fmt.Errorf("minimum retention is 1 hour")
	}

	id := generateID()
	now := time.Now()

	backup := &Backup{
		ID:          id,
		Name:        name,
		Description: description,
		State:       StateCreating,
		SourcePath:  sourcePath,
		StoragePath: storagePath,
		Retention: Retention{
			Duration:  retentionDuration,
			ExpiresAt: now.Add(retentionDuration),
			CanExtend: canExtend,
		},
		CreatedAt: now,
		Tags:      tags,
	}

	m.mu.Lock()
	m.backups[id] = backup
	m.addAudit(id, "create", fmt.Sprintf("Backup created: %s", name), true)
	m.mu.Unlock()

	return backup, nil
}

// Seal transitions a backup from "creating" to "sealed" (immutable).
// Once sealed, the backup data cannot be modified.
func (m *Manager) Seal(id, dataChecksum string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	backup, ok := m.backups[id]
	if !ok {
		return fmt.Errorf("backup %s not found", id)
	}
	if backup.State != StateCreating {
		m.addAudit(id, "seal_attempt", fmt.Sprintf("Failed: state is %s", backup.State), false)
		return fmt.Errorf("cannot seal backup in state %s", backup.State)
	}

	backup.State = StateSealed
	backup.SealedAt = time.Now()
	backup.Checksum = dataChecksum

	m.addAudit(id, "seal", fmt.Sprintf("Backup sealed with checksum %s", safePrefix(dataChecksum, 16)), true)
	return nil
}

// Verify checks backup integrity by comparing checksums.
func (m *Manager) Verify(id, currentChecksum string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	backup, ok := m.backups[id]
	if !ok {
		return false, fmt.Errorf("backup %s not found", id)
	}
	if backup.State != StateSealed {
		return false, fmt.Errorf("backup %s is not sealed (state: %s)", id, backup.State)
	}

	backup.AccessCount++
	match := backup.Checksum == currentChecksum

	m.addAudit(id, "verify", fmt.Sprintf("Checksum match: %v", match), match)
	return match, nil
}

// Delete attempts to delete a backup. Only possible after retention expires.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	backup, ok := m.backups[id]
	if !ok {
		return fmt.Errorf("backup %s not found", id)
	}

	// Cannot delete sealed backups before expiry
	if backup.State == StateSealed && time.Now().Before(backup.Retention.ExpiresAt) {
		m.addAudit(id, "delete_blocked",
			fmt.Sprintf("Retention active until %s", backup.Retention.ExpiresAt.Format(time.RFC3339)),
			false)
		return fmt.Errorf("backup %s is immutable until %s", id, backup.Retention.ExpiresAt.Format(time.RFC3339))
	}

	backup.State = StateDestroyed
	m.addAudit(id, "delete", "Backup destroyed", true)
	return nil
}

// ExtendRetention extends the immutability period of a sealed backup.
func (m *Manager) ExtendRetention(id string, additionalDuration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	backup, ok := m.backups[id]
	if !ok {
		return fmt.Errorf("backup %s not found", id)
	}
	if backup.State != StateSealed {
		return fmt.Errorf("backup %s is not sealed", id)
	}
	if !backup.Retention.CanExtend {
		return fmt.Errorf("backup %s retention cannot be extended", id)
	}

	oldExpiry := backup.Retention.ExpiresAt
	backup.Retention.ExpiresAt = backup.Retention.ExpiresAt.Add(additionalDuration)

	m.addAudit(id, "extend", fmt.Sprintf("Extended from %s to %s",
		oldExpiry.Format(time.RFC3339),
		backup.Retention.ExpiresAt.Format(time.RFC3339)), true)
	return nil
}

// Get retrieves a backup by ID.
func (m *Manager) Get(id string) (*Backup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	backup, ok := m.backups[id]
	if !ok {
		return nil, fmt.Errorf("backup %s not found", id)
	}
	backup.AccessCount++
	return backup, nil
}

// List returns all backups matching optional state filter.
func (m *Manager) List(stateFilter BackupState) []*Backup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Backup
	for _, b := range m.backups {
		if stateFilter == "" || b.State == stateFilter {
			result = append(result, b)
		}
	}
	return result
}

// Expire checks and marks backups whose retention has passed.
func (m *Manager) Expire() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	count := 0
	for _, b := range m.backups {
		if b.State == StateSealed && now.After(b.Retention.ExpiresAt) {
			b.State = StateExpired
			m.addAudit(b.ID, "expire", "Retention period ended", true)
			count++
		}
	}
	return count
}

// GetAuditLog returns the audit trail.
func (m *Manager) GetAuditLog() []AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]AuditEntry, len(m.auditLog))
	copy(result, m.auditLog)
	return result
}

func (m *Manager) addAudit(backupID, action, details string, success bool) {
	m.auditLog = append(m.auditLog, AuditEntry{
		Timestamp: time.Now(),
		BackupID:  backupID,
		Action:    action,
		Details:   details,
		Success:   success,
	})
}

// GenerateChecksum computes SHA-256 of data (helper for callers).
func GenerateChecksum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// safePrefix returns the first n characters of s, or s if shorter.
func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func generateID() string {
	b := make([]byte, 8)
	// Use time-based pseudo-random for simplicity
	for i := range b {
		b[i] = byte(time.Now().UnixNano() >> (8 * i))
	}
	return hex.EncodeToString(b)
}
