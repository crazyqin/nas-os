package smarthealthscore

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 智能健康评分 HTTP 处理器。
type Handlers struct {
	scorer *Scorer
}

// NewHandlers 创建处理器。
func NewHandlers(scorer *Scorer) *Handlers {
	return &Handlers{scorer: scorer}
}

// RegisterRoutes 注册路由到指定路由组。
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	health := api.Group("/healthscore")
	{
		// GET /api/healthscore - 获取综合健康评分
		health.GET("", h.GetHealthScore)
		// GET /api/healthscore/trends - 获取健康趋势
		health.GET("/trends", h.GetTrends)
		// GET /api/healthscore/alerts - 获取告警记录
		health.GET("/alerts", h.GetAlerts)
		// GET /api/healthscore/components - 获取各维度评分
		health.GET("/components", h.GetComponents)
	}
}

// GetHealthScore 获取综合健康评分。
// @Summary 获取综合健康评分
// @Description 计算并返回系统综合健康评分，涵盖磁盘、网络、安全、性能、可用性五大维度
// @Tags healthscore
// @Accept json
// @Produce json
// @Success 200 {object} HealthScore
// @Router /api/healthscore [get]
func (h *Handlers) GetHealthScore(c *gin.Context) {
	score, err := h.scorer.CalculateOverallScore()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "评分计算失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    score,
	})
}

// GetTrends 获取健康趋势。
// @Summary 获取健康趋势
// @Description 获取系统健康评分的历史趋势数据，支持按天数和维度过滤
// @Tags healthscore
// @Accept json
// @Produce json
// @Param days query int false "查询最近N天" default(30)
// @Param limit query int false "最大条数" default(100)
// @Param category query string false "过滤特定维度(disk/network/security/performance/availability)"
// @Success 200 {object} TrendResponse
// @Router /api/healthscore/trends [get]
func (h *Handlers) GetTrends(c *gin.Context) {
	var query TrendQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "参数错误",
			"message": err.Error(),
		})
		return
	}

	// 验证category参数
	if query.Category != "" && !isValidCategory(query.Category) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "无效的维度类别",
			"message": "category必须是: disk, network, security, performance, availability",
		})
		return
	}

	trends := h.scorer.GetTrends(query)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    trends,
	})
}

// GetAlerts 获取告警记录。
// @Summary 获取告警记录
// @Description 获取系统健康评分的告警记录，支持按天数和维度过滤
// @Tags healthscore
// @Accept json
// @Produce json
// @Param days query int false "查询最近N天" default(30)
// @Param limit query int false "最大条数" default(50)
// @Param category query string false "过滤特定维度(disk/network/security/performance/availability)"
// @Success 200 {object} map[string]interface{}
// @Router /api/healthscore/alerts [get]
func (h *Handlers) GetAlerts(c *gin.Context) {
	var query AlertQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "参数错误",
			"message": err.Error(),
		})
		return
	}

	// 验证category参数
	if query.Category != "" && !isValidCategory(query.Category) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "无效的维度类别",
			"message": "category必须是: disk, network, security, performance, availability",
		})
		return
	}

	alerts := h.scorer.GetAlerts(query)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"alerts":      alerts,
			"total_count": len(alerts),
		},
	})
}

// GetComponents 获取各维度独立评分。
// @Summary 获取各维度独立评分
// @Description 获取磁盘、网络、安全、性能、可用性各维度的独立评分详情
// @Tags healthscore
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/healthscore/components [get]
func (h *Handlers) GetComponents(c *gin.Context) {
	components, err := h.scorer.GetComponents()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "获取维度评分失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"components": components,
			"config":     h.scorer.GetConfig(),
		},
	})
}

// isValidCategory 验证是否为有效的评分维度。
func isValidCategory(cat ScoreCategory) bool {
	switch cat {
	case CategoryDisk, CategoryNetwork, CategorySecurity, CategoryPerformance, CategoryAvailability:
		return true
	default:
		return false
	}
}
