// Package fileindex 提供文件全文索引与搜索功能
// Version: v1.0.0 - 文件索引
package fileindex

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// IndexEntry 索引条目
type IndexEntry struct {
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	IsDir     bool      `json:"isDir"`
	ModTime   time.Time `json:"modTime"`
	Extension string    `json:"extension"`
	Checksum  string    `json:"checksum,omitempty"`
	LineCount int       `json:"lineCount,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	Summary   string    `json:"summary,omitempty"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Entry      IndexEntry `json:"entry"`
	Score      float64    `json:"score"`
	Snippets   []string   `json:"snippets,omitempty"`
	MatchCount int        `json:"matchCount"`
}

// SearchQuery 搜索查询
type SearchQuery struct {
	Keyword    string   `json:"keyword"`
	Path       string   `json:"path"`
	Extensions []string `json:"extensions"`
	MinSize    int64    `json:"minSize"`
	MaxSize    int64    `json:"maxSize"`
	After      string   `json:"after"`
	Before     string   `json:"before"`
	SearchType string   `json:"searchType"` // name, content, all
	Limit      int      `json:"limit"`
	Offset     int      `json:"offset"`
}

// IndexStats 索引统计
type IndexStats struct {
	TotalFiles int            `json:"totalFiles"`
	TotalDirs  int            `json:"totalDirs"`
	TotalSize  int64          `json:"totalSize"`
	IndexedAt  time.Time      `json:"indexedAt"`
	Duration   string         `json:"duration"`
	IndexSize  int64          `json:"indexSize"`
	Extensions map[string]int `json:"extensions"`
}

// Indexer 文件索引器
type Indexer struct {
	logger   *zap.Logger
	mu       sync.RWMutex
	entries  map[string]*IndexEntry
	basePath string
	excludes map[string]bool
	maxSize  int64
}

var (
	ErrNotIndexed = fmt.Errorf("索引尚未建立")
	ErrPathDenied = fmt.Errorf("路径不在允许范围内")
)

// NewIndexer 创建文件索引器
func NewIndexer(logger *zap.Logger, basePath string) *Indexer {
	return &Indexer{
		logger:   logger,
		entries:  make(map[string]*IndexEntry),
		basePath: basePath,
		maxSize:  100 * 1024 * 1024, // 100MB
		excludes: map[string]bool{
			".git":         true,
			"node_modules": true,
			".DS_Store":    true,
			"Thumbs.db":    true,
			".Trash":       true,
		},
	}
}

// Build 构建索引
func (idx *Indexer) Build() (*IndexStats, error) {
	start := time.Now()
	idx.mu.Lock()
	idx.entries = make(map[string]*IndexEntry)
	idx.mu.Unlock()

	stats := &IndexStats{
		Extensions: make(map[string]int),
	}

	err := filepath.Walk(idx.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}

		name := info.Name()
		// 排除隐藏文件和目录
		if idx.excludes[name] || (strings.HasPrefix(name, ".") && name != ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		entry := &IndexEntry{
			Path:      path,
			Name:      name,
			Size:      info.Size(),
			IsDir:     info.IsDir(),
			ModTime:   info.ModTime(),
			Extension: strings.ToLower(filepath.Ext(name)),
		}

		if info.IsDir() {
			stats.TotalDirs++
		} else {
			stats.TotalFiles++
			stats.TotalSize += info.Size()
			if entry.Extension != "" {
				stats.Extensions[entry.Extension]++
			}

			// 索引文本文件内容
			if idx.isTextFile(entry.Extension) && info.Size() <= idx.maxSize {
				idx.indexFileContent(entry)
			}
		}

		idx.mu.Lock()
		idx.entries[path] = entry
		idx.mu.Unlock()

		return nil
	})

	stats.IndexedAt = time.Now()
	stats.Duration = time.Since(start).String()

	idx.logger.Info("索引构建完成",
		zap.Int("files", stats.TotalFiles),
		zap.Int("dirs", stats.TotalDirs),
		zap.String("duration", stats.Duration))

	return stats, err
}

// isTextFile 判断是否为文本文件
func (idx *Indexer) isTextFile(ext string) bool {
	textExts := map[string]bool{
		".txt": true, ".md": true, ".go": true, ".py": true,
		".js": true, ".ts": true, ".json": true, ".yaml": true,
		".yml": true, ".toml": true, ".xml": true, ".html": true,
		".css": true, ".sh": true, ".bash": true, ".conf": true,
		".cfg": true, ".ini": true, ".log": true, ".csv": true,
		".rst": true, ".tex": true, ".sql": true, ".c": true,
		".cpp": true, ".h": true, ".java": true, ".rs": true,
		".rb": true, ".php": true, ".pl": true, ".lua": true,
		".vim": true, ".el": true, ".lisp": true,
	}
	return textExts[ext]
}

