// Package compliancecenter HTTP API 处理器
package compliancecenter

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	cc *ComplianceCenter
}

// NewHandler 创建处理器
func NewHandler(cc *ComplianceCenter) *Handler {
	return &Handler{cc: cc}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	group := rg.Group("/compliance-center")
	{
		group.GET("/standards", h.ListStandards)
		group.GET("/checks", h.ListChecks)
		group.POST("/checks/run", h.RunChecks)
		group.GET("/reports", h.ListReports)
		group.POST("/reports/generate", h.GenerateReport)
		group.GET("/findings", h.ListFindings)
		group.GET("/stats", h.GetStats)
	}
}

// ListStandards 获取合规标准列表
func (h *Handler) ListStandards(c *gin.Context) {
	standards := []gin.H{
		{"id": StandardGDPR, "name": "GDPR", "description": "通用数据保护条例"},
		{"id": StandardCCPA, "name": "CCPA", "description": "加州消费者隐私法案"},
		{"id": StandardHIPAA, "name": "HIPAA", "description": "健康保险可携性和责任法案"},
		{"id": StandardSOX, "name": "SOX", "description": "萨班斯-奥克斯利法案"},
		{"id": StandardPCIDSS, "name": "PCI-DSS", "description": "支付卡行业数据安全标准"},
		{"id": StandardISO27001, "name": "ISO27001", "description": "信息安全管理体系"},
		{"id": StandardMLPS2, "name": "MLPS2.0", "description": "等保2.0"},
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": standards})
}

// ListChecks 获取检查项列表
func (h *Handler) ListChecks(c *gin.Context) {
	standard := ComplianceStandard(c.Query("standard"))
	checks := h.cc.ListChecks(standard)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": checks, "total": len(checks)})
}

// RunChecksRequest 运行检查请求
type RunChecksRequest struct {
	Standard ComplianceStandard `json:"standard"`
	CheckIDs []string           `json:"checkIds"`
}

// RunChecks 运行合规检查
func (h *Handler) RunChecks(c *gin.Context) {
	var req RunChecksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	checks := h.cc.ListChecks(req.Standard)
	results := make([]*ComplianceCheck, 0)
	for _, check := range checks {
		if len(req.CheckIDs) > 0 {
			found := false
			for _, id := range req.CheckIDs {
				if check.ID == id {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		check.LastChecked = time.Now()
		check.Status = "checked"
		results = append(results, check)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"checked": len(results),
			"results": results,
		},
	})
}

// ListReports 获取报告列表
func (h *Handler) ListReports(c *gin.Context) {
	standard := ComplianceStandard(c.Query("standard"))
	reports := h.cc.ListReports(standard)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": reports, "total": len(reports)})
}

// GenerateReportRequest 生成报告请求
type GenerateReportRequest struct {
	Standard ComplianceStandard `json:"standard" binding:"required"`
}

// GenerateReport 生成合规报告
func (h *Handler) GenerateReport(c *gin.Context) {
	var req GenerateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	report, err := h.cc.GenerateReport(req.Standard)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": report})
}

// ListFindings 获取发现列表
func (h *Handler) ListFindings(c *gin.Context) {
	standard := ComplianceStandard(c.Query("standard"))
	reports := h.cc.ListReports(standard)

	findings := make([]ComplianceCheck, 0)
	for _, report := range reports {
		findings = append(findings, report.Findings...)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": findings, "total": len(findings)})
}

// GetStats 获取统计信息
func (h *Handler) GetStats(c *gin.Context) {
	checks := h.cc.ListChecks("")
	reports := h.cc.ListReports("")

	totalChecks := len(checks)
	passedChecks := 0
	for _, check := range checks {
		if check.Score >= check.MaxScore*0.8 {
			passedChecks++
		}
	}

	totalReports := len(reports)
	compliantReports := 0
	for _, report := range reports {
		if report.Status == "compliant" {
			compliantReports++
		}
	}

	stats := gin.H{
		"totalChecks":      totalChecks,
		"passedChecks":     passedChecks,
		"failedChecks":     totalChecks - passedChecks,
		"totalReports":     totalReports,
		"compliantReports": compliantReports,
		"standards": []string{
			string(StandardGDPR),
			string(StandardCCPA),
			string(StandardHIPAA),
			string(StandardSOX),
			string(StandardPCIDSS),
			string(StandardISO27001),
			string(StandardMLPS2),
		},
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}
