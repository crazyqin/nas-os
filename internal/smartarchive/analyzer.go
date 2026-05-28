package smartarchive

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// Analyzer 数据使用模式分析器.
type Analyzer struct {
	mu sync.RWMutex

	// 配置
	config AnalyzerConfig

	// 访问模式数据
	patterns map[string]*AccessPattern

	// 历史统计
	history []AccessSnapshot

	// 运行状态
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// AccessSnapshot 访问快照.
type AccessSnapshot struct {
	Timestamp time.Time                `json:"timestamp"`
	Patterns  map[string]*AccessPattern `json:"patterns"`
	Summary   *AccessSummary            `json:"summary"`
}

// AccessSummary 访问摘要.
type AccessSummary struct {
	TotalFiles     int64   `json:"totalFiles"`
	TotalAccesses  int64   `json:"totalAccesses"`
	HotFiles       int64   `json:"hotFiles"`
	WarmFiles      int64   `json:"warmFiles"`
	ColdFiles      int64   `json:"coldFiles"`
	IceFiles       int64   `json:"iceFiles"`
	AvgHeatScore   float64 `json:"avgHeatScore"`
	TrendDirection string  `json:"trendDirection"` // up/down/stable
}

// AnalysisResult 分析结果.
type AnalysisResult struct {
	Timestamp    time.Time                       `json:"timestamp"`
	Summary      *AccessSummary                  `json:"summary"`
	Patterns     map[string]*AccessPattern        `json:"patterns"`
	Recommendations []ArchiveRecommendation       `json:"recommendations"`
	Insights     []string                         `json:"insights"`
}

// ArchiveRecommendation 归档建议.
type ArchiveRecommendation struct {
	FilePath    string        `json:"filePath"`
	CurrentTier StorageTier   `json:"currentTier"`
	TargetTier  StorageTier   `json:"targetTier"`
	Action      ArchiveAction `json:"action"`
	Reason      string        `json:"reason"`
	Priority    int           `json:"priority"` // 1-10
	Confidence  float64       `json:"confidence"` // 0-1
	EstSaving   int64         `json:"estSaving"` // 预计节省空间
}

// TrendData 趋势数据.
type TrendData struct {
	Period      string    `json:"period"`
	StartValue  float64   `json:"startValue"`
	EndValue    float64   `json:"endValue"`
	Change      float64   `json:"change"`
	ChangeRate  float64   `json:"changeRate"`
	Direction   string    `json:"direction"` // up/down/stable
	Confidence  float64   `json:"confidence"`
}

// NewAnalyzer 创建分析器.
func NewAnalyzer(config AnalyzerConfig) *Analyzer {
	ctx, cancel := context.WithCancel(context.Background())

	return &Analyzer{
		config:   config,
		patterns: make(map[string]*AccessPattern),
		history:  make([]AccessSnapshot, 0),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start 启动分析器.
func (a *Analyzer) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return fmt.Errorf("分析器已在运行中")
	}

	a.running = true
	go a.analysisLoop()

	log.Println("[Analyzer] 数据分析器已启动")
	return nil
}

// Stop 停止分析器.
func (a *Analyzer) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return
	}

	a.cancel()
	a.running = false
	log.Println("[Analyzer] 数据分析器已停止")
}

// RecordAccess 记录文件访问.
func (a *Analyzer) RecordAccess(filePath string, accessType string, bytes int64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	pattern, exists := a.patterns[filePath]
	if !exists {
		pattern = &AccessPattern{
			FilePath:    filePath,
			CurrentTier: TierHot,
			FirstAccess: time.Now(),
			Tags:        make([]string, 0),
		}
		a.patterns[filePath] = pattern
	}

	// 更新访问统计
	pattern.TotalAccesses++
	pattern.LastAccess = time.Now()

	switch accessType {
	case "read":
		pattern.ReadCount++
		pattern.ReadBytes += bytes
	case "write":
		pattern.WriteCount++
		pattern.WriteBytes += bytes
	}

	// 更新热度评分
	a.updateHeatScore(pattern)
}

// RecordAccessBatch 批量记录访问.
func (a *Analyzer) RecordAccessBatch(records []AccessRecord) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, record := range records {
		pattern, exists := a.patterns[record.FilePath]
		if !exists {
			pattern = &AccessPattern{
				FilePath:    record.FilePath,
				CurrentTier: TierHot,
				FirstAccess: time.Now(),
				Tags:        make([]string, 0),
			}
			a.patterns[record.FilePath] = pattern
		}

		pattern.TotalAccesses++
		pattern.LastAccess = time.Now()

		switch record.AccessType {
		case "read":
			pattern.ReadCount++
			pattern.ReadBytes += record.Bytes
		case "write":
			pattern.WriteCount++
			pattern.WriteBytes += record.Bytes
		}
	}

	// 批量更新热度评分
	for _, pattern := range a.patterns {
		a.updateHeatScore(pattern)
	}
}

