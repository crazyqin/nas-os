// Package ailoganalyzer 提供 AI 日志分析器核心业务逻辑
package ailoganalyzer

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager AI 日志分析器管理器.
type Manager struct {
	logs      map[string]*LogEntry
	patterns  map[string]*LogPattern
	rules     map[string]*AnomalyRule
	alerts    map[string]*Alert
	clusters  map[string]*LogCluster
	streams   map[string]*LogStream
	policies  map[string]*RetentionPolicy
	analyses  map[string]*RootCauseAnalysis
	mu        sync.RWMutex
	stopCh    map[string]chan struct{} // 停止流的通道
}

// NewManager 创建 AI 日志分析器管理器.
func NewManager() *Manager {
	return &Manager{
		logs:     make(map[string]*LogEntry),
		patterns: make(map[string]*LogPattern),
		rules:    make(map[string]*AnomalyRule),
		alerts:   make(map[string]*Alert),
		clusters: make(map[string]*LogCluster),
		streams:  make(map[string]*LogStream),
		policies: make(map[string]*RetentionPolicy),
		analyses: make(map[string]*RootCauseAnalysis),
		stopCh:   make(map[string]chan struct{}),
	}
}

// ========== 日志管理 ==========

// AddLog 添加日志条目.
func (m *Manager) AddLog(entry *LogEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	if entry.Labels == nil {
		entry.Labels = make(map[string]string)
	}
	if entry.Metadata == nil {
		entry.Metadata = make(map[string]interface{})
	}

	// 模式匹配
	m.matchPattern(entry)

	// 聚类
	m.clusterLog(entry)

	// 异常检测
	m.detectAnomaly(entry)

	m.logs[entry.ID] = entry
}

// GetLog 获取日志条目.
func (m *Manager) GetLog(id string) (*LogEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	log, ok := m.logs[id]
	if !ok {
		return nil, ErrLogNotFound
	}
	return log, nil
}

// QueryLogs 查询日志.
func (m *Manager) QueryLogs(req QueryLogsRequest) ([]*LogEntry, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 默认分页
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 50
	}
	if req.PageSize > 1000 {
		req.PageSize = 1000
	}

	var result []*LogEntry
	for _, log := range m.logs {
		// 时间范围过滤
		if req.StartTime != nil && log.Timestamp.Before(*req.StartTime) {
			continue
		}
		if req.EndTime != nil && log.Timestamp.After(*req.EndTime) {
			continue
		}

		// 级别过滤
		if req.Level != "" && log.Level != req.Level {
			continue
		}

		// 来源过滤
		if req.Source != "" && log.Source != req.Source {
			continue
		}

		// 关键词过滤
		if req.Keyword != "" && !strings.Contains(strings.ToLower(log.Message), strings.ToLower(req.Keyword)) {
			continue
		}

		// 正则过滤
		if req.Regex != "" {
			matched, err := regexp.MatchString(req.Regex, log.Message)
			if err != nil || !matched {
				continue
			}
		}

		// 模式ID过滤
		if req.PatternID != "" && log.PatternID != req.PatternID {
			continue
		}

		// 聚类ID过滤
		if req.ClusterID != "" && log.ClusterID != req.ClusterID {
			continue
		}

		result = append(result, log)
	}

	// 按时间倒序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	total := len(result)
	start := (req.Page - 1) * req.PageSize
	if start >= total {
		return nil, total
	}
	end := start + req.PageSize
	if end > total {
		end = total
	}

	return result[start:end], total
}

// DeleteLogs 删除指定时间之前的日志.
func (m *Manager) DeleteLogs(before time.Time) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for id, log := range m.logs {
		if log.Timestamp.Before(before) {
			delete(m.logs, id)
			count++
		}
	}
	return count
}

// ========== 模式识别 ==========

