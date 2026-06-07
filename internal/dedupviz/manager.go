// Package dedupviz 提供文件去重可视化核心逻辑
package dedupviz

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 去重可视化管理器
type Manager struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	config      *DedupvizConfig
	scanResults map[string]*ScanResult
	lastScan    *ScanResult
	stopChan    chan struct{}
}

// NewManager 创建去重可视化管理器
func NewManager(logger *zap.Logger, config *DedupvizConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultDedupvizConfig()
	}
	return &Manager{
		logger:      logger,
		config:      config,
		scanResults: make(map[string]*ScanResult),
		stopChan:    make(chan struct{}),
	}
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ScanDirectory 扫描目录查找重复文件
func (m *Manager) ScanDirectory(paths []string, scanConfig *ScanConfig) (*ScanResult, error) {
	if scanConfig == nil {
		scanConfig = DefaultScanConfig()
	}

	scanID := generateID()
	result := &ScanResult{
		ScanID:      scanID,
		Status:      ScanStatusScanning,
		TargetPaths: paths,
		StartedAt:   time.Now(),
		Groups:      make([]DuplicateGroup, 0),
	}

	m.mu.Lock()
	m.scanResults[scanID] = result
	m.lastScan = result
	m.mu.Unlock()

	// 启动扫描（异步）
	go m.executeScan(result, scanConfig)

	return result, nil
}

// executeScan 执行扫描
func (m *Manager) executeScan(result *ScanResult, config *ScanConfig) {
	// 用于收集文件 hash
	hashMap := make(map[string][]DuplicateFile)
	var mu sync.Mutex
	var totalFiles int64
	var totalSize int64

	// 遍历所有路径
	for _, rootPath := range result.TargetPaths {
		err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // 跳过错误
			}

			// 跳过目录
			if info.IsDir() {
				// 检查排除路径
				for _, exclude := range config.ExcludePaths {
					if strings.HasPrefix(path, exclude) {
						return filepath.SkipDir
					}
				}
				return nil
			}

			// 检查文件大小
			if info.Size() < config.MinFileSize || (config.MaxFileSize > 0 && info.Size() > config.MaxFileSize) {
				return nil
			}

			// 检查排除模式
			for _, pattern := range config.ExcludePatterns {
				matched, _ := filepath.Match(pattern, filepath.Base(path))
				if matched {
					return nil
				}
			}

			// 计算文件 hash
			hash, err := m.computeFileHash(path)
			if err != nil {
				return nil
			}

			mu.Lock()
			hashMap[hash] = append(hashMap[hash], DuplicateFile{
				Path:       path,
				Size:       info.Size(),
				Hash:       hash,
				ModifiedAt: info.ModTime(),
			})
			totalFiles++
			totalSize += info.Size()
			mu.Unlock()

			// 更新进度
			result.Progress = float64(totalFiles) / float64(totalFiles+100) * 100

			return nil
		})

		if err != nil {
			m.logger.Warn("walk error", zap.String("path", rootPath), zap.Error(err))
		}
	}

	// 分析重复
	result.Status = ScanStatusAnalyzing
	result.TotalFiles = int(totalFiles)
	result.TotalSize = totalSize

	// 处理重复组
	for hash, files := range hashMap {
		if len(files) < 2 {
			continue
		}

		// 选择保留的文件
		keepPath, reason := m.selectKeepFile(files)
		markOriginal(files, keepPath)

		fileType := classifyFileType(files[0].Path)
		fileSize := files[0].Size
		wastedSize := fileSize * int64(len(files)-1)

		group := DuplicateGroup{
			Hash:       hash,
			FileType:   fileType,
			FileCount:  len(files),
			FileSize:   fileSize,
			TotalSize:  fileSize * int64(len(files)),
			WastedSize: wastedSize,
			Files:      files,
			KeepPath:   keepPath,
			KeepReason: reason,
		}

		result.Groups = append(result.Groups, group)
		result.DuplicateFiles += len(files) - 1
		result.WastedSize += wastedSize
	}

	// 按浪费空间排序
	sort.Slice(result.Groups, func(i, j int) bool {
		return result.Groups[i].WastedSize > result.Groups[j].WastedSize
	})

	result.DuplicateGroups = len(result.Groups)
	now := time.Now()
	result.CompletedAt = &now
	result.Duration = now.Sub(result.StartedAt)
	result.Status = ScanStatusCompleted
	result.Progress = 100

	m.logger.Info("scan completed",
		zap.String("scan_id", result.ScanID),
		zap.Int("total_files", result.TotalFiles),
		zap.Int("duplicate_groups", result.DuplicateGroups),
		zap.Int64("wasted_size", result.WastedSize),
	)
}

