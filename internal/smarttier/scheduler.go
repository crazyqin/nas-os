// Package smarttier 提供智能存储分层调度器
// 增强 hybridpool 的自动分层策略，支持：
// - I/O模式感知（顺序/随机/突发）
// - 预取预测（基于访问模式预加载数据到SSD）
// - 自适应阈值（根据SSD容量使用率动态调整提升/降级阈值）
// - 批量迁移（减少碎片，提升迁移效率）
package smarttier

import (
	"log"
	"math"
	"sync"
	"time"
)

// IOPattern I/O访问模式
type IOPattern string

const (
	PatternSequential IOPattern = "sequential" // 顺序读写
	PatternRandom     IOPattern = "random"     // 随机读写
	PatternBurst      IOPattern = "burst"      // 突发访问
	PatternStreaming  IOPattern = "streaming"  // 流式访问（大文件连续读）
)

// AccessWindow 访问时间窗口
type AccessWindow struct {
	Timestamp time.Time `json:"timestamp"`
	Count     int64     `json:"count"`
	Bytes     int64     `json:"bytes"`
}

// FileIOMetadata 文件I/O元数据
type FileIOMetadata struct {
	FilePath       string          `json:"filePath"`
	Pattern        IOPattern       `json:"pattern"`
	AccessWindows  []AccessWindow  `json:"-"`           // 最近N个时间窗口的访问记录
	WindowSize     time.Duration   `json:"windowSize"`  // 窗口大小
	MaxWindows     int             `json:"maxWindows"`  // 最大窗口数
	AvgInterval    time.Duration   `json:"avgInterval"` // 平均访问间隔
	BurstDetected  bool            `json:"burstDetected"`
	PredictedNext  time.Time       `json:"predictedNext"` // 预测下次访问时间
	ConfidenceScore float64        `json:"confidenceScore"` // 预测置信度 0-1
}

// SchedulerConfig 分层调度器配置
type SchedulerConfig struct {
	// 基础配置
	SSDCapacityBytes     int64         `json:"ssdCapacityBytes"`
	SSDUsageThreshold    float64       `json:"ssdUsageThreshold"`    // SSD使用率上限（如0.85=85%）
	TieringInterval      time.Duration `json:"tieringInterval"`      // 分层扫描间隔

	// 自适应阈值
	EnableAdaptiveThreshold bool    `json:"enableAdaptiveThreshold"`
	BasePromoteThreshold    float64 `json:"basePromoteThreshold"`  // 基础提升阈值
	BaseDemoteThreshold     float64 `json:"baseDemoteThreshold"`   // 基础降级阈值
	AdaptiveSensitivity     float64 `json:"adaptiveSensitivity"`   // 自适应灵敏度

	// 预取配置
	EnablePrefetch      bool          `json:"enablePrefetch"`
	PrefetchWindow      time.Duration `json:"prefetchWindow"`      // 预取时间窗口
	MinConfidence       float64       `json:"minConfidence"`       // 最低预取置信度

	// 批量迁移
	BatchSize           int           `json:"batchSize"`           // 单次批量迁移文件数
	MigrationBandwidth  int64         `json:"migrationBandwidth"`  // 迁移带宽限制(bytes/s)

	// I/O模式检测
	WindowCount         int           `json:"windowCount"`         // 分析窗口数
	WindowDuration      time.Duration `json:"windowDuration"`      // 每个窗口时长
	BurstThreshold      float64       `json:"burstThreshold"`      // 突发检测阈值（倍数）
}

// DefaultSchedulerConfig 返回默认配置
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		SSDCapacityBytes:     500 * 1024 * 1024 * 1024, // 500GB
		SSDUsageThreshold:    0.85,
		TieringInterval:      5 * time.Minute,
		EnableAdaptiveThreshold: true,
		BasePromoteThreshold: 70,
		BaseDemoteThreshold:  30,
		AdaptiveSensitivity:  0.5,
		EnablePrefetch:       true,
		PrefetchWindow:       10 * time.Minute,
		MinConfidence:        0.6,
		BatchSize:            50,
		MigrationBandwidth:   100 * 1024 * 1024, // 100MB/s
		WindowCount:          12,
		WindowDuration:       5 * time.Minute,
		BurstThreshold:       3.0,
	}
}

