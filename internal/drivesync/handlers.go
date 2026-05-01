// Package drivesync 提供 HTTP API handlers
package drivesync

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler Drive Sync HTTP handler.
type Handler struct {
	manager *Manager
}

// NewHandler 创建 HTTP handler.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	sync := rg.Group("/drivesync")
	{
		// 同步任务
		sync.POST("/sync", h.CreateTask)
		sync.GET("/sync", h.ListTasks)
		sync.GET("/sync/:id", h.GetTask)
		sync.PUT("/sync/:id", h.UpdateTask)
		sync.DELETE("/sync/:id", h.DeleteTask)
		sync.POST("/sync/:id/pause", h.PauseTask)
		sync.POST("/sync/:id/resume", h.ResumeTask)

		// 文件信息和版本
		sync.GET("/files/*path", h.GetFileInfo)
		sync.GET("/files/*path/versions", h.GetFileVersions)
		sync.POST("/files/*path/restore/:versionId", h.RestoreVersion)
		sync.GET("/files/*path/diff/:v1/:v2", h.DiffVersions)

		// 文件锁
		sync.POST("/lock/*path", h.LockFile)
		sync.DELETE("/lock/*path", h.UnlockFile)
		sync.GET("/locks", h.ListLocks)

		// 冲突
		sync.GET("/conflicts", h.ListConflicts)
		sync.POST("/conflicts/:id/resolve", h.ResolveConflict)

		// 协作
		sync.GET("/activity", h.GetActivities)

		// 统计
		sync.GET("/stats", h.GetStats)
	}
}

// ========== 同步任务 API ==========

// CreateTask 创建同步任务.
func (h *Handler) CreateTask(c *gin.Context) {
	var input SyncTaskInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	task, err := h.manager.CreateTask(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, task)
}

// ListTasks 列出同步任务.
func (h *Handler) ListTasks(c *gin.Context) {
	tasks := h.manager.ListTasks()
	c.JSON(http.StatusOK, gin.H{"tasks": tasks, "total": len(tasks)})
}

// GetTask 获取任务详情.
func (h *Handler) GetTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

// UpdateTask 更新任务.
func (h *Handler) UpdateTask(c *gin.Context) {
	id := c.Param("id")
	var input SyncTaskInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	task, err := h.manager.UpdateTask(id, input)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// DeleteTask 删除任务.
func (h *Handler) DeleteTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteTask(id); err != nil {
		if err == ErrSyncTaskRunning {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务已删除"})
}

// PauseTask 暂停任务.
func (h *Handler) PauseTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.PauseTask(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务已暂停"})
}

// ResumeTask 恢复任务.
func (h *Handler) ResumeTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.ResumeTask(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务已恢复"})
}

// ========== 文件信息 API ==========

// GetFileInfo 获取文件信息.
func (h *Handler) GetFileInfo(c *gin.Context) {
	path := c.Param("path")
	if path == "" || path == "/" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径不能为空"})
		return
	}

	// 获取版本历史
	versions := h.manager.GetFileVersions(path)

	// 获取锁信息
	lock := h.manager.GetFileLock(path)

	// 获取评论
	comments := h.manager.GetComments(path)

	info := &FileInfo{
		Path:     path,
		Versions: versions,
		Lock:     lock,
		Comments: comments,
	}

	c.JSON(http.StatusOK, info)
}

// GetFileVersions 获取文件版本历史.
func (h *Handler) GetFileVersions(c *gin.Context) {
	path := c.Param("path")
	versions := h.manager.GetFileVersions(path)
	c.JSON(http.StatusOK, gin.H{"versions": versions, "total": len(versions)})
}

// RestoreVersion 恢复文件版本.
func (h *Handler) RestoreVersion(c *gin.Context) {
	path := c.Param("path")
	versionID := c.Param("versionId")

	version, err := h.manager.RestoreVersion(path, versionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "版本已恢复", "version": version})
}

// DiffVersions 版本对比.
func (h *Handler) DiffVersions(c *gin.Context) {
	path := c.Param("path")
	v1 := c.Param("v1")
	v2 := c.Param("v2")

	diff, err := h.manager.DiffVersions(path, v1, v2)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, diff)
}

// ========== 文件锁 API ==========

// LockFile 锁定文件.
func (h *Handler) LockFile(c *gin.Context) {
	path := c.Param("path")
	var input FileLockInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	lock, err := h.manager.LockFile(path, input)
	if err != nil {
		if err == ErrFileLocked {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, lock)
}

// UnlockFile 解锁文件.
func (h *Handler) UnlockFile(c *gin.Context) {
	path := c.Param("path")
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 user_id 参数"})
		return
	}

	if err := h.manager.UnlockFile(path, userID); err != nil {
		if err == ErrFileNotLocked {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "文件已解锁"})
}

// ListLocks 列出文件锁.
func (h *Handler) ListLocks(c *gin.Context) {
	locks := h.manager.ListLocks()
	c.JSON(http.StatusOK, gin.H{"locks": locks, "total": len(locks)})
}

// ========== 冲突 API ==========

// ListConflicts 列出冲突.
func (h *Handler) ListConflicts(c *gin.Context) {
	conflicts := h.manager.ListConflicts()
	c.JSON(http.StatusOK, gin.H{"conflicts": conflicts, "total": len(conflicts)})
}

// ResolveConflict 解决冲突.
func (h *Handler) ResolveConflict(c *gin.Context) {
	id := c.Param("id")
	var input ConflictResolveInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	resolvedBy := c.Query("resolved_by")
	if resolvedBy == "" {
		resolvedBy = "system"
	}

	if err := h.manager.ResolveConflict(id, input.Resolution, resolvedBy); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "冲突已解决"})
}

// ========== 协作 API ==========

// GetActivities 获取活动流.
func (h *Handler) GetActivities(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	activities := h.manager.GetActivities(limit)
	c.JSON(http.StatusOK, gin.H{"activities": activities, "total": len(activities)})
}

// ========== 统计 API ==========

// GetStats 获取同步统计.
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}
