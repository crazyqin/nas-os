package accesspattern

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// AccessPatternManager 访问模式分析管理器
type AccessPatternManager struct {
	mu       sync.RWMutex
	config   AccessPatternConfig
	records  map[string][]*AccessRecord    // file_path -> 访问记录
	analyses map[string]*PatternAnalysis   // file_path -> 分析结果
	stats    *AccessStats                  // 统计
}

// NewAccessPatternManager 创建管理器
func NewAccessPatternManager(config *AccessPatternConfig) *AccessPatternManager {
	cfg := DefaultAccessPatternConfig()
	if config != nil {
		cfg = *config
	}

	return &AccessPatternManager{
		config:   cfg,
		records:  make(map[string][]*AccessRecord),
		analyses: make(map[string]*PatternAnalysis),
		stats: &AccessStats{
			ByTemperature: make(map[string]int),
			ByAccessMode:  make(map[string]int),
			TopFiles:      make([]FileAccess, 0),
		},
	}
}

// RecordAccess 记录文件访问
func (m *AccessPatternManager) RecordAccess(req *RecordAccessRequest) (*AccessRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.FilePath == "" {
		return nil, fmt.Errorf("文件路径不能为空")
	}

	record := &AccessRecord{
		ID:         fmt.Sprintf("acc_%d", time.Now().UnixNano()),
		FilePath:   req.FilePath,
		FileSize:   req.FileSize,
		AccessTime: time.Now(),
		AccessMode: req.AccessMode,
		UserID:     req.UserID,
		UserAgent:  req.UserAgent,
		ClientIP:   req.ClientIP,
	}

	// 设置默认访问模式
	if record.AccessMode == "" {
		record.AccessMode = "read"
	}

	m.records[req.FilePath] = append(m.records[req.FilePath], record)

	// 更新统计
	m.stats.TotalRecords++
	m.stats.TotalAccesses++
	m.stats.ByAccessMode[record.AccessMode]++

	// 检查是否为新文件
	if len(m.records[req.FilePath]) == 1 {
		m.stats.UniqueFiles++
	}

	return record, nil
}

// AnalyzeFile 分析单个文件的访问模式
func (m *AccessPatternManager) AnalyzeFile(filePath string) (*PatternAnalysis, error) {
	m.mu.RLock()
	records, exists := m.records[filePath]
	m.mu.RUnlock()

	if !exists || len(records) == 0 {
		return nil, fmt.Errorf("没有找到文件的访问记录: %s", filePath)
	}

	analysis := m.analyzeRecords(filePath, records)

	// 缓存分析结果
	m.mu.Lock()
	m.analyses[filePath] = analysis
	m.updateTemperatureStats()
	m.stats.LastAnalyzedAt = &analysis.AnalyzedAt
	m.mu.Unlock()

	return analysis, nil
}

// AnalyzeAll 分析所有文件
func (m *AccessPatternManager) AnalyzeAll() []PatternAnalysis {
	m.mu.RLock()
	files := make([]string, 0, len(m.records))
	for path := range m.records {
		files = append(files, path)
	}
	m.mu.RUnlock()

	results := make([]PatternAnalysis, 0, len(files))
	for _, filePath := range files {
		analysis, err := m.AnalyzeFile(filePath)
		if err != nil {
			continue
		}
		results = append(results, *analysis)
	}

	return results
}

// GetAnalysis 获取文件的分析结果
func (m *AccessPatternManager) GetAnalysis(filePath string) (*PatternAnalysis, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	analysis, exists := m.analyses[filePath]
	if !exists {
		return nil, fmt.Errorf("分析结果不存在: %s", filePath)
	}

	return analysis, nil
}

