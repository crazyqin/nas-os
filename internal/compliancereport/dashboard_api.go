// Package compliancereport 合规仪表盘 API
// 提供合规总览、基线检查、自动审计的 API 接口
package compliancereport

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ComplianceDashboardHandlers 合规仪表盘 API 处理器.
type ComplianceDashboardHandlers struct {
	reportGen       *ReportGenerator
	baselineScanner *SecurityBaselineScanner
	auditEngine     *AutoAuditEngine
	dashboardSvc    *DashboardService
}

// NewComplianceDashboardHandlers 创建合规仪表盘处理器.
func NewComplianceDashboardHandlers(
	reportGen *ReportGenerator,
	auditEngine *AutoAuditEngine,
	dashboardSvc *DashboardService,
) *ComplianceDashboardHandlers {
	return &ComplianceDashboardHandlers{
		reportGen:       reportGen,
		baselineScanner: NewSecurityBaselineScanner(),
		auditEngine:     auditEngine,
		dashboardSvc:    dashboardSvc,
	}
}

// RegisterRoutes 注册合规仪表盘路由.
func (h *ComplianceDashboardHandlers) RegisterRoutes(api *gin.RouterGroup) {
	compliance := api.Group("/compliance")
	{
		// 安全基线检查
		compliance.GET("/baseline", h.getBaselineCheck)
		compliance.POST("/baseline/scan", h.triggerBaselineScan)

		// 自动化审计
		compliance.POST("/audit", h.triggerAudit)
		compliance.GET("/audit/status", h.getAuditStatus)
		compliance.GET("/audit/results", h.getAuditResults)
		compliance.GET("/audit/results/:id", h.getAuditResultByID)
		compliance.GET("/audit/alerts", h.getAuditAlerts)
		compliance.PUT("/audit/config", h.updateAuditConfig)
		compliance.GET("/audit/config", h.getAuditConfig)

		// 合规仪表盘
		compliance.GET("/dashboard", h.getDashboard)
		compliance.GET("/dashboard/overview", h.getDashboardOverview)
		compliance.GET("/dashboard/trends", h.getDashboardTrends)
	}
}

// BaselineScanRequest 基线扫描请求.
type BaselineScanRequest struct {
	Standard   SecurityBaselineStandard `json:"standard" binding:"required"`
	Categories []BaselineCategory       `json:"categories,omitempty"`
}

// GET /api/v1/compliance/baseline
func (h *ComplianceDashboardHandlers) getBaselineCheck(c *gin.Context) {
	standard := c.DefaultQuery("standard", "cis")
	category := c.Query("category")

	var categories []BaselineCategory
	if category != "" {
		categories = []BaselineCategory{BaselineCategory(category)}
	}

	report := h.baselineScanner.GenerateBaselineReport(
		c.Request.Context(),
		SecurityBaselineStandard(standard),
		categories,
	)

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "success",
		Data:    report,
	})
}

// POST /api/v1/compliance/baseline/scan
func (h *ComplianceDashboardHandlers) triggerBaselineScan(c *gin.Context) {
	var req BaselineScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	report := h.baselineScanner.GenerateBaselineReport(
		c.Request.Context(),
		req.Standard,
		req.Categories,
	)

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "基线扫描完成",
		Data:    report,
	})
}

// POST /api/v1/compliance/audit
func (h *ComplianceDashboardHandlers) triggerAudit(c *gin.Context) {
	if h.auditEngine == nil {
		c.JSON(http.StatusServiceUnavailable, APIResponse{
			Code:    503,
			Message: "自动审计引擎未初始化",
		})
		return
	}

	result := h.auditEngine.ExecuteAudit()

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "审计完成",
		Data:    result,
	})
}

// GET /api/v1/compliance/audit/status
func (h *ComplianceDashboardHandlers) getAuditStatus(c *gin.Context) {
	if h.auditEngine == nil {
		c.JSON(http.StatusOK, APIResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"enabled": false,
				"running": false,
			},
		})
		return
	}

	status := h.auditEngine.GetAuditStatus()

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "success",
		Data:    status,
	})
}

// GET /api/v1/compliance/audit/results
func (h *ComplianceDashboardHandlers) getAuditResults(c *gin.Context) {
	if h.auditEngine == nil {
		c.JSON(http.StatusOK, APIResponse{
			Code:    0,
			Message: "success",
			Data:    []*AuditResult{},
		})
		return
	}

	results := h.auditEngine.GetResults()

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "success",
		Data:    results,
	})
}

