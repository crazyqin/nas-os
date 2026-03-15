package project

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// BillingHandlers 计费管理 HTTP 处理器
type BillingHandlers struct {
	billingManager *BillingManager
	projectManager *Manager
}

// NewBillingHandlers 创建计费处理器
func NewBillingHandlers(billingMgr *BillingManager, projectMgr *Manager) *BillingHandlers {
	return &BillingHandlers{
		billingManager: billingMgr,
		projectManager: projectMgr,
	}
}

// RegisterBillingRoutes 注册计费路由
func (h *BillingHandlers) RegisterBillingRoutes(router *gin.RouterGroup) {
	// ========== 项目配额管理 ==========
	projects := router.Group("/projects")
	{
		projects.GET("/:id/quotas", h.listQuotas)
		projects.POST("/:id/quotas", h.setQuota)
		projects.GET("/:id/quotas/:resource_type", h.getQuota)
		projects.DELETE("/:id/quotas/:resource_type", h.deleteQuota)

		// ========== 资源分配 ==========
		projects.POST("/:id/resources/allocate", h.allocateResource)
		projects.POST("/:id/resources/release", h.releaseResource)
		projects.GET("/:id/resources/usage", h.getQuotaUsage)

		// ========== 计费记录 ==========
		projects.GET("/:id/billing", h.listBillingRecords)
		projects.POST("/:id/billing", h.createBillingRecord)
		projects.GET("/:id/billing/:billing_id", h.getBillingRecord)
		projects.PUT("/:id/billing/:billing_id/status", h.updateBillingStatus)

		// ========== 成本分析 ==========
		projects.GET("/:id/cost-analysis", h.getCostAnalysis)

		// ========== 资源使用报告 ==========
		projects.GET("/:id/usage-report", h.getResourceUsageReport)
	}
}

// ========== 请求结构 ==========

// SetQuotaRequest 设置配额请求
type SetQuotaRequest struct {
	ResourceType string  `json:"resource_type" binding:"required"`
	HardLimit    int64   `json:"hard_limit" binding:"required"`
	SoftLimit    int64   `json:"soft_limit"`
	UnitPrice    float64 `json:"unit_price"`
	Currency     string  `json:"currency"`
}

// AllocateResourceRequest 资源分配请求
type AllocateResourceRequest struct {
	ResourceType string `json:"resource_type" binding:"required"`
	Amount       int64  `json:"amount" binding:"required"`
	Description  string `json:"description,omitempty"`
}

// ReleaseResourceRequest 资源释放请求
type ReleaseResourceRequest struct {
	ResourceType string `json:"resource_type" binding:"required"`
	Amount       int64  `json:"amount" binding:"required"`
	Description  string `json:"description,omitempty"`
}

// CreateBillingRecordRequest 创建计费记录请求
type CreateBillingRecordRequest struct {
	PeriodStart  time.Time     `json:"period_start" binding:"required"`
	PeriodEnd    time.Time     `json:"period_end" binding:"required"`
	ResourceCosts []ResourceCost `json:"resource_costs" binding:"required"`
	Discount     float64       `json:"discount"`
	TaxRate      float64       `json:"tax_rate"`
}

// UpdateBillingStatusRequest 更新计费状态请求
type UpdateBillingStatusRequest struct {
	Status string `json:"status" binding:"required"`
	PaidAt *time.Time `json:"paid_at,omitempty"`
}

// CostAnalysisRequest 成本分析请求参数
type CostAnalysisQuery struct {
	PeriodStart string `form:"period_start"`
	PeriodEnd   string `form:"period_end"`
}

// UsageReportRequest 资源使用报告请求参数
type UsageReportQuery struct {
	PeriodStart string `form:"period_start"`
	PeriodEnd   string `form:"period_end"`
}

// ========== 配额管理 API ==========

// listQuotas 列出项目配额
// @Summary 列出项目资源配额
// @Description 获取项目的所有资源配额配置
// @Tags billing
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Success 200 {object} map[string]interface{}
// @Router /projects/{id}/quotas [get]
func (h *BillingHandlers) listQuotas(c *gin.Context) {
	projectID := c.Param("id")

	quotas := h.billingManager.ListQuotas(projectID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    quotas,
	})
}

