package zfsenhanced

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// PerformanceMonitor 性能监控器
type PerformanceMonitor struct {
	poolManager     *PoolManager
	metricsHistory  map[string][]PerformanceMetrics
	bottlenecks     []IOBottleneck
	recommendations []PerformanceTuningRecommendation
}

// NewPerformanceMonitor 创建性能监控器
func NewPerformanceMonitor(pm *PoolManager) *PerformanceMonitor {
	return &PerformanceMonitor{
		poolManager:     pm,
		metricsHistory:  make(map[string][]PerformanceMetrics),
		bottlenecks:     make([]IOBottleneck, 0),
		recommendations: make([]PerformanceTuningRecommendation, 0),
	}
}

// CollectMetrics 采集性能指标
func (pm *PerformanceMonitor) CollectMetrics(ctx context.Context, poolName string) (*PerformanceMetrics, error) {
	metrics := &PerformanceMetrics{
		PoolName:  poolName,
		Timestamp: time.Now(),
	}

	// 采集ARC统计
	if err := pm.collectARCStats(ctx, metrics); err != nil {
		return nil, fmt.Errorf("failed to collect ARC stats: %w", err)
	}

	// 采集IO统计
	if err := pm.collectIOStats(ctx, poolName, metrics); err != nil {
		return nil, fmt.Errorf("failed to collect IO stats: %w", err)
	}

	// 采集ZIL统计
	if err := pm.collectZILStats(ctx, poolName, metrics); err != nil {
		return nil, fmt.Errorf("failed to collect ZIL stats: %w", err)
	}

	// 保存历史
	pm.metricsHistory[poolName] = append(pm.metricsHistory[poolName], *metrics)

	// 限制历史记录数
	if len(pm.metricsHistory[poolName]) > 1000 {
		pm.metricsHistory[poolName] = pm.metricsHistory[poolName][len(pm.metricsHistory[poolName])-1000:]
	}

	return metrics, nil
}

// GetMetricsHistory 获取指标历史
func (pm *PerformanceMonitor) GetMetricsHistory(poolName string) []PerformanceMetrics {
	history, exists := pm.metricsHistory[poolName]
	if !exists {
		return nil
	}

	result := make([]PerformanceMetrics, len(history))
	copy(result, history)
	return result
}

// GetARCConfig 获取ARC配置
func (pm *PerformanceMonitor) GetARCConfig(ctx context.Context) (*ARCConfig, error) {
	config := &ARCConfig{}

	cmd := exec.CommandContext(ctx, "cat", "/proc/spl/kstat/zfs/arcstats")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to read arcstats: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		name := fields[0]
		value := fields[2]

		switch name {
		case "c_max":
			config.MaxSizeBytes, _ = strconv.ParseInt(value, 10, 64)
		case "c_min":
			config.MinSizeBytes, _ = strconv.ParseInt(value, 10, 64)
		case "size":
			config.SizeBytes, _ = strconv.ParseInt(value, 10, 64)
		case "c":
			config.TargetBytes, _ = strconv.ParseInt(value, 10, 64)
		case "hits":
			config.Hits, _ = strconv.ParseInt(value, 10, 64)
		case "misses":
			config.Misses, _ = strconv.ParseInt(value, 10, 64)
		case "prefetch_enabled":
			config.PrefetchEnabled = value == "1"
		}
	}

	// 计算命中率
	total := config.Hits + config.Misses
	if total > 0 {
		config.HitRatePercent = float64(config.Hits) / float64(total) * 100
	}

	return config, nil
}

// GetL2ARCConfig 获取L2ARC配置
func (pm *PerformanceMonitor) GetL2ARCConfig(ctx context.Context) (*L2ARCConfig, error) {
	config := &L2ARCConfig{}

	cmd := exec.CommandContext(ctx, "cat", "/proc/spl/kstat/zfs/arcstats")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to read arcstats: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		name := fields[0]
		value := fields[2]

		switch name {
		case "l2_size":
			config.SizeBytes, _ = strconv.ParseInt(value, 10, 64)
		case "l2_write_bytes":
			config.WriteSizeBytes, _ = strconv.ParseInt(value, 10, 64)
		case "l2_hits":
			config.Hits, _ = strconv.ParseInt(value, 10, 64)
		case "l2_misses":
			config.Misses, _ = strconv.ParseInt(value, 10, 64)
		}
	}

	config.Enabled = config.SizeBytes > 0
	total := config.Hits + config.Misses
	if total > 0 {
		config.HitRatePercent = float64(config.Hits) / float64(total) * 100
	}

	return config, nil
}

