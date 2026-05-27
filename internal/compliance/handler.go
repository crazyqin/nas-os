// Package compliance 提供合规报告生成功能
package compliance

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 合规引擎 HTTP 处理器.
type Handlers struct {
	engine *ComplianceEngine
}

// NewHandlers 创建处理器.
func NewHandlers(engine *ComplianceEngine) *Handlers {
	return &Handlers{engine: engine}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	complianceGroup := api.Group("/compliance")
	{
		// 合规标准
		complianceGroup.GET("/standards", h.listStandards)
		complianceGroup.GET("/standards/:id", h.getStandard)

		// 检查项
		complianceGroup.GET("/standards/:id/checks", h.getCheckItems)

		// 合规检查
		complianceGroup.POST("/check/:standard", h.runCheck)

		// 报告
		complianceGroup.GET("/reports", h.listReports)
		complianceGroup.GET("/reports/latest/:standard", h.getLatestReport)

		// 仪表盘
		complianceGroup.GET("/dashboard", h.getDashboard)
	}
}

// listStandards 列出所有合规标准.
func (h *Handlers) listStandards(c *gin.Context) {
	standards := h.engine.GetStandards()
	c.JSON(http.StatusOK, gin.H{
		"standards": standards,
		"total":     len(standards),
	})
}

// getStandard 获取标准详情.
func (h *Handlers) getStandard(c *gin.Context) {
	standardID := ComplianceStandard(c.Param("id"))

	info, err := h.engine.GetStandardInfo(standardID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, info)
}

// getCheckItems 获取标准的检查项.
func (h *Handlers) getCheckItems(c *gin.Context) {
	standardID := ComplianceStandard(c.Param("id"))

	checks, err := h.engine.GetCheckItems(standardID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"checks": checks,
		"total":  len(checks),
	})
}

// runCheck 执行合规检查.
func (h *Handlers) runCheck(c *gin.Context) {
	standardID := ComplianceStandard(c.Param("standard"))

	report, err := h.engine.RunComplianceCheck(standardID)
	if err != nil {
		code := http.StatusInternalServerError
		if err == ErrStandardNotFound {
			code = http.StatusNotFound
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "合规检查完成",
		"report":  report,
	})
}

// listReports 列出所有报告.
func (h *Handlers) listReports(c *gin.Context) {
	reports := h.engine.GetReports()

	// 转换为摘要格式
	summaries := make([]ReportSummary, len(reports))
	for i, report := range reports {
		summaries[i] = ReportSummary{
			ID:          report.ID,
			GeneratedAt: report.GeneratedAt,
			Standard:    report.Standard,
			Score:       report.OverallScore,
			Status:      report.OverallStatus,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"reports": summaries,
		"total":   len(summaries),
	})
}

// getLatestReport 获取最新报告.
func (h *Handlers) getLatestReport(c *gin.Context) {
	standardID := ComplianceStandard(c.Param("standard"))

	report, err := h.engine.GetLatestReport(standardID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, report)
}

// getDashboard 获取仪表盘数据.
func (h *Handlers) getDashboard(c *gin.Context) {
	dashboard := h.engine.GetDashboard()
	c.JSON(http.StatusOK, dashboard)
}
