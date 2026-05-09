// Package permaudit 提供权限审计报告功能。
// 扫描系统中的用户权限配置，检测过度授权、孤儿权限、权限漂移等问题。
// 参考群晖 DSM 的权限管理，增加自动化审计报告能力。
package permaudit

import "time"

// UserPerm 用户权限快照
type UserPerm struct {
	UserID    string    `json:"user_id"`
	UserName  string    `json:"user_name"`
	Groups    []string  `json:"groups"`
	Shares    []string  `json:"shares"`   // 可访问的共享目录
	Services  []string  `json:"services"` // 可用服务（smb/ftp/ssh等）
	IsAdmin   bool      `json:"is_admin"`
	LastLogin time.Time `json:"last_login"`
	CreatedAt time.Time `json:"created_at"`
	// PasswordLen 密码长度（仅审计用，不传实际密码）
	PasswordLen int `json:"password_len,omitempty"`
}

// PermIssue 权限问题
type PermIssue struct {
	Type           string `json:"type"`       // over-privileged / orphan / stale / weak-password / no-2fa
	Severity       string `json:"severity"`   // low / medium / high / critical
	UserID         string `json:"user_id"`
	UserName       string `json:"user_name"`
	Resource       string `json:"resource"`
	Description    string `json:"description"`
	Recommendation string `json:"recommendation"`
}

// AuditReport 审计报告
type AuditReport struct {
	GeneratedAt     time.Time      `json:"generated_at"`
	TotalUsers      int            `json:"total_users"`
	AdminCount      int            `json:"admin_count"`
	ActiveUsers     int            `json:"active_users"`    // 30天内登录
	InactiveUsers   int            `json:"inactive_users"`
	Issues          []PermIssue    `json:"issues"`
	IssueSummary    map[string]int `json:"issue_summary"`     // 按类型统计
	SeveritySummary map[string]int `json:"severity_summary"`  // 按严重级别统计
	Score           int            `json:"score"`             // 安全评分 0-100
	Recommendations []string       `json:"recommendations"`   // 总体建议
}
