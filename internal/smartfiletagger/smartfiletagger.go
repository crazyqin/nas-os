// Package smartfiletagger 提供智能文件标签系统
// AI自动标签、语义搜索、标签云、跨文件关联、批量标签管理
// 对标群晖 Synology Photos 智能标签 + TrueNAS 文件索引
package smartfiletagger

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ========== 常量 ==========

const (
	Version             = "1.0.0"
	MaxTags             = 10000
	MaxFiles            = 100000
	MaxTagLength        = 64
	MaxAutoTagsPerFile  = 20
	MinConfidence       = 0.3
	HighConfidence      = 0.8
	IndexRebuildTimeout = 5 * time.Minute
	BatchSize           = 100
)

// ========== 类型定义 ==========

// TagCategory 标签分类
type TagCategory string

const (
	CategoryDocument  TagCategory = "document"
	CategoryImage     TagCategory = "image"
	CategoryVideo     TagCategory = "video"
	CategoryAudio     TagCategory = "audio"
	CategoryArchive   TagCategory = "archive"
	CategoryCode      TagCategory = "code"
	CategoryCustom    TagCategory = "custom"
	CategoryAI        TagCategory = "ai_generated"
	CategorySystem    TagCategory = "system"
	CategoryDuplicate TagCategory = "duplicate"
)

