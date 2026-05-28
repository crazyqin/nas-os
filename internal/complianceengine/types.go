// Package complianceengine 提供多标准合规检查引擎功能
// 支持 CIS、STIG、GDPR、等保2.0、PCI-DSS 等合规标准的自动化扫描、评分、报告生成和修复跟踪
package complianceengine

import (
	"sync"
	"time"
)

// ComplianceStandard 合规标准类型
type ComplianceStandard string

const (
	StandardCIS    ComplianceStandard = "cis"     // CIS 基准
	StandardSTIG   ComplianceStandard = "stig"    // STIG 安全技术实施指南
	StandardGDPR   ComplianceStandard = "gdpr"    // GDPR 通用数据保护条例
	StandardMLPS2  ComplianceStandard = "mlps2"   // 等保2.0
	StandardPCIDSS ComplianceStandard = "pci-dss" // PCI-DSS 支付卡行业数据安全标准
	StandardCustom ComplianceStandard = "custom"  // 自定义规则
)

// SeverityLevel 严重程度
type SeverityLevel string

const (
	SeverityCritical SeverityLevel = "critical" // 严重
	SeverityHigh     SeverityLevel = "high"     // 高
	SeverityMedium   SeverityLevel = "medium"   // 中
	SeverityLow      SeverityLevel = "low"      // 低
	SeverityInfo     SeverityLevel = "info"     // 信息
)

// RuleCategory 规则类别
type RuleCategory string

const (
	CategoryAccessControl   RuleCategory = "access_control"   // 访问控制
	CategoryAuditLogging    RuleCategory = "audit_logging"    // 审计日志
	CategoryDataProtection  RuleCategory = "data_protection"  // 数据保护
	CategoryNetworkSecurity RuleCategory = "network_security" // 网络安全
	CategorySystemHardening RuleCategory = "system_hardening" // 系统加固
	CategoryIdentityAuth    RuleCategory = "identity_auth"    // 身份认证
	CategoryEncryption      RuleCategory = "encryption"       // 加密
	CategoryBackupRecovery  RuleCategory = "backup_recovery"  // 备份恢复
	CategoryIncidentResponse RuleCategory = "incident_response" // 事件响应
	CategoryPrivacyControl  RuleCategory = "privacy_control"  // 隐私控制
	CategoryPaymentSecurity RuleCategory = "payment_security" // 支付安全
)

// ScanStatus 扫描状态
type ScanStatus string

const (
	StatusPending   ScanStatus = "pending"   // 待执行
	StatusRunning   ScanStatus = "running"   // 运行中
	StatusCompleted ScanStatus = "completed" // 已完成
	StatusFailed    ScanStatus = "failed"    // 失败
	StatusCancelled ScanStatus = "cancelled" // 已取消
)

// CheckResult 检查结果
type CheckResult string

const (
	ResultPass    CheckResult = "pass"    // 通过
	ResultFail    CheckResult = "fail"    // 不通过
	ResultWarning CheckResult = "warning" // 警告
	ResultSkip    CheckResult = "skip"    // 跳过
	ResultError   CheckResult = "error"   // 错误
)

// ReportFormat 报告格式
type ReportFormat string

const (
	FormatJSON ReportFormat = "json" // JSON 格式
	FormatHTML ReportFormat = "html" // HTML 格式
	FormatPDF  ReportFormat = "pdf"  // PDF 格式
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"    // 待处理
	TaskInProgress TaskStatus = "in_progress" // 进行中
	TaskCompleted  TaskStatus = "completed"  // 已完成
	TaskFailed     TaskStatus = "failed"     // 失败
	TaskCancelled  TaskStatus = "cancelled"  // 已取消
)

// AlertSeverity 告警级别
type AlertSeverity string

const (
	AlertCritical AlertSeverity = "critical" // 严重
	AlertHigh     AlertSeverity = "high"     // 高
	AlertMedium   AlertSeverity = "medium"   // 中
	AlertLow      AlertSeverity = "low"      // 低
)

