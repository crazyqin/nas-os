package ransomware

import (
	"time"
)

// ========== 核心类型定义 ==========

// ThreatLevel 威胁等级.
type ThreatLevel string

const (
	ThreatLevelNone     ThreatLevel = "none"
	ThreatLevelLow      ThreatLevel = "low"
	ThreatLevelMedium   ThreatLevel = "medium"
	ThreatLevelHigh     ThreatLevel = "high"
	ThreatLevelCritical ThreatLevel = "critical"
)

// DetectionType 检测类型.
type DetectionType string

const (
	DetectionTypeSignature DetectionType = "signature" // 特征库匹配
	DetectionTypeBehavior  DetectionType = "behavior"  // 行为分析
	DetectionTypeHeuristic DetectionType = "heuristic" // 启发式检测
	DetectionTypeExtension DetectionType = "extension" // 扩展名检测
	DetectionTypePattern   DetectionType = "pattern"   // 文件模式检测
	DetectionTypeEntropy   DetectionType = "entropy"   // 熵值检测（加密文件）
)

// FileEvent 文件事件.
type FileEvent struct {
	ID           string                 `json:"id"`
	Timestamp    time.Time              `json:"timestamp"`
	Path         string                 `json:"path"`
	OldPath      string                 `json:"old_path,omitempty"`      // 重命名时使用
	Operation    FileOperation          `json:"operation"`               // 操作类型
	Size         int64                  `json:"size"`                    // 文件大小
	OldSize      int64                  `json:"old_size,omitempty"`      // 原大小
	Extension    string                 `json:"extension"`               // 文件扩展名
	OldExtension string                 `json:"old_extension,omitempty"` // 原扩展名
	ProcessName  string                 `json:"process_name,omitempty"`  // 触发进程
	ProcessPID   int                    `json:"process_pid,omitempty"`   // 进程PID
	UserID       string                 `json:"user_id,omitempty"`       // 操作用户
	Entropies    map[string]float64     `json:"entropies,omitempty"`     // 熵值信息
	Metadata     map[string]interface{} `json:"metadata,omitempty"`      // 其他元数据
}

// FileOperation 文件操作类型.
type FileOperation string

const (
	FileOpCreate   FileOperation = "create"
	FileOpModify   FileOperation = "modify"
	FileOpDelete   FileOperation = "delete"
	FileOpRename   FileOperation = "rename"
	FileOpMove     FileOperation = "move"
	FileOpWrite    FileOperation = "write"
	FileOpTruncate FileOperation = "truncate"
)