// AccessRecord 访问记录.
type AccessRecord struct {
	FilePath   string `json:"filePath"`
	AccessType string `json:"accessType"` // read/write
	Bytes      int64  `json:"bytes"`
}

// updateHeatScore 更新热度评分.
func (a *Analyzer) updateHeatScore(pattern *AccessPattern) {
	// 计算访问频率（次/天）
	if !pattern.FirstAccess.IsZero() {
		duration := time.Since(pattern.FirstAccess)
		if duration > 0 {
			days := duration.Hours() / 24
			if days > 0 {
				pattern.AccessFrequency = float64(pattern.TotalAccesses) / days
			}
		}
	}

	// 计算闲置时长
	if !pattern.LastAccess.IsZero() {
		pattern.IdleDuration = time.Since(pattern.LastAccess)
	}

	// 计算热度评分（0-100）
	heatScore := 0.0

	// 访问频率因子（权重 40%）
	freqScore := math.Min(pattern.AccessFrequency*10, 100)
	heatScore += freqScore * 0.4

	// 最近访问因子（权重 30%）
	recencyScore := 100.0
	if pattern.IdleDuration > 0 {
		hours := pattern.IdleDuration.Hours()
		switch {
		case hours <= 1:
			recencyScore = 100
		case hours <= 24:
			recencyScore = 80
		case hours <= 168: // 1 week
			recencyScore = 50
		case hours <= 720: // 1 month
			recencyScore = 20
		default:
			recencyScore = 5
		}
	}
	heatScore += recencyScore * 0.3

	// 读写比例因子（权重 15%）
	totalIO := pattern.ReadBytes + pattern.WriteBytes
	if totalIO > 0 {
		readRatio := float64(pattern.ReadBytes) / float64(totalIO)
		ioScore := readRatio*60 + 40 // 读多写少得分高
		heatScore += ioScore * 0.15
	}

	// 数据量因子（权重 15%）
	sizeScore := 50.0
	if pattern.Size > 0 {
		sizeMB := float64(pattern.Size) / (1024 * 1024)
		switch {
		case sizeMB <= 1:
			sizeScore = 90 // 小文件访问快
		case sizeMB <= 100:
			sizeScore = 70
		case sizeMB <= 1024:
			sizeScore = 50
		default:
			sizeScore = 30 // 大文件可能需要冷存储
		}
	}
	heatScore += sizeScore * 0.15

	pattern.HeatScore = math.Min(heatScore, 100)

	// 更新趋势评分
	a.updateTrendScore(pattern)

	// 更新推荐
	a.updateRecommendation(pattern)
}

// updateTrendScore 更新趋势评分.
func (a *Analyzer) updateTrendScore(pattern *AccessPattern) {
	// 简化的趋势计算
	// 实际应该基于历史数据计算移动平均
	if pattern.IdleDuration < 24*time.Hour {
		pattern.TrendScore = 10 // 升温
	} else if pattern.IdleDuration < 7*24*time.Hour {
		pattern.TrendScore = 0 // 稳定
	} else {
		pattern.TrendScore = -10 // 降温
	}
}

// updateRecommendation 更新推荐.
func (a *Analyzer) updateRecommendation(pattern *AccessPattern) {
	// 根据热度评分推荐层级
	switch {
	case pattern.HeatScore >= a.config.HotThreshold:
		pattern.RecommendedTier = TierHot
		pattern.RecommendedAction = ArchiveActionMove
		pattern.Confidence = 0.9
	case pattern.HeatScore >= a.config.WarmThreshold:
		pattern.RecommendedTier = TierWarm
		pattern.RecommendedAction = ArchiveActionMove
		pattern.Confidence = 0.8
	case pattern.HeatScore >= a.config.ColdThreshold:
		pattern.RecommendedTier = TierCold
		pattern.RecommendedAction = ArchiveActionCompress
		pattern.Confidence = 0.7
	default:
		pattern.RecommendedTier = TierIce
		pattern.RecommendedAction = ArchiveActionCompress
		pattern.Confidence = 0.6
	}

	// 如果已经在推荐层级，提高置信度
	if pattern.CurrentTier == pattern.RecommendedTier {
		pattern.Confidence = math.Min(pattern.Confidence+0.1, 1.0)
	}
}

// GetPattern 获取文件访问模式.
func (a *Analyzer) GetPattern(filePath string) (*AccessPattern, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	pattern, exists := a.patterns[filePath]
	if !exists {
		return nil, fmt.Errorf("文件 %s 的访问模式不存在", filePath)
	}

	return pattern, nil
}

