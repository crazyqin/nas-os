// Package smartpricing - 智能定价引擎 REST API 处理器
package smartpricing

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PricingHandler 智能定价 API 处理器.
type PricingHandler struct {
	manager *PricingManager
	logger  *zap.Logger
}

// NewPricingHandler 创建定价处理器.
func NewPricingHandler(manager *PricingManager, logger *zap.Logger) *PricingHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &PricingHandler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterPricingRoutes 注册定价路由.
func (h *PricingHandler) RegisterPricingRoutes(rg *gin.RouterGroup) {
	pricing := rg.Group("/pricing")
	{
		// 价格计算
		pricing.POST("/calculate", h.calculatePrice)

		// 使用量查询
		pricing.GET("/usage/:userID", h.getUsageMetrics)

		// 折扣管理
		pricing.POST("/discount", h.applyDiscount)

		// 发票管理
		pricing.POST("/invoice", h.generateInvoice)
		pricing.GET("/invoice/:invoiceID", h.getInvoice)
		pricing.GET("/invoices/:userID", h.getUserInvoices)

		// 定价信息
		pricing.GET("/tiers", h.getTiers)
		pricing.GET("/plans", h.getPlans)
		pricing.GET("/rules", h.getPriceRules)
	}
}

// calculatePrice 计算价格
// POST /api/v1/pricing/calculate.
func (h *PricingHandler) calculatePrice(c *gin.Context) {
	var req struct {
		UserID       string  `json:"user_id" binding:"required"`
		ResourceType string  `json:"resource_type" binding:"required"`
		Usage        float64 `json:"usage" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}

	result, err := h.manager.CalculatePrice(req.UserID, req.ResourceType, req.Usage)
	if err != nil {
		h.logger.Error("Failed to calculate price", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "计算价格失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// getUsageMetrics 获取使用量指标
// GET /api/v1/pricing/usage/:userID.
func (h *PricingHandler) getUsageMetrics(c *gin.Context) {
	userID := c.Param("userID")
	resourceType := c.Query("resource_type")

	var startTime, endTime time.Time
	if v := c.Query("start_time"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			startTime = t
		}
	}
	if v := c.Query("end_time"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			endTime = t
		}
	}

	metrics, err := h.manager.GetUsageMetrics(userID, resourceType, startTime, endTime)
	if err != nil {
		h.logger.Error("Failed to get usage metrics", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "获取使用量失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"metrics": metrics,
			"total":   len(metrics),
		},
	})
}

// applyDiscount 应用折扣
// POST /api/v1/pricing/discount.
func (h *PricingHandler) applyDiscount(c *gin.Context) {
	var req struct {
		UserID string  `json:"user_id" binding:"required"`
		Type   string  `json:"type" binding:"required"`
		Value  float64 `json:"value" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}

	result, err := h.manager.ApplyDiscount(req.UserID, req.Type, req.Value)
	if err != nil {
		h.logger.Error("Failed to apply discount", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "应用折扣失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// generateInvoice 生成发票
// POST /api/v1/pricing/invoice.
func (h *PricingHandler) generateInvoice(c *gin.Context) {
	var req struct {
		UserID      string `json:"user_id" binding:"required"`
		PeriodStart string `json:"period_start" binding:"required"`
		PeriodEnd   string `json:"period_end" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}

	periodStart, err := time.Parse(time.RFC3339, req.PeriodStart)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "无效的开始日期: " + err.Error(),
		})
		return
	}

	periodEnd, err := time.Parse(time.RFC3339, req.PeriodEnd)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "无效的结束日期: " + err.Error(),
		})
		return
	}

	invoice, err := h.manager.GenerateInvoice(req.UserID, periodStart, periodEnd)
	if err != nil {
		h.logger.Error("Failed to generate invoice", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "生成发票失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    invoice,
	})
}

// getInvoice 获取发票
// GET /api/v1/pricing/invoice/:invoiceID.
func (h *PricingHandler) getInvoice(c *gin.Context) {
	invoiceID := c.Param("invoiceID")

	invoice, err := h.manager.GetInvoice(invoiceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "发票未找到: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    invoice,
	})
}

// getUserInvoices 获取用户发票列表
// GET /api/v1/pricing/invoices/:userID.
func (h *PricingHandler) getUserInvoices(c *gin.Context) {
	userID := c.Param("userID")

	invoices := h.manager.GetUserInvoices(userID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"invoices": invoices,
			"total":    len(invoices),
		},
	})
}

// getTiers 获取定价层级
// GET /api/v1/pricing/tiers.
func (h *PricingHandler) getTiers(c *gin.Context) {
	tiers := h.manager.GetTiers()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"tiers": tiers,
			"total": len(tiers),
		},
	})
}

// getPlans 获取计费方案
// GET /api/v1/pricing/plans.
func (h *PricingHandler) getPlans(c *gin.Context) {
	plans := h.manager.GetPlans()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"plans": plans,
			"total": len(plans),
		},
	})
}

// getPriceRules 获取价格规则
// GET /api/v1/pricing/rules.
func (h *PricingHandler) getPriceRules(c *gin.Context) {
	rules := h.manager.GetPriceRules()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"rules": rules,
			"total": len(rules),
		},
	})
}
