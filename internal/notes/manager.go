// Package notes 提供笔记应用核心业务逻辑
package notes

import (
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 笔记管理器.
type Manager struct {
	notes      map[string]*Note
	notebooks  map[string]*Notebook
	tags       map[string]*Tag
	shareLinks map[string]*ShareLink
	versions   map[string][]*NoteVersion // noteID -> versions
	mu         sync.RWMutex
}

// NewManager 创建笔记管理器.
func NewManager() *Manager {
	return &Manager{
		notes:      make(map[string]*Note),
		notebooks:  make(map[string]*Notebook),
		tags:       make(map[string]*Tag),
		shareLinks: make(map[string]*ShareLink),
		versions:   make(map[string][]*NoteVersion),
	}
}

// ========== 笔记 CRUD ==========

// CreateNote 创建笔记.
func (m *Manager) CreateNote(req CreateNoteRequest) *Note {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	note := &Note{
		ID:         uuid.New().String(),
		Title:      req.Title,
		Content:    req.Content,
		Author:     req.Author,
		NotebookID: req.NotebookID,
		Tags:       req.Tags,
		IsPinned:   false,
		IsFavorite: false,
		IsPublic:   false,
		CreatedAt:  now,
		UpdatedAt:  now,
		Version:    1,
		WordCount:  countWords(req.Content),
	}

	if note.Tags == nil {
		note.Tags = []string{}
	}

	m.notes[note.ID] = note

	// 保存初始版本
	m.saveVersion(note)

	// 更新标签统计
	m.updateTagCounts()

	// 更新笔记本笔记数
	if note.NotebookID != "" {
		if nb, ok := m.notebooks[note.NotebookID]; ok {
			nb.NoteCount++
		}
	}

	log.Printf("[notes] 创建笔记: %s (notebook=%s)", note.Title, note.NotebookID)
	return note
}

// GetNote 获取笔记.
func (m *Manager) GetNote(id string) (*Note, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	note, ok := m.notes[id]
	if !ok {
		return nil, fmt.Errorf("note %q not found", id)
	}
	return note, nil
}

// ListNotes 列出所有笔记.
func (m *Manager) ListNotes() []*Note {
	m.mu.RLock()
	defer m.mu.RUnlock()

	notes := make([]*Note, 0, len(m.notes))
	for _, n := range m.notes {
		notes = append(notes, n)
	}

	sort.Slice(notes, func(i, j int) bool {
		return notes[i].UpdatedAt.After(notes[j].UpdatedAt)
	})

	return notes
}

// ListNotesByNotebook 按笔记本列出笔记.
func (m *Manager) ListNotesByNotebook(notebookID string) []*Note {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var notes []*Note
	for _, n := range m.notes {
		if n.NotebookID == notebookID {
			notes = append(notes, n)
		}
	}

	sort.Slice(notes, func(i, j int) bool {
		return notes[i].UpdatedAt.After(notes[j].UpdatedAt)
	})

	return notes
}

// UpdateNote 更新笔记.
func (m *Manager) UpdateNote(id string, req UpdateNoteRequest) (*Note, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	note, ok := m.notes[id]
	if !ok {
		return nil, fmt.Errorf("note %q not found", id)
	}

	if req.Title != nil {
		note.Title = *req.Title
	}
	if req.Content != nil {
		note.Content = *req.Content
		note.WordCount = countWords(*req.Content)
	}
	if req.Tags != nil {
		note.Tags = req.Tags
	}
	if req.IsPinned != nil {
		note.IsPinned = *req.IsPinned
	}
	if req.IsFavorite != nil {
		note.IsFavorite = *req.IsFavorite
	}
	if req.IsPublic != nil {
		note.IsPublic = *req.IsPublic
	}

	note.UpdatedAt = time.Now()
	note.Version++

	// 保存版本历史
	m.saveVersion(note)

	// 更新标签统计
	m.updateTagCounts()

	log.Printf("[notes] 更新笔记: %s (version=%d)", note.Title, note.Version)
	return note, nil
}

// DeleteNote 删除笔记.
func (m *Manager) DeleteNote(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	note, ok := m.notes[id]
	if !ok {
		return fmt.Errorf("note %q not found", id)
	}

	// 删除关联的分享链接
	for sid, link := range m.shareLinks {
		if link.NoteID == id {
			delete(m.shareLinks, sid)
		}
	}

	// 删除版本历史
	delete(m.versions, id)

	// 更新笔记本笔记数
	if note.NotebookID != "" {
		if nb, ok := m.notebooks[note.NotebookID]; ok {
			nb.NoteCount--
		}
	}

	delete(m.notes, id)

	// 更新标签统计
	m.updateTagCounts()

	log.Printf("[notes] 删除笔记: %s", note.Title)
	return nil
}

// ========== 笔记本管理 ==========

// CreateNotebook 创建笔记本.
func (m *Manager) CreateNotebook(req CreateNotebookRequest) *Notebook {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	nb := &Notebook{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Owner:       req.Owner,
		Color:       req.Color,
		Icon:        req.Icon,
		NoteCount:   0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	m.notebooks[nb.ID] = nb
	log.Printf("[notes] 创建笔记本: %s", nb.Name)
	return nb
}

// GetNotebook 获取笔记本.
func (m *Manager) GetNotebook(id string) (*Notebook, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nb, ok := m.notebooks[id]
	if !ok {
		return nil, fmt.Errorf("notebook %q not found", id)
	}
	return nb, nil
}

// ListNotebooks 列出所有笔记本.
func (m *Manager) ListNotebooks() []*Notebook {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nbs := make([]*Notebook, 0, len(m.notebooks))
	for _, nb := range m.notebooks {
		nbs = append(nbs, nb)
	}

	sort.Slice(nbs, func(i, j int) bool {
		return nbs[i].Name < nbs[j].Name
	})

	return nbs
}

// UpdateNotebook 更新笔记本.
func (m *Manager) UpdateNotebook(id string, req UpdateNotebookRequest) (*Notebook, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	nb, ok := m.notebooks[id]
	if !ok {
		return nil, fmt.Errorf("notebook %q not found", id)
	}

	if req.Name != nil {
		nb.Name = *req.Name
	}
	if req.Description != nil {
		nb.Description = *req.Description
	}
	if req.Color != nil {
		nb.Color = *req.Color
	}
	if req.Icon != nil {
		nb.Icon = *req.Icon
	}

	nb.UpdatedAt = time.Now()

	log.Printf("[notes] 更新笔记本: %s", nb.Name)
	return nb, nil
}

// DeleteNotebook 删除笔记本.
func (m *Manager) DeleteNotebook(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.notebooks[id]; !ok {
		return fmt.Errorf("notebook %q not found", id)
	}

	// 将笔记本中的笔记移到默认笔记本
	for _, note := range m.notes {
		if note.NotebookID == id {
			note.NotebookID = ""
		}
	}

	delete(m.notebooks, id)
	log.Printf("[notes] 删除笔记本: %s", id)
	return nil
}

// ========== 标签管理 ==========

// ListTags 列出所有标签.
func (m *Manager) ListTags() []*Tag {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tags := make([]*Tag, 0, len(m.tags))
	for _, t := range m.tags {
		tags = append(tags, t)
	}

	sort.Slice(tags, func(i, j int) bool {
		return tags[i].NoteCount > tags[j].NoteCount
	})

	return tags
}

// ListNotesByTag 按标签列出笔记.
func (m *Manager) ListNotesByTag(tagName string) []*Note {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var notes []*Note
	tagLower := strings.ToLower(tagName)
	for _, n := range m.notes {
		for _, t := range n.Tags {
			if strings.ToLower(t) == tagLower {
				notes = append(notes, n)
				break
			}
		}
	}

	sort.Slice(notes, func(i, j int) bool {
		return notes[i].UpdatedAt.After(notes[j].UpdatedAt)
	})

	return notes
}

// updateTagCounts 更新标签统计.
func (m *Manager) updateTagCounts() {
	m.tags = make(map[string]*Tag)
	for _, note := range m.notes {
		for _, tagName := range note.Tags {
			tagKey := strings.ToLower(tagName)
			if tag, ok := m.tags[tagKey]; ok {
				tag.NoteCount++
			} else {
				m.tags[tagKey] = &Tag{
					ID:        uuid.New().String(),
					Name:      tagName,
					NoteCount: 1,
					CreatedAt: time.Now(),
				}
			}
		}
	}
}

// ========== 笔记分享 ==========

// CreateShareLink 创建分享链接.
func (m *Manager) CreateShareLink(noteID string, req CreateShareLinkRequest) (*ShareLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.notes[noteID]; !ok {
		return nil, fmt.Errorf("note %q not found", noteID)
	}

	link := &ShareLink{
		ID:         uuid.New().String(),
		NoteID:     noteID,
		Token:      generateToken(),
		Password:   req.Password,
		ExpiresAt:  req.ExpiresAt,
		AllowEdit:  req.AllowEdit,
		VisitCount: 0,
		CreatedAt:  time.Now(),
	}

	m.shareLinks[link.ID] = link

	// 标记笔记为公开
	m.notes[noteID].IsPublic = true

	log.Printf("[notes] 创建分享链接: note=%s, token=%s", noteID, link.Token)
	return link, nil
}

// GetShareLink 获取分享链接.
func (m *Manager) GetShareLink(token string) (*ShareLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, link := range m.shareLinks {
		if link.Token == token {
			// 检查是否过期
			if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
				return nil, fmt.Errorf("share link expired")
			}
			return link, nil
		}
	}
	return nil, fmt.Errorf("share link not found")
}

// AccessSharedNote 访问分享的笔记.
func (m *Manager) AccessSharedNote(token, password string) (*Note, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var link *ShareLink
	for _, l := range m.shareLinks {
		if l.Token == token {
			link = l
			break
		}
	}

	if link == nil {
		return nil, fmt.Errorf("share link not found")
	}

	// 检查是否过期
	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("share link expired")
	}

	// 检查密码
	if link.Password != "" && link.Password != password {
		return nil, fmt.Errorf("invalid password")
	}

	// 增加访问计数
	link.VisitCount++

	note, ok := m.notes[link.NoteID]
	if !ok {
		return nil, fmt.Errorf("note not found")
	}

	return note, nil
}

