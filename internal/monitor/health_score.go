package monitor

import (
	"math"
	"sync"
	"time"
)

// HealthScorer 系统健康评分器
type HealthScorer struct {
	mu         sync.RWMutex
	manager    *Manager
	history    []*HealthScore
	maxHistory int
	weights    HealthWeights
	thresholds HealthThresholds
	lastScore  *HealthScore
	collector  *MetricsCollector
}

// HealthWeights 健康评分权重
type HealthWeights struct {
	CPU     float64 `json:"cpu"`     // CPU 权重
	Memory  float64 `json:"memory"`  // 内存权重
	Disk    float64 `json:"disk"`    // 磁盘权重
	Network float64 `json:"network"` // 网络权重
	SMART   float64 `json:"smart"`   // SMART 权重
	Uptime  float64 `json:"uptime"`  // 运行时间权重
}

// HealthThresholds 健康评分阈值
type HealthThresholds struct {
	CPUWarning      float64 `json:"cpu_warning"`       // CPU 警告阈值
	CPUCritical     float64 `json:"cpu_critical"`      // CPU 严重阈值
	MemoryWarning   float64 `json:"memory_warning"`    // 内存警告阈值
	MemoryCritical  float64 `json:"memory_critical"`   // 内存严重阈值
	DiskWarning     float64 `json:"disk_warning"`      // 磁盘警告阈值
	DiskCritical    float64 `json:"disk_critical"`     // 磁盘严重阈值
	LoadPerCore     float64 `json:"load_per_core"`     // 每核心负载阈值
	SwapUsage       float64 `json:"swap_usage"`        // Swap 使用阈值
	MaxTemperature  int     `json:"max_temperature"`   // 最高温度
	UptimeGraceDays int     `json:"uptime_grace_days"` // 运行时间宽限天数
}

// HealthScore 健康评分
type HealthScore struct {
	TotalScore      float64            `json:"total_score"`       // 总分 0-100
	Grade           string             `json:"grade"`             // 等级 A/B/C/D/F
	Components      ComponentScores    `json:"components"`        // 组件评分
	Issues          []HealthIssue      `json:"issues"`            // 问题列表
	Recommendations []string           `json:"recommendations"`   // 建议列表
	Timestamp       time.Time          `json:"timestamp"`         // 时间戳
	Trend           ScoreTrend         `json:"trend"`             // 趋势
	Details         map[string]float64 `json:"details,omitempty"` // 详细数据
}

// ComponentScores 组件评分
type ComponentScores struct {
	CPU     ComponentHealth `json:"cpu"`
	Memory  ComponentHealth `json:"memory"`
	Disk    ComponentHealth `json:"disk"`
	Network ComponentHealth `json:"network"`
	SMART   ComponentHealth `json:"smart"`
	System  ComponentHealth `json:"system"`
}

// ComponentHealth 组件健康状态
type ComponentHealth struct {
	Score     float64 `json:"score"`     // 评分 0-100
	Status    string  `json:"status"`    // healthy, warning, critical
	Message   string  `json:"message"`   // 状态消息
	Weight    float64 `json:"weight"`    // 权重
	Threshold float64 `json:"threshold"` // 阈值
}

// HealthIssue 健康问题
type HealthIssue struct {
	Component string    `json:"component"` // 组件名称
	Severity  string    `json:"severity"`  // 问题严重程度
	Message   string    `json:"message"`   // 问题描述
	Value     float64   `json:"value"`     // 当前值
	Threshold float64   `json:"threshold"` // 阈值
	Timestamp time.Time `json:"timestamp"` // 发现时间
}

// ScoreTrend 评分趋势
type ScoreTrend struct {
	Direction    string  `json:"direction"`     // up, down, stable
	Change       float64 `json:"change"`        // 变化值
	PreviousHour float64 `json:"previous_hour"` // 上小时分数
	PreviousDay  float64 `json:"previous_day"`  // 上天分数
}

