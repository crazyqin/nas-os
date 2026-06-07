// Package selectiveadsync - 选择性AD同步模块
// 从 Active Directory 选择性同步指定的 OU（组织单元），而非全量同步
// 参考群晖 DSM 7.3 的 "Smarter domain control" 功能
package selectiveadsync

import (
	"time"
)

// ============================================================
// 配置类型
// ============================================================

// OUSyncConfig OU同步配置
type OUSyncConfig struct {
	DomainController string        `json:"domain_controller"` // 域控制器地址
	BaseDN           string        `json:"base_dn"`           // 基础DN
	BindDN           string        `json:"bind_dn"`           // 绑定DN
	BindPassword     string        `json:"bind_password"`     // 绑定密码
	SyncInterval     time.Duration `json:"sync_interval"`     // 同步间隔
	Enabled          bool          `json:"enabled"`           // 是否启用
	LastSyncAt       time.Time     `json:"last_sync_at"`      // 最后同步时间
}

// DefaultOUSyncConfig 默认OU同步配置
func DefaultOUSyncConfig() OUSyncConfig {
	return OUSyncConfig{
		SyncInterval: 24 * time.Hour,
		Enabled:      false,
	}
}

// ============================================================
// OU 相关类型
// ============================================================

// OUInfo 组织单元信息
type OUInfo struct {
	DN          string    `json:"dn"`           // 可分辨名称
	Name        string    `json:"name"`         // OU名称
	Description string    `json:"description"`  // 描述
	ParentDN    string    `json:"parent_dn"`    // 父级DN
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`   // 更新时间
	ObjectCount int       `json:"object_count"` // 对象数量
	IsSelected  bool      `json:"is_selected"`  // 是否被选中同步
}

// OUFilter OU过滤器
type OUFilter struct {
	IncludeOUs []string `json:"include_ous"` // 包含的OU列表
	ExcludeOUs []string `json:"exclude_ous"` // 排除的OU列表
	Patterns   []string `json:"patterns"`    // 匹配模式（正则表达式）
}

// SyncRule 同步规则
type SyncRule struct {
	ID            string    `json:"id"`             // 规则ID
	Name          string    `json:"name"`           // 规则名称
	Description   string    `json:"description"`    // 描述
	Filter        OUFilter  `json:"filter"`         // OU过滤器
	SyncUsers     bool      `json:"sync_users"`     // 是否同步用户
	SyncGroups    bool      `json:"sync_groups"`    // 是否同步组
	SyncComputers bool      `json:"sync_computers"` // 是否同步计算机
	Enabled       bool      `json:"enabled"`        // 是否启用
	CreatedAt     time.Time `json:"created_at"`     // 创建时间
	UpdatedAt     time.Time `json:"updated_at"`     // 更新时间
}

// ============================================================
// 同步状态类型
// ============================================================

// SyncStatus 同步状态枚举
type SyncStatus string

const (
	SyncStatusIdle    SyncStatus = "idle"    // 空闲
	SyncStatusSyncing SyncStatus = "syncing" // 同步中
	SyncStatusSuccess SyncStatus = "success" // 成功
	SyncStatusFailed  SyncStatus = "failed"  // 失败
	SyncStatusPartial SyncStatus = "partial" // 部分成功
)

// SyncResult 同步结果
type SyncResult struct {
	ID           string       `json:"id"`            // 同步ID
	Status       SyncStatus   `json:"status"`        // 同步状态
	StartTime    time.Time    `json:"start_time"`    // 开始时间
	EndTime      time.Time    `json:"end_time"`      // 结束时间
	Duration     string       `json:"duration"`      // 耗时
	TotalOUs     int          `json:"total_ous"`     // 总OU数
	SyncedOUs    int          `json:"synced_ous"`    // 已同步OU数
	TotalUsers   int          `json:"total_users"`   // 总用户数
	SyncedUsers  int          `json:"synced_users"`  // 已同步用户数
	TotalGroups  int          `json:"total_groups"`  // 总组数
	SyncedGroups int          `json:"synced_groups"` // 已同步组数
	ErrorMessage string       `json:"error_message"` // 错误消息
	Details      []SyncDetail `json:"details"`       // 详细信息
}

// SyncDetail 同步详细信息
type SyncDetail struct {
	OUName      string `json:"ou_name"`      // OU名称
	ObjectType  string `json:"object_type"`  // 对象类型: user, group, computer
	ObjectCount int    `json:"object_count"` // 对象数量
	Status      string `json:"status"`       // 状态
	Message     string `json:"message"`      // 消息
}

// ============================================================
// 同步历史类型
// ============================================================

// SyncHistory 同步历史记录
type SyncHistory struct {
	ID        string     `json:"id"`         // 同步ID
	Status    SyncStatus `json:"status"`     // 状态
	StartTime time.Time  `json:"start_time"` // 开始时间
	EndTime   time.Time  `json:"end_time"`   // 结束时间
	Duration  string     `json:"duration"`   // 耗时
	Summary   string     `json:"summary"`    // 摘要
	RuleCount int        `json:"rule_count"` // 应用规则数
}

// ============================================================
// 统计类型
// ============================================================

// SyncStats 同步统计
type SyncStats struct {
	TotalSyncs      int       `json:"total_syncs"`       // 总同步次数
	SuccessSyncs    int       `json:"success_syncs"`     // 成功次数
	FailedSyncs     int       `json:"failed_syncs"`      // 失败次数
	LastSyncTime    time.Time `json:"last_sync_time"`    // 最后同步时间
	TotalOUs        int       `json:"total_ous"`         // 总OU数
	SelectedOUs     int       `json:"selected_ous"`      // 已选择OU数
	TotalUsers      int       `json:"total_users"`       // 总用户数
	TotalGroups     int       `json:"total_groups"`      // 总组数
	AvgSyncDuration string    `json:"avg_sync_duration"` // 平均同步耗时
}

// ============================================================
// HTTP 请求/响应类型
// ============================================================

// OUListResponse OU列表响应
type OUListResponse struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Data    []OUInfo `json:"data,omitempty"`
}

// OUSelectionRequest OU选择请求
type OUSelectionRequest struct {
	OUDNs   []string `json:"ou_dns"`  // 要选择的OU DN列表
	Replace bool     `json:"replace"` // 是否替换现有选择
}

// SyncRequest 同步请求
type SyncRequest struct {
	RuleIDs []string `json:"rule_ids"` // 要应用的规则ID列表，为空则应用所有规则
	DryRun  bool     `json:"dry_run"`  // 是否仅模拟运行
}

// SyncResponse 同步响应
type SyncResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// SyncResultResponse 同步结果响应
type SyncResultResponse struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    SyncResult `json:"data"`
}

// SyncHistoryResponse 同步历史响应
type SyncHistoryResponse struct {
	Code    int           `json:"code"`
	Message string        `json:"message"`
	Data    []SyncHistory `json:"data,omitempty"`
}

// StatsResponse 统计响应
type StatsResponse struct {
	Code    int       `json:"code"`
	Message string    `json:"message"`
	Data    SyncStats `json:"data"`
}

// RuleListResponse 规则列表响应
type RuleListResponse struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    []SyncRule `json:"data,omitempty"`
}

// RuleRequest 规则请求
type RuleRequest struct {
	Rule SyncRule `json:"rule"`
}

// ============================================================
// LDAP 配置类型
// ============================================================

// LDAPConfig LDAP 连接配置
type LDAPConfig struct {
	Host     string `json:"host"`      // 主机地址
	Port     int    `json:"port"`      // 端口，默认389
	UseSSL   bool   `json:"use_ssl"`   // 是否使用SSL
	BaseDN   string `json:"base_dn"`   // 基础DN
	BindDN   string `json:"bind_dn"`   // 绑定DN
	BindPass string `json:"bind_pass"` // 绑定密码
	Timeout  int    `json:"timeout"`   // 超时（秒）
}

// DefaultLDAPConfig 默认LDAP配置
func DefaultLDAPConfig() LDAPConfig {
	return LDAPConfig{
		Port:    389,
		Timeout: 30,
	}
}
