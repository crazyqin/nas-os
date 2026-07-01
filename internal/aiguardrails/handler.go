package aiguardrails

import (
	"fmt"
	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers AI 安全护栏 HTTP 处理器.
type Handlers struct {
	svc *Service
}

// NewHandlers 创建 AI 安全护栏处理器.
func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

// RegisterRoutes 注册 AI 护栏路由.
func (h *Handlers) RegisterRoutes(apiGroup *gin.RouterGroup) {
	ag := apiGroup.Group("/aiguardrails")
	{
		ag.POST("/filter/input", h.filterInput)
		ag.POST("/filter/output", h.filterOutput)
		ag.GET("/config", h.getConfig)
		ag.PUT("/config", h.updateConfig)
		ag.POST("/policy", h.createPolicy)
		ag.GET("/policy", h.listPolicies)
		ag.GET("/policy/:id", h.getPolicy)
		ag.PUT("/policy/:id", h.updatePolicy)
		ag.DELETE("/policy/:id", h.deletePolicy)
		ag.PATCH("/policy/:id/toggle", h.togglePolicy)
		ag.GET("/audit", h.queryAudit)
	}
}

// filterInput 过滤输入
// @Summary 过滤输入文本
// @Description 对 AI 输入文本进行安全护栏检测，包括 PII 检测、Prompt Injection 防护等
// @Tags aiguardrails
// @Accept json
// @Produce json
// @Param request body FilterRequest true "过滤请求"
// @Success 200 {object} api.Response{data=FilterResponse}
// @Failure 400 {object} api.Response
// @Router /aiguardrails/filter/input [post].
func (h *Handlers) filterInput(c *gin.Context) {
	var req FilterRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	resp, err := h.svc.FilterInput(req)
	if err != nil {
		if err == ErrInputBlocked {
			api.OK(c, resp)
			return
		}
		if err == ErrInputTooLong {
			api.BadRequest(c, err.Error())
			return
		}
		if err == ErrModelBlocked {
			api.Forbidden(c, err.Error())
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, resp)
}

// filterOutput 过滤输出
// @Summary 过滤输出文本
// @Description 对 AI 输出文本进行安全护栏检测
// @Tags aiguardrails
// @Accept json
// @Produce json
// @Param request body FilterRequest true "过滤请求"
// @Success 200 {object} api.Response{data=FilterResponse}
// @Failure 400 {object} api.Response
// @Router /aiguardrails/filter/output [post].
func (h *Handlers) filterOutput(c *gin.Context) {
	var req FilterRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	resp, err := h.svc.FilterOutput(req)
	if err != nil {
		if err == ErrOutputBlocked {
			api.OK(c, resp)
			return
		}
		if err == ErrOutputTooLong {
			api.BadRequest(c, err.Error())
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, resp)
}

// getConfig 获取全局配置
// @Summary 获取 AI 护栏全局配置
// @Tags aiguardrails
// @Produce json
// @Success 200 {object} api.Response{data=AIGuardrailConfig}
// @Router /aiguardrails/config [get].
func (h *Handlers) getConfig(c *gin.Context) {
	api.OK(c, h.svc.GetConfig())
}

// updateConfig 更新全局配置
// @Summary 更新 AI 护栏全局配置
// @Tags aiguardrails
// @Accept json
// @Produce json
// @Param request body ConfigRequest true "配置请求"
// @Success 200 {object} api.Response{data=AIGuardrailConfig}
// @Router /aiguardrails/config [put].
func (h *Handlers) updateConfig(c *gin.Context) {
	var req ConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	cfg := h.svc.UpdateConfig(req)
	api.OK(c, cfg)
}

// createPolicy 创建策略
// @Summary 创建 AI 护栏策略
// @Tags aiguardrails
// @Accept json
// @Produce json
// @Param request body PolicyRequest true "策略请求"
// @Success 201 {object} api.Response{data=GuardrailPolicy}
// @Failure 400 {object} api.Response
// @Router /aiguardrails/policy [post].
func (h *Handlers) createPolicy(c *gin.Context) {
	var req PolicyRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	policy, err := h.svc.CreatePolicy(req)
	if err != nil {
		if err == ErrInvalidPolicyType || err == ErrInvalidRuleType {
			api.BadRequest(c, err.Error())
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.Created(c, policy)
}

// listPolicies 列出所有策略
// @Summary 列出 AI 护栏策略
// @Tags aiguardrails
// @Produce json
// @Success 200 {object} api.Response{data=ListResponse}
// @Router /aiguardrails/policy [get].
func (h *Handlers) listPolicies(c *gin.Context) {
	policies := h.svc.ListPolicies()
	api.OK(c, ListResponse{
		Items: policies,
		Total: len(policies),
	})
}

// getPolicy 获取策略详情
// @Summary 获取 AI 护栏策略详情
// @Tags aiguardrails
// @Produce json
// @Param id path string true "策略 ID"
// @Success 200 {object} api.Response{data=GuardrailPolicy}
// @Failure 404 {object} api.Response
// @Router /aiguardrails/policy/{id} [get].
func (h *Handlers) getPolicy(c *gin.Context) {
	policyID := c.Param("id")
	if policyID == "" {
		api.BadRequest(c, "策略 ID 不能为空")
		return
	}

	policy, err := h.svc.GetPolicy(policyID)
	if err != nil {
		if err == ErrPolicyNotFound {
			api.NotFound(c, "护栏策略未找到")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, policy)
}

// updatePolicy 更新策略
// @Summary 更新 AI 护栏策略
// @Tags aiguardrails
// @Accept json
// @Produce json
// @Param id path string true "策略 ID"
// @Param request body PolicyRequest true "策略请求"
// @Success 200 {object} api.Response{data=GuardrailPolicy}
// @Failure 400 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /aiguardrails/policy/{id} [put].
func (h *Handlers) updatePolicy(c *gin.Context) {
	policyID := c.Param("id")
	if policyID == "" {
		api.BadRequest(c, "策略 ID 不能为空")
		return
	}

	var req PolicyRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	policy, err := h.svc.UpdatePolicy(policyID, req)
	if err != nil {
		if err == ErrPolicyNotFound {
			api.NotFound(c, "护栏策略未找到")
			return
		}
		if err == ErrInvalidPolicyType || err == ErrInvalidRuleType {
			api.BadRequest(c, err.Error())
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, policy)
}

// deletePolicy 删除策略
// @Summary 删除 AI 护栏策略
// @Tags aiguardrails
// @Produce json
// @Param id path string true "策略 ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /aiguardrails/policy/{id} [delete].
func (h *Handlers) deletePolicy(c *gin.Context) {
	policyID := c.Param("id")
	if policyID == "" {
		api.BadRequest(c, "策略 ID 不能为空")
		return
	}

	if err := h.svc.DeletePolicy(policyID); err != nil {
		if err == ErrPolicyNotFound {
			api.NotFound(c, "护栏策略未找到")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "策略已删除", nil)
}

// togglePolicy 启用/禁用策略
// @Summary 启用或禁用 AI 护栏策略
// @Tags aiguardrails
// @Accept json
// @Produce json
// @Param id path string true "策略 ID"
// @Param request body object true "启用状态" example({"enabled": true})
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /aiguardrails/policy/{id}/toggle [patch].
func (h *Handlers) togglePolicy(c *gin.Context) {
	policyID := c.Param("id")
	if policyID == "" {
		api.BadRequest(c, "策略 ID 不能为空")
		return
	}

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.svc.TogglePolicy(policyID, body.Enabled); err != nil {
		if err == ErrPolicyNotFound {
			api.NotFound(c, "护栏策略未找到")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "策略状态已更新", nil)
}

// queryAudit 查询审计日志
// @Summary 查询 AI 护栏审计日志
// @Description 按条件查询 AI 安全护栏审计日志
// @Tags aiguardrails
// @Produce json
// @Param direction query string false "方向 input/output"
// @Param user query string false "用户"
// @Param action query string false "动作"
// @Param limit query int false "返回条数上限"
// @Success 200 {object} api.Response{data=[]AuditLogEntry}
// @Router /aiguardrails/audit [get].
func (h *Handlers) queryAudit(c *gin.Context) {
	query := AuditQuery{
		Direction: c.Query("direction"),
		User:      c.Query("user"),
		Action:    Action(c.Query("action")),
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && limit > 0 {
			query.Limit = limit
		}
	}

	entries := h.svc.QueryAudit(query)
	api.OK(c, entries)
}