// ComplianceRule 合规规则定义
type ComplianceRule struct {
	ID          string             `json:"id"`                   // 规则ID
	Standard    ComplianceStandard `json:"standard"`             // 合规标准
	Category    RuleCategory       `json:"category"`             // 规则类别
	Severity    SeverityLevel      `json:"severity"`             // 严重程度
	Title       string             `json:"title"`                // 规则标题
	Description string             `json:"description"`          // 规则描述
	Requirement string             `json:"requirement"`          // 合规要求
	Remediation string             `json:"remediation"`          // 修复建议
	Enabled     bool               `json:"enabled"`              // 是否启用
	IsCustom    bool               `json:"isCustom"`             // 是否自定义规则
	Tags        []string           `json:"tags,omitempty"`       // 标签
	References  []string           `json:"references,omitempty"` // 参考链接
	CreatedAt   time.Time          `json:"createdAt"`            // 创建时间
	UpdatedAt   time.Time          `json:"updatedAt"`            // 更新时间
}

// CheckDetail 检查详情
type CheckDetail struct {
	RuleID        string        `json:"ruleId"`                  // 规则ID
	Result        CheckResult   `json:"result"`                  // 检查结果
	Message       string        `json:"message"`                 // 检查信息
	Evidence      string        `json:"evidence,omitempty"`      // 证据
	ActualValue   string        `json:"actualValue,omitempty"`   // 实际值
	ExpectedValue string        `json:"expectedValue,omitempty"` // 期望值
	CheckedAt     time.Time     `json:"checkedAt"`               // 检查时间
	Duration      time.Duration `json:"duration"`                // 检查耗时
}

// ComplianceScan 合规扫描任务
type ComplianceScan struct {
	ID          string               `json:"id"`             // 扫描ID
	Standards   []ComplianceStandard `json:"standards"`      // 扫描的标准
	Status      ScanStatus           `json:"status"`         // 扫描状态
	StartTime   time.Time            `json:"startTime"`      // 开始时间
	EndTime     time.Time            `json:"endTime"`        // 结束时间
	Duration    time.Duration        `json:"duration"`       // 扫描耗时
	TotalRules  int                  `json:"totalRules"`     // 总规则数
	PassedRules int                  `json:"passedRules"`    // 通过规则数
	FailedRules int                  `json:"failedRules"`    // 未通过规则数
	WarnRules   int                  `json:"warnRules"`      // 警告规则数
	SkipRules   int                  `json:"skipRules"`      // 跳过规则数
	ErrorRules  int                  `json:"errorRules"`     // 错误规则数
	Score       float64              `json:"score"`          // 合规分数 (0-100)
	Checks      []CheckDetail        `json:"checks"`         // 检查详情
	Errors      []ScanError          `json:"errors,omitempty"` // 扫描错误
}

// ScanError 扫描错误
type ScanError struct {
	RuleID  string `json:"ruleId"`            // 规则ID
	Error   string `json:"error"`             // 错误信息
	Details string `json:"details,omitempty"` // 详细信息
}

// GapAnalysis 差距分析
type GapAnalysis struct {
	ID          string               `json:"id"`           // 分析ID
	Standards   []ComplianceStandard `json:"standards"`    // 分析的标准
	GeneratedAt time.Time            `json:"generatedAt"`  // 生成时间
	Score       float64              `json:"score"`        // 总体合规分数
	Categories  []CategoryGap        `json:"categories"`   // 分类差距
	Gaps        []ComplianceGap      `json:"gaps"`         // 合规差距列表
	Actions     []RecommendedAction  `json:"actions"`      // 推荐操作
}

// CategoryGap 分类差距
type CategoryGap struct {
	Category    RuleCategory `json:"category"`    // 规则类别
	Score       float64      `json:"score"`       // 分类分数
	TotalChecks int          `json:"totalChecks"` // 总检查数
	Passed      int          `json:"passed"`      // 通过数
	Failed      int          `json:"failed"`      // 未通过数
	GapCount    int          `json:"gapCount"`    // 差距数量
}

// ComplianceGap 合规差距
type ComplianceGap struct {
	RuleID      string        `json:"ruleId"`      // 规则ID
	Standard    ComplianceStandard `json:"standard"` // 合规标准
	Severity    SeverityLevel `json:"severity"`    // 严重程度
	Title       string        `json:"title"`       // 差距标题
	Description string        `json:"description"` // 差距描述
	Current     string        `json:"current"`     // 当前状态
	Required    string        `json:"required"`    // 要求状态
	Impact      string        `json:"impact"`      // 影响
}

