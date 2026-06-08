package securityaudit

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 安全审计 HTTP 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建安全审计处理器.
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{manager: mgr}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	audit := api.Group("/securityaudit")
	{
		// 仪表板
		audit.GET("/dashboard", h.getDashboard)

		// 配置
		audit.GET("/config", h.getConfig)
		audit.PUT("/config", h.updateConfig)

		// 安全检查
		checks := audit.Group("/checks")
		{
			checks.GET("", h.getCheckList)
			checks.POST("/run", h.runChecks)
			checks.POST("/run/:category", h.runChecksByCategory)
			checks.GET("/results", h.getCheckResults)
		}

		// 安全评分
		score := audit.Group("/score")
		{
			score.GET("", h.getScore)
			score.GET("/history", h.getScoreHistory)
			score.GET("/breakdown", h.getScoreBreakdown)
		}

		// 漏洞扫描
		vulns := audit.Group("/vulnerabilities")
		{
			vulns.POST("/scan", h.runVulnScan)
			vulns.GET("", h.getVulnerabilities)
			vulns.GET("/:id", h.getVulnerability)
			vulns.PUT("/:id/status", h.updateVulnStatus)
			vulns.POST("/:id/fix", h.fixVulnerability)
			vulns.GET("/report/latest", h.getLatestScanReport)
		}

		// 加固建议
		hardening := audit.Group("/hardening")
		{
			hardening.GET("/suggestions", h.getHardeningSuggestions)
			hardening.GET("/suggestions/:category", h.getHardeningSuggestionsByCategory)
			hardening.GET("/report", h.getHardeningReport)
			hardening.POST("/:id/apply", h.applyHardening)
			hardening.POST("/:id/dismiss", h.dismissHardening)
		}

		// 审计日志
		logs := audit.Group("/logs")
		{
			logs.GET("", h.getAuditLogs)
			logs.GET("/report", h.getAuditReport)
			logs.GET("/export", h.exportAuditLogs)
			logs.GET("/stats", h.getAuditStats)
		}

		// 完整审计
		audit.POST("/full-audit", h.runFullAudit)
	}
}

// ========== 通用响应 ==========

type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func success(data interface{}) apiResponse {
	return apiResponse{Code: 0, Message: "success", Data: data}
}

func apiError(code int, message string) apiResponse {
	return apiResponse{Code: code, Message: message}
}

// ========== 仪表板 ==========

func (h *Handlers) getDashboard(c *gin.Context) {
	dashboard := h.manager.GetDashboard()
	c.JSON(http.StatusOK, success(dashboard))
}

// ========== 配置 ==========

func (h *Handlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, success(config))
}

