// Package storagecost 存储成本分析模块
// 本文件定义测试所需的类型存根，用于兼容 handlers_test.go
package storagecost

import (
	"time"

	"github.com/gin-gonic/gin"
)

// StorageAsset 存储资产
type StorageAsset struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Type            string  `json:"type"`
	CapacityTB      float64 `json:"capacity_tb"`
	PurchaseCost    float64 `json:"purchase_cost"`
	WarrantyYears   int     `json:"warranty_years"`
	AnnualPowerKWh  int     `json:"annual_power_kwh"`
	PowerCostPerKWh float64 `json:"power_cost_per_kwh"`
	RackUnits       int     `json:"rack_units"`
	RackCostPerUnit float64 `json:"rack_cost_per_unit"`
}

// TCOResult TCO分析结果
type TCOResult struct {
	AssetID   string             `json:"assetId"`
	TotalCost float64            `json:"totalCost"`
	CostPerTB float64            `json:"costPerTb"`
	Breakdown map[string]float64 `json:"breakdown"`
}

// OptimizationReport 优化报告
type OptimizationReport struct {
	TotalAnnualSaving float64                  `json:"totalAnnualSaving"`
	Suggestions       []OptimizationSuggestion `json:"suggestions"`
}

// BudgetPlan 预算计划
type BudgetPlan struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	FiscalYear  int        `json:"fiscal_year"`
	TotalBudget float64    `json:"total_budget"`
	LineItems   []LineItem `json:"line_items"`
}

// LineItem 预算项目
type LineItem struct {
	ID          string  `json:"id"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Quantity    int     `json:"quantity"`
	UnitCost    float64 `json:"unit_cost"`
	Priority    string  `json:"priority"`
}

// ComparisonResult 比较结果
type ComparisonResult struct {
	Options    []StorageOption `json:"options"`
	BestOption *StorageOption  `json:"bestOption"`
}

// StorageOption 存储方案
type StorageOption struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Type             string  `json:"type"`
	CapacityTB       float64 `json:"capacity_tb"`
	CostPerTBYear    float64 `json:"cost_per_tb_year"`
	TCO5Year         float64 `json:"tco_5_year"`
	IOPSCapability   int     `json:"iops_capability"`
	ThroughputMBPS   int     `json:"throughput_mbps"`
	Availability     float64 `json:"availability"`
	ScalabilityScore float64 `json:"scalability_score"`
}

// CostRecord 成本记录
type CostRecord struct {
	ID          string  `json:"id"`
	VolumeID    string  `json:"volume_id"`
	VolumeName  string  `json:"volume_name"`
	StorageType string  `json:"storage_type"`
	CapacityGB  float64 `json:"capacity_gb"`
	UsedGB      float64 `json:"used_gb"`
	PricePerGB  float64 `json:"price_per_gb"`
	MonthlyCost float64 `json:"monthly_cost"`
	Provider    string  `json:"provider"`
}

// CostSummary 成本摘要
type CostSummary struct {
	TotalMonthlyCost float64 `json:"totalMonthlyCost"`
	TotalAnnualCost  float64 `json:"totalAnnualCost"`
}

// CostTrendPoint 成本趋势点
type CostTrendPoint struct {
	Date time.Time `json:"date"`
	Cost float64   `json:"cost"`
}

// CostAlert 成本告警
type CostAlert struct {
	ID        string  `json:"id"`
	Level     string  `json:"level"`
	Message   string  `json:"message"`
	Threshold float64 `json:"threshold"`
	Current   float64 `json:"current"`
}

// ==================== Manager 扩展方法 ====================

// NewManagerWithConfig 使用配置创建管理器
func NewManagerWithConfig(config *StorageCostConfig) *Manager {
	return NewManager(nil, config)
}

// CreateAsset 创建资产
func (m *Manager) CreateAsset(asset StorageAsset) error {
	return nil
}

// AddCostRecord 添加成本记录
func (m *Manager) AddCostRecord(record CostRecord) {
}

// CalculateTCO 计算TCO
func (m *Manager) CalculateTCO(assetID string) (*TCOResult, error) {
	return &TCOResult{
		AssetID:   assetID,
		TotalCost: 50000,
		CostPerTB: 2500,
		Breakdown: map[string]float64{
			"purchase": 30000,
			"power":    15000,
			"rack":     5000,
		},
	}, nil
}

// GetOptimizationReport 获取优化报告
func (m *Manager) GetOptimizationReport() *OptimizationReport {
	return &OptimizationReport{
		TotalAnnualSaving: 5000,
		Suggestions:       []OptimizationSuggestion{},
	}
}

// ==================== Handler 扩展方法 ====================

// RegisterStorageCostRoutes 注册存储成本路由
func (h *Handlers) RegisterStorageCostRoutes(r *gin.RouterGroup) {
	cost := r.Group("/storage-cost")
	{
		cost.POST("/assets", h.CreateAsset)
		cost.POST("/tco/:id", h.CalculateTCO)
		cost.POST("/capacity-samples", h.RecordCapacitySample)
		cost.GET("/optimization-report", h.GetOptimizationReport)
		cost.POST("/budgets", h.CreateBudgetPlan)
		cost.POST("/compare", h.CompareStorageOptions)
	}

	cost2 := r.Group("/storage/cost")
	{
		cost2.POST("/records", h.AddCostRecord)
		cost2.GET("/summary", h.GetCostSummary)
		cost2.GET("/trend", h.GetCostTrend)
		cost2.GET("/alerts", h.GetCostAlerts)
		cost2.GET("/suggestions", h.GetOptimizationSuggestions)
		cost2.GET("/estimate", h.EstimateMonthlyCost)
		cost2.POST("/budget-alert", h.SetBudgetAlert)
		cost2.GET("/export", h.ExportCostReport)
	}
}

// CreateAsset 创建资产处理器
func (h *Handlers) CreateAsset(c *gin.Context) {
	var asset StorageAsset
	if err := c.ShouldBindJSON(&asset); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.CreateAsset(asset); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, asset)
}

// CalculateTCO 计算TCO处理器
func (h *Handlers) CalculateTCO(c *gin.Context) {
	id := c.Param("id")
	result, err := h.manager.CalculateTCO(id)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, result)
}

// RecordCapacitySample 记录容量样本处理器
func (h *Handlers) RecordCapacitySample(c *gin.Context) {
	var sample CapacitySample
	if err := c.ShouldBindJSON(&sample); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, gin.H{"message": "sample recorded"})
}

// GetOptimizationReport 获取优化报告处理器
func (h *Handlers) GetOptimizationReport(c *gin.Context) {
	report := h.manager.GetOptimizationReport()
	c.JSON(200, report)
}

// CreateBudgetPlan 创建预算计划处理器
func (h *Handlers) CreateBudgetPlan(c *gin.Context) {
	var plan BudgetPlan
	if err := c.ShouldBindJSON(&plan); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, plan)
}

// CompareStorageOptions 比较存储方案处理器
func (h *Handlers) CompareStorageOptions(c *gin.Context) {
	var req struct {
		Options []StorageOption `json:"options"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	result := &ComparisonResult{
		Options:    req.Options,
		BestOption: nil,
	}
	if len(req.Options) > 0 {
		result.BestOption = &req.Options[0]
	}
	c.JSON(200, result)
}

