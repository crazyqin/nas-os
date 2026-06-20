// Package filecollector 文件收集门户数据模型定义。
// 包含收集任务、分享链接、上传门户、存储管理、通知服务、访问日志等核心数据结构。
// 对标群晖 DSM 7.3 的"文件请求"功能。
package filecollector

import (
	"time"
)

// TaskStatus 收集任务状态
type TaskStatus string

const (
	TaskStatusActive   TaskStatus = "active"   // 活跃中
	TaskStatusPaused   TaskStatus = "paused"   // 已暂停
	TaskStatusClosed   TaskStatus = "closed"   // 已关闭
	TaskStatusArchived TaskStatus = "archived" // 已归档
)

// LinkStatus 分享链接状态
type LinkStatus string

const (
	LinkStatusActive  LinkStatus = "active"  // 活跃
	LinkStatusExpired LinkStatus = "expired" // 已过期
	LinkStatusRevoked LinkStatus = "revoked" // 已撤销
)

// UploadStatus 上传状态
type UploadStatus string

const (
	UploadStatusPending    UploadStatus = "pending"    // 待处理
	UploadStatusUploading  UploadStatus = "uploading"  // 上传中
	UploadStatusCompleted  UploadStatus = "completed"  // 已完成
	UploadStatusFailed     UploadStatus = "failed"     // 失败
	UploadStatusQuarantine UploadStatus = "quarantine" // 隔离（疑似恶意文件）
)

// NotificationType 通知类型
type NotificationType string

const (
	NotificationNewUpload   NotificationType = "new_upload"   // 新文件上传
	NotificationTaskFull    NotificationType = "task_full"    // 任务存储已满
	NotificationLinkExpired NotificationType = "link_expired" // 链接过期
	NotificationUploadFail  NotificationType = "upload_fail"  // 上传失败
)

// NotificationChannel 通知渠道
type NotificationChannel string

const (
	NotificationEmail   NotificationChannel = "email"   // 邮件通知
	NotificationWebhook NotificationChannel = "webhook" // Webhook 回调
	NotificationSMS     NotificationChannel = "sms"     // 短信通知
	NotificationInApp   NotificationChannel = "in_app"  // 应用内通知
)

// StorageCategory 存储分类
type StorageCategory string

const (
	StorageCategoryDocument StorageCategory = "document" // 文档
	StorageCategoryImage    StorageCategory = "image"    // 图片
	StorageCategoryVideo    StorageCategory = "video"    // 视频
	StorageCategoryAudio    StorageCategory = "audio"    // 音频
	StorageCategoryArchive  StorageCategory = "archive"  // 压缩包
	StorageCategoryOther    StorageCategory = "other"    // 其他
)

// FileCollectorConfig 文件收集门户配置
type FileCollectorConfig struct {
	ListenAddr        string        `json:"listen_addr"`         // 监听地址
	BaseURL           string        `json:"base_url"`            // 外部访问基础 URL
	StorageRoot       string        `json:"storage_root"`        // 收集文件存储根目录
	MaxFileSize       int64         `json:"max_file_size"`       // 单文件最大大小（字节）
	MaxTaskSize       int64         `json:"max_task_size"`       // 单任务最大存储（字节）
	DefaultLinkExpiry time.Duration `json:"default_link_expiry"` // 默认链接过期时间
	MaxActiveTasks    int           `json:"max_active_tasks"`    // 最大活跃任务数
	MaxLinksPerTask   int           `json:"max_links_per_task"`  // 每任务最大链接数
	EnableVirusScan   bool          `json:"enable_virus_scan"`   // 启用病毒扫描
	EnableDedup       bool          `json:"enable_dedup"`        // 启用文件去重
	EnableNotify      bool          `json:"enable_notify"`       // 启用通知
	RetentionDays     int           `json:"retention_days"`      // 文件保留天数
}

// DefaultFileCollectorConfig 返回默认配置
func DefaultFileCollectorConfig() *FileCollectorConfig {
	return &FileCollectorConfig{
		ListenAddr:        ":8080",
		BaseURL:           "https://localhost:8080",
		StorageRoot:       "/mnt/collector",
		MaxFileSize:       5 * 1024 * 1024 * 1024, // 5GB
		MaxTaskSize:       50 * 1024 * 1024 * 1024, // 50GB
		DefaultLinkExpiry: 7 * 24 * time.Hour,      // 7 天
		MaxActiveTasks:    100,
		MaxLinksPerTask:   10,
		EnableVirusScan:   true,
		EnableDedup:       false,
		EnableNotify:      true,
		RetentionDays:     90,
	}
}

