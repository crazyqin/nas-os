// Package docmanager 提供文档管理系统功能
package docmanager

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// Manager 文档管理器
type Manager struct {
	mu         sync.RWMutex
	documents  map[string]*Document
	categories map[string]*Category
	tags       map[string]*Tag
	nextDocID  int
	nextCatID  int
	nextTagID  int
}

// NewManager 创建新的文档管理器
func NewManager() *Manager {
	return &Manager{
		documents:  make(map[string]*Document),
		categories: make(map[string]*Category),
		tags:       make(map[string]*Tag),
		nextDocID:  1,
		nextCatID:  1,
		nextTagID:  1,
	}
}

// CreateDocument 创建文档
func (m *Manager) CreateDocument(req CreateDocumentRequest) (*Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Title == "" {
		return nil, fmt.Errorf("文档标题不能为空")
	}

	now := time.Now()
	id := fmt.Sprintf("doc_%d", m.nextDocID)
	m.nextDocID++

	doc := &Document{
		ID:            id,
		Title:         req.Title,
		Content:       req.Content,
		Tags:          req.Tags,
		Category:      req.Category,
		CreatedAt:     now,
		UpdatedAt:     now,
		FilePath:      req.FilePath,
		MimeType:      req.MimeType,
		Size:          req.Size,
		ThumbnailPath: req.ThumbnailPath,
	}

	m.documents[id] = doc
	log.Printf("文档已创建: %s (%s)", id, req.Title)
	return doc, nil
}

// GetDocument 获取文档
func (m *Manager) GetDocument(id string) (*Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	doc, exists := m.documents[id]
	if !exists {
		return nil, fmt.Errorf("文档不存在: %s", id)
	}
	return doc, nil
}

// UpdateDocument 更新文档
func (m *Manager) UpdateDocument(id string, req UpdateDocumentRequest) (*Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	doc, exists := m.documents[id]
	if !exists {
		return nil, fmt.Errorf("文档不存在: %s", id)
	}

	if req.Title != "" {
		doc.Title = req.Title
	}
	if req.Content != "" {
		doc.Content = req.Content
	}
	if req.Tags != nil {
		doc.Tags = req.Tags
	}
	if req.Category != "" {
		doc.Category = req.Category
	}
	if req.FilePath != "" {
		doc.FilePath = req.FilePath
	}
	if req.MimeType != "" {
		doc.MimeType = req.MimeType
	}
	if req.Size > 0 {
		doc.Size = req.Size
	}
	if req.ThumbnailPath != "" {
		doc.ThumbnailPath = req.ThumbnailPath
	}
	doc.UpdatedAt = time.Now()

	log.Printf("文档已更新: %s", id)
	return doc, nil
}

// DeleteDocument 删除文档
func (m *Manager) DeleteDocument(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.documents[id]; !exists {
		return fmt.Errorf("文档不存在: %s", id)
	}

	delete(m.documents, id)
	log.Printf("文档已删除: %s", id)
	return nil
}

// ListDocuments 列出文档（分页）
func (m *Manager) ListDocuments(page, pageSize int) ([]Document, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	// 将所有文档放入切片
	allDocs := make([]Document, 0, len(m.documents))
	for _, doc := range m.documents {
		allDocs = append(allDocs, *doc)
	}

	total := len(allDocs)
	start := (page - 1) * pageSize
	if start >= total {
		return []Document{}, total, nil
	}

	end := start + pageSize
	if end > total {
		end = total
	}

	return allDocs[start:end], total, nil
}

// SearchDocuments 搜索文档
func (m *Manager) SearchDocuments(query SearchQuery) (*SearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	page := query.Page
	pageSize := query.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	var matched []Document
	for _, doc := range m.documents {
		if matchDocument(doc, query) {
			matched = append(matched, *doc)
		}
	}

	total := len(matched)
	start := (page - 1) * pageSize
	if start >= total {
		return &SearchResult{
			Documents: []Document{},
			Total:     total,
			Page:      page,
			PageSize:  pageSize,
		}, nil
	}

	end := start + pageSize
	if end > total {
		end = total
	}

	return &SearchResult{
		Documents: matched[start:end],
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
	}, nil
}

