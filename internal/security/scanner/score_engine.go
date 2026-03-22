package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ScoreEngine 安全评分引擎
type ScoreEngine struct {
	categories  []ScoreCategory
	history     []ScoreHistory
	config      ScoreEngineConfig
	mu          sync.RWMutex
	storagePath string
}

// ScoreEngineConfig 评分引擎配置
type ScoreEngineConfig struct {
	HistoryRetention int `json:"history_retention"` // 历史记录保留天数
	MaxHistoryCount  int `json:"max_history_count"` // 最大历史记录数
}

// DefaultScoreEngineConfig 默认评分引擎配置
func DefaultScoreEngineConfig() ScoreEngineConfig {
	return ScoreEngineConfig{
		HistoryRetention: 90,
		MaxHistoryCount:  365,
	}
}

// NewScoreEngine 创建安全评分引擎
func NewScoreEngine(config ScoreEngineConfig) *ScoreEngine {
	storagePath := "/var/lib/nas-os/security/scores"
	if err := os.MkdirAll(storagePath, 0750); err != nil {
		// 无法创建存储目录，使用临时目录
		storagePath = os.TempDir()
	}

	engine := &ScoreEngine{
		categories:  getDefaultScoreCategories(),
		history:     make([]ScoreHistory, 0),
		config:      config,
		storagePath: storagePath,
	}

	// 加载历史记录
	engine.loadHistory()

	return engine
}

// getDefaultScoreCategories 获取默认评分类别
func getDefaultScoreCategories() []ScoreCategory {
	return []ScoreCategory{
		{
			ID:          "file_permissions",
			Name:        "文件权限",
			Description: "文件系统权限配置安全性",
			Weight:      0.25,
			MaxScore:    100,
			Enabled:     true,
		},
		{
			ID:          "sensitive_data",
			Name:        "敏感数据",
			Description: "敏感数据保护状况",
			Weight:      0.20,
			MaxScore:    100,
			Enabled:     true,
		},
		{
			ID:          "vulnerability",
			Name:        "漏洞风险",
			Description: "已知漏洞风险状况",
			Weight:      0.25,
			MaxScore:    100,
			Enabled:     true,
		},
		{
			ID:          "configuration",
			Name:        "配置安全",
			Description: "系统配置安全状况",
			Weight:      0.15,
			MaxScore:    100,
			Enabled:     true,
		},
		{
			ID:          "malware",
			Name:        "恶意软件",
			Description: "恶意软件检测状况",
			Weight:      0.15,
			MaxScore:    100,
			Enabled:     true,
		},
	}
}

// ========== 评分计算 ==========

// CalculateScore 计算综合安全评分
func (se *ScoreEngine) CalculateScore(
	permissionResult *PermissionCheckResult,
	scanReport *FileScanReport,
	vulnResult *VulnerabilityScanResult,
) *SecurityScore {
	se.mu.Lock()
	defer se.mu.Unlock()

	now := time.Now()
	score := &SecurityScore{
		CalculatedAt:    now,
		CategoryScores:  make(map[string]int),
		CategoryWeights: make(map[string]float64),
		Findings:        make([]*ScoreFinding, 0),
	}

	// 计算各类别分数
	for _, cat := range se.categories {
		if !cat.Enabled {
			continue
		}

		var categoryScore int
		switch cat.ID {
		case "file_permissions":
			categoryScore = se.calculatePermissionScore(permissionResult)
		case "sensitive_data":
			categoryScore = se.calculateSensitiveDataScore(scanReport)
		case "vulnerability":
			categoryScore = se.calculateVulnerabilityScore(vulnResult)
		case "configuration":
			categoryScore = se.calculateConfigurationScore(scanReport)
		case "malware":
			categoryScore = se.calculateMalwareScore(scanReport)
		default:
			categoryScore = 100
		}

		score.CategoryScores[cat.ID] = categoryScore
		score.CategoryWeights[cat.ID] = cat.Weight
	}

	// 计算加权总分
	totalScore := 0.0
	for catID, catScore := range score.CategoryScores {
		weight := score.CategoryWeights[catID]
		totalScore += float64(catScore) * weight
	}

	score.OverallScore = int(totalScore)

	// 计算等级
	score.Grade = scoreToGrade(score.OverallScore)
	score.Level = scoreToLevel(score.OverallScore)

	// 计算趋势
	if len(se.history) > 0 {
		lastScore := se.history[len(se.history)-1].Score
		score.PreviousScore = &lastScore
		score.ChangeFromPrevious = score.OverallScore - lastScore

		if score.ChangeFromPrevious > 5 {
			score.Trend = "improving"
		} else if score.ChangeFromPrevious < -5 {
			score.Trend = "declining"
		} else {
			score.Trend = "stable"
		}
	} else {
		score.Trend = "stable"
	}

	// 收集发现
	score.Findings = se.collectFindings(permissionResult, scanReport, vulnResult)

	// 保存到历史
	se.addToHistory(score)

	return score
}

