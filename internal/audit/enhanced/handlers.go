package enhanced

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 增强审计模块 HTTP 处理器
type Handlers struct {
	loginAuditor     *LoginAuditor
	operationAuditor *OperationAuditor
	sensitiveManager *SensitiveOperationManager
	reportGenerator  *ReportGenerator
}

// NewHandlers 创建增强审计处理器
func NewHandlers(
	loginAuditor *LoginAuditor,
	operationAuditor *OperationAuditor,
	sensitiveManager *SensitiveOperationManager,
) *Handlers {
	h := &Handlers{
		loginAuditor:     loginAuditor,
		operationAuditor: operationAuditor,
		sensitiveManager: sensitiveManager,
	}

	if loginAuditor != nil || operationAuditor != nil {
		h.reportGenerator = NewReportGenerator(loginAuditor, operationAuditor, sensitiveManager)
	}

	return h
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	audit := api.Group("/audit/enhanced")
	{
		// 登录审计
		login := audit.Group("/login")
		{
			login.GET("/entries", h.getLoginEntries)
			login.GET("/entries/:id", h.getLoginEntry)
			login.GET("/sessions", h.getActiveSessions)
			login.GET("/sessions/:session_id", h.getSession)
			login.DELETE("/sessions/:session_id", h.terminateSession)
			login.GET("/statistics", h.getLoginStatistics)
			login.GET("/high-risk", h.getHighRiskLogins)
			login.GET("/patterns/:user_id", h.getLoginPattern)
			login.POST("/record", h.recordLoginEvent)
		}

		// 操作审计
		operations := audit.Group("/operations")
		{
			operations.GET("/entries", h.getOperationEntries)
			operations.GET("/entries/:id", h.getOperationEntry)
			operations.GET("/chains/:correlation_id", h.getOperationChain)
			operations.GET("/resource", h.getResourceOperations)
			operations.GET("/user/:user_id", h.getUserOperations)
			operations.GET("/sensitive", h.getSensitiveOperations)
			operations.GET("/statistics", h.getOperationStatistics)
			operations.POST("/record", h.recordOperation)
			operations.POST("/chain/start", h.startOperationChain)
			operations.POST("/chain/:correlation_id/end", h.endOperationChain)
		}

		// 敏感操作管理
		sensitive := audit.Group("/sensitive")
		{
			sensitive.GET("/operations", h.listSensitiveOperations)
			sensitive.POST("/operations", h.addSensitiveOperation)
			sensitive.PUT("/operations/:id", h.updateSensitiveOperation)
			sensitive.DELETE("/operations/:id", h.deleteSensitiveOperation)
			sensitive.GET("/events", h.getSensitiveEvents)
			sensitive.GET("/summary", h.getSensitiveSummary)
			sensitive.GET("/approvals", h.getPendingApprovals)
			sensitive.GET("/approvals/:id", h.getApproval)
			sensitive.POST("/approvals/:id/approve", h.approveOperation)
			sensitive.POST("/approvals/:id/reject", h.rejectOperation)
		}

		// 报告生成
		reports := audit.Group("/reports")
		{
			reports.GET("", h.listReports)
			reports.POST("/generate", h.generateReport)
			reports.GET("/:report_id", h.getReport)
			reports.DELETE("/:report_id", h.deleteReport)
		}

		// 仪表板数据
		audit.GET("/dashboard", h.getDashboardData)
	}
}

// ========== 登录审计处理器 ==========

// getLoginEntries 获取登录审计条目
func (h *Handlers) getLoginEntries(c *gin.Context) {
	if h.loginAuditor == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "登录审计未启用"))
		return
	}

	// 解析查询参数
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil {
		limit = 50
	}
	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil {
		offset = 0
	}

	opts := LoginQueryOptions{
		Limit:  limit,
		Offset: offset,
	}

	// 时间范围
	if startTime := c.Query("start_time"); startTime != "" {
		t, err := time.Parse(time.RFC3339, startTime)
		if err == nil {
			opts.StartTime = &t
		}
	}
	if endTime := c.Query("end_time"); endTime != "" {
		t, err := time.Parse(time.RFC3339, endTime)
		if err == nil {
			opts.EndTime = &t
		}
	}

	// 筛选条件
	opts.UserID = c.Query("user_id")
	opts.Username = c.Query("username")
	opts.IP = c.Query("ip")
	opts.EventType = LoginEventType(c.Query("event_type"))
	opts.AuthMethod = AuthMethod(c.Query("auth_method"))
	opts.Status = c.Query("status")
	if minScore := c.Query("min_risk_score"); minScore != "" {
		if score, err := strconv.Atoi(minScore); err == nil {
			opts.MinRiskScore = score
		}
	}

	entries, total := h.loginAuditor.Query(opts)

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"total":   total,
		"entries": entries,
	}))
}

