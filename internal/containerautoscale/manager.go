// Package containerautoscale 提供容器自动扩缩容核心逻辑
package containerautoscale

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 容器自动扩缩容管理器
type Manager struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	config      *AutoScaleConfig
	containers  map[string]*Container
	policies    map[string]*ScalePolicy
	quotas      map[string]*ResourceQuota
	metrics     []MetricPoint
	events      []*ScaleEvent
	alerts      []*Alert
	suggestions []*CostSuggestion
	predictors  map[string]*predictor
	stopChan    chan struct{}
	running     bool
}

// predictor 预测器
type predictor struct {
	values     []float64
	timestamps []time.Time
	window     int
}

// NewManager 创建自动扩缩容管理器
func NewManager(logger *zap.Logger, config *AutoScaleConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultAutoScaleConfig()
	}
	return &Manager{
		logger:      logger,
		config:      config,
		containers:  make(map[string]*Container),
		policies:    make(map[string]*ScalePolicy),
		quotas:      make(map[string]*ResourceQuota),
		metrics:     make([]MetricPoint, 0),
		events:      make([]*ScaleEvent, 0),
		alerts:      make([]*Alert, 0),
		suggestions: make([]*CostSuggestion, 0),
		predictors:  make(map[string]*predictor),
		stopChan:    make(chan struct{}),
	}
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Start 启动管理器
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("manager already running")
	}
	m.running = true
	m.mu.Unlock()

	go m.metricsCollector(ctx)
	go m.scalingLoop(ctx)
	go m.cleanupLoop(ctx)

	m.logger.Info("container autoscale manager started")
	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}
	m.running = false
	close(m.stopChan)
	m.logger.Info("container autoscale manager stopped")
}

// metricsCollector 指标采集循环
func (m *Manager) metricsCollector(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(m.config.MetricsIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.collectMetrics()
		}
	}
}

// collectMetrics 采集所有容器的指标
func (m *Manager) collectMetrics() {
	m.mu.RLock()
	containers := make([]*Container, 0, len(m.containers))
	for _, c := range m.containers {
		containers = append(containers, c)
	}
	m.mu.RUnlock()

	for _, c := range containers {
		now := time.Now()
		metrics := []MetricPoint{
			{Timestamp: now, Type: MetricCPU, Value: simulateMetric(MetricCPU, c.ServiceName), ServiceName: c.ServiceName, ContainerID: c.ID},
			{Timestamp: now, Type: MetricMemory, Value: simulateMetric(MetricMemory, c.ServiceName), ServiceName: c.ServiceName, ContainerID: c.ID},
			{Timestamp: now, Type: MetricRequests, Value: simulateMetric(MetricRequests, c.ServiceName), ServiceName: c.ServiceName, ContainerID: c.ID},
		}

		m.mu.Lock()
		m.metrics = append(m.metrics, metrics...)
		// 限制内存中指标数量
		if len(m.metrics) > 100000 {
			m.metrics = m.metrics[len(m.metrics)-50000:]
		}
		m.mu.Unlock()

		// 更新预测器
		for _, mp := range metrics {
			m.updatePredictor(mp)
		}
	}
}

// simulateMetric 模拟指标采集（实际应从监控系统获取）
func simulateMetric(metricType MetricType, serviceName string) float64 {
	// 基于时间的模拟值
	hour := time.Now().Hour()
	base := 0.0
	switch metricType {
	case MetricCPU:
		base = 30.0 + float64(hour)*1.5
	case MetricMemory:
		base = 40.0 + float64(hour)*0.8
	case MetricRequests:
		base = 100.0 + float64(hour)*10.0
	}
	// 添加一些随机波动
	b := make([]byte, 1)
	rand.Read(b)
	fluctuation := float64(b[0]%20-10) / 10.0
	return math.Max(0, math.Min(100, base+fluctuation*base*0.1))
}

// scalingLoop 扩缩容循环
func (m *Manager) scalingLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.evaluatePolicies()
		}
	}
}

// evaluatePolicies 评估所有策略
func (m *Manager) evaluatePolicies() {
	m.mu.RLock()
	policies := make([]*ScalePolicy, 0, len(m.policies))
	for _, p := range m.policies {
		if p.Enabled {
			policies = append(policies, p)
		}
	}
	m.mu.RUnlock()

	for _, policy := range policies {
		m.evaluatePolicy(policy)
	}
}

