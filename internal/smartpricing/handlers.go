// Package smartpricing - 智能定价分析 HTTP API 处理器
package smartpricing

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 智能定价分析 HTTP 处理器.
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建新的 HTTP 处理器.
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes 注册 API 路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	smartpricing := rg.Group("/smartpricing")
	{
		smartpricing.GET("/plans", h.GetPlans)
		smartpricing.POST("/compare", h.CompareCost)
		smartpricing.GET("/recommendations", h.GetRecommendations)
		smartpricing.GET("/trends", h.GetCostTrends)
	}
}

// GetPlans 获取存储方案列表
// GET /api/smartpricing/plans.
func (h *Handler) GetPlans(c *gin.Context) {
	h.logger.Info("Getting storage plans")

	plans := h.manager.GetPlans()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"plans":        plans,
			"total":        len(plans),
			"generated_at": time.Now(),
		},
	})
}

// CompareCost 成本对比
// POST /api/smartpricing/compare.
func (h *Handler) CompareCost(c *gin.Context) {
	h.logger.Info("Comparing storage costs")

	var req CostCompareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.CompareCost(req)
	if err != nil {
		h.logger.Error("Failed to compare costs", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to compare costs: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// GetRecommendations 获取优化建议
// GET /api/smartpricing/recommendations.
func (h *Handler) GetRecommendations(c *gin.Context) {
	h.logger.Info("Getting optimization recommendations")

	// 从查询参数获取参数
	storageGB := 1000.0 // 默认1TB
	if v := c.Query("storage_gb"); v != "" {
		if _, err := fmt.Sscanf(v, "%f", &storageGB); err != nil {
			storageGB = 1000.0
		}
	}

	currentProvider := StorageProvider(c.DefaultQuery("provider", "aws_s3"))
	currentTier := StorageTier(c.DefaultQuery("tier", "standard"))
	monthlyCost := 0.0
	if v := c.Query("monthly_cost"); v != "" {
		if _, err := fmt.Sscanf(v, "%f", &monthlyCost); err != nil {
			monthlyCost = 0.0
		}
	}

	result, err := h.manager.GetRecommendations(storageGB, currentProvider, currentTier, monthlyCost)
	if err != nil {
		h.logger.Error("Failed to get recommendations", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get recommendations: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// GetCostTrends 获取成本趋势
// GET /api/smartpricing/trends.
func (h *Handler) GetCostTrends(c *gin.Context) {
	h.logger.Info("Getting cost trends")

	var req CostTrendRequest

	// 从查询参数获取
	provider := c.Query("provider")
	if provider != "" {
		req.Provider = StorageProvider(provider)
	}

	tier := c.Query("tier")
	if tier != "" {
		req.Tier = StorageTier(tier)
	}

	// 默认最近12个月
	now := time.Now()
	req.EndDate = now
	req.StartDate = now.AddDate(-1, 0, 0)

	if v := c.Query("start_date"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			req.StartDate = t
		}
	}

	if v := c.Query("end_date"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			req.EndDate = t
		}
	}

	req.Interval = c.DefaultQuery("interval", "monthly")

	result, err := h.manager.GetCostTrends(req)
	if err != nil {
		h.logger.Error("Failed to get cost trends", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get cost trends: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}
