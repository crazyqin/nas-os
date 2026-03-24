// Package storage 提供存储空间分析功能
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// SpaceAnalyzer 存储空间分析器
type SpaceAnalyzer struct {
	manager   *Manager
	history   *SpaceHistory
	mu        sync.RWMutex
	dataDir   string // 数据存储目录
	indexPath string // 历史数据索引文件路径
}

// SpaceHistory 空间使用历史记录
type SpaceHistory struct {
	Records []SpaceRecord `json:"records"`
	mu      sync.RWMutex
}

// SpaceRecord 单次空间记录
type SpaceRecord struct {
	Timestamp time.Time       `json:"timestamp"`
	Volumes   []VolumeRecord  `json:"volumes"`
	FileStats FileTypeSummary `json:"fileStats"`
}

// VolumeRecord 卷空间记录
type VolumeRecord struct {
	Name      string `json:"name"`
	Total     uint64 `json:"total"`
	Used      uint64 `json:"used"`
	Free      uint64 `json:"free"`
	UsedRatio float64 `json:"usedRatio"`
}

// FileTypeSummary 文件类型统计摘要
type FileTypeSummary struct {
	ByExtension map[string]FileTypeStat `json:"byExtension"`
	TotalFiles  uint64                  `json:"totalFiles"`
	TotalSize   uint64                  `json:"totalSize"`
}

// FileTypeStat 文件类型统计
type FileTypeStat struct {
	Extension string `json:"extension"`
	Count     uint64 `json:"count"`
	Size      uint64 `json:"size"`
	Percent   float64 `json:"percent"` // 占总大小百分比
}

// LargeFile 大文件信息
type LargeFile struct {
	Path     string    `json:"path"`
	Size     uint64    `json:"size"`
	Modified time.Time `json:"modified"`
	Type     string    `json:"type"` // 文件类型/扩展名
}

// DirectoryInfo 目录信息
type DirectoryInfo struct {
	Path      string `json:"path"`
	Size      uint64 `json:"size"`
	FileCount uint64 `json:"fileCount"`
	DirCount  uint64 `json:"dirCount"`
	Percent   float64 `json:"percent"` // 占总大小百分比
}

// SpaceTrend 空间趋势预测
type SpaceTrend struct {
	VolumeName     string    `json:"volumeName"`
	CurrentUsed    uint64    `json:"currentUsed"`
	CurrentTotal   uint64    `json:"currentTotal"`
	GrowthRate7D   float64   `json:"growthRate7d"`   // 7天增长率（字节/天）
	GrowthRate30D  float64   `json:"growthRate30d"`  // 30天增长率（字节/天）
	DaysToFull7D   int       `json:"daysToFull7d"`   // 按7天趋势预计填满天数
	DaysToFull30D  int       `json:"daysToFull30d"`  // 按30天趋势预计填满天数
	Predicted7D    uint64    `json:"predicted7d"`    // 7天后预计使用量
	Predicted30D   uint64    `json:"predicted30d"`   // 30天后预计使用量
	TrendDirection string    `json:"trendDirection"` // up, down, stable
	UpdatedAt      time.Time `json:"updatedAt"`
}

// AnalyzeResult 分析结果
type AnalyzeResult struct {
	VolumeName         string            `json:"volumeName"`
	AnalyzedAt         time.Time         `json:"analyzedAt"`
	TotalSize          uint64            `json:"totalSize"`
	UsedSize           uint64            `json:"usedSize"`
	FreeSize           uint64            `json:"freeSize"`
	FileTypeDistribution FileTypeDistribution `json:"fileTypeDistribution"`
	LargeFiles         []LargeFile       `json:"largeFiles"`
	DirectoryRanking   []DirectoryInfo   `json:"directoryRanking"`
	Trend              *SpaceTrend       `json:"trend,omitempty"`
	Warnings           []string          `json:"warnings"`
}

