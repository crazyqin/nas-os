// Package office OnlyOffice 文档编辑集成
// 提供在线文档编辑、协作编辑、格式转换等功能
package office

import (
	"time"
)

// ========== 核心配置 ==========

// Config OnlyOffice 集成配置
type Config struct {
	// Document Server 配置
	ServerURL string `json:"server_url"` // OnlyOffice Document Server URL，如 http://onlyoffice:80
	SecretKey string `json:"secret_key"` // JWT 密钥（用于签名）

	// 功能开关
	Enabled      bool `json:"enabled"`       // 是否启用在线编辑
	CallbackAuth bool `json:"callback_auth"` // 回调是否需要认证

	// 支持的文件类型
	EnabledTypes []string `json:"enabled_types"` // 支持编辑的文件类型，如 ["docx", "xlsx", "pptx"]

	// 编辑器配置
	EditorConfig EditorConfig `json:"editor_config"`

	// 会话配置
	SessionTimeout int `json:"session_timeout"` // 会话超时时间（秒），默认 3600

	// 存储路径
	ConfigPath string `json:"-"` // 配置文件存储路径
}

// EditorConfig 编辑器配置
type EditorConfig struct {
	// 界面语言
	Lang string `json:"lang"` // zh-CN, en-US 等

	// 模式
	Mode string `json:"mode"` // edit 或 view

	// 协作配置
	CoEditing CoEditingConfig `json:"co_editing"`

	// 自定义配置
	Customization CustomizationConfig `json:"customization"`
}

// CoEditingConfig 协作编辑配置
type CoEditingConfig struct {
	Enabled    bool   `json:"enabled"`     // 是否启用协作编辑
	AutoSave   bool   `json:"auto_save"`   // 是否自动保存
	SaveDelay  int    `json:"save_delay"`  // 自动保存延迟（秒）
	ShowChanges bool  `json:"show_changes"` // 是否显示变更追踪
}

// CustomizationConfig 自定义配置
type CustomizationConfig struct {
	// 界面元素
	HideRightMenu bool `json:"hide_right_menu"` // 隐藏右侧菜单
	HideHeader    bool `json:"hide_header"`     // 隐藏头部

	// 功能限制
	Forcesave    bool `json:"forcesave"`     // 强制保存
	CommentAuthor bool `json:"comment_author"` // 是否允许评论

	// Logo 配置
	LogoURL string `json:"logo_url"` // 自定义 Logo URL
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		ServerURL:    "http://localhost:8080",
		SecretKey:    "",
		Enabled:      false,
		CallbackAuth: true,
		EnabledTypes: []string{
			"doc", "docx", "docm", "dotx", "dotm", "odt", "fodt", "ott", "rtf", "txt", "html", "htm", "mht", "pdf", "djvu", "fb2", "epub", "xps", "oxps",
			"xls", "xlsx", "xlsm", "xlt", "xltx", "xltm", "ods", "fods", "ots", "csv",
			"ppt", "pptx", "pptm", "pot", "potx", "potm", "odp", "fodp", "otp", "ppsx", "ppsm", "pps", "ppam",
		},
		EditorConfig: EditorConfig{
			Lang: "zh-CN",
			Mode: "edit",
			CoEditing: CoEditingConfig{
				Enabled:     true,
				AutoSave:    true,
				SaveDelay:   5,
				ShowChanges: true,
			},
			Customization: CustomizationConfig{
				HideRightMenu: false,
				HideHeader:    false,
				Forcesave:     true,
			},
		},
		SessionTimeout: 3600,
	}
}

// ========== 会话管理 ==========

