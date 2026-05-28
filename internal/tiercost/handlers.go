// Package tiercost 提供存储分层成本分析功能
package tiercost

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 存储分层成本分析 HTTP 处理器.
type Handlers struct {
	analyzer *TierCostAnalyzer
	logger   *slog.Logger
}

// NewHandlers 创建处理器.
func NewHandlers(analyzer *TierCostAnalyzer) *Handlers {
	return &Handlers{
		analyzer: analyzer,
		logger:   slog.Default(),
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	tierCostGroup := api.Group("/tier-cost")
	{
		// 分层成本分析
		tierCostGroup.GET("/analysis", h.getAnalysis)

		// 分层建议
		tierCostGroup.GET("/recommendations", h.getRecommendations)

		// 成本趋势
		tierCostGroup.GET("/trends", h.getCostTrends)

		// 模拟分层方案
		tierCostGroup.POST("/simulate", h.simulatePlan)

		// 更新存储单价
		tierCostGroup.PUT("/pricing", h.updatePricing)
	}
}

// getAnalysis 获取分层成本分析.
func (h *Handlers) getAnalysis(c *gin.Context) {
	report := h.analyzer.AnalyzeCost()
	c.JSON(http.StatusOK, report)
}

// getRecommendations 获取分层建议.
func (h *Handlers) getRecommendations(c *gin.Context) {
	recommendations, savings := h.analyzer.GetRecommendations()
	c.JSON(http.StatusOK, gin.H{
		"recommendations":   recommendations,
		"total_savings":     savings,
		"total_count":       len(recommendations),
	})
}

// getCostTrends 获取成本趋势.
func (h *Handlers) getCostTrends(c *gin.Context) {
	months := 12
	if m, err := parseIntQuery(c, "months"); err == nil && m > 0 {
		months = m
	}

	trends := h.analyzer.GetCostTrends(months)
	c.JSON(http.StatusOK, gin.H{
		"trends": trends,
		"total":  len(trends),
	})
}

// simulatePlan 模拟分层方案.
func (h *Handlers) simulatePlan(c *gin.Context) {
	var req SimulateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	result, err := h.analyzer.SimulateTierPlan(&req)
	if err != nil {
		code := http.StatusInternalServerError
		if err == ErrInvalidInput {
			code = http.StatusBadRequest
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// updatePricing 更新存储单价.
func (h *Handlers) updatePricing(c *gin.Context) {
	var req PricingUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.analyzer.UpdatePricing(&req); err != nil {
		code := http.StatusInternalServerError
		if err == ErrInvalidPricing {
			code = http.StatusBadRequest
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	pricing := h.analyzer.GetPricing()
	c.JSON(http.StatusOK, gin.H{
		"message": "存储单价已更新",
		"pricing": gin.H{
			"nvme_price_per_tb_year": pricing.NVMePricePerTBYear,
			"ssd_price_per_tb_year":  pricing.SSDPricePerTBYear,
			"hdd_price_per_tb_year":  pricing.HDDPricePerTBYear,
		},
	})
}

// parseIntQuery 解析整数查询参数.
func parseIntQuery(c *gin.Context, key string) (int, error) {
	val := c.DefaultQuery(key, "0")
	var result int
	_, err := fmt.Sscanf(val, "%d", &result)
	return result, err
}
