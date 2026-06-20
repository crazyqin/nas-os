package sharedtags

import (
	"log"
	"math"
	"sort"
	"time"
)

// TagStats provides tag usage statistics and trend analysis
type TagStats struct {
	manager *TagManager
	tagger  *FileTagger
}

// NewTagStats creates a new TagStats instance
func NewTagStats(manager *TagManager, tagger *FileTagger) *TagStats {
	s := &TagStats{
		manager: manager,
		tagger:  tagger,
	}
	log.Println("标签统计系统已初始化")
	return s
}

// GetTopTags returns the most used tags
func (s *TagStats) GetTopTags(limit int) []*TagStatsResult {
	tags := s.manager.ListTags("")

	if limit > len(tags) {
		limit = len(tags)
	}

	var results []*TagStatsResult
	for i := 0; i < limit; i++ {
		tag := tags[i]
		catName := ""
		if tag.CategoryID != "" {
			if cat, err := s.manager.GetCategory(tag.CategoryID); err == nil {
				catName = cat.Name
			}
		}

		results = append(results, &TagStatsResult{
			TagID:        tag.ID,
			TagName:      tag.Name,
			FileCount:    s.tagger.CountFilesWithTag(tag.ID),
			UsageCount:   tag.UsageCount,
			LastUsedAt:   tag.UpdatedAt,
			TrendScore:   s.calculateTrendScore(tag),
			CategoryName: catName,
		})
	}

	return results
}

// GetCategoryStats returns statistics for each category
func (s *TagStats) GetCategoryStats() map[string]*CategoryStats {
	categories := s.manager.ListCategories("")
	result := make(map[string]*CategoryStats)

	for _, cat := range categories {
		tags := s.manager.ListTags(cat.ID)
		var totalFiles int64
		var totalUsage int64
		for _, tag := range tags {
			totalFiles += s.tagger.CountFilesWithTag(tag.ID)
			totalUsage += tag.UsageCount
		}

		result[cat.ID] = &CategoryStats{
			CategoryID:   cat.ID,
			CategoryName: cat.Name,
			CategoryType: string(cat.Type),
			TagCount:     int64(len(tags)),
			TotalFiles:   totalFiles,
			TotalUsage:   totalUsage,
		}
	}

	return result
}

// CategoryStats represents statistics for a category
type CategoryStats struct {
	CategoryID   string `json:"categoryId"`   // 分类ID
	CategoryName string `json:"categoryName"` // 分类名称
	CategoryType string `json:"categoryType"` // 分类类型
	TagCount     int64  `json:"tagCount"`     // 标签数量
	TotalFiles   int64  `json:"totalFiles"`   // 关联文件总数
	TotalUsage   int64  `json:"totalUsage"`   // 总使用次数
}

// GetTagTrend returns usage trend data for a tag over a period
func (s *TagStats) GetTagTrend(tagID string, days int) []*TagTrendPoint {
	if days <= 0 {
		days = 30
	}

	tag, err := s.manager.GetTag(tagID)
	if err != nil {
		return nil
	}

	files := s.tagger.GetTagFiles(tagID)
	now := time.Now()

	// Group files by day
	dayCount := make(map[string]int64)
	for _, ft := range files {
		dayKey := ft.CreatedAt.Format("2006-01-02")
		dayCount[dayKey]++
	}

	var points []*TagTrendPoint
	var cumulative int64

	for i := days - 1; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		dayKey := date.Format("2006-01-02")
		newFiles := dayCount[dayKey]
		cumulative += newFiles

		points = append(points, &TagTrendPoint{
			Date:       date,
			TagID:      tag.ID,
			TagName:    tag.Name,
			NewFiles:   newFiles,
			TotalFiles: cumulative,
		})
	}

	return points
}

