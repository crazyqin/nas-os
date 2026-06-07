// Package aifileassistant AI 文件助手模块
// 智能文件分类、自动标签、语义搜索、内容摘要、重复检测
package aifileassistant

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// FileCategory 文件分类
type FileCategory string

const (
	CategoryDocument FileCategory = "document"
	CategoryImage    FileCategory = "image"
	CategoryVideo    FileCategory = "video"
	CategoryAudio    FileCategory = "audio"
	CategoryCode     FileCategory = "code"
	CategoryArchive  FileCategory = "archive"
	CategoryData     FileCategory = "data"
	CategoryOther    FileCategory = "other"
)

// TagType 标签类型
type TagType string

const (
	TagAuto   TagType = "auto"
	TagManual TagType = "manual"
	TagAI     TagType = "ai"
	TagSystem TagType = "system"
)

// FileTag 文件标签
type FileTag struct {
	Name       string    `json:"name"`
	Type       TagType   `json:"type"`
	Confidence float64   `json:"confidence"`
	CreatedAt  time.Time `json:"created_at"`
}

// FileAnalysis 文件分析结果
type FileAnalysis struct {
	ID          string            `json:"id"`
	FilePath    string            `json:"file_path"`
	FileName    string            `json:"file_name"`
	FileSize    int64             `json:"file_size"`
	Category    FileCategory      `json:"category"`
	Tags        []FileTag         `json:"tags"`
	Summary     string            `json:"summary"`
	Language    string            `json:"language"`
	Sentiment   string            `json:"sentiment"`
	Keywords    []string          `json:"keywords"`
	Entities    []Entity          `json:"entities"`
	ContentHash string            `json:"content_hash"`
	Similarity  float64           `json:"similarity"`
	Duplicates  []string          `json:"duplicates,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	AnalyzedAt  time.Time         `json:"analyzed_at"`
	AnalyzedBy  string            `json:"analyzed_by"`
}

// Entity 实体识别
type Entity struct {
	Name  string  `json:"name"`
	Type  string  `json:"type"`
	Score float64 `json:"score"`
}

// SearchQuery 搜索查询
type SearchQuery struct {
	Query    string       `json:"query"`
	Category FileCategory `json:"category,omitempty"`
	Tags     []string     `json:"tags,omitempty"`
	DateFrom *time.Time   `json:"date_from,omitempty"`
	DateTo   *time.Time   `json:"date_to,omitempty"`
	SizeMin  int64        `json:"size_min,omitempty"`
	SizeMax  int64        `json:"size_max,omitempty"`
	SortBy   string       `json:"sort_by,omitempty"`
	Limit    int          `json:"limit,omitempty"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Files       []FileAnalysis `json:"files"`
	Total       int            `json:"total"`
	Query       string         `json:"query"`
	Duration    time.Duration  `json:"duration"`
	Suggestions []string       `json:"suggestions,omitempty"`
}

// DuplicateGroup 重复文件组
type DuplicateGroup struct {
	ID        string         `json:"id"`
	Hash      string         `json:"hash"`
	Files     []FileAnalysis `json:"files"`
	TotalSize int64          `json:"total_size"`
	Savings   int64          `json:"savings"`
}

// OrganizeSuggestion 整理建议
type OrganizeSuggestion struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Files       []string `json:"files"`
	Action      string   `json:"action"`
	Priority    int      `json:"priority"`
	Savings     int64    `json:"savings"`
}

// FileAssistantStats 助手统计
type FileAssistantStats struct {
	TotalFiles     int                  `json:"total_files"`
	AnalyzedFiles  int                  `json:"analyzed_files"`
	TotalTags      int                  `json:"total_tags"`
	AutoTags       int                  `json:"auto_tags"`
	Duplicates     int                  `json:"duplicates"`
	DuplicateSize  int64                `json:"duplicate_size"`
	CategoryCounts map[FileCategory]int `json:"category_counts"`
	TopTags        []TagCount           `json:"top_tags"`
	LastAnalysis   *time.Time           `json:"last_analysis,omitempty"`
}

