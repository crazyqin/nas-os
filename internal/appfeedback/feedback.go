package appfeedback

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// 常见错误
var (
	ErrFeedbackNotFound = errors.New("feedback not found")
	ErrInvalidRating    = errors.New("invalid rating: must be between 0.5 and 5.0 in 0.5 increments")
	ErrAlreadyVoted     = errors.New("user already voted on this feedback")
	ErrAlreadyReported  = errors.New("user already reported this feedback")
	ErrReplyNotFound    = errors.New("reply not found")
)

// Store 反馈存储接口
type Store interface {
	// 反馈操作
	CreateFeedback(feedback *Feedback) error
	GetFeedback(id string) (*Feedback, error)
	UpdateFeedback(feedback *Feedback) error
	DeleteFeedback(id string) error
	ListFeedbacks(params ListFeedbackParams) (*PaginatedFeedbacks, error)

	// 回复操作
	CreateReply(reply *FeedbackReply) error
	GetReply(id string) (*FeedbackReply, error)
	ListReplies(feedbackID string) ([]*FeedbackReply, error)
	DeleteReply(id string) error

	// 投票操作
	CreateVote(vote *Vote) error
	GetVote(feedbackID, userID string) (*Vote, error)
	DeleteVote(feedbackID, userID string) error

	// 举报操作
	CreateReport(report *Report) error
	GetReport(feedbackID, userID string) (*Report, error)

	// 统计操作
	GetRatingStats(appID string) (*RatingStats, error)
	GetRatingTrend(appID string, days int) ([]TrendPoint, error)
}

// MemoryStore 内存存储实现（用于测试）
type MemoryStore struct {
	mu        sync.RWMutex
	feedbacks map[string]*Feedback
	replies   map[string]*FeedbackReply
	votes     map[string]map[string]*Vote   // feedbackID -> userID -> vote
	reports   map[string]map[string]*Report // feedbackID -> userID -> report
}

// NewMemoryStore 创建内存存储
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		feedbacks: make(map[string]*Feedback),
		replies:   make(map[string]*FeedbackReply),
		votes:     make(map[string]map[string]*Vote),
		reports:   make(map[string]map[string]*Report),
	}
}

func (s *MemoryStore) CreateFeedback(feedback *Feedback) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.feedbacks[feedback.ID] = feedback
	return nil
}

func (s *MemoryStore) GetFeedback(id string) (*Feedback, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.feedbacks[id]
	if !ok {
		return nil, ErrFeedbackNotFound
	}
	return f, nil
}

func (s *MemoryStore) UpdateFeedback(feedback *Feedback) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.feedbacks[feedback.ID]; !ok {
		return ErrFeedbackNotFound
	}
	s.feedbacks[feedback.ID] = feedback
	return nil
}

func (s *MemoryStore) DeleteFeedback(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.feedbacks[id]; !ok {
		return ErrFeedbackNotFound
	}
	delete(s.feedbacks, id)
	delete(s.votes, id)
	delete(s.reports, id)
	return nil
}

func (s *MemoryStore) ListFeedbacks(params ListFeedbackParams) (*PaginatedFeedbacks, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []*Feedback
	for _, f := range s.feedbacks {
		if params.AppID != "" && f.AppID != params.AppID {
			continue
		}
		if params.Version != "" && f.Version != params.Version {
			continue
		}
		if params.Category != "" && f.Category != params.Category {
			continue
		}
		filtered = append(filtered, f)
	}

	// 排序
	switch params.Sort {
	case SortNewest:
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
		})
	case SortHighest:
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Rating > filtered[j].Rating
		})
	case SortLowest:
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Rating < filtered[j].Rating
		})
	case SortMostHelpful:
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Helpful > filtered[j].Helpful
		})
	default:
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
		})
	}

	// 分页
	total := len(filtered)
	page := params.Page
	if page < 1 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	return &PaginatedFeedbacks{
		Feedbacks: convertFeedbackSlice(filtered[start:end]),
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
		HasMore:   end < total,
	}, nil
}

