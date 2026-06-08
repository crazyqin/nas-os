package securityaudit

import (
	"sync"
	"time"
)

// ScoreEngine 安全评分引擎.
type ScoreEngine struct {
	history []SecurityScoreHistory
	mu      sync.RWMutex
}

// NewScoreEngine 创建评分引擎.
func NewScoreEngine() *ScoreEngine {
	return &ScoreEngine{
		history: make([]SecurityScoreHistory, 0),
	}
}

// CalculateScore 计算安全评分.
func (e *ScoreEngine) CalculateScore(checkResults []SecurityCheckResult) SecurityScore {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 初始化各类别分数
	scores := map[SecurityCheckCategory]struct {
		passed int
		total  int
	}{
		CategoryAuth:       {0, 0},
		CategoryNetwork:    {0, 0},
		CategorySystem:     {0, 0},
		CategoryFile:       {0, 0},
		CategoryCrypto:     {0, 0},
		CategoryAccess:     {0, 0},
		CategoryPatch:      {0, 0},
		CategoryBackup:     {0, 0},
		CategoryContainer:  {0, 0},
		CategoryCompliance: {0, 0},
	}

	// 统计各类别通过情况
	for _, result := range checkResults {
		if s, ok := scores[result.Category]; ok {
			s.total++
			if result.Status == StatusPass {
				s.passed++
			}
			scores[result.Category] = s
		}
	}

	// 计算各类别分数（0-100）
	categoryScores := map[SecurityCheckCategory]int{}
	for category, s := range scores {
		if s.total > 0 {
			categoryScores[category] = (s.passed * 100) / s.total
		} else {
			categoryScores[category] = 100 // 没有检查项则满分
		}
	}

	// 计算加权总分
	// 权重分配：认证 20%，网络 15%，系统 15%，文件 10%，加密 10%，访问 10%，补丁 5%，备份 5%，容器 5%，合规 5%
	weights := map[SecurityCheckCategory]float64{
		CategoryAuth:       0.20,
		CategoryNetwork:    0.15,
		CategorySystem:     0.15,
		CategoryFile:       0.10,
		CategoryCrypto:     0.10,
		CategoryAccess:     0.10,
		CategoryPatch:      0.05,
		CategoryBackup:     0.05,
		CategoryContainer:  0.05,
		CategoryCompliance: 0.05,
	}

	totalScore := 0.0
	for category, score := range categoryScores {
		totalScore += float64(score) * weights[category]
	}

	overall := int(totalScore)

	// 确定等级
	grade := e.calculateGrade(overall)

	// 确定趋势
	trend := "stable"
	if len(e.history) > 0 {
		lastScore := e.history[len(e.history)-1].Score
		if overall > lastScore {
			trend = "up"
		} else if overall < lastScore {
			trend = "down"
		}
	}

	score := SecurityScore{
		Overall:      overall,
		Auth:         categoryScores[CategoryAuth],
		Network:      categoryScores[CategoryNetwork],
		System:       categoryScores[CategorySystem],
		File:         categoryScores[CategoryFile],
		Crypto:       categoryScores[CategoryCrypto],
		Access:       categoryScores[CategoryAccess],
		Patch:        categoryScores[CategoryPatch],
		Backup:       categoryScores[CategoryBackup],
		Container:    categoryScores[CategoryContainer],
		Compliance:   categoryScores[CategoryCompliance],
		Grade:        grade,
		Trend:        trend,
		CalculatedAt: time.Now(),
	}

	// 记录历史
	e.history = append(e.history, SecurityScoreHistory{
		Timestamp: time.Now(),
		Score:     overall,
		Grade:     grade,
	})

	// 保留最近 365 天的历史
	if len(e.history) > 365 {
		e.history = e.history[len(e.history)-365:]
	}

	return score
}

// calculateGrade 计算等级.
func (e *ScoreEngine) calculateGrade(score int) string {
	switch {
	case score >= 95:
		return "A+"
	case score >= 90:
		return "A"
	case score >= 85:
		return "B+"
	case score >= 80:
		return "B"
	case score >= 75:
		return "C+"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

// GetHistory 获取评分历史.
func (e *ScoreEngine) GetHistory(days int) []SecurityScoreHistory {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if days <= 0 {
		return e.history
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	result := make([]SecurityScoreHistory, 0)

	for _, h := range e.history {
		if h.Timestamp.After(cutoff) {
			result = append(result, h)
		}
	}

	return result
}

// GetScoreTrend 获取评分趋势.
func (e *ScoreEngine) GetScoreTrend(days int) map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	history := e.GetHistory(days)
	if len(history) < 2 {
		return map[string]interface{}{
			"trend":    "stable",
			"change":   0,
			"history":  history,
		}
	}

	first := history[0].Score
	last := history[len(history)-1].Score
	change := last - first

	trend := "stable"
	if change > 0 {
		trend = "up"
	} else if change < 0 {
		trend = "down"
	}

	return map[string]interface{}{
		"trend":   trend,
		"change":  change,
		"first":   first,
		"last":    last,
		"history": history,
	}
}

// GetCategoryBreakdown 获取各类别分数明细.
func (e *ScoreEngine) GetCategoryBreakdown(score SecurityScore) map[string]interface{} {
	return map[string]interface{}{
		"categories": []map[string]interface{}{
			{"name": "认证安全", "score": score.Auth, "weight": "20%"},
			{"name": "网络安全", "score": score.Network, "weight": "15%"},
			{"name": "系统安全", "score": score.System, "weight": "15%"},
			{"name": "文件安全", "score": score.File, "weight": "10%"},
			{"name": "加密安全", "score": score.Crypto, "weight": "10%"},
			{"name": "访问控制", "score": score.Access, "weight": "10%"},
			{"name": "补丁管理", "score": score.Patch, "weight": "5%"},
			{"name": "备份安全", "score": score.Backup, "weight": "5%"},
			{"name": "容器安全", "score": score.Container, "weight": "5%"},
			{"name": "合规检查", "score": score.Compliance, "weight": "5%"},
		},
		"overall": score.Overall,
		"grade":   score.Grade,
	}
}
