package spotlight

import (
	"strings"
	"time"
	"unicode"

	"go.uber.org/zap"
)

// NewManager 创建管理器
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		logger:    logger,
		index:     newIndex(),
		tokenizer: newTokenizer(),
		config:    DefaultConfig(),
		stopCh:    make(chan struct{}),
	}
}

// NewManagerWithConfig 使用自定义配置创建管理器
func NewManagerWithConfig(logger *zap.Logger, config *SpotlightConfig) *Manager {
	m := NewManager(logger)
	if config != nil {
		m.config = config
	}
	return m
}

// newTokenizer 创建分词器
func newTokenizer() *tokenizer {
	return &tokenizer{
		stopWords: defaultStopWords(),
	}
}

// defaultStopWords 默认停用词
func defaultStopWords() map[string]bool {
	words := []string{
		// 英文停用词
		"a", "an", "the", "is", "are", "was", "were", "be", "been", "being",
		"have", "has", "had", "do", "does", "did", "will", "would", "could",
		"should", "may", "might", "shall", "can", "need", "dare", "ought",
		"used", "to", "of", "in", "for", "on", "with", "at", "by", "from",
		"up", "about", "into", "through", "during", "before", "after",
		"above", "below", "between", "out", "off", "over", "under", "again",
		"further", "then", "once", "here", "there", "when", "where", "why",
		"how", "all", "each", "every", "both", "few", "more", "most", "other",
		"some", "such", "no", "nor", "not", "only", "own", "same", "so",
		"than", "too", "very", "s", "t", "just", "don", "now",
		// 中文停用词
		"的", "了", "在", "是", "我", "有", "和", "就", "不", "人", "都",
		"一", "一个", "上", "也", "很", "到", "说", "要", "去", "你",
		"会", "着", "没有", "看", "好", "自己", "这",
	}
	stop := make(map[string]bool, len(words))
	for _, w := range words {
		stop[w] = true
	}
	return stop
}

// IndexDocument 索引单个文档
func (m *Manager) IndexDocument(doc *Document) error {
	m.index.mu.Lock()
	defer m.index.mu.Unlock()

	if doc.ID == "" {
		doc.ID = generateDocID(doc.Path)
	}
	doc.IndexedAt = time.Now()

	// 如果文档已存在，先删除旧索引
	if _, exists := m.index.docs[doc.ID]; exists {
		m.removeFromIndexLocked(doc.ID)
	}

	// 存储文档
	m.index.docs[doc.ID] = doc

	// 索引文件名
	nameTokens := m.tokenizer.tokenize(doc.Name)
	for _, token := range nameTokens {
		m.addToIndex(token, doc.ID, "name", 1)
	}

	// 索引内容
	var contentTokens []string
	if doc.Content != "" {
		contentTokens = m.tokenizer.tokenize(doc.Content)
		for i, token := range contentTokens {
			m.addToIndex(token, doc.ID, "content", i)
		}
	}

	// 索引标签
	for _, tag := range doc.Tags {
		tagTokens := m.tokenizer.tokenize(tag)
		for _, token := range tagTokens {
			m.addToIndex(token, doc.ID, "tags", 0)
		}
	}

	// 索引扩展名
	if doc.Extension != "" {
		ext := strings.ToLower(strings.TrimPrefix(doc.Extension, "."))
		m.addToIndex(ext, doc.ID, "extension", 0)
	}

	m.logger.Debug("document indexed",
		zap.String("id", doc.ID),
		zap.String("path", doc.Path),
		zap.Int("name_tokens", len(nameTokens)),
		zap.Int("content_tokens", len(contentTokens)))

	return nil
}

// IndexDocuments 批量索引文档
func (m *Manager) IndexDocuments(docs []*Document) (int, error) {
	indexed := 0
	for _, doc := range docs {
		if err := m.IndexDocument(doc); err != nil {
			m.logger.Error("failed to index document",
				zap.String("path", doc.Path),
				zap.Error(err))
			continue
		}
		indexed++
	}
	return indexed, nil
}