// CreatePattern 创建日志模式.
func (m *Manager) CreatePattern(req CreatePatternRequest) *LogPattern {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	pattern := &LogPattern{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Regex:       req.Regex,
		Keywords:    req.Keywords,
		Level:       req.Level,
		IsAnomaly:   req.IsAnomaly,
		Severity:    req.Severity,
		Enabled:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if pattern.Keywords == nil {
		pattern.Keywords = []string{}
	}

	m.patterns[pattern.ID] = pattern
	return pattern
}

// GetPattern 获取日志模式.
func (m *Manager) GetPattern(id string) (*LogPattern, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pattern, ok := m.patterns[id]
	if !ok {
		return nil, ErrRuleNotFound
	}
	return pattern, nil
}

// ListPatterns 列出所有日志模式.
func (m *Manager) ListPatterns() []*LogPattern {
	m.mu.RLock()
	defer m.mu.RUnlock()

	patterns := make([]*LogPattern, 0, len(m.patterns))
	for _, p := range m.patterns {
		patterns = append(patterns, p)
	}

	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].CreatedAt.After(patterns[j].CreatedAt)
	})

	return patterns
}

// UpdatePattern 更新日志模式.
func (m *Manager) UpdatePattern(id string, req UpdatePatternRequest) (*LogPattern, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pattern, ok := m.patterns[id]
	if !ok {
		return nil, ErrRuleNotFound
	}

	if req.Name != nil {
		pattern.Name = *req.Name
	}
	if req.Description != nil {
		pattern.Description = *req.Description
	}
	if req.Regex != nil {
		pattern.Regex = *req.Regex
	}
	if req.Keywords != nil {
		pattern.Keywords = req.Keywords
	}
	if req.Level != nil {
		pattern.Level = *req.Level
	}
	if req.IsAnomaly != nil {
		pattern.IsAnomaly = *req.IsAnomaly
	}
	if req.Severity != nil {
		pattern.Severity = *req.Severity
	}
	if req.Enabled != nil {
		pattern.Enabled = *req.Enabled
	}

	pattern.UpdatedAt = time.Now()
	return pattern, nil
}

// DeletePattern 删除日志模式.
func (m *Manager) DeletePattern(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.patterns[id]; !ok {
		return ErrRuleNotFound
	}
	delete(m.patterns, id)
	return nil
}

// matchPattern 匹配日志模式（内部方法，调用时已持有锁）.
func (m *Manager) matchPattern(entry *LogEntry) {
	for _, pattern := range m.patterns {
		if !pattern.Enabled {
			continue
		}

		// 级别匹配
		if pattern.Level != "" && entry.Level != pattern.Level {
			continue
		}

		matched := false

		// 正则匹配
		if pattern.Regex != "" {
			re, err := regexp.Compile(pattern.Regex)
			if err == nil && re.MatchString(entry.Message) {
				matched = true
			}
		}

		// 关键词匹配
		if !matched && len(pattern.Keywords) > 0 {
			msgLower := strings.ToLower(entry.Message)
			for _, kw := range pattern.Keywords {
				if strings.Contains(msgLower, strings.ToLower(kw)) {
					matched = true
					break
				}
			}
		}

		if matched {
			entry.PatternID = pattern.ID
			if pattern.IsAnomaly {
				entry.Metadata["is_anomaly"] = true
				entry.Metadata["severity"] = pattern.Severity
			}
			break
		}
	}
}

// ========== 异常检测 ==========

// CreateRule 创建异常检测规则.
func (m *Manager) CreateRule(req CreateRuleRequest) *AnomalyRule {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	rule := &AnomalyRule{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Threshold:   req.Threshold,
		Window:      time.Duration(req.Window) * time.Second,
		Level:       req.Level,
		PatternID:   req.PatternID,
		TimeStart:   req.TimeStart,
		TimeEnd:     req.TimeEnd,
		Enabled:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	m.rules[rule.ID] = rule
	return rule
}

// GetRule 获取异常检测规则.
func (m *Manager) GetRule(id string) (*AnomalyRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, ok := m.rules[id]
	if !ok {
		return nil, ErrRuleNotFound
	}
	return rule, nil
}

// ListRules 列出所有异常检测规则.
func (m *Manager) ListRules() []*AnomalyRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*AnomalyRule, 0, len(m.rules))
	for _, r := range m.rules {
		rules = append(rules, r)
	}

	sort.Slice(rules, func(i, j int) bool {
		return rules[i].CreatedAt.After(rules[j].CreatedAt)
	})

	return rules
}

