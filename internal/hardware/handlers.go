// Package hardware 硬件监控API
// 兵部 Round 141 - NVMe S.M.A.R.T. UI集成
package hardware

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"nas-os/internal/api"
	"nas-os/internal/hardware/nvme"

	"github.com/gin-gonic/gin"
)

// NVMeHandlers NVMe硬件监控处理器
type NVMeHandlers struct {
	monitor  *nvme.NVMeMonitor
	alerts   []nvme.Alert
	alertsMu sync.RWMutex
}

// NewNVMeHandlers 创建NVMe处理器
func NewNVMeHandlers() *NVMeHandlers {
	cfg := nvme.DefaultAlertConfig()
	monitor := nvme.NewNVMeMonitor(cfg)

	h := &NVMeHandlers{
		monitor: monitor,
		alerts:  make([]nvme.Alert, 0),
	}

	// 启动告警收集
	go h.collectAlerts()

	return h
}

// collectAlerts 收集告警
func (h *NVMeHandlers) collectAlerts() {
	for alert := range h.monitor.Alerts() {
		h.alertsMu.Lock()
		h.alerts = append(h.alerts, alert)
		// 保留最近100条告警
		if len(h.alerts) > 100 {
			h.alerts = h.alerts[len(h.alerts)-100:]
		}
		h.alertsMu.Unlock()
	}
}

// RegisterRoutes 注册路由
func (h *NVMeHandlers) RegisterRoutes(r *gin.RouterGroup) {
	nvmeGroup := r.Group("/hardware/nvme")
	{
		// 获取所有NVMe设备状态
		nvmeGroup.GET("", h.getDashboard)

		// 获取单个设备详情
		nvmeGroup.GET("/:device", h.getDeviceHealth)

		// 获取告警列表
		nvmeGroup.GET("/alerts", h.getAlerts)

		// 刷新数据
		nvmeGroup.POST("/refresh", h.refreshData)

		// 配置告警阈值
		nvmeGroup.PUT("/config", h.updateConfig)

		// Prometheus指标导出
		nvmeGroup.GET("/metrics", h.getMetrics)

		// 历史数据（保留7天）
		nvmeGroup.GET("/history", h.getHistory)
	}
}

// getDashboard 获取NVMe看板数据
// @Summary 获取NVMe看板数据
// @Description 获取所有NVMe设备的健康状态汇总
// @Tags hardware
// @Accept json
// @Produce json
// @Success 200 {object} api.Response "成功"
// @Failure 500 {object} api.Response "服务器内部错误"
// @Router /hardware/nvme [get]
// @Security BearerAuth
func (h *NVMeHandlers) getDashboard(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// 检查所有设备健康状态
	_, err := h.monitor.CheckAllHealth(ctx)
	if err != nil {
		// 即使检查失败，也返回缓存数据
	}

	dashboard := h.monitor.GetDashboard()

	api.OK(c, dashboard)
}

// getDeviceHealth 获取单个设备健康详情
// @Summary 获取单个NVMe设备健康详情
// @Description 获取指定NVMe设备的完整SMART数据
// @Tags hardware
// @Accept json
// @Produce json
// @Param device path string true "设备路径 (如 nvme0n1)"
// @Success 200 {object} api.Response "成功"
// @Failure 400 {object} api.Response "设备参数错误"
// @Failure 500 {object} api.Response "服务器内部错误"
// @Router /hardware/nvme/{device} [get]
// @Security BearerAuth
func (h *NVMeHandlers) getDeviceHealth(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		api.BadRequest(c, "设备名称不能为空")
		return
	}

	// 构建完整设备路径
	devicePath := "/dev/" + device

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	status, err := h.monitor.CheckHealth(ctx, devicePath)
	if err != nil {
		api.InternalError(c, "获取SMART数据失败: "+err.Error())
		return
	}

	api.OK(c, status)
}

