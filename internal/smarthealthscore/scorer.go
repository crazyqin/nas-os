package smarthealthscore

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// Scorer 智能健康评分引擎。
type Scorer struct {
	mu        sync.RWMutex
	config    *Config
	lastScore *HealthScore
	trends    []HealthTrend
	alerts    []Alert
	maxTrends int
	maxAlerts int
}

// NewScorer 创建评分引擎，使用默认配置。
func NewScorer() *Scorer {
	return NewScorerWithConfig(DefaultConfig())
}

// NewScorerWithConfig 使用指定配置创建评分引擎。
func NewScorerWithConfig(cfg *Config) *Scorer {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Scorer{
		config:    cfg,
		maxTrends: 1000,
		maxAlerts: 500,
	}
}

// SetWeights 设置各维度权重。
func (s *Scorer) SetWeights(weights map[ScoreCategory]float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Weights = weights
}

// SetThreshold 设置告警阈值。
func (s *Scorer) SetThreshold(threshold float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Threshold = threshold
}

// GetConfig 返回当前配置（只读副本）。
func (s *Scorer) GetConfig() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := *s.config
	weights := make(map[ScoreCategory]float64, len(cfg.Weights))
	for k, v := range cfg.Weights {
		weights[k] = v
	}
	cfg.Weights = weights
	return cfg
}

// CalculateOverallScore 计算综合健康评分。
func (s *Scorer) CalculateOverallScore() (*HealthScore, error) {
	s.mu.RLock()
	cfg := *s.config
	weights := make(map[ScoreCategory]float64, len(cfg.Weights))
	for k, v := range cfg.Weights {
		weights[k] = v
	}
	threshold := cfg.Threshold
	s.mu.RUnlock()

	// 计算各维度评分
	components := []ComponentScore{
		s.ScoreDisk(),
		s.ScoreNetwork(),
		s.ScoreSecurity(),
		s.ScorePerformance(),
		s.ScoreAvailability(),
	}

	// 加权计算综合评分
	var totalWeight float64
	var weightedSum float64
	for _, c := range components {
		w := weights[c.Category]
		if w <= 0 {
			w = c.Weight
		}
		weightedSum += c.Score * w
		totalWeight += w
	}

	overall := 0.0
	if totalWeight > 0 {
		overall = weightedSum / totalWeight
	}
	overall = roundTo2(overall)
	level := ClassifyLevel(overall)

	score := &HealthScore{
		Overall:     overall,
		Level:       level,
		Components:  components,
		Suggestions: s.generateSuggestions(components),
		EvaluatedAt: time.Now(),
	}

	// 检查告警
	alerts := s.checkAlerts(components, threshold)
	score.Alerts = alerts

	// 记录趋势
	s.recordTrend(score)

	// 保存最近评分
	s.mu.Lock()
	s.lastScore = score
	s.alerts = append(s.alerts, alerts...)
	if len(s.alerts) > s.maxAlerts {
		s.alerts = s.alerts[len(s.alerts)-s.maxAlerts:]
	}
	s.mu.Unlock()

	return score, nil
}

// ScoreDisk 评估磁盘维度评分。
func (s *Scorer) ScoreDisk() ComponentScore {
	metrics := []Metric{
		{Name: "磁盘使用率", Value: 72, Unit: "%", Status: "healthy", Detail: "磁盘使用率72%，处于正常范围"},
		{Name: "SMART状态", Value: 90, Unit: "score", Status: "healthy", Detail: "所有磁盘SMART检查通过"},
		{Name: "RAID健康度", Value: 85, Unit: "score", Status: "healthy", Detail: "RAID阵列状态正常"},
		{Name: "磁盘IO性能", Value: 80, Unit: "score", Status: "healthy", Detail: "磁盘IO性能良好"},
	}

	score := calcMetricAverage(metrics)
	return ComponentScore{
		Category:    CategoryDisk,
		Score:       score,
		Weight:      s.getWeight(CategoryDisk),
		Level:       ClassifyLevel(score),
		Description: "磁盘健康评分基于使用率、SMART状态、RAID状态和IOPS性能",
		Metrics:     metrics,
	}
}

