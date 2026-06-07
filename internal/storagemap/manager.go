package storagemap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ScanPath 扫描路径
func (m *StorageMapManager) ScanPath(rootPath string) (*StorageTree, error) {
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("cannot access path: %w", err)
	}

	if !info.IsDir() {
		return &StorageTree{
			ID:       generateID(rootPath),
			Path:     rootPath,
			Name:     info.Name(),
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			FileType: getFileType(info.Name()),
		}, nil
	}

	tree := &StorageTree{
		ID:       generateID(rootPath),
		Path:     rootPath,
		Name:     info.Name(),
		ModTime:  info.ModTime(),
		FileType: "directory",
	}

	err = m.scanDirectory(rootPath, tree, 0)
	if err != nil {
		return nil, err
	}

	// 保存树
	m.mu.Lock()
	m.trees[rootPath] = tree
	m.mu.Unlock()

	return tree, nil
}

// scanDirectory 扫描目录
func (m *StorageMapManager) scanDirectory(dirPath string, parent *StorageTree, depth int) error {
	if depth > m.config.MaxDepth {
		return nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		// 检查排除模式
		if m.shouldExclude(entry.Name()) {
			continue
		}

		fullPath := filepath.Join(dirPath, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		node := &StorageTree{
			ID:       generateID(fullPath),
			Path:     fullPath,
			Name:     entry.Name(),
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			Parent:   dirPath,
			FileType: getFileType(entry.Name()),
		}

		if entry.IsDir() {
			node.FileType = "directory"
			parent.DirCount++

			err = m.scanDirectory(fullPath, node, depth+1)
			if err != nil {
				continue
			}
		} else {
			parent.FileCount++
			if info.Size() < m.config.MinFileSize {
				continue
			}
		}

		parent.Size += node.Size
		parent.Children = append(parent.Children, node)
	}

	return nil
}

// shouldExclude 检查是否排除
func (m *StorageMapManager) shouldExclude(name string) bool {
	for _, pattern := range m.config.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
		// 支持通配符
		if strings.Contains(pattern, "*") {
			if matched, _ := filepath.Match(pattern, name); matched {
				return true
			}
		}
	}
	return false
}

// getFileType 获取文件类型
func getFileType(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg":
		return "image"
	case ".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm":
		return "video"
	case ".mp3", ".wav", ".flac", ".aac", ".ogg", ".wma":
		return "audio"
	case ".doc", ".docx", ".pdf", ".txt", ".rtf", ".odt":
		return "document"
	case ".xls", ".xlsx", ".csv":
		return "spreadsheet"
	case ".ppt", ".pptx":
		return "presentation"
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2":
		return "archive"
	case ".exe", ".msi", ".dmg", ".deb", ".rpm":
		return "executable"
	case ".iso", ".img":
		return "diskimage"
	case ".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".rs":
		return "code"
	default:
		if ext == "" {
			return "unknown"
		}
		return ext[1:] // 去掉点号
	}
}

// generateID 生成唯一ID
func generateID(path string) string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

// GetUsageSummary 获取使用摘要
func (m *StorageMapManager) GetUsageSummary(path string) (*UsageSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tree, exists := m.trees[path]
	if !exists {
		return nil, fmt.Errorf("storage tree not found: %s", path)
	}

	summary := &UsageSummary{
		Path:          path,
		TotalSize:     tree.Size,
		FileCount:     tree.FileCount,
		DirCount:      tree.DirCount,
		ScanTime:      tree.ModTime,
		TypeBreakdown: make(map[string]TypeStats),
	}

	m.calculateUsageSummary(tree, summary)

	return summary, nil
}

// calculateUsageSummary 计算使用摘要
func (m *StorageMapManager) calculateUsageSummary(tree *StorageTree, summary *UsageSummary) {
	if tree.FileType != "directory" {
		stats, exists := summary.TypeBreakdown[tree.FileType]
		if !exists {
			stats = TypeStats{}
		}
		stats.Count++
		stats.Size += tree.Size
		summary.TypeBreakdown[tree.FileType] = stats
	}

	for _, child := range tree.Children {
		m.calculateUsageSummary(child, summary)
	}
}

// UsageSummary 使用摘要
type UsageSummary struct {
	Path          string               `json:"path"`
	TotalSize     int64                `json:"total_size"`
	FileCount     int64                `json:"file_count"`
	DirCount      int64                `json:"dir_count"`
	ScanTime      time.Time            `json:"scan_time"`
	TypeBreakdown map[string]TypeStats `json:"type_breakdown"`
}

// TypeStats 类型统计
type TypeStats struct {
	Count int64 `json:"count"`
	Size  int64 `json:"size"`
}

// CompareSnapshots 比较快照
func (m *StorageMapManager) CompareSnapshots(path1, path2 string) (*SnapshotDiff, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tree1, exists1 := m.trees[path1]
	tree2, exists2 := m.trees[path2]

	if !exists1 || !exists2 {
		return nil, fmt.Errorf("one or both snapshots not found")
	}

	diff := &SnapshotDiff{
		Path1:    path1,
		Path2:    path2,
		SizeDiff: tree2.Size - tree1.Size,
		Added:    []*StorageTree{},
		Removed:  []*StorageTree{},
		Modified: []*StorageTree{},
	}

	m.compareTrees(tree1, tree2, diff)

	return diff, nil
}

// compareTrees 比较树
func (m *StorageMapManager) compareTrees(tree1, tree2 *StorageTree, diff *SnapshotDiff) {
	// 简化实现：比较子节点
	map1 := make(map[string]*StorageTree)
	map2 := make(map[string]*StorageTree)

	for _, child := range tree1.Children {
		map1[child.Name] = child
	}
	for _, child := range tree2.Children {
		map2[child.Name] = child
	}

	// 找出新增的
	for name, child := range map2 {
		if _, exists := map1[name]; !exists {
			diff.Added = append(diff.Added, child)
		}
	}

	// 找出删除的
	for name, child := range map1 {
		if _, exists := map2[name]; !exists {
			diff.Removed = append(diff.Removed, child)
		}
	}

	// 找出修改的
	for name, child2 := range map2 {
		if child1, exists := map1[name]; exists {
			if child1.Size != child2.Size || child1.ModTime != child2.ModTime {
				diff.Modified = append(diff.Modified, child2)
			}
		}
	}
}

// SnapshotDiff 快照差异
type SnapshotDiff struct {
	Path1    string         `json:"path1"`
	Path2    string         `json:"path2"`
	SizeDiff int64          `json:"size_diff"`
	Added    []*StorageTree `json:"added"`
	Removed  []*StorageTree `json:"removed"`
	Modified []*StorageTree `json:"modified"`
}
