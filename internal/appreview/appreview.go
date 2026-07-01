// Package appreview 提供应用中心评价系统，参考群晖 DSM 应用商店评价机制。
// 支持用户对已安装应用评分、写评论、开发者回复评价、评价审核与搜索聚合。
package appreview

import (
	"crypto/rand"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
	"time"
)

// ============================================================
// 数据模型
// ============================================================

// Review 评价结构
type Review struct {
	ID             string            `json:"id"`              // 评价 ID
	AppID          string            `json:"app_id"`          // 应用 ID
	UserID         string            `json:"user_id"`         // 用户 ID
	UserName       string            `json:"user_name"`       // 用户名
	Rating         int               `json:"rating"`          // 评分 1-5 星
	Content        string            `json:"content"`         // 评论内容
	Title          string            `json:"title,omitempty"` // 评论标题
	CreatedAt      time.Time         `json:"created_at"`      // 创建时间
	UpdatedAt      time.Time         `json:"updated_at"`      // 更新时间
	Helpful        int               `json:"helpful"`         // 有用数
	NotHelpful     int               `json:"not_helpful"`      // 无用数
	Version        string            `json:"version"`        // 评价时的应用版本
	Verified       bool              `json:"verified"`        // 是否已验证安装
	DeveloperReply *DeveloperReply   `json:"developer_reply,omitempty"` // 开发者回复
	Reports        []*ReviewReport   `json:"reports,omitempty"`        // 举报记录
	Hidden         bool              `json:"hidden"`          // 是否被隐藏
	HiddenReason   string            `json:"hidden_reason,omitempty"`   // 隐藏原因
}

// DeveloperReply 开发者回复评价
type DeveloperReply struct {
	ReviewID  string    `json:"review_id"`   // 关联评价 ID
	Content   string    `json:"content"`    // 回复内容
	AuthorID  string    `json:"author_id"`  // 开发者 ID
	AuthorName string  `json:"author_name"`// 开发者名称
	CreatedAt time.Time `json:"created_at"` // 回复时间
	UpdatedAt time.Time `json:"updated_at"` // 更新时间
}

// ReviewReport 评价举报
type ReviewReport struct {
	ReviewID string    `json:"review_id"` // 评价 ID
	UserID   string    `json:"user_id"`  // 举报用户 ID
	Reason   string    `json:"reason"`    // 举报原因
	Detail   string    `json:"detail"`    // 详细说明
	At       time.Time `json:"at"`        // 举报时间
}

// ReviewStats 应用评价统计
type ReviewStats struct {
	AppID          string  `json:"app_id"`            // 应用 ID
	AverageRating  float64 `json:"average_rating"`   // 平均分
	TotalReviews   int     `json:"total_reviews"`     // 评价总数
	Distribution   map[int]int `json:"distribution"`  // 评分分布 {1: n, 2: n, ...5: n}
	VerifiedCount  int     `json:"verified_count"`    // 已验证用户评价数
	RepliedCount   int     `json:"replied_count"`     // 已有开发者回复的评价数
}

// ReviewSortMode 评价排序模式
type ReviewSortMode string

const (
	SortNewest    ReviewSortMode = "newest"     // 最新
	SortOldest    ReviewSortMode = "oldest"     // 最早
	SortHighest   ReviewSortMode = "highest"    // 评分最高
	SortLowest    ReviewSortMode = "lowest"     // 评分最低
	SortHelpful   ReviewSortMode = "helpful"    // 最有用
)

