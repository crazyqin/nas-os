package selfserviceportal

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler HTTP API handler.
type Handler struct {
	portal *Portal
}

// NewHandler 创建新的 handler.
func NewHandler(portal *Portal) *Handler {
	return &Handler{portal: portal}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	selfservice := rg.Group("/selfservice")
	{
		// 配额管理
		selfservice.POST("/quota/requests", h.submitQuotaRequest)
		selfservice.GET("/quota/requests/:id", h.getQuotaRequest)
		selfservice.GET("/quota/requests", h.listQuotaRequests)
		selfservice.POST("/quota/requests/:id/approve", h.approveQuotaRequest)
		selfservice.POST("/quota/requests/:id/reject", h.rejectQuotaRequest)

		// 权限管理
		selfservice.POST("/permissions/requests", h.submitPermissionRequest)
		selfservice.GET("/permissions/requests/:id", h.getPermissionRequest)
		selfservice.GET("/permissions/requests", h.listPermissionRequests)
		selfservice.POST("/permissions/requests/:id/approve", h.approvePermissionRequest)
		selfservice.POST("/permissions/requests/:id/reject", h.rejectPermissionRequest)

		// 备份恢复
		selfservice.POST("/backup/restore-points", h.createRestorePoint)
		selfservice.GET("/backup/restore-points", h.listRestorePoints)
		selfservice.POST("/backup/restore", h.submitRestoreRequest)
		selfservice.POST("/backup/restore/:id/approve", h.approveRestoreRequest)

		// 问题报告
		selfservice.POST("/issues", h.submitIssueTicket)
		selfservice.GET("/issues/:id", h.getIssueTicket)
		selfservice.GET("/issues", h.listIssueTickets)
		selfservice.POST("/issues/:id/assign", h.assignIssueTicket)
		selfservice.POST("/issues/:id/resolve", h.resolveIssueTicket)

		// 用户统计
		selfservice.GET("/stats", h.getUserStats)

		// 通知
		selfservice.GET("/notifications", h.listNotifications)
		selfservice.POST("/notifications/:id/read", h.markNotificationRead)

		// 审批记录
		selfservice.GET("/approvals/:ticketId", h.getApprovals)

		// 自动审批规则
		selfservice.GET("/rules", h.listAutoApprovalRules)
		selfservice.POST("/rules", h.addAutoApprovalRule)
	}
}

// ========== 配额管理 Handlers ==========

type submitQuotaRequestInput struct {
	UserID      string `json:"user_id" binding:"required"`
	CurrentGB   int64  `json:"current_gb" binding:"required"`
	RequestedGB int64  `json:"requested_gb" binding:"required"`
	Reason      string `json:"reason"`
}

func (h *Handler) submitQuotaRequest(c *gin.Context) {
	var input submitQuotaRequestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req, err := h.portal.SubmitQuotaRequest(input.UserID, input.CurrentGB, input.RequestedGB, input.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, req)
}

func (h *Handler) getQuotaRequest(c *gin.Context) {
	id := c.Param("id")
	req, err := h.portal.GetQuotaRequest(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, req)
}

func (h *Handler) listQuotaRequests(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}
	requests := h.portal.ListQuotaRequests(userID)
	c.JSON(http.StatusOK, requests)
}

type approvalInput struct {
	ApproverID string `json:"approver_id" binding:"required"`
	Comment    string `json:"comment"`
}

func (h *Handler) approveQuotaRequest(c *gin.Context) {
	id := c.Param("id")
	var input approvalInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.portal.ApproveQuotaRequest(id, input.ApproverID, input.Comment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "approved"})
}

func (h *Handler) rejectQuotaRequest(c *gin.Context) {
	id := c.Param("id")
	var input approvalInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.portal.RejectQuotaRequest(id, input.ApproverID, input.Comment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "rejected"})
}

// ========== 权限管理 Handlers ==========

type submitPermissionRequestInput struct {
	UserID    string         `json:"user_id" binding:"required"`
	SharePath string         `json:"share_path" binding:"required"`
	PermType  PermissionType `json:"perm_type" binding:"required"`
	Temporary bool           `json:"temporary"`
	ExpiresAt *time.Time     `json:"expires_at,omitempty"`
	Reason    string         `json:"reason"`
}

func (h *Handler) submitPermissionRequest(c *gin.Context) {
	var input submitPermissionRequestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req, err := h.portal.SubmitPermissionRequest(input.UserID, input.SharePath, input.PermType, input.Temporary, input.ExpiresAt, input.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, req)
}

func (h *Handler) getPermissionRequest(c *gin.Context) {
	id := c.Param("id")
	req, err := h.portal.GetPermissionRequest(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, req)
}

func (h *Handler) listPermissionRequests(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}
	requests := h.portal.ListPermissionRequests(userID)
	c.JSON(http.StatusOK, requests)
}

func (h *Handler) approvePermissionRequest(c *gin.Context) {
	id := c.Param("id")
	var input approvalInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.portal.ApprovePermissionRequest(id, input.ApproverID, input.Comment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "approved"})
}

