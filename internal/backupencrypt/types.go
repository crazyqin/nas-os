package backupencrypt

import (
	"time"
)

// EncryptionAlgorithm represents the encryption algorithm type
type EncryptionAlgorithm string

const (
	AES256GCM EncryptionAlgorithm = "aes256gcm"
	ChaCha20  EncryptionAlgorithm = "chacha20"
)

// BackupStatus represents the backup status
type BackupStatus string

const (
	StatusPending    BackupStatus = "pending"
	StatusEncrypting BackupStatus = "encrypting"
	StatusUploading  BackupStatus = "uploading"
	StatusCompleted  BackupStatus = "completed"
	StatusFailed     BackupStatus = "failed"
)

// RestoreStatus represents the restore job status
type RestoreStatus string

const (
	RestorePending   RestoreStatus = "pending"
	RestoreRestoring RestoreStatus = "restoring"
	RestoreCompleted RestoreStatus = "completed"
	RestoreFailed    RestoreStatus = "failed"
)

// IntegrityStatus represents the integrity check status
type IntegrityStatus string

const (
	IntegrityPassing IntegrityStatus = "passing"
	IntegrityFailing IntegrityStatus = "failing"
)

// EncryptedBackup represents an encrypted backup
type EncryptedBackup struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	SourcePath       string              `json:"source_path"`
	DestPath         string              `json:"dest_path"`
	Status           BackupStatus        `json:"status"`
	EncryptionAlgo   EncryptionAlgorithm `json:"encryption_algo"`
	KeyID            string              `json:"key_id"`
	Size             int64               `json:"size"`
	EncryptedSize    int64               `json:"encrypted_size"`
	CompressionRatio float64             `json:"compression_ratio"`
	Checksum         string              `json:"checksum"`
	Progress         float64             `json:"progress"`
	CreatedAt        time.Time           `json:"created_at"`
	CompletedAt      *time.Time          `json:"completed_at,omitempty"`
}

// BackupKey represents an encryption key
type BackupKey struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Algorithm   EncryptionAlgorithm `json:"algorithm"`
	KeyData     string              `json:"key_data"`
	CreatedAt   time.Time           `json:"created_at"`
	ExpiresAt   *time.Time          `json:"expires_at,omitempty"`
	RevokedAt   *time.Time          `json:"revoked_at,omitempty"`
	Fingerprint string              `json:"fingerprint"`
	IsPrimary   bool                `json:"is_primary"`
}

// BackupSchedule represents a backup schedule
type BackupSchedule struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	CronExpr         string     `json:"cron_expr"`
	SourcePaths      []string   `json:"source_paths"`
	DestPath         string     `json:"dest_path"`
	Retention        int        `json:"retention"`
	EncryptionKeyID  string     `json:"encryption_key_id"`
	Incremental      bool       `json:"incremental"`
	CompressionLevel int        `json:"compression_level"`
	Enabled          bool       `json:"enabled"`
	LastRun          *time.Time `json:"last_run,omitempty"`
	NextRun          *time.Time `json:"next_run,omitempty"`
}

// RestoreJob represents a restore job
type RestoreJob struct {
	ID          string        `json:"id"`
	BackupID    string        `json:"backup_id"`
	DestPath    string        `json:"dest_path"`
	Status      RestoreStatus `json:"status"`
	Progress    float64       `json:"progress"`
	KeyID       string        `json:"key_id"`
	VerifyOnly  bool          `json:"verify_only"`
	CreatedAt   time.Time     `json:"created_at"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
}

// IntegrityCheck represents an integrity check result
type IntegrityCheck struct {
	ID            string          `json:"id"`
	BackupID      string          `json:"backup_id"`
	Status        IntegrityStatus `json:"status"`
	LastChecked   time.Time       `json:"last_checked"`
	ChecksumMatch bool            `json:"checksum_match"`
	FilesChecked  int             `json:"files_checked"`
	FilesFailed   int             `json:"files_failed"`
}

// BackupEncryptConfig represents the backup encryption configuration
type BackupEncryptConfig struct {
	DefaultAlgo       EncryptionAlgorithm `json:"default_algo"`
	KeyStorePath      string              `json:"key_store_path"`
	ChunkSize         int64               `json:"chunk_size"`
	MaxParallel       int                 `json:"max_parallel"`
	VerifyAfterBackup bool                `json:"verify_after_backup"`
	AutoKeyRotation   bool                `json:"auto_key_rotation"`
}
