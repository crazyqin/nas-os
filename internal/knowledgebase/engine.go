// Package knowledgebase 提供个人知识库管理功能
package knowledgebase

import (
	"errors"
	"strings"
	"sync"
	"time"
)

// Engine 知识库引擎.
type Engine struct {
	mu         sync.RWMutex
	docs       map[string]*Document
	workspaces map[string]*Workspace
	templates  map[string]*Template
	links      []Link
	notes      map[string]*Note
	idCounter  int64
}

// NewEngine 创建知识库引擎.
func NewEngine() *Engine {
	return &Engine{
		docs:       make(map[string]*Document),
		workspaces: make(map[string]*Workspace),
		templates:  make(map[string]*Template),
		links:      make([]Link, 0),
		notes:      make(map[string]*Note),
	}
}

// generateID 生成唯一ID.
func (e *Engine) generateID(prefix string) string {
	e.idCounter++
	return prefix + "_" + time.Now().Format("20060102150405") + "_" + string(rune('A'+e.idCounter%26))
}

// CreateDoc 创建文档.
func (e *Engine) CreateDoc(req CreateDocRequest) (*Document, error) {
	if req.Title == "" {
		return nil, errors.New("标题不能为空")
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	doc := &Document{
		ID:          e.generateID("doc"),
		Title:       req.Title,
		Content:     req.Content,
		Author:      req.Author,
		WorkspaceID: req.WorkspaceID,
		Tags:        req.Tags,
		Links:       make([]Link, 0),
		IsTemplate:  req.IsTemplate,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	e.docs[doc.ID] = doc

	if req.WorkspaceID != "" {
		if ws, ok := e.workspaces[req.WorkspaceID]; ok {
			ws.DocCount++
		}
	}

	return doc, nil
}

// GetDoc 获取文档.
func (e *Engine) GetDoc(id string) (*Document, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	doc, ok := e.docs[id]
	if !ok {
		return nil, errors.New("文档不存在")
	}
	return doc, nil
}

// UpdateDoc 更新文档.
func (e *Engine) UpdateDoc(id string, req UpdateDocRequest) (*Document, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	doc, ok := e.docs[id]
	if !ok {
		return nil, errors.New("文档不存在")
	}

	if req.Title != nil {
		doc.Title = *req.Title
	}
	if req.Content != nil {
		doc.Content = *req.Content
	}
	if req.Tags != nil {
		doc.Tags = req.Tags
	}
	if req.IsFavorite != nil {
		doc.IsFavorite = *req.IsFavorite
	}
	if req.IsTemplate != nil {
		doc.IsTemplate = *req.IsTemplate
	}
	doc.UpdatedAt = time.Now()

	return doc, nil
}

// DeleteDoc 删除文档.
func (e *Engine) DeleteDoc(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.docs[id]; !ok {
		return errors.New("文档不存在")
	}

	// 删除相关链接
	newLinks := make([]Link, 0)
	for _, l := range e.links {
		if l.SourceID != id && l.TargetID != id {
			newLinks = append(newLinks, l)
		}
	}
	e.links = newLinks

	// 删除相关笔记
	for nid, note := range e.notes {
		if note.DocID == id {
			delete(e.notes, nid)
		}
	}

	// 更新工作空间计数
	if doc := e.docs[id]; doc.WorkspaceID != "" {
		if ws, ok := e.workspaces[doc.WorkspaceID]; ok {
			ws.DocCount--
		}
	}

	delete(e.docs, id)
	return nil
}

// ListDocs 列出所有文档.
func (e *Engine) ListDocs(workspaceID string) []*Document {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Document, 0)
	for _, doc := range e.docs {
		if workspaceID == "" || doc.WorkspaceID == workspaceID {
			result = append(result, doc)
		}
	}
	return result
}

// SearchDocs 搜索文档.
func (e *Engine) SearchDocs(query SearchQuery) []SearchResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	results := make([]SearchResult, 0)
	queryLower := strings.ToLower(query.Query)

	for _, doc := range e.docs {
		if query.WorkspaceID != "" && doc.WorkspaceID != query.WorkspaceID {
			continue
		}
		if query.Author != "" && doc.Author != query.Author {
			continue
		}

		score := 0.0
		titleLower := strings.ToLower(doc.Title)
		contentLower := strings.ToLower(doc.Content)

		if strings.Contains(titleLower, queryLower) {
			score += 2.0
		}
		if strings.Contains(contentLower, queryLower) {
			score += 1.0
		}

		// 标签匹配
		for _, tag := range query.Tags {
			for _, docTag := range doc.Tags {
				if strings.EqualFold(tag, docTag) {
					score += 0.5
				}
			}
		}

		if score > 0 {
			snippet := ""
			if idx := strings.Index(contentLower, queryLower); idx >= 0 {
				start := idx - 50
				if start < 0 {
					start = 0
				}
				end := idx + len(query.Query) + 50
				if end > len(doc.Content) {
					end = len(doc.Content)
				}
				snippet = doc.Content[start:end]
			}

			results = append(results, SearchResult{
				Doc:     *doc,
				Score:   score,
				Snippet: snippet,
			})
		}
	}

	// 按分数排序
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// 分页
	if query.Limit > 0 && query.Offset < len(results) {
		end := query.Offset + query.Limit
		if end > len(results) {
			end = len(results)
		}
		return results[query.Offset:end]
	}

	return results
}

