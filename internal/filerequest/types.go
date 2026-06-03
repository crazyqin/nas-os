// Package filerequest 提供文件收集请求功能，支持创建文件收集请求、生成分享链接、
// 匿名上传、过期管理、上传限制及邮件通知。参考群晖 DSM 7.3 的文件请求功能。
package filerequest

import "time"

// RequestStatus 文件请求状态
type RequestStatus string

const (
	// RequestStatusActive 活跃状态，可接受上传
	RequestStatusActive RequestStatus = "active"
	// RequestStatusExpired 已过期
	RequestStatusExpired RequestStatus = "expired"
	// RequestStatusClosed 已关闭
	RequestStatusClosed RequestStatus = "closed"
	// RequestStatusDisabled 已禁用
	RequestStatusDisabled RequestStatus = "disabled"
)

// UploadStatus 上传状态
type UploadStatus string

const (
	// UploadStatusSuccess 上传成功
	UploadStatusSuccess UploadStatus = "success"
	// UploadStatusFailed 上传失败
	UploadStatusFailed UploadStatus = "failed"
	// UploadStatusPending 等待处理
	UploadStatusPending UploadStatus = "pending"
)

// FileRequest 文件收集请求
type FileRequest struct {
	// 唯一标识
	ID string `json:"id"`
	// 请求标题
	Title string `json:"title"`
	// 请求描述
	Description string `json:"description,omitempty"`
	// 创建者用户ID
	CreatorID string `json:"creator_id"`
	// 创建者用户名
	CreatorName string `json:"creator_name"`
	// 文件保存目录
	DestinationPath string `json:"destination_path"`
	// 请求状态
	Status RequestStatus `json:"status"`
	// 过期时间
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// 是否允许匿名上传
	AllowAnonymous bool `json:"allow_anonymous"`
	// 是否需要上传者信息
	RequireUploaderInfo bool `json:"require_uploader_info"`
	// 最大文件数量限制（0表示不限制）
	MaxFileCount int `json:"max_file_count"`
	// 单文件最大大小（字节，0表示不限制）
	MaxFileSize int64 `json:"max_file_size"`
	// 允许的文件扩展名（空表示不限制）
	AllowedExtensions []string `json:"allowed_extensions,omitempty"`
	// 是否发送邮件通知
	NotifyOnUpload bool `json:"notify_on_upload"`
	// 通知邮箱列表
	NotifyEmails []string `json:"notify_emails,omitempty"`
	// 已接收的文件数量
	ReceivedFileCount int `json:"received_file_count"`
	// 已接收的文件总大小（字节）
	ReceivedTotalSize int64 `json:"received_total_size"`
	// 创建时间
	CreatedAt time.Time `json:"created_at"`
	// 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// RequestLink 分享链接
type RequestLink struct {
	// 唯一标识
	ID string `json:"id"`
	// 关联的文件请求ID
	RequestID string `json:"request_id"`
	// 分享链接URL
	URL string `json:"url"`
	// 访问令牌
	Token string `json:"token"`
	// 链接是否启用
	IsActive bool `json:"is_active"`
	// 访问密码（可选）
	Password string `json:"password,omitempty"`
	// 最大访问次数（0表示不限制）
	MaxAccessCount int `json:"max_access_count"`
	// 已访问次数
	AccessCount int `json:"access_count"`
	// 过期时间
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// 创建时间
	CreatedAt time.Time `json:"created_at"`
	// 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// UploadInfo 上传文件信息
type UploadInfo struct {
	// 唯一标识
	ID string `json:"id"`
	// 关联的文件请求ID
	RequestID string `json:"request_id"`
	// 关联的分享链接ID
	LinkID string `json:"link_id"`
	// 原始文件名
	OriginalName string `json:"original_name"`
	// 存储文件名
	StoredName string `json:"stored_name"`
	// 文件大小（字节）
	FileSize int64 `json:"file_size"`
	// 文件MIME类型
	MimeType string `json:"mime_type"`
	// 文件扩展名
	Extension string `json:"extension"`
	// 存储路径
	StoragePath string `json:"storage_path"`
	// 上传状态
	Status UploadStatus `json:"status"`
	// 上传者名称（如果需要）
	UploaderName string `json:"uploader_name,omitempty"`
	// 上传者邮箱（如果需要）
	UploaderEmail string `json:"uploader_email,omitempty"`
	// 上传者IP地址
	UploaderIP string `json:"uploader_ip,omitempty"`
	// 备注
	Comment string `json:"comment,omitempty"`
	// 上传时间
	UploadedAt time.Time `json:"uploaded_at"`
}

// RequestStats 文件请求统计信息
type RequestStats struct {
	// 总请求数
	TotalRequests int `json:"total_requests"`
	// 活跃请求数
	ActiveRequests int `json:"active_requests"`
	// 过期请求数
	ExpiredRequests int `json:"expired_requests"`
	// 已关闭请求数
	ClosedRequests int `json:"closed_requests"`
	// 总上传文件数
	TotalUploads int `json:"total_uploads"`
	// 今日上传文件数
	TodayUploads int `json:"today_uploads"`
	// 总上传大小（字节）
	TotalUploadSize int64 `json:"total_upload_size"`
	// 今日上传大小（字节）
	TodayUploadSize int64 `json:"today_upload_size"`
	// 总访问次数
	TotalAccessCount int `json:"total_access_count"`
	// 活跃链接数
	ActiveLinks int `json:"active_links"`
}

// CreateRequestRequest 创建文件请求
type CreateRequestRequest struct {
	// 请求标题
	Title string `json:"title" binding:"required"`
	// 请求描述
	Description string `json:"description,omitempty"`
	// 创建者用户ID
	CreatorID string `json:"creator_id" binding:"required"`
	// 创建者用户名
	CreatorName string `json:"creator_name" binding:"required"`
	// 文件保存目录
	DestinationPath string `json:"destination_path" binding:"required"`
	// 过期时间（可选）
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// 是否允许匿名上传
	AllowAnonymous bool `json:"allow_anonymous"`
	// 是否需要上传者信息
	RequireUploaderInfo bool `json:"require_uploader_info"`
	// 最大文件数量限制
	MaxFileCount int `json:"max_file_count,omitempty"`
	// 单文件最大大小（字节）
	MaxFileSize int64 `json:"max_file_size,omitempty"`
	// 允许的文件扩展名
	AllowedExtensions []string `json:"allowed_extensions,omitempty"`
	// 是否发送邮件通知
	NotifyOnUpload bool `json:"notify_on_upload"`
	// 通知邮箱列表
	NotifyEmails []string `json:"notify_emails,omitempty"`
}

// UpdateRequestRequest 更新文件请求
type UpdateRequestRequest struct {
	// 请求标题
	Title string `json:"title,omitempty"`
	// 请求描述
	Description string `json:"description,omitempty"`
	// 过期时间
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// 是否允许匿名上传
	AllowAnonymous *bool `json:"allow_anonymous,omitempty"`
	// 是否需要上传者信息
	RequireUploaderInfo *bool `json:"require_uploader_info,omitempty"`
	// 最大文件数量限制
	MaxFileCount *int `json:"max_file_count,omitempty"`
	// 单文件最大大小（字节）
	MaxFileSize *int64 `json:"max_file_size,omitempty"`
	// 允许的文件扩展名
	AllowedExtensions []string `json:"allowed_extensions,omitempty"`
	// 是否发送邮件通知
	NotifyOnUpload *bool `json:"notify_on_upload"`
	// 通知邮箱列表
	NotifyEmails []string `json:"notify_emails,omitempty"`
}

// CreateLinkRequest 创建分享链接
type CreateLinkRequest struct {
	// 是否需要密码
	Password string `json:"password,omitempty"`
	// 最大访问次数
	MaxAccessCount int `json:"max_access_count,omitempty"`
	// 过期时间
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// UploadFileRequest 上传文件请求
type UploadFileRequest struct {
	// 原始文件名
	OriginalName string `json:"original_name" binding:"required"`
	// 文件大小（字节）
	FileSize int64 `json:"file_size" binding:"required"`
	// 文件MIME类型
	MimeType string `json:"mime_type,omitempty"`
	// 上传者名称
	UploaderName string `json:"uploader_name,omitempty"`
	// 上传者邮箱
	UploaderEmail string `json:"uploader_email,omitempty"`
	// 备注
	Comment string `json:"comment,omitempty"`
}

// ListRequestsRequest 列出请求
type ListRequestsRequest struct {
	// 按创建者ID过滤
	CreatorID string `json:"creator_id,omitempty"`
	// 按状态过滤
	Status RequestStatus `json:"status,omitempty"`
	// 页码（从1开始）
	Page int `json:"page,omitempty"`
	// 每页数量
	PageSize int `json:"page_size,omitempty"`
}

// DefaultUploadLimits 默认上传限制
func DefaultUploadLimits() (maxFileCount int, maxFileSize int64) {
	return 100, 1024 * 1024 * 1024 // 100个文件，1GB
}