// TierDecision 分层决策
type TierDecision struct {
	FilePath    string    `json:"filePath"`
	Action      string    `json:"action"`      // promote/demote/prefetch/keep
	FromTier    string    `json:"fromTier"`
	ToTier      string    `json:"toTier"`
	Reason      string    `json:"reason"`
	Priority    int       `json:"priority"`    // 1-10, 10最高
	HeatScore   float64   `json:"heatScore"`
	Confidence  float64   `json:"confidence"`
}

// SchedulerStats 调度器统计
type SchedulerStats struct {
	TotalDecisions    int64   `json:"totalDecisions"`
	PromoteCount      int64   `json:"promoteCount"`
	DemoteCount       int64   `json:"demoteCount"`
	PrefetchCount     int64   `json:"prefetchCount"`
	PrefetchHits      int64   `json:"prefetchHits"`     // 预取命中次数
	PrefetchMisses    int64   `json:"prefetchMisses"`   // 预取未命中次数
	PrefetchHitRate   float64 `json:"prefetchHitRate"`
	AdaptiveShifts    int64   `json:"adaptiveShifts"`   // 自适应阈值调整次数
	SSDUsagePercent   float64 `json:"ssdUsagePercent"`
	CurrentPromoteThreshold float64 `json:"currentPromoteThreshold"`
	CurrentDemoteThreshold  float64 `json:"currentDemoteThreshold"`
	BatchesProcessed  int64   `json:"batchesProcessed"`
	AvgBatchTimeMs    float64 `json:"avgBatchTimeMs"`
}

// SmartTierScheduler 智能分层调度器
type SmartTierScheduler struct {
	config    SchedulerConfig
	ioMeta    map[string]*FileIOMetadata
	stats     SchedulerStats
	decisions []TierDecision
	mu        sync.RWMutex
	running   bool
	stopCh    chan struct{}

	// 自适应阈值
	currentPromoteThreshold float64
	currentDemoteThreshold  float64

	// 外部接口
	onDecide func(decision TierDecision) // 决策回调
	onMigrate func(filePath, fromTier, toTier string) error // 迁移回调
}

// NewSmartTierScheduler 创建智能分层调度器
func NewSmartTierScheduler(config SchedulerConfig) *SmartTierScheduler {
	return &SmartTierScheduler{
		config:    config,
		ioMeta:    make(map[string]*FileIOMetadata),
		decisions: make([]TierDecision, 0),
		stopCh:    make(chan struct{}),
		currentPromoteThreshold: config.BasePromoteThreshold,
		currentDemoteThreshold:  config.BaseDemoteThreshold,
	}
}

// SetMigrateCallback 设置迁移回调
func (s *SmartTierScheduler) SetMigrateCallback(fn func(filePath, fromTier, toTier string) error) {
	s.onMigrate = fn
}

// SetDecideCallback 设置决策回调
func (s *SmartTierScheduler) SetDecideCallback(fn func(decision TierDecision)) {
	s.onDecide = fn
}

// Start 启动调度器
func (s *SmartTierScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go s.scheduleLoop()
	log.Printf("[SmartTier] scheduler started, interval=%v, prefetch=%v, adaptive=%v",
		s.config.TieringInterval, s.config.EnablePrefetch, s.config.EnableAdaptiveThreshold)
}

// Stop 停止调度器
func (s *SmartTierScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	close(s.stopCh)
	s.running = false
	log.Printf("[SmartTier] scheduler stopped")
}

