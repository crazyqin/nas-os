// Package zfsenhanced ZFS增强管理模块 - HTTP API 处理器
package zfsenhanced

import (
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

	// 池管理
	r.GET("/pools", h.ListPools)
	r.GET("/pools/:name", h.GetPoolStatus)

	// Scrub调度
	r.GET("/scrub/policies", h.ListScrubPolicies)
	r.POST("/scrub/policies", h.CreateScrubPolicy)
	r.PUT("/scrub/policies/:id", h.UpdateScrubPolicy)
	r.DELETE("/scrub/policies/:id", h.DeleteScrubPolicy)
	r.GET("/scrub/policies/:id", h.GetScrubPolicy)
	r.POST("/scrub/run/:pool", h.RunScrub)
	r.POST("/scrub/cancel/:pool", h.CancelScrub)
	r.GET("/scrub/jobs", h.GetScrubJobs)

	// 快照生命周期
	r.GET("/snapshot/lifecycle", h.ListSnapshotLifecycles)
	r.POST("/snapshot/lifecycle", h.CreateSnapshotLifecycle)
	r.PUT("/snapshot/lifecycle/:id", h.UpdateSnapshotLifecycle)
	r.DELETE("/snapshot/lifecycle/:id", h.DeleteSnapshotLifecycle)
	r.GET("/snapshot/lifecycle/:id", h.GetSnapshotLifecycle)
	r.POST("/snapshot/lifecycle/:id/execute", h.ExecuteSnapshotLifecycle)
	r.GET("/snapshot/templates", h.ListSnapshotTemplates)

	// 自动修复
	r.POST("/repair/detect/:pool", h.DetectAndRepair)
	r.GET("/repair/tasks", h.GetRepairTasks)

	// 性能监控
	r.GET("/metrics/realtime/:pool", h.GetRealtimeMetrics)
	r.GET("/metrics/history/:pool", h.GetMetricsHistory)
	r.GET("/metrics/recommendations/:pool", h.GetPerformanceRecommendations)

	// 存储池分析
	r.GET("/analysis/:pool", h.AnalyzePool)
	r.GET("/analysis/:pool/cached", h.GetPoolAnalysis)
	r.GET("/analysis/:pool/capacity", h.GetCapacityTrend)
	r.GET("/analysis/:pool/fragmentation", h.GetFragmentation)

	// 告警
	r.GET("/alerts", h.GetAlerts)
	r.POST("/alerts/:id/ack", h.AcknowledgeAlert)
	r.POST("/alerts/:id/resolve", h.ResolveAlert)
	r.GET("/alerts/config", h.GetAlertConfig)
	r.PUT("/alerts/config", h.UpdateAlertConfig)
}

// ========== 池管理 ==========

func (h *Handler) ListPools(c *gin.Context) {
	pools, err := h.mgr.ListPools(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: pools})
}

func (h *Handler) GetPoolStatus(c *gin.Context) {
	name := c.Param("name")
	pool, err := h.mgr.GetPoolStatus(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: pool})
}

// ========== Scrub调度 ==========

func (h *Handler) ListScrubPolicies(c *gin.Context) {
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: h.mgr.ListScrubPolicies()})
}

func (h *Handler) CreateScrubPolicy(c *gin.Context) {
	var req ScrubSchedulePolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: "请求参数错误: " + err.Error()})
		return
	}
	policy, err := h.mgr.CreateScrubPolicy(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: policy})
}

func (h *Handler) UpdateScrubPolicy(c *gin.Context) {
	id := c.Param("id")
	var req ScrubSchedulePolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: "请求参数错误: " + err.Error()})
		return
	}
	policy, err := h.mgr.UpdateScrubPolicy(id, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: policy})
}

func (h *Handler) DeleteScrubPolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.DeleteScrubPolicy(id); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok"})
}

func (h *Handler) GetScrubPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.mgr.GetScrubPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: policy})
}

func (h *Handler) RunScrub(c *gin.Context) {
	pool := c.Param("pool")
	priority := ScrubPriorityNormal
	if p := c.Query("priority"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			priority = ScrubPriority(v)
		}
	}
	job, err := h.mgr.RunScrub(c.Request.Context(), pool, priority)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "Scrub 已启动", Data: job})
}

func (h *Handler) CancelScrub(c *gin.Context) {
	pool := c.Param("pool")
	if err := h.mgr.CancelScrub(c.Request.Context(), pool); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "Scrub 已取消"})
}

