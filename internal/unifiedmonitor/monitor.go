package unifiedmonitor

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// HealthScore 系统健康评分
type HealthScore struct {
	Score    int              `json:"score"`    // 0-100分
	Level    string           `json:"level"`    // good/warning/critical
	Details  map[string]int   `json:"details"`  // 各项得分
	LastEval time.Time        `json:"last_eval"`
}

// MetricPoint 指标数据点
type MetricPoint struct {
	Timestamp time.Time              `json:"timestamp"`
	NodeID    string                 `json:"node_id"`
	Name      string                 `json:"name"`
	Value     float64                `json:"value"`
	Labels    map[string]string      `json:"labels,omitempty"`
}

// AlertRule 告警规则
type AlertRule struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Type      RuleType          `json:"type"`      // threshold/trend/anomaly
	Metric    string            `json:"metric"`
	Condition AlertCondition    `json:"condition"`
	Threshold float64           `json:"threshold"`
	Duration  time.Duration     `json:"duration"`
	Severity  AlertSeverity     `json:"severity"`
	Labels    map[string]string `json:"labels,omitempty"`
	Enabled   bool              `json:"enabled"`
	CreatedAt time.Time         `json:"created_at"`
}

// RuleType 规则类型
type RuleType string

const (
	RuleTypeThreshold RuleType = "threshold"
	RuleTypeTrend     RuleType = "trend"
	RuleTypeAnomaly   RuleType = "anomaly"
)

// AlertCondition 告警条件
type AlertCondition string

const (
	ConditionAbove AlertCondition = "above"
	ConditionBelow AlertCondition = "below"
	ConditionEqual AlertCondition = "equal"
	ConditionRateIncrease AlertCondition = "rate_increase"
	ConditionRateDecrease AlertCondition = "rate_decrease"
)

// AlertSeverity 告警严重级别
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// Alert 告警实例
type Alert struct {
	ID         string            `json:"id"`
	RuleID     string            `json:"rule_id"`
	RuleName   string            `json:"rule_name"`
	Severity   AlertSeverity     `json:"severity"`
	Message    string            `json:"message"`
	Value      float64           `json:"value"`
	Threshold  float64           `json:"threshold"`
	NodeID     string            `json:"node_id"`
	Labels     map[string]string `json:"labels,omitempty"`
	Status     AlertStatus       `json:"status"`
	Triggered  time.Time         `json:"triggered"`
	Resolved   *time.Time        `json:"resolved,omitempty"`
}

// AlertStatus 告警状态
type AlertStatus string

const (
	AlertStatusFiring   AlertStatus = "firing"
	AlertStatusResolved AlertStatus = "resolved"
	AlertStatusSilenced AlertStatus = "silenced"
)

// DashboardData 仪表板数据
type DashboardData struct {
	HealthScore    HealthScore              `json:"health_score"`
	ActiveAlerts   []Alert                  `json:"active_alerts"`
	RecentMetrics  map[string][]MetricPoint `json:"recent_metrics"`
	NodeStatus     map[string]NodeStatus    `json:"node_status"`
	TopIssues      []string                 `json:"top_issues"`
	Timestamp      time.Time                `json:"timestamp"`
}

// NodeStatus 节点状态
type NodeStatus struct {
	NodeID    string    `json:"node_id"`
	Online    bool      `json:"online"`
	LastSeen  time.Time `json:"last_seen"`
	CPUPercent float64  `json:"cpu_percent"`
	MemPercent float64  `json:"mem_percent"`
	DiskPercent float64 `json:"disk_percent"`
}

// MetricStore 指标存储接口
type MetricStore interface {
	Store(ctx context.Context, point MetricPoint) error
	Query(ctx context.Context, name string, nodeID string, start, end time.Time) ([]MetricPoint, error)
	Aggregate(ctx context.Context, name string, nodeID string, start, end time.Time, interval time.Duration) ([]MetricPoint, error)
}

// AlertStore 告警存储接口
type AlertStore interface {
	Store(ctx context.Context, alert Alert) error
	Query(ctx context.Context, status AlertStatus, limit int) ([]Alert, error)
	UpdateStatus(ctx context.Context, alertID string, status AlertStatus) error
}

// Monitor 统一监控服务
type Monitor struct {
	mu           sync.RWMutex
	metricStore  MetricStore
	alertStore   AlertStore
	rules        map[string]*AlertRule
	alerts       map[string]*Alert
	nodeStatus   map[string]*NodeStatus
	config       MonitorConfig
	stopCh       chan struct{}
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	MetricRetention     time.Duration `json:"metric_retention"`
	AlertDedupWindow    time.Duration `json:"alert_dedup_window"`
	MaxAlerts           int           `json:"max_alerts"`
}

