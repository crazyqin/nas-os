// Package tieringrules 提供数据分层自定义规则功能，
// 按访问频率/修改时间自动分层，热数据留高性能存储，冷数据迁低成本存储。
// 对标 DSM 7.3 的存储分层能力。
package tieringrules

import "time"

// ConditionType 规则条件类型.
type ConditionType string

const (
	ConditionAccessFreq ConditionType = "access_freq" // 访问频率
	ConditionModifyTime ConditionType = "modify_time" // 修改时间
	ConditionSize       ConditionType = "size"        // 文件大小
)

// ActionType 迁移动作.
type ActionType string

const (
	ActionMove    ActionType = "move"    // 移动到目标池
	ActionCopy    ActionType = "copy"    // 复制到目标池
	ActionArchive ActionType = "archive" // 归档
)

// TieringRule 数据分层规则.
type TieringRule struct {
	// 规则唯一标识
	ID string `json:"id"`
	// 规则名称
	Name string `json:"name"`
	// 条件类型
	Condition ConditionType `json:"condition"`
	// 阈值（频率次/天数/字节，根据 Condition 解释）
	Threshold int64 `json:"threshold"`
	// 源存储池
	SourcePool string `json:"source_pool"`
	// 目标存储池
	TargetPool string `json:"target_pool"`
	// 迁移动作
	Action ActionType `json:"action"`
	// 是否启用
	Enabled bool `json:"enabled"`
	// 创建时间
	CreatedAt time.Time `json:"created_at"`
	// 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// MigrationRecord 迁移记录.
type MigrationRecord struct {
	// 记录 ID
	ID string `json:"id"`
	// 触发的规则 ID
	RuleID string `json:"rule_id"`
	// 规则名称
	RuleName string `json:"rule_name"`
	// 文件路径
	FilePath string `json:"file_path"`
	// 源池
	SourcePool string `json:"source_pool"`
	// 目标池
	TargetPool string `json:"target_pool"`
	// 文件大小
	FileSize int64 `json:"file_size"`
	// 迁移状态
	Status MigrationStatus `json:"status"`
	// 迁移时间
	MigratedAt time.Time `json:"migrated_at"`
	// 错误信息
	Error string `json:"error,omitempty"`
}

// MigrationStatus 迁移状态.
type MigrationStatus string

const (
	MigrationStatusPending MigrationStatus = "pending"
	MigrationStatusSuccess MigrationStatus = "success"
	MigrationStatusFailed  MigrationStatus = "failed"
)

// CreateRuleRequest 创建规则请求.
type CreateRuleRequest struct {
	Name       string        `json:"name"`
	Condition  ConditionType `json:"condition"`
	Threshold  int64         `json:"threshold"`
	SourcePool string        `json:"source_pool"`
	TargetPool string        `json:"target_pool"`
	Action     ActionType    `json:"action"`
	Enabled    bool          `json:"enabled"`
}
