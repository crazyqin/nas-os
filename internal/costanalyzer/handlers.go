package costanalyzer

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers 智能成本优化器 HTTP 处理器.
type Handlers struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{
		manager: manager,
		logger:  zap.NewNop(),
	}
}

// RegisterRoutes 注册路由到 /api/v1 下.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	sco := api.Group("/smart-cost")
	{
		// 资产管理
		sco.POST("/assets", h.addAsset)
		sco.GET("/assets", h.listAssets)
		sco.GET("/assets/:id", h.getAsset)
		sco.DELETE("/assets/:id", h.removeAsset)

		// 成本记录
		sco.POST("/costs", h.recordCost)
		sco.GET("/costs", h.listCosts)

		// 成本汇总与趋势
		sco.GET("/summary", h.getCostSummary)
		sco.GET("/trend", h.analyzeTrend)

		// 优化建议
		sco.GET("/optimizations", h.getOptimizations)
		sco.GET("/cold-data", h.getColdData)

		// ROI 计算
		sco.POST("/roi", h.calculateROI)

		// 报告
		sco.POST("/reports", h.generateReport)
		sco.GET("/reports", h.listReports)
		sco.GET("/reports/:id", h.getReport)
		sco.GET("/reports/:id/export", h.exportReport)

		// 配置
		sco.GET("/config", h.getConfig)
		sco.PUT("/config", h.updateConfig)
	}
}

// ============================================================
// 请求/响应结构
// ============================================================

type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type addAssetRequest struct {
	ID            string  `json:"id" binding:"required"`
	Name          string  `json:"name" binding:"required"`
	Type          string  `json:"type" binding:"required"`
	CapacityBytes int64   `json:"capacity_bytes" binding:"required"`
	UsedBytes     int64   `json:"used_bytes"`
	PurchaseCost  float64 `json:"purchase_cost"`
	MonthlyOpex   float64 `json:"monthly_opex"`
	WarrantyYears int     `json:"warranty_years"`
	PurchaseDate  string  `json:"purchase_date"`
	Pool          string  `json:"pool"`
	Volume        string  `json:"volume"`
	Provider      string  `json:"provider"`
}

type recordCostRequest struct {
	AssetID     string  `json:"asset_id" binding:"required"`
	StorageType string  `json:"storage_type"`
	CapacityGB  float64 `json:"capacity_gb"`
	UsedGB      float64 `json:"used_gb"`
	PricePerGB  float64 `json:"price_per_gb_month"`
	TotalCost   float64 `json:"total_cost"`
	PeriodStart string  `json:"period_start"`
	PeriodEnd   string  `json:"period_end"`
}

type roiRequest struct {
	InvestmentCost float64 `json:"investment_cost" binding:"required"`
	AnnualSaving   float64 `json:"annual_saving" binding:"required"`
	AnnualOpex     float64 `json:"annual_opex"`
	ProjectYears   int     `json:"project_years" binding:"required"`
	DiscountRate   float64 `json:"discount_rate"`
}

type reportRequest struct {
	ReportName  string `json:"report_name" binding:"required"`
	PeriodStart string `json:"period_start" binding:"required"`
	PeriodEnd   string `json:"period_end" binding:"required"`
}

type trendRequest struct {
	Granularity string `form:"granularity"`
	Months      int    `form:"months"`
}

type summaryRequest struct {
	PeriodStart string `form:"period_start"`
	PeriodEnd   string `form:"period_end"`
}

// ============================================================
// 资产管理 handlers
// ============================================================

func (h *Handlers) addAsset(c *gin.Context) {
	var req addAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	purchaseDate := time.Now()
	if req.PurchaseDate != "" {
		if t, err := time.Parse("2006-01-02", req.PurchaseDate); err == nil {
			purchaseDate = t
		}
	}

	asset := &StorageAsset{
		ID:            req.ID,
		Name:          req.Name,
		Type:          StorageType(req.Type),
		CapacityBytes: req.CapacityBytes,
		UsedBytes:     req.UsedBytes,
		PurchaseCost:  req.PurchaseCost,
		MonthlyOpex:   req.MonthlyOpex,
		WarrantyYears: req.WarrantyYears,
		PurchaseDate:  purchaseDate,
		Pool:          req.Pool,
		Volume:        req.Volume,
		Provider:      req.Provider,
	}

	if err := h.manager.AddAsset(asset); err != nil {
		c.JSON(http.StatusInternalServerError, apiResponse{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, apiResponse{Code: 0, Message: "asset added", Data: asset})
}

func (h *Handlers) listAssets(c *gin.Context) {
	assets := h.manager.ListAssets()
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: assets})
}

func (h *Handlers) getAsset(c *gin.Context) {
	id := c.Param("id")
	asset, err := h.manager.GetAsset(id)
	if err != nil {
		c.JSON(http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: asset})
}

func (h *Handlers) removeAsset(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RemoveAsset(id); err != nil {
		c.JSON(http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "asset removed"})
}