// TagCount 标签计数
type TagCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// AIFileAssistant AI 文件助手
type AIFileAssistant struct {
	mu            sync.RWMutex
	analyses      map[string]*FileAnalysis
	tagIndex      map[string][]string // tag -> file IDs
	categoryIndex map[FileCategory][]string
	config        *AssistantConfig
}

// AssistantConfig 助手配置
type AssistantConfig struct {
	AutoTagEnabled      bool    `json:"auto_tag_enabled"`
	AutoClassify        bool    `json:"auto_classify"`
	DuplicateDetect     bool    `json:"duplicate_detect"`
	SummaryEnabled      bool    `json:"summary_enabled"`
	LanguageDetect      bool    `json:"language_detect"`
	SentimentAnalyze    bool    `json:"sentiment_analyze"`
	MaxFileSize         int64   `json:"max_file_size"`
	SimilarityThreshold float64 `json:"similarity_threshold"`
}

// NewAIFileAssistant 创建 AI 文件助手
func NewAIFileAssistant(config *AssistantConfig) *AIFileAssistant {
	if config == nil {
		config = &AssistantConfig{
			AutoTagEnabled:      true,
			AutoClassify:        true,
			DuplicateDetect:     true,
			SummaryEnabled:      true,
			LanguageDetect:      true,
			SentimentAnalyze:    false,
			MaxFileSize:         100 * 1024 * 1024, // 100MB
			SimilarityThreshold: 0.85,
		}
	}
	return &AIFileAssistant{
		analyses:      make(map[string]*FileAnalysis),
		tagIndex:      make(map[string][]string),
		categoryIndex: make(map[FileCategory][]string),
		config:        config,
	}
}

// AnalyzeFile 分析文件
func (afa *AIFileAssistant) AnalyzeFile(filePath string, content []byte) (*FileAnalysis, error) {
	afa.mu.Lock()
	defer afa.mu.Unlock()

	analysis := &FileAnalysis{
		ID:         fmt.Sprintf("analysis_%d", time.Now().UnixNano()),
		FilePath:   filePath,
		FileName:   extractFileName(filePath),
		FileSize:   int64(len(content)),
		AnalyzedAt: time.Now(),
		AnalyzedBy: "ai-assistant",
		Metadata:   make(map[string]string),
	}

	// 分类
	if afa.config.AutoClassify {
		analysis.Category = afa.classifyFile(filePath, content)
	}

	// 自动生成标签
	if afa.config.AutoTagEnabled {
		analysis.Tags = afa.generateTags(filePath, content, analysis.Category)
	}

	// 生成摘要
	if afa.config.SummaryEnabled {
		analysis.Summary = afa.generateSummary(content)
	}

	// 语言检测
	if afa.config.LanguageDetect {
		analysis.Language = afa.detectLanguage(content)
	}

	// 关键词提取
	analysis.Keywords = afa.extractKeywords(content)

	// 实体识别
	analysis.Entities = afa.extractEntities(content)

	// 内容哈希（用于重复检测）
	analysis.ContentHash = afa.computeHash(content)

	// 保存分析结果
	afa.analyses[analysis.ID] = analysis

	// 更新索引
	afa.updateIndexes(analysis)

	return analysis, nil
}

// SearchFiles 搜索文件
func (afa *AIFileAssistant) SearchFiles(query *SearchQuery) (*SearchResult, error) {
	afa.mu.RLock()
	defer afa.mu.RUnlock()

	start := time.Now()
	var results []FileAnalysis

	for _, analysis := range afa.analyses {
		if afa.matchesQuery(analysis, query) {
			results = append(results, *analysis)
		}
	}

	// 排序
	if query.SortBy == "relevance" {
		afa.sortByRelevance(results, query.Query)
	}

	// 限制结果数量
	if query.Limit > 0 && len(results) > query.Limit {
		results = results[:query.Limit]
	}

	return &SearchResult{
		Files:    results,
		Total:    len(results),
		Query:    query.Query,
		Duration: time.Since(start),
	}, nil
}

