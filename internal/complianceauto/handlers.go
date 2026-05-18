// Package complianceauto 提供自动化合规检查功能
package complianceauto

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 合规自动化 API 处理器
type Handlers struct {
	scanner    *Scanner
	reporter   *Reporter
	remediator *Remediator
}

// NewHandlers 创建处理器
func NewHandlers(scanner *Scanner, reporter *Reporter, remediator *Remediator) *Handlers {
	return &Handlers{
		scanner:    scanner,
		reporter:   reporter,
		remediator: remediator,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	ca := r.Group("/compliance-auto")
	{
		// 扫描操作
		ca.POST("/scan", h.startScan)
		ca.POST("/scan/cancel", h.cancelScan)
		ca.GET("/scan/status", h.getScanStatus)
		ca.GET("/scan/last", h.getLastScan)

		// 规则管理
		ca.GET("/rules", h.getRules)
		ca.GET("/rules/:id", h.getRule)
		ca.GET("/rules/standard/:standard", h.getRulesByStandard)
		ca.GET("/rules/category/:category", h.getRulesByCategory)
		ca.PUT("/rules/:id/enable", h.enableRule)
		ca.PUT("/rules/:id/disable", h.disableRule)

		// 报告生成
		ca.POST("/reports/generate", h.generateReport)
		ca.GET("/reports/:id", h.getReport)
		ca.GET("/reports/:id/download", h.downloadReport)

		// 修复操作
		ca.GET("/remediations", h.getRemediations)
		ca.GET("/remediations/:id", h.getRemediation)
		ca.POST("/remediations/:id/execute", h.executeRemediation)
		ca.POST("/remediations/execute-all", h.executeAllRemediations)

		// 统计信息
		ca.GET("/stats", h.getStats)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ScanRequest 扫描请求
type ScanRequest struct {
	Standards []ComplianceStandard `json:"standards"` // 要扫描的标准，为空则扫描所有
}

// startScan 开始合规扫描
func (h *Handlers) startScan(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 使用默认配置扫描所有标准
		req.Standards = []ComplianceStandard{}
	}

	scan, err := h.scanner.Scan(c.Request.Context(), req.Standards)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "扫描完成",
		Data:    scan,
	})
}

// cancelScan 取消扫描
func (h *Handlers) cancelScan(c *gin.Context) {
	h.scanner.CancelScan()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "扫描已取消",
	})
}

// getScanStatus 获取扫描状态
func (h *Handlers) getScanStatus(c *gin.Context) {
	lastScan := h.scanner.GetLastScan()
	if lastScan == nil {
		c.JSON(http.StatusOK, response{
			Code:    0,
			Message: "暂无扫描记录",
			Data: map[string]string{
				"status": "idle",
			},
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"status":  lastScan.Status,
			"scanId":  lastScan.ID,
			"progress": map[string]int{
				"total":   lastScan.TotalRules,
				"passed":  lastScan.PassedRules,
				"failed":  lastScan.FailedRules,
				"warning": lastScan.WarnRules,
				"skip":    lastScan.SkipRules,
				"error":   lastScan.ErrorRules,
			},
		},
	})
}

// getLastScan 获取最近扫描结果
func (h *Handlers) getLastScan(c *gin.Context) {
	lastScan := h.scanner.GetLastScan()
	if lastScan == nil {
		c.JSON(http.StatusOK, response{
			Code:    0,
			Message: "暂无扫描记录",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    lastScan,
	})
}

// getRules 获取所有规则
func (h *Handlers) getRules(c *gin.Context) {
	rules := h.scanner.GetRules()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    rules,
	})
}

// getRule 获取单个规则
func (h *Handlers) getRule(c *gin.Context) {
	ruleID := c.Param("id")
	rules := h.scanner.GetRules()

	for _, rule := range rules {
		if rule.ID == ruleID {
			c.JSON(http.StatusOK, response{
				Code:    0,
				Message: "success",
				Data:    rule,
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, response{
		Code:    404,
		Message: "规则未找到",
	})
}

// getRulesByStandard 按标准获取规则
func (h *Handlers) getRulesByStandard(c *gin.Context) {
	standard := ComplianceStandard(c.Param("standard"))
	rules := h.scanner.GetRulesByStandard(standard)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    rules,
	})
}

// getRulesByCategory 按类别获取规则
func (h *Handlers) getRulesByCategory(c *gin.Context) {
	category := RuleCategory(c.Param("category"))
	rules := h.scanner.GetRulesByCategory(category)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    rules,
	})
}

