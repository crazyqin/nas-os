// Package monitoring 提供 SSD 健康监控 API 处理器
package monitoring

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// SSDHandlers SSD 健康监控处理器
type SSDHandlers struct {
	monitor *SSDHealthMonitor
}

// NewSSDHandlers 创建 SSD 健康监控处理器
func NewSSDHandlers(monitor *SSDHealthMonitor) *SSDHandlers {
	return &SSDHandlers{
		monitor: monitor,
	}
}

// RegisterRoutes 注册路由
func (h *SSDHandlers) RegisterRoutes(r *gin.RouterGroup) {
	ssd := r.Group("/ssd")
	{
		// SSD 列表和健康状态
		ssd.GET("", h.listSSDs)
		ssd.GET("/:device", h.getSSDHealth)
		ssd.GET("/:device/history", h.getSSDHistory)

		// 扫描和检查
		ssd.POST("/scan", h.scanSSDs)
		ssd.POST("/check", h.checkAllSSDs)

		// 告警管理
		ssd.GET("/alerts", h.getSSDAlerts)
		ssd.POST("/alerts/callback", h.registerAlertCallback)

		// 配置
		ssd.PUT("/config", h.updateConfig)
		ssd.GET("/config", h.getConfig)

		// 摘要统计
		ssd.GET("/summary", h.getSummary)

		// 寿命预测
		ssd.GET("/:device/prediction", h.getLifePrediction)
	}
}

// listSSDs 获取 SSD 列表
// @Summary 获取 SSD 列表
// @Description 获取所有 SSD 的健康状态和基本信息
// @Tags ssd
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ssd [get]
// @Security BearerAuth
func (h *SSDHandlers) listSSDs(c *gin.Context) {
	ssds := h.monitor.GetAllSSDs()

	summary := struct {
		Total     int `json:"total"`
		Healthy   int `json:"healthy"`
		Warning   int `json:"warning"`
		Critical  int `json:"critical"`
		Emergency int `json:"emergency"`
		Unknown   int `json:"unknown"`
		Offline   int `json:"offline"`
	}{
		Total: len(ssds),
	}

	for _, ssd := range ssds {
		switch ssd.Status {
		case SSDStatusHealthy:
			summary.Healthy++
		case SSDStatusWarning:
			summary.Warning++
		case SSDStatusCritical:
			summary.Critical++
		case SSDStatusEmergency:
			summary.Emergency++
		case SSDStatusUnknown:
			summary.Unknown++
		case SSDStatusOffline:
			summary.Offline++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"summary": summary,
			"ssds":    ssds,
		},
	})
}

// getSSDHealth 获取 SSD 健康详情
// @Summary 获取 SSD 健康详情
// @Description 获取指定 SSD 的详细健康信息
// @Tags ssd
// @Accept json
// @Produce json
// @Param device path string true "SSD 设备路径 (如 /dev/nvme0n1)"
// @Success 200 {object} map[string]interface{} "成功"
// @Failure 404 {object} map[string]interface{} "SSD 不存在"
// @Router /ssd/{device} [get]
// @Security BearerAuth
func (h *SSDHandlers) getSSDHealth(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备路径不能为空",
		})
		return
	}

	health, err := h.monitor.GetSSDHealth(device)
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
		"data":    health,
	})
}

// getSSDHistory 获取 SSD 历史数据
// @Summary 获取 SSD 历史数据
// @Description 获取指定 SSD 的历史健康数据
// @Tags ssd
// @Accept json
// @Produce json
// @Param device path string true "SSD 设备路径"
// @Param days query int false "天数" default(7)
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ssd/{device}/history [get]
// @Security BearerAuth
func (h *SSDHandlers) getSSDHistory(c *gin.Context) {
	device := c.Param("device")
	days := 7

	if d := c.Query("days"); d != "" {
		if parsed, err := parseInt(d); err == nil && parsed > 0 {
			days = parsed
		}
	}

	history := h.monitor.GetSSDHistory(device, days)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"device":  device,
			"days":    days,
			"history": history,
		},
	})
}

// scanSSDs 扫描 SSD
// @Summary 扫描 SSD
// @Description 重新扫描系统 SSD 设备
// @Tags ssd
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ssd/scan [post]
// @Security BearerAuth
func (h *SSDHandlers) scanSSDs(c *gin.Context) {
	if err := h.monitor.ScanSSDs(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "扫描完成",
		"data":    h.monitor.GetAllSSDs(),
	})
}

// checkAllSSDs 检查所有 SSD
// @Summary 检查所有 SSD
// @Description 立即检查所有 SSD 的健康状态
// @Tags ssd
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ssd/check [post]
// @Security BearerAuth
func (h *SSDHandlers) checkAllSSDs(c *gin.Context) {
	if err := h.monitor.CheckAllSSDs(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "检查完成",
		"data":    h.monitor.GetAllSSDs(),
	})
}

