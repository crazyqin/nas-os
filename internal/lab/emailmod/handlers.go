package emailmod

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 邮件审核 HTTP 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{manager: mgr}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	emailMod := api.Group("/email-mod")
	{
		// 策略管理
		emailMod.GET("/policies", h.listPolicies)
		emailMod.POST("/policies", h.createPolicy)
		emailMod.GET("/policies/:id", h.getPolicy)
		emailMod.PUT("/policies/:id", h.updatePolicy)
		emailMod.DELETE("/policies/:id", h.deletePolicy)

		// 审核队列
		emailMod.GET("/queue", h.queryQueue)
		emailMod.GET("/queue/:id", h.getQueueItem)
		emailMod.POST("/queue/:id/approve", h.approve)
		emailMod.POST("/queue/:id/reject", h.reject)

		// 审计日志
		emailMod.GET("/audit", h.queryAudit)

		// 统计
		emailMod.GET("/stats", h.getStats)

		// 邮件提交（测试/集成用）
		emailMod.POST("/submit", h.submitEmail)
	}
}

// ==================== 策略 API ====================

// createPolicy 创建策略
// @Summary 创建审核策略
// @Description 创建邮件审核策略
// @Tags email-mod
// @Accept json
// @Produce json
// @Param body body PolicyInput true "策略参数"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /email-mod/policies [post].
func (h *Handlers) createPolicy(c *gin.Context) {
	var input PolicyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	p, err := h.manager.CreatePolicy(input)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse(p))
}

// listPolicies 列出策略
// @Summary 列出所有审核策略
// @Tags email-mod
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /email-mod/policies [get].
func (h *Handlers) listPolicies(c *gin.Context) {
	policies, err := h.manager.ListPolicies()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, SuccessResponse(policies))
}

// getPolicy 获取策略
// @Summary 获取审核策略详情
// @Tags email-mod
// @Produce json
// @Param id path string true "策略ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /email-mod/policies/{id} [get].
func (h *Handlers) getPolicy(c *gin.Context) {
	id := c.Param("id")
	p, err := h.manager.GetPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse(ErrCodeNotFound, ErrPolicyNotFound))
		return
	}
	c.JSON(http.StatusOK, SuccessResponse(p))
}

// updatePolicy 更新策略
// @Summary 更新审核策略
// @Tags email-mod
// @Accept json
// @Produce json
// @Param id path string true "策略ID"
// @Param body body PolicyInput true "策略参数"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /email-mod/policies/{id} [put].
func (h *Handlers) updatePolicy(c *gin.Context) {
	id := c.Param("id")
	var input PolicyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	p, err := h.manager.UpdatePolicy(id, input)
	if err != nil {
		if err.Error() == ErrPolicyNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse(ErrCodeNotFound, ErrPolicyNotFound))
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(p))
}

// deletePolicy 删除策略
// @Summary 删除审核策略
// @Tags email-mod
// @Produce json
// @Param id path string true "策略ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /email-mod/policies/{id} [delete].
func (h *Handlers) deletePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePolicy(id); err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse(ErrCodeNotFound, ErrPolicyNotFound))
		return
	}
	c.JSON(http.StatusOK, SuccessResponse(nil))
}

// ==================== 审核队列 API ====================

// queryQueue 查询审核队列
// @Summary 查询审核队列
// @Tags email-mod
// @Produce json
// @Param status query string false "状态 (pending/approved/rejected)"
// @Param policy_id query string false "策略ID"
// @Param from query string false "发件人"
// @Param keyword query string false "关键词"
// @Param limit query int false "分页限制" default(50)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} map[string]interface{}
// @Router /email-mod/queue [get].
func (h *Handlers) queryQueue(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	opts := QueueQueryOptions{
		Status:   ReviewStatus(c.Query("status")),
		PolicyID: c.Query("policy_id"),
		From:     c.Query("from"),
		Keyword:  c.Query("keyword"),
		Limit:    limit,
		Offset:   offset,
	}

	items, total, err := h.manager.QueryQueue(opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(map[string]interface{}{
		"items":  items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}))
}

// getQueueItem 获取队列条目
// @Summary 获取审核队列条目详情
// @Tags email-mod
// @Produce json
// @Param id path string true "队列ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /email-mod/queue/{id} [get].
func (h *Handlers) getQueueItem(c *gin.Context) {
	id := c.Param("id")
	item, err := h.manager.GetQueueItem(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse(ErrCodeNotFound, ErrQueueItemNotFound))
		return
	}
	c.JSON(http.StatusOK, SuccessResponse(item))
}