// computeFileHash 计算文件 SHA256 hash
func (m *Manager) computeFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// selectKeepFile 选择保留的文件
func (m *Manager) selectKeepFile(files []DuplicateFile) (string, string) {
	if len(files) == 0 {
		return "", ""
	}

	// 策略：保留路径最短的（通常是原始位置）
	// 如果路径长度相同，保留最新的
	keep := files[0]
	reason := "最早添加"

	for _, f := range files[1:] {
		// 优先选择路径更短的
		if len(f.Path) < len(keep.Path) {
			keep = f
			reason = "路径更短（可能是原始位置）"
		} else if len(f.Path) == len(keep.Path) && f.ModifiedAt.After(keep.ModifiedAt) {
			keep = f
			reason = "最新修改"
		}
	}

	return keep.Path, reason
}

// markOriginal 标记原始文件
func markOriginal(files []DuplicateFile, keepPath string) {
	for i := range files {
		files[i].IsOriginal = files[i].Path == keepPath
	}
}

// classifyFileType 根据扩展名分类文件类型
func classifyFileType(path string) FileType {
	ext := strings.ToLower(filepath.Ext(path))

	for fileType, extensions := range fileTypeExtensions {
		for _, e := range extensions {
			if ext == e {
				return fileType
			}
		}
	}

	return FileTypeOther
}

// GetScanResult 获取扫描结果
func (m *Manager) GetScanResult(scanID string) (*ScanResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, ok := m.scanResults[scanID]
	if !ok {
		return nil, fmt.Errorf("scan result %s not found", scanID)
	}
	return result, nil
}

// GetLastScanResult 获取最近一次扫描结果
func (m *Manager) GetLastScanResult() *ScanResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastScan
}

// ListScanResults 列出扫描结果
func (m *Manager) ListScanResults() []*ScanResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]*ScanResult, 0, len(m.scanResults))
	for _, r := range m.scanResults {
		results = append(results, r)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].StartedAt.After(results[j].StartedAt)
	})

	return results
}

// GetVisualizationData 获取可视化数据
func (m *Manager) GetVisualizationData(scanID string) (*VisualizationData, error) {
	m.mu.RLock()
	result, ok := m.scanResults[scanID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("scan result %s not found", scanID)
	}

	if result.Status != ScanStatusCompleted {
		return nil, fmt.Errorf("scan not completed, current status: %s", result.Status)
	}

	return m.buildVisualizationData(result), nil
}

// GetLastVisualizationData 获取最近一次扫描的可视化数据
func (m *Manager) GetLastVisualizationData() (*VisualizationData, error) {
	m.mu.RLock()
	result := m.lastScan
	m.mu.RUnlock()

	if result == nil {
		return nil, fmt.Errorf("no scan results available")
	}

	if result.Status != ScanStatusCompleted {
		return nil, fmt.Errorf("scan not completed, current status: %s", result.Status)
	}

	return m.buildVisualizationData(result), nil
}