// ReviewFilter 评价筛选条件
type ReviewFilter struct {
	AppID    string         `json:"app_id,omitempty"`    // 按应用筛选
	UserID   string         `json:"user_id,omitempty"`   // 按用户筛选
	Rating   *int           `json:"rating,omitempty"`    // 按评分筛选
	MinRating int           `json:"min_rating,omitempty"`// 最低评分
	MaxRating int           `json:"max_rating,omitempty"`// 最高评分
	HasReply  *bool         `json:"has_reply,omitempty"` // 是否有开发者回复
	Verified *bool          `json:"verified,omitempty"`  // 仅已验证
	Hidden   *bool          `json:"hidden,omitempty"`    // 是否包含隐藏（管理员用）
	Sort     ReviewSortMode `json:"sort,omitempty"`      // 排序方式
	Keyword  string         `json:"keyword,omitempty"`   // 关键词搜索
}

// ============================================================
// ReviewManager 评价管理器
// ============================================================

// ReviewManager 评价管理器，管理所有评价的增删改查
type ReviewManager struct {
	mu      sync.RWMutex
	reviews map[string]*Review // reviewID -> Review
}

// NewReviewManager 创建评价管理器
func NewReviewManager() *ReviewManager {
	return &ReviewManager{
		reviews: make(map[string]*Review),
	}
}

// CreateReview 创建评价
func (rm *ReviewManager) CreateReview(appID, userID, userName, title, content, version string, rating int) (*Review, error) {
	if appID == "" {
		return nil, ErrEmptyAppID
	}
	if userID == "" {
		return nil, ErrEmptyUserID
	}
	if rating < 1 || rating > 5 {
		return nil, ErrInvalidRating
	}
	if strings.TrimSpace(content) == "" {
		return nil, ErrEmptyContent
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	// 检查用户是否已评价过该应用
	for _, r := range rm.reviews {
		if r.AppID == appID && r.UserID == userID && !r.Hidden {
			return nil, ErrDuplicateReview
		}
	}

	now := time.Now()
	review := &Review{
		ID:        generateReviewID(),
		AppID:     appID,
		UserID:    userID,
		UserName:  userName,
		Rating:    rating,
		Content:   content,
		Title:     title,
		Version:   version,
		Verified:  true, // 假设已验证安装
		CreatedAt: now,
		UpdatedAt: now,
	}

	rm.reviews[review.ID] = review
	return review, nil
}

// GetReview 获取单条评价
func (rm *ReviewManager) GetReview(reviewID string) (*Review, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	r, ok := rm.reviews[reviewID]
	if !ok {
		return nil, ErrReviewNotFound
	}
	return r, nil
}

// UpdateReview 更新评价内容
func (rm *ReviewManager) UpdateReview(reviewID, userID, content string, rating int) (*Review, error) {
	if rating < 1 || rating > 5 {
		return nil, ErrInvalidRating
	}
	if strings.TrimSpace(content) == "" {
		return nil, ErrEmptyContent
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	r, ok := rm.reviews[reviewID]
	if !ok {
		return nil, ErrReviewNotFound
	}
	if r.UserID != userID {
		return nil, ErrNotReviewOwner
	}

	r.Content = content
	r.Rating = rating
	r.UpdatedAt = time.Now()
	return r, nil
}

// DeleteReview 删除评价
func (rm *ReviewManager) DeleteReview(reviewID, userID string, isAdmin bool) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	r, ok := rm.reviews[reviewID]
	if !ok {
		return ErrReviewNotFound
	}
	if r.UserID != userID && !isAdmin {
		return ErrNotReviewOwner
	}
	delete(rm.reviews, reviewID)
	return nil
}

// GetReviewsByApp 获取应用的所有评价
func (rm *ReviewManager) GetReviewsByApp(appID string, includeHidden bool) []*Review {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var result []*Review
	for _, r := range rm.reviews {
		if r.AppID != appID {
			continue
		}
		if r.Hidden && !includeHidden {
			continue
		}
		result = append(result, r)
	}
	return result
}

// GetReviewsByUser 获取用户的所有评价
func (rm *ReviewManager) GetReviewsByUser(userID string) []*Review {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var result []*Review
	for _, r := range rm.reviews {
		if r.UserID == userID {
			result = append(result, r)
		}
	}
	return result
}

// VoteHelpful 标记评价有用/无用
func (rm *ReviewManager) VoteHelpful(reviewID string, helpful bool) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	r, ok := rm.reviews[reviewID]
	if !ok {
		return ErrReviewNotFound
	}
	if helpful {
		r.Helpful++
	} else {
		r.NotHelpful++
	}
	return nil
}