// approve 批准邮件
// @Summary 批准邮件
// @Tags email-mod
// @Accept json
// @Produce json
// @Param id path string true "队列ID"
// @Param body body ReviewInput false "审核意见"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /email-mod/queue/{id}/approve [post].
func (h *Handlers) approve(c *gin.Context) {
	id := c.Param("id")

	var input ReviewInput
	_ = c.ShouldBindJSON(&input)

	// 从上下文获取审核人信息
	reviewerID := c.GetString("user_id")
	reviewerName := c.GetString("username")
	if reviewerID == "" {
		reviewerID = "admin"
		reviewerName = "管理员"
	}

	item, err := h.manager.Approve(id, reviewerID, reviewerName, input.Comment)
	if err != nil {
		switch err.Error() {
		case ErrQueueItemNotFound:
			c.JSON(http.StatusNotFound, ErrorResponse(ErrCodeNotFound, ErrQueueItemNotFound))
		case ErrAlreadyReviewed:
			c.JSON(http.StatusConflict, ErrorResponse(ErrCodeConflict, ErrAlreadyReviewed))
		case ErrNotCurrentReviewer:
			c.JSON(http.StatusForbidden, ErrorResponse(403, ErrNotCurrentReviewer))
		default:
			c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(item))
}

// reject 拒绝邮件
// @Summary 拒绝邮件
// @Tags email-mod
// @Accept json
// @Produce json
// @Param id path string true "队列ID"
// @Param body body ReviewInput false "审核意见"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /email-mod/queue/{id}/reject [post].
func (h *Handlers) reject(c *gin.Context) {
	id := c.Param("id")

	var input ReviewInput
	_ = c.ShouldBindJSON(&input)

	reviewerID := c.GetString("user_id")
	reviewerName := c.GetString("username")
	if reviewerID == "" {
		reviewerID = "admin"
		reviewerName = "管理员"
	}

	item, err := h.manager.Reject(id, reviewerID, reviewerName, input.Comment)
	if err != nil {
		switch err.Error() {
		case ErrQueueItemNotFound:
			c.JSON(http.StatusNotFound, ErrorResponse(ErrCodeNotFound, ErrQueueItemNotFound))
		case ErrAlreadyReviewed:
			c.JSON(http.StatusConflict, ErrorResponse(ErrCodeConflict, ErrAlreadyReviewed))
		case ErrNotCurrentReviewer:
			c.JSON(http.StatusForbidden, ErrorResponse(403, ErrNotCurrentReviewer))
		default:
			c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(item))
}

// ==================== 审计日志 API ====================

// queryAudit 查询审计日志
// @Summary 查询审核审计日志
// @Tags email-mod
// @Produce json
// @Param status query string false "状态"
// @Param policy_id query string false "策略ID"
// @Param reviewer_id query string false "审核人ID"
// @Param keyword query string false "关键词"
// @Param start_time query string false "开始时间 (RFC3339)"
// @Param end_time query string false "结束时间 (RFC3339)"
// @Param limit query int false "分页限制" default(50)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} map[string]interface{}
// @Router /email-mod/audit [get].
func (h *Handlers) queryAudit(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	opts := AuditQueryOptions{
		Status:     ReviewStatus(c.Query("status")),
		PolicyID:   c.Query("policy_id"),
		ReviewerID: c.Query("reviewer_id"),
		Keyword:    c.Query("keyword"),
		Limit:      limit,
		Offset:     offset,
	}

	if st := c.Query("start_time"); st != "" {
		t, err := time.Parse(time.RFC3339, st)
		if err == nil {
			opts.StartTime = &t
		}
	}
	if et := c.Query("end_time"); et != "" {
		t, err := time.Parse(time.RFC3339, et)
		if err == nil {
			opts.EndTime = &t
		}
	}

	entries, total, err := h.manager.QueryAudit(opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(map[string]interface{}{
		"entries": entries,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	}))
}

// ==================== 统计 API ====================

// getStats 获取审核统计
// @Summary 获取审核统计信息
// @Tags email-mod
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /email-mod/stats [get].
func (h *Handlers) getStats(c *gin.Context) {
	stats, err := h.manager.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, SuccessResponse(stats))
}

// ==================== 邮件提交 API ====================

// submitEmailRequest 提交邮件请求.
type submitEmailRequest struct {
	From        string       `json:"from" binding:"required"`
	To          []string     `json:"to" binding:"required,min=1"`
	CC          []string     `json:"cc"`
	Subject     string       `json:"subject"`
	Body        string       `json:"body"`
	Attachments []Attachment `json:"attachments"`
}

// submitEmail 提交邮件到审核
// @Summary 提交邮件到审核队列
// @Description 提交邮件，根据策略决定是否需要审核
// @Tags email-mod
// @Accept json
// @Produce json
// @Param body body submitEmailRequest true "邮件信息"
// @Success 200 {object} map[string]interface{}
// @Router /email-mod/submit [post].
func (h *Handlers) submitEmail(c *gin.Context) {
	var req submitEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	item, err := h.manager.SubmitEmail(req.From, req.To, req.CC, req.Subject, req.Body, req.Attachments)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	if item == nil {
		// 无匹配策略，放行
		c.JSON(http.StatusOK, SuccessResponse(map[string]interface{}{
			"queued":  false,
			"message": "邮件已放行，无需审核",
		}))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(map[string]interface{}{
		"queued":  true,
		"message": "邮件已进入审核队列",
		"item":    item,
	}))
}
