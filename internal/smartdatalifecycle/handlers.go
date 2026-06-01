package smartdatalifecycle

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler HTTP API 处理器
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建 HTTP 处理器
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes 注册 API 路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	lifecycle := rg.Group("/lifecycle")
	{
		// 状态
		lifecycle.GET("/status", h.GetStatus)
		lifecycle.GET("/config", h.GetConfig)
		lifecycle.PUT("/config", h.UpdateConfig)
		lifecycle.GET("/stats", h.GetStats)

		// 数据项
		lifecycle.GET("/items", h.ListItems)
		lifecycle.GET("/items/:id", h.GetItem)
		lifecycle.POST("/items", h.RegisterItem)
		lifecycle.PUT("/items/:id/stage", h.UpdateItemStage)
		lifecycle.POST("/items/:id/access", h.RecordAccess)

		// 保留策略
		lifecycle.GET("/policies/retention", h.ListRetentionPolicies)
		lifecycle.POST("/policies/retention", h.CreateRetentionPolicy)
		lifecycle.GET("/policies/retention/:id", h.GetRetentionPolicy)
		lifecycle.PUT("/policies/retention/:id", h.UpdateRetentionPolicy)
		lifecycle.DELETE("/policies/retention/:id", h.DeleteRetentionPolicy)
		lifecycle.POST("/items/:id/retention", h.SetItemRetentionPolicy)
		lifecycle.POST("/items/:id/legal-hold", h.SetLegalHold)

		// 归档策略
		lifecycle.GET("/policies/archive", h.ListArchivePolicies)
		lifecycle.POST("/policies/archive", h.CreateArchivePolicy)
		lifecycle.GET("/policies/archive/:id", h.GetArchivePolicy)
		lifecycle.DELETE("/policies/archive/:id", h.DeleteArchivePolicy)
		lifecycle.GET("/archive/candidates", h.GetArchiveCandidates)
		lifecycle.POST("/archive/execute", h.ExecuteArchive)

		// 清理规则
		lifecycle.GET("/rules/cleanup", h.ListCleanupRules)
		lifecycle.POST("/rules/cleanup", h.CreateCleanupRule)
		lifecycle.GET("/rules/cleanup/:id", h.GetCleanupRule)
		lifecycle.DELETE("/rules/cleanup/:id", h.DeleteCleanupRule)
		lifecycle.POST("/cleanup/execute/:id", h.ExecuteCleanup)
		lifecycle.GET("/cleanup/preview/:id", h.GetCleanupPreview)

		// 迁移
		lifecycle.GET("/migrations", h.ListMigrationTasks)
		lifecycle.GET("/migrations/:id", h.GetMigrationTask)
		lifecycle.POST("/migrations/force", h.ForceMigration)
		lifecycle.GET("/migrations/stats", h.GetMigrationStats)
		lifecycle.GET("/migrations/running", h.GetRunningMigrations)

		// 重复数据
		lifecycle.GET("/dedup/groups", h.GetDuplicateGroups)
		lifecycle.POST("/dedup/scan", h.ScanDuplicates)
		lifecycle.POST("/dedup/cleanup/:groupId", h.CleanupDuplicates)
		lifecycle.GET("/dedup/stats", h.GetDedupStats)

		// 事件
		lifecycle.GET("/events", h.GetEvents)

		// 过期检查
		lifecycle.GET("/expiring", h.GetExpiringItems)
		lifecycle.GET("/retention/stats", h.GetRetentionStats)
	}
}

// ============================================================
// 状态与配置
// ============================================================

// GetStatus 获取系统状态
func (h *Handler) GetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"running": h.manager.IsRunning(),
		"config":  h.manager.GetConfig(),
	})
}

// GetConfig 获取配置
func (h *Handler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, h.manager.GetConfig())
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	Archive   *ArchiveConfig   `json:"archive,omitempty"`
	Cleanup   *CleanupConfig   `json:"cleanup,omitempty"`
	Migration *MigrationConfig `json:"migration,omitempty"`
	Retention *RetentionConfig `json:"retention,omitempty"`
	Dedup     *DedupConfig     `json:"dedup,omitempty"`
}

