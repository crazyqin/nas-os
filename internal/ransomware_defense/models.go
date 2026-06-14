// Package ransomware_defense 提供勒索软件防护模块
// 参考 TrueNAS 26 Ransomware Defense 功能
// 包含蜜罐文件管理、行为分析检测、威胁评分和自动响应机制
package ransomware_defense

import (
	"sync"
	"time"
)

// =============================================================================
// 威胁级别定义
// =============================================================================

// ThreatLevel 威胁级别
type ThreatLevel int

const (
	// ThreatLevelNone 无威胁
	ThreatLevelNone ThreatLevel = iota
	// ThreatLevelLow 低威胁 - 单文件异常
	ThreatLevelLow
	// ThreatLevelMedium 中威胁 - 多文件异常
	ThreatLevelMedium
	// ThreatLevelHigh 高威胁 - 批量加密行为
	ThreatLevelHigh
	// ThreatLevelCritical 严重威胁 - 勒索软件确认
	ThreatLevelCritical
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

// =============================================================================
// 文件操作类型
// =============================================================================

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
	// FileOpRead 文件读取
	FileOpRead FileOperation = "read"
)

// =============================================================================
// 共享协议类型
// =============================================================================

// ShareProtocol 共享协议
type ShareProtocol string

const (
	// ProtocolSMB SMB协议
	ProtocolSMB ShareProtocol = "smb"
	// ProtocolNFS NFS协议
	ProtocolNFS ShareProtocol = "nfs"
	// ProtocolWebDAV WebDAV协议
	ProtocolWebDAV ShareProtocol = "webdav"
	// ProtocolFTP FTP协议
	ProtocolFTP ShareProtocol = "ftp"
)

// =============================================================================
// 响应动作类型
// =============================================================================

// ResponseAction 响应动作
type ResponseAction string

const (
	// ActionAlert 仅告警
	ActionAlert ResponseAction = "alert"
	// ActionBlockIP 阻止IP访问
	ActionBlockIP ResponseAction = "block_ip"
	// ActionDisableShare 禁用共享
	ActionDisableShare ResponseAction = "disable_share"
	// ActionReadOnly 设为只读
	ActionReadOnly ResponseAction = "read_only"
	// ActionRestrictAccess 限制访问
	ActionRestrictAccess ResponseAction = "restrict_access"
	// ActionPauseSnapshotDelete 暂停快照删除
	ActionPauseSnapshotDelete ResponseAction = "pause_snapshot_delete"
	// ActionAutoRestore 自动快照恢复
	ActionAutoRestore ResponseAction = "auto_restore"
	// ActionIsolate 隔离用户/进程
	ActionIsolate ResponseAction = "isolate"
)

// =============================================================================
// 蜜罐相关数据模型
// =============================================================================

// HoneypotFile 蜜罐文件定义
type HoneypotFile struct {
	// ID 蜜罐唯一标识
	ID string `json:"id"`

	// Path 蜜罐文件路径
	Path string `json:"path"`

	// FileName 文件名
	FileName string `json:"fileName"`

	// FileType 文件类型（document/image/database/archive/code）
	FileType string `json:"fileType"`

	// ContentHash 内容哈希（SHA256）
	ContentHash string `json:"contentHash"`

	// FileSize 文件大小
	FileSize int64 `json:"fileSize"`

	// ShareName 所属共享名
	ShareName string `json:"shareName"`

	// Protocol 共享协议
	Protocol ShareProtocol `json:"protocol"`

	// Enabled 是否启用
	Enabled bool `json:"enabled"`

	// Tags 自定义标签
	Tags []string `json:"tags,omitempty"`

	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updatedAt"`

	// LastCheckedAt 最后检查时间
	LastCheckedAt time.Time `json:"lastCheckedAt"`

	// TriggerCount 触发次数
	TriggerCount int64 `json:"triggerCount"`

	// Metadata 扩展元数据
	Metadata map[string]string `json:"metadata,omitempty"`
}

// HoneypotStatus 蜜罐状态
type HoneypotStatus struct {
	// TotalHoneypots 总蜜罐数
	TotalHoneypots int `json:"totalHoneypots"`

	// ActiveHoneypots 活跃蜜罐数
	ActiveHoneypots int `json:"activeHoneypots"`

	// TriggeredHoneypots 已触发的蜜罐数
	TriggeredHoneypots int `json:"triggeredHoneypots"`

	// LastDeploymentTime 最后部署时间
	LastDeploymentTime *time.Time `json:"lastDeploymentTime,omitempty"`

	// MonitoredShares 监控的共享列表
	MonitoredShares []string `json:"monitoredShares"`
}

