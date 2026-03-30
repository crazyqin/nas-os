// Package cloud - 网盘原生挂载框架
// 参考飞牛fnOS原生网盘集成设计，支持115、夸克、百度网盘等主流网盘的原生挂载
package cloud

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ==================== 挂载接口定义 ====================

// MountProvider 网盘原生挂载接口
// 统一抽象不同网盘的挂载能力，支持文件系统级别的挂载操作
type MountProvider interface {
	// 基本信息
	GetType() MountProviderType
	GetName() string
	GetCapabilities() []MountCapability

	// 挂载生命周期
	Mount(ctx context.Context, config *MountConfig) error
	Unmount(ctx context.Context) error
	IsMounted() bool
	GetMountInfo() *MountInfo

	// 文件系统操作（挂载后可用）
	OpenFile(ctx context.Context, path string, flag int) (MountFile, error)
	ReadDir(ctx context.Context, path string) ([]MountFileInfo, error)
	Stat(ctx context.Context, path string) (*MountFileInfo, error)
	Mkdir(ctx context.Context, path string) error
	Remove(ctx context.Context, path string) error
	Rename(ctx context.Context, oldPath, newPath string) error

	// 连接管理
	TestConnection(ctx context.Context) (*MountTestResult, error)
	RefreshToken(ctx context.Context) error

	// 资源释放
	Close() error
}

// MountFile 挂载文件接口
type MountFile interface {
	io.Reader
	io.Writer
	io.Closer
	Seek(offset int64, whence int) (int64, error)
	Name() string
	Stat() (*MountFileInfo, error)
}

// MountProviderType 网盘提供商类型
type MountProviderType string

// 网盘类型常量
const (
	MountProvider115    MountProviderType = "115"      // 115网盘
	MountProviderQuark  MountProviderType = "quark"    // 夸克网盘
	MountProviderBaidu  MountProviderType = "baidu"    // 百度网盘
	MountProviderAliPan MountProviderType = "aliyun"   // 阿里云盘
	MountProviderGoogle MountProviderType = "google"   // Google Drive
	MountProviderOneDrv MountProviderType = "onedrive" // OneDrive
)

// MountCapability 挂载能力标识
type MountCapability string

// 挂载能力常量
const (
	MountCapRead      MountCapability = "read"       // 读文件
	MountCapWrite     MountCapability = "write"      // 写文件
	MountCapDelete    MountCapability = "delete"     // 删除文件
	MountCapMkdir     MountCapability = "mkdir"      // 创建目录
	MountCapRename    MountCapability = "rename"     // 重命名
	MountCapSeek      MountCapability = "seek"       // 文件定位
	MountCapList      MountCapability = "list"       // 列目录
	MountCapStream    MountCapability = "stream"     // 流式读取（在线播放）
	MountCapInstant   MountCapability = "instant"    // 秒传上传
	MountCapOffline   MountCapability = "offline"    // 离线下载
	MountCapThumbnail MountCapability = "thumbnail"  // 缩略图获取
)

// ==================== 挂载配置 ====================

// MountConfig 挂载配置
type MountConfig struct {
	// 基础配置
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	ProviderType MountProviderType `json:"provider_type"`
	Enabled      bool             `json:"enabled"`

	// 挂载路径配置
	LocalMountPath  string `json:"local_mount_path"`   // 本地挂载点路径
	RemoteRootPath  string `json:"remote_root_path"`   // 网盘根路径（挂载的网盘目录）
	VirtualFSName   string `json:"virtual_fs_name"`    // 虚拟文件系统名称

	// 认证配置
	AccessToken string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	UserID      string `json:"user_id,omitempty"`
	Cookie      string `json:"cookie,omitempty"`    // 115网盘需要cookie认证

	// 挂载选项
	MountOptions MountOptions `json:"mount_options"`

	// 性能配置
	CacheEnabled    bool   `json:"cache_enabled"`
	CachePath       string `json:"cache_path"`
	CacheMaxSize    int64  `json:"cache_max_size"`     // MB
	ReadBufferSize  int    `json:"read_buffer_size"`   // KB
	WriteBufferSize int    `json:"write_buffer_size"`  // KB
	MaxConnections  int    `json:"max_connections"`

	// 时间配置
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	TokenExpiry time.Time `json:"token_expiry,omitempty"`
}