// UpdateConfig 更新配置
func (h *Handler) UpdateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := h.manager.GetConfig()

	if req.Archive != nil {
		config.Archive = *req.Archive
	}
	if req.Cleanup != nil {
		config.Cleanup = *req.Cleanup
	}
	if req.Migration != nil {
		config.Migration = *req.Migration
	}
	if req.Retention != nil {
		config.Retention = *req.Retention
	}
	if req.Dedup != nil {
		config.Dedup = *req.Dedup
	}

	h.manager.UpdateConfig(config)
	c.JSON(http.StatusOK, gin.H{"message": "config updated"})
}

// GetStats 获取统计信息
func (h *Handler) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.manager.GetStats())
}

// ============================================================
// 数据项管理
// ============================================================

// ListItems 列出数据项
func (h *Handler) ListItems(c *gin.Context) {
	stage := LifecycleStage(c.Query("stage"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	items := h.manager.ListItems(stage, limit, offset)
	c.JSON(http.StatusOK, gin.H{
		"items":  items,
		"total":  len(items),
		"limit":  limit,
		"offset": offset,
	})
}

// GetItem 获取数据项
func (h *Handler) GetItem(c *gin.Context) {
	id := c.Param("id")
	item, ok := h.manager.GetItem(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// RegisterItemRequest 注册数据项请求
type RegisterItemRequest struct {
	Path           string             `json:"path" binding:"required"`
	Name           string             `json:"name"`
	Size           int64              `json:"size"`
	ContentType    string             `json:"content_type"`
	Classification DataClassification `json:"classification"`
	Tags           []string           `json:"tags"`
	Metadata       map[string]string  `json:"metadata"`
}

// RegisterItem 注册数据项
func (h *Handler) RegisterItem(c *gin.Context) {
	var req RegisterItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	item := &DataItem{
		ID:             "item-" + strconv.FormatInt(time.Now().UnixNano(), 36),
		Path:           req.Path,
		Name:           req.Name,
		Size:           req.Size,
		ContentType:    req.ContentType,
		Classification: req.Classification,
		Tags:           req.Tags,
		Metadata:       req.Metadata,
	}

	if err := h.manager.RegisterItem(item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, item)
}

// UpdateItemStageRequest 更新数据项阶段请求
type UpdateItemStageRequest struct {
	Stage LifecycleStage `json:"stage" binding:"required"`
}

// UpdateItemStage 更新数据项阶段
func (h *Handler) UpdateItemStage(c *gin.Context) {
	id := c.Param("id")
	var req UpdateItemStageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.UpdateItemStage(id, req.Stage, "manual"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "stage updated"})
}

// RecordAccess 记录访问
func (h *Handler) RecordAccess(c *gin.Context) {
	id := c.Param("id")
	opType := c.DefaultQuery("type", "read")

	if err := h.manager.RecordAccess(id, opType); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "access recorded"})
}

// ============================================================
// 保留策略
// ============================================================

// ListRetentionPolicies 列出保留策略
func (h *Handler) ListRetentionPolicies(c *gin.Context) {
	policies := h.manager.ListRetentionPolicies()
	c.JSON(http.StatusOK, gin.H{"policies": policies})
}

// CreateRetentionPolicy 创建保留策略
func (h *Handler) CreateRetentionPolicy(c *gin.Context) {
	var req CreatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy := &RetentionPolicy{
		ID:               "rp-" + strconv.FormatInt(time.Now().UnixNano(), 36),
		Name:             req.Name,
		Description:      req.Description,
		Classification:   req.Classification,
		RetentionDays:    req.RetentionDays,
		ExpirationAction: req.ExpirationAction,
		CompliancePolicy: req.CompliancePolicy,
		FilePatterns:     req.FilePatterns,
		PathPrefixes:     req.PathPrefixes,
	}

	h.manager.AddRetentionPolicy(policy)
	c.JSON(http.StatusCreated, policy)
}

// GetRetentionPolicy 获取保留策略
func (h *Handler) GetRetentionPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, ok := h.manager.GetRetentionPolicy(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "policy not found"})
		return
	}
	c.JSON(http.StatusOK, policy)
}

// UpdateRetentionPolicy 更新保留策略
func (h *Handler) UpdateRetentionPolicy(c *gin.Context) {
	id := c.Param("id")
	var req RetentionPolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.ID = id

	if err := h.manager.UpdateRetentionPolicy(&req); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "policy updated"})
}

