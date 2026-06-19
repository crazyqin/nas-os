// Package systemclone - HTTP API 处理器 v2
// 新增路由：RAID1 镜像管理、健康监控、故障转移、在线扩容迁移
package systemclone

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 系统克隆 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	clone := rg.Group("/system-clone")
	{
		// 克隆任务
		clone.GET("/tasks", h.listTasks)
		clone.POST("/tasks", h.createTask)
		clone.GET("/tasks/:id", h.getTask)
		clone.POST("/tasks/:id/start", h.startTask)

		// 镜像管理
		clone.GET("/images", h.listImages)
		clone.POST("/images", h.createImage)

		// 恢复
		clone.POST("/restore", h.restore)

		// PXE 部署
		clone.POST("/pxe", h.configurePXE)

		// 统计
		clone.GET("/stats", h.getStats)

		// ============================================================
		// 新增路由 - RAID1 系统盘镜像
		// ============================================================

		// 镜像管理
		clone.GET("/mirrors", h.listMirrors)
		clone.POST("/mirrors", h.createMirror)
		clone.GET("/mirrors/:id", h.getMirror)
		clone.DELETE("/mirrors/:id", h.deleteMirror)

		// 故障转移
		clone.POST("/mirrors/:id/failover", h.manualFailover)
		clone.GET("/failover-events", h.listFailoverEvents)

		// 在线迁移
		clone.POST("/mirrors/:id/migrate", h.startMigration)
		clone.GET("/migrations", h.listMigrations)
		clone.GET("/migrations/:id", h.getMigration)

		// 在线扩容
		clone.POST("/mirrors/:id/expand", h.startExpand)
		clone.GET("/expansions", h.listExpansions)
		clone.GET("/expansions/:id", h.getExpansion)

		// 健康监控
		clone.GET("/health/disks", h.listDiskHealth)
		clone.GET("/health/disks/:device", h.getDiskHealth)
		clone.POST("/health/monitor/start", h.startHealthMonitor)
		clone.POST("/health/monitor/stop", h.stopHealthMonitor)
		clone.GET("/health/config", h.getHealthConfig)
		clone.PUT("/health/config", h.updateHealthConfig)

		// 镜像统计
		clone.GET("/mirror-stats", h.getMirrorStats)
	}
}

// ============================================================
// 原有处理器
// ============================================================

func (h *Handlers) listTasks(c *gin.Context) {
	tasks := h.manager.ListCloneTasks()
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

func (h *Handlers) createTask(c *gin.Context) {
	var task DiskCloneTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.CreateCloneTask(&task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, task)
}

func (h *Handlers) getTask(c *gin.Context) {
	task, err := h.manager.GetCloneTask(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *Handlers) startTask(c *gin.Context) {
	if err := h.manager.StartClone(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "started"})
}

func (h *Handlers) listImages(c *gin.Context) {
	images := h.manager.ListImages()
	c.JSON(http.StatusOK, gin.H{"images": images})
}

func (h *Handlers) createImage(c *gin.Context) {
	var image BackupImage
	if err := c.ShouldBindJSON(&image); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.CreateImage(&image); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, image)
}

func (h *Handlers) restore(c *gin.Context) {
	var req struct {
		ImageID    string `json:"imageId"`
		TargetDisk string `json:"targetDisk"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	task, err := h.manager.RestoreFromImage(req.ImageID, req.TargetDisk)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, task)
}

func (h *Handlers) configurePXE(c *gin.Context) {
	var config PXEDeployConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.ConfigurePXE(&config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "configured"})
}

func (h *Handlers) getStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.manager.GetStats())
}

// ============================================================
// 新增处理器 - RAID1 镜像管理
// ============================================================

func (h *Handlers) listMirrors(c *gin.Context) {
	mirrors := h.manager.ListMirrors()
	c.JSON(http.StatusOK, gin.H{"mirrors": mirrors})
}

func (h *Handlers) createMirror(c *gin.Context) {
	var req struct {
		PrimaryDisk   string   `json:"primaryDisk" binding:"required"`
		SecondaryDisk string   `json:"secondaryDisk" binding:"required"`
		SpareDisks    []string `json:"spareDisks"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	mirror, err := h.manager.CreateMirror(req.PrimaryDisk, req.SecondaryDisk, req.SpareDisks)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, mirror)
}

func (h *Handlers) getMirror(c *gin.Context) {
	mirror, err := h.manager.GetMirror(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, mirror)
}

func (h *Handlers) deleteMirror(c *gin.Context) {
	if err := h.manager.DeleteMirror(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// ============================================================
// 新增处理器 - 故障转移
// ============================================================

func (h *Handlers) manualFailover(c *gin.Context) {
	mirrorID := c.Param("id")
	var req struct {
		TargetDisk string `json:"targetDisk" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	event, err := h.manager.ManualFailover(mirrorID, req.TargetDisk)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, event)
}

func (h *Handlers) listFailoverEvents(c *gin.Context) {
	events := h.manager.ListFailoverEvents()
	c.JSON(http.StatusOK, gin.H{"events": events})
}

// ============================================================
// 新增处理器 - 在线迁移
// ============================================================

func (h *Handlers) startMigration(c *gin.Context) {
	mirrorID := c.Param("id")
	var req struct {
		SourceDisk string `json:"sourceDisk" binding:"required"`
		TargetDisk string `json:"targetDisk" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	task, err := h.manager.StartMigration(mirrorID, req.SourceDisk, req.TargetDisk)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, task)
}

func (h *Handlers) listMigrations(c *gin.Context) {
	tasks := h.manager.ListMigrationTasks()
	c.JSON(http.StatusOK, gin.H{"migrations": tasks})
}

func (h *Handlers) getMigration(c *gin.Context) {
	task, err := h.manager.GetMigrationTask(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

// ============================================================
// 新增处理器 - 在线扩容
// ============================================================

func (h *Handlers) startExpand(c *gin.Context) {
	mirrorID := c.Param("id")
	var req struct {
		OldDisk string `json:"oldDisk" binding:"required"`
		NewDisk string `json:"newDisk" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	task, err := h.manager.StartExpand(mirrorID, req.OldDisk, req.NewDisk)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, task)
}

func (h *Handlers) listExpansions(c *gin.Context) {
	tasks := h.manager.ListExpandTasks()
	c.JSON(http.StatusOK, gin.H{"expansions": tasks})
}

func (h *Handlers) getExpansion(c *gin.Context) {
	task, err := h.manager.GetExpandTask(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

// ============================================================
// 新增处理器 - 健康监控
// ============================================================

func (h *Handlers) listDiskHealth(c *gin.Context) {
	healthList := h.manager.ListDiskHealth()
	c.JSON(http.StatusOK, gin.H{"disks": healthList})
}

func (h *Handlers) getDiskHealth(c *gin.Context) {
	health, err := h.manager.GetDiskHealth(c.Param("device"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, health)
}

func (h *Handlers) startHealthMonitor(c *gin.Context) {
	if err := h.manager.StartHealthMonitor(); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "started"})
}

func (h *Handlers) stopHealthMonitor(c *gin.Context) {
	h.manager.StopHealthMonitor()
	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

func (h *Handlers) getHealthConfig(c *gin.Context) {
	config := h.manager.GetHealthConfig()
	c.JSON(http.StatusOK, config)
}

func (h *Handlers) updateHealthConfig(c *gin.Context) {
	var config HealthMonitorConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.manager.UpdateHealthConfig(config)
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (h *Handlers) getMirrorStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.manager.GetMirrorStats())
}