// NewHealthScorer 创建健康评分器
func NewHealthScorer(manager *Manager) *HealthScorer {
	scorer := &HealthScorer{
		manager:    manager,
		history:    make([]*HealthScore, 0),
		maxHistory: 1000,
		weights: HealthWeights{
			CPU:     0.25,
			Memory:  0.25,
			Disk:    0.25,
			Network: 0.10,
			SMART:   0.10,
			Uptime:  0.05,
		},
		thresholds: HealthThresholds{
			CPUWarning:      70,
			CPUCritical:     90,
			MemoryWarning:   75,
			MemoryCritical:  90,
			DiskWarning:     80,
			DiskCritical:    95,
			LoadPerCore:     2.0,
			SwapUsage:       50,
			MaxTemperature:  60,
			UptimeGraceDays: 1,
		},
	}

	scorer.collector = NewMetricsCollector(manager, scorer)

	return scorer
}

// SetWeights 设置权重
func (hs *HealthScorer) SetWeights(weights HealthWeights) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.weights = weights
}

// SetThresholds 设置阈值
func (hs *HealthScorer) SetThresholds(thresholds HealthThresholds) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.thresholds = thresholds
}

// CalculateScore 计算健康评分
func (hs *HealthScorer) CalculateScore() *HealthScore {
	hs.mu.RLock()
	weights := hs.weights
	thresholds := hs.thresholds
	hs.mu.RUnlock()

	score := &HealthScore{
		Timestamp:       time.Now(),
		Issues:          make([]HealthIssue, 0),
		Recommendations: make([]string, 0),
		Details:         make(map[string]float64),
	}

	// 获取系统统计
	stats, err := hs.manager.GetSystemStats()
	if err == nil {
		score.Components.CPU = hs.calculateCPUScore(stats, thresholds)
		score.Components.Memory = hs.calculateMemoryScore(stats, thresholds)
		score.Components.System = hs.calculateSystemScore(stats, thresholds)
		score.Details["cpu_usage"] = stats.CPUUsage
		score.Details["memory_usage"] = stats.MemoryUsage
		score.Details["load_1"] = stats.LoadAvg[0]
	}

	// 获取磁盘统计
	diskStats, err := hs.manager.GetDiskStats()
	if err == nil {
		score.Components.Disk = hs.calculateDiskScore(diskStats, thresholds)
	}

	// 获取网络统计
	netStats, err := hs.manager.GetNetworkStats()
	if err == nil {
		score.Components.Network = hs.calculateNetworkScore(netStats)
	}

	// 获取 SMART 信息
	smartInfos, err := hs.manager.CheckDisks()
	if err == nil {
		score.Components.SMART = hs.calculateSMARTScore(smartInfos, thresholds)
	}

	// 计算总分
	totalWeight := weights.CPU + weights.Memory + weights.Disk + weights.Network + weights.SMART + weights.Uptime
	score.TotalScore = (score.Components.CPU.Score*weights.CPU +
		score.Components.Memory.Score*weights.Memory +
		score.Components.Disk.Score*weights.Disk +
		score.Components.Network.Score*weights.Network +
		score.Components.SMART.Score*weights.SMART) / totalWeight

	// 生成等级
	score.Grade = hs.scoreToGrade(score.TotalScore)

	// 计算趋势
	hs.calculateTrend(score)

	// 生成建议
	hs.generateRecommendations(score)

	// 保存历史
	hs.mu.Lock()
	hs.history = append(hs.history, score)
	if len(hs.history) > hs.maxHistory {
		hs.history = hs.history[1:]
	}
	hs.lastScore = score
	hs.mu.Unlock()

	return score
}

