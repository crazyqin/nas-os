// Package storageroi 提供存储ROI分析系统
// 存储基础设施ROI计算、TCO总拥有成本分析、容量规划与成本预测
package storageroi

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"
)

// ========== 常量 ==========

const (
	Version = "1.0.0"
)

// ========== 存储类型 ==========

// StorageType 存储类型
type StorageType string

const (
	StorageTypeHDD     StorageType = "hdd"
	StorageTypeSSD     StorageType = "ssd"
	StorageTypeNVMe    StorageType = "nvme"
	StorageTypeTape    StorageType = "tape"
	StorageTypeCloud   StorageType = "cloud"
	StorageTypeHybrid  StorageType = "hybrid"
)

// ========== 成本类型 ==========

// CostCategory 成本类别
type CostCategory string

const (
	CostCategoryHardware    CostCategory = "hardware"
	CostCategorySoftware    CostCategory = "software"
	CostCategoryPower       CostCategory = "power"
	CostCategoryCooling     CostCategory = "cooling"
	CostCategoryNetwork     CostCategory = "network"
	CostCategoryLabor       CostCategory = "labor"
	CostCategoryMaintenance CostCategory = "maintenance"
	CostCategoryCloud       CostCategory = "cloud"
	CostCategoryBackup      CostCategory = "backup"
	CostCategoryCompliance  CostCategory = "compliance"
)

// ========== 数据结构 ==========

// StorageAsset 存储资产
type StorageAsset struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Type         StorageType `json:"type"`
	Capacity     int64       `json:"capacity"`      // 总容量 (bytes)
	Used         int64       `json:"used"`          // 已用容量
	CostPerTB    float64     `json:"cost_per_tb"`   // 每TB成本
	PurchaseDate time.Time   `json:"purchase_date"`
	Lifespan     int         `json:"lifespan"`      // 预期寿命 (年)
	PowerWatts   float64     `json:"power_watts"`   // 功耗
	Location     string      `json:"location"`
}

// CostRecord 成本记录
type CostRecord struct {
	ID          string       `json:"id"`
	AssetID     string       `json:"asset_id"`
	Category    CostCategory `json:"category"`
	Amount      float64      `json:"amount"`
	Currency    string       `json:"currency"`
	Date        time.Time    `json:"date"`
	Description string       `json:"description"`
	Recurring   bool         `json:"recurring"`
	Period      string       `json:"period,omitempty"` // "monthly", "yearly"
}

// ROIReport ROI报告
type ROIReport struct {
	ID              string          `json:"id"`
	GeneratedAt     time.Time       `json:"generated_at"`
	Period          string          `json:"period"`
	TotalInvestment float64         `json:"total_investment"`
	TotalSavings    float64         `json:"total_savings"`
	NetBenefit      float64         `json:"net_benefit"`
	ROIPercent      float64         `json:"roi_percent"`
	PaybackPeriod   float64         `json:"payback_period"` // 月
	NPV             float64         `json:"npv"`            // 净现值
	IRR             float64         `json:"irr"`            // 内部收益率
	BreakEvenPoint  time.Time       `json:"break_even_point"`
	Categories      []CategoryBreakdown `json:"categories"`
	Recommendations []string        `json:"recommendations"`
}

// CategoryBreakdown 分类明细
type CategoryBreakdown struct {
	Category CostCategory `json:"category"`
	Amount   float64      `json:"amount"`
	Share    float64      `json:"share"`
	Trend    string       `json:"trend"` // "up", "down", "stable"
}

// TCOResult TCO分析结果
type TCOResult struct {
	ID              string            `json:"id"`
	GeneratedAt     time.Time         `json:"generated_at"`
	AnalysisPeriod  int               `json:"analysis_period"` // 年
	InitialCost     float64           `json:"initial_cost"`
	AnnualCosts     map[string]float64 `json:"annual_costs"`
	TotalCost       float64           `json:"total_cost"`
	CostPerTB       float64           `json:"cost_per_tb"`
	CostPerTBMonth  float64           `json:"cost_per_tb_month"`
	Breakdown       []CategoryBreakdown `json:"breakdown"`
	Projection      []CostProjection  `json:"projection"`
}

// CostProjection 成本预测
type CostProjection struct {
	Year        int     `json:"year"`
	Capacity    int64   `json:"capacity"`
	TotalCost   float64 `json:"total_cost"`
	CostPerTB   float64 `json:"cost_per_tb"`
	GrowthRate  float64 `json:"growth_rate"`
}