// DefaultConfig 默认配置
func DefaultConfig() MonitorConfig {
	return MonitorConfig{
		HealthCheckInterval: 30 * time.Second,
		MetricRetention:     7 * 24 * time.Hour,
		AlertDedupWindow:    5 * time.Minute,
		MaxAlerts:           1000,
	}
}

// NewMonitor 创建监控服务
func NewMonitor(metricStore MetricStore, alertStore AlertStore, config MonitorConfig) *Monitor {
	return &Monitor{
		metricStore: metricStore,
		alertStore:  alertStore,
		rules:       make(map[string]*AlertRule),
		alerts:      make(map[string]*Alert),
		nodeStatus:  make(map[string]*NodeStatus),
		config:      config,
		stopCh:      make(chan struct{}),
	}
}

// Start 启动监控服务
func (m *Monitor) Start(ctx context.Context) error {
	go m.healthCheckLoop(ctx)
	go m.evaluateRulesLoop(ctx)
	return nil
}

// Stop 停止监控服务
func (m *Monitor) Stop() {
	close(m.stopCh)
}

// RecordMetric 记录指标
func (m *Monitor) RecordMetric(ctx context.Context, point MetricPoint) error {
	if err := m.metricStore.Store(ctx, point); err != nil {
		return fmt.Errorf("store metric: %w", err)
	}
	
	m.updateNodeStatus(point)
	return nil
}

// GetHealthScore 获取系统健康评分
func (m *Monitor) GetHealthScore(ctx context.Context) (*HealthScore, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	score := &HealthScore{
		Details:  make(map[string]int),
		LastEval: time.Now(),
	}
	
	// CPU评分 (权重30%)
	cpuScore := m.evaluateCPUHealth()
	score.Details["cpu"] = cpuScore
	
	// 内存评分 (权重25%)
	memScore := m.evaluateMemoryHealth()
	score.Details["memory"] = memScore
	
	// 磁盘评分 (权重25%)
	diskScore := m.evaluateDiskHealth()
	score.Details["disk"] = diskScore
	
	// 网络评分 (权重20%)
	networkScore := m.evaluateNetworkHealth()
	score.Details["network"] = networkScore
	
	// 计算总分
	totalScore := float64(cpuScore)*0.3 + float64(memScore)*0.25 + 
		float64(diskScore)*0.25 + float64(networkScore)*0.2
	score.Score = int(math.Round(totalScore))
	
	// 确定等级
	switch {
	case score.Score >= 80:
		score.Level = "good"
	case score.Score >= 60:
		score.Level = "warning"
	default:
		score.Level = "critical"
	}
	
	return score, nil
}

// QueryMetrics 查询指标
func (m *Monitor) QueryMetrics(ctx context.Context, name, nodeID string, start, end time.Time) ([]MetricPoint, error) {
	return m.metricStore.Query(ctx, name, nodeID, start, end)
}

// AddRule 添加告警规则
func (m *Monitor) AddRule(rule *AlertRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if rule.ID == "" {
		return fmt.Errorf("rule ID is required")
	}
	
	rule.CreatedAt = time.Now()
	m.rules[rule.ID] = rule
	return nil
}

// RemoveRule 删除告警规则
func (m *Monitor) RemoveRule(ruleID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rules, ruleID)
}

// GetAlerts 获取告警列表
func (m *Monitor) GetAlerts(ctx context.Context, status AlertStatus, limit int) ([]Alert, error) {
	return m.alertStore.Query(ctx, status, limit)
}

// AcknowledgeAlert 确认告警
func (m *Monitor) AcknowledgeAlert(ctx context.Context, alertID string) error {
	return m.alertStore.UpdateStatus(ctx, alertID, AlertStatusSilenced)
}

// ResolveAlert 解决告警
func (m *Monitor) ResolveAlert(ctx context.Context, alertID string) error {
	return m.alertStore.UpdateStatus(ctx, alertID, AlertStatusResolved)
}

// GetDashboard 获取仪表板数据
func (m *Monitor) GetDashboard(ctx context.Context) (*DashboardData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	health, err := m.GetHealthScore(ctx)
	if err != nil {
		return nil, fmt.Errorf("get health score: %w", err)
	}
	
	alerts, err := m.alertStore.Query(ctx, AlertStatusFiring, 20)
	if err != nil {
		return nil, fmt.Errorf("get alerts: %w", err)
	}
	
	recentMetrics := make(map[string][]MetricPoint)
	metrics := []string{"cpu_usage", "memory_usage", "disk_usage", "network_throughput"}
	end := time.Now()
	start := end.Add(-1 * time.Hour)
	
	for _, metric := range metrics {
		points, err := m.metricStore.Query(ctx, metric, "", start, end)
		if err != nil {
			continue
		}
		recentMetrics[metric] = points
	}
	
	topIssues := m.identifyTopIssues()
	
	return &DashboardData{
		HealthScore:   *health,
		ActiveAlerts:  alerts,
		RecentMetrics: recentMetrics,
		NodeStatus:    m.getNodeStatusMap(),
		TopIssues:     topIssues,
		Timestamp:     time.Now(),
	}, nil
}

