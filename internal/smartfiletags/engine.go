package smartfiletags

import (
	"errors"
	"strings"
	"sync"
	"time"
)

// 智能文件标签系统 - 对标群晖DSM 7.3 Drive共享标签
// 支持标签创建、文件打标、智能分类、标签搜索

// 错误定义
var (
	ErrInvalidTagID    = errors.New("invalid tag ID")
	ErrTagNotFound     = errors.New("tag not found")
	ErrInvalidFileID   = errors.New("invalid file ID")
	ErrFileNotFound    = errors.New("file not found")
	ErrTagExists       = errors.New("tag already exists")
	ErrMaxTagsExceeded = errors.New("maximum tags exceeded")
	ErrUnauthorized    = errors.New("unauthorized")
)

// TagConfig 标签配置
type TagConfig struct {
	MaxTagsPerFile     int    `json:"max_tags_per_file"`
	MaxTagsPerUser     int    `json:"max_tags_per_user"`
	AllowUserCreate    bool   `json:"allow_user_create"`
	AutoTagEnabled     bool   `json:"auto_tag_enabled"`
	SuggestionEnabled  bool   `json:"suggestion_enabled"`
	MaxSuggestions     int    `json:"max_suggestions"`
	ColorPalette       string `json:"color_palette"`
}

// DefaultTagConfig 默认配置
func DefaultTagConfig() *TagConfig {
	return &TagConfig{
		MaxTagsPerFile:    20,
		MaxTagsPerUser:    500,
		AllowUserCreate:   true,
		AutoTagEnabled:    true,
		SuggestionEnabled: true,
		MaxSuggestions:    10,
		ColorPalette:      "default",
	}
}

// Tag 标签
type Tag struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Color       string    `json:"color"`
	Icon        string    `json:"icon,omitempty"`
	Description string    `json:"description,omitempty"`
	Category    string    `json:"category,omitempty"`
	IsSystem    bool      `json:"is_system"`
	IsPublic    bool      `json:"is_public"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	FileCount   int       `json:"file_count"`
	UsageCount  int       `json:"usage_count"`
}

// FileTag 文件标签关联
type FileTag struct {
	FileID    string    `json:"file_id"`
	TagID     string    `json:"tag_id"`
	TagName   string    `json:"tag_name"`
	AppliedBy string    `json:"applied_by"`
	AppliedAt time.Time `json:"applied_at"`
	IsAuto    bool      `json:"is_auto"`
	Confidence float64  `json:"confidence,omitempty"`
}

// TagSuggestion 标签建议
type TagSuggestion struct {
	Tag        *Tag    `json:"tag"`
	Score      float64 `json:"score"`
	Reason     string  `json:"reason"`
	IsNew      bool    `json:"is_new"`
}

// TagStats 标签统计
type TagStats struct {
	TotalTags      int            `json:"total_tags"`
	TotalFiles     int            `json:"total_files"`
	TotalRelations int            `json:"total_relations"`
	TopTags        []*TagCount    `json:"top_tags"`
	CategoryStats  map[string]int `json:"category_stats"`
	RecentTags     []*Tag         `json:"recent_tags"`
}

// TagCount 标签计数
type TagCount struct {
	Tag   *Tag `json:"tag"`
	Count int  `json:"count"`
}

// TagEngine 标签引擎
type TagEngine struct {
	mu          sync.RWMutex
	config      *TagConfig
	tags        map[string]*Tag
	fileTags    map[string][]*FileTag   // file_id -> tags
	tagFiles    map[string][]string     // tag_id -> file_ids
	userTags    map[string][]string     // user_id -> tag_ids
	nameIndex   map[string]string       // tag_name -> tag_id (lowercase)
	categories  map[string][]string     // category -> tag_ids
	running     bool
	stopCh      chan struct{}
	stats       *TagStats
	autoTagger  *AutoTagger
}

// AutoTagger 自动标签器
type AutoTagger struct {
	rules      []*AutoTagRule
	keywords   map[string][]string // tag_name -> keywords
	patterns   map[string]string   // pattern -> tag_name
}

// AutoTagRule 自动标签规则
type AutoTagRule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	TagID       string   `json:"tag_id"`
	TagName     string   `json:"tag_name"`
	Conditions  []string `json:"conditions"`
	Patterns    []string `json:"patterns"`
	Confidence  float64  `json:"confidence"`
	Enabled     bool     `json:"enabled"`
	CreatedBy   string   `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// NewTagEngine 创建标签引擎
