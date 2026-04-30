// Package active Active Backup REST API 处理器
// 提供 /api/v1/backup/active/* 路由
package active

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler Active Backup API 处理器
type Handler struct {
	engine    *Engine
	manager   *BackupManager
	restore   *RestoreManager
	dashboard *DashboardHandler
	logger    *zap.Logger
}

// NewHandler 创建 Active Backup API 处理器
func NewHandler(engine *Engine, manager *BackupManager, restore *RestoreManager, dashboard *DashboardHandler, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		engine:    engine,
		manager:   manager,
		restore:   restore,
		dashboard: dashboard,
		logger:    logger,
	}
}

// RegisterRoutes 注册 Active Backup API 路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	active := rg.Group("/backup/active")
	{
		// 备份任务 CRUD
		active.GET("/jobs", h.listJobs)
		active.POST("/jobs", h.createJob)
		active.GET("/jobs/:jobId", h.getJob)
		active.PUT("/jobs/:jobId", h.updateJob)
		active.DELETE("/jobs/:jobId", h.deleteJob)

		// 备份执行
		active.POST("/jobs/:jobId/run", h.runJob)
		active.POST("/jobs/:jobId/cancel", h.cancelJob)

		// 快照管理
		active.GET("/snapshots", h.listSnapshots)
		active.GET("/snapshots/:snapshotId", h.getSnapshot)

		// 恢复操作
		active.POST("/restore/single-file", h.restoreSingleFile)
		active.POST("/restore/full", h.restoreFull)
		active.GET("/restore/tasks", h.listRestoreTasks)
		active.GET("/restore/tasks/:taskId", h.getRestoreTask)
		active.POST("/restore/tasks/:taskId/cancel", h.cancelRestore)

		// 恢复点
		active.GET("/restore-points", h.listRestorePoints)

		// 引擎状态
		active.GET("/engine/status", h.engineStatus)
		active.GET("/engine/stats", h.engineStats)
		active.GET("/engine/task-runs", h.listTaskRuns)

		// 代理管理
		active.GET("/agents", h.listAgents)
		active.GET("/agents/:agentId", h.getAgent)
		active.DELETE("/agents/:agentId", h.deleteAgent)

		// 仪表板
		active.GET("/dashboard/summary", h.dashboardSummary)
		active.GET("/dashboard/storage-trend", h.dashboardStorageTrend)
		active.GET("/dashboard/restore-points", h.dashboardRestorePoints)
		active.GET("/dashboard/ws", h.dashboardWebSocket)
	}
}

// ==================== 备份任务 CRUD ====================

func (h *Handler) listJobs(c *gin.Context) {
	jobs := h.manager.ListJobs()
	c.JSON(http.StatusOK, gin.H{
		"jobs":  jobs,
		"total": len(jobs),
	})
}

func (h *Handler) createJob(c *gin.Context) {
	var job BackupJob
	if err := c.ShouldBindJSON(&job); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求体: " + err.Error()})
		return
	}

	created, err := h.manager.CreateJob(c.Request.Context(), &job)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, created)
}

func (h *Handler) getJob(c *gin.Context) {
	jobID := c.Param("jobId")
	job, err := h.manager.GetJob(jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, job)
}

func (h *Handler) updateJob(c *gin.Context) {
	jobID := c.Param("jobId")
	var job BackupJob
	if err := c.ShouldBindJSON(&job); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求体: " + err.Error()})
		return
	}
	job.ID = jobID

	existing, err := h.manager.GetJob(jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if existing.Status == BackupStatusRunning {
		c.JSON(http.StatusConflict, gin.H{"error": "任务正在运行中，无法修改"})
		return
	}

	// 更新字段
	existing.Name = job.Name
	existing.Description = job.Description
	if job.Source.Paths != nil {
		existing.Source = job.Source
	}
	if job.Destination.Path != "" {
		existing.Destination = job.Destination
	}
	if job.Policy.Type != "" {
		existing.Policy = job.Policy
	}
	if job.Schedule.Cron != "" {
		existing.Schedule = job.Schedule
	}
	if job.Labels != nil {
		existing.Labels = job.Labels
	}

	c.JSON(http.StatusOK, existing)
}

