// Package disk 提供磁盘监控 API 处理器
// Version: v2.45.0 - 集成 SMART 监控到现有监控 API
package disk

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 磁盘监控处理器
type Handlers struct {
	monitor *SMARTMonitor
}

// NewHandlers 创建磁盘监控处理器
func NewHandlers(monitor *SMARTMonitor) *Handlers {
	return &Handlers{
		monitor: monitor,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	disk := r.Group("/disk")
	{
		// 磁盘列表和健康状态
		disk.GET("", h.listDisks)
		disk.GET("/:device", h.getDiskInfo)

		// SMART 数据
		disk.GET("/:device/smart", h.getSMARTData)
		disk.GET("/:device/health", h.getDiskHealth)
		disk.GET("/:device/history", h.getDiskHistory)
		disk.GET("/:device/predictions", h.getDiskPredictions)

		// 告警管理
		disk.GET("/alerts", h.getAlerts)
		disk.POST("/alerts/:id/acknowledge", h.acknowledgeAlert)
		disk.GET("/alerts/rules", h.getAlertRules)
		disk.PUT("/alerts/rules/:id", h.updateAlertRule)

		// 配置
		disk.PUT("/config/weights", h.setScoreWeights)

		// 扫描和检查
		disk.POST("/scan", h.scanDisks)
		disk.POST("/check", h.checkAllDisks)

		// 导出
		disk.GET("/export", h.exportData)
		disk.POST("/import", h.importData)
	}
}

// listDisks 获取磁盘列表
// @Summary 获取磁盘列表
// @Description 获取所有磁盘的健康状态和基本信息
// @Tags disk
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk [get]
// @Security BearerAuth
func (h *Handlers) listDisks(c *gin.Context) {
	disks := h.monitor.GetAllDisks()

	summary := struct {
		Total     int `json:"total"`
		Healthy   int `json:"healthy"`
		Warning   int `json:"warning"`
		Critical  int `json:"critical"`
		Unknown   int `json:"unknown"`
		Offline   int `json:"offline"`
	}{}

	for _, disk := range disks {
		summary.Total++
		switch disk.Status {
		case StatusHealthy:
			summary.Healthy++
		case StatusWarning:
			summary.Warning++
		case StatusCritical:
			summary.Critical++
		case StatusUnknown:
			summary.Unknown++
		case StatusOffline:
			summary.Offline++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"summary": summary,
			"disks":   disks,
		},
	})
}

// getDiskInfo 获取磁盘详情
// @Summary 获取磁盘详情
// @Description 获取指定磁盘的详细信息和健康状态
// @Tags disk
// @Accept json
// @Produce json
// @Param device path string true "磁盘设备路径 (如 /dev/sda)"
// @Success 200 {object} map[string]interface{} "成功"
// @Failure 404 {object} map[string]interface{} "磁盘不存在"
// @Router /disk/{device} [get]
// @Security BearerAuth
func (h *Handlers) getDiskInfo(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备路径不能为空",
		})
		return
	}

	disk, err := h.monitor.GetDiskInfo(device)
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

// getSMARTData 获取 SMART 数据
// @Summary 获取 SMART 数据
// @Description 获取指定磁盘的 SMART 原始数据
// @Tags disk
// @Accept json
// @Produce json
// @Param device path string true "磁盘设备路径"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/{device}/smart [get]
// @Security BearerAuth
func (h *Handlers) getSMARTData(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备路径不能为空",
		})
		return
	}

	disk, err := h.monitor.GetDiskInfo(device)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	if disk.SmartData == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "无 SMART 数据",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    disk.SmartData,
	})
}

// getDiskHealth 获取磁盘健康评分
// @Summary 获取磁盘健康评分
// @Description 获取指定磁盘的健康评分和分项评分
// @Tags disk
// @Accept json
// @Produce json
// @Param device path string true "磁盘设备路径"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/{device}/health [get]
// @Security BearerAuth
func (h *Handlers) getDiskHealth(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备路径不能为空",
		})
		return
	}

	disk, err := h.monitor.GetDiskInfo(device)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	if disk.HealthScore == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "无健康评分数据",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    disk.HealthScore,
	})
}

// getDiskHistory 获取磁盘历史数据
// @Summary 获取磁盘历史数据
// @Description 获取指定磁盘的历史健康数据
// @Tags disk
// @Accept json
// @Produce json
// @Param device path string true "磁盘设备路径"
// @Param duration query string false "时间范围 (24h/7d/30d)" default(7d)
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/{device}/history [get]
// @Security BearerAuth
func (h *Handlers) getDiskHistory(c *gin.Context) {
	device := c.Param("device")
	durationStr := c.DefaultQuery("duration", "7d")

	var duration time.Duration
	switch durationStr {
	case "24h":
		duration = 24 * time.Hour
	case "7d":
		duration = 7 * 24 * time.Hour
	case "30d":
		duration = 30 * 24 * time.Hour
	default:
		duration = 7 * 24 * time.Hour
	}

	history := h.monitor.GetHistory(device, duration)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"device":   device,
			"duration": durationStr,
			"points":   history,
		},
	})
}