// UpdateRule 更新异常检测规则.
func (m *Manager) UpdateRule(id string, req UpdateRuleRequest) (*AnomalyRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, ok := m.rules[id]
	if !ok {
		return nil, ErrRuleNotFound
	}

	if req.Name != nil {
		rule.Name = *req.Name
	}
	if req.Description != nil {
		rule.Description = *req.Description
	}
	if req.Threshold != nil {
		rule.Threshold = *req.Threshold
	}
	if req.Window != nil {
		rule.Window = time.Duration(*req.Window) * time.Second
	}
	if req.Level != nil {
		rule.Level = *req.Level
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}

	rule.UpdatedAt = time.Now()
	return rule, nil
}

// DeleteRule 删除异常检测规则.
func (m *Manager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.rules[id]; !ok {
		return ErrRuleNotFound
	}
	delete(m.rules, id)
	return nil
}

// detectAnomaly 异常检测（内部方法，调用时已持有锁）.
func (m *Manager) detectAnomaly(entry *LogEntry) {
	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}

		triggered := false

		switch rule.Type {
		case "frequency":
			// 频率突增检测
			triggered = m.detectFrequencyAnomaly(entry, rule)
		case "pattern":
			// 模式匹配检测
			triggered = entry.PatternID == rule.PatternID
		case "time":
			// 异常时间段检测
			triggered = m.detectTimeAnomaly(entry, rule)
		}

		if triggered {
			m.createOrUpdateAlert(rule, entry)
		}
	}
}

// detectFrequencyAnomaly 检测频率突增.
func (m *Manager) detectFrequencyAnomaly(entry *LogEntry, rule *AnomalyRule) bool {
	if rule.Window <= 0 || rule.Threshold <= 0 {
		return false
	}

	count := 0
	windowStart := time.Now().Add(-rule.Window)

	for _, log := range m.logs {
		if log.Timestamp.Before(windowStart) {
			continue
		}
		if rule.Level != "" && log.Level != rule.Level {
			continue
		}
		if rule.PatternID != "" && log.PatternID != rule.PatternID {
			continue
		}
		count++
	}

	return count >= rule.Threshold
}

// detectTimeAnomaly 检测异常时间段.
func (m *Manager) detectTimeAnomaly(entry *LogEntry, rule *AnomalyRule) bool {
	if rule.TimeStart == "" || rule.TimeEnd == "" {
		return false
	}

	now := entry.Timestamp
	currentTime := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())

	// 判断当前时间是否在异常时间段内
	return currentTime >= rule.TimeStart && currentTime <= rule.TimeEnd
}

// createOrUpdateAlert 创建或更新告警.
func (m *Manager) createOrUpdateAlert(rule *AnomalyRule, entry *LogEntry) {
	// 查找同一规则的活跃告警
	for _, alert := range m.alerts {
		if alert.RuleID == rule.ID && alert.Status == AlertStatusActive {
			alert.Count++
			alert.LastSeen = time.Now()
			alert.LogIDs = append(alert.LogIDs, entry.ID)
			return
		}
	}

	// 创建新告警
	alert := &Alert{
		ID:        uuid.New().String(),
		RuleID:    rule.ID,
		RuleName:  rule.Name,
		Level:     AlertLevelMedium,
		Status:    AlertStatusActive,
		Message:   fmt.Sprintf("规则 [%s] 触发告警: %s", rule.Name, entry.Message),
		LogIDs:    []string{entry.ID},
		Count:     1,
		FirstSeen: time.Now(),
		LastSeen:  time.Now(),
	}

	// 根据规则类型设置告警级别
	switch rule.Type {
	case "frequency":
		alert.Level = AlertLevelHigh
	case "time":
		alert.Level = AlertLevelMedium
	}

	m.alerts[alert.ID] = alert
}

// ========== 告警管理 ==========

// GetAlert 获取告警.
func (m *Manager) GetAlert(id string) (*Alert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alert, ok := m.alerts[id]
	if !ok {
		return nil, ErrAlertNotFound
	}
	return alert, nil
}

// ListAlerts 列出所有告警.
func (m *Manager) ListAlerts(status string) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]*Alert, 0, len(m.alerts))
	for _, a := range m.alerts {
		if status != "" && a.Status != status {
			continue
		}
		alerts = append(alerts, a)
	}

	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].LastSeen.After(alerts[j].LastSeen)
	})

	return alerts
}