// CapacityPlan 容量规划
type CapacityPlan struct {
	ID              string           `json:"id"`
	GeneratedAt     time.Time        `json:"generated_at"`
	CurrentUsage    int64            `json:"current_usage"`
	CurrentCapacity int64            `json:"current_capacity"`
	GrowthRate      float64          `json:"growth_rate"` // 每月增长率
	Projection      []CapacityPoint  `json:"projection"`
	Recommendations []CapRecommendation `json:"recommendations"`
}

// CapacityPoint 容量点
type CapacityPoint struct {
	Date        time.Time `json:"date"`
	Used        int64     `json:"used"`
	Available   int64     `json:"available"`
	Utilization float64   `json:"utilization"`
}

// CapRecommendation 容量建议
type CapRecommendation struct {
	Type        string  `json:"type"`
	Priority    string  `json:"priority"`
	Description string  `json:"description"`
	Cost        float64 `json:"cost"`
	Timeline    string  `json:"timeline"`
}

// CloudComparison 云端对比
type CloudComparison struct {
	ID               string            `json:"id"`
	GeneratedAt      time.Time         `json:"generated_at"`
	LocalTCO         float64           `json:"local_tco"`
	CloudTCO         float64           `json:"cloud_tco"`
	Savings          float64           `json:"savings"`
	Recommendation   string            `json:"recommendation"`
	Providers        []CloudProvider   `json:"providers"`
	Breakdown        []ComparisonItem  `json:"breakdown"`
}

// CloudProvider 云服务商
type CloudProvider struct {
	Name       string  `json:"name"`
	MonthlyCost float64 `json:"monthly_cost"`
	Storage    int64   `json:"storage"`
	Egress     float64 `json:"egress_cost"`
	Features   []string `json:"features"`
}

// ComparisonItem 对比项
type ComparisonItem struct {
	Category string  `json:"category"`
	Local    float64 `json:"local"`
	Cloud    float64 `json:"cloud"`
	Diff     float64 `json:"diff"`
}

// ========== 分析器 ==========

// ROIAnalyzer ROI分析器
type ROIAnalyzer struct {
	mu       sync.RWMutex
	assets   map[string]*StorageAsset
	costs    []*CostRecord
	reports  map[string]*ROIReport
	tcoResults map[string]*TCOResult
	plans    map[string]*CapacityPlan
}

// NewROIAnalyzer 创建ROI分析器
func NewROIAnalyzer() *ROIAnalyzer {
	return &ROIAnalyzer{
		assets:     make(map[string]*StorageAsset),
		costs:      make([]*CostRecord, 0),
		reports:    make(map[string]*ROIReport),
		tcoResults: make(map[string]*TCOResult),
		plans:      make(map[string]*CapacityPlan),
	}
}

// AddAsset 添加存储资产
func (a *ROIAnalyzer) AddAsset(asset *StorageAsset) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if asset.ID == "" {
		asset.ID = fmt.Sprintf("asset-%d", time.Now().UnixNano())
	}
	a.assets[asset.ID] = asset
	return nil
}

// RemoveAsset 移除存储资产
func (a *ROIAnalyzer) RemoveAsset(assetID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.assets[assetID]; !exists {
		return fmt.Errorf("asset not found: %s", assetID)
	}
	delete(a.assets, assetID)
	return nil
}

// GetAssets 获取所有资产
func (a *ROIAnalyzer) GetAssets() []*StorageAsset {
	a.mu.RLock()
	defer a.mu.RUnlock()

	assets := make([]*StorageAsset, 0, len(a.assets))
	for _, asset := range a.assets {
		assets = append(assets, asset)
	}
	return assets
}

// AddCostRecord 添加成本记录
func (a *ROIAnalyzer) AddCostRecord(record *CostRecord) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if record.ID == "" {
		record.ID = fmt.Sprintf("cost-%d", time.Now().UnixNano())
	}
	a.costs = append(a.costs, record)
	return nil
}

