// Package storagehealthgrade - 存储健康评分管理器
package storagehealthgrade

import (
	"fmt"
	"sync"
	"time"
)

// Manager 健康评分管理器
type Manager struct {
	mu      sync.RWMutex
	current *HealthReport
	history []*TrendPoint
	alerts  []*HealthAlert
	stats   *HealthStats
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		history: make([]*TrendPoint, 0),
		alerts:  make([]*HealthAlert, 0),
		stats:   &HealthStats{},
	}
}

// RunAssessment 运行健康评估
func (m *Manager) RunAssessment() *HealthReport {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	dimensions := []*DimensionScore{
		{Type: DimStorage, Name: "存储使用率", Score: 72, Weight: 0.25, Grade: GradeC, UpdatedAt: now},
		{Type: DimRAID, Name: "RAID 状态", Score: 95, Weight: 0.2, Grade: GradeA, UpdatedAt: now},
		{Type: DimSMART, Name: "磁盘健康", Score: 88, Weight: 0.2, Grade: GradeB, UpdatedAt: now},
		{Type: DimPerf, Name: "性能指标", Score: 82, Weight: 0.15, Grade: GradeB, UpdatedAt: now},
		{Type: DimSecurity, Name: "安全状态", Score: 90, Weight: 0.1, Grade: GradeA, UpdatedAt: now},
		{Type: DimBackup, Name: "备份状态", Score: 65, Weight: 0.1, Grade: GradeC, UpdatedAt: now},
	}

	totalScore := 0.0
	totalWeight := 0.0
	for _, d := range dimensions {
		totalScore += d.Score * d.Weight
		totalWeight += d.Weight
	}
	overallScore := totalScore / totalWeight

	report := &HealthReport{
		ID:           fmt.Sprintf("report-%d", now.UnixNano()),
		OverallScore: overallScore,
		OverallGrade: m.scoreToGrade(overallScore),
		Dimensions:   dimensions,
		Trend:        m.calcTrend(overallScore),
		LastChecked:  now,
		NextCheck:    now.Add(24 * time.Hour),
	}

	m.current = report
	m.history = append(m.history, &TrendPoint{Timestamp: now, Score: overallScore, Grade: report.OverallGrade})

	// 更新统计
	m.stats.CurrentGrade = report.OverallGrade
	m.stats.CurrentScore = overallScore
	m.stats.Trend = report.Trend
	m.stats.TotalChecks++
	if overallScore > m.stats.BestScore || m.stats.BestScore == 0 {
		m.stats.BestScore = overallScore
	}
	if overallScore < m.stats.WorstScore || m.stats.WorstScore == 0 {
		m.stats.WorstScore = overallScore
	}

	// 生成告警
	for _, d := range dimensions {
		if d.Score < 60 {
			alert := &HealthAlert{
				ID:        fmt.Sprintf("alert-%d", now.UnixNano()),
				Dimension: d.Type,
				Level:     "warning",
				Message:   fmt.Sprintf("%s 评分较低: %.1f", d.Name, d.Score),
				Score:     d.Score,
				CreatedAt: now,
			}
			m.alerts = append(m.alerts, alert)
			m.stats.AlertCount++
			m.stats.UnresolvedAlerts++
		}
	}

	return report
}

// GetCurrent 获取当前报告
func (m *Manager) GetCurrent() *HealthReport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// GetHistory 获取历史趋势
func (m *Manager) GetHistory() []*TrendPoint {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.history
}

// GetAlerts 获取告警
func (m *Manager) GetAlerts() []*HealthAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alerts
}

// GetStats 获取统计
func (m *Manager) GetStats() *HealthStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

func (m *Manager) scoreToGrade(score float64) Grade {
	switch {
	case score >= 90:
		return GradeA
	case score >= 75:
		return GradeB
	case score >= 60:
		return GradeC
	case score >= 40:
		return GradeD
	default:
		return GradeF
	}
}

func (m *Manager) calcTrend(current float64) string {
	if len(m.history) < 2 {
		return "stable"
	}
	prev := m.history[len(m.history)-2].Score
	diff := current - prev
	switch {
	case diff > 5:
		return "improving"
	case diff < -5:
		return "declining"
	default:
		return "stable"
	}
}
