// Package storageanomaly - 存储异常检测管理器
// 指标收集、异常检测引擎、告警生成
package storageanomaly

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// AnomalyManager 异常检测管理器
type AnomalyManager struct {
	mu          sync.RWMutex
	config      AnomalyConfig
	rules       map[string]*AnomalyRule     // 规则
	events      []*AnomalyEvent             // 事件列表
	metrics     map[string][]*StorageMetrics // 设备ID -> 指标历史
	lastAlerts  map[string]time.Time        // 规则ID -> 上次告警时间
	alertCounts map[string]int              // 规则ID -> 告警计数
}

// NewAnomalyManager 创建异常检测管理器
func NewAnomalyManager(config *AnomalyConfig) *AnomalyManager {
	cfg := DefaultAnomalyConfig()
	if config != nil {
		cfg = *config
	}

	manager := &AnomalyManager{
		config:      cfg,
		rules:       make(map[string]*AnomalyRule),
		events:      make([]*AnomalyEvent, 0),
		metrics:     make(map[string][]*StorageMetrics),
		lastAlerts:  make(map[string]time.Time),
		alertCounts: make(map[string]int),
	}

	// 初始化默认规则
	manager.initDefaultRules()

	return manager
}

// initDefaultRules 初始化默认规则
func (m *AnomalyManager) initDefaultRules() {
	defaults := []AnomalyRule{
		{
			ID:          "capacity_growth",
			Name:        "容量增长异常",
			Description: "检测容量异常快速增长",
			Type:        AnomalyTypeCapacityGrowth,
			Enabled:     true,
			Severity:    SeverityWarning,
			Conditions: []Condition{
				{Field: "growth_rate", Operator: "gt", Value: m.config.CapacityGrowthThreshold},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "iops_spike",
			Name:        "IOPS峰值异常",
			Description: "检测IOPS异常峰值",
			Type:        AnomalyTypeIOPSSpike,
			Enabled:     true,
			Severity:    SeverityWarning,
			Conditions: []Condition{
				{Field: "iops_ratio", Operator: "gt", Value: m.config.IOPSSpikeThreshold},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "latency_spike",
			Name:        "延迟峰值异常",
			Description: "检测延迟异常峰值",
			Type:        AnomalyTypeLatencySpike,
			Enabled:     true,
			Severity:    SeverityWarning,
			Conditions: []Condition{
				{Field: "latency", Operator: "gt", Value: m.config.LatencySpikeThreshold},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "data_corruption",
			Name:        "数据损坏检测",
			Description: "检测潜在的数据损坏",
			Type:        AnomalyTypeDataCorruption,
			Enabled:     true,
			Severity:    SeverityCritical,
			Conditions: []Condition{
				{Field: "corrupted_files", Operator: "gt", Value: 0},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "disk_usage_critical",
			Name:        "磁盘使用率严重",
			Description: "磁盘使用率超过90%",
			Type:        AnomalyTypeCapacityGrowth,
			Enabled:     true,
			Severity:    SeverityCritical,
			Conditions: []Condition{
				{Field: "usage_percent", Operator: "gt", Value: 90.0},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for i := range defaults {
		m.rules[defaults[i].ID] = &defaults[i]
	}
}

// CollectMetrics 收集存储指标
func (m *AnomalyManager) CollectMetrics(metrics *StorageMetrics) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if metrics.DeviceID == "" {
		return fmt.Errorf("设备ID不能为空")
	}

	if metrics.CollectedAt.IsZero() {
		metrics.CollectedAt = time.Now()
	}

	// 保存指标
	m.metrics[metrics.DeviceID] = append(m.metrics[metrics.DeviceID], metrics)

	// 清理过期指标
	m.cleanupOldMetrics(metrics.DeviceID)

	return nil
}

// cleanupOldMetrics 清理过期指标
func (m *AnomalyManager) cleanupOldMetrics(deviceID string) {
	history := m.metrics[deviceID]
	if len(history) == 0 {
		return
	}

	cutoff := time.Now().Add(-m.config.HistoryWindow)
	validIdx := 0
	for _, h := range history {
		if h.CollectedAt.After(cutoff) {
			history[validIdx] = h
			validIdx++
		}
	}
	m.metrics[deviceID] = history[:validIdx]
}

// DetectAnomalies 检测异常
func (m *AnomalyManager) DetectAnomalies(deviceID string) (*DetectionResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history, exists := m.metrics[deviceID]
	if !exists || len(history) == 0 {
		return &DetectionResult{
			HasAnomaly: false,
			Summary:    "无指标数据",
			AnalyzedAt: time.Now(),
		}, nil
	}

	if len(history) < m.config.MinSamples {
		return &DetectionResult{
			HasAnomaly: false,
			Summary:    fmt.Sprintf("样本不足，需要至少 %d 个样本", m.config.MinSamples),
			AnalyzedAt: time.Now(),
		}, nil
	}

	latest := history[len(history)-1]
	events := make([]AnomalyEvent, 0)

	// 检测容量增长异常
	if m.config.EnableCapacityGrowth {
		capacityEvents := m.detectCapacityAnomaly(deviceID, latest, history)
		events = append(events, capacityEvents...)
	}

	// 检测IOPS异常
	if m.config.EnableIOPSAnomaly {
		iopsEvents := m.detectIOPSAnomaly(deviceID, latest, history)
		events = append(events, iopsEvents...)
	}

	// 检测延迟异常
	if m.config.EnableLatencyAnomaly {
		latencyEvents := m.detectLatencyAnomaly(deviceID, latest, history)
		events = append(events, latencyEvents...)
	}

	// 检测数据损坏
	if m.config.EnableDataCorruption {
		corruptionEvents := m.detectDataCorruption(deviceID, latest)
		events = append(events, corruptionEvents...)
	}

	result := &DetectionResult{
		HasAnomaly:   len(events) > 0,
		AnomalyCount: len(events),
		Events:       events,
		Summary:      m.generateSummary(events),
		AnalyzedAt:   time.Now(),
	}

	// 保存事件到历史记录
	for i := range events {
		m.addEvent(&events[i])
	}

	return result, nil
}

// detectCapacityAnomaly 检测容量异常
func (m *AnomalyManager) detectCapacityAnomaly(deviceID string, latest *StorageMetrics, history []*StorageMetrics) []AnomalyEvent {
	events := make([]AnomalyEvent, 0)

	// 检查使用率是否过高
	if latest.UsagePercent > 90 {
		events = append(events, AnomalyEvent{
			ID:          generateID(),
			RuleID:      "disk_usage_critical",
			RuleName:    "磁盘使用率严重",
			Type:        AnomalyTypeCapacityGrowth,
			Severity:    SeverityCritical,
			DeviceID:    deviceID,
			MountPoint:  latest.MountPoint,
			Title:       "磁盘使用率过高",
			Description: fmt.Sprintf("磁盘使用率达到 %.1f%%，超过90%%警戒线", latest.UsagePercent),
			Metrics:     latest,
			Suggestions: []string{
				"清理不必要的文件",
				"扩展存储容量",
				"归档旧数据",
			},
			DetectedAt: time.Now(),
		})
	}

	// 检查容量增长速率
	if len(history) >= 2 {
		old := history[0]
		hoursDiff := latest.CollectedAt.Sub(old.CollectedAt).Hours()
		if hoursDiff > 0 {
			oldUsedGB := float64(old.UsedSpace) / (1024 * 1024 * 1024)
			newUsedGB := float64(latest.UsedSpace) / (1024 * 1024 * 1024)
			growthRate := (newUsedGB - oldUsedGB) / oldUsedGB * 100 / (hoursDiff / 24) // 每天增长率

			if growthRate > m.config.CapacityGrowthThreshold {
				events = append(events, AnomalyEvent{
					ID:          generateID(),
					RuleID:      "capacity_growth",
					RuleName:    "容量增长异常",
					Type:        AnomalyTypeCapacityGrowth,
					Severity:    SeverityWarning,
					DeviceID:    deviceID,
					MountPoint:  latest.MountPoint,
					Title:       "容量增长异常",
					Description: fmt.Sprintf("容量日增长率达到 %.1f%%，超过阈值 %.1f%%", growthRate, m.config.CapacityGrowthThreshold),
					Metrics:     latest,
					Suggestions: []string{
						"检查是否有异常的写入操作",
						"检查是否有日志文件过度增长",
						"考虑设置容量告警",
					},
					DetectedAt: time.Now(),
				})
			}
		}
	}

	return events
}

// detectIOPSAnomaly 检测IOPS异常
func (m *AnomalyManager) detectIOPSAnomaly(deviceID string, latest *StorageMetrics, history []*StorageMetrics) []AnomalyEvent {
	events := make([]AnomalyEvent, 0)

	// 计算平均IOPS
	var totalReadIOPS, totalWriteIOPS float64
	for _, h := range history {
		totalReadIOPS += h.ReadIOPS
		totalWriteIOPS += h.WriteIOPS
	}
	avgReadIOPS := totalReadIOPS / float64(len(history))
	avgWriteIOPS := totalWriteIOPS / float64(len(history))

	// 检查当前IOPS是否异常
	if avgReadIOPS > 0 && latest.ReadIOPS > avgReadIOPS*m.config.IOPSSpikeThreshold {
		events = append(events, AnomalyEvent{
			ID:          generateID(),
			RuleID:      "iops_spike",
			RuleName:    "IOPS峰值异常",
			Type:        AnomalyTypeIOPSSpike,
			Severity:    SeverityWarning,
			DeviceID:    deviceID,
			MountPoint:  latest.MountPoint,
			Title:       "读IOPS异常峰值",
			Description: fmt.Sprintf("当前读IOPS %.0f，是平均值 %.0f 的 %.1f 倍", latest.ReadIOPS, avgReadIOPS, latest.ReadIOPS/avgReadIOPS),
			Metrics:     latest,
			Suggestions: []string{
				"检查是否有批量读取操作",
				"检查是否有异常的访问模式",
			},
			DetectedAt: time.Now(),
		})
	}

	if avgWriteIOPS > 0 && latest.WriteIOPS > avgWriteIOPS*m.config.IOPSSpikeThreshold {
		events = append(events, AnomalyEvent{
			ID:          generateID(),
			RuleID:      "iops_spike",
			RuleName:    "IOPS峰值异常",
			Type:        AnomalyTypeIOPSSpike,
			Severity:    SeverityWarning,
			DeviceID:    deviceID,
			MountPoint:  latest.MountPoint,
			Title:       "写IOPS异常峰值",
			Description: fmt.Sprintf("当前写IOPS %.0f，是平均值 %.0f 的 %.1f 倍", latest.WriteIOPS, avgWriteIOPS, latest.WriteIOPS/avgWriteIOPS),
			Metrics:     latest,
			Suggestions: []string{
				"检查是否有批量写入操作",
				"检查是否有数据备份任务",
			},
			DetectedAt: time.Now(),
		})
	}

	return events
}

// detectLatencyAnomaly 检测延迟异常
func (m *AnomalyManager) detectLatencyAnomaly(deviceID string, latest *StorageMetrics, history []*StorageMetrics) []AnomalyEvent {
	events := make([]AnomalyEvent, 0)

	// 计算平均延迟
	var totalReadLatency, totalWriteLatency float64
	count := 0
	for _, h := range history {
		if h.ReadLatency > 0 || h.WriteLatency > 0 {
			totalReadLatency += h.ReadLatency
			totalWriteLatency += h.WriteLatency
			count++
		}
	}

	if count == 0 {
		return events
	}

	avgReadLatency := totalReadLatency / float64(count)
	avgWriteLatency := totalWriteLatency / float64(count)

	// 检查延迟是否异常
	if latest.ReadLatency > m.config.LatencySpikeThreshold {
		events = append(events, AnomalyEvent{
			ID:          generateID(),
			RuleID:      "latency_spike",
			RuleName:    "延迟峰值异常",
			Type:        AnomalyTypeLatencySpike,
			Severity:    SeverityWarning,
			DeviceID:    deviceID,
			MountPoint:  latest.MountPoint,
			Title:       "读延迟异常",
			Description: fmt.Sprintf("读延迟 %.1fms，超过阈值 %.1fms", latest.ReadLatency, m.config.LatencySpikeThreshold),
			Metrics:     latest,
			Suggestions: []string{
				"检查磁盘健康状态",
				"检查是否有其他进程占用IO",
			},
			DetectedAt: time.Now(),
		})
	}

	if latest.WriteLatency > m.config.LatencySpikeThreshold {
		events = append(events, AnomalyEvent{
			ID:          generateID(),
			RuleID:      "latency_spike",
			RuleName:    "延迟峰值异常",
			Type:        AnomalyTypeLatencySpike,
			Severity:    SeverityWarning,
			DeviceID:    deviceID,
			MountPoint:  latest.MountPoint,
			Title:       "写延迟异常",
			Description: fmt.Sprintf("写延迟 %.1fms，超过阈值 %.1fms", latest.WriteLatency, m.config.LatencySpikeThreshold),
			Metrics:     latest,
			Suggestions: []string{
				"检查磁盘健康状态",
				"检查RAID状态",
			},
			DetectedAt: time.Now(),
		})
	}

	// 检查延迟是否显著高于平均值
	if avgReadLatency > 0 && latest.ReadLatency > avgReadLatency*3 {
		events = append(events, AnomalyEvent{
			ID:          generateID(),
			RuleID:      "latency_spike",
			RuleName:    "延迟峰值异常",
			Type:        AnomalyTypeLatencySpike,
			Severity:    SeverityWarning,
			DeviceID:    deviceID,
			MountPoint:  latest.MountPoint,
			Title:       "读延迟显著升高",
			Description: fmt.Sprintf("读延迟 %.1fms，是平均值 %.1fms 的 %.1f 倍", latest.ReadLatency, avgReadLatency, latest.ReadLatency/avgReadLatency),
			Metrics:     latest,
			DetectedAt:  time.Now(),
		})
	}

	if avgWriteLatency > 0 && latest.WriteLatency > avgWriteLatency*3 {
		events = append(events, AnomalyEvent{
			ID:          generateID(),
			RuleID:      "latency_spike",
			RuleName:    "延迟峰值异常",
			Type:        AnomalyTypeLatencySpike,
			Severity:    SeverityWarning,
			DeviceID:    deviceID,
			MountPoint:  latest.MountPoint,
			Title:       "写延迟显著升高",
			Description: fmt.Sprintf("写延迟 %.1fms，是平均值 %.1fms 的 %.1f 倍", latest.WriteLatency, avgWriteLatency, latest.WriteLatency/avgWriteLatency),
			Metrics:     latest,
			DetectedAt:  time.Now(),
		})
	}

	return events
}

// detectDataCorruption 检测数据损坏
func (m *AnomalyManager) detectDataCorruption(deviceID string, latest *StorageMetrics) []AnomalyEvent {
	events := make([]AnomalyEvent, 0)

	if latest.CorruptedFiles > 0 {
		severity := SeverityWarning
		if latest.CorruptedFiles > 10 {
			severity = SeverityCritical
		}

		events = append(events, AnomalyEvent{
			ID:          generateID(),
			RuleID:      "data_corruption",
			RuleName:    "数据损坏检测",
			Type:        AnomalyTypeDataCorruption,
			Severity:    severity,
			DeviceID:    deviceID,
			MountPoint:  latest.MountPoint,
			Title:       "检测到数据损坏",
			Description: fmt.Sprintf("发现 %d 个损坏文件", latest.CorruptedFiles),
			Metrics:     latest,
			Suggestions: []string{
				"立即备份重要数据",
				"检查磁盘健康状态",
				"运行文件系统检查",
				"检查RAID状态",
			},
			DetectedAt: time.Now(),
		})
	}

	if latest.ErrorCount > 0 {
		events = append(events, AnomalyEvent{
			ID:          generateID(),
			RuleID:      "data_corruption",
			RuleName:    "数据损坏检测",
			Type:        AnomalyTypeDataCorruption,
			Severity:    SeverityWarning,
			DeviceID:    deviceID,
			MountPoint:  latest.MountPoint,
			Title:       "存储错误",
			Description: fmt.Sprintf("检测到 %d 次存储错误", latest.ErrorCount),
			Metrics:     latest,
			Suggestions: []string{
				"检查磁盘SMART数据",
				"检查连接线缆",
			},
			DetectedAt: time.Now(),
		})
	}

	return events
}

// generateSummary 生成摘要
func (m *AnomalyManager) generateSummary(events []AnomalyEvent) string {
	if len(events) == 0 {
		return "未发现异常"
	}

	criticalCount := 0
	warningCount := 0
	for _, e := range events {
		switch e.Severity {
		case SeverityCritical, SeverityFatal:
			criticalCount++
		case SeverityWarning:
			warningCount++
		}
	}

	if criticalCount > 0 {
		return fmt.Sprintf("发现 %d 个严重异常和 %d 个警告", criticalCount, warningCount)
	}
	return fmt.Sprintf("发现 %d 个警告", warningCount)
}

// CreateRule 创建规则
func (m *AnomalyManager) CreateRule(req *CreateRuleRequest) (*AnomalyRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("规则名称不能为空")
	}

	rule := &AnomalyRule{
		ID:          generateID(),
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Enabled:     true,
		Severity:    req.Severity,
		Conditions:  req.Conditions,
		Actions:     req.Actions,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.rules[rule.ID] = rule
	return rule, nil
}

// GetRule 获取规则
func (m *AnomalyManager) GetRule(id string) (*AnomalyRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, exists := m.rules[id]
	if !exists {
		return nil, fmt.Errorf("规则不存在: %s", id)
	}

	return rule, nil
}

// ListRules 列出规则
func (m *AnomalyManager) ListRules() []AnomalyRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]AnomalyRule, 0, len(m.rules))
	for _, rule := range m.rules {
		rules = append(rules, *rule)
	}

	return rules
}

// UpdateRule 更新规则
func (m *AnomalyManager) UpdateRule(req *UpdateRuleRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, exists := m.rules[req.ID]
	if !exists {
		return fmt.Errorf("规则不存在: %s", req.ID)
	}

	if req.Name != "" {
		rule.Name = req.Name
	}
	if req.Description != "" {
		rule.Description = req.Description
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if req.Severity != "" {
		rule.Severity = req.Severity
	}
	if req.Conditions != nil {
		rule.Conditions = req.Conditions
	}
	if req.Actions != nil {
		rule.Actions = req.Actions
	}
	rule.UpdatedAt = time.Now()

	return nil
}

// DeleteRule 删除规则
func (m *AnomalyManager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[id]; !exists {
		return fmt.Errorf("规则不存在: %s", id)
	}

	delete(m.rules, id)
	return nil
}

// GetEvent 获取事件
func (m *AnomalyManager) GetEvent(id string) (*AnomalyEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, event := range m.events {
		if event.ID == id {
			return event, nil
		}
	}

	return nil, fmt.Errorf("事件不存在: %s", id)
}

// ListEvents 列出事件
func (m *AnomalyManager) ListEvents(deviceID string, severity AnomalySeverity, limit int) []AnomalyEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := make([]AnomalyEvent, 0)
	for _, event := range m.events {
		if deviceID != "" && event.DeviceID != deviceID {
			continue
		}
		if severity != "" && event.Severity != severity {
			continue
		}
		events = append(events, *event)
	}

	// 按时间倒序
	for i := 0; i < len(events)/2; i++ {
		j := len(events) - 1 - i
		events[i], events[j] = events[j], events[i]
	}

	if limit > 0 && len(events) > limit {
		events = events[:limit]
	}

	return events
}

// AckEvent 确认事件
func (m *AnomalyManager) AckEvent(eventID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, event := range m.events {
		if event.ID == eventID {
			now := time.Now()
			event.AckedAt = &now
			event.AckedBy = userID
			return nil
		}
	}

	return fmt.Errorf("事件不存在: %s", eventID)
}

// ResolveEvent 解决事件
func (m *AnomalyManager) ResolveEvent(eventID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, event := range m.events {
		if event.ID == eventID {
			event.Resolved = true
			now := time.Now()
			event.ResolvedAt = &now
			return nil
		}
	}

	return fmt.Errorf("事件不存在: %s", eventID)
}

// GetStats 获取统计信息
func (m *AnomalyManager) GetStats() *AnomalyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &AnomalyStats{}

	for _, event := range m.events {
		stats.TotalEvents++
		if event.AckedAt == nil {
			stats.UnackedEvents++
		}
		if !event.Resolved {
			stats.UnresolvedEvents++
		}
		switch event.Severity {
		case SeverityCritical, SeverityFatal:
			stats.CriticalEvents++
		case SeverityWarning:
			stats.WarningEvents++
		}
	}

	for _, rule := range m.rules {
		if rule.Enabled {
			stats.ActiveRules++
		}
	}

	return stats
}

// addEvent 添加事件
func (m *AnomalyManager) addEvent(event *AnomalyEvent) {
	// 检查冷却时间
	_, exists := m.rules[event.RuleID]
	if exists {
		lastAlert, hasLast := m.lastAlerts[event.RuleID]
		if hasLast && time.Since(lastAlert) < m.config.AlertCooldown {
			return
		}

		// 检查每小时告警数
		count := m.alertCounts[event.RuleID]
		if count >= m.config.MaxAlertsPerHour {
			return
		}
	}

	m.events = append(m.events, event)
	m.lastAlerts[event.RuleID] = time.Now()
	m.alertCounts[event.RuleID]++
}

// ResetAlertCounts 重置告警计数
func (m *AnomalyManager) ResetAlertCounts() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.alertCounts = make(map[string]int)
}

// generateID 生成随机ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// calculateMean 计算平均值
func calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// calculateStdDev 计算标准差
func calculateStdDev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	mean := calculateMean(values)
	sumSquares := 0.0
	for _, v := range values {
		diff := v - mean
		sumSquares += diff * diff
	}
	return math.Sqrt(sumSquares / float64(len(values)))
}
