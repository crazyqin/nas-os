// Package emailmod 提供邮件审核功能（Email Moderation）
// 对标群晖 DSM 7.3 Email Moderation：策略管理、审核队列、多级审核、审计记录
package emailmod

import "time"

// ========== 审核状态 ==========

// ReviewStatus 审核状态.
type ReviewStatus string

// 审核状态常量.
const (
	StatusPending  ReviewStatus = "pending"  // 待审核
	StatusApproved ReviewStatus = "approved" // 已批准
	StatusRejected ReviewStatus = "rejected" // 已拒绝
)

// ========== 策略匹配类型 ==========

// MatchType 匹配类型.
type MatchType string

// 匹配类型常量.
const (
	MatchExact  MatchType = "exact"  // 精确匹配
	MatchRegex  MatchType = "regex"  // 正则匹配
	MatchGlob   MatchType = "glob"   // 通配符匹配
	MatchDomain MatchType = "domain" // 域名匹配
)

// ========== 审核策略 ==========

// Policy 审核策略.
type Policy struct {
	ID                string       `json:"id"`                            // 策略ID
	Name              string       `json:"name"`                          // 策略名称
	Description       string       `json:"description,omitempty"`         // 描述
	Enabled           bool         `json:"enabled"`                       // 是否启用
	Priority          int          `json:"priority"`                      // 优先级（数值越小越优先）
	SenderPatterns    []string     `json:"sender_patterns,omitempty"`     // 发件人匹配模式
	RecipientPatterns []string     `json:"recipient_patterns,omitempty"`  // 收件人匹配模式
	Keywords          []string     `json:"keywords,omitempty"`            // 主题/正文关键词
	AttachmentTypes   []string     `json:"attachment_types,omitempty"`    // 附件类型（如 .exe, .zip）
	MaxSizeMB         int          `json:"max_size_mb,omitempty"`         // 附件大小阈值（MB）
	Reviewers         []Reviewer   `json:"reviewers"`                     // 审核人列表
	MatchType         MatchType    `json:"match_type"`                    // 匹配类型
	CreatedAt         time.Time    `json:"created_at"`                    // 创建时间
	UpdatedAt         time.Time    `json:"updated_at"`                    // 更新时间
}

// PolicyInput 创建/更新策略输入.
type PolicyInput struct {
	Name              string     `json:"name" binding:"required"`         // 策略名称
	Description       string     `json:"description"`                     // 描述
	Enabled           *bool      `json:"enabled"`                         // 是否启用
	Priority          int        `json:"priority"`                        // 优先级
	SenderPatterns    []string   `json:"sender_patterns"`                 // 发件人匹配模式
	RecipientPatterns []string   `json:"recipient_patterns"`              // 收件人匹配模式
	Keywords          []string   `json:"keywords"`                        // 关键词
	AttachmentTypes   []string   `json:"attachment_types"`                // 附件类型
	MaxSizeMB         int        `json:"max_size_mb"`                     // 大小阈值
	Reviewers         []Reviewer `json:"reviewers" binding:"required,min=1"` // 审核人
	MatchType         MatchType  `json:"match_type"`                      // 匹配类型
}

// Reviewer 审核人.
type Reviewer struct {
	UserID   string `json:"user_id"`             // 用户ID
	Username string `json:"username"`             // 用户名
	Level    int    `json:"level"`                // 审核级别（1=一级审核，2=二级审核...）
	Email    string `json:"email,omitempty"`      // 通知邮箱
}

// ========== 审核队列 ==========

// QueueItem 审核队列条目.
type QueueItem struct {
	ID            string       `json:"id"`                       // 队列ID
	PolicyID      string       `json:"policy_id"`                // 匹配的策略ID
	PolicyName    string       `json:"policy_name"`              // 策略名称（冗余，方便显示）
	MessageID     string       `json:"message_id"`               // 邮件消息ID
	From          string       `json:"from"`                     // 发件人
	To            []string     `json:"to"`                       // 收件人列表
	CC            []string     `json:"cc,omitempty"`             // 抄送列表
	Subject       string       `json:"subject"`                  // 邮件主题
	BodyPreview   string       `json:"body_preview,omitempty"`   // 正文预览
	Attachments   []Attachment `json:"attachments,omitempty"`    // 附件列表
	SizeMB        float64      `json:"size_mb,omitempty"`        // 邮件大小(MB)
	Status        ReviewStatus `json:"status"`                   // 审核状态
	CurrentLevel  int          `json:"current_level"`            // 当前审核级别
	MaxLevel      int          `json:"max_level"`                // 最大审核级别
	Reviews       []ReviewLog  `json:"reviews"`                  // 审核记录
	CreatedAt     time.Time    `json:"created_at"`               // 创建时间
	ReviewedAt    *time.Time   `json:"reviewed_at,omitempty"`    // 最后审核时间
	ExpiresAt     *time.Time   `json:"expires_at,omitempty"`     // 过期时间
}

