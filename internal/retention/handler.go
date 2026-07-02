package retention

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler 保留策略HTTP处理器.
type Handler struct {
	engine *RetentionEngine
}

// NewHandler 创建处理器.
func NewHandler(engine *RetentionEngine) *Handler {
	return &Handler{engine: engine}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	retention := rg.Group("/retention")
	{
		// 策略管理
		retention.GET("/policies", h.ListPolicies)
		retention.POST("/policies", h.CreatePolicy)
		retention.PUT("/policies/:id", h.UpdatePolicy)
		retention.DELETE("/policies/:id", h.DeletePolicy)
		retention.POST("/policies/:id/apply", h.ApplyPolicy)

		// 策略模拟
		retention.POST("/simulate", h.Simulate)

		// 法律保留
		retention.GET("/legal-holds", h.ListLegalHolds)
		retention.POST("/legal-holds", h.CreateLegalHold)
		retention.DELETE("/legal-holds/:id", h.ReleaseLegalHold)

		// 审计与合规
		retention.GET("/audit-log", h.GetAuditLog)
		retention.GET("/compliance-report", h.GetComplianceReport)
		retention.GET("/expiring", h.GetExpiringFiles)
	}
}

// ListPolicies 策略列表.
func (h *Handler) ListPolicies(c *gin.Context) {
	policies := h.engine.ListPolicies()
	c.JSON(http.StatusOK, gin.H{
		"policies": policies,
		"total":    len(policies),
	})
}

// CreatePolicy 创建策略.
func (h *Handler) CreatePolicy(c *gin.Context) {
	var policy RetentionPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	created, err := h.engine.CreatePolicy(&policy)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

// UpdatePolicy 更新策略.
func (h *Handler) UpdatePolicy(c *gin.Context) {
	id := c.Param("id")
	var update RetentionPolicy
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updated, err := h.engine.UpdatePolicy(id, &update)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

// DeletePolicy 删除策略.
func (h *Handler) DeletePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.engine.DeletePolicy(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "policy deleted"})
}

// ApplyPolicy 应用策略.
func (h *Handler) ApplyPolicy(c *gin.Context) {
	id := c.Param("id")
	result, err := h.engine.ApplyPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// Simulate 模拟策略影响.
func (h *Handler) Simulate(c *gin.Context) {
	var policy RetentionPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result := h.engine.Simulate(&policy)
	c.JSON(http.StatusOK, result)
}

// ListLegalHolds 法律保留列表.
func (h *Handler) ListLegalHolds(c *gin.Context) {
	holds := h.engine.ListLegalHolds()
	c.JSON(http.StatusOK, gin.H{
		"legalHolds": holds,
		"total":      len(holds),
	})
}

// CreateLegalHold 创建法律保留.
func (h *Handler) CreateLegalHold(c *gin.Context) {
	var hold LegalHold
	if err := c.ShouldBindJSON(&hold); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	created, err := h.engine.CreateLegalHold(&hold)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

// ReleaseLegalHold 解除法律保留.
func (h *Handler) ReleaseLegalHold(c *gin.Context) {
	id := c.Param("id")
	if err := h.engine.ReleaseLegalHold(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "legal hold released"})
}

// GetAuditLog 审计日志.
func (h *Handler) GetAuditLog(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 100
	}
	logs := h.engine.GetAuditLog(limit)
	c.JSON(http.StatusOK, gin.H{
		"entries": logs,
		"total":   len(logs),
	})
}

// GetComplianceReport 合规报告.
func (h *Handler) GetComplianceReport(c *gin.Context) {
	report := h.engine.GetComplianceReport()
	c.JSON(http.StatusOK, report)
}

// GetExpiringFiles 即将过期文件.
func (h *Handler) GetExpiringFiles(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "7")
	days, err := strconv.Atoi(daysStr)
	if err != nil {
		days = 7
	}
	files := h.engine.GetExpiringFiles(days)
	c.JSON(http.StatusOK, gin.H{
		"files": files,
		"total": len(files),
		"days":  days,
	})
}
