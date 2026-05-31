// Package datalifecycle - 数据生命周期 HTTP API 处理器
package datalifecycle

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 数据生命周期 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/v1/lifecycle 路由组
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	lifecycle := r.Group("/lifecycle")
	{
		// 策略管理
		lifecycle.POST("/policies", h.createPolicy)
		lifecycle.GET("/policies", h.listPolicies)
		lifecycle.GET("/policies/:id", h.getPolicy)
		lifecycle.PUT("/policies/:id", h.updatePolicy)
		lifecycle.DELETE("/policies/:id", h.deletePolicy)

		// 数据记录
		lifecycle.POST("/records", h.createRecord)
		lifecycle.GET("/records", h.listRecords)
		lifecycle.GET("/records/:id", h.getRecord)
		lifecycle.POST("/records/:id/transition", h.transitionPhase)

		// 合规保留
		lifecycle.POST("/holds", h.createHold)
		lifecycle.GET("/holds", h.listHolds)
		lifecycle.POST("/holds/:id/release", h.releaseHold)

		// 数据迁移
		lifecycle.POST("/migrations", h.createMigration)
		lifecycle.GET("/migrations", h.listMigrations)
		lifecycle.GET("/migrations/:id", h.getMigration)
		lifecycle.POST("/migrations/:id/start", h.startMigration)

		// 数据销毁
		lifecycle.POST("/destructions", h.createDestruction)
		lifecycle.GET("/destructions/:id", h.getDestruction)
		lifecycle.POST("/destructions/:id/approve", h.approveDestruction)
		lifecycle.POST("/destructions/:id/execute", h.executeDestruction)

		// 策略模板
		lifecycle.POST("/templates", h.createTemplate)
		lifecycle.GET("/templates", h.listTemplates)

		// 批量操作
		lifecycle.POST("/batch-apply", h.batchApplyPolicy)

		// 访问分析
		lifecycle.GET("/access-report", h.getAccessReport)

		// 自动化控制
		lifecycle.POST("/auto-migrate/run", h.runAutoMigrate)
		lifecycle.POST("/auto-cleanup/run", h.runAutoCleanup)
		lifecycle.PUT("/auto-migrate/toggle", h.toggleAutoMigrate)
		lifecycle.PUT("/auto-cleanup/toggle", h.toggleAutoCleanup)

		// 审计日志
		lifecycle.GET("/audit-log", h.getAuditLog)

		// 状态
		lifecycle.GET("/status", h.getStatus)
	}
}

// ========== 策略管理处理器 ==========

func (h *Handlers) createPolicy(c *gin.Context) {
	var req LifecyclePolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy, err := h.manager.CreatePolicy(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, policy)
}

func (h *Handlers) listPolicies(c *gin.Context) {
	enabledStr := c.Query("enabled")
	var enabled *bool
	if enabledStr != "" {
		e := enabledStr == "true"
		enabled = &e
	}

	policies := h.manager.ListPolicies(enabled)
	c.JSON(http.StatusOK, gin.H{
		"policies": policies,
		"total":    len(policies),
	})
}

func (h *Handlers) getPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policy)
}

func (h *Handlers) updatePolicy(c *gin.Context) {
	id := c.Param("id")
	var req LifecyclePolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy, err := h.manager.UpdatePolicy(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, policy)
}

func (h *Handlers) deletePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePolicy(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "策略已删除"})
}

// ========== 数据记录处理器 ==========

func (h *Handlers) createRecord(c *gin.Context) {
	var req DataRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	record, err := h.manager.CreateRecord(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, record)
}

func (h *Handlers) listRecords(c *gin.Context) {
	phase := LifecyclePhase(c.Query("phase"))
	tier := StorageTier(c.Query("tier"))

	records := h.manager.ListRecords(phase, tier)
	c.JSON(http.StatusOK, gin.H{
		"records": records,
		"total":   len(records),
	})
}

func (h *Handlers) getRecord(c *gin.Context) {
	id := c.Param("id")
	record, err := h.manager.GetRecord(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, record)
}

