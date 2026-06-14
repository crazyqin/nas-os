// Package healthpredictor 健康报告生成器
package healthpredictor

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// Reporter 健康报告生成器
type Reporter struct {
	mu       sync.RWMutex
	reports  []*HealthReport
	maxReports int
}

// NewReporter 创建报告生成器
func NewReporter(maxReports int) *Reporter {
	return &Reporter{
		reports:    make([]*HealthReport, 0, maxReports),
		maxReports: maxReports,
	}
}

// Generate 生成健康报告
func (r *Reporter) Generate(
	metrics *SystemMetrics,
	anomalies []AnomalyDetectionResult,
	predictions []Prediction,
	heals []HealAction,
) *HealthReport {
	report := &HealthReport{
		ID:        fmt.Sprintf("report-%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		Metrics:   metrics,
		Anomalies: anomalies,
		Predictions: predictions,
		ActiveHeals: heals,
	}

	// 计算健康评分
	report.Score = r.calcHealthScore(metrics, anomalies, predictions)

	// 确定健康等级
	report.OverallHealth = r.scoreToLevel(report.Score)

	// 生成建议
	report.Recommendations = r.generateRecommendations(metrics, anomalies, predictions)

	// 生成摘要
	report.Summary = r.generateSummary(report)

	// 存储报告
	r.mu.Lock()
	r.reports = append(r.reports, report)
	if len(r.reports) > r.maxReports {
		r.reports = r.reports[len(r.reports)-r.maxReports:]
	}
	r.mu.Unlock()

	return report
}

// calcHealthScore 计算健康评分 (0-100)
func (r *Reporter) calcHealthScore(
	metrics *SystemMetrics,
	anomalies []AnomalyDetectionResult,
	predictions []Prediction,
) float64 {
	score := 100.0

	// 基于使用率扣分
	score -= r.metricPenalty(metrics.CPUUsage, 70, 90, 20)
	score -= r.metricPenalty(metrics.MemoryUsage, 80, 95, 20)
	score -= r.metricPenalty(metrics.DiskUsage, 80, 95, 20)
	score -= r.metricPenalty(metrics.DiskTemp, 45, 55, 10)

	// 基于异常扣分
	for _, a := range anomalies {
		switch a.Level {
		case AnomalyCritical:
			score -= 15
		case AnomalyWarning:
			score -= 5
		}
	}

	// 基于预测扣分
	for _, p := range predictions {
		switch p.Severity {
		case HealthCritical:
			score -= 20
		case HealthPoor:
			score -= 10
		case HealthFair:
			score -= 5
		}
	}

	// 限制范围
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return math.Round(score*10) / 10
}

// metricPenalty 计算指标惩罚分
func (r *Reporter) metricPenalty(value, warnThreshold, critThreshold, maxPenalty float64) float64 {
	if value >= critThreshold {
		return maxPenalty
	}
	if value >= warnThreshold {
		ratio := (value - warnThreshold) / (critThreshold - warnThreshold)
		return maxPenalty * ratio
	}
	return 0
}

// scoreToLevel 评分转等级
func (r *Reporter) scoreToLevel(score float64) HealthLevel {
	switch {
	case score >= 90:
		return HealthExcellent
	case score >= 75:
		return HealthGood
	case score >= 60:
		return HealthFair
	case score >= 40:
		return HealthPoor
	default:
		return HealthCritical
	}
}

// generateRecommendations 生成优化建议
func (r *Reporter) generateRecommendations(
	metrics *SystemMetrics,
	anomalies []AnomalyDetectionResult,
	predictions []Prediction,
) []Recommendation {
	var recs []Recommendation
	recID := 0

	// CPU 相关建议
	if metrics.CPUUsage > 80 {
		recID++
		recs = append(recs, Recommendation{
			ID:          fmt.Sprintf("rec-%d", recID),
			Category:    "cpu",
			Title:       "CPU 使用率偏高",
			Description: fmt.Sprintf("当前 CPU 使用率 %.1f%%，建议检查高占用进程或考虑扩容", metrics.CPUUsage),
			Priority:    HealthFair,
			MetricType:  MetricCPUUsage,
		})
	}

	// 内存相关建议
	if metrics.MemoryUsage > 85 {
		recID++
		recs = append(recs, Recommendation{
			ID:          fmt.Sprintf("rec-%d", recID),
			Category:    "memory",
			Title:       "内存使用率偏高",
			Description: fmt.Sprintf("当前内存使用率 %.1f%%，建议检查内存泄漏或增加内存", metrics.MemoryUsage),
			Priority:    HealthFair,
			MetricType:  MetricMemoryUsage,
		})
	}

	// 磁盘相关建议
	if metrics.DiskUsage > 80 {
		recID++
		recs = append(recs, Recommendation{
			ID:          fmt.Sprintf("rec-%d", recID),
			Category:    "disk",
			Title:       "磁盘空间紧张",
			Description: fmt.Sprintf("当前磁盘使用率 %.1f%%，建议清理日志和临时文件", metrics.DiskUsage),
			Priority:    HealthFair,
			MetricType:  MetricDiskUsage,
		})
	}

	// 磁盘温度建议
	if metrics.DiskTemp > 45 {
		recID++
		recs = append(recs, Recommendation{
			ID:          fmt.Sprintf("rec-%d", recID),
			Category:    "thermal",
			Title:       "磁盘温度偏高",
			Description: fmt.Sprintf("当前磁盘温度 %.1f°C，建议检查散热系统", metrics.DiskTemp),
			Priority:    HealthFair,
			MetricType:  MetricDiskTemp,
		})
	}

	// 基于异常的建议
	for _, a := range anomalies {
		if a.Level == AnomalyCritical {
			recID++
			recs = append(recs, Recommendation{
				ID:          fmt.Sprintf("rec-%d", recID),
				Category:    "anomaly",
				Title:       fmt.Sprintf("%s 异常", metricDisplayName(a.MetricType)),
				Description: a.Description,
				Priority:    HealthPoor,
				MetricType:  a.MetricType,
			})
		}
	}

	// 基于预测的建议
	for _, p := range predictions {
		recID++
		recs = append(recs, Recommendation{
			ID:          fmt.Sprintf("rec-%d", recID),
			Category:    "prediction",
			Title:       p.Description,
			Description: fmt.Sprintf("概率 %.0f%%, 预计 %.0f 分钟后发生", p.Probability*100, p.TimeToImpact.Minutes()),
			Priority:    p.Severity,
			MetricType:  p.MetricType,
		})
	}

	return recs
}

// generateSummary 生成文本摘要
func (r *Reporter) generateSummary(report *HealthReport) string {
	healthEmoji := map[HealthLevel]string{
		HealthExcellent: "🟢",
		HealthGood:      "🔵",
		HealthFair:      "🟡",
		HealthPoor:      "🟠",
		HealthCritical:  "🔴",
	}

	emoji := healthEmoji[report.OverallHealth]
	if emoji == "" {
		emoji = "⚪"
	}

	summary := fmt.Sprintf("%s 系统健康状态: %s (评分: %.1f/100)\n",
		emoji, report.OverallHealth, report.Score)

	// 关键指标
	summary += fmt.Sprintf("• CPU: %.1f%% | 内存: %.1f%% | 磁盘: %.1f%% | 温度: %.1f°C\n",
		report.Metrics.CPUUsage, report.Metrics.MemoryUsage,
		report.Metrics.DiskUsage, report.Metrics.DiskTemp)

	// 异常数
	if len(report.Anomalies) > 0 {
		summary += fmt.Sprintf("• 检测到 %d 个异常\n", len(report.Anomalies))
	}

	// 预测数
	if len(report.Predictions) > 0 {
		summary += fmt.Sprintf("• %d 个故障预测\n", len(report.Predictions))
	}

	// 修复状态
	if len(report.ActiveHeals) > 0 {
		success := 0
		failed := 0
		for _, h := range report.ActiveHeals {
			switch h.Status {
			case HealSuccess:
				success++
			case HealFailed:
				failed++
			}
		}
		if success > 0 || failed > 0 {
			summary += fmt.Sprintf("• 自动修复: %d 成功, %d 失败\n", success, failed)
		}
	}

	// 建议
	if len(report.Recommendations) > 0 {
		summary += fmt.Sprintf("• %d 条优化建议\n", len(report.Recommendations))
	}

	return summary
}

// GetLatestReport 获取最新报告
func (r *Reporter) GetLatestReport() *HealthReport {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.reports) == 0 {
		return nil
	}
	return r.reports[len(r.reports)-1]
}

// GetReports 获取报告列表
func (r *Reporter) GetReports(limit int) []*HealthReport {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 || limit > len(r.reports) {
		limit = len(r.reports)
	}

	result := make([]*HealthReport, limit)
	copy(result, r.reports[len(r.reports)-limit:])
	return result
}

// GetReport 获取指定报告
func (r *Reporter) GetReport(id string) *HealthReport {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, report := range r.reports {
		if report.ID == id {
			return report
		}
	}
	return nil
}
