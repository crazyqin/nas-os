// Package compliance 提供合规中心 REST API 处理器
package compliance

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 合规中心 API 处理器
// 对标 TrueNAS 25.10 的安全合规和审计能力
type Handlers struct {
	manager          *Manager
	fipsChecker      *FIPSComplianceChecker
	auditLogger      *AuditLogger
	complianceScanner *ComplianceScanner
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// NewEnhancedHandlers 创建增强版处理器（包含 FIPS、审计、合规扫描）
func NewEnhancedHandlers(manager *Manager, auditLogDir string) (*Handlers, error) {
	fipsChecker := NewFIPSComplianceChecker(FIPSLevel1)
	auditLogger, err := NewAuditLogger(auditLogDir, 100000, 90)
	if err != nil {
		return nil, err
	}
	complianceScanner := NewComplianceScanner()

	return &Handlers{
		manager:           manager,
		fipsChecker:       fipsChecker,
		auditLogger:       auditLogger,
		complianceScanner: complianceScanner,
	}, nil
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	compliance := r.Group("/compliance")
	{
		// 原有路由
		// 规则管理
		compliance.GET("/rules", h.listRules)
		compliance.POST("/rules", h.addRule)
		compliance.GET("/rules/:id", h.getRule)
		compliance.PUT("/rules/:id", h.updateRule)
		compliance.DELETE("/rules/:id", h.deleteRule)

		// 扫描
		compliance.POST("/scan", h.runScan)
		compliance.GET("/scan/:id", h.getScanResult)

		// 报告
		compliance.POST("/report", h.generateReport)
		compliance.GET("/report", h.listReports)
		compliance.GET("/report/:id", h.getReport)

		// 数据分类
		compliance.POST("/classify", h.classifyData)
		compliance.GET("/classify", h.listClassifications)
		compliance.GET("/classify/:id", h.getClassification)
		compliance.GET("/categories", h.getCategories)

		// 整改计划
		compliance.POST("/plan", h.createPlan)
		compliance.GET("/plan", h.listPlans)
		compliance.GET("/plan/:id", h.getPlan)

		// 法规列表
		compliance.GET("/regulations", h.getRegulations)
	}
}

// RegisterEnhancedRoutes 注册增强版路由（对标 TrueNAS 25.10 FIPS 和审计能力）
func (h *Handlers) RegisterEnhancedRoutes(r *gin.RouterGroup) {
	compliance := r.Group("/compliance")
	{
		// 合规状态
		compliance.GET("/status", h.getComplianceStatus)

		// FIPS 合规
		compliance.GET("/fips/status", h.getFIPSStatus)
		compliance.POST("/fips/self-test", h.runFIPSSelfTest)
		compliance.GET("/fips/algorithms", h.getFIPSAlgorithms)
		compliance.POST("/fips/generate-key", h.generateFIPSKey)
		compliance.POST("/fips/rotate-key", h.rotateFIPSKey)
		compliance.GET("/fips/report", h.getFIPSReport)

		// 安全审计
		compliance.GET("/audit-log", h.getAuditLog)
		compliance.POST("/audit-log", h.logAuditEvent)
		compliance.GET("/audit-log/export", h.exportAuditLogs)
		compliance.GET("/audit/anomalies", h.detectAnomalies)
		compliance.GET("/audit/profiles", h.getUserProfiles)
		compliance.GET("/audit/profiles/:userId", h.getUserProfile)
		compliance.GET("/audit/report", h.getAuditReport)

		// 合规扫描
		compliance.POST("/scan/cis", h.runCISCheck)
		compliance.POST("/scan/stig", h.runSTIGCheck)
		compliance.POST("/scan/gdpr", h.runGDPRCheck)
		compliance.POST("/scan/full", h.runFullComplianceScan)
		compliance.GET("/checks", h.getComplianceChecks)
		compliance.GET("/standards/cis", h.getCISBenchmark)
		compliance.GET("/standards/stig", h.getSTIGChecks)
		compliance.GET("/standards/gdpr", h.getGDPRArticles)

		// 合规报告列表
		compliance.GET("/reports", h.listEnhancedReports)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// listRules 获取规则列表
func (h *Handlers) listRules(c *gin.Context) {
	regulation := c.Query("regulation")
	rules := h.manager.ListRules(regulation)
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    rules,
	})
}

// addRule 添加规则
func (h *Handlers) addRule(c *gin.Context) {
	var rule ComplianceRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	if err := h.manager.AddRule(&rule); err != nil {
		c.JSON(http.StatusConflict, response{
			Code:    http.StatusConflict,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    http.StatusCreated,
		Message: "Rule added successfully",
		Data:    rule,
	})
}

// getRule 获取规则
func (h *Handlers) getRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.manager.GetRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    rule,
	})
}

