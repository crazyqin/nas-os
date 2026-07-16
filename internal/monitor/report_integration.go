package monitor

import (
	"time"
)

// ReportIntegration 监控报告集成服务（不依赖 Lab 包，保持 monitor 在生产依赖图内纯净）.
type ReportIntegration struct {
	manager   *Manager
	scorer    *HealthScorer
	collector *MetricsCollector
}

// NewReportIntegration 创建报告集成服务.
func NewReportIntegration(manager *Manager) *ReportIntegration {
	scorer := NewHealthScorer(manager)
	collector := NewMetricsCollector(manager, scorer)

	return &ReportIntegration{
		manager:   manager,
		scorer:    scorer,
		collector: collector,
	}
}

// GetHealthScorer 获取健康评分器.
func (ri *ReportIntegration) GetHealthScorer() *HealthScorer {
	return ri.scorer
}

// GetMetricsCollector 获取指标收集器.
func (ri *ReportIntegration) GetMetricsCollector() *MetricsCollector {
	return ri.collector
}

// Start 启动监控和收集.
func (ri *ReportIntegration) Start() {
	ri.collector.Start()
}

// Stop 停止监控和收集.
func (ri *ReportIntegration) Stop() {
	ri.collector.Stop()
}

// GetQuickHealthReport 获取快速健康报告（map 形态，不依赖 lab/reports 类型）.
func (ri *ReportIntegration) GetQuickHealthReport() map[string]interface{} {
	score := ri.scorer.CalculateScore()
	stats, _ := ri.manager.GetSystemStats()

	report := map[string]interface{}{
		"timestamp": time.Now(),
	}

	if score != nil {
		report["health_score"] = score.TotalScore
		report["health_grade"] = score.Grade
		report["trend"] = score.Trend.Direction
		report["recommendations"] = score.Recommendations
	}

	if stats != nil {
		report["cpu_usage"] = stats.CPUUsage
		report["memory_usage"] = stats.MemoryUsage
		report["uptime"] = stats.Uptime
	}

	return report
}

// CreateMonitorSnapshot builds a lab-free system snapshot for optional Lab adapters.
// Callers that need lab/reports types should live under internal/lab, not this package.
func (ri *ReportIntegration) CreateMonitorSnapshot() map[string]interface{} {
	out := map[string]interface{}{}
	if stats, err := ri.manager.GetSystemStats(); err == nil && stats != nil {
		out["system"] = map[string]interface{}{
			"cpu_usage":    stats.CPUUsage,
			"memory_usage": stats.MemoryUsage,
			"memory_total": stats.MemoryTotal,
			"uptime":       stats.Uptime,
			"load_avg":     stats.LoadAvg,
		}
	}
	if score := ri.scorer.CalculateScore(); score != nil {
		out["health_score"] = score.TotalScore
		out["health_grade"] = score.Grade
	}
	return out
}