// RecommendedAction 推荐操作
type RecommendedAction struct {
	ID          string        `json:"id"`          // 操作ID
	Priority    SeverityLevel `json:"priority"`    // 优先级
	RuleID      string        `json:"ruleId"`      // 关联规则ID
	Title       string        `json:"title"`       // 操作标题
	Description string        `json:"description"` // 操作描述
	Action      string        `json:"action"`      // 具体操作
	Effort      string        `json:"effort"`      // 工作量评估
	AutoFixable bool          `json:"autoFixable"` // 是否可自动修复
}

// ComplianceReport 合规报告
type ComplianceReport struct {
	ID          string               `json:"id"`                // 报告ID
	Title       string               `json:"title"`             // 报告标题
	Format      ReportFormat         `json:"format"`            // 报告格式
	ScanID      string               `json:"scanId"`            // 关联扫描ID
	Standards   []ComplianceStandard `json:"standards"`         // 合规标准
	GeneratedAt time.Time            `json:"generatedAt"`       // 生成时间
	Summary     ReportSummary        `json:"summary"`           // 摘要
	GapAnalysis *GapAnalysis         `json:"gapAnalysis,omitempty"` // 差距分析
	Trends      *TrendData           `json:"trends,omitempty"`  // 趋势数据
	Content     []byte               `json:"-"`                 // 报告内容（二进制）
	ContentURL  string               `json:"contentUrl,omitempty"` // 报告下载链接
}

// ReportSummary 报告摘要
type ReportSummary struct {
	ComplianceScore float64 `json:"complianceScore"` // 合规分数
	TotalChecks     int     `json:"totalChecks"`     // 总检查数
	PassedChecks    int     `json:"passedChecks"`    // 通过数
	FailedChecks    int     `json:"failedChecks"`    // 未通过数
	WarningChecks   int     `json:"warningChecks"`   // 警告数
	CriticalFindings int   `json:"criticalFindings"` // 严重发现数
	HighFindings    int     `json:"highFindings"`    // 高危发现数
	MediumFindings  int     `json:"mediumFindings"`  // 中危发现数
	LowFindings     int     `json:"lowFindings"`     // 低危发现数
}

// TrendData 趋势数据
type TrendData struct {
	History     []TrendPoint `json:"history"`     // 历史数据点
	Trend       string       `json:"trend"`       // 趋势方向 (up/down/stable)
	ChangeRate  float64      `json:"changeRate"`  // 变化率
	PeriodDays  int          `json:"periodDays"`  // 统计周期（天）
}

// TrendPoint 趋势数据点
type TrendPoint struct {
	Date   time.Time `json:"date"`   // 日期
	Score  float64   `json:"score"`  // 分数
	ScanID string    `json:"scanId"` // 扫描ID
}

// ComplianceAlert 合规告警
type ComplianceAlert struct {
	ID        string        `json:"id"`        // 告警ID
	RuleID    string        `json:"ruleId"`    // 关联规则ID
	Severity  AlertSeverity `json:"severity"`  // 告警级别
	Title     string        `json:"title"`     // 告警标题
	Message   string        `json:"message"`   // 告警内容
	Status    string        `json:"status"`    // 告警状态 (active/acknowledged/resolved)
	CreatedAt time.Time     `json:"createdAt"` // 创建时间
	UpdatedAt time.Time     `json:"updatedAt"` // 更新时间
	Notified  bool          `json:"notified"`  // 是否已通知
}

// RemediationTask 修复任务
type RemediationTask struct {
	ID          string           `json:"id"`              // 任务ID
	RuleID      string           `json:"ruleId"`          // 关联规则ID
	Title       string           `json:"title"`           // 任务标题
	Description string           `json:"description"`     // 任务描述
	Assignee    string           `json:"assignee"`        // 负责人
	Priority    SeverityLevel    `json:"priority"`        // 优先级
	Status      TaskStatus       `json:"status"`          // 任务状态
	Commands    []string         `json:"commands"`        // 修复命令
	AutoFix     bool             `json:"autoFix"`         // 是否可自动修复
	DueDate     *time.Time       `json:"dueDate,omitempty"` // 截止日期
	CreatedAt   time.Time        `json:"createdAt"`       // 创建时间
	UpdatedAt   time.Time        `json:"updatedAt"`       // 更新时间
	CompletedAt *time.Time       `json:"completedAt,omitempty"` // 完成时间
	Result      string           `json:"result,omitempty"` // 执行结果
}