// Tag 标签定义
type Tag struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Category   TagCategory `json:"category"`
	Color      string      `json:"color"`
	UsageCount int         `json:"usage_count"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
	IsAuto     bool        `json:"is_auto"`
	Confidence float64     `json:"confidence"`
}

// FileTag 文件标签关联
type FileTag struct {
	FilePath   string    `json:"file_path"`
	TagID      string    `json:"tag_id"`
	TagName    string    `json:"tag_name"`
	Confidence float64   `json:"confidence"`
	Source     string    `json:"source"` // manual, ai, rule
	CreatedAt  time.Time `json:"created_at"`
}

// FileIndex 文件索引
type FileIndex struct {
	FilePath    string    `json:"file_path"`
	FileSize    int64     `json:"file_size"`
	ModTime     time.Time `json:"mod_time"`
	ContentType string    `json:"content_type"`
	Hash        string    `json:"hash"`
	Tags        []string  `json:"tags"`
	IndexedAt   time.Time `json:"indexed_at"`
}

// TagRule 标签规则
type TagRule struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Pattern    string      `json:"pattern"`
	Category   TagCategory `json:"category"`
	Tags       []string    `json:"tags"`
	Priority   int         `json:"priority"`
	Enabled    bool        `json:"enabled"`
	CreatedAt  time.Time   `json:"created_at"`
	MatchCount int         `json:"match_count"`
}

// TagCloudEntry 标签云条目
type TagCloudEntry struct {
	Tag    *Tag   `json:"tag"`
	Weight int    `json:"weight"`
	Size   string `json:"size"` // small, medium, large, xlarge
}

// SearchResult 搜索结果
type SearchResult struct {
	FilePath   string   `json:"file_path"`
	Tags       []string `json:"tags"`
	Score      float64  `json:"score"`
	MatchedTag string   `json:"matched_tag"`
}

// TagStats 标签统计
type TagStats struct {
	TotalTags      int             `json:"total_tags"`
	TotalFiles     int             `json:"total_files"`
	TotalRelations int             `json:"total_relations"`
	ByCategory     map[string]int  `json:"by_category"`
	TopTags        []TagCloudEntry `json:"top_tags"`
	AutoTagRatio   float64         `json:"auto_tag_ratio"`
}

// ========== 核心管理器 ==========

// Manager 智能文件标签管理器
type Manager struct {
	mu          sync.RWMutex
	tags        map[string]*Tag
	fileIndex   map[string]*FileIndex
	fileTags    map[string][]*FileTag
	rules       []*TagRule
	dataDir     string
	ctx         context.Context
	cancel      context.CancelFunc
	tagCounter  int64
	ruleCounter int64
}

// NewManager 创建管理器
func NewManager(dataDir string) (*Manager, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		tags:      make(map[string]*Tag),
		fileIndex: make(map[string]*FileIndex),
		fileTags:  make(map[string][]*FileTag),
		rules:     make([]*TagRule, 0),
		dataDir:   dataDir,
		ctx:       ctx,
		cancel:    cancel,
	}

	// 初始化默认规则
	m.initDefaultRules()
	return m, nil
}

// initDefaultRules 初始化默认标签规则
func (m *Manager) initDefaultRules() {
	defaults := []struct {
		name     string
		pattern  string
		category TagCategory
		tags     []string
	}{
		{"文档文件", "*.doc,*.docx,*.pdf,*.txt,*.md", CategoryDocument, []string{"文档", "文本"}},
		{"图片文件", "*.jpg,*.png,*.gif,*.bmp,*.svg,*.webp", CategoryImage, []string{"图片", "媒体"}},
		{"视频文件", "*.mp4,*.avi,*.mkv,*.mov,*.wmv", CategoryVideo, []string{"视频", "媒体"}},
		{"音频文件", "*.mp3,*.wav,*.flac,*.aac,*.ogg", CategoryAudio, []string{"音频", "媒体"}},
		{"压缩文件", "*.zip,*.rar,*.7z,*.tar,*.gz", CategoryArchive, []string{"压缩包", "归档"}},
		{"代码文件", "*.go,*.py,*.js,*.ts,*.java,*.c,*.cpp", CategoryCode, []string{"代码", "开发"}},
	}

	for i, d := range defaults {
		m.ruleCounter++
		m.rules = append(m.rules, &TagRule{
			ID:        fmt.Sprintf("rule_%d", i+1),
			Name:      d.name,
			Pattern:   d.pattern,
			Category:  d.category,
			Tags:      d.tags,
			Priority:  10,
			Enabled:   true,
			CreatedAt: time.Now(),
		})
	}
}

// ========== 标签管理 ==========

// CreateTag 创建标签
func (m *Manager) CreateTag(name string, category TagCategory, color string) (*Tag, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(name) == 0 || len(name) > MaxTagLength {
		return nil, errors.New("标签名长度无效")
	}

	// 检查重复
	for _, t := range m.tags {
		if t.Name == name && t.Category == category {
			return nil, fmt.Errorf("标签已存在: %s/%s", name, category)
		}
	}

	m.tagCounter++
	tag := &Tag{
		ID:        fmt.Sprintf("tag_%d", m.tagCounter),
		Name:      name,
		Category:  category,
		Color:     color,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.tags[tag.ID] = tag
	return tag, nil
}

// GetTag 获取标签
func (m *Manager) GetTag(id string) (*Tag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tag, ok := m.tags[id]
	if !ok {
		return nil, fmt.Errorf("标签不存在: %s", id)
	}
	return tag, nil
}

// DeleteTag 删除标签
func (m *Manager) DeleteTag(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tags[id]; !ok {
		return fmt.Errorf("标签不存在: %s", id)
	}

	// 移除所有文件关联
	for filePath, tags := range m.fileTags {
		filtered := make([]*FileTag, 0, len(tags))
		for _, ft := range tags {
			if ft.TagID != id {
				filtered = append(filtered, ft)
			}
		}
		m.fileTags[filePath] = filtered
	}

	delete(m.tags, id)
	return nil
}

// ListTags 列出所有标签
func (m *Manager) ListTags(category TagCategory) []*Tag {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Tag, 0)
	for _, t := range m.tags {
		if category == "" || t.Category == category {
			result = append(result, t)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].UsageCount > result[j].UsageCount
	})
	return result
}

// ========== 文件标签操作 ==========

// AddFileTag 为文件添加标签
func (m *Manager) AddFileTag(filePath, tagID string, confidence float64, source string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tag, ok := m.tags[tagID]
	if !ok {
		return fmt.Errorf("标签不存在: %s", tagID)
	}

	if confidence < 0 || confidence > 1 {
		confidence = 0.5
	}

	// 检查是否已存在
	for _, ft := range m.fileTags[filePath] {
		if ft.TagID == tagID {
			ft.Confidence = confidence
			ft.Source = source
			return nil
		}
	}

	ft := &FileTag{
		FilePath:   filePath,
		TagID:      tagID,
		TagName:    tag.Name,
		Confidence: confidence,
		Source:     source,
		CreatedAt:  time.Now(),
	}
	m.fileTags[filePath] = append(m.fileTags[filePath], ft)
	tag.UsageCount++
	return nil
}

// RemoveFileTag 移除文件标签
func (m *Manager) RemoveFileTag(filePath, tagID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tags := m.fileTags[filePath]
	for i, ft := range tags {
		if ft.TagID == tagID {
			m.fileTags[filePath] = append(tags[:i], tags[i+1:]...)
			if tag, ok := m.tags[tagID]; ok {
				tag.UsageCount--
			}
			return nil
		}
	}
	return fmt.Errorf("文件标签不存在: %s -> %s", filePath, tagID)
}

// GetFileTags 获取文件的所有标签
func (m *Manager) GetFileTags(filePath string) []*FileTag {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.fileTags[filePath]
}

// GetTagFiles 获取标签关联的所有文件
func (m *Manager) GetTagFiles(tagID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]string, 0)
	for filePath, tags := range m.fileTags {
		for _, ft := range tags {
			if ft.TagID == tagID {
				result = append(result, filePath)
				break
			}
		}
	}
	sort.Strings(result)
	return result
}

// ========== 智能标签 ==========

// AutoTagFile 自动为文件生成标签
func (m *Manager) AutoTagFile(ctx context.Context, filePath string) ([]*FileTag, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	results := make([]*FileTag, 0)
	ext := strings.ToLower(filepath.Ext(filePath))

	// 应用规则匹配
	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}
		if m.matchPattern(ext, rule.Pattern) {
			rule.MatchCount++
			for _, tagName := range rule.Tags {
				tag := m.findOrCreateTagUnsafe(tagName, rule.Category)
				ft := &FileTag{
					FilePath:   filePath,
					TagID:      tag.ID,
					TagName:    tag.Name,
					Confidence: HighConfidence,
					Source:     "rule",
					CreatedAt:  time.Now(),
				}
				results = append(results, ft)
				tag.UsageCount++
			}
		}
	}

	// 基于文件大小的标签
	if info.Size() > 100*1024*1024 {
		tag := m.findOrCreateTagUnsafe("大文件", CategorySystem)
		ft := &FileTag{
			FilePath:   filePath,
			TagID:      tag.ID,
			TagName:    tag.Name,
			Confidence: 0.9,
			Source:     "ai",
			CreatedAt:  time.Now(),
		}
		results = append(results, ft)
		tag.UsageCount++
	}

	// 基于路径的标签
	dir := filepath.Dir(filePath)
	if strings.Contains(dir, "backup") || strings.Contains(dir, "备份") {
		tag := m.findOrCreateTagUnsafe("备份", CategorySystem)
		ft := &FileTag{
			FilePath:   filePath,
			TagID:      tag.ID,
			TagName:    tag.Name,
			Confidence: 0.85,
			Source:     "ai",
			CreatedAt:  time.Now(),
		}
		results = append(results, ft)
		tag.UsageCount++
	}

	// 保存关联
	m.fileTags[filePath] = append(m.fileTags[filePath], results...)
	return results, nil
}

// findOrCreateTagUnsafe 查找或创建标签（无锁）
func (m *Manager) findOrCreateTagUnsafe(name string, category TagCategory) *Tag {
	for _, t := range m.tags {
		if t.Name == name {
			return t
		}
	}
	m.tagCounter++
	tag := &Tag{
		ID:        fmt.Sprintf("tag_%d", m.tagCounter),
		Name:      name,
		Category:  category,
		IsAuto:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.tags[tag.ID] = tag
	return tag
}

// matchPattern 匹配文件模式
func (m *Manager) matchPattern(ext, patterns string) bool {
	for _, p := range strings.Split(patterns, ",") {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "*") {
			if ext == strings.TrimPrefix(p, "*") {
				return true
			}
		}
	}
	return false
}

// ========== 文件索引 ==========

// IndexFile 索引文件
func (m *Manager) IndexFile(ctx context.Context, filePath string) (*FileIndex, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	hash, err := m.computeFileHash(filePath)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	idx := &FileIndex{
		FilePath:    filePath,
		FileSize:    info.Size(),
		ModTime:     info.ModTime(),
		ContentType: m.detectContentType(filePath),
		Hash:        hash,
		Tags:        make([]string, 0),
		IndexedAt:   time.Now(),
	}

	// 收集标签
	if tags, ok := m.fileTags[filePath]; ok {
		for _, ft := range tags {
			idx.Tags = append(idx.Tags, ft.TagName)
		}
	}

	m.fileIndex[filePath] = idx
	return idx, nil
}

// BatchIndex 批量索引目录
func (m *Manager) BatchIndex(ctx context.Context, dir string) (int, error) {
	count := 0
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if info.IsDir() {
			return nil
		}
		if _, err := m.IndexFile(ctx, path); err == nil {
			count++
		}
		return nil
	})
	return count, err
}

// computeFileHash 计算文件哈希
func (m *Manager) computeFileHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	buf := make([]byte, 64*1024)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// detectContentType 检测文件类型
func (m *Manager) detectContentType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	typeMap := map[string]string{
		".doc": "application/msword", ".docx": "application/vnd.openxmlformats",
		".pdf": "application/pdf", ".txt": "text/plain", ".md": "text/markdown",
		".jpg": "image/jpeg", ".png": "image/png", ".gif": "image/gif",
		".mp4": "video/mp4", ".avi": "video/x-msvideo", ".mkv": "video/x-matroska",
		".mp3": "audio/mpeg", ".wav": "audio/wav", ".flac": "audio/flac",
		".zip": "application/zip", ".tar": "application/x-tar", ".gz": "application/gzip",
		".go": "text/x-go", ".py": "text/x-python", ".js": "text/javascript",
	}
	if t, ok := typeMap[ext]; ok {
		return t
	}
	return "application/octet-stream"
}

// ========== 搜索 ==========

// SearchByTag 按标签搜索文件
func (m *Manager) SearchByTag(tagName string) []SearchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]SearchResult, 0)
	for filePath, tags := range m.fileTags {
		for _, ft := range tags {
			if strings.EqualFold(ft.TagName, tagName) {
				fileTags := make([]string, 0, len(tags))
				for _, t := range tags {
					fileTags = append(fileTags, t.TagName)
				}
				results = append(results, SearchResult{
					FilePath:   filePath,
					Tags:       fileTags,
					Score:      ft.Confidence,
					MatchedTag: ft.TagName,
				})
				break
			}
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	return results
}

// SemanticSearch 语义搜索
func (m *Manager) SemanticSearch(query string) []SearchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	query = strings.ToLower(query)
	results := make([]SearchResult, 0)

	for filePath, tags := range m.fileTags {
		score := 0.0
		matchedTag := ""
		fileTags := make([]string, 0, len(tags))

		for _, ft := range tags {
			fileTags = append(fileTags, ft.TagName)
			tagLower := strings.ToLower(ft.TagName)
			if tagLower == query {
				score += 1.0
				matchedTag = ft.TagName
			} else if strings.Contains(tagLower, query) || strings.Contains(query, tagLower) {
				score += 0.6
				matchedTag = ft.TagName
			}
		}

		// 文件路径匹配
		if strings.Contains(strings.ToLower(filePath), query) {
			score += 0.3
		}

		if score > 0 {
			results = append(results, SearchResult{
				FilePath:   filePath,
				Tags:       fileTags,
				Score:      math.Min(score, 1.0),
				MatchedTag: matchedTag,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	return results
}

// ========== 标签云 ==========

// GetTagCloud 获取标签云
func (m *Manager) GetTagCloud(limit int) []TagCloudEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]TagCloudEntry, 0)
	maxCount := 0

	for _, t := range m.tags {
		if t.UsageCount > maxCount {
			maxCount = t.UsageCount
		}
	}

	for _, t := range m.tags {
		size := "small"
		if maxCount > 0 {
			ratio := float64(t.UsageCount) / float64(maxCount)
			switch {
			case ratio > 0.8:
				size = "xlarge"
			case ratio > 0.6:
				size = "large"
			case ratio > 0.3:
				size = "medium"
			}
		}
		entries = append(entries, TagCloudEntry{Tag: t, Weight: t.UsageCount, Size: size})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Weight > entries[j].Weight
	})

	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries
}

// ========== 规则管理 ==========

// CreateRule 创建标签规则
func (m *Manager) CreateRule(name, pattern string, category TagCategory, tags []string, priority int) *TagRule {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ruleCounter++
	rule := &TagRule{
		ID:        fmt.Sprintf("rule_%d", m.ruleCounter),
		Name:      name,
		Pattern:   pattern,
		Category:  category,
		Tags:      tags,
		Priority:  priority,
		Enabled:   true,
		CreatedAt: time.Now(),
	}
	m.rules = append(m.rules, rule)
	return rule
}

// ListRules 列出规则
func (m *Manager) ListRules() []*TagRule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.rules
}

// ========== 统计 ==========

// GetStats 获取统计信息
func (m *Manager) GetStats() *TagStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &TagStats{
		TotalTags:  len(m.tags),
		TotalFiles: len(m.fileIndex),
		ByCategory: make(map[string]int),
	}

	autoCount := 0
	for _, t := range m.tags {
		stats.ByCategory[string(t.Category)]++
		if t.IsAuto {
			autoCount++
		}
	}

	for _, tags := range m.fileTags {
		stats.TotalRelations += len(tags)
	}

	if stats.TotalTags > 0 {
		stats.AutoTagRatio = float64(autoCount) / float64(stats.TotalTags)
	}

	stats.TopTags = m.GetTagCloud(10)
	return stats
}

// Close 关闭管理器
func (m *Manager) Close() error {
	m.cancel()
	return nil
}
