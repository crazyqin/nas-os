package audit

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 审计模块 HTTP 处理器
type Handlers struct {
	manager  *Manager
	reporter *ComplianceReporter
}

// NewHandlers 创建审计处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{
		manager:  manager,
		reporter: NewComplianceReporter(manager),
	}
}

// RegisterRoutes 注册路由
// 注意：调用方应在应用此路由组前添加认证和权限中间件
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	audit := api.Group("/audit")
	{
		// 日志查询 - 需要审计查看权限
		audit.GET("/logs", h.getLogs)
		audit.GET("/logs/:id", h.getLogByID)
		audit.GET("/statistics", h.getStatistics)
		audit.GET("/timeline", h.getTimeline)

		// 仪表板数据
		audit.GET("/dashboard", h.getDashboard)

		// 导出 - 需要审计导出权限
		audit.GET("/export", h.exportLogs)

		// 完整性验证 - 需要管理员权限
		audit.GET("/integrity", h.verifyIntegrity)

		// 合规报告 - 需要管理员权限
		audit.GET("/compliance/report", h.getComplianceReport)
		audit.GET("/compliance/standards", h.getComplianceStandards)

		// 配置管理 - 需要管理员权限
		audit.GET("/config", h.getConfig)
		audit.PUT("/config", h.updateConfig)

		// 日志记录接口 - 内部使用，需要特殊权限
		audit.POST("/log", h.createLog)
		audit.POST("/log/auth", h.logAuth)
		audit.POST("/log/access", h.logAccess)
		audit.POST("/log/security", h.logSecurity)
		audit.POST("/log/data", h.logDataOperation)
		audit.POST("/log/config-change", h.logConfigChange)
	}
}

// ========== 日志查询 ==========

// getLogs 获取审计日志列表
func (h *Handlers) getLogs(c *gin.Context) {
	// 解析查询参数
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	opts := QueryOptions{
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
	if level := c.Query("level"); level != "" {
		opts.Level = Level(level)
	}
	if category := c.Query("category"); category != "" {
		opts.Category = Category(category)
	}
	if userID := c.Query("user_id"); userID != "" {
		opts.UserID = userID
	}
	if username := c.Query("username"); username != "" {
		opts.Username = username
	}
	if ip := c.Query("ip"); ip != "" {
		opts.IP = ip
	}
	if status := c.Query("status"); status != "" {
		opts.Status = Status(status)
	}
	if event := c.Query("event"); event != "" {
		opts.Event = event
	}
	if resource := c.Query("resource"); resource != "" {
		opts.Resource = resource
	}
	if keyword := c.Query("keyword"); keyword != "" {
		opts.Keyword = keyword
	}

	// 执行查询
	result, err := h.manager.Query(opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(result))
}

// getLogByID 根据ID获取日志
func (h *Handlers) getLogByID(c *gin.Context) {
	id := c.Param("id")

	entry, err := h.manager.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse(ErrCodeNotFound, ErrEntryNotFound))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(entry))
}

// getStatistics 获取审计统计
func (h *Handlers) getStatistics(c *gin.Context) {
	stats := h.manager.GetStatistics()
	c.JSON(http.StatusOK, SuccessResponse(stats))
}

// getTimeline 获取事件时间线
func (h *Handlers) getTimeline(c *gin.Context) {
	// 解析时间范围
	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	if st := c.Query("start_time"); st != "" {
		t, err := time.Parse(time.RFC3339, st)
		if err == nil {
			startTime = t
		}
	}
	if et := c.Query("end_time"); et != "" {
		t, err := time.Parse(time.RFC3339, et)
		if err == nil {
			endTime = t
		}
	}

	category := Category(c.Query("category"))

	timeline := h.reporter.GenerateTimeline(startTime, endTime, category)
	c.JSON(http.StatusOK, SuccessResponse(timeline))
}

// getDashboard 获取仪表板数据
func (h *Handlers) getDashboard(c *gin.Context) {
	data := h.reporter.GenerateDashboardData()
	c.JSON(http.StatusOK, SuccessResponse(data))
}

// ========== 导出 ==========

