package storageanalytics

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Analyzer 存储分析引擎.
type Analyzer struct {
	mu      sync.RWMutex
	config  *Config
	logger  *zap.Logger
	lastReport *StorageReport
	lastCollect *CollectResult
}

// NewAnalyzer 创建分析引擎.
func NewAnalyzer(config *Config, logger *zap.Logger) *Analyzer {
	if config == nil {
		config = DefaultConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Analyzer{
		config: config,
		logger: logger,
	}
}

// Analyze 对采集结果进行全量分析，生成报告.
func (a *Analyzer) Analyze(result *CollectResult) *StorageReport {
	report := &StorageReport{
		ScanPath:    result.ScanPath,
		GeneratedAt: time.Now(),
	}

	// 存储概览
	report.Summary = a.buildSummary(result)

	// 文件类型统计
	report.FileTypeStats = a.analyzeFileTypeStats(result)

	// Top N 目录
	report.TopDirectories = a.getTopDirectories(result)

	// 大小分布
	report.SizeDist = a.analyzeSizeDistribution(result)

	// 年龄分布
	report.AgeDist = a.analyzeAgeDistribution(result)

	// 访问频率分布
	report.AccessDist = a.analyzeAccessDistribution(result)

	// 健康指标
	report.Health = a.analyzeHealth(result)

	// 趋势分析（基于历史报告）
	report.Trends = a.analyzeTrends(result)

	// 智能洞察
	report.Insights = a.analyzeInsights(result)

	a.mu.Lock()
	a.lastReport = report
	a.lastCollect = result
	a.mu.Unlock()

	a.logger.Info("分析完成",
		zap.String("path", result.ScanPath),
		zap.Int64("total_size", result.TotalSize),
		zap.Int("files", result.TotalFiles),
	)

	return report
}

// GetLastReport 获取最后一次分析报告.
func (a *Analyzer) GetLastReport() (*StorageReport, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.lastReport == nil {
		return nil, ErrNoAnalysisData
	}
	return a.lastReport, nil
}

// GetLastCollect 获取最后一次采集结果.
func (a *Analyzer) GetLastCollect() (*CollectResult, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.lastCollect == nil {
		return nil, ErrNoAnalysisData
	}
	return a.lastCollect, nil
}

// buildSummary 构建存储概览.
func (a *Analyzer) buildSummary(result *CollectResult) Summary {
	s := Summary{
		TotalSize:  result.TotalSize,
		TotalFiles: result.TotalFiles,
		TotalDirs:  result.TotalDirs,
	}

	if len(result.Files) == 0 {
		return s
	}

	// 找最大文件和最老文件
	var largestSize int64
	var oldestTime time.Time
	for _, f := range result.Files {
		if f.Size > largestSize {
			largestSize = f.Size
			s.LargestFile = f.Path
			s.LargestSize = f.Size
		}
		if oldestTime.IsZero() || f.ModTime.Before(oldestTime) {
			oldestTime = f.ModTime
			s.OldestFile = f.Path
		}
	}

	if !oldestTime.IsZero() {
		s.OldestAge = formatDuration(time.Since(oldestTime))
	}

	// 平均文件大小
	s.AvgFileSize = result.TotalSize / int64(result.TotalFiles)

	// 中位数文件大小
	sizes := make([]int64, len(result.Files))
	for i, f := range result.Files {
		sizes[i] = f.Size
	}
	sort.Slice(sizes, func(i, j int) bool { return sizes[i] < sizes[j] })
	mid := len(sizes) / 2
	if len(sizes)%2 == 0 && mid > 0 {
		s.MedianFileSize = (sizes[mid-1] + sizes[mid]) / 2
	} else if mid < len(sizes) {
		s.MedianFileSize = sizes[mid]
	}

	return s
}

// analyzeFileTypeStats 按文件类型统计.
func (a *Analyzer) analyzeFileTypeStats(result *CollectResult) []CategoryStat {
	counts := map[FileType]*CategoryStat{
		FileTypeImage:    {Category: "图片"},
		FileTypeVideo:    {Category: "视频"},
		FileTypeDocument: {Category: "文档"},
		FileTypeArchive:  {Category: "压缩包"},
		FileTypeCode:     {Category: "代码"},
		FileTypeOther:    {Category: "其他"},
	}

	for _, f := range result.Files {
		if cs, ok := counts[f.FileType]; ok {
			cs.FileCount++
			cs.TotalSize += f.Size
		}
	}

	stats := make([]CategoryStat, 0, len(counts))
	for _, cs := range counts {
		if result.TotalSize > 0 {
			cs.Percentage = round2(float64(cs.TotalSize) / float64(result.TotalSize) * 100)
		}
		stats = append(stats, *cs)
	}

	// 按大小降序排序
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].TotalSize > stats[j].TotalSize
	})

	return stats
}

