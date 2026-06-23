// Package aifilesearch AI 智能文件搜索
package aifilesearch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SearchQuery 搜索查询
type SearchQuery struct {
	Query     string            `json:"query"`
	Path      string            `json:"path,omitempty"`
	Filters   map[string]string `json:"filters,omitempty"`
	Limit     int               `json:"limit,omitempty"`
	Offset    int               `json:"offset,omitempty"`
}

// SearchResult 搜索结果
type SearchResult struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"modTime"`
	IsDir       bool      `json:"isDir"`
	Score       float64   `json:"score"`
	MatchType   string    `json:"matchType"`
	Snippet     string    `json:"snippet,omitempty"`
	ContentType string    `json:"contentType,omitempty"`
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Query       string         `json:"query"`
	TotalCount  int            `json:"totalCount"`
	Results     []SearchResult `json:"results"`
	Suggestions []string       `json:"suggestions,omitempty"`
	SearchTime  int64          `json:"searchTime"`
}

// FileIndex 文件索引
type FileIndex struct {
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Content   string    `json:"content,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	IndexedAt time.Time `json:"indexedAt"`
}

// Config AI 文件搜索配置
type Config struct {
	IndexRoot     string `json:"indexRoot"`
	MaxIndexSize  int64  `json:"maxIndexSize"`
	EnableContent bool   `json:"enableContent"`
	EnableOCR     bool   `json:"enableOCR"`
}

// Manager AI 文件搜索管理器
type Manager struct {
	mu       sync.RWMutex
	config   Config
	index    map[string]*FileIndex
	stopChan chan struct{}
}

// NewManager 创建 AI 文件搜索管理器
func NewManager(config Config) *Manager {
	return &Manager{
		config:   config,
		index:    make(map[string]*FileIndex),
		stopChan: make(chan struct{}),
	}
}

// Start 启动索引服务
func (m *Manager) Start() {
	go m.indexLoop()
}

// Stop 停止索引服务
func (m *Manager) Stop() {
	close(m.stopChan)
}

func (m *Manager) indexLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// 初始索引
	m.buildIndex()

	for {
		select {
		case <-ticker.C:
			m.buildIndex()
		case <-m.stopChan:
			return
		}
	}
}

func (m *Manager) buildIndex() {
	if m.config.IndexRoot == "" {
		return
	}

	filepath.Walk(m.config.IndexRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// 跳过隐藏文件
		if strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 索引文件
		if !info.IsDir() {
			m.indexFile(path, info)
		}

		return nil
	})
}

func (m *Manager) indexFile(path string, info os.FileInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查文件大小限制
	if m.config.MaxIndexSize > 0 && info.Size() > m.config.MaxIndexSize {
		return
	}

	index := &FileIndex{
		Path:      path,
		Name:      info.Name(),
		Tags:      extractTags(path),
		Metadata:  extractMetadata(path, info),
		IndexedAt: time.Now(),
	}

	// 如果启用内容索引，读取文本内容
	if m.config.EnableContent && isTextFile(path) {
		content, err := readFileContent(path, 1024*1024) // 最大 1MB
		if err == nil {
			index.Content = content
		}
	}

	m.index[path] = index
}

// Search 搜索文件
func (m *Manager) Search(query SearchQuery) SearchResponse {
	start := time.Now()

	m.mu.RLock()
	defer m.mu.RUnlock()

	query.Query = strings.ToLower(query.Query)
	var results []SearchResult

	for _, idx := range m.index {
		score := m.calculateScore(idx, query)
		if score > 0 {
			result := SearchResult{
				ID:        idx.Path,
				Path:      idx.Path,
				Name:      idx.Name,
				Score:     score,
				ModTime:   idx.IndexedAt,
			}

			// 获取文件信息
			if info, err := os.Stat(idx.Path); err == nil {
				result.Size = info.Size()
				result.IsDir = info.IsDir()
				result.ModTime = info.ModTime()
			}

			// 匹配类型
			if strings.Contains(strings.ToLower(idx.Name), query.Query) {
				result.MatchType = "filename"
			} else if strings.Contains(strings.ToLower(idx.Content), query.Query) {
				result.MatchType = "content"
				result.Snippet = extractSnippet(idx.Content, query.Query, 100)
			} else {
				result.MatchType = "tag"
			}

			results = append(results, result)
		}
	}

	// 排序
	sortResults(results)

	// 分页
	totalCount := len(results)
	if query.Offset > 0 && query.Offset < len(results) {
		results = results[query.Offset:]
	}
	if query.Limit > 0 && query.Limit < len(results) {
		results = results[:query.Limit]
	}

	return SearchResponse{
		Query:       query.Query,
		TotalCount:  totalCount,
		Results:     results,
		Suggestions: m.generateSuggestions(query.Query),
		SearchTime:  time.Since(start).Milliseconds(),
	}
}

