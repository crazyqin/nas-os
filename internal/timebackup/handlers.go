// Package timebackup 提供 REST API 处理器
package timebackup

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 时间备份 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/v1/timebackup.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	tb := r.Group("/timebackup")
	{
		tb.GET("/tasks", h.listTasks)
		tb.POST("/tasks", h.createTask)
		tb.GET("/tasks/:id", h.getTask)
		tb.DELETE("/tasks/:id", h.deleteTask)
		tb.POST("/tasks/:id/trigger", h.triggerSnapshot)

		tb.GET("/snapshots", h.listVersions)
		tb.GET("/snapshots/:id", h.getSnapshotsByTask)
		tb.DELETE("/snapshots/:id", h.deleteSnapshot)

		tb.POST("/restore", h.restoreSnapshot)
		tb.POST("/diff", h.diffSnapshots)
	}
}

// listTasks 列出所有备份任务.
func (h *Handlers) listTasks(c *gin.Context) {
	tasks := h.manager.ListTasks()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(tasks),
			"tasks": tasks,
		},
	})
}

// createTask 创建备份任务.
func (h *Handlers) createTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	// 设置默认策略
	if req.Strategy == "" {
		req.Strategy = StrategyCopy
	}
	// 设置默认保留策略
	if req.Retention == nil {
		req.Retention = &RetentionPolicy{
			Mode:     RetentionByCount,
			MaxCount: 10,
		}
	}

	task, err := h.manager.CreateTask(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "任务创建成功",
		Data:    task,
	})
}

// getTask 获取备份任务详情.
func (h *Handlers) getTask(c *gin.Context) {
	taskID := c.Param("id")

	task, err := h.manager.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    task,
	})
}

// deleteTask 删除备份任务.
func (h *Handlers) deleteTask(c *gin.Context) {
	taskID := c.Param("id")

	if err := h.manager.DeleteTask(taskID); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "任务删除成功",
	})
}

// triggerSnapshot 手动触发快照.
func (h *Handlers) triggerSnapshot(c *gin.Context) {
	taskID := c.Param("id")

	snap, err := h.manager.CreateSnapshot(taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "快照创建成功",
		Data:    snap,
	})
}

// listVersions 列出版本.
func (h *Handlers) listVersions(c *gin.Context) {
	taskID := c.Query("task_id")
	path := c.Query("path")
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	versions := h.manager.ListVersions(taskID, path, limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(versions),
			"versions": versions,
		},
	})
}

// getSnapshotsByTask 获取任务的所有快照.
func (h *Handlers) getSnapshotsByTask(c *gin.Context) {
	taskID := c.Param("id")

	// 参数 id 可能是 taskID 或 snapshotID
	// 先按 taskID 查询快照列表
	snapshots := h.manager.GetSnapshotsByTask(taskID)
	if len(snapshots) > 0 {
		c.JSON(http.StatusOK, response{
			Code:    0,
			Message: "success",
			Data: gin.H{
				"task_id":   taskID,
				"total":     len(snapshots),
				"snapshots": snapshots,
			},
		})
		return
	}

	c.JSON(http.StatusNotFound, response{
		Code:    1,
		Message: "未找到任务或快照: " + taskID,
	})
}

// deleteSnapshot 删除快照.
func (h *Handlers) deleteSnapshot(c *gin.Context) {
	snapshotID := c.Param("id")

	if err := h.manager.DeleteSnapshot(snapshotID); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "快照删除成功",
	})
}

// restoreSnapshot 恢复快照.
func (h *Handlers) restoreSnapshot(c *gin.Context) {
	var req RestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	if err := h.manager.RestoreSnapshot(req.SnapshotID, req.TargetPath, req.Overwrite); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "恢复成功",
		Data: gin.H{
			"snapshot_id": req.SnapshotID,
			"target_path": req.TargetPath,
		},
	})
}

// diffSnapshots 对比两个快照.
func (h *Handlers) diffSnapshots(c *gin.Context) {
	var req DiffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	result, err := h.manager.DiffSnapshots(req.SnapshotOld, req.SnapshotNew)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}
