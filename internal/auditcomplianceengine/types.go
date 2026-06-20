package auditcomplianceengine

import "errors"

// 框架相关错误
var (
	ErrNilFramework         = errors.New("framework cannot be nil")
	ErrInvalidFrameworkID   = errors.New("invalid framework ID")
	ErrFrameworkNotFound    = errors.New("framework not found")
	ErrFrameworkAlreadyExists = errors.New("framework already exists")
)

// 控制项相关错误
var (
	ErrNilControl         = errors.New("control cannot be nil")
	ErrInvalidControlID   = errors.New("invalid control ID")
	ErrControlNotFound    = errors.New("control not found")
	ErrControlAlreadyExists = errors.New("control already exists")
)

// 发现相关错误
var (
	ErrNilFinding          = errors.New("finding cannot be nil")
	ErrInvalidFindingID    = errors.New("invalid finding ID")
	ErrFindingNotFound     = errors.New("finding not found")
	ErrFindingAlreadyResolved = errors.New("finding already resolved")
)

// 审计日志相关错误
var (
	ErrNilAuditEntry = errors.New("audit entry cannot be nil")
	ErrNilFilter     = errors.New("filter cannot be nil")
)

// 评估相关错误
var (
	ErrAssessmentFailed    = errors.New("assessment failed")
	ErrInvalidThreshold    = errors.New("invalid threshold")
	ErrRuleNotFound        = errors.New("assessment rule not found")
)

// 报告相关错误
var (
	ErrReportNotFound     = errors.New("report not found")
	ErrReportGeneration   = errors.New("report generation failed")
	ErrInvalidReportType  = errors.New("invalid report type")
)

// 证据相关错误
var (
	ErrNilEvidence       = errors.New("evidence cannot be nil")
	ErrInvalidEvidenceID = errors.New("invalid evidence ID")
	ErrEvidenceNotFound  = errors.New("evidence not found")
	ErrEvidenceExpired   = errors.New("evidence has expired")
)

// 配置相关错误
var (
	ErrInvalidConfig     = errors.New("invalid configuration")
	ErrEncryptionRequired = errors.New("encryption is required")
	ErrInvalidDataRegion = errors.New("invalid data region")
)