// exportLogs 导出日志
func (h *Handlers) exportLogs(c *gin.Context) {
	// 解析参数
	format := ExportFormat(c.DefaultQuery("format", "json"))
	if format != ExportJSON && format != ExportCSV && format != ExportXML {
		format = ExportJSON
	}

	// 时间范围
	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	if st := c.Query("start_time"); st != "" {
		t, err := time.Parse(time.RFC3339, st)
		if err == nil {
			startTime = t
		}
	}
	if et := c.Query("end_time"); et != "" {
		t, err := time.Parse(time.RFC3339, et)
		if err == nil {
			endTime = t
		}
	}

	// 分类筛选
	categories := make([]Category, 0)
	if cats := c.Query("categories"); cats != "" {
		for _, cat := range splitCategories(cats) {
			categories = append(categories, Category(cat))
		}
	}

	includeSigs := c.Query("include_signatures") == "true"

	opts := ExportOptions{
		Format:            format,
		StartTime:         startTime,
		EndTime:           endTime,
		Categories:        categories,
		IncludeSignatures: includeSigs,
	}

	data, err := h.manager.Export(opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, ErrExportFailed))
		return
	}

	// 设置响应头
	contentType := "application/json"
	fileExt := "json"
	switch format {
	case ExportCSV:
		contentType = "text/csv"
		fileExt = "csv"
	case ExportXML:
		contentType = "application/xml"
		fileExt = "xml"
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "attachment; filename=audit-logs."+fileExt)
	c.Data(http.StatusOK, contentType, data)
}

// splitCategories 分割分类字符串
func splitCategories(s string) []string {
	result := make([]string, 0)
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if i > start {
				result = append(result, s[start:i])
			}
			start = i + 1
		}
	}
	return result
}

// ========== 完整性验证 ==========

// verifyIntegrity 验证日志完整性
func (h *Handlers) verifyIntegrity(c *gin.Context) {
	report := h.manager.VerifyIntegrity()
	c.JSON(http.StatusOK, SuccessResponse(report))
}

// ========== 合规报告 ==========

// getComplianceReport 获取合规报告
func (h *Handlers) getComplianceReport(c *gin.Context) {
	// 解析参数
	standard := ComplianceStandard(c.DefaultQuery("standard", "gdpr"))
	if standard == "" {
		standard = ComplianceGDPR
	}

	// 时间范围
	startTime := time.Now().Add(-30 * 24 * time.Hour) // 默认30天
	endTime := time.Now()

	if st := c.Query("start_time"); st != "" {
		t, err := time.Parse(time.RFC3339, st)
		if err == nil {
			startTime = t
		}
	}
	if et := c.Query("end_time"); et != "" {
		t, err := time.Parse(time.RFC3339, et)
		if err == nil {
			endTime = t
		}
	}

	report, err := h.reporter.GenerateReport(standard, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(report))
}

// getComplianceStandards 获取支持的合规标准列表
func (h *Handlers) getComplianceStandards(c *gin.Context) {
	standards := []map[string]string{
		{"code": "gdpr", "name": "GDPR (欧盟通用数据保护条例)"},
		{"code": "mlps", "name": "等级保护 (中国网络安全等级保护)"},
		{"code": "iso27001", "name": "ISO 27001 (信息安全管理体系)"},
		{"code": "hipaa", "name": "HIPAA (美国健康保险携带和责任法案)"},
		{"code": "pci", "name": "PCI DSS (支付卡行业数据安全标准)"},
		{"code": "sox", "name": "SOX (萨班斯-奥克斯利法案)"},
	}
	c.JSON(http.StatusOK, SuccessResponse(standards))
}

// ========== 配置管理 ==========

// getConfig 获取审计配置
func (h *Handlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, SuccessResponse(config))
}

// updateConfig 更新审计配置
func (h *Handlers) updateConfig(c *gin.Context) {
	var config Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	h.manager.SetConfig(config)

	// 记录配置变更
	userID := c.GetString("user_id")
	username := c.GetString("username")
	ip := c.ClientIP()

	_ = h.manager.LogConfigChange(userID, username, ip, "audit_config", "update", nil, config)

	c.JSON(http.StatusOK, SuccessResponse(nil))
}

// ========== 日志记录接口 ==========

// createLog 创建审计日志
func (h *Handlers) createLog(c *gin.Context) {
	var entry Entry
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	// 从上下文获取用户信息
	if entry.UserID == "" {
		entry.UserID = c.GetString("user_id")
	}
	if entry.Username == "" {
		entry.Username = c.GetString("username")
	}
	if entry.IP == "" {
		entry.IP = c.ClientIP()
	}

	if err := h.manager.Log(&entry); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse(gin.H{"id": entry.ID}))
}

