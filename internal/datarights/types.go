// Package datarights 提供 GDPR 数据主体权利管理功能
// 包括访问权、删除权（被遗忘权）、可携带权、隐私影响评估和合规报告
package datarights

import (
	"time"
)

// RightType 数据权利类型
type RightType string

const (
	RightAccess      RightType = "access"      // 访问权
	RightErasure     RightType = "erasure"      // 删除权（被遗忘权）
	RightPortability RightType = "portability"  // 可携带权
)

// RequestStatus 请求状态
type RequestStatus string

const (
	StatusPending    RequestStatus = "pending"
	StatusProcessing RequestStatus = "processing"
	StatusCompleted  RequestStatus = "completed"
	StatusRejected   RequestStatus = "rejected"
)

// DataSubject 数据主体
type DataSubject struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	TenantID string `json:"tenant_id,omitempty"`
}

// DataRightRequest 数据权利请求
type DataRightRequest struct {
	ID          string        `json:"id"`
	Type        RightType     `json:"type"`
	Subject     DataSubject   `json:"subject"`
	Status      RequestStatus `json:"status"`
	Reason      string        `json:"reason,omitempty"`
	ProcessedAt *time.Time    `json:"processed_at,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// DeletionResult 删除结果
type DeletionResult struct {
	RequestID      string   `json:"request_id"`
	DeletedRecords int      `json:"deleted_records"`
	DeletedTables  []string `json:"deleted_tables"`
	FailedTables   []string `json:"failed_tables,omitempty"`
	CompletedAt    time.Time `json:"completed_at"`
}

// ExportFormat 导出格式
type ExportFormat string

const (
	FormatJSON ExportFormat = "json"
	FormatCSV  ExportFormat = "csv"
)

// ExportResult 导出结果
type ExportResult struct {
	RequestID   string       `json:"request_id"`
	Format      ExportFormat `json:"format"`
	RecordCount int          `json:"record_count"`
	FilePath    string       `json:"file_path"`
	FileSize    int64        `json:"file_size"`
	CompletedAt time.Time    `json:"completed_at"`
}

// PrivacyImpactAssessment 隐私影响评估
type PrivacyImpactAssessment struct {
	ID              string    `json:"id"`
	ProjectName     string    `json:"project_name"`
	Description     string    `json:"description"`
	DataCategories  []string  `json:"data_categories"`
	LegalBasis      string    `json:"legal_basis"`
	RiskLevel       string    `json:"risk_level"` // low, medium, high, critical
	RiskScore       int       `json:"risk_score"` // 0-100
	Mitigations     []string  `json:"mitigations"`
	Status          string    `json:"status"` // draft, approved, rejected
	AssessedBy      string    `json:"assessed_by"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ComplianceReport 合规报告
type ComplianceReport struct {
	ID                   string    `json:"id"`
	GeneratedAt          time.Time `json:"generated_at"`
	TotalRequests        int       `json:"total_requests"`
	CompletedRequests    int       `json:"completed_requests"`
	PendingRequests      int       `json:"pending_requests"`
	RejectedRequests     int       `json:"rejected_requests"`
	AverageResponseDays  float64   `json:"average_response_days"`
	AccessRequests       int       `json:"access_requests"`
	ErasureRequests      int       `json:"erasure_requests"`
	PortabilityRequests  int       `json:"portability_requests"`
	ComplianceScore      float64   `json:"compliance_score"` // 0-100
	OverdueRequests      int       `json:"overdue_requests"`
	ActivePIAs           int       `json:"active_pias"`
}

// Config 数据权利管理配置
type Config struct {
	Enabled           bool `json:"enabled"`
	MaxRequests       int  `json:"max_requests"`
	ResponseDeadlineDays int `json:"response_deadline_days"`
	AuditEnabled      bool `json:"audit_enabled"`
}
