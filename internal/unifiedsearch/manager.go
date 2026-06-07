// Package unifiedsearch 提供核心管理逻辑
package unifiedsearch

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// Manager 统一搜索管理器
type Manager struct {
	mu          sync.RWMutex
	index       map[string]*SearchIndex  // id -> index entry
	pathIndex   map[string]string        // path -> id
	tagIndex    map[string][]string      // tag -> []id
	typeIndex   map[ContentType][]string // type -> []id
	tasks       map[string]*IndexTask
	history     []*SearchHistory
	hotSearches map[string]int // query -> count
	stats       *IndexStats
	config      *SearchConfig
	stopChan    chan struct{}
	running     bool
}

// SearchConfig 搜索配置
type SearchConfig struct {
	MaxHistory      int     `json:"max_history"`
	MaxHotSearches  int     `json:"max_hot_searches"`
	FuzzyThreshold  float64 `json:"fuzzy_threshold"`
	HighlightPre    string  `json:"highlight_pre"`
	HighlightPost   string  `json:"highlight_post"`
	SummaryLength   int     `json:"summary_length"`
	MaxPageSize     int     `json:"max_page_size"`
	DefaultPageSize int     `json:"default_page_size"`
}

// DefaultSearchConfig 默认搜索配置
func DefaultSearchConfig() *SearchConfig {
	return &SearchConfig{
		MaxHistory:      1000,
		MaxHotSearches:  100,
		FuzzyThreshold:  0.6,
		HighlightPre:    "<mark>",
		HighlightPost:   "</mark>",
		SummaryLength:   200,
		MaxPageSize:     100,
		DefaultPageSize: 20,
	}
}

// NewManager 创建搜索管理器
func NewManager(config *SearchConfig) *Manager {
	if config == nil {
		config = DefaultSearchConfig()
	}

	return &Manager{
		index:       make(map[string]*SearchIndex),
		pathIndex:   make(map[string]string),
		tagIndex:    make(map[string][]string),
		typeIndex:   make(map[ContentType][]string),
		tasks:       make(map[string]*IndexTask),
		history:     make([]*SearchHistory, 0),
		hotSearches: make(map[string]int),
		stats:       DefaultIndexStats(),
		config:      config,
		stopChan:    make(chan struct{}),
	}
}

// Search 执行搜索
func (m *Manager) Search(query *SearchQuery) (*SearchResponse, error) {
	if query == nil || query.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	start := time.Now()

	// 应用默认值
	m.applyQueryDefaults(query)

	// 记录搜索历史
	m.addSearchHistory(query.Query, 0)

	// 执行搜索
	results := m.executeSearch(query)

	// 排序
	m.sortResults(results, query.SortBy)

	// 分页
	total := len(results)
	totalPages := (total + query.PageSize - 1) / query.PageSize
	startIdx := (query.Page - 1) * query.PageSize
	endIdx := startIdx + query.PageSize
	if startIdx > total {
		startIdx = total
	}
	if endIdx > total {
		endIdx = total
	}
	pageResults := results[startIdx:endIdx]

	// 转换为 SearchResult
	searchResults := make([]SearchResult, 0, len(pageResults))
	for _, idx := range pageResults {
		sr := SearchResult{
			ID:          idx.ID,
			Path:        idx.Path,
			Name:        idx.Name,
			Extension:   idx.Extension,
			ContentType: idx.ContentType,
			MimeType:    idx.MimeType,
			Size:        idx.Size,
			Tags:        idx.Tags,
			Metadata:    idx.Metadata,
			Score:       idx.Score,
			CreatedAt:   idx.CreatedAt,
			ModifiedAt:  idx.ModifiedAt,
		}

		// 生成摘要
		if idx.Content != "" {
			sr.Summary = m.generateSummary(idx.Content, query.Query)
		}

		// 高亮
		if query.Highlight {
			sr.Highlights = m.generateHighlights(idx, query.Query)
		}

		searchResults = append(searchResults, sr)
	}

	// 更新搜索历史结果数
	m.updateLastSearchResultCount(len(results))

	// 生成搜索建议
	suggestions := m.generateSuggestions(query.Query)

	elapsed := time.Since(start)

	return &SearchResponse{
		Query:       query.Query,
		Total:       total,
		Page:        query.Page,
		PageSize:    query.PageSize,
		TotalPages:  totalPages,
		Results:     searchResults,
		Suggestions: suggestions,
		TimeMs:      elapsed.Milliseconds(),
	}, nil
}