// evaluateCPUHealth 评估CPU健康度
func (m *Monitor) evaluateCPUHealth() int {
	totalScore := 0
	count := 0
	
	for _, node := range m.nodeStatus {
		count++
		switch {
		case node.CPUPercent < 70:
			totalScore += 100
		case node.CPUPercent < 85:
			totalScore += 70
		case node.CPUPercent < 95:
			totalScore += 40
		default:
			totalScore += 10
		}
	}
	
	if count == 0 {
		return 100
	}
	return totalScore / count
}

// evaluateMemoryHealth 评估内存健康度
func (m *Monitor) evaluateMemoryHealth() int {
	totalScore := 0
	count := 0
	
	for _, node := range m.nodeStatus {
		count++
		switch {
		case node.MemPercent < 75:
			totalScore += 100
		case node.MemPercent < 85:
			totalScore += 70
		case node.MemPercent < 95:
			totalScore += 40
		default:
			totalScore += 10
		}
	}
	
	if count == 0 {
		return 100
	}
	return totalScore / count
}

// evaluateDiskHealth 评估磁盘健康度
func (m *Monitor) evaluateDiskHealth() int {
	totalScore := 0
	count := 0
	
	for _, node := range m.nodeStatus {
		count++
		switch {
		case node.DiskPercent < 80:
			totalScore += 100
		case node.DiskPercent < 90:
			totalScore += 70
		case node.DiskPercent < 95:
			totalScore += 40
		default:
			totalScore += 10
		}
	}
	
	if count == 0 {
		return 100
	}
	return totalScore / count
}

// evaluateNetworkHealth 评估网络健康度
func (m *Monitor) evaluateNetworkHealth() int {
	// 简化实现：基于节点在线状态
	onlineCount := 0
	totalCount := 0
	
	for _, node := range m.nodeStatus {
		totalCount++
		if node.Online {
			onlineCount++
		}
	}
	
	if totalCount == 0 {
		return 100
	}
	
	return (onlineCount * 100) / totalCount
}

// updateNodeStatus 更新节点状态
func (m *Monitor) updateNodeStatus(point MetricPoint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	node, exists := m.nodeStatus[point.NodeID]
	if !exists {
		node = &NodeStatus{
			NodeID: point.NodeID,
			Online: true,
		}
		m.nodeStatus[point.NodeID] = node
	}
	
	node.LastSeen = time.Now()
	node.Online = true
	
	switch point.Name {
	case "cpu_usage":
		node.CPUPercent = point.Value
	case "memory_usage":
		node.MemPercent = point.Value
	case "disk_usage":
		node.DiskPercent = point.Value
	}
}

// evaluateRulesLoop 定期评估告警规则
func (m *Monitor) evaluateRulesLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.evaluateAllRules(ctx)
		}
	}
}

// evaluateAllRules 评估所有规则
func (m *Monitor) evaluateAllRules(ctx context.Context) {
	m.mu.RLock()
	rules := make([]*AlertRule, 0, len(m.rules))
	for _, rule := range m.rules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}
	m.mu.RUnlock()
	
	for _, rule := range rules {
		m.evaluateRule(ctx, rule)
	}
}

// evaluateRule 评估单个规则
func (m *Monitor) evaluateRule(ctx context.Context, rule *AlertRule) {
	end := time.Now()
	start := end.Add(-rule.Duration)
	
	points, err := m.metricStore.Query(ctx, rule.Metric, "", start, end)
	if err != nil || len(points) == 0 {
		return
	}
	
	var shouldAlert bool
	var alertValue float64
	
	switch rule.Type {
	case RuleTypeThreshold:
		alertValue = points[len(points)-1].Value
		shouldAlert = m.evaluateThreshold(rule.Condition, alertValue, rule.Threshold)
		
	case RuleTypeTrend:
		alertValue = m.calculateTrend(points)
		shouldAlert = m.evaluateTrend(rule.Condition, alertValue, rule.Threshold)
		
	case RuleTypeAnomaly:
		alertValue = points[len(points)-1].Value
		mean, stddev := m.calculateStats(points)
		shouldAlert = m.evaluateAnomaly(alertValue, mean, stddev, rule.Threshold)
	}
	
	if shouldAlert {
		m.createAlert(rule, alertValue)
	}
}