func convertFeedbackSlice(feedbacks []*Feedback) []Feedback {
	result := make([]Feedback, len(feedbacks))
	for i, f := range feedbacks {
		result[i] = *f
	}
	return result
}

func (s *MemoryStore) CreateReply(reply *FeedbackReply) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.replies[reply.ID] = reply
	if f, ok := s.feedbacks[reply.FeedbackID]; ok {
		f.ReplyCount++
	}
	return nil
}

func (s *MemoryStore) GetReply(id string) (*FeedbackReply, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.replies[id]
	if !ok {
		return nil, ErrReplyNotFound
	}
	return r, nil
}

func (s *MemoryStore) ListReplies(feedbackID string) ([]*FeedbackReply, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*FeedbackReply
	for _, r := range s.replies {
		if r.FeedbackID == feedbackID {
			result = append(result, r)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})

	return result, nil
}

func (s *MemoryStore) DeleteReply(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.replies[id]
	if !ok {
		return ErrReplyNotFound
	}
	if f, ok := s.feedbacks[r.FeedbackID]; ok {
		f.ReplyCount--
	}
	delete(s.replies, id)
	return nil
}

func (s *MemoryStore) CreateVote(vote *Vote) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.votes[vote.FeedbackID]; !ok {
		s.votes[vote.FeedbackID] = make(map[string]*Vote)
	}
	if _, ok := s.votes[vote.FeedbackID][vote.UserID]; ok {
		return ErrAlreadyVoted
	}

	s.votes[vote.FeedbackID][vote.UserID] = vote

	if f, ok := s.feedbacks[vote.FeedbackID]; ok {
		if vote.IsHelpful {
			f.Helpful++
		} else {
			f.NotHelpful++
		}
	}

	return nil
}

func (s *MemoryStore) GetVote(feedbackID, userID string) (*Vote, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if votes, ok := s.votes[feedbackID]; ok {
		if v, ok := votes[userID]; ok {
			return v, nil
		}
	}
	return nil, nil
}

func (s *MemoryStore) DeleteVote(feedbackID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	votes, ok := s.votes[feedbackID]
	if !ok {
		return nil
	}
	v, ok := votes[userID]
	if !ok {
		return nil
	}

	if f, ok := s.feedbacks[feedbackID]; ok {
		if v.IsHelpful {
			f.Helpful--
		} else {
			f.NotHelpful--
		}
	}

	delete(votes, userID)
	return nil
}

func (s *MemoryStore) CreateReport(report *Report) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.reports[report.FeedbackID]; !ok {
		s.reports[report.FeedbackID] = make(map[string]*Report)
	}
	if _, ok := s.reports[report.FeedbackID][report.UserID]; ok {
		return ErrAlreadyReported
	}

	s.reports[report.FeedbackID][report.UserID] = report

	if f, ok := s.feedbacks[report.FeedbackID]; ok {
		f.ReportCount++
	}

	return nil
}

func (s *MemoryStore) GetReport(feedbackID, userID string) (*Report, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if reports, ok := s.reports[feedbackID]; ok {
		if r, ok := reports[userID]; ok {
			return r, nil
		}
	}
	return nil, nil
}

func (s *MemoryStore) GetRatingStats(appID string) (*RatingStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &RatingStats{
		AppID:        appID,
		Distribution: make(map[int]int),
	}

	var totalRating float64
	for _, f := range s.feedbacks {
		if f.AppID != appID {
			continue
		}
		stats.TotalCount++
		totalRating += float64(f.Rating)
		star := int(f.Rating)
		if star < 1 {
			star = 1
		}
		if star > 5 {
			star = 5
		}
		stats.Distribution[star]++
	}

	if stats.TotalCount > 0 {
		stats.Average = totalRating / float64(stats.TotalCount)
	}

	return stats, nil
}

