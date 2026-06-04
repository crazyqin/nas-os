// Package websharepro - 跨协议互操作模块
// 提供 SMB/NFS/WebDAV 统一访问层
// 实现协议无关的文件操作接口
package websharepro

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// ProtocolType 协议类型
type ProtocolType string

const (
	ProtocolSMB     ProtocolType = "smb"
	ProtocolNFS     ProtocolType = "nfs"
	ProtocolWebDAV  ProtocolType = "webdav"
	ProtocolLocal   ProtocolType = "local"
	ProtocolS3      ProtocolType = "s3"
)

// ProtocolStatus 协议状态
type ProtocolStatus string

const (
	StatusActive     ProtocolStatus = "active"
	StatusInactive   ProtocolStatus = "inactive"
	StatusError      ProtocolStatus = "error"
	StatusConnecting ProtocolStatus = "connecting"
)

// FileInfo 文件信息（协议无关）
type FileInfo struct {
	Path        string       `json:"path"`
	Name        string       `json:"name"`
	Size        int64        `json:"size"`
	IsDir       bool         `json:"isDir"`
	Mode        uint32       `json:"mode"`
	ModTime     time.Time    `json:"modTime"`
	AccessTime  time.Time    `json:"accessTime"`
	CreateTime  time.Time    `json:"createTime"`
	Owner       string       `json:"owner"`
	Group       string       `json:"group"`
	ContentType string       `json:"contentType"`
	Protocol    ProtocolType `json:"protocol"`
	Attributes  map[string]any `json:"attributes,omitempty"`
}

// ProtocolConfig 协议配置
type ProtocolConfig struct {
	Type     ProtocolType     `json:"type"`
	Endpoint string           `json:"endpoint"`
	Username string           `json:"username,omitempty"`
	Password string           `json:"password,omitempty"`
	Domain   string           `json:"domain,omitempty"`
	Options  map[string]string `json:"options,omitempty"`
	Timeout  time.Duration    `json:"timeout"`
	MaxConns int              `json:"maxConns"`
}

// ProtocolConnection 协议连接
type ProtocolConnection struct {
	Config    *ProtocolConfig  `json:"config"`
	Status    ProtocolStatus   `json:"status"`
	ConnectedAt time.Time      `json:"connectedAt"`
	LastUsed  time.Time        `json:"lastUsed"`
	Error     string           `json:"error,omitempty"`
	connID    string
}

// ProtocolAdapter 协议适配器接口
type ProtocolAdapter interface {
	Connect(ctx context.Context, config *ProtocolConfig) error
	Disconnect() error
	Stat(ctx context.Context, path string) (*FileInfo, error)
	ReadDir(ctx context.Context, path string) ([]*FileInfo, error)
	Read(ctx context.Context, path string) (io.ReadCloser, error)
	Write(ctx context.Context, path string, reader io.Reader, size int64) error
	Delete(ctx context.Context, path string) error
	Mkdir(ctx context.Context, path string) error
	Rename(ctx context.Context, oldPath, newPath string) error
	Copy(ctx context.Context, src, dst string) error
	GetStatus() ProtocolStatus
}

// UnifiedFileSystem 统一文件系统
type UnifiedFileSystem struct {
	mu          sync.RWMutex
	adapters    map[ProtocolType]ProtocolAdapter
	connections map[string]*ProtocolConnection
	mounts      map[string]*MountPoint
	transfers   map[string]*TransferTask
	rateLimiter *RateLimiter
}

// MountPoint 挂载点
type MountPoint struct {
	Path       string         `json:"path"`
	Protocol   ProtocolType   `json:"protocol"`
	RemotePath string         `json:"remotePath"`
	Config     *ProtocolConfig `json:"config"`
	ReadOnly   bool           `json:"readOnly"`
	AutoMount  bool           `json:"autoMount"`
	MountedAt  time.Time      `json:"mountedAt"`
	IsActive   bool           `json:"isActive"`
}

// TransferTask 传输任务
type TransferTask struct {
	ID          string        `json:"id"`
	SrcPath     string        `json:"srcPath"`
	DstPath     string        `json:"dstPath"`
	SrcProtocol ProtocolType  `json:"srcProtocol"`
	DstProtocol ProtocolType  `json:"dstProtocol"`
	Size        int64         `json:"size"`
	Transferred int64         `json:"transferred"`
	Status      string        `json:"status"`
	Error       string        `json:"error,omitempty"`
	StartTime   time.Time     `json:"startTime"`
	EndTime     *time.Time    `json:"endTime,omitempty"`
	Speed       int64         `json:"speed"` // bytes/sec
}