// calculatePermissionScore 计算权限评分
func (se *ScoreEngine) calculatePermissionScore(result *PermissionCheckResult) int {
	if result == nil || result.TotalChecked == 0 {
		return 100
	}

	score := 100

	// 根据问题扣分
	score -= result.CriticalIssues * 20
	score -= result.WarningIssues * 5

	// 根据问题比例扣分
	issueRatio := float64(result.IssuesFound) / float64(result.TotalChecked)
	score -= int(issueRatio * 30)

	if score < 0 {
		score = 0
	}

	return score
}

// calculateSensitiveDataScore 计算敏感数据评分
func (se *ScoreEngine) calculateSensitiveDataScore(report *FileScanReport) int {
	if report == nil {
		return 100
	}

	score := 100

	// 根据敏感数据发现扣分
	sensitiveCount := report.Summary.FindingsByType[string(FindingTypeSensitiveData)]
	score -= sensitiveCount * 10

	if score < 0 {
		score = 0
	}

	return score
}

// calculateVulnerabilityScore 计算漏洞评分
func (se *ScoreEngine) calculateVulnerabilityScore(result *VulnerabilityScanResult) int {
	if result == nil {
		return 100
	}

	score := 100

	// 根据漏洞数量和严重程度扣分
	score -= result.CriticalVulns * 25
	score -= result.HighVulns * 15
	score -= result.MediumVulns * 5
	score -= result.LowVulns * 2

	if score < 0 {
		score = 0
	}

	return score
}

// calculateConfigurationScore 计算配置评分
func (se *ScoreEngine) calculateConfigurationScore(report *FileScanReport) int {
	if report == nil {
		return 100
	}

	score := 100

	// 根据配置问题扣分
	configCount := report.Summary.FindingsByType[string(FindingTypeConfiguration)]
	score -= configCount * 8

	if score < 0 {
		score = 0
	}

	return score
}

// calculateMalwareScore 计算恶意软件评分
func (se *ScoreEngine) calculateMalwareScore(report *FileScanReport) int {
	if report == nil {
		return 100
	}

	score := 100

	// 根据恶意软件发现扣分
	malwareCount := report.Summary.FindingsByType[string(FindingTypeMalware)]
	score -= malwareCount * 30

	if score < 0 {
		score = 0
	}

	return score
}

// collectFindings 收集发现
func (se *ScoreEngine) collectFindings(
	permissionResult *PermissionCheckResult,
	scanReport *FileScanReport,
	vulnResult *VulnerabilityScanResult,
) []*ScoreFinding {
	findings := make([]*ScoreFinding, 0)

	// 权限问题
	if permissionResult != nil {
		for _, issue := range permissionResult.Issues {
			if issue.Severity == SeverityCritical || issue.Severity == SeverityHigh {
				findings = append(findings, &ScoreFinding{
					Category:    "file_permissions",
					ID:          issue.Path,
					Title:       issue.Issue,
					Description: issue.Risk,
					Severity:    issue.Severity,
					Impact:      severityToImpact(issue.Severity),
					Remediation: issue.RecommendedMode,
				})
			}
		}
	}

	// 扫描发现
	if scanReport != nil {
		for _, finding := range scanReport.Findings {
			if finding.Severity == SeverityCritical || finding.Severity == SeverityHigh {
				findings = append(findings, &ScoreFinding{
					Category:    string(finding.Type),
					ID:          finding.ID,
					Title:       finding.Description,
					Description: finding.FilePath,
					Severity:    finding.Severity,
					Impact:      severityToImpact(finding.Severity),
					Remediation: finding.Remediation,
				})
			}
		}
	}

	// 漏洞发现
	if vulnResult != nil {
		for _, vuln := range vulnResult.Vulnerabilities {
			if vuln.Severity == SeverityCritical || vuln.Severity == SeverityHigh {
				findings = append(findings, &ScoreFinding{
					Category:    "vulnerability",
					ID:          vuln.CVE,
					Title:       vuln.Name,
					Description: vuln.Description,
					Severity:    vuln.Severity,
					Impact:      int(vuln.CVSSScore * 5),
					Remediation: vuln.FixedVersion,
				})
			}
		}
	}

	// 按严重程度排序
	sort.Slice(findings, func(i, j int) bool {
		severityOrder := map[Severity]int{
			SeverityCritical: 0,
			SeverityHigh:     1,
			SeverityMedium:   2,
			SeverityLow:      3,
		}
		return severityOrder[findings[i].Severity] < severityOrder[findings[j].Severity]
	})

	// 限制数量
	if len(findings) > 50 {
		findings = findings[:50]
	}

	return findings
}

