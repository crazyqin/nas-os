package smarttiering

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler provides HTTP handlers for smart tiering management.
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler creates a new smart tiering HTTP handler.
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{manager: manager, logger: logger}
}

// RegisterRoutes registers smart tiering API routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	tiering := rg.Group("/tiering")
	{
		// 状态
		tiering.GET("/status", h.GetStatus)
		tiering.GET("/config", h.GetConfig)
		tiering.PUT("/config", h.UpdateConfig)

		// 文件热度
		tiering.GET("/files/heat", h.GetFileHeat)
		tiering.GET("/files/tier/:tier", h.GetFilesByTier)
		tiering.POST("/files/register", h.RegisterFile)
		tiering.POST("/files/access", h.RecordAccess)

		// 迁移
		tiering.GET("/migrations", h.GetMigrationEvents)
		tiering.POST("/migrations/force", h.ForceMigrate)

		// 成本
		tiering.GET("/cost/report", h.GetCostReport)
		tiering.GET("/cost/budget", h.GetBudgetStatus)

		// 监控
		tiering.GET("/metrics", h.GetMetrics)
		tiering.GET("/metrics/history", h.GetMetricsHistory)
		tiering.GET("/summary", h.GetTierSummary)
	}

	// 任务要求的API端点
	smarttiering := rg.Group("/smarttiering")
	{
		smarttiering.GET("/stats", h.GetTieringStats)
		smarttiering.POST("/migrate", h.TriggerMigration)
		smarttiering.GET("/recommendations", h.GetMigrationRecommendations)
	}

	// 任务要求的标准路由
	tier := rg.Group("/tier")
	{
		tier.GET("/policies", h.ListPolicies)
		tier.POST("/policies", h.CreatePolicy)
		tier.DELETE("/policies/:id", h.DeletePolicyTier)
		tier.GET("/placement", h.GetPlacement)
		tier.GET("/placement/all", h.GetAllPlacements)
		tier.POST("/migrate", h.TriggerMigrationTier)
		tier.GET("/stats", h.GetTierStats)
		tier.GET("/access-pattern", h.GetAccessPattern)
	}
}

// GetStatus handles GET /api/v1/tiering/status.
func (h *Handler) GetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"running": h.manager.IsRunning(),
		"config":  h.manager.GetConfig(),
	})
}

// GetConfig handles GET /api/v1/tiering/config.
func (h *Handler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, h.manager.GetConfig())
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	Predictor     *PredictorConfig     `json:"predictor,omitempty"`
	Migrator      *MigratorConfig      `json:"migrator,omitempty"`
	CostOptimizer *CostOptimizerConfig `json:"cost_optimizer,omitempty"`
	Monitor       *MonitorConfig       `json:"monitor,omitempty"`
}

// UpdateConfig handles PUT /api/v1/tiering/config.
func (h *Handler) UpdateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := h.manager.GetConfig()
	if req.Predictor != nil {
		config.Predictor = *req.Predictor
	}
	if req.Migrator != nil {
		config.Migrator = *req.Migrator
	}
	if req.CostOptimizer != nil {
		config.CostOptimizer = *req.CostOptimizer
	}
	if req.Monitor != nil {
		config.Monitor = *req.Monitor
	}

	h.manager.UpdateConfig(config)
	c.JSON(http.StatusOK, gin.H{"message": "config updated", "config": config})
}

// GetFileHeat handles GET /api/v1/tiering/files/heat?path=xxx.
func (h *Handler) GetFileHeat(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}

	heat, exists := h.manager.GetFileHeat(path)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"path": path,
		"heat": heat,
	})
}

// GetFilesByTier handles GET /api/v1/tiering/files/tier/:tier.
func (h *Handler) GetFilesByTier(c *gin.Context) {
	tierStr := c.Param("tier")
	tier := ParseTier(tierStr)

	files := h.manager.predictor.GetFilesByTier(tier)
	c.JSON(http.StatusOK, gin.H{
		"tier":  tierStr,
		"files": files,
		"count": len(files),
	})
}

// RegisterFileRequest 注册文件请求
type RegisterFileRequest struct {
	Path        string   `json:"path" binding:"required"`
	Size        int64    `json:"size" binding:"required"`
	Tier        string   `json:"tier"`
	ContentType string   `json:"content_type"`
	Tags        []string `json:"tags"`
}