// UpdateAlert 更新告警状态.
func (m *Manager) UpdateAlert(id string, req UpdateAlertRequest) (*Alert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[id]
	if !ok {
		return nil, ErrAlertNotFound
	}

	if req.Status != nil {
		alert.Status = *req.Status
		if *req.Status == AlertStatusResolved {
			now := time.Now()
			alert.ResolvedAt = &now
		}
	}
	if req.Notes != nil {
		alert.Notes = *req.Notes
	}

	return alert, nil
}

// DeleteAlert 删除告警.
func (m *Manager) DeleteAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.alerts[id]; !ok {
		return ErrAlertNotFound
	}
	delete(m.alerts, id)
	return nil
}

// ========== 日志聚类 ==========

// clusterLog 日志聚类（内部方法，调用时已持有锁）.
func (m *Manager) clusterLog(entry *LogEntry) {
	// 简化的聚类：基于消息模板（移除数字和变量）
	template := m.extractTemplate(entry.Message)

	for _, cluster := range m.clusters {
		if cluster.Pattern == template {
			cluster.Count++
			cluster.LastSeen = time.Now()
			if len(cluster.SampleIDs) < 5 {
				cluster.SampleIDs = append(cluster.SampleIDs, entry.ID)
			}
			entry.ClusterID = cluster.ID
			return
		}
	}

	// 创建新聚类
	cluster := &LogCluster{
		ID:        uuid.New().String(),
		Pattern:   template,
		Count:     1,
		FirstSeen: time.Now(),
		LastSeen:  time.Now(),
		SampleIDs: []string{entry.ID},
		Level:     entry.Level,
		Source:    entry.Source,
	}

	m.clusters[cluster.ID] = cluster
	entry.ClusterID = cluster.ID
}

// extractTemplate 提取消息模板.
func (m *Manager) extractTemplate(message string) string {
	// 移除数字
	re := regexp.MustCompile(`\d+`)
	template := re.ReplaceAllString(message, "*")
	// 移除常见变量模式
	re2 := regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	template = re2.ReplaceAllString(template, "*")
	// 移除IP地址
	re3 := regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
	template = re3.ReplaceAllString(template, "*")
	return template
}

// ListClusters 列出所有聚类.
func (m *Manager) ListClusters() []*LogCluster {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clusters := make([]*LogCluster, 0, len(m.clusters))
	for _, c := range m.clusters {
		clusters = append(clusters, c)
	}

	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Count > clusters[j].Count
	})

	return clusters
}

// GetCluster 获取聚类详情.
func (m *Manager) GetCluster(id string) (*LogCluster, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cluster, ok := m.clusters[id]
	if !ok {
		return nil, fmt.Errorf("cluster %s not found", id)
	}
	return cluster, nil
}

// ========== 根因分析 ==========

// AnalyzeRootCause 执行根因分析.
func (m *Manager) AnalyzeRootCause(alertID string) (*RootCauseAnalysis, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[alertID]
	if !ok {
		return nil, ErrAlertNotFound
	}

	// 收集相关日志
	relatedLogs := make([]*LogEntry, 0, len(alert.LogIDs))
	for _, logID := range alert.LogIDs {
		if log, exists := m.logs[logID]; exists {
			relatedLogs = append(relatedLogs, log)
		}
	}

	// 构建时间线
	timeline := make([]TimelineEntry, 0, len(relatedLogs))
	for _, log := range relatedLogs {
		timeline = append(timeline, TimelineEntry{
			Timestamp: log.Timestamp,
			Event:     log.Message,
			LogID:     log.ID,
			Level:     log.Level,
		})
	}

	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Timestamp.Before(timeline[j].Timestamp)
	})

	// 分析根因
	rootCause := m.analyzeRootCauseFromLogs(relatedLogs)
	suggestions := m.generateSuggestions(alert, relatedLogs)

	analysis := &RootCauseAnalysis{
		ID:          uuid.New().String(),
		AlertID:     alertID,
		RootCause:   rootCause,
		Timeline:    timeline,
		RelatedLogs: alert.LogIDs,
		Suggestions: suggestions,
		CreatedAt:   time.Now(),
	}

	m.analyses[analysis.ID] = analysis
	return analysis, nil
}

