package healthprobe

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 健康探针管理器
type Manager struct {
	mu         sync.RWMutex
	logger     *zap.Logger
	config     *Config
	probes     map[string]Probe
	rules      map[string]*Rule
	notifiers  []Notifier
	alerts     []*Alert
	history    []*HistoryRecord
	lastStatus *HealthStatus
	startTime  time.Time
	running    bool
	stopCh     chan struct{}
}

// NewManager 创建健康探针管理器
func NewManager(logger *zap.Logger, config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		logger:    logger,
		config:    config,
		probes:    make(map[string]Probe),
		rules:     make(map[string]*Rule),
		notifiers: make([]Notifier, 0),
		alerts:    make([]*Alert, 0),
		history:   make([]*HistoryRecord, 0),
		startTime: time.Now(),
		stopCh:    make(chan struct{}),
	}
}

// RegisterProbe 注册探针
func (m *Manager) RegisterProbe(probe Probe) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.probes[probe.Name()] = probe
	m.logger.Info("注册健康探针",
		zap.String("name", probe.Name()),
		zap.String("type", string(probe.Type())),
		zap.String("category", string(probe.Category())),
	)
}

// UnregisterProbe 注销探针
func (m *Manager) UnregisterProbe(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.probes, name)
	m.logger.Info("注销健康探针", zap.String("name", name))
}

// AddRule 添加检查规则
func (m *Manager) AddRule(rule *Rule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rule.Weight <= 0 {
		rule.Weight = 1.0
	}
	m.rules[rule.Name] = rule
	m.logger.Info("添加健康规则",
		zap.String("name", rule.Name),
		zap.String("type", string(rule.Type)),
	)
}

// RemoveRule 移除检查规则
func (m *Manager) RemoveRule(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rules, name)
}

// AddNotifier 添加告警通知器
func (m *Manager) AddNotifier(notifier Notifier) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifiers = append(m.notifiers, notifier)
}

// Start 启动定期检测
func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopCh = make(chan struct{})
	m.mu.Unlock()

	m.logger.Info("启动健康探针管理器", zap.Duration("interval", m.config.Interval))
	go m.run(ctx)
}

// Stop 停止定期检测
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopCh)
	m.logger.Info("停止健康探针管理器")
}

// IsRunning 是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// run 定期检测循环
func (m *Manager) run(ctx context.Context) {
	ticker := time.NewTicker(m.config.Interval)
	defer ticker.Stop()

	// 立即执行一次
	m.Check(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.Check(ctx)
		}
	}
}

// Check 执行所有探针检测并聚合状态
func (m *Manager) Check(ctx context.Context) *HealthStatus {
	m.mu.RLock()
	probeList := make([]Probe, 0, len(m.probes))
	for _, p := range m.probes {
		probeList = append(probeList, p)
	}
	m.mu.RUnlock()

	status := &HealthStatus{
		Timestamp: time.Now(),
		Uptime:    time.Since(m.startTime),
		Probes:    make(map[string]*ProbeResult),
		Summary:   &StatusSummary{},
		Metadata:  make(map[string]interface{}),
	}

	// 并发执行所有探针
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, probe := range probeList {
		wg.Add(1)
		go func(p Probe) {
			defer wg.Done()
			result := m.runProbe(ctx, p)

			mu.Lock()
			status.Probes[p.Name()] = result
			status.Summary.Total++
			switch result.Level {
			case LevelHealthy:
				status.Summary.Healthy++
			case LevelDegraded:
				status.Summary.Degraded++
			case LevelCritical:
				status.Summary.Critical++
			default:
				status.Summary.Unknown++
			}
			mu.Unlock()
		}(probe)
	}

	wg.Wait()

	// 应用规则评估
	m.evaluateRules(status)

	// 计算健康评分
	status.Score = m.calculateScore(status)

	// 确定总体健康级别
	status.Level = m.determineLevel(status)

	// 记录历史
	m.recordHistory(status)

	// 趋势分析
	if m.config.EnableTrend {
		status.Trend = m.analyzeTrend()
	}

	// 检查告警
	m.checkAlerts(ctx, status)

	// 更新最后状态
	m.mu.Lock()
	m.lastStatus = status
	m.mu.Unlock()

	m.logger.Debug("健康检查完成",
		zap.String("level", string(status.Level)),
		zap.Float64("score", status.Score),
		zap.Int("probes", status.Summary.Total),
	)

	return status
}

