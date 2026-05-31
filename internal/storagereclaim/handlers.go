package storagereclaim

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Handler 存储回收HTTP处理器。
type Handler struct {
	manager *ReclaimManager
}

// NewHandler 创建处理器。
func NewHandler(manager *ReclaimManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由。
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	group := rg.Group("/storagereclaim")
	{
		// 扫描
		group.POST("/scan", h.StartScan)
		group.GET("/scan/status", h.GetScanStatus)
		group.GET("/scan/result", h.GetScanResult)

		// 文件
		group.GET("/files", h.ListFiles)
		group.GET("/files/:id", h.GetFile)

		// 重复文件
		group.GET("/duplicates", h.GetDuplicates)

		// 回收
		group.POST("/reclaim", h.ReclaimSpace)
		group.GET("/reclaim/history", h.GetReclaimHistory)

		// 回收站
		group.GET("/recycle-bin", h.GetRecycleBin)
		group.GET("/recycle-bin/stats", h.GetRecycleBinStats)
		group.POST("/recycle-bin/:id/restore", h.RestoreFile)
		group.DELETE("/recycle-bin", h.PurgeRecycleBin)

		// 存储分析
		group.GET("/analysis/overview", h.GetStorageOverview)
		group.GET("/analysis/directories", h.GetDirectoryStats)
		group.GET("/analysis/file-types", h.GetFileTypeStats)
		group.GET("/analysis/users", h.GetUserStats)

		// 配置
		group.GET("/config", h.GetConfig)
		group.PUT("/config", h.UpdateConfig)
	}
}

// ========== 扫描 ==========

// StartScan 开始扫描。
func (h *Handler) StartScan(c *gin.Context) {
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// 如果没有 body，使用默认路径
		req.Paths = nil
	}

	result, err := h.manager.Scan(req.Paths)
	if err != nil {
		if err == ErrScanInProgress {
			c.JSON(http.StatusConflict, gin.H{"error": "扫描正在进行中"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetScanStatus 获取扫描状态。
func (h *Handler) GetScanStatus(c *gin.Context) {
	status := h.manager.GetScanStatus()
	c.JSON(http.StatusOK, gin.H{"status": status})
}

// GetScanResult 获取扫描结果。
func (h *Handler) GetScanResult(c *gin.Context) {
	result := h.manager.GetLastScanResult()
	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "尚未执行扫描"})
		return
	}
	c.JSON(http.StatusOK, result)
}

// ========== 文件 ==========

// ListFiles 列出文件。
func (h *Handler) ListFiles(c *gin.Context) {
	junkOnly := c.Query("junk_only") == "true"
	minScore, _ := strconv.ParseFloat(c.Query("min_score"), 64)

	files := h.manager.GetFiles(junkOnly, minScore)
	c.JSON(http.StatusOK, gin.H{
		"files": files,
		"total": len(files),
	})
}

// GetFile 获取文件详情。
func (h *Handler) GetFile(c *gin.Context) {
	id := c.Param("id")
	file, ok := h.manager.GetFile(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}
	c.JSON(http.StatusOK, file)
}

// ========== 重复文件 ==========

// GetDuplicates 获取重复文件组。
func (h *Handler) GetDuplicates(c *gin.Context) {
	groups := h.manager.GetDuplicates()

	var totalWasted int64
	for _, g := range groups {
		totalWasted += g.WastedSize
	}

	c.JSON(http.StatusOK, gin.H{
		"groups":       groups,
		"total_groups": len(groups),
		"total_wasted": totalWasted,
	})
}

// ========== 回收 ==========

// ReclaimSpaceRequest 回收请求。
type ReclaimSpaceRequest struct {
	DryRun   bool     `json:"dry_run"`
	MinScore float64  `json:"min_score"`
	Types    []string `json:"types"`
	MaxFiles int      `json:"max_files"`
}

// ReclaimSpace 回收空间。
func (h *Handler) ReclaimSpace(c *gin.Context) {
	var req ReclaimSpaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 设置默认最低评分
	if req.MinScore == 0 {
		req.MinScore = h.manager.GetConfig().ReclaimThreshold
	}

	// 转换垃圾文件类型
	var junkTypes []JunkFileType
	for _, t := range req.Types {
		junkTypes = append(junkTypes, JunkFileType(t))
	}

	task, err := h.manager.ReclaimSpace(req.MinScore, junkTypes, req.MaxFiles, req.DryRun)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// GetReclaimHistory 获取回收历史。
func (h *Handler) GetReclaimHistory(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	history := h.manager.GetReclaimHistory(limit)
	c.JSON(http.StatusOK, gin.H{
		"history": history,
		"total":   len(history),
	})
}

// ========== 回收站 ==========

// GetRecycleBin 获取回收站内容。
func (h *Handler) GetRecycleBin(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	items := h.manager.GetRecycleBin(limit, offset)
	stats := h.manager.GetRecycleBinStats()

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"stats": stats,
	})
}

