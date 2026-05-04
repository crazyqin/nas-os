package storageanalytics

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Collector 文件系统数据采集器.
type Collector struct {
	config *Config
	logger *zap.Logger
}

// NewCollector 创建数据采集器.
func NewCollector(config *Config, logger *zap.Logger) *Collector {
	if config == nil {
		config = DefaultConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Collector{
		config: config,
		logger: logger,
	}
}

// Collect 采集指定路径的文件系统数据.
func (c *Collector) Collect(rootPath string, maxDepth int, topN int) (*CollectResult, error) {
	if _, err := os.Stat(rootPath); err != nil {
		return nil, err
	}
	if topN <= 0 {
		topN = c.config.DefaultTopN
	}

	result := &CollectResult{
		ScanPath: rootPath,
		ScanTime: time.Now(),
	}

	dirSizes := make(map[string]*DirectoryInfo)
	var mu sync.Mutex

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			c.logger.Warn("访问文件失败，跳过", zap.String("path", path), zap.Error(err))
			return nil
		}

		// 检查深度
		if maxDepth > 0 {
			rel, _ := filepath.Rel(rootPath, path)
			depth := strings.Count(rel, string(os.PathSeparator))
			if depth > maxDepth && info.IsDir() {
				return filepath.SkipDir
			}
		}

		// 跳过符号链接
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		if info.IsDir() {
			mu.Lock()
			dirSizes[path] = &DirectoryInfo{
				Path: path,
			}
			mu.Unlock()
			return nil
		}

		// 大文件跳过检查
		if c.config.MaxFileSizeForAnalysis > 0 && info.Size() > c.config.MaxFileSizeForAnalysis {
			c.logger.Debug("跳过大文件", zap.String("path", path), zap.Int64("size", info.Size()))
			return nil
		}

		fi := FileInfo{
			Path:       path,
			Size:       info.Size(),
			ModTime:    info.ModTime(),
			AccessTime: getAccessTime(info),
			IsDir:      false,
			FileType:   ClassifyFile(path),
		}

		mu.Lock()
		result.Files = append(result.Files, fi)
		result.TotalSize += fi.Size
		result.TotalFiles++

		// 累加目录统计
		dir := filepath.Dir(path)
		if ds, ok := dirSizes[dir]; ok {
			ds.TotalSize += fi.Size
			ds.FileCount++
		}
		mu.Unlock()

		return nil
	})
	if err != nil {
		return nil, err
	}

	// 计算目录总大小（包含子目录）
	calculateDirTotalSizes(dirSizes, rootPath)

	// 收集目录结果
	result.TotalDirs = len(dirSizes)
	for _, ds := range dirSizes {
		if ds.Path != rootPath {
			result.Directories = append(result.Directories, *ds)
		}
	}

	c.logger.Info("采集完成",
		zap.String("path", rootPath),
		zap.Int("files", result.TotalFiles),
		zap.Int("dirs", result.TotalDirs),
		zap.Int64("total_size", result.TotalSize),
	)

	return result, nil
}

// ClassifyFile 根据扩展名分类文件.
func ClassifyFile(path string) FileType {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	// 图片
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".webp", ".ico", ".tiff", ".tif", ".heic", ".heif", ".raw", ".cr2", ".nef":
		return FileTypeImage
	// 视频
	case ".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".mts", ".rmvb", ".rm", ".3gp", ".mpg", ".mpeg":
		return FileTypeVideo
	// 文档
	case ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt", ".md", ".csv", ".rtf", ".odt", ".ods", ".odp", ".pages", ".numbers", ".key":
		return FileTypeDocument
	// 压缩包
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz", ".tgz", ".tar.gz", ".tar.bz2", ".tar.xz", ".zst", ".lz4":
		return FileTypeArchive
	// 代码
	case ".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".h", ".hpp", ".rs", ".rb", ".php", ".sh", ".bash", ".html", ".css", ".scss", ".json", ".yaml", ".yml", ".xml", ".sql", ".swift", ".kt", ".scala", ".lua", ".r", ".m", ".vue", ".jsx", ".tsx":
		return FileTypeCode
	default:
		return FileTypeOther
	}
}

// classifyAccessFrequency 根据最后访问时间判断访问频率.
func classifyAccessFrequency(accessTime time.Time) AccessFrequency {
	if accessTime.IsZero() {
		return AccessNever
	}
	since := time.Since(accessTime)
	switch {
	case since < 7*24*time.Hour:
		return AccessFrequent
	case since < 30*24*time.Hour:
		return AccessOccasional
	case since < 365*24*time.Hour:
		return AccessRare
	default:
		return AccessNever
	}
}

// classifyAgeBracket 根据修改时间判断年龄区间.
func classifyAgeBracket(modTime time.Time) AgeBracket {
	age := time.Since(modTime)
	switch {
	case age < 7*24*time.Hour:
		return AgeLT7Days
	case age < 30*24*time.Hour:
		return Age7To30Days
	case age < 90*24*time.Hour:
		return Age30To90Days
	case age < 365*24*time.Hour:
		return Age90To365
	default:
		return AgeGT1Year
	}
}

// classifySizeBracket 根据文件大小判断区间.
func classifySizeBracket(size int64) SizeBracket {
	switch {
	case size < 1*1024*1024:
		return SizeLT1MB
	case size < 100*1024*1024:
		return Size1MBTo100
	case size < 1024*1024*1024:
		return Size100MBTo1G
	default:
		return SizeGT1GB
	}
}

// calculateDirTotalSizes 计算每个目录的总大小（含子目录）.
func calculateDirTotalSizes(dirSizes map[string]*DirectoryInfo, rootPath string) {
	// 按路径深度降序排列（叶子节点优先）
	dirs := make([]string, 0, len(dirSizes))
	for p := range dirSizes {
		dirs = append(dirs, p)
	}

	// 简单的从深到浅排序
	for i := 0; i < len(dirs)-1; i++ {
		for j := i + 1; j < len(dirs); j++ {
			if len(dirs[j]) > len(dirs[i]) {
				dirs[i], dirs[j] = dirs[j], dirs[i]
			}
		}
	}

	for _, dir := range dirs {
		if dir == rootPath {
			continue
		}
		parent := filepath.Dir(dir)
		if ds, ok := dirSizes[parent]; ok {
			ds.TotalSize += dirSizes[dir].TotalSize
			ds.DirCount++
		}
	}
}

// getAccessTime 获取文件访问时间（跨平台兼容）.
func getAccessTime(info os.FileInfo) time.Time {
	// Go 标准库 Stat 返回的 FileInfo 在 Linux 上包含 Atime
	// 使用 syscall 方式获取
	stat := getStat(info)
	if stat != nil {
		return stat.Atime
	}
	// 回退：使用修改时间作为近似
	return info.ModTime()
}
