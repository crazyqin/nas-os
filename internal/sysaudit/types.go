// Package sysaudit 提供系统级审计日志管理功能
// 负责用户操作、系统操作和安全事件的审计记录
package sysaudit

import "time"

// ========== 日志级别 ==========

// Level 审计日志级别.
type Level string

// 日志级别常量.
const (
	LevelInfo     Level = "info"     // 信息级别
	LevelWarning  Level = "warning"  // 警告级别
	LevelError    Level = "error"    // 错误级别
	LevelCritical Level = "critical" // 严重级别
)

// ========== 事件分类 ==========

// Category 审计事件分类.
type Category string

// 事件分类常量.
const (
	CategoryUserOp       Category = "user_operation"   // 用户操作
	CategorySysOp        Category = "system_operation" // 系统操作
	CategorySecEvent     Category = "security_event"   // 安全事件
	CategoryAuthEvent    Category = "auth_event"       // 认证事件
	CategoryConfigChange Category = "config_change"    // 配置变更
	CategoryAccessCtrl   Category = "access_control"   // 访问控制
	CategoryDataOp       Category = "data_operation"   // 数据操作
	CategoryNetEvent     Category = "network_event"    // 网络事件
)

// ========== 操作状态 ==========

// Status 操作状态.
type Status string

// 操作状态常量.
const (
	StatusSuccess Status = "success" // 成功
	StatusFailure Status = "failure" // 失败
	StatusDenied  Status = "denied"  // 拒绝
	StatusPending Status = "pending" // 待处理
)

// ========== 审计事件严重级别 ==========

// Severity 事件严重级别（用于规则引擎）.
type Severity string

// 严重级别常量.
const (
	SeverityLow      Severity = "low"      // 低
	SeverityMedium   Severity = "medium"   // 中
	SeverityHigh     Severity = "high"     // 高
	SeverityCritical Severity = "critical" // 严重
)

// ========== 核心数据结构 ==========

// Entry 系统审计日志条目.
type Entry struct {
	ID          string                 `json:"id"`                     // 唯一标识
	Timestamp   time.Time              `json:"timestamp"`              // 时间戳
	Level       Level                  `json:"level"`                  // 日志级别
	Category    Category               `json:"category"`               // 事件分类
	Event       string                 `json:"event"`                  // 事件类型
	UserID      string                 `json:"user_id,omitempty"`      // 用户ID
	Username    string                 `json:"username,omitempty"`     // 用户名
	IP          string                 `json:"ip,omitempty"`           // 客户端IP
	UserAgent   string                 `json:"user_agent,omitempty"`   // 用户代理
	Resource    string                 `json:"resource,omitempty"`     // 操作资源
	Action      string                 `json:"action,omitempty"`       // 操作类型
	Status      Status                 `json:"status"`                 // 操作状态
	Message     string                 `json:"message,omitempty"`      // 日志消息
	Details     map[string]interface{} `json:"details,omitempty"`      // 详细信息
	SessionID   string                 `json:"session_id,omitempty"`   // 会话ID
	Hostname    string                 `json:"hostname,omitempty"`     // 主机名
	ProcessName string                 `json:"process_name,omitempty"` // 进程名
	Signature   string                 `json:"signature,omitempty"`    // 数字签名（防篡改）
}

// ========== 查询和筛选 ==========

// QueryOptions 审计日志查询选项.
type QueryOptions struct {
	Limit     int        `json:"limit"`                // 返回数量限制
	Offset    int        `json:"offset"`               // 偏移量
	StartTime *time.Time `json:"start_time,omitempty"` // 开始时间
	EndTime   *time.Time `json:"end_time,omitempty"`   // 结束时间
	Level     Level      `json:"level,omitempty"`      // 日志级别
	Category  Category   `json:"category,omitempty"`   // 事件分类
	UserID    string     `json:"user_id,omitempty"`    // 用户ID
	Username  string     `json:"username,omitempty"`   // 用户名
	IP        string     `json:"ip,omitempty"`         // IP地址
	Status    Status     `json:"status,omitempty"`     // 操作状态
	Event     string     `json:"event,omitempty"`      // 事件类型
	Resource  string     `json:"resource,omitempty"`   // 资源
	Keyword   string     `json:"keyword,omitempty"`    // 关键词搜索
	SessionID string     `json:"session_id,omitempty"` // 会话ID
	Hostname  string     `json:"hostname,omitempty"`   // 主机名
}

