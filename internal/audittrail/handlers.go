// Package audittrail 提供合规审计追踪功能
package audittrail

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers 审计追踪 HTTP 处理器.
type Handlers struct {
	logger *zap.Logger
	mgr    *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(logger *zap.Logger, mgr *Manager) *Handlers {
	return &Handlers{
		logger: logger,
		mgr:    mgr,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	auditGroup := rg.Group("/audit-trail")
	{
		// 审计记录
		auditGroup.POST("/records", h.createRecord)
		auditGroup.GET("/records", h.queryRecords)
		auditGroup.GET("/records/:id", h.getRecord)

		// 操作链
		auditGroup.GET("/chains/:request_id", h.getOperationChain)

		// 异常检测
		auditGroup.GET("/anomalies", h.getAnomalies)
		auditGroup.PUT("/anomalies/:id/resolve", h.resolveAnomaly)

		// 异常规则
		auditGroup.POST("/rules", h.createRule)
		auditGroup.GET("/rules", h.getRules)
		auditGroup.PUT("/rules/:id", h.updateRule)
		auditGroup.DELETE("/rules/:id", h.deleteRule)

		// 合规报告
		auditGroup.POST("/reports", h.generateReport)
		auditGroup.GET("/reports/:id", h.getReport)

		// 统计
		auditGroup.GET("/stats", h.getStats)

		// 导出
		auditGroup.POST("/export", h.exportRecords)
	}
}

// createRecord 创建审计记录.
func (h *Handlers) createRecord(c *gin.Context) {
	var record AuditRecord
	if err := c.ShouldBindJSON(&record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.mgr.RecordOperation(&record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "记录创建失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "审计记录已创建",
		"record_id":  record.ID,
		"checksum":   record.Checksum,
		"expires_at": record.ExpiresAt,
	})
}

// getRecord 获取审计记录.
func (h *Handlers) getRecord(c *gin.Context) {
	id := c.Param("id")
	record, err := h.mgr.GetRecord(id)
	if err != nil {
		if err == ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, record)
}

// queryRecords 查询审计记录.
func (h *Handlers) queryRecords(c *gin.Context) {
	query := AuditQuery{
		UserID:       c.Query("user_id"),
		UserName:     c.Query("user_name"),
		UserIP:       c.Query("user_ip"),
		Resource:     c.Query("resource"),
		ResourceType: c.Query("resource_type"),
		RequestID:    c.Query("request_id"),
	}

	if action := c.Query("action"); action != "" {
		query.Action = ActionType(action)
	}
	if result := c.Query("result"); result != "" {
		query.Result = ActionResult(result)
	}
	if tag := c.Query("compliance_tag"); tag != "" {
		query.ComplianceTag = ComplianceStandard(tag)
	}

	if startTime := c.Query("start_time"); startTime != "" {
		t, err := time.Parse(time.RFC3339, startTime)
		if err == nil {
			query.StartTime = &t
		}
	}
	if endTime := c.Query("end_time"); endTime != "" {
		t, err := time.Parse(time.RFC3339, endTime)
		if err == nil {
			query.EndTime = &t
		}
	}

	records := h.mgr.QueryRecords(query)
	c.JSON(http.StatusOK, gin.H{
		"records": records,
		"total":   len(records),
	})
}

// getOperationChain 获取操作链.
func (h *Handlers) getOperationChain(c *gin.Context) {
	requestID := c.Param("request_id")
	chain, err := h.mgr.GetOperationChain(requestID)
	if err != nil {
		if err == ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "操作链不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, chain)
}

// getAnomalies 获取异常列表.
func (h *Handlers) getAnomalies(c *gin.Context) {
	var resolved *bool
	if r := c.Query("resolved"); r != "" {
		val := r == "true"
		resolved = &val
	}

	anomalies := h.mgr.GetAnomalies(resolved)
	c.JSON(http.StatusOK, gin.H{
		"anomalies": anomalies,
		"total":     len(anomalies),
	})
}

// resolveAnomaly 解决异常.
func (h *Handlers) resolveAnomaly(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		ResolvedBy string `json:"resolved_by" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.mgr.ResolveAnomaly(id, req.ResolvedBy); err != nil {
		if err == ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "异常不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "异常已标记为已解决"})
}

// createRule 创建异常规则.
func (h *Handlers) createRule(c *gin.Context) {
	var rule AnomalyRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	h.mgr.AddAnomalyRule(&rule)
	c.JSON(http.StatusCreated, gin.H{
		"message": "规则已创建",
		"rule_id": rule.ID,
	})
}

// getRules 获取所有规则.
func (h *Handlers) getRules(c *gin.Context) {
	rules := h.mgr.GetAnomalyRules()
	c.JSON(http.StatusOK, gin.H{
		"rules": rules,
		"total": len(rules),
	})
}

// updateRule 更新规则.
func (h *Handlers) updateRule(c *gin.Context) {
	id := c.Param("id")
	var rule AnomalyRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	rule.ID = id

	if err := h.mgr.UpdateAnomalyRule(&rule); err != nil {
		if err == ErrRuleNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "规则已更新"})
}

// deleteRule 删除规则.
func (h *Handlers) deleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.DeleteAnomalyRule(id); err != nil {
		if err == ErrRuleNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "规则已删除"})
}

// generateReport 生成合规报告.
func (h *Handlers) generateReport(c *gin.Context) {
	var req struct {
		Standard ComplianceStandard `json:"standard" binding:"required"`
		Start    time.Time          `json:"start" binding:"required"`
		End      time.Time          `json:"end" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	report, err := h.mgr.GenerateComplianceReport(req.Standard, req.Start, req.End)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "报告生成失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, report)
}

// getReport 获取报告.
func (h *Handlers) getReport(c *gin.Context) {
	id := c.Param("id")
	report, err := h.mgr.GetComplianceReport(id)
	if err != nil {
		if err == ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "报告不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, report)
}

// getStats 获取统计信息.
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.mgr.GetComplianceStats()
	c.JSON(http.StatusOK, stats)
}

// exportRecords 导出记录.
func (h *Handlers) exportRecords(c *gin.Context) {
	var req ExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	data, err := h.mgr.ExportRecords(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导出失败: " + err.Error()})
		return
	}

	contentType := "application/octet-stream"
	switch req.Format {
	case FormatJSON:
		contentType = "application/json"
	case FormatCSV:
		contentType = "text/csv"
	case FormatPDF:
		contentType = "application/pdf"
	}

	fileName := req.FileName
	if fileName == "" {
		fileName = "audit_export." + string(req.Format)
	}

	c.Header("Content-Disposition", "attachment; filename="+fileName)
	c.Data(http.StatusOK, contentType, data)
}