// calculateCPUScore 计算 CPU 评分
func (hs *HealthScorer) calculateCPUScore(stats *SystemStats, thresholds HealthThresholds) ComponentHealth {
	health := ComponentHealth{
		Weight:    hs.weights.CPU,
		Threshold: thresholds.CPUWarning,
	}

	// 基于 CPU 使用率评分
	usage := stats.CPUUsage
	switch {
	case usage >= thresholds.CPUCritical:
		health.Score = 20
		health.Status = "critical"
		health.Message = "CPU 使用率过高"
		hs.addIssue(&health, "cpu", "critical", "CPU 使用率过高", usage, thresholds.CPUCritical)
	case usage >= thresholds.CPUWarning:
		health.Score = 60 + (thresholds.CPUCritical-usage)/(thresholds.CPUCritical-thresholds.CPUWarning)*20
		health.Status = "warning"
		health.Message = "CPU 使用率较高"
		hs.addIssue(&health, "cpu", "warning", "CPU 使用率较高", usage, thresholds.CPUWarning)
	default:
		health.Score = 100 - usage*0.3
		health.Status = "healthy"
		health.Message = "CPU 使用正常"
	}

	// 检查负载
	if len(stats.LoadAvg) > 0 {
		loadPerCore := stats.LoadAvg[0] // 简化，实际应除以核心数
		if loadPerCore > thresholds.LoadPerCore {
			health.Score -= 10
			hs.addIssue(&health, "cpu", "warning", "系统负载较高", loadPerCore, thresholds.LoadPerCore)
		}
	}

	return health
}

// calculateMemoryScore 计算内存评分
func (hs *HealthScorer) calculateMemoryScore(stats *SystemStats, thresholds HealthThresholds) ComponentHealth {
	health := ComponentHealth{
		Weight:    hs.weights.Memory,
		Threshold: thresholds.MemoryWarning,
	}

	usage := stats.MemoryUsage
	switch {
	case usage >= thresholds.MemoryCritical:
		health.Score = 20
		health.Status = "critical"
		health.Message = "内存使用率过高"
		hs.addIssue(&health, "memory", "critical", "内存使用率过高", usage, thresholds.MemoryCritical)
	case usage >= thresholds.MemoryWarning:
		health.Score = 60 + (thresholds.MemoryCritical-usage)/(thresholds.MemoryCritical-thresholds.MemoryWarning)*20
		health.Status = "warning"
		health.Message = "内存使用率较高"
		hs.addIssue(&health, "memory", "warning", "内存使用率较高", usage, thresholds.MemoryWarning)
	default:
		health.Score = 100 - usage*0.3
		health.Status = "healthy"
		health.Message = "内存使用正常"
	}

	// 检查 Swap 使用
	if stats.SwapUsage > thresholds.SwapUsage {
		health.Score -= 10
		hs.addIssue(&health, "memory", "warning", "Swap 使用率较高", stats.SwapUsage, thresholds.SwapUsage)
	}

	return health
}

// calculateDiskScore 计算磁盘评分
func (hs *HealthScorer) calculateDiskScore(disks []*DiskStats, thresholds HealthThresholds) ComponentHealth {
	health := ComponentHealth{
		Weight:    hs.weights.Disk,
		Threshold: thresholds.DiskWarning,
		Score:     100,
		Status:    "healthy",
		Message:   "磁盘状态正常",
	}

	if len(disks) == 0 {
		health.Message = "无法获取磁盘信息"
		health.Score = 80
		return health
	}

	criticalCount := 0
	warningCount := 0

	for _, disk := range disks {
		// 跳过伪文件系统
		if disk.FSType == "tmpfs" || disk.FSType == "devtmpfs" || disk.FSType == "overlay" {
			continue
		}

		switch {
		case disk.UsagePercent >= thresholds.DiskCritical:
			criticalCount++
			health.Score -= 15
			hs.addIssue(&health, "disk", "critical",
				"磁盘 "+disk.MountPoint+" 空间不足",
				disk.UsagePercent, thresholds.DiskCritical)
		case disk.UsagePercent >= thresholds.DiskWarning:
			warningCount++
			health.Score -= 5
			hs.addIssue(&health, "disk", "warning",
				"磁盘 "+disk.MountPoint+" 空间较紧张",
				disk.UsagePercent, thresholds.DiskWarning)
		}
	}

	if criticalCount > 0 {
		health.Status = "critical"
		health.Message = "存在磁盘空间严重不足"
	} else if warningCount > 0 {
		health.Status = "warning"
		health.Message = "部分磁盘空间较紧张"
	}

	if health.Score < 0 {
		health.Score = 10
	}

	return health
}