func (h *Handlers) updateConfig(c *gin.Context) {
	var config SecurityAuditConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	if err := h.manager.UpdateConfig(config); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

// ========== 安全检查 ==========

func (h *Handlers) getCheckList(c *gin.Context) {
	checks := h.manager.GetSecurityCheckList()
	c.JSON(http.StatusOK, success(checks))
}

func (h *Handlers) runChecks(c *gin.Context) {
	results := h.manager.RunSecurityChecks()
	c.JSON(http.StatusOK, success(results))
}

func (h *Handlers) runChecksByCategory(c *gin.Context) {
	category := SecurityCheckCategory(c.Param("category"))
	results := h.manager.RunSecurityChecksByCategory(category)
	c.JSON(http.StatusOK, success(results))
}

func (h *Handlers) getCheckResults(c *gin.Context) {
	// 返回最新的检查结果
	results := h.manager.RunSecurityChecks()
	c.JSON(http.StatusOK, success(results))
}

// ========== 安全评分 ==========

func (h *Handlers) getScore(c *gin.Context) {
	score := h.manager.GetSecurityScore()
	c.JSON(http.StatusOK, success(score))
}

func (h *Handlers) getScoreHistory(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	history := h.manager.GetScoreHistory(days)
	c.JSON(http.StatusOK, success(history))
}

func (h *Handlers) getScoreBreakdown(c *gin.Context) {
	score := h.manager.GetSecurityScore()
	engine := NewScoreEngine()
	breakdown := engine.GetCategoryBreakdown(score)
	c.JSON(http.StatusOK, success(breakdown))
}

// ========== 漏洞扫描 ==========

func (h *Handlers) runVulnScan(c *gin.Context) {
	report := h.manager.RunVulnerabilityScan()
	c.JSON(http.StatusOK, success(report))
}

func (h *Handlers) getVulnerabilities(c *gin.Context) {
	severity := VulnerabilitySeverity(c.Query("severity"))
	status := VulnerabilityStatus(c.Query("status"))
	vulns := h.manager.GetVulnerabilities(severity, status)
	c.JSON(http.StatusOK, success(vulns))
}

func (h *Handlers) getVulnerability(c *gin.Context) {
	id := c.Param("id")
	vuln, err := h.manager.GetVulnerability(id)
	if err != nil {
		c.JSON(http.StatusNotFound, apiError(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, success(vuln))
}

func (h *Handlers) updateVulnStatus(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Status VulnerabilityStatus `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	actor := c.GetString("username")
	if actor == "" {
		actor = "unknown"
	}

	if err := h.manager.UpdateVulnerabilityStatus(id, req.Status, actor); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

func (h *Handlers) fixVulnerability(c *gin.Context) {
	id := c.Param("id")
	actor := c.GetString("username")
	if actor == "" {
		actor = "unknown"
	}

	if err := h.manager.FixVulnerability(id, actor); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

func (h *Handlers) getLatestScanReport(c *gin.Context) {
	report := h.manager.GetLatestScanReport()
	if report == nil {
		c.JSON(http.StatusNotFound, apiError(404, "未找到扫描报告"))
		return
	}
	c.JSON(http.StatusOK, success(report))
}

// ========== 加固建议 ==========

func (h *Handlers) getHardeningSuggestions(c *gin.Context) {
	suggestions := h.manager.GetHardeningSuggestions()
	c.JSON(http.StatusOK, success(suggestions))
}

func (h *Handlers) getHardeningSuggestionsByCategory(c *gin.Context) {
	category := HardeningCategory(c.Param("category"))
	suggestions := h.manager.GetHardeningSuggestionsByCategory(category)
	c.JSON(http.StatusOK, success(suggestions))
}

func (h *Handlers) getHardeningReport(c *gin.Context) {
	report := h.manager.GetHardeningReport()
	c.JSON(http.StatusOK, success(report))
}

func (h *Handlers) applyHardening(c *gin.Context) {
	id := c.Param("id")
	actor := c.GetString("username")
	if actor == "" {
		actor = "unknown"
	}

	if err := h.manager.ApplyHardeningSuggestion(id, actor); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

func (h *Handlers) dismissHardening(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Reason string `json:"reason"`
	}
	c.ShouldBindJSON(&req)

	actor := c.GetString("username")
	if actor == "" {
		actor = "unknown"
	}

	if err := h.manager.DismissHardeningSuggestion(id, actor, req.Reason); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

// ========== 审计日志 ==========

func (h *Handlers) getAuditLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	filters := make(map[string]string)
	for _, key := range []string{"event_type", "severity", "actor", "status", "resource"} {
		if value := c.Query(key); value != "" {
			filters[key] = value
		}
	}

	logs := h.manager.GetAuditLogs(limit, offset, filters)
	c.JSON(http.StatusOK, success(logs))
}

func (h *Handlers) getAuditReport(c *gin.Context) {
	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")

	var startTime, endTime time.Time
	var err error

	if startTimeStr != "" {
		startTime, err = time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, apiError(400, "无效的开始时间"))
			return
		}
	} else {
		startTime = time.Now().Add(-24 * time.Hour)
	}

	if endTimeStr != "" {
		endTime, err = time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, apiError(400, "无效的结束时间"))
			return
		}
	} else {
		endTime = time.Now()
	}

	report := h.manager.GetAuditReport(startTime, endTime)
	c.JSON(http.StatusOK, success(report))
}

func (h *Handlers) exportAuditLogs(c *gin.Context) {
	format := c.DefaultQuery("format", "json")
	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")

	var startTime, endTime time.Time
	var err error

	if startTimeStr != "" {
		startTime, err = time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, apiError(400, "无效的开始时间"))
			return
		}
	} else {
		startTime = time.Now().Add(-24 * time.Hour)
	}

	if endTimeStr != "" {
		endTime, err = time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, apiError(400, "无效的结束时间"))
			return
		}
	} else {
		endTime = time.Now()
	}

	data, err := h.manager.ExportAuditLogs(startTime, endTime, format)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError(500, err.Error()))
		return
	}

	contentType := "application/json"
	if format == "csv" {
		contentType = "text/csv"
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "attachment; filename=security-audit-logs."+format)
	c.Data(http.StatusOK, contentType, data)
}

func (h *Handlers) getAuditStats(c *gin.Context) {
	logger := NewAuditLogger()
	stats := logger.GetStats()
	c.JSON(http.StatusOK, success(stats))
}

// ========== 完整审计 ==========

func (h *Handlers) runFullAudit(c *gin.Context) {
	result := h.manager.RunFullAudit()
	c.JSON(http.StatusOK, success(result))
}