// GET /api/v1/compliance/audit/results/:id
func (h *ComplianceDashboardHandlers) getAuditResultByID(c *gin.Context) {
	if h.auditEngine == nil {
		c.JSON(http.StatusServiceUnavailable, APIResponse{
			Code:    503,
			Message: "自动审计引擎未初始化",
		})
		return
	}

	id := c.Param("id")
	result, ok := h.auditEngine.GetResultByID(id)
	if !ok {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    404,
			Message: "审计结果不存在",
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// GET /api/v1/compliance/audit/alerts
func (h *ComplianceDashboardHandlers) getAuditAlerts(c *gin.Context) {
	if h.auditEngine == nil {
		c.JSON(http.StatusOK, APIResponse{
			Code:    0,
			Message: "success",
			Data:    []AuditAlert{},
		})
		return
	}

	alerts := h.auditEngine.GetAlerts()

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "success",
		Data:    alerts,
	})
}

// PUT /api/v1/compliance/audit/config
func (h *ComplianceDashboardHandlers) updateAuditConfig(c *gin.Context) {
	if h.auditEngine == nil {
		c.JSON(http.StatusServiceUnavailable, APIResponse{
			Code:    503,
			Message: "自动审计引擎未初始化",
		})
		return
	}

	var config AuditConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	h.auditEngine.SetConfig(config)

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "审计配置已更新",
		Data:    config,
	})
}

// GET /api/v1/compliance/audit/config
func (h *ComplianceDashboardHandlers) getAuditConfig(c *gin.Context) {
	if h.auditEngine == nil {
		c.JSON(http.StatusOK, APIResponse{
			Code:    0,
			Message: "success",
			Data:    DefaultAuditConfig(),
		})
		return
	}

	config := h.auditEngine.GetConfig()

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "success",
		Data:    config,
	})
}

// ComplianceDashboard 合规仪表盘数据.
type ComplianceDashboard struct {
	OverallScore        int                  `json:"overall_score"`
	OverallStatus       string               `json:"overall_status"`
	ComplianceReports   int                  `json:"compliance_reports"`
	BaselineScore       int                  `json:"baseline_score"`
	BaselineStatus      string               `json:"baseline_status"`
	TotalViolations     int                  `json:"total_violations"`
	CriticalViolations  int                  `json:"critical_violations"`
	PendingRemediations int                  `json:"pending_remediations"`
	LastAuditTime       *time.Time           `json:"last_audit_time,omitempty"`
	LastAuditScore      int                  `json:"last_audit_score"`
	StandardsStatus     []StandardStatusItem `json:"standards_status"`
	BaselineStatusItems []BaselineStatusItem `json:"baseline_status_items"`
	RecentAlerts        []AuditAlert         `json:"recent_alerts"`
	ComplianceTrend     []TrendDataPoint     `json:"compliance_trend"`
}