// getTopDirectories 获取Top N大目录.
func (a *Analyzer) getTopDirectories(result *CollectResult) []DirectoryInfo {
	if len(result.Directories) == 0 {
		return nil
	}

	dirs := make([]DirectoryInfo, len(result.Directories))
	copy(dirs, result.Directories)

	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].TotalSize > dirs[j].TotalSize
	})

	if len(dirs) > a.config.DefaultTopN {
		dirs = dirs[:a.config.DefaultTopN]
	}

	return dirs
}

// analyzeSizeDistribution 文件大小分布分析.
func (a *Analyzer) analyzeSizeDistribution(result *CollectResult) []SizeDistribution {
	brackets := map[SizeBracket]*SizeDistribution{
		SizeLT1MB:     {Bracket: SizeLT1MB},
		Size1MBTo100:  {Bracket: Size1MBTo100},
		Size100MBTo1G: {Bracket: Size100MBTo1G},
		SizeGT1GB:     {Bracket: SizeGT1GB},
	}

	for _, f := range result.Files {
		b := classifySizeBracket(f.Size)
		if d, ok := brackets[b]; ok {
			d.FileCount++
			d.TotalSize += f.Size
		}
	}

	dists := make([]SizeDistribution, 0, len(brackets))
	for _, d := range brackets {
		if result.TotalSize > 0 {
			d.Percentage = round2(float64(d.TotalSize) / float64(result.TotalSize) * 100)
		}
		dists = append(dists, *d)
	}

	return dists
}

// analyzeAgeDistribution 文件年龄分布分析.
func (a *Analyzer) analyzeAgeDistribution(result *CollectResult) []AgeDistribution {
	brackets := map[AgeBracket]*AgeDistribution{
		AgeLT7Days:    {Bracket: AgeLT7Days},
		Age7To30Days:  {Bracket: Age7To30Days},
		Age30To90Days: {Bracket: Age30To90Days},
		Age90To365:    {Bracket: Age90To365},
		AgeGT1Year:    {Bracket: AgeGT1Year},
	}

	for _, f := range result.Files {
		b := classifyAgeBracket(f.ModTime)
		if d, ok := brackets[b]; ok {
			d.FileCount++
			d.TotalSize += f.Size
		}
	}

	dists := make([]AgeDistribution, 0, len(brackets))
	for _, d := range brackets {
		if result.TotalSize > 0 {
			d.Percentage = round2(float64(d.TotalSize) / float64(result.TotalSize) * 100)
		}
		dists = append(dists, *d)
	}

	return dists
}

// analyzeAccessDistribution 访问频率分布分析.
func (a *Analyzer) analyzeAccessDistribution(result *CollectResult) []AccessDistribution {
	freqs := map[AccessFrequency]*AccessDistribution{
		AccessFrequent:   {Frequency: AccessFrequent},
		AccessOccasional: {Frequency: AccessOccasional},
		AccessRare:       {Frequency: AccessRare},
		AccessNever:      {Frequency: AccessNever},
	}

	for _, f := range result.Files {
		freq := classifyAccessFrequency(f.AccessTime)
		if d, ok := freqs[freq]; ok {
			d.FileCount++
			d.TotalSize += f.Size
		}
	}

	dists := make([]AccessDistribution, 0, len(freqs))
	for _, d := range freqs {
		if result.TotalSize > 0 {
			d.Percentage = round2(float64(d.TotalSize) / float64(result.TotalSize) * 100)
		}
		dists = append(dists, *d)
	}

	return dists
}

