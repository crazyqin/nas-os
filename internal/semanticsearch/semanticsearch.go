package semanticsearch

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// Document 文档
type Document struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Path      string    `json:"path"`
	Type      string    `json:"type"`
	Tags      []string  `json:"tags"`
	Vector    []float64 `json:"vector,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SearchQuery 搜索查询
type SearchQuery struct {
	Text      string   `json:"text"`
	Limit     int      `json:"limit"`
	Threshold float64  `json:"threshold"`
	Filters   []Filter `json:"filters,omitempty"`
}

// Filter 过滤器
type Filter struct {
	Field string      `json:"field"`
	Value interface{} `json:"value"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Document *Document `json:"document"`
	Score    float64   `json:"score"`
	Matches  []Match   `json:"matches,omitempty"`
}

// Match 匹配项
type Match struct {
	Field   string `json:"field"`
	Context string `json:"context"`
}

// SearchMetrics 搜索指标
type SearchMetrics struct {
	TotalDocuments  int     `json:"total_documents"`
	TotalSearches   int64   `json:"total_searches"`
	AverageLatency  float64 `json:"average_latency_ms"`
	IndexSize       int     `json:"index_size"`
}

// SemanticEngine 语义搜索引擎
type SemanticEngine struct {
	mu        sync.RWMutex
	documents map[string]*Document
	index     map[string][]string // 简单倒排索引
	metrics   *SearchMetrics
	logger    *slog.Logger
}

// NewSemanticEngine 创建语义搜索引擎
func NewSemanticEngine(logger *slog.Logger) *SemanticEngine {
	if logger == nil {
		logger = slog.Default()
	}

	return &SemanticEngine{
		documents: make(map[string]*Document),
		index:     make(map[string][]string),
		metrics:   &SearchMetrics{},
		logger:    logger,
	}
}

// IndexDocument 索引文档
func (e *SemanticEngine) IndexDocument(doc *Document) error {
	if doc == nil {
		return errors.New("document cannot be nil")
	}
	if doc.ID == "" {
		return errors.New("document ID cannot be empty")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	doc.UpdatedAt = time.Now()
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = doc.UpdatedAt
	}

	// 生成简单向量（基于词频）
	doc.Vector = e.generateVector(doc.Content)

	// 更新倒排索引
	e.updateIndex(doc)

	e.documents[doc.ID] = doc
	e.metrics.TotalDocuments = len(e.documents)

	e.logger.Debug("文档已索引", "id", doc.ID, "title", doc.Title)
	return nil
}

// RemoveDocument 移除文档
func (e *SemanticEngine) RemoveDocument(docID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.documents[docID]; !exists {
		return fmt.Errorf("document %s not found", docID)
	}

	// 从索引中移除
	e.removeFromIndex(docID)

	delete(e.documents, docID)
	e.metrics.TotalDocuments = len(e.documents)

	e.logger.Debug("文档已移除", "id", docID)
	return nil
}