// applyQueryDefaults 应用查询默认值
func (m *Manager) applyQueryDefaults(query *SearchQuery) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = m.config.DefaultPageSize
	}
	if query.PageSize > m.config.MaxPageSize {
		query.PageSize = m.config.MaxPageSize
	}
	if query.BooleanOp == "" {
		query.BooleanOp = BooleanAND
	}
	if query.SortBy == "" {
		query.SortBy = SortRelevance
	}
	if query.FuzzyLevel <= 0 {
		query.FuzzyLevel = 1
	}
}

// executeSearch 执行搜索
func (m *Manager) executeSearch(query *SearchQuery) []*SearchIndex {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 解析查询词
	terms := m.parseQueryTerms(query.Query, query.BooleanOp)

	// 收集匹配的索引
	var candidates []*SearchIndex
	for _, idx := range m.index {
		if m.matchesFilters(idx, query) {
			candidates = append(candidates, idx)
		}
	}

	// 如果指定了类型过滤，进一步过滤
	if len(query.Types) > 0 {
		filtered := make([]*SearchIndex, 0)
		for _, idx := range candidates {
			for _, t := range query.Types {
				if idx.ContentType == t {
					filtered = append(filtered, idx)
					break
				}
			}
		}
		candidates = filtered
	}

	// 打分和匹配
	results := make([]*SearchIndex, 0)
	for _, idx := range candidates {
		score := m.calculateScore(idx, terms, query)
		if score > 0 {
			idxCopy := *idx
			idxCopy.Score = score
			results = append(results, &idxCopy)
		}
	}

	return results
}

// parseQueryTerms 解析查询词
func (m *Manager) parseQueryTerms(query string, op BooleanOp) []string {
	// 移除布尔操作符关键词，提取搜索词
	query = strings.TrimSpace(query)

	// 简单分词：按空格分割
	terms := strings.Fields(query)

	// 过滤掉布尔操作符
	filtered := make([]string, 0)
	for _, t := range terms {
		upper := strings.ToUpper(t)
		if upper != "AND" && upper != "OR" && upper != "NOT" {
			filtered = append(filtered, t)
		}
	}

	return filtered
}

// matchesFilters 检查是否匹配过滤条件
func (m *Manager) matchesFilters(idx *SearchIndex, query *SearchQuery) bool {
	// 路径前缀过滤
	if query.Path != "" && !strings.HasPrefix(idx.Path, query.Path) {
		return false
	}

	// 标签过滤
	if len(query.Tags) > 0 {
		hasTag := false
		for _, qt := range query.Tags {
			for _, it := range idx.Tags {
				if strings.EqualFold(qt, it) {
					hasTag = true
					break
				}
			}
			if hasTag {
				break
			}
		}
		if !hasTag {
			return false
		}
	}

	// 日期范围过滤
	if query.DateFrom != nil && idx.ModifiedAt.Before(*query.DateFrom) {
		return false
	}
	if query.DateTo != nil && idx.ModifiedAt.After(*query.DateTo) {
		return false
	}

	// 大小范围过滤
	if query.SizeMin != nil && idx.Size < *query.SizeMin {
		return false
	}
	if query.SizeMax != nil && idx.Size > *query.SizeMax {
		return false
	}

	return true
}

