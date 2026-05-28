// Package notes 提供笔记应用功能
package notes

import (
	"time"
)

// Note 笔记.
type Note struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Content     string       `json:"content"` // Markdown 格式
	Author      string       `json:"author"`
	NotebookID  string       `json:"notebook_id"`
	Tags        []string     `json:"tags,omitempty"`
	IsPinned    bool         `json:"is_pinned"`
	IsFavorite  bool         `json:"is_favorite"`
	IsPublic    bool         `json:"is_public"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Version     int          `json:"version"`
	WordCount   int          `json:"word_count"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Notebook 笔记本.
type Notebook struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Owner       string    `json:"owner"`
	Color       string    `json:"color,omitempty"`   // 笔记本颜色
	Icon        string    `json:"icon,omitempty"`     // 笔记本图标
	NoteCount   int       `json:"note_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Tag 标签.
type Tag struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	NoteCount int       `json:"note_count"`
	CreatedAt time.Time `json:"created_at"`
}

// ShareLink 分享链接.
type ShareLink struct {
	ID          string    `json:"id"`
	NoteID      string    `json:"note_id"`
	Token       string    `json:"token"`
	Password    string    `json:"password,omitempty"` // 密码保护
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	AllowEdit   bool      `json:"allow_edit"`
	VisitCount  int       `json:"visit_count"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   string    `json:"created_by"`
}

// Attachment 笔记附件.
type Attachment struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"`
	MimeType  string    `json:"mime_type"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// NoteVersion 笔记版本历史.
type NoteVersion struct {
	ID        string    `json:"id"`
	NoteID    string    `json:"note_id"`
	Version   int       `json:"version"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}

// ========== 请求/响应结构 ==========

// CreateNoteRequest 创建笔记请求.
type CreateNoteRequest struct {
	Title      string   `json:"title" binding:"required"`
	Content    string   `json:"content"`
	Author     string   `json:"author"`
	NotebookID string   `json:"notebook_id"`
	Tags       []string `json:"tags,omitempty"`
}

// UpdateNoteRequest 更新笔记请求.
type UpdateNoteRequest struct {
	Title      *string  `json:"title,omitempty"`
	Content    *string  `json:"content,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	IsPinned   *bool    `json:"is_pinned,omitempty"`
	IsFavorite *bool    `json:"is_favorite,omitempty"`
	IsPublic   *bool    `json:"is_public,omitempty"`
}

// CreateNotebookRequest 创建笔记本请求.
type CreateNotebookRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description,omitempty"`
	Owner       string `json:"owner"`
	Color       string `json:"color,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

// UpdateNotebookRequest 更新笔记本请求.
type UpdateNotebookRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Color       *string `json:"color,omitempty"`
	Icon        *string `json:"icon,omitempty"`
}

// CreateShareLinkRequest 创建分享链接请求.
type CreateShareLinkRequest struct {
	Password  string     `json:"password,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	AllowEdit bool       `json:"allow_edit"`
}

// SearchRequest 搜索请求.
type SearchRequest struct {
	Query      string `form:"q" binding:"required"`
	NotebookID string `form:"notebook_id"`
	Tag        string `form:"tag"`
}

// ImportMarkdownRequest 导入 Markdown 请求.
type ImportMarkdownRequest struct {
	Title      string `json:"title" binding:"required"`
	Content    string `json:"content" binding:"required"`
	NotebookID string `json:"notebook_id"`
	Author     string `json:"author"`
}

// ExportNotesRequest 导出笔记请求.
type ExportNotesRequest struct {
	NoteIDs  []string `json:"note_ids" binding:"required"`
	Format   string   `json:"format"`   // markdown, html, pdf
}