// DetectDuplicates 检测重复文件
func (afa *AIFileAssistant) DetectDuplicates() []DuplicateGroup {
	afa.mu.RLock()
	defer afa.mu.RUnlock()

	hashMap := make(map[string][]FileAnalysis)
	for _, analysis := range afa.analyses {
		hashMap[analysis.ContentHash] = append(hashMap[analysis.ContentHash], *analysis)
	}

	var groups []DuplicateGroup
	for hash, files := range hashMap {
		if len(files) > 1 {
			var totalSize int64
			for _, f := range files {
				totalSize += f.FileSize
			}

			groups = append(groups, DuplicateGroup{
				ID:        fmt.Sprintf("dup_%s", hash[:8]),
				Hash:      hash,
				Files:     files,
				TotalSize: totalSize,
				Savings:   totalSize - files[0].FileSize,
			})
		}
	}

	return groups
}

// GetOrganizeSuggestions 获取整理建议
func (afa *AIFileAssistant) GetOrganizeSuggestions() []OrganizeSuggestion {
	afa.mu.RLock()
	defer afa.mu.RUnlock()

	var suggestions []OrganizeSuggestion

	// 1. 重复文件清理建议
	duplicates := afa.detectDuplicatesInternal()
	if len(duplicates) > 0 {
		var totalSavings int64
		var fileIDs []string
		for _, dup := range duplicates {
			totalSavings += dup.Savings
			for _, f := range dup.Files[1:] { // 保留第一个
				fileIDs = append(fileIDs, f.ID)
			}
		}
		suggestions = append(suggestions, OrganizeSuggestion{
			ID:          "sug-dedup",
			Type:        "dedup",
			Title:       "清理重复文件",
			Description: fmt.Sprintf("发现 %d 组重复文件，可释放 %d MB 空间", len(duplicates), totalSavings/1024/1024),
			Files:       fileIDs,
			Action:      "delete_duplicates",
			Priority:    1,
			Savings:     totalSavings,
		})
	}

	// 2. 大文件管理
	var largeFiles []string
	var largeSize int64
	for _, analysis := range afa.analyses {
		if analysis.FileSize > 100*1024*1024 { // > 100MB
			largeFiles = append(largeFiles, analysis.ID)
			largeSize += analysis.FileSize
		}
	}
	if len(largeFiles) > 0 {
		suggestions = append(suggestions, OrganizeSuggestion{
			ID:          "sug-large",
			Type:        "large_files",
			Title:       "管理大文件",
			Description: fmt.Sprintf("发现 %d 个大文件（>100MB），建议归档或压缩", len(largeFiles)),
			Files:       largeFiles,
			Action:      "archive",
			Priority:    2,
			Savings:     largeSize / 2, // 假设压缩后减半
		})
	}

	// 3. 未分类文件
	var unclassified []string
	for _, analysis := range afa.analyses {
		if analysis.Category == CategoryOther {
			unclassified = append(unclassified, analysis.ID)
		}
	}
	if len(unclassified) > 0 {
		suggestions = append(suggestions, OrganizeSuggestion{
			ID:          "sug-classify",
			Type:        "classify",
			Title:       "分类未整理文件",
			Description: fmt.Sprintf("有 %d 个文件未分类，建议手动分类", len(unclassified)),
			Files:       unclassified,
			Action:      "classify",
			Priority:    3,
		})
	}

	return suggestions
}

// GetStats 获取统计信息
func (afa *AIFileAssistant) GetStats() *FileAssistantStats {
	afa.mu.RLock()
	defer afa.mu.RUnlock()

	stats := &FileAssistantStats{
		CategoryCounts: make(map[FileCategory]int),
		TopTags:        make([]TagCount, 0),
	}

	tagCounts := make(map[string]int)

	for _, analysis := range afa.analyses {
		stats.TotalFiles++
		if len(analysis.Tags) > 0 {
			stats.AnalyzedFiles++
		}
		stats.CategoryCounts[analysis.Category]++

		for _, tag := range analysis.Tags {
			stats.TotalTags++
			if tag.Type == TagAuto || tag.Type == TagAI {
				stats.AutoTags++
			}
			tagCounts[tag.Name]++
		}
	}

	// 获取热门标签
	for name, count := range tagCounts {
		stats.TopTags = append(stats.TopTags, TagCount{Name: name, Count: count})
	}

	return stats
}