// calculateNetworkScore 计算网络评分
func (hs *HealthScorer) calculateNetworkScore(nets []*NetworkStats) ComponentHealth {
	health := ComponentHealth{
		Weight:    hs.weights.Network,
		Threshold: 0,
		Score:     100,
		Status:    "healthy",
		Message:   "网络状态正常",
	}

	if len(nets) == 0 {
		health.Message = "无网络接口"
		return health
	}

	// 检查错误率
	for _, net := range nets {
		totalPackets := net.RXPackets + net.TXPackets
		if totalPackets > 0 {
			errorRate := float64(net.RXErrors+net.TXErrors) / float64(totalPackets) * 100
			if errorRate > 1.0 {
				health.Score -= 20
				health.Status = "warning"
				health.Message = "网络存在错误"
				hs.addIssue(&health, "network", "warning",
					"网络接口 "+net.Interface+" 存在错误",
					errorRate, 1.0)
			}
		}
	}

	return health
}

// calculateSMARTScore 计算 SMART 评分
func (hs *HealthScorer) calculateSMARTScore(smarts []*SMARTInfo, thresholds HealthThresholds) ComponentHealth {
	health := ComponentHealth{
		Weight:    hs.weights.SMART,
		Threshold: float64(thresholds.MaxTemperature),
		Score:     100,
		Status:    "healthy",
		Message:   "磁盘 SMART 状态正常",
	}

	if len(smarts) == 0 {
		health.Message = "无 SMART 数据"
		return health
	}

	for _, smart := range smarts {
		// 检查健康状态
		if smart.Health != "PASSED" {
			health.Score = 20
			health.Status = "critical"
			health.Message = "磁盘 SMART 检测失败"
			hs.addIssue(&health, "smart", "critical",
				"磁盘 "+smart.Device+" SMART 状态异常",
				0, 0)
		}

		// 检查温度
		if smart.Temperature > thresholds.MaxTemperature {
			health.Score -= 10
			hs.addIssue(&health, "smart", "warning",
				"磁盘 "+smart.Device+" 温度过高",
				float64(smart.Temperature), float64(thresholds.MaxTemperature))
		}
	}

	return health
}

// calculateSystemScore 计算系统评分
func (hs *HealthScorer) calculateSystemScore(stats *SystemStats, thresholds HealthThresholds) ComponentHealth {
	health := ComponentHealth{
		Weight:  hs.weights.Uptime,
		Score:   100,
		Status:  "healthy",
		Message: "系统运行正常",
	}

	// 运行时间影响（刚启动时扣分）
	uptimeDays := float64(stats.UptimeSeconds) / 86400
	if uptimeDays < float64(thresholds.UptimeGraceDays) {
		health.Score -= 10
		health.Message = "系统刚启动"
	}

	return health
}

// addIssue 添加问题
func (hs *HealthScorer) addIssue(health *ComponentHealth, component, severity, message string, value, threshold float64) {
	health.Message = message
}