// getLoginEntry 获取单个登录审计条目
func (h *Handlers) getLoginEntry(c *gin.Context) {
	if h.loginAuditor == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "登录审计未启用"))
		return
	}

	id := c.Param("id")
	entry := h.loginAuditor.GetByID(id)

	if entry == nil {
		c.JSON(http.StatusNotFound, ErrorResponse(404, "登录记录不存在"))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(entry))
}

// getActiveSessions 获取活跃会话
func (h *Handlers) getActiveSessions(c *gin.Context) {
	if h.loginAuditor == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "登录审计未启用"))
		return
	}

	userID := c.Query("user_id")
	if userID != "" {
		sessions := h.loginAuditor.GetUserActiveSessions(userID)
		c.JSON(http.StatusOK, SuccessResponse(sessions))
		return
	}

	// 返回当前用户的会话
	currentUserID := c.GetString("user_id")
	if currentUserID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, "缺少用户ID"))
		return
	}

	sessions := h.loginAuditor.GetUserActiveSessions(currentUserID)
	c.JSON(http.StatusOK, SuccessResponse(sessions))
}

// getSession 获取会话详情
func (h *Handlers) getSession(c *gin.Context) {
	if h.loginAuditor == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "登录审计未启用"))
		return
	}

	sessionID := c.Param("session_id")
	session := h.loginAuditor.GetActiveSession(sessionID)

	if session == nil {
		c.JSON(http.StatusNotFound, ErrorResponse(404, "会话不存在或已过期"))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(session))
}

// terminateSession 终止会话
func (h *Handlers) terminateSession(c *gin.Context) {
	if h.loginAuditor == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "登录审计未启用"))
		return
	}

	sessionID := c.Param("session_id")
	success := h.loginAuditor.TerminateSession(sessionID)

	if !success {
		c.JSON(http.StatusNotFound, ErrorResponse(404, "会话不存在"))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{"message": "会话已终止"}))
}

// getLoginStatistics 获取登录统计
func (h *Handlers) getLoginStatistics(c *gin.Context) {
	if h.loginAuditor == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "登录审计未启用"))
		return
	}

	// 解析时间范围
	end := time.Now()
	start := end.Add(-24 * time.Hour)

	if st := c.Query("start_time"); st != "" {
		t, err := time.Parse(time.RFC3339, st)
		if err == nil {
			start = t
		}
	}
	if et := c.Query("end_time"); et != "" {
		t, err := time.Parse(time.RFC3339, et)
		if err == nil {
			end = t
		}
	}

	stats := h.loginAuditor.GetLoginStatistics(start, end)
	c.JSON(http.StatusOK, SuccessResponse(stats))
}

// getHighRiskLogins 获取高风险登录
func (h *Handlers) getHighRiskLogins(c *gin.Context) {
	if h.loginAuditor == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "登录审计未启用"))
		return
	}

	minScore := 70
	if score := c.Query("min_score"); score != "" {
		if s, err := strconv.Atoi(score); err == nil {
			minScore = s
		}
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}

	entries := h.loginAuditor.GetHighRiskLogins(minScore, limit)
	c.JSON(http.StatusOK, SuccessResponse(entries))
}

// getLoginPattern 获取用户登录模式
func (h *Handlers) getLoginPattern(c *gin.Context) {
	if h.loginAuditor == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "登录审计未启用"))
		return
	}

	userID := c.Param("user_id")
	pattern := h.loginAuditor.GetLoginPattern(userID)

	if pattern == nil {
		c.JSON(http.StatusNotFound, ErrorResponse(404, "用户登录模式不存在"))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(pattern))
}

