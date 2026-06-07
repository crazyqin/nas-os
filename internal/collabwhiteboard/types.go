// Package collabwhiteboard 提供协作白板功能
package collabwhiteboard

import (
	"time"
)

// Board 白板.
type Board struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Owner       string    `json:"owner"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	Elements    []Element `json:"elements,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Element 白板元素.
type Element struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // stroke, shape, text, image
	X         float64   `json:"x"`
	Y         float64   `json:"y"`
	Width     float64   `json:"width,omitempty"`
	Height    float64   `json:"height,omitempty"`
	Rotation  float64   `json:"rotation,omitempty"`
	Layer     int       `json:"layer"`
	Visible   bool      `json:"visible"`
	Locked    bool      `json:"locked"`
	Style     Style     `json:"style"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Style 元素样式.
type Style struct {
	StrokeColor  string  `json:"stroke_color,omitempty"`
	StrokeWidth  float64 `json:"stroke_width,omitempty"`
	FillColor    string  `json:"fill_color,omitempty"`
	Opacity      float64 `json:"opacity,omitempty"`
	FontSize     int     `json:"font_size,omitempty"`
	FontFamily   string  `json:"font_family,omitempty"`
	FontBold     bool    `json:"font_bold,omitempty"`
	FontItalic   bool    `json:"font_italic,omitempty"`
	BorderRadius float64 `json:"border_radius,omitempty"`
	DashPattern  []int   `json:"dash_pattern,omitempty"`
}

// Stroke 笔画.
type Stroke struct {
	Points []Point `json:"points"`
	Tool   string  `json:"tool"` // pen, pencil, highlighter, eraser
	Style  Style   `json:"style"`
}

// Point 坐标点.
type Point struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Width float64 `json:"width,omitempty"`
}

// Shape 形状.
type Shape struct {
	ShapeType string  `json:"shape_type"` // rect, circle, triangle, line, arrow, diamond
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Width     float64 `json:"width"`
	Height    float64 `json:"height"`
	Style     Style   `json:"style"`
}

// TextElement 文本元素.
type TextElement struct {
	Text   string  `json:"text"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Style  Style   `json:"style"`
}

// ImageElement 图片元素.
type ImageElement struct {
	URL    string  `json:"url"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Alt    string  `json:"alt,omitempty"`
}

// Cursor 协作者光标.
type Cursor struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	X         float64   `json:"x"`
	Y         float64   `json:"y"`
	Color     string    `json:"color"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Version 版本记录.
type Version struct {
	ID        string    `json:"id"`
	BoardID   string    `json:"board_id"`
	UserID    string    `json:"user_id"`
	Elements  []Element `json:"elements"`
	Comment   string    `json:"comment,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Operation 操作记录.
type Operation struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // add, update, delete, move
	ElementID string    `json:"element_id"`
	Data      Element   `json:"data"`
	UserID    string    `json:"user_id"`
	Timestamp time.Time `json:"timestamp"`
}

// BoardTemplate 白板模板.
type BoardTemplate struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Elements    []Element `json:"elements"`
	Category    string    `json:"category,omitempty"`
}

// CreateBoardRequest 创建白板请求.
type CreateBoardRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description,omitempty"`
	Owner       string `json:"owner"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

// UpdateBoardRequest 更新白板请求.
type UpdateBoardRequest struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Width       *int    `json:"width,omitempty"`
	Height      *int    `json:"height,omitempty"`
}

// AddElementRequest 添加元素请求.
type AddElementRequest struct {
	Type   string  `json:"type" binding:"required"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
	Layer  int     `json:"layer,omitempty"`
	Style  Style   `json:"style"`
}

// UpdateElementRequest 更新元素请求.
type UpdateElementRequest struct {
	X        *float64 `json:"x,omitempty"`
	Y        *float64 `json:"y,omitempty"`
	Width    *float64 `json:"width,omitempty"`
	Height   *float64 `json:"height,omitempty"`
	Rotation *float64 `json:"rotation,omitempty"`
	Layer    *int     `json:"layer,omitempty"`
	Visible  *bool    `json:"visible,omitempty"`
	Locked   *bool    `json:"locked,omitempty"`
	Style    *Style   `json:"style,omitempty"`
}

// MoveElementRequest 移动元素请求.
type MoveElementRequest struct {
	X float64 `json:"x" binding:"required"`
	Y float64 `json:"y" binding:"required"`
}

// ResizeElementRequest 调整大小请求.
type ResizeElementRequest struct {
	Width  float64 `json:"width" binding:"required"`
	Height float64 `json:"height" binding:"required"`
}

// CursorUpdate 光标更新.
type CursorUpdate struct {
	UserID   string  `json:"user_id"`
	Username string  `json:"username"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Color    string  `json:"color"`
}

// ExportRequest 导出请求.
type ExportRequest struct {
	Format string `json:"format" binding:"required"` // svg, png, pdf, json
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// ExportResult 导出结果.
type ExportResult struct {
	Format   string `json:"format"`
	Data     []byte `json:"data"`
	MimeType string `json:"mime_type"`
}

// Collaborator 协作者.
type Collaborator struct {
	UserID   string    `json:"user_id"`
	Username string    `json:"username"`
	Role     string    `json:"role"` // owner, editor, viewer
	Color    string    `json:"color"`
	JoinedAt time.Time `json:"joined_at"`
}

// AddCollaboratorRequest 添加协作者请求.
type AddCollaboratorRequest struct {
	UserID   string `json:"user_id" binding:"required"`
	Username string `json:"username" binding:"required"`
	Role     string `json:"role" binding:"required"`
	Color    string `json:"color"`
}
