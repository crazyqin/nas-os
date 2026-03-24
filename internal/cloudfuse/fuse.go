// Package cloudfuse provides cloud storage mounting via FUSE
// FUSE文件系统实现 - 将网盘映射为本地目录
package cloudfuse

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"nas-os/internal/cloudsync"
)

// CloudFS 云存储文件系统
type CloudFS struct {
	mu          sync.RWMutex
	config      *MountConfig
	provider    cloudsync.Provider
	root        *Dir
	cache       *CacheManager
	inoMap      map[string]uint64 // path -> inode
	nextIno     uint64
	uid         uint32
	gid         uint32
	ctx         context.Context
	cancel      context.CancelFunc
	stats       *MountStats
}

// NewCloudFS 创建云存储文件系统
func NewCloudFS(cfg *MountConfig, provider cloudsync.Provider, cache *CacheManager) (*CloudFS, error) {
	ctx, cancel := context.WithCancel(context.Background())

	fsys := &CloudFS{
		config:   cfg,
		provider: provider,
		cache:    cache,
		inoMap:   make(map[string]uint64),
		nextIno:  2, // 1 保留给root
		uid:      uint32(os.Getuid()),
		gid:      uint32(os.Getgid()),
		ctx:      ctx,
		cancel:   cancel,
		stats: &MountStats{
			StartTime: time.Now(),
		},
	}

	// 创建根目录
	fsys.root = &Dir{
		fs:     fsys,
		path:   "/",
		inode:  1,
		name:   "",
		node: &FileNode{
			ID:      "root",
			Path:    "/",
			IsDir:   true,
			ModTime: time.Now(),
			Mode:    0755 | syscall.S_IFDIR,
		},
	}

	return fsys, nil
}

// Root 返回根目录
func (f *CloudFS) Root() (fs.Node, error) {
	return f.root, nil
}

// Statfs 返回文件系统统计信息
func (f *CloudFS) Statfs(ctx context.Context, req *fuse.StatfsRequest, resp *fuse.StatfsResponse) error {
	resp.Blocks = 1 << 30 // 1TB total
	resp.Bfree = 1 << 29  // 512GB free
	resp.Bavail = 1 << 29
	resp.Files = 1000000
	resp.Ffree = 500000
	resp.Bsize = 4096
	resp.Namelen = 255
	resp.Frsize = 4096
	return nil
}

// getInode 获取或分配inode
func (f *CloudFS) getInode(path string) uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()

	if ino, ok := f.inoMap[path]; ok {
		return ino
	}

	ino := f.nextIno
	f.nextIno++
	f.inoMap[path] = ino
	return ino
}

// Dir 目录节点
type Dir struct {
	fs    *CloudFS
	path  string
	inode uint64
	name  string
	mu    sync.RWMutex
	node  *FileNode
	loaded bool
}

// Attr 返回目录属性
func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = d.inode
	a.Mode = os.FileMode(d.node.Mode)
	if a.Mode == 0 {
		a.Mode = 0755 | os.ModeDir
	}
	a.Uid = d.fs.uid
	a.Gid = d.fs.gid
	a.Mtime = d.node.ModTime
	a.Ctime = d.node.ModTime
	a.Atime = d.node.ModTime
	return nil
}

// Lookup 查找文件或目录
func (d *Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	d.mu.RLock()
	if d.loaded && d.node.Children != nil {
		for _, child := range d.node.Children {
			if child.Name == name {
				d.mu.RUnlock()
				if child.IsDir {
					return &Dir{
						fs:    d.fs,
						path:  child.Path,
						inode: d.fs.getInode(child.Path),
						name:  name,
						node:  child,
					}, nil
				}
				return &File{
					fs:    d.fs,
					path:  child.Path,
					inode: d.fs.getInode(child.Path),
					name:  name,
					node:  child,
				}, nil
			}
		}
	}
	d.mu.RUnlock()

	// 从远程加载
	remotePath := filepath.Join(d.path, name)
	fileInfo, err := d.fs.provider.Stat(ctx, remotePath)
	if err != nil {
		return nil, fuse.ENOENT
	}

	child := &FileNode{
		ID:       fileInfo.Hash,
		Name:     name,
		Path:     remotePath,
		IsDir:    fileInfo.IsDir,
		Size:     fileInfo.Size,
		ModTime:  fileInfo.ModTime,
		RemoteID: fileInfo.Hash,
	}

	if fileInfo.IsDir {
		return &Dir{
			fs:    d.fs,
			path:  remotePath,
			inode: d.fs.getInode(remotePath),
			name:  name,
			node:  child,
		}, nil
	}

	return &File{
		fs:    d.fs,
		path:  remotePath,
		inode: d.fs.getInode(remotePath),
		name:  name,
		node:  child,
	}, nil
}

