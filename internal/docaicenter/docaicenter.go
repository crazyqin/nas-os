// Package docaicenter 提供文档AI处理中心
// OCR识别、文档转换、摘要生成、关键词提取、智能分类
// 对标群晖文档查看器AI增强 + TrueNAS文件处理
package docaicenter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ========== 常量 ==========

const (
	Version           = "1.0.0"
	MaxDocuments      = 50000
	MaxSummaryLength  = 500
	MaxKeywords       = 20
	MaxOCRPages       = 1000
	ProcessTimeout    = 5 * time.Minute
	BatchProcessSize  = 10
	MinTextLength     = 10
	ConfidenceThreshold = 0.7
)

// ========== 类型定义 ==========

// DocumentType 文档类型
type DocumentType string

const (
	DocTypePDF      DocumentType = "pdf"
	DocTypeWord     DocumentType = "word"
	DocTypeExcel    DocumentType = "excel"
	DocTypePowerPoint DocumentType = "powerpoint"
	DocTypeText     DocumentType = "text"
	DocTypeMarkdown DocumentType = "markdown"
	DocTypeHTML     DocumentType = "html"
	DocTypeImage    DocumentType = "image"
	DocTypeEPUB     DocumentType = "epub"
	DocTypeUnknown  DocumentType = "unknown"
)

// ProcessingStatus 处理状态
type ProcessingStatus string

const (
	StatusPending    ProcessingStatus = "pending"
	StatusProcessing ProcessingStatus = "processing"
	StatusCompleted  ProcessingStatus = "completed"
	StatusFailed     ProcessingStatus = "failed"
)

// Document 文档记录
type Document struct {
	ID          string           `json:"id"`
	FilePath    string           `json:"file_path"`
	FileName    string           `json:"file_name"`
	FileSize    int64            `json:"file_size"`
	Type        DocumentType     `json:"type"`
	Hash        string           `json:"hash"`
	Title       string           `json:"title"`
	Author      string           `json:"author"`
	Language    string           `json:"language"`
	PageCount   int              `json:"page_count"`
	WordCount   int              `json:"word_count"`
	Summary     string           `json:"summary"`
	Keywords    []string         `json:"keywords"`
	Categories  []string         `json:"categories"`
	Status      ProcessingStatus `json:"status"`
	Error       string           `json:"error,omitempty"`
	ProcessedAt *time.Time       `json:"processed_at,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// OCRResult OCR结果
type OCRResult struct {
	PageNum    int     `json:"page_num"`
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Language   string  `json:"language"`
	BoundingBox []int  `json:"bounding_box,omitempty"`
}

// SearchResult 文档搜索结果
type SearchResult struct {
	Document   *Document `json:"document"`
	Score      float64   `json:"score"`
	MatchType  string    `json:"match_type"`
	Highlight  string    `json:"highlight"`
}

// DocStats 文档统计
type DocStats struct {
	TotalDocs      int            `json:"total_docs"`
	ProcessedDocs  int            `json:"processed_docs"`
	FailedDocs     int            `json:"failed_docs"`
	TotalPages     int            `json:"total_pages"`
	TotalWords     int            `json:"total_words"`
	ByType         map[string]int `json:"by_type"`
	ByLanguage     map[string]int `json:"by_language"`
	ProcessingRate float64        `json:"processing_rate"`
}

// ConversionTask 转换任务
type ConversionTask struct {
	ID          string    `json:"id"`
	SourcePath  string    `json:"source_path"`
	TargetPath  string    `json:"target_path"`
	TargetType  string    `json:"target_type"`
	Status      ProcessingStatus `json:"status"`
	Progress    int       `json:"progress"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// ========== 核心管理器 ==========

// Manager 文档AI管理器
type Manager struct {
	mu           sync.RWMutex
	documents    map[string]*Document
	conversions  map[string]*ConversionTask
	dataDir      string
	ctx          context.Context
	cancel       context.CancelFunc
	docCounter   int64
	convCounter  int64
}

// NewManager 创建管理器
func NewManager(dataDir string) (*Manager, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		documents:   make(map[string]*Document),
		conversions: make(map[string]*ConversionTask),
		dataDir:     dataDir,
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

// ========== 文档处理 ==========

// ProcessDocument 处理文档
func (m *Manager) ProcessDocument(ctx context.Context, filePath string) (*Document, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("文件不存在: %w", err)
	}

	hash, err := m.computeHash(filePath)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	// 检查重复
	for _, doc := range m.documents {
		if doc.Hash == hash {
			m.mu.Unlock()
			return doc, nil
		}
	}

	m.docCounter++
	doc := &Document{
		ID:        fmt.Sprintf("doc_%d", m.docCounter),
		FilePath:  filePath,
		FileName:  filepath.Base(filePath),
		FileSize:  info.Size(),
		Type:      m.detectDocType(filePath),
		Hash:      hash,
		Status:    StatusProcessing,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.documents[doc.ID] = doc
	m.mu.Unlock()

	// 异步处理
	go m.processDocAsync(doc)

	return doc, nil
}

// processDocAsync 异步处理文档
func (m *Manager) processDocAsync(doc *Document) {
	ctx, cancel := context.WithTimeout(context.Background(), ProcessTimeout)
	defer cancel()

	var err error
	defer func() {
		m.mu.Lock()
		if err != nil {
			doc.Status = StatusFailed
			doc.Error = err.Error()
		} else {
			now := time.Now()
			doc.Status = StatusCompleted
			doc.ProcessedAt = &now
		}
		doc.UpdatedAt = time.Now()
		m.mu.Unlock()
	}()

	// 提取文本
	text, err := m.extractText(ctx, doc.FilePath, doc.Type)
	if err != nil {
		return
	}

	doc.WordCount = len(strings.Fields(text))
	doc.PageCount = m.estimatePages(text, doc.Type)

	// 生成摘要
	doc.Summary = m.generateSummary(text)

	// 提取关键词
	doc.Keywords = m.extractKeywords(text)

	// 分类
	doc.Categories = m.classifyDocument(text, doc.Type)

	// 检测语言
	doc.Language = m.detectLanguage(text)
}

// extractText 提取文本
func (m *Manager) extractText(ctx context.Context, filePath string, docType DocumentType) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	text := string(data)
	if len(text) < MinTextLength {
		return "", errors.New("文本内容过短")
	}
	return text, nil
}

// generateSummary 生成摘要
func (m *Manager) generateSummary(text string) string {
	sentences := strings.Split(text, "。")
	if len(sentences) == 0 {
		return ""
	}

	summary := ""
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if len(s) < 5 {
			continue
		}
		if len(summary)+len(s) > MaxSummaryLength {
			break
		}
		summary += s + "。"
	}
	return strings.TrimSpace(summary)
}

