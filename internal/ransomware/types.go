// Package ransomware 提供勒索软件检测与防护功能
// v2.295.0 - 勒索软件检测框架
package ransomware

import (
	"time"
)

// DetectionLevel 检测级别
type DetectionLevel string

const (
	// DetectionLevelLow 低敏感度 - 仅检测明显威胁
	DetectionLevelLow DetectionLevel = "low"
	// DetectionLevelMedium 中等敏感度 - 平衡检测
	DetectionLevelMedium DetectionLevel = "medium"
	// DetectionLevelHigh 高敏感度 - 激进检测
	DetectionLevelHigh DetectionLevel = "high"
)

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

// FileEventType 文件事件类型
type FileEventType string

const (
	// FileEventCreate 文件创建
	FileEventCreate FileEventType = "create"
	// FileEventModify 文件修改
	FileEventModify FileEventType = "modify"
	// FileEventDelete 文件删除
	FileEventDelete FileEventType = "delete"
	// FileEventRename 文件重命名
	FileEventRename FileEventType = "rename"
	// FileEventEncrypt 文件加密（检测到加密特征）
	FileEventEncrypt FileEventType = "encrypt"
	// FileEventBulkWrite 批量写入
	FileEventBulkWrite FileEventType = "bulk_write"
)

// ProtectionAction 防护动作
type ProtectionAction string

const (
	// ActionAlert 仅告警
	ActionAlert ProtectionAction = "alert"
	// ActionBlock 阻止操作
	ActionBlock ProtectionAction = "block"
	// ActionQuarantine 隔离文件
	ActionQuarantine ProtectionAction = "quarantine"
	// ActionSnapshot 立即创建快照保护
	ActionSnapshot ProtectionAction = "snapshot"
	// ActionLockdown 锁定整个卷
	ActionLockdown ProtectionAction = "lockdown"
)

// DetectionConfig 检测配置
type DetectionConfig struct {
	// Enabled 是否启用检测
	Enabled bool `json:"enabled"`

	// Level 检测敏感度级别
	Level DetectionLevel `json:"level"`

	// WatchPaths 监控路径列表
	WatchPaths []string `json:"watchPaths"`

	// ExcludePaths 排除路径列表
	ExcludePaths []string `json:"excludePaths"`

	// SuspiciousExtensions 可疑文件扩展名列表
	SuspiciousExtensions []string `json:"suspiciousExtensions"`

	// BulkWriteThreshold 批量写入阈值（单位：文件数/秒）
	BulkWriteThreshold int `json:"bulkWriteThreshold"`

	// EncryptionDetectionEnabled 是否启用加密检测
	EncryptionDetectionEnabled bool `json:"encryptionDetectionEnabled"`

	// RapidDeleteThreshold 快速删除阈值（单位：文件数/秒）
	RapidDeleteThreshold int `json:"rapidDeleteThreshold"`

	// AutoProtectionEnabled 是否启用自动防护
	AutoProtectionEnabled bool `json:"autoProtectionEnabled"`

	// AutoProtectionAction 自动防护动作
	AutoProtectionAction ProtectionAction `json:"autoProtectionAction"`

	// SnapshotOnThreat 威胁时创建快照
	SnapshotOnThreat bool `json:"snapshotOnThreat"`

	// NotifyChannels 通知渠道
	NotifyChannels []string `json:"notifyChannels"`
}

// FileEvent 文件事件
type FileEvent struct {
	// ID 事件ID
	ID string `json:"id"`

	// Type 事件类型
	Type FileEventType `json:"type"`

	// Path 文件路径
	Path string `json:"path"`

	// OldPath 原路径（重命名事件）
	OldPath string `json:"oldPath,omitempty"`

	// Size 文件大小
	Size int64 `json:"size"`

	// Extension 文件扩展名
	Extension string `json:"extension"`

	// UserID 用户ID
	UserID string `json:"userId"`

	// ProcessName 进程名
	ProcessName string `json:"processName"`

	// ProcessID 进程ID
	ProcessID int `json:"processId"`

	// Timestamp 时间戳
	Timestamp time.Time `json:"timestamp"`

	// IsEncrypted 是否被加密
	IsEncrypted bool `json:"isEncrypted"`

	// Entropy 文件熵值（用于加密检测）
	Entropy float64 `json:"entropy,omitempty"`

	// WriteOnceProtected 是否受 WriteOnce 保护
	WriteOnceProtected bool `json:"writeOnceProtected"`
}

// ThreatIndicator 威胁指标
type ThreatIndicator struct {
	// Type 指标类型
	Type string `json:"type"`

	// Description 描述
	Description string `json:"description"`

	// Weight 权重
	Weight int `json:"weight"`

	// Value 观测值
	Value interface{} `json:"value"`

	// Threshold 阈值
	Threshold interface{} `json:"threshold"`
}

