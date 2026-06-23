package diskhealth

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// PredictiveAnalyzer 预测分析器
type PredictiveAnalyzer struct {
	history map[string][]*HealthSnapshot
	mu      sync.RWMutex
}

// HealthSnapshot 健康快照
type HealthSnapshot struct {
	Timestamp   time.Time   `json:"timestamp"`
	Score       HealthScore `json:"score"`
	Temperature int         `json:"temperature"`
	Reallocated int64       `json:"reallocated"`
	Pending     int64       `json:"pending"`
	PowerOnHrs  int64       `json:"powerOnHrs"`
}

// TrendAnalysis 趋势分析
type TrendAnalysis struct {
	Device         string      `json:"device"`
	CurrentScore   HealthScore `json:"currentScore"`
	AvgScore       float64     `json:"avgScore"`
	TrendDirection string      `json:"trendDirection"` // up/down/stable
	TrendRate      float64     `json:"trendRate"`      // 每天变化率
	DaysToFailure  int         `json:"daysToFailure"`  // 预计故障天数
	Confidence     float64     `json:"confidence"`     // 置信度 0-1
}

// NewPredictiveAnalyzer 创建预测分析器
func NewPredictiveAnalyzer() *PredictiveAnalyzer {
	return &PredictiveAnalyzer{
		history: make(map[string][]*HealthSnapshot),
	}
}

// AddSnapshot 添加快照
func (a *PredictiveAnalyzer) AddSnapshot(device string, snapshot *HealthSnapshot) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, ok := a.history[device]; !ok {
		a.history[device] = make([]*HealthSnapshot, 0)
	}

	a.history[device] = append(a.history[device], snapshot)

	// 保留最近 90 天的数据
	cutoff := time.Now().AddDate(0, 0, -90)
	var filtered []*HealthSnapshot
	for _, s := range a.history[device] {
		if s.Timestamp.After(cutoff) {
			filtered = append(filtered, s)
		}
	}
	a.history[device] = filtered
}

// AnalyzeTrend 分析趋势
func (a *PredictiveAnalyzer) AnalyzeTrend(device string) (*TrendAnalysis, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	snapshots, ok := a.history[device]
	if !ok || len(snapshots) < 2 {
		return nil, fmt.Errorf("insufficient data for device %s", device)
	}

	analysis := &TrendAnalysis{
		Device:       device,
		CurrentScore: snapshots[len(snapshots)-1].Score,
	}

	// 计算平均分
	var totalScore float64
	for _, s := range snapshots {
		totalScore += float64(s.Score)
	}
	analysis.AvgScore = totalScore / float64(len(snapshots))

	// 线性回归分析趋势
	n := float64(len(snapshots))
	var sumX, sumY, sumXY, sumX2 float64
	baseTime := snapshots[0].Timestamp.Unix()

	for _, s := range snapshots {
		x := float64(s.Timestamp.Unix()-baseTime) / 86400 // 转换为天
		y := float64(s.Score)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// 斜率 = (n*sumXY - sumX*sumY) / (n*sumX2 - sumX^2)
	denominator := n*sumX2 - sumX*sumX
	if denominator != 0 {
		slope := (n*sumXY - sumX*sumY) / denominator
		analysis.TrendRate = slope

		if slope > 0.1 {
			analysis.TrendDirection = "up"
		} else if slope < -0.1 {
			analysis.TrendDirection = "down"
		} else {
			analysis.TrendDirection = "stable"
		}

		// 预测故障天数（分数降到 30 以下）
		if slope < 0 {
			currentScore := float64(analysis.CurrentScore)
			if currentScore > 30 {
				analysis.DaysToFailure = int((currentScore - 30) / (-slope))
			}
		} else {
			analysis.DaysToFailure = -1 // 趋势向好，无法预测
		}
	}

	// 计算置信度（基于数据点数量和分布）
	analysis.Confidence = math.Min(float64(len(snapshots))/30.0, 1.0) * 0.9

	return analysis, nil
}

// PredictFailure 预测故障概率
func (a *PredictiveAnalyzer) PredictFailure(device string, days int) (float64, error) {
	trend, err := a.AnalyzeTrend(device)
	if err != nil {
		return 0, err
	}

	// 基于趋势和当前状态计算故障概率
	baseProb := 0.0

	// 根据当前分数
	switch {
	case trend.CurrentScore < 30:
		baseProb = 0.6
	case trend.CurrentScore < 50:
		baseProb = 0.3
	case trend.CurrentScore < 70:
		baseProb = 0.1
	default:
		baseProb = 0.02
	}

	// 根据趋势调整
	if trend.TrendDirection == "down" {
		// 恶化趋势，增加概率
		declineRate := math.Abs(trend.TrendRate)
		baseProb += declineRate * float64(days) * 0.01
	} else if trend.TrendDirection == "up" {
		// 改善趋势，降低概率
		baseProb *= 0.7
	}

	// 限制在 0-1 范围
	if baseProb > 1.0 {
		baseProb = 1.0
	}
	if baseProb < 0 {
		baseProb = 0
	}

	return baseProb, nil
}

// GetHistory 获取历史记录
func (a *PredictiveAnalyzer) GetHistory(device string, days int) []*HealthSnapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()

	snapshots, ok := a.history[device]
	if !ok {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	var result []*HealthSnapshot
	for _, s := range snapshots {
		if s.Timestamp.After(cutoff) {
			result = append(result, s)
		}
	}
	return result
}
