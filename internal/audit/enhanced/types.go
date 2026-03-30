// Package enhanced 提供增强的安全审计日志功能
// 包括登录审计增强、操作审计追踪、敏感操作标记和审计报告生成
package enhanced

import "time"

// ========== 登录审计类型 ==========

// LoginEventType 登录事件类型.
type LoginEventType string

// 登录事件类型常量.
const (
	LoginEventSuccess         LoginEventType = "success"          // 登录成功
	LoginEventFailure         LoginEventType = "failure"          // 登录失败
	LoginEventLogout          LoginEventType = "logout"           // 登出
	LoginEventSessionExpired  LoginEventType = "session_expired"  // 会话过期
	LoginEventPasswordChange  LoginEventType = "password_change"  // 密码修改
	LoginEventMFAEnabled      LoginEventType = "mfa_enabled"      // 启用MFA
	LoginEventMFADisabled     LoginEventType = "mfa_disabled"     // 禁用MFA
	LoginEventAccountLocked   LoginEventType = "account_locked"   // 账户锁定
	LoginEventAccountUnlocked LoginEventType = "account_unlocked" // 账户解锁
)

// AuthMethod 认证方式.
type AuthMethod string

// 认证方式常量.
const (
	AuthMethodPassword AuthMethod = "password" // 密码认证
	AuthMethodOTP      AuthMethod = "otp"      // 一次性密码
	AuthMethodTOTP     AuthMethod = "totp"     // 时间同步OTP
	AuthMethodWebAuthn AuthMethod = "webauthn" // WebAuthn
	AuthMethodLDAP     AuthMethod = "ldap"     // LDAP认证
	AuthMethodSSO      AuthMethod = "sso"      // 单点登录
	AuthMethodAPIKey   AuthMethod = "api_key"  // API密钥
	AuthMethodToken    AuthMethod = "token"    // Token认证
)

