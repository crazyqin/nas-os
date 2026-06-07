package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditAction represents the type of action being audited
type AuditAction string

const (
	ActionSyncStart      AuditAction = "sync_start"
	ActionSyncComplete   AuditAction = "sync_complete"
	ActionSyncError      AuditAction = "sync_error"
	ActionFileUpload     AuditAction = "file_upload"
	ActionFileDownload   AuditAction = "file_download"
	ActionFileDelete     AuditAction = "file_delete"
	ActionConflict       AuditAction = "conflict_resolved"
	ActionVersionRestore AuditAction = "version_restore"
	ActionTaskCreate     AuditAction = "task_create"
	ActionTaskDelete     AuditAction = "task_delete"
	ActionAccessDenied   AuditAction = "access_denied"
)

// AuditEntry represents a single audit log entry
type AuditEntry struct {
	ID        string      `json:"id"`
	Timestamp time.Time   `json:"timestamp"`
	UserID    string      `json:"user_id"`
	Username  string      `json:"username"`
	Action    AuditAction `json:"action"`
	TaskID    string      `json:"task_id,omitempty"`
	FilePath  string      `json:"file_path,omitempty"`
	Details   string      `json:"details,omitempty"`
	SourceIP  string      `json:"source_ip,omitempty"`
	Status    string      `json:"status"` // success, denied, error
}

// AuditLogger handles audit logging for sync operations
type AuditLogger struct {
	mu      sync.Mutex
	logDir  string
	maxAge  time.Duration
	file    *os.File
	encoder *json.Encoder
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logDir string, maxAgeDays int) (*AuditLogger, error) {
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return nil, fmt.Errorf("create audit log dir: %w", err)
	}

	al := &AuditLogger{
		logDir: logDir,
		maxAge: time.Duration(maxAgeDays) * 24 * time.Hour,
	}

	if err := al.rotate(); err != nil {
		return nil, err
	}

	return al, nil
}

// Log records an audit entry
func (al *AuditLogger) Log(entry AuditEntry) error {
	al.mu.Lock()
	defer al.mu.Unlock()

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	entry.ID = fmt.Sprintf("%d", entry.Timestamp.UnixNano())

	return al.encoder.Encode(entry)
}

// Close closes the audit log file
func (al *AuditLogger) Close() error {
	al.mu.Lock()
	defer al.mu.Unlock()
	if al.file != nil {
		return al.file.Close()
	}
	return nil
}

// rotate opens a new log file for the current date
func (al *AuditLogger) rotate() error {
	if al.file != nil {
		al.file.Close()
	}

	filename := fmt.Sprintf("sync-audit-%s.jsonl", time.Now().Format("2006-01-02"))
	path := filepath.Join(al.logDir, filename)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}

	al.file = f
	al.encoder = json.NewEncoder(f)

	return nil
}

// PurgeOldLogs removes audit logs older than the retention period
func (al *AuditLogger) PurgeOldLogs() error {
	entries, err := os.ReadDir(al.logDir)
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-al.maxAge)
	var deleted int

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(al.logDir, entry.Name()))
			deleted++
		}
	}

	return nil
}
