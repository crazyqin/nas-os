// Package audittrail 提供合规审计追踪功能
package audittrail

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrRecordNotFound 审计记录不存在.
	ErrRecordNotFound = errors.New("审计记录不存在")
	// ErrRecordImmutable 审计记录不可修改.
	ErrRecordImmutable = errors.New("审计记录不可修改")
	// ErrInvalidTimeRange 无效的时间范围.
	ErrInvalidTimeRange = errors.New("无效的时间范围")
	// ErrInvalidQuery 无效的查询参数.
	ErrInvalidQuery = errors.New("无效的查询参数")
	// ErrRuleNotFound 规则不存在.
	ErrRuleNotFound = errors.New("规则不存在")
	// ErrExportFailed 导出失败.
	ErrExportFailed = errors.New("导出失败")
)

// ========== 合规标准 ==========

// ComplianceStandard 合规标准.
type ComplianceStandard string

const (
	// StandardSOC2 SOC2 标准.
	StandardSOC2 ComplianceStandard = "SOC2"
	// StandardGDPR GDPR 标准.
	StandardGDPR ComplianceStandard = "GDPR"
	// StandardHIPAA HIPAA 标准.
	StandardHIPAA ComplianceStandard = "HIPAA"
	// StandardAll 所有标准.
	StandardAll ComplianceStandard = "ALL"
)

// ========== 保留策略 ==========

// RetentionPolicy 数据保留策略.
type RetentionPolicy string

const (
	// Retention7Years 保留7年.
	Retention7Years RetentionPolicy = "7_years"
	// Retention10Years 保留10年.
	Retention10Years RetentionPolicy = "10_years"
	// RetentionPermanent 永久保留.
	RetentionPermanent RetentionPolicy = "permanent"
)

// ========== 导出格式 ==========

// ExportFormat 导出格式.
type ExportFormat string

const (
	// FormatJSON JSON 格式.
	FormatJSON ExportFormat = "json"
	// FormatCSV CSV 格式.
	FormatCSV ExportFormat = "csv"
	// FormatPDF PDF 格式.
	FormatPDF ExportFormat = "pdf"
)

// ========== 操作类型 ==========

// ActionType 操作类型.
type ActionType string

const (
	// ActionCreate 创建操作.
	ActionCreate ActionType = "CREATE"
	// ActionRead 读取操作.
	ActionRead ActionType = "READ"
	// ActionUpdate 更新操作.
	ActionUpdate ActionType = "UPDATE"
	// ActionDelete 删除操作.
	ActionDelete ActionType = "DELETE"
	// ActionLogin 登录操作.
	ActionLogin ActionType = "LOGIN"
	// ActionLogout 登出操作.
	ActionLogout ActionType = "LOGOUT"
	// ActionExport 导出操作.
	ActionExport ActionType = "EXPORT"
	// ActionConfig 配置变更.
	ActionConfig ActionType = "CONFIG"
)

// ========== 操作结果 ==========

// ActionResult 操作结果.
type ActionResult string

const (
	// ResultSuccess 操作成功.
	ResultSuccess ActionResult = "SUCCESS"
	// ResultFailed 操作失败.
	ResultFailed ActionResult = "FAILED"
	// ResultDenied 操作拒绝.
	ResultDenied ActionResult = "DENIED"
	// ResultError 系统错误.
	ResultError ActionResult = "ERROR"
)

// ========== 异常级别 ==========

// AnomalyLevel 异常级别.
type AnomalyLevel string

const (
	// AnomalyLow 低级别异常.
	AnomalyLow AnomalyLevel = "LOW"
	// AnomalyMedium 中级别异常.
	AnomalyMedium AnomalyLevel = "MEDIUM"
	// AnomalyHigh 高级别异常.
	AnomalyHigh AnomalyLevel = "HIGH"
	// AnomalyCritical 严重异常.
	AnomalyCritical AnomalyLevel = "CRITICAL"
)

// ========== 核心类型 ==========

