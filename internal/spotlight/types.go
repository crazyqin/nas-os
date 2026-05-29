// Package spotlight 提供全文搜索引擎功能
package spotlight

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// FileType 文件类型
type FileType string

const (
	FileTypeDocument FileType = "document"
	FileTypeImage    FileType = "image"
	FileTypeVideo    FileType = "video"
	FileTypeAudio    FileType = "audio"
	FileTypeArchive  FileType = "archive"
	FileTypeCode     FileType = "code"
	FileTypeOther    FileType = "other"
)

// Document 索引文档
type Document struct {
	ID        string            `json:"id"`
	Path      string            `json:"path"`
	Name      string            `json:"name"`
	Extension string            `json:"extension"`
	Size      int64             `json:"size"`
	MimeType  string            `json:"mime_type"`
	FileType  FileType          `json:"file_type"`
	Content   string            `json:"content,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	IndexedAt time.Time         `json:"indexed_at"`
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query      string     `json:"query" form:"q"`
	Path       string     `json:"path" form:"path"`
	FileType   FileType   `json:"file_type" form:"file_type"`
	Extensions []string   `json:"extensions" form:"extensions"`
	Tags       []string   `json:"tags" form:"tags"`
	MinSize    *int64     `json:"min_size" form:"min_size"`
	MaxSize    *int64     `json:"max_size" form:"max_size"`
	After      *time.Time `json:"after" form:"after"`
	Before     *time.Time `json:"before" form:"before"`
	Page       int        `json:"page" form:"page"`
	PageSize   int        `json:"page_size" form:"page_size"`
	SortBy     string     `json:"sort_by" form:"sort_by"`       // relevance, date, size, name
	SortOrder  string     `json:"sort_order" form:"sort_order"` // asc, desc
}

// SearchResult 搜索结果
type SearchResult struct {
	Documents  []ScoredDocument `json:"documents"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	TotalPages int              `json:"total_pages"`
	Query      string           `json:"query"`
	Duration   string           `json:"duration"`
	Suggestions []string        `json:"suggestions,omitempty"`
}

// ScoredDocument 带评分的文档
type ScoredDocument struct {
	Document
	Score       float64  `json:"score"`
	Highlights  []string `json:"highlights,omitempty"`
	MatchReason string   `json:"match_reason,omitempty"`
}

// SuggestRequest 搜索建议请求
type SuggestRequest struct {
	Query    string `json:"query" form:"q"`
	Limit    int    `json:"limit" form:"limit"`
}

// SuggestResponse 搜索建议响应
type SuggestResponse struct {
	Suggestions []Suggestion `json:"suggestions"`
	Query       string       `json:"query"`
}

// Suggestion 单个搜索建议
type Suggestion struct {
	Text        string  `json:"text"`
	Score       float64 `json:"score"`
	Type        string  `json:"type"` // completion, correction, related
	Description string  `json:"description,omitempty"`
}

// IndexStats 索引统计
type IndexStats struct {
	TotalDocuments  int       `json:"total_documents"`
	TotalTerms      int       `json:"total_terms"`
	IndexSize       int64     `json:"index_size"`
	LastIndexedAt   time.Time `json:"last_indexed_at"`
	IndexDuration   string    `json:"index_duration"`
	DocumentsByType map[FileType]int `json:"documents_by_type"`
}

// invertedIndex 倒排索引
type invertedIndex struct {
	mu       sync.RWMutex
	index    map[string]map[string]positions // term -> docID -> positions
	docs     map[string]*Document            // docID -> Document
	trieRoot *trieNode                        // 前缀树根节点
}

// positions 词项在文档中的位置
type positions struct {
	Fields    []string `json:"fields"`    // 出现的字段 (name, content, tags)
	Count     int      `json:"count"`     // 出现次数
	Positions []int    `json:"positions"` // 在content中的位置
}

// trieNode 前缀树节点
type trieNode struct {
	children map[rune]*trieNode
	isEnd    bool
	terms    []string // 以该节点结尾的所有词项
	count    int      // 词频
}

// Manager 管理器
type Manager struct {
	logger    *zap.Logger
	index     *invertedIndex
	tokenizer *tokenizer
	config    *SpotlightConfig
	stopCh    chan struct{}
}

// Handlers HTTP 处理器
type Handlers struct {
	logger *zap.Logger
	mgr    *Manager
}