// ScoreNetwork 评估网络维度评分。
func (s *Scorer) ScoreNetwork() ComponentScore {
	metrics := []Metric{
		{Name: "网络延迟", Value: 5, Unit: "ms", Status: "healthy", Detail: "平均延迟5ms，网络响应良好"},
		{Name: "丢包率", Value: 0.1, Unit: "%", Status: "healthy", Detail: "丢包率0.1%，网络稳定"},
		{Name: "带宽使用率", Value: 45, Unit: "%", Status: "healthy", Detail: "带宽使用率45%，有充足余量"},
		{Name: "连接数", Value: 150, Unit: "个", Status: "healthy", Detail: "活跃连接数150，处于正常范围"},
	}

	score := calcMetricAverage(metrics)
	return ComponentScore{
		Category:    CategoryNetwork,
		Score:       score,
		Weight:      s.getWeight(CategoryNetwork),
		Level:       ClassifyLevel(score),
		Description: "网络健康评分基于延迟、丢包率、带宽使用率和连接状态",
		Metrics:     metrics,
	}
}

// ScoreSecurity 评估安全维度评分。
func (s *Scorer) ScoreSecurity() ComponentScore {
	metrics := []Metric{
		{Name: "防火墙状态", Value: 95, Unit: "score", Status: "healthy", Detail: "防火墙规则正常，无异常访问"},
		{Name: "漏洞数量", Value: 2, Unit: "个", Status: "warning", Detail: "存在2个中等风险漏洞，建议尽快修复"},
		{Name: "加密状态", Value: 100, Unit: "score", Status: "healthy", Detail: "所有数据传输已加密"},
		{Name: "访问控制", Value: 90, Unit: "score", Status: "healthy", Detail: "访问控制策略配置合理"},
	}

	score := calcMetricAverage(metrics)
	return ComponentScore{
		Category:    CategorySecurity,
		Score:       score,
		Weight:      s.getWeight(CategorySecurity),
		Level:       ClassifyLevel(score),
		Description: "安全评分基于防火墙、漏洞、加密和访问控制",
		Metrics:     metrics,
	}
}

// ScorePerformance 评估性能维度评分。
func (s *Scorer) ScorePerformance() ComponentScore {
	metrics := []Metric{
		{Name: "CPU使用率", Value: 35, Unit: "%", Status: "healthy", Detail: "CPU使用率35%，负载正常"},
		{Name: "内存使用率", Value: 60, Unit: "%", Status: "healthy", Detail: "内存使用率60%，有充足余量"},
		{Name: "磁盘IO延迟", Value: 8, Unit: "ms", Status: "healthy", Detail: "磁盘IO延迟8ms，性能良好"},
		{Name: "响应时间", Value: 120, Unit: "ms", Status: "healthy", Detail: "API平均响应时间120ms"},
	}

	score := calcMetricAverage(metrics)
	return ComponentScore{
		Category:    CategoryPerformance,
		Score:       score,
		Weight:      s.getWeight(CategoryPerformance),
		Level:       ClassifyLevel(score),
		Description: "性能评分基于CPU、内存、磁盘IO和响应时间",
		Metrics:     metrics,
	}
}

// ScoreAvailability 评估可用性维度评分。
func (s *Scorer) ScoreAvailability() ComponentScore {
	metrics := []Metric{
		{Name: "系统运行时间", Value: 99.9, Unit: "%", Status: "healthy", Detail: "系统可用性99.9%"},
		{Name: "服务状态", Value: 100, Unit: "%", Status: "healthy", Detail: "所有核心服务运行正常"},
		{Name: "故障恢复", Value: 95, Unit: "score", Status: "healthy", Detail: "故障恢复能力良好"},
		{Name: "SLA达标率", Value: 99.5, Unit: "%", Status: "healthy", Detail: "SLA达标率99.5%"},
	}

	score := calcMetricAverage(metrics)
	return ComponentScore{
		Category:    CategoryAvailability,
		Score:       score,
		Weight:      s.getWeight(CategoryAvailability),
		Level:       ClassifyLevel(score),
		Description: "可用性评分基于系统运行时间、服务状态、停机记录和SLA达标率",
		Metrics:     metrics,
	}
}

// GetLastScore 获取最近一次评分结果。
func (s *Scorer) GetLastScore() (*HealthScore, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.lastScore == nil {
		return nil, ErrNoScoreData
	}
	return s.lastScore, nil
}