// Search 语义搜索
func (e *SemanticEngine) Search(query *SearchQuery) ([]*SearchResult, error) {
	if query == nil {
		return nil, errors.New("query cannot be nil")
	}
	if query.Text == "" {
		return nil, errors.New("query text cannot be empty")
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	start := time.Now()

	// 生成查询向量
	queryVector := e.generateVector(query.Text)

	// 计算相似度
	var results []*SearchResult
	for _, doc := range e.documents {
		// 应用过滤器
		if !e.matchesFilters(doc, query.Filters) {
			continue
		}

		score := e.cosineSimilarity(queryVector, doc.Vector)
		if query.Threshold > 0 && score < query.Threshold {
			continue
		}

		result := &SearchResult{
			Document: doc,
			Score:    score,
			Matches:  e.findMatches(doc, query.Text),
		}
		results = append(results, result)
	}

	// 按分数排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// 限制结果数量
	if query.Limit > 0 && len(results) > query.Limit {
		results = results[:query.Limit]
	}

	// 更新指标
	latency := float64(time.Since(start).Microseconds()) / 1000.0
	e.metrics.TotalSearches++
	e.metrics.AverageLatency = (e.metrics.AverageLatency*float64(e.metrics.TotalSearches-1) + latency) / float64(e.metrics.TotalSearches)

	e.logger.Debug("搜索完成", "query", query.Text, "results", len(results), "latency_ms", latency)
	return results, nil
}

// GetDocument 获取文档
func (e *SemanticEngine) GetDocument(docID string) (*Document, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	doc, exists := e.documents[docID]
	if !exists {
		return nil, fmt.Errorf("document %s not found", docID)
	}

	return doc, nil
}

// ListDocuments 列出文档
func (e *SemanticEngine) ListDocuments(limit, offset int) []*Document {
	e.mu.RLock()
	defer e.mu.RUnlock()

	docs := make([]*Document, 0, len(e.documents))
	for _, doc := range e.documents {
		docs = append(docs, doc)
	}

	sort.Slice(docs, func(i, j int) bool {
		return docs[i].UpdatedAt.After(docs[j].UpdatedAt)
	})

	if offset >= len(docs) {
		return nil
	}

	docs = docs[offset:]
	if limit > 0 && len(docs) > limit {
		docs = docs[:limit]
	}

	return docs
}

// GetMetrics 获取指标
func (e *SemanticEngine) GetMetrics() *SearchMetrics {
	e.mu.RLock()
	defer e.mu.RUnlock()

	metrics := *e.metrics
	metrics.IndexSize = len(e.index)
	return &metrics
}

// generateVector 生成简单向量
func (e *SemanticEngine) generateVector(text string) []float64 {
	// 简单实现：基于字符频率生成向量
	words := strings.Fields(strings.ToLower(text))
	vector := make([]float64, 128)

	for _, word := range words {
		hash := e.simpleHash(word)
		idx := hash % 128
		vector[idx] += 1.0
	}

	// 归一化
 norm := 0.0
	for _, v := range vector {
		norm += v * v
	}
	if norm > 0 {
		norm = math.Sqrt(norm)
		for i := range vector {
			vector[i] /= norm
		}
	}

	return vector
}

// simpleHash 简单哈希
func (e *SemanticEngine) simpleHash(s string) uint32 {
	var hash uint32
	for _, c := range s {
		hash = hash*31 + uint32(c)
	}
	return hash
}

// cosineSimilarity 余弦相似度
func (e *SemanticEngine) cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	dotProduct := 0.0
	normA := 0.0
	normB := 0.0

	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// updateIndex 更新倒排索引
func (e *SemanticEngine) updateIndex(doc *Document) {
	words := strings.Fields(strings.ToLower(doc.Content + " " + doc.Title))
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:")
		if len(word) > 2 {
			e.index[word] = append(e.index[word], doc.ID)
		}
	}
}

// removeFromIndex 从索引移除
func (e *SemanticEngine) removeFromIndex(docID string) {
	for word, ids := range e.index {
		for i, id := range ids {
			if id == docID {
				e.index[word] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
	}
}

// matchesFilters 匹配过滤器
func (e *SemanticEngine) matchesFilters(doc *Document, filters []Filter) bool {
	for _, filter := range filters {
		switch filter.Field {
		case "type":
			if doc.Type != filter.Value.(string) {
				return false
			}
		case "tag":
			tag := filter.Value.(string)
			found := false
			for _, t := range doc.Tags {
				if t == tag {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}

// findMatches 查找匹配
func (e *SemanticEngine) findMatches(doc *Document, query string) []Match {
	var matches []Match
	queryWords := strings.Fields(strings.ToLower(query))

	content := strings.ToLower(doc.Content)
	title := strings.ToLower(doc.Title)

	for _, word := range queryWords {
		if strings.Contains(title, word) {
			matches = append(matches, Match{Field: "title", Context: doc.Title})
		}
		if strings.Contains(content, word) {
			// 提取上下文
			idx := strings.Index(content, word)
			start := idx - 50
			if start < 0 {
				start = 0
			}
			end := idx + len(word) + 50
			if end > len(content) {
				end = len(content)
			}
			matches = append(matches, Match{Field: "content", Context: doc.Content[start:end]})
		}
	}

	return matches
}
