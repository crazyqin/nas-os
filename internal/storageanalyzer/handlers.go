// Package storageanalyzer 存储分析 - HTTP API
package storageanalyzer

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group("/storage-analyzer")
	{
		group.POST("/scan", h.StartScan)
		group.GET("/scans", h.ListScans)
		group.GET("/scans/:id", h.GetScanResult)
		group.GET("/scans/:id/duplicates", h.GetDuplicates)
		group.GET("/trend", h.GetTrend)
	}
}

// StartScan 启动扫描
func (h *Handler) StartScan(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	c.ShouldBindJSON(&req)
	if req.Path == "" {
		req.Path = "/"
	}

	result, err := h.manager.StartScan(req.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"code": 0, "message": "扫描已启动", "data": result})
}

// ListScans 列出扫描
func (h *Handler) ListScans(c *gin.Context) {
	scans := h.manager.ListScans()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": scans})
}

// GetScanResult 获取扫描结果
func (h *Handler) GetScanResult(c *gin.Context) {
	id := c.Param("id")
	result, err := h.manager.GetScanResult(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}

// GetDuplicates 获取重复文件
func (h *Handler) GetDuplicates(c *gin.Context) {
	id := c.Param("id")
	minSize := int64(1024 * 1024) // 默认1MB
	if s := c.Query("min_size"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			minSize = v
		}
	}
	dupes, err := h.manager.GetDuplicates(id, minSize)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": dupes})
}

// GetTrend 获取容量趋势
func (h *Handler) GetTrend(c *gin.Context) {
	days := 30
	if d := c.Query("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			days = v
		}
	}
	trend := h.manager.GetTrend(days)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": trend})
}