// SpotlightConfig 配置
type SpotlightConfig struct {
	MaxIndexSize     int    `json:"max_index_size"`
	MinTermLength    int    `json:"min_term_length"`
	MaxTermLength    int    `json:"max_term_length"`
	EnableCJK       bool   `json:"enable_cjk"`
	EnableStemming   bool   `json:"enable_stemming"`
	IndexBatchSize   int    `json:"index_batch_size"`
	SearchTimeout    int    `json:"search_timeout_ms"`
	MaxResults       int    `json:"max_results"`
	SuggestionLimit  int    `json:"suggestion_limit"`
}

// tokenizer 分词器
type tokenizer struct {
	stopWords map[string]bool
}

// newIndex 创建新倒排索引
func newIndex() *invertedIndex {
	return &invertedIndex{
		index:    make(map[string]map[string]positions),
		docs:     make(map[string]*Document),
		trieRoot: newTrieNode(),
	}
}

// newTrieNode 创建新前缀树节点
func newTrieNode() *trieNode {
	return &trieNode{
		children: make(map[rune]*trieNode),
	}
}

// NewManager 创建新的 Spotlight 管理器
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	config := DefaultConfig()
	return &Manager{
		logger:    logger,
		index:     newIndex(),
		tokenizer: &tokenizer{stopWords: make(map[string]bool)},
		config:    config,
		stopCh:    make(chan struct{}),
	}
}

// Close 关闭管理器
func (m *Manager) Close() {
	close(m.stopCh)
}

// DefaultConfig 默认配置
func DefaultConfig() *SpotlightConfig {
	return &SpotlightConfig{
		MaxIndexSize:    1000000,
		MinTermLength:   2,
		MaxTermLength:   100,
		EnableCJK:       true,
		EnableStemming:  true,
		IndexBatchSize:  1000,
		SearchTimeout:   5000,
		MaxResults:      1000,
		SuggestionLimit: 10,
	}
}

// response 通用响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// IndexDocument 索引单个文档
func (m *Manager) IndexDocument(doc *Document) error {
	m.index.mu.Lock()
	defer m.index.mu.Unlock()

	if doc.ID == "" {
		doc.ID = doc.Path
	}
	doc.IndexedAt = time.Now()
	m.index.docs[doc.ID] = doc

	// 简单的关键词索引
	terms := m.tokenizeQuery(doc.Name + " " + doc.Content)
	for _, term := range terms {
		if m.index.index[term] == nil {
			m.index.index[term] = make(map[string]positions)
		}
		m.index.index[term][doc.ID] = positions{
			Fields: []string{"name", "content"},
			Count:  1,
		}
	}
	return nil
}

// IndexDocuments 批量索引文档
func (m *Manager) IndexDocuments(docs []*Document) (int, error) {
	indexed := 0
	for _, doc := range docs {
		if err := m.IndexDocument(doc); err != nil {
			return indexed, err
		}
		indexed++
	}
	return indexed, nil
}

// RemoveDocument 删除文档
func (m *Manager) RemoveDocument(id string) error {
	m.index.mu.Lock()
	defer m.index.mu.Unlock()

	if _, ok := m.index.docs[id]; !ok {
		return fmt.Errorf("document not found: %s", id)
	}

	// 从倒排索引中移除
	for term, docMap := range m.index.index {
		delete(docMap, id)
		if len(docMap) == 0 {
			delete(m.index.index, term)
		}
	}

	delete(m.index.docs, id)
	return nil
}

// GetDocument 获取文档
func (m *Manager) GetDocument(id string) (*Document, bool) {
	m.index.mu.RLock()
	defer m.index.mu.RUnlock()

	doc, ok := m.index.docs[id]
	if !ok {
		return nil, false
	}
	result := *doc
	return &result, true
}

// GetStats 获取索引统计
func (m *Manager) GetStats() map[string]interface{} {
	m.index.mu.RLock()
	defer m.index.mu.RUnlock()

	return map[string]interface{}{
		"total_documents": len(m.index.docs),
		"total_terms":    len(m.index.index),
		"config":         m.config,
	}
}

// insert 插入词项到前缀树
func (t *trieNode) insert(term string) {
	node := t
	for _, ch := range term {
		if node.children == nil {
			node.children = make(map[rune]*trieNode)
		}
		child, ok := node.children[ch]
		if !ok {
			child = &trieNode{}
			node.children[ch] = child
		}
		node = child
	}
	node.isEnd = true
	node.terms = append(node.terms, term)
	node.count++
}

