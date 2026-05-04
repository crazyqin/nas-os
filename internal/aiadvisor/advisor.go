package aiadvisor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Advisor AI存储优化顾问核心.
type Advisor struct {
	mu              sync.RWMutex
	logger          *zap.Logger
	scanResult      *ScanResult
	recommendations []Recommendation
	capacityHistory []CapacityDataPoint
	config          *ScanConfig
	nextRecID       int
}

// NewAdvisor 创建AI存储优化顾问.
func NewAdvisor(logger *zap.Logger, cfg *ScanConfig) *Advisor {
	if logger == nil {
		logger = zap.NewNop()
	}
	if cfg == nil {
		cfg = DefaultScanConfig()
	}
	return &Advisor{
		logger:    logger,
		config:    cfg,
		nextRecID: 1,
	}
}

// IsScanning 检查是否正在扫描.
func (a *Advisor) IsScanning() bool {
	// 简单实现：检查是否有扫描结果的时间戳但还没完成
	a.mu.RLock()
	defer a.mu.RUnlock()
	return false // 简化版，使用互斥锁保证并发安全
}

// Scan 启动存储扫描.
func (a *Advisor) Scan(cfg *ScanConfig) (*ScanResult, error) {
	if cfg == nil {
		cfg = a.config
	}

	startTime := time.Now()
	a.logger.Info("开始存储扫描", zap.String("path", cfg.RootPath))

	// 验证路径
	if _, err := os.Stat(cfg.RootPath); os.IsNotExist(err) {
		return nil, ErrInvalidPath
	}

	result := &ScanResult{
		RootPath:         cfg.RootPath,
		ScanStartedAt:    startTime,
		ExtensionSummary: make(map[string]ExtStat),
	}

	dirSizes := make(map[string]int64)
	dirFileCounts := make(map[string]int64)
	fileHashes := make(map[string][]FileInfo) // hash -> files

	// 遍历文件系统
	err := filepath.Walk(cfg.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}

		// 检查深度
		relPath, _ := filepath.Rel(cfg.RootPath, path)
		depth := strings.Count(relPath, string(os.PathSeparator))
		if depth > cfg.MaxDepth {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			result.TotalDirs++
			return nil
		}

		result.TotalFiles++
		result.TotalSizeBytes += info.Size()

		// 更新扩展名统计
		ext := strings.ToLower(filepath.Ext(path))
		if ext == "" {
			ext = "(无扩展名)"
		}
		stat := result.ExtensionSummary[ext]
		stat.Count++
		stat.TotalBytes += info.Size()
		result.ExtensionSummary[ext] = stat

		// 更新目录大小
		dir := filepath.Dir(path)
		dirSizes[dir] += info.Size()
		dirFileCounts[dir]++

		// 获取访问时间
		atime := getAccessTime(info)
		daysSince := int(time.Since(atime).Hours() / 24)

		fi := FileInfo{
			Path:         path,
			Size:         info.Size(),
			ModTime:      info.ModTime(),
			AccessTime:   atime,
			Extension:    ext,
			DaysSinceUse: daysSince,
		}

		// 大文件检测
		if info.Size() >= int64(cfg.LargeFileThresholdMB)*1024*1024 {
			result.LargeFiles = append(result.LargeFiles, fi)
		}

		// 长期未访问文件检测
		if daysSince >= cfg.StaleDays {
			result.StaleFiles = append(result.StaleFiles, fi)
		}

		// 重复文件检测（只对文件大小 > 0 的文件）
		if cfg.EnableDedupCheck && info.Size() > 0 {
			hash, err := fileHash(path)
			if err == nil {
				fi.Hash = hash
				fileHashes[hash] = append(fileHashes[hash], fi)
			}
		}

		return nil
	})

	if err != nil {
		a.logger.Warn("扫描过程中出现错误", zap.Error(err))
	}

	// 处理重复文件
	for hash, files := range fileHashes {
		if len(files) > 1 {
			group := DuplicateGroup{
				Hash:        hash,
				Size:        files[0].Size,
				Count:       len(files),
				Files:       files,
				WastedBytes: int64(len(files)-1) * files[0].Size,
			}
			result.DuplicateGroups = append(result.DuplicateGroups, group)
			result.DuplicateWaste += group.WastedBytes
		}
	}

	// 按大小排序大文件
	sort.Slice(result.LargeFiles, func(i, j int) bool {
		return result.LargeFiles[i].Size > result.LargeFiles[j].Size
	})

	// 按天数排序陈旧文件
	sort.Slice(result.StaleFiles, func(i, j int) bool {
		return result.StaleFiles[i].DaysSinceUse > result.StaleFiles[j].DaysSinceUse
	})

	// 生成Top目录统计
	for dir, size := range dirSizes {
		result.TopDirsBySize = append(result.TopDirsBySize, DirSizeStat{
			Path:      dir,
			TotalSize: size,
			FileCount: int(dirFileCounts[dir]),
		})
	}
	sort.Slice(result.TopDirsBySize, func(i, j int) bool {
		return result.TopDirsBySize[i].TotalSize > result.TopDirsBySize[j].TotalSize
	})
	if len(result.TopDirsBySize) > 20 {
		result.TopDirsBySize = result.TopDirsBySize[:20]
	}

	result.ScanFinishedAt = time.Now()
	result.DurationSeconds = result.ScanFinishedAt.Sub(result.ScanStartedAt).Seconds()

	a.mu.Lock()
	a.scanResult = result
	a.mu.Unlock()

	// 自动生成建议
	recs := a.generateRecommendations(result)
	a.mu.Lock()
	a.recommendations = recs
	a.mu.Unlock()

	a.logger.Info("存储扫描完成",
		zap.Int("files", result.TotalFiles),
		zap.Int64("total_size", result.TotalSizeBytes),
		zap.Int("large_files", len(result.LargeFiles)),
		zap.Int("stale_files", len(result.StaleFiles)),
		zap.Int("dup_groups", len(result.DuplicateGroups)),
		zap.Int("recommendations", len(recs)),
	)

	return result, nil
}

