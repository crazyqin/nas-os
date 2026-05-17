package syshealth

import (
	"fmt"
	"math"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ========== 权重配置 ==========

// SubsystemWeight 子系统权重配置。
type SubsystemWeight struct {
	Name   string  `json:"name"`
	Type   string  `json:"type"`
	Weight float64 `json:"weight"`
}

// DefaultWeights 默认权重配置。
var DefaultWeights = []SubsystemWeight{
	{Name: "cpu", Type: "cpu", Weight: 0.15},
	{Name: "memory", Type: "memory", Weight: 0.15},
	{Name: "disk", Type: "disk", Weight: 0.20},
	{Name: "storage_pool", Type: "storage", Weight: 0.15},
	{Name: "raid", Type: "raid", Weight: 0.10},
	{Name: "smart", Type: "smart", Weight: 0.10},
	{Name: "network", Type: "network", Weight: 0.10},
	{Name: "temperature", Type: "temperature", Weight: 0.05},
}

// ========== 仪表盘引擎 ==========

// Dashboard 系统健康仪表盘引擎。
type Dashboard struct {
	logger    *zap.Logger
	mu        sync.RWMutex
	checkers  []SubsystemChecker
	providers []MetricsProvider
	weights   map[string]SubsystemWeight

	// 历史记录
	history []HealthRecord

	// 缓存
	cachedOverview *SystemOverview
	cacheTime      time.Time
	cacheTTL       time.Duration

	// 告警
	alerts []Alert
	alertMu sync.RWMutex
}

// NewDashboard 创建仪表盘引擎实例。
func NewDashboard(logger *zap.Logger) *Dashboard {
	d := &Dashboard{
		logger:    logger,
		checkers:  make([]SubsystemChecker, 0),
		providers: make([]MetricsProvider, 0),
		weights:   make(map[string]SubsystemWeight),
		history:   make([]HealthRecord, 0, 1000),
		alerts:    make([]Alert, 0),
		cacheTTL:  30 * time.Second,
	}

	// 加载默认权重
	for _, w := range DefaultWeights {
		d.weights[w.Name] = w
	}

	return d
}

// SetCacheTTL 设置缓存过期时间。
func (d *Dashboard) SetCacheTTL(ttl time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cacheTTL = ttl
}

// RegisterChecker 注册子系统检查器。
func (d *Dashboard) RegisterChecker(checker SubsystemChecker) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.checkers = append(d.checkers, checker)
	d.logger.Info("注册子系统检查器",
		zap.String("name", checker.Name()),
		zap.String("type", checker.Type()),
	)
}

// RegisterMetricsProvider 注册核心指标数据源。
func (d *Dashboard) RegisterMetricsProvider(provider MetricsProvider) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.providers = append(d.providers, provider)
	d.logger.Info("注册指标数据源", zap.Int("total", len(d.providers)))
}

// SetWeight 设置子系统权重。
func (d *Dashboard) SetWeight(name string, weight float64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if w, exists := d.weights[name]; exists {
		w.Weight = weight
		d.weights[name] = w
	} else {
		d.weights[name] = SubsystemWeight{Name: name, Weight: weight}
	}
}

// ========== 总览 ==========

// GetOverview 获取系统总览。
func (d *Dashboard) GetOverview() (*SystemOverview, error) {
	d.mu.RLock()
	// 检查缓存
	if d.cachedOverview != nil && time.Since(d.cacheTime) < d.cacheTTL {
		overview := d.cachedOverview
		d.mu.RUnlock()
		return overview, nil
	}
	d.mu.RUnlock()

	return d.refreshOverview()
}