// extractKeywords 提取关键词
func (m *Manager) extractKeywords(text string) []string {
	words := strings.Fields(text)
	freq := make(map[string]int)

	stopWords := map[string]bool{
		"的": true, "了": true, "在": true, "是": true, "我": true,
		"有": true, "和": true, "就": true, "不": true, "人": true,
		"都": true, "一": true, "一个": true, "上": true, "也": true,
		"很": true, "到": true, "说": true, "要": true, "去": true,
		"你": true, "会": true, "着": true, "没有": true, "看": true,
		"好": true, "自己": true, "这": true, "the": true, "a": true,
		"an": true, "is": true, "are": true, "was": true, "were": true,
		"be": true, "been": true, "being": true, "have": true, "has": true,
		"had": true, "do": true, "does": true, "did": true, "will": true,
	}

	for _, w := range words {
		w = strings.ToLower(strings.Trim(w, ".,;:!?\"'()[]{}"))
		if len(w) < 2 || stopWords[w] {
			continue
		}
		freq[w]++
	}

	type wordFreq struct {
		word  string
		count int
	}
	wfs := make([]wordFreq, 0, len(freq))
	for w, c := range freq {
		wfs = append(wfs, wordFreq{w, c})
	}
	sort.Slice(wfs, func(i, j int) bool {
		return wfs[i].count > wfs[j].count
	})

	keywords := make([]string, 0, MaxKeywords)
	for i := 0; i < len(wfs) && i < MaxKeywords; i++ {
		keywords = append(keywords, wfs[i].word)
	}
	return keywords
}

// classifyDocument 分类文档
func (m *Manager) classifyDocument(text string, docType DocumentType) []string {
	categories := make([]string, 0)
	textLower := strings.ToLower(text)

	if docType == DocTypeCode || strings.Contains(text, "package ") || strings.Contains(text, "func ") {
		categories = append(categories, "代码")
	}
	if strings.Contains(textLower, "report") || strings.Contains(text, "报告") {
		categories = append(categories, "报告")
	}
	if strings.Contains(textLower, "manual") || strings.Contains(text, "手册") || strings.Contains(text, "指南") {
		categories = append(categories, "文档")
	}
	if strings.Contains(text, "论文") || strings.Contains(textLower, "abstract") {
		categories = append(categories, "学术")
	}
	if len(categories) == 0 {
		categories = append(categories, "通用")
	}
	return categories
}

// detectLanguage 检测语言
func (m *Manager) detectLanguage(text string) string {
	chineseCount := 0
	englishCount := 0
	for _, r := range text {
		if r >= 0x4e00 && r <= 0x9fff {
			chineseCount++
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			englishCount++
		}
	}
	if chineseCount > englishCount {
		return "zh-CN"
	}
	return "en"
}

// estimatePages 估算页数
func (m *Manager) estimatePages(text string, docType DocumentType) int {
	lines := strings.Count(text, "\n") + 1
	switch docType {
	case DocTypePDF:
		return (lines + 49) / 50
	default:
		return (lines + 59) / 60
	}
}

