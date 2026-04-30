// Package notes Note Station 笔记系统
// 对标群晖 Note Station：笔记CRUD、笔记本管理、分享、全文搜索
package notes

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 笔记格式 ==========

// NoteFormat 笔记内容格式.
type NoteFormat string

const (
	FormatMarkdown NoteFormat = "markdown" // Markdown 格式
	FormatRichText NoteFormat = "richtext" // 富文本（HTML）
)

// ========== 笔记模型 ==========

// Note 笔记.
type Note struct {
	ID         string     `json:"id"`          // 笔记ID
	Title      string     `json:"title"`       // 标题
	Content    string     `json:"content"`     // 内容
	Format     NoteFormat `json:"format"`       // 内容格式
	NotebookID string     `json:"notebook_id"` // 所属笔记本ID
	Tags       []string   `json:"tags"`        // 标签
	Pinned     bool       `json:"pinned"`      // 是否置顶
	Favorite   bool       `json:"favorite"`    // 是否收藏
	OwnerID    string     `json:"owner_id"`    // 所有者ID
	OwnerName  string     `json:"owner_name"`  // 所有者名称
	WordCount  int        `json:"word_count"`  // 字数统计
	CreatedAt  time.Time  `json:"created_at"`  // 创建时间
	UpdatedAt  time.Time  `json:"updated_at"`  // 更新时间
	DeletedAt  *time.Time `json:"deleted_at"`  // 删除时间（回收站）
}

// NoteInput 创建/更新笔记输入.
type NoteInput struct {
	Title      string     `json:"title" binding:"required"`
	Content    string     `json:"content"`
	Format     NoteFormat `json:"format"`
	NotebookID string     `json:"notebook_id"`
	Tags       []string   `json:"tags"`
	Pinned     *bool      `json:"pinned"`
	Favorite   *bool      `json:"favorite"`
}

// ========== 笔记本模型 ==========