// ReadDirAll 列出目录内容
func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 列出远程目录
	files, err := d.fs.provider.List(ctx, d.path, false)
	if err != nil {
		return nil, err
	}

	var entries []fuse.Dirent
	d.node.Children = nil

	for _, f := range files {
		name := filepath.Base(f.Path)
		child := &FileNode{
			Name:     name,
			Path:     f.Path,
			IsDir:    f.IsDir,
			Size:     f.Size,
			ModTime:  f.ModTime,
			RemoteID: f.Hash,
		}

		var dtype fuse.DirentType
		if f.IsDir {
			dtype = fuse.DT_Dir
			child.Mode = 0755 | syscall.S_IFDIR
		} else {
			dtype = fuse.DT_File
			child.Mode = 0644 | syscall.S_IFREG
		}

		d.node.Children = append(d.node.Children, child)
		entries = append(entries, fuse.Dirent{
			Inode: d.fs.getInode(f.Path),
			Type:  dtype,
			Name:  name,
		})
	}

	d.loaded = true
	return entries, nil
}

// Mkdir 创建目录
func (d *Dir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	if d.fs.config.ReadOnly {
		return nil, fuse.EPERM
	}

	remotePath := filepath.Join(d.path, req.Name)

	if err := d.fs.provider.CreateDir(ctx, remotePath); err != nil {
		return nil, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	child := &FileNode{
		Name:    req.Name,
		Path:    remotePath,
		IsDir:   true,
		ModTime: time.Now(),
		Mode:    uint32(req.Mode) | syscall.S_IFDIR,
	}
	d.node.Children = append(d.node.Children, child)

	return &Dir{
		fs:    d.fs,
		path:  remotePath,
		inode: d.fs.getInode(remotePath),
		name:  req.Name,
		node:  child,
	}, nil
}

// Create 创建文件
func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	if d.fs.config.ReadOnly {
		return nil, nil, fuse.EPERM
	}

	remotePath := filepath.Join(d.path, req.Name)

	child := &FileNode{
		Name:    req.Name,
		Path:    remotePath,
		IsDir:   false,
		ModTime: time.Now(),
		Mode:    uint32(req.Mode) | syscall.S_IFREG,
		Dirty:   true,
	}

	file := &File{
		fs:    d.fs,
		path:  remotePath,
		inode: d.fs.getInode(remotePath),
		name:  req.Name,
		node:  child,
	}

	// 创建空的临时文件用于写入
	if d.fs.cache != nil {
		cachePath := d.fs.cache.GetCachePath(remotePath)
		if err := os.MkdirAll(filepath.Dir(cachePath), 0750); err == nil {
			f, err := os.Create(cachePath)
			if err == nil {
				f.Close()
				file.cachedPath = cachePath
				child.CachedPath = cachePath
			}
		}
	}

	d.mu.Lock()
	d.node.Children = append(d.node.Children, child)
	d.mu.Unlock()

	return file, file, nil
}

// Remove 删除文件或目录
func (d *Dir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	if d.fs.config.ReadOnly {
		return fuse.EPERM
	}

	remotePath := filepath.Join(d.path, req.Name)

	if req.Dir {
		if err := d.fs.provider.DeleteDir(ctx, remotePath); err != nil {
			return err
		}
	} else {
		if err := d.fs.provider.Delete(ctx, remotePath); err != nil {
			return err
		}
	}

	// 删除缓存
	if d.fs.cache != nil {
		d.fs.cache.Remove(remotePath)
	}

	// 更新本地缓存
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, child := range d.node.Children {
		if child.Name == req.Name {
			d.node.Children = append(d.node.Children[:i], d.node.Children[i+1:]...)
			break
		}
	}

	return nil
}

// Rename 重命名
func (d *Dir) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	if d.fs.config.ReadOnly {
		return fuse.EPERM
	}

	srcPath := filepath.Join(d.path, req.OldName)
	
	destDir, ok := newDir.(*Dir)
	if !ok {
		return fuse.Errno(syscall.EINVAL)
	}
	
	dstPath := filepath.Join(destDir.path, req.NewName)

	// 大多数云盘API不支持原子重命名，使用复制+删除方式
	// 这里暂时返回不支持，等待provider扩展
	_ = srcPath
	_ = dstPath
	return fuse.ENOSYS
}

// File 文件节点
type File struct {
	fs         *CloudFS
	path       string
	inode      uint64
	name       string
	mu         sync.RWMutex
	node       *FileNode
	cachedPath string
	writers    int
}

// Attr 返回文件属性
func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = f.inode
	a.Mode = os.FileMode(f.node.Mode)
	if a.Mode == 0 {
		a.Mode = 0644
	}
	a.Uid = f.fs.uid
	a.Gid = f.fs.gid
	a.Size = uint64(f.node.Size)
	a.Mtime = f.node.ModTime
	a.Ctime = f.node.ModTime
	a.Atime = f.node.ModTime
	return nil
}