// QueryResult 查询结果.
type QueryResult struct {
	Total   int      `json:"total"`   // 总数量
	Entries []*Entry `json:"entries"` // 日志条目
}

// ========== 导出选项 ==========

// ExportFormat 导出格式.
type ExportFormat string

// 导出格式常量.
const (
	ExportJSON ExportFormat = "json"
	ExportCSV  ExportFormat = "csv"
	ExportXML  ExportFormat = "xml"
	ExportYAML ExportFormat = "yaml"
)

// ExportOptions 导出选项.
type ExportOptions struct {
	Format            ExportFormat `json:"format"`             // 导出格式
	StartTime         time.Time    `json:"start_time"`         // 开始时间
	EndTime           time.Time    `json:"end_time"`           // 结束时间
	Categories        []Category   `json:"categories"`         // 包含的分类
	IncludeSignatures bool         `json:"include_signatures"` // 包含签名
	Compress          bool         `json:"compress"`           // 压缩导出
}

// ========== 归档选项 ==========

// ArchiveOptions 归档选项.
type ArchiveOptions struct {
	StartTime    time.Time `json:"start_time"`    // 归档起始时间
	EndTime      time.Time `json:"end_time"`      // 归档结束时间
	ArchivePath  string    `json:"archive_path"`  // 归档存储路径
	Compress     bool      `json:"compress"`      // 是否压缩
	DeleteSource bool      `json:"delete_source"` // 归档后删除源数据
}

// ArchiveResult 归档结果.
type ArchiveResult struct {
	ArchivedCount int    `json:"archived_count"` // 归档条数
	ArchivePath   string `json:"archive_path"`   // 归档文件路径
	FileSize      int64  `json:"file_size"`      // 归档文件大小
}

// ========== 统计信息 ==========

// Statistics 审计统计.
type Statistics struct {
	TotalEntries     int            `json:"total_entries"`          // 总日志数
	TodayEntries     int            `json:"today_entries"`          // 今日日志数
	FailedAuthToday  int            `json:"failed_auth_today"`      // 今日失败认证
	SecurityAlerts   int            `json:"security_alerts"`        // 安全告警数
	TopUsers         []UserActivity `json:"top_users"`              // 活跃用户
	TopIPs           []IPActivity   `json:"top_ips"`                // 活跃IP
	EventsByCategory map[string]int `json:"events_by_category"`     // 分类统计
	EventsByLevel    map[string]int `json:"events_by_level"`        // 级别统计
	EventsByStatus   map[string]int `json:"events_by_status"`       // 状态统计
	StorageUsed      int64          `json:"storage_used"`           // 存储使用量(字节)
	OldestEntry      *time.Time     `json:"oldest_entry,omitempty"` // 最早日志时间
	NewestEntry      *time.Time     `json:"newest_entry,omitempty"` // 最新日志时间
}

// UserActivity 用户活动统计.
type UserActivity struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Count    int    `json:"count"`
}

// IPActivity IP活动统计.
type IPActivity struct {
	IP    string `json:"ip"`
	Count int    `json:"count"`
}

// ========== 完整性验证 ==========

// IntegrityReport 完整性验证报告.
type IntegrityReport struct {
	GeneratedAt     time.Time       `json:"generated_at"`
	TotalEntries    int             `json:"total_entries"`
	Verified        int             `json:"verified"`
	Tampered        int             `json:"tampered"`
	Missing         int             `json:"missing"`
	TamperedEntries []TamperedEntry `json:"tampered_entries,omitempty"`
	Valid           bool            `json:"valid"`
}

