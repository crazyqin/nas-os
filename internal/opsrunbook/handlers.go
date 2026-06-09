package opsrunbook

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers HTTP API 处理器
type Handlers struct {
	manager  *Manager
	executor *Executor
}

// NewHandlers 创建 HTTP 处理器
func NewHandlers(manager *Manager, executor *Executor) *Handlers {
	return &Handlers{
		manager:  manager,
		executor: executor,
	}
}

// RegisterRoutes 注册 API 路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	runbooks := r.Group("/runbooks")
	{
		runbooks.GET("", h.ListRunbooks)
		runbooks.POST("", h.CreateRunbook)
		runbooks.GET("/:id", h.GetRunbook)
		runbooks.PUT("/:id", h.UpdateRunbook)
		runbooks.DELETE("/:id", h.DeleteRunbook)
		runbooks.POST("/:id/execute", h.ExecuteRunbook)
		runbooks.GET("/:id/stats", h.GetRunbookStats)
		runbooks.GET("/:id/executions", h.ListRunbookExecutions)
	}

	executions := r.Group("/executions")
	{
		executions.GET("", h.ListExecutions)
		executions.GET("/:id", h.GetExecution)
		executions.GET("/:id/logs", h.GetExecutionLogs)
	}

	approvals := r.Group("/approvals")
	{
		approvals.GET("", h.ListPendingApprovals)
		approvals.POST("/:id/approve", h.ApproveStep)
		approvals.POST("/:id/reject", h.RejectStep)
	}

	templates := r.Group("/templates")
	{
		templates.GET("", h.ListTemplates)
		templates.POST("/:id/instantiate", h.InstantiateTemplate)
	}
}

// === 运维手册 CRUD ===

