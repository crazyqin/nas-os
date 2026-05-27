package resourceoptimizer

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler HTTP处理器.
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建Handler.
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	return &Handler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/analyze", h.Analyze)
	rg.GET("/snapshot", h.GetSnapshot)
	rg.GET("/history", h.GetHistory)
	rg.GET("/recommendations", h.GetRecommendations)
	rg.GET("/trends", h.GetTrends)
	rg.GET("/analysis/last", h.GetLastAnalysis)
}

// Analyze 执行综合分析.
func (h *Handler) Analyze(c *gin.Context) {
	var req AnalyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 使用默认请求
		req = *DefaultAnalyzeRequest()
	}

	result, err := h.manager.Analyze(c.Request.Context(), &req)
	if err != nil {
		if err == ErrAnalysisInProgress {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		h.logger.Error("分析失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "分析失败"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetSnapshot 获取当前资源快照.
func (h *Handler) GetSnapshot(c *gin.Context) {
	snapshot, err := h.manager.CollectSnapshot(c.Request.Context())
	if err != nil {
		h.logger.Error("采集快照失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "采集快照失败"})
		return
	}

	c.JSON(http.StatusOK, snapshot)
}

// GetHistory 获取历史数据.
func (h *Handler) GetHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	history := h.manager.GetHistory(limit)
	c.JSON(http.StatusOK, gin.H{
		"data":  history,
		"count": len(history),
	})
}

// GetRecommendations 获取优化建议.
func (h *Handler) GetRecommendations(c *gin.Context) {
	resourceTypesParam := c.Query("resource_types")
	var resourceTypes []ResourceType

	if resourceTypesParam != "" {
		types := strings.Split(resourceTypesParam, ",")
		for _, t := range types {
			rt := ResourceType(strings.TrimSpace(t))
			switch rt {
			case ResourceCPU, ResourceMemory, ResourceDisk, ResourceNetwork:
				resourceTypes = append(resourceTypes, rt)
			}
		}
	}

	recommendations := h.manager.GetRecommendations(resourceTypes)
	c.JSON(http.StatusOK, gin.H{
		"recommendations": recommendations,
		"total":           len(recommendations),
	})
}

// GetTrends 获取趋势预测.
func (h *Handler) GetTrends(c *gin.Context) {
	resourceTypesParam := c.Query("resource_types")
	var resourceTypes []ResourceType

	if resourceTypesParam != "" {
		types := strings.Split(resourceTypesParam, ",")
		for _, t := range types {
			rt := ResourceType(strings.TrimSpace(t))
			switch rt {
			case ResourceCPU, ResourceMemory, ResourceDisk, ResourceNetwork:
				resourceTypes = append(resourceTypes, rt)
			}
		}
	}

	trends := h.manager.GetTrends(resourceTypes)
	c.JSON(http.StatusOK, gin.H{
		"trends": trends,
		"total":  len(trends),
	})
}

// GetLastAnalysis 获取最后一次分析结果.
func (h *Handler) GetLastAnalysis(c *gin.Context) {
	result := h.manager.GetLastAnalysis()
	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "尚未执行分析"})
		return
	}

	c.JSON(http.StatusOK, result)
}
