package immutablesnap

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers REST API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由（前缀 /api/v1/snapshots）
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	snap := r.Group("/snapshots")
	{
		// 快照管理
		snap.GET("", h.ListSnapshots)
		snap.POST("", h.CreateSnapshot)
		snap.GET("/:id", h.GetSnapshot)
		snap.DELETE("/:id", h.DeleteSnapshot)
		snap.POST("/:id/lock", h.LockSnapshot)
		snap.POST("/:id/unlock", h.UnlockSnapshot)
		snap.POST("/:id/verify", h.VerifySnapshot)
		snap.POST("/:id/rollback", h.RollbackSnapshot)

		// 策略管理
		snap.GET("/policies", h.ListPolicies)
		snap.POST("/policies", h.CreatePolicy)
		snap.GET("/policies/:id", h.GetPolicy)
		snap.PUT("/policies/:id", h.UpdatePolicy)
		snap.DELETE("/policies/:id", h.DeletePolicy)
		snap.POST("/policies/:id/run", h.RunPolicy)

		// 保留规则
		snap.GET("/retention-rules", h.ListRetentionRules)
		snap.POST("/retention-rules", h.CreateRetentionRule)
		snap.DELETE("/retention-rules/:id", h.DeleteRetentionRule)
		snap.POST("/retention/apply", h.ApplyRetention)

		// 复制任务
		snap.GET("/replication-jobs", h.ListReplicationJobs)
		snap.POST("/replication-jobs", h.CreateReplicationJob)
		snap.GET("/replication-jobs/:id", h.GetReplicationJob)
		snap.POST("/replication-jobs/:id/start", h.StartReplication)

		// 威胁检测
		snap.POST("/threats", h.ReportThreat)
		snap.GET("/threats", h.GetThreatEvents)
		snap.GET("/alert-config", h.GetAlertConfig)
		snap.PUT("/alert-config", h.SetAlertConfig)

		// 统计
		snap.GET("/stats", h.GetStats)
		snap.GET("/space-usage", h.GetSpaceUsage)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ==================== 快照处理器 ====================

// ListSnapshots GET /api/v1/snapshots
func (h *Handlers) ListSnapshots(c *gin.Context) {
	statusFilter := SnapshotStatus(c.Query("status"))
	datasetFilter := c.Query("dataset")
	snapshots := h.manager.ListSnapshots(statusFilter, datasetFilter)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "ok",
		Data:    snapshots,
	})
}

// CreateSnapshot POST /api/v1/snapshots
func (h *Handlers) CreateSnapshot(c *gin.Context) {
	var req struct {
		DatasetName    string   `json:"dataset_name" binding:"required"`
		SourcePath     string   `json:"source_path"`
		StoragePath    string   `json:"storage_path"`
		RetentionHours int      `json:"retention_hours"`
		Tags           []string `json:"tags"`
		AutoLock       bool     `json:"auto_lock"`
		WORM           bool     `json:"worm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	snap, err := h.manager.CreateSnapshot(req.DatasetName, req.SourcePath, req.StoragePath, req.RetentionHours, req.Tags, req.WORM)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 2, Message: err.Error()})
		return
	}

	if req.AutoLock {
		if err := h.manager.LockSnapshot(snap.ID, req.WORM); err != nil {
			c.JSON(http.StatusInternalServerError, response{Code: 3, Message: "created but failed to lock: " + err.Error()})
			return
		}
		snap, _ = h.manager.GetSnapshot(snap.ID)
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "snapshot created",
		Data:    snap,
	})
}

// GetSnapshot GET /api/v1/snapshots/:id
func (h *Handlers) GetSnapshot(c *gin.Context) {
	id := c.Param("id")
	snap, err := h.manager.GetSnapshot(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "ok",
		Data:    snap,
	})
}

// DeleteSnapshot DELETE /api/v1/snapshots/:id
func (h *Handlers) DeleteSnapshot(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteSnapshot(id); err != nil {
		c.JSON(http.StatusForbidden, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "snapshot deleted",
	})
}

// LockSnapshot POST /api/v1/snapshots/:id/lock
func (h *Handlers) LockSnapshot(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		WORM bool `json:"worm"`
	}
	// 允许空 body
	c.ShouldBindJSON(&req)

	if err := h.manager.LockSnapshot(id, req.WORM); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	snap, _ := h.manager.GetSnapshot(id)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "snapshot locked",
		Data:    snap,
	})
}

// UnlockSnapshot POST /api/v1/snapshots/:id/unlock
func (h *Handlers) UnlockSnapshot(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.UnlockSnapshot(id); err != nil {
		c.JSON(http.StatusForbidden, response{Code: 1, Message: err.Error()})
		return
	}

	snap, _ := h.manager.GetSnapshot(id)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "snapshot unlocked",
		Data:    snap,
	})
}

// VerifySnapshot POST /api/v1/snapshots/:id/verify
func (h *Handlers) VerifySnapshot(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Checksum string `json:"checksum" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	result, err := h.manager.VerifyAndUpdateStatus(id, req.Checksum)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 2, Message: err.Error()})
		return
	}

	status := http.StatusOK
	if !result.Valid {
		status = http.StatusConflict
	}

	c.JSON(status, response{
		Code:    0,
		Message: "verification complete",
		Data:    result,
	})
}