// runProbe 执行单个探针
func (m *Manager) runProbe(ctx context.Context, probe Probe) *ProbeResult {
	probeCtx, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	start := time.Now()
	result, err := probe.Collect(probeCtx)
	duration := time.Since(start)

	if result == nil {
		result = &ProbeResult{
			Name:      probe.Name(),
			Category:  probe.Category(),
			Type:      probe.Type(),
			Level:     LevelUnknown,
			Timestamp: time.Now(),
			Duration:  duration,
			Details:   make(map[string]interface{}),
		}
	}

	if err != nil {
		result.Level = LevelCritical
		result.Message = fmt.Sprintf("探针执行失败: %v", err)
		if result.Details == nil {
			result.Details = make(map[string]interface{})
		}
		result.Details["error"] = err.Error()
	}

	result.Duration = duration
	if result.Timestamp.IsZero() {
		result.Timestamp = time.Now()
	}

	return result
}

// evaluateRules 应用规则评估
func (m *Manager) evaluateRules(status *HealthStatus) {
	m.mu.RLock()
	rules := make([]*Rule, 0, len(m.rules))
	for _, r := range m.rules {
		if r.Enabled {
			rules = append(rules, r)
		}
	}
	m.mu.RUnlock()

	for _, rule := range rules {
		// 查找匹配类型的探针
		for _, probe := range status.Probes {
			if probe.Type == rule.Type {
				level := m.evaluateRule(rule, probe.Value)
				if level == LevelCritical {
					probe.Level = LevelCritical
					probe.Message = rule.Message
				} else if level == LevelDegraded && probe.Level == LevelHealthy {
					probe.Level = LevelDegraded
					probe.Message = rule.Message
				}
				break
			}
		}
	}
}

// evaluateRule 评估单条规则
func (m *Manager) evaluateRule(rule *Rule, value float64) HealthLevel {
	var triggered bool
	switch rule.Operator {
	case "gt":
		triggered = value > rule.Threshold
	case "lt":
		triggered = value < rule.Threshold
	case "gte":
		triggered = value >= rule.Threshold
	case "lte":
		triggered = value <= rule.Threshold
	case "eq":
		triggered = math.Abs(value-rule.Threshold) < 0.001
	default:
		triggered = value > rule.Threshold
	}

	if triggered {
		return rule.Level
	}
	return LevelHealthy
}

// calculateScore 计算健康评分 (0-100)
func (m *Manager) calculateScore(status *HealthStatus) float64 {
	if status.Summary.Total == 0 {
		return 100.0
	}

	m.mu.RLock()
	rules := make(map[string]*Rule)
	for k, v := range m.rules {
		rules[k] = v
	}
	m.mu.RUnlock()

	totalWeight := 0.0
	weightedScore := 0.0

	for _, probe := range status.Probes {
		weight := 1.0
		// 查找匹配规则的权重
		for _, rule := range rules {
			if rule.Type == probe.Type {
				weight = rule.Weight
				break
			}
		}

		totalWeight += weight
		switch probe.Level {
		case LevelHealthy:
			weightedScore += 100.0 * weight
		case LevelDegraded:
			weightedScore += 60.0 * weight
		case LevelCritical:
			weightedScore += 0.0
		default:
			weightedScore += 50.0 * weight
		}
	}

	if totalWeight == 0 {
		return 100.0
	}
	return weightedScore / totalWeight
}