// getDiskPredictions 获取磁盘故障预测
// @Summary 获取磁盘故障预测
// @Description 获取指定磁盘的故障预测信息
// @Tags disk
// @Accept json
// @Produce json
// @Param device path string true "磁盘设备路径"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/{device}/predictions [get]
// @Security BearerAuth
func (h *Handlers) getDiskPredictions(c *gin.Context) {
	device := c.Param("device")

	disk, err := h.monitor.GetDiskInfo(device)
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
		"data": gin.H{
			"device":      device,
			"predictions": disk.Predictions,
		},
	})
}

// getAlerts 获取告警列表
// @Summary 获取告警列表
// @Description 获取磁盘健康告警列表
// @Tags disk
// @Accept json
// @Produce json
// @Param device query string false "过滤设备"
// @Param acknowledged query bool false "是否包含已确认告警"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/alerts [get]
// @Security BearerAuth
func (h *Handlers) getAlerts(c *gin.Context) {
	device := c.Query("device")
	acknowledged := c.Query("acknowledged") == "true"

	alerts := h.monitor.GetAlerts(device, acknowledged)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    alerts,
	})
}

// acknowledgeAlert 确认告警
// @Summary 确认告警
// @Description 确认指定的磁盘健康告警
// @Tags disk
// @Accept json
// @Produce json
// @Param id path string true "告警 ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Failure 404 {object} map[string]interface{} "告警不存在"
// @Router /disk/alerts/{id}/acknowledge [post]
// @Security BearerAuth
func (h *Handlers) acknowledgeAlert(c *gin.Context) {
	id := c.Param("id")

	if err := h.monitor.AcknowledgeAlert(id); err != nil {
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

// getAlertRules 获取告警规则
// @Summary 获取告警规则
// @Description 获取所有磁盘健康告警规则
// @Tags disk
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/alerts/rules [get]
// @Security BearerAuth
func (h *Handlers) getAlertRules(c *gin.Context) {
	rules := h.monitor.GetAlertRules()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    rules,
	})
}

// updateAlertRule 更新告警规则
// @Summary 更新告警规则
// @Description 更新指定的告警规则
// @Tags disk
// @Accept json
// @Produce json
// @Param id path string true "规则 ID"
// @Param request body AlertRule true "规则配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/alerts/rules/{id} [put]
// @Security BearerAuth
func (h *Handlers) updateAlertRule(c *gin.Context) {
	var req AlertRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	req.ID = c.Param("id")
	if req.LastTriggered == nil {
		req.LastTriggered = make(map[string]time.Time)
	}

	h.monitor.SetAlertRule(&req)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "规则已更新",
	})
}

// setScoreWeights 设置评分权重
// @Summary 设置评分权重
// @Description 设置健康评分的计算权重
// @Tags disk
// @Accept json
// @Produce json
// @Param request body ScoreWeights true "权重配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/config/weights [put]
// @Security BearerAuth
func (h *Handlers) setScoreWeights(c *gin.Context) {
	var weights ScoreWeights
	if err := c.ShouldBindJSON(&weights); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	h.monitor.SetScoreWeights(&weights)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "权重已更新",
	})
}

// scanDisks 扫描磁盘
// @Summary 扫描磁盘
// @Description 重新扫描系统磁盘
// @Tags disk
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/scan [post]
// @Security BearerAuth
func (h *Handlers) scanDisks(c *gin.Context) {
	if err := h.monitor.ScanDisks(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "扫描完成",
		"data":    h.monitor.GetAllDisks(),
	})
}

// checkAllDisks 检查所有磁盘
// @Summary 检查所有磁盘
// @Description 立即检查所有磁盘的 SMART 数据
// @Tags disk
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/check [post]
// @Security BearerAuth
func (h *Handlers) checkAllDisks(c *gin.Context) {
	if err := h.monitor.CheckAllDisks(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "检查完成",
		"data":    h.monitor.GetAllDisks(),
	})
}

// exportData 导出数据
// @Summary 导出数据
// @Description 导出磁盘监控数据为 JSON
// @Tags disk
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/export [get]
// @Security BearerAuth
func (h *Handlers) exportData(c *gin.Context) {
	data, err := h.monitor.ExportJSON()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.Data(http.StatusOK, "application/json", data)
}

// importData 导入数据
// @Summary 导入数据
// @Description 从 JSON 导入告警规则配置
// @Tags disk
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "导入数据"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/import [post]
// @Security BearerAuth
func (h *Handlers) importData(c *gin.Context) {
	data, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.monitor.ImportJSON(data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "导入成功",
	})
}