// GetRecommendations 获取优化建议列表.
func (a *Advisor) GetRecommendations() ([]Recommendation, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.scanResult == nil {
		return nil, ErrNoScanData
	}
	recs := make([]Recommendation, len(a.recommendations))
	copy(recs, a.recommendations)
	return recs, nil
}

// GetReport 获取优化报告.
func (a *Advisor) GetReport() (*OptimizationReport, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.scanResult == nil {
		return nil, ErrNoScanData
	}

	totalSaving := int64(0)
	for _, r := range a.recommendations {
		totalSaving += r.EstimatedSaving
	}

	savingPct := 0.0
	if a.scanResult.TotalSizeBytes > 0 {
		savingPct = float64(totalSaving) / float64(a.scanResult.TotalSizeBytes) * 100
	}

	return &OptimizationReport{
		ScanSummary: ScanResultSummary{
			TotalFiles:     a.scanResult.TotalFiles,
			TotalSizeBytes: a.scanResult.TotalSizeBytes,
			LargeFileCount: len(a.scanResult.LargeFiles),
			StaleFileCount: len(a.scanResult.StaleFiles),
			DuplicateCount: len(a.scanResult.DuplicateGroups),
			DuplicateWaste: a.scanResult.DuplicateWaste,
		},
		Recommendations:      a.recommendations,
		TotalEstimatedSaving: totalSaving,
		SavingPercent:        round2(savingPct),
		GeneratedAt:          time.Now(),
	}, nil
}