// recordLoginEvent 记录登录事件
func (h *Handlers) recordLoginEvent(c *gin.Context) {
	if h.loginAuditor == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "登录审计未启用"))
		return
	}

	var req struct {
		UserID        string     `json:"user_id" binding:"required"`
		Username      string     `json:"username" binding:"required"`
		IP            string     `json:"ip" binding:"required"`
		UserAgent     string     `json:"user_agent"`
		AuthMethod    AuthMethod `json:"auth_method"`
		Status        string     `json:"status" binding:"required"`
		FailureReason string     `json:"failure_reason"`
		DeviceID      string     `json:"device_id"`
		DeviceName    string     `json:"device_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, err.Error()))
		return
	}

	entry := h.loginAuditor.RecordLogin(
		req.UserID, req.Username, req.IP, req.UserAgent,
		req.AuthMethod, req.Status, req.FailureReason,
		req.DeviceID, req.DeviceName,
	)

	c.JSON(http.StatusCreated, SuccessResponse(entry))
}

// ========== 操作审计处理器 ==========

// getOperationEntries 获取操作审计条目
func (h *Handlers) getOperationEntries(c *gin.Context) {
	if h.operationAuditor == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "操作审计未启用"))
		return
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil {
		limit = 50
	}
	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil {
		offset = 0
	}

	opts := OperationQueryOptions{
		Limit:  limit,
		Offset: offset,
	}

	// 时间范围
	if startTime := c.Query("start_time"); startTime != "" {
		t, err := time.Parse(time.RFC3339, startTime)
		if err == nil {
			opts.StartTime = &t
		}
	}
	if endTime := c.Query("end_time"); endTime != "" {
		t, err := time.Parse(time.RFC3339, endTime)
		if err == nil {
			opts.EndTime = &t
		}
	}

	// 筛选条件
	opts.UserID = c.Query("user_id")
	opts.Username = c.Query("username")
	opts.IP = c.Query("ip")
	opts.Category = OperationCategory(c.Query("category"))
	opts.Action = OperationAction(c.Query("action"))
	opts.ResourceType = c.Query("resource_type")
	opts.ResourceID = c.Query("resource_id")
	opts.Status = c.Query("status")
	opts.CorrelationID = c.Query("correlation_id")

	if isSensitive := c.Query("is_sensitive"); isSensitive != "" {
		val := isSensitive == "true"
		opts.IsSensitive = &val
	}

	if minScore := c.Query("min_risk_score"); minScore != "" {
		if score, err := strconv.Atoi(minScore); err == nil {
			opts.MinRiskScore = score
		}
	}

	entries, total := h.operationAuditor.Query(opts)

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"total":   total,
		"entries": entries,
	}))
}

// getOperationEntry 获取单个操作审计条目
func (h *Handlers) getOperationEntry(c *gin.Context) {
	if h.operationAuditor == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "操作审计未启用"))
		return
	}

	id := c.Param("id")
	entry := h.operationAuditor.GetByID(id)

	if entry == nil {
		c.JSON(http.StatusNotFound, ErrorResponse(404, "操作记录不存在"))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(entry))
}

// getOperationChain 获取操作链
func (h *Handlers) getOperationChain(c *gin.Context) {
	if h.operationAuditor == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "操作审计未启用"))
		return
	}

	correlationID := c.Param("correlation_id")
	chain := h.operationAuditor.GetOperationChain(correlationID)

	if chain == nil {
		c.JSON(http.StatusNotFound, ErrorResponse(404, "操作链不存在"))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(chain))
}

// getResourceOperations 获取资源操作历史
func (h *Handlers) getResourceOperations(c *gin.Context) {
	if h.operationAuditor == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "操作审计未启用"))
		return
	}

	resourceType := c.Query("type")
	resourceID := c.Query("id")
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil {
		limit = 50
	}

	if resourceType == "" || resourceID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, "缺少资源类型或ID"))
		return
	}

	entries := h.operationAuditor.GetByResource(resourceType, resourceID, limit)
	c.JSON(http.StatusOK, SuccessResponse(entries))
}

// getUserOperations 获取用户操作历史
func (h *Handlers) getUserOperations(c *gin.Context) {
	if h.operationAuditor == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "操作审计未启用"))
		return
	}

	userID := c.Param("user_id")
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil {
		limit = 50
	}

	entries := h.operationAuditor.GetUserOperations(userID, limit)
	c.JSON(http.StatusOK, SuccessResponse(entries))
}

// getSensitiveOperations 获取敏感操作
func (h *Handlers) getSensitiveOperations(c *gin.Context) {
	if h.operationAuditor == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "操作审计未启用"))
		return
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil {
		limit = 50
	}

	entries := h.operationAuditor.GetSensitiveOperations(limit)
	c.JSON(http.StatusOK, SuccessResponse(entries))
}

// getOperationStatistics 获取操作统计
func (h *Handlers) getOperationStatistics(c *gin.Context) {
	if h.operationAuditor == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "操作审计未启用"))
		return
	}

	end := time.Now()
	start := end.Add(-24 * time.Hour)

	if st := c.Query("start_time"); st != "" {
		t, err := time.Parse(time.RFC3339, st)
		if err == nil {
			start = t
		}
	}
	if et := c.Query("end_time"); et != "" {
		t, err := time.Parse(time.RFC3339, et)
		if err == nil {
			end = t
		}
	}

	stats := h.operationAuditor.GetStatistics(start, end)
	c.JSON(http.StatusOK, SuccessResponse(stats))
}

// recordOperation 记录操作
func (h *Handlers) recordOperation(c *gin.Context) {
	if h.operationAuditor == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "操作审计未启用"))
		return
	}

	var req struct {
		UserID       string                 `json:"user_id" binding:"required"`
		Username     string                 `json:"username" binding:"required"`
		IP           string                 `json:"ip" binding:"required"`
		UserAgent    string                 `json:"user_agent"`
		SessionID    string                 `json:"session_id"`
		Category     OperationCategory      `json:"category" binding:"required"`
		Action       OperationAction        `json:"action" binding:"required"`
		ResourceType string                 `json:"resource_type" binding:"required"`
		ResourceID   string                 `json:"resource_id" binding:"required"`
		ResourceName string                 `json:"resource_name"`
		ResourcePath string                 `json:"resource_path"`
		Status       string                 `json:"status" binding:"required"`
		OldValue     interface{}            `json:"old_value"`
		NewValue     interface{}            `json:"new_value"`
		Details      map[string]interface{} `json:"details"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, err.Error()))
		return
	}

	entry := h.operationAuditor.RecordOperation(
		req.UserID, req.Username, req.IP, req.UserAgent, req.SessionID,
		req.Category, req.Action,
		req.ResourceType, req.ResourceID, req.ResourceName, req.ResourcePath,
		req.Status, req.OldValue, req.NewValue, req.Details,
	)

	c.JSON(http.StatusCreated, SuccessResponse(entry))
}

