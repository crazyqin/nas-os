// Package predictivecache 提供预测性缓存预热功能.
// 基于访问模式预测数据需求，提前加载到缓存中.
package predictivecache

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AccessPattern 访问模式类型
type AccessPattern string

const (
	PatternSequential AccessPattern = "sequential"
	PatternRandom     AccessPattern = "random"
	PatternPeriodic   AccessPattern = "periodic"
	PatternHotspot    AccessPattern = "hotspot"
	PatternSeasonal   AccessPattern = "seasonal"
)

// CacheLevel 缓存层级
type CacheLevel string

const (
	CacheL1 CacheLevel = "l1" // 内存缓存
	CacheL2 CacheLevel = "l2" // SSD缓存
	CacheL3 CacheLevel = "l3" // HDD缓存
)

// PredictionConfidence 预测置信度
type PredictionConfidence string

const (
	ConfidenceHigh   PredictionConfidence = "high"
	ConfidenceMedium PredictionConfidence = "medium"
	ConfidenceLow    PredictionConfidence = "low"
)

// FileAccessRecord 文件访问记录
type FileAccessRecord struct {
	FilePath   string    `json:"file_path"`
	AccessTime time.Time `json:"access_time"`
	UserID     string    `json:"user_id"`
	Operation  string    `json:"operation"` // read, write, delete
	SizeBytes  int64     `json:"size_bytes"`
	Duration   int       `json:"duration_ms"`
}

// AccessPatternAnalysis 访问模式分析
type AccessPatternAnalysis struct {
	FilePath      string               `json:"file_path"`
	Pattern       AccessPattern        `json:"pattern"`
	Frequency     float64              `json:"frequency"` // 每天访问次数
	LastAccess    time.Time            `json:"last_access"`
	NextPredicted time.Time            `json:"next_predicted"`
	Confidence    PredictionConfidence `json:"confidence"`
	TrendScore    float64              `json:"trend_score"` // 趋势分数 (0-1)
	Seasonality   float64              `json:"seasonality"` // 季节性因子
}

// CacheEntry 缓存条目
type CacheEntry struct {
	ID         string     `json:"id"`
	FilePath   string     `json:"file_path"`
	CacheLevel CacheLevel `json:"cache_level"`
	SizeBytes  int64      `json:"size_bytes"`
	Priority   int        `json:"priority"` // 优先级 (1-10)
	HitCount   int        `json:"hit_count"`
	MissCount  int        `json:"miss_count"`
	HitRate    float64    `json:"hit_rate"`
	LoadedAt   time.Time  `json:"loaded_at"`
	LastAccess time.Time  `json:"last_access"`
	ExpiresAt  time.Time  `json:"expires_at"`
	Pinned     bool       `json:"pinned"` // 是否固定在缓存中
}

