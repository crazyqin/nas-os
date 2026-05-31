// Package privacyimpact 实现隐私影响评估模块
// 支持数据操作的隐私风险评估、个人敏感信息检测、合规检查
// 提供隐私风险评分、数据流向追踪、审计日志和合规报告生成
package privacyimpact

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrNotRunning 评估器未运行.
	ErrNotRunning = errors.New("privacy impact assessor not running")
	// ErrAssessmentNotFound 评估报告不存在.
	ErrAssessmentNotFound = errors.New("assessment not found")
	// ErrInvalidConfig 配置无效.
	ErrInvalidConfig = errors.New("invalid configuration")
	// ErrAlreadyRunning 评估器已在运行.
	ErrAlreadyRunning = errors.New("privacy impact assessor already running")
	// ErrInvalidOperation 无效的数据操作.
	ErrInvalidOperation = errors.New("invalid data operation")
	// ErrDataNotFound 数据未找到.
	ErrDataNotFound = errors.New("data not found")
)

// ========== 数据操作类型 ==========

// DataOperation 数据操作类型.
type DataOperation string

const (
	// OpUpload 数据上传.
	OpUpload DataOperation = "upload"
	// OpShare 数据分享.
	OpShare DataOperation = "share"
	// OpExport 数据导出.
	OpExport DataOperation = "export"
	// OpDownload 数据下载.
	OpDownload DataOperation = "download"
	// OpDelete 数据删除.
	OpDelete DataOperation = "delete"
	// OpProcess 数据处理.
	OpProcess DataOperation = "process"
	// OpTransfer 数据传输.
	OpTransfer DataOperation = "transfer"
)

// ========== 风险等级 ==========

// RiskLevel 风险等级.
type RiskLevel string

const (
	// RiskCritical 严重风险.
	RiskCritical RiskLevel = "critical"
	// RiskHigh 高风险.
	RiskHigh RiskLevel = "high"
	// RiskMedium 中风险.
	RiskMedium RiskLevel = "medium"
	// RiskLow 低风险.
	RiskLow RiskLevel = "low"
	// RiskNone 无风险.
	RiskNone RiskLevel = "none"
)

// ========== 敏感信息类型 ==========

// PIIType 个人敏感信息类型.
type PIIType string

const (
	// PIIIDCard 身份证号.
	PIIIDCard PIIType = "id_card"
	// PIIPhone 手机号.
	PIIPhone PIIType = "phone"
	// PIIEmail 邮箱.
	PIIEmail PIIType = "email"
	// PIIBankCard 银行卡号.
	PIIBankCard PIIType = "bank_card"
	// PIIAddress 地址.
	PIIAddress PIIType = "address"
	// PIIName 姓名.
	PIIName PIIType = "name"
	// PIIPassport 护照号.
	PIIPassport PIIType = "passport"
	// PIIDriverLicense 驾驶证号.
	PIIDriverLicense PIIType = "driver_license"
	// PIISocialSecurity 社保号.
	PIISocialSecurity PIIType = "social_security"
	// PIIIP IP地址.
	PIIIP PIIType = "ip_address"
)

// ========== 合规框架 ==========

// ComplianceFramework 合规框架.
type ComplianceFramework string

const (
	// FrameworkGDPR GDPR通用数据保护条例.
	FrameworkGDPR ComplianceFramework = "GDPR"
	// FrameworkPIPL 个人信息保护法.
	FrameworkPIPL ComplianceFramework = "PIPL"
	// FrameworkCCPA CCPA加州消费者隐私法.
	FrameworkCCPA ComplianceFramework = "CCPA"
)

// ========== 合规状态 ==========

// ComplianceStatus 合规状态.
type ComplianceStatus string

const (
	// StatusCompliant 合规.
	StatusCompliant ComplianceStatus = "compliant"
	// StatusNonCompliant 不合规.
	StatusNonCompliant ComplianceStatus = "non_compliant"
	// StatusPartial 部分合规.
	StatusPartial ComplianceStatus = "partial"
	// StatusInProgress 进行中.
	StatusInProgress ComplianceStatus = "in_progress"
)