// ThreatAssessment 威胁评估结果
type ThreatAssessment struct {
	// AssessmentID 评估ID
	AssessmentID string `json:"assessmentId"`

	// Timestamp 时间戳
	Timestamp time.Time `json:"timestamp"`

	// Level 威胁级别
	Level ThreatLevel `json:"level"`

	// Score 威胁分数 (0-100)
	Score int `json:"score"`

	// Indicators 威胁指标列表
	Indicators []ThreatIndicator `json:"indicators"`

	// AffectedFiles 受影响文件列表
	AffectedFiles []string `json:"affectedFiles"`

	// SourcePath 源路径（检测范围）
	SourcePath string `json:"sourcePath"`

	// RecommendedAction 推荐动作
	RecommendedAction ProtectionAction `json:"recommendedAction"`

	// Confidence 置信度 (0-100)
	Confidence int `json:"confidence"`

	// Details 详细信息
	Details map[string]interface{} `json:"details,omitempty"`
}

// ProtectionEvent 防护事件
type ProtectionEvent struct {
	// ID 事件ID
	ID string `json:"id"`

	// Timestamp 时间戳
	Timestamp time.Time `json:"timestamp"`

	// Action 采取的防护动作
	Action ProtectionAction `json:"action"`

	// Assessment 关联的威胁评估
	Assessment *ThreatAssessment `json:"assessment"`

	// Success 是否成功
	Success bool `json:"success"`

	// Message 结果消息
	Message string `json:"message"`

	// SnapshotID 创建的快照ID（如果有）
	SnapshotID string `json:"snapshotId,omitempty"`

	// LockedVolumes 锁定的卷列表（如果有）
	LockedVolumes []string `json:"lockedVolumes,omitempty"`
}

// DetectionStats 检测统计
type DetectionStats struct {
	// TotalEvents 总事件数
	TotalEvents int64 `json:"totalEvents"`

	// ThreatsDetected 检测到的威胁数
	ThreatsDetected int64 `json:"threatsDetected"`

	// FalsePositives 误报数
	FalsePositives int64 `json:"falsePositives"`

	// ProtectionsTriggered 触发的防护次数
	ProtectionsTriggered int64 `json:"protectionsTriggered"`

	// SnapshotsCreated 创建的保护快照数
	SnapshotsCreated int64 `json:"snapshotsCreated"`

	// LastThreatTime 最后威胁时间
	LastThreatTime *time.Time `json:"lastThreatTime,omitempty"`

	// LastThreatLevel 最后威胁级别
	LastThreatLevel ThreatLevel `json:"lastThreatLevel"`
}

// BehaviorPattern 行为模式
type BehaviorPattern struct {
	// PatternID 模式ID
	PatternID string `json:"patternId"`

	// Name 模式名称
	Name string `json:"name"`

	// Description 描述
	Description string `json:"description"`

	// Indicators 指标列表
	Indicators []PatternIndicator `json:"indicators"`

	// Severity 严重性
	Severity ThreatLevel `json:"severity"`

	// ConfidenceWeight 置信度权重
	ConfidenceWeight int `json:"confidenceWeight"`
}

// PatternIndicator 模式指标
type PatternIndicator struct {
	// EventType 事件类型
	EventType FileEventType `json:"eventType"`

	// MinCount 最小次数阈值
	MinCount int `json:"minCount"`

	// TimeWindowSec 时间窗口（秒）
	TimeWindowSec int `json:"timeWindowSec"`

	// ExtensionPattern 扩展名模式（正则）
	ExtensionPattern string `json:"extensionPattern,omitempty"`

	// EntropyMin 最小熵值阈值
	EntropyMin float64 `json:"entropyMin,omitempty"`
}

// WriteOncePolicy WriteOnce 策略配置
type WriteOncePolicy struct {
	// Enabled 是否启用
	Enabled bool `json:"enabled"`

	// ProtectedPaths 保护路径列表
	ProtectedPaths []string `json:"protectedPaths"`

	// AllowDelete 是否允许删除
	AllowDelete bool `json:"allowDelete"`

	// AllowRename 是否允许重命名
	AllowRename bool `json:"allowRename"`

	// AuditEnabled 是否启用审计
	AuditEnabled bool `json:"auditEnabled"`
}

// SnapshotProtectionConfig 快照保护配置
type SnapshotProtectionConfig struct {
	// Enabled 是否启用
	Enabled bool `json:"enabled"`

	// AutoSnapshotOnThreat 威胁时自动快照
	AutoSnapshotOnThreat bool `json:"autoSnapshotOnThreat"`

	// PreemptiveSnapshot 预防性快照（定时）
	PreemptiveSnapshot bool `json:"preemptiveSnapshot"`

	// SnapshotInterval 快照间隔（分钟）
	SnapshotInterval int `json:"snapshotInterval"`

	// MaxSnapshots 最大快照数
	MaxSnapshots int `json:"maxSnapshots"`

	// ProtectedVolumes 保护卷列表
	ProtectedVolumes []string `json:"protectedVolumes"`
}

// DetectorStatus 检测器状态
type DetectorStatus struct {
	// Running 是否运行中
	Running bool `json:"running"`

	// Uptime 运行时长（秒）
	Uptime int64 `json:"uptime"`

	// Stats 统计数据
	Stats DetectionStats `json:"stats"`

	// Config 当前配置
	Config DetectionConfig `json:"config"`

	// LastError 最后错误
	LastError string `json:"lastError,omitempty"`

	// ActiveThreats 当前活跃威胁数
	ActiveThreats int `json:"activeThreats"`
}