// ListPatterns 列出所有访问模式.
func (a *Analyzer) ListPatterns(minHeatScore float64, limit int) []*AccessPattern {
	a.mu.RLock()
	defer a.mu.RUnlock()

	patterns := make([]*AccessPattern, 0)
	for _, p := range a.patterns {
		if p.HeatScore >= minHeatScore {
			patterns = append(patterns, p)
		}
	}

	// 按热度评分排序
	sortPatterns(patterns)

	if limit > 0 && len(patterns) > limit {
		patterns = patterns[:limit]
	}

	return patterns
}

// Analyze 执行分析.
func (a *Analyzer) Analyze() *AnalysisResult {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := &AnalysisResult{
		Timestamp:       time.Now(),
		Patterns:        a.patterns,
		Recommendations: make([]ArchiveRecommendation, 0),
		Insights:        make([]string, 0),
	}

	// 生成摘要
	result.Summary = a.generateSummary()

	// 生成推荐
	result.Recommendations = a.generateRecommendations()

	// 生成洞察
	result.Insights = a.generateInsights()

	// 保存快照
	snapshot := AccessSnapshot{
		Timestamp: time.Now(),
		Patterns:  copyPatterns(a.patterns),
		Summary:   result.Summary,
	}
	a.history = append(a.history, snapshot)

	// 限制历史大小
	if len(a.history) > int(a.config.MaxRecords) {
		a.history = a.history[len(a.history)-int(a.config.MaxRecords):]
	}

	return result
}

// generateSummary 生成摘要.
func (a *Analyzer) generateSummary() *AccessSummary {
	summary := &AccessSummary{
		TotalFiles: int64(len(a.patterns)),
	}

	for _, p := range a.patterns {
		summary.TotalAccesses += p.TotalAccesses

		switch {
		case p.HeatScore >= a.config.HotThreshold:
			summary.HotFiles++
		case p.HeatScore >= a.config.WarmThreshold:
			summary.WarmFiles++
		case p.HeatScore >= a.config.ColdThreshold:
			summary.ColdFiles++
		default:
			summary.IceFiles++
		}

		summary.AvgHeatScore += p.HeatScore
	}

	if summary.TotalFiles > 0 {
		summary.AvgHeatScore /= float64(summary.TotalFiles)
	}

	// 判断趋势
	if len(a.history) > 0 {
		lastSnapshot := a.history[len(a.history)-1]
		if lastSnapshot.Summary != nil {
			if summary.AvgHeatScore > lastSnapshot.Summary.AvgHeatScore {
				summary.TrendDirection = "up"
			} else if summary.AvgHeatScore < lastSnapshot.Summary.AvgHeatScore {
				summary.TrendDirection = "down"
			} else {
				summary.TrendDirection = "stable"
			}
		}
	}

	return summary
}

// generateRecommendations 生成推荐.
func (a *Analyzer) generateRecommendations() []ArchiveRecommendation {
	recommendations := make([]ArchiveRecommendation, 0)

	for _, p := range a.patterns {
		// 只推荐需要迁移的文件
		if p.CurrentTier != p.RecommendedTier {
			rec := ArchiveRecommendation{
				FilePath:    p.FilePath,
				CurrentTier: p.CurrentTier,
				TargetTier:  p.RecommendedTier,
				Action:      p.RecommendedAction,
				Confidence:  p.Confidence,
				EstSaving:   a.estimateSaving(p),
			}

			// 计算优先级
			rec.Priority = a.calculatePriority(p)

			// 生成原因
			rec.Reason = a.generateReason(p)

			recommendations = append(recommendations, rec)
		}
	}

	// 按优先级排序
	sortRecommendations(recommendations)

	return recommendations
}

// generateInsights 生成洞察.
func (a *Analyzer) generateInsights() []string {
	insights := make([]string, 0)

	// 热数据比例
	totalFiles := float64(len(a.patterns))
	if totalFiles > 0 {
		hotRatio := float64(a.countByTier(TierHot)) / totalFiles * 100
		if hotRatio > 50 {
			insights = append(insights, fmt.Sprintf("热数据占比 %.1f%%，考虑扩大热数据层容量", hotRatio))
		}
	}

	// 冷数据清理
	coldFiles := a.countByTier(TierCold) + a.countByTier(TierIce)
	if coldFiles > 1000 {
		insights = append(insights, fmt.Sprintf("发现 %d 个冷/冰冻文件，建议执行清理策略", coldFiles))
	}

	// 访问模式异常
	for _, p := range a.patterns {
		if p.TrendScore > 20 {
			insights = append(insights, fmt.Sprintf("文件 %s 访问频率急剧上升，可能需要提升到热数据层", p.FilePath))
		}
	}

	return insights
}

// estimateSaving 估算节省空间.
func (a *Analyzer) estimateSaving(pattern *AccessPattern) int64 {
	// 如果是压缩归档，估算压缩后的大小
	if pattern.RecommendedAction == ArchiveActionCompress {
		// 假设压缩率 50%
		return pattern.Size / 2
	}
	return 0
}