// calculateScore 计算相关度评分
func (m *Manager) calculateScore(idx *SearchIndex, terms []string, query *SearchQuery) float64 {
	if len(terms) == 0 {
		return 0
	}

	score := 0.0
	matchedTerms := 0

	nameLower := strings.ToLower(idx.Name)
	contentLower := strings.ToLower(idx.Content)
	pathLower := strings.ToLower(idx.Path)

	for _, term := range terms {
		termLower := strings.ToLower(term)
		termScore := 0.0

		// 文件名匹配（权重最高）
		if strings.Contains(nameLower, termLower) {
			termScore += 10.0
			// 完全匹配文件名（不含扩展名）
			nameNoExt := strings.TrimSuffix(nameLower, strings.ToLower(idx.Extension))
			if nameNoExt == termLower {
				termScore += 5.0
			}
		}

		// 内容匹配
		if strings.Contains(contentLower, termLower) {
			termScore += 5.0
			// 词频加分
			count := strings.Count(contentLower, termLower)
			if count > 1 {
				termScore += math.Log2(float64(count)) * 2.0
			}
		}

		// 路径匹配
		if strings.Contains(pathLower, termLower) {
			termScore += 2.0
		}

		// 标签匹配
		for _, tag := range idx.Tags {
			if strings.Contains(strings.ToLower(tag), termLower) {
				termScore += 8.0
				break
			}
		}

		// 元数据匹配
		for _, v := range idx.Metadata {
			if strings.Contains(strings.ToLower(v), termLower) {
				termScore += 3.0
				break
			}
		}

		// 模糊匹配
		if query.Fuzzy && termScore == 0 {
			fuzzyScore := m.fuzzyMatch(termLower, nameLower, contentLower)
			if fuzzyScore >= m.config.FuzzyThreshold {
				termScore += fuzzyScore * 5.0
			}
		}

		if termScore > 0 {
			matchedTerms++
			score += termScore
		}
	}

	// 根据布尔操作符判断是否匹配
	switch query.BooleanOp {
	case BooleanAND:
		if matchedTerms < len(terms) {
			return 0
		}
	case BooleanOR:
		if matchedTerms == 0 {
			return 0
		}
	case BooleanNOT:
		// NOT 操作：如果所有词都不匹配则得分
		if matchedTerms > 0 {
			return 0
		}
		score = 1.0
	}

	// 时间衰减（越新越高分）
	daysSinceModified := time.Since(idx.ModifiedAt).Hours() / 24
	if daysSinceModified < 30 {
		score *= 1.2
	} else if daysSinceModified < 365 {
		score *= 1.1
	}

	return score
}

// fuzzyMatch 模糊匹配（编辑距离）
func (m *Manager) fuzzyMatch(term, name, content string) float64 {
	bestScore := 0.0

	// 对文件名进行模糊匹配
	nameWords := m.tokenize(name)
	for _, w := range nameWords {
		sim := m.similarity(term, w)
		if sim > bestScore {
			bestScore = sim
		}
	}

	// 对内容进行模糊匹配（只检查前1000字符）
	contentWords := m.tokenize(content[:min(len(content), 1000)])
	for _, w := range contentWords {
		sim := m.similarity(term, w)
		if sim > bestScore {
			bestScore = sim
		}
	}

	return bestScore
}

// similarity 计算两个字符串的相似度（基于编辑距离）
func (m *Manager) similarity(a, b string) float64 {
	if a == b {
		return 1.0
	}

	lenA := len(a)
	lenB := len(b)
	if lenA == 0 || lenB == 0 {
		return 0
	}

	// 简化的相似度计算：基于共同字符比例
	common := 0
	for _, r := range a {
		if strings.ContainsRune(b, r) {
			common++
		}
	}

	return float64(common) / math.Max(float64(lenA), float64(lenB))
}

// tokenize 分词
func (m *Manager) tokenize(text string) []string {
	text = strings.ToLower(text)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_'
	})
	return words
}

// generateSummary 生成内容摘要
func (m *Manager) generateSummary(content, query string) string {
	if content == "" {
		return ""
	}

	terms := strings.Fields(strings.ToLower(query))
	contentLower := strings.ToLower(content)

	// 查找第一个匹配项的位置
	bestPos := 0
	for _, term := range terms {
		pos := strings.Index(contentLower, term)
		if pos >= 0 && (bestPos == 0 || pos < bestPos) {
			bestPos = pos
		}
	}

	// 从匹配位置前后截取摘要
	start := bestPos - m.config.SummaryLength/4
	if start < 0 {
		start = 0
	}
	end := start + m.config.SummaryLength
	if end > len(content) {
		end = len(content)
	}

	summary := content[start:end]

	// 添加省略号
	if start > 0 {
		summary = "..." + summary
	}
	if end < len(content) {
		summary = summary + "..."
	}

	return summary
}

// generateHighlights 生成高亮
func (m *Manager) generateHighlights(idx *SearchIndex, query string) map[string]string {
	highlights := make(map[string]string)
	terms := strings.Fields(strings.ToLower(query))

	// 高亮文件名
	nameHighlighted := m.highlightText(idx.Name, terms)
	if nameHighlighted != idx.Name {
		highlights["name"] = nameHighlighted
	}

	// 高亮内容摘要
	if idx.Content != "" {
		summary := m.generateSummary(idx.Content, query)
		contentHighlighted := m.highlightText(summary, terms)
		if contentHighlighted != summary {
			highlights["content"] = contentHighlighted
		}
	}

	// 高亮路径
	pathHighlighted := m.highlightText(idx.Path, terms)
	if pathHighlighted != idx.Path {
		highlights["path"] = pathHighlighted
	}

	return highlights
}