// GenerateHeatMap 生成热力图
func (m *AccessPatternManager) GenerateHeatMap(startTime, endTime time.Time, limit int) *HeatMap {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]HeatMapEntry, 0)

	for filePath, records := range m.records {
		// 过滤时间范围内的记录
		count := 0
		for _, record := range records {
			if record.AccessTime.After(startTime) && record.AccessTime.Before(endTime) {
				count++
			}
		}

		if count == 0 {
			continue
		}

		// 获取分析结果
		analysis, exists := m.analyses[filePath]
		if !exists {
			// 快速计算热度
			analysis = m.quickAnalyze(filePath, records)
		}

		entry := HeatMapEntry{
			FilePath:    filePath,
			HeatScore:   analysis.HeatScore,
			Temperature: analysis.Temperature,
			AccessCount: count,
			Size:        analysis.FileSize,
		}

		entries = append(entries, entry)
	}

	// 按热度排序
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].HeatScore > entries[j].HeatScore
	})

	// 限制数量
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	// 计算汇总
	summary := m.calculateHeatMapSummary(entries)

	return &HeatMap{
		GeneratedAt: time.Now(),
		TimeRange: TimeRange{
			Start: startTime,
			End:   endTime,
		},
		Entries: entries,
		Summary: summary,
	}
}

// GetStats 获取统计信息
func (m *AccessPatternManager) GetStats() *AccessStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 计算 Top Files
	stats := *m.stats
	stats.TopFiles = m.calculateTopFiles(10)

	return &stats
}

// GenerateTieringReport 生成分层报告
func (m *AccessPatternManager) GenerateTieringReport() *TieringReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	suggestions := make([]TieringSuggestion, 0)

	for filePath, analysis := range m.analyses {
		if analysis.SuggestedTier != "" {
			suggestion := TieringSuggestion{
				FilePath:      filePath,
				SuggestedTier: analysis.SuggestedTier,
				Temperature:   analysis.Temperature,
				HeatScore:     analysis.HeatScore,
				Reason:        m.generateTieringReason(analysis),
				Priority:      m.calculatePriority(analysis),
			}
			suggestions = append(suggestions, suggestion)
		}
	}

	// 按优先级排序
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Priority > suggestions[j].Priority
	})

	// 计算汇总
	summary := TieringSummary{
		TotalFiles:    len(m.analyses),
		NeedMigration: len(suggestions),
	}

	for _, s := range suggestions {
		if s.SuggestedTier == "cold" && s.Temperature == TemperatureHot {
			summary.PotentialSavings += m.analyses[s.FilePath].FileSize
		}
	}

	return &TieringReport{
		GeneratedAt: time.Now(),
		Suggestions: suggestions,
		Summary:     summary,
	}
}

// Cleanup 清理过期记录
func (m *AccessPatternManager) Cleanup() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -m.config.RetentionDays)
	removed := 0

	for filePath, records := range m.records {
		filtered := make([]*AccessRecord, 0, len(records))
		for _, record := range records {
			if record.AccessTime.After(cutoff) {
				filtered = append(filtered, record)
			} else {
				removed++
			}
		}

		if len(filtered) == 0 {
			delete(m.records, filePath)
			delete(m.analyses, filePath)
		} else {
			m.records[filePath] = filtered
		}
	}

	m.stats.TotalRecords -= removed
	return removed
}

// ============================================================
// 内部分析方法
// ============================================================

// analyzeRecords 分析访问记录
func (m *AccessPatternManager) analyzeRecords(filePath string, records []*AccessRecord) *PatternAnalysis {
	if len(records) == 0 {
		return nil
	}

	// 按时间排序
	sort.Slice(records, func(i, j int) bool {
		return records[i].AccessTime.Before(records[j].AccessTime)
	})

	analysis := &PatternAnalysis{
		FilePath:      filePath,
		TotalAccesses: len(records),
		FirstAccess:   records[0].AccessTime,
		LastAccess:    records[len(records)-1].AccessTime,
		AnalyzedAt:    time.Now(),
	}

	// 文件大小（取最新的记录）
	analysis.FileSize = records[len(records)-1].FileSize

	// 计算访问间隔
	analysis.AccessInterval = m.calculateAccessInterval(records)

	// 计算热度评分
	analysis.HeatScore = m.calculateHeatScore(records, analysis)

	// 确定温度
	analysis.Temperature = m.determineTemperature(analysis)

	// 分析访问模式
	analysis.AccessPattern = m.analyzeAccessPattern(records)

	// 分析时间分布
	analysis.AccessHours = m.analyzeAccessHours(records)
	analysis.AccessDays = m.analyzeAccessDays(records)

	// 计算读写比
	analysis.ReadWriteRatio = m.calculateReadWriteRatio(records)

	// 建议存储层级
	analysis.SuggestedTier = m.suggestTier(analysis)

	return analysis
}