// determineLevel 确定总体健康级别
func (m *Manager) determineLevel(status *HealthStatus) HealthLevel {
	if status.Summary.Critical > 0 {
		return LevelCritical
	}
	if status.Summary.Degraded > 0 {
		return LevelDegraded
	}
	if status.Summary.Unknown > 0 && status.Summary.Healthy == 0 {
		return LevelUnknown
	}
	return LevelHealthy
}

// recordHistory 记录历史
func (m *Manager) recordHistory(status *HealthStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record := &HistoryRecord{
		Timestamp: status.Timestamp,
		Level:     status.Level,
		Score:     status.Score,
		Probes:    status.Summary.Total,
	}

	m.history = append(m.history, record)
	if len(m.history) > m.config.HistorySize {
		m.history = m.history[len(m.history)-m.config.HistorySize:]
	}
}

// analyzeTrend 分析趋势
func (m *Manager) analyzeTrend() *TrendAnalysis {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.history) < 2 {
		return &TrendAnalysis{
			Direction: "stable",
			History:   m.history,
		}
	}

	window := m.config.TrendWindow
	if window > len(m.history) {
		window = len(m.history)
	}

	recent := m.history[len(m.history)-window:]
	if len(recent) < 2 {
		return &TrendAnalysis{
			Direction: "stable",
			History:   recent,
		}
	}

	// 计算平均分数变化
	first := recent[0]
	last := recent[len(recent)-1]
	scoreDelta := last.Score - first.Score

	// 计算趋势方向
	direction := "stable"
	if scoreDelta > 5 {
		direction = "improving"
	} else if scoreDelta < -5 {
		direction = "degrading"
	}

	trend := &TrendAnalysis{
		Direction:  direction,
		ScoreDelta: scoreDelta,
		History:    recent,
	}

	// 简单线性预测
	if direction == "degrading" && len(recent) >= 3 {
		avgDelta := scoreDelta / float64(len(recent)-1)
		if avgDelta < 0 {
			currentScore := last.Score
			estimatedMinutes := 0.0
			targetLevel := LevelHealthy

			if currentScore > 40 {
				estimatedMinutes = (currentScore - 40) / math.Abs(avgDelta)
				targetLevel = LevelCritical
			} else if currentScore > 70 {
				estimatedMinutes = (currentScore - 70) / math.Abs(avgDelta)
				targetLevel = LevelDegraded
			}

			if estimatedMinutes > 0 {
				trend.Prediction = &TrendPrediction{
					EstimatedLevel: targetLevel,
					EstimatedTime:  time.Now().Add(time.Duration(estimatedMinutes) * time.Minute),
					Confidence:     math.Min(0.9, 0.5+float64(len(recent))*0.05),
				}
			}
		}
	}

	return trend
}

// checkAlerts 检查并触发告警
func (m *Manager) checkAlerts(ctx context.Context, status *HealthStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, probe := range status.Probes {
		if probe.Level == LevelCritical || probe.Level == LevelDegraded {
			// 检查冷却期
			if m.isInCooldown(name, probe.Level) {
				continue
			}

			severity := SeverityWarning
			if probe.Level == LevelCritical {
				severity = SeverityCritical
			}

			alert := &Alert{
				ID:        fmt.Sprintf("%s-%s-%d", name, probe.Level, time.Now().Unix()),
				Probe:     name,
				Severity:  severity,
				Level:     probe.Level,
				Message:   probe.Message,
				Value:     probe.Value,
				Timestamp: time.Now(),
			}

			m.alerts = append(m.alerts, alert)
			m.logger.Warn("健康告警",
				zap.String("probe", name),
				zap.String("level", string(probe.Level)),
				zap.String("message", probe.Message),
			)

			// 异步发送通知
			go m.sendNotifications(ctx, alert)
		}
	}
}