// HoneypotConfig 蜜罐配置
type HoneypotConfig struct {
	// Enabled 是否启用蜜罐
	Enabled bool `json:"enabled"`

	// AutoDeploy 是否自动部署
	AutoDeploy bool `json:"autoDeploy"`

	// DeploymentPaths 部署路径列表
	DeploymentPaths []string `json:"deploymentPaths"`

	// CheckIntervalSec 检查间隔（秒）
	CheckIntervalSec int `json:"checkIntervalSec"`

	// HoneypotDensity 每个共享目录的蜜罐数量
	HoneypotDensity int `json:"honeypotDensity"`

	// FileTypes 蜜罐文件类型
	FileTypes []string `json:"fileTypes"`

	// NamingPattern 命名模式
	NamingPattern string `json:"namingPattern"`
}

// DefaultHoneypotConfig 返回默认蜜罐配置
func DefaultHoneypotConfig() HoneypotConfig {
	return HoneypotConfig{
		Enabled:          true,
		AutoDeploy:       true,
		DeploymentPaths:  []string{},
		CheckIntervalSec: 30,
		HoneypotDensity:  3,
		FileTypes: []string{
			"document", "image", "database", "archive", "code",
		},
		NamingPattern: "mixed",
	}
}

// =============================================================================
// 文件活动与行为分析数据模型
// =============================================================================

// FileActivity 文件活动记录
type FileActivity struct {
	// Path 文件路径
	Path string `json:"path"`

	// Operation 操作类型
	Operation FileOperation `json:"operation"`

	// OldPath 旧路径（重命名操作时）
	OldPath string `json:"oldPath,omitempty"`

	// Size 文件大小
	Size int64 `json:"size"`

	// ContentHash 内容哈希
	ContentHash string `json:"contentHash,omitempty"`

	// Entropy 文件内容熵值（0-8）
	Entropy float64 `json:"entropy,omitempty"`

	// SourceIP 来源IP
	SourceIP string `json:"sourceIp"`

	// SourceUser 来源用户
	SourceUser string `json:"sourceUser"`

	// ProcessName 进程名
	ProcessName string `json:"processName"`

	// ProcessID 进程ID
	ProcessID int `json:"processId"`

	// Protocol 共享协议
	Protocol ShareProtocol `json:"protocol"`

	// ShareName 共享名称
	ShareName string `json:"shareName"`

	// Timestamp 操作时间
	Timestamp time.Time `json:"timestamp"`
}

// EncryptionSignature 加密签名特征
type EncryptionSignature struct {
	// ID 签名ID
	ID string `json:"id"`

	// Name 签名名称
	Name string `json:"name"`

	// Description 描述
	Description string `json:"description"`

	// KnownExtensions 已知的加密后缀名
	KnownExtensions []string `json:"knownExtensions"`

	// MagicBytes 文件魔数特征
	MagicBytes []byte `json:"magicBytes,omitempty"`

	// EntropyRange 熵值范围 [min, max]
	EntropyRange [2]float64 `json:"entropyRange"`

	// RansomNotePatterns 勒索信特征字符串
	RansomNotePatterns []string `json:"ransomNotePatterns,omitempty"`

	// Severity 严重性
	Severity ThreatLevel `json:"severity"`
}