// calculateAccessInterval 计算平均访问间隔
func (m *AccessPatternManager) calculateAccessInterval(records []*AccessRecord) time.Duration {
	if len(records) < 2 {
		return 0
	}

	var totalDuration time.Duration
	for i := 1; i < len(records); i++ {
		totalDuration += records[i].AccessTime.Sub(records[i-1].AccessTime)
	}

	return totalDuration / time.Duration(len(records)-1)
}

// calculateHeatScore 计算热度评分 (0-100)
func (m *AccessPatternManager) calculateHeatScore(records []*AccessRecord, analysis *PatternAnalysis) float64 {
	if len(records) == 0 {
		return 0
	}

	score := 0.0

	// 1. 访问频率得分 (40%)
	freqScore := m.calculateFrequencyScore(records)
	score += freqScore * 0.4

	// 2. 时间衰减得分 (30%)
	recencyScore := m.calculateRecencyScore(records)
	score += recencyScore * 0.3

	// 3. 访问规律性得分 (20%)
	regularityScore := m.calculateRegularityScore(records)
	score += regularityScore * 0.2

	// 4. 近期活跃度得分 (10%)
	recentActivityScore := m.calculateRecentActivityScore(records)
	score += recentActivityScore * 0.1

	return math.Min(100, math.Max(0, score))
}

// calculateFrequencyScore 计算访问频率得分
func (m *AccessPatternManager) calculateFrequencyScore(records []*AccessRecord) float64 {
	// 计算每天平均访问次数
	if len(records) < 2 {
		return 20
	}

	duration := records[len(records)-1].AccessTime.Sub(records[0].AccessTime)
	days := duration.Hours() / 24
	if days < 1 {
		days = 1
	}

	avgPerDay := float64(len(records)) / days

	// 评分：每天1次=50分，每天5次=80分，每天10次=100分
	switch {
	case avgPerDay >= 10:
		return 100
	case avgPerDay >= 5:
		return 80 + (avgPerDay-5)*4
	case avgPerDay >= 1:
		return 50 + (avgPerDay-1)*7.5
	case avgPerDay >= 0.1:
		return 20 + (avgPerDay-0.1)*33
	default:
		return 10
	}
}

// calculateRecencyScore 计算时间衰减得分
func (m *AccessPatternManager) calculateRecencyScore(records []*AccessRecord) float64 {
	if len(records) == 0 {
		return 0
	}

	lastAccess := records[len(records)-1].AccessTime
	hoursSinceLastAccess := time.Since(lastAccess).Hours()

	// 最近24小时内访问=100分，每过一天减5分
	switch {
	case hoursSinceLastAccess < 24:
		return 100
	case hoursSinceLastAccess < 168: // 1周
		return 80
	case hoursSinceLastAccess < 720: // 1月
		return 50
	case hoursSinceLastAccess < 2160: // 3月
		return 30
	default:
		return 10
	}
}

// calculateRegularityScore 计算访问规律性得分
func (m *AccessPatternManager) calculateRegularityScore(records []*AccessRecord) float64 {
	if len(records) < 3 {
		return 30
	}

	// 计算访问间隔的标准差
	intervals := make([]float64, 0, len(records)-1)
	for i := 1; i < len(records); i++ {
		intervals = append(intervals, records[i].AccessTime.Sub(records[i-1].AccessTime).Hours())
	}

	mean := 0.0
	for _, v := range intervals {
		mean += v
	}
	mean /= float64(len(intervals))

	variance := 0.0
	for _, v := range intervals {
		variance += (v - mean) * (v - mean)
	}
	variance /= float64(len(intervals))

	stddev := math.Sqrt(variance)

	// 标准差越小，规律性越高
	// stddev < 24h = 90分，< 72h = 60分，< 168h = 40分
	switch {
	case stddev < 24:
		return 90
	case stddev < 72:
		return 60
	case stddev < 168:
		return 40
	default:
		return 20
	}
}

