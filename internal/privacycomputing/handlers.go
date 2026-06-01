package privacycomputing

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册隐私计算模块的 HTTP 路由
func RegisterRoutes(r *gin.RouterGroup) {
	engine := NewPrivacyEngine()
	h := &Handler{engine: engine}

	pcGroup := r.Group("/privacycomputing")
	{
		// 健康检查
		pcGroup.GET("/health", h.Health)

		// 联邦学习
		federatedGroup := pcGroup.Group("/federated")
		{
			federatedGroup.POST("/tasks", h.CreateFederatedTask)
			federatedGroup.GET("/tasks", h.ListFederatedTasks)
			federatedGroup.GET("/tasks/:id", h.GetFederatedTask)
			federatedGroup.POST("/tasks/:id/start", h.StartFederatedTraining)
			federatedGroup.POST("/tasks/:id/evaluate", h.EvaluateFederatedModel)
			federatedGroup.DELETE("/tasks/:id", h.DeleteFederatedTask)
		}

		// 安全多方计算
		mpcGroup := pcGroup.Group("/mpc")
		{
			mpcGroup.POST("/protocols", h.CreateMPCProtocol)
			mpcGroup.GET("/protocols", h.ListMPCProtocols)
			mpcGroup.GET("/protocols/:id", h.GetMPCProtocol)
			mpcGroup.POST("/protocols/:id/start", h.StartMPCComputation)
			mpcGroup.DELETE("/protocols/:id", h.DeleteMPCProtocol)
			mpcGroup.POST("/secret/split", h.SplitSecret)
			mpcGroup.POST("/secret/reconstruct", h.ReconstructSecret)
		}

		// 差分隐私
		dpGroup := pcGroup.Group("/differential")
		{
			dpGroup.GET("/config", h.GetDifferentialConfig)
			dpGroup.PUT("/config", h.SetDifferentialConfig)
			dpGroup.GET("/budget", h.GetPrivacyBudget)
			dpGroup.POST("/budget", h.SetPrivacyBudget)
			dpGroup.POST("/noise", h.AddNoise)
			dpGroup.POST("/noise/mean", h.PrivateMean)
			dpGroup.POST("/noise/histogram", h.PrivateHistogram)
			dpGroup.POST("/noise/count", h.PrivateCount)
			dpGroup.POST("/budget/reset", h.ResetPrivacyBudget)
			dpGroup.GET("/queries", h.GetQueryLogs)
		}

		// 数据脱敏
		maskingGroup := pcGroup.Group("/masking")
		{
			maskingGroup.POST("/rules", h.CreateMaskRule)
			maskingGroup.GET("/rules", h.ListMaskRules)
			maskingGroup.GET("/rules/:id", h.GetMaskRule)
			maskingGroup.PUT("/rules/:id", h.UpdateMaskRule)
			maskingGroup.DELETE("/rules/:id", h.DeleteMaskRule)
			maskingGroup.POST("/apply", h.ApplyMask)
			maskingGroup.POST("/apply/table", h.ApplyTableMask)
			maskingGroup.PUT("/rules/:id/toggle", h.ToggleMaskRule)
		}
	}
}

// Handler HTTP API 处理器
type Handler struct {
	engine *PrivacyEngine
}

// NewPrivacyEngine 创建隐私计算引擎
func NewPrivacyEngine() *PrivacyEngine {
	return &PrivacyEngine{
		federatedMgr:    NewFederatedManager(),
		mpcMgr:          NewMPCManager(),
		differentialMgr: NewDifferentialManager(),
		maskingMgr:      NewMaskingManager(),
	}
}

// Health 健康检查
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "privacycomputing",
		"modules": []string{"federated", "mpc", "differential", "masking"},
	})
}

// ==================== 联邦学习 API ====================

// CreateFederatedTask 创建联邦学习任务
func (h *Handler) CreateFederatedTask(c *gin.Context) {
	var req CreateFederatedTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	task, err := h.engine.federatedMgr.CreateTask(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, task)
}

// ListFederatedTasks 列出联邦学习任务
func (h *Handler) ListFederatedTasks(c *gin.Context) {
	tasks := h.engine.federatedMgr.ListTasks()
	c.JSON(http.StatusOK, tasks)
}

// GetFederatedTask 获取联邦学习任务
func (h *Handler) GetFederatedTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.engine.federatedMgr.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

// StartFederatedTraining 开始联邦学习训练
func (h *Handler) StartFederatedTraining(c *gin.Context) {
	id := c.Param("id")
	if err := h.engine.federatedMgr.StartTraining(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "训练已启动", "task_id": id})
}

// EvaluateFederatedModel 评估联邦学习模型
func (h *Handler) EvaluateFederatedModel(c *gin.Context) {
	id := c.Param("id")
	metrics, err := h.engine.federatedMgr.EvaluateModel(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, metrics)
}

// DeleteFederatedTask 删除联邦学习任务
func (h *Handler) DeleteFederatedTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.engine.federatedMgr.DeleteTask(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务已删除"})
}

// ==================== 安全多方计算 API ====================

// CreateMPCProtocol 创建MPC协议
func (h *Handler) CreateMPCProtocol(c *gin.Context) {
	var req CreateMPCProtocolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	protocol, err := h.engine.mpcMgr.CreateProtocol(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, protocol)
}

// ListMPCProtocols 列出MPC协议
func (h *Handler) ListMPCProtocols(c *gin.Context) {
	protocols := h.engine.mpcMgr.ListProtocols()
	c.JSON(http.StatusOK, protocols)
}

// GetMPCProtocol 获取MPC协议
func (h *Handler) GetMPCProtocol(c *gin.Context) {
	id := c.Param("id")
	protocol, err := h.engine.mpcMgr.GetProtocol(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, protocol)
}

