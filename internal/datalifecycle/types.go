// Package datalifecycle 数据生命周期管理模块
// 支持数据自动分层、基于规则的数据迁移、保留策略管理、自动归档清理、
// 数据血缘追踪、存储成本优化建议和生命周期事件审计日志。
package datalifecycle

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrPolicyNotFound 策略不存在.
	ErrPolicyNotFound = errors.New("策略不存在")
	// ErrRuleNotFound 规则不存在.
	ErrRuleNotFound = errors.New("规则不存在")
	// ErrLineageNotFound 数据血缘记录不存在.
	ErrLineageNotFound = errors.New("数据血缘记录不存在")
	// ErrEventNotFound 审计事件不存在.
	ErrEventNotFound = errors.New("审计事件不存在")
	// ErrPolicyAlreadyExists 策略已存在.
	ErrPolicyAlreadyExists = errors.New("策略已存在")
	// ErrInvalidTier 无效的存储层级.
	ErrInvalidTier = errors.New("无效的存储层级")
	// ErrInvalidAction 无效的动作类型.
	ErrInvalidAction = errors.New("无效的动作类型")
	// ErrPathRequired 必须指定路径.
	ErrPathRequired = errors.New("必须指定路径")
	// ErrPolicyDisabled 策略已禁用.
	ErrPolicyDisabled = errors.New("策略已禁用")
)

// ========== 存储层级类型 ==========

// Tier 存储层级.
type Tier string

const (
	// TierHot 热数据层（SSD/NVMe）——高频访问.
	TierHot Tier = "hot"
	// TierWarm 温数据层（HDD）——中频访问.
	TierWarm Tier = "warm"
	// TierCold 冷数据层（归档存储）——低频访问.
	TierCold Tier = "cold"
	// TierArchive 归档层（云/磁带）——极少访问.
	TierArchive Tier = "archive"
)

// validTiers 有效存储层级集合.
var validTiers = map[Tier]bool{
	TierHot:     true,
	TierWarm:    true,
	TierCold:    true,
	TierArchive: true,
}

// IsValidTier 检查存储层级是否有效.
func IsValidTier(t Tier) bool {
	return validTiers[t]
}

// ========== 迁移动作类型 ==========

// ActionType 生命周期动作类型.
type ActionType string

const (
	// ActionTierUp 向更高性能层迁移.
	ActionTierUp ActionType = "tier_up"
	// ActionTierDown 向更低性能层迁移.
	ActionTierDown ActionType = "tier_down"
	// ActionCompress 压缩数据.
	ActionCompress ActionType = "compress"
	// ActionArchive 归档数据.
	ActionArchive ActionType = "archive"
	// ActionDelete 删除数据.
	ActionDelete ActionType = "delete"
	// ActionSnapshot 创建快照.
	ActionSnapshot ActionType = "snapshot"
	// ActionNotify 发送通知.
	ActionNotify ActionType = "notify"
)

// validActions 有效动作集合.
var validActions = map[ActionType]bool{
	ActionTierUp:   true,
	ActionTierDown: true,
	ActionCompress: true,
	ActionArchive:  true,
	ActionDelete:   true,
	ActionSnapshot: true,
	ActionNotify:   true,
}

// IsValidAction 检查动作类型是否有效.
func IsValidAction(a ActionType) bool {
	return validActions[a]
}

// ========== 事件类型 ==========

// EventType 生命周期事件类型.
type EventType string

const (
	// EventTierChange 存储层变更.
	EventTierChange EventType = "tier_change"
	// EventMigration 数据迁移.
	EventMigration EventType = "migration"
	// EventArchive 归档事件.
	EventArchive EventType = "archive"
	// EventDelete 删除事件.
	EventDelete EventType = "delete"
	// EventCompress 压缩事件.
	EventCompress EventType = "compress"
	// EventRetentionPolicy 策略触发.
	EventRetentionPolicy EventType = "retention_policy"
	// EventLineageUpdate 血缘更新.
	EventLineageUpdate EventType = "lineage_update"
	// EventCostAlert 成本告警.
	EventCostAlert EventType = "cost_alert"
)

// ========== 保留策略相关 ==========

// RetentionMode 保留模式.
type RetentionMode string

const (
	// RetentionModeTime 基于时间保留.
	RetentionModeTime RetentionMode = "time"
	// RetentionModeVersion 基于版本数保留.
	RetentionModeVersion RetentionMode = "version"
	// RetentionModeSpace 基于空间限制保留.
	RetentionModeSpace RetentionMode = "space"
	// RetentionModeCount 基于文件数量保留.
	RetentionModeCount RetentionMode = "count"
)

