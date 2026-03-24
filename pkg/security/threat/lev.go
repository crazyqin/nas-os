// Package threat - LEV (Likelihood of Exploitation Value) 威胁优先级计算
// 群晖DSM 7.3风格的漏洞利用可能性评估系统
package threat

import (
	"fmt"
	"math"
	"time"
)

// ========== LEV 威胁优先级系统 ==========

// LEVScore LEV 综合评分
type LEVScore struct {
	CVEID             string    `json:"cve_id"`
	CVSSScore         float64   `json:"cvss_score"`          // CVSS 基础评分 0-10
	EPSSScore         float64   `json:"epss_score"`          // EPSS 概率评分 0-1
	EPSSPercentile    float64   `json:"epss_percentile"`     // EPSS 百分位 0-1
	IsInKEV           bool      `json:"is_in_kev"`           // 是否在 KEV 目录中
	KEVDueDate        *time.Time `json:"kev_due_date"`       // KEV 修复期限
	IsRansomware      bool      `json:"is_ransomware"`       // 是否与勒索软件相关
	VulnerabilityAge  int       `json:"vulnerability_age"`   // 漏洞年龄（天）
	AssetCriticality  float64   `json:"asset_criticality"`   // 资产关键性 0-1
	ExposureLevel     float64   `json:"exposure_level"`      // 暴露程度 0-1
	NetworkAccessible bool      `json:"network_accessible"`  // 是否网络可访问

	// 计算得出的值
	LEVScore          float64   `json:"lev_score"`           // LEV 综合评分 0-100
	Priority          int       `json:"priority"`            // 优先级 1-4 (1最高)
	RiskLevel         string    `json:"risk_level"`          // critical/high/medium/low
	ExploitLikelihood string    `json:"exploit_likelihood"`  // known/likely/potential/unlikely
	RemediationUrgency string   `json:"remediation_urgency"` // immediate/soon/scheduled/deferred
	DueDate           *time.Time `json:"due_date"`           // 建议修复期限

	// 评分因子贡献
	FactorContributions FactorContributions `json:"factor_contributions"`
}

// FactorContributions 各因子贡献值
type FactorContributions struct {
	CVSSContribution      float64 `json:"cvss_contribution"`
	EPSSContribution      float64 `json:"epss_contribution"`
	KEVContribution       float64 `json:"kev_contribution"`
	AgeContribution       float64 `json:"age_contribution"`
	AssetContribution     float64 `json:"asset_contribution"`
	ExposureContribution  float64 `json:"exposure_contribution"`
	RansomwareContribution float64 `json:"ransomware_contribution"`
}

// LEVCalculator LEV 计算器
type LEVCalculator struct {
	config LEVConfig
}

// LEVConfig LEV 配置
type LEVConfig struct {
	// 各因子权重
	CVSSWeight       float64 `json:"cvss_weight"`
	EPSSWeight       float64 `json:"epss_weight"`
	KEVWeight        float64 `json:"kev_weight"`
	AgeWeight        float64 `json:"age_weight"`
	AssetWeight      float64 `json:"asset_weight"`
	ExposureWeight   float64 `json:"exposure_weight"`
	RansomwareWeight float64 `json:"ransomware_weight"`

	// 阈值配置
	EPSSHighThreshold     float64 `json:"epss_high_threshold"`     // EPSS 高风险阈值
	EPSSMediumThreshold   float64 `json:"epss_medium_threshold"`   // EPSS 中风险阈值
	PercentileHighThreshold float64 `json:"percentile_high_threshold"` // 百分位高阈值
	CriticalScoreThreshold float64 `json:"critical_score_threshold"`  // 严重评分阈值
	HighScoreThreshold    float64 `json:"high_score_threshold"`     // 高评分阈值
}

// DefaultLEVConfig 默认 LEV 配置
// 参考群晖DSM 7.3的权重设置
func DefaultLEVConfig() LEVConfig {
	return LEVConfig{
		CVSSWeight:           0.20,
		EPSSWeight:           0.20,
		KEVWeight:            0.25,
		AgeWeight:            0.10,
		AssetWeight:          0.10,
		ExposureWeight:       0.10,
		RansomwareWeight:     0.05,

		EPSSHighThreshold:      0.1,
		EPSSMediumThreshold:    0.01,
		PercentileHighThreshold: 0.9,
		CriticalScoreThreshold:  80.0,
		HighScoreThreshold:     60.0,
	}
}

