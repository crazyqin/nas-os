// Package smartdedup 提供内容感知的智能文件去重功能
package smartdedup

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 智能去重 HTTP 处理器.
type Handlers struct {
	mgr *Manager
}

// NewHandlers 创建 HTTP 处理器.
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{mgr: mgr}
}

// RegisterRoutes 注册路由到 Gin 路由组.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/smart-dedup")
	{
		// 扫描
		g.POST("/scan", h.Scan)
		g.POST("/scan/cancel", h.CancelScan)

		// 去重
		g.POST("/dedup", h.Dedup)
		g.POST("/dedup/cancel", h.CancelDedup)

		// 重复组
		g.GET("/duplicates", h.GetDuplicates)

		// 统计
		g.GET("/stats", h.GetStats)

		// 条目
		g.GET("/entries", h.ListEntries)
		g.GET("/entries/:id", h.GetEntry)

		// 引用
		g.GET("/refs", h.ListRefs)
		g.GET("/refs/:id", h.GetRef)

		// 配置
		g.GET("/config", h.GetConfig)
		g.PUT("/config", h.UpdateConfig)

		// 后端检测
		g.GET("/backend/detect", h.DetectBackend)
	}
}

// Scan 执行去重扫描.
func (h *Handlers) Scan(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "invalid request: " + err.Error()})
		return
	}

	result, err := h.mgr.Scan(context.Background(), req.Paths)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

// CancelScan 取消扫描.
func (h *Handlers) CancelScan(c *gin.Context) {
	h.mgr.CancelScan()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "scan cancelled"})
}

// Dedup 执行去重操作.
func (h *Handlers) Dedup(c *gin.Context) {
	var req DedupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "invalid request: " + err.Error()})
		return
	}

	result, err := h.mgr.Dedup(context.Background(), req.Groups)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

// CancelDedup 取消去重.
func (h *Handlers) CancelDedup(c *gin.Context) {
	h.mgr.CancelDedup()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "dedup cancelled"})
}

// GetDuplicates 获取重复组列表.
func (h *Handlers) GetDuplicates(c *gin.Context) {
	groups := h.mgr.GetDuplicateGroups()
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"total":  len(groups),
			"groups": groups,
		},
	})
}

// GetStats 获取统计信息.
func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.mgr.GetStats()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stats})
}

// ListEntries 列出所有条目.
func (h *Handlers) ListEntries(c *gin.Context) {
	entries := h.mgr.ListEntries()
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"total":   len(entries),
			"entries": entries,
		},
	})
}

// GetEntry 获取指定条目.
func (h *Handlers) GetEntry(c *gin.Context) {
	entry, ok := h.mgr.GetEntry(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": "entry not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": entry})
}

// ListRefs 列出所有引用.
func (h *Handlers) ListRefs(c *gin.Context) {
	refs := h.mgr.ListRefCounts()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": refs})
}

// GetRef 获取指定引用.
func (h *Handlers) GetRef(c *gin.Context) {
	ref, ok := h.mgr.GetRefCount(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": "ref not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": ref})
}

// GetConfig 获取配置.
func (h *Handlers) GetConfig(c *gin.Context) {
	cfg := h.mgr.Config()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": cfg})
}

// UpdateConfig 更新配置.
func (h *Handlers) UpdateConfig(c *gin.Context) {
	var req updateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "invalid request: " + err.Error()})
		return
	}

	cfg := h.mgr.Config()
	// 复制一份配置
	updated := *cfg

	if req.Enabled != nil {
		updated.Enabled = *req.Enabled
	}
	if req.Backend != nil {
		updated.Backend = *req.Backend
	}
	if req.Mode != nil {
		updated.Mode = *req.Mode
	}
	if req.Action != nil {
		updated.Action = *req.Action
	}
	if req.ScanPaths != nil {
		updated.ScanPaths = *req.ScanPaths
	}
	if req.ExcludePaths != nil {
		updated.ExcludePaths = *req.ExcludePaths
	}
	if req.ExcludePatterns != nil {
		updated.ExcludePatterns = *req.ExcludePatterns
	}
	if req.MinFileSize != nil {
		updated.MinFileSize = *req.MinFileSize
	}
	if req.MaxFileSize != nil {
		updated.MaxFileSize = *req.MaxFileSize
	}
	if req.ScheduleCron != nil {
		updated.ScheduleCron = *req.ScheduleCron
	}
	if req.ScheduleEnabled != nil {
		updated.ScheduleEnabled = *req.ScheduleEnabled
	}
	if req.RealtimeEnabled != nil {
		updated.RealtimeEnabled = *req.RealtimeEnabled
	}
	if req.MaxWorkers != nil {
		updated.MaxWorkers = *req.MaxWorkers
	}
	if req.MaxMemoryMB != nil {
		updated.MaxMemoryMB = *req.MaxMemoryMB
	}
	if req.HashCache != nil {
		updated.HashCache = *req.HashCache
	}
	if req.DryRun != nil {
		updated.DryRun = *req.DryRun
	}
	if req.VerifyAfter != nil {
		updated.VerifyAfter = *req.VerifyAfter
	}
	if req.MaxRefPerFile != nil {
		updated.MaxRefPerFile = *req.MaxRefPerFile
	}

	if err := h.mgr.UpdateConfig(&updated); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": &updated})
}

// DetectBackend 检测存储后端.
func (h *Handlers) DetectBackend(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		path = "/data"
	}

	backend, err := h.mgr.DetectBackend(path)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{
				"path":    path,
				"backend": "",
				"error":   err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"path":    path,
			"backend": backend,
		},
	})
}