// SetDeveloperReply 设置开发者回复
func (rm *ReviewManager) SetDeveloperReply(reviewID, devID, devName, content string) error {
	if strings.TrimSpace(content) == "" {
		return ErrEmptyContent
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	r, ok := rm.reviews[reviewID]
	if !ok {
		return ErrReviewNotFound
	}

	now := time.Now()
	if r.DeveloperReply == nil {
		r.DeveloperReply = &DeveloperReply{
			ReviewID:  reviewID,
			Content:   content,
			AuthorID:  devID,
			AuthorName: devName,
			CreatedAt: now,
			UpdatedAt: now,
		}
	} else {
		r.DeveloperReply.Content = content
		r.DeveloperReply.AuthorID = devID
		r.DeveloperReply.AuthorName = devName
		r.DeveloperReply.UpdatedAt = now
	}
	return nil
}

// GetAllReviews 获取所有评价（管理员）
func (rm *ReviewManager) GetAllReviews() []*Review {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make([]*Review, 0, len(rm.reviews))
	for _, r := range rm.reviews {
		result = append(result, r)
	}
	return result
}

// Count 返回评价总数
func (rm *ReviewManager) Count() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return len(rm.reviews)
}

// ============================================================
// ReviewModerator 评价审核
// ============================================================

// ReviewModerator 评价审核器，处理举报和隐藏不当评价
type ReviewModerator struct {
	mu         sync.Mutex
	manager    *ReviewManager
	reports    map[string][]*ReviewReport // reviewID -> reports
	reportCount map[string]int             // reviewID -> report count
	threshold   int                         // 自动隐藏的举报阈值
}

// NewReviewModerator 创建评价审核器
func NewReviewModerator(manager *ReviewManager, autoHideThreshold int) *ReviewModerator {
	if autoHideThreshold <= 0 {
		autoHideThreshold = 5
	}
	return &ReviewModerator{
		manager:     manager,
		reports:     make(map[string][]*ReviewReport),
		reportCount: make(map[string]int),
		threshold:   autoHideThreshold,
	}
}

// ReportReview 举报评价
func (mod *ReviewModerator) ReportReview(reviewID, userID, reason, detail string) error {
	if reason == "" {
		return ErrEmptyReason
	}

	// 检查评价是否存在
	if _, err := mod.manager.GetReview(reviewID); err != nil {
		return err
	}

	mod.mu.Lock()
	defer mod.mu.Unlock()

	// 检查用户是否已举报过该评价
	for _, rep := range mod.reports[reviewID] {
		if rep.UserID == userID {
			return ErrDuplicateReport
		}
	}

	report := &ReviewReport{
		ReviewID: reviewID,
		UserID:   userID,
		Reason:   reason,
		Detail:   detail,
		At:       time.Now(),
	}
	mod.reports[reviewID] = append(mod.reports[reviewID], report)
	mod.reportCount[reviewID]++

	// 达到阈值自动隐藏
	if mod.reportCount[reviewID] >= mod.threshold {
		return mod.manager.HideReview(reviewID, "auto: too many reports")
	}
	return nil
}

// HideReview 隐藏评价（管理员操作）
func (rm *ReviewManager) HideReview(reviewID, reason string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	r, ok := rm.reviews[reviewID]
	if !ok {
		return ErrReviewNotFound
	}
	r.Hidden = true
	r.HiddenReason = reason
	return nil
}

