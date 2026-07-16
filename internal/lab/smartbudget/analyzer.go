// Package smartbudget 提供预算分析逻辑
package smartbudget

import (
	"math"
	"sort"
	"time"
)

// Analyzer 预算分析器.
type Analyzer struct {
	manager *Manager
}

// NewAnalyzer 创建分析器.
func NewAnalyzer(manager *Manager) *Analyzer {
	return &Analyzer{
		manager: manager,
	}
}

// ========== 成本分析 ==========

// CostAnalysisResult 成本分析结果.
type CostAnalysisResult struct {
	TotalCost     float64            `json:"total_cost"`
	AverageCost   float64            `json:"average_cost"`
	MaxCost       float64            `json:"max_cost"`
	MinCost       float64            `json:"min_cost"`
	StdDev        float64            `json:"std_dev"`
	ByCategory    map[string]float64 `json:"by_category"`
	ByDepartment  map[string]float64 `json:"by_department"`
	ByProvider    map[string]float64 `json:"by_provider"`
	TrendAnalysis *TrendAnalysis     `json:"trend_analysis"`
	Anomalies     []AnomalyDetection `json:"anomalies"`
}

// TrendAnalysis 趋势分析结果.
type TrendAnalysis struct {
	Direction   Trend   `json:"direction"`    // up, down, stable
	GrowthRate  float64 `json:"growth_rate"`  // 月增长率
	Forecast3M  float64 `json:"forecast_3m"`  // 3个月预测
	Forecast6M  float64 `json:"forecast_6m"`  // 6个月预测
	Forecast12M float64 `json:"forecast_12m"` // 12个月预测
	Confidence  float64 `json:"confidence"`   // 置信度
}

// AnomalyDetection 异常检测.
type AnomalyDetection struct {
	Date      time.Time `json:"date"`
	Category  string    `json:"category"`
	Amount    float64   `json:"amount"`
	Expected  float64   `json:"expected"`
	Deviation float64   `json:"deviation"`
	Severity  string    `json:"severity"` // low, medium, high
}

// AnalyzeCosts 分析成本数据.
func (a *Analyzer) AnalyzeCosts(query CostQueryRequest) *CostAnalysisResult {
	costs := a.manager.GetCostBreakdowns(query)

	if len(costs) == 0 {
		return &CostAnalysisResult{
			ByCategory:   make(map[string]float64),
			ByDepartment: make(map[string]float64),
			ByProvider:   make(map[string]float64),
		}
	}

	result := &CostAnalysisResult{
		ByCategory:   make(map[string]float64),
		ByDepartment: make(map[string]float64),
		ByProvider:   make(map[string]float64),
		Anomalies:    make([]AnomalyDetection, 0),
	}

	// 收集所有金额
	amounts := make([]float64, 0, len(costs))
	for _, c := range costs {
		amounts = append(amounts, c.Amount)
		result.TotalCost += c.Amount
		result.ByCategory[c.Category] += c.Amount
		if c.Department != "" {
			result.ByDepartment[c.Department] += c.Amount
		}
		if c.Provider != "" {
			result.ByProvider[c.Provider] += c.Amount
		}
	}

	// 计算统计值
	result.AverageCost = result.TotalCost / float64(len(costs))
	result.MaxCost = amounts[0]
	result.MinCost = amounts[0]

	for _, a := range amounts {
		if a > result.MaxCost {
			result.MaxCost = a
		}
		if a < result.MinCost {
			result.MinCost = a
		}
	}

	// 计算标准差
	variance := 0.0
	for _, a := range amounts {
		diff := a - result.AverageCost
		variance += diff * diff
	}
	variance /= float64(len(amounts))
	result.StdDev = math.Sqrt(variance)

	// 检测异常
	result.Anomalies = a.detectAnomalies(costs, result.AverageCost, result.StdDev)

	// 趋势分析
	result.TrendAnalysis = a.analyzeTrend(query)

	return result
}

// detectAnomalies 检测异常数据.
func (a *Analyzer) detectAnomalies(costs []CostBreakdown, avg, stdDev float64) []AnomalyDetection {
	anomalies := make([]AnomalyDetection, 0)

	threshold := 2.0 // 2个标准差

	for _, c := range costs {
		deviation := math.Abs(c.Amount-avg) / stdDev
		if deviation > threshold {
			severity := "low"
			if deviation > 3 {
				severity = "high"
			} else if deviation > 2.5 {
				severity = "medium"
			}

			anomalies = append(anomalies, AnomalyDetection{
				Date:      time.Now(),
				Category:  c.Category,
				Amount:    c.Amount,
				Expected:  avg,
				Deviation: deviation,
				Severity:  severity,
			})
		}
	}

	return anomalies
}