// Notebook 笔记本.
type Notebook struct {
	ID          string    `json:"id"`           // 笔记本ID
	Name        string    `json:"name"`         // 名称
	Description string    `json:"description"`  // 描述
	Color       string    `json:"color"`        // 颜色标识
	Icon        string    `json:"icon"`         // 图标
	OwnerID     string    `json:"owner_id"`     // 所有者ID
	NoteCount   int       `json:"note_count"`   // 笔记数
	SortOrder   int       `json:"sort_order"`   // 排序顺序
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`   // 更新时间
}

// NotebookInput 创建/更新笔记本输入.
type NotebookInput struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Color       string `json:"color"`
	Icon        string `json:"icon"`
	SortOrder   *int   `json:"sort_order"`
}

// ========== 分享模型 ==========

// NoteShare 笔记分享.
type NoteShare struct {
	ID         string     `json:"id"`          // 分享ID
	NoteID     string     `json:"note_id"`     // 笔记ID
	Token      string     `json:"token"`       // 分享Token（公开链接用）
	Password   string     `json:"password"`    // 访问密码（加密存储）
	HasPassword bool      `json:"has_password"` // 是否有密码保护
	Permission string     `json:"permission"`  // 权限: view, edit
	ExpiresAt  *time.Time `json:"expires_at"`  // 过期时间
	MaxViews   int        `json:"max_views"`   // 最大访问次数
	ViewCount  int        `json:"view_count"`  // 已访问次数
	CreatorID  string     `json:"creator_id"`  // 创建者ID
	CreatedAt  time.Time  `json:"created_at"`  // 创建时间
}

// ShareInput 创建分享输入.
type ShareInput struct {
	Permission string `json:"permission"` // view, edit
	Password   string `json:"password"`   // 访问密码（可选）
	ExpiresIn  int    `json:"expires_in"` // 有效期（小时），0表示永久
	MaxViews   int    `json:"max_views"`  // 最大访问次数，0表示无限
}

// ========== 搜索模型 ==========

// SearchQuery 搜索查询.
type SearchQuery struct {
	Keyword    string   `json:"keyword"`      // 关键词
	NotebookID string   `json:"notebook_id"` // 笔记本过滤
	Tags       []string `json:"tags"`         // 标签过滤
	Format     NoteFormat `json:"format"`     // 格式过滤
	Pinned     *bool    `json:"pinned"`       // 置顶过滤
	Favorite   *bool    `json:"favorite"`     // 收藏过滤
	Limit      int      `json:"limit"`        // 返回数量
	Offset     int      `json:"offset"`       // 偏移量
	SortBy     string   `json:"sort_by"`      // 排序字段: updated_at, created_at, title
	SortOrder  string   `json:"sort_order"`   // 排序顺序: asc, desc
}

// SearchResult 搜索结果.
type SearchResult struct {
	Notes   []*Note `json:"notes"`
	Total   int     `json:"total"`
	Limit   int     `json:"limit"`
	Offset  int     `json:"offset"`
	Keyword string  `json:"keyword"`
}

// ========== 错误定义 ==========

var (
	ErrNoteNotFound     = errors.New("笔记不存在")
	ErrNotebookNotFound = errors.New("笔记本不存在")
	ErrShareNotFound    = errors.New("分享不存在")
	ErrShareExpired     = errors.New("分享已过期")
	ErrShareMaxViews    = errors.New("分享访问次数已达上限")
	ErrPasswordRequired = errors.New("需要访问密码")
	ErrPasswordWrong    = errors.New("密码错误")
	ErrNoteInTrash      = errors.New("笔记在回收站中")
	ErrInvalidFormat    = errors.New("无效的笔记格式")
)

// ========== Store 存储 ==========

// Store 笔记存储.
type Store struct {
	mu        sync.RWMutex
	notes     map[string]*Note           // noteID -> Note
	notebooks map[string]*Notebook       // notebookID -> Notebook
	shares    map[string]*NoteShare      // shareID -> NoteShare
	tagIndex  map[string]map[string]bool // tag -> noteID set
}

// NewStore 创建存储.
func NewStore() *Store {
	return &Store{
		notes:     make(map[string]*Note),
		notebooks: make(map[string]*Notebook),
		shares:    make(map[string]*NoteShare),
		tagIndex:  make(map[string]map[string]bool),
	}
}

// ========== 笔记 CRUD ==========

// CreateNote 创建笔记.
func (s *Store) CreateNote(input NoteInput, ownerID, ownerName string) (*Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if input.Title == "" {
		return nil, errors.New("标题不能为空")
	}

	// 验证笔记本存在
	if input.NotebookID != "" {
		if _, exists := s.notebooks[input.NotebookID]; !exists {
			return nil, ErrNotebookNotFound
		}
	}

	format := input.Format
	if format == "" {
		format = FormatMarkdown
	}
	if format != FormatMarkdown && format != FormatRichText {
		return nil, ErrInvalidFormat
	}

	now := time.Now()
	note := &Note{
		ID:         generateID(),
		Title:      input.Title,
		Content:    input.Content,
		Format:     format,
		NotebookID: input.NotebookID,
		Tags:       input.Tags,
		OwnerID:    ownerID,
		OwnerName:  ownerName,
		WordCount:  countWords(input.Content),
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if input.Pinned != nil {
		note.Pinned = *input.Pinned
	}
	if input.Favorite != nil {
		note.Favorite = *input.Favorite
	}

	s.notes[note.ID] = note

	// 更新标签索引
	s.updateTagIndex(note.ID, nil, note.Tags)

	return note, nil
}

// GetNote 获取笔记.
func (s *Store) GetNote(id string) (*Note, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	note, exists := s.notes[id]
	if !exists {
		return nil, ErrNoteNotFound
	}
	if note.DeletedAt != nil {
		return nil, ErrNoteInTrash
	}
	return note, nil
}

// UpdateNote 更新笔记.
func (s *Store) UpdateNote(id string, input NoteInput) (*Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	note, exists := s.notes[id]
	if !exists {
		return nil, ErrNoteNotFound
	}
	if note.DeletedAt != nil {
		return nil, ErrNoteInTrash
	}

	// 验证笔记本
	if input.NotebookID != "" {
		if _, exists := s.notebooks[input.NotebookID]; !exists {
			return nil, ErrNotebookNotFound
		}
		note.NotebookID = input.NotebookID
	}

	if input.Title != "" {
		note.Title = input.Title
	}
	if input.Content != "" {
		note.Content = input.Content
		note.WordCount = countWords(input.Content)
	}
	if input.Format != "" {
		if input.Format != FormatMarkdown && input.Format != FormatRichText {
			return nil, ErrInvalidFormat
		}
		note.Format = input.Format
	}
	if input.Tags != nil {
		oldTags := note.Tags
		note.Tags = input.Tags
		s.updateTagIndex(note.ID, oldTags, input.Tags)
	}
	if input.Pinned != nil {
		note.Pinned = *input.Pinned
	}
	if input.Favorite != nil {
		note.Favorite = *input.Favorite
	}

	note.UpdatedAt = time.Now()
	return note, nil
}

// DeleteNote 软删除笔记（移到回收站）.
func (s *Store) DeleteNote(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	note, exists := s.notes[id]
	if !exists {
		return ErrNoteNotFound
	}

	now := time.Now()
	note.DeletedAt = &now
	note.UpdatedAt = now

	// 清除标签索引
	s.updateTagIndex(note.ID, note.Tags, nil)

	return nil
}

// RestoreNote 从回收站恢复笔记.
func (s *Store) RestoreNote(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	note, exists := s.notes[id]
	if !exists {
		return ErrNoteNotFound
	}

	note.DeletedAt = nil
	note.UpdatedAt = time.Now()

	// 恢复标签索引
	s.updateTagIndex(note.ID, nil, note.Tags)

	return nil
}

// PermanentDeleteNote 永久删除笔记.
func (s *Store) PermanentDeleteNote(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	note, exists := s.notes[id]
	if !exists {
		return ErrNoteNotFound
	}

	// 清除标签索引
	s.updateTagIndex(note.ID, note.Tags, nil)

	// 删除相关分享
	for shareID, share := range s.shares {
		if share.NoteID == id {
			delete(s.shares, shareID)
		}
	}

	delete(s.notes, id)
	return nil
}

// ListNotes 列出笔记.
func (s *Store) ListNotes(notebookID string, limit, offset int) ([]*Note, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Note
	for _, note := range s.notes {
		if note.DeletedAt != nil {
			continue
		}
		if notebookID != "" && note.NotebookID != notebookID {
			continue
		}
		result = append(result, note)
	}

	// 排序：置顶优先，然后按更新时间倒序
	sort.Slice(result, func(i, j int) bool {
		if result[i].Pinned != result[j].Pinned {
			return result[i].Pinned
		}
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})

	total := len(result)
	if offset >= total {
		return []*Note{}, total
	}
	end := offset + limit
	if end > total {
		end = total
	}

	return result[offset:end], total
}

// ListTrashNotes 列出回收站笔记.
func (s *Store) ListTrashNotes(limit, offset int) ([]*Note, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Note
	for _, note := range s.notes {
		if note.DeletedAt != nil {
			result = append(result, note)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].DeletedAt.After(*result[j].DeletedAt)
	})

	total := len(result)
	if offset >= total {
		return []*Note{}, total
	}
	end := offset + limit
	if end > total {
		end = total
	}

	return result[offset:end], total
}

// ToggleFavorite 切换收藏状态.
func (s *Store) ToggleFavorite(id string) (*Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	note, exists := s.notes[id]
	if !exists {
		return nil, ErrNoteNotFound
	}
	if note.DeletedAt != nil {
		return nil, ErrNoteInTrash
	}

	note.Favorite = !note.Favorite
	note.UpdatedAt = time.Now()
	return note, nil
}

// TogglePin 切换置顶状态.
func (s *Store) TogglePin(id string) (*Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	note, exists := s.notes[id]
	if !exists {
		return nil, ErrNoteNotFound
	}
	if note.DeletedAt != nil {
		return nil, ErrNoteInTrash
	}

	note.Pinned = !note.Pinned
	note.UpdatedAt = time.Now()
	return note, nil
}

// GetNoteTags 获取笔记的所有标签.
func (s *Store) GetNoteTags(noteID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	note, exists := s.notes[noteID]
	if !exists {
		return nil, ErrNoteNotFound
	}
	return note.Tags, nil
}

// GetAllTags 获取所有标签.
func (s *Store) GetAllTags() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tags := make(map[string]int)
	for tag, noteIDs := range s.tagIndex {
		// 只计算未删除笔记的标签
		count := 0
		for noteID := range noteIDs {
			if note, exists := s.notes[noteID]; exists && note.DeletedAt == nil {
				count++
			}
		}
		if count > 0 {
			tags[tag] = count
		}
	}
	return tags
}

// ========== 笔记本 CRUD ==========

// CreateNotebook 创建笔记本.
func (s *Store) CreateNotebook(input NotebookInput, ownerID string) (*Notebook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if input.Name == "" {
		return nil, errors.New("笔记本名称不能为空")
	}

	now := time.Now()
	notebook := &Notebook{
		ID:          generateID(),
		Name:        input.Name,
		Description: input.Description,
		Color:       input.Color,
		Icon:        input.Icon,
		OwnerID:     ownerID,
		SortOrder:   0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if input.SortOrder != nil {
		notebook.SortOrder = *input.SortOrder
	}
	if notebook.Color == "" {
		notebook.Color = "#3B82F6"
	}

	s.notebooks[notebook.ID] = notebook
	return notebook, nil
}

// GetNotebook 获取笔记本.
func (s *Store) GetNotebook(id string) (*Notebook, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	notebook, exists := s.notebooks[id]
	if !exists {
		return nil, ErrNotebookNotFound
	}
	return notebook, nil
}

// UpdateNotebook 更新笔记本.
func (s *Store) UpdateNotebook(id string, input NotebookInput) (*Notebook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	notebook, exists := s.notebooks[id]
	if !exists {
		return nil, ErrNotebookNotFound
	}

	if input.Name != "" {
		notebook.Name = input.Name
	}
	if input.Description != "" {
		notebook.Description = input.Description
	}
	if input.Color != "" {
		notebook.Color = input.Color
	}
	if input.Icon != "" {
		notebook.Icon = input.Icon
	}
	if input.SortOrder != nil {
		notebook.SortOrder = *input.SortOrder
	}

	notebook.UpdatedAt = time.Now()

	// 更新笔记数
	count := 0
	for _, note := range s.notes {
		if note.NotebookID == id && note.DeletedAt == nil {
			count++
		}
	}
	notebook.NoteCount = count

	return notebook, nil
}

// DeleteNotebook 删除笔记本.
func (s *Store) DeleteNotebook(id string, moveNotesTo string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.notebooks[id]; !exists {
		return ErrNotebookNotFound
	}

	// 移动或取消关联笔记
	for _, note := range s.notes {
		if note.NotebookID == id {
			note.NotebookID = moveNotesTo
			note.UpdatedAt = time.Now()
		}
	}

	delete(s.notebooks, id)
	return nil
}

// ListNotebooks 列出笔记本.
func (s *Store) ListNotebooks(ownerID string) []*Notebook {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Notebook
	for _, nb := range s.notebooks {
		if ownerID != "" && nb.OwnerID != ownerID {
			continue
		}
		// 更新笔记数
		count := 0
		for _, note := range s.notes {
			if note.NotebookID == nb.ID && note.DeletedAt == nil {
				count++
			}
		}
		nb.NoteCount = count
		result = append(result, nb)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].SortOrder < result[j].SortOrder
	})

	return result
}

// ========== 分享管理 ==========

// CreateShare 创建分享链接.
func (s *Store) CreateShare(noteID string, input ShareInput, creatorID string) (*NoteShare, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.notes[noteID]; !exists {
		return nil, ErrNoteNotFound
	}

	permission := input.Permission
	if permission == "" {
		permission = "view"
	}
	if permission != "view" && permission != "edit" {
		return nil, errors.New("无效的权限类型")
	}

	now := time.Now()
	share := &NoteShare{
		ID:         generateID(),
		NoteID:     noteID,
		Token:      generateToken(),
		Permission: permission,
		MaxViews:   input.MaxViews,
		ViewCount:  0,
		CreatorID:  creatorID,
		CreatedAt:  now,
	}

	if input.Password != "" {
		share.Password = input.Password
		share.HasPassword = true
	}

	if input.ExpiresIn > 0 {
		expires := now.Add(time.Duration(input.ExpiresIn) * time.Hour)
		share.ExpiresAt = &expires
	}

	s.shares[share.ID] = share
	return share, nil
}

// GetShare 获取分享.
func (s *Store) GetShare(id string) (*NoteShare, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	share, exists := s.shares[id]
	if !exists {
		return nil, ErrShareNotFound
	}
	return share, nil
}

// GetShareByToken 通过Token获取分享.
func (s *Store) GetShareByToken(token string) (*NoteShare, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, share := range s.shares {
		if share.Token == token {
			return share, nil
		}
	}
	return nil, ErrShareNotFound
}

// AccessShare 访问分享（验证密码和过期）.
func (s *Store) AccessShare(token, password string) (*Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var share *NoteShare
	for _, sh := range s.shares {
		if sh.Token == token {
			share = sh
			break
		}
	}
	if share == nil {
		return nil, ErrShareNotFound
	}

	// 检查过期
	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		return nil, ErrShareExpired
	}

	// 检查访问次数
	if share.MaxViews > 0 && share.ViewCount >= share.MaxViews {
		return nil, ErrShareMaxViews
	}

	// 验证密码
	if share.HasPassword {
		if password == "" {
			return nil, ErrPasswordRequired
		}
		if share.Password != password {
			return nil, ErrPasswordWrong
		}
	}

	// 增加访问次数
	share.ViewCount++

	note, exists := s.notes[share.NoteID]
	if !exists {
		return nil, ErrNoteNotFound
	}

	return note, nil
}

// ListNoteShares 列出笔记的所有分享.
func (s *Store) ListNoteShares(noteID string) []*NoteShare {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*NoteShare
	for _, share := range s.shares {
		if share.NoteID == noteID {
			result = append(result, share)
		}
	}
	return result
}

// DeleteShare 删除分享.
func (s *Store) DeleteShare(shareID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.shares[shareID]; !exists {
		return ErrShareNotFound
	}

	delete(s.shares, shareID)
	return nil
}

// ========== 搜索 ==========

// Search 搜索笔记.
func (s *Store) Search(query SearchQuery) *SearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}

	keyword := strings.ToLower(strings.TrimSpace(query.Keyword))
	var result []*Note

	for _, note := range s.notes {
		if note.DeletedAt != nil {
			continue
		}

		// 笔记本过滤
		if query.NotebookID != "" && note.NotebookID != query.NotebookID {
			continue
		}

		// 格式过滤
		if query.Format != "" && note.Format != query.Format {
			continue
		}

		// 置顶过滤
		if query.Pinned != nil && note.Pinned != *query.Pinned {
			continue
		}

		// 收藏过滤
		if query.Favorite != nil && note.Favorite != *query.Favorite {
			continue
		}

		// 标签过滤
		if len(query.Tags) > 0 {
			hasTag := false
			for _, tag := range query.Tags {
				for _, noteTag := range note.Tags {
					if strings.EqualFold(tag, noteTag) {
						hasTag = true
						break
					}
				}
				if hasTag {
					break
				}
			}
			if !hasTag {
				continue
			}
		}

		// 关键词匹配（标题+内容）
		if keyword != "" {
			titleMatch := strings.Contains(strings.ToLower(note.Title), keyword)
			contentMatch := strings.Contains(strings.ToLower(note.Content), keyword)
			tagMatch := false
			for _, tag := range note.Tags {
				if strings.Contains(strings.ToLower(tag), keyword) {
					tagMatch = true
					break
				}
			}
			if !titleMatch && !contentMatch && !tagMatch {
				continue
			}
		}

		result = append(result, note)
	}

	// 排序
	sortBy := query.SortBy
	if sortBy == "" {
		sortBy = "updated_at"
	}
	sortOrder := query.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}

	sort.Slice(result, func(i, j int) bool {
		// 置顶优先
		if result[i].Pinned != result[j].Pinned {
			return result[i].Pinned
		}

		var less bool
		switch sortBy {
		case "created_at":
			less = result[i].CreatedAt.Before(result[j].CreatedAt)
		case "title":
			less = strings.ToLower(result[i].Title) < strings.ToLower(result[j].Title)
		default: // updated_at
			less = result[i].UpdatedAt.Before(result[j].UpdatedAt)
		}

		if sortOrder == "desc" {
			return !less
		}
		return less
	})

	total := len(result)
	if offset >= total {
		return &SearchResult{
			Notes:   []*Note{},
			Total:   total,
			Limit:   limit,
			Offset:  offset,
			Keyword: query.Keyword,
		}
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return &SearchResult{
		Notes:   result[offset:end],
		Total:   total,
		Limit:   limit,
		Offset:  offset,
		Keyword: query.Keyword,
	}
}

// ========== 辅助方法 ==========

// updateTagIndex 更新标签索引.
func (s *Store) updateTagIndex(noteID string, oldTags, newTags []string) {
	// 移除旧标签
	for _, tag := range oldTags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" {
			continue
		}
		if noteIDs, exists := s.tagIndex[tag]; exists {
			delete(noteIDs, noteID)
			if len(noteIDs) == 0 {
				delete(s.tagIndex, tag)
			}
		}
	}

	// 添加新标签
	for _, tag := range newTags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" {
			continue
		}
		if s.tagIndex[tag] == nil {
			s.tagIndex[tag] = make(map[string]bool)
		}
		s.tagIndex[tag][noteID] = true
	}
}

// countWords 统计字数.
func countWords(content string) int {
	if content == "" {
		return 0
	}
	// 简单实现：按字符数统计
	return len([]rune(content))
}

// generateID 生成随机 ID.
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return uuid.New().String()
	}
	return hex.EncodeToString(b)
}

// generateToken 生成分享Token.
func generateToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return uuid.New().String()
	}
	return hex.EncodeToString(b)
}

// GetNoteStats 获取笔记统计.
func (s *Store) GetNoteStats(ownerID string) map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totalNotes := 0
	deletedNotes := 0
	totalNotebooks := 0
	favorites := 0
	totalWords := 0

	for _, note := range s.notes {
		if note.OwnerID == ownerID || ownerID == "" {
			if note.DeletedAt != nil {
				deletedNotes++
			} else {
				totalNotes++
				totalWords += note.WordCount
				if note.Favorite {
					favorites++
				}
			}
		}
	}

	for _, nb := range s.notebooks {
		if nb.OwnerID == ownerID || ownerID == "" {
			totalNotebooks++
		}
	}

	return map[string]interface{}{
		"total_notes":     totalNotes,
		"deleted_notes":   deletedNotes,
		"total_notebooks": totalNotebooks,
		"favorites":       favorites,
		"total_words":     totalWords,
		"total_tags":      len(s.tagIndex),
	}
}

// EmptyTrash 清空回收站.
func (s *Store) EmptyTrash() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for id, note := range s.notes {
		if note.DeletedAt != nil {
			// 清除标签索引
			s.updateTagIndex(id, note.Tags, nil)
			// 删除相关分享
			for shareID, share := range s.shares {
				if share.NoteID == id {
					delete(s.shares, shareID)
				}
			}
			delete(s.notes, id)
			count++
		}
	}
	return count
}

// Close 关闭存储.
func (s *Store) Close() error {
	return nil
}
