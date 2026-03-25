// Package audit 提供安全审计日志管理功能
// 负责法务合规和知识产权相关的审计记录
package audit

import "time"

// ========== 审计日志类型 ==========

// Level 审计日志级别.
type Level string

// 日志级别常量.
const (
	LevelInfo     Level = "info"     // 信息级别
	LevelWarning  Level = "warning"  // 警告级别
	LevelError    Level = "error"    // 错误级别
	LevelCritical Level = "critical" // 严重级别
)

// Category 审计日志分类.
type Category string

// 审计日志分类常量.
const (
	CategoryAuth       Category = "auth"       // 认证相关
	CategoryAccess     Category = "access"     // 访问控制
	CategoryData       Category = "data"       // 数据操作
	CategorySystem     Category = "system"     // 系统配置
	CategorySecurity   Category = "security"   // 安全事件
	CategoryCompliance Category = "compliance" // 合规相关
	CategoryFile       Category = "file"       // 文件操作
	CategoryNetwork    Category = "network"    // 网络操作
	CategoryUser       Category = "user"       // 用户管理
	CategoryAudit      Category = "audit"      // 审计自身操作
)

// Status 操作状态.
type Status string

// 操作状态常量.
const (
	StatusSuccess Status = "success" // 成功
	StatusFailure Status = "failure" // 失败
	StatusPending Status = "pending" // 待处理
)

// Entry 审计日志条目.
type Entry struct {
	ID        string                 `json:"id"`                   // 唯一标识
	Timestamp time.Time              `json:"timestamp"`            // 时间戳
	Level     Level                  `json:"level"`                // 日志级别
	Category  Category               `json:"category"`             // 日志分类
	Event     string                 `json:"event"`                // 事件类型
	UserID    string                 `json:"user_id,omitempty"`    // 用户ID
	Username  string                 `json:"username,omitempty"`   // 用户名
	IP        string                 `json:"ip,omitempty"`         // 客户端IP
	UserAgent string                 `json:"user_agent,omitempty"` // 用户代理
	Resource  string                 `json:"resource,omitempty"`   // 操作资源
	Action    string                 `json:"action,omitempty"`     // 操作类型
	Status    Status                 `json:"status"`               // 操作状态
	Message   string                 `json:"message,omitempty"`    // 日志消息
	Details   map[string]interface{} `json:"details,omitempty"`    // 详细信息
	Signature string                 `json:"signature,omitempty"`  // 数字签名（防篡改）
}

// ========== 查询和筛选 ==========

// QueryOptions 审计日志查询选项.
type QueryOptions struct {
	Limit     int        `json:"limit"`                // 返回数量限制
	Offset    int        `json:"offset"`               // 偏移量
	StartTime *time.Time `json:"start_time,omitempty"` // 开始时间
	EndTime   *time.Time `json:"end_time,omitempty"`   // 结束时间
	Level     Level      `json:"level,omitempty"`      // 日志级别
	Category  Category   `json:"category,omitempty"`   // 日志分类
	UserID    string     `json:"user_id,omitempty"`    // 用户ID
	Username  string     `json:"username,omitempty"`   // 用户名
	IP        string     `json:"ip,omitempty"`         // IP地址
	Status    Status     `json:"status,omitempty"`     // 操作状态
	Event     string     `json:"event,omitempty"`      // 事件类型
	Resource  string     `json:"resource,omitempty"`   // 资源
	Keyword   string     `json:"keyword,omitempty"`    // 关键词搜索
}

// QueryResult 查询结果.
type QueryResult struct {
	Total   int      `json:"total"`   // 总数量
	Entries []*Entry `json:"entries"` // 日志条目
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
	Severity    Level                  `json:"severity"`    // 严重程度
	Category    Category               `json:"category"`    // 分类
	Title       string                 `json:"title"`       // 标题
	Description string                 `json:"description"` // 描述
	Evidence    []*Entry               `json:"evidence"`    // 证据日志
	Metadata    map[string]interface{} `json:"metadata"`    // 元数据
}

// ========== 审计配置 ==========

// Config 审计配置.
type Config struct {
	Enabled           bool              `json:"enabled"`            // 是否启用审计
	LogPath           string            `json:"log_path"`           // 日志存储路径
	MaxEntries        int               `json:"max_entries"`        // 最大日志条数
	MaxAgeDays        int               `json:"max_age_days"`       // 最大保留天数
	AutoSave          bool              `json:"auto_save"`          // 自动保存
	SaveInterval      time.Duration     `json:"save_interval"`      // 保存间隔
	EnableSignatures  bool              `json:"enable_signatures"`  // 启用签名防篡改
	EnableCompression bool              `json:"enable_compression"` // 启用压缩
	CompressionType   string            `json:"compression_type"`   // 压缩类型 (gzip, zstd)
	RetentionPolicies []RetentionPolicy `json:"retention_policies"` // 保留策略
}

// RetentionPolicy 日志保留策略.
type RetentionPolicy struct {
	Category Category `json:"category"`  // 适用分类
	MaxAge   int      `json:"max_age"`   // 最大保留天数
	MaxCount int      `json:"max_count"` // 最大条数
	Compress bool     `json:"compress"`  // 是否压缩
}

// ========== 统计信息 ==========

// Statistics 审计统计.
type Statistics struct {
	TotalEntries     int            `json:"total_entries"`          // 总日志数
	TodayEntries     int            `json:"today_entries"`          // 今日日志数
	FailedAuthToday  int            `json:"failed_auth_today"`      // 今日失败认证
	SuccessAuthToday int            `json:"success_auth_today"`     // 今日成功认证
	TopUsers         []UserActivity `json:"top_users"`              // 活跃用户
	TopIPs           []IPActivity   `json:"top_ips"`                // 活跃IP
	EventsByCategory map[string]int `json:"events_by_category"`     // 分类统计
	EventsByLevel    map[string]int `json:"events_by_level"`        // 级别统计
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

// ========== 导出选项 ==========

// ExportFormat 导出格式.
type ExportFormat string

// 导出格式常量.
const (
	ExportJSON ExportFormat = "json"
	ExportCSV  ExportFormat = "csv"
	ExportXML  ExportFormat = "xml"
	ExportPDF  ExportFormat = "pdf"
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
)