// buildVisualizationData 构建可视化数据
func (m *Manager) buildVisualizationData(result *ScanResult) *VisualizationData {
	data := &VisualizationData{
		BySize:        make([]SizeDistribution, 0),
		ByType:        make([]TypeDistribution, 0),
		ByDirectory:   make([]DirDistribution, 0),
		TopDuplicates: make([]DuplicateGroup, 0),
	}

	// 概览数据
	uniqueFiles := result.TotalFiles - result.DuplicateFiles
	uniqueSize := result.TotalSize - result.WastedSize
	avgDupSize := int64(0)
	if result.DuplicateGroups > 0 {
		avgDupSize = result.WastedSize / int64(result.DuplicateGroups)
	}

	data.Overview = OverviewData{
		TotalFiles:       result.TotalFiles,
		TotalSize:        result.TotalSize,
		DuplicateFiles:   result.DuplicateFiles,
		DuplicateSize:    result.WastedSize,
		WastedSize:       result.WastedSize,
		DedupRatio:       float64(result.DuplicateFiles) / float64(result.TotalFiles) * 100,
		UniqueFiles:      uniqueFiles,
		UniqueSize:       uniqueSize,
		AvgDuplicateSize: avgDupSize,
	}

	// 按大小分布
	sizeRanges := []struct {
		label string
		min   int64
		max   int64
	}{
		{"0-1KB", 0, 1024},
		{"1KB-1MB", 1024, 1048576},
		{"1MB-10MB", 1048576, 10485760},
		{"10MB-100MB", 10485760, 104857600},
		{"100MB-1GB", 104857600, 1073741824},
		{"1GB+", 1073741824, 0},
	}

	for _, sr := range sizeRanges {
		dist := SizeDistribution{
			RangeLabel: sr.label,
			MinSize:    sr.min,
			MaxSize:    sr.max,
		}
		for _, group := range result.Groups {
			if group.FileSize >= sr.min && (sr.max == 0 || group.FileSize < sr.max) {
				dist.Count += group.FileCount
				dist.TotalSize += group.TotalSize
				dist.WastedSize += group.WastedSize
			}
		}
		if result.TotalSize > 0 {
			dist.Percentage = float64(dist.WastedSize) / float64(result.WastedSize) * 100
		}
		data.BySize = append(data.BySize, dist)
	}

	// 按类型分布
	typeMap := make(map[FileType]*TypeDistribution)
	for _, group := range result.Groups {
		dist, ok := typeMap[group.FileType]
		if !ok {
			dist = &TypeDistribution{FileType: group.FileType}
			typeMap[group.FileType] = dist
		}
		dist.Count += group.FileCount
		dist.TotalSize += group.TotalSize
		dist.WastedSize += group.WastedSize
	}

	for _, dist := range typeMap {
		if result.WastedSize > 0 {
			dist.Percentage = float64(dist.WastedSize) / float64(result.WastedSize) * 100
		}
		data.ByType = append(data.ByType, *dist)
	}

	// 按浪费空间排序
	sort.Slice(data.ByType, func(i, j int) bool {
		return data.ByType[i].WastedSize > data.ByType[j].WastedSize
	})

	// 按目录分布
	dirMap := make(map[string]*DirDistribution)
	for _, group := range result.Groups {
		for _, file := range group.Files {
			dir := filepath.Dir(file.Path)
			// 只取前两级目录
			parts := strings.Split(dir, string(filepath.Separator))
			if len(parts) > 3 {
				dir = strings.Join(parts[:3], string(filepath.Separator))
			}

			dist, ok := dirMap[dir]
			if !ok {
				dist = &DirDistribution{Directory: dir}
				dirMap[dir] = dist
			}
			dist.FileCount++
			if !file.IsOriginal {
				dist.DupCount++
				dist.WastedSize += file.Size
			}
			dist.TotalSize += file.Size
		}
	}

	for _, dist := range dirMap {
		if result.WastedSize > 0 {
			dist.Percentage = float64(dist.WastedSize) / float64(result.WastedSize) * 100
		}
		data.ByDirectory = append(data.ByDirectory, *dist)
	}

	// 按浪费空间排序
	sort.Slice(data.ByDirectory, func(i, j int) bool {
		return data.ByDirectory[i].WastedSize > data.ByDirectory[j].WastedSize
	})

	// Top 重复文件组（最多 20 个）
	topCount := 20
	if len(result.Groups) < topCount {
		topCount = len(result.Groups)
	}
	data.TopDuplicates = result.Groups[:topCount]

	return data
}

// DeleteDuplicates 删除重复文件
func (m *Manager) DeleteDuplicates(req *DeleteRequest) (*DeleteResult, error) {
	m.mu.RLock()
	scan := m.lastScan
	m.mu.RUnlock()

	if scan == nil || scan.Status != ScanStatusCompleted {
		return nil, fmt.Errorf("no completed scan available")
	}

	// 查找重复组
	var group *DuplicateGroup
	for _, g := range scan.Groups {
		if g.Hash == req.GroupHash {
			group = &g
			break
		}
	}

	if group == nil {
		return nil, fmt.Errorf("duplicate group with hash %s not found", req.GroupHash)
	}

	result := &DeleteResult{
		DeletedFiles: make([]string, 0),
		FailedFiles:  make([]FailedFile, 0),
		DryRun:       req.DryRun,
	}

	for _, file := range group.Files {
		// 跳过要保留的文件
		if file.Path == req.KeepPath {
			continue
		}

		if req.DryRun {
			result.DeletedFiles = append(result.DeletedFiles, file.Path)
			result.FreedSpace += file.Size
			continue
		}

		// 执行删除
		if err := os.Remove(file.Path); err != nil {
			result.FailedFiles = append(result.FailedFiles, FailedFile{
				Path:  file.Path,
				Error: err.Error(),
			})
			continue
		}

		result.DeletedFiles = append(result.DeletedFiles, file.Path)
		result.FreedSpace += file.Size
	}

	m.logger.Info("delete duplicates completed",
		zap.String("group_hash", req.GroupHash),
		zap.Int("deleted", len(result.DeletedFiles)),
		zap.Int("failed", len(result.FailedFiles)),
		zap.Int64("freed", result.FreedSpace),
		zap.Bool("dry_run", req.DryRun),
	)

	return result, nil
}

