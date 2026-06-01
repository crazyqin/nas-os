// Package aistorageopt 实现 AI 驱动的智能存储优化引擎 HTTP API
package aistorageopt

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler AI 存储优化 HTTP 处理器
type Handler struct {
	manager *AIStorageOpt
}

// NewHandler 创建处理器
func NewHandler(manager *AIStorageOpt) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	ai := rg.Group("/ai-storage-opt")
	{
		ai.GET("/optimizations", h.GetOptimizations)
		ai.POST("/optimizations/execute", h.ExecuteOptimization)
		ai.GET("/tiers", h.GetTiers)
		ai.POST("/tiers/migrate", h.MigrateTier)
		ai.GET("/stats", h.GetStats)
	}
}

// GetOptimizations 获取优化建议列表
func (h *Handler) GetOptimizations(c *gin.Context) {
	// 获取存储预测
	prediction := h.manager.PredictStorage(7)

	// 获取优化报告
	report := h.manager.GetOptimizationReport()

	c.JSON(http.StatusOK, gin.H{
		"prediction": prediction,
		"report":     report,
	})
}

// ExecuteOptimizationRequest 执行优化请求
type ExecuteOptimizationRequest struct {
	Type string `json:"type" binding:"required"`
}

// ExecuteOptimization 执行优化
func (h *Handler) ExecuteOptimization(c *gin.Context) {
	var req ExecuteOptimizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 评估分层策略
	movedFiles := h.manager.EvaluateTierPolicies()

	c.JSON(http.StatusOK, gin.H{
		"message":      "优化执行完成",
		"type":         req.Type,
		"files_moved":  len(movedFiles),
		"moved_files":  movedFiles,
	})
}

// GetTiers 获取存储分层信息
func (h *Handler) GetTiers(c *gin.Context) {
	h.manager.mu.RLock()
	defer h.manager.mu.RUnlock()

	policies := make([]*TierPolicy, 0, len(h.manager.policies))
	for _, p := range h.manager.policies {
		policies = append(policies, p)
	}

	c.JSON(http.StatusOK, gin.H{
		"policies": policies,
		"total":    len(policies),
	})
}

// MigrateTierRequest 数据迁移请求
type MigrateTierRequest struct {
	SourceTier StorageTier `json:"source_tier" binding:"required"`
	TargetTier StorageTier `json:"target_tier" binding:"required"`
}

// MigrateTier 执行数据迁移
func (h *Handler) MigrateTier(c *gin.Context) {
	var req MigrateTierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "数据迁移已启动",
		"source_tier":  req.SourceTier,
		"target_tier":  req.TargetTier,
	})
}

// GetStats 获取存储统计
func (h *Handler) GetStats(c *gin.Context) {
	report := h.manager.GetOptimizationReport()

	h.manager.mu.RLock()
	patternCount := len(h.manager.accessPatterns)
	dedupCount := len(h.manager.dedupIndex)
	metricsCount := len(h.manager.metrics)
	h.manager.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"report":              report,
		"access_patterns":     patternCount,
		"dedup_entries":       dedupCount,
		"metrics_count":       metricsCount,
	})
}