// GetZILConfig 获取ZIL配置
func (pm *PerformanceMonitor) GetZILConfig(ctx context.Context, poolName string) (*ZILConfig, error) {
	config := &ZILConfig{
		Enabled: true,
	}

	// 获取ZIL统计
	cmd := exec.CommandContext(ctx, "zpool", "iostat", "-vy", poolName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get ZIL stats: %w", err)
	}

	// 解析ZIL统计
	re := regexp.MustCompile(`zil_commit_count\s+(\d+)`)
	if matches := re.FindStringSubmatch(string(output)); len(matches) > 1 {
		config.SyncCount, _ = strconv.ParseInt(matches[1], 10, 64)
	}

	re = regexp.MustCompile(`zil_commit_writer_count\s+(\d+)`)
	if matches := re.FindStringSubmatch(string(output)); len(matches) > 1 {
		config.WriteBytes, _ = strconv.ParseInt(matches[1], 10, 64)
	}

	return config, nil
}

// AnalyzeIOBottlenecks 分析IO瓶颈
func (pm *PerformanceMonitor) AnalyzeIOBottlenecks(ctx context.Context, poolName string) ([]IOBottleneck, error) {
	var bottlenecks []IOBottleneck

	// 获取磁盘统计
	cmd := exec.CommandContext(ctx, "iostat", "-x", "1", "2")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get iostat: %w", err)
	}

	// 解析iostat输出
	lines := strings.Split(string(output), "\n")
	inStats := false
	headerFound := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.Contains(line, "Device") {
			inStats = true
			headerFound = true
			continue
		}

		if inStats && headerFound {
			fields := strings.Fields(line)
			if len(fields) >= 10 {
				device := fields[0]

				// 只分析主磁盘
				if !isMainDisk(device) {
					continue
				}

				utilStr := strings.TrimSuffix(fields[len(fields)-1], "%")
				utilization, _ := strconv.ParseFloat(utilStr, 64)

				await, _ := strconv.ParseFloat(fields[9], 64)

				if utilization > 80 || await > 50 {
					bottleneck := IOBottleneck{
						Device:      device,
						Utilization: utilization,
						AwaitMs:     await,
					}

					// 生成建议
					if utilization > 90 {
						bottleneck.Type = "high_utilization"
						bottleneck.Recommendation = "Consider adding more disks or using faster storage (SSD/NVMe)"
					} else if await > 100 {
						bottleneck.Type = "high_latency"
						bottleneck.Recommendation = "Check disk health, consider replacing slow disks"
					} else {
						bottleneck.Type = "moderate"
						bottleneck.Recommendation = "Monitor for sustained high utilization"
					}

					bottlenecks = append(bottlenecks, bottleneck)
				}
			}
		}
	}

	pm.bottlenecks = bottlenecks
	return bottlenecks, nil
}