// DeleteShareLink 删除分享链接.
func (m *Manager) DeleteShareLink(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.shareLinks[id]; !ok {
		return fmt.Errorf("share link %q not found", id)
	}

	delete(m.shareLinks, id)
	return nil
}

// ListShareLinks 列出笔记的分享链接.
func (m *Manager) ListShareLinks(noteID string) []*ShareLink {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var links []*ShareLink
	for _, l := range m.shareLinks {
		if l.NoteID == noteID {
			links = append(links, l)
		}
	}
	return links
}

// ========== 搜索功能 ==========

// SearchNotes 全文搜索笔记.
func (m *Manager) SearchNotes(query string) []*Note {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queryLower := strings.ToLower(query)
	var results []*Note

	for _, note := range m.notes {
		if strings.Contains(strings.ToLower(note.Title), queryLower) ||
			strings.Contains(strings.ToLower(note.Content), queryLower) {
			results = append(results, note)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].UpdatedAt.After(results[j].UpdatedAt)
	})

	return results
}

// SearchNotesAdvanced 高级搜索笔记.
func (m *Manager) SearchNotesAdvanced(query, notebookID, tag string) []*Note {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queryLower := strings.ToLower(query)
	var results []*Note

	for _, note := range m.notes {
		// 笔记本过滤
		if notebookID != "" && note.NotebookID != notebookID {
			continue
		}

		// 标签过滤
		if tag != "" {
			hasTag := false
			for _, t := range note.Tags {
				if strings.ToLower(t) == strings.ToLower(tag) {
					hasTag = true
					break
				}
			}
			if !hasTag {
				continue
			}
		}

		// 全文搜索
		if strings.Contains(strings.ToLower(note.Title), queryLower) ||
			strings.Contains(strings.ToLower(note.Content), queryLower) {
			results = append(results, note)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].UpdatedAt.After(results[j].UpdatedAt)
	})

	return results
}

