package storage_efficiency

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Analyzer 存储效率分析引擎.
type Analyzer struct {
	mu          sync.RWMutex
	dataDir     string                  // 数据存储目录
	history     []internalRecord        // 历史记录
	tasks       map[string]*AnalyzeResult // 运行中的分析任务
	sampleRate  int                     // 默认采样率
	maxFileSize int64                   // 单文件最大采样大小（字节）
}

// NewAnalyzer 创建存储效率分析器.
func NewAnalyzer(dataDir string) *Analyzer {
	if dataDir == "" {
		dataDir = "/var/lib/nas-os/efficiency"
	}

	a := &Analyzer{
		dataDir:     dataDir,
		history:     []internalRecord{},
		tasks:       make(map[string]*AnalyzeResult),
		sampleRate:  10,
		maxFileSize: 100 * 1024 * 1024, // 100MB
	}

	_ = a.loadHistory()
	return a
}

// loadHistory 从磁盘加载历史记录.
func (a *Analyzer) loadHistory() error {
	path := filepath.Join(a.dataDir, "efficiency_history.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取效率历史数据失败: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	return json.Unmarshal(data, &a.history)
}

// saveHistory 将历史记录持久化到磁盘.
func (a *Analyzer) saveHistory() error {
	a.mu.RLock()
	data, err := json.MarshalIndent(a.history, "", "  ")
	a.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("序列化效率历史数据失败: %w", err)
	}

	if err := os.MkdirAll(a.dataDir, 0755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	path := filepath.Join(a.dataDir, "efficiency_history.json")
	return os.WriteFile(path, data, 0644)
}

// Analyze 执行存储效率分析.
func (a *Analyzer) Analyze(path string, sampleRate int, deepScan bool) (*EfficiencySummary, error) {
	if path == "" {
		path = "/"
	}
	if sampleRate <= 0 || sampleRate > 100 {
		sampleRate = a.sampleRate
	}

	files, err := a.collectFiles(path, sampleRate, deepScan)
	if err != nil {
		return nil, fmt.Errorf("收集文件信息失败: %w", err)
	}

	compStats := a.analyzeCompression(files)
	dedupStats := a.analyzeDedup(files)
	summary := a.buildSummary(compStats, dedupStats)
	a.recordHistory(summary)

	return summary, nil
}

// AnalyzeAsync 异步执行分析，返回任务ID.
func (a *Analyzer) AnalyzeAsync(path string, sampleRate int, deepScan bool) *AnalyzeResult {
	taskID := uuid.New().String()
	if path == "" {
		path = "/"
	}

	result := &AnalyzeResult{
		TaskID:    taskID,
		Status:    StatusRunning,
		StartedAt: time.Now(),
		Path:      path,
		Message:   "分析任务已启动",
	}

	a.mu.Lock()
	a.tasks[taskID] = result
	a.mu.Unlock()

	go func() {
		_, err := a.Analyze(path, sampleRate, deepScan)
		a.mu.Lock()
		if err != nil {
			result.Status = StatusFailed
			result.Message = fmt.Sprintf("分析失败: %v", err)
		} else {
			result.Status = StatusCompleted
			result.Message = "分析完成"
		}
		a.mu.Unlock()
	}()

	return result
}

// GetCompressionStats 获取压缩统计详情.
func (a *Analyzer) GetCompressionStats(path string) (*CompressionStats, error) {
	if path == "" {
		path = "/"
	}

	files, err := a.collectFiles(path, a.sampleRate, false)
	if err != nil {
		return nil, fmt.Errorf("收集文件信息失败: %w", err)
	}

	return a.analyzeCompression(files), nil
}

// GetDedupStats 获取去重统计详情.
func (a *Analyzer) GetDedupStats(path string) (*DedupStats, error) {
	if path == "" {
		path = "/"
	}

	files, err := a.collectFiles(path, a.sampleRate, true)
	if err != nil {
		return nil, fmt.Errorf("收集文件信息失败: %w", err)
	}

	return a.analyzeDedup(files), nil
}

// GetTrends 获取历史趋势数据.
func (a *Analyzer) GetTrends(days int) *TrendData {
	if days <= 0 {
		days = 30
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	cutoff := time.Now().AddDate(0, 0, -days)
	var points []TrendPoint

	for _, r := range a.history {
		if r.Timestamp.After(cutoff) {
			points = append(points, TrendPoint{
				Date:             r.Timestamp.Format("2006-01-02"),
				LogicalSize:      r.LogicalSize,
				PhysicalSize:     r.PhysicalSize,
				SpaceSaved:       r.SpaceSaved,
				CompressionRatio: r.CompressionRatio,
				DedupRatio:       r.DedupRatio,
			})
		}
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].Date < points[j].Date
	})

	return &TrendData{
		Days:   days,
		Points: points,
	}
}