// UnhideReview 取消隐藏评价（管理员操作）
func (rm *ReviewManager) UnhideReview(reviewID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	r, ok := rm.reviews[reviewID]
	if !ok {
		return ErrReviewNotFound
	}
	r.Hidden = false
	r.HiddenReason = ""
	return nil
}

// GetReports 获取评价的举报记录
func (mod *ReviewModerator) GetReports(reviewID string) []*ReviewReport {
	mod.mu.Lock()
	defer mod.mu.Unlock()
	return mod.reports[reviewID]
}

// GetReportCount 获取评价的举报次数
func (mod *ReviewModerator) GetReportCount(reviewID string) int {
	mod.mu.Lock()
	defer mod.mu.Unlock()
	return mod.reportCount[reviewID]
}

// SetThreshold 设置自动隐藏举报阈值
func (mod *ReviewModerator) SetThreshold(threshold int) {
	mod.mu.Lock()
	defer mod.mu.Unlock()
	mod.threshold = threshold
}

// ============================================================
// ReviewSearcher 评价搜索和排序
// ============================================================

// ReviewSearcher 评价搜索与排序器
type ReviewSearcher struct {
	manager *ReviewManager
}

// NewReviewSearcher 创建评价搜索器
func NewReviewSearcher(manager *ReviewManager) *ReviewSearcher {
	return &ReviewSearcher{manager: manager}
}

// Search 根据筛选条件搜索评价
func (rs *ReviewSearcher) Search(filter ReviewFilter) []*Review {
	var candidates []*Review

	// 确定候选集
	if filter.AppID != "" {
		candidates = rs.manager.GetReviewsByApp(filter.AppID, filter.Hidden != nil && *filter.Hidden)
	} else {
		all := rs.manager.GetAllReviews()
		candidates = make([]*Review, 0, len(all))
		for _, r := range all {
			if r.Hidden && (filter.Hidden == nil || !*filter.Hidden) {
				continue
			}
			candidates = append(candidates, r)
		}
	}

	// 应用筛选
	var result []*Review
	for _, r := range candidates {
		if !matchFilter(r, &filter) {
			continue
		}
		result = append(result, r)
	}

	// 排序
	sortReviews(result, filter.Sort)
	return result
}

// matchFilter 检查评价是否匹配筛选条件
func matchFilter(r *Review, f *ReviewFilter) bool {
	if f.UserID != "" && r.UserID != f.UserID {
		return false
	}
	if f.Rating != nil && r.Rating != *f.Rating {
		return false
	}
	if f.MinRating > 0 && r.Rating < f.MinRating {
		return false
	}
	if f.MaxRating > 0 && r.Rating > f.MaxRating {
		return false
	}
	if f.HasReply != nil {
		hasReply := r.DeveloperReply != nil
		if *f.HasReply != hasReply {
			return false
		}
	}
	if f.Verified != nil && *f.Verified != r.Verified {
		return false
	}
	if f.Keyword != "" {
		kw := strings.ToLower(f.Keyword)
		if !strings.Contains(strings.ToLower(r.Content), kw) &&
			!strings.Contains(strings.ToLower(r.Title), kw) {
			return false
		}
	}
	return true
}

// sortReviews 按指定方式排序评价
func sortReviews(reviews []*Review, mode ReviewSortMode) {
	switch mode {
	case SortNewest:
		sort.Slice(reviews, func(i, j int) bool {
			return reviews[i].CreatedAt.After(reviews[j].CreatedAt)
		})
	case SortOldest:
		sort.Slice(reviews, func(i, j int) bool {
			return reviews[i].CreatedAt.Before(reviews[j].CreatedAt)
		})
	case SortHighest:
		sort.Slice(reviews, func(i, j int) bool {
			return reviews[i].Rating > reviews[j].Rating
		})
	case SortLowest:
		sort.Slice(reviews, func(i, j int) bool {
			return reviews[i].Rating < reviews[j].Rating
		})
	case SortHelpful:
		sort.Slice(reviews, func(i, j int) bool {
			return reviews[i].Helpful > reviews[j].Helpful
		})
	default:
		// 默认按最新排序
		sort.Slice(reviews, func(i, j int) bool {
			return reviews[i].CreatedAt.After(reviews[j].CreatedAt)
		})
	}
}