// evaluatePolicy 评估单个策略
func (m *Manager) evaluatePolicy(policy *ScalePolicy) {
	switch policy.Strategy {
	case StrategyThreshold:
		m.evaluateThreshold(policy)
	case StrategyPredict:
		m.evaluatePredictive(policy)
	case StrategySchedule:
		m.evaluateSchedule(policy)
	case StrategyManual:
		// 手动策略不自动执行
	}
}

// evaluateThreshold 阈值策略评估
func (m *Manager) evaluateThreshold(policy *ScalePolicy) {
	if policy.Threshold == nil {
		return
	}

	avgValue := m.getAverageMetric(policy.ServiceName, policy.MetricType, policy.Threshold.EvaluationPeriods)
	if avgValue < 0 {
		return
	}

	// 检查冷却期
	if !m.checkCooldown(policy) {
		return
	}

	m.mu.RLock()
	container, exists := m.containers[policy.ServiceName]
	if !exists {
		m.mu.RUnlock()
		return
	}
	currentReplicas := container.Replicas
	m.mu.RUnlock()

	var direction ScaleDirection
	var targetReplicas int
	var reason string

	if avgValue >= policy.Threshold.ScaleUpThreshold {
		direction = ScaleUp
		targetReplicas = currentReplicas + policy.Threshold.ScaleUpStep
		reason = fmt.Sprintf("metric %s value %.2f >= threshold %.2f", policy.MetricType, avgValue, policy.Threshold.ScaleUpThreshold)
	} else if avgValue <= policy.Threshold.ScaleDownThreshold {
		direction = ScaleDown
		targetReplicas = currentReplicas - policy.Threshold.ScaleDownStep
		reason = fmt.Sprintf("metric %s value %.2f <= threshold %.2f", policy.MetricType, avgValue, policy.Threshold.ScaleDownThreshold)
	} else {
		return
	}

	// 检查配额
	if !m.checkQuota(policy.ServiceName, targetReplicas) {
		m.createAlert(policy.ServiceName, AlertWarning, "quota_exceeded", "扩缩操作被配额限制阻止")
		return
	}

	// 执行扩缩
	event := m.executeScale(policy.ServiceName, direction, currentReplicas, targetReplicas, policy.Strategy, reason, string(policy.MetricType), avgValue)
	if event.Success {
		policy.LastScaleTime = time.Now()
	}
}

// evaluatePredictive 预测策略评估
func (m *Manager) evaluatePredictive(policy *ScalePolicy) {
	result := m.Predict(context.Background(), policy.ServiceName, policy.MetricType, "15m")
	if result == nil || result.Confidence < 0.7 {
		return
	}

	if !m.checkCooldown(policy) {
		return
	}

	m.mu.RLock()
	container, exists := m.containers[policy.ServiceName]
	if !exists {
		m.mu.RUnlock()
		return
	}
	currentReplicas := container.Replicas
	m.mu.RUnlock()

	if result.RecommendedReps != currentReplicas {
		var direction ScaleDirection
		if result.RecommendedReps > currentReplicas {
			direction = ScaleUp
		} else {
			direction = ScaleDown
		}
		reason := fmt.Sprintf("predictive scaling: predicted value %.2f, confidence %.2f", result.PredictedValue, result.Confidence)
		event := m.executeScale(policy.ServiceName, direction, currentReplicas, result.RecommendedReps, StrategyPredict, reason, string(policy.MetricType), result.PredictedValue)
		if event.Success {
			policy.LastScaleTime = time.Now()
		}
	}
}

// evaluateSchedule 定时策略评估
func (m *Manager) evaluateSchedule(policy *ScalePolicy) {
	now := time.Now()

	for _, rule := range policy.Schedules {
		if !rule.Enabled {
			continue
		}
		if !rule.StartDate.IsZero() && now.Before(rule.StartDate) {
			continue
		}
		if !rule.EndDate.IsZero() && now.After(rule.EndDate) {
			continue
		}

		// 简化：检查当前时间是否匹配（实际应解析 cron 表达式）
		if m.matchCron(rule.CronExpr, now) {
			m.mu.RLock()
			container, exists := m.containers[policy.ServiceName]
			if !exists {
				m.mu.RUnlock()
				continue
			}
			currentReplicas := container.Replicas
			m.mu.RUnlock()

			if rule.Replicas != currentReplicas {
				var direction ScaleDirection
				if rule.Replicas > currentReplicas {
					direction = ScaleUp
				} else {
					direction = ScaleDown
				}
				reason := fmt.Sprintf("schedule rule %s triggered", rule.Name)
				m.executeScale(policy.ServiceName, direction, currentReplicas, rule.Replicas, StrategySchedule, reason, "", 0)
			}
		}
	}
}