// FileTypeDistribution 文件类型分布
type FileTypeDistribution struct {
	Categories map[string]CategoryStat `json:"categories"`
	ByExtension []FileTypeStat          `json:"byExtension"`
	Unknown    FileTypeStat            `json:"unknown"`
}

// CategoryStat 分类统计
type CategoryStat struct {
	Name        string   `json:"name"`
	Extensions []string `json:"extensions"`
	Count       uint64   `json:"count"`
	Size        uint64   `json:"size"`
	Percent     float64  `json:"percent"`
}

// AnalyzeOptions 分析选项
type AnalyzeOptions struct {
	Path            string `json:"path"`            // 分析路径（默认卷挂载点）
	IncludeHidden   bool   `json:"includeHidden"`   // 包含隐藏文件
	LargeFileThreshold uint64 `json:"largeFileThreshold"` // 大文件阈值（默认100MB）
	TopDirCount     int    `json:"topDirCount"`     // 返回前N个目录（默认10）
	TopFileTypes    int    `json:"topFileTypes"`    // 返回前N个文件类型（默认20）
	AnalyzeDepth    int    `json:"analyzeDepth"`    // 分析深度（默认无限制）
	EnableTrend     bool   `json:"enableTrend"`    // 启用趋势预测
}

// DefaultAnalyzeOptions 默认分析选项
var DefaultAnalyzeOptions = AnalyzeOptions{
	IncludeHidden:       false,
	LargeFileThreshold: 100 * 1024 * 1024, // 100MB
	TopDirCount:         10,
	TopFileTypes:        20,
	AnalyzeDepth:        -1, // 无限制
	EnableTrend:         true,
}

// 文件类型分类映射
var fileTypeCategories = map[string][]string{
	"video":    {".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".m2ts", ".ts"},
	"audio":    {".mp3", ".flac", ".wav", ".aac", ".m4a", ".ogg", ".wma", ".ape", ".alac"},
	"image":    {".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tiff", ".raw", ".heic", ".svg"},
	"document": {".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt", ".md", ".rtf", ".odt", ".ods", ".odp"},
	"archive":  {".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz", ".zst"},
	"code":     {".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".h", ".rs", ".rb", ".php", ".sh", ".bat"},
	"data":     {".json", ".xml", ".yaml", ".yml", ".csv", ".sql", ".db", ".sqlite", ".parquet"},
	"disk":     {".iso", ".img", ".vhd", ".vmdk", ".qcow2"},
}

// NewSpaceAnalyzer 创建空间分析器
func NewSpaceAnalyzer(manager *Manager, dataDir string) *SpaceAnalyzer {
	if dataDir == "" {
		dataDir = "/var/lib/nas-os/storage"
	}
	
	sa := &SpaceAnalyzer{
		manager:   manager,
		dataDir:   dataDir,
		indexPath: filepath.Join(dataDir, "space_history.json"),
		history:   &SpaceHistory{Records: []SpaceRecord{}},
	}
	
	// 加载历史数据
	sa.loadHistory()
	
	return sa
}

// loadHistory 加载历史数据
func (sa *SpaceAnalyzer) loadHistory() error {
	data, err := os.ReadFile(sa.indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在是正常的
		}
		return fmt.Errorf("读取历史数据失败: %w", err)
	}
	
	sa.history.mu.Lock()
	defer sa.history.mu.Unlock()
	
	return json.Unmarshal(data, sa.history)
}

// saveHistory 保存历史数据
func (sa *SpaceAnalyzer) saveHistory() error {
	sa.history.mu.RLock()
	data, err := json.MarshalIndent(sa.history, "", "  ")
	sa.history.mu.RUnlock()
	
	if err != nil {
		return fmt.Errorf("序列化历史数据失败: %w", err)
	}
	
	// 确保目录存在
	if err := os.MkdirAll(sa.dataDir, 0755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}
	
	return os.WriteFile(sa.indexPath, data, 0644)
}

// Analyze 执行空间分析
func (sa *SpaceAnalyzer) Analyze(volumeName string, opts AnalyzeOptions) (*AnalyzeResult, error) {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	
	// 获取卷信息
	vol := sa.manager.GetVolume(volumeName)
	if vol == nil {
		return nil, fmt.Errorf("卷不存在: %s", volumeName)
	}
	
	// 确定分析路径
	analyzePath := opts.Path
	if analyzePath == "" {
		analyzePath = vol.MountPoint
	}
	
	// 设置默认选项
	if opts.LargeFileThreshold == 0 {
		opts.LargeFileThreshold = DefaultAnalyzeOptions.LargeFileThreshold
	}
	if opts.TopDirCount == 0 {
		opts.TopDirCount = DefaultAnalyzeOptions.TopDirCount
	}
	if opts.TopFileTypes == 0 {
		opts.TopFileTypes = DefaultAnalyzeOptions.TopFileTypes
	}
	if opts.AnalyzeDepth == 0 {
		opts.AnalyzeDepth = DefaultAnalyzeOptions.AnalyzeDepth
	}
	
	result := &AnalyzeResult{
		VolumeName: volumeName,
		AnalyzedAt: time.Now(),
		TotalSize:  vol.Size,
		UsedSize:   vol.Used,
		FreeSize:   vol.Free,
		Warnings:   []string{},
	}
	
	// 文件类型分布统计
	fileStats, err := sa.analyzeFileTypes(analyzePath, opts)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("文件类型分析部分失败: %v", err))
	} else {
		result.FileTypeDistribution = fileStats
	}
	
	// 大文件检测
	largeFiles, err := sa.findLargeFiles(analyzePath, opts)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("大文件检测部分失败: %v", err))
	} else {
		result.LargeFiles = largeFiles
	}
	
	// 目录空间占用排行
	dirRanking, err := sa.rankDirectories(analyzePath, opts)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("目录排行分析部分失败: %v", err))
	} else {
		result.DirectoryRanking = dirRanking
	}
	
	// 空间趋势预测
	if opts.EnableTrend {
		trend := sa.predictTrend(volumeName, vol)
		result.Trend = trend
	}
	
	// 记录本次分析数据
	sa.recordAnalysis(volumeName, vol, result.FileTypeDistribution)
	
	return result, nil
}