// NewLEVCalculator 创建 LEV 计算器
func NewLEVCalculator(config LEVConfig) *LEVCalculator {
	return &LEVCalculator{config: config}
}

// Calculate 计算 LEV 评分
func (lc *LEVCalculator) Calculate(input LEVInput) *LEVScore {
	score := &LEVScore{
		CVEID:             input.CVEID,
		CVSSScore:         input.CVSSScore,
		EPSSScore:         input.EPSSScore,
		EPSSPercentile:    input.EPSSPercentile,
		IsInKEV:           input.IsInKEV,
		KEVDueDate:        input.KEVDueDate,
		IsRansomware:      input.IsRansomware,
		VulnerabilityAge:  input.VulnerabilityAge,
		AssetCriticality:  input.AssetCriticality,
		ExposureLevel:     input.ExposureLevel,
		NetworkAccessible: input.NetworkAccessible,
	}

	// 计算各因子贡献
	lc.calculateContributions(score)

	// 计算综合 LEV 评分
	lc.calculateLEVScore(score)

	// 确定优先级
	lc.determinePriority(score)

	// 确定风险等级
	lc.determineRiskLevel(score)

	// 确定利用可能性
	lc.determineExploitLikelihood(score)

	// 确定修复紧急程度
	lc.determineRemediationUrgency(score)

	// 计算建议修复期限
	lc.calculateDueDate(score)

	return score
}

// LEVInput LEV 计算输入
type LEVInput struct {
	CVEID             string     `json:"cve_id"`
	CVSSScore         float64    `json:"cvss_score"`
	EPSSScore         float64    `json:"epss_score"`
	EPSSPercentile    float64    `json:"epss_percentile"`
	IsInKEV           bool       `json:"is_in_kev"`
	KEVDueDate        *time.Time `json:"kev_due_date"`
	IsRansomware      bool       `json:"is_ransomware"`
	VulnerabilityAge  int        `json:"vulnerability_age"`  // 天
	AssetCriticality  float64    `json:"asset_criticality"`  // 0-1
	ExposureLevel     float64    `json:"exposure_level"`     // 0-1
	NetworkAccessible bool       `json:"network_accessible"`
}

// calculateContributions 计算各因子贡献
func (lc *LEVCalculator) calculateContributions(score *LEVScore) {
	cfg := lc.config

	// 1. CVSS 贡献 (0-10 -> 0-100)
	cvssNormalized := score.CVSSScore * 10
	score.FactorContributions.CVSSContribution = cvssNormalized * cfg.CVSSWeight

	// 2. EPSS 贡献 (0-1 -> 0-100)
	epssNormalized := score.EPSSScore * 100
	score.FactorContributions.EPSSContribution = epssNormalized * cfg.EPSSWeight

	// 3. KEV 贡献 (在 KEV 目录中 = 100)
	kevValue := 0.0
	if score.IsInKEV {
		kevValue = 100.0
	}
	score.FactorContributions.KEVContribution = kevValue * cfg.KEVWeight

	// 4. 漏洞年龄贡献 (新漏洞风险更高)
	ageFactor := lc.calculateAgeFactor(score.VulnerabilityAge)
	score.FactorContributions.AgeContribution = ageFactor * 100 * cfg.AgeWeight

	// 5. 资产关键性贡献
	score.FactorContributions.AssetContribution = score.AssetCriticality * 100 * cfg.AssetWeight

	// 6. 暴露程度贡献
	exposureFactor := score.ExposureLevel
	if score.NetworkAccessible {
		exposureFactor = math.Max(exposureFactor, 0.8) // 网络可访问最低 0.8
	}
	score.FactorContributions.ExposureContribution = exposureFactor * 100 * cfg.ExposureWeight

	// 7. 勒索软件关联贡献
	ransomwareValue := 0.0
	if score.IsRansomware {
		ransomwareValue = 100.0
	}
	score.FactorContributions.RansomwareContribution = ransomwareValue * cfg.RansomwareWeight
}

// calculateAgeFactor 计算年龄因子
func (lc *LEVCalculator) calculateAgeFactor(ageDays int) float64 {
	switch {
	case ageDays < 7:
		return 1.0 // 一周内，最高风险
	case ageDays < 30:
		return 0.9
	case ageDays < 90:
		return 0.7
	case ageDays < 365:
		return 0.5
	default:
		return 0.3
	}
}

