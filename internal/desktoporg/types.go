package desktoporg

import (
	"time"
)

// DesktopIcon 桌面图标
type DesktopIcon struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	IconURL   string    `json:"icon_url"`
	AppID     string    `json:"app_id"`             // 关联的应用ID
	Type      IconType  `json:"type"`               // 图标类型
	Position  Position  `json:"position"`           // 位置
	GroupID   string    `json:"group_id,omitempty"` // 所属分组ID
	ScreenID  string    `json:"screen_id"`          // 所属屏幕ID
	Size      IconSize  `json:"size"`               // 图标大小
	Visible   bool      `json:"visible"`            // 是否可见
	Locked    bool      `json:"locked"`             // 是否锁定位置
	Command   string    `json:"command,omitempty"`  // 启动命令
	Tooltip   string    `json:"tooltip,omitempty"`  // 提示信息
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IconType 图标类型
type IconType string

const (
	IconTypeApp       IconType = "app"       // 应用图标
	IconTypeFile      IconType = "file"      // 文件快捷方式
	IconTypeFolder    IconType = "folder"    // 文件夹
	IconTypeWidget    IconType = "widget"    // 小组件
	IconTypeShortcut  IconType = "shortcut"  // 快捷方式
	IconTypeSeparator IconType = "separator" // 分隔符
)

// Position 图标位置
type Position struct {
	X     int `json:"x"`     // X坐标（网格单位）
	Y     int `json:"y"`     // Y坐标（网格单位）
	Index int `json:"index"` // 排序索引
}

// IconSize 图标大小
type IconSize string

const (
	SizeSmall  IconSize = "small"  // 小图标 48x48
	SizeMedium IconSize = "medium" // 中图标 64x64
	SizeLarge  IconSize = "large"  // 大图标 96x96
)

// IconGroup 图标分组
type IconGroup struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Icon        string      `json:"icon,omitempty"`  // 分组图标URL
	Color       string      `json:"color,omitempty"` // 分组颜色
	Position    Position    `json:"position"`        // 分组在桌面的位置
	ScreenID    string      `json:"screen_id"`       // 所属屏幕ID
	Collapsed   bool        `json:"collapsed"`       // 是否折叠
	IconIDs     []string    `json:"icon_ids"`        // 包含的图标ID列表
	Layout      GroupLayout `json:"layout"`          // 分组内布局
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// GroupLayout 分组内布局
type GroupLayout string

const (
	LayoutGrid   GroupLayout = "grid"   // 网格布局
	LayoutList   GroupLayout = "list"   // 列表布局
	LayoutCircle GroupLayout = "circle" // 环形布局
)

// DesktopLayout 桌面布局配置
type DesktopLayout struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	IsDefault   bool           `json:"is_default"`
	Screens     []ScreenLayout `json:"screens"`
	GridSize    GridSize       `json:"grid_size"`
	Theme       LayoutTheme    `json:"theme"`
	IconOrder   []string       `json:"icon_order"` // 全局图标排序
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// ScreenLayout 屏幕布局
type ScreenLayout struct {
	ScreenID   string   `json:"screen_id"`
	Name       string   `json:"name"`
	Resolution Size     `json:"resolution"`
	IconIDs    []string `json:"icon_ids"`
	GroupIDs   []string `json:"group_ids"`
	Background string   `json:"background,omitempty"` // 背景图片URL
	Primary    bool     `json:"primary"`              // 是否主屏幕
}

// GridSize 网格大小配置
type GridSize struct {
	Columns int `json:"columns"` // 列数
	Rows    int `json:"rows"`    // 行数
	CellW   int `json:"cell_w"`  // 单元格宽度（像素）
	CellH   int `json:"cell_h"`  // 单元格高度（像素）
	Gap     int `json:"gap"`     // 间距（像素）
}

// Size 尺寸
type Size struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// LayoutTheme 布局主题
type LayoutTheme struct {
	Background    string `json:"background,omitempty"`
	IconStyle     string `json:"icon_style"`     // icon_style: flat, material, skeuomorphic
	LabelPosition string `json:"label_position"` // bottom, right, hidden
	LabelColor    string `json:"label_color"`
	ShowGrid      bool   `json:"show_grid"`
	AnimateIcons  bool   `json:"animate_icons"`
}

// CreateIconRequest 创建图标请求
type CreateIconRequest struct {
	Name     string   `json:"name" binding:"required"`
	IconURL  string   `json:"icon_url"`
	AppID    string   `json:"app_id"`
	Type     IconType `json:"type"`
	Position Position `json:"position"`
	GroupID  string   `json:"group_id"`
	ScreenID string   `json:"screen_id"`
	Size     IconSize `json:"size"`
	Command  string   `json:"command"`
	Tooltip  string   `json:"tooltip"`
}

// UpdateIconRequest 更新图标请求
type UpdateIconRequest struct {
	Name     *string   `json:"name,omitempty"`
	IconURL  *string   `json:"icon_url,omitempty"`
	Position *Position `json:"position,omitempty"`
	GroupID  *string   `json:"group_id,omitempty"`
	ScreenID *string   `json:"screen_id,omitempty"`
	Size     *IconSize `json:"size,omitempty"`
	Visible  *bool     `json:"visible,omitempty"`
	Locked   *bool     `json:"locked,omitempty"`
	Command  *string   `json:"command,omitempty"`
	Tooltip  *string   `json:"tooltip,omitempty"`
}

// MoveIconRequest 移动图标请求
type MoveIconRequest struct {
	Position Position `json:"position" binding:"required"`
	ScreenID string   `json:"screen_id"`
}

// CreateGroupRequest 创建分组请求
type CreateGroupRequest struct {
	Name        string      `json:"name" binding:"required"`
	Description string      `json:"description"`
	Icon        string      `json:"icon"`
	Color       string      `json:"color"`
	Position    Position    `json:"position"`
	ScreenID    string      `json:"screen_id"`
	Layout      GroupLayout `json:"layout"`
}

// UpdateGroupRequest 更新分组请求
type UpdateGroupRequest struct {
	Name        *string      `json:"name,omitempty"`
	Description *string      `json:"description,omitempty"`
	Icon        *string      `json:"icon,omitempty"`
	Color       *string      `json:"color,omitempty"`
	Position    *Position    `json:"position,omitempty"`
	Collapsed   *bool        `json:"collapsed,omitempty"`
	Layout      *GroupLayout `json:"layout,omitempty"`
}

// GroupAddIconRequest 分组添加图标请求
type GroupAddIconRequest struct {
	IconID string `json:"icon_id" binding:"required"`
}

// APIResponse 统一API响应
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

// ListResponse 列表响应
type ListResponse struct {
	Items    interface{} `json:"items"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}