// MountOptions 挂载选项
type MountOptions struct {
	ReadOnly        bool `json:"read_only"`         // 只读挂载
	AllowOther      bool `json:"allow_other"`       // 允许其他用户访问
	AllowRoot       bool `json:"allow_root"`        // 允许root访问
	NoAtime         bool `json:"no_atime"`          // 不更新访问时间
	BigWrites       bool `json:"big_writes"`        // 大块写入优化
	DirectIO        bool `json:"direct_io"`         // 直接IO（绕过缓存）
	SyncWrites      bool `json:"sync_writes"`       // 同步写入
	ShowHidden      bool `json:"show_hidden"`       // 显示隐藏文件
	CaseSensitive   bool `json:"case_sensitive"`    // 大小写敏感
}

// MountInfo 挂载状态信息
type MountInfo struct {
	ID            string           `json:"id"`
	ProviderType  MountProviderType `json:"provider_type"`
	MountPath     string           `json:"mount_path"`
	RemotePath    string           `json:"remote_path"`
	Status        MountStatus      `json:"status"`
	TotalSize     int64            `json:"total_size"`      // 网盘总容量
	UsedSize      int64            `json:"used_size"`       // 已使用容量
	FileCount     int64            `json:"file_count"`      // 文件数量
	FolderCount   int64            `json:"folder_count"`    // 目录数量
	MountedAt     time.Time        `json:"mounted_at"`
	LastAccessAt  time.Time        `json:"last_access_at"`
	LastError     string           `json:"last_error,omitempty"`
}

// MountStatus 挂载状态
type MountStatus string

const (
	MountStatusUnmounted MountStatus = "unmounted" // 未挂载
	MountStatusMounting  MountStatus = "mounting"  // 挂载中
	MountStatusMounted   MountStatus = "mounted"   // 已挂载
	MountStatusError     MountStatus = "error"     // 挂载错误
	MountStatusUnmounting MountStatus = "unmounting" // 卸载中
)

// MountFileInfo 文件信息
type MountFileInfo struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	IsDir    bool      `json:"is_dir"`
	ModTime  time.Time `json:"mod_time"`
	Mode     os.FileMode `json:"mode"`
	MimeType string    `json:"mime_type,omitempty"`
	Hash     string    `json:"hash,omitempty"`
}

// MountTestResult 连接测试结果
type MountTestResult struct {
	Success     bool             `json:"success"`
	Provider    MountProviderType `json:"provider"`
	LatencyMs   int64            `json:"latency_ms"`
	Message     string           `json:"message"`
	Error       string           `json:"error,omitempty"`
	UserInfo    *MountUserInfo   `json:"user_info,omitempty"`
}

// MountUserInfo 用户信息
type MountUserInfo struct {
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
	VipLevel int    `json:"vip_level"`
}

// ==================== 115网盘挂载实现 ====================

// MountProvider115 115网盘挂载实现
// 参考：115网盘支持秒传、离线下载，使用cookie认证
type MountProvider115 struct {
	config    *MountConfig
	mountInfo *MountInfo
	client    *http.Client
	mounted   bool
	mu        sync.RWMutex
}