// GenerateTuningRecommendations 生成调优建议
func (pm *PerformanceMonitor) GenerateTuningRecommendations(ctx context.Context, poolName string) ([]PerformanceTuningRecommendation, error) {
	var recommendations []PerformanceTuningRecommendation

	// 检查ARC配置
	arcConfig, err := pm.GetARCConfig(ctx)
	if err == nil {
		if arcConfig.HitRatePercent < 90 {
			recommendations = append(recommendations, PerformanceTuningRecommendation{
				Category:    "arc",
				Current:     fmt.Sprintf("Hit rate: %.1f%%", arcConfig.HitRatePercent),
				Recommended: "Increase ARC size (zfs_arc_max) to improve cache hit rate",
				Impact:      "high",
				Description: "Low ARC hit rate indicates insufficient memory for caching. Increasing ARC size can significantly improve read performance.",
				Priority:    1,
			})
		}

		if arcConfig.SizeBytes < arcConfig.MaxSizeBytes/2 {
			recommendations = append(recommendations, PerformanceTuningRecommendation{
				Category:    "arc",
				Current:     fmt.Sprintf("Size: %d bytes", arcConfig.SizeBytes),
				Recommended: "ARC size is well below maximum, consider if system memory is constrained",
				Impact:      "medium",
				Description: "ARC is not using available memory. Check if other processes are consuming memory.",
				Priority:    3,
			})
		}
	}

	// 检查L2ARC配置
	l2arcConfig, err := pm.GetL2ARCConfig(ctx)
	if err == nil {
		if !l2arcConfig.Enabled && arcConfig.HitRatePercent < 95 {
			recommendations = append(recommendations, PerformanceTuningRecommendation{
				Category:    "l2arc",
				Current:     "L2ARC not configured",
				Recommended: "Add SSD as L2ARC to extend read cache",
				Impact:      "high",
				Description: "L2ARC can significantly improve read performance by caching data on fast SSD storage.",
				Priority:    2,
			})
		}

		if l2arcConfig.Enabled && l2arcConfig.HitRatePercent < 50 {
			recommendations = append(recommendations, PerformanceTuningRecommendation{
				Category:    "l2arc",
				Current:     fmt.Sprintf("L2ARC hit rate: %.1f%%", l2arcConfig.HitRatePercent),
				Recommended: "Review L2ARC configuration, consider adjusting l2arc_write_max",
				Impact:      "medium",
				Description: "Low L2ARC hit rate may indicate write throttling or insufficient L2ARC size.",
				Priority:    3,
			})
		}
	}

	// 检查ZIL配置
	zilConfig, err := pm.GetZILConfig(ctx, poolName)
	if err == nil {
		if zilConfig.Devices == nil || len(zilConfig.Devices) == 0 {
			recommendations = append(recommendations, PerformanceTuningRecommendation{
				Category:    "zil",
				Current:     "Using pool for ZIL (no dedicated SLOG)",
				Recommended: "Add dedicated SSD as SLOG for write-intensive workloads",
				Impact:      "high",
				Description: "Dedicated SLOG can significantly reduce write latency for synchronous writes.",
				Priority:    2,
			})
		}
	}

	// 检查压缩配置
	pool, err := pm.poolManager.GetPoolStatus(ctx, poolName)
	if err == nil {
		compression := pool.Properties["compression"]
		if compression == "off" || compression == "" {
			recommendations = append(recommendations, PerformanceTuningRecommendation{
				Category:    "compression",
				Current:     "Compression disabled",
				Recommended: "Enable lz4 compression for good balance of performance and space savings",
				Impact:      "medium",
				Description: "LZ4 compression provides excellent performance with minimal CPU overhead and can reduce storage usage.",
				Priority:    3,
			})
		}
	}

	// 检查记录大小
	if err == nil {
		recordsize := pool.Properties["recordsize"]
		if recordsize != "" {
			// 对于随机IO密集型工作负载，建议较小的记录大小
			// 对于顺序IO密集型工作负载，建议较大的记录大小
			recommendations = append(recommendations, PerformanceTuningRecommendation{
				Category:    "recordsize",
				Current:     fmt.Sprintf("recordsize: %s", recordsize),
				Recommended: "Adjust recordsize based on workload: 128K for general, 1M for large files, 16K-32K for databases",
				Impact:      "medium",
				Description: "Record size affects both performance and space efficiency. Match to your workload characteristics.",
				Priority:    4,
			})
		}
	}

	pm.recommendations = recommendations
	return recommendations, nil
}

