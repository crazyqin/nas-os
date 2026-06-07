// Package smarttag 智能标签系统 - AI自动分类/标记文件
package smarttag

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// TagType 标签类型
type TagType string

const (
	TagAuto   TagType = "auto"   // AI自动生成
	TagManual TagType = "manual" // 手动添加
	TagSystem TagType = "system" // 系统标签
)

// Tag 标签
type Tag struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	Icon      string    `json:"icon,omitempty"`
	Type      TagType   `json:"type"`
	Count     int       `json:"count"`
	CreatedAt time.Time `json:"created_at"`
}

// FileTag 文件标签关联
type FileTag struct {
	FilePath  string    `json:"file_path"`
	TagIDs    []string  `json:"tag_ids"`
	AutoScore float64   `json:"auto_score,omitempty"` // AI置信度
	TaggedAt  time.Time `json:"tagged_at"`
	TaggedBy  string    `json:"tagged_by"` // "ai" or user id
}

// ClassificationRule 分类规则
type ClassificationRule struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Patterns   []string `json:"patterns"`   // 文件名/路径模式
	Extensions []string `json:"extensions"` // 扩展名
	TagIDs     []string `json:"tag_ids"`
	Priority   int      `json:"priority"`
	Enabled    bool     `json:"enabled"`
}

// AISuggestion AI建议
type AISuggestion struct {
	FilePath string   `json:"file_path"`
	TagIDs   []string `json:"tag_ids"`
	Score    float64  `json:"score"`
	Reason   string   `json:"reason"`
}

// Config 配置
type Config struct {
	AutoTag       bool    `json:"auto_tag"`
	AIEnabled     bool    `json:"ai_enabled"`
	ScanInterval  int     `json:"scan_interval_hours"`
	MinConfidence float64 `json:"min_confidence"`
	MaxTags       int     `json:"max_tags_per_file"`
}

// Manager 管理器
type Manager struct {
	mu       sync.RWMutex
	tags     map[string]*Tag
	fileTags map[string]*FileTag
	rules    []*ClassificationRule
	config   *Config
	dataFile string
}

var (
	ErrTagNotFound   = errors.New("tag not found")
	ErrFileNotTagged = errors.New("file not tagged")
	ErrDuplicateTag  = errors.New("tag already exists")
)

// NewManager 创建管理器
func NewManager(dataFile string) *Manager {
	return &Manager{
		tags:     make(map[string]*Tag),
		fileTags: make(map[string]*FileTag),
		config: &Config{
			AutoTag:       true,
			AIEnabled:     true,
			MinConfidence: 0.7,
			MaxTags:       5,
		},
		dataFile: dataFile,
	}
}

// Initialize 初始化
func (m *Manager) Initialize() error {
	m.loadDefaultTags()
	m.loadDefaultRules()
	return m.load()
}

func (m *Manager) loadDefaultTags() {
	defaults := []Tag{
		{ID: "photo", Name: "照片", Color: "#4CAF50", Icon: "📷", Type: TagSystem},
		{ID: "video", Name: "视频", Color: "#FF5722", Icon: "🎬", Type: TagSystem},
		{ID: "document", Name: "文档", Color: "#2196F3", Icon: "📄", Type: TagSystem},
		{ID: "music", Name: "音乐", Color: "#9C27B0", Icon: "🎵", Type: TagSystem},
		{ID: "archive", Name: "压缩包", Color: "#795548", Icon: "📦", Type: TagSystem},
		{ID: "code", Name: "代码", Color: "#607D8B", Icon: "💻", Type: TagSystem},
		{ID: "important", Name: "重要", Color: "#F44336", Icon: "⭐", Type: TagSystem},
		{ID: "work", Name: "工作", Color: "#FF9800", Icon: "💼", Type: TagSystem},
		{ID: "personal", Name: "个人", Color: "#E91E63", Icon: "👤", Type: TagSystem},
		{ID: "backup", Name: "备份", Color: "#00BCD4", Icon: "💾", Type: TagSystem},
	}
	for i := range defaults {
		defaults[i].CreatedAt = time.Now()
		m.tags[defaults[i].ID] = &defaults[i]
	}
}

func (m *Manager) loadDefaultRules() {
	m.rules = []*ClassificationRule{
		{ID: "r-photo", Name: "照片文件", Extensions: []string{".jpg", ".jpeg", ".png", ".gif", ".heic", ".raw"}, TagIDs: []string{"photo"}, Priority: 10, Enabled: true},
		{ID: "r-video", Name: "视频文件", Extensions: []string{".mp4", ".mkv", ".avi", ".mov", ".wmv"}, TagIDs: []string{"video"}, Priority: 10, Enabled: true},
		{ID: "r-doc", Name: "文档文件", Extensions: []string{".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx"}, TagIDs: []string{"document"}, Priority: 10, Enabled: true},
		{ID: "r-music", Name: "音乐文件", Extensions: []string{".mp3", ".flac", ".wav", ".aac", ".ogg"}, TagIDs: []string{"music"}, Priority: 10, Enabled: true},
		{ID: "r-code", Name: "代码文件", Extensions: []string{".go", ".py", ".js", ".ts", ".java", ".cpp", ".rs"}, TagIDs: []string{"code"}, Priority: 10, Enabled: true},
	}
}