// RegisterFile handles POST /api/v1/tiering/files/register.
func (h *Handler) RegisterFile(c *gin.Context) {
	var req RegisterFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	meta := FileMetadata{
		Path:        req.Path,
		Size:        req.Size,
		CurrentTier: ParseTier(req.Tier),
		ContentType: req.ContentType,
		Tags:        req.Tags,
		CreatedAt:   time.Now(),
		ModifiedAt:  time.Now(),
		AccessedAt:  time.Now(),
	}

	h.manager.RegisterFile(meta)
	c.JSON(http.StatusCreated, gin.H{"message": "file registered", "path": req.Path})
}

// RecordAccessRequest 记录访问请求
type RecordAccessRequest struct {
	Path   string `json:"path" binding:"required"`
	OpType string `json:"op_type" binding:"required"` // read, write, delete
	Size   int64  `json:"size"`
}

// RecordAccess handles POST /api/v1/tiering/files/access.
func (h *Handler) RecordAccess(c *gin.Context) {
	var req RecordAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	record := AccessRecord{
		Path:      req.Path,
		OpType:    req.OpType,
		Size:      req.Size,
		Timestamp: time.Now(),
	}

	h.manager.RecordAccess(record)
	c.JSON(http.StatusOK, gin.H{"message": "access recorded"})
}

// GetMigrationEvents handles GET /api/v1/tiering/migrations.
func (h *Handler) GetMigrationEvents(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 50
	}

	events := h.manager.GetMigrationEvents(limit)
	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"count":  len(events),
	})
}

// ForceMigrateRequest 强制迁移请求
type ForceMigrateRequest struct {
	Path     string `json:"path" binding:"required"`
	FromTier string `json:"from_tier" binding:"required"`
	ToTier   string `json:"to_tier" binding:"required"`
	FileSize int64  `json:"file_size"`
}

// ForceMigrate handles POST /api/v1/tiering/migrations/force.
func (h *Handler) ForceMigrate(c *gin.Context) {
	var req ForceMigrateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.manager.ForceMigrate(
		c.Request.Context(),
		req.Path,
		ParseTier(req.FromTier),
		ParseTier(req.ToTier),
		req.FileSize,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "migration queued"})
}

// GetCostReport handles GET /api/v1/tiering/cost/report.
func (h *Handler) GetCostReport(c *gin.Context) {
	report, err := h.manager.GetCostReport(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, report)
}

// GetBudgetStatus handles GET /api/v1/tiering/cost/budget.
func (h *Handler) GetBudgetStatus(c *gin.Context) {
	used, budget, remaining, err := h.manager.costOptimizer.GetBudgetStatus(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"used":      used,
		"budget":    budget,
		"remaining": remaining,
	})
}

// GetMetrics handles GET /api/v1/tiering/metrics.
func (h *Handler) GetMetrics(c *gin.Context) {
	metrics := h.manager.GetMetrics()
	if metrics == nil {
		c.JSON(http.StatusOK, gin.H{"message": "no metrics available"})
		return
	}
	c.JSON(http.StatusOK, metrics)
}

// GetMetricsHistory handles GET /api/v1/tiering/metrics/history.
func (h *Handler) GetMetricsHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 100
	}

	history := h.manager.GetMetricsHistory(limit)
	c.JSON(http.StatusOK, gin.H{
		"metrics": history,
		"count":   len(history),
	})
}

// GetTierSummary handles GET /api/v1/tiering/summary.
func (h *Handler) GetTierSummary(c *gin.Context) {
	summary := h.manager.GetTierSummary()
	c.JSON(http.StatusOK, summary)
}

// GetTieringStats handles GET /api/smarttiering/stats.
// 返回分层统计信息
func (h *Handler) GetTieringStats(c *gin.Context) {
	metrics := h.manager.GetMetrics()
	if metrics == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "no_data",
			"message": "no metrics available yet",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total_files":       metrics.TotalFiles,
		"total_size_gb":     metrics.TotalSizeGB,
		"tier_distribution": metrics.TierDistribution,
		"tier_sizes_gb":     metrics.TierSizesGB,
		"avg_heat_scores":   metrics.AvgHeatScores,
		"hit_rates":         metrics.HitRates,
		"migration_count":   metrics.MigrationCount,
		"last_updated":      metrics.Timestamp,
	})
}