// RecordIO 记录I/O访问
func (s *SmartTierScheduler) RecordIO(filePath string, bytesRead, bytesWritten int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, exists := s.ioMeta[filePath]
	if !exists {
		meta = &FileIOMetadata{
			FilePath:   filePath,
			WindowSize: s.config.WindowDuration,
			MaxWindows: s.config.WindowCount,
		}
		s.ioMeta[filePath] = meta
	}

	now := time.Now()
	totalBytes := bytesRead + bytesWritten

	// 更新当前窗口
	if len(meta.AccessWindows) == 0 || now.Sub(meta.AccessWindows[len(meta.AccessWindows)-1].Timestamp) >= meta.WindowSize {
		// 新窗口
		meta.AccessWindows = append(meta.AccessWindows, AccessWindow{
			Timestamp: now,
			Count:     1,
			Bytes:     totalBytes,
		})
		// 保持窗口数在限制内
		if len(meta.AccessWindows) > meta.MaxWindows {
			meta.AccessWindows = meta.AccessWindows[len(meta.AccessWindows)-meta.MaxWindows:]
		}
	} else {
		// 更新当前窗口
		last := &meta.AccessWindows[len(meta.AccessWindows)-1]
		last.Count++
		last.Bytes += totalBytes
	}

	// 更新I/O模式
	meta.Pattern = s.detectIOPattern(meta)

	// 更新平均访问间隔
	s.updateAvgInterval(meta)

	// 检测突发
	meta.BurstDetected = s.detectBurst(meta)

	// 预测下次访问
	if s.config.EnablePrefetch {
		meta.PredictedNext, meta.ConfidenceScore = s.predictNextAccess(meta)
	}
}

// AnalyzeAndDecide 分析并生成分层决策
func (s *SmartTierScheduler) AnalyzeAndDecide(fileHeatScores map[string]float64, currentTiers map[string]string) []TierDecision {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var decisions []TierDecision

	for filePath, heatScore := range fileHeatScores {
		currentTier := currentTiers[filePath]
		meta := s.ioMeta[filePath]

		decision := s.makeDecision(filePath, heatScore, currentTier, meta)
		if decision.Action != "keep" {
			decisions = append(decisions, decision)
		}
	}

	// 按优先级排序
	sortDecisions(decisions)

	// 批量限制
	if len(decisions) > s.config.BatchSize {
		decisions = decisions[:s.config.BatchSize]
	}

	return decisions
}

// makeDecision 单文件决策
func (s *SmartTierScheduler) makeDecision(filePath string, heatScore float64, currentTier string, meta *FileIOMetadata) TierDecision {
	decision := TierDecision{
		FilePath:  filePath,
		Action:    "keep",
		FromTier:  currentTier,
		HeatScore: heatScore,
	}

	// 自适应阈值调整
	promoteThreshold := s.currentPromoteThreshold
	demoteThreshold := s.currentDemoteThreshold

	if meta != nil {
		// I/O模式感知调整
		switch meta.Pattern {
		case PatternSequential, PatternStreaming:
			// 顺序/流式访问对SSD收益大，降低提升阈值
			promoteThreshold *= 0.85
		case PatternRandom:
			// 随机读写对SSD收益最大
			promoteThreshold *= 0.75
		case PatternBurst:
			// 突发访问需要快速提升
			promoteThreshold *= 0.7
		}

		// 突发检测：临时提升优先级
		if meta.BurstDetected {
			decision.Priority = 9
			decision.Reason = "burst detected"
			if heatScore >= demoteThreshold && currentTier != "hot" {
				decision.Action = "promote"
				decision.ToTier = "hot"
				decision.Confidence = 0.9
				return decision
			}
		}

		// 预取决策
		if s.config.EnablePrefetch && meta.ConfidenceScore >= s.config.MinConfidence {
			timeUntilAccess := time.Until(meta.PredictedNext)
			if timeUntilAccess > 0 && timeUntilAccess <= s.config.PrefetchWindow {
				if currentTier != "hot" {
					decision.Action = "prefetch"
					decision.ToTier = "hot"
					decision.Priority = 7
					decision.Confidence = meta.ConfidenceScore
					decision.Reason = "prefetch predicted access"
					return decision
				}
			}
		}
	}

	// 标准分层决策
	switch {
	case heatScore >= promoteThreshold && currentTier != "hot":
		decision.Action = "promote"
		decision.ToTier = "hot"
		decision.Priority = int(math.Min(10, heatScore/10))
		decision.Reason = "heat score above promote threshold"
	case heatScore <= demoteThreshold && currentTier == "hot":
		decision.Action = "demote"
		decision.ToTier = "cold"
		decision.Priority = int(math.Max(1, (100-heatScore)/10))
		decision.Reason = "heat score below demote threshold"
	case heatScore > demoteThreshold && heatScore < promoteThreshold && currentTier == "cold":
		decision.Action = "promote"
		decision.ToTier = "warm"
		decision.Priority = 4
		decision.Reason = "warm tier promotion"
	}

	return decision
}

