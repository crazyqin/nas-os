package appfeedback

import (
	"math"
	"time"
)

// Rating 表示评分（1-5星，支持半星）
type Rating float32

const (
	MinRating Rating = 0.5
	MaxRating Rating = 5.0
)

// Validate 验证评分是否有效
func (r Rating) Validate() bool {
	if r < MinRating || r > MaxRating {
		return false
	}
	// 检查是否为 0.5 的倍数
	v := float64(r)
	remainder := math.Mod(v, 0.5)
	return math.Abs(remainder) < 1e-9 || math.Abs(remainder-0.5) < 1e-9
}

// FeedbackCategory 反馈分类
type FeedbackCategory string

const (
	CategoryBugReport      FeedbackCategory = "bug_report"
	CategoryFeatureRequest FeedbackCategory = "feature_request"
	CategoryUsageIssue     FeedbackCategory = "usage_issue"
	CategoryPraise         FeedbackCategory = "praise"
)

// ReportReason 举报原因
type ReportReason string

const (
	ReportSpam          ReportReason = "spam"
	ReportInappropriate ReportReason = "inappropriate"
	ReportOffensive     ReportReason = "offensive"
	ReportOther         ReportReason = "other"
)

// SortOrder 评论排序方式
type SortOrder string

const (
	SortNewest      SortOrder = "newest"
	SortHighest     SortOrder = "highest"
	SortLowest      SortOrder = "lowest"
	SortMostHelpful SortOrder = "most_helpful"
)

// Feedback 反馈/评论
type Feedback struct {
	ID          string           `json:"id"`
	AppID       string           `json:"app_id"`
	Version     string           `json:"version"`
	UserID      string           `json:"user_id"`
	Rating      Rating           `json:"rating"`
	Title       string           `json:"title,omitempty"`
	Content     string           `json:"content"`
	Category    FeedbackCategory `json:"category"`
	Helpful     int              `json:"helpful"`
	NotHelpful  int              `json:"not_helpful"`
	ReplyCount  int              `json:"reply_count"`
	ReportCount int              `json:"report_count"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// FeedbackReply 评论回复
type FeedbackReply struct {
	ID         string    `json:"id"`
	FeedbackID string    `json:"feedback_id"`
	UserID     string    `json:"user_id"`
	IsDev      bool      `json:"is_dev"` // 是否为开发者回复
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

// Vote 投票记录
type Vote struct {
	ID         string    `json:"id"`
	FeedbackID string    `json:"feedback_id"`
	UserID     string    `json:"user_id"`
	IsHelpful  bool      `json:"is_helpful"` // true=有用, false=无用
	CreatedAt  time.Time `json:"created_at"`
}

// Report 举报记录
type Report struct {
	ID         string       `json:"id"`
	FeedbackID string       `json:"feedback_id"`
	UserID     string       `json:"user_id"`
	Reason     ReportReason `json:"reason"`
	Details    string       `json:"details,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
}

// RatingStats 评分统计
type RatingStats struct {
	AppID        string       `json:"app_id"`
	TotalCount   int          `json:"total_count"`
	Average      float64      `json:"average"`
	Distribution map[int]int  `json:"distribution"`    // 星级分布，key为1-5
	Trend        []TrendPoint `json:"trend,omitempty"` // 趋势数据
}

// TrendPoint 趋势数据点
type TrendPoint struct {
	Date    string  `json:"date"` // YYYY-MM-DD
	Average float64 `json:"average"`
	Count   int     `json:"count"`
}

// CommentSummary 评论摘要
type CommentSummary struct {
	AppID       string    `json:"app_id"`
	TotalCount  int       `json:"total_count"`
	Keywords    []string  `json:"keywords"`
	Positive    []string  `json:"positive"`
	Negative    []string  `json:"negative"`
	GeneratedAt time.Time `json:"generated_at"`
}

// CreateFeedbackRequest 创建反馈请求
type CreateFeedbackRequest struct {
	AppID    string           `json:"app_id" binding:"required"`
	Version  string           `json:"version" binding:"required"`
	Rating   Rating           `json:"rating" binding:"required"`
	Title    string           `json:"title,omitempty"`
	Content  string           `json:"content" binding:"required"`
	Category FeedbackCategory `json:"category" binding:"required"`
}

// CreateReplyRequest 创建回复请求
type CreateReplyRequest struct {
	Content string `json:"content" binding:"required"`
	IsDev   bool   `json:"is_dev"`
}

// CreateVoteRequest 创建投票请求
type CreateVoteRequest struct {
	IsHelpful bool `json:"is_helpful"`
}

// CreateReportRequest 创建举报请求
type CreateReportRequest struct {
	Reason  ReportReason `json:"reason" binding:"required"`
	Details string       `json:"details,omitempty"`
}

// ListFeedbackParams 列表查询参数
type ListFeedbackParams struct {
	AppID    string
	Version  string
	Category FeedbackCategory
	Sort     SortOrder
	Page     int
	PageSize int
}

// PaginatedFeedbacks 分页反馈列表
type PaginatedFeedbacks struct {
	Feedbacks []Feedback `json:"feedbacks"`
	Total     int        `json:"total"`
	Page      int        `json:"page"`
	PageSize  int        `json:"page_size"`
	HasMore   bool       `json:"has_more"`
}