// TriggerMigrationRequest 手动迁移请求
// 触发迁移请求
// type TriggerMigrationRequest struct {
// 	Path     string `json:"path" binding:"required"`
// 	FromTier string `json:"from_tier" binding:"required"`
// 	ToTier   string `json:"to_tier" binding:"required"`
// 	FileSize int64  `json:"file_size"`
// }

// TriggerMigration handles POST /api/smarttiering/migrate.
// 手动触发迁移
func (h *Handler) TriggerMigration(c *gin.Context) {
	var req ForceMigrateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.manager.ForceMigrate(
		c.Request.Context(),
		req.Path,
		ParseTier(req.FromTier),
		ParseTier(req.ToTier),
		req.FileSize,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "migration queued",
		"path":      req.Path,
		"from_tier": req.FromTier,
		"to_tier":   req.ToTier,
	})
}

// GetMigrationRecommendations handles GET /api/smarttiering/recommendations.
// 返回迁移建议
func (h *Handler) GetMigrationRecommendations(c *gin.Context) {
	report, err := h.manager.GetCostReport(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"recommendations":      report.Recommendations,
		"total_savings":        report.SavingsPercent,
		"current_cost":         report.TotalCostPerMonth,
		"optimal_cost":         report.OptimalCostPerMonth,
		"recommendation_count": len(report.Recommendations),
	})
}

// ========== /tier 路由处理器 ==========

// ListPolicies handles GET /api/v1/tier/policies
func (h *Handler) ListPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": policies})
}

// CreatePolicyRequest 创建策略请求
type CreatePolicyRequest struct {
	Name        string     `json:"name" binding:"required"`
	Description string     `json:"description"`
	Priority    int        `json:"priority"`
	Rules       []TierRule `json:"rules"`
}

// CreatePolicy handles POST /api/v1/tier/policies
func (h *Handler) CreatePolicy(c *gin.Context) {
	var req CreatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "请求参数错误: " + err.Error()})
		return
	}

	policy := TierPolicy{
		Name:        req.Name,
		Description: req.Description,
		Priority:    req.Priority,
		Rules:       req.Rules,
		Enabled:     true,
	}

	result, err := h.manager.CreatePolicy(policy)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "ok", "data": result})
}

// DeletePolicyTier handles DELETE /api/v1/tier/policies/:id
func (h *Handler) DeletePolicyTier(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePolicy(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "策略已删除"})
}

// GetPlacement handles GET /api/v1/tier/placement?path=xxx
func (h *Handler) GetPlacement(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "path 参数必填"})
		return
	}

	placement, err := h.manager.GetPlacement(path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": placement})
}

// GetAllPlacements handles GET /api/v1/tier/placement/all
func (h *Handler) GetAllPlacements(c *gin.Context) {
	placements := h.manager.GetPlacements()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": placements, "count": len(placements)})
}

// TriggerMigrationTierRequest 迁移请求
type TriggerMigrationTierRequest struct {
	FilePath string `json:"file_path" binding:"required"`
	FromTier string `json:"from_tier" binding:"required"`
	ToTier   string `json:"to_tier" binding:"required"`
	FileSize int64  `json:"file_size"`
}

// TriggerMigrationTier handles POST /api/v1/tier/migrate
func (h *Handler) TriggerMigrationTier(c *gin.Context) {
	var req TriggerMigrationTierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "请求参数错误: " + err.Error()})
		return
	}

	err := h.manager.ForceMigrate(
		c.Request.Context(),
		req.FilePath,
		ParseTier(req.FromTier),
		ParseTier(req.ToTier),
		req.FileSize,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "迁移任务已提交"})
}

// GetTierStats handles GET /api/v1/tier/stats
func (h *Handler) GetTierStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": stats})
}

// GetAccessPattern handles GET /api/v1/tier/access-pattern?path=xxx
func (h *Handler) GetAccessPattern(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "path 参数必填"})
		return
	}

	pattern, err := h.manager.GetAccessPattern(path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": pattern})
}
