// Package smarthealthpredict 提供 REST API 处理器（Gin 框架）
package smarthealthpredict

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// response 标准响应结构
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handler 健康预测 API 处理器
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建处理器
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{manager: manager, logger: logger}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	health := r.Group("/storage/health")
	{
		// 磁盘扫描
		health.POST("/scan", h.ScanDisk)

		// 磁盘管理
		health.GET("/disks", h.ListDisks)
		// 磁盘报告（使用查询参数避免路径中 / 的问题）
		health.GET("/report", h.GetDiskReport)

		// 历史数据
		health.GET("/history", h.GetDiskHistory)

		// 告警
		health.GET("/alerts", h.GetAlerts)

		// 系统状态
		health.GET("/status", h.GetStatus)
	}
}

// ========== 磁盘扫描 ==========

// ScanDisk 扫描磁盘健康状态
func (h *Handler) ScanDisk(c *gin.Context) {
	device := c.Query("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "请提供磁盘设备参数 (device)"})
		return
	}

	report, err := h.manager.ScanDisk(c.Request.Context(), device)
	if err != nil {
		h.logger.Error("扫描磁盘失败", zap.String("device", device), zap.Error(err))
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "扫描完成", Data: report})
}

// ========== 磁盘管理 ==========

// ListDisks 列出所有已扫描的磁盘
func (h *Handler) ListDisks(c *gin.Context) {
	disks := h.manager.GetDiskList()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(disks),
			"disks": disks,
		},
	})
}

// GetDiskReport 获取单个磁盘的健康报告
func (h *Handler) GetDiskReport(c *gin.Context) {
	device := c.Query("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "请提供磁盘设备参数 (device)"})
		return
	}

	// 先扫描获取最新数据
	report, err := h.manager.ScanDisk(c.Request.Context(), device)
	if err != nil {
		h.logger.Error("获取磁盘报告失败", zap.String("device", device), zap.Error(err))
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: report})
}

// ========== 历史数据 ==========

// GetDiskHistory 获取磁盘历史数据
func (h *Handler) GetDiskHistory(c *gin.Context) {
	device := c.Query("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "请提供磁盘设备参数 (device)"})
		return
	}

	daysStr := c.DefaultQuery("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		days = 30
	}

	history := h.manager.GetDiskHistory(device, days)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"device":  device,
			"history": history,
			"days":    days,
			"count":   len(history),
		},
	})
}

// ========== 告警 ==========

// GetAlerts 获取所有磁盘告警
func (h *Handler) GetAlerts(c *gin.Context) {
	alerts := h.manager.GetAlerts()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"alerts": alerts,
			"total":  len(alerts),
		},
	})
}

// ========== 系统状态 ==========

// GetStatus 获取系统整体状态
func (h *Handler) GetStatus(c *gin.Context) {
	status := h.manager.GetSystemStatus()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: status})
}