// matchCron 简化的 cron 匹配（实际应使用 cron 库）
func (m *Manager) matchCron(expr string, t time.Time) bool {
	// 简化实现：每小时匹配
	return t.Minute() == 0
}

// getAverageMetric 获取平均指标值
func (m *Manager) getAverageMetric(serviceName string, metricType MetricType, periods int) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if periods <= 0 {
		periods = 3
	}

	cutoff := time.Now().Add(-time.Duration(periods*m.config.MetricsIntervalSec) * time.Second)
	count := 0
	sum := 0.0

	for i := len(m.metrics) - 1; i >= 0; i-- {
		if m.metrics[i].Timestamp.Before(cutoff) {
			break
		}
		if m.metrics[i].ServiceName == serviceName && m.metrics[i].Type == metricType {
			sum += m.metrics[i].Value
			count++
		}
	}

	if count == 0 {
		return -1
	}
	return sum / float64(count)
}

// checkCooldown 检查冷却期
func (m *Manager) checkCooldown(policy *ScalePolicy) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if policy.LastScaleTime.IsZero() {
		return true
	}

	cooldown := m.config.DefaultCooldownSec
	if policy.CooldownSec > 0 {
		cooldown = policy.CooldownSec
	}

	return time.Since(policy.LastScaleTime) >= time.Duration(cooldown)*time.Second
}

// checkQuota 检查配额
func (m *Manager) checkQuota(serviceName string, targetReplicas int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	quota, exists := m.quotas[serviceName]
	if !exists {
		return true // 没有配额限制
	}

	if targetReplicas > quota.MaxReplicas {
		return false
	}
	if targetReplicas < quota.MinReplicas {
		return false
	}
	return true
}

// executeScale 执行扩缩操作
func (m *Manager) executeScale(serviceName string, direction ScaleDirection, fromReplicas, toReplicas int, strategy ScaleStrategy, reason, metricName string, metricValue float64) *ScaleEvent {
	event := &ScaleEvent{
		ID:            generateID(),
		ServiceName:   serviceName,
		Direction:     direction,
		FromReplicas:  fromReplicas,
		ToReplicas:    toReplicas,
		Strategy:      strategy,
		Reason:        reason,
		TriggerMetric: metricName,
		MetricValue:   metricValue,
		CreatedAt:     time.Now(),
	}

	// 检查每小时扩缩次数限制
	if m.getRecentScaleCount(serviceName) >= m.config.MaxScaleEventsPerHour {
		event.Success = false
		event.Error = "scale rate limit exceeded"
		m.createAlert(serviceName, AlertWarning, "rate_limit", "扩缩操作频率超限")
		m.mu.Lock()
		m.events = append(m.events, event)
		m.mu.Unlock()
		return event
	}

	// 更新容器副本数
	m.mu.Lock()
	container, exists := m.containers[serviceName]
	if exists {
		container.Replicas = toReplicas
		container.UpdatedAt = time.Now()
	}
	m.mu.Unlock()

	if !exists {
		event.Success = false
		event.Error = "container not found"
	} else {
		event.Success = true
		m.logger.Info("scale event",
			zap.String("service", serviceName),
			zap.String("direction", string(direction)),
			zap.Int("from", fromReplicas),
			zap.Int("to", toReplicas),
			zap.String("reason", reason))
	}

	m.mu.Lock()
	m.events = append(m.events, event)
	// 限制事件历史
	if len(m.events) > 10000 {
		m.events = m.events[len(m.events)-5000:]
	}
	m.mu.Unlock()

	return event
}

// getRecentScaleCount 获取近期扩缩次数
func (m *Manager) getRecentScaleCount(serviceName string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cutoff := time.Now().Add(-1 * time.Hour)
	count := 0
	for i := len(m.events) - 1; i >= 0; i-- {
		if m.events[i].CreatedAt.Before(cutoff) {
			break
		}
		if m.events[i].ServiceName == serviceName && m.events[i].Success {
			count++
		}
	}
	return count
}

