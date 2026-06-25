// Package compliancereport 提供合规报告 HTTP API
package compliance

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 合规报告 HTTP 处理器.
type Handlers struct {
	generator *ReportGenerator
	standards *StandardsManager
	exporter  *PDFExporter
}

// NewHandlers 创建合规报告处理器.
func NewHandlers(generator *ReportGenerator, standards *StandardsManager) *Handlers {
	return &Handlers{
		generator: generator,
		standards: standards,
		exporter:  NewPDFExporter(standards),
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	cr := api.Group("/compliance-report")
	{
		cr.GET("/standards", h.listStandards)
		cr.POST("/scan", h.triggerScan)
		cr.GET("/reports", h.listReports)
		cr.GET("/reports/:id", h.getReport)
		cr.GET("/reports/:id/export", h.exportReport)
		cr.GET("/status", h.getStatus)
	}
}

// APIResponse 通用 API 响应.
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func okResp(data interface{}) APIResponse {
	return APIResponse{Code: 0, Message: "success", Data: data}
}

func errResp(code int, message string) APIResponse {
	return APIResponse{Code: code, Message: message}
}

// GET /api/v1/compliance-report/standards
func (h *Handlers) listStandards(c *gin.Context) {
	standards := h.standards.ListStandards()
	c.JSON(http.StatusOK, okResp(standards))
}

// POST /api/v1/compliance-report/scan
func (h *Handlers) triggerScan(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errResp(400, "请求参数错误: "+err.Error()))
		return
	}

	report, err := h.generator.GenerateReport(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, errResp(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, okResp(report))
}

// GET /api/v1/compliance-report/reports
func (h *Handlers) listReports(c *gin.Context) {
	var standard *ComplianceStandard
	if s := c.Query("standard"); s != "" {
		cs := ComplianceStandard(s)
		standard = &cs
	}

	reports := h.generator.ListReports(standard)
	c.JSON(http.StatusOK, okResp(reports))
}

// GET /api/v1/compliance-report/reports/:id
func (h *Handlers) getReport(c *gin.Context) {
	id := c.Param("id")
	report, ok := h.generator.GetReport(id)
	if !ok {
		c.JSON(http.StatusNotFound, errResp(404, "报告不存在"))
		return
	}

	c.JSON(http.StatusOK, okResp(report))
}

// GET /api/v1/compliance-report/reports/:id/export
func (h *Handlers) exportReport(c *gin.Context) {
	id := c.Param("id")
	report, exists := h.generator.GetReport(id)
	if !exists {
		c.JSON(http.StatusNotFound, errResp(404, "报告不存在"))
		return
	}

	format := c.DefaultQuery("format", "html")
	switch format {
	case "text":
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(http.StatusOK, h.exporter.ExportToText(report))
	case "html", "pdf":
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, h.exporter.ExportToHTML(report))
	default:
		c.JSON(http.StatusBadRequest, errResp(400, "不支持的导出格式，支持: html, text"))
	}
}

// GET /api/v1/compliance-report/status
func (h *Handlers) getStatus(c *gin.Context) {
	status := h.generator.GetStatus()
	c.JSON(http.StatusOK, okResp(status))
}