// RateLimiter 速率限制器
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[ProtocolType]*tokenBucket
}

type tokenBucket struct {
	tokens   float64
	maxToken float64
	rate     float64
	lastTime time.Time
}

// NewUnifiedFileSystem 创建统一文件系统
func NewUnifiedFileSystem() *UnifiedFileSystem {
	fs := &UnifiedFileSystem{
		adapters:    make(map[ProtocolType]ProtocolAdapter),
		connections: make(map[string]*ProtocolConnection),
		mounts:      make(map[string]*MountPoint),
		transfers:   make(map[string]*TransferTask),
		rateLimiter: &RateLimiter{
			limiters: make(map[ProtocolType]*tokenBucket),
		},
	}

	// 注册内置适配器
	fs.adapters[ProtocolLocal] = &LocalAdapter{}
	fs.adapters[ProtocolSMB] = &SMBAdapter{}
	fs.adapters[ProtocolNFS] = &NFSAdapter{}
	fs.adapters[ProtocolWebDAV] = &WebDAVAdapter{}
	fs.adapters[ProtocolS3] = &S3Adapter{}

	return fs
}

// RegisterAdapter 注册协议适配器
func (fs *UnifiedFileSystem) RegisterAdapter(protocol ProtocolType, adapter ProtocolAdapter) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.adapters[protocol] = adapter
}

// Mount 挂载远程路径
func (fs *UnifiedFileSystem) Mount(mountPoint string, config *ProtocolConfig) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	adapter, exists := fs.adapters[config.Type]
	if !exists {
		return fmt.Errorf("unsupported protocol: %s", config.Type)
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	if err := adapter.Connect(ctx, config); err != nil {
		return fmt.Errorf("connect to %s: %w", config.Endpoint, err)
	}

	fs.mounts[mountPoint] = &MountPoint{
		Path:       mountPoint,
		Protocol:   config.Type,
		RemotePath: config.Endpoint,
		Config:     config,
		MountedAt:  time.Now(),
		IsActive:   true,
	}

	connID := fmt.Sprintf("%s-%d", config.Type, time.Now().UnixNano())
	fs.connections[connID] = &ProtocolConnection{
		Config:      config,
		Status:      StatusActive,
		ConnectedAt: time.Now(),
		LastUsed:    time.Now(),
		connID:      connID,
	}

	return nil
}

// Unmount 卸载路径
func (fs *UnifiedFileSystem) Unmount(mountPoint string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	mount, exists := fs.mounts[mountPoint]
	if !exists {
		return fmt.Errorf("mount point not found: %s", mountPoint)
	}

	adapter, ok := fs.adapters[mount.Protocol]
	if ok {
		adapter.Disconnect()
	}

	mount.IsActive = false
	delete(fs.mounts, mountPoint)
	return nil
}

// Stat 获取文件信息
func (fs *UnifiedFileSystem) Stat(ctx context.Context, path string) (*FileInfo, error) {
	fs.mu.RLock()
	adapter, mountPath, protocol := fs.resolvePath(path)
	fs.mu.RUnlock()

	if adapter == nil {
		return nil, fmt.Errorf("no adapter for path: %s", path)
	}

	info, err := adapter.Stat(ctx, mountPath)
	if err != nil {
		return nil, err
	}

	info.Protocol = protocol
	return info, nil
}

// ReadDir 读取目录
func (fs *UnifiedFileSystem) ReadDir(ctx context.Context, path string) ([]*FileInfo, error) {
	fs.mu.RLock()
	adapter, mountPath, protocol := fs.resolvePath(path)
	fs.mu.RUnlock()

	if adapter == nil {
		return nil, fmt.Errorf("no adapter for path: %s", path)
	}

	entries, err := adapter.ReadDir(ctx, mountPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		entry.Protocol = protocol
	}
	return entries, nil
}

// ReadFile 读取文件
func (fs *UnifiedFileSystem) ReadFile(ctx context.Context, path string) (io.ReadCloser, error) {
	fs.mu.RLock()
	adapter, mountPath, _ := fs.resolvePath(path)
	fs.mu.RUnlock()

	if adapter == nil {
		return nil, fmt.Errorf("no adapter for path: %s", path)
	}

	return adapter.Read(ctx, mountPath)
}

// WriteFile 写入文件
func (fs *UnifiedFileSystem) WriteFile(ctx context.Context, path string, reader io.Reader, size int64) error {
	fs.mu.RLock()
	adapter, mountPath, _ := fs.resolvePath(path)
	fs.mu.RUnlock()

	if adapter == nil {
		return fmt.Errorf("no adapter for path: %s", path)
	}

	return adapter.Write(ctx, mountPath, reader, size)
}