// analyzeFileTypes 分析文件类型分布
func (sa *SpaceAnalyzer) analyzeFileTypes(rootPath string, opts AnalyzeOptions) (FileTypeDistribution, error) {
	distribution := FileTypeDistribution{
		Categories:   make(map[string]CategoryStat),
		ByExtension:  []FileTypeStat{},
		Unknown:      FileTypeStat{Extension: "unknown"},
	}
	
	// 初始化分类
	for cat := range fileTypeCategories {
		distribution.Categories[cat] = CategoryStat{
			Name:        cat,
			Extensions:  fileTypeCategories[cat],
			Count:       0,
			Size:        0,
		}
	}
	
	extStats := make(map[string]FileTypeStat)
	var totalSize uint64
	var totalFiles uint64
	
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过错误文件
		}
		
		// 跳过隐藏文件
		if !opts.IncludeHidden && strings.HasPrefix(filepath.Base(path), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		
		// 只处理文件
		if info.IsDir() {
			return nil
		}
		
		// 检查深度
		if opts.AnalyzeDepth > 0 {
			depth := strings.Count(strings.TrimPrefix(path, rootPath), string(filepath.Separator))
			if depth > opts.AnalyzeDepth {
				return nil
			}
		}
		
		ext := strings.ToLower(filepath.Ext(path))
		size := uint64(info.Size())
		
		totalFiles++
		totalSize += size
		
		// 统计扩展名
		stat := extStats[ext]
		stat.Extension = ext
		stat.Count++
		stat.Size += size
		extStats[ext] = stat
		
		// 分类统计
		categorized := false
		for catName, extensions := range fileTypeCategories {
			for _, catExt := range extensions {
				if ext == catExt {
					cat := distribution.Categories[catName]
					cat.Count++
					cat.Size += size
					distribution.Categories[catName] = cat
					categorized = true
					break
				}
			}
			if categorized {
				break
			}
		}
		
		if !categorized {
			distribution.Unknown.Count++
			distribution.Unknown.Size += size
		}
		
		return nil
	})
	
	if err != nil {
		return distribution, err
	}
	
	// 计算百分比
	for ext, stat := range extStats {
		if totalSize > 0 {
			stat.Percent = float64(stat.Size) / float64(totalSize) * 100
		}
		extStats[ext] = stat
	}
	
	for catName, cat := range distribution.Categories {
		if totalSize > 0 {
			cat.Percent = float64(cat.Size) / float64(totalSize) * 100
		}
		distribution.Categories[catName] = cat
	}
	
	if totalSize > 0 {
		distribution.Unknown.Percent = float64(distribution.Unknown.Size) / float64(totalSize) * 100
	}
	
	// 按大小排序扩展名
	extList := make([]FileTypeStat, 0, len(extStats))
	for _, stat := range extStats {
		extList = append(extList, stat)
	}
	sort.Slice(extList, func(i, j int) bool {
		return extList[i].Size > extList[j].Size
	})
	
	// 限制数量
	if len(extList) > opts.TopFileTypes {
		extList = extList[:opts.TopFileTypes]
	}
	distribution.ByExtension = extList
	
	return distribution, nil
}