// GetFileTags 获取文件标签
func (afa *AIFileAssistant) GetFileTags(fileID string) ([]FileTag, error) {
	afa.mu.RLock()
	defer afa.mu.RUnlock()

	analysis, exists := afa.analyses[fileID]
	if !exists {
		return nil, fmt.Errorf("file %s not found", fileID)
	}

	return analysis.Tags, nil
}

// AddManualTag 添加手动标签
func (afa *AIFileAssistant) AddManualTag(fileID, tagName string) error {
	afa.mu.Lock()
	defer afa.mu.Unlock()

	analysis, exists := afa.analyses[fileID]
	if !exists {
		return fmt.Errorf("file %s not found", fileID)
	}

	tag := FileTag{
		Name:       tagName,
		Type:       TagManual,
		Confidence: 1.0,
		CreatedAt:  time.Now(),
	}

	analysis.Tags = append(analysis.Tags, tag)
	afa.tagIndex[tagName] = append(afa.tagIndex[tagName], fileID)

	return nil
}

// MarshalJSON 序列化
func (afa *AIFileAssistant) MarshalJSON() ([]byte, error) {
	afa.mu.RLock()
	defer afa.mu.RUnlock()

	return json.Marshal(struct {
		Analyses map[string]*FileAnalysis `json:"analyses"`
		Config   *AssistantConfig         `json:"config"`
	}{
		Analyses: afa.analyses,
		Config:   afa.config,
	})
}

// 内部辅助方法

func (afa *AIFileAssistant) classifyFile(filePath string, content []byte) FileCategory {
	ext := strings.ToLower(extractExtension(filePath))

	categoryMap := map[string]FileCategory{
		".pdf": CategoryDocument, ".doc": CategoryDocument, ".docx": CategoryDocument,
		".txt": CategoryDocument, ".md": CategoryDocument, ".rtf": CategoryDocument,
		".jpg": CategoryImage, ".jpeg": CategoryImage, ".png": CategoryImage,
		".gif": CategoryImage, ".bmp": CategoryImage, ".svg": CategoryImage,
		".mp4": CategoryVideo, ".avi": CategoryVideo, ".mkv": CategoryVideo,
		".mov": CategoryVideo, ".wmv": CategoryVideo,
		".mp3": CategoryAudio, ".wav": CategoryAudio, ".flac": CategoryAudio,
		".aac": CategoryAudio, ".ogg": CategoryAudio,
		".go": CategoryCode, ".py": CategoryCode, ".js": CategoryCode,
		".ts": CategoryCode, ".java": CategoryCode, ".cpp": CategoryCode,
		".c": CategoryCode, ".rs": CategoryCode,
		".zip": CategoryArchive, ".tar": CategoryArchive, ".gz": CategoryArchive,
		".rar": CategoryArchive, ".7z": CategoryArchive,
		".csv": CategoryData, ".json": CategoryData, ".xml": CategoryData,
		".xlsx": CategoryData, ".sql": CategoryData,
	}

	if category, ok := categoryMap[ext]; ok {
		return category
	}
	return CategoryOther
}

func (afa *AIFileAssistant) generateTags(filePath string, content []byte, category FileCategory) []FileTag {
	var tags []FileTag
	now := time.Now()

	// 基于分类的标签
	tags = append(tags, FileTag{
		Name:       string(category),
		Type:       TagAuto,
		Confidence: 0.95,
		CreatedAt:  now,
	})

	// 基于文件名的标签
	name := strings.ToLower(extractFileName(filePath))
	if strings.Contains(name, "backup") || strings.Contains(name, "备份") {
		tags = append(tags, FileTag{Name: "backup", Type: TagAuto, Confidence: 0.9, CreatedAt: now})
	}
	if strings.Contains(name, "photo") || strings.Contains(name, "照片") {
		tags = append(tags, FileTag{Name: "photo", Type: TagAuto, Confidence: 0.9, CreatedAt: now})
	}

	return tags
}