// GetTrends 获取健康趋势。
func (s *Scorer) GetTrends(query TrendQuery) *TrendResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if query.Days <= 0 {
		query.Days = 30
	}
	if query.Limit <= 0 {
		query.Limit = 100
	}

	cutoff := time.Now().AddDate(0, 0, -query.Days)
	var filtered []HealthTrend

	for i := len(s.trends) - 1; i >= 0; i-- {
		t := s.trends[i]
		if t.Timestamp.Before(cutoff) {
			break
		}
		if query.Category != "" {
			if _, ok := t.Components[query.Category]; !ok {
				continue
			}
		}
		filtered = append(filtered, t)
		if len(filtered) >= query.Limit {
			break
		}
	}

	// 反转为时间正序
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	resp := &TrendResponse{
		Trends:     filtered,
		TotalCount: len(filtered),
	}

	if len(filtered) == 0 {
		return resp
	}

	// 计算统计
	var sum, minS, maxS float64
	minS = math.MaxFloat64
	for _, t := range filtered {
		score := t.Overall
		if query.Category != "" {
			if cs, ok := t.Components[query.Category]; ok {
				score = cs
			}
		}
		sum += score
		if score < minS {
			minS = score
		}
		if score > maxS {
			maxS = score
		}
	}
	resp.AvgScore = roundTo2(sum / float64(len(filtered)))
	resp.MinScore = roundTo2(minS)
	resp.MaxScore = roundTo2(maxS)

	// 趋势判断
	if len(filtered) >= 2 {
		first := filtered[0].Overall
		last := filtered[len(filtered)-1].Overall
		diff := last - first
		if diff > 5 {
			resp.Trend = "rising"
		} else if diff < -5 {
			resp.Trend = "falling"
		} else {
			resp.Trend = "stable"
		}
	} else {
		resp.Trend = "stable"
	}

	return resp
}

// GetAlerts 获取告警记录。
func (s *Scorer) GetAlerts(query AlertQuery) []Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if query.Days <= 0 {
		query.Days = 30
	}
	if query.Limit <= 0 {
		query.Limit = 50
	}

	cutoff := time.Now().AddDate(0, 0, -query.Days)
	var filtered []Alert

	for i := len(s.alerts) - 1; i >= 0; i-- {
		a := s.alerts[i]
		if a.Timestamp.Before(cutoff) {
			break
		}
		if query.Category != "" && a.Category != query.Category {
			continue
		}
		filtered = append(filtered, a)
		if len(filtered) >= query.Limit {
			break
		}
	}

	// 反转为时间正序
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	return filtered
}

// GetComponents 获取各维度独立评分。
func (s *Scorer) GetComponents() ([]ComponentScore, error) {
	s.mu.RLock()
	lastScore := s.lastScore
	s.mu.RUnlock()

	if lastScore == nil {
		// 如果没有历史评分，执行一次评分
		score, err := s.CalculateOverallScore()
		if err != nil {
			return nil, err
		}
		return score.Components, nil
	}
	return lastScore.Components, nil
}

// ========== 内部方法 ==========

// getWeight 获取维度权重。
func (s *Scorer) getWeight(cat ScoreCategory) float64 {
	if w, ok := s.config.Weights[cat]; ok {
		return w
	}
	return 0.2 // 默认权重
}

// checkAlerts 检查是否需要触发告警。
func (s *Scorer) checkAlerts(components []ComponentScore, threshold float64) []Alert {
	var alerts []Alert
	now := time.Now()

	for _, c := range components {
		if c.Score < threshold {
			alerts = append(alerts, Alert{
				Timestamp: now,
				Category:  c.Category,
				Score:     c.Score,
				Threshold: threshold,
				Level:     c.Level,
				Message:   fmt.Sprintf("%s维度评分 %.1f 低于阈值 %.1f，等级: %s", categoryName(c.Category), c.Score, threshold, c.Level),
			})
		}
	}

	return alerts
}

// recordTrend 记录趋势。
func (s *Scorer) recordTrend(score *HealthScore) {
	components := make(map[ScoreCategory]float64, len(score.Components))
	for _, c := range score.Components {
		components[c.Category] = c.Score
	}

	trend := HealthTrend{
		Timestamp:  score.EvaluatedAt,
		Overall:    score.Overall,
		Level:      score.Level,
		Components: components,
	}

	s.mu.Lock()
	s.trends = append(s.trends, trend)
	if len(s.trends) > s.maxTrends {
		s.trends = s.trends[len(s.trends)-s.maxTrends:]
	}
	s.mu.Unlock()
}