// RollbackSnapshot POST /api/v1/snapshots/:id/rollback
func (h *Handlers) RollbackSnapshot(c *gin.Context) {
	id := c.Param("id")

	result, err := h.manager.RollbackToSnapshot(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "rollback completed",
		Data:    result,
	})
}

// ==================== 策略处理器 ====================

// ListPolicies GET /api/v1/snapshots/policies
func (h *Handlers) ListPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "ok",
		Data:    policies,
	})
}

// CreatePolicy POST /api/v1/snapshots/policies
func (h *Handlers) CreatePolicy(c *gin.Context) {
	var policy SnapshotPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	if err := h.manager.CreatePolicy(&policy); err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 2, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "policy created",
		Data:    policy,
	})
}

// GetPolicy GET /api/v1/snapshots/policies/:id
func (h *Handlers) GetPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "ok",
		Data:    policy,
	})
}

// UpdatePolicy PUT /api/v1/snapshots/policies/:id
func (h *Handlers) UpdatePolicy(c *gin.Context) {
	id := c.Param("id")
	var policy SnapshotPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	if err := h.manager.UpdatePolicy(id, &policy); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 2, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "policy updated",
		Data:    policy,
	})
}

// DeletePolicy DELETE /api/v1/snapshots/policies/:id
func (h *Handlers) DeletePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePolicy(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "policy deleted",
	})
}

// RunPolicy POST /api/v1/snapshots/policies/:id/run
func (h *Handlers) RunPolicy(c *gin.Context) {
	id := c.Param("id")

	snap, err := h.manager.RunPolicy(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "policy executed",
		Data:    snap,
	})
}

// ==================== 保留规则处理器 ====================

// ListRetentionRules GET /api/v1/snapshots/retention-rules
func (h *Handlers) ListRetentionRules(c *gin.Context) {
	rules := h.manager.ListRetentionRules()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "ok",
		Data:    rules,
	})
}

// CreateRetentionRule POST /api/v1/snapshots/retention-rules
func (h *Handlers) CreateRetentionRule(c *gin.Context) {
	var rule RetentionRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	if err := h.manager.CreateRetentionRule(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 2, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "retention rule created",
		Data:    rule,
	})
}

// DeleteRetentionRule DELETE /api/v1/snapshots/retention-rules/:id
func (h *Handlers) DeleteRetentionRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteRetentionRule(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "retention rule deleted",
	})
}