// LoginSession 登录会话信息.
type LoginSession struct {
	SessionID    string            `json:"session_id"`
	UserID       string            `json:"user_id"`
	Username     string            `json:"username"`
	IP           string            `json:"ip"`
	UserAgent    string            `json:"user_agent"`
	DeviceID     string            `json:"device_id,omitempty"`
	DeviceName   string            `json:"device_name,omitempty"`
	AuthMethod   AuthMethod        `json:"auth_method"`
	LoginTime    time.Time         `json:"login_time"`
	LastActivity time.Time         `json:"last_activity"`
	ExpiresAt    time.Time         `json:"expires_at"`
	IsActive     bool              `json:"is_active"`
	Location     *GeoLocation      `json:"location,omitempty"`
	RiskScore    int               `json:"risk_score"` // 0-100
	RiskFactors  []string          `json:"risk_factors"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// GeoLocation 地理位置.
type GeoLocation struct {
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	Region      string  `json:"region,omitempty"`
	City        string  `json:"city,omitempty"`
	Latitude    float64 `json:"latitude,omitempty"`
	Longitude   float64 `json:"longitude,omitempty"`
	Timezone    string  `json:"timezone,omitempty"`
	ISP         string  `json:"isp,omitempty"`
}

// LoginAuditEntry 登录审计条目.
type LoginAuditEntry struct {
	ID            string                 `json:"id"`
	Timestamp     time.Time              `json:"timestamp"`
	EventType     LoginEventType         `json:"event_type"`
	UserID        string                 `json:"user_id"`
	Username      string                 `json:"username"`
	IP            string                 `json:"ip"`
	UserAgent     string                 `json:"user_agent"`
	AuthMethod    AuthMethod             `json:"auth_method"`
	DeviceID      string                 `json:"device_id,omitempty"`
	DeviceName    string                 `json:"device_name,omitempty"`
	Location      *GeoLocation           `json:"location,omitempty"`
	Status        string                 `json:"status"` // success, failure
	FailureReason string                 `json:"failure_reason,omitempty"`
	SessionID     string                 `json:"session_id,omitempty"`
	PreviousIP    string                 `json:"previous_ip,omitempty"`    // 上次登录IP
	PreviousLogin *time.Time             `json:"previous_login,omitempty"` // 上次登录时间
	RiskScore     int                    `json:"risk_score"`
	RiskFactors   []string               `json:"risk_factors"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// LoginPattern 登录模式分析.
type LoginPattern struct {
	UserID             string    `json:"user_id"`
	Username           string    `json:"username"`
	TotalLogins        int       `json:"total_logins"`
	SuccessfulLogins   int       `json:"successful_logins"`
	FailedLogins       int       `json:"failed_logins"`
	UniqueIPs          int       `json:"unique_ips"`
	UniqueDevices      int       `json:"unique_devices"`
	UniqueLocations    int       `json:"unique_locations"`
	MostUsedIP         string    `json:"most_used_ip"`
	MostUsedDevice     string    `json:"most_used_device"`
	MostUsedLocation   string    `json:"most_used_location"`
	AvgSessionDuration int       `json:"avg_session_duration"` // 秒
	LastLoginTime      time.Time `json:"last_login_time"`
	LastLoginIP        string    `json:"last_login_ip"`
	AnomalousLogins    int       `json:"anomalous_logins"`
}

// ========== 操作审计类型 ==========

// OperationCategory 操作类别.
type OperationCategory string

// 操作类别常量.
const (
	OperationCategoryFile      OperationCategory = "file"      // 文件操作
	OperationCategoryUser      OperationCategory = "user"      // 用户管理
	OperationCategorySystem    OperationCategory = "system"    // 系统配置
	OperationCategoryNetwork   OperationCategory = "network"   // 网络配置
	OperationCategoryStorage   OperationCategory = "storage"   // 存储管理
	OperationCategorySecurity  OperationCategory = "security"  // 安全配置
	OperationCategoryBackup    OperationCategory = "backup"    // 备份恢复
	OperationCategoryShare     OperationCategory = "share"     // 共享管理
	OperationCategoryContainer OperationCategory = "container" // 容器管理
	OperationCategoryVM        OperationCategory = "vm"        // 虚拟机管理
)

// OperationAction 操作动作.
type OperationAction string

// 操作动作常量.
const (
	ActionCreate   OperationAction = "create"
	ActionRead     OperationAction = "read"
	ActionUpdate   OperationAction = "update"
	ActionDelete   OperationAction = "delete"
	ActionExecute  OperationAction = "execute"
	ActionMove     OperationAction = "move"
	ActionCopy     OperationAction = "copy"
	ActionRename   OperationAction = "rename"
	ActionDownload OperationAction = "download"
	ActionUpload   OperationAction = "upload"
	ActionShare    OperationAction = "share"
	ActionUnshare  OperationAction = "unshare"
	ActionEnable   OperationAction = "enable"
	ActionDisable  OperationAction = "disable"
	ActionStart    OperationAction = "start"
	ActionStop     OperationAction = "stop"
	ActionRestart  OperationAction = "restart"
)

// OperationAuditEntry 操作审计条目.
type OperationAuditEntry struct {
	ID               string                 `json:"id"`
	Timestamp        time.Time              `json:"timestamp"`
	CorrelationID    string                 `json:"correlation_id,omitempty"` // 关联ID，用于追踪关联操作
	UserID           string                 `json:"user_id"`
	Username         string                 `json:"username"`
	IP               string                 `json:"ip"`
	UserAgent        string                 `json:"user_agent"`
	SessionID        string                 `json:"session_id,omitempty"`
	Category         OperationCategory      `json:"category"`
	Action           OperationAction        `json:"action"`
	ResourceType     string                 `json:"resource_type"`           // 资源类型：file, user, volume等
	ResourceID       string                 `json:"resource_id"`             // 资源ID
	ResourceName     string                 `json:"resource_name"`           // 资源名称
	ResourcePath     string                 `json:"resource_path,omitempty"` // 资源路径
	OldValue         interface{}            `json:"old_value,omitempty"`
	NewValue         interface{}            `json:"new_value,omitempty"`
	Status           string                 `json:"status"` // success, failure, partial
	ErrorMessage     string                 `json:"error_message,omitempty"`
	Duration         int64                  `json:"duration,omitempty"` // 操作耗时(ms)
	IsSensitive      bool                   `json:"is_sensitive"`
	SensitivityLevel string                 `json:"sensitivity_level,omitempty"` // low, medium, high, critical
	RiskScore        int                    `json:"risk_score"`
	Details          map[string]interface{} `json:"details,omitempty"`
}

// OperationChain 操作链（用于追踪一系列相关操作）.
type OperationChain struct {
	CorrelationID string                 `json:"correlation_id"`
	UserID        string                 `json:"user_id"`
	Username      string                 `json:"username"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       *time.Time             `json:"end_time,omitempty"`
	TotalOps      int                    `json:"total_ops"`
	Operations    []*OperationAuditEntry `json:"operations"`
	Status        string                 `json:"status"` // ongoing, completed, failed
}

// ========== 敏感操作标记 ==========

// SensitivityLevel 敏感级别.
type SensitivityLevel string

// 敏感级别常量.
const (
	SensitivityLow      SensitivityLevel = "low"
	SensitivityMedium   SensitivityLevel = "medium"
	SensitivityHigh     SensitivityLevel = "high"
	SensitivityCritical SensitivityLevel = "critical"
)

// SensitiveOperation 敏感操作定义.
type SensitiveOperation struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Category         OperationCategory `json:"category"`
	Action           OperationAction   `json:"action"`
	ResourcePattern  string            `json:"resource_pattern,omitempty"` // 资源匹配模式
	SensitivityLevel SensitivityLevel  `json:"sensitivity_level"`
	RequiresMFA      bool              `json:"requires_mfa"`
	RequiresApproval bool              `json:"requires_approval"`          // 是否需要审批
	ApprovalTimeout  int               `json:"approval_timeout,omitempty"` // 审批超时(分钟)
	NotifyAdmins     bool              `json:"notify_admins"`
	NotifyUser       bool              `json:"notify_user"`
	LogDetails       bool              `json:"log_details"` // 是否记录详细变更
	Tags             []string          `json:"tags,omitempty"`
}

// SensitiveOperationEvent 敏感操作事件.
type SensitiveOperationEvent struct {
	ID            string                 `json:"id"`
	Timestamp     time.Time              `json:"timestamp"`
	OperationID   string                 `json:"operation_id"`
	OperationName string                 `json:"operation_name"`
	UserID        string                 `json:"user_id"`
	Username      string                 `json:"username"`
	IP            string                 `json:"ip"`
	SessionID     string                 `json:"session_id,omitempty"`
	Resource      string                 `json:"resource"`
	Details       map[string]interface{} `json:"details,omitempty"`
	Approved      bool                   `json:"approved"`
	ApprovedBy    string                 `json:"approved_by,omitempty"`
	ApprovedAt    *time.Time             `json:"approved_at,omitempty"`
	ApprovalNotes string                 `json:"approval_notes,omitempty"`
	Blocked       bool                   `json:"blocked"`
	BlockReason   string                 `json:"block_reason,omitempty"`
	RiskScore     int                    `json:"risk_score"`
}

// OperationApproval 操作审批请求.
type OperationApproval struct {
	ID            string                 `json:"id"`
	OperationID   string                 `json:"operation_id"`
	RequestedBy   string                 `json:"requested_by"`
	RequestorName string                 `json:"requestor_name"`
	RequestTime   time.Time              `json:"request_time"`
	Operation     *SensitiveOperation    `json:"operation"`
	Resource      string                 `json:"resource"`
	Details       map[string]interface{} `json:"details,omitempty"`
	Status        string                 `json:"status"` // pending, approved, rejected, expired
	ApprovedBy    string                 `json:"approved_by,omitempty"`
	ApprovedAt    *time.Time             `json:"approved_at,omitempty"`
	RejectedBy    string                 `json:"rejected_by,omitempty"`
	RejectedAt    *time.Time             `json:"rejected_at,omitempty"`
	RejectReason  string                 `json:"reject_reason,omitempty"`
	ExpiresAt     time.Time              `json:"expires_at"`
	Notes         string                 `json:"notes,omitempty"`
}

// ========== 审计报告类型 ==========

// AuditReportType 报告类型.
type AuditReportType string

// 报告类型常量.
const (
	ReportTypeLogin        AuditReportType = "login"         // 登录报告
	ReportTypeOperation    AuditReportType = "operation"     // 操作报告
	ReportTypeSensitive    AuditReportType = "sensitive"     // 敏感操作报告
	ReportTypeSecurity     AuditReportType = "security"      // 安全报告
	ReportTypeCompliance   AuditReportType = "compliance"    // 合规报告
	ReportTypeUserActivity AuditReportType = "user_activity" // 用户活动报告
	ReportTypeRiskAnalysis AuditReportType = "risk_analysis" // 风险分析报告
	ReportTypeExecutive    AuditReportType = "executive"     // 执行摘要报告
)

// AuditReport 审计报告.
type AuditReport struct {
	ReportID        string                 `json:"report_id"`
	ReportType      AuditReportType        `json:"report_type"`
	Title           string                 `json:"title"`
	Description     string                 `json:"description"`
	GeneratedAt     time.Time              `json:"generated_at"`
	GeneratedBy     string                 `json:"generated_by"`
	PeriodStart     time.Time              `json:"period_start"`
	PeriodEnd       time.Time              `json:"period_end"`
	Summary         *ReportSummary         `json:"summary"`
	LoginAnalysis   *LoginAnalysis         `json:"login_analysis,omitempty"`
	OperationStats  *OperationStatistics   `json:"operation_stats,omitempty"`
	SensitiveOps    *SensitiveOpsSummary   `json:"sensitive_ops,omitempty"`
	RiskAnalysis    *RiskAnalysis          `json:"risk_analysis,omitempty"`
	Recommendations []string               `json:"recommendations,omitempty"`
	ChartData       map[string]interface{} `json:"chart_data,omitempty"`
}

// ReportSummary 报告摘要.
type ReportSummary struct {
	TotalEvents      int                   `json:"total_events"`
	UniqueUsers      int                   `json:"unique_users"`
	UniqueIPs        int                   `json:"unique_ips"`
	SuccessfulOps    int                   `json:"successful_ops"`
	FailedOps        int                   `json:"failed_ops"`
	SensitiveOps     int                   `json:"sensitive_ops"`
	HighRiskEvents   int                   `json:"high_risk_events"`
	SecurityAlerts   int                   `json:"security_alerts"`
	EventsByCategory map[string]int        `json:"events_by_category"`
	EventsByDay      map[string]int        `json:"events_by_day"`
	TopUsers         []UserActivitySummary `json:"top_users"`
	TopOperations    []OperationCount      `json:"top_operations"`
}

// LoginAnalysis 登录分析.
type LoginAnalysis struct {
	TotalLogins        int                `json:"total_logins"`
	SuccessfulLogins   int                `json:"successful_logins"`
	FailedLogins       int                `json:"failed_logins"`
	UniqueUsers        int                `json:"unique_users"`
	UniqueIPs          int                `json:"unique_ips"`
	UniqueDevices      int                `json:"unique_devices"`
	AvgSessionDuration int                `json:"avg_session_duration"` // 分钟
	PeakLoginHours     []int              `json:"peak_login_hours"`
	TopLocations       []LocationCount    `json:"top_locations"`
	TopDevices         []DeviceCount      `json:"top_devices"`
	FailedByReason     map[string]int     `json:"failed_by_reason"`
	AnomalousLogins    []*LoginAuditEntry `json:"anomalous_logins,omitempty"`
	MFAUsageRate       float64            `json:"mfa_usage_rate"` // 百分比
}

// OperationStatistics 操作统计.
type OperationStatistics struct {
	TotalOperations   int                      `json:"total_operations"`
	SuccessfulOps     int                      `json:"successful_ops"`
	FailedOps         int                      `json:"failed_ops"`
	AvgDuration       int64                    `json:"avg_duration"` // ms
	OpsByCategory     map[string]int           `json:"ops_by_category"`
	OpsByAction       map[string]int           `json:"ops_by_action"`
	OpsByUser         []UserOperationCount     `json:"ops_by_user"`
	OpsByResource     []ResourceOperationCount `json:"ops_by_resource"`
	OpsByHour         map[int]int              `json:"ops_by_hour"`
	TopResources      []ResourceCount          `json:"top_resources"`
	SensitiveOpCount  int                      `json:"sensitive_op_count"`
	FailedOpsByReason map[string]int           `json:"failed_ops_by_reason"`
}

// SensitiveOpsSummary 敏感操作摘要.
type SensitiveOpsSummary struct {
	TotalSensitiveOps int                    `json:"total_sensitive_ops"`
	ApprovedOps       int                    `json:"approved_ops"`
	RejectedOps       int                    `json:"rejected_ops"`
	BlockedOps        int                    `json:"blocked_ops"`
	OpsByLevel        map[string]int         `json:"ops_by_level"`
	OpsByType         []SensitiveOpCount     `json:"ops_by_type"`
	TopUsers          []UserSensitiveOpCount `json:"top_users"`
	PendingApprovals  int                    `json:"pending_approvals"`
	AvgApprovalTime   int                    `json:"avg_approval_time"` // 分钟
}

// RiskAnalysis 风险分析.
type RiskAnalysis struct {
	OverallRiskScore  int                `json:"overall_risk_score"` // 0-100
	RiskLevel         string             `json:"risk_level"`         // low, medium, high, critical
	HighRiskEvents    int                `json:"high_risk_events"`
	HighRiskUsers     []UserRisk         `json:"high_risk_users"`
	HighRiskIPs       []IPRisk           `json:"high_risk_ips"`
	RiskTrends        []RiskTrend        `json:"risk_trends"`
	AnomalyScore      int                `json:"anomaly_score"`
	ThreatIndicators  []ThreatIndicator  `json:"threat_indicators"`
	MitigationActions []MitigationAction `json:"mitigation_actions"`
}

// UserActivitySummary 用户活动摘要.
type UserActivitySummary struct {
	UserID         string `json:"user_id"`
	Username       string `json:"username"`
	EventCount     int    `json:"event_count"`
	LoginCount     int    `json:"login_count"`
	OperationCount int    `json:"operation_count"`
	RiskScore      int    `json:"risk_score"`
}

// OperationCount 操作计数.
type OperationCount struct {
	Operation string `json:"operation"`
	Count     int    `json:"count"`
}

// LocationCount 地理位置计数.
type LocationCount struct {
	Location string `json:"location"`
	Count    int    `json:"count"`
}

// DeviceCount 设备计数.
type DeviceCount struct {
	Device string `json:"device"`
	Count  int    `json:"count"`
}

// UserOperationCount 用户操作计数.
type UserOperationCount struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Count    int    `json:"count"`
}

// ResourceOperationCount 资源操作计数.
type ResourceOperationCount struct {
	ResourceType string `json:"resource_type"`
	Count        int    `json:"count"`
}

// ResourceCount 资源计数.
type ResourceCount struct {
	Resource string `json:"resource"`
	Count    int    `json:"count"`
}

// SensitiveOpCount 敏感操作计数.
type SensitiveOpCount struct {
	OperationName string `json:"operation_name"`
	Count         int    `json:"count"`
}

// UserSensitiveOpCount 用户敏感操作计数.
type UserSensitiveOpCount struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Count    int    `json:"count"`
}

// UserRisk 用户风险.
type UserRisk struct {
	UserID      string   `json:"user_id"`
	Username    string   `json:"username"`
	RiskScore   int      `json:"risk_score"`
	RiskFactors []string `json:"risk_factors"`
}

// IPRisk IP风险.
type IPRisk struct {
	IP          string   `json:"ip"`
	RiskScore   int      `json:"risk_score"`
	RiskFactors []string `json:"risk_factors"`
	EventCount  int      `json:"event_count"`
}

// RiskTrend 风险趋势.
type RiskTrend struct {
	Date       string `json:"date"`
	RiskScore  int    `json:"risk_score"`
	EventCount int    `json:"event_count"`
}

// ThreatIndicator 威胁指标.
type ThreatIndicator struct {
	Type        string   `json:"type"` // brute_force, credential_stuffing, privilege_escalation, etc.
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
	Count       int      `json:"count"`
	Sources     []string `json:"sources"`
}

// MitigationAction 缓解措施.
type MitigationAction struct {
	Priority    int    `json:"priority"`
	Action      string `json:"action"`
	Description string `json:"description"`
	Status      string `json:"status"` // pending, in_progress, completed
}

// ========== 查询选项 ==========

// LoginQueryOptions 登录审计查询选项.
type LoginQueryOptions struct {
	Limit        int            `json:"limit"`
	Offset       int            `json:"offset"`
	StartTime    *time.Time     `json:"start_time,omitempty"`
	EndTime      *time.Time     `json:"end_time,omitempty"`
	UserID       string         `json:"user_id,omitempty"`
	Username     string         `json:"username,omitempty"`
	IP           string         `json:"ip,omitempty"`
	EventType    LoginEventType `json:"event_type,omitempty"`
	AuthMethod   AuthMethod     `json:"auth_method,omitempty"`
	Status       string         `json:"status,omitempty"`
	MinRiskScore int            `json:"min_risk_score,omitempty"`
}

// OperationQueryOptions 操作审计查询选项.
type OperationQueryOptions struct {
	Limit            int               `json:"limit"`
	Offset           int               `json:"offset"`
	StartTime        *time.Time        `json:"start_time,omitempty"`
	EndTime          *time.Time        `json:"end_time,omitempty"`
	UserID           string            `json:"user_id,omitempty"`
	Username         string            `json:"username,omitempty"`
	IP               string            `json:"ip,omitempty"`
	Category         OperationCategory `json:"category,omitempty"`
	Action           OperationAction   `json:"action,omitempty"`
	ResourceType     string            `json:"resource_type,omitempty"`
	ResourceID       string            `json:"resource_id,omitempty"`
	Status           string            `json:"status,omitempty"`
	IsSensitive      *bool             `json:"is_sensitive,omitempty"`
	SensitivityLevel SensitivityLevel  `json:"sensitivity_level,omitempty"`
	CorrelationID    string            `json:"correlation_id,omitempty"`
	MinRiskScore     int               `json:"min_risk_score,omitempty"`
}

// ReportGenerateOptions 报告生成选项.
type ReportGenerateOptions struct {
	ReportType     AuditReportType     `json:"report_type"`
	PeriodStart    time.Time           `json:"period_start"`
	PeriodEnd      time.Time           `json:"period_end"`
	UserIDs        []string            `json:"user_ids,omitempty"`
	Categories     []OperationCategory `json:"categories,omitempty"`
	IncludeDetails bool                `json:"include_details"`
	Format         string              `json:"format"` // json, pdf, html
	Language       string              `json:"language"`
}

// ========== API响应类型 ==========

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

// 错误码定义.
const (
	ErrCodeInvalidParam  = 400
	ErrCodeNotFound      = 404
	ErrCodeInternalError = 500
	ErrCodeUnauthorized  = 401
	ErrCodeForbidden     = 403
)