// AuditRecord 审计记录（不可变）.
type AuditRecord struct {
	// ID 唯一标识.
	ID string `json:"id"`
	// Timestamp 操作时间.
	Timestamp time.Time `json:"timestamp"`
	// UserID 操作用户ID.
	UserID string `json:"user_id"`
	// UserName 操作用户名.
	UserName string `json:"user_name"`
	// UserIP 用户IP地址.
	UserIP string `json:"user_ip"`
	// UserAgent 用户代理.
	UserAgent string `json:"user_agent"`
	// Action 操作类型.
	Action ActionType `json:"action"`
	// Resource 资源标识.
	Resource string `json:"resource"`
	// ResourceType 资源类型.
	ResourceType string `json:"resource_type"`
	// Result 操作结果.
	Result ActionResult `json:"result"`
	// Details 操作详情.
	Details map[string]interface{} `json:"details,omitempty"`
	// RequestID 请求ID（用于链式追踪）.
	RequestID string `json:"request_id"`
	// ParentID 父操作ID（用于链式追踪）.
	ParentID string `json:"parent_id,omitempty"`
	// Metadata 元数据.
	Metadata map[string]string `json:"metadata,omitempty"`
	// ComplianceTags 合规标签.
	ComplianceTags []ComplianceStandard `json:"compliance_tags,omitempty"`
	// RetentionPolicy 保留策略.
	RetentionPolicy RetentionPolicy `json:"retention_policy"`
	// ExpiresAt 过期时间（永久保留为零值）.
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	// Checksum 数据校验和（确保不可篡改）.
	Checksum string `json:"checksum"`
}

// AnomalyRule 异常检测规则.
type AnomalyRule struct {
	// ID 规则ID.
	ID string `json:"id"`
	// Name 规则名称.
	Name string `json:"name"`
	// Description 规则描述.
	Description string `json:"description"`
	// Level 异常级别.
	Level AnomalyLevel `json:"level"`
	// Enabled 是否启用.
	Enabled bool `json:"enabled"`
	// Conditions 触发条件.
	Conditions []RuleCondition `json:"conditions"`
	// Action 触发动作.
	Action string `json:"action"`
}

// RuleCondition 规则条件.
type RuleCondition struct {
	// Field 字段名.
	Field string `json:"field"`
	// Operator 操作符 (eq, neq, gt, lt, contains, in).
	Operator string `json:"operator"`
	// Value 期望值.
	Value interface{} `json:"value"`
	// TimeWindow 时间窗口（分钟）.
	TimeWindow int `json:"time_window,omitempty"`
	// Threshold 阈值.
	Threshold int `json:"threshold,omitempty"`
}

// AnomalyDetection 异常检测结果.
type AnomalyDetection struct {
	// ID 异常ID.
	ID string `json:"id"`
	// RuleID 触发的规则ID.
	RuleID string `json:"rule_id"`
	// RuleName 规则名称.
	RuleName string `json:"rule_name"`
	// Level 异常级别.
	Level AnomalyLevel `json:"level"`
	// DetectedAt 检测时间.
	DetectedAt time.Time `json:"detected_at"`
	// RelatedRecords 相关记录ID.
	RelatedRecords []string `json:"related_records"`
	// Description 异常描述.
	Description string `json:"description"`
	// Resolved 是否已解决.
	Resolved bool `json:"resolved"`
	// ResolvedAt 解决时间.
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	// ResolvedBy 解决人.
	ResolvedBy string `json:"resolved_by,omitempty"`
}

// ComplianceReport 合规报告.
type ComplianceReport struct {
	// ID 报告ID.
	ID string `json:"id"`
	// Standard 合规标准.
	Standard ComplianceStandard `json:"standard"`
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generated_at"`
	// PeriodStart 报告周期开始.
	PeriodStart time.Time `json:"period_start"`
	// PeriodEnd 报告周期结束.
	PeriodEnd time.Time `json:"period_end"`
	// Summary 摘要.
	Summary ReportSummary `json:"summary"`
	// Findings 发现项.
	Findings []ComplianceFinding `json:"findings"`
	// Recommendations 建议.
	Recommendations []string `json:"recommendations"`
	// Status 报告状态.
	Status string `json:"status"`
}

