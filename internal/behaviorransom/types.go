// Package behaviorransom 提供基于行为分析的勒索软件检测功能
package behaviorransom

import (
	"time"
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

// String 返回威胁级别的字符串表示
func (t ThreatLevel) String() string {
	switch t {
	case ThreatLevelLow:
		return "low"
	case ThreatLevelMedium:
		return "medium"
	case ThreatLevelHigh:
		return "high"
	case ThreatLevelCritical:
		return "critical"
	default:
		return "none"
	}
}

// FileOperation 文件操作类型
type FileOperation string

const (
	// FileOpCreate 文件创建
	FileOpCreate FileOperation = "create"
	// FileOpModify 文件修改
	FileOpModify FileOperation = "modify"
	// FileOpDelete 文件删除
	FileOpDelete FileOperation = "delete"
	// FileOpRename 文件重命名
	FileOpRename FileOperation = "rename"
)

// ResponseAction 响应动作
type ResponseAction string

const (
	// ActionAlert 仅告警
	ActionAlert ResponseAction = "alert"
	// ActionBlock 阻止操作
	ActionBlock ResponseAction = "block"
	// ActionQuarantine 隔离文件
	ActionQuarantine ResponseAction = "quarantine"
	// ActionIsolate 隔离进程/用户
	ActionIsolate ResponseAction = "isolate"
)

// FileActivity 文件活动记录
type FileActivity struct {
	// Path 文件路径
	Path string `json:"path"`

	// Operation 操作类型
	Operation FileOperation `json:"operation"`

	// Size 文件大小
	Size int64 `json:"size"`

	// OldPath 旧路径（重命名时）
	OldPath string `json:"oldPath,omitempty"`

	// ProcessName 进程名
	ProcessName string `json:"processName"`

	// ProcessID 进程ID
	ProcessID int `json:"processId"`

	// UserID 用户ID
	UserID string `json:"userId"`

	// Timestamp 时间戳
	Timestamp time.Time `json:"timestamp"`

	// Entropy 文件熵值
	Entropy float64 `json:"entropy,omitempty"`
}

// BehaviorPattern 行为模式定义
type BehaviorPattern struct {
	// ID 模式ID
	ID string `json:"id"`

	// Name 模式名称
	Name string `json:"name"`

	// Description 模式描述
	Description string `json:"description"`

	// Severity 严重性
	Severity ThreatLevel `json:"severity"`

	// Indicators 模式指标
	Indicators []PatternIndicator `json:"indicators"`

	// Weight 权重（用于计算综合得分）
	Weight int `json:"weight"`
}

// PatternIndicator 模式指标
type PatternIndicator struct {
	// Type 指标类型
	Type string `json:"type"`

	// Threshold 阈值
	Threshold float64 `json:"threshold"`

	// TimeWindowSec 时间窗口（秒）
	TimeWindowSec int `json:"timeWindowSec"`
}

// ThreatEvent 威胁事件
type ThreatEvent struct {
	// ID 事件ID
	ID string `json:"id"`

	// ThreatLevel 威胁级别
	ThreatLevel ThreatLevel `json:"threatLevel"`

	// Score 威胁分数 (0-100)
	Score int `json:"score"`

	// Pattern 匹配的模式
	Pattern string `json:"pattern"`

	// Description 描述
	Description string `json:"description"`

	// SourcePath 源路径
	SourcePath string `json:"sourcePath"`

	// AffectedFiles 受影响文件列表
	AffectedFiles []string `json:"affectedFiles"`

	// Action 响应动作
	Action ResponseAction `json:"action"`

	// Timestamp 时间戳
	Timestamp time.Time `json:"timestamp"`

	// ProcessName 触发进程
	ProcessName string `json:"processName"`

	// ProcessID 触发进程ID
	ProcessID int `json:"processId"`

	// UserID 关联用户ID
	UserID string `json:"userId"`

	// EntropyDelta 熵值变化量
	EntropyDelta float64 `json:"entropyDelta,omitempty"`

	// Details 详细信息
	Details map[string]interface{} `json:"details,omitempty"`
}

// DetectorConfig 检测器配置
type DetectorConfig struct {
	// Enabled 是否启用
	Enabled bool `json:"enabled"`

	// WindowSizeSec 分析窗口大小（秒）
	WindowSizeSec int `json:"windowSizeSec"`

	// FileRateThreshold 文件操作速率阈值（次/窗口）
	FileRateThreshold int `json:"fileRateThreshold"`

	// EntropyThreshold 熵值阈值（超过此值视为加密）
	EntropyThreshold float64 `json:"entropyThreshold"`

	// EntropyDeltaThreshold 熵值变化阈值
	EntropyDeltaThreshold float64 `json:"entropyDeltaThreshold"`

	// BlockThreshold 阻断阈值（分数超过此值自动阻断）
	BlockThreshold int `json:"blockThreshold"`

	// AutoQuarantine 是否自动隔离
	AutoQuarantine bool `json:"autoQuarantine"`

	// WatchPaths 监控路径列表
	WatchPaths []string `json:"watchPaths"`

	// ExcludePaths 排除路径列表
	ExcludePaths []string `json:"excludePaths"`

	// SuspiciousExtensions 可疑扩展名列表
	SuspiciousExtensions []string `json:"suspiciousExtensions"`

	// MaxAlertHistory 最大告警历史数
	MaxAlertHistory int `json:"maxAlertHistory"`
}

// DefaultDetectorConfig 返回默认配置
func DefaultDetectorConfig() DetectorConfig {
	return DetectorConfig{
		Enabled:               true,
		WindowSizeSec:         60,
		FileRateThreshold:     50,
		EntropyThreshold:      7.5,
		EntropyDeltaThreshold: 2.0,
		BlockThreshold:        80,
		AutoQuarantine:        true,
		WatchPaths:            []string{},
		ExcludePaths:          []string{"/tmp", "/proc", "/sys"},
		SuspiciousExtensions: []string{
			".encrypted", ".locked", ".crypto", ".crypted",
			".cry", ".wnry", ".wncry", ".wcry",
		},
		MaxAlertHistory: 1000,
	}
}

// ManagerStatus 管理器状态
type ManagerStatus struct {
	// Running 是否运行中
	Running bool `json:"running"`

	// Uptime 运行时长（秒）
	Uptime int64 `json:"uptime"`

	// TotalActivities 总活动数
	TotalActivities int64 `json:"totalActivities"`

	// TotalThreats 检测到的威胁总数
	TotalThreats int64 `json:"totalThreats"`

	// BlockedThreats 被阻断的威胁数
	BlockedThreats int64 `json:"blockedThreats"`

	// LastThreatTime 最后威胁时间
	LastThreatTime *time.Time `json:"lastThreatTime,omitempty"`

	// ActiveThreats 当前活跃威胁数
	ActiveThreats int `json:"activeThreats"`

	// Config 当前配置
	Config DetectorConfig `json:"config"`
}

// EntropyStats 熵值统计
type EntropyStats struct {
	// MeanEntropy 平均熵值
	MeanEntropy float64 `json:"meanEntropy"`

	// MaxEntropy 最大熵值
	MaxEntropy float64 `json:"maxEntropy"`

	// MinEntropy 最小熵值
	MinEntropy float64 `json:"minEntropy"`

	// HighEntropyFiles 高熵值文件数
	HighEntropyFiles int `json:"highEntropyFiles"`

	// EntropyDistribution 熵值分布
	EntropyDistribution map[string]int `json:"entropyDistribution"`

	// SampleCount 样本数
	SampleCount int `json:"sampleCount"`
}

// QuarantineRecord 隔离记录
type QuarantineRecord struct {
	// ID 记录ID
	ID string `json:"id"`

	// OriginalPath 原始路径
	OriginalPath string `json:"originalPath"`

	// QuarantinePath 隔离路径
	QuarantinePath string `json:"quarantinePath"`

	// Reason 隔离原因
	Reason string `json:"reason"`

	// ThreatEventID 关联威胁事件ID
	ThreatEventID string `json:"threatEventId"`

	// Timestamp 隔离时间
	Timestamp time.Time `json:"timestamp"`

	// FileHash 文件哈希
	FileHash string `json:"fileHash"`

	// FileSize 文件大小
	FileSize int64 `json:"fileSize"`

	// Restored 是否已恢复
	Restored bool `json:"restored"`

	// RestoredAt 恢复时间
	RestoredAt *time.Time `json:"restoredAt,omitempty"`
}
