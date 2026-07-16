// Package antivirus - HTTP API 处理器
package antivirus

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 病毒扫描 HTTP 处理器.
type Handlers struct {
	mgr *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{mgr: mgr}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	av := api.Group("/antivirus")
	{
		// 扫描任务
		av.POST("/scans", h.CreateScan)
		av.GET("/scans", h.ListScans)
		av.GET("/scans/:id", h.GetScan)
		av.GET("/scans/:id/report", h.GetReport)
		av.POST("/scans/:id/cancel", h.CancelScan)
		// 隔离区
		av.GET("/quarantine", h.ListQuarantine)
		av.POST("/quarantine/:id/restore", h.RestoreQuarantine)
		av.DELETE("/quarantine/:id", h.DeleteQuarantine)
		// 白名单
		av.GET("/whitelist", h.ListWhitelist)
		av.POST("/whitelist", h.AddWhitelist)
		av.DELETE("/whitelist/:id", h.RemoveWhitelist)
		// 病毒库
		av.GET("/virusdb", h.GetVirusDBStatus)
		av.POST("/virusdb/update", h.UpdateVirusDB)
		// 实时监控
		av.GET("/monitor", h.GetMonitorConfig)
		av.PUT("/monitor", h.UpdateMonitorConfig)
		// 统计
		av.GET("/stats", h.GetStats)
	}
}

func (h *Handlers) CreateScan(c *gin.Context) {
	var req CreateScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	task, err := h.mgr.CreateScan(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, task)
}

func (h *Handlers) ListScans(c *gin.Context) {
	c.JSON(http.StatusOK, h.mgr.ListScans())
}

func (h *Handlers) GetScan(c *gin.Context) {
	task, err := h.mgr.GetScan(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *Handlers) GetReport(c *gin.Context) {
	report, err := h.mgr.GetScanReport(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, report)
}

func (h *Handlers) CancelScan(c *gin.Context) {
	if err := h.mgr.CancelScan(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已取消"})
}

func (h *Handlers) ListQuarantine(c *gin.Context) {
	c.JSON(http.StatusOK, h.mgr.GetQuarantineList())
}

func (h *Handlers) RestoreQuarantine(c *gin.Context) {
	if err := h.mgr.RestoreFromQuarantine(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已恢复"})
}

func (h *Handlers) DeleteQuarantine(c *gin.Context) {
	if err := h.mgr.DeleteFromQuarantine(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handlers) ListWhitelist(c *gin.Context) {
	c.JSON(http.StatusOK, h.mgr.ListWhitelist())
}

func (h *Handlers) AddWhitelist(c *gin.Context) {
	var req WhitelistAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	entry := h.mgr.AddWhitelist(req)
	c.JSON(http.StatusCreated, entry)
}

func (h *Handlers) RemoveWhitelist(c *gin.Context) {
	if err := h.mgr.RemoveWhitelist(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已移除"})
}

func (h *Handlers) GetVirusDBStatus(c *gin.Context) {
	c.JSON(http.StatusOK, h.mgr.GetVirusDBStatus())
}

func (h *Handlers) UpdateVirusDB(c *gin.Context) {
	if err := h.mgr.UpdateVirusDB(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "病毒库已更新"})
}

func (h *Handlers) GetMonitorConfig(c *gin.Context) {
	c.JSON(http.StatusOK, h.mgr.GetMonitorConfig())
}

func (h *Handlers) UpdateMonitorConfig(c *gin.Context) {
	var req UpdateMonitorConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.mgr.UpdateMonitorConfig(req)
	c.JSON(http.StatusOK, h.mgr.GetMonitorConfig())
}

func (h *Handlers) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.mgr.GetStats())
}