// BehaviorPattern 行为模式定义
type BehaviorPattern struct {
	// ID 模式ID
	ID string `json:"id"`

	// Name 模式名称
	Name string `json:"name"`

	// Description 模式描述
	Description string `json:"description"`

	// Severity 默认严重性
	Severity ThreatLevel `json:"severity"`

	// Indicators 模式指标
	Indicators []PatternIndicator `json:"indicators"`

	// Weight 权重
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

// =============================================================================
// 威胁评分系统数据模型
// =============================================================================

// ThreatScore 威胁评分详情
type ThreatScore struct {
	// OverallScore 综合评分 (0-100)
	OverallScore int `json:"overallScore"`

	// HoneypotScore 蜜罐触发得分
	HoneypotScore int `json:"honeypotScore"`

	// BehaviorScore 行为分析得分
	BehaviorScore int `json:"behaviorScore"`

	// EncryptionScore 加密签名得分
	EncryptionScore int `json:"encryptionScore"`

	// SnapshotDeltaScore 快照对比得分
	SnapshotDeltaScore int `json:"snapshotDeltaScore"`

	// RateScore 操作速率得分
	RateScore int `json:"rateScore"`

	// Level 威胁级别
	Level ThreatLevel `json:"level"`

	// Factors 得分因子明细
	Factors []ScoreFactor `json:"factors"`
}

// ScoreFactor 得分因子
type ScoreFactor struct {
	// Name 因子名称
	Name string `json:"name"`

	// Score 该因子得分
	Score int `json:"score"`

	// MaxScore 最大可能得分
	MaxScore int `json:"maxScore"`

	// Weight 权重
	Weight float64 `json:"weight"`

	// Description 描述
	Description string `json:"description"`
}

// =============================================================================
// 快照对比数据模型
// =============================================================================

// SnapshotDelta 快照对比差异
type SnapshotDelta struct {
	// SnapshotID 快照ID
	SnapshotID string `json:"snapshotId"`

	// SnapshotTime 快照时间
	SnapshotTime time.Time `json:"snapshotTime"`

	// NewFiles 新增文件数
	NewFiles int `json:"newFiles"`

	// DeletedFiles 删除文件数
	DeletedFiles int `json:"deletedFiles"`

	// ModifiedFiles 修改文件数
	ModifiedFiles int `json:"modifiedFiles"`

	// RenamedFiles 重命名文件数
	RenamedFiles int `json:"renamedFiles"`

	// EntropyIncrease 熵值增加文件数
	EntropyIncrease int `json:"entropyIncrease"`

	// SuspiciousExtensions 可疑扩展名变更数
	SuspiciousExtensions int `json:"suspiciousExtensions"`

	// TotalSizeDelta 总大小变化 (bytes)
	TotalSizeDelta int64 `json:"totalSizeDelta"`
}

// =============================================================================
// 威胁事件
// =============================================================================

// ThreatEvent 威胁事件
type ThreatEvent struct {
	// ID 事件ID
	ID string `json:"id"`

	// ThreatLevel 威胁级别
	ThreatLevel ThreatLevel `json:"threatLevel"`

	// Score 威胁评分
	Score *ThreatScore `json:"score"`

	// DetectionMethod 检测方法
	DetectionMethod string `json:"detectionMethod"`

	// Description 描述
	Description string `json:"description"`

	// SourceIP 来源IP
	SourceIP string `json:"sourceIp"`

	// SourceUser 来源用户
	SourceUser string `json:"sourceUser"`

	// ProcessName 触发进程
	ProcessName string `json:"processName"`

	// ProcessID 触发进程ID
	ProcessID int `json:"processId"`

	// AffectedShare 受影响的共享
	AffectedShare string `json:"affectedShare"`

	// Protocol 共享协议
	Protocol ShareProtocol `json:"protocol"`

	// AffectedFiles 受影响文件列表
	AffectedFiles []string `json:"affectedFiles"`

	// TriggeredHoneypot 触发的蜜罐ID（如果有）
	TriggeredHoneypot string `json:"triggeredHoneypot,omitempty"`

	// Action 执行的响应动作
	Action ResponseAction `json:"action"`

	// Timestamp 事件时间
	Timestamp time.Time `json:"timestamp"`

	// SnapshotDelta 快照差异（如果有）
	SnapshotDelta *SnapshotDelta `json:"snapshotDelta,omitempty"`

	// Details 详细信息
	Details map[string]interface{} `json:"details,omitempty"`

	// Resolved 是否已处理
	Resolved bool `json:"resolved"`

	// ResolvedAt 处理时间
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
}

// =============================================================================
// 防护配置
// =============================================================================

// DefenseConfig 防护总配置
type DefenseConfig struct {
	// Enabled 是否启用
	Enabled bool `json:"enabled"`

	// Honeypot 蜜罐配置
	Honeypot HoneypotConfig `json:"honeypot"`

	// MonitorPaths 监控路径列表
	MonitorPaths []string `json:"monitorPaths"`

	// ExcludePaths 排除路径列表
	ExcludePaths []string `json:"excludePaths"`

	// WatchedProtocols 监控的协议
	WatchedProtocols []ShareProtocol `json:"watchedProtocols"`

	// WindowSizeSec 分析窗口大小（秒）
	WindowSizeSec int `json:"windowSizeSec"`

	// FileRateThreshold 文件操作速率阈值（次/窗口）
	FileRateThreshold int `json:"fileRateThreshold"`

	// EntropyThreshold 熵值阈值
	EntropyThreshold float64 `json:"entropyThreshold"`

	// ThreatScoreThreshold 触发响应的威胁分数阈值
	ThreatScoreThreshold int `json:"threatScoreThreshold"`

	// AutoBlockIP 是否自动阻止IP
	AutoBlockIP bool `json:"autoBlockIP"`

	// AutoDisableShare 是否自动禁用共享
	AutoDisableShare bool `json:"autoDisableShare"`

	// AutoReadOnly 是否自动设为只读
	AutoReadOnly bool `json:"autoReadOnly"`

	// AutoSnapshotProtect 是否自动保护快照
	AutoSnapshotProtect bool `json:"autoSnapshotProtect"`

	// AutoRestore 是否自动从快照恢复
	AutoRestore bool `json:"autoRestore"`

	// AutoRestoreSnapshotAge 自动恢复时选择的快照最大年龄（小时）
	AutoRestoreSnapshotAge int `json:"autoRestoreSnapshotAge"`

	// SuspiciousExtensions 可疑扩展名列表
	SuspiciousExtensions []string `json:"suspiciousExtensions"`

	// MaxAlertHistory 最大告警历史数
	MaxAlertHistory int `json:"maxAlertHistory"`
}

// DefaultDefenseConfig 返回默认防护配置
func DefaultDefenseConfig() DefenseConfig {
	return DefenseConfig{
		Enabled: true,
		Honeypot: DefaultHoneypotConfig(),
		ExcludePaths: []string{"/tmp", "/proc", "/sys", "/dev"},
		WatchedProtocols: []ShareProtocol{ProtocolSMB, ProtocolNFS},
		WindowSizeSec:         60,
		FileRateThreshold:     50,
		EntropyThreshold:      7.5,
		ThreatScoreThreshold:  70,
		AutoBlockIP:           true,
		AutoDisableShare:      true,
		AutoReadOnly:          true,
		AutoSnapshotProtect:   true,
		AutoRestore:           false,
		AutoRestoreSnapshotAge: 24,
		SuspiciousExtensions: []string{
			".encrypted", ".locked", ".crypto", ".crypted",
			".cry", ".wnry", ".wncry", ".wcry",
			".locky", ".cerber", ".zepto", ".odin",
			".thor", ".aesir", ".zzzzz", ".micro",
		},
		MaxAlertHistory: 1000,
	}
}

// =============================================================================
// 模块状态
// =============================================================================

// DefenseStatus 防护模块状态
type DefenseStatus struct {
	// Running 是否运行中
	Running bool `json:"running"`

	// Uptime 运行时长（秒）
	Uptime int64 `json:"uptime"`

	// Config 当前配置
	Config DefenseConfig `json:"config"`

	// HoneypotStatus 蜜罐状态
	HoneypotStatus HoneypotStatus `json:"honeypotStatus"`

	// TotalActivities 总活动数
	TotalActivities int64 `json:"totalActivities"`

	// TotalThreats 检测到的威胁总数
	TotalThreats int64 `json:"totalThreats"`

	// BlockedIPs 已阻止的IP数量
	BlockedIPs int `json:"blockedIPs"`

	// DisabledShares 已禁用的共享数量
	DisabledShares int `json:"disabledShares"`

	// ProtectedSnapshots 已保护的快照数量
	ProtectedSnapshots int `json:"protectedSnapshots"`

	// LastThreatTime 最后威胁时间
	LastThreatTime *time.Time `json:"lastThreatTime,omitempty"`

	// ActiveThreats 当前活跃威胁数
	ActiveThreats int `json:"activeThreats"`
}

// =============================================================================
// IP阻止记录
// =============================================================================

// BlockedIP IP阻止记录
type BlockedIP struct {
	// IP 被阻止的IP地址
	IP string `json:"ip"`

	// Reason 阻止原因
	Reason string `json:"reason"`

	// ThreatEventID 关联的威胁事件ID
	ThreatEventID string `json:"threatEventId"`

	// ThreatScore 触发时的威胁分数
	ThreatScore int `json:"threatScore"`

	// BlockedAt 阻止时间
	BlockedAt time.Time `json:"blockedAt"`

	// ExpiresAt 过期时间（可选，永久为nil）
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`

	// Permanent 是否永久阻止
	Permanent bool `json:"permanent"`
}

// =============================================================================
// 共享保护记录
// =============================================================================

// ShareProtection 共享保护状态
type ShareProtection struct {
	// ShareName 共享名称
	ShareName string `json:"shareName"`

	// Protocol 共享协议
	Protocol ShareProtocol `json:"protocol"`

	// OriginalMode 原始访问模式
	OriginalMode string `json:"originalMode"`

	// CurrentMode 当前访问模式
	CurrentMode string `json:"currentMode"`

	// ProtectionType 保护类型
	ProtectionType ResponseAction `json:"protectionType"`

	// Reason 保护原因
	Reason string `json:"reason"`

	// ThreatEventID 关联的威胁事件ID
	ThreatEventID string `json:"threatEventId"`

	// AppliedAt 应用时间
	AppliedAt time.Time `json:"appliedAt"`

	// AutoReleaseAt 自动释放时间（可选）
	AutoReleaseAt *time.Time `json:"autoReleaseAt,omitempty"`
}

// =============================================================================
// 快照保护记录
// =============================================================================

// SnapshotProtection 快照保护记录
type SnapshotProtection struct {
	// SnapshotID 快照ID
	SnapshotID string `json:"snapshotId"`

	// Dataset 数据集
	Dataset string `json:"dataset"`

	// Reason 保护原因
	Reason string `json:"reason"`

	// ThreatEventID 关联的威胁事件ID
	ThreatEventID string `json:"threatEventId"`

	// ProtectedAt 保护时间
	ProtectedAt time.Time `json:"protectedAt"`

	// DeleteProtected 是否禁止删除
	DeleteProtected bool `json:"deleteProtected"`
}

// =============================================================================
// 共享管理接口（供外部实现注入）
// =============================================================================

// ShareManager 共享管理接口
type ShareManager interface {
	// DisableShare 禁用共享
	DisableShare(name string, protocol ShareProtocol) error

	// EnableShare 启用共享
	EnableShare(name string, protocol ShareProtocol) error

	// SetReadOnly 设为只读
	SetReadOnly(name string, protocol ShareProtocol) error

	// SetReadWrite 设为读写
	SetReadWrite(name string, protocol ShareProtocol) error

	// ListShares 列出所有共享
	ListShares() ([]ShareInfo, error)
}

// ShareInfo 共享信息
type ShareInfo struct {
	Name     string        `json:"name"`
	Path     string        `json:"path"`
	Protocol ShareProtocol `json:"protocol"`
	Enabled  bool          `json:"enabled"`
	ReadOnly bool          `json:"readOnly"`
}

// SnapshotManager 快照管理接口
type SnapshotManager interface {
	// ListSnapshots 列出快照
	ListSnapshots(dataset string) ([]SnapshotInfo, error)

	// ProtectSnapshot 保护快照（防止删除）
	ProtectSnapshot(snapshotID string) error

	// UnprotectSnapshot 取消快照保护
	UnprotectSnapshot(snapshotID string) error

	// CompareSnapshots 对比两个快照
	CompareSnapshots(from, to string) (*SnapshotDelta, error)

	// RestoreSnapshot 恢复快照
	RestoreSnapshot(snapshotID string) error
}

// SnapshotInfo 快照信息
type SnapshotInfo struct {
	ID        string    `json:"id"`
	Dataset   string    `json:"dataset"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
	Protected bool      `json:"protected"`
}

// FirewallManager 防火墙管理接口
type FirewallManager interface {
	// BlockIP 阻止IP地址
	BlockIP(ip string, duration time.Duration, reason string) error

	// UnblockIP 取消阻止IP地址
	UnblockIP(ip string) error

	// IsBlocked 检查IP是否被阻止
	IsBlocked(ip string) bool

	// ListBlocked 列出被阻止的IP
	ListBlocked() ([]BlockedIP, error)
}

// =============================================================================
// 内部同步原语
// =============================================================================

// defenseState 防护内部状态（由Engine持有）
type defenseState struct {
	mu                  sync.RWMutex
	activities          []FileActivity
	threats             []ThreatEvent
	blockedIPs          []BlockedIP
	shareProtections    []ShareProtection
	snapshotProtections []SnapshotProtection
	totalActivities     int64
	totalThreats        int64
	startTime           time.Time
}