// ========== 核心数据结构 ==========

// LifecyclePolicy 生命周期策略.
type LifecyclePolicy struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Enabled     bool          `json:"enabled"`
	Priority    int           `json:"priority"` // 优先级，数值越小越先执行

	// 匹配条件
	PathPattern string   `json:"path_pattern"` // glob 模式匹配路径
	Extensions  []string `json:"extensions,omitempty"`
	MinSize     int64    `json:"min_size,omitempty"` // 字节
	MaxSize     int64    `json:"max_size,omitempty"`
	Tags        []string `json:"tags,omitempty"`

	// 当前层级限制（只对处于该层级的数据生效）
	SourceTier Tier `json:"source_tier,omitempty"`

	// 触发条件
	TriggerDays int    `json:"trigger_days"` // 文件最后访问后多少天触发
	Schedule    string `json:"schedule,omitempty"` // cron 表达式

	// 动作列表
	Actions []PolicyAction `json:"actions"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PolicyAction 策略动作.
type PolicyAction struct {
	Type       ActionType `json:"type"`
	TargetTier Tier       `json:"target_tier,omitempty"` // 迁移目标层
	CompressAlgo string   `json:"compress_algo,omitempty"` // 压缩算法
	NotifyMsg  string     `json:"notify_msg,omitempty"` // 通知消息模板
}

// DataItem 数据项.
type DataItem struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	CurrentTier Tier      `json:"current_tier"`
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	ModifiedAt  time.Time `json:"modified_at"`
	AccessedAt  time.Time `json:"accessed_at"`
	AccessCount int64     `json:"access_count"`
}

// MigrationRecord 迁移记录.
type MigrationRecord struct {
	ID          string    `json:"id"`
	FilePath    string    `json:"file_path"`
	SourceTier  Tier      `json:"source_tier"`
	TargetTier  Tier      `json:"target_tier"`
	Action      ActionType `json:"action"`
	PolicyID    string    `json:"policy_id"`
	Status      string    `json:"status"` // pending, running, completed, failed
	ErrorMsg    string    `json:"error_msg,omitempty"`
	BytesMoved  int64     `json:"bytes_moved"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

// RetentionPolicy 数据保留策略.
type RetentionPolicy struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Enabled     bool          `json:"enabled"`
	Mode        RetentionMode `json:"mode"`

	// 时间模式参数
	RetentionDays int `json:"retention_days,omitempty"`

	// 版本模式参数
	MaxVersions int `json:"max_versions,omitempty"`

	// 空间模式参数
	MaxSizeBytes int64 `json:"max_size_bytes,omitempty"`

	// 数量模式参数
	MaxCount int `json:"max_count,omitempty"`

	// 匹配条件
	PathPattern string   `json:"path_pattern"`
	Extensions  []string `json:"extensions,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DataLineage 数据血缘记录.
type DataLineage struct {
	ID          string    `json:"id"`
	FilePath    string    `json:"file_path"`
	SourcePath  string    `json:"source_path,omitempty"` // 来源文件
	Operation   string    `json:"operation"`             // copy, move, transform, merge, split
	Operator    string    `json:"operator,omitempty"`    // 操作者
	Timestamp   time.Time `json:"timestamp"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// LineageGraph 血缘关系图.
type LineageGraph struct {
	Root    *LineageNode   `json:"root"`
	All     []*LineageNode `json:"all"`
}

