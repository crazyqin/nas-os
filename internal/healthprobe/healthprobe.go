// Package healthprobe 提供系统健康探针功能，聚合多维度健康状态
// Version: v1.0.0 - 健康探针模块
package healthprobe

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"go.uber.org/zap"
)

// HealthLevel 健康级别
type HealthLevel string

const (
	LevelHealthy  HealthLevel = "healthy"
	LevelDegraded HealthLevel = "degraded"
	LevelCritical HealthLevel = "critical"
	LevelUnknown  HealthLevel = "unknown"
)

// Severity 告警严重程度
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// MetricType 指标类型
type MetricType string

const (
	MetricCPU     MetricType = "cpu"
	MetricMemory  MetricType = "memory"
	MetricDisk    MetricType = "disk"
	MetricNetwork MetricType = "network"
	MetricTemp    MetricType = "temperature"
	MetricCustom  MetricType = "custom"
)

// ProbeResult 单项探针结果
type ProbeResult struct {
	Name      string                 `json:"name"`
	Type      MetricType             `json:"type"`
	Level     HealthLevel            `json:"level"`
	Value     float64                `json:"value"`
	Unit      string                 `json:"unit"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Duration  time.Duration          `json:"duration"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// HealthStatus 聚合健康状态
type HealthStatus struct {
	Level      HealthLevel              `json:"level"`
	Score      float64                  `json:"score"` // 0-100 健康评分
	Timestamp  time.Time                `json:"timestamp"`
	Uptime     time.Duration            `json:"uptime"`
	Probes     map[string]*ProbeResult  `json:"probes"`
	Summary    *StatusSummary           `json:"summary"`
	Trend      *TrendAnalysis           `json:"trend,omitempty"`
	Alerts     []*Alert                 `json:"alerts,omitempty"`
	Metadata   map[string]interface{}   `json:"metadata,omitempty"`
}

// StatusSummary 状态摘要
type StatusSummary struct {
	Total    int `json:"total"`
	Healthy  int `json:"healthy"`
	Degraded int `json:"degraded"`
	Critical int `json:"critical"`
	Unknown  int `json:"unknown"`
}

// TrendAnalysis 趋势分析
type TrendAnalysis struct {
	Direction   string             `json:"direction"` // improving, stable, degrading
	ScoreDelta  float64            `json:"scoreDelta"`
	History     []*HistoryRecord   `json:"history"`
	Prediction  *TrendPrediction   `json:"prediction,omitempty"`
}

// TrendPrediction 趋势预测
type TrendPrediction struct {
	EstimatedLevel    HealthLevel `json:"estimatedLevel"`
	EstimatedTime     time.Time   `json:"estimatedTime"`
	Confidence        float64     `json:"confidence"`
}

// HistoryRecord 历史记录
type HistoryRecord struct {
	Timestamp time.Time   `json:"timestamp"`
	Level     HealthLevel `json:"level"`
	Score     float64     `json:"score"`
	Probes    int         `json:"probes"`
}

// Alert 健康告警
type Alert struct {
	ID        string     `json:"id"`
	Probe     string     `json:"probe"`
	Severity  Severity   `json:"severity"`
	Level     HealthLevel `json:"level"`
	Message   string     `json:"message"`
	Value     float64    `json:"value"`
	Threshold float64    `json:"threshold"`
	Timestamp time.Time  `json:"timestamp"`
	Resolved  bool       `json:"resolved"`
}

// Rule 健康检查规则
type Rule struct {
	Name      string      `json:"name"`
	Type      MetricType  `json:"type"`
	Threshold float64     `json:"threshold"`
	Level     HealthLevel `json:"level"`
	Operator  string      `json:"operator"` // gt, lt, gte, lte, eq
	Weight    float64     `json:"weight"`   // 权重 0-1
	Message   string      `json:"message"`
	Enabled   bool        `json:"enabled"`
}

// Probe 探针接口
type Probe interface {
	Name() string
	Type() MetricType
	Collect(ctx context.Context) (*ProbeResult, error)
}

// ProbeFunc 函数式探针
type ProbeFunc struct {
	name    string
	mtype   MetricType
	collect func(ctx context.Context) (*ProbeResult, error)
}

// NewProbeFunc 创建函数式探针
func NewProbeFunc(name string, mtype MetricType, fn func(ctx context.Context) (*ProbeResult, error)) *ProbeFunc {
	return &ProbeFunc{
		name:    name,
		mtype:   mtype,
		collect: fn,
	}
}

func (p *ProbeFunc) Name() string                                    { return p.name }
func (p *ProbeFunc) Type() MetricType                                { return p.mtype }
func (p *ProbeFunc) Collect(ctx context.Context) (*ProbeResult, error) { return p.collect(ctx) }

// Notifier 告警通知接口
type Notifier interface {
	Notify(ctx context.Context, alert *Alert) error
}

// NotifierFunc 函数式通知器
type NotifierFunc func(ctx context.Context, alert *Alert) error

func (f NotifierFunc) Notify(ctx context.Context, alert *Alert) error { return f(ctx, alert) }

// Config 健康探针配置
type Config struct {
	Interval       time.Duration `json:"interval"`       // 检测间隔
	Timeout        time.Duration `json:"timeout"`        // 单项检测超时
	HistorySize    int           `json:"historySize"`    // 历史记录大小
	AlertCooldown  time.Duration `json:"alertCooldown"`  // 告警冷却时间
	EnableTrend    bool          `json:"enableTrend"`    // 启用趋势分析
	TrendWindow    int           `json:"trendWindow"`    // 趋势分析窗口大小
	AutoStart      bool          `json:"autoStart"`      // 自动启动
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Interval:      30 * time.Second,
		Timeout:       10 * time.Second,
		HistorySize:   1440, // 24h @ 1min intervals
		AlertCooldown: 5 * time.Minute,
		EnableTrend:   true,
		TrendWindow:   10,
		AutoStart:     false,
	}
}

