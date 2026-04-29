// Package costanalysis 提供存储成本分析功能
package costanalysis

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 存储成本分析 HTTP 处理器.
type Handlers struct {
	analyzer *Analyzer
}

// NewHandlers 创建处理器.
func NewHandlers(analyzer *Analyzer) *Handlers {
	return &Handlers{analyzer: analyzer}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	costGroup := api.Group("/cost-analysis")
	{
		// 存储池管理
		costGroup.GET("/pools", h.listPools)
		costGroup.POST("/pools", h.registerPool)
		costGroup.GET("/pools/:id", h.getPool)
		costGroup.DELETE("/pools/:id", h.removePool)

		// 成本分析
		costGroup.GET("/pools/:id/cost-per-tb", h.getCostPerTB)
		costGroup.GET("/pools/:id/roi", h.getROIAnalysis)
		costGroup.GET("/pools/:id/capacity-plan", h.getCapacityPlan)
		costGroup.GET("/pools/:id/optimization", h.getOptimization)

		// 历史数据
		costGroup.POST("/pools/:id/growth-data", h.addGrowthData)

		// 层级对比
		costGroup.GET("/tier-comparison", h.getTierComparison)
	}
}

// listPools 列出所有存储池.
func (h *Handlers) listPools(c *gin.Context) {
	pools := h.analyzer.ListPools()
	c.JSON(http.StatusOK, gin.H{
		"pools": pools,
		"total": len(pools),
	})
}

// registerPool 注册存储池.
func (h *Handlers) registerPool(c *gin.Context) {
	var pool StoragePool
	if err := c.ShouldBindJSON(&pool); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if pool.ID == "" {
		pool.ID = "pool-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	if pool.CreatedAt.IsZero() {
		pool.CreatedAt = time.Now()
	}

	h.analyzer.RegisterPool(&pool)
	c.JSON(http.StatusCreated, gin.H{
		"message": "存储池注册成功",
		"pool":    pool,
	})
}

// getPool 获取存储池信息.
func (h *Handlers) getPool(c *gin.Context) {
	poolID := c.Param("id")
	pool, err := h.analyzer.GetPool(poolID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pool)
}

// removePool 移除存储池.
func (h *Handlers) removePool(c *gin.Context) {
	poolID := c.Param("id")
	h.analyzer.RemovePool(poolID)
	c.JSON(http.StatusOK, gin.H{"message": "存储池已移除", "pool_id": poolID})
}

// getCostPerTB 获取每TB成本.
func (h *Handlers) getCostPerTB(c *gin.Context) {
	poolID := c.Param("id")
	result, err := h.analyzer.CalculateCostPerTB(poolID)
	if err != nil {
		code := http.StatusInternalServerError
		if err == ErrPoolNotFound {
			code = http.StatusNotFound
		} else if err == ErrInvalidInput {
			code = http.StatusBadRequest
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// getTierComparison 获取层级成本对比.
func (h *Handlers) getTierComparison(c *gin.Context) {
	result := h.analyzer.CompareTiers()
	c.JSON(http.StatusOK, result)
}

// getCapacityPlan 获取容量规划建议.
func (h *Handlers) getCapacityPlan(c *gin.Context) {
	poolID := c.Param("id")
	months := 12
	if m, err := strconv.Atoi(c.DefaultQuery("months", "12")); err == nil && m > 0 {
		months = m
	}

	result, err := h.analyzer.GenerateCapacityPlan(poolID, months)
	if err != nil {
		code := http.StatusInternalServerError
		if err == ErrPoolNotFound {
			code = http.StatusNotFound
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// getROIAnalysis 获取ROI分析.
func (h *Handlers) getROIAnalysis(c *gin.Context) {
	poolID := c.Param("id")
	years := 0.0
	if y, err := strconv.ParseFloat(c.DefaultQuery("years", "0"), 64); err == nil {
		years = y
	}

	result, err := h.analyzer.AnalyzeROI(poolID, years)
	if err != nil {
		code := http.StatusInternalServerError
		if err == ErrPoolNotFound {
			code = http.StatusNotFound
		} else if err == ErrInvalidInput {
			code = http.StatusBadRequest
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// getOptimization 获取成本优化建议.
func (h *Handlers) getOptimization(c *gin.Context) {
	poolID := c.Param("id")
	result, err := h.analyzer.GenerateOptimizationReport(poolID)
	if err != nil {
		code := http.StatusInternalServerError
		if err == ErrPoolNotFound {
			code = http.StatusNotFound
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// addGrowthData 添加历史增长数据.
func (h *Handlers) addGrowthData(c *gin.Context) {
	poolID := c.Param("id")

	var point GrowthDataPoint
	if err := c.ShouldBindJSON(&point); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if point.Timestamp.IsZero() {
		point.Timestamp = time.Now()
	}

	h.analyzer.AddGrowthData(poolID, point)
	c.JSON(http.StatusCreated, gin.H{
		"message": "增长数据已添加",
		"pool_id": poolID,
	})
}