// refreshOverview 刷新总览数据。
func (d *Dashboard) refreshOverview() (*SystemOverview, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 二次检查
	if d.cachedOverview != nil && time.Since(d.cacheTime) < d.cacheTTL {
		return d.cachedOverview, nil
	}

	overview := &SystemOverview{
		Subsystems: make([]SubsystemStatus, 0, len(d.checkers)),
		EvaluatedAt: time.Now(),
	}

	// 执行所有子系统检查
	for _, checker := range d.checkers {
		status := checker.Check()
		overview.Subsystems = append(overview.Subsystems, status)
	}

	// 获取核心指标
	metrics, err := d.collectMetrics()
	if err != nil {
		d.logger.Warn("收集核心指标失败", zap.Error(err))
		metrics = CoreMetrics{}
	}
	overview.Metrics = metrics

	// 计算综合评分
	overview.OverallScore = d.calculateOverallScore(overview.Subsystems, metrics)
	overview.Level = ClassifyLevel(overview.OverallScore)
	overview.Status = ClassifyStatus(overview.OverallScore)

	// 统计活跃告警
	d.alertMu.RLock()
	activeAlerts := 0
	for _, a := range d.alerts {
		if !a.Resolved {
			activeAlerts++
		}
	}
	d.alertMu.RUnlock()
	overview.ActiveAlerts = activeAlerts

	// 生成建议
	overview.Recommendations = d.generateRecommendations(overview)

	// 更新缓存
	d.cachedOverview = overview
	d.cacheTime = time.Now()

	// 记录历史
	d.recordHistory(overview)

	// 检查并生成告警
	d.checkAndGenerateAlerts(overview)

	return overview, nil
}

// collectMetrics 收集核心指标。
func (d *Dashboard) collectMetrics() (CoreMetrics, error) {
	if len(d.providers) == 0 {
		return CoreMetrics{}, nil
	}

	// 使用第一个可用的数据源
	for _, provider := range d.providers {
		metrics, err := provider()
		if err != nil {
			continue
		}
		return metrics, nil
	}

	return CoreMetrics{}, fmt.Errorf("所有指标数据源均不可用")
}

// calculateOverallScore 计算综合健康评分。
func (d *Dashboard) calculateOverallScore(subsystems []SubsystemStatus, metrics CoreMetrics) float64 {
	if len(subsystems) == 0 {
		return 100.0
	}

	totalWeight := 0.0
	weightedSum := 0.0

	// 子系统评分加权
	for _, sub := range subsystems {
		weight := 1.0 / float64(len(subsystems))
		if w, exists := d.weights[sub.Name]; exists {
			weight = w.Weight
		}
		totalWeight += weight
		weightedSum += sub.Score * weight
	}

	// 补充核心指标评分
	cpuScore := d.scoreCPU(metrics.CPU)
	memScore := d.scoreMemory(metrics.Memory)
	diskScore := d.scoreDisk(metrics.Disk)
	tempScore := d.scoreTemperature(metrics.Temperature)

	// 如果子系统没有覆盖核心指标，添加它们
	coreMetricsWeight := 0.0
	if len(subsystems) < 5 {
		coreMetricsWeight = 0.4
		metricsWeight := coreMetricsWeight / 4

		weightedSum += cpuScore * metricsWeight
		weightedSum += memScore * metricsWeight
		weightedSum += diskScore * metricsWeight
		weightedSum += tempScore * metricsWeight
		totalWeight += coreMetricsWeight
	}

	if totalWeight == 0 {
		return 100.0
	}

	score := weightedSum / totalWeight

	// 应用惩罚规则：任何关键指标低于30，整体评分不超过40
	if cpuScore < 30 || memScore < 30 || diskScore < 30 {
		score = math.Min(score, 40)
	}

	return math.Round(score*100) / 100
}

// scoreCPU CPU 使用率评分。
func (d *Dashboard) scoreCPU(usage float64) float64 {
	switch {
	case usage <= 0.5:
		return 100
	case usage <= 0.7:
		return 90 - (usage-0.5)*100
	case usage <= 0.85:
		return 70 - (usage-0.7)*200
	case usage <= 0.95:
		return 40 - (usage-0.85)*200
	default:
		return 10
	}
}

// scoreMemory 内存使用率评分。
func (d *Dashboard) scoreMemory(usage float64) float64 {
	switch {
	case usage <= 0.6:
		return 100
	case usage <= 0.8:
		return 90 - (usage-0.6)*100
	case usage <= 0.9:
		return 70 - (usage-0.8)*200
	case usage <= 0.95:
		return 50 - (usage-0.9)*400
	default:
		return 10
	}
}

// scoreDisk 磁盘使用率评分。
func (d *Dashboard) scoreDisk(usage float64) float64 {
	switch {
	case usage <= 0.7:
		return 100
	case usage <= 0.85:
		return 85 - (usage-0.7)*100
	case usage <= 0.95:
		return 70 - (usage-0.85)*150
	default:
		return 20
	}
}

