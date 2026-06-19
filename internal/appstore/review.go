// Package appstore 应用评分和评论系统
// 支持用户对应用进行评分、评论、回复，以及评分统计
package appstore

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// ========== 评分和评论系统 ==========

// ReviewStore 评分评论存储
type ReviewStore struct {
	mu       sync.RWMutex
	reviews  map[string][]*Review // appID -> reviews
	config   *ReviewConfig
}

// ReviewConfig 评论系统配置
type ReviewConfig struct {
	MaxReviewLength  int  `json:"maxReviewLength"`  // 评论最大长度
	AllowAnonymous   bool `json:"allowAnonymous"`   // 允许匿名评论
	RequireInstall   bool `json:"requireInstall"`   // 必须安装后才能评论
	MinRating        int  `json:"minRating"`         // 最低评分
	MaxRating        int  `json:"maxRating"`         // 最高评分
	EditWindowHours  int  `json:"editWindowHours"`  // 编辑时间窗口（小时）
}

// DefaultReviewConfig 默认评论配置
func DefaultReviewConfig() *ReviewConfig {
	return &ReviewConfig{
		MaxReviewLength:  2000,
		AllowAnonymous:   false,
		RequireInstall:   true,
		MinRating:        1,
		MaxRating:        5,
		EditWindowHours:  24,
	}
}

