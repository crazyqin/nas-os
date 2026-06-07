// Package smartbackupverify 提供备份智能验证功能
package smartbackupverify

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrBackupNotFound 备份不存在.
	ErrBackupNotFound = errors.New("备份不存在")
	// ErrVerifyTaskNotFound 验证任务不存在.
	ErrVerifyTaskNotFound = errors.New("验证任务不存在")
	// ErrVerifyTaskRunning 验证任务正在运行.
	ErrVerifyTaskRunning = errors.New("验证任务正在运行")
	// ErrRestoreTestFailed 恢复测试失败.
	ErrRestoreTestFailed = errors.New("恢复测试失败")
	// ErrReportNotFound 报告不存在.
	ErrReportNotFound = errors.New("报告不存在")
)

// ========== 验证状态 ==========

// VerifyStatus 验证状态.
type VerifyStatus string

const (
	// VerifyStatusPending 等待中.
	VerifyStatusPending VerifyStatus = "pending"
	// VerifyStatusRunning 验证中.
	VerifyStatusRunning VerifyStatus = "running"
	// VerifyStatusPassed 通过.
	VerifyStatusPassed VerifyStatus = "passed"
	// VerifyStatusFailed 失败.
	VerifyStatusFailed VerifyStatus = "failed"
	// VerifyStatusWarning 警告.
	VerifyStatusWarning VerifyStatus = "warning"
)

// ========== 健康度等级 ==========

// HealthLevel 健康度等级.
type HealthLevel string

const (
	// HealthLevelExcellent 优秀 (90-100).
	HealthLevelExcellent HealthLevel = "excellent"
	// HealthLevelGood 良好 (70-89).
	HealthLevelGood HealthLevel = "good"
	// HealthLevelFair 一般 (50-69).
	HealthLevelFair HealthLevel = "fair"
	// HealthLevelPoor 较差 (30-49).
	HealthLevelPoor HealthLevel = "poor"
	// HealthLevelCritical 危险 (0-29).
	HealthLevelCritical HealthLevel = "critical"
)

// ========== 告警级别 ==========

// AlertSeverity 告警级别.
type AlertSeverity string

const (
	// AlertSeverityInfo 信息.
	AlertSeverityInfo AlertSeverity = "info"
	// AlertSeverityWarning 警告.
	AlertSeverityWarning AlertSeverity = "warning"
	// AlertSeverityError 错误.
	AlertSeverityError AlertSeverity = "error"
	// AlertSeverityCritical 严重.
	AlertSeverityCritical AlertSeverity = "critical"
)

// ========== 数据结构 ==========

// BackupInfo 备份信息.
type BackupInfo struct {
	ID        string    `json:"id"`         // 备份 ID
	TaskID    string    `json:"task_id"`    // 任务 ID
	Source    string    `json:"source"`     // 来源路径
	DestPath  string    `json:"dest_path"`  // 目标路径
	Size      int64     `json:"size"`       // 大小（字节）
	Checksum  string    `json:"checksum"`   // 校验和
	CreatedAt time.Time `json:"created_at"` // 创建时间
}

// VerifyTask 验证任务.
type VerifyTask struct {
	ID          string       `json:"id"`           // 任务 ID
	BackupID    string       `json:"backup_id"`    // 关联备份 ID
	Status      VerifyStatus `json:"status"`       // 验证状态
	StartedAt   time.Time    `json:"started_at"`   // 开始时间
	CompletedAt *time.Time   `json:"completed_at"` // 完成时间
	Duration    float64      `json:"duration"`     // 耗时（秒）
	ErrorMsg    string       `json:"error_msg"`    // 错误信息
	Checks      []CheckItem  `json:"checks"`       // 检查项
}

// CheckItem 检查项.
type CheckItem struct {
	Name     string       `json:"name"`     // 检查项名称
	Status   VerifyStatus `json:"status"`   // 状态
	Detail   string       `json:"detail"`   // 详情
	Duration float64      `json:"duration"` // 耗时（秒）
}