// calculatePriority 计算优先级.
func (a *Analyzer) calculatePriority(pattern *AccessPattern) int {
	priority := 5

	// 热度差异越大，优先级越高
	heatDiff := pattern.HeatScore - a.getTierThreshold(pattern.CurrentTier)
	if heatDiff > 30 {
		priority += 3
	} else if heatDiff > 10 {
		priority += 1
	}

	// 置信度越高，优先级越高
	if pattern.Confidence > 0.8 {
		priority += 2
	}

	// 限制在 1-10 范围
	if priority > 10 {
		priority = 10
	}
	if priority < 1 {
		priority = 1
	}

	return priority
}

// getTierThreshold 获取层级阈值.
func (a *Analyzer) getTierThreshold(tier StorageTier) float64 {
	switch tier {
	case TierHot:
		return a.config.HotThreshold
	case TierWarm:
		return a.config.WarmThreshold
	case TierCold:
		return a.config.ColdThreshold
	default:
		return 0
	}
}

// generateReason 生成推荐原因.
func (a *Analyzer) generateReason(pattern *AccessPattern) string {
	switch {
	case pattern.HeatScore >= a.config.HotThreshold:
		return "访问频繁，建议存储在高性能层"
	case pattern.HeatScore >= a.config.WarmThreshold:
		return "访问适中，建议存储在温数据层"
	case pattern.HeatScore >= a.config.ColdThreshold:
		return "访问较少，建议压缩归档到冷数据层"
	default:
		return "几乎无访问，建议归档到冰冻层"
	}
}

// countByTier 统计指定层级的文件数.
func (a *Analyzer) countByTier(tier StorageTier) int {
	count := 0
	for _, p := range a.patterns {
		if p.CurrentTier == tier {
			count++
		}
	}
	return count
}

// GetTrend 获取趋势数据.
func (a *Analyzer) GetTrend(filePath string, duration time.Duration) *TrendData {
	a.mu.RLock()
	defer a.mu.RUnlock()

	trend := &TrendData{
		Period: duration.String(),
	}

	// 从历史数据中查找
	cutoff := time.Now().Add(-duration)
	var startValue, endValue float64
	found := false

	for _, snapshot := range a.history {
		if snapshot.Timestamp.Before(cutoff) {
			continue
		}

		if pattern, exists := snapshot.Patterns[filePath]; exists {
			if !found {
				startValue = pattern.HeatScore
				found = true
			}
			endValue = pattern.HeatScore
		}
	}

	if found {
		trend.StartValue = startValue
		trend.EndValue = endValue
		trend.Change = endValue - startValue
		if startValue > 0 {
			trend.ChangeRate = trend.Change / startValue * 100
		}

		if trend.Change > 5 {
			trend.Direction = "up"
		} else if trend.Change < -5 {
			trend.Direction = "down"
		} else {
			trend.Direction = "stable"
		}

		trend.Confidence = 0.7 // 简化的置信度计算
	}

	return trend
}

// analysisLoop 分析循环.
func (a *Analyzer) analysisLoop() {
	ticker := time.NewTicker(a.config.AnalysisInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.Analyze()
		}
	}
}

// GetHistory 获取历史快照.
func (a *Analyzer) GetHistory(limit int) []AccessSnapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if limit <= 0 || limit > len(a.history) {
		limit = len(a.history)
	}

	start := len(a.history) - limit
	if start < 0 {
		start = 0
	}

	return a.history[start:]
}

// ClearHistory 清除历史数据.
func (a *Analyzer) ClearHistory() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.history = make([]AccessSnapshot, 0)
	log.Println("[Analyzer] 历史数据已清除")
}

// sortPatterns 按热度评分排序.
func sortPatterns(patterns []*AccessPattern) {
	for i := 0; i < len(patterns); i++ {
		for j := i + 1; j < len(patterns); j++ {
			if patterns[j].HeatScore > patterns[i].HeatScore {
				patterns[i], patterns[j] = patterns[j], patterns[i]
			}
		}
	}
}

// sortRecommendations 按优先级排序.
func sortRecommendations(recs []ArchiveRecommendation) {
	for i := 0; i < len(recs); i++ {
		for j := i + 1; j < len(recs); j++ {
			if recs[j].Priority > recs[i].Priority {
				recs[i], recs[j] = recs[j], recs[i]
			}
		}
	}
}

// copyPatterns 复制访问模式.
func copyPatterns(patterns map[string]*AccessPattern) map[string]*AccessPattern {
	result := make(map[string]*AccessPattern, len(patterns))
	for k, v := range patterns {
		copy := *v
		result[k] = &copy
	}
	return result
}