// createAlert 创建告警
func (m *Manager) createAlert(serviceName string, level AlertLevel, title, message string) {
	alert := &Alert{
		ID:          generateID(),
		ServiceName: serviceName,
		Level:       level,
		Title:       title,
		Message:     message,
		Resolved:    false,
		CreatedAt:   time.Now(),
	}

	m.mu.Lock()
	m.alerts = append(m.alerts, alert)
	if len(m.alerts) > 1000 {
		m.alerts = m.alerts[len(m.alerts)-500:]
	}
	m.mu.Unlock()

	m.logger.Warn("alert created",
		zap.String("service", serviceName),
		zap.String("level", string(level)),
		zap.String("title", title))
}

// updatePredictor 更新预测器
func (m *Manager) updatePredictor(mp MetricPoint) {
	key := mp.ServiceName + ":" + string(mp.Type)

	m.mu.Lock()
	defer m.mu.Unlock()

	p, exists := m.predictors[key]
	if !exists {
		p = &predictor{window: 60}
		m.predictors[key] = p
	}

	p.values = append(p.values, mp.Value)
	p.timestamps = append(p.timestamps, mp.Timestamp)

	// 限制窗口大小
	if len(p.values) > p.window {
		p.values = p.values[len(p.values)-p.window:]
		p.timestamps = p.timestamps[len(p.timestamps)-p.window:]
	}
}

// Predict 预测指标值
func (m *Manager) Predict(ctx context.Context, serviceName string, metricType MetricType, horizon string) *PredictResult {
	key := serviceName + ":" + string(metricType)

	m.mu.RLock()
	p, exists := m.predictors[key]
	if !exists || len(p.values) < 10 {
		m.mu.RUnlock()
		return nil
	}
	values := make([]float64, len(p.values))
	copy(values, p.values)
	m.mu.RUnlock()

	// 简单线性回归预测
	n := len(values)
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for i, v := range values {
		x := float64(i)
		sumX += x
		sumY += v
		sumXY += x * v
		sumX2 += x * x
	}

	denom := float64(n)*sumX2 - sumX*sumX
	if denom == 0 {
		return nil
	}

	slope := (float64(n)*sumXY - sumX*sumY) / denom
	intercept := (sumY - slope*sumX) / float64(n)

	// 预测未来值
	futureX := float64(n + 5) // 预测5个时间点后
	predicted := slope*futureX + intercept

	// 计算置信度（基于残差）
	residualSum := 0.0
	for i, v := range values {
		estimated := slope*float64(i) + intercept
		diff := v - estimated
		residualSum += diff * diff
	}
	rmse := math.Sqrt(residualSum / float64(n))
	confidence := math.Max(0, 1.0-rmse/(math.Abs(sumY/float64(n))+1))

	// 根据预测值计算推荐副本数
	m.mu.RLock()
	container, exists := m.containers[serviceName]
	currentReplicas := 1
	if exists {
		currentReplicas = container.Replicas
	}
	m.mu.RUnlock()

	recommendedReps := currentReplicas
	if metricType == MetricCPU || metricType == MetricMemory {
		if predicted > 80 {
			recommendedReps = int(math.Ceil(predicted / 70.0 * float64(currentReplicas)))
		} else if predicted < 30 && currentReplicas > 1 {
			recommendedReps = int(math.Max(1, float64(currentReplicas)*predicted/50.0))
		}
	}

	return &PredictResult{
		ServiceName:     serviceName,
		PredictedValue:  predicted,
		Confidence:      confidence,
		RecommendedReps: recommendedReps,
		Horizon:         horizon,
		CreatedAt:       time.Now(),
	}
}

// RegisterContainer 注册容器
func (m *Manager) RegisterContainer(c *Container) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c.UpdatedAt = time.Now()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	m.containers[c.ServiceName] = c
	m.logger.Info("container registered", zap.String("service", c.ServiceName))
}

// UnregisterContainer 注销容器
func (m *Manager) UnregisterContainer(serviceName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.containers, serviceName)
	delete(m.policies, serviceName)
	delete(m.quotas, serviceName)
	m.logger.Info("container unregistered", zap.String("service", serviceName))
}

// ListContainers 列出所有容器
func (m *Manager) ListContainers() []*Container {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Container, 0, len(m.containers))
	for _, c := range m.containers {
		cp := *c
		result = append(result, &cp)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ServiceName < result[j].ServiceName
	})
	return result
}