// startOperationChain 开始操作链
func (h *Handlers) startOperationChain(c *gin.Context) {
	if h.operationAuditor == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "操作审计未启用"))
		return
	}

	var req struct {
		UserID   string `json:"user_id" binding:"required"`
		Username string `json:"username" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, err.Error()))
		return
	}

	correlationID := h.operationAuditor.StartOperationChain(req.UserID, req.Username)
	c.JSON(http.StatusCreated, SuccessResponse(gin.H{"correlation_id": correlationID}))
}

// endOperationChain 结束操作链
func (h *Handlers) endOperationChain(c *gin.Context) {
	if h.operationAuditor == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "操作审计未启用"))
		return
	}

	correlationID := c.Param("correlation_id")
	status := c.DefaultQuery("status", "completed")

	h.operationAuditor.EndOperationChain(correlationID, status)
	c.JSON(http.StatusOK, SuccessResponse(gin.H{"message": "操作链已结束"}))
}

// ========== 敏感操作管理处理器 ==========

// listSensitiveOperations 列出敏感操作定义
func (h *Handlers) listSensitiveOperations(c *gin.Context) {
	if h.sensitiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "敏感操作管理未启用"))
		return
	}

	operations := h.sensitiveManager.ListOperations()
	c.JSON(http.StatusOK, SuccessResponse(operations))
}

// addSensitiveOperation 添加敏感操作定义
func (h *Handlers) addSensitiveOperation(c *gin.Context) {
	if h.sensitiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "敏感操作管理未启用"))
		return
	}

	var op SensitiveOperation
	if err := c.ShouldBindJSON(&op); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, err.Error()))
		return
	}

	if err := h.sensitiveManager.AddOperation(&op); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(500, err.Error()))
		return
	}
	c.JSON(http.StatusCreated, SuccessResponse(op))
}

// updateSensitiveOperation 更新敏感操作定义
func (h *Handlers) updateSensitiveOperation(c *gin.Context) {
	if h.sensitiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "敏感操作管理未启用"))
		return
	}

	id := c.Param("id")
	var op SensitiveOperation
	if err := c.ShouldBindJSON(&op); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, err.Error()))
		return
	}

	op.ID = id
	if err := h.sensitiveManager.UpdateOperation(&op); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, SuccessResponse(op))
}

// deleteSensitiveOperation 删除敏感操作定义
func (h *Handlers) deleteSensitiveOperation(c *gin.Context) {
	if h.sensitiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "敏感操作管理未启用"))
		return
	}

	id := c.Param("id")
	h.sensitiveManager.DeleteOperation(id)
	c.JSON(http.StatusOK, SuccessResponse(gin.H{"message": "已删除"}))
}

// getSensitiveEvents 获取敏感操作事件
func (h *Handlers) getSensitiveEvents(c *gin.Context) {
	if h.sensitiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "敏感操作管理未启用"))
		return
	}

	end := time.Now()
	start := end.Add(-24 * time.Hour)

	if st := c.Query("start_time"); st != "" {
		t, err := time.Parse(time.RFC3339, st)
		if err == nil {
			start = t
		}
	}
	if et := c.Query("end_time"); et != "" {
		t, err := time.Parse(time.RFC3339, et)
		if err == nil {
			end = t
		}
	}

	userID := c.Query("user_id")
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil {
		limit = 50
	}

	events := h.sensitiveManager.QueryEvents(start, end, userID, limit)
	c.JSON(http.StatusOK, SuccessResponse(events))
}

// getSensitiveSummary 获取敏感操作摘要
func (h *Handlers) getSensitiveSummary(c *gin.Context) {
	if h.sensitiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "敏感操作管理未启用"))
		return
	}

	end := time.Now()
	start := end.Add(-24 * time.Hour)

	if st := c.Query("start_time"); st != "" {
		t, err := time.Parse(time.RFC3339, st)
		if err == nil {
			start = t
		}
	}
	if et := c.Query("end_time"); et != "" {
		t, err := time.Parse(time.RFC3339, et)
		if err == nil {
			end = t
		}
	}

	summary := h.sensitiveManager.GetSummary(start, end)
	c.JSON(http.StatusOK, SuccessResponse(summary))
}

// getPendingApprovals 获取待审批列表
func (h *Handlers) getPendingApprovals(c *gin.Context) {
	if h.sensitiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "敏感操作管理未启用"))
		return
	}

	approvals := h.sensitiveManager.GetPendingApprovals()
	c.JSON(http.StatusOK, SuccessResponse(approvals))
}

// getApproval 获取审批详情
func (h *Handlers) getApproval(c *gin.Context) {
	if h.sensitiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "敏感操作管理未启用"))
		return
	}

	id := c.Param("id")
	approval := h.sensitiveManager.GetApproval(id)

	if approval == nil {
		c.JSON(http.StatusNotFound, ErrorResponse(404, "审批不存在"))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(approval))
}

// approveOperation 批准操作
func (h *Handlers) approveOperation(c *gin.Context) {
	if h.sensitiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "敏感操作管理未启用"))
		return
	}

	id := c.Param("id")
	var req struct {
		Notes string `json:"notes"`
	}

	c.ShouldBindJSON(&req) // 可选

	approvedBy := c.GetString("user_id")
	if approvedBy == "" {
		approvedBy = "system"
	}

	if err := h.sensitiveManager.ApproveOperation(id, approvedBy, req.Notes); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, SuccessResponse(gin.H{"message": "已批准"}))
}

// rejectOperation 拒绝操作
func (h *Handlers) rejectOperation(c *gin.Context) {
	if h.sensitiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "敏感操作管理未启用"))
		return
	}

	id := c.Param("id")
	var req struct {
		Reason string `json:"reason" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, "需要提供拒绝原因"))
		return
	}

	rejectedBy := c.GetString("user_id")
	if rejectedBy == "" {
		rejectedBy = "system"
	}

	if err := h.sensitiveManager.RejectOperation(id, rejectedBy, req.Reason); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, SuccessResponse(gin.H{"message": "已拒绝"}))
}

