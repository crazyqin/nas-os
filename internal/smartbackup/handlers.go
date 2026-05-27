package smartbackup

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler HTTP 处理器
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建新的处理器实例
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	return &Handler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	smartbackup := rg.Group("/smartbackup")
	{
		// 策略管理
		smartbackup.POST("/policies", h.CreatePolicy)
		smartbackup.GET("/policies", h.ListPolicies)
		smartbackup.GET("/policies/:id", h.GetPolicy)
		smartbackup.PUT("/policies/:id", h.UpdatePolicy)
		smartbackup.DELETE("/policies/:id", h.DeletePolicy)

		// 策略分析
		smartbackup.POST("/analyze", h.AnalyzeStrategy)
		smartbackup.POST("/policies/:id/optimize-window", h.OptimizeBackupWindow)
		smartbackup.GET("/policies/:id/evaluate", h.EvaluatePolicy)

		// 执行记录
		smartbackup.POST("/executions", h.RecordExecution)
		smartbackup.GET("/executions", h.ListExecutions)
		smartbackup.GET("/executions/:id", h.GetExecution)
	}
}

// CreatePolicy 创建备份策略
func (h *Handler) CreatePolicy(c *gin.Context) {
	var policy BackupPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		h.logger.Error("Failed to bind request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.manager.CreatePolicy(&policy); err != nil {
		h.logger.Error("Failed to create policy", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, policy)
}

// GetPolicy 获取备份策略
func (h *Handler) GetPolicy(c *gin.Context) {
	id := c.Param("id")

	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		h.logger.Error("Failed to get policy", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, policy)
}

// ListPolicies 列出所有备份策略
func (h *Handler) ListPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()
	c.JSON(http.StatusOK, policies)
}

// UpdatePolicy 更新备份策略
func (h *Handler) UpdatePolicy(c *gin.Context) {
	id := c.Param("id")

	var policy BackupPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		h.logger.Error("Failed to bind request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	policy.ID = id
	if err := h.manager.UpdatePolicy(&policy); err != nil {
		h.logger.Error("Failed to update policy", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, policy)
}

// DeletePolicy 删除备份策略
func (h *Handler) DeletePolicy(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeletePolicy(id); err != nil {
		h.logger.Error("Failed to delete policy", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Policy deleted successfully"})
}

// AnalyzeStrategy 分析并推荐备份策略
func (h *Handler) AnalyzeStrategy(c *gin.Context) {
	var analysis StrategyAnalysis
	if err := c.ShouldBindJSON(&analysis); err != nil {
		h.logger.Error("Failed to bind request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	strategy, err := h.manager.AnalyzeStrategy(&analysis)
	if err != nil {
		h.logger.Error("Failed to analyze strategy", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, strategy)
}

// OptimizeBackupWindow 优化备份窗口
func (h *Handler) OptimizeBackupWindow(c *gin.Context) {
	id := c.Param("id")

	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		h.logger.Error("Failed to get policy", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	var changeFreq ChangeFrequency
	if err := c.ShouldBindJSON(&changeFreq); err != nil {
		// 如果没有提供变化频率，使用nil
		changeFreq = ChangeFrequency{}
	}

	optimization, err := h.manager.OptimizeBackupWindow(policy, &changeFreq)
	if err != nil {
		h.logger.Error("Failed to optimize backup window", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, optimization)
}

// EvaluatePolicy 评估策略效果
func (h *Handler) EvaluatePolicy(c *gin.Context) {
	id := c.Param("id")

	evaluation, err := h.manager.EvaluatePolicy(id)
	if err != nil {
		h.logger.Error("Failed to evaluate policy", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, evaluation)
}

// RecordExecution 记录备份执行
func (h *Handler) RecordExecution(c *gin.Context) {
	var execution BackupExecution
	if err := c.ShouldBindJSON(&execution); err != nil {
		h.logger.Error("Failed to bind request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.manager.RecordExecution(&execution); err != nil {
		h.logger.Error("Failed to record execution", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, execution)
}

// GetExecution 获取备份执行记录
func (h *Handler) GetExecution(c *gin.Context) {
	id := c.Param("id")

	execution, err := h.manager.GetExecution(id)
	if err != nil {
		h.logger.Error("Failed to get execution", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, execution)
}

// ListExecutions 列出执行记录
func (h *Handler) ListExecutions(c *gin.Context) {
	policyID := c.Query("policy_id")

	executions := h.manager.ListExecutions(policyID)
	c.JSON(http.StatusOK, executions)
}
