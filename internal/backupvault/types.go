// Package backupvault 提供备份保险库功能，支持多地备份策略、备份去重加密、恢复演练、RTO/RPO 管理。
package backupvault

import "time"

// VaultStatus 保险库状态
type VaultStatus string

const (
	VaultStatusActive   VaultStatus = "active"
	VaultStatusInactive VaultStatus = "inactive"
	VaultStatusLocked   VaultStatus = "locked"
	VaultStatusError    VaultStatus = "error"
)

// JobStatus 任务状态
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// BackupType 备份类型
type BackupType string

const (
	BackupTypeFull    BackupType = "full"    // 全量备份
	BackupTypeIncremental BackupType = "incremental" // 增量备份
	BackupTypeDifferential BackupType = "differential" // 差异备份
)

// EncryptionType 加密类型
type EncryptionType string

const (
	EncryptionAES256 EncryptionType = "aes256"
	EncryptionChaCha20 EncryptionType = "chacha20"
	EncryptionNone   EncryptionType = "none"
)

// SLALevel SLA 级别
type SLALevel string

const (
	SLALevelBronze SLALevel = "bronze" // 铜级 - 基础
	SLALevelSilver SLALevel = "silver" // 银级 - 标准
	SLALevelGold   SLALevel = "gold"   // 金级 - 高级
	SLALevelPlatinum SLALevel = "platinum" // 铂级 - 顶级
)

// TestStatus 演练状态
type TestStatus string

const (
	TestStatusPending  TestStatus = "pending"
	TestStatusRunning  TestStatus = "running"
	TestStatusSuccess  TestStatus = "success"
	TestStatusFailed   TestStatus = "failed"
)

// Vault 备份保险库
type Vault struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Description   string      `json:"description,omitempty"`
	Status        VaultStatus `json:"status"`
	Location      string      `json:"location"`
	RemoteURL     string      `json:"remote_url,omitempty"`
	CapacityBytes int64       `json:"capacity_bytes"`
	UsedBytes     int64       `json:"used_bytes"`
	Encryption    EncryptionType `json:"encryption"`
	RetentionDays int         `json:"retention_days"`
	IsPrimary     bool        `json:"is_primary"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// BackupJob 备份任务
type BackupJob struct {
	ID             string      `json:"id"`
	VaultID        string      `json:"vault_id"`
	Name           string      `json:"name"`
	SourcePath     string      `json:"source_path"`
	BackupType     BackupType  `json:"backup_type"`
	Status         JobStatus   `json:"status"`
	TotalBytes     int64       `json:"total_bytes"`
	ProcessedBytes int64       `json:"processed_bytes"`
	CompressedBytes int64      `json:"compressed_bytes"`
	DeduplicatedBytes int64    `json:"deduplicated_bytes"`
	SpeedMBps      float64     `json:"speed_mbps"`
	ErrorMessage   string      `json:"error_message,omitempty"`
	ScheduledAt    *time.Time  `json:"scheduled_at,omitempty"`
	StartedAt      *time.Time  `json:"started_at,omitempty"`
	CompletedAt    *time.Time  `json:"completed_at,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

// DedupStats 去重统计
type DedupStats struct {
	ID                string    `json:"id"`
	VaultID           string    `json:"vault_id"`
	TotalChunks       int64     `json:"total_chunks"`
	UniqueChunks      int64     `json:"unique_chunks"`
	DuplicateChunks   int64     `json:"duplicate_chunks"`
	TotalBytes        int64     `json:"total_bytes"`
	DeduplicatedBytes int64     `json:"deduplicated_bytes"`
	SavedBytes        int64     `json:"saved_bytes"`
	DedupRatio        float64   `json:"dedup_ratio"`
	SpaceSavings      float64   `json:"space_savings"` // 百分比
	CompressionRatio  float64   `json:"compression_ratio"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// RestoreTest 恢复演练
type RestoreTest struct {
	ID            string      `json:"id"`
	VaultID       string      `json:"vault_id"`
	JobID         string      `json:"job_id"`
	Name          string      `json:"name"`
	Status        TestStatus  `json:"status"`
	TargetPath    string      `json:"target_path"`
	TotalBytes    int64       `json:"total_bytes"`
	RestoredBytes int64       `json:"restored_bytes"`
	RTOActual     int         `json:"rto_actual"`     // 实际恢复时间（分钟）
	RTOTarget     int         `json:"rto_target"`     // 目标恢复时间（分钟）
	RPOActual     int         `json:"rpo_actual"`     // 实际数据丢失（分钟）
	RPOTarget     int         `json:"rpo_target"`     // 目标数据丢失（分钟）
	IsSuccessful  bool        `json:"is_successful"`
	ErrorMessage  string      `json:"error_message,omitempty"`
	VerifiedFiles int         `json:"verified_files"`
	CorruptFiles  int         `json:"corrupt_files"`
	StartedAt     *time.Time  `json:"started_at,omitempty"`
	CompletedAt   *time.Time  `json:"completed_at,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
}

// SLAPolicy SLA 策略
type SLAPolicy struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Description      string   `json:"description,omitempty"`
	Level            SLALevel `json:"level"`
	RTOTarget        int      `json:"rto_target"`        // 目标恢复时间（分钟）
	RPOTarget        int      `json:"rpo_target"`        // 目标数据丢失（分钟）
	BackupFrequency  string   `json:"backup_frequency"`  // hourly, daily, weekly
	RetentionDays    int      `json:"retention_days"`
	MinCopies        int      `json:"min_copies"`        // 最少副本数
	GeoRedundancy    bool     `json:"geo_redundancy"`    // 地理冗余
	EncryptionRequired bool   `json:"encryption_required"`
	TestFrequency    string   `json:"test_frequency"`    // monthly, quarterly, yearly
	IsActive         bool     `json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// BackupVaultConfig 备份保险库配置
type BackupVaultConfig struct {
	Enabled           bool            `json:"enabled"`
	DefaultEncryption EncryptionType  `json:"default_encryption"`
	DefaultRetention  int             `json:"default_retention"`  // 天
	MaxConcurrentJobs int             `json:"max_concurrent_jobs"`
	ChunkSizeKB       int             `json:"chunk_size_kb"`
	DedupEnabled      bool            `json:"dedup_enabled"`
	CompressionEnabled bool           `json:"compression_enabled"`
	DefaultSLALevel   SLALevel        `json:"default_sla_level"`
	AlertOnFailure    bool            `json:"alert_on_failure"`
}

// DefaultBackupVaultConfig 默认配置
func DefaultBackupVaultConfig() *BackupVaultConfig {
	return &BackupVaultConfig{
		Enabled:           true,
		DefaultEncryption: EncryptionAES256,
		DefaultRetention:  30,
		MaxConcurrentJobs: 3,
		ChunkSizeKB:       64,
		DedupEnabled:      true,
		CompressionEnabled: true,
		DefaultSLALevel:   SLALevelSilver,
		AlertOnFailure:    true,
	}
}