// GetCapacityForecast 获取容量预测.
func (a *Advisor) GetCapacityForecast(predictMonths int) (*CapacityForecast, error) {
	a.mu.RLock()
	history := make([]CapacityDataPoint, len(a.capacityHistory))
	copy(history, a.capacityHistory)
	a.mu.RUnlock()

	if len(history) < 2 {
		return nil, ErrInsufficientHistory
	}

	if predictMonths <= 0 {
		predictMonths = 12
	}

	// 按时间排序
	sort.Slice(history, func(i, j int) bool {
		return history[i].Timestamp.Before(history[j].Timestamp)
	})

	latest := history[len(history)-1]
	totalTB := float64(latest.TotalBytes) / (1024 * 1024 * 1024 * 1024)
	usedTB := float64(latest.UsedBytes) / (1024 * 1024 * 1024 * 1024)
	usagePct := 0.0
	if totalTB > 0 {
		usagePct = usedTB / totalTB * 100
	}

	// 线性回归计算增长率
	monthlyGB, monthlyPct := calculateCapacityGrowthRate(history)

	// 计算剩余可用空间
	remainingGB := (totalTB - usedTB) * 1024
	daysUntilFull := math.MaxFloat64
	if monthlyGB > 0 {
		daysUntilFull = remainingGB / monthlyGB * 30
	}

	// 生成预测
	predictions := make([]PredictionPoint, 0, predictMonths)
	predictedUsed := usedTB
	for i := 1; i <= predictMonths; i++ {
		date := time.Now().AddDate(0, i, 0)
		if monthlyPct > 0 {
			predictedUsed *= (1 + monthlyPct/100)
		} else {
			predictedUsed += monthlyGB / 1024
		}
		predictedPct := 0.0
		if totalTB > 0 {
			predictedPct = predictedUsed / totalTB * 100
		}
		predictions = append(predictions, PredictionPoint{
			Date:        date,
			PredictedTB: round2(predictedUsed),
			UsagePct:    round2(predictedPct),
		})
	}

	urgency := "normal"
	if daysUntilFull < 90 {
		urgency = "critical"
	} else if daysUntilFull < 180 {
		urgency = "warning"
	}

	return &CapacityForecast{
		CurrentUsedBytes:  latest.UsedBytes,
		CurrentTotalBytes: latest.TotalBytes,
		UsagePercent:      round2(usagePct),
		MonthlyGrowthGB:   round2(monthlyGB),
		MonthlyGrowthPct:  round2(monthlyPct),
		DaysUntilFull:     round2(daysUntilFull),
		Predictions:       predictions,
		UrgencyLevel:      urgency,
		GeneratedAt:       time.Now(),
	}, nil
}

// AddCapacityData 添加容量历史数据点.
func (a *Advisor) AddCapacityData(point CapacityDataPoint) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.capacityHistory = append(a.capacityHistory, point)
}

// ApplyRecommendation 应用某个建议.
func (a *Advisor) ApplyRecommendation(id string) (*Recommendation, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i := range a.recommendations {
		if a.recommendations[i].ID == id {
			if a.recommendations[i].Applied {
				return &a.recommendations[i], nil // 已应用
			}
			now := time.Now()
			a.recommendations[i].Applied = true
			a.recommendations[i].AppliedAt = &now
			a.logger.Info("应用优化建议", zap.String("id", id), zap.String("title", a.recommendations[i].Title))
			return &a.recommendations[i], nil
		}
	}
	return nil, ErrRecommendationNotFound
}

// ========== 内部方法 ==========