// RemoveDocument 删除文档索引
func (m *Manager) RemoveDocument(docID string) error {
	m.index.mu.Lock()
	defer m.index.mu.Unlock()

	if _, exists := m.index.docs[docID]; !exists {
		return nil
	}

	m.removeFromIndexLocked(docID)
	delete(m.index.docs, docID)

	m.logger.Debug("document removed from index", zap.String("id", docID))
	return nil
}

// GetDocument 获取文档
func (m *Manager) GetDocument(docID string) (*Document, bool) {
	m.index.mu.RLock()
	defer m.index.mu.RUnlock()

	doc, exists := m.index.docs[docID]
	return doc, exists
}

// GetStats 获取索引统计
func (m *Manager) GetStats() *IndexStats {
	m.index.mu.RLock()
	defer m.index.mu.RUnlock()

	stats := &IndexStats{
		TotalDocuments:  len(m.index.docs),
		TotalTerms:      len(m.index.index),
		DocumentsByType: make(map[FileType]int),
	}

	var totalSize int64
	var lastIndexed time.Time

	for _, doc := range m.index.docs {
		totalSize += doc.Size
		stats.DocumentsByType[doc.FileType]++
		if doc.IndexedAt.After(lastIndexed) {
			lastIndexed = doc.IndexedAt
		}
	}

	stats.IndexSize = totalSize
	stats.LastIndexedAt = lastIndexed

	return stats
}

// addToIndex 添加到倒排索引
func (m *Manager) addToIndex(term, docID, field string, position int) {
	if term == "" {
		return
	}

	term = strings.ToLower(term)

	if _, exists := m.index.index[term]; !exists {
		m.index.index[term] = make(map[string]positions)
	}

	pos := m.index.index[term][docID]
	pos.Fields = appendUnique(pos.Fields, field)
	pos.Count++
	pos.Positions = append(pos.Positions, position)
	m.index.index[term][docID] = pos

	// 添加到前缀树
	m.index.trieRoot.insert(term)
}

// removeFromIndexLocked 从索引中删除文档（需要持有锁）
func (m *Manager) removeFromIndexLocked(docID string) {
	for term, postings := range m.index.index {
		if _, exists := postings[docID]; exists {
			delete(postings, docID)
			if len(postings) == 0 {
				delete(m.index.index, term)
			}
		}
	}
}

// splitByLanguage 按语言分割文本
func splitByLanguage(text string) []string {
	var segments []string
	var current strings.Builder
	var currentIsCJK bool

	for i, r := range []rune(text) {
		isCJK := isCJKRune(r)
		if i == 0 {
			currentIsCJK = isCJK
		}

		if isCJK != currentIsCJK && current.Len() > 0 {
			segments = append(segments, current.String())
			current.Reset()
			currentIsCJK = isCJK
		}

		if isCJK || !unicode.IsSpace(r) {
			current.WriteRune(r)
		} else if current.Len() > 0 {
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		segments = append(segments, current.String())
	}

	return segments
}

// isCJK 检查字符串是否包含CJK字符
func isCJK(s string) bool {
	for _, r := range []rune(s) {
		if isCJKRune(r) {
			return true
		}
	}
	return false
}

// isCJKRune 检查字符是否是CJK字符
func isCJKRune(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
		(r >= 0x3400 && r <= 0x4DBF) || // CJK Unified Ideographs Extension A
		(r >= 0x20000 && r <= 0x2A6DF) || // CJK Unified Ideographs Extension B
		(r >= 0x2A700 && r <= 0x2CEAF) || // CJK Unified Ideographs Extensions C-F
		(r >= 0xF900 && r <= 0xFAFF) || // CJK Compatibility Ideographs
		(r >= 0x2F800 && r <= 0x2FA1F) // CJK Compatibility Ideographs Supplement
}

// tokenizeCJK 中文分词
func tokenizeCJK(text string) []string {
	runes := []rune(text)
	var tokens []string

	// 单字符切分
	for _, r := range runes {
		if !unicode.IsSpace(r) {
			tokens = append(tokens, string(r))
		}
	}

	// bigram 切分
	for i := 0; i < len(runes)-1; i++ {
		if !unicode.IsSpace(runes[i]) && !unicode.IsSpace(runes[i+1]) {
			tokens = append(tokens, string(runes[i:i+2]))
		}
	}

	return tokens
}

// tokenizeEnglish 英文分词
func tokenizeEnglish(text string) []string {
	// 按空格和标点分割
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	})

	var tokens []string
	for _, field := range fields {
		// 转小写
		token := strings.ToLower(field)
		if len(token) > 0 {
			tokens = append(tokens, token)
			// 添加词干（简单实现：去掉常见后缀）
			if stem := simpleStem(token); stem != token {
				tokens = append(tokens, stem)
			}
		}
	}

	return tokens
}