func NewTagEngine(config *TagConfig) *TagEngine {
	if config == nil {
		config = DefaultTagConfig()
	}
	return &TagEngine{
		config:     config,
		tags:       make(map[string]*Tag),
		fileTags:   make(map[string][]*FileTag),
		tagFiles:   make(map[string][]string),
		userTags:   make(map[string][]string),
		nameIndex:  make(map[string]string),
		categories: make(map[string][]string),
		stats:      &TagStats{},
		autoTagger: &AutoTagger{
			rules:    make([]*AutoTagRule, 0),
			keywords: make(map[string][]string),
			patterns: make(map[string]string),
		},
	}
}

// Start 启动引擎
func (te *TagEngine) Start() error {
	te.mu.Lock()
	defer te.mu.Unlock()

	if te.running {
		return nil
	}

	te.running = true
	te.stopCh = make(chan struct{})
	return nil
}

// Stop 停止引擎
func (te *TagEngine) Stop() error {
	te.mu.Lock()
	defer te.mu.Unlock()

	if !te.running {
		return nil
	}

	close(te.stopCh)
	te.running = false
	return nil
}

// CreateTag 创建标签
func (te *TagEngine) CreateTag(tag *Tag) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	if tag.ID == "" {
		return ErrInvalidTagID
	}

	// 检查名称是否重复
	if existingID, exists := te.nameIndex[tag.Name]; exists {
		if existingID != tag.ID {
			return ErrTagExists
		}
	}

	tag.CreatedAt = time.Now()
	tag.UpdatedAt = time.Now()
	te.tags[tag.ID] = tag

	// 更新索引
	te.nameIndex[tag.Name] = tag.ID
	if tag.Category != "" {
		te.categories[tag.Category] = append(te.categories[tag.Category], tag.ID)
	}

	if tag.CreatedBy != "" {
		te.userTags[tag.CreatedBy] = append(te.userTags[tag.CreatedBy], tag.ID)
	}

	te.stats.TotalTags++
	return nil
}

// UpdateTag 更新标签
func (te *TagEngine) UpdateTag(tag *Tag) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	existing, exists := te.tags[tag.ID]
	if !exists {
		return ErrTagNotFound
	}

	// 更新名称索引
	if existing.Name != tag.Name {
		delete(te.nameIndex, existing.Name)
		te.nameIndex[tag.Name] = tag.ID
	}

	tag.UpdatedAt = time.Now()
	tag.CreatedAt = existing.CreatedAt
	tag.FileCount = existing.FileCount
	tag.UsageCount = existing.UsageCount
	te.tags[tag.ID] = tag

	return nil
}

// DeleteTag 删除标签
func (te *TagEngine) DeleteTag(tagID string) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	tag, exists := te.tags[tagID]
	if !exists {
		return ErrTagNotFound
	}

	// 删除所有文件关联
	for _, fileID := range te.tagFiles[tagID] {
		tags := te.fileTags[fileID]
		for i, ft := range tags {
			if ft.TagID == tagID {
				te.fileTags[fileID] = append(tags[:i], tags[i+1:]...)
				break
			}
		}
	}

	// 清理索引
	delete(te.nameIndex, tag.Name)
	delete(te.tagFiles, tagID)

	if tag.Category != "" {
		ids := te.categories[tag.Category]
		for i, id := range ids {
			if id == tagID {
				te.categories[tag.Category] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
	}

	delete(te.tags, tagID)
	te.stats.TotalTags--
	return nil
}

// GetTag 获取标签
func (te *TagEngine) GetTag(tagID string) (*Tag, error) {
	te.mu.RLock()
	defer te.mu.RUnlock()

	tag, exists := te.tags[tagID]
	if !exists {
		return nil, ErrTagNotFound
	}

	return tag, nil
}

// GetTagByName 按名称获取标签
func (te *TagEngine) GetTagByName(name string) (*Tag, error) {
	te.mu.RLock()
	defer te.mu.RUnlock()

	tagID, exists := te.nameIndex[name]
	if !exists {
		return nil, ErrTagNotFound
	}

	return te.tags[tagID], nil
}

// ListTags 列出所有标签
func (te *TagEngine) ListTags(category string) []*Tag {
	te.mu.RLock()
	defer te.mu.RUnlock()

	var tags []*Tag

	if category != "" {
		tagIDs := te.categories[category]
		for _, tagID := range tagIDs {
			if tag, exists := te.tags[tagID]; exists {
				tags = append(tags, tag)
			}
		}
	} else {
		for _, tag := range te.tags {
			tags = append(tags, tag)
		}
	}

	return tags
}

// ApplyTagToFile 给文件打标签
func (te *TagEngine) ApplyTagToFile(fileID, tagID, userID string, isAuto bool, confidence float64) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	if fileID == "" {
		return ErrInvalidFileID
	}

	tag, exists := te.tags[tagID]
	if !exists {
		return ErrTagNotFound
	}

	// 检查是否已存在
	for _, ft := range te.fileTags[fileID] {
		if ft.TagID == tagID {
			return nil // 已存在，跳过
		}
	}

	// 检查限制
	if len(te.fileTags[fileID]) >= te.config.MaxTagsPerFile {
		return ErrMaxTagsExceeded
	}

	fileTag := &FileTag{
		FileID:     fileID,
		TagID:      tagID,
		TagName:    tag.Name,
		AppliedBy:  userID,
		AppliedAt:  time.Now(),
		IsAuto:     isAuto,
		Confidence: confidence,
	}

	te.fileTags[fileID] = append(te.fileTags[fileID], fileTag)
	te.tagFiles[tagID] = append(te.tagFiles[tagID], fileID)

	tag.FileCount++
	tag.UsageCount++
	tag.UpdatedAt = time.Now()

	te.stats.TotalRelations++
	return nil
}

