// Package docmanager 提供文档管理系统功能
package docmanager

import (
	"time"
)

// Document 表示一个文档
type Document struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Content       string    `json:"content"`
	Tags          []string  `json:"tags"`
	Category      string    `json:"category"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	FilePath      string    `json:"file_path"`
	MimeType      string    `json:"mime_type"`
	Size          int64     `json:"size"`
	OCRText       string    `json:"ocr_text"`
	ThumbnailPath string    `json:"thumbnail_path"`
}

// Category 表示一个文档分类
type Category struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ParentID    string `json:"parent_id"`
}

// Tag 表示一个文档标签
type Tag struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// SearchQuery 表示搜索查询条件
type SearchQuery struct {
	Query    string    `json:"query"`
	Tags     []string  `json:"tags"`
	Category string    `json:"category"`
	DateFrom time.Time `json:"date_from"`
	DateTo   time.Time `json:"date_to"`
	MimeType string    `json:"mime_type"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}

// SearchResult 表示搜索结果
type SearchResult struct {
	Documents []Document `json:"documents"`
	Total     int        `json:"total"`
	Page      int        `json:"page"`
	PageSize  int        `json:"page_size"`
}

// OCRResult 表示OCR识别结果
type OCRResult struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Language   string  `json:"language"`
	Pages      int     `json:"pages"`
}

// CreateDocumentRequest 创建文档请求
type CreateDocumentRequest struct {
	Title         string   `json:"title"`
	Content       string   `json:"content"`
	Tags          []string `json:"tags"`
	Category      string   `json:"category"`
	FilePath      string   `json:"file_path"`
	MimeType      string   `json:"mime_type"`
	Size          int64    `json:"size"`
	ThumbnailPath string   `json:"thumbnail_path"`
}

// UpdateDocumentRequest 更新文档请求
type UpdateDocumentRequest struct {
	Title         string   `json:"title,omitempty"`
	Content       string   `json:"content,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	Category      string   `json:"category,omitempty"`
	FilePath      string   `json:"file_path,omitempty"`
	MimeType      string   `json:"mime_type,omitempty"`
	Size          int64    `json:"size,omitempty"`
	ThumbnailPath string   `json:"thumbnail_path,omitempty"`
}

// CreateCategoryRequest 创建分类请求
type CreateCategoryRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ParentID    string `json:"parent_id"`
}

// CreateTagRequest 创建标签请求
type CreateTagRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}