// calculateRecentActivityScore 计算近期活跃度得分
func (m *AccessPatternManager) calculateRecentActivityScore(records []*AccessRecord) float64 {
	// 统计最近7天的访问次数
	recentCount := 0
	cutoff := time.Now().AddDate(0, 0, -7)

	for _, record := range records {
		if record.AccessTime.After(cutoff) {
			recentCount++
		}
	}

	switch {
	case recentCount >= 20:
		return 100
	case recentCount >= 10:
		return 80
	case recentCount >= 5:
		return 60
	case recentCount >= 1:
		return 40
	default:
		return 10
	}
}

// determineTemperature 确定数据温度
func (m *AccessPatternManager) determineTemperature(analysis *PatternAnalysis) DataTemperature {
	daysSinceLastAccess := time.Since(analysis.LastAccess).Hours() / 24

	switch {
	case daysSinceLastAccess <= float64(m.config.HotThresholdDays) &&
		analysis.TotalAccesses >= m.config.HotAccessCount:
		return TemperatureHot
	case daysSinceLastAccess <= float64(m.config.WarmThresholdDays) &&
		analysis.TotalAccesses >= m.config.WarmAccessCount:
		return TemperatureWarm
	default:
		return TemperatureCold
	}
}

// analyzeAccessPattern 分析访问模式
func (m *AccessPatternManager) analyzeAccessPattern(records []*AccessRecord) string {
	if len(records) < 3 {
		return "unknown"
	}

	// 分析访问时间间隔的分布
	intervals := make([]float64, 0, len(records)-1)
	for i := 1; i < len(records); i++ {
		intervals = append(intervals, records[i].AccessTime.Sub(records[i-1].AccessTime).Seconds())
	}

	// 计算变异系数
	mean := 0.0
	for _, v := range intervals {
		mean += v
	}
	mean /= float64(len(intervals))

	if mean == 0 {
		return "burst"
	}

	variance := 0.0
	for _, v := range intervals {
		variance += (v - mean) * (v - mean)
	}
	variance /= float64(len(intervals))

	cv := math.Sqrt(variance) / mean

	// CV < 0.5 = 顺序访问，CV > 2 = 随机访问，否则为突发访问
	switch {
	case cv < 0.5:
		return "sequential"
	case cv > 2:
		return "random"
	default:
		return "burst"
	}
}

// analyzeAccessHours 分析访问时间分布（24小时）
func (m *AccessPatternManager) analyzeAccessHours(records []*AccessRecord) []int {
	hours := make([]int, 24)
	for _, record := range records {
		hours[record.AccessTime.Hour()]++
	}
	return hours
}

// analyzeAccessDays 分析访问日期分布（7天）
func (m *AccessPatternManager) analyzeAccessDays(records []*AccessRecord) []int {
	days := make([]int, 7)
	for _, record := range records {
		days[int(record.AccessTime.Weekday())]++
	}
	return days
}

// calculateReadWriteRatio 计算读写比
func (m *AccessPatternManager) calculateReadWriteRatio(records []*AccessRecord) float64 {
	readCount := 0
	writeCount := 0

	for _, record := range records {
		if record.AccessMode == "read" {
			readCount++
		} else if record.AccessMode == "write" {
			writeCount++
		}
	}

	if writeCount == 0 {
		return float64(readCount)
	}

	return float64(readCount) / float64(writeCount)
}

