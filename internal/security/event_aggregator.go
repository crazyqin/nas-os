// Package security provides event aggregation and correlation analysis
// Version: 2.40.0 - Event Aggregation Module
package security

import (
	"sync"
	"time"
)

// ========== 事件聚合器 ==========

// EventAggregator 事件聚合器.
type EventAggregator struct {
	config       AggregatorConfig
	eventGroups  map[string]*EventGroup
	correlations []*EventCorrelation
	mu           sync.RWMutex
}

// AggregatorConfig 聚合器配置.
type AggregatorConfig struct {
	Enabled              bool          `json:"enabled"`
	GroupWindow          time.Duration `json:"group_window"`          // 分组时间窗口
	MaxGroupSize         int           `json:"max_group_size"`        // 最大分组大小
	CorrelationThreshold float64       `json:"correlation_threshold"` // 关联阈值
	CleanupInterval      time.Duration `json:"cleanup_interval"`      // 清理间隔
}

// DefaultAggregatorConfig 默认聚合器配置.
func DefaultAggregatorConfig() AggregatorConfig {
	return AggregatorConfig{
		Enabled:              true,
		GroupWindow:          5 * time.Minute,
		MaxGroupSize:         100,
		CorrelationThreshold: 0.7,
		CleanupInterval:      time.Hour,
	}
}

// EventGroup 事件组.
type EventGroup struct {
	ID         string           `json:"id"`
	Type       string           `json:"type"`       // 分组类型
	SourceIP   string           `json:"source_ip"`  // 来源IP
	Username   string           `json:"username"`   // 用户名
	Events     []*AuditLogEntry `json:"events"`     // 相关事件
	FirstSeen  time.Time        `json:"first_seen"` // 首次出现
	LastSeen   time.Time        `json:"last_seen"`  // 最后出现
	RiskScore  float64          `json:"risk_score"` // 风险评分
	Correlated bool             `json:"correlated"` // 是否已关联
	RootCause  string           `json:"root_cause"` // 根因分析
	Summary    string           `json:"summary"`    // 事件摘要
}

// EventCorrelation 事件关联.
type EventCorrelation struct {
	ID              string    `json:"id"`
	GroupIDs        []string  `json:"group_ids"`        // 关联的组ID
	CorrelationType string    `json:"correlation_type"` // 关联类型
	Confidence      float64   `json:"confidence"`       // 置信度
	Description     string    `json:"description"`      // 描述
	Timestamp       time.Time `json:"timestamp"`
	Severity        string    `json:"severity"`
}

// AggregationReport 聚合报告.
type AggregationReport struct {
	GeneratedAt       time.Time           `json:"generated_at"`
	TotalEvents       int                 `json:"total_events"`
	TotalGroups       int                 `json:"total_groups"`
	TotalCorrelations int                 `json:"total_correlations"`
	Groups            []*EventGroup       `json:"groups"`
	Correlations      []*EventCorrelation `json:"correlations"`
	TopRisks          []*EventGroup       `json:"top_risks"` // 高风险事件组
	Timeline          []TimelineEntry     `json:"timeline"`  // 时间线
}

// TimelineEntry 时间线条目.
type TimelineEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Count     int       `json:"count"`
	Severity  string    `json:"severity"`
}

// NewEventAggregator 创建事件聚合器.
func NewEventAggregator(config AggregatorConfig) *EventAggregator {
	return &EventAggregator{
		config:       config,
		eventGroups:  make(map[string]*EventGroup),
		correlations: make([]*EventCorrelation, 0),
	}
}

