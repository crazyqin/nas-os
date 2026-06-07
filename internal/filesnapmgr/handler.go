// Package filesnapmgr 提供 REST API 处理器
package filesnapmgr

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 快照管理 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	snap := r.Group("/snapshots")
	{
		// 快照 CRUD
		snap.POST("", h.CreateSnapshot)
		snap.GET("", h.ListSnapshots)
		snap.GET("/stats", h.GetStats)
		snap.GET("/:id", h.GetSnapshot)
		snap.DELETE("/:id", h.DeleteSnapshot)

		// 快照操作
		snap.POST("/:id/mount", h.MountSnapshot)
		snap.POST("/:id/unmount", h.UnmountSnapshot)
		snap.POST("/:id/rollback", h.RollbackSnapshot)
		snap.POST("/:id/clone", h.CloneSnapshot)

		// 快照比较
		snap.POST("/diff", h.DiffSnapshots)
	}

	// 策略管理
	policy := r.Group("/snapshot-policies")
	{
		policy.POST("", h.CreatePolicy)
		policy.GET("", h.ListPolicies)
		policy.GET("/:id", h.GetPolicy)
		policy.PUT("/:id", h.UpdatePolicy)
		policy.DELETE("/:id", h.DeletePolicy)
		policy.POST("/:id/execute", h.ExecutePolicy)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// CreateSnapshot 创建快照
func (h *Handlers) CreateSnapshot(c *gin.Context) {
	var req struct {
		Volume      string       `json:"volume" binding:"required"`
		Name        string       `json:"name" binding:"required"`
		Description string       `json:"description"`
		Type        SnapshotType `json:"type"`
		Tags        []string     `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	snapshotType := req.Type
	if snapshotType == "" {
		snapshotType = h.manager.GetConfig().DefaultType
	}

	snap, err := h.manager.CreateSnapshot(req.Volume, req.Name, req.Description, snapshotType, req.Tags)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "snapshot created", Data: snap})
}

// ListSnapshots 列出快照
func (h *Handlers) ListSnapshots(c *gin.Context) {
	volume := c.Query("volume")
	snapshotType := SnapshotType(c.Query("type"))

	snapshots := h.manager.ListSnapshots(volume, snapshotType)
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: snapshots})
}

// GetSnapshot 获取快照详情
func (h *Handlers) GetSnapshot(c *gin.Context) {
	id := c.Param("id")
	snap, err := h.manager.GetSnapshot(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: snap})
}

// DeleteSnapshot 删除快照
func (h *Handlers) DeleteSnapshot(c *gin.Context) {
	id := c.Param("id")
	force := c.Query("force") == "true"

	if err := h.manager.DeleteSnapshot(id, force); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "snapshot deleted"})
}

// MountSnapshot 挂载快照
func (h *Handlers) MountSnapshot(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		MountPoint string `json:"mount_point" binding:"required"`
		ReadOnly   bool   `json:"read_only"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	mountReq := &MountRequest{
		SnapshotID: id,
		MountPoint: req.MountPoint,
		ReadOnly:   req.ReadOnly,
	}

	if err := h.manager.MountSnapshot(mountReq); err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "snapshot mounted"})
}

// UnmountSnapshot 卸载快照
func (h *Handlers) UnmountSnapshot(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.UnmountSnapshot(id); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "snapshot unmounted"})
}

// RollbackSnapshot 回滚快照
func (h *Handlers) RollbackSnapshot(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Force          bool `json:"force"`
		CreateSnapshot bool `json:"create_snapshot"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// 使用默认值
		req.Force = false
		req.CreateSnapshot = true
	}

	rollbackReq := &RollbackRequest{
		SnapshotID:     id,
		Force:          req.Force,
		CreateSnapshot: req.CreateSnapshot,
	}

	result, err := h.manager.RollbackSnapshot(rollbackReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "rollback completed", Data: result})
}

// CloneSnapshot 克隆快照
func (h *Handlers) CloneSnapshot(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		CloneName  string `json:"clone_name" binding:"required"`
		MountPoint string `json:"mount_point"`
		ReadOnly   bool   `json:"read_only"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	cloneReq := &CloneRequest{
		SnapshotID: id,
		CloneName:  req.CloneName,
		MountPoint: req.MountPoint,
		ReadOnly:   req.ReadOnly,
	}

	result, err := h.manager.CloneSnapshot(cloneReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "snapshot cloned", Data: result})
}

// DiffSnapshots 比较快照
func (h *Handlers) DiffSnapshots(c *gin.Context) {
	var req struct {
		Snapshot1ID string `json:"snapshot1_id" binding:"required"`
		Snapshot2ID string `json:"snapshot2_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	result, err := h.manager.DiffSnapshots(req.Snapshot1ID, req.Snapshot2ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: result})
}

// GetStats 获取统计
func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}

// ========== 策略 API ==========

// CreatePolicy 创建策略
func (h *Handlers) CreatePolicy(c *gin.Context) {
	var req struct {
		Name      string       `json:"name" binding:"required"`
		Volume    string       `json:"volume" binding:"required"`
		Type      SnapshotType `json:"type"`
		Schedule  string       `json:"schedule" binding:"required"`
		Retention Retention    `json:"retention"`
		Tags      []string     `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	if req.Type == "" {
		req.Type = h.manager.GetConfig().DefaultType
	}
	if req.Retention == (Retention{}) {
		req.Retention = DefaultRetention()
	}

	policy, err := h.manager.CreatePolicy(req.Name, req.Volume, req.Type, req.Schedule, req.Retention)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "policy created", Data: policy})
}

// ListPolicies 列出策略
func (h *Handlers) ListPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: policies})
}

// GetPolicy 获取策略
func (h *Handlers) GetPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: policy})
}

// UpdatePolicy 更新策略
func (h *Handlers) UpdatePolicy(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Enabled   *bool      `json:"enabled"`
		Schedule  *string    `json:"schedule"`
		Retention *Retention `json:"retention"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	if err := h.manager.UpdatePolicy(id, req.Enabled, req.Schedule, req.Retention); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "policy updated"})
}

// DeletePolicy 删除策略
func (h *Handlers) DeletePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePolicy(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "policy deleted"})
}

// ExecutePolicy 执行策略
func (h *Handlers) ExecutePolicy(c *gin.Context) {
	id := c.Param("id")
	snap, err := h.manager.ExecutePolicy(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "policy executed", Data: snap})
}
