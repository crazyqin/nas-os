// Package fileintegrity 提供文件完整性监控功能，支持实时监控、基线管理和变更检测。
// 对标 FIM（File Integrity Monitoring）标准，为 NAS 系统提供文件安全防护。
package fileintegrity

import "time"

// HashAlgorithm 哈希算法
type HashAlgorithm string

const (
	HashSHA256 HashAlgorithm = "sha256"
	HashSHA512 HashAlgorithm = "sha512"
	HashBLAKE3 HashAlgorithm = "blake3"
)

// ChangeType 变更类型
type ChangeType string

const (
	ChangeCreated    ChangeType = "created"
	ChangeModified   ChangeType = "modified"
	ChangeDeleted    ChangeType = "deleted"
	ChangePermission ChangeType = "permission"
	ChangeOwnership  ChangeType = "ownership"
	ChangeRenamed    ChangeType = "renamed"
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertInfo     AlertLevel = "info"
	AlertWarning  AlertLevel = "warning"
	AlertCritical AlertLevel = "critical"
)

// ScanMode 扫描模式
type ScanMode string

const (
	ScanModeFull        ScanMode = "full"
	ScanModeIncremental ScanMode = "incremental"
)

// AlertChannel 告警通道
type AlertChannel string

const (
	AlertChannelWebhook AlertChannel = "webhook"
	AlertChannelEmail   AlertChannel = "email"
	AlertChannelNotify  AlertChannel = "notify"
)

// MonitorStatus 监控状态
type MonitorStatus string

const (
	MonitorStatusIdle     MonitorStatus = "idle"
	MonitorStatusScanning MonitorStatus = "scanning"
	MonitorStatusWatching MonitorStatus = "watching"
	MonitorStatusError    MonitorStatus = "error"
)

// FileEntry 基线中的文件条目
type FileEntry struct {
	Path          string        `json:"path"`
	Hash          string        `json:"hash"`
	HashAlgorithm HashAlgorithm `json:"hash_algorithm"`
	Size          int64         `json:"size"`
	ModTime       time.Time     `json:"mod_time"`
	Mode          uint32        `json:"mode"`
	UID           uint32        `json:"uid"`
	GID           uint32        `json:"gid"`
	IsDir         bool          `json:"is_dir"`
	ScannedAt     time.Time     `json:"scanned_at"`
}

// Baseline 完整性基线
type Baseline struct {
	ID            string                `json:"id"`
	Name          string                `json:"name"`
	Description   string                `json:"description"`
	Entries       map[string]*FileEntry `json:"entries"`
	HashAlgorithm HashAlgorithm         `json:"hash_algorithm"`
	FileCount     int                   `json:"file_count"`
	TotalSize     int64                 `json:"total_size"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
	Metadata      map[string]string     `json:"metadata,omitempty"`
}

// FileChange 文件变更事件
type FileChange struct {
	ID           string     `json:"id"`
	Path         string     `json:"path"`
	ChangeType   ChangeType `json:"change_type"`
	BaselineHash string     `json:"baseline_hash,omitempty"`
	CurrentHash  string     `json:"current_hash,omitempty"`
	OldMode      uint32     `json:"old_mode,omitempty"`
	NewMode      uint32     `json:"new_mode,omitempty"`
	OldUID       uint32     `json:"old_uid,omitempty"`
	NewUID       uint32     `json:"new_uid,omitempty"`
	OldGID       uint32     `json:"old_gid,omitempty"`
	NewGID       uint32     `json:"new_gid,omitempty"`
	DetectedAt   time.Time  `json:"detected_at"`
	RuleID       string     `json:"rule_id,omitempty"`
	AlertLevel   AlertLevel `json:"alert_level"`
	Acknowledged bool       `json:"acknowledged"`
	Notes        string     `json:"notes,omitempty"`
}

// MonitorRule 监控规则
type MonitorRule struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	Enabled         bool           `json:"enabled"`
	Paths           []string       `json:"paths"`
	ExcludePaths    []string       `json:"exclude_paths"`
	ExcludePatterns []string       `json:"exclude_patterns"`
	MaxDepth        int            `json:"max_depth"`
	HashAlgorithm   HashAlgorithm  `json:"hash_algorithm"`
	AlertLevel      AlertLevel     `json:"alert_level"`
	AlertChannels   []AlertChannel `json:"alert_channels"`
	WebhookURL      string         `json:"webhook_url,omitempty"`
	EmailTo         string         `json:"email_to,omitempty"`
	WatchPermission bool           `json:"watch_permission"`
	WatchOwnership  bool           `json:"watch_ownership"`
	Schedule        string         `json:"schedule,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// Alert 告警信息
type Alert struct {
	ID        string       `json:"id"`
	RuleID    string       `json:"rule_id"`
	RuleName  string       `json:"rule_name"`
	Change    *FileChange  `json:"change"`
	Level     AlertLevel   `json:"level"`
	Channel   AlertChannel `json:"channel"`
	Message   string       `json:"message"`
	SentAt    time.Time    `json:"sent_at"`
	Delivered bool         `json:"delivered"`
	Error     string       `json:"error,omitempty"`
}

// ScanRequest 扫描请求
type ScanRequest struct {
	RuleIDs     []string `json:"rule_ids,omitempty"`
	Mode        ScanMode `json:"mode"`
	Paths       []string `json:"paths,omitempty"`
	ForceRehash bool     `json:"force_rehash"`
}

// ScanResult 扫描结果
type ScanResult struct {
	ID           string        `json:"id"`
	Mode         ScanMode      `json:"mode"`
	RuleIDs      []string      `json:"rule_ids"`
	FilesScanned int           `json:"files_scanned"`
	ChangesFound int           `json:"changes_found"`
	Changes      []*FileChange `json:"changes,omitempty"`
	Errors       []string      `json:"errors,omitempty"`
	StartedAt    time.Time     `json:"started_at"`
	FinishedAt   time.Time     `json:"finished_at"`
	Duration     time.Duration `json:"duration"`
}

// IntegrityReport 完整性校验报告
type IntegrityReport struct {
	ID                string        `json:"id"`
	BaselineID        string        `json:"baseline_id"`
	BaselineName      string        `json:"baseline_name"`
	TotalFiles        int           `json:"total_files"`
	VerifiedFiles     int           `json:"verified_files"`
	ModifiedFiles     int           `json:"modified_files"`
	NewFiles          int           `json:"new_files"`
	DeletedFiles      int           `json:"deleted_files"`
	PermissionChanges int           `json:"permission_changes"`
	IntegrityScore    float64       `json:"integrity_score"`
	Changes           []*FileChange `json:"changes,omitempty"`
	GeneratedAt       time.Time     `json:"generated_at"`
	Duration          time.Duration `json:"duration"`
}

// AuditLogEntry 审计日志条目
type AuditLogEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Details   string    `json:"details"`
	UserID    string    `json:"user_id,omitempty"`
	Source    string    `json:"source"`
}