// highlightText 高亮文本中的关键词
func (m *Manager) highlightText(text string, terms []string) string {
	result := text
	for _, term := range terms {
		termLower := strings.ToLower(term)
		// 使用正则替换（不区分大小写）
		re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(termLower))
		result = re.ReplaceAllString(result, m.config.HighlightPre+term+m.config.HighlightPost)
	}
	return result
}

// sortResults 排序结果
func (m *Manager) sortResults(results []*SearchIndex, sortBy SortOrder) {
	switch sortBy {
	case SortRelevance:
		sort.Slice(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})
	case SortDateDesc:
		sort.Slice(results, func(i, j int) bool {
			return results[i].ModifiedAt.After(results[j].ModifiedAt)
		})
	case SortDateAsc:
		sort.Slice(results, func(i, j int) bool {
			return results[i].ModifiedAt.Before(results[j].ModifiedAt)
		})
	case SortSizeDesc:
		sort.Slice(results, func(i, j int) bool {
			return results[i].Size > results[j].Size
		})
	case SortSizeAsc:
		sort.Slice(results, func(i, j int) bool {
			return results[i].Size < results[j].Size
		})
	case SortNameAsc:
		sort.Slice(results, func(i, j int) bool {
			return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
		})
	case SortNameDesc:
		sort.Slice(results, func(i, j int) bool {
			return strings.ToLower(results[i].Name) > strings.ToLower(results[j].Name)
		})
	}
}

