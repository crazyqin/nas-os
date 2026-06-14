// Package webshare 提供 WebShare 数据模型定义。
// 包含文件条目、分享链接、访问日志、API 响应等核心数据结构。
package webshare

import (
	"sync"
	"time"
)

// Permission 访问权限级别
type Permission string

const (
	PermissionView     Permission = "view"     // 只读浏览
	PermissionDownload Permission = "download" // 浏览+下载
	PermissionEdit     Permission = "edit"     // 浏览+下载+编辑
	PermissionAdmin    Permission = "admin"    // 完全管理权限
)

// SortField 排序字段
type SortField string

const (
	SortByName    SortField = "name"
	SortBySize    SortField = "size"
	SortByModTime SortField = "mod_time"
	SortByType    SortField = "type"
)

// SortDirection 排序方向
type SortDirection string

const (
	SortAsc  SortDirection = "asc"
	SortDesc SortDirection = "desc"
)

// FileType 文件类型分类
type FileType string

const (
	FileTypeFile      FileType = "file"
	FileTypeDirectory FileType = "directory"
	FileTypeSymlink   FileType = "symlink"
)

// Protocol 文件访问协议
type Protocol string

const (
	ProtocolLocal Protocol = "local" // 本地文件系统
	ProtocolSMB   Protocol = "smb"   // SMB/CIFS
	ProtocolNFS   Protocol = "nfs"   // NFSv4
	ProtocolAFP   Protocol = "afp"   // Apple Filing Protocol
)

// WebShareConfig WebShare 服务配置
type WebShareConfig struct {
	ListenAddr       string        `json:"listen_addr"`        // 监听地址，如 ":8443"
	BaseURL          string        `json:"base_url"`           // 外部访问基础 URL
	RootPath         string        `json:"root_path"`          // 共享根目录
	MaxFileSize      int64         `json:"max_file_size"`      // 最大上传文件大小（字节）
	DefaultExpiry    time.Duration `json:"default_expiry"`     // 默认分享链接过期时间
	MaxActiveLinks   int           `json:"max_active_links"`   // 最大活跃分享链接数
	EnableFIPS       bool          `json:"enable_fips"`        // 启用 FIPS 加密传输
	EnableSearch     bool          `json:"enable_search"`      // 启用全文搜索
	ShowHiddenFiles  bool          `json:"show_hidden_files"`  // 是否显示隐藏文件
	EnableSnapshots  bool          `json:"enable_snapshots"`   // 启用快照时间线
	EnableThumbnails bool          `json:"enable_thumbnails"`  // 启用缩略图生成
	TLSCertFile      string        `json:"tls_cert_file"`      // TLS 证书路径
	TLSKeyFile       string        `json:"tls_key_file"`       // TLS 私钥路径
	EnableSMB        bool          `json:"enable_smb"`         // 启用 SMB 协议支持
	EnableNFS        bool          `json:"enable_nfs"`         // 启用 NFS 协议支持
	SMBMountPoint    string        `json:"smb_mount_point"`    // SMB 挂载点
	NFSMountPoint    string        `json:"nfs_mount_point"`    // NFS 挂载点
}

// DefaultWebShareConfig 返回默认配置
func DefaultWebShareConfig() *WebShareConfig {
	return &WebShareConfig{
		ListenAddr:       ":8443",
		BaseURL:          "https://localhost:8443",
		RootPath:         "/mnt",
		MaxFileSize:      10 * 1024 * 1024 * 1024, // 10GB
		DefaultExpiry:    7 * 24 * time.Hour,       // 7 天
		MaxActiveLinks:   1000,
		EnableFIPS:       true,
		EnableSearch:     true,
		ShowHiddenFiles:  false,
		EnableSnapshots:  true,
		EnableThumbnails: true,
		EnableSMB:        false,
		EnableNFS:        false,
		SMBMountPoint:    "/mnt/smb",
		NFSMountPoint:    "/mnt/nfs",
	}
}