// StartMPCComputation 开始MPC计算
func (h *Handler) StartMPCComputation(c *gin.Context) {
	id := c.Param("id")
	if err := h.engine.mpcMgr.StartComputation(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "计算已启动", "protocol_id": id})
}

// DeleteMPCProtocol 删除MPC协议
func (h *Handler) DeleteMPCProtocol(c *gin.Context) {
	id := c.Param("id")
	if err := h.engine.mpcMgr.DeleteProtocol(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "协议已删除"})
}

// SplitSecret 分割秘密
func (h *Handler) SplitSecret(c *gin.Context) {
	var req struct {
		Secret    []byte `json:"secret"`
		N         int    `json:"n"`
		Threshold int    `json:"threshold"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	shares, err := h.engine.mpcMgr.SplitSecret(req.Secret, req.N, req.Threshold)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"shares": shares})
}

// ReconstructSecret 重构秘密
func (h *Handler) ReconstructSecret(c *gin.Context) {
	var req struct {
		Shares []SecretShare `json:"shares"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	secret, err := h.engine.mpcMgr.ReconstructSecret(req.Shares)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"secret": secret})
}

// ==================== 差分隐私 API ====================

// GetDifferentialConfig 获取差分隐私配置
func (h *Handler) GetDifferentialConfig(c *gin.Context) {
	config := h.engine.differentialMgr.GetConfig()
	c.JSON(http.StatusOK, config)
}

// SetDifferentialConfig 设置差分隐私配置
func (h *Handler) SetDifferentialConfig(c *gin.Context) {
	var config DifferentialPrivacyConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if err := h.engine.differentialMgr.SetConfig(config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "配置已更新"})
}

// GetPrivacyBudget 获取隐私预算
func (h *Handler) GetPrivacyBudget(c *gin.Context) {
	budget := h.engine.differentialMgr.GetBudget()
	c.JSON(http.StatusOK, budget)
}

// SetPrivacyBudget 设置隐私预算
func (h *Handler) SetPrivacyBudget(c *gin.Context) {
	var req struct {
		Epsilon float64 `json:"epsilon"`
		Delta   float64 `json:"delta"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if err := h.engine.differentialMgr.SetBudget(req.Epsilon, req.Delta); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "预算已更新"})
}

// AddNoise 添加差分隐私噪声
func (h *Handler) AddNoise(c *gin.Context) {
	var req AddNoiseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	response, err := h.engine.differentialMgr.AddNoise(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// PrivateMean 计算隐私保护的均值
func (h *Handler) PrivateMean(c *gin.Context) {
	var req struct {
		Data    []float64 `json:"data"`
		Epsilon float64   `json:"epsilon"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	mean, err := h.engine.differentialMgr.PrivateMean(req.Data, req.Epsilon)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"mean": mean})
}

// PrivateHistogram 计算隐私保护的直方图
func (h *Handler) PrivateHistogram(c *gin.Context) {
	var req struct {
		Data    []int   `json:"data"`
		NBins   int     `json:"nbins"`
		Epsilon float64 `json:"epsilon"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	histogram, err := h.engine.differentialMgr.PrivateHistogram(req.Data, req.NBins, req.Epsilon)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"histogram": histogram})
}

// PrivateCount 计算隐私保护的计数
func (h *Handler) PrivateCount(c *gin.Context) {
	var req struct {
		Data    []bool  `json:"data"`
		Epsilon float64 `json:"epsilon"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	count, err := h.engine.differentialMgr.PrivateCount(req.Data, req.Epsilon)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"count": count})
}

// ResetPrivacyBudget 重置隐私预算
func (h *Handler) ResetPrivacyBudget(c *gin.Context) {
	h.engine.differentialMgr.ResetBudget()
	c.JSON(http.StatusOK, gin.H{"message": "隐私预算已重置"})
}

// GetQueryLogs 获取查询日志
func (h *Handler) GetQueryLogs(c *gin.Context) {
	logs := h.engine.differentialMgr.GetQueryLogs()
	c.JSON(http.StatusOK, logs)
}

// ==================== 数据脱敏 API ====================

// CreateMaskRule 创建脱敏规则
func (h *Handler) CreateMaskRule(c *gin.Context) {
	var req CreateMaskRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	rule, err := h.engine.maskingMgr.CreateRule(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, rule)
}

// ListMaskRules 列出脱敏规则
func (h *Handler) ListMaskRules(c *gin.Context) {
	rules := h.engine.maskingMgr.ListRules()
	c.JSON(http.StatusOK, rules)
}

// GetMaskRule 获取脱敏规则
func (h *Handler) GetMaskRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.engine.maskingMgr.GetRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rule)
}

// UpdateMaskRule 更新脱敏规则
func (h *Handler) UpdateMaskRule(c *gin.Context) {
	id := c.Param("id")
	var req CreateMaskRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	rule, err := h.engine.maskingMgr.UpdateRule(id, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rule)
}

// DeleteMaskRule 删除脱敏规则
func (h *Handler) DeleteMaskRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.engine.maskingMgr.DeleteRule(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "规则已删除"})
}

// ApplyMask 应用脱敏
func (h *Handler) ApplyMask(c *gin.Context) {
	var req ApplyMaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	result, err := h.engine.maskingMgr.ApplyMask(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ApplyTableMask 应用表格脱敏
func (h *Handler) ApplyTableMask(c *gin.Context) {
	var req ApplyTableMaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	result, err := h.engine.maskingMgr.ApplyTableMask(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ToggleMaskRule 切换脱敏规则状态
func (h *Handler) ToggleMaskRule(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if err := h.engine.maskingMgr.ToggleRule(id, req.Enabled); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "规则状态已更新"})
}