// Delete 删除文件
func (fs *UnifiedFileSystem) Delete(ctx context.Context, path string) error {
	fs.mu.RLock()
	adapter, mountPath, _ := fs.resolvePath(path)
	fs.mu.RUnlock()

	if adapter == nil {
		return fmt.Errorf("no adapter for path: %s", path)
	}

	return adapter.Delete(ctx, mountPath)
}

// Mkdir 创建目录
func (fs *UnifiedFileSystem) Mkdir(ctx context.Context, path string) error {
	fs.mu.RLock()
	adapter, mountPath, _ := fs.resolvePath(path)
	fs.mu.RUnlock()

	if adapter == nil {
		return fmt.Errorf("no adapter for path: %s", path)
	}

	return adapter.Mkdir(ctx, mountPath)
}

// Rename 重命名
func (fs *UnifiedFileSystem) Rename(ctx context.Context, oldPath, newPath string) error {
	fs.mu.RLock()
	adapterOld, mountOld, _ := fs.resolvePath(oldPath)
	_, mountNew, _ := fs.resolvePath(newPath)
	fs.mu.RUnlock()

	if adapterOld == nil {
		return fmt.Errorf("no adapter for path: %s", oldPath)
	}

	return adapterOld.Rename(ctx, mountOld, mountNew)
}

// Copy 跨协议复制
func (fs *UnifiedFileSystem) Copy(ctx context.Context, srcPath, dstPath string) (*TransferTask, error) {
	fs.mu.RLock()
	srcAdapter, srcMount, srcProtocol := fs.resolvePath(srcPath)
	dstAdapter, dstMount, dstProtocol := fs.resolvePath(dstPath)
	fs.mu.RUnlock()

	if srcAdapter == nil {
		return nil, fmt.Errorf("no adapter for source: %s", srcPath)
	}
	if dstAdapter == nil {
		return nil, fmt.Errorf("no adapter for destination: %s", dstPath)
	}

	// 读取源文件
	reader, err := srcAdapter.Read(ctx, srcMount)
	if err != nil {
		return nil, fmt.Errorf("read source: %w", err)
	}
	defer reader.Close()

	// 获取源文件大小
	srcInfo, err := srcAdapter.Stat(ctx, srcMount)
	if err != nil {
		return nil, fmt.Errorf("stat source: %w", err)
	}

	// 创建传输任务
	taskID := fmt.Sprintf("transfer-%d", time.Now().UnixNano())
	task := &TransferTask{
		ID:          taskID,
		SrcPath:     srcPath,
		DstPath:     dstPath,
		SrcProtocol: srcProtocol,
		DstProtocol: dstProtocol,
		Size:        srcInfo.Size,
		Status:      "running",
		StartTime:   time.Now(),
	}

	fs.mu.Lock()
	fs.transfers[taskID] = task
	fs.mu.Unlock()

	// 异步执行复制
	go func() {
		startTime := time.Now()

		// 使用带进度跟踪的 reader
		progressReader := &progressReader{
			reader: reader,
			onRead: func(n int) {
				fs.mu.Lock()
				task.Transferred += int64(n)
				elapsed := time.Since(startTime).Seconds()
				if elapsed > 0 {
					task.Speed = int64(float64(task.Transferred) / elapsed)
				}
				fs.mu.Unlock()
			},
		}

		err := dstAdapter.Write(ctx, dstMount, progressReader, srcInfo.Size)
		now := time.Now()
		task.EndTime = &now

		if err != nil {
			task.Status = "failed"
			task.Error = err.Error()
		} else {
			task.Status = "completed"
			task.Transferred = srcInfo.Size
		}
	}()

	return task, nil
}

// GetTransfer 获取传输任务状态
func (fs *UnifiedFileSystem) GetTransfer(taskID string) (*TransferTask, bool) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	task, exists := fs.transfers[taskID]
	return task, exists
}

// ListMounts 列出挂载点
func (fs *UnifiedFileSystem) ListMounts() []*MountPoint {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	var mounts []*MountPoint
	for _, mount := range fs.mounts {
		mounts = append(mounts, mount)
	}
	return mounts
}

// GetSupportedProtocols 获取支持的协议列表
func (fs *UnifiedFileSystem) GetSupportedProtocols() []ProtocolType {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	var protocols []ProtocolType
	for p := range fs.adapters {
		protocols = append(protocols, p)
	}
	return protocols
}