// TamperedEntry 被篡改的日志条目.
type TamperedEntry struct {
	EntryID     string    `json:"entry_id"`
	Timestamp   time.Time `json:"timestamp"`
	Reason      string    `json:"reason"`
	OriginalSig string    `json:"original_sig,omitempty"`
	ComputedSig string    `json:"computed_sig,omitempty"`
}

// ========== 合规报告 ==========

// ComplianceStandard 合规标准.
type ComplianceStandard string

// 合规标准常量.
const (
	ComplianceGDPR     ComplianceStandard = "gdpr"     // GDPR
	ComplianceHIPAA    ComplianceStandard = "hipaa"    // HIPAA
	ComplianceSOX      ComplianceStandard = "sox"      // SOX
	ComplianceISO27001 ComplianceStandard = "iso27001" // ISO 27001
	ComplianceMLPS     ComplianceStandard = "mlps"     // 等级保护（中国）
	CompliancePCI      ComplianceStandard = "pci"      // PCI DSS
)

// ComplianceReport 合规报告.
type ComplianceReport struct {
	ReportID        string              `json:"report_id"`       // 报告ID
	Standard        ComplianceStandard  `json:"standard"`        // 合规标准
	GeneratedAt     time.Time           `json:"generated_at"`    // 生成时间
	PeriodStart     time.Time           `json:"period_start"`    // 统计周期开始
	PeriodEnd       time.Time           `json:"period_end"`      // 统计周期结束
	Summary         ComplianceSummary   `json:"summary"`         // 摘要统计
	Findings        []ComplianceFinding `json:"findings"`        // 合规发现
	Recommendations []string            `json:"recommendations"` // 改进建议
	Score           float64             `json:"score"`           // 合规评分 (0-100)
	Passed          bool                `json:"passed"`          // 是否通过
}

// ComplianceSummary 合规摘要统计.
type ComplianceSummary struct {
	TotalEvents        int            `json:"total_events"`         // 总事件数
	AuthEvents         int            `json:"auth_events"`          // 认证事件
	FailedAuthAttempts int            `json:"failed_auth_attempts"` // 失败认证尝试
	DataAccessEvents   int            `json:"data_access_events"`   // 数据访问事件
	ConfigChanges      int            `json:"config_changes"`       // 配置变更
	SecurityAlerts     int            `json:"security_alerts"`      // 安全告警
	UniqueUsers        int            `json:"unique_users"`         // 活跃用户数
	UniqueIPs          int            `json:"unique_ips"`           // 活跃IP数
	EventsByCategory   map[string]int `json:"events_by_category"`   // 分类统计
	EventsByLevel      map[string]int `json:"events_by_level"`      // 级别统计
	EventsByHour       map[int]int    `json:"events_by_hour"`       // 小时统计
}

// ComplianceFinding 合规发现项.
type ComplianceFinding struct {
	ID          string                 `json:"id"`          // 发现ID
	Severity    Severity               `json:"severity"`    // 严重程度
	Category    Category               `json:"category"`    // 分类
	Title       string                 `json:"title"`       // 标题
	Description string                 `json:"description"` // 描述
	Evidence    []*Entry               `json:"evidence"`    // 证据日志
	Metadata    map[string]interface{} `json:"metadata"`    // 元数据
	Remediation string                 `json:"remediation"` // 修复建议
}

// ========== 审计规则引擎 ==========

// AuditRule 审计规则.
type AuditRule struct {
	ID          string        `json:"id"`          // 规则ID
	Name        string        `json:"name"`        // 规则名称
	Description string        `json:"description"` // 规则描述
	Enabled     bool          `json:"enabled"`     // 是否启用
	Priority    int           `json:"priority"`    // 优先级（数值越小优先级越高）
	Severity    Severity      `json:"severity"`    // 触发严重级别
	Category    Category      `json:"category"`    // 适用分类
	Conditions  []Condition   `json:"conditions"`  // 触发条件（AND 逻辑）
	Actions     []RuleAction  `json:"actions"`     // 触发动作
	Cooldown    time.Duration `json:"cooldown"`    // 冷却时间（防止重复触发）
	CreatedAt   time.Time     `json:"created_at"`  // 创建时间
	UpdatedAt   time.Time     `json:"updated_at"`  // 更新时间
}