// DeleteRetentionPolicy 删除保留策略
func (h *Handler) DeleteRetentionPolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteRetentionPolicy(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "policy deleted"})
}

// SetItemRetentionPolicyRequest 设置数据项保留策略请求
type SetItemRetentionPolicyRequest struct {
	PolicyID string `json:"policy_id" binding:"required"`
}

// SetItemRetentionPolicy 设置数据项保留策略
func (h *Handler) SetItemRetentionPolicy(c *gin.Context) {
	itemID := c.Param("id")
	var req SetItemRetentionPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.retentionMgr.SetRetentionPolicy(itemID, req.PolicyID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "retention policy set"})
}

// SetLegalHoldRequest 设置法律冻结请求
type SetLegalHoldRequest struct {
	Hold bool `json:"hold"`
}

// SetLegalHold 设置法律冻结
func (h *Handler) SetLegalHold(c *gin.Context) {
	itemID := c.Param("id")
	var req SetLegalHoldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.retentionMgr.SetLegalHold(itemID, req.Hold); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "legal hold updated"})
}

// ============================================================
// 归档策略
// ============================================================

// ListArchivePolicies 列出归档策略
func (h *Handler) ListArchivePolicies(c *gin.Context) {
	policies := h.manager.ListArchivePolicies()
	c.JSON(http.StatusOK, gin.H{"policies": policies})
}

// CreateArchivePolicy 创建归档策略
func (h *Handler) CreateArchivePolicy(c *gin.Context) {
	var req CreateArchivePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy := &ArchivePolicy{
		ID:              "ap-" + strconv.FormatInt(time.Now().UnixNano(), 36),
		Name:            req.Name,
		Description:     req.Description,
		Enabled:         req.Enabled,
		Trigger:         req.Trigger,
		MaxAccessCount:  req.MaxAccessCount,
		DaysSinceAccess: req.DaysSinceAccess,
		FileAgeDays:     req.FileAgeDays,
		TargetStage:     req.TargetStage,
		Schedule:        req.Schedule,
		FilePatterns:    req.FilePatterns,
		PathPrefixes:    req.PathPrefixes,
		ExcludePatterns: req.ExcludePatterns,
	}

	h.manager.AddArchivePolicy(policy)
	c.JSON(http.StatusCreated, policy)
}

// GetArchivePolicy 获取归档策略
func (h *Handler) GetArchivePolicy(c *gin.Context) {
	id := c.Param("id")
	policy, ok := h.manager.GetArchivePolicy(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "policy not found"})
		return
	}
	c.JSON(http.StatusOK, policy)
}

// DeleteArchivePolicy 删除归档策略
func (h *Handler) DeleteArchivePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteArchivePolicy(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "policy deleted"})
}

// GetArchiveCandidates 获取归档候选
func (h *Handler) GetArchiveCandidates(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	candidates := h.manager.archiver.GetArchiveCandidates(limit)
	c.JSON(http.StatusOK, gin.H{
		"candidates": candidates,
		"total":      len(candidates),
	})
}

// ExecuteArchive 执行归档
func (h *Handler) ExecuteArchive(c *gin.Context) {
	if err := h.manager.archiver.Run(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "archive executed"})
}

// ============================================================
// 清理规则
// ============================================================

// ListCleanupRules 列出清理规则
func (h *Handler) ListCleanupRules(c *gin.Context) {
	rules := h.manager.ListCleanupRules()
	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

// CreateCleanupRule 创建清理规则
func (h *Handler) CreateCleanupRule(c *gin.Context) {
	var req CreateCleanupRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule := &CleanupRule{
		ID:               "cr-" + strconv.FormatInt(time.Now().UnixNano(), 36),
		Name:             req.Name,
		Description:      req.Description,
		Enabled:          req.Enabled,
		RuleType:         req.RuleType,
		ExpireDays:       req.ExpireDays,
		TempFilePatterns: req.TempFilePatterns,
		RemoveEmptyDirs:  req.RemoveEmptyDirs,
		LogRetentionDays: req.LogRetentionDays,
		Schedule:         req.Schedule,
	}

	h.manager.AddCleanupRule(rule)
	c.JSON(http.StatusCreated, rule)
}

// GetCleanupRule 获取清理规则
func (h *Handler) GetCleanupRule(c *gin.Context) {
	id := c.Param("id")
	rule, ok := h.manager.GetCleanupRule(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}
	c.JSON(http.StatusOK, rule)
}

// DeleteCleanupRule 删除清理规则
func (h *Handler) DeleteCleanupRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteCleanupRule(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "rule deleted"})
}