// resolvePath 解析路径到协议适配器
func (fs *UnifiedFileSystem) resolvePath(path string) (ProtocolAdapter, string, ProtocolType) {
	// 匹配最长前缀
	var bestMatch *MountPoint
	bestLen := 0

	for _, mount := range fs.mounts {
		if mount.IsActive && len(mount.Path) > bestLen && (path == mount.Path || len(path) > len(mount.Path) && path[:len(mount.Path)] == mount.Path) {
			bestMatch = mount
			bestLen = len(mount.Path)
		}
	}

	if bestMatch == nil {
		// 默认使用本地适配器
		if adapter, ok := fs.adapters[ProtocolLocal]; ok {
			return adapter, path, ProtocolLocal
		}
		return nil, "", ""
	}

	adapter, ok := fs.adapters[bestMatch.Protocol]
	if !ok {
		return nil, "", ""
	}

	// 计算相对路径
	relativePath := path[len(bestMatch.Path):]
	if relativePath == "" {
		relativePath = "/"
	} else if relativePath[0] != '/' {
		relativePath = "/" + relativePath
	}

	remotePath := bestMatch.RemotePath + relativePath
	return adapter, remotePath, bestMatch.Protocol
}

// progressReader 带进度回调的 reader
type progressReader struct {
	reader io.Reader
	onRead func(int)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 && pr.onRead != nil {
		pr.onRead(n)
	}
	return n, err
}

// ---- 协议适配器实现 ----

// LocalAdapter 本地文件系统适配器
type LocalAdapter struct {
	connected bool
}

func (a *LocalAdapter) Connect(_ context.Context, _ *ProtocolConfig) error {
	a.connected = true
	return nil
}

func (a *LocalAdapter) Disconnect() error {
	a.connected = false
	return nil
}

func (a *LocalAdapter) Stat(_ context.Context, _ string) (*FileInfo, error) {
	return nil, errors.New("local adapter: not implemented")
}

func (a *LocalAdapter) ReadDir(_ context.Context, _ string) ([]*FileInfo, error) {
	return nil, errors.New("local adapter: not implemented")
}

func (a *LocalAdapter) Read(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, errors.New("local adapter: not implemented")
}

func (a *LocalAdapter) Write(_ context.Context, _ string, _ io.Reader, _ int64) error {
	return errors.New("local adapter: not implemented")
}

func (a *LocalAdapter) Delete(_ context.Context, _ string) error {
	return errors.New("local adapter: not implemented")
}

func (a *LocalAdapter) Mkdir(_ context.Context, _ string) error {
	return errors.New("local adapter: not implemented")
}

func (a *LocalAdapter) Rename(_ context.Context, _, _ string) error {
	return errors.New("local adapter: not implemented")
}

func (a *LocalAdapter) Copy(_ context.Context, _, _ string) error {
	return errors.New("local adapter: not implemented")
}

func (a *LocalAdapter) GetStatus() ProtocolStatus {
	if a.connected {
		return StatusActive
	}
	return StatusInactive
}

// SMBAdapter SMB 协议适配器（桩实现）
type SMBAdapter struct{ connected bool }

func (a *SMBAdapter) Connect(_ context.Context, _ *ProtocolConfig) error {
	a.connected = true
	return nil
}
func (a *SMBAdapter) Disconnect() error                                    { a.connected = false; return nil }
func (a *SMBAdapter) Stat(_ context.Context, _ string) (*FileInfo, error)  { return nil, errors.New("smb: not implemented") }
func (a *SMBAdapter) ReadDir(_ context.Context, _ string) ([]*FileInfo, error) { return nil, errors.New("smb: not implemented") }
func (a *SMBAdapter) Read(_ context.Context, _ string) (io.ReadCloser, error)  { return nil, errors.New("smb: not implemented") }
func (a *SMBAdapter) Write(_ context.Context, _ string, _ io.Reader, _ int64) error { return errors.New("smb: not implemented") }
func (a *SMBAdapter) Delete(_ context.Context, _ string) error             { return errors.New("smb: not implemented") }
func (a *SMBAdapter) Mkdir(_ context.Context, _ string) error              { return errors.New("smb: not implemented") }
func (a *SMBAdapter) Rename(_ context.Context, _, _ string) error          { return errors.New("smb: not implemented") }
func (a *SMBAdapter) Copy(_ context.Context, _, _ string) error            { return errors.New("smb: not implemented") }
func (a *SMBAdapter) GetStatus() ProtocolStatus {
	if a.connected { return StatusActive }
	return StatusInactive
}

// NFSAdapter NFS 协议适配器（桩实现）
type NFSAdapter struct{ connected bool }