// updateRule 更新规则
func (h *Handlers) updateRule(c *gin.Context) {
	id := c.Param("id")
	var rule ComplianceRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	rule.ID = id
	if err := h.manager.UpdateRule(&rule); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "Rule updated successfully",
		Data:    rule,
	})
}

// deleteRule 删除规则
func (h *Handlers) deleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteRule(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "Rule deleted successfully",
	})
}

// runScan 执行扫描
func (h *Handlers) runScan(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	report, err := h.manager.RunScan(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "Scan completed",
		Data:    report,
	})
}

// getScanResult 获取扫描结果
func (h *Handlers) getScanResult(c *gin.Context) {
	id := c.Param("id")
	result, ok := h.manager.scanResults[id]
	if !ok {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: "Scan result not found",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    result,
	})
}

// generateReport 生成报告
func (h *Handlers) generateReport(c *gin.Context) {
	var req struct {
		Regulation string       `json:"regulation"`
		Period     ReportPeriod `json:"period"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	if req.Period.Start.IsZero() {
		req.Period.Start = time.Now().AddDate(0, -1, 0)
	}
	if req.Period.End.IsZero() {
		req.Period.End = time.Now()
	}

	report, err := h.manager.GenerateReport(req.Regulation, req.Period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "Report generated",
		Data:    report,
	})
}

// listReports 获取报告列表
func (h *Handlers) listReports(c *gin.Context) {
	regulation := c.Query("regulation")
	reports := h.manager.ListReports(regulation)
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    reports,
	})
}

// getReport 获取报告
func (h *Handlers) getReport(c *gin.Context) {
	id := c.Param("id")
	report, err := h.manager.GetReport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    report,
	})
}

// classifyData 数据分类
func (h *Handlers) classifyData(c *gin.Context) {
	var req ScanDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	result, err := h.manager.ClassifyData(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "Classification completed",
		Data:    result,
	})
}

// listClassifications 获取分类列表
func (h *Handlers) listClassifications(c *gin.Context) {
	category := c.Query("category")
	classifications := h.manager.ListClassifications(category)
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    classifications,
	})
}

// getClassification 获取分类
func (h *Handlers) getClassification(c *gin.Context) {
	id := c.Param("id")
	cls, err := h.manager.GetClassification(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    cls,
	})
}

// getCategories 获取数据类别
func (h *Handlers) getCategories(c *gin.Context) {
	categories := h.manager.GetCategories()
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    categories,
	})
}

// createPlan 创建整改计划
func (h *Handlers) createPlan(c *gin.Context) {
	reportID := c.Query("report_id")
	if reportID == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "report_id is required",
		})
		return
	}

	var plan RemediationPlan
	if err := c.ShouldBindJSON(&plan); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	if err := h.manager.CreatePlan(reportID, &plan); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    http.StatusCreated,
		Message: "Plan created successfully",
		Data:    plan,
	})
}

// listPlans 获取计划列表
func (h *Handlers) listPlans(c *gin.Context) {
	reportID := c.Query("report_id")
	plans := h.manager.ListPlans(reportID)
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    plans,
	})
}

// getPlan 获取计划
func (h *Handlers) getPlan(c *gin.Context) {
	id := c.Param("id")
	plan, err := h.manager.GetPlan(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    plan,
	})
}

// getRegulations 获取法规列表
func (h *Handlers) getRegulations(c *gin.Context) {
	regulations := h.manager.GetRegulations()
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    regulations,
	})
}

// ==================== 增强版 API ====================

// getComplianceStatus 获取合规状态总览
// GET /api/v1/compliance/status
func (h *Handlers) getComplianceStatus(c *gin.Context) {
	if h.fipsChecker == nil || h.complianceScanner == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "Enhanced compliance module not initialized",
		})
		return
	}

	// FIPS 状态
	fipsStatus := h.fipsChecker.CheckStatus()

	// 快速合规检查
	fullReport := h.complianceScanner.RunFullComplianceScan()

	status := ComplianceStatusResponse{
		FIPSEnabled:     h.fipsChecker.IsFIPSEnabled(),
		FIPSCompliant:   fipsStatus.Compliant,
		FIPSLevel:       string(fipsStatus.Level),
		CISCompliant:    fullReport.CISReport.Summary.Compliant,
		CISTotal:        fullReport.CISReport.Summary.TotalChecks,
		STIGCompliant:   fullReport.STIGReport.Summary.Compliant,
		STIGTotal:       fullReport.STIGReport.Summary.TotalChecks,
		GDPRCompliant:   fullReport.GDPRReport.Summary.Compliant,
		GDPRTotal:       fullReport.GDPRReport.Summary.TotalChecks,
		OverallScore:    fullReport.OverallScore,
		SelfTestOK:      fipsStatus.SelfTestOK,
		ActiveKeys:      fipsStatus.KeyManagement.ActiveKeys,
		ExpiredKeys:     fipsStatus.KeyManagement.ExpiredKeys,
		IssuesCount:     len(fipsStatus.Issues),
		LastCheck:       fipsStatus.LastCheck,
		NextCheck:       fipsStatus.NextCheck,
		CheckedAt:       time.Now(),
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    status,
	})
}

// ComplianceStatusResponse 合规状态响应
type ComplianceStatusResponse struct {
	FIPSEnabled   bool      `json:"fips_enabled"`
	FIPSCompliant bool      `json:"fips_compliant"`
	FIPSLevel     string    `json:"fips_level"`
	CISCompliant  int       `json:"cis_compliant"`
	CISTotal      int       `json:"cis_total"`
	STIGCompliant int       `json:"stig_compliant"`
	STIGTotal     int       `json:"stig_total"`
	GDPRCompliant int       `json:"gdpr_compliant"`
	GDPRTotal     int       `json:"gdpr_total"`
	OverallScore  float64   `json:"overall_score"`
	SelfTestOK    bool      `json:"self_test_ok"`
	ActiveKeys    int       `json:"active_keys"`
	ExpiredKeys   int       `json:"expired_keys"`
	IssuesCount   int       `json:"issues_count"`
	LastCheck     time.Time `json:"last_check"`
	NextCheck     time.Time `json:"next_check"`
	CheckedAt     time.Time `json:"checked_at"`
}

// getFIPSStatus 获取 FIPS 合规状态
// GET /api/v1/compliance/fips/status
func (h *Handlers) getFIPSStatus(c *gin.Context) {
	if h.fipsChecker == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "FIPS checker not initialized",
		})
		return
	}

	status := h.fipsChecker.CheckStatus()
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    status,
	})
}

// runFIPSSelfTest 运行 FIPS 自检
// POST /api/v1/compliance/fips/self-test
func (h *Handlers) runFIPSSelfTest(c *gin.Context) {
	if h.fipsChecker == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "FIPS checker not initialized",
		})
		return
	}

	status := h.fipsChecker.CheckStatus()
	result := map[string]interface{}{
		"self_test_ok": status.SelfTestOK,
		"checked_at":   status.CheckedAt,
		"issues":       status.Issues,
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "Self-test completed",
		Data:    result,
	})
}

// getFIPSAlgorithms 获取 FIPS 批准的算法列表
// GET /api/v1/compliance/fips/algorithms
func (h *Handlers) getFIPSAlgorithms(c *gin.Context) {
	if h.fipsChecker == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "FIPS checker not initialized",
		})
		return
	}

	algorithms := h.fipsChecker.GetApprovedAlgorithms()
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    algorithms,
	})
}

// generateFIPSKey 生成 FIPS 合规密钥
// POST /api/v1/compliance/fips/generate-key
func (h *Handlers) generateFIPSKey(c *gin.Context) {
	if h.fipsChecker == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "FIPS checker not initialized",
		})
		return
	}

	var req struct {
		Algorithm string `json:"algorithm" binding:"required"`
		KeySize   int    `json:"key_size" binding:"required"`
		Usage     string `json:"usage" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	key, err := h.fipsChecker.GenerateFIPSKey(req.Algorithm, req.KeySize, req.Usage)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    http.StatusCreated,
		Message: "FIPS key generated",
		Data:    key,
	})
}