// search 在前缀树中搜索
func (t *trieNode) search(prefix string, limit int) []Suggestion {
	results := make([]Suggestion, 0)
	if t == nil {
		return results
	}

	node := t
	for _, ch := range prefix {
		child, ok := node.children[ch]
		if !ok {
			return results
		}
		node = child
	}

	// 收集所有以 prefix 开头的词
	var collect func(n *trieNode)
	collect = func(n *trieNode) {
		if len(results) >= limit {
			return
		}
		if n.isEnd {
			for _, term := range n.terms {
				results = append(results, Suggestion{
					Text:  term,
					Score: float64(n.count),
					Type:  "completion",
				})
			}
		}
		for _, child := range n.children {
			collect(child)
		}
	}
	collect(node)
	return results
}

// isCJK 检测是否为 CJK 字符
func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
		(r >= 0x3400 && r <= 0x4DBF) || // CJK Extension A
		(r >= 0x20000 && r <= 0x2A6DF) || // CJK Extension B
		(r >= 0xF900 && r <= 0xFAFF) || // CJK Compatibility Ideographs
		(r >= 0x2F800 && r <= 0x2FA1F) // CJK Compatibility Ideographs Supplement
}

// isPunctuation 检测是否为分隔符（空格/标点）
func isPunctuation(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', ',', '.', ';', '!', '?', ':', '/', '\\', '(', ')', '[', ']', '{', '}', '"', '\'', '-', '_', '@', '#', '$', '%', '^', '&', '*', '+', '=', '<', '>', '|', '~', '`':
		return true
	}
	return false
}

// tokenize 分词
func (t *tokenizer) tokenize(text string) []string {
	if text == "" {
		return nil
	}

	words := make([]string, 0)
	runes := []rune(text)
	length := len(runes)

	i := 0
	for i < length {
		r := runes[i]

		// 跳过分隔符
		if isPunctuation(r) {
			i++
			continue
		}

		// CJK 字符段：生成 bigram 滑动窗口
		if isCJK(r) {
			cjkStart := i
			for i < length && isCJK(runes[i]) {
				i++
			}
			cjkSegment := runes[cjkStart:i]

			// 生成 bigrams
			for j := 0; j < len(cjkSegment)-1; j++ {
				words = append(words, string(cjkSegment[j:j+2]))
			}
			// 单字符也加入（如果只有一个 CJK 字符）
			if len(cjkSegment) == 1 {
				words = append(words, string(cjkSegment))
			}
			continue
		}

		// 非 CJK 字符段（英文等）：按标点分割
		wordStart := i
		for i < length && !isPunctuation(runes[i]) && !isCJK(runes[i]) {
			i++
		}
		word := string(runes[wordStart:i])
		if word != "" {
			words = append(words, word)
		}
	}

	return words
}

// containsTag 检查字符串切片是否包含指定字符串
func containsTag(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// sortSuggestions 排序建议
func sortSuggestions(suggestions []Suggestion) {
	// 按分数降序排序
	for i := 0; i < len(suggestions)-1; i++ {
		for j := i + 1; j < len(suggestions); j++ {
			if suggestions[j].Score > suggestions[i].Score {
				suggestions[i], suggestions[j] = suggestions[j], suggestions[i]
			}
		}
	}
}

// addToIndex 添加到倒排索引（外部调用，加锁版本）
func (m *Manager) addToIndex(term, docID, field string, position int) {
	m.index.mu.Lock()
	defer m.index.mu.Unlock()

	m.addToIndexLocked(term, docID, field, position)
}

// addToIndexLocked 添加到倒排索引（内部调用，不加锁版本，调用方需确保已持有锁）
func (m *Manager) addToIndexLocked(term, docID, field string, position int) {
	if m.index.index[term] == nil {
		m.index.index[term] = make(map[string]positions)
	}
	pos, ok := m.index.index[term][docID]
	if !ok {
		pos = positions{Fields: []string{field}, Count: 1, Positions: []int{position}}
	} else {
		pos.Count++
		pos.Positions = append(pos.Positions, position)
		if !containsString(pos.Fields, field) {
			pos.Fields = append(pos.Fields, field)
		}
	}
	m.index.index[term][docID] = pos
}

// containsString 检查字符串切片是否包含指定字符串
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