// analyzeHealth 计算存储健康指标.
func (a *Analyzer) analyzeHealth(result *CollectResult) HealthMetrics {
	if result.TotalFiles == 0 {
		return HealthMetrics{}
	}

	// 碎片化评分：基于文件大小的标准差和目录深度
	fragScore := a.calcFragmentationScore(result)

	// 效率评分：大文件占比越高效率越高（顺序读写更高效）
	effScore := a.calcEfficiencyScore(result)

	// 冗余率：基于文件名相似性估算
	redundancy := a.estimateRedundancy(result)

	// 备份覆盖率：基于文件年龄和修改频率估算
	backupCoverage := a.estimateBackupCoverage(result)

	overall := (fragScore*0.2 + effScore*0.3 + (1-redundancy)*100*0.25 + backupCoverage*0.25)

	return HealthMetrics{
		FragmentationScore: round2(fragScore),
		EfficiencyScore:    round2(effScore),
		RedundancyRate:     round2(redundancy),
		BackupCoverage:     round2(backupCoverage),
		OverallScore:       round2(math.Min(100, math.Max(0, overall))),
	}
}

// calcFragmentationScore 碎片化评分.
func (a *Analyzer) calcFragmentationScore(result *CollectResult) float64 {
	if len(result.Files) < 2 {
		return 100
	}

	// 计算文件大小标准差
	var sum, sumSq float64
	n := float64(len(result.Files))
	for _, f := range result.Files {
		v := float64(f.Size)
		sum += v
		sumSq += v * v
	}
	mean := sum / n
	variance := sumSq/n - mean*mean
	if variance < 0 {
		variance = 0
	}
	stddev := math.Sqrt(variance)

	// 标准差越小，碎片化越低，评分越高
	// 用变异系数(CV)来衡量
	cv := 0.0
	if mean > 0 {
		cv = stddev / mean
	}

	// CV < 1 → 高分，CV > 5 → 低分
	score := 100 - cv*20
	return math.Max(0, math.Min(100, score))
}

// calcEfficiencyScore 存储效率评分.
func (a *Analyzer) calcEfficiencyScore(result *CollectResult) float64 {
	if result.TotalFiles == 0 {
		return 100
	}

	// 评估维度：
	// 1. 大文件占比（>100MB的文件越多效率越高）
	// 2. 临时/缓存文件占比（越低越好）
	// 3. 文件平均大小
	largeFileCount := 0
	largeFileSize := int64(0)
	wasteCount := 0
	wasteSize := int64(0)

	for _, f := range result.Files {
		if f.Size > 100*1024*1024 {
			largeFileCount++
			largeFileSize += f.Size
		}
		if a.isWasteFile(f.Path) {
			wasteCount++
			wasteSize += f.Size
		}
	}

	// 大文件存储效率加分
	largeRatio := float64(largeFileSize) / float64(result.TotalSize) * 100
	// 垃圾文件扣分
	wasteRatio := float64(wasteSize) / float64(result.TotalSize) * 100
	// 平均文件大小适中（10KB-10MB为最佳）
	avgSize := float64(result.TotalSize) / float64(result.TotalFiles)
	avgScore := 100.0
	if avgSize < 10*1024 {
		avgScore = 60 // 太多小文件
	} else if avgSize > 100*1024*1024 {
		avgScore = 80 // 文件偏大
	}

	return math.Max(0, math.Min(100, largeRatio*0.3+(100-wasteRatio)*0.4+avgScore*0.3))
}

// estimateRedundancy 估算数据冗余率.
func (a *Analyzer) estimateRedundancy(result *CollectResult) float64 {
	if len(result.Files) < 2 {
		return 0
	}

	// 简化实现：基于相同大小文件的比例估算
	sizeCount := make(map[int64]int)
	for _, f := range result.Files {
		sizeCount[f.Size]++
	}

	duplicateCount := 0
	for _, count := range sizeCount {
		if count > 1 {
			duplicateCount += count - 1
		}
	}

	rate := float64(duplicateCount) / float64(len(result.Files))
	return math.Min(1, rate)
}

// estimateBackupCoverage 估算备份覆盖率.
func (a *Analyzer) estimateBackupCoverage(result *CollectResult) float64 {
	if len(result.Files) == 0 {
		return 0
	}

	// 启发式规则：
	// 1. 最近7天内修改的文件 → 覆盖率基于是否有 .bak/.backup 存在
	// 2. 超过30天未修改的文件 → 默认假设已备份
	recentFiles := 0
	oldFiles := 0
	for _, f := range result.Files {
		if time.Since(f.ModTime) < 7*24*time.Hour {
			recentFiles++
		} else if time.Since(f.ModTime) > 30*24*time.Hour {
			oldFiles++
		}
	}

	// 简化估算：旧文件假设已备份80%，新文件假设50%
	total := float64(len(result.Files))
	if total == 0 {
		return 0
	}
	coverage := (float64(oldFiles)*0.8 + float64(recentFiles)*0.5) / total
	return math.Min(1, math.Max(0, coverage))
}