// CreateWorkspace 创建工作空间.
func (e *Engine) CreateWorkspace(req CreateWorkspaceRequest) (*Workspace, error) {
	if req.Name == "" {
		return nil, errors.New("名称不能为空")
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	ws := &Workspace{
		ID:          e.generateID("ws"),
		Name:        req.Name,
		Description: req.Description,
		Owner:       req.Owner,
		CreatedAt:   time.Now(),
	}
	e.workspaces[ws.ID] = ws
	return ws, nil
}

// GetWorkspace 获取工作空间.
func (e *Engine) GetWorkspace(id string) (*Workspace, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ws, ok := e.workspaces[id]
	if !ok {
		return nil, errors.New("工作空间不存在")
	}
	return ws, nil
}

// UpdateWorkspace 更新工作空间.
func (e *Engine) UpdateWorkspace(id string, req UpdateWorkspaceRequest) (*Workspace, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	ws, ok := e.workspaces[id]
	if !ok {
		return nil, errors.New("工作空间不存在")
	}

	if req.Name != nil {
		ws.Name = *req.Name
	}
	if req.Description != nil {
		ws.Description = *req.Description
	}

	return ws, nil
}

// DeleteWorkspace 删除工作空间.
func (e *Engine) DeleteWorkspace(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.workspaces[id]; !ok {
		return errors.New("工作空间不存在")
	}

	// 移动文档到默认空间
	for _, doc := range e.docs {
		if doc.WorkspaceID == id {
			doc.WorkspaceID = ""
		}
	}

	delete(e.workspaces, id)
	return nil
}

// ListWorkspaces 列出工作空间.
func (e *Engine) ListWorkspaces(owner string) []*Workspace {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Workspace, 0)
	for _, ws := range e.workspaces {
		if owner == "" || ws.Owner == owner {
			result = append(result, ws)
		}
	}
	return result
}

// CreateTemplate 创建模板.
func (e *Engine) CreateTemplate(name, description, content, category string) (*Template, error) {
	if name == "" {
		return nil, errors.New("名称不能为空")
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	tpl := &Template{
		ID:          e.generateID("tpl"),
		Name:        name,
		Description: description,
		Content:     content,
		Category:    category,
		CreatedAt:   time.Now(),
	}
	e.templates[tpl.ID] = tpl
	return tpl, nil
}

// GetTemplate 获取模板.
func (e *Engine) GetTemplate(id string) (*Template, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tpl, ok := e.templates[id]
	if !ok {
		return nil, errors.New("模板不存在")
	}
	return tpl, nil
}

// ListTemplates 列出模板.
func (e *Engine) ListTemplates(category string) []*Template {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Template, 0)
	for _, tpl := range e.templates {
		if category == "" || tpl.Category == category {
			result = append(result, tpl)
		}
	}
	return result
}

// DeleteTemplate 删除模板.
func (e *Engine) DeleteTemplate(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.templates[id]; !ok {
		return errors.New("模板不存在")
	}
	delete(e.templates, id)
	return nil
}

// CreateDocFromTemplate 从模板创建文档.
func (e *Engine) CreateDocFromTemplate(templateID string, req CreateDocRequest) (*Document, error) {
	e.mu.RLock()
	tpl, ok := e.templates[templateID]
	e.mu.RUnlock()

	if !ok {
		return nil, errors.New("模板不存在")
	}

	if req.Content == "" {
		req.Content = tpl.Content
	}
	return e.CreateDoc(req)
}

// AddNote 添加笔记.
func (e *Engine) AddNote(docID, content, author string) (*Note, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.docs[docID]; !ok {
		return nil, errors.New("文档不存在")
	}

	note := &Note{
		ID:        e.generateID("note"),
		DocID:     docID,
		Content:   content,
		Author:    author,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	e.notes[note.ID] = note
	return note, nil
}

// GetNotes 获取文档的笔记.
func (e *Engine) GetNotes(docID string) []*Note {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Note, 0)
	for _, note := range e.notes {
		if note.DocID == docID {
			result = append(result, note)
		}
	}
	return result
}

// GetTagStats 获取标签统计.
func (e *Engine) GetTagStats() []TagStat {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tagCount := make(map[string]int)
	for _, doc := range e.docs {
		for _, tag := range doc.Tags {
			tagCount[tag]++
		}
	}

	stats := make([]TagStat, 0)
	for tag, count := range tagCount {
		stats = append(stats, TagStat{Tag: tag, Count: count})
	}
	return stats
}

// GetFavorites 获取收藏文档.
func (e *Engine) GetFavorites() []*Document {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Document, 0)
	for _, doc := range e.docs {
		if doc.IsFavorite {
			result = append(result, doc)
		}
	}
	return result
}
