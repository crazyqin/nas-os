// Package accessaudit 提供零信任访问审计功能
package accessaudit

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 访问审计 HTTP 处理器.
type Handlers struct {
	auditor *Auditor
}

// NewHandlers 创建处理器.
func NewHandlers(auditor *Auditor) *Handlers {
	return &Handlers{auditor: auditor}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	auditGroup := api.Group("/access-audit")
	{
		// 访问记录
		auditGroup.POST("/records", h.recordAccess)
		auditGroup.GET("/records", h.queryRecords)
		auditGroup.GET("/records/:id", h.getRecord)

		// 异常检测
		auditGroup.GET("/anomalies", h.getAnomalies)
		auditGroup.PUT("/anomalies/:id/resolve", h.resolveAnomaly)

		// 审计报告
		auditGroup.GET("/reports", h.generateReport)

		// 风险统计
		auditGroup.GET("/risk-stats", h.getRiskStats)
	}
}

// recordAccess 记录访问.
func (h *Handlers) recordAccess(c *gin.Context) {
	var record AccessRecord
	if err := c.ShouldBindJSON(&record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	h.auditor.RecordAccess(&record)
	c.JSON(http.StatusCreated, gin.H{
		"message":    "访问记录已创建",
		"record_id":  record.ID,
		"risk_score": record.RiskScore,
		"risk_level": record.RiskLevel,
	})
}

// queryRecords 查询访问记录.
func (h *Handlers) queryRecords(c *gin.Context) {
	query := AccessQuery{
		UserID:       c.Query("user_id"),
		SourceIP:     c.Query("source_ip"),
		Resource:     c.Query("resource"),
		ResourceType: c.Query("resource_type"),
		Action:       c.Query("action"),
		Status:       AccessStatus(c.Query("status")),
		RiskLevel:    RiskLevel(c.Query("risk_level")),
	}

	// 解析时间参数
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

	// 解析分页参数
	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			query.Limit = l
		}
	}
	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			query.Offset = o
		}
	}

	// 解析风险评分
	if minScore := c.Query("min_risk_score"); minScore != "" {
		if s, err := strconv.ParseFloat(minScore, 64); err == nil {
			query.MinRiskScore = &s
		}
	}

	records := h.auditor.Query(query)
	c.JSON(http.StatusOK, gin.H{
		"records": records,
		"total":   len(records),
	})
}

// getRecord 获取单条记录.
func (h *Handlers) getRecord(c *gin.Context) {
	recordID := c.Param("id")

	query := AccessQuery{}
	records := h.auditor.Query(query)

	for _, record := range records {
		if record.ID == recordID {
			c.JSON(http.StatusOK, record)
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": ErrRecordNotFound.Error()})
}

// getAnomalies 获取异常列表.
func (h *Handlers) getAnomalies(c *gin.Context) {
	anomalies := h.auditor.GetAnomalies()
	c.JSON(http.StatusOK, gin.H{
		"anomalies": anomalies,
		"total":     len(anomalies),
	})
}

// resolveAnomaly 标记异常为已解决.
func (h *Handlers) resolveAnomaly(c *gin.Context) {
	anomalyID := c.Param("id")

	if h.auditor.ResolveAnomaly(anomalyID) {
		c.JSON(http.StatusOK, gin.H{"message": "异常已标记为已解决"})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "异常记录不存在"})
	}
}

// generateReport 生成审计报告.
func (h *Handlers) generateReport(c *gin.Context) {
	// 解析时间范围
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

	report, err := h.auditor.GenerateReport(startTime, endTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, report)
}

// getRiskStats 获取风险统计.
func (h *Handlers) getRiskStats(c *gin.Context) {
	// 默认查询最近24小时
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

	report, err := h.auditor.GenerateReport(startTime, endTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total_records":     report.TotalRecords,
		"avg_risk_score":    report.AvgRiskScore,
		"high_risk_count":   report.HighRiskCount,
		"risk_distribution": report.RiskDistribution,
		"anomaly_count":     len(report.Anomalies),
	})
}
