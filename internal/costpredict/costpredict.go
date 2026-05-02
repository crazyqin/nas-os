// Package costpredict 提供基于历史数据的存储成本智能预测功能。
// 支持线性回归、指数平滑等预测算法，多维度成本分析和优化建议。
package costpredict

import (
	"errors"
	"math"
	"sort"
	"sync"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrInsufficientData 历史数据不足.
	ErrInsufficientData = errors.New("历史数据不足，至少需要2个数据点")
	// ErrInvalidParams 无效输入参数.
	ErrInvalidParams = errors.New("无效输入参数")
	// ErrCurrencyNotFound 币种不存在.
	ErrCurrencyNotFound = errors.New("不支持的币种")
	// ErrReportNotFound 报告不存在.
	ErrReportNotFound = errors.New("报告不存在")
)

// ========== 币种 ==========

// Currency 币种定义.
type Currency string

const (
	// CNY 人民币.
	CNY Currency = "CNY"
	// USD 美元.
	USD Currency = "USD"
	// EUR 欧元.
	EUR Currency = "EUR"
	// JPY 日元.
	JPY Currency = "JPY"
)

// CurrencyRate 币种汇率（相对 CNY）.
type CurrencyRate struct {
	Code Currency `json:"code"`
	Name string   `json:"name"`
	// Rate 兑换1单位CNY所需的外币数量.
	Rate float64 `json:"rate"`
}

// ========== 存储类型 ==========

// StorageType 存储类型.
type StorageType string

const (
	// StorageTypeHDD 机械硬盘.
	StorageTypeHDD StorageType = "hdd"
	// StorageTypeSSD 固态硬盘.
	StorageTypeSSD StorageType = "ssd"
	// StorageTypeNVMe NVMe固态.
	StorageTypeNVMe StorageType = "nvme"
	// StorageTypeObject 对象存储.
	StorageTypeObject StorageType = "object"
	// StorageTypeTape 磁带存储.
	StorageTypeTape StorageType = "tape"
)

// ========== 核心数据结构 ==========

// CostRecord 成本记录.
type CostRecord struct {
	// Time 记录时间.
	Time time.Time `json:"time"`
	// Department 所属部门.
	Department string `json:"department"`
	// Project 所属项目.
	Project string `json:"project"`
	// StorageType 存储类型.
	StorageType StorageType `json:"storage_type"`
	// CostCost 当期成本（CNY）.
	Cost float64 `json:"cost"`
	// UsedCapacity 已用容量（字节）.
	UsedCapacity int64 `json:"used_capacity"`
	// TotalCapacity 总容量（字节）.
	TotalCapacity int64 `json:"total_capacity"`
}

// PredictionResult 预测结果.
type PredictionResult struct {
	// Method 预测方法.
	Method string `json:"method"`
	// PredictedCost 预测成本（CNY）.
	PredictedCost float64 `json:"predicted_cost"`
	// ConfidenceLow 置信区间下限.
	ConfidenceLow float64 `json:"confidence_low"`
	// ConfidenceHigh 置信区间上限.
	ConfidenceHigh float64 `json:"confidence_high"`
	// PredictedCapacity 预测容量（字节）.
	PredictedCapacity int64 `json:"predicted_capacity"`
	// PeriodsAhead 预测未来期数.
	PeriodsAhead int `json:"periods_ahead"`
}

// OptimizationSuggestion 优化建议.
type OptimizationSuggestion struct {
	// Type 建议类型.
	Type string `json:"type"`
	// Title 建议标题.
	Title string `json:"title"`
	// Description 详细描述.
	Description string `json:"description"`
	// EstimatedSaving 预估节省金额（CNY/月）.
	EstimatedSaving float64 `json:"estimated_saving"`
	// ImpactLevel 影响程度: low/medium/high.
	ImpactLevel string `json:"impact_level"`
}

// CapacityGrowthForecast 容量增长预测.
type CapacityGrowthForecast struct {
	// Month 月份.
	Month time.Time `json:"month"`
	// PredictedUsed 预测已用容量（字节）.
	PredictedUsed int64 `json:"predicted_used"`
	// GrowthRate 增长率.
	GrowthRate float64 `json:"growth_rate"`
	// IsFull 是否将满（使用率>90%）.
	IsFull bool `json:"is_full"`
}