// suggestTier 建议存储层级
func (m *AccessPatternManager) suggestTier(analysis *PatternAnalysis) string {
	switch analysis.Temperature {
	case TemperatureHot:
		return "ssd"
	case TemperatureWarm:
		return "hdd"
	case TemperatureCold:
		return "archive"
	default:
		return "auto"
	}
}

// quickAnalyze 快速分析（用于热力图）
func (m *AccessPatternManager) quickAnalyze(filePath string, records []*AccessRecord) *PatternAnalysis {
	return &PatternAnalysis{
		FilePath:      filePath,
		TotalAccesses: len(records),
		HeatScore:     float64(len(records)) * 10,
		Temperature:   TemperatureWarm,
		FileSize:      records[len(records)-1].FileSize,
	}
}

// calculateHeatMapSummary 计算热力图汇总
func (m *AccessPatternManager) calculateHeatMapSummary(entries []HeatMapEntry) HeatMapSummary {
	summary := HeatMapSummary{
		TotalFiles: len(entries),
	}

	for _, entry := range entries {
		summary.TotalSize += entry.Size

		switch entry.Temperature {
		case TemperatureHot:
			summary.HotFiles++
			summary.HotSize += entry.Size
		case TemperatureWarm:
			summary.WarmFiles++
			summary.WarmSize += entry.Size
		case TemperatureCold:
			summary.ColdFiles++
			summary.ColdSize += entry.Size
		}

		summary.AvgHeatScore += entry.HeatScore
	}

	if len(entries) > 0 {
		summary.AvgHeatScore /= float64(len(entries))
	}

	return summary
}

// calculateTopFiles 计算热门文件
func (m *AccessPatternManager) calculateTopFiles(limit int) []FileAccess {
	type fileStats struct {
		path        string
		count       int
		size        int64
		lastAccess  time.Time
	}

	stats := make(map[string]*fileStats)
	for filePath, records := range m.records {
		if _, exists := stats[filePath]; !exists {
			stats[filePath] = &fileStats{
				path: filePath,
			}
		}

		stats[filePath].count += len(records)
		if len(records) > 0 {
			stats[filePath].size = records[len(records)-1].FileSize
			stats[filePath].lastAccess = records[len(records)-1].AccessTime
		}
	}

	// 转换为切片并排序
	fileAccesses := make([]FileAccess, 0, len(stats))
	for _, s := range stats {
		fileAccesses = append(fileAccesses, FileAccess{
			FilePath:     s.path,
			AccessCount:  s.count,
			TotalSize:    s.size,
			LastAccessAt: s.lastAccess,
		})
	}

	sort.Slice(fileAccesses, func(i, j int) bool {
		return fileAccesses[i].AccessCount > fileAccesses[j].AccessCount
	})

	if len(fileAccesses) > limit {
		fileAccesses = fileAccesses[:limit]
	}

	return fileAccesses
}

// generateTieringReason 生成分层建议原因
func (m *AccessPatternManager) generateTieringReason(analysis *PatternAnalysis) string {
	switch analysis.Temperature {
	case TemperatureHot:
		return "数据热度高，建议存储在高速SSD"
	case TemperatureWarm:
		return "数据热度中等，建议存储在普通HDD"
	case TemperatureCold:
		return "数据热度低，建议归档存储"
	default:
		return "无法确定建议"
	}
}

// calculatePriority 计算建议优先级
func (m *AccessPatternManager) calculatePriority(analysis *PatternAnalysis) int {
	// 热度越高，优先级越高
	switch {
	case analysis.HeatScore >= 80:
		return 9
	case analysis.HeatScore >= 60:
		return 7
	case analysis.HeatScore >= 40:
		return 5
	case analysis.HeatScore >= 20:
		return 3
	default:
		return 1
	}
}

// updateTemperatureStats 更新温度统计
func (m *AccessPatternManager) updateTemperatureStats() {
	m.stats.ByTemperature = map[string]int{
		"hot":  0,
		"warm": 0,
		"cold": 0,
	}

	for _, analysis := range m.analyses {
		m.stats.ByTemperature[string(analysis.Temperature)]++
	}
}
