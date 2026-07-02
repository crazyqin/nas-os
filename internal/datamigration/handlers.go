// Package datamigration HTTP API 处理器 (gin)
package datamigration

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器.
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group("/data-migration")
	{
		group.GET("/migrations", h.ListMigrations)
		group.POST("/migrations", h.CreateMigration)
		group.GET("/migrations/:id", h.GetMigration)
		group.POST("/migrations/:id/start", h.StartMigration)
		group.POST("/migrations/:id/pause", h.PauseMigration)
		group.POST("/migrations/:id/cancel", h.CancelMigration)
		group.GET("/migrations/:id/progress", h.GetProgress)
		group.GET("/sources", h.ListSources)
		group.GET("/targets", h.ListTargets)
	}
}

// ListMigrations 获取迁移任务列表.
func (h *Handler) ListMigrations(c *gin.Context) {
	status := c.Query("status")
	migrations := h.manager.ListMigrations(status)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": migrations})
}

// CreateMigration 创建迁移任务.
func (h *Handler) CreateMigration(c *gin.Context) {
	var req CreateMigrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	migration, err := h.manager.CreateMigration(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": migration})
}

// GetMigration 获取任务详情.
func (h *Handler) GetMigration(c *gin.Context) {
	id := c.Param("id")
	migration, err := h.manager.GetMigration(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": migration})
}

// StartMigration 启动迁移.
func (h *Handler) StartMigration(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StartMigration(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "迁移已启动"})
}

// PauseMigration 暂停迁移.
func (h *Handler) PauseMigration(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.PauseMigration(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "迁移已暂停"})
}

// CancelMigration 取消迁移.
func (h *Handler) CancelMigration(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.CancelMigration(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "迁移已取消"})
}

// GetProgress 获取进度.
func (h *Handler) GetProgress(c *gin.Context) {
	id := c.Param("id")
	migration, err := h.manager.GetMigration(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}

	progressData := gin.H{
		"status":       migration.Status,
		"progress":     migration.Progress,
		"started_at":   migration.StartedAt,
		"completed_at": migration.CompletedAt,
		"error":        migration.Error,
		"elapsed":      0,
	}
	if migration.StartedAt != nil {
		end := time.Now()
		if migration.CompletedAt != nil {
			end = *migration.CompletedAt
		}
		progressData["elapsed"] = int(end.Sub(*migration.StartedAt).Seconds())
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": progressData})
}

// ListSources 获取支持的数据源类型.
func (h *Handler) ListSources(c *gin.Context) {
	sources := []gin.H{
		{"type": "local", "name": "本地存储", "description": "本机磁盘/挂载点"},
		{"type": "nfs", "name": "NFS", "description": "网络文件系统"},
		{"type": "smb", "name": "SMB/CIFS", "description": "Windows共享/Samba"},
		{"type": "s3", "name": "S3兼容", "description": "AWS S3/MinIO/其他S3兼容存储"},
		{"type": "rsync", "name": "Rsync", "description": "Rsync远程同步"},
		{"type": "synology", "name": "群晖NAS", "description": "Synology DiskStation"},
		{"type": "truenas", "name": "TrueNAS", "description": "TrueNAS/FreeNAS"},
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": sources})
}

// ListTargets 获取支持的目标类型.
func (h *Handler) ListTargets(c *gin.Context) {
	targets := []gin.H{
		{"type": "local", "name": "本地存储", "description": "本机磁盘/挂载点"},
		{"type": "nfs", "name": "NFS", "description": "网络文件系统"},
		{"type": "smb", "name": "SMB/CIFS", "description": "Windows共享/Samba"},
		{"type": "s3", "name": "S3兼容", "description": "AWS S3/MinIO/其他S3兼容存储"},
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": targets})
}