// BudgetAlert 预算告警.
type BudgetAlert struct {
	// Department 部门.
	Department string `json:"department"`
	// Project 项目.
	Project string `json:"project"`
	// BudgetAmount 预算金额（CNY）.
	BudgetAmount float64 `json:"budget_amount"`
	// PredictedCost 预测成本（CNY）.
	PredictedCost float64 `json:"predicted_cost"`
	// OverrunAmount 超支金额.
	OverrunAmount float64 `json:"overrun_amount"`
	// OverrunPercent 超支百分比.
	OverrunPercent float64 `json:"overrun_percent"`
	// AlertLevel 告警等级: warning/critical.
	AlertLevel string `json:"alert_level"`
}

// CostReport 成本报告.
type CostReport struct {
	// ID 报告ID.
	ID string `json:"id"`
	// ReportType 报告类型: monthly/quarterly/yearly.
	ReportType string `json:"report_type"`
	// PeriodStart 周期开始.
	PeriodStart time.Time `json:"period_start"`
	// PeriodEnd 周期结束.
	PeriodEnd time.Time `json:"period_end"`
	// TotalCost 总成本.
	TotalCost float64 `json:"total_cost"`
	// Currency 币种.
	Currency Currency `json:"currency"`
	// DepartmentCosts 部门成本明细.
	DepartmentCosts map[string]float64 `json:"department_costs"`
	// ProjectCosts 项目成本明细.
	ProjectCosts map[string]float64 `json:"project_costs"`
	// StorageTypeCosts 存储类型成本明细.
	StorageTypeCosts map[string]float64 `json:"storage_type_costs"`
	// Predictions 预测信息.
	Predictions []PredictionResult `json:"predictions"`
	// Suggestions 优化建议.
	Suggestions []OptimizationSuggestion `json:"suggestions"`
	// Alerts 告警.
	Alerts []BudgetAlert `json:"alerts"`
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generated_at"`
}

// ========== 预测引擎 ==========

// Predictor 预测引擎.
type Predictor struct {
	mu           sync.RWMutex
	records      []CostRecord
	currencies   map[Currency]CurrencyRate
	reports      map[string]*CostReport
	budgetLimits map[string]float64 // department -> budget
}

// NewPredictor 创建预测引擎.
func NewPredictor() *Predictor {
	return &Predictor{
		records:      make([]CostRecord, 0),
		reports:      make(map[string]*CostReport),
		budgetLimits: make(map[string]float64),
		currencies: map[Currency]CurrencyRate{
			CNY: {Code: CNY, Name: "人民币", Rate: 1.0},
			USD: {Code: USD, Name: "美元", Rate: 0.14},
			EUR: {Code: EUR, Name: "欧元", Rate: 0.13},
			JPY: {Code: JPY, Name: "日元", Rate: 21.0},
		},
	}
}

// AddRecord 添加成本记录.
func (p *Predictor) AddRecord(r CostRecord) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.records = append(p.records, r)
}

// AddRecords 批量添加成本记录.
func (p *Predictor) AddRecords(records []CostRecord) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.records = append(p.records, records...)
}

// SetBudgetLimit 设置部门预算.
func (p *Predictor) SetBudgetLimit(department string, amount float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.budgetLimits[department] = amount
}

// GetBudgetLimit 获取部门预算.
func (p *Predictor) GetBudgetLimit(department string) (float64, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	amt, ok := p.budgetLimits[department]
	return amt, ok
}

// SetCurrencyRate 设置币种汇率.
func (p *Predictor) SetCurrencyRate(rate CurrencyRate) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.currencies[rate.Code] = rate
}

// GetRecords 获取所有成本记录.
func (p *Predictor) GetRecords() []CostRecord {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]CostRecord, len(p.records))
	copy(out, p.records)
	return out
}

// ========== 线性回归 ==========

// LinearRegression 执行简单线性回归，返回斜率和截距.
// 使用最小二乘法拟合 y = slope*x + intercept.
func LinearRegression(x, y []float64) (slope, intercept float64, err error) {
	if len(x) != len(y) || len(x) < 2 {
		return 0, 0, ErrInsufficientData
	}
	n := float64(len(x))
	var sumX, sumY, sumXY, sumX2 float64
	for i := range x {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
	}
	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0, sumY / n, nil
	}
	slope = (n*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / n
	return slope, intercept, nil
}

// ========== 指数平滑 ==========