// CollectionTask 文件收集任务
type CollectionTask struct {
	ID            string        `json:"id"`                       // 唯一标识
	Name          string        `json:"name"`                     // 任务名称
	Description   string        `json:"description,omitempty"`    // 任务描述
	Status        TaskStatus    `json:"status"`                   // 任务状态
	CreatedBy     string        `json:"created_by"`               // 创建者
	StoragePath   string        `json:"storage_path"`             // 存储路径
	MaxSize       int64         `json:"max_size"`                 // 最大存储限制（字节）
	CurrentSize   int64         `json:"current_size"`             // 当前已用大小（字节）
	FileCount     int           `json:"file_count"`               // 已收集文件数
	MaxFiles      int           `json:"max_files"`                // 最大文件数限制
	AllowedExts   []string      `json:"allowed_exts,omitempty"`   // 允许的文件扩展名
	BlockedExts   []string      `json:"blocked_exts,omitempty"`   // 禁止的文件扩展名
	RequireAuth   bool          `json:"require_auth"`             // 上传需要密码
	UploadPassword string       `json:"upload_password,omitempty"`// 上传密码（哈希）
	AutoClassify  bool          `json:"auto_classify"`            // 自动分类文件
	CreatedAt     time.Time     `json:"created_at"`               // 创建时间
	UpdatedAt     time.Time     `json:"updated_at"`               // 更新时间
	ExpiresAt     *time.Time    `json:"expires_at,omitempty"`     // 过期时间
	Links         []*ShareLink  `json:"links,omitempty"`          // 关联的分享链接
	Notifications []NotificationPreference `json:"notifications,omitempty"` // 通知偏好
}