func (m *Manager) calculateScore(idx *FileIndex, query SearchQuery) float64 {
	score := 0.0
	queryLower := query.Query

	// 文件名匹配
	if strings.Contains(strings.ToLower(idx.Name), queryLower) {
		score += 10.0
		// 完全匹配加分
		if strings.ToLower(idx.Name) == queryLower {
			score += 5.0
		}
	}

	// 内容匹配
	if strings.Contains(strings.ToLower(idx.Content), queryLower) {
		score += 5.0
	}

	// 标签匹配
	for _, tag := range idx.Tags {
		if strings.Contains(strings.ToLower(tag), queryLower) {
			score += 3.0
		}
	}

	// 过滤器
	if query.Filters != nil {
		if ext, ok := query.Filters["ext"]; ok {
			if !strings.HasSuffix(strings.ToLower(idx.Name), strings.ToLower(ext)) {
				return 0
			}
		}
		if path, ok := query.Filters["path"]; ok {
			if !strings.HasPrefix(idx.Path, path) {
				return 0
			}
		}
	}

	return score
}

func (m *Manager) generateSuggestions(query string) []string {
	var suggestions []string
	queryLower := strings.ToLower(query)

	// 基于索引生成建议
	seen := make(map[string]bool)
	for _, idx := range m.index {
		name := strings.ToLower(idx.Name)
		if strings.Contains(name, queryLower) && !seen[name] {
			suggestions = append(suggestions, idx.Name)
			seen[name] = true
			if len(suggestions) >= 5 {
				break
			}
		}
	}

	return suggestions
}

// GetIndexStats 获取索引统计
func (m *Manager) GetIndexStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalSize int64
	fileCount := 0
	dirCount := 0

	for _, idx := range m.index {
		if info, err := os.Stat(idx.Path); err == nil {
			if info.IsDir() {
				dirCount++
			} else {
				fileCount++
				totalSize += info.Size()
			}
		}
	}

	return map[string]interface{}{
		"totalFiles": fileCount,
		"totalDirs":  dirCount,
		"totalSize":  totalSize,
		"indexSize":  len(m.index),
	}
}

// RebuildIndex 重建索引
func (m *Manager) RebuildIndex() {
	m.mu.Lock()
	m.index = make(map[string]*FileIndex)
	m.mu.Unlock()
	m.buildIndex()
}

func extractTags(path string) []string {
	var tags []string
	ext := filepath.Ext(path)
	if ext != "" {
		tags = append(tags, ext[1:])
	}
	return tags
}

func extractMetadata(path string, info os.FileInfo) map[string]string {
	return map[string]string{
		"ext":      filepath.Ext(path),
		"dir":      filepath.Dir(path),
		"size":     fmt.Sprintf("%d", info.Size()),
		"modTime":  info.ModTime().Format(time.RFC3339),
	}
}

func isTextFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	textExts := []string{".txt", ".md", ".go", ".py", ".js", ".ts", ".html", ".css", ".json", ".xml", ".yaml", ".yml", ".toml", ".ini", ".conf", ".log"}
	for _, t := range textExts {
		if ext == t {
			return true
		}
	}
	return false
}

func readFileContent(path string, maxSize int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	stat, _ := file.Stat()
	if stat.Size() > maxSize {
		buf := make([]byte, maxSize)
		n, _ := file.Read(buf)
		return string(buf[:n]), nil
	}

	buf := make([]byte, stat.Size())
	n, _ := file.Read(buf)
	return string(buf[:n]), nil
}

func extractSnippet(content, query string, contextLen int) string {
	idx := strings.Index(strings.ToLower(content), strings.ToLower(query))
	if idx < 0 {
		return ""
	}

	start := idx - contextLen
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + contextLen
	if end > len(content) {
		end = len(content)
	}

	snippet := content[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}

	return snippet
}

func sortResults(results []SearchResult) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}
