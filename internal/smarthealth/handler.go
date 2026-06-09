package smarthealth

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler API 处理器.
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建 API 处理器.
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	return &Handler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	// 健康检查路由（测试期望的路径）
	health := r.Group("/health")
	{
		health.GET("", h.GetHealth)
		health.POST("/check", h.RunCheck)
		health.GET("/trends", h.GetTrendsAPI)
		health.GET("/alerts", h.GetAlertsAPI)
		health.GET("/config", h.GetConfigAPI)
		health.PUT("/config", h.UpdateConfigAPI)
	}

	// SMART 健康管理路由
	smart := r.Group("/smarthealth")
	{
		// 磁盘扫描
		smart.POST("/scan", h.ScanDisks)

		// 健康查询
		smart.GET("/disks", h.GetAllDisksHealth)
		smart.GET("/disks/:id", h.GetDiskHealth)
		smart.GET("/disks/:id/trend", h.GetDiskTrend)

		// 故障预测
		smart.GET("/disks/:id/prediction", h.PredictFailure)

		// 告警管理
		smart.GET("/alerts", h.GetAlerts)
		smart.POST("/alerts/:id/ack", h.AcknowledgeAlert)
		smart.POST("/alerts/:id/resolve", h.ResolveAlert)

		// 报告
		smart.GET("/report", h.GenerateReport)
	}
}

// ScanDisks 扫描磁盘.
func (h *Handler) ScanDisks(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许空请求体
		req = ScanRequest{}
	}

	disks, err := h.manager.ScanDisks(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "扫描磁盘失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "扫描完成",
		"data":    disks,
	})
}

// GetAllDisksHealth 获取所有磁盘健康信息.
func (h *Handler) GetAllDisksHealth(c *gin.Context) {
	disks := h.manager.GetAllDisksHealth()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    disks,
	})
}

// GetDiskHealth 获取磁盘健康信息.
func (h *Handler) GetDiskHealth(c *gin.Context) {
	diskID := c.Param("id")
	disk, err := h.manager.GetDiskHealth(diskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    disk,
	})
}

// GetDiskTrend 获取磁盘趋势.
func (h *Handler) GetDiskTrend(c *gin.Context) {
	diskID := c.Param("id")
	daysStr := c.DefaultQuery("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil {
		days = 30
	}

	trends := h.manager.GetDiskTrend(diskID, days)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    trends,
	})
}

// PredictFailure 预测故障.
func (h *Handler) PredictFailure(c *gin.Context) {
	diskID := c.Param("id")
	prediction, err := h.manager.PredictFailure(diskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    prediction,
	})
}

// GetAlerts 获取告警.
func (h *Handler) GetAlerts(c *gin.Context) {
	level := AlertLevel(c.Query("level"))
	unresolved := c.Query("unresolved") == "true"

	alerts := h.manager.GetHealthAlerts(level, unresolved)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    alerts,
	})
}

// AcknowledgeAlert 确认告警.
func (h *Handler) AcknowledgeAlert(c *gin.Context) {
	alertID := c.Param("id")
	if err := h.manager.AcknowledgeAlert(alertID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "告警已确认",
	})
}

// ResolveAlert 解决告警.
func (h *Handler) ResolveAlert(c *gin.Context) {
	alertID := c.Param("id")
	if err := h.manager.ResolveAlert(alertID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "告警已解决",
	})
}

// GenerateReport 生成报告.
func (h *Handler) GenerateReport(c *gin.Context) {
	report := h.manager.GenerateReport()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    report,
	})
}

// GetHealth 获取健康状态.
func (h *Handler) GetHealth(c *gin.Context) {
	result := h.manager.RunManualCheck()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// RunCheck 运行健康检查.
func (h *Handler) RunCheck(c *gin.Context) {
	result := h.manager.RunManualCheck()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "检查完成",
		"data":    result,
	})
}

// GetTrendsAPI 获取趋势数据.
func (h *Handler) GetTrendsAPI(c *gin.Context) {
	hoursStr := c.DefaultQuery("hours", "24")
	hours, err := strconv.Atoi(hoursStr)
	if err != nil {
		hours = 24
	}
	trends := h.manager.GetTrends(hours)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    trends,
	})
}

// GetAlertsAPI 获取告警.
func (h *Handler) GetAlertsAPI(c *gin.Context) {
	alerts := h.manager.GetAlerts(false)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    alerts,
	})
}

// GetConfigAPI 获取配置.
func (h *Handler) GetConfigAPI(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    config,
	})
}

// UpdateConfigAPI 更新配置.
func (h *Handler) UpdateConfigAPI(c *gin.Context) {
	var config PatrolConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}
	if err := h.manager.UpdateConfig(&config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新配置失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "配置已更新",
	})
}
