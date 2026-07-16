// Package stigreport 提供STIG合规报告的HTTP处理器
package stigreport

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers STIG合规报告HTTP处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	stigGroup := api.Group("/stig")
	{
		// 规则管理
		stigGroup.POST("/rules", h.addRule)
		stigGroup.GET("/rules", h.listRules)
		stigGroup.GET("/rules/:id", h.getRule)

		// 检查
		stigGroup.POST("/check/:ruleId", h.runCheck)
		stigGroup.POST("/scan", h.runAutomatedScan)

		// 报告
		stigGroup.POST("/reports", h.generateReport)
		stigGroup.GET("/reports", h.getReports)

		// 调度
		stigGroup.POST("/schedules", h.scheduleScan)
		stigGroup.GET("/schedules", h.getSchedules)

		// 统计
		stigGroup.GET("/stats", h.getStats)
		stigGroup.GET("/rate", h.getComplianceRate)
		stigGroup.GET("/findings", h.getFindings)
	}
}

// addRule 添加规则.
func (h *Handlers) addRule(c *gin.Context) {
	var rule STIGRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.manager.AddRule(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "规则添加成功",
		"rule":    rule,
	})
}

// listRules 列出规则.
func (h *Handlers) listRules(c *gin.Context) {
	var category *CheckCategory
	if cat := c.Query("category"); cat != "" {
		c := CheckCategory(cat)
		category = &c
	}

	rules := h.manager.ListRules(category)
	c.JSON(http.StatusOK, gin.H{
		"rules": rules,
		"total": len(rules),
	})
}

// getRule 获取规则.
func (h *Handlers) getRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.manager.GetRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rule)
}

// runCheck 执行检查.
func (h *Handlers) runCheck(c *gin.Context) {
	ruleID := c.Param("ruleId")
	var req struct {
		Status  ComplianceStatus `json:"status" binding:"required"`
		Details string           `json:"details"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.manager.RunCheck(ruleID, req.Status, req.Details); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "检查完成"})
}

// runAutomatedScan 运行自动扫描.
func (h *Handlers) runAutomatedScan(c *gin.Context) {
	results := h.manager.RunAutomatedScan()
	c.JSON(http.StatusOK, gin.H{
		"message": "自动扫描完成",
		"results": results,
		"total":   len(results),
	})
}

// generateReport 生成报告.
func (h *Handlers) generateReport(c *gin.Context) {
	var req struct {
		Title  string `json:"title" binding:"required"`
		Period string `json:"period" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	report := h.manager.GenerateReport(req.Title, req.Period)
	c.JSON(http.StatusCreated, gin.H{
		"message": "报告生成成功",
		"report":  report,
	})
}

// getReports 获取报告.
func (h *Handlers) getReports(c *gin.Context) {
	reports := h.manager.GetReports(10)
	c.JSON(http.StatusOK, gin.H{
		"reports": reports,
		"total":   len(reports),
	})
}

// scheduleScan 调度扫描.
func (h *Handlers) scheduleScan(c *gin.Context) {
	var schedule ScheduledScan
	if err := c.ShouldBindJSON(&schedule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.manager.ScheduleScan(&schedule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "扫描调度创建成功",
		"schedule": schedule,
	})
}

// getSchedules 获取调度.
func (h *Handlers) getSchedules(c *gin.Context) {
	schedules := h.manager.GetSchedules()
	c.JSON(http.StatusOK, gin.H{
		"schedules": schedules,
		"total":     len(schedules),
	})
}

// getStats 获取统计.
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}

// getComplianceRate 获取合规率.
func (h *Handlers) getComplianceRate(c *gin.Context) {
	rate := h.manager.GetComplianceRate()
	c.JSON(http.StatusOK, gin.H{
		"compliance_rate": rate,
	})
}

// getFindings 获取发现.
func (h *Handlers) getFindings(c *gin.Context) {
	findings := h.manager.GetFindingsBySeverity()
	c.JSON(http.StatusOK, gin.H{
		"findings": findings,
	})
}