// RansomwareSignature 勒索软件特征.
type RansomwareSignature struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`          // 勒索软件名称
	Family       string      `json:"family"`        // 家族
	Extensions   []string    `json:"extensions"`    // 加密后扩展名
	Patterns     []string    `json:"patterns"`      // 文件内容特征码
	RansomNote   []string    `json:"ransom_note"`   // 勒索信文件名
	RegistryKeys []string    `json:"registry_keys"` // 注册表键（Windows）
	IOC          []string    `json:"ioc"`           // 威胁指标（IP、域名、URL等）
	FirstSeen    string      `json:"first_seen"`    // 首次发现时间
	LastUpdated  string      `json:"last_updated"`  // 最后更新时间
	Severity     ThreatLevel `json:"severity"`
	Description  string      `json:"description"`
	Reference    string      `json:"reference"` // 参考链接
}

// BehaviorPattern 行为模式.
type BehaviorPattern struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Conditions  []Condition `json:"conditions"`
	Weight      int         `json:"weight"`    // 权重
	Threshold   int         `json:"threshold"` // 触发阈值
	Severity    ThreatLevel `json:"severity"`
	Enabled     bool        `json:"enabled"`
}

// Condition 行为条件.
type Condition struct {
	Type       string      `json:"type"`                  // 条件类型
	Field      string      `json:"field"`                 // 字段名
	Operator   string      `json:"operator"`              // 操作符 (eq, ne, gt, lt, gte, lte, contains, matches, in)
	Value      interface{} `json:"value"`                 // 期望值
	TimeWindow int         `json:"time_window,omitempty"` // 时间窗口（秒）
	Count      int         `json:"count,omitempty"`       // 计数阈值
}

// DetectionResult 检测结果.
type DetectionResult struct {
	ID              string                 `json:"id"`
	Timestamp       time.Time              `json:"timestamp"`
	ThreatLevel     ThreatLevel            `json:"threat_level"`
	DetectionType   DetectionType          `json:"detection_type"`
	SignatureID     string                 `json:"signature_id,omitempty"`
	SignatureName   string                 `json:"signature_name,omitempty"`
	BehaviorID      string                 `json:"behavior_id,omitempty"`
	BehaviorName    string                 `json:"behavior_name,omitempty"`
	FilePath        string                 `json:"file_path"`
	FileCount       int                    `json:"file_count,omitempty"`
	Details         map[string]interface{} `json:"details"`
	Confidence      float64                `json:"confidence"` // 置信度 0-1
	AffectedFiles   []string               `json:"affected_files,omitempty"`
	SuggestedAction string                 `json:"suggested_action"`
	Quarantined     bool                   `json:"quarantined"`
	ProcessInfo     *ProcessInfo           `json:"process_info,omitempty"`
}

// ProcessInfo 进程信息.
type ProcessInfo struct {
	PID          int      `json:"pid"`
	Name         string   `json:"name"`
	Path         string   `json:"path"`
	CmdLine      string   `json:"cmdline"`
	User         string   `json:"user"`
	ParentPID    int      `json:"parent_pid"`
	OpenFiles    []string `json:"open_files,omitempty"`
	NetworkConns []string `json:"network_conns,omitempty"`
}

// QuarantineEntry 隔离条目.
type QuarantineEntry struct {
	ID             string                 `json:"id"`
	OriginalPath   string                 `json:"original_path"`
	QuarantinePath string                 `json:"quarantine_path"`
	FileSize       int64                  `json:"file_size"`
	FileHash       string                 `json:"file_hash"` // SHA256
	Timestamp      time.Time              `json:"timestamp"`
	Reason         string                 `json:"reason"`
	DetectionID    string                 `json:"detection_id"`
	ThreatLevel    ThreatLevel            `json:"threat_level"`
	SignatureName  string                 `json:"signature_name,omitempty"`
	Restored       bool                   `json:"restored"`
	RestoredAt     *time.Time             `json:"restored_at,omitempty"`
	RestoredBy     string                 `json:"restored_by,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// Alert 勒索软件告警.
type Alert struct {
	ID             string                 `json:"id"`
	Timestamp      time.Time              `json:"timestamp"`
	Severity       ThreatLevel            `json:"severity"`
	Type           string                 `json:"type"` // alert type
	Title          string                 `json:"title"`
	Message        string                 `json:"message"`
	DetectionID    string                 `json:"detection_id"`
	AffectedPath   string                 `json:"affected_path"`
	AffectedFiles  []string               `json:"affected_files"`
	ProcessInfo    *ProcessInfo           `json:"process_info,omitempty"`
	SignatureName  string                 `json:"signature_name,omitempty"`
	Confidence     float64                `json:"confidence"`
	ActionTaken    []string               `json:"action_taken"` // 已采取的行动
	Status         AlertStatus            `json:"status"`
	AcknowledgedBy string                 `json:"acknowledged_by,omitempty"`
	AcknowledgedAt *time.Time             `json:"acknowledged_at,omitempty"`
	ResolvedAt     *time.Time             `json:"resolved_at,omitempty"`
	Details        map[string]interface{} `json:"details,omitempty"`
}

// AlertStatus 告警状态.
type AlertStatus string

const (
	AlertStatusNew           AlertStatus = "new"
	AlertStatusAcknowledged  AlertStatus = "acknowledged"
	AlertStatusInvestigating AlertStatus = "investigating"
	AlertStatusResolved      AlertStatus = "resolved"
	AlertStatusFalsePositive AlertStatus = "false_positive"
)