// evaluateThreshold 评估阈值条件
func (m *Monitor) evaluateThreshold(cond AlertCondition, value, threshold float64) bool {
	switch cond {
	case ConditionAbove:
		return value > threshold
	case ConditionBelow:
		return value < threshold
	case ConditionEqual:
		return math.Abs(value-threshold) < 0.01
	}
	return false
}

// evaluateTrend 评估趋势条件
func (m *Monitor) evaluateTrend(cond AlertCondition, trend, threshold float64) bool {
	switch cond {
	case ConditionRateIncrease:
		return trend > threshold
	case ConditionRateDecrease:
		return trend < -threshold
	}
	return false
}

// evaluateAnomaly 评估异常条件
func (m *Monitor) evaluateAnomaly(value, mean, stddev, sensitivity float64) bool {
	if stddev == 0 {
		return false
	}
	zScore := math.Abs(value-mean) / stddev
	return zScore > sensitivity
}

// calculateTrend 计算趋势（线性回归斜率）
func (m *Monitor) calculateTrend(points []MetricPoint) float64 {
	if len(points) < 2 {
		return 0
	}
	
	n := float64(len(points))
	var sumX, sumY, sumXY, sumX2 float64
	
	for i, p := range points {
		x := float64(i)
		y := p.Value
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	
	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0
	}
	
	slope := (n*sumXY - sumX*sumY) / denominator
	return slope
}

// calculateStats 计算均值和标准差
func (m *Monitor) calculateStats(points []MetricPoint) (mean, stddev float64) {
	if len(points) == 0 {
		return 0, 0
	}
	
	var sum float64
	for _, p := range points {
		sum += p.Value
	}
	mean = sum / float64(len(points))
	
	var variance float64
	for _, p := range points {
		diff := p.Value - mean
		variance += diff * diff
	}
	variance /= float64(len(points))
	stddev = math.Sqrt(variance)
	
	return mean, stddev
}

// createAlert 创建告警
func (m *Monitor) createAlert(rule *AlertRule, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 告警去重
	for _, existing := range m.alerts {
		if existing.RuleID == rule.ID && 
			existing.Status == AlertStatusFiring &&
			time.Since(existing.Triggered) < m.config.AlertDedupWindow {
			return
		}
	}
	
	alert := &Alert{
		ID:        fmt.Sprintf("%s-%d", rule.ID, time.Now().UnixNano()),
		RuleID:    rule.ID,
		RuleName:  rule.Name,
		Severity:  rule.Severity,
		Message:   fmt.Sprintf("%s: %s %.2f (threshold: %.2f)", rule.Name, rule.Condition, value, rule.Threshold),
		Value:     value,
		Threshold: rule.Threshold,
		Labels:    rule.Labels,
		Status:    AlertStatusFiring,
		Triggered: time.Now(),
	}
	
	m.alerts[alert.ID] = alert
	
	// 异步存储
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = m.alertStore.Store(ctx, *alert)
	}()
}

// healthCheckLoop 健康检查循环
func (m *Monitor) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkNodeHealth()
		}
	}
}

// checkNodeHealth 检查节点健康状态
func (m *Monitor) checkNodeHealth() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	threshold := m.config.HealthCheckInterval * 3
	for _, node := range m.nodeStatus {
		if time.Since(node.LastSeen) > threshold {
			node.Online = false
		}
	}
}

// identifyTopIssues 识别主要问题
func (m *Monitor) identifyTopIssues() []string {
	issues := make([]string, 0)
	
	for _, node := range m.nodeStatus {
		if !node.Online {
			issues = append(issues, fmt.Sprintf("节点 %s 离线", node.NodeID))
			continue
		}
		if node.CPUPercent > 90 {
			issues = append(issues, fmt.Sprintf("节点 %s CPU 使用率过高 (%.1f%%)", node.NodeID, node.CPUPercent))
		}
		if node.MemPercent > 90 {
			issues = append(issues, fmt.Sprintf("节点 %s 内存使用率过高 (%.1f%%)", node.NodeID, node.MemPercent))
		}
		if node.DiskPercent > 90 {
			issues = append(issues, fmt.Sprintf("节点 %s 磁盘使用率过高 (%.1f%%)", node.NodeID, node.DiskPercent))
		}
	}
	
	// 按严重程度排序
	sort.Slice(issues, func(i, j int) bool {
		return len(issues[i]) > len(issues[j])
	})
	
	if len(issues) > 5 {
		return issues[:5]
	}
	return issues
}

// getNodeStatusMap 获取节点状态映射
func (m *Monitor) getNodeStatusMap() map[string]NodeStatus {
	result := make(map[string]NodeStatus)
	for id, node := range m.nodeStatus {
		result[id] = *node
	}
	return result
}