// EditingSession 编辑会话
type EditingSession struct {
	// 基本信息
	ID           string    `json:"id"`            // 会话唯一 ID
	FileID       string    `json:"file_id"`       // 文件 ID
	FileName     string    `json:"file_name"`      // 文件名
	FileKey      string    `json:"file_key"`      // OnlyOffice 文件唯一标识
	FilePath     string    `json:"file_path"`     // 文件存储路径
	FileSize     int64     `json:"file_size"`     // 文件大小（字节）

	// 用户信息
	UserID       string    `json:"user_id"`       // 用户 ID
	UserName     string    `json:"user_name"`     // 用户显示名
	UserGroup    string    `json:"user_group"`    // 用户组（可选）

	// 时间信息
	StartedAt    time.Time `json:"started_at"`    // 会话开始时间
	LastActiveAt time.Time `json:"last_active_at"`// 最后活动时间
	ExpiresAt    time.Time `json:"expires_at"`    // 过期时间

	// 回调配置
	CallbackURL  string    `json:"callback_url"`  // 回调 URL
	DocumentURL  string    `json:"document_url"`  // 文档访问 URL

	// 状态
	Status       SessionStatus `json:"status"`        // 会话状态
	Created      bool          `json:"created"`      // 是否已创建（OnlyOffice 端）

	// 元数据
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// SessionStatus 会话状态
type SessionStatus string

const (
	SessionStatusActive   SessionStatus = "active"    // 活动中
	SessionStatusEditing  SessionStatus = "editing"   // 编辑中
	SessionStatusSaving   SessionStatus = "saving"    // 保存中
	SessionStatusClosed   SessionStatus = "closed"    // 已关闭
	SessionStatusExpired  SessionStatus = "expired"   // 已过期
	SessionStatusError    SessionStatus = "error"     // 错误
)

// IsExpired 检查会话是否过期
func (s *EditingSession) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsActive 检查会话是否活动
func (s *EditingSession) IsActive() bool {
	return s.Status == SessionStatusActive || s.Status == SessionStatusEditing
}

// ========== 回调处理 ==========

// CallbackRequest OnlyOffice 回调请求
type CallbackRequest struct {
	// 密钥
	Key string `json:"key"` // 文档唯一标识

	// 状态
	Status int `json:"status"` // 回调状态码

	// 文件信息（状态 2 时提供）
	URL string `json:"url,omitempty"` // 编辑后文档的下载地址
	ChangeURL string `json:"changesurl,omitempty"` // 变更历史下载地址

	// 用户信息
	UserID string `json:"user_id,omitempty"` // 最后修改用户 ID
	User   string  `json:"user,omitempty"`   // 用户名

	// 创建者
	Users []CallbackUser `json:"users,omitempty"` // 当前打开文档的用户

	// 历史记录
	History History `json:"history,omitempty"`

	// 其他信息
	Changes   []Change `json:"changes,omitempty"` // 变更记录
	Actions   []Action `json:"actions,omitempty"`  // 用户操作
	Token     string   `json:"token,omitempty"`    // JWT Token（如果启用）
	Forcesave bool     `json:"forcesave,omitempty"` // 是否强制保存
}

// CallbackUser 回调用户信息
type CallbackUser struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	Group    string `json:"group,omitempty"`
}

// History 文档历史
type History struct {
	CurrentVersion string       `json:"currentVersion"`
	History        []HistoryItem `json:"history,omitempty"`
}

// HistoryItem 历史项
type HistoryItem struct {
	Version    string `json:"version"`
	Key        string `json:"key"`
	CreatedAt  string `json:"created"`
	User       CallbackUser `json:"user"`
	Changes    string `json:"changes,omitempty"`
	ServerVersion string `json:"serverVersion,omitempty"`
}

// Change 变更记录
type Change struct {
	User   CallbackUser `json:"user"`
	Type   string       `json:"type"`
	Time   string       `json:"time"`
}

// Action 用户操作
type Action struct {
	Type   int    `json:"type"`   // 1=连接，2=断开，3=强制保存
	UserID string `json:"userid"` // 用户 ID
}

