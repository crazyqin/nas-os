// Package complianceengine 提供合规审计引擎功能
package complianceengine

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 合规引擎HTTP处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{
		manager: manager,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	compliance := rg.Group("/complianceengine")
	{
		compliance.GET("/rules", h.GetRules)
		compliance.POST("/rules", h.CreateRule)
		compliance.POST("/scan", h.TriggerScan)
		compliance.GET("/reports", h.GetReports)
		compliance.POST("/reports", h.GenerateReport)
		compliance.GET("/issues", h.GetIssues)
		compliance.PUT("/issues/:id", h.UpdateIssue)
		compliance.GET("/trends", h.GetTrends)
	}
}

// GetRules 获取合规规则列表
func (h *Handler) GetRules(c *gin.Context) {
	standard := c.Query("standard")
	category := c.Query("category")

	rules := h.manager.ListRules(ComplianceStandard(standard), RuleCategory(category))
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    rules,
	})
}

// CreateRule 创建合规规则
func (h *Handler) CreateRule(c *gin.Context) {
	var rule ComplianceRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request: " + err.Error(),
		})
		return
	}

	created, err := h.manager.CreateRule(rule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "create failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "success",
		"data":    created,
	})
}

// TriggerScan 触发合规扫描
func (h *Handler) TriggerScan(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request: " + err.Error(),
		})
		return
	}

	scan, err := h.manager.StartScan([]ComplianceStandard{ComplianceStandard(req.Standard)})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "scan failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "scan started",
		"data":    scan,
	})
}

// GetReports 获取审计报告
func (h *Handler) GetReports(c *gin.Context) {
	reports := h.manager.ListReports()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    reports,
	})
}

// GenerateReport 生成合规报告
func (h *Handler) GenerateReport(c *gin.Context) {
	var req struct {
		ScanID string `json:"scanId" binding:"required"`
		Format string `json:"format"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request: " + err.Error(),
		})
		return
	}

	format := ReportFormat(req.Format)
	if format == "" {
		format = FormatJSON
	}

	report, err := h.manager.GenerateReport(req.ScanID, format)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "generate failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "success",
		"data":    report,
	})
}

// GetIssues 获取问题列表
func (h *Handler) GetIssues(c *gin.Context) {
	status := c.Query("status")
	severity := c.Query("severity")

	alerts := h.manager.ListAlerts(AlertSeverity(severity), status)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    alerts,
	})
}

// UpdateIssue 更新问题状态
func (h *Handler) UpdateIssue(c *gin.Context) {
	id := c.Param("id")
	var req IssueUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request: " + err.Error(),
		})
		return
	}

	// 更新告警状态
	if req.Status == "resolved" {
		if err := h.manager.ResolveAlert(id); err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": err.Error(),
			})
			return
		}
	} else if req.Status == "acknowledged" {
		if err := h.manager.AcknowledgeAlert(id); err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// GetTrends 获取合规趋势
func (h *Handler) GetTrends(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}