// analyzeTrends 趋势分析.
func (a *Analyzer) analyzeTrends(result *CollectResult) TrendAnalysis {
	a.mu.RLock()
	prev := a.lastCollect
	a.mu.RUnlock()

	trend := TrendAnalysis{
		DailyGrowthRate: 0,
		DaysUntilFull:   -1,
	}

	// 模拟每日趋势（基于文件修改时间的分布）
	dailyMap := make(map[string]*TrendPoint)
	weeklyMap := make(map[string]*TrendPoint)
	monthlyMap := make(map[string]*TrendPoint)

	for _, f := range result.Files {
		dayKey := f.ModTime.Format("2006-01-02")
		weekKey := f.ModTime.Format("2006-W01")
		monthKey := f.ModTime.Format("2006-01")

		if dp, ok := dailyMap[dayKey]; ok {
			dp.TotalSize += f.Size
			dp.FileCount++
		} else {
			dailyMap[dayKey] = &TrendPoint{
				Date:      f.ModTime.Truncate(24 * time.Hour),
				TotalSize: f.Size,
				FileCount: 1,
			}
		}

		if wp, ok := weeklyMap[weekKey]; ok {
			wp.TotalSize += f.Size
			wp.FileCount++
		} else {
			weeklyMap[weekKey] = &TrendPoint{
				Date:      f.ModTime.Truncate(7 * 24 * time.Hour),
				TotalSize: f.Size,
				FileCount: 1,
			}
		}

		if mp, ok := monthlyMap[monthKey]; ok {
			mp.TotalSize += f.Size
			mp.FileCount++
		} else {
			monthlyMap[monthKey] = &TrendPoint{
				Date:      f.ModTime.Truncate(30 * 24 * time.Hour),
				TotalSize: f.Size,
				FileCount: 1,
			}
		}
	}

	// 转换为切片
	trend.Daily = mapToSortedSlice(dailyMap)
	trend.Weekly = mapToSortedSlice(weeklyMap)
	trend.Monthly = mapToSortedSlice(monthlyMap)

	// 计算增长：如果有多次分析记录
	if prev != nil && prev.TotalSize > 0 {
		sizeDelta := result.TotalSize - prev.TotalSize
		timeDelta := result.ScanTime.Sub(prev.ScanTime)
		daysDelta := int64(timeDelta.Hours() / 24)
		if daysDelta > 0 {
			trend.DailyGrowthRate = sizeDelta / daysDelta
		} else if sizeDelta != 0 {
			// 两次分析间隔不足一天，用秒级精度估算日增长率
			secsDelta := int64(timeDelta.Seconds())
			if secsDelta > 0 {
				trend.DailyGrowthRate = sizeDelta * 86400 / secsDelta
			}
		}
	}

	// 各类数据增长对比
	trend.CategoryGrowth = a.calcCategoryGrowth(result, prev)

	return trend
}

// calcCategoryGrowth 计算各类数据增长对比.
func (a *Analyzer) calcCategoryGrowth(current *CollectResult, prev *CollectResult) []CategoryGrowthInfo {
	currentByType := make(map[FileType]int64)
	for _, f := range current.Files {
		currentByType[f.FileType] += f.Size
	}

	prevByType := make(map[FileType]int64)
	if prev != nil {
		for _, f := range prev.Files {
			prevByType[f.FileType] += f.Size
		}
	}

	categoryNames := map[FileType]string{
		FileTypeImage:    "图片",
		FileTypeVideo:    "视频",
		FileTypeDocument: "文档",
		FileTypeArchive:  "压缩包",
		FileTypeCode:     "代码",
		FileTypeOther:    "其他",
	}

	result := make([]CategoryGrowthInfo, 0, len(categoryNames))
	for ft, name := range categoryNames {
		currentSize := currentByType[ft]
		prevSize := prevByType[ft]
		growth := currentSize - prevSize
		growthPct := 0.0
		if prevSize > 0 {
			growthPct = float64(growth) / float64(prevSize) * 100
		}

		result = append(result, CategoryGrowthInfo{
			Category:      name,
			GrowthBytes:   growth,
			GrowthPercent: round2(growthPct),
			CurrentSize:   currentSize,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].GrowthBytes > result[j].GrowthBytes
	})

	return result
}