// GetRecycleBinStats 获取回收站统计。
func (h *Handler) GetRecycleBinStats(c *gin.Context) {
	stats := h.manager.GetRecycleBinStats()
	c.JSON(http.StatusOK, stats)
}

// RestoreFile 恢复文件。
func (h *Handler) RestoreFile(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RestoreFromRecycleBin(id); err != nil {
		if err == ErrFileNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
			return
		}
		if err == ErrAlreadyDeleted {
			c.JSON(http.StatusGone, gin.H{"error": "文件已被彻底删除"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "文件已恢复"})
}

// PurgeRecycleBin 清空回收站。
func (h *Handler) PurgeRecycleBin(c *gin.Context) {
	olderThanDays, _ := strconv.Atoi(c.DefaultQuery("older_than_days", "0"))

	purgedSize, err := h.manager.PurgeRecycleBin(olderThanDays)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "回收站已清空",
		"purged_size": purgedSize,
	})
}

// ========== 存储分析 ==========

// GetStorageOverview 获取存储总览。
func (h *Handler) GetStorageOverview(c *gin.Context) {
	overview := h.manager.GetStorageOverview()
	c.JSON(http.StatusOK, overview)
}

// GetDirectoryStats 获取目录统计。
func (h *Handler) GetDirectoryStats(c *gin.Context) {
	overview := h.manager.GetStorageOverview()
	c.JSON(http.StatusOK, gin.H{
		"directories": overview.DirectoryStats,
		"total":       len(overview.DirectoryStats),
	})
}

// GetFileTypeStats 获取文件类型统计。
func (h *Handler) GetFileTypeStats(c *gin.Context) {
	overview := h.manager.GetStorageOverview()

	// 按大小排序
	stats := overview.FileTypeStats
	for i := 0; i < len(stats)-1; i++ {
		for j := i + 1; j < len(stats); j++ {
			if stats[j].TotalSize > stats[i].TotalSize {
				stats[i], stats[j] = stats[j], stats[i]
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"file_types": stats,
		"total":      len(stats),
	})
}

// GetUserStats 获取用户统计。
func (h *Handler) GetUserStats(c *gin.Context) {
	overview := h.manager.GetStorageOverview()

	// 按大小排序
	stats := overview.UserStats
	for i := 0; i < len(stats)-1; i++ {
		for j := i + 1; j < len(stats); j++ {
			if stats[j].TotalSize > stats[i].TotalSize {
				stats[i], stats[j] = stats[j], stats[i]
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"users": stats,
		"total": len(stats),
	})
}

// ========== 配置 ==========

// GetConfig 获取配置。
func (h *Handler) GetConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, config)
}

// UpdateConfig 更新配置。
func (h *Handler) UpdateConfig(c *gin.Context) {
	var config ReclaimConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.UpdateConfig(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 返回时隐藏敏感路径（只显示路径列表的前缀）
	safeConfig := config
	if len(safeConfig.ScanPaths) > 0 {
		var masked []string
		for _, p := range safeConfig.ScanPaths {
			parts := strings.SplitN(p, "/", 3)
			if len(parts) > 2 {
				masked = append(masked, "/"+parts[1]+"/***")
			} else {
				masked = append(masked, p)
			}
		}
		safeConfig.ScanPaths = masked
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "配置已更新",
		"config":  safeConfig,
	})
}