// analyzeRootCauseFromLogs 从日志中分析根因.
func (m *Manager) analyzeRootCauseFromLogs(logs []*LogEntry) string {
	if len(logs) == 0 {
		return "无法确定根因：无相关日志"
	}

	// 统计错误来源
	sourceCount := make(map[string]int)
	levelCount := make(map[string]int)
	for _, log := range logs {
		sourceCount[log.Source]++
		levelCount[log.Level]++
	}

	// 找出主要来源
	var mainSource string
	maxCount := 0
	for source, count := range sourceCount {
		if count > maxCount {
			maxCount = count
			mainSource = source
		}
	}

	// 根据级别和来源生成根因描述
	if levelCount[LevelError] > 0 || levelCount[LevelFatal] > 0 {
		return fmt.Sprintf("来自 %s 的 %d 个错误日志表明系统异常", mainSource, levelCount[LevelError]+levelCount[LevelFatal])
	}
	if levelCount[LevelWarn] > 0 {
		return fmt.Sprintf("来自 %s 的 %d 个警告日志表明潜在问题", mainSource, levelCount[LevelWarn])
	}

	return fmt.Sprintf("来自 %s 的日志异常，共 %d 条", mainSource, len(logs))
}

// generateSuggestions 生成建议.
func (m *Manager) generateSuggestions(alert *Alert, logs []*LogEntry) []string {
	suggestions := []string{}

	if alert.Level == AlertLevelCritical || alert.Level == AlertLevelHigh {
		suggestions = append(suggestions, "立即检查系统状态和服务可用性")
		suggestions = append(suggestions, "检查相关服务的资源使用情况（CPU/内存/磁盘）")
	}

	if len(logs) > 10 {
		suggestions = append(suggestions, "日志量较大，建议检查是否有日志风暴")
	}

	suggestions = append(suggestions, "检查告警规则的阈值设置是否合理")
	suggestions = append(suggestions, "查看完整的日志上下文以了解问题全貌")

	return suggestions
}

// GetAnalysis 获取根因分析结果.
func (m *Manager) GetAnalysis(id string) (*RootCauseAnalysis, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	analysis, ok := m.analyses[id]
	if !ok {
		return nil, fmt.Errorf("analysis %s not found", id)
	}
	return analysis, nil
}

// ListAnalyses 列出所有根因分析结果.
func (m *Manager) ListAnalyses() []*RootCauseAnalysis {
	m.mu.RLock()
	defer m.mu.RUnlock()

	analyses := make([]*RootCauseAnalysis, 0, len(m.analyses))
	for _, a := range m.analyses {
		analyses = append(analyses, a)
	}

	sort.Slice(analyses, func(i, j int) bool {
		return analyses[i].CreatedAt.After(analyses[j].CreatedAt)
	})

	return analyses
}

// ========== 日志流 ==========

// CreateStream 创建日志流.
func (m *Manager) CreateStream(req CreateStreamRequest) *LogStream {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream := &LogStream{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Source:    req.Source,
		Enabled:   true,
		Running:   false,
		CreatedAt: time.Now(),
	}

	m.streams[stream.ID] = stream
	return stream
}

// GetStream 获取日志流.
func (m *Manager) GetStream(id string) (*LogStream, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stream, ok := m.streams[id]
	if !ok {
		return nil, ErrStreamNotFound
	}
	return stream, nil
}

// ListStreams 列出所有日志流.
func (m *Manager) ListStreams() []*LogStream {
	m.mu.RLock()
	defer m.mu.RUnlock()

	streams := make([]*LogStream, 0, len(m.streams))
	for _, s := range m.streams {
		streams = append(streams, s)
	}
	return streams
}

// StartStream 启动日志流.
func (m *Manager) StartStream(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream, ok := m.streams[id]
	if !ok {
		return ErrStreamNotFound
	}

	if stream.Running {
		return ErrStreamAlreadyRunning
	}

	stream.Running = true
	stopCh := make(chan struct{})
	m.stopCh[id] = stopCh

	// 模拟日志流采集（实际应用中会 tail -f 文件）
	go m.simulateLogStream(stream, stopCh)

	return nil
}

// StopStream 停止日志流.
func (m *Manager) StopStream(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream, ok := m.streams[id]
	if !ok {
		return ErrStreamNotFound
	}

	if !stream.Running {
		return nil
	}

	if stopCh, exists := m.stopCh[id]; exists {
		close(stopCh)
		delete(m.stopCh, id)
	}

	stream.Running = false
	return nil
}