// CalculateROI 计算ROI
func (a *ROIAnalyzer) CalculateROI(period string, years int) *ROIReport {
	a.mu.RLock()
	defer a.mu.RUnlock()

	report := &ROIReport{
		ID:          fmt.Sprintf("roi-%d", time.Now().UnixNano()),
		GeneratedAt: time.Now(),
		Period:      period,
	}

	// 计算总投资
	totalInvestment := 0.0
	for _, asset := range a.assets {
		totalInvestment += float64(asset.Capacity) / (1024 * 1024 * 1024 * 1024) * asset.CostPerTB
	}
	report.TotalInvestment = totalInvestment

	// 计算年度成本节省（相比云存储）
	cloudCostPerTBYear := 240.0 // 假设云存储每TB每年$240
	localCostPerTBYear := 0.0
	for _, asset := range a.assets {
		localCostPerTBYear += asset.CostPerTB / float64(asset.Lifespan)
	}
	totalTB := 0.0
	for _, asset := range a.assets {
		totalTB += float64(asset.Capacity) / (1024 * 1024 * 1024 * 1024)
	}
	annualSavings := (cloudCostPerTBYear - localCostPerTBYear) * totalTB
	report.TotalSavings = annualSavings * float64(years)

	// 计算净收益和ROI
	report.NetBenefit = report.TotalSavings - report.TotalInvestment
	if report.TotalInvestment > 0 {
		report.ROIPercent = (report.NetBenefit / report.TotalInvestment) * 100
	}

	// 计算回收期
	if annualSavings > 0 {
		report.PaybackPeriod = report.TotalInvestment / annualSavings * 12 // 月
		report.BreakEvenPoint = time.Now().AddDate(0, int(report.PaybackPeriod), 0)
	}

	// 计算NPV
	discountRate := 0.08 // 8% 折现率
	report.NPV = -report.TotalInvestment
	for y := 1; y <= years; y++ {
		report.NPV += annualSavings / math.Pow(1+discountRate, float64(y))
	}

	// 生成分类明细
	categoryMap := make(map[CostCategory]float64)
	for _, cost := range a.costs {
		categoryMap[cost.Category] += cost.Amount
	}
	totalCosts := 0.0
	for _, amount := range categoryMap {
		totalCosts += amount
	}
	for cat, amount := range categoryMap {
		report.Categories = append(report.Categories, CategoryBreakdown{
			Category: cat,
			Amount:   amount,
			Share:    amount / totalCosts * 100,
			Trend:    "stable",
		})
	}
	sort.Slice(report.Categories, func(i, j int) bool {
		return report.Categories[i].Amount > report.Categories[j].Amount
	})

	// 生成建议
	report.Recommendations = a.generateRecommendations(report)

	a.reports[report.ID] = report
	return report
}

// CalculateTCO 计算TCO
func (a *ROIAnalyzer) CalculateTCO(years int) *TCOResult {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := &TCOResult{
		ID:             fmt.Sprintf("tco-%d", time.Now().UnixNano()),
		GeneratedAt:    time.Now(),
		AnalysisPeriod: years,
		AnnualCosts:    make(map[string]float64),
	}

	// 初始成本（硬件采购）
	initialCost := 0.0
	for _, asset := range a.assets {
		initialCost += float64(asset.Capacity) / (1024 * 1024 * 1024 * 1024) * asset.CostPerTB
	}
	result.InitialCost = initialCost

	// 年度运营成本
	powerCost := 0.0
	coolingCost := 0.0
	maintenanceCost := 0.0
	for _, asset := range a.assets {
		// 电费 ($0.12/kWh)
		annualPower := asset.PowerWatts * 24 * 365 / 1000
		powerCost += annualPower * 0.12
		// 冷却成本 (功耗的40%)
		coolingCost += annualPower * 0.12 * 0.4
		// 维护成本 (硬件成本的10%)
		hardwareCost := float64(asset.Capacity) / (1024 * 1024 * 1024 * 1024) * asset.CostPerTB
		maintenanceCost += hardwareCost * 0.1
	}

	result.AnnualCosts["power"] = powerCost
	result.AnnualCosts["cooling"] = coolingCost
	result.AnnualCosts["maintenance"] = maintenanceCost
	result.AnnualCosts["labor"] = 5000 // 假设每年$5000人力成本
	result.AnnualCosts["network"] = 1200 // 假设每年$1200网络成本

	// 总成本
	totalAnnual := 0.0
	for _, cost := range result.AnnualCosts {
		totalAnnual += cost
	}
	result.TotalCost = initialCost + totalAnnual*float64(years)

	// 每TB成本
	totalTB := 0.0
	for _, asset := range a.assets {
		totalTB += float64(asset.Capacity) / (1024 * 1024 * 1024 * 1024)
	}
	if totalTB > 0 {
		result.CostPerTB = result.TotalCost / totalTB
		result.CostPerTBMonth = result.CostPerTB / float64(years) / 12
	}

	// 成本预测
	for y := 1; y <= years; y++ {
		growthRate := 0.2 // 假设每年20%数据增长
		projectedCapacity := totalTB * math.Pow(1+growthRate, float64(y))
		annualCost := totalAnnual * math.Pow(1+0.05, float64(y)) // 成本每年增长5%
		result.Projection = append(result.Projection, CostProjection{
			Year:       y,
			Capacity:   int64(projectedCapacity * 1024 * 1024 * 1024 * 1024),
			TotalCost:  initialCost + annualCost*float64(y),
			CostPerTB:  (initialCost + annualCost*float64(y)) / projectedCapacity,
			GrowthRate: growthRate,
		})
	}

	// 分类明细
	totalCost := 0.0
	for _, cost := range result.AnnualCosts {
		totalCost += cost
	}
	for cat, amount := range result.AnnualCosts {
		result.Breakdown = append(result.Breakdown, CategoryBreakdown{
			Category: CostCategory(cat),
			Amount:   amount * float64(years),
			Share:    amount / totalCost * 100,
		})
	}

	a.tcoResults[result.ID] = result
	return result
}