// findLargeFiles 查找大文件
func (sa *SpaceAnalyzer) findLargeFiles(rootPath string, opts AnalyzeOptions) ([]LargeFile, error) {
	var largeFiles []LargeFile
	
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		// 跳过隐藏文件
		if !opts.IncludeHidden && strings.HasPrefix(filepath.Base(path), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		
		// 只处理文件
		if info.IsDir() {
			return nil
		}
		
		// 检查是否为大文件
		if uint64(info.Size()) >= opts.LargeFileThreshold {
			largeFiles = append(largeFiles, LargeFile{
				Path:     path,
				Size:     uint64(info.Size()),
				Modified: info.ModTime(),
				Type:     strings.ToLower(filepath.Ext(path)),
			})
		}
		
		return nil
	})
	
	if err != nil {
		return largeFiles, err
	}
	
	// 按大小排序
	sort.Slice(largeFiles, func(i, j int) bool {
		return largeFiles[i].Size > largeFiles[j].Size
	})
	
	// 限制返回数量
	if len(largeFiles) > 100 {
		largeFiles = largeFiles[:100]
	}
	
	return largeFiles, nil
}

// rankDirectories 计算目录空间占用排行
func (sa *SpaceAnalyzer) rankDirectories(rootPath string, opts AnalyzeOptions) ([]DirectoryInfo, error) {
	dirSizes := make(map[string]*DirectoryInfo)
	
	// 初始化根目录
	dirSizes[rootPath] = &DirectoryInfo{
		Path:      rootPath,
		Size:      0,
		FileCount: 0,
		DirCount:  0,
	}
	
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		// 跳过隐藏文件
		if !opts.IncludeHidden && strings.HasPrefix(filepath.Base(path), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		
		// 获取相对路径的所有父目录
		relPath := strings.TrimPrefix(path, rootPath)
		if relPath == "" {
			return nil
		}
		
		parent := rootPath
		parts := strings.Split(strings.Trim(relPath, string(filepath.Separator)), string(filepath.Separator))
		
		for i, part := range parts {
			if part == "" {
				continue
			}
			
			currentPath := filepath.Join(parent, part)
			
			if dirInfo, exists := dirSizes[currentPath]; exists {
				if info.IsDir() {
					dirInfo.DirCount++
				} else {
					dirInfo.FileCount++
					dirInfo.Size += uint64(info.Size())
				}
			} else if i < len(parts)-1 || info.IsDir() {
				// 创建新的目录条目
				dirSizes[currentPath] = &DirectoryInfo{
					Path:      currentPath,
					Size:      0,
					FileCount: 0,
					DirCount:  0,
				}
				if info.IsDir() {
					dirSizes[currentPath].DirCount++
				}
			}
			
			// 同时更新所有父目录的大小
			for parentPath := parent; parentPath != "" && parentPath != rootPath; {
				if p, exists := dirSizes[parentPath]; exists {
					if !info.IsDir() {
						p.Size += uint64(info.Size())
						p.FileCount++
					}
				}
				parentPath = filepath.Dir(parentPath)
				if parentPath == "." || parentPath == "/" {
					break
				}
			}
			
			parent = currentPath
		}
		
		// 更新根目录统计
		if !info.IsDir() {
			dirSizes[rootPath].Size += uint64(info.Size())
			dirSizes[rootPath].FileCount++
		}
		
		return nil
	})
	
	if err != nil {
		return nil, err
	}
	
	// 转换为切片并排序
	var ranking []DirectoryInfo
	totalSize := dirSizes[rootPath].Size
	
	for path, info := range dirSizes {
		if path == rootPath {
			continue // 排除根目录
		}
		
		if totalSize > 0 {
			info.Percent = float64(info.Size) / float64(totalSize) * 100
		}
		ranking = append(ranking, *info)
	}
	
	sort.Slice(ranking, func(i, j int) bool {
		return ranking[i].Size > ranking[j].Size
	})
	
	// 限制返回数量
	if len(ranking) > opts.TopDirCount {
		ranking = ranking[:opts.TopDirCount]
	}
	
	return ranking, nil
}

