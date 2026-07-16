package immusnap

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 不可变快照 HTTP API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	snap := r.Group("/immutable-snapshots")
	{
		snap.GET("", h.ListSnapshots)
		snap.POST("", h.CreateSnapshot)
		snap.GET("/stats", h.GetStats)
		snap.GET("/policies", h.GetPolicy)
		snap.PUT("/policies", h.UpdatePolicy)
		snap.POST("/verify", h.VerifySnapshot)
		snap.POST("/threat", h.ReportThreat)
		snap.POST("/:id/lock", h.LockSnapshot)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ListSnapshots GET /api/v1/immutable-snapshots.
func (h *Handlers) ListSnapshots(c *gin.Context) {
	statusFilter := SnapshotStatus(c.Query("status"))
	snapshots := h.manager.ListSnapshots(statusFilter)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "ok",
		Data:    snapshots,
	})
}

// CreateSnapshot POST /api/v1/immutable-snapshots.
func (h *Handlers) CreateSnapshot(c *gin.Context) {
	var req struct {
		DatasetName    string   `json:"dataset_name" binding:"required"`
		SourcePath     string   `json:"source_path"`
		StoragePath    string   `json:"storage_path"`
		RetentionHours int      `json:"retention_hours"`
		Tags           []string `json:"tags"`
		AutoLock       bool     `json:"auto_lock"` // 创建后立即锁定
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	snap, err := h.manager.CreateSnapshot(req.DatasetName, req.SourcePath, req.StoragePath, req.RetentionHours, req.Tags)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 2, Message: err.Error()})
		return
	}

	// 自动锁定
	if req.AutoLock {
		if err := h.manager.Lock(snap.ID); err != nil {
			c.JSON(http.StatusInternalServerError, response{Code: 3, Message: "created but failed to lock: " + err.Error()})
			return
		}
		// 重新获取锁定后的快照
		snap, _ = h.manager.GetSnapshot(snap.ID)
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "snapshot created",
		Data:    snap,
	})
}

// LockSnapshot POST /api/v1/immutable-snapshots/:id/lock.
func (h *Handlers) LockSnapshot(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.Lock(id); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	snap, _ := h.manager.GetSnapshot(id)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "snapshot locked (immutable)",
		Data:    snap,
	})
}

// GetPolicy GET /api/v1/immutable-snapshots/policies.
func (h *Handlers) GetPolicy(c *gin.Context) {
	policy := h.manager.GetPolicy()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "ok",
		Data:    policy,
	})
}

// UpdatePolicy PUT /api/v1/immutable-snapshots/policies.
func (h *Handlers) UpdatePolicy(c *gin.Context) {
	var policy RetentionPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	if err := h.manager.UpdatePolicy(policy); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 2, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "policy updated",
		Data:    policy,
	})
}

// VerifySnapshot POST /api/v1/immutable-snapshots/verify.
func (h *Handlers) VerifySnapshot(c *gin.Context) {
	var req struct {
		SnapshotID string `json:"snapshot_id" binding:"required"`
		Checksum   string `json:"checksum" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	result, err := h.manager.VerifyIntegrity(req.SnapshotID, req.Checksum)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 2, Message: err.Error()})
		return
	}

	status := http.StatusOK
	if !result.Valid {
		status = http.StatusConflict // 409 表示数据冲突
	}

	c.JSON(status, response{
		Code:    0,
		Message: "verification complete",
		Data:    result,
	})
}

// GetStats GET /api/v1/immutable-snapshots/stats.
func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "ok",
		Data:    stats,
	})
}

// ReportThreat POST /api/v1/immutable-snapshots/threat.
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