// ========== 报告处理器 ==========

// listReports 列出报告
func (h *Handlers) listReports(c *gin.Context) {
	if h.reportGenerator == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "报告生成未启用"))
		return
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil {
		limit = 20
	}
	reports, err := h.reportGenerator.ListReports(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(reports))
}

// generateReport 生成报告
func (h *Handlers) generateReport(c *gin.Context) {
	if h.reportGenerator == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "报告生成未启用"))
		return
	}

	var opts ReportGenerateOptions
	if err := c.ShouldBindJSON(&opts); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, err.Error()))
		return
	}

	// 设置默认时间范围
	if opts.PeriodStart.IsZero() {
		opts.PeriodStart = time.Now().Add(-24 * time.Hour)
	}
	if opts.PeriodEnd.IsZero() {
		opts.PeriodEnd = time.Now()
	}

	report, err := h.reportGenerator.GenerateReport(opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(500, err.Error()))
		return
	}

	// 保存报告
	if err := h.reportGenerator.SaveReport(report); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse(report))
}

// getReport 获取报告
func (h *Handlers) getReport(c *gin.Context) {
	if h.reportGenerator == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "报告生成未启用"))
		return
	}

	reportID := c.Param("report_id")
	report, err := h.reportGenerator.LoadReport(reportID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse(404, "报告不存在"))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(report))
}