// indexFileContent 索引文件内容
func (idx *Indexer) indexFileContent(entry *IndexEntry) {
	f, err := os.Open(entry.Path)
	if err != nil {
		return
	}
	defer f.Close()

	hasher := sha256.New()
	lines := 0
	reader := bufio.NewReader(io.TeeReader(f, hasher))

	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			lines++
			// 截取前 500 字符作为摘要
			if entry.Summary == "" && len(strings.TrimSpace(line)) > 10 {
				summary := strings.TrimSpace(line)
				if len(summary) > 500 {
					summary = summary[:500]
				}
				entry.Summary = summary
			}
		}
		if err != nil {
			break
		}
	}

	entry.LineCount = lines
	entry.Checksum = fmt.Sprintf("%x", hasher.Sum(nil))[:16]
}

// Search 搜索文件
func (idx *Indexer) Search(query SearchQuery) []SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.entries) == 0 {
		return nil
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}

	var results []SearchResult
	keyword := strings.ToLower(query.Keyword)

	for _, entry := range idx.entries {
		if !idx.matchesQuery(entry, query) {
			continue
		}

		score := idx.calcScore(entry, keyword, query.SearchType)
		if score <= 0 {
			continue
		}

		result := SearchResult{
			Entry: *entry,
			Score: score,
		}

		// 内容搜索时提取匹配片段
		if query.SearchType == "content" || query.SearchType == "all" {
			result.Snippets = idx.extractSnippets(entry.Path, query.Keyword)
			result.MatchCount = len(result.Snippets)
		}

		results = append(results, result)
	}

	// 按分数排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// 分页
	if query.Offset > 0 && query.Offset < len(results) {
		results = results[query.Offset:]
	}
	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

// matchesQuery 检查条目是否匹配查询条件
func (idx *Indexer) matchesQuery(entry *IndexEntry, query SearchQuery) bool {
	// 路径前缀过滤
	if query.Path != "" && !strings.HasPrefix(entry.Path, query.Path) {
		return false
	}

	// 扩展名过滤
	if len(query.Extensions) > 0 {
		found := false
		for _, ext := range query.Extensions {
			if entry.Extension == strings.ToLower(ext) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 大小过滤
	if query.MinSize > 0 && entry.Size < query.MinSize {
		return false
	}
	if query.MaxSize > 0 && entry.Size > query.MaxSize {
		return false
	}

	return true
}

// calcScore 计算搜索分数
func (idx *Indexer) calcScore(entry *IndexEntry, keyword, searchType string) float64 {
	if keyword == "" {
		return 1.0
	}

	score := 0.0
	nameLower := strings.ToLower(entry.Name)
	pathLower := strings.ToLower(entry.Path)

	// 名称匹配
	if searchType == "name" || searchType == "all" || searchType == "" {
		if strings.Contains(nameLower, keyword) {
			score += 10.0
			// 完全匹配加分
			if nameLower == keyword {
				score += 5.0
			}
		}
		if strings.Contains(pathLower, keyword) {
			score += 3.0
		}
	}

	// 内容匹配
	if searchType == "content" || searchType == "all" {
		if entry.Summary != "" && strings.Contains(strings.ToLower(entry.Summary), keyword) {
			score += 5.0
		}
	}

	return score
}

// extractSnippets 提取匹配片段
func (idx *Indexer) extractSnippets(path, keyword string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var snippets []string
	lower := strings.ToLower(keyword)
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.Contains(strings.ToLower(line), lower) {
			snippet := strings.TrimSpace(line)
			if len(snippet) > 200 {
				snippet = snippet[:200] + "..."
			}
			snippets = append(snippets, fmt.Sprintf("L%d: %s", lineNum, snippet))
			if len(snippets) >= 5 {
				break
			}
		}
	}

	return snippets
}

// GetEntry 获取单个索引条目
func (idx *Indexer) GetEntry(path string) (*IndexEntry, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	entry, ok := idx.entries[path]
	return entry, ok
}

// Stats 获取索引统计
func (idx *Indexer) Stats() IndexStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	stats := IndexStats{
		Extensions: make(map[string]int),
	}

	for _, entry := range idx.entries {
		if entry.IsDir {
			stats.TotalDirs++
		} else {
			stats.TotalFiles++
			stats.TotalSize += entry.Size
			if entry.Extension != "" {
				stats.Extensions[entry.Extension]++
			}
		}
	}

	return stats
}

// ListRecent 最近修改的文件
func (idx *Indexer) ListRecent(limit int) []IndexEntry {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	var files []*IndexEntry
	for _, entry := range idx.entries {
		if !entry.IsDir {
			files = append(files, entry)
		}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})

	if len(files) > limit {
		files = files[:limit]
	}

	result := make([]IndexEntry, len(files))
	for i, f := range files {
		result[i] = *f
	}
	return result
}

// ListLargest 最大的文件
func (idx *Indexer) ListLargest(limit int) []IndexEntry {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	var files []*IndexEntry
	for _, entry := range idx.entries {
		if !entry.IsDir {
			files = append(files, entry)
		}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Size > files[j].Size
	})

	if len(files) > limit {
		files = files[:limit]
	}

	result := make([]IndexEntry, len(files))
	for i, f := range files {
		result[i] = *f
	}
	return result
}

// SetExcludes 设置排除列表
func (idx *Indexer) SetExcludes(excludes []string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.excludes = make(map[string]bool)
	for _, e := range excludes {
		idx.excludes[e] = true
	}
}

// Count 返回索引条目数
func (idx *Indexer) Count() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.entries)
}