// GetTask 获取分析任务状态.
func (a *Analyzer) GetTask(taskID string) *AnalyzeResult {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.tasks[taskID]
}

// ========== 新增：去重检测和清理建议 ==========

// DetectDuplicates 检测重复文件并生成清理建议.
func (a *Analyzer) DetectDuplicates(path string) (*DuplicateDetectionResult, error) {
	if path == "" {
		path = "/"
	}

	startTime := time.Now()

	// 收集所有文件（100% 采样，深度扫描）
	files, err := a.collectFiles(path, 100, true)
	if err != nil {
		return nil, fmt.Errorf("收集文件信息失败: %w", err)
	}

	// 按哈希分组
	hashGroups := make(map[string][]fileInfo)
	for _, f := range files {
		if f.hash != "" {
			hashGroups[f.hash] = append(hashGroups[f.hash], f)
		}
	}

	result := &DuplicateDetectionResult{
		TotalFiles:  len(files),
		Groups:      []DuplicateGroup{},
		Suggestions: []CleanupSuggestion{},
	}

	var totalWasted int64

	for hash, group := range hashGroups {
		if len(group) < 2 {
			continue // 不是重复文件
		}

		result.DuplicateGroups++
		result.DuplicateFiles += len(group) - 1

		// 找出最旧的文件（建议保留）
		oldestIdx := 0
		for i, f := range group {
			if f.modTime.Before(group[oldestIdx].modTime) {
				oldestIdx = i
			}
		}

		dupGroup := DuplicateGroup{
			Hash:      hash,
			Size:      group[0].size,
			Count:     len(group),
			TotalSize: group[0].size * int64(len(group)),
			Files:     make([]DuplicateFile, 0, len(group)),
			CanDelete: []CleanupSuggestion{},
		}

		for i, f := range group {
			df := DuplicateFile{
				Path:     f.path,
				Size:     f.size,
				ModTime:  f.modTime,
				IsOldest: i == oldestIdx,
			}
			dupGroup.Files = append(dupGroup.Files, df)

			// 非最旧文件生成清理建议
			if i != oldestIdx {
				saved := f.size
				totalWasted += saved
				dupGroup.CanDelete = append(dupGroup.CanDelete, CleanupSuggestion{
					FilePath:   f.path,
					Reason:     fmt.Sprintf("与 %s 重复（哈希: %s...）", group[oldestIdx].path, hash[:8]),
					SavedBytes: saved,
					KeepFile:   group[oldestIdx].path,
				})
			}
		}

		result.Groups = append(result.Groups, dupGroup)
		result.Suggestions = append(result.Suggestions, dupGroup.CanDelete...)
	}

	result.TotalWasted = totalWasted

	// 按浪费空间排序（最大的在前）
	sort.Slice(result.Groups, func(i, j int) bool {
		return result.Groups[i].TotalSize > result.Groups[j].TotalSize
	})
	sort.Slice(result.Suggestions, func(i, j int) bool {
		return result.Suggestions[i].SavedBytes > result.Suggestions[j].SavedBytes
	})

	_ = startTime // 可用于性能统计

	return result, nil
}

// ========== 新增：存储空间使用分析 ==========

// AnalyzeUsage 分析存储空间使用情况.
func (a *Analyzer) AnalyzeUsage(path string, groupBy string) (*StorageUsageAnalysis, error) {
	if path == "" {
		path = "/"
	}
	if groupBy == "" {
		groupBy = "all"
	}

	startTime := time.Now()

	// 收集所有文件
	files, err := a.collectFilesWithDetails(path)
	if err != nil {
		return nil, fmt.Errorf("收集文件信息失败: %w", err)
	}

	analysis := &StorageUsageAnalysis{
		ScanPath:   path,
		TotalFiles: len(files),
		ScanTime:   startTime,
	}

	var totalSize int64
	for _, f := range files {
		totalSize += f.size
	}
	analysis.TotalSize = totalSize

	// 按类型分析
	if groupBy == "all" || groupBy == "type" {
		analysis.ByType = a.groupByType(files, totalSize)
	}

	// 按用户分析
	if groupBy == "all" || groupBy == "user" {
		analysis.ByUser = a.groupByUser(files, totalSize)
	}

	// 按时间分析
	if groupBy == "all" || groupBy == "time" {
		analysis.ByTime = a.groupByTime(files)
	}

	analysis.Duration = time.Since(startTime)

	return analysis, nil
}