// recordAnalysis 记录分析结果
func (sa *SpaceAnalyzer) recordAnalysis(volumeName string, vol *Volume, dist FileTypeDistribution) {
	record := SpaceRecord{
		Timestamp: time.Now(),
		Volumes: []VolumeRecord{
			{
				Name:      volumeName,
				Total:     vol.Size,
				Used:      vol.Used,
				Free:      vol.Free,
				UsedRatio: float64(vol.Used) / float64(vol.Size),
			},
		},
	}
	
	// 简化文件统计
	record.FileStats = FileTypeSummary{
		ByExtension: make(map[string]FileTypeStat),
		TotalFiles:  0,
		TotalSize:   vol.Used,
	}
	
	for _, stat := range dist.ByExtension {
		record.FileStats.ByExtension[stat.Extension] = stat
		record.FileStats.TotalFiles += stat.Count
	}
	
	sa.history.mu.Lock()
	sa.history.Records = append(sa.history.Records, record)
	
	// 保留最近90天的记录
	cutoff := time.Now().AddDate(0, 0, -90)
	var filtered []SpaceRecord
	for _, r := range sa.history.Records {
		if r.Timestamp.After(cutoff) {
			filtered = append(filtered, r)
		}
	}
	sa.history.Records = filtered
	sa.history.mu.Unlock()
	
	// 异步保存
	go sa.saveHistory()
}

