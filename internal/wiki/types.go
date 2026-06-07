// Package wiki 提供知识库管理功能，支持文档树管理、Markdown 编辑、版本历史、全文搜索、权限控制。
package wiki

import "time"

// ============================================================
// 知识库相关类型
// ============================================================

// Wiki 知识库
type Wiki struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	OwnerID     string        `json:"owner_id"`
	IsPublic    bool          `json:"is_public"`
	Pages       []*Page       `json:"pages"`
	Permissions []*Permission `json:"permissions"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// Page 页面
type Page struct {
	ID        string      `json:"id"`
	WikiID    string      `json:"wiki_id"`
	Title     string      `json:"title"`
	Content   string      `json:"content"`
	ParentID  string      `json:"parent_id"`
	Path      string      `json:"path"`
	Tags      []string    `json:"tags"`
	AuthorID  string      `json:"author_id"`
	Status    string      `json:"status"`
	Version   int         `json:"version"`
	Children  []*Page     `json:"children"`
	Revisions []*Revision `json:"revisions"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// Revision 版本历史
type Revision struct {
	ID        string    `json:"id"`
	PageID    string    `json:"page_id"`
	Version   int       `json:"version"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	AuthorID  string    `json:"author_id"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

// SearchResult 搜索结果
type SearchResult struct {
	PageID      string    `json:"page_id"`
	WikiID      string    `json:"wiki_id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Path        string    `json:"path"`
	Score       float64   `json:"score"`
	Highlighted string    `json:"highlighted"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Permission 权限
type Permission struct {
	WikiID  string `json:"wiki_id"`
	PageID  string `json:"page_id"`
	UserID  string `json:"user_id"`
	CanView bool   `json:"can_view"`
	CanEdit bool   `json:"can_edit"`
}

// ============================================================
// 请求类型
// ============================================================

// CreateWikiRequest 创建知识库请求
type CreateWikiRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	OwnerID     string `json:"owner_id" binding:"required"`
	IsPublic    bool   `json:"is_public"`
}

// CreatePageRequest 创建页面请求
type CreatePageRequest struct {
	Title    string   `json:"title" binding:"required"`
	Content  string   `json:"content"`
	ParentID string   `json:"parent_id"`
	Tags     []string `json:"tags"`
	AuthorID string   `json:"author_id" binding:"required"`
}

// UpdatePageRequest 更新页面请求
type UpdatePageRequest struct {
	Title    *string  `json:"title"`
	Content  *string  `json:"content"`
	Tags     []string `json:"tags"`
	AuthorID string   `json:"author_id" binding:"required"`
	Comment  string   `json:"comment"`
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query  string `json:"query" binding:"required"`
	WikiID string `json:"wiki_id"`
	Limit  int    `json:"limit"`
}

// SetPermissionRequest 设置权限请求
type SetPermissionRequest struct {
	UserID  string `json:"user_id" binding:"required"`
	CanView bool   `json:"can_view"`
	CanEdit bool   `json:"can_edit"`
}