// ManualScale 手动扩缩
func (m *Manager) ManualScale(ctx context.Context, req *ScaleRequest) (*ScaleEvent, error) {
	m.mu.RLock()
	container, exists := m.containers[req.ServiceName]
	if !exists {
		m.mu.RUnlock()
		return nil, fmt.Errorf("service %s not found", req.ServiceName)
	}
	currentReplicas := container.Replicas
	m.mu.RUnlock()

	if !m.checkQuota(req.ServiceName, req.Replicas) {
		return nil, fmt.Errorf("target replicas %d exceeds quota", req.Replicas)
	}

	var direction ScaleDirection
	if req.Replicas > currentReplicas {
		direction = ScaleUp
	} else if req.Replicas < currentReplicas {
		direction = ScaleDown
	} else {
		return nil, fmt.Errorf("target replicas same as current")
	}

	reason := req.Reason
	if reason == "" {
		reason = "manual scale"
	}

	event := m.executeScale(req.ServiceName, direction, currentReplicas, req.Replicas, StrategyManual, reason, "", 0)
	if !event.Success {
		return event, fmt.Errorf("scale failed: %s", event.Error)
	}
	return event, nil
}

// SetPolicy 设置扩缩策略
func (m *Manager) SetPolicy(policy *ScalePolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy.UpdatedAt = time.Now()
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = time.Now()
	}
	if policy.ID == "" {
		policy.ID = generateID()
	}
	m.policies[policy.ServiceName] = policy
	m.logger.Info("policy set", zap.String("service", policy.ServiceName), zap.String("strategy", string(policy.Strategy)))
}

// GetPolicy 获取策略
func (m *Manager) GetPolicy(serviceName string) (*ScalePolicy, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.policies[serviceName]
	if !ok {
		return nil, false
	}
	cp := *p
	return &cp, true
}

// DeletePolicy 删除策略
func (m *Manager) DeletePolicy(serviceName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.policies, serviceName)
}

// ListPolicies 列出所有策略
func (m *Manager) ListPolicies() []*ScalePolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ScalePolicy, 0, len(m.policies))
	for _, p := range m.policies {
		cp := *p
		result = append(result, &cp)
	}
	return result
}

// SetQuota 设置资源配额
func (m *Manager) SetQuota(quota *ResourceQuota) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if quota.ID == "" {
		quota.ID = generateID()
	}
	m.quotas[quota.ServiceName] = quota
	m.logger.Info("quota set", zap.String("service", quota.ServiceName))
}

// GetQuota 获取配额
func (m *Manager) GetQuota(serviceName string) (*ResourceQuota, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	q, ok := m.quotas[serviceName]
	if !ok {
		return nil, false
	}
	cp := *q
	return &cp, true
}

// DeleteQuota 删除配额
func (m *Manager) DeleteQuota(serviceName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.quotas, serviceName)
}

// ListQuotas 列出所有配额
func (m *Manager) ListQuotas() []*ResourceQuota {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ResourceQuota, 0, len(m.quotas))
	for _, q := range m.quotas {
		cp := *q
		result = append(result, &cp)
	}
	return result
}

// RecordMetric 记录指标
func (m *Manager) RecordMetric(mp MetricPoint) {
	m.mu.Lock()
	m.metrics = append(m.metrics, mp)
	if len(m.metrics) > 100000 {
		m.metrics = m.metrics[len(m.metrics)-50000:]
	}
	m.mu.Unlock()

	m.updatePredictor(mp)
}

// GetMetrics 获取指标
func (m *Manager) GetMetrics(query *MetricsQuery) []MetricPoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]MetricPoint, 0)
	for _, mp := range m.metrics {
		if query.ServiceName != "" && mp.ServiceName != query.ServiceName {
			continue
		}
		if query.MetricType != "" && mp.Type != query.MetricType {
			continue
		}
		if !query.StartTime.IsZero() && mp.Timestamp.Before(query.StartTime) {
			continue
		}
		if !query.EndTime.IsZero() && mp.Timestamp.After(query.EndTime) {
			continue
		}
		result = append(result, mp)
	}
	return result
}

