package zfsmaintenance

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler ZFS 维护 HTTP 处理器
type Handler struct {
	maintainer *ZFSMaintainer
}

// NewHandler 创建 HTTP 处理器
func NewHandler(maintainer *ZFSMaintainer) *Handler {
	return &Handler{maintainer: maintainer}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	zfs := rg.Group("/zfs-maintenance")
	{
		// 存储池管理
		zfs.GET("/pools", h.listPools)
		zfs.GET("/pools/:id", h.getPool)
		zfs.POST("/pools", h.registerPool)
		zfs.GET("/pools/:id/health", h.checkPoolHealth)
		zfs.GET("/pools/:id/report", h.getReport)
		zfs.GET("/pools/:id/compression", h.analyzeCompression)

		// Scrub 扫描
		zfs.POST("/scrub/:poolId", h.startScrub)
		zfs.GET("/scrub/:id", h.getScrubStatus)
		zfs.GET("/scrubs", h.listScrubTasks)

		// 自动快照
		zfs.POST("/snapshots", h.createAutoSnapshot)
		zfs.GET("/snapshots/:id", h.getAutoSnapshot)
		zfs.GET("/snapshots", h.listAutoSnapshots)
		zfs.POST("/snapshots/:id/cleanup", h.cleanupSnapshots)

		// 快照复制
		zfs.POST("/replications", h.createReplication)
		zfs.GET("/replications", h.listReplications)

		// ARC 缓存
		zfs.GET("/arc", h.getARCStats)
		zfs.POST("/arc/optimize", h.optimizeARC)
	}
}

// listPools 列出存储池
func (h *Handler) listPools(c *gin.Context) {
	pools := h.maintainer.ListPools()
	c.JSON(http.StatusOK, gin.H{"pools": pools, "total": len(pools)})
}

// getPool 获取存储池
func (h *Handler) getPool(c *gin.Context) {
	pool, err := h.maintainer.GetPool(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pool)
}

// registerPoolReq 注册存储池请求
type registerPoolReq struct {
	ID            string  `json:"id" binding:"required"`
	Name          string  `json:"name" binding:"required"`
	TotalSize     int64   `json:"total_size"`
	UsedSize      int64   `json:"used_size"`
	FreeSize      int64   `json:"free_size"`
	Compression   float64 `json:"compression"`
	Deduplication float64 `json:"deduplication"`
}

// registerPool 注册存储池
func (h *Handler) registerPool(c *gin.Context) {
	var req registerPoolReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pool := &ZPool{
		ID:            req.ID,
		Name:          req.Name,
		TotalSize:     req.TotalSize,
		UsedSize:      req.UsedSize,
		FreeSize:      req.FreeSize,
		Compression:   req.Compression,
		Deduplication: req.Deduplication,
	}

	if err := h.maintainer.RegisterPool(pool); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, pool)
}

// checkPoolHealth 检查存储池健康
func (h *Handler) checkPoolHealth(c *gin.Context) {
	health, err := h.maintainer.CheckPoolHealth(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"health": health})
}

// getReport 获取维护报告
func (h *Handler) getReport(c *gin.Context) {
	report, err := h.maintainer.GenerateReport(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, report)
}

// analyzeCompression 分析压缩
func (h *Handler) analyzeCompression(c *gin.Context) {
	result, err := h.maintainer.AnalyzeCompression(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// startScrub 启动清理
func (h *Handler) startScrub(c *gin.Context) {
	task, err := h.maintainer.StartScrub(c.Param("poolId"))
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, task)
}

// getScrubStatus 获取清理状态
func (h *Handler) getScrubStatus(c *gin.Context) {
	task, err := h.maintainer.GetScrubStatus(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

// listScrubTasks 列出清理任务
func (h *Handler) listScrubTasks(c *gin.Context) {
	poolID := c.Query("pool_id")
	tasks := h.maintainer.ListScrubTasks(poolID)
	c.JSON(http.StatusOK, gin.H{"tasks": tasks, "total": len(tasks)})
}

// createAutoSnapshotReq 创建自动快照请求
type createAutoSnapshotReq struct {
	ID        string `json:"id" binding:"required"`
	Name      string `json:"name" binding:"required"`
	Dataset   string `json:"dataset" binding:"required"`
	Schedule  string `json:"schedule" binding:"required"`
	Recursive bool   `json:"recursive"`
	MaxCount  int    `json:"max_count"`
	Enabled   bool   `json:"enabled"`
}

// createAutoSnapshot 创建自动快照策略
func (h *Handler) createAutoSnapshot(c *gin.Context) {
	var req createAutoSnapshotReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy := &AutoSnapshot{
		ID:        req.ID,
		Name:      req.Name,
		Dataset:   req.Dataset,
		Schedule:  req.Schedule,
		Recursive: req.Recursive,
		MaxCount:  req.MaxCount,
		Enabled:   req.Enabled,
	}

	if err := h.maintainer.CreateAutoSnapshot(policy); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, policy)
}

// getAutoSnapshot 获取自动快照策略
func (h *Handler) getAutoSnapshot(c *gin.Context) {
	policy, err := h.maintainer.GetAutoSnapshot(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policy)
}

// listAutoSnapshots 列出自动快照策略
func (h *Handler) listAutoSnapshots(c *gin.Context) {
	policies := h.maintainer.ListAutoSnapshots()
	c.JSON(http.StatusOK, gin.H{"policies": policies, "total": len(policies)})
}

// cleanupSnapshots 清理过期快照
func (h *Handler) cleanupSnapshots(c *gin.Context) {
	removed, err := h.maintainer.Cleanup(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"removed": removed})
}

// createReplicationReq 创建复制任务请求
type createReplicationReq struct {
	ID            string `json:"id" binding:"required"`
	SourcePool    string `json:"source_pool" binding:"required"`
	TargetPool    string `json:"target_pool" binding:"required"`
	SourceDataset string `json:"source_dataset"`
	TargetDataset string `json:"target_dataset"`
}

// createReplication 创建复制任务
func (h *Handler) createReplication(c *gin.Context) {
	var req createReplicationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rep := &SnapshotReplication{
		ID:            req.ID,
		SourcePool:    req.SourcePool,
		TargetPool:    req.TargetPool,
		SourceDataset: req.SourceDataset,
		TargetDataset: req.TargetDataset,
	}

	if err := h.maintainer.CreateReplication(rep); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, rep)
}

// listReplications 列出复制任务
func (h *Handler) listReplications(c *gin.Context) {
	reps := h.maintainer.ListReplications()
	c.JSON(http.StatusOK, gin.H{"replications": reps, "total": len(reps)})
}

// getARCStats 获取 ARC 统计
func (h *Handler) getARCStats(c *gin.Context) {
	stats := h.maintainer.GetARCStats()
	c.JSON(http.StatusOK, stats)
}

// optimizeARC 优化 ARC
func (h *Handler) optimizeARC(c *gin.Context) {
	stats, err := h.maintainer.OptimizeARC()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}