// isInCooldown 检查是否在冷却期内
func (m *Manager) isInCooldown(probeName string, level HealthLevel) bool {
	cutoff := time.Now().Add(-m.config.AlertCooldown)
	for i := len(m.alerts) - 1; i >= 0; i-- {
		alert := m.alerts[i]
		if alert.Probe == probeName && alert.Level == level && alert.Timestamp.After(cutoff) {
			return true
		}
	}
	return false
}

// sendNotifications 发送告警通知
func (m *Manager) sendNotifications(ctx context.Context, alert *Alert) {
	for _, notifier := range m.notifiers {
		if err := notifier.Notify(ctx, alert); err != nil {
			m.logger.Error("发送告警通知失败",
				zap.String("probe", alert.Probe),
				zap.Error(err),
			)
		}
	}
}

// GetStatus 获取当前健康状态
func (m *Manager) GetStatus() *HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastStatus
}

// GetHistory 获取历史记录
func (m *Manager) GetHistory(limit int) []*HistoryRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}
	start := len(m.history) - limit
	result := make([]*HistoryRecord, limit)
	copy(result, m.history[start:])
	return result
}

// GetAlerts 获取告警列表
func (m *Manager) GetAlerts(limit int, includeResolved bool) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Alert
	for i := len(m.alerts) - 1; i >= 0; i-- {
		if !includeResolved && m.alerts[i].Resolved {
			continue
		}
		result = append(result, m.alerts[i])
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// ResolveAlert 解决告警
func (m *Manager) ResolveAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, alert := range m.alerts {
		if alert.ID == id {
			alert.Resolved = true
			return nil
		}
	}
	return fmt.Errorf("告警 %s 未找到", id)
}

// GetProbes 获取所有已注册探针名称
func (m *Manager) GetProbes() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.probes))
	for name := range m.probes {
		names = append(names, name)
	}
	return names
}

// GetRules 获取所有规则
func (m *Manager) GetRules() []*Rule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*Rule, 0, len(m.rules))
	for _, r := range m.rules {
		rules = append(rules, r)
	}
	return rules
}

// GenerateReport 生成健康报告
func (m *Manager) GenerateReport() *HealthReport {
	status := m.GetStatus()
	if status == nil {
		status = m.Check(context.Background())
	}

	report := &HealthReport{
		GeneratedAt: time.Now(),
		Summary:     status.Summary,
		Score:       status.Score,
		Level:       status.Level,
		Probes:      status.Probes,
		Alerts:      m.GetAlerts(10, false),
		Trend:       status.Trend,
	}

	// 找出问题最严重的探针
	var issues []*ProbeResult
	for _, probe := range status.Probes {
		if probe.Level == LevelCritical || probe.Level == LevelDegraded {
			issues = append(issues, probe)
		}
	}
	report.TopIssues = issues

	// 生成建议
	report.Recommendations = m.generateRecommendations(status)

	return report
}

// generateRecommendations 生成改进建议
func (m *Manager) generateRecommendations(status *HealthStatus) []string {
	var recommendations []string

	for _, probe := range status.Probes {
		switch probe.Type {
		case MetricCPU:
			if probe.Level != LevelHealthy {
				recommendations = append(recommendations, "CPU 使用率过高，建议检查高 CPU 进程或增加计算资源")
			}
		case MetricMemory:
			if probe.Level != LevelHealthy {
				recommendations = append(recommendations, "内存使用率过高，建议检查内存泄漏或增加内存容量")
			}
		case MetricDisk:
			if probe.Level != LevelHealthy {
				recommendations = append(recommendations, "磁盘空间不足，建议清理文件或扩展存储容量")
			}
		case MetricTemp:
			if probe.Level != LevelHealthy {
				recommendations = append(recommendations, "温度过高，建议检查散热系统或降低负载")
			}
		case MetricSMART:
			if probe.Level != LevelHealthy {
				recommendations = append(recommendations, "磁盘 SMART 状态异常，建议备份数据并更换磁盘")
			}
		}
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "系统健康状态良好，无需特别处理")
	}

	return recommendations
}