// ========== 类别管理 ==========

// AddCategory 添加评分类别
func (se *ScoreEngine) AddCategory(category ScoreCategory) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.categories = append(se.categories, category)
}

// UpdateCategory 更新评分类别
func (se *ScoreEngine) UpdateCategory(category ScoreCategory) {
	se.mu.Lock()
	defer se.mu.Unlock()

	for i, cat := range se.categories {
		if cat.ID == category.ID {
			se.categories[i] = category
			break
		}
	}
}

// RemoveCategory 移除评分类别
func (se *ScoreEngine) RemoveCategory(categoryID string) {
	se.mu.Lock()
	defer se.mu.Unlock()

	for i, cat := range se.categories {
		if cat.ID == categoryID {
			se.categories = append(se.categories[:i], se.categories[i+1:]...)
			break
		}
	}
}

// GetCategories 获取所有评分类别
func (se *ScoreEngine) GetCategories() []ScoreCategory {
	se.mu.RLock()
	defer se.mu.RUnlock()

	categories := make([]ScoreCategory, len(se.categories))
	copy(categories, se.categories)
	return categories
}

// ========== 历史记录 ==========

// addToHistory 添加到历史记录
func (se *ScoreEngine) addToHistory(score *SecurityScore) {
	historyEntry := ScoreHistory{
		Date:     score.CalculatedAt,
		Score:    score.OverallScore,
		Grade:    score.Grade,
		Findings: len(score.Findings),
	}

	se.history = append(se.history, historyEntry)

	// 限制历史记录数量
	if len(se.history) > se.config.MaxHistoryCount {
		se.history = se.history[len(se.history)-se.config.MaxHistoryCount:]
	}

	// 保存到文件
	se.saveHistory()
}

// GetHistory 获取历史记录
func (se *ScoreEngine) GetHistory(limit int) []ScoreHistory {
	se.mu.RLock()
	defer se.mu.RUnlock()

	if limit <= 0 || limit > len(se.history) {
		limit = len(se.history)
	}

	start := len(se.history) - limit
	if start < 0 {
		start = 0
	}

	history := make([]ScoreHistory, limit)
	copy(history, se.history[start:])

	return history
}

// GetHistoryByDateRange 获取日期范围内的历史记录
func (se *ScoreEngine) GetHistoryByDateRange(start, end time.Time) []ScoreHistory {
	se.mu.RLock()
	defer se.mu.RUnlock()

	history := make([]ScoreHistory, 0)
	for _, h := range se.history {
		if (h.Date.Equal(start) || h.Date.After(start)) &&
			(h.Date.Equal(end) || h.Date.Before(end)) {
			history = append(history, h)
		}
	}

	return history
}

// ========== 趋势分析 ==========

// GetTrendAnalysis 获取趋势分析
func (se *ScoreEngine) GetTrendAnalysis(days int) map[string]interface{} {
	se.mu.RLock()
	defer se.mu.RUnlock()

	if len(se.history) == 0 {
		return map[string]interface{}{
			"trend":  "unknown",
			"change": 0,
		}
	}

	// 获取指定天数的历史
	startIdx := len(se.history) - days
	if startIdx < 0 {
		startIdx = 0
	}

	recentHistory := se.history[startIdx:]

	if len(recentHistory) < 2 {
		return map[string]interface{}{
			"trend":   "stable",
			"change":  0,
			"average": recentHistory[0].Score,
		}
	}

	// 计算平均分
	totalScore := 0
	for _, h := range recentHistory {
		totalScore += h.Score
	}
	avgScore := totalScore / len(recentHistory)

	// 计算变化
	firstScore := recentHistory[0].Score
	lastScore := recentHistory[len(recentHistory)-1].Score
	change := lastScore - firstScore

	// 确定趋势
	trend := "stable"
	if change > 10 {
		trend = "improving"
	} else if change < -10 {
		trend = "declining"
	}

	// 计算趋势线斜率
	var sumX, sumY, sumXY, sumX2 float64
	n := float64(len(recentHistory))
	for i, h := range recentHistory {
		x := float64(i)
		y := float64(h.Score)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)

	return map[string]interface{}{
		"trend":       trend,
		"change":      change,
		"average":     avgScore,
		"slope":       slope,
		"first_score": firstScore,
		"last_score":  lastScore,
		"data_points": len(recentHistory),
	}
}