// RemoveTagFromFile 移除文件标签
func (te *TagEngine) RemoveTagFromFile(fileID, tagID string) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	tags := te.fileTags[fileID]
	found := false
	for i, ft := range tags {
		if ft.TagID == tagID {
			te.fileTags[fileID] = append(tags[:i], tags[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return ErrTagNotFound
	}

	// 从 tagFiles 中移除
	fileIDs := te.tagFiles[tagID]
	for i, id := range fileIDs {
		if id == fileID {
			te.tagFiles[tagID] = append(fileIDs[:i], fileIDs[i+1:]...)
			break
		}
	}

	if tag, exists := te.tags[tagID]; exists {
		tag.FileCount--
		tag.UpdatedAt = time.Now()
	}

	te.stats.TotalRelations--
	return nil
}

// GetFileTags 获取文件的所有标签
func (te *TagEngine) GetFileTags(fileID string) []*FileTag {
	te.mu.RLock()
	defer te.mu.RUnlock()

	return te.fileTags[fileID]
}

// GetTagFiles 获取标签下的所有文件
func (te *TagEngine) GetTagFiles(tagID string) []string {
	te.mu.RLock()
	defer te.mu.RUnlock()

	return te.tagFiles[tagID]
}

// SearchByTag 按标签搜索文件
func (te *TagEngine) SearchByTag(tagNames []string) []string {
	te.mu.RLock()
	defer te.mu.RUnlock()

	if len(tagNames) == 0 {
		return nil
	}

	// 获取第一个标签的文件列表
	firstTagID, exists := te.nameIndex[tagNames[0]]
	if !exists {
		return nil
	}

	result := make(map[string]bool)
	for _, fileID := range te.tagFiles[firstTagID] {
		result[fileID] = true
	}

	// 与其他标签取交集
	for _, tagName := range tagNames[1:] {
		tagID, exists := te.nameIndex[tagName]
		if !exists {
			return nil
		}

		newResult := make(map[string]bool)
		for _, fileID := range te.tagFiles[tagID] {
			if result[fileID] {
				newResult[fileID] = true
			}
		}
		result = newResult
	}

	files := make([]string, 0, len(result))
	for fileID := range result {
		files = append(files, fileID)
	}

	return files
}

// SuggestTags 建议标签
func (te *TagEngine) SuggestTags(fileID string, fileName string, content string) []*TagSuggestion {
	te.mu.RLock()
	defer te.mu.RUnlock()

	if !te.config.SuggestionEnabled {
		return nil
	}

	var suggestions []*TagSuggestion

	// 基于文件名匹配
	for _, tag := range te.tags {
		score := 0.0
		reason := ""

		// 关键词匹配
		if keywords, exists := te.autoTagger.keywords[tag.Name]; exists {
			for _, keyword := range keywords {
				if containsIgnoreCase(fileName, keyword) || containsIgnoreCase(content, keyword) {
					score += 0.3
					reason = "keyword_match"
				}
			}
		}

		// 历史模式匹配
		if score > 0 {
			suggestions = append(suggestions, &TagSuggestion{
				Tag:    tag,
				Score:  score,
				Reason: reason,
				IsNew:  false,
			})
		}
	}

	// 限制建议数量
	if len(suggestions) > te.config.MaxSuggestions {
		suggestions = suggestions[:te.config.MaxSuggestions]
	}

	return suggestions
}

// AutoTag 自动标签
func (te *TagEngine) AutoTag(fileID, fileName, content string) ([]*FileTag, error) {
	te.mu.Lock()
	defer te.mu.Unlock()

	if !te.config.AutoTagEnabled {
		return nil, nil
	}

	var applied []*FileTag

	for _, rule := range te.autoTagger.rules {
		if !rule.Enabled {
			continue
		}

		matched := false
		for _, pattern := range rule.Patterns {
			if containsIgnoreCase(fileName, pattern) || containsIgnoreCase(content, pattern) {
				matched = true
				break
			}
		}

		if matched {
			fileTag := &FileTag{
				FileID:     fileID,
				TagID:      rule.TagID,
				TagName:    rule.TagName,
				AppliedBy:  "system",
				AppliedAt:  time.Now(),
				IsAuto:     true,
				Confidence: rule.Confidence,
			}

			te.fileTags[fileID] = append(te.fileTags[fileID], fileTag)
			te.tagFiles[rule.TagID] = append(te.tagFiles[rule.TagID], fileID)

			if tag, exists := te.tags[rule.TagID]; exists {
				tag.FileCount++
				tag.UsageCount++
			}

			applied = append(applied, fileTag)
		}
	}

	return applied, nil
}

// AddAutoTagRule 添加自动标签规则
func (te *TagEngine) AddAutoTagRule(rule *AutoTagRule) {
	te.mu.Lock()
	defer te.mu.Unlock()

	rule.CreatedAt = time.Now()
	te.autoTagger.rules = append(te.autoTagger.rules, rule)

	// 更新关键词索引
	if rule.TagName != "" {
		te.autoTagger.keywords[rule.TagName] = append(
			te.autoTagger.keywords[rule.TagName],
			rule.Patterns...,
		)
	}
}

// GetStats 获取统计信息
func (te *TagEngine) GetStats() *TagStats {
	te.mu.RLock()
	defer te.mu.RUnlock()

	stats := *te.stats
	stats.TotalTags = len(te.tags)
	stats.TotalFiles = len(te.fileTags)

	// 计算 TopTags
	type tagCount struct {
		tag   *Tag
		count int
	}

	var tagCounts []tagCount
	for _, tag := range te.tags {
		tagCounts = append(tagCounts, tagCount{tag: tag, count: tag.FileCount})
	}

	// 简单排序
	for i := 0; i < len(tagCounts); i++ {
		for j := i + 1; j < len(tagCounts); j++ {
			if tagCounts[j].count > tagCounts[i].count {
				tagCounts[i], tagCounts[j] = tagCounts[j], tagCounts[i]
			}
		}
	}

	// 取前10
	topN := 10
	if len(tagCounts) < topN {
		topN = len(tagCounts)
	}
	stats.TopTags = make([]*TagCount, topN)
	for i := 0; i < topN; i++ {
		stats.TopTags[i] = &TagCount{
			Tag:   tagCounts[i].tag,
			Count: tagCounts[i].count,
		}
	}

	// 分类统计
	stats.CategoryStats = make(map[string]int)
	for category, tagIDs := range te.categories {
		stats.CategoryStats[category] = len(tagIDs)
	}

	// 最近标签
	var recentTags []*Tag
	for _, tag := range te.tags {
		recentTags = append(recentTags, tag)
	}
	// 按更新时间排序
	for i := 0; i < len(recentTags); i++ {
		for j := i + 1; j < len(recentTags); j++ {
			if recentTags[j].UpdatedAt.After(recentTags[i].UpdatedAt) {
				recentTags[i], recentTags[j] = recentTags[j], recentTags[i]
			}
		}
	}
	recentN := 10
	if len(recentTags) < recentN {
		recentN = len(recentTags)
	}
	stats.RecentTags = recentTags[:recentN]

	return &stats
}

// containsIgnoreCase 忽略大小写包含检查
func containsIgnoreCase(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) == 0 {
		return false
	}
	lowerS := strings.ToLower(s)
	lowerSubstr := strings.ToLower(substr)
	return strings.Contains(lowerS, lowerSubstr)
}