// Review 评论
type Review struct {
	ID         string    `json:"id"`
	AppID      string    `json:"appId"`
	UserID     string    `json:"userId"`
	UserName   string    `json:"userName"`
	Rating     int       `json:"rating"`      // 1-5
	Title      string    `json:"title"`       // 评论标题
	Content    string    `json:"content"`     // 评论内容
	Version    string    `json:"version"`     // 评论时的应用版本
	Helpful    int       `json:"helpful"`     // 有用票数
	Unhelpful  int       `json:"unhelpful"`   // 无用票数
	Reply      *Reply    `json:"reply,omitempty"` // 开发者回复
	Verified   bool      `json:"verified"`    // 已验证安装
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Reply 开发者回复
type Reply struct {
	Content   string    `json:"content"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
}

// ReviewStats 评分统计
type ReviewStats struct {
	AppID       string         `json:"appId"`
	TotalCount  int            `json:"totalCount"`  // 总评论数
	AvgRating   float64        `json:"avgRating"`   // 平均评分
	RatingDist  map[int]int    `json:"ratingDist"`  // 评分分布 {1: count, 2: count, ...}
	RecentCount int            `json:"recentCount"` // 最近30天评论数
	Trend       string         `json:"trend"`       // 趋势: "up", "down", "stable"
}

// ReviewListOptions 评论列表选项
type ReviewListOptions struct {
	SortBy    string `json:"sortBy"`    // "newest", "highest", "lowest", "helpful"
	MinRating int    `json:"minRating"` // 最低评分过滤
	Verified  *bool  `json:"verified"`  // 仅已验证
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
}

// ReviewVoteResult 投票结果
type ReviewVoteResult struct {
	Helpful   int  `json:"helpful"`
	Unhelpful int  `json:"unhelpful"`
	Success   bool `json:"success"`
}

// NewReviewStore 创建评论存储
func NewReviewStore(config *ReviewConfig) *ReviewStore {
	if config == nil {
		config = DefaultReviewConfig()
	}
	return &ReviewStore{
		reviews: make(map[string][]*Review),
		config:  config,
	}
}

// SubmitReview 提交评论
func (rs *ReviewStore) SubmitReview(review *Review) error {
	if review.AppID == "" {
		return fmt.Errorf("应用ID不能为空")
	}
	if review.UserID == "" {
		return fmt.Errorf("用户ID不能为空")
	}
	if review.Rating < rs.config.MinRating || review.Rating > rs.config.MaxRating {
		return fmt.Errorf("评分必须在 %d-%d 之间", rs.config.MinRating, rs.config.MaxRating)
	}
	if len(review.Content) > rs.config.MaxReviewLength {
		return fmt.Errorf("评论内容不能超过 %d 字符", rs.config.MaxReviewLength)
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()

	// 检查是否已有评论（每用户每应用一条）
	existing := rs.reviews[review.AppID]
	for _, r := range existing {
		if r.UserID == review.UserID {
			return fmt.Errorf("您已对该应用提交过评论，请使用更新功能")
		}
	}

	review.ID = fmt.Sprintf("review_%s_%s_%d", review.AppID, review.UserID, time.Now().UnixMilli())
	review.CreatedAt = time.Now()
	review.UpdatedAt = time.Now()
	review.Helpful = 0
	review.Unhelpful = 0

	rs.reviews[review.AppID] = append(rs.reviews[review.AppID], review)
	return nil
}

// UpdateReview 更新评论
func (rs *ReviewStore) UpdateReview(reviewID, userID string, rating int, title, content string) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	for _, reviews := range rs.reviews {
		for _, r := range reviews {
			if r.ID == reviewID {
				if r.UserID != userID {
					return fmt.Errorf("只能编辑自己的评论")
				}
				// 检查编辑时间窗口
				editDeadline := r.CreatedAt.Add(time.Duration(rs.config.EditWindowHours) * time.Hour)
				if time.Now().After(editDeadline) {
					return fmt.Errorf("评论已超过 %d 小时编辑窗口", rs.config.EditWindowHours)
				}
				if rating >= rs.config.MinRating && rating <= rs.config.MaxRating {
					r.Rating = rating
				}
				if title != "" {
					r.Title = title
				}
				if content != "" {
					if len(content) > rs.config.MaxReviewLength {
						return fmt.Errorf("评论内容不能超过 %d 字符", rs.config.MaxReviewLength)
					}
					r.Content = content
				}
				r.UpdatedAt = time.Now()
				return nil
			}
		}
	}
	return fmt.Errorf("评论不存在")
}

// DeleteReview 删除评论
func (rs *ReviewStore) DeleteReview(reviewID, userID string) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	for appID, reviews := range rs.reviews {
		for i, r := range reviews {
			if r.ID == reviewID {
				if r.UserID != userID {
					return fmt.Errorf("只能删除自己的评论")
				}
				rs.reviews[appID] = append(reviews[:i], reviews[i+1:]...)
				return nil
			}
		}
	}
	return fmt.Errorf("评论不存在")
}

// GetReviews 获取应用评论列表
func (rs *ReviewStore) GetReviews(appID string, opts *ReviewListOptions) ([]*Review, int) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	reviews, exists := rs.reviews[appID]
	if !exists {
		return nil, 0
	}

	// 过滤
	var filtered []*Review
	for _, r := range reviews {
		if opts != nil {
			if opts.MinRating > 0 && r.Rating < opts.MinRating {
				continue
			}
			if opts.Verified != nil && *opts.Verified != r.Verified {
				continue
			}
		}
		filtered = append(filtered, r)
	}

	// 排序
	if opts != nil {
		switch opts.SortBy {
		case "highest":
			sort.Slice(filtered, func(i, j int) bool { return filtered[i].Rating > filtered[j].Rating })
		case "lowest":
			sort.Slice(filtered, func(i, j int) bool { return filtered[i].Rating < filtered[j].Rating })
		case "helpful":
			sort.Slice(filtered, func(i, j int) bool { return filtered[i].Helpful > filtered[j].Helpful })
		default: // newest
			sort.Slice(filtered, func(i, j int) bool { return filtered[i].CreatedAt.After(filtered[j].CreatedAt) })
		}
	}

	total := len(filtered)

	// 分页
	if opts != nil && opts.PageSize > 0 {
		page := opts.Page
		if page < 1 {
			page = 1
		}
		start := (page - 1) * opts.PageSize
		if start >= len(filtered) {
			return nil, total
		}
		end := start + opts.PageSize
		if end > len(filtered) {
			end = len(filtered)
		}
		filtered = filtered[start:end]
	}

	return filtered, total
}

// GetReviewStats 获取应用评分统计
func (rs *ReviewStore) GetReviewStats(appID string) *ReviewStats {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	reviews, exists := rs.reviews[appID]
	if !exists {
		return &ReviewStats{
			AppID:      appID,
			RatingDist: make(map[int]int),
		}
	}

	stats := &ReviewStats{
		AppID:      appID,
		TotalCount: len(reviews),
		RatingDist: make(map[int]int),
	}

	var totalRating float64
	thirtyDaysAgo := time.Now().Add(-30 * 24 * time.Hour)
	sixtyDaysAgo := time.Now().Add(-60 * 24 * time.Hour)

	var recentCount, olderCount int
	for _, r := range reviews {
		totalRating += float64(r.Rating)
		stats.RatingDist[r.Rating]++
		if r.CreatedAt.After(thirtyDaysAgo) {
			recentCount++
		} else if r.CreatedAt.After(sixtyDaysAgo) {
			olderCount++
		}
	}

	if stats.TotalCount > 0 {
		stats.AvgRating = totalRating / float64(stats.TotalCount)
	}

	stats.RecentCount = recentCount

	// 趋势判断
	if recentCount > olderCount*2 {
		stats.Trend = "up"
	} else if recentCount < olderCount/2 {
		stats.Trend = "down"
	} else {
		stats.Trend = "stable"
	}

	return stats
}

// VoteReview 对评论投票（有用/无用）
func (rs *ReviewStore) VoteReview(reviewID, userID string, helpful bool) (*ReviewVoteResult, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	for _, reviews := range rs.reviews {
		for _, r := range reviews {
			if r.ID == reviewID {
				if r.UserID == userID {
					return nil, fmt.Errorf("不能对自己的评论投票")
				}
				if helpful {
					r.Helpful++
				} else {
					r.Unhelpful++
				}
				return &ReviewVoteResult{
					Helpful:   r.Helpful,
					Unhelpful: r.Unhelpful,
					Success:   true,
				}, nil
			}
		}
	}
	return nil, fmt.Errorf("评论不存在")
}

// ReplyToReview 开发者回复评论
func (rs *ReviewStore) ReplyToReview(reviewID, author, content string) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	for _, reviews := range rs.reviews {
		for _, r := range reviews {
			if r.ID == reviewID {
				r.Reply = &Reply{
					Content:   content,
					Author:    author,
					CreatedAt: time.Now(),
				}
				return nil
			}
		}
	}
	return fmt.Errorf("评论不存在")
}

// GetUserReview 获取用户对某应用的评论
func (rs *ReviewStore) GetUserReview(appID, userID string) *Review {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	reviews, exists := rs.reviews[appID]
	if !exists {
		return nil
	}

	for _, r := range reviews {
		if r.UserID == userID {
			return r
		}
	}
	return nil
}

// GetTopReviews 获取热门评论（按有用票数排序）
func (rs *ReviewStore) GetTopReviews(appID string, limit int) []*Review {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	reviews, exists := rs.reviews[appID]
	if !exists {
		return nil
	}

	sorted := make([]*Review, len(reviews))
	copy(sorted, reviews)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Helpful > sorted[j].Helpful
	})

	if limit > 0 && len(sorted) > limit {
		sorted = sorted[:limit]
	}

	return sorted
}