// GenerateCapacityPlan 生成容量规划
func (a *ROIAnalyzer) GenerateCapacityPlan(months int, growthRate float64) *CapacityPlan {
	a.mu.RLock()
	defer a.mu.RUnlock()

	plan := &CapacityPlan{
		ID:          fmt.Sprintf("plan-%d", time.Now().UnixNano()),
		GeneratedAt: time.Now(),
		GrowthRate:  growthRate,
	}

	// 计算当前使用情况
	totalCapacity := int64(0)
	totalUsed := int64(0)
	for _, asset := range a.assets {
		totalCapacity += asset.Capacity
		totalUsed += asset.Used
	}
	plan.CurrentUsage = totalUsed
	plan.CurrentCapacity = totalCapacity

	// 生成预测点
	currentUsed := float64(totalUsed)
	for m := 1; m <= months; m++ {
		currentUsed *= (1 + growthRate)
		utilization := currentUsed / float64(totalCapacity)
		plan.Projection = append(plan.Projection, CapacityPoint{
			Date:        time.Now().AddDate(0, m, 0),
			Used:        int64(currentUsed),
			Available:   totalCapacity - int64(currentUsed),
			Utilization: utilization * 100,
		})

		// 添加建议
		if utilization > 0.8 && len(plan.Recommendations) == 0 {
			plan.Recommendations = append(plan.Recommendations, CapRecommendation{
				Type:        "expand",
				Priority:    "high",
				Description: fmt.Sprintf("预计 %d 个月后容量将达到 %.0f%%，建议扩容", m, utilization*100),
				Cost:        float64(totalCapacity) / (1024 * 1024 * 1024 * 1024) * 100, // 假设$100/TB
				Timeline:    fmt.Sprintf("%d 个月内", m),
			})
		}
	}

	a.plans[plan.ID] = plan
	return plan
}