// setQuota 设置资源配额
// @Summary 设置资源配额
// @Description 为项目设置或更新资源配额
// @Tags billing
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Param request body SetQuotaRequest true "配额设置参数"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /projects/{id}/quotas [post]
func (h *BillingHandlers) setQuota(c *gin.Context) {
	projectID := c.Param("id")

	var req SetQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	currency := req.Currency
	if currency == "" {
		currency = "CNY"
	}

	quota, err := h.billingManager.SetQuota(
		projectID,
		ResourceType(req.ResourceType),
		req.HardLimit,
		req.SoftLimit,
		req.UnitPrice,
		currency,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    quota,
	})
}

// getQuota 获取指定资源配额
// @Summary 获取指定资源配额
// @Description 获取项目指定资源类型的配额配置
// @Tags billing
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Param resource_type path string true "资源类型"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /projects/{id}/quotas/{resource_type} [get]
func (h *BillingHandlers) getQuota(c *gin.Context) {
	projectID := c.Param("id")
	resourceType := c.Param("resource_type")

	quota, err := h.billingManager.GetQuota(projectID, ResourceType(resourceType))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    quota,
	})
}

// deleteQuota 删除资源配额
// @Summary 删除资源配额
// @Description 删除项目指定资源类型的配额配置
// @Tags billing
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Param resource_type path string true "资源类型"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /projects/{id}/quotas/{resource_type} [delete]
func (h *BillingHandlers) deleteQuota(c *gin.Context) {
	projectID := c.Param("id")
	resourceType := c.Param("resource_type")

	if err := h.billingManager.DeleteQuota(projectID, ResourceType(resourceType)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// ========== 资源分配 API ==========

// allocateResource 分配资源
// @Summary 分配资源
// @Description 为项目分配资源（计入配额使用）
// @Tags billing
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Param request body AllocateResourceRequest true "资源分配参数"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /projects/{id}/resources/allocate [post]
func (h *BillingHandlers) allocateResource(c *gin.Context) {
	projectID := c.Param("id")

	var req AllocateResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	uid, _ := userID.(string)
	if uid == "" {
		uid = "system"
	}

	if err := h.billingManager.AllocateResource(
		projectID,
		ResourceType(req.ResourceType),
		req.Amount,
		uid,
		req.Description,
	); err != nil {
		if err == ErrQuotaExceeded {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// releaseResource 释放资源
// @Summary 释放资源
// @Description 释放项目资源（减少配额使用）
// @Tags billing
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Param request body ReleaseResourceRequest true "资源释放参数"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /projects/{id}/resources/release [post]
func (h *BillingHandlers) releaseResource(c *gin.Context) {
	projectID := c.Param("id")

	var req ReleaseResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	uid, _ := userID.(string)
	if uid == "" {
		uid = "system"
	}

	if err := h.billingManager.ReleaseResource(
		projectID,
		ResourceType(req.ResourceType),
		req.Amount,
		uid,
		req.Description,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// getQuotaUsage 获取配额使用记录
// @Summary 获取配额使用记录
// @Description 获取项目的资源使用历史记录
// @Tags billing
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Param limit query int false "限制数量" default(50)
// @Success 200 {object} map[string]interface{}
// @Router /projects/{id}/resources/usage [get]
func (h *BillingHandlers) getQuotaUsage(c *gin.Context) {
	projectID := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	usage := h.billingManager.GetQuotaUsage(projectID, limit)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    usage,
	})
}

// ========== 计费记录 API ==========

// listBillingRecords 列出计费记录
// @Summary 列出计费记录
// @Description 获取项目的计费记录列表
// @Tags billing
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Param limit query int false "限制数量" default(20)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} map[string]interface{}
// @Router /projects/{id}/billing [get]
func (h *BillingHandlers) listBillingRecords(c *gin.Context) {
	projectID := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	records := h.billingManager.ListBillingRecords(projectID, limit, offset)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    records,
	})
}

// createBillingRecord 创建计费记录
// @Summary 创建计费记录
// @Description 为项目创建新的计费记录
// @Tags billing
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Param request body CreateBillingRecordRequest true "计费记录参数"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /projects/{id}/billing [post]
func (h *BillingHandlers) createBillingRecord(c *gin.Context) {
	projectID := c.Param("id")

	var req CreateBillingRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	record, err := h.billingManager.CreateBillingRecord(
		projectID,
		req.PeriodStart,
		req.PeriodEnd,
		req.ResourceCosts,
		req.Discount,
		req.TaxRate,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    record,
	})
}

// getBillingRecord 获取计费记录详情
// @Summary 获取计费记录详情
// @Description 根据ID获取计费记录详情
// @Tags billing
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Param billing_id path string true "计费记录ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /projects/{id}/billing/{billing_id} [get]
func (h *BillingHandlers) getBillingRecord(c *gin.Context) {
	billingID := c.Param("billing_id")

	record, err := h.billingManager.GetBillingRecord(billingID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    record,
	})
}