// rotateFIPSKey 轮换 FIPS 密钥
// POST /api/v1/compliance/fips/rotate-key
func (h *Handlers) rotateFIPSKey(c *gin.Context) {
	if h.fipsChecker == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "FIPS checker not initialized",
		})
		return
	}

	var req struct {
		KeyID string `json:"key_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	newKey, err := h.fipsChecker.RotateFIPSKey(req.KeyID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "FIPS key rotated",
		Data:    newKey,
	})
}

// getFIPSReport 获取 FIPS 合规报告
// GET /api/v1/compliance/fips/report
func (h *Handlers) getFIPSReport(c *gin.Context) {
	if h.fipsChecker == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "FIPS checker not initialized",
		})
		return
	}

	report := h.fipsChecker.GenerateFIPSReport()
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    report,
	})
}

// getAuditLog 获取审计日志
// GET /api/v1/compliance/audit-log
func (h *Handlers) getAuditLog(c *gin.Context) {
	if h.auditLogger == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "Audit logger not initialized",
		})
		return
	}

	query := &AuditQuery{
		Limit: 50,
	}

	// 解析查询参数
	if actorID := c.Query("actor_id"); actorID != "" {
		query.ActorID = actorID
	}
	if eventType := c.Query("event_type"); eventType != "" {
		query.EventTypes = []AuditEventType{AuditEventType(eventType)}
	}
	if severity := c.Query("severity"); severity != "" {
		query.Severities = []AuditSeverity{AuditSeverity(severity)}
	}
	if resourceType := c.Query("resource_type"); resourceType != "" {
		query.ResourceType = resourceType
	}
	if status := c.Query("status"); status != "" {
		query.Status = status
	}
	if ip := c.Query("ip_address"); ip != "" {
		query.IPAddress = ip
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && limit > 0 {
			query.Limit = limit
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		var offset int
		if _, err := fmt.Sscanf(offsetStr, "%d", &offset); err == nil && offset >= 0 {
			query.Offset = offset
		}
	}

	result, err := h.auditLogger.QueryEvents(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    result,
	})
}

// logAuditEvent 记录审计事件
// POST /api/v1/compliance/audit-log
func (h *Handlers) logAuditEvent(c *gin.Context) {
	if h.auditLogger == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "Audit logger not initialized",
		})
		return
	}

	var event AuditEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	if err := h.auditLogger.LogEvent(&event); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    http.StatusCreated,
		Message: "Audit event logged",
		Data:    event,
	})
}

// exportAuditLogs 导出审计日志
// GET /api/v1/compliance/audit-log/export
func (h *Handlers) exportAuditLogs(c *gin.Context) {
	if h.auditLogger == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "Audit logger not initialized",
		})
		return
	}

	format := c.DefaultQuery("format", "jsonl")

	data, err := h.auditLogger.ExportAuditLogs(&AuditQuery{}, format)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.Data(http.StatusOK, "application/octet-stream", data)
}

// detectAnomalies 检测异常行为
// GET /api/v1/compliance/audit/anomalies
func (h *Handlers) detectAnomalies(c *gin.Context) {
	if h.auditLogger == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "Audit logger not initialized",
		})
		return
	}

	anomalies := h.auditLogger.DetectAnomalies()
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    anomalies,
	})
}

// getUserProfiles 获取用户行为画像列表
// GET /api/v1/compliance/audit/profiles
func (h *Handlers) getUserProfiles(c *gin.Context) {
	if h.auditLogger == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "Audit logger not initialized",
		})
		return
	}

	profiles := h.auditLogger.ListUserProfiles()
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    profiles,
	})
}

// getUserProfile 获取指定用户行为画像
// GET /api/v1/compliance/audit/profiles/:userId
func (h *Handlers) getUserProfile(c *gin.Context) {
	if h.auditLogger == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "Audit logger not initialized",
		})
		return
	}

	userID := c.Param("userId")
	profile, err := h.auditLogger.GetUserProfile(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    profile,
	})
}

// getAuditReport 获取审计报告
// GET /api/v1/compliance/audit/report
func (h *Handlers) getAuditReport(c *gin.Context) {
	if h.auditLogger == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "Audit logger not initialized",
		})
		return
	}

	period := ReportPeriod{
		Start: time.Now().AddDate(0, -1, 0), // 默认最近一个月
		End:   time.Now(),
	}

	report := h.auditLogger.GenerateAuditReport(period)
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    report,
	})
}

// runCISCheck 运行 CIS 基准检查
// POST /api/v1/compliance/scan/cis
func (h *Handlers) runCISCheck(c *gin.Context) {
	if h.complianceScanner == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "Compliance scanner not initialized",
		})
		return
	}

	report := h.complianceScanner.RunCISCheck()
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "CIS check completed",
		Data:    report,
	})
}

// runSTIGCheck 运行 STIG 合规检查
// POST /api/v1/compliance/scan/stig
func (h *Handlers) runSTIGCheck(c *gin.Context) {
	if h.complianceScanner == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "Compliance scanner not initialized",
		})
		return
	}

	report := h.complianceScanner.RunSTIGCheck()
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "STIG check completed",
		Data:    report,
	})
}

// runGDPRCheck 运行 GDPR 合规检查
// POST /api/v1/compliance/scan/gdpr
func (h *Handlers) runGDPRCheck(c *gin.Context) {
	if h.complianceScanner == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "Compliance scanner not initialized",
		})
		return
	}

	report := h.complianceScanner.RunGDPRCheck()
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "GDPR check completed",
		Data:    report,
	})
}

// runFullComplianceScan 运行完整合规扫描
// POST /api/v1/compliance/scan/full
func (h *Handlers) runFullComplianceScan(c *gin.Context) {
	if h.complianceScanner == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "Compliance scanner not initialized",
		})
		return
	}

	report := h.complianceScanner.RunFullComplianceScan()
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "Full compliance scan completed",
		Data:    report,
	})
}

// getComplianceChecks 获取所有合规检查项
// GET /api/v1/compliance/checks
func (h *Handlers) getComplianceChecks(c *gin.Context) {
	if h.complianceScanner == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "Compliance scanner not initialized",
		})
		return
	}

	checks := h.complianceScanner.GetComplianceChecks()
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    checks,
	})
}

// getCISBenchmark 获取 CIS 基准配置
// GET /api/v1/compliance/standards/cis
func (h *Handlers) getCISBenchmark(c *gin.Context) {
	if h.complianceScanner == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "Compliance scanner not initialized",
		})
		return
	}

	benchmark := h.complianceScanner.GetCISBenchmark()
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    benchmark,
	})
}

// getSTIGChecks 获取 STIG 检查配置
// GET /api/v1/compliance/standards/stig
func (h *Handlers) getSTIGChecks(c *gin.Context) {
	if h.complianceScanner == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "Compliance scanner not initialized",
		})
		return
	}

	checks := h.complianceScanner.GetSTIGChecks()
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    checks,
	})
}

// getGDPRArticles 获取 GDPR 条款
// GET /api/v1/compliance/standards/gdpr
func (h *Handlers) getGDPRArticles(c *gin.Context) {
	if h.complianceScanner == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    http.StatusServiceUnavailable,
			Message: "Compliance scanner not initialized",
		})
		return
	}

	articles := h.complianceScanner.GetGDPRArticles()
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    articles,
	})
}

// listEnhancedReports 列出增强版报告
// GET /api/v1/compliance/reports
func (h *Handlers) listEnhancedReports(c *gin.Context) {
	// 返回已有报告列表
	reports := h.manager.ListReports("")
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    reports,
	})
}