// ========== 报告生成 ==========

// GenerateScoreReport 生成评分报告
func (se *ScoreEngine) GenerateScoreReport(score *SecurityScore) map[string]interface{} {
	report := map[string]interface{}{
		"overall_score": score.OverallScore,
		"grade":         score.Grade,
		"level":         score.Level,
		"calculated_at": score.CalculatedAt,
		"trend":         score.Trend,
		"categories":    make([]map[string]interface{}, 0),
		"findings":      score.Findings,
	}

	// 添加类别详情
	for _, cat := range se.categories {
		catScore := score.CategoryScores[cat.ID]
		catWeight := score.CategoryWeights[cat.ID]

		categories, ok := report["categories"].([]map[string]interface{})
		if !ok {
			categories = make([]map[string]interface{}, 0)
		}
		categories = append(categories, map[string]interface{}{
			"id":          cat.ID,
			"name":        cat.Name,
			"description": cat.Description,
			"score":       catScore,
			"weight":      catWeight,
			"grade":       scoreToGrade(catScore),
			"status":      getCategoryStatus(catScore),
		})
		report["categories"] = categories
	}

	// 添加趋势信息
	if score.PreviousScore != nil {
		report["previous_score"] = *score.PreviousScore
		report["change"] = score.ChangeFromPrevious
	}

	// 添加建议
	report["recommendations"] = se.generateRecommendations(score)

	return report
}

// generateRecommendations 生成建议
func (se *ScoreEngine) generateRecommendations(score *SecurityScore) []string {
	recommendations := make([]string, 0)

	// 根据各类别分数给出建议
	for catID, catScore := range score.CategoryScores {
		if catScore < 60 {
			switch catID {
			case "file_permissions":
				recommendations = append(recommendations, "审查并修复文件权限问题")
			case "sensitive_data":
				recommendations = append(recommendations, "检查并移除敏感数据泄露")
			case "vulnerability":
				recommendations = append(recommendations, "更新软件以修复已知漏洞")
			case "configuration":
				recommendations = append(recommendations, "审查并加固系统配置")
			case "malware":
				recommendations = append(recommendations, "清理恶意软件并加强防护")
			}
		}
	}

	// 根据发现给出建议
	criticalFindings := 0
	for _, finding := range score.Findings {
		if finding.Severity == SeverityCritical {
			criticalFindings++
		}
	}

	if criticalFindings > 0 {
		recommendations = append(recommendations, "立即处理严重安全问题")
	}

	return recommendations
}

// ========== 持久化 ==========

// saveHistory 保存历史记录
func (se *ScoreEngine) saveHistory() {
	filename := filepath.Join(se.storagePath, "history.json")

	data, err := json.MarshalIndent(se.history, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(filename, data, 0600)
}

// loadHistory 加载历史记录
func (se *ScoreEngine) loadHistory() {
	filename := filepath.Join(se.storagePath, "history.json")

	data, err := os.ReadFile(filename)
	if err != nil {
		return
	}

	var history []ScoreHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return
	}

	se.history = history
}

// ========== 辅助函数 ==========

func scoreToGrade(score int) string {
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

func scoreToLevel(score int) string {
	switch {
	case score >= 90:
		return "excellent"
	case score >= 75:
		return "good"
	case score >= 60:
		return "fair"
	case score >= 40:
		return "poor"
	default:
		return "critical"
	}
}

func severityToImpact(severity Severity) int {
	switch severity {
	case SeverityCritical:
		return 25
	case SeverityHigh:
		return 15
	case SeverityMedium:
		return 5
	case SeverityLow:
		return 2
	default:
		return 1
	}
}

func getCategoryStatus(score int) string {
	switch {
	case score >= 80:
		return "healthy"
	case score >= 60:
		return "warning"
	default:
		return "critical"
	}
}