// ExponentialSmoothing 执行简单指数平滑.
// alpha 为平滑系数 (0,1)，返回平滑后的序列.
func ExponentialSmoothing(data []float64, alpha float64) ([]float64, error) {
	if len(data) < 1 {
		return nil, ErrInsufficientData
	}
	if alpha <= 0 || alpha >= 1 {
		alpha = 0.3
	}
	result := make([]float64, len(data))
	result[0] = data[0]
	for i := 1; i < len(data); i++ {
		result[i] = alpha*data[i] + (1-alpha)*result[i-1]
	}
	return result, nil
}

// DoubleExponentialSmoothing 双指数平滑（Holt方法），捕捉趋势.
// alpha 为水平平滑系数，beta 为趋势平滑系数.
func DoubleExponentialSmoothing(data []float64, alpha, beta float64) (level, trend []float64, err error) {
	if len(data) < 2 {
		return nil, nil, ErrInsufficientData
	}
	if alpha <= 0 || alpha >= 1 {
		alpha = 0.3
	}
	if beta <= 0 || beta >= 1 {
		beta = 0.1
	}
	n := len(data)
	level = make([]float64, n)
	trend = make([]float64, n)

	level[0] = data[0]
	trend[0] = data[1] - data[0]

	for i := 1; i < n; i++ {
		level[i] = alpha*data[i] + (1-alpha)*(level[i-1]+trend[i-1])
		trend[i] = beta*(level[i]-level[i-1]) + (1-beta)*trend[i-1]
	}
	return level, trend, nil
}

// ========== 多维度成本预测 ==========

// PredictCost 预测成本.
// periodsAhead 为预测未来期数，每期为1个月.
func (p *Predictor) PredictCost(periodsAhead int) ([]PredictionResult, error) {
	if periodsAhead < 1 {
		return nil, ErrInvalidParams
	}
	p.mu.RLock()
	records := make([]CostRecord, len(p.records))
	copy(records, p.records)
	p.mu.RUnlock()

	if len(records) < 2 {
		return nil, ErrInsufficientData
	}

	// 按时间排序
	sort.Slice(records, func(i, j int) bool {
		return records[i].Time.Before(records[j].Time)
	})

	costs := make([]float64, len(records))
	for i, r := range records {
		costs[i] = r.Cost
	}

	var results []PredictionResult

	// 方法1: 线性回归
	x := make([]float64, len(costs))
	for i := range x {
		x[i] = float64(i)
	}
	slope, intercept, err := LinearRegression(x, costs)
	if err == nil {
		predicted := slope*float64(len(costs)+periodsAhead-1) + intercept
		stdErr := calcStdError(x, costs, slope, intercept)
		results = append(results, PredictionResult{
			Method:          "linear_regression",
			PredictedCost:   math.Max(0, predicted),
			ConfidenceLow:   math.Max(0, predicted-1.96*stdErr),
			ConfidenceHigh:  predicted + 1.96*stdErr,
			PredictedCapacity: p.predictCapacity(records, periodsAhead),
			PeriodsAhead:    periodsAhead,
		})
	}

	// 方法2: 指数平滑
	level, trend, err := DoubleExponentialSmoothing(costs, 0.3, 0.1)
	if err == nil {
		n := len(level)
		predicted := level[n-1] + float64(periodsAhead)*trend[n-1]
		residuals := calcResidualsHolt(costs, level, trend)
		stdErr := calcArrayStdDev(residuals)
		results = append(results, PredictionResult{
			Method:          "exponential_smoothing",
			PredictedCost:   math.Max(0, predicted),
			ConfidenceLow:   math.Max(0, predicted-1.96*stdErr),
			ConfidenceHigh:  predicted + 1.96*stdErr,
			PredictedCapacity: p.predictCapacity(records, periodsAhead),
			PeriodsAhead:    periodsAhead,
		})
	}

	return results, nil
}

// PredictCostByDepartment 按部门预测成本.
func (p *Predictor) PredictCostByDepartment(department string, periodsAhead int) ([]PredictionResult, error) {
	p.mu.RLock()
	var filtered []CostRecord
	for _, r := range p.records {
		if r.Department == department {
			filtered = append(filtered, r)
		}
	}
	p.mu.RUnlock()

	orig := p.records
	p.records = filtered
	results, err := p.PredictCost(periodsAhead)
	p.records = orig
	return results, err
}

