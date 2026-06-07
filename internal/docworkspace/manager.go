// Package docworkspace 提供文档工作区功能
// 在线文档创建/编辑、版本历史、多人协作、评论批注、模板管理、全文搜索、权限管理
package docworkspace

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ========== 核心类型 ==========

// Permission 权限类型
type Permission string

const (
	PermView  Permission = "view"  // 查看
	PermEdit  Permission = "edit"  // 编辑
	PermAdmin Permission = "admin" // 管理
)

// Document 文档
type Document struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Author    string    `json:"author"`
	Tags      []string  `json:"tags,omitempty"`
	Category  string    `json:"category"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// DocVersion 文档版本
type DocVersion struct {
	Version   int       `json:"version"`
	Content   string    `json:"content"`
	Author    string    `json:"author"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Comment 评论/批注
type Comment struct {
	ID        string    `json:"id"`
	DocID     string    `json:"docId"`
	Author    string    `json:"author"`
	Content   string    `json:"content"`
	Position  int       `json:"position"` // 文档内位置
	CreatedAt time.Time `json:"createdAt"`
}

// Template 文档模板
type Template struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	Category  string    `json:"category"`
	CreatedAt time.Time `json:"createdAt"`
}

// DocCategory 文档分类
type DocCategory struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ParentID string `json:"parentId,omitempty"`
}

// Collaborator 协作者
type Collaborator struct {
	UserID     string     `json:"userId"`
	Permission Permission `json:"permission"`
	DocID      string     `json:"docId"`
}

// SearchResult 搜索结果
type SearchResult struct {
	DocID     string  `json:"docId"`
	Title     string  `json:"title"`
	Snippet   string  `json:"snippet"`
	MatchText string  `json:"matchText"`
	Relevance float64 `json:"relevance"`
}

// ========== Manager ==========

// Manager 文档工作区管理器
type Manager struct {
	mu            sync.RWMutex
	docs          map[string]*Document
	versions      map[string][]DocVersion // docID -> versions
	comments      map[string][]Comment    // docID -> comments
	templates     map[string]*Template
	collaborators map[string][]Collaborator // docID -> collaborators
	categories    map[string]*DocCategory
}

// NewManager 创建管理器
func NewManager() *Manager {
	m := &Manager{
		docs:          make(map[string]*Document),
		versions:      make(map[string][]DocVersion),
		comments:      make(map[string][]Comment),
		templates:     make(map[string]*Template),
		collaborators: make(map[string][]Collaborator),
		categories:    make(map[string]*DocCategory),
	}
	m.initDefaults()
	return m
}

// initDefaults 初始化默认模板和分类
func (m *Manager) initDefaults() {
	m.templates["tpl-meeting"] = &Template{
		ID: "tpl-meeting", Name: "会议纪要", Content: "# 会议纪要\n\n## 参会人员\n\n## 议题\n\n## 决议\n\n## 后续事项\n",
		Category: "办公", CreatedAt: time.Now(),
	}
	m.templates["tpl-weekly"] = &Template{
		ID: "tpl-weekly", Name: "周报", Content: "# 周报\n\n## 本周完成\n\n## 下周计划\n\n## 问题与风险\n",
		Category: "办公", CreatedAt: time.Now(),
	}
	m.templates["tpl-prd"] = &Template{
		ID: "tpl-prd", Name: "需求文档", Content: "# 需求文档\n\n## 背景\n\n## 需求描述\n\n## 验收标准\n\n## 排期\n",
		Category: "产品", CreatedAt: time.Now(),
	}

	m.categories["cat-general"] = &DocCategory{ID: "cat-general", Name: "通用"}
	m.categories["cat-tech"] = &DocCategory{ID: "cat-tech", Name: "技术"}
	m.categories["cat-product"] = &DocCategory{ID: "cat-product", Name: "产品"}
}

// ========== 文档 CRUD ==========

// CreateDoc 创建文档
func (m *Manager) CreateDoc(doc *Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if doc.ID == "" {
		return fmt.Errorf("document ID is required")
	}
	if _, exists := m.docs[doc.ID]; exists {
		return fmt.Errorf("document %s already exists", doc.ID)
	}

	now := time.Now()
	doc.Version = 1
	doc.CreatedAt = now
	doc.UpdatedAt = now
	m.docs[doc.ID] = doc

	// 保存初始版本
	m.versions[doc.ID] = []DocVersion{{
		Version: 1, Content: doc.Content, Author: doc.Author,
		Message: "初始版本", Timestamp: now,
	}}

	return nil
}

// UpdateDoc 更新文档内容
func (m *Manager) UpdateDoc(id string, content string, author string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	doc, ok := m.docs[id]
	if !ok {
		return fmt.Errorf("document %s not found", id)
	}

	doc.Content = content
	doc.Version++
	doc.Author = author
	doc.UpdatedAt = time.Now()

	// 保存版本历史
	m.versions[id] = append(m.versions[id], DocVersion{
		Version: doc.Version, Content: content, Author: author,
		Message: fmt.Sprintf("版本 %d", doc.Version), Timestamp: time.Now(),
	})

	return nil
}

// GetDoc 获取文档
func (m *Manager) GetDoc(id string) *Document {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.docs[id]
}

// DeleteDoc 删除文档
func (m *Manager) DeleteDoc(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.docs[id]; !ok {
		return fmt.Errorf("document %s not found", id)
	}

	delete(m.docs, id)
	delete(m.versions, id)
	delete(m.comments, id)
	delete(m.collaborators, id)
	return nil
}

