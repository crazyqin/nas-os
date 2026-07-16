// Package filelifecycle - 文件生命周期 HTTP API 处理器
package filelifecycle

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 文件生命周期 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/v1/file-lifecycle 路由组.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	fl := r.Group("/file-lifecycle")
	{
		// 配置
		fl.GET("/config", h.getConfig)
		fl.PUT("/config", h.updateConfig)

		// 分层策略
		fl.POST("/tiering-policies", h.createTieringPolicy)
		fl.GET("/tiering-policies", h.listTieringPolicies)
		fl.GET("/tiering-policies/:id", h.getTieringPolicy)
		fl.PUT("/tiering-policies/:id", h.updateTieringPolicy)
		fl.DELETE("/tiering-policies/:id", h.deleteTieringPolicy)

		// 保留策略
		fl.POST("/retention-policies", h.createRetentionPolicy)
		fl.GET("/retention-policies", h.listRetentionPolicies)
		fl.GET("/retention-policies/:id", h.getRetentionPolicy)
		fl.PUT("/retention-policies/:id", h.updateRetentionPolicy)
		fl.DELETE("/retention-policies/:id", h.deleteRetentionPolicy)

		// 文件记录
		fl.POST("/files", h.createFileRecord)
		fl.GET("/files", h.listFileRecords)
		fl.GET("/files/:id", h.getFileRecord)
		fl.POST("/files/:id/tier", h.changeFileTier)
		fl.POST("/files/:id/stage", h.changeFileStage)

		// 合规保留
		fl.POST("/holds", h.createHold)
		fl.GET("/holds", h.listHolds)
		fl.POST("/holds/:id/release", h.releaseHold)

		// 迁移任务
		fl.POST("/migrations", h.createMigration)
		fl.GET("/migrations", h.listMigrations)
		fl.GET("/migrations/:id", h.getMigration)
		fl.POST("/migrations/:id/start", h.startMigration)
		fl.POST("/migrations/:id/cancel", h.cancelMigration)

		// 批量迁移
		fl.POST("/batch-migrate", h.batchMigrate)

		// 销毁
		fl.POST("/destructions", h.createDestruction)
		fl.GET("/destructions/:id", h.getDestruction)
		fl.POST("/destructions/:id/approve", h.approveDestruction)
		fl.POST("/destructions/:id/execute", h.executeDestruction)

		// 自动化
		fl.POST("/auto-migrate/run", h.runAutoMigrate)
		fl.POST("/auto-cleanup/run", h.runAutoCleanup)
		fl.PUT("/auto-migrate/toggle", h.toggleAutoMigrate)
		fl.PUT("/auto-cleanup/toggle", h.toggleAutoCleanup)

		// 分析报告
		fl.GET("/report", h.getReport)

		// 审计日志
		fl.GET("/audit-log", h.getAuditLog)

		// 状态
		fl.GET("/status", h.getStatus)
	}
}

// ==================== 配置处理器 ====================

func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	c.JSON(http.StatusOK, cfg)
}

func (h *Handlers) updateConfig(c *gin.Context) {
	var req FileLifecycleConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.manager.UpdateConfig(req)
	c.JSON(http.StatusOK, gin.H{"message": "配置已更新"})
}

// ==================== 分层策略处理器 ====================

func (h *Handlers) createTieringPolicy(c *gin.Context) {
	var req TieringPolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy, err := h.manager.CreateTieringPolicy(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, policy)
}

func (h *Handlers) listTieringPolicies(c *gin.Context) {
	enabledStr := c.Query("enabled")
	var enabled *bool
	if enabledStr != "" {
		e := enabledStr == "true"
		enabled = &e
	}

	policies := h.manager.ListTieringPolicies(enabled)
	c.JSON(http.StatusOK, gin.H{
		"policies": policies,
		"total":    len(policies),
	})
}

func (h *Handlers) getTieringPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.manager.GetTieringPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policy)
}

func (h *Handlers) updateTieringPolicy(c *gin.Context) {
	id := c.Param("id")
	var req TieringPolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy, err := h.manager.UpdateTieringPolicy(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, policy)
}

func (h *Handlers) deleteTieringPolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteTieringPolicy(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "分层策略已删除"})
}

// ==================== 保留策略处理器 ====================

func (h *Handlers) createRetentionPolicy(c *gin.Context) {
	var req RetentionPolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy, err := h.manager.CreateRetentionPolicy(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, policy)
}

