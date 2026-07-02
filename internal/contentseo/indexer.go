// Package contentseo 提供内容索引逻辑
package contentseo

import (
	"log"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// Indexer 内容索引器.
type Indexer struct {
	engine    *Engine
	config    *Config
	index     map[string]*ContentIndex // fileID -> ContentIndex
	indexPath map[string]string        // path -> fileID
	tokens    map[string][]string      // token -> fileIDs
	queryLog  []QueryLog               // 查询日志
	mu        sync.RWMutex
	stopCh    chan struct{}
	running   bool
}

// QueryLog 查询日志.
type QueryLog struct {
	Keyword   string    `json:"keyword"`
	Timestamp time.Time `json:"timestamp"`
}

// NewIndexer 创建索引器.
func NewIndexer(engine *Engine, config *Config) *Indexer {
	return &Indexer{
		engine:    engine,
		config:    config,
		index:     make(map[string]*ContentIndex),
		indexPath: make(map[string]string),
		tokens:    make(map[string][]string),
		queryLog:  make([]QueryLog, 0),
		stopCh:    make(chan struct{}),
	}
}

// Search 执行搜索.
func (idx *Indexer) Search(query SearchQuery) ([]ContentIndex, int, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	keywords := tokenize(query.Keyword)
	if len(keywords) == 0 {
		return nil, 0, nil
	}

	// 收集匹配的文件ID
	matchScores := make(map[string]float64)

	for _, keyword := range keywords {
		keyword = strings.ToLower(keyword)

		// 精确匹配
		if fileIDs, ok := idx.tokens[keyword]; ok {
			for _, fileID := range fileIDs {
				matchScores[fileID] += 1.0
			}
		}

		// 前缀匹配
		for token, fileIDs := range idx.tokens {
			if strings.HasPrefix(token, keyword) && token != keyword {
				for _, fileID := range fileIDs {
					matchScores[fileID] += 0.5
				}
			}
		}
	}

	// 应用过滤器
	var results []ContentIndex
	for fileID, score := range matchScores {
		item, ok := idx.index[fileID]
		if !ok {
			continue
		}

		// 应用过滤条件
		if !applyFilters(item, query.Filters) {
			continue
		}

		result := *item
		result.Score = score
		results = append(results, result)
	}

	// 排序
	sortResults(results, query.SortBy)

	// 分页
	total := len(results)
	start := (query.Page - 1) * query.PageSize
	if start >= total {
		return nil, total, nil
	}

	end := start + query.PageSize
	if end > total {
		end = total
	}

	return results[start:end], total, nil
}

// IndexDocument 索引文档.
func (idx *Indexer) IndexDocument(path, title, content string, tags []string, language string) string {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// 检查是否已存在
	if fileID, ok := idx.indexPath[path]; ok {
		// 更新现有索引
		item := idx.index[fileID]
		item.Title = title
		item.Content = content
		item.Tags = tags
		item.Language = language
		item.IndexedAt = time.Now()

		// 重新建立token索引
		idx.reindexTokens(fileID, title, content, tags)
		return fileID
	}

	// 创建新索引
	fileID := uuid.New().String()
	item := &ContentIndex{
		FileID:    fileID,
		Path:      path,
		Title:     title,
		Content:   content,
		Tags:      tags,
		Language:  language,
		IndexedAt: time.Now(),
	}

	idx.index[fileID] = item
	idx.indexPath[path] = fileID

	// 建立token索引
	idx.reindexTokens(fileID, title, content, tags)

	return fileID
}

// RemoveDocument 移除文档.
func (idx *Indexer) RemoveDocument(path string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	fileID, ok := idx.indexPath[path]
	if !ok {
		return
	}

	// 移除token索引
	for token, fileIDs := range idx.tokens {
		newIDs := make([]string, 0, len(fileIDs))
		for _, id := range fileIDs {
			if id != fileID {
				newIDs = append(newIDs, id)
			}
		}
		if len(newIDs) == 0 {
			delete(idx.tokens, token)
		} else {
			idx.tokens[token] = newIDs
		}
	}

	delete(idx.index, fileID)
	delete(idx.indexPath, path)
}

// Rebuild 重建索引.
func (idx *Indexer) Rebuild(fullRebuild bool) error {
	idx.mu.Lock()
	if idx.running {
		idx.mu.Unlock()
		return nil
	}
	idx.running = true
	idx.mu.Unlock()

	defer func() {
		idx.mu.Lock()
		idx.running = false
		idx.mu.Unlock()
	}()

	if fullRebuild {
		idx.mu.Lock()
		idx.index = make(map[string]*ContentIndex)
		idx.indexPath = make(map[string]string)
		idx.tokens = make(map[string][]string)
		idx.mu.Unlock()
		log.Println("全量索引重建完成")
	} else {
		log.Println("增量索引更新完成")
	}

	return nil
}

// Stop 停止索引器.
func (idx *Indexer) Stop() {
	select {
	case <-idx.stopCh:
		// 已关闭
	default:
		close(idx.stopCh)
	}
}

// GetIndexedCount 获取已索引数量.
func (idx *Indexer) GetIndexedCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.index)
}