func (h *Handler) deleteJob(c *gin.Context) {
	jobID := c.Param("jobId")
	if err := h.manager.DeleteJob(c.Request.Context(), jobID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// ==================== 备份执行 ====================

func (h *Handler) runJob(c *gin.Context) {
	jobID := c.Param("jobId")

	var req struct {
		BackupType BackupType `json:"backup_type"`
	}
	// 允许空请求体
	c.ShouldBindJSON(&req)

	if req.BackupType == "" {
		job, err := h.manager.GetJob(jobID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		req.BackupType = job.Policy.Type
	}

	taskRun, err := h.engine.SubmitTask(c.Request.Context(), jobID, req.BackupType)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, taskRun)
}

func (h *Handler) cancelJob(c *gin.Context) {
	jobID := c.Param("jobId")
	if err := h.engine.CancelTask(jobID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

// ==================== 快照管理 ====================

func (h *Handler) listSnapshots(c *gin.Context) {
	jobID := c.Query("job_id")
	snapshots := h.manager.ListSnapshots(jobID)
	c.JSON(http.StatusOK, gin.H{
		"snapshots": snapshots,
		"total":     len(snapshots),
	})
}

func (h *Handler) getSnapshot(c *gin.Context) {
	snapshotID := c.Param("snapshotId")
	snap, err := h.manager.GetSnapshot(snapshotID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snap)
}

// ==================== 恢复操作 ====================

func (h *Handler) restoreSingleFile(c *gin.Context) {
	var req struct {
		JobID      string            `json:"job_id" binding:"required"`
		SnapshotID string            `json:"snapshot_id" binding:"required"`
		Files      []string          `json:"files" binding:"required"`
		TargetPath string            `json:"target_path" binding:"required"`
		Options    RestoreExecOptions `json:"options"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求体: " + err.Error()})
		return
	}

	task, err := h.restore.CreateSingleFileRestore(c.Request.Context(), req.JobID, req.SnapshotID, req.Files, req.TargetPath, req.Options)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 异步执行恢复
	go h.restore.ExecuteRestore(c.Request.Context(), task.ID)

	c.JSON(http.StatusAccepted, task)
}

func (h *Handler) restoreFull(c *gin.Context) {
	var req struct {
		JobID      string            `json:"job_id" binding:"required"`
		SnapshotID string            `json:"snapshot_id" binding:"required"`
		TargetPath string            `json:"target_path" binding:"required"`
		Options    RestoreExecOptions `json:"options"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求体: " + err.Error()})
		return
	}

	task, err := h.restore.CreateFullRestore(c.Request.Context(), req.JobID, req.SnapshotID, req.TargetPath, req.Options)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	go h.restore.ExecuteRestore(c.Request.Context(), task.ID)

	c.JSON(http.StatusAccepted, task)
}

func (h *Handler) listRestoreTasks(c *gin.Context) {
	jobID := c.Query("job_id")
	tasks := h.restore.ListRestoreTasks(jobID)
	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"total": len(tasks),
	})
}

func (h *Handler) getRestoreTask(c *gin.Context) {
	taskID := c.Param("taskId")
	task, err := h.restore.GetRestoreTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *Handler) cancelRestore(c *gin.Context) {
	taskID := c.Param("taskId")
	if err := h.restore.CancelRestore(taskID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

// ==================== 恢复点 ====================

func (h *Handler) listRestorePoints(c *gin.Context) {
	jobID := c.Query("job_id")
	points := h.restore.ListRestorePoints(jobID)
	c.JSON(http.StatusOK, gin.H{
		"restore_points": points,
		"total":          len(points),
	})
}

// ==================== 引擎状态 ====================

func (h *Handler) engineStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"state": string(h.engine.GetState()),
	})
}

func (h *Handler) engineStats(c *gin.Context) {
	stats := h.engine.GetStats()
	c.JSON(http.StatusOK, stats)
}

func (h *Handler) listTaskRuns(c *gin.Context) {
	jobID := c.Query("job_id")
	runs := h.engine.ListTaskRuns(jobID)
	c.JSON(http.StatusOK, gin.H{
		"task_runs": runs,
		"total":     len(runs),
	})
}

// ==================== 代理管理 ====================

func (h *Handler) listAgents(c *gin.Context) {
	registry := h.engine.GetAgentRegistry()
	var status AgentStatus
	if s := c.Query("status"); s != "" {
		status = AgentStatus(s)
	}
	agents := registry.ListAgents(status)
	c.JSON(http.StatusOK, gin.H{
		"agents": agents,
		"total":  len(agents),
	})
}

func (h *Handler) getAgent(c *gin.Context) {
	agentID := c.Param("agentId")
	registry := h.engine.GetAgentRegistry()
	agent, err := registry.GetAgent(agentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, agent)
}

func (h *Handler) deleteAgent(c *gin.Context) {
	agentID := c.Param("agentId")
	registry := h.engine.GetAgentRegistry()
	registry.Unregister(agentID)
	c.JSON(http.StatusOK, gin.H{"status": "unregistered"})
}

// ==================== 仪表板 ====================

func (h *Handler) dashboardSummary(c *gin.Context) {
	if h.dashboard != nil {
		h.dashboard.GetSummary(c)
		return
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{"error": "仪表板未初始化"})
}

func (h *Handler) dashboardStorageTrend(c *gin.Context) {
	if h.dashboard != nil {
		h.dashboard.GetStorageTrend(c)
		return
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{"error": "仪表板未初始化"})
}

func (h *Handler) dashboardRestorePoints(c *gin.Context) {
	if h.dashboard != nil {
		h.dashboard.GetRestorePoints(c)
		return
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{"error": "仪表板未初始化"})
}

func (h *Handler) dashboardWebSocket(c *gin.Context) {
	if h.dashboard != nil {
		h.dashboard.HandleWebSocket(c)
		return
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{"error": "仪表板未初始化"})
}