// ListDocs 列出文档（支持分类和标签过滤）
func (m *Manager) ListDocs(category string, tags []string) []Document {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Document
	for _, doc := range m.docs {
		if category != "" && doc.Category != category {
			continue
		}
		if len(tags) > 0 && !hasAnyTag(doc.Tags, tags) {
			continue
		}
		result = append(result, *doc)
	}
	return result
}

// hasAnyTag 检查文档标签是否包含任一指定标签
func hasAnyTag(docTags, filterTags []string) bool {
	for _, ft := range filterTags {
		for _, dt := range docTags {
			if dt == ft {
				return true
			}
		}
	}
	return false
}

// ========== 版本管理 ==========

// GetVersions 获取文档版本历史
func (m *Manager) GetVersions(docID string) []DocVersion {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.versions[docID]
}

// RevertToVersion 回退到指定版本
func (m *Manager) RevertToVersion(docID string, version int, author string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	doc, ok := m.docs[docID]
	if !ok {
		return fmt.Errorf("document %s not found", docID)
	}

	versions := m.versions[docID]
	for _, v := range versions {
		if v.Version == version {
			doc.Content = v.Content
			doc.Version++
			doc.Author = author
			doc.UpdatedAt = time.Now()

			m.versions[docID] = append(m.versions[docID], DocVersion{
				Version: doc.Version, Content: v.Content, Author: author,
				Message: fmt.Sprintf("回退到版本 %d", version), Timestamp: time.Now(),
			})
			return nil
		}
	}
	return fmt.Errorf("version %d not found for document %s", version, docID)
}

// ========== 评论 ==========

// AddComment 添加评论
func (m *Manager) AddComment(comment *Comment) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.docs[comment.DocID]; !ok {
		return fmt.Errorf("document %s not found", comment.DocID)
	}

	comment.ID = fmt.Sprintf("cmt-%d", len(m.comments[comment.DocID])+1)
	comment.CreatedAt = time.Now()
	m.comments[comment.DocID] = append(m.comments[comment.DocID], *comment)
	return nil
}

// GetComments 获取文档评论
func (m *Manager) GetComments(docID string) []Comment {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.comments[docID]
}

// ========== 模板 ==========

// CreateTemplate 创建模板
func (m *Manager) CreateTemplate(tmpl *Template) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tmpl.ID == "" {
		return fmt.Errorf("template ID is required")
	}
	tmpl.CreatedAt = time.Now()
	m.templates[tmpl.ID] = tmpl
	return nil
}

// ListTemplates 列出所有模板
func (m *Manager) ListTemplates() []Template {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tmpls := make([]Template, 0, len(m.templates))
	for _, t := range m.templates {
		tmpls = append(tmpls, *t)
	}
	return tmpls
}

// ========== 搜索 ==========

// SearchDocs 全文搜索
func (m *Manager) SearchDocs(query string) []SearchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	query = strings.ToLower(query)
	var results []SearchResult

	for _, doc := range m.docs {
		titleMatch := strings.Contains(strings.ToLower(doc.Title), query)
		contentMatch := strings.Contains(strings.ToLower(doc.Content), query)

		if !titleMatch && !contentMatch {
			continue
		}

		relevance := 0.0
		snippet := ""
		matchText := ""

		if titleMatch {
			relevance += 0.6
			matchText = doc.Title
		}
		if contentMatch {
			relevance += 0.4
			// 提取匹配片段
			idx := strings.Index(strings.ToLower(doc.Content), query)
			start := idx - 30
			if start < 0 {
				start = 0
			}
			end := idx + len(query) + 30
			if end > len(doc.Content) {
				end = len(doc.Content)
			}
			snippet = doc.Content[start:end]
			if matchText == "" {
				matchText = snippet
			}
		}

		results = append(results, SearchResult{
			DocID: doc.ID, Title: doc.Title,
			Snippet: snippet, MatchText: matchText, Relevance: relevance,
		})
	}
	return results
}

// ========== 权限管理 ==========

// SetPermission 设置协作者权限
func (m *Manager) SetPermission(docID, userID string, perm string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.docs[docID]; !ok {
		return fmt.Errorf("document %s not found", docID)
	}

	permission := Permission(perm)
	if permission != PermView && permission != PermEdit && permission != PermAdmin {
		return fmt.Errorf("invalid permission: %s", perm)
	}

	collabs := m.collaborators[docID]
	for i, c := range collabs {
		if c.UserID == userID {
			collabs[i].Permission = permission
			m.collaborators[docID] = collabs
			return nil
		}
	}

	m.collaborators[docID] = append(m.collaborators[docID], Collaborator{
		UserID: userID, Permission: permission, DocID: docID,
	})
	return nil
}

// ========== 导出 ==========

// ExportDoc 导出文档
func (m *Manager) ExportDoc(id string, format string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	doc, ok := m.docs[id]
	if !ok {
		return nil, fmt.Errorf("document %s not found", id)
	}

	switch format {
	case "markdown", "md":
		return []byte(doc.Content), nil
	case "html":
		html := fmt.Sprintf("<html><head><title>%s</title></head><body><pre>%s</pre></body></html>",
			doc.Title, doc.Content)
		return []byte(html), nil
	case "pdf":
		// PDF 导出占位
		return []byte(fmt.Sprintf("[PDF] %s\n%s", doc.Title, doc.Content)), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}
