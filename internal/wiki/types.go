// Package wiki provides Wiki knowledge base management functionality.
// 支持Wiki空间、页面、版本历史、搜索、评论等功能
package wiki

import (
	"time"
)

// ============================================================
// Wiki空间相关类型
// ============================================================

// Space Wiki空间
type Space struct {
	ID          string    `json:"id"`           // 空间ID
	Name        string    `json:"name"`         // 空间名称
	Description string    `json:"description"`  // 空间描述
	Icon        string    `json:"icon"`         // 空间图标
	IsPublic    bool      `json:"is_public"`    // 是否公开
	OwnerID     string    `json:"owner_id"`     // 所有者ID
	Members     []*Member `json:"members"`      // 成员列表
	PageCount   int       `json:"page_count"`   // 页面数量
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`   // 更新时间
}

// Page Wiki页面
type Page struct {
	ID          string     `json:"id"`           // 页面ID
	SpaceID     string     `json:"space_id"`     // 所属空间ID
	Title       string     `json:"title"`        // 页面标题
	Content     string     `json:"content"`      // 页面内容 (Markdown)
	HTMLContent string     `json:"html_content"` // HTML渲染内容
	ParentID    string     `json:"parent_id"`    // 父页面ID (空表示顶级页面)
	Path        string     `json:"path"`         // 页面路径 (如 /docs/getting-started)
	Tags        []string   `json:"tags"`         // 标签列表
	AuthorID    string     `json:"author_id"`    // 作者ID
	AuthorName  string     `json:"author_name"`  // 作者名称
	Status      string     `json:"status"`       // 状态: draft, published, archived
	IsFavorite  bool       `json:"is_favorite"`  // 是否收藏
	ViewCount   int        `json:"view_count"`   // 查看次数
	Version     int        `json:"version"`      // 当前版本号
	Comments    []*Comment `json:"comments"`     // 评论列表
	Children    []*Page    `json:"children"`     // 子页面 (树形结构)
	CreatedAt   time.Time  `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time  `json:"updated_at"`   // 更新时间
	UpdatedBy   string     `json:"updated_by"`   // 最后更新者ID
}

// PageVersion 页面版本历史
type PageVersion struct {
	ID        string    `json:"id"`         // 版本ID
	PageID    string    `json:"page_id"`    // 所属页面ID
	Version   int       `json:"version"`    // 版本号
	Title     string    `json:"title"`      // 页面标题
	Content   string    `json:"content"`    // 页面内容
	AuthorID  string    `json:"author_id"`  // 修改者ID
	AuthorName string   `json:"author_name"` // 修改者名称
	Comment   string    `json:"comment"`    // 版本说明
	CreatedAt time.Time `json:"created_at"` // 创建时间
}

// Comment 页面评论
type Comment struct {
	ID        string    `json:"id"`         // 评论ID
	PageID    string    `json:"page_id"`    // 所属页面ID
	UserID    string    `json:"user_id"`    // 评论者ID
	Username  string    `json:"username"`   // 评论者用户名
	Content   string    `json:"content"`    // 评论内容
	ParentID  string    `json:"parent_id"`  // 父评论ID (回复)
	CreatedAt time.Time `json:"created_at"` // 创建时间
	UpdatedAt time.Time `json:"updated_at"` // 更新时间
}

// Member 空间成员
type Member struct {
	UserID   string    `json:"user_id"`   // 用户ID
	Username string    `json:"username"`  // 用户名
	Role     string    `json:"role"`      // 角色: owner, admin, editor, viewer
	JoinedAt time.Time `json:"joined_at"` // 加入时间
}

// Permission 页面权限
type Permission struct {
	PageID   string `json:"page_id"`   // 页面ID
	UserID   string `json:"user_id"`   // 用户ID
	CanView  bool   `json:"can_view"`  // 可查看
	CanEdit  bool   `json:"can_edit"`  // 可编辑
	CanAdmin bool   `json:"can_admin"` // 可管理
}

// SearchResult 搜索结果
type SearchResult struct {
	PageID      string    `json:"page_id"`      // 页面ID
	SpaceID     string    `json:"space_id"`     // 空间ID
	Title       string    `json:"title"`        // 页面标题
	Content     string    `json:"content"`      // 内容摘要
	Path        string    `json:"path"`         // 页面路径
	Score       float64   `json:"score"`        // 相关度分数
	Highlighted string    `json:"highlighted"`  // 高亮内容
	UpdatedAt   time.Time `json:"updated_at"`   // 更新时间
}

// ============================================================
// 请求/响应类型
// ============================================================

// CreateSpaceRequest 创建空间请求
type CreateSpaceRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	IsPublic    bool   `json:"is_public"`
}

// UpdateSpaceRequest 更新空间请求
type UpdateSpaceRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Icon        *string `json:"icon"`
	IsPublic    *bool   `json:"is_public"`
}

// CreatePageRequest 创建页面请求
type CreatePageRequest struct {
	SpaceID  string   `json:"space_id" binding:"required"`
	Title    string   `json:"title" binding:"required"`
	Content  string   `json:"content"`
	ParentID string   `json:"parent_id"`
	Tags     []string `json:"tags"`
	Status   string   `json:"status"`
}

// UpdatePageRequest 更新页面请求
type UpdatePageRequest struct {
	Title   *string  `json:"title"`
	Content *string  `json:"content"`
	Tags    []string `json:"tags"`
	Status  *string  `json:"status"`
	Comment string   `json:"comment"` // 版本说明
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query   string `json:"query" binding:"required"`
	SpaceID string `json:"space_id"` // 限定空间
	Limit   int    `json:"limit"`    // 返回数量限制
	Offset  int    `json:"offset"`   // 偏移量
}

// AddCommentRequest 添加评论请求
type AddCommentRequest struct {
	Content  string `json:"content" binding:"required"`
	ParentID string `json:"parent_id"` // 回复评论ID
}

// ExportRequest 导出请求
type ExportRequest struct {
	PageIDs  []string `json:"page_ids"`  // 指定页面ID
	SpaceID  string   `json:"space_id"`  // 导出整个空间
	Format   string   `json:"format"`    // 格式: markdown, html, json
}

// ImportRequest 导入请求
type ImportRequest struct {
	SpaceID  string `json:"space_id" binding:"required"`
	Format   string `json:"format" binding:"required"` // 格式: markdown, html, json
	Content  string `json:"content"`                   // 内容
	Filename string `json:"filename"`                  // 文件名
}