// LineageNode 血缘节点.
type LineageNode struct {
	FilePath string         `json:"file_path"`
	Parent   string         `json:"parent,omitempty"`
	Operation string        `json:"operation,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Children  []*LineageNode `json:"children,omitempty"`
}

// CostSuggestion 成本优化建议.
type CostSuggestion struct {
	ID          string    `json:"id"`
	FilePath    string    `json:"file_path"`
	CurrentTier Tier      `json:"current_tier"`
	SuggestedTier Tier    `json:"suggested_tier"`
	CurrentCost float64   `json:"current_cost"`  // 月度成本估算（元）
	SuggestedCost float64 `json:"suggested_cost"` // 优化后月度成本
	Savings     float64   `json:"savings"`        // 预计节省
	Reason      string    `json:"reason"`
	CreatedAt   time.Time `json:"created_at"`
}

// CostSummary 成本概览.
type CostSummary struct {
	TotalItems    int                `json:"total_items"`
	TotalSize     int64              `json:"total_size"`
	TotalCost     float64            `json:"total_cost"`
	ByTier        map[Tier]TierCost  `json:"by_tier"`
	Suggestions   []*CostSuggestion  `json:"suggestions,omitempty"`
	TotalSavings  float64            `json:"total_savings"`
	GeneratedAt   time.Time          `json:"generated_at"`
}

// TierCost 层级成本.
type TierCost struct {
	Tier      Tier    `json:"tier"`
	ItemCount int     `json:"item_count"`
	TotalSize int64   `json:"total_size"`
	Cost      float64 `json:"cost"`
}

// AuditEvent 审计事件.
type AuditEvent struct {
	ID        string    `json:"id"`
	EventType EventType `json:"event_type"`
	FilePath  string    `json:"file_path"`
	Details   string    `json:"details"`
	Operator  string    `json:"operator,omitempty"`
	PolicyID  string    `json:"policy_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// ========== 请求/响应结构 ==========

// CreatePolicyRequest 创建策略请求.
type CreatePolicyRequest struct {
	Name        string       `json:"name" binding:"required"`
	Description string       `json:"description"`
	Enabled     bool         `json:"enabled"`
	Priority    int          `json:"priority"`
	PathPattern string       `json:"path_pattern" binding:"required"`
	Extensions  []string     `json:"extensions"`
	MinSize     int64        `json:"min_size"`
	MaxSize     int64        `json:"max_size"`
	Tags        []string     `json:"tags"`
	SourceTier  Tier         `json:"source_tier"`
	TriggerDays int          `json:"trigger_days"`
	Schedule    string       `json:"schedule"`
	Actions     []PolicyAction `json:"actions" binding:"required,min=1"`
}

// CreateRetentionPolicyRequest 创建保留策略请求.
type CreateRetentionPolicyRequest struct {
	Name          string        `json:"name" binding:"required"`
	Description   string        `json:"description"`
	Enabled       bool          `json:"enabled"`
	Mode          RetentionMode `json:"mode" binding:"required"`
	RetentionDays int           `json:"retention_days"`
	MaxVersions   int           `json:"max_versions"`
	MaxSizeBytes  int64         `json:"max_size_bytes"`
	MaxCount      int           `json:"max_count"`
	PathPattern   string        `json:"path_pattern" binding:"required"`
	Extensions    []string      `json:"extensions"`
}

// CreateLineageRequest 创建血缘记录请求.
type CreateLineageRequest struct {
	FilePath   string            `json:"file_path" binding:"required"`
	SourcePath string            `json:"source_path"`
	Operation  string            `json:"operation" binding:"required"`
	Operator   string            `json:"operator"`
	Metadata   map[string]string `json:"metadata"`
}

// EvaluateRequest 评估策略请求.
type EvaluateRequest struct {
	PolicyID string `json:"policy_id"`
	DryRun   bool   `json:"dry_run"`
}

// EvaluateResult 评估结果.
type EvaluateResult struct {
	MatchedItems int      `json:"matched_items"`
	Actions      []string `json:"actions"`
	DryRun       bool     `json:"dry_run"`
	Details      []*MigrationRecord `json:"details,omitempty"`
}

// ========== 存储接口 ==========

// Store 数据生命周期存储接口.
type Store interface {
	// 策略管理
	SavePolicy(policy *LifecyclePolicy) error
	GetPolicy(id string) (*LifecyclePolicy, error)
	ListPolicies() ([]*LifecyclePolicy, error)
	DeletePolicy(id string) error

	// 数据项管理
	SaveDataItem(item *DataItem) error
	GetDataItem(id string) (*DataItem, error)
	ListDataItems(pathPrefix string, tier Tier) ([]*DataItem, error)
	DeleteDataItem(id string) error

	// 迁移记录
	SaveMigration(m *MigrationRecord) error
	ListMigrations(policyID string, limit int) ([]*MigrationRecord, error)

	// 保留策略
	SaveRetentionPolicy(policy *RetentionPolicy) error
	GetRetentionPolicy(id string) (*RetentionPolicy, error)
	ListRetentionPolicies() ([]*RetentionPolicy, error)
	DeleteRetentionPolicy(id string) error

	// 数据血缘
	SaveLineage(lineage *DataLineage) error
	GetLineage(id string) (*DataLineage, error)
	GetLineageByPath(filePath string) ([]*DataLineage, error)
	DeleteLineage(id string) error

	// 成本数据
	SaveCostSuggestion(s *CostSuggestion) error
	ListCostSuggestions() ([]*CostSuggestion, error)

	// 审计日志
	SaveAuditEvent(event *AuditEvent) error
	ListAuditEvents(eventType EventType, limit int) ([]*AuditEvent, error)
}