// ========== 评估状态 ==========

// AssessmentStatus 评估状态.
type AssessmentStatus string

const (
	// AssessmentPending 待评估.
	AssessmentPending AssessmentStatus = "pending"
	// AssessmentInProgress 评估中.
	AssessmentInProgress AssessmentStatus = "in_progress"
	// AssessmentCompleted 评估完成.
	AssessmentCompleted AssessmentStatus = "completed"
	// AssessmentFailed 评估失败.
	AssessmentFailed AssessmentStatus = "failed"
)

// ========== 核心配置 ==========

// Config 隐私影响评估配置.
type Config struct {
	// Enabled 是否启用.
	Enabled bool `json:"enabled"`
	// AutoAssess 是否自动评估.
	AutoAssess bool `json:"autoAssess"`
	// RiskThreshold 风险阈值（高于此值触发告警）.
	RiskThreshold float64 `json:"riskThreshold"`
	// EnabledFrameworks 启用的合规框架.
	EnabledFrameworks []ComplianceFramework `json:"enabledFrameworks"`
	// ScanPaths 数据扫描路径.
	ScanPaths []string `json:"scanPaths"`
	// NotifyEmail 通知邮箱.
	NotifyEmail string `json:"notifyEmail"`
	// RetentionDays 评估报告保留天数.
	RetentionDays int `json:"retentionDays"`
	// MaxAuditLogSize 最大审计日志条数.
	MaxAuditLogSize int `json:"maxAuditLogSize"`
	// DataFlowTrackingEnabled 是否启用数据流向追踪.
	DataFlowTrackingEnabled bool `json:"dataFlowTrackingEnabled"`
}

// ========== 隐私风险评估 ==========