// logAuthRequest 认证日志请求
type logAuthRequest struct {
	Event     string                 `json:"event" binding:"required"`
	UserID    string                 `json:"user_id"`
	Username  string                 `json:"username"`
	IP        string                 `json:"ip"`
	UserAgent string                 `json:"user_agent"`
	Status    Status                 `json:"status" binding:"required"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details"`
}

// logAuth 记录认证事件
func (h *Handlers) logAuth(c *gin.Context) {
	var req logAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	// 补充信息
	if req.UserID == "" {
		req.UserID = c.GetString("user_id")
	}
	if req.Username == "" {
		req.Username = c.GetString("username")
	}
	if req.IP == "" {
		req.IP = c.ClientIP()
	}

	err := h.manager.LogAuth(req.Event, req.UserID, req.Username, req.IP, req.UserAgent, req.Status, req.Message, req.Details)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse(nil))
}

// logAccessRequest 访问日志请求
type logAccessRequest struct {
	UserID   string                 `json:"user_id"`
	Username string                 `json:"username"`
	IP       string                 `json:"ip"`
	Resource string                 `json:"resource" binding:"required"`
	Action   string                 `json:"action" binding:"required"`
	Status   Status                 `json:"status" binding:"required"`
	Details  map[string]interface{} `json:"details"`
}

// logAccess 记录访问事件
func (h *Handlers) logAccess(c *gin.Context) {
	var req logAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	if req.UserID == "" {
		req.UserID = c.GetString("user_id")
	}
	if req.Username == "" {
		req.Username = c.GetString("username")
	}
	if req.IP == "" {
		req.IP = c.ClientIP()
	}

	err := h.manager.LogAccess(req.UserID, req.Username, req.IP, req.Resource, req.Action, req.Status, req.Details)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse(nil))
}

// logSecurityRequest 安全日志请求
type logSecurityRequest struct {
	Event    string                 `json:"event" binding:"required"`
	UserID   string                 `json:"user_id"`
	Username string                 `json:"username"`
	IP       string                 `json:"ip"`
	Level    Level                  `json:"level" binding:"required"`
	Message  string                 `json:"message" binding:"required"`
	Details  map[string]interface{} `json:"details"`
}

// logSecurity 记录安全事件
func (h *Handlers) logSecurity(c *gin.Context) {
	var req logSecurityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	if req.UserID == "" {
		req.UserID = c.GetString("user_id")
	}
	if req.Username == "" {
		req.Username = c.GetString("username")
	}
	if req.IP == "" {
		req.IP = c.ClientIP()
	}

	err := h.manager.LogSecurity(req.Event, req.UserID, req.Username, req.IP, req.Level, req.Message, req.Details)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse(nil))
}

// logDataOperationRequest 数据操作日志请求
type logDataOperationRequest struct {
	UserID   string                 `json:"user_id"`
	Username string                 `json:"username"`
	IP       string                 `json:"ip"`
	Resource string                 `json:"resource" binding:"required"`
	Action   string                 `json:"action" binding:"required"`
	Status   Status                 `json:"status" binding:"required"`
	Details  map[string]interface{} `json:"details"`
}

// logDataOperation 记录数据操作
func (h *Handlers) logDataOperation(c *gin.Context) {
	var req logDataOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	if req.UserID == "" {
		req.UserID = c.GetString("user_id")
	}
	if req.Username == "" {
		req.Username = c.GetString("username")
	}
	if req.IP == "" {
		req.IP = c.ClientIP()
	}

	err := h.manager.LogDataOperation(req.UserID, req.Username, req.IP, req.Resource, req.Action, req.Status, req.Details)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse(nil))
}

// logConfigChangeRequest 配置变更日志请求
type logConfigChangeRequest struct {
	UserID   string      `json:"user_id"`
	Username string      `json:"username"`
	IP       string      `json:"ip"`
	Resource string      `json:"resource" binding:"required"`
	Action   string      `json:"action" binding:"required"`
	OldValue interface{} `json:"old_value"`
	NewValue interface{} `json:"new_value"`
}

// logConfigChange 记录配置变更
func (h *Handlers) logConfigChange(c *gin.Context) {
	var req logConfigChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	if req.UserID == "" {
		req.UserID = c.GetString("user_id")
	}
	if req.Username == "" {
		req.Username = c.GetString("username")
	}
	if req.IP == "" {
		req.IP = c.ClientIP()
	}

	err := h.manager.LogConfigChange(req.UserID, req.Username, req.IP, req.Resource, req.Action, req.OldValue, req.NewValue)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse(nil))
}
