// Package cloudfuse provides cloud storage mounting via FUSE
// 类型定义
package cloudfuse

import (
	"context"
	"os"
	"sync"
	"time"

	"nas-os/internal/cloudsync"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

// MountType 挂载类型
type MountType string

const (
	MountType115         MountType = "115"          // 115网盘
	MountTypeQuark       MountType = "quark"        // 夸克网盘
	MountTypeAliyunPan   MountType = "aliyun_pan"   // 阿里云盘
	MountTypeOneDrive    MountType = "onedrive"     // Microsoft OneDrive
	MountTypeGoogleDrive MountType = "google_drive" // Google Drive
	MountTypeWebDAV      MountType = "webdav"       // WebDAV
	MountTypeS3          MountType = "s3"            // S3 兼容存储
)

// MountStatus 挂载状态
type MountStatus string

const (
	MountStatusIdle       MountStatus = "idle"       // 空闲
	MountStatusMounting   MountStatus = "mounting"   // 挂载中
	MountStatusMounted    MountStatus = "mounted"    // 已挂载
	MountStatusUnmounting MountStatus = "unmounting" // 卸载中
	MountStatusError      MountStatus = "error"      // 错误
)

// MountConfig 挂载配置
type MountConfig struct {
	// 基本信息
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Type        MountType  `json:"type"`
	MountPoint  string     `json:"mountPoint"`
	RemotePath  string     `json:"remotePath,omitempty"`
	Enabled     bool       `json:"enabled"`
	AutoMount   bool       `json:"autoMount"`
	ReadOnly    bool       `json:"readOnly"`
	AllowOther  bool       `json:"allowOther"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`

	// 缓存配置
	CacheEnabled bool   `json:"cacheEnabled"`
	CacheDir     string `json:"cacheDir,omitempty"`
	CacheSize    int64  `json:"cacheSize,omitempty"` // MB

	// 115网盘 / 夸克网盘 / 阿里云盘
	AccessToken  string `json:"accessToken,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	UserID       string `json:"userId,omitempty"`
	DriveID      string `json:"driveId,omitempty"`

	// S3 / WebDAV
	Endpoint   string `json:"endpoint,omitempty"`
	Bucket     string `json:"bucket,omitempty"`
	AccessKey  string `json:"accessKey,omitempty"`
	SecretKey  string `json:"secretKey,omitempty"`
	Region     string `json:"region,omitempty"`
	PathStyle  bool   `json:"pathStyle,omitempty"`
	Insecure   bool   `json:"insecure,omitempty"`

	// OneDrive / Google Drive
	ClientID   string `json:"clientId,omitempty"`
	TenantID   string `json:"tenantId,omitempty"`

	// 其他
	RootFolder string `json:"rootFolder,omitempty"`
}

// MountInfo 挂载信息（API 返回）
type MountInfo struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Type           MountType    `json:"type"`
	MountPoint     string       `json:"mountPoint"`
	RemotePath     string       `json:"remotePath,omitempty"`
	Status         MountStatus  `json:"status"`
	Enabled        bool         `json:"enabled"`
	AutoMount      bool         `json:"autoMount"`
	ReadOnly       bool         `json:"readOnly"`
	CreatedAt      time.Time    `json:"createdAt"`
	UpdatedAt      time.Time    `json:"updatedAt"`
	MountedAt      *time.Time   `json:"mountedAt,omitempty"`
	Error          string       `json:"error,omitempty"`
	ReadBytes      int64        `json:"readBytes"`
	WriteBytes     int64        `json:"writeBytes"`
	ReadOps        int64        `json:"readOps"`
	WriteOps       int64        `json:"writeOps"`
	CacheHitRate   float64      `json:"cacheHitRate"`
	CacheUsedBytes int64        `json:"cacheUsedBytes"`
}

// MountStats 挂载统计
type MountStats struct {
	MountID        string    `json:"mountId"`
	StartTime      time.Time `json:"startTime"`
	Uptime         int64     `json:"uptime"` // seconds
	TotalReadBytes  int64    `json:"totalReadBytes"`
	TotalWriteBytes int64    `json:"totalWriteBytes"`
	TotalReadOps    int64    `json:"totalReadOps"`
	TotalWriteOps   int64    `json:"totalWriteOps"`
	CacheHits       int64    `json:"cacheHits"`
	CacheMisses     int64    `json:"cacheMisses"`
}

// MountRequest 挂载请求
type MountRequest struct {
	Name        string    `json:"name" binding:"required"`
	Type        MountType `json:"type" binding:"required"`
	MountPoint  string    `json:"mountPoint" binding:"required"`
	RemotePath  string    `json:"remotePath,omitempty"`
	AutoMount   bool      `json:"autoMount"`
	ReadOnly    bool      `json:"readOnly"`
	AllowOther  bool      `json:"allowOther"`

	// 缓存配置
	CacheEnabled bool   `json:"cacheEnabled"`
	CacheDir     string `json:"cacheDir,omitempty"`
	CacheSize    int64  `json:"cacheSize,omitempty"`

	// 115网盘 / 夸克网盘 / 阿里云盘
	AccessToken   string `json:"accessToken,omitempty"`
	RefreshToken  string `json:"refreshToken,omitempty"`
	UserID        string `json:"userId,omitempty"`
	DriveID       string `json:"driveId,omitempty"`

	// S3 / WebDAV
	Endpoint  string `json:"endpoint,omitempty"`
	Bucket    string `json:"bucket,omitempty"`
	AccessKey string `json:"accessKey,omitempty"`
	SecretKey string `json:"secretKey,omitempty"`
	Region    string `json:"region,omitempty"`
	PathStyle bool   `json:"pathStyle,omitempty"`
	Insecure  bool   `json:"insecure,omitempty"`

	// OneDrive / Google Drive
	ClientID string `json:"clientId,omitempty"`
	TenantID string `json:"tenantId,omitempty"`

	// 其他
	RootFolder string `json:"rootFolder,omitempty"`
}

// MountListResponse 挂载列表响应
type MountListResponse struct {
	Total int64       `json:"total"`
	Items []MountInfo `json:"items"`
}

// OperationResult 操作结果
type OperationResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ProviderInfo 提供商信息
type ProviderInfo struct {
	Type        MountType `json:"type"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Features    []string  `json:"features"`
}

// SupportedProviders 返回支持的提供商列表
func SupportedProviders() []ProviderInfo {
	return []ProviderInfo{
		{
			Type:        MountType115,
			Name:        "115网盘",
			Description: "115网盘挂载",
			Features:    []string{"read", "write", "stream"},
		},
		{
			Type:        MountTypeQuark,
			Name:        "夸克网盘",
			Description: "夸克网盘挂载",
			Features:    []string{"read", "write", "stream"},
		},
		{
			Type:        MountTypeAliyunPan,
			Name:        "阿里云盘",
			Description: "阿里云盘挂载",
			Features:    []string{"read", "write", "stream"},
		},
		{
			Type:        MountTypeOneDrive,
			Name:        "OneDrive",
			Description: "Microsoft OneDrive 挂载",
			Features:    []string{"read", "write", "stream"},
		},
		{
			Type:        MountTypeGoogleDrive,
			Name:        "Google Drive",
			Description: "Google Drive 挂载",
			Features:    []string{"read", "write", "stream"},
		},
		{
			Type:        MountTypeWebDAV,
			Name:        "WebDAV",
			Description: "WebDAV 协议挂载",
			Features:    []string{"read", "write"},
		},
		{
			Type:        MountTypeS3,
			Name:        "S3",
			Description: "S3 兼容存储挂载",
			Features:    []string{"read", "write", "multipart"},
		},
	}
}

// CacheManager 缓存管理器
type CacheManager struct {
	mu        sync.RWMutex
	cacheDir  string
	maxSize   int64 // bytes
	usedSize  int64
	hits      int64
	misses    int64
	cacheMap  map[string]*cacheEntry
}

type cacheEntry struct {
	path      string
	size      int64
	createdAt time.Time
}

// NewCacheManager 创建缓存管理器
func NewCacheManager(cacheDir string, maxSizeMB int64) (*CacheManager, error) {
	return &CacheManager{
		cacheDir: cacheDir,
		maxSize:  maxSizeMB * 1024 * 1024,
		cacheMap: make(map[string]*cacheEntry),
	}, nil
}

// Get 获取缓存
func (c *CacheManager) Get(remotePath string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.cacheMap[remotePath]
	if !ok {
		c.misses++
		return "", false
	}

	c.hits++
	return entry.path, true
}

// Put 添加缓存
func (c *CacheManager) Put(remotePath, localPath string, size int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cacheMap[remotePath] = &cacheEntry{
		path:      localPath,
		size:      size,
		createdAt: time.Now(),
	}
	c.usedSize += size

	return nil
}

// Remove 删除缓存
func (c *CacheManager) Remove(remotePath string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.cacheMap[remotePath]; ok {
		c.usedSize -= entry.size
		delete(c.cacheMap, remotePath)
	}
}

// Clear 清空缓存
func (c *CacheManager) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cacheMap = make(map[string]*cacheEntry)
	c.usedSize = 0

	return nil
}

