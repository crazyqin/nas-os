package docker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// AppRating 应用评分
type AppRating struct {
	ID         string    `json:"id"`
	TemplateID string    `json:"templateId"`
	UserID     string    `json:"userId"`
	UserName   string    `json:"userName"`
	Rating     int       `json:"rating"` // 1-5 星
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	Helpful    int       `json:"helpful"`   // 有用数
	HelpfulBy  []string  `json:"helpfulBy"` // 点赞用户
	Verified   bool      `json:"verified"`  // 已验证购买
}

// RatingStats 评分统计
type RatingStats struct {
	TemplateID   string  `json:"templateId"`
	TotalReviews int     `json:"totalReviews"`
	AverageScore float64 `json:"averageScore"`
	Distribution struct {
		Five  int `json:"five"`
		Four  int `json:"four"`
		Three int `json:"three"`
		Two   int `json:"two"`
		One   int `json:"one"`
	} `json:"distribution"`
}

// RatingManager 评分管理器
type RatingManager struct {
	dataDir  string
	dataFile string
	ratings  map[string][]*AppRating // templateID -> ratings
	stats    map[string]*RatingStats // templateID -> stats
	mu       sync.RWMutex
}

// NewRatingManager 创建评分管理器
func NewRatingManager(dataDir string) (*RatingManager, error) {
	dataFile := filepath.Join(dataDir, "app-ratings.json")

	rm := &RatingManager{
		dataDir:  dataDir,
		dataFile: dataFile,
		ratings:  make(map[string][]*AppRating),
		stats:    make(map[string]*RatingStats),
	}

	// 创建目录
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	// 加载数据
	if err := rm.load(); err != nil {
		// 文件不存在不影响启动
		fmt.Printf("加载评分数据失败: %v\n", err)
	}

	return rm, nil
}

// load 加载数据
func (rm *RatingManager) load() error {
	data, err := os.ReadFile(rm.dataFile)
	if err != nil {
		return err
	}

	var allRatings []*AppRating
	if err := json.Unmarshal(data, &allRatings); err != nil {
		return err
	}

	// 按 templateID 分组
	for _, r := range allRatings {
		rm.ratings[r.TemplateID] = append(rm.ratings[r.TemplateID], r)
	}

	// 计算统计
	for templateID := range rm.ratings {
		rm.calculateStats(templateID)
	}

	return nil
}

