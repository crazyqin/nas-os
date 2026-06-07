// Package aisearch 提供搜索建议和自动补全功能
package aisearch

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// Suggester 搜索建议器
type Suggester struct {
	mu             sync.RWMutex
	trie           *Trie
	history        []SearchHistory
	hotWords       map[string]int64
	maxHistory     int
	maxSuggestions int
}

// NewSuggester 创建搜索建议器
func NewSuggester(maxHistory, maxSuggestions int) *Suggester {
	return &Suggester{
		trie:           NewTrie(),
		history:        make([]SearchHistory, 0),
		hotWords:       make(map[string]int64),
		maxHistory:     maxHistory,
		maxSuggestions: maxSuggestions,
	}
}

// AddDocument 添加文档到建议索引
func (s *Suggester) AddDocument(doc *SearchIndex) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 添加文件名
	s.trie.Insert(doc.FileName, doc.ID)

	// 添加标签
	for _, tag := range doc.Tags {
		s.trie.Insert(tag, doc.ID)
	}

	// 添加内容关键词
	words := extractKeywords(doc.Content)
	for _, word := range words {
		s.trie.Insert(word, doc.ID)
	}
}

// RemoveDocument 从建议索引中移除文档
func (s *Suggester) RemoveDocument(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.trie.RemoveByID(id)
}

// Suggest 获取搜索建议
func (s *Suggester) Suggest(prefix string, limit int) []Suggestion {
	if limit <= 0 {
		limit = s.maxSuggestions
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	suggestions := make([]Suggestion, 0)
	seen := make(map[string]bool)

	// 1. 从 Trie 中获取前缀匹配
	trieResults := s.trie.Search(prefix, limit*2)
	for _, result := range trieResults {
		if !seen[result.Text] {
			seen[result.Text] = true
			suggestions = append(suggestions, Suggestion{
				Text:  result.Text,
				Score: result.Score,
			})
		}
	}

	// 2. 从搜索历史中获取
	for _, h := range s.history {
		if strings.Contains(strings.ToLower(h.Keyword), strings.ToLower(prefix)) {
			if !seen[h.Keyword] {
				seen[h.Keyword] = true
				suggestions = append(suggestions, Suggestion{
					Text:  h.Keyword,
					Score: 0.8,
				})
			}
		}
	}

	// 3. 从热词中获取
	for word, count := range s.hotWords {
		if strings.Contains(strings.ToLower(word), strings.ToLower(prefix)) {
			if !seen[word] {
				seen[word] = true
				suggestions = append(suggestions, Suggestion{
					Text:  word,
					Score: float64(count) / 1000,
					Count: count,
				})
			}
		}
	}

	// 排序并截断
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Score > suggestions[j].Score
	})

	if len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}

	return suggestions
}

// AddHistory 添加搜索历史
func (s *Suggester) AddHistory(history SearchHistory) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.history = append(s.history, history)

	// 限制历史记录数量
	if len(s.history) > s.maxHistory {
		s.history = s.history[len(s.history)-s.maxHistory:]
	}

	// 更新热词
	words := strings.Fields(history.Keyword)
	for _, word := range words {
		s.hotWords[word]++
	}
}

// GetHotWords 获取热词
func (s *Suggester) GetHotWords(limit int) []HotWord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hotWords := make([]HotWord, 0, len(s.hotWords))
	for word, count := range s.hotWords {
		hotWords = append(hotWords, HotWord{
			Word:  word,
			Count: count,
		})
	}

	sort.Slice(hotWords, func(i, j int) bool {
		return hotWords[i].Count > hotWords[j].Count
	})

	if len(hotWords) > limit {
		hotWords = hotWords[:limit]
	}

	return hotWords
}

// ClearHistory 清除搜索历史
func (s *Suggester) ClearHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.history = make([]SearchHistory, 0)
}

// Trie 前缀树
type Trie struct {
	root *TrieNode
}

// TrieNode 前缀树节点
type TrieNode struct {
	children map[rune]*TrieNode
	ids      map[string]bool
	isEnd    bool
	score    float64
	text     string
}

// NewTrie 创建前缀树
func NewTrie() *Trie {
	return &Trie{
		root: &TrieNode{
			children: make(map[rune]*TrieNode),
			ids:      make(map[string]bool),
		},
	}
}

// Insert 插入词
func (t *Trie) Insert(word string, id string) {
	node := t.root
	for _, ch := range strings.ToLower(word) {
		if _, ok := node.children[ch]; !ok {
			node.children[ch] = &TrieNode{
				children: make(map[rune]*TrieNode),
				ids:      make(map[string]bool),
			}
		}
		node = node.children[ch]
	}
	node.isEnd = true
	node.ids[id] = true
	node.text = word
	node.score++
}

