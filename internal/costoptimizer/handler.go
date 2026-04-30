package costoptimizer

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 成本优化HTTP处理器
type Handler struct {
	optimizer *CostOptimizer
}

// NewHandler 创建处理器
func NewHandler(optimizer *CostOptimizer) *Handler {
	return &Handler{optimizer: optimizer}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	cost := rg.Group("/cost-optimizer")
	{
		cost.GET("/report", h.GetReport)
		cost.POST("/allocations", h.SetAllocations)
		cost.GET("/profiles", h.GetProfiles)
		cost.GET("/suggestions", h.GetSuggestions)
	}
}

// GetReport 获取成本报告
func (h *Handler) GetReport(c *gin.Context) {
	report := h.optimizer.GenerateReport()
	c.JSON(http.StatusOK, report)
}

// SetAllocations 设置存储分配
func (h *Handler) SetAllocations(c *gin.Context) {
	var allocs []StorageAllocation
	if err := c.ShouldBindJSON(&allocs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.optimizer.SetAllocations(allocs)
	c.JSON(http.StatusOK, gin.H{"message": "allocations updated", "count": len(allocs)})
}

// GetProfiles 获取成本画像
func (h *Handler) GetProfiles(c *gin.Context) {
	c.JSON(http.StatusOK, h.optimizer.profiles)
}

// GetSuggestions 获取优化建议
func (h *Handler) GetSuggestions(c *gin.Context) {
	report := h.optimizer.GenerateReport()
	c.JSON(http.StatusOK, report.Suggestions)
}
