// Package ransomware provides enhanced ransomware detection and protection
// 参考TrueNAS 26 Ransomware Defense特性设计
package ransomware

import (
	"time"
)

// ========== 威胁等级定义 ==========

// ThreatLevel 威胁等级.
type ThreatLevel string

const (
	ThreatLevelNone     ThreatLevel = "none"
	ThreatLevelLow      ThreatLevel = "low"
	ThreatLevelMedium   ThreatLevel = "medium"
	ThreatLevelHigh     ThreatLevel = "high"
	ThreatLevelCritical ThreatLevel = "critical"
)

// ThreatLevelValue 威胁等级数值.
var ThreatLevelValue = map[ThreatLevel]int{
	ThreatLevelNone:     0,
	ThreatLevelLow:      25,
	ThreatLevelMedium:   50,
	ThreatLevelHigh:     75,
	ThreatLevelCritical: 100,
}

// ========== 检测类型 ==========

// DetectionType 检测类型.
type DetectionType string

const (
	DetectionTypeSignature DetectionType = "signature" // 特征签名匹配
	DetectionTypeBehavior  DetectionType = "behavior"  // 行为模式分析
	DetectionTypeEntropy   DetectionType = "entropy"   // 熵值异常检测
	DetectionTypeHoneypot  DetectionType = "honeypot"  // 诱饵文件触发
	DetectionTypeExtension DetectionType = "extension" // 扩展名检测
	DetectionTypePattern   DetectionType = "pattern"   // 文件模式匹配
	DetectionTypeMulti     DetectionType = "multi"     // 多因子综合检测
)

// ========== 文件事件 ==========