// ========== 版本历史 ==========

// GetNoteVersions 获取笔记版本历史.
func (m *Manager) GetNoteVersions(noteID string) []*NoteVersion {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions := m.versions[noteID]
	if versions == nil {
		return []*NoteVersion{}
	}

	// 返回副本，按版本号降序
	result := make([]*NoteVersion, len(versions))
	copy(result, versions)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version > result[j].Version
	})

	return result
}

// GetNoteVersion 获取笔记指定版本.
func (m *Manager) GetNoteVersion(noteID string, version int) (*NoteVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions := m.versions[noteID]
	for _, v := range versions {
		if v.Version == version {
			return v, nil
		}
	}
	return nil, fmt.Errorf("version %d not found for note %s", version, noteID)
}

// RestoreNoteVersion 恢复笔记到指定版本.
func (m *Manager) RestoreNoteVersion(noteID string, version int) (*Note, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	note, ok := m.notes[noteID]
	if !ok {
		return nil, fmt.Errorf("note %q not found", noteID)
	}

	versions := m.versions[noteID]
	var targetVersion *NoteVersion
	for _, v := range versions {
		if v.Version == version {
			targetVersion = v
			break
		}
	}

	if targetVersion == nil {
		return nil, fmt.Errorf("version %d not found for note %s", version, noteID)
	}

	note.Title = targetVersion.Title
	note.Content = targetVersion.Content
	note.WordCount = countWords(targetVersion.Content)
	note.UpdatedAt = time.Now()
	note.Version++

	// 保存新版本
	m.saveVersion(note)

	log.Printf("[notes] 恢复笔记 %s 到版本 %d", noteID, version)
	return note, nil
}

