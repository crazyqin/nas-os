// Package unifiedsearch 核心管理器，协调搜索引擎和索引任务
package unifiedsearch

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager 统一搜索管理器
type Manager struct {
	mu          sync.RWMutex
	engine      *SearchEngine
	logger      *zap.Logger
	config      *SearchConfig
	index       map[string]*SearchIndex  // id -> index entry（内存缓存）
	pathIndex   map[string]string        // path -> id
	tagIndex    map[string][]string      // tag -> []id
	typeIndex   map[ContentType][]string // type -> []id
	tasks       map[string]*IndexTask
	history     []*SearchHistory
	hotSearches map[string]int // query -> count
	stopChan    chan struct{}
	running     bool
}

// SearchConfig 搜索配置
type SearchConfig struct {
	IndexDir        string  `json:"index_dir"`
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
		IndexDir:        "/var/lib/nas-os/search-index",
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
func NewManager(config *SearchConfig, logger *zap.Logger) (*Manager, error) {
	if config == nil {
		config = DefaultSearchConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	engine, err := NewSearchEngine(logger, config.IndexDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create search engine: %w", err)
	}

	return &Manager{
		engine:      engine,
		logger:      logger,
		config:      config,
		index:       make(map[string]*SearchIndex),
		pathIndex:   make(map[string]string),
		tagIndex:    make(map[string][]string),
		typeIndex:   make(map[ContentType][]string),
		tasks:       make(map[string]*IndexTask),
		history:     make([]*SearchHistory, 0),
		hotSearches: make(map[string]int),
		stopChan:    make(chan struct{}),
	}, nil
}

// Start 启动管理器
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("manager is already running")
	}

	if err := m.engine.Start(); err != nil {
		return fmt.Errorf("failed to start search engine: %w", err)
	}

	m.running = true
	m.logger.Info("unified search manager started")
	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	close(m.stopChan)
	m.running = false

	if err := m.engine.Stop(); err != nil {
		m.logger.Error("failed to stop search engine", zap.Error(err))
	}

	m.logger.Info("unified search manager stopped")
	return nil
}

// Search 执行搜索
func (m *Manager) Search(query *SearchQuery) (*SearchResponse, error) {
	if query == nil || query.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	// 使用 bleve 搜索引擎
	resp, err := m.engine.Search(query)
	if err != nil {
		return nil, err
	}

	// 生成搜索建议
	suggestions, _ := m.engine.GetSuggestions(query.Query, 5)
	resp.Suggestions = suggestions

	// 记录搜索历史和热门搜索（在同一把锁内完成）
	m.recordSearch(query.Query, resp.Total)

	return resp, nil
}

// recordSearch 记录搜索历史和热门搜索
func (m *Manager) recordSearch(query string, resultCount int) {
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
		minQuery := ""
		minCount := 999999999
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

	// 添加到内存缓存索引
	m.index[idx.ID] = idx
	m.pathIndex[idx.Path] = idx.ID

	// 更新标签索引
	for _, tag := range idx.Tags {
		tagLower := strings.ToLower(tag)
		m.tagIndex[tagLower] = append(m.tagIndex[tagLower], idx.ID)
	}

	// 更新类型索引
	m.typeIndex[idx.ContentType] = append(m.typeIndex[idx.ContentType], idx.ID)

	// 索引到 bleve
	if err := m.engine.IndexDocument(idx); err != nil {
		m.logger.Warn("failed to index to bleve", zap.Error(err))
	}

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

	// 从 bleve 移除
	if err := m.engine.RemoveDocument(id); err != nil {
		m.logger.Warn("failed to remove from bleve", zap.Error(err))
	}

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

	// 重新索引
	if err := m.engine.IndexDocument(idx); err != nil {
		m.logger.Warn("failed to re-index document", zap.Error(err))
	}

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

	// 实际实现中，这里会遍历文件系统、提取内容、构建索引
	// 目前使用模拟实现
	m.logger.Info("index task started",
		zap.String("id", task.ID),
		zap.String("type", string(task.Type)),
		zap.String("path", task.Path))

	time.Sleep(100 * time.Millisecond)

	m.mu.Lock()
	completedAt := time.Now()
	task.CompletedAt = &completedAt
	task.Status = TaskStatusCompleted
	m.mu.Unlock()

	m.logger.Info("index task completed", zap.String("id", task.ID))
}

// PauseIndex 暂停索引
func (m *Manager) PauseIndex() error {
	// 简化实现
	return nil
}

// ResumeIndex 恢复索引
func (m *Manager) ResumeIndex() error {
	// 简化实现
	return nil
}

// RebuildIndex 重建索引
func (m *Manager) RebuildIndex() error {
	return m.engine.RebuildIndex()
}

// GetIndexStats 获取索引统计
func (m *Manager) GetIndexStats() *IndexStats {
	return m.engine.GetStats()
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
	suggestions, err := m.engine.GetSuggestions(query, limit)
	if err != nil {
		m.logger.Warn("failed to get suggestions", zap.Error(err))
		return []string{}
	}
	return suggestions
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
	m.logger.Debug("search history added", zap.String("query", query), zap.Int("count", len(m.history)))

	// 限制历史大小
	if len(m.history) > m.config.MaxHistory {
		m.history = m.history[len(m.history)-m.config.MaxHistory:]
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

// fuzzyMatch 模糊匹配（编辑距离）
func (m *Manager) fuzzyMatch(term, name, content string) float64 {
	bestScore := 0.0

	// 对文件名进行模糊匹配
	nameWords := tokenize(name)
	for _, w := range nameWords {
		sim := similarity(term, w)
		if sim > bestScore {
			bestScore = sim
		}
	}

	// 对内容进行模糊匹配（只检查前1000字符）
	contentLower := content
	if len(contentLower) > 1000 {
		contentLower = contentLower[:1000]
	}
	contentWords := tokenize(contentLower)
	for _, w := range contentWords {
		sim := similarity(term, w)
		if sim > bestScore {
			bestScore = sim
		}
	}

	return bestScore
}

// similarity 计算两个字符串的相似度（基于编辑距离）
func similarity(a, b string) float64 {
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

	return float64(common) / float64(max(lenA, lenB))
}

// tokenize 分词
func tokenize(text string) []string {
	text = strings.ToLower(text)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_'
	})
	return words
}

// max 返回两个整数中较大的一个
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min 返回两个整数中较小的一个
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ContainsRegex 检查字符串是否匹配正则表达式
func ContainsRegex(text, pattern string) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(text)
}