// ============================================================
// ReviewAggregator 应用中心评价聚合
// ============================================================

// ReviewAggregator 评价聚合器，计算应用评价统计
type ReviewAggregator struct {
	manager *ReviewManager
}

// NewReviewAggregator 创建评价聚合器
func NewReviewAggregator(manager *ReviewManager) *ReviewAggregator {
	return &ReviewAggregator{manager: manager}
}

// GetStats 获取应用评价统计
func (ra *ReviewAggregator) GetStats(appID string) *ReviewStats {
	reviews := ra.manager.GetReviewsByApp(appID, false)

	stats := &ReviewStats{
		AppID:        appID,
		Distribution: map[int]int{1: 0, 2: 0, 3: 0, 4: 0, 5: 0},
	}

	totalRating := 0
	for _, r := range reviews {
		stats.TotalReviews++
		totalRating += r.Rating
		stats.Distribution[r.Rating]++
		if r.Verified {
			stats.VerifiedCount++
		}
		if r.DeveloperReply != nil {
			stats.RepliedCount++
		}
	}

	if stats.TotalReviews > 0 {
		stats.AverageRating = float64(totalRating) / float64(stats.TotalReviews)
		// 保留一位小数
		stats.AverageRating = float64(int(stats.AverageRating*10+0.5)) / 10
	}

	return stats
}

// GetTopRatedApps 获取评分最高的应用列表
func (ra *ReviewAggregator) GetTopRatedApps(appIDs []string, limit int) []*ReviewStats {
	var statsList []*ReviewStats
	for _, appID := range appIDs {
		s := ra.GetStats(appID)
		if s.TotalReviews > 0 {
			statsList = append(statsList, s)
		}
	}

	sort.Slice(statsList, func(i, j int) bool {
		if statsList[i].AverageRating != statsList[j].AverageRating {
			return statsList[i].AverageRating > statsList[j].AverageRating
		}
		return statsList[i].TotalReviews > statsList[j].TotalReviews
	})

	if limit > 0 && len(statsList) > limit {
		statsList = statsList[:limit]
	}
	return statsList
}

// ============================================================
// RESTful API 接口处理器（标准库 net/http）
// ============================================================

// APIHandler RESTful API 处理器
type APIHandler struct {
	manager   *ReviewManager
	moderator *ReviewModerator
	searcher  *ReviewSearcher
	agg       *ReviewAggregator
}

// NewAPIHandler 创建 API 处理器
func NewAPIHandler(rm *ReviewManager) *APIHandler {
	return &APIHandler{
		manager:   rm,
		moderator: NewReviewModerator(rm, 5),
		searcher:  NewReviewSearcher(rm),
		agg:       NewReviewAggregator(rm),
	}
}

// CreateReviewRequest 创建评价请求
type CreateReviewRequest struct {
	AppID    string `json:"app_id"`
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	Version  string `json:"version"`
	Rating   int    `json:"rating"`
}

// ReplyReviewRequest 开发者回复请求
type ReplyReviewRequest struct {
	ReviewID  string `json:"review_id"`
	AuthorID  string `json:"author_id"`
	AuthorName string `json:"author_name"`
	Content   string `json:"content"`
}

// ReportReviewRequest 举报评价请求
type ReportReviewRequest struct {
	ReviewID string `json:"review_id"`
	UserID   string `json:"user_id"`
	Reason   string `json:"reason"`
	Detail   string `json:"detail"`
}