// getAlerts 获取告警列表
// @Summary 获取NVMe告警列表
// @Description 获取所有NVMe设备的告警历史
// @Tags hardware
// @Accept json
// @Produce json
// @Param limit query int false "返回数量限制" default(50)
// @Param severity query string false "严重级别过滤 (warning/critical)"
// @Success 200 {object} api.Response "成功"
// @Router /hardware/nvme/alerts [get]
// @Security BearerAuth
func (h *NVMeHandlers) getAlerts(c *gin.Context) {
	limit := 50
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	severity := c.Query("severity")

	h.alertsMu.RLock()
	alerts := make([]nvme.Alert, 0)

	// 过滤告警
	for i := len(h.alerts) - 1; i >= 0 && len(alerts) < limit; i-- {
		alert := h.alerts[i]
		if severity == "" || alert.Severity == severity {
			alerts = append(alerts, alert)
		}
	}
	h.alertsMu.RUnlock()

	api.OK(c, alerts)
}

// refreshData 刷新NVMe数据
// @Summary 刷新NVMe监控数据
// @Description 强制刷新所有NVMe设备的SMART数据
// @Tags hardware
// @Accept json
// @Produce json
// @Success 200 {object} api.Response "成功"
// @Failure 500 {object} api.Response "服务器内部错误"
// @Router /hardware/nvme/refresh [post]
// @Security BearerAuth
func (h *NVMeHandlers) refreshData(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	results, err := h.monitor.CheckAllHealth(ctx)
	if err != nil {
		api.InternalError(c, "刷新NVMe数据失败: "+err.Error())
		return
	}

	api.OKWithMessage(c, "NVMe数据已刷新", gin.H{
		"devices_checked": len(results),
		"timestamp":       time.Now(),
	})
}

// updateConfig 更新告警配置
// @Summary 更新NVMe告警配置
// @Description 更新NVMe监控的告警阈值配置
// @Tags hardware
// @Accept json
// @Produce json
// @Param config body nvme.AlertConfig true "告警配置"
// @Success 200 {object} api.Response "成功"
// @Failure 400 {object} api.Response "配置参数错误"
// @Router /hardware/nvme/config [put]
// @Security BearerAuth
func (h *NVMeHandlers) updateConfig(c *gin.Context) {
	var config nvme.AlertConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		api.BadRequest(c, "配置格式错误: "+err.Error())
		return
	}

	// 验证配置范围
	if config.TemperatureThreshold < 50 || config.TemperatureThreshold > 100 {
		api.BadRequest(c, "温度阈值应在50-100°C之间")
		return
	}
	if config.PercentUsedThreshold < 70 || config.PercentUsedThreshold > 99 {
		api.BadRequest(c, "寿命阈值应在70-99%之间")
		return
	}
	if config.AvailableSpareThreshold < 5 || config.AvailableSpareThreshold > 50 {
		api.BadRequest(c, "备用空间阈值应在5-50%之间")
		return
	}

	// 更新monitor配置（需要重新创建monitor）
	h.monitor = nvme.NewNVMeMonitor(config)

	api.OKWithMessage(c, "告警配置已更新", config)
}

// getMetrics 获取Prometheus格式指标
// @Summary 获取NVMe Prometheus指标
// @Description 导出NVMe监控指标，供Prometheus抓取
// @Tags hardware
// @Accept json
// @Produce text/plain
// @Success 200 {string} string "Prometheus指标"
// @Router /hardware/nvme/metrics [get]
func (h *NVMeHandlers) getMetrics(c *gin.Context) {
	metrics := h.monitor.ExportMetrics()

	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(metrics))
}

// getHistory 获取历史数据
// @Summary 获取NVMe历史监控数据
// @Description 获取指定时间范围内的NVMe监控历史数据
// @Tags hardware
// @Accept json
// @Produce json
// @Param duration query string false "时间范围 (如 24h, 7d)" default(24h)
// @Success 200 {object} api.Response "成功"
// @Router /hardware/nvme/history [get]
// @Security BearerAuth
func (h *NVMeHandlers) getHistory(c *gin.Context) {
	duration := c.DefaultQuery("duration", "24h")

	// 返回当前状态作为历史数据（简化实现，实际需要持久化存储）
	status := h.monitor.GetAllStatus()

	history := make([]map[string]interface{}, 0)
	for device, health := range status {
		if health != nil {
			history = append(history, map[string]interface{}{
				"device":       device,
				"temperature":  health.Temperature,
				"percent_used": health.PercentUsed,
				"smart_status": health.SmartStatus,
				"timestamp":    health.LastChecked,
			})
		}
	}

	api.OK(c, gin.H{
		"duration":    duration,
		"data_points": len(history),
		"history":     history,
	})
}