// CompareWithCloud 与云存储对比
func (a *ROIAnalyzer) CompareWithCloud(years int) *CloudComparison {
	a.mu.RLock()
	defer a.mu.RUnlock()

	comp := &CloudComparison{
		ID:          fmt.Sprintf("cloud-%d", time.Now().UnixNano()),
		GeneratedAt: time.Now(),
	}

	// 计算本地TCO
	tco := a.CalculateTCO(years)
	comp.LocalTCO = tco.TotalCost

	// 计算云存储成本
	totalTB := 0.0
	for _, asset := range a.assets {
		totalTB += float64(asset.Capacity) / (1024 * 1024 * 1024 * 1024)
	}

	// 云服务商定价
	providers := []CloudProvider{
		{
			Name:        "AWS S3 Standard",
			MonthlyCost: totalTB * 23, // $23/TB/月
			Storage:     int64(totalTB * 1024 * 1024 * 1024 * 1024),
			Egress:      totalTB * 0.09 * 1024, // $0.09/GB
			Features:    []string{"11个9持久性", "全球分发", "版本控制"},
		},
		{
			Name:        "Azure Blob Storage",
			MonthlyCost: totalTB * 18, // $18/TB/月
			Storage:     int64(totalTB * 1024 * 1024 * 1024 * 1024),
			Egress:      totalTB * 0.087 * 1024,
			Features:    []string{"分层存储", "CDN集成", "AI集成"},
		},
		{
			Name:        "Google Cloud Storage",
			MonthlyCost: totalTB * 20, // $20/TB/月
			Storage:     int64(totalTB * 1024 * 1024 * 1024 * 1024),
			Egress:      totalTB * 0.12 * 1024,
			Features:    []string{"多区域", "生命周期管理", "BigQuery集成"},
		},
	}
	comp.Providers = providers

	// 计算平均云成本
	avgCloudMonthly := 0.0
	for _, p := range providers {
		avgCloudMonthly += p.MonthlyCost
	}
	avgCloudMonthly /= float64(len(providers))
	comp.CloudTCO = avgCloudMonthly * 12 * float64(years)

	// 计算节省
	comp.Savings = comp.CloudTCO - comp.LocalTCO
	if comp.Savings > 0 {
		comp.Recommendation = "本地存储更经济"
	} else {
		comp.Recommendation = "云存储更经济"
	}

	// 生成对比明细
	comp.Breakdown = []ComparisonItem{
		{Category: "硬件/存储", Local: tco.InitialCost, Cloud: 0},
		{Category: "电力冷却", Local: (tco.AnnualCosts["power"] + tco.AnnualCosts["cooling"]) * float64(years), Cloud: 0},
		{Category: "维护人力", Local: (tco.AnnualCosts["maintenance"] + tco.AnnualCosts["labor"]) * float64(years), Cloud: avgCloudMonthly * 12 * float64(years)},
	}

	return comp
}

// generateRecommendations 生成建议
func (a *ROIAnalyzer) generateRecommendations(report *ROIReport) []string {
	var recs []string

	if report.ROIPercent > 20 {
		recs = append(recs, "ROI表现优秀，建议继续投资本地存储基础设施")
	} else if report.ROIPercent > 0 {
		recs = append(recs, "ROI为正但收益有限，建议优化运营成本")
	} else {
		recs = append(recs, "ROI为负，建议评估云存储方案")
	}

	if report.PaybackPeriod > 36 {
		recs = append(recs, "回收期较长，建议分阶段投资")
	}

	// 检查成本分布
	for _, cat := range report.Categories {
		if cat.Share > 40 {
			recs = append(recs, fmt.Sprintf("%s 占比过高 (%.0f%%)，建议优化", cat.Category, cat.Share))
		}
	}

	return recs
}

// GetReports 获取所有报告
func (a *ROIAnalyzer) GetReports() []*ROIReport {
	a.mu.RLock()
	defer a.mu.RUnlock()

	reports := make([]*ROIReport, 0, len(a.reports))
	for _, r := range a.reports {
		reports = append(reports, r)
	}
	return reports
}

// ========== HTTP API ==========

// Handler HTTP API处理器
type Handler struct {
	analyzer *ROIAnalyzer
}

// NewHandler 创建处理器
func NewHandler(analyzer *ROIAnalyzer) *Handler {
	return &Handler{analyzer: analyzer}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/assets", h.handleAssets)
	mux.HandleFunc(prefix+"/costs", h.handleCosts)
	mux.HandleFunc(prefix+"/roi", h.handleROI)
	mux.HandleFunc(prefix+"/tco", h.handleTCO)
	mux.HandleFunc(prefix+"/capacity", h.handleCapacity)
	mux.HandleFunc(prefix+"/cloud-compare", h.handleCloudCompare)
	mux.HandleFunc(prefix+"/reports", h.handleReports)
}

func (h *Handler) handleAssets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(h.analyzer.GetAssets())
	case http.MethodPost:
		var asset StorageAsset
		if err := json.NewDecoder(r.Body).Decode(&asset); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.analyzer.AddAsset(&asset)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(asset)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleCosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var record CostRecord
	if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.analyzer.AddCostRecord(&record)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(record)
}

func (h *Handler) handleROI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	report := h.analyzer.CalculateROI("yearly", 5)
	json.NewEncoder(w).Encode(report)
}

func (h *Handler) handleTCO(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	result := h.analyzer.CalculateTCO(5)
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) handleCapacity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	plan := h.analyzer.GenerateCapacityPlan(12, 0.05)
	json.NewEncoder(w).Encode(plan)
}

func (h *Handler) handleCloudCompare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	comp := h.analyzer.CompareWithCloud(5)
	json.NewEncoder(w).Encode(comp)
}

func (h *Handler) handleReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(h.analyzer.GetReports())
}