// ApplyRetention POST /api/v1/snapshots/retention/apply
func (h *Handlers) ApplyRetention(c *gin.Context) {
	count, err := h.manager.ApplyRetention()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "retention applied",
		Data: gin.H{
			"deleted_count": count,
		},
	})
}

// ==================== 复制任务处理器 ====================

// ListReplicationJobs GET /api/v1/snapshots/replication-jobs
func (h *Handlers) ListReplicationJobs(c *gin.Context) {
	snapshotID := c.Query("snapshot_id")
	jobs := h.manager.ListReplicationJobs(snapshotID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "ok",
		Data:    jobs,
	})
}

// CreateReplicationJob POST /api/v1/snapshots/replication-jobs
func (h *Handlers) CreateReplicationJob(c *gin.Context) {
	var req struct {
		SnapshotID     string `json:"snapshot_id" binding:"required"`
		RemoteEndpoint string `json:"remote_endpoint" binding:"required"`
		RemotePath     string `json:"remote_path" binding:"required"`
		MaxRetries     int    `json:"max_retries"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	job, err := h.manager.CreateReplicationJob(req.SnapshotID, req.RemoteEndpoint, req.RemotePath, req.MaxRetries)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 2, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "replication job created",
		Data:    job,
	})
}

// GetReplicationJob GET /api/v1/snapshots/replication-jobs/:id
func (h *Handlers) GetReplicationJob(c *gin.Context) {
	id := c.Param("id")
	job, err := h.manager.GetReplicationJob(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "ok",
		Data:    job,
	})
}

// StartReplication POST /api/v1/snapshots/replication-jobs/:id/start
func (h *Handlers) StartReplication(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StartReplication(id); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "replication started",
	})
}

// ==================== 威胁检测处理器 ====================

// ReportThreat POST /api/v1/snapshots/threats
func (h *Handlers) ReportThreat(c *gin.Context) {
	var req struct {
		Level        ThreatLevel `json:"level" binding:"required"`
		ModifiedRate float64     `json:"modified_rate"`
		Description  string      `json:"description"`
		DatasetName  string      `json:"dataset_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	event, err := h.manager.ReportThreat(req.Level, req.ModifiedRate, req.Description, req.DatasetName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 2, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "threat reported",
		Data:    event,
	})
}

// GetThreatEvents GET /api/v1/snapshots/threats
func (h *Handlers) GetThreatEvents(c *gin.Context) {
	events := h.manager.GetThreatEvents()

	// 可选：按级别过滤
	levelFilter := c.Query("level")
	if levelFilter != "" {
		var filtered []ThreatEvent
		for _, e := range events {
			if string(e.Level) == levelFilter {
				filtered = append(filtered, e)
			}
		}
		events = filtered
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "ok",
		Data:    events,
	})
}

// GetAlertConfig GET /api/v1/snapshots/alert-config
func (h *Handlers) GetAlertConfig(c *gin.Context) {
	config := h.manager.GetAlertConfig()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "ok",
		Data:    config,
	})
}

// SetAlertConfig PUT /api/v1/snapshots/alert-config
func (h *Handlers) SetAlertConfig(c *gin.Context) {
	var config AlertConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	h.manager.SetAlertConfig(config)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "alert config updated",
		Data:    config,
	})
}

// ==================== 统计处理器 ====================

// GetStats GET /api/v1/snapshots/stats
func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "ok",
		Data:    stats,
	})
}

// GetSpaceUsage GET /api/v1/snapshots/space-usage
func (h *Handlers) GetSpaceUsage(c *gin.Context) {
	dataset := c.Query("dataset")
	usage := h.manager.GetSpaceUsage(dataset)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "ok",
		Data:    usage,
	})
}

// ==================== 辅助函数 ====================

// parseLimitOffset 解析分页参数
func parseLimitOffset(c *gin.Context, defaultLimit int) (limit, offset int) {
	limit = defaultLimit
	offset = 0

	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	return limit, offset
}
