package dockerresilience

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler Docker韧性增强HTTP处理器
type Handler struct {
	dr *DockerResilience
}

// NewHandler 创建处理器
func NewHandler(dr *DockerResilience) *Handler {
	return &Handler{dr: dr}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	group := rg.Group("/dockerresilience")
	{
		group.GET("/runs", h.GetRuns)
		group.GET("/checks", h.GetChecks)
		group.GET("/policy", h.GetPolicy)
		group.POST("/record", h.RecordRun)
		group.POST("/retry", h.CheckRetry)
		group.POST("/health", h.RunHealthChecks)
		group.PUT("/policy", h.UpdatePolicy)
	}
}

// GetRuns 获取运行记录
func (h *Handler) GetRuns(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": h.dr.GetRuns()})
}

// GetChecks 获取健康检查
func (h *Handler) GetChecks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": h.dr.GetChecks()})
}

// GetPolicy 获取重试策略
func (h *Handler) GetPolicy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": h.dr.GetPolicy()})
}

// RecordRun 记录运行
func (h *Handler) RecordRun(c *gin.Context) {
	var run WorkflowRun
	if err := c.ShouldBindJSON(&run); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.dr.RecordRun(run)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// CheckRetry 检查是否应重试
func (h *Handler) CheckRetry(c *gin.Context) {
	var req struct {
		RunID string `json:"runId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	shouldRetry := h.dr.ShouldRetry(req.RunID)
	delay := h.dr.GetRetryDelay(1)
	c.JSON(http.StatusOK, gin.H{
		"status":      "ok",
		"shouldRetry": shouldRetry,
		"delayMs":     delay.Milliseconds(),
	})
}

// RunHealthChecks 执行健康检查
func (h *Handler) RunHealthChecks(c *gin.Context) {
	checks := h.dr.RunHealthChecks()
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": checks})
}

// UpdatePolicy 更新重试策略
func (h *Handler) UpdatePolicy(c *gin.Context) {
	var policy RetryPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.dr.UpdatePolicy(policy)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
