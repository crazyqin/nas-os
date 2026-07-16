package selfserviceportal

import (
	"sync"
	"time"
)

// TicketStatus 工单状态.
type TicketStatus string

const (
	TicketPending  TicketStatus = "pending"
	TicketApproved TicketStatus = "approved"
	TicketRejected TicketStatus = "rejected"
	TicketResolved TicketStatus = "resolved"
)

// TicketType 工单类型.
type TicketType string

const (
	TicketTypeQuota  TicketType = "quota"
	TicketTypePerm   TicketType = "permission"
	TicketTypeBackup TicketType = "backup"
	TicketTypeIssue  TicketType = "issue"
)

// ApprovalStatus 审批状态.
type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalRejected ApprovalStatus = "rejected"
)

// PermissionType 权限类型.
type PermissionType string

const (
	PermRead  PermissionType = "read"
	PermWrite PermissionType = "write"
	PermAdmin PermissionType = "admin"
)

// QuotaRequest 配额申请.
type QuotaRequest struct {
	ID          string       `json:"id"`
	UserID      string       `json:"user_id"`
	CurrentGB   int64        `json:"current_gb"`
	RequestedGB int64        `json:"requested_gb"`
	Reason      string       `json:"reason"`
	Status      TicketStatus `json:"status"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// PermissionRequest 权限申请.
type PermissionRequest struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	SharePath string         `json:"share_path"`
	PermType  PermissionType `json:"perm_type"`
	Temporary bool           `json:"temporary"`
	ExpiresAt *time.Time     `json:"expires_at,omitempty"`
	Reason    string         `json:"reason"`
	Status    TicketStatus   `json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// RestorePoint 恢复点.
type RestorePoint struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	FilePath  string    `json:"file_path"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// RestoreRequest 恢复请求.
type RestoreRequest struct {
	ID           string       `json:"id"`
	UserID       string       `json:"user_id"`
	RestorePoint string       `json:"restore_point_id"`
	TargetPath   string       `json:"target_path"`
	Status       TicketStatus `json:"status"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// IssueTicket 问题工单.
type IssueTicket struct {
	ID          string       `json:"id"`
	UserID      string       `json:"user_id"`
	Subject     string       `json:"subject"`
	Description string       `json:"description"`
	Category    string       `json:"category"`
	Priority    string       `json:"priority"`
	Status      TicketStatus `json:"status"`
	AssignedTo  string       `json:"assigned_to,omitempty"`
	Resolution  string       `json:"resolution,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// Approval 审批记录.
type Approval struct {
	ID         string         `json:"id"`
	TicketID   string         `json:"ticket_id"`
	TicketType TicketType     `json:"ticket_type"`
	ApproverID string         `json:"approver_id"`
	Status     ApprovalStatus `json:"status"`
	Comment    string         `json:"comment,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

// UserStats 用户统计.
type UserStats struct {
	UserID          string  `json:"user_id"`
	TotalQuotaGB    int64   `json:"total_quota_gb"`
	UsedQuotaGB     int64   `json:"used_quota_gb"`
	UsagePercent    float64 `json:"usage_percent"`
	ActiveTickets   int     `json:"active_tickets"`
	ResolvedTickets int     `json:"resolved_tickets"`
	RestorePoints   int     `json:"restore_points"`
	SharedFolders   int     `json:"shared_folders"`
}

// Notification 通知.
type Notification struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

// AutoApprovalRule 自动审批规则.
type AutoApprovalRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// MaxAutoApproveGB 自动审批的最大申请量(GB)
	MaxAutoApproveGB int64 `json:"max_auto_approve_gb"`
	// MaxPercentOfCurrent 当前配额的最大百分比(申请量 < 当前配额 * Percent / 100 自动通过)
	MaxPercentOfCurrent float64 `json:"max_percent_of_current"`
	Enabled             bool    `json:"enabled"`
}

// Portal 自助门户.
type Portal struct {
	mu                sync.RWMutex
	quotaRequests     map[string]*QuotaRequest
	permRequests      map[string]*PermissionRequest
	restoreRequests   map[string]*RestoreRequest
	issueTickets      map[string]*IssueTicket
	approvals         map[string][]*Approval
	notifications     map[string][]*Notification
	restorePoints     map[string][]*RestorePoint
	userStats         map[string]*UserStats
	autoApprovalRules []*AutoApprovalRule
	nextID            int64
}