// AddDocument 添加文档到索引
func (m *Manager) AddDocument(idx *SearchIndex) error {
	if idx == nil || idx.Path == "" {
		return fmt.Errorf("invalid document: path is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已存在
	if existingID, ok := m.pathIndex[idx.Path]; ok {
		// 更新现有条目
		existing := m.index[existingID]
		m.removeFromIndexes(existing)
		idx.ID = existingID
	} else {
		idx.ID = uuid.New().String()
	}

	if idx.IndexedAt.IsZero() {
		idx.IndexedAt = time.Now()
	}

	// 添加到索引
	m.index[idx.ID] = idx
	m.pathIndex[idx.Path] = idx.ID

	// 更新标签索引
	for _, tag := range idx.Tags {
		tagLower := strings.ToLower(tag)
		m.tagIndex[tagLower] = append(m.tagIndex[tagLower], idx.ID)
	}

	// 更新类型索引
	m.typeIndex[idx.ContentType] = append(m.typeIndex[idx.ContentType], idx.ID)

	// 更新统计
	m.updateStats()

	return nil
}

// RemoveDocument 从索引中移除文档
func (m *Manager) RemoveDocument(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	id, ok := m.pathIndex[path]
	if !ok {
		return fmt.Errorf("document not found: %s", path)
	}

	idx := m.index[id]
	m.removeFromIndexes(idx)
	delete(m.index, id)
	delete(m.pathIndex, path)

	m.updateStats()
	return nil
}

// removeFromIndexes 从辅助索引中移除
func (m *Manager) removeFromIndexes(idx *SearchIndex) {
	// 从标签索引移除
	for _, tag := range idx.Tags {
		tagLower := strings.ToLower(tag)
		ids := m.tagIndex[tagLower]
		for i, id := range ids {
			if id == idx.ID {
				m.tagIndex[tagLower] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
	}

	// 从类型索引移除
	ids := m.typeIndex[idx.ContentType]
	for i, id := range ids {
		if id == idx.ID {
			m.typeIndex[idx.ContentType] = append(ids[:i], ids[i+1:]...)
			break
		}
	}
}

// UpdateDocument 更新文档
func (m *Manager) UpdateDocument(req *UpdateIndexRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx, ok := m.index[req.ID]
	if !ok {
		return fmt.Errorf("document not found: %s", req.ID)
	}

	// 更新字段
	if req.Name != "" {
		idx.Name = req.Name
	}
	if req.Tags != nil {
		m.removeFromIndexes(idx)
		idx.Tags = req.Tags
		// 重新添加到标签索引
		for _, tag := range idx.Tags {
			tagLower := strings.ToLower(tag)
			m.tagIndex[tagLower] = append(m.tagIndex[tagLower], idx.ID)
		}
	}
	if req.Content != "" {
		idx.Content = req.Content
	}
	if req.Metadata != nil {
		for k, v := range req.Metadata {
			if idx.Metadata == nil {
				idx.Metadata = make(map[string]string)
			}
			idx.Metadata[k] = v
		}
	}

	idx.ModifiedAt = time.Now()
	m.updateStats()
	return nil
}

// BuildIndex 构建索引
func (m *Manager) BuildIndex(path string) (*IndexTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task := &IndexTask{
		ID:        uuid.New().String(),
		Type:      TaskTypeFull,
		Status:    TaskStatusPending,
		Path:      path,
		CreatedAt: time.Now(),
	}

	m.tasks[task.ID] = task
	m.stats.Status = IndexStatusBuilding

	// 异步执行索引构建
	go m.executeIndexTask(task)

	return task, nil
}

// IncrementalUpdate 增量更新索引
func (m *Manager) IncrementalUpdate(path string) (*IndexTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task := &IndexTask{
		ID:        uuid.New().String(),
		Type:      TaskTypeIncremental,
		Status:    TaskStatusPending,
		Path:      path,
		CreatedAt: time.Now(),
	}

	m.tasks[task.ID] = task
	m.stats.Status = IndexStatusBuilding

	go m.executeIndexTask(task)

	return task, nil
}

// executeIndexTask 执行索引任务
func (m *Manager) executeIndexTask(task *IndexTask) {
	m.mu.Lock()
	now := time.Now()
	task.StartedAt = &now
	task.Status = TaskStatusRunning
	m.mu.Unlock()

	// 模拟索引过程
	// 实际实现中，这里会遍历文件系统、提取内容、构建索引
	time.Sleep(100 * time.Millisecond)

	m.mu.Lock()
	completedAt := time.Now()
	task.CompletedAt = &completedAt
	task.Status = TaskStatusCompleted
	m.stats.Status = IndexStatusIdle
	m.stats.LastIndexedAt = &completedAt
	m.mu.Unlock()
}

// PauseIndex 暂停索引
func (m *Manager) PauseIndex() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stats.Status != IndexStatusBuilding {
		return fmt.Errorf("index is not building, current status: %s", m.stats.Status)
	}

	m.stats.Status = IndexStatusPaused
	return nil
}

// ResumeIndex 恢复索引
func (m *Manager) ResumeIndex() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stats.Status != IndexStatusPaused {
		return fmt.Errorf("index is not paused, current status: %s", m.stats.Status)
	}

	m.stats.Status = IndexStatusBuilding
	return nil
}

// RebuildIndex 重建索引
func (m *Manager) RebuildIndex() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 清空所有索引
	m.index = make(map[string]*SearchIndex)
	m.pathIndex = make(map[string]string)
	m.tagIndex = make(map[string][]string)
	m.typeIndex = make(map[ContentType][]string)

	m.stats = DefaultIndexStats()
	m.stats.Status = IndexStatusBuilding

	// 异步重建
	go func() {
		time.Sleep(100 * time.Millisecond)
		m.mu.Lock()
		m.stats.Status = IndexStatusIdle
		now := time.Now()
		m.stats.LastIndexedAt = &now
		m.mu.Unlock()
	}()

	return nil
}

// GetIndexStats 获取索引统计
func (m *Manager) GetIndexStats() *IndexStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := *m.stats
	stats.ContentTypes = make(map[ContentType]int)
	for k, v := range m.stats.ContentTypes {
		stats.ContentTypes[k] = v
	}
	return &stats
}

// GetTask 获取索引任务
func (m *Manager) GetTask(id string) (*IndexTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	return task, nil
}

// ListTasks 列出索引任务
func (m *Manager) ListTasks() []*IndexTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*IndexTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	return tasks
}

// GetSearchHistory 获取搜索历史
func (m *Manager) GetSearchHistory(limit int) []*SearchHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	// 返回最近的
	start := len(m.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*SearchHistory, limit)
	copy(result, m.history[start:])

	// 反转，最新的在前
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// ClearSearchHistory 清空搜索历史
func (m *Manager) ClearSearchHistory() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history = make([]*SearchHistory, 0)
}

