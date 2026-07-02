// Package ransomshield 提供勒索软件高级防护功能
// 对标 TrueNAS 26 勒索检测功能，在 ransomware 基础模块上增强：
// - AI 驱动的熵分析与行为模式识别
// - 蜜罐文件陷阱与自动快照
// - 实时进程阻断与攻击回滚恢复
package ransomware

import (
	"time"
)

// ============================================================
// 威胁规则与模式
// ============================================================

// ThreatLevel 威胁级别.
type ThreatLevel int

const (
	ThreatLevelNone     ThreatLevel = 0 // 无威胁
	ThreatLevelLow      ThreatLevel = 1 // 低威胁
	ThreatLevelMedium   ThreatLevel = 2 // 中等威胁
	ThreatLevelHigh     ThreatLevel = 3 // 高威胁
	ThreatLevelCritical ThreatLevel = 4 // 严重威胁
)

// String 威胁级别字符串.
func (l ThreatLevel) String() string {
	switch l {
	case ThreatLevelCritical:
		return "critical"
	case ThreatLevelHigh:
		return "high"
	case ThreatLevelMedium:
		return "medium"
	case ThreatLevelLow:
		return "low"
	default:
		return "none"
	}
}

// AttackPhase 攻击阶段.
type AttackPhase string

const (
	AttackPhaseRecon    AttackPhase = "recon"    // 侦察阶段
	AttackPhaseDelivery AttackPhase = "delivery" // 投递阶段
	AttackPhaseExecute  AttackPhase = "execute"  // 执行阶段
	AttackPhaseEncrypt  AttackPhase = "encrypt"  // 加密阶段
	AttackPhaseExfil    AttackPhase = "exfil"    // 数据窃取阶段
	AttackPhaseRansom   AttackPhase = "ransom"   // 勒索阶段
)

