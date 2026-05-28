package collaborativenotes

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// NoteStatus 笔记状态
type NoteStatus string

const (
	StatusDraft    NoteStatus = "draft"
	StatusPublished NoteStatus = "published"
	StatusArchived NoteStatus = "archived"
)

// ConflictResolution 冲突解决策略
type ConflictResolution string

const (
	ConflictLastWrite  ConflictResolution = "last_write_wins"
	ConflictFirstWrite ConflictResolution = "first_write_wins"
	ConflictManual     ConflictResolution = "manual"
)

// ExportFormat 导出格式
type ExportFormat string

const (
	FormatMarkdown ExportFormat = "markdown"
	FormatHTML     ExportFormat = "html"
	FormatPDF      ExportFormat = "pdf"
)

// ImportFormat 导入格式
type ImportFormat string

const (
	ImportMarkdown ImportFormat = "markdown"
	ImportHTML     ImportFormat = "html"
)

// Note 笔记
type Note struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Content     string       `json:"content"`
	Markdown    string       `json:"markdown"`
	Status      NoteStatus   `json:"status"`
	NotebookID  string       `json:"notebook_id"`
	Tags        []string     `json:"tags,omitempty"`
	Author      string       `json:"author"`
	Collaborators []string   `json:"collaborators,omitempty"`
	IsPinned    bool         `json:"is_pinned"`
	IsFavorite  bool         `json:"is_favorite"`
	Version     int          `json:"version"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	UpdatedBy   string       `json:"updated_by"`
}

// Notebook 笔记本
type Notebook struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	OwnerID     string    `json:"owner_id"`
	ParentID    string    `json:"parent_id,omitempty"` // 支持嵌套笔记本
	Color       string    `json:"color,omitempty"`
	Icon        string    `json:"icon,omitempty"`
	IsShared    bool      `json:"is_shared"`
	SharedWith  []string  `json:"shared_with,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Tag 标签
type Tag struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color,omitempty"`
	NoteCount int   `json:"note_count"`
}

// Version 版本记录
type Version struct {
	ID        string    `json:"id"`
	NoteID    string    `json:"note_id"`
	Version   int       `json:"version"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Markdown  string    `json:"markdown"`
	Author    string    `json:"author"`
	Message   string    `json:"message,omitempty"` // 版本说明
	CreatedAt time.Time `json:"created_at"`
}

// EditOperation 编辑操作（用于OT冲突解决）
type EditOperation struct {
	ID        string    `json:"id"`
	NoteID    string    `json:"note_id"`
	UserID    string    `json:"user_id"`
	Type      string    `json:"type"` // insert, delete, replace
	Position  int       `json:"position"`
	Content   string    `json:"content"`
	Length    int       `json:"length,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Version   int       `json:"version"`
}

// CollaborationSession 协作会话
type CollaborationSession struct {
	ID        string    `json:"id"`
	NoteID    string    `json:"note_id"`
	UserID    string    `json:"user_id"`
	StartedAt time.Time `json:"started_at"`
	LastPing  time.Time `json:"last_ping"`
	IsActive  bool      `json:"is_active"`
}