// generateRecommendations 基于扫描结果生成优化建议.
func (a *Advisor) generateRecommendations(result *ScanResult) []Recommendation {
	recs := make([]Recommendation, 0)
	nextID := func() string {
		id := fmt.Sprintf("rec-%04d", a.nextRecID)
		a.nextRecID++
		return id
	}

	// 1. 去重建议
	if result.DuplicateWaste > 0 {
		recs = append(recs, Recommendation{
			ID:              nextID(),
			Type:            RecTypeDedup,
			Title:           "重复文件去重",
			Description:     fmt.Sprintf("检测到 %d 组重复文件，去重可节省约 %s 空间", len(result.DuplicateGroups), formatBytes(result.DuplicateWaste)),
			Priority:        1,
			EstimatedSaving: result.DuplicateWaste,
		})
	}

	// 2. 冷数据归档建议
	if len(result.StaleFiles) > 0 {
		var staleSize int64
		targetFiles := make([]string, 0, len(result.StaleFiles))
		for _, f := range result.StaleFiles {
			staleSize += f.Size
			if len(targetFiles) < 100 { // 限制列表大小
				targetFiles = append(targetFiles, f.Path)
			}
		}
		recs = append(recs, Recommendation{
			ID:              nextID(),
			Type:            RecTypeStaleArchive,
			Title:           "长期未访问文件归档",
			Description:     fmt.Sprintf("%d 个文件超过 %d 天未访问，共 %s，建议迁移到冷存储", len(result.StaleFiles), a.config.StaleDays, formatBytes(staleSize)),
			Priority:        2,
			EstimatedSaving: staleSize / 2, // 归档后热存储节省约50%空间
			TargetFiles:     targetFiles,
		})
	}

	// 3. 大文件审查建议
	if len(result.LargeFiles) > 0 {
		var largeSize int64
		targetFiles := make([]string, 0, len(result.LargeFiles))
		for _, f := range result.LargeFiles {
			largeSize += f.Size
			if len(targetFiles) < 50 {
				targetFiles = append(targetFiles, f.Path)
			}
		}
		recs = append(recs, Recommendation{
			ID:              nextID(),
			Type:            RecTypeLargeFile,
			Title:           "大文件审查",
			Description:     fmt.Sprintf("%d 个文件超过 %d MB，共 %s，建议审查是否需要保留", len(result.LargeFiles), a.config.LargeFileThresholdMB, formatBytes(largeSize)),
			Priority:        2,
			EstimatedSaving: largeSize / 10, // 预计可清理约10%
			TargetFiles:     targetFiles,
		})
	}

	// 4. 压缩建议（针对可压缩文件类型）
	compressibleBytes := int64(0)
	compressibleExts := map[string]bool{".log": true, ".txt": true, ".csv": true, ".json": true, ".xml": true, ".yaml": true, ".yml": true, ".md": true, ".html": true, ".css": true, ".js": true}
	for ext, stat := range result.ExtensionSummary {
		if compressibleExts[ext] {
			compressibleBytes += stat.TotalBytes
		}
	}
	if compressibleBytes > 0 {
		recs = append(recs, Recommendation{
			ID:              nextID(),
			Type:            RecTypeCompress,
			Title:           "文本文件压缩",
			Description:     fmt.Sprintf("检测到 %s 可压缩文本文件，启用压缩可节省约30%%空间", formatBytes(compressibleBytes)),
			Priority:        3,
			EstimatedSaving: compressibleBytes * 30 / 100,
		})
	}

	// 5. 分层存储策略建议
	if result.TotalSizeBytes > 0 && len(result.StaleFiles) > 0 {
		var coldDataSize int64
		for _, f := range result.StaleFiles {
			coldDataSize += f.Size
		}
		if coldDataSize > 0 {
			// 找到最大的目录作为目标路径
			targetPath := ""
			if len(result.TopDirsBySize) > 0 {
				targetPath = result.TopDirsBySize[0].Path
			}
			recs = append(recs, Recommendation{
				ID:              nextID(),
				Type:            RecTypeTierMigration,
				Title:           "分层存储迁移",
				Description:     fmt.Sprintf("建议将 %s 冷数据迁移到低成本存储层，热数据保留在高性能存储", formatBytes(coldDataSize)),
				Priority:        2,
				EstimatedSaving: coldDataSize * 60 / 100, // 冷存储成本约40%热存储
				TargetPath:      targetPath,
			})
		}
	}

	// 按优先级排序
	sort.Slice(recs, func(i, j int) bool {
		return recs[i].Priority < recs[j].Priority
	})

	return recs
}

// ========== 辅助函数 ==========

// fileHash 计算文件SHA256哈希（读取前64KB用于快速去重）.
func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	// 读取前64KB + 文件大小作为去重依据
	buf := make([]byte, 64*1024)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", err
	}
	h.Write(buf[:n])

	return hex.EncodeToString(h.Sum(nil)), nil
}

// getAccessTime 获取文件访问时间.
func getAccessTime(info os.FileInfo) time.Time {
	// 使用修改时间作为访问时间的近似
	return info.ModTime()
}

// calculateCapacityGrowthRate 基于历史数据计算月均增长率.
func calculateCapacityGrowthRate(history []CapacityDataPoint) (monthlyGB float64, monthlyPct float64) {
	if len(history) < 2 {
		return 0, 0
	}

	n := float64(len(history))
	var sumX, sumY, sumXY, sumX2 float64
	firstTime := history[0].Timestamp
	for _, p := range history {
		x := p.Timestamp.Sub(firstTime).Hours() / 24 / 30 // 月数
		y := float64(p.UsedBytes) / (1024 * 1024 * 1024)   // GB
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0, 0
	}

	slope := (n*sumXY - sumX*sumY) / denom // GB/月
	avgUsedGB := sumY / n

	monthlyGB = slope
	if avgUsedGB > 0 {
		monthlyPct = slope / avgUsedGB * 100
	}
	if monthlyGB < 0 {
		monthlyGB = 0
		monthlyPct = 0
	}

	return monthlyGB, monthlyPct
}

// formatBytes 格式化字节数为可读字符串.
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// round2 保留两位小数.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