// enableRule 启用规则
func (h *Handlers) enableRule(c *gin.Context) {
	ruleID := c.Param("id")
	if err := h.scanner.EnableRule(ruleID); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "规则已启用",
	})
}

// disableRule 禁用规则
func (h *Handlers) disableRule(c *gin.Context) {
	ruleID := c.Param("id")
	if err := h.scanner.DisableRule(ruleID); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "规则已禁用",
	})
}

// ReportRequest 报告生成请求
type ReportRequest struct {
	ScanID  string `json:"scanId"`  // 扫描ID
	Title   string `json:"title"`   // 报告标题
	Format  string `json:"format"`  // 报告格式: pdf, html, json
}

// generateReport 生成合规报告
func (h *Handlers) generateReport(c *gin.Context) {
	var req ReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 获取最近扫描结果
	lastScan := h.scanner.GetLastScan()
	if lastScan == nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "暂无扫描结果，请先执行扫描",
		})
		return
	}

	report, err := h.reporter.GenerateReport(lastScan, req.Title, req.Format)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: "生成报告失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "报告生成成功",
		Data:    report,
	})
}

// getReport 获取报告
func (h *Handlers) getReport(c *gin.Context) {
	reportID := c.Param("id")
	report, err := h.reporter.GetReport(reportID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: "报告未找到",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    report,
	})
}

// downloadReport 下载报告
func (h *Handlers) downloadReport(c *gin.Context) {
	reportID := c.Param("id")
	report, err := h.reporter.GetReport(reportID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: "报告未找到",
		})
		return
	}

	// 根据报告格式设置Content-Type
	contentType := "application/json"
	filename := "compliance-report.json"

	switch report.Format {
	case "pdf":
		contentType = "application/pdf"
		filename = "compliance-report.pdf"
	case "html":
		contentType = "text/html"
		filename = "compliance-report.html"
	}

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, contentType, report.Content)
}

// getRemediations 获取修复建议列表
func (h *Handlers) getRemediations(c *gin.Context) {
	lastScan := h.scanner.GetLastScan()
	if lastScan == nil {
		c.JSON(http.StatusOK, response{
			Code:    0,
			Message: "暂无扫描结果",
			Data:    []RemediationAction{},
		})
		return
	}

	remediations := h.remediator.GetRemediations(lastScan)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    remediations,
	})
}

// getRemediation 获取单个修复建议
func (h *Handlers) getRemediation(c *gin.Context) {
	remediationID := c.Param("id")
	remediation, err := h.remediator.GetRemediation(remediationID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: "修复建议未找到",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    remediation,
	})
}

// executeRemediation 执行单个修复
func (h *Handlers) executeRemediation(c *gin.Context) {
	remediationID := c.Param("id")
	result, err := h.remediator.ExecuteRemediation(remediationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: "执行修复失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "修复执行完成",
		Data:    result,
	})
}

// ExecuteAllRequest 批量执行请求
type ExecuteAllRequest struct {
	RiskLevel SeverityLevel `json:"riskLevel"` // 最大风险等级
	DryRun    bool          `json:"dryRun"`    // 是否试运行
}

// executeAllRemediations 批量执行修复
func (h *Handlers) executeAllRemediations(c *gin.Context) {
	var req ExecuteAllRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 默认只执行低风险修复
		req.RiskLevel = SeverityLow
		req.DryRun = true
	}

	lastScan := h.scanner.GetLastScan()
	if lastScan == nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "暂无扫描结果",
		})
		return
	}

	results, err := h.remediator.ExecuteAll(lastScan, req.RiskLevel, req.DryRun)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: "批量执行失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "批量修复执行完成",
		Data:    results,
	})
}

// getStats 获取统计信息
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.scanner.GetStats()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}