// PrivacyAssessment 隐私风险评估报告.
type PrivacyAssessment struct {
	// ID 评估ID.
	ID string `json:"id"`
	// Title 评估标题.
	Title string `json:"title"`
	// Operation 数据操作类型.
	Operation DataOperation `json:"operation"`
	// DataType 数据类型描述.
	DataType string `json:"dataType"`
	// DataDescription 数据描述.
	DataDescription string `json:"dataDescription"`
	// Source 来源.
	Source string `json:"source"`
	// Destination 目标.
	Destination string `json:"destination"`
	// RiskScore 风险评分 (0-100).
	RiskScore float64 `json:"riskScore"`
	// RiskLevel 风险等级.
	RiskLevel RiskLevel `json:"riskLevel"`
	// Status 评估状态.
	Status AssessmentStatus `json:"status"`
	// PIIDetected 检测到的PII类型.
	PIIDetected []PIIType `json:"piiDetected"`
	// ComplianceResults 合规检查结果.
	ComplianceResults []ComplianceResult `json:"complianceResults"`
	// DataFlow 数据流向记录.
	DataFlow *DataFlowRecord `json:"dataFlow,omitempty"`
	// Recommendations 建议列表.
	Recommendations []Recommendation `json:"recommendations"`
	// AssessedAt 评估时间.
	AssessedAt time.Time `json:"assessedAt"`
	// AssessedBy 评估者.
	AssessedBy string `json:"assessedBy"`
	// Metadata 附加元数据.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ========== PII检测结果 ==========

// PIIDetectionResult PII检测结果.
type PIIDetectionResult struct {
	// ID 结果ID.
	ID string `json:"id"`
	// ScanTime 扫描时间.
	ScanTime time.Time `json:"scanTime"`
	// TotalFiles 扫描文件总数.
	TotalFiles int `json:"totalFiles"`
	// SensitiveFiles 敏感文件数.
	SensitiveFiles int `json:"sensitiveFiles"`
	// Findings PII发现列表.
	Findings []PIIFinding `json:"findings"`
	// Summary PII类型统计.
	Summary map[PIIType]int `json:"summary"`
}

// PIIFinding PII发现项.
type PIIFinding struct {
	// ID 发现ID.
	ID string `json:"id"`
	// FilePath 文件路径.
	FilePath string `json:"filePath"`
	// PIIType PII类型.
	PIIType PIIType `json:"piiType"`
	// MatchCount 匹配数量.
	MatchCount int `json:"matchCount"`
	// Sample 样本（脱敏）.
	Sample string `json:"sample"`
	// LineNumber 行号.
	LineNumber int `json:"lineNumber"`
	// RiskLevel 风险等级.
	RiskLevel RiskLevel `json:"riskLevel"`
	// DetectedAt 检测时间.
	DetectedAt time.Time `json:"detectedAt"`
}

// ========== 合规检查结果 ==========

// ComplianceResult 合规检查结果.
type ComplianceResult struct {
	// Framework 合规框架.
	Framework ComplianceFramework `json:"framework"`
	// Status 合规状态.
	Status ComplianceStatus `json:"status"`
	// Score 合规分数 (0-100).
	Score float64 `json:"score"`
	// Checks 检查项列表.
	Checks []ComplianceCheck `json:"checks"`
	// Violations 违规项列表.
	Violations []ComplianceViolation `json:"violations"`
	// CheckedAt 检查时间.
	CheckedAt time.Time `json:"checkedAt"`
}

// ComplianceCheck 合规检查项.
type ComplianceCheck struct {
	// ID 检查项ID.
	ID string `json:"id"`
	// Name 检查项名称.
	Name string `json:"name"`
	// Description 描述.
	Description string `json:"description"`
	// Status 状态.
	Status ComplianceStatus `json:"status"`
	// Score 分数.
	Score float64 `json:"score"`
	// MaxScore 满分.
	MaxScore float64 `json:"maxScore"`
	// Evidence 证据.
	Evidence []string `json:"evidence"`
}

// ComplianceViolation 合规违规项.
type ComplianceViolation struct {
	// ID 违规项ID.
	ID string `json:"id"`
	// Framework 合规框架.
	Framework ComplianceFramework `json:"framework"`
	// Article 条款.
	Article string `json:"article"`
	// Title 标题.
	Title string `json:"title"`
	// Description 描述.
	Description string `json:"description"`
	// Severity 严重程度.
	Severity RiskLevel `json:"severity"`
	// Remediation 整改建议.
	Remediation string `json:"remediation"`
}

// ========== 数据流向追踪 ==========

// DataFlowRecord 数据流向记录.
type DataFlowRecord struct {
	// ID 记录ID.
	ID string `json:"id"`
	// Operation 数据操作.
	Operation DataOperation `json:"operation"`
	// Source 来源.
	Source DataEndpoint `json:"source"`
	// Destination 目标.
	Destination DataEndpoint `json:"destination"`
	// DataCategories 数据分类.
	DataCategories []string `json:"dataCategories"`
	// Volume 数据量（字节）.
	Volume int64 `json:"volume"`
	// Encrypted 是否加密传输.
	Encrypted bool `json:"encrypted"`
	// Timestamp 时间戳.
	Timestamp time.Time `json:"timestamp"`
	// UserID 操作用户ID.
	UserID string `json:"userId"`
	// UserName 操作用户名.
	UserName string `json:"userName"`
	// IPAddress 操作IP.
	IPAddress string `json:"ipAddress"`
	// Status 状态.
	Status string `json:"status"`
}

// DataEndpoint 数据端点.
type DataEndpoint struct {
	// Type 端点类型 (local/remote/cloud/external).
	Type string `json:"type"`
	// Location 位置.
	Location string `json:"location"`
	// Country 国家/地区.
	Country string `json:"country"`
	// Organization 组织.
	Organization string `json:"organization"`
}

// ========== 建议 ==========

// Recommendation 隐私保护建议.
type Recommendation struct {
	// ID 建议ID.
	ID string `json:"id"`
	// Category 分类.
	Category string `json:"category"`
	// Title 标题.
	Title string `json:"title"`
	// Description 描述.
	Description string `json:"description"`
	// Priority 优先级 (1-5, 1最高).
	Priority int `json:"priority"`
	// EffortLevel 工作量.
	EffortLevel string `json:"effortLevel"`
	// ImpactLevel 影响级别.
	ImpactLevel string `json:"impactLevel"`
	// Steps 整改步骤.
	Steps []string `json:"steps"`
}

// ========== 审计日志 ==========

// AuditEvent 隐私审计事件.
type AuditEvent struct {
	// ID 事件ID.
	ID string `json:"id"`
	// Timestamp 时间戳.
	Timestamp time.Time `json:"timestamp"`
	// UserID 操作用户ID.
	UserID string `json:"userId"`
	// UserName 操作用户名.
	UserName string `json:"userName"`
	// Action 操作类型.
	Action string `json:"action"`
	// Resource 资源.
	Resource string `json:"resource"`
	// ResourceID 资源ID.
	ResourceID string `json:"resourceId"`
	// Details 详情.
	Details string `json:"details"`
	// IPAddress IP地址.
	IPAddress string `json:"ipAddress"`
	// Result 结果 (success/failure/denied).
	Result string `json:"result"`
	// RiskLevel 风险等级.
	RiskLevel RiskLevel `json:"riskLevel"`
	// AssessmentID 关联评估ID.
	AssessmentID string `json:"assessmentId,omitempty"`
}

// ========== 统计数据 ==========

// PrivacyStats 隐私评估统计.
type PrivacyStats struct {
	// TotalAssessments 总评估数.
	TotalAssessments int `json:"totalAssessments"`
	// CompletedAssessments 已完成评估数.
	CompletedAssessments int `json:"completedAssessments"`
	// PendingAssessments 待评估数.
	PendingAssessments int `json:"pendingAssessments"`
	// AverageRiskScore 平均风险评分.
	AverageRiskScore float64 `json:"averageRiskScore"`
	// HighRiskCount 高风险评估数.
	HighRiskCount int `json:"highRiskCount"`
	// CriticalRiskCount 严重风险评估数.
	CriticalRiskCount int `json:"criticalRiskCount"`
	// TotalPIIDetected 检测到的PII总数.
	TotalPIIDetected int `json:"totalPiiDetected"`
	// PIIByType 按类型统计PII.
	PIIByType map[PIIType]int `json:"piiByType"`
	// ComplianceScores 各框架合规分数.
	ComplianceScores map[ComplianceFramework]float64 `json:"complianceScores"`
	// TotalAuditEvents 审计事件总数.
	TotalAuditEvents int `json:"totalAuditEvents"`
	// DataFlowRecords 数据流向记录数.
	DataFlowRecords int `json:"dataFlowRecords"`
	// LastAssessmentTime 最后评估时间.
	LastAssessmentTime time.Time `json:"lastAssessmentTime"`
}

// ========== 合规报告 ==========

// ComplianceReport 隐私合规报告.
type ComplianceReport struct {
	// ID 报告ID.
	ID string `json:"id"`
	// Title 标题.
	Title string `json:"title"`
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generatedAt"`
	// ValidUntil 有效期至.
	ValidUntil time.Time `json:"validUntil"`
	// GeneratedBy 生成者.
	GeneratedBy string `json:"generatedBy"`
	// OverallScore 总体合规分.
	OverallScore float64 `json:"overallScore"`
	// RiskLevel 总体风险等级.
	RiskLevel RiskLevel `json:"riskLevel"`
	// FrameworkScores 各框架分数.
	FrameworkScores map[ComplianceFramework]float64 `json:"frameworkScores"`
	// TotalAssessments 评估总数.
	TotalAssessments int `json:"totalAssessments"`
	// ViolationsTotal 违规总数.
	ViolationsTotal int `json:"violationsTotal"`
	// ViolationsCritical 严重违规数.
	ViolationsCritical int `json:"violationsCritical"`
	// Recommendations 建议列表.
	Recommendations []Recommendation `json:"recommendations"`
	// Summary 摘要.
	Summary string `json:"summary"`
	// Content 报告内容.
	Content string `json:"content,omitempty"`
}
