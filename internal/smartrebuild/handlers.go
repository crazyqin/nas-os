// Package smartrebuild 智能RAID重建引擎 - HTTP API 处理器
package smartrebuild

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// APIResponse 统一API响应格式
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handler HTTP处理器
type Handler struct {
	mgr *Manager
}

// RegisterRoutes 注册HTTP路由
func RegisterRoutes(r *gin.RouterGroup, mgr *Manager) {
	h := &Handler{mgr: mgr}

	// 重建任务管理
	r.POST("/jobs", h.CreateJob)
	r.GET("/jobs", h.ListJobs)
	r.GET("/jobs/:id", h.GetJob)
	r.POST("/jobs/:id/start", h.StartJob)
	r.POST("/jobs/:id/pause", h.PauseJob)
	r.POST("/jobs/:id/cancel", h.CancelJob)
	r.PUT("/jobs/:id/progress", h.UpdateProgress)

	// 智能优先级
	r.POST("/prioritize", h.PrioritizeSegments)
	r.PUT("/hot-score/:segmentId", h.UpdateHotScore)

	// 并行调度
	r.POST("/schedule-parallel", h.ScheduleParallel)
	r.GET("/active-jobs", h.GetActiveJobCount)

	// 进度和预测
	r.GET("/progress/snapshot", h.GetProgressSnapshot)

	// 性能保护
	r.POST("/throttle/:jobId", h.ThrottleRebuild)
	r.PUT("/io-metrics", h.SetIOMetrics)

	// 调度计划
	r.POST("/schedules", h.CreateSchedule)
	r.GET("/schedules", h.ListSchedules)
	r.GET("/schedules/:id", h.GetSchedule)
	r.DELETE("/schedules/:id", h.DeleteSchedule)
}

// ========== 任务管理 ==========

// CreateJobRequest 创建任务请求
type CreateJobRequest struct {
	PoolName    string     `json:"pool_name" binding:"required"`
	TargetDisk  DiskInfo   `json:"target_disk" binding:"required"`
	SourceDisks []DiskInfo `json:"source_disks" binding:"required,min=1"`
}

func (h *Handler) CreateJob(c *gin.Context) {
	var req CreateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}

	job, err := h.mgr.CreateJob(c.Request.Context(), req.PoolName, req.TargetDisk, req.SourceDisks)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Code: -1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{Code: 0, Message: "ok", Data: job})
}

func (h *Handler) ListJobs(c *gin.Context) {
	jobs := h.mgr.ListJobs(c.Request.Context())
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: jobs})
}

func (h *Handler) GetJob(c *gin.Context) {
	id := c.Param("id")
	job, err := h.mgr.GetJob(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: job})
}

func (h *Handler) StartJob(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.StartJob(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "job started"})
}

func (h *Handler) PauseJob(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.PauseJob(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "job paused"})
}

func (h *Handler) CancelJob(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.CancelJob(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "job cancelled"})
}

// UpdateProgressRequest 更新进度请求
type UpdateProgressRequest struct {
	RebuiltBytes int64 `json:"rebuilt_bytes"`
	CurrentSpeed int64 `json:"current_speed"`
}

func (h *Handler) UpdateProgress(c *gin.Context) {
	id := c.Param("id")
	var req UpdateProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}

	if err := h.mgr.UpdateProgress(id, req.RebuiltBytes, req.CurrentSpeed); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Code: -1, Message: err.Error()})
		return
	}

	job, _ := h.mgr.GetJob(c.Request.Context(), id)
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: job})
}

// ========== 智能优先级 ==========

// PrioritizeRequest 优先级排序请求
type PrioritizeRequest struct {
	Segments []DataSegment `json:"segments" binding:"required"`
}

func (h *Handler) PrioritizeSegments(c *gin.Context) {
	var req PrioritizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}

	result := h.mgr.PrioritizeSegments(req.Segments)
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: result})
}

// UpdateHotScoreRequest 更新热度请求
type UpdateHotScoreRequest struct {
	Score float64 `json:"score" binding:"required,min=0,max=1"`
}

func (h *Handler) UpdateHotScore(c *gin.Context) {
	segmentID := c.Param("segmentId")
	var req UpdateHotScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}

	h.mgr.UpdateHotScore(segmentID, req.Score)
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok"})
}

// ========== 并行调度 ==========

// ScheduleParallelRequest 并行调度请求
type ScheduleParallelRequest struct {
	JobIDs []string `json:"job_ids" binding:"required,min=1"`
}

func (h *Handler) ScheduleParallel(c *gin.Context) {
	var req ScheduleParallelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}

	// 获取指定的任务
	jobs := make([]*RebuildJob, 0)
	for _, id := range req.JobIDs {
		job, err := h.mgr.GetJob(c.Request.Context(), id)
		if err != nil {
			continue
		}
		jobs = append(jobs, job)
	}

	if err := h.mgr.ScheduleParallel(c.Request.Context(), jobs); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: jobs})
}

func (h *Handler) GetActiveJobCount(c *gin.Context) {
	count := h.mgr.GetActiveJobCount()
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: gin.H{"active_jobs": count}})
}

// ========== 进度和预测 ==========

func (h *Handler) GetProgressSnapshot(c *gin.Context) {
	snapshot := h.mgr.GetProgressSnapshot()
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: snapshot})
}

// ========== 性能保护 ==========

// ThrottleRequest 限速请求
type ThrottleRequest struct {
	BusinessIOPS int64 `json:"business_iops"`
}

func (h *Handler) ThrottleRebuild(c *gin.Context) {
	jobID := c.Param("jobId")
	var req ThrottleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}

	speed, err := h.mgr.ThrottleRebuild(jobID, req.BusinessIOPS)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Code: -1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data: gin.H{
			"throttled_speed_bytes": speed,
			"throttled_speed_mbps":  speed / (1024 * 1024),
		},
	})
}

func (h *Handler) SetIOMetrics(c *gin.Context) {
	var metrics IOMetrics
	if err := c.ShouldBindJSON(&metrics); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}

	h.mgr.SetIOMetrics(metrics)
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok"})
}

// ========== 调度计划 ==========

func (h *Handler) CreateSchedule(c *gin.Context) {
	var schedule RebuildSchedule
	if err := c.ShouldBindJSON(&schedule); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}

	created, err := h.mgr.CreateSchedule(c.Request.Context(), &schedule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Code: -1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{Code: 0, Message: "ok", Data: created})
}

func (h *Handler) ListSchedules(c *gin.Context) {
	schedules := h.mgr.ListSchedules(c.Request.Context())
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: schedules})
}

func (h *Handler) GetSchedule(c *gin.Context) {
	id := c.Param("id")
	schedule, err := h.mgr.GetSchedule(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: schedule})
}

func (h *Handler) DeleteSchedule(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.DeleteSchedule(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "deleted"})
}

// ========== 辅助函数 ==========

// parseIDParam 解析ID参数
func parseIDParam(c *gin.Context, param string) (string, error) {
	id := c.Param(param)
	if id == "" {
		return "", fmt.Errorf("missing %s parameter", param)
	}
	return id, nil
}

// parseIntParam 解析整数参数
func parseIntParam(c *gin.Context, param string, defaultVal int) int {
	val := c.Query(param)
	if val == "" {
		return defaultVal
	}
	intVal, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return intVal
}