// ProbeManager 健康探针管理器
type ProbeManager struct {
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

// NewProbeManager 创建健康探针管理器
func NewProbeManager(logger *zap.Logger, config *Config) *ProbeManager {
	if config == nil {
		config = DefaultConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ProbeManager{
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
func (pm *ProbeManager) RegisterProbe(probe Probe) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.probes[probe.Name()] = probe
	pm.logger.Info("注册健康探针", zap.String("name", probe.Name()), zap.String("type", string(probe.Type())))
}

// UnregisterProbe 注销探针
func (pm *ProbeManager) UnregisterProbe(name string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.probes, name)
	pm.logger.Info("注销健康探针", zap.String("name", name))
}

// AddRule 添加检查规则
func (pm *ProbeManager) AddRule(rule *Rule) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if rule.Weight <= 0 {
		rule.Weight = 1.0
	}
	pm.rules[rule.Name] = rule
	pm.logger.Info("添加健康规则", zap.String("name", rule.Name), zap.String("type", string(rule.Type)))
}

// RemoveRule 移除检查规则
func (pm *ProbeManager) RemoveRule(name string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.rules, name)
}

// AddNotifier 添加告警通知器
func (pm *ProbeManager) AddNotifier(notifier Notifier) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.notifiers = append(pm.notifiers, notifier)
}

// Start 启动定期检测
func (pm *ProbeManager) Start(ctx context.Context) {
	pm.mu.Lock()
	if pm.running {
		pm.mu.Unlock()
		return
	}
	pm.running = true
	pm.stopCh = make(chan struct{})
	pm.mu.Unlock()

	pm.logger.Info("启动健康探针管理器", zap.Duration("interval", pm.config.Interval))

	go pm.run(ctx)
}

// Stop 停止定期检测
func (pm *ProbeManager) Stop() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if !pm.running {
		return
	}
	pm.running = false
	close(pm.stopCh)
	pm.logger.Info("停止健康探针管理器")
}