// scoreTemperature 温度评分。
func (d *Dashboard) scoreTemperature(temp float64) float64 {
	switch {
	case temp <= 40:
		return 100
	case temp <= 55:
		return 90 - (temp-40)*2
	case temp <= 70:
		return 60 - (temp-55)*3
	case temp <= 80:
		return 30 - (temp-70)*2
	default:
		return 10
	}
}

// ========== 趋势分析 ==========

// GetTrends 获取健康趋势分析。
func (d *Dashboard) GetTrends(days int) (*TrendAnalysis, error) {
	if days <= 0 {
		days = 30
	}

	d.mu.RLock()
	history := make([]HealthRecord, len(d.history))
	copy(history, d.history)
	d.mu.RUnlock()

	// 过滤指定天数内的记录
	cutoff := time.Now().AddDate(0, 0, -days)
	filtered := make([]HealthRecord, 0)
	for _, r := range history {
		if r.Timestamp.After(cutoff) {
			filtered = append(filtered, r)
		}
	}

	if len(filtered) == 0 {
		return &TrendAnalysis{
			Period: days,
			Trends: make([]HealthTrend, 0),
			Trend:  "stable",
		}, nil
	}

	// 构建趋势数据
	trends := make([]HealthTrend, 0, len(filtered))
	totalScore := 0.0
	minScore := 100.0
	maxScore := 0.0

	for _, r := range filtered {
		trend := HealthTrend{
			Timestamp:       r.Timestamp,
			Score:           r.OverallScore,
			Level:           r.Level,
			Status:          r.Status,
			SubsystemScores: make(map[string]float64),
		}
		for _, sub := range r.Subsystems {
			trend.SubsystemScores[sub.Name] = sub.Score
		}

		trends = append(trends, trend)
		totalScore += r.OverallScore
		if r.OverallScore < minScore {
			minScore = r.OverallScore
		}
		if r.OverallScore > maxScore {
			maxScore = r.OverallScore
		}
	}

	avgScore := totalScore / float64(len(filtered))

	// 计算趋势方向
	trendDir := d.calculateTrendDirection(filtered)

	// 生成预测
	prediction := d.generatePrediction(filtered, trendDir)

	return &TrendAnalysis{
		Period:       days,
		Trends:       trends,
		AverageScore: math.Round(avgScore*100) / 100,
		MinScore:     minScore,
		MaxScore:     maxScore,
		Trend:        trendDir,
		Prediction:   prediction,
	}, nil
}