// Close 关闭缓存管理器
func (c *CacheManager) Close() error {
	return nil
}

// GetCachePath 获取缓存路径
func (c *CacheManager) GetCachePath(remotePath string) string {
	return c.cacheDir + remotePath
}

// UsedSize 已用缓存大小
func (c *CacheManager) UsedSize() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.usedSize
}

// HitRate 缓存命中率
func (c *CacheManager) HitRate() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	if total == 0 {
		return 0
	}
	return float64(c.hits) / float64(total)
}

// Stats 获取缓存统计
func (c *CacheManager) Stats() (hits, misses, evictions, usedSize, maxSize int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits, c.misses, 0, c.usedSize, c.maxSize
}

// CloudFS FUSE 文件系统
type CloudFS struct {
	config  *MountConfig
	provider cloudsync.Provider
	cache   *CacheManager
	stats   *MountStats
	mu      sync.RWMutex
}

// NewCloudFS 创建云文件系统
func NewCloudFS(config *MountConfig, provider cloudsync.Provider, cache *CacheManager) (*CloudFS, error) {
	return &CloudFS{
		config:   config,
		provider: provider,
		cache:    cache,
		stats: &MountStats{
			StartTime: time.Now(),
		},
	}, nil
}

// Root 返回根节点
func (f *CloudFS) Root() (fs.Node, error) {
	return &DirNode{
		fs:   f,
		path: "/",
	}, nil
}