// ReportSummary 报告摘要.
type ReportSummary struct {
	// TotalRecords 总记录数.
	TotalRecords int `json:"total_records"`
	// SuccessCount 成功操作数.
	SuccessCount int `json:"success_count"`
	// FailedCount 失败操作数.
	FailedCount int `json:"failed_count"`
	// DeniedCount 拒绝操作数.
	DeniedCount int `json:"denied_count"`
	// AnomalyCount 异常数.
	AnomalyCount int `json:"anomaly_count"`
	// UniqueUsers 唯一用户数.
	UniqueUsers int `json:"unique_users"`
	// ComplianceScore 合规评分 (0-100).
	ComplianceScore float64 `json:"compliance_score"`
}

// ComplianceFinding 合规发现项.
type ComplianceFinding struct {
	// Category 类别.
	Category string `json:"category"`
	// Description 描述.
	Description string `json:"description"`
	// Severity 严重程度.
	Severity string `json:"severity"`
	// Evidence 证据（相关记录ID）.
	Evidence []string `json:"evidence"`
	// Recommendation 建议.
	Recommendation string `json:"recommendation"`
}

// AuditQuery 审计查询条件.
type AuditQuery struct {
	// UserID 用户ID过滤.
	UserID string `json:"user_id,omitempty"`
	// UserName 用户名过滤.
	UserName string `json:"user_name,omitempty"`
	// UserIP 用户IP过滤.
	UserIP string `json:"user_ip,omitempty"`
	// Action 操作类型过滤.
	Action ActionType `json:"action,omitempty"`
	// Resource 资源过滤.
	Resource string `json:"resource,omitempty"`
	// ResourceType 资源类型过滤.
	ResourceType string `json:"resource_type,omitempty"`
	// Result 结果过滤.
	Result ActionResult `json:"result,omitempty"`
	// RequestID 请求ID过滤.
	RequestID string `json:"request_id,omitempty"`
	// ComplianceTag 合规标签过滤.
	ComplianceTag ComplianceStandard `json:"compliance_tag,omitempty"`
	// StartTime 开始时间.
	StartTime *time.Time `json:"start_time,omitempty"`
	// EndTime 结束时间.
	EndTime *time.Time `json:"end_time,omitempty"`
	// Limit 返回数量限制.
	Limit int `json:"limit,omitempty"`
	// Offset 偏移量.
	Offset int `json:"offset,omitempty"`
}

// ExportRequest 导出请求.
type ExportRequest struct {
	// Format 导出格式.
	Format ExportFormat `json:"format"`
	// Query 查询条件.
	Query AuditQuery `json:"query"`
	// FileName 文件名.
	FileName string `json:"file_name,omitempty"`
}

// OperationChain 操作链.
type OperationChain struct {
	// RequestID 请求ID.
	RequestID string `json:"request_id"`
	// Records 操作记录链.
	Records []*AuditRecord `json:"records"`
	// StartTime 开始时间.
	StartTime time.Time `json:"start_time"`
	// EndTime 结束时间.
	EndTime time.Time `json:"end_time"`
	// Duration 持续时间.
	Duration time.Duration `json:"duration"`
	// FinalResult 最终结果.
	FinalResult ActionResult `json:"final_result"`
}

// ComplianceStats 合规统计.
type ComplianceStats struct {
	// TotalRecords 总记录数.
	TotalRecords int `json:"total_records"`
	// RecordsByStandard 按标准统计.
	RecordsByStandard map[ComplianceStandard]int `json:"records_by_standard"`
	// RecordsByAction 按操作类型统计.
	RecordsByAction map[ActionType]int `json:"records_by_action"`
	// RecordsByResult 按结果统计.
	RecordsByResult map[ActionResult]int `json:"records_by_result"`
	// AnomalyCount 异常数.
	AnomalyCount int `json:"anomaly_count"`
	// ResolvedAnomalies 已解决异常数.
	ResolvedAnomalies int `json:"resolved_anomalies"`
}