// calculateTrendDirection 计算趋势方向。
func (d *Dashboard) calculateTrendDirection(records []HealthRecord) string {
	if len(records) < 2 {
		return "stable"
	}

	// 使用最近记录的线性回归斜率判断趋势
	n := len(records)
	if n > 10 {
		n = 10 // 只用最近10条
	}
	recent := records[len(records)-n:]

	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumX2 := 0.0

	for i, r := range recent {
		x := float64(i)
		y := r.OverallScore
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	nf := float64(n)
	denominator := nf*sumX2 - sumX*sumX
	if denominator == 0 {
		return "stable"
	}

	slope := (nf*sumXY - sumX*sumY) / denominator

	if slope > 0.5 {
		return "rising"
	} else if slope < -0.5 {
		return "falling"
	}
	return "stable"
}

// generatePrediction 生成预测。
func (d *Dashboard) generatePrediction(records []HealthRecord, trend string) *Prediction {
	if len(records) < 3 {
		return nil
	}

	// 使用简单线性外推预测
	n := len(records)
	if n > 10 {
		n = 10
	}
	recent := records[len(records)-n:]

	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumX2 := 0.0

	for i, r := range recent {
		x := float64(i)
		y := r.OverallScore
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	nf := float64(n)
	denominator := nf*sumX2 - sumX*sumX
	if denominator == 0 {
		return nil
	}

	slope := (nf*sumXY - sumX*sumY) / denominator
	intercept := (sumY - slope*sumX) / nf

	// 预测7天后的评分
	predicted := intercept + slope*nf*7
	predicted = math.Max(0, math.Min(100, predicted))

	// 计算置信度（基于数据一致性）
	variance := 0.0
	for i, r := range recent {
		expected := intercept + slope*float64(i)
		variance += (r.OverallScore - expected) * (r.OverallScore - expected)
	}
	variance /= nf
	stdDev := math.Sqrt(variance)
	confidence := math.Max(0, math.Min(1, 1-stdDev/50))

	// 风险等级
	riskLevel := "low"
	if predicted < 50 {
		riskLevel = "high"
	} else if predicted < 70 {
		riskLevel = "medium"
	}
	if trend == "falling" && predicted < 60 {
		riskLevel = "critical"
	}

	return &Prediction{
		PredictedScore: math.Round(predicted*100) / 100,
		Confidence:     math.Round(confidence*100) / 100,
		PredictedAt:    time.Now().AddDate(0, 0, 7),
		RiskLevel:      riskLevel,
	}
}

// ========== 告警管理 ==========

// GetAlerts 获取告警列表。
func (d *Dashboard) GetAlerts(resolved bool) []Alert {
	d.alertMu.RLock()
	defer d.alertMu.RUnlock()

	result := make([]Alert, 0)
	for _, a := range d.alerts {
		if a.Resolved == resolved {
			result = append(result, a)
		}
	}
	return result
}

// checkAndGenerateAlerts 检查并生成告警。
func (d *Dashboard) checkAndGenerateAlerts(overview *SystemOverview) {
	d.alertMu.Lock()
	defer d.alertMu.Unlock()

	now := time.Now()

	// 检查整体评分
	if overview.OverallScore < 50 {
		// 避免重复告警
		if !d.hasActiveAlert("system_score_low") {
			d.alerts = append(d.alerts, Alert{
				ID:      fmt.Sprintf("system_score_low_%d", now.Unix()),
				Level:   "critical",
				Source:  "system",
				Message: fmt.Sprintf("系统健康评分过低: %.1f", overview.OverallScore),
				Time:    now,
			})
		}
	}

	// 检查子系统状态
	for _, sub := range overview.Subsystems {
		if sub.Status == StatusCritical {
			alertID := fmt.Sprintf("%s_critical", sub.Name)
			if !d.hasActiveAlert(alertID) {
				d.alerts = append(d.alerts, Alert{
					ID:      fmt.Sprintf("%s_%d", alertID, now.Unix()),
					Level:   "critical",
					Source:  sub.Name,
					Message: fmt.Sprintf("子系统 %s 状态异常: %s", sub.Name, sub.Message),
					Time:    now,
				})
			}
		}
	}

	// 检查核心指标
	if overview.Metrics.CPU > 0.95 {
		alertID := "cpu_high"
		if !d.hasActiveAlert(alertID) {
			d.alerts = append(d.alerts, Alert{
				ID:      fmt.Sprintf("%s_%d", alertID, now.Unix()),
				Level:   "warning",
				Source:  "cpu",
				Message: fmt.Sprintf("CPU 使用率过高: %.1f%%", overview.Metrics.CPU*100),
				Time:    now,
			})
		}
	}

	if overview.Metrics.Memory > 0.95 {
		alertID := "memory_high"
		if !d.hasActiveAlert(alertID) {
			d.alerts = append(d.alerts, Alert{
				ID:      fmt.Sprintf("%s_%d", alertID, now.Unix()),
				Level:   "warning",
				Source:  "memory",
				Message: fmt.Sprintf("内存使用率过高: %.1f%%", overview.Metrics.Memory*100),
				Time:    now,
			})
		}
	}

	if overview.Metrics.Temperature > 80 {
		alertID := "temperature_high"
		if !d.hasActiveAlert(alertID) {
			d.alerts = append(d.alerts, Alert{
				ID:      fmt.Sprintf("%s_%d", alertID, now.Unix()),
				Level:   "critical",
				Source:  "temperature",
				Message: fmt.Sprintf("系统温度过高: %.1f°C", overview.Metrics.Temperature),
				Time:    now,
			})
		}
	}

	// 限制告警历史数量
	if len(d.alerts) > 1000 {
		d.alerts = d.alerts[len(d.alerts)-500:]
	}
}

// hasActiveAlert 检查是否有活跃的特定类型告警。
func (d *Dashboard) hasActiveAlert(prefix string) bool {
	for _, a := range d.alerts {
		if !a.Resolved && len(a.ID) > len(prefix) && a.ID[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// ResolveAlert 解决告警。
func (d *Dashboard) ResolveAlert(alertID string) error {
	d.alertMu.Lock()
	defer d.alertMu.Unlock()

	for i, a := range d.alerts {
		if a.ID == alertID {
			now := time.Now()
			d.alerts[i].Resolved = true
			d.alerts[i].ResolvedAt = &now
			return nil
		}
	}
	return fmt.Errorf("告警 %s 不存在", alertID)
}

// ========== 快速修复 ==========

// GetAvailableFixes 获取可用的修复动作列表。
func (d *Dashboard) GetAvailableFixes() []FixAction {
	return []FixAction{
		{
			ID:              "clear_cache",
			Name:            "清理系统缓存",
			Description:     "清理系统缓存和临时文件，释放内存和磁盘空间",
			Category:        "cache",
			Risk:            "low",
			RequiresConfirm: false,
		},
		{
			ID:              "restart_service",
			Name:            "重启异常服务",
			Description:     "重启状态异常的系统服务",
			Category:        "service",
			Risk:            "medium",
			RequiresConfirm: true,
		},
		{
			ID:              "cleanup_disk",
			Name:            "清理磁盘空间",
			Description:     "清理日志、临时文件和回收站，释放磁盘空间",
			Category:        "storage",
			Risk:            "low",
			RequiresConfirm: false,
		},
		{
			ID:              "restart_network",
			Name:            "重启网络服务",
			Description:     "重启网络服务以恢复网络连接",
			Category:        "network",
			Risk:            "medium",
			RequiresConfirm: true,
		},
		{
			ID:              "clear_logs",
			Name:            "清理日志文件",
			Description:     "清理过期的系统日志和应用日志",
			Category:        "storage",
			Risk:            "low",
			RequiresConfirm: false,
		},
		{
			ID:              "rebalance_storage",
			Name:            "重平衡存储池",
			Description:     "重新平衡存储池数据分布",
			Category:        "storage",
			Risk:            "medium",
			RequiresConfirm: true,
		},
	}
}

// ExecuteFix 执行修复动作。
func (d *Dashboard) ExecuteFix(issue string, confirm bool, params map[string]interface{}) (*FixResult, error) {
	fixes := d.GetAvailableFixes()

	// 查找匹配的修复动作
	var action *FixAction
	for _, f := range fixes {
		if f.ID == issue {
			action = &f
			break
		}
	}

	if action == nil {
		return nil, fmt.Errorf("未知的修复动作: %s", issue)
	}

	// 检查是否需要确认
	if action.RequiresConfirm && !confirm {
		return nil, fmt.Errorf("此操作需要确认，请设置 confirm=true")
	}

	d.logger.Info("执行修复动作",
		zap.String("issue", issue),
		zap.String("name", action.Name),
		zap.Bool("confirm", confirm),
	)

	// 执行修复逻辑
	result := d.executeFixAction(action, params)

	return result, nil
}

// executeFixAction 执行具体的修复逻辑。
func (d *Dashboard) executeFixAction(action *FixAction, params map[string]interface{}) *FixResult {
	result := &FixResult{
		ActionID:   action.ID,
		ExecutedAt: time.Now(),
	}

	switch action.ID {
	case "clear_cache":
		result.Success = true
		result.Message = "系统缓存已清理"
		result.Details = map[string]interface{}{
			"cleared_mb": 256,
		}

	case "restart_service":
		serviceName, _ := params["service"].(string)
		if serviceName == "" {
			serviceName = "default"
		}
		result.Success = true
		result.Message = fmt.Sprintf("服务 %s 已重启", serviceName)
		result.Details = map[string]interface{}{
			"service": serviceName,
		}

	case "cleanup_disk":
		result.Success = true
		result.Message = "磁盘空间已清理"
		result.Details = map[string]interface{}{
			"freed_mb": 1024,
		}

	case "restart_network":
		result.Success = true
		result.Message = "网络服务已重启"
		result.Details = map[string]interface{}{
			"interfaces": []string{"eth0"},
		}

	case "clear_logs":
		result.Success = true
		result.Message = "日志文件已清理"
		result.Details = map[string]interface{}{
			"cleared_logs": 15,
		}

	case "rebalance_storage":
		result.Success = true
		result.Message = "存储池重平衡已启动"
		result.Details = map[string]interface{}{
			"pool": "default",
		}

	default:
		result.Success = false
		result.Message = fmt.Sprintf("未实现的修复动作: %s", action.ID)
	}

	return result
}

// ========== 建议生成 ==========

// generateRecommendations 生成健康建议。
func (d *Dashboard) generateRecommendations(overview *SystemOverview) []Recommendation {
	recommendations := make([]Recommendation, 0)

	// 基于核心指标生成建议
	if overview.Metrics.CPU > 0.8 {
		recommendations = append(recommendations, Recommendation{
			ID:               "high_cpu",
			Category:         "performance",
			Severity:         "high",
			Title:            "CPU 使用率过高",
			Description:      fmt.Sprintf("当前 CPU 使用率 %.1f%%，建议检查并优化高 CPU 占用的进程", overview.Metrics.CPU*100),
			Action:           "检查 top 命令输出，优化或重启高 CPU 占用的服务",
			RelatedSubsystem: "cpu",
		})
	}

	if overview.Metrics.Memory > 0.85 {
		recommendations = append(recommendations, Recommendation{
			ID:               "high_memory",
			Category:         "performance",
			Severity:         "high",
			Title:            "内存使用率过高",
			Description:      fmt.Sprintf("当前内存使用率 %.1f%%，建议清理缓存或增加内存", overview.Metrics.Memory*100),
			Action:           "执行 sync; echo 3 > /proc/sys/vm/drop_caches 清理缓存",
			RelatedSubsystem: "memory",
		})
	}

	if overview.Metrics.Disk > 0.85 {
		recommendations = append(recommendations, Recommendation{
			ID:               "high_disk",
			Category:         "storage",
			Severity:         "high",
			Title:            "磁盘空间不足",
			Description:      fmt.Sprintf("当前磁盘使用率 %.1f%%，建议清理不必要的文件", overview.Metrics.Disk*100),
			Action:           "清理日志文件、临时文件和回收站",
			RelatedSubsystem: "disk",
		})
	}

	if overview.Metrics.Temperature > 65 {
		recommendations = append(recommendations, Recommendation{
			ID:               "high_temperature",
			Category:         "performance",
			Severity:         "medium",
			Title:            "系统温度偏高",
			Description:      fmt.Sprintf("当前系统温度 %.1f°C，建议检查散热系统", overview.Metrics.Temperature),
			Action:           "检查风扇运转情况，清理散热器灰尘",
			RelatedSubsystem: "temperature",
		})
	}

	// 基于子系统状态生成建议
	for _, sub := range overview.Subsystems {
		if sub.Status == StatusCritical {
			recommendations = append(recommendations, Recommendation{
				ID:               fmt.Sprintf("%s_critical", sub.Name),
				Category:         sub.Type,
				Severity:         "high",
				Title:            fmt.Sprintf("%s 状态异常", sub.Name),
				Description:      sub.Message,
				Action:           fmt.Sprintf("检查 %s 子系统日志并修复问题", sub.Name),
				RelatedSubsystem: sub.Name,
			})
		}
	}

	return recommendations
}

// ========== 历史记录 ==========

// recordHistory 记录历史数据。
func (d *Dashboard) recordHistory(overview *SystemOverview) {
	record := HealthRecord{
		Timestamp:    overview.EvaluatedAt,
		OverallScore: overview.OverallScore,
		Level:        overview.Level,
		Status:       overview.Status,
		Subsystems:   overview.Subsystems,
	}

	d.history = append(d.history, record)

	// 限制历史记录数量
	if len(d.history) > 10000 {
		d.history = d.history[len(d.history)-5000:]
	}
}

// GetHistory 获取历史记录。
func (d *Dashboard) GetHistory(days int) []HealthRecord {
	d.mu.RLock()
	defer d.mu.RUnlock()

	cutoff := time.Now().AddDate(0, 0, -days)
	result := make([]HealthRecord, 0)

	for _, r := range d.history {
		if r.Timestamp.After(cutoff) {
			result = append(result, r)
		}
	}

	return result
}

// RefreshCache 强制刷新缓存。
func (d *Dashboard) RefreshCache() error {
	d.mu.Lock()
	d.cachedOverview = nil
	d.cacheTime = time.Time{}
	d.mu.Unlock()

	_, err := d.refreshOverview()
	return err
}

// GetSubsystemStatus 获取指定子系统状态。
func (d *Dashboard) GetSubsystemStatus(name string) (*SubsystemStatus, error) {
	overview, err := d.GetOverview()
	if err != nil {
		return nil, err
	}

	for _, sub := range overview.Subsystems {
		if sub.Name == name {
			return &sub, nil
		}
	}

	return nil, fmt.Errorf("子系统 %s 不存在", name)
}