// ShareLink 安全分享链接
type ShareLink struct {
	ID            string     `json:"id"`                       // 唯一标识
	TaskID        string     `json:"task_id"`                  // 关联任务 ID
	Token         string     `json:"token"`                    // 访问令牌
	Name          string     `json:"name,omitempty"`           // 链接名称
	Status        LinkStatus `json:"status"`                   // 链接状态
	Password      string     `json:"password,omitempty"`       // 访问密码（哈希）
	MaxDownloads  int        `json:"max_downloads,omitempty"`  // 最大下载次数
	MaxUploads    int        `json:"max_uploads,omitempty"`    // 最大上传次数
	UploadCount   int        `json:"upload_count"`             // 已上传次数
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`     // 过期时间
	CreatedBy     string     `json:"created_by"`               // 创建者
	CreatedAt     time.Time  `json:"created_at"`               // 创建时间
	LastAccessAt  *time.Time `json:"last_access_at,omitempty"` // 最后访问时间
	AccessCount   int        `json:"access_count"`             // 访问次数
	CustomMessage string     `json:"custom_message,omitempty"` // 自定义欢迎消息
}

// UploadRecord 上传记录
type UploadRecord struct {
	ID           string       `json:"id"`                       // 唯一标识
	TaskID       string       `json:"task_id"`                  // 关联任务 ID
	LinkID       string       `json:"link_id"`                  // 关联链接 ID
	FileName     string       `json:"file_name"`                // 原始文件名
	StoredName   string       `json:"stored_name"`              // 存储文件名
	FilePath     string       `json:"file_path"`                // 存储路径
	FileSize     int64        `json:"file_size"`                // 文件大小
	MimeType     string       `json:"mime_type"`                // MIME 类型
	Checksum     string       `json:"checksum"`                 // 文件校验和
	Status       UploadStatus `json:"status"`                   // 上传状态
	UploaderIP   string       `json:"uploader_ip"`              // 上传者 IP
	UploaderName string       `json:"uploader_name,omitempty"`  // 上传者姓名
	UploaderNote string       `json:"uploader_note,omitempty"`  // 上传者备注
	Category     StorageCategory `json:"category,omitempty"`    // 文件分类
	UploadedAt   time.Time    `json:"uploaded_at"`              // 上传时间
	CompletedAt  *time.Time   `json:"completed_at,omitempty"`   // 完成时间
}

// StorageStats 存储统计
type StorageStats struct {
	TaskID       string `json:"task_id"`        // 任务 ID
	TotalFiles   int    `json:"total_files"`    // 总文件数
	TotalSize    int64  `json:"total_size"`     // 总大小
	ByCategory   map[StorageCategory]int64     `json:"by_category"`    // 按分类统计
	ByExtension  map[string]int64             `json:"by_extension"`   // 按扩展名统计
	OldestFile   *time.Time `json:"oldest_file,omitempty"`  // 最早文件
	NewestFile   *time.Time `json:"newest_file,omitempty"`  // 最新文件
}

// NotificationPreference 通知偏好
type NotificationPreference struct {
	Channel NotificationChannel `json:"channel"`          // 通知渠道
	Target  string              `json:"target"`           // 通知目标（邮箱、URL 等）
	Types   []NotificationType  `json:"types"`            // 关注的通知类型
	Enabled bool                `json:"enabled"`          // 是否启用
}

// Notification 通知消息
type Notification struct {
	ID        string             `json:"id"`         // 唯一标识
	TaskID    string             `json:"task_id"`    // 关联任务
	Type      NotificationType   `json:"type"`       // 通知类型
	Channel   NotificationChannel `json:"channel"`   // 通知渠道
	Target    string             `json:"target"`     // 通知目标
	Subject   string             `json:"subject"`    // 通知主题
	Body      string             `json:"body"`       // 通知内容
	Sent      bool               `json:"sent"`       // 是否已发送
	SentAt    *time.Time         `json:"sent_at,omitempty"` // 发送时间
	CreatedAt time.Time          `json:"created_at"` // 创建时间
}

// AccessLogEntry 访问日志条目
type AccessLogEntry struct {
	ID         string    `json:"id"`                    // 唯一标识
	TaskID     string    `json:"task_id"`               // 关联任务 ID
	LinkID     string    `json:"link_id"`               // 关联链接 ID
	Action     string    `json:"action"`                // 操作类型：view, upload, download
	FileName   string    `json:"file_name,omitempty"`   // 文件名
	FileSize   int64     `json:"file_size,omitempty"`   // 文件大小
	IP         string    `json:"ip"`                    // 客户端 IP
	UserAgent  string    `json:"user_agent"`            // 用户代理
	UploaderName string  `json:"uploader_name,omitempty"` // 上传者姓名
	Success    bool      `json:"success"`               // 是否成功
	ErrorMsg   string    `json:"error_msg,omitempty"`   // 错误信息
	Timestamp  time.Time `json:"timestamp"`             // 时间戳
}

// ==================== 请求/响应结构 ====================

// CreateTaskRequest 创建收集任务请求
type CreateTaskRequest struct {
	Name          string                   `json:"name" binding:"required"`
	Description   string                   `json:"description,omitempty"`
	MaxSize       int64                    `json:"max_size,omitempty"`
	MaxFiles      int                      `json:"max_files,omitempty"`
	AllowedExts   []string                 `json:"allowed_exts,omitempty"`
	BlockedExts   []string                 `json:"blocked_exts,omitempty"`
	RequireAuth   bool                     `json:"require_auth"`
	UploadPassword string                  `json:"upload_password,omitempty"`
	AutoClassify  bool                     `json:"auto_classify"`
	ExpiresAt     *time.Time               `json:"expires_at,omitempty"`
	Notifications []NotificationPreference `json:"notifications,omitempty"`
}

// UpdateTaskRequest 更新收集任务请求
type UpdateTaskRequest struct {
	Name          *string                  `json:"name,omitempty"`
	Description   *string                  `json:"description,omitempty"`
	Status        *TaskStatus              `json:"status,omitempty"`
	MaxSize       *int64                   `json:"max_size,omitempty"`
	MaxFiles      *int                     `json:"max_files,omitempty"`
	AllowedExts   []string                 `json:"allowed_exts,omitempty"`
	BlockedExts   []string                 `json:"blocked_exts,omitempty"`
	RequireAuth   *bool                    `json:"require_auth,omitempty"`
	UploadPassword *string                 `json:"upload_password,omitempty"`
	AutoClassify  *bool                    `json:"auto_classify,omitempty"`
	ExpiresAt     *time.Time               `json:"expires_at,omitempty"`
	Notifications []NotificationPreference `json:"notifications,omitempty"`
}

// CreateLinkRequest 创建分享链接请求
type CreateLinkRequest struct {
	Name          string     `json:"name,omitempty"`
	Password      string     `json:"password,omitempty"`
	MaxDownloads  int        `json:"max_downloads,omitempty"`
	MaxUploads    int        `json:"max_uploads,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	CustomMessage string     `json:"custom_message,omitempty"`
}

// UploadRequest 上传请求元数据
type UploadRequest struct {
	UploaderName string `json:"uploader_name,omitempty"` // 上传者姓名
	UploaderNote string `json:"uploader_note,omitempty"` // 上传者备注
}

// VerifyPasswordRequest 验证密码请求
type VerifyPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

// APIResponse 统一 API 响应结构
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// PaginatedResponse 分页响应
type PaginatedResponse struct {
	Items      interface{} `json:"items"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// TaskListResponse 任务列表响应
type TaskListResponse struct {
	Tasks []*CollectionTask `json:"tasks"`
	Total int               `json:"total"`
}

// LinkListResponse 链接列表响应
type LinkListResponse struct {
	Links []*ShareLink `json:"links"`
	Total int          `json:"total"`
}

// AccessLogResponse 访问日志响应
type AccessLogResponse struct {
	Logs  []*AccessLogEntry `json:"logs"`
	Total int               `json:"total"`
}

// UploadListResponse 上传记录列表响应
type UploadListResponse struct {
	Uploads []*UploadRecord `json:"uploads"`
	Total   int             `json:"total"`
}