// analyzeInsights 智能洞察分析.
func (a *Analyzer) analyzeInsights(result *CollectResult) InsightAnalysis {
	var insights []Insight
	var wastedSpace int64

	// 1. 异常增长检测
	anomalies := a.detectAnomalies(result)
	insights = append(insights, anomalies...)

	// 2. 存储浪费识别
	wastes := a.detectWaste(result)
	for _, w := range wastes {
		wastedSpace += w.Saving
	}
	insights = append(insights, wastes...)

	// 3. 优化建议
	optims := a.generateOptimizations(result)
	insights = append(insights, optims...)

	totalSaving := int64(0)
	for _, in := range insights {
		totalSaving += in.Saving
	}

	return InsightAnalysis{
		Insights:             insights,
		WastedSpace:          wastedSpace,
		TotalPotentialSaving: totalSaving,
	}
}

// detectAnomalies 异常增长检测.
func (a *Analyzer) detectAnomalies(result *CollectResult) []Insight {
	var insights []Insight

	// 统计每个目录的大小
	dirSizes := make(map[string]int64)
	for _, d := range result.Directories {
		dirSizes[d.Path] = d.TotalSize
	}

	if len(dirSizes) == 0 {
		return insights
	}

	// 计算平均和标准差
	var sum, sumSq float64
	n := float64(len(dirSizes))
	for _, s := range dirSizes {
		v := float64(s)
		sum += v
		sumSq += v * v
	}
	mean := sum / n
	variance := sumSq/n - mean*mean
	if variance <= 0 {
		return insights
	}
	stddev := math.Sqrt(variance)

	// 超过 2 倍标准差的目录视为异常
	for dir, size := range dirSizes {
		if size > int64(mean+2*stddev) && size > 100*1024*1024 {
			insights = append(insights, Insight{
				Type:     "anomaly",
				Severity: "high",
				Title:    "目录异常增长",
				Detail:   dir + " 占用 " + formatBytes(size) + "，明显高于平均值",
				Saving:   0,
				Action:   "检查该目录是否正常，考虑清理或迁移",
			})
		}
	}

	return insights
}

// detectWaste 存储浪费识别.
func (a *Analyzer) detectWaste(result *CollectResult) []Insight {
	var insights []Insight
	var totalWaste int64

	// 统计临时文件、日志、缓存
	wasteCategories := map[string]int64{
		"临时文件": 0,
		"日志文件": 0,
		"缓存文件": 0,
	}
	wasteFiles := map[string][]string{
		"临时文件": {},
		"日志文件": {},
		"缓存文件": {},
	}

	for _, f := range result.Files {
		path := f.Path
		if a.isWasteFile(path) {
			switch {
			case isTempFile(path):
				wasteCategories["临时文件"] += f.Size
				if len(wasteFiles["临时文件"]) < 5 {
					wasteFiles["临时文件"] = append(wasteFiles["临时文件"], path)
				}
			case isLogFile(path):
				wasteCategories["日志文件"] += f.Size
				if len(wasteFiles["日志文件"]) < 5 {
					wasteFiles["日志文件"] = append(wasteFiles["日志文件"], path)
				}
			default:
				wasteCategories["缓存文件"] += f.Size
				if len(wasteFiles["缓存文件"]) < 5 {
					wasteFiles["缓存文件"] = append(wasteFiles["缓存文件"], path)
				}
			}
			totalWaste += f.Size
		}
	}

	for category, size := range wasteCategories {
		if size > 0 {
			severity := "low"
			if size > 1024*1024*1024 { // > 1GB
				severity = "high"
			} else if size > 100*1024*1024 { // > 100MB
				severity = "medium"
			}

			detail := "占用 " + formatBytes(size)
			if len(wasteFiles[category]) > 0 {
				detail += "，示例: " + wasteFiles[category][0]
			}

			insights = append(insights, Insight{
				Type:     "waste",
				Severity: severity,
				Title:    category + "占用存储空间",
				Detail:   detail,
				Saving:   size,
				Action:   "建议定期清理" + category,
			})
		}
	}

	return insights
}