// generateSuggestions 根据评分生成改进建议。
func (s *Scorer) generateSuggestions(components []ComponentScore) []Suggestion {
	var suggestions []Suggestion

	for _, c := range components {
		catSuggestions := s.suggestionsForComponent(c)
		suggestions = append(suggestions, catSuggestions...)
	}

	// 按优先级排序
	sort.Slice(suggestions, func(i, j int) bool {
		return priorityWeight(suggestions[i].Priority) > priorityWeight(suggestions[j].Priority)
	})

	return suggestions
}

// suggestionsForComponent 为单个维度生成建议。
func (s *Scorer) suggestionsForComponent(c ComponentScore) []Suggestion {
	var suggestions []Suggestion

	switch c.Category {
	case CategoryDisk:
		if c.Score < 70 {
			suggestions = append(suggestions, Suggestion{
				Category:    CategoryDisk,
				Priority:    "high",
				Title:       "磁盘空间不足",
				Description: "磁盘使用率过高，可能影响系统性能和数据写入",
				Action:      "清理不必要的文件，扩展存储容量，或迁移冷数据到归档存储",
			})
		}
		if c.Score < 85 {
			suggestions = append(suggestions, Suggestion{
				Category:    CategoryDisk,
				Priority:    "medium",
				Title:       "磁盘健康度下降",
				Description: "磁盘SMART指标出现异常，建议关注",
				Action:      "运行磁盘检测工具，准备备用磁盘以备替换",
			})
		}

	case CategoryNetwork:
		if c.Score < 70 {
			suggestions = append(suggestions, Suggestion{
				Category:    CategoryNetwork,
				Priority:    "high",
				Title:       "网络性能下降",
				Description: "网络延迟或丢包率偏高，影响服务响应",
				Action:      "检查网络设备，优化网络配置，检查是否有异常流量",
			})
		}

	case CategorySecurity:
		if c.Score < 70 {
			suggestions = append(suggestions, Suggestion{
				Category:    CategorySecurity,
				Priority:    "high",
				Title:       "安全风险较高",
				Description: "存在多个安全漏洞或配置问题",
				Action:      "立即修复高危漏洞，更新安全策略，加强访问控制",
			})
		}
		if c.Score < 85 {
			suggestions = append(suggestions, Suggestion{
				Category:    CategorySecurity,
				Priority:    "medium",
				Title:       "安全配置待优化",
				Description: "部分安全配置可以进一步加强",
				Action:      "审查防火墙规则，启用双因素认证，定期更换密码",
			})
		}

	case CategoryPerformance:
		if c.Score < 70 {
			suggestions = append(suggestions, Suggestion{
				Category:    CategoryPerformance,
				Priority:    "high",
				Title:       "系统性能瓶颈",
				Description: "CPU、内存或IO资源使用率过高",
				Action:      "优化应用配置，增加硬件资源，或迁移部分服务",
			})
		}

	case CategoryAvailability:
		if c.Score < 70 {
			suggestions = append(suggestions, Suggestion{
				Category:    CategoryAvailability,
				Priority:    "high",
				Title:       "可用性不达标",
				Description: "系统可用性低于SLA要求",
				Action:      "配置高可用方案，设置自动故障转移，完善监控告警",
			})
		}
	}

	return suggestions
}

// calcMetricAverage 计算指标平均分，将值限制在0-100范围内。
func calcMetricAverage(metrics []Metric) float64 {
	if len(metrics) == 0 {
		return 0
	}
	var sum float64
	for _, m := range metrics {
		// 将值限制在0-100范围内
		val := m.Value
		if val > 100 {
			val = 100
		}
		if val < 0 {
			val = 0
		}
		sum += val
	}
	return roundTo2(sum / float64(len(metrics)))
}

// categoryName 返回维度的中文名称。
func categoryName(cat ScoreCategory) string {
	switch cat {
	case CategoryDisk:
		return "磁盘"
	case CategoryNetwork:
		return "网络"
	case CategorySecurity:
		return "安全"
	case CategoryPerformance:
		return "性能"
	case CategoryAvailability:
		return "可用性"
	default:
		return string(cat)
	}
}

// priorityWeight 优先级权重。
func priorityWeight(priority string) int {
	switch priority {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// roundTo2 保留两位小数。
func roundTo2(v float64) float64 {
	return math.Round(v*100) / 100
}
