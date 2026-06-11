// Package ransombehaviorai 提供基于 AI 的勒索软件行为检测引擎
// 对标 TrueNAS 26 勒索软件检测功能，实现行为评分、置信度计算与自动响应
package ransombehaviorai

import "time"

// ThreatLevel 威胁级别
type ThreatLevel int

const (
	// ThreatLevelNone 无威胁
	ThreatLevelNone ThreatLevel = 0
	// ThreatLevelLow 低威胁
	ThreatLevelLow ThreatLevel = 1
	// ThreatLevelMedium 中等威胁
	ThreatLevelMedium ThreatLevel = 2
	// ThreatLevelHigh 高威胁
	ThreatLevelHigh ThreatLevel = 3
	// ThreatLevelCritical 严重威胁
	ThreatLevelCritical ThreatLevel = 4
)

// String 返回威胁级别的文字描述
func (t ThreatLevel) String() string {
	switch t {
	case ThreatLevelNone:
		return "none"
	case ThreatLevelLow:
		return "low"
	case ThreatLevelMedium:
		return "medium"
	case ThreatLevelHigh:
		return "high"
	case ThreatLevelCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// ResponseAction 自动响应动作
type ResponseAction string

const (
	// ActionAlert 仅告警
	ActionAlert ResponseAction = "alert"
	// ActionQuarantine 隔离进程/文件
	ActionQuarantine ResponseAction = "quarantine"
	// ActionSnapshot 立即创建快照保护
	ActionSnapshot ResponseAction = "snapshot"
	// ActionIsolate 网络隔离
	ActionIsolate ResponseAction = "isolate"
	// ActionLockdown 锁定卷
	ActionLockdown ResponseAction = "lockdown"
)

// FileEventType 文件事件类型
type FileEventType string

const (
	FileEventCreate      FileEventType = "create"
	FileEventModify      FileEventType = "modify"
	FileEventDelete      FileEventType = "delete"
	FileEventRename      FileEventType = "rename"
	FileEventBulkWrite   FileEventType = "bulk_write"
	FileEventEncrypt     FileEventType = "encrypt"
	FileEventExtensionChg FileEventType = "extension_change"
)

// IOEventType IO 事件类型
type IOEventType string

const (
	IOEventBurstWrite IOEventType = "burst_write"
	IOEventAnomalousRW IOEventType = "anomalous_rw_ratio"
	IOEventHighThroughput IOEventType = "high_throughput"
)

// ProcessEventType 进程事件类型
type ProcessEventType string

const (
	ProcessEventSuspicious ProcessEventType = "suspicious_process"
	ProcessEventPrivEsc    ProcessEventType = "privilege_escalation"
	ProcessEventAnomalous  ProcessEventType = "anomalous_behavior"
)

// ============================================================
// Configuration
// ============================================================

// Config AI 行为检测引擎总配置
type Config struct {
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
	// FileMonitor 文件行为监控配置
	FileMonitor FileMonitorConfig `json:"fileMonitor"`
	// IOMonitor IO 模式监控配置
	IOMonitor IOMonitorConfig `json:"ioMonitor"`
	// ProcessMonitor 进程行为监控配置
	ProcessMonitor ProcessMonitorConfig `json:"processMonitor"`
	// AIModel AI 模型推理配置
	AIModel AIModelConfig `json:"aiModel"`
	// AutoResponse 自动响应配置
	AutoResponse AutoResponseConfig `json:"autoResponse"`
	// WatchPaths 监控路径
	WatchPaths []string `json:"watchPaths"`
	// ExcludePaths 排除路径
	ExcludePaths []string `json:"excludePaths"`
	// NotifyChannels 通知渠道
	NotifyChannels []string `json:"notifyChannels"`
}

// FileMonitorConfig 文件行为监控配置
type FileMonitorConfig struct {
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
	// BulkWriteThreshold 批量写入阈值（文件数/秒）
	BulkWriteThreshold int `json:"bulkWriteThreshold"`
	// BulkRenameThreshold 批量重命名阈值（文件数/秒）
	BulkRenameThreshold int `json:"bulkRenameThreshold"`
	// EntropyThreshold 熵值阈值（>7.5 视为加密）
	EntropyThreshold float64 `json:"entropyThreshold"`
	// SuspiciousExtensions 可疑扩展名列表
	SuspiciousExtensions []string `json:"suspiciousExtensions"`
	// TimeWindowSec 滑动时间窗口（秒）
	TimeWindowSec int `json:"timeWindowSec"`
}

// IOMonitorConfig IO 模式监控配置
type IOMonitorConfig struct {
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
	// BurstWriteThresholdBps 突发写入带宽阈值（字节/秒）
	BurstWriteThresholdBps int64 `json:"burstWriteThresholdBps"`
	// AnomalousRWRatio 异常读写比阈值（写入/读取 > 此值触发）
	AnomalousRWRatio float64 `json:"anomalousRWRatio"`
	// SampleIntervalSec 采样间隔（秒）
	SampleIntervalSec int `json:"sampleIntervalSec"`
	// WindowSize 样本窗口大小
	WindowSize int `json:"windowSize"`
}

// ProcessMonitorConfig 进程行为监控配置
type ProcessMonitorConfig struct {
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
	// SuspiciousProcessNames 可疑进程名列表
	SuspiciousProcessNames []string `json:"suspiciousProcessNames"`
	// PrivEscThreshold 权限提升检测阈值
	PrivEscThreshold int `json:"privEscThreshold"`
	// MaxFileOpenPerProcess 每进程最大打开文件数
	MaxFileOpenPerProcess int `json:"maxFileOpenPerProcess"`
}

// AIModelConfig AI 模型推理配置
type AIModelConfig struct {
	// ScoreThreshold 行为评分告警阈值（0-100）
	ScoreThreshold int `json:"scoreThreshold"`
	// ConfidenceThreshold 置信度阈值（0-100）
	ConfidenceThreshold int `json:"confidenceThreshold"`
	// WeightFile 文件行为权重
	WeightFile float64 `json:"weightFile"`
	// WeightIO IO 行为权重
	WeightIO float64 `json:"weightIO"`
	// WeightProcess 进程行为权重
	WeightProcess float64 `json:"weightProcess"`
}

// AutoResponseConfig 自动响应配置
type AutoResponseConfig struct {
	// Enabled 是否启用自动响应
	Enabled bool `json:"enabled"`
	// ThresholdLevel 触发自动响应的最低威胁级别
	ThresholdLevel ThreatLevel `json:"thresholdLevel"`
	// DefaultAction 默认响应动作
	DefaultAction ResponseAction `json:"defaultAction"`
	// SnapshotOnThreat 威胁时自动创建快照
	SnapshotOnThreat bool `json:"snapshotOnThreat"`
	// IsolateOnCritical 严重威胁时网络隔离
	IsolateOnCritical bool `json:"isolateOnCritical"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled: true,
		FileMonitor: FileMonitorConfig{
			Enabled:              true,
			BulkWriteThreshold:   50,
			BulkRenameThreshold:  10,
			EntropyThreshold:     7.5,
			SuspiciousExtensions: []string{".encrypted", ".locked", ".crypto", ".crypt", ".enc", ".crypted", ".locky", ".wncry"},
			TimeWindowSec:        60,
		},
		IOMonitor: IOMonitorConfig{
			Enabled:                true,
			BurstWriteThresholdBps: 100 * 1024 * 1024, // 100 MB/s
			AnomalousRWRatio:       10.0,
			SampleIntervalSec:      5,
			WindowSize:             60,
		},
		ProcessMonitor: ProcessMonitorConfig{
			Enabled:               true,
			SuspiciousProcessNames: []string{"cryptolocker", "wannacry", "petya", "ryuk", "maze", "revil", "lockbit", "conti", "blackcat", "cl0p"},
			PrivEscThreshold:       3,
			MaxFileOpenPerProcess:  1000,
		},
		AIModel: AIModelConfig{
			ScoreThreshold:      70,
			ConfidenceThreshold: 60,
			WeightFile:          0.4,
			WeightIO:            0.3,
			WeightProcess:       0.3,
		},
		AutoResponse: AutoResponseConfig{
			Enabled:           true,
			ThresholdLevel:    ThreatLevelHigh,
			DefaultAction:     ActionAlert,
			SnapshotOnThreat:  true,
			IsolateOnCritical: true,
		},
	}
}

// ============================================================
// File Behavior Types
// ============================================================

// FileBehaviorEvent 文件行为事件
type FileBehaviorEvent struct {
	ID          string        `json:"id"`
	Type        FileEventType `json:"type"`
	Path        string        `json:"path"`
	OldPath     string        `json:"oldPath,omitempty"`
	Size        int64         `json:"size"`
	Extension   string        `json:"extension"`
	UserID      string        `json:"userId"`
	ProcessName string        `json:"processName"`
	ProcessID   int           `json:"processId"`
	Timestamp   time.Time     `json:"timestamp"`
	Entropy     float64       `json:"entropy,omitempty"`
}

// FileBehaviorScore 文件行为评分
type FileBehaviorScore struct {
	// EncryptionLikelihood 加密可能性（0-100）
	EncryptionLikelihood int `json:"encryptionLikelihood"`
	// BulkRenameScore 批量重命名可疑度（0-100）
	BulkRenameScore int `json:"bulkRenameScore"`
	// BulkWriteScore 批量写入可疑度（0-100）
	BulkWriteScore int `json:"bulkWriteScore"`
	// TotalScore 综合评分（0-100）
	TotalScore int `json:"totalScore"`
}

// ============================================================
// IO Behavior Types
// ============================================================

// IOSample IO 采样数据
type IOSample struct {
	Timestamp    time.Time `json:"timestamp"`
	ReadBytes    int64     `json:"readBytes"`
	WriteBytes   int64     `json:"writeBytes"`
	ReadOps      int64     `json:"readOps"`
	WriteOps     int64     `json:"writeOps"`
	SourcePath   string    `json:"sourcePath"`
	ProcessName  string    `json:"processName"`
}

// IOBehaviorScore IO 行为评分
type IOBehaviorScore struct {
	// BurstWriteScore 突发写入可疑度（0-100）
	BurstWriteScore int `json:"burstWriteScore"`
	// RWRatioScore 读写比可疑度（0-100）
	RWRatioScore int `json:"rwRatioScore"`
	// TotalScore 综合评分（0-100）
	TotalScore int `json:"totalScore"`
}

// ============================================================
// Process Behavior Types
// ============================================================

// ProcessBehaviorEvent 进程行为事件
type ProcessBehaviorEvent struct {
	ID          string            `json:"id"`
	Type        ProcessEventType  `json:"type"`
	ProcessName string            `json:"processName"`
	ProcessID   int               `json:"processId"`
	UserID      string            `json:"userId"`
	CmdLine     string            `json:"cmdLine,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
	Details     map[string]string `json:"details,omitempty"`
}

// ProcessBehaviorScore 进程行为评分
type ProcessBehaviorScore struct {
	// SuspiciousProcessScore 可疑进程评分（0-100）
	SuspiciousProcessScore int `json:"suspiciousProcessScore"`
	// PrivEscScore 权限提升可疑度（0-100）
	PrivEscScore int `json:"privEscScore"`
	// AnomalousScore 异常行为评分（0-100）
	AnomalousScore int `json:"anomalousScore"`
	// TotalScore 综合评分（0-100）
	TotalScore int `json:"totalScore"`
}

// ============================================================
// AI Inference Types
// ============================================================

// BehaviorAssessment AI 行为评估结果
type BehaviorAssessment struct {
	// AssessmentID 评估 ID
	AssessmentID string `json:"assessmentId"`
	// Timestamp 评估时间
	Timestamp time.Time `json:"timestamp"`
	// ThreatLevel 威胁级别
	ThreatLevel ThreatLevel `json:"threatLevel"`
	// Score 综合行为评分（0-100）
	Score int `json:"score"`
	// Confidence 置信度（0-100）
	Confidence int `json:"confidence"`
	// FileScore 文件行为评分
	FileScore FileBehaviorScore `json:"fileScore"`
	// IOScore IO 行为评分
	IOScore IOBehaviorScore `json:"ioScore"`
	// ProcessScore 进程行为评分
	ProcessScore ProcessBehaviorScore `json:"processScore"`
	// RecommendedAction 推荐动作
	RecommendedAction ResponseAction `json:"recommendedAction"`
	// Indicators 威胁指标
	Indicators []ThreatIndicator `json:"indicators"`
	// AffectedFiles 受影响文件
	AffectedFiles []string `json:"affectedFiles,omitempty"`
	// Details 详情
	Details map[string]interface{} `json:"details,omitempty"`
}

// ThreatIndicator 威胁指标
type ThreatIndicator struct {
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Weight      int         `json:"weight"`
	Value       interface{} `json:"value"`
	Threshold   interface{} `json:"threshold"`
}

// ============================================================
// Response Types
// ============================================================

// ResponseEvent 响应事件
type ResponseEvent struct {
	ID           string         `json:"id"`
	Timestamp    time.Time      `json:"timestamp"`
	Action       ResponseAction `json:"action"`
	Assessment   *BehaviorAssessment `json:"assessment"`
	Success      bool           `json:"success"`
	Message      string         `json:"message"`
	SnapshotID   string         `json:"snapshotId,omitempty"`
	QuarantineID string         `json:"quarantineId,omitempty"`
}

// ============================================================
// Stats & Status Types
// ============================================================

// Stats 引擎统计
type Stats struct {
	TotalEvents         int64      `json:"totalEvents"`
	FileEvents          int64      `json:"fileEvents"`
	IOEvents            int64      `json:"ioEvents"`
	ProcessEvents       int64      `json:"processEvents"`
	ThreatsDetected     int64      `json:"threatsDetected"`
	ResponsesTriggered  int64      `json:"responsesTriggered"`
	SnapshotsCreated    int64      `json:"snapshotsCreated"`
	LastThreatTime      *time.Time `json:"lastThreatTime,omitempty"`
	LastThreatLevel     ThreatLevel `json:"lastThreatLevel"`
}

// EngineStatus 引擎状态
type EngineStatus struct {
	Running       bool    `json:"running"`
	Uptime        int64   `json:"uptime"`
	Stats         Stats   `json:"stats"`
	ActiveThreats int     `json:"activeThreats"`
	LastError     string  `json:"lastError,omitempty"`
}

// ============================================================
// API Request/Response Types
// ============================================================

// ReportEventRequest 上报事件请求
type ReportEventRequest struct {
	FileEvents    []FileBehaviorEvent    `json:"fileEvents,omitempty"`
	IOEvents      []IOSample             `json:"ioEvents,omitempty"`
	ProcessEvents []ProcessBehaviorEvent `json:"processEvents,omitempty"`
}

// AssessmentResponse 评估响应
type AssessmentResponse struct {
	Assessment *BehaviorAssessment `json:"assessment"`
	Action     ResponseAction      `json:"action,omitempty"`
	Message    string              `json:"message,omitempty"`
}