// Entry 文件/目录条目
type Entry struct {
	Name          string    `json:"name"`                     // 文件名
	Path          string    `json:"path"`                     // 相对路径
	AbsolutePath  string    `json:"absolute_path"`            // 绝对路径
	Type          FileType  `json:"type"`                     // 文件类型
	Size          int64     `json:"size"`                     // 文件大小（字节）
	ModTime       time.Time `json:"mod_time"`                 // 修改时间
	IsHidden      bool      `json:"is_hidden"`                // 是否隐藏文件
	Extension     string    `json:"extension"`                // 文件扩展名
	MimeType      string    `json:"mime_type"`                // MIME 类型
	Permission    string    `json:"permission"`               // 文件权限
	SymlinkTarget string    `json:"symlink_target,omitempty"` // 符号链接目标
	Protocol      Protocol  `json:"protocol"`                 // 访问协议
	ProtocolPath  string    `json:"protocol_path,omitempty"`  // 协议特定路径
}

// DirectoryListing 目录列表响应
type DirectoryListing struct {
	Path       string  `json:"path"`                  // 当前路径
	ParentPath string  `json:"parent_path"`           // 父目录路径
	Entries    []Entry `json:"entries"`               // 文件/目录列表
	TotalCount int     `json:"total_count"`           // 总条目数
	TotalSize  int64   `json:"total_size"`            // 目录总大小
	ShowHidden bool    `json:"show_hidden"`           // 是否显示隐藏文件
	FilteredBy string  `json:"filtered_by,omitempty"` // 过滤条件
	Protocol   Protocol `json:"protocol"`             // 访问协议
}