// PredictCostByProject 按项目预测成本.
func (p *Predictor) PredictCostByProject(project string, periodsAhead int) ([]PredictionResult, error) {
	p.mu.RLock()
	var filtered []CostRecord
	for _, r := range p.records {
		if r.Project == project {
			filtered = append(filtered, r)
		}
	}
	p.mu.RUnlock()

	orig := p.records
	p.records = filtered
	results, err := p.PredictCost(periodsAhead)
	p.records = orig
	return results, err
}

// PredictCostByStorageType 按存储类型预测成本.
func (p *Predictor) PredictCostByStorageType(st StorageType, periodsAhead int) ([]PredictionResult, error) {
	p.mu.RLock()
	var filtered []CostRecord
	for _, r := range p.records {
		if r.StorageType == st {
			filtered = append(filtered, r)
		}
	}
	p.mu.RUnlock()

	orig := p.records
	p.records = filtered
	results, err := p.PredictCost(periodsAhead)
	p.records = orig
	return results, err
}

// ========== 容量增长预测 ==========

// PredictCapacityGrowth 预测容量增长，返回未来months个月的预测.
func (p *Predictor) PredictCapacityGrowth(months int) ([]CapacityGrowthForecast, error) {
	if months < 1 {
		return nil, ErrInvalidParams
	}
	p.mu.RLock()
	records := make([]CostRecord, len(p.records))
	copy(records, p.records)
	p.mu.RUnlock()

	if len(records) < 2 {
		return nil, ErrInsufficientData
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Time.Before(records[j].Time)
	})

	// 提取容量数据
	capacities := make([]float64, len(records))
	for i, r := range records {
		capacities[i] = float64(r.UsedCapacity)
	}

	// 计算历史增长率
	var rates []float64
	for i := 1; i < len(capacities); i++ {
		if capacities[i-1] > 0 {
			rates = append(rates, (capacities[i]-capacities[i-1])/capacities[i-1])
		}
	}
	avgRate := 0.0
	if len(rates) > 0 {
		for _, r := range rates {
			avgRate += r
		}
		avgRate /= float64(len(rates))
	}

	// 取最近的容量和总容量
	lastRecord := records[len(records)-1]
	baseCapacity := float64(lastRecord.UsedCapacity)
	totalCapacity := float64(lastRecord.TotalCapacity)

	forecasts := make([]CapacityGrowthForecast, months)
	for i := 0; i < months; i++ {
		predicted := baseCapacity * math.Pow(1+avgRate, float64(i+1))
		forecasts[i] = CapacityGrowthForecast{
			Month:          lastRecord.Time.AddDate(0, i+1, 0),
			PredictedUsed:  int64(predicted),
			GrowthRate:     avgRate,
			IsFull:         totalCapacity > 0 && predicted/totalCapacity > 0.9,
		}
	}
	return forecasts, nil
}

// predictCapacity 预测指定期数后的容量.
func (p *Predictor) predictCapacity(records []CostRecord, periodsAhead int) int64 {
	if len(records) < 2 {
		return 0
	}
	capacities := make([]float64, len(records))
	for i, r := range records {
		capacities[i] = float64(r.UsedCapacity)
	}
	var rates []float64
	for i := 1; i < len(capacities); i++ {
		if capacities[i-1] > 0 {
			rates = append(rates, (capacities[i]-capacities[i-1])/capacities[i-1])
		}
	}
	avgRate := 0.0
	if len(rates) > 0 {
		for _, r := range rates {
			avgRate += r
		}
		avgRate /= float64(len(rates))
	}
	last := capacities[len(capacities)-1]
	return int64(last * math.Pow(1+avgRate, float64(periodsAhead)))
}

// ========== 优化建议引擎 ==========