// ThreatRule 威胁检测规则.
type ThreatRule struct {
	ID          string      `json:"id"`          // 规则ID
	Name        string      `json:"name"`        // 规则名称
	Description string      `json:"description"` // 规则描述
	Enabled     bool        `json:"enabled"`     // 是否启用
	Level       ThreatLevel `json:"level"`       // 命中时的威胁级别
	Phase       AttackPhase `json:"phase"`       // 对应攻击阶段
	Weight      int         `json:"weight"`      // 权重 (0-100)
	Conditions  []Condition `json:"conditions"`  // 触发条件列表
	Actions     []Action    `json:"actions"`     // 触发动作列表
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// Condition 触发条件.
type Condition struct {
	Type          ConditionType `json:"type"`            // 条件类型
	Operator      string        `json:"operator"`        // 操作符: gt, lt, eq, gte, lte, regex
	Value         interface{}   `json:"value"`           // 阈值
	TimeWindowSec int           `json:"time_window_sec"` // 时间窗口（秒）
}

// ConditionType 条件类型.
type ConditionType string

const (
	ConditionEntropy        ConditionType = "entropy"          // 文件熵值
	ConditionWriteFreq      ConditionType = "write_freq"       // 写入频率
	ConditionRenameFreq     ConditionType = "rename_freq"      // 重命名频率
	ConditionDeleteFreq     ConditionType = "delete_freq"      // 删除频率
	ConditionExtChange      ConditionType = "ext_change"       // 扩展名变更
	ConditionFileSizeChange ConditionType = "file_size_change" // 文件大小突变
	ConditionProcessAnomaly ConditionType = "process_anomaly"  // 进程异常行为
	ConditionHoneypotAccess ConditionType = "honeypot_access"  // 蜜罐访问
	ConditionNetworkAnomaly ConditionType = "network_anomaly"  // 网络异常
)

// Action 触发动作.
type Action struct {
	Type   ActionType        `json:"type"`             // 动作类型
	Params map[string]string `json:"params,omitempty"` // 动作参数
}

// ActionType 动作类型.
type ActionType string

const (
	ActionTypeAlert       ActionType = "alert"        // 告警
	ActionTypeBlock       ActionType = "block"        // 阻断
	ActionTypeQuarantine  ActionType = "quarantine"   // 隔离
	ActionTypeSnapshot    ActionType = "snapshot"     // 快照
	ActionTypeLockdown    ActionType = "lockdown"     // 锁定
	ActionTypeKillProcess ActionType = "kill_process" // 杀进程
	ActionTypeRollback    ActionType = "rollback"     // 回滚
)

// ============================================================
// 防护策略
// ============================================================

// ShieldPolicy 防护策略.
type ShieldPolicy struct {
	ID                  string       `json:"id"`                     // 策略ID
	Name                string       `json:"name"`                   // 策略名称
	Description         string       `json:"description"`            // 策略描述
	Enabled             bool         `json:"enabled"`                // 是否启用
	Priority            int          `json:"priority"`               // 优先级 (1-100)
	WatchPaths          []string     `json:"watch_paths"`            // 监控路径
	ExcludePaths        []string     `json:"exclude_paths"`          // 排除路径
	EntropyThreshold    float64      `json:"entropy_threshold"`      // 熵值阈值 (0-8)
	HoneypotEnabled     bool         `json:"honeypot_enabled"`       // 蜜罐功能
	HoneypotPaths       []string     `json:"honeypot_paths"`         // 蜜罐文件路径
	AutoSnapshot        bool         `json:"auto_snapshot"`          // 自动快照
	SnapshotRetention   int          `json:"snapshot_retention"`     // 快照保留数
	RealtimeBlock       bool         `json:"realtime_block"`         // 实时阻断
	MaxBlockDurationSec int          `json:"max_block_duration_sec"` // 最大阻断时长（秒）
	Rules               []string     `json:"rules"`                  // 关联规则ID列表
	Actions             []ActionType `json:"actions"`                // 默认动作列表
	NotifyChannels      []string     `json:"notify_channels"`        // 通知渠道
	CreatedAt           time.Time    `json:"created_at"`
	UpdatedAt           time.Time    `json:"updated_at"`
}

// DefaultShieldPolicy 返回默认防护策略.
func DefaultShieldPolicy() ShieldPolicy {
	return ShieldPolicy{
		ID:                  "default",
		Name:                "默认防护策略",
		Description:         "NAS 默认勒索软件防护策略",
		Enabled:             true,
		Priority:            50,
		WatchPaths:          []string{"/data"},
		ExcludePaths:        []string{"/data/.snapshots", "/data/.trash"},
		EntropyThreshold:    7.5,
		HoneypotEnabled:     true,
		HoneypotPaths:       []string{"/data/.honeypot"},
		AutoSnapshot:        true,
		SnapshotRetention:   10,
		RealtimeBlock:       true,
		MaxBlockDurationSec: 300,
		Actions:             []ActionType{ActionTypeAlert, ActionTypeSnapshot},
		NotifyChannels:      []string{"email", "webhook"},
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
}

// ============================================================
// 攻击模式
// ============================================================

// AttackPattern 攻击模式定义.
type AttackPattern struct {
	ID          string      `json:"id"`          // 模式ID
	Name        string      `json:"name"`        // 模式名称
	Description string      `json:"description"` // 模式描述
	Phase       AttackPhase `json:"phase"`       // 攻击阶段
	Severity    ThreatLevel `json:"severity"`    // 严重性
	Indicators  []Indicator `json:"indicators"`  // 指标列表
	Confidence  float64     `json:"confidence"`  // 置信度 (0-1)
	Signature   string      `json:"signature"`   // 模式签名
	Enabled     bool        `json:"enabled"`     // 是否启用
	CreatedAt   time.Time   `json:"created_at"`
}

// Indicator 攻击指标.
type Indicator struct {
	Type       string  `json:"type"`        // 指标类型
	Weight     float64 `json:"weight"`      // 权重
	Threshold  float64 `json:"threshold"`   // 阈值
	WindowSize int     `json:"window_size"` // 时间窗口（秒）
}

// ============================================================
// 恢复点
// ============================================================

// RecoveryPoint 恢复点.
type RecoveryPoint struct {
	ID           string         `json:"id"`            // 恢复点ID
	Name         string         `json:"name"`          // 恢复点名称
	Description  string         `json:"description"`   // 描述
	Type         RecoveryType   `json:"type"`          // 类型
	Path         string         `json:"path"`          // 快照路径
	SizeBytes    int64          `json:"size_bytes"`    // 大小
	TriggerEvent string         `json:"trigger_event"` // 触发事件ID
	ThreatLevel  ThreatLevel    `json:"threat_level"`  // 威胁级别
	FilesCount   int            `json:"files_count"`   // 文件数量
	Status       RecoveryStatus `json:"status"`        // 状态
	CreatedAt    time.Time      `json:"created_at"`
	ExpiresAt    *time.Time     `json:"expires_at,omitempty"` // 过期时间
}

// RecoveryType 恢复点类型.
type RecoveryType string

const (
	RecoveryTypeAuto       RecoveryType = "auto"       // 自动创建（威胁触发）
	RecoveryTypeManual     RecoveryType = "manual"     // 手动创建
	RecoveryTypeScheduled  RecoveryType = "scheduled"  // 定时创建
	RecoveryTypePreemptive RecoveryType = "preemptive" // 预防性快照
)

// RecoveryStatus 恢复点状态.
type RecoveryStatus string

const (
	RecoveryStatusReady    RecoveryStatus = "ready"    // 就绪
	RecoveryStatusCreating RecoveryStatus = "creating" // 创建中
	RecoveryStatusExpired  RecoveryStatus = "expired"  // 已过期
	RecoveryStatusRollback RecoveryStatus = "rollback" // 正在回滚
)

// ============================================================
// 蜜罐相关
// ============================================================

// HoneypotFile 蜜罐文件.
type HoneypotFile struct {
	ID           string     `json:"id"`                     // 蜜罐ID
	Path         string     `json:"path"`                   // 文件路径
	Name         string     `json:"name"`                   // 文件名
	SizeBytes    int64      `json:"size_bytes"`             // 文件大小
	Extension    string     `json:"extension"`              // 扩展名
	Hash         string     `json:"hash"`                   // 文件哈希
	Triggered    bool       `json:"triggered"`              // 是否被触发
	TriggerCount int        `json:"trigger_count"`          // 触发次数
	LastTrigger  *time.Time `json:"last_trigger,omitempty"` // 最后触发时间
	CreatedAt    time.Time  `json:"created_at"`
}

// HoneypotConfig 蜜罐配置.
type HoneypotConfig struct {
	Enabled            bool     `json:"enabled"`              // 是否启用
	BasePaths          []string `json:"base_paths"`           // 蜜罐部署路径
	FileCount          int      `json:"file_count"`           // 每路径蜜罐文件数
	FileExtensions     []string `json:"file_extensions"`      // 蜜罐文件扩展名
	RefreshIntervalMin int      `json:"refresh_interval_min"` // 刷新间隔（分钟）
}

// DefaultHoneypotConfig 默认蜜罐配置.
func DefaultHoneypotConfig() HoneypotConfig {
	return HoneypotConfig{
		Enabled:            true,
		BasePaths:          []string{"/data"},
		FileCount:          20,
		FileExtensions:     []string{".docx", ".xlsx", ".pdf", ".jpg", ".txt", ".mp4"},
		RefreshIntervalMin: 1440, // 每天刷新
	}
}

// ============================================================
// 检测结果与统计
// ============================================================

// ThreatEvent 威胁事件.
type ThreatEvent struct {
	ID          string      `json:"id"`           // 事件ID
	RuleID      string      `json:"rule_id"`      // 触发规则ID
	Level       ThreatLevel `json:"level"`        // 威胁级别
	Phase       AttackPhase `json:"phase"`        // 攻击阶段
	Score       int         `json:"score"`        // 威胁分数 (0-100)
	Confidence  float64     `json:"confidence"`   // 置信度 (0-1)
	SourcePath  string      `json:"source_path"`  // 源路径
	ProcessName string      `json:"process_name"` // 进程名
	ProcessID   int         `json:"process_id"`   // 进程ID
	Indicators  []string    `json:"indicators"`   // 命中指标
	Actions     []string    `json:"actions"`      // 已执行动作
	Details     string      `json:"details"`      // 详细描述
	Resolved    bool        `json:"resolved"`     // 是否已解决
	CreatedAt   time.Time   `json:"created_at"`
	ResolvedAt  *time.Time  `json:"resolved_at,omitempty"`
}

// ShieldStatus 防护状态.
type ShieldStatus struct {
	Running           bool         `json:"running"`               // 是否运行中
	Uptime            int64        `json:"uptime"`                // 运行时长（秒）
	PoliciesActive    int          `json:"policies_active"`       // 活跃策略数
	RulesActive       int          `json:"rules_active"`          // 活跃规则数
	HoneypotsDeployed int          `json:"honeypots_deployed"`    // 已部署蜜罐数
	ThreatsDetected   int64        `json:"threats_detected"`      // 检测到的威胁数
	ThreatsBlocked    int64        `json:"threats_blocked"`       // 阻断的威胁数
	SnapshotsCreated  int64        `json:"snapshots_created"`     // 创建的快照数
	RecoveryPoints    int          `json:"recovery_points"`       // 可用恢复点数
	LastThreat        *ThreatEvent `json:"last_threat,omitempty"` // 最近威胁
	Stats             ShieldStats  `json:"stats"`                 // 统计信息
}

// ShieldStats 防护统计.
type ShieldStats struct {
	TotalScans        int64     `json:"total_scans"`         // 总扫描次数
	TotalFilesScanned int64     `json:"total_files_scanned"` // 扫描文件总数
	HighEntropyFiles  int64     `json:"high_entropy_files"`  // 高熵文件数
	AnomaliesDetected int64     `json:"anomalies_detected"`  // 异常检测数
	BlocksTriggered   int64     `json:"blocks_triggered"`    // 阻断触发数
	QuarantinesDone   int64     `json:"quarantines_done"`    // 隔离次数
	RollbacksDone     int64     `json:"rollbacks_done"`      // 回滚次数
	LastScanTime      time.Time `json:"last_scan_time"`      // 最后扫描时间
	ScanDurationMs    int64     `json:"scan_duration_ms"`    // 扫描耗时
}

// ============================================================
// API 请求/响应
// ============================================================

// CreatePolicyRequest 创建策略请求.
type CreatePolicyRequest struct {
	Name                string       `json:"name" binding:"required"`
	Description         string       `json:"description"`
	Enabled             bool         `json:"enabled"`
	Priority            int          `json:"priority"`
	WatchPaths          []string     `json:"watch_paths"`
	ExcludePaths        []string     `json:"exclude_paths"`
	EntropyThreshold    float64      `json:"entropy_threshold"`
	HoneypotEnabled     bool         `json:"honeypot_enabled"`
	AutoSnapshot        bool         `json:"auto_snapshot"`
	SnapshotRetention   int          `json:"snapshot_retention"`
	RealtimeBlock       bool         `json:"realtime_block"`
	MaxBlockDurationSec int          `json:"max_block_duration_sec"`
	Actions             []ActionType `json:"actions"`
	NotifyChannels      []string     `json:"notify_channels"`
}

// UpdatePolicyRequest 更新策略请求.
type UpdatePolicyRequest struct {
	ID                  string       `json:"id" binding:"required"`
	Name                string       `json:"name"`
	Description         string       `json:"description"`
	Enabled             *bool        `json:"enabled"`
	Priority            *int         `json:"priority"`
	WatchPaths          []string     `json:"watch_paths"`
	ExcludePaths        []string     `json:"exclude_paths"`
	EntropyThreshold    *float64     `json:"entropy_threshold"`
	HoneypotEnabled     *bool        `json:"honeypot_enabled"`
	AutoSnapshot        *bool        `json:"auto_snapshot"`
	SnapshotRetention   *int         `json:"snapshot_retention"`
	RealtimeBlock       *bool        `json:"realtime_block"`
	MaxBlockDurationSec *int         `json:"max_block_duration_sec"`
	Actions             []ActionType `json:"actions"`
	NotifyChannels      []string     `json:"notify_channels"`
}

// ThreatListResponse 威胁列表响应.
type ThreatListResponse struct {
	Threats []ThreatEvent `json:"threats"`
	Total   int           `json:"total"`
	Page    int           `json:"page"`
	PerPage int           `json:"per_page"`
}

// RecoveryListResponse 恢复点列表响应.
type RecoveryListResponse struct {
	Points []RecoveryPoint `json:"points"`
	Total  int             `json:"total"`
}

// RollbackRequest 回滚请求.
type RollbackRequest struct {
	RecoveryPointID string `json:"recovery_point_id" binding:"required"`
	TargetPath      string `json:"target_path" binding:"required"`
	DryRun          bool   `json:"dry_run"`
}