// APIResponse 标准 API 响应
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// HandleCreateReview 创建评价 API
// POST /api/appreview/reviews
func (h *APIHandler) HandleCreateReview(req CreateReviewRequest) APIResponse {
	review, err := h.manager.CreateReview(
		req.AppID, req.UserID, req.UserName,
		req.Title, req.Content, req.Version, req.Rating,
	)
	if err != nil {
		return APIResponse{Success: false, Error: err.Error()}
	}
	return APIResponse{Success: true, Data: review}
}

// HandleGetReviews 获取评价列表 API
// GET /api/appreview/reviews?app_id=xxx&sort=newest
func (h *APIHandler) HandleGetReviews(filter ReviewFilter) APIResponse {
	reviews := h.searcher.Search(filter)
	return APIResponse{Success: true, Data: reviews}
}

// HandleGetReviewStats 获取应用评价统计 API
// GET /api/appreview/stats?app_id=xxx
func (h *APIHandler) HandleGetReviewStats(appID string) APIResponse {
	stats := h.agg.GetStats(appID)
	return APIResponse{Success: true, Data: stats}
}

// HandleReplyReview 开发者回复评价 API
// POST /api/appreview/reviews/reply
func (h *APIHandler) HandleReplyReview(req ReplyReviewRequest) APIResponse {
	err := h.manager.SetDeveloperReply(req.ReviewID, req.AuthorID, req.AuthorName, req.Content)
	if err != nil {
		return APIResponse{Success: false, Error: err.Error()}
	}
	review, err := h.manager.GetReview(req.ReviewID)
	if err != nil {
		return APIResponse{Success: false, Error: err.Error()}
	}
	return APIResponse{Success: true, Data: review}
}

// HandleReportReview 举报评价 API
// POST /api/appreview/reviews/report
func (h *APIHandler) HandleReportReview(req ReportReviewRequest) APIResponse {
	err := h.moderator.ReportReview(req.ReviewID, req.UserID, req.Reason, req.Detail)
	if err != nil {
		return APIResponse{Success: false, Error: err.Error()}
	}
	return APIResponse{Success: true, Data: map[string]string{"status": "reported"}}
}

// HandleDeleteReview 删除评价 API
// DELETE /api/appreview/reviews/{id}
func (h *APIHandler) HandleDeleteReview(reviewID, userID string, isAdmin bool) APIResponse {
	err := h.manager.DeleteReview(reviewID, userID, isAdmin)
	if err != nil {
		return APIResponse{Success: false, Error: err.Error()}
	}
	return APIResponse{Success: true, Data: map[string]string{"status": "deleted"}}
}

// HandleVoteHelpful 标记有用/无用 API
// POST /api/appreview/reviews/{id}/vote
func (h *APIHandler) HandleVoteHelpful(reviewID string, helpful bool) APIResponse {
	err := h.manager.VoteHelpful(reviewID, helpful)
	if err != nil {
		return APIResponse{Success: false, Error: err.Error()}
	}
	return APIResponse{Success: true, Data: map[string]bool{"helpful": helpful}}
}

// ============================================================
// 错误定义
// ============================================================

var (
	ErrEmptyAppID      = reviewError("app_id 不能为空")
	ErrEmptyUserID     = reviewError("user_id 不能为空")
	ErrInvalidRating   = reviewError("评分必须在 1-5 之间")
	ErrEmptyContent    = reviewError("评论内容不能为空")
	ErrEmptyReason     = reviewError("举报原因不能为空")
	ErrReviewNotFound  = reviewError("评价不存在")
	ErrNotReviewOwner  = reviewError("无权操作此评价")
	ErrDuplicateReview = reviewError("用户已评价过该应用")
	ErrDuplicateReport  = reviewError("用户已举报过该评价")
)

// reviewError 自定义错误类型
type reviewError string

func (e reviewError) Error() string { return string(e) }

// ============================================================
// 辅助函数
// ============================================================

// generateReviewID 生成评价 ID
func generateReviewID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "rev_" + hex.EncodeToString(b)
}