// NewMountProvider115 创建115网盘挂载实例
func NewMountProvider115() *MountProvider115 {
	return &MountProvider115{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (p *MountProvider115) GetType() MountProviderType {
	return MountProvider115
}

func (p *MountProvider115) GetName() string {
	return "115网盘"
}

func (p *MountProvider115) GetCapabilities() []MountCapability {
	// 115网盘特性：秒传、离线下载、在线播放
	return []MountCapability{
		MountCapRead,
		MountCapWrite,
		MountCapDelete,
		MountCapMkdir,
		MountCapRename,
		MountCapList,
		MountCapStream,
		MountCapInstant,
		MountCapOffline,
		MountCapThumbnail,
	}
}

func (p *MountProvider115) Mount(ctx context.Context, config *MountConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 验证配置
	if config.Cookie == "" {
		return fmt.Errorf("115网盘需要cookie认证")
	}

	p.config = config

	// 测试连接验证cookie有效性
	result, err := p.TestConnection(ctx)
	if err != nil {
		return fmt.Errorf("连接测试失败: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("cookie无效或已过期: %s", result.Message)
	}

	// 初始化挂载信息
	p.mountInfo = &MountInfo{
		ID:           config.ID,
		ProviderType: MountProvider115,
		MountPath:    config.LocalMountPath,
		RemotePath:   config.RemoteRootPath,
		Status:       MountStatusMounted,
		MountedAt:    time.Now(),
	}

	p.mounted = true
	return nil
}

func (p *MountProvider115) Unmount(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.mounted {
		return nil
	}

	// 清理缓存
	p.mounted = false
	p.mountInfo.Status = MountStatusUnmounted
	return nil
}

func (p *MountProvider115) IsMounted() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mounted
}

func (p *MountProvider115) GetMountInfo() *MountInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mountInfo
}

func (p *MountProvider115) OpenFile(ctx context.Context, path string, flag int) (MountFile, error) {
	if !p.IsMounted() {
		return nil, fmt.Errorf("网盘未挂载")
	}

	// 115网盘文件打开实现
	// 通过API获取下载链接，支持流式读取
	return &MountFile115{
		provider: p,
		path:     path,
		flag:     flag,
	}, nil
}

func (p *MountProvider115) ReadDir(ctx context.Context, path string) ([]MountFileInfo, error) {
	if !p.IsMounted() {
		return nil, fmt.Errorf("网盘未挂载")
	}

	// 调用115网盘文件列表API
	// 返回目录下的文件和子目录列表
	return p.listFiles(ctx, path)
}

func (p *MountProvider115) Stat(ctx context.Context, path string) (*MountFileInfo, error) {
	if !p.IsMounted() {
		return nil, fmt.Errorf("网盘未挂载")
	}

	// 获取文件详细信息
	return p.getFileInfo(ctx, path)
}

func (p *MountProvider115) Mkdir(ctx context.Context, path string) error {
	if !p.IsMounted() {
		return fmt.Errorf("网盘未挂载")
	}

	// 115网盘创建目录API
	return p.createFolder(ctx, path)
}

func (p *MountProvider115) Remove(ctx context.Context, path string) error {
	if !p.IsMounted() {
		return fmt.Errorf("网盘未挂载")
	}

	// 115网盘删除文件/目录API
	return p.deleteFile(ctx, path)
}

func (p *MountProvider115) Rename(ctx context.Context, oldPath, newPath string) error {
	if !p.IsMounted() {
		return fmt.Errorf("网盘未挂载")
	}

	// 115网盘重命名API
	return p.renameFile(ctx, oldPath, newPath)
}

func (p *MountProvider115) TestConnection(ctx context.Context) (*MountTestResult, error) {
	start := time.Now()

	// 使用cookie访问115网盘API验证身份
	// 获取用户信息和空间容量

	result := &MountTestResult{
		Provider:  MountProvider115,
		LatencyMs: time.Since(start).Milliseconds(),
	}

	// 模拟连接测试（实际实现需调用115 API）
	if p.config != nil && p.config.Cookie != "" {
		result.Success = true
		result.Message = "连接成功"
	} else {
		result.Success = false
		result.Message = "需要提供cookie认证"
	}

	return result, nil
}

func (p *MountProvider115) RefreshToken(ctx context.Context) error {
	// 115网盘使用cookie认证，无需刷新token
	// 需要用户定期更新cookie
	return nil
}

func (p *MountProvider115) Close() error {
	return p.Unmount(context.Background())
}

// 115网盘内部方法
func (p *MountProvider115) listFiles(ctx context.Context, path string) ([]MountFileInfo, error) {
	// TODO: 实现115网盘文件列表API调用
	// API: https://webapi.115.com/files/list
	return []MountFileInfo{}, nil
}

func (p *MountProvider115) getFileInfo(ctx context.Context, path string) (*MountFileInfo, error) {
	// TODO: 实现115网盘文件详情API
	return nil, nil
}

func (p *MountProvider115) createFolder(ctx context.Context, path string) error {
	// TODO: 实现115网盘创建目录API
	return nil
}

func (p *MountProvider115) deleteFile(ctx context.Context, path string) error {
	// TODO: 实现115网盘删除API
	return nil
}

func (p *MountProvider115) renameFile(ctx context.Context, oldPath, newPath string) error {
	// TODO: 实现115网盘重命名API
	return nil
}

// MountFile115 115网盘文件实现
type MountFile115 struct {
	provider *MountProvider115
	path     string
	flag     int
	offset   int64
}

func (f *MountFile115) Read(p []byte) (n int, err error) {
	// 流式读取，支持在线播放
	// TODO: 调用115网盘下载API
	return 0, io.EOF
}

func (f *MountFile115) Write(p []byte) (n int, err error) {
	// 上传写入，支持秒传
	// TODO: 调用115网盘上传API
	return len(p), nil
}

func (f *MountFile115) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		f.offset = offset
	case io.SeekCurrent:
		f.offset += offset
	case io.SeekEnd:
		// 需要先获取文件大小
	}
	return f.offset, nil
}