// BatchDeleteDuplicates 批量删除重复文件
func (m *Manager) BatchDeleteDuplicates(req *BatchDeleteRequest) (*DeleteResult, error) {
	m.mu.RLock()
	scan := m.lastScan
	m.mu.RUnlock()

	if scan == nil || scan.Status != ScanStatusCompleted {
		return nil, fmt.Errorf("no completed scan available")
	}

	result := &DeleteResult{
		DeletedFiles: make([]string, 0),
		FailedFiles:  make([]FailedFile, 0),
		DryRun:       req.DryRun,
	}

	for _, group := range scan.Groups {
		// 过滤条件
		if req.FileType != "" && group.FileType != req.FileType {
			continue
		}
		if req.MinSize > 0 && group.FileSize < req.MinSize {
			continue
		}
		if req.MaxSize > 0 && group.FileSize > req.MaxSize {
			continue
		}

		// 确定保留文件
		keepPath := group.KeepPath
		if req.KeepPolicy != "" {
			keepPath = m.selectKeepFileByPolicy(group.Files, req.KeepPolicy)
		}

		// 删除其他文件
		for _, file := range group.Files {
			if file.Path == keepPath {
				continue
			}

			if req.DryRun {
				result.DeletedFiles = append(result.DeletedFiles, file.Path)
				result.FreedSpace += file.Size
				continue
			}

			if err := os.Remove(file.Path); err != nil {
				result.FailedFiles = append(result.FailedFiles, FailedFile{
					Path:  file.Path,
					Error: err.Error(),
				})
				continue
			}

			result.DeletedFiles = append(result.DeletedFiles, file.Path)
			result.FreedSpace += file.Size
		}
	}

	return result, nil
}

// selectKeepFileByPolicy 按策略选择保留文件
func (m *Manager) selectKeepFileByPolicy(files []DuplicateFile, policy string) string {
	if len(files) == 0 {
		return ""
	}

	switch policy {
	case "newest":
		keep := files[0]
		for _, f := range files[1:] {
			if f.ModifiedAt.After(keep.ModifiedAt) {
				keep = f
			}
		}
		return keep.Path

	case "oldest":
		keep := files[0]
		for _, f := range files[1:] {
			if f.ModifiedAt.Before(keep.ModifiedAt) {
				keep = f
			}
		}
		return keep.Path

	case "shortest_path":
		keep := files[0]
		for _, f := range files[1:] {
			if len(f.Path) < len(keep.Path) {
				keep = f
			}
		}
		return keep.Path

	default:
		return files[0].Path
	}
}

// GetDuplicatesByType 按类型获取重复文件
func (m *Manager) GetDuplicatesByType(fileType FileType) ([]DuplicateGroup, error) {
	m.mu.RLock()
	scan := m.lastScan
	m.mu.RUnlock()

	if scan == nil || scan.Status != ScanStatusCompleted {
		return nil, fmt.Errorf("no completed scan available")
	}

	groups := make([]DuplicateGroup, 0)
	for _, group := range scan.Groups {
		if group.FileType == fileType {
			groups = append(groups, group)
		}
	}

	return groups, nil
}

// GetDuplicatesBySizeRange 按大小范围获取重复文件
func (m *Manager) GetDuplicatesBySizeRange(minSize, maxSize int64) ([]DuplicateGroup, error) {
	m.mu.RLock()
	scan := m.lastScan
	m.mu.RUnlock()

	if scan == nil || scan.Status != ScanStatusCompleted {
		return nil, fmt.Errorf("no completed scan available")
	}

	groups := make([]DuplicateGroup, 0)
	for _, group := range scan.Groups {
		if group.FileSize >= minSize && (maxSize <= 0 || group.FileSize <= maxSize) {
			groups = append(groups, group)
		}
	}

	return groups, nil
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *DedupvizConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *DedupvizConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}

// ExportScanResult 导出扫描结果为 JSON
func (m *Manager) ExportScanResult(scanID string) ([]byte, error) {
	m.mu.RLock()
	result, ok := m.scanResults[scanID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("scan result %s not found", scanID)
	}

	return json.MarshalIndent(result, "", "  ")
}