// simpleStem 简单词干提取
func simpleStem(word string) string {
	suffixes := []string{"ing", "tion", "sion", "ment", "ness", "able", "ible", "ful", "less", "ous", "ive", "ly", "ed", "er", "es", "s"}

	for _, suffix := range suffixes {
		if strings.HasSuffix(word, suffix) && len(word)-len(suffix) >= 3 {
			return strings.TrimSuffix(word, suffix)
		}
	}
	return word
}

// insert 向前缀树插入词项
func (n *trieNode) insert(term string) {
	node := n
	for _, r := range term {
		if _, exists := node.children[r]; !exists {
			node.children[r] = newTrieNode()
		}
		node = node.children[r]
	}
	node.isEnd = true
	node.count++
	if !contains(node.terms, term) {
		node.terms = append(node.terms, term)
	}
}

// search 前缀搜索
func (n *trieNode) search(prefix string, limit int) []Suggestion {
	node := n
	for _, r := range prefix {
		if _, exists := node.children[r]; !exists {
			return nil
		}
		node = node.children[r]
	}

	var suggestions []Suggestion
	node.collect(prefix, &suggestions, limit)

	// 按分数排序
	sortSuggestions(suggestions)
	if len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}

	return suggestions
}

// collect 收集前缀树中的词项
func (n *trieNode) collect(prefix string, suggestions *[]Suggestion, limit int) {
	if len(*suggestions) >= limit {
		return
	}

	if n.isEnd {
		for _, term := range n.terms {
			*suggestions = append(*suggestions, Suggestion{
				Text:  term,
				Score: float64(n.count),
				Type:  "completion",
			})
		}
	}

	for r, child := range n.children {
		child.collect(prefix+string(r), suggestions, limit)
	}
}

// appendUnique 追加不重复字符串
func appendUnique(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}

// contains 检查切片是否包含字符串
func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// sortSuggestions 排序搜索建议
func sortSuggestions(suggestions []Suggestion) {
	// 简单冒泡排序，按分数降序
	for i := 0; i < len(suggestions)-1; i++ {
		for j := 0; j < len(suggestions)-i-1; j++ {
			if suggestions[j].Score < suggestions[j+1].Score {
				suggestions[j], suggestions[j+1] = suggestions[j+1], suggestions[j]
			}
		}
	}
}

// generateDocID 生成文档ID
func generateDocID(path string) string {
	// 使用路径作为ID
	return path
}

// Close 关闭管理器
func (m *Manager) Close() {
	close(m.stopCh)
}

// IndexDocument 索引单个文档（Tokenizer 方法）
func (t *tokenizer) tokenize(text string) []string {
	// 分离中英文
	segments := splitByLanguage(text)
	var tokens []string

	for _, seg := range segments {
		if isCJK(seg) {
			tokens = append(tokens, tokenizeCJK(seg)...)
		} else {
			tokens = append(tokens, tokenizeEnglish(seg)...)
		}
	}

	// 过滤停用词和短词
	var filtered []string
	for _, token := range tokens {
		token = strings.ToLower(strings.TrimSpace(token))
		if len(token) < 2 {
			continue
		}
		if len(token) > 100 {
			continue
		}
		if t.stopWords[token] {
			continue
		}
		filtered = append(filtered, token)
	}

	return filtered
}