// ============================================================
// 成本记录 handlers
// ============================================================

func (h *Handlers) recordCost(c *gin.Context) {
	var req recordCostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	entry := &CostEntry{
		AssetID:     req.AssetID,
		StorageType: StorageType(req.StorageType),
		CapacityGB:  req.CapacityGB,
		UsedGB:      req.UsedGB,
		PricePerGB:  req.PricePerGB,
		TotalCost:   req.TotalCost,
	}

	if req.PeriodStart != "" {
		if t, err := time.Parse("2006-01-02", req.PeriodStart); err == nil {
			entry.PeriodStart = t
		}
	}
	if req.PeriodEnd != "" {
		if t, err := time.Parse("2006-01-02", req.PeriodEnd); err == nil {
			entry.PeriodEnd = t
		}
	}

	// 关联资产名
	if asset, err := h.manager.GetAsset(req.AssetID); err == nil {
		entry.AssetName = asset.Name
	}

	if err := h.manager.RecordCost(entry); err != nil {
		c.JSON(http.StatusInternalServerError, apiResponse{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, apiResponse{Code: 0, Message: "cost recorded", Data: entry})
}

func (h *Handlers) listCosts(c *gin.Context) {
	entries := h.manager.ListCostEntries()
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: entries})
}

// ============================================================
// 成本汇总与趋势 handlers
// ============================================================

func (h *Handlers) getCostSummary(c *gin.Context) {
	var req summaryRequest
	_ = c.ShouldBindQuery(&req)

	periodStart := time.Now().AddDate(0, -1, 0)
	periodEnd := time.Now()

	if req.PeriodStart != "" {
		if t, err := time.Parse("2006-01-02", req.PeriodStart); err == nil {
			periodStart = t
		}
	}
	if req.PeriodEnd != "" {
		if t, err := time.Parse("2006-01-02", req.PeriodEnd); err == nil {
			periodEnd = t
		}
	}

	summary := h.manager.GetCostSummary(periodStart, periodEnd)
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: summary})
}

func (h *Handlers) analyzeTrend(c *gin.Context) {
	var req trendRequest
	_ = c.ShouldBindQuery(&req)

	granularity := TrendMonthly
	if req.Granularity != "" {
		granularity = TrendGranularity(req.Granularity)
	}
	months := req.Months
	if months <= 0 {
		months = 6
	}

	trend := h.manager.AnalyzeTrend(granularity, months)
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: trend})
}

// ============================================================
// 优化建议 handlers
// ============================================================

func (h *Handlers) getOptimizations(c *gin.Context) {
	suggestions := h.manager.GenerateOptimizations()
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: suggestions})
}

func (h *Handlers) getColdData(c *gin.Context) {
	coldData := h.manager.DetectColdData()
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: coldData})
}

// ============================================================
// ROI handlers
// ============================================================

func (h *Handlers) calculateROI(c *gin.Context) {
	var req roiRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if req.DiscountRate == 0 {
		req.DiscountRate = 0.08 // 默认 8%
	}

	input := &ROIInput{
		InvestmentCost: req.InvestmentCost,
		AnnualSaving:   req.AnnualSaving,
		AnnualOpex:     req.AnnualOpex,
		ProjectYears:   req.ProjectYears,
		DiscountRate:   req.DiscountRate,
	}

	result, err := h.manager.CalculateROI(input)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: result})
}

// ============================================================
// 报告 handlers
// ============================================================

func (h *Handlers) generateReport(c *gin.Context) {
	var req reportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	periodStart, err := time.Parse("2006-01-02", req.PeriodStart)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{Code: 1, Message: "invalid period_start: " + err.Error()})
		return
	}
	periodEnd, err := time.Parse("2006-01-02", req.PeriodEnd)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{Code: 1, Message: "invalid period_end: " + err.Error()})
		return
	}

	report := h.manager.GenerateReport(req.ReportName, periodStart, periodEnd)
	c.JSON(http.StatusCreated, apiResponse{Code: 0, Message: "report generated", Data: report})
}

func (h *Handlers) listReports(c *gin.Context) {
	reports := h.manager.ListReports()
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: reports})
}

func (h *Handlers) getReport(c *gin.Context) {
	id := c.Param("id")
	report, err := h.manager.GetReport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: report})
}

func (h *Handlers) exportReport(c *gin.Context) {
	id := c.Param("id")
	format := c.Query("format")
	if format == "" {
		format = "csv"
	}

	if format == "csv" {
		csv, err := h.manager.ExportReportAsCSV(id)
		if err != nil {
			c.JSON(http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
			return
		}
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=smartcost_%s.csv", id))
		c.String(http.StatusOK, csv)
		return
	}

	// JSON 格式
	report, err := h.manager.GetReport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=smartcost_%s.json", id))
	c.JSON(http.StatusOK, report)
}

// ============================================================
// 配置 handlers
// ============================================================

func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: cfg})
}

func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg SmartCostConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	h.manager.UpdateConfig(&cfg)
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "config updated"})
}