// RepairSuggestion 修复建议
type RepairSuggestion struct {
	ID          string     `json:"id"`
	ChangeID    string     `json:"change_id"`
	Path        string     `json:"path"`
	ChangeType  ChangeType `json:"change_type"`
	Suggestion  string     `json:"suggestion"`
	Action      string     `json:"action"`
	Risk        AlertLevel `json:"risk"`
	Automated   bool       `json:"automated"`
	Commands    []string   `json:"commands,omitempty"`
	RestoreHash string     `json:"restore_hash,omitempty"`
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	Enabled          bool          `json:"enabled"`
	DefaultAlgorithm HashAlgorithm `json:"default_algorithm"`
	MaxFileSize      int64         `json:"max_file_size"`
	ScanInterval     time.Duration `json:"scan_interval"`
	BaselineDir      string        `json:"baseline_dir"`
	AlertBufferSize  int           `json:"alert_buffer_size"`
	WorkerCount      int           `json:"worker_count"`
	RealTimeWatch    bool          `json:"real_time_watch"`
	LogRetentionDays int           `json:"log_retention_days"`
}

// DefaultMonitorConfig 默认监控配置
func DefaultMonitorConfig() *MonitorConfig {
	return &MonitorConfig{
		Enabled:          true,
		DefaultAlgorithm: HashSHA256,
		MaxFileSize:      100 * 1024 * 1024, // 100MB
		ScanInterval:     time.Hour,
		BaselineDir:      "/var/lib/nas-os/fim/baselines",
		AlertBufferSize:  1000,
		WorkerCount:      4,
		RealTimeWatch:    true,
		LogRetentionDays: 90,
	}
}

// ListBaselinesRequest 基线列表请求
type ListBaselinesRequest struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// ListChangesRequest 变更列表请求
type ListChangesRequest struct {
	RuleID   string     `json:"rule_id,omitempty"`
	Level    AlertLevel `json:"level,omitempty"`
	Since    *time.Time `json:"since,omitempty"`
	Until    *time.Time `json:"until,omitempty"`
	Acked    *bool      `json:"acked,omitempty"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}

// ExportAuditLogRequest 审计日志导出请求
type ExportAuditLogRequest struct {
	Since  *time.Time `json:"since,omitempty"`
	Until  *time.Time `json:"until,omitempty"`
	Format string     `json:"format"` // json, csv
	Action string     `json:"action,omitempty"`
}

// PaginatedResult 分页结果
type PaginatedResult struct {
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Items    interface{} `json:"items"`
}