// GetScaleEvents 获取扩缩历史
func (m *Manager) GetScaleEvents(serviceName string, limit int) []*ScaleEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	result := make([]*ScaleEvent, 0)
	for i := len(m.events) - 1; i >= 0; i-- {
		if serviceName != "" && m.events[i].ServiceName != serviceName {
			continue
		}
		result = append(result, m.events[i])
		if len(result) >= limit {
			break
		}
	}
	return result
}

// GetAlerts 获取告警
func (m *Manager) GetAlerts(resolved bool, limit int) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	result := make([]*Alert, 0)
	for i := len(m.alerts) - 1; i >= 0; i-- {
		if m.alerts[i].Resolved != resolved {
			continue
		}
		result = append(result, m.alerts[i])
		if len(result) >= limit {
			break
		}
	}
	return result
}

// ResolveAlert 解决告警
func (m *Manager) ResolveAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, a := range m.alerts {
		if a.ID == alertID {
			now := time.Now()
			a.Resolved = true
			a.ResolvedAt = &now
			return nil
		}
	}
	return fmt.Errorf("alert %s not found", alertID)
}

// GenerateCostSuggestions 生成成本优化建议
func (m *Manager) GenerateCostSuggestions() []*CostSuggestion {
	m.mu.RLock()
	containers := make([]*Container, 0, len(m.containers))
	for _, c := range m.containers {
		cp := *c
		containers = append(containers, &cp)
	}
	m.mu.RUnlock()

	suggestions := make([]*CostSuggestion, 0)

	for _, c := range containers {
		// 检查是否有低利用率
		avgCPU := m.getAverageMetric(c.ServiceName, MetricCPU, 10)
		avgMem := m.getAverageMetric(c.ServiceName, MetricMemory, 10)

		if avgCPU > 0 && avgCPU < 20 && c.Replicas > c.MinReplicas {
			suggestions = append(suggestions, &CostSuggestion{
				ID:            generateID(),
				ServiceName:   c.ServiceName,
				Type:          "rightsize",
				Description:   fmt.Sprintf("CPU 平均使用率仅 %.1f%%，建议减少副本数", avgCPU),
				EstimatedSave: float64(c.Replicas-c.MinReplicas) * 10.0, // 简化成本计算
				Priority:      "medium",
				CreatedAt:     time.Now(),
			})
		}

		if avgMem > 0 && avgMem < 30 && c.Replicas > c.MinReplicas {
			suggestions = append(suggestions, &CostSuggestion{
				ID:            generateID(),
				ServiceName:   c.ServiceName,
				Type:          "rightsize",
				Description:   fmt.Sprintf("内存平均使用率仅 %.1f%%，建议减少副本数或降低资源配置", avgMem),
				EstimatedSave: float64(c.Replicas-c.MinReplicas) * 8.0,
				Priority:      "low",
				CreatedAt:     time.Now(),
			})
		}

		// 检查是否有定时策略可用于降本
		m.mu.RLock()
		policy, exists := m.policies[c.ServiceName]
		hasSchedule := exists && len(policy.Schedules) > 0
		isThreshold := exists && policy.Strategy == StrategyThreshold
		m.mu.RUnlock()

		if isThreshold && !hasSchedule {
			suggestions = append(suggestions, &CostSuggestion{
				ID:            generateID(),
				ServiceName:   c.ServiceName,
				Type:          "schedule",
				Description:   "建议配置定时策略，在低峰期自动缩减副本",
				EstimatedSave: 15.0,
				Priority:      "low",
				CreatedAt:     time.Now(),
			})
		}
	}

	m.mu.Lock()
	m.suggestions = suggestions
	m.mu.Unlock()

	return suggestions
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *AutoScaleConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *AutoScaleConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}

// cleanupLoop 清理过期数据
func (m *Manager) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.cleanup()
		}
	}
}

// cleanup 清理过期指标和事件
func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -m.config.HistoryRetentionDays)

	// 清理过期指标
	validMetrics := make([]MetricPoint, 0)
	for _, mp := range m.metrics {
		if mp.Timestamp.After(cutoff) {
			validMetrics = append(validMetrics, mp)
		}
	}
	m.metrics = validMetrics

	// 清理过期事件
	validEvents := make([]*ScaleEvent, 0)
	for _, e := range m.events {
		if e.CreatedAt.After(cutoff) {
			validEvents = append(validEvents, e)
		}
	}
	m.events = validEvents

	m.logger.Info("cleanup completed",
		zap.Int("metrics", len(m.metrics)),
		zap.Int("events", len(m.events)))
}