func (s *MemoryStore) GetRatingTrend(appID string, days int) ([]TrendPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 按日期聚合
	dailyData := make(map[string]struct {
		totalRating float64
		count       int
	})

	now := time.Now()
	startDate := now.AddDate(0, 0, -days)

	for _, f := range s.feedbacks {
		if f.AppID != appID {
			continue
		}
		if f.CreatedAt.Before(startDate) {
			continue
		}
		dateKey := f.CreatedAt.Format("2006-01-02")
		data := dailyData[dateKey]
		data.totalRating += float64(f.Rating)
		data.count++
		dailyData[dateKey] = data
	}

	var trend []TrendPoint
	for i := 0; i < days; i++ {
		date := startDate.AddDate(0, 0, i+1)
		dateKey := date.Format("2006-01-02")
		data := dailyData[dateKey]
		point := TrendPoint{
			Date:  dateKey,
			Count: data.count,
		}
		if data.count > 0 {
			point.Average = data.totalRating / float64(data.count)
		}
		trend = append(trend, point)
	}

	return trend, nil
}

// Service 反馈服务
type Service struct {
	store Store
}

// NewService 创建反馈服务
func NewService(store Store) *Service {
	return &Service{store: store}
}

// CreateFeedback 创建反馈
func (s *Service) CreateFeedback(userID string, req CreateFeedbackRequest) (*Feedback, error) {
	if !req.Rating.Validate() {
		return nil, ErrInvalidRating
	}

	if req.Category == "" {
		req.Category = CategoryPraise
	}

	now := time.Now()
	feedback := &Feedback{
		ID:        generateID(),
		AppID:     req.AppID,
		Version:   req.Version,
		UserID:    userID,
		Rating:    req.Rating,
		Title:     req.Title,
		Content:   req.Content,
		Category:  req.Category,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.store.CreateFeedback(feedback); err != nil {
		return nil, err
	}

	return feedback, nil
}

// GetFeedback 获取反馈
func (s *Service) GetFeedback(id string) (*Feedback, error) {
	return s.store.GetFeedback(id)
}

// UpdateFeedback 更新反馈
func (s *Service) UpdateFeedback(id, userID string, req CreateFeedbackRequest) (*Feedback, error) {
	feedback, err := s.store.GetFeedback(id)
	if err != nil {
		return nil, err
	}

	if feedback.UserID != userID {
		return nil, errors.New("unauthorized: can only update own feedback")
	}

	if !req.Rating.Validate() {
		return nil, ErrInvalidRating
	}

	feedback.Rating = req.Rating
	feedback.Title = req.Title
	feedback.Content = req.Content
	feedback.Category = req.Category
	feedback.Version = req.Version
	feedback.UpdatedAt = time.Now()

	if err := s.store.UpdateFeedback(feedback); err != nil {
		return nil, err
	}

	return feedback, nil
}

// DeleteFeedback 删除反馈
func (s *Service) DeleteFeedback(id, userID string) error {
	feedback, err := s.store.GetFeedback(id)
	if err != nil {
		return err
	}

	if feedback.UserID != userID {
		return errors.New("unauthorized: can only delete own feedback")
	}

	return s.store.DeleteFeedback(id)
}

// ListFeedbacks 列出反馈
func (s *Service) ListFeedbacks(params ListFeedbackParams) (*PaginatedFeedbacks, error) {
	return s.store.ListFeedbacks(params)
}

// CreateReply 创建回复
func (s *Service) CreateReply(feedbackID, userID string, req CreateReplyRequest) (*FeedbackReply, error) {
	_, err := s.store.GetFeedback(feedbackID)
	if err != nil {
		return nil, err
	}

	reply := &FeedbackReply{
		ID:         generateID(),
		FeedbackID: feedbackID,
		UserID:     userID,
		IsDev:      req.IsDev,
		Content:    req.Content,
		CreatedAt:  time.Now(),
	}

	if err := s.store.CreateReply(reply); err != nil {
		return nil, err
	}

	return reply, nil
}

// ListReplies 列出回复
func (s *Service) ListReplies(feedbackID string) ([]*FeedbackReply, error) {
	return s.store.ListReplies(feedbackID)
}

// DeleteReply 删除回复
func (s *Service) DeleteReply(replyID, userID string) error {
	reply, err := s.store.GetReply(replyID)
	if err != nil {
		return err
	}

	if reply.UserID != userID {
		return errors.New("unauthorized: can only delete own reply")
	}

	return s.store.DeleteReply(replyID)
}

// Vote 投票
func (s *Service) Vote(feedbackID, userID string, req CreateVoteRequest) (*Vote, error) {
	_, err := s.store.GetFeedback(feedbackID)
	if err != nil {
		return nil, err
	}

	// 检查是否已投票，如果已投票则更新
	existing, _ := s.store.GetVote(feedbackID, userID)
	if existing != nil {
		// 先删除旧投票
		_ = s.store.DeleteVote(feedbackID, userID)
	}

	vote := &Vote{
		ID:         generateID(),
		FeedbackID: feedbackID,
		UserID:     userID,
		IsHelpful:  req.IsHelpful,
		CreatedAt:  time.Now(),
	}

	if err := s.store.CreateVote(vote); err != nil {
		return nil, err
	}

	return vote, nil
}

// Report 举报
func (s *Service) Report(feedbackID, userID string, req CreateReportRequest) (*Report, error) {
	_, err := s.store.GetFeedback(feedbackID)
	if err != nil {
		return nil, err
	}

	report := &Report{
		ID:         generateID(),
		FeedbackID: feedbackID,
		UserID:     userID,
		Reason:     req.Reason,
		Details:    req.Details,
		CreatedAt:  time.Now(),
	}

	if err := s.store.CreateReport(report); err != nil {
		return nil, err
	}

	return report, nil
}

// GetRatingStats 获取评分统计
func (s *Service) GetRatingStats(appID string) (*RatingStats, error) {
	stats, err := s.store.GetRatingStats(appID)
	if err != nil {
		return nil, err
	}

	// 获取趋势数据（最近30天）
	trend, err := s.store.GetRatingTrend(appID, 30)
	if err != nil {
		return nil, err
	}
	stats.Trend = trend

	return stats, nil
}

// GenerateSummary 生成评论摘要
func (s *Service) GenerateSummary(appID string) (*CommentSummary, error) {
	params := ListFeedbackParams{
		AppID:    appID,
		PageSize: 1000, // 获取足够多的评论
		Sort:     SortNewest,
	}

	result, err := s.store.ListFeedbacks(params)
	if err != nil {
		return nil, err
	}

	summary := &CommentSummary{
		AppID:       appID,
		TotalCount:  result.Total,
		GeneratedAt: time.Now(),
	}

	// 提取关键词和正负面评论
	keywordCount := make(map[string]int)
	var positive, negative []string

	for _, f := range result.Feedbacks {
		// 简单关键词提取
		words := strings.Fields(strings.ToLower(f.Content))
		for _, word := range words {
			word = strings.Trim(word, ".,!?;:")
			if len(word) > 3 {
				keywordCount[word]++
			}
		}

		// 分类正负面评论
		if f.Rating >= 4 {
			positive = append(positive, truncateString(f.Content, 100))
		} else if f.Rating <= 2 {
			negative = append(negative, truncateString(f.Content, 100))
		}
	}

	// 获取前10个高频关键词
	type keywordFreq struct {
		word  string
		count int
	}
	var keywords []keywordFreq
	for word, count := range keywordCount {
		keywords = append(keywords, keywordFreq{word, count})
	}
	sort.Slice(keywords, func(i, j int) bool {
		return keywords[i].count > keywords[j].count
	})

	for i, kw := range keywords {
		if i >= 10 {
			break
		}
		summary.Keywords = append(summary.Keywords, kw.word)
	}

	// 限制正负面评论数量
	if len(positive) > 5 {
		positive = positive[:5]
	}
	if len(negative) > 5 {
		negative = negative[:5]
	}

	summary.Positive = positive
	summary.Negative = negative

	return summary, nil
}

// 辅助函数
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
