package storagedash

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 存储仪表盘 HTTP 处理器
type Handler struct {
	dashboard *Dashboard
	logger    *zap.Logger
}

// NewHandler 创建 HTTP 处理器实例
func NewHandler(dashboard *Dashboard, logger *zap.Logger) *Handler {
	return &Handler{
		dashboard: dashboard,
		logger:    logger,
	}
}

// RegisterRoutes 注册路由到 gin 路由组
// 路由前缀建议使用 /api/v1/dashboard
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/storage", h.handleGetStorage)
	rg.GET("/trends", h.handleGetTrends)
	rg.GET("/alerts", h.handleGetAlerts)
	rg.POST("/refresh", h.handleRefresh)
}

// handleGetStorage 获取存储概览
// GET /api/v1/dashboard/storage
func (h *Handler) handleGetStorage(c *gin.Context) {
	overview, err := h.dashboard.GetOverview()
	if err != nil {
		h.logger.Error("获取存储概览失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "获取存储概览失败",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    overview,
	})
}

// handleGetTrends 获取容量趋势
// GET /api/v1/dashboard/trends?days=7
func (h *Handler) handleGetTrends(c *gin.Context) {
	days := 7
	if d := c.Query("days"); d != "" {
		parsed, err := strconv.Atoi(d)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "参数 days 无效",
				"details": "days 必须为正整数",
			})
			return
		}
		days = parsed
	}

	trends, err := h.dashboard.GetCapacityTrends(days)
	if err != nil {
		h.logger.Error("获取容量趋势失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "获取容量趋势失败",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data": gin.H{
			"days":   days,
			"trends": trends,
		},
	})
}

// handleGetAlerts 获取告警汇总
// GET /api/v1/dashboard/alerts
func (h *Handler) handleGetAlerts(c *gin.Context) {
	alerts, err := h.dashboard.GetAlerts()
	if err != nil {
		h.logger.Error("获取告警汇总失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "获取告警汇总失败",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    alerts,
	})
}

// handleRefresh 刷新缓存
// POST /api/v1/dashboard/refresh
func (h *Handler) handleRefresh(c *gin.Context) {
	if err := h.dashboard.RefreshCache(); err != nil {
		h.logger.Error("刷新缓存失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "刷新缓存失败",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "缓存已刷新",
	})
}