// groupByType 按文件类型分组统计.
func (a *Analyzer) groupByType(files []fileInfo, totalSize int64) []UsageByType {
	typeMap := make(map[string]*UsageByType)

	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f.path))
		if ext == "" {
			ext = "(无扩展名)"
		}

		ut, exists := typeMap[ext]
		if !exists {
			ut = &UsageByType{
				Extension: ext,
			}
			typeMap[ext] = ut
		}

		ut.FileCount++
		ut.TotalSize += f.size

		if f.size > ut.LargestSize {
			ut.LargestSize = f.size
			ut.LargestPath = f.path
		}
	}

	result := make([]UsageByType, 0, len(typeMap))
	for _, ut := range typeMap {
		if ut.FileCount > 0 {
			ut.AvgSize = ut.TotalSize / int64(ut.FileCount)
		}
		if totalSize > 0 {
			ut.Percent = float64(ut.TotalSize) / float64(totalSize) * 100
		}
		result = append(result, *ut)
	}

	// 按总大小降序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalSize > result[j].TotalSize
	})

	return result
}

// groupByUser 按用户分组统计.
func (a *Analyzer) groupByUser(files []fileInfo, totalSize int64) []UsageByUser {
	type userKey struct {
		uid  int
		name string
	}

	userMap := make(map[userKey]*UsageByUser)

	for _, f := range files {
		uid := f.uid
		username := f.username
		if username == "" {
			username = fmt.Sprintf("uid_%d", uid)
		}

		key := userKey{uid: uid, name: username}
		uu, exists := userMap[key]
		if !exists {
			uu = &UsageByUser{
				Username: username,
				UID:      uid,
			}
			userMap[key] = uu
		}

		uu.FileCount++
		uu.TotalSize += f.size
	}

	result := make([]UsageByUser, 0, len(userMap))
	for _, uu := range userMap {
		if totalSize > 0 {
			uu.Percent = float64(uu.TotalSize) / float64(totalSize) * 100
		}
		// 尝试获取主目录
		if u, err := user.Lookup(uu.Username); err == nil {
			uu.HomeDir = u.HomeDir
		}
		result = append(result, *uu)
	}

	// 按总大小降序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalSize > result[j].TotalSize
	})

	return result
}

// groupByTime 按时间分组统计.
func (a *Analyzer) groupByTime(files []fileInfo) []UsageByTime {
	timeMap := make(map[string]*UsageByTime)

	for _, f := range files {
		period := f.modTime.Format("2006-01") // 按月分组

		ut, exists := timeMap[period]
		if !exists {
			ut = &UsageByTime{
				Period: period,
			}
			timeMap[period] = ut
		}

		ut.FileCount++
		ut.TotalSize += f.size
		ut.NewFiles++ // 近似：该月创建的文件
	}

	result := make([]UsageByTime, 0, len(timeMap))
	for _, ut := range timeMap {
		result = append(result, *ut)
	}

	// 按时间升序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Period < result[j].Period
	})

	return result
}

// ========== 新增：存储成本估算 ==========