// matchDocument 检查文档是否匹配搜索条件
func matchDocument(doc *Document, query SearchQuery) bool {
	// 关键词匹配（标题、内容、OCR文本）
	if query.Query != "" {
		q := strings.ToLower(query.Query)
		titleMatch := strings.Contains(strings.ToLower(doc.Title), q)
		contentMatch := strings.Contains(strings.ToLower(doc.Content), q)
		ocrMatch := strings.Contains(strings.ToLower(doc.OCRText), q)
		if !titleMatch && !contentMatch && !ocrMatch {
			return false
		}
	}

	// 标签匹配
	if len(query.Tags) > 0 {
		tagSet := make(map[string]bool)
		for _, t := range doc.Tags {
			tagSet[t] = true
		}
		for _, qt := range query.Tags {
			if !tagSet[qt] {
				return false
			}
		}
	}

	// 分类匹配
	if query.Category != "" && doc.Category != query.Category {
		return false
	}

	// 日期范围匹配
	if !query.DateFrom.IsZero() && doc.CreatedAt.Before(query.DateFrom) {
		return false
	}
	if !query.DateTo.IsZero() && doc.CreatedAt.After(query.DateTo) {
		return false
	}

	// MIME类型匹配
	if query.MimeType != "" && doc.MimeType != query.MimeType {
		return false
	}

	return true
}

// AddTag 给文档添加标签
func (m *Manager) AddTag(docID, tagID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	doc, exists := m.documents[docID]
	if !exists {
		return fmt.Errorf("文档不存在: %s", docID)
	}

	tag, exists := m.tags[tagID]
	if !exists {
		return fmt.Errorf("标签不存在: %s", tagID)
	}

	// 检查是否已存在
	for _, t := range doc.Tags {
		if t == tag.Name {
			return nil // 已存在，不重复添加
		}
	}

	doc.Tags = append(doc.Tags, tag.Name)
	doc.UpdatedAt = time.Now()
	log.Printf("已为文档 %s 添加标签: %s", docID, tag.Name)
	return nil
}

// RemoveTag 移除文档标签
func (m *Manager) RemoveTag(docID, tagID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	doc, exists := m.documents[docID]
	if !exists {
		return fmt.Errorf("文档不存在: %s", docID)
	}

	tag, exists := m.tags[tagID]
	if !exists {
		return fmt.Errorf("标签不存在: %s", tagID)
	}

	for i, t := range doc.Tags {
		if t == tag.Name {
			doc.Tags = append(doc.Tags[:i], doc.Tags[i+1:]...)
			doc.UpdatedAt = time.Now()
			log.Printf("已从文档 %s 移除标签: %s", docID, tag.Name)
			return nil
		}
	}

	return fmt.Errorf("文档 %s 未包含标签: %s", docID, tag.Name)
}

// SetCategory 设置文档分类
func (m *Manager) SetCategory(docID, catID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	doc, exists := m.documents[docID]
	if !exists {
		return fmt.Errorf("文档不存在: %s", docID)
	}

	cat, exists := m.categories[catID]
	if !exists {
		return fmt.Errorf("分类不存在: %s", catID)
	}

	doc.Category = cat.Name
	doc.UpdatedAt = time.Now()
	log.Printf("已设置文档 %s 分类: %s", docID, cat.Name)
	return nil
}

// ProcessOCR 处理文档OCR
func (m *Manager) ProcessOCR(docID string) (*OCRResult, error) {
	m.mu.RLock()
	doc, exists := m.documents[docID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("文档不存在: %s", docID)
	}

	// 模拟OCR处理
	ocrResult := &OCRResult{
		Text:       fmt.Sprintf("[OCR识别结果] 文档 '%s' 的内容已提取", doc.Title),
		Confidence: 0.95,
		Language:   "zh-CN",
		Pages:      1,
	}

	// 更新文档的OCR文本
	m.mu.Lock()
	doc.OCRText = ocrResult.Text
	doc.UpdatedAt = time.Now()
	m.mu.Unlock()

	log.Printf("文档 %s OCR处理完成", docID)
	return ocrResult, nil
}