// WarmingTask 预热任务
type WarmingTask struct {
	ID          string     `json:"id"`
	FilePath    string     `json:"file_path"`
	CacheLevel  CacheLevel `json:"cache_level"`
	Priority    int        `json:"priority"`
	Status      string     `json:"status"` // pending, warming, completed, failed
	Progress    float64    `json:"progress"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Error       string     `json:"error,omitempty"`
}

// PredictionModel 预测模型配置
type PredictionModel struct {
	WindowSize          int     `json:"window_size"`          // 分析窗口大小（天）
	MinSamples          int     `json:"min_samples"`          // 最小样本数
	ConfidenceThreshold float64 `json:"confidence_threshold"` // 置信度阈值
	DecayFactor         float64 `json:"decay_factor"`         // 时间衰减因子
	TrendWeight         float64 `json:"trend_weight"`         // 趋势权重
	SeasonWeight        float64 `json:"season_weight"`        // 季节性权重
}

// CachePolicy 缓存策略
type CachePolicy struct {
	MaxL1SizeGB     float64 `json:"max_l1_size_gb"`   // L1缓存最大容量
	MaxL2SizeGB     float64 `json:"max_l2_size_gb"`   // L2缓存最大容量
	MaxL3SizeGB     float64 `json:"max_l3_size_gb"`   // L3缓存最大容量
	EvictionPolicy  string  `json:"eviction_policy"`  // lru, lfu, fifo, adaptive
	TTLHours        int     `json:"ttl_hours"`        // 默认TTL
	AutoWarming     bool    `json:"auto_warming"`     // 自动预热
	WarmingSchedule string  `json:"warming_schedule"` // 预热计划 (cron表达式)
}

// Manager 预测缓存管理器
type Manager struct {
	mu              sync.RWMutex
	accessRecords   map[string][]*FileAccessRecord
	patterns        map[string]*AccessPatternAnalysis
	cacheEntries    map[string]*CacheEntry
	warmingTasks    map[string]*WarmingTask
	predictionModel *PredictionModel
	cachePolicy     *CachePolicy
	hits            int64
	misses          int64
}

// NewManager 创建新的预测缓存管理器
func NewManager() *Manager {
	return &Manager{
		accessRecords: make(map[string][]*FileAccessRecord),
		patterns:      make(map[string]*AccessPatternAnalysis),
		cacheEntries:  make(map[string]*CacheEntry),
		warmingTasks:  make(map[string]*WarmingTask),
		predictionModel: &PredictionModel{
			WindowSize:          30,
			MinSamples:          5,
			ConfidenceThreshold: 0.7,
			DecayFactor:         0.95,
			TrendWeight:         0.3,
			SeasonWeight:        0.2,
		},
		cachePolicy: &CachePolicy{
			MaxL1SizeGB:     8,
			MaxL2SizeGB:     100,
			MaxL3SizeGB:     1000,
			EvictionPolicy:  "adaptive",
			TTLHours:        24,
			AutoWarming:     true,
			WarmingSchedule: "0 2 * * *", // 每天凌晨2点
		},
	}
}

// RecordAccess 记录文件访问
func (m *Manager) RecordAccess(record *FileAccessRecord) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.accessRecords[record.FilePath] = append(m.accessRecords[record.FilePath], record)

	// 限制记录数量
	if len(m.accessRecords[record.FilePath]) > 1000 {
		m.accessRecords[record.FilePath] = m.accessRecords[record.FilePath][100:]
	}

	// 更新缓存命中统计
	if entry, exists := m.cacheEntries[record.FilePath]; exists {
		entry.HitCount++
		entry.LastAccess = time.Now()
		m.hits++
	} else {
		m.misses++
	}
}

// AnalyzePatterns 分析访问模式
func (m *Manager) AnalyzePatterns(filePath string) *AccessPatternAnalysis {
	m.mu.Lock()
	defer m.mu.Unlock()

	records, exists := m.accessRecords[filePath]
	if !exists || len(records) < m.predictionModel.MinSamples {
		return &AccessPatternAnalysis{
			FilePath:   filePath,
			Pattern:    PatternRandom,
			Confidence: ConfidenceLow,
		}
	}

	// 计算访问频率
	frequency := m.calculateFrequency(records)

	// 检测访问模式
	pattern := m.detectPattern(records)

	// 计算趋势分数
	trendScore := m.calculateTrend(records)

	// 计算季节性
	seasonality := m.calculateSeasonality(records)

	// 预测下次访问时间
	nextPredicted := m.predictNextAccess(records, pattern, frequency)

	// 计算置信度
	confidence := m.calculateConfidence(records, pattern, trendScore)

	analysis := &AccessPatternAnalysis{
		FilePath:      filePath,
		Pattern:       pattern,
		Frequency:     frequency,
		LastAccess:    records[len(records)-1].AccessTime,
		NextPredicted: nextPredicted,
		Confidence:    confidence,
		TrendScore:    trendScore,
		Seasonality:   seasonality,
	}

	m.patterns[filePath] = analysis
	return analysis
}

// calculateFrequency 计算访问频率
func (m *Manager) calculateFrequency(records []*FileAccessRecord) float64 {
	if len(records) < 2 {
		return 0
	}

	// 计算时间跨度
	firstAccess := records[0].AccessTime
	lastAccess := records[len(records)-1].AccessTime
	duration := lastAccess.Sub(firstAccess)

	if duration.Hours() < 1 {
		return float64(len(records))
	}

	// 返回每天访问次数
	return float64(len(records)) / (duration.Hours() / 24)
}

// detectPattern 检测访问模式
func (m *Manager) detectPattern(records []*FileAccessRecord) AccessPattern {
	if len(records) < 3 {
		return PatternRandom
	}

	// 计算访问间隔
	intervals := make([]float64, 0)
	for i := 1; i < len(records); i++ {
		interval := records[i].AccessTime.Sub(records[i-1].AccessTime).Seconds()
		intervals = append(intervals, interval)
	}

	// 计算间隔的标准差
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

	// 如果标准差很小，可能是周期性访问
	if stddev < mean*0.2 && mean > 0 {
		return PatternPeriodic
	}

	// 检查是否是顺序访问
	sequentialCount := 0
	for i := 1; i < len(records); i++ {
		if records[i].AccessTime.Sub(records[i-1].AccessTime).Minutes() < 5 {
			sequentialCount++
		}
	}
	if float64(sequentialCount)/float64(len(records)) > 0.7 {
		return PatternSequential
	}

	// 检查是否是热点访问
	if m.calculateFrequency(records) > 10 { // 每天超过10次
		return PatternHotspot
	}

	return PatternRandom
}

// calculateTrend 计算趋势分数
func (m *Manager) calculateTrend(records []*FileAccessRecord) float64 {
	if len(records) < 10 {
		return 0.5
	}

	// 将记录分为两半，比较访问频率
	half := len(records) / 2
	firstHalf := records[:half]
	secondHalf := records[half:]

	firstFreq := m.calculateFrequency(firstHalf)
	secondFreq := m.calculateFrequency(secondHalf)

	if firstFreq == 0 {
		return 0.5
	}

	// 计算趋势 (0-1, >0.5表示上升趋势)
	trend := secondFreq / (firstFreq + secondFreq)
	return math.Max(0, math.Min(1, trend))
}

// calculateSeasonality 计算季节性因子
func (m *Manager) calculateSeasonality(records []*FileAccessRecord) float64 {
	if len(records) < 7 {
		return 0
	}

	// 按小时统计访问分布
	hourCounts := make(map[int]int)
	for _, r := range records {
		hour := r.AccessTime.Hour()
		hourCounts[hour]++
	}

	// 计算分布的熵
	total := float64(len(records))
	entropy := 0.0
	for _, count := range hourCounts {
		p := float64(count) / total
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}

	// 归一化到0-1
	maxEntropy := math.Log2(24)
	return 1 - (entropy / maxEntropy)
}

// predictNextAccess 预测下次访问时间
func (m *Manager) predictNextAccess(records []*FileAccessRecord, pattern AccessPattern, frequency float64) time.Time {
	if len(records) == 0 {
		return time.Time{}
	}

	lastAccess := records[len(records)-1].AccessTime

	switch pattern {
	case PatternPeriodic:
		// 计算平均间隔
		if len(records) >= 2 {
			totalInterval := 0.0
			for i := 1; i < len(records); i++ {
				totalInterval += records[i].AccessTime.Sub(records[i-1].AccessTime).Seconds()
			}
			avgInterval := totalInterval / float64(len(records)-1)
			return lastAccess.Add(time.Duration(avgInterval) * time.Second)
		}
	case PatternHotspot:
		// 热点文件很快会被再次访问
		return lastAccess.Add(time.Hour)
	case PatternSeasonal:
		// 下一个周期
		return lastAccess.Add(24 * time.Hour)
	}

	// 默认：基于频率预测
	if frequency > 0 {
		intervalHours := 24 / frequency
		return lastAccess.Add(time.Duration(intervalHours * float64(time.Hour)))
	}

	return lastAccess.Add(24 * time.Hour)
}

// calculateConfidence 计算预测置信度
func (m *Manager) calculateConfidence(records []*FileAccessRecord, pattern AccessPattern, trendScore float64) PredictionConfidence {
	sampleScore := math.Min(1, float64(len(records))/50) // 样本数量分数
	patternScore := 0.5
	switch pattern {
	case PatternPeriodic:
		patternScore = 0.9
	case PatternHotspot:
		patternScore = 0.8
	case PatternSequential:
		patternScore = 0.7
	case PatternSeasonal:
		patternScore = 0.75
	}

	confidence := (sampleScore + patternScore + trendScore) / 3

	if confidence > 0.7 {
		return ConfidenceHigh
	} else if confidence > 0.4 {
		return ConfidenceMedium
	}
	return ConfidenceLow
}

// LoadToCache 加载文件到缓存
func (m *Manager) LoadToCache(filePath string, sizeBytes int64, cacheLevel CacheLevel) (*CacheEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查缓存容量
	if !m.checkCacheCapacity(cacheLevel, sizeBytes) {
		// 尝试驱逐
		m.evictCache(cacheLevel, sizeBytes)
	}

	entry := &CacheEntry{
		ID:         uuid.New().String(),
		FilePath:   filePath,
		CacheLevel: cacheLevel,
		SizeBytes:  sizeBytes,
		Priority:   m.calculatePriority(filePath),
		LoadedAt:   time.Now(),
		LastAccess: time.Now(),
		ExpiresAt:  time.Now().Add(time.Duration(m.cachePolicy.TTLHours) * time.Hour),
	}

	m.cacheEntries[filePath] = entry
	return entry, nil
}

// checkCacheCapacity 检查缓存容量
func (m *Manager) checkCacheCapacity(level CacheLevel, newSize int64) bool {
	var maxSize int64
	switch level {
	case CacheL1:
		maxSize = int64(m.cachePolicy.MaxL1SizeGB * 1024 * 1024 * 1024)
	case CacheL2:
		maxSize = int64(m.cachePolicy.MaxL2SizeGB * 1024 * 1024 * 1024)
	case CacheL3:
		maxSize = int64(m.cachePolicy.MaxL3SizeGB * 1024 * 1024 * 1024)
	}

	currentSize := int64(0)
	for _, entry := range m.cacheEntries {
		if entry.CacheLevel == level {
			currentSize += entry.SizeBytes
		}
	}

	return currentSize+newSize <= maxSize
}

// evictCache 驱逐缓存
func (m *Manager) evictCache(level CacheLevel, neededBytes int64) {
	// 收集同层级的缓存条目
	entries := make([]*CacheEntry, 0)
	for _, entry := range m.cacheEntries {
		if entry.CacheLevel == level && !entry.Pinned {
			entries = append(entries, entry)
		}
	}

	// 根据策略排序
	switch m.cachePolicy.EvictionPolicy {
	case "lru":
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].LastAccess.Before(entries[j].LastAccess)
		})
	case "lfu":
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].HitCount < entries[j].HitCount
		})
	case "fifo":
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].LoadedAt.Before(entries[j].LoadedAt)
		})
	default: // adaptive
		sort.Slice(entries, func(i, j int) bool {
			scoreI := m.calculateEvictionScore(entries[i])
			scoreJ := m.calculateEvictionScore(entries[j])
			return scoreI < scoreJ
		})
	}

	// 驱逐直到有足够空间
	freedBytes := int64(0)
	for _, entry := range entries {
		if freedBytes >= neededBytes {
			break
		}
		freedBytes += entry.SizeBytes
		delete(m.cacheEntries, entry.FilePath)
	}
}

// calculateEvictionScore 计算驱逐分数 (越低越容易被驱逐)
func (m *Manager) calculateEvictionScore(entry *CacheEntry) float64 {
	age := time.Since(entry.LastAccess).Hours()
	hitRate := entry.HitRate
	priority := float64(entry.Priority) / 10

	// 综合分数
	return hitRate*0.4 + priority*0.3 + (1/(1+age))*0.3
}

// calculatePriority 计算文件优先级
func (m *Manager) calculatePriority(filePath string) int {
	analysis := m.patterns[filePath]
	if analysis == nil {
		return 5 // 默认中等优先级
	}

	priority := 5

	// 根据模式调整
	switch analysis.Pattern {
	case PatternHotspot:
		priority = 9
	case PatternPeriodic:
		priority = 7
	case PatternSequential:
		priority = 6
	case PatternSeasonal:
		priority = 6
	}

	// 根据置信度调整
	switch analysis.Confidence {
	case ConfidenceHigh:
		priority += 1
	case ConfidenceLow:
		priority -= 1
	}

	// 根据趋势调整
	if analysis.TrendScore > 0.7 {
		priority += 1
	} else if analysis.TrendScore < 0.3 {
		priority -= 1
	}

	return max(1, min(10, priority))
}

// CreateWarmingTask 创建预热任务
func (m *Manager) CreateWarmingTask(filePath string, cacheLevel CacheLevel) *WarmingTask {
	m.mu.Lock()
	defer m.mu.Unlock()

	task := &WarmingTask{
		ID:         uuid.New().String(),
		FilePath:   filePath,
		CacheLevel: cacheLevel,
		Priority:   m.calculatePriority(filePath),
		Status:     "pending",
		StartedAt:  time.Now(),
	}

	m.warmingTasks[task.ID] = task
	return task
}

// GetWarmingTask 获取预热任务
func (m *Manager) GetWarmingTask(taskID string) (*WarmingTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.warmingTasks[taskID]
	if !exists {
		return nil, fmt.Errorf("预热任务 %s 不存在", taskID)
	}

	return task, nil
}

// ListWarmingTasks 列出预热任务
func (m *Manager) ListWarmingTasks(status string) []*WarmingTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*WarmingTask, 0)
	for _, task := range m.warmingTasks {
		if status == "" || task.Status == status {
			tasks = append(tasks, task)
		}
	}

	// 按优先级排序
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Priority > tasks[j].Priority
	})

	return tasks
}

// GetCacheStats 获取缓存统计
func (m *Manager) GetCacheStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_entries": len(m.cacheEntries),
		"total_hits":    m.hits,
		"total_misses":  m.misses,
		"hit_rate":      float64(0),
		"warming_tasks": len(m.warmingTasks),
	}

	if m.hits+m.misses > 0 {
		stats["hit_rate"] = float64(m.hits) / float64(m.hits+m.misses)
	}

	// 按层级统计
	levelStats := make(map[string]map[string]interface{})
	for _, level := range []CacheLevel{CacheL1, CacheL2, CacheL3} {
		levelStats[string(level)] = map[string]interface{}{
			"count": 0,
			"size":  int64(0),
		}
	}

	for _, entry := range m.cacheEntries {
		ls := levelStats[string(entry.CacheLevel)]
		ls["count"] = ls["count"].(int) + 1
		ls["size"] = ls["size"].(int64) + entry.SizeBytes
	}

	stats["levels"] = levelStats

	return stats
}

// GetPatternAnalysis 获取模式分析
func (m *Manager) GetPatternAnalysis(filePath string) *AccessPatternAnalysis {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.patterns[filePath]
}

// ListPatterns 列出所有模式分析
func (m *Manager) ListPatterns(minConfidence PredictionConfidence) []*AccessPatternAnalysis {
	m.mu.RLock()
	defer m.mu.RUnlock()

	patterns := make([]*AccessPatternAnalysis, 0)
	for _, p := range m.patterns {
		switch minConfidence {
		case ConfidenceHigh:
			if p.Confidence == ConfidenceHigh {
				patterns = append(patterns, p)
			}
		case ConfidenceMedium:
			if p.Confidence == ConfidenceHigh || p.Confidence == ConfidenceMedium {
				patterns = append(patterns, p)
			}
		default:
			patterns = append(patterns, p)
		}
	}

	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].TrendScore > patterns[j].TrendScore
	})

	return patterns
}

// AutoWarm 自动预热
func (m *Manager) AutoWarm() []*WarmingTask {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.cachePolicy.AutoWarming {
		return nil
	}

	tasks := make([]*WarmingTask, 0)

	// 分析所有访问记录
	for filePath, records := range m.accessRecords {
		if len(records) < m.predictionModel.MinSamples {
			continue
		}

		// 检查是否已在缓存中
		if _, exists := m.cacheEntries[filePath]; exists {
			continue
		}

		// 分析模式
		analysis := m.analyzePatternInternal(records)
		if analysis == nil {
			continue
		}

		// 检查置信度
		if analysis.Confidence == ConfidenceLow {
			continue
		}

		// 检查是否应该预热
		if m.shouldWarm(analysis) {
			// 确定缓存层级
			cacheLevel := m.determineCacheLevel(records)

			task := &WarmingTask{
				ID:         uuid.New().String(),
				FilePath:   filePath,
				CacheLevel: cacheLevel,
				Priority:   m.calculatePriority(filePath),
				Status:     "pending",
				StartedAt:  time.Now(),
			}

			m.warmingTasks[task.ID] = task
			tasks = append(tasks, task)
		}
	}

	return tasks
}

// analyzePatternInternal 内部模式分析
func (m *Manager) analyzePatternInternal(records []*FileAccessRecord) *AccessPatternAnalysis {
	if len(records) < 2 {
		return nil
	}

	frequency := m.calculateFrequency(records)
	pattern := m.detectPattern(records)
	trendScore := m.calculateTrend(records)
	seasonality := m.calculateSeasonality(records)
	nextPredicted := m.predictNextAccess(records, pattern, frequency)
	confidence := m.calculateConfidence(records, pattern, trendScore)

	return &AccessPatternAnalysis{
		FilePath:      records[0].FilePath,
		Pattern:       pattern,
		Frequency:     frequency,
		LastAccess:    records[len(records)-1].AccessTime,
		NextPredicted: nextPredicted,
		Confidence:    confidence,
		TrendScore:    trendScore,
		Seasonality:   seasonality,
	}
}

// shouldWarm 判断是否应该预热
func (m *Manager) shouldWarm(analysis *AccessPatternAnalysis) bool {
	// 检查预测时间
	if analysis.NextPredicted.IsZero() {
		return false
	}

	// 如果预测时间在未来24小时内，应该预热
	hoursUntilAccess := time.Until(analysis.NextPredicted).Hours()
	if hoursUntilAccess > 0 && hoursUntilAccess < 24 {
		return true
	}

	// 如果是热点文件且趋势上升
	if analysis.Pattern == PatternHotspot && analysis.TrendScore > 0.6 {
		return true
	}

	return false
}

// determineCacheLevel 确定缓存层级
func (m *Manager) determineCacheLevel(records []*FileAccessRecord) CacheLevel {
	// 计算平均文件大小
	totalSize := int64(0)
	for _, r := range records {
		totalSize += r.SizeBytes
	}
	avgSize := totalSize / int64(len(records))

	// 小文件放L1，中等文件放L2，大文件放L3
	if avgSize < 10*1024*1024 { // < 10MB
		return CacheL1
	} else if avgSize < 1024*1024*1024 { // < 1GB
		return CacheL2
	}
	return CacheL3
}

// GetCacheEntry 获取缓存条目
func (m *Manager) GetCacheEntry(filePath string) *CacheEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.cacheEntries[filePath]
	if !exists {
		return nil
	}

	// 检查是否过期
	if time.Now().After(entry.ExpiresAt) && !entry.Pinned {
		delete(m.cacheEntries, filePath)
		return nil
	}

	return entry
}

// PinCache 固定缓存条目
func (m *Manager) PinCache(filePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.cacheEntries[filePath]
	if !exists {
		return fmt.Errorf("缓存条目 %s 不存在", filePath)
	}

	entry.Pinned = true
	return nil
}

// UnpinCache 取消固定缓存条目
func (m *Manager) UnpinCache(filePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.cacheEntries[filePath]
	if !exists {
		return fmt.Errorf("缓存条目 %s 不存在", filePath)
	}

	entry.Pinned = false
	return nil
}

// ClearCache 清空缓存
func (m *Manager) ClearCache(level CacheLevel) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for path, entry := range m.cacheEntries {
		if level == "" || entry.CacheLevel == level {
			if !entry.Pinned {
				delete(m.cacheEntries, path)
				count++
			}
		}
	}

	return count
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