// GetCompressionStats 获取压缩统计
func (pm *PerformanceMonitor) GetCompressionStats(ctx context.Context, poolName, dataset string) (*CompressionStats, error) {
	cmd := exec.CommandContext(ctx, "zfs", "get", "-H", "-p", "used,referenced,compressratio,compression", dataset)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get compression stats: %w", err)
	}

	stats := &CompressionStats{
		PoolName: poolName,
		Dataset:  dataset,
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		property := fields[1]
		value := fields[2]

		switch property {
		case "used":
			stats.CompressedBytes, _ = strconv.ParseInt(value, 10, 64)
		case "referenced":
			stats.UncompressedBytes, _ = strconv.ParseInt(value, 10, 64)
		case "compressratio":
			re := regexp.MustCompile(`(\d+\.?\d*)x`)
			if matches := re.FindStringSubmatch(value); len(matches) > 1 {
				stats.CompressRatio, _ = strconv.ParseFloat(matches[1], 64)
			}
		case "compression":
			stats.CompressionType = CompressionType(value)
		}
	}

	// 计算节省的空间
	if stats.CompressRatio > 0 {
		stats.SavedBytes = stats.UncompressedBytes - stats.CompressedBytes
		stats.ReductionPercent = float64(stats.SavedBytes) / float64(stats.UncompressedBytes) * 100
	}

	return stats, nil
}

// GetDedupStats 获取去重统计
func (pm *PerformanceMonitor) GetDedupStats(ctx context.Context, poolName string) (*DedupStats, error) {
	cmd := exec.CommandContext(ctx, "zpool", "get", "-H", "-p", "dedup", poolName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get dedup stats: %w", err)
	}

	stats := &DedupStats{
		PoolName: poolName,
	}

	fields := strings.Fields(strings.TrimSpace(string(output)))
	if len(fields) >= 3 {
		stats.DedupMode = DedupMode(fields[2])
	}

	// 获取去重表信息
	cmd = exec.CommandContext(ctx, "zdb", "-DD", poolName)
	output, err = cmd.CombinedOutput()
	if err == nil {
		// 解析去重表信息
		re := regexp.MustCompile(`DDT entries\s+(\d+)`)
		if matches := re.FindStringSubmatch(string(output)); len(matches) > 1 {
			stats.DedupTableEntries, _ = strconv.ParseInt(matches[1], 10, 64)
		}

		re = regexp.MustCompile(`DDT bytes\s+(\d+)`)
		if matches := re.FindStringSubmatch(string(output)); len(matches) > 1 {
			stats.DedupTableSize, _ = strconv.ParseInt(matches[1], 10, 64)
		}
	}

	return stats, nil
}

// GetCapacityTrend 获取容量趋势
func (pm *PerformanceMonitor) GetCapacityTrend(ctx context.Context, poolName string) (*CapacityTrend, error) {
	pool, err := pm.poolManager.GetPoolStatus(ctx, poolName)
	if err != nil {
		return nil, err
	}

	trend := &CapacityTrend{
		Timestamp:   time.Now(),
		TotalBytes:  pool.SizeBytes,
		UsedBytes:   pool.UsedBytes,
		FreeBytes:   pool.FreeBytes,
		UsedPercent: pool.UsedPercent,
	}

	// 计算增长趋势
	history, exists := pm.poolManager.capacityHistory[poolName]
	if exists && len(history) > 1 {
		last := history[len(history)-1]
		first := history[0]

		if last.Timestamp.After(first.Timestamp) {
			daysDiff := last.Timestamp.Sub(first.Timestamp).Hours() / 24
			if daysDiff > 0 {
				bytesDiff := last.UsedBytes - first.UsedBytes
				trend.GrowthRateDay = float64(bytesDiff) / daysDiff

				if trend.GrowthRateDay > 0 && trend.FreeBytes > 0 {
					trend.DaysUntilFull = int(float64(trend.FreeBytes) / trend.GrowthRateDay)
				}
			}
		}
	}

	// 更新历史记录
	pm.poolManager.capacityHistory[poolName] = append(pm.poolManager.capacityHistory[poolName], *trend)

	// 限制历史记录数
	if len(pm.poolManager.capacityHistory[poolName]) > 1000 {
		pm.poolManager.capacityHistory[poolName] = pm.poolManager.capacityHistory[poolName][len(pm.poolManager.capacityHistory[poolName])-1000:]
	}

	return trend, nil
}

