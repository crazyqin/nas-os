// Package media provides fuzzy matching for media titles
package media

import (
	"math"
	"regexp"
	"strings"
	"sync"
)

// FuzzyMatcher 模糊匹配器 - 用于标题匹配评分
type FuzzyMatcher struct {
	threshold float64
	cache     map[string]map[string]float64
	mu        sync.RWMutex
}

// NewFuzzyMatcher 创建模糊匹配器
func NewFuzzyMatcher(threshold float64) *FuzzyMatcher {
	if threshold < 0 {
		threshold = 0.75
	}
	if threshold > 1 {
		threshold = 1
	}
	return &FuzzyMatcher{
		threshold: threshold,
		cache:     make(map[string]map[string]float64),
	}
}

// Score 计算两个标题的匹配分数
func (m *FuzzyMatcher) Score(title1, title2 string) float64 {
	// 预处理
	t1 := normalizeTitle(title1)
	t2 := normalizeTitle(title2)

	// 检查缓存
	m.mu.RLock()
	if cached, ok := m.cache[t1]; ok {
		if score, ok := cached[t2]; ok {
			m.mu.RUnlock()
			return score
		}
	}
	m.mu.RUnlock()

	// 计算多种匹配分数
	scores := []float64{
		m.levenshteinScore(t1, t2),
		m.jaccardScore(t1, t2),
		m.tokenMatchScore(t1, t2),
		m.acronymScore(t1, t2),
	}

	// 取最高分
	maxScore := 0.0
	for _, s := range scores {
		if s > maxScore {
			maxScore = s
		}
	}

	// 缓存结果
	m.mu.Lock()
	if m.cache[t1] == nil {
		m.cache[t1] = make(map[string]float64)
	}
	m.cache[t1][t2] = maxScore
	m.mu.Unlock()

	return maxScore
}

// IsMatch 检查是否匹配（高于阈值）
func (m *FuzzyMatcher) IsMatch(title1, title2 string) bool {
	return m.Score(title1, title2) >= m.threshold
}

// BestMatch 从候选列表中找到最佳匹配
func (m *FuzzyMatcher) BestMatch(title string, candidates []string) (string, float64) {
	bestCandidate := ""
	bestScore := 0.0

	for _, c := range candidates {
		score := m.Score(title, c)
		if score > bestScore {
			bestScore = score
			bestCandidate = c
		}
	}

	return bestCandidate, bestScore
}

// FindMatches 找到所有匹配的候选（高于阈值）
func (m *FuzzyMatcher) FindMatches(title string, candidates []string) []MatchResult {
	results := make([]MatchResult, 0)
	for _, c := range candidates {
		score := m.Score(title, c)
		if score >= m.threshold {
			results = append(results, MatchResult{
				Title:  c,
				Score:  score,
			})
		}
	}
	return results
}

// MatchResult 匹配结果
type MatchResult struct {
	Title string
	Score float64
}

// ====== 各种匹配算法 ======

// levenshteinScore 基于 Levenshtein 距离的相似度
func (m *FuzzyMatcher) levenshteinScore(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	// Levenshtein 距离
	dist := levenshteinDistance(s1, s2)

	// 相似度 = 1 - dist / max(len1, len2)
	maxLen := math.Max(float64(len(s1)), float64(len(s2)))
	if maxLen == 0 {
		return 1.0
	}

	return 1.0 - float64(dist)/maxLen
}

// levenshteinDistance 计算 Levenshtein 距离
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// 使用 DP 优化版本
	// 只保留两行
	prevRow := make([]int, len(s2)+1)
	currRow := make([]int, len(s2)+1)

	// 初始化第一行
	for j := 0; j <= len(s2); j++ {
		prevRow[j] = j
	}

	for i := 1; i <= len(s1); i++ {
		currRow[0] = i
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			currRow[j] = min(
				prevRow[j]+1,     // 删除
				currRow[j-1]+1,   // 插入
				prevRow[j-1]+cost, // 替换
			)
		}
		// 交换行
		prevRow, currRow = currRow, prevRow
	}

	return prevRow[len(s2)]
}

// jaccardScore Jaccard 相似度（基于词集合）
func (m *FuzzyMatcher) jaccardScore(s1, s2 string) float64 {
	tokens1 := tokenize(s1)
	tokens2 := tokenize(s2)

	if len(tokens1) == 0 && len(tokens2) == 0 {
		return 1.0
	}

	// 计算交集和并集
	intersection := 0
	set1 := make(map[string]bool)
	for _, t := range tokens1 {
		set1[t] = true
	}

	set2 := make(map[string]bool)
	for _, t := range tokens2 {
		set2[t] = true
		if set1[t] {
			intersection++
		}
	}

	union := len(set1) + len(set2) - intersection
	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

// tokenMatchScore 词匹配分数（考虑顺序）
func (m *FuzzyMatcher) tokenMatchScore(s1, s2 string) float64 {
	tokens1 := tokenize(s1)
	tokens2 := tokenize(s2)

	if len(tokens1) == 0 || len(tokens2) == 0 {
		return 0
	}

	// 计算连续匹配
	matches := 0
	maxMatches := min(len(tokens1), len(tokens2))

	for i := 0; i < maxMatches; i++ {
		if tokens1[i] == tokens2[i] {
			matches++
		}
	}

	// 加权：开头匹配更重要
	score := float64(matches) / float64(maxMatches)

	// 如果标题开头相同，额外加分
	if len(tokens1) > 0 && len(tokens2) > 0 && tokens1[0] == tokens2[0] {
		score += 0.1
	}

	return minFloat(score, 1.0)
}

// acronymScore 首字母缩写匹配
func (m *FuzzyMatcher) acronymScore(s1, s2 string) float64 {
	// 提取首字母
	acr1 := acronym(s1)
	acr2 := acronym(s2)

	if len(acr1) == 0 || len(acr2) == 0 {
		return 0
	}

	// 检查其中一个是否是另一个的子串
	if strings.Contains(acr1, acr2) || strings.Contains(acr2, acr1) {
		return 0.85
	}

	// 计算相似度
	sim := m.levenshteinScore(acr1, acr2)
	return sim * 0.7 // 缩写匹配权重较低
}

// ====== 辅助函数 ======

// normalizeTitle 标准化标题
func normalizeTitle(title string) string {
	// 转小写
	title = strings.ToLower(title)

	// 移除标点
	title = regexp.MustCompile(`[^\w\s\p{Han}]`).ReplaceAllString(title, "")

	// 移除多余空格
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")

	return strings.TrimSpace(title)
}

// tokenize 分词
func tokenize(s string) []string {
	// 按空格分词
	tokens := strings.Split(s, " ")

	// 过滤空词
	result := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if t != "" {
			result = append(result, t)
		}
	}

	// 中文分词（简单：每字一词）
	// 这里简化处理，实际应该用更复杂的分词算法
	for _, c := range s {
		if c >= 0x4E00 && c <= 0x9FFF { // 汉字范围
			result = append(result, string(c))
		}
	}

	return result
}

// acronym 提取首字母缩写
func acronym(s string) string {
	tokens := tokenize(s)
	result := ""
	for _, t := range tokens {
		if len(t) > 0 {
			// 英文取首字母
			if t[0] >= 'a' && t[0] <= 'z' {
				result += string(t[0])
			} else {
				// 中文或其它字符保留完整词
				result += t
			}
		}
	}
	return result
}

// minFloat 取最小值（浮点版）
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// minInt 取最小值（整数版）
func minInt(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}