// analyzeTrend 分析趋势.
func (a *Analyzer) analyzeTrend(query CostQueryRequest) *TrendAnalysis {
	trends := a.manager.GetCostTrends(TrendQueryRequest{
		Department: query.Department,
		Category:   query.Category,
		Months:     6,
	})

	if len(trends) < 2 {
		return &TrendAnalysis{
			Direction:  TrendStable,
			Confidence: 0,
		}
	}

	// 计算简单线性回归
	n := float64(len(trends))
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0

	for i, t := range trends {
		x := float64(i)
		y := t.Amount
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// 斜率 = (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)

	// 截距 = (sumY - slope*sumX) / n
	intercept := (sumY - slope*sumX) / n

	// 确定趋势方向
	direction := TrendStable
	growthRate := 0.0
	if slope > 10 {
		direction = TrendUp
		growthRate = slope / trends[len(trends)-1].Amount * 100
	} else if slope < -10 {
		direction = TrendDown
		growthRate = slope / trends[len(trends)-1].Amount * 100
	}

	// 计算预测值
	lastX := float64(len(trends) - 1)
	confidence := 70.0 // 基础置信度

	return &TrendAnalysis{
		Direction:   direction,
		GrowthRate:  growthRate,
		Forecast3M:  intercept + slope*(lastX+3),
		Forecast6M:  intercept + slope*(lastX+6),
		Forecast12M: intercept + slope*(lastX+12),
		Confidence:  confidence,
	}
}

// ========== 预算利用率分析 ==========

// BudgetUtilization 预算利用率.
type BudgetUtilization struct {
	PlanID        string  `json:"plan_id"`
	PlanName      string  `json:"plan_name"`
	Department    string  `json:"department"`
	BudgetCap     float64 `json:"budget_cap"`
	CurrentUse    float64 `json:"current_use"`
	Remaining     float64 `json:"remaining"`
	Utilization   float64 `json:"utilization"` // 0-100
	DaysRemaining int     `json:"days_remaining"`
	DailyBurnRate float64 `json:"daily_burn_rate"`
	ProjectedOver float64 `json:"projected_over"` // 预计超支
}

// AnalyzeUtilization 分析预算利用率.
func (a *Analyzer) AnalyzeUtilization() []BudgetUtilization {
	plans := a.manager.ListPlans()
	utilizations := make([]BudgetUtilization, 0, len(plans))

	for _, plan := range plans {
		util := BudgetUtilization{
			PlanID:     plan.ID,
			PlanName:   plan.Name,
			Department: plan.Department,
			BudgetCap:  plan.MonthlyCap,
			CurrentUse: plan.CurrentUse,
		}

		// 计算剩余
		util.Remaining = plan.MonthlyCap - plan.CurrentUse

		// 计算利用率
		if plan.MonthlyCap > 0 {
			util.Utilization = (plan.CurrentUse / plan.MonthlyCap) * 100
		}

		// 计算本月剩余天数
		now := time.Now()
		endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location())
		util.DaysRemaining = int(endOfMonth.Sub(now).Hours() / 24)

		// 计算日均消耗率
		dayOfMonth := now.Day()
		if dayOfMonth > 0 {
			util.DailyBurnRate = plan.CurrentUse / float64(dayOfMonth)
		}

		// 预测本月结束时的使用量
		projectedTotal := plan.CurrentUse + util.DailyBurnRate*float64(util.DaysRemaining)
		if projectedTotal > plan.MonthlyCap {
			util.ProjectedOver = projectedTotal - plan.MonthlyCap
		}

		utilizations = append(utilizations, util)
	}

	// 按利用率排序
	sort.Slice(utilizations, func(i, j int) bool {
		return utilizations[i].Utilization > utilizations[j].Utilization
	})

	return utilizations
}

// ========== 成本优化分析 ==========

// OptimizationImpact 优化影响分析.
type OptimizationImpact struct {
	TotalSavings    float64            `json:"total_savings"`
	ByType          map[string]float64 `json:"by_type"`
	ByDepartment    map[string]float64 `json:"by_department"`
	ROI             float64            `json:"roi"`
	PaybackPeriod   int                `json:"payback_period"` // 月
	Recommendations []string           `json:"recommendations"`
}

// AnalyzeOptimizationImpact 分析优化影响.
func (a *Analyzer) AnalyzeOptimizationImpact() *OptimizationImpact {
	opts := a.manager.GetOptimizations()

	impact := &OptimizationImpact{
		ByType:          make(map[string]float64),
		ByDepartment:    make(map[string]float64),
		Recommendations: make([]string, 0),
	}

	if len(opts) == 0 {
		return impact
	}

	// 按类型汇总
	for _, opt := range opts {
		impact.TotalSavings += opt.SavingEst
		impact.ByType[string(opt.Type)] += opt.SavingEst
		if opt.Department != "" {
			impact.ByDepartment[opt.Department] += opt.SavingEst
		}
	}

	// 生成建议
	for optType, saving := range impact.ByType {
		switch optType {
		case string(OptTypeColdMigration):
			impact.Recommendations = append(impact.Recommendations,
				"冷数据迁移可显著降低存储成本，建议优先实施")
		case string(OptTypeDedup):
			impact.Recommendations = append(impact.Recommendations,
				"数据去重可减少存储空间占用，建议定期执行")
		case string(OptTypeCompress):
			impact.Recommendations = append(impact.Recommendations,
				"数据压缩可进一步降低存储成本")
		}
		_ = saving
	}

	// 计算ROI和回收期
	implementationCost := impact.TotalSavings * 0.1 // 假设实施成本为节省的10%
	if implementationCost > 0 {
		impact.ROI = (impact.TotalSavings - implementationCost) / implementationCost * 100
		impact.PaybackPeriod = int(implementationCost / (impact.TotalSavings / 12))
	}

	return impact
}

