// Package tcodash 提供存储 TCO 实时仪表盘功能
// 对标 Synology 存储分析器和 TrueNAS 容量规划
package tcodash

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// CostCategory 成本分类
type CostCategory string

const (
	CostHardware    CostCategory = "hardware"
	CostPower       CostCategory = "power"
	CostStorage     CostCategory = "storage"
	CostNetwork     CostCategory = "network"
	CostMaintenance CostCategory = "maintenance"
	CostSoftware    CostCategory = "software"
	CostCloud       CostCategory = "cloud"
	CostBackup      CostCategory = "backup"
)

// CostEntry 成本条目
type CostEntry struct {
	ID          string       `json:"id"`
	Category    CostCategory `json:"category"`
	Description string       `json:"description"`
	Amount      float64      `json:"amount"`
	Currency    string       `json:"currency"`
	Timestamp   time.Time    `json:"timestamp"`
	Recurring   bool         `json:"recurring"`
	Period      string       `json:"period,omitempty"`
}

// StorageAsset 存储资产
type StorageAsset struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	CapacityTB    float64   `json:"capacity_tb"`
	UsedTB        float64   `json:"used_tb"`
	CostPerTB     float64   `json:"cost_per_tb"`
	PurchaseDate  time.Time `json:"purchase_date"`
	WarrantyYears int       `json:"warranty_years"`
	PowerWatts    float64   `json:"power_watts"`
	HealthPct     float64   `json:"health_pct"`
}

// UsageMetric 使用量指标
type UsageMetric struct {
	Timestamp  time.Time `json:"timestamp"`
	Volume     string    `json:"volume"`
	UsedTB     float64   `json:"used_tb"`
	TotalTB    float64   `json:"total_tb"`
	IOps       int       `json:"iops"`
	Throughput float64   `json:"throughput_mbps"`
	LatencyMS  float64   `json:"latency_ms"`
}

// Dashboard 仪表盘
type Dashboard struct {
	mu              sync.RWMutex
	entries         []*CostEntry
	assets          []*StorageAsset
	usageHistory    []*UsageMetric
	electricityRate float64
	currency        string
}

// NewDashboard 创建仪表盘
func NewDashboard(electricityRate float64, currency string) *Dashboard {
	if currency == "" {
		currency = "CNY"
	}
	return &Dashboard{
		entries:         make([]*CostEntry, 0),
		assets:          make([]*StorageAsset, 0),
		usageHistory:    make([]*UsageMetric, 0),
		electricityRate: electricityRate,
		currency:        currency,
	}
}

// AddCostEntry 添加成本条目
func (d *Dashboard) AddCostEntry(entry *CostEntry) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.entries = append(d.entries, entry)
}

// AddAsset 添加存储资产
func (d *Dashboard) AddAsset(asset *StorageAsset) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.assets = append(d.assets, asset)
}

// RecordUsage 记录使用量
func (d *Dashboard) RecordUsage(metric *UsageMetric) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.usageHistory = append(d.usageHistory, metric)
	// 保留最近 1000 条
	if len(d.usageHistory) > 1000 {
		d.usageHistory = d.usageHistory[len(d.usageHistory)-1000:]
	}
}

// TCOReport TCO 报告
type TCOReport struct {
	TotalCost        float64                  `json:"total_cost"`
	Currency         string                   `json:"currency"`
	CostByCategory   map[CostCategory]float64 `json:"cost_by_category"`
	Assets           []*StorageAsset          `json:"assets"`
	TotalCapacityTB  float64                  `json:"total_capacity_tb"`
	TotalUsedTB      float64                  `json:"total_used_tb"`
	UtilizationPct   float64                  `json:"utilization_pct"`
	CostPerTB        float64                  `json:"cost_per_tb"`
	MonthlyPowerCost float64                  `json:"monthly_power_cost"`
	AnnualPowerCost  float64                  `json:"annual_power_cost"`
	SavingsEstimate  float64                  `json:"savings_estimate"`
	Trend            []*UsageMetric           `json:"trend"`
	ROIAnalysis      *ROIAnalysis             `json:"roi_analysis"`
	GeneratedAt      time.Time                `json:"generated_at"`
}

// ROIAnalysis ROI 分析
type ROIAnalysis struct {
	InvestmentTotal float64 `json:"investment_total"`
	AnnualSavings   float64 `json:"annual_savings"`
	PaybackMonths   int     `json:"payback_months"`
	ThreeYearROI    float64 `json:"three_year_roi"`
	FiveYearROI     float64 `json:"five_year_roi"`
	NPV             float64 `json:"npv"`
	DiscountRate    float64 `json:"discount_rate"`
}