// EstimateCost 估算存储成本.
func (a *Analyzer) EstimateCost(path string, tierCosts map[string]float64, currency string) (*CostEstimate, error) {
	if path == "" {
		path = "/"
	}
	if currency == "" {
		currency = "CNY"
	}

	// 执行分析获取效率数据
	summary, err := a.Analyze(path, a.sampleRate, true)
	if err != nil {
		return nil, fmt.Errorf("分析存储效率失败: %w", err)
	}

	// 默认成本（元/GB/月）
	defaultCosts := map[StorageTier]float64{
		TierSSD:   0.5,  // SSD: 0.5元/GB/月
		TierHDD:   0.1,  // HDD: 0.1元/GB/月
		TierCloud: 0.15, // 云存储: 0.15元/GB/月
		TierTape:  0.02, // 磁带: 0.02元/GB/月
	}

	// 应用自定义成本
	if tierCosts != nil {
		if v, ok := tierCosts["ssd"]; ok {
			defaultCosts[TierSSD] = v
		}
		if v, ok := tierCosts["hdd"]; ok {
			defaultCosts[TierHDD] = v
		}
		if v, ok := tierCosts["cloud"]; ok {
			defaultCosts[TierCloud] = v
		}
		if v, ok := tierCosts["tape"]; ok {
			defaultCosts[TierTape] = v
		}
	}

	totalSizeGB := float64(summary.TotalLogicalSize) / (1024 * 1024 * 1024)
	effectiveSizeGB := float64(summary.TotalPhysicalSize) / (1024 * 1024 * 1024)

	// 假设数据分布在不同层：60% SSD热数据，30% HDD温数据，10% 冷数据
	tiers := []TierCost{
		{
			Tier:        TierSSD,
			CostPerGB:   defaultCosts[TierSSD],
			CostPerTB:   defaultCosts[TierSSD] * 1024,
			TotalSizeGB: effectiveSizeGB * 0.6,
			MonthlyCost: effectiveSizeGB * 0.6 * defaultCosts[TierSSD],
			YearlyCost:  effectiveSizeGB * 0.6 * defaultCosts[TierSSD] * 12,
		},
		{
			Tier:        TierHDD,
			CostPerGB:   defaultCosts[TierHDD],
			CostPerTB:   defaultCosts[TierHDD] * 1024,
			TotalSizeGB: effectiveSizeGB * 0.3,
			MonthlyCost: effectiveSizeGB * 0.3 * defaultCosts[TierHDD],
			YearlyCost:  effectiveSizeGB * 0.3 * defaultCosts[TierHDD] * 12,
		},
		{
			Tier:        TierCloud,
			CostPerGB:   defaultCosts[TierCloud],
			CostPerTB:   defaultCosts[TierCloud] * 1024,
			TotalSizeGB: effectiveSizeGB * 0.1,
			MonthlyCost: effectiveSizeGB * 0.1 * defaultCosts[TierCloud],
			YearlyCost:  effectiveSizeGB * 0.1 * defaultCosts[TierCloud] * 12,
		},
	}

	var totalMonthly, totalYearly float64
	for _, t := range tiers {
		totalMonthly += t.MonthlyCost
		totalYearly += t.YearlyCost
	}

	// 如果没有压缩/去重节省，成本就是全部
	noSavingsMonthly := totalSizeGB * defaultCosts[TierHDD] // 假设无优化时全部用HDD

	estimate := &CostEstimate{
		TotalSizeGB:     totalSizeGB,
		EffectiveSizeGB: effectiveSizeGB,
		SavingsPercent:  summary.SpaceSavedPercent,
		Tiers:           tiers,
		TotalMonthly:    totalMonthly,
		TotalYearly:     totalYearly,
		SavingsMonthly:  noSavingsMonthly - totalMonthly,
		SavingsYearly:   (noSavingsMonthly - totalMonthly) * 12,
		Currency:        currency,
		EstimatedAt:     time.Now(),
	}

	return estimate, nil
}

// ========== 内部方法 ==========

// fileInfo 文件元信息.
type fileInfo struct {
	path         string
	size         int64
	modTime      time.Time
	hash         string
	compressible bool
	uid          int
	username     string
}

// collectFilesWithDetails 收集文件详细信息（包含用户信息）.
func (a *Analyzer) collectFilesWithDetails(rootPath string) ([]fileInfo, error) {
	var files []fileInfo

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		name := info.Name()
		if strings.HasPrefix(name, ".") || name == "proc" || name == "sys" || name == "dev" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return nil
		}

		fi := fileInfo{
			path:         path,
			size:         info.Size(),
			modTime:      info.ModTime(),
			compressible: isCompressible(path),
		}

		// 获取文件所有者信息（Unix平台）
		if sysInfo := getFileInfo(info.Sys()); sysInfo != nil {
			fi.uid = sysInfo.uid
			if u, err := user.LookupId(strconv.Itoa(sysInfo.uid)); err == nil {
				fi.username = u.Username
			}
		}

		files = append(files, fi)
		return nil
	})

	return files, err
}