// NotificationConfig 通知配置
type NotificationConfig struct {
	Enabled    bool     `json:"enabled"`    // 是否启用通知
	Channels   []string `json:"channels"`   // 通知渠道 (email/webhook/console)
	Recipients []string `json:"recipients"` // 接收人
	WebhookURL string   `json:"webhookUrl"` // Webhook URL
	MinSeverity AlertSeverity `json:"minSeverity"` // 最低告警级别
}

// EngineConfig 引擎配置
type EngineConfig struct {
	Enabled         bool               `json:"enabled"`         // 是否启用
	ScanInterval    time.Duration      `json:"scanInterval"`    // 扫描间隔
	AutoScan        bool               `json:"autoScan"`        // 是否自动扫描
	AutoRemediate   bool               `json:"autoRemediate"`   // 是否自动修复
	MaxConcurrent   int                `json:"maxConcurrent"`   // 最大并发扫描数
	ReportRetention int                `json:"reportRetention"` // 报告保留天数
	Notification    NotificationConfig `json:"notification"`    // 通知配置
	EnabledStandards []ComplianceStandard `json:"enabledStandards"` // 启用的标准
}

// ComplianceStats 合规统计
type ComplianceStats struct {
	mu sync.RWMutex `json:"-"`

	TotalScans      int64      `json:"totalScans"`      // 总扫描次数
	SuccessfulScans int64      `json:"successfulScans"` // 成功扫描数
	FailedScans     int64      `json:"failedScans"`     // 失败扫描数
	TotalRules      int        `json:"totalRules"`      // 总规则数
	EnabledRules    int        `json:"enabledRules"`    // 启用规则数
	CustomRules     int        `json:"customRules"`     // 自定义规则数
	LastScanTime    *time.Time `json:"lastScanTime,omitempty"` // 最近扫描时间
	LastScanStatus  ScanStatus `json:"lastScanStatus"`  // 最近扫描状态
	CurrentScore    float64    `json:"currentScore"`    // 当前合规分数
	PreviousScore   float64    `json:"previousScore"`   // 上次合规分数
	ScoreTrend      string     `json:"scoreTrend"`      // 分数趋势
	TotalAlerts     int64      `json:"totalAlerts"`     // 总告警数
	ActiveAlerts    int64      `json:"activeAlerts"`    // 活跃告警数
	TotalTasks      int64      `json:"totalTasks"`      // 总任务数
	CompletedTasks  int64      `json:"completedTasks"`  // 已完成任务数
	PendingTasks    int64      `json:"pendingTasks"`    // 待处理任务数
}

// GetSnapshot 获取统计快照（线程安全）
func (s *ComplianceStats) GetSnapshot() *ComplianceStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &ComplianceStats{
		TotalScans:      s.TotalScans,
		SuccessfulScans: s.SuccessfulScans,
		FailedScans:     s.FailedScans,
		TotalRules:      s.TotalRules,
		EnabledRules:    s.EnabledRules,
		CustomRules:     s.CustomRules,
		LastScanTime:    s.LastScanTime,
		LastScanStatus:  s.LastScanStatus,
		CurrentScore:    s.CurrentScore,
		PreviousScore:   s.PreviousScore,
		ScoreTrend:      s.ScoreTrend,
		TotalAlerts:     s.TotalAlerts,
		ActiveAlerts:    s.ActiveAlerts,
		TotalTasks:      s.TotalTasks,
		CompletedTasks:  s.CompletedTasks,
		PendingTasks:    s.PendingTasks,
	}
}

// UpdateScore 更新合规分数（线程安全）
func (s *ComplianceStats) UpdateScore(newScore float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PreviousScore = s.CurrentScore
	s.CurrentScore = newScore
	if newScore > s.PreviousScore {
		s.ScoreTrend = "up"
	} else if newScore < s.PreviousScore {
		s.ScoreTrend = "down"
	} else {
		s.ScoreTrend = "stable"
	}
}