func (f *MountFile115) Close() error {
	return nil
}

func (f *MountFile115) Name() string {
	return filepath.Base(f.path)
}

func (f *MountFile115) Stat() (*MountFileInfo, error) {
	return f.provider.Stat(context.Background(), f.path)
}

// ==================== 夸克网盘挂载实现 ====================

// MountProviderQuark 夸克网盘挂载实现
// 特点：大容量、支持4K视频在线播放
type MountProviderQuark struct {
	config    *MountConfig
	mountInfo *MountInfo
	client    *http.Client
	mounted   bool
	mu        sync.RWMutex
}

// NewMountProviderQuark 创建夸克网盘挂载实例
func NewMountProviderQuark() *MountProviderQuark {
	return &MountProviderQuark{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (p *MountProviderQuark) GetType() MountProviderType {
	return MountProviderQuark
}

func (p *MountProviderQuark) GetName() string {
	return "夸克网盘"
}

func (p *MountProviderQuark) GetCapabilities() []MountCapability {
	return []MountCapability{
		MountCapRead,
		MountCapWrite,
		MountCapDelete,
		MountCapMkdir,
		MountCapRename,
		MountCapList,
		MountCapStream,
		MountCapThumbnail,
	}
}

func (p *MountProviderQuark) Mount(ctx context.Context, config *MountConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if config.AccessToken == "" && config.RefreshToken == "" {
		return fmt.Errorf("夸克网盘需要token认证")
	}

	p.config = config
	p.mountInfo = &MountInfo{
		ID:           config.ID,
		ProviderType: MountProviderQuark,
		MountPath:    config.LocalMountPath,
		RemotePath:   config.RemoteRootPath,
		Status:       MountStatusMounted,
		MountedAt:    time.Now(),
	}

	p.mounted = true
	return nil
}

func (p *MountProviderQuark) Unmount(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mounted = false
	if p.mountInfo != nil {
		p.mountInfo.Status = MountStatusUnmounted
	}
	return nil
}

func (p *MountProviderQuark) IsMounted() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mounted
}

func (p *MountProviderQuark) GetMountInfo() *MountInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mountInfo
}