// GenerateOptimizationSuggestions 生成成本优化建议.
func (p *Predictor) GenerateOptimizationSuggestions() []OptimizationSuggestion {
	p.mu.RLock()
	records := make([]CostRecord, len(p.records))
	copy(records, p.records)
	p.mu.RUnlock()

	var suggestions []OptimizationSuggestion

	if len(records) == 0 {
		return suggestions
	}

	// 分析冷数据
	coldDataSaving := p.analyzeColdData(records)
	if coldDataSaving > 0 {
		suggestions = append(suggestions, OptimizationSuggestion{
			Type:            "cold_archive",
			Title:           "冷数据归档建议",
			Description:     "检测到有数据超过90天未访问，建议将冷数据迁移到低成本存储层（如磁带或对象存储）",
			EstimatedSaving: coldDataSaving,
			ImpactLevel:     "medium",
		})
	}

	// 分析重复数据
	dedupSaving := p.analyzeDedup(records)
	if dedupSaving > 0 {
		suggestions = append(suggestions, OptimizationSuggestion{
			Type:            "deduplication",
			Title:           "重复数据删除建议",
			Description:     "检测到存储中存在重复数据，建议启用数据去重以节省存储空间和成本",
			EstimatedSaving: dedupSaving,
			ImpactLevel:     "high",
		})
	}

	// 分析压缩收益
	compressSaving := p.analyzeCompression(records)
	if compressSaving > 0 {
		suggestions = append(suggestions, OptimizationSuggestion{
			Type:            "compression",
			Title:           "数据压缩建议",
			Description:     "启用透明压缩可有效减少存储空间占用，预估可节省20-40%存储成本",
			EstimatedSaving: compressSaving,
			ImpactLevel:     "medium",
		})
	}

	// 分析存储分层
	tierSaving := p.analyzeTiering(records)
	if tierSaving > 0 {
		suggestions = append(suggestions, OptimizationSuggestion{
			Type:            "tiering",
			Title:           "存储分层优化建议",
			Description:     "根据数据访问频率，建议将热数据保留在SSD/NVMe，温数据迁移到HDD，冷数据归档",
			EstimatedSaving: tierSaving,
			ImpactLevel:     "high",
		})
	}

	return suggestions
}

// analyzeColdData 分析冷数据归档潜力.
func (p *Predictor) analyzeColdData(records []CostRecord) float64 {
	// 假设使用率超过80%的存储池中，约15%是冷数据
	var totalCost float64
	for _, r := range records {
		if r.TotalCapacity > 0 {
			usageRate := float64(r.UsedCapacity) / float64(r.TotalCapacity)
			if usageRate > 0.8 {
				totalCost += r.Cost
			}
		}
	}
	return totalCost * 0.15
}

// analyzeDedup 分析去重收益.
func (p *Predictor) analyzeDedup(records []CostRecord) float64 {
	// 假设平均去重率约25%
	var totalCost float64
	for _, r := range records {
		if r.StorageType == StorageTypeHDD || r.StorageType == StorageTypeSSD {
			totalCost += r.Cost
		}
	}
	return totalCost * 0.25
}

// analyzeCompression 分析压缩收益.
func (p *Predictor) analyzeCompression(records []CostRecord) float64 {
	// 假设压缩率约30%
	var totalCost float64
	for _, r := range records {
		totalCost += r.Cost
	}
	return totalCost * 0.30
}

// analyzeTiering 分析分层优化收益.
func (p *Predictor) analyzeTiering(records []CostRecord) float64 {
	// 高成本存储中约20%可以迁移到低成本层
	var highTierCost float64
	for _, r := range records {
		if r.StorageType == StorageTypeNVMe || r.StorageType == StorageTypeSSD {
			highTierCost += r.Cost
		}
	}
	return highTierCost * 0.20
}

// ========== 预算告警 ==========

// CheckBudgetAlerts 检查预算告警.
func (p *Predictor) CheckBudgetAlerts(periodsAhead int) ([]BudgetAlert, error) {
	p.mu.RLock()
	budgets := make(map[string]float64)
	for k, v := range p.budgetLimits {
		budgets[k] = v
	}
	p.mu.RUnlock()

	if len(budgets) == 0 {
		return nil, nil
	}

	var alerts []BudgetAlert
	for dept, budget := range budgets {
		results, err := p.PredictCostByDepartment(dept, periodsAhead)
		if err != nil || len(results) == 0 {
			continue
		}
		// 取线性回归结果
		predicted := results[0].PredictedCost
		if predicted > budget*0.8 {
			level := "warning"
			if predicted > budget {
				level = "critical"
			}
			alerts = append(alerts, BudgetAlert{
				Department:    dept,
				BudgetAmount:  budget,
				PredictedCost: predicted,
				OverrunAmount: math.Max(0, predicted-budget),
				OverrunPercent: ((predicted - budget) / budget) * 100,
				AlertLevel:    level,
			})
		}
	}
	return alerts, nil
}

// ========== 成本报告 ==========