// DeleteStream 删除日志流.
func (m *Manager) DeleteStream(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream, ok := m.streams[id]
	if !ok {
		return ErrStreamNotFound
	}

	// 如果正在运行，先停止
	if stream.Running {
		if stopCh, exists := m.stopCh[id]; exists {
			close(stopCh)
			delete(m.stopCh, id)
		}
	}

	delete(m.streams, id)
	return nil
}

// simulateLogStream 模拟日志流采集.
func (m *Manager) simulateLogStream(stream *LogStream, stopCh chan struct{}) {
	// 实际应用中这里会实现 tail -f 逻辑
	// 这里只是模拟
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			// 模拟日志写入
			entry := &LogEntry{
				Source:  stream.Source,
				Level:   LevelInfo,
				Message: fmt.Sprintf("Stream %s heartbeat", stream.Name),
			}
			m.AddLog(entry)
		}
	}
}

// ========== 保留策略 ==========

// CreateRetentionPolicy 创建保留策略.
func (m *Manager) CreateRetentionPolicy(req CreateRetentionPolicyRequest) *RetentionPolicy {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy := &RetentionPolicy{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Level:       req.Level,
		Source:      req.Source,
		MaxAge:      req.MaxAgeDays,
		MaxCount:    req.MaxCount,
		ArchivePath: req.ArchivePath,
		Enabled:     true,
		CreatedAt:   time.Now(),
	}

	m.policies[policy.ID] = policy
	return policy
}

// ListRetentionPolicies 列出所有保留策略.
func (m *Manager) ListRetentionPolicies() []*RetentionPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*RetentionPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// DeleteRetentionPolicy 删除保留策略.
func (m *Manager) DeleteRetentionPolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[id]; !ok {
		return fmt.Errorf("policy %s not found", id)
	}
	delete(m.policies, id)
	return nil
}

// ApplyRetentionPolicies 应用保留策略.
func (m *Manager) ApplyRetentionPolicies() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	totalDeleted := 0
	now := time.Now()

	for _, policy := range m.policies {
		if !policy.Enabled {
			continue
		}

		deleted := 0
		for id, log := range m.logs {
			// 按级别过滤
			if policy.Level != "" && log.Level != policy.Level {
				continue
			}
			// 按来源过滤
			if policy.Source != "" && log.Source != policy.Source {
				continue
			}
			// 按时间过滤
			if policy.MaxAge > 0 {
				maxAge := time.Duration(policy.MaxAge) * 24 * time.Hour
				if now.Sub(log.Timestamp) > maxAge {
					delete(m.logs, id)
					deleted++
				}
			}
		}

		totalDeleted += deleted
	}

	return totalDeleted
}

// ========== 统计 ==========

// GetStats 获取日志统计.
func (m *Manager) GetStats(req StatsQueryRequest) *LogStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &LogStats{
		LogsByLevel:  make(map[string]int),
		LogsBySource: make(map[string]int),
		TopErrors:    make([]PatternStat, 0),
		TrendData:    make([]TrendPoint, 0),
	}

	// 按时间过滤
	var filteredLogs []*LogEntry
	for _, log := range m.logs {
		if req.StartTime != nil && log.Timestamp.Before(*req.StartTime) {
			continue
		}
		if req.EndTime != nil && log.Timestamp.After(*req.EndTime) {
			continue
		}
		if req.Source != "" && log.Source != req.Source {
			continue
		}
		filteredLogs = append(filteredLogs, log)
	}

	stats.TotalLogs = len(filteredLogs)

	// 统计各级别数量
	errorCount := 0
	for _, log := range filteredLogs {
		stats.LogsByLevel[log.Level]++
		stats.LogsBySource[log.Source]++
		if log.Level == LevelError || log.Level == LevelFatal {
			errorCount++
		}
	}

	// 计算错误率
	if stats.TotalLogs > 0 {
		stats.ErrorRate = float64(errorCount) / float64(stats.TotalLogs) * 100
	}

	// Top 错误模式
	patternCount := make(map[string]int)
	patternLastSeen := make(map[string]time.Time)
	for _, log := range filteredLogs {
		if log.PatternID != "" && (log.Level == LevelError || log.Level == LevelFatal) {
			patternCount[log.PatternID]++
			if log.Timestamp.After(patternLastSeen[log.PatternID]) {
				patternLastSeen[log.PatternID] = log.Timestamp
			}
		}
	}

	for patternID, count := range patternCount {
		patternName := patternID
		if p, ok := m.patterns[patternID]; ok {
			patternName = p.Name
		}
		stats.TopErrors = append(stats.TopErrors, PatternStat{
			PatternID:   patternID,
			PatternName: patternName,
			Count:       count,
			LastSeen:    patternLastSeen[patternID],
		})
	}

	sort.Slice(stats.TopErrors, func(i, j int) bool {
		return stats.TopErrors[i].Count > stats.TopErrors[j].Count
	})

	if len(stats.TopErrors) > 10 {
		stats.TopErrors = stats.TopErrors[:10]
	}

	// 趋势数据（简化版）
	stats.TrendData = m.generateTrendData(filteredLogs, req.Interval)

	return stats
}

