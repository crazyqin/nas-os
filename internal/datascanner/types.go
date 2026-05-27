// Package datascanner 提供隐私数据扫描功能，支持 PII 检测、风险评估与合规映射
package datascanner

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrTaskNotFound 扫描任务不存在.
	ErrTaskNotFound = errors.New("scan task not found")
	// ErrResultNotFound 扫描结果不存在.
	ErrResultNotFound = errors.New("scan result not found")
	// ErrTaskRunning 任务正在运行中，无法重复启动.
	ErrTaskRunning = errors.New("scan task is already running")
	// ErrTaskNotRunning 任务未运行，无法暂停.
	ErrTaskNotRunning = errors.New("scan task is not running")
	// ErrInvalidPath 无效的扫描路径.
	ErrInvalidPath = errors.New("invalid scan path")
	// ErrWhitelistNotFound 白名单规则不存在.
	ErrWhitelistNotFound = errors.New("whitelist rule not found")
)

// ========== 风险等级 ==========

// RiskLevel 风险等级.
type RiskLevel string

const (
	RiskHigh   RiskLevel = "high"   // 高风险：身份证号、银行卡号等
	RiskMedium RiskLevel = "medium" // 中风险：手机号、邮箱等
	RiskLow    RiskLevel = "low"    // 低风险：地址、姓名等
)

// ========== PII 数据类型 ==========

// PIIType 个人可识别信息类型.
type PIIType string

const (
	PIIIDCard          PIIType = "id_card"           // 身份证号
	PIIPhone           PIIType = "phone"             // 手机号
	PIIBankCard        PIIType = "bank_card"         // 银行卡号
	PIIEmail           PIIType = "email"             // 邮箱
	PIIAddress         PIIType = "address"           // 地址
	PIIName            PIIType = "name"              // 姓名
	PIICreditCode      PIIType = "credit_code"       // 统一社会信用代码
	PIIPassport        PIIType = "passport"          // 护照号
	PIIMilitaryID      PIIType = "military_id"       // 军官证号
	PIILicensePlate    PIIType = "license_plate"     // 车牌号
	PIICustom          PIIType = "custom"            // 自定义规则
)

// ========== 文件类型 ==========

// FileType 文件类型.
type FileType string

const (
	FileTypeText     FileType = "text"      // 纯文本
	FileTypeDocument FileType = "document"  // Office 文档
	FileTypePDF      FileType = "pdf"       // PDF
	FileTypeImage    FileType = "image"     // 图片（OCR）
)

// ========== 扫描任务状态 ==========

// TaskStatus 任务状态.
type TaskStatus string

const (
	TaskStatusPending  TaskStatus = "pending"   // 待执行
	TaskStatusRunning  TaskStatus = "running"   // 运行中
	TaskStatusPaused   TaskStatus = "paused"    // 已暂停
	TaskStatusCanceled TaskStatus = "canceled"  // 已取消
	TaskStatusDone     TaskStatus = "done"      // 已完成
	TaskStatusFailed   TaskStatus = "failed"    // 失败
)

// ========== 合规标准 ==========

// ComplianceStandard 合规标准.
type ComplianceStandard string

const (
	ComplianceGDPR       ComplianceStandard = "gdpr"        // 欧盟 GDPR
	CompliancePIPL       ComplianceStandard = "pipl"        // 个人信息保护法
	ComplianceCSL        ComplianceStandard = "csl"         // 网络安全法
)

// ========== 报告格式 ==========

// ReportFormat 报告格式.
type ReportFormat string

const (
	ReportJSON ReportFormat = "json"
	ReportCSV  ReportFormat = "csv"
	ReportPDF  ReportFormat = "pdf"
)

// ========== 核心数据结构 ==========

