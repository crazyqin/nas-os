// Package notestation 提供笔记管理核心业务逻辑
package notestation

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 笔记管理器.
type Manager struct {
	notes     map[string]*Note
	notebooks map[string]*Notebook
	mu        sync.RWMutex
}

// NewManager 创建笔记管理器.
func NewManager() *Manager {
	return &Manager{
		notes:     make(map[string]*Note),
		notebooks: make(map[string]*Notebook),
	}
}

// ========== Note CRUD ==========

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
		Tags:       req.Tags,
		CreatedAt:  now,
		UpdatedAt:  now,
		IsPinned:   false,
		IsFavorite: false,
		NotebookID: req.NotebookID,
	}

	if note.Tags == nil {
		note.Tags = []string{}
	}

	m.notes[note.ID] = note
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

	note.UpdatedAt = time.Now()
	return note, nil
}

// DeleteNote 删除笔记.
func (m *Manager) DeleteNote(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.notes[id]; !ok {
		return fmt.Errorf("note %q not found", id)
	}
	delete(m.notes, id)
	return nil
}

// ========== Notebook CRUD ==========

// CreateNotebook 创建笔记本.
func (m *Manager) CreateNotebook(req CreateNotebookRequest) *Notebook {
	m.mu.Lock()
	defer m.mu.Unlock()

	nb := &Notebook{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   time.Now(),
		Owner:       req.Owner,
	}

	m.notebooks[nb.ID] = nb
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
		return nbs[i].CreatedAt.After(nbs[j].CreatedAt)
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

	return nb, nil
}

// DeleteNotebook 删除笔记本及其下所有笔记.
func (m *Manager) DeleteNotebook(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.notebooks[id]; !ok {
		return fmt.Errorf("notebook %q not found", id)
	}

	// 删除该笔记本下的所有笔记
	for nid, note := range m.notes {
		if note.NotebookID == id {
			delete(m.notes, nid)
		}
	}

	delete(m.notebooks, id)
	return nil
}

// ========== 标签管理 ==========

// ListNotesByTag 按标签过滤笔记.
func (m *Manager) ListNotesByTag(tag string) []*Note {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Note
	for _, n := range m.notes {
		for _, t := range n.Tags {
			if t == tag {
				result = append(result, n)
				break
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})

	return result
}

// GetTagStats 获取标签统计.
func (m *Manager) GetTagStats() []TagStat {
	m.mu.RLock()
	defer m.mu.RUnlock()

	counts := make(map[string]int)
	for _, n := range m.notes {
		for _, tag := range n.Tags {
			counts[tag]++
		}
	}

	stats := make([]TagStat, 0, len(counts))
	for tag, count := range counts {
		stats = append(stats, TagStat{Tag: tag, Count: count})
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Count > stats[j].Count
	})

	return stats
}

// ========== 搜索 ==========

// SearchNotes 全文搜索笔记标题和内容.
func (m *Manager) SearchNotes(query string) []*Note {
	m.mu.RLock()
	defer m.mu.RUnlock()

	q := strings.ToLower(query)
	var result []*Note

	for _, n := range m.notes {
		if strings.Contains(strings.ToLower(n.Title), q) ||
			strings.Contains(strings.ToLower(n.Content), q) {
			result = append(result, n)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})

	return result
}

// ========== 收藏/置顶 ==========

// GetPinnedNotes 获取置顶笔记.
func (m *Manager) GetPinnedNotes() []*Note {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Note
	for _, n := range m.notes {
		if n.IsPinned {
			result = append(result, n)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})

	return result
}

// GetFavoriteNotes 获取收藏笔记.
func (m *Manager) GetFavoriteNotes() []*Note {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Note
	for _, n := range m.notes {
		if n.IsFavorite {
			result = append(result, n)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})

	return result
}

// ========== 导入/导出 ==========

// ImportMarkdown 导入 Markdown 文件为笔记.
func (m *Manager) ImportMarkdown(req ImportRequest) *Note {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	// 从文件名推导标题
	title := req.Filename
	if strings.HasSuffix(strings.ToLower(title), ".md") {
		title = title[:len(title)-3]
	}

	note := &Note{
		ID:         uuid.New().String(),
		Title:      title,
		Content:    req.Content,
		Author:     req.Author,
		Tags:       []string{},
		CreatedAt:  now,
		UpdatedAt:  now,
		IsPinned:   false,
		IsFavorite: false,
		NotebookID: req.NotebookID,
	}

	m.notes[note.ID] = note
	return note
}

// ExportNotes 导出笔记为 Markdown 格式.
func (m *Manager) ExportNotes(noteIDs []string) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]string, len(noteIDs))
	for _, id := range noteIDs {
		note, ok := m.notes[id]
		if !ok {
			return nil, fmt.Errorf("note %q not found", id)
		}
		filename := note.Title + ".md"
		result[filename] = note.Content
	}

	return result, nil
}

// ========== 最近编辑列表 ==========

// GetRecentNotes 获取最近编辑的笔记.
func (m *Manager) GetRecentNotes(limit int) []*Note {
	m.mu.RLock()
	defer m.mu.RUnlock()

	notes := make([]*Note, 0, len(m.notes))
	for _, n := range m.notes {
		notes = append(notes, n)
	}

	sort.Slice(notes, func(i, j int) bool {
		return notes[i].UpdatedAt.After(notes[j].UpdatedAt)
	})

	if limit > 0 && limit < len(notes) {
		notes = notes[:limit]
	}

	return notes
}

// ListNotesByNotebook 列出指定笔记本下的笔记.
func (m *Manager) ListNotesByNotebook(notebookID string) []*Note {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Note
	for _, n := range m.notes {
		if n.NotebookID == notebookID {
			result = append(result, n)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})

	return result
}