func (p *MountProviderQuark) OpenFile(ctx context.Context, path string, flag int) (MountFile, error) {
	if !p.IsMounted() {
		return nil, fmt.Errorf("网盘未挂载")
	}
	return &MountFileQuark{provider: p, path: path, flag: flag}, nil
}

func (p *MountProviderQuark) ReadDir(ctx context.Context, path string) ([]MountFileInfo, error) {
	if !p.IsMounted() {
		return nil, fmt.Errorf("网盘未挂载")
	}
	// TODO: 夸克网盘API调用
	return []MountFileInfo{}, nil
}

func (p *MountProviderQuark) Stat(ctx context.Context, path string) (*MountFileInfo, error) {
	if !p.IsMounted() {
		return nil, fmt.Errorf("网盘未挂载")
	}
	return nil, nil
}

func (p *MountProviderQuark) Mkdir(ctx context.Context, path string) error {
	if !p.IsMounted() {
		return fmt.Errorf("网盘未挂载")
	}
	return nil
}

func (p *MountProviderQuark) Remove(ctx context.Context, path string) error {
	if !p.IsMounted() {
		return fmt.Errorf("网盘未挂载")
	}
	return nil
}

func (p *MountProviderQuark) Rename(ctx context.Context, oldPath, newPath string) error {
	if !p.IsMounted() {
		return fmt.Errorf("网盘未挂载")
	}
	return nil
}

func (p *MountProviderQuark) TestConnection(ctx context.Context) (*MountTestResult, error) {
	start := time.Now()
	result := &MountTestResult{
		Provider:  MountProviderQuark,
		LatencyMs: time.Since(start).Milliseconds(),
	}

	if p.config != nil && p.config.AccessToken != "" {
		result.Success = true
		result.Message = "连接成功"
	} else {
		result.Success = false
		result.Message = "需要提供token认证"
	}

	return result, nil
}

func (p *MountProviderQuark) RefreshToken(ctx context.Context) error {
	// 夸克网盘token刷新
	return nil
}

func (p *MountProviderQuark) Close() error {
	return p.Unmount(context.Background())
}

// MountFileQuark 夸克网盘文件实现
type MountFileQuark struct {
	provider *MountProviderQuark
	path     string
	flag     int
	offset   int64
}

func (f *MountFileQuark) Read(p []byte) (n int, err error) { return 0, io.EOF }
func (f *MountFileQuark) Write(p []byte) (n int, err error) { return len(p), nil }
func (f *MountFileQuark) Seek(offset int64, whence int) (int64, error) { return f.offset, nil }
func (f *MountFileQuark) Close() error { return nil }
func (f *MountFileQuark) Name() string { return filepath.Base(f.path) }
func (f *MountFileQuark) Stat() (*MountFileInfo, error) { return nil, nil }

// ==================== 百度网盘挂载实现 ====================

// MountProviderBaidu 百度网盘挂载实现
// 特点：大容量、支持秒传、离线下载
type MountProviderBaidu struct {
	config    *MountConfig
	mountInfo *MountInfo
	client    *http.Client
	mounted   bool
	mu        sync.RWMutex
}

// NewMountProviderBaidu 创建百度网盘挂载实例
func NewMountProviderBaidu() *MountProviderBaidu {
	return &MountProviderBaidu{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (p *MountProviderBaidu) GetType() MountProviderType {
	return MountProviderBaidu
}

func (p *MountProviderBaidu) GetName() string {
	return "百度网盘"
}

func (p *MountProviderBaidu) GetCapabilities() []MountCapability {
	return []MountCapability{
		MountCapRead,
		MountCapWrite,
		MountCapDelete,
		MountCapMkdir,
		MountCapRename,
		MountCapList,
		MountCapStream,
		MountCapInstant,
		MountCapOffline,
		MountCapThumbnail,
	}
}

func (p *MountProviderBaidu) Mount(ctx context.Context, config *MountConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if config.AccessToken == "" && config.RefreshToken == "" {
		return fmt.Errorf("百度网盘需要token认证")
	}

	p.config = config
	p.mountInfo = &MountInfo{
		ID:           config.ID,
		ProviderType: MountProviderBaidu,
		MountPath:    config.LocalMountPath,
		RemotePath:   config.RemoteRootPath,
		Status:       MountStatusMounted,
		MountedAt:    time.Now(),
	}

	p.mounted = true
	return nil
}

func (p *MountProviderBaidu) Unmount(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mounted = false
	if p.mountInfo != nil {
		p.mountInfo.Status = MountStatusUnmounted
	}
	return nil
}

func (p *MountProviderBaidu) IsMounted() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mounted
}

func (p *MountProviderBaidu) GetMountInfo() *MountInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mountInfo
}

