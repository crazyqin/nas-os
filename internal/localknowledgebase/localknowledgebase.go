package localknowledgebase

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// KnowledgeEntry 知识条目.
type KnowledgeEntry struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Category  string    `json:"category"`
	Tags      []string  `json:"tags"`
	Source    string    `json:"source"`
	Vector    []float64 `json:"vector,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Query 查询.
type Query struct {
	Text     string `json:"text"`
	Category string `json:"category,omitempty"`
	Limit    int    `json:"limit"`
}

// QueryResult 查询结果.
type QueryResult struct {
	Entry *KnowledgeEntry `json:"entry"`
	Score float64         `json:"score"`
}

// KnowledgeBaseMetrics 知识库指标.
type KnowledgeBaseMetrics struct {
	TotalEntries int   `json:"total_entries"`
	Categories   int   `json:"categories"`
	TotalQueries int64 `json:"total_queries"`
}

// LocalKnowledgeBase 本地AI知识库.
type LocalKnowledgeBase struct {
	mu         sync.RWMutex
	entries    map[string]*KnowledgeEntry
	categories map[string]int
	metrics    *KnowledgeBaseMetrics
	logger     *slog.Logger
}

// NewLocalKnowledgeBase 创建本地知识库.
func NewLocalKnowledgeBase(logger *slog.Logger) *LocalKnowledgeBase {
	if logger == nil {
		logger = slog.Default()
	}

	return &LocalKnowledgeBase{
		entries:    make(map[string]*KnowledgeEntry),
		categories: make(map[string]int),
		metrics:    &KnowledgeBaseMetrics{},
		logger:     logger,
	}
}

// AddEntry 添加知识条目.
func (kb *LocalKnowledgeBase) AddEntry(entry *KnowledgeEntry) error {
	if entry == nil {
		return errors.New("entry cannot be nil")
	}
	if entry.ID == "" {
		return errors.New("entry ID cannot be empty")
	}
	if entry.Title == "" {
		return errors.New("entry title cannot be empty")
	}
	if entry.Content == "" {
		return errors.New("entry content cannot be empty")
	}

	kb.mu.Lock()
	defer kb.mu.Unlock()

	if _, exists := kb.entries[entry.ID]; exists {
		return fmt.Errorf("entry %s already exists", entry.ID)
	}

	entry.CreatedAt = time.Now()
	entry.UpdatedAt = time.Now()

	// 生成简单向量
	entry.Vector = kb.generateVector(entry.Content)

	kb.entries[entry.ID] = entry
	kb.categories[entry.Category]++

	kb.metrics.TotalEntries = len(kb.entries)
	kb.metrics.Categories = len(kb.categories)

	kb.logger.Info("知识条目已添加", "id", entry.ID, "title", entry.Title, "category", entry.Category)
	return nil
}

// UpdateEntry 更新知识条目.
func (kb *LocalKnowledgeBase) UpdateEntry(entry *KnowledgeEntry) error {
	if entry == nil {
		return errors.New("entry cannot be nil")
	}
	if entry.ID == "" {
		return errors.New("entry ID cannot be empty")
	}

	kb.mu.Lock()
	defer kb.mu.Unlock()

	existing, exists := kb.entries[entry.ID]
	if !exists {
		return fmt.Errorf("entry %s not found", entry.ID)
	}

	// 更新分类计数
	if existing.Category != entry.Category {
		kb.categories[existing.Category]--
		if kb.categories[existing.Category] == 0 {
			delete(kb.categories, existing.Category)
		}
		kb.categories[entry.Category]++
		kb.metrics.Categories = len(kb.categories)
	}

	entry.CreatedAt = existing.CreatedAt
	entry.UpdatedAt = time.Now()
	entry.Vector = kb.generateVector(entry.Content)

	kb.entries[entry.ID] = entry

	kb.logger.Info("知识条目已更新", "id", entry.ID)
	return nil
}

// RemoveEntry 移除知识条目.
func (kb *LocalKnowledgeBase) RemoveEntry(entryID string) error {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	entry, exists := kb.entries[entryID]
	if !exists {
		return fmt.Errorf("entry %s not found", entryID)
	}

	kb.categories[entry.Category]--
	if kb.categories[entry.Category] == 0 {
		delete(kb.categories, entry.Category)
	}

	delete(kb.entries, entryID)
	kb.metrics.TotalEntries = len(kb.entries)
	kb.metrics.Categories = len(kb.categories)

	kb.logger.Info("知识条目已移除", "id", entryID)
	return nil
}

// Query 查询知识库.
func (kb *LocalKnowledgeBase) Query(query *Query) ([]*QueryResult, error) {
	if query == nil {
		return nil, errors.New("query cannot be nil")
	}
	if query.Text == "" {
		return nil, errors.New("query text cannot be empty")
	}

	kb.mu.RLock()
	defer kb.mu.RUnlock()

	kb.metrics.TotalQueries++

	queryVector := kb.generateVector(query.Text)

	var results []*QueryResult
	for _, entry := range kb.entries {
		// 按分类过滤
		if query.Category != "" && entry.Category != query.Category {
			continue
		}

		score := kb.similarity(queryVector, entry.Vector)
		if score > 0.1 { // 阈值
			results = append(results, &QueryResult{
				Entry: entry,
				Score: score,
			})
		}
	}

	// 按分数排序
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// 限制结果
	if query.Limit > 0 && len(results) > query.Limit {
		results = results[:query.Limit]
	}

	kb.logger.Debug("查询完成", "query", query.Text, "results", len(results))
	return results, nil
}

// GetEntry 获取知识条目.
func (kb *LocalKnowledgeBase) GetEntry(entryID string) (*KnowledgeEntry, error) {
	kb.mu.RLock()
	defer kb.mu.RUnlock()

	entry, exists := kb.entries[entryID]
	if !exists {
		return nil, fmt.Errorf("entry %s not found", entryID)
	}

	return entry, nil
}

// ListCategories 列出分类.
func (kb *LocalKnowledgeBase) ListCategories() map[string]int {
	kb.mu.RLock()
	defer kb.mu.RUnlock()

	categories := make(map[string]int)
	for k, v := range kb.categories {
		categories[k] = v
	}

	return categories
}

// GetMetrics 获取指标.
func (kb *LocalKnowledgeBase) GetMetrics() *KnowledgeBaseMetrics {
	kb.mu.RLock()
	defer kb.mu.RUnlock()

	return &KnowledgeBaseMetrics{
		TotalEntries: kb.metrics.TotalEntries,
		Categories:   kb.metrics.Categories,
		TotalQueries: kb.metrics.TotalQueries,
	}
}

// generateVector 生成向量.
func (kb *LocalKnowledgeBase) generateVector(text string) []float64 {
	vector := make([]float64, 64)
	for i, ch := range text {
		vector[i%64] += float64(ch)
	}

	// 归一化
	norm := 0.0
	for _, v := range vector {
		norm += v * v
	}
	if norm > 0 {
		for i := range vector {
			vector[i] /= norm
		}
	}

	return vector
}

// similarity 相似度计算.
func (kb *LocalKnowledgeBase) similarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	dot := 0.0
	normA := 0.0
	normB := 0.0

	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (normA * normB)
}