// FileEvent 文件操作事件.
type FileEvent struct {
	ID           string                 `json:"id"`
	Timestamp    time.Time              `json:"timestamp"`
	Path         string                 `json:"path"`
	OldPath      string                 `json:"old_path,omitempty"`
	Operation    FileOperation          `json:"operation"`
	Size         int64                  `json:"size"`
	OldSize      int64                  `json:"old_size,omitempty"`
	Extension    string                 `json:"extension"`
	OldExtension string                 `json:"old_extension,omitempty"`
	Entropies    map[string]float64     `json:"entropies,omitempty"`
	ProcessName  string                 `json:"process_name,omitempty"`
	ProcessPID   int                    `json:"process_pid,omitempty"`
	UserID       string                 `json:"user_id,omitempty"`
	ClientIP     string                 `json:"client_ip,omitempty"`
	Protocol     string                 `json:"protocol,omitempty"` // SMB, NFS, WebDAV
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// FileOperation 文件操作类型.
type FileOperation string

const (
	FileOpCreate  FileOperation = "create"
	FileOpModify  FileOperation = "modify"
	FileOpDelete  FileOperation = "delete"
	FileOpRename  FileOperation = "rename"
	FileOpMove    FileOperation = "move"
	FileOpWrite   FileOperation = "write"
	FileOpRead    FileOperation = "read"
	FileOpEncrypt FileOperation = "encrypt"
	FileOpDecrypt FileOperation = "decrypt"
)

// ========== 检测结果 ==========

// DetectionResult 检测结果.
type DetectionResult struct {
	ID              string                 `json:"id"`
	Timestamp       time.Time              `json:"timestamp"`
	ThreatLevel     ThreatLevel            `json:"threat_level"`
	ThreatScore     int                    `json:"threat_score"` // 多因子综合评分 0-100
	DetectionType   DetectionType          `json:"detection_type"`
	DetectionTypes  []DetectionType        `json:"detection_types"` // 多因子时记录各类型
	SignatureID     string                 `json:"signature_id,omitempty"`
	SignatureName   string                 `json:"signature_name,omitempty"`
	BehaviorID      string                 `json:"behavior_id,omitempty"`
	BehaviorName    string                 `json:"behavior_name,omitempty"`
	HoneypotFileID  string                 `json:"honeypot_file_id,omitempty"`
	FilePath        string                 `json:"file_path"`
	FileCount       int                    `json:"file_count,omitempty"`
	AffectedFiles   []string               `json:"affected_files,omitempty"`
	Confidence      float64                `json:"confidence"` // 置信度 0-1
	EntropyValue    float64                `json:"entropy_value,omitempty"`
	ProcessInfo     *ProcessInfo           `json:"process_info,omitempty"`
	SuggestedAction string                 `json:"suggested_action"`
	AutoIsolated    bool                   `json:"auto_isolated"`
	Details         map[string]interface{} `json:"details"`

	// 多因子贡献（用于威胁评分）
	FactorScores FactorScores `json:"factor_scores"`
}

// FactorScores 各因子评分贡献.
type FactorScores struct {
	BehaviorScore  int `json:"behavior_score"`  // 行为因子 0-100
	EntropyScore   int `json:"entropy_score"`   // 熵值因子 0-100
	SignatureScore int `json:"signature_score"` // 签名因子 0-100
	ExtensionScore int `json:"extension_score"` // 扩展名因子 0-100
	HoneypotScore  int `json:"honeypot_score"`  // 诱饵因子 0-100
	TimestampScore int `json:"timestamp_score"` // 时间模式因子 0-100
	UserScore      int `json:"user_score"`      // 用户行为因子 0-100
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

// ========== 勒索软件签名 ==========

// RansomwareSignature 勒索软件特征签名.
type RansomwareSignature struct {
	ID              string      `json:"id"`
	Name            string      `json:"name"`
	Family          string      `json:"family"`
	Aliases         []string    `json:"aliases"`
	Extensions      []string    `json:"extensions"`        // 加密后扩展名
	RansomNoteFiles []string    `json:"ransom_note_files"` // 勒索信文件名
	Patterns        []string    `json:"patterns"`          // 内容特征码
	IOCs            []IOC       `json:"iocs"`              // 威胁指标
	FirstSeen       time.Time   `json:"first_seen"`
	LastUpdated     time.Time   `json:"last_updated"`
	Severity        ThreatLevel `json:"severity"`
	Description     string      `json:"description"`
	References      []string    `json:"references"`
}

// IOC 威胁指标.
type IOC struct {
	Type  string `json:"type"` // ip, domain, url, hash, email
	Value string `json:"value"`
}

// ========== 行为模式 ==========

// BehaviorPattern 行为模式定义.
type BehaviorPattern struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Conditions  []Condition `json:"conditions"`
	Weight      int         `json:"weight"`
	Threshold   int         `json:"threshold"`
	Severity    ThreatLevel `json:"severity"`
	Enabled     bool        `json:"enabled"`
}

// Condition 行为条件.
type Condition struct {
	Type       string      `json:"type"`
	Field      string      `json:"field"`
	Operator   string      `json:"operator"`
	Value      interface{} `json:"value"`
	TimeWindow int         `json:"time_window,omitempty"` // 秒
	Count      int         `json:"count,omitempty"`
}

// ========== 告警 ==========

// Alert 勒索软件告警.
type Alert struct {
	ID              string                 `json:"id"`
	Timestamp       time.Time              `json:"timestamp"`
	Severity        ThreatLevel            `json:"severity"`
	Type            string                 `json:"type"`
	Title           string                 `json:"title"`
	Message         string                 `json:"message"`
	DetectionID     string                 `json:"detection_id"`
	DetectionResult *DetectionResult       `json:"detection_result,omitempty"`
	AffectedPath    string                 `json:"affected_path"`
	AffectedFiles   []string               `json:"affected_files"`
	ProcessInfo     *ProcessInfo           `json:"process_info,omitempty"`
	AttackSource    *AttackSource          `json:"attack_source,omitempty"`
	Confidence      float64                `json:"confidence"`
	ActionsTaken    []string               `json:"actions_taken"`
	Recommendations []string               `json:"recommendations"`
	Status          AlertStatus            `json:"status"`
	AcknowledgedBy  string                 `json:"acknowledged_by,omitempty"`
	AcknowledgedAt  *time.Time             `json:"acknowledged_at,omitempty"`
	ResolvedAt      *time.Time             `json:"resolved_at,omitempty"`
	Details         map[string]interface{} `json:"details,omitempty"`
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

// AttackSource 攻击来源.
type AttackSource struct {
	Process   *ProcessInfo `json:"process,omitempty"`
	User      string       `json:"user,omitempty"`
	ClientIP  string       `json:"client_ip,omitempty"`
	Protocol  string       `json:"protocol,omitempty"` // SMB, NFS, WebDAV, FTP
	ShareName string       `json:"share_name,omitempty"`
	SessionID string       `json:"session_id,omitempty"`
}

// ========== 隔离 ==========

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
	ThreatScore    int                    `json:"threat_score"`
	SignatureName  string                 `json:"signature_name,omitempty"`
	Restored       bool                   `json:"restored"`
	RestoredAt     *time.Time             `json:"restored_at,omitempty"`
	RestoredBy     string                 `json:"restored_by,omitempty"`
	IsolatedBy     string                 `json:"isolated_by,omitempty"` // auto, manual
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// ========== SMB审计事件 ==========

// SMBAuditEvent SMB审计事件.
type SMBAuditEvent struct {
	EventID       string                 `json:"event_id"`
	Timestamp     time.Time              `json:"timestamp"`
	SessionID     string                 `json:"session_id"`
	ShareName     string                 `json:"share_name"`
	Username      string                 `json:"username,omitempty"`
	ClientIP      string                 `json:"client_ip"`
	ClientPort    int                    `json:"client_port,omitempty"`
	Operation     string                 `json:"operation"`
	FilePath      string                 `json:"file_path"`
	OldPath       string                 `json:"old_path,omitempty"`
	Status        string                 `json:"status"` // success, failure, denied
	BytesRead     int64                  `json:"bytes_read,omitempty"`
	BytesWritten  int64                  `json:"bytes_written,omitempty"`
	FileSize      int64                  `json:"file_size,omitempty"`
	IsDirectory   bool                   `json:"is_directory,omitempty"`
	RiskScore     int                    `json:"risk_score,omitempty"`
	Suspicious    bool                   `json:"suspicious"`
	SuspicionType []string               `json:"suspicion_type,omitempty"`
	Details       map[string]interface{} `json:"details,omitempty"`
}

// ========== 统计 ==========

// Statistics 模块统计.
type Statistics struct {
	TotalEvents       int64                   `json:"total_events"`
	TotalDetections   int64                   `json:"total_detections"`
	TotalAlerts       int64                   `json:"total_alerts"`
	TotalQuarantined  int64                   `json:"total_quarantined"`
	QuarantineSize    int64                   `json:"quarantine_size"`
	HoneypotTriggered int64                   `json:"honeypot_triggered"`
	ByThreatLevel     map[ThreatLevel]int64   `json:"by_threat_level"`
	ByDetectionType   map[DetectionType]int64 `json:"by_detection_type"`
	LastDetection     *time.Time              `json:"last_detection,omitempty"`
	LastAlert         *time.Time              `json:"last_alert,omitempty"`
	Uptime            time.Duration           `json:"uptime"`
}

// ========== 配置 ==========

// Config 勒索软件防护配置.
type Config struct {
	Enabled       bool                `json:"enabled"`
	Monitor       MonitorConfig       `json:"monitor"`
	Signature     SignatureConfig     `json:"signature"`
	Honeypot      HoneypotConfig      `json:"honeypot"`
	ThreatScoring ThreatScoringConfig `json:"threat_scoring"`
	Quarantine    QuarantineConfig    `json:"quarantine"`
	Isolation     IsolationConfig     `json:"isolation"`
	Alert         AlertConfig         `json:"alert"`
	SMBAudit      SMBAuditConfig      `json:"smb_audit"`
}

// MonitorConfig 监控配置.
type MonitorConfig struct {
	Enabled         bool          `json:"enabled"`
	WatchPaths      []string      `json:"watch_paths"`
	ExcludePaths    []string      `json:"exclude_paths"`
	MaxFileSize     int64         `json:"max_file_size"`
	BehaviorWindow  time.Duration `json:"behavior_window"`
	MaxEvents       int           `json:"max_events"`
	EventBufferSize int           `json:"event_buffer_size"`
}

// SignatureConfig 签名库配置.
type SignatureConfig struct {
	Enabled        bool          `json:"enabled"`
	AutoUpdate     bool          `json:"auto_update"`
	UpdateURL      string        `json:"update_url"`
	UpdateInterval time.Duration `json:"update_interval"`
	LastUpdated    time.Time     `json:"last_updated"`
}

// HoneypotConfig 诱饵文件配置.
type HoneypotConfig struct {
	Enabled        bool          `json:"enabled"`
	DeployPaths    []string      `json:"deploy_paths"`
	FilesPerPath   int           `json:"files_per_path"`
	FileTypes      []string      `json:"file_types"`
	NamePatterns   []string      `json:"name_patterns"`
	MinFileSize    int64         `json:"min_file_size"`
	MaxFileSize    int64         `json:"max_file_size"`
	CheckInterval  time.Duration `json:"check_interval"`
	AlertOnAccess  bool          `json:"alert_on_access"`
	AlertOnModify  bool          `json:"alert_on_modify"`
	AlertOnDelete  bool          `json:"alert_on_delete"`
	AlertOnRename  bool          `json:"alert_on_rename"`
	AutoRedeploy   bool          `json:"auto_redeploy"`
	ContentPattern string        `json:"content_pattern"` // random, structured, realistic
}

// ThreatScoringConfig 威胁评分配置（多因子检测）.
type ThreatScoringConfig struct {
	Enabled                bool    `json:"enabled"`
	BehaviorWeight         float64 `json:"behavior_weight"`          // 行为因子权重
	EntropyWeight          float64 `json:"entropy_weight"`           // 熵值因子权重
	SignatureWeight        float64 `json:"signature_weight"`         // 签名因子权重
	ExtensionWeight        float64 `json:"extension_weight"`         // 扩展名因子权重
	HoneypotWeight         float64 `json:"honeypot_weight"`          // 诱饵因子权重
	TimestampPatternWeight float64 `json:"timestamp_pattern_weight"` // 时间模式权重
	UserBehaviorWeight     float64 `json:"user_behavior_weight"`     // 用户行为权重

	// 阈值
	EntropyThreshold       float64 `json:"entropy_threshold"`      // 熵值阈值
	RapidChangeThreshold   int     `json:"rapid_change_threshold"` // 快速变更阈值
	RapidChangeWindow      int     `json:"rapid_change_window"`    // 快速变更时间窗口(秒)
	CriticalScoreThreshold int     `json:"critical_score_threshold"`
	HighScoreThreshold     int     `json:"high_score_threshold"`

	// 加成系数
	KEVBoost            float64 `json:"kev_boost"`             // KEV漏洞加成
	RansomwareBoost     float64 `json:"ransomware_boost"`      // 勒索软件关联加成
	MultipleFactorBoost float64 `json:"multiple_factor_boost"` // 多因子匹配加成
}

// QuarantineConfig 隔离配置.
type QuarantineConfig struct {
	Enabled       bool          `json:"enabled"`
	QuarantineDir string        `json:"quarantine_dir"`
	MaxSize       int64         `json:"max_size"`
	MaxAge        time.Duration `json:"max_age"`
	AutoDelete    bool          `json:"auto_delete"`
}

// IsolationConfig 自动隔离配置.
type IsolationConfig struct {
	Enabled              bool          `json:"enabled"`
	AutoIsolateThreshold int           `json:"auto_isolate_threshold"` // 自动隔离评分阈值
	AutoIsolateLevel     ThreatLevel   `json:"auto_isolate_level"`     // 自动隔离威胁等级
	IsolateProcess       bool          `json:"isolate_process"`        // 隔离可疑进程
	IsolateShare         bool          `json:"isolate_share"`          // 隔离受影响的共享
	IsolateUser          bool          `json:"isolate_user"`           // 禁用可疑用户
	EnableReadOnlyMode   bool          `json:"enable_read_only_mode"`  // 启用只读模式
	BlockNetworkAccess   bool          `json:"block_network_access"`   // 阻断网络访问
	CoolDownPeriod       time.Duration `json:"cool_down_period"`       // 冷却期
	MaxIsolationDuration time.Duration `json:"max_isolation_duration"` // 最大隔离时长
}

// AlertConfig 告警配置.
type AlertConfig struct {
	Enabled        bool           `json:"enabled"`
	Channels       []AlertChannel `json:"channels"`
	CooldownPeriod time.Duration  `json:"cooldown_period"`
	MinSeverity    ThreatLevel    `json:"min_severity"`
	MaxAlerts      int            `json:"max_alerts"`
}

// AlertChannel 告警渠道.
type AlertChannel struct {
	Type    string                 `json:"type"` // email, webhook, sms, push, discord
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config"`
}

// SMBAuditConfig SMB审计配置.
type SMBAuditConfig struct {
	Enabled                  bool     `json:"enabled"`
	AuditLevel               string   `json:"audit_level"` // minimal, standard, detailed, full
	LogPath                  string   `json:"log_path"`
	MaxLogAgeDays            int      `json:"max_log_age_days"`
	MaxLogSizeMB             int      `json:"max_log_size_mb"`
	LogFileRead              bool     `json:"log_file_read"`
	LogFileWrite             bool     `json:"log_file_write"`
	LogFileDelete            bool     `json:"log_file_delete"`
	LogFileRename            bool     `json:"log_file_rename"`
	SpotlightAudit           bool     `json:"spotlight_audit"`            // Spotlight搜索审计
	SpotlightLogQueries      bool     `json:"spotlight_log_queries"`      // 记录搜索查询
	SpotlightAlertSuspicious bool     `json:"spotlight_alert_suspicious"` // 搜索可疑内容告警
	ExcludeShares            []string `json:"exclude_shares"`
	ExcludeUsers             []string `json:"exclude_users"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() Config {
	return Config{
		Enabled: true,
		Monitor: MonitorConfig{
			Enabled:         true,
			WatchPaths:      []string{"/data", "/shares", "/mnt"},
			ExcludePaths:    []string{"/proc", "/sys", "/dev", "/run"},
			MaxFileSize:     100 * 1024 * 1024, // 100MB
			BehaviorWindow:  5 * time.Minute,
			MaxEvents:       10000,
			EventBufferSize: 1000,
		},
		Signature: SignatureConfig{
			Enabled:        true,
			AutoUpdate:     true,
			UpdateInterval: 24 * time.Hour,
		},
		Honeypot: HoneypotConfig{
			Enabled:        true,
			DeployPaths:    []string{"/data/shares", "/shares"},
			FilesPerPath:   5,
			FileTypes:      []string{".doc", ".docx", ".xls", ".xlsx", ".pdf", ".jpg", ".zip"},
			NamePatterns:   []string{"financial_report", "project_plan", "backup_data", "important"},
			MinFileSize:    1024,
			MaxFileSize:    102400,
			CheckInterval:  30 * time.Second,
			AlertOnModify:  true,
			AlertOnDelete:  true,
			AlertOnRename:  true,
			AutoRedeploy:   true,
			ContentPattern: "realistic",
		},
		ThreatScoring: ThreatScoringConfig{
			Enabled:                true,
			BehaviorWeight:         0.25,
			EntropyWeight:          0.20,
			SignatureWeight:        0.25,
			ExtensionWeight:        0.10,
			HoneypotWeight:         0.15,
			TimestampPatternWeight: 0.05,
			UserBehaviorWeight:     0.05,
			EntropyThreshold:       7.5,
			RapidChangeThreshold:   50,
			RapidChangeWindow:      60,
			CriticalScoreThreshold: 80,
			HighScoreThreshold:     60,
			KEVBoost:               1.2,
			RansomwareBoost:        1.1,
			MultipleFactorBoost:    1.15,
		},
		Quarantine: QuarantineConfig{
			Enabled:       true,
			QuarantineDir: "/var/lib/nas-os/quarantine",
			MaxSize:       10 * 1024 * 1024 * 1024, // 10GB
			MaxAge:        30 * 24 * time.Hour,
			AutoDelete:    true,
		},
		Isolation: IsolationConfig{
			Enabled:              true,
			AutoIsolateThreshold: 80,
			AutoIsolateLevel:     ThreatLevelCritical,
			IsolateProcess:       true,
			IsolateShare:         true,
			EnableReadOnlyMode:   true,
			CoolDownPeriod:       5 * time.Minute,
			MaxIsolationDuration: 24 * time.Hour,
		},
		Alert: AlertConfig{
			Enabled:        true,
			CooldownPeriod: 5 * time.Minute,
			MinSeverity:    ThreatLevelMedium,
			MaxAlerts:      1000,
		},
		SMBAudit: SMBAuditConfig{
			Enabled:                  true,
			AuditLevel:               "standard",
			LogPath:                  "/var/log/nas-os/audit/smb",
			MaxLogAgeDays:            90,
			MaxLogSizeMB:             100,
			LogFileRead:              true,
			LogFileWrite:             true,
			LogFileDelete:            true,
			LogFileRename:            true,
			SpotlightAudit:           true,
			SpotlightLogQueries:      true,
			SpotlightAlertSuspicious: true,
			ExcludeShares:            []string{"IPC$", "print$"},
		},
	}
}