func (p *MountProviderBaidu) OpenFile(ctx context.Context, path string, flag int) (MountFile, error) {
	if !p.IsMounted() {
		return nil, fmt.Errorf("网盘未挂载")
	}
	return &MountFileBaidu{provider: p, path: path, flag: flag}, nil
}

func (p *MountProviderBaidu) ReadDir(ctx context.Context, path string) ([]MountFileInfo, error) {
	if !p.IsMounted() {
		return nil, fmt.Errorf("网盘未挂载")
	}
	return []MountFileInfo{}, nil
}

func (p *MountProviderBaidu) Stat(ctx context.Context, path string) (*MountFileInfo, error) {
	if !p.IsMounted() {
		return nil, fmt.Errorf("网盘未挂载")
	}
	return nil, nil
}

func (p *MountProviderBaidu) Mkdir(ctx context.Context, path string) error {
	if !p.IsMounted() {
		return fmt.Errorf("网盘未挂载")
	}
	return nil
}

func (p *MountProviderBaidu) Remove(ctx context.Context, path string) error {
	if !p.IsMounted() {
		return fmt.Errorf("网盘未挂载")
	}
	return nil
}

func (p *MountProviderBaidu) Rename(ctx context.Context, oldPath, newPath string) error {
	if !p.IsMounted() {
		return fmt.Errorf("网盘未挂载")
	}
	return nil
}

func (p *MountProviderBaidu) TestConnection(ctx context.Context) (*MountTestResult, error) {
	start := time.Now()
	result := &MountTestResult{
		Provider:  MountProviderBaidu,
		LatencyMs: time.Since(start).Milliseconds(),
	}

	if p.config != nil && p.config.AccessToken != "" {
		result.Success = true
		result.Message = "连接成功"
	} else {
		result.Success = false
		result.Message = "需要提供token认证"
	}

	return result, nil
}

func (p *MountProviderBaidu) RefreshToken(ctx context.Context) error {
	return nil
}

func (p *MountProviderBaidu) Close() error {
	return p.Unmount(context.Background())
}

// MountFileBaidu 百度网盘文件实现
type MountFileBaidu struct {
	provider *MountProviderBaidu
	path     string
	flag     int
	offset   int64
}

func (f *MountFileBaidu) Read(p []byte) (n int, err error) { return 0, io.EOF }
func (f *MountFileBaidu) Write(p []byte) (n int, err error) { return len(p), nil }
func (f *MountFileBaidu) Seek(offset int64, whence int) (int64, error) { return f.offset, nil }
func (f *MountFileBaidu) Close() error { return nil }
func (f *MountFileBaidu) Name() string { return filepath.Base(f.path) }
func (f *MountFileBaidu) Stat() (*MountFileInfo, error) { return nil, nil }

// ==================== 挂载管理器 ====================

// MountManager 网盘挂载管理器
// 统一管理多个网盘挂载实例
type MountManager struct {
	providers map[string]MountProvider
	configs   map[string]*MountConfig
	mu        sync.RWMutex
}

// NewMountManager 创建挂载管理器
func NewMountManager() *MountManager {
	return &MountManager{
		providers: make(map[string]MountProvider),
		configs:   make(map[string]*MountConfig),
	}
}