func (h *Handler) GetScrubJobs(c *gin.Context) {
	pool := c.Query("pool")
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: h.mgr.GetScrubJobs(pool)})
}

// ========== 快照生命周期 ==========

func (h *Handler) ListSnapshotLifecycles(c *gin.Context) {
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: h.mgr.ListSnapshotLifecycles()})
}

func (h *Handler) CreateSnapshotLifecycle(c *gin.Context) {
	var req SnapshotLifecyclePolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: "请求参数错误: " + err.Error()})
		return
	}
	policy, err := h.mgr.CreateSnapshotLifecycle(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: policy})
}

func (h *Handler) UpdateSnapshotLifecycle(c *gin.Context) {
	id := c.Param("id")
	var req SnapshotLifecyclePolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: "请求参数错误: " + err.Error()})
		return
	}
	policy, err := h.mgr.UpdateSnapshotLifecycle(id, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: policy})
}

func (h *Handler) DeleteSnapshotLifecycle(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.DeleteSnapshotLifecycle(id); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok"})
}

func (h *Handler) GetSnapshotLifecycle(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.mgr.GetSnapshotLifecycle(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: policy})
}

func (h *Handler) ExecuteSnapshotLifecycle(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.ExecuteSnapshotLifecycle(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "快照策略执行完成"})
}

func (h *Handler) ListSnapshotTemplates(c *gin.Context) {
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: h.mgr.ListSnapshotTemplates()})
}

// ========== 自动修复 ==========

func (h *Handler) DetectAndRepair(c *gin.Context) {
	pool := c.Param("pool")
	tasks, err := h.mgr.DetectAndRepair(c.Request.Context(), pool)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: tasks})
}

func (h *Handler) GetRepairTasks(c *gin.Context) {
	pool := c.Query("pool")
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: h.mgr.GetRepairTasks(pool)})
}

// ========== 性能监控 ==========

func (h *Handler) GetRealtimeMetrics(c *gin.Context) {
	pool := c.Param("pool")
	metrics, err := h.mgr.GetRealtimeMetrics(c.Request.Context(), pool)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: metrics})
}

func (h *Handler) GetMetricsHistory(c *gin.Context) {
	pool := c.Param("pool")
	limit := 100
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: h.mgr.GetMetricsHistory(pool, limit)})
}

func (h *Handler) GetPerformanceRecommendations(c *gin.Context) {
	pool := c.Param("pool")
	recs, err := h.mgr.GetPerformanceRecommendations(c.Request.Context(), pool)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: recs})
}

// ========== 存储池分析 ==========

func (h *Handler) AnalyzePool(c *gin.Context) {
	pool := c.Param("pool")
	analysis, err := h.mgr.AnalyzePool(c.Request.Context(), pool)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: analysis})
}

func (h *Handler) GetPoolAnalysis(c *gin.Context) {
	pool := c.Param("pool")
	analysis, err := h.mgr.GetPoolAnalysis(pool)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: analysis})
}

func (h *Handler) GetCapacityTrend(c *gin.Context) {
	pool := c.Param("pool")
	trend, err := h.mgr.GetCapacityTrend(c.Request.Context(), pool)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: trend})
}

func (h *Handler) GetFragmentation(c *gin.Context) {
	pool := c.Param("pool")
	frag, err := h.mgr.GetFragmentation(c.Request.Context(), pool)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: gin.H{"pool": pool, "fragmentation": frag}})
}

// ========== 告警 ==========

func (h *Handler) GetAlerts(c *gin.Context) {
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: h.mgr.GetAlerts()})
}

func (h *Handler) AcknowledgeAlert(c *gin.Context) {
	id := c.Param("id")
	ackedBy := c.Query("acked_by")
	if ackedBy == "" {
		ackedBy = "admin"
	}
	if err := h.mgr.AcknowledgeAlert(id, ackedBy); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "告警已确认"})
}

func (h *Handler) ResolveAlert(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.ResolveAlert(id); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "告警已解决"})
}

func (h *Handler) GetAlertConfig(c *gin.Context) {
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: h.mgr.GetAlertConfig()})
}

func (h *Handler) UpdateAlertConfig(c *gin.Context) {
	var config AlertConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: "请求参数错误: " + err.Error()})
		return
	}
	h.mgr.UpdateAlertConfig(config)
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "告警配置已更新"})
}
