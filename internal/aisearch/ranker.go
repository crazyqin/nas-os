// Package aisearch 提供搜索结果排序算法
package aisearch

import (
	"math"
	"sort"
	"strings"
	"time"
)

// Ranker 结果排序器
type Ranker struct {
	weights *RankWeights
}

// RankWeights 排序权重配置
type RankWeights struct {
	TextRelevance float64 `json:"textRelevance"` // 文本相关性权重
	VectorSimilarity float64 `json:"vectorSimilarity"` // 语义相似度权重
	Recency       float64 `json:"recency"`       // 时间新鲜度权重
	Frequency     float64 `json:"frequency"`     // 使用频率权重
	FileSize      float64 `json:"fileSize"`      // 文件大小权重
	NameMatch     float64 `json:"nameMatch"`     // 文件名匹配权重
	TagMatch      float64 `json:"tagMatch"`      // 标签匹配权重
}

// NewRanker 创建排序器
func NewRanker(weights *RankWeights) *Ranker {
	if weights == nil {
		weights = DefaultRankWeights()
	}
	return &Ranker{weights: weights}
}

// DefaultRankWeights 默认排序权重
func DefaultRankWeights() *RankWeights {
	return &RankWeights{
		TextRelevance:    0.35,
		VectorSimilarity: 0.25,
		Recency:          0.15,
		Frequency:        0.10,
		FileSize:         0.05,
		NameMatch:        0.07,
		TagMatch:         0.03,
	}
}