// GenerateReport 生成 TCO 报告
func (d *Dashboard) GenerateReport() *TCOReport {
	d.mu.RLock()
	defer d.mu.RUnlock()

	report := &TCOReport{
		Currency:       d.currency,
		CostByCategory: make(map[CostCategory]float64),
		GeneratedAt:    time.Now(),
	}

	// 汇总成本
	for _, entry := range d.entries {
		report.TotalCost += entry.Amount
		report.CostByCategory[entry.Category] += entry.Amount
	}

	// 汇总存储容量
	for _, asset := range d.assets {
		report.TotalCapacityTB += asset.CapacityTB
		report.TotalUsedTB += asset.UsedTB
		report.MonthlyPowerCost += asset.PowerWatts * 24 * 30 * d.electricityRate / 1000
	}
	report.AnnualPowerCost = report.MonthlyPowerCost * 12

	// 利用率
	if report.TotalCapacityTB > 0 {
		report.UtilizationPct = (report.TotalUsedTB / report.TotalCapacityTB) * 100
	}

	// 每TB成本
	if report.TotalUsedTB > 0 {
		report.CostPerTB = report.TotalCost / report.TotalUsedTB
	}

	// 节省估算（通过去重和分层可节省约 15-25%）
	if report.TotalCost > 0 {
		report.SavingsEstimate = report.TotalCost * 0.20
	}

	// 趋势
	if len(d.usageHistory) > 0 {
		report.Trend = d.usageHistory[len(d.usageHistory)-min(30, len(d.usageHistory)):]
	}

	// ROI 分析
	report.ROIAnalysis = d.calculateROI(report)

	return report
}

func (d *Dashboard) calculateROI(report *TCOReport) *ROIAnalysis {
	discountRate := 0.05
	investment := report.TotalCost
	annualSavings := report.SavingsEstimate

	if annualSavings <= 0 {
		return &ROIAnalysis{
			InvestmentTotal: investment,
			DiscountRate:    discountRate,
		}
	}

	payback := int(investment / annualSavings * 12)
	if payback <= 0 {
		payback = 1
	}

	threeYearSavings := annualSavings * 3
	fiveYearSavings := annualSavings * 5

	roi3 := ((threeYearSavings - investment) / investment) * 100
	roi5 := ((fiveYearSavings - investment) / investment) * 100

	// NPV: savings * (1 - (1+r)^-n) / r - investment
	npv5 := annualSavings*(1-1/(1+discountRate)*5)/discountRate - investment

	return &ROIAnalysis{
		InvestmentTotal: investment,
		AnnualSavings:   annualSavings,
		PaybackMonths:   payback,
		ThreeYearROI:    roi3,
		FiveYearROI:     roi5,
		NPV:             npv5,
		DiscountRate:    discountRate,
	}
}

// GetCostBreakdown 获取成本分解
func (d *Dashboard) GetCostBreakdown() []CostBreakdownItem {
	d.mu.RLock()
	defer d.mu.RUnlock()

	categoryTotals := make(map[CostCategory]float64)
	var total float64
	for _, entry := range d.entries {
		categoryTotals[entry.Category] += entry.Amount
		total += entry.Amount
	}

	var items []CostBreakdownItem
	for cat, amount := range categoryTotals {
		pct := 0.0
		if total > 0 {
			pct = (amount / total) * 100
		}
		items = append(items, CostBreakdownItem{
			Category:   cat,
			Amount:     amount,
			Percentage: pct,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Amount > items[j].Amount
	})
	return items
}

// CostBreakdownItem 成本分解项
type CostBreakdownItem struct {
	Category   CostCategory `json:"category"`
	Amount     float64      `json:"amount"`
	Percentage float64      `json:"percentage"`
}

// FormatReport 格式化报告
func (d *Dashboard) FormatReport(report *TCOReport) string {
	var sb strings.Builder
	sb.WriteString("存储 TCO 仪表盘:\n")
	sb.WriteString(strings.Repeat("═", 50) + "\n")
	sb.WriteString(fmt.Sprintf("总成本: %.2f %s\n", report.TotalCost, report.Currency))
	sb.WriteString(fmt.Sprintf("总容量: %.1f TB / 已用: %.1f TB (%.1f%%)\n",
		report.TotalCapacityTB, report.TotalUsedTB, report.UtilizationPct))
	sb.WriteString(fmt.Sprintf("每 TB 成本: %.2f %s/TB\n", report.CostPerTB, report.Currency))
	sb.WriteString(fmt.Sprintf("月度电费: %.2f %s\n", report.MonthlyPowerCost, report.Currency))
	sb.WriteString(fmt.Sprintf("预估节省: %.2f %s (20%%)\n\n", report.SavingsEstimate, report.Currency))

	sb.WriteString("成本分解:\n")
	breakdown := d.GetCostBreakdown()
	for _, item := range breakdown {
		sb.WriteString(fmt.Sprintf("  %-15s %.2f %s (%.1f%%)\n",
			item.Category, item.Amount, report.Currency, item.Percentage))
	}

	if report.ROIAnalysis != nil && report.ROIAnalysis.AnnualSavings > 0 {
		sb.WriteString("\nROI 分析:\n")
		sb.WriteString(fmt.Sprintf("  回本周期: %d 个月\n", report.ROIAnalysis.PaybackMonths))
		sb.WriteString(fmt.Sprintf("  3年 ROI: %.1f%%\n", report.ROIAnalysis.ThreeYearROI))
	}

	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