// detectIOPattern 检测I/O访问模式
func (s *SmartTierScheduler) detectIOPattern(meta *FileIOMetadata) IOPattern {
	if len(meta.AccessWindows) < 3 {
		return PatternRandom // 默认随机
	}

	// 计算窗口间访问量的标准差
	var counts []float64
	for _, w := range meta.AccessWindows {
		counts = append(counts, float64(w.Count))
	}

	mean := meanValue(counts)
	stddev := stdDev(counts, mean)

	// 变异系数
	cv := stddev / mean

	switch {
	case cv < 0.3:
		// 低变异 -> 顺序/流式
		if len(meta.AccessWindows) > 0 {
			avgBytes := meanBytesPerWindow(meta.AccessWindows)
			if avgBytes > 4*1024*1024 { // >4MB per window
				return PatternStreaming
			}
		}
		return PatternSequential
	case cv > 2.0:
		// 高变异 -> 突发
		return PatternBurst
	default:
		return PatternRandom
	}
}

// detectBurst 检测突发访问
func (s *SmartTierScheduler) detectBurst(meta *FileIOMetadata) bool {
	if len(meta.AccessWindows) < 2 {
		return false
	}
	// 最近窗口访问量 vs 平均值
	recent := float64(meta.AccessWindows[len(meta.AccessWindows)-1].Count)
	avg := float64(0)
	for _, w := range meta.AccessWindows[:len(meta.AccessWindows)-1] {
		avg += float64(w.Count)
	}
	avg /= float64(len(meta.AccessWindows) - 1)

	return recent > avg*s.config.BurstThreshold
}

// predictNextAccess 预测下次访问时间
func (s *SmartTierScheduler) predictNextAccess(meta *FileIOMetadata) (time.Time, float64) {
	if len(meta.AccessWindows) < 3 {
		return time.Time{}, 0
	}

	// 基于访问间隔的指数平滑预测
	var intervals []time.Duration
	for i := 1; i < len(meta.AccessWindows); i++ {
		intervals = append(intervals, meta.AccessWindows[i].Timestamp.Sub(meta.AccessWindows[i-1].Timestamp))
	}

	if len(intervals) == 0 {
		return time.Time{}, 0
	}

	// 指数平滑 (α=0.3)
	alpha := 0.3
	smoothed := float64(intervals[0])
	for _, interval := range intervals[1:] {
		smoothed = alpha*float64(interval) + (1-alpha)*smoothed
	}

	predictedInterval := time.Duration(smoothed)
	lastAccess := meta.AccessWindows[len(meta.AccessWindows)-1].Timestamp
	predicted := lastAccess.Add(predictedInterval)

	// 置信度：基于间隔的一致性
	var intervalFloats []float64
	for _, iv := range intervals {
		intervalFloats = append(intervalFloats, float64(iv))
	}
	mean := meanValue(intervalFloats)
	sd := stdDev(intervalFloats, mean)
	confidence := 1.0 - math.Min(1.0, sd/mean) // 变异系数越小，置信度越高

	return predicted, confidence
}

// updateAvgInterval 更新平均访问间隔
func (s *SmartTierScheduler) updateAvgInterval(meta *FileIOMetadata) {
	if len(meta.AccessWindows) < 2 {
		return
	}
	var total time.Duration
	for i := 1; i < len(meta.AccessWindows); i++ {
		total += meta.AccessWindows[i].Timestamp.Sub(meta.AccessWindows[i-1].Timestamp)
	}
	meta.AvgInterval = total / time.Duration(len(meta.AccessWindows)-1)
}

// scheduleLoop 调度主循环
func (s *SmartTierScheduler) scheduleLoop() {
	ticker := time.NewTicker(s.config.TieringInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.runAdaptiveAdjustment()
		}
	}
}