// generateOptimizations 生成优化建议.
func (a *Analyzer) generateOptimizations(result *CollectResult) []Insight {
	var insights []Insight

	// 大文件优化建议
	largeFiles := make([]FileInfo, 0)
	for _, f := range result.Files {
		if f.Size > 1024*1024*1024 { // > 1GB
			largeFiles = append(largeFiles, f)
		}
	}
	if len(largeFiles) > 0 {
		totalLargeSize := int64(0)
		for _, f := range largeFiles {
			totalLargeSize += f.Size
		}
		insights = append(insights, Insight{
			Type:     "optimization",
			Severity: "medium",
			Title:    "大文件压缩优化",
			Detail:   formatBytes(totalLargeSize) + " 数据可考虑压缩存储",
			Saving:   totalLargeSize / 5, // 预估20%压缩率
			Action:   "对大于1GB的文件启用压缩，预计节省20%空间",
		})
	}

	// 冷数据归档建议
	var coldDataSize int64
	for _, f := range result.Files {
		if time.Since(f.AccessTime) > 365*24*time.Hour {
			coldDataSize += f.Size
		}
	}
	if coldDataSize > 1024*1024*1024 { // > 1GB
		insights = append(insights, Insight{
			Type:     "optimization",
			Severity: "medium",
			Title:    "冷数据归档",
			Detail:   "超过1年未访问的数据有 " + formatBytes(coldDataSize),
			Saving:   coldDataSize,
			Action:   "将冷数据归档到低成本存储层",
		})
	}

	// 重复文件检测建议
	dupCount := a.countDuplicateFiles(result)
	if dupCount > 10 {
		insights = append(insights, Insight{
			Type:     "optimization",
			Severity: "low",
			Title:    "潜在重复文件",
			Detail:   "发现 " + itoa(dupCount) + " 组大小相同的文件，可能存在重复",
			Saving:   0,
			Action:   "建议使用去重工具检查并清理重复文件",
		})
	}

	return insights
}

// isWasteFile 判断是否为浪费文件.
func (a *Analyzer) isWasteFile(path string) bool {
	lower := strings.ToLower(path)
	return isTempFile(lower) || isLogFile(lower) || isCacheFile(lower)
}

// isTempFile 判断是否为临时文件.
func isTempFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".tmp") ||
		strings.HasSuffix(lower, ".temp") ||
		strings.HasSuffix(lower, ".swp") ||
		strings.Contains(lower, ".bak") ||
		strings.Contains(lower, ".old")
}

// isLogFile 判断是否为日志文件.
func isLogFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".log")
}

// isCacheFile 判断是否为缓存文件.
func isCacheFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.Contains(lower, "cache") ||
		strings.Contains(lower, "__pycache__") ||
		strings.Contains(lower, "node_modules/.cache") ||
		strings.HasSuffix(lower, ".cache") ||
		strings.Contains(lower, "thumbs.db") ||
		strings.Contains(lower, ".ds_store")
}

// countDuplicateFiles 统计潜在重复文件数量.
func (a *Analyzer) countDuplicateFiles(result *CollectResult) int {
	sizeCount := make(map[int64]int)
	for _, f := range result.Files {
		sizeCount[f.Size]++
	}
	dup := 0
	for _, count := range sizeCount {
		if count > 1 {
			dup += count - 1
		}
	}
	return dup
}

// mapToSortedSlice 将趋势map转换为按日期排序的切片.
func mapToSortedSlice(m map[string]*TrendPoint) []TrendPoint {
	slice := make([]TrendPoint, 0, len(m))
	for _, p := range m {
		slice = append(slice, *p)
	}
	sort.Slice(slice, func(i, j int) bool {
		return slice[i].Date.Before(slice[j].Date)
	})
	return slice
}

// round2 保留两位小数.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// formatBytes 格式化字节数.
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case bytes >= TB:
		return formatFloat(float64(bytes)/float64(TB)) + " TB"
	case bytes >= GB:
		return formatFloat(float64(bytes)/float64(GB)) + " GB"
	case bytes >= MB:
		return formatFloat(float64(bytes)/float64(MB)) + " MB"
	case bytes >= KB:
		return formatFloat(float64(bytes)/float64(KB)) + " KB"
	default:
		return itoa(int(bytes)) + " B"
	}
}

// formatDuration 格式化时间段.
func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	switch {
	case days >= 365:
		return itoa(days/365) + "年" + itoa((days%365)/30) + "月"
	case days >= 30:
		return itoa(days/30) + "月" + itoa(days%30) + "天"
	default:
		return itoa(days) + "天"
	}
}

// formatFloat 格式化浮点数.
func formatFloat(v float64) string {
	if v == float64(int64(v)) {
		return itoa(int(v))
	}
	return fmt.Sprintf("%.1f", v)
}

// itoa int转string.
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