// Open 打开文件
func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 检查缓存
	if f.fs.cache != nil {
		cachedPath, ok := f.fs.cache.Get(f.path)
		if ok {
			f.cachedPath = cachedPath
			return f, nil
		}
	}

	// 下载文件到缓存
	if f.fs.cache != nil && !f.node.Dirty {
		cachePath := f.fs.cache.GetCachePath(f.path)
		if err := os.MkdirAll(filepath.Dir(cachePath), 0750); err == nil {
			if err := f.fs.provider.Download(ctx, f.path, cachePath); err == nil {
				f.cachedPath = cachePath
				f.fs.cache.Put(f.path, cachePath, f.node.Size)
				f.node.CachedPath = cachePath
			}
		}
	}

	return f, nil
}

// Read 读取文件内容
func (f *File) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	start := time.Now()
	defer func() {
		f.fs.stats.TotalReadOps++
		f.fs.stats.TotalReadBytes += int64(len(resp.Data))
		f.fs.stats.AvgReadLatencyMs = (f.fs.stats.AvgReadLatencyMs + time.Since(start).Milliseconds()) / 2
	}()

	f.mu.RLock()
	defer f.mu.RUnlock()

	// 如果有缓存文件，从缓存读取
	if f.cachedPath != "" {
		file, err := os.Open(f.cachedPath)
		if err == nil {
			defer file.Close()
			buf := make([]byte, req.Size)
			n, err := file.ReadAt(buf, req.Offset)
			if err != nil && err != io.EOF {
				return err
			}
			resp.Data = buf[:n]
			return nil
		}
	}

	// 直接从远程读取（小文件或无缓存）
	// 注意：大多数云盘API不支持范围读取，需要下载整个文件
	// 这里我们使用临时文件
	tmpPath := filepath.Join(os.TempDir(), "cloudfuse-"+f.path)
	if err := f.fs.provider.Download(ctx, f.path, tmpPath); err != nil {
		return err
	}
	defer os.Remove(tmpPath)

	file, err := os.Open(tmpPath)
	if err != nil {
		return err
	}
	defer file.Close()

	buf := make([]byte, req.Size)
	n, err := file.ReadAt(buf, req.Offset)
	if err != nil && err != io.EOF {
		return err
	}
	resp.Data = buf[:n]
	return nil
}

// Write 写入文件内容
func (f *File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	if f.fs.config.ReadOnly {
		return fuse.EPERM
	}

	start := time.Now()
	defer func() {
		f.fs.stats.TotalWriteOps++
		f.fs.stats.TotalWriteBytes += int64(resp.Size)
		f.fs.stats.AvgWriteLatencyMs = (f.fs.stats.AvgWriteLatencyMs + time.Since(start).Milliseconds()) / 2
	}()

	f.mu.Lock()
	defer f.mu.Unlock()

	// 确保有缓存文件
	if f.cachedPath == "" {
		if f.fs.cache != nil {
			f.cachedPath = f.fs.cache.GetCachePath(f.path)
		} else {
			f.cachedPath = filepath.Join(os.TempDir(), "cloudfuse-"+f.path)
		}
		if err := os.MkdirAll(filepath.Dir(f.cachedPath), 0750); err != nil {
			return err
		}
	}

	// 打开或创建缓存文件
	file, err := os.OpenFile(f.cachedPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// 写入数据
	n, err := file.WriteAt(req.Data, req.Offset)
	if err != nil {
		return err
	}

	resp.Size = n
	f.node.Dirty = true
	f.node.Size = max(f.node.Size, req.Offset+int64(n))
	f.node.ModTime = time.Now()

	return nil
}

// Flush 刷新文件（同步到远程）
func (f *File) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	if !f.node.Dirty || f.fs.config.ReadOnly {
		return nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.cachedPath == "" {
		return nil
	}

	// 上传到远程
	if err := f.fs.provider.Upload(ctx, f.cachedPath, f.path); err != nil {
		return err
	}

	f.node.Dirty = false
	return nil
}

// Release 释放文件句柄
func (f *File) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	// 如果有未保存的修改，自动上传
	if f.node.Dirty && !f.fs.config.ReadOnly {
		go func() {
			if f.cachedPath != "" {
				f.fs.provider.Upload(context.Background(), f.cachedPath, f.path)
				f.mu.Lock()
				f.node.Dirty = false
				f.mu.Unlock()
			}
		}()
	}
	return nil
}

// Fsync 同步文件
func (f *File) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	return f.Flush(ctx, &fuse.FlushRequest{})
}

// Setattr 设置文件属性
func (f *File) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if req.Valid.Size() {
		// 截断文件
		if f.cachedPath != "" {
			if err := os.Truncate(f.cachedPath, int64(req.Size)); err != nil {
				return err
			}
			f.node.Size = int64(req.Size)
			f.node.Dirty = true
		}
	}

	if req.Valid.Mtime() {
		f.node.ModTime = req.Mtime
	}

	return nil
}

// Getxattr 获取扩展属性
func (f *File) Getxattr(ctx context.Context, req *fuse.GetxattrRequest, resp *fuse.GetxattrResponse) error {
	return fuse.ENOSYS
}

// Listxattr 列出扩展属性
func (f *File) Listxattr(ctx context.Context, req *fuse.ListxattrRequest, resp *fuse.ListxattrResponse) error {
	return fuse.ENOSYS
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}