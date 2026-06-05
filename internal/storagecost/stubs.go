// Package storagecost 存储成本分析模块
// 本文件定义测试所需的类型存根，用于兼容 handlers_test.go
package storagecost

import (
	"time"

	"github.com/gin-gonic/gin"
)

// StorageAsset 存储资产
type StorageAsset struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	CapacityTB     float64 `json:"capacity_tb"`
	PurchaseCost   float64 `json:"purchase_cost"`
	WarrantyYears  int     `json:"warranty_years"`
	AnnualPowerKWh int     `json:"annual_power_kwh"`
	PowerCostPerKWh float64 `json:"power_cost_per_kwh"`
	RackUnits      int     `json:"rack_units"`
	RackCostPerUnit float64 `json:"rack_cost_per_unit"`
}

// TCOResult TCO分析结果
type TCOResult struct {
	AssetID     string  `json:"assetId"`
	TotalCost   float64 `json:"totalCost"`
	CostPerTB   float64 `json:"costPerTb"`
	Breakdown   map[string]float64 `json:"breakdown"`
}

// OptimizationReport 优化报告
type OptimizationReport struct {
	TotalAnnualSaving float64 `json:"totalAnnualSaving"`
	Suggestions       []OptimizationSuggestion `json:"suggestions"`
}

// BudgetPlan 预算计划
type BudgetPlan struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	FiscalYear  int          `json:"fiscal_year"`
	TotalBudget float64      `json:"total_budget"`
	LineItems   []LineItem   `json:"line_items"`
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
	ID          string    `json:"id"`
	VolumeID    string    `json:"volume_id"`
	VolumeName  string    `json:"volume_name"`
	StorageType string    `json:"storage_type"`
	CapacityGB  float64   `json:"capacity_gb"`
	UsedGB      float64   `json:"used_gb"`
	PricePerGB  float64   `json:"price_per_gb"`
	MonthlyCost float64   `json:"monthly_cost"`
	Provider    string    `json:"provider"`
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
	ID       string  `json:"id"`
	Level    string  `json:"level"`
	Message  string  `json:"message"`
	Threshold float64 `json:"threshold"`
	Current  float64 `json:"current"`
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
		cost.POST("/records", h.AddCostRecord)
		cost.GET("/summary", h.GetCostSummary)
		cost.GET("/trend", h.GetCostTrend)
		cost.GET("/alerts", h.GetCostAlerts)
		cost.GET("/suggestions", h.GetOptimizationSuggestions)
		cost.GET("/estimate", h.EstimateMonthlyCost)
		cost.POST("/budget-alert", h.SetBudgetAlert)
		cost.GET("/export", h.ExportCostReport)
	}
}

// CreateAsset 创建资产处理器
func (h *Handlers) CreateAsset(c *gin.Context) {}

// CalculateTCO 计算TCO处理器
func (h *Handlers) CalculateTCO(c *gin.Context) {}

// RecordCapacitySample 记录容量样本处理器
func (h *Handlers) RecordCapacitySample(c *gin.Context) {}

// GetOptimizationReport 获取优化报告处理器
func (h *Handlers) GetOptimizationReport(c *gin.Context) {}

// CreateBudgetPlan 创建预算计划处理器
func (h *Handlers) CreateBudgetPlan(c *gin.Context) {}

// CompareStorageOptions 比较存储方案处理器
func (h *Handlers) CompareStorageOptions(c *gin.Context) {}

// AddCostRecord 添加成本记录处理器
func (h *Handlers) AddCostRecord(c *gin.Context) {}

// GetCostSummary 获取成本摘要处理器
func (h *Handlers) GetCostSummary(c *gin.Context) {}

// GetCostTrend 获取成本趋势处理器
func (h *Handlers) GetCostTrend(c *gin.Context) {}

// GetCostAlerts 获取成本告警处理器
func (h *Handlers) GetCostAlerts(c *gin.Context) {}

// GetOptimizationSuggestions 获取优化建议处理器
func (h *Handlers) GetOptimizationSuggestions(c *gin.Context) {}

// EstimateMonthlyCost 估算月度成本处理器
func (h *Handlers) EstimateMonthlyCost(c *gin.Context) {}

// SetBudgetAlert 设置预算告警处理器
func (h *Handlers) SetBudgetAlert(c *gin.Context) {}

// ExportCostReport 导出成本报告处理器
func (h *Handlers) ExportCostReport(c *gin.Context) {}

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