// updateBillingStatus 更新计费状态
// @Summary 更新计费状态
// @Description 更新计费记录的支付状态
// @Tags billing
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Param billing_id path string true "计费记录ID"
// @Param request body UpdateBillingStatusRequest true "状态更新参数"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /projects/{id}/billing/{billing_id}/status [put]
func (h *BillingHandlers) updateBillingStatus(c *gin.Context) {
	billingID := c.Param("billing_id")

	var req UpdateBillingStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.billingManager.UpdateBillingStatus(billingID, req.Status, req.PaidAt); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// ========== 成本分析 API ==========

// getCostAnalysis 获取成本分析
// @Summary 获取成本分析
// @Description 获取项目的成本分析报告
// @Tags billing
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Param period_start query string false "开始日期 (YYYY-MM-DD)"
// @Param period_end query string false "结束日期 (YYYY-MM-DD)"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /projects/{id}/cost-analysis [get]
func (h *BillingHandlers) getCostAnalysis(c *gin.Context) {
	projectID := c.Param("id")

	// 解析时间参数
	periodStartStr := c.DefaultQuery("period_start", "")
	periodEndStr := c.DefaultQuery("period_end", "")

	var periodStart, periodEnd time.Time
	var err error

	if periodStartStr != "" {
		periodStart, err = time.Parse("2006-01-02", periodStartStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid period_start format"})
			return
		}
	} else {
		// 默认最近30天
		periodStart = time.Now().AddDate(0, 0, -30)
	}

	if periodEndStr != "" {
		periodEnd, err = time.Parse("2006-01-02", periodEndStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid period_end format"})
			return
		}
	} else {
		periodEnd = time.Now()
	}

	analysis, err := h.billingManager.GetCostAnalysis(projectID, periodStart, periodEnd)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    analysis,
	})
}

// ========== 资源使用报告 API ==========

// getResourceUsageReport 获取资源使用报告
// @Summary 获取资源使用报告
// @Description 获取项目的资源使用报告
// @Tags billing
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Param period_start query string false "开始日期 (YYYY-MM-DD)"
// @Param period_end query string false "结束日期 (YYYY-MM-DD)"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /projects/{id}/usage-report [get]
func (h *BillingHandlers) getResourceUsageReport(c *gin.Context) {
	projectID := c.Param("id")

	// 解析时间参数
	periodStartStr := c.DefaultQuery("period_start", "")
	periodEndStr := c.DefaultQuery("period_end", "")

	var periodStart, periodEnd time.Time
	var err error

	if periodStartStr != "" {
		periodStart, err = time.Parse("2006-01-02", periodStartStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid period_start format"})
			return
		}
	} else {
		periodStart = time.Now().AddDate(0, 0, -7)
	}

	if periodEndStr != "" {
		periodEnd, err = time.Parse("2006-01-02", periodEndStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid period_end format"})
			return
		}
	} else {
		periodEnd = time.Now()
	}

	report, err := h.billingManager.GetResourceUsageReport(projectID, periodStart, periodEnd)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    report,
	})
}