func (a *NFSAdapter) Connect(_ context.Context, _ *ProtocolConfig) error {
	a.connected = true
	return nil
}
func (a *NFSAdapter) Disconnect() error                                    { a.connected = false; return nil }
func (a *NFSAdapter) Stat(_ context.Context, _ string) (*FileInfo, error)  { return nil, errors.New("nfs: not implemented") }
func (a *NFSAdapter) ReadDir(_ context.Context, _ string) ([]*FileInfo, error) { return nil, errors.New("nfs: not implemented") }
func (a *NFSAdapter) Read(_ context.Context, _ string) (io.ReadCloser, error)  { return nil, errors.New("nfs: not implemented") }
func (a *NFSAdapter) Write(_ context.Context, _ string, _ io.Reader, _ int64) error { return errors.New("nfs: not implemented") }
func (a *NFSAdapter) Delete(_ context.Context, _ string) error             { return errors.New("nfs: not implemented") }
func (a *NFSAdapter) Mkdir(_ context.Context, _ string) error              { return errors.New("nfs: not implemented") }
func (a *NFSAdapter) Rename(_ context.Context, _, _ string) error          { return errors.New("nfs: not implemented") }
func (a *NFSAdapter) Copy(_ context.Context, _, _ string) error            { return errors.New("nfs: not implemented") }
func (a *NFSAdapter) GetStatus() ProtocolStatus {
	if a.connected { return StatusActive }
	return StatusInactive
}

// WebDAVAdapter WebDAV 协议适配器（桩实现）
type WebDAVAdapter struct{ connected bool }

func (a *WebDAVAdapter) Connect(_ context.Context, _ *ProtocolConfig) error {
	a.connected = true
	return nil
}
func (a *WebDAVAdapter) Disconnect() error                                    { a.connected = false; return nil }
func (a *WebDAVAdapter) Stat(_ context.Context, _ string) (*FileInfo, error)  { return nil, errors.New("webdav: not implemented") }
func (a *WebDAVAdapter) ReadDir(_ context.Context, _ string) ([]*FileInfo, error) { return nil, errors.New("webdav: not implemented") }
func (a *WebDAVAdapter) Read(_ context.Context, _ string) (io.ReadCloser, error)  { return nil, errors.New("webdav: not implemented") }
func (a *WebDAVAdapter) Write(_ context.Context, _ string, _ io.Reader, _ int64) error { return errors.New("webdav: not implemented") }
func (a *WebDAVAdapter) Delete(_ context.Context, _ string) error             { return errors.New("webdav: not implemented") }
func (a *WebDAVAdapter) Mkdir(_ context.Context, _ string) error              { return errors.New("webdav: not implemented") }
func (a *WebDAVAdapter) Rename(_ context.Context, _, _ string) error          { return errors.New("webdav: not implemented") }
func (a *WebDAVAdapter) Copy(_ context.Context, _, _ string) error            { return errors.New("webdav: not implemented") }
func (a *WebDAVAdapter) GetStatus() ProtocolStatus {
	if a.connected { return StatusActive }
	return StatusInactive
}

// S3Adapter S3 协议适配器（桩实现）
type S3Adapter struct{ connected bool }

func (a *S3Adapter) Connect(_ context.Context, _ *ProtocolConfig) error {
	a.connected = true
	return nil
}
func (a *S3Adapter) Disconnect() error                                    { a.connected = false; return nil }
func (a *S3Adapter) Stat(_ context.Context, _ string) (*FileInfo, error)  { return nil, errors.New("s3: not implemented") }
func (a *S3Adapter) ReadDir(_ context.Context, _ string) ([]*FileInfo, error) { return nil, errors.New("s3: not implemented") }
func (a *S3Adapter) Read(_ context.Context, _ string) (io.ReadCloser, error)  { return nil, errors.New("s3: not implemented") }
func (a *S3Adapter) Write(_ context.Context, _ string, _ io.Reader, _ int64) error { return errors.New("s3: not implemented") }
func (a *S3Adapter) Delete(_ context.Context, _ string) error             { return errors.New("s3: not implemented") }
func (a *S3Adapter) Mkdir(_ context.Context, _ string) error              { return errors.New("s3: not implemented") }
func (a *S3Adapter) Rename(_ context.Context, _, _ string) error          { return errors.New("s3: not implemented") }
func (a *S3Adapter) Copy(_ context.Context, _, _ string) error            { return errors.New("s3: not implemented") }
func (a *S3Adapter) GetStatus() ProtocolStatus {
	if a.connected { return StatusActive }
	return StatusInactive
}
