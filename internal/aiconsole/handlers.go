package aiconsole

import (
	"net/http"
	"strconv"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers AI Console HTTP 处理器.
type Handlers struct {
	service *Service
}

// NewHandlers 创建处理器实例.
func NewHandlers(service *Service) *Handlers {
	return &Handlers{service: service}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(apiGroup *gin.RouterGroup) {
	aiConsole := apiGroup.Group("/ai-console")
	{
		// 模型管理
		aiConsole.POST("/models", h.CreateModel)
		aiConsole.GET("/models", h.ListModels)
		aiConsole.GET("/models/:id", h.GetModel)
		aiConsole.DELETE("/models/:id", h.DeleteModel)

		// 聊天
		aiConsole.POST("/chat", h.Chat)

		// 脱敏规则 CRUD
		aiConsole.POST("/redact-rules", h.CreateRule)
		aiConsole.GET("/redact-rules", h.ListRules)
		aiConsole.GET("/redact-rules/:id", h.GetRule)
		aiConsole.PUT("/redact-rules/:id", h.UpdateRule)
		aiConsole.DELETE("/redact-rules/:id", h.DeleteRule)

		// 审计日志
		aiConsole.GET("/audit", h.QueryAuditLogs)
	}
}

// ==================== 模型管理 ====================

// CreateModel 添加 AI 模型配置
// @Summary 添加 AI 模型配置
// @Description 创建新的 AI 模型连接配置
// @Tags ai-console
// @Accept json
// @Produce json
// @Param request body CreateModelRequest true "模型配置"
// @Success 201 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /api/v1/ai-console/models [post].
func (h *Handlers) CreateModel(c *gin.Context) {
	var req CreateModelRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	m, err := h.service.CreateModel(req)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.Created(c, m)
}

// ListModels 列出所有模型
// @Summary 列出 AI 模型
// @Description 获取所有已配置的 AI 模型
// @Tags ai-console
// @Produce json
// @Success 200 {object} api.Response
// @Router /api/v1/ai-console/models [get].
func (h *Handlers) ListModels(c *gin.Context) {
	models, err := h.service.ListModels()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	// 隐藏 API Key
	for _, m := range models {
		if m.APIKey != "" {
			m.APIKey = maskAPIKey(m.APIKey)
		}
	}

	api.OK(c, models)
}

// GetModel 获取单个模型
// @Summary 获取 AI 模型详情
// @Tags ai-console
// @Produce json
// @Param id path string true "模型 ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /api/v1/ai-console/models/{id} [get].
func (h *Handlers) GetModel(c *gin.Context) {
	id := c.Param("id")
	m, err := h.service.GetModel(id)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}
	if m == nil {
		api.NotFound(c, "模型不存在")
		return
	}
	if m.APIKey != "" {
		m.APIKey = maskAPIKey(m.APIKey)
	}
	api.OK(c, m)
}

// DeleteModel 删除模型
// @Summary 删除 AI 模型
// @Tags ai-console
// @Param id path string true "模型 ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /api/v1/ai-console/models/{id} [delete].
func (h *Handlers) DeleteModel(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteModel(id); err != nil {
		api.InternalError(c, err.Error())
		return
	}
	api.OKWithMessage(c, "删除成功", nil)
}

// ==================== 聊天 ====================

// Chat 发送聊天请求（自动脱敏）
// @Summary 发送聊天请求
// @Description 发送聊天请求，自动对 PII 信息进行脱敏处理
// @Tags ai-console
// @Accept json
// @Produce json
// @Param request body ChatRequest true "聊天请求"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /api/v1/ai-console/chat [post].
func (h *Handlers) Chat(c *gin.Context) {
	var req ChatRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if len(req.Messages) == 0 {
		api.BadRequest(c, "消息不能为空")
		return
	}

	userID := c.GetString("user_id")
	username := c.GetString("username")
	ip := c.ClientIP()

	resp, _, err := h.service.Chat(c.Request.Context(), req, userID, username, ip)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, resp)
}