// reindexTokens 重建token索引.
func (idx *Indexer) reindexTokens(fileID, title, content string, tags []string) {
	// 移除旧的token引用
	for token, fileIDs := range idx.tokens {
		newIDs := make([]string, 0, len(fileIDs))
		for _, id := range fileIDs {
			if id != fileID {
				newIDs = append(newIDs, id)
			}
		}
		if len(newIDs) == 0 {
			delete(idx.tokens, token)
		} else {
			idx.tokens[token] = newIDs
		}
	}

	// 分词并建立新索引
	allText := title + " " + content + " " + strings.Join(tags, " ")
	tokens := tokenize(allText)

	for _, token := range tokens {
		token = strings.ToLower(token)
		if token == "" {
			continue
		}
		idx.tokens[token] = append(idx.tokens[token], fileID)
	}
}

// tokenize 中英文分词.
func tokenize(text string) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			// 中文字符单独成词
			if unicode.Is(unicode.Han, r) {
				tokens = append(tokens, string(r))
			}
		} else if unicode.Is(unicode.Han, r) {
			// 中文字符
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, string(r))
		} else {
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// applyFilters 应用过滤条件.
func applyFilters(item *ContentIndex, filters []Filter) bool {
	for _, filter := range filters {
		switch filter.Field {
		case "language":
			if !matchFilter(item.Language, filter) {
				return false
			}
		case "tags":
			matched := false
			for _, tag := range item.Tags {
				if matchFilter(tag, filter) {
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
		case "path":
			if !matchFilter(item.Path, filter) {
				return false
			}
		}
	}
	return true
}

// matchFilter 匹配过滤条件.
func matchFilter(value string, filter Filter) bool {
	filterValue, ok := filter.Value.(string)
	if !ok {
		return false
	}

	switch filter.Operator {
	case "eq":
		return value == filterValue
	case "ne":
		return value != filterValue
	case "contains":
		return strings.Contains(strings.ToLower(value), strings.ToLower(filterValue))
	default:
		return false
	}
}

// sortResults 排序结果.
func sortResults(results []ContentIndex, sortBy SortBy) {
	// 简单的插入排序
	for i := 1; i < len(results); i++ {
		key := results[i]
		j := i - 1

		for j >= 0 && shouldSwap(results[j], key, sortBy) {
			results[j+1] = results[j]
			j--
		}
		results[j+1] = key
	}
}

// shouldSwap 判断是否需要交换.
func shouldSwap(a, b ContentIndex, sortBy SortBy) bool {
	switch sortBy {
	case SortByTimeDesc:
		return a.IndexedAt.Before(b.IndexedAt)
	case SortByTimeAsc:
		return a.IndexedAt.After(b.IndexedAt)
	case SortBySizeDesc:
		return len(a.Content) < len(b.Content)
	case SortBySizeAsc:
		return len(a.Content) > len(b.Content)
	default: // relevance
		return a.Score < b.Score
	}
}