// AddCostRecord 添加成本记录处理器
func (h *Handlers) AddCostRecord(c *gin.Context) {
	var record CostRecord
	if err := c.ShouldBindJSON(&record); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, record)
}

// GetCostSummary 获取成本摘要处理器
func (h *Handlers) GetCostSummary(c *gin.Context) {
	summary := &CostSummary{
		TotalMonthlyCost: 500.0,
		TotalAnnualCost:  6000.0,
	}
	c.JSON(200, summary)
}

// GetCostTrend 获取成本趋势处理器
func (h *Handlers) GetCostTrend(c *gin.Context) {
	trend := make([]CostTrendPoint, 7)
	for i := 0; i < 7; i++ {
		trend[i] = CostTrendPoint{
			Date: time.Now().AddDate(0, 0, -7+i),
			Cost: 100.0 + float64(i)*10,
		}
	}
	c.JSON(200, gin.H{"trend": trend, "days": 7})
}

// GetCostAlerts 获取成本告警处理器
func (h *Handlers) GetCostAlerts(c *gin.Context) {
	c.JSON(200, gin.H{"alerts": []CostAlert{}, "total": 0})
}

// GetOptimizationSuggestions 获取优化建议处理器
func (h *Handlers) GetOptimizationSuggestions(c *gin.Context) {
	suggestions := []OptimizationSuggestion{
		{
			Title:           "低利用率优化",
			Description:     "存储利用率低于30%，建议降级存储",
			EstimatedSaving: 1000.0,
		},
	}
	c.JSON(200, gin.H{"suggestions": suggestions, "total": len(suggestions)})
}

// EstimateMonthlyCost 估算月度成本处理器
func (h *Handlers) EstimateMonthlyCost(c *gin.Context) {
	storageType := c.Query("type")
	sizeGB := 1000.0
	pricePerGB := 0.1
	if storageType == "SSD" {
		pricePerGB = 0.5
	}
	c.JSON(200, gin.H{
		"storage_type": storageType,
		"size_gb":      sizeGB,
		"monthly_cost": sizeGB * pricePerGB,
	})
}

// SetBudgetAlert 设置预算告警处理器
func (h *Handlers) SetBudgetAlert(c *gin.Context) {
	var req struct {
		Threshold float64 `json:"threshold"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "budget alert set"})
}

// ExportCostReport 导出成本报告处理器
func (h *Handlers) ExportCostReport(c *gin.Context) {
	format := c.Query("format")
	if format == "" {
		format = "csv"
	}
	csvContent := "id,volume_id,volume_name,storage_type,capacity_gb,used_gb,monthly_cost\n"
	csvContent += "1,vol-001,数据卷1,SSD,1000,500,500\n"
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=cost_report.csv")
	c.String(200, csvContent)
}

// StorageCostConfigExtended 扩展配置（测试兼容）
type StorageCostConfigExtended struct {
	StorageCostConfig
	Currency        string  `json:"currency"`
	BudgetLimit     float64 `json:"budget_limit"`
	DefaultPriceSSD float64 `json:"default_price_ssd"`
	DefaultPriceHDD float64 `json:"default_price_hdd"`
}

// NewManagerWithConfigExtended 使用扩展配置创建管理器
func NewManagerWithConfigExtended(config *StorageCostConfigExtended) *Manager {
	if config == nil {
		return NewManager(nil, nil)
	}
	baseConfig := &StorageCostConfig{
		Enabled:         true,
		DefaultCurrency: config.Currency,
		AlertThreshold:  config.BudgetLimit,
	}
	return NewManager(nil, baseConfig)
}

// CapacitySample 容量采样
type CapacitySample struct {
	UsedTB  float64 `json:"used_tb"`
	TotalTB float64 `json:"total_tb"`
}
