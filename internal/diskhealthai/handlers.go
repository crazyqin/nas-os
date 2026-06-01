// Package diskhealthai HTTP API 处理器
package diskhealthai

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Service 业务接口（由后续 service 层实现）
type Service interface {
	// ListDisks 获取磁盘列表
	ListDisks() []DiskInfo
	// GetDisk 获取单个磁盘信息
	GetDisk(device string) (*DiskInfo, error)
	// GetSMART 获取磁盘 SMART 数据
	GetSMART(device string) (*SMARTSnapshot, error)
	// Predict 获取故障预测
	Predict(device string) (*HealthReport, error)
	// TriggerScan 触发全盘扫描
	TriggerScan() error
	// ListAlerts 获取告警列表
	ListAlerts() []Alert
	// GetHistory 获取历史趋势
	GetHistory(device string, days int) (*TrendAnalysis, error)
}

// Handler HTTP 处理器
type Handler struct {
	svc Service
}

// NewHandler 创建处理器
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group("/disk-health-ai")
	{
		group.GET("/disks", h.ListDisks)
		group.GET("/disks/:device", h.GetDisk)
		group.GET("/disks/:device/smart", h.GetSMART)
		group.GET("/disks/:device/predict", h.Predict)
		group.POST("/scan", h.TriggerScan)
		group.GET("/alerts", h.ListAlerts)
		group.GET("/history", h.GetHistory)
	}
}

// ListDisks 获取磁盘列表
// GET /disk-health-ai/disks
func (h *Handler) ListDisks(c *gin.Context) {
	disks := h.svc.ListDisks()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    disks,
	})
}

// GetDisk 获取单个磁盘详情
// GET /disk-health-ai/disks/:device
func (h *Handler) GetDisk(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 device 参数"})
		return
	}

	disk, err := h.svc.GetDisk(device)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    disk,
	})
}

// GetSMART 获取 SMART 数据
// GET /disk-health-ai/disks/:device/smart
func (h *Handler) GetSMART(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 device 参数"})
		return
	}

	smart, err := h.svc.GetSMART(device)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    smart,
	})
}

// Predict 获取故障预测
// GET /disk-health-ai/disks/:device/predict
func (h *Handler) Predict(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 device 参数"})
		return
	}

	report, err := h.svc.Predict(device)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    report,
	})
}

// TriggerScan 触发全盘扫描
// POST /disk-health-ai/scan
func (h *Handler) TriggerScan(c *gin.Context) {
	if err := h.svc.TriggerScan(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "扫描任务已触发",
	})
}

// ListAlerts 获取告警列表
// GET /disk-health-ai/alerts
func (h *Handler) ListAlerts(c *gin.Context) {
	alerts := h.svc.ListAlerts()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    alerts,
	})
}

// GetHistory 获取历史趋势
// GET /disk-health-ai/history?device=xxx&days=90
func (h *Handler) GetHistory(c *gin.Context) {
	device := c.Query("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 device 参数"})
		return
	}

	days := 90
	if d, err := strconv.Atoi(c.Query("days")); err == nil && d > 0 {
		days = d
	}

	trend, err := h.svc.GetHistory(device, days)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    trend,
	})
}