// collectFiles 收集文件元信息（支持采样）.
func (a *Analyzer) collectFiles(rootPath string, sampleRate int, deepScan bool) ([]fileInfo, error) {
	var files []fileInfo
	count := 0

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		name := info.Name()
		if strings.HasPrefix(name, ".") || name == "proc" || name == "sys" || name == "dev" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return nil
		}

		count++
		if sampleRate < 100 && count%sampleRate != 0 {
			return nil
		}

		fi := fileInfo{
			path:         path,
			size:         info.Size(),
			modTime:      info.ModTime(),
			compressible: isCompressible(path),
		}

		if deepScan && info.Size() <= a.maxFileSize {
			hash, err := fileHash(path)
			if err == nil {
				fi.hash = hash
			}
		}

		files = append(files, fi)
		return nil
	})

	return files, err
}

// analyzeCompression 分析压缩效率.
func (a *Analyzer) analyzeCompression(files []fileInfo) *CompressionStats {
	stats := &CompressionStats{}

	if len(files) == 0 {
		return stats
	}

	var totalRatio float64
	ratioCount := 0
	bestRatio := 0.0
	worstRatio := math.MaxFloat64

	for _, f := range files {
		if !f.compressible {
			stats.UncompressedFiles++
			continue
		}

		stats.CompressedFiles++
		stats.TotalOriginalSize += f.size

		estimatedCompressed := estimateCompressedSize(f)
		stats.TotalCompressedSize += estimatedCompressed

		if f.size > 0 {
			ratio := float64(f.size) / float64(estimatedCompressed)
			if ratio > 0 {
				totalRatio += ratio
				ratioCount++
				if ratio > bestRatio {
					bestRatio = ratio
				}
				if ratio < worstRatio {
					worstRatio = ratio
				}
			}
		}
	}

	if ratioCount > 0 {
		stats.AverageRatio = totalRatio / float64(ratioCount)
	}
	if bestRatio > 0 {
		stats.BestRatio = bestRatio
	}
	if worstRatio < math.MaxFloat64 {
		stats.WorstRatio = worstRatio
	}

	return stats
}

// analyzeDedup 分析去重效率.
func (a *Analyzer) analyzeDedup(files []fileInfo) *DedupStats {
	stats := &DedupStats{
		TotalFiles: len(files),
	}

	if len(files) == 0 {
		return stats
	}

	sizeGroups := make(map[int64][]fileInfo)
	for _, f := range files {
		sizeGroups[f.size] = append(sizeGroups[f.size], f)
	}

	hashSet := make(map[string]int)
	duplicateCount := 0
	uniqueCount := 0

	for _, group := range sizeGroups {
		if len(group) == 1 {
			uniqueCount++
			hashSet[group[0].hash] = 1
			continue
		}

		for _, f := range group {
			if f.hash == "" {
				hash := fmt.Sprintf("size_%d", f.size)
				hashSet[hash]++
			} else {
				hashSet[f.hash]++
			}
		}
	}

	for _, count := range hashSet {
		if count > 1 {
			duplicateCount += count - 1
		}
		uniqueCount++
	}

	stats.UniqueFiles = uniqueCount
	stats.DuplicateFiles = duplicateCount
	if stats.TotalFiles > 0 {
		stats.DedupPercent = float64(duplicateCount) / float64(stats.TotalFiles) * 100
	}

	blockSize := int64(4096)
	totalBlocks := 0
	uniqueBlocks := make(map[string]bool)

	for _, f := range files {
		blocks := (f.size + blockSize - 1) / blockSize
		totalBlocks += int(blocks)

		if f.hash != "" {
			uniqueBlocks[f.hash] = true
		} else {
			uniqueBlocks[fmt.Sprintf("size_%d", f.size)] = true
		}
	}

	stats.TotalBlocks = totalBlocks
	stats.UniqueBlocks = len(uniqueBlocks)
	if totalBlocks > 0 {
		stats.BlockDedupRatio = float64(stats.UniqueBlocks) / float64(totalBlocks)
	}

	savedBytes := int64(0)
	for _, f := range files {
		hash := f.hash
		if hash == "" {
			hash = fmt.Sprintf("size_%d", f.size)
		}
		if hashSet[hash] > 1 {
			savedBytes += f.size
		}
	}
	stats.SpaceSavedBytes = savedBytes / 2

	return stats
}