// deleteReport 删除报告
func (h *Handlers) deleteReport(c *gin.Context) {
	if h.reportGenerator == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "报告生成未启用"))
		return
	}

	reportID := c.Param("report_id")
	// 实际删除文件
	filename := "/var/log/nas-os/audit/reports/" + reportID + ".json"
	if err := os.Remove(filename); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(500, "删除失败"))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{"message": "已删除"}))
}

// ========== 仪表板处理器 ==========

// getDashboardData 获取仪表板数据
func (h *Handlers) getDashboardData(c *gin.Context) {
	end := time.Now()
	start := end.Add(-24 * time.Hour)

	dashboard := gin.H{
		"period": gin.H{
			"start": start,
			"end":   end,
		},
	}

	// 登录统计
	if h.loginAuditor != nil {
		loginStats := h.loginAuditor.GetLoginStatistics(start, end)
		dashboard["login"] = loginStats

		highRisk := h.loginAuditor.GetHighRiskLogins(70, 10)
		dashboard["high_risk_logins"] = highRisk
	}

	// 操作统计
	if h.operationAuditor != nil {
		opStats := h.operationAuditor.GetStatistics(start, end)
		dashboard["operations"] = opStats

		sensitiveOps := h.operationAuditor.GetSensitiveOperations(10)
		dashboard["recent_sensitive_ops"] = sensitiveOps
	}

	// 敏感操作统计
	if h.sensitiveManager != nil {
		sensitiveSummary := h.sensitiveManager.GetSummary(start, end)
		dashboard["sensitive_summary"] = sensitiveSummary

		pendingApprovals := h.sensitiveManager.GetPendingApprovals()
		dashboard["pending_approvals"] = len(pendingApprovals)
	}

	// 风险分析
	if h.reportGenerator != nil {
		riskAnalysis := h.reportGenerator.generateRiskAnalysis(start, end)
		dashboard["risk_analysis"] = riskAnalysis
	}

	c.JSON(http.StatusOK, SuccessResponse(dashboard))
}
