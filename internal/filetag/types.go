package filetag

import (
	"time"
)

// Tag 标签定义.
type Tag struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Color       string    `json:"color,omitempty"`       // 标签颜色，如 #FF5733
	Description string    `json:"description,omitempty"` // 标签描述
	Category    string    `json:"category,omitempty"`    // 标签分类
	CreatedBy   string    `json:"created_by"`            // 创建者
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// FileTag 文件标签关联.
type FileTag struct {
	ID       string    `json:"id"`
	FilePath string    `json:"file_path"`      // 文件路径
	TagID    string    `json:"tag_id"`         // 标签ID
	TagName  string    `json:"tag_name"`       // 标签名称（冗余）
	TagColor string    `json:"tag_color"`      // 标签颜色（冗余）
	TaggedBy string    `json:"tagged_by"`      // 打标签的用户
	TaggedAt time.Time `json:"tagged_at"`      // 打标签时间
	Note     string    `json:"note,omitempty"` // 备注
}

// TagStats 标签统计.
type TagStats struct {
	TagID      string `json:"tag_id"`
	TagName    string `json:"tag_name"`
	FileCount  int    `json:"file_count"`  // 使用该标签的文件数
	UsageCount int    `json:"usage_count"` // 总使用次数
}

// TagCategory 标签分类.
type TagCategory struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	TagCount    int       `json:"tag_count"` // 该分类下的标签数
	CreatedAt   time.Time `json:"created_at"`
}

// SearchRequest 搜索请求.
type SearchRequest struct {
	Tags     []string `json:"tags,omitempty"`      // 按标签ID搜索
	TagNames []string `json:"tag_names,omitempty"` // 按标签名称搜索
	FilePath string   `json:"file_path,omitempty"` // 按文件路径搜索
	Category string   `json:"category,omitempty"`  // 按分类搜索
	Operator string   `json:"operator,omitempty"`  // 操作符：and/or
	Page     int      `json:"page,omitempty"`
	PageSize int      `json:"page_size,omitempty"`
}

// SearchResponse 搜索响应.
type SearchResponse struct {
	Files    []*FileTag `json:"files"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}

// BatchTagRequest 批量打标签请求.
type BatchTagRequest struct {
	FilePaths []string `json:"file_paths"` // 文件路径列表
	TagIDs    []string `json:"tag_ids"`    // 标签ID列表
	TaggedBy  string   `json:"tagged_by"`  // 操作用户
	Note      string   `json:"note,omitempty"`
}

// BatchUntagRequest 批量移除标签请求.
type BatchUntagRequest struct {
	FilePaths []string `json:"file_paths"` // 文件路径列表
	TagIDs    []string `json:"tag_ids"`    // 标签ID列表（为空则移除所有标签）
}

// APIResponse 通用API响应.
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}