// runAdaptiveAdjustment 运行自适应阈值调整
func (s *SmartTierScheduler) runAdaptiveAdjustment() {
	if !s.config.EnableAdaptiveThreshold {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 根据SSD使用率动态调整阈值
	ssdUsage := s.estimateSSDUsage()
	s.stats.SSDUsagePercent = ssdUsage

	sensitivity := s.config.AdaptiveSensitivity

	if ssdUsage > s.config.SSDUsageThreshold {
		// SSD快满了，提高提升阈值（更难提升到SSD）
		factor := 1 + (ssdUsage-s.config.SSDUsageThreshold)*sensitivity*10
		s.currentPromoteThreshold = math.Min(95, s.config.BasePromoteThreshold*factor)
		s.currentDemoteThreshold = math.Min(70, s.config.BaseDemoteThreshold*factor)
		s.stats.AdaptiveShifts++
		log.Printf("[SmartTier] adaptive: SSD usage %.1f%%, promote threshold %.1f -> %.1f",
			ssdUsage*100, s.config.BasePromoteThreshold, s.currentPromoteThreshold)
	} else if ssdUsage < s.config.SSDUsageThreshold*0.6 {
		// SSD空间充足，降低提升阈值（更容易提升）
		factor := 1 - (s.config.SSDUsageThreshold*0.6-ssdUsage)*sensitivity*5
		factor = math.Max(0.7, factor)
		s.currentPromoteThreshold = math.Max(50, s.config.BasePromoteThreshold*factor)
		s.currentDemoteThreshold = math.Max(15, s.config.BaseDemoteThreshold*factor)
	}

	s.stats.CurrentPromoteThreshold = s.currentPromoteThreshold
	s.stats.CurrentDemoteThreshold = s.currentDemoteThreshold
}

// estimateSSDUsage 估算SSD使用率
func (s *SmartTierScheduler) estimateSSDUsage() float64 {
	// 统计被标记为hot的文件总大小
	var hotBytes int64
	for _, meta := range s.ioMeta {
		// 简化：根据热度推算是否在SSD上
		if meta.Pattern == PatternSequential || meta.Pattern == PatternRandom {
			// 高频访问文件可能在SSD上
			hotBytes += int64(len(meta.AccessWindows)) * 1024 * 1024 // 估算
		}
	}
	if s.config.SSDCapacityBytes == 0 {
		return 0
	}
	return float64(hotBytes) / float64(s.config.SSDCapacityBytes)
}

// GetStats 获取调度器统计
func (s *SmartTierScheduler) GetStats() SchedulerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stats := s.stats
	if stats.PrefetchHits+stats.PrefetchMisses > 0 {
		stats.PrefetchHitRate = float64(stats.PrefetchHits) / float64(stats.PrefetchHits+stats.PrefetchMisses) * 100
	}
	return stats
}

// GetIOPatterns 获取所有文件的I/O模式
func (s *SmartTierScheduler) GetIOPatterns() map[string]IOPattern {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]IOPattern)
	for path, meta := range s.ioMeta {
		result[path] = meta.Pattern
	}
	return result
}

// RecordPrefetchHit 记录预取命中
func (s *SmartTierScheduler) RecordPrefetchHit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.PrefetchHits++
}

// RecordPrefetchMiss 记录预取未命中
func (s *SmartTierScheduler) RecordPrefetchMiss() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.PrefetchMisses++
}

// --- 辅助函数 ---

func sortDecisions(decisions []TierDecision) {
	for i := 1; i < len(decisions); i++ {
		for j := i; j > 0 && decisions[j].Priority > decisions[j-1].Priority; j-- {
			decisions[j], decisions[j-1] = decisions[j-1], decisions[j]
		}
	}
}

func meanValue(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func stdDev(values []float64, mean float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		diff := v - mean
		sum += diff * diff
	}
	return math.Sqrt(sum / float64(len(values)))
}

func meanBytesPerWindow(windows []AccessWindow) float64 {
	if len(windows) == 0 {
		return 0
	}
	total := int64(0)
	for _, w := range windows {
		total += w.Bytes
	}
	return float64(total) / float64(len(windows))
}
