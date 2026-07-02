package syshealth

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 系统健康仪表盘 HTTP 处理器。
type Handler struct {
	dashboard *Dashboard
	logger    *zap.Logger
}

// NewHandler 创建 HTTP 处理器实例。
func NewHandler(dashboard *Dashboard, logger *zap.Logger) *Handler {
	return &Handler{
		dashboard: dashboard,
		logger:    logger,
	}
}

// RegisterRoutes 注册路由到 gin 路由组。
// 路由前缀: /api/v1/syshealth.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/overview", h.handleOverview)
	rg.GET("/trends", h.handleTrends)
	rg.GET("/alerts", h.handleAlerts)
	rg.POST("/alerts/:id/resolve", h.handleResolveAlert)
	rg.GET("/fixes", h.handleListFixes)
	rg.POST("/fix/:issue", h.handleFix)
	rg.GET("/subsystem/:name", h.handleSubsystem)
	rg.GET("/history", h.handleHistory)
	rg.POST("/refresh", h.handleRefresh)
}

// ========== 总览 ==========

// handleOverview 获取系统总览。
// GET /api/v1/syshealth/overview.
func (h *Handler) handleOverview(c *gin.Context) {
	overview, err := h.dashboard.GetOverview()
	if err != nil {
		h.logger.Error("获取系统总览失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": "获取系统总览失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, OverviewResponse{
		Code:    0,
		Message: "ok",
		Data:    *overview,
	})
}

// ========== 趋势 ==========

// handleTrends 获取健康趋势。
// GET /api/v1/syshealth/trends?days=30.
func (h *Handler) handleTrends(c *gin.Context) {
	days := 30
	if d := c.Query("days"); d != "" {
		parsed, err := strconv.Atoi(d)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    1,
				"message": "参数 days 无效",
				"error":   "days 必须为正整数",
			})
			return
		}
		days = parsed
	}

	trends, err := h.dashboard.GetTrends(days)
	if err != nil {
		h.logger.Error("获取健康趋势失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": "获取健康趋势失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, TrendsResponse{
		Code:    0,
		Message: "ok",
		Data:    *trends,
	})
}

// ========== 告警 ==========

// handleAlerts 获取告警列表。
// GET /api/v1/syshealth/alerts?resolved=false.
func (h *Handler) handleAlerts(c *gin.Context) {
	resolved := false
	if r := c.Query("resolved"); r == "true" {
		resolved = true
	}

	alerts := h.dashboard.GetAlerts(resolved)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data": gin.H{
			"alerts": alerts,
			"total":  len(alerts),
		},
	})
}

// handleResolveAlert 解决告警。
// POST /api/v1/syshealth/alerts/:id/resolve.
func (h *Handler) handleResolveAlert(c *gin.Context) {
	alertID := c.Param("id")
	if alertID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "告警 ID 不能为空",
		})
		return
	}

	if err := h.dashboard.ResolveAlert(alertID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "告警已解决",
	})
}

// ========== 快速修复 ==========

// handleListFixes 获取可用修复动作列表。
// GET /api/v1/syshealth/fixes.
func (h *Handler) handleListFixes(c *gin.Context) {
	fixes := h.dashboard.GetAvailableFixes()

	c.JSON(http.StatusOK, AvailableFixesResponse{
		Code:    0,
		Message: "ok",
		Data:    fixes,
	})
}

// handleFix 执行快速修复。
// POST /api/v1/syshealth/fix/:issue.
func (h *Handler) handleFix(c *gin.Context) {
	issue := c.Param("issue")
	if issue == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "修复动作 ID 不能为空",
		})
		return
	}

	var req FixRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 如果没有请求体，使用默认值
		req = FixRequest{
			Issue:   issue,
			Confirm: true, // 默认确认
		}
	}

	// 如果请求体中没有 issue，使用路径参数
	if req.Issue == "" {
		req.Issue = issue
	}

	result, err := h.dashboard.ExecuteFix(req.Issue, req.Confirm, req.Params)
	if err != nil {
		h.logger.Error("执行修复失败",
			zap.String("issue", issue),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}

	h.logger.Info("修复执行成功",
		zap.String("issue", issue),
		zap.Bool("success", result.Success),
	)

	c.JSON(http.StatusOK, FixResponse{
		Code:    0,
		Message: "修复操作已执行",
		Data:    *result,
	})
}

// ========== 子系统 ==========

// handleSubsystem 获取指定子系统状态。
// GET /api/v1/syshealth/subsystem/:name.
func (h *Handler) handleSubsystem(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "子系统名称不能为空",
		})
		return
	}

	status, err := h.dashboard.GetSubsystemStatus(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    status,
	})
}

// ========== 历史记录 ==========

// handleHistory 获取历史记录。
// GET /api/v1/syshealth/history?days=7.
func (h *Handler) handleHistory(c *gin.Context) {
	days := 7
	if d := c.Query("days"); d != "" {
		parsed, err := strconv.Atoi(d)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    1,
				"message": "参数 days 无效",
				"error":   "days 必须为正整数",
			})
			return
		}
		days = parsed
	}

	history := h.dashboard.GetHistory(days)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data": gin.H{
			"days":    days,
			"records": history,
			"total":   len(history),
		},
	})
}

// ========== 刷新缓存 ==========

// handleRefresh 强制刷新缓存。
// POST /api/v1/syshealth/refresh.
func (h *Handler) handleRefresh(c *gin.Context) {
	if err := h.dashboard.RefreshCache(); err != nil {
		h.logger.Error("刷新缓存失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": "刷新缓存失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "缓存已刷新",
	})
}