// ========== 预算健康度评估 ==========

// HealthScore 健康度评分.
type HealthScore struct {
	Score       float64  `json:"score"` // 0-100
	Grade       string   `json:"grade"` // A, B, C, D, F
	Factors     []Factor `json:"factors"`
	Suggestions []string `json:"suggestions"`
}

// Factor 评估因子.
type Factor struct {
	Name    string  `json:"name"`
	Score   float64 `json:"score"`
	Weight  float64 `json:"weight"`
	Details string  `json:"details"`
}

// AssessHealth 评估预算健康度.
func (a *Analyzer) AssessHealth() *HealthScore {
	utilizations := a.AnalyzeUtilization()
	optImpact := a.AnalyzeOptimizationImpact()

	score := &HealthScore{
		Factors:     make([]Factor, 0),
		Suggestions: make([]string, 0),
	}

	totalScore := 0.0
	totalWeight := 0.0

	// 因子1: 预算利用率 (权重40%)
	utilFactor := Factor{
		Name:   "预算利用率",
		Weight: 40,
	}

	if len(utilizations) > 0 {
		avgUtil := 0.0
		for _, u := range utilizations {
			avgUtil += u.Utilization
		}
		avgUtil /= float64(len(utilizations))

		// 理想利用率: 70-90%
		switch {
		case avgUtil >= 70 && avgUtil <= 90:
			utilFactor.Score = 100
			utilFactor.Details = "预算利用率处于理想范围"
		case avgUtil >= 50 && avgUtil < 70:
			utilFactor.Score = 80
			utilFactor.Details = "预算利用率偏低，可适当增加投入"
			score.Suggestions = append(score.Suggestions, "建议评估是否有更多资源需求")
		case avgUtil > 90 && avgUtil <= 100:
			utilFactor.Score = 60
			utilFactor.Details = "预算利用率偏高，需关注是否超支"
			score.Suggestions = append(score.Suggestions, "建议审查近期支出，避免超支")
		case avgUtil > 100:
			utilFactor.Score = 30
			utilFactor.Details = "已超出预算"
			score.Suggestions = append(score.Suggestions, "立即审查支出，调整预算或削减成本")
		default:
			utilFactor.Score = 70
			utilFactor.Details = "预算使用率过低"
		}
	}

	score.Factors = append(score.Factors, utilFactor)
	totalScore += utilFactor.Score * utilFactor.Weight / 100
	totalWeight += utilFactor.Weight

	// 因子2: 成本趋势 (权重30%)
	trendFactor := Factor{
		Name:   "成本趋势",
		Weight: 30,
	}

	costAnalysis := a.AnalyzeCosts(CostQueryRequest{})
	if costAnalysis.TrendAnalysis != nil {
		switch costAnalysis.TrendAnalysis.Direction {
		case TrendDown:
			trendFactor.Score = 100
			trendFactor.Details = "成本呈下降趋势，表现良好"
		case TrendStable:
			trendFactor.Score = 80
			trendFactor.Details = "成本保持稳定"
		case TrendUp:
			if costAnalysis.TrendAnalysis.GrowthRate > 10 {
				trendFactor.Score = 40
				trendFactor.Details = "成本增长过快，需要关注"
				score.Suggestions = append(score.Suggestions, "成本增长超过10%，建议审查支出结构")
			} else {
				trendFactor.Score = 60
				trendFactor.Details = "成本略有上升"
			}
		}
	}

	score.Factors = append(score.Factors, trendFactor)
	totalScore += trendFactor.Score * trendFactor.Weight / 100
	totalWeight += trendFactor.Weight

	// 因子3: 优化采纳度 (权重30%)
	optFactor := Factor{
		Name:   "优化采纳度",
		Weight: 30,
	}

	if optImpact.TotalSavings > 0 {
		optFactor.Score = 70
		optFactor.Details = "已识别优化机会"
		if len(optImpact.Recommendations) > 0 {
			score.Suggestions = append(score.Suggestions, optImpact.Recommendations...)
		}
	} else {
		optFactor.Score = 50
		optFactor.Details = "暂无优化建议"
	}

	score.Factors = append(score.Factors, optFactor)
	totalScore += optFactor.Score * optFactor.Weight / 100
	totalWeight += optFactor.Weight

	// 计算总分
	if totalWeight > 0 {
		score.Score = totalScore / totalWeight * 100
	}

	// 确定等级
	switch {
	case score.Score >= 90:
		score.Grade = "A"
	case score.Score >= 80:
		score.Grade = "B"
	case score.Score >= 70:
		score.Grade = "C"
	case score.Score >= 60:
		score.Grade = "D"
	default:
		score.Grade = "F"
	}

	return score
}