// SearchResult 搜索结果
type SearchResult struct {
	NoteID    string  `json:"note_id"`
	Title     string  `json:"title"`
	Content   string  `json:"content_snippet"`
	Notebook  string  `json:"notebook_name"`
	Tags      []string `json:"tags,omitempty"`
	Score     float64 `json:"score"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NoteStats 笔记统计
type NoteStats struct {
	TotalNotes      int            `json:"total_notes"`
	NotesByStatus   map[string]int `json:"notes_by_status"`
	NotesByNotebook map[string]int `json:"notes_by_notebook"`
	TotalVersions   int            `json:"total_versions"`
	TotalTags       int            `json:"total_tags"`
	ActiveCollabs   int            `json:"active_collaborations"`
}

// CollaborativeNotes 协作笔记系统
type CollaborativeNotes struct {
	mu              sync.RWMutex
	notes           map[string]*Note
	notebooks       map[string]*Notebook
	tags            map[string]*Tag
	versions        map[string][]*Version        // noteID -> versions
	operations      map[string][]*EditOperation  // noteID -> operations
	sessions        map[string]*CollaborationSession
	noteTags        map[string][]string          // noteID -> tagIDs
	conflictRes     ConflictResolution
}

// NewCollaborativeNotes 创建协作笔记系统
func NewCollaborativeNotes() *CollaborativeNotes {
	return &CollaborativeNotes{
		notes:        make(map[string]*Note),
		notebooks:    make(map[string]*Notebook),
		tags:         make(map[string]*Tag),
		versions:     make(map[string][]*Version),
		operations:   make(map[string][]*EditOperation),
		sessions:     make(map[string]*CollaborationSession),
		noteTags:     make(map[string][]string),
		conflictRes:  ConflictLastWrite,
	}
}

// ==================== 笔记本管理 ====================

// CreateNotebook 创建笔记本
func (cn *CollaborativeNotes) CreateNotebook(notebook *Notebook) error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	if _, exists := cn.notebooks[notebook.ID]; exists {
		return fmt.Errorf("笔记本 %s 已存在", notebook.ID)
	}

	// 验证父笔记本存在
	if notebook.ParentID != "" {
		if _, exists := cn.notebooks[notebook.ParentID]; !exists {
			return fmt.Errorf("父笔记本 %s 不存在", notebook.ParentID)
		}
	}

	notebook.CreatedAt = time.Now()
	notebook.UpdatedAt = time.Now()

	cn.notebooks[notebook.ID] = notebook
	return nil
}

// GetNotebook 获取笔记本
func (cn *CollaborativeNotes) GetNotebook(notebookID string) (*Notebook, error) {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	notebook, exists := cn.notebooks[notebookID]
	if !exists {
		return nil, fmt.Errorf("笔记本 %s 不存在", notebookID)
	}
	return notebook, nil
}

// UpdateNotebook 更新笔记本
func (cn *CollaborativeNotes) UpdateNotebook(notebook *Notebook) error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	if _, exists := cn.notebooks[notebook.ID]; !exists {
		return fmt.Errorf("笔记本 %s 不存在", notebook.ID)
	}

	notebook.UpdatedAt = time.Now()
	cn.notebooks[notebook.ID] = notebook
	return nil
}

// DeleteNotebook 删除笔记本
func (cn *CollaborativeNotes) DeleteNotebook(notebookID string) error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	if _, exists := cn.notebooks[notebookID]; !exists {
		return fmt.Errorf("笔记本 %s 不存在", notebookID)
	}

	// 检查是否有子笔记本
	for _, nb := range cn.notebooks {
		if nb.ParentID == notebookID {
			return fmt.Errorf("笔记本 %s 包含子笔记本，无法删除", notebookID)
		}
	}

	// 检查是否有笔记
	for _, note := range cn.notes {
		if note.NotebookID == notebookID {
			return fmt.Errorf("笔记本 %s 包含笔记，无法删除", notebookID)
		}
	}

	delete(cn.notebooks, notebookID)
	return nil
}

// ListNotebooks 列出笔记本
func (cn *CollaborativeNotes) ListNotebooks(userID string, parentID string) []*Notebook {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	notebooks := make([]*Notebook, 0)
	for _, notebook := range cn.notebooks {
		// 检查权限：所有者或被分享者
		hasAccess := notebook.OwnerID == userID || contains(notebook.SharedWith, userID)
		if !hasAccess {
			continue
		}

		if parentID == "" && notebook.ParentID == "" {
			notebooks = append(notebooks, notebook)
		} else if parentID != "" && notebook.ParentID == parentID {
			notebooks = append(notebooks, notebook)
		}
	}

	sort.Slice(notebooks, func(i, j int) bool {
		return notebooks[i].Name < notebooks[j].Name
	})

	return notebooks
}

// ShareNotebook 分享笔记本
func (cn *CollaborativeNotes) ShareNotebook(notebookID string, userID string) error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	notebook, exists := cn.notebooks[notebookID]
	if !exists {
		return fmt.Errorf("笔记本 %s 不存在", notebookID)
	}

	if contains(notebook.SharedWith, userID) {
		return fmt.Errorf("用户 %s 已有访问权限", userID)
	}

	notebook.IsShared = true
	notebook.SharedWith = append(notebook.SharedWith, userID)
	notebook.UpdatedAt = time.Now()

	return nil
}

// ==================== 笔记管理 ====================

// CreateNote 创建笔记
func (cn *CollaborativeNotes) CreateNote(note *Note) error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	if _, exists := cn.notes[note.ID]; exists {
		return fmt.Errorf("笔记 %s 已存在", note.ID)
	}

	// 验证笔记本存在
	if note.NotebookID != "" {
		if _, exists := cn.notebooks[note.NotebookID]; !exists {
			return fmt.Errorf("笔记本 %s 不存在", note.NotebookID)
		}
	}

	note.Status = StatusDraft
	note.Version = 1
	note.CreatedAt = time.Now()
	note.UpdatedAt = time.Now()

	cn.notes[note.ID] = note

	// 创建初始版本
	cn.createVersion(note, "创建笔记", note.Author)

	// 更新标签计数
	for _, tagID := range note.Tags {
		if tag, exists := cn.tags[tagID]; exists {
			tag.NoteCount++
		}
		cn.noteTags[note.ID] = append(cn.noteTags[note.ID], tagID)
	}

	return nil
}

// GetNote 获取笔记
func (cn *CollaborativeNotes) GetNote(noteID string) (*Note, error) {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	note, exists := cn.notes[noteID]
	if !exists {
		return nil, fmt.Errorf("笔记 %s 不存在", noteID)
	}
	return note, nil
}

// UpdateNote 更新笔记（支持协作编辑）
func (cn *CollaborativeNotes) UpdateNote(note *Note, author string, message string) error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	existing, exists := cn.notes[note.ID]
	if !exists {
		return fmt.Errorf("笔记 %s 不存在", note.ID)
	}

	// 版本冲突检测
	if note.Version != existing.Version {
		switch cn.conflictRes {
		case ConflictLastWrite:
			// 最后写入胜出，继续更新
		case ConflictFirstWrite:
			return fmt.Errorf("版本冲突：笔记已被其他用户修改，请刷新后重试")
		case ConflictManual:
			return fmt.Errorf("版本冲突：需要手动解决冲突")
		}
	}

	// 保留原有字段，只更新提供的字段
	if note.Title != "" {
		existing.Title = note.Title
	}
	if note.Content != "" {
		existing.Content = note.Content
	}
	if note.Markdown != "" {
		existing.Markdown = note.Markdown
	}
	if note.Status != "" {
		existing.Status = note.Status
	}
	if note.NotebookID != "" {
		existing.NotebookID = note.NotebookID
	}
	if note.Tags != nil {
		existing.Tags = note.Tags
	}

	existing.Version = existing.Version + 1
	existing.UpdatedAt = time.Now()
	existing.UpdatedBy = author

	// 更新传入的note对象，以便调用者获取最新版本
	note.Version = existing.Version
	note.UpdatedAt = existing.UpdatedAt
	note.UpdatedBy = existing.UpdatedBy
	note.Author = existing.Author
	note.Collaborators = existing.Collaborators
	note.IsPinned = existing.IsPinned
	note.IsFavorite = existing.IsFavorite
	note.CreatedAt = existing.CreatedAt

	// 创建版本记录
	cn.createVersion(existing, message, author)

	// 更新标签计数
	cn.updateNoteTags(existing)

	return nil
}

// DeleteNote 删除笔记
func (cn *CollaborativeNotes) DeleteNote(noteID string) error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	if _, exists := cn.notes[noteID]; !exists {
		return fmt.Errorf("笔记 %s 不存在", noteID)
	}

	// 清理标签计数
	if tagIDs, exists := cn.noteTags[noteID]; exists {
		for _, tagID := range tagIDs {
			if tag, exists := cn.tags[tagID]; exists {
				tag.NoteCount--
			}
		}
		delete(cn.noteTags, noteID)
	}

	delete(cn.notes, noteID)
	delete(cn.versions, noteID)
	delete(cn.operations, noteID)

	return nil
}

// ListNotes 列出笔记
func (cn *CollaborativeNotes) ListNotes(notebookID string, status NoteStatus, author string) []*Note {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	notes := make([]*Note, 0)
	for _, note := range cn.notes {
		if notebookID != "" && note.NotebookID != notebookID {
			continue
		}
		if status != "" && note.Status != status {
			continue
		}
		if author != "" && note.Author != author {
			continue
		}
		notes = append(notes, note)
	}

	sort.Slice(notes, func(i, j int) bool {
		return notes[i].UpdatedAt.After(notes[j].UpdatedAt)
	})

	return notes
}

// MoveNote 移动笔记到其他笔记本
func (cn *CollaborativeNotes) MoveNote(noteID string, notebookID string) error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	note, exists := cn.notes[noteID]
	if !exists {
		return fmt.Errorf("笔记 %s 不存在", noteID)
	}

	if notebookID != "" {
		if _, exists := cn.notebooks[notebookID]; !exists {
			return fmt.Errorf("笔记本 %s 不存在", notebookID)
		}
	}

	note.NotebookID = notebookID
	note.UpdatedAt = time.Now()

	return nil
}

// PinNote 置顶笔记
func (cn *CollaborativeNotes) PinNote(noteID string) error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	note, exists := cn.notes[noteID]
	if !exists {
		return fmt.Errorf("笔记 %s 不存在", noteID)
	}

	note.IsPinned = !note.IsPinned
	note.UpdatedAt = time.Now()

	return nil
}

// FavoriteNote 收藏笔记
func (cn *CollaborativeNotes) FavoriteNote(noteID string) error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	note, exists := cn.notes[noteID]
	if !exists {
		return fmt.Errorf("笔记 %s 不存在", noteID)
	}

	note.IsFavorite = !note.IsFavorite
	note.UpdatedAt = time.Now()

	return nil
}

// ==================== 标签管理 ====================

// CreateTag 创建标签
func (cn *CollaborativeNotes) CreateTag(tag *Tag) error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	if _, exists := cn.tags[tag.ID]; exists {
		return fmt.Errorf("标签 %s 已存在", tag.ID)
	}

	tag.NoteCount = 0
	cn.tags[tag.ID] = tag

	return nil
}

// GetTag 获取标签
func (cn *CollaborativeNotes) GetTag(tagID string) (*Tag, error) {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	tag, exists := cn.tags[tagID]
	if !exists {
		return nil, fmt.Errorf("标签 %s 不存在", tagID)
	}
	return tag, nil
}

// UpdateTag 更新标签
func (cn *CollaborativeNotes) UpdateTag(tag *Tag) error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	if _, exists := cn.tags[tag.ID]; !exists {
		return fmt.Errorf("标签 %s 不存在", tag.ID)
	}

	cn.tags[tag.ID] = tag
	return nil
}

// DeleteTag 删除标签
func (cn *CollaborativeNotes) DeleteTag(tagID string) error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	if _, exists := cn.tags[tagID]; !exists {
		return fmt.Errorf("标签 %s 不存在", tagID)
	}

	delete(cn.tags, tagID)

	// 从所有笔记中移除该标签
	for noteID, tagIDs := range cn.noteTags {
		newTags := make([]string, 0)
		for _, tid := range tagIDs {
			if tid != tagID {
				newTags = append(newTags, tid)
			}
		}
		cn.noteTags[noteID] = newTags
	}

	return nil
}

// ListTags 列出所有标签
func (cn *CollaborativeNotes) ListTags() []*Tag {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	tags := make([]*Tag, 0)
	for _, tag := range cn.tags {
		tags = append(tags, tag)
	}

	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})

	return tags
}

// AddNoteTag 为笔记添加标签
func (cn *CollaborativeNotes) AddNoteTag(noteID string, tagID string) error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	if _, exists := cn.notes[noteID]; !exists {
		return fmt.Errorf("笔记 %s 不存在", noteID)
	}

	if _, exists := cn.tags[tagID]; !exists {
		return fmt.Errorf("标签 %s 不存在", tagID)
	}

	if contains(cn.noteTags[noteID], tagID) {
		return nil // 已存在，忽略
	}

	cn.noteTags[noteID] = append(cn.noteTags[noteID], tagID)
	cn.tags[tagID].NoteCount++

	return nil
}

// RemoveNoteTag 移除笔记标签
func (cn *CollaborativeNotes) RemoveNoteTag(noteID string, tagID string) error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	if _, exists := cn.notes[noteID]; !exists {
		return fmt.Errorf("笔记 %s 不存在", noteID)
	}

	tagIDs := cn.noteTags[noteID]
	newTags := make([]string, 0)
	found := false
	for _, tid := range tagIDs {
		if tid != tagID {
			newTags = append(newTags, tid)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("笔记 %s 没有标签 %s", noteID, tagID)
	}

	cn.noteTags[noteID] = newTags
	if tag, exists := cn.tags[tagID]; exists {
		tag.NoteCount--
	}

	return nil
}

// GetNoteTags 获取笔记的标签
func (cn *CollaborativeNotes) GetNoteTags(noteID string) []*Tag {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	tags := make([]*Tag, 0)
	for _, tagID := range cn.noteTags[noteID] {
		if tag, exists := cn.tags[tagID]; exists {
			tags = append(tags, tag)
		}
	}

	return tags
}

// ==================== 版本历史 ====================

// GetVersions 获取笔记的版本历史
func (cn *CollaborativeNotes) GetVersions(noteID string) ([]*Version, error) {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	if _, exists := cn.notes[noteID]; !exists {
		return nil, fmt.Errorf("笔记 %s 不存在", noteID)
	}

	versions := cn.versions[noteID]
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Version > versions[j].Version
	})

	return versions, nil
}

// GetVersion 获取特定版本
func (cn *CollaborativeNotes) GetVersion(noteID string, version int) (*Version, error) {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	if _, exists := cn.notes[noteID]; !exists {
		return nil, fmt.Errorf("笔记 %s 不存在", noteID)
	}

	for _, v := range cn.versions[noteID] {
		if v.Version == version {
			return v, nil
		}
	}

	return nil, fmt.Errorf("版本 %d 不存在", version)
}

// RevertToVersion 回滚到指定版本
func (cn *CollaborativeNotes) RevertToVersion(noteID string, version int, author string) error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	note, exists := cn.notes[noteID]
	if !exists {
		return fmt.Errorf("笔记 %s 不存在", noteID)
	}

	var targetVersion *Version
	for _, v := range cn.versions[noteID] {
		if v.Version == version {
			targetVersion = v
			break
		}
	}

	if targetVersion == nil {
		return fmt.Errorf("版本 %d 不存在", version)
	}

	note.Title = targetVersion.Title
	note.Content = targetVersion.Content
	note.Markdown = targetVersion.Markdown
	note.Version++
	note.UpdatedAt = time.Now()
	note.UpdatedBy = author

	cn.createVersion(note, fmt.Sprintf("回滚到版本 %d", version), author)

	return nil
}

// createVersion 创建版本记录（内部方法，需要锁）
func (cn *CollaborativeNotes) createVersion(note *Note, message string, author string) {
	version := &Version{
		ID:        fmt.Sprintf("%s-v%d", note.ID, note.Version),
		NoteID:    note.ID,
		Version:   note.Version,
		Title:     note.Title,
		Content:   note.Content,
		Markdown:  note.Markdown,
		Author:    author,
		Message:   message,
		CreatedAt: time.Now(),
	}

	cn.versions[note.ID] = append(cn.versions[note.ID], version)
}

// updateNoteTags 更新笔记标签（内部方法，需要锁）
func (cn *CollaborativeNotes) updateNoteTags(note *Note) {
	// 移除旧标签计数
	if oldTags, exists := cn.noteTags[note.ID]; exists {
		for _, tagID := range oldTags {
			if tag, exists := cn.tags[tagID]; exists {
				tag.NoteCount--
			}
		}
	}

	// 设置新标签
	cn.noteTags[note.ID] = note.Tags
	for _, tagID := range note.Tags {
		if tag, exists := cn.tags[tagID]; exists {
			tag.NoteCount++
		}
	}
}

// ==================== 协作功能 ====================

// StartCollaboration 开始协作会话
func (cn *CollaborativeNotes) StartCollaboration(noteID string, userID string) (*CollaborationSession, error) {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	if _, exists := cn.notes[noteID]; !exists {
		return nil, fmt.Errorf("笔记 %s 不存在", noteID)
	}

	session := &CollaborationSession{
		ID:        fmt.Sprintf("%s-%s-%d", noteID, userID, time.Now().UnixNano()),
		NoteID:    noteID,
		UserID:    userID,
		StartedAt: time.Now(),
		LastPing:  time.Now(),
		IsActive:  true,
	}

	cn.sessions[session.ID] = session

	// 添加到笔记的协作者列表
	note := cn.notes[noteID]
	if !contains(note.Collaborators, userID) {
		note.Collaborators = append(note.Collaborators, userID)
	}

	return session, nil
}

// EndCollaboration 结束协作会话
func (cn *CollaborativeNotes) EndCollaboration(sessionID string) error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	session, exists := cn.sessions[sessionID]
	if !exists {
		return fmt.Errorf("协作会话 %s 不存在", sessionID)
	}

	session.IsActive = false

	// 从笔记的协作者列表中移除
	note, noteExists := cn.notes[session.NoteID]
	if noteExists {
		newCollabs := make([]string, 0)
		for _, collab := range note.Collaborators {
			if collab != session.UserID {
				newCollabs = append(newCollabs, collab)
			}
		}
		note.Collaborators = newCollabs
	}

	return nil
}

// PingCollaboration 心跳保活
func (cn *CollaborativeNotes) PingCollaboration(sessionID string) error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	session, exists := cn.sessions[sessionID]
	if !exists {
		return fmt.Errorf("协作会话 %s 不存在", sessionID)
	}

	if !session.IsActive {
		return fmt.Errorf("协作会话 %s 已结束", sessionID)
	}

	session.LastPing = time.Now()
	return nil
}

// GetActiveCollaborators 获取笔记的活跃协作者
func (cn *CollaborativeNotes) GetActiveCollaborators(noteID string) []string {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	activeUsers := make(map[string]bool)
	for _, session := range cn.sessions {
		if session.NoteID == noteID && session.IsActive {
			// 检查会话是否超时（5分钟无心跳）
			if time.Since(session.LastPing) < 5*time.Minute {
				activeUsers[session.UserID] = true
			}
		}
	}

	users := make([]string, 0)
	for userID := range activeUsers {
		users = append(users, userID)
	}

	return users
}

// SubmitEdit 提交编辑操作（OT冲突解决）
func (cn *CollaborativeNotes) SubmitEdit(op *EditOperation) error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	note, exists := cn.notes[op.NoteID]
	if !exists {
		return fmt.Errorf("笔记 %s 不存在", op.NoteID)
	}

	op.Timestamp = time.Now()
	op.Version = note.Version

	cn.operations[op.NoteID] = append(cn.operations[op.NoteID], op)

	// 应用操作到笔记内容
	switch op.Type {
	case "insert":
		if op.Position <= len(note.Content) {
			note.Content = note.Content[:op.Position] + op.Content + note.Content[op.Position:]
		}
	case "delete":
		if op.Position+op.Length <= len(note.Content) {
			note.Content = note.Content[:op.Position] + note.Content[op.Position+op.Length:]
		}
	case "replace":
		if op.Position+op.Length <= len(note.Content) {
			note.Content = note.Content[:op.Position] + op.Content + note.Content[op.Position+op.Length:]
		}
	}

	note.Version++
	note.UpdatedAt = time.Now()
	note.UpdatedBy = op.UserID

	return nil
}

// GetEditOperations 获取笔记的编辑操作历史
func (cn *CollaborativeNotes) GetEditOperations(noteID string) []*EditOperation {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	return cn.operations[noteID]
}

// SetConflictResolution 设置冲突解决策略
func (cn *CollaborativeNotes) SetConflictResolution(resolution ConflictResolution) {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	cn.conflictRes = resolution
}

// ==================== 全文搜索 ====================

// SearchNotes 全文搜索笔记
func (cn *CollaborativeNotes) SearchNotes(query string, userID string) []*SearchResult {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	results := make([]*SearchResult, 0)
	query = strings.ToLower(query)

	for _, note := range cn.notes {
		// 检查访问权限
		if note.Author != userID && !contains(note.Collaborators, userID) {
			// 检查笔记本是否共享给该用户
			notebook, exists := cn.notebooks[note.NotebookID]
			if !exists || (notebook.OwnerID != userID && !contains(notebook.SharedWith, userID)) {
				continue
			}
		}

		score := 0.0
		snippet := ""

		// 标题匹配
		titleLower := strings.ToLower(note.Title)
		if strings.Contains(titleLower, query) {
			score += 10.0
			snippet = note.Title
		}

		// 内容匹配
		contentLower := strings.ToLower(note.Content)
		if idx := strings.Index(contentLower, query); idx != -1 {
			score += 5.0
			start := idx - 50
			if start < 0 {
				start = 0
			}
			end := idx + len(query) + 50
			if end > len(note.Content) {
				end = len(note.Content)
			}
			snippet = "..." + note.Content[start:end] + "..."
		}

		// 标签匹配
		for _, tagID := range cn.noteTags[note.ID] {
			if tag, exists := cn.tags[tagID]; exists {
				if strings.Contains(strings.ToLower(tag.Name), query) {
					score += 3.0
				}
			}
		}

		if score > 0 {
			notebookName := ""
			if nb, exists := cn.notebooks[note.NotebookID]; exists {
				notebookName = nb.Name
			}

			results = append(results, &SearchResult{
				NoteID:    note.ID,
				Title:     note.Title,
				Content:   snippet,
				Notebook:  notebookName,
				Tags:      cn.noteTags[note.ID],
				Score:     score,
				UpdatedAt: note.UpdatedAt,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// ==================== 导入导出 ====================

// ExportNote 导出笔记
func (cn *CollaborativeNotes) ExportNote(noteID string, format ExportFormat) (string, error) {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	note, exists := cn.notes[noteID]
	if !exists {
		return "", fmt.Errorf("笔记 %s 不存在", noteID)
	}

	switch format {
	case FormatMarkdown:
		return cn.exportMarkdown(note), nil
	case FormatHTML:
		return cn.exportHTML(note), nil
	case FormatPDF:
		// PDF导出需要外部库，这里返回简化版本
		return cn.exportPDFPlaceholder(note), nil
	default:
		return "", fmt.Errorf("不支持的导出格式: %s", format)
	}
}

// ImportNote 导入笔记
func (cn *CollaborativeNotes) ImportNote(notebookID string, content string, format ImportFormat, author string) (*Note, error) {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	if notebookID != "" {
		if _, exists := cn.notebooks[notebookID]; !exists {
			return nil, fmt.Errorf("笔记本 %s 不存在", notebookID)
		}
	}

	note := &Note{
		ID:         fmt.Sprintf("imported-%d", time.Now().UnixNano()),
		NotebookID: notebookID,
		Author:     author,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	switch format {
	case ImportMarkdown:
		cn.importMarkdown(note, content)
	case ImportHTML:
		cn.importHTML(note, content)
	default:
		return nil, fmt.Errorf("不支持的导入格式: %s", format)
	}

	cn.notes[note.ID] = note
	cn.createVersion(note, "导入笔记", author)

	return note, nil
}

// exportMarkdown 导出为Markdown
func (cn *CollaborativeNotes) exportMarkdown(note *Note) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n", note.Title))
	sb.WriteString(note.Markdown)

	if len(note.Tags) > 0 {
		sb.WriteString("\n\n---\n")
		sb.WriteString("标签: ")
		for i, tagID := range note.Tags {
			if tag, exists := cn.tags[tagID]; exists {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(tag.Name)
			}
		}
	}

	return sb.String()
}

// exportHTML 导出为HTML
func (cn *CollaborativeNotes) exportHTML(note *Note) string {
	var sb strings.Builder

	sb.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	sb.WriteString(fmt.Sprintf("<title>%s</title>\n", note.Title))
	sb.WriteString("<meta charset=\"UTF-8\">\n")
	sb.WriteString("</head>\n<body>\n")
	sb.WriteString(fmt.Sprintf("<h1>%s</h1>\n", note.Title))
	sb.WriteString(fmt.Sprintf("<div>%s</div>\n", note.Content))
	sb.WriteString("</body>\n</html>")

	return sb.String()
}

// exportPDFPlaceholder PDF导出占位符
func (cn *CollaborativeNotes) exportPDFPlaceholder(note *Note) string {
	return fmt.Sprintf("[PDF Export] Title: %s\nContent: %s", note.Title, note.Content)
}

// importMarkdown 从Markdown导入
func (cn *CollaborativeNotes) importMarkdown(note *Note, content string) {
	lines := strings.SplitN(content, "\n", 2)

	// 提取标题
	if len(lines) > 0 && strings.HasPrefix(lines[0], "# ") {
		note.Title = strings.TrimPrefix(lines[0], "# ")
		if len(lines) > 1 {
			note.Markdown = lines[1]
			note.Content = lines[1]
		}
	} else {
		note.Title = "未命名笔记"
		note.Markdown = content
		note.Content = content
	}

	note.Status = StatusDraft
	note.Version = 1
}

// importHTML 从HTML导入
func (cn *CollaborativeNotes) importHTML(note *Note, content string) {
	// 简化的HTML解析
	note.Title = "导入的HTML笔记"
	note.Content = content
	note.Markdown = content
	note.Status = StatusDraft
	note.Version = 1
}

// ==================== 统计 ====================

// GetStats 获取统计信息
func (cn *CollaborativeNotes) GetStats(userID string) *NoteStats {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	stats := &NoteStats{
		NotesByStatus:   make(map[string]int),
		NotesByNotebook: make(map[string]int),
	}

	for _, note := range cn.notes {
		if note.Author == userID || contains(note.Collaborators, userID) {
			stats.TotalNotes++
			stats.NotesByStatus[string(note.Status)]++

			if nb, exists := cn.notebooks[note.NotebookID]; exists {
				stats.NotesByNotebook[nb.Name]++
			}
		}
	}

	// 统计版本数
	for _, versions := range cn.versions {
		stats.TotalVersions += len(versions)
	}

	stats.TotalTags = len(cn.tags)

	// 统计活跃协作者
	for _, session := range cn.sessions {
		if session.IsActive && time.Since(session.LastPing) < 5*time.Minute {
			stats.ActiveCollabs++
		}
	}

	return stats
}

// ==================== 辅助函数 ====================

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