func (h *Handlers) listRetentionPolicies(c *gin.Context) {
	enabledStr := c.Query("enabled")
	var enabled *bool
	if enabledStr != "" {
		e := enabledStr == "true"
		enabled = &e
	}

	policies := h.manager.ListRetentionPolicies(enabled)
	c.JSON(http.StatusOK, gin.H{
		"policies": policies,
		"total":    len(policies),
	})
}

func (h *Handlers) getRetentionPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.manager.GetRetentionPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policy)
}

func (h *Handlers) updateRetentionPolicy(c *gin.Context) {
	id := c.Param("id")
	var req RetentionPolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy, err := h.manager.UpdateRetentionPolicy(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, policy)
}

func (h *Handlers) deleteRetentionPolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteRetentionPolicy(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "保留策略已删除"})
}

// ==================== 文件记录处理器 ====================

func (h *Handlers) createFileRecord(c *gin.Context) {
	var req FileRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	record, err := h.manager.CreateFileRecord(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, record)
}

func (h *Handlers) listFileRecords(c *gin.Context) {
	tier := FileTier(c.Query("tier"))
	stage := LifecycleStage(c.Query("stage"))
	category := FileCategory(c.Query("category"))

	records := h.manager.ListFileRecords(tier, stage, category)
	c.JSON(http.StatusOK, gin.H{
		"records": records,
		"total":   len(records),
	})
}

func (h *Handlers) getFileRecord(c *gin.Context) {
	id := c.Param("id")
	record, err := h.manager.GetFileRecord(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, record)
}

func (h *Handlers) changeFileTier(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Tier   FileTier `json:"tier" binding:"required"`
		Reason string   `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.ChangeFileTier(id, req.Tier, req.Reason); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "存储层级已变更"})
}

func (h *Handlers) changeFileStage(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Stage  LifecycleStage `json:"stage" binding:"required"`
		Reason string         `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.ChangeFileStage(id, req.Stage, req.Reason); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "生命周期阶段已变更"})
}

// ==================== 合规保留处理器 ====================

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

// ==================== 迁移任务处理器 ====================

func (h *Handlers) createMigration(c *gin.Context) {
	var req FileMigration
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
	state := MigrationState(c.Query("state"))
	migrations := h.manager.ListMigrations(state)
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

func (h *Handlers) cancelMigration(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.CancelMigration(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "迁移已取消"})
}

// ==================== 批量迁移处理器 ====================

func (h *Handlers) batchMigrate(c *gin.Context) {
	var req BatchMigrateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.manager.BatchMigrate(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ==================== 销毁处理器 ====================

func (h *Handlers) createDestruction(c *gin.Context) {
	var req DestructionRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	record, err := h.manager.CreateDestruction(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, record)
}

func (h *Handlers) getDestruction(c *gin.Context) {
	id := c.Param("id")
	record, err := h.manager.GetDestruction(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, record)
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

// ==================== 自动化处理器 ====================

func (h *Handlers) runAutoMigrate(c *gin.Context) {
	count := h.manager.RunAutoMigrateNow()
	c.JSON(http.StatusOK, gin.H{
		"message":       "自动迁移已触发",
		"migratedFiles": count,
	})
}

func (h *Handlers) runAutoCleanup(c *gin.Context) {
	count := h.manager.RunAutoCleanupNow()
	c.JSON(http.StatusOK, gin.H{
		"message":      "自动清理已触发",
		"cleanedFiles": count,
	})
}

func (h *Handlers) toggleAutoMigrate(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.manager.SetAutoMigrate(req.Enabled)
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
	h.manager.SetAutoCleanup(req.Enabled)
	c.JSON(http.StatusOK, gin.H{
		"message": "自动清理状态已更新",
		"enabled": req.Enabled,
	})
}

// ==================== 分析报告处理器 ====================

func (h *Handlers) getReport(c *gin.Context) {
	report := h.manager.GenerateReport()
	c.JSON(http.StatusOK, report)
}

// ==================== 审计日志处理器 ====================

func (h *Handlers) getAuditLog(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	action := c.Query("action")
	logs := h.manager.GetAuditLog(limit, action)
	c.JSON(http.StatusOK, gin.H{
		"logs":  logs,
		"total": len(logs),
	})
}

// ==================== 状态处理器 ====================

func (h *Handlers) getStatus(c *gin.Context) {
	status := h.manager.GetStatus()
	c.JSON(http.StatusOK, status)
}

// ==================== 辅助函数 ====================

// nowNow 返回当前时间，便于测试替身.
func nowNow() time.Time {
	return time.Now()
}