// GetIOBottlenecks 获取IO瓶颈
func (pm *PerformanceMonitor) GetIOBottlenecks() []IOBottleneck {
	result := make([]IOBottleneck, len(pm.bottlenecks))
	copy(result, pm.bottlenecks)
	return result
}

// GetRecommendations 获取调优建议
func (pm *PerformanceMonitor) GetRecommendations() []PerformanceTuningRecommendation {
	result := make([]PerformanceTuningRecommendation, len(pm.recommendations))
	copy(result, pm.recommendations)
	return result
}

// --- 内部方法 ---

func (pm *PerformanceMonitor) collectARCStats(ctx context.Context, metrics *PerformanceMetrics) error {
	cmd := exec.CommandContext(ctx, "cat", "/proc/spl/kstat/zfs/arcstats")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		name := fields[0]
		value := fields[2]

		switch name {
		case "hits":
			metrics.ARCHits, _ = strconv.ParseInt(value, 10, 64)
		case "misses":
			metrics.ARCMisses, _ = strconv.ParseInt(value, 10, 64)
		case "size":
			metrics.ARCSizeBytes, _ = strconv.ParseInt(value, 10, 64)
		case "c":
			metrics.ARCTargetBytes, _ = strconv.ParseInt(value, 10, 64)
		case "l2_hits":
			metrics.L2ARCCacheHits, _ = strconv.ParseInt(value, 10, 64)
		case "l2_misses":
			metrics.L2ARCCacheMisses, _ = strconv.ParseInt(value, 10, 64)
		case "l2_size":
			metrics.L2ARCSizeBytes, _ = strconv.ParseInt(value, 10, 64)
		}
	}

	return nil
}

func (pm *PerformanceMonitor) collectIOStats(ctx context.Context, poolName string, metrics *PerformanceMetrics) error {
	cmd := exec.CommandContext(ctx, "zpool", "iostat", "-vy", poolName, "1", "2")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	// 解析最后一组统计
	lines := strings.Split(string(output), "\n")
	var lastBlock []string
	var currentBlock []string

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if len(currentBlock) > 0 {
				lastBlock = currentBlock
				currentBlock = nil
			}
		} else {
			currentBlock = append(currentBlock, line)
		}
	}

	if len(currentBlock) > 0 {
		lastBlock = currentBlock
	}

	// 解析IO统计
	for _, line := range lastBlock {
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			// 尝试解析read/write IOPS
			if strings.Contains(line, "read") {
				metrics.ReadIOPS, _ = strconv.ParseInt(fields[1], 10, 64)
			}
			if strings.Contains(line, "write") {
				metrics.WriteIOPS, _ = strconv.ParseInt(fields[1], 10, 64)
			}
		}
	}

	return nil
}

func (pm *PerformanceMonitor) collectZILStats(ctx context.Context, poolName string, metrics *PerformanceMetrics) error {
	cmd := exec.CommandContext(ctx, "zpool", "iostat", "-vy", poolName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	// 解析ZIL统计
	re := regexp.MustCompile(`zil_commit_count\s+(\d+)`)
	if matches := re.FindStringSubmatch(string(output)); len(matches) > 1 {
		metrics.ZILSyncCount, _ = strconv.ParseInt(matches[1], 10, 64)
	}

	re = regexp.MustCompile(`zil_commit_writer_count\s+(\d+)`)
	if matches := re.FindStringSubmatch(string(output)); len(matches) > 1 {
		metrics.ZILWriteBytes, _ = strconv.ParseInt(matches[1], 10, 64)
	}

	return nil
}

func isMainDisk(name string) bool {
	prefixes := []string{"sd", "nvme", "mmcblk", "vd"}
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			suffix := name[len(p):]
			if suffix == "" {
				return false
			}
			if p == "mmcblk" {
				return !strings.Contains(suffix, "p")
			}
			if strings.Contains(suffix, "p") && regexp.MustCompile(`p\d+$`).MatchString(suffix) {
				return false
			}
			if p == "sd" || p == "vd" {
				return regexp.MustCompile(`^[a-z]+$`).MatchString(suffix)
			}
			return true
		}
	}
	return false
}