func (h *Handler) rejectPermissionRequest(c *gin.Context) {
	id := c.Param("id")
	var input approvalInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.portal.RejectPermissionRequest(id, input.ApproverID, input.Comment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "rejected"})
}

// ========== 备份恢复 Handlers ==========

type createRestorePointInput struct {
	UserID    string `json:"user_id" binding:"required"`
	FilePath  string `json:"file_path" binding:"required"`
	SizeBytes int64  `json:"size_bytes"`
}

func (h *Handler) createRestorePoint(c *gin.Context) {
	var input createRestorePointInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rp := h.portal.CreateRestorePoint(input.UserID, input.FilePath, input.SizeBytes)
	c.JSON(http.StatusCreated, rp)
}

func (h *Handler) listRestorePoints(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}
	points := h.portal.ListRestorePoints(userID)
	c.JSON(http.StatusOK, points)
}

type submitRestoreRequestInput struct {
	UserID       string `json:"user_id" binding:"required"`
	RestorePoint string `json:"restore_point_id" binding:"required"`
	TargetPath   string `json:"target_path" binding:"required"`
}

func (h *Handler) submitRestoreRequest(c *gin.Context) {
	var input submitRestoreRequestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req, err := h.portal.SubmitRestoreRequest(input.UserID, input.RestorePoint, input.TargetPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, req)
}

func (h *Handler) approveRestoreRequest(c *gin.Context) {
	id := c.Param("id")
	var input approvalInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.portal.ApproveRestoreRequest(id, input.ApproverID, input.Comment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "approved"})
}

// ========== 问题报告 Handlers ==========

type submitIssueTicketInput struct {
	UserID      string `json:"user_id" binding:"required"`
	Subject     string `json:"subject" binding:"required"`
	Description string `json:"description" binding:"required"`
	Category    string `json:"category"`
	Priority    string `json:"priority"`
}

func (h *Handler) submitIssueTicket(c *gin.Context) {
	var input submitIssueTicketInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ticket := h.portal.SubmitIssueTicket(input.UserID, input.Subject, input.Description, input.Category, input.Priority)
	c.JSON(http.StatusCreated, ticket)
}

func (h *Handler) getIssueTicket(c *gin.Context) {
	id := c.Param("id")
	ticket, err := h.portal.GetIssueTicket(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, ticket)
}

func (h *Handler) listIssueTickets(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}
	tickets := h.portal.ListIssueTickets(userID)
	c.JSON(http.StatusOK, tickets)
}

type assignIssueInput struct {
	AssigneeID string `json:"assignee_id" binding:"required"`
}

func (h *Handler) assignIssueTicket(c *gin.Context) {
	id := c.Param("id")
	var input assignIssueInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.portal.AssignIssueTicket(id, input.AssigneeID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "assigned"})
}

type resolveIssueInput struct {
	Resolution string `json:"resolution" binding:"required"`
}

func (h *Handler) resolveIssueTicket(c *gin.Context) {
	id := c.Param("id")
	var input resolveIssueInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.portal.ResolveIssueTicket(id, input.Resolution); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "resolved"})
}

// ========== 用户统计 Handlers ==========

func (h *Handler) getUserStats(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}
	stats := h.portal.GetUserStats(userID)
	c.JSON(http.StatusOK, stats)
}

// ========== 通知管理 Handlers ==========

func (h *Handler) listNotifications(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}
	notifs := h.portal.ListNotifications(userID)
	c.JSON(http.StatusOK, notifs)
}

func (h *Handler) markNotificationRead(c *gin.Context) {
	userID := c.Query("user_id")
	notifID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	if err := h.portal.MarkNotificationRead(userID, notifID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "marked as read"})
}

// ========== 审批记录 Handlers ==========

func (h *Handler) getApprovals(c *gin.Context) {
	ticketID := c.Param("ticketId")
	approvals := h.portal.GetApprovals(ticketID)
	c.JSON(http.StatusOK, approvals)
}

// ========== 自动审批规则 Handlers ==========

func (h *Handler) listAutoApprovalRules(c *gin.Context) {
	rules := h.portal.ListAutoApprovalRules()
	c.JSON(http.StatusOK, rules)
}

type addAutoApprovalRuleInput struct {
	Name                string  `json:"name" binding:"required"`
	Description         string  `json:"description"`
	MaxAutoApproveGB    int64   `json:"max_auto_approve_gb"`
	MaxPercentOfCurrent float64 `json:"max_percent_of_current"`
	Enabled             bool    `json:"enabled"`
}

func (h *Handler) addAutoApprovalRule(c *gin.Context) {
	var input addAutoApprovalRuleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule := &AutoApprovalRule{
		Name:                input.Name,
		Description:         input.Description,
		MaxAutoApproveGB:    input.MaxAutoApproveGB,
		MaxPercentOfCurrent: input.MaxPercentOfCurrent,
		Enabled:             input.Enabled,
	}

	h.portal.AddAutoApprovalRule(rule)
	c.JSON(http.StatusCreated, rule)
}