// CreateTag 创建标签
func (m *Manager) CreateTag(tag *Tag) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.tags[tag.ID]; exists {
		return ErrDuplicateTag
	}
	tag.Type = TagManual
	tag.CreatedAt = time.Now()
	m.tags[tag.ID] = tag
	return m.save()
}

// DeleteTag 删除标签
func (m *Manager) DeleteTag(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tags[id]; !ok {
		return ErrTagNotFound
	}
	delete(m.tags, id)
	return m.save()
}

// GetTag 获取标签
func (m *Manager) GetTag(id string) (*Tag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tag, ok := m.tags[id]
	if !ok {
		return nil, ErrTagNotFound
	}
	return tag, nil
}

// ListTags 列出标签
func (m *Manager) ListTags(tagType TagType) []*Tag {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Tag
	for _, t := range m.tags {
		if tagType == "" || t.Type == tagType {
			result = append(result, t)
		}
	}
	return result
}

// TagFile 标记文件
func (m *Manager) TagFile(filePath string, tagIDs []string, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ft, exists := m.fileTags[filePath]
	if !exists {
		ft = &FileTag{FilePath: filePath, TaggedAt: time.Now(), TaggedBy: userID}
		m.fileTags[filePath] = ft
	}
	ft.TagIDs = append(ft.TagIDs, tagIDs...)
	ft.TagIDs = uniqueStrings(ft.TagIDs)

	for _, id := range tagIDs {
		if tag, ok := m.tags[id]; ok {
			tag.Count++
		}
	}
	return m.save()
}

// UntagFile 移除文件标签
func (m *Manager) UntagFile(filePath string, tagIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ft, ok := m.fileTags[filePath]
	if !ok {
		return ErrFileNotTagged
	}
	removeSet := make(map[string]bool)
	for _, id := range tagIDs {
		removeSet[id] = true
	}
	var kept []string
	for _, id := range ft.TagIDs {
		if !removeSet[id] {
			kept = append(kept, id)
		}
	}
	ft.TagIDs = kept
	return m.save()
}

// GetFileTags 获取文件标签
func (m *Manager) GetFileTags(filePath string) *FileTag {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.fileTags[filePath]
}

// ClassifyFile AI分类文件
func (m *Manager) ClassifyFile(filePath string) *AISuggestion {
	ext := getExtension(filePath)
	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}
		for _, e := range rule.Extensions {
			if e == ext {
				return &AISuggestion{
					FilePath: filePath,
					TagIDs:   rule.TagIDs,
					Score:    0.95,
					Reason:   fmt.Sprintf("匹配规则: %s", rule.Name),
				}
			}
		}
	}
	return &AISuggestion{FilePath: filePath, TagIDs: []string{}, Score: 0, Reason: "无法分类"}
}

// BatchClassify 批量分类
func (m *Manager) BatchClassify(filePaths []string) []*AISuggestion {
	var results []*AISuggestion
	for _, fp := range filePaths {
		results = append(results, m.ClassifyFile(fp))
	}
	return results
}

// SearchByTag 按标签搜索
func (m *Manager) SearchByTag(tagIDs []string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tagSet := make(map[string]bool)
	for _, id := range tagIDs {
		tagSet[id] = true
	}
	var result []string
	for path, ft := range m.fileTags {
		for _, id := range ft.TagIDs {
			if tagSet[id] {
				result = append(result, path)
				break
			}
		}
	}
	return result
}

// GetStats 获取统计
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	autoCount := 0
	for _, t := range m.tags {
		if t.Type == TagAuto {
			autoCount++
		}
	}
	return map[string]interface{}{
		"total_tags":      len(m.tags),
		"auto_tags":       autoCount,
		"total_file_tags": len(m.fileTags),
		"total_rules":     len(m.rules),
	}
}

func (m *Manager) load() error {
	if m.dataFile == "" {
		return nil
	}
	data, err := os.ReadFile(m.dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &m.fileTags)
}

func (m *Manager) save() error {
	if m.dataFile == "" {
		return nil
	}
	data, err := json.MarshalIndent(m.fileTags, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.dataFile, data, 0644)
}

func getExtension(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i:]
		}
		if path[i] == '/' {
			break
		}
	}
	return ""
}

func uniqueStrings(s []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}