// ShareLink 分享链接
type ShareLink struct {
	ID            string     `json:"id"`                       // 唯一标识
	Name          string     `json:"name"`                     // 链接名称
	Path          string     `json:"path"`                     // 分享路径
	Token         string     `json:"token"`                    // 访问令牌
	Permission    Permission `json:"permission"`               // 权限级别
	Password      string     `json:"password,omitempty"`       // 访问密码（哈希）
	MaxDownloads  int        `json:"max_downloads,omitempty"`  // 最大下载次数
	DownloadCount int        `json:"download_count"`           // 已下载次数
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`     // 过期时间
	CreatedBy     string     `json:"created_by"`               // 创建者
	CreatedAt     time.Time  `json:"created_at"`               // 创建时间
	UpdatedAt     time.Time  `json:"updated_at"`               // 更新时间
	IsActive      bool       `json:"is_active"`                // 是否激活
	AccessCount   int        `json:"access_count"`             // 访问次数
	LastAccessAt  *time.Time `json:"last_access_at,omitempty"` // 最后访问时间
	Protocol      Protocol   `json:"protocol"`                 // 访问协议
}

// CreateShareLinkRequest 创建分享链接请求
type CreateShareLinkRequest struct {
	Name         string        `json:"name" binding:"required"`
	Path         string        `json:"path" binding:"required"`
	Permission   Permission    `json:"permission"`
	Password     string        `json:"password,omitempty"`
	MaxDownloads int           `json:"max_downloads,omitempty"`
	Expiry       time.Duration `json:"expiry,omitempty"`
	CreatedBy    string        `json:"created_by" binding:"required"`
	Protocol     Protocol      `json:"protocol,omitempty"`
}

// ShareLinkStats 分享链接统计
type ShareLinkStats struct {
	TotalLinks     int `json:"total_links"`
	ActiveLinks    int `json:"active_links"`
	TotalDownloads int `json:"total_downloads"`
	TotalAccesses  int `json:"total_accesses"`
}

// AccessLog 访问日志
type AccessLog struct {
	ID        string    `json:"id"`
	ShareID   string    `json:"share_id"`
	Action    string    `json:"action"`     // view, download, upload, delete
	Path      string    `json:"path"`       // 访问路径
	IP        string    `json:"ip"`         // 客户端 IP
	UserAgent string    `json:"user_agent"` // 用户代理
	UserID    string    `json:"user_id"`    // 用户 ID
	Protocol  Protocol  `json:"protocol"`   // 访问协议
	Timestamp time.Time `json:"timestamp"`
}

// FileOperation 文件操作请求
type FileOperation struct {
	Action  string   `json:"action" binding:"required"` // copy, move, rename, delete
	Paths   []string `json:"paths" binding:"required"`  // 源路径列表
	Dest    string   `json:"dest,omitempty"`            // 目标路径（copy/move/rename 需要）
	NewName string   `json:"new_name,omitempty"`        // 新名称（rename 需要）
}

// CreateFolderRequest 创建文件夹请求
type CreateFolderRequest struct {
	Path     string   `json:"path" binding:"required"` // 文件夹路径
	Name     string   `json:"name" binding:"required"` // 文件夹名称
	Protocol Protocol `json:"protocol,omitempty"`       // 访问协议
}

// UploadResponse 上传响应
type UploadResponse struct {
	FileName string   `json:"file_name"` // 文件名
	Size     int64    `json:"size"`      // 文件大小
	Path     string   `json:"path"`      // 保存路径
	Protocol Protocol `json:"protocol"`  // 访问协议
	Message  string   `json:"message"`   // 提示信息
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query     string   `json:"query" binding:"required"` // 搜索关键词
	Path      string   `json:"path,omitempty"`           // 限定搜索路径
	Limit     int      `json:"limit,omitempty"`          // 结果数量限制
	Offset    int      `json:"offset,omitempty"`         // 分页偏移
	FileTypes []string `json:"file_types,omitempty"`     // 文件类型过滤
	Protocol  Protocol `json:"protocol,omitempty"`        // 协议过滤
}

// Snapshot 快照信息
type Snapshot struct {
	ID        string    `json:"id"`         // 快照 ID
	Name      string    `json:"name"`       // 快照名称
	Path      string    `json:"path"`       // 快照路径
	Size      int64     `json:"size"`       // 快照大小
	CreatedAt time.Time `json:"created_at"` // 创建时间
	IsAuto    bool      `json:"is_auto"`    // 是否自动快照
}

// WebShareStats WebShare 服务整体统计
type WebShareStats struct {
	TotalFiles    int64  `json:"total_files"`     // 总文件数
	TotalDirs     int64  `json:"total_dirs"`      // 总目录数
	TotalSize     int64  `json:"total_size"`      // 总大小
	UsedSpace     int64  `json:"used_space"`      // 已用空间
	FreeSpace     int64  `json:"free_space"`      // 剩余空间
	ActiveLinks   int    `json:"active_links"`    // 活跃分享链接数
	ActiveUsers   int    `json:"active_users"`    // 活跃用户数
	IndexStatus   string `json:"index_status"`    // 索引状态
	IndexedFiles  int64  `json:"indexed_files"`   // 已索引文件数
	SearchEnabled bool   `json:"search_enabled"`  // 搜索是否启用
	FIPSEnabled   bool   `json:"fips_enabled"`    // FIPS 是否启用
	SMBEnabled    bool   `json:"smb_enabled"`     // SMB 是否启用
	NFSEnabled    bool   `json:"nfs_enabled"`     // NFS 是否启用
}

// ProtocolStatus 协议状态
type ProtocolStatus struct {
	Protocol Protocol `json:"protocol"`           // 协议类型
	Enabled  bool     `json:"enabled"`            // 是否启用
	MountPoint string  `json:"mount_point"`       // 挂载点
	Status   string   `json:"status"`             // 状态
	ActiveConnections int `json:"active_connections"` // 活跃连接数
}

// APIResponse 统一 API 响应结构
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// filterCache 缓存目录过滤结果，避免重复计算
type filterCache struct {
	mu    sync.RWMutex
	cache map[string]*DirectoryListing
	ttl   time.Duration
}

// newFilterCache 创建过滤缓存
func newFilterCache(ttl time.Duration) *filterCache {
	return &filterCache{
		cache: make(map[string]*DirectoryListing),
		ttl:   ttl,
	}
}

// get 获取缓存结果
func (fc *filterCache) get(key string) (*DirectoryListing, bool) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	result, ok := fc.cache[key]
	return result, ok
}

// set 设置缓存结果
func (fc *filterCache) set(key string, listing *DirectoryListing) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.cache[key] = listing
}

// invalidate 使缓存失效
func (fc *filterCache) invalidate(key string) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	delete(fc.cache, key)
}

// invalidateAll 清空所有缓存
func (fc *filterCache) invalidateAll() {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.cache = make(map[string]*DirectoryListing)
}