// StandardStatusItem 标准状态项.
type StandardStatusItem struct {
	Standard string `json:"standard"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Score    int    `json:"score"`
}

// BaselineStatusItem 基线状态项.
type BaselineStatusItem struct {
	Standard string `json:"standard"`
	Name     string `json:"name"`
	Score    int    `json:"score"`
	Checks   int    `json:"checks"`
	Passed   int    `json:"passed"`
	Failed   int    `json:"failed"`
}

// GET /api/v1/compliance/dashboard
func (h *ComplianceDashboardHandlers) getDashboard(c *gin.Context) {
	dashboard := h.buildDashboard()

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "success",
		Data:    dashboard,
	})
}

// GET /api/v1/compliance/dashboard/overview
func (h *ComplianceDashboardHandlers) getDashboardOverview(c *gin.Context) {
	dashboard := h.buildDashboard()

	overview := map[string]interface{}{
		"overall_score":       dashboard.OverallScore,
		"overall_status":      dashboard.OverallStatus,
		"total_violations":    dashboard.TotalViolations,
		"critical_violations": dashboard.CriticalViolations,
		"last_audit_time":     dashboard.LastAuditTime,
		"last_audit_score":    dashboard.LastAuditScore,
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "success",
		Data:    overview,
	})
}

// GET /api/v1/compliance/dashboard/trends
func (h *ComplianceDashboardHandlers) getDashboardTrends(c *gin.Context) {
	dashboard := h.buildDashboard()

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "success",
		Data:    dashboard.ComplianceTrend,
	})
}

// buildDashboard 构建仪表盘数据.
func (h *ComplianceDashboardHandlers) buildDashboard() *ComplianceDashboard {
	dashboard := &ComplianceDashboard{
		StandardsStatus:     make([]StandardStatusItem, 0),
		BaselineStatusItems: make([]BaselineStatusItem, 0),
		RecentAlerts:        make([]AuditAlert, 0),
		ComplianceTrend:     make([]TrendDataPoint, 0),
	}

	// 获取合规报告状态
	status := h.reportGen.GetStatus()
	dashboard.OverallScore = status.OverallScore
	dashboard.OverallStatus = string(status.OverallStatus)
	dashboard.ComplianceReports = status.TotalReports
	dashboard.PendingRemediations = status.PendingRemediation

	// 各标准状态
	standardsManager := NewStandardsManager()
	for _, std := range status.Standards {
		stdInfo, _ := standardsManager.GetStandard(std.Standard)
		dashboard.StandardsStatus = append(dashboard.StandardsStatus, StandardStatusItem{
			Standard: string(std.Standard),
			Name:     stdInfo.Name,
			Status:   string(std.Status),
			Score:    std.Score,
		})
	}

	// 基线检查状态
	baselineStandards := []SecurityBaselineStandard{BaselineCIS, BaselineNIST}
	totalBaselineScore := 0
	baselineCount := 0

	for _, baselineStd := range baselineStandards {
		report := h.baselineScanner.GenerateBaselineReport(nil, baselineStd, nil)
		stdName := string(baselineStd)
		switch baselineStd {
		case BaselineCIS:
			stdName = "CIS Benchmark"
		case BaselineNIST:
			stdName = "NIST SP 800-53"
		case BaselineSTIG:
			stdName = "DISA STIG"
		}

		dashboard.BaselineStatusItems = append(dashboard.BaselineStatusItems, BaselineStatusItem{
			Standard: string(baselineStd),
			Name:     stdName,
			Score:    report.Score,
			Checks:   report.TotalChecks,
			Passed:   report.Passed,
			Failed:   report.Failed,
		})

		totalBaselineScore += report.Score
		baselineCount++
	}

	if baselineCount > 0 {
		dashboard.BaselineScore = totalBaselineScore / baselineCount
	}
	dashboard.BaselineStatus = determineComplianceStatusByScore(dashboard.BaselineScore)

	// 审计引擎状态
	if h.auditEngine != nil {
		latest := h.auditEngine.GetLatestResult()
		if latest != nil {
			dashboard.LastAuditTime = &latest.StartTime
			dashboard.LastAuditScore = latest.OverallScore
			dashboard.TotalViolations = latest.TotalViolations
			dashboard.CriticalViolations = latest.CriticalViolations

			// 最近告警（最多 10 条）
			alerts := latest.Alerts
			if len(alerts) > 10 {
				alerts = alerts[len(alerts)-10:]
			}
			dashboard.RecentAlerts = alerts
		}
	}

	// 趋势数据
	dashboard.ComplianceTrend = generateDashboardTrend(dashboard.OverallScore)

	return dashboard
}

// determineComplianceStatusByScore 根据分数确定合规状态.
func determineComplianceStatusByScore(score int) string {
	switch {
	case score >= 90:
		return "compliant"
	case score >= 60:
		return "pending_review"
	default:
		return "non_compliant"
	}
}

// generateDashboardTrend 生成仪表盘趋势数据.
func generateDashboardTrend(currentScore int) []TrendDataPoint {
	trend := make([]TrendDataPoint, 7)
	now := time.Now()

	for i := 6; i >= 0; i-- {
		date := now.AddDate(0, 0, -(6 - i))
		score := currentScore
		if i > 0 {
			score = currentScore - (6-i)*2
			if score < 0 {
				score = 0
			}
			if score > 100 {
				score = 100
			}
		}

		trend[6-i] = TrendDataPoint{
			Date:   date.Format("2006-01-02"),
			Score:  score,
			Status: determineComplianceStatusByScore(score),
		}
	}

	return trend
}