// saveVersion 保存笔记版本.
func (m *Manager) saveVersion(note *Note) {
	version := &NoteVersion{
		ID:        uuid.New().String(),
		NoteID:    note.ID,
		Version:   note.Version,
		Title:     note.Title,
		Content:   note.Content,
		Author:    note.Author,
		CreatedAt: time.Now(),
	}

	m.versions[note.ID] = append(m.versions[note.ID], version)

	// 限制版本数量，保留最近50个版本
	versions := m.versions[note.ID]
	if len(versions) > 50 {
		m.versions[note.ID] = versions[len(versions)-50:]
	}
}

// ========== 辅助函数 ==========

// countWords 统计字数.
func countWords(content string) int {
	if content == "" {
		return 0
	}
	return len(strings.Fields(content))
}

// generateToken 生成分享 token.
func generateToken() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// GetStats 获取笔记统计信息.
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pinnedCount := 0
	favoriteCount := 0
	publicCount := 0
	totalWords := 0

	for _, note := range m.notes {
		if note.IsPinned {
			pinnedCount++
		}
		if note.IsFavorite {
			favoriteCount++
		}
		if note.IsPublic {
			publicCount++
		}
		totalWords += note.WordCount
	}

	return map[string]interface{}{
		"total_notes":     len(m.notes),
		"total_notebooks": len(m.notebooks),
		"total_tags":      len(m.tags),
		"pinned_notes":    pinnedCount,
		"favorite_notes":  favoriteCount,
		"public_notes":    publicCount,
		"share_links":     len(m.shareLinks),
		"total_words":     totalWords,
	}
}