// IsRunning 是否运行中
func (pm *ProbeManager) IsRunning() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.running
}

// run 定期检测循环
func (pm *ProbeManager) run(ctx context.Context) {
	ticker := time.NewTicker(pm.config.Interval)
	defer ticker.Stop()

	// 立即执行一次
	pm.Check(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-pm.stopCh:
			return
		case <-ticker.C:
			pm.Check(ctx)
		}
	}
}

// Check 执行所有探针检测并聚合状态
func (pm *ProbeManager) Check(ctx context.Context) *HealthStatus {
	pm.mu.RLock()
	probeList := make([]Probe, 0, len(pm.probes))
	for _, p := range pm.probes {
		probeList = append(probeList, p)
	}
	pm.mu.RUnlock()

	status := &HealthStatus{
		Timestamp: time.Now(),
		Uptime:    time.Since(pm.startTime),
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
			result := pm.runProbe(ctx, p)

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
	pm.evaluateRules(status)

	// 计算健康评分
	status.Score = pm.calculateScore(status)

	// 确定总体健康级别
	status.Level = pm.determineLevel(status)

	// 记录历史
	pm.recordHistory(status)

	// 趋势分析
	if pm.config.EnableTrend {
		status.Trend = pm.analyzeTrend()
	}

	// 检查告警
	pm.checkAlerts(ctx, status)

	// 更新最后状态
	pm.mu.Lock()
	pm.lastStatus = status
	pm.mu.Unlock()

	pm.logger.Debug("健康检查完成",
		zap.String("level", string(status.Level)),
		zap.Float64("score", status.Score),
		zap.Int("probes", status.Summary.Total))

	return status
}

// runProbe 执行单个探针
func (pm *ProbeManager) runProbe(ctx context.Context, probe Probe) *ProbeResult {
	probeCtx, cancel := context.WithTimeout(ctx, pm.config.Timeout)
	defer cancel()

	start := time.Now()
	result, err := probe.Collect(probeCtx)
	duration := time.Since(start)

	if result == nil {
		result = &ProbeResult{
			Name:      probe.Name(),
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
func (pm *ProbeManager) evaluateRules(status *HealthStatus) {
	pm.mu.RLock()
	rules := make([]*Rule, 0, len(pm.rules))
	for _, r := range pm.rules {
		if r.Enabled {
			rules = append(rules, r)
		}
	}
	pm.mu.RUnlock()

	for _, rule := range rules {
		probe, exists := status.Probes[rule.Type.String()]
		if !exists {
			// 检查是否有匹配类型的探针
			for name, p := range status.Probes {
				if p.Type == rule.Type {
					probe = p
					_ = name
					break
				}
			}
		}
		if probe == nil {
			continue
		}

		level := pm.evaluateRule(rule, probe.Value)
		if level == LevelCritical {
			probe.Level = LevelCritical
			probe.Message = rule.Message
		} else if level == LevelDegraded && probe.Level == LevelHealthy {
			probe.Level = LevelDegraded
			probe.Message = rule.Message
		}
	}
}

// evaluateRule 评估单条规则
func (pm *ProbeManager) evaluateRule(rule *Rule, value float64) HealthLevel {
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
func (pm *ProbeManager) calculateScore(status *HealthStatus) float64 {
	if status.Summary.Total == 0 {
		return 100.0
	}

	pm.mu.RLock()
	rules := make(map[string]*Rule)
	for k, v := range pm.rules {
		rules[k] = v
	}
	pm.mu.RUnlock()

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
func (pm *ProbeManager) determineLevel(status *HealthStatus) HealthLevel {
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
func (pm *ProbeManager) recordHistory(status *HealthStatus) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	record := &HistoryRecord{
		Timestamp: status.Timestamp,
		Level:     status.Level,
		Score:     status.Score,
		Probes:    status.Summary.Total,
	}

	pm.history = append(pm.history, record)
	if len(pm.history) > pm.config.HistorySize {
		pm.history = pm.history[len(pm.history)-pm.config.HistorySize:]
	}
}

// analyzeTrend 分析趋势
func (pm *ProbeManager) analyzeTrend() *TrendAnalysis {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if len(pm.history) < 2 {
		return &TrendAnalysis{
			Direction: "stable",
			History:   pm.history,
		}
	}

	window := pm.config.TrendWindow
	if window > len(pm.history) {
		window = len(pm.history)
	}

	recent := pm.history[len(pm.history)-window:]
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
		// 计算平均下降速率
		avgDelta := scoreDelta / float64(len(recent)-1)
		if avgDelta < 0 {
			// 预测何时会降到 critical (< 40) 或 degraded (< 70)
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
func (pm *ProbeManager) checkAlerts(ctx context.Context, status *HealthStatus) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for name, probe := range status.Probes {
		if probe.Level == LevelCritical || probe.Level == LevelDegraded {
			// 检查冷却期
			if pm.isInCooldown(name, probe.Level) {
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

			pm.alerts = append(pm.alerts, alert)
			pm.logger.Warn("健康告警",
				zap.String("probe", name),
				zap.String("level", string(probe.Level)),
				zap.String("message", probe.Message))

			// 异步发送通知
			go pm.sendNotifications(ctx, alert)
		}
	}
}

// isInCooldown 检查是否在冷却期内
func (pm *ProbeManager) isInCooldown(probeName string, level HealthLevel) bool {
	cutoff := time.Now().Add(-pm.config.AlertCooldown)
	for i := len(pm.alerts) - 1; i >= 0; i-- {
		alert := pm.alerts[i]
		if alert.Probe == probeName && alert.Level == level && alert.Timestamp.After(cutoff) {
			return true
		}
	}
	return false
}

// sendNotifications 发送告警通知
func (pm *ProbeManager) sendNotifications(ctx context.Context, alert *Alert) {
	for _, notifier := range pm.notifiers {
		if err := notifier.Notify(ctx, alert); err != nil {
			pm.logger.Error("发送告警通知失败",
				zap.String("probe", alert.Probe),
				zap.Error(err))
		}
	}
}

// GetStatus 获取当前健康状态
func (pm *ProbeManager) GetStatus() *HealthStatus {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.lastStatus
}

// GetHistory 获取历史记录
func (pm *ProbeManager) GetHistory(limit int) []*HistoryRecord {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if limit <= 0 || limit > len(pm.history) {
		limit = len(pm.history)
	}
	start := len(pm.history) - limit
	result := make([]*HistoryRecord, limit)
	copy(result, pm.history[start:])
	return result
}

// GetAlerts 获取告警列表
func (pm *ProbeManager) GetAlerts(limit int, includeResolved bool) []*Alert {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var result []*Alert
	for i := len(pm.alerts) - 1; i >= 0; i-- {
		if !includeResolved && pm.alerts[i].Resolved {
			continue
		}
		result = append(result, pm.alerts[i])
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// ResolveAlert 解决告警
func (pm *ProbeManager) ResolveAlert(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, alert := range pm.alerts {
		if alert.ID == id {
			alert.Resolved = true
			return nil
		}
	}
	return fmt.Errorf("告警 %s 未找到", id)
}

// GetProbes 获取所有已注册探针名称
func (pm *ProbeManager) GetProbes() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	names := make([]string, 0, len(pm.probes))
	for name := range pm.probes {
		names = append(names, name)
	}
	return names
}

// GetRules 获取所有规则
func (pm *ProbeManager) GetRules() []*Rule {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	rules := make([]*Rule, 0, len(pm.rules))
	for _, r := range pm.rules {
		rules = append(rules, r)
	}
	return rules
}

// String 返回 MetricType 的字符串表示
func (t MetricType) String() string {
	return string(t)
}