type createRunbookRequest struct {
	Name        string    `json:"name" binding:"required"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Severity    Severity  `json:"severity"`
	Tags        []string  `json:"tags"`
	Trigger     TriggerType `json:"trigger"`
	Steps       []*Step   `json:"steps" binding:"required"`
	Variables   []*Variable `json:"variables"`
	Timeout     string    `json:"timeout"`
	RollbackOn  string    `json:"rollback_on"`
	Author      string    `json:"author"`
}

// ListRunbooks 列出运维手册
func (h *Handlers) ListRunbooks(c *gin.Context) {
	var filter RunbookFilter
	filter.Category = c.Query("category")
	filter.Severity = Severity(c.Query("severity"))
	filter.Status = RunbookStatus(c.Query("status"))
	filter.Trigger = TriggerType(c.Query("trigger"))
	filter.Search = c.Query("search")

	runbooks := h.manager.ListRunbooks(filter)
	c.JSON(http.StatusOK, gin.H{
		"runbooks": runbooks,
		"total":    len(runbooks),
	})
}

// CreateRunbook 创建运维手册
func (h *Handlers) CreateRunbook(c *gin.Context) {
	var req createRunbookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id := generateID("rb")
	timeout := 30 * time.Minute
	if req.Timeout != "" {
		if d, err := time.ParseDuration(req.Timeout); err == nil {
			timeout = d
		}
	}

	rollbackOn := req.RollbackOn
	if rollbackOn == "" {
		rollbackOn = "failure"
	}

	rb := &Runbook{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		Severity:    req.Severity,
		Tags:        req.Tags,
		Trigger:     req.Trigger,
		Steps:       req.Steps,
		Variables:   req.Variables,
		Timeout:     timeout,
		RollbackOn:  rollbackOn,
		Author:      req.Author,
	}

	if err := h.manager.RegisterRunbook(rb); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, rb)
}

// GetRunbook 获取运维手册详情
func (h *Handlers) GetRunbook(c *gin.Context) {
	id := c.Param("id")
	rb, err := h.manager.GetRunbook(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rb)
}

// UpdateRunbook 更新运维手册
func (h *Handlers) UpdateRunbook(c *gin.Context) {
	id := c.Param("id")
	existing, err := h.manager.GetRunbook(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	var req createRunbookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	existing.Name = req.Name
	existing.Description = req.Description
	existing.Category = req.Category
	existing.Severity = req.Severity
	existing.Tags = req.Tags
	existing.Trigger = req.Trigger
	existing.Steps = req.Steps
	existing.Variables = req.Variables
	existing.RollbackOn = req.RollbackOn
	if req.Timeout != "" {
		if d, err := time.ParseDuration(req.Timeout); err == nil {
			existing.Timeout = d
		}
	}

	if err := h.manager.UpdateRunbook(existing); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, existing)
}

// DeleteRunbook 删除运维手册
func (h *Handlers) DeleteRunbook(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteRunbook(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "runbook deleted"})
}

// === 执行 ===

type executeRequest struct {
	Variables  map[string]string `json:"variables"`
	Operator   string            `json:"operator"`
}

// ExecuteRunbook 执行运维手册
func (h *Handlers) ExecuteRunbook(c *gin.Context) {
	id := c.Param("id")
	var req executeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许空 body
		req = executeRequest{}
	}

	operator := req.Operator
	if operator == "" {
		operator = "api"
	}

	execution, err := h.executor.Execute(
		c.Request.Context(),
		id,
		TriggerManual,
		"",
		req.Variables,
		operator,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, execution)
}

// GetRunbookStats 获取运维手册执行统计
func (h *Handlers) GetRunbookStats(c *gin.Context) {
	id := c.Param("id")
	stats, err := h.manager.GetExecutionStats(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// ListRunbookExecutions 列出运维手册的执行记录
func (h *Handlers) ListRunbookExecutions(c *gin.Context) {
	id := c.Param("id")
	executions := h.manager.ListExecutions(ExecutionFilter{
		RunbookID: id,
		Limit:     50,
	})
	c.JSON(http.StatusOK, gin.H{
		"executions": executions,
		"total":      len(executions),
	})
}

// === 执行记录 ===

// ListExecutions 列出所有执行记录
func (h *Handlers) ListExecutions(c *gin.Context) {
	filter := ExecutionFilter{
		Status: StepStatus(c.Query("status")),
	}
	executions := h.manager.ListExecutions(filter)
	c.JSON(http.StatusOK, gin.H{
		"executions": executions,
		"total":      len(executions),
	})
}

// GetExecution 获取执行记录详情
func (h *Handlers) GetExecution(c *gin.Context) {
	id := c.Param("id")
	exec, err := h.manager.GetExecution(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, exec)
}

// GetExecutionLogs 获取执行日志
func (h *Handlers) GetExecutionLogs(c *gin.Context) {
	id := c.Param("id")
	exec, err := h.manager.GetExecution(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	var logs []gin.H
	for _, step := range exec.Steps {
		logs = append(logs, gin.H{
			"step_id":   step.StepID,
			"step_name": step.StepName,
			"status":    step.Status,
			"output":    step.Output,
			"error":     step.Error,
			"started":   step.StartedAt,
			"duration":  step.Duration.String(),
			"retries":   step.Retries,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"execution_id": id,
		"logs":         logs,
	})
}

// === 审批 ===

type approvalAction struct {
	Operator string `json:"operator" binding:"required"`
	Reason   string `json:"reason"`
}

// ListPendingApprovals 列出待审批请求
func (h *Handlers) ListPendingApprovals(c *gin.Context) {
	h.manager.mu.RLock()
	defer h.manager.mu.RUnlock()

	var pending []*ApprovalRequest
	for _, req := range h.manager.approvals {
		if req.ApprovedAt == nil {
			pending = append(pending, req)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"approvals": pending,
		"total":     len(pending),
	})
}

// ApproveStep 审批通过
func (h *Handlers) ApproveStep(c *gin.Context) {
	id := c.Param("id")
	var req approvalAction
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.Approve(id, req.Operator, req.Reason); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "approved"})
}

// RejectStep 审批拒绝
func (h *Handlers) RejectStep(c *gin.Context) {
	id := c.Param("id")
	var req approvalAction
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.Reject(id, req.Operator, req.Reason); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "rejected"})
}

// === 模板 ===

// ListTemplates 列出可用模板
func (h *Handlers) ListTemplates(c *gin.Context) {
	templates := LoadBuiltInTemplates()
	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"total":     len(templates),
	})
}

// InstantiateTemplate 从模板创建运维手册
func (h *Handlers) InstantiateTemplate(c *gin.Context) {
	templateID := c.Param("id")
	templates := LoadBuiltInTemplates()

	var template *Runbook
	for _, t := range templates {
		if t.ID == templateID {
			template = t
			break
		}
	}

	if template == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}

	var overrides struct {
		Name     string   `json:"name"`
		Tags     []string `json:"tags"`
		Author   string   `json:"author"`
	}
	c.ShouldBindJSON(&overrides)

	// 创建副本
	rb := *template
	rb.ID = generateID("rb")
	if overrides.Name != "" {
		rb.Name = overrides.Name
	}
	if overrides.Author != "" {
		rb.Author = overrides.Author
	}
	if len(overrides.Tags) > 0 {
		rb.Tags = overrides.Tags
	}

	if err := h.manager.RegisterRunbook(&rb); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, rb)
}

// generateID 生成唯一ID
func generateID(prefix string) string {
	return prefix + "_" + time.Now().Format("20060102150405") + "_" + randomHex(6)
}

// randomHex 生成随机十六进制字符串
func randomHex(n int) string {
	const hex = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = hex[time.Now().UnixNano()%16]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