// calculateLEVScore 计算 LEV 综合评分
func (lc *LEVCalculator) calculateLEVScore(score *LEVScore) {
	total := score.FactorContributions.CVSSContribution +
		score.FactorContributions.EPSSContribution +
		score.FactorContributions.KEVContribution +
		score.FactorContributions.AgeContribution +
		score.FactorContributions.AssetContribution +
		score.FactorContributions.ExposureContribution +
		score.FactorContributions.RansomwareContribution

	// 应用 KEV 加成 (群晖DSM 7.3特性：KEV 漏洞额外提升优先级)
	if score.IsInKEV {
		total = total * 1.2 // KEV 漏洞评分提升 20%
	}

	// 应用勒索软件加成
	if score.IsRansomware {
		total = total * 1.1 // 勒索软件相关再提升 10%
	}

	// 限制在 0-100 范围
	if total > 100 {
		total = 100
	}
	if total < 0 {
		total = 0
	}

	score.LEVScore = math.Round(total*100) / 100
}

// determinePriority 确定优先级
func (lc *LEVCalculator) determinePriority(score *LEVScore) {
	switch {
	case score.LEVScore >= 80:
		score.Priority = 1 // 最高优先级
	case score.LEVScore >= 60:
		score.Priority = 2
	case score.LEVScore >= 40:
		score.Priority = 3
	default:
		score.Priority = 4
	}

	// KEV 漏洞优先级提升
	if score.IsInKEV && score.Priority > 1 {
		score.Priority--
	}
}

// determineRiskLevel 确定风险等级
func (lc *LEVCalculator) determineRiskLevel(score *LEVScore) {
	switch {
	case score.LEVScore >= 80:
		score.RiskLevel = "critical"
	case score.LEVScore >= 60:
		score.RiskLevel = "high"
	case score.LEVScore >= 40:
		score.RiskLevel = "medium"
	default:
		score.RiskLevel = "low"
	}
}

// determineExploitLikelihood 确定利用可能性
func (lc *LEVCalculator) determineExploitLikelihood(score *LEVScore) {
	switch {
	case score.IsInKEV:
		if score.IsRansomware {
			score.ExploitLikelihood = "known" // 已知利用且勒索软件关联
		} else {
			score.ExploitLikelihood = "known" // 已知利用
		}
	case score.EPSSScore >= lc.config.EPSSHighThreshold:
		score.ExploitLikelihood = "likely" // 高 EPSS 评分，很可能利用
	case score.EPSSScore >= lc.config.EPSSMediumThreshold:
		score.ExploitLikelihood = "potential" // 中等 EPSS 评分，潜在利用
	default:
		score.ExploitLikelihood = "unlikely" // 低 EPSS 评分，不太可能利用
	}
}

// determineRemediationUrgency 确定修复紧急程度
func (lc *LEVCalculator) determineRemediationUrgency(score *LEVScore) {
	switch {
	case score.Priority == 1:
		score.RemediationUrgency = "immediate"
	case score.Priority == 2:
		score.RemediationUrgency = "soon"
	case score.Priority == 3:
		score.RemediationUrgency = "scheduled"
	default:
		score.RemediationUrgency = "deferred"
	}

	// KEV 过期漏洞立即修复
	if score.KEVDueDate != nil && score.KEVDueDate.Before(time.Now()) {
		score.RemediationUrgency = "immediate"
	}
}

// calculateDueDate 计算建议修复期限
func (lc *LEVCalculator) calculateDueDate(score *LEVScore) {
	now := time.Now()

	// KEV 有官方期限
	if score.KEVDueDate != nil {
		score.DueDate = score.KEVDueDate
		return
	}

	// 根据优先级计算建议期限
	switch score.Priority {
	case 1:
		// 7 天内
		due := now.AddDate(0, 0, 7)
		score.DueDate = &due
	case 2:
		// 30 天内
		due := now.AddDate(0, 0, 30)
		score.DueDate = &due
	case 3:
		// 90 天内
		due := now.AddDate(0, 0, 90)
		score.DueDate = &due
	default:
		// 180 天内
		due := now.AddDate(0, 0, 180)
		score.DueDate = &due
	}
}

// ========== 批量计算 ==========

// CalculateBatch 批量计算 LEV 评分
func (lc *LEVCalculator) CalculateBatch(inputs []LEVInput) []*LEVScore {
	scores := make([]*LEVScore, len(inputs))
	for i, input := range inputs {
		scores[i] = lc.Calculate(input)
	}
	return scores
}