// Attachment 附件信息.
type Attachment struct {
	Name     string  `json:"name"`               // 文件名
	SizeMB   float64 `json:"size_mb"`            // 大小(MB)
	MimeType string  `json:"mime_type,omitempty"` // MIME类型
}

// ReviewInput 审核操作输入.
type ReviewInput struct {
	Comment string `json:"comment"` // 审核意见
}

// ReviewLog 审核记录.
type ReviewLog struct {
	Level     int        `json:"level"`               // 审核级别
	UserID    string     `json:"user_id"`             // 审核人ID
	Username  string     `json:"username"`             // 审核人名称
	Status    ReviewStatus `json:"status"`             // 审核结果
	Comment   string     `json:"comment,omitempty"`   // 审核意见
	ReviewedAt time.Time `json:"reviewed_at"`         // 审核时间
}

// ========== 审计记录 ==========

// AuditEntry 审计记录条目.
type AuditEntry struct {
	ID          string       `json:"id"`                     // 审计ID
	QueueID     string       `json:"queue_id"`               // 队列ID
	MessageID   string       `json:"message_id"`             // 邮件消息ID
	PolicyID    string       `json:"policy_id"`              // 策略ID
	PolicyName  string       `json:"policy_name"`            // 策略名称
	From        string       `json:"from"`                   // 发件人
	To          string       `json:"to"`                     // 收件人（逗号分隔）
	Subject     string       `json:"subject"`                // 邮件主题
	Status      ReviewStatus `json:"status"`                 // 最终状态
	ReviewerID  string       `json:"reviewer_id,omitempty"`  // 审核人ID
	ReviewerName string     `json:"reviewer_name,omitempty"` // 审核人名称
	Comment     string       `json:"comment,omitempty"`      // 审核意见
	Action      string       `json:"action"`                 // 操作类型（approve/reject/auto_approve/auto_reject）
	CreatedAt   time.Time    `json:"created_at"`             // 记录时间
}

// ========== 查询选项 ==========

// QueueQueryOptions 审核队列查询选项.
type QueueQueryOptions struct {
	Status   ReviewStatus `json:"status,omitempty"`   // 状态筛选
	PolicyID string       `json:"policy_id,omitempty"` // 策略ID筛选
	From     string       `json:"from,omitempty"`       // 发件人筛选
	Keyword  string       `json:"keyword,omitempty"`    // 关键词搜索
	Limit    int          `json:"limit"`                // 分页限制
	Offset   int          `json:"offset"`               // 分页偏移
}

// AuditQueryOptions 审计查询选项.
type AuditQueryOptions struct {
	Status      ReviewStatus `json:"status,omitempty"`        // 状态筛选
	PolicyID    string       `json:"policy_id,omitempty"`     // 策略ID筛选
	ReviewerID  string       `json:"reviewer_id,omitempty"`   // 审核人筛选
	Keyword     string       `json:"keyword,omitempty"`       // 关键词搜索
	StartTime   *time.Time   `json:"start_time,omitempty"`    // 开始时间
	EndTime     *time.Time   `json:"end_time,omitempty"`      // 结束时间
	Limit       int          `json:"limit"`                   // 分页限制
	Offset      int          `json:"offset"`                  // 分页偏移
}

// ========== 统计 ==========

// Stats 审核统计.
type Stats struct {
	TotalPending    int            `json:"total_pending"`     // 待审核数
	TotalApproved   int            `json:"total_approved"`    // 已批准数
	TotalRejected   int            `json:"total_rejected"`    // 已拒绝数
	TodayProcessed  int            `json:"today_processed"`   // 今日处理数
	ByPolicy        map[string]int `json:"by_policy"`         // 按策略统计
	ByReviewer      map[string]int `json:"by_reviewer"`       // 按审核人统计
}

// ========== 错误定义 ==========

// 错误码.
const (
	ErrCodeInvalidParam  = 400
	ErrCodeNotFound      = 404
	ErrCodeConflict      = 409
	ErrCodeInternalError = 500
)

// 错误消息.
var (
	ErrPolicyNotFound    = "审核策略不存在"
	ErrQueueItemNotFound = "审核队列条目不存在"
	ErrAlreadyReviewed   = "该邮件已被审核"
	ErrNotCurrentReviewer = "您不是当前级别的审核人"
	ErrInvalidPolicy     = "无效的策略参数"
	ErrEmptyReviewers    = "审核人列表不能为空"
)

// ========== API 响应 ==========

// SuccessResponse 成功响应.
func SuccessResponse(data interface{}) map[string]interface{} {
	return map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    data,
	}
}

// ErrorResponse 错误响应.
func ErrorResponse(code int, message string) map[string]interface{} {
	return map[string]interface{}{
		"code":    code,
		"message": message,
	}
}