// AutoClassify 自动分类文档
func (m *Manager) AutoClassify(docID string) (string, error) {
	m.mu.RLock()
	doc, exists := m.documents[docID]
	m.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("文档不存在: %s", docID)
	}

	// 模拟自动分类逻辑
	category := "通用"
	title := strings.ToLower(doc.Title)
	content := strings.ToLower(doc.Content)

	if strings.Contains(title, "合同") || strings.Contains(content, "甲方") {
		category = "合同"
	} else if strings.Contains(title, "报告") || strings.Contains(content, "结论") {
		category = "报告"
	} else if strings.Contains(title, "发票") || strings.Contains(content, "金额") {
		category = "财务"
	} else if strings.HasSuffix(title, ".pdf") || strings.Contains(doc.MimeType, "pdf") {
		category = "PDF文档"
	}

	// 更新文档分类
	m.mu.Lock()
	doc.Category = category
	doc.UpdatedAt = time.Now()
	m.mu.Unlock()

	log.Printf("文档 %s 已自动分类为: %s", docID, category)
	return category, nil
}

// GetCategories 获取所有分类
func (m *Manager) GetCategories() ([]Category, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cats := make([]Category, 0, len(m.categories))
	for _, cat := range m.categories {
		cats = append(cats, *cat)
	}
	return cats, nil
}

// CreateCategory 创建分类
func (m *Manager) CreateCategory(req CreateCategoryRequest) (*Category, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("分类名称不能为空")
	}

	// 检查名称是否重复
	for _, cat := range m.categories {
		if cat.Name == req.Name {
			return nil, fmt.Errorf("分类已存在: %s", req.Name)
		}
	}

	// 验证父分类是否存在
	if req.ParentID != "" {
		if _, exists := m.categories[req.ParentID]; !exists {
			return nil, fmt.Errorf("父分类不存在: %s", req.ParentID)
		}
	}

	id := fmt.Sprintf("cat_%d", m.nextCatID)
	m.nextCatID++

	cat := &Category{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		ParentID:    req.ParentID,
	}

	m.categories[id] = cat
	log.Printf("分类已创建: %s (%s)", id, req.Name)
	return cat, nil
}

// GetTags 获取所有标签
func (m *Manager) GetTags() ([]Tag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tags := make([]Tag, 0, len(m.tags))
	for _, tag := range m.tags {
		tags = append(tags, *tag)
	}
	return tags, nil
}

// CreateTag 创建标签
func (m *Manager) CreateTag(req CreateTagRequest) (*Tag, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("标签名称不能为空")
	}

	// 检查名称是否重复
	for _, tag := range m.tags {
		if tag.Name == req.Name {
			return nil, fmt.Errorf("标签已存在: %s", req.Name)
		}
	}

	id := fmt.Sprintf("tag_%d", m.nextTagID)
	m.nextTagID++

	tag := &Tag{
		ID:    id,
		Name:  req.Name,
		Color: req.Color,
	}

	m.tags[id] = tag
	log.Printf("标签已创建: %s (%s)", id, req.Name)
	return tag, nil
}

// GetStats 获取统计信息
func (m *Manager) GetStats() (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_documents":  len(m.documents),
		"total_categories": len(m.categories),
		"total_tags":       len(m.tags),
	}

	// 按分类统计
	catCount := make(map[string]int)
	totalSize := int64(0)
	for _, doc := range m.documents {
		if doc.Category != "" {
			catCount[doc.Category]++
		}
		totalSize += doc.Size
	}
	stats["documents_by_category"] = catCount
	stats["total_size"] = totalSize

	// 按MIME类型统计
	mimeCount := make(map[string]int)
	for _, doc := range m.documents {
		if doc.MimeType != "" {
			mimeCount[doc.MimeType]++
		}
	}
	stats["documents_by_mime_type"] = mimeCount

	return stats, nil
}