func (h *Handlers) transitionPhase(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Phase  LifecyclePhase `json:"phase" binding:"required"`
		Reason string         `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.TransitionPhase(id, req.Phase, req.Reason); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "阶段转换成功"})
}

// ========== 合规保留处理器 ==========

func (h *Handlers) createHold(c *gin.Context) {
	var req ComplianceHold
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hold, err := h.manager.CreateHold(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, hold)
}

func (h *Handlers) listHolds(c *gin.Context) {
	activeStr := c.Query("active")
	var active *bool
	if activeStr != "" {
		a := activeStr == "true"
		active = &a
	}

	holds := h.manager.ListHolds(active)
	c.JSON(http.StatusOK, gin.H{
		"holds": holds,
		"total": len(holds),
	})
}

func (h *Handlers) releaseHold(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		ReleasedBy string `json:"releasedBy" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.ReleaseHold(id, req.ReleasedBy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "合规保留已释放"})
}

// ========== 数据迁移处理器 ==========

func (h *Handlers) createMigration(c *gin.Context) {
	var req DataMigration
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	migration, err := h.manager.CreateMigration(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, migration)
}

func (h *Handlers) listMigrations(c *gin.Context) {
	status := MigrationStatus(c.Query("status"))
	migrations := h.manager.ListMigrations(status)
	c.JSON(http.StatusOK, gin.H{
		"migrations": migrations,
		"total":      len(migrations),
	})
}

func (h *Handlers) getMigration(c *gin.Context) {
	id := c.Param("id")
	migration, err := h.manager.GetMigration(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, migration)
}

func (h *Handlers) startMigration(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StartMigration(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "迁移已启动"})
}

// ========== 数据销毁处理器 ==========

func (h *Handlers) createDestruction(c *gin.Context) {
	var req DestructionRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	destruction, err := h.manager.CreateDestruction(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, destruction)
}

func (h *Handlers) getDestruction(c *gin.Context) {
	id := c.Param("id")
	destruction, err := h.manager.GetDestruction(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, destruction)
}

func (h *Handlers) approveDestruction(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		ApprovedBy string `json:"approvedBy" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.ApproveDestruction(id, req.ApprovedBy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "销毁已批准"})
}

func (h *Handlers) executeDestruction(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.ExecuteDestruction(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "销毁已执行"})
}

// ========== 策略模板处理器 ==========

func (h *Handlers) createTemplate(c *gin.Context) {
	var req PolicyTemplate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	template, err := h.manager.CreateTemplate(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, template)
}

func (h *Handlers) listTemplates(c *gin.Context) {
	templates := h.manager.ListTemplates()
	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"total":     len(templates),
	})
}

// ========== 批量操作处理器 ==========

func (h *Handlers) batchApplyPolicy(c *gin.Context) {
	var req BatchApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.manager.BatchApplyPolicy(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ========== 访问分析处理器 ==========

func (h *Handlers) getAccessReport(c *gin.Context) {
	report := h.manager.GenerateAccessReport()
	c.JSON(http.StatusOK, report)
}

// ========== 审计日志处理器 ==========

func (h *Handlers) getAuditLog(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 100
	}

	logs := h.manager.GetAuditLog(limit)
	c.JSON(http.StatusOK, gin.H{
		"logs":  logs,
		"total": len(logs),
	})
}

// ========== 状态处理器 ==========

func (h *Handlers) getStatus(c *gin.Context) {
	status := h.manager.GetStatus()
	c.JSON(http.StatusOK, status)
}

// ========== 自动化控制处理器 ==========

func (h *Handlers) runAutoMigrate(c *gin.Context) {
	h.manager.RunAutoMigrateNow()
	c.JSON(http.StatusOK, gin.H{"message": "自动迁移已触发"})
}

func (h *Handlers) runAutoCleanup(c *gin.Context) {
	h.manager.RunAutoCleanupNow()
	c.JSON(http.StatusOK, gin.H{"message": "自动清理已触发"})
}

func (h *Handlers) toggleAutoMigrate(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.manager.SetAutoMigrateEnabled(req.Enabled)
	c.JSON(http.StatusOK, gin.H{
		"message": "自动迁移状态已更新",
		"enabled": req.Enabled,
	})
}

func (h *Handlers) toggleAutoCleanup(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.manager.SetAutoCleanupEnabled(req.Enabled)
	c.JSON(http.StatusOK, gin.H{
		"message": "自动清理状态已更新",
		"enabled": req.Enabled,
	})
}
