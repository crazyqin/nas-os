// Package knowledgebase 提供个人知识库管理功能
package knowledgebase

import (
	"time"
)

// Document 知识库文档.
type Document struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"` // Markdown 格式
	Author      string    `json:"author"`
	WorkspaceID string    `json:"workspace_id"`
	Tags        []string  `json:"tags,omitempty"`
	Links       []Link    `json:"links,omitempty"`
	IsTemplate  bool      `json:"is_template"`
	IsFavorite  bool      `json:"is_favorite"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Note 笔记条目.
type Note struct {
	ID        string    `json:"id"`
	DocID     string    `json:"doc_id"`
	Content   string    `json:"content"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Tag 标签.
type Tag struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
	Count int    `json:"count"`
}

// Workspace 工作空间.
type Workspace struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Owner       string    `json:"owner"`
	DocCount    int       `json:"doc_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// Link 文档链接（双向链接）.
type Link struct {
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
	Type     string `json:"type"` // reference, embed, backlink
	Label    string `json:"label,omitempty"`
}

// SearchQuery 搜索查询.
type SearchQuery struct {
	Query       string   `form:"q" binding:"required"`
	WorkspaceID string   `form:"workspace_id,omitempty"`
	Tags        []string `form:"tags,omitempty"`
	Author      string   `form:"author,omitempty"`
	Limit       int      `form:"limit,omitempty"`
	Offset      int      `form:"offset,omitempty"`
}

// SearchResult 搜索结果.
type SearchResult struct {
	Doc     Document `json:"doc"`
	Score   float64  `json:"score"`
	Snippet string   `json:"snippet"`
}

// GraphNode 知识图谱节点.
type GraphNode struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Group string `json:"group,omitempty"`
	Size  int    `json:"size"`
}

// GraphEdge 知识图谱边.
type GraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
	Weight int    `json:"weight"`
}

// GraphData 知识图谱数据.
type GraphData struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// Template 文档模板.
type Template struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Content     string    `json:"content"`
	Category    string    `json:"category,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateDocRequest 创建文档请求.
type CreateDocRequest struct {
	Title       string   `json:"title" binding:"required"`
	Content     string   `json:"content"`
	Author      string   `json:"author"`
	WorkspaceID string   `json:"workspace_id"`
	Tags        []string `json:"tags,omitempty"`
	IsTemplate  bool     `json:"is_template,omitempty"`
}

// UpdateDocRequest 更新文档请求.
type UpdateDocRequest struct {
	Title      *string  `json:"title,omitempty"`
	Content    *string  `json:"content,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	IsFavorite *bool    `json:"is_favorite,omitempty"`
	IsTemplate *bool    `json:"is_template,omitempty"`
}

// CreateWorkspaceRequest 创建工作空间请求.
type CreateWorkspaceRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description,omitempty"`
	Owner       string `json:"owner"`
}

// UpdateWorkspaceRequest 更新工作空间请求.
type UpdateWorkspaceRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// CreateLinkRequest 创建链接请求.
type CreateLinkRequest struct {
	SourceID string `json:"source_id" binding:"required"`
	TargetID string `json:"target_id" binding:"required"`
	Type     string `json:"type" binding:"required"`
	Label    string `json:"label,omitempty"`
}

// ImportRequest 导入请求.
type ImportRequest struct {
	Source      string `json:"source" binding:"required"` // notion, obsidian, confluence, evernote
	Content     []byte `json:"content" binding:"required"`
	WorkspaceID string `json:"workspace_id"`
	Author      string `json:"author"`
}

// ExportRequest 导出请求.
type ExportRequest struct {
	DocIDs []string `json:"doc_ids" binding:"required"`
	Format string   `json:"format" binding:"required"` // markdown, json, html
}

// TagStat 标签统计.
type TagStat struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// RefStat 引用统计.
type RefStat struct {
	DocID     string `json:"doc_id"`
	Title     string `json:"title"`
	RefCount  int    `json:"ref_count"`
	BackCount int    `json:"back_count"`
}
