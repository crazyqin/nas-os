// Package domainsync 提供智能域同步功能（对标群晖 DSM 7.3 Smart Domain Sync）
package domainsync

import (
	"errors"
	"time"
)

// 基本错误定义.
var (
	ErrConfigNotFound      = errors.New("域同步配置未找到")
	ErrConnectionFailed    = errors.New("域控制器连接失败")
	ErrOUFetchFailed       = errors.New("获取 OU 列表失败")
	ErrSyncInProgress      = errors.New("同步正在进行中")
	ErrSyncNotRunning      = errors.New("同步未运行")
	ErrInvalidConfig       = errors.New("无效的域同步配置")
	ErrSyncFailed          = errors.New("域同步失败")
	ErrDCNotConfigured     = errors.New("域控制器未配置")
	ErrInvalidOUSource     = errors.New("无效的 OU 来源")
	ErrSyncScheduleInvalid = errors.New("无效的同步计划")
)

// SyncStrategy 同步策略.
type SyncStrategy string

// 同步策略常量.
const (
	SyncStrategyFull        SyncStrategy = "full"        // 全量同步
	SyncStrategyIncremental SyncStrategy = "incremental" // 增量同步
	SyncStrategyScheduled   SyncStrategy = "scheduled"   // 定时同步
)

// SyncStatus 同步状态.
type SyncStatus string

// 同步状态常量.
const (
	SyncStatusIdle      SyncStatus = "idle"      // 空闲
	SyncStatusRunning   SyncStatus = "running"   // 同步中
	SyncStatusCompleted SyncStatus = "completed" // 已完成
	SyncStatusFailed    SyncStatus = "failed"    // 失败
	SyncStatusCancelled SyncStatus = "cancelled" // 已取消
)

// OU 表示 Active Directory 中的组织单元.
type OU struct {
	DN          string `json:"dn"`                     // 可分辨名称
	Name        string `json:"name"`                   // OU 名称
	Description string `json:"description,omitempty"`  // 描述
	ParentDN    string `json:"parent_dn,omitempty"`     // 父 OU 的 DN
	Level       int    `json:"level"`                   // 层级深度（从 0 开始）
	Enabled     bool   `json:"enabled"`                 // 是否启用
	UserCount   int    `json:"user_count,omitempty"`    // 用户数（可选填充）
	GroupCount  int    `json:"group_count,omitempty"`   // 组数（可选填充）
}

// DCConfig 域控制器连接配置.
type DCConfig struct {
	Host           string        `json:"host"`                      // 域控制器地址
	Port           int           `json:"port"`                      // 端口（默认 389/636）
	Domain         string        `json:"domain"`                    // 域名 如 example.com
	BaseDN         string        `json:"base_dn"`                   // 基础 DN
	BindDN         string        `json:"bind_dn,omitempty"`         // 绑定 DN
	BindPassword   string        `json:"bind_password,omitempty"`   // 绑定密码
	UseTLS         bool          `json:"use_tls"`                   // 是否使用 TLS
	SkipTLSVerify  bool          `json:"skip_tls_verify,omitempty"` // 跳过 TLS 验证
	ConnectTimeout time.Duration `json:"connect_timeout"`           // 连接超时
}

// SyncConfig 域同步配置.
type SyncConfig struct {
	// 域控制器配置
	DCConfig DCConfig `json:"dc_config"`

	// 同步策略
	Strategy SyncStrategy `json:"strategy"` // full / incremental / scheduled

	// 选择性同步 - 要同步的 OU 列表
	SelectedOUs []string `json:"selected_ous,omitempty"` // OU DN 列表

	// 同步内容
	SyncUsers  bool `json:"sync_users"`  // 同步用户
	SyncGroups bool `json:"sync_groups"` // 同步组

	// 定时同步设置（仅 strategy=scheduled 时生效）
	ScheduleInterval time.Duration `json:"schedule_interval,omitempty"` // 同步间隔
	ScheduleCron     string        `json:"schedule_cron,omitempty"`     // Cron 表达式（可选）

	// 冲突处理
	ConflictResolution string `json:"conflict_resolution,omitempty"` // skip / overwrite / merge

	// 连接池
	PoolSize int `json:"pool_size,omitempty"`
}

// SyncResult 同步结果.
type SyncResult struct {
	ID        string        `json:"id"`         // 同步任务 ID
	StartTime time.Time     `json:"start_time"` // 开始时间
	EndTime   time.Time     `json:"end_time"`   // 结束时间
	Duration  time.Duration `json:"duration"`   // 耗时
	Status    SyncStatus    `json:"status"`     // 状态
	Strategy  SyncStrategy  `json:"strategy"`   // 使用的策略

	// 同步统计
	UsersSynced     int `json:"users_synced"`     // 同步的用户数
	GroupsSynced    int `json:"groups_synced"`    // 同步的组数
	OUSynced        int `json:"ou_synced"`        // 同步的 OU 数
	UsersCreated    int `json:"users_created"`    // 新增用户
	UsersUpdated    int `json:"users_updated"`    // 更新用户
	UsersDeleted    int `json:"users_deleted"`    // 删除用户
	GroupsCreated   int `json:"groups_created"`   // 新增组
	GroupsUpdated   int `json:"groups_updated"`   // 更新组
	GroupsDeleted   int `json:"groups_deleted"`   // 删除组

	// 错误信息
	Errors   []SyncError `json:"errors,omitempty"`
	Message  string      `json:"message,omitempty"`
	Progress int         `json:"progress"` // 0-100 进度
	Success  bool        `json:"success"`  // 是否成功
}

// SyncError 同步错误.
type SyncError struct {
	Type    string `json:"type"`     // ou, user, group
	DN      string `json:"dn"`       // 对象 DN
	Message string `json:"message"`  // 错误信息
	Code    string `json:"code,omitempty"` // 错误码
}

// DomainSyncStatus 域同步总状态.
type DomainSyncStatus struct {
	Enabled       bool          `json:"enabled"`        // 是否启用
	Status        SyncStatus    `json:"status"`         // 当前状态
	Strategy      SyncStrategy  `json:"strategy"`       // 当前策略
	LastSyncTime  *time.Time    `json:"last_sync_time"` // 上次同步时间
	LastSyncID    string        `json:"last_sync_id"`   // 上次同步 ID
	NextSyncTime  *time.Time    `json:"next_sync_time"` // 下次同步时间（定时模式）
	DCConnected   bool          `json:"dc_connected"`   // 域控制器是否已连接
	OUCount       int           `json:"ou_count"`       // 已配置的 OU 数量
	SelectedCount int           `json:"selected_count"` // 已选择的 OU 数量
	LastResult    *SyncResult   `json:"last_result"`    // 最后同步结果
	ScheduleCron  string        `json:"schedule_cron"`  // 定时表达式
}

// DefaultSyncConfig 获取默认同步配置.
func DefaultSyncConfig() SyncConfig {
	return SyncConfig{
		DCConfig: DCConfig{
			Port:           389,
			ConnectTimeout: 10 * time.Second,
		},
		Strategy:           SyncStrategyFull,
		SyncUsers:          true,
		SyncGroups:         true,
		ConflictResolution: "merge",
		PoolSize:           5,
	}
}