// Condition 规则条件.
type Condition struct {
	Field    string      `json:"field"`    // 字段名 (level, category, event, user_id, ip, status, message, details.xxx)
	Operator string      `json:"operator"` // 操作符 (eq, neq, contains, not_contains, in, not_in, gt, lt, regex, exists)
	Value    interface{} `json:"value"`    // 比较值
}

// RuleAction 规则触发动作.
type RuleAction struct {
	Type   string                 `json:"type"`             // 动作类型 (alert, notify, block, log, webhook)
	Params map[string]interface{} `json:"params,omitempty"` // 动作参数
}

// RuleMatchResult 规则匹配结果.
type RuleMatchResult struct {
	Matched   bool         `json:"matched"`    // 是否匹配
	RuleID    string       `json:"rule_id"`    // 匹配的规则ID
	RuleName  string       `json:"rule_name"`  // 匹配的规则名称
	Severity  Severity     `json:"severity"`   // 触发严重级别
	Actions   []RuleAction `json:"actions"`    // 需执行的动作
	Entry     *Entry       `json:"entry"`      // 触发的审计日志
	MatchedAt time.Time    `json:"matched_at"` // 匹配时间
}

// ========== 配置 ==========

// Config 系统审计配置.
type Config struct {
	Enabled           bool              `json:"enabled"`            // 是否启用
	LogPath           string            `json:"log_path"`           // 日志存储路径
	ArchivePath       string            `json:"archive_path"`       // 归档存储路径
	MaxEntries        int               `json:"max_entries"`        // 最大内存日志条数
	MaxAgeDays        int               `json:"max_age_days"`       // 最大保留天数
	AutoSave          bool              `json:"auto_save"`          // 自动保存
	SaveInterval      time.Duration     `json:"save_interval"`      // 保存间隔
	EnableSignatures  bool              `json:"enable_signatures"`  // 启用签名防篡改
	EnableCompression bool              `json:"enable_compression"` // 启用压缩
	CompressionType   string            `json:"compression_type"`   // 压缩类型
	RetentionPolicies []RetentionPolicy `json:"retention_policies"` // 保留策略
	Rules             []*AuditRule      `json:"rules"`              // 审计规则
	EnableRuleEngine  bool              `json:"enable_rule_engine"` // 启用规则引擎
}

// RetentionPolicy 日志保留策略.
type RetentionPolicy struct {
	Category Category `json:"category"`  // 适用分类
	MaxAge   int      `json:"max_age"`   // 最大保留天数
	MaxCount int      `json:"max_count"` // 最大条数
	Compress bool     `json:"compress"`  // 是否压缩
}

// ========== API 响应类型 ==========

// APIResponse 通用API响应.
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// SuccessResponse 成功响应.
func SuccessResponse(data interface{}) APIResponse {
	return APIResponse{Code: 0, Message: "success", Data: data}
}

// ErrorResponse 错误响应.
func ErrorResponse(code int, message string) APIResponse {
	return APIResponse{Code: code, Message: message}
}

// ========== 错误定义 ==========

// 错误码定义.
const (
	ErrCodeInvalidParam  = 400
	ErrCodeNotFound      = 404
	ErrCodeInternalError = 500
	ErrCodeAuditDisabled = 503
)

// 错误消息定义.
var (
	ErrAuditDisabled    = "审计功能未启用"
	ErrInvalidTimeRange = "无效的时间范围"
	ErrEntryNotFound    = "审计日志不存在"
	ErrInvalidSignature = "无效的数字签名"
	ErrExportFailed     = "导出失败"
	ErrArchiveFailed    = "归档失败"
	ErrRuleNotFound     = "审计规则不存在"
	ErrRuleInvalid      = "审计规则无效"
)