// detectDocType 检测文档类型
func (m *Manager) detectDocType(filePath string) DocumentType {
	ext := strings.ToLower(filepath.Ext(filePath))
	typeMap := map[string]DocumentType{
		".pdf": DocTypePDF, ".doc": DocTypeWord, ".docx": DocTypeWord,
		".xls": DocTypeExcel, ".xlsx": DocTypeExcel,
		".ppt": DocTypePowerPoint, ".pptx": DocTypePowerPoint,
		".txt": DocTypeText, ".md": DocTypeMarkdown,
		".html": DocTypeHTML, ".htm": DocTypeHTML,
		".epub": DocTypeEPUB,
		".jpg": DocTypeImage, ".png": DocTypeImage, ".bmp": DocTypeImage,
	}
	if t, ok := typeMap[ext]; ok {
		return t
	}
	return DocTypeUnknown
}

// computeHash 计算文件哈希
func (m *Manager) computeHash(filePath string) (string, error) {
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

// ========== 搜索 ==========

// Search 搜索文档
func (m *Manager) Search(query string, limit int) []SearchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	query = strings.ToLower(query)
	results := make([]SearchResult, 0)

	for _, doc := range m.documents {
		if doc.Status != StatusCompleted {
			continue
		}

		score := 0.0
		matchType := ""
		highlight := ""

		// 标题匹配
		if strings.Contains(strings.ToLower(doc.FileName), query) {
			score += 2.0
			matchType = "title"
			highlight = doc.FileName
		}

		// 关键词匹配
		for _, kw := range doc.Keywords {
			if strings.Contains(strings.ToLower(kw), query) {
				score += 1.5
				matchType = "keyword"
				highlight = kw
				break
			}
		}

		// 摘要匹配
		if strings.Contains(strings.ToLower(doc.Summary), query) {
			score += 1.0
			matchType = "summary"
			idx := strings.Index(strings.ToLower(doc.Summary), query)
			start := idx - 20
			if start < 0 {
				start = 0
			}
			end := idx + len(query) + 20
			if end > len(doc.Summary) {
				end = len(doc.Summary)
			}
			highlight = doc.Summary[start:end]
		}

		if score > 0 {
			results = append(results, SearchResult{
				Document:  doc,
				Score:     score,
				MatchType: matchType,
				Highlight: highlight,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

// GetDocument 获取文档
func (m *Manager) GetDocument(id string) (*Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	doc, ok := m.documents[id]
	if !ok {
		return nil, fmt.Errorf("文档不存在: %s", id)
	}
	return doc, nil
}

// ListDocuments 列出文档
func (m *Manager) ListDocuments(docType DocumentType, limit int) []*Document {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Document, 0)
	for _, doc := range m.documents {
		if docType == "" || doc.Type == docType {
			result = append(result, doc)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

// ========== 转换 ==========

// ConvertDocument 转换文档
func (m *Manager) ConvertDocument(sourcePath, targetType string) (*ConversionTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.convCounter++
	task := &ConversionTask{
		ID:         fmt.Sprintf("conv_%d", m.convCounter),
		SourcePath: sourcePath,
		TargetPath: strings.TrimSuffix(sourcePath, filepath.Ext(sourcePath)) + "." + targetType,
		TargetType: targetType,
		Status:     StatusPending,
		CreatedAt:  time.Now(),
	}
	m.conversions[task.ID] = task

	go m.processConversion(task)
	return task, nil
}

// processConversion 处理转换
func (m *Manager) processConversion(task *ConversionTask) {
	m.mu.Lock()
	task.Status = StatusProcessing
	m.mu.Unlock()

	// 模拟转换
	time.Sleep(100 * time.Millisecond)

	m.mu.Lock()
	now := time.Now()
	task.Status = StatusCompleted
	task.Progress = 100
	task.CompletedAt = &now
	m.mu.Unlock()
}

// GetConversion 获取转换任务
func (m *Manager) GetConversion(id string) (*ConversionTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.conversions[id]
	if !ok {
		return nil, fmt.Errorf("转换任务不存在: %s", id)
	}
	return task, nil
}

// ========== 统计 ==========

// GetStats 获取统计
func (m *Manager) GetStats() *DocStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &DocStats{
		TotalDocs: len(m.documents),
		ByType:    make(map[string]int),
		ByLanguage: make(map[string]int),
	}

	for _, doc := range m.documents {
		stats.ByType[string(doc.Type)]++
		stats.ByLanguage[doc.Language]++
		stats.TotalPages += doc.PageCount
		stats.TotalWords += doc.WordCount

		switch doc.Status {
		case StatusCompleted:
			stats.ProcessedDocs++
		case StatusFailed:
			stats.FailedDocs++
		}
	}

	if stats.TotalDocs > 0 {
		stats.ProcessingRate = float64(stats.ProcessedDocs) / float64(stats.TotalDocs)
	}
	return stats
}

// Close 关闭
func (m *Manager) Close() error {
	m.cancel()
	return nil
}