// calculateTrend 计算趋势
func (hs *HealthScorer) calculateTrend(score *HealthScore) {
	hs.mu.RLock()
	history := hs.history
	hs.mu.RUnlock()

	if len(history) == 0 {
		score.Trend = ScoreTrend{
			Direction: "stable",
			Change:    0,
		}
		return
	}

	// 获取一小时前的分数
	var prevHourScore, prevDayScore float64
	now := time.Now()

	for i := len(history) - 1; i >= 0; i-- {
		age := now.Sub(history[i].Timestamp)
		if age >= time.Hour && prevHourScore == 0 {
			prevHourScore = history[i].TotalScore
		}
		if age >= 24*time.Hour && prevDayScore == 0 {
			prevDayScore = history[i].TotalScore
			break
		}
	}

	if prevHourScore == 0 {
		prevHourScore = score.TotalScore
	}
	if prevDayScore == 0 {
		prevDayScore = score.TotalScore
	}

	change := score.TotalScore - prevHourScore

	var direction string
	if math.Abs(change) < 2 {
		direction = "stable"
	} else if change > 0 {
		direction = "up"
	} else {
		direction = "down"
	}

	score.Trend = ScoreTrend{
		Direction:    direction,
		Change:       change,
		PreviousHour: prevHourScore,
		PreviousDay:  prevDayScore,
	}
}

// generateRecommendations 生成建议
func (hs *HealthScorer) generateRecommendations(score *HealthScore) {
	// CPU 建议
	if score.Components.CPU.Score < 70 {
		score.Recommendations = append(score.Recommendations,
			"建议检查 CPU 占用进程，考虑优化或限制资源使用")
	}

	// 内存建议
	if score.Components.Memory.Score < 70 {
		score.Recommendations = append(score.Recommendations,
			"建议增加内存或优化内存使用，关闭不必要的服务")
	}

	// 磁盘建议
	if score.Components.Disk.Score < 70 {
		score.Recommendations = append(score.Recommendations,
			"建议清理磁盘空间或扩容存储")
	}

	// SMART 建议
	if score.Components.SMART.Score < 80 {
		score.Recommendations = append(score.Recommendations,
			"建议备份重要数据，检查磁盘健康状况")
	}

	// 总体建议
	if score.TotalScore < 60 {
		score.Recommendations = append(score.Recommendations,
			"系统健康状况较差，建议进行全面检查")
	}
}

// scoreToGrade 分数转等级
func (hs *HealthScorer) scoreToGrade(score float64) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

// GetHistory 获取历史记录
func (hs *HealthScorer) GetHistory(limit int) []*HealthScore {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	if limit <= 0 || limit > len(hs.history) {
		limit = len(hs.history)
	}

	start := len(hs.history) - limit
	result := make([]*HealthScore, limit)
	copy(result, hs.history[start:])

	return result
}

// GetLastScore 获取最近评分
func (hs *HealthScorer) GetLastScore() *HealthScore {
	hs.mu.RLock()
	defer hs.mu.RUnlock()
	return hs.lastScore
}

// GetScoreStats 获取评分统计
func (hs *HealthScorer) GetScoreStats(duration time.Duration) map[string]interface{} {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	now := time.Now()
	cutoff := now.Add(-duration)

	var scores []float64
	for _, h := range hs.history {
		if h.Timestamp.After(cutoff) {
			scores = append(scores, h.TotalScore)
		}
	}

	if len(scores) == 0 {
		return map[string]interface{}{
			"count": 0,
		}
	}

	var sum, min, max float64
	min = scores[0]
	max = scores[0]

	for _, s := range scores {
		sum += s
		if s < min {
			min = s
		}
		if s > max {
			max = s
		}
	}

	avg := sum / float64(len(scores))

	// 计算标准差
	var variance float64
	for _, s := range scores {
		variance += math.Pow(s-avg, 2)
	}
	stdDev := math.Sqrt(variance / float64(len(scores)))

	return map[string]interface{}{
		"count":      len(scores),
		"average":    avg,
		"min":        min,
		"max":        max,
		"std_dev":    stdDev,
		"trend":      hs.lastScore.Trend.Direction,
		"last_score": hs.lastScore.TotalScore,
	}
}
