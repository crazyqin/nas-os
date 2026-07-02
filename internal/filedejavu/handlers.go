// Package filedejavu 提供重复文件智能检测功能
package filedejavu

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// Handlers HTTP API 处理器.
type Handlers struct {
	mu       sync.RWMutex
	detector *Detector
}

// NewHandlers 创建处理器.
func NewHandlers() *Handlers {
	return &Handlers{}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	fd := r.Group("/filedejavu")
	{
		// 扫描操作
		fd.POST("/scan", h.startScan)
		fd.POST("/scan/cancel", h.cancelScan)
		fd.GET("/scan/status", h.scanStatus)

		// 结果查询
		fd.GET("/duplicates", h.getDuplicates)
		fd.GET("/duplicates/:id", h.getDuplicateGroup)

		// 去重操作
		fd.POST("/dedup", h.batchDedup)
		fd.POST("/dedup/dry-run", h.dryRunDedup)

		// 配置
		fd.GET("/config", h.getConfig)
		fd.PUT("/config", h.updateConfig)
	}
}

// startScan 启动扫描.
func (h *Handlers) startScan(c *gin.Context) {
	var config ScanConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "invalid request body: " + err.Error(),
		})
		return
	}

	// 验证路径
	if len(config.Paths) == 0 {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "paths is required",
		})
		return
	}

	// 应用默认值
	defaults := DefaultScanConfig()
	if config.Threshold <= 0 {
		config.Threshold = defaults.Threshold
	}
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = defaults.MaxWorkers
	}

	h.mu.Lock()
	h.detector = NewDetector(&config)
	detector := h.detector
	h.mu.Unlock()

	// 异步执行扫描
	go func() {
		detector.Scan(c.Request.Context())
	}()

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"message": "scan started",
			"status":  "running",
		},
	})
}

// cancelScan 取消扫描.
func (h *Handlers) cancelScan(c *gin.Context) {
	h.mu.RLock()
	detector := h.detector
	h.mu.RUnlock()

	if detector == nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "no active scan",
		})
		return
	}

	detector.Cancel()
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"message": "scan cancelled",
		},
	})
}

// scanStatus 获取扫描状态.
func (h *Handlers) scanStatus(c *gin.Context) {
	h.mu.RLock()
	detector := h.detector
	h.mu.RUnlock()

	if detector == nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data: gin.H{
				"status": "idle",
			},
		})
		return
	}

	result := detector.Result()
	if result == nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data: gin.H{
				"status": "idle",
			},
		})
		return
	}

	result.mu.RLock()
	defer result.mu.RUnlock()

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"status":         result.Status,
			"totalFiles":     result.TotalFiles,
			"totalSize":      result.TotalSize,
			"duplicateCount": result.DuplicateCount,
			"savingsTotal":   result.SavingsTotal,
			"duration":       result.Duration.String(),
			"startTime":      result.StartTime,
			"endTime":        result.EndTime,
			"error":          result.Error,
		},
	})
}

// getDuplicates 获取所有重复组.
func (h *Handlers) getDuplicates(c *gin.Context) {
	h.mu.RLock()
	detector := h.detector
	h.mu.RUnlock()

	if detector == nil || detector.Result() == nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data: gin.H{
				"groups": []interface{}{},
				"total":  0,
			},
		})
		return
	}

	result := detector.Result()
	groups := result.GetGroups()

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"groups":         groups,
			"total":          len(groups),
			"duplicateCount": result.DuplicateCount,
			"savingsTotal":   result.SavingsTotal,
		},
	})
}

// getDuplicateGroup 获取单个重复组详情.
func (h *Handlers) getDuplicateGroup(c *gin.Context) {
	groupID := c.Param("id")

	h.mu.RLock()
	detector := h.detector
	h.mu.RUnlock()

	if detector == nil || detector.Result() == nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "no scan result available",
		})
		return
	}

	result := detector.Result()
	for _, group := range result.GetGroups() {
		if group.ID == groupID {
			c.JSON(http.StatusOK, APIResponse{
				Success: true,
				Data:    group,
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, APIResponse{
		Success: false,
		Error:   "group not found",
	})
}

// batchDedup 批量去重.
func (h *Handlers) batchDedup(c *gin.Context) {
	var req BatchDedupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "invalid request body: " + err.Error(),
		})
		return
	}

	// 非 dry-run 模式需要确认
	if !req.DryRun {
		confirm := c.GetHeader("X-Confirm-Dedup")
		if confirm != "true" {
			c.JSON(http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "non-dry-run mode requires X-Confirm-Dedup: true header",
			})
			return
		}
	}

	h.mu.RLock()
	detector := h.detector
	h.mu.RUnlock()

	if detector == nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "no scan result available",
		})
		return
	}

	result, err := detector.BatchDedup(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    result,
	})
}

// dryRunDedup 试运行去重.
func (h *Handlers) dryRunDedup(c *gin.Context) {
	var req BatchDedupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "invalid request body: " + err.Error(),
		})
		return
	}

	req.DryRun = true

	h.mu.RLock()
	detector := h.detector
	h.mu.RUnlock()

	if detector == nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "no scan result available",
		})
		return
	}

	result, err := detector.BatchDedup(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    result,
	})
}

// getConfig 获取配置.
func (h *Handlers) getConfig(c *gin.Context) {
	h.mu.RLock()
	detector := h.detector
	h.mu.RUnlock()

	config := DefaultScanConfig()
	if detector != nil {
		config = detector.Config()
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    config,
	})
}

// updateConfig 更新配置.
func (h *Handlers) updateConfig(c *gin.Context) {
	var config ScanConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "invalid request body: " + err.Error(),
		})
		return
	}

	h.mu.Lock()
	if h.detector != nil {
		// 创建新的检测器，保留旧结果
		oldResult := h.detector.Result()
		h.detector = NewDetector(&config)
		if oldResult != nil {
			h.detector.mu.Lock()
			h.detector.result = oldResult
			h.detector.mu.Unlock()
		}
	} else {
		h.detector = NewDetector(&config)
	}
	h.mu.Unlock()

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    config,
	})
}