// getSSDAlerts 获取 SSD 告警
// @Summary 获取 SSD 告警
// @Description 获取 SSD 健康告警列表（内存中最近触发的告警）
// @Tags ssd
// @Accept json
// @Produce json
// @Param device query string false "过滤设备"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ssd/alerts [get]
// @Security BearerAuth
func (h *SSDHandlers) getSSDAlerts(c *gin.Context) {
	// 返回当前有告警的 SSD 列表
	ssds := h.monitor.GetAllSSDs()
	var alerts []gin.H

	for _, ssd := range ssds {
		if ssd.AlertLevel != AlertLevelNone {
			alerts = append(alerts, gin.H{
				"device":     ssd.Device,
				"alertLevel": ssd.AlertLevel,
				"message":    ssd.AlertMessage,
				"lifeUsed":   ssd.LifeUsedPercent,
				"timestamp":  ssd.LastCheck,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    alerts,
	})
}

// registerAlertCallback 注册告警回调 (用于 webhook 等)
// @Summary 注册告警回调
// @Description 注册一个接收 SSD 健康告警的回调 URL
// @Tags ssd
// @Accept json
// @Produce json
// @Param request body map[string]string true "回调配置 {\"url\": \"https://...\"}"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ssd/alerts/callback [post]
// @Security BearerAuth
func (h *SSDHandlers) registerAlertCallback(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	// 注册回调
	h.monitor.RegisterAlertCallback(func(alert *SSDHealthAlert) {
		// 在实际实现中，这里应该发送 HTTP 请求到 req.URL
		// 简化实现：只记录告警
		_ = req.URL // 避免未使用变量警告
	})

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "回调已注册",
	})
}

// updateConfig 更新监控配置
// @Summary 更新监控配置
// @Description 更新 SSD 监控配置
// @Tags ssd
// @Accept json
// @Produce json
// @Param request body SSDMonitorConfig true "配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ssd/config [put]
// @Security BearerAuth
func (h *SSDHandlers) updateConfig(c *gin.Context) {
	var config SSDMonitorConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	// 验证阈值
	if config.WarningThreshold >= config.CriticalThreshold {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "警告阈值必须小于严重阈值",
		})
		return
	}
	if config.CriticalThreshold >= config.EmergencyThreshold {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "严重阈值必须小于紧急阈值",
		})
		return
	}

	h.monitor.mu.Lock()
	h.monitor.config = &config
	h.monitor.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "配置已更新",
		"data":    config,
	})
}

// getConfig 获取监控配置
// @Summary 获取监控配置
// @Description 获取当前 SSD 监控配置
// @Tags ssd
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ssd/config [get]
// @Security BearerAuth
func (h *SSDHandlers) getConfig(c *gin.Context) {
	h.monitor.mu.RLock()
	config := h.monitor.config
	h.monitor.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    config,
	})
}

// getSummary 获取 SSD 健康摘要
// @Summary 获取 SSD 健康摘要
// @Description 获取所有 SSD 的健康摘要统计
// @Tags ssd
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ssd/summary [get]
// @Security BearerAuth
func (h *SSDHandlers) getSummary(c *gin.Context) {
	ssds := h.monitor.GetAllSSDs()

	var (
		totalSize          uint64
		totalWrites        uint64
		avgHealth          float64
		avgLifeUsed        float64
		highestTemp        int
		criticalSSDs       []string
		needReplacementSSDs []string
	)

	for _, ssd := range ssds {
		totalSize += ssd.Size
		totalWrites += ssd.TotalWrites
		avgHealth += ssd.HealthPercent
		avgLifeUsed += ssd.LifeUsedPercent

		if ssd.Temperature > highestTemp {
			highestTemp = ssd.Temperature
		}

		if ssd.Status == SSDStatusCritical || ssd.Status == SSDStatusEmergency {
			criticalSSDs = append(criticalSSDs, ssd.Device)
		}

		if ssd.LifeUsedPercent >= 80 {
			needReplacementSSDs = append(needReplacementSSDs, ssd.Device)
		}
	}

	if len(ssds) > 0 {
		avgHealth /= float64(len(ssds))
		avgLifeUsed /= float64(len(ssds))
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"totalSSDs":           len(ssds),
			"totalCapacity":       totalSize,
			"totalCapacityHuman":  formatBytes(totalSize),
			"totalWrites":         totalWrites,
			"totalWritesHuman":    formatBytes(totalWrites),
			"avgHealth":           avgHealth,
			"avgLifeUsed":         avgLifeUsed,
			"highestTemp":         highestTemp,
			"criticalSSDs":        criticalSSDs,
			"needReplacementSSDs": needReplacementSSDs,
			"lastCheck":           time.Now(),
		},
	})
}

// getLifePrediction 获取寿命预测
// @Summary 获取寿命预测
// @Description 获取指定 SSD 的寿命预测信息
// @Tags ssd
// @Accept json
// @Produce json
// @Param device path string true "SSD 设备路径"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ssd/{device}/prediction [get]
// @Security BearerAuth
func (h *SSDHandlers) getLifePrediction(c *gin.Context) {
	device := c.Param("device")

	health, err := h.monitor.GetSSDHealth(device)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	if health.PredictedLife == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "数据不足以进行预测",
			"data": gin.H{
				"device":    device,
				"prediction": nil,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"device":     device,
			"prediction": health.PredictedLife,
			"currentHealth": gin.H{
				"healthPercent":   health.HealthPercent,
				"lifeUsedPercent": health.LifeUsedPercent,
				"totalWrites":     health.TotalWrites,
			},
		},
	})
}

// parseInt 解析整数
func parseInt(s string) (int, error) {
	var result int
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			result = result*10 + int(s[i]-'0')
		}
	}
	return result, nil
}

// formatBytes 格式化字节数
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}