// GetHotSearches 获取热门搜索
func (m *Manager) GetHotSearches(limit int) []*HotSearch {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hot := make([]*HotSearch, 0, len(m.hotSearches))
	for q, c := range m.hotSearches {
		hot = append(hot, &HotSearch{Query: q, Count: c})
	}

	sort.Slice(hot, func(i, j int) bool {
		return hot[i].Count > hot[j].Count
	})

	if limit > 0 && limit < len(hot) {
		hot = hot[:limit]
	}

	return hot
}

// GetSuggestions 获取搜索建议
func (m *Manager) GetSuggestions(query string, limit int) []string {
	if limit <= 0 {
		limit = 10
	}
	return m.generateSuggestions(query)[:min(len(m.generateSuggestions(query)), limit)]
}

// generateSuggestions 生成搜索建议
func (m *Manager) generateSuggestions(query string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	suggestions := make([]string, 0)
	queryLower := strings.ToLower(query)

	// 从搜索历史中匹配
	for _, h := range m.history {
		if strings.Contains(strings.ToLower(h.Query), queryLower) && h.Query != query {
			suggestions = append(suggestions, h.Query)
			if len(suggestions) >= 5 {
				break
			}
		}
	}

	// 从文件名中匹配
	count := 0
	for _, idx := range m.index {
		if strings.Contains(strings.ToLower(idx.Name), queryLower) {
			suggestions = append(suggestions, idx.Name)
			count++
			if count >= 5 {
				break
			}
		}
	}

	// 从标签中匹配
	for tag := range m.tagIndex {
		if strings.Contains(tag, queryLower) {
			suggestions = append(suggestions, tag)
			if len(suggestions) >= 10 {
				break
			}
		}
	}

	// 去重
	seen := make(map[string]bool)
	unique := make([]string, 0)
	for _, s := range suggestions {
		if !seen[s] {
			seen[s] = true
			unique = append(unique, s)
		}
	}

	return unique
}

// addSearchHistory 添加搜索历史
func (m *Manager) addSearchHistory(query string, resultCount int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	history := &SearchHistory{
		ID:          uuid.New().String(),
		Query:       query,
		ResultCount: resultCount,
		SearchedAt:  time.Now(),
	}

	m.history = append(m.history, history)

	// 限制历史大小
	if len(m.history) > m.config.MaxHistory {
		m.history = m.history[len(m.history)-m.config.MaxHistory:]
	}

	// 更新热门搜索
	m.hotSearches[query]++
	if len(m.hotSearches) > m.config.MaxHotSearches {
		// 移除最少的
		minQuery := ""
		minCount := math.MaxInt32
		for q, c := range m.hotSearches {
			if c < minCount {
				minCount = c
				minQuery = q
			}
		}
		if minQuery != "" {
			delete(m.hotSearches, minQuery)
		}
	}
}

// updateLastSearchResultCount 更新最后一次搜索的结果数
func (m *Manager) updateLastSearchResultCount(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.history) > 0 {
		m.history[len(m.history)-1].ResultCount = count
	}
}

// updateStats 更新统计信息
func (m *Manager) updateStats() {
	m.stats.TotalDocuments = len(m.index)

	// 统计各类型数量
	m.stats.ContentTypes = make(map[ContentType]int)
	for _, idx := range m.index {
		m.stats.ContentTypes[idx.ContentType]++
	}
}

// GetDocument 获取文档
func (m *Manager) GetDocument(id string) (*SearchIndex, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	idx, ok := m.index[id]
	if !ok {
		return nil, fmt.Errorf("document not found: %s", id)
	}
	return idx, nil
}

// ListDocuments 列出文档
func (m *Manager) ListDocuments(contentType ContentType, limit int) []*SearchIndex {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var docs []*SearchIndex

	if contentType != "" {
		ids := m.typeIndex[contentType]
		for _, id := range ids {
			if idx, ok := m.index[id]; ok {
				docs = append(docs, idx)
			}
		}
	} else {
		for _, idx := range m.index {
			docs = append(docs, idx)
		}
	}

	sort.Slice(docs, func(i, j int) bool {
		return docs[i].IndexedAt.After(docs[j].IndexedAt)
	})

	if limit > 0 && limit < len(docs) {
		docs = docs[:limit]
	}

	return docs
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *SearchConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if config != nil {
		m.config = config
	}
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *SearchConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// min 返回两个整数中较小的一个
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