// buildSummary 根据压缩和去重统计构建总览.
func (a *Analyzer) buildSummary(comp *CompressionStats, dedup *DedupStats) *EfficiencySummary {
	totalLogical := comp.TotalOriginalSize
	if totalLogical == 0 {
		totalLogical = dedup.SpaceSavedBytes * 2
	}

	totalPhysical := comp.TotalCompressedSize
	if totalPhysical == 0 {
		totalPhysical = totalLogical
	}

	spaceSaved := totalLogical - totalPhysical
	if spaceSaved < 0 {
		spaceSaved = 0
	}

	summary := &EfficiencySummary{
		TotalLogicalSize:  totalLogical,
		TotalPhysicalSize: totalPhysical,
		SpaceSaved:        spaceSaved,
		UpdatedAt:         time.Now(),
	}

	if totalPhysical > 0 {
		summary.CompressionRatio = float64(totalLogical) / float64(totalPhysical)
	}

	summary.DedupRatio = dedup.BlockDedupRatio

	if totalLogical > 0 {
		summary.SpaceSavedPercent = float64(spaceSaved) / float64(totalLogical) * 100
	}

	return summary
}

// recordHistory 记录历史数据.
func (a *Analyzer) recordHistory(summary *EfficiencySummary) {
	record := internalRecord{
		Timestamp:        time.Now(),
		LogicalSize:      summary.TotalLogicalSize,
		PhysicalSize:     summary.TotalPhysicalSize,
		CompressionRatio: summary.CompressionRatio,
		DedupRatio:       summary.DedupRatio,
		SpaceSaved:       summary.SpaceSaved,
	}

	a.mu.Lock()
	a.history = append(a.history, record)

	cutoff := time.Now().AddDate(0, 0, -90)
	filtered := make([]internalRecord, 0, len(a.history))
	for _, r := range a.history {
		if r.Timestamp.After(cutoff) {
			filtered = append(filtered, r)
		}
	}
	a.history = filtered
	a.mu.Unlock()

	go func() { _ = a.saveHistory() }()
}

// isCompressible 判断文件是否可压缩.
func isCompressible(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))

	compressed := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".webp": true, ".mp4": true, ".mkv": true, ".avi": true,
		".mov": true, ".mp3": true, ".flac": true, ".aac": true,
		".ogg": true, ".zip": true, ".rar": true, ".7z": true,
		".gz": true, ".bz2": true, ".xz": true, ".zst": true,
		".lz4": true, ".snappy": true,
	}

	if compressed[ext] {
		return false
	}

	compressible := map[string]bool{
		".txt": true, ".log": true, ".csv": true, ".json": true,
		".xml": true, ".yaml": true, ".yml": true, ".md": true,
		".html": true, ".htm": true, ".css": true, ".js": true,
		".ts": true, ".go": true, ".py": true, ".java": true,
		".c": true, ".cpp": true, ".h": true, ".rs": true,
		".sql": true, ".doc": true, ".docx": true, ".xls": true,
		".xlsx": true, ".ppt": true, ".pptx": true, ".pdf": true,
		".rtf": true, ".odt": true, ".ods": true, ".odp": true,
		".svg": true, ".psd": true,
	}

	return compressible[ext]
}

// estimateCompressedSize 估算文件压缩后大小.
func estimateCompressedSize(f fileInfo) int64 {
	ext := strings.ToLower(filepath.Ext(f.path))

	ratios := map[string]float64{
		".txt":  0.3,
		".log":  0.1,
		".csv":  0.2,
		".json": 0.2,
		".xml":  0.2,
		".yaml": 0.25,
		".yml":  0.25,
		".md":   0.3,
		".html": 0.3,
		".htm":  0.3,
		".css":  0.3,
		".js":   0.35,
		".ts":   0.35,
		".go":   0.35,
		".py":   0.35,
		".java": 0.35,
		".sql":  0.25,
		".doc":  0.6,
		".docx": 0.5,
		".xls":  0.6,
		".xlsx": 0.5,
		".ppt":  0.7,
		".pptx": 0.6,
		".pdf":  0.8,
		".rtf":  0.4,
		".svg":  0.3,
		".psd":  0.7,
	}

	ratio, ok := ratios[ext]
	if !ok {
		ratio = 0.5
	}

	return int64(float64(f.size) * ratio)
}

// fileHash 计算文件SHA256哈希.
func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