// Search 搜索前缀
func (t *Trie) Search(prefix string, limit int) []Suggestion {
	node := t.root
	for _, ch := range strings.ToLower(prefix) {
		if _, ok := node.children[ch]; !ok {
			return nil
		}
		node = node.children[ch]
	}

	results := make([]Suggestion, 0)
	t.collect(node, prefix, &results, limit)

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

// collect 收集节点下的所有词
func (t *Trie) collect(node *TrieNode, prefix string, results *[]Suggestion, limit int) {
	if len(*results) >= limit {
		return
	}

	if node.isEnd {
		*results = append(*results, Suggestion{
			Text:  node.text,
			Score: node.score,
		})
	}

	for ch, child := range node.children {
		t.collect(child, prefix+string(ch), results, limit)
	}
}

// RemoveByID 根据 ID 移除
func (t *Trie) RemoveByID(id string) {
	t.removeByIDHelper(t.root, id)
}

// removeByIDHelper 递归移除 ID
func (t *Trie) removeByIDHelper(node *TrieNode, id string) {
	delete(node.ids, id)
	if len(node.ids) == 0 {
		node.isEnd = false
	}

	for _, child := range node.children {
		t.removeByIDHelper(child, id)
	}
}

// extractKeywords 提取关键词
func extractKeywords(content string) []string {
	if content == "" {
		return nil
	}

	// 简单分词：按空格和标点分割
	words := strings.FieldsFunc(content, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' ||
			r == ',' || r == '.' || r == '!' || r == '?' ||
			r == ';' || r == ':' || r == '"' || r == '\'' ||
			r == '(' || r == ')' || r == '[' || r == ']' ||
			r == '{' || r == '}' || r == '/' || r == '\\' ||
			r == '@' || r == '#' || r == '$' || r == '%' ||
			r == '^' || r == '&' || r == '*' || r == '+' ||
			r == '=' || r == '|' || r == '~' || r == '`'
	})

	// 过滤短词和停用词
	keywords := make([]string, 0)
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "shall": true, "can": true,
		"and": true, "or": true, "but": true, "if": true, "then": true,
		"else": true, "when": true, "at": true, "by": true,
		"for": true, "with": true, "about": true, "against": true, "between": true,
		"through": true, "during": true, "before": true, "after": true, "above": true,
		"below": true, "to": true, "up": true, "down": true,
		"in": true, "out": true, "on": true, "off": true, "over": true,
		"under": true, "again": true, "further": true, "once": true,
		"的": true, "了": true, "在": true, "是": true, "我": true,
		"有": true, "和": true, "就": true, "不": true, "人": true,
		"都": true, "一": true, "一个": true, "上": true, "也": true,
		"很": true, "到": true, "说": true, "要": true, "去": true,
		"你": true, "会": true, "着": true, "没有": true, "看": true,
		"好": true, "自己": true, "这": true, "他": true, "她": true,
	}

	for _, word := range words {
		word = strings.ToLower(strings.TrimSpace(word))
		if len(word) < 2 {
			continue
		}
		if stopWords[word] {
			continue
		}
		keywords = append(keywords, word)
	}

	// 去重并限制数量
	seen := make(map[string]bool)
	unique := make([]string, 0)
	for _, word := range keywords {
		if !seen[word] {
			seen[word] = true
			unique = append(unique, word)
		}
	}

	if len(unique) > 100 {
		unique = unique[:100]
	}

	return unique
}

// AutoComplete 自动补全
type AutoComplete struct {
	suggester *Suggester
	cache     map[string][]Suggestion
	cacheTTL  time.Duration
	mu        sync.RWMutex
}

// NewAutoComplete 创建自动补全
func NewAutoComplete(suggester *Suggester, cacheTTL time.Duration) *AutoComplete {
	return &AutoComplete{
		suggester: suggester,
		cache:     make(map[string][]Suggestion),
		cacheTTL:  cacheTTL,
	}
}

// Complete 自动补全
func (ac *AutoComplete) Complete(prefix string, limit int) []Suggestion {
	ac.mu.RLock()
	if cached, ok := ac.cache[prefix]; ok {
		ac.mu.RUnlock()
		return cached
	}
	ac.mu.RUnlock()

	suggestions := ac.suggester.Suggest(prefix, limit)

	ac.mu.Lock()
	ac.cache[prefix] = suggestions
	ac.mu.Unlock()

	// 异步清理过期缓存
	go func() {
		time.Sleep(ac.cacheTTL)
		ac.mu.Lock()
		delete(ac.cache, prefix)
		ac.mu.Unlock()
	}()

	return suggestions
}

// ClearCache 清除缓存
func (ac *AutoComplete) ClearCache() {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.cache = make(map[string][]Suggestion)
}