// GetTrendingTags returns tags with increasing usage trends
func (s *TagStats) GetTrendingTags(days int, limit int) []*TagStatsResult {
	tags := s.manager.ListTags("")
	var trending []*TagStatsResult

	for _, tag := range tags {
		score := s.calculateTrendScore(tag)
		if score > 0 {
			catName := ""
			if tag.CategoryID != "" {
				if cat, err := s.manager.GetCategory(tag.CategoryID); err == nil {
					catName = cat.Name
				}
			}
			trending = append(trending, &TagStatsResult{
				TagID:        tag.ID,
				TagName:      tag.Name,
				FileCount:    s.tagger.CountFilesWithTag(tag.ID),
				UsageCount:   tag.UsageCount,
				LastUsedAt:   tag.UpdatedAt,
				TrendScore:   score,
				CategoryName: catName,
			})
		}
	}

	sort.Slice(trending, func(i, j int) bool {
		return trending[i].TrendScore > trending[j].TrendScore
	})

	if limit > len(trending) {
		limit = len(trending)
	}

	return trending[:limit]
}

// GetUnusedTags returns tags that have no associated files
func (s *TagStats) GetUnusedTags() []*Tag {
	tags := s.manager.ListTags("")
	var unused []*Tag

	for _, tag := range tags {
		if s.tagger.CountFilesWithTag(tag.ID) == 0 {
			unused = append(unused, tag)
		}
	}

	return unused
}

// GetTagSummary returns a summary of all tag statistics
func (s *TagStats) GetTagSummary() *TagSummary {
	tags := s.manager.ListTags("")
	categories := s.manager.ListCategories("")

	var totalFiles int64
	var totalUsage int64
	var activeTags int64

	for _, tag := range tags {
		fileCount := s.tagger.CountFilesWithTag(tag.ID)
		totalFiles += fileCount
		totalUsage += tag.UsageCount
		if fileCount > 0 {
			activeTags++
		}
	}

	return &TagSummary{
		TotalTags:      int64(len(tags)),
		ActiveTags:     activeTags,
		TotalCategories: int64(len(categories)),
		TotalFiles:     totalFiles,
		TotalUsage:     totalUsage,
	}
}

// TagSummary represents overall tag system summary
type TagSummary struct {
	TotalTags       int64 `json:"totalTags"`       // 总标签数
	ActiveTags      int64 `json:"activeTags"`      // 活跃标签数（有关联文件）
	TotalCategories int64 `json:"totalCategories"` // 总分类数
	TotalFiles      int64 `json:"totalFiles"`      // 总关联文件数
	TotalUsage      int64 `json:"totalUsage"`      // 总使用次数
}

// calculateTrendScore calculates a trend score based on recent activity
func (s *TagStats) calculateTrendScore(tag *Tag) float64 {
	files := s.tagger.GetTagFiles(tag.ID)
	if len(files) == 0 {
		return 0
	}

	now := time.Now()
	recentWindow := 7 * 24 * time.Hour // 7 days
	oldWindow := 30 * 24 * time.Hour    // 30 days

	var recentCount, oldCount int64
	for _, ft := range files {
		age := now.Sub(ft.CreatedAt)
		if age <= recentWindow {
			recentCount++
		} else if age <= oldWindow {
			oldCount++
		}
	}

	// Trend score: recent activity relative to older activity
	if oldCount == 0 {
		if recentCount > 0 {
			return 100.0 // New trending tag
		}
		return 0
	}

	score := (float64(recentCount) / float64(oldCount)) * 50.0
	return math.Min(score, 100.0)
}

// GetTagUsageByCategory returns tag usage grouped by category
func (s *TagStats) GetTagUsageByCategory() map[string]int64 {
	tags := s.manager.ListTags("")
	result := make(map[string]int64)

	for _, tag := range tags {
		catName := "未分类"
		if tag.CategoryID != "" {
			if cat, err := s.manager.GetCategory(tag.CategoryID); err == nil {
				catName = cat.Name
			}
		}
		result[catName] += s.tagger.CountFilesWithTag(tag.ID)
	}

	return result
}