// Rank 排序结果
func (r *Ranker) Rank(results []SearchResult, query *SearchQuery) []SearchResult {
	if len(results) == 0 {
		return results
	}

	// 计算每个结果的综合得分
	for i := range results {
		results[i].Score = r.calculateScore(results[i], query)
	}

	// 按综合得分排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// calculateScore 计算综合得分
func (r *Ranker) calculateScore(result SearchResult, query *SearchQuery) float64 {
	score := 0.0

	// 文本相关性得分
	textScore := r.normalizeTextScore(result.TextScore)
	score += textScore * r.weights.TextRelevance

	// 语义相似度得分
	vectorScore := result.VectorScore
	score += vectorScore * r.weights.VectorSimilarity

	// 时间新鲜度得分
	recencyScore := r.calculateRecencyScore(result.ModifiedAt)
	score += recencyScore * r.weights.Recency

	// 文件名匹配得分
	nameScore := r.calculateNameMatchScore(result.FileName, query.Keyword)
	score += nameScore * r.weights.NameMatch

	// 标签匹配得分
	tagScore := r.calculateTagMatchScore(result.Tags, query.Keyword)
	score += tagScore * r.weights.TagMatch

	// 文件大小得分 (偏好中等大小文件)
	sizeScore := r.calculateSizeScore(result.FileSize)
	score += sizeScore * r.weights.FileSize

	return score
}

// normalizeTextScore 归一化文本得分
func (r *Ranker) normalizeTextScore(score float64) float64 {
	// 使用 sigmoid 函数归一化到 0-1
	return 1.0 / (1.0 + math.Exp(-score/10))
}

// calculateRecencyScore 计算时间新鲜度得分
func (r *Ranker) calculateRecencyScore(modifiedAt time.Time) float64 {
	days := time.Since(modifiedAt).Hours() / 24

	// 7天内满分，之后指数衰减
	if days <= 7 {
		return 1.0
	}
	if days <= 30 {
		return 0.8
	}
	if days <= 90 {
		return 0.6
	}
	if days <= 365 {
		return 0.4
	}
	return 0.2
}

// calculateNameMatchScore 计算文件名匹配得分
func (r *Ranker) calculateNameMatchScore(fileName, keyword string) float64 {
	nameLower := strings.ToLower(fileName)
	keywordLower := strings.ToLower(keyword)

	// 完全匹配
	if nameLower == keywordLower {
		return 1.0
	}

	// 包含关键词
	if strings.Contains(nameLower, keywordLower) {
		// 关键词占比越高，得分越高
		ratio := float64(len(keyword)) / float64(len(fileName))
		return 0.5 + ratio*0.5
	}

	// 前缀匹配
	if strings.HasPrefix(nameLower, keywordLower) {
		return 0.8
	}

	// 后缀匹配
	if strings.HasSuffix(nameLower, keywordLower) {
		return 0.6
	}

	// 模糊匹配 (检查关键词的字符是否都出现在文件名中)
	matched := 0
	for _, c := range keywordLower {
		if strings.ContainsRune(nameLower, c) {
			matched++
		}
	}
	if matched == len(keywordLower) {
		return 0.3
	}

	return 0.0
}

// calculateTagMatchScore 计算标签匹配得分
func (r *Ranker) calculateTagMatchScore(tags []string, keyword string) float64 {
	if len(tags) == 0 {
		return 0.0
	}

	keywordLower := strings.ToLower(keyword)
	maxScore := 0.0

	for _, tag := range tags {
		tagLower := strings.ToLower(tag)

		// 完全匹配
		if tagLower == keywordLower {
			return 1.0
		}

		// 包含匹配
		if strings.Contains(tagLower, keywordLower) {
			score := float64(len(keyword)) / float64(len(tag))
			if score > maxScore {
				maxScore = score
			}
		}
	}

	return maxScore
}

// calculateSizeScore 计算文件大小得分 (偏好中等大小文件)
func (r *Ranker) calculateSizeScore(size int64) float64 {
	// 100KB - 10MB 的文件得分最高
	const (
		minOptimal = 100 * 1024        // 100KB
		maxOptimal = 10 * 1024 * 1024  // 10MB
	)

	if size >= minOptimal && size <= maxOptimal {
		return 1.0
	}

	if size < minOptimal {
		// 太小的文件得分较低
		ratio := float64(size) / float64(minOptimal)
		return 0.3 + ratio*0.7
	}

	// 太大的文件得分随大小递减
	ratio := float64(maxOptimal) / float64(size)
	return math.Max(0.2, ratio)
}

// RankByTime 按时间排序
func (r *Ranker) RankByTime(results []SearchResult, ascending bool) []SearchResult {
	sort.Slice(results, func(i, j int) bool {
		if ascending {
			return results[i].ModifiedAt.Before(results[j].ModifiedAt)
		}
		return results[i].ModifiedAt.After(results[j].ModifiedAt)
	})
	return results
}

// RankBySize 按大小排序
func (r *Ranker) RankBySize(results []SearchResult, ascending bool) []SearchResult {
	sort.Slice(results, func(i, j int) bool {
		if ascending {
			return results[i].FileSize < results[j].FileSize
		}
		return results[i].FileSize > results[j].FileSize
	})
	return results
}

// RankByRelevance 按相关性排序
func (r *Ranker) RankByRelevance(results []SearchResult) []SearchResult {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	return results
}

// RankByFrequency 按使用频率排序
func (r *Ranker) RankByFrequency(results []SearchResult, frequencies map[string]int64) []SearchResult {
	sort.Slice(results, func(i, j int) bool {
		freqI := frequencies[results[i].ID]
		freqJ := frequencies[results[j].ID]
		return freqI > freqJ
	})
	return results
}

// Deduplicate 去重
func (r *Ranker) Deduplicate(results []SearchResult) []SearchResult {
	seen := make(map[string]bool)
	unique := make([]SearchResult, 0, len(results))

	for _, result := range results {
		if !seen[result.ID] {
			seen[result.ID] = true
			unique = append(unique, result)
		}
	}

	return unique
}

// BoostResults 提升特定结果的得分
func (r *Ranker) BoostResults(results []SearchResult, boostFunc func(*SearchResult) float64) []SearchResult {
	for i := range results {
		boost := boostFunc(&results[i])
		results[i].Score *= boost
	}
	return results
}

// FilterByScore 按得分过滤
func (r *Ranker) FilterByScore(results []SearchResult, minScore float64) []SearchResult {
	filtered := make([]SearchResult, 0)
	for _, result := range results {
		if result.Score >= minScore {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

// NormalizeScores 归一化得分到 0-1 范围
func (r *Ranker) NormalizeScores(results []SearchResult) []SearchResult {
	if len(results) == 0 {
		return results
	}

	// 找到最大和最小得分
	maxScore := results[0].Score
	minScore := results[0].Score
	for _, result := range results[1:] {
		if result.Score > maxScore {
			maxScore = result.Score
		}
		if result.Score < minScore {
			minScore = result.Score
		}
	}

	// 归一化
	scoreRange := maxScore - minScore
	if scoreRange == 0 {
		for i := range results {
			results[i].Score = 1.0
		}
	} else {
		for i := range results {
			results[i].Score = (results[i].Score - minScore) / scoreRange
		}
	}

	return results
}