// save 保存数据
func (rm *RatingManager) save() error {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var allRatings []*AppRating
	for _, ratings := range rm.ratings {
		allRatings = append(allRatings, ratings...)
	}

	data, err := json.MarshalIndent(allRatings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(rm.dataFile, data, 0644)
}

// calculateStats 计算统计
func (rm *RatingManager) calculateStats(templateID string) {
	ratings, ok := rm.ratings[templateID]
	if !ok {
		return
	}

	stats := &RatingStats{
		TemplateID:   templateID,
		TotalReviews: len(ratings),
	}

	var total float64
	for _, r := range ratings {
		total += float64(r.Rating)
		switch r.Rating {
		case 5:
			stats.Distribution.Five++
		case 4:
			stats.Distribution.Four++
		case 3:
			stats.Distribution.Three++
		case 2:
			stats.Distribution.Two++
		case 1:
			stats.Distribution.One++
		}
	}

	if len(ratings) > 0 {
		stats.AverageScore = total / float64(len(ratings))
	}

	rm.stats[templateID] = stats
}

// AddRating 添加评分
func (rm *RatingManager) AddRating(templateID, userID, userName string, rating int, title, content string) (*AppRating, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// 验证评分
	if rating < 1 || rating > 5 {
		return nil, fmt.Errorf("评分必须在 1-5 之间")
	}

	// 检查是否已评分
	for _, r := range rm.ratings[templateID] {
		if r.UserID == userID {
			// 更新已有评分
			r.Rating = rating
			r.Title = title
			r.Content = content
			r.UpdatedAt = time.Now()
			rm.calculateStats(templateID)
			rm.save()
			return r, nil
		}
	}

	// 创建新评分
	r := &AppRating{
		ID:         fmt.Sprintf("rating-%d", time.Now().UnixNano()),
		TemplateID: templateID,
		UserID:     userID,
		UserName:   userName,
		Rating:     rating,
		Title:      title,
		Content:    content,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	rm.ratings[templateID] = append(rm.ratings[templateID], r)
	rm.calculateStats(templateID)
	rm.save()

	return r, nil
}

// GetRatings 获取评分列表
func (rm *RatingManager) GetRatings(templateID string, sortby string, limit, offset int) []*AppRating {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	ratings, ok := rm.ratings[templateID]
	if !ok {
		return []*AppRating{}
	}

	// 复制以避免修改原数据
	result := make([]*AppRating, len(ratings))
	copy(result, ratings)

	// 排序
	switch sortby {
	case "helpful":
		sort.Slice(result, func(i, j int) bool {
			return result[i].Helpful > result[j].Helpful
		})
	case "recent":
		sort.Slice(result, func(i, j int) bool {
			return result[i].CreatedAt.After(result[j].CreatedAt)
		})
	case "rating_high":
		sort.Slice(result, func(i, j int) bool {
			return result[i].Rating > result[j].Rating
		})
	case "rating_low":
		sort.Slice(result, func(i, j int) bool {
			return result[i].Rating < result[j].Rating
		})
	default:
		// 默认按时间排序
		sort.Slice(result, func(i, j int) bool {
			return result[i].CreatedAt.After(result[j].CreatedAt)
		})
	}

	// 分页
	if offset >= len(result) {
		return []*AppRating{}
	}

	end := offset + limit
	if end > len(result) {
		end = len(result)
	}

	return result[offset:end]
}

// GetStats 获取统计
func (rm *RatingManager) GetStats(templateID string) *RatingStats {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	stats, ok := rm.stats[templateID]
	if !ok {
		return &RatingStats{TemplateID: templateID}
	}
	return stats
}

// DeleteRating 删除评分
func (rm *RatingManager) DeleteRating(templateID, ratingID, userID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	ratings, ok := rm.ratings[templateID]
	if !ok {
		return fmt.Errorf("评分不存在")
	}

	for i, r := range ratings {
		if r.ID == ratingID {
			if r.UserID != userID {
				return fmt.Errorf("无权删除他人评分")
			}
			rm.ratings[templateID] = append(ratings[:i], ratings[i+1:]...)
			rm.calculateStats(templateID)
			rm.save()
			return nil
		}
	}

	return fmt.Errorf("评分不存在")
}

// MarkHelpful 标记有用
func (rm *RatingManager) MarkHelpful(templateID, ratingID, userID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	ratings, ok := rm.ratings[templateID]
	if !ok {
		return fmt.Errorf("评分不存在")
	}

	for _, r := range ratings {
		if r.ID == ratingID {
			// 检查是否已标记
			for _, uid := range r.HelpfulBy {
				if uid == userID {
					return fmt.Errorf("已标记过")
				}
			}
			r.Helpful++
			r.HelpfulBy = append(r.HelpfulBy, userID)
			rm.save()
			return nil
		}
	}

	return fmt.Errorf("评分不存在")
}

// VerifyPurchase 验证购买（已安装应用可标记为验证）
func (rm *RatingManager) VerifyPurchase(templateID, userID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	ratings, ok := rm.ratings[templateID]
	if !ok {
		return nil
	}

	for _, r := range ratings {
		if r.UserID == userID {
			r.Verified = true
			rm.save()
			return nil
		}
	}

	return nil
}

// GetUserRating 获取用户评分
func (rm *RatingManager) GetUserRating(templateID, userID string) *AppRating {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	ratings, ok := rm.ratings[templateID]
	if !ok {
		return nil
	}

	for _, r := range ratings {
		if r.UserID == userID {
			return r
		}
	}

	return nil
}