// ScanTask 扫描任务.
type ScanTask struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Path            string       `json:"path"`
	Recursive       bool         `json:"recursive"`
	FileTypes       []FileType   `json:"file_types"`
	PIITypes        []PIIType    `json:"pii_types"`
	Status          TaskStatus   `json:"status"`
	Progress        float64      `json:"progress"` // 0.0 ~ 1.0
	TotalFiles      int          `json:"total_files"`
	ScannedFiles    int          `json:"scanned_files"`
	FoundItems      int          `json:"found_items"`
	WhitelistID     string       `json:"whitelist_id,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
	StartedAt       *time.Time   `json:"started_at,omitempty"`
	CompletedAt     *time.Time   `json:"completed_at,omitempty"`
	Error           string       `json:"error,omitempty"`
}

// ScanResult 单条扫描结果.
type ScanResult struct {
	ID          string       `json:"id"`
	TaskID      string       `json:"task_id"`
	FilePath    string       `json:"file_path"`
	LineNumber  int          `json:"line_number"`
	ColumnStart int          `json:"column_start"`
	ColumnEnd   int          `json:"column_end"`
	PIIType     PIIType      `json:"pii_type"`
	MatchedText string       `json:"matched_text"` // 脱敏后的匹配文本
	Context     string       `json:"context"`      // 上下文片段
	RiskLevel   RiskLevel    `json:"risk_level"`
	RiskScore   float64      `json:"risk_score"`   // 0.0 ~ 100.0
	Compliance  []ComplianceStandard `json:"compliance,omitempty"`
	Suggestion  string       `json:"suggestion,omitempty"` // 脱敏建议
	CreatedAt   time.Time    `json:"created_at"`
}

// ScanReport 扫描报告.
type ScanReport struct {
	ID            string          `json:"id"`
	TaskID        string          `json:"task_id"`
	Format        ReportFormat    `json:"format"`
	Summary       ReportSummary   `json:"summary"`
	TopRiskFiles  []FileRiskStat  `json:"top_risk_files"`
	GeneratedAt   time.Time       `json:"generated_at"`
}

// ReportSummary 报告概要统计.
type ReportSummary struct {
	TotalFiles     int               `json:"total_files"`
	ScannedFiles   int               `json:"scanned_files"`
	TotalFindings  int               `json:"total_findings"`
	RiskDist       RiskDistribution  `json:"risk_distribution"`
	PIIDist        map[PIIType]int   `json:"pii_distribution"`
}

// RiskDistribution 风险等级分布.
type RiskDistribution struct {
	High   int `json:"high"`
	Medium int `json:"medium"`
	Low    int `json:"low"`
}

// FileRiskStat 文件风险统计.
type FileRiskStat struct {
	FilePath   string     `json:"file_path"`
	Findings   int        `json:"findings"`
	RiskScore  float64    `json:"risk_score"`
	RiskLevel  RiskLevel  `json:"risk_level"`
}

// DesensitizeStrategy 脱敏策略建议.
type DesensitizeStrategy struct {
	PIIType    PIIType `json:"pii_type"`
	Strategy   string  `json:"strategy"`   // 如 "掩码", "哈希", "截断", "替换"
	Example    string  `json:"example"`    // 示例：110101********1234
	Compliance []ComplianceStandard `json:"compliance"`
}

// WhitelistRule 白名单规则.
type WhitelistRule struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	ExcludeDirs  []string `json:"exclude_dirs,omitempty"`  // 排除目录
	ExcludeExts  []string `json:"exclude_exts,omitempty"`  // 排除文件扩展名
	ExcludeFiles []string `json:"exclude_files,omitempty"` // 排除特定文件路径
	MarkedFiles  []string `json:"marked_files,omitempty"`  // 已标记/审核过的文件（跳过）
	CreatedAt    time.Time `json:"created_at"`
}

// ========== 请求/响应结构 ==========

// CreateTaskRequest 创建扫描任务请求.
type CreateTaskRequest struct {
	Name        string     `json:"name" binding:"required"`
	Path        string     `json:"path" binding:"required"`
	Recursive   bool       `json:"recursive"`
	FileTypes   []FileType `json:"file_types"`
	PIITypes    []PIIType  `json:"pii_types"`
	WhitelistID string     `json:"whitelist_id,omitempty"`
}

// UpdateWhitelistRequest 更新白名单请求.
type UpdateWhitelistRequest struct {
	Name         *string  `json:"name,omitempty"`
	ExcludeDirs  []string `json:"exclude_dirs,omitempty"`
	ExcludeExts  []string `json:"exclude_exts,omitempty"`
	ExcludeFiles []string `json:"exclude_files,omitempty"`
	MarkedFiles  []string `json:"marked_files,omitempty"`
}

// CreateWhitelistRequest 创建白名单请求.
type CreateWhitelistRequest struct {
	Name         string   `json:"name" binding:"required"`
	ExcludeDirs  []string `json:"exclude_dirs,omitempty"`
	ExcludeExts  []string `json:"exclude_exts,omitempty"`
	ExcludeFiles []string `json:"exclude_files,omitempty"`
	MarkedFiles  []string `json:"marked_files,omitempty"`
}

// GenerateReportRequest 生成报告请求.
type GenerateReportRequest struct {
	TaskID string      `json:"task_id" binding:"required"`
	Format ReportFormat `json:"format" binding:"required"`
}
