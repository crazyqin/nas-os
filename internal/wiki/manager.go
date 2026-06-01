// Package wiki 提供知识库核心管理逻辑
package wiki

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager 知识库管理器
type Manager struct {
	mu    sync.RWMutex
	wikis map[string]*Wiki
}

// NewManager 创建知识库管理器
func NewManager() *Manager {
	return &Manager{
		wikis: make(map[string]*Wiki),
	}
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ListWikis 列出所有知识库
func (m *Manager) ListWikis() []*Wiki {
	m.mu.RLock()
	defer m.mu.RUnlock()

	wikis := make([]*Wiki, 0, len(m.wikis))
	for _, w := range m.wikis {
		wikis = append(wikis, w)
	}
	sort.Slice(wikis, func(i, j int) bool {
		return wikis[i].CreatedAt.After(wikis[j].CreatedAt)
	})
	return wikis
}

// CreateWiki 创建知识库
func (m *Manager) CreateWiki(req *CreateWikiRequest) (*Wiki, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	wiki := &Wiki{
		ID:          generateID(),
		Name:        req.Name,
		Description: req.Description,
		OwnerID:     req.OwnerID,
		IsPublic:    req.IsPublic,
		Pages:       make([]*Page, 0),
		Permissions: make([]*Permission, 0),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	m.wikis[wiki.ID] = wiki
	return wiki, nil
}

// GetWiki 获取知识库
func (m *Manager) GetWiki(id string) (*Wiki, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	wiki, ok := m.wikis[id]
	if !ok {
		return nil, fmt.Errorf("wiki not found: %s", id)
	}
	return wiki, nil
}

// DeleteWiki 删除知识库
func (m *Manager) DeleteWiki(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.wikis[id]; !ok {
		return fmt.Errorf("wiki not found: %s", id)
	}
	delete(m.wikis, id)
	return nil
}

// CreatePage 创建页面
func (m *Manager) CreatePage(wikiID string, req *CreatePageRequest) (*Page, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	wiki, ok := m.wikis[wikiID]
	if !ok {
		return nil, fmt.Errorf("wiki not found: %s", wikiID)
	}

	now := time.Now()
	pageID := generateID()

	// 构建路径
	path := "/" + strings.ReplaceAll(strings.ToLower(req.Title), " ", "-")
	if req.ParentID != "" {
		parent := findPage(wiki.Pages, req.ParentID)
		if parent == nil {
			return nil, fmt.Errorf("parent page not found: %s", req.ParentID)
		}
		path = parent.Path + path
	}

	tags := req.Tags
	if tags == nil {
		tags = make([]string, 0)
	}

	page := &Page{
		ID:        pageID,
		WikiID:    wikiID,
		Title:     req.Title,
		Content:   req.Content,
		ParentID:  req.ParentID,
		Path:      path,
		Tags:      tags,
		AuthorID:  req.AuthorID,
		Status:    "published",
		Version:   1,
		Children:  make([]*Page, 0),
		Revisions: make([]*Revision, 0),
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 保存初始版本
	page.Revisions = append(page.Revisions, &Revision{
		ID:        generateID(),
		PageID:    pageID,
		Version:   1,
		Title:     req.Title,
		Content:   req.Content,
		AuthorID:  req.AuthorID,
		Comment:   "创建页面",
		CreatedAt: now,
	})

	// 挂载到父页面或顶级
	if req.ParentID != "" {
		parent := findPage(wiki.Pages, req.ParentID)
		if parent != nil {
			parent.Children = append(parent.Children, page)
		}
	} else {
		wiki.Pages = append(wiki.Pages, page)
	}

	wiki.UpdatedAt = now
	return page, nil
}

// UpdatePage 更新页面
func (m *Manager) UpdatePage(wikiID, pageID string, req *UpdatePageRequest) (*Page, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	wiki, ok := m.wikis[wikiID]
	if !ok {
		return nil, fmt.Errorf("wiki not found: %s", wikiID)
	}

	page := findPage(wiki.Pages, pageID)
	if page == nil {
		return nil, fmt.Errorf("page not found: %s", pageID)
	}

	now := time.Now()

	if req.Title != nil {
		page.Title = *req.Title
	}
	if req.Content != nil {
		page.Content = *req.Content
	}
	if req.Tags != nil {
		page.Tags = req.Tags
	}

	page.Version++
	page.UpdatedAt = now

	// 保存版本历史
	comment := req.Comment
	if comment == "" {
		comment = fmt.Sprintf("更新到版本 %d", page.Version)
	}
	page.Revisions = append(page.Revisions, &Revision{
		ID:        generateID(),
		PageID:    pageID,
		Version:   page.Version,
		Title:     page.Title,
		Content:   page.Content,
		AuthorID:  req.AuthorID,
		Comment:   comment,
		CreatedAt: now,
	})

	wiki.UpdatedAt = now
	return page, nil
}

// GetPage 获取页面
func (m *Manager) GetPage(wikiID, pageID string) (*Page, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	wiki, ok := m.wikis[wikiID]
	if !ok {
		return nil, fmt.Errorf("wiki not found: %s", wikiID)
	}

	page := findPage(wiki.Pages, pageID)
	if page == nil {
		return nil, fmt.Errorf("page not found: %s", pageID)
	}
	return page, nil
}

// DeletePage 删除页面
func (m *Manager) DeletePage(wikiID, pageID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	wiki, ok := m.wikis[wikiID]
	if !ok {
		return fmt.Errorf("wiki not found: %s", wikiID)
	}

	removed := removePage(&wiki.Pages, pageID)
	if !removed {
		return fmt.Errorf("page not found: %s", pageID)
	}

	wiki.UpdatedAt = time.Now()
	return nil
}

// GetHistory 获取页面版本历史
func (m *Manager) GetHistory(wikiID, pageID string) ([]*Revision, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	wiki, ok := m.wikis[wikiID]
	if !ok {
		return nil, fmt.Errorf("wiki not found: %s", wikiID)
	}

	page := findPage(wiki.Pages, pageID)
	if page == nil {
		return nil, fmt.Errorf("page not found: %s", pageID)
	}

	// 按版本倒序
	revisions := make([]*Revision, len(page.Revisions))
	copy(revisions, page.Revisions)
	sort.Slice(revisions, func(i, j int) bool {
		return revisions[i].Version > revisions[j].Version
	})

	return revisions, nil
}

// SearchPages 全文搜索
func (m *Manager) SearchPages(req *SearchRequest) []*SearchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}

	query := strings.ToLower(req.Query)
	results := make([]*SearchResult, 0)

	for _, wiki := range m.wikis {
		if req.WikiID != "" && wiki.ID != req.WikiID {
			continue
		}
		searchInPages(wiki.ID, wiki.Pages, query, &results)
	}

	// 按分数排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

// SetPermission 设置权限
func (m *Manager) SetPermission(wikiID string, req *SetPermissionRequest) (*Permission, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	wiki, ok := m.wikis[wikiID]
	if !ok {
		return nil, fmt.Errorf("wiki not found: %s", wikiID)
	}

	// 查找已有权限
	for _, p := range wiki.Permissions {
		if p.UserID == req.UserID {
			p.CanView = req.CanView
			p.CanEdit = req.CanEdit
			wiki.UpdatedAt = time.Now()
			return p, nil
		}
	}

	perm := &Permission{
		WikiID:  wikiID,
		UserID:  req.UserID,
		CanView: req.CanView,
		CanEdit: req.CanEdit,
	}

	wiki.Permissions = append(wiki.Permissions, perm)
	wiki.UpdatedAt = time.Now()
	return perm, nil
}

// RemovePermission 移除权限
func (m *Manager) RemovePermission(wikiID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	wiki, ok := m.wikis[wikiID]
	if !ok {
		return fmt.Errorf("wiki not found: %s", wikiID)
	}

	for i, p := range wiki.Permissions {
		if p.UserID == userID {
			wiki.Permissions = append(wiki.Permissions[:i], wiki.Permissions[i+1:]...)
			wiki.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("permission not found for user: %s", userID)
}

// ============================================================
// 辅助函数
// ============================================================

func findPage(pages []*Page, pageID string) *Page {
	for _, p := range pages {
		if p.ID == pageID {
			return p
		}
		if found := findPage(p.Children, pageID); found != nil {
			return found
		}
	}
	return nil
}

func removePage(pages *[]*Page, pageID string) bool {
	for i, p := range *pages {
		if p.ID == pageID {
			*pages = append((*pages)[:i], (*pages)[i+1:]...)
			return true
		}
		if removePage(&p.Children, pageID) {
			return true
		}
	}
	return false
}

func searchInPages(wikiID string, pages []*Page, query string, results *[]*SearchResult) {
	for _, p := range pages {
		titleMatch := strings.Contains(strings.ToLower(p.Title), query)
		contentMatch := strings.Contains(strings.ToLower(p.Content), query)

		if titleMatch || contentMatch {
			score := 0.0
			if titleMatch {
				score += 10.0
			}
			if contentMatch {
				score += 5.0
			}

			highlighted := p.Content
			if len(highlighted) > 200 {
				highlighted = highlighted[:200] + "..."
			}

			*results = append(*results, &SearchResult{
				PageID:      p.ID,
				WikiID:      wikiID,
				Title:       p.Title,
				Content:     highlighted,
				Path:        p.Path,
				Score:       score,
				Highlighted: highlighted,
				UpdatedAt:   p.UpdatedAt,
			})
		}

		searchInPages(wikiID, p.Children, query, results)
	}
}