// generateTrendData 生成趋势数据.
func (m *Manager) generateTrendData(logs []*LogEntry, interval string) []TrendPoint {
	if len(logs) == 0 {
		return nil
	}

	// 按时间排序
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Timestamp.Before(logs[j].Timestamp)
	})

	// 简化：按小时分组
	buckets := make(map[int64]*TrendPoint)
	for _, log := range logs {
		var key int64
		switch interval {
		case "minute":
			key = log.Timestamp.Unix() / 60
		case "hour":
			key = log.Timestamp.Unix() / 3600
		case "day":
			key = log.Timestamp.Unix() / 86400
		default:
			key = log.Timestamp.Unix() / 3600
		}

		if _, ok := buckets[key]; !ok {
			buckets[key] = &TrendPoint{
				Timestamp: log.Timestamp.Truncate(time.Hour),
			}
		}
		buckets[key].Count++
		if log.Level == LevelError || log.Level == LevelFatal {
			buckets[key].Errors++
		}
	}

	result := make([]TrendPoint, 0, len(buckets))
	for _, p := range buckets {
		result = append(result, *p)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	return result
}

// RunAnalysis 执行综合分析.
func (m *Manager) RunAnalysis(req StatsQueryRequest) *AnalysisResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := &AnalysisResult{}

	// 过滤日志
	var filteredLogs []*LogEntry
	for _, log := range m.logs {
		if req.StartTime != nil && log.Timestamp.Before(*req.StartTime) {
			continue
		}
		if req.EndTime != nil && log.Timestamp.After(*req.EndTime) {
			continue
		}
		if req.Source != "" && log.Source != req.Source {
			continue
		}
		filteredLogs = append(filteredLogs, log)
	}

	result.TotalLogs = len(filteredLogs)

	// 统计异常
	for _, log := range filteredLogs {
		if isAnomaly, ok := log.Metadata["is_anomaly"].(bool); ok && isAnomaly {
			result.Anomalies++
		}
	}

	// 统计模式
	patternCount := make(map[string]int)
	for _, log := range filteredLogs {
		if log.PatternID != "" {
			patternCount[log.PatternID]++
		}
	}
	for patternID, count := range patternCount {
		patternName := patternID
		if p, ok := m.patterns[patternID]; ok {
			patternName = p.Name
		}
		result.Patterns = append(result.Patterns, PatternStat{
			PatternID:   patternID,
			PatternName: patternName,
			Count:       count,
		})
	}

	// 聚类信息
	clusterCount := make(map[string]int)
	for _, log := range filteredLogs {
		if log.ClusterID != "" {
			clusterCount[log.ClusterID]++
		}
	}
	for clusterID, count := range clusterCount {
		if cluster, ok := m.clusters[clusterID]; ok {
			clusterCopy := *cluster
			clusterCopy.Count = count
			result.Clusters = append(result.Clusters, clusterCopy)
		}
	}

	// 生成摘要
	result.Summary = fmt.Sprintf(
		"分析期间共 %d 条日志，发现 %d 个异常，%d 种模式，%d 个聚类",
		result.TotalLogs, result.Anomalies, len(result.Patterns), len(result.Clusters),
	)

	return result
}
