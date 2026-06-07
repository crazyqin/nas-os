// Package notestation 提供笔记管理功能，对标群晖 Note Station
package notestation

import (
	"time"
)

// Attachment 笔记附件.
type Attachment struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"`
	MimeType  string    `json:"mime_type"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// Note 笔记.
type Note struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Content     string       `json:"content"` // Markdown 格式
	Author      string       `json:"author"`
	Tags        []string     `json:"tags,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	IsPinned    bool         `json:"is_pinned"`
	IsFavorite  bool         `json:"is_favorite"`
	NotebookID  string       `json:"notebook_id"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Notebook 笔记本.
type Notebook struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	Owner       string    `json:"owner"`
}

// CreateNoteRequest 创建笔记请求.
type CreateNoteRequest struct {
	Title      string   `json:"title" binding:"required"`
	Content    string   `json:"content"`
	Author     string   `json:"author"`
	Tags       []string `json:"tags,omitempty"`
	NotebookID string   `json:"notebook_id"`
}

// UpdateNoteRequest 更新笔记请求.
type UpdateNoteRequest struct {
	Title      *string  `json:"title,omitempty"`
	Content    *string  `json:"content,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	IsPinned   *bool    `json:"is_pinned,omitempty"`
	IsFavorite *bool    `json:"is_favorite,omitempty"`
}

// CreateNotebookRequest 创建笔记本请求.
type CreateNotebookRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description,omitempty"`
	Owner       string `json:"owner"`
}

// UpdateNotebookRequest 更新笔记本请求.
type UpdateNotebookRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// SearchRequest 搜索请求.
type SearchRequest struct {
	Query string `form:"q" binding:"required"`
}

// TagStat 标签统计.
type TagStat struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// ImportRequest 导入请求.
type ImportRequest struct {
	Filename   string `json:"filename" binding:"required"`
	Content    string `json:"content" binding:"required"`
	NotebookID string `json:"notebook_id"`
	Author     string `json:"author"`
}

// ExportRequest 导出请求.
type ExportRequest struct {
	NoteIDs []string `json:"note_ids" binding:"required"`
}