// DirNode 目录节点
type DirNode struct {
	fs   *CloudFS
	path string
}

// FileNode 文件节点
type FileNode struct {
	fs   *CloudFS
	path string
	info *cloudsync.FileInfo
}

// Attr 实现 fs.Node 接口 - DirNode
func (d *DirNode) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0755
	a.Uid = uint32(0)
	a.Gid = uint32(0)
	return nil
}

// Attr 实现 fs.Node 接口 - FileNode
func (f *FileNode) Attr(ctx context.Context, a *fuse.Attr) error {
	if f.info != nil {
		a.Mode = 0644
		a.Size = uint64(f.info.Size)
		a.Mtime = f.info.ModTime
	} else {
		a.Mode = 0644
	}
	a.Uid = uint32(0)
	a.Gid = uint32(0)
	return nil
}

// Lookup 实现 fs.NodeStringLookuper 接口
func (d *DirNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	remotePath := d.path + name

	// 检查是否为目录
	files, err := d.fs.provider.List(ctx, d.path, false)
	if err != nil {
		return nil, fuse.ENOENT
	}

	for _, file := range files {
		if file.Path == remotePath || file.Path == remotePath+"/" {
			if file.IsDir {
				return &DirNode{
					fs:   d.fs,
					path: remotePath + "/",
				}, nil
			}
			return &FileNode{
				fs:   d.fs,
				path: remotePath,
				info: &file,
			}, nil
		}
	}

	return nil, fuse.ENOENT
}

// ReadDirAll 实现 fs.HandleReadDirAller 接口
func (d *DirNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	files, err := d.fs.provider.List(ctx, d.path, false)
	if err != nil {
		return nil, err
	}

	var entries []fuse.Dirent
	for _, file := range files {
		name := file.Path
		if len(name) > 0 && name[len(name)-1] == '/' {
			name = name[:len(name)-1]
		}
		// 只取最后一段
		for i := len(name) - 1; i >= 0; i-- {
			if name[i] == '/' {
				name = name[i+1:]
				break
			}
		}

		if file.IsDir {
			entries = append(entries, fuse.Dirent{
				Name: name,
				Type: fuse.DT_Dir,
			})
		} else {
			entries = append(entries, fuse.Dirent{
				Name: name,
				Type: fuse.DT_File,
			})
		}
	}

	return entries, nil
}

// Read 实现 fs.HandleReader 接口
func (f *FileNode) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	// TODO: 实现文件读取逻辑
	// 这需要从 provider 下载文件内容
	return fuse.ENOSYS
}

// Write 实现 fs.HandleWriter 接口
func (f *FileNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	// TODO: 实现文件写入逻辑
	if f.fs.config.ReadOnly {
		return fuse.Errno(30) // EROFS - read-only file system
	}
	return fuse.ENOSYS
}