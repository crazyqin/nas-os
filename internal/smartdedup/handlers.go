// Package smartdedup 提供内容感知的智能文件去重功能
package smartdedup

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 智能去重 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	sd := r.Group("/smart-dedup")
	{
		// 扫描操作
		sd.POST("/scan", h.scan)
		sd.POST("/scan/cancel", h.cancelScan)

		// 去重操作
		sd.POST("/dedup", h.dedup)
		sd.POST("/dedup/cancel", h.cancelDedup)

		// 重复组
		sd.GET("/duplicates", h.getDuplicates)

		// 统计
		sd.GET("/stats", h.getStats)

		// 条目管理
		sd.GET("/entries", h.listEntries)
		sd.GET("/entries/:id", h.getEntry)

		// 引用计数
		sd.GET("/refs", h.listRefs)
		sd.GET("/refs/:hash", h.getRef)

		// 配置管理
		sd.GET("/config", h.getConfig)
		sd.PUT("/config", h.updateConfig)

		// 存储后端
		sd.GET("/backend/detect", h.detectBackend)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ScanRequest 扫描请求.
type ScanRequest struct {
	Paths []string `json:"paths"`
}

// scan 扫描重复文件.
func (h *Handlers) scan(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 使用默认路径
		req.Paths = []string{}
	}

	result, err := h.manager.Scan(c.Request.Context(), req.Paths)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "scan completed",
		Data:    result,
	})
}

// cancelScan 取消扫描.
func (h *Handlers) cancelScan(c *gin.Context) {
	h.manager.CancelScan()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "scan cancelled",
	})
}

// DedupRequest 去重请求.
type DedupRequest struct {
	Groups []DuplicateGroup `json:"groups"`
}

// dedup 执行去重.
func (h *Handlers) dedup(c *gin.Context) {
	var req DedupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.Dedup(c.Request.Context(), req.Groups)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "dedup completed",
		Data:    result,
	})
}

// cancelDedup 取消去重.
func (h *Handlers) cancelDedup(c *gin.Context) {
	h.manager.CancelDedup()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "dedup cancelled",
	})
}

// getDuplicates 获取重复组.
func (h *Handlers) getDuplicates(c *gin.Context) {
	groups := h.manager.GetDuplicateGroups()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(groups),
			"groups": groups,
		},
	})
}

// getStats 获取统计.
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// listEntries 列出条目.
func (h *Handlers) listEntries(c *gin.Context) {
	entries := h.manager.ListEntries()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(entries),
			"entries": entries,
		},
	})
}

// getEntry 获取条目.
func (h *Handlers) getEntry(c *gin.Context) {
	id := c.Param("id")
	entry, ok := h.manager.GetEntry(id)
	if !ok {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: "entry not found",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    entry,
	})
}

// listRefs 列出引用计数.
func (h *Handlers) listRefs(c *gin.Context) {
	refs := h.manager.ListRefCounts()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(refs),
			"refs":  refs,
		},
	})
}

// getRef 获取引用计数.
func (h *Handlers) getRef(c *gin.Context) {
	hash := c.Param("hash")
	ref, ok := h.manager.GetRefCount(hash)
	if !ok {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: "ref not found",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    ref,
	})
}

// getConfig 获取配置.
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.manager.Config()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    cfg,
	})
}

// updateConfigRequest 更新配置请求.
type updateConfigRequest struct {
	Enabled          *bool           `json:"enabled,omitempty"`
	Backend          *StorageBackend `json:"backend,omitempty"`
	Mode             *DedupMode      `json:"mode,omitempty"`
	Action           *DedupAction    `json:"action,omitempty"`
	ScanPaths        []string        `json:"scanPaths,omitempty"`
	ExcludePaths     []string        `json:"excludePaths,omitempty"`
	ExcludePatterns  []string        `json:"excludePatterns,omitempty"`
	MinFileSize      *int64          `json:"minFileSize,omitempty"`
	MaxFileSize      *int64          `json:"maxFileSize,omitempty"`
	ScheduleCron     *string         `json:"scheduleCron,omitempty"`
	ScheduleEnabled  *bool           `json:"scheduleEnabled,omitempty"`
	RealtimeEnabled  *bool           `json:"realtimeEnabled,omitempty"`
	MaxWorkers       *int            `json:"maxWorkers,omitempty"`
	MaxMemoryMB      *int            `json:"maxMemoryMB,omitempty"`
	DryRun           *bool           `json:"dryRun,omitempty"`
	VerifyAfter      *bool           `json:"verifyAfter,omitempty"`
}

// updateConfig 更新配置.
func (h *Handlers) updateConfig(c *gin.Context) {
	var req updateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	cfg := h.manager.Config()
	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
	}
	if req.Backend != nil {
		cfg.Backend = *req.Backend
	}
	if req.Mode != nil {
		cfg.Mode = *req.Mode
	}
	if req.Action != nil {
		cfg.Action = *req.Action
	}
	if req.ScanPaths != nil {
		cfg.ScanPaths = req.ScanPaths
	}
	if req.ExcludePaths != nil {
		cfg.ExcludePaths = req.ExcludePaths
	}
	if req.ExcludePatterns != nil {
		cfg.ExcludePatterns = req.ExcludePatterns
	}
	if req.MinFileSize != nil {
		cfg.MinFileSize = *req.MinFileSize
	}
	if req.MaxFileSize != nil {
		cfg.MaxFileSize = *req.MaxFileSize
	}
	if req.ScheduleCron != nil {
		cfg.ScheduleCron = *req.ScheduleCron
	}
	if req.ScheduleEnabled != nil {
		cfg.ScheduleEnabled = *req.ScheduleEnabled
	}
	if req.RealtimeEnabled != nil {
		cfg.RealtimeEnabled = *req.RealtimeEnabled
	}
	if req.MaxWorkers != nil {
		cfg.MaxWorkers = *req.MaxWorkers
	}
	if req.MaxMemoryMB != nil {
		cfg.MaxMemoryMB = *req.MaxMemoryMB
	}
	if req.DryRun != nil {
		cfg.DryRun = *req.DryRun
	}
	if req.VerifyAfter != nil {
		cfg.VerifyAfter = *req.VerifyAfter
	}

	if err := h.manager.UpdateConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "config updated",
		Data:    cfg,
	})
}

// detectBackend 检测存储后端.
func (h *Handlers) detectBackend(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		path = "/"
	}

	backend, err := h.manager.DetectBackend(path)
	if err != nil {
		c.JSON(http.StatusOK, response{
			Code:    0,
			Message: "detection failed: " + err.Error(),
			Data: gin.H{
				"path":    path,
				"backend": "unknown",
			},
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"path":    path,
			"backend": backend,
		},
	})
}