// CallbackStatus 回调状态码
const (
	CallbackStatusEditing    = 1 // 正在编辑
	CallbackStatusSaved      = 2 // 已保存，可以下载
	CallbackStatusSaving     = 3 // 正在保存
	CallbackStatusClosed     = 4 // 文档已关闭
	CallbackStatusForceSave  = 6 // 强制保存
	CallbackStatusCorrupted  = 7 // 文档错误
	CallbackStatusClosedErr  = 8 // 关闭错误
)

// CallbackResponse 回调响应
type CallbackResponse struct {
	Error int `json:"error"` // 0=成功
}

// ========== 文件关联 ==========

// FileAssociation 文件关联配置
type FileAssociation struct {
	Extension string   `json:"extension"` // 文件扩展名（不含点）
	MimeType  string   `json:"mime_type"` // MIME 类型
	CanEdit   bool     `json:"can_edit"`  // 是否可编辑
	CanView   bool     `json:"can_view"`  // 是否可查看
	Converter string   `json:"converter,omitempty"` // 转换器标识
	Icon      string   `json:"icon,omitempty"`     // 图标路径
}

// DefaultFileAssociations 返回默认文件关联
func DefaultFileAssociations() map[string]FileAssociation {
	return map[string]FileAssociation{
		// 文档类型
		"doc":   {Extension: "doc", MimeType: "application/msword", CanEdit: true, CanView: true},
		"docx":  {Extension: "docx", MimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", CanEdit: true, CanView: true},
		"docm":  {Extension: "docm", MimeType: "application/vnd.ms-word.document.macroEnabled.12", CanEdit: true, CanView: true},
		"dotx":  {Extension: "dotx", MimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.template", CanEdit: true, CanView: true},
		"dotm":  {Extension: "dotm", MimeType: "application/vnd.ms-word.template.macroEnabled.12", CanEdit: true, CanView: true},
		"odt":   {Extension: "odt", MimeType: "application/vnd.oasis.opendocument.text", CanEdit: true, CanView: true},
		"fodt":  {Extension: "fodt", MimeType: "application/vnd.oasis.opendocument.text-flat-xml", CanEdit: true, CanView: true},
		"ott":   {Extension: "ott", MimeType: "application/vnd.oasis.opendocument.text-template", CanEdit: true, CanView: true},
		"rtf":   {Extension: "rtf", MimeType: "application/rtf", CanEdit: true, CanView: true},
		"txt":   {Extension: "txt", MimeType: "text/plain", CanEdit: true, CanView: true},
		"html":  {Extension: "html", MimeType: "text/html", CanEdit: true, CanView: true},
		"htm":   {Extension: "htm", MimeType: "text/html", CanEdit: true, CanView: true},
		"pdf":   {Extension: "pdf", MimeType: "application/pdf", CanEdit: false, CanView: true},
		"djvu":  {Extension: "djvu", MimeType: "image/vnd.djvu", CanEdit: false, CanView: true},
		"fb2":   {Extension: "fb2", MimeType: "application/x-fictionbook+xml", CanEdit: false, CanView: true},
		"epub":  {Extension: "epub", MimeType: "application/epub+zip", CanEdit: false, CanView: true},
		"xps":   {Extension: "xps", MimeType: "application/vnd.ms-xpsdocument", CanEdit: false, CanView: true},
		"oxps":  {Extension: "oxps", MimeType: "application/oxps", CanEdit: false, CanView: true},

		// 电子表格类型
		"xls":   {Extension: "xls", MimeType: "application/vnd.ms-excel", CanEdit: true, CanView: true},
		"xlsx":  {Extension: "xlsx", MimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", CanEdit: true, CanView: true},
		"xlsm":  {Extension: "xlsm", MimeType: "application/vnd.ms-excel.sheet.macroEnabled.12", CanEdit: true, CanView: true},
		"xlt":   {Extension: "xlt", MimeType: "application/vnd.ms-excel", CanEdit: true, CanView: true},
		"xltx":  {Extension: "xltx", MimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.template", CanEdit: true, CanView: true},
		"xltm":  {Extension: "xltm", MimeType: "application/vnd.ms-excel.template.macroEnabled.12", CanEdit: true, CanView: true},
		"ods":   {Extension: "ods", MimeType: "application/vnd.oasis.opendocument.spreadsheet", CanEdit: true, CanView: true},
		"fods":  {Extension: "fods", MimeType: "application/vnd.oasis.opendocument.spreadsheet-flat-xml", CanEdit: true, CanView: true},
		"ots":   {Extension: "ots", MimeType: "application/vnd.oasis.opendocument.spreadsheet-template", CanEdit: true, CanView: true},
		"csv":   {Extension: "csv", MimeType: "text/csv", CanEdit: true, CanView: true},

		// 演示文稿类型
		"ppt":   {Extension: "ppt", MimeType: "application/vnd.ms-powerpoint", CanEdit: true, CanView: true},
		"pptx":  {Extension: "pptx", MimeType: "application/vnd.openxmlformats-officedocument.presentationml.presentation", CanEdit: true, CanView: true},
		"pptm":  {Extension: "pptm", MimeType: "application/vnd.ms-powerpoint.presentation.macroEnabled.12", CanEdit: true, CanView: true},
		"pot":   {Extension: "pot", MimeType: "application/vnd.ms-powerpoint", CanEdit: true, CanView: true},
		"potx":  {Extension: "potx", MimeType: "application/vnd.openxmlformats-officedocument.presentationml.template", CanEdit: true, CanView: true},
		"potm":  {Extension: "potm", MimeType: "application/vnd.ms-powerpoint.template.macroEnabled.12", CanEdit: true, CanView: true},
		"odp":   {Extension: "odp", MimeType: "application/vnd.oasis.opendocument.presentation", CanEdit: true, CanView: true},
		"fodp":  {Extension: "fodp", MimeType: "application/vnd.oasis.opendocument.presentation-flat-xml", CanEdit: true, CanView: true},
		"otp":   {Extension: "otp", MimeType: "application/vnd.oasis.opendocument.presentation-template", CanEdit: true, CanView: true},
		"ppsx":  {Extension: "ppsx", MimeType: "application/vnd.openxmlformats-officedocument.presentationml.slideshow", CanEdit: true, CanView: true},
		"ppsm":  {Extension: "ppsm", MimeType: "application/vnd.ms-powerpoint.slideshow.macroEnabled.12", CanEdit: true, CanView: true},
		"pps":   {Extension: "pps", MimeType: "application/vnd.ms-powerpoint", CanEdit: true, CanView: true},
		"ppam":  {Extension: "ppam", MimeType: "application/vnd.ms-powerpoint.addin.macroEnabled.12", CanEdit: false, CanView: true},
	}
}

// ========== 编辑器配置（用于前端） ==========

// EditorInitConfig 编辑器初始化配置（传递给 OnlyOffice 前端）
type EditorInitConfig struct {
	// 文档配置
	Document DocumentConfig `json:"document"`

	// 文档类型
	DocumentType string `json:"documentType"` // word, cell, slide

	// 编辑器配置
	Editor EditorOptions `json:"editorConfig"`

	// 类型
	Type string `json:"type"` // desktop, mobile, embedded

	// JWT Token（如果启用）
	Token string `json:"token,omitempty"`
}

// DocumentConfig 文档配置
type DocumentConfig struct {
	// 文件类型（扩展名）
	FileType string `json:"fileType"`

	// 文档唯一标识
	Key string `json:"key"`

	// 文档标题
	Title string `json:"title"`

	// 文档 URL
	URL string `json:"url"`

	// 文档 URL（用于直接访问）
	DirectURL string `json:"directUrl,omitempty"`

	// 文档权限
	Permissions Permissions `json:"permissions"`
}

// Permissions 文档权限
type Permissions struct {
	Comment       bool `json:"comment"`        // 是否允许评论
	Copy         bool `json:"copy"`           // 是否允许复制
	Download     bool `json:"download"`       // 是否允许下载
	Edit         bool `json:"edit"`           // 是否允许编辑
	ModifyFilter bool `json:"modifyFilter"`   // 是否允许修改筛选
	ModifyContentControl bool `json:"modifyContentControl"` // 是否允许修改内容控件
	Print        bool `json:"print"`          // 是否允许打印
	Protect      bool `json:"protect"`        // 是否允许保护文档
	Review       bool `json:"review"`         // 是否允许审阅
	FillForms    bool `json:"fillForms"`      // 是否允许填写表单
}

// EditorOptions 编辑器选项
type EditorOptions struct {
	// 回调 URL
	CallbackURL string `json:"callbackUrl"`

	// 创建 URL（用于新建文档）
	CreateURL string `json:"createUrl,omitempty"`

	// 语言
	Lang string `json:"lang"`

	// 模式
	Mode string `json:"mode"` // edit, view

	// 用户信息
	User EditorUser `json:"user"`

	// 自定义
	Customization CustomizationOptions `json:"customization,omitempty"`

	// 协作
	CoEditing CoEditingOptions `json:"coEditing,omitempty"`
}

// EditorUser 编辑器用户
type EditorUser struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Group string `json:"group,omitempty"`
}

// CustomizationOptions 自定义选项
type CustomizationOptions struct {
	Forcesave     bool   `json:"forcesave"`
	HideRightMenu bool   `json:"hideRightMenu"`
	Logo          Logo   `json:"logo,omitempty"`
}

// Logo Logo 配置
type Logo struct {
	Image string `json:"image,omitempty"`
	URL   string `json:"url,omitempty"`
}

// CoEditingOptions 协作选项
type CoEditingOptions struct {
	Mode string `json:"mode"` // fast, strict
}

// ========== API 请求/响应 ==========

// EditRequest 编辑请求
type EditRequest struct {
	FileID   string `json:"file_id" binding:"required"`   // 文件 ID
	Mode     string `json:"mode"`                          // edit 或 view，默认 edit
	Language string `json:"language"`                      // 界面语言，默认 zh-CN
}

// EditResponse 编辑响应
type EditResponse struct {
	SessionID     string           `json:"session_id"`     // 会话 ID
	EditorConfig  EditorInitConfig `json:"editor_config"`  // 编辑器配置
	EditorURL     string           `json:"editor_url"`     // 编辑器页面 URL
	ExpiresAt     time.Time        `json:"expires_at"`     // 过期时间
}

// SessionListResponse 会话列表响应
type SessionListResponse struct {
	Total    int              `json:"total"`
	Sessions []EditingSession `json:"sessions"`
}

// ConfigResponse 配置响应
type ConfigResponse struct {
	Config       Config                     `json:"config"`
	Associations map[string]FileAssociation `json:"associations"`
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	ServerURL      *string  `json:"server_url,omitempty"`
	SecretKey      *string  `json:"secret_key,omitempty"`
	Enabled        *bool    `json:"enabled,omitempty"`
	CallbackAuth   *bool    `json:"callback_auth,omitempty"`
	EnabledTypes   []string `json:"enabled_types,omitempty"`
	SessionTimeout *int     `json:"session_timeout,omitempty"`
	EditorConfig   *EditorConfig `json:"editor_config,omitempty"`
}

// ========== 错误定义 ==========

// 错误常量
var (
	ErrNotEnabled        = "OnlyOffice 服务未启用"
	ErrServerNotReachable = "OnlyOffice 服务不可达"
	ErrInvalidConfig     = "配置无效"
	ErrSessionNotFound   = "会话不存在"
	ErrSessionExpired    = "会话已过期"
	ErrFileNotFound       = "文件不存在"
	ErrFileTypeNotSupported = "不支持的文件类型"
	ErrPermissionDenied   = "没有权限"
	ErrCallbackFailed     = "回调处理失败"
	ErrSaveFailed         = "保存失败"
	ErrInvalidToken       = "无效的 Token"
)