// SortByPriority 按优先级排序
func (lc *LEVCalculator) SortByPriority(scores []*LEVScore) []*LEVScore {
	// 使用快速排序
	result := make([]*LEVScore, len(scores))
	copy(result, scores)

	lc.quickSort(result, 0, len(result)-1)
	return result
}

func (lc *LEVCalculator) quickSort(scores []*LEVScore, low, high int) {
	if low < high {
		pivot := lc.partition(scores, low, high)
		lc.quickSort(scores, low, pivot-1)
		lc.quickSort(scores, pivot+1, high)
	}
}

func (lc *LEVCalculator) partition(scores []*LEVScore, low, high int) int {
	pivot := scores[high].LEVScore
	i := low - 1

	for j := low; j < high; j++ {
		if scores[j].LEVScore >= pivot {
			i++
			scores[i], scores[j] = scores[j], scores[i]
		}
	}

	scores[i+1], scores[high] = scores[high], scores[i+1]
	return i + 1
}

// ========== 统计报告 ==========

// LEVReport LEV 报告
type LEVReport struct {
	GeneratedAt           time.Time         `json:"generated_at"`
	TotalVulnerabilities  int               `json:"total_vulnerabilities"`
	ByPriority            map[int]int       `json:"by_priority"`
	ByRiskLevel           map[string]int    `json:"by_risk_level"`
	ByExploitLikelihood   map[string]int    `json:"by_exploit_likelihood"`
	KEVCount              int               `json:"kev_count"`
	RansomwareCount       int               `json:"ransomware_count"`
	OverdueCount          int               `json:"overdue_count"`
	AverageLEVScore       float64           `json:"average_lev_score"`
	TopRisks              []*LEVScore       `json:"top_risks"`
	RemediationSummary    RemediationSummary `json:"remediation_summary"`
}

// RemediationSummary 修复摘要
type RemediationSummary struct {
	Immediate int `json:"immediate"` // 需立即修复
	Soon      int `json:"soon"`      // 近期修复
	Scheduled int `json:"scheduled"` // 计划修复
	Deferred  int `json:"deferred"`  // 延后修复
}

// GenerateReport 生成 LEV 报告
func (lc *LEVCalculator) GenerateReport(scores []*LEVScore) *LEVReport {
	report := &LEVReport{
		GeneratedAt:          time.Now(),
		TotalVulnerabilities: len(scores),
		ByPriority:           make(map[int]int),
		ByRiskLevel:          make(map[string]int),
		ByExploitLikelihood:  make(map[string]int),
		TopRisks:             make([]*LEVScore, 0),
	}

	var totalScore float64
	now := time.Now()

	for _, score := range scores {
		// 统计优先级
		report.ByPriority[score.Priority]++

		// 统计风险等级
		report.ByRiskLevel[score.RiskLevel]++

		// 统计利用可能性
		report.ByExploitLikelihood[score.ExploitLikelihood]++

		// KEV 统计
		if score.IsInKEV {
			report.KEVCount++
		}

		// 勒索软件统计
		if score.IsRansomware {
			report.RansomwareCount++
		}

		// 过期统计
		if score.DueDate != nil && score.DueDate.Before(now) {
			report.OverdueCount++
		}

		// 修复紧急程度统计
		switch score.RemediationUrgency {
		case "immediate":
			report.RemediationSummary.Immediate++
		case "soon":
			report.RemediationSummary.Soon++
		case "scheduled":
			report.RemediationSummary.Scheduled++
		case "deferred":
			report.RemediationSummary.Deferred++
		}

		totalScore += score.LEVScore
	}

	// 平均评分
	if len(scores) > 0 {
		report.AverageLEVScore = totalScore / float64(len(scores))
	}

	// 获取前 10 个最高风险
	sorted := lc.SortByPriority(scores)
	topCount := 10
	if len(sorted) < topCount {
		topCount = len(sorted)
	}
	report.TopRisks = sorted[:topCount]

	return report
}

// ========== 字符串表示 ==========

func (s *LEVScore) String() string {
	return fmt.Sprintf("LEVScore{CVE: %s, LEV: %.2f, Priority: %d, Level: %s, Likelihood: %s}",
		s.CVEID, s.LEVScore, s.Priority, s.RiskLevel, s.ExploitLikelihood)
}