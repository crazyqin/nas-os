// Package compliance 提供合规中心 REST API 处理器
package compliance

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 合规中心 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	compliance := r.Group("/compliance")
	{
		// 规则管理
		compliance.GET("/rules", h.listRules)
		compliance.POST("/rules", h.addRule)
		compliance.GET("/rules/:id", h.getRule)
		compliance.PUT("/rules/:id", h.updateRule)
		compliance.DELETE("/rules/:id", h.deleteRule)

		// 扫描
		compliance.POST("/scan", h.runScan)
		compliance.GET("/scan/:id", h.getScanResult)

		// 报告
		compliance.POST("/report", h.generateReport)
		compliance.GET("/report", h.listReports)
		compliance.GET("/report/:id", h.getReport)

		// 数据分类
		compliance.POST("/classify", h.classifyData)
		compliance.GET("/classify", h.listClassifications)
		compliance.GET("/classify/:id", h.getClassification)
		compliance.GET("/categories", h.getCategories)

		// 整改计划
		compliance.POST("/plan", h.createPlan)
		compliance.GET("/plan", h.listPlans)
		compliance.GET("/plan/:id", h.getPlan)

		// 法规列表
		compliance.GET("/regulations", h.getRegulations)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// listRules 获取规则列表
func (h *Handlers) listRules(c *gin.Context) {
	regulation := c.Query("regulation")
	rules := h.manager.ListRules(regulation)
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    rules,
	})
}

// addRule 添加规则
func (h *Handlers) addRule(c *gin.Context) {
	var rule ComplianceRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	if err := h.manager.AddRule(&rule); err != nil {
		c.JSON(http.StatusConflict, response{
			Code:    http.StatusConflict,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    http.StatusCreated,
		Message: "Rule added successfully",
		Data:    rule,
	})
}

// getRule 获取规则
func (h *Handlers) getRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.manager.GetRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    rule,
	})
}

// updateRule 更新规则
func (h *Handlers) updateRule(c *gin.Context) {
	id := c.Param("id")
	var rule ComplianceRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	rule.ID = id
	if err := h.manager.UpdateRule(&rule); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "Rule updated successfully",
		Data:    rule,
	})
}

// deleteRule 删除规则
func (h *Handlers) deleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteRule(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "Rule deleted successfully",
	})
}

// runScan 执行扫描
func (h *Handlers) runScan(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	report, err := h.manager.RunScan(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "Scan completed",
		Data:    report,
	})
}

// getScanResult 获取扫描结果
func (h *Handlers) getScanResult(c *gin.Context) {
	id := c.Param("id")
	result, ok := h.manager.scanResults[id]
	if !ok {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: "Scan result not found",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    result,
	})
}

// generateReport 生成报告
func (h *Handlers) generateReport(c *gin.Context) {
	var req struct {
		Regulation string      `json:"regulation"`
		Period     ReportPeriod `json:"period"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	if req.Period.Start.IsZero() {
		req.Period.Start = time.Now().AddDate(0, -1, 0)
	}
	if req.Period.End.IsZero() {
		req.Period.End = time.Now()
	}

	report, err := h.manager.GenerateReport(req.Regulation, req.Period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "Report generated",
		Data:    report,
	})
}

// listReports 获取报告列表
func (h *Handlers) listReports(c *gin.Context) {
	regulation := c.Query("regulation")
	reports := h.manager.ListReports(regulation)
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    reports,
	})
}

// getReport 获取报告
func (h *Handlers) getReport(c *gin.Context) {
	id := c.Param("id")
	report, err := h.manager.GetReport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    report,
	})
}

// classifyData 数据分类
func (h *Handlers) classifyData(c *gin.Context) {
	var req ScanDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	result, err := h.manager.ClassifyData(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "Classification completed",
		Data:    result,
	})
}

// listClassifications 获取分类列表
func (h *Handlers) listClassifications(c *gin.Context) {
	category := c.Query("category")
	classifications := h.manager.ListClassifications(category)
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    classifications,
	})
}

// getClassification 获取分类
func (h *Handlers) getClassification(c *gin.Context) {
	id := c.Param("id")
	cls, err := h.manager.GetClassification(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    cls,
	})
}

// getCategories 获取数据类别
func (h *Handlers) getCategories(c *gin.Context) {
	categories := h.manager.GetCategories()
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    categories,
	})
}

// createPlan 创建整改计划
func (h *Handlers) createPlan(c *gin.Context) {
	reportID := c.Query("report_id")
	if reportID == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "report_id is required",
		})
		return
	}

	var plan RemediationPlan
	if err := c.ShouldBindJSON(&plan); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	if err := h.manager.CreatePlan(reportID, &plan); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    http.StatusCreated,
		Message: "Plan created successfully",
		Data:    plan,
	})
}

// listPlans 获取计划列表
func (h *Handlers) listPlans(c *gin.Context) {
	reportID := c.Query("report_id")
	plans := h.manager.ListPlans(reportID)
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    plans,
	})
}

// getPlan 获取计划
func (h *Handlers) getPlan(c *gin.Context) {
	id := c.Param("id")
	plan, err := h.manager.GetPlan(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    plan,
	})
}

// getRegulations 获取法规列表
func (h *Handlers) getRegulations(c *gin.Context) {
	regulations := h.manager.GetRegulations()
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    regulations,
	})
}