// ExecuteCleanup 执行清理
func (h *Handler) ExecuteCleanup(c *gin.Context) {
	ruleID := c.Param("id")
	result, err := h.manager.cleaner.ExecuteCleanup(c.Request.Context(), ruleID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetCleanupPreview 获取清理预览
func (h *Handler) GetCleanupPreview(c *gin.Context) {
	ruleID := c.Param("id")
	result, err := h.manager.cleaner.GetCleanupPreview(c.Request.Context(), ruleID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// ============================================================
// 迁移
// ============================================================

// ListMigrationTasks 列出迁移任务
func (h *Handler) ListMigrationTasks(c *gin.Context) {
	status := MigrationStatus(c.Query("status"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	tasks := h.manager.ListMigrationTasks(status, limit, offset)
	c.JSON(http.StatusOK, gin.H{
		"tasks":  tasks,
		"total":  len(tasks),
		"limit":  limit,
		"offset": offset,
	})
}

// GetMigrationTask 获取迁移任务
func (h *Handler) GetMigrationTask(c *gin.Context) {
	id := c.Param("id")
	task, ok := h.manager.GetMigrationTask(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	c.JSON(http.StatusOK, task)
}

// ForceMigrationRequest 强制迁移请求
type ForceMigrationRequest struct {
	ItemID      string          `json:"item_id" binding:"required"`
	TargetStage LifecycleStage  `json:"target_stage" binding:"required"`
}

// ForceMigration 强制迁移
func (h *Handler) ForceMigration(c *gin.Context) {
	var req ForceMigrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, err := h.manager.migrator.ForceMigration(c.Request.Context(), req.ItemID, req.TargetStage)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, task)
}

// GetMigrationStats 获取迁移统计
func (h *Handler) GetMigrationStats(c *gin.Context) {
	stats := h.manager.migrator.GetMigrationStats()
	c.JSON(http.StatusOK, stats)
}

// GetRunningMigrations 获取运行中的迁移
func (h *Handler) GetRunningMigrations(c *gin.Context) {
	migrations := h.manager.migrator.GetRunningMigrations()
	c.JSON(http.StatusOK, gin.H{"migrations": migrations})
}

// ============================================================
// 重复数据
// ============================================================

// GetDuplicateGroups 获取重复数据组
func (h *Handler) GetDuplicateGroups(c *gin.Context) {
	groups := h.manager.deduplicator.GetDuplicateGroups()
	c.JSON(http.StatusOK, gin.H{
		"groups": groups,
		"total":  len(groups),
	})
}

// ScanDuplicates 扫描重复数据
func (h *Handler) ScanDuplicates(c *gin.Context) {
	result, err := h.manager.deduplicator.Scan(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// CleanupDuplicates 清理重复数据
func (h *Handler) CleanupDuplicates(c *gin.Context) {
	groupID := c.Param("groupId")
	reclaimed, err := h.manager.deduplicator.CleanupDuplicates(c.Request.Context(), groupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"reclaimed": reclaimed,
		"message":   "duplicates cleaned up",
	})
}

// GetDedupStats 获取去重统计
func (h *Handler) GetDedupStats(c *gin.Context) {
	stats := h.manager.deduplicator.GetDedupStats()
	c.JSON(http.StatusOK, stats)
}

// ============================================================
// 事件查询
// ============================================================

// GetEvents 获取事件列表
func (h *Handler) GetEvents(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	events := h.manager.GetEvents(limit, offset)
	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"total":  len(events),
		"limit":  limit,
		"offset": offset,
	})
}

// ============================================================
// 过期检查
// ============================================================

// GetExpiringItems 获取即将过期的数据项
func (h *Handler) GetExpiringItems(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	items := h.manager.retentionMgr.GetExpiringItems(days)
	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": len(items),
		"days":  days,
	})
}

// GetRetentionStats 获取保留策略统计
func (h *Handler) GetRetentionStats(c *gin.Context) {
	stats := h.manager.retentionMgr.GetRetentionStats()
	c.JSON(http.StatusOK, stats)
}