// GenerateReport 生成成本报告.
func (p *Predictor) GenerateReport(reportType string, currency Currency) (*CostReport, error) {
	p.mu.RLock()
	records := make([]CostRecord, len(p.records))
	copy(records, p.records)
	p.mu.RUnlock()

	if len(records) == 0 {
		return nil, ErrInsufficientData
	}

	// 确定报告周期
	now := time.Now()
	var start, end time.Time
	switch reportType {
	case "monthly":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 1, 0).Add(-time.Nanosecond)
	case "quarterly":
		quarter := (int(now.Month())-1)/3*3 + 1
		start = time.Date(now.Year(), time.Month(quarter), 1, 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 3, 0).Add(-time.Nanosecond)
	case "yearly":
		start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		end = time.Date(now.Year(), 12, 31, 23, 59, 59, 0, now.Location())
	default:
		return nil, ErrInvalidParams
	}

	// 转换汇率
	rate, ok := p.currencies[currency]
	if !ok {
		return nil, ErrCurrencyNotFound
	}
	convertRate := rate.Rate

	// 筛选周期内的记录
	var periodRecords []CostRecord
	for _, r := range records {
		if !r.Time.Before(start) && !r.Time.After(end) {
			periodRecords = append(periodRecords, r)
		}
	}

	report := &CostReport{
		ID:               reportType + "-" + now.Format("20060102-150405"),
		ReportType:       reportType,
		PeriodStart:      start,
		PeriodEnd:        end,
		Currency:         currency,
		DepartmentCosts:  make(map[string]float64),
		ProjectCosts:     make(map[string]float64),
		StorageTypeCosts: make(map[string]float64),
		GeneratedAt:      now,
	}

	for _, r := range periodRecords {
		cost := r.Cost * convertRate
		report.TotalCost += cost
		report.DepartmentCosts[r.Department] += cost
		report.ProjectCosts[r.Project] += cost
		report.StorageTypeCosts[string(r.StorageType)] += cost
	}

	// 生成预测
	predictions, err := p.PredictCost(3)
	if err == nil {
		report.Predictions = predictions
	}

	// 生成优化建议
	report.Suggestions = p.GenerateOptimizationSuggestions()

	// 生成告警
	alerts, _ := p.CheckBudgetAlerts(3)
	report.Alerts = alerts

	// 保存报告
	p.mu.Lock()
	p.reports[report.ID] = report
	p.mu.Unlock()

	return report, nil
}

// GetReport 获取已生成的报告.
func (p *Predictor) GetReport(id string) (*CostReport, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	report, ok := p.reports[id]
	if !ok {
		return nil, ErrReportNotFound
	}
	return report, nil
}

// ListReports 列出所有已生成的报告.
func (p *Predictor) ListReports() []*CostReport {
	p.mu.RLock()
	defer p.mu.RUnlock()
	reports := make([]*CostReport, 0, len(p.reports))
	for _, r := range p.reports {
		reports = append(reports, r)
	}
	return reports
}

// ConvertCost 币种转换.
func (p *Predictor) ConvertCost(amountCNY float64, to Currency) (float64, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	rate, ok := p.currencies[to]
	if !ok {
		return 0, ErrCurrencyNotFound
	}
	return amountCNY * rate.Rate, nil
}

// ListCurrencies 列出支持的币种.
func (p *Predictor) ListCurrencies() []CurrencyRate {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]CurrencyRate, 0, len(p.currencies))
	for _, r := range p.currencies {
		result = append(result, r)
	}
	return result
}

// ========== 辅助函数 ==========

// calcStdError 计算回归标准误差.
func calcStdError(x, y []float64, slope, intercept float64) float64 {
	n := float64(len(x))
	if n <= 2 {
		return 0
	}
	var ssRes float64
	for i := range x {
		predicted := slope*x[i] + intercept
		diff := y[i] - predicted
		ssRes += diff * diff
	}
	return math.Sqrt(ssRes / (n - 2))
}

// calcResidualsHolt 计算Holt方法残差.
func calcResidualsHolt(data, level, trend []float64) []float64 {
	residuals := make([]float64, len(data))
	for i := range data {
		predicted := level[i]
		if i > 0 {
			predicted = level[i-1] + trend[i-1]
		}
		residuals[i] = data[i] - predicted
	}
	return residuals
}

// calcArrayStdDev 计算数组标准差.
func calcArrayStdDev(data []float64) float64 {
	if len(data) < 2 {
		return 0
	}
	var sum float64
	for _, v := range data {
		sum += v
	}
	mean := sum / float64(len(data))
	var variance float64
	for _, v := range data {
		diff := v - mean
		variance += diff * diff
	}
	return math.Sqrt(variance / float64(len(data)-1))
}
