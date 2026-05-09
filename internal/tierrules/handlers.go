package tierrules

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers 分层规则 HTTP 处理器.
type Handlers struct {
	engine *Engine
	logger *zap.Logger
}

// NewHandlers 创建分层规则 HTTP 处理器.
func NewHandlers(engine *Engine, logger *zap.Logger) *Handlers {
	return &Handlers{
		engine: engine,
		logger: logger,
	}
}

// RegisterRoutes 注册分层规则路由到 gin 路由组.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	rules := rg.Group("/tierrules")
	{
		rules.POST("", h.CreateRule)
		rules.GET("", h.ListRules)
		rules.DELETE("/:name", h.DeleteRule)
		rules.POST("/evaluate", h.EvaluateFile)
		rules.POST("/run", h.RunBatch)
		rules.GET("/stats", h.GetStats)
	}
}

// CreateRule 创建分层规则.
// POST /api/v1/tierrules
func (h *Handlers) CreateRule(c *gin.Context) {
	var req CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	rule := TierRule{
		Name:        req.Name,
		Description: req.Description,
		SourceTier:  req.SourceTier,
		TargetTier:  req.TargetTier,
		Conditions:  req.Conditions,
		Priority:    req.Priority,
		Enabled:     req.Enabled,
	}

	// 默认启用
	if !rule.Enabled && rule.Priority == 0 {
		rule.Enabled = true
	}

	if err := h.engine.AddRule(rule); err != nil {
		status := http.StatusInternalServerError
		switch err {
		case ErrRuleNameEmpty, ErrSameTier, ErrInvalidTier:
			status = http.StatusBadRequest
		case ErrRuleNameDuplicate:
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "规则创建成功",
		"rule":    rule,
	})
}

// ListRules 列出所有分层规则.
// GET /api/v1/tierrules
func (h *Handlers) ListRules(c *gin.Context) {
	rules := h.engine.ListRules()
	c.JSON(http.StatusOK, gin.H{
		"rules": rules,
		"total": len(rules),
	})
}

// DeleteRule 删除分层规则.
// DELETE /api/v1/tierrules/:name
func (h *Handlers) DeleteRule(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少规则名称"})
		return
	}

	if err := h.engine.RemoveRule(name); err != nil {
		if err == ErrRuleNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "规则删除成功",
		"name":    name,
	})
}

// EvaluateFile 评估文件应迁移到哪个层级.
// POST /api/v1/tierrules/evaluate
func (h *Handlers) EvaluateFile(c *gin.Context) {
	var req EvaluateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	tier, err := h.engine.Evaluate(req.File)
	if err != nil {
		if err == ErrNoMatchingRule {
			c.JSON(http.StatusOK, EvaluateResponse{
				File:            req.File.Path,
				CurrentTier:     req.File.CurrentTier,
				RecommendedTier: req.File.CurrentTier,
				MatchedRule:     "",
				ShouldMigrate:   false,
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, EvaluateResponse{
		File:            req.File.Path,
		CurrentTier:     req.File.CurrentTier,
		RecommendedTier: StorageTier(tier),
		MatchedRule:     "", // 匹配规则名由引擎内部记录
		ShouldMigrate:   true,
	})
}

// RunBatch 执行批量分层迁移.
// POST /api/v1/tierrules/run
func (h *Handlers) RunBatch(c *gin.Context) {
	var req RunRequest
	// 允许空 body
	_ = c.ShouldBindJSON(&req)

	var stats *TierStats
	var err error

	if req.DryRun {
		stats, err = h.engine.RunBatchDryRun(c.Request.Context())
	} else {
		stats, err = h.engine.RunBatch(c.Request.Context())
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, RunResponse{
		Stats:  *stats,
		DryRun: req.DryRun,
	})
}

// GetStats 获取分层迁移统计.
// GET /api/v1/tierrules/stats
func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.engine.GetStats()
	c.JSON(http.StatusOK, stats)
}