// MonitorConfig 监控配置.
type MonitorConfig struct {
	Enabled             bool          `json:"enabled"`
	WatchPaths          []string      `json:"watch_paths"`          // 监控路径
	ExcludePaths        []string      `json:"exclude_paths"`        // 排除路径
	MaxFileSize         int64         `json:"max_file_size"`        // 最大文件大小（字节）
	EntropyThreshold    float64       `json:"entropy_threshold"`    // 熵值阈值
	EncryptionThreshold int           `json:"encryption_threshold"` // 加密文件数量阈值
	BehaviorWindow      time.Duration `json:"behavior_window"`      // 行为分析时间窗口
	MaxEvents           int           `json:"max_events"`           // 最大事件缓存数
	AlertCooldown       time.Duration `json:"alert_cooldown"`       // 告警冷却时间
}

// SignatureDBConfig 特征库配置.
type SignatureDBConfig struct {
	Enabled        bool          `json:"enabled"`
	AutoUpdate     bool          `json:"auto_update"`
	UpdateURL      string        `json:"update_url"`
	UpdateInterval time.Duration `json:"update_interval"`
	LastUpdated    time.Time     `json:"last_updated"`
}

// QuarantineConfig 隔离配置.
type QuarantineConfig struct {
	Enabled       bool          `json:"enabled"`
	QuarantineDir string        `json:"quarantine_dir"`
	MaxSize       int64         `json:"max_size"`    // 最大隔离区大小（字节）
	MaxAge        time.Duration `json:"max_age"`     // 文件最大保留时间
	AutoDelete    bool          `json:"auto_delete"` // 自动删除过期文件
}

// AlertConfig 告警配置.
type AlertConfig struct {
	Enabled        bool           `json:"enabled"`
	Channels       []AlertChannel `json:"channels"`        // 告警渠道
	CooldownPeriod time.Duration  `json:"cooldown_period"` // 冷却期
	MinSeverity    ThreatLevel    `json:"min_severity"`    // 最低告警级别
	MaxAlerts      int            `json:"max_alerts"`      // 最大告警数
}

// AlertChannel 告警渠道.
type AlertChannel struct {
	Type    string                 `json:"type"` // email, webhook, sms, push
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config"`
}

// Config 勒索软件检测模块配置.
type Config struct {
	Monitor     MonitorConfig     `json:"monitor"`
	SignatureDB SignatureDBConfig `json:"signature_db"`
	Quarantine  QuarantineConfig  `json:"quarantine"`
	Alert       AlertConfig       `json:"alert"`
}

// Statistics 统计信息.
type Statistics struct {
	TotalEvents      int64                   `json:"total_events"`
	TotalDetections  int64                   `json:"total_detections"`
	TotalAlerts      int64                   `json:"total_alerts"`
	TotalQuarantined int64                   `json:"total_quarantined"`
	QuarantineSize   int64                   `json:"quarantine_size"`
	ByThreatLevel    map[ThreatLevel]int64   `json:"by_threat_level"`
	ByDetectionType  map[DetectionType]int64 `json:"by_detection_type"`
	LastDetection    *time.Time              `json:"last_detection,omitempty"`
	Uptime           time.Duration           `json:"uptime"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() Config {
	return Config{
		Monitor: MonitorConfig{
			Enabled:             true,
			WatchPaths:          []string{"/mnt", "/data", "/shares"},
			ExcludePaths:        []string{"/proc", "/sys", "/dev", "/run"},
			MaxFileSize:         100 * 1024 * 1024, // 100MB
			EntropyThreshold:    7.5,
			EncryptionThreshold: 10,
			BehaviorWindow:      5 * time.Minute,
			MaxEvents:           10000,
			AlertCooldown:       5 * time.Minute,
		},
		SignatureDB: SignatureDBConfig{
			Enabled:        true,
			AutoUpdate:     true,
			UpdateInterval: 24 * time.Hour,
		},
		Quarantine: QuarantineConfig{
			Enabled:       true,
			QuarantineDir: "/var/lib/nas-os/quarantine",
			MaxSize:       10 * 1024 * 1024 * 1024, // 10GB
			MaxAge:        30 * 24 * time.Hour,     // 30天
			AutoDelete:    true,
		},
		Alert: AlertConfig{
			Enabled:        true,
			CooldownPeriod: 5 * time.Minute,
			MinSeverity:    ThreatLevelMedium,
			MaxAlerts:      1000,
		},
	}
}