// Aggregate 聚合事件.
func (a *EventAggregator) Aggregate(events []*AuditLogEntry) *AggregationReport {
	if !a.config.Enabled {
		return &AggregationReport{}
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// 清理旧的分组
	a.cleanupOldGroups()

	// 按时间和特征分组
	for _, event := range events {
		a.groupEvent(event)
	}

	// 检测事件关联
	a.detectCorrelations()

	// 生成报告
	return a.generateReport()
}

// groupEvent 将事件分组.
func (a *EventAggregator) groupEvent(event *AuditLogEntry) {
	// 生成分组键
	groupKey := a.generateGroupKey(event)

	group, exists := a.eventGroups[groupKey]
	if !exists {
		// 创建新分组
		group = &EventGroup{
			ID:        generateGroupID(),
			Type:      a.determineGroupType(event),
			SourceIP:  event.IP,
			Username:  event.Username,
			Events:    make([]*AuditLogEntry, 0),
			FirstSeen: event.Timestamp,
			LastSeen:  event.Timestamp,
		}
		a.eventGroups[groupKey] = group
	}

	// 添加事件到分组
	group.Events = append(group.Events, event)
	group.LastSeen = event.Timestamp

	// 限制分组大小
	if len(group.Events) > a.config.MaxGroupSize {
		group.Events = group.Events[len(group.Events)-a.config.MaxGroupSize:]
	}

	// 更新风险评分
	group.RiskScore = a.calculateGroupRiskScore(group)
}

// generateGroupKey 生成分组键.
func (a *EventAggregator) generateGroupKey(event *AuditLogEntry) string {
	// 基于IP、用户名和事件类型生成键
	return event.IP + ":" + event.Username + ":" + event.Event
}

// determineGroupType 确定分组类型.
func (a *EventAggregator) determineGroupType(event *AuditLogEntry) string {
	switch event.Category {
	case "auth":
		if event.Event == "login_failure" {
			return "brute_force"
		}
		return "authentication"
	case "security":
		return "security_incident"
	case "config":
		return "configuration_change"
	case "file":
		return "file_access"
	default:
		return "general"
	}
}

// calculateGroupRiskScore 计算分组风险评分.
func (a *EventAggregator) calculateGroupRiskScore(group *EventGroup) float64 {
	score := 0.0

	for _, event := range group.Events {
		// 根据事件级别评分
		switch event.Level {
		case "critical":
			score += 10
		case "error":
			score += 5
		case "warning":
			score += 2
		case "info":
			score += 0.5
		}

		// 失败事件加权
		if event.Status == "failure" {
			score += 2
		}
	}

	// 根据事件数量调整
	if len(group.Events) > 10 {
		score *= 1.5
	} else if len(group.Events) > 5 {
		score *= 1.2
	}

	return score
}

// detectCorrelations 检测事件关联.
func (a *EventAggregator) detectCorrelations() {
	groups := make([]*EventGroup, 0)
	for _, g := range a.eventGroups {
		groups = append(groups, g)
	}

	// 检测IP关联
	a.detectIPCorrelations(groups)

	// 检测时间关联
	a.detectTimeCorrelations(groups)

	// 检测因果关联
	a.detectCausalCorrelations(groups)
}

// detectIPCorrelations 检测IP关联.
func (a *EventAggregator) detectIPCorrelations(groups []*EventGroup) {
	ipGroups := make(map[string][]*EventGroup)

	for _, g := range groups {
		if g.SourceIP != "" {
			ipGroups[g.SourceIP] = append(ipGroups[g.SourceIP], g)
		}
	}

	for _, ipGroupList := range ipGroups {
		if len(ipGroupList) >= 2 {
			// 同一IP多个事件组，可能有关联
			groupIDs := make([]string, len(ipGroupList))
			for i, g := range ipGroupList {
				groupIDs[i] = g.ID
			}

			correlation := &EventCorrelation{
				ID:              generateCorrelationID(),
				GroupIDs:        groupIDs,
				CorrelationType: "same_source_ip",
				Confidence:      0.8,
				Description:     "来自同一IP的多个安全事件",
				Timestamp:       time.Now(),
				Severity:        "medium",
			}

			a.correlations = append(a.correlations, correlation)
		}
	}
}

// detectTimeCorrelations 检测时间关联.
func (a *EventAggregator) detectTimeCorrelations(groups []*EventGroup) {
	// 检测短时间内发生的多个事件组
	window := 10 * time.Minute

	for i, g1 := range groups {
		for j, g2 := range groups {
			if i >= j {
				continue
			}

			// 检查时间窗口
			timeDiff := g1.FirstSeen.Sub(g2.FirstSeen)
			if timeDiff < 0 {
				timeDiff = -timeDiff
			}

			if timeDiff <= window && g1.SourceIP != g2.SourceIP {
				// 不同IP但在短时间内发生，可能有关联
				correlation := &EventCorrelation{
					ID:              generateCorrelationID(),
					GroupIDs:        []string{g1.ID, g2.ID},
					CorrelationType: "temporal",
					Confidence:      0.6,
					Description:     "短时间内发生的多个安全事件",
					Timestamp:       time.Now(),
					Severity:        "medium",
				}

				a.correlations = append(a.correlations, correlation)
			}
		}
	}
}

// detectCausalCorrelations 检测因果关联.
func (a *EventAggregator) detectCausalCorrelations(groups []*EventGroup) {
	// 检测可能的因果关系
	causalPatterns := []struct {
		causeType  string
		effectType string
		confidence float64
	}{
		{"brute_force", "authentication", 0.7},
		{"brute_force", "account_lock", 0.9},
		{"authentication", "privilege_escalation", 0.6},
		{"configuration_change", "security_incident", 0.5},
	}

	for _, pattern := range causalPatterns {
		var causeGroups, effectGroups []*EventGroup

		for _, g := range groups {
			switch g.Type {
			case pattern.causeType:
				causeGroups = append(causeGroups, g)
			case pattern.effectType:
				effectGroups = append(effectGroups, g)
			}
		}

		// 查找时间上匹配的因果对
		for _, cause := range causeGroups {
			for _, effect := range effectGroups {
				if effect.FirstSeen.After(cause.FirstSeen) &&
					effect.FirstSeen.Sub(cause.FirstSeen) <= 30*time.Minute {
					correlation := &EventCorrelation{
						ID:              generateCorrelationID(),
						GroupIDs:        []string{cause.ID, effect.ID},
						CorrelationType: "causal",
						Confidence:      pattern.confidence,
						Description:     "检测到可能的因果关系",
						Timestamp:       time.Now(),
						Severity:        "high",
					}

					a.correlations = append(a.correlations, correlation)
				}
			}
		}
	}
}

// cleanupOldGroups 清理旧的分组.
func (a *EventAggregator) cleanupOldGroups() {
	cutoff := time.Now().Add(-a.config.GroupWindow * 10)

	for key, group := range a.eventGroups {
		if group.LastSeen.Before(cutoff) {
			delete(a.eventGroups, key)
		}
	}

	// 清理旧的关联
	var validCorrelations []*EventCorrelation
	for _, c := range a.correlations {
		if c.Timestamp.After(cutoff) {
			validCorrelations = append(validCorrelations, c)
		}
	}
	a.correlations = validCorrelations
}

// generateReport 生成报告.
func (a *EventAggregator) generateReport() *AggregationReport {
	report := &AggregationReport{
		GeneratedAt:       time.Now(),
		TotalGroups:       len(a.eventGroups),
		TotalCorrelations: len(a.correlations),
		Groups:            make([]*EventGroup, 0, len(a.eventGroups)),
		Correlations:      a.correlations,
		TopRisks:          make([]*EventGroup, 0),
		Timeline:          make([]TimelineEntry, 0),
	}

	// 收集所有分组
	for _, g := range a.eventGroups {
		report.Groups = append(report.Groups, g)
		report.TotalEvents += len(g.Events)
	}

	// 找出高风险分组
	report.TopRisks = a.findTopRiskGroups(report.Groups, 5)

	// 生成时间线
	report.Timeline = a.generateTimeline(report.Groups)

	return report
}

// findTopRiskGroups 找出高风险分组.
func (a *EventAggregator) findTopRiskGroups(groups []*EventGroup, limit int) []*EventGroup {
	// 按风险评分排序
	sorted := make([]*EventGroup, len(groups))
	copy(sorted, groups)

	// 简单排序
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].RiskScore > sorted[i].RiskScore {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	if limit > len(sorted) {
		limit = len(sorted)
	}

	return sorted[:limit]
}

// generateTimeline 生成时间线.
func (a *EventAggregator) generateTimeline(groups []*EventGroup) []TimelineEntry {
	// 按小时聚合
	hourlyCounts := make(map[int64]map[string]int)

	for _, g := range groups {
		for _, e := range g.Events {
			hour := e.Timestamp.Truncate(time.Hour).Unix()
			if _, ok := hourlyCounts[hour]; !ok {
				hourlyCounts[hour] = map[string]int{
					"info":     0,
					"warning":  0,
					"error":    0,
					"critical": 0,
				}
			}
			hourlyCounts[hour][e.Level]++
		}
	}

	var timeline []TimelineEntry
	for ts, counts := range hourlyCounts {
		// 找出最高严重级别
		severity := "info"
		if counts["critical"] > 0 {
			severity = "critical"
		} else if counts["error"] > 0 {
			severity = "error"
		} else if counts["warning"] > 0 {
			severity = "warning"
		}

		total := 0
		for _, c := range counts {
			total += c
		}

		timeline = append(timeline, TimelineEntry{
			Timestamp: time.Unix(ts, 0),
			Count:     total,
			Severity:  severity,
		})
	}

	return timeline
}

// GetGroupByID 根据ID获取事件组.
func (a *EventAggregator) GetGroupByID(id string) *EventGroup {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, g := range a.eventGroups {
		if g.ID == id {
			return g
		}
	}
	return nil
}

// GetCorrelationsByGroupID 获取与指定分组相关的关联.
func (a *EventAggregator) GetCorrelationsByGroupID(groupID string) []*EventCorrelation {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []*EventCorrelation
	for _, c := range a.correlations {
		for _, gid := range c.GroupIDs {
			if gid == groupID {
				result = append(result, c)
				break
			}
		}
	}
	return result
}

// ========== 辅助函数 ==========

// generateGroupID 生成分组ID.
func generateGroupID() string {
	return "grp-" + time.Now().Format("20060102150405") + "-" + randomString(6)
}

// generateCorrelationID 生成关联ID.
func generateCorrelationID() string {
	return "corr-" + time.Now().Format("20060102150405") + "-" + randomString(6)
}

// randomString 生成随机字符串.
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}