// predictTrend 预测空间趋势
func (sa *SpaceAnalyzer) predictTrend(volumeName string, vol *Volume) *SpaceTrend {
	trend := &SpaceTrend{
		VolumeName:    volumeName,
		CurrentUsed:   vol.Used,
		CurrentTotal:  vol.Size,
		UpdatedAt:     time.Now(),
	}
	
	sa.history.mu.RLock()
	defer sa.history.mu.RUnlock()
	
	// 过滤出该卷的记录
	var volumeRecords []SpaceRecord
	for _, r := range sa.history.Records {
		for _, v := range r.Volumes {
			if v.Name == volumeName {
				volumeRecords = append(volumeRecords, r)
				break
			}
		}
	}
	
	if len(volumeRecords) < 2 {
		trend.TrendDirection = "stable"
		trend.GrowthRate7D = 0
		trend.GrowthRate30D = 0
		return trend
	}
	
	// 计算增长率
	now := time.Now()
	day7Ago := now.AddDate(0, 0, -7)
	day30Ago := now.AddDate(0, 0, -30)
	
	var records7D, records30D []SpaceRecord
	for _, r := range volumeRecords {
		if r.Timestamp.After(day7Ago) {
			records7D = append(records7D, r)
		}
		if r.Timestamp.After(day30Ago) {
			records30D = append(records30D, r)
		}
	}
	
	// 计算7天增长率
	if len(records7D) >= 2 {
		sort.Slice(records7D, func(i, j int) bool {
			return records7D[i].Timestamp.Before(records7D[j].Timestamp)
		})
		
		oldest := records7D[0]
		newest := records7D[len(records7D)-1]
		
		days := newest.Timestamp.Sub(oldest.Timestamp).Hours() / 24
		if days > 0 {
			sizeDiff := float64(newest.Volumes[0].Used) - float64(oldest.Volumes[0].Used)
			trend.GrowthRate7D = sizeDiff / days
			
			// 预测7天后
			trend.Predicted7D = vol.Used + uint64(trend.GrowthRate7D*7)
			
			// 计算填满天数
			if trend.GrowthRate7D > 0 {
				freeSpace := float64(vol.Free)
				trend.DaysToFull7D = int(freeSpace / trend.GrowthRate7D)
			} else {
				trend.DaysToFull7D = -1 // 永不填满
			}
		}
	}
	
	// 计算30天增长率
	if len(records30D) >= 2 {
		sort.Slice(records30D, func(i, j int) bool {
			return records30D[i].Timestamp.Before(records30D[j].Timestamp)
		})
		
		oldest := records30D[0]
		newest := records30D[len(records30D)-1]
		
		days := newest.Timestamp.Sub(oldest.Timestamp).Hours() / 24
		if days > 0 {
			sizeDiff := float64(newest.Volumes[0].Used) - float64(oldest.Volumes[0].Used)
			trend.GrowthRate30D = sizeDiff / days
			
			// 预测30天后
			trend.Predicted30D = vol.Used + uint64(trend.GrowthRate30D*30)
			
			// 计算填满天数
			if trend.GrowthRate30D > 0 {
				freeSpace := float64(vol.Free)
				trend.DaysToFull30D = int(freeSpace / trend.GrowthRate30D)
			} else {
				trend.DaysToFull30D = -1 // 永不填满
			}
		}
	}
	
	// 判断趋势方向
	avgGrowth := (trend.GrowthRate7D + trend.GrowthRate30D) / 2
	threshold := float64(vol.Size) * 0.001 // 0.1% 作为稳定阈值
	
	if avgGrowth > threshold {
		trend.TrendDirection = "up"
	} else if avgGrowth < -threshold {
		trend.TrendDirection = "down"
	} else {
		trend.TrendDirection = "stable"
	}
	
	return trend
}

// GetHistory 获取历史记录
func (sa *SpaceAnalyzer) GetHistory(volumeName string, days int) ([]SpaceRecord, error) {
	sa.history.mu.RLock()
	defer sa.history.mu.RUnlock()
	
	if days <= 0 {
		days = 30
	}
	
	cutoff := time.Now().AddDate(0, 0, -days)
	var result []SpaceRecord
	
	for _, r := range sa.history.Records {
		if r.Timestamp.After(cutoff) {
			// 过滤指定卷
			for _, v := range r.Volumes {
				if volumeName == "" || v.Name == volumeName {
					result = append(result, r)
					break
				}
			}
		}
	}
	
	return result, nil
}

// FormatBytes 格式化字节大小
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// GetFileTypeCategory 获取文件类型分类
func GetFileTypeCategory(ext string) string {
	ext = strings.ToLower(ext)
	for catName, extensions := range fileTypeCategories {
		for _, catExt := range extensions {
			if ext == catExt {
				return catName
			}
		}
	}
	return "other"
}