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

		// 目标管理
		smartbackup.POST("/targets", h.CreateTarget)
		smartbackup.GET("/targets", h.ListTargets)
		smartbackup.GET("/targets/:id", h.GetTarget)
		smartbackup.DELETE("/targets/:id", h.DeleteTarget)

		// 策略分析
		smartbackup.POST("/analyze", h.AnalyzeStrategy)
		smartbackup.POST("/policies/:id/optimize-window", h.OptimizeBackupWindow)
		smartbackup.GET("/policies/:id/evaluate", h.EvaluatePolicy)

		// 执行记录
		smartbackup.POST("/executions", h.RecordExecution)
		smartbackup.GET("/executions", h.ListExecutions)
		smartbackup.GET("/executions/:id", h.GetExecution)

		// 备份链路
		smartbackup.POST("/chains", h.CreateBackupChain)
		smartbackup.GET("/chains", h.ListBackupChains)
		smartbackup.GET("/chains/:id", h.GetBackupChain)

		// 备份验证与恢复测试
		smartbackup.POST("/verify/:execution_id", h.VerifyBackup)
		smartbackup.POST("/test-recovery/:execution_id", h.TestRecovery)
		smartbackup.GET("/verifications", h.ListVerifications)
		smartbackup.GET("/verifications/:id", h.GetVerification)

		// 智能调度
		smartbackup.POST("/optimize-schedule", h.OptimizeSchedule)

		// 统计
		smartbackup.GET("/stats", h.GetStats)
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

// CreateTarget 创建备份目标
func (h *Handler) CreateTarget(c *gin.Context) {
	var target BackupTarget
	if err := c.ShouldBindJSON(&target); err != nil {
		h.logger.Error("Failed to bind request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.manager.CreateTarget(&target); err != nil {
		h.logger.Error("Failed to create target", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, target)
}

// GetTarget 获取备份目标
func (h *Handler) GetTarget(c *gin.Context) {
	id := c.Param("id")

	target, err := h.manager.GetTarget(id)
	if err != nil {
		h.logger.Error("Failed to get target", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, target)
}

// ListTargets 列出所有备份目标
func (h *Handler) ListTargets(c *gin.Context) {
	targets := h.manager.ListTargets()
	c.JSON(http.StatusOK, targets)
}

// DeleteTarget 删除备份目标
func (h *Handler) DeleteTarget(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteTarget(id); err != nil {
		h.logger.Error("Failed to delete target", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Target deleted successfully"})
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

// CreateBackupChain 创建备份链路
func (h *Handler) CreateBackupChain(c *gin.Context) {
	var chain BackupChain
	if err := c.ShouldBindJSON(&chain); err != nil {
		h.logger.Error("Failed to bind request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.manager.CreateBackupChain(&chain); err != nil {
		h.logger.Error("Failed to create backup chain", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, chain)
}

// GetBackupChain 获取备份链路
func (h *Handler) GetBackupChain(c *gin.Context) {
	id := c.Param("id")

	chain, err := h.manager.GetBackupChain(id)
	if err != nil {
		h.logger.Error("Failed to get backup chain", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, chain)
}

// ListBackupChains 列出备份链路
func (h *Handler) ListBackupChains(c *gin.Context) {
	policyID := c.Query("policy_id")

	chains := h.manager.ListBackupChains(policyID)
	c.JSON(http.StatusOK, chains)
}

// VerifyBackup 验证备份
func (h *Handler) VerifyBackup(c *gin.Context) {
	executionID := c.Param("execution_id")

	result, err := h.manager.VerifyBackup(executionID)
	if err != nil {
		h.logger.Error("Failed to verify backup", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// TestRecovery 测试恢复
func (h *Handler) TestRecovery(c *gin.Context) {
	executionID := c.Param("execution_id")

	var req struct {
		TestPath string `json:"test_path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.TestPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "test_path is required"})
		return
	}

	result, err := h.manager.TestRecovery(executionID, req.TestPath)
	if err != nil {
		h.logger.Error("Failed to test recovery", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetVerification 获取验证结果
func (h *Handler) GetVerification(c *gin.Context) {
	id := c.Param("id")

	result, err := h.manager.GetVerification(id)
	if err != nil {
		h.logger.Error("Failed to get verification", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ListVerifications 列出验证结果
func (h *Handler) ListVerifications(c *gin.Context) {
	executionID := c.Query("execution_id")

	results := h.manager.ListVerifications(executionID)
	c.JSON(http.StatusOK, results)
}

// OptimizeSchedule 优化调度
func (h *Handler) OptimizeSchedule(c *gin.Context) {
	var metrics LoadMetrics
	if err := c.ShouldBindJSON(&metrics); err != nil {
		h.logger.Error("Failed to bind request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	optimization, err := h.manager.OptimizeSchedule(&metrics)
	if err != nil {
		h.logger.Error("Failed to optimize schedule", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, optimization)
}

// GetStats 获取统计信息
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}