// ==================== 脱敏规则 CRUD ====================

// CreateRule 创建脱敏规则
// @Summary 创建脱敏规则
// @Description 创建新的隐私数据脱敏规则
// @Tags ai-console
// @Accept json
// @Produce json
// @Param request body CreateRuleRequest true "脱敏规则"
// @Success 201 {object} api.Response
// @Failure 400 {object} api.Response
// @Router /api/v1/ai-console/redact-rules [post].
func (h *Handlers) CreateRule(c *gin.Context) {
	var req CreateRuleRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	r, err := h.service.CreateRule(req)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.Created(c, r)
}

// ListRules 列出脱敏规则
// @Summary 列出脱敏规则
// @Tags ai-console
// @Produce json
// @Success 200 {object} api.Response
// @Router /api/v1/ai-console/redact-rules [get].
func (h *Handlers) ListRules(c *gin.Context) {
	rules, err := h.service.ListRules()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}
	api.OK(c, rules)
}

// GetRule 获取脱敏规则
// @Summary 获取脱敏规则详情
// @Tags ai-console
// @Param id path string true "规则 ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /api/v1/ai-console/redact-rules/{id} [get].
func (h *Handlers) GetRule(c *gin.Context) {
	id := c.Param("id")
	r, err := h.service.GetRule(id)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}
	if r == nil {
		api.NotFound(c, "规则不存在")
		return
	}
	api.OK(c, r)
}

// UpdateRule 更新脱敏规则
// @Summary 更新脱敏规则
// @Tags ai-console
// @Accept json
// @Param id path string true "规则 ID"
// @Param request body UpdateRuleRequest true "更新内容"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /api/v1/ai-console/redact-rules/{id} [put].
func (h *Handlers) UpdateRule(c *gin.Context) {
	id := c.Param("id")
	var req UpdateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	r, err := h.service.UpdateRule(id, req)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, r)
}

// DeleteRule 删除脱敏规则
// @Summary 删除脱敏规则
// @Tags ai-console
// @Param id path string true "规则 ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /api/v1/ai-console/redact-rules/{id} [delete].
func (h *Handlers) DeleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteRule(id); err != nil {
		api.InternalError(c, err.Error())
		return
	}
	api.OKWithMessage(c, "删除成功", nil)
}

// ==================== 审计日志 ====================

// QueryAuditLogs 查询审计日志
// @Summary 查询 AI 审计日志
// @Tags ai-console
// @Produce json
// @Param userId query string false "用户 ID"
// @Param action query string false "操作类型"
// @Param success query bool false "是否成功"
// @Param page query int false "页码"
// @Param pageSize query int false "每页条数"
// @Success 200 {object} api.Response
// @Router /api/v1/ai-console/audit [get].
func (h *Handlers) QueryAuditLogs(c *gin.Context) {
	var filter AuditQueryFilter
	filter.UserID = c.Query("userId")
	filter.Action = c.Query("action")

	if successStr := c.Query("success"); successStr != "" {
		success := successStr == "true"
		filter.Success = &success
	}

	if pageStr := c.Query("page"); pageStr != "" {
		if v, err := strconv.Atoi(pageStr); err == nil && v > 0 {
			filter.Page = v
		}
	}
	if pageSizeStr := c.Query("pageSize"); pageSizeStr != "" {
		if v, err := strconv.Atoi(pageSizeStr); err == nil && v > 0 {
			filter.PageSize = v
		}
	}

	entries, total, err := h.service.QueryAuditLogs(filter)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	api.Page(c, entries, total, page, pageSize)
}

// ==================== 工具函数 ====================

// maskAPIKey 对 API Key 进行掩码处理.
func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

// bindAndValidate 通用请求绑定与验证（兼容性封装，供未使用 api 包的场景）.
func bindAndValidate(c *gin.Context, obj interface{}) error {
	if err := c.ShouldBindJSON(obj); err != nil {
		return err
	}
	return nil
}

// jsonOK 返回成功 JSON 响应（兼容性封装）.
func jsonOK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data})
}