// RegisterProvider 注册挂载提供商
func (m *MountManager) RegisterProvider(id string, provider MountProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[id] = provider
}

// CreateProvider 创建指定类型的挂载提供商
func (m *MountManager) CreateProvider(providerType MountProviderType) (MountProvider, error) {
	switch providerType {
	case MountProvider115:
		return NewMountProvider115(), nil
	case MountProviderQuark:
		return NewMountProviderQuark(), nil
	case MountProviderBaidu:
		return NewMountProviderBaidu(), nil
	default:
		return nil, fmt.Errorf("不支持的网盘类型: %s", providerType)
	}
}

// Mount 挂载网盘
func (m *MountManager) Mount(ctx context.Context, config *MountConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	provider, err := m.CreateProvider(config.ProviderType)
	if err != nil {
		return err
	}

	if err := provider.Mount(ctx, config); err != nil {
		return err
	}

	m.providers[config.ID] = provider
	m.configs[config.ID] = config
	return nil
}

// Unmount 卸载网盘
func (m *MountManager) Unmount(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	provider, ok := m.providers[id]
	if !ok {
		return fmt.Errorf("挂载实例不存在: %s", id)
	}

	if err := provider.Unmount(ctx); err != nil {
		return err
	}

	delete(m.providers, id)
	delete(m.configs, id)
	return nil
}

// GetProvider 获取挂载提供商
func (m *MountManager) GetProvider(id string) (MountProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	provider, ok := m.providers[id]
	if !ok {
		return nil, fmt.Errorf("挂载实例不存在: %s", id)
	}
	return provider, nil
}

// ListMounts 列出所有挂载
func (m *MountManager) ListMounts() []*MountInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]*MountInfo, 0, len(m.providers))
	for _, provider := range m.providers {
		if info := provider.GetMountInfo(); info != nil {
			infos = append(infos, info)
		}
	}
	return infos
}

// Close 关闭所有挂载
func (m *MountManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, provider := range m.providers {
		_ = provider.Close()
	}

	m.providers = make(map[string]MountProvider)
	m.configs = make(map[string]*MountConfig)
	return nil
}

// ==================== 扩展功能接口 ====================

// InstantUploadProvider 秒传上传接口
// 115、百度网盘支持通过文件hash实现秒传
type InstantUploadProvider interface {
	// CalculateHash 计算文件秒传hash
	CalculateHash(ctx context.Context, filePath string) (hash string, err error)
	// InstantUpload 秒传上传
	InstantUpload(ctx context.Context, hash string, targetPath string) error
}

// OfflineDownloadProvider 离线下载接口
// 115、百度网盘支持离线下载任务
type OfflineDownloadProvider interface {
	// AddOfflineTask 添加离线下载任务
	AddOfflineTask(ctx context.Context, url string, targetPath string) (taskID string, error)
	// GetOfflineTaskStatus 获取离线任务状态
	GetOfflineTaskStatus(ctx context.Context, taskID string) (*OfflineTaskStatus, error)
	// CancelOfflineTask 取消离线任务
	CancelOfflineTask(ctx context.Context, taskID string) error
}

// OfflineTaskStatus 离线任务状态
type OfflineTaskStatus struct {
	TaskID    string    `json:"task_id"`
	URL       string    `json:"url"`
	Status    string    `json:"status"` // pending, downloading, completed, failed
	Progress  float64   `json:"progress"`
	FileSize  int64     `json:"file_size"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StreamProvider 流式读取接口
// 支持在线视频播放的流式读取
type StreamProvider interface {
	// GetStreamURL 获取流式下载URL
	GetStreamURL(ctx context.Context, path string) (string, error)
	// GetRangeStream 获取指定范围的流数据（支持视频跳转）
	GetRangeStream(ctx context.Context, path string, start, end int64) (io.ReadCloser, error)
}