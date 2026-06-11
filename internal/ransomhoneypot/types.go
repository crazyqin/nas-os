// Package ransomhoneypot 提供 AI 驱动的勒索软件蜜罐检测功能
// 参考 TrueNAS 26 Ransomware Detection，通过蜜罐诱饵文件、
// 文件行为监控和 AI 模式识别来检测并阻止勒索软件攻击
package ransomhoneypot

import (
	"errors"
	"time"
)

// ============================================================
// 常量定义
// ============================================================

// 诱饵文件类型 — 模拟真实 NAS 文件结构
const (
	DecoyTypeDocument = "document" // 文档类 (.docx, .xlsx, .pdf)
	DecoyTypePhoto    = "photo"    // 照片类 (.jpg, .png, .raw)
	DecoyTypeVideo    = "video"    // 视频类 (.mp4, .mkv)
	DecoyTypeDatabase = "database" // 数据库类 (.sqlite, .mdb)
	DecoyTypeCode     = "code"     // 代码类 (.py, .go, .js)
	DecoyTypeBackup   = "backup"   // 备份类 (.bak, .tar.gz)
	DecoyTypeConfig   = "config"   // 配置文件 (.yaml, .conf, .env)
)

// 威胁级别
const (
	ThreatLevelNone     = 0 // 无威胁
	ThreatLevelLow      = 1 // 低威胁：单文件异常访问
	ThreatLevelMedium   = 2 // 中威胁：多文件异常修改
	ThreatLevelHigh     = 3 // 高威胁：批量加密/重命名行为
	ThreatLevelCritical = 4 // 严重：确认勒索软件活动
)

// 响应动作
const (
	ResponseActionAlert       = "alert"       // 仅告警
	ResponseActionIsolate     = "isolate"     // 隔离受影响文件
	ResponseActionLockShare   = "lock_share"  // 锁定共享目录
	ResponseActionBlockIP     = "block_ip"    // 封锁来源 IP
	ResponseActionKillProcess = "kill_process" // 终止可疑进程
	ResponseActionSnapshot    = "snapshot"    // 创建即时快照
)

// AI 检测权重因子
const (
	WeightEntropyChange   = 0.35 // 熵值变化权重
	WeightBatchRename     = 0.25 // 批量重命名权重
	WeightFileChangeRate  = 0.20 // 文件变更速率权重
	WeightDecoyTrigger    = 0.15 // 蜜罐触发权重
	WeightExtensionChange = 0.05 // 扩展名篡改权重
)

// 默认配置值
const (
	DefaultEntropyThreshold    = 7.0   // 默认加密检测熵值阈值
	DefaultDecoyCount          = 50    // 每个监控目录默认诱饵文件数
	DefaultMonitorIntervalSec  = 5     // 默认监控间隔（秒）
	DefaultMaxEvents           = 50000 // 最大事件保留数
	DefaultBatchThreshold      = 20    // 批量操作检测阈值（文件数/分钟）
	DefaultIsolationQuarantine = "/var/quarantine/ransomware"
)

// 错误定义
var (
	ErrDecoyNotFound    = errors.New("诱饵文件不存在")
	ErrDecoyExists      = errors.New("诱饵文件已存在")
	ErrInvalidPath      = errors.New("无效路径")
	ErrInvalidConfig    = errors.New("无效配置")
	ErrNotMonitoring    = errors.New("未启动监控")
	ErrAlreadyMonitoring = errors.New("监控已在运行中")
	ErrIsolationFailed  = errors.New("文件隔离失败")
	ErrPatternNotFound  = errors.New("检测模式不存在")
)

// ============================================================
// 数据结构定义
// ============================================================

// DecoyFile 诱饵文件描述
type DecoyFile struct {
	ID            string    `json:"id"`             // 诱饵ID
	Path          string    `json:"path"`           // 文件绝对路径
	Type          string    `json:"type"`           // 诱饵类型（document/photo/...）
	FileName      string    `json:"file_name"`      // 文件名
	FileSize      int64     `json:"file_size"`      // 文件大小（字节）
	ContentHash   string    `json:"content_hash"`   // SHA256 内容哈希
	Entropy       float64   `json:"entropy"`        // 初始熵值
	Enabled       bool      `json:"enabled"`        // 是否启用
	MonitorDir    string    `json:"monitor_dir"`    // 所属监控目录
	Tags          []string  `json:"tags"`           // 自定义标签
	CreatedAt     time.Time `json:"created_at"`     // 创建时间
	LastCheckedAt time.Time `json:"last_checked_at"` // 最后检查时间
	TriggerCount  int64     `json:"trigger_count"`  // 被触发次数
	Isolated      bool      `json:"isolated"`       // 是否已被隔离
}

// MonitorTarget 监控目标目录
type MonitorTarget struct {
	ID          string   `json:"id"`           // 目标ID
	Path        string   `json:"path"`         // 监控目录路径
	ShareName   string   `json:"share_name"`   // 共享名称
	DecoyCount  int      `json:"decoy_count"`  // 部署的诱饵数量
	Enabled     bool     `json:"enabled"`      // 是否启用
	Recursive   bool     `json:"recursive"`    // 是否递归监控子目录
	WatchTypes  []string `json:"watch_types"`  // 监控的诱饵类型列表
	CreatedAt   time.Time `json:"created_at"`  // 创建时间
}

// FileChangeEvent 文件变更事件
type FileChangeEvent struct {
	ID            string    `json:"id"`              // 事件ID
	Timestamp     time.Time `json:"timestamp"`       // 事件时间
	EventType     string    `json:"event_type"`      // 事件类型（modify/delete/rename/encrypt）
	FilePath      string    `json:"file_path"`       // 受影响文件路径
	OldPath       string    `json:"old_path,omitempty"` // 重命名前路径
	OldHash       string    `json:"old_hash"`        // 变更前哈希
	NewHash       string    `json:"new_hash"`        // 变更后哈希
	EntropyBefore float64   `json:"entropy_before"`  // 变更前熵值
	EntropyAfter  float64   `json:"entropy_after"`   // 变更后熵值
	FileSize      int64     `json:"file_size"`       // 文件大小
	SourceIP      string    `json:"source_ip"`       // 来源IP
	SourceUser    string    `json:"source_user"`     // 来源用户
	ProcessName   string    `json:"process_name"`    // 进程名
	ProcessPID    int       `json:"process_pid"`     // 进程PID
	IsDecoy       bool      `json:"is_decoy"`        // 是否命中诱饵
	DecoyID       string    `json:"decoy_id"`        // 命中的诱饵ID（如适用）
}

// ThreatDetection 威胁检测结果
type ThreatDetection struct {
	ID              string    `json:"id"`               // 检测ID
	Timestamp       time.Time `json:"timestamp"`        // 检测时间
	ThreatLevel     int       `json:"threat_level"`     // 威胁级别 (0-4)
	ConfidenceScore float64   `json:"confidence_score"` // AI 置信度分数 (0.0-1.0)
	Description     string    `json:"description"`      // 检测描述
	TriggeredEvents []string  `json:"triggered_events"` // 触发事件ID列表
	TriggeredDecoys []string  `json:"triggered_decoys"` // 触发的诱饵ID列表
	AffectedFiles   []string  `json:"affected_files"`   // 受影响文件列表
	SourceIP        string    `json:"source_ip"`        // 主要来源IP
	SourceUser      string    `json:"source_user"`      // 主要来源用户
	ResponseAction  string    `json:"response_action"`  // 采取的响应动作
	Isolated        bool      `json:"isolated"`         // 是否已隔离
	IsolationPath   string    `json:"isolation_path"`   // 隔离存储路径
	AutoResponded   bool      `json:"auto_responded"`   // 是否自动响应
	Details         string    `json:"details"`          // 详细分析说明
}

// AIAnalysisResult AI 行为分析结果
type AIAnalysisResult struct {
	EntropyScore      float64            `json:"entropy_score"`       // 熵值异常得分
	BatchRenameScore  float64            `json:"batch_rename_score"`  // 批量重命名得分
	FileChangeScore   float64            `json:"file_change_score"`   // 文件变更速率得分
	DecoyTriggerScore float64            `json:"decoy_trigger_score"` // 蜜罐触发得分
	ExtChangeScore    float64            `json:"ext_change_score"`    // 扩展名篡改得分
	OverallScore      float64            `json:"overall_score"`       // 综合得分 (0.0-1.0)
	ThreatLevel       int                `json:"threat_level"`        // 判定威胁级别
	PatternMatches    []PatternMatch     `json:"pattern_matches"`     // 匹配的威胁模式
	Indicators        []BehaviorIndicator `json:"indicators"`         // 行为指标列表
}

// PatternMatch 威胁模式匹配
type PatternMatch struct {
	PatternID   string  `json:"pattern_id"`   // 模式ID
	PatternName string  `json:"pattern_name"` // 模式名称
	MatchScore  float64 `json:"match_score"`  // 匹配分数
	Matched     bool    `json:"matched"`      // 是否匹配
}

// BehaviorIndicator 行为指标
type BehaviorIndicator struct {
	Name        string      `json:"name"`        // 指标名称
	Description string      `json:"description"` // 描述
	Value       interface{} `json:"value"`       // 观测值
	Threshold   interface{} `json:"threshold"`   // 阈值
	Exceeded    bool        `json:"exceeded"`    // 是否超过阈值
	Weight      float64     `json:"weight"`      // 权重
}

// ThreatPattern 威胁检测模式
type ThreatPattern struct {
	ID             string   `json:"id"`              // 模式ID
	Name           string   `json:"name"`            // 模式名称
	Description    string   `json:"description"`     // 描述
	Enabled        bool     `json:"enabled"`         // 是否启用
	MinEntropy     float64  `json:"min_entropy"`     // 最低熵值阈值
	BatchThreshold int      `json:"batch_threshold"` // 批量操作阈值
	TimeWindowSec  int      `json:"time_window_sec"` // 检测时间窗口（秒）
	SuspiciousExts []string `json:"suspicious_exts"` // 可疑扩展名
	Weight         float64  `json:"weight"`          // 综合权重
	Severity       int      `json:"severity"`        // 严重性级别
}

// HoneypotConfig 蜜罐系统配置
type HoneypotConfig struct {
	Enabled             bool     `json:"enabled"`               // 是否启用蜜罐系统
	DecoyCountPerDir    int      `json:"decoy_count_per_dir"`   // 每目录诱饵文件数
	MonitorIntervalSec  int      `json:"monitor_interval_sec"`  // 监控间隔（秒）
	EntropyThreshold    float64  `json:"entropy_threshold"`     // 熵值异常阈值
	BatchThreshold      int      `json:"batch_threshold"`       // 批量操作阈值
	MaxEvents           int      `json:"max_events"`            // 最大事件数
	AutoResponse        bool     `json:"auto_response"`         // 自动响应开关
	DefaultAction       string   `json:"default_action"`        // 默认响应动作
	QuarantinePath      string   `json:"quarantine_path"`       // 隔离存储路径
	ProtectedExtensions []string `json:"protected_extensions"`  // 受保护扩展名列表
	ExemptUsers         []string `json:"exempt_users"`          // 豁免用户
	ExemptIPs           []string `json:"exempt_ips"`            // 豁免IP
	BackupOnThreat      bool     `json:"backup_on_threat"`      // 威胁时自动备份
	AlertWebhook        string   `json:"alert_webhook"`         // 告警 Webhook URL
}

// DetectionStats 检测统计
type DetectionStats struct {
	TotalDecoys      int        `json:"total_decoys"`       // 诱饵文件总数
	ActiveDecoys     int        `json:"active_decoys"`      // 活跃诱饵数
	TriggeredDecoys  int        `json:"triggered_decoys"`   // 被触发的诱饵数
	TotalEvents      int64      `json:"total_events"`       // 总事件数
	ThreatsDetected  int64      `json:"threats_detected"`   // 检测到的威胁数
	ThreatsBlocked   int64      `json:"threats_blocked"`    // 拦截的威胁数
	FilesIsolated    int64      `json:"files_isolated"`     // 隔离的文件数
	LastDetection    *time.Time `json:"last_detection"`     // 最近检测时间
	UptimeSeconds    int64      `json:"uptime_seconds"`     // 运行时长（秒）
	TopThreatSources []ThreatSource `json:"top_threat_sources"` // 主要威胁来源
}

// ThreatSource 威胁来源统计
type ThreatSource struct {
	IP        string    `json:"ip"`         // 来源IP
	User      string    `json:"user"`       // 来源用户
	Count     int64     `json:"count"`      // 事件计数
	LastSeen  time.Time `json:"last_seen"`  // 最后出现时间
	MaxLevel  int       `json:"max_level"`  // 最高威胁级别
}
