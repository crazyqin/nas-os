// Package compliancetracker 提供合规审计追踪功能
package compliancetracker

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 合规审计 HTTP 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	complianceGroup := api.Group("/compliance")
	{
		// 规则管理
		complianceGroup.POST("/rules", h.addRule)
		complianceGroup.GET("/rules", h.listRules)
		complianceGroup.GET("/rules/:id", h.getRule)
		complianceGroup.PUT("/rules/:id", h.updateRule)
		complianceGroup.DELETE("/rules/:id", h.deleteRule)

		// 合规检查
		complianceGroup.POST("/checks", h.runCheck)
		complianceGroup.POST("/checks/all", h.runAllChecks)
		complianceGroup.GET("/checks", h.queryChecks)

		// 审计日志
		complianceGroup.GET("/audit-logs", h.getAuditLogs)

		// 报告生成
		complianceGroup.GET("/reports", h.generateReport)

		// 统计
		complianceGroup.GET("/stats", h.getStats)
	}
}

// addRule 添加合规规则.
func (h *Handlers) addRule(c *gin.Context) {
	var rule ComplianceRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.manager.AddRule(&rule); err != nil {
		if err == ErrDuplicateRule {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "合规规则已创建",
		"rule_id": rule.ID,
	})
}

// listRules 列出合规规则.
func (h *Handlers) listRules(c *gin.Context) {
	ruleType := RuleType(c.Query("rule_type"))

	var enabled *bool
	if enabledStr := c.Query("enabled"); enabledStr != "" {
		e := enabledStr == "true"
		enabled = &e
	}

	rules := h.manager.ListRules(ruleType, enabled)
	c.JSON(http.StatusOK, gin.H{
		"rules": rules,
		"total": len(rules),
	})
}

// getRule 获取合规规则.
func (h *Handlers) getRule(c *gin.Context) {
	ruleID := c.Param("id")

	rule, err := h.manager.GetRule(ruleID)
	if err != nil {
		if err == ErrRuleNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rule)
}

// updateRule 更新合规规则.
func (h *Handlers) updateRule(c *gin.Context) {
	ruleID := c.Param("id")

	var rule ComplianceRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	rule.ID = ruleID
	if err := h.manager.UpdateRule(&rule); err != nil {
		if err == ErrRuleNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "合规规则已更新"})
}

// deleteRule 删除合规规则.
func (h *Handlers) deleteRule(c *gin.Context) {
	ruleID := c.Param("id")

	if err := h.manager.DeleteRule(ruleID); err != nil {
		if err == ErrRuleNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "合规规则已删除"})
}

// runCheck 执行合规检查.
func (h *Handlers) runCheck(c *gin.Context) {
	var req struct {
		RuleID     string `json:"rule_id" binding:"required"`
		Target     string `json:"target" binding:"required"`
		TargetType string `json:"target_type"`
		CheckedBy  string `json:"checked_by"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	check, err := h.manager.RunCheck(req.RuleID, req.Target, req.TargetType, req.CheckedBy)
	if err != nil {
		if err == ErrRuleNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, check)
}

// runAllChecks 执行所有检查.
func (h *Handlers) runAllChecks(c *gin.Context) {
	var req struct {
		Target     string `json:"target" binding:"required"`
		TargetType string `json:"target_type"`
		CheckedBy  string `json:"checked_by"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	checks, err := h.manager.RunAllChecks(req.Target, req.TargetType, req.CheckedBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"checks": checks,
		"total":  len(checks),
	})
}

// queryChecks 查询合规检查记录.
func (h *Handlers) queryChecks(c *gin.Context) {
	filter := QueryFilter{
		RuleID:     c.Query("rule_id"),
		Status:     ComplianceStatus(c.Query("status")),
		Target:     c.Query("target"),
		TargetType: c.Query("target_type"),
		Severity:   SeverityLevel(c.Query("severity")),
	}

	if startTime := c.Query("start_time"); startTime != "" {
		t, err := time.Parse(time.RFC3339, startTime)
		if err == nil {
			filter.StartTime = &t
		}
	}
	if endTime := c.Query("end_time"); endTime != "" {
		t, err := time.Parse(time.RFC3339, endTime)
		if err == nil {
			filter.EndTime = &t
		}
	}

	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filter.Limit = l
		}
	}
	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			filter.Offset = o
		}
	}

	checks := h.manager.QueryChecks(filter)
	c.JSON(http.StatusOK, gin.H{
		"checks": checks,
		"total":  len(checks),
	})
}

// getAuditLogs 获取审计日志.
func (h *Handlers) getAuditLogs(c *gin.Context) {
	filter := QueryFilter{
		Target: c.Query("target"),
	}

	if startTime := c.Query("start_time"); startTime != "" {
		t, err := time.Parse(time.RFC3339, startTime)
		if err == nil {
			filter.StartTime = &t
		}
	}
	if endTime := c.Query("end_time"); endTime != "" {
		t, err := time.Parse(time.RFC3339, endTime)
		if err == nil {
			filter.EndTime = &t
		}
	}

	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filter.Limit = l
		}
	}
	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			filter.Offset = o
		}
	}

	logs := h.manager.GetAuditLogs(filter)
	c.JSON(http.StatusOK, gin.H{
		"logs":  logs,
		"total": len(logs),
	})
}

// generateReport 生成合规报告.
func (h *Handlers) generateReport(c *gin.Context) {
	startTimeStr := c.DefaultQuery("start_time", time.Now().AddDate(0, 0, -7).Format(time.RFC3339))
	endTimeStr := c.DefaultQuery("end_time", time.Now().Format(time.RFC3339))

	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的开始时间"})
		return
	}

	endTime, err := time.Parse(time.RFC3339, endTimeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的结束时间"})
		return
	}

	report, err := h.manager.GenerateReport(startTime, endTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, report)
}

// getStats 获取合规统计.
func (h *Handlers) getStats(c *gin.Context) {
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	if start := c.Query("start_time"); start != "" {
		if t, err := time.Parse(time.RFC3339, start); err == nil {
			startTime = t
		}
	}
	if end := c.Query("end_time"); end != "" {
		if t, err := time.Parse(time.RFC3339, end); err == nil {
			endTime = t
		}
	}

	stats, err := h.manager.GetComplianceStats(startTime, endTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}