// RestoreTestResult 恢复测试结果.
type RestoreTestResult struct {
	ID          string    `json:"id"`           // 测试 ID
	BackupID    string    `json:"backup_id"`    // 关联备份 ID
	Success     bool      `json:"success"`      // 是否成功
	RestorePath string    `json:"restore_path"` // 恢复路径
	FileCount   int       `json:"file_count"`   // 恢复文件数
	TotalSize   int64     `json:"total_size"`   // 恢复大小
	Duration    float64   `json:"duration"`     // 耗时（秒）
	ErrorMsg    string    `json:"error_msg"`    // 错误信息
	TestedAt    time.Time `json:"tested_at"`    // 测试时间
}

// HealthScore 健康度评分.
type HealthScore struct {
	BackupID  string      `json:"backup_id"`  // 备份 ID
	Score     int         `json:"score"`      // 评分 (0-100)
	Level     HealthLevel `json:"level"`      // 健康度等级
	Factors   []Factor    `json:"factors"`    // 评分因素
	UpdatedAt time.Time   `json:"updated_at"` // 更新时间
}

// Factor 评分因素.
type Factor struct {
	Name   string `json:"name"`   // 因素名称
	Score  int    `json:"score"`  // 因素得分
	Weight int    `json:"weight"` // 权重
	Detail string `json:"detail"` // 详情
}

// VerifyReport 验证报告.
type VerifyReport struct {
	ID          string             `json:"id"`           // 报告 ID
	BackupID    string             `json:"backup_id"`    // 备份 ID
	GeneratedAt time.Time          `json:"generated_at"` // 生成时间
	Summary     ReportSummary      `json:"summary"`      // 摘要
	VerifyTask  *VerifyTask        `json:"verify_task"`  // 验证任务
	RestoreTest *RestoreTestResult `json:"restore_test"` // 恢复测试
	HealthScore *HealthScore       `json:"health_score"` // 健康度评分
	Alerts      []Alert            `json:"alerts"`       // 告警列表
}

// ReportSummary 报告摘要.
type ReportSummary struct {
	Status      VerifyStatus `json:"status"`       // 总体状态
	Message     string       `json:"message"`      // 摘要信息
	TotalChecks int          `json:"total_checks"` // 总检查数
	Passed      int          `json:"passed"`       // 通过数
	Failed      int          `json:"failed"`       // 失败数
	Warnings    int          `json:"warnings"`     // 警告数
}

// Alert 告警.
type Alert struct {
	ID        string        `json:"id"`         // 告警 ID
	BackupID  string        `json:"backup_id"`  // 备份 ID
	Severity  AlertSeverity `json:"severity"`   // 级别
	Title     string        `json:"title"`      // 标题
	Message   string        `json:"message"`    // 消息
	CreatedAt time.Time     `json:"created_at"` // 创建时间
	Resolved  bool          `json:"resolved"`   // 是否已解决
}

// ========== 请求/响应 ==========

// VerifyRequest 验证请求.
type VerifyRequest struct {
	BackupID       string `json:"backup_id" binding:"required"` // 备份 ID
	RunRestoreTest bool   `json:"run_restore_test"`             // 是否运行恢复测试
}

// BackupRegisterRequest 注册备份请求.
type BackupRegisterRequest struct {
	TaskID   string `json:"task_id" binding:"required"`   // 任务 ID
	Source   string `json:"source" binding:"required"`    // 来源路径
	DestPath string `json:"dest_path" binding:"required"` // 目标路径
	Size     int64  `json:"size"`                         // 大小
	Checksum string `json:"checksum"`                     // 校验和
}

// AlertResolveRequest 解决告警请求.
type AlertResolveRequest struct {
	AlertID string `json:"alert_id" binding:"required"` // 告警 ID
}

// VerifyStats 验证统计.
type VerifyStats struct {
	TotalBackups    int     `json:"total_backups"`    // 总备份数
	VerifiedBackups int     `json:"verified_backups"` // 已验证数
	FailedBackups   int     `json:"failed_backups"`   // 失败数
	AvgHealthScore  float64 `json:"avg_health_score"` // 平均健康度
	ActiveAlerts    int     `json:"active_alerts"`    // 活跃告警数
}