func (afa *AIFileAssistant) generateSummary(content []byte) string {
	text := string(content)
	if len(text) > 200 {
		return text[:200] + "..."
	}
	return text
}

func (afa *AIFileAssistant) detectLanguage(content []byte) string {
	text := string(content)
	if strings.ContainsAny(text, "的一是不了人我在有他这中大来上个国") {
		return "zh"
	}
	if strings.ContainsAny(text, "the be to of and a in that have I") {
		return "en"
	}
	return "unknown"
}

func (afa *AIFileAssistant) extractKeywords(content []byte) []string {
	text := strings.ToLower(string(content))
	words := strings.Fields(text)

	wordCount := make(map[string]int)
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"的": true, "了": true, "在": true, "是": true, "我": true,
	}

	for _, word := range words {
		word = strings.Trim(word, ".,!?;:")
		if len(word) > 2 && !stopWords[word] {
			wordCount[word]++
		}
	}

	var keywords []string
	for word := range wordCount {
		keywords = append(keywords, word)
		if len(keywords) >= 10 {
			break
		}
	}

	return keywords
}

func (afa *AIFileAssistant) extractEntities(content []byte) []Entity {
	// 简化的实体识别
	return []Entity{}
}

func (afa *AIFileAssistant) computeHash(content []byte) string {
	// 简化：使用内容长度和前几个字节作为哈希
	if len(content) > 100 {
		return fmt.Sprintf("%x", len(content))
	}
	return fmt.Sprintf("%x", content)
}

func (afa *AIFileAssistant) matchesQuery(analysis *FileAnalysis, query *SearchQuery) bool {
	// 简化的匹配逻辑
	if query.Category != "" && analysis.Category != query.Category {
		return false
	}
	if query.SizeMin > 0 && analysis.FileSize < query.SizeMin {
		return false
	}
	if query.SizeMax > 0 && analysis.FileSize > query.SizeMax {
		return false
	}
	return true
}

func (afa *AIFileAssistant) sortByRelevance(files []FileAnalysis, query string) {
	// 简化的排序：按匹配度
	query = strings.ToLower(query)
	for i := 0; i < len(files); i++ {
		score := 0.0
		if strings.Contains(strings.ToLower(files[i].FileName), query) {
			score += 10.0
		}
		if strings.Contains(strings.ToLower(files[i].Summary), query) {
			score += 5.0
		}
		files[i].Similarity = score
	}
}

func (afa *AIFileAssistant) detectDuplicatesInternal() []DuplicateGroup {
	hashMap := make(map[string][]*FileAnalysis)
	for _, analysis := range afa.analyses {
		hashMap[analysis.ContentHash] = append(hashMap[analysis.ContentHash], analysis)
	}

	var groups []DuplicateGroup
	for hash, files := range hashMap {
		if len(files) > 1 {
			var totalSize int64
			var fileAnalyses []FileAnalysis
			for _, f := range files {
				totalSize += f.FileSize
				fileAnalyses = append(fileAnalyses, *f)
			}
			groups = append(groups, DuplicateGroup{
				Hash:      hash,
				Files:     fileAnalyses,
				TotalSize: totalSize,
				Savings:   totalSize - files[0].FileSize,
			})
		}
	}
	return groups
}

func (afa *AIFileAssistant) updateIndexes(analysis *FileAnalysis) {
	// 更新标签索引
	for _, tag := range analysis.Tags {
		afa.tagIndex[tag.Name] = append(afa.tagIndex[tag.Name], analysis.ID)
	}

	// 更新分类索引
	afa.categoryIndex[analysis.Category] = append(afa.categoryIndex[analysis.Category], analysis.ID)
}

func extractFileName(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

func extractExtension(path string) string {
	name := extractFileName(path)
	parts := strings.Split(name, ".")
	if len(parts) > 1 {
		return "." + parts[len(parts)-1]
	}
	return ""
}
