package filemanager

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// Browser 文件浏览器.
type Browser struct {
	rootPath string
	logger   *zap.Logger
}

// NewBrowser 创建文件浏览器.
func NewBrowser(rootPath string, logger *zap.Logger) *Browser {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Browser{
		rootPath: rootPath,
		logger:   logger,
	}
}

// ListDirectory 列出目录内容.
func (b *Browser) ListDirectory(path string, showHidden bool) (*DirectoryListing, error) {
	// 验证并清理路径
	cleanPath, err := b.validatePath(path)
	if err != nil {
		return nil, err
	}

	// 检查路径是否存在
	info, err := os.Stat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("路径不存在: %s", path)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("路径不是目录: %s", path)
	}

	// 读取目录内容
	entries, err := os.ReadDir(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	items := make([]*FileNode, 0, len(entries))
	var totalSize int64

	for _, entry := range entries {
		// 跳过隐藏文件（如果不显示）
		if !showHidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		node, err := b.buildFileNode(cleanPath, entry)
		if err != nil {
			b.logger.Warn("获取文件信息失败",
				zap.String("path", filepath.Join(cleanPath, entry.Name())),
				zap.Error(err))
			continue
		}

		items = append(items, node)
		totalSize += node.Size
	}

	// 排序：目录在前，文件在后，同类型按名称排序
	sort.Slice(items, func(i, j int) bool {
		if items[i].Type != items[j].Type {
			return items[i].Type == FileTypeDirectory
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	// 获取磁盘空间信息
	var diskUsage DiskUsage
	if usage, err := b.getDiskUsage(cleanPath); err == nil {
		diskUsage = *usage
	}

	// 计算父目录
	parent := filepath.Dir(cleanPath)
	if parent == cleanPath {
		parent = ""
	}

	return &DirectoryListing{
		Path:      cleanPath,
		Parent:    parent,
		Items:     items,
		Total:     len(items),
		TotalSize: totalSize,
		FreeSpace: diskUsage.Free,
		UsedSpace: diskUsage.Used,
	}, nil
}

// GetTree 获取目录树.
func (b *Browser) GetTree(path string, opts TreeOptions) (*FileNode, error) {
	cleanPath, err := b.validatePath(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("路径不存在: %s", path)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("路径不是目录: %s", path)
	}

	node, err := b.buildTree(cleanPath, opts, 0)
	if err != nil {
		return nil, err
	}

	return node, nil
}

// buildTree 递归构建目录树.
func (b *Browser) buildTree(path string, opts TreeOptions, depth int) (*FileNode, error) {
	if depth > opts.MaxDepth {
		return nil, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	node := &FileNode{
		Name:    info.Name(),
		Path:    path,
		Type:    FileTypeDirectory,
		ModTime: info.ModTime(),
		Mode:    info.Mode().String(),
	}

	if opts.IncludeSize {
		node.Size = b.calculateDirSize(path, opts.MaxDepth-depth)
	}

	// 读取子目录
	entries, err := os.ReadDir(path)
	if err != nil {
		return node, nil // 权限问题，返回当前节点
	}

	children := make([]*FileNode, 0)
	for _, entry := range entries {
		if !opts.ShowHidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		entryPath := filepath.Join(path, entry.Name())

		if entry.IsDir() {
			child, err := b.buildTree(entryPath, opts, depth+1)
			if err != nil {
				continue
			}
			if child != nil {
				children = append(children, child)
			}
		} else {
			// 非目录只在第一层添加
			if depth < opts.MaxDepth {
				childInfo, err := entry.Info()
				if err != nil {
					continue
				}
				children = append(children, &FileNode{
					Name:    entry.Name(),
					Path:    entryPath,
					Type:    FileTypeFile,
					Size:    childInfo.Size(),
					ModTime: childInfo.ModTime(),
					Mode:    childInfo.Mode().String(),
				})
			}
		}
	}

	// 按名称排序
	sort.Slice(children, func(i, j int) bool {
		if children[i].Type != children[j].Type {
			return children[i].Type == FileTypeDirectory
		}
		return strings.ToLower(children[i].Name) < strings.ToLower(children[j].Name)
	})

	node.Children = children
	node.ChildrenCount = len(children)

	return node, nil
}

// GetFileNode 获取单个文件节点信息.
func (b *Browser) GetFileNode(path string) (*FileNode, error) {
	cleanPath, err := b.validatePath(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Lstat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	node := &FileNode{
		Name:     info.Name(),
		Path:     cleanPath,
		Size:     info.Size(),
		ModTime:  info.ModTime(),
		Mode:     info.Mode().String(),
		IsHidden: strings.HasPrefix(info.Name(), "."),
	}

	// 判断文件类型
	if info.IsDir() {
		node.Type = FileTypeDirectory
		// 计算子项数量
		entries, err := os.ReadDir(cleanPath)
		if err == nil {
			node.ChildrenCount = len(entries)
		}
	} else if info.Mode()&os.ModeSymlink != 0 {
		node.Type = FileTypeSymlink
		target, err := os.Readlink(cleanPath)
		if err == nil {
			node.SymlinkTarget = target
		}
	} else {
		node.Type = FileTypeFile
		node.Extension = strings.TrimPrefix(filepath.Ext(cleanPath), ".")
		node.MIMEType = getMIMEType(cleanPath)
	}

	return node, nil
}

// GetFileAttributes 获取文件详细属性.
func (b *Browser) GetFileAttributes(path string) (*FileAttributes, error) {
	cleanPath, err := b.validatePath(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Lstat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	attrs := &FileAttributes{
		Path:      cleanPath,
		Name:      info.Name(),
		Size:      info.Size(),
		Mode:      info.Mode().String(),
		ModeOctal: fmt.Sprintf("%04o", info.Mode().Perm()),
		MIMEType:  getMIMEType(cleanPath),
		ModTime:   info.ModTime(),
		IsHidden:  strings.HasPrefix(info.Name(), "."),
	}

	// 判断文件类型
	if info.IsDir() {
		attrs.Type = FileTypeDirectory
	} else if info.Mode()&os.ModeSymlink != 0 {
		attrs.Type = FileTypeSymlink
		attrs.IsSymlink = true
		target, err := os.Readlink(cleanPath)
		if err == nil {
			attrs.SymlinkTarget = target
		}
	} else {
		attrs.Type = FileTypeFile
		attrs.Extension = strings.TrimPrefix(filepath.Ext(cleanPath), ".")
	}

	// 获取系统级文件信息
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		attrs.Inode = stat.Ino
		attrs.Links = uint64(stat.Nlink)
		attrs.UID = int(stat.Uid)
		attrs.GID = int(stat.Gid)
		attrs.AccessTime = time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
		attrs.CreateTime = time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)

		// 尝试获取用户名和组名
		attrs.Owner = getUserName(stat.Uid)
		attrs.Group = getGroupName(stat.Gid)
	}

	return attrs, nil
}

// validatePath 验证并清理路径.
func (b *Browser) validatePath(path string) (string, error) {
	if path == "" {
		path = b.rootPath
	}

	// 转换为绝对路径
	if !filepath.IsAbs(path) {
		path = filepath.Join(b.rootPath, path)
	}

	// 清理路径
	cleanPath := filepath.Clean(path)

	// 安全检查：防止路径遍历攻击
	if !strings.HasPrefix(cleanPath, b.rootPath) {
		return "", fmt.Errorf("路径超出根目录范围: %s", path)
	}

	return cleanPath, nil
}

// buildFileNode 构建文件节点.
func (b *Browser) buildFileNode(parentPath string, entry os.DirEntry) (*FileNode, error) {
	fullPath := filepath.Join(parentPath, entry.Name())
	info, err := entry.Info()
	if err != nil {
		return nil, err
	}

	node := &FileNode{
		Name:      entry.Name(),
		Path:      fullPath,
		Size:      info.Size(),
		ModTime:   info.ModTime(),
		Mode:      info.Mode().String(),
		IsHidden:  strings.HasPrefix(entry.Name(), "."),
		Extension: strings.TrimPrefix(filepath.Ext(entry.Name()), "."),
	}

	// 判断文件类型
	if entry.IsDir() {
		node.Type = FileTypeDirectory
		// 统计子项
		subEntries, err := os.ReadDir(fullPath)
		if err == nil {
			node.ChildrenCount = len(subEntries)
		}
	} else if info.Mode()&os.ModeSymlink != 0 {
		node.Type = FileTypeSymlink
		target, err := os.Readlink(fullPath)
		if err == nil {
			node.SymlinkTarget = target
		}
	} else {
		node.Type = FileTypeFile
		node.MIMEType = getMIMEType(fullPath)
	}

	return node, nil
}

// calculateDirSize 计算目录大小.
func (b *Browser) calculateDirSize(path string, maxDepth int) int64 {
	if maxDepth <= 0 {
		return 0
	}

	var size int64
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0
	}

	for _, entry := range entries {
		if entry.IsDir() {
			size += b.calculateDirSize(filepath.Join(path, entry.Name()), maxDepth-1)
		} else {
			info, err := entry.Info()
			if err == nil {
				size += info.Size()
			}
		}
	}

	return size
}

// getDiskUsage 获取磁盘使用情况.
func (b *Browser) getDiskUsage(path string) (*DiskUsage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil, err
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	used := total - free

	var usedPercent float64
	if total > 0 {
		usedPercent = float64(used) / float64(total) * 100
	}

	return &DiskUsage{
		Path:        path,
		Total:       int64(total),
		Free:        int64(free),
		Used:        int64(used),
		UsedPercent: usedPercent,
	}, nil
}

// getMIMEType 根据扩展名获取MIME类型.
func getMIMEType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	mimeTypes := map[string]string{
		// 图片
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".bmp":  "image/bmp",
		".webp": "image/webp",
		".svg":  "image/svg+xml",
		".ico":  "image/x-icon",

		// 视频
		".mp4":  "video/mp4",
		".avi":  "video/x-msvideo",
		".mkv":  "video/x-matroska",
		".mov":  "video/quicktime",
		".wmv":  "video/x-ms-wmv",
		".flv":  "video/x-flv",
		".webm": "video/webm",
		".m4v":  "video/mp4",

		// 音频
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".flac": "audio/flac",
		".aac":  "audio/aac",
		".ogg":  "audio/ogg",
		".wma":  "audio/x-ms-wma",
		".m4a":  "audio/mp4",

		// 文档
		".pdf":  "application/pdf",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".ppt":  "application/vnd.ms-powerpoint",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".odt":  "application/vnd.oasis.opendocument.text",
		".ods":  "application/vnd.oasis.opendocument.spreadsheet",

		// 压缩
		".zip": "application/zip",
		".tar": "application/x-tar",
		".gz":  "application/gzip",
		".bz2": "application/x-bzip2",
		".7z":  "application/x-7z-compressed",
		".rar": "application/x-rar-compressed",

		// 代码/文本
		".txt":  "text/plain",
		".html": "text/html",
		".htm":  "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".json": "application/json",
		".xml":  "application/xml",
		".yaml": "text/yaml",
		".yml":  "text/yaml",
		".md":   "text/markdown",
		".csv":  "text/csv",
		".go":   "text/x-go",
		".py":   "text/x-python",
		".java": "text/x-java",
		".c":    "text/x-c",
		".cpp":  "text/x-c++",
		".h":    "text/x-c",
		".rs":   "text/x-rust",
		".sh":   "application/x-shellscript",
		".bash": "application/x-shellscript",

		// 其他
		".iso": "application/x-iso9660-image",
		".bin": "application/octet-stream",
		".exe": "application/x-msdownload",
	}

	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// getUserName 根据UID获取用户名.
func getUserName(uid uint32) string {
	// 简化实现，返回UID字符串
	return fmt.Sprintf("%d", uid)
}

// getGroupName 根据GID获取组名.
func getGroupName(gid uint32) string {
	// 简化实现，返回GID字符串
	return fmt.Sprintf("%d